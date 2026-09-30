//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/hal/noop"
	gputypes "github.com/energye/gpui/gpu/types"
)

type videoPoolCase struct {
	W int `json:"w"`
	H int `json:"h"`
}

func loadVideoPoolCases(t *testing.T) []videoPoolCase {
	t.Helper()
	path := filepath.Join("testdata", "video_direct_pool.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var v struct {
		Cases []videoPoolCase `json:"cases"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	if len(v.Cases) == 0 {
		t.Fatalf("%s has no cases", path)
	}
	return v.Cases
}

func openNoopVideoDevice(t *testing.T) hal.Device {
	t.Helper()
	inst, err := noop.Backend{}.CreateInstance(&hal.InstanceDescriptor{Backends: gputypes.BackendsPrimary})
	if err != nil {
		t.Fatalf("noop CreateInstance: %v", err)
	}
	ad, err := inst.RequestAdapter(nil)
	if err != nil {
		t.Fatalf("noop RequestAdapter: %v", err)
	}
	dev, err := ad.RequestDevice(nil)
	if err != nil {
		t.Fatalf("noop RequestDevice: %v", err)
	}
	return dev
}

// stubQueue forces the command-copy capability one way or the other.
type stubQueue struct {
	hal.Queue
	copies bool
}

func (q stubQueue) SupportsCommandBufferCopies() bool { return q.copies }

// stubDevice overrides Queue/Limits only, rest passes through.
type stubDevice struct {
	hal.Device
	queue hal.Queue
}

func (d stubDevice) Queue() hal.Queue { return d.queue }

func TestVideoBackendQueryBothCopyModes(t *testing.T) {
	dev := openNoopVideoDevice(t)
	base := dev.Queue()
	if base == nil {
		t.Fatal("noop queue is nil")
	}
	for _, copies := range []bool{false, true} {
		sd := stubDevice{Device: dev, queue: stubQueue{Queue: base, copies: copies}}
		caps := QueryVideoBackend(sd)
		if !caps.HasDevice {
			t.Fatalf("copies=%v: HasDevice=false", copies)
		}
		if caps.SupportsCommandCopies != copies {
			t.Fatalf("copies=%v: got %v", copies, caps.SupportsCommandCopies)
		}
		if caps.MaxTexture2D == 0 {
			t.Fatalf("copies=%v: MaxTexture2D=0", copies)
		}
		if caps.MaxStagingBytes == 0 {
			t.Fatalf("copies=%v: MaxStagingBytes=0", copies)
		}
		if caps.RowPitchAlignment != 256 {
			t.Fatalf("copies=%v: RowPitchAlignment=%d, want 256", copies, caps.RowPitchAlignment)
		}
	}
	if caps := QueryVideoBackend(nil); caps.HasDevice {
		t.Fatal("QueryVideoBackend(nil).HasDevice=true, want false")
	}
}

func TestVideoBorrowWithoutWindow(t *testing.T) {
	_, _, _, _, ok := BorrowVideoBackend()
	if ok {
		t.Skip("shared device is open in this process, borrow path covered elsewhere")
	}
}

func TestVideoPoolBorrowReturnNoLeak(t *testing.T) {
	cases := loadVideoPoolCases(t)
	dev := openNoopVideoDevice(t)
	pool := NewVideoTexturePool(dev)
	defer pool.Close()
	var slots []*VideoSlot
	for _, c := range cases {
		s, err := pool.Acquire(c.W, c.H)
		if err != nil {
			t.Fatalf("Acquire %dx%d: %v", c.W, c.H, err)
		}
		if s.Texture == nil || s.View == nil {
			t.Fatalf("Acquire %dx%d returned nil texture/view", c.W, c.H)
		}
		slots = append(slots, s)
	}
	st := pool.Stats()
	if st.Live != len(cases) {
		t.Fatalf("Live=%d, want %d", st.Live, len(cases))
	}
	for _, s := range slots {
		pool.Release(s)
	}
	st = pool.Stats()
	if st.Live != 0 || st.Idle != len(cases) {
		t.Fatalf("after release Live=%d Idle=%d, want 0/%d", st.Live, st.Idle, len(cases))
	}
	if st.Evictions != 0 {
		t.Fatalf("Evictions=%d, want 0", st.Evictions)
	}
	// Same size reuses idle slot in place.
	first := cases[0]
	a, err := pool.Acquire(first.W, first.H)
	if err != nil {
		t.Fatalf("re-acquire: %v", err)
	}
	pool.Release(a)
	st = pool.Stats()
	if st.Live != 0 {
		t.Fatalf("Live=%d after reuse cycle, want 0", st.Live)
	}
}

func TestVideoFallbackCountsCPU(t *testing.T) {
	dc := NewContext(64, 64)
	before := dc.RenderPathStats().CPUFallbackOps
	beforeTotal := VideoFallbackTotal()
	dc.RecordVideoFallback("no-device")
	st := dc.RenderPathStats()
	if st.CPUFallbackOps != before+1 {
		t.Fatalf("CPUFallbackOps=%d, want %d", st.CPUFallbackOps, before+1)
	}
	if st.LastCPUFallbackReason != "video:no-device" {
		t.Fatalf("reason=%q, want video:no-device", st.LastCPUFallbackReason)
	}
	if VideoFallbackTotal() != beforeTotal+1 {
		t.Fatalf("VideoFallbackTotal=%d, want %d", VideoFallbackTotal(), beforeTotal+1)
	}
}
