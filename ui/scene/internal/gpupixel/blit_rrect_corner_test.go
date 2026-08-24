package gpupixel

import (
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
	"github.com/energye/gpui/ui/scene"
)

// TestCompositeFramePacketTextured_BlitzRRectCornerPixels pins the rounded
// corners of a cached clipped picture in the steady blit frame. The C8
// pass-ownership rework made clip subtrees cacheable again; the bounds
// texture stores the UNclipped rect and the composite-time RRect clip must
// round the corners when blitting. Regression: steady frames showed sharp
// corners (green in the corner holes) while vector-replay frames (resize)
// were correct — C8 target-round, 2026-08-24.
func TestCompositeFramePacketTextured_BlitzRRectCornerPixels(t *testing.T) {
	requireGPU(t)
	const W, H = 120, 100
	dc := render.NewContext(W, H)
	defer dc.Close()

	view, release := dc.CreateOffscreenTexture(W, H)
	if release == nil || view.IsNil() {
		t.Skip("CreateOffscreenTexture unavailable")
	}
	defer release()

	scene.ResetLayerIDGen()
	b := scene.NewLayerBuilder()
	b.PushOffset(10, 10)
	b.PushOffset(20, 10) // clip lands at (30,30)-(70,70), radius 8
	b.PushClipRRect(0, 0, 40, 40, 8)
	pl := b.AddPicture(true)
	pl.SetCacheKey(9101)
	pl.Record(func(r *scene.PictureRecorder) {
		r.FillRect(0, 0, 50, 50, 77/255., 204/255., 115/255., 1) // green overfill
	})
	b.Pop() // clip
	b.Pop() // inner offset
	b.Pop() // outer offset
	pkt := b.BuildPacket(1, 1, W, H)

	tex := scene.NewPictureTextureCache(dc, 8)
	for i := 0; i < 3; i++ {
		dc.SetRGB(0.08, 0.09, 0.11)
		dc.DrawRectangle(0, 0, W, H)
		_ = dc.Fill()
		if err := dc.PresentFrame(view, W, H, func() error { return nil }); err != nil {
			t.Fatalf("pre present %d: %v", i, err)
		}
	}

	// Frame 1 records via the isolated offscreen pass; frame 2 is pure blit.
	if st := scene.CompositeFramePacketTextured(pkt, dc, tex); !tex.Has(9101) {
		t.Fatalf("clipped picture not cached")
	} else if st.ReplayedOps != 0 {
		t.Fatalf("frame1 replayed %d ops want 0", st.ReplayedOps)
	}
	if st2 := scene.CompositeFramePacketTextured(pkt, dc, tex); st2.ReplayedOps != 0 {
		t.Fatalf("steady frame replayed %d ops want 0 (pure blit)", st2.ReplayedOps)
	}
	if err := dc.FlushGPUWithView(view, W, H); err != nil {
		t.Fatalf("flush: %v", err)
	}

	out := render.NewContext(W, H)
	defer out.Close()
	out.ClearWithColor(render.Black)
	out.DrawGPUTexture(view, 0, 0, W, H)
	if err := out.FlushGPU(); err != nil {
		t.Fatalf("composite: %v", err)
	}
	sample := func(x, y int) (uint8, uint8, uint8) {
		img := out.Image()
		r, g, bb, _ := img.At(x, y).RGBA()
		return uint8(r >> 8), uint8(g >> 8), uint8(bb >> 8)
	}
	isGreen := func(r, g, b uint8) bool { return g > 140 && r < 130 }
	isDark := func(r, g, b uint8) bool { return r < 60 && g < 60 && b < 60 }

	// Center and straight-edge interior must be green (content present).
	for _, pt := range [][2]int{{50, 40}, {50, 32}, {34, 40}} {
		r, g, b := sample(pt[0], pt[1])
		if !isGreen(r, g, b) {
			t.Fatalf("interior (%d,%d) rgb=%d,%d,%d want green — content lost", pt[0], pt[1], r, g, b)
		}
	}
	// Corner holes (outside the r=8 quarter circles). Clip rect is
	// (30,20)-(70,60); e.g. TL corner center is (38,28) and (31,22) sits at
	// squared distance 85 > 64 — green there means the blit ignored the clip.
	corners := [][2]int{{31, 22}, {69, 22}, {31, 58}, {69, 58}}
	for _, pt := range corners {
		r, g, b := sample(pt[0], pt[1])
		if !isDark(r, g, b) {
			t.Fatalf("rounded-corner hole (%d,%d) rgb=%d,%d,%d want dark background — blit lost the rounded clip", pt[0], pt[1], r, g, b)
		}
	}
}
