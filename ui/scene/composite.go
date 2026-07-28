package scene

import "github.com/energye/gpui/render"

// CompositeStats counts layers visited / pictures drawn during CompositeToContext.
type CompositeStats struct {
	LayersVisited     int
	PicturesDrawn     int
	OpsReplayed       int
	ClipsApplied      int
	TransformsApplied int
	FiltersApplied    int
}

// CompositeToContext walks the retained layer tree in paint order and applies
// each node onto dc (Flutter-like layer composite subset on a single canvas):
//
//	Offset / Boundary → Translate
//	ClipRect / ClipRRect → clip stack
//	Transform → TX/TY → Rotate → Scale (matches TransformLayer docs)
//	Picture → Picture.Replay
//	Opacity → PushLayer(Normal, opacity) isolation
//	ColorFilter / ImageFilter → isolate children, Apply*, composite back
//
// This does not claim dirty-rect Present, TextureLayer, BackdropFilter product,
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
	}
}

func compositeLayer(l Layer, dc *render.Context, st *CompositeStats) {
	if l == nil || dc == nil {
		return
	}
	st.LayersVisited++

	switch t := l.(type) {
	case *PictureLayer:
		if len(t.Picture.Ops) > 0 {
			n := t.Picture.OpCount()
			t.Picture.Replay(dc)
			st.PicturesDrawn++
			st.OpsReplayed += n
			t.NeedsRaster = false
			t.Picture.Valid = true
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
		if t.Rotation != 0 {
			dc.Rotate(t.Rotation)
		}
		if sx != 1 || sy != 1 {
			dc.Scale(sx, sy)
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
