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

type trailCaseJSON struct {
	Name       string      `json:"name"`
	MaxPoints  int         `json:"max_points"`
	HeadWidth  float64     `json:"head_width"`
	TailWidth  float64     `json:"tail_width"`
	Start      []float64   `json:"start"`
	End        []float64   `json:"end"`
	Joint      string      `json:"joint"`
	Points     [][]float64 `json:"points"`
	WantWidths []float64   `json:"want_widths"`
	WantColors [][]float64 `json:"want_colors"`
	WantLefts  [][]float64 `json:"want_lefts"`
	WantRights [][]float64 `json:"want_rights"`
	WantVerts  int         `json:"want_verts"`
}

type poolTrailCaseJSON struct {
	Name       string    `json:"name"`
	Origin     []float64 `json:"origin"`
	ShapeKind  string    `json:"shape_kind"`
	Dir        []float64 `json:"dir"`
	AngleDeg   float64   `json:"angle_deg"`
	Extents    []float64 `json:"extents"`
	Inner      float64   `json:"ring_inner"`
	Outer      float64   `json:"ring_outer"`
	Rate       float64   `json:"rate"`
	Max        int       `json:"max"`
	Speed      []float64 `json:"speed"`
	LifeMs     []int64   `json:"life_ms"`
	Gravity    []float64 `json:"gravity"`
	Start      []float64 `json:"start"`
	End        []float64 `json:"end"`
	Seed       uint64    `json:"seed"`
	Spawn      int       `json:"spawn"`
	UpdateMs   int64     `json:"update_ms"`
	TrailMax   int       `json:"trail_max"`
	TrailHead  float64   `json:"trail_head"`
	TrailTail  float64   `json:"trail_tail"`
	TrailJoint string    `json:"trail_joint"`
	WantSpawn  int       `json:"want_spawned"`
	WantAlive  int       `json:"want_alive"`
	WantChild  int       `json:"want_child"`
	WantTrail  int       `json:"want_trails"`
	WantPts    int       `json:"want_points"`
}

type gpuTrailFile struct {
	Trails []trailCaseJSON     `json:"trails"`
	Pools  []poolTrailCaseJSON `json:"pools"`
}

func loadGPUTrailCases(t *testing.T) gpuTrailFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "gpu_trail_cases.json"))
	if err != nil {
		t.Fatalf("read gpu_trail_cases.json: %v", err)
	}
	var f gpuTrailFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode gpu_trail_cases.json: %v", err)
	}
	if len(f.Trails) == 0 || len(f.Pools) == 0 {
		t.Fatal("gpu_trail_cases.json has no trails or pools")
	}
	return f
}

func mustFindTrailCase(t *testing.T, f gpuTrailFile, name string) trailCaseJSON {
	t.Helper()
	for _, c := range f.Trails {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("gpu_trail_cases.json has no trail %q", name)
	return trailCaseJSON{}
}

func mustFindPoolTrailCase(t *testing.T, f gpuTrailFile, name string) poolTrailCaseJSON {
	t.Helper()
	for _, c := range f.Pools {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("gpu_trail_cases.json has no pool %q", name)
	return poolTrailCaseJSON{}
}

func buildTrailFromCase(t *testing.T, c trailCaseJSON) *Trail {
	t.Helper()
	joint, ok := ParseJoint(c.Joint)
	if !ok {
		t.Fatalf("%s unknown joint %q", c.Name, c.Joint)
	}
	cfg, err := NewTrailConfig(c.MaxPoints, c.HeadWidth, c.TailWidth,
		colorOf(t, c.Start, c.Name+"/start"), colorOf(t, c.End, c.Name+"/end"), joint)
	if err != nil {
		t.Fatalf("%s NewTrailConfig: %v", c.Name, err)
	}
	tr, err := NewTrail(cfg)
	if err != nil {
		t.Fatalf("%s NewTrail: %v", c.Name, err)
	}
	for i, p := range c.Points {
		if len(p) != 2 {
			t.Fatalf("%s point %d has %d numbers, want 2", c.Name, i, len(p))
		}
		if err := tr.Push(core.V2(p[0], p[1])); err != nil {
			t.Fatalf("%s Push %d: %v", c.Name, i, err)
		}
	}
	return tr
}

func poolEmitterConfig(t *testing.T, c poolTrailCaseJSON) EmitterConfig {
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
	dir := vec2Of(t, c.Dir, c.Name+"/dir")
	var shape Shape
	var err error
	switch c.ShapeKind {
	case "point":
		shape, err = PointShape(dir)
	case "cone":
		shape, err = ConeShape(dir, c.AngleDeg*math.Pi/180)
	case "box":
		shape, err = BoxShape(dir, vec2Of(t, c.Extents, c.Name+"/extents"))
	case "ring":
		shape, err = RingShape(c.Inner, c.Outer)
	default:
		t.Fatalf("%s unknown shape kind %q", c.Name, c.ShapeKind)
	}
	if err != nil {
		t.Fatalf("%s shape: %v", c.Name, err)
	}
	return EmitterConfig{
		Origin: origin, Rate: c.Rate, Max: c.Max,
		SpeedMin: c.Speed[0], SpeedMax: c.Speed[1],
		LifeMin: core.Milliseconds(c.LifeMs[0]), LifeMax: core.Milliseconds(c.LifeMs[1]),
		Gravity: gravity, Start: start, End: end,
		Shape: shape, Turbulence: NoTurbulence(), Sub: NoSub(),
	}
}

func buildPoolFromCase(t *testing.T, c poolTrailCaseJSON) *GPUPool {
	t.Helper()
	joint, ok := ParseJoint(c.TrailJoint)
	if !ok {
		t.Fatalf("%s unknown trail joint %q", c.Name, c.TrailJoint)
	}
	tcfg, err := NewTrailConfig(c.TrailMax, c.TrailHead, c.TrailTail,
		core.RGBA(1, 1, 1, 1), core.RGBA(1, 1, 1, 0), joint)
	if err != nil {
		t.Fatalf("%s trail cfg: %v", c.Name, err)
	}
	p, err := NewGPUPool(poolEmitterConfig(t, c), c.Seed, tcfg)
	if err != nil {
		t.Fatalf("%s NewGPUPool: %v", c.Name, err)
	}
	return p
}

func trailApprox(t *testing.T, what string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s = %v, want %v", what, got, want)
	}
}

// A: widths taper head-ward, colors ramp head to tail, joints expand right.
func TestGPUTrailCasesFromFile(t *testing.T) {
	f := loadGPUTrailCases(t)
	for _, c := range f.Trails {
		tr := buildTrailFromCase(t, c)
		if tr.Len() != len(c.Points) {
			t.Errorf("%s Len = %d, want %d", c.Name, tr.Len(), len(c.Points))
		}
		if len(c.Points) == 0 {
			if verts, err := tr.Verts(); err != nil || verts != nil {
				t.Errorf("%s empty Verts = %v,%v, want nil,nil", c.Name, verts, err)
			}
			if segs, err := tr.Segments(); err != nil || segs != nil {
				t.Errorf("%s empty Segments = %v,%v, want nil,nil", c.Name, segs, err)
			}
			if _, err := tr.WidthAt(0); core.CodeOf(err) != core.CodeInvalidArg {
				t.Errorf("%s empty WidthAt err = %v, want invalid-arg", c.Name, err)
			}
			continue
		}
		if len(c.WantWidths) != len(c.Points) || len(c.WantColors) != len(c.Points) {
			t.Fatalf("%s golden widths/colors len mismatch", c.Name)
		}
		for i := range c.Points {
			w, err := tr.WidthAt(i)
			if err != nil {
				t.Fatalf("%s WidthAt %d: %v", c.Name, i, err)
			}
			trailApprox(t, c.Name+"/width", w, c.WantWidths[i])
			got, err := tr.ColorAt(i)
			if err != nil {
				t.Fatalf("%s ColorAt %d: %v", c.Name, i, err)
			}
			want := colorOf(t, c.WantColors[i], c.Name+"/want_color")
			if !got.ApproxEqual(want, 1e-9) {
				t.Errorf("%s color %d = %+v, want %+v", c.Name, i, got, want)
			}
		}
		verts, err := tr.Verts()
		if err != nil {
			t.Fatalf("%s Verts: %v", c.Name, err)
		}
		if len(verts) != c.WantVerts {
			t.Errorf("%s verts = %d, want %d", c.Name, len(verts), c.WantVerts)
		}
		if len(c.WantLefts) != len(verts) || len(c.WantRights) != len(verts) {
			t.Fatalf("%s golden verts len mismatch", c.Name)
		}
		for i, v := range verts {
			if math.IsNaN(v.Left.X) || math.IsNaN(v.Left.Y) || math.IsNaN(v.Right.X) || math.IsNaN(v.Right.Y) ||
				math.IsInf(v.Left.X, 0) || math.IsInf(v.Left.Y, 0) || math.IsInf(v.Right.X, 0) || math.IsInf(v.Right.Y, 0) {
				t.Fatalf("%s vert %d non-finite: %+v", c.Name, i, v)
			}
			trailApprox(t, c.Name+"/left.x", v.Left.X, c.WantLefts[i][0])
			trailApprox(t, c.Name+"/left.y", v.Left.Y, c.WantLefts[i][1])
			trailApprox(t, c.Name+"/right.x", v.Right.X, c.WantRights[i][0])
			trailApprox(t, c.Name+"/right.y", v.Right.Y, c.WantRights[i][1])
		}
		segs, err := tr.Segments()
		if err != nil {
			t.Fatalf("%s Segments: %v", c.Name, err)
		}
		if wantSegs := len(c.Points) - 1; len(segs) != wantSegs {
			t.Errorf("%s segments = %d, want %d", c.Name, len(segs), wantSegs)
		}
		for i, s := range segs {
			w0, _ := tr.WidthAt(i)
			w1, _ := tr.WidthAt(i + 1)
			if s.W0 != w0 || s.W1 != w1 {
				t.Errorf("%s seg %d widths = %v/%v, want %v/%v", c.Name, i, s.W0, s.W1, w0, w1)
			}
			d := s.P1.Sub(s.P0)
			if math.Abs(s.Angle-math.Atan2(d.Y, d.X)) > 1e-12 {
				t.Errorf("%s seg %d angle = %v", c.Name, i, s.Angle)
			}
		}
	}
	// Joint discriminator: same corner fans more pairs when round.
	miter := buildTrailFromCase(t, mustFindTrailCase(t, f, "corner_miter"))
	bevel := buildTrailFromCase(t, mustFindTrailCase(t, f, "corner_bevel"))
	round := buildTrailFromCase(t, mustFindTrailCase(t, f, "corner_round"))
	mv, _ := miter.Verts()
	bv, _ := bevel.Verts()
	rv, _ := round.Verts()
	if len(mv) != 3 || len(bv) != 3 {
		t.Errorf("corner miter/bevel verts = %d/%d, want 3/3", len(mv), len(bv))
	}
	if len(rv) != 3+RoundArcSteps+1 {
		t.Errorf("corner round verts = %d, want %d", len(rv), 3+RoundArcSteps+1)
	}
	// Spike: miter extension clamps to the bevel length past the limit.
	spike := buildTrailFromCase(t, mustFindTrailCase(t, f, "spike_miter"))
	sv, _ := spike.Verts()
	mid := sv[1]
	hw, _ := spike.WidthAt(1)
	if got := mid.Left.Sub(mid.Center).Length(); math.Abs(got-hw/2) > 1e-9 {
		t.Errorf("spike corner ext = %v, want bevel %v (limit %v)", got, hw/2, MiterLimit)
	}
	// Knife reads wide to narrow: head width above tail on every case.
	for _, c := range f.Trails {
		if len(c.Points) < 2 {
			continue
		}
		tr := buildTrailFromCase(t, c)
		head, _ := tr.WidthAt(tr.Len() - 1)
		tail, _ := tr.WidthAt(0)
		if head < tail {
			t.Errorf("%s head %v below tail %v, want wide-to-narrow", c.Name, head, tail)
		}
	}
}

// B: empty, zero, oversized, and corrupt inputs never crash.
func TestGPUTrailEdgesNoCrash(t *testing.T) {
	f := loadGPUTrailCases(t)
	empty := buildTrailFromCase(t, mustFindTrailCase(t, f, "empty"))
	if empty.Len() != 0 || empty.Points() != nil {
		t.Error("empty Len/Points non-zero")
	}
	if n, err := empty.AppendToBatch(sprite.NewBatch(), "tex/slash", core.NewRect(0, 0, 8, 8)); err != nil || n != 0 {
		t.Errorf("empty Append = %d,%v, want 0,nil", n, err)
	}
	if err := empty.Push(core.V2(math.NaN(), 0)); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("NaN push err = %v, want invalid-arg", err)
	}
	if empty.Len() != 0 {
		t.Error("NaN push stored a point")
	}
	if _, err := empty.WidthAt(-1); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("neg index err = %v, want invalid-arg", err)
	}
	if _, err := empty.ColorAt(3); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("oor index err = %v, want invalid-arg", err)
	}
	// Ring overflow drops the oldest, never grows past MaxPoints.
	cfg, _ := NewTrailConfig(4, 6, 2, core.White, core.Transparent, JointMiter)
	ring, err := NewTrail(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 9; i++ {
		if err := ring.Push(core.V2(float64(i), 0)); err != nil {
			t.Fatal(err)
		}
	}
	if ring.Len() != 4 {
		t.Fatalf("ring Len = %d, want 4", ring.Len())
	}
	if got := ring.Points()[0]; got != core.V2(5, 0) {
		t.Errorf("ring oldest = %+v, want (5,0)", got)
	}
	mut := ring.Points()
	mut[0] = core.V2(99999, 99999)
	if again := ring.Points(); again[0] == core.V2(99999, 99999) {
		t.Error("Points aliases the trail")
	}
	// Bad recipes report invalid-arg and store nothing.
	badCfgs := []struct {
		name string
		mk   func() (TrailConfig, error)
	}{
		{"zero-max", func() (TrailConfig, error) { return NewTrailConfig(0, 6, 2, core.White, core.Transparent, JointMiter) }},
		{"over-max", func() (TrailConfig, error) {
			return NewTrailConfig(MaxTrailPointsCap+1, 6, 2, core.White, core.Transparent, JointMiter)
		}},
		{"neg-head", func() (TrailConfig, error) { return NewTrailConfig(8, -1, 2, core.White, core.Transparent, JointMiter) }},
		{"nan-tail", func() (TrailConfig, error) {
			return NewTrailConfig(8, 6, math.NaN(), core.White, core.Transparent, JointMiter)
		}},
		{"bad-color", func() (TrailConfig, error) {
			return NewTrailConfig(8, 6, 2, core.RGBA(math.NaN(), 0, 0, 1), core.Transparent, JointMiter)
		}},
		{"bad-joint", func() (TrailConfig, error) {
			return NewTrailConfig(8, 6, 2, core.White, core.Transparent, JointKind(99))
		}},
	}
	for _, b := range badCfgs {
		if _, err := b.mk(); core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("%s err = %v, want invalid-arg", b.name, err)
		}
	}
	if _, err := NewTrail(TrailConfig{}); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("zero cfg err = %v, want invalid-arg", err)
	}
	if _, err := NewGPUPool(poolEmitterConfig(t, mustFindPoolTrailCase(t, f, "slash_small")), 1, TrailConfig{}); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("bad trail pool err = %v, want invalid-arg", err)
	}
	over := poolEmitterConfig(t, mustFindPoolTrailCase(t, f, "slash_small"))
	over.Max = MaxParticlesCap + 1
	tcfg, _ := NewTrailConfig(8, 6, 2, core.White, core.Transparent, JointMiter)
	if _, err := NewGPUPool(over, 1, tcfg); core.CodeOf(err) != core.CodeOutOfMemory {
		t.Errorf("over-max pool err = %v, want out-of-memory", err)
	}
	// Bad wiring reports invalid-arg and appends nothing.
	pool := buildPoolFromCase(t, mustFindPoolTrailCase(t, f, "slash_small"))
	b := sprite.NewBatch()
	if _, err := pool.AppendToBatch(nil, "tex/slash", core.NewRect(0, 0, 8, 8)); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil batch err = %v, want invalid-arg", err)
	}
	if _, err := pool.AppendToBatch(b, "", core.NewRect(0, 0, 8, 8)); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty image err = %v, want invalid-arg", err)
	}
	if _, err := pool.AppendToBatch(b, "tex/slash", core.NewRect(math.NaN(), 0, 8, 8)); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nan src err = %v, want invalid-arg", err)
	}
	if b.Len() != 0 {
		t.Errorf("bad wiring stored %d, want 0", b.Len())
	}
	if _, err := pool.Spawn(-2); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("Spawn(neg) err = %v, want invalid-arg", err)
	}
	// Nil receivers never panic.
	var nilT *Trail
	if err := nilT.Push(core.V2(1, 2)); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil Push err = %v, want invalid-arg", err)
	}
	if nilT.Len() != 0 || nilT.Points() != nil || nilT.Config() != (TrailConfig{}) {
		t.Error("nil trail getters non-zero")
	}
	if _, err := nilT.WidthAt(0); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil WidthAt err = %v, want invalid-arg", err)
	}
	if _, err := nilT.ColorAt(0); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil ColorAt err = %v, want invalid-arg", err)
	}
	if _, err := nilT.Verts(); err == nil {
		t.Error("nil Verts err nil, want invalid-arg")
	}
	if _, err := nilT.AppendToBatch(b, "tex/slash", core.NewRect(0, 0, 8, 8)); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil trail Append err = %v, want invalid-arg", err)
	}
	nilT.Clear()
	var nilP *GPUPool
	if n, err := nilP.Spawn(1); err == nil || n != 0 {
		t.Errorf("nil pool Spawn = %d,%v, want 0,error", n, err)
	}
	if s, d := nilP.Update(core.Milliseconds(16)); s != 0 || d != 0 {
		t.Errorf("nil pool Update = %d/%d, want 0/0", s, d)
	}
	if nilP.Alive() != 0 || nilP.Trails() != 0 || nilP.TotalPoints() != 0 || nilP.Particles() != nil || nilP.TrailOf(0.5) != nil {
		t.Error("nil pool getters non-zero")
	}
	nilP.Clear()
	if _, err := nilP.AppendToBatch(b, "tex/slash", core.NewRect(0, 0, 8, 8)); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil pool Append err = %v, want invalid-arg", err)
	}
	if got, want := JointMiter.String(), "miter"; got != want {
		t.Errorf("miter String = %q, want %q", got, want)
	}
	if _, ok := ParseJoint("zigzag"); ok {
		t.Error("ParseJoint(zigzag) ok true, want false")
	}
}

// C: pool tracks the CPU emitter bitwise; batch wiring replays identically.
func TestGPUPoolCPUParity(t *testing.T) {
	f := loadGPUTrailCases(t)
	for _, c := range f.Pools {
		cpu, err := NewEmitter(poolEmitterConfig(t, c), c.Seed)
		if err != nil {
			t.Fatalf("%s NewEmitter: %v", c.Name, err)
		}
		pool := buildPoolFromCase(t, c)
		if _, err := cpu.Spawn(c.Spawn); err != nil {
			t.Fatalf("%s cpu Spawn: %v", c.Name, err)
		}
		if _, err := pool.Spawn(c.Spawn); err != nil {
			t.Fatalf("%s pool Spawn: %v", c.Name, err)
		}
		cpu.Update(core.Milliseconds(c.UpdateMs))
		pool.Update(core.Milliseconds(c.UpdateMs))
		if pool.Spawned() != cpu.Spawned() || pool.Alive() != cpu.Alive() || pool.ChildSpawned() != cpu.ChildSpawned() {
			t.Fatalf("%s pool %d/%d/%d vs cpu %d/%d/%d", c.Name,
				pool.Spawned(), pool.Alive(), pool.ChildSpawned(), cpu.Spawned(), cpu.Alive(), cpu.ChildSpawned())
		}
		if pool.Spawned() != c.WantSpawn || pool.Alive() != c.WantAlive || pool.ChildSpawned() != c.WantChild {
			t.Errorf("%s golden spawned/alive/child = %d/%d/%d, want %d/%d/%d",
				c.Name, pool.Spawned(), pool.Alive(), pool.ChildSpawned(), c.WantSpawn, c.WantAlive, c.WantChild)
		}
		pp, pc := pool.Particles(), cpu.Particles()
		if len(pp) != len(pc) {
			t.Fatalf("%s particle len %d vs %d", c.Name, len(pp), len(pc))
		}
		for i := range pp {
			if pp[i] != pc[i] {
				t.Fatalf("%s particle %d diverged: %+v vs %+v", c.Name, i, pp[i], pc[i])
			}
		}
		if pool.Trails() != c.WantTrail {
			t.Errorf("%s trails = %d, want %d", c.Name, pool.Trails(), c.WantTrail)
		}
		if pool.Trails() != pool.Alive() {
			t.Errorf("%s trails %d vs alive %d, want equal", c.Name, pool.Trails(), pool.Alive())
		}
		if pool.TotalPoints() != c.WantPts {
			t.Errorf("%s points = %d, want %d", c.Name, pool.TotalPoints(), c.WantPts)
		}
		// Every live seed owns a ring ending at its particle.
		for _, pt := range pp {
			tr := pool.TrailOf(pt.Seed)
			if tr == nil || tr.Len() == 0 {
				t.Fatalf("%s seed %v has no trail", c.Name, pt.Seed)
			}
			pts := tr.Points()
			if last := pts[len(pts)-1]; last != pt.Pos {
				t.Fatalf("%s trail head %+v vs particle %+v", c.Name, last, pt.Pos)
			}
		}
		if pool.TrailOf(-1) != nil {
			t.Errorf("%s TrailOf(missing) non-nil", c.Name)
		}
		// Batch wiring replays byte-identical between two pools.
		mkBatch := func(p *GPUPool) []sprite.Sprite {
			bt := sprite.NewBatch()
			if _, err := p.AppendToBatch(bt, "tex/slash", core.NewRect(0, 0, 8, 8)); err != nil {
				t.Fatalf("%s Append: %v", c.Name, err)
			}
			var out []sprite.Sprite
			bt.Flush(func(_ core.AssetID, ss []sprite.Sprite) { out = append(out, ss...) })
			return out
		}
		again := buildPoolFromCase(t, c)
		if _, err := again.Spawn(c.Spawn); err != nil {
			t.Fatal(err)
		}
		again.Update(core.Milliseconds(c.UpdateMs))
		sa, sb := mkBatch(pool), mkBatch(again)
		if len(sa) != len(sb) {
			t.Fatalf("%s batch len %d vs %d", c.Name, len(sa), len(sb))
		}
		for i := range sa {
			if sa[i].Dst != sb[i].Dst || sa[i].Opacity != sb[i].Opacity {
				t.Fatalf("%s batch diverged at %d", c.Name, i)
			}
		}
	}
	// Emitted slices never alias the pool.
	c := mustFindPoolTrailCase(t, f, "slash_small")
	pool := buildPoolFromCase(t, c)
	if _, err := pool.Spawn(4); err != nil {
		t.Fatal(err)
	}
	pool.Update(core.Milliseconds(16))
	pa := pool.Particles()
	if len(pa) > 0 {
		pa[0].Pos = core.V2(99999, 99999)
		if again := pool.Particles(); again[0].Pos == core.V2(99999, 99999) {
			t.Error("Particles aliases the pool")
		}
		tr := pool.TrailOf(pa[0].Seed)
		if tr != nil {
			before := tr.Len()
			_ = tr.Push(core.V2(99999, 99999))
			if pool.TrailOf(pa[0].Seed).Len() != before {
				t.Error("TrailOf aliases the pool")
			}
		}
	}
}

// D: thousands of trail points update plus wiring with a measured cost.
func TestGPUPoolPerfThousands(t *testing.T) {
	f := loadGPUTrailCases(t)
	base := mustFindPoolTrailCase(t, f, "slash_small")
	// Synthetic load only (no golden): golden counts stay in
	// gpu_trail_cases.json. Opaque ramp so no point is skipped.
	cfg := poolEmitterConfig(t, base)
	cfg.Max = 4096
	cfg.Rate = 0
	cfg.Start = core.RGBA(1, 0.9, 0.4, 1)
	cfg.End = core.RGBA(1, 0.3, 0.1, 1)
	tcfg, err := NewTrailConfig(8, 8, 1, core.White, core.White, JointMiter)
	if err != nil {
		t.Fatal(err)
	}
	big, err := NewGPUPool(cfg, 20260915, tcfg)
	if err != nil {
		t.Fatalf("big NewGPUPool: %v", err)
	}
	const n = 3000
	start := time.Now()
	if got, err := big.Spawn(n); err != nil || got != n {
		t.Fatalf("Spawn 3000 = %d,%v", got, err)
	}
	spawnEl := time.Since(start)
	if big.Trails() != n {
		t.Fatalf("Trails = %d, want %d", big.Trails(), n)
	}
	t0 := time.Now()
	s, d := big.Update(core.Milliseconds(16))
	upEl := time.Since(t0)
	bt := sprite.NewBatch()
	t1 := time.Now()
	appended, err := big.AppendToBatch(bt, "tex/slash", core.NewRect(0, 0, 8, 8))
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	appendEl := time.Since(t1)
	t2 := time.Now()
	calls := bt.Flush(func(_ core.AssetID, _ []sprite.Sprite) {})
	flushEl := time.Since(t2)
	if s != 0 || d != 0 {
		t.Errorf("steady 16ms spawned/died = %d/%d, want 0/0", s, d)
	}
	if appended != big.TotalPoints() {
		t.Errorf("appended %d vs points %d, want equal", appended, big.TotalPoints())
	}
	if calls != 1 {
		t.Errorf("flush calls = %d, want 1", calls)
	}
	t.Logf("gpu-trail-3000: spawn %v update %v append %v flush %v (points=%d draws=%d)",
		spawnEl, upEl, appendEl, flushEl, big.TotalPoints(), calls)
}

// E: long runs neither grow nor diverge between replays.
func TestGPUTrailLongRunStable(t *testing.T) {
	f := loadGPUTrailCases(t)
	c := mustFindPoolTrailCase(t, f, "slash_small")
	mk := func() *GPUPool {
		p := buildPoolFromCase(t, c)
		if _, err := p.Spawn(60); err != nil {
			t.Fatalf("Spawn: %v", err)
		}
		return p
	}
	first := mk()
	first.Update(core.Milliseconds(16))
	wantPts := first.Particles()
	wantTotal := first.TotalPoints()
	for i := 0; i < 2000; i++ {
		p := mk()
		p.Update(core.Milliseconds(16))
		got := p.Particles()
		if len(got) != len(wantPts) {
			t.Fatalf("rep %d len %d vs %d", i, len(got), len(wantPts))
		}
		for j := range got {
			if got[j] != wantPts[j] {
				t.Fatalf("rep %d diverged at %d", i, j)
			}
		}
		if p.TotalPoints() != wantTotal {
			t.Fatalf("rep %d points %d vs %d", i, p.TotalPoints(), wantTotal)
		}
	}
	// Budget never grows: repeated fill plus drain returns to baseline.
	p := buildPoolFromCase(t, c)
	for i := 0; i < 200; i++ {
		if _, err := p.Spawn(50); err != nil {
			t.Fatalf("cycle %d Spawn: %v", i, err)
		}
		p.Update(core.Milliseconds(500))
		if p.Alive() > c.Max {
			t.Fatalf("cycle %d alive %d over max %d", i, p.Alive(), c.Max)
		}
		if p.Trails() > p.Alive() {
			t.Fatalf("cycle %d trails %d over alive %d", i, p.Trails(), p.Alive())
		}
		p.Clear()
		if p.Alive() != 0 || p.Trails() != 0 || p.TotalPoints() != 0 || p.Spawned() != 0 {
			t.Fatalf("cycle %d not back to zero", i)
		}
	}
	// Ring cap holds under sustained pushes.
	trail := buildTrailFromCase(t, mustFindTrailCase(t, f, "straight_miter"))
	for i := 0; i < 600; i++ {
		if err := trail.Push(core.V2(float64(i), 0)); err != nil {
			t.Fatal(err)
		}
	}
	if trail.Len() != 8 {
		t.Fatalf("capped Len = %d, want 8", trail.Len())
	}
	if got := trail.Points()[0]; got != core.V2(592, 0) {
		t.Errorf("capped oldest = %+v, want (592,0)", got)
	}
}

// F: knife taper plus joint shape stand in for the window (slash_small).
// The frozen wants in gpu_trail_cases.json are the evidence both backends
// share; shape assertions below pin the meaning, not just the numbers.
func TestGPUTrailOffscreenGolden(t *testing.T) {
	f := loadGPUTrailCases(t)
	// Head color is Start, tail fades to End, middle lerps halfway.
	straight := buildTrailFromCase(t, mustFindTrailCase(t, f, "straight_miter"))
	head, _ := straight.ColorAt(straight.Len() - 1)
	tail, _ := straight.ColorAt(0)
	if !head.ApproxEqual(core.RGBA(1, 0.8, 0.2, 1), 1e-9) {
		t.Errorf("head color = %+v, want start", head)
	}
	if !tail.ApproxEqual(core.RGBA(1, 0.2, 0.1, 0), 1e-9) {
		t.Errorf("tail color = %+v, want end", tail)
	}
	// Taper is monotone narrowing toward the tail.
	prev := math.Inf(1)
	for i := straight.Len() - 1; i >= 0; i-- {
		w, _ := straight.WidthAt(i)
		if w > prev {
			t.Errorf("taper broke at %d: %v after %v", i, w, prev)
		}
		prev = w
	}
	// Pool golden wants are frozen in the file.
	for _, c := range f.Pools {
		p := buildPoolFromCase(t, c)
		if _, err := p.Spawn(c.Spawn); err != nil {
			t.Fatal(err)
		}
		p.Update(core.Milliseconds(c.UpdateMs))
		if p.Spawned() != c.WantSpawn || p.Alive() != c.WantAlive || p.ChildSpawned() != c.WantChild {
			t.Errorf("%s golden spawned/alive/child = %d/%d/%d, want %d/%d/%d",
				c.Name, p.Spawned(), p.Alive(), p.ChildSpawned(), c.WantSpawn, c.WantAlive, c.WantChild)
		}
		if p.Trails() != c.WantTrail || p.TotalPoints() != c.WantPts {
			t.Errorf("%s golden trails/points = %d/%d, want %d/%d",
				c.Name, p.Trails(), p.TotalPoints(), c.WantTrail, c.WantPts)
		}
	}
	// Trail wiring draws through one batch call (the window intent
	// game_particle --case=trail batches every track in one submit).
	p := buildPoolFromCase(t, mustFindPoolTrailCase(t, f, "slash_small"))
	if _, err := p.Spawn(8); err != nil {
		t.Fatal(err)
	}
	p.Update(core.Milliseconds(16))
	bt := sprite.NewBatch()
	n, err := p.AppendToBatch(bt, "tex/slash", core.NewRect(0, 0, 8, 8))
	if err != nil || n == 0 {
		t.Fatalf("slash Append = %d,%v", n, err)
	}
	if got := bt.Flush(func(core.AssetID, []sprite.Sprite) {}); got != 1 {
		t.Errorf("slash Flush = %d, want 1", got)
	}
}
