package render

import (
	"bytes"
	"math"
	"testing"
)

func rrectTestRect(x, y, w, h float64) Rect {
	return NewRect(Pt(x, y), Pt(x+w, y+h))
}

func rrectNear(a, b float64) bool {
	return math.Abs(a-b) <= 1e-9
}

func TestRRectTypeReduction(t *testing.T) {
	rect := rrectTestRect(10, 20, 100, 60)

	tests := []struct {
		name  string
		rr    RRect
		want  RRectType
		radii [4]Point
	}{
		{
			name: "empty",
			rr:   NewRRect(rrectTestRect(10, 20, 0, 60)),
			want: RRectEmpty,
		},
		{
			name:  "rect",
			rr:    NewRRect(rect),
			want:  RRectRect,
			radii: [4]Point{},
		},
		{
			name:  "simple",
			rr:    NewRRectXY(rect, 8, 8),
			want:  RRectSimple,
			radii: [4]Point{{8, 8}, {8, 8}, {8, 8}, {8, 8}},
		},
		{
			name:  "oval",
			rr:    NewRRectXY(rect, 50, 30),
			want:  RRectOval,
			radii: [4]Point{{50, 30}, {50, 30}, {50, 30}, {50, 30}},
		},
		{
			name:  "ninepatch",
			rr:    NewRRectRadii(rect, [4]Point{{8, 8}, {16, 8}, {16, 18}, {8, 18}}),
			want:  RRectNinePatch,
			radii: [4]Point{{8, 8}, {16, 8}, {16, 18}, {8, 18}},
		},
		{
			name:  "complex",
			rr:    NewRRectRadii(rect, [4]Point{{80, 8}, {20, 8}, {80, 8}, {20, 8}}),
			want:  RRectComplex,
			radii: [4]Point{{80, 8}, {20, 8}, {80, 8}, {20, 8}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rr.Type(); got != tt.want {
				t.Fatalf("Type() = %v, want %v", got, tt.want)
			}
			for i := 0; i < 4; i++ {
				got := tt.rr.Radii(RRectCorner(i))
				if !rrectNear(got.X, tt.radii[i].X) || !rrectNear(got.Y, tt.radii[i].Y) {
					t.Fatalf("Radii(%d) = %+v, want %+v", i, got, tt.radii[i])
				}
			}
		})
	}
}

func TestRRectTypePresenceHelpers(t *testing.T) {
	rect := rrectTestRect(10, 20, 100, 60)
	if !NewRRect(rect).IsRect() {
		t.Fatal("rect RRect should report IsRect")
	}
	if !NewRRectXY(rect, 8, 8).IsSimple() {
		t.Fatal("uniform RRect should report IsSimple")
	}
	if !NewRRectXY(rect, 50, 30).IsOval() {
		t.Fatal("full-radii RRect should report IsOval")
	}
	if !NewRRectRadii(rect, [4]Point{{8, 8}, {16, 8}, {16, 18}, {8, 18}}).IsNinePatch() {
		t.Fatal("axis-aligned per-side radii should report IsNinePatch")
	}
	if !NewRRectRadii(rect, [4]Point{{80, 8}, {20, 8}, {80, 8}, {20, 8}}).IsComplex() {
		t.Fatal("mismatched left/right radii should report IsComplex")
	}
}

func TestRRectRadiiClampAndScale(t *testing.T) {
	rect := rrectTestRect(10, 20, 100, 60)

	// setRectXY scales both axes proportionally when a pair would overlap.
	rr := NewRRectXY(rect, 200, 200)
	want := 30.0 // min(100/(2*200), 60/(2*200)) * 200
	for i := 0; i < 4; i++ {
		got := rr.Radii(RRectCorner(i))
		if !rrectNear(got.X, want) || !rrectNear(got.Y, want) {
			t.Fatalf("scaled radii[%d] = %+v, want (%v,%v)", i, got, want, want)
		}
	}
	if rr.Type() != RRectSimple {
		t.Fatalf("scaled uniform RRect type = %v, want Simple", rr.Type())
	}

	// A zero axis makes the whole corner square, as in SkRRect.
	rr = NewRRectRadii(rect, [4]Point{{8, 0}, {8, 8}, {8, 8}, {8, 8}})
	if got := rr.Radii(RRectUpperLeft); got != (Point{}) {
		t.Fatalf("corner with zero axis = %+v, want square corner", got)
	}

	// Negative radii are clamped to zero, i.e. a plain rectangle.
	rr = NewRRectXY(rect, -5, 8)
	if rr.Type() != RRectRect {
		t.Fatalf("negative-radius RRect type = %v, want Rect", rr.Type())
	}
}

func TestRRectCornerAccessAndRect(t *testing.T) {
	rect := rrectTestRect(10, 20, 100, 60)
	want := [4]Point{{1, 2}, {3, 4}, {5, 6}, {7, 8}}
	rr := NewRRectRadii(rect, want)
	if got := rr.Rect(); got != rect {
		t.Fatalf("Rect() = %+v, want %+v", got, rect)
	}
	for i := 0; i < 4; i++ {
		if got := rr.Radii(RRectCorner(i)); got != want[i] {
			t.Fatalf("Radii(%d) = %+v, want %+v", i, got, want[i])
		}
	}
	got := rr.RadiiArray()
	got[0] = Point{X: 99, Y: 99}
	if rr.Radii(RRectUpperLeft) != want[0] {
		t.Fatal("RadiiArray must return a copy")
	}
}

func TestRRectPathUsesPerCornerRadii(t *testing.T) {
	rect := rrectTestRect(10, 20, 100, 60)
	rr := NewRRectRadii(rect, [4]Point{{8, 6}, {16, 12}, {24, 18}, {10, 30}})

	p := NewPath()
	p.RRect(rr)
	if p.NumVerbs() != 10 {
		t.Fatalf("NumVerbs() = %d, want 10", p.NumVerbs())
	}
	coords := p.Coords()
	checkPoint := func(name string, idx int, want Point) {
		t.Helper()
		got := Pt(coords[idx], coords[idx+1])
		if !rrectNear(got.X, want.X) || !rrectNear(got.Y, want.Y) {
			t.Fatalf("%s = %+v, want %+v", name, got, want)
		}
	}
	checkPoint("move", 0, Pt(18, 20))
	checkPoint("top-right start", 2, Pt(94, 20))
	checkPoint("top-right end", 8, Pt(110, 32))
	checkPoint("bottom-right start", 10, Pt(110, 62))
	checkPoint("bottom-right end", 16, Pt(86, 80))
	checkPoint("bottom-left start", 18, Pt(20, 80))
	checkPoint("bottom-left end", 24, Pt(10, 50))
	checkPoint("top-left start", 26, Pt(10, 26))
	checkPoint("top-left end", 32, Pt(18, 20))
}

func TestRRectPathSimpleMatchesRoundRect(t *testing.T) {
	const x, y, w, h, r = 10, 20, 100, 60, 12
	want := BuildPath().RoundRect(x, y, w, h, r).Path()
	got := NewPath()
	got.RRect(NewRRectXY(rrectTestRect(x, y, w, h), r, r))

	if got.NumVerbs() != want.NumVerbs() {
		t.Fatalf("NumVerbs() = %d, want %d", got.NumVerbs(), want.NumVerbs())
	}
	gv, wv := got.Verbs(), want.Verbs()
	gc, wc := got.Coords(), want.Coords()
	for i := range wv {
		if gv[i] != wv[i] {
			t.Fatalf("verb[%d] = %v, want %v", i, gv[i], wv[i])
		}
	}
	for i := range wc {
		if !rrectNear(gc[i], wc[i]) {
			t.Fatalf("coord[%d] = %v, want %v", i, gc[i], wc[i])
		}
	}
}

func TestContextDrawRRectSimpleMatchesRoundedRectangle(t *testing.T) {
	const size = 160
	a := NewContext(size, size)
	b := NewContext(size, size)
	defer func() { _ = a.Close() }()
	defer func() { _ = b.Close() }()

	for _, dc := range []*Context{a, b} {
		dc.ClearWithColor(White)
		dc.SetRGB(1, 0, 0)
	}
	a.DrawRoundedRectangle(20, 20, 120, 120, 18)
	a.Fill()
	b.DrawRRect(NewRRectXY(rrectTestRect(20, 20, 120, 120), 18, 18))
	b.Fill()
	_ = a.FlushGPU()
	_ = b.FlushGPU()

	if !bytes.Equal(a.pixmap.Data(), b.pixmap.Data()) {
		t.Fatal("DrawRRect with uniform radii must match DrawRoundedRectangle pixel-for-pixel")
	}
}

func TestDrawRRectPerCornerCoverage(t *testing.T) {
	dc := NewContext(200, 200)
	defer func() { _ = dc.Close() }()
	dc.ClearWithColor(White)
	dc.SetRGB(1, 0, 0)
	rr := NewRRectRadii(rrectTestRect(40, 40, 120, 120), [4]Point{
		{30, 30}, {10, 10}, {20, 20}, {5, 5},
	})
	dc.DrawRRect(rr)
	dc.Fill()
	_ = dc.FlushGPU()

	if c := dc.pixmap.GetPixel(100, 100); c.R < 0.9 || c.G > 0.1 || c.B > 0.1 {
		t.Fatalf("center = %+v, want red", c)
	}
	if c := dc.pixmap.GetPixel(45, 45); c.R < 0.9 || c.G < 0.9 || c.B < 0.9 {
		t.Fatalf("outside top-left corner = %+v, want white", c)
	}
	if c := dc.pixmap.GetPixel(60, 60); c.R < 0.9 || c.G > 0.1 || c.B > 0.1 {
		t.Fatalf("inside top-left corner = %+v, want red", c)
	}
}

func TestClipRRectDispatch(t *testing.T) {
	rect := rrectTestRect(20, 20, 120, 120)
	uniform := NewRRectXY(rect, 12, 12)
	elliptical := NewRRectXY(rect, 20, 10)
	ninePatch := NewRRectRadii(rect, [4]Point{{8, 8}, {16, 8}, {16, 18}, {8, 18}})
	complexRRect := NewRRectRadii(rect, [4]Point{{80, 8}, {20, 8}, {80, 8}, {20, 8}})

	tests := []struct {
		name        string
		setup       func(*Context)
		rr          RRect
		wantRRect   bool
		wantGPUPath bool
	}{
		{name: "uniform fast path", rr: uniform, wantRRect: true, wantGPUPath: false},
		{name: "elliptical corners", rr: elliptical, wantRRect: false, wantGPUPath: true},
		{name: "nine patch", rr: ninePatch, wantRRect: false, wantGPUPath: true},
		{name: "complex", rr: complexRRect, wantRRect: false, wantGPUPath: true},
		{
			name:  "rotated uniform falls back to path",
			setup: func(dc *Context) { dc.Rotate(0.3) },
			rr:    uniform, wantRRect: false, wantGPUPath: true,
		},
		{
			name:  "non-uniform scale falls back to path",
			setup: func(dc *Context) { dc.Scale(2, 1) },
			rr:    uniform, wantRRect: false, wantGPUPath: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dc := NewContext(200, 200)
			defer func() { _ = dc.Close() }()
			if tt.setup != nil {
				tt.setup(dc)
			}
			dc.ClipRRect(tt.rr)
			if got := dc.clipStack.IsRRectOnly(); got != tt.wantRRect {
				t.Fatalf("IsRRectOnly() = %v, want %v", got, tt.wantRRect)
			}
			if got := dc.gpuClipPath != nil; got != tt.wantGPUPath {
				t.Fatalf("gpuClipPath set = %v, want %v", got, tt.wantGPUPath)
			}
		})
	}
}

func TestClipRRectCoveragePixels(t *testing.T) {
	dc := NewContext(200, 200)
	defer func() { _ = dc.Close() }()
	dc.ClearWithColor(White)
	dc.ClipRRect(NewRRectRadii(rrectTestRect(40, 40, 120, 120), [4]Point{
		{30, 30}, {10, 10}, {20, 20}, {5, 5},
	}))
	dc.SetRGB(1, 0, 0)
	dc.DrawRectangle(0, 0, 200, 200)
	dc.Fill()
	_ = dc.FlushGPU()

	// Inside the rounded rectangle.
	if c := dc.pixmap.GetPixel(100, 100); c.R < 0.9 || c.G > 0.1 || c.B > 0.1 {
		t.Fatalf("inside = %+v, want red", c)
	}
	// Outside the top-left rounded corner, but inside its bounding square.
	if c := dc.pixmap.GetPixel(45, 45); c.R < 0.9 || c.G < 0.9 || c.B < 0.9 {
		t.Fatalf("outside corner = %+v, want white", c)
	}
	// Deep inside the corner arc.
	if c := dc.pixmap.GetPixel(60, 60); c.R < 0.9 || c.G > 0.1 || c.B > 0.1 {
		t.Fatalf("inside corner = %+v, want red", c)
	}
	// The AA band just outside the arc is neither fully white nor fully red.
	// Red paint over a white background keeps R at 1; the partial coverage shows
	// up as a G/B channel between the two endpoints.
	if c := dc.pixmap.GetPixel(48, 48); !(c.G > 0.2 && c.G < 0.98) {
		t.Fatalf("AA band = %+v, want partial red coverage", c)
	}
}

// TestRRectSquareCornersEmitNoDegenerateCubics pins the emitter shape: only
// rounded corners get a cubic; a square corner stays a plain join (Skia
// addRRect). Zero-length cubics used to poison the raster edge table.
func TestRRectSquareCornersEmitNoDegenerateCubics(t *testing.T) {
	rc := Rect{Min: Point{X: 30, Y: 20}, Max: Point{X: 190, Y: 130}}
	for ci := 0; ci < 4; ci++ {
		radii := [4]Point{}
		radii[ci] = Point{X: 40, Y: 40}
		p := NewPath()
		p.RRect(NewRRectRadii(rc, radii))
		nCubic, degenerate := 0, 0
		var cur Point
		p.Iterate(func(v PathVerb, c []float64) {
			switch v {
			case MoveTo, LineTo:
				cur = Point{X: c[0], Y: c[1]}
			case CubicTo:
				nCubic++
				end := Point{X: c[4], Y: c[5]}
				if end.Sub(cur).Length() < 1e-12 {
					degenerate++
				}
				cur = end
			}
		})
		if nCubic != 1 || degenerate != 0 {
			t.Fatalf("corner %d: cubics=%d degenerate=%d, want 1/0", ci, nCubic, degenerate)
		}
	}
	// All four rounded keeps the historical ten-verb shape.
	all := [4]Point{{X: 40, Y: 40}, {X: 40, Y: 40}, {X: 40, Y: 40}, {X: 40, Y: 40}}
	p := NewPath()
	p.RRect(NewRRectRadii(rc, all))
	if p.NumVerbs() != 10 {
		t.Fatalf("fully rounded RRect verbs=%d, want 10", p.NumVerbs())
	}
}

// TestRRectSingleCornerRadiiRasterizeRounded is the pixel-level regression: with
// only one corner rounded, that corner must come out clear and the three square
// corners inked — in both render modes, since the degenerate-cubic edge bug made
// the raster drop the arc entirely.
func TestRRectSingleCornerRadiiRasterizeRounded(t *testing.T) {
	x, y, w, h, r := 30.0, 20.0, 160.0, 110.0, 40.0
	corners := [4][2]int{{34, 24}, {186, 24}, {186, 126}, {34, 126}} // 4px inside each corner
	for mode, env := range map[string]string{"cpu": "cpu", "gpu": ""} {
		t.Setenv("GOGPU_RENDER_MODE", env)
		for ci := 0; ci < 4; ci++ {
			radii := [4]Point{}
			radii[ci] = Point{X: r, Y: r}
			p := NewPath()
			p.RRect(NewRRectRadii(
				Rect{Min: Point{X: x, Y: y}, Max: Point{X: x + w, Y: y + h}}, radii))
			dc := NewContext(194, 134)
			dc.Clear()
			dc.SetRGB(1, 0, 0)
			dc.DrawPath(p)
			if err := dc.Fill(); err != nil {
				t.Fatalf("%s corner %d: fill: %v", mode, ci, err)
			}
			im := dc.Image()
			for k := 0; k < 4; k++ {
				rr, _, _, _ := im.At(corners[k][0], corners[k][1]).RGBA()
				inked := uint8(rr>>8) > 120
				want := k != ci // rounded corner must stay clear
				if inked != want {
					t.Errorf("%s corner %d: probe %d inked=%v want %v", mode, ci, k, inked, want)
				}
			}
			_ = dc.Close()
		}
	}
}
