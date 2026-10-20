package render_test

import (
	"image"
	"math"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	"github.com/energye/gpui/render/text"
)

func newMiscTestContext(t *testing.T, w, h int) *render.Context {
	t.Helper()
	// These contracts inspect PixmapForTest directly; force the CPU path so the
	// package-level render/gpu registration from the GPU test file cannot defer
	// the pixels until Image/Flush.
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	dc := render.NewContext(w, h)
	t.Cleanup(func() { _ = dc.Close() })
	return dc
}

func miscPixel(dc *render.Context, x, y int) (uint8, uint8, uint8, uint8) {
	pm := dc.PixmapForTest()
	data := pm.Data()
	i := (y*pm.Width() + x) * 4
	return data[i], data[i+1], data[i+2], data[i+3]
}

func TestDrawColorFillsClipAndPreservesState(t *testing.T) {
	dc := newMiscTestContext(t, 80, 60)
	dc.ClearWithColor(render.RGBA{R: 1, A: 1})

	// Leave a current path and an unrelated current paint behind. DrawColor
	// must fill the clip without consuming either.
	dc.SetRGBA(0, 1, 0, 1)
	dc.DrawRectangle(12, 12, 6, 6)
	dc.ClipRect(10, 10, 30, 20)
	dc.DrawColor(render.RGBA{B: 1, A: 1}, render.BlendNormal)

	if r, g, b, a := miscPixel(dc, 20, 20); r != 0 || g != 0 || b != 255 || a != 255 {
		t.Fatalf("inside clip = (%d,%d,%d,%d), want opaque blue", r, g, b, a)
	}
	if r, g, b, a := miscPixel(dc, 5, 5); r != 255 || g != 0 || b != 0 || a != 255 {
		t.Fatalf("outside clip = (%d,%d,%d,%d), want opaque red", r, g, b, a)
	}

	// The preserved path must still be fillable with the preserved paint.
	if err := dc.Fill(); err != nil {
		t.Fatalf("Fill preserved path: %v", err)
	}
	if r, g, b, a := miscPixel(dc, 14, 14); r != 0 || g != 255 || b != 0 || a != 255 {
		t.Fatalf("preserved path fill = (%d,%d,%d,%d), want opaque green", r, g, b, a)
	}
}

func TestDrawPaintUsesBrushClipAndPreservesPath(t *testing.T) {
	dc := newMiscTestContext(t, 80, 60)
	dc.ClearWithColor(render.RGBA{R: 1, A: 1})

	brush := render.NewLinearGradientBrush(10, 0, 50, 0)
	brush.AddColorStop(0, render.RGBA{R: 1, A: 1})
	brush.AddColorStop(1, render.RGBA{B: 1, A: 1})
	dc.SetFillBrush(brush)
	// Current path, deliberately not the clip: DrawPaint fills the clip only.
	dc.DrawRectangle(30, 12, 6, 6)
	dc.ClipRect(10, 10, 40, 30)
	dc.DrawPaint()

	lr, lg, lb, la := miscPixel(dc, 12, 20)
	rr, rg, rb, ra := miscPixel(dc, 48, 20)
	if la != 255 || lr <= rr || lb >= rb || lb > 110 {
		t.Fatalf("gradient left = (%d,%d,%d,%d), want red-dominant left edge", lr, lg, lb, la)
	}
	if ra != 255 || rb <= lb || rr >= lr || rr > 110 {
		t.Fatalf("gradient right = (%d,%d,%d,%d), want blue-dominant right edge", rr, rg, rb, ra)
	}
	if r, g, b, a := miscPixel(dc, 60, 20); r != 255 || g != 0 || b != 0 || a != 255 {
		t.Fatalf("outside clip = (%d,%d,%d,%d), want opaque red", r, g, b, a)
	}

	// Path survives and can be filled with the current (now solid) paint.
	dc.SetRGBA(0, 1, 0, 1)
	if err := dc.Fill(); err != nil {
		t.Fatalf("Fill preserved path: %v", err)
	}
	if r, g, b, a := miscPixel(dc, 32, 14); r != 0 || g != 255 || b != 0 || a != 255 {
		t.Fatalf("preserved path fill = (%d,%d,%d,%d), want opaque green", r, g, b, a)
	}
}

func TestClipBoundsLocalNestedAndTransformed(t *testing.T) {
	dc := newMiscTestContext(t, 100, 80)

	if got, want := dc.ClipBounds(), image.Rect(0, 0, 100, 80); got != want {
		t.Fatalf("unclipped bounds = %v, want %v", got, want)
	}
	dc.ClipRect(10, 20, 30, 40)
	if got, want := dc.ClipBounds(), image.Rect(10, 20, 40, 60); got != want {
		t.Fatalf("rect bounds = %v, want %v", got, want)
	}
	dc.ClipRect(15, 25, 10, 10)
	if got, want := dc.ClipBounds(), image.Rect(15, 25, 25, 35); got != want {
		t.Fatalf("nested bounds = %v, want %v", got, want)
	}
	dc.ResetClip()

	// Local bounds must undo the full CTM, including translation and scale.
	dc.Push()
	dc.Translate(5, 7)
	dc.Scale(2, 2)
	dc.ClipRect(10, 20, 10, 10)
	if got, want := dc.ClipBounds(), image.Rect(10, 20, 20, 30); got != want {
		t.Fatalf("transformed bounds = %v, want %v", got, want)
	}
	dc.Pop()

	// A rotated path clip is conservatively reported as its local AABB.
	dc.Push()
	dc.Translate(50, 40)
	dc.Rotate(math.Pi / 4)
	p := render.NewPath()
	p.MoveTo(-10, -10)
	p.LineTo(10, -10)
	p.LineTo(10, 10)
	p.LineTo(-10, 10)
	p.Close()
	dc.DrawPath(p)
	dc.Clip()
	got := dc.ClipBounds()
	for _, pt := range []image.Point{{X: -11, Y: -11}, {X: 11, Y: 11}} {
		if !pt.In(got) {
			t.Fatalf("rotated local bounds %v does not contain corner %v", got, pt)
		}
	}
	if got.Empty() {
		t.Fatalf("rotated local bounds must not be empty")
	}
	dc.Pop()
}

func TestDrawParagraphDrawsLinesAtOrigin(t *testing.T) {
	dc := newMiscTestContext(t, 240, 160)
	dc.ClearWithColor(render.RGBA{R: 1, G: 1, B: 1, A: 1})
	dc.SetRGBA(0, 0, 0, 1)

	source, err := text.NewFontSourceFromFile("text/testdata/source-sans/TTF/SourceSans3-Regular.ttf")
	if err != nil {
		t.Skipf("test font unavailable: %v", err)
	}
	layout := text.LayoutText("HH\nHH", source.Face(28), text.LayoutOptions{
		LineSpacing: 2.0,
	})
	if layout == nil || len(layout.Lines) < 2 {
		t.Fatalf("layout lines = %d, want 2", len(layout.Lines))
	}
	dc.DrawParagraph(layout, 20, 20)

	// Two line bands separated by the 2.0 line spacing must both carry ink.
	countInk := func(y0, y1 int) int {
		n := 0
		for y := y0; y < y1; y++ {
			for x := 0; x < 240; x++ {
				if r, g, b, a := miscPixel(dc, x, y); a != 0 && (r < 240 || g < 240 || b < 240) {
					n++
				}
			}
		}
		return n
	}
	l0 := layout.Lines[0]
	l1 := layout.Lines[1]
	band0 := countInk(20+int(l0.Y-l0.Ascent), 20+int(l0.Y+l0.Descent)+1)
	band1 := countInk(20+int(l1.Y-l1.Ascent), 20+int(l1.Y+l1.Descent)+1)
	if band0 == 0 || band1 == 0 {
		t.Fatalf("paragraph ink bands = %d/%d, want both lines visible", band0, band1)
	}
	// The x origin must be honored: no ink left of x=20 (allow AA at 19).
	for y := 0; y < 160; y++ {
		for x := 0; x < 19; x++ {
			if r, g, b, a := miscPixel(dc, x, y); a != 0 && (r < 240 || g < 240 || b < 240) {
				t.Fatalf("paragraph ink at x=%d before origin", x)
			}
		}
	}
}

func TestDrawShadowDrawsBlurredOffsetSilhouetteOnly(t *testing.T) {
	dc := newMiscTestContext(t, 160, 140)
	dc.ClearWithColor(render.RGBA{R: 1, G: 1, B: 1, A: 1})

	path := render.NewPath()
	path.MoveTo(30, 30)
	path.LineTo(100, 30)
	path.LineTo(100, 80)
	path.LineTo(30, 80)
	path.Close()
	dc.DrawShadow(path, render.RGBA{A: 1}, 10, false)

	// The silhouette itself is not painted. The blur has a long soft tail, so
	// above the path may carry a few units of shadow; it must stay near white.
	if r, g, b, a := miscPixel(dc, 65, 12); r < 245 || g < 245 || b < 245 || a != 255 {
		t.Fatalf("above shadow = (%d,%d,%d,%d), want near white", r, g, b, a)
	}
	// Below the path the offset/blurred shadow must be visible but soft. The
	// destination is opaque white, so alpha stays 255; darkness is the signal.
	r, g, b, _ := miscPixel(dc, 65, 90)
	if r > 245 || g > 245 || b > 245 {
		t.Fatalf("below shadow = (%d,%d,%d), want visible soft shadow", r, g, b)
	}
	if r < 150 {
		t.Fatalf("below shadow = (%d,%d,%d), want soft 25%% spot, not opaque black", r, g, b)
	}
	// The interior above the offset is not painted as an opaque occluder;
	// only the blurred shadow (well above black) may reach it.
	if r, g, b, _ := miscPixel(dc, 65, 35); r < 150 || g < 150 || b < 150 {
		t.Fatalf("DrawShadow painted an occluder: (%d,%d,%d)", r, g, b)
	}

	// No-op contracts.
	before := append([]byte(nil), dc.PixmapForTest().Data()...)
	dc.DrawShadow(nil, render.RGBA{A: 1}, 10, false)
	dc.DrawShadow(path, render.RGBA{A: 1}, 0, false)
	after := dc.PixmapForTest().Data()
	for i := range before {
		if before[i] != after[i] {
			t.Fatalf("no-op DrawShadow changed pixel byte %d", i)
		}
	}
}

func TestIsolatedLayerUsesPhysicalSurfaceOnHiDPI(t *testing.T) {
	dc := render.NewContextWithScale(100, 80, 2)
	t.Cleanup(func() { _ = dc.Close() })
	dc.ClearWithColor(render.RGBA{R: 1, A: 1})

	// A layer must cover the full physical surface, not just the top-left
	// logical quadrant. DrawShadow depends on this.
	dc.PushLayerIsolated(1)
	dc.SetRGBA(0, 1, 0, 1)
	dc.DrawRectangle(60, 10, 30, 20)
	if err := dc.Fill(); err != nil {
		t.Fatalf("Fill in layer: %v", err)
	}
	dc.PopLayer()

	img := dc.Image()
	if r, g, b, a := img.At(130, 40).RGBA(); r>>8 != 0 || g>>8 != 255 || b>>8 != 0 || a>>8 != 255 {
		t.Fatalf("bottom-right device pixel = (%d,%d,%d,%d), want opaque green", r>>8, g>>8, b>>8, a>>8)
	}
	if r, g, b, a := img.At(70, 20).RGBA(); r>>8 != 255 || g>>8 != 0 || b>>8 != 0 || a>>8 != 255 {
		t.Fatalf("left-of-shape device pixel = (%d,%d,%d,%d), want opaque red", r>>8, g>>8, b>>8, a>>8)
	}
}

func TestDrawColorAdvancedBlendUsesLayer(t *testing.T) {
	dc := newMiscTestContext(t, 40, 30)
	dc.ClearWithColor(render.RGBA{R: 1, A: 1})
	dc.ClipRect(5, 5, 20, 15)
	dc.DrawColor(render.RGBA{R: 0.2, G: 0.3, B: 0.8, A: 1}, render.BlendScreen)
	if r, g, b, a := miscPixel(dc, 10, 10); r != 255 || abs(int(g)-76) > 3 || abs(int(b)-204) > 3 || a != 255 {
		t.Fatalf("screen blend = (%d,%d,%d,%d), want (255,76,204,255)", r, g, b, a)
	}
	if r, g, b, a := miscPixel(dc, 2, 2); r != 255 || g != 0 || b != 0 || a != 255 {
		t.Fatalf("outside clip = (%d,%d,%d,%d), want red", r, g, b, a)
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
