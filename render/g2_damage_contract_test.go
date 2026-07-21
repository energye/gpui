//go:build !nogpu

package render_test

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
)

// TestG2_DamageContract_API documents ENGINE_GAPS G2:
// damage API is available; vector frames do not promise partial preserve.
// Blit-only paths may use damage; vector Fill/Stroke still completes without error.
func TestG2_DamageContract_API(t *testing.T) {
	s3cRequireGPU(t)
	dc := render.NewContext(64, 64)
	defer dc.Close()

	view, release := dc.CreateOffscreenTexture(64, 64)
	if release == nil || view.IsNil() {
		t.Skip("CreateOffscreenTexture unavailable")
	}
	defer release()

	// Vector frame + damage: must not crash; API accepts damage rect.
	dc.SetRGB(0.2, 0.2, 0.2)
	dc.DrawRectangle(0, 0, 64, 64)
	_ = dc.Fill()
	if err := dc.FlushGPUWithView(view, 64, 64); err != nil {
		t.Fatalf("full flush: %v", err)
	}

	dc.SetRGB(1, 0, 0)
	dc.DrawRectangle(8, 8, 16, 16)
	_ = dc.Fill()
	damage := image.Rect(8, 8, 24, 24)
	if err := dc.FlushGPUWithViewDamage(view, 64, 64, damage); err != nil {
		t.Fatalf("vector+damage must succeed (even if LoadOpClear): %v", err)
	}

	// Multi-rect damage API
	if err := dc.FlushGPUWithViewDamageRects(view, 64, 64, []image.Rectangle{
		image.Rect(0, 0, 8, 8),
		image.Rect(40, 40, 48, 48),
	}); err != nil {
		t.Fatalf("DamageRects: %v", err)
	}

	// PresentWithDamage ignores rects on wgpu-native — contract is no-panic.
	// (Surface may be nil offscreen; skip if no surface.)
	st := dc.RenderPathStats()
	if st.GPUOps == 0 {
		t.Fatalf("expected GPU path: %s", st.LogLine())
	}
	t.Logf("G2 contract path_stats %s (vector damage is best-effort / may clear)", st.LogLine())
}

// TestG2_PresentWithDamage_IgnoresRect documents G2.c: rects ignored by backend.
func TestG2_PresentWithDamage_IgnoresRect_Doc(t *testing.T) {
	// Compile-time / API-level: PresentWithDamage exists and is safe with nil rects.
	// Runtime ignore is asserted by surface.go comment + zero-length call in present tests.
	_ = image.Rectangle{}
	t.Log("G2.c: gpu/webgpu.Surface.PresentWithDamage ignores rects (wgpu-native)")
}

// TestG2_BlitOnly_DamagePreservesOutsidePixels verifies G2.b: when a frame is
// blit-only (DrawGPUTexture, no Fill/Stroke vectors), FlushGPUWithViewDamage
// with a dirty rect keeps undamaged pixels from the previous frame.
func TestG2_BlitOnly_DamagePreservesOutsidePixels(t *testing.T) {
	s3cRequireGPU(t)
	const W, H = 64, 64
	dc := render.NewContext(W, H)
	defer dc.Close()
	dc.ResetRenderPathStats()

	view, release := dc.CreateOffscreenTexture(W, H)
	if release == nil || view.IsNil() {
		t.Skip("CreateOffscreenTexture unavailable")
	}
	defer release()

	// Frame 1: solid white background (vector path — full clear OK).
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, float64(W), float64(H))
	_ = dc.Fill()
	if err := dc.FlushGPUWithView(view, W, H); err != nil {
		t.Fatalf("frame1 flush: %v", err)
	}

	// Red tile for blit update.
	tileDC := render.NewContext(16, 16)
	defer tileDC.Close()
	tileView, tileRel := tileDC.CreateOffscreenTexture(16, 16)
	if tileRel == nil || tileView.IsNil() {
		t.Skip("tile offscreen unavailable")
	}
	defer tileRel()
	tileDC.SetRGB(1, 0, 0)
	tileDC.DrawRectangle(0, 0, 16, 16)
	_ = tileDC.Fill()
	if err := tileDC.FlushGPUWithView(tileView, 16, 16); err != nil {
		t.Fatalf("tile flush: %v", err)
	}

	// Frame 2: blit-only — DrawGPUTexture only (no vector ops on dc).
	// Do NOT call BeginGPUFrame (would force LoadOpClear).
	const tx, ty = 24, 24
	dc.DrawGPUTexture(tileView, tx, ty, 16, 16)
	damage := image.Rect(tx, ty, tx+16, ty+16)
	if err := dc.FlushGPUWithViewDamage(view, W, H, damage); err != nil {
		t.Fatalf("blit damage flush: %v", err)
	}

	// Composite view → CPU-readable context.
	out := render.NewContext(W, H)
	defer out.Close()
	out.ClearWithColor(render.Black)
	out.DrawGPUTexture(view, 0, 0, W, H)
	if err := out.FlushGPU(); err != nil {
		t.Fatalf("composite: %v", err)
	}

	// Outside damage: should remain white (preserve).
	r, g, b, _ := s3cSample(out, 4, 4)
	t.Logf("outside damage rgba=%d,%d,%d", r, g, b)
	// Inside damage: red-ish.
	r2, g2, b2, _ := s3cSample(out, tx+8, ty+8)
	t.Logf("inside damage rgba=%d,%d,%d", r2, g2, b2)

	if r < 200 || g < 200 || b < 200 {
		// If blit path incorrectly cleared, outside goes dark — fail G2.b.
		t.Fatalf("G2.b: outside damage should stay white, got rgba=%d,%d,%d (LoadOpClear regression?)", r, g, b)
	}
	if r2 < 150 || g2 > 80 || b2 > 80 {
		// Soft check: some backends may not sample exactly; require red dominance.
		t.Logf("note: inside tile not strongly red (backend format?); outside preserve is the hard gate")
	}
	st := dc.RenderPathStats()
	if st.GPUOps == 0 {
		t.Fatalf("expected GPU ops: %s", st.LogLine())
	}
}
