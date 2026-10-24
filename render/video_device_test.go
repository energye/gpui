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
	"testing"

	"github.com/energye/gpui/gpu/hal"
)

// fakeVideoDevice satisfies hal.Device through the embedded interface; the
// test only compares identities, it never calls a device method.
type fakeVideoDevice struct{ hal.Device }

// fakeVideoDeviceCtx satisfies gpuContextOps through the embedded interface and
// adds the optional Device() probe that videoDeviceMatches reads.
type fakeVideoDeviceCtx struct {
	gpuContextOps
	dev hal.Device
}

func (f fakeVideoDeviceCtx) Device() hal.Device { return f.dev }

// TestVideoDeviceMatches locks V-3: a slot built on device A must be rejected
// by a session drawing on device B. Accepting it queues work that never lands
// (the bridge then records a direct draw with no pixels), so the caller must
// take the counted CPU fallback instead.
func TestVideoDeviceMatches(t *testing.T) {
	devA := &fakeVideoDevice{}
	devB := &fakeVideoDevice{}

	if !videoDeviceMatches(fakeVideoDeviceCtx{dev: devA}, devA) {
		t.Fatal("same device rejected: the direct path would fall back for no reason")
	}
	if videoDeviceMatches(fakeVideoDeviceCtx{dev: devA}, devB) {
		t.Fatal("cross-device slot accepted (V-3): the queued quad can never land")
	}
	if !videoDeviceMatches(fakeVideoDeviceCtx{dev: nil}, devB) {
		t.Fatal("unknown drawing device rejected: CPU-only/optional sessions must keep the old path")
	}
	if !videoDeviceMatches(fakeVideoDeviceCtx{dev: devA}, nil) {
		t.Fatal("unknown slot device rejected: an unowned slot must keep the old path")
	}
	if !videoDeviceMatches(nil, devB) {
		t.Fatal("nil context rejected: the optional probe must fail open")
	}
}
