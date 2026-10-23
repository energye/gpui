//go:build !nogpu

package render_test

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
)

// clipOpsWhiteBG prepares a white background and returns the context.
func clipOpsWhiteBG(w, h int) *render.Context {
	dc := render.NewContext(w, h)
	dc.ClearWithColor(render.White)
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, float64(w), float64(h))
	_ = dc.Fill()
	return dc
}

// clipOpsFillRed fills the whole canvas red under the current clip.
func clipOpsFillRed(dc *render.Context, w, h int) {
	dc.SetRGB(1, 0, 0)
	dc.DrawRectangle(0, 0, float64(w), float64(h))
	_ = dc.Fill()
}

// clipOpsNestedSameDir builds a clip path of two nested, same-direction
// rectangles (outer and inner both clockwise).
func clipOpsNestedSameDir(dc *render.Context) {
	dc.MoveTo(20, 20)
	dc.LineTo(100, 20)
	dc.LineTo(100, 100)
	dc.LineTo(20, 100)
	dc.ClosePath()
	dc.MoveTo(45, 45)
	dc.LineTo(75, 45)
	dc.LineTo(75, 75)
	dc.LineTo(45, 75)
	dc.ClosePath()
}

// TestClipOps_FillRuleWinding pins Skia semantics for clip paths: the clip is
// filled using the current fill rule. Nested same-direction contours are solid
// under NonZero (default) because the inner winding number is 2.
func TestClipOps_FillRuleWinding(t *testing.T) {
	const w, h = 120, 120
	dc := clipOpsWhiteBG(w, h)
	defer dc.Close()

	clipOpsNestedSameDir(dc)
	dc.Clip()
	clipOpsFillRed(dc, w, h)

	if r, g, b, _ := s3cSample(dc, 60, 60); !(r > 200 && g < 60 && b < 60) {
		t.Errorf("NonZero inner region = %d,%d,%d, want red (winding 2 stays filled)", r, g, b)
	}
	if r, g, b, _ := s3cSample(dc, 35, 35); !(r > 200 && g < 60 && b < 60) {
		t.Errorf("NonZero ring = %d,%d,%d, want red", r, g, b)
	}
	if r, g, b, _ := s3cSample(dc, 5, 60); int(r)+int(g)+int(b) < 720 {
		t.Errorf("NonZero outside = %d,%d,%d, want white", r, g, b)
	}
}

// TestClipOps_FillRuleEvenOdd pins the even-odd clip path semantics: the same
// nested path punches the inner contour out when the fill rule is EvenOdd.
func TestClipOps_FillRuleEvenOdd(t *testing.T) {
	const w, h = 120, 120
	dc := clipOpsWhiteBG(w, h)
	defer dc.Close()

	dc.SetFillRule(render.FillRuleEvenOdd)
	clipOpsNestedSameDir(dc)
	dc.Clip()
	clipOpsFillRed(dc, w, h)

	if r, g, b, _ := s3cSample(dc, 60, 60); int(r)+int(g)+int(b) < 720 {
		t.Errorf("EvenOdd inner region = %d,%d,%d, want white (punched out)", r, g, b)
	}
	if r, g, b, _ := s3cSample(dc, 35, 35); !(r > 200 && g < 60 && b < 60) {
		t.Errorf("EvenOdd ring = %d,%d,%d, want red", r, g, b)
	}
}

// TestClipOps_DisjointSubpaths pins multi-subpath clip correctness: two
// disjoint closed rects must both stay inside the clip, with no paint leaking
// between them (regression for cross-subpath edges and Close-to-wrong-point).
func TestClipOps_DisjointSubpaths(t *testing.T) {
	const w, h = 120, 120
	dc := clipOpsWhiteBG(w, h)
	defer dc.Close()

	rect := func(x, y, rw, rh float64) {
		dc.MoveTo(x, y)
		dc.LineTo(x+rw, y)
		dc.LineTo(x+rw, y+rh)
		dc.LineTo(x, y+rh)
		dc.ClosePath()
	}
	rect(10, 10, 40, 40)
	rect(70, 70, 40, 40)
	dc.Clip()
	clipOpsFillRed(dc, w, h)

	if r, g, b, _ := s3cSample(dc, 30, 30); !(r > 200 && g < 60 && b < 60) {
		t.Errorf("first subpath = %d,%d,%d, want red", r, g, b)
	}
	if r, g, b, _ := s3cSample(dc, 90, 90); !(r > 200 && g < 60 && b < 60) {
		t.Errorf("second subpath = %d,%d,%d, want red", r, g, b)
	}
	if r, g, b, _ := s3cSample(dc, 60, 60); int(r)+int(g)+int(b) < 720 {
		t.Errorf("gap between subpaths = %d,%d,%d, want white", r, g, b)
	}
}

// TestClipOps_OpenSubpathImplicitClose pins fill semantics for unclosed
// contours: an open triangle clip still covers its area.
func TestClipOps_OpenSubpathImplicitClose(t *testing.T) {
	const w, h = 120, 120
	dc := clipOpsWhiteBG(w, h)
	defer dc.Close()

	dc.MoveTo(60, 20)
	dc.LineTo(20, 100)
	dc.LineTo(100, 100)
	// deliberately no ClosePath
	dc.Clip()
	clipOpsFillRed(dc, w, h)

	if r, g, b, _ := s3cSample(dc, 60, 60); !(r > 200 && g < 60 && b < 60) {
		t.Errorf("open triangle inside = %d,%d,%d, want red (implicit close)", r, g, b)
	}
	if r, g, b, _ := s3cSample(dc, 30, 30); int(r)+int(g)+int(b) < 720 {
		t.Errorf("open triangle outside = %d,%d,%d, want white", r, g, b)
	}
}

// TestClipOps_RectDifferenceFractionalAA pins analytic coverage at fractional
// difference-hole edges (Skia AA clip semantics; regression for integer
// rounding + hard edges).
func TestClipOps_RectDifferenceFractionalAA(t *testing.T) {
	assertRectDifferenceFractionalAA(t)
}

// TestClipOps_RectDifferenceFractionalAA_CPUFallback pins the same fractional
// edge coverage when the draw falls back to the software renderer after a GPU
// clip-mask install. The installed combined clip×user mask must not be applied
// on top of the clip stack coverage (regression for coverage² on CPU fallback).
func TestClipOps_RectDifferenceFractionalAA_CPUFallback(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	assertRectDifferenceFractionalAA(t)
}

func assertRectDifferenceFractionalAA(t *testing.T) {
	t.Helper()
	const w, h = 120, 120
	dc := clipOpsWhiteBG(w, h)
	defer dc.Close()

	dc.ClipRect(0, 0, h, h)
	dc.ClipRectOp(30.25, 30.25, 30, 30, render.ClipOpDifference)
	dc.SetRGB(0, 0.6, 0)
	dc.DrawRectangle(0, 0, float64(w), float64(h))
	_ = dc.Fill()

	// Fully inside the hole: untouched white.
	if r, g, b, _ := s3cSample(dc, 45, 45); int(r)+int(g)+int(b) < 720 {
		t.Errorf("hole center = %d,%d,%d, want white", r, g, b)
	}
	// Fully outside the hole: solid green.
	if r, g, b, _ := s3cSample(dc, 29, 45); !(r < 40 && g > 120 && b < 40) {
		t.Errorf("outside hole = %d,%d,%d, want green", r, g, b)
	}
	// Left fractional edge x=30: 25% green over white -> ~(191,229,191).
	if r, g, b, _ := s3cSample(dc, 30, 45); !(r > 150 && r < 230 && g > r && b > 150) {
		t.Errorf("left AA edge = %d,%d,%d, want ~(191,229,191)", r, g, b)
	}
	// Right fractional edge x=60: 75% green over white -> ~(64,178,64).
	if r, g, b, _ := s3cSample(dc, 60, 45); !(r < 120 && g > r+40 && b < 120) {
		t.Errorf("right AA edge = %d,%d,%d, want ~(64,178,64)", r, g, b)
	}
}

// clipOpsPathCtmScene paints the pinned scene: a full-canvas rect clip, then a
// box that must land on the device rectangle (70,40)-(110,80) applied as a path
// clip, then red over the whole canvas. dx/dy/sx/sy describe the CTM the path
// verbs travel through; the user-space coordinates are derived from them, so
// every variant resolves to the same device geometry.
func clipOpsPathCtmScene(dc *render.Context, op render.ClipOp, dx, dy, sx, sy float64) {
	const w, h = 120, 120
	dc.ClearWithColor(render.White)
	dc.ClipRect(0, 0, w, h)
	dc.Push()
	if dx != 0 || dy != 0 {
		dc.Translate(dx, dy)
	}
	if sx != 1 || sy != 1 {
		dc.Scale(sx, sy)
	}
	x0, y0 := (70-dx)/sx, (40-dy)/sy
	cw, ch := 40/sx, 40/sy
	dc.MoveTo(x0, y0)
	dc.LineTo(x0+cw, y0)
	dc.LineTo(x0+cw, y0+ch)
	dc.LineTo(x0, y0+ch)
	dc.ClosePath()
	dc.ClipPathOp(op)
	dc.SetRGB(1, 0, 0)
	// Device rect (0,0)-(w,h), expressed in user space.
	dc.DrawRectangle(-dx/sx, -dy/sy, float64(w)/sx, float64(h)/sy)
	_ = dc.Fill()
	dc.Pop()
}

// TestClipOps_PathCtmInvariance pins the property a path clip must satisfy: the
// clipped region is the shape as it lands in device space, independent of which
// CTM put it there. Every variant must match the identity scene bit for bit and
// carry the same ink the box covers. Regression: ClipPathOp transformed the
// already-baked verbs by the full matrix, doubling the offset and clipping away
// the content the clip was meant to keep.
func TestClipOps_PathCtmInvariance(t *testing.T) {
	const w, h = 120, 120
	t.Setenv("GOGPU_RENDER_MODE", "cpu")

	subtests := []struct {
		name string
		op   render.ClipOp
		ink  int
	}{
		// The box is 40x40 device pixels; replace and intersect both reduce the
		// clip to it, difference punches it out of the full canvas.
		{"replace", render.ClipOpReplace, 40 * 40},
		{"intersect", render.ClipOpIntersect, 40 * 40},
		{"difference", render.ClipOpDifference, w*h - 40*40},
	}
	variants := []struct {
		name           string
		dx, dy, sx, sy float64
	}{
		{"identity", 0, 0, 1, 1},
		{"translate", 40, 25, 1, 1},
		{"scale", 0, 0, 2, 2},
		{"translate+scale", 20, 10, 1.25, 1.25},
	}
	for _, tt := range subtests {
		t.Run(tt.name, func(t *testing.T) {
			var reference *image.RGBA
			for _, v := range variants {
				t.Run(v.name, func(t *testing.T) {
					dc := render.NewContext(w, h)
					clipOpsPathCtmScene(dc, tt.op, v.dx, v.dy, v.sx, v.sy)
					img := clipOpsImage(dc)
					dc.Close()

					if ink := clipOpsRedPixels(img); ink != tt.ink {
						t.Errorf("%v under %s: %d red pixels, want %d", tt.op, v.name, ink, tt.ink)
					}
					if reference == nil {
						reference = img
						return
					}
					if diff := clipOpsPixelDiff(reference, img); diff != 0 {
						t.Errorf("%v under %s: %d/%d pixels differ from the identity scene",
							tt.op, v.name, diff, w*h)
						for y := 0; y < h; y++ {
							for x := 0; x < w; x++ {
								if reference.RGBAAt(x, y) != img.RGBAAt(x, y) {
									t.Errorf("first diff at (%d,%d): identity=%v variant=%v",
										x, y, reference.RGBAAt(x, y), img.RGBAAt(x, y))
									return
								}
							}
						}
					}
				})
			}
		})
	}
}

// clipOpsImage copies the context's pixels out so the reference scene survives
// the context being closed.
func clipOpsImage(dc *render.Context) *image.RGBA {
	img, ok := dc.Image().(*image.RGBA)
	if !ok {
		panic("context image is not *image.RGBA")
	}
	out := image.NewRGBA(image.Rect(0, 0, img.Bounds().Dx(), img.Bounds().Dy()))
	copy(out.Pix, img.Pix)
	return out
}

func clipOpsPixelDiff(a, b *image.RGBA) int {
	diff := 0
	for y := 0; y < a.Bounds().Dy(); y++ {
		for x := 0; x < a.Bounds().Dx(); x++ {
			if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
				diff++
			}
		}
	}
	return diff
}

func clipOpsRedPixels(img *image.RGBA) int {
	n := 0
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if r>>8 > 200 && g>>8 < 60 && b>>8 < 60 {
				n++
			}
		}
	}
	return n
}

// TestClipOps_PathReplace pins ClipPathOp(ClipOpReplace): the previous clip is
// discarded and replaced by the path region.
func TestClipOps_PathReplace(t *testing.T) {
	const w, h = 120, 120
	dc := clipOpsWhiteBG(w, h)
	defer dc.Close()

	dc.ClipRect(0, 0, 60, 120) // left half
	dc.MoveTo(70, 40)
	dc.LineTo(110, 40)
	dc.LineTo(110, 80)
	dc.LineTo(70, 80)
	dc.ClosePath()
	dc.ClipPathOp(render.ClipOpReplace)
	clipOpsFillRed(dc, w, h)

	if r, g, b, _ := s3cSample(dc, 90, 60); !(r > 200 && g < 60 && b < 60) {
		t.Errorf("replaced clip region = %d,%d,%d, want red", r, g, b)
	}
	if r, g, b, _ := s3cSample(dc, 10, 60); int(r)+int(g)+int(b) < 720 {
		t.Errorf("old clip must be discarded = %d,%d,%d, want white", r, g, b)
	}
}

// TestClipOps_PathDifference pins ClipPathOp(ClipOpDifference) with a path:
// covered area is subtracted from the current clip.
func TestClipOps_PathDifference(t *testing.T) {
	const w, h = 120, 120
	dc := clipOpsWhiteBG(w, h)
	defer dc.Close()

	dc.ClipRect(0, 0, 120, 120)
	dc.MoveTo(35, 35)
	dc.LineTo(85, 35)
	dc.LineTo(85, 85)
	dc.LineTo(35, 85)
	dc.ClosePath()
	dc.ClipPathOp(render.ClipOpDifference)
	clipOpsFillRed(dc, w, h)

	if r, g, b, _ := s3cSample(dc, 60, 60); int(r)+int(g)+int(b) < 720 {
		t.Errorf("difference hole = %d,%d,%d, want white", r, g, b)
	}
	if r, g, b, _ := s3cSample(dc, 15, 60); !(r > 200 && g < 60 && b < 60) {
		t.Errorf("outside hole = %d,%d,%d, want red", r, g, b)
	}
}
