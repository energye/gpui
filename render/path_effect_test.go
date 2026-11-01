//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import (
	"math"
	"os"
	"testing"
)

func effectPolyline(t *testing.T) *Path {
	t.Helper()
	p := NewPath()
	p.MoveTo(0, 0)
	p.LineTo(100, 0)
	p.LineTo(100, 100)
	p.LineTo(0, 100)
	p.Close()
	return p
}

func near1e(v, want float64) bool {
	return math.Abs(v-want) <= 1e-4
}

func pathVerbCounts(p *Path) map[PathVerb]int {
	m := map[PathVerb]int{}
	for _, v := range p.Verbs() {
		m[v]++
	}
	return m
}

func stampSquare(side float64) *Path {
	p := NewPath()
	p.Rectangle(0, 0, side, side)
	return p
}

func stampRect(w, h float64) *Path {
	p := NewPath()
	p.Rectangle(0, 0, w, h)
	return p
}

func TestPathEffectConstructorsRejectInvalid(t *testing.T) {
	proto := stampSquare(4)
	if NewDashPathEffect(nil) != nil {
		t.Fatal("nil dash yielded an effect")
	}
	if NewDashPathEffect(NewDash(0)) != nil {
		t.Fatal("all-zero dash yielded an effect")
	}
	if NewCornerPathEffect(0) != nil || NewCornerPathEffect(-3) != nil {
		t.Fatal("non-positive corner radius yielded an effect")
	}
	if NewDiscretePathEffect(0, 1) != nil || NewDiscretePathEffect(4, 0) != nil {
		t.Fatal("degenerate discrete parameters yielded an effect")
	}
	if NewPath1DPathEffect(nil, Path1DTranslate, 10, 0) != nil {
		t.Fatal("nil proto yielded a 1D effect")
	}
	if NewPath1DPathEffect(proto, Path1DTranslate, 0, 0) != nil {
		t.Fatal("advance 0 yielded a 1D effect")
	}
	if NewPath1DPathEffect(proto, Path1DTranslate, 10, math.NaN()) != nil {
		t.Fatal("NaN phase yielded a 1D effect")
	}
	if NewPath2DPathEffect(proto, 0, 10) != nil || NewPath2DPathEffect(proto, 10, -1) != nil {
		t.Fatal("degenerate 2D grid parameters yielded an effect")
	}
	corner := NewCornerPathEffect(5)
	if got := SumPathEffects(nil, corner); got != corner {
		t.Fatal("Sum(nil, x) must fold to x")
	}
	if got := ComposePathEffects(corner, nil); got != corner {
		t.Fatal("Compose(x, nil) must fold to x")
	}
	if SumPathEffects(nil, nil) != nil || ComposePathEffects(nil, nil) != nil {
		t.Fatal("combining two nils must stay nil")
	}
}

// TestDashPathEffectMatchesApplyDash: the dash effect must equal the
// convenience field's engine output exactly (one dasher, no second copy).
func TestDashPathEffectMatchesApplyDash(t *testing.T) {
	src := effectPolyline(t)
	dash := NewDash(10, 5)
	viaEffect, ok := NewDashPathEffect(dash).FilterPath(src)
	if !ok {
		t.Fatal("dash effect rejected a drawable path")
	}
	viaEngine := ApplyDash(src, dash)
	if viaEffect.NumVerbs() != viaEngine.NumVerbs() {
		t.Fatalf("verb count %d vs engine %d", viaEffect.NumVerbs(), viaEngine.NumVerbs())
	}
	a, b := viaEffect.Coords(), viaEngine.Coords()
	for i := range b {
		if !near1e(a[i], b[i]) {
			t.Fatalf("coord[%d] %v vs engine %v", i, a[i], b[i])
		}
	}
}

func TestCornerPathEffectCutsCorners(t *testing.T) {
	src := effectPolyline(t)
	out, ok := NewCornerPathEffect(10).FilterPath(src)
	if !ok {
		t.Fatal("corner effect rejected")
	}
	vc := pathVerbCounts(out)
	if vc[QuadTo] != 4 {
		t.Fatalf("quadratic count = %d, want 4 (one per corner); verbs %+v", vc[QuadTo], vc)
	}
	if vc[LineTo] == 0 {
		t.Fatal("corner effect removed every straight run")
	}
	coords := out.Coords()
	if !near1e(coords[0], 10) || !near1e(coords[1], 0) {
		t.Fatalf("start after UL corner cut = (%v,%v), want (10,0)", coords[0], coords[1])
	}
}

func TestDiscretePathEffectDeterministic(t *testing.T) {
	src := NewPath()
	src.MoveTo(0, 0)
	src.LineTo(100, 0)
	e := NewDiscretePathEffect(8, 3)
	a, ok := e.FilterPath(src)
	if !ok {
		t.Fatal("discrete rejected")
	}
	b, _ := e.FilterPath(src)
	if a.NumVerbs() != b.NumVerbs() {
		t.Fatal("discrete not deterministic across calls")
	}
	ac, bc := a.Coords(), b.Coords()
	for i := range ac {
		if ac[i] != bc[i] {
			t.Fatalf("discrete drift at coord %d: %v vs %v", i, ac[i], bc[i])
		}
	}
	offAxis := false
	for i := 1; i < len(ac); i += 2 {
		if math.Abs(ac[i]) > 1e-9 {
			offAxis = true
			break
		}
	}
	if !offAxis {
		t.Fatal("discrete produced zero deviation on a straight line")
	}
	// Engine Path.Discrete walks half-period chunks over Flatten(0.35) samples
	// (E.02 form): ~2 verbs per drawn chunk plus seam moves, well above the
	// naive 100/8.
	if n := a.NumVerbs(); n < 20 || n > 34 {
		t.Fatalf("segment verbs = %d, want E.02 chunked output ~20-34", n)
	}
}

// TestPath1DWalkBounds pins Skia's loop bounds and PostScript phase
// normalization: stamps at offset, offset+advance, ... while distance <
// length (no stamp exactly at the contour end).
func TestPath1DWalkBoundsAndPhase(t *testing.T) {
	line := NewPath()
	line.MoveTo(0, 0)
	line.LineTo(100, 0)

	plain, ok := NewPath1DPathEffect(stampSquare(10), Path1DTranslate, 10, 0).FilterPath(line)
	if !ok {
		t.Fatal("1D effect rejected a straight line")
	}
	if moves := pathVerbCounts(plain)[MoveTo]; moves != 10 {
		t.Fatalf("stamp count = %d, want 10 (d<length excludes the endpoint stamp)", moves)
	}
	c := plain.Coords()
	if !near1e(c[0], 0) || !near1e(c[1], 0) {
		t.Fatalf("first stamp anchor = (%v,%v), want (0,0)", c[0], c[1])
	}
	// Each square stamp emits 4 points; second stamp's MoveTo pair:
	if !near1e(c[8], 10) || !near1e(c[9], 0) {
		t.Fatalf("second stamp anchor = (%v,%v), want (10,0)", c[8], c[9])
	}

	// phase=2 (PostScript): first stamp starts at 8, then 18…98.
	ph, ok := NewPath1DPathEffect(stampSquare(4), Path1DTranslate, 10, 2).FilterPath(line)
	if !ok {
		t.Fatal("phase effect rejected")
	}
	pc := ph.Coords()
	if !near1e(pc[0], 8) {
		t.Fatalf("phase-2 first anchor x = %v, want 8", pc[0])
	}
	// Negative phase flips sign without the inversion step.
	np, _ := NewPath1DPathEffect(stampSquare(4), Path1DTranslate, 10, -2).FilterPath(line)
	if nc := np.Coords(); !near1e(nc[0], 2) {
		t.Fatalf("phase -2 first anchor x = %v, want 2", nc[0])
	}
}

func TestPath1DRotateFollowsTangent(t *testing.T) {
	vertical := NewPath()
	vertical.MoveTo(0, 0)
	vertical.LineTo(0, 100)
	out, ok := NewPath1DPathEffect(stampSquare(10), Path1DRotate, 100, 0).FilterPath(vertical)
	if !ok {
		t.Fatal("rotate effect rejected")
	}
	c := out.Coords()
	// Tangent at the start points +y; the stamp's x axis must map onto it
	// (anchor at the stamp origin, not its bbox min).
	if !near1e(c[0], 0) || !near1e(c[1], 0) {
		t.Fatalf("first anchor = (%v,%v), want (0,0)", c[0], c[1])
	}
	if !near1e(c[2], 0) || !near1e(c[3], 10) {
		t.Fatalf("rotated x-corner = (%v,%v), want (0,10)", c[2], c[3])
	}
}

func TestPath1DMorphPointWiseAlongPath(t *testing.T) {
	horizontal := NewPath()
	horizontal.MoveTo(10, 20)
	horizontal.LineTo(110, 20)
	out, ok := NewPath1DPathEffect(stampRect(20, 10), Path1DMorph, 100, 0).FilterPath(horizontal)
	if !ok {
		t.Fatal("morph rejected")
	}
	c := out.Coords()
	// Stamp (0,0)→path@d=0=(10,20); (20,0)→path@d=20=(30,20);
	// (20,10)→(30,30); (0,10)→(10,30).
	want := [][2]float64{{10, 20}, {30, 20}, {30, 30}, {10, 30}}
	for i, w := range want {
		if !near1e(c[2*i], w[0]) || !near1e(c[2*i+1], w[1]) {
			t.Fatalf("morph point %d = (%v,%v), want %v", i, c[2*i], c[2*i+1], w)
		}
	}

	// The defining morph property: stamp x rides ARC LENGTH, not the x axis.
	// On a 45° line the local (20,0) corner lands at arc 20 → (20/√2, 20/√2).
	diag := NewPath()
	diag.MoveTo(0, 0)
	diag.LineTo(100, 100)
	d2, ok := NewPath1DPathEffect(stampRect(20, 0), Path1DMorph, 100, 0).FilterPath(diag)
	if !ok {
		t.Fatal("morph rejected on diagonal")
	}
	dc := d2.Coords()
	if !near1e(dc[0], 0) || !near1e(dc[1], 0) {
		t.Fatalf("diag first point = (%v,%v), want (0,0)", dc[0], dc[1])
	}
	arc := 20 / math.Sqrt2
	if !near1e(dc[2], arc) || !near1e(dc[3], arc) {
		t.Fatalf("morph arc-length point = (%v,%v), want (%v,%v)", dc[2], dc[3], arc, arc)
	}
}

func TestPath2DGridCoversBounds(t *testing.T) {
	src := NewPath()
	src.Rectangle(0, 0, 50, 50)
	proto := stampSquare(10)
	out, ok := NewPath2DPathEffect(proto, 20, 25).FilterPath(src)
	if !ok {
		t.Fatal("2D effect rejected")
	}
	vc := pathVerbCounts(out)
	// gx from 0-10=-10 to ≤50 step 20 → {-10,10,30,50} = 4;
	// gy from -10 to ≤50 step 25 → {-10,15,40} = 3.
	if want := 4 * 3; vc[MoveTo] != want {
		t.Fatalf("stamp count = %d, want %d", vc[MoveTo], want)
	}
}

// TestComposeOrderPins: compose applies inner first — swapping the operands
// must change the geometry (negative-control target).
func TestComposeOrderPins(t *testing.T) {
	src := effectPolyline(t)
	dash := NewDashPathEffect(NewDash(12, 6))
	corner := NewCornerPathEffect(8)

	fc, ok1 := ComposePathEffects(corner, dash).FilterPath(src) // dash first, corner after
	if !ok1 {
		t.Fatal("compose(dash→corner) rejected")
	}
	cf, ok2 := ComposePathEffects(dash, corner).FilterPath(src) // corner first, dash after
	if !ok2 {
		t.Fatal("compose(corner→dash) rejected")
	}
	co1, co2 := fc.Coords(), cf.Coords()
	same := len(co1) == len(co2) && fc.NumVerbs() == cf.NumVerbs()
	if same {
		for i := range co1 {
			if !near1e(co1[i], co2[i]) {
				same = false
				break
			}
		}
	}
	if same {
		t.Fatal("compose order has no effect — inner/outer swapped silently")
	}
	// Inner-first identity: compose(corner, dash) == dash then corner manually,
	// verb-for-verb and coordinate-for-coordinate (bit-for-bit, same pipeline).
	manual, _ := dash.FilterPath(src)
	manual2, _ := corner.FilterPath(manual)
	if manual2.NumVerbs() != fc.NumVerbs() {
		t.Fatalf("compose verb count %d vs manual chain %d", fc.NumVerbs(), manual2.NumVerbs())
	}
	if cm := manual2.Coords(); len(cm) != len(co1) {
		t.Fatalf("compose coord count %d vs manual chain %d", len(co1), len(cm))
	} else {
		for i := range cm {
			if cm[i] != co1[i] {
				t.Fatalf("compose coord[%d] = %v, manual chain %v (order swapped silently)", i, co1[i], cm[i])
			}
		}
	}
	// Swapped operands form the other chain: compose(dash, corner) ==
	// corner then dash manually.
	rev1, _ := corner.FilterPath(src)
	rev2, _ := dash.FilterPath(rev1)
	if rev2.NumVerbs() != cf.NumVerbs() {
		t.Fatalf("swapped compose verbs %d vs manual reverse chain %d", cf.NumVerbs(), rev2.NumVerbs())
	}
	if rc, cc := rev2.Coords(), cf.Coords(); len(rc) == len(cc) {
		for i := range rc {
			if rc[i] != cc[i] {
				t.Fatalf("swapped compose coord[%d] = %v, manual reverse chain %v", i, cc[i], rc[i])
			}
		}
	} else {
		t.Fatalf("swapped compose coord count %d vs manual reverse chain %d", len(cc), len(rc))
	}
}

func TestSumPathEffectsUnionsBoth(t *testing.T) {
	src := NewPath()
	src.MoveTo(0, 0)
	src.LineTo(100, 0)
	a, _ := NewDiscretePathEffect(10, 2).FilterPath(src)
	b, _ := NewPath1DPathEffect(stampSquare(4), Path1DTranslate, 25, 0).FilterPath(src)
	sum, ok := SumPathEffects(
		NewDiscretePathEffect(10, 2),
		NewPath1DPathEffect(stampSquare(4), Path1DTranslate, 25, 0),
	).FilterPath(src)
	if !ok {
		t.Fatal("sum rejected")
	}
	if sum.NumVerbs() != a.NumVerbs()+b.NumVerbs() {
		t.Fatalf("sum verbs = %d, want %d+%d", sum.NumVerbs(), a.NumVerbs(), b.NumVerbs())
	}
}

// TestStampEffectsRejectPathologicalDensity pins the Skia governor: an absurd
// stamp density (or a step so small the walk never progresses) must be
// rejected by the walk bound instead of looping for billions of stamps.
func TestStampEffectsRejectPathologicalDensity(t *testing.T) {
	line := NewPath()
	line.MoveTo(0, 0)
	line.LineTo(20, 0)
	if out, ok := NewPath1DPathEffect(stampSquare(2), Path1DTranslate, 1e-9, 0).FilterPath(line); ok || out != nil {
		t.Fatalf("absurd 1D advance accepted: ok=%v", ok)
	}
	long := NewPath()
	long.MoveTo(0, 0)
	long.LineTo(1<<30, 0)
	if out, ok := NewPath1DPathEffect(stampSquare(2), Path1DMorph, 1e-3, 0).FilterPath(long); ok || out != nil {
		t.Fatal("non-progressing 1D walk accepted")
	}
	grid := NewPath()
	grid.Rectangle(0, 0, 100, 100)
	if out, ok := NewPath2DPathEffect(stampSquare(2), 1e-9, 10).FilterPath(grid); ok || out != nil {
		t.Fatal("absurd 2D grid density accepted")
	}
	// Sane density still stamps normally.
	if out, ok := NewPath1DPathEffect(stampSquare(2), Path1DTranslate, 5, 0).FilterPath(line); !ok || out.NumVerbs() == 0 {
		t.Fatal("governor clipped a sane 1D walk")
	}
	if out, ok := NewPath2DPathEffect(stampSquare(2), 20, 20).FilterPath(grid); !ok || out.NumVerbs() == 0 {
		t.Fatal("governor clipped a sane 2D grid")
	}
}

func TestPathEffectHashIsParameterStable(t *testing.T) {
	e1 := NewCornerPathEffect(10)
	e2 := NewCornerPathEffect(10)
	e3 := NewCornerPathEffect(11)
	if PathEffectHash(e1) != PathEffectHash(e2) {
		t.Fatal("equal-parameter corner effects hashed apart")
	}
	if PathEffectHash(e1) == PathEffectHash(e3) {
		t.Fatal("different radii share one hash")
	}
	d1 := NewDashPathEffect(NewDash(5, 3))
	d2 := NewDashPathEffect(NewDash(5, 3))
	if PathEffectHash(d1) != PathEffectHash(d2) || PathEffectHash(d1) == PathEffectHash(e1) {
		t.Fatal("dash effect hash unstable or collides across kinds")
	}
	if PathEffectHash(nil) != 0 {
		t.Fatal("nil effect must hash 0")
	}
	s1 := NewPath1DPathEffect(stampSquare(10), Path1DTranslate, 20, 0)
	s2 := NewPath1DPathEffect(stampSquare(12), Path1DTranslate, 20, 0)
	if PathEffectHash(s1) == PathEffectHash(s2) {
		t.Fatal("1D effect ignores prototype geometry in the hash")
	}
	// Custom effects: reference-shaped instances key by identity; value-shaped
	// ones are unkeyable (0) so caches apply them instead of sharing one entry.
	p1, p2 := &ptrShiftEffect{}, &ptrShiftEffect{}
	if PathEffectHash(p1) == 0 || PathEffectHash(p2) == 0 || PathEffectHash(p1) == PathEffectHash(p2) {
		t.Fatal("pointer-shaped custom effects must key per instance")
	}
	if PathEffectHash(valShiftEffect{}) != 0 || PathEffectHash(valShiftEffect{shift: 5}) != 0 {
		t.Fatal("value-shaped custom effect must report unkeyable (0)")
	}
}

type ptrShiftEffect struct{ shift float64 }

func (v *ptrShiftEffect) FilterPath(p *Path) (*Path, bool) {
	return p.Transform(Translate(0, v.shift)), true
}

type valShiftEffect struct{ shift float64 }

func (v valShiftEffect) FilterPath(p *Path) (*Path, bool) {
	return p.Transform(Translate(0, v.shift)), true
}

func withCPUModeForEffect(t *testing.T) {
	t.Helper()
	os.Setenv("GOGPU_RENDER_MODE", "cpu")
	t.Cleanup(func() { _ = os.Unsetenv("GOGPU_RENDER_MODE") })
}

// TestStrokePathEffectCPUPath pins the mount: an effect stroke through Context
// rasterizes like the hand-filtered equivalent path, and differs from plain.
func TestStrokePathEffectCPUPath(t *testing.T) {
	withCPUModeForEffect(t)
	line := NewPath()
	line.MoveTo(20, 50)
	line.LineTo(180, 50)

	dashEffect := NewDashPathEffect(NewDash(8, 8))
	dcA := NewContext(200, 100)
	defer dcA.Close()
	dcA.ClearWithColor(White)
	dcA.SetRGB(1, 0, 0)
	dcA.SetStroke(DefaultStroke().WithWidth(4).WithPathEffect(dashEffect))
	dcA.DrawPath(line)
	dcA.Stroke()

	// The convenience dash field must give the identical result (same engine).
	dcB := NewContext(200, 100)
	defer dcB.Close()
	dcB.ClearWithColor(White)
	dcB.SetRGB(1, 0, 0)
	dcB.SetStroke(DefaultStroke().WithWidth(4).WithDashPattern(8, 8))
	dcB.DrawPath(line)
	dcB.Stroke()
	if !pixelsEqual(dcA.pixmap, dcB.pixmap) {
		t.Fatal("dash effect must rasterize identically to the Stroke.Dash field")
	}

	dcC := NewContext(200, 100)
	defer dcC.Close()
	dcC.ClearWithColor(White)
	dcC.SetRGB(1, 0, 0)
	dcC.SetStroke(DefaultStroke().WithWidth(4))
	dcC.DrawPath(line)
	dcC.Stroke()
	if pixelsEqual(dcA.pixmap, dcC.pixmap) {
		t.Fatal("dash effect produced no visual difference")
	}
}

// TestStrokePathEffectTakesPrecedenceOverDash: both slots set — the effect
// owns the geometry (Skia's single-slot semantics).
func TestStrokePathEffectTakesPrecedenceOverDash(t *testing.T) {
	withCPUModeForEffect(t)
	cornerProto := NewPath()
	cornerProto.MoveTo(20, 70)
	cornerProto.LineTo(100, 20)
	cornerProto.LineTo(180, 70)

	renderWith := func(st Stroke) *Pixmap {
		dc := NewContext(200, 100)
		t.Cleanup(func() { _ = dc.Close() })
		dc.ClearWithColor(White)
		dc.SetRGB(0, 0, 1)
		dc.SetStroke(st)
		dc.DrawPath(cornerProto)
		dc.Stroke()
		return dc.pixmap
	}

	dash := NewDash(8, 8)
	effOnly := renderWith(DefaultStroke().WithWidth(4).WithPathEffect(NewCornerPathEffect(6)))
	both := renderWith(DefaultStroke().WithWidth(4).WithDash(dash).WithPathEffect(NewCornerPathEffect(6)))
	if !pixelsEqual(effOnly, both) {
		t.Fatal("Dash leaked into a stroke that carries a PathEffect")
	}
	plain := renderWith(DefaultStroke().WithWidth(4).WithDash(dash))
	if pixelsEqual(plain, effOnly) {
		t.Fatal("effect precedence pin lost discrimination")
	}
}

// TestStrokePathEffectSoftwareRendererMount pins the second CPU mount —
// SoftwareRenderer.Stroke (device-space geometry, its own dash-scaling
// convention). The DashPathEffect must rasterize bit-for-bit like the
// Stroke.Dash convenience field (ApplyDash is the one engine behind both),
// and a stamp effect must actually change the picture.
func TestStrokePathEffectSoftwareRendererMount(t *testing.T) {
	line := NewPath()
	line.MoveTo(10, 50)
	line.LineTo(190, 50)
	zigzag := NewPath()
	zigzag.MoveTo(10, 70)
	zigzag.LineTo(60, 20)
	zigzag.LineTo(110, 70)
	zigzag.LineTo(160, 20)
	zigzag.LineTo(190, 50)

	strokeWith := func(p *Path, st Stroke) *Pixmap {
		r := NewSoftwareRenderer(200, 100)
		pm := NewPixmap(200, 100)
		pm.Clear(White)
		paint := NewPaint()
		paint.SetBrush(Solid(Red))
		s := st
		paint.Stroke = &s
		if err := r.Stroke(pm, p, paint); err != nil {
			t.Fatalf("Stroke returned error: %v", err)
		}
		return pm
	}

	dashEffect := strokeWith(line, DefaultStroke().WithWidth(4).
		WithPathEffect(NewDashPathEffect(NewDash(8, 8))))
	dashField := strokeWith(line, DefaultStroke().WithWidth(4).WithDashPattern(8, 8))
	if !pixelsEqual(dashEffect, dashField) {
		t.Fatal("dash effect must rasterize identically to Stroke.Dash through SoftwareRenderer")
	}
	plain := strokeWith(line, DefaultStroke().WithWidth(4))
	if pixelsEqual(dashEffect, plain) {
		t.Fatal("dash effect had no effect through SoftwareRenderer")
	}

	stamped := strokeWith(zigzag, DefaultStroke().WithWidth(2).
		WithPathEffect(NewPath1DPathEffect(stampSquare(6), Path1DTranslate, 18, 0)))
	plainZigzag := strokeWith(zigzag, DefaultStroke().WithWidth(2))
	if pixelsEqual(stamped, plainZigzag) {
		t.Fatal("1D stamp effect had no effect through SoftwareRenderer")
	}
}

func pixelsEqual(a, b *Pixmap) bool {
	if a.Width() != b.Width() || a.Height() != b.Height() {
		return false
	}
	da, db := a.Data(), b.Data()
	for i := range da {
		if da[i] != db[i] {
			return false
		}
	}
	return true
}
