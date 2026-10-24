//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package scene

import (
	"image"
	"math"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/recording"
)

// CompositeStats counts layers visited / pictures drawn during CompositeToContext.
type CompositeStats struct {
	LayersVisited     int
	PicturesDrawn     int
	OpsReplayed       int
	ClipsApplied      int
	TransformsApplied int
	FiltersApplied    int
	// TexturesVisited counts texture layers walked; TexturesPainted counts the
	// ones whose source drew pixels, TexturesMissed the ones that resolved to
	// nothing (closed producer / no frame). Visited == Painted+Missed.
	TexturesVisited int
	TexturesPainted int
	TexturesMissed  int
}

// CompositeToContext walks the retained layer tree in paint order and applies
// each node onto dc (Flutter-like layer composite subset on a single canvas):
//
//	Offset / Boundary → Translate
//	ClipRect / ClipRRect → clip stack
//	Transform → TX/TY → Rotate → Scale (matches TransformLayer docs)
//	Picture → recording playback onto dc
//	Texture → the registered source paints its quad under dc's CTM
//	Opacity → PushLayer(Normal, opacity) isolation
//	ColorFilter / ImageFilter → isolate children, Apply*, composite back
//
// This does not claim dirty-rect Present, BackdropFilter product,
// or full Flutter multi-RT compositor. dc must be non-nil.
func CompositeToContext(root Layer, dc *render.Context) CompositeStats {
	st := CompositeStats{}
	if root == nil || dc == nil {
		return st
	}
	compositeLayer(root, dc, &st)
	return st
}

// CompositeFramePacket composites pkt.Root then pkt.Overlay (if any).
func CompositeFramePacket(pkt *FramePacket, dc *render.Context) CompositeStats {
	st := CompositeStats{}
	if pkt == nil || dc == nil {
		return st
	}
	if pkt.Root != nil {
		s := CompositeToContext(pkt.Root, dc)
		st = addStats(st, s)
	}
	if pkt.Overlay != nil {
		s := CompositeToContext(pkt.Overlay, dc)
		st = addStats(st, s)
	}
	return st
}

func addStats(a, b CompositeStats) CompositeStats {
	return CompositeStats{
		LayersVisited:     a.LayersVisited + b.LayersVisited,
		PicturesDrawn:     a.PicturesDrawn + b.PicturesDrawn,
		OpsReplayed:       a.OpsReplayed + b.OpsReplayed,
		ClipsApplied:      a.ClipsApplied + b.ClipsApplied,
		TransformsApplied: a.TransformsApplied + b.TransformsApplied,
		FiltersApplied:    a.FiltersApplied + b.FiltersApplied,
		TexturesVisited:   a.TexturesVisited + b.TexturesVisited,
		TexturesPainted:   a.TexturesPainted + b.TexturesPainted,
		TexturesMissed:    a.TexturesMissed + b.TexturesMissed,
	}
}

func compositeLayer(l Layer, dc *render.Context, st *CompositeStats) {
	if l == nil || dc == nil {
		return
	}
	st.LayersVisited++

	switch t := l.(type) {
	case *PictureLayer:
		if t.Recording.PaintOpCount() > 0 {
			st.PicturesDrawn++
			st.OpsReplayed += t.Recording.PaintOpCount()
			_ = t.Recording.Playback(recording.NewCanvasBackend(dc))
			t.NeedsRaster = false
		}
		return // leaf

	case *TextureLayer:
		// Texture content never raster-caches: the producer can rewrite the
		// texture without the UI tree changing, so the layer repaints every
		// frame it is in.
		st.TexturesVisited++
		if _, drawn := paintTextureLayer(t, dc); drawn {
			st.TexturesPainted++
		} else {
			st.TexturesMissed++
		}
		return // leaf

	case *OffsetLayer:
		dc.Push()
		dc.Translate(t.DX, t.DY)
		for _, ch := range t.Children() {
			compositeLayer(ch, dc, st)
		}
		dc.Pop()
		return

	case *BoundaryLayer:
		dc.Push()
		dc.Translate(t.DX, t.DY)
		for _, ch := range t.Children() {
			compositeLayer(ch, dc, st)
		}
		dc.Pop()
		return

	case *ClipRectLayer:
		if t.W > 0 && t.H > 0 {
			dc.Push()
			dc.ClipRect(t.X, t.Y, t.W, t.H)
			st.ClipsApplied++
			for _, ch := range t.Children() {
				compositeLayer(ch, dc, st)
			}
			dc.Pop()
			return
		}

	case *ClipRRectLayer:
		if t.W > 0 && t.H > 0 {
			dc.Push()
			if t.Radius > 0 {
				dc.ClipRoundRect(t.X, t.Y, t.W, t.H, t.Radius)
			} else {
				dc.ClipRect(t.X, t.Y, t.W, t.H)
			}
			st.ClipsApplied++
			for _, ch := range t.Children() {
				compositeLayer(ch, dc, st)
			}
			dc.Pop()
			return
		}

	case *TransformLayer:
		sx, sy := t.EffectiveScale()
		dc.Push()
		if t.TX != 0 || t.TY != 0 {
			dc.Translate(t.TX, t.TY)
		}
		// Pivot around the subtree center — see textured.go pushCompositeCTM.
		if t.Rotation != 0 || sx != 1 || sy != 1 {
			dc.Translate(t.CX, t.CY)
			if t.Rotation != 0 {
				dc.Rotate(t.Rotation)
			}
			if sx != 1 || sy != 1 {
				dc.Scale(sx, sy)
			}
			dc.Translate(-t.CX, -t.CY)
		}
		st.TransformsApplied++
		for _, ch := range t.Children() {
			compositeLayer(ch, dc, st)
		}
		dc.Pop()
		return

	case *OpacityLayer:
		op := t.Opacity
		if op < 0 {
			op = 0
		}
		if op > 1 {
			op = 1
		}
		if op >= 1-1e-9 {
			for _, ch := range t.Children() {
				compositeLayer(ch, dc, st)
			}
			return
		}
		if op <= 1e-9 {
			return // fully transparent
		}
		// Cheap opacity-group for non-overlapping UI; isolation via PushLayerIsolated
		// is available for filter layers below.
		dc.PushLayer(render.BlendNormal, op)
		for _, ch := range t.Children() {
			compositeLayer(ch, dc, st)
		}
		dc.PopLayer()
		return

	case *ColorFilterLayer:
		// True offscreen isolation so ApplyColorMatrix hits only the subtree.
		dc.PushLayerIsolated(1)
		for _, ch := range t.Children() {
			compositeLayer(ch, dc, st)
		}
		dc.ApplyColorMatrix(t.Matrix)
		dc.PopLayer()
		st.FiltersApplied++
		return

	case *ImageFilterLayer:
		dc.PushLayerIsolated(1)
		for _, ch := range t.Children() {
			compositeLayer(ch, dc, st)
		}
		if t.BlurRadius > 0 {
			dc.ApplyBlur(t.BlurRadius)
		}
		dc.PopLayer()
		st.FiltersApplied++
		return

	case *BackdropFilterLayer:
		op := t.Opacity
		if op <= 0 {
			op = 1
		}
		if op > 1 {
			op = 1
		}
		// Snapshot parent → optional blur → children on top → composite.
		dc.PushBackdropLayer(render.BlendNormal, op)
		if t.BlurRadius > 0 {
			dc.ApplyBlur(t.BlurRadius)
		}
		for _, ch := range t.Children() {
			compositeLayer(ch, dc, st)
		}
		dc.PopLayer()
		st.FiltersApplied++
		return

	case *ContainerLayer:
		// bare container
		for _, ch := range t.Children() {
			compositeLayer(ch, dc, st)
		}
		return
	}

	// Default: walk children (unknown / interface-only layers).
	for _, ch := range l.Children() {
		compositeLayer(ch, dc, st)
	}
}

// paintTextureLayer resolves a texture layer's source and draws its quad under
// dc's current transform. drawn=false means the rect was NOT covered (source
// unregistered, no frame yet, or no device), which the caller must report as a
// miss instead of damaging over stale pixels. Group opacity from an enclosing
// OpacityLayer is inherited by the source's own draw.
func paintTextureLayer(t *TextureLayer, dc *render.Context) (direct, drawn bool) {
	if t == nil || dc == nil || t.TextureID == 0 {
		return false, false
	}
	src, ok := render.GetTextureSource(t.TextureID)
	if !ok {
		return false, false
	}
	direct, drawn = src.PaintTexture(dc, render.TexturePaintOptions{
		X: t.X, Y: t.Y, Width: t.W, Height: t.H, DrawIndex: t.DrawIndex,
	})
	if drawn {
		t.painted = true
	}
	return direct, drawn
}

// textureLayerDamage maps a texture layer's quad into logical surface coords
// under the current transform (TrackDamageRect takes logical, the CTM is
// device). Ceil on the far edge so a fractional quad never damages short.
func textureLayerDamage(dc *render.Context, t *TextureLayer) image.Rectangle {
	if t == nil || t.W <= 0 || t.H <= 0 {
		return image.Rectangle{}
	}
	rect := image.Rect(int(math.Floor(t.X)), int(math.Floor(t.Y)),
		int(math.Ceil(t.X+t.W)), int(math.Ceil(t.Y+t.H)))
	return transformBounds(dc, rect)
}
