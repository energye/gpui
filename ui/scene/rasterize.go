package scene

// RasterStats records per-frame dirty-layer raster work (F02).
type RasterStats struct {
	// RasterLayerCount is how many layers were re-rasterized this frame.
	RasterLayerCount int
	// SkippedLayerCount is picture/boundary layers reused without re-raster.
	SkippedLayerCount int
	// DirtyIDs is the dirty set considered.
	DirtyIDs []uint64
}

// RasterizeDirty walks the packet layer tree and counts re-raster vs skip.
// A PictureLayer or BoundaryLayer is re-rasterized iff its id is in DirtyLayerIDs
// (or NeedsRaster for pictures). Clean layers are "static reuse".
func RasterizeDirty(pkt *FramePacket) RasterStats {
	st := RasterStats{}
	if pkt == nil {
		return st
	}
	st.DirtyIDs = append([]uint64(nil), pkt.DirtyLayerIDs...)
	dirty := map[uint64]struct{}{}
	for _, id := range pkt.DirtyLayerIDs {
		dirty[id] = struct{}{}
	}

	var walk func(Layer)
	walk = func(l Layer) {
		if l == nil {
			return
		}
		id := l.LayerID()
		switch t := l.(type) {
		case *PictureLayer:
			if _, ok := dirty[id]; ok || t.NeedsRaster {
				st.RasterLayerCount++
				t.NeedsRaster = false
				t.Picture.Valid = true
			} else {
				st.SkippedLayerCount++
			}
		case *BoundaryLayer:
			if _, ok := dirty[id]; ok {
				st.RasterLayerCount++
			} else {
				// Boundary with clean id: still walk children for nested pictures.
				st.SkippedLayerCount++
			}
		}
		for _, ch := range l.Children() {
			walk(ch)
		}
	}
	walk(pkt.Root)
	if pkt.Overlay != nil {
		walk(pkt.Overlay)
	}
	return st
}
