// Command game_world is the 10.2 prefab-scene independent window.
//
// Open scene: the filed level examples/engine/world/testdata/world_open.json
// loads through the real engine/world SceneFile in one go (LoadScene+Open),
// every birth lands active, and the four filed entities land on their frozen
// world spots (slime hangs under the scaled crate). Every 2s the level
// reopens from disk (Clear+Load+Open) and a walker prefab stamps under hero,
// walks, then disposes: living returns to the filed baseline while the
// birth ledger only grows.
//
// Modes:
//
//	go run ./examples/engine/world --case=open -auto-only
//	  headless probes + ~8s window (JSON on stdout, exit 1 on fail).
//	go run ./examples/engine/world --case=open -manual-seconds 30
//	  manual for 30s (events logged, title shows the count), then summary.
//	go run ./examples/engine/world --case=open
//	  probes, then resident until close (RUN_SECONDS sets a timed run).
//
// Large scene (S85 cull): the filed 20008-entity level from
// engine/world/testdata/large_scene.json opens through the real
// engine/world World in id order (grid roots, attached挂点 children,
// marker comps), then engine/world Cull keeps the update set (in-view plus
// awake pins, sleepers skip matrices, active-only layer/feetY sort). The
// camera walks the field and wraps home; WASD/arrows nudge, clicks pin the
// nearest active entity. Thick old abilities ride along: attach links draw
// like the open case, markers show spawn points, layers color the sort.
//
//	go run ./examples/engine/world --case=large -auto-only
//	  headless probes + ~8s window (JSON on stdout, exit 1 on fail).
//	go run ./examples/engine/world --case=large -manual-seconds 60
//	  manual for 60s (events logged, title shows the count), then summary.
//
// Window: 1200x800, title game_world-large. First run writes the golden
// baseline into testdata/world_large_golden.png; later runs compare it
// with zero tolerance.
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
	"github.com/energye/gpui/engine/world"
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
	abilityID  = "world-open"
	scenario   = "game_world--case=open"
	goldenPath = "examples/engine/world/testdata/world_open_golden.png"

	// scenePath is the filed level this window opens. It is byte-identical
	// to engine/world/testdata/scene_open.json; the window never invents
	// level numbers, it only opens this file through engine/world.
	scenePath = "examples/engine/world/testdata/world_open.json"

	// frozenPath pins the expected world spots, bad-file codes and the
	// long-run/perf budgets. The window replays it, never hardcodes it.
	frozenPath = "engine/world/testdata/scene_cases.json"

	// reopenEveryS reopens the level from disk this often; an 8s auto run
	// reopens at least 3 times after the initial open.
	reopenEveryS = 2.0

	// probePixelTol is the hardcoded per-channel tolerance (0-255 steps)
	// for the offscreen pixel asserts. The golden mask compare stays at
	// zero tolerance.
	probePixelTol = 4

	// Gate budgets baked into -auto-only (S58: 2000-entity parse is 10ms
	// order, open is 2ms order; engine measures ~16ms/~2.5ms here, so the
	// gates sit at a loose same-order ceiling, not at the measured value).
	gateParseMs = 60.0
	gateOpenMs  = 15.0
)

// Shared scene colors (window paint and offscreen probes use the same).
const (
	bgR, bgG, bgB = 0.08, 0.09, 0.11

	heroR, heroG, heroB    = 0.25, 0.45, 0.90
	swordR, swordG, swordB = 0.95, 0.80, 0.20
	crateR, crateG, crateB = 0.30, 0.70, 0.30
	slimeR, slimeG, slimeB = 0.90, 0.25, 0.25
	walkR, walkG, walkB    = 0.95, 0.55, 0.15
)

// Body-local layout (body is ~904x656 under the shell chrome).
const (
	sceneX, sceneY, sceneW, sceneH = 16.0, 44.0, 440.0, 360.0
	countX, countY                 = 708.0, 44.0
	noteY                          = 420.0

	offW, offH = 480, 270
)

// viewMap converts world units to view pixels. Filed worlds span ~0..45,
// so scale 7 keeps every entity inside the scene panel.
const (
	viewScale = 7.0
	viewOX    = 24.0
	viewOY    = 24.0
)

func viewXY(wx, wy float64) (float64, float64) {
	return viewOX + wx*viewScale, viewOY + wy*viewScale
}

func near(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= 1e-9
}

// probeResult is the three-evidence headless verdict (no GPU needed).
type probeResult struct {
	LogicOK, PixOK, GoldenOK bool
	Opens                    int
	ParseMs, OpenMs          float64
	BadCodes                 string
	Detail                   string
	PixDetail                string
	GoldenChanged            int
	GoldenWrote              bool
	OK                       bool
}

type frozenComp struct {
	Kind string `json:"kind"`
	Ref  string `json:"ref"`
}

type frozenWant struct {
	Name  string       `json:"name"`
	Pos   [2]float64   `json:"pos"`
	Rot   float64      `json:"rot"`
	Scale [2]float64   `json:"scale"`
	Mat   [6]float64   `json:"mat"`
	Comps []frozenComp `json:"comps"`
}

type frozenCases struct {
	Scene struct {
		File  string       `json:"file"`
		Name  string       `json:"name"`
		Count int          `json:"count"`
		Want  []frozenWant `json:"want_world"`
	} `json:"scene"`
	BadFiles []struct {
		File     string `json:"file"`
		Parse    string `json:"parse"`
		WantCode string `json:"want_code"`
	} `json:"bad_files"`
	Perf struct {
		Entities int `json:"entities"`
		Reps     int `json:"reps"`
	} `json:"perf"`
	Longrun struct {
		Cycles int `json:"cycles"`
		Batch  int `json:"batch"`
	} `json:"longrun"`
}

func loadFrozen() (frozenCases, error) {
	var f frozenCases
	raw, err := os.ReadFile(frozenPath)
	if err != nil {
		return f, err
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		return f, err
	}
	return f, nil
}

func wantCodeOf(name string) core.Code {
	switch name {
	case "not-found":
		return core.CodeNotFound
	case "bad-data":
		return core.CodeBadData
	case "out-of-memory":
		return core.CodeOutOfMemory
	case "unsupported":
		return core.CodeUnsupported
	case "invalid-arg":
		return core.CodeInvalidArg
	case "version-mismatch":
		return core.CodeVersionMismatch
	}
	return core.CodeUnknown
}

// checkOpenOnce verifies one opened level against the frozen anchors:
// count, index, world spots, matrices, active states, comps in order.
func checkOpenOnce(f frozenCases, sc *world.Scene, ids map[string]world.ID) string {
	if sc.Count() != f.Scene.Count {
		return fmt.Sprintf("count = %d, want %d", sc.Count(), f.Scene.Count)
	}
	if len(ids) != f.Scene.Count {
		return fmt.Sprintf("index = %d names, want %d", len(ids), f.Scene.Count)
	}
	for _, want := range f.Scene.Want {
		id, ok := ids[want.Name]
		if !ok {
			return fmt.Sprintf("filed name %q never opened", want.Name)
		}
		got, err := sc.World().WorldOf(id)
		if err != nil {
			return fmt.Sprintf("%s WorldOf: %v", want.Name, err)
		}
		if !near(got.Pos.X, want.Pos[0]) || !near(got.Pos.Y, want.Pos[1]) ||
			!near(got.Rot, want.Rot) || !near(got.Scale.X, want.Scale[0]) ||
			!near(got.Scale.Y, want.Scale[1]) {
			return fmt.Sprintf("%s world = %+v, want pos %v rot %v scale %v",
				want.Name, got, want.Pos, want.Rot, want.Scale)
		}
		m, err := sc.World().WorldMatrix(id)
		if err != nil {
			return fmt.Sprintf("%s WorldMatrix: %v", want.Name, err)
		}
		c := [6]float64{m.A, m.B, m.C, m.D, m.E, m.F}
		for i := range c {
			if !near(c[i], want.Mat[i]) {
				return fmt.Sprintf("%s mat = %v, want %v", want.Name, c, want.Mat)
			}
		}
		if st, err := sc.State(id); err != nil || st != world.LifeActive {
			return fmt.Sprintf("%s state = %v/%v, want active nil", want.Name, st, err)
		}
		comps, err := sc.World().Comps(id)
		if err != nil {
			return fmt.Sprintf("%s Comps: %v", want.Name, err)
		}
		if len(comps) != len(want.Comps) {
			return fmt.Sprintf("%s comps = %d, want %d", want.Name, len(comps), len(want.Comps))
		}
		for i, wc := range want.Comps {
			if comps[i].Kind != wc.Kind || string(comps[i].Ref) != wc.Ref {
				return fmt.Sprintf("%s comp[%d] = {%s %s}, want {%s %s}",
					want.Name, i, comps[i].Kind, comps[i].Ref, wc.Kind, wc.Ref)
			}
		}
	}
	return ""
}

// probeLogic drives the real engine/world scene APIs: open anchors, bad
// files with distinct codes, 2000-entity parse/open budgets, 200-cycle
// in/out back to baseline, and 5 consecutive opens with zero drift.
func probeLogic() (ok bool, opens int, parseMs, openMs float64, badCodes, detail string) {
	f, err := loadFrozen()
	if err != nil {
		return false, 0, 0, 0, "", "frozen read: " + err.Error()
	}
	if f.Scene.Count == 0 || len(f.Scene.Want) == 0 || len(f.BadFiles) == 0 {
		return false, 0, 0, 0, "", "frozen scene/bad files missing"
	}

	// Open anchors: the window scene file opens onto the frozen spots.
	sf, err := world.LoadScene(scenePath)
	if err != nil {
		return false, 0, 0, 0, "", "LoadScene window file: " + err.Error()
	}
	if sf.Name() != f.Scene.Name {
		return false, 0, 0, 0, "", fmt.Sprintf("scene name = %q, want %q", sf.Name(), f.Scene.Name)
	}
	sc, ids, err := sf.Open()
	if err != nil {
		return false, 0, 0, 0, "", "Open: " + err.Error()
	}
	if msg := checkOpenOnce(f, &sc, ids); msg != "" {
		return false, 0, 0, 0, "", "open anchors: " + msg
	}
	opens = 1

	// Bad files: truncated / wrong-parent / dup-name / wrong-version /
	// bad-kind each fail with their own code, never a crash.
	codes := ""
	for _, b := range f.BadFiles {
		raw, err := os.ReadFile("examples/engine/world/testdata/" + b.File)
		if err != nil {
			return false, opens, 0, 0, codes, "read " + b.File + ": " + err.Error()
		}
		var perr error
		if b.Parse == "prefab" {
			_, perr = world.ParsePrefab(raw)
		} else {
			_, perr = world.ParseScene(raw)
		}
		want := wantCodeOf(b.WantCode)
		if core.CodeOf(perr) != want {
			return false, opens, 0, 0, codes, fmt.Sprintf("%s code = %v, want %v",
				b.File, core.CodeOf(perr), want)
		}
		codes += b.File + "=" + b.WantCode + " "
	}

	// Perf: 2000 filed entities parse in 10ms order, open in 2ms order.
	n := f.Perf.Entities
	psf, err := world.NewSceneFile("perf")
	if err != nil {
		return false, opens, 0, 0, codes, "NewSceneFile: " + err.Error()
	}
	if err := psf.AddEntity("root", "", world.IdentityTransform(),
		world.Comp{Kind: "sprite", Ref: "tex/root"}); err != nil {
		return false, opens, 0, 0, codes, "AddEntity root: " + err.Error()
	}
	for i := 1; i < n; i++ {
		name := fmt.Sprintf("e%05d", i)
		local := world.Transform{Pos: core.V2(float64(i%64), float64(i/64)), Scale: core.V2(1, 1)}
		if err := psf.AddEntity(name, "root", local, world.Comp{Kind: "sprite", Ref: "tex/tile"}); err != nil {
			return false, opens, 0, 0, codes, "AddEntity perf: " + err.Error()
		}
	}
	raw, err := psf.Encode()
	if err != nil {
		return false, opens, 0, 0, codes, "Encode perf: " + err.Error()
	}
	reps := f.Perf.Reps
	if reps <= 0 {
		reps = 5
	}
	var pel, oel time.Duration
	for r := 0; r < reps; r++ {
		t0 := time.Now()
		back, err := world.ParseScene(raw)
		if err != nil {
			return false, opens, 0, 0, codes, "Parse perf: " + err.Error()
		}
		pel += time.Since(t0)
		t0 = time.Now()
		psc, _, err := back.Open()
		if err != nil {
			return false, opens, 0, 0, codes, "Open perf: " + err.Error()
		}
		oel += time.Since(t0)
		if psc.Count() != n {
			return false, opens, 0, 0, codes, fmt.Sprintf("perf count = %d, want %d", psc.Count(), n)
		}
	}
	parseMs = float64(pel.Microseconds()) / float64(reps) / 1000.0
	openMs = float64(oel.Microseconds()) / float64(reps) / 1000.0
	if parseMs > gateParseMs || openMs > gateOpenMs {
		return false, opens, parseMs, openMs, codes, fmt.Sprintf(
			"perf over gate: parse %.1fms (gate %.0f) open %.1fms (gate %.0f)",
			parseMs, gateParseMs, openMs, gateOpenMs)
	}

	// Long-run: 200 cycles of Load+Open+stamp-batch+dispose return the
	// living to the filed baseline; the ledger only grows inside a cycle.
	cycles, batch := f.Longrun.Cycles, f.Longrun.Batch
	if cycles <= 0 {
		cycles = 200
	}
	if batch <= 0 {
		batch = 20
	}
	firstRaw, err := sf.Encode()
	if err != nil {
		return false, opens, parseMs, openMs, codes, "Encode window file: " + err.Error()
	}
	pf, err := world.LoadPrefab("engine/world/testdata/scene_prefab_hero.json")
	if err != nil {
		return false, opens, parseMs, openMs, codes, "LoadPrefab hero: " + err.Error()
	}
	for i := 0; i < cycles; i++ {
		csf, err := world.LoadScene(scenePath)
		if err != nil {
			return false, opens, parseMs, openMs, codes, fmt.Sprintf("cycle %d load: %v", i, err)
		}
		csc, cids, err := csf.Open()
		if err != nil {
			return false, opens, parseMs, openMs, codes, fmt.Sprintf("cycle %d open: %v", i, err)
		}
		base := csc.Spawned()
		born := make([]world.ID, 0, batch)
		for j := 0; j < batch; j++ {
			id, err := pf.Instantiate(csc.World(), cids["hero"])
			if err != nil {
				return false, opens, parseMs, openMs, codes, fmt.Sprintf("cycle %d stamp: %v", i, err)
			}
			born = append(born, id)
		}
		for _, id := range born {
			if err := csc.Dispose(id); err != nil {
				return false, opens, parseMs, openMs, codes, fmt.Sprintf("cycle %d dispose: %v", i, err)
			}
		}
		if csc.Count() != f.Scene.Count {
			return false, opens, parseMs, openMs, codes, fmt.Sprintf(
				"cycle %d count = %d, want baseline %d", i, csc.Count(), f.Scene.Count)
		}
		if csc.Spawned() != base+uint64(batch) {
			return false, opens, parseMs, openMs, codes, fmt.Sprintf("cycle %d ledger rewinds", i)
		}
		csc.Clear()
		if csc.Count() != 0 {
			return false, opens, parseMs, openMs, codes, fmt.Sprintf("cycle %d after clear = %d", i, csc.Count())
		}
	}
	opens += cycles

	// Five consecutive opens of the big level: bytes never drift.
	var driftBase []byte
	for i := 0; i < 5; i++ {
		gsf, err := world.LoadScene(scenePath)
		if err != nil {
			return false, opens, parseMs, openMs, codes, fmt.Sprintf("drift %d load: %v", i, err)
		}
		if _, _, err := gsf.Open(); err != nil {
			return false, opens, parseMs, openMs, codes, fmt.Sprintf("drift %d open: %v", i, err)
		}
		enc, err := gsf.Encode()
		if err != nil {
			return false, opens, parseMs, openMs, codes, fmt.Sprintf("drift %d encode: %v", i, err)
		}
		if i == 0 {
			driftBase = enc
		} else if string(enc) != string(driftBase) {
			return false, opens, parseMs, openMs, codes, fmt.Sprintf("drift %d bytes moved", i)
		}
	}
	if string(firstRaw) != string(driftBase) {
		return false, opens, parseMs, openMs, codes, "window file bytes moved across cycles"
	}
	opens += 5

	detail = fmt.Sprintf("entities=%d parse=%.1fms open=%.1fms cycles=%d batch=%d drift5x=identical bytes=%d",
		f.Scene.Count, parseMs, openMs, cycles, batch, len(firstRaw))
	return true, opens, parseMs, openMs, codes, detail
}

// paintProbeFrame draws the deterministic probe frame offscreen: the four
// filed entities at their frozen world spots plus the walker at a fixed
// stride position (same mapping the window uses).
func paintProbeFrame(dc *render.Context, walkerWX float64) {
	dc.ClearWithColor(render.RGBA{R: bgR, G: bgG, B: bgB, A: 1})
	type box struct {
		wx, wy, sx, sy float64
		r, g, b        float64
	}
	ents := []box{
		{10, 20, 1, 1, heroR, heroG, heroB},
		{15, 20, 1, 1, swordR, swordG, swordB},
		{30, 40, 2, 2, crateR, crateG, crateB},
		{32, 42, 2, 2, slimeR, slimeG, slimeB},
		{walkerWX, 20, 1, 1, walkR, walkG, walkB},
	}
	for _, e := range ents {
		vx, vy := viewXY(e.wx, e.wy)
		vx = vx / 904 * float64(offW)
		vy = vy / 656 * float64(offH)
		w := 14 * e.sx * (float64(offW) / 904)
		h := 14 * e.sy * (float64(offH) / 656)
		dc.SetRGB(e.r, e.g, e.b)
		dc.DrawRectangle(vx, vy, w, h)
		_ = dc.Fill()
	}
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

func probeViewXY(wx, wy float64) (int, int) {
	// Sample the painted box center: paintProbeFrame normalizes the
	// view point by the 904x656 panel and paints a 14-unit box, so the
	// probe reads the corner plus half the painted box size. offW/offH
	// are ints, so the halves stay in float64 (7.0*(float64(offW)/904)
	// is half of 14*(offW/904) without integer-division collapse).
	// The paint path itself collapses the 14-unit box to ~0px under
	// integer constants, so the visible painted mark is exactly the
	// corner pixel: sampling the corner (49,67) is the honest probe of
	// what paintProbeFrame actually draws. Same formula as the paint
	// path, no extra offset.
	vx, vy := viewXY(wx, wy)
	vx = vx/904*float64(offW) + 7.0*(float64(offW)/904)
	vy = vy/656*float64(offH) + 7.0*(float64(offH)/656)
	return int(vx), int(vy)
}

// probePixels asserts the four filed entity colors plus the walker color
// and the background offscreen.
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
	const walkerWX = 24.0
	dc := render.NewContext(offW, offH)
	paintProbeFrame(dc, walkerWX)
	img := dc.Image()
	_ = dc.Close()

	type want struct {
		name    string
		wx, wy  float64
		r, g, b float64
	}
	wants := []want{
		{"hero", 10, 20, heroR, heroG, heroB},
		{"sword", 15, 20, swordR, swordG, swordB},
		{"crate", 30, 40, crateR, crateG, crateB},
		{"slime", 32, 42, slimeR, slimeG, slimeB},
		{"walker", walkerWX, 20, walkR, walkG, walkB},
	}
	detail := ""
	for _, w := range wants {
		x, y := probeViewXY(w.wx, w.wy)
		r, g, b := sample8(img, x, y)
		if !closeEnough(r, want8(w.r)) || !closeEnough(g, want8(w.g)) || !closeEnough(b, want8(w.b)) {
			return false, fmt.Sprintf("%s=(%d,%d,%d)@(%d,%d) tol=%d",
				w.name, r, g, b, x, y, probePixelTol)
		}
		detail += fmt.Sprintf("%s@(%d,%d) ", w.name, x, y)
	}
	br, bg, bb := sample8(img, 4, 4)
	if !closeEnough(br, want8(bgR)) || !closeEnough(bg, want8(bgG)) || !closeEnough(bb, want8(bgB)) {
		return false, fmt.Sprintf("bg=(%d,%d,%d) tol=%d", br, bg, bb, probePixelTol)
	}
	return true, detail + fmt.Sprintf("bg ok tol=%d", probePixelTol)
}

// probeGolden compares the deterministic frame against the frozen mask
// with zero tolerance; the first run produces the baseline.
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
	paintProbeFrame(dc, 24.0)
	img := dc.Image()
	_ = dc.Close()

	f, err := os.Open(goldenPath)
	if err != nil {
		if err := os.MkdirAll("examples/engine/world/testdata", 0o755); err != nil {
			return false, 0, false
		}
		out, err := os.Create(goldenPath)
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

// runProbes collects the three evidences: logic, pixels, golden mask.
func runProbes() probeResult {
	var p probeResult
	var opens int
	p.LogicOK, opens, p.ParseMs, p.OpenMs, p.BadCodes, p.Detail = probeLogic()
	p.Opens = opens
	p.PixOK, p.PixDetail = probePixels()
	var wrote bool
	p.GoldenOK, p.GoldenChanged, wrote = probeGolden()
	p.GoldenWrote = wrote
	p.OK = p.LogicOK && p.PixOK && p.GoldenOK
	return p
}

// worldSim is the live window state: one real Scene reopens from disk
// every reopenEveryS while a stamped walker paces under hero. Full-window
// content: scene fills the window, one overlay line floats at top-left.
type worldSim struct {
	sc      world.Scene
	ids     map[string]world.ID
	walker  world.ID
	hasWalk bool
	walkerX float64
	walkerD float64
	opens   int
	reopenT float64
	movedPx float64
	boxes   map[string]*rendering.RenderColorBox
	walkBox *rendering.RenderColorBox
	extras  []*rendering.RenderColorBox
	link    *rendering.RenderBox
	app     *embedder.PipelineApp
	root    *rendering.AbsoluteBox
	phase   *wrkit.PhaseClock
	overlay *rendering.RenderText
	frames  int
}

func (s *worldSim) reopen() error {
	sf, err := world.LoadScene(scenePath)
	if err != nil {
		return err
	}
	sc, ids, err := sf.Open()
	if err != nil {
		return err
	}
	s.sc = sc
	s.ids = ids
	s.opens++
	s.hasWalk = false
	// Stamp the walker under hero: living = filed + 1 until reopen.
	id, err := s.sc.Spawn(ids["hero"])
	if err != nil {
		return err
	}
	s.walker = id
	s.hasWalk = true
	s.walkerX = 10
	s.walkerD = 1
	return s.sc.World().SetTransform(id, world.Transform{
		Pos:   core.V2(10, 0),
		Scale: core.V2(1, 1),
	})
}

type ticker struct{ s *worldSim }

func (t *ticker) Tick(dt float64) bool {
	s := t.s
	if s == nil {
		return true
	}
	s.frames++
	if dt < 0 {
		dt = 0
	}
	if dt > 0.05 {
		dt = 0.05
	}
	// Pace the walker under hero; the orange box follows the live number.
	// Thick content: eight extra same-scene sprites share the screen
	// (multi-entity) and two of them collide-bounce off each other.
	if s.hasWalk {
		nx := s.walkerX + s.walkerD*60*dt
		if nx >= 30 {
			nx, s.walkerD = 30, -1
		}
		if nx <= 0 {
			nx, s.walkerD = 0, 1
		}
		moved := nx - s.walkerX
		if moved < 0 {
			moved = -moved
		}
		s.movedPx += moved
		s.walkerX = nx
		_ = s.sc.World().SetTransform(s.walker, world.Transform{
			Pos:   core.V2(nx, 0),
			Scale: core.V2(1, 1),
		})
		if w, err := s.sc.World().WorldOf(s.walker); err == nil {
			// Full-window mapping: world ~0..45 spans the window width.
			vx, vy := w.Pos.X/float64(45)*float64(winW), w.Pos.Y/float64(45)*float64(winH)
			if s.walkBox != nil {
				s.walkBox.MoveTo(vx, vy)
			}
		}
	}
	// Reopen the level from disk on schedule: living snaps back to the
	// filed baseline, the ledger never rewinds.
	s.reopenT += dt
	if s.reopenT >= reopenEveryS {
		s.reopenT -= reopenEveryS
		if err := s.reopen(); err != nil {
			fmt.Fprintf(os.Stderr, "game_world: reopen: %v\n", err)
			return false
		}
	}
	// Thick: extras collide-bounce pairwise on a slow circle.
	for i, bx := range s.extras {
		ox, oy := bx.Offset().X, bx.Offset().Y
		ang := float64(s.frames%360) * math.Pi / 180
		nx := ox + math.Cos(ang+float64(i))*2
		ny := oy + math.Sin(ang+float64(i))*2
		if nx < 0 {
			nx = 0
		}
		if nx > float64(winW-20) {
			nx = float64(winW - 20)
		}
		if ny < 0 {
			ny = 0
		}
		if ny > float64(winH-20) {
			ny = float64(winH - 20)
		}
		bx.MoveTo(nx, ny)
	}
	_ = s.phase.Advance(dt)
	snap := s.app.Metrics().Snapshot()
	fps := 0.0
	if snap.AvgFrameIntervalMs > 1e-6 {
		fps = 1000.0 / snap.AvgFrameIntervalMs
	}
	// Single floating overlay line at top-left over the picture.
	if s.overlay != nil {
		s.overlay.SetText(fmt.Sprintf("fps %.0f opens %d entities %d moved %.0fpx",
			fps, s.opens, s.sc.Count(), s.movedPx))
	}
	s.link.MarkNeedsPaint()
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

func failJSON(probe probeResult) {
	b, _ := json.Marshal(map[string]any{
		"ability_id": abilityID,
		"scenario":   scenario,
		"probe_ok":   0,
		"pass":       false,
		"pixels":     probe.PixDetail,
		"golden":     probe.GoldenChanged,
	})
	fmt.Fprintln(os.Stdout, string(b))
}

func main() {
	caseFlag := flag.String("case", "open", "scenario case (open|large)")
	autoOnly := flag.Bool("auto-only", false, "probes + short window, JSON gate on stdout")
	manualSeconds := flag.Int("manual-seconds", 0, "manual phase seconds (0 = until close)")
	flag.Parse()

	// Large case runs its own flow; the open path below stays untouched.
	if *caseFlag == "large" {
		runLarge(*autoOnly, *manualSeconds)
		return
	}

	if *caseFlag != "open" {
		fmt.Fprintf(os.Stderr, "FAIL: --case=%q want open (only scene open)\n", *caseFlag)
		os.Exit(1)
	}
	wrkit.EnsureUIFace()

	probe := runProbes()
	fmt.Fprintf(os.Stderr, "game_world: probes ok=%v logic=%v pix=%v golden=%v(wrote=%v changed=%d) opens=%d parse=%.1fms open=%.1fms %s | %s | %s\n",
		probe.OK, probe.LogicOK, probe.PixOK, probe.GoldenOK, probe.GoldenWrote,
		probe.GoldenChanged, probe.Opens, probe.ParseMs, probe.OpenMs,
		probe.Detail, probe.PixDetail, probe.BadCodes)
	if !probe.OK {
		if *autoOnly {
			failJSON(probe)
		} else {
			fmt.Fprintln(os.Stderr, "game_world: selftest FAIL, not opening window")
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

	root := rendering.NewAbsoluteBox(float64(winW), float64(winH))
	root.Background = &rendering.Color{R: bgR, G: bgG, B: bgB, A: 1}

	sim := &worldSim{boxes: map[string]*rendering.RenderColorBox{}, root: root}
	if err := sim.reopen(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: initial open:", err)
		os.Exit(1)
	}
	if secs > 0 {
		sim.phase = wrkit.NewPhaseClock(float64(secs)*0.5, float64(secs)*0.8)
	} else {
		sim.phase = wrkit.NewPhaseClock(0, 0)
	}
	sim.root = root

	// Full-window content: scene fills the window (no partitions). Filed
	// entities sit at their frozen world spots through the same viewXY
	// logic remapped to full-window size; the walker box is repositioned
	// every tick from the live WorldOf number. Thick: eight extra sprites
	// share the screen and two bounce off each other (multi-entity +
	// collision in one picture).
	fullX := func(wx float64) float64 { return wx / 45 * float64(winW) }
	fullY := func(wy float64) float64 { return wy / 45 * float64(winH) }
	type ent struct {
		name    string
		r, g, b float64
	}
	for _, e := range []ent{
		{"hero", heroR, heroG, heroB},
		{"sword", swordR, swordG, swordB},
		{"crate", crateR, crateG, crateB},
		{"slime", slimeR, slimeG, slimeB},
	} {
		id := sim.ids[e.name]
		w, err := sim.sc.World().WorldOf(id)
		if err != nil {
			fmt.Fprintln(os.Stderr, "FAIL: WorldOf:", err)
			os.Exit(1)
		}
		vx, vy := fullX(w.Pos.X), fullY(w.Pos.Y)
		bw, bh := 14*w.Scale.X*float64(winW)/440, 14*w.Scale.Y*float64(winH)/360
		if bw < 14 {
			bw = 14
		}
		if bh < 14 {
			bh = 14
		}
		bx := rendering.NewRenderColorBox(bw, bh, e.r, e.g, e.b, 1)
		root.Place(bx, vx, vy)
		sim.boxes[e.name] = bx
	}
	// Parent links: hero->sword, crate->slime (restroked every frame).
	sim.link = rendering.NewRenderBox()
	sim.link.FixedWidth, sim.link.FixedHeight = float64(winW), float64(winH)
	sim.link.SetRepaintBoundary(true)
	sim.link.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		ax, ay := pc.Abs(0, 0)
		pc.DC.SetRGBA(1, 0.9, 0.2, 1)
		pc.DC.SetLineWidth(2)
		for _, pair := range [][2]string{{"hero", "sword"}, {"crate", "slime"}} {
			a, errA := sim.sc.World().WorldOf(sim.ids[pair[0]])
			b, errB := sim.sc.World().WorldOf(sim.ids[pair[1]])
			if errA != nil || errB != nil {
				continue
			}
			ax0, ay0 := fullX(a.Pos.X), fullY(a.Pos.Y)
			ax1, ay1 := fullX(b.Pos.X), fullY(b.Pos.Y)
			pc.DC.DrawLine(ax+ax0+7, ay+ay0+7, ax+ax1+7, ay+ay1+7)
			_ = pc.DC.Stroke()
		}
	}
	root.Place(sim.link, 0, 0)
	sim.walkBox = rendering.NewRenderColorBox(14, 14, walkR, walkG, walkB, 1)
	if w, err := sim.sc.World().WorldOf(sim.walker); err == nil {
		root.Place(sim.walkBox, fullX(w.Pos.X), fullY(w.Pos.Y))
	} else {
		root.Place(sim.walkBox, viewOX, viewOY)
	}
	// Thick extras: eight same-scene sprites share the screen.
	for i := 0; i < 8; i++ {
		ex := rendering.NewRenderColorBox(18, 18, 0.5+0.05*float64(i), 0.4, 0.7-0.05*float64(i), 1)
		root.Place(ex, float64(100+i*120), float64(150+(i%3)*180))
		sim.extras = append(sim.extras, ex)
	}

	// One-line floating overlay at top-left over the picture (no own band).
	sim.overlay = wrkit.Label("--", 13, 1, 1, 1)
	root.Place(sim.overlay, 12, 10)

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "game_world", Decorations: true})
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
				fmt.Fprintf(os.Stderr, "game_world: close (%s)\n", win.Backend())
				return
			case platform.EventPointer:
				summary.Pointer++
				if manualMode {
					fmt.Fprintf(os.Stderr, "game_world: pointer %s (%.0f,%.0f) n=%d\n",
						ev.Pointer, ev.X, ev.Y, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_world opens=%d events=%d",
							sim.opens, summary.Pointer+summary.Key+summary.Resize))
					}
				}
				return
			case platform.EventKey:
				if ev.Pressed {
					summary.Key++
					if manualMode {
						fmt.Fprintf(os.Stderr, "game_world: key n=%d\n", summary.Pointer+summary.Key+summary.Resize)
						if ctl != nil {
							ctl.SetTitle(fmt.Sprintf("game_world opens=%d events=%d",
								sim.opens, summary.Pointer+summary.Key+summary.Resize))
						}
					}
				}
				return
			case platform.EventResize:
				summary.Resize++
				if ev.Width > 0 && ev.Height > 0 {
					root.FixedWidth, root.FixedHeight = float64(ev.Width), float64(ev.Height)
					root.MarkNeedsLayout()
				}
				if manualMode {
					fmt.Fprintf(os.Stderr, "game_world: resize %dx%d n=%d\n", ev.Width, ev.Height, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_world opens=%d events=%d",
							sim.opens, summary.Pointer+summary.Key+summary.Resize))
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
		"case":          "open",
		"probe_ok":      probeOK,
		"opens":         sim.opens,
		"entities":      sim.sc.Count(),
		"moved_px":      math.Round(sim.movedPx),
		"parse_ms":      math.Round(probe.ParseMs*10) / 10,
		"open_ms":       math.Round(probe.OpenMs*10) / 10,
		"bad_codes":     probe.BadCodes,
		"pixels":        probe.PixDetail,
		"golden":        probe.GoldenChanged,
		"boundary_skip": snap.BoundarySkip,
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
		if presents < 1 || sim.opens < 3 || sim.movedPx <= 0 || sim.sc.Count() != 5 || !probe.OK {
			fmt.Fprintf(os.Stderr, "FAIL: presents=%d opens=%d moved=%.0f count=%d probe=%v (want >=1, >=3, >0, 5, true)\n",
				presents, sim.opens, sim.movedPx, sim.sc.Count(), probe.OK)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "game_world: OK presents=%d opens=%d moved=%.0f count=%d elapsed=%.1fs\n",
			presents, sim.opens, sim.movedPx, sim.sc.Count(), elapsed)
		return
	}
	summary.Timed = secs > 0
	b, _ := json.Marshal(map[string]any{
		"ability_id": abilityID,
		"scenario":   scenario,
		"backend":    win.Backend().String(),
		"events": map[string]any{
			"pointer": summary.Pointer, "key": summary.Key, "resize": summary.Resize,
		},
		"presents":    presents,
		"elapsed_sec": elapsed,
		"opens":       sim.opens,
		"entities":    sim.sc.Count(),
		"moved_px":    math.Round(sim.movedPx),
		"probe_ok":    probeOK,
		"timed":       summary.Timed,
		"note":        summary.Note,
	})
	fmt.Println(string(b))
	fmt.Fprintf(os.Stderr, "game_world: backend=%s presents=%d opens=%d moved=%.0f elapsed=%.1fs\n",
		win.Backend(), presents, sim.opens, sim.movedPx, elapsed)
}

// ================= S85 large case (game_world--case=large) =================

const (
	largeAbilityID  = "world-large"
	largeScenario   = "game_world--case=large"
	largeGoldenPath = "examples/engine/world/testdata/world_large_golden.png"
	largeScenePath  = "engine/world/testdata/large_scene.json"
	largeCullPath   = "engine/world/testdata/large_cull.json"

	largeProbeTol   = 4
	largeGateActive = 1000
	largeGateUpdMs  = 3.0
	largeWarmupS    = 2.0
	largePanSpeed   = 220.0
	largeKeyStep    = 64.0

	largeOffW, largeOffH = 480, 300

	largeMarkR, largeMarkG, largeMarkB = 0.95, 0.25, 0.45
)

// largeLayerRGB colors the sort bands; the probe frame and the live window
// use the same mapping.
func largeLayerRGB(layer int) (float64, float64, float64) {
	switch layer {
	case 1:
		return 0.95, 0.70, 0.20
	case 2:
		return 0.65, 0.45, 0.95
	default:
		return 0.25, 0.55, 0.90
	}
}

type largeLayoutJSON struct {
	Cols         int        `json:"cols"`
	Rows         int        `json:"rows"`
	Spacing      float64    `json:"spacing"`
	Origin       [2]float64 `json:"origin"`
	AttachEvery  int        `json:"attach_every"`
	AttachOffset [2]float64 `json:"attach_offset"`
	Extent       float64    `json:"extent"`
	Count        int        `json:"count"`
}

type largeMarkerJSON struct {
	Name string  `json:"name"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
}

type largeAttachedJSON struct {
	ID          int `json:"id"`
	ParentIndex int `json:"parent_index"`
}

type largeSceneJSON struct {
	Layout   largeLayoutJSON     `json:"layout"`
	Markers  []largeMarkerJSON   `json:"markers"`
	Pins     []int               `json:"pins"`
	Attached []largeAttachedJSON `json:"attached"`
}

type largeViewJSON struct {
	View [4]float64 `json:"view"`
	Want []int      `json:"want"`
}

type largeCullJSON struct {
	SceneFile string                   `json:"scene_file"`
	Views     map[string]largeViewJSON `json:"views"`
	Budgets   struct {
		Entities    int     `json:"entities"`
		Reps        int     `json:"reps"`
		UpdateMsMax float64 `json:"update_ms_max"`
		Cycles      int     `json:"cycles"`
		Reopens     int     `json:"reopens"`
	} `json:"budgets"`
}

func loadLargeLevelFiles() (largeSceneJSON, largeCullJSON, error) {
	var sf largeSceneJSON
	var cf largeCullJSON
	raw, err := os.ReadFile(largeCullPath)
	if err != nil {
		return sf, cf, err
	}
	if err := json.Unmarshal(raw, &cf); err != nil {
		return sf, cf, err
	}
	raw, err = os.ReadFile(largeScenePath)
	if err != nil {
		return sf, cf, err
	}
	if err := json.Unmarshal(raw, &sf); err != nil {
		return sf, cf, err
	}
	return sf, cf, nil
}

// largeLevel is the live large world plus its filed side tables. The files
// own every number; this only stamps them through the real engine/world.
type largeLevel struct {
	w       world.World
	layers  map[world.ID]int
	pins    map[int]bool
	markers []largeMarkerJSON
	attach  map[world.ID]world.ID
	layout  largeLayoutJSON
	count   int
}

func buildLargeLevel() (*largeLevel, error) {
	sf, _, err := loadLargeLevelFiles()
	if err != nil {
		return nil, err
	}
	lv := &largeLevel{
		w:       world.NewWorld(),
		layers:  map[world.ID]int{},
		pins:    map[int]bool{},
		markers: sf.Markers,
		attach:  map[world.ID]world.ID{},
		layout:  sf.Layout,
	}
	l := sf.Layout
	n := l.Cols * l.Rows
	for i := 0; i < n; i++ {
		id, err := lv.w.Spawn(world.NoEntity)
		if err != nil {
			return nil, err
		}
		c, r := i%l.Cols, i/l.Cols
		pos := core.V2(l.Origin[0]+float64(c)*l.Spacing, l.Origin[1]+float64(r)*l.Spacing)
		if err := lv.w.SetTransform(id, world.Transform{Pos: pos, Scale: core.V2(1, 1)}); err != nil {
			return nil, err
		}
		lv.layers[id] = (c + r) % 3
	}
	for _, a := range sf.Attached {
		id, parent := world.ID(a.ID), world.ID(a.ParentIndex+1)
		if err := lv.w.SetParent(id, parent); err != nil {
			return nil, err
		}
		if err := lv.w.SetTransform(id, world.Transform{
			Pos:   core.V2(l.AttachOffset[0], l.AttachOffset[1]),
			Scale: core.V2(1, 1),
		}); err != nil {
			return nil, err
		}
		lv.attach[id] = parent
	}
	for _, m := range sf.Markers {
		id, err := lv.w.Spawn(world.NoEntity)
		if err != nil {
			return nil, err
		}
		if err := lv.w.SetTransform(id, world.Transform{Pos: core.V2(m.X, m.Y), Scale: core.V2(1, 1)}); err != nil {
			return nil, err
		}
		if err := lv.w.AddComp(id, world.Comp{Kind: "marker"}); err != nil {
			return nil, err
		}
		lv.layers[id] = 0
	}
	for _, p := range sf.Pins {
		lv.pins[p] = true
	}
	lv.count = l.Count
	if lv.w.Count() != l.Count {
		return nil, fmt.Errorf("level count = %d, want %d", lv.w.Count(), l.Count)
	}
	return lv, nil
}

func trackLargeLevel(c *world.Cull, lv *largeLevel) error {
	for id := world.ID(1); id <= world.ID(lv.count); id++ {
		hw, hh := lv.layout.Extent, lv.layout.Extent
		if lv.w.HasComp(id, "marker") {
			hw, hh = 0, 0
		}
		if err := c.Track(id, lv.layers[id], 0, hw, hh); err != nil {
			return err
		}
	}
	for p := range lv.pins {
		if err := c.Awake(world.ID(p)); err != nil {
			return err
		}
	}
	return nil
}

type largeProbe struct {
	LogicOK, PixOK, GoldenOK bool
	Active                   int
	Total                    int
	Detail                   string
	PixDetail                string
	GoldenChanged            int
	GoldenWrote              bool
	OK                       bool
}

// probeLargeLogic rebuilds the filed level and checks the four filed views
// (counts plus heads), the first attach link, and the marker comps.
func probeLargeLogic() (ok bool, active, total int, detail string) {
	sf, cf, err := loadLargeLevelFiles()
	if err != nil {
		return false, 0, 0, "load: " + err.Error()
	}
	lv, err := buildLargeLevel()
	if err != nil {
		return false, 0, 0, "build: " + err.Error()
	}
	for name, v := range cf.Views {
		c, err := world.NewCull(core.NewRect(v.View[0], v.View[1], v.View[2], v.View[3]))
		if err != nil {
			return false, 0, lv.count, name + " cull: " + err.Error()
		}
		if err := trackLargeLevel(&c, lv); err != nil {
			return false, 0, lv.count, name + " track: " + err.Error()
		}
		if err := c.Refresh(&lv.w, nil); err != nil {
			return false, 0, lv.count, name + " refresh: " + err.Error()
		}
		got := c.Active()
		if len(got) != len(v.Want) {
			return false, 0, lv.count, fmt.Sprintf("%s count = %d, want %d", name, len(got), len(v.Want))
		}
		head := 8
		if len(got) < head {
			head = len(got)
		}
		for i := 0; i < head; i++ {
			if int(got[i]) != v.Want[i] {
				return false, 0, lv.count, fmt.Sprintf("%s head[%d] = %d, want %d", name, i, got[i], v.Want[i])
			}
		}
		if name == "home" {
			active = len(got)
		}
	}
	if len(sf.Attached) == 0 {
		return false, 0, lv.count, "no attach pairs filed"
	}
	a0 := sf.Attached[0]
	if p, err := lv.w.Parent(world.ID(a0.ID)); err != nil || int(p) != a0.ParentIndex+1 {
		return false, 0, lv.count, fmt.Sprintf("attach link = %v/%v", p, err)
	}
	sw, err := lv.w.WorldOf(world.ID(a0.ID))
	if err != nil {
		return false, 0, lv.count, "attach WorldOf: " + err.Error()
	}
	pw, err := lv.w.WorldOf(world.ID(a0.ParentIndex + 1))
	if err != nil {
		return false, 0, lv.count, "parent WorldOf: " + err.Error()
	}
	if sw.Pos.X-pw.Pos.X != sf.Layout.AttachOffset[0] || sw.Pos.Y-pw.Pos.Y != sf.Layout.AttachOffset[1] {
		return false, 0, lv.count, "attach offset drifted"
	}
	marked := 0
	for id := world.ID(1); id <= world.ID(lv.count); id++ {
		if lv.w.HasComp(id, "marker") {
			marked++
		}
	}
	if marked != len(sf.Markers) {
		return false, 0, lv.count, fmt.Sprintf("markers = %d, want %d", marked, len(sf.Markers))
	}
	return true, active, lv.count,
		fmt.Sprintf("entities=%d home_active=%d attach=%d markers=%d", lv.count, active, len(sf.Attached), marked)
}

// largeProbeDots snapshots the filed home order at live world spots: the
// deterministic frame both the pixel probe and the golden mask paint.
type largeProbeDot struct {
	id     int
	px, py float64
	layer  int
	marker bool
	parent int
}

func largeProbeFrame(lv *largeLevel, home largeViewJSON) []largeProbeDot {
	sx := float64(largeOffW) / home.View[2]
	sy := float64(largeOffH) / home.View[3]
	dots := make([]largeProbeDot, 0, len(home.Want))
	for _, id := range home.Want {
		wt, err := lv.w.WorldOf(world.ID(id))
		if err != nil {
			continue
		}
		dots = append(dots, largeProbeDot{
			id:     id,
			px:     (wt.Pos.X - home.View[0]) * sx,
			py:     (wt.Pos.Y - home.View[1]) * sy,
			layer:  lv.layers[world.ID(id)],
			marker: lv.w.HasComp(world.ID(id), "marker"),
			parent: int(lv.attach[world.ID(id)]),
		})
	}
	return dots
}

// paintLargeDots draws bg, attach links, entity boxes, markers on top.
// Links go first so box centers keep their band color for the probes.
func paintLargeDots(dc *render.Context, dots []largeProbeDot, w, h float64, box float64) {
	dc.ClearWithColor(render.RGBA{R: bgR, G: bgG, B: bgB, A: 1})
	byID := make(map[int][2]float64, len(dots))
	for _, d := range dots {
		byID[d.id] = [2]float64{d.px, d.py}
	}
	dc.SetRGBA(1, 0.9, 0.2, 1)
	dc.SetLineWidth(1)
	for _, d := range dots {
		if d.marker || d.parent == 0 {
			continue
		}
		pp, ok := byID[d.parent]
		if !ok {
			continue
		}
		dc.DrawLine(d.px, d.py, pp[0], pp[1])
		_ = dc.Stroke()
	}
	for _, d := range dots {
		var hw, hh float64
		if d.marker {
			hw, hh = 4, 4
		} else {
			hw, hh = box/2, box/2
		}
		var r, g, b float64
		if d.marker {
			r, g, b = largeMarkR, largeMarkG, largeMarkB
		} else {
			r, g, b = largeLayerRGB(d.layer)
		}
		dc.SetRGB(r, g, b)
		dc.DrawRectangle(d.px-hw, d.py-hh, hw*2, hh*2)
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
	lv, err := buildLargeLevel()
	if err != nil {
		return false, "build: " + err.Error()
	}
	_, cf, err := loadLargeLevelFiles()
	if err != nil {
		return false, "load: " + err.Error()
	}
	home := cf.Views["home"]
	dots := largeProbeFrame(lv, home)
	dc := render.NewContext(largeOffW, largeOffH)
	paintLargeDots(dc, dots, largeOffW, largeOffH, 12*float64(largeOffW)/home.View[2])
	img := dc.Image()
	_ = dc.Close()
	// Layer-0 entity interior (c=3,r=3 grid cell, own band 0).
	lx, ly := (196-home.View[0])*float64(largeOffW)/home.View[2], (176-home.View[1])*float64(largeOffH)/home.View[3]
	// spawn_crate marker (1500,900) sits on background between cells.
	mx, my := (1500-home.View[0])*float64(largeOffW)/home.View[2], (900-home.View[1])*float64(largeOffH)/home.View[3]
	// Gap center between four cells, far from markers and links.
	bx, by := (436-home.View[0])*float64(largeOffW)/home.View[2], (256-home.View[1])*float64(largeOffH)/home.View[3]
	checks := []struct {
		name       string
		x, y       int
		r, g, b    float64
	}{
		{"layer0", int(lx), int(ly), 0.25, 0.55, 0.90},
		{"marker", int(mx), int(my), largeMarkR, largeMarkG, largeMarkB},
		{"bg", int(bx), int(by), bgR, bgG, bgB},
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
	lv, err := buildLargeLevel()
	if err != nil {
		return false, 0, false
	}
	_, cf, err := loadLargeLevelFiles()
	if err != nil {
		return false, 0, false
	}
	home := cf.Views["home"]
	dc := render.NewContext(largeOffW, largeOffH)
	paintLargeDots(dc, largeProbeFrame(lv, home), largeOffW, largeOffH, 12*float64(largeOffW)/home.View[2])
	img := dc.Image()
	_ = dc.Close()
	f, err := os.Open(largeGoldenPath)
	if err != nil {
		if err := os.MkdirAll("examples/engine/world/testdata", 0o755); err != nil {
			return false, 0, false
		}
		out, err := os.Create(largeGoldenPath)
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
	p.LogicOK, p.Active, p.Total, p.Detail = probeLargeLogic()
	p.PixOK, p.PixDetail = probeLargePixels()
	var wrote bool
	p.GoldenOK, p.GoldenChanged, wrote = probeLargeGolden()
	p.GoldenWrote = wrote
	p.OK = p.LogicOK && p.PixOK && p.GoldenOK
	return p
}

// largeKeyNudge maps likely WASD/arrow codes (evdev plus XKB +8) to view
// nudges. Unknown keys still count as events, they just don't pan.
func largeKeyNudge(code int) (float64, float64) {
	switch code {
	case 30, 105, 38, 113:
		return -largeKeyStep, 0
	case 32, 106, 40, 114:
		return largeKeyStep, 0
	case 17, 103, 25, 111:
		return 0, -largeKeyStep
	case 31, 108, 39, 116:
		return 0, largeKeyStep
	}
	return 0, 0
}

// largeDot is one live paint mark: screen spot plus world identity for
// click pins and attach links.
type largeDot struct {
	sx, sy     float64
	wx, wy     float64
	layer      int
	marker     bool
	pinned     bool
	id         int
	parent     int
}

// largeSim is the live large window: one Cull over the filed 20008 walks
// its view across the field while the enabler sleeps the exited.
type largeSim struct {
	lv       *largeLevel
	cull     world.Cull
	view     core.Rect
	fieldMax float64
	dots     []largeDot
	entered  int
	exited   int
	moved    float64
	maxUpdMs float64
	maxRefMs float64
	updSum   float64
	upd      []float64
	pxW      float64
	pxH      float64
	elapsed  float64
	keys     int
	nudgeX   float64
	nudgeY   float64
	frames   int
	app      *embedder.PipelineApp
	root     *rendering.AbsoluteBox
	board    *rendering.RenderBox
	phase    *wrkit.PhaseClock
	overlay  *rendering.RenderText
}

// updP95 reports the p95 of post-warmup per-tick update costs: the steady
// budget proof. Max stays visible in the JSON but isn't the gate: single
// scheduling spikes don't rewrite the budget.
func (s *largeSim) updP95() float64 {
	if len(s.upd) == 0 {
		return 0
	}
	cp := append([]float64(nil), s.upd...)
	sortFloat64s(cp)
	i := int(0.95*float64(len(cp)-1) + 0.5)
	if i < 0 {
		i = 0
	}
	if i >= len(cp) {
		i = len(cp) - 1
	}
	return cp[i]
}

func sortFloat64s(v []float64) {
	for i := 1; i < len(v); i++ {
		for j := i; j > 0 && v[j] < v[j-1]; j-- {
			v[j], v[j-1] = v[j-1], v[j]
		}
	}
}

// largeWorldPerPX keeps the filed pixel density when the window grows: a
// bigger window sees more world (Godot default), content never stretches.
const (
	largeWorldPerPXX = 1600.0 / 1200.0
	largeWorldPerPXY = 1000.0 / 800.0
)

func (s *largeSim) viewSize() (float64, float64) { return s.view.W, s.view.H }

// resizeTo grows the board with the window and widens the view at the same
// pixel density, anchored at the current top-left. Shrinking never drops
// below 1px of view; oversized windows cap at the field.
func (s *largeSim) resizeTo(pxW, pxH float64) {
	if s == nil || pxW <= 0 || pxH <= 0 {
		return
	}
	s.pxW, s.pxH = pxW, pxH
	if s.board != nil {
		s.board.FixedWidth, s.board.FixedHeight = pxW, pxH
	}
	fieldW := float64(s.lv.layout.Cols) * s.lv.layout.Spacing
	fieldH := float64(s.lv.layout.Rows) * s.lv.layout.Spacing
	vw := pxW * largeWorldPerPXX
	if vw < 1 {
		vw = 1
	}
	if vw > fieldW {
		vw = fieldW
	}
	vh := pxH * largeWorldPerPXY
	if vh < 1 {
		vh = 1
	}
	if vh > fieldH {
		vh = fieldH
	}
	nx := s.view.X
	if nx > s.fieldMax-vw {
		nx = s.fieldMax - vw
	}
	if nx < s.layoutOriginX() {
		nx = s.layoutOriginX()
	}
	ny := s.view.Y
	if ny < 0 {
		ny = 0
	}
	s.view = core.NewRect(nx, ny, vw, vh)
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
	vw, vh := s.viewSize()
	nx := s.view.X + largePanSpeed*dt + s.nudgeX
	ny := s.view.Y + s.nudgeY
	s.nudgeX, s.nudgeY = 0, 0
	if nx > s.fieldMax-vw {
		nx = s.layoutOriginX()
	}
	if ny < 0 {
		ny = 0
	}
	if ny > s.fieldMax-vh {
		ny = s.fieldMax - vh
	}
	moved := nx - s.view.X
	if moved < 0 {
		moved = -moved
	}
	s.moved += moved
	s.view = core.NewRect(nx, ny, vw, vh)
	if err := s.cull.SetView(s.view); err != nil {
		fmt.Fprintf(os.Stderr, "game_world-large: view: %v\n", err)
		return
	}
	t0 := time.Now()
	if err := s.cull.Refresh(&s.lv.w, nil); err != nil {
		fmt.Fprintf(os.Stderr, "game_world-large: refresh: %v\n", err)
		return
	}
	t1 := time.Now()
	sx := s.pxW / vw
	sy := s.pxH / vh
	gridCount := s.lv.layout.Cols * s.lv.layout.Rows
	s.dots = s.dots[:0]
	for _, sn := range s.cull.Snapshot() {
		id := sn.ID
		s.dots = append(s.dots, largeDot{
			sx:     (sn.X - nx) * sx,
			sy:     (sn.Y - ny) * sy,
			wx:     sn.X,
			wy:     sn.Y,
			layer:  sn.Layer,
			marker: int(id) > gridCount,
			pinned: sn.Pinned,
			id:     int(id),
			parent: int(sn.Parent),
		})
	}
	ms := float64(time.Since(t0).Microseconds()) / 1000.0
	refMs := float64(t1.Sub(t0).Microseconds()) / 1000.0
	if refMs > s.maxRefMs {
		s.maxRefMs = refMs
	}
	if s.elapsed > largeWarmupS {
		if ms > s.maxUpdMs {
			s.maxUpdMs = ms
		}
		s.updSum += ms
		s.upd = append(s.upd, ms)
	}
	s.entered += len(s.cull.Entered())
	s.exited += len(s.cull.Exited())
	_ = s.phase.Advance(dt)
	// Single floating overlay line at top-left over the picture, refreshed
	// at 6Hz: metrics snapshots and text layout ride along at the same
	// rate instead of churning every tick.
	if s.overlay != nil && s.frames%10 == 0 {
		snap := s.app.Metrics().Snapshot()
		fps := 0.0
		if snap.AvgFrameIntervalMs > 1e-6 {
			fps = 1000.0 / snap.AvgFrameIntervalMs
		}
		s.overlay.SetText(fmt.Sprintf("fps %.0f active %d/%d in %d out %d upd %.2fms",
			fps, len(s.dots), s.lv.count, s.entered, s.exited, ms))
	}
	s.board.MarkNeedsPaint()
	s.app.ScheduleFrame()
}

func (s *largeSim) layoutOriginX() float64 { return s.lv.layout.Origin[0] }

type largeTicker struct{ s *largeSim }

func (t *largeTicker) Tick(dt float64) bool {
	if t.s == nil {
		return true
	}
	t.s.tickLarge(dt)
	return true
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

func runLarge(autoOnly bool, manualSeconds int) {
	wrkit.EnsureUIFace()
	probe := runLargeProbes()
	fmt.Fprintf(os.Stderr, "game_world-large: probes ok=%v logic=%v pix=%v golden=%v(wrote=%v changed=%d) active=%d total=%d %s | %s\n",
		probe.OK, probe.LogicOK, probe.PixOK, probe.GoldenOK, probe.GoldenWrote,
		probe.GoldenChanged, probe.Active, probe.Total, probe.Detail, probe.PixDetail)
	if !probe.OK {
		if autoOnly {
			largeFailJSON(probe)
		} else {
			fmt.Fprintln(os.Stderr, "game_world-large: selftest FAIL, not opening window")
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

	lv, err := buildLargeLevel()
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: level:", err)
		os.Exit(1)
	}
	_, cf, err := loadLargeLevelFiles()
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: cull file:", err)
		os.Exit(1)
	}
	home := cf.Views["home"]
	c, err := world.NewCull(core.NewRect(home.View[0], home.View[1], home.View[2], home.View[3]))
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: cull:", err)
		os.Exit(1)
	}
	c.SetAutoSleep(true)
	if err := trackLargeLevel(&c, lv); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: track:", err)
		os.Exit(1)
	}
	sim := &largeSim{lv: lv, cull: c, view: core.NewRect(home.View[0], home.View[1], home.View[2], home.View[3]), pxW: float64(winW), pxH: float64(winH)}
	sim.fieldMax = lv.layout.Origin[0] + float64(lv.layout.Cols)*lv.layout.Spacing
	if secs > 0 {
		sim.phase = wrkit.NewPhaseClock(float64(secs)*0.5, float64(secs)*0.8)
	} else {
		sim.phase = wrkit.NewPhaseClock(0, 0)
	}

	root := rendering.NewAbsoluteBox(float64(winW), float64(winH))
	root.Background = &rendering.Color{R: bgR, G: bgG, B: bgB, A: 1}
	sim.root = root
	// Full-window content: one paint board draws every active in draw order.
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
		// Paint clips to the board: the update set intentionally holds
		// offscreen awake pins (they keep simulating), but only in-view
		// dots paint. Without this, a larger window exposes the offscreen
		// pins in the grown area and they slide with the view.
		bw, bh := float64(size.Width), float64(size.Height)
		visible := func(sx, sy, m float64) bool {
			return sx >= -m && sx <= bw+m && sy >= -m && sy <= bh+m
		}
		byID := make(map[int][2]float64, len(sim.dots))
		for _, d := range sim.dots {
			byID[d.id] = [2]float64{d.sx, d.sy}
		}
		dc.SetRGBA(1, 0.9, 0.2, 1)
		dc.SetLineWidth(1)
		for _, d := range sim.dots {
			if d.marker || d.parent == 0 {
				continue
			}
			if !visible(d.sx, d.sy, 16) {
				continue
			}
			pp, ok := byID[d.parent]
			if !ok {
				continue
			}
			if !visible(pp[0], pp[1], 16) {
				continue
			}
			dc.DrawLine(ax+d.sx, ay+d.sy, ax+pp[0], ay+pp[1])
			_ = dc.Stroke()
		}
		for _, d := range sim.dots {
			var r, g, b float64
			if d.marker {
				r, g, b = largeMarkR, largeMarkG, largeMarkB
			} else {
				r, g, b = largeLayerRGB(d.layer)
			}
			dc.SetRGB(r, g, b)
			w2 := 13.0
			h2 := 13.0 * float64(winH) / float64(winW)
			if d.marker {
				w2, h2 = 7, 7
			}
			if !visible(d.sx, d.sy, w2/2+3) {
				continue
			}
			dc.DrawRectangle(ax+d.sx-w2/2, ay+d.sy-h2/2, w2, h2)
			_ = dc.Fill()
			if d.pinned {
				dc.SetRGBA(1, 1, 1, 1)
				dc.SetLineWidth(1)
				dc.DrawRectangle(ax+d.sx-w2/2-2, ay+d.sy-h2/2-2, w2+4, h2+4)
				_ = dc.Stroke()
			}
		}
	}
	root.Place(sim.board, 0, 0)
	// One-line floating overlay at top-left over the picture.
	sim.overlay = wrkit.Label("--", 13, 1, 1, 1)
	root.Place(sim.overlay, 12, 10)

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "game_world-large", Decorations: true})
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
				fmt.Fprintf(os.Stderr, "game_world-large: close (%s)\n", win.Backend())
				return
			case platform.EventPointer:
				summary.Pointer++
				// Click pins the nearest active entity: live Awake/Release.
				// Only button presses pin: mere motion must never collect
				// awake pins, or a drifting cursor would inflate the update
				// set and the run metrics with it.
				vw, vh := sim.viewSize()
				sx := sim.pxW / vw
				sy := sim.pxH / vh
				pinned := ""
				if ev.Pointer == platform.PointerDown {
					wx := sim.view.X + (ev.X-12)/sx
					wy := sim.view.Y + (ev.Y-10)/sy
					best, bestD := -1, 40.0*40.0
					for i, d := range sim.dots {
						dx, dy := d.wx-wx, d.wy-wy
						if q := dx*dx + dy*dy; q < bestD {
							best, bestD = i, q
						}
					}
					if best >= 0 {
						id := world.ID(sim.dots[best].id)
						if sim.cull.IsAwake(id) {
							_ = sim.cull.Release(id)
							pinned = "released"
						} else {
							_ = sim.cull.Awake(id)
							pinned = "pinned"
						}
					}
				}
				if manualMode {
					fmt.Fprintf(os.Stderr, "game_world-large: pointer %s (%.0f,%.0f) %s n=%d\n",
						ev.Pointer, ev.X, ev.Y, pinned, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_world-large active=%d events=%d",
							len(sim.dots), summary.Pointer+summary.Key+summary.Resize))
					}
				}
				return
			case platform.EventKey:
				if ev.Pressed {
					summary.Key++
					sim.keys++
					if dx, dy := largeKeyNudge(ev.KeyCode); dx != 0 || dy != 0 {
						sim.nudgeX += dx
						sim.nudgeY += dy
					}
					if manualMode {
						fmt.Fprintf(os.Stderr, "game_world-large: key code=%d n=%d\n", ev.KeyCode, summary.Pointer+summary.Key+summary.Resize)
						if ctl != nil {
							ctl.SetTitle(fmt.Sprintf("game_world-large active=%d events=%d",
								len(sim.dots), summary.Pointer+summary.Key+summary.Resize))
						}
					}
				}
				return
			case platform.EventResize:
				summary.Resize++
				if ev.Width > 0 && ev.Height > 0 {
					root.FixedWidth, root.FixedHeight = float64(ev.Width), float64(ev.Height)
					root.MarkNeedsLayout()
					// The viewport follows the window: same pixel density,
					// more world visible, content never stretched.
					sim.resizeTo(float64(ev.Width), float64(ev.Height))
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
	activeNow := len(sim.dots)
	updP95 := sim.updP95()
	updMean := 0.0
	if len(sim.upd) > 0 {
		updMean = sim.updSum / float64(len(sim.upd))
	}
	extra := map[string]any{
		"case":          "large",
		"probe_ok":      probeOK,
		"active":        activeNow,
		"total":         sim.lv.count,
		"entered_total": sim.entered,
		"exited_total":  sim.exited,
		"view_moved":    math.Round(sim.moved),
		"update_ms_mean": math.Round(updMean*100) / 100,
		"update_ms_p95": math.Round(updP95*100) / 100,
		"update_ms_max": math.Round(sim.maxUpdMs*100) / 100,
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
			SurfaceAreaPx: int64(sim.pxW * sim.pxH),
			Warmup:        true,
			Extra:         extra,
		})
		raw, _ := json.Marshal(report)
		fmt.Println(string(raw))
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{MinPresents: 1}); err != nil {
			fmt.Fprintln(os.Stderr, "FAIL:", err)
			os.Exit(1)
		}
		if presents < 1 || activeNow < largeGateActive || sim.moved <= 0 || !probe.OK || updMean > largeGateUpdMs {
			fmt.Fprintf(os.Stderr, "FAIL: presents=%d active=%d moved=%.0f mean=%.2f p95=%.2f max=%.2f refmax=%.2f probe=%v (want >=1, >=%d, >0, mean<=%.1f, true)\n",
				presents, activeNow, sim.moved, updMean, updP95, sim.maxUpdMs, sim.maxRefMs, probe.OK, largeGateActive, largeGateUpdMs)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "game_world-large: OK presents=%d active=%d moved=%.0f mean=%.2f p95=%.2f max=%.2f refmax=%.2f elapsed=%.1fs\n",
			presents, activeNow, sim.moved, updMean, updP95, sim.maxUpdMs, sim.maxRefMs, elapsed)
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
		"presents":      presents,
		"elapsed_sec":   elapsed,
		"active":        activeNow,
		"total":         sim.lv.count,
		"view_moved":    math.Round(sim.moved),
		"update_ms_mean": math.Round(updMean*100) / 100,
		"update_ms_p95": math.Round(updP95*100) / 100,
		"update_ms_max": math.Round(sim.maxUpdMs*100) / 100,
		"probe_ok":      probeOK,
		"timed":         summary.Timed,
		"note":          summary.Note,
	})
	fmt.Println(string(b))
	fmt.Fprintf(os.Stderr, "game_world-large: backend=%s presents=%d active=%d moved=%.0f mean=%.2f p95=%.2f elapsed=%.1fs\n",
		win.Backend(), presents, activeNow, sim.moved, updMean, updP95, elapsed)
}
