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

func TestDetectArcRejectsEllipticalArc(t *testing.T) {
	dc := NewContext(1200, 800)
	defer dc.Close()

	dc.DrawEllipticalArc(1060, 461.5, 90, 55, 3.4, 5.8)
	s := DetectShape(dc.path)
	if s.Kind != ShapeUnknown {
		t.Fatalf("elliptical arc kind=%d, want ShapeUnknown (SDF arc sector is circle-only)", s.Kind)
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
