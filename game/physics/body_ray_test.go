package physics

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/game/core"
)

type rayBodyDef struct {
	Name    string     `json:"name"`
	Shape   string     `json:"shape"`
	Pos     [2]float64 `json:"pos"`
	Half    [2]float64 `json:"half"`
	Radius  float64    `json:"radius"`
	Layer   uint32     `json:"layer"`
	Mask    uint32     `json:"mask"`
	Trigger bool       `json:"trigger"`
}

type rayDef struct {
	Name    string     `json:"name"`
	Origin  [2]float64 `json:"origin"`
	Dir     [2]float64 `json:"dir"`
	MaxDist float64    `json:"maxdist"`
	Mask    uint32     `json:"mask"`
}

type castDef struct {
	Name       string     `json:"name"`
	Ray        string     `json:"ray"`
	Bodies     []string   `json:"bodies"`
	WantHit    bool       `json:"want_hit"`
	WantBody   string     `json:"want_body"`
	WantDist   float64    `json:"want_dist"`
	WantPos    [2]float64 `json:"want_pos"`
	WantNormal [2]float64 `json:"want_normal"`
	WantTrig   bool       `json:"want_trigger"`
}

type standDef struct {
	Name     string `json:"name"`
	Rider    string `json:"rider"`
	Platform string `json:"platform"`
	Want     bool   `json:"want"`
}

type carryDef struct {
	Name        string     `json:"name"`
	Rider       string     `json:"rider"`
	Platform    string     `json:"platform"`
	Delta       [2]float64 `json:"delta"`
	WantCarried bool       `json:"want_carried"`
	WantPos     [2]float64 `json:"want_rider_pos"`
}

type rayFile struct {
	Bodies []rayBodyDef `json:"bodies"`
	Rays   []rayDef     `json:"rays"`
	Casts  []castDef    `json:"casts"`
	Stands []standDef   `json:"stands"`
	Carrys []carryDef   `json:"carries"`
}

func loadRayCases(t *testing.T) rayFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "body_ray_cases.json"))
	if err != nil {
		t.Fatalf("read body_ray_cases.json: %v", err)
	}
	var f rayFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode body_ray_cases.json: %v", err)
	}
	if len(f.Bodies) == 0 || len(f.Rays) == 0 || len(f.Casts) == 0 {
		t.Fatal("body_ray_cases.json has no bodies, rays, or casts")
	}
	if len(f.Stands) == 0 || len(f.Carrys) == 0 {
		t.Fatal("body_ray_cases.json has no stands or carries")
	}
	return f
}

func buildRayBody(t *testing.T, d rayBodyDef) Body {
	t.Helper()
	pos := core.V2(d.Pos[0], d.Pos[1])
	switch d.Shape {
	case "box":
		b, err := NewBox(d.Name, pos, core.V2(d.Half[0], d.Half[1]), d.Layer, d.Mask, d.Trigger)
		if err != nil {
			t.Fatalf("NewBox %q: %v", d.Name, err)
		}
		return b
	case "circle":
		b, err := NewCircle(d.Name, pos, d.Radius, d.Layer, d.Mask, d.Trigger)
		if err != nil {
			t.Fatalf("NewCircle %q: %v", d.Name, err)
		}
		return b
	default:
		t.Fatalf("unknown shape %q for %q", d.Shape, d.Name)
		return Body{}
	}
}

func rayBodyMap(t *testing.T, f rayFile) map[string]Body {
	t.Helper()
	m := map[string]Body{}
	for _, d := range f.Bodies {
		if _, dup := m[d.Name]; dup {
			t.Fatalf("duplicate body %q", d.Name)
		}
		m[d.Name] = buildRayBody(t, d)
	}
	return m
}

func rayMap(t *testing.T, f rayFile) map[string]Ray {
	t.Helper()
	m := map[string]Ray{}
	for _, d := range f.Rays {
		if _, dup := m[d.Name]; dup {
			t.Fatalf("duplicate ray %q", d.Name)
		}
		r, err := NewRay(core.V2(d.Origin[0], d.Origin[1]), core.V2(d.Dir[0], d.Dir[1]), d.MaxDist, d.Mask)
		if err != nil {
			t.Fatalf("NewRay %q: %v", d.Name, err)
		}
		m[d.Name] = r
	}
	return m
}

func mustFindRay(t *testing.T, m map[string]Ray, name string) Ray {
	t.Helper()
	r, ok := m[name]
	if !ok {
		t.Fatalf("ray %q not found", name)
	}
	return r
}

func mustFindRayBody(t *testing.T, m map[string]Body, what, name string) Body {
	t.Helper()
	b, ok := m[name]
	if !ok {
		t.Fatalf("%s unknown body %q", what, name)
	}
	return b
}

func castBodies(t *testing.T, m map[string]Body, c castDef) []Body {
	t.Helper()
	out := make([]Body, 0, len(c.Bodies))
	for _, n := range c.Bodies {
		out = append(out, mustFindRayBody(t, m, "cast "+c.Name, n))
	}
	return out
}

// A:射线平台对:最近命中加触发标记加站立加带着走全落在冻结数上.
func TestRayPlatformFromCases(t *testing.T) {
	f := loadRayCases(t)
	bm := rayBodyMap(t, f)
	rm := rayMap(t, f)
	for _, c := range f.Casts {
		ray := mustFindRay(t, rm, c.Ray)
		bodies := castBodies(t, bm, c)
		snap := append([]Body(nil), bodies...)
		got, hit, err := CastRay(bodies, ray)
		if err != nil {
			t.Errorf("cast %s: %v", c.Name, err)
			continue
		}
		if hit != c.WantHit {
			t.Errorf("cast %s hit = %v, want %v", c.Name, hit, c.WantHit)
			continue
		}
		if !hit {
			continue
		}
		if got.Name != c.WantBody {
			t.Errorf("cast %s body = %q, want %q", c.Name, got.Name, c.WantBody)
		}
		if got.Dist != c.WantDist {
			t.Errorf("cast %s dist = %v, want %v", c.Name, got.Dist, c.WantDist)
		}
		wantPos := core.V2(c.WantPos[0], c.WantPos[1])
		if got.Pos != wantPos {
			t.Errorf("cast %s pos = %v, want %v", c.Name, got.Pos, wantPos)
		}
		wantN := core.V2(c.WantNormal[0], c.WantNormal[1])
		if got.Normal != wantN {
			t.Errorf("cast %s normal = %v, want %v", c.Name, got.Normal, wantN)
		}
		if got.Trigger != c.WantTrig {
			t.Errorf("cast %s trigger = %v, want %v", c.Name, got.Trigger, c.WantTrig)
		}
		wantIdx := -1
		for i, n := range c.Bodies {
			if n == c.WantBody {
				wantIdx = i
				break
			}
		}
		if got.Index != wantIdx {
			t.Errorf("cast %s index = %d, want %d", c.Name, got.Index, wantIdx)
		}
		for i := range bodies {
			if !sameBody(bodies[i], snap[i]) {
				t.Errorf("cast %s mutated input", c.Name)
				break
			}
		}
	}
	for _, s := range f.Stands {
		rider := mustFindRayBody(t, bm, "stand "+s.Name, s.Rider)
		plat := mustFindRayBody(t, bm, "stand "+s.Name, s.Platform)
		if got := IsStandingOn(rider, plat); got != s.Want {
			t.Errorf("stand %s = %v, want %v", s.Name, got, s.Want)
		}
		if got := IsStandingOn(plat, rider); got && s.Want && s.Rider != s.Platform {
			// Standing faces up only; the flipped pair must stay false
			// except for the degenerate same-body probe covered in B.
			t.Errorf("stand %s flipped = true, want false", s.Name)
		}
	}
	for _, c := range f.Carrys {
		rider := mustFindRayBody(t, bm, "carry "+c.Name, c.Rider)
		plat := mustFindRayBody(t, bm, "carry "+c.Name, c.Platform)
		delta := core.V2(c.Delta[0], c.Delta[1])
		carried, err := CarryRider(&rider, plat, delta)
		if err != nil {
			t.Errorf("carry %s: %v", c.Name, err)
			continue
		}
		if carried != c.WantCarried {
			t.Errorf("carry %s carried = %v, want %v", c.Name, carried, c.WantCarried)
		}
		wantPos := core.V2(c.WantPos[0], c.WantPos[1])
		if rider.Pos != wantPos {
			t.Errorf("carry %s rider pos = %v, want %v", c.Name, rider.Pos, wantPos)
		}
		if carried {
			moved := plat
			moved.Pos = plat.Pos.Add(delta)
			if !IsStandingOn(rider, moved) {
				t.Errorf("carry %s lost contact after step", c.Name)
			}
		}
	}
}

// B:同位空零超大坏数据全不崩不卡死,占位加报错.
func TestRayEdgesNoCrash(t *testing.T) {
	f := loadRayCases(t)
	bm := rayBodyMap(t, f)
	wall := mustFindRayBody(t, bm, "edge", "wall_box")
	good, err := NewRay(core.V2(0, 0), core.V2(1, 0), 20, 1)
	if err != nil {
		t.Fatalf("good ray: %v", err)
	}
	// Empty and nil bodies report no hit, never an error.
	if _, hit, err := CastRay(nil, good); err != nil || hit {
		t.Errorf("nil bodies = %v/%v, want no hit", hit, err)
	}
	if _, hit, err := CastRay([]Body{}, good); err != nil || hit {
		t.Errorf("empty bodies = %v/%v, want no hit", hit, err)
	}
	// Mask 0 hits nothing even on a direct line.
	masked, _ := NewRay(core.V2(0, 0), core.V2(1, 0), 20, 0)
	if _, hit, err := CastRay([]Body{wall}, masked); err != nil || hit {
		t.Errorf("mask0 = %v/%v, want no hit", hit, err)
	}
	// Same-position twin boxes: tie keeps the smaller index, no crash.
	dup0, _ := NewBox("dup0", core.V2(10, 0), core.V2(2, 2), 1, 1, false)
	dup1, _ := NewBox("dup1", core.V2(10, 0), core.V2(2, 2), 1, 1, false)
	got, hit, err := CastRay([]Body{dup0, dup1}, good)
	if err != nil || !hit || got.Index != 0 || got.Dist != 8 {
		t.Errorf("twin boxes = %+v/%v/%v, want index 0 dist 8", got, hit, err)
	}
	// Zero-size points: on-line point still needs exact collinearity;
	// off-line never hits and never panics.
	pt, _ := NewBox("pt", core.V2(10, 3), core.Vec2{}, 1, 1, false)
	if _, hit, err := CastRay([]Body{pt}, good); err != nil || hit {
		t.Errorf("off-line point = %v/%v, want no hit", hit, err)
	}
	ptOn, _ := NewBox("pton", core.V2(10, 0), core.Vec2{}, 1, 1, false)
	if _, hit, err := CastRay([]Body{ptOn}, good); err != nil || !hit {
		t.Errorf("on-line point = %v/%v, want hit", hit, err)
	}
	// Bad rays store nothing and name the fault.
	badOrigins := []core.Vec2{{X: math.NaN()}, {Y: math.Inf(1)}}
	for _, p := range badOrigins {
		if _, err := NewRay(p, core.V2(1, 0), 5, 1); core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("bad origin %v code = %v, want invalid-arg", p, core.CodeOf(err))
		}
	}
	for _, d := range []core.Vec2{{}, {X: math.NaN()}, {Y: math.Inf(-1)}} {
		if _, err := NewRay(core.V2(0, 0), d, 5, 1); core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("bad dir %v code = %v, want invalid-arg", d, core.CodeOf(err))
		}
	}
	for _, m := range []float64{-1, math.NaN(), math.Inf(1)} {
		if _, err := NewRay(core.V2(0, 0), core.V2(1, 0), m, 1); core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("bad maxdist %v code = %v, want invalid-arg", m, core.CodeOf(err))
		}
	}
	badRay := Ray{Origin: core.V2(math.NaN(), 0), Dir: core.V2(1, 0), MaxDist: 5, Mask: 1}
	if badRay.Valid() {
		t.Error("bad ray Valid = true, want false")
	}
	if _, _, err := CastRay([]Body{wall}, badRay); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("bad ray cast code = %v, want invalid-arg", core.CodeOf(err))
	}
	// Hand-built bad bodies fail closed with the input untouched.
	bad := Body{Name: "neg-half", Shape: ShapeBox, Pos: core.V2(10, 0), Half: core.V2(-1, 1), Layer: 1, Mask: 1}
	mix := []Body{wall, bad}
	snap := append([]Body(nil), mix...)
	if _, _, err := CastRay(mix, good); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("bad body cast = %v, want invalid-arg", err)
	}
	for i := range mix {
		if !sameBody(mix[i], snap[i]) {
			t.Errorf("bad body cast mutated input")
			break
		}
	}
	// Huge-but-finite inputs never panic.
	huge, _ := NewBox("huge", core.V2(1e308, 0), core.V2(1e308, 1), 1, 1, false)
	hugeRay, _ := NewRay(core.V2(-1e308, 0), core.V2(1, 0), 1e308, 1)
	_, _, _ = CastRay([]Body{huge, wall}, hugeRay)
	_, _, _ = CastRay([]Body{huge}, good)
	if IsStandingOn(huge, wall) && IsStandingOn(wall, huge) {
		t.Error("huge standing both ways, want at most one")
	}
	// Carry bad roads: nil rider, bad bodies, bad delta all fail closed.
	if _, err := CarryRider(nil, wall, core.V2(1, 0)); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil rider code = %v, want invalid-arg", core.CodeOf(err))
	}
	rider := mustFindRayBody(t, bm, "edge", "rider_on_plat")
	plat := mustFindRayBody(t, bm, "edge", "plat_move")
	before := rider
	for _, d := range []core.Vec2{{X: math.NaN()}, {Y: math.Inf(1)}} {
		if _, err := CarryRider(&rider, plat, d); core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("bad delta %v code = %v, want invalid-arg", d, core.CodeOf(err))
		}
	}
	if !sameBody(rider, before) {
		t.Error("bad delta mutated rider")
	}
	badRider := Body{Name: "bad", Shape: Shape(9), Pos: core.V2(0, 3), Half: core.V2(1, 1), Layer: 1, Mask: 1}
	if _, err := CarryRider(&badRider, plat, core.V2(1, 0)); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("bad rider carry = %v, want invalid-arg", err)
	}
	if IsStandingOn(badRider, plat) || IsStandingOn(plat, badRider) {
		t.Error("bad body standing = true, want false")
	}
	// Degenerate same-body standing never panics (point on itself path).
	self, _ := NewBox("self", core.V2(0, 0), core.Vec2{}, 1, 1, false)
	_ = IsStandingOn(self, self)
	_, _ = CarryRider(&self, self, core.Vec2{})
}

// C:纯算数不画画,两边同数靠边界往返无损加逐位重放(C不适用像素,记边界等价).
func TestRayBoundaryIdentical(t *testing.T) {
	f := loadRayCases(t)
	bm := rayBodyMap(t, f)
	rm := rayMap(t, f)
	for name, r := range rm {
		if back := core.Vec2FromRenderPoint(r.Origin.ToRenderPoint()); back != r.Origin {
			t.Errorf("%s origin boundary = %v, want %v", name, back, r.Origin)
		}
		if back := core.Vec2FromRenderPoint(r.Dir.ToRenderPoint()); back != r.Dir {
			t.Errorf("%s dir boundary = %v, want %v", name, back, r.Dir)
		}
	}
	probe, hit, err := CastRay(castBodies(t, bm, mustFindCast(t, f, "cast_wall")), mustFindRay(t, rm, "ray_hit_wall"))
	if err != nil || !hit {
		t.Fatalf("probe cast: %v/%v", probe, err)
	}
	if back := core.Vec2FromRenderPoint(probe.Pos.ToRenderPoint()); back != probe.Pos {
		t.Error("hit pos boundary diverged")
	}
	// Same cast replays hit-for-hit.
	bodies := castBodies(t, bm, mustFindCast(t, f, "cast_trigger_nearest"))
	ray := mustFindRay(t, rm, "ray_trigger")
	snap := append([]Body(nil), bodies...)
	a, ahit, err := CastRay(bodies, ray)
	if err != nil {
		t.Fatalf("trigger cast: %v", err)
	}
	b, bhit, err := CastRay(bodies, ray)
	if err != nil {
		t.Fatalf("trigger replay: %v", err)
	}
	if ahit != bhit || a != b {
		t.Fatalf("replay diverged: %+v vs %+v", a, b)
	}
	for i := range bodies {
		if !sameBody(bodies[i], snap[i]) {
			t.Fatal("cast mutated the input")
		}
	}
	// Standing and carry replay stable.
	rider := mustFindRayBody(t, bm, "replay", "rider_on_plat")
	plat := mustFindRayBody(t, bm, "replay", "plat_move")
	first := IsStandingOn(rider, plat)
	for i := 0; i < 100; i++ {
		if IsStandingOn(rider, plat) != first {
			t.Fatal("standing replay diverged")
		}
	}
	r0 := rider
	c0, err := CarryRider(&r0, plat, core.V2(1, 0))
	if err != nil || !c0 {
		t.Fatalf("carry probe: %v/%v", r0, err)
	}
	r1 := rider
	c1, err := CarryRider(&r1, plat, core.V2(1, 0))
	if err != nil || !c1 || r1 != r0 {
		t.Fatal("carry replay diverged")
	}
}

func mustFindCast(t *testing.T, f rayFile, name string) castDef {
	t.Helper()
	for _, c := range f.Casts {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("body_ray_cases.json has no cast %q", name)
	return castDef{}
}

// D:百盒射线跑得动,耗时调用有数.
func TestRayPerfHundred(t *testing.T) {
	// Synthetic load only (no golden): golden stays in body_ray_cases.json.
	r := core.NewRand(20260915)
	const n = 100
	bodies := make([]Body, n)
	for i := 0; i < n; i++ {
		pos := core.V2(r.RangeFloat(-500, 500), r.RangeFloat(-500, 500))
		b, err := NewBox("p", pos, core.V2(8, 8), 1, 1, i%10 == 0)
		if err != nil {
			t.Fatalf("box %d: %v", i, err)
		}
		bodies[i] = b
	}
	ray, err := NewRay(core.V2(-600, 0), core.V2(1, 0), 1400, 1)
	if err != nil {
		t.Fatalf("perf ray: %v", err)
	}
	const reps = 2000
	var hits int
	start := time.Now()
	for i := 0; i < reps; i++ {
		_, hit, err := CastRay(bodies, ray)
		if err != nil {
			t.Fatalf("rep %d: %v", i, err)
		}
		if hit {
			hits++
		}
	}
	el := time.Since(start)
	t.Logf("ray-100: %d casts x %d boxes in %v (%.1f us/cast, %d hits)", reps, n, el, float64(el.Microseconds())/reps, hits)
}

// E:长跑不穿:万次重放逐位一致,平台带着走不丢不穿.
func TestRayLongRunStable(t *testing.T) {
	f := loadRayCases(t)
	bm := rayBodyMap(t, f)
	rm := rayMap(t, f)
	bodies := castBodies(t, bm, mustFindCast(t, f, "cast_trigger_nearest"))
	ray := mustFindRay(t, rm, "ray_trigger")
	first, fhit, err := CastRay(bodies, ray)
	if err != nil {
		t.Fatalf("trigger cast: %v", err)
	}
	snap := append([]Body(nil), bodies...)
	for i := 0; i < 10000; i++ {
		got, hit, err := CastRay(bodies, ray)
		if err != nil {
			t.Fatalf("rep %d: %v", i, err)
		}
		if hit != fhit || got != first {
			t.Fatalf("rep %d drifted: %+v vs %+v", i, got, first)
		}
	}
	for i := range bodies {
		if !sameBody(bodies[i], snap[i]) {
			t.Fatal("10k casts mutated the input")
		}
	}
	// A rider walks off and back: standing flips and returns, carry keeps
	// contact while aboard and leaves the walker behind once off.
	plat, _ := NewBox("plat", core.V2(0, 0), core.V2(5, 1), 1, 1, false)
	for _, x := range []float64{0, 20, 0} {
		rider, _ := NewBox("hero", core.V2(x, 3), core.V2(2, 2), 1, 1, false)
		want := x == 0
		if got := IsStandingOn(rider, plat); got != want {
			t.Errorf("walk x=%v standing = %v, want %v", x, got, want)
		}
		r := rider
		carried, err := CarryRider(&r, plat, core.V2(2, 0))
		if err != nil {
			t.Fatalf("walk x=%v carry: %v", x, err)
		}
		if carried != want {
			t.Errorf("walk x=%v carried = %v, want %v", x, carried, want)
		}
		if want && r.Pos != core.V2(x+2, 3) {
			t.Errorf("walk x=%v carried pos = %v, want %v", x, r.Pos, core.V2(x+2, 3))
		}
		if !want && r.Pos != rider.Pos {
			t.Errorf("walk x=%v off-board rider moved", x)
		}
	}
	// Platform chain: 10k identical steps keep the rider grounded.
	prider, _ := NewBox("rider", core.V2(0, 3), core.V2(2, 2), 1, 1, false)
	pplat, _ := NewBox("pplat", core.V2(0, 0), core.V2(5, 1), 1, 1, false)
	step := core.V2(0.1, 0)
	for i := 0; i < 10000; i++ {
		carried, err := CarryRider(&prider, pplat, step)
		if err != nil || !carried {
			t.Fatalf("chain step %d: %v/%v", i, prider, err)
		}
		pplat.Pos = pplat.Pos.Add(step)
		if !IsStandingOn(prider, pplat) {
			t.Fatalf("chain step %d lost contact", i)
		}
	}
	if prider.Pos.X < 999 || prider.Pos.X > 1001 {
		t.Errorf("chain rider x = %v, want ~1000", prider.Pos.X)
	}
}

// F:离屏金对照窗(W2免窗,game_physics--case=hit后建):冻结数加形状断言.
func TestRayOffscreenGolden(t *testing.T) {
	f := loadRayCases(t)
	bm := rayBodyMap(t, f)
	rm := rayMap(t, f)
	// Golden numbers stay frozen.
	for _, c := range f.Casts {
		got, hit, err := CastRay(castBodies(t, bm, c), mustFindRay(t, rm, c.Ray))
		if err != nil {
			t.Fatalf("golden %s: %v", c.Name, err)
		}
		if hit != c.WantHit {
			t.Fatalf("golden %s hit = %v, want %v", c.Name, hit, c.WantHit)
		}
		if !hit {
			continue
		}
		if got.Name != c.WantBody || got.Dist != c.WantDist || got.Trigger != c.WantTrig {
			t.Fatalf("golden %s = %+v, want %s/%v/trig=%v", c.Name, got, c.WantBody, c.WantDist, c.WantTrig)
		}
		if got.Pos != core.V2(c.WantPos[0], c.WantPos[1]) || got.Normal != core.V2(c.WantNormal[0], c.WantNormal[1]) {
			t.Fatalf("golden %s pos/normal = %v/%v", c.Name, got.Pos, got.Normal)
		}
	}
	for _, s := range f.Stands {
		if got := IsStandingOn(bm[s.Rider], bm[s.Platform]); got != s.Want {
			t.Fatalf("golden %s standing = %v, want %v", s.Name, got, s.Want)
		}
	}
	for _, c := range f.Carrys {
		r := bm[c.Rider]
		carried, err := CarryRider(&r, bm[c.Platform], core.V2(c.Delta[0], c.Delta[1]))
		if err != nil || carried != c.WantCarried || r.Pos != core.V2(c.WantPos[0], c.WantPos[1]) {
			t.Fatalf("golden %s carry = %v/%v/%v", c.Name, r.Pos, carried, err)
		}
	}
	// Shape: edge touch counts, a hair gap does not.
	_, edgeHit, _ := CastRay([]Body{bm["wall_box"]}, rm["ray_edge"])
	if !edgeHit {
		t.Error("edge ray = miss, want hit (edge counts)")
	}
	if IsStandingOn(bm["rider_gap"], bm["plat_move"]) {
		t.Error("0.5 gap standing = true, want false")
	}
	if !IsStandingOn(bm["rider_stand"], bm["floor_box"]) {
		t.Error("exact touch standing = false, want true")
	}
	// Shape: side touch is overlap but not standing; below never stands.
	if !Overlaps(bm["rider_side"], bm["plat_move"]) {
		t.Error("side geometry = miss, want overlap")
	}
	if IsStandingOn(bm["rider_side"], bm["plat_move"]) {
		t.Error("side standing = true, want false")
	}
	if IsStandingOn(bm["floor_box"], bm["rider_stand"]) {
		t.Error("below standing = true, want false")
	}
	// Shape: geometry says touch, masks say no: cast stays silent.
	if _, hit, _ := CastRay([]Body{bm["ghost_box"]}, rm["ray_masked"]); hit {
		t.Error("masked cast = hit, want miss")
	}
	// Shape: trigger is reported with flag, nearest wins either order.
	trig, hit, _ := CastRay([]Body{bm["trigger_zone"], bm["wall_box"]}, rm["ray_trigger"])
	if !hit || !trig.Trigger || trig.Name != "trigger_zone" {
		t.Errorf("trigger nearest = %+v/%v, want flagged trigger_zone", trig, hit)
	}
	swap, _, _ := CastRay([]Body{bm["wall_box"], bm["trigger_zone"]}, rm["ray_trigger"])
	if swap.Name != trig.Name || swap.Dist != trig.Dist || swap.Pos != trig.Pos || swap.Normal != trig.Normal || swap.Trigger != trig.Trigger {
		t.Error("order changed nearest, want index-stable nearest")
	}
	// Shape: tilemap solid cell feeds ray as a plain box (7.1 wiring).
	tile, hit, _ := CastRay([]Body{bm["tile_solid_10"]}, rm["ray_tile_probe"])
	if !hit || tile.Dist != 16 || tile.Pos != core.V2(16, 8) {
		t.Errorf("tile probe = %+v/%v, want dist 16 pos (16,8)", tile, hit)
	}
	// Shape: non-normalized dir never scales the distance.
	a, _, _ := CastRay([]Body{bm["wall_box"]}, rm["ray_hit_wall"])
	b, _, _ := CastRay([]Body{bm["wall_box"]}, rm["ray_hit_wall_nonnorm"])
	if a != b {
		t.Error("nonnorm dir changed hit, want same numbers")
	}
	// Shape: zero range only touches from inside; down probe faces up.
	if _, hit, _ := CastRay([]Body{bm["wall_box"]}, rm["ray_zero_range"]); hit {
		t.Error("zero range outside = hit, want miss")
	}
	down, _, _ := CastRay([]Body{bm["floor_box"]}, rm["ray_down_probe"])
	if down.Normal != core.V2(0, 1) || down.Dist != 9 {
		t.Errorf("down probe = %+v, want dist 9 normal (0,1)", down)
	}
}
