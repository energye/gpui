package core_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
)

func TestPaintGeomHelpersNoPanic(t *testing.T) {
	dc := render.NewContext(64, 64)
	defer dc.Close()
	pc := &core.PaintContext{DC: dc, Scale: 1}
	col := render.RGBA{R: 0.1, G: 0.4, B: 0.9, A: 1}
	pc.FillLocalRoundRect(4, 4, 40, 24, 6, col)
	pc.StrokeLocalRoundRect(4, 4, 40, 24, 6, 1, col)
	pc.FillLocalCircle(32, 32, 10, col)
	pc.StrokeLocalCircle(32, 32, 12, 1, col)
	pc.PaintLocalCheck(16, 16, 0, col)
	pc.PaintLocalClose(16, 16, 3, 1.5, col)
	pc.StrokeLocalPolyline([]float64{0, 0, 10, 10, 20, 0}, 1.5, col)
	if pc.SnapToDevice(1.4) != 1 {
		t.Fatalf("snap %v", pc.SnapToDevice(1.4))
	}
	// hairline center at scale 1 → n+0.5
	if h := pc.SnapHairlineCenter(2.1); h != 2.5 {
		t.Fatalf("hairline %v want 2.5", h)
	}
}
