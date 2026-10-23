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
	"math"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/recording"
)

// RenderPictureBox is a leaf box whose own content is a retained display
// list built by Record (Flutter RepaintBoundary with a full display list).
//
// Unlike RenderColorBox/RenderText/RenderImage (own content reconstructed
// from fields), a picture box records arbitrary vector content: circles,
// ovals, arcs, round rects, lines, gradients, paths, text and images.
// Live paint and cache record run the SAME Record closure, so replay can
// never diverge from live paint by construction.
//
// Cache contract (correctness-first): Record must be a pure function of the
// box state. The cache holds no content fingerprint for this type
// (contentKeyOf returns 0) and relies on NeedsPaint for invalidation —
// call MarkNeedsPaint whenever the produced commands would change, otherwise
// clean frames replay a stale display list. Static content never changes, so
// it skips forever after the first record. Use as a RepaintBoundary; a
// non-boundary picture box inside an AbsoluteBox disables that parent's
// caching (absoluteContentCacheable rejects it) instead of risking a stale
// bake — same discipline as Viewport (boundaryCacheable).
type RenderPictureBox struct {
	Base
	Width, Height float64
	// Record builds the box's own display list at absolute origin (ox,oy) on
	// any render.Canvas — the live context on a FullPaint frame, the parent's
	// recorder on a retained frame. Nil Record paints nothing and is never
	// cacheable.
	Record func(dc render.Canvas, ox, oy float64)
}

// NewRenderPictureBox creates a picture box; record may be nil (paints
// nothing until SetRecord installs one).
func NewRenderPictureBox(w, h float64, record func(dc render.Canvas, ox, oy float64)) *RenderPictureBox {
	b := &RenderPictureBox{Width: w, Height: h, Record: record}
	b.Init(b)
	return b
}

// SetRecord installs a new record closure and dirties paint (the cache holds
// no fingerprint for this type, so any Record change must re-record).
func (b *RenderPictureBox) SetRecord(record func(dc render.Canvas, ox, oy float64)) {
	b.Record = record
	b.MarkNeedsPaint()
}

// Layout implements RenderObject.
func (b *RenderPictureBox) Layout(c Constraints) Size {
	if sz, ok := b.LayoutSkipIfClean(c); ok {
		return sz
	}
	out := c.Tighten(Size{Width: b.Width, Height: b.Height})
	b.setSize(out)
	b.RememberConstraints(c)
	b.clearLayoutDirty()
	return out
}

// Paint implements RenderObject.
//
// Clean + valid entry replays from cache (boundary_skip); dirty or missing
// entry builds via Record, replays the same commands live, and stores them.
// Single build — live paint and cache record can never diverge.
//
// On a recording pass (no live context) the closure draws into the parent's
// recorder directly: the layer's display list IS the content, so there is
// nothing to splice and no cache entry to keep (the retained path re-records
// this box only when it is dirty, and the boundary cache only pays off when a
// live frame can skip the closure).
func (b *RenderPictureBox) Paint(pc *PaintContext) {
	if pc == nil {
		return
	}
	if pc.BoundaryCache != nil && pc.BoundaryCache.tryReplay(pc, b) {
		return
	}
	pc.NotePaintVisit()
	sz := b.size
	w, h := sz.Width, sz.Height
	if w <= 0 {
		w = b.Width
	}
	if h <= 0 {
		h = b.Height
	}
	var built *recording.Recording
	if live, ok := pc.LiveContext(); ok && b.Record != nil {
		ox, oy := pc.OriginX, pc.OriginY
		built = recordBoxList(b, w, h, ox, oy)
		if built.PaintOpCount() > 0 {
			_ = built.Playback(recording.NewCanvasBackend(live))
		}
	} else if b.Record != nil {
		b.Record(pc.DC, pc.OriginX, pc.OriginY)
	}
	pc.NoteDebugRepaint(w, h)
	b.clearPaintDirty()
	if built != nil && pc.BoundaryCache != nil && b.IsRepaintBoundary() {
		pc.BoundaryCache.StoreRecording(pc, b, built)
	}
}

// recordBoxList seals the box's own display list at absolute origin (ox,oy),
// sized to its paint box so a surface-wide post-effect reports the box, not
// the window.
func recordBoxList(b *RenderPictureBox, w, h, ox, oy float64) *recording.Recording {
	rec := recording.NewRecorder(int(math.Ceil(w)), int(math.Ceil(h)))
	b.Record(rec, ox, oy)
	return rec.FinishRecording()
}

// HitTest implements RenderObject.
func (b *RenderPictureBox) HitTest(p Point) RenderObject {
	sz := b.size
	if p.X >= 0 && p.Y >= 0 && p.X < sz.Width && p.Y < sz.Height {
		return b
	}
	return nil
}
