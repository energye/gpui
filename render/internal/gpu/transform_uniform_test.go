//go:build !nogpu

package gpu

import (
	"math"
	"testing"

	"github.com/energye/gpui/render"
)

func TestF2_SimilarityGate(t *testing.T) {
	okCases := map[string]render.Matrix{
		"identity":    render.Identity(),
		"translate":   render.Translate(30, -12),
		"rotate":      render.Rotate(0.7),
		"uniform":     render.Scale(2, 2),
		"rot+scale":   render.Translate(5, 5).Multiply(render.Rotate(1.1).Multiply(render.Scale(0.5, 0.5))),
		"stage-fit":   render.Translate(3, 4).Multiply(render.Scale(0.9375, 0.9375)),
		"huge-scale":  render.Scale(100, 100),
		"tiny-scale":  render.Scale(0.01, 0.01),
		"neg-uniform": render.Scale(-2, -2),
	}
	for name, m := range okCases {
		s, ok := uniformSimilarityOK(m)
		if !ok {
			t.Errorf("%s: expected similarity ok", name)
			continue
		}
		want := math.Sqrt(m.A*m.A + m.D*m.D)
		if math.Abs(s-want) > 1e-9 {
			t.Errorf("%s: scale %v want %v", name, s, want)
		}
	}
	badCases := map[string]render.Matrix{
		"non-uniform": {A: 2, E: 3},
		"skew":        render.Shear(0.5, 0),
		"skew-y":      render.Shear(0, 0.3),
		"singular-0":  render.Scale(0, 0),
		"singular-x":  render.Scale(0, 2),
		"near-sing":   {A: 1e-12, E: 1e-12},
	}
	for name, m := range badCases {
		if _, ok := uniformSimilarityOK(m); ok {
			t.Errorf("%s: expected similarity reject", name)
		}
	}
}

func TestF2_UnbakeRoundTrip(t *testing.T) {
	orig := render.NewPath()
	orig.MoveTo(10, 20)
	orig.LineTo(100, 40)
	orig.QuadraticTo(130, 80, 90, 110)
	orig.CubicTo(60, 130, 40, 90, 10, 20)
	orig.Close()
	m := render.Translate(300, -50).Multiply(render.Rotate(0.9))
	baked := orig.Transform(m)
	inv, ok := invertAffine(m)
	if !ok {
		t.Fatal("test matrix should invert")
	}
	back := unbakePath(baked, inv)
	vo, co := orig.Verbs(), orig.Coords()
	vb, cb := back.Verbs(), back.Coords()
	if len(vo) != len(vb) || len(co) != len(cb) {
		t.Fatalf("verb/coords len mismatch %d/%d %d/%d", len(vo), len(vb), len(co), len(cb))
	}
	for i := range co {
		if math.Abs(co[i]-cb[i]) > 1e-9 {
			t.Fatalf("coord %d: %v vs %v", i, co[i], cb[i])
		}
	}
	// The F2 property: the same shape at DIFFERENT placements hashes equal
	// (rotation/translation round-trip + -0 normalization + quantization).
	// Without it the user-space cache never hits across animation frames.
	spokes := render.NewPath()
	spokes.MoveTo(-56, 0)
	spokes.LineTo(56, 0)
	spokes.MoveTo(0, -56)
	spokes.LineTo(0, 56)
	var want uint64
	for i, ang := range []float64{0.5, 0.51, 1.1, 2.0, -0.3} {
		pm := render.Translate(790, 382).Multiply(render.Rotate(ang))
		pb := spokes.Transform(pm)
		pinv, ok := invertAffine(pm)
		if !ok {
			t.Fatal("invert")
		}
		h := unbakedHash(pb, pinv)
		if i == 0 {
			want = h
		} else if h != want {
			t.Fatalf("placement %d: hash %x want %x (cache would miss every frame)", i, h, want)
		}
	}
	// Singular matrix refuses to invert.
	if _, ok := invertAffine(render.Scale(0, 1)); ok {
		t.Fatal("singular should not invert")
	}
}

func TestF2_StencilUniformLayout(t *testing.T) {
	if stencilFillUniformSize != 64 {
		t.Fatalf("stencil uniform must be 64B, got %d", stencilFillUniformSize)
	}
	m := render.Translate(7, -3)
	buf := makeStencilUniform(1200, 700, m, [4]float32{0.5, 0.25, 0.125, 1})
	if len(buf) != 64 {
		t.Fatalf("len %d", len(buf))
	}
	if got := decodeFloat32(buf[0:4]); got != 1200 {
		t.Errorf("w %v", got)
	}
	if got := decodeFloat32(buf[24:28]); got != 7 {
		t.Errorf("m02 %v", got)
	}
	if got := decodeFloat32(buf[40:44]); got != -3 {
		t.Errorf("m12 %v", got)
	}
}

func TestF2_CacheScaleKeying(t *testing.T) {
	c := NewPathGeometryCache()
	p := render.NewPath()
	p.MoveTo(0, 0)
	p.QuadraticTo(50, -40, 100, 0)
	p.Close()
	h := hashPathContent(p)
	raw := func() *render.Path { return p }
	if _, _, _, _, ok := c.GetOrTessellateAAKeyed(h, render.FillRuleNonZero, false, true, 1, raw); !ok {
		t.Fatal("insert miss failed")
	}
	if _, _, _, _, ok := c.GetOrTessellateAAKeyed(h, render.FillRuleNonZero, false, true, 1, raw); !ok {
		t.Fatal("same-scale should hit")
	}
	hits, _, _ := c.Stats()
	if hits != 1 {
		t.Fatalf("want 1 hit, got %d", hits)
	}
	// Different scale → distinct entry (miss then fill).
	if _, _, _, _, ok := c.GetOrTessellateAAKeyed(h, render.FillRuleNonZero, false, true, 2, raw); !ok {
		t.Fatal("scaled insert failed")
	}
	_, _, entries := c.Stats()
	if entries != 2 {
		t.Fatalf("want 2 entries, got %d", entries)
	}
}

func TestF2_StrokeKeyedHashConsistent(t *testing.T) {
	p := render.NewPath()
	p.MoveTo(0, 0)
	p.LineTo(60, 20)
	paint := render.NewPaint()
	paint.SetStroke(render.Stroke{Width: 5})
	k1 := makeStrokeCacheKey(p, paint, false, 0)
	k2 := makeStrokeCacheKeyHashed(hashPathContent(p), effectiveStrokeWidth(paint),
		int(paint.EffectiveLineCap()), int(paint.EffectiveLineJoin()),
		paint.EffectiveMiterLimit(), 0, false)
	if k1 != k2 {
		t.Fatalf("keyed hash inconsistent: %+v vs %+v", k1, k2)
	}
}

func TestF2_UserScaleTolerance(t *testing.T) {
	// A curve tessellated in user space at scale 2 must subdivide at least
	// as finely (in device px) as the device-space version.
	p := render.NewPath()
	p.MoveTo(0, 0)
	p.CubicTo(30, -60, 90, 60, 120, 0)
	p.Close()
	a := NewFanTessellator()
	a.TessellatePath(p)
	b := NewFanTessellator()
	b.SetUserScale(2)
	b.TessellatePath(p)
	if len(b.Vertices()) < len(a.Vertices()) {
		t.Fatalf("scaled tessellation coarser: %d vs %d verts", len(b.Vertices()), len(a.Vertices()))
	}
	// Straight lines are scale-invariant (no subdivision either way).
	q := render.NewPath()
	q.MoveTo(0, 0)
	q.LineTo(100, 0)
	q.LineTo(100, 50)
	q.Close()
	c := NewFanTessellator()
	c.TessellatePath(q)
	d := NewFanTessellator()
	d.SetUserScale(3)
	d.TessellatePath(q)
	if len(c.Vertices()) != len(d.Vertices()) {
		t.Fatalf("line tessellation should be scale-invariant: %d vs %d", len(c.Vertices()), len(d.Vertices()))
	}
}
