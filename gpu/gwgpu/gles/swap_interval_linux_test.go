//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build linux && !(js && wasm)

package gles

import (
	"testing"

	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
)

// The EGL swap interval is the only lever for driver-side pacing, and a wrong
// mapping either reintroduces the double vblank wait (Fifo mapped to 0) or
// tears (Immediate mapped to 1). Same mapping as the WGL twin
// (resource_windows.go) and metal's setDisplaySyncEnabled:.
func TestSwapIntervalForPresentMode(t *testing.T) {
	cases := []struct {
		mode gputypes.PresentMode
		want int32
	}{
		{hal.PresentModeFifo, 1},
		{hal.PresentModeFifoRelaxed, 1},
		{hal.PresentModeImmediate, 0},
		{hal.PresentModeMailbox, 0},
		{gputypes.PresentModeUndefined, 1},
	}
	for _, c := range cases {
		if got := int32(swapIntervalForPresentMode(c.mode)); got != c.want {
			t.Errorf("swapIntervalForPresentMode(%v) = %d, want %d", c.mode, got, c.want)
		}
	}
}

// applySwapInterval runs on every present, so its guards matter: no EGL
// surface (headless/pbuffer configure) and no surface configuration must stay
// silent no-ops instead of reaching the FFI or panicking.
func TestApplySwapIntervalGuards(t *testing.T) {
	var nilSurf *Surface
	nilSurf.applySwapInterval() // must not panic

	(&Surface{}).applySwapInterval() // no EGL surface, no config

	s := &Surface{}
	s.config = &hal.SurfaceConfiguration{PresentMode: hal.PresentModeImmediate}
	s.applySwapInterval() // config present, EGL surface still 0

	if s.swapIntervalMode != gputypes.PresentModeUndefined {
		t.Errorf("guarded call recorded mode %v, want Undefined", s.swapIntervalMode)
	}
}
