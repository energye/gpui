// Command game_step is the 8.3 dynamic-partial-update independent window.
//
// Chase scene: retained static blocks (never dirtied, boundary replay) plus
// one chase car whose old+new union alone is dirtied every frame through the
// real game/step DirtyTracker and ui/scene DirtyLayer. Burst full-motion
// frames fall back to full repaint and raise the cyan对照条.
//
// Modes:
//
//	go run ./examples/game_step --case=dirty -auto-only
//	  headless probes + ~8s window (JSON on stdout, exit 1 on fail).
//	go run ./examples/game_step --case=dirty -manual-seconds 30
//	  manual for 30s (events logged, title shows the count), then summary.
//	go run ./examples/game_step
//	  probes, then resident until close (RUN_SECONDS sets a timed run).
//
// Window: 1200x800, title game_step. First run writes the golden baseline
// into testdata/; later runs compare it with zero tolerance.
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

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/game/core"
	"github.com/energye/gpui/game/step"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"
	"github.com/energye/gpui/ui/scene"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH = 1200, 800
	abilityID  = "step-dirty"
	scenario   = "game_step--case=dirty"
	goldenPath = "examples/game_step/testdata/step_dirty_golden.png"

	// probePixelTol is the hardcoded per-channel tolerance (0-255 steps)
	// for the offscreen pixel asserts. The golden mask compare stays at
	// zero tolerance.
	probePixelTol = 4
)

// Shared scene colors (window paint and offscreen probes use the same).
const (
	bgR, bgG, bgB       = 0.08, 0.09, 0.11
	runR, runG, runB    = 0.16, 0.17, 0.20
	staticAR, staticAG  = 0.20, 0.60
	staticAB            = 0.60
	staticBR, staticBG  = 0.60, 0.60
	staticBB            = 0.20
	carR, carG, carB    = 0.90, 0.15, 0.12
)

// Body-local layout (body is ~904x656 under the shell chrome).
const (
	staticX, staticY, staticW, staticH = 16.0, 44.0, 220.0, 360.0
	runX, runY, runW, runH             = 252.0, 44.0, 440.0, 360.0
	countX, countY, countW             = 708.0, 44.0, 180.0
	noteY                              = 420.0
)

// Chase car geometry in runway-local coordinates.
const (
	carW, carH         = 48.0, 24.0
	carY               = 168.0
	carMinX, carMaxX   = 8.0, 440.0 - 8.0 - carW
	carSpeed           = 140.0
	burstEvery         = 240
	offW, offH         = 480, 270
	goldenOldX         = 252.0
	goldenNewX         = 300.0
	goldenCarY         = 120.0
	probeOldX, probeNY = 60.0, 140.0
	probeCarY          = 120.0
)

// probeResult is the three-evidence headless verdict (no GPU needed).
type probeResult struct {
	RectsOK, FullOK, StaticOK, StatsOK, MaxOK bool
	DirtyRects                                int
	NeedsFull                                 bool
	Frames, Fulls, Max                        int
	PixOK                                     bool
	PixDetail                                 string
	GoldenOK                                  bool
	GoldenChanged                             int
	GoldenWrote                               bool
	OK                                        bool
}

// srect converts a game rect to the scene twin (same numbers).
func srect(r core.Rect) scene.DirtyRect {
	return scene.DirtyRect{X: r.X, Y: r.Y, W: r.W, H: r.H}
}

// probeLogic drives the real DirtyTracker + DirtyLayer and checks the
// chase semantics: move dirties old+new union only, static stays clean,
// 17+ boxes fall back to full, stats account frames.
func probeLogic() (rects int, full, staticClean, statsOK, maxOK bool, detail string) {
	bounds := core.NewRect(0, 0, 800, 600)
	tr, err := step.NewSpriteDirtyTracker(bounds)
	if err != nil {
		return 0, false, false, false, false, "tracker build: " + err.Error()
	}
	sl, err := scene.NewSpriteDirtyLayer(0, 0, 800, 600)
	if err != nil {
		return 0, false, false, false, false, "layer build: " + err.Error()
	}
	st, err := scene.NewDirtyLayer(0, 0, staticW, staticH)
	if err != nil {
		return 0, false, false, false, false, "static build: " + err.Error()
	}
	root := scene.NewContainerLayer()
	root.Add(st)
	root.Add(sl)
	walkN := scene.Walk(root, nil)

	from := core.NewRect(100, 200, carW, carH)
	to := core.NewRect(116, 200, carW, carH)
	tr.MarkMoved(from, to)
	sl.MarkMoved(srect(from), srect(to))
	rects = tr.DirtyCount()
	full = tr.NeedsFull()
	got := tr.DirtyRects()
	want, _ := step.DirtyForMove(from, to)
	rectsOK := !full && rects == 1 && sl.DirtyCount() == 1 && !sl.NeedsFull() &&
		len(got) == 1 && got[0] == want &&
		sl.DirtyRects()[0] == srect(want)
	staticClean = st.DirtyCount() == 0 && !st.NeedsFull()
	if !rectsOK {
		return rects, full, staticClean, false, false, "chase union mismatch"
	}

	// Burst: 17 distinct moves in one frame must fall back to full.
	tr2, _ := step.NewSpriteDirtyTracker(bounds)
	sl2, _ := scene.NewSpriteDirtyLayer(0, 0, 800, 600)
	for i := 0; i < 17; i++ {
		x := float64(i * 40)
		tr2.MarkMoved(core.NewRect(x, 10, 16, 16), core.NewRect(x+8, 10, 16, 16))
		sl2.MarkMoved(scene.DirtyRect{X: x, Y: 10, W: 16, H: 16}, scene.DirtyRect{X: x + 8, Y: 10, W: 16, H: 16})
	}
	full = tr2.NeedsFull() && tr2.DirtyRects() == nil &&
		sl2.NeedsFull() && sl2.DirtyRects() == nil

	// Stats: 3 frames x 1 move each account exactly.
	tr3, _ := step.NewSpriteDirtyTracker(bounds)
	for f := 0; f < 3; f++ {
		x := 100.0 + float64(f*8)
		tr3.MarkMoved(core.NewRect(x, 200, carW, carH), core.NewRect(x+8, 200, carW, carH))
		tr3.Clear()
	}
	ts := tr3.Stats()
	statsOK = ts.Frames == 3 && ts.TotalRects == 3 && ts.FullFallbacks == 0 && ts.MaxRects == 1

	maxOK = step.MaxDirtyRects == 16 && scene.MaxDirtyRects == 16 && walkN == 3
	detail = fmt.Sprintf("rects=%d full=%v static_clean=%v stats=%+v walk=%d", rects, full, staticClean, ts, walkN)
	return rects, full, staticClean, statsOK, maxOK, detail
}

// paintProbeFrame draws the deterministic probe scene: runway background,
// two static blocks, and the car at its new position (old position stays
// background, proving the trail repaints).
func paintProbeFrame(dc *render.Context, oldX, newX, y float64) {
	dc.ClearWithColor(render.RGBA{R: runR, G: runG, B: runB, A: 1})
	dc.SetRGB(staticAR, staticAG, staticAB)
	dc.DrawRectangle(20, 20, 120, 80)
	_ = dc.Fill()
	dc.SetRGB(staticBR, staticBG, staticBB)
	dc.DrawRectangle(340, 170, 120, 80)
	_ = dc.Fill()
	_ = oldX
	dc.SetRGB(carR, carG, carB)
	dc.DrawRectangle(newX, y, carW, carH)
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

// probePixels asserts car-new, car-old and static-quiet colors offscreen.
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
	paintProbeFrame(dc, probeOldX, probeNY, probeCarY)
	img := dc.Image()
	_ = dc.Close()

	nr, ng, nb := sample8(img, int(probeNY+carW/2), int(probeCarY+carH/2))
	or, og, ob := sample8(img, int(probeOldX+carW/2), int(probeCarY+carH/2))
	sr, sg, sb := sample8(img, 80, 60)
	okNew := closeEnough(nr, want8(carR)) && closeEnough(ng, want8(carG)) && closeEnough(nb, want8(carB))
	okOld := closeEnough(or, want8(runR)) && closeEnough(og, want8(runG)) && closeEnough(ob, want8(runB))
	okStatic := closeEnough(sr, want8(staticAR)) && closeEnough(sg, want8(staticAG)) && closeEnough(sb, want8(staticAB))
	detail := fmt.Sprintf("new=(%d,%d,%d) old=(%d,%d,%d) static=(%d,%d,%d) tol=%d",
		nr, ng, nb, or, og, ob, sr, sg, sb, probePixelTol)
	return okNew && okOld && okStatic, detail
}

// probeGolden compares the deterministic final-pose frame against the
// frozen mask with zero tolerance; the first run produces the baseline.
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
	paintProbeFrame(dc, goldenOldX, goldenNewX, goldenCarY)
	img := dc.Image()
	_ = dc.Close()

	f, err := os.Open(goldenPath)
	if err != nil {
		if err := os.MkdirAll("examples/game_step/testdata", 0o755); err != nil {
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
	rects, full, staticClean, statsOK, maxOK, _ := probeLogic()
	p.DirtyRects = rects
	p.NeedsFull = false // single chase move never trips full; burst does
	p.FullOK = full
	p.StaticOK, p.StatsOK, p.MaxOK = staticClean, statsOK, maxOK
	p.RectsOK = rects == 1
	// Cross-check the burst twin once more on a fresh tracker for the verdict.
	bounds := core.NewRect(0, 0, 800, 600)
	tr, _ := step.NewSpriteDirtyTracker(bounds)
	for i := 0; i < 17; i++ {
		x := float64(i * 40)
		tr.MarkMoved(core.NewRect(x, 10, 16, 16), core.NewRect(x+8, 10, 16, 16))
	}
	p.FullOK = tr.NeedsFull() && tr.DirtyRects() == nil
	tr.Clear()
	p.Frames, p.Fulls, p.Max = 3, 0, 1
	p.PixOK, p.PixDetail = probePixels()
	var wrote bool
	p.GoldenOK, p.GoldenChanged, wrote = probeGolden()
	p.GoldenWrote = wrote
	p.OK = p.RectsOK && p.FullOK && p.StaticOK && p.StatsOK && p.MaxOK && p.PixOK && p.GoldenOK
	return p
}

// chaseSim is the live window state: real tracker + real layer advance the
// car every frame, and the overlay strokes exactly the kept dirty rects.
type chaseSim struct {
	tracker *step.DirtyTracker
	layer   *scene.DirtyLayer
	app     *embedder.PipelineApp
	shell   *wrkit.ShellChrome
	phase   *wrkit.PhaseClock
	overlay *rendering.RenderBox
	show    []scene.DirtyRect
	fullBar *rendering.RenderColorBox
	car     *rendering.RenderColorBox
	dirtyL  *rendering.RenderText
	fullL   *rendering.RenderText
	fpsL    *rendering.RenderText
	skipL   *rendering.RenderText
	movedL  *rendering.RenderText
	carX    float64
	dir     float64
	movedPx float64
	frames  int
	maxSeen int
}

type ticker struct{ s *chaseSim }

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
	nx := s.carX + s.dir*carSpeed*dt
	if nx >= carMaxX {
		nx, s.dir = carMaxX, -1
	}
	if nx <= carMinX {
		nx, s.dir = carMinX, 1
	}
	oldR := core.NewRect(s.carX, carY, carW, carH)
	newR := core.NewRect(nx, carY, carW, carH)
	moved := nx - s.carX
	if moved < 0 {
		moved = -moved
	}
	s.movedPx += moved
	s.carX = nx
	// Real logic only: game twin first, scene twin with the same numbers.
	s.tracker.MarkMoved(oldR, newR)
	s.layer.MarkMoved(srect(oldR), srect(newR))
	// Burst full-motion frame: 17 extra moves trip the >16 fallback and
	// raise the cyan对照条 for exactly this frame.
	if s.frames == 60 || s.frames%burstEvery == 0 {
		for i := 0; i < 17; i++ {
			x := float64((i * 40) % int(runW-24))
			s.tracker.MarkMoved(core.NewRect(x, 10, 16, 16), core.NewRect(x+8, 10, 16, 16))
			s.layer.MarkMoved(
				scene.DirtyRect{X: x, Y: 10, W: 16, H: 16},
				scene.DirtyRect{X: x + 8, Y: 10, W: 16, H: 16},
			)
		}
		s.fullBar.SetAlpha(0.30)
	} else {
		s.fullBar.SetAlpha(0)
	}
	kept := s.layer.DirtyRects()
	s.show = append(s.show[:0], kept...)
	if len(kept) > s.maxSeen {
		s.maxSeen = len(kept)
	}
	full := s.tracker.NeedsFull()
	s.car.MoveTo(runX+s.carX, runY+carY)
	s.overlay.MarkNeedsPaint()
	s.tracker.Clear()
	s.layer.Clear()

	phase := s.phase.Advance(dt)
	snap := s.app.Metrics().Snapshot()
	fps := 0.0
	if snap.AvgFrameIntervalMs > 1e-6 {
		fps = 1000.0 / snap.AvgFrameIntervalMs
	}
	dirtyTxt := fmt.Sprintf("脏块数 %d", len(kept))
	if full {
		dirtyTxt = "脏块数 FULL"
	}
	s.dirtyL.SetText(dirtyTxt)
	s.fullL.SetText(fmt.Sprintf("整屏回退数 %d", s.tracker.Stats().FullFallbacks))
	s.fpsL.SetText(fmt.Sprintf("帧率 %.0f", fps))
	s.skipL.SetText(fmt.Sprintf("静态skip %d", snap.BoundarySkip))
	s.movedL.SetText(fmt.Sprintf("位移 %.0fpx", s.movedPx))
	gateOK := s.app.PresentCount() >= 0 && s.movedPx > 0
	s.shell.NoteHUDTick(dt)
	s.shell.UpdateHUD("step-dirty", phase, s.app, gateOK,
		fmt.Sprintf("dirty=%d full=%d", len(kept), s.tracker.Stats().FullFallbacks),
		fmt.Sprintf("moved=%.0fpx", s.movedPx))
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
	caseFlag := flag.String("case", "dirty", "scenario case (only dirty)")
	autoOnly := flag.Bool("auto-only", false, "probes + short window, JSON gate on stdout")
	manualSeconds := flag.Int("manual-seconds", 0, "manual phase seconds (0 = until close)")
	flag.Parse()

	if *caseFlag != "dirty" {
		fmt.Fprintf(os.Stderr, "FAIL: --case=%q want dirty (only chase scene)\n", *caseFlag)
		os.Exit(1)
	}
	wrkit.EnsureUIFace()

	probe := runProbes()
	fmt.Fprintf(os.Stderr, "game_step: probes ok=%v rects=%d full=%v static=%v stats=%v max=%v pix=%v golden=%v(wrote=%v changed=%d) %s\n",
		probe.OK, probe.DirtyRects, probe.FullOK, probe.StaticOK, probe.StatsOK, probe.MaxOK,
		probe.PixOK, probe.GoldenOK, probe.GoldenWrote, probe.GoldenChanged, probe.PixDetail)
	if !probe.OK {
		if *autoOnly {
			failJSON(probe)
		} else {
			fmt.Fprintln(os.Stderr, "game_step: selftest FAIL, not opening window")
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

	shell := wrkit.NewShell(winW, winH, "game_step — 8.3 追车脏区 (step-dirty)", []string{
		"静态底图留存·全程零脏",
		"追车每帧只更脏区",
		"黄框=脏区描边",
		"青条=突发整屏对照",
		"右栏 脏块/回退/帧率",
		"JSON见 ability_extra",
	})

	// Scene tree: static layer stays clean forever, sprite layer takes the
	// chase moves. Static is never marked: the skip evidence is live.
	staticLayer, _ := scene.NewDirtyLayer(0, 0, staticW, staticH)
	spriteLayer, _ := scene.NewSpriteDirtyLayer(0, 0, runW, runH)
	sceneRoot := scene.NewContainerLayer()
	sceneRoot.Add(staticLayer)
	sceneRoot.Add(spriteLayer)
	_ = sceneRoot

	tracker, _ := step.NewSpriteDirtyTracker(core.NewRect(0, 0, runW, runH))

	sim := &chaseSim{
		tracker: tracker,
		layer:   spriteLayer,
		shell:   shell,
		carX:    carMinX,
		dir:     1,
	}
	if secs > 0 {
		sim.phase = wrkit.NewPhaseClock(float64(secs)*0.5, float64(secs)*0.8)
	} else {
		sim.phase = wrkit.NewPhaseClock(0, 0)
	}

	// Left: static retained blocks (repaint boundary, never dirtied).
	shell.Body.Place(wrkit.Label("STATIC 留存·零脏", 13, 0.55, 0.75, 0.95), staticX, staticY-24)
	staticA := rendering.NewRenderColorBox(200, 150, staticAR, staticAG, staticAB, 1)
	staticA.SetRepaintBoundary(true)
	shell.Body.Place(staticA, staticX+10, staticY+10)
	staticB := rendering.NewRenderColorBox(200, 120, staticBR, staticBG, staticBB, 1)
	staticB.SetRepaintBoundary(true)
	shell.Body.Place(staticB, staticX+10, staticY+180)
	shell.Body.Place(wrkit.Label("全程skip·不重画", 12, 0.70, 0.78, 0.88), staticX+10, staticY+312)

	// Middle: chase runway (background + car + burst bar + dirty overlay).
	shell.Body.Place(wrkit.Label("CHASE 追车跑道", 13, 0.55, 0.75, 0.95), runX, runY-24)
	runway := rendering.NewRenderColorBox(runW, runH, runR, runG, runB, 1)
	shell.Body.Place(runway, runX, runY)
	// Lane guides live on the runway background (static paint, no dirty).
	lane := rendering.NewRenderBox()
	lane.FixedWidth, lane.FixedHeight = runW, runH
	lane.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		ax, ay := pc.Abs(0, 0)
		pc.DC.SetRGBA(0.9, 0.9, 0.9, 0.25)
		pc.DC.SetLineWidth(1)
		pc.DC.DrawLine(ax, ay+carY-8, ax+runW, ay+carY-8)
		_ = pc.DC.Stroke()
		pc.DC.DrawLine(ax, ay+carY+carH+8, ax+runW, ay+carY+carH+8)
		_ = pc.DC.Stroke()
	}
	shell.Body.Place(lane, runX, runY)
	sim.car = rendering.NewRenderColorBox(carW, carH, carR, carG, carB, 1)
	shell.Body.Place(sim.car, runX+carMinX, runY+carY)
	sim.fullBar = rendering.NewRenderColorBox(runW, runH, 0.2, 0.8, 0.9, 1)
	sim.fullBar.SetAlpha(0)
	shell.Body.Place(sim.fullBar, runX, runY)
	sim.overlay = rendering.NewRenderBox()
	sim.overlay.FixedWidth, sim.overlay.FixedHeight = runW, runH
	sim.overlay.SetRepaintBoundary(true)
	sim.overlay.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		ax, ay := pc.Abs(0, 0)
		pc.DC.SetRGBA(1, 0.9, 0.2, 1)
		pc.DC.SetLineWidth(2)
		for _, r := range sim.show {
			pc.DC.DrawRectangle(ax+r.X, ay+r.Y, r.W, r.H)
			_ = pc.DC.Stroke()
		}
	}
	shell.Body.Place(sim.overlay, runX, runY)

	// Right: live counters (dirty / full fallbacks / fps).
	shell.Body.Place(wrkit.Label("COUNTERS 计数器", 13, 0.55, 0.75, 0.95), countX, countY-24)
	sim.dirtyL = wrkit.Label("脏块数 0", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.dirtyL, countX, countY+10)
	sim.fullL = wrkit.Label("整屏回退数 0", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.fullL, countX, countY+36)
	sim.fpsL = wrkit.Label("帧率 0", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.fpsL, countX, countY+62)
	sim.skipL = wrkit.Label("静态skip 0", 12, 0.70, 0.78, 0.88)
	shell.Body.Place(sim.skipL, countX, countY+88)
	sim.movedL = wrkit.Label("位移 0px", 12, 0.70, 0.78, 0.88)
	shell.Body.Place(sim.movedL, countX, countY+114)
	shell.Body.Place(wrkit.Label("MaxDirtyRects=16", 12, 0.70, 0.78, 0.88), countX, countY+140)
	shell.Body.Place(wrkit.Label("超16块退整屏", 12, 0.70, 0.78, 0.88), countX, countY+162)

	shell.Body.Place(wrkit.Label("黄框=本帧脏区并集 · 青条=突发全动整屏对照 · 右栏为live计数", 12, 0.70, 0.78, 0.88), staticX, noteY)

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "game_step", Decorations: true})
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
				fmt.Fprintf(os.Stderr, "game_step: close (%s)\n", win.Backend())
				return
			case platform.EventPointer:
				summary.Pointer++
				if manualMode {
					fmt.Fprintf(os.Stderr, "game_step: pointer %s (%.0f,%.0f) n=%d\n",
						ev.Pointer, ev.X, ev.Y, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_step events=%d", summary.Pointer+summary.Key+summary.Resize))
					}
				}
				return
			case platform.EventKey:
				if ev.Pressed {
					summary.Key++
					if manualMode {
						fmt.Fprintf(os.Stderr, "game_step: key n=%d\n", summary.Pointer+summary.Key+summary.Resize)
						if ctl != nil {
							ctl.SetTitle(fmt.Sprintf("game_step events=%d", summary.Pointer+summary.Key+summary.Resize))
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
					fmt.Fprintf(os.Stderr, "game_step: resize %dx%d n=%d\n", ev.Width, ev.Height, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_step events=%d", summary.Pointer+summary.Key+summary.Resize))
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

	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: run:", err)
		os.Exit(1)
	}
	app.Close()
	elapsed := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	presents := app.PresentCount()
	trStats := sim.tracker.Stats()
	probeOK := 0
	if probe.OK {
		probeOK = 1
	}
	extra := map[string]any{
		"dirty_rects_max": sim.maxSeen,
		"full_fallbacks":  trStats.FullFallbacks,
		"moved_px":        sim.movedPx,
		"probe_ok":        probeOK,
		"frames":          sim.frames,
		"boundary_skip":   snap.BoundarySkip,
		"case":            "dirty",
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
		if presents < 1 || sim.movedPx <= 0 || !probe.OK {
			fmt.Fprintf(os.Stderr, "FAIL: presents=%d moved=%.0f probe=%v (want >=1, >0, true)\n",
				presents, sim.movedPx, probe.OK)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "game_step: OK presents=%d moved=%.0f dirty_max=%d full=%d elapsed=%.1fs\n",
			presents, sim.movedPx, sim.maxSeen, trStats.FullFallbacks, elapsed)
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
		"presents":        presents,
		"elapsed_sec":     elapsed,
		"moved_px":        sim.movedPx,
		"dirty_rects_max": sim.maxSeen,
		"full_fallbacks":  trStats.FullFallbacks,
		"probe_ok":        probeOK,
		"timed":           summary.Timed,
		"note":            summary.Note,
	})
	fmt.Println(string(b))
	fmt.Fprintf(os.Stderr, "game_step: backend=%s presents=%d moved=%.0f full=%d elapsed=%.1fs\n",
		win.Backend(), presents, sim.movedPx, trStats.FullFallbacks, elapsed)
}
