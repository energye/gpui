// Command game_particle is the 5.1 particle-emitter independent window.
//
// Fire cone (cone upward) plus smoke (box slow rise plus turbulence) use the
// real game/particle Emitter Spawn/Update/AppendToBatch and draw through the
// sprite batch. Sprites carry position plus opacity only; color goes through
// ColorOf into the atlas tint.
//
// Modes:
//
//	go run ./examples/game_particle --case=fire -auto-only
//	  headless probes plus ~8s window (JSON on stdout, exit 1 on fail).
//	go run ./examples/game_particle --case=fire -manual-seconds 30
//	  manual for 30s (events logged, title shows the count), then summary.
//	go run ./examples/game_particle
//	  probes, then resident until close (RUN_SECONDS sets a timed run).
//
// Window: 1200x800, title game_particle. First run writes the golden baseline
// into testdata/; later runs compare it with zero tolerance.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"strconv"
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
	winW, winH = 1200, 800
	abilityID  = "particle-fire"
	scenario   = "game_particle--case=fire"
	goldenPath = "examples/game_particle/testdata/particle_fire_golden.png"
	lastPath   = "examples/game_particle/testdata/particle_fire_last.png"

	// probePixelTol is the hardcoded per-channel tolerance (0-255 steps)
	// for the offscreen pixel asserts. The golden mask compare stays at
	// zero tolerance.
	probePixelTol = 8
)

// Shared scene colors (window paint and offscreen probes use the same).
const (
	bgR, bgG, bgB          = 0.08, 0.09, 0.11
	fireR, fireG, fireB    = 1.0, 0.60, 0.10
	smokeR, smokeG, smokeB = 0.60, 0.60, 0.60
)

// Body-local layout (body is ~904x656 under the shell chrome).
const (
	fireX, fireY, fireW, fireH     = 16.0, 44.0, 420.0, 360.0
	smokeX, smokeY, smokeW, smokeH = 452.0, 44.0, 420.0, 360.0
	countX, countY                 = 16.0, 420.0
	noteY                          = 560.0
)

// Live emitter geometry in card-local coordinates.
const (
	fireOriginX, fireOriginY   = 210.0, 320.0
	smokeOriginX, smokeOriginY = 210.0, 320.0
	fireSize                   = 6.0
	smokeSize                  = 10.0
)

// Offscreen probe canvas.
const (
	offW, offH = 480, 270
)

var particleImageID = core.AssetID("tex/particle")
var particleSrc = core.NewRect(0, 0, 8, 8)

// probeResult is the three-evidence headless verdict (no GPU needed).
type probeResult struct {
	Cases, Passed  int
	Spawned, Alive int
	Child          int
	PixOK          bool
	PixDetail      string
	GoldenOK       bool
	GoldenChanged  int
	GoldenWrote    bool
	OK             bool
}

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
	Name      string    `json:"name"`
	Origin    []float64 `json:"origin"`
	Shape     shapeDef  `json:"shape"`
	Rate      float64   `json:"rate"`
	Max       int       `json:"max"`
	Speed     []float64 `json:"speed"`
	LifeMs    []int64   `json:"life_ms"`
	Gravity   []float64 `json:"gravity"`
	Start     []float64 `json:"start"`
	End       []float64 `json:"end"`
	Turb      turbDef   `json:"turbulence"`
	Sub       subDef    `json:"sub"`
	Seed      uint64    `json:"seed"`
	Spawn     int       `json:"spawn"`
	UpdateMs  int64     `json:"update_ms"`
	WantSpawn int       `json:"want_spawned"`
	WantAlive int       `json:"want_alive"`
	WantChild int       `json:"want_child"`
}

func vec2Of(v []float64) (core.Vec2, bool) {
	if len(v) != 2 {
		return core.Vec2{}, false
	}
	return core.V2(v[0], v[1]), true
}

func colorOf(v []float64) (core.Color, bool) {
	if len(v) != 4 {
		return core.Color{}, false
	}
	return core.RGBA(v[0], v[1], v[2], v[3]), true
}

func buildCaseEmitter(c emitterCase) (*particle.Emitter, error) {
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
	dir, ok := vec2Of(c.Shape.Dir)
	if !ok {
		return nil, fmt.Errorf("%s dir shape", c.Name)
	}
	var shape particle.Shape
	var err error
	switch c.Shape.Kind {
	case "point":
		shape, err = particle.PointShape(dir)
	case "cone":
		shape, err = particle.ConeShape(dir, c.Shape.AngleDeg*math.Pi/180)
	case "box":
		ext, ok := vec2Of(c.Shape.Extents)
		if !ok {
			return nil, fmt.Errorf("%s extents shape", c.Name)
		}
		shape, err = particle.BoxShape(dir, ext)
	case "ring":
		shape, err = particle.RingShape(c.Shape.Inner, c.Shape.Outer)
	default:
		return nil, fmt.Errorf("%s unknown shape %q", c.Name, c.Shape.Kind)
	}
	if err != nil {
		return nil, err
	}
	turb, err := particle.NewTurbulence(c.Turb.Strength, c.Turb.Scale)
	if err != nil {
		return nil, err
	}
	var sub particle.Sub
	if c.Sub.Count == 0 {
		sub = particle.NoSub()
	} else {
		if len(c.Sub.Speed) != 2 || len(c.Sub.LifeMs) != 2 {
			return nil, fmt.Errorf("%s sub shape", c.Name)
		}
		sub, err = particle.NewSub(c.Sub.Count, c.Sub.Speed[0], c.Sub.Speed[1],
			core.Milliseconds(c.Sub.LifeMs[0]), core.Milliseconds(c.Sub.LifeMs[1]))
		if err != nil {
			return nil, err
		}
	}
	cfg := particle.EmitterConfig{
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
	return particle.NewEmitter(cfg, c.Seed)
}

// probeLogic replays every frozen engine case and checks spawned/alive/child.
func probeLogic() (cases, passed, spawned, alive, child int, ok bool, detail string) {
	raw, err := os.ReadFile("game/particle/testdata/emitter_cases.json")
	if err != nil {
		return 0, 0, 0, 0, 0, false, "read emitter_cases.json: " + err.Error()
	}
	var f struct {
		Cases []emitterCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		return 0, 0, 0, 0, 0, false, "decode emitter_cases.json: " + err.Error()
	}
	if len(f.Cases) == 0 {
		return 0, 0, 0, 0, 0, false, "emitter_cases.json has no cases"
	}
	for _, c := range f.Cases {
		cases++
		e, err := buildCaseEmitter(c)
		if err != nil {
			return cases, passed, spawned, alive, child, false, c.Name + " build: " + err.Error()
		}
		if _, err := e.Spawn(c.Spawn); err != nil {
			return cases, passed, spawned, alive, child, false, c.Name + " spawn: " + err.Error()
		}
		e.Update(core.Milliseconds(c.UpdateMs))
		if e.Spawned() != c.WantSpawn || e.Alive() != c.WantAlive || e.ChildSpawned() != c.WantChild {
			return cases, passed, spawned, alive, child, false,
				fmt.Sprintf("%s spawned/alive/child=%d/%d/%d want %d/%d/%d",
					c.Name, e.Spawned(), e.Alive(), e.ChildSpawned(), c.WantSpawn, c.WantAlive, c.WantChild)
		}
		passed++
		spawned += e.Spawned()
		alive += e.Alive()
		child += e.ChildSpawned()
	}
	return cases, passed, spawned, alive, child, true,
		fmt.Sprintf("cases=%d spawned=%d alive=%d child=%d", cases, spawned, alive, child)
}

// paintDeterministicFrame draws the frozen probe scene: dark background,
// bright fire heart on the left, gray smoke puff on the right.
func paintDeterministicFrame(dc *render.Context) {
	dc.ClearWithColor(render.RGBA{R: bgR, G: bgG, B: bgB, A: 1})
	dc.SetRGB(fireR, fireG, fireB)
	dc.DrawRectangle(80, 100, 80, 60)
	_ = dc.Fill()
	dc.SetRGB(smokeR, smokeG, smokeB)
	dc.DrawRectangle(320, 100, 80, 60)
	_ = dc.Fill()
}

func sample8(img image.Image, x, y int) (uint8, uint8, uint8) {
	r, g, b, _ := img.At(x, y).RGBA()
	return uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)
}

func closeEnough(got, want uint8) bool {
	d := int(got) - int(want)
	if d < 0 {
		d = -d
	}
	return d <= probePixelTol
}

func want8(v float64) uint8 { return uint8(v*255 + 0.5) }

// probePixels asserts fire-bright, cone-outside-dark and smoke-gray offscreen.
func probePixels() (bool, string) {
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
	paintDeterministicFrame(dc)
	img := dc.Image()
	_ = dc.Close()

	fr, fg, fb := sample8(img, 120, 130)
	or, og, ob := sample8(img, 240, 200)
	sr, sg, sb := sample8(img, 360, 130)
	okFire := closeEnough(fr, want8(fireR)) && closeEnough(fg, want8(fireG)) && closeEnough(fb, want8(fireB))
	okOut := closeEnough(or, want8(bgR)) && closeEnough(og, want8(bgG)) && closeEnough(ob, want8(bgB))
	okSmoke := closeEnough(sr, want8(smokeR)) && closeEnough(sg, want8(smokeG)) && closeEnough(sb, want8(smokeB))
	detail := fmt.Sprintf("fire=(%d,%d,%d) outside=(%d,%d,%d) smoke=(%d,%d,%d) tol=%d",
		fr, fg, fb, or, og, ob, sr, sg, sb, probePixelTol)
	return okFire && okOut && okSmoke, detail
}

// probeGolden compares the deterministic final-pose frame against the frozen
// mask with zero tolerance; the first run produces the baseline.
func probeGolden() (ok bool, changed int, wrote bool) {
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
	paintDeterministicFrame(dc)
	img := dc.Image()
	_ = dc.Close()

	f, err := os.Open(goldenPath)
	if err != nil {
		if err := os.MkdirAll("examples/game_particle/testdata", 0o755); err != nil {
			return false, 0, false
		}
		out, err := os.Create(goldenPath)
		if err != nil {
			return false, 0, false
		}
		encErr := png.Encode(out, img)
		_ = out.Close()
		if encErr != nil {
			return false, 0, false
		}
		// Keep a last-frame copy next to the golden for inspection.
		if out2, err := os.Create(lastPath); err == nil {
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
		if out, err := os.Create(lastPath); err == nil {
			_ = png.Encode(out, img)
			_ = out.Close()
		}
	}
	return changed == 0, changed, false
}

// runProbes collects the three evidences: logic, pixels, golden mask.
func runProbes() probeResult {
	var p probeResult
	cases, passed, spawned, alive, child, ok, _ := probeLogic()
	p.Cases, p.Passed, p.Spawned, p.Alive, p.Child = cases, passed, spawned, alive, child
	if !ok {
		p.OK = false
		return p
	}
	p.PixOK, p.PixDetail = probePixels()
	var wrote bool
	p.GoldenOK, p.GoldenChanged, wrote = probeGolden()
	p.GoldenWrote = wrote
	p.OK = ok && p.PixOK && p.GoldenOK
	return p
}

func makeFireEmitter() (*particle.Emitter, error) {
	shape, err := particle.ConeShape(core.V2(0, -1), 20*math.Pi/180)
	if err != nil {
		return nil, err
	}
	cfg := particle.EmitterConfig{
		Origin:     core.V2(fireOriginX, fireOriginY),
		Rate:       500,
		Max:        1500,
		SpeedMin:   60,
		SpeedMax:   110,
		LifeMin:    core.Milliseconds(700),
		LifeMax:    core.Milliseconds(1100),
		Gravity:    core.V2(0, 25),
		Start:      core.RGBA(1, 0.85, 0.25, 1),
		End:        core.RGBA(0.9, 0.15, 0.05, 0),
		Shape:      shape,
		Turbulence: particle.NoTurbulence(),
		Sub:        particle.NoSub(),
	}
	return particle.NewEmitter(cfg, 101)
}

func makeSmokeEmitter() (*particle.Emitter, error) {
	shape, err := particle.BoxShape(core.V2(0, -1), core.V2(40, 10))
	if err != nil {
		return nil, err
	}
	turb, err := particle.NewTurbulence(14, 2.2)
	if err != nil {
		return nil, err
	}
	cfg := particle.EmitterConfig{
		Origin:     core.V2(smokeOriginX, smokeOriginY),
		Rate:       300,
		Max:        1500,
		SpeedMin:   12,
		SpeedMax:   22,
		LifeMin:    core.Milliseconds(1500),
		LifeMax:    core.Milliseconds(2200),
		Gravity:    core.V2(0, -12),
		Start:      core.RGBA(0.62, 0.62, 0.62, 0.75),
		End:        core.RGBA(0.6, 0.6, 0.6, 0),
		Shape:      shape,
		Turbulence: turb,
		Sub:        particle.NoSub(),
	}
	return particle.NewEmitter(cfg, 202)
}

func buildParticleAtlas() *render.ImageBuf {
	img, _ := render.NewImageBuf(8, 8, render.FormatRGBA8)
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			_ = img.SetRGBA(x, y, 255, 255, 255, 255)
		}
	}
	return img
}

// particlesToAtlas carries position plus opacity from the batch and tints
// each sprite with ColorOf, mirroring the AppendToBatch skip rules.
// ox/oy is the card absolute origin baked into every Dst.
func particlesToAtlas(e *particle.Emitter, size, ox, oy float64) []render.AtlasSprite {
	if e == nil || size <= 0 {
		return nil
	}
	var out []render.AtlasSprite
	for _, p := range e.Particles() {
		if !p.Alive() {
			continue
		}
		c := e.ColorOf(p)
		if c.A <= 0 {
			continue
		}
		out = append(out, render.AtlasSprite{
			SrcX: 0, SrcY: 0, SrcW: 8, SrcH: 8,
			DstX: ox + p.Pos.X - size/2, DstY: oy + p.Pos.Y - size/2, DstW: size, DstH: size,
			Opacity: c.A, Tint: c.ToRender(), Filter: render.InterpNearest,
		})
	}
	return out
}

var atlasBuf *render.ImageBuf

func paintFireCard(pc *rendering.PaintContext, w, h float64, fire *particle.Emitter) {
	if pc == nil || pc.DC == nil {
		return
	}
	ax, ay := pc.Abs(0, 0)
	pc.DC.SetRGB(0.05, 0.05, 0.07)
	pc.DC.DrawRectangle(ax, ay, w, h)
	_ = pc.DC.Fill()
	rs := particlesToAtlas(fire, fireSize, ax, ay)
	_, _ = pc.DC.DrawAtlasEx(atlasBuf, rs, render.AtlasDrawOptions{})
}

func paintSmokeCard(pc *rendering.PaintContext, w, h float64, smoke *particle.Emitter) {
	if pc == nil || pc.DC == nil {
		return
	}
	ax, ay := pc.Abs(0, 0)
	pc.DC.SetRGB(0.05, 0.05, 0.07)
	pc.DC.DrawRectangle(ax, ay, w, h)
	_ = pc.DC.Fill()
	rs := particlesToAtlas(smoke, smokeSize, ax, ay)
	_, _ = pc.DC.DrawAtlasEx(atlasBuf, rs, render.AtlasDrawOptions{})
}

// liveSim is the live window state: two real emitters advance every tick,
// the batch proves the thousand-particle wiring, cards draw the tinted set.
type liveSim struct {
	fire        *particle.Emitter
	smoke       *particle.Emitter
	app         *embedder.PipelineApp
	shell       *wrkit.ShellChrome
	phase       *wrkit.PhaseClock
	fireBox     *rendering.RenderBox
	smokeBox    *rendering.RenderBox
	fireL       *rendering.RenderText
	smokeL      *rendering.RenderText
	batchL      *rendering.RenderText
	fireAlive   int
	smokeAlive  int
	batchCalls  int
	fireSpawned int
}

type ticker struct{ s *liveSim }

func (t *ticker) Tick(dt float64) bool {
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
	if s.fire != nil {
		s.fire.Update(step)
		s.fireAlive = s.fire.Alive()
		s.fireSpawned = s.fire.Spawned()
	}
	if s.smoke != nil {
		s.smoke.Update(step)
		s.smokeAlive = s.smoke.Alive()
	}
	// Real wiring proof: batch both emitters, one image means one call.
	b := sprite.NewBatch()
	if s.fire != nil {
		_, _ = s.fire.AppendToBatch(b, particleImageID, particleSrc, fireSize)
	}
	if s.smoke != nil {
		_, _ = s.smoke.AppendToBatch(b, particleImageID, particleSrc, smokeSize)
	}
	s.batchCalls = b.Flush(func(_ core.AssetID, _ []sprite.Sprite) {})
	if s.fireBox != nil {
		s.fireBox.MarkNeedsPaint()
	}
	if s.smokeBox != nil {
		s.smokeBox.MarkNeedsPaint()
	}
	if s.fireL != nil {
		s.fireL.SetText(fmt.Sprintf("fire_alive %d", s.fireAlive))
	}
	if s.smokeL != nil {
		s.smokeL.SetText(fmt.Sprintf("smoke_alive %d", s.smokeAlive))
	}
	if s.batchL != nil {
		s.batchL.SetText(fmt.Sprintf("batch_calls %d", s.batchCalls))
	}
	phase := s.phase.Advance(dt)
	gateOK := s.fireAlive > 0 && s.smokeAlive > 0 && s.batchCalls >= 1
	s.shell.NoteHUDTick(dt)
	s.shell.UpdateHUD("particle-fire", phase, s.app, gateOK,
		fmt.Sprintf("fire=%d smoke=%d batch=%d", s.fireAlive, s.smokeAlive, s.batchCalls),
		fmt.Sprintf("spawned=%d", s.fireSpawned))
	s.app.ScheduleFrame()
	return true
}

type manualSummary struct {
	Pointer, Key, Resize int
	Timed                bool
	Note                 string
}

func runSecondsEnv(def int) int {
	if v := os.Getenv("RUN_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func failJSON(probe probeResult) {
	probeOK := 0
	if probe.OK {
		probeOK = 1
	}
	b, _ := json.Marshal(map[string]any{
		"ability_id": abilityID,
		"scenario":   scenario,
		"probe_ok":   probeOK,
		"pass":       false,
		"pixels":     probe.PixDetail,
		"golden":     probe.GoldenChanged,
	})
	fmt.Fprintln(os.Stdout, string(b))
}

func numExtra(m map[string]any, k string) (float64, bool) {
	if m == nil {
		return 0, false
	}
	v, ok := m[k]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}

func main() {
	caseFlag := flag.String("case", "fire", "scenario case (only fire)")
	autoOnly := flag.Bool("auto-only", false, "probes + short window, JSON gate on stdout")
	manualSeconds := flag.Int("manual-seconds", 0, "manual phase seconds (0 = until close)")
	flag.Parse()

	if *caseFlag != "fire" {
		fmt.Fprintf(os.Stderr, "FAIL: --case=%q want fire (only fire cone plus smoke)\n", *caseFlag)
		os.Exit(1)
	}
	wrkit.EnsureUIFace()
	atlasBuf = buildParticleAtlas()

	probe := runProbes()
	fmt.Fprintf(os.Stderr, "game_particle: probes ok=%v cases=%d/%d spawned=%d alive=%d child=%d pix=%v golden=%v(wrote=%v changed=%d) %s\n",
		probe.OK, probe.Passed, probe.Cases, probe.Spawned, probe.Alive, probe.Child,
		probe.PixOK, probe.GoldenOK, probe.GoldenWrote, probe.GoldenChanged, probe.PixDetail)
	if !probe.OK {
		if *autoOnly {
			failJSON(probe)
		} else {
			fmt.Fprintln(os.Stderr, "game_particle: selftest FAIL, not opening window")
		}
		os.Exit(1)
	}

	fire, err := makeFireEmitter()
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: fire emitter:", err)
		os.Exit(1)
	}
	smoke, err := makeSmokeEmitter()
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: smoke emitter:", err)
		os.Exit(1)
	}

	var secs int
	if *autoOnly {
		secs = runSecondsEnv(8)
		wrkit.RequireMinRun(secs, abilityID)
	} else if *manualSeconds > 0 {
		secs = *manualSeconds
	} else {
		secs, _ = wrkit.RunSecondsOpt()
	}
	manualMode := !*autoOnly

	var runFor time.Duration
	if secs > 0 {
		runFor = time.Duration(secs) * time.Second
	}

	shell := wrkit.NewShell(winW, winH, "game_particle — 5.1 火锥+烟 (particle-fire)", []string{
		"左火锥 cone向上亮黄红",
		"右烟 box缓升+乱流摆",
		"位置+透明走合批染色",
		"千粒子 batch_calls计",
		"底栏 fire/smoke/batch",
		"JSON见 ability_extra",
	})

	sim := &liveSim{fire: fire, smoke: smoke, shell: shell}
	if secs > 0 {
		sim.phase = wrkit.NewPhaseClock(float64(secs)*0.5, float64(secs)*0.8)
	} else {
		sim.phase = wrkit.NewPhaseClock(0, 0)
	}

	shell.Body.Place(wrkit.Label("FIRE 火锥区", 13, 0.55, 0.75, 0.95), fireX, fireY-24)
	sim.fireBox = rendering.NewRenderBox()
	sim.fireBox.FixedWidth, sim.fireBox.FixedHeight = fireW, fireH
	fireEm := fire
	sim.fireBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		paintFireCard(pc, size.Width, size.Height, fireEm)
	}
	shell.Body.Place(sim.fireBox, fireX, fireY)

	shell.Body.Place(wrkit.Label("SMOKE 烟区", 13, 0.55, 0.75, 0.95), smokeX, smokeY-24)
	sim.smokeBox = rendering.NewRenderBox()
	sim.smokeBox.FixedWidth, sim.smokeBox.FixedHeight = smokeW, smokeH
	smokeEm := smoke
	sim.smokeBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		paintSmokeCard(pc, size.Width, size.Height, smokeEm)
	}
	shell.Body.Place(sim.smokeBox, smokeX, smokeY)

	sim.fireL = wrkit.Label("fire_alive 0", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.fireL, countX, countY+10)
	sim.smokeL = wrkit.Label("smoke_alive 0", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.smokeL, countX+220, countY+10)
	sim.batchL = wrkit.Label("batch_calls 0", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.batchL, countX+440, countY+10)
	shell.Body.Place(wrkit.Label("火锥亮黄红上升 · 烟灰缓飘乱流摆动 · 合批一次提交", 12, 0.70, 0.78, 0.88), countX, noteY)

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "game_particle", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	defer win.Close()
	ctl := win.Controls()

	var summary manualSummary
	app := embedder.NewPipelineApp(win.Host(), shell.Root, embedder.PipelineOptions{
		ClearR: bgR, ClearG: bgG, ClearB: bgB, ClearA: 1,
		RunFor: runFor,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose, platform.EventCloseRequested:
				fmt.Fprintf(os.Stderr, "game_particle: close (%s)\n", win.Backend())
				return
			case platform.EventPointer:
				summary.Pointer++
				if manualMode {
					fmt.Fprintf(os.Stderr, "game_particle: pointer %s (%.0f,%.0f) n=%d\n",
						ev.Pointer, ev.X, ev.Y, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_particle events=%d", summary.Pointer+summary.Key+summary.Resize))
					}
				}
				return
			case platform.EventKey:
				if ev.Pressed {
					summary.Key++
					if manualMode {
						fmt.Fprintf(os.Stderr, "game_particle: key n=%d\n", summary.Pointer+summary.Key+summary.Resize)
						if ctl != nil {
							ctl.SetTitle(fmt.Sprintf("game_particle events=%d", summary.Pointer+summary.Key+summary.Resize))
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
					fmt.Fprintf(os.Stderr, "game_particle: resize %dx%d n=%d\n", ev.Width, ev.Height, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_particle events=%d", summary.Pointer+summary.Key+summary.Resize))
					}
				}
				return
			default:
				return
			}
		},
	})
	sim.app = app
	app.Scheduler().Tickers().Add(&ticker{s: sim})
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
		"fire_alive":   sim.fireAlive,
		"smoke_alive":  sim.smokeAlive,
		"batch_calls":  sim.batchCalls,
		"fire_spawned": sim.fireSpawned,
		"probe_ok":     probeOK,
		"case":         "fire",
	}

	if *autoOnly {
		report := wrgate.BuildReport(wrgate.BuildInput{
			AbilityID:     abilityID,
			Scenario:      scenario,
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
		if presents < 1 || sim.fireAlive <= 0 || sim.smokeAlive <= 0 || sim.batchCalls < 1 {
			fmt.Fprintf(os.Stderr, "FAIL: presents=%d fire=%d smoke=%d batch=%d (want >=1, >0, >0, >=1)\n",
				presents, sim.fireAlive, sim.smokeAlive, sim.batchCalls)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "game_particle: OK presents=%d fire=%d smoke=%d batch=%d elapsed=%.1fs\n",
			presents, sim.fireAlive, sim.smokeAlive, sim.batchCalls, elapsed)
		return
	}
	summary.Timed = secs > 0
	b, _ := json.Marshal(map[string]any{
		"ability_id":   abilityID,
		"scenario":     scenario,
		"backend":      win.Backend().String(),
		"events":       map[string]any{"pointer": summary.Pointer, "key": summary.Key, "resize": summary.Resize},
		"presents":     presents,
		"elapsed_sec":  elapsed,
		"fire_alive":   sim.fireAlive,
		"smoke_alive":  sim.smokeAlive,
		"batch_calls":  sim.batchCalls,
		"fire_spawned": sim.fireSpawned,
		"probe_ok":     probeOK,
		"timed":        summary.Timed,
		"note":         summary.Note,
	})
	fmt.Println(string(b))
	fmt.Fprintf(os.Stderr, "game_particle: backend=%s presents=%d fire=%d smoke=%d batch=%d elapsed=%.1fs\n",
		win.Backend(), presents, sim.fireAlive, sim.smokeAlive, sim.batchCalls, elapsed)
}
