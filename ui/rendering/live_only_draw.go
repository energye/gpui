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
// A display list cannot describe everything: GPU texture views and video frames
// ride handles that only exist on the thread owning the device. Paint code must
// route such draws through the façades here rather than type-asserting pc.DC —
// on a recording walk a façade returns false and counts, so "painted nothing
// because the target cannot hold it" always leaves a number behind instead of an
// invisible hole.
var liveOnlySkips atomic.Int64

// LiveOnlySkips returns how many live-only draws met a recording walk and were
// refused (process-wide). Read it before and after a paint to prove a window
// records nothing it cannot replay.
func LiveOnlySkips() int64 { return liveOnlySkips.Load() }

// ResetLiveOnlySkips zeroes the counter (tests / per-window measurement).
func ResetLiveOnlySkips() { liveOnlySkips.Store(0) }

// RequireLiveContext hands paint code the live *render.Context for a draw a
// display list cannot describe: masks, GPU texture views, glyph-outline
// extraction. On a recording walk it returns nil, false and counts the refusal
// in LiveOnlySkips, so "skipped because unrecordable" always leaves a number
// instead of an invisible hole.
func (pc *PaintContext) RequireLiveContext() (*render.Context, bool) {
	dc, live := pc.LiveContext()
	if !live {
		liveOnlySkips.Add(1)
	}
	return dc, live
}

// DrawGPUTexture paints a live GPU texture view into the current walk
// (render.Context.DrawGPUTexture subset). x,y are absolute canvas coordinates
// (use pc.Abs), width/height are device px.
//
// Returns false without drawing when the walk records into a display list; the
// refusal is counted in LiveOnlySkips.
func DrawGPUTexture(pc *PaintContext, view render.TextureView, x, y float64, width, height int) bool {
	dc, live := pc.RequireLiveContext()
	if !live {
		return false
	}
	dc.DrawGPUTexture(view, x, y, width, height)
	return true
}
