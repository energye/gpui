//go:build !nogpu

package render_test

import (
	"testing"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/render"
)

// Backdrop layers are a full-surface snapshot of the parent at push time.
// These pins cover three present-only regressions found while building the
// A-group backdrop window (the CPU snapshot stayed green in all three):
//
//  1. The layer RT's first pass cleared the CPU-uploaded seed, so the layer
//     composited empty content and the presented frame went black.
//  2. The Pop blit honored the active clip; the parent queue had already been
//     materialized mid-frame, so everything outside the clip went black on
//     screen while the snapshot kept the parent.
//  3. Returning to a layer RT after rendering an inner RT was treated as a
//     fresh view and cleared it; a nested backdrop wiped the enclosing layer's
//     earlier content.

func backdropPresentRGBA(t *testing.T, dc *render.Context, w, h int) []byte {
	t.Helper()
	view, release := dc.CreateOffscreenTexture(w, h)
	if view.IsNil() || release == nil {
		t.Fatal("CreateOffscreenTexture unavailable")
	}
	defer release()
	if err := dc.FlushGPUWithView(view, uint32(w), uint32(h)); err != nil { //nolint:gosec
		t.Fatalf("FlushGPUWithView: %v", err)
	}
	type readbacker interface {
		ReadbackViewRGBA(view gpucontext.TextureView, ww, hh int) ([]byte, error)
	}
	rb, ok := dc.GPURenderContext().(readbacker)
	if !ok || rb == nil {
		t.Fatal("ReadbackViewRGBA unavailable")
	}
	rgba, err := rb.ReadbackViewRGBA(view, w, h)
	if err != nil || len(rgba) < w*h*4 {
		t.Fatalf("readback: err=%v len=%d", err, len(rgba))
	}
	return rgba
}

func backdropPixel(rgba []byte, w, x, y int) [4]int {
	i := (y*w + x) * 4
	return [4]int{int(rgba[i]), int(rgba[i+1]), int(rgba[i+2]), int(rgba[i+3])}
}

func backdropNear(got [4]int, want [4]int, tol int) bool {
	for i := 0; i < 4; i++ {
		d := got[i] - want[i]
		if d < 0 {
			d = -d
		}
		if d > tol {
			return false
		}
	}
	return true
}

// TestBackdropLayerPresentKeepsSeed pins #1: the seeded backdrop must survive
// the layer's first GPU pass.
func TestBackdropLayerPresentKeepsSeed(t *testing.T) {
	requireNativeGPU(t)
	const w, h = 64, 32
	dc := render.NewContext(w, h)
	defer dc.Close()

	dc.BeginFrame()
	// Background as a draw op (a pixmap-only clear is not replayable).
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.SetRGB(0.9, 0.1, 0.1)
	dc.DrawRectangle(0, 0, 20, h)
	_ = dc.Fill()

	// Full-surface backdrop plus a 50% black dim over the left half only.
	dc.PushBackdropLayer(render.BlendNormal, 1)
	dc.SetRGBA(0, 0, 0, 0.5)
	dc.DrawRectangle(0, 0, 20, h)
	_ = dc.Fill()
	dc.PopLayer()

	rgba := backdropPresentRGBA(t, dc, w, h)
	// Left: seeded red dimmed by 50%. A wiped seed presents black (0,0,0),
	// which is exactly the live-window failure this pin guards.
	if got := backdropPixel(rgba, w, 10, 16); !backdropNear(got, [4]int{115, 13, 13, 255}, 3) {
		t.Fatalf("dimmed backdrop = %v want ~{115,13,13,255} (seed lost)", got)
	}
	// Right: backdrop restores the white parent unchanged.
	if got := backdropPixel(rgba, w, 50, 16); !backdropNear(got, [4]int{255, 255, 255, 255}, 2) {
		t.Fatalf("backdrop outside dim = %v want white", got)
	}
}

// TestBackdropLayerClipCompositeFullSurface pins #2: a clipped backdrop layer
// still requires a full-surface Pop blit, because the parent queue was
// consumed by the mid-frame materialization.
func TestBackdropLayerClipCompositeFullSurface(t *testing.T) {
	requireNativeGPU(t)
	const w, h = 64, 32
	dc := render.NewContext(w, h)
	defer dc.Close()

	dc.BeginFrame()
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.SetRGB(0.9, 0.1, 0.1)
	dc.DrawRectangle(0, 0, 16, h)
	_ = dc.Fill()
	dc.SetRGB(0.1, 0.2, 0.9)
	dc.DrawRectangle(48, 0, 16, h)
	_ = dc.Fill()

	dc.ClipRect(24, 0, 16, h)
	dc.PushBackdropLayer(render.BlendNormal, 1)
	dc.SetRGBA(0, 0, 0, 0.5)
	dc.DrawRectangle(24, 0, 16, h)
	_ = dc.Fill()
	dc.PopLayer()
	dc.ResetClip()

	rgba := backdropPresentRGBA(t, dc, w, h)
	// Outside the clip: the parent must be present (a clip-limited blit
	// leaves these black on screen).
	if got := backdropPixel(rgba, w, 8, 16); !backdropNear(got, [4]int{229, 25, 25, 255}, 3) {
		t.Fatalf("left of clip = %v want red (clip-limited composite)", got)
	}
	if got := backdropPixel(rgba, w, 56, 16); !backdropNear(got, [4]int{25, 51, 229, 255}, 3) {
		t.Fatalf("right of clip = %v want blue (clip-limited composite)", got)
	}
	// Inside the clip: dimmed white.
	if got := backdropPixel(rgba, w, 32, 16); !backdropNear(got, [4]int{127, 127, 127, 255}, 3) {
		t.Fatalf("inside clip = %v want dimmed white", got)
	}
}

// TestBackdropLayerNestedKeepsParentContent pins #3: a nested backdrop leaves
// the enclosing layer RT to render its own RT and then returns; the enclosing
// content drawn before the nested push must survive.
func TestBackdropLayerNestedKeepsParentContent(t *testing.T) {
	requireNativeGPU(t)
	const w, h = 64, 32
	dc := render.NewContext(w, h)
	defer dc.Close()

	dc.BeginFrame()
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	dc.PushLayerIsolated(1)
	dc.SetRGB(0, 1, 0)
	dc.DrawRectangle(0, 0, w, 8)
	_ = dc.Fill()

	dc.PushBackdropLayer(render.BlendNormal, 1)
	dc.SetRGBA(0, 0, 0, 0.5)
	dc.DrawRectangle(16, 0, 32, h)
	_ = dc.Fill()
	dc.PopLayer()
	dc.PopLayer()

	rgba := backdropPresentRGBA(t, dc, w, h)
	// Outer layer band outside the nested dim: pure green.
	if got := backdropPixel(rgba, w, 4, 4); !backdropNear(got, [4]int{0, 255, 0, 255}, 2) {
		t.Fatalf("outer band = %v want green (outer RT cleared on return)", got)
	}
	// Outer band under the nested dim: dimmed green (nested snapshot saw the
	// enclosing layer, not the base white).
	if got := backdropPixel(rgba, w, 32, 4); !backdropNear(got, [4]int{0, 127, 0, 255}, 3) {
		t.Fatalf("nested dim over band = %v want ~{0,127,0}", got)
	}
	// Outer layer transparent here: base white shows through the dim.
	if got := backdropPixel(rgba, w, 4, 20); !backdropNear(got, [4]int{255, 255, 255, 255}, 2) {
		t.Fatalf("base outside band = %v want white", got)
	}
}

// TestBackdropLayerBoundedFilterKeepsParentOutside pins #4: a filter inside a
// bounded backdrop layer must not bleed over the parent outside the layer
// bounds. Skia crops a saveLayer filter's output to the layer bounds, and the
// Pop blit is full-surface (pin #2), so blurred outside pixels would overwrite
// the parent on screen even though the CPU snapshot math can hide it.
func TestBackdropLayerBoundedFilterKeepsParentOutside(t *testing.T) {
	requireNativeGPU(t)
	const w, h = 64, 32
	dc := render.NewContext(w, h)
	defer dc.Close()

	dc.BeginFrame()
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	// Red block whose left/right edges sit outside the layer bounds; a small
	// black square inside the bounds gives the blur something to smear.
	dc.SetRGB(0.9, 0.1, 0.1)
	dc.DrawRectangle(20, 0, 24, h)
	_ = dc.Fill()
	dc.SetRGB(0, 0, 0)
	dc.DrawRectangle(30, 12, 4, 8)
	_ = dc.Fill()

	dc.ClipRect(24, 0, 16, h)
	dc.PushBackdropLayer(render.BlendNormal, 1)
	dc.ApplyBlur(3)
	dc.PopLayer()
	dc.ResetClip()

	rgba := backdropPresentRGBA(t, dc, w, h)
	// Outside the bounds, next to the parent's hard red edges: the parent must
	// come through untouched (a blurred layer smears these toward white).
	if got := backdropPixel(rgba, w, 22, 16); !backdropNear(got, [4]int{229, 25, 25, 255}, 3) {
		t.Fatalf("left of bounds = %v want raw red (filter bled outside bounds)", got)
	}
	if got := backdropPixel(rgba, w, 42, 16); !backdropNear(got, [4]int{229, 25, 25, 255}, 3) {
		t.Fatalf("right of bounds = %v want raw red (filter bled outside bounds)", got)
	}
	// Inside the bounds the blur really ran: the black square's interior leaks
	// red instead of staying pure black.
	if got := backdropPixel(rgba, w, 32, 16); got[0] < 20 {
		t.Fatalf("inside bounds = %v want blurred mix (filter did not run)", got)
	}
}
