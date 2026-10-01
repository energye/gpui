//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package physics

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/engine/core"
	"github.com/energye/gpui/engine/renderconv"
)

const epsSnap = 1e-9

type snapCfgDef struct {
	Name   string  `json:"name"`
	Mode   string  `json:"mode"`
	Snap   float64 `json:"snap"`
	MaxRad float64 `json:"max_rad"`
	Margin float64 `json:"margin"`
	Valid  bool    `json:"valid"`
}

type snapSlopeDef struct {
	Name   string     `json:"name"`
	A      [2]float64 `json:"a"`
	B      [2]float64 `json:"b"`
	OneWay bool       `json:"one_way"`
}

type snapNormalDef struct {
	Slope string     `json:"slope"`
	Want  [2]float64 `json:"want"`
	OK    bool       `json:"ok"`
}

type snapGroundDef struct {
	Name string     `json:"name"`
	Cfg  string     `json:"cfg"`
	Feet [2]float64 `json:"feet"`
	Want string     `json:"want"`
	OK   bool       `json:"ok"`
}

type snapStepDef struct {
	Name       string     `json:"name"`
	Cfg        string     `json:"cfg"`
	Slopes     []string   `json:"slopes"`
	Feet       [2]float64 `json:"feet"`
	Vel        [2]float64 `json:"vel"`
	Dt         float64    `json:"dt"`
	WantPos    [2]float64 `json:"want_pos"`
	WantGround string     `json:"want_ground"`
	WantFloor  bool       `json:"want_floor"`
	WantHead   bool       `json:"want_head"`
}

type snapDefaultsDef struct {
	Snap   float64 `json:"snap_length"`
	MaxRad float64 `json:"floor_max_angle_rad"`
	Margin float64 `json:"safe_margin"`
	Mode   string  `json:"mode"`
}

type snapFile struct {
	Defaults snapDefaultsDef `json:"defaults"`
	Configs  []snapCfgDef    `json:"configs"`
	Slopes   []snapSlopeDef  `json:"slopes"`
	Normals  []snapNormalDef `json:"normals"`
	Grounded []snapGroundDef `json:"grounded"`
	Steps    []snapStepDef   `json:"steps"`
}

func loadSnapCases(t *testing.T) snapFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "snap_mode_cases.json"))
	if err != nil {
		t.Fatalf("read snap_mode_cases.json: %v", err)
	}
	var f snapFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode snap_mode_cases.json: %v", err)
	}
	if len(f.Configs) == 0 || len(f.Slopes) == 0 || len(f.Steps) == 0 {
		t.Fatal("snap_mode_cases.json has no configs, slopes, or steps")
	}
	return f
}

func parseSnapMode(t *testing.T, s string) MotionMode {
	t.Helper()
	switch s {
	case "grounded":
		return MotionModeGrounded
	case "floating":
		return MotionModeFloating
	default:
		return MotionMode(-1)
	}
}

func buildSnapCfg(t *testing.T, d snapCfgDef) SnapConfig {
	t.Helper()
	return SnapConfig{
		Mode:          parseSnapMode(t, d.Mode),
		SnapLength:    d.Snap,
		FloorMaxAngle: d.MaxRad,
		SafeMargin:    d.Margin,
	}
}

func snapCfgMap(t *testing.T, f snapFile) map[string]SnapConfig {
	t.Helper()
	m := map[string]SnapConfig{}
	for _, d := range f.Configs {
		if _, dup := m[d.Name]; dup {
			t.Fatalf("duplicate snap config %q", d.Name)
		}
		m[d.Name] = buildSnapCfg(t, d)
	}
	return m
}

func snapSlopeMap(t *testing.T, f snapFile) map[string]Slope {
	t.Helper()
	m := map[string]Slope{}
	for _, d := range f.Slopes {
		if _, dup := m[d.Name]; dup {
			t.Fatalf("duplicate snap slope %q", d.Name)
		}
		a := core.V2(d.A[0], d.A[1])
		b := core.V2(d.B[0], d.B[1])
		var (
			s   Slope
			err error
		)
		if d.OneWay {
			s, err = NewOneWay(d.Name, a, b)
		} else {
			s, err = NewSlope(d.Name, a, b)
		}
		if err != nil {
			t.Fatalf("snap slope %q: %v", d.Name, err)
		}
		m[d.Name] = s
	}
	return m
}

func mustSnapCfg(t *testing.T, m map[string]SnapConfig, name string) SnapConfig {
	t.Helper()
	c, ok := m[name]
	if !ok {
		t.Fatalf("unknown snap config %q", name)
	}
	return c
}

func mustSnapSlope(t *testing.T, m map[string]Slope, name string) Slope {
	t.Helper()
	s, ok := m[name]
	if !ok {
		t.Fatalf("unknown snap slope %q", name)
	}
	return s
}

func snapClose(got, want float64) bool {
	return math.Abs(got-want) <= epsSnap
}

func snapCloseVec(got core.Vec2, want [2]float64) bool {
	return snapClose(got.X, want[0]) && snapClose(got.Y, want[1])
}

// snapSameVec compares two vectors NaN-aware: NaN never equals itself,
// so a plain != would cry mutation on untouched NaN inputs.
func snapSameVec(a, b core.Vec2) bool {
	return snapSameFloat(a.X, b.X) && snapSameFloat(a.Y, b.Y)
}

func snapSameFloat(a, b float64) bool {
	if math.IsNaN(a) && math.IsNaN(b) {
		return true
	}
	return a == b
}

// A:贴地两档对:默认值加两档加法线加吸附加带吸单步全落在冻结数上.
func TestSnapModeFromCases(t *testing.T) {
	f := loadSnapCases(t)
	cm := snapCfgMap(t, f)
	sm := snapSlopeMap(t, f)

	// Godot D18 defaults stay frozen: snap 0.1, max angle 45deg, margin
	// 0.08, grounded gear.
	if DefaultSnapLength != f.Defaults.Snap || DefaultSafeMargin != f.Defaults.Margin ||
		DefaultFloorMaxAngle != f.Defaults.MaxRad || f.Defaults.Mode != "grounded" {
		t.Errorf("defaults = %v/%v/%v, want 0.1/pi/4/0.08 grounded",
			DefaultSnapLength, DefaultFloorMaxAngle, DefaultSafeMargin)
	}
	dc := DefaultSnapConfig()
	if !dc.Valid() || dc.Mode != MotionModeGrounded || dc.SnapLength != 0.1 ||
		dc.FloorMaxAngle != math.Pi/4 || dc.SafeMargin != 0.08 {
		t.Errorf("DefaultSnapConfig = %+v, want D18 grounded", dc)
	}
	if MotionModeGrounded.String() != "grounded" || MotionModeFloating.String() != "floating" {
		t.Error("motion mode keys diverged")
	}
	if UpDirection() != core.V2(0, -1) {
		t.Errorf("up = %v, want (0,-1)", UpDirection())
	}

	// Config validity matches the frozen table.
	for _, d := range f.Configs {
		if got := mustSnapCfg(t, cm, d.Name).Valid(); got != d.Valid {
			t.Errorf("config %s valid = %v, want %v", d.Name, got, d.Valid)
		}
	}

	// Floor normals point up; walls hold no up normal.
	for _, n := range f.Normals {
		got, ok := FloorNormal(mustSnapSlope(t, sm, n.Slope))
		if ok != n.OK {
			t.Errorf("normal %s ok = %v, want %v", n.Slope, ok, n.OK)
			continue
		}
		if ok && !snapCloseVec(got, n.Want) {
			t.Errorf("normal %s = %v, want %v", n.Slope, got, n.Want)
		}
	}

	// Snap window: gap bridges, far misses, steep never floors, floating
	// never floors even on exact contact.
	for _, g := range f.Grounded {
		feet := core.V2(g.Feet[0], g.Feet[1])
		var all []Slope
		for _, d := range f.Slopes {
			all = append(all, sm[d.Name])
		}
		got, ok := SnapGrounded(feet, all, mustSnapCfg(t, cm, g.Cfg))
		if ok != g.OK {
			t.Errorf("snap %s ok = %v, want %v", g.Name, ok, g.OK)
			continue
		}
		if ok && got.Name() != g.Want {
			t.Errorf("snap %s = %q, want %q", g.Name, got.Name(), g.Want)
		}
		if IsOnFloor(feet, all, mustSnapCfg(t, cm, g.Cfg)) != g.OK {
			t.Errorf("floor %s disagrees with snap", g.Name)
		}
	}

	// Steps with snap: falling near-miss snaps, far stays air, rising
	// passes one-way then lands, steep touches but is not floor.
	for _, sd := range f.Steps {
		var slopes []Slope
		for _, n := range sd.Slopes {
			slopes = append(slopes, mustSnapSlope(t, sm, n))
		}
		snap := append([]Slope(nil), slopes...)
		feet := core.V2(sd.Feet[0], sd.Feet[1])
		vel := core.V2(sd.Vel[0], sd.Vel[1])
		pos, ground, floor, head, err := StepWithSnap(feet, vel, sd.Dt, slopes, mustSnapCfg(t, cm, sd.Cfg))
		if err != nil {
			t.Errorf("snap step %s: %v", sd.Name, err)
			continue
		}
		if !snapCloseVec(pos, sd.WantPos) {
			t.Errorf("snap step %s pos = %v, want %v", sd.Name, pos, sd.WantPos)
		}
		if floor != sd.WantFloor || head != sd.WantHead {
			t.Errorf("snap step %s floor/head = %v/%v, want %v/%v",
				sd.Name, floor, head, sd.WantFloor, sd.WantHead)
		}
		if floor && ground.Name() != sd.WantGround {
			t.Errorf("snap step %s ground = %q, want %q", sd.Name, ground.Name(), sd.WantGround)
		}
		if !floor && sd.WantGround != "" {
			t.Errorf("snap step %s ground = %q with floor=false, want empty", sd.Name, ground.Name())
		}
		for i := range slopes {
			if slopes[i] != snap[i] {
				t.Errorf("snap step %s mutated slopes", sd.Name)
				break
			}
		}
	}
}

// B:坏档坏坡空零超大不崩,占位加报错.
func TestSnapModeEdgesNoCrash(t *testing.T) {
	f := loadSnapCases(t)
	cm := snapCfgMap(t, f)
	sm := snapSlopeMap(t, f)
	def := mustSnapCfg(t, cm, "default")
	flat := mustSnapSlope(t, sm, "flat")

	// Empty and nil slope lists sample nothing, never an error.
	if _, ok := SnapGrounded(core.V2(50, 100), nil, def); ok {
		t.Error("nil slopes snap = true, want false")
	}
	if IsOnFloor(core.V2(50, 100), []Slope{}, def) {
		t.Error("empty slopes floor = true, want false")
	}
	if _, _, floor, _, err := StepWithSnap(core.V2(50, 90), core.V2(0, 100), 0.1, nil, def); err != nil || floor {
		t.Errorf("nil slopes snap step = %v/%v, want airborne move", floor, err)
	}

	// Bad configs fail closed with feet unchanged, never guessed.
	badCfgs := []SnapConfig{
		{Mode: MotionMode(-1), SnapLength: 0.1, FloorMaxAngle: math.Pi / 4, SafeMargin: 0.08},
		{Mode: MotionModeGrounded, SnapLength: -0.1, FloorMaxAngle: math.Pi / 4, SafeMargin: 0.08},
		{Mode: MotionModeGrounded, SnapLength: math.NaN(), FloorMaxAngle: math.Pi / 4, SafeMargin: 0.08},
		{Mode: MotionModeGrounded, SnapLength: 0.1, FloorMaxAngle: -1, SafeMargin: 0.08},
		{Mode: MotionModeGrounded, SnapLength: 0.1, FloorMaxAngle: math.Pi / 4, SafeMargin: math.Inf(1)},
	}
	for i, c := range badCfgs {
		if c.Valid() {
			t.Errorf("bad cfg %d reports valid", i)
		}
		if _, ok := SnapGrounded(core.V2(50, 100), []Slope{flat}, c); ok {
			t.Errorf("bad cfg %d snap = true, want false", i)
		}
		feet := core.V2(50, 90)
		if got, _, _, _, err := StepWithSnap(feet, core.V2(0, 1), 0.1, []Slope{flat}, c); core.CodeOf(err) != core.CodeInvalidArg || got != feet {
			t.Errorf("bad cfg %d step = %v/%v, want feet held + invalid-arg", i, got, err)
		}
	}

	// Bad motion inputs fail closed with feet unchanged, like Step.
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
		if got, _, _, _, err := StepWithSnap(tc.feet, tc.vel, tc.dt, []Slope{flat}, def); core.CodeOf(err) != core.CodeInvalidArg || !snapSameVec(got, tc.feet) {
			t.Errorf("%s snap step = %v/%v, want feet held + invalid-arg", tc.name, got, err)
		}
	}

	// Hand-built bad slopes fail closed in StepWithSnap, soft in snap.
	bad := Slope{}
	if _, ok := SnapGrounded(core.V2(50, 100), []Slope{bad, flat}, def); !ok {
		t.Error("snap with one bad slope skipped everything, want flat still sampled")
	}
	if _, _, _, _, err := StepWithSnap(feet, core.V2(0, 1), 0.1, []Slope{flat, bad}, def); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("bad slope snap step code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, ok := FloorNormal(bad); ok {
		t.Error("zero slope normal = ok, want false")
	}

	// Zero value is valid and strict: exact contact on dead-flat only.
	zero := SnapConfig{}
	if !zero.Valid() {
		t.Error("zero config Valid = false, want true (exact flat-only)")
	}
	if _, ok := SnapGrounded(core.V2(50, 100), []Slope{flat}, zero); !ok {
		t.Error("zero config exact contact = air, want floor")
	}
	if _, ok := SnapGrounded(core.V2(50, 99.99), []Slope{flat}, zero); ok {
		t.Error("zero config gap = floor, want air")
	}

	// Huge-but-finite inputs never panic.
	huge, _ := NewSlope("huge", core.V2(1e308, 1e308), core.V2(-1e308, -1e308))
	_, _ = FloorNormal(huge)
	_, _ = SnapGrounded(core.V2(1e307, 1e307), []Slope{huge}, def)
	_, _, _, _, _ = StepWithSnap(core.V2(0, 0), core.V2(1e308, 1e308), 0.001, []Slope{flat}, def)
}

// C:纯算数不画画,两边同数靠边界往返无损加逐位重放(C不适用像素,记边界等价).
func TestSnapModeBoundaryIdentical(t *testing.T) {
	f := loadSnapCases(t)
	cm := snapCfgMap(t, f)
	sm := snapSlopeMap(t, f)
	def := mustSnapCfg(t, cm, "default")
	// Render boundary is lossless for every frozen endpoint and foot.
	for name, s := range sm {
		for _, p := range []core.Vec2{s.A(), s.B()} {
			if back := renderconv.Vec2FromRenderPoint(renderconv.Vec2ToRenderPoint(p)); back != p {
				t.Errorf("%s boundary = %v, want %v", name, back, p)
			}
		}
	}
	for _, g := range f.Grounded {
		feet := core.V2(g.Feet[0], g.Feet[1])
		if back := renderconv.Vec2FromRenderPoint(renderconv.Vec2ToRenderPoint(feet)); back != feet {
			t.Errorf("feet %v boundary diverged", g.Feet)
		}
	}
	// Same snap step replays verbatim, input never mutated.
	flat := mustSnapSlope(t, sm, "flat")
	slopes := []Slope{flat}
	snap := append([]Slope(nil), slopes...)
	feet := core.V2(50, 99.95)
	vel := core.V2(0, 10)
	aPos, aGround, aFloor, aHead, err := StepWithSnap(feet, vel, 0.001, slopes, def)
	if err != nil {
		t.Fatalf("snap probe: %v", err)
	}
	bPos, bGround, bFloor, bHead, err := StepWithSnap(feet, vel, 0.001, slopes, def)
	if err != nil {
		t.Fatalf("snap replay: %v", err)
	}
	if aPos != bPos || aGround != bGround || aFloor != bFloor || aHead != bHead {
		t.Fatal("snap replay diverged")
	}
	for i := range slopes {
		if slopes[i] != snap[i] {
			t.Fatal("snap step mutated the input")
		}
	}
	// Floating replays the same Step motion with floor dropped.
	flo := mustSnapCfg(t, cm, "floating")
	fPos, _, fFloor, fHead, err := StepWithSnap(feet, vel, 0.001, slopes, flo)
	if err != nil || fFloor || fHead {
		t.Errorf("float replay = %v/%v/%v, want motion held, no floor/head", fPos, fFloor, err)
	}
	sPos, _, _, _, _ := Step(feet, vel, 0.001, slopes)
	if fPos != sPos {
		t.Errorf("float motion = %v, want Step motion %v", fPos, sPos)
	}
}

// D:吸附查询跑得动,耗时调用有数.
func TestSnapModePerfSnap(t *testing.T) {
	// Synthetic load only (no golden): golden stays in snap_mode_cases.json.
	long, err := NewSlope("long", core.V2(0, 0), core.V2(10000, -1000))
	if err != nil {
		t.Fatalf("long: %v", err)
	}
	def := DefaultSnapConfig()
	const reps = 5000
	start := time.Now()
	var sum float64
	holds := 0
	for i := 0; i < reps; i++ {
		x := float64(i%10001) + 0.5
		y, ok := long.GroundYAt(x)
		if !ok {
			t.Fatalf("rep %d GroundYAt ok=false", i)
		}
		sum += y
		if _, ok := SnapGrounded(core.V2(x, y), []Slope{long}, def); ok {
			holds++
		}
	}
	const steps = 2000
	for i := 0; i < steps; i++ {
		x := float64(i%10000) + 0.25
		feet := core.V2(x, -200)
		vel := core.V2(60, 300)
		if pos, _, floor, _, err := StepWithSnap(feet, vel, 0.016, []Slope{long}, def); err != nil {
			t.Fatalf("rep %d snap step: %v", i, err)
		} else {
			sum += pos.X + pos.Y
			if floor {
				holds++
			}
		}
	}
	el := time.Since(start)
	t.Logf("snap-mode: %d snaps + %d steps on 10k slope in %v (%.1f us/op, sum %.1f, holds %d)",
		reps, steps, el, float64(el.Microseconds())/float64(reps+steps), sum, holds)
}

// E:长跑不掉:万次重放逐位一致,吸附带着走不丢.
func TestSnapModeLongRunStable(t *testing.T) {
	f := loadSnapCases(t)
	cm := snapCfgMap(t, f)
	sm := snapSlopeMap(t, f)
	def := mustSnapCfg(t, cm, "default")
	flat := mustSnapSlope(t, sm, "flat")
	slopes := []Slope{flat}
	feet := core.V2(50, 99.95)
	vel := core.V2(0, 10)
	firstPos, firstGround, firstFloor, firstHead, err := StepWithSnap(feet, vel, 0.001, slopes, def)
	if err != nil {
		t.Fatalf("snap probe: %v", err)
	}
	snap := append([]Slope(nil), slopes...)
	for i := 0; i < 10000; i++ {
		pos, ground, floor, head, err := StepWithSnap(feet, vel, 0.001, slopes, def)
		if err != nil {
			t.Fatalf("rep %d: %v", i, err)
		}
		if pos != firstPos || ground != firstGround || floor != firstFloor || head != firstHead {
			t.Fatalf("rep %d drifted: %v vs %v", i, pos, firstPos)
		}
	}
	for i := range slopes {
		if slopes[i] != snap[i] {
			t.Fatal("10k snap steps mutated the input")
		}
	}
	// Carried rider keeps the snap across 10k identical float deltas.
	oneway := mustSnapSlope(t, sm, "oneway")
	carry := core.V2(50, 80)
	for i := 0; i < 10000; i++ {
		next, _, floor, _, err := StepWithSnap(carry, core.V2(0.001, 0), 1, []Slope{oneway}, def)
		if err != nil || !floor {
			t.Fatalf("carry rep %d: %v/%v", i, next, err)
		}
		carry = next
	}
	if !snapClose(carry.X, 60) || carry.Y != 80 {
		t.Errorf("carry end = %v, want (~60,80)", carry)
	}
	// Jump up through the one-way and fall back: pass once, land once.
	up, _, floor, head, err := StepWithSnap(core.V2(50, 90), core.V2(0, -200), 0.1, []Slope{oneway}, def)
	if err != nil || floor || head || !snapCloseVec(up, [2]float64{50, 70}) {
		t.Fatalf("snap pass = %v/%v/%v/%v, want [50 70] airborne", up, floor, head, err)
	}
	down, _, floor, head, err := StepWithSnap(up, core.V2(0, 100), 0.1, []Slope{oneway}, def)
	if err != nil || !floor || head || !snapCloseVec(down, [2]float64{50, 80}) {
		t.Fatalf("snap land = %v/%v/%v/%v, want [50 80] floor", down, floor, head, err)
	}
}

// F:离屏金对照窗(W11 game_physics --case=jump后建):冻结数加形状断言.
func TestSnapModeOffscreenGolden(t *testing.T) {
	f := loadSnapCases(t)
	cm := snapCfgMap(t, f)
	sm := snapSlopeMap(t, f)
	def := mustSnapCfg(t, cm, "default")
	// Golden numbers stay frozen.
	for _, g := range f.Grounded {
		feet := core.V2(g.Feet[0], g.Feet[1])
		var all []Slope
		for _, d := range f.Slopes {
			all = append(all, sm[d.Name])
		}
		if got, ok := SnapGrounded(feet, all, mustSnapCfg(t, cm, g.Cfg)); ok != g.OK || (ok && got.Name() != g.Want) {
			t.Fatalf("golden snap %s = %q/%v, want %q/%v", g.Name, got.Name(), ok, g.Want, g.OK)
		}
	}
	for _, sd := range f.Steps {
		var slopes []Slope
		for _, n := range sd.Slopes {
			slopes = append(slopes, mustSnapSlope(t, sm, n))
		}
		pos, ground, floor, head, err := StepWithSnap(core.V2(sd.Feet[0], sd.Feet[1]), core.V2(sd.Vel[0], sd.Vel[1]), sd.Dt, slopes, mustSnapCfg(t, cm, sd.Cfg))
		if err != nil {
			t.Fatalf("golden snap step %s: %v", sd.Name, err)
		}
		if !snapCloseVec(pos, sd.WantPos) || floor != sd.WantFloor || head != sd.WantHead {
			t.Fatalf("golden snap step %s = %v/%v/%v, want %v/%v/%v",
				sd.Name, pos, floor, head, sd.WantPos, sd.WantFloor, sd.WantHead)
		}
		if floor && ground.Name() != sd.WantGround {
			t.Fatalf("golden snap step %s ground = %q, want %q", sd.Name, ground.Name(), sd.WantGround)
		}
	}
	// Shape: exact contact floors in grounded, never in floating.
	flat := mustSnapSlope(t, sm, "flat")
	flo := mustSnapCfg(t, cm, "floating")
	if !IsOnFloor(core.V2(50, 100), []Slope{flat}, def) {
		t.Error("exact contact floor = false, want true")
	}
	if IsOnFloor(core.V2(50, 100), []Slope{flat}, flo) {
		t.Error("floating exact contact = floor, want never floor")
	}
	// Shape: gentle ramp is floor, steep wall is touch-only.
	if _, ok := SnapGrounded(core.V2(50, 90), []Slope{mustSnapSlope(t, sm, "ramp")}, def); !ok {
		t.Error("ramp contact = air, want floor")
	}
	if _, ok := SnapGrounded(core.V2(10, 50), []Slope{mustSnapSlope(t, sm, "steep")}, def); ok {
		t.Error("steep contact = floor, want touch-only")
	}
	// Shape: single up through one-way passes, solid reports the bump;
	// falling lands on the higher surface first.
	oneway := mustSnapSlope(t, sm, "oneway")
	_, _, _, head, _ := StepWithSnap(core.V2(50, 90), core.V2(0, -200), 0.1, []Slope{oneway}, def)
	if head {
		t.Error("oneway snap pass head = true, want false")
	}
	_, _, _, head, _ = StepWithSnap(core.V2(50, 110), core.V2(0, -200), 0.1, []Slope{flat}, def)
	if !head {
		t.Error("solid snap pass head = false, want true")
	}
	up := []Slope{oneway, flat}
	if _, g, ok, _, _ := StepWithSnap(core.V2(50, 70), core.V2(0, 500), 0.1, up, def); !ok || g.Name() != "oneway" {
		t.Errorf("snap stack fall = %q/%v, want oneway first", g.Name(), ok)
	}
}
