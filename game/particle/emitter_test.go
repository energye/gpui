package particle

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/game/core"
	"github.com/energye/gpui/game/sprite"
)

type shapeDef struct {
	Kind     string    `json:"kind"`
	Dir      []float64 `json:"dir"`
	AngleDeg float64   `json:"angle_deg"`
	Extents  []float64 `json:"extents"`
	Inner    float64   `json:"ring_inner"`
	Outer    float64   `json:"ring_outer"`
}

type turbDef struct {
	Strength float64 `json:"strength"`
	Scale    float64 `json:"scale"`
}

type subDef struct {
	Count  int       `json:"count"`
	Speed  []float64 `json:"speed"`
	LifeMs []int64   `json:"life_ms"`
}

type emitterCase struct {
	Name       string    `json:"name"`
	Origin     []float64 `json:"origin"`
	Shape      shapeDef  `json:"shape"`
	Rate       float64   `json:"rate"`
	Max        int       `json:"max"`
	Speed      []float64 `json:"speed"`
	LifeMs     []int64   `json:"life_ms"`
	Gravity    []float64 `json:"gravity"`
	Start      []float64 `json:"start"`
	End        []float64 `json:"end"`
	Turbulence turbDef   `json:"turbulence"`
	Sub        subDef    `json:"sub"`
	Seed       uint64    `json:"seed"`
	Spawn      int       `json:"spawn"`
	UpdateMs   int64     `json:"update_ms"`
	WantSpawn  int       `json:"want_spawned"`
	WantAlive  int       `json:"want_alive"`
	WantChild  int       `json:"want_child"`
}

type emitterFile struct {
	Cases []emitterCase `json:"cases"`
}

func loadEmitterCases(t *testing.T) emitterFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "emitter_cases.json"))
	if err != nil {
		t.Fatalf("read emitter_cases.json: %v", err)
	}
	var f emitterFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode emitter_cases.json: %v", err)
	}
	if len(f.Cases) == 0 {
		t.Fatal("emitter_cases.json has no cases")
	}
	return f
}

func mustFindEmitterCase(t *testing.T, f emitterFile, name string) emitterCase {
	t.Helper()
	for _, c := range f.Cases {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("emitter_cases.json has no case %q", name)
	return emitterCase{}
}

func vec2Of(t *testing.T, v []float64, what string) core.Vec2 {
	t.Helper()
	if len(v) != 2 {
		t.Fatalf("%s has %d numbers, want 2", what, len(v))
	}
	return core.V2(v[0], v[1])
}

func colorOf(t *testing.T, v []float64, what string) core.Color {
	t.Helper()
	if len(v) != 4 {
		t.Fatalf("%s has %d numbers, want 4", what, len(v))
	}
	return core.RGBA(v[0], v[1], v[2], v[3])
}

func buildEmitter(t *testing.T, c emitterCase) *Emitter {
	t.Helper()
	origin := vec2Of(t, c.Origin, c.Name+"/origin")
	gravity := vec2Of(t, c.Gravity, c.Name+"/gravity")
	start := colorOf(t, c.Start, c.Name+"/start")
	end := colorOf(t, c.End, c.Name+"/end")
	if len(c.Speed) != 2 {
		t.Fatalf("%s speed has %d numbers, want 2", c.Name, len(c.Speed))
	}
	if len(c.LifeMs) != 2 {
		t.Fatalf("%s life_ms has %d numbers, want 2", c.Name, len(c.LifeMs))
	}
	var shape Shape
	var err error
	dir := vec2Of(t, c.Shape.Dir, c.Name+"/shape/dir")
	switch c.Shape.Kind {
	case "point":
		shape, err = PointShape(dir)
	case "cone":
		shape, err = ConeShape(dir, c.Shape.AngleDeg*math.Pi/180)
	case "box":
		ext := vec2Of(t, c.Shape.Extents, c.Name+"/shape/extents")
		shape, err = BoxShape(dir, ext)
	case "ring":
		shape, err = RingShape(c.Shape.Inner, c.Shape.Outer)
	default:
		t.Fatalf("%s unknown shape kind %q", c.Name, c.Shape.Kind)
	}
	if err != nil {
		t.Fatalf("%s shape: %v", c.Name, err)
	}
	turb, err := NewTurbulence(c.Turbulence.Strength, c.Turbulence.Scale)
	if err != nil {
		t.Fatalf("%s turbulence: %v", c.Name, err)
	}
	var sub Sub
	if c.Sub.Count == 0 {
		sub = NoSub()
	} else {
		if len(c.Sub.Speed) != 2 || len(c.Sub.LifeMs) != 2 {
			t.Fatalf("%s sub speed/life shape", c.Name)
		}
		sub, err = NewSub(c.Sub.Count, c.Sub.Speed[0], c.Sub.Speed[1],
			core.Milliseconds(c.Sub.LifeMs[0]), core.Milliseconds(c.Sub.LifeMs[1]))
		if err != nil {
			t.Fatalf("%s sub: %v", c.Name, err)
		}
	}
	cfg := EmitterConfig{
		Origin:     origin,
		Rate:       c.Rate,
		Max:        c.Max,
		SpeedMin:   c.Speed[0],
		SpeedMax:   c.Speed[1],
		LifeMin:    core.Milliseconds(c.LifeMs[0]),
		LifeMax:    core.Milliseconds(c.LifeMs[1]),
		Gravity:    gravity,
		Start:      start,
		End:        end,
		Shape:      shape,
		Turbulence: turb,
		Sub:        sub,
	}
	e, err := NewEmitter(cfg, c.Seed)
	if err != nil {
		t.Fatalf("%s NewEmitter: %v", c.Name, err)
	}
	return e
}

func approxEq(a, b, eps float64) bool { return math.Abs(a-b) < eps }

// A: count, speed, life, gravity, color, shape, turbulence, sub all match.
func TestEmitterCasesFromFile(t *testing.T) {
	f := loadEmitterCases(t)
	for _, c := range f.Cases {
		e := buildEmitter(t, c)
		gotSpawn, err := e.Spawn(c.Spawn)
		if err != nil {
			t.Fatalf("%s Spawn: %v", c.Name, err)
		}
		// Zero-spawn case parks here; shape checks need live particles.
		if c.Spawn == 0 {
			if gotSpawn != 0 || e.Alive() != 0 {
				t.Errorf("%s zero spawn = %d alive %d, want 0/0", c.Name, gotSpawn, e.Alive())
			}
			s, d := e.Update(core.Milliseconds(c.UpdateMs))
			if s != 0 || d != 0 || e.Alive() != 0 {
				t.Errorf("%s zero update spawned=%d died=%d alive=%d, want 0/0/0", c.Name, s, d, e.Alive())
			}
			continue
		}
		if gotSpawn != c.Spawn {
			t.Errorf("%s Spawn = %d, want %d", c.Name, gotSpawn, c.Spawn)
		}
		origin := vec2Of(t, c.Origin, c.Name+"/origin")
		start := colorOf(t, c.Start, c.Name+"/start")
		// Fresh births: age zero, life/speed/shape/color in range.
		fresh := e.Particles()
		if len(fresh) != c.Spawn {
			t.Fatalf("%s fresh len = %d, want %d", c.Name, len(fresh), c.Spawn)
		}
		for i, p := range fresh {
			if p.Age != 0 {
				t.Errorf("%s p%d Age = %v, want 0", c.Name, i, p.Age)
			}
			if int64(p.Life) < c.LifeMs[0] || int64(p.Life) > c.LifeMs[1] {
				t.Errorf("%s p%d Life = %v, want [%d,%d]", c.Name, i, p.Life, c.LifeMs[0], c.LifeMs[1])
			}
			sp := p.Vel.Length()
			if sp < c.Speed[0]-1e-9 || sp > c.Speed[1]+1e-9 {
				t.Errorf("%s p%d speed = %v, want [%v,%v]", c.Name, i, sp, c.Speed[0], c.Speed[1])
			}
			if p.AgeFrac() != 0 {
				t.Errorf("%s p%d AgeFrac = %v, want 0", c.Name, i, p.AgeFrac())
			}
			if got := p.ColorAt(start, colorOf(t, c.End, c.Name+"/end")); !got.ApproxEqual(start, 1e-9) {
				t.Errorf("%s p%d birth color = %+v, want start %+v", c.Name, i, got, start)
			}
			switch c.Shape.Kind {
			case "point":
				if !p.Pos.ApproxEqual(origin, 1e-9) {
					t.Errorf("%s p%d pos = %+v, want origin %+v", c.Name, i, p.Pos, origin)
				}
			case "cone":
				dir := vec2Of(t, c.Shape.Dir, c.Name+"/dir")
				ang := math.Abs(p.Vel.Angle(dir.Normalize()))
				limit := c.Shape.AngleDeg*math.Pi/180 + 1e-9
				if ang > limit {
					t.Errorf("%s p%d cone angle = %v, want <= %v", c.Name, i, ang, limit)
				}
			case "box":
				ext := vec2Of(t, c.Shape.Extents, c.Name+"/ext")
				dx := math.Abs(p.Pos.X - origin.X)
				dy := math.Abs(p.Pos.Y - origin.Y)
				if dx > ext.X+1e-9 || dy > ext.Y+1e-9 {
					t.Errorf("%s p%d pos = %+v outside box ext %+v", c.Name, i, p.Pos, ext)
				}
			case "ring":
				r := p.Pos.Sub(origin).Length()
				if r < c.Shape.Inner-1e-9 || r > c.Shape.Outer+1e-9 {
					t.Errorf("%s p%d radius = %v, want [%v,%v]", c.Name, i, r, c.Shape.Inner, c.Shape.Outer)
				}
				if r > 1e-9 {
					radial := p.Pos.Sub(origin).Normalize()
					velDir := p.Vel.Normalize()
					if radial.Dot(velDir) < 1-1e-9 {
						t.Errorf("%s p%d vel not radial (dot=%v)", c.Name, i, radial.Dot(velDir))
					}
				}
			}
		}
		// Snapshot one particle for the gravity check (turbulence-free only).
		var before Particle
		var hasBefore bool
		if c.Turbulence.Strength == 0 && len(fresh) > 0 {
			before = fresh[0]
			hasBefore = true
		}
		dt := core.Milliseconds(c.UpdateMs)
		e.Update(dt)
		if e.Spawned() != c.WantSpawn {
			t.Errorf("%s Spawned = %d, want %d", c.Name, e.Spawned(), c.WantSpawn)
		}
		if e.Alive() != c.WantAlive {
			t.Errorf("%s Alive = %d, want %d", c.Name, e.Alive(), c.WantAlive)
		}
		if e.ChildSpawned() != c.WantChild {
			t.Errorf("%s ChildSpawned = %d, want %d", c.Name, e.ChildSpawned(), c.WantChild)
		}
		if hasBefore && c.Sub.Count == 0 {
			// Semi-implicit Euler: vel += g*dt, pos += velNew*dt.
			dtSec := dt.Seconds()
			g := vec2Of(t, c.Gravity, c.Name+"/gravity")
			after := e.Particles()
			// Rate births append after survivors; survivor 0 stays first
			// while no deaths happened (update < min life covers this).
			if len(after) > 0 && c.UpdateMs < c.LifeMs[0] {
				got := after[0]
				wantVel := core.V2(before.Vel.X+g.X*dtSec, before.Vel.Y+g.Y*dtSec)
				if !got.Vel.ApproxEqual(wantVel, 1e-6) {
					t.Errorf("%s gravity vel = %+v, want %+v", c.Name, got.Vel, wantVel)
				}
				wantPos := core.V2(before.Pos.X+wantVel.X*dtSec, before.Pos.Y+wantVel.Y*dtSec)
				if !got.Pos.ApproxEqual(wantPos, 1e-6) {
					t.Errorf("%s gravity pos = %+v, want %+v", c.Name, got.Pos, wantPos)
				}
			}
		}
		// Color ramp: mid lerp and end park.
		mid := Particle{Age: core.Milliseconds(500), Life: core.Milliseconds(1000)}
		wantMid := start.Lerp(colorOf(t, c.End, c.Name+"/end"), 0.5)
		if got := mid.ColorAt(start, colorOf(t, c.End, c.Name+"/end")); !got.ApproxEqual(wantMid, 1e-9) {
			t.Errorf("%s mid color = %+v, want %+v", c.Name, got, wantMid)
		}
		// Turbulence drift: same seed with/without drift must diverge,
		// and the drift vector stays bounded by strength.
		if c.Name == "turb_drift" {
			plain := buildEmitter(t, mustFindEmitterCase(t, f, "point_fire"))
			// Align seeds for a fair drift comparison is not required;
			// here we only pin the bound on the vector itself.
			tv := e.Config().Turbulence.Vec(0.25, 0.2)
			if math.Abs(tv.X) > e.Config().Turbulence.Strength+1e-9 || math.Abs(tv.Y) > e.Config().Turbulence.Strength+1e-9 {
				t.Errorf("%s turb vec = %+v exceeds strength", c.Name, tv)
			}
			_ = plain
			// Drift pair: identical recipe except turbulence must diverge.
			cfg := e.Config()
			cfg.Turbulence = NoTurbulence()
			a, err := NewEmitter(cfg, c.Seed)
			if err != nil {
				t.Fatalf("%s no-turb rebuild: %v", c.Name, err)
			}
			b, err := NewEmitter(e.Config(), c.Seed)
			if err != nil {
				t.Fatalf("%s turb rebuild: %v", c.Name, err)
			}
			if _, err := a.Spawn(4); err != nil {
				t.Fatal(err)
			}
			if _, err := b.Spawn(4); err != nil {
				t.Fatal(err)
			}
			a.Update(core.Milliseconds(200))
			b.Update(core.Milliseconds(200))
			pa, pb := a.Particles(), b.Particles()
			same := true
			for i := range pa {
				if !pa[i].Pos.ApproxEqual(pb[i].Pos, 1e-9) {
					same = false
					break
				}
			}
			if same {
				t.Errorf("%s turbulence did not drift positions", c.Name)
			}
		}
		// Sub rule: children are flagged, live in the sub life range.
		if c.Sub.Count > 0 {
			kids := 0
			for _, p := range e.Particles() {
				if p.Child {
					kids++
					if int64(p.Life) < c.Sub.LifeMs[0] || int64(p.Life) > c.Sub.LifeMs[1] {
						t.Errorf("%s child life = %v, want [%d,%d]", c.Name, p.Life, c.Sub.LifeMs[0], c.Sub.LifeMs[1])
					}
				}
			}
			if kids != c.WantChild {
				t.Errorf("%s live children = %d, want %d", c.Name, kids, c.WantChild)
			}
		}
	}
}

// B: empty, zero, oversized, and corrupt inputs never crash.
func TestEmitterEdgesNoCrash(t *testing.T) {
	// Zero emission parks at zero.
	f := loadEmitterCases(t)
	zc := mustFindEmitterCase(t, f, "zero_rate")
	z := buildEmitter(t, zc)
	if n, err := z.Spawn(0); err != nil || n != 0 {
		t.Errorf("Spawn(0) = %d,%v, want 0,nil", n, err)
	}
	if s, d := z.Update(core.Milliseconds(100)); s != 0 || d != 0 {
		t.Errorf("zero Update = %d/%d, want 0/0", s, d)
	}
	if z.Alive() != 0 || z.Spawned() != 0 || z.Died() != 0 || z.ChildSpawned() != 0 {
		t.Error("zero counters non-zero")
	}
	if z.Particles() != nil {
		t.Error("zero Particles non-nil, want nil")
	}
	// Non-positive dt is a quiet no-op.
	e := buildEmitter(t, mustFindEmitterCase(t, f, "point_fire"))
	if _, err := e.Spawn(3); err != nil {
		t.Fatalf("Spawn 3: %v", err)
	}
	if s, d := e.Update(0); s != 0 || d != 0 || e.Alive() != 3 {
		t.Errorf("Update(0) = %d/%d alive %d, want 0/0/3", s, d, e.Alive())
	}
	if s, d := e.Update(core.Milliseconds(-5)); s != 0 || d != 0 || e.Alive() != 3 {
		t.Errorf("Update(neg) = %d/%d alive %d, want 0/0/3", s, d, e.Alive())
	}
	// Oversized spawn clamps to Max quietly.
	smallCfg := e.Config()
	smallCfg.Max = 4
	smallCfg.Rate = 0
	small, err := NewEmitter(smallCfg, 7)
	if err != nil {
		t.Fatalf("small NewEmitter: %v", err)
	}
	if n, err := small.Spawn(10); err != nil || n != 4 || small.Alive() != 4 {
		t.Errorf("clamp Spawn = %d,%v alive %d, want 4,nil/4", n, err, small.Alive())
	}
	if n, err := small.Spawn(10); err != nil || n != 0 || small.Alive() != 4 {
		t.Errorf("full Spawn = %d,%v alive %d, want 0,nil/4", n, err, small.Alive())
	}
	// Bad constructors report codes and store nothing.
	if _, err := NewEmitter(EmitterConfig{}, 0); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty cfg err = %v, want invalid-arg", err)
	}
	over := e.Config()
	over.Max = MaxParticlesCap + 1
	if _, err := NewEmitter(over, 0); core.CodeOf(err) != core.CodeOutOfMemory {
		t.Errorf("over-max err = %v, want out-of-memory", err)
	}
	badCfgs := []func(EmitterConfig) EmitterConfig{
		func(c EmitterConfig) EmitterConfig { c.Rate = -1; return c },
		func(c EmitterConfig) EmitterConfig { c.SpeedMin = 90; c.SpeedMax = 10; return c },
		func(c EmitterConfig) EmitterConfig { c.LifeMin = 0; return c },
		func(c EmitterConfig) EmitterConfig { c.Gravity = core.V2(math.NaN(), 0); return c },
		func(c EmitterConfig) EmitterConfig { c.Shape = Shape{Kind: ShapeKind(99)}; return c },
		func(c EmitterConfig) EmitterConfig { c.Sub = Sub{Count: MaxSubPerDeath + 1}; return c },
	}
	for i, mut := range badCfgs {
		if _, err := NewEmitter(mut(e.Config()), 0); core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("bad cfg[%d] err = %v, want invalid-arg", i, err)
		}
	}
	if _, err := PointShape(core.V2(math.NaN(), 0)); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("bad point err = %v, want invalid-arg", err)
	}
	if _, err := ConeShape(core.Vec2{}, 0.2); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("zero-dir cone err = %v, want invalid-arg", err)
	}
	if _, err := BoxShape(core.V2(0, -1), core.V2(-1, 0)); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("neg extents err = %v, want invalid-arg", err)
	}
	if _, err := RingShape(9, 4); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("inner>outer err = %v, want invalid-arg", err)
	}
	if _, err := NewTurbulence(-1, 0); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("neg strength err = %v, want invalid-arg", err)
	}
	if _, err := NewSub(-1, 0, 0, 0, 0); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("neg sub err = %v, want invalid-arg", err)
	}
	if _, err := e.Spawn(-2); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("Spawn(neg) err = %v, want invalid-arg", err)
	}
	// Bad batch wiring reports invalid-arg and appends nothing.
	b := sprite.NewBatch()
	if _, err := e.AppendToBatch(nil, "tex/fire", core.NewRect(0, 0, 8, 8), 6); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil batch err = %v, want invalid-arg", err)
	}
	if _, err := e.AppendToBatch(b, "", core.NewRect(0, 0, 8, 8), 6); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty image err = %v, want invalid-arg", err)
	}
	if _, err := e.AppendToBatch(b, "tex/fire", core.NewRect(0, 0, 8, 8), 0); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("zero size err = %v, want invalid-arg", err)
	}
	if b.Len() != 0 {
		t.Errorf("bad wiring stored %d, want 0", b.Len())
	}
	// Nil receivers never panic.
	var nilE *Emitter
	if n, err := nilE.Spawn(1); err == nil || n != 0 {
		t.Errorf("nil Spawn = %d,%v, want 0,error", n, err)
	}
	if s, d := nilE.Update(core.Milliseconds(16)); s != 0 || d != 0 {
		t.Errorf("nil Update = %d/%d, want 0/0", s, d)
	}
	if nilE.Alive() != 0 || nilE.Spawned() != 0 || nilE.Died() != 0 || nilE.ChildSpawned() != 0 {
		t.Error("nil counters non-zero")
	}
	if nilE.Particles() != nil {
		t.Error("nil Particles non-nil")
	}
	nilE.Clear()
	if _, err := nilE.AppendToBatch(b, "tex/fire", core.NewRect(0, 0, 8, 8), 6); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil Append err = %v, want invalid-arg", err)
	}
	var nilP *Particle
	nilP.step(0.016, core.Vec2{}, NoTurbulence())
	if (Particle{}).AgeFrac() != 1 {
		t.Error("zero-life AgeFrac != 1")
	}
	if (Particle{Age: 10, Life: 10}).Alive() {
		t.Error("Age==Life Alive true, want false")
	}
}

// C: replay is bitwise identical and copies never alias the emitter.
func TestEmitterBoundaryIdentical(t *testing.T) {
	f := loadEmitterCases(t)
	for _, c := range f.Cases {
		build := func() *Emitter { return buildEmitter(t, c) }
		a, b := build(), build()
		if _, err := a.Spawn(c.Spawn); err != nil {
			t.Fatalf("%s Spawn: %v", c.Name, err)
		}
		if _, err := b.Spawn(c.Spawn); err != nil {
			t.Fatalf("%s Spawn: %v", c.Name, err)
		}
		a.Update(core.Milliseconds(c.UpdateMs))
		b.Update(core.Milliseconds(c.UpdateMs))
		pa, pb := a.Particles(), b.Particles()
		if len(pa) != len(pb) {
			t.Fatalf("%s replay len %d vs %d", c.Name, len(pa), len(pb))
		}
		for i := range pa {
			if pa[i] != pb[i] {
				t.Fatalf("%s replay diverged at %d: %+v vs %+v", c.Name, i, pa[i], pb[i])
			}
			ca, cb := a.ColorOf(pa[i]), b.ColorOf(pb[i])
			if ca != cb {
				t.Fatalf("%s color replay diverged at %d", c.Name, i)
			}
		}
		// Emitted slices are fresh copies.
		if len(pa) > 0 {
			pa[0].Pos = core.V2(99999, 99999)
			again := a.Particles()
			if again[0].Pos.ApproxEqual(core.V2(99999, 99999), 1e-9) {
				t.Fatalf("%s Particles aliases live set", c.Name)
			}
		}
		// Batch wiring is deterministic and never mutates the emitter.
		mkBatch := func(e *Emitter) []sprite.Sprite {
			bt := sprite.NewBatch()
			if _, err := e.AppendToBatch(bt, "tex/fire", core.NewRect(0, 0, 8, 8), 6); err != nil {
				t.Fatalf("%s Append: %v", c.Name, err)
			}
			var out []sprite.Sprite
			bt.Flush(func(_ core.AssetID, ss []sprite.Sprite) { out = append(out, ss...) })
			return out
		}
		before := a.Alive()
		sa, sb := mkBatch(a), mkBatch(b)
		if a.Alive() != before {
			t.Fatalf("%s Append mutated emitter", c.Name)
		}
		if len(sa) != len(sb) {
			t.Fatalf("%s batch len %d vs %d", c.Name, len(sa), len(sb))
		}
		for i := range sa {
			if sa[i].Dst != sb[i].Dst || sa[i].Opacity != sb[i].Opacity {
				t.Fatalf("%s batch diverged at %d", c.Name, i)
			}
		}
	}
	// Config snapshots never alias the emitter.
	e := buildEmitter(t, mustFindEmitterCase(t, f, "point_fire"))
	cfg := e.Config()
	cfg.Rate = -999
	if e.Config().Rate == -999 {
		t.Error("Config aliases emitter")
	}
}

// D: 1000 particles update plus sprite wiring with a measured cost.
func TestEmitterPerfThousand(t *testing.T) {
	f := loadEmitterCases(t)
	base := mustFindEmitterCase(t, f, "point_fire")
	e := buildEmitter(t, base)
	// Synthetic load only (no golden): golden counts stay in
	// emitter_cases.json. Seeded emitter keeps the load replayable.
	e.Config()
	cfg := e.Config()
	cfg.Max = 2048
	cfg.Rate = 0
	big, err := NewEmitter(cfg, 20260915)
	if err != nil {
		t.Fatalf("big NewEmitter: %v", err)
	}
	const n = 1000
	start := time.Now()
	if got, err := big.Spawn(n); err != nil || got != n {
		t.Fatalf("Spawn 1000 = %d,%v", got, err)
	}
	spawnEl := time.Since(start)
	if big.Alive() != n {
		t.Fatalf("Alive = %d, want %d", big.Alive(), n)
	}
	bt := sprite.NewBatch()
	t0 := time.Now()
	s, d := big.Update(core.Milliseconds(16))
	upEl := time.Since(t0)
	t1 := time.Now()
	appended, err := big.AppendToBatch(bt, "tex/fire", core.NewRect(0, 0, 8, 8), 6)
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	appendEl := time.Since(t1)
	t2 := time.Now()
	calls := bt.Flush(func(_ core.AssetID, ss []sprite.Sprite) {})
	flushEl := time.Since(t2)
	if s != 0 || d != 0 {
		t.Errorf("steady 16ms spawned/died = %d/%d, want 0/0", s, d)
	}
	if appended != n || calls != 1 {
		t.Errorf("append/calls = %d/%d, want %d/1", appended, calls, n)
	}
	t.Logf("emitter-1000: spawn %v update %v append %v flush %v (%d draws)", spawnEl, upEl, appendEl, flushEl, calls)
}

// E: long runs neither grow nor diverge between replays.
func TestEmitterLongRunStable(t *testing.T) {
	f := loadEmitterCases(t)
	c := mustFindEmitterCase(t, f, "cone_fire")
	mk := func() *Emitter {
		e := buildEmitter(t, c)
		if _, err := e.Spawn(60); err != nil {
			t.Fatalf("Spawn: %v", err)
		}
		return e
	}
	first := mk()
	first.Update(core.Milliseconds(16))
	want := first.Particles()
	for i := 0; i < 2000; i++ {
		e := mk()
		e.Update(core.Milliseconds(16))
		got := e.Particles()
		if len(got) != len(want) {
			t.Fatalf("rep %d len %d vs %d", i, len(got), len(want))
		}
		for j := range got {
			if got[j] != want[j] {
				t.Fatalf("rep %d diverged at %d", i, j)
			}
		}
	}
	// Budget never grows: repeated fill plus drain returns to baseline.
	e := buildEmitter(t, c)
	for i := 0; i < 200; i++ {
		if _, err := e.Spawn(50); err != nil {
			t.Fatalf("cycle %d Spawn: %v", i, err)
		}
		e.Update(core.Milliseconds(500))
		if e.Alive() > c.Max {
			t.Fatalf("cycle %d alive %d over max %d", i, e.Alive(), c.Max)
		}
		e.Clear()
		if e.Alive() != 0 || e.Spawned() != 0 || e.Died() != 0 || e.ChildSpawned() != 0 {
			t.Fatalf("cycle %d not back to zero", i)
		}
	}
	// Sub long run stays capped as well.
	sub := buildEmitter(t, mustFindEmitterCase(t, f, "sub_ember"))
	for i := 0; i < 200; i++ {
		if _, err := sub.Spawn(6); err != nil {
			t.Fatalf("sub cycle %d Spawn: %v", i, err)
		}
		sub.Update(core.Milliseconds(500))
		if sub.Alive() > 256 {
			t.Fatalf("sub cycle %d alive %d over budget", i, sub.Alive())
		}
		sub.Clear()
	}
}

// F: offscreen golden stands in for the window (fire cone plus smoke).
// The frozen counts in emitter_cases.json are the evidence both backends
// share; shape assertions below pin the meaning, not just the numbers.
func TestEmitterOffscreenGolden(t *testing.T) {
	f := loadEmitterCases(t)
	// Fire cone points up and stays inside its spread.
	cone := mustFindEmitterCase(t, f, "cone_fire")
	e := buildEmitter(t, cone)
	if _, err := e.Spawn(cone.Spawn); err != nil {
		t.Fatal(err)
	}
	for _, p := range e.Particles() {
		if p.Vel.Y >= 0 {
			t.Errorf("fire vel %+v not upward (neg Y)", p.Vel)
		}
	}
	// Smoke box spans its extents and drifts with turbulence.
	box := mustFindEmitterCase(t, f, "box_smoke")
	sm := buildEmitter(t, box)
	if _, err := sm.Spawn(4); err != nil {
		t.Fatal(err)
	}
	ext := vec2Of(t, box.Shape.Extents, "box/ext")
	seenLeft, seenRight := false, false
	for _, p := range sm.Particles() {
		dx := p.Pos.X - box.Origin[0]
		if dx < -ext.X+1e-9 || dx > ext.X+1e-9 {
			t.Errorf("smoke x %v outside ext %v", dx, ext.X)
		}
		if dx < 0 {
			seenLeft = true
		}
		if dx > 0 {
			seenRight = true
		}
	}
	if !seenLeft || !seenRight {
		t.Error("smoke box did not span both sides")
	}
	sm.Update(core.Milliseconds(300))
	rise := 0
	for _, p := range sm.Particles() {
		if p.Pos.Y < box.Origin[1] {
			rise++
		}
	}
	if rise == 0 {
		t.Error("smoke did not rise after 300ms")
	}
	// Ring puff stays annular and moves outward.
	ring := mustFindEmitterCase(t, f, "ring_puff")
	rg := buildEmitter(t, ring)
	if _, err := rg.Spawn(ring.Spawn); err != nil {
		t.Fatal(err)
	}
	for _, p := range rg.Particles() {
		r := p.Pos.Sub(core.V2(ring.Origin[0], ring.Origin[1])).Length()
		if r < ring.Shape.Inner-1e-9 || r > ring.Shape.Outer+1e-9 {
			t.Errorf("ring radius %v outside [%v,%v]", r, ring.Shape.Inner, ring.Shape.Outer)
		}
	}
	// Wants in the file are the frozen golden.
	for _, c := range f.Cases {
		e := buildEmitter(t, c)
		if _, err := e.Spawn(c.Spawn); err != nil {
			t.Fatal(err)
		}
		e.Update(core.Milliseconds(c.UpdateMs))
		if e.Spawned() != c.WantSpawn || e.Alive() != c.WantAlive || e.ChildSpawned() != c.WantChild {
			t.Errorf("%s golden spawned/alive/child = %d/%d/%d, want %d/%d/%d",
				c.Name, e.Spawned(), e.Alive(), e.ChildSpawned(), c.WantSpawn, c.WantAlive, c.WantChild)
		}
	}
	// Sprite wiring carries position plus alpha for the window intent
	// (game_particle --case=fire draws the live set in one batch).
	fire := buildEmitter(t, mustFindEmitterCase(t, f, "point_fire"))
	if _, err := fire.Spawn(8); err != nil {
		t.Fatal(err)
	}
	bt := sprite.NewBatch()
	n, err := fire.AppendToBatch(bt, "tex/fire", core.NewRect(0, 0, 8, 8), 6)
	if err != nil || n == 0 {
		t.Fatalf("fire Append = %d,%v", n, err)
	}
	if got := bt.Flush(func(core.AssetID, []sprite.Sprite) {}); got != 1 {
		t.Errorf("fire Flush = %d, want 1", got)
	}
}
