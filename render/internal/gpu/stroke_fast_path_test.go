//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/render"
)

type nonSolidStrokePattern struct{}

func (nonSolidStrokePattern) ColorAt(_, _ float64) render.RGBA {
	return render.RGBA{R: 1}
}

func TestStrokeSolidFastPathEligible(t *testing.T) {
	path := render.NewPath()
	path.Rectangle(0, 0, 10, 10)

	solid := render.NewPaint()
	solid.SetBrush(render.Solid(render.Red))
	if !strokeSolidFastPathEligible(true, path, path, solid) {
		t.Fatal("solid paint should use the solid fast tail")
	}

	pattern := render.NewPaint()
	pattern.SetBrush(render.BrushFromPattern(nonSolidStrokePattern{}))
	if strokeSolidFastPathEligible(true, path, path, pattern) {
		t.Fatal("non-solid paint must not use the solid-only fast tail")
	}

	if strokeSolidFastPathEligible(true, path, path, nil) {
		t.Fatal("nil paint must not use the fast tail")
	}
}

func TestFillPathUserRejectsNonSolidPaint(t *testing.T) {
	rc := &GPURenderContext{shared: NewGPUShared()}
	paint := render.NewPaint()
	paint.SetBrush(render.BrushFromPattern(nonSolidStrokePattern{}))
	if err := rc.fillPathUser(render.GPURenderTarget{}, render.NewPath(), paint, render.Identity(), 1); err != render.ErrFallbackToCPU {
		t.Fatalf("fillPathUser non-solid err=%v want %v", err, render.ErrFallbackToCPU)
	}
}
