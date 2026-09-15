// Command game_save is the 16.2 quality-tier independent window.
//
// Three cards side by side, one per frozen tier (high 1000 particles plus
// 8 lights plus scale 1.00, medium 500 plus 4 plus 0.75, low 200 plus 2
// plus 0.50). The live scene highlights one card at a time and switches
// the real game/save Quality handle every 2s; the switch only replaces
// the level, the scene never reloads, nothing flashes.
//
// Modes:
//
//	go run ./examples/game_save --case=q123 -auto-only
//	  headless probes + ~8s window (JSON on stdout, exit 1 on fail).
//	go run ./examples/game_save --case=q123 -manual-seconds 30
//	  manual for 30s (events logged, title shows the count), then summary.
//	go run ./examples/game_save --case=q123
//	  probes, then resident until close (RUN_SECONDS sets a timed run).
//
// Window: 1200x800, title game_save. First run writes the golden baseline
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
	"github.com/energye/gpui/game/save"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH = 1200, 800
	abilityID  = "save-quality"
	scenario   = "game_save--case=q123"
	goldenPath = "examples/game_save/testdata/save_q123_golden.png"

	// frozenPath is the engine-side frozen tier table. The window never
	// hardcodes tier numbers; it replays this file through the real API.
	frozenPath = "game/save/testdata/quality_cases.json"

	// switchEveryS flips the live tier this often; an 8s auto run flips
	// at least 3 times (one full high->medium->low cycle plus one).
	switchEveryS = 2.0

	// dotTol is the hardcoded per-channel tolerance (0-255 steps) for
	// classifying a dot pixel offscreen. The golden mask compare stays
	// at zero tolerance.
	dotTol = 8
)

// Shared scene colors (window paint and offscreen probes use the same).
const (
	bgR, bgG, bgB = 0.08, 0.09, 0.11

	highR, highG, highB = 0.20, 0.22, 0.28
	medR, medG, medB    = 0.16, 0.17, 0.22
	lowR, lowG, lowB    = 0.12, 0.13, 0.17

	dotR, dotG, dotB = 1.0, 0.80, 0.40
	barR, barG, barB = 0.35, 0.55, 0.75
	barBgR, barBgG   = 0.25, 0.27
	barBgB           = 0.32
)

// Body-local layout (body is ~904x656 under the shell chrome).
const (
	cardW, cardH   = 280.0, 420.0
	cardY          = 44.0
	cardX0, cardX1 = 16.0, 312.0
	cardX2         = 608.0
	barH           = 14.0
	noteY          = 480.0

	offW, offH = 480, 270
	offCardW   = 140
	offCardH   = 200
	offCardY   = 30
	offCardX0  = 10
	offCardX1  = 170
	offCardX2  = 330
	offBarH    = 8
	offDot     = 2
	liveDot    = 3
)

var tierOrder = []save.Level{save.LevelHigh, save.LevelMedium, save.LevelLow}

func tierColor(l save.Level) (float64, float64, float64) {
	switch l {
	case save.LevelHigh:
		return highR, highG, highB
	case save.LevelMedium:
		return medR, medG, medB
	default:
		return lowR, lowG, lowB
	}
}

// dotPos is the deterministic dot layout shared by the window and the
// offscreen probes: dot i of a card lands on the same relative spot.
func dotPos(i int, w, h float64) (float64, float64) {
	x := float64((i*73+11)%int(w-4)) + 2
	y := float64((i*149+37)%int(h-24)) + 2
	return x, y
}

// paintCard fills one tier card: background, spec.Particles dots, and
// the resolution scale bar (width = scale * card width).
func paintCard(dc *render.Context, ox, oy, w, h float64, spec save.Spec, r, g, b float64, dot int) {
	dc.SetRGBA(r, g, b, 1)
	dc.DrawRectangle(ox, oy, w, h)
	_ = dc.Fill()
	dc.SetRGBA(dotR, dotG, dotB, 1)
	for i := 0; i < spec.Particles; i++ {
		dx, dy := dotPos(i, w, h)
		dc.DrawRectangle(ox+dx, oy+dy, float64(dot), float64(dot))
		_ = dc.Fill()
	}
	dc.SetRGBA(barBgR, barBgG, barBgB, 1)
	dc.DrawRectangle(ox+8, oy+h-8-barH, w-16, barH)
	_ = dc.Fill()
	dc.SetRGBA(barR, barG, barB, 1)
	dc.DrawRectangle(ox+8, oy+h-8-barH, (w-16)*spec.Scale, barH)
	_ = dc.Fill()
}

// paintQ123Frame draws the deterministic probe frame offscreen.
func paintQ123Frame(dc *render.Context) {
	dc.ClearWithColor(render.RGBA{R: bgR, G: bgG, B: bgB, A: 1})
	xs := []int{offCardX0, offCardX1, offCardX2}
	for i, lvl := range tierOrder {
		spec, err := save.SpecFor(lvl)
		if err != nil {
			continue
		}
		r, g, b := tierColor(lvl)
		ox, oy := float64(xs[i]), float64(offCardY)
		dc.SetRGBA(r, g, b, 1)
		dc.DrawRectangle(ox, oy, float64(offCardW), float64(offCardH))
		_ = dc.Fill()
		dc.SetRGBA(dotR, dotG, dotB, 1)
		for d := 0; d < spec.Particles; d++ {
			dx, dy := dotPos(d, float64(offCardW), float64(offCardH))
			dc.DrawRectangle(ox+dx, oy+dy, offDot, offDot)
			_ = dc.Fill()
		}
		dc.SetRGBA(barBgR, barBgG, barBgB, 1)
		dc.DrawRectangle(ox+6, oy+float64(offCardH)-6-offBarH, float64(offCardW)-12, offBarH)
		_ = dc.Fill()
		dc.SetRGBA(barR, barG, barB, 1)
		dc.DrawRectangle(ox+6, oy+float64(offCardH)-6-offBarH, (float64(offCardW)-12)*spec.Scale, offBarH)
		_ = dc.Fill()
	}
}

func sample8(img image.Image, x, y int) (uint8, uint8, uint8) {
	r, g, b, _ := img.At(x, y).RGBA()
	return uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)
}

func closeEnough(got, want uint8, tol int) bool {
	d := int(got) - int(want)
	if d < 0 {
		d = -d
	}
	return d <= tol
}

func want8(v float64) uint8 { return uint8(v*255 + 0.5) }

type frozenLevel struct {
	Name      string  `json:"name"`
	Particles int     `json:"particles"`
	Lights    int     `json:"lights"`
	Scale     float64 `json:"scale"`
}

type frozenFile struct {
	Levels []frozenLevel `json:"levels"`
}

// probeResult is the three-evidence headless verdict (no GPU needed).
type probeResult struct {
	LogicOK, PixOK, GoldenOK bool
	Switches                 int
	Detail                   string
	PixDetail                string
	GoldenChanged            int
	GoldenWrote              bool
	OK                       bool
}

// probeLogic drives the real Quality API against the frozen file: tier
// mapping, rapid flicker continuity, bad-name rejection, encode stability.
func probeLogic() (bool, int, string) {
	raw, err := os.ReadFile(frozenPath)
	if err != nil {
		return false, 0, "frozen read: " + err.Error()
	}
	var f frozenFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return false, 0, "frozen decode: " + err.Error()
	}
	if len(f.Levels) != 3 {
		return false, 0, fmt.Sprintf("frozen levels = %d, want 3", len(f.Levels))
	}
	for _, want := range f.Levels {
		lvl, err := save.ParseLevel(want.Name)
		if err != nil {
			return false, 0, "ParseLevel " + want.Name + ": " + err.Error()
		}
		spec, err := save.SpecFor(lvl)
		if err != nil {
			return false, 0, "SpecFor " + want.Name + ": " + err.Error()
		}
		if spec.Particles != want.Particles || spec.Lights != want.Lights || spec.Scale != want.Scale {
			return false, 0, fmt.Sprintf("%s spec = %+v, frozen %+v", want.Name, spec, want)
		}
	}
	// Rapid flicker: 90 switches land back on the start, same scene.
	start, err := save.NewQuality(save.LevelHigh)
	if err != nil {
		return false, 0, "NewQuality: " + err.Error()
	}
	flick := start
	seq := []save.Level{save.LevelMedium, save.LevelLow, save.LevelHigh}
	switches := 0
	for i := 0; i < 30; i++ {
		for _, to := range seq {
			if err := flick.Switch(to); err != nil {
				return false, switches, "flicker: " + err.Error()
			}
			switches++
		}
	}
	if !flick.Equal(start) {
		return false, switches, "flicker diverged"
	}
	// Bad tiers never sneak in.
	for _, bad := range []string{"", "ultra", "HIGH"} {
		if _, err := save.ParseLevel(bad); err == nil {
			return false, switches, "bad name " + bad + " accepted"
		}
	}
	if err := flick.Switch(save.Level("ultra")); err == nil {
		return false, switches, "bad Switch accepted"
	}
	// Encode stability: 100 rounds, same bytes.
	first, err := start.Encode()
	if err != nil {
		return false, switches, "Encode: " + err.Error()
	}
	for i := 0; i < 100; i++ {
		raw, err := start.Encode()
		if err != nil || string(raw) != string(first) {
			return false, switches, "encode drift"
		}
	}
	detail := fmt.Sprintf("levels=3 flicker=%d stable=100 bytes=%d", switches, len(first))
	return true, switches, detail
}

// countDots counts dot-colored pixels inside one offscreen card.
func countDots(img image.Image, x0, y0, w, h int) (dots, bars int) {
	dr, dg, db := want8(dotR), want8(dotG), want8(dotB)
	br, bg, bb := want8(barR), want8(barG), want8(barB)
	for y := y0; y < y0+h; y++ {
		for x := x0; x < x0+w; x++ {
			r, g, b := sample8(img, x, y)
			if closeEnough(r, dr, dotTol) && closeEnough(g, dg, dotTol) && closeEnough(b, db, dotTol) {
				dots++
			}
			if closeEnough(r, br, dotTol) && closeEnough(g, bg, dotTol) && closeEnough(b, bb, dotTol) {
				bars++
			}
		}
	}
	return dots, bars
}

// probePixels asserts the tier visual order offscreen: high holds more
// dots and a longer scale bar than medium, medium more than low.
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
	paintQ123Frame(dc)
	img := dc.Image()
	_ = dc.Close()

	xs := []int{offCardX0, offCardX1, offCardX2}
	counts := make([]int, 3)
	bars := make([]int, 3)
	for i := range xs {
		counts[i], bars[i] = countDots(img, xs[i], offCardY, offCardW, offCardH)
	}
	ok := counts[0] > counts[1] && counts[1] > counts[2] && counts[2] > 0 &&
		bars[0] > bars[1] && bars[1] > bars[2] && bars[2] > 0
	detail := fmt.Sprintf("dots=%d/%d/%d bars=%d/%d/%d tol=%d",
		counts[0], counts[1], counts[2], bars[0], bars[1], bars[2], dotTol)
	return ok, detail
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
	paintQ123Frame(dc)
	img := dc.Image()
	_ = dc.Close()

	f, err := os.Open(goldenPath)
	if err != nil {
		if err := os.MkdirAll("examples/game_save/testdata", 0o755); err != nil {
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
	var switches int
	p.LogicOK, switches, p.Detail = probeLogic()
	p.Switches = switches
	p.PixOK, p.PixDetail = probePixels()
	var wrote bool
	p.GoldenOK, p.GoldenChanged, wrote = probeGolden()
	p.GoldenWrote = wrote
	p.OK = p.LogicOK && p.PixOK && p.GoldenOK
	return p
}

// qSim is the live window state: one real Quality handle cycles tiers,
// the highlight frame follows, counters feed the gate JSON.
type qSim struct {
	q            save.Quality
	active       int
	tierClock    float64
	switches     int
	switchErrors int
	continuous   bool
	frames       [3]int
	elapsed      [3]float64
	tierPresents [3]int64
	lastPresents int64
	app          *embedder.PipelineApp
	shell        *wrkit.ShellChrome
	phase        *wrkit.PhaseClock
	overlays     [3]*rendering.RenderBox
	activeL      *rendering.RenderText
	switchL      *rendering.RenderText
	fpsL         *rendering.RenderText
	specL        [3]*rendering.RenderText
	framesTotal  int
}

type ticker struct{ s *qSim }

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
	s.framesTotal++
	s.frames[s.active]++
	s.elapsed[s.active] += dt
	// Attribute real presents to the active tier: the delta since the
	// last tick belongs to whichever tier is on screen now.
	if s.app != nil {
		cur := s.app.PresentCount()
		if d := cur - s.lastPresents; d > 0 {
			s.tierPresents[s.active] += d
		}
		s.lastPresents = cur
	}
	s.tierClock += dt
	if s.tierClock >= switchEveryS {
		s.tierClock -= switchEveryS
		next := (s.active + 1) % 3
		// Same-scene switch: only the level changes, nothing reloads.
		if err := s.q.Switch(tierOrder[next]); err != nil {
			s.switchErrors++
			s.continuous = false
		} else if s.q.Level() != tierOrder[next] {
			s.switchErrors++
			s.continuous = false
		} else {
			s.active = next
			s.switches++
		}
		for i := range s.overlays {
			s.overlays[i].MarkNeedsPaint()
		}
	}
	phase := s.phase.Advance(dt)
	snap := s.app.Metrics().Snapshot()
	fps := 0.0
	if snap.AvgFrameIntervalMs > 1e-6 {
		fps = 1000.0 / snap.AvgFrameIntervalMs
	}
	spec := s.q.Spec()
	s.activeL.SetText(fmt.Sprintf("当前档 %s", string(s.q.Level())))
	s.switchL.SetText(fmt.Sprintf("切档数 %d 坏切 %d", s.switches, s.switchErrors))
	s.fpsL.SetText(fmt.Sprintf("帧率 %.0f", fps))
	for i := range s.overlays {
		s.overlays[i].MarkNeedsPaint()
	}
	_ = spec
	gateOK := s.switchErrors == 0
	s.shell.NoteHUDTick(dt)
	s.shell.UpdateHUD("save-q123", phase, s.app, gateOK,
		fmt.Sprintf("tier=%s switches=%d", string(s.q.Level()), s.switches),
		fmt.Sprintf("frames=%d/%d/%d", s.frames[0], s.frames[1], s.frames[2]))
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
	caseFlag := flag.String("case", "q123", "scenario case (only q123)")
	autoOnly := flag.Bool("auto-only", false, "probes + short window, JSON gate on stdout")
	manualSeconds := flag.Int("manual-seconds", 0, "manual phase seconds (0 = until close)")
	flag.Parse()

	if *caseFlag != "q123" {
		fmt.Fprintf(os.Stderr, "FAIL: --case=%q want q123 (only three-tier scene)\n", *caseFlag)
		os.Exit(1)
	}
	wrkit.EnsureUIFace()

	probe := runProbes()
	fmt.Fprintf(os.Stderr, "game_save: probes ok=%v logic=%v pix=%v golden=%v(wrote=%v changed=%d) %s | %s\n",
		probe.OK, probe.LogicOK, probe.PixOK, probe.GoldenOK, probe.GoldenWrote,
		probe.GoldenChanged, probe.Detail, probe.PixDetail)
	if !probe.OK {
		if *autoOnly {
			failJSON(probe)
		} else {
			fmt.Fprintln(os.Stderr, "game_save: selftest FAIL, not opening window")
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

	shell := wrkit.NewShell(winW, winH, "game_save — 16.2 三档 (save-q123)", []string{
		"高1000粒8灯1.00 · 中500粒4灯0.75 · 低200粒2灯0.50",
		"黄框=当前档·每2秒一切",
		"切档只换档·场景不重载不闪",
		"右栏 当前档/切档数/帧率",
		"点数=粒子数·底条=分辨率比",
		"JSON见 ability_extra",
	})

	q0, err := save.NewQuality(save.LevelHigh)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: NewQuality:", err)
		os.Exit(1)
	}
	sim := &qSim{q: q0, continuous: true}
	if secs > 0 {
		sim.phase = wrkit.NewPhaseClock(float64(secs)*0.5, float64(secs)*0.8)
	} else {
		sim.phase = wrkit.NewPhaseClock(0, 0)
	}
	sim.shell = shell

	titles := []string{"HIGH 高", "MEDIUM 中", "LOW 低"}
	xs := []float64{cardX0, cardX1, cardX2}
	for i, lvl := range tierOrder {
		spec, _ := save.SpecFor(lvl)
		r, g, b := tierColor(lvl)
		shell.Body.Place(wrkit.Label(titles[i], 13, 0.55, 0.75, 0.95), xs[i], cardY-24)
		card := rendering.NewRenderBox()
		card.FixedWidth, card.FixedHeight = cardW, cardH
		rCopy, gCopy, bCopy, specCopy := r, g, b, spec
		card.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
			if pc == nil || pc.DC == nil {
				return
			}
			ax, ay := pc.Abs(0, 0)
			paintCard(pc.DC, ax, ay, cardW, cardH, specCopy, rCopy, gCopy, bCopy, liveDot)
		}
		shell.Body.Place(card, xs[i], cardY)
		hl := rendering.NewRenderBox()
		hl.FixedWidth, hl.FixedHeight = cardW, cardH
		idx := i
		hl.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
			if pc == nil || pc.DC == nil {
				return
			}
			ax, ay := pc.Abs(0, 0)
			if sim.active == idx {
				pc.DC.SetRGBA(1, 0.9, 0.2, 1)
				pc.DC.SetLineWidth(3)
			} else {
				pc.DC.SetRGBA(0.4, 0.42, 0.48, 1)
				pc.DC.SetLineWidth(1)
			}
			pc.DC.DrawRectangle(ax, ay, cardW, cardH)
			_ = pc.DC.Stroke()
		}
		shell.Body.Place(hl, xs[i], cardY)
		sim.overlays[i] = hl
		info := wrkit.Label(fmt.Sprintf("%d粒 %d灯 %.2f", spec.Particles, spec.Lights, spec.Scale),
			12, 0.70, 0.78, 0.88)
		shell.Body.Place(info, xs[i], cardY+cardH+8)
		sim.specL[i] = info
	}

	shell.Body.Place(wrkit.Label("COUNTERS 计数器", 13, 0.55, 0.75, 0.95), cardX2, cardY+cardH+36)
	sim.activeL = wrkit.Label("当前档 high", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.activeL, cardX2, cardY+cardH+62)
	sim.switchL = wrkit.Label("切档数 0 坏切 0", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.switchL, cardX2, cardY+cardH+88)
	sim.fpsL = wrkit.Label("帧率 0", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.fpsL, cardX2, cardY+cardH+114)

	shell.Body.Place(wrkit.Label("黄框=当前档 · 切档只换档不重载 · 点数即粒子数", 12, 0.70, 0.78, 0.88), cardX0, noteY)

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "game_save", Decorations: true})
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
				fmt.Fprintf(os.Stderr, "game_save: close (%s)\n", win.Backend())
				return
			case platform.EventPointer:
				summary.Pointer++
				if manualMode {
					fmt.Fprintf(os.Stderr, "game_save: pointer %s (%.0f,%.0f) n=%d\n",
						ev.Pointer, ev.X, ev.Y, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_save switches=%d events=%d",
							sim.switches, summary.Pointer+summary.Key+summary.Resize))
					}
				}
				return
			case platform.EventKey:
				if ev.Pressed {
					summary.Key++
					if manualMode {
						fmt.Fprintf(os.Stderr, "game_save: key n=%d\n", summary.Pointer+summary.Key+summary.Resize)
						if ctl != nil {
							ctl.SetTitle(fmt.Sprintf("game_save switches=%d events=%d",
								sim.switches, summary.Pointer+summary.Key+summary.Resize))
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
					fmt.Fprintf(os.Stderr, "game_save: resize %dx%d n=%d\n", ev.Width, ev.Height, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_save switches=%d events=%d",
							sim.switches, summary.Pointer+summary.Key+summary.Resize))
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
	cont := 0
	if sim.continuous && sim.switchErrors == 0 {
		cont = 1
	}
	fpsTier := make([]float64, 3)
	for i := range fpsTier {
		if sim.elapsed[i] > 1e-9 {
			// Steady rate = real presents attributed to the tier over
			// the wall time that tier was live, not ticker ticks.
			fpsTier[i] = float64(sim.tierPresents[i]) / sim.elapsed[i]
		}
	}
	// Tier numbers come from the live engine specs, never hardcoded here.
	hiSpec, _ := save.SpecFor(save.LevelHigh)
	medSpec, _ := save.SpecFor(save.LevelMedium)
	lowSpec, _ := save.SpecFor(save.LevelLow)
	extra := map[string]any{
		"case":           "q123",
		"probe_ok":       probeOK,
		"switches":       sim.switches,
		"switch_errors":  sim.switchErrors,
		"continuous":     cont,
		"frames_high":    sim.frames[0],
		"frames_medium":  sim.frames[1],
		"frames_low":     sim.frames[2],
		"fps_high":       fpsTier[0],
		"fps_medium":     fpsTier[1],
		"fps_low":        fpsTier[2],
		"particles_high": hiSpec.Particles,
		"particles_med":  medSpec.Particles,
		"particles_low":  lowSpec.Particles,
		"lights_high":    hiSpec.Lights,
		"lights_med":     medSpec.Lights,
		"lights_low":     lowSpec.Lights,
		"scale_high":     hiSpec.Scale,
		"scale_med":      medSpec.Scale,
		"scale_low":      lowSpec.Scale,
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
		if presents < 1 || !probe.OK || sim.switches < 3 || sim.switchErrors != 0 || cont != 1 {
			fmt.Fprintf(os.Stderr, "FAIL: presents=%d probe=%v switches=%d errors=%d continuous=%d (want >=1, true, >=3, 0, 1)\n",
				presents, probe.OK, sim.switches, sim.switchErrors, cont)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "game_save: OK presents=%d switches=%d errors=%d fps=%.0f/%.0f/%.0f elapsed=%.1fs\n",
			presents, sim.switches, sim.switchErrors, fpsTier[0], fpsTier[1], fpsTier[2], elapsed)
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
		"presents":      presents,
		"elapsed_sec":   elapsed,
		"switches":      sim.switches,
		"switch_errors": sim.switchErrors,
		"continuous":    cont,
		"frames":        []int{sim.frames[0], sim.frames[1], sim.frames[2]},
		"fps_tier":      fpsTier,
		"probe_ok":      probeOK,
		"timed":         summary.Timed,
		"note":          summary.Note,
	})
	fmt.Println(string(b))
	fmt.Fprintf(os.Stderr, "game_save: backend=%s presents=%d switches=%d errors=%d elapsed=%.1fs\n",
		win.Backend(), presents, sim.switches, sim.switchErrors, elapsed)
}
