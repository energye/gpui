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
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/recording"
)

// RasterStats records per-frame dirty-layer raster work (F02).
type RasterStats struct {
	// RasterLayerCount is how many layers were re-rasterized this frame.
	RasterLayerCount int
	// SkippedLayerCount is picture/boundary layers reused without re-raster.
	SkippedLayerCount int
	// DirtyIDs is the dirty set considered.
	DirtyIDs []uint64
	// ReplayedOps is the total display-list draw-command count of dirty
	// layers when a Context is provided.
	ReplayedOps int
}

// RasterizeDirty walks the packet layer tree and counts re-raster vs skip.
// A PictureLayer or BoundaryLayer is re-rasterized iff its id is in DirtyLayerIDs
// (or NeedsRaster for pictures). Clean layers are "static reuse".
//
// Recordings are immutable once built on the UI thread; replay happens only
// for layers in the dirty set. This is not dirty-rect Present.
func RasterizeDirty(pkt *FramePacket) RasterStats {
	return RasterizeDirtyToContext(pkt, nil)
}

// RasterizeDirtyToContext is RasterizeDirty plus optional replay of dirty
// PictureLayers that hold a non-empty recording onto dc. Clean pictures skip
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
				if dc != nil && t.Recording.PaintOpCount() > 0 {
					st.ReplayedOps += t.Recording.PaintOpCount()
					// 无可见内容的 recording 一笔都落不下去（HasVisiblePaint 与
					// 重放跳过条件同构），跳过回放，计数照记。
					if t.Recording.HasVisiblePaint() {
						_ = t.Recording.Playback(recording.NewCanvasBackend(dc))
					}
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

// CountPictureOps sums display-list draw-command counts of every PictureLayer
// in the packet. Replay-free: walks the tree only.
func CountPictureOps(pkt *FramePacket) int {
	if pkt == nil {
		return 0
	}
	n := 0
	Walk(pkt.Root, func(l Layer) {
		if pl, ok := l.(*PictureLayer); ok {
			n += pl.Recording.PaintOpCount()
		}
	})
	if pkt.Overlay != nil {
		Walk(pkt.Overlay, func(l Layer) {
			if pl, ok := l.(*PictureLayer); ok {
				n += pl.Recording.PaintOpCount()
			}
		})
	}
	return n
}
