//go:build !nogpu

package render_test

import (
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
