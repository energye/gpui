package scene_test

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	"github.com/energye/gpui/render/recording"
	"github.com/energye/gpui/ui/scene"
)

// TestCompositeFallback_ReplaysRecordingWithoutGPU locks the retained-path
// contract for OnPaint-only layers (input-box borders): their recording lives
// on the layer from birth (built on the UI thread), and without GPU the
// texture cache degrades to no-op — the composite must still vector-replay
// the recording instead of leaving a transparent hole.
func TestCompositeFallback_ReplaysRecordingWithoutGPU(t *testing.T) {
	scene.ResetLayerIDGen()
	b := scene.NewLayerBuilder()
	b.PushOffset(10, 10)
	pl := b.AddPicture(true)
	pl.SetCacheKey(303)
	// Bounds-sized recording (same as input-box borders): content in
	// layer-local coords, positioned by the ancestor offset at composite.
	rec := recording.NewRecorder(20, 10)
	rec.SetRGBA(0, 0, 1, 1)
	rec.DrawRectangle(0, 0, 20, 10)
	_ = rec.Fill()
	pl.SetRecording(rec.FinishRecording())
	if got := pl.Recording.Bounds(); got != image.Rect(0, 0, 20, 10) {
		t.Fatalf("recording bounds=%v want (0,0,20,10)", got)
	}
	b.Pop() // offset
	pkt := b.BuildPacket(1, 1, 100, 80)

	dc := render.NewContext(100, 80)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	tex := scene.NewPictureTextureCache(dc, 0) // no GPU → replay fallback

	scene.CompositeFramePacketTextured(pkt, dc, tex)
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	// Content must include the recording: blue at (10+10,10+5).
	img := dc.Image()
	rr, gg, bb := sampleAt(img, 20, 15)
	if bb < 0xC000 || rr > 0x4000 || gg > 0x4000 {
		t.Fatalf("recording pixel (20,15)=#%04x%04x%04x want blue", rr, gg, bb)
	}
}
