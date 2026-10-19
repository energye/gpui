package clip

import "testing"

// rectSOA builds a closed rectangle subpath.
func rectSOA(x0, y0, x1, y1 float64) *soaPath {
	return newSOAPath().moveTo(x0, y0).lineTo(x1, y0).lineTo(x1, y1).lineTo(x0, y1).close()
}

// TestMaskClipper_FillRuleWinding pins the two fill rules for clip masks with
// nested same-direction contours (winding 1 in the ring, winding 2 inside the
// inner rectangle): NonZero keeps both, EvenOdd punches the inner contour out.
func TestMaskClipper_FillRuleWinding(t *testing.T) {
	outer := rectSOA(10, 10, 90, 90)
	inner := rectSOA(30, 30, 70, 70)
	verbs := append(append([]PathVerb{}, outer.verbs...), inner.verbs...)
	coords := append(append([]float64{}, outer.coords...), inner.coords...)
	bounds := NewRect(0, 0, 100, 100)

	nonZero, err := NewMaskClipper(verbs, coords, bounds, true, FillRuleNonZero)
	if err != nil {
		t.Fatalf("NewMaskClipper(NonZero) error = %v", err)
	}
	if got := nonZero.Coverage(50, 50); got != 255 {
		t.Errorf("NonZero inner coverage = %d, want 255", got)
	}
	if got := nonZero.Coverage(20, 50); got != 255 {
		t.Errorf("NonZero ring coverage = %d, want 255", got)
	}

	evenOdd, err := NewMaskClipper(verbs, coords, bounds, true, FillRuleEvenOdd)
	if err != nil {
		t.Fatalf("NewMaskClipper(EvenOdd) error = %v", err)
	}
	if got := evenOdd.Coverage(50, 50); got != 0 {
		t.Errorf("EvenOdd inner coverage = %d, want 0", got)
	}
	if got := evenOdd.Coverage(20, 50); got != 255 {
		t.Errorf("EvenOdd ring coverage = %d, want 255", got)
	}
}

// TestMaskClipper_DisjointSubpaths pins multi-contour masks: both rectangles
// stay covered, no paint leaks between them (regression for cross-subpath
// edges and Close targeting the wrong start point).
func TestMaskClipper_DisjointSubpaths(t *testing.T) {
	a := rectSOA(10, 10, 50, 50)
	b := rectSOA(70, 70, 110, 110)
	verbs := append(append([]PathVerb{}, a.verbs...), b.verbs...)
	coords := append(append([]float64{}, a.coords...), b.coords...)
	bounds := NewRect(0, 0, 120, 120)

	mc, err := NewMaskClipper(verbs, coords, bounds, true, FillRuleNonZero)
	if err != nil {
		t.Fatalf("NewMaskClipper() error = %v", err)
	}
	if got := mc.Coverage(30, 30); got != 255 {
		t.Errorf("first subpath coverage = %d, want 255", got)
	}
	if got := mc.Coverage(90, 90); got != 255 {
		t.Errorf("second subpath coverage = %d, want 255", got)
	}
	if got := mc.Coverage(60, 60); got != 0 {
		t.Errorf("gap between subpaths coverage = %d, want 0", got)
	}
}

// TestMaskClipper_OpenSubpathImplicitClose pins fill semantics for unclosed
// contours: the trailing subpath is filled as if closed (Skia behavior).
func TestMaskClipper_OpenSubpathImplicitClose(t *testing.T) {
	p := newSOAPath().moveTo(60, 20).lineTo(20, 100).lineTo(100, 100)
	bounds := NewRect(0, 0, 120, 120)

	mc, err := NewMaskClipper(p.verbs, p.coords, bounds, true, FillRuleNonZero)
	if err != nil {
		t.Fatalf("NewMaskClipper() error = %v", err)
	}
	if got := mc.Coverage(60, 60); got == 0 {
		t.Error("open triangle inside coverage = 0, want > 0 (implicit close)")
	}
	if got := mc.Coverage(30, 30); got != 0 {
		t.Errorf("open triangle outside coverage = %d, want 0", got)
	}
}

// TestRectDifferenceMask_FractionalAA pins analytic partial coverage at
// fractional hole edges (regression for integer rounding + hard edges).
func TestRectDifferenceMask_FractionalAA(t *testing.T) {
	bounds := NewRect(0, 0, 120, 120)
	hole := NewRect(30.25, 30.25, 30, 30)

	mc, err := NewRectDifferenceMask(bounds, hole)
	if err != nil {
		t.Fatalf("NewRectDifferenceMask() error = %v", err)
	}

	tests := []struct {
		name    string
		x, y    float64
		want    int
		wantTol int
	}{
		{"hole center", 45, 45, 0, 0},
		{"solid outside", 29, 45, 255, 0},
		{"left edge pixel", 30, 45, 64, 1},   // 75% hole -> 25% coverage
		{"right edge pixel", 60, 45, 191, 1}, // 25% hole -> 75% coverage
		{"corner pixel", 30, 30, 112, 1},     // 75% * 75% hole -> 43.75% coverage
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := int(mc.Coverage(tt.x, tt.y))
			diff := got - tt.want
			if diff < 0 {
				diff = -diff
			}
			if diff > tt.wantTol {
				t.Errorf("Coverage(%v, %v) = %d, want %d (±%d)", tt.x, tt.y, got, tt.want, tt.wantTol)
			}
		})
	}
}
