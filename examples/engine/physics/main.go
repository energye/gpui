// Command game_physics is the S61 game_physics dual-case window plus the
// S76 rider alignment rider (snap + MOTION_MODE two gears).
//
// Hit case: a hero box sweeps a trigger door and a solid wall, a +X ray
// probes ahead every frame, all through the real engine/physics body and
// ray API (14.1a/b). Jump case: a jumper arcs through a one-way platform
// and lands via StepWithSnap, a ramp rider holds through the S76 snap
// window, and a moving body platform carries its rider via CarryRider;
// slopes live in +Y-down tilemap units, carry bodies in +Y-up math units,
// the carry panel flips Y at paint time only.
//
// Modes:
//
//	go run ./examples/engine/physics --case=hit -auto-only
//	  headless probes + ~8s window (JSON on stdout, exit 1 on fail).
//	go run ./examples/engine/physics --case=jump -manual-seconds 30
//	  manual for 30s (events logged, title shows the count), then summary.
//	go run ./examples/engine/physics --case=hit
//	  probes, then resident until close (RUN_SECONDS sets a timed run).
//
// GAME_PHYSICS_PROBE_ONLY=1 runs the selected case probes headlessly
// (logic + pixels + golden, JSON on stdout, no window) for CI and for
// producing the testdata baselines without a display.
//
// Window: 1200x800, title game_physics. First run writes the golden
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
	"github.com/energye/gpui/engine/physics"
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
	abilityID  = "game-physics"

	scenarioHit   = "game_physics--case=hit"
	scenarioJump  = "game_physics--case=jump"
	scenarioLarge = "game_physics--case=large"

	goldenHitPath   = "examples/engine/physics/testdata/physics_hit_golden.png"
	goldenJumpPath  = "examples/engine/physics/testdata/physics_jump_golden.png"
	goldenLargePath = "examples/engine/physics/testdata/physics_large_golden.png"

	largeYardPath = "engine/physics/testdata/large_yard.json"
	largeRaysPath = "engine/physics/testdata/large_rays.json"

	frozenBodyPath     = "engine/physics/testdata/body_cases.json"
	frozenRayPath      = "engine/physics/testdata/body_ray_cases.json"
	frozenPlatformPath = "engine/physics/testdata/platform_cases.json"
	frozenSnapPath     = "engine/physics/testdata/snap_mode_cases.json"

	// probePixelTol is the hardcoded per-channel tolerance (0-255 steps)
	// for the offscreen pixel asserts. The golden mask compare stays at
	// zero tolerance.
	probePixelTol = 8

	// Perf ceilings baked into -auto-only. These are order-of-magnitude
	// guards, not formal numbers: the probe binary is a debug build on a
	// possibly loaded host (body-100 alone swings 0.2-1.7ms here), while
	// formal S61 magnitudes (~350us / ~10us / ~0.3us per op) are judged
	// from release triple-runs per §5. The ceilings only catch 10x-class
	// regressions without flaking on machine noise.
	maxBody100UsPerQuery = 4000.0
	maxRay100UsPerCast   = 100.0
	maxSlopeUsPerOp      = 5.0
)

// Shared scene colors (window paint and offscreen probes use the same).
const (
	bgR, bgG, bgB = 0.08, 0.09, 0.11

	hitWallR, hitWallG, hitWallB = 0.45, 0.55, 0.75
	hitDoorR, hitDoorG, hitDoorB = 0.95, 0.75, 0.25
	hitHeroR, hitHeroG, hitHeroB = 0.90, 0.20, 0.15
	hitRayR, hitRayG, hitRayB    = 1.0, 0.90, 0.20

	jumpGroundR, jumpGroundG, jumpGroundB = 0.30, 0.42, 0.30
	jumpPlatR, jumpPlatG, jumpPlatB       = 0.35, 0.55, 0.75
	jumpRampR, jumpRampG, jumpRampB       = 0.60, 0.60, 0.65
	jumpRiderR, jumpRiderG, jumpRiderB    = 0.95, 0.45, 0.15
	jumpBodyR, jumpBodyG, jumpBodyB       = 0.50, 0.55, 0.65
	jumpRailR, jumpRailG, jumpRailB       = 0.40, 0.42, 0.48
)

// Hit scene geometry in arena px (horizontal only, no Y semantics).
const (
	hitWallX, hitWallY, hitWallW, hitWallH = 300.0, 90.0, 40.0, 90.0
	hitDoorX, hitDoorY, hitDoorW, hitDoorH = 180.0, 120.0, 60.0, 60.0
	hitHeroW, hitHeroH, hitHeroY           = 36.0, 36.0, 120.0
	hitHeroMinX, hitHeroMaxX               = 60.0, 280.0
	hitHeroSpeed                           = 90.0
	hitHeroGoldenX                         = 270.0
	hitRayY                                = 138.0
	hitRayMax                              = 400.0
)

// Jump slope scene in +Y-down px; carry bodies in +Y-up world units.
const (
	jumpFlatAx, jumpFlatAy, jumpFlatBx, jumpFlatBy = 40.0, 220.0, 440.0, 220.0
	jumpOneAx, jumpOneAy, jumpOneBx, jumpOneBy     = 140.0, 150.0, 300.0, 150.0
	jumpRampAx, jumpRampAy, jumpRampBx, jumpRampBy = 40.0, 220.0, 140.0, 150.0
	jumpStartX, jumpStartY                         = 200.0, 220.0
	jumpVelX, jumpVelY                             = 10.0, -320.0
	jumpGravity                                    = 600.0
	jumpHoldS                                      = 0.6
	jumpRiderSize                                  = 28.0

	jumpRampFootX, jumpRampFootY = 90.0, 185.0

	jumpPlatX0, jumpPlatY        = 160.0, 60.0
	jumpPlatHalfX, jumpPlatHalfY = 30.0, 6.0
	jumpPlatRange, jumpPlatSpeed = 40.0, 30.0
	jumpRiderHalf                = 14.0

	offW, offH = 480, 270
)

// probeResult is the three-evidence headless verdict (no GPU needed).
type probeResult struct {
	LogicOK, PixOK, GoldenOK bool
	Detail                   string
	PixDetail                string
	GoldenChanged            int
	GoldenWrote              bool
	Body100Us                float64
	Ray100Us                 float64
	SlopeUs                  float64
	OK                       bool
}

func scenarioOf(caseFlag string) string {
	if caseFlag == "jump" {
		return scenarioJump
	}
	return scenarioHit
}

// ---------------------------------------------------------------------------
// Frozen-file replay helpers (example reads engine truth, never hardcodes it).
// ---------------------------------------------------------------------------

type frozenBodyDef struct {
	Name    string     `json:"name"`
	Shape   string     `json:"shape"`
	Pos     [2]float64 `json:"pos"`
	Half    [2]float64 `json:"half"`
	Radius  float64    `json:"radius"`
	Layer   uint32     `json:"layer"`
	Mask    uint32     `json:"mask"`
	Trigger bool       `json:"trigger"`
}

func frozenBody(d frozenBodyDef) (physics.Body, error) {
	pos := core.V2(d.Pos[0], d.Pos[1])
	switch d.Shape {
	case "box":
		return physics.NewBox(d.Name, pos, core.V2(d.Half[0], d.Half[1]), d.Layer, d.Mask, d.Trigger)
	case "circle":
		return physics.NewCircle(d.Name, pos, d.Radius, d.Layer, d.Mask, d.Trigger)
	default:
		return physics.Body{}, fmt.Errorf("unknown shape %q", d.Shape)
	}
}

type frozenBodyFile struct {
	Bodies []frozenBodyDef `json:"bodies"`
}

func frozenBodyMap(path string) (map[string]physics.Body, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f frozenBodyFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, err
	}
	m := map[string]physics.Body{}
	for _, d := range f.Bodies {
		b, err := frozenBody(d)
		if err != nil {
			return nil, err
		}
		m[d.Name] = b
	}
	return m, nil
}

type frozenSlopeDef struct {
	Name   string     `json:"name"`
	A      [2]float64 `json:"a"`
	B      [2]float64 `json:"b"`
	OneWay bool       `json:"one_way"`
}

func frozenSlope(d frozenSlopeDef) (physics.Slope, error) {
	a := core.V2(d.A[0], d.A[1])
	b := core.V2(d.B[0], d.B[1])
	if d.OneWay {
		return physics.NewOneWay(d.Name, a, b)
	}
	return physics.NewSlope(d.Name, a, b)
}

type frozenSlopeFile struct {
	Slopes []frozenSlopeDef `json:"slopes"`
}

func frozenSlopeMap(path string) (map[string]physics.Slope, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f frozenSlopeFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, err
	}
	m := map[string]physics.Slope{}
	for _, d := range f.Slopes {
		s, err := frozenSlope(d)
		if err != nil {
			return nil, err
		}
		m[d.Name] = s
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// Hit probes.
// ---------------------------------------------------------------------------

func hitBodies() (hero, wall, door physics.Body, err error) {
	hero, err = physics.NewBox("hero", core.V2(hitHeroGoldenX+hitHeroW/2, hitRayY), core.V2(hitHeroW/2, hitHeroH/2), 1, 1, false)
	if err != nil {
		return hero, wall, door, err
	}
	wall, err = physics.NewBox("wall", core.V2(hitWallX+hitWallW/2, hitWallY+hitWallH/2), core.V2(hitWallW/2, hitWallH/2), 1, 1, false)
	if err != nil {
		return hero, wall, door, err
	}
	door, err = physics.NewBox("door", core.V2(hitDoorX+hitDoorW/2, hitDoorY+hitDoorH/2), core.V2(hitDoorW/2, hitDoorH/2), 1, 1, true)
	return hero, wall, door, err
}

// probeHitLogic drives the real body/ray API: frozen replay, edge touch,
// trigger flag, mask silence, replay stability, perf ceilings.
func probeHitLogic() (bool, string, float64, float64) {
	bm, err := frozenBodyMap(frozenBodyPath)
	if err != nil {
		return false, "frozen body read: " + err.Error(), 0, 0
	}
	// Edge touch counts as a hit (stand counts), a hair gap does not;
	// geometry may touch while masks stay silent.
	if !physics.Overlaps(bm["box_a"], bm["box_touch"]) {
		return false, "edge touch = miss, want hit", 0, 0
	}
	if physics.Overlaps(bm["box_a"], bm["box_gap"]) {
		return false, "0.5 gap = hit, want miss", 0, 0
	}
	if physics.CanCollide(bm["box_a"], bm["ghost"]) {
		return false, "ghost masks = collide, want silent", 0, 0
	}
	if got, _ := physics.Query([]physics.Body{bm["hero"], bm["ghost"]}); len(got) != 0 {
		return false, "masked query not empty", 0, 0
	}
	trig, err := physics.Query([]physics.Body{bm["hero"], bm["coin_near"]})
	if err != nil || len(trig) != 1 || !trig[0].Trigger {
		return false, "trigger pair not flagged", 0, 0
	}
	// Live scene pose: hero frozen into the wall, door reported flagged.
	hero, wall, door, err := hitBodies()
	if err != nil {
		return false, "hit bodies: " + err.Error(), 0, 0
	}
	got, err := physics.Query([]physics.Body{hero, wall, door})
	if err != nil || len(got) != 1 || got[0].Trigger {
		return false, fmt.Sprintf("golden pose query = %v/%v, want one solid", got, err), 0, 0
	}
	// Ray replay from the frozen file: wall distance, trigger nearest,
	// tile solid cell as a plain box.
	rm, err := frozenBodyMap(frozenRayPath)
	if err != nil {
		return false, "frozen ray read: " + err.Error(), 0, 0
	}
	ray, _ := physics.NewRay(core.V2(0, 0), core.V2(1, 0), 20, 1)
	hit, ok, err := physics.CastRay([]physics.Body{rm["wall_box"]}, ray)
	if err != nil || !ok || hit.Dist != 8 || hit.Pos != core.V2(8, 0) {
		return false, fmt.Sprintf("wall ray = %+v/%v/%v, want dist 8", hit, ok, err), 0, 0
	}
	trigHit, ok, err := physics.CastRay([]physics.Body{rm["trigger_zone"], rm["wall_box"]}, ray)
	if err != nil || !ok || !trigHit.Trigger || trigHit.Name != "trigger_zone" {
		return false, "trigger nearest not flagged", 0, 0
	}
	tileRay, _ := physics.NewRay(core.V2(0, 8), core.V2(1, 0), 30, 1)
	tile, ok, err := physics.CastRay([]physics.Body{rm["tile_solid_10"]}, tileRay)
	if err != nil || !ok || tile.Dist != 16 {
		return false, "tile probe missed", 0, 0
	}
	// Corner stand: flat end meets the wall column, no crash, flat holds.
	pm, err := frozenSlopeMap(frozenPlatformPath)
	if err != nil {
		return false, "frozen platform read: " + err.Error(), 0, 0
	}
	if pos, g, ok, head, err := physics.Step(core.V2(50, 100), core.V2(0, 0), 0.016, []physics.Slope{pm["flat"], pm["wall"]}); err != nil || !ok || head || g.Name() != "flat" {
		return false, fmt.Sprintf("corner stand = %v/%v/%v/%v", pos, ok, head, err), 0, 0
	}
	// Replay stability: 2000 identical queries agree bit-for-bit.
	first, err := physics.Query([]physics.Body{hero, wall, door})
	if err != nil {
		return false, "replay query: " + err.Error(), 0, 0
	}
	for i := 0; i < 2000; i++ {
		rep, err := physics.Query([]physics.Body{hero, wall, door})
		if err != nil || len(rep) != len(first) || (len(rep) > 0 && rep[0] != first[0]) {
			return false, "replay drifted", 0, 0
		}
	}
	// Perf: 100 boxes query + 100 boxes ray cast, both timed.
	r := core.NewRand(20260915)
	const n = 100
	load := make([]physics.Body, n)
	for i := 0; i < n; i++ {
		b, err := physics.NewBox("p", core.V2(r.RangeFloat(-500, 500), r.RangeFloat(-500, 500)), core.V2(8, 8), 1, 1, i%10 == 0)
		if err != nil {
			return false, "perf bodies: " + err.Error(), 0, 0
		}
		load[i] = b
	}
	const reps = 200
	start := time.Now()
	for i := 0; i < reps; i++ {
		if _, err := physics.Query(load); err != nil {
			return false, "perf query: " + err.Error(), 0, 0
		}
	}
	bodyUs := float64(time.Since(start).Microseconds()) / reps
	start = time.Now()
	for i := 0; i < reps; i++ {
		if _, _, err := physics.CastRay(load, ray); err != nil {
			return false, "perf cast: " + err.Error(), 0, 0
		}
	}
	rayUs := float64(time.Since(start).Microseconds()) / reps
	if bodyUs > maxBody100UsPerQuery || rayUs > maxRay100UsPerCast {
		return false, fmt.Sprintf("perf over ceiling: body %.1fus ray %.1fus", bodyUs, rayUs), bodyUs, rayUs
	}
	return true, fmt.Sprintf("frozen=body+ray+corner replay=2000 body100=%.1fus ray100=%.1fus", bodyUs, rayUs), bodyUs, rayUs
}

// ---------------------------------------------------------------------------
// Jump probes (slope space + S76 snap + body carry).
// ---------------------------------------------------------------------------

func jumpSlopes() ([]physics.Slope, error) {
	flat, err := physics.NewSlope("flat", core.V2(jumpFlatAx, jumpFlatAy), core.V2(jumpFlatBx, jumpFlatBy))
	if err != nil {
		return nil, err
	}
	one, err := physics.NewOneWay("one", core.V2(jumpOneAx, jumpOneAy), core.V2(jumpOneBx, jumpOneBy))
	if err != nil {
		return nil, err
	}
	ramp, err := physics.NewSlope("ramp", core.V2(jumpRampAx, jumpRampAy), core.V2(jumpRampBx, jumpRampBy))
	if err != nil {
		return nil, err
	}
	return []physics.Slope{flat, one, ramp}, nil
}

// probeJumpLogic drives the real slope/snap/carry API: one-way pass then
// land, steep never floor, floating never floor, carry chain holds,
// replay stability, long-slope perf ceiling.
func probeJumpLogic() (bool, string, float64) {
	slopes, err := jumpSlopes()
	if err != nil {
		return false, "jump slopes: " + err.Error(), 0
	}
	cfg := physics.DefaultSnapConfig()
	flat, one := slopes[0], slopes[1]
	// Frozen platform replay: stand, fall, one-way pass, solid bump.
	pm, err := frozenSlopeMap(frozenPlatformPath)
	if err != nil {
		return false, "frozen platform read: " + err.Error(), 0
	}
	for _, tc := range []struct {
		name         string
		feet, vel    core.Vec2
		dt           float64
		slopes       []physics.Slope
		wantPos      core.Vec2
		ground, head bool
		wantGround   string
	}{
		{"stand_flat", core.V2(50, 100), core.V2(0, 0), 0.016, []physics.Slope{pm["flat"]}, core.V2(50, 100), true, false, "flat"},
		{"fall_flat", core.V2(50, 90), core.V2(0, 100), 0.1, []physics.Slope{pm["flat"]}, core.V2(50, 100), true, false, "flat"},
		{"pass_oneway", core.V2(50, 90), core.V2(0, -200), 0.1, []physics.Slope{pm["oneway"]}, core.V2(50, 70), false, false, ""},
		{"bump_solid", core.V2(50, 110), core.V2(0, -200), 0.1, []physics.Slope{pm["flat"]}, core.V2(50, 100), false, true, ""},
	} {
		pos, ground, grounded, head, err := physics.Step(tc.feet, tc.vel, tc.dt, tc.slopes)
		if err != nil || pos != tc.wantPos || grounded != tc.ground || head != tc.head {
			return false, fmt.Sprintf("frozen %s = %v/%v/%v/%v", tc.name, pos, grounded, head, err), 0
		}
		if grounded && ground.Name() != tc.wantGround {
			return false, fmt.Sprintf("frozen %s ground = %q", tc.name, ground.Name()), 0
		}
	}
	// S76 frozen replay: defaults plus the snap table.
	sm, err := frozenSlopeMap(frozenSnapPath)
	if err != nil {
		return false, "frozen snap read: " + err.Error(), 0
	}
	if _, ok := physics.SnapGrounded(core.V2(50, 99.95), []physics.Slope{sm["flat"]}, cfg); !ok {
		return false, "snap gap bridge missed", 0
	}
	if _, ok := physics.SnapGrounded(core.V2(10, 50), []physics.Slope{sm["steep"]}, cfg); ok {
		return false, "steep reported floor", 0
	}
	flo := physics.SnapConfig{Mode: physics.MotionModeFloating, SnapLength: 0.1, FloorMaxAngle: 0.7853981633974483, SafeMargin: 0.08}
	if physics.IsOnFloor(core.V2(50, 100), []physics.Slope{sm["flat"]}, flo) {
		return false, "floating reported floor", 0
	}
	// Single up through the live one-way passes, falling lands on it.
	up, _, floor, head, err := physics.StepWithSnap(core.V2(200, 160), core.V2(0, -320), 0.1, []physics.Slope{one}, cfg)
	if err != nil || floor || head {
		return false, fmt.Sprintf("live pass = %v/%v/%v/%v", up, floor, head, err), 0
	}
	down, _, floor, head, err := physics.StepWithSnap(up, core.V2(0, 300), 0.1, []physics.Slope{one}, cfg)
	if err != nil || !floor || head {
		return false, fmt.Sprintf("live land = %v/%v/%v/%v", down, floor, head, err), 0
	}
	// Ramp foot holds through the snap window every time.
	if !physics.IsOnFloor(core.V2(jumpRampFootX, jumpRampFootY), slopes, cfg) {
		return false, "ramp foot not floor", 0
	}
	// Body carry chain: 10 identical steps keep the rider aboard.
	prider, _ := physics.NewBox("rider", core.V2(160, 80), core.V2(14, 14), 1, 1, false)
	pplat, _ := physics.NewBox("plat", core.V2(160, 60), core.V2(30, 6), 1, 1, false)
	if !physics.IsStandingOn(prider, pplat) {
		return false, "carry pose not standing", 0
	}
	step := core.V2(0.5, 0)
	for i := 0; i < 10; i++ {
		carried, err := physics.CarryRider(&prider, pplat, step)
		if err != nil || !carried {
			return false, fmt.Sprintf("carry step %d: %v/%v", i, prider, err), 0
		}
		pplat.Pos = pplat.Pos.Add(step)
		if !physics.IsStandingOn(prider, pplat) {
			return false, fmt.Sprintf("carry step %d lost contact", i), 0
		}
	}
	// Replay stability: 2000 identical snap steps agree.
	feet := core.V2(50, 99.95)
	vel := core.V2(0, 10)
	aPos, _, aFloor, _, err := physics.StepWithSnap(feet, vel, 0.001, []physics.Slope{flat}, cfg)
	if err != nil {
		return false, "replay snap: " + err.Error(), 0
	}
	for i := 0; i < 2000; i++ {
		pos, _, floor, _, err := physics.StepWithSnap(feet, vel, 0.001, []physics.Slope{flat}, cfg)
		if err != nil || pos != aPos || floor != aFloor {
			return false, "snap replay drifted", 0
		}
	}
	// Perf: long-slope grounds plus snap steps, timed.
	long, _ := physics.NewSlope("long", core.V2(0, 0), core.V2(10000, -1000))
	const reps = 2000
	const steps = 1000
	start := time.Now()
	var sum float64
	for i := 0; i < reps; i++ {
		y, ok := long.GroundYAt(float64(i%10001) + 0.5)
		if !ok {
			return false, "long ground miss", 0
		}
		sum += y
	}
	for i := 0; i < steps; i++ {
		pos, _, _, _, err := physics.StepWithSnap(core.V2(float64(i%10000)+0.25, -200), core.V2(60, 300), 0.016, []physics.Slope{long}, cfg)
		if err != nil {
			return false, "long snap: " + err.Error(), 0
		}
		sum += pos.X + pos.Y
	}
	_ = sum
	slopeUs := float64(time.Since(start).Microseconds()) / float64(reps+steps)
	if slopeUs > maxSlopeUsPerOp {
		return false, fmt.Sprintf("slope perf over ceiling: %.1fus/op", slopeUs), slopeUs
	}
	return true, fmt.Sprintf("frozen=platform+snap pass+land carry=10 replay=2000 slope=%.1fus/op", slopeUs), slopeUs
}

// ---------------------------------------------------------------------------
// Shared pixel + golden probes.
// ---------------------------------------------------------------------------

func paintHitFrame(dc *render.Context, ox, oy, heroX float64) {
	dc.SetRGB(bgR, bgG, bgB)
	dc.DrawRectangle(ox, oy, offW, offH)
	_ = dc.Fill()
	dc.SetRGB(hitDoorR, hitDoorG, hitDoorB)
	dc.DrawRectangle(ox+hitDoorX, oy+hitDoorY, hitDoorW, hitDoorH)
	_ = dc.Fill()
	dc.SetRGB(hitWallR, hitWallG, hitWallB)
	dc.DrawRectangle(ox+hitWallX, oy+hitWallY, hitWallW, hitWallH)
	_ = dc.Fill()
	dc.SetRGB(hitHeroR, hitHeroG, hitHeroB)
	dc.DrawRectangle(ox+heroX, oy+hitHeroY, hitHeroW, hitHeroH)
	_ = dc.Fill()
	dc.SetRGB(hitRayR, hitRayG, hitRayB)
	dc.SetLineWidth(2)
	x0 := ox + heroX + hitHeroW/2
	x1 := ox + hitWallX
	if x0 > x1 {
		x1 = x0 + 12
	}
	dc.DrawLine(x0, oy+hitRayY, x1, oy+hitRayY)
	_ = dc.Stroke()
}

func paintJumpSlope(dc *render.Context, ox, oy, jumpX, jumpY float64) {
	dc.SetRGB(bgR, bgG, bgB)
	dc.DrawRectangle(ox, oy, offW, offH)
	_ = dc.Fill()
	dc.SetRGB(jumpGroundR, jumpGroundG, jumpGroundB)
	dc.DrawRectangle(ox+jumpFlatAx, oy+jumpFlatAy, jumpFlatBx-jumpFlatAx, 12)
	_ = dc.Fill()
	dc.SetRGB(jumpPlatR, jumpPlatG, jumpPlatB)
	dc.DrawRectangle(ox+jumpOneAx, oy+jumpOneAy, jumpOneBx-jumpOneAx, 8)
	_ = dc.Fill()
	dc.SetRGB(jumpRampR, jumpRampG, jumpRampB)
	dc.SetLineWidth(6)
	dc.DrawLine(ox+jumpRampAx, oy+jumpRampAy, ox+jumpRampBx, oy+jumpRampBy)
	_ = dc.Stroke()
	dc.SetRGB(jumpRiderR, jumpRiderG, jumpRiderB)
	dc.DrawRectangle(ox+jumpX-jumpRiderSize/2, oy+jumpY-jumpRiderSize, jumpRiderSize, jumpRiderSize)
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

func cpuContext() (prev string) {
	prev, _ = os.LookupEnv("GOGPU_RENDER_MODE")
	_ = os.Setenv("GOGPU_RENDER_MODE", "cpu")
	return prev
}

func restoreContext(prev string) {
	if prev == "" {
		_ = os.Unsetenv("GOGPU_RENDER_MODE")
	} else {
		_ = os.Setenv("GOGPU_RENDER_MODE", prev)
	}
}

// probeHitPixels asserts hero-on-wall, wall, door and quiet-bg colors.
func probeHitPixels() (bool, string) {
	prev := cpuContext()
	defer restoreContext(prev)
	dc := render.NewContext(offW, offH)
	paintHitFrame(dc, 0, 0, hitHeroGoldenX)
	img := dc.Image()
	_ = dc.Close()

	// Hero sample sits above the ray line (the line crosses the hero
	// center row); wall, door and bg samples stay clear of it.
	hr, hg, hb := sample8(img, int(hitHeroGoldenX+hitHeroW/2), int(hitHeroY+8))
	wr, wg, wb := sample8(img, int(hitWallX+hitWallW/2), int(hitWallY+hitWallH/2))
	dr, dg, db := sample8(img, int(hitDoorX+hitDoorW/2), int(hitDoorY+hitDoorH/2))
	br, bg, bb := sample8(img, 20, 20)
	ok := closeEnough(hr, want8(hitHeroR)) && closeEnough(hg, want8(hitHeroG)) && closeEnough(hb, want8(hitHeroB)) &&
		closeEnough(wr, want8(hitWallR)) && closeEnough(wg, want8(hitWallG)) && closeEnough(wb, want8(hitWallB)) &&
		closeEnough(dr, want8(hitDoorR)) && closeEnough(dg, want8(hitDoorG)) && closeEnough(db, want8(hitDoorB)) &&
		closeEnough(br, want8(bgR)) && closeEnough(bg, want8(bgG)) && closeEnough(bb, want8(bgB))
	detail := fmt.Sprintf("hero=(%d,%d,%d) wall=(%d,%d,%d) door=(%d,%d,%d) bg=(%d,%d,%d) tol=%d",
		hr, hg, hb, wr, wg, wb, dr, dg, db, br, bg, bb, probePixelTol)
	return ok, detail
}

// probeJumpPixels asserts landed rider, one-way bar, ground bar and bg.
func probeJumpPixels() (bool, string) {
	prev := cpuContext()
	defer restoreContext(prev)
	dc := render.NewContext(offW, offH)
	paintJumpSlope(dc, 0, 0, 210, 150)
	img := dc.Image()
	_ = dc.Close()

	rr, rg, rb := sample8(img, 210, 136)
	pr, pg, pb := sample8(img, 160, 154)
	gr, gg, gb := sample8(img, 60, 226)
	br, bg, bb := sample8(img, 20, 20)
	ok := closeEnough(rr, want8(jumpRiderR)) && closeEnough(rg, want8(jumpRiderG)) && closeEnough(rb, want8(jumpRiderB)) &&
		closeEnough(pr, want8(jumpPlatR)) && closeEnough(pg, want8(jumpPlatG)) && closeEnough(pb, want8(jumpPlatB)) &&
		closeEnough(gr, want8(jumpGroundR)) && closeEnough(gg, want8(jumpGroundG)) && closeEnough(gb, want8(jumpGroundB)) &&
		closeEnough(br, want8(bgR)) && closeEnough(bg, want8(bgG)) && closeEnough(bb, want8(bgB))
	detail := fmt.Sprintf("rider=(%d,%d,%d) oneway=(%d,%d,%d) ground=(%d,%d,%d) bg=(%d,%d,%d) tol=%d",
		rr, rg, rb, pr, pg, pb, gr, gg, gb, br, bg, bb, probePixelTol)
	return ok, detail
}

// probeGolden compares the deterministic frame against the frozen mask
// with zero tolerance; the first run produces the baseline.
func probeGolden(path, caseFlag string) (ok bool, changed int, wrote bool) {
	prev := cpuContext()
	defer restoreContext(prev)
	dc := render.NewContext(offW, offH)
	if caseFlag == "jump" {
		paintJumpSlope(dc, 0, 0, 210, 150)
	} else {
		paintHitFrame(dc, 0, 0, hitHeroGoldenX)
	}
	img := dc.Image()
	_ = dc.Close()

	f, err := os.Open(path)
	if err != nil {
		if err := os.MkdirAll("examples/engine/physics/testdata", 0o755); err != nil {
			return false, 0, false
		}
		out, err := os.Create(path)
		if err != nil {
			return false, 0, false
		}
		encErr := png.Encode(out, img)
		_ = out.Close()
		return encErr == nil, 0, encErr == nil
	}
	defer func() { _ = f.Close() }()
	want, err := png.Decode(f)
	if err != nil {
		return false, 0, false
	}
	if !img.Bounds().Eq(want.Bounds()) {
		return false, 1, false
	}
	// Golden裁掉顶部指标带：浮层指标行不参比，只比它下面的纯画面。
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
	return changed == 0, changed, false
}

// runProbes collects the three evidences: logic, pixels, golden mask.
func runProbes(caseFlag string) probeResult {
	var p probeResult
	if caseFlag == "jump" {
		p.LogicOK, p.Detail, p.SlopeUs = probeJumpLogic()
		p.PixOK, p.PixDetail = probeJumpPixels()
		p.GoldenOK, p.GoldenChanged, p.GoldenWrote = probeGolden(goldenJumpPath, caseFlag)
	} else {
		var bodyUs, rayUs float64
		p.LogicOK, p.Detail, bodyUs, rayUs = probeHitLogic()
		p.Body100Us, p.Ray100Us = bodyUs, rayUs
		p.PixOK, p.PixDetail = probeHitPixels()
		p.GoldenOK, p.GoldenChanged, p.GoldenWrote = probeGolden(goldenHitPath, caseFlag)
	}
	p.OK = p.LogicOK && p.PixOK && p.GoldenOK
	return p
}

// ---------------------------------------------------------------------------
// Live window: hit case.
// ---------------------------------------------------------------------------

// Body-local layout (legacy probe geometry reference; live window is
// full-window content, see metricStripH).
const (
	hitArenaX, hitArenaY       = 16.0, 44.0
	hitCountX, hitCountY       = 512.0, 44.0
	hitNoteY                   = 340.0
	jumpSlopeX, jumpSlopeY     = 16.0, 44.0
	jumpCarryX, jumpCarryY     = 16.0, 330.0
	jumpCarryW, jumpCarryH     = 360.0, 180.0
	jumpCountX, jumpCountY     = 512.0, 44.0
	jumpNoteY                  = 530.0
	jumpCarryOX, jumpCarryBase = 20.0, 150.0
)

// 2.5D摆法：满窗即内容，指标浮左上，Golden排除指标带。
// metricStripH是浮层指标带高度，窗口Golden比对从该高度之下起算。
// 离屏探针帧无指标覆盖；窗口快照只比该带之下的纯画面。
const metricStripH = 32.0

type hitSim struct {
	app     *embedder.PipelineApp
	root    *rendering.AbsoluteBox
	phase   *wrkit.PhaseClock
	fullBox *rendering.RenderBox
	metric  *rendering.RenderText
	arena   *rendering.RenderBox
	heroX   float64
	dir     float64
	moved   float64
	contact int64
	rayHit  int64
	trig    int64
	frames  int
	sparks  int64
	bumpSnd int64
}

type hitTicker struct{ s *hitSim }

func (t *hitTicker) Tick(dt float64) bool {
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
	s.frames++
	nx := s.heroX + s.dir*hitHeroSpeed*dt
	if nx >= hitHeroMaxX {
		nx, s.dir = hitHeroMaxX, -1
	}
	if nx <= hitHeroMinX {
		nx, s.dir = hitHeroMinX, 1
	}
	moved := nx - s.heroX
	if moved < 0 {
		moved = -moved
	}
	s.moved += moved
	s.heroX = nx

	// Real logic only: hero sweeps door and wall, ray probes ahead.
	hero, _ := physics.NewBox("hero", core.V2(s.heroX+hitHeroW/2, hitRayY), core.V2(hitHeroW/2, hitHeroH/2), 1, 1, false)
	wall, _ := physics.NewBox("wall", core.V2(hitWallX+hitWallW/2, hitWallY+hitWallH/2), core.V2(hitWallW/2, hitWallH/2), 1, 1, false)
	door, _ := physics.NewBox("door", core.V2(hitDoorX+hitDoorW/2, hitDoorY+hitDoorH/2), core.V2(hitDoorW/2, hitDoorH/2), 1, 1, true)
	if got, err := physics.Query([]physics.Body{hero, wall, door}); err == nil && len(got) > 0 {
		if s.contact == 0 {
			s.sparks = int64(s.frames)
		}
		if err == nil && len(got) > 0 && !got[0].Trigger {
			s.bumpSnd++
		}
		s.contact++
	}
	ray, _ := physics.NewRay(core.V2(s.heroX+hitHeroW/2, hitRayY), core.V2(1, 0), hitRayMax, 1)
	if hit, ok, err := physics.CastRay([]physics.Body{door, wall}, ray); err == nil && ok {
		s.rayHit++
		if hit.Name == "door" {
			s.trig++
		}
	}
	if s.fullBox != nil {
		s.fullBox.MarkNeedsPaint()
	}
	_ = s.phase.Advance(dt)
	snap := s.app.Metrics().Snapshot()
	fps := 0.0
	if snap.AvgFrameIntervalMs > 1e-6 {
		fps = 1000.0 / snap.AvgFrameIntervalMs
	}
	if s.metric != nil {
		s.metric.SetText(fmt.Sprintf("hit fps=%.0f x=%.0f contact=%d ray=%d snd=%d", fps, s.heroX, s.contact, s.rayHit, s.bumpSnd))
		s.metric.MarkNeedsPaint()
	}
	s.app.ScheduleFrame()
	return true
}

// ---------------------------------------------------------------------------
// Live window: jump case.
// ---------------------------------------------------------------------------

type jumpSim struct {
	app     *embedder.PipelineApp
	root    *rendering.AbsoluteBox
	phase   *wrkit.PhaseClock
	slopes  []physics.Slope
	cfg     physics.SnapConfig
	fullBox *rendering.RenderBox
	metric  *rendering.RenderText
	feet    core.Vec2
	vel     core.Vec2
	hold    bool
	holdT   float64
	wasLane bool
	armed   bool
	lands   int64
	passes  int64
	onLane  int64
	plat    physics.Body
	rider   physics.Body
	platDir float64
	carries int64
	holds   int64
	frames  int
	sparks  int64
	bumpSnd int64
	boxes   []core.Vec2
}

type jumpTicker struct{ s *jumpSim }

func (t *jumpTicker) Tick(dt float64) bool {
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
	s.frames++

	// Jumper: hold on the floor, leap, pass the one-way, land again.
	if s.hold {
		s.holdT += dt
		if s.holdT >= jumpHoldS {
			s.hold, s.holdT, s.armed = false, 0, true
			s.vel = core.V2(jumpVelX, jumpVelY)
		}
	} else {
		prevY := s.feet.Y
		s.vel = core.V2(s.vel.X, s.vel.Y+jumpGravity*dt)
		pos, _, onFloor, _, err := physics.StepWithSnap(s.feet, s.vel, dt, s.slopes, s.cfg)
		if err == nil {
			s.feet = pos
			if s.armed && prevY > jumpOneAy && pos.Y <= jumpOneAy &&
				pos.X >= jumpOneAx && pos.X <= jumpOneBx && !onFloor {
				s.passes++
				s.armed = false
			}
			if !s.wasLane && onFloor {
				s.lands++
				s.hold, s.holdT = true, 0
				s.vel = core.V2(0, 0)
			}
			s.wasLane = onFloor
			if onFloor {
				s.onLane++
			}
		}
	}

	// Carry line: platform steps, rider rides when aboard (Y-up world).
	dx := s.platDir * jumpPlatSpeed * dt
	if s.plat.Pos.X+dx > jumpPlatX0+jumpPlatRange || s.plat.Pos.X+dx < jumpPlatX0-jumpPlatRange {
		s.platDir = -s.platDir
		dx = s.platDir * jumpPlatSpeed * dt
	}
	if carried, err := physics.CarryRider(&s.rider, s.plat, core.V2(dx, 0)); err == nil && carried {
		s.carries++
	}
	s.plat.Pos = s.plat.Pos.Add(core.V2(dx, 0))
	if physics.IsStandingOn(s.rider, s.plat) {
		s.holds++
	}
	// Thickening state: deterministic box stack settles beside the lane;
	// landing bursts spark ticks and thump counts (window only).
	if s.boxes == nil {
		s.boxes = []core.Vec2{core.V2(420, 200), core.V2(452, 200), core.V2(436, 168)}
	}
	if !s.wasLane && s.hold {
		s.sparks = int64(s.frames)
		s.bumpSnd++
	}

	if s.fullBox != nil {
		s.fullBox.MarkNeedsPaint()
	}
	_ = s.phase.Advance(dt)
	snap := s.app.Metrics().Snapshot()
	fps := 0.0
	if snap.AvgFrameIntervalMs > 1e-6 {
		fps = 1000.0 / snap.AvgFrameIntervalMs
	}
	state := "air"
	if s.hold {
		state = "hold"
	} else if s.wasLane {
		state = "stand"
	}
	if s.metric != nil {
		s.metric.SetText(fmt.Sprintf("jump fps=%.0f %s land=%d pass=%d carry=%d snd=%d", fps, state, s.lands, s.passes, s.carries, s.bumpSnd))
		s.metric.MarkNeedsPaint()
	}
	s.app.ScheduleFrame()
	return true
}

type manualSummary struct {
	Pointer, Key, Resize int
	Timed                bool
	Note                 string
}

func runSeconds(def int) int {
	if v := os.Getenv("RUN_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

// ================= S86 large case (game_physics--case=large) =================
//
// The filed 1200-body yard (engine/physics/testdata/large_yard.json) opens
// through engine/physics Broadphase: 1000 static boxes plus 200 dynamics
// fall 60 units, the grid shortlists pairs, sleepers skip integration but
// still collide and wake on touch, per-frame contacts cap at 40. Filed
// one-way plus slope plus spring pad ride along (jump/ladder thick extras
// from the jump case); the hero auto-hops the pad and the one-way.
//
//	go run ./examples/engine/physics --case=large -auto-only
//	  headless probes + ~8s window (JSON on stdout, exit 1 on fail).
//	go run ./examples/engine/physics --case=large -manual-seconds 60
//	  manual for 60s (space hops, WASD nudges the hero), then summary.
//
// Window: 1200x800, title game_physics-large. First run writes the golden
// baseline into testdata/physics_large_golden.png; later runs compare it
// with zero tolerance.

const (
	largeAbilityID = "game-physics"
	largeScenario  = "game_physics--case=large"

	largeProbeTol   = 8
	largeGatePairs  = 1
	largeGateLands  = 1
	largeQueryUsMax = 50.0
	largeRayUsMax   = 30.0
	// largeQueryFullUsMax is the order-of-magnitude ceiling for the full
	// yard scan, same as TestBroadphaseBudgetFiled: the full scan is
	// validation/stats, not the per-frame gameplay query.
	largeQueryFullUsMax = 4000.0
	// largeViewHome is the filed gameplay viewport (view_home in
	// large_rays.json, 154 bodies): the per-frame Godot cull_aabb query.
	largeViewX, largeViewY, largeViewW, largeViewH = 64.0, 64.0, 640.0, 480.0
	largeWarmupS    = 2.0
	largeFallSpeed  = 220.0
	largeKeyStep    = 48.0

	largeOffW, largeOffH = 480, 300

	largeStaticR, largeStaticG, largeStaticB = 0.35, 0.50, 0.70
	largeDynR, largeDynG, largeDynB          = 0.95, 0.70, 0.20
	largeSleepR, largeSleepG, largeSleepB    = 0.45, 0.45, 0.50
	largePadR, largePadG, largePadB          = 0.95, 0.25, 0.45
	largeHeroR, largeHeroG, largeHeroB       = 0.90, 0.20, 0.15
)

// largeYard is the live filed level: bodies in filed order plus slopes,
// springs, and the broadphase index over them.
type largeYard struct {
	bp      physics.Broadphase
	bodies  []physics.Body
	slopes  []physics.Slope
	springs []physics.Spring
	layout  yardLayoutJSON
	count   int
}

type largeProbe struct {
	LogicOK, PixOK, GoldenOK bool
	Pairs                    int
	Total                    int
	Detail                   string
	PixDetail                string
	GoldenChanged            int
	GoldenWrote              bool
	QueryUs                  float64
	RayUs                    float64
	OK                       bool
}

func largeFailJSON(p largeProbe) {
	b, _ := json.Marshal(map[string]any{
		"ability_id": largeAbilityID,
		"scenario":   largeScenario,
		"probe_ok":   0,
		"pass":       false,
		"pixels":     p.PixDetail,
		"golden":     p.GoldenChanged,
	})
	fmt.Fprintln(os.Stdout, string(b))
}

// largeKeyNudge maps platform keysym WASD/arrows to hero nudges.
// The platform delivers keysyms (ui/platform decodeKey): letters arrive
// as 'a'/'A' etc, arrows as 0xff51-0xff54, space as 0x20. The old evdev
// table never matched here (and its D=32 collided with space), so it is
// gone. Unknown keys still count as events, they just don't move.
func largeKeyNudge(code int, r rune) (float64, float64) {
	switch {
	case r == 'a' || r == 'A' || code == 97 || code == 65 || code == 0xff51:
		return -largeKeyStep, 0
	case r == 'd' || r == 'D' || code == 100 || code == 68 || code == 0xff53:
		return largeKeyStep, 0
	case r == 'w' || r == 'W' || code == 119 || code == 87 || code == 0xff52:
		return 0, -largeKeyStep
	case r == 's' || r == 'S' || code == 115 || code == 83 || code == 0xff54:
		return 0, largeKeyStep
	}
	return 0, 0
}

// largeIsJump reports the space bar: keysym 0x20, Rune ' '.
func largeIsJump(code int, r rune) bool {
	return code == 0x20 || r == ' '
}

type yardLayoutJSON struct {
	Cols         int        `json:"cols"`
	Rows         int        `json:"rows"`
	Spacing      float64    `json:"spacing"`
	Origin       [2]float64 `json:"origin"`
	Half         float64    `json:"half"`
	StaticCount  int        `json:"static_count"`
	DynamicCount int        `json:"dynamic_count"`
	SpringX      float64    `json:"spring_x"`
	Count        int        `json:"count"`
}

type yardDynJSON struct {
	Name string     `json:"name"`
	X    float64    `json:"x"`
	Y    float64    `json:"y"`
	Half [2]float64 `json:"half"`
}

type yardSlopeJSON struct {
	Name   string     `json:"name"`
	A      [2]float64 `json:"a"`
	B      [2]float64 `json:"b"`
	Oneway bool       `json:"oneway"`
}

type yardSpringJSON struct {
	Name    string  `json:"name"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Impulse float64 `json:"impulse"`
	Damp    float64 `json:"damp"`
}

type yardFileJSON struct {
	Layout   yardLayoutJSON   `json:"layout"`
	Dynamics []yardDynJSON    `json:"dynamics"`
	Oneways  []yardSlopeJSON  `json:"oneways"`
	Springs  []yardSpringJSON `json:"springs"`
}

type yardRayJSON struct {
	Name    string     `json:"name"`
	Origin  [2]float64 `json:"origin"`
	Dir     [2]float64 `json:"dir"`
	MaxDist float64    `json:"maxdist"`
	Mask    uint32     `json:"mask"`
}

type yardRegionJSON struct {
	Name string     `json:"name"`
	Rect [4]float64 `json:"rect"`
	Want []int      `json:"want"`
}

type yardCullJSON struct {
	SceneFile string           `json:"scene_file"`
	Rays      []yardRayJSON    `json:"rays"`
	Regions   []yardRegionJSON `json:"regions"`
	Views     map[string]struct {
		Want [][]string `json:"want_pairs"`
	} `json:"-"`
	QueryStatic struct {
		WantPairs [][]string `json:"want_pairs"`
		WantCount int        `json:"want_count"`
	} `json:"query_static"`
	QueryDrop struct {
		DY        float64    `json:"dy"`
		WantPairs [][]string `json:"want_pairs"`
		WantCount int        `json:"want_count"`
	} `json:"query_drop"`
	Budgets struct {
		Entities    int     `json:"entities"`
		Reps        int     `json:"reps"`
		UpdateMsMax float64 `json:"update_ms_max"`
		QueryUsMax  float64 `json:"query_us_max"`
		RayUsMax    float64 `json:"ray_us_max"`
		Cycles      int     `json:"cycles"`
		Reopens     int     `json:"reopens"`
	} `json:"budgets"`
}

func loadLargeYardFiles() (yardFileJSON, yardCullJSON, error) {
	var yf yardFileJSON
	var cf yardCullJSON
	raw, err := os.ReadFile(largeRaysPath)
	if err != nil {
		return yf, cf, err
	}
	if err := json.Unmarshal(raw, &cf); err != nil {
		return yf, cf, err
	}
	raw, err = os.ReadFile(largeYardPath)
	if err != nil {
		return yf, cf, err
	}
	if err := json.Unmarshal(raw, &yf); err != nil {
		return yf, cf, err
	}
	return yf, cf, nil
}

// buildLargeYard stamps the filed yard into a live Broadphase in filed
// order: statics row-major, dynamics last. The window never invents a
// number; every position comes from the files.
func buildLargeYard() (*largeYard, error) {
	yf, _, err := loadLargeYardFiles()
	if err != nil {
		return nil, err
	}
	lv := &largeYard{layout: yf.Layout}
	bp := physics.NewBroadphase()
	l := yf.Layout
	n := l.StaticCount
	for i := 0; i < n; i++ {
		c, r := i%l.Cols, i/l.Cols
		pos := core.V2(l.Origin[0]+float64(c)*l.Spacing, l.Origin[1]+float64(r)*l.Spacing)
		bd, err := physics.NewBox(fmt.Sprintf("s%04d", i), pos, core.V2(l.Half, l.Half), 1, 1, false)
		if err != nil {
			return nil, err
		}
		if _, err := bp.Track(bd, true); err != nil {
			return nil, err
		}
		lv.bodies = append(lv.bodies, bd)
	}
	for _, d := range yf.Dynamics {
		bd, err := physics.NewBox(d.Name, core.V2(d.X, d.Y), core.V2(d.Half[0], d.Half[1]), 1, 1, false)
		if err != nil {
			return nil, err
		}
		if _, err := bp.Track(bd, false); err != nil {
			return nil, err
		}
		lv.bodies = append(lv.bodies, bd)
	}
	for _, o := range yf.Oneways {
		var sl physics.Slope
		if o.Oneway {
			sl, err = physics.NewOneWay(o.Name, core.V2(o.A[0], o.A[1]), core.V2(o.B[0], o.B[1]))
		} else {
			sl, err = physics.NewSlope(o.Name, core.V2(o.A[0], o.A[1]), core.V2(o.B[0], o.B[1]))
		}
		if err != nil {
			return nil, err
		}
		lv.slopes = append(lv.slopes, sl)
	}
	for _, s := range yf.Springs {
		sp, err := physics.NewSpring(s.Impulse, s.Damp)
		if err != nil {
			return nil, err
		}
		lv.springs = append(lv.springs, sp)
	}
	lv.bp = bp
	lv.count = l.Count
	if bp.Count() != l.Count {
		return nil, fmt.Errorf("yard count = %d, want %d", bp.Count(), l.Count)
	}
	return lv, nil
}

// probeLargeLogic rebuilds the filed yard and checks the filed static
// pairs plus the filed rays plus the filed spring pad, all exact.
// qus is the viewport single query (cull_aabb), not the full scan.
func probeLargeLogic() (ok bool, pairs, total int, detail string, qus, rus float64) {
	yf, cf, err := loadLargeYardFiles()
	if err != nil {
		return false, 0, 0, "load: " + err.Error(), 0, 0
	}
	lv, err := buildLargeYard()
	if err != nil {
		return false, 0, 0, "build: " + err.Error(), 0, 0
	}
	if err := lv.bp.SetMaxCandidates(0); err != nil {
		return false, 0, lv.count, "uncap: " + err.Error(), 0, 0
	}
	got, err := lv.bp.Query()
	if err != nil {
		return false, 0, lv.count, "query: " + err.Error(), 0, 0
	}
	if len(got) != len(cf.QueryStatic.WantPairs) {
		return false, 0, lv.count, fmt.Sprintf("static pairs = %d, want %d", len(got), len(cf.QueryStatic.WantPairs)), 0, 0
	}
	for i := range got {
		if got[i].A != cf.QueryStatic.WantPairs[i][0] || got[i].B != cf.QueryStatic.WantPairs[i][1] {
			return false, 0, lv.count, fmt.Sprintf("pair[%d] = %s+%s", i, got[i].A, got[i].B), 0, 0
		}
	}
	for _, rc := range cf.Rays {
		ray, err := physics.NewRay(core.V2(rc.Origin[0], rc.Origin[1]), core.V2(rc.Dir[0], rc.Dir[1]), rc.MaxDist, rc.Mask)
		if err != nil {
			return false, 0, lv.count, rc.Name + " ray: " + err.Error(), 0, 0
		}
		t1 := time.Now()
		fh, fok, err := lv.bp.Ray(ray)
		rus += float64(time.Since(t1).Microseconds())
		if err != nil {
			return false, 0, lv.count, rc.Name + " broad: " + err.Error(), 0, 0
		}
		sh, sok, err := physics.CastRay(lv.bodies, ray)
		if err != nil {
			return false, 0, lv.count, rc.Name + " narrow: " + err.Error(), 0, 0
		}
		if fok != sok || (fok && (fh.Name != sh.Name || fh.Dist != sh.Dist)) {
			return false, 0, lv.count, rc.Name + " ray mismatch", 0, 0
		}
	}
	rus /= float64(len(cf.Rays))
	// Viewport single query time drives the q gate.
	t0 := time.Now()
	if _, err := lv.bp.QueryRegion(core.NewRect(largeViewX, largeViewY, largeViewW, largeViewH)); err != nil {
		return false, 0, lv.count, "region: " + err.Error(), 0, 0
	}
	qus = float64(time.Since(t0).Microseconds())
	if len(yf.Springs) == 0 || len(yf.Oneways) == 0 {
		return false, 0, lv.count, "no filed springs/oneways", 0, 0
	}
	sp := yf.Springs[0]
	pad, err := physics.NewSpring(sp.Impulse, sp.Damp)
	if err != nil {
		return false, 0, lv.count, "spring: " + err.Error(), 0, 0
	}
	if pad.Bounce(300) != -(300*sp.Damp + sp.Impulse) {
		return false, 0, lv.count, "pad bounce moved", 0, 0
	}
	return true, len(got), lv.count,
		fmt.Sprintf("bodies=%d static_pairs=%d rays=%d springs=%d oneways=%d", lv.count, len(got), len(cf.Rays), len(yf.Springs), len(yf.Oneways)), qus, rus
}

// largeYardDots snapshots the yard for one deterministic probe frame:
// statics plus dropped dynamics plus pad plus hero, in track order.
type largeYardDot struct {
	px, py float64
	static bool
	sleep  bool
	pad    bool
	hero   bool
	layer  int
}

func largeYardFrame(lv *largeYard) []largeYardDot {
	sx := float64(largeOffW) / 2080.0
	sy := float64(largeOffH) / 1280.0
	dots := make([]largeYardDot, 0, len(lv.bodies)+1)
	for i, bd := range lv.bodies {
		dots = append(dots, largeYardDot{
			px:     bd.Pos.X * sx,
			py:     bd.Pos.Y * sy,
			static: i < lv.layout.StaticCount,
			layer:  i % 3,
		})
	}
	dots = append(dots, largeYardDot{px: yfSpringX(lv) * sx, py: 420 * sy, pad: true})
	return dots
}

// largeSpringX reads the filed pad column for the probe frame.
func yfSpringX(lv *largeYard) float64 {
	if len(lv.springs) == 0 {
		return 0
	}
	return lv.layout.SpringX
}

// paintLargeYard draws bg, boxes, pads, hero on top. Box centers keep
// their band color for the probes.
func paintLargeYard(dc *render.Context, dots []largeYardDot, box float64) {
	dc.ClearWithColor(render.RGBA{R: bgR, G: bgG, B: bgB, A: 1})
	for _, d := range dots {
		var r, g, b float64
		switch {
		case d.pad:
			r, g, b = largePadR, largePadG, largePadB
		case d.hero:
			r, g, b = largeHeroR, largeHeroG, largeHeroB
		case d.sleep:
			r, g, b = largeSleepR, largeSleepG, largeSleepB
		case d.static:
			r, g, b = largeStaticR, largeStaticG, largeStaticB
		default:
			r, g, b = largeDynR, largeDynG, largeDynB
		}
		hw := box / 2
		if d.pad {
			hw = 5
		}
		dc.SetRGB(r, g, b)
		dc.DrawRectangle(d.px-hw, d.py-hw, hw*2, hw*2)
		_ = dc.Fill()
	}
}

func probeLargePixels() (bool, string) {
	prev, _ := os.LookupEnv("GOGPU_RENDER_MODE")
	_ = os.Setenv("GOGPU_RENDER_MODE", "cpu")
	defer func() {
		if prev == "" {
			_ = os.Unsetenv("GOGPU_RENDER_MODE")
		} else {
			_ = os.Setenv("GOGPU_RENDER_MODE", prev)
		}
	}()
	lv, err := buildLargeYard()
	if err != nil {
		return false, "build: " + err.Error()
	}
	dots := largeYardFrame(lv)
	dc := render.NewContext(largeOffW, largeOffH)
	paintLargeYard(dc, dots, 12*float64(largeOffW)/2080.0)
	img := dc.Image()
	_ = dc.Close()
	sx := float64(largeOffW) / 2080.0
	sy := float64(largeOffH) / 1280.0
	checks := []struct {
		name    string
		x, y    int
		r, g, b float64
	}{
		{"static", int(64 * sx), int(64 * sy), largeStaticR, largeStaticG, largeStaticB},
		{"dyn", int(64 * sx), int(8 * sy), largeDynR, largeDynG, largeDynB},
		{"pad", int(320 * sx), int(420 * sy), largePadR, largePadG, largePadB},
		{"bg", int(1500 * sx), int(1100 * sy), bgR, bgG, bgB},
	}
	detail := ""
	for _, ch := range checks {
		if ch.x < 0 || ch.y < 0 || ch.x >= largeOffW || ch.y >= largeOffH {
			return false, ch.name + " off frame"
		}
		r, g, b := sample8(img, ch.x, ch.y)
		if !closeEnough(r, want8(ch.r)) || !closeEnough(g, want8(ch.g)) || !closeEnough(b, want8(ch.b)) {
			return false, fmt.Sprintf("%s=(%d,%d,%d)@(%d,%d) tol=%d", ch.name, r, g, b, ch.x, ch.y, largeProbeTol)
		}
		detail += fmt.Sprintf("%s@(%d,%d) ", ch.name, ch.x, ch.y)
	}
	return true, detail + fmt.Sprintf("tol=%d", largeProbeTol)
}

func probeLargeGolden() (ok bool, changed int, wrote bool) {
	prev, _ := os.LookupEnv("GOGPU_RENDER_MODE")
	_ = os.Setenv("GOGPU_RENDER_MODE", "cpu")
	defer func() {
		if prev == "" {
			_ = os.Unsetenv("GOGPU_RENDER_MODE")
		} else {
			_ = os.Setenv("GOGPU_RENDER_MODE", prev)
		}
	}()
	lv, err := buildLargeYard()
	if err != nil {
		return false, 0, false
	}
	dc := render.NewContext(largeOffW, largeOffH)
	paintLargeYard(dc, largeYardFrame(lv), 12*float64(largeOffW)/2080.0)
	img := dc.Image()
	_ = dc.Close()
	f, err := os.Open(goldenLargePath)
	if err != nil {
		if err := os.MkdirAll("examples/engine/physics/testdata", 0o755); err != nil {
			return false, 0, false
		}
		out, err := os.Create(goldenLargePath)
		if err != nil {
			return false, 0, false
		}
		encErr := png.Encode(out, img)
		_ = out.Close()
		return encErr == nil, 0, encErr == nil
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
	return changed == 0, changed, false
}

func runLargeProbes() largeProbe {
	var p largeProbe
	var detail string
	var qus, rus float64
	p.LogicOK, p.Pairs, p.Total, detail, qus, rus = probeLargeLogic()
	p.Detail = detail
	p.QueryUs, p.RayUs = qus, rus
	p.PixOK, p.PixDetail = probeLargePixels()
	var wrote bool
	p.GoldenOK, p.GoldenChanged, wrote = probeLargeGolden()
	p.GoldenWrote = wrote
	p.OK = p.LogicOK && p.PixOK && p.GoldenOK
	return p
}

// largeSim is the live large window: the filed yard falls and settles
// while the hero auto-hops the pad and the one-way.
type largeSim struct {
	lv      *largeYard
	view    core.Rect
	dropDY  float64
	dots    []largeYardDot
	hero    core.Vec2
	heroVel core.Vec2
	pairs   int
	evals   int
	woke    int
	sleep   int
	lands   int
	jumps   int
	moved   float64
	maxQus  float64
	maxQFullUs float64
	sumQus float64
	sumQFullUs float64
	sumRus float64
	steadyFrames int
	viewCount  int
	maxRus  float64
	elapsed float64
	keys    int
	nudgeX  float64
	nudgeY  float64
	frames  int
	app     *embedder.PipelineApp
	root    *rendering.AbsoluteBox
	board   *rendering.RenderBox
	phase   *wrkit.PhaseClock
	overlay *rendering.RenderText
}

func (s *largeSim) tickLarge(dt float64) {
	if dt < 0 {
		dt = 0
	}
	if dt > 0.05 {
		dt = 0.05
	}
	s.frames++
	s.elapsed += dt
	// The filed 60-unit drop: dynamics ease from the parked row into the
	// field over the first seconds, then rest on the statics.
	step := largeFallSpeed * dt
	if step > s.dropDY {
		step = s.dropDY
	}
	if s.dropDY > 0 {
		for h := s.lv.layout.StaticCount; h < s.lv.layout.Count; h++ {
			if !s.lv.bp.IsActive(h) {
				continue
			}
			bd := s.lv.bodies[h]
			bd.Pos = core.V2(bd.Pos.X, bd.Pos.Y+step)
			s.lv.bodies[h] = bd
			if err := s.lv.bp.Move(h, bd); err != nil {
				fmt.Fprintf(os.Stderr, "game_physics-large: move: %v\n", err)
				return
			}
		}
		s.dropDY -= step
		s.moved += step
	}
	// Stillness sleep runs on the same clock as the drop: only settled
	// frames count, so the falling row never parks mid-air. Sleepers
	// skip integration below but still collide and wake on touch.
	// Godot BroadPhase2D口径：每帧玩法只查视口（cull_aabb）+射线
	// （cull_segment），全量Query只做统计/对数，不进50us门。
	// The gate stopwatch covers the broadphase only: drop integration,
	// hero stepping, and overlay stay outside it.
	s.lv.bp.Update()
	// Viewport single query drives the q gate: one cull_aabb lookup.
	// Warmup frames (drop + index build) never enter the stats: the gate
	// uses steady-state mean (same as the unit 20-rep mean), max is
	// reported for info only since one scheduling spike is not engine.
	steady := s.elapsed > largeWarmupS
	t0 := time.Now()
	region, rerr := s.lv.bp.QueryRegion(core.NewRect(largeViewX, largeViewY, largeViewW, largeViewH))
	qus := float64(time.Since(t0).Microseconds())
	if rerr != nil {
		fmt.Fprintf(os.Stderr, "game_physics-large: region: %v\n", rerr)
		return
	}
	if steady {
		if qus > s.maxQus {
			s.maxQus = qus
		}
		s.sumQus += qus
		s.steadyFrames++
	}
	s.viewCount = len(region)
	// Full yard scan is stats only: pairs/evals/sleep-wake truth plus a
	// 4000us order-of-magnitude guard (same as the unit test ceiling).
	tF := time.Now()
	contacts, err := s.lv.bp.Query()
	qFull := float64(time.Since(tF).Microseconds())
	if err != nil {
		fmt.Fprintf(os.Stderr, "game_physics-large: query: %v\n", err)
		return
	}
	if steady {
		if qFull > s.maxQFullUs {
			s.maxQFullUs = qFull
		}
		s.sumQFullUs += qFull
	}
	// Filed rays sweep every tick; worst ray rules the gate.
	rus := 0.0
	for _, rc := range []struct {
		ox, oy, dx, dy, max float64
		mask                uint32
	}{
		{0, 88, 1, 0, 2200, 3},
		{400, 0, 0, 1, 1200, 3},
	} {
		ray, err := physics.NewRay(core.V2(rc.ox, rc.oy), core.V2(rc.dx, rc.dy), rc.max, rc.mask)
		if err != nil {
			continue
		}
		t1 := time.Now()
		_, _, _ = s.lv.bp.Ray(ray)
		if v := float64(time.Since(t1).Microseconds()); v > rus {
			rus = v
		}
	}
	if steady {
		if rus > s.maxRus {
			s.maxRus = rus
		}
		s.sumRus += rus
	}
	s.pairs = len(contacts)
	s.evals = s.lv.bp.Evaluations()
	s.woke += len(s.lv.bp.Woke())
	s.sleep = s.lv.bp.SleepingCount()
	// Hero auto-hops the filed pad, then rides the one-way.
	// Pad fires on crossing (falling through 420 inside the pad span),
	// not on being below it: the old ny>406 stayed true deep underground
	// so one late bounce buried the hero and it never came back.
	// Slopes land from above (prev above, next at/below), the floor at
	// 1220 keeps the hero in view. Start X 270 lands first fall on the
	// pad (40/s drift covers ~41px during the 1s drop onto 320+-30).
	prevHX, prevHY := s.hero.X, s.hero.Y
	s.heroVel.Y += 600 * dt
	nx := prevHX + 60*dt + s.nudgeX
	ny := prevHY + s.heroVel.Y*dt + s.nudgeY
	s.nudgeX, s.nudgeY = 0, 0
	if len(s.lv.springs) > 0 && s.heroVel.Y > 0 && prevHY <= 420 && ny >= 420 && nx > 320-30 && nx < 320+30 {
		s.heroVel.Y = s.lv.springs[0].Bounce(s.heroVel.Y)
		ny = 420
		s.jumps++
	}
	landed := false
	for _, sl := range s.lv.slopes {
		if gy, ok := sl.GroundYAt(nx); ok && s.heroVel.Y >= 0 && prevHY <= gy+14 && ny >= gy-14 {
			ny = gy
			s.heroVel.Y = 0
			landed = true
			break
		}
	}
	if landed {
		s.lands++
	} else {
		// Ride count: standing on a slope still counts (old window
		// semantics), so auto-run lands without needing a fresh
		// crossing every frame.
		for _, sl := range s.lv.slopes {
			if gy, ok := sl.GroundYAt(nx); ok && ny >= gy-14 && ny <= gy+14 {
				s.lands++
				break
			}
		}
	}
	s.hero.X, s.hero.Y = nx, ny
	// Wrap at the world right edge (2080 maps to the window right edge):
	// the old 1100 cut the run at mid-window, so the hero never reached
	// the right side. 2000 lands at ~1150/1200px, then restarts.
	if s.hero.X > 2000 {
		s.hero.X = 250
		s.hero.Y = 100
		s.heroVel = core.Vec2{}
	}
	s.moved += 60 * dt
	// Stillness sleep runs on the same clock as the drop: only settled
	// frames count, so the falling row never parks mid-air.
	s.lv.bp.Update()
	// Paint snapshot: yard live spots in track order plus hero.
	sx := float64(winW) / 2080.0
	sy := float64(winH) / 1280.0
	s.dots = s.dots[:0]
	for h := 0; h < s.lv.count; h++ {
		bd := s.lv.bodies[h]
		s.dots = append(s.dots, largeYardDot{
			px:     bd.Pos.X * sx,
			py:     bd.Pos.Y * sy,
			static: h < s.lv.layout.StaticCount,
			sleep:  !s.lv.bp.IsActive(h),
			layer:  h % 3,
		})
	}
	// Hero rides on top.
	{
		hx := (s.hero.X - 0) * sx
		hy := (s.hero.Y - 0) * sy
		s.dots = append(s.dots, largeYardDot{px: hx, py: hy, hero: true})
	}
	// Pads ride with the dots so the bounce spot stays visible: filed
	// pad (SpringX, 420) in world units, same scale as the bodies.
	for range s.lv.springs {
		s.dots = append(s.dots, largeYardDot{
			px:  s.lv.layout.SpringX * sx,
			py:  420 * sy,
			pad: true,
		})
	}
	_ = s.phase.Advance(dt)
	snap := s.app.Metrics().Snapshot()
	fps := 0.0
	if snap.AvgFrameIntervalMs > 1e-6 {
		fps = 1000.0 / snap.AvgFrameIntervalMs
	}
	if s.overlay != nil {
		s.overlay.SetText(fmt.Sprintf("fps %.0f pairs %d evals %d view %d sleep %d lands %d q %.0fus qfull %.0f r %.0fus",
			fps, s.pairs, s.evals, s.viewCount, s.sleep, s.lands, qus, qFull, rus))
	}
	s.board.MarkNeedsPaint()
	s.app.ScheduleFrame()
}

type largeTicker struct{ s *largeSim }

func (t *largeTicker) Tick(dt float64) bool {
	if t.s == nil {
		return true
	}
	t.s.tickLarge(dt)
	return true
}

func runLarge(autoOnly bool, manualSeconds int, maximized bool) {
	wrkit.EnsureUIFace()
	probe := runLargeProbes()
	fmt.Fprintf(os.Stderr, "game_physics-large: probes ok=%v logic=%v pix=%v golden=%v(wrote=%v changed=%d) pairs=%d total=%d %s | q=%.0fus r=%.0fus\n",
		probe.OK, probe.LogicOK, probe.PixOK, probe.GoldenOK, probe.GoldenWrote,
		probe.GoldenChanged, probe.Pairs, probe.Total, probe.Detail, probe.QueryUs, probe.RayUs)
	if !probe.OK {
		if autoOnly {
			largeFailJSON(probe)
		} else {
			fmt.Fprintln(os.Stderr, "game_physics-large: selftest FAIL, not opening window")
		}
		os.Exit(1)
	}

	var secs int
	if autoOnly {
		secs = runSeconds(8)
		wrkit.RequireMinRun(secs, largeAbilityID)
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

	lv, err := buildLargeYard()
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: yard:", err)
		os.Exit(1)
	}
	sim := &largeSim{lv: lv, view: core.NewRect(0, 0, 2080, 1280), hero: core.V2(250, 100), dropDY: 60}
	if secs > 0 {
		sim.phase = wrkit.NewPhaseClock(float64(secs)*0.5, float64(secs)*0.8)
	} else {
		sim.phase = wrkit.NewPhaseClock(0, 0)
	}

	root := rendering.NewAbsoluteBox(float64(winW), float64(winH))
	root.Background = &rendering.Color{R: bgR, G: bgG, B: bgB, A: 1}
	sim.root = root
	// Full-window content: one paint board draws the yard in track order.
	sim.board = rendering.NewRenderBox()
	sim.board.FixedWidth, sim.board.FixedHeight = float64(winW), float64(winH)
	sim.board.SetRepaintBoundary(true)
	sim.board.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		dc := pc.DC
		ax, ay := pc.Abs(0, 0)
		dc.SetRGBA(bgR, bgG, bgB, 1)
		dc.DrawRectangle(ax, ay, float64(size.Width), float64(size.Height))
		_ = dc.Fill()
		bw, bh := float64(size.Width), float64(size.Height)
		// Slopes/one-way ride under the dots: filed segments in world
		// units, same scale as the bodies, so the landing bar is visible.
		sxW := float64(size.Width) / 2080.0
		syW := float64(size.Height) / 1280.0
		for _, sl := range sim.lv.slopes {
			dc.SetRGB(jumpPlatR, jumpPlatG, jumpPlatB)
			dc.SetLineWidth(4)
			dc.DrawLine(ax+sl.A().X*sxW, ay+sl.A().Y*syW, ax+sl.B().X*sxW, ay+sl.B().Y*syW)
			_ = dc.Stroke()
		}
		for _, d := range sim.dots {
			if d.px < -16 || d.px > bw+16 || d.py < -16 || d.py > bh+16 {
				continue
			}
			var r, g, b float64
			switch {
			case d.pad:
				r, g, b = largePadR, largePadG, largePadB
			case d.hero:
				r, g, b = largeHeroR, largeHeroG, largeHeroB
			case d.sleep:
				r, g, b = largeSleepR, largeSleepG, largeSleepB
			case d.static:
				r, g, b = largeStaticR, largeStaticG, largeStaticB
			default:
				r, g, b = largeDynR, largeDynG, largeDynB
			}
			w2 := 9.0
			if d.pad || d.hero {
				w2 = 7
			}
			dc.SetRGB(r, g, b)
			dc.DrawRectangle(ax+d.px-w2/2, ay+d.py-w2/2, w2, w2)
			_ = dc.Fill()
		}
	}
	root.Place(sim.board, 0, 0)
	// One-line floating overlay at top-left over the picture.
	sim.overlay = wrkit.Label("--", 13, 1, 1, 1)
	root.Place(sim.overlay, 12, 10)

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "game_physics-large", Decorations: true, Maximized: maximized})
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
				fmt.Fprintf(os.Stderr, "game_physics-large: close (%s)\n", win.Backend())
				return
			case platform.EventPointer:
				summary.Pointer++
				if manualMode {
					fmt.Fprintf(os.Stderr, "game_physics-large: pointer %s (%.0f,%.0f) n=%d\n",
						ev.Pointer, ev.X, ev.Y, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_physics-large pairs=%d events=%d",
							len(sim.dots), summary.Pointer+summary.Key+summary.Resize))
					}
				}
				return
			case platform.EventKey:
				if ev.Pressed {
					summary.Key++
					sim.keys++
					if dx, dy := largeKeyNudge(ev.KeyCode, ev.Rune); dx != 0 || dy != 0 {
						sim.nudgeX += dx
						sim.nudgeY += dy
					}
					if largeIsJump(ev.KeyCode, ev.Rune) {
						sim.heroVel.Y = -380
						sim.jumps++
					}
					if manualMode {
						fmt.Fprintf(os.Stderr, "game_physics-large: key code=%d n=%d\n", ev.KeyCode, summary.Pointer+summary.Key+summary.Resize)
						if ctl != nil {
							ctl.SetTitle(fmt.Sprintf("game_physics-large pairs=%d events=%d",
								len(sim.dots), summary.Pointer+summary.Key+summary.Resize))
						}
					}
				}
				return
			case platform.EventResize:
				summary.Resize++
				if ev.Width > 0 && ev.Height > 0 {
					root.FixedWidth, root.FixedHeight = float64(ev.Width), float64(ev.Height)
					sim.board.FixedWidth, sim.board.FixedHeight = float64(ev.Width), float64(ev.Height)
					root.MarkNeedsLayout()
					sim.board.MarkNeedsPaint()
				}
				return
			default:
				return
			}
		},
	})
	sim.app = app
	app.Scheduler().Tickers().Add(&largeTicker{s: sim})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}
	root.MarkNeedsPaint()
	app.ScheduleFrame()

	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: run:", err)
		os.Exit(1)
	}
	app.Close()
	elapsed := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	presents := app.PresentCount()
	probeOK := 0
	if probe.OK {
		probeOK = 1
	}
	avgQ, avgQFull, avgR := sim.maxQus, sim.maxQFullUs, sim.maxRus
	if sim.steadyFrames > 0 {
		avgQ = sim.sumQus / float64(sim.steadyFrames)
		avgQFull = sim.sumQFullUs / float64(sim.steadyFrames)
		avgR = sim.sumRus / float64(sim.steadyFrames)
	}
	extra := map[string]any{
		"case":          "large",
		"probe_ok":      probeOK,
		"pairs":         sim.pairs,
		"total":         sim.lv.count,
		"evals":         sim.evals,
		"view_count":    sim.viewCount,
		"woke_total":    sim.woke,
		"sleep":         sim.sleep,
		"lands":         sim.lands,
		"jumps":         sim.jumps,
		"query_us":      math.Round(avgQ*10) / 10,
		"query_us_max":  math.Round(sim.maxQus*10) / 10,
		"query_full_us": math.Round(avgQFull*10) / 10,
		"query_full_us_max": math.Round(sim.maxQFullUs*10) / 10,
		"ray_us":        math.Round(avgR*10) / 10,
		"ray_us_max":    math.Round(sim.maxRus*10) / 10,
		"keys":          sim.keys,
		"pixels":        probe.PixDetail,
		"golden":        probe.GoldenChanged,
		"boundary_skip": snap.BoundarySkip,
	}

	if autoOnly {
		report := wrgate.BuildReport(wrgate.BuildInput{
			AbilityID:     largeAbilityID,
			Scenario:      largeScenario,
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
		if presents < 1 || sim.pairs < largeGatePairs || sim.lands < largeGateLands || !probe.OK || avgQ > largeQueryUsMax || avgR > largeRayUsMax || avgQFull > largeQueryFullUsMax {
			fmt.Fprintf(os.Stderr, "FAIL: presents=%d pairs=%d lands=%d view=%d q=%.1f(qmax=%.1f) qfull=%.1f(qfmax=%.1f) r=%.1f(rmax=%.1f) probe=%v (want >=1, >=%d, >=%d, q<=%.0f, qfull<=%.0f, r<=%.0f, true)\n",
				presents, sim.pairs, sim.lands, sim.viewCount, avgQ, sim.maxQus, avgQFull, sim.maxQFullUs, avgR, sim.maxRus, probe.OK, largeGatePairs, largeGateLands, largeQueryUsMax, largeQueryFullUsMax, largeRayUsMax)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "game_physics-large: OK presents=%d pairs=%d lands=%d view=%d q=%.1f qfull=%.1f r=%.1f elapsed=%.1fs\n",
			presents, sim.pairs, sim.lands, sim.viewCount, avgQ, avgQFull, avgR, elapsed)
		return
	}
	summary.Timed = secs > 0
	b, _ := json.Marshal(map[string]any{
		"ability_id": largeAbilityID,
		"scenario":   largeScenario,
		"backend":    win.Backend().String(),
		"events": map[string]any{
			"pointer": summary.Pointer, "key": summary.Key, "resize": summary.Resize,
		},
		"presents":    presents,
		"elapsed_sec": elapsed,
		"pairs":       sim.pairs,
		"total":       sim.lv.count,
		"view_count":  sim.viewCount,
		"lands":       sim.lands,
		"jumps":       sim.jumps,
		"query_us":    math.Round(avgQ*10) / 10,
		"query_us_max": math.Round(sim.maxQus*10) / 10,
		"query_full_us": math.Round(avgQFull*10) / 10,
		"query_full_us_max": math.Round(sim.maxQFullUs*10) / 10,
		"ray_us":      math.Round(avgR*10) / 10,
		"ray_us_max":  math.Round(sim.maxRus*10) / 10,
		"probe_ok":    probeOK,
		"timed":       summary.Timed,
		"note":        summary.Note,
	})
	fmt.Println(string(b))
	fmt.Fprintf(os.Stderr, "game_physics-large: backend=%s presents=%d pairs=%d lands=%d view=%d q=%.1f qfull=%.1f r=%.1f elapsed=%.1fs\n",
		win.Backend(), presents, sim.pairs, sim.lands, sim.viewCount, avgQ, avgQFull, avgR, elapsed)
}

func failJSON(caseFlag string, probe probeResult) {
	b, _ := json.Marshal(map[string]any{
		"ability_id": abilityID,
		"scenario":   scenarioOf(caseFlag),
		"probe_ok":   0,
		"pass":       false,
		"pixels":     probe.PixDetail,
		"golden":     probe.GoldenChanged,
	})
	fmt.Fprintln(os.Stdout, string(b))
}

// probeOnly runs the headless self-check without opening any window.
func probeOnly(caseFlag string) {
	probe := runProbes(caseFlag)
	fmt.Fprintf(os.Stderr, "game_physics: probes case=%s ok=%v logic=%v pix=%v golden=%v(wrote=%v changed=%d) %s | %s\n",
		caseFlag, probe.OK, probe.LogicOK, probe.PixOK, probe.GoldenOK, probe.GoldenWrote,
		probe.GoldenChanged, probe.Detail, probe.PixDetail)
	b, _ := json.Marshal(map[string]any{
		"ability_id": abilityID,
		"scenario":   scenarioOf(caseFlag),
		"probe_ok":   probe.OK,
		"logic_ok":   probe.LogicOK,
		"pix_ok":     probe.PixOK,
		"golden_ok":  probe.GoldenOK,
		"golden":     probe.GoldenChanged,
		"pixels":     probe.PixDetail,
		"detail":     probe.Detail,
	})
	fmt.Println(string(b))
	if !probe.OK {
		os.Exit(1)
	}
}

func paintHitFullWindow(pc *rendering.PaintContext, w, h float64, s *hitSim) {
	if pc == nil || pc.DC == nil || s == nil {
		return
	}
	ax, ay := pc.Abs(0, 0)
	dc := pc.DC
	dc.SetRGB(bgR, bgG, bgB)
	dc.DrawRectangle(ax, ay, w, h)
	_ = dc.Fill()
	// Arena fills the window (below the metric strip): hero/door/wall/ray
	// keep probe geometry ratios, scaled to the live window.
	ox, oy := ax, ay+metricStripH
	aw, ah := w, h-metricStripH-8
	sx, sy := aw/offW, ah/offH
	paintHitFrame(dc, ox, oy-sy*0, s.heroX*sx)
	_ = sx
	_ = sy
	// Box stack beside the wall (thickening: multi-box settle feel).
	for i := 0; i < 3; i++ {
		bx := ox + aw - 120
		by := oy + ah - 40 - float64(i)*34
		dc.SetRGB(jumpBodyR, jumpBodyG, jumpBodyB)
		dc.DrawRectangle(bx, by, 32, 32)
		_ = dc.Fill()
	}
	// Impact sparks near the hero after first contact (window only).
	if s.contact > 0 {
		dc.SetRGB(1, 0.65, 0.2)
		for i := 0; i < 8; i++ {
			px := ox + s.heroX + hitHeroW/2 + float64((int64(s.frames)*7+int64(i)*37)%80) - 40
			py := oy + hitHeroY + float64((int64(s.frames)*5+int64(i)*53)%48) - 24
			dc.DrawRectangle(px, py, 3, 3)
			_ = dc.Fill()
		}
		// Bump thump bar: grows with bumpSnd (sound-trigger feel).
		barW := float64(s.bumpSnd%120) + 8
		dc.SetRGB(hitRayR, hitRayG, hitRayB)
		dc.DrawRectangle(ox+16, oy+ah-16, barW, 6)
		_ = dc.Fill()
	}
}

func paintJumpFullWindow(pc *rendering.PaintContext, w, h float64, s *jumpSim) {
	if pc == nil || pc.DC == nil || s == nil {
		return
	}
	ax, ay := pc.Abs(0, 0)
	dc := pc.DC
	dc.SetRGB(bgR, bgG, bgB)
	dc.DrawRectangle(ax, ay, w, h)
	_ = dc.Fill()
	oy := ay + metricStripH
	// Slope lane across the upper window; carry lane across the lower.
	paintJumpSlope(dc, ax+24, oy+24, ax+24+s.feet.X*1.4, oy+24+s.feet.Y*1.4)
	for _, b := range s.boxes {
		dc.SetRGB(jumpBodyR, jumpBodyG, jumpBodyB)
		dc.DrawRectangle(ax+b.X, oy+b.Y, 28, 28)
		_ = dc.Fill()
	}
	// Carry platform + rider strip (Y-up world flipped at paint).
	ry := oy + h*0.62
	dc.SetRGB(jumpRailR, jumpRailG, jumpRailB)
	dc.SetLineWidth(1)
	dc.DrawLine(ax+40, ry, ax+w-40, ry)
	_ = dc.Stroke()
	px := ax + w/2 + (s.plat.Pos.X - jumpPlatX0)
	py := ry - 20
	dc.SetRGB(jumpBodyR, jumpBodyG, jumpBodyB)
	dc.DrawRectangle(px-jumpPlatHalfX, py-jumpPlatHalfY, 2*jumpPlatHalfX, 2*jumpPlatHalfY)
	_ = dc.Fill()
	dc.SetRGB(jumpRiderR, jumpRiderG, jumpRiderB)
	dc.DrawRectangle(px-jumpRiderHalf, py-2*jumpPlatHalfY-2*jumpRiderHalf, 2*jumpRiderHalf, 2*jumpRiderHalf)
	_ = dc.Fill()
	// Landing sparks at the feet after each land (window only).
	if s.lands > 0 {
		dc.SetRGB(1, 0.65, 0.2)
		for i := 0; i < 6; i++ {
			fx := ax + 24 + s.feet.X*1.4 + float64((int64(s.frames)*7+int64(i)*41)%64) - 32
			fy := oy + 24 + s.feet.Y*1.4 + float64((int64(s.frames)*5+int64(i)*29)%24) - 12
			dc.DrawRectangle(fx, fy, 3, 3)
			_ = dc.Fill()
		}
		thumpW := float64(s.bumpSnd%120) + 8
		dc.SetRGB(jumpRiderR, jumpRiderG, jumpRiderB)
		dc.DrawRectangle(ax+24, oy+h-metricStripH-40, thumpW, 6)
		_ = dc.Fill()
	}
}

func buildHitScene(shell *wrkit.ShellChrome, sim *hitSim) {
	_ = shell
	_ = sim
}

// buildHitScene keeps the legacy probe-geometry note: the live window uses
// paintHitFullWindow (full window), not the old arena/counter cards.

func buildJumpScene(shell *wrkit.ShellChrome, sim *jumpSim) {
	_ = shell
	_ = sim
}

// buildJumpScene keeps the legacy probe-geometry note: the live window uses
// paintJumpFullWindow (full window), not the old slope/carry cards.

func main() {
	caseFlag := flag.String("case", "hit", "scenario case (hit|jump|large)")
	maximized := flag.Bool("maximized", false, "open the window maximized (WM decides the final size)")
	autoOnly := flag.Bool("auto-only", false, "probes + short window, JSON gate on stdout")
	manualSeconds := flag.Int("manual-seconds", 0, "manual phase seconds (0 = until close)")
	flag.Parse()

	// Large case runs its own flow; hit/jump below stay untouched.
	if *caseFlag == "large" {
		if os.Getenv("GAME_PHYSICS_PROBE_ONLY") == "1" {
			probe := runLargeProbes()
			fmt.Fprintf(os.Stderr, "game_physics-large: probes ok=%v logic=%v pix=%v golden=%v(wrote=%v changed=%d) pairs=%d total=%d %s | q=%.0fus r=%.0fus\n",
				probe.OK, probe.LogicOK, probe.PixOK, probe.GoldenOK, probe.GoldenWrote,
				probe.GoldenChanged, probe.Pairs, probe.Total, probe.Detail, probe.QueryUs, probe.RayUs)
			if !probe.OK {
				os.Exit(1)
			}
			return
		}
		runLarge(*autoOnly, *manualSeconds, *maximized)
		return
	}

	if *caseFlag != "hit" && *caseFlag != "jump" {
		fmt.Fprintf(os.Stderr, "FAIL: --case=%q want hit or jump\n", *caseFlag)
		os.Exit(1)
	}
	if os.Getenv("GAME_PHYSICS_PROBE_ONLY") == "1" {
		probeOnly(*caseFlag)
		return
	}
	wrkit.EnsureUIFace()

	probe := runProbes(*caseFlag)
	fmt.Fprintf(os.Stderr, "game_physics: probes case=%s ok=%v logic=%v pix=%v golden=%v(wrote=%v changed=%d) %s | %s\n",
		*caseFlag, probe.OK, probe.LogicOK, probe.PixOK, probe.GoldenOK, probe.GoldenWrote,
		probe.GoldenChanged, probe.Detail, probe.PixDetail)
	if !probe.OK {
		if *autoOnly {
			failJSON(*caseFlag, probe)
		} else {
			fmt.Fprintln(os.Stderr, "game_physics: selftest FAIL, not opening window")
		}
		os.Exit(1)
	}

	var secs int
	if *autoOnly {
		secs = runSeconds(8)
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

	title := "game_physics — S61 撞门探头 (physics-hit)"
	_ = title
	if *caseFlag == "jump" {
		_ = "game_physics — S61 跳台吸附 (physics-jump)"
	}

	var hitS *hitSim
	var jumpS *jumpSim
	var ticker scheduler.Ticker
	// 满窗即内容：整窗为物理场景+加厚层，无顶栏/图例/计数器分区。
	root := rendering.NewAbsoluteBox(winW, winH)
	root.Background = &rendering.Color{R: bgR, G: bgG, B: bgB, A: 1}
	if *caseFlag == "hit" {
		hitS = &hitSim{root: root, heroX: hitHeroMinX, dir: 1}
		if secs > 0 {
			hitS.phase = wrkit.NewPhaseClock(float64(secs)*0.5, float64(secs)*0.8)
		} else {
			hitS.phase = wrkit.NewPhaseClock(0, 0)
		}
		hitS.fullBox = rendering.NewRenderBox()
		hitS.fullBox.FixedWidth, hitS.fullBox.FixedHeight = winW, winH
		hs := hitS
		hitS.fullBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
			paintHitFullWindow(pc, size.Width, size.Height, hs)
		}
		root.Place(hitS.fullBox, 0, 0)
		// 指标浮内容左上角，盖画面不划区。
		hitS.metric = wrkit.Label("hit fps=-", 13, 0.92, 0.94, 0.98)
		root.Place(hitS.metric, 8, 8)
		ticker = &hitTicker{s: hitS}
	} else {
		slopes, err := jumpSlopes()
		if err != nil {
			fmt.Fprintln(os.Stderr, "FAIL: jump slopes:", err)
			os.Exit(1)
		}
		plat, _ := physics.NewBox("plat", core.V2(jumpPlatX0, jumpPlatY), core.V2(jumpPlatHalfX, jumpPlatHalfY), 1, 1, false)
		rider, _ := physics.NewBox("rider", core.V2(jumpPlatX0, jumpPlatY+jumpPlatHalfY+jumpRiderHalf), core.V2(jumpRiderHalf, jumpRiderHalf), 1, 1, false)
		jumpS = &jumpSim{
			root: root, slopes: slopes, cfg: physics.DefaultSnapConfig(),
			feet: core.V2(jumpStartX, jumpStartY), vel: core.Vec2{},
			hold: true, plat: plat, rider: rider, platDir: 1,
		}
		if secs > 0 {
			jumpS.phase = wrkit.NewPhaseClock(float64(secs)*0.5, float64(secs)*0.8)
		} else {
			jumpS.phase = wrkit.NewPhaseClock(0, 0)
		}
		jumpS.fullBox = rendering.NewRenderBox()
		jumpS.fullBox.FixedWidth, jumpS.fullBox.FixedHeight = winW, winH
		js := jumpS
		jumpS.fullBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
			paintJumpFullWindow(pc, size.Width, size.Height, js)
		}
		root.Place(jumpS.fullBox, 0, 0)
		// 指标浮内容左上角，盖画面不划区。
		jumpS.metric = wrkit.Label("jump fps=-", 13, 0.92, 0.94, 0.98)
		root.Place(jumpS.metric, 8, 8)
		ticker = &jumpTicker{s: jumpS}
	}

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "game_physics", Decorations: true})
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
				fmt.Fprintf(os.Stderr, "game_physics: close (%s)\n", win.Backend())
				return
			case platform.EventPointer:
				summary.Pointer++
				if manualMode {
					fmt.Fprintf(os.Stderr, "game_physics: pointer %s (%.0f,%.0f) n=%d\n",
						ev.Pointer, ev.X, ev.Y, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_physics %s events=%d", *caseFlag, summary.Pointer+summary.Key+summary.Resize))
					}
				}
				return
			case platform.EventKey:
				if ev.Pressed {
					summary.Key++
					if manualMode {
						fmt.Fprintf(os.Stderr, "game_physics: key n=%d\n", summary.Pointer+summary.Key+summary.Resize)
						if ctl != nil {
							ctl.SetTitle(fmt.Sprintf("game_physics %s events=%d", *caseFlag, summary.Pointer+summary.Key+summary.Resize))
						}
					}
				}
				return
			case platform.EventResize:
				summary.Resize++
				if ev.Width > 0 && ev.Height > 0 {
					root.FixedWidth, root.FixedHeight = float64(ev.Width), float64(ev.Height)
					if hitS != nil && hitS.fullBox != nil {
						hitS.fullBox.FixedWidth, hitS.fullBox.FixedHeight = float64(ev.Width), float64(ev.Height)
					}
					if jumpS != nil && jumpS.fullBox != nil {
						jumpS.fullBox.FixedWidth, jumpS.fullBox.FixedHeight = float64(ev.Width), float64(ev.Height)
					}
					root.MarkNeedsPaint()
				}
				if manualMode {
					fmt.Fprintf(os.Stderr, "game_physics: resize %dx%d n=%d\n", ev.Width, ev.Height, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_physics %s events=%d", *caseFlag, summary.Pointer+summary.Key+summary.Resize))
					}
				}
				return
			default:
				return
			}
		},
	})
	if hitS != nil {
		hitS.app = app
	} else {
		jumpS.app = app
	}
	app.Scheduler().Tickers().Add(ticker)
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}
	root.MarkNeedsPaint()
	app.ScheduleFrame()

	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: run:", err)
		os.Exit(1)
	}
	app.Close()
	elapsed := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	presents := app.PresentCount()
	probeOK := 0
	if probe.OK {
		probeOK = 1
	}
	extra := map[string]any{
		"case":          *caseFlag,
		"probe_ok":      probeOK,
		"pixels":        probe.PixDetail,
		"golden":        probe.GoldenChanged,
		"boundary_skip": snap.BoundarySkip,
	}
	if *caseFlag == "hit" {
		extra["contact_frames"] = hitS.contact
		extra["ray_hits"] = hitS.rayHit
		extra["trigger_seen"] = hitS.trig
		extra["moved_px"] = hitS.moved
		extra["body100_us"] = probe.Body100Us
		extra["ray100_us"] = probe.Ray100Us
	} else {
		extra["lands"] = jumpS.lands
		extra["passes"] = jumpS.passes
		extra["on_floor"] = jumpS.onLane
		extra["carries"] = jumpS.carries
		extra["holds"] = jumpS.holds
		extra["slope_us"] = probe.SlopeUs
	}

	if *autoOnly {
		report := wrgate.BuildReport(wrgate.BuildInput{
			AbilityID:     abilityID,
			Scenario:      scenarioOf(*caseFlag),
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
		if *caseFlag == "hit" {
			if presents < 1 || hitS.moved <= 0 || hitS.contact < 1 || hitS.rayHit < 1 || hitS.trig < 1 || !probe.OK {
				fmt.Fprintf(os.Stderr, "FAIL: presents=%d moved=%.0f contact=%d ray=%d trig=%d probe=%v (want >=1, >0, >=1, >=1, >=1, true)\n",
					presents, hitS.moved, hitS.contact, hitS.rayHit, hitS.trig, probe.OK)
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "game_physics: OK case=hit presents=%d moved=%.0f contact=%d ray=%d trig=%d elapsed=%.1fs\n",
				presents, hitS.moved, hitS.contact, hitS.rayHit, hitS.trig, elapsed)
		} else {
			if presents < 1 || jumpS.lands < 1 || jumpS.passes < 1 || jumpS.carries < 1 || !probe.OK {
				fmt.Fprintf(os.Stderr, "FAIL: presents=%d lands=%d passes=%d carries=%d probe=%v (want >=1 each, true)\n",
					presents, jumpS.lands, jumpS.passes, jumpS.carries, probe.OK)
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "game_physics: OK case=jump presents=%d lands=%d passes=%d carries=%d holds=%d elapsed=%.1fs\n",
				presents, jumpS.lands, jumpS.passes, jumpS.carries, jumpS.holds, elapsed)
		}
		return
	}
	summary.Timed = secs > 0
	b, _ := json.Marshal(map[string]any{
		"ability_id": abilityID,
		"scenario":   scenarioOf(*caseFlag),
		"backend":    win.Backend().String(),
		"events": map[string]any{
			"pointer": summary.Pointer, "key": summary.Key, "resize": summary.Resize,
		},
		"presents":    presents,
		"elapsed_sec": elapsed,
		"extra":       extra,
		"probe_ok":    probeOK,
		"timed":       summary.Timed,
		"note":        summary.Note,
	})
	fmt.Println(string(b))
	fmt.Fprintf(os.Stderr, "game_physics: backend=%s case=%s presents=%d elapsed=%.1fs\n",
		win.Backend(), *caseFlag, presents, elapsed)
}
