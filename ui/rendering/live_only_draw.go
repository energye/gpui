//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package rendering

import (
	"sync/atomic"

	"github.com/energye/gpui/render"
)

// Live-only draws.
//
// A display list cannot describe everything: masks, glyph-outline extraction and
// offscreen passes ride handles that only exist on the thread owning the device.
// Paint code must route such draws through the façades here rather than
// type-asserting pc.DC — on a recording walk a façade returns false and counts,
// so "painted nothing because the target cannot hold it" always leaves a number
// behind instead of an invisible hole.
var liveOnlySkips atomic.Int64

// LiveOnlySkips returns how many live-only draws met a recording walk and were
// refused (process-wide). Read it before and after a paint to prove a window
// records nothing it cannot replay.
func LiveOnlySkips() int64 { return liveOnlySkips.Load() }

// ResetLiveOnlySkips zeroes the counter (tests / per-window measurement).
func ResetLiveOnlySkips() { liveOnlySkips.Store(0) }

// RequireLiveContext hands paint code the live *render.Context for a draw a
// display list cannot describe: masks, GPU flushes, glyph-outline extraction.
// On a recording walk it returns nil, false and counts the refusal in
// LiveOnlySkips, so "skipped because unrecordable" always leaves a number
// instead of an invisible hole.
func (pc *PaintContext) RequireLiveContext() (*render.Context, bool) {
	dc, live := pc.LiveContext()
	if !live {
		liveOnlySkips.Add(1)
	}
	return dc, live
}

// gpuTextureCanvas is a draw target that can take a device-owned texture view:
// the live *render.Context queues the quad, and a display-list Recorder stores
// the command so playback binds the view on the raster thread (Flutter
// TextureLayer semantics).
type gpuTextureCanvas interface {
	DrawGPUTexture(view render.TextureView, x, y float64, width, height int)
}

// DrawGPUTexture paints a live GPU texture view into the current walk
// (render.Context.DrawGPUTexture subset). x,y are in the walk's user space
// (pc.Abs for origin-relative paint), width/height in the same space.
//
// A recording walk stores the command instead of refusing it; only a target
// with no texture capability is refused, counted in LiveOnlySkips.
func DrawGPUTexture(pc *PaintContext, view render.TextureView, x, y float64, width, height int) bool {
	if pc == nil || pc.DC == nil {
		liveOnlySkips.Add(1)
		return false
	}
	target, ok := pc.DC.(gpuTextureCanvas)
	if !ok {
		liveOnlySkips.Add(1)
		return false
	}
	target.DrawGPUTexture(view, x, y, width, height)
	return true
}
