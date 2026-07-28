package scene

import "github.com/energye/gpui/render"

// RasterStats records per-frame dirty-layer raster work (F02).
type RasterStats struct {
	// RasterLayerCount is how many layers were re-rasterized this frame.
	RasterLayerCount int
	// SkippedLayerCount is picture/boundary layers reused without re-raster.
	SkippedLayerCount int
	// DirtyIDs is the dirty set considered.
	DirtyIDs []uint64
	// ReplayedOps is the total display-list ops applied when a Context is provided.
	ReplayedOps int
}

// RasterizeDirty walks the packet layer tree and counts re-raster vs skip.
// A PictureLayer or BoundaryLayer is re-rasterized iff its id is in DirtyLayerIDs
// (or NeedsRaster for pictures). Clean layers are "static reuse".
//
// Display-list ops on Picture are retained; Valid is set true after a dirty pass
// so subsequent clean frames can skip. This is not dirty-rect Present.
func RasterizeDirty(pkt *FramePacket) RasterStats {
	return RasterizeDirtyToContext(pkt, nil)
}

// RasterizeDirtyToContext is RasterizeDirty plus optional Replay of dirty
// PictureLayers that hold a non-empty display list onto dc. Clean pictures skip
// replay (static reuse of prior surface content is the caller's responsibility
// when dc is used only for dirty layers). dc may be nil for stats-only.
func RasterizeDirtyToContext(pkt *FramePacket, dc *render.Context) RasterStats {
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
				if dc != nil && len(t.Picture.Ops) > 0 {
					n := t.Picture.OpCount()
					t.Picture.Replay(dc)
					st.ReplayedOps += n
				}
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
