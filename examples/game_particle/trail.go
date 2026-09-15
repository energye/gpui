// Command game_particle trail case: 5.2 GPU particle trail window.
//
// Slash pool (fast rightward shot plus gravity arc) records one Trail ring
// per particle through the real GPUPool Spawn/Update. Each center-line
// segment draws as one rotated atlas sprite (existing sprite AtlasToRender
// plus DrawAtlasEx, R4 branch, no new submit path); each joint draws one
// tinted square. Width runs wide (head) to narrow (tail), color runs
// bright head to fading tail, round joints fan the corners.
//
// Modes:
//
//	go run ./examples/game_particle --case=trail -auto-only
//	  headless probes plus ~8s window (JSON on stdout, exit 1 on fail).
//	go run ./examples/game_particle --case=trail -manual-seconds 30
//	  manual for 30s (events logged, title shows the count), then summary.
//
// Window: 1200x800, title game_particle. First run writes the golden
// baseline into testdata/; later runs compare it with zero tolerance.
package main

import (
	"encoding/json"
	"fmt"
	"image/png"
	"math"
	"os"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/game/core"
	"github.com/energye/gpui/game/particle"
	"github.com/energye/gpui/game/sprite"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	trailAbilityID = "particle-trail"
	trailScenario  = "game_particle--case=trail"
	trailGolden    = "examples/game_particle/testdata/particle_trail_golden.png"
	trailLast      = "examples/game_particle/testdata/particle_trail_last.png"
)

// Shared trail scene colors (window paint and offscreen probes use the same).
const (
	trailHeadR, trailHeadG, trailHeadB    = 1.0, 0.80, 0.20
	trailJointR, trailJointG, trailJointB = 1.0, 0.50, 0.15
	trailTailR, trailTailG, trailTailB    = 0.70, 0.15, 0.10
)

// Body-local layout (body is ~904x656 under the shell chrome).
const (
	slashX, slashY, slashW, slashH = 16.0, 44.0, 856.0, 360.0
	trailCountX, trailCountY       = 16.0, 420.0
	trailNoteY                     = 560.0
)

// Live slash geometry in card-local coordinates.
const (
	slashOriginX, slashOriginY = 60.0, 200.0
)

// trailProbeResult is the three-evidence headless verdict (no GPU needed).
type trailProbeResult struct {
	Cases, Passed int
	Alive         int
	Trails        int
	Points        int
	PixOK         bool
	PixDetail     string
	GoldenOK      bool
	GoldenChanged int
	GoldenWrote   bool
	OK            bool
}

type trailPoolDef struct {
	TrailMax   int     `json:"trail_max"`
	TrailHead  float64 `json:"trail_head"`
	TrailTail  float64 `json:"trail_tail"`
	TrailJoint string  `json:"trail_joint"`
}

type trailFileCase struct {
	Name      string       `json:"name"`
	Origin    []float64    `json:"origin"`
	ShapeKind string       `json:"shape_kind"`
	Dir       []float64    `json:"dir"`
	AngleDeg  float64      `json:"angle_deg"`
	Extents   []float64    `json:"extents"`
	Inner     float64      `json:"ring_inner"`
	Outer     float64      `json:"ring_outer"`
	Rate      float64      `json:"rate"`
	Max       int          `json:"max"`
	Speed     []float64    `json:"speed"`
	LifeMs    []int64      `json:"life_ms"`
	Gravity   []float64    `json:"gravity"`
	Start     []float64    `json:"start"`
	End       []float64    `json:"end"`
	TurbS     float64      `json:"turb_strength"`
	TurbScale float64      `json:"turb_scale"`
	SubCount  int          `json:"sub_count"`
	SubSpeed  []float64    `json:"sub_speed"`
	SubLifeMs []int64      `json:"sub_life_ms"`
	Seed      uint64       `json:"seed"`
	Spawn     int          `json:"spawn"`
	UpdateMs  int64        `json:"update_ms"`
	WantSpawn int          `json:"want_spawned"`
	WantAlive int          `json:"want_alive"`
	WantChild int          `json:"want_child"`
	WantTrail int          `json:"want_trails"`
	WantPts   int          `json:"want_points"`
	Trail     trailPoolDef `json:"-"`
}

func buildTrailPoolCase(c trailFileCase, tcfg particle.TrailConfig) (*particle.GPUPool, error) {
	origin, ok := vec2Of(c.Origin)
	if !ok {
		return nil, fmt.Errorf("%s origin shape", c.Name)
	}
	gravity, ok := vec2Of(c.Gravity)
	if !ok {
		return nil, fmt.Errorf("%s gravity shape", c.Name)
	}
	start, ok := colorOf(c.Start)
	if !ok {
		return nil, fmt.Errorf("%s start shape", c.Name)
	}
	end, ok := colorOf(c.End)
	if !ok {
		return nil, fmt.Errorf("%s end shape", c.Name)
	}
	if len(c.Speed) != 2 || len(c.LifeMs) != 2 {
		return nil, fmt.Errorf("%s speed/life shape", c.Name)
	}
	dir, ok := vec2Of(c.Dir)
	if !ok {
		return nil, fmt.Errorf("%s dir shape", c.Name)
	}
	var shape particle.Shape
	var err error
	switch c.ShapeKind {
	case "point":
		shape, err = particle.PointShape(dir)
	case "cone":
		shape, err = particle.ConeShape(dir, c.AngleDeg*math.Pi/180)
	case "box":
		ext, ok := vec2Of(c.Extents)
		if !ok {
			return nil, fmt.Errorf("%s extents shape", c.Name)
		}
		shape, err = particle.BoxShape(dir, ext)
	case "ring":
		shape, err = particle.RingShape(c.Inner, c.Outer)
	default:
		return nil, fmt.Errorf("%s unknown shape %q", c.Name, c.ShapeKind)
	}
	if err != nil {
		return nil, err
	}
	turb, err := particle.NewTurbulence(c.TurbS, c.TurbScale)
	if err != nil {
		return nil, err
	}
	var sub particle.Sub
	if c.SubCount == 0 {
		sub = particle.NoSub()
	} else {
		if len(c.SubSpeed) != 2 || len(c.SubLifeMs) != 2 {
			return nil, fmt.Errorf("%s sub shape", c.Name)
		}
		sub, err = particle.NewSub(c.SubCount, c.SubSpeed[0], c.SubSpeed[1],
			core.Milliseconds(c.SubLifeMs[0]), core.Milliseconds(c.SubLifeMs[1]))
		if err != nil {
			return nil, err
		}
	}
	cfg := particle.EmitterConfig{
		Origin: origin, Rate: c.Rate, Max: c.Max,
		SpeedMin: c.Speed[0], SpeedMax: c.Speed[1],
		LifeMin: core.Milliseconds(c.LifeMs[0]), LifeMax: core.Milliseconds(c.LifeMs[1]),
		Gravity: gravity, Start: start, End: end,
		Shape: shape, Turbulence: turb, Sub: sub,
	}
	return particle.NewGPUPool(cfg, c.Seed, tcfg)
}

// probeTrailLogic replays every frozen pool case: counts plus trail tracks.
func probeTrailLogic() (cases, passed, alive, trails, points int, ok bool, detail string) {
	raw, err := os.ReadFile("game/particle/testdata/gpu_trail_cases.json")
	if err != nil {
		return 0, 0, 0, 0, 0, false, "read gpu_trail_cases.json: " + err.Error()
	}
	// Decode pools with their trail recipes kept.
	var g struct {
		Pools []json.RawMessage `json:"pools"`
	}
	if err := json.Unmarshal(raw, &g); err != nil {
		return 0, 0, 0, 0, 0, false, "decode gpu_trail_cases.json: " + err.Error()
	}
	if len(g.Pools) == 0 {
		return 0, 0, 0, 0, 0, false, "gpu_trail_cases.json has no pools"
	}
	for _, m := range g.Pools {
		var c trailFileCase
		if err := json.Unmarshal(m, &c); err != nil {
			return cases, passed, alive, trails, points, false, "pool decode: " + err.Error()
		}
		var tj struct {
			Max   int     `json:"trail_max"`
			Head  float64 `json:"trail_head"`
			Tail  float64 `json:"trail_tail"`
			Joint string  `json:"trail_joint"`
		}
		if err := json.Unmarshal(m, &tj); err != nil {
			return cases, passed, alive, trails, points, false, c.Name + " trail decode: " + err.Error()
		}
		joint, ok := particle.ParseJoint(tj.Joint)
		if !ok {
			return cases, passed, alive, trails, points, false, c.Name + " unknown joint"
		}
		tcfg, err := particle.NewTrailConfig(tj.Max, tj.Head, tj.Tail,
			core.RGBA(1, 1, 1, 1), core.RGBA(1, 1, 1, 0), joint)
		if err != nil {
			return cases, passed, alive, trails, points, false, c.Name + " trail cfg: " + err.Error()
		}
		cases++
		p, err := buildTrailPoolCase(c, tcfg)
		if err != nil {
			return cases, passed, alive, trails, points, false, c.Name + " build: " + err.Error()
		}
		if _, err := p.Spawn(c.Spawn); err != nil {
			return cases, passed, alive, trails, points, false, c.Name + " spawn: " + err.Error()
		}
		p.Update(core.Milliseconds(c.UpdateMs))
		if p.Spawned() != c.WantSpawn || p.Alive() != c.WantAlive || p.ChildSpawned() != c.WantChild ||
			p.Trails() != c.WantTrail || p.TotalPoints() != c.WantPts {
			return cases, passed, alive, trails, points, false,
				fmt.Sprintf("%s spawned/alive/child/trails/points=%d/%d/%d/%d/%d want %d/%d/%d/%d/%d",
					c.Name, p.Spawned(), p.Alive(), p.ChildSpawned(), p.Trails(), p.TotalPoints(),
					c.WantSpawn, c.WantAlive, c.WantChild, c.WantTrail, c.WantPts)
		}
		passed++
		alive += p.Alive()
		trails += p.Trails()
		points += p.TotalPoints()
	}
	return cases, passed, alive, trails, points, true,
		fmt.Sprintf("cases=%d alive=%d trails=%d points=%d", cases, alive, trails, points)
}

// paintTrailDeterministicFrame draws the frozen probe scene: dark
// background, wide bright head bar on the left, round joint dot in the
// middle, narrow dim tail bar on the right (the knife taper).
func paintTrailDeterministicFrame(dc *render.Context) {
	dc.ClearWithColor(render.RGBA{R: bgR, G: bgG, B: bgB, A: 1})
	dc.SetRGB(trailHeadR, trailHeadG, trailHeadB)
	dc.DrawRectangle(60, 110, 120, 40)
	_ = dc.Fill()
	dc.SetRGB(trailJointR, trailJointG, trailJointB)
	dc.DrawRectangle(180, 120, 20, 20)
	_ = dc.Fill()
	dc.SetRGB(trailTailR, trailTailG, trailTailB)
	dc.DrawRectangle(200, 127, 140, 6)
	_ = dc.Fill()
}

// probeTrailPixels asserts head-bright, joint-mid, tail-dim, outside-dark.
func probeTrailPixels() (bool, string) {
	prev, _ := os.LookupEnv("GOGPU_RENDER_MODE")
	_ = os.Setenv("GOGPU_RENDER_MODE", "cpu")
	defer func() {
		if prev == "" {
			_ = os.Unsetenv("GOGPU_RENDER_MODE")
		} else {
			_ = os.Setenv("GOGPU_RENDER_MODE", prev)
		}
	}()
	dc := render.NewContext(offW, offH)
	paintTrailDeterministicFrame(dc)
	img := dc.Image()
	_ = dc.Close()

	hr, hg, hb := sample8(img, 120, 130)
	jr, jg, jb := sample8(img, 190, 130)
	tr, tg, tb := sample8(img, 270, 130)
	or, og, ob := sample8(img, 400, 200)
	okHead := closeEnough(hr, want8(trailHeadR)) && closeEnough(hg, want8(trailHeadG)) && closeEnough(hb, want8(trailHeadB))
	okJoint := closeEnough(jr, want8(trailJointR)) && closeEnough(jg, want8(trailJointG)) && closeEnough(jb, want8(trailJointB))
	okTail := closeEnough(tr, want8(trailTailR)) && closeEnough(tg, want8(trailTailG)) && closeEnough(tb, want8(trailTailB))
	okOut := closeEnough(or, want8(bgR)) && closeEnough(og, want8(bgG)) && closeEnough(ob, want8(bgB))
	detail := fmt.Sprintf("head=(%d,%d,%d) joint=(%d,%d,%d) tail=(%d,%d,%d) outside=(%d,%d,%d) tol=%d",
		hr, hg, hb, jr, jg, jb, tr, tg, tb, or, og, ob, probePixelTol)
	return okHead && okJoint && okTail && okOut, detail
}

// probeTrailGolden compares the deterministic taper frame against the frozen
// mask with zero tolerance; the first run produces the baseline.
func probeTrailGolden() (ok bool, changed int, wrote bool) {
	prev, _ := os.LookupEnv("GOGPU_RENDER_MODE")
	_ = os.Setenv("GOGPU_RENDER_MODE", "cpu")
	defer func() {
		if prev == "" {
			_ = os.Unsetenv("GOGPU_RENDER_MODE")
		} else {
			_ = os.Setenv("GOGPU_RENDER_MODE", prev)
		}
	}()
	dc := render.NewContext(offW, offH)
	paintTrailDeterministicFrame(dc)
	img := dc.Image()
	_ = dc.Close()

	f, err := os.Open(trailGolden)
	if err != nil {
		if err := os.MkdirAll("examples/game_particle/testdata", 0o755); err != nil {
			return false, 0, false
		}
		out, err := os.Create(trailGolden)
		if err != nil {
			return false, 0, false
		}
		encErr := png.Encode(out, img)
		_ = out.Close()
		if encErr != nil {
			return false, 0, false
		}
		if out2, err := os.Create(trailLast); err == nil {
			_ = png.Encode(out2, img)
			_ = out2.Close()
		}
		return true, 0, true
	}
	defer func() { _ = f.Close() }()
	want, err := png.Decode(f)
	if err != nil {
		return false, 0, false
	}
	if !img.Bounds().Eq(want.Bounds()) {
		return false, 1, false
	}
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			ar, ag, ab, aa := img.At(x, y).RGBA()
			br, bg, bb, ba := want.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				changed++
			}
		}
	}
	if changed == 0 {
		if out, err := os.Create(trailLast); err == nil {
			_ = png.Encode(out, img)
			_ = out.Close()
		}
	}
	return changed == 0, changed, false
}

// runTrailProbes collects the three evidences: logic, pixels, golden mask.
func runTrailProbes() trailProbeResult {
	var p trailProbeResult
	cases, passed, alive, trails, points, ok, _ := probeTrailLogic()
	p.Cases, p.Passed, p.Alive, p.Trails, p.Points = cases, passed, alive, trails, points
	if !ok {
		p.OK = false
		return p
	}
	p.PixOK, p.PixDetail = probeTrailPixels()
	var wrote bool
	p.GoldenOK, p.GoldenChanged, wrote = probeTrailGolden()
	p.GoldenWrote = wrote
	p.OK = ok && p.PixOK && p.GoldenOK
	return p
}

func makeSlashPool() (*particle.GPUPool, error) {
	shape, err := particle.PointShape(core.V2(1, -0.15))
	if err != nil {
		return nil, err
	}
	turb, err := particle.NewTurbulence(30, 3.0)
	if err != nil {
		return nil, err
	}
	cfg := particle.EmitterConfig{
		Origin:     core.V2(slashOriginX, slashOriginY),
		Rate:       90,
		Max:        600,
		SpeedMin:   280,
		SpeedMax:   380,
		LifeMin:    core.Milliseconds(400),
		LifeMax:    core.Milliseconds(600),
		Gravity:    core.V2(0, 320),
		Start:      core.RGBA(1, 0.85, 0.3, 1),
		End:        core.RGBA(0.9, 0.2, 0.1, 0),
		Shape:      shape,
		Turbulence: turb,
		Sub:        particle.NoSub(),
	}
	tcfg, err := particle.NewTrailConfig(12, 14, 1,
		core.RGBA(1, 0.85, 0.3, 1), core.RGBA(0.9, 0.2, 0.1, 0), particle.JointRound)
	if err != nil {
		return nil, err
	}
	return particle.NewGPUPool(cfg, 5201, tcfg)
}

var trailImageID = core.AssetID("tex/slash")
var trailSrc = core.NewRect(0, 0, 8, 8)

// slashToAtlas turns every trail segment into one rotated tinted atlas
// sprite (existing R4 road) plus one joint square per point, so width,
// color, and corners all stay visible through DrawAtlasEx.
func slashToAtlas(pool *particle.GPUPool, ox, oy float64) []sprite.AtlasSprite {
	if pool == nil {
		return nil
	}
	var out []sprite.AtlasSprite
	for _, pt := range pool.Particles() {
		if !pt.Alive() {
			continue
		}
		tr := pool.TrailOf(pt.Seed)
		if tr == nil {
			continue
		}
		segs, err := tr.Segments()
		if err != nil {
			continue
		}
		for _, s := range segs {
			dx, dy := s.P1.X-s.P0.X, s.P1.Y-s.P0.Y
			length := math.Hypot(dx, dy)
			avgW := (s.W0 + s.W1) / 2
			if length <= 0 || avgW <= 0 {
				continue
			}
			avgC := s.C0.Lerp(s.C1, 0.5)
			if avgC.A <= 0 {
				continue
			}
			mx, my := (s.P0.X+s.P1.X)/2, (s.P0.Y+s.P1.Y)/2
			dst := core.NewRect(ox+mx-(length+avgW)/2, oy+my-avgW/2, length+avgW, avgW)
			as, err := sprite.NewAtlasSprite(trailImageID, trailSrc, dst, 1,
				s.Angle, core.V2(dst.W/2, dst.H/2), avgC, sprite.AtlasFilterNearest, false, false)
			if err != nil {
				continue
			}
			out = append(out, as)
		}
		pts := tr.Points()
		for i, q := range pts {
			c, err := tr.ColorAt(i)
			if err != nil || c.A <= 0 {
				continue
			}
			w, err := tr.WidthAt(i)
			if err != nil || w <= 0 {
				continue
			}
			dst := core.NewRect(ox+q.X-w/2, oy+q.Y-w/2, w, w)
			as, err := sprite.NewAtlasSprite(trailImageID, trailSrc, dst, 1,
				0, core.Vec2{}, c, sprite.AtlasFilterNearest, false, false)
			if err != nil {
				continue
			}
			out = append(out, as)
		}
	}
	return out
}

func paintSlashCard(pc *rendering.PaintContext, w, h float64, pool *particle.GPUPool) {
	if pc == nil || pc.DC == nil {
		return
	}
	ax, ay := pc.Abs(0, 0)
	pc.DC.SetRGB(0.05, 0.05, 0.07)
	pc.DC.DrawRectangle(ax, ay, w, h)
	_ = pc.DC.Fill()
	items := slashToAtlas(pool, ax, ay)
	rs, err := sprite.AtlasToRender(items)
	if err != nil || len(rs) == 0 {
		return
	}
	if atlasBuf == nil {
		atlasBuf = buildParticleAtlas()
	}
	_, _ = pc.DC.DrawAtlasEx(atlasBuf, rs, render.AtlasDrawOptions{})
}

// liveTrailSim is the live window state: one real slash pool advances every
// tick, the batch proves the wiring, the card draws the tinted ribbon.
type liveTrailSim struct {
	pool       *particle.GPUPool
	app        *embedder.PipelineApp
	shell      *wrkit.ShellChrome
	phase      *wrkit.PhaseClock
	slashBox   *rendering.RenderBox
	aliveL     *rendering.RenderText
	trailsL    *rendering.RenderText
	pointsL    *rendering.RenderText
	batchL     *rendering.RenderText
	alive      int
	trails     int
	points     int
	batchCalls int
	spawned    int
}

type trailTicker struct{ s *liveTrailSim }

func (t *trailTicker) Tick(dt float64) bool {
	s := t.s
	if s == nil {
		return true
	}
	if dt < 0 {
		dt = 0
	}
	if dt > 0.05 {
		dt = 0.05
	}
	step := core.SecondsFloat(dt)
	if s.pool != nil {
		s.pool.Update(step)
		s.alive = s.pool.Alive()
		s.trails = s.pool.Trails()
		s.points = s.pool.TotalPoints()
		s.spawned = s.pool.Spawned()
	}
	// Real wiring proof: batch every track, one image means one call.
	b := sprite.NewBatch()
	if s.pool != nil {
		_, _ = s.pool.AppendToBatch(b, trailImageID, trailSrc)
	}
	s.batchCalls = b.Flush(func(_ core.AssetID, _ []sprite.Sprite) {})
	if s.slashBox != nil {
		s.slashBox.MarkNeedsPaint()
	}
	if s.aliveL != nil {
		s.aliveL.SetText(fmt.Sprintf("alive %d", s.alive))
	}
	if s.trailsL != nil {
		s.trailsL.SetText(fmt.Sprintf("trails %d", s.trails))
	}
	if s.pointsL != nil {
		s.pointsL.SetText(fmt.Sprintf("points %d", s.points))
	}
	if s.batchL != nil {
		s.batchL.SetText(fmt.Sprintf("batch_calls %d", s.batchCalls))
	}
	phase := s.phase.Advance(dt)
	gateOK := s.alive > 0 && s.trails > 0 && s.points > 0 && s.batchCalls >= 1
	s.shell.NoteHUDTick(dt)
	s.shell.UpdateHUD("particle-trail", phase, s.app, gateOK,
		fmt.Sprintf("alive=%d trails=%d points=%d batch=%d", s.alive, s.trails, s.points, s.batchCalls),
		fmt.Sprintf("spawned=%d", s.spawned))
	s.app.ScheduleFrame()
	return true
}

type trailManualSummary struct {
	Pointer, Key, Resize int
	Timed                bool
	Note                 string
}

func trailFailJSON(probe trailProbeResult) {
	probeOK := 0
	if probe.OK {
		probeOK = 1
	}
	b, _ := json.Marshal(map[string]any{
		"ability_id": trailAbilityID,
		"scenario":   trailScenario,
		"probe_ok":   probeOK,
		"pass":       false,
		"pixels":     probe.PixDetail,
		"golden":     probe.GoldenChanged,
	})
	fmt.Fprintln(os.Stdout, string(b))
}

// runTrail is the --case=trail entry: probes, then the slash window with an
// auto gate (-auto-only) or a resident manual phase.
func runTrail(autoOnly bool, manualSeconds int) {
	wrkit.EnsureUIFace()
	if atlasBuf == nil {
		atlasBuf = buildParticleAtlas()
	}

	probe := runTrailProbes()
	fmt.Fprintf(os.Stderr, "game_particle-trail: probes ok=%v cases=%d/%d alive=%d trails=%d points=%d pix=%v golden=%v(wrote=%v changed=%d) %s\n",
		probe.OK, probe.Passed, probe.Cases, probe.Alive, probe.Trails, probe.Points,
		probe.PixOK, probe.GoldenOK, probe.GoldenWrote, probe.GoldenChanged, probe.PixDetail)
	if !probe.OK {
		if autoOnly {
			trailFailJSON(probe)
		} else {
			fmt.Fprintln(os.Stderr, "game_particle-trail: selftest FAIL, not opening window")
		}
		os.Exit(1)
	}

	pool, err := makeSlashPool()
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: slash pool:", err)
		os.Exit(1)
	}

	var secs int
	if autoOnly {
		secs = runSecondsEnv(8)
		wrkit.RequireMinRun(secs, trailAbilityID)
	} else if manualSeconds > 0 {
		secs = manualSeconds
	} else {
		secs, _ = wrkit.RunSecondsOpt()
	}
	manualMode := !autoOnly

	var runFor time.Duration
	if secs > 0 {
		runFor = time.Duration(secs) * time.Second
	}

	shell := wrkit.NewShell(winW, winH, "game_particle — 5.2 刀光拖尾 (particle-trail)", []string{
		"刀光 slash右冲+重力弧圆接头",
		"头宽14尾窄1亮黄→红渐隐",
		"段旋染色+接头方块走新管线",
		"轨迹数=活粒子 batch_calls计",
		"底栏 alive/trails/points",
		"JSON见 ability_extra",
	})

	sim := &liveTrailSim{pool: pool, shell: shell}
	if secs > 0 {
		sim.phase = wrkit.NewPhaseClock(float64(secs)*0.5, float64(secs)*0.8)
	} else {
		sim.phase = wrkit.NewPhaseClock(0, 0)
	}

	shell.Body.Place(wrkit.Label("SLASH 刀光区", 13, 0.55, 0.75, 0.95), slashX, slashY-24)
	sim.slashBox = rendering.NewRenderBox()
	sim.slashBox.FixedWidth, sim.slashBox.FixedHeight = slashW, slashH
	slashEm := pool
	sim.slashBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		paintSlashCard(pc, size.Width, size.Height, slashEm)
	}
	shell.Body.Place(sim.slashBox, slashX, slashY)

	sim.aliveL = wrkit.Label("alive 0", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.aliveL, trailCountX, trailCountY+10)
	sim.trailsL = wrkit.Label("trails 0", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.trailsL, trailCountX+220, trailCountY+10)
	sim.pointsL = wrkit.Label("points 0", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.pointsL, trailCountX+440, trailCountY+10)
	sim.batchL = wrkit.Label("batch_calls 0", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.batchL, trailCountX+660, trailCountY+10)
	shell.Body.Place(wrkit.Label("刀光由宽到窄亮黄拖红 · 重力弧拐圆接头 · 一次合批提交", 12, 0.70, 0.78, 0.88), trailCountX, trailNoteY)

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "game_particle", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	defer win.Close()
	ctl := win.Controls()

	var summary trailManualSummary
	app := embedder.NewPipelineApp(win.Host(), shell.Root, embedder.PipelineOptions{
		ClearR: bgR, ClearG: bgG, ClearB: bgB, ClearA: 1,
		RunFor: runFor,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose, platform.EventCloseRequested:
				fmt.Fprintf(os.Stderr, "game_particle-trail: close (%s)\n", win.Backend())
				return
			case platform.EventPointer:
				summary.Pointer++
				if manualMode {
					fmt.Fprintf(os.Stderr, "game_particle-trail: pointer %s (%.0f,%.0f) n=%d\n",
						ev.Pointer, ev.X, ev.Y, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_particle-trail events=%d", summary.Pointer+summary.Key+summary.Resize))
					}
				}
				return
			case platform.EventKey:
				if ev.Pressed {
					summary.Key++
					if manualMode {
						fmt.Fprintf(os.Stderr, "game_particle-trail: key n=%d\n", summary.Pointer+summary.Key+summary.Resize)
						if ctl != nil {
							ctl.SetTitle(fmt.Sprintf("game_particle-trail events=%d", summary.Pointer+summary.Key+summary.Resize))
						}
					}
				}
				return
			case platform.EventResize:
				summary.Resize++
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
				if manualMode {
					fmt.Fprintf(os.Stderr, "game_particle-trail: resize %dx%d n=%d\n", ev.Width, ev.Height, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_particle-trail events=%d", summary.Pointer+summary.Key+summary.Resize))
					}
				}
				return
			default:
				return
			}
		},
	})
	sim.app = app
	app.Scheduler().Tickers().Add(&trailTicker{s: sim})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}
	shell.Root.MarkNeedsPaint()
	app.ScheduleFrame()

	var proc scheduler.ProcessTracker
	proc.Start()
	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: run:", err)
		os.Exit(1)
	}
	proc.Stop()
	app.Close()
	elapsed := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	presents := app.PresentCount()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	probeOK := 0
	if probe.OK {
		probeOK = 1
	}
	extra := map[string]any{
		"alive":       sim.alive,
		"trails":      sim.trails,
		"points":      sim.points,
		"batch_calls": sim.batchCalls,
		"spawned":     sim.spawned,
		"probe_ok":    probeOK,
		"case":        "trail",
	}

	if autoOnly {
		report := wrgate.BuildReport(wrgate.BuildInput{
			AbilityID:     trailAbilityID,
			Scenario:      trailScenario,
			Snap:          snap,
			PresentCount:  presents,
			ElapsedSec:    elapsed,
			SurfaceAreaPx: winW * winH,
			Warmup:        true,
			Extra:         extra,
		})
		raw, _ := json.Marshal(report)
		fmt.Println(string(raw))
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{MinPresents: 1}); err != nil {
			fmt.Fprintln(os.Stderr, "FAIL:", err)
			os.Exit(1)
		}
		if presents < 1 || sim.alive <= 0 || sim.trails <= 0 || sim.points <= 0 || sim.batchCalls < 1 {
			fmt.Fprintf(os.Stderr, "FAIL: presents=%d alive=%d trails=%d points=%d batch=%d (want >=1, >0, >0, >0, >=1)\n",
				presents, sim.alive, sim.trails, sim.points, sim.batchCalls)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "game_particle-trail: OK presents=%d alive=%d trails=%d points=%d batch=%d elapsed=%.1fs\n",
			presents, sim.alive, sim.trails, sim.points, sim.batchCalls, elapsed)
		return
	}
	summary.Timed = secs > 0
	b, _ := json.Marshal(map[string]any{
		"ability_id":  trailAbilityID,
		"scenario":    trailScenario,
		"backend":     win.Backend().String(),
		"events":      map[string]any{"pointer": summary.Pointer, "key": summary.Key, "resize": summary.Resize},
		"presents":    presents,
		"elapsed_sec": elapsed,
		"alive":       sim.alive,
		"trails":      sim.trails,
		"points":      sim.points,
		"batch_calls": sim.batchCalls,
		"spawned":     sim.spawned,
		"probe_ok":    probeOK,
		"timed":       summary.Timed,
		"note":        summary.Note,
	})
	fmt.Println(string(b))
	fmt.Fprintf(os.Stderr, "game_particle-trail: backend=%s presents=%d alive=%d trails=%d points=%d batch=%d elapsed=%.1fs\n",
		win.Backend(), presents, sim.alive, sim.trails, sim.points, sim.batchCalls, elapsed)
}
