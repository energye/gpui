// Package gpupixel holds GPU-pixel contract tests for the retained composite
// path. Isolated in its own test package because importing render/gpu
// registers a global GPU accelerator that changes environment assumptions of
// ui/scene's own tests (several assume no accelerator).
package gpupixel

import (
	"fmt"
	"image"
	"os"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
	"github.com/energye/gpui/ui/scene"
)

// requireGPU skips cleanly when no wgpu backend is reachable.
func requireGPU(t *testing.T) {
	t.Helper()
	if render.Accelerator() == nil {
		t.Skip("GPU accelerator not registered")
	}
	dc := render.NewContext(8, 8)
	defer dc.Close()
	dc.SetRGB(1, 0, 0)
	dc.DrawRectangle(0, 0, 8, 8)
	_ = dc.Fill()
	if err := dc.FlushGPU(); err != nil {
		t.Skipf("GPU flush unavailable: %v", err)
	}
	if dc.RenderPathStats().GPUOps == 0 {
		t.Skipf("no GPU ops on probe: %s", dc.RenderPathStats().LogLine())
	}
}

// TestCompositeFramePacketTextured_ClipReplayPixels is the GPU pixel contract
// for the retained steady frame. Since the C8 pass-ownership rework, clipped
// subtrees are CACHEABLE again: the record runs in an isolated offscreen pass,
// so the steady frame is pure blit and pixels must be correct in both the
// record-era and steady frames (content present, geometry clipped).
func TestCompositeFramePacketTextured_ClipReplayPixels(t *testing.T) {
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
	b.PushOffset(10, 10) // outer container (Align-like)
	b.PushOffset(20, 10) // inner node offset — real windows nest these
	b.PushClipRRect(0, 0, 40, 40, 8)
	pl := b.AddPicture(true)
	pl.SetCacheKey(9001)
	pl.Record(func(r *scene.PictureRecorder) {
		r.FillRect(0, 0, 50, 50, 77/255., 204/255., 115/255., 1) // green
	})
	b.Pop() // clip
	b.Pop() // inner offset
	b.Pop() // outer offset
	pkt := b.BuildPacket(1, 1, W, H)

	tex := scene.NewPictureTextureCache(dc, 8)

	// Mirror the real window: several full present cycles before the first
	// retained composite (warm-up + steady frames).
	for i := 0; i < 5; i++ {
		dc.SetRGB(0.08, 0.09, 0.11)
		dc.DrawRectangle(0, 0, W, H)
		_ = dc.Fill()
		if err := dc.PresentFrame(view, W, H, func() error { return nil }); err != nil {
			t.Fatalf("pre present %d: %v", i, err)
		}
	}

	// Frame 1: dirty — the clip picture records into a bounds texture via an
	// isolated offscreen pass, then blits.
	st := scene.CompositeFramePacketTextured(pkt, dc, tex)
	if !tex.Has(9001) {
		t.Fatalf("clipped picture not cached (must record in isolated pass)")
	}
	if st.ReplayedOps != 0 {
		t.Fatalf("frame1 replayed %d ops want 0 (record+blit path)", st.ReplayedOps)
	}
	if err := dc.FlushGPUWithView(view, W, H); err != nil {
		t.Fatalf("frame1 flush: %v", err)
	}
	if os.Getenv("CLIP_DBG") == "1" {
		dbg1 := render.NewContext(W, H)
		defer dbg1.Close()
		dbg1.ClearWithColor(render.Black)
		dbg1.DrawGPUTexture(view, 0, 0, W, H)
		_ = dbg1.FlushGPU()
		img := dbg1.Image()
		for _, pt := range []image.Point{{50, 40}, {30, 30}, {15, 15}, {5, 90}} {
			r0, g0, b0, _ := img.At(pt.X, pt.Y).RGBA()
			fmt.Fprintf(os.Stderr, "CLIPDBG-F1 (%d,%d) rgb=%d,%d,%d\n", pt.X, pt.Y, r0>>8, g0>>8, b0>>8)
		}
	}

	// Frame 2: clean — PURE BLIT steady state, zero vector replay.
	st2 := scene.CompositeFramePacketTextured(pkt, dc, tex)
	if st2.ReplayedOps != 0 {
		t.Fatalf("steady frame replayed %d ops want 0 (pure blit)", st2.ReplayedOps)
	}
	if err := dc.FlushGPUWithView(view, W, H); err != nil {
		t.Fatalf("frame2 flush: %v", err)
	}
	if os.Getenv("CLIP_DBG") == "1" {
		dbg := render.NewContext(W, H)
		defer dbg.Close()
		dbg.ClearWithColor(render.Black)
		dbg.DrawGPUTexture(view, 0, 0, W, H)
		_ = dbg.FlushGPU()
		img := dbg.Image()
		for _, pt := range []image.Point{{50, 40}, {30, 30}, {15, 15}, {5, 90}} {
			r0, g0, b0, _ := img.At(pt.X, pt.Y).RGBA()
			fmt.Fprintf(os.Stderr, "CLIPDBG (%d,%d) rgb=%d,%d,%d\n", pt.X, pt.Y, r0>>8, g0>>8, b0>>8)
		}
	}

	// Composite the offscreen result into a CPU-readable context.
	out := render.NewContext(W, H)
	defer out.Close()
	out.ClearWithColor(render.Black)
	out.DrawGPUTexture(view, 0, 0, W, H)
	if err := out.FlushGPU(); err != nil {
		t.Fatalf("composite: %v", err)
	}

	sample := func(x, y int) (uint8, uint8, uint8) {
		img := out.Image()
		r, g, b, _ := img.At(x, y).RGBA()
		return uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)
	}
	rr, gg, bb := sample(50, 40) // clip center
	if gg < 140 || rr > 130 || bb > 160 {
		t.Fatalf("steady-frame blit lost clip content: (50,40) rgb=%d,%d,%d want green", rr, gg, bb)
	}
	r3, g3, b3 := sample(5, 90) // far outside
	if !(r3 < 60 && g3 < 60 && b3 < 60) {
		t.Fatalf("far outside (5,90) rgb=%d,%d,%d want black — blit leaked", r3, g3, b3)
	}
}

// TestCompositeFramePacketTextured_TranslateBlitPixels pins the cacheable
// side: a pure-translate TransformLayer (rotation=0, scale=1) keeps the
// bounds-texture fast path — record into cache, blit every steady frame,
// pixels land at the translated geometry.
func TestCompositeFramePacketTextured_TranslateBlitPixels(t *testing.T) {
	requireGPU(t)
	const W, H = 100, 100
	dc := render.NewContext(W, H)
	defer dc.Close()

	view, release := dc.CreateOffscreenTexture(W, H)
	if release == nil || view.IsNil() {
		t.Skip("CreateOffscreenTexture unavailable")
	}
	defer release()

	scene.ResetLayerIDGen()
	b := scene.NewLayerBuilder()
	tr := b.PushTransform(30, 30, 0, 1, 1)
	pl := b.AddPicture(true)
	pl.SetCacheKey(9002)
	pl.Record(func(r *scene.PictureRecorder) {
		r.FillRect(0, 0, 40, 40, 217/255., 77/255., 71/255., 1) // red
	})
	_ = tr
	b.Pop()
	pkt := b.BuildPacket(1, 1, W, H)

	tex := scene.NewPictureTextureCache(dc, 8)
	scene.CompositeFramePacketTextured(pkt, dc, tex)
	if !tex.Has(9002) {
		t.Fatalf("translate picture not cached (bounds-texture path lost)")
	}
	if err := dc.FlushGPUWithView(view, W, H); err != nil {
		t.Fatalf("frame flush: %v", err)
	}

	out := render.NewContext(W, H)
	defer out.Close()
	out.ClearWithColor(render.Black)
	out.DrawGPUTexture(view, 0, 0, W, H)
	if err := out.FlushGPU(); err != nil {
		t.Fatalf("composite: %v", err)
	}
	img := out.Image()
	r0, g0, b0, _ := img.At(50, 50).RGBA()
	R, G, B := uint8(r0>>8), uint8(g0>>8), uint8(b0>>8)
	if R < 150 || G > 110 || B > 110 {
		t.Fatalf("transform target center (50,50) rgb=%d,%d,%d want red", R, G, B)
	}
}
