package rendering

import "github.com/energye/gpui/ui/scene"

// BuildLayerTree walks the render tree and builds a retained scene.Layer tree.
// RepaintBoundary nodes become scene.BoundaryLayer units with dirty flags.
// The returned builder's Root is shared by pointer across frames (COW).
func BuildLayerTree(root RenderObject) *scene.LayerBuilder {
	b := scene.NewLayerBuilder()
	if root == nil {
		return b
	}
	appendNode(root, b)
	return b
}

func appendNode(n RenderObject, b *scene.LayerBuilder) {
	if n == nil || b == nil {
		return
	}
	off := n.Offset()

	// Transform nodes push scene.TransformLayer (P1).
	if tr, ok := n.(*RenderTransform); ok {
		rot, sx, sy := tr.TransformParams()
		if n.IsRepaintBoundary() {
			b.PushBoundary(off.X, off.Y, "transform", n.NeedsPaint() || SubtreeNeedsPaint(n))
			b.PushTransform(0, 0, rot, sx, sy)
			for _, ch := range n.Children() {
				appendNode(ch, b)
			}
			if len(n.Children()) == 0 {
				b.AddPicture(n.NeedsPaint())
			}
			b.Pop() // transform
			b.Pop() // boundary
			return
		}
		b.PushTransform(off.X, off.Y, rot, sx, sy)
		if len(n.Children()) == 0 {
			b.AddPicture(n.NeedsPaint())
		}
		for _, ch := range n.Children() {
			appendNode(ch, b)
		}
		b.Pop()
		return
	}

	if n.IsRepaintBoundary() {
		b.PushBoundary(off.X, off.Y, typeName(n), n.NeedsPaint() || SubtreeNeedsPaint(n))
		for _, ch := range n.Children() {
			appendNode(ch, b)
		}
		// Leaf content inside boundary: ensure a picture slot when no children.
		if len(n.Children()) == 0 {
			b.AddPicture(n.NeedsPaint())
		}
		b.Pop()
		return
	}
	b.PushOffset(off.X, off.Y)
	if len(n.Children()) == 0 {
		b.AddPicture(n.NeedsPaint())
	}
	for _, ch := range n.Children() {
		appendNode(ch, b)
	}
	b.Pop()
}

func typeName(n RenderObject) string {
	switch n.(type) {
	case *RenderColorBox:
		return "color"
	case *RenderBox:
		return "box"
	case *RenderTransform:
		return "transform"
	default:
		return "node"
	}
}

// BuildFramePacket layouts are assumed already flushed; builds a packet for raster.
// Overlay band is an empty F13 reserve; use overlay.State.AttachToPacket to fill it.
func BuildFramePacket(root RenderObject, frameID uint64, dpr, w, h float64) *scene.FramePacket {
	b := BuildLayerTree(root)
	return b.BuildPacket(frameID, dpr, w, h)
}

// BuildFramePacketOverlay builds the main packet then attaches ov to pkt.Overlay.
// ov may be nil (empty band). Main DirtyLayerIDs are preserved and overlay dirties appended.
func BuildFramePacketOverlay(root RenderObject, frameID uint64, dpr, w, h float64, attach func(pkt *scene.FramePacket)) *scene.FramePacket {
	pkt := BuildFramePacket(root, frameID, dpr, w, h)
	if attach != nil && pkt != nil {
		attach(pkt)
	}
	return pkt
}
