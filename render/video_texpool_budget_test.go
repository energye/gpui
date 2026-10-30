//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !nogpu

package render

import (
	"testing"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/hal/noop"
	"github.com/energye/gpui/gpu/types"
)

// TestVideoTexturePool_MaxSlotsEviction pins the pool-slot budget semantics
// (BudgetVideoStage.MaxEntries, pre-P2.4b value 8): when live+idle reaches
// the cap, the oldest idle slot is evicted before a new size is created.
func TestVideoTexturePool_MaxSlotsEviction(t *testing.T) {
	be := noop.NewBackend()
	inst, err := be.CreateInstance(nil)
	if err != nil {
		t.Fatalf("noop CreateInstance: %v", err)
	}
	defer inst.Release()
	adapters := inst.EnumerateAdapters(nil)
	if len(adapters) == 0 {
		t.Fatal("noop: no adapters")
	}
	opened, err := adapters[0].Adapter.Open(0, types.Limits{})
	if err != nil {
		t.Fatalf("noop Open: %v", err)
	}
	defer opened.Device.Release()
	dev := opened.Device

	p := NewVideoTexturePool(dev)
	p.mu.Lock()
	p.maxSlots = 2
	p.mu.Unlock()

	s1, err := p.Acquire(64, 64)
	if err != nil {
		t.Fatalf("Acquire 1: %v", err)
	}
	s2, err := p.Acquire(96, 96)
	if err != nil {
		t.Fatalf("Acquire 2: %v", err)
	}
	p.Release(s1) // idle: [64x64]; live: {96x96}
	p.Release(s2) // idle: [64x64, 96x96]

	// Third distinct size at cap: the oldest idle slot (64x64) is evicted.
	s3, err := p.Acquire(128, 128)
	if err != nil {
		t.Fatalf("Acquire 3: %v", err)
	}
	defer s3.Texture.Destroy()
	if p.evictions != 1 {
		t.Fatalf("evictions = %d, want 1", p.evictions)
	}
	p.mu.Lock()
	idle := len(p.idle)
	p.mu.Unlock()
	if idle != 1 {
		t.Fatalf("idle slots = %d, want 1 (oldest evicted)", idle)
	}
	// Sanity: the slot API contract the eviction depends on.
	var _ hal.Texture = s2.Texture
}
