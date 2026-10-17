//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build (windows || linux) && !(js && wasm)

package gles

import (
	"testing"

	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
)

// GL implements exactly two present modes (Fifo = blocking swap, Immediate =
// non-blocking swap), so a caller's preference list must resolve to one of
// them and never to a mode the backend cannot honour. WebGPU twin:
// pickPresentMode with FifoRelaxed/Mailbox supported natively.
func TestGLPresentModePreference(t *testing.T) {
	cases := []struct {
		name   string
		prefer []gputypes.PresentMode
		want   gputypes.PresentMode
	}{
		{"no preference keeps the Fifo default", nil, hal.PresentModeFifo},
		{"vsync preference stays blocking", []gputypes.PresentMode{hal.PresentModeFifo, hal.PresentModeImmediate}, hal.PresentModeFifo},
		{"non-blocking preference resolves to Immediate", []gputypes.PresentMode{hal.PresentModeFifoRelaxed, hal.PresentModeMailbox, hal.PresentModeImmediate}, hal.PresentModeImmediate},
		{"explicit Immediate", []gputypes.PresentMode{hal.PresentModeImmediate}, hal.PresentModeImmediate},
		{"unimplementable list falls back to Fifo", []gputypes.PresentMode{hal.PresentModeFifoRelaxed, hal.PresentModeMailbox}, hal.PresentModeFifo},
	}
	for _, c := range cases {
		if got := pickGLPresentMode(c.prefer); got != c.want {
			t.Errorf("%s: pickGLPresentMode(%v) = %v, want %v", c.name, c.prefer, got, c.want)
		}
	}
}

// Both preference setters must feed pickGLPresentMode: SetPreferVSync keeps the
// blocking swap, SetPreferFifoRelaxed (the "framework paces" request) must not.
func TestSwapchainPreferenceSetters(t *testing.T) {
	sc := &Swapchain{}
	sc.SetPreferVSync()
	if got := pickGLPresentMode(sc.PreferPresentModes); got != hal.PresentModeFifo {
		t.Errorf("SetPreferVSync resolves to %v, want Fifo", got)
	}
	sc.SetPreferFifoRelaxed()
	if got := pickGLPresentMode(sc.PreferPresentModes); got != hal.PresentModeImmediate {
		t.Errorf("SetPreferFifoRelaxed resolves to %v, want Immediate", got)
	}
}
