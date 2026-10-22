package recording

import (
	"math"
	"testing"

	"github.com/energye/gpui/render"
)

const epsilon = 1e-9

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < epsilon
}

// Matrix is an alias of render.Matrix, so these tests pin only what the
// recording package owns: that its constructors forward to render's and that
// the float-pair TransformPoint helper matches the context-facing signature
// used by Recorder.TransformPoint. The matrix algebra itself is covered by
// render/matrix_test.go.

func TestConstructorsForwardToRender(t *testing.T) {
	tests := []struct {
		name string
		got  Matrix
		want render.Matrix
	}{
		{"identity", Identity(), render.Identity()},
		{"translate", Translate(10, 20), render.Translate(10, 20)},
		{"scale", Scale(2, 3), render.Scale(2, 3)},
		{"rotate", Rotate(math.Pi / 3), render.Rotate(math.Pi / 3)},
		{"shear", Shear(0.5, 0.25), render.Shear(0.5, 0.25)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s constructor = %+v, want %+v", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestTransformPointHelper(t *testing.T) {
	m := Translate(100, 100).Multiply(Scale(2, 3))

	x, y := TransformPoint(m, 10, 10)
	if !almostEqual(x, 120) || !almostEqual(y, 130) {
		t.Errorf("TransformPoint(m, 10, 10) = (%v, %v), want (120, 130)", x, y)
	}

	rot := Rotate(math.Pi / 2)
	x, y = TransformPoint(rot, 1, 0)
	if !almostEqual(x, 0) || !almostEqual(y, 1) {
		t.Errorf("Rotate(90deg) (1, 0) = (%v, %v), want (0, 1)", x, y)
	}
}

func TestRecorderTransformPointTracksCTM(t *testing.T) {
	rec := NewRecorder(100, 100)
	rec.Translate(30, 40)
	rec.Scale(2, 2)

	x, y := rec.TransformPoint(5, 5)
	if !almostEqual(x, 40) || !almostEqual(y, 50) {
		t.Errorf("TransformPoint(5, 5) after translate+scale = (%v, %v), want (40, 50)", x, y)
	}
}

func TestInvertRoundTrip(t *testing.T) {
	m := Scale(2, 3).Multiply(Translate(10, 20))
	inv := m.Invert()

	if result := m.Multiply(inv); !result.IsIdentity() {
		t.Errorf("m.Multiply(m.Invert()) = %+v, want identity", result)
	}
	if result := inv.Multiply(m); !result.IsIdentity() {
		t.Errorf("m.Invert().Multiply(m) = %+v, want identity", result)
	}
}

func TestInvert_Singular(t *testing.T) {
	// Singular matrix (determinant = 0)
	m := Matrix{A: 1, B: 2, C: 0, D: 2, E: 4, F: 0}
	inv := m.Invert()

	// Should return identity for singular matrix
	if !inv.IsIdentity() {
		t.Errorf("Singular matrix inversion should return identity, got %+v", inv)
	}
}

// --- Rect tests ---

func TestNewRect(t *testing.T) {
	r := NewRect(10, 20, 100, 50)

	if r.X() != 10 {
		t.Errorf("X() = %v, want 10", r.X())
	}
	if r.Y() != 20 {
		t.Errorf("Y() = %v, want 20", r.Y())
	}
	if r.Width() != 100 {
		t.Errorf("Width() = %v, want 100", r.Width())
	}
	if r.Height() != 50 {
		t.Errorf("Height() = %v, want 50", r.Height())
	}
}

func TestNewRectFromPoints(t *testing.T) {
	// Test with points in order
	r1 := NewRectFromPoints(10, 20, 110, 70)
	if r1.MinX != 10 || r1.MinY != 20 || r1.MaxX != 110 || r1.MaxY != 70 {
		t.Errorf("NewRectFromPoints(10, 20, 110, 70) = %+v", r1)
	}

	// Test with points reversed
	r2 := NewRectFromPoints(110, 70, 10, 20)
	if r2.MinX != 10 || r2.MinY != 20 || r2.MaxX != 110 || r2.MaxY != 70 {
		t.Errorf("NewRectFromPoints(110, 70, 10, 20) = %+v", r2)
	}
}

func TestRect_IsEmpty(t *testing.T) {
	tests := []struct {
		name string
		r    Rect
		want bool
	}{
		{"normal", NewRect(0, 0, 100, 100), false},
		{"zero width", NewRect(0, 0, 0, 100), true},
		{"zero height", NewRect(0, 0, 100, 0), true},
		{"negative width", NewRect(100, 0, -50, 100), true},
		{"negative height", NewRect(0, 100, 100, -50), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.IsEmpty(); got != tt.want {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRect_Contains(t *testing.T) {
	r := NewRect(10, 20, 100, 50)

	tests := []struct {
		name string
		x, y float64
		want bool
	}{
		{"inside", 50, 40, true},
		{"top-left corner", 10, 20, true},
		{"bottom-right corner", 110, 70, true},
		{"outside left", 5, 40, false},
		{"outside right", 115, 40, false},
		{"outside top", 50, 15, false},
		{"outside bottom", 50, 75, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := r.Contains(tt.x, tt.y); got != tt.want {
				t.Errorf("Contains(%v, %v) = %v, want %v", tt.x, tt.y, got, tt.want)
			}
		})
	}
}

func TestRect_Union(t *testing.T) {
	r1 := NewRect(10, 20, 100, 50)
	r2 := NewRect(50, 60, 100, 50)

	u := r1.Union(r2)

	if u.MinX != 10 || u.MinY != 20 || u.MaxX != 150 || u.MaxY != 110 {
		t.Errorf("Union() = %+v, want (10, 20, 150, 110)", u)
	}
}

func TestRect_Intersect(t *testing.T) {
	r1 := NewRect(10, 20, 100, 50)
	r2 := NewRect(50, 40, 100, 50)

	i := r1.Intersect(r2)

	if i.MinX != 50 || i.MinY != 40 || i.MaxX != 110 || i.MaxY != 70 {
		t.Errorf("Intersect() = %+v, want (50, 40, 110, 70)", i)
	}
}

func TestRect_Intersect_NoOverlap(t *testing.T) {
	r1 := NewRect(10, 20, 30, 30)
	r2 := NewRect(100, 100, 50, 50)

	i := r1.Intersect(r2)

	if !i.IsEmpty() {
		t.Errorf("Intersect() with no overlap should be empty, got %+v", i)
	}
}

func TestRect_Inset(t *testing.T) {
	r := NewRect(10, 20, 100, 50)
	inset := r.Inset(5, 10)

	if inset.MinX != 15 || inset.MinY != 30 || inset.MaxX != 105 || inset.MaxY != 60 {
		t.Errorf("Inset(5, 10) = %+v, want (15, 30, 105, 60)", inset)
	}
}

func TestRect_Offset(t *testing.T) {
	r := NewRect(10, 20, 100, 50)
	offset := r.Offset(15, -5)

	if offset.MinX != 25 || offset.MinY != 15 || offset.MaxX != 125 || offset.MaxY != 65 {
		t.Errorf("Offset(15, -5) = %+v, want (25, 15, 125, 65)", offset)
	}
}
