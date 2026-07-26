package scene

// LayerBuilder constructs a retained layer tree from paint-time callbacks.
// P2: used by rendering.BuildLayerTree.
type LayerBuilder struct {
	stack []*ContainerLayer
	root  *ContainerLayer

	// DirtyBoundaryIDs collected when a boundary reports needs paint.
	DirtyBoundaryIDs []uint64
}

// NewLayerBuilder starts with a root container.
func NewLayerBuilder() *LayerBuilder {
	root := NewContainerLayer()
	return &LayerBuilder{
		root:  root,
		stack: []*ContainerLayer{root},
	}
}

// Root returns the built main-band root.
func (b *LayerBuilder) Root() Layer {
	if b == nil {
		return nil
	}
	return b.root
}

func (b *LayerBuilder) current() *ContainerLayer {
	if b == nil || len(b.stack) == 0 {
		return nil
	}
	return b.stack[len(b.stack)-1]
}

// PushOffset pushes an offset layer.
func (b *LayerBuilder) PushOffset(dx, dy float64) *OffsetLayer {
	o := NewOffsetLayer(dx, dy)
	b.current().Add(o)
	b.stack = append(b.stack, &o.ContainerLayer)
	return o
}

// PushBoundary pushes a repaint boundary layer.
func (b *LayerBuilder) PushBoundary(dx, dy float64, source string, paintDirty bool) *BoundaryLayer {
	bl := NewBoundaryLayer(dx, dy, source)
	b.current().Add(bl)
	b.stack = append(b.stack, &bl.ContainerLayer)
	if paintDirty {
		b.DirtyBoundaryIDs = append(b.DirtyBoundaryIDs, bl.LayerID())
	}
	return bl
}

// PushOpacity pushes an opacity layer.
func (b *LayerBuilder) PushOpacity(opacity float64) *OpacityLayer {
	o := NewOpacityLayer(opacity)
	b.current().Add(o)
	b.stack = append(b.stack, &o.ContainerLayer)
	return o
}

// AddPicture appends a picture layer to the current container.
func (b *LayerBuilder) AddPicture(needsRaster bool) *PictureLayer {
	p := NewPictureLayer()
	p.NeedsRaster = needsRaster
	p.Picture.Valid = !needsRaster
	b.current().Add(p)
	return p
}

// Pop removes the current layer from the stack (not the root).
func (b *LayerBuilder) Pop() {
	if b == nil || len(b.stack) <= 1 {
		return
	}
	b.stack = b.stack[:len(b.stack)-1]
}

// BuildPacket creates a FramePacket sharing the built root (no deep copy).
func (b *LayerBuilder) BuildPacket(frameID uint64, dpr, w, h float64) *FramePacket {
	if b == nil {
		return nil
	}
	pkt := &FramePacket{
		FrameID:       frameID,
		DPR:           dpr,
		Width:         w,
		Height:        h,
		Root:          b.root,
		Overlay:       EnsureOverlayBand(nil),
		DirtyLayerIDs: append([]uint64(nil), b.DirtyBoundaryIDs...),
		Generation:    frameID,
	}
	// Picture layers that need raster also dirty.
	Walk(b.root, func(l Layer) {
		if pl, ok := l.(*PictureLayer); ok && pl.NeedsRaster {
			pkt.DirtyLayerIDs = append(pkt.DirtyLayerIDs, pl.LayerID())
		}
	})
	return pkt
}
