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
	default:
		return "node"
	}
}

// BuildFramePacket layouts are assumed already flushed; builds a packet for raster.
func BuildFramePacket(root RenderObject, frameID uint64, dpr, w, h float64) *scene.FramePacket {
	b := BuildLayerTree(root)
	return b.BuildPacket(frameID, dpr, w, h)
}
