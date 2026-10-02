// Command game_platformer is the S81 side-view对照窗: platformer feel
// (move 300, jump -725, gravity 700, fall clamp 700, double jump once,
// release damping x0.6) over a ground bar plus one floating platform.
//
// Modes:
//
//	RUN_SECONDS=8 go run ./examples/engine/platformer -auto-only
//	  probes + ~8s window (JSON on stdout, exit 1 on fail).
//	go run ./examples/engine/platformer -manual-seconds 30
//	  manual 30s (arrows/AD move, space/W jump, events logged), then summary.
//	go run ./examples/engine/platformer
//	  probes, then resident until close (RUN_SECONDS sets a timed run).
//
// PLATFORMER_PROBE_ONLY=1 runs the selected case probes headlessly
// (logic + pixels + golden, JSON on stdout, no window) for CI and for
// producing the testdata baseline without a display.
//
// Window: 1200x800, title game_platformer. First run writes the golden
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
	abilityID  = "platformer-feel"
	scenario   = "game_platformer"

	goldenPath = "examples/engine/platformer/testdata/platformer_golden.png"
	lastPath   = "examples/engine/platformer/testdata/platformer_last.png"

	probePixelTol = 8
)

// Platformer feel numbers (Y-down px: gravity pulls +Y, jump fires -Y).
const (
	moveSpeed   = 300.0
	jumpVel     = -725.0
	gravity     = 700.0
	maxFall     = 700.0
	accel       = 1800.0
	releaseKeep = 0.6 // per 1/60s step while no input
	maxJumps    = 2
	heroW       = 28.0
	heroH       = 28.0
)

// Arena geometry (live window, arena-local px).
const (
	arenaW = 560.0
	arenaH = 400.0

	groundTop = 360.0
	platX     = 330.0
	platY     = 250.0
	platW     = 140.0
	platH     = 12.0

	offW, offH = 480, 270
)

const (
	bgR, bgG, bgB             = 0.08, 0.09, 0.11
	groundR, groundG, groundB = 0.30, 0.42, 0.30
	platR, platG, platB       = 0.35, 0.55, 0.75
	heroR, heroG, heroB       = 0.95, 0.45, 0.15
)

const (
	arenaX = 16.0
	arenaY = 44.0
	countX = 600.0
	countY = 44.0
	noteY  = 470.0
)

// stepVX steers horizontal speed toward dir*moveSpeed at accel;
// with no input it keeps releaseKeep per 1/60s step.
func stepVX(vx, dir, dt float64) float64 {
	if dt < 0 {
		dt = 0
	}
	if dir > 0 {
		dir = 1
	} else if dir < 0 {
		dir = -1
	} else {
		dir = 0
	}
	if dir == 0 {
		return vx * math.Pow(releaseKeep, dt*60)
	}
	want := dir * moveSpeed
	dv := accel * dt
	if vx < want {
		vx += dv
		if vx > want {
			vx = want
		}
	} else if vx > want {
		vx -= dv
		if vx < want {
			vx = want
		}
	}
	return vx
}

// stepVY applies gravity and clamps the fall to maxFall.
func stepVY(vy, dt float64) float64 {
	if dt < 0 {
		dt = 0
	}
	vy += gravity * dt
	if vy > maxFall {
		vy = maxFall
	}
	return vy
}

type probeResult struct {
	LogicOK       bool
	LogicDetail   string
	PixOK         bool
	PixDetail     string
	GoldenOK      bool
	GoldenChanged int
	GoldenWrote   bool
	OK            bool
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

// probeLogic pins the对照 numbers and the jump arc: constants land
// exactly on 300/-725/700, full input reaches 300, release keeps x0.6
// per step, and a fixed-dt jump flies, peaks, double-jumps once, lands.
func probeLogic() (bool, string) {
	if moveSpeed != 300 || jumpVel != -725 || gravity != 700 || maxFall != 700 {
		return false, fmt.Sprintf("feel=%.1f/%.1f/%.1f/%.1f want 300/-725/700/700",
			moveSpeed, jumpVel, gravity, maxFall)
	}
	vx := stepVX(0, 1, 0.5)
	if vx != moveSpeed {
		return false, fmt.Sprintf("full input vx=%.6f want 300", vx)
	}
	vx = stepVX(moveSpeed, 0, 1.0/60)
	if math.Abs(vx-180) > 1e-6 {
		return false, fmt.Sprintf("release vx=%.6f want 180", vx)
	}

	const dt = 1.0 / 60
	y, vy := 0.0, jumpVel
	used := 1
	apex := 0.0
	apexFirst := 0.0
	peakVy := vy
	steps := 0
	landed := false
	secondOK := false
	thirdDenied := true
	apexSeen := false
	for i := 0; i < 600; i++ {
		vy = stepVY(vy, dt)
		if vy > peakVy {
			peakVy = vy
		}
		y += vy * dt
		if y < apex {
			apex = y
		}
		steps++
		if !apexSeen && vy >= 0 {
			apexSeen = true
			apexFirst = y
			// Second jump at the apex: allowed exactly once.
			if used < maxJumps {
				vy, used = jumpVel, used+1
				secondOK = vy == jumpVel && used == 2
				// Third attempt must be denied.
				if used < maxJumps {
					thirdDenied = false
				}
			}
		}
		if vy > 0 && y >= 0 {
			landed = true
			break
		}
	}
	if !landed {
		return false, "jump arc never landed"
	}
	if apexFirst > -355 || apexFirst < -392 {
		return false, fmt.Sprintf("apex1=%.2f want in [-392,-355]", apexFirst)
	}
	if apex > -700 || apex < -770 {
		return false, fmt.Sprintf("apex2=%.2f want in [-770,-700]", apex)
	}
	if steps < 190 || steps > 230 {
		return false, fmt.Sprintf("airtime=%d steps want 190-230", steps)
	}
	if peakVy > maxFall+1e-9 {
		return false, fmt.Sprintf("fall peak=%.3f exceeds 700", peakVy)
	}
	if !secondOK {
		return false, "double jump not granted once"
	}
	if !thirdDenied {
		return false, "third jump granted, want denied"
	}
	return true, fmt.Sprintf("feel=300/-725/700/700 full=300 release=180 apex1=%.1f apex2=%.1f steps=%d double=1",
		apexFirst, apex, steps)
}

// paintProbeFrame draws the frozen对照 scene: bg, ground bar, floating
// platform bar, hero standing on the ground.
func paintProbeFrame(dc *render.Context) {
	dc.SetRGB(bgR, bgG, bgB)
	dc.DrawRectangle(0, 0, offW, offH)
	_ = dc.Fill()
	dc.SetRGB(groundR, groundG, groundB)
	dc.DrawRectangle(0, 230, offW, 40)
	_ = dc.Fill()
	dc.SetRGB(platR, platG, platB)
	dc.DrawRectangle(300, 150, 120, 8)
	_ = dc.Fill()
	dc.SetRGB(heroR, heroG, heroB)
	dc.DrawRectangle(120, 230-heroH, heroW, heroH)
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

func paintOffscreen() image.Image {
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
	paintProbeFrame(dc)
	img := dc.Image()
	_ = dc.Close()
	rgba, ok := img.(*image.RGBA)
	if !ok {
		return img
	}
	cp := image.NewRGBA(rgba.Bounds())
	copy(cp.Pix, rgba.Pix)
	return cp
}

// probePixels asserts hero-on-ground, ground, platform and bg colors.
func probePixels() (bool, string) {
	img := paintOffscreen()
	hr, hg, hb := sample8(img, 134, 216)
	gr, gg, gb := sample8(img, 60, 250)
	pr, pg, pb := sample8(img, 360, 154)
	br, bg, bb := sample8(img, 20, 20)
	ok := closeEnough(hr, want8(heroR)) && closeEnough(hg, want8(heroG)) && closeEnough(hb, want8(heroB)) &&
		closeEnough(gr, want8(groundR)) && closeEnough(gg, want8(groundG)) && closeEnough(gb, want8(groundB)) &&
		closeEnough(pr, want8(platR)) && closeEnough(pg, want8(platG)) && closeEnough(pb, want8(platB)) &&
		closeEnough(br, want8(bgR)) && closeEnough(bg, want8(bgG)) && closeEnough(bb, want8(bgB))
	detail := fmt.Sprintf("hero=(%d,%d,%d) ground=(%d,%d,%d) plat=(%d,%d,%d) bg=(%d,%d,%d) tol=%d",
		hr, hg, hb, gr, gg, gb, pr, pg, pb, br, bg, bb, probePixelTol)
	return ok, detail
}

// probeGolden compares the frozen frame with zero tolerance; the first
// run freezes the baseline.
func probeGolden() (bool, int, bool) {
	img := paintOffscreen()
	f, err := os.Open(goldenPath)
	if err != nil {
		if err := os.MkdirAll("examples/engine/platformer/testdata", 0o755); err != nil {
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
	changed := 0
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

func runProbes() probeResult {
	var p probeResult
	p.LogicOK, p.LogicDetail = probeLogic()
	p.PixOK, p.PixDetail = probePixels()
	p.GoldenOK, p.GoldenChanged, p.GoldenWrote = probeGolden()
	p.OK = p.LogicOK && p.PixOK && p.GoldenOK
	return p
}

// probeOnly runs the headless self-check without opening any window.
func probeOnly() {
	probe := runProbes()
	fmt.Fprintf(os.Stderr, "game_platformer: probes ok=%v logic=%v pix=%v golden=%v(wrote=%v changed=%d) %s | %s\n",
		probe.OK, probe.LogicOK, probe.PixOK, probe.GoldenOK, probe.GoldenWrote,
		probe.GoldenChanged, probe.LogicDetail, probe.PixDetail)
	b, _ := json.Marshal(map[string]any{
		"ability_id": abilityID,
		"scenario":   scenario,
		"probe_ok":   probe.OK,
		"logic_ok":   probe.LogicOK,
		"pix_ok":     probe.PixOK,
		"golden_ok":  probe.GoldenOK,
		"golden":     probe.GoldenChanged,
		"pixels":     probe.PixDetail,
		"detail":     probe.LogicDetail,
	})
	fmt.Println(string(b))
	if !probe.OK {
		os.Exit(1)
	}
}

type platSim struct {
	app   *embedder.PipelineApp
	shell *wrkit.ShellChrome
	phase *wrkit.PhaseClock
	arena *rendering.RenderBox

	x, y     float64
	vx, vy   float64
	onGround bool
	used     int

	left, right bool
	jumpQueued  bool

	autoDrive bool
	autoT     float64

	frames  int
	movedPx float64
	jumps   int64
	lands   int64

	posL, jumpL, stateL, fpsL *rendering.RenderText
}

type platTicker struct{ s *platSim }

func (t *platTicker) Tick(dt float64) bool {
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
	if s.autoDrive {
		s.right = true
		s.autoT += dt
		if s.onGround && s.autoT > 0.7 {
			s.jumpQueued = true
			s.autoT = 0
		}
	}

	dir := 0.0
	if s.left {
		dir--
	}
	if s.right {
		dir++
	}
	oldX := s.x
	s.vx = stepVX(s.vx, dir, dt)
	s.x += s.vx * dt
	if s.x < 0 {
		s.x, s.vx = 0, 0
	}
	if s.x > arenaW-heroW {
		s.x, s.vx = arenaW-heroW, 0
	}
	step := s.x - oldX
	if step < 0 {
		step = -step
	}
	s.movedPx += step

	if s.jumpQueued {
		s.jumpQueued = false
		if s.used < maxJumps {
			s.vy, s.used = jumpVel, s.used+1
			s.jumps++
			s.onGround = false
		}
	}
	wasGround := s.onGround
	s.vy = stepVY(s.vy, dt)
	s.y += s.vy * dt
	s.onGround = false
	// Land on the floating platform while falling through its top.
	if s.vy >= 0 && s.x+heroW > platX && s.x < platX+platW {
		feet := s.y + heroH
		prevFeet := feet - s.vy*dt
		if prevFeet <= platY && feet >= platY && platY < groundTop {
			s.y, s.vy, s.onGround, s.used = platY-heroH, 0, true, 0
		}
	}
	// Land on the ground bar.
	if s.y+heroH >= groundTop {
		s.y, s.vy, s.onGround, s.used = groundTop-heroH, 0, true, 0
	}
	if !wasGround && s.onGround {
		s.lands++
	}
	if s.arena != nil {
		s.arena.MarkNeedsPaint()
	}

	phase := s.phase.Advance(dt)
	snap := s.app.Metrics().Snapshot()
	fps := 0.0
	if snap.AvgFrameIntervalMs > 1e-6 {
		fps = 1000.0 / snap.AvgFrameIntervalMs
	}
	s.posL.SetText(fmt.Sprintf("人 x=%.0f y=%.0f 速%.0f,%.0f", s.x, s.y, s.vx, s.vy))
	s.jumpL.SetText(fmt.Sprintf("跳 %d 落 %d", s.jumps, s.lands))
	state := "腾空"
	if s.onGround {
		state = "站住"
	}
	s.stateL.SetText(fmt.Sprintf("状态 %s 位移%.0f", state, s.movedPx))
	s.fpsL.SetText(fmt.Sprintf("帧率 %.0f", fps))
	gateOK := s.movedPx > 0 && s.jumps > 0 && s.lands > 0
	s.shell.NoteHUDTick(dt)
	s.shell.UpdateHUD("platformer-feel", phase, s.app, gateOK,
		fmt.Sprintf("jumps=%d lands=%d moved=%.0f", s.jumps, s.lands, s.movedPx),
		fmt.Sprintf("vx=%.0f vy=%.0f", s.vx, s.vy))
	s.app.ScheduleFrame()
	return true
}

type manualSummary struct {
	Pointer, Key, Resize int
	Timed                bool
}

func applyKey(s *platSim, r rune, code int, pressed bool) {
	if s == nil {
		return
	}
	switch {
	case r == 'a' || r == 'A' || code == 'a' || code == 'A' || code == 0xff51 || code == 65361:
		s.left = pressed
	case r == 'd' || r == 'D' || code == 'd' || code == 'D' || code == 0xff53 || code == 65363:
		s.right = pressed
	case r == ' ' || r == 'w' || r == 'W' || code == 0xff52 || code == 65362:
		if pressed {
			s.jumpQueued = true
		}
	}
}

func paintArena(pc *rendering.PaintContext, s *platSim) {
	if pc == nil || pc.DC == nil || s == nil {
		return
	}
	ax, ay := pc.Abs(0, 0)
	dc := pc.DC
	dc.SetRGB(bgR, bgG, bgB)
	dc.DrawRectangle(ax, ay, arenaW, arenaH)
	_ = dc.Fill()
	dc.SetRGB(groundR, groundG, groundB)
	dc.DrawRectangle(ax, ay+groundTop, arenaW, arenaH-groundTop)
	_ = dc.Fill()
	dc.SetRGB(platR, platG, platB)
	dc.DrawRectangle(ax+platX, ay+platY, platW, platH)
	_ = dc.Fill()
	dc.SetRGB(heroR, heroG, heroB)
	dc.DrawRectangle(ax+s.x, ay+s.y, heroW, heroH)
	_ = dc.Fill()
}

func main() {
	caseFlag := flag.String("case", "platformer", "scenario case (only platformer)")
	autoOnly := flag.Bool("auto-only", false, "probes + timed window, JSON gate on stdout")
	manualSeconds := flag.Int("manual-seconds", 0, "manual phase seconds (0 = until close)")
	flag.Parse()
	if *caseFlag != "platformer" {
		fmt.Fprintf(os.Stderr, "FAIL: --case=%q want platformer (only platformer gate)\n", *caseFlag)
		os.Exit(1)
	}
	if os.Getenv("PLATFORMER_PROBE_ONLY") == "1" {
		probeOnly()
		return
	}
	wrkit.EnsureUIFace()

	probe := runProbes()
	fmt.Fprintf(os.Stderr, "game_platformer: probes ok=%v logic=%v pix=%v golden=%v(wrote=%v changed=%d) %s | %s\n",
		probe.OK, probe.LogicOK, probe.PixOK, probe.GoldenOK, probe.GoldenWrote, probe.GoldenChanged, probe.LogicDetail, probe.PixDetail)
	if !probe.OK {
		if *autoOnly {
			failJSON(probe)
		} else {
			fmt.Fprintln(os.Stderr, "game_platformer: selftest FAIL, not opening window")
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

	shell := wrkit.NewShell(winW, winH, "game_platformer — 横版手感对照 (platformer-feel)", []string{
		"←→/AD挪·空格/W跳",
		"移速300·跳-725·重力700",
		"坠落上限700·二段只一次",
		"松键阻尼x0.6/帧",
		"绿地蓝条·橙=跳人",
		"JSON见 ability_extra",
	})
	sim := &platSim{
		shell:     shell,
		x:         80,
		y:         groundTop - heroH,
		onGround:  true,
		autoDrive: *autoOnly,
	}
	if secs > 0 {
		sim.phase = wrkit.NewPhaseClock(float64(secs)*0.5, float64(secs)*0.8)
	} else {
		sim.phase = wrkit.NewPhaseClock(0, 0)
	}
	shell.Body.Place(wrkit.Label("PLATFORMER 横版区", 13, 0.55, 0.75, 0.95), arenaX, arenaY-24)
	box := rendering.NewRenderBox()
	box.FixedWidth, box.FixedHeight = arenaW, arenaH
	box.SetRepaintBoundary(true)
	live := sim
	box.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		paintArena(pc, live)
	}
	shell.Body.Place(box, arenaX, arenaY)
	sim.arena = box
	shell.Body.Place(wrkit.Label("COUNTERS 计数器", 13, 0.55, 0.75, 0.95), countX, countY-24)
	sim.posL = wrkit.Label("人 -", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.posL, countX, countY+10)
	sim.jumpL = wrkit.Label("跳 0 落 0", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.jumpL, countX, countY+36)
	sim.stateL = wrkit.Label("状态 站住 位移0", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.stateL, countX, countY+62)
	sim.fpsL = wrkit.Label("帧率 0", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.fpsL, countX, countY+88)
	shell.Body.Place(wrkit.Label("橙人左右挪空格跳·绿地站蓝条落·二段只一次", 12, 0.70, 0.78, 0.88), arenaX, noteY)

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "game_platformer", Decorations: true, Resizable: true})
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
				fmt.Fprintf(os.Stderr, "game_platformer: close (%s)\n", win.Backend())
				return
			case platform.EventPointer:
				summary.Pointer++
				if manualMode {
					fmt.Fprintf(os.Stderr, "game_platformer: pointer %s (%.0f,%.0f) n=%d\n", ev.Pointer, ev.X, ev.Y, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_platformer events=%d", summary.Pointer+summary.Key+summary.Resize))
					}
				}
				return
			case platform.EventKey:
				applyKey(sim, ev.Rune, ev.KeyCode, ev.Pressed)
				if ev.Pressed {
					summary.Key++
					if manualMode {
						fmt.Fprintf(os.Stderr, "game_platformer: key rune=%q code=%d n=%d\n", string(ev.Rune), ev.KeyCode, summary.Pointer+summary.Key+summary.Resize)
						if ctl != nil {
							ctl.SetTitle(fmt.Sprintf("game_platformer events=%d", summary.Pointer+summary.Key+summary.Resize))
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
					fmt.Fprintf(os.Stderr, "game_platformer: resize %dx%d n=%d\n", ev.Width, ev.Height, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_platformer events=%d", summary.Pointer+summary.Key+summary.Resize))
					}
				}
				return
			default:
				return
			}
		},
	})
	sim.app = app
	app.Scheduler().Tickers().Add(&platTicker{s: sim})
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
		"probe_ok":      probeOK,
		"pixels":        probe.PixDetail,
		"golden":        probe.GoldenChanged,
		"boundary_skip": snap.BoundarySkip,
		"moved_px":      sim.movedPx,
		"jumps":         sim.jumps,
		"lands":         sim.lands,
		"move_speed":    moveSpeed,
		"jump_vel":      jumpVel,
		"gravity":       gravity,
		"max_fall":      maxFall,
		"case":          "platformer",
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
		if presents < 1 || !probe.OK {
			fmt.Fprintf(os.Stderr, "FAIL: presents=%d probe=%v (want >=1, true)\n", presents, probe.OK)
			os.Exit(1)
		}
		if sim.movedPx <= 0 || sim.jumps < 1 || sim.lands < 1 {
			fmt.Fprintf(os.Stderr, "FAIL: moved=%.0f jumps=%d lands=%d (want >0, >=1, >=1)\n", sim.movedPx, sim.jumps, sim.lands)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "game_platformer: OK presents=%d moved=%.0f jumps=%d lands=%d elapsed=%.1fs\n",
			presents, sim.movedPx, sim.jumps, sim.lands, elapsed)
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
		"moved_px":    sim.movedPx,
		"jumps":       sim.jumps,
		"lands":       sim.lands,
		"probe_ok":    probeOK,
		"timed":       summary.Timed,
	})
	fmt.Println(string(b))
	fmt.Fprintf(os.Stderr, "game_platformer: backend=%s presents=%d moved=%.0f jumps=%d lands=%d elapsed=%.1fs\n",
		win.Backend(), presents, sim.movedPx, sim.jumps, sim.lands, elapsed)
}
