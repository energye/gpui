package rendering_test

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/recording"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scene"
)

// The retained path records a layer in its own local space and lets the
// compositor place it with a layer offset, while the live path draws the same
// node straight into the window with the node's origin baked into every
// coordinate. Those two agree only if the display list's relative commands
// compose onto the playback CTM exactly as they composed onto the paint CTM;
// the retired picture carrier baked absolute values and could not express the
// difference at all. These tests lock the property.

// parityPaint draws in local coordinates through a translate/rotate/scale chain
// and a clip, using the origin-aware helpers — the convention every widget
// follows, and the one that makes a recording position-independent.
func parityPaint(pc *rendering.PaintContext, _ rendering.Size) {
	rendering.FillRect(pc, 0, 0, 80, 80, 0.9, 0.2, 0.2, 1)

	pc.PushTransform(rendering.TranslateMatrix(8, 6))
	pc.PushTransform(rendering.RotateMatrix(0.35))
	rendering.FillRect(pc, 0, 0, 24, 12, 0.1, 0.1, 0.1, 1)
	pc.PopTransform()
	pc.PopTransform()

	pc.Save()
	pc.PushClipRect(10, 10, 30, 20)
	pc.PushTransform(rendering.TranslateMatrix(30, 20))
	pc.PushTransform(rendering.ScaleMatrix(1.5, 0.75))
	rendering.StrokeRect(pc, 0, 0, 20, 20, 3, 0.1, 0.6, 0.9, 1)
	pc.PopTransform()
	pc.PopTransform()
	pc.PopClip()
	pc.RestoreCanvas()

	p := render.NewPath()
	p.MoveTo(4, 40)
	p.LineTo(44, 44)
	p.LineTo(20, 68)
	p.Close()
	rendering.FillPath(pc, p, 0.2, 0.8, 0.3, 1)
}

// TestParity_LiveVsRecordedReplay_TransformChain paints one tree twice — once
// live into a context, once through the retained layer tree and playback — and
// requires the two frames to agree on every pixel.
func TestParity_LiveVsRecordedReplay_TransformChain(t *testing.T) {
	const w, h = 120, 120

	child := rendering.NewRenderBox()
	child.FixedWidth, child.FixedHeight = 80, 80
	child.OnPaint = parityPaint
	root := rendering.NewAbsoluteBox(w, h)
	root.Place(child, 18, 24)
	root.Layout(rendering.Tight(w, h))

	live := render.NewContext(w, h)
	defer live.Close()
	live.BeginFrame()
	live.ClearWithColor(render.White)
	root.Paint(rendering.NewPaintContext(live, 1))

	scene.ResetLayerIDGen()
	pkt := rendering.BuildFramePacket(root, 1, 1, w, h)
	replay := render.NewContext(w, h)
	defer replay.Close()
	replay.BeginFrame()
	replay.ClearWithColor(render.White)
	st := scene.CompositeFramePacket(pkt, replay)
	if st.PicturesDrawn == 0 || st.OpsReplayed == 0 {
		t.Fatalf("retained path replayed nothing: pictures=%d ops=%d", st.PicturesDrawn, st.OpsReplayed)
	}
	assertImagesEqual(t, live.Image(), replay.Image())
}

// TestParity_ReplayInsideOnPaint_FoldsIntoLayerRecording covers the widget that
// keeps its own display list and replays it from OnPaint. The build pass hands
// OnPaint a recorder, so the replay has to fold those commands into the layer's
// recording; a playback surface that only accepted a live context would drop the
// content from every retained window.
func TestParity_ReplayInsideOnPaint_FoldsIntoLayerRecording(t *testing.T) {
	const w, h = 100, 100

	prepared := recording.NewRecorder(40, 40)
	prepared.SetRGBA(0.85, 0.15, 0.85, 1)
	prepared.FillRectangle(0, 0, 40, 40)
	list := prepared.FinishRecording()

	box := rendering.NewRenderBox()
	box.FixedWidth, box.FixedHeight = 40, 40
	box.OnPaint = func(pc *rendering.PaintContext, _ rendering.Size) {
		// Local-space display list: the live path has to place it itself, the
		// recording path gets the placement from the layer offset.
		pc.DC.Translate(pc.OriginX, pc.OriginY)
		if err := list.Playback(recording.NewCanvasBackend(pc.DC)); err != nil {
			t.Errorf("replay onto %T: %v", pc.DC, err)
		}
	}
	root := rendering.NewAbsoluteBox(w, h)
	root.Place(box, 10, 10)
	root.Layout(rendering.Tight(w, h))

	live := render.NewContext(w, h)
	defer live.Close()
	live.BeginFrame()
	live.ClearWithColor(render.White)
	root.Paint(rendering.NewPaintContext(live, 1))

	scene.ResetLayerIDGen()
	pkt := rendering.BuildFramePacket(root, 1, 1, w, h)
	var recorded *recording.Recording
	scene.Walk(pkt.Root, func(l scene.Layer) {
		pl, ok := l.(*scene.PictureLayer)
		if !ok || pl.Recording.PaintOpCount() == 0 {
			return
		}
		if recorded != nil {
			t.Fatalf("two content layers in a one-box tree: cannot attribute the replay")
		}
		recorded = pl.Recording
	})
	if recorded == nil {
		t.Fatal("layer tree holds no replayed content: OnPaint's playback never reached the recording")
	}
	if !recorded.HasVisiblePaint() {
		t.Fatal("folded recording reports no visible paint")
	}

	replay := render.NewContext(w, h)
	defer replay.Close()
	replay.BeginFrame()
	replay.ClearWithColor(render.White)
	scene.CompositeFramePacket(pkt, replay)
	assertImagesEqual(t, live.Image(), replay.Image())
	r, g, b := sampleAt(replay.Image(), 30, 30)
	if r < 0xA000 || b < 0xA000 || g > 0x6000 {
		t.Fatalf("(30,30)=#%04x%04x%04x want the replayed magenta fill", r, g, b)
	}
}

// assertImagesEqual fails on the first differing pixel, reporting its position.
func assertImagesEqual(t *testing.T, want, got image.Image) {
	t.Helper()
	if want.Bounds() != got.Bounds() {
		t.Fatalf("size mismatch: live=%v replay=%v", want.Bounds(), got.Bounds())
	}
	wb := want.Bounds()
	firstDiff := -1
	diffs := 0
	for y := wb.Min.Y; y < wb.Max.Y; y++ {
		for x := wb.Min.X; x < wb.Max.X; x++ {
			r1, g1, b1, a1 := want.At(x, y).RGBA()
			r2, g2, b2, a2 := got.At(x, y).RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				if diffs == 0 {
					firstDiff = (y-wb.Min.Y)*wb.Dx() + (x - wb.Min.X)
					t.Errorf("first differing pixel (%d,%d): live=#%04x%04x%04x%04x replay=#%04x%04x%04x%04x",
						x, y, r1, g1, b1, a1, r2, g2, b2, a2)
				}
				diffs++
			}
		}
	}
	if diffs > 0 {
		t.Fatalf("live and retained frames differ in %d pixels (first at index %d)", diffs, firstDiff)
	}
}
