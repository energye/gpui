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

const epsPlatform = 1e-9

type slopeDef struct {
	Name   string     `json:"name"`
	A      [2]float64 `json:"a"`
	B      [2]float64 `json:"b"`
	OneWay bool       `json:"one_way"`
}

type groundDef struct {
	Slope string  `json:"slope"`
	X     float64 `json:"x"`
	WantY float64 `json:"want_y"`
	OK    bool    `json:"ok"`
}

type angleDef struct {
	Slope   string  `json:"slope"`
	WantRad float64 `json:"want_rad"`
}

type walkDef struct {
	Slope  string  `json:"slope"`
	MaxRad float64 `json:"max_rad"`
	Want   bool    `json:"want"`
}

type slideDef struct {
	Slope string     `json:"slope"`
	Want  [2]float64 `json:"want"`
	OK    bool       `json:"ok"`
}

type feetDef struct {
	Shape  string     `json:"shape"`
	Pos    [2]float64 `json:"pos"`
	Half   [2]float64 `json:"half"`
	Radius float64    `json:"radius"`
	Want   [2]float64 `json:"want"`
	OK     bool       `json:"ok"`
}

type groundedDef struct {
	Feet [2]float64 `json:"feet"`
	Want string     `json:"want"`
	OK   bool       `json:"ok"`
}

type stepDef struct {
	Name         string     `json:"name"`
	Slopes       []string   `json:"slopes"`
	Feet         [2]float64 `json:"feet"`
	Vel          [2]float64 `json:"vel"`
	Dt           float64    `json:"dt"`
	WantPos      [2]float64 `json:"want_pos"`
	WantGround   string     `json:"want_ground"`
	WantGrounded bool       `json:"want_grounded"`
	WantHead     bool       `json:"want_head"`
}

type platformFile struct {
	Slopes   []slopeDef    `json:"slopes"`
	Grounds  []groundDef   `json:"grounds"`
	Angles   []angleDef    `json:"angles"`
	Walkable []walkDef     `json:"walkable"`
	Slides   []slideDef    `json:"slides"`
	Feet     []feetDef     `json:"feet"`
	Grounded []groundedDef `json:"grounded"`
	Steps    []stepDef     `json:"steps"`
}

func loadPlatformCases(t *testing.T) platformFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "platform_cases.json"))
	if err != nil {
		t.Fatalf("read platform_cases.json: %v", err)
	}
	var f platformFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode platform_cases.json: %v", err)
	}
	if len(f.Slopes) == 0 || len(f.Grounds) == 0 || len(f.Steps) == 0 {
		t.Fatal("platform_cases.json has no slopes, grounds, or steps")
	}
	return f
}

func buildSlope(t *testing.T, d slopeDef) Slope {
	t.Helper()
	a := core.V2(d.A[0], d.A[1])
	b := core.V2(d.B[0], d.B[1])
	if d.OneWay {
		s, err := NewOneWay(d.Name, a, b)
		if err != nil {
			t.Fatalf("NewOneWay %q: %v", d.Name, err)
		}
		return s
	}
	s, err := NewSlope(d.Name, a, b)
	if err != nil {
		t.Fatalf("NewSlope %q: %v", d.Name, err)
	}
	return s
}

func slopeMap(t *testing.T, f platformFile) map[string]Slope {
	t.Helper()
	m := map[string]Slope{}
	for _, d := range f.Slopes {
		if _, dup := m[d.Name]; dup {
			t.Fatalf("duplicate slope %q", d.Name)
		}
		m[d.Name] = buildSlope(t, d)
	}
	return m
}

func mustFindSlope(t *testing.T, m map[string]Slope, name string) Slope {
	t.Helper()
	s, ok := m[name]
	if !ok {
		t.Fatalf("unknown slope %q", name)
	}
	return s
}

func mustFindStep(t *testing.T, f platformFile, name string) stepDef {
	t.Helper()
	for _, s := range f.Steps {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("platform_cases.json has no step %q", name)
	return stepDef{}
}

func stepSlopes(t *testing.T, m map[string]Slope, d stepDef) []Slope {
	t.Helper()
	out := make([]Slope, 0, len(d.Slopes))
	for _, n := range d.Slopes {
		out = append(out, mustFindSlope(t, m, n))
	}
	return out
}

func closeFloat(got, want float64) bool {
	return math.Abs(got-want) <= epsPlatform
}

func platSameFloat(a, b float64) bool {
	if math.IsNaN(a) && math.IsNaN(b) {
		return true
	}
	return a == b
}

func sameVec(a, b core.Vec2) bool {
	return platSameFloat(a.X, b.X) && platSameFloat(a.Y, b.Y)
}

func closeVec(got core.Vec2, want [2]float64) bool {
	return closeFloat(got.X, want[0]) && closeFloat(got.Y, want[1])
}

// A:站滑跳单向上跳对:地面高度加角度加可站加滑向加脚底加贴地加单步全落在冻结数上.
func TestPlatformFromCases(t *testing.T) {
	f := loadPlatformCases(t)
	m := slopeMap(t, f)
	for _, g := range f.Grounds {
		s := mustFindSlope(t, m, g.Slope)
		got, ok := s.GroundYAt(g.X)
		if ok != g.OK || (ok && !closeFloat(got, g.WantY)) {
			t.Errorf("ground %s x=%v = %v/%v, want %v/%v", g.Slope, g.X, got, ok, g.WantY, g.OK)
		}
	}
	for _, a := range f.Angles {
		s := mustFindSlope(t, m, a.Slope)
		got, ok := s.Angle()
		if !ok || !closeFloat(got, a.WantRad) {
			t.Errorf("angle %s = %v/%v, want %v/true", a.Slope, got, ok, a.WantRad)
		}
	}
	for _, w := range f.Walkable {
		s := mustFindSlope(t, m, w.Slope)
		if got := s.Walkable(w.MaxRad); got != w.Want {
			t.Errorf("walkable %s max=%v = %v, want %v", w.Slope, w.MaxRad, got, w.Want)
		}
	}
	for _, s := range f.Slides {
		sl := mustFindSlope(t, m, s.Slope)
		got, ok := sl.SlideDir()
		if ok != s.OK {
			t.Errorf("slide %s ok = %v, want %v", s.Slope, ok, s.OK)
			continue
		}
		if ok && !closeVec(got, s.Want) {
			t.Errorf("slide %s = %v, want %v", s.Slope, got, s.Want)
		}
	}
	for _, fd := range f.Feet {
		var b Body
		var err error
		pos := core.V2(fd.Pos[0], fd.Pos[1])
		switch fd.Shape {
		case "box":
			b, err = NewBox("feet", pos, core.V2(fd.Half[0], fd.Half[1]), 1, 1, false)
		case "circle":
			b, err = NewCircle("feet", pos, fd.Radius, 1, 1, false)
		default:
			t.Fatalf("unknown feet shape %q", fd.Shape)
		}
		if err != nil {
			t.Fatalf("feet body: %v", err)
		}
		got, ok := FeetOf(b)
		if ok != fd.OK || (ok && !closeVec(got, fd.Want)) {
			t.Errorf("feet %v = %v/%v, want %v/%v", fd.Pos, got, ok, fd.Want, fd.OK)
			continue
		}
		if ok {
			back, ok := WithFeet(b, got)
			if !ok || back.Pos != b.Pos {
				t.Errorf("feet round trip = %v/%v, want %v/true", back.Pos, ok, b.Pos)
			}
		}
	}
	for _, g := range f.Grounded {
		feet := core.V2(g.Feet[0], g.Feet[1])
		var all []Slope
		for _, d := range f.Slopes {
			all = append(all, m[d.Name])
		}
		got, ok := Grounded(feet, all)
		if ok != g.OK {
			t.Errorf("grounded %v ok = %v, want %v", g.Feet, ok, g.OK)
			continue
		}
		if ok && got.Name() != g.Want {
			t.Errorf("grounded %v = %q, want %q", g.Feet, got.Name(), g.Want)
		}
	}
	for _, sd := range f.Steps {
		slopes := stepSlopes(t, m, sd)
		feet := core.V2(sd.Feet[0], sd.Feet[1])
		vel := core.V2(sd.Vel[0], sd.Vel[1])
		snap := append([]Slope(nil), slopes...)
		pos, ground, grounded, head, err := Step(feet, vel, sd.Dt, slopes)
		if err != nil {
			t.Errorf("step %s: %v", sd.Name, err)
			continue
		}
		if !closeVec(pos, sd.WantPos) {
			t.Errorf("step %s pos = %v, want %v", sd.Name, pos, sd.WantPos)
		}
		if grounded != sd.WantGrounded || head != sd.WantHead {
			t.Errorf("step %s grounded/head = %v/%v, want %v/%v", sd.Name, grounded, head, sd.WantGrounded, sd.WantHead)
		}
		if grounded && ground.Name() != sd.WantGround {
			t.Errorf("step %s ground = %q, want %q", sd.Name, ground.Name(), sd.WantGround)
		}
		if !grounded && sd.WantGround != "" {
			t.Errorf("step %s ground = %q with grounded=false, want empty", sd.Name, ground.Name())
		}
		for i := range slopes {
			if slopes[i] != snap[i] {
				t.Errorf("step %s mutated slopes", sd.Name)
				break
			}
		}
	}
}

// B:卡角空零超大坏数据全不崩不卡死,占位加报错.
func TestPlatformEdgesNoCrash(t *testing.T) {
	f := loadPlatformCases(t)
	m := slopeMap(t, f)
	flat := mustFindSlope(t, m, "flat")
	wall := mustFindSlope(t, m, "wall")

	// Empty and nil slope lists sample nothing, never an error.
	if _, ok := Grounded(core.V2(50, 100), nil); ok {
		t.Error("nil slopes grounded = true, want false")
	}
	if _, ok := Grounded(core.V2(50, 100), []Slope{}); ok {
		t.Error("empty slopes grounded = true, want false")
	}
	pos, _, grounded, head, err := Step(core.V2(50, 90), core.V2(0, 100), 0.1, nil)
	if err != nil || grounded || head {
		t.Errorf("nil slopes step = %v/%v/%v/%v, want move airborne", pos, grounded, head, err)
	}
	if !closeVec(pos, [2]float64{50, 100}) {
		t.Errorf("nil slopes pos = %v, want [50 100]", pos)
	}

	// Corner: flat end meets the wall column; wall never claims a height.
	if _, ok := wall.GroundYAt(50); ok {
		t.Error("wall GroundYAt = ok, want false (vertical holds no height)")
	}
	if _, ok := wall.GroundYAt(0); ok {
		t.Error("wall off-column = ok, want false")
	}
	if _, ok := flat.GroundYAt(101); ok {
		t.Error("flat past end = ok, want false")
	}
	if _, ok := Grounded(core.V2(50, 50), []Slope{flat, wall}); ok {
		t.Error("mid-air corner grounded = true, want false")
	}
	if pos, g, ok, head, err := Step(core.V2(50, 100), core.V2(0, 0), 0.016, []Slope{flat, wall}); err != nil || !ok || head || g.Name() != "flat" {
		t.Errorf("corner stand = %v/%q/%v/%v/%v, want flat grounded", pos, g.Name(), ok, head, err)
	}

	// Bad constructors store nothing and name the fault.
	badPts := []core.Vec2{{X: math.NaN()}, {Y: math.Inf(1)}, {X: math.Inf(-1), Y: 1}}
	for _, p := range badPts {
		if _, err := NewSlope("bad", p, core.V2(1, 1)); core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("bad slope pos %v code = %v, want invalid-arg", p, core.CodeOf(err))
		}
		if _, err := NewOneWay("bad", p, core.V2(1, 1)); core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("bad oneway pos %v code = %v, want invalid-arg", p, core.CodeOf(err))
		}
	}
	if _, err := NewSlope("zero", core.V2(1, 1), core.V2(1, 1)); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("zero slope code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := NewOneWay("zero", core.V2(2, 3), core.V2(2, 3)); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("zero oneway code = %v, want invalid-arg", core.CodeOf(err))
	}
	zero := Slope{}
	if zero.Valid() {
		t.Error("zero slope Valid = true, want false")
	}
	if _, ok := zero.GroundYAt(0); ok {
		t.Error("zero GroundYAt = ok, want false")
	}
	if _, ok := zero.Angle(); ok {
		t.Error("zero Angle = ok, want false")
	}
	if zero.Walkable(1) {
		t.Error("zero Walkable = true, want false")
	}
	if _, ok := zero.SlideDir(); ok {
		t.Error("zero SlideDir = ok, want false")
	}

	// Hand-built bad slopes fail closed in Step, soft in Grounded.
	// Zero (a==b) is invalid but comparable, so the mutation check below
	// stays exact; NaN bodies need NaN-aware compare (see sameVec).
	bad := Slope{}
	if bad.Valid() {
		t.Error("zero slope Valid = true, want false")
	}
	badNaN := Slope{name: "nan", a: core.V2(math.NaN(), 0), b: core.V2(1, 1)}
	if badNaN.Valid() {
		t.Error("nan slope Valid = true, want false")
	}
	if _, ok := Grounded(core.V2(50, 100), []Slope{bad, flat}); !ok {
		t.Error("Grounded with one bad slope skipped everything, want flat still sampled")
	}
	snap := []Slope{flat, bad}
	keep := append([]Slope(nil), snap...)
	if _, _, _, _, err := Step(core.V2(50, 90), core.V2(0, 100), 0.1, snap); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("bad slope Step code = %v, want invalid-arg", core.CodeOf(err))
	}
	for i := range snap {
		if snap[i] != keep[i] {
			t.Error("bad Step mutated slopes")
			break
		}
	}

	// Bad motion inputs fail closed with feet unchanged, never guessed.
	feet := core.V2(50, 90)
	for _, tc := range []struct {
		name string
		feet core.Vec2
		vel  core.Vec2
		dt   float64
	}{
		{"nan-feet", core.V2(math.NaN(), 0), core.V2(0, 1), 0.1},
		{"nan-vel", feet, core.Vec2{X: math.NaN()}, 0.1},
		{"inf-dt", feet, core.V2(0, 1), math.Inf(1)},
		{"neg-dt", feet, core.V2(0, 1), -0.1},
	} {
		if got, _, _, _, err := Step(tc.feet, tc.vel, tc.dt, []Slope{flat}); core.CodeOf(err) != core.CodeInvalidArg || !sameVec(got, tc.feet) {
			t.Errorf("%s Step = %v/%v, want feet unchanged + invalid-arg", tc.name, got, err)
		}
	}

	// Bad angles and bodies never walk or stand.
	if flat.Walkable(math.NaN()) || flat.Walkable(-1) || flat.Walkable(math.Inf(1)) {
		t.Error("bad maxAngle Walkable = true, want false")
	}
	weird := Body{Name: "weird", Shape: Shape(9), Pos: core.V2(0, 0), Half: core.V2(1, 1), Layer: 1, Mask: 1}
	if weird.Valid() {
		t.Error("weird body Valid = true, want false")
	}
	if _, ok := FeetOf(weird); ok {
		t.Error("invalid body FeetOf = ok, want false")
	}
	if _, ok := WithFeet(weird, core.V2(0, 0)); ok {
		t.Error("invalid body WithFeet = ok, want false")
	}
	if _, ok := WithFeet(flatValidBox(), core.V2(math.NaN(), 0)); ok {
		t.Error("nan feet WithFeet = ok, want false")
	}

	// Huge-but-finite inputs never panic.
	huge, _ := NewSlope("huge", core.V2(1e308, 1e308), core.V2(-1e308, -1e308))
	_, _ = huge.GroundYAt(0)
	_, _ = huge.Angle()
	_ = huge.Walkable(1)
	_, _ = huge.SlideDir()
	_, _ = Grounded(core.V2(1e308, 1e308), []Slope{huge})
	_, _, _, _, _ = Step(core.V2(0, 0), core.V2(1e308, 1e308), 0.001, []Slope{flat})
}

func flatValidBox() Body {
	b, _ := NewBox("box", core.V2(0, 0), core.V2(5, 5), 1, 1, false)
	return b
}

// C:纯算数不画画,两边同数靠边界往返无损加逐位重放(C不适用像素,记边界等价).
func TestPlatformBoundaryIdentical(t *testing.T) {
	f := loadPlatformCases(t)
	m := slopeMap(t, f)
	// Render boundary is lossless for every frozen endpoint and foot.
	for name, s := range m {
		for _, p := range []core.Vec2{s.A(), s.B()} {
			if back := core.Vec2FromRenderPoint(p.ToRenderPoint()); back != p {
				t.Errorf("%s boundary = %v, want %v", name, back, p)
			}
		}
	}
	for _, g := range f.Grounded {
		feet := core.V2(g.Feet[0], g.Feet[1])
		if back := core.Vec2FromRenderPoint(feet.ToRenderPoint()); back != feet {
			t.Errorf("feet %v boundary diverged", g.Feet)
		}
	}
	// Same step replays position-for-position, flag-for-flag.
	sd := mustFindStep(t, f, "fall_flat")
	slopes := stepSlopes(t, m, sd)
	feet := core.V2(sd.Feet[0], sd.Feet[1])
	vel := core.V2(sd.Vel[0], sd.Vel[1])
	snap := append([]Slope(nil), slopes...)
	aPos, aGround, aGrounded, aHead, err := Step(feet, vel, sd.Dt, slopes)
	if err != nil {
		t.Fatalf("fall_flat: %v", err)
	}
	bPos, bGround, bGrounded, bHead, err := Step(feet, vel, sd.Dt, slopes)
	if err != nil {
		t.Fatalf("fall_flat replay: %v", err)
	}
	if aPos != bPos || aGround != bGround || aGrounded != bGrounded || aHead != bHead {
		t.Fatalf("replay diverged: %v/%v vs %v/%v", aPos, aGrounded, bPos, bGrounded)
	}
	for i := range slopes {
		if slopes[i] != snap[i] {
			t.Fatal("step mutated the input")
		}
	}
	// dt==0 only samples the standing pose, never the head path.
	pos, g, grounded, head, err := Step(feet, vel, 0, slopes)
	if err != nil || head || !closeVec(pos, sd.Feet) {
		t.Errorf("dt=0 = %v/%v/%v/%v, want feet held, no head", pos, g.Name(), grounded, err)
	}
}

// D:长坡查询跑得动,耗时调用有数.
func TestPlatformPerfLongSlope(t *testing.T) {
	// Synthetic load only (no golden): golden stays in platform_cases.json.
	long, err := NewSlope("long", core.V2(0, 0), core.V2(10000, -1000))
	if err != nil {
		t.Fatalf("long: %v", err)
	}
	const reps = 5000
	start := time.Now()
	var sum float64
	for i := 0; i < reps; i++ {
		x := float64(i%10001) + 0.5
		y, ok := long.GroundYAt(x)
		if !ok {
			t.Fatalf("rep %d GroundYAt ok=false", i)
		}
		sum += y
	}
	const steps = 2000
	landed := 0
	for i := 0; i < steps; i++ {
		x := float64(i%10000) + 0.25
		feet := core.V2(x, -200)
		vel := core.V2(60, 300)
		if pos, _, grounded, _, err := Step(feet, vel, 0.016, []Slope{long}); err != nil {
			t.Fatalf("rep %d Step: %v", i, err)
		} else {
			sum += pos.X + pos.Y
			if grounded {
				landed++
			}
		}
	}
	el := time.Since(start)
	t.Logf("platform-long: %d grounds + %d steps on 10k slope in %v (%.1f us/op, sum %.1f, landed %d)", reps, steps, el, float64(el.Microseconds())/float64(reps+steps), sum, landed)
}

// E:长跑不掉:万次重放逐位一致,走格往返不粘.
func TestPlatformLongRunStable(t *testing.T) {
	f := loadPlatformCases(t)
	m := slopeMap(t, f)
	sd := mustFindStep(t, f, "stand_flat")
	slopes := stepSlopes(t, m, sd)
	feet := core.V2(sd.Feet[0], sd.Feet[1])
	vel := core.V2(sd.Vel[0], sd.Vel[1])
	firstPos, firstGround, firstGrounded, firstHead, err := Step(feet, vel, sd.Dt, slopes)
	if err != nil {
		t.Fatalf("stand_flat: %v", err)
	}
	snap := append([]Slope(nil), slopes...)
	for i := 0; i < 10000; i++ {
		pos, ground, grounded, head, err := Step(feet, vel, sd.Dt, slopes)
		if err != nil {
			t.Fatalf("rep %d: %v", i, err)
		}
		if pos != firstPos || ground != firstGround || grounded != firstGrounded || head != firstHead {
			t.Fatalf("rep %d drifted: %v vs %v", i, pos, firstPos)
		}
	}
	for i := range slopes {
		if slopes[i] != snap[i] {
			t.Fatal("10k steps mutated the input")
		}
	}
	// Walk right along the flat and back: grounded holds, never sticks.
	flat := mustFindSlope(t, m, "flat")
	right, _, ok, _, err := Step(core.V2(0, 100), core.V2(100, 0), 0.5, []Slope{flat})
	if err != nil || !ok || !closeVec(right, [2]float64{50, 100}) {
		t.Fatalf("walk right = %v/%v/%v, want [50 100] grounded", right, ok, err)
	}
	back, _, ok, _, err := Step(right, core.V2(-100, 0), 0.5, []Slope{flat})
	if err != nil || !ok || !closeVec(back, [2]float64{0, 100}) {
		t.Fatalf("walk back = %v/%v/%v, want [0 100] grounded", back, ok, err)
	}
	// Jump up through the one-way and fall back: pass once, land once.
	oneway := mustFindSlope(t, m, "oneway")
	up, _, grounded, head, err := Step(core.V2(50, 90), core.V2(0, -200), 0.1, []Slope{oneway})
	if err != nil || grounded || head || !closeVec(up, [2]float64{50, 70}) {
		t.Fatalf("jump pass = %v/%v/%v/%v, want [50 70] airborne", up, grounded, head, err)
	}
	down, _, grounded, head, err := Step(up, core.V2(0, 100), 0.1, []Slope{oneway})
	if err != nil || !grounded || head || !closeVec(down, [2]float64{50, 80}) {
		t.Fatalf("fall land = %v/%v/%v/%v, want [50 80] grounded", down, grounded, head, err)
	}
}

// F:离屏金对照窗(W2先离屏,game_physics --case=jump后建):冻结数加形状断言.
func TestPlatformOffscreenGolden(t *testing.T) {
	f := loadPlatformCases(t)
	m := slopeMap(t, f)
	// Golden numbers stay frozen.
	for _, g := range f.Grounds {
		if got, ok := mustFindSlope(t, m, g.Slope).GroundYAt(g.X); ok != g.OK || (ok && !closeFloat(got, g.WantY)) {
			t.Fatalf("golden ground %s x=%v = %v/%v, want %v/%v", g.Slope, g.X, got, ok, g.WantY, g.OK)
		}
	}
	// Shape: endpoints count, a hair past the end does not.
	flat := mustFindSlope(t, m, "flat")
	if _, ok := flat.GroundYAt(0); !ok {
		t.Error("flat x=0 = miss, want hit (end counts)")
	}
	if _, ok := flat.GroundYAt(100); !ok {
		t.Error("flat x=100 = miss, want hit (end counts)")
	}
	if _, ok := flat.GroundYAt(100.5); ok {
		t.Error("flat x=100.5 = hit, want miss")
	}
	// Shape: flat is level, wall is vertical, ramps sit between.
	if a, _ := flat.Angle(); a != 0 {
		t.Errorf("flat angle = %v, want 0", a)
	}
	if a, _ := mustFindSlope(t, m, "wall").Angle(); a != math.Pi/2 {
		t.Errorf("wall angle = %v, want pi/2", a)
	}
	rampA, _ := mustFindSlope(t, m, "ramp").Angle()
	steepA, _ := mustFindSlope(t, m, "steep").Angle()
	if !(rampA > 0 && rampA < math.Pi/4 && steepA > math.Pi/4 && steepA < math.Pi/2) {
		t.Errorf("ramp/steep angles = %v/%v, want gentle <45 < steep <90", rampA, steepA)
	}
	// Shape: gentle stands, steep slides; slide always points down (+Y).
	if !mustFindSlope(t, m, "ramp").Walkable(math.Pi / 4) {
		t.Error("ramp 45 walkable = false, want true")
	}
	if mustFindSlope(t, m, "steep").Walkable(math.Pi / 4) {
		t.Error("steep 45 walkable = true, want false")
	}
	for _, n := range []string{"ramp", "steep", "wall"} {
		d, ok := mustFindSlope(t, m, n).SlideDir()
		if !ok || d.Y <= 0 {
			t.Errorf("%s slide = %v/%v, want downhill +Y", n, d, ok)
		}
	}
	if _, ok := flat.SlideDir(); ok {
		t.Error("flat slide = ok, want false (level has no downhill)")
	}
	// Shape: one-way lets the jump through, solid reports the bump.
	oneway := mustFindSlope(t, m, "oneway")
	_, _, _, head, _ := Step(core.V2(50, 90), core.V2(0, -200), 0.1, []Slope{oneway})
	if head {
		t.Error("oneway jump head = true, want false (single up passes)")
	}
	_, _, _, head, _ = Step(core.V2(50, 110), core.V2(0, -200), 0.1, []Slope{flat})
	if !head {
		t.Error("solid jump head = false, want true")
	}
	// Shape: falling lands on the highest surface first.
	up := []Slope{oneway, flat}
	if _, g, ok, _, _ := Step(core.V2(50, 70), core.V2(0, 500), 0.1, up); !ok || g.Name() != "oneway" {
		t.Errorf("stack fall = %q/%v, want oneway first", g.Name(), ok)
	}
	// Mixed golden steps replay verbatim.
	for _, sd := range f.Steps {
		pos, ground, grounded, head, err := Step(core.V2(sd.Feet[0], sd.Feet[1]), core.V2(sd.Vel[0], sd.Vel[1]), sd.Dt, stepSlopes(t, m, sd))
		if err != nil {
			t.Fatalf("golden step %s: %v", sd.Name, err)
		}
		if !closeVec(pos, sd.WantPos) || grounded != sd.WantGrounded || head != sd.WantHead {
			t.Fatalf("golden step %s = %v/%v/%v, want %v/%v/%v", sd.Name, pos, grounded, head, sd.WantPos, sd.WantGrounded, sd.WantHead)
		}
		if grounded && ground.Name() != sd.WantGround {
			t.Fatalf("golden step %s ground = %q, want %q", sd.Name, ground.Name(), sd.WantGround)
		}
	}
}
