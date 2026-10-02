// Command game_25d_basis is the S79 true-2.5D basis window.
//
// Six view modes press 3D points flat through the real engine/camera
// Basis25D (read-only from here); YSort25D orders the blocks back to
// front; ShadowMath25D lands one shadow per block on the ground line.
// Keys 1-6 switch the view; WASD nudges the hero block.
//
// Modes:
//
//	go run ./examples/game_25d_basis -auto-only
//	  headless probes + ~8s window (JSON on stdout, exit 1 on fail).
//	go run ./examples/game_25d_basis -manual-seconds 30
//	  manual for 30s (events logged, title shows the count), then summary.
//	go run ./examples/game_25d_basis
//	  probes, then resident until close (RUN_SECONDS sets a timed run).
//
// Window: 1200x800, title game_25d_basis. First run writes the golden
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

	"github.com/energye/gpui/engine/camera"
	"github.com/energye/gpui/engine/core"
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
	abilityID  = "basis-25d"
	scenario   = "game_25d_basis"
	goldenPath = "examples/game_25d_basis/testdata/basis_view_golden.png"

	// epsEngine matches the camera package bar: matrices bit-close,
	// projections within 1e-9.
	epsEngine = 1e-9
	// probePixelTol is the per-channel tolerance (0-255 steps) for the
	// offscreen pixel asserts. The golden mask compare stays at zero.
	probePixelTol = 4
)

// Scene colors (window paint and offscreen probes use the same).
const (
	skyR, skyG, skyB       = 0.10, 0.12, 0.18
	blockR, blockG, blockB = 0.25, 0.55, 0.35
	heroR, heroG, heroB    = 0.90, 0.15, 0.12
	shadR, shadG, shadB    = 0.05, 0.05, 0.08
)

// Body-local layout (body is ~904x656 under the shell chrome).
const (
	stageX, stageY, stageW, stageH = 16.0, 44.0, 600.0, 360.0
	infoX, infoY                   = 632.0, 44.0
	noteY                          = 424.0

	offW, offH = 480, 270
)

// viewNames mirrors camera.ViewMode order for labels and keys 1-6.
var viewNames = []string{"45deg", "iso", "top", "front", "oblY", "oblZ"}

// blocks are the demo scene: fixed 3D feet positions plus sizes.
var blocks = []core.Vec3{
	{X: 0, Y: 0, Z: 0},
	{X: 2, Y: 0, Z: 1},
	{X: 1, Y: 1, Z: 2},
	{X: 3, Y: 0, Z: 0},
}

func basisFor(mode int) (camera.Basis25D, error) {
	return camera.NewBasis25D(camera.ViewMode(mode))
}

// probeResult is the three-evidence headless verdict (no GPU needed).
type probeResult struct {
	BasisOK, ProjOK, SortOK, ShadOK bool
	Views                           int
	SortN                           int
	PixOK                           bool
	PixDetail                       string
	GoldenOK                        bool
	GoldenChanged                   int
	GoldenWrote                     bool
	OK                              bool
	Detail                          string
}

// probeLogic replays the frozen engine cases: six axes bit-close, six
// projections within 1e-9, sort order stable, shadow lands on ground.
func probeLogic() (basisOK, projOK, sortOK, shadOK bool, views, sortN int, detail string) {
	raw, err := os.ReadFile("engine/camera/testdata/basis25d_cases.json")
	if err != nil {
		return false, false, false, false, 0, 0, "frozen read: " + err.Error()
	}
	var f struct {
		Basis []struct {
			Mode int        `json:"mode"`
			X    [2]float64 `json:"x"`
			Y    [2]float64 `json:"y"`
			Z    [2]float64 `json:"z"`
		} `json:"basis"`
		Project []struct {
			Mode int        `json:"mode"`
			In   [3]float64 `json:"in"`
			Want [2]float64 `json:"want"`
		} `json:"project"`
		Shadow struct {
			Ground float64    `json:"ground"`
			In     [3]float64 `json:"in"`
			Want   [2]float64 `json:"want"`
		} `json:"shadow"`
		YSort struct {
			Order [][]float64 `json:"order"`
		} `json:"ysort"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		return false, false, false, false, 0, 0, "frozen decode: " + err.Error()
	}
	basisOK = len(f.Basis) == 6
	for _, c := range f.Basis {
		b, err := basisFor(c.Mode)
		if err != nil {
			return false, false, false, false, 0, 0, fmt.Sprintf("mode %d: %v", c.Mode, err)
		}
		if math.Abs(b.X.X-c.X[0]) > epsEngine || math.Abs(b.X.Y-c.X[1]) > epsEngine ||
			math.Abs(b.Y.X-c.Y[0]) > epsEngine || math.Abs(b.Y.Y-c.Y[1]) > epsEngine ||
			math.Abs(b.Z.X-c.Z[0]) > epsEngine || math.Abs(b.Z.Y-c.Z[1]) > epsEngine {
			return false, false, false, false, 0, 0, fmt.Sprintf("mode %d axes off", c.Mode)
		}
	}
	projOK = true
	for _, c := range f.Project {
		b, err := basisFor(c.Mode)
		if err != nil {
			return false, false, false, false, 0, 0, fmt.Sprintf("mode %d: %v", c.Mode, err)
		}
		got, ok := b.Project(core.V3(c.In[0], c.In[1], c.In[2]))
		if !ok || math.Abs(got.X-c.Want[0]) > epsEngine || math.Abs(got.Y-c.Want[1]) > epsEngine {
			return true, false, false, false, 6, 0, fmt.Sprintf("mode %d proj off", c.Mode)
		}
	}
	items := make([]camera.YSortItem, len(f.YSort.Order))
	for i, o := range f.YSort.Order {
		items[i] = camera.YSortItem{Pos: core.V3(o[0], o[1], o[2]), Order: i}
	}
	sorted, err := camera.YSort(items)
	if err != nil {
		return true, true, false, false, 6, 0, "sort: " + err.Error()
	}
	sortOK = true
	for i := 1; i < len(sorted); i++ {
		if camera.YSortLess(sorted[i], sorted[i-1]) {
			return true, true, false, false, 6, 0, "sort unstable"
		}
	}
	b0, _ := basisFor(0)
	got, ok := camera.LandShadow(b0, core.V3(f.Shadow.In[0], f.Shadow.In[1], f.Shadow.In[2]), f.Shadow.Ground)
	if !ok || math.Abs(got.X-f.Shadow.Want[0]) > epsEngine || math.Abs(got.Y-f.Shadow.Want[1]) > epsEngine {
		return true, true, true, false, 6, len(sorted), "shadow off"
	}
	if !camera.ShadowVisible(core.V3(0, 1, 0), 0, true) {
		return true, true, true, false, 6, len(sorted), "shadow must show"
	}
	if _, ok := camera.LandShadow(b0, core.V3(0, -1, 0), 0); ok {
		return true, true, true, false, 6, len(sorted), "below ground must hide"
	}
	return true, true, true, true, 6, len(sorted), fmt.Sprintf("views=6 sort=%d", len(sorted))
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

// paintProbeFrame draws the deterministic probe scene through the real
// basis: sky plus two pressed blocks and the hero dot.
func paintProbeFrame(dc *render.Context) {
	dc.ClearWithColor(render.RGBA{R: skyR, G: skyG, B: skyB, A: 1})
	b, _ := basisFor(0)
	for i, p := range blocks[:2] {
		flat, ok := b.Project(p)
		if !ok {
			continue
		}
		x := 40 + flat.X*2
		y := 60 + flat.Y*2
		if i == 0 {
			dc.SetRGB(blockR, blockG, blockB)
		} else {
			dc.SetRGB(blockR*0.8, blockG*0.8, blockB*0.9)
		}
		dc.DrawRectangle(x, y, 120, 60)
		_ = dc.Fill()
	}
	flat, ok := b.Project(blocks[2])
	if ok {
		dc.SetRGB(heroR, heroG, heroB)
		dc.DrawRectangle(40+flat.X*2, 60+flat.Y*2, 24, 24)
		_ = dc.Fill()
	}
}

func withCPUMode(fn func()) {
	prev, _ := os.LookupEnv("GOGPU_RENDER_MODE")
	_ = os.Setenv("GOGPU_RENDER_MODE", "cpu")
	defer func() {
		if prev == "" {
			_ = os.Unsetenv("GOGPU_RENDER_MODE")
		} else {
			_ = os.Setenv("GOGPU_RENDER_MODE", prev)
		}
	}()
	fn()
}

// probePixels asserts sky/block/hero colors offscreen.
func probePixels() (bool, string) {
	var img image.Image
	withCPUMode(func() {
		dc := render.NewContext(offW, offH)
		paintProbeFrame(dc)
		img = dc.Image()
		_ = dc.Close()
	})
	sr, sg, sb := sample8(img, 10, 10)
	br, bg, bb := sample8(img, 100, 80)
	hr, hg, hb := sample8(img, 110, 112)
	ok := closeEnough(sr, want8(skyR)) && closeEnough(sg, want8(skyG)) && closeEnough(sb, want8(skyB)) &&
		closeEnough(br, want8(blockR)) && closeEnough(bg, want8(blockG)) && closeEnough(bb, want8(blockB)) &&
		closeEnough(hr, want8(heroR)) && closeEnough(hg, want8(heroG)) && closeEnough(hb, want8(heroB))
	detail := fmt.Sprintf("sky=(%d,%d,%d) block=(%d,%d,%d) hero=(%d,%d,%d) tol=%d",
		sr, sg, sb, br, bg, bb, hr, hg, hb, probePixelTol)
	return ok, detail
}

// probeGolden compares the deterministic frame against the frozen mask
// with zero tolerance; the first run produces the baseline.
func probeGolden() (ok bool, changed int, wrote bool) {
	var img image.Image
	withCPUMode(func() {
		dc := render.NewContext(offW, offH)
		paintProbeFrame(dc)
		img = dc.Image()
		_ = dc.Close()
	})
	f, err := os.Open(goldenPath)
	if err != nil {
		if err := os.MkdirAll("examples/game_25d_basis/testdata", 0o755); err != nil {
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
	p.BasisOK, p.ProjOK, p.SortOK, p.ShadOK, p.Views, p.SortN, p.Detail = probeLogic()
	p.PixOK, p.PixDetail = probePixels()
	var wrote bool
	p.GoldenOK, p.GoldenChanged, wrote = probeGolden()
	p.GoldenWrote = wrote
	p.OK = p.BasisOK && p.ProjOK && p.SortOK && p.ShadOK && p.PixOK && p.GoldenOK
	return p
}

// basisSim is the live window state: view mode plus hero nudge.
type basisSim struct {
	mode       int
	hero       core.Vec3
	app        *embedder.PipelineApp
	shell      *wrkit.ShellChrome
	phase      *wrkit.PhaseClock
	stage      *rendering.RenderBox
	modeL      *rendering.RenderText
	orderL     *rendering.RenderText
	shadL      *rendering.RenderText
	fpsL       *rendering.RenderText
	frames     int
	modes      int
	heroTravel float64
	lastFlat   core.Vec2
}

type ticker struct{ s *basisSim }

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
	s.frames++
	b, err := basisFor(s.mode)
	if err == nil {
		if flat, ok := b.Project(s.hero); ok {
			s.heroTravel += math.Hypot(flat.X-s.lastFlat.X, flat.Y-s.lastFlat.Y)
			s.lastFlat = flat
		}
		items := make([]camera.YSortItem, len(blocks))
		for i, p := range blocks {
			items[i] = camera.YSortItem{Pos: p, Order: i}
		}
		if sorted, err := camera.YSort(items); err == nil {
			txt := "盖 "
			for i, it := range sorted {
				if i > 0 {
					txt += ">"
				}
				txt += fmt.Sprintf("%d", it.Order)
			}
			s.orderL.SetText(txt)
		}
		if sh, ok := camera.LandShadow(b, s.hero, 0); ok {
			s.shadL.SetText(fmt.Sprintf("影 %.0f,%.0f", sh.X, sh.Y))
		} else {
			s.shadL.SetText("影 藏")
		}
		s.modeL.SetText(fmt.Sprintf("视角 %s(%d/6) 英雄 %.0f,%.0f,%.0f", viewNames[s.mode], s.mode+1, s.hero.X, s.hero.Y, s.hero.Z))
	}
	s.stage.MarkNeedsPaint()
	snap := s.app.Metrics().Snapshot()
	fps := 0.0
	if snap.AvgFrameIntervalMs > 1e-6 {
		fps = 1000.0 / snap.AvgFrameIntervalMs
	}
	s.fpsL.SetText(fmt.Sprintf("帧率 %.0f", fps))
	phase := s.phase.Advance(dt)
	gateOK := s.frames > 0
	s.shell.NoteHUDTick(dt)
	s.shell.UpdateHUD("basis-25d", phase, s.app, gateOK,
		fmt.Sprintf("view=%s", viewNames[s.mode]),
		fmt.Sprintf("frames=%d modes=%d", s.frames, s.modes))
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
	flag.String("case", "basis", "scenario case (only basis)")
	autoOnly := flag.Bool("auto-only", false, "probes + short window, JSON gate on stdout")
	manualSeconds := flag.Int("manual-seconds", 0, "manual phase seconds (0 = until close)")
	flag.Parse()

	if flag.Lookup("case").Value.String() != "basis" {
		fmt.Fprintf(os.Stderr, "FAIL: --case=%q want basis (only basis road)\n", flag.Lookup("case").Value.String())
		os.Exit(1)
	}
	wrkit.EnsureUIFace()

	probe := runProbes()
	fmt.Fprintf(os.Stderr, "game_25d_basis: probes ok=%v %s pix=%v golden=%v(wrote=%v changed=%d) %s\n",
		probe.OK, probe.Detail, probe.PixOK, probe.GoldenOK,
		probe.GoldenWrote, probe.GoldenChanged, probe.PixDetail)
	if !probe.OK {
		if *autoOnly {
			failJSON(probe)
		} else {
			fmt.Fprintln(os.Stderr, "game_25d_basis: selftest FAIL, not opening window")
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

	shell := wrkit.NewShell(winW, winH, "game_25d_basis — S79 真2.5D六视角 (basis-25d)", []string{
		"六视角只换基向量·SCALE=32",
		"Y排序低y在下·影子落线",
		"1-6切视角·WASD挪英雄",
		"前后盖对·影子不飘",
		"JSON见 ability_extra",
	})

	sim := &basisSim{
		mode:  0,
		hero:  core.V3(1, 1, 2),
		shell: shell,
	}
	if b, err := basisFor(0); err == nil {
		if flat, ok := b.Project(sim.hero); ok {
			sim.lastFlat = flat
		}
	}
	if secs > 0 {
		sim.phase = wrkit.NewPhaseClock(float64(secs)*0.5, float64(secs)*0.8)
	} else {
		sim.phase = wrkit.NewPhaseClock(0, 0)
	}

	shell.Body.Place(wrkit.Label("STAGE 六视角", 13, 0.55, 0.75, 0.95), stageX, stageY-24)
	sim.stage = rendering.NewRenderBox()
	sim.stage.FixedWidth, sim.stage.FixedHeight = stageW, stageH
	sim.stage.SetRepaintBoundary(true)
	sim.stage.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		ax, ay := pc.Abs(0, 0)
		pc.DC.SetRGBA(skyR, skyG, skyB, 1)
		pc.DC.DrawRectangle(ax, ay, stageW, stageH)
		_ = pc.DC.Fill()
		b, err := basisFor(sim.mode)
		if err != nil {
			return
		}
		// Ground line: the shadow rail all shadows sit on.
		pc.DC.SetRGB(0.4, 0.45, 0.55)
		pc.DC.DrawRectangle(ax+20, ay+stageH-60, stageW-40, 3)
		_ = pc.DC.Fill()
		items := make([]camera.YSortItem, len(blocks))
		for i, p := range blocks {
			items[i] = camera.YSortItem{Pos: p, Order: i}
		}
		sorted, err := camera.YSort(items)
		if err != nil {
			return
		}
		// Back-to-front: lower y first, hero uses the hero color.
		for _, it := range sorted {
			flat, ok := b.Project(it.Pos)
			if !ok {
				continue
			}
			x := ax + stageW/2 + flat.X*2
			y := ay + stageH/2 + flat.Y*2
			isHero := it.Pos == sim.hero
			if isHero {
				pc.DC.SetRGB(heroR, heroG, heroB)
			} else {
				pc.DC.SetRGB(blockR, blockG, blockB)
			}
			pc.DC.DrawRectangle(x-30, y-20, 60, 40)
			_ = pc.DC.Fill()
			// Shadow: one dark plate on the ground line per block.
			if sh, ok := camera.LandShadow(b, it.Pos, 0); ok {
				sx := ax + stageW/2 + sh.X*2
				pc.DC.SetRGB(shadR, shadG, shadB)
				pc.DC.DrawRectangle(sx-24, ay+stageH-64, 48, 8)
				_ = pc.DC.Fill()
			}
		}
	}
	shell.Body.Place(sim.stage, stageX, stageY)

	shell.Body.Place(wrkit.Label("READOUT 读数", 13, 0.55, 0.75, 0.95), infoX, infoY-24)
	sim.modeL = wrkit.Label("视角 45deg", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.modeL, infoX, infoY+10)
	sim.orderL = wrkit.Label("盖 --", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.orderL, infoX, infoY+36)
	sim.shadL = wrkit.Label("影 --", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.shadL, infoX, infoY+62)
	sim.fpsL = wrkit.Label("帧率 0", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.fpsL, infoX, infoY+88)
	shell.Body.Place(wrkit.Label("1-6切视角·WASD挪英雄", 12, 0.70, 0.78, 0.88), infoX, infoY+114)
	shell.Body.Place(wrkit.Label("红块=英雄 · 深块=影子 · 地面线=影轨", 12, 0.70, 0.78, 0.88), infoX, infoY+136)

	shell.Body.Place(wrkit.Label("六视角盖对影子落线，1-6随时切不闪", 12, 0.70, 0.78, 0.88), stageX, noteY)

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "game_25d_basis", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	defer win.Close()
	ctl := win.Controls()

	var summary manualSummary
	app := embedder.NewPipelineApp(win.Host(), shell.Root, embedder.PipelineOptions{
		ClearR: skyR, ClearG: skyG, ClearB: skyB, ClearA: 1,
		RunFor: runFor,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose, platform.EventCloseRequested:
				fmt.Fprintf(os.Stderr, "game_25d_basis: close (%s)\n", win.Backend())
				return
			case platform.EventPointer:
				summary.Pointer++
				if manualMode {
					fmt.Fprintf(os.Stderr, "game_25d_basis: pointer %s (%.0f,%.0f) n=%d\n",
						ev.Pointer, ev.X, ev.Y, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_25d_basis events=%d", summary.Pointer+summary.Key+summary.Resize))
					}
				}
				return
			case platform.EventKey:
				if ev.Pressed {
					summary.Key++
					switch ev.Rune {
					case '1', '2', '3', '4', '5', '6':
						sim.mode = int(ev.Rune - '1')
						sim.modes++
					case 'a', 'A':
						sim.hero.X--
					case 'd', 'D':
						sim.hero.X++
					case 'w', 'W':
						sim.hero.Y++
					case 's', 'S':
						sim.hero.Y--
					}
					if manualMode {
						fmt.Fprintf(os.Stderr, "game_25d_basis: key %c n=%d\n", ev.Rune, summary.Pointer+summary.Key+summary.Resize)
						if ctl != nil {
							ctl.SetTitle(fmt.Sprintf("game_25d_basis view=%s events=%d", viewNames[sim.mode], summary.Pointer+summary.Key+summary.Resize))
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
					fmt.Fprintf(os.Stderr, "game_25d_basis: resize %dx%d n=%d\n", ev.Width, ev.Height, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_25d_basis events=%d", summary.Pointer+summary.Key+summary.Resize))
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
	probeOK := 0
	if probe.OK {
		probeOK = 1
	}
	extra := map[string]any{
		"case":           "basis",
		"probe_ok":       probeOK,
		"basis_ok":       probe.BasisOK,
		"proj_ok":        probe.ProjOK,
		"sort_ok":        probe.SortOK,
		"shadow_ok":      probe.ShadOK,
		"views":          probe.Views,
		"sort_n":         probe.SortN,
		"modes":          sim.modes,
		"frames":         sim.frames,
		"hero_travel_px": sim.heroTravel,
		"boundary_skip":  snap.BoundarySkip,
		"pixels":         probe.PixDetail,
		"golden":         probe.GoldenChanged,
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
		if presents < 1 || !probe.OK || sim.frames < 1 {
			fmt.Fprintf(os.Stderr, "FAIL: presents=%d probe=%v frames=%d (want >=1, true, >=1)\n",
				presents, probe.OK, sim.frames)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "game_25d_basis: OK presents=%d views=%d sort=%d modes=%d travel=%.0f elapsed=%.1fs\n",
			presents, probe.Views, probe.SortN, sim.modes, sim.heroTravel, elapsed)
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
		"views":       probe.Views,
		"modes":       sim.modes,
		"probe_ok":    probeOK,
		"timed":       summary.Timed,
		"note":        summary.Note,
	})
	fmt.Println(string(b))
	fmt.Fprintf(os.Stderr, "game_25d_basis: backend=%s presents=%d views=%d modes=%d elapsed=%.1fs\n",
		win.Backend(), presents, probe.Views, sim.modes, elapsed)
}

var _ = scheduler.FrameMetrics{}
