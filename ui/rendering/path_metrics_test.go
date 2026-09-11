package rendering_test

import (
	"math"
	"testing"

	"github.com/energye/gpui/ui/rendering"
)

func almostEq(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Fatalf("%s: got %v want %v (tol %v)", name, got, want, tol)
	}
}

// TestPathMetrics_Line_LengthAndEndpoints drives the shipped UI metrics API
// on a horizontal line: length, position at 0 / mid / end.
func TestPathMetrics_Line_LengthAndEndpoints(t *testing.T) {
	p := rendering.NewPath()
	p.MoveTo(0, 0)
	p.LineTo(100, 0)

	ms := rendering.ComputePathMetrics(p, 0.25)
	if len(ms) != 1 {
		t.Fatalf("contours=%d want 1", len(ms))
	}
	almostEq(t, "length", ms[0].Length(), 100, 1e-6)
	almostEq(t, "PathTotalLength", rendering.PathTotalLength(p, 0.25), 100, 1e-6)

	x0, y0, ok := rendering.PathPositionAt(p, 0, 0.25)
	if !ok {
		t.Fatal("PositionAt(0) not ok")
	}
	almostEq(t, "start X", x0, 0, 1e-6)
	almostEq(t, "start Y", y0, 0, 1e-6)

	x1, y1, ok := rendering.PathPositionAt(p, 100, 0.25)
	if !ok {
		t.Fatal("PositionAt(end) not ok")
	}
	almostEq(t, "end X", x1, 100, 1e-6)
	almostEq(t, "end Y", y1, 0, 1e-6)

	xm, ym, ok := rendering.PathPositionAt(p, 50, 0.25)
	if !ok {
		t.Fatal("PositionAt(mid) not ok")
	}
	almostEq(t, "mid X", xm, 50, 1e-6)
	almostEq(t, "mid Y", ym, 0, 1e-6)

	// Tangent along +X.
	_, _, tx, ty, ok := rendering.PathTangentAt(p, 25, 0.25)
	if !ok {
		t.Fatal("TangentAt not ok")
	}
	almostEq(t, "tangent X", tx, 1, 1e-6)
	almostEq(t, "tangent Y", ty, 0, 1e-6)
}

// TestPathMetrics_ClosedRect_IncludesClosingEdge verifies Close contributes
// to perimeter (unlike historical render.Path.Length which skipped Close).
func TestPathMetrics_ClosedRect_IncludesClosingEdge(t *testing.T) {
	p := rendering.NewPath()
	p.MoveTo(0, 0)
	p.LineTo(10, 0)
	p.LineTo(10, 10)
	p.LineTo(0, 10)
	p.Close() // closing edge back to (0,0)

	got := rendering.PathTotalLength(p, 0.25)
	almostEq(t, "closed square perimeter", got, 40, 1e-6)

	ms := rendering.ComputePathMetrics(p, 0.25)
	if len(ms) != 1 {
		t.Fatalf("contours=%d want 1", len(ms))
	}
	// Sample at end of perimeter should be near start.
	x, y, ok := rendering.PathPositionAt(p, 40, 0.25)
	if !ok {
		t.Fatal("PositionAt(perimeter) not ok")
	}
	almostEq(t, "wrap X", x, 0, 1e-6)
	almostEq(t, "wrap Y", y, 0, 1e-6)
}

// TestPathMetrics_Cubic_NonTrivialLength samples a cubic with positive length
// and endpoint agreement (shipped ComputePathMetrics / PositionAt).
func TestPathMetrics_Cubic_NonTrivialLength(t *testing.T) {
	p := rendering.NewPath()
	p.MoveTo(0, 0)
	p.CubicTo(30, 80, 70, 80, 100, 0)

	ms := rendering.ComputePathMetrics(p, 0.1)
	if len(ms) != 1 {
		t.Fatalf("contours=%d want 1", len(ms))
	}
	L := ms[0].Length()
	if L < 100 {
		// Chord is 100; bowed cubic must be strictly longer.
		t.Fatalf("cubic length %v want > 100 (bowed path)", L)
	}
	if L > 250 {
		t.Fatalf("cubic length %v unreasonably large", L)
	}

	x0, y0, ok := rendering.PathPositionAt(p, 0, 0.1)
	if !ok {
		t.Fatal("start sample")
	}
	almostEq(t, "cubic start X", x0, 0, 0.5)
	almostEq(t, "cubic start Y", y0, 0, 0.5)

	x1, y1, ok := rendering.PathPositionAt(p, L, 0.1)
	if !ok {
		t.Fatal("end sample")
	}
	almostEq(t, "cubic end X", x1, 100, 0.5)
	almostEq(t, "cubic end Y", y1, 0, 0.5)
}

// TestPathMetrics_MultiContour_SeparateLengths ensures MoveTo starts a new metric.
func TestPathMetrics_MultiContour_SeparateLengths(t *testing.T) {
	p := rendering.NewPath()
	p.MoveTo(0, 0)
	p.LineTo(10, 0)
	p.MoveTo(0, 5)
	p.LineTo(20, 5)

	ms := rendering.ComputePathMetrics(p, 0.25)
	if len(ms) != 2 {
		t.Fatalf("contours=%d want 2", len(ms))
	}
	almostEq(t, "c0", ms[0].Length(), 10, 1e-6)
	almostEq(t, "c1", ms[1].Length(), 20, 1e-6)
	almostEq(t, "total", rendering.PathTotalLength(p, 0.25), 30, 1e-6)

	// PathPositionAt uses first non-empty contour only.
	x, y, ok := rendering.PathPositionAt(p, 5, 0.25)
	if !ok {
		t.Fatal("first contour sample")
	}
	almostEq(t, "first mid X", x, 5, 1e-6)
	almostEq(t, "first mid Y", y, 0, 1e-6)
}

func TestPathMetrics_EmptyPath(t *testing.T) {
	p := rendering.NewPath()
	if ms := rendering.ComputePathMetrics(p, 0.25); len(ms) != 0 {
		t.Fatalf("empty path metrics=%d", len(ms))
	}
	if _, _, ok := rendering.PathPositionAt(p, 0, 0.25); ok {
		t.Fatal("empty PositionAt should not ok")
	}
	if rendering.PathTotalLength(nil, 0.25) != 0 {
		t.Fatal("nil total")
	}
}

// TestPathMetrics_FillPathStillWorks ensures metrics land without breaking draw.
