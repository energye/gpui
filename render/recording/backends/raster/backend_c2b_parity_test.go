package raster

import (
	"image"
	"os"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/recording"
	"github.com/energye/gpui/render/text"
)

// S2b parity contract: every method family the recording surface gained in C2b
// must replay bit-identically to the live call. Each test paints the same scene
// twice — once straight onto a render.Context, once through a Recorder and back
// out via ContextBackend — and compares every byte. A difference means the
// display list lost or invented state; ink that is missing from BOTH images is a
// vacuous test, so the helper refuses those too.

func assertC2bParity(t *testing.T, w, h int, paint func(p render.Canvas)) {
	t.Helper()
	t.Setenv("GOGPU_RENDER_MODE", "cpu")

	live := render.NewContext(w, h)
	paint(live)
	liveImg, ok := live.Image().(*image.RGBA)
	if !ok {
		t.Fatalf("live image type = %T, want *image.RGBA", live.Image())
	}
	if opaquePixels(liveImg) == 0 {
		t.Fatalf("scene painted no ink over %dx%d — parity here would prove nothing", w, h)
	}

	rec := recording.NewRecorder(w, h)
	paint(rec)
	played := playbackRGBA(t, rec.FinishRecording())

	if diff := countPixelDiff(liveImg, played); diff != 0 {
		t.Errorf("playback differs from live painting in %d of %d pixels", diff, w*h)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if liveImg.RGBAAt(x, y) != played.RGBAAt(x, y) {
					t.Errorf("first diff at (%d,%d): live=%v replay=%v", x, y,
						liveImg.RGBAAt(x, y), played.RGBAAt(x, y))
					return
				}
			}
		}
	}
}

func opaquePixels(img *image.RGBA) int {
	n := 0
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			if img.RGBAAt(x, y).A != 0 {
				n++
			}
		}
	}
	return n
}

// TestC2bImagePatternParity covers SetFillPattern/SetStrokePattern: a source
// sub-rect, a stored sampling inverse, opacity, clamp and a solid pattern. The
// pattern is the one family whose recorded form is data rather than a reference,
// so bit equality here is what proves the inverse travelled un-rescaled.
func TestC2bImagePatternParity(t *testing.T) {
	img := c2bPatternImage(t)

	assertC2bParity(t, 180, 90, func(p render.Canvas) {
		// Tiled pattern through an affine sampling transform, filling a rect.
		p.Push()
		p.Translate(6, 8)
		pat := render.NewImagePattern(img, 2, 1, 7, 5)
		pat.SetTransform(render.Translate(3, 4).Multiply(render.Scale(1.5, 1.25)))
		pat.SetOpacity(0.8)
		p.SetFillPattern(pat)
		p.DrawRectangle(0, 0, 60, 40)
		if err := p.Fill(); err != nil {
			t.Errorf("fill pattern: %v", err)
		}
		p.Pop()

		// Clamped whole-image pattern used as stroke paint.
		p.Push()
		p.Translate(80, 10)
		strokePat := render.NewImagePattern(img, 0, 0, 0, 0)
		strokePat.SetClamp(true)
		p.SetStrokePattern(strokePat)
		p.SetLineWidth(6)
		p.DrawRectangle(0, 0, 44, 30)
		if err := p.Stroke(); err != nil {
			t.Errorf("stroke pattern: %v", err)
		}
		p.Pop()

		// A solid pattern collapses to the context's inline colour.
		p.Push()
		p.SetFillPattern(render.NewSolidPattern(render.RGBA{R: 1, G: 1, B: 0, A: 1}))
		p.DrawCircle(150, 65, 12)
		if err := p.Fill(); err != nil {
			t.Errorf("fill solid pattern: %v", err)
		}
		p.Pop()
	})
}

// TestC2bSetStrokeParity covers the SetStroke family. The Recorder decomposes
// the style into the same individual state the draw commands already carry, so
// a caps/join/miter/dash style must land unchanged.
func TestC2bSetStrokeParity(t *testing.T) {
	assertC2bParity(t, 180, 90, func(p render.Canvas) {
		p.Push()
		p.Translate(10, 14)
		p.SetRGBA(0.1, 0.1, 0.1, 1)
		p.SetStroke(render.Stroke{
			Width:      7,
			Cap:        render.LineCapRound,
			Join:       render.LineJoinBevel,
			MiterLimit: 9,
			Dash:       &render.Dash{Array: []float64{8, 4}, Offset: 3},
		})
		p.MoveTo(0, 0)
		p.LineTo(50, 26)
		p.LineTo(110, 4)
		if err := p.Stroke(); err != nil {
			t.Errorf("dashed stroke: %v", err)
		}
		p.Pop()

		// Miter joins with a tight limit, and a solid line after ClearDash.
		p.Push()
		p.Translate(16, 60)
		p.SetRGBA(0, 0.4, 1, 1)
		p.SetStroke(render.Stroke{Width: 5, Cap: render.LineCapSquare, Join: render.LineJoinMiter, MiterLimit: 1.2})
		p.MoveTo(0, 0)
		p.LineTo(30, -18)
		p.LineTo(60, 2)
		if err := p.Stroke(); err != nil {
			t.Errorf("miter stroke: %v", err)
		}
		p.Pop()

		// SetStroke with no dash clears whatever dash was current.
		p.Push()
		p.SetRGBA(1, 0.2, 0, 1)
		p.SetDash(2, 2)
		p.SetStroke(render.Stroke{Width: 4, Cap: render.LineCapButt, Join: render.LineJoinRound, MiterLimit: 4})
		p.MoveTo(100, 78)
		p.LineTo(170, 78)
		if err := p.Stroke(); err != nil {
			t.Errorf("cleared dash stroke: %v", err)
		}
		p.Pop()
	})
}

// TestC2bClipOpParity covers ClipRectOp and ClipPathOp: all three SkClipOps on
// both, plus a fill rule, because Context.ClipPathOp reads the rule off the
// paint rather than from an argument.
func TestC2bClipOpParity(t *testing.T) {
	assertC2bParity(t, 200, 120, func(p render.Canvas) {
		// Replace, then intersect, then difference on a rect clip.
		p.Push()
		p.SetRGBA(1, 0, 0, 1)
		p.DrawRectangle(4, 4, 60, 50)
		if err := p.Fill(); err != nil {
			t.Errorf("red fill: %v", err)
		}
		p.ClipRectOp(14, 14, 40, 30, render.ClipOpReplace)
		p.ClipRectOp(24, 4, 60, 60, render.ClipOpIntersect)
		p.SetRGBA(0, 0.8, 0, 1)
		p.DrawRectangle(0, 0, 80, 70)
		if err := p.Fill(); err != nil {
			t.Errorf("green fill: %v", err)
		}
		p.SetRGBA(0, 0, 1, 1)
		p.DrawRectangle(28, 18, 20, 20)
		if err := p.Fill(); err != nil {
			t.Errorf("blue subtract: %v", err)
		}
		p.ClipRectOp(28, 18, 20, 20, render.ClipOpDifference)
		p.SetRGBA(1, 1, 0, 1)
		p.DrawRectangle(10, 10, 60, 40)
		if err := p.Fill(); err != nil {
			t.Errorf("yellow clipped: %v", err)
		}
		p.Pop()

		// Path clips: replace + intersect + difference, even-odd star.
		p.Push()
		p.Translate(110, 10)
		p.SetFillRule(render.FillRuleEvenOdd)
		p.MoveTo(40, 0)
		p.LineTo(48, 30)
		p.LineTo(20, 12)
		p.LineTo(60, 12)
		p.LineTo(32, 30)
		p.ClosePath()
		p.ClipPathOp(render.ClipOpReplace)
		p.SetRGBA(0.9, 0.2, 0.9, 1)
		p.DrawRectangle(-20, -20, 120, 120)
		if err := p.Fill(); err != nil {
			t.Errorf("purple clip: %v", err)
		}
		p.Pop()

		p.Push()
		p.Translate(110, 60)
		p.ClipRectOp(0, 0, 80, 50, render.ClipOpReplace)
		p.MoveTo(10, 10)
		p.LineTo(50, 10)
		p.LineTo(50, 45)
		p.ClosePath()
		p.ClipPathOp(render.ClipOpIntersect)
		p.SetRGBA(0, 0.7, 0.7, 1)
		p.DrawRectangle(0, 0, 80, 50)
		if err := p.Fill(); err != nil {
			t.Errorf("teal intersect: %v", err)
		}
		p.MoveTo(30, 20)
		p.LineTo(75, 20)
		p.LineTo(75, 50)
		p.ClosePath()
		p.ClipPathOp(render.ClipOpDifference)
		p.SetRGBA(1, 0.5, 0, 1)
		p.DrawRectangle(0, 0, 80, 50)
		if err := p.Fill(); err != nil {
			t.Errorf("orange difference: %v", err)
		}
		p.Pop()
	})
}

// TestC2bMeshParity covers DrawMesh/DrawMeshEx/DrawVerticesEx: an indexed mesh,
// a triangle-list mesh without per-vertex colours (which falls back to the
// recorded paint) and a coloured fan.
func TestC2bMeshParity(t *testing.T) {
	assertC2bParity(t, 180, 100, func(p render.Canvas) {
		p.Push()
		p.Translate(8, 8)
		p.SetRGBA(0, 0, 0, 1)
		p.DrawMesh(render.Mesh{
			Positions: []render.Point{{X: 0, Y: 0}, {X: 40, Y: 0}, {X: 40, Y: 34}, {X: 0, Y: 34}, {X: 20, Y: 50}},
			Colors: []render.RGBA{
				{R: 1, G: 0, B: 0, A: 1},
				{R: 0, G: 1, B: 0, A: 1},
				{R: 0, G: 0, B: 1, A: 1},
				{R: 1, G: 1, B: 0, A: 1},
				{R: 1, G: 0.5, B: 0, A: 1},
			},
			Indices: []uint16{0, 1, 2, 0, 2, 3, 3, 2, 4},
		})
		p.Pop()

		// Triangle list with no colours: the paint at record time is the fallback.
		p.Push()
		p.Translate(70, 12)
		p.Scale(1.4, 1)
		p.SetRGBA(0.2, 0.1, 0.6, 1)
		_, err := p.DrawMeshEx(render.Mesh{
			Positions: []render.Point{{X: 0, Y: 0}, {X: 30, Y: 8}, {X: 12, Y: 34}},
		}, render.VertDrawOptions{})
		if err != nil {
			t.Errorf("DrawMeshEx: %v", err)
		}
		p.Pop()

		// DrawVerticesEx carries the same vertices DrawVertices records.
		p.Push()
		p.Translate(120, 40)
		p.SetRGBA(1, 0, 1, 1)
		res, err := p.DrawVerticesEx([]render.Point{
			{X: 0, Y: 0}, {X: 40, Y: 0}, {X: 40, Y: 40}, {X: 0, Y: 40},
		}, []render.RGBA{
			{R: 1, G: 1, B: 1, A: 1},
			{R: 0.2, G: 0.9, B: 0.2, A: 1},
			{R: 0.9, G: 0.2, B: 0.2, A: 1},
			{R: 0.1, G: 0.1, B: 0.9, A: 1},
		}, render.VertexModeTriangleFan, render.VertDrawOptions{})
		if err != nil {
			t.Errorf("DrawVerticesEx: %v", err)
		}
		_ = res // the route the rasterizer took is a live-only fact
		p.Pop()
	})
}

// TestC2bTextStateParity covers SetTextMode and SetLCDLayout — Context state no
// draw command carries, which is exactly why they needed real commands.
func TestC2bTextStateParity(t *testing.T) {
	face := c2bFace(t, 22)

	assertC2bParity(t, 240, 90, func(p render.Canvas) {
		p.SetFont(face)
		p.SetRGBA(0.05, 0.05, 0.05, 1)

		p.SetTextMode(render.TextModeAliased)
		p.SetLCDLayout(render.LCDLayoutBGR)
		p.DrawString("Hjp Qä", 8, 26)

		p.SetTextMode(render.TextModeVector)
		p.SetLCDLayout(render.LCDLayoutNone)
		p.DrawString("Hjp Qä", 8, 56)

		p.SetTextMode(render.TextModeAuto)
		p.DrawString("Hjp Qä", 8, 84)
	})
}

// TestC2bAntiAliasParity covers SetAntiAlias. The command existed but was never
// dispatched, so an aliased recording used to replay smooth.
func TestC2bAntiAliasParity(t *testing.T) {
	assertC2bParity(t, 160, 90, func(p render.Canvas) {
		p.SetRGBA(0.1, 0.5, 0.9, 1)
		p.SetAntiAlias(false)
		p.Push()
		p.Translate(34, 34)
		p.Rotate(0.4)
		p.DrawRectangle(-22, -14, 44, 28)
		if err := p.Fill(); err != nil {
			t.Errorf("aliased fill: %v", err)
		}
		p.Pop()

		p.SetAntiAlias(true)
		p.SetRGBA(0.9, 0.3, 0.1, 1)
		p.Push()
		p.Translate(110, 40)
		p.Rotate(0.4)
		p.DrawRectangle(-22, -14, 44, 28)
		if err := p.Fill(); err != nil {
			t.Errorf("anti-aliased fill: %v", err)
		}
		p.Pop()
	})
}

// TestC2bSetTransformParity covers the absolute transform setter, whose recorded
// form is the delta that carries the previous matrix into it.
func TestC2bSetTransformParity(t *testing.T) {
	assertC2bParity(t, 160, 110, func(p render.Canvas) {
		p.Push()
		p.Translate(20, 20)
		p.Scale(3, 2)
		p.SetTransform(render.Matrix{A: 1.5, B: 0.4, C: -0.4, D: 1.5, E: 30, F: 12})
		p.SetRGBA(0, 0.6, 0.2, 1)
		p.DrawRectangle(0, 0, 24, 18)
		if err := p.Fill(); err != nil {
			t.Errorf("set-transform fill: %v", err)
		}
		// Identity through the same channel, then geometry in device space.
		p.SetTransform(render.Identity())
		p.SetRGBA(1, 0.7, 0, 1)
		p.DrawRectangle(90, 70, 30, 20)
		if err := p.Fill(); err != nil {
			t.Errorf("identity fill: %v", err)
		}
		p.Pop()
	})
}

// c2bPatternImage is a 12x8 buffer with a distinct value per pixel, so a lost
// source region, a re-inverted matrix or a dropped clamp shows up as a colour
// difference rather than as a plausible-looking tile.
func c2bPatternImage(t *testing.T) *render.ImageBuf {
	t.Helper()
	buf, err := render.NewImageBuf(12, 8, render.FormatRGBA8)
	if err != nil {
		t.Fatalf("NewImageBuf: %v", err)
	}
	for y := 0; y < 8; y++ {
		for x := 0; x < 12; x++ {
			if err := buf.SetRGBA(x, y, byte(x*20), byte(y*30), byte((x+y)*10), 255); err != nil {
				t.Fatalf("SetRGBA(%d,%d): %v", x, y, err)
			}
		}
	}
	return buf
}

// c2bFace loads a system TrueType font, skipping when none exists: text state
// cannot be shown to travel without real glyph rasterization.
func c2bFace(t *testing.T, size float64) text.Face {
	t.Helper()
	paths := []string{
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/liberation/LiberationSans-Regular.ttf",
		"/Library/Fonts/Arial.ttf",
		"C:\\Windows\\Fonts\\arial.ttf",
	}
	var path string
	for _, cand := range paths {
		if _, err := os.Stat(cand); err == nil {
			path = cand
			break
		}
	}
	if path == "" {
		t.Skipf("no system TrueType font available — text state parity cannot be shown")
	}
	source, err := text.NewFontSourceFromFile(path)
	if err != nil {
		t.Skipf("font %s unusable: %v", path, err)
	}
	t.Cleanup(func() { _ = source.Close() })
	return source.Face(size)
}
