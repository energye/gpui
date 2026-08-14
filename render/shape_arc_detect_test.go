package render

import (
	"math"
	"testing"
)

func TestDetectArcFromDrawArc(t *testing.T) {
	dc := NewContext(1200, 800)
	defer dc.Close()

	dc.DrawArc(1060, 461.5, 100, 0.3, 2.6)
	s := DetectShape(dc.path)
	if s.Kind != ShapeArc {
		t.Fatalf("DrawArc path kind=%d, want ShapeArc", s.Kind)
	}
	if math.Abs(s.CenterX-1060) > 1e-3 || math.Abs(s.CenterY-461.5) > 1e-3 {
		t.Errorf("center=(%.3f,%.3f), want (1060,461.5)", s.CenterX, s.CenterY)
	}
	if math.Abs(s.RadiusX-100) > 1e-3 || math.Abs(s.RadiusY-100) > 1e-3 {
		t.Errorf("radius=(%.3f,%.3f), want (100,100)", s.RadiusX, s.RadiusY)
	}
	if math.Abs(s.Angle0-0.3) > 1e-3 || math.Abs(s.Angle1-2.6) > 1e-3 {
		t.Errorf("angles=(%.3f,%.3f), want (0.3,2.6)", s.Angle0, s.Angle1)
	}
}

func TestDetectEllipticalArc(t *testing.T) {
	dc := NewContext(1200, 800)
	defer dc.Close()

	dc.DrawEllipticalArc(1060, 461.5, 90, 55, 3.4, 5.8)
	s := DetectShape(dc.path)
	if s.Kind != ShapeArc {
		t.Fatalf("elliptical arc kind=%d, want ShapeArc (elliptical arc SDF sector)", s.Kind)
	}
	// The least-squares fit is biased by the per-axis kappa approximation of
	// a flat ellipse (rx/ry ~1.6) by a few tenths of a pixel.
	if math.Abs(s.CenterX-1060) > 0.5 || math.Abs(s.CenterY-461.5) > 0.5 {
		t.Errorf("center=(%.3f,%.3f), want (1060,461.5)±0.5", s.CenterX, s.CenterY)
	}
	if math.Abs(s.RadiusX-90) > 1 || math.Abs(s.RadiusY-55) > 1 {
		t.Errorf("radius=(%.3f,%.3f), want (90,55)±1", s.RadiusX, s.RadiusY)
	}
	// Angle window must span the true arc (wrap-normalized a1 >= a0) and
	// contain both endpoint parametric angles.
	span := s.Angle1 - s.Angle0
	for s.Angle1 < s.Angle0 {
		s.Angle1 += 2 * math.Pi
	}
	if math.Abs(s.Angle1-s.Angle0-2.4) > 0.05 {
		t.Errorf("angle span=%.3f, want 2.4", s.Angle1-s.Angle0)
	}
	_ = span
	ang := func(x, y float64) float64 {
		a := math.Atan2((y-s.CenterY)/s.RadiusY, (x-s.CenterX)/s.RadiusX)
		for a < s.Angle0-1e-9 {
			a += 2 * math.Pi
		}
		return a
	}
	if a := ang(1060+90*math.Cos(3.4), 461.5+55*math.Sin(3.4)); a > s.Angle1+1e-9 {
		t.Errorf("start angle=%.3f outside [%.3f,%.3f]", a, s.Angle0, s.Angle1)
	}
	if a := ang(1060+90*math.Cos(5.8), 461.5+55*math.Sin(5.8)); a > s.Angle1+1e-9 {
		t.Errorf("end angle=%.3f outside [%.3f,%.3f]", a, s.Angle0, s.Angle1)
	}
}

func TestDetectArcRejectsLinePath(t *testing.T) {
	p := NewPath()
	p.MoveTo(10, 10)
	p.LineTo(100, 100)
	s := DetectShape(p)
	if s.Kind != ShapeUnknown {
		t.Fatalf("line path kind=%d, want ShapeUnknown", s.Kind)
	}
}

func TestDetectArcRejectsFullCircle(t *testing.T) {
	dc := NewContext(300, 300)
	defer dc.Close()

	dc.DrawCircle(150, 150, 80)
	s := DetectShape(dc.path)
	// A closed circle is ShapeCircle, not ShapeArc.
	if s.Kind != ShapeCircle {
		t.Fatalf("closed circle kind=%d, want ShapeCircle", s.Kind)
	}
}
