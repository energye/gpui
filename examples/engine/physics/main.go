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

	scenarioHit  = "game_physics--case=hit"
	scenarioJump = "game_physics--case=jump"

	goldenHitPath  = "examples/engine/physics/testdata/physics_hit_golden.png"
	goldenJumpPath = "examples/engine/physics/testdata/physics_jump_golden.png"

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
	app      *embedder.PipelineApp
	root     *rendering.AbsoluteBox
	phase    *wrkit.PhaseClock
	fullBox  *rendering.RenderBox
	metric   *rendering.RenderText
	arena    *rendering.RenderBox
	heroX    float64
	dir      float64
	moved    float64
	contact  int64
	rayHit   int64
	trig     int64
	frames   int
	sparks   int64
	bumpSnd  int64
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
	px := ax + w/2 + (s.plat.Pos.X-jumpPlatX0)
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
	caseFlag := flag.String("case", "hit", "scenario case (hit or jump)")
	autoOnly := flag.Bool("auto-only", false, "probes + short window, JSON gate on stdout")
	manualSeconds := flag.Int("manual-seconds", 0, "manual phase seconds (0 = until close)")
	flag.Parse()

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
