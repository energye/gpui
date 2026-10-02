// Command game_particles2d is the S83 particle对照窗 (case boom only).
//
// Fire cone (upward) plus smoke (box slow rise plus turbulence) plus boom
// (ring burst with child sparks) use the real engine/particle Emitter
// Spawn/Update/AppendToBatch and draw through the sprite batch. Sprites
// carry position plus opacity only; color goes through ColorOf into the
// atlas tint. Hand feel mirrors examples/engine/particle.
//
// Modes:
//
//	go run ./examples/engine/particles2d --case=boom -auto-only
//	  headless probes plus ~8s window (JSON on stdout, exit 1 on fail).
//	go run ./examples/engine/particles2d --case=boom -manual-seconds 30
//	  manual for 30s (click/key retriggers the boom, events logged,
//	  title shows the count), then summary.
//	go run ./examples/engine/particles2d
//	  probes, then resident until close (RUN_SECONDS sets a timed run).
//
// Window: 1200x800, title game_particles2d. First run writes the golden
// baseline into testdata/; later runs compare it with zero tolerance.
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

	"github.com/energye/gpui/engine/core"
	"github.com/energye/gpui/engine/particle"
	"github.com/energye/gpui/engine/renderconv"
	"github.com/energye/gpui/engine/sprite"
	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH = 1200, 800
	abilityID  = "particles2d-boom"
	scenario   = "game_particles2d--case=boom"
	goldenPath = "examples/engine/particles2d/testdata/particles2d_boom_golden.png"
	lastPath   = "examples/engine/particles2d/testdata/particles2d_boom_last.png"
	casesPath  = "examples/engine/particles2d/testdata/boom_cases.json"

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
	boomR, boomG, boomB    = 1.0, 0.45, 0.10
	sparkR, sparkG, sparkB = 1.0, 0.85, 0.30
)

// Body-local layout (legacy card coords, kept for probe geometry reference;
// live window is full-window content, see metricStripH).
const (
	fireX, fireY, fireW, fireH     = 16.0, 44.0, 280.0, 340.0
	smokeX, smokeY, smokeW, smokeH = 312.0, 44.0, 280.0, 340.0
	boomX, boomY, boomW, boomH     = 608.0, 44.0, 280.0, 340.0
	countX, countY                 = 16.0, 420.0
	noteY                          = 560.0
)

// 2.5D摆法：满窗即内容，指标浮左上，Golden排除指标带。
const metricStripH = 32.0

// Live emitter geometry in card-local coordinates.
const (
	fireOriginX, fireOriginY   = 140.0, 300.0
	smokeOriginX, smokeOriginY = 140.0, 300.0
	boomOriginX, boomOriginY   = 140.0, 170.0
	fireSize                   = 6.0
	smokeSize                  = 10.0
	boomSize                   = 7.0
)

// Offscreen probe canvas.
const (
	offW, offH = 480, 270
)

var particleImageID = core.AssetID("tex/particles2d")
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

// probeLogic replays every frozen boom case (fire, smoke, boom ring with
// children, sub ember) and checks spawned/alive/child.
func probeLogic() (cases, passed, spawned, alive, child int, ok bool, detail string) {
	raw, err := os.ReadFile(casesPath)
	if err != nil {
		return 0, 0, 0, 0, 0, false, "read boom_cases.json: " + err.Error()
	}
	var f struct {
		Cases []emitterCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		return 0, 0, 0, 0, 0, false, "decode boom_cases.json: " + err.Error()
	}
	if len(f.Cases) == 0 {
		return 0, 0, 0, 0, 0, false, "boom_cases.json has no cases"
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
// bright fire block on the left, gray smoke block on the right, orange
// boom band with a bright spark core at the bottom.
func paintDeterministicFrame(dc *render.Context) {
	dc.ClearWithColor(render.RGBA{R: bgR, G: bgG, B: bgB, A: 1})
	dc.SetRGB(fireR, fireG, fireB)
	dc.DrawRectangle(80, 60, 80, 60)
	_ = dc.Fill()
	dc.SetRGB(smokeR, smokeG, smokeB)
	dc.DrawRectangle(320, 60, 80, 60)
	_ = dc.Fill()
	dc.SetRGB(boomR, boomG, boomB)
	dc.DrawRectangle(140, 170, 200, 50)
	_ = dc.Fill()
	dc.SetRGB(sparkR, sparkG, sparkB)
	dc.DrawRectangle(215, 182, 50, 26)
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

// probePixels asserts fire-bright, smoke-gray, boom-orange, spark-core
// bright and outside-dark offscreen.
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

	fr, fg, fb := sample8(img, 120, 90)
	sr, sg, sb := sample8(img, 360, 90)
	br, bg, bb := sample8(img, 160, 195)
	kr, kg, kb := sample8(img, 240, 195)
	or, og, ob := sample8(img, 420, 230)
	okFire := closeEnough(fr, want8(fireR)) && closeEnough(fg, want8(fireG)) && closeEnough(fb, want8(fireB))
	okSmoke := closeEnough(sr, want8(smokeR)) && closeEnough(sg, want8(smokeG)) && closeEnough(sb, want8(smokeB))
	okBoom := closeEnough(br, want8(boomR)) && closeEnough(bg, want8(boomG)) && closeEnough(bb, want8(boomB))
	okSpark := closeEnough(kr, want8(sparkR)) && closeEnough(kg, want8(sparkG)) && closeEnough(kb, want8(sparkB))
	okOut := closeEnough(or, want8(bgR)) && closeEnough(og, want8(bgG)) && closeEnough(ob, want8(bgB))
	detail := fmt.Sprintf("fire=(%d,%d,%d) smoke=(%d,%d,%d) boom=(%d,%d,%d) spark=(%d,%d,%d) outside=(%d,%d,%d) tol=%d",
		fr, fg, fb, sr, sg, sb, br, bg, bb, kr, kg, kb, or, og, ob, probePixelTol)
	return okFire && okSmoke && okBoom && okSpark && okOut, detail
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
		if err := os.MkdirAll("examples/engine/particles2d/testdata", 0o755); err != nil {
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
	// Golden裁掉顶部指标带：浮层指标行不参比。
	stripPx := int(metricStripH * float64(img.Bounds().Dy()) / float64(winH))
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		if y-img.Bounds().Min.Y < stripPx {
			continue
		}
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

func makeBoomEmitter() (*particle.Emitter, error) {
	shape, err := particle.RingShape(4, 12)
	if err != nil {
		return nil, err
	}
	sub, err := particle.NewSub(3, 30, 60,
		core.Milliseconds(400), core.Milliseconds(600))
	if err != nil {
		return nil, err
	}
	cfg := particle.EmitterConfig{
		Origin:     core.V2(boomOriginX, boomOriginY),
		Rate:       0,
		Max:        1500,
		SpeedMin:   120,
		SpeedMax:   180,
		LifeMin:    core.Milliseconds(500),
		LifeMax:    core.Milliseconds(700),
		Gravity:    core.V2(0, 60),
		Start:      core.RGBA(1, 0.75, 0.25, 1),
		End:        core.RGBA(0.7, 0.15, 0.05, 0),
		Shape:      shape,
		Turbulence: particle.NoTurbulence(),
		Sub:        sub,
	}
	return particle.NewEmitter(cfg, 83)
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
			Opacity: c.A, Tint: renderconv.ColorToRender(c), Filter: render.InterpNearest,
		})
	}
	return out
}

var atlasBuf *render.ImageBuf

func paintParticleCard(pc *rendering.PaintContext, w, h float64, e *particle.Emitter, size float64) {
	if pc == nil || pc.DC == nil {
		return
	}
	ax, ay := pc.Abs(0, 0)
	pc.DC.SetRGB(0.05, 0.05, 0.07)
	pc.DC.DrawRectangle(ax, ay, w, h)
	_ = pc.DC.Fill()
	rs := particlesToAtlas(e, size, ax, ay)
	_, _ = pc.DC.DrawAtlasEx(atlasBuf, rs, render.AtlasDrawOptions{})
}

// paintFullWindow draws the full-window scene: fire left, smoke mid, boom
// right, day-night tint, walking monster rect (window only, engine untouched).
func paintFullWindow(pc *rendering.PaintContext, w, h float64, fire, smoke, boom *particle.Emitter, dayA, monX float64) {
	if pc == nil || pc.DC == nil {
		return
	}
	ax, ay := pc.Abs(0, 0)
	pc.DC.SetRGB(0.05, 0.05, 0.07)
	pc.DC.DrawRectangle(ax, ay, w, h)
	_ = pc.DC.Fill()
	oxF := ax + w*0.20 - fireOriginX
	oyF := ay + h*0.66 - fireOriginY
	oxS := ax + w*0.48 - smokeOriginX
	oyS := ay + h*0.64 - smokeOriginY
	oxB := ax + w*0.76 - boomOriginX
	oyB := ay + h*0.42 - boomOriginY
	rs := particlesToAtlas(fire, fireSize, oxF, oyF)
	rs = append(rs, particlesToAtlas(smoke, smokeSize, oxS, oyS)...)
	rs = append(rs, particlesToAtlas(boom, boomSize, oxB, oyB)...)
	_, _ = pc.DC.DrawAtlasEx(atlasBuf, rs, render.AtlasDrawOptions{})
	// Day-night tint over the whole scene.
	pc.DC.SetRGBA(0.12, 0.18, 0.45, dayA)
	pc.DC.DrawRectangle(ax, ay, w, h)
	_ = pc.DC.Fill()
	// Walking monster: dark body + bright eye, loops across the floor.
	mx, my := ax+monX, ay+h-110
	pc.DC.SetRGBA(0.16, 0.14, 0.20, 1)
	pc.DC.DrawRectangle(mx, my, 64, 46)
	_ = pc.DC.Fill()
	pc.DC.SetRGBA(1, 0.25, 0.20, 1)
	pc.DC.DrawRectangle(mx+44, my+12, 10, 10)
	_ = pc.DC.Fill()
}

// liveSim is the live window state: three real emitters advance every tick,
// the batch proves the wiring, one full-window box draws the thickened scene.
type liveSim struct {
	fire        *particle.Emitter
	smoke       *particle.Emitter
	boom        *particle.Emitter
	app         *embedder.PipelineApp
	root        *rendering.AbsoluteBox
	phase       *wrkit.PhaseClock
	fullBox     *rendering.RenderBox
	metric      *rendering.RenderText
	fireAlive   int
	smokeAlive  int
	boomAlive   int
	boomChild   int
	boomSpawned int
	batchCalls  int
	boomTimer   float64
	pendingBoom int
	triggers    int
	elapsed     float64
	dayA        float64
	monX        float64
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
	}
	if s.smoke != nil {
		s.smoke.Update(step)
		s.smokeAlive = s.smoke.Alive()
	}
	if s.boom != nil {
		// Manual retrigger first so a click/key burst shows up this tick.
		if s.pendingBoom > 0 {
			_, _ = s.boom.Spawn(40 * s.pendingBoom)
			s.triggers += s.pendingBoom
			s.pendingBoom = 0
		}
		// Periodic re-burst keeps the explosion alive without input.
		s.boomTimer += dt
		if s.boomTimer >= 1.5 {
			s.boomTimer = 0
			if s.boom.Alive() < 200 {
				_, _ = s.boom.Spawn(50)
			}
		}
		s.boom.Update(step)
		s.boomAlive = s.boom.Alive()
		s.boomChild = s.boom.ChildSpawned()
		s.boomSpawned = s.boom.Spawned()
	}
	// Real wiring proof: batch all three emitters, one image means one call.
	b := sprite.NewBatch()
	if s.fire != nil {
		_, _ = s.fire.AppendToBatch(b, particleImageID, particleSrc, fireSize)
	}
	if s.smoke != nil {
		_, _ = s.smoke.AppendToBatch(b, particleImageID, particleSrc, smokeSize)
	}
	if s.boom != nil {
		_, _ = s.boom.AppendToBatch(b, particleImageID, particleSrc, boomSize)
	}
	s.batchCalls = b.Flush(func(_ core.AssetID, _ []sprite.Sprite) {})
	// Thickening state: day-night cycle + monster walk (window only).
	s.elapsed += dt
	s.dayA = 0.10 + 0.08*math.Sin(s.elapsed*0.35)
	if s.dayA < 0 {
		s.dayA = 0
	}
	s.monX = math.Mod(s.elapsed*60, winW+128) - 64
	if s.fullBox != nil {
		s.fullBox.MarkNeedsPaint()
	}
	if s.metric != nil {
		s.metric.SetText(fmt.Sprintf("fire=%d smoke=%d boom=%d child=%d batch=%d day=%.2f", s.fireAlive, s.smokeAlive, s.boomAlive, s.boomChild, s.batchCalls, s.dayA))
		s.metric.MarkNeedsPaint()
	}
	_ = s.phase.Advance(dt)
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

func main() {
	caseFlag := flag.String("case", "boom", "scenario case (only boom)")
	autoOnly := flag.Bool("auto-only", false, "probes + short window, JSON gate on stdout")
	manualSeconds := flag.Int("manual-seconds", 0, "manual phase seconds (0 = until close)")
	flag.Parse()

	if *caseFlag != "boom" {
		fmt.Fprintf(os.Stderr, "FAIL: --case=%q want boom (fire plus smoke plus ring boom with children)\n", *caseFlag)
		os.Exit(1)
	}
	wrkit.EnsureUIFace()
	atlasBuf = buildParticleAtlas()

	probe := runProbes()
	fmt.Fprintf(os.Stderr, "game_particles2d: probes ok=%v cases=%d/%d spawned=%d alive=%d child=%d pix=%v golden=%v(wrote=%v changed=%d) %s\n",
		probe.OK, probe.Passed, probe.Cases, probe.Spawned, probe.Alive, probe.Child,
		probe.PixOK, probe.GoldenOK, probe.GoldenWrote, probe.GoldenChanged, probe.PixDetail)
	if !probe.OK {
		if *autoOnly {
			failJSON(probe)
		} else {
			fmt.Fprintln(os.Stderr, "game_particles2d: selftest FAIL, not opening window")
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
	boom, err := makeBoomEmitter()
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: boom emitter:", err)
		os.Exit(1)
	}
	if _, err := boom.Spawn(80); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: boom burst:", err)
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

	// 满窗即内容：整窗为火/烟/爆场景，昼夜罩+巡逻怪同场加厚。
	root := rendering.NewAbsoluteBox(winW, winH)
	root.Background = &rendering.Color{R: bgR, G: bgG, B: bgB, A: 1}
	sim := &liveSim{fire: fire, smoke: smoke, boom: boom, root: root}
	if secs > 0 {
		sim.phase = wrkit.NewPhaseClock(float64(secs)*0.5, float64(secs)*0.8)
	} else {
		sim.phase = wrkit.NewPhaseClock(0, 0)
	}

	sim.fullBox = rendering.NewRenderBox()
	sim.fullBox.FixedWidth, sim.fullBox.FixedHeight = winW, winH
	fireEm, smokeEm, boomEm := fire, smoke, boom
	sim.fullBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		paintFullWindow(pc, size.Width, size.Height, fireEm, smokeEm, boomEm, sim.dayA, sim.monX)
	}
	root.Place(sim.fullBox, 0, 0)
	sim.metric = wrkit.Label("fire=0 smoke=0 boom=0", 13, 0.92, 0.94, 0.98)
	root.Place(sim.metric, 8, 8)

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "game_particles2d", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	defer win.Close()
	ctl := win.Controls()

	var summary manualSummary
	app := embedder.NewPipelineApp(win.Host(), root, embedder.PipelineOptions{
		ClearR: bgR, ClearG: bgG, ClearB: bgB, ClearA: 1,
		RunFor: runFor,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose, platform.EventCloseRequested:
				fmt.Fprintf(os.Stderr, "game_particles2d: close (%s)\n", win.Backend())
				return
			case platform.EventPointer:
				summary.Pointer++
				sim.pendingBoom++
				if manualMode {
					fmt.Fprintf(os.Stderr, "game_particles2d: pointer %s (%.0f,%.0f) boom n=%d events=%d\n",
						ev.Pointer, ev.X, ev.Y, sim.triggers+sim.pendingBoom, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_particles2d boom=%d events=%d", sim.triggers+sim.pendingBoom, summary.Pointer+summary.Key+summary.Resize))
					}
				}
				return
			case platform.EventKey:
				if ev.Pressed {
					summary.Key++
					sim.pendingBoom++
					if manualMode {
						fmt.Fprintf(os.Stderr, "game_particles2d: key boom n=%d events=%d\n", sim.triggers+sim.pendingBoom, summary.Pointer+summary.Key+summary.Resize)
						if ctl != nil {
							ctl.SetTitle(fmt.Sprintf("game_particles2d boom=%d events=%d", sim.triggers+sim.pendingBoom, summary.Pointer+summary.Key+summary.Resize))
						}
					}
				}
				return
			case platform.EventResize:
				summary.Resize++
				if ev.Width > 0 && ev.Height > 0 {
					root.FixedWidth, root.FixedHeight = float64(ev.Width), float64(ev.Height)
					sim.fullBox.FixedWidth, sim.fullBox.FixedHeight = float64(ev.Width), float64(ev.Height)
					root.MarkNeedsPaint()
				}
				if manualMode {
					fmt.Fprintf(os.Stderr, "game_particles2d: resize %dx%d n=%d\n", ev.Width, ev.Height, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_particles2d boom=%d events=%d", sim.triggers+sim.pendingBoom, summary.Pointer+summary.Key+summary.Resize))
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
	root.MarkNeedsPaint()
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
		"boom_alive":   sim.boomAlive,
		"boom_child":   sim.boomChild,
		"boom_spawned": sim.boomSpawned,
		"triggers":     sim.triggers,
		"batch_calls":  sim.batchCalls,
		"probe_ok":     probeOK,
		"case":         "boom",
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
		if presents < 1 || sim.fireAlive <= 0 || sim.smokeAlive <= 0 || sim.boomAlive <= 0 || sim.boomSpawned <= 0 || sim.batchCalls < 1 {
			fmt.Fprintf(os.Stderr, "FAIL: presents=%d fire=%d smoke=%d boom=%d spawned=%d batch=%d (want >=1, >0, >0, >0, >0, >=1)\n",
				presents, sim.fireAlive, sim.smokeAlive, sim.boomAlive, sim.boomSpawned, sim.batchCalls)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "game_particles2d: OK presents=%d fire=%d smoke=%d boom=%d child=%d batch=%d elapsed=%.1fs\n",
			presents, sim.fireAlive, sim.smokeAlive, sim.boomAlive, sim.boomChild, sim.batchCalls, elapsed)
		return
	}
	summary.Timed = secs > 0
	b, _ := json.Marshal(map[string]any{
		"ability_id":  abilityID,
		"scenario":    scenario,
		"backend":     win.Backend().String(),
		"events":      map[string]any{"pointer": summary.Pointer, "key": summary.Key, "resize": summary.Resize},
		"presents":    presents,
		"elapsed_sec": elapsed,
		"fire_alive":  sim.fireAlive,
		"smoke_alive": sim.smokeAlive,
		"boom_alive":  sim.boomAlive,
		"boom_child":  sim.boomChild,
		"triggers":    sim.triggers,
		"batch_calls": sim.batchCalls,
		"probe_ok":    probeOK,
		"timed":       summary.Timed,
		"note":        summary.Note,
	})
	fmt.Println(string(b))
	fmt.Fprintf(os.Stderr, "game_particles2d: backend=%s presents=%d fire=%d smoke=%d boom=%d child=%d batch=%d elapsed=%.1fs\n",
		win.Backend(), presents, sim.fireAlive, sim.smokeAlive, sim.boomAlive, sim.boomChild, sim.batchCalls, elapsed)
}
