package embedder

import (
	"github.com/energye/gpui/ui/raster"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scene"
)

// SubmitLayerPacket builds a FramePacket from the render root and enqueues a
// raster job. P2: the job may only hold the packet; full dirty-layer raster is P3.
// execute, if non-nil, runs on the raster thread with the packet.
func (a *App) SubmitLayerPacket(root rendering.RenderObject, frameID uint64, execute func(*scene.FramePacket) error) bool {
	if a == nil || a.loop == nil || root == nil {
		return false
	}
	w, h := 0, 0
	scale := 1.0
	if a.host != nil {
		w, h = a.host.Size()
		scale = a.host.ScaleFactor()
		if scale <= 0 {
			scale = 1
		}
	}
	pkt := rendering.BuildFramePacket(root, frameID, scale, float64(w), float64(h))
	job := raster.FrameJob{
		Run: func() error {
			if execute != nil {
				return execute(pkt)
			}
			return nil
		},
	}
	if a.loop.TrySubmit(job) {
		return true
	}
	// Backpressure: drop unstarted work by not blocking UI (Flutter-like prefer latest).
	// Caller may ScheduleFrame again.
	return false
}
