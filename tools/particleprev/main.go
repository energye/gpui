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
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH = 1200, 800
	abilityID  = "particle-preview"
	scenario   = "tools/particleprev"

	goldenPathFmt = "tools/particleprev/testdata/particle_preview_%s_golden.png"
	lastPathFmt   = "tools/particleprev/testdata/particle_preview_%s_last.png"

	probePixelTol = 8
)

const (
	bgR, bgG, bgB = 0.08, 0.09, 0.11
)

const (
	offW, offH = 480, 270
)

type probeResult struct {
	Spawned, Alive, Child int
	Shape                 string
	EmptyOK               bool
	BadOK                 bool
	ParityOK              bool
	PixOK                 bool
	PixDetail             string
	GoldenOK              bool
	GoldenChanged         int
	GoldenWrote           bool
	OK                    bool
}

func paintEffectFrame(dc *render.Context, eff Effect) {
	dc.ClearWithColor(render.RGBA{R: bgR, G: bgG, B: bgB, A: 1})
	em, err := eff.BuildEmitter()
	if err != nil {
		dc.SetRGB(0.6, 0.65, 0.75)
		dc.DrawRectangle(20, 20, 100, 30)
		_ = dc.Fill()
		return
	}
	if _, err := em.Spawn(eff.ProbeSpawn); err != nil {
		return
	}
	em.Update(core.Milliseconds(eff.ProbeUpdateMs))
	cx, cy := float64(offW)/2, float64(offH)/2
	maxR := 0.0
	for _, p := range em.Particles() {
		if !p.Alive() {
			continue
		}
		dx := p.Pos.X - eff.Config().Origin.X
		dy := p.Pos.Y - eff.Config().Origin.Y
		if d := math.Hypot(dx, dy); d > maxR {
			maxR = d
		}
	}
	if maxR < 1 {
		maxR = 1
	}
	for _, p := range em.Particles() {
		if !p.Alive() {
			continue
		}
		c := em.ColorOf(p)
		if c.A <= 0 {
			continue
		}
		dx := (p.Pos.X - eff.Config().Origin.X) / maxR
		dy := (p.Pos.Y - eff.Config().Origin.Y) / maxR
		x := cx + dx*(float64(offW)/2-12)
		y := cy + dy*(float64(offH)/2-12)
		if x < 2 {
			x = 2
		}
		if y < 2 {
			y = 2
		}
		if x > offW-6 {
			x = offW - 6
		}
		if y > offH-6 {
			y = offH - 6
		}
		cr, cg, cb, _ := c.ToRender().RGBA()
		dc.SetRGB(float64(cr)/255.0, float64(cg)/255.0, float64(cb)/255.0)
		dc.DrawRectangle(x, y, 4, 4)
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

func probePixels(eff Effect) (bool, string) {
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
	paintEffectFrame(dc, eff)
	img := dc.Image()
	_ = dc.Close()

	br, bg, bb := sample8(img, 4, 4)
	okBG := closeEnough(br, want8(bgR)) && closeEnough(bg, want8(bgG)) && closeEnough(bb, want8(bgB))
	lit := 0
	for y := 0; y < offH; y += 2 {
		for x := 0; x < offW; x += 2 {
			r, g, b := sample8(img, x, y)
			if !closeEnough(r, want8(bgR)) || !closeEnough(g, want8(bgG)) || !closeEnough(b, want8(bgB)) {
				lit++
			}
		}
	}
	detail := fmt.Sprintf("corner=(%d,%d,%d) lit=%d tol=%d", br, bg, bb, lit, probePixelTol)
	return okBG && lit >= 20, detail
}

func goldenPaths(name string) (string, string) {
	if name == "" {
		name = "unknown"
	}
	return fmt.Sprintf(goldenPathFmt, name), fmt.Sprintf(lastPathFmt, name)
}

func probeGolden(eff Effect) (ok bool, changed int, wrote bool) {
	goldenPath, lastPath := goldenPaths(eff.Name)
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
	paintEffectFrame(dc, eff)
	img := dc.Image()
	_ = dc.Close()

	f, err := os.Open(goldenPath)
	if err != nil {
		if err := os.MkdirAll("tools/particleprev/testdata", 0o755); err != nil {
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

func runProbes(dir, name string) (probeResult, Effect) {
	var r probeResult
	// Display side: the requested effect, placeholder when missing so an
	// empty choice still opens instead of crashing.
	disp, derr := OpenEffect(dir, name)
	if derr != nil {
		return r, disp
	}
	if !disp.HasFile {
		// Missing effect file: still gate the canonical fire probes so the
		// empty choice proves "opens without crashing" via the window run.
		fire, ferr := OpenEffect(dir, "fire")
		if ferr != nil || !fire.HasFile {
			return r, disp
		}
		r.ParityOK = VerifyReplay(fire) == nil
		r.PixOK, r.PixDetail = probePixels(fire)
		r.GoldenOK, r.GoldenChanged, r.GoldenWrote = probeGolden(fire)
		r.Shape = "placeholder"
	} else {
		r.Spawned, r.Alive, r.Child, r.Shape = disp.Spawned, disp.Alive, disp.Child, disp.ShapeKind
		r.ParityOK = VerifyReplay(disp) == nil
		r.PixOK, r.PixDetail = probePixels(disp)
		r.GoldenOK, r.GoldenChanged, r.GoldenWrote = probeGolden(disp)
	}

	miss, merr := OpenEffect(dir, "no_such_effect")
	r.EmptyOK = merr == nil && !miss.HasFile && len(miss.Notes) == 1

	bad, _ := OpenEffect(dir, "bad")
	_, berr := OpenEffect(dir, "bad")
	r.BadOK = berr != nil && !bad.HasFile && bad.FileErr != ""

	r.OK = r.ParityOK && r.PixOK && r.GoldenOK && r.EmptyOK && r.BadOK
	if disp.HasFile {
		r.OK = r.OK && disp.Spawned > 0 && disp.Alive > 0
	}
	return r, disp
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

func runSecondsEnv(def int) int {
	if v := os.Getenv("RUN_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
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

var atlasBuf *render.ImageBuf

func paintLiveCard(pc *rendering.PaintContext, w, h float64, e *particle.Emitter, size float64) {
	if pc == nil || pc.DC == nil {
		return
	}
	ax, ay := pc.Abs(0, 0)
	pc.DC.SetRGB(bgR, bgG, bgB)
	pc.DC.DrawRectangle(ax, ay, w, h)
	_ = pc.DC.Fill()
	if e == nil {
		return
	}
	minX, minY, maxX, maxY := 0.0, 0.0, 1.0, 1.0
	first := true
	for _, p := range e.Particles() {
		if !p.Alive() {
			continue
		}
		if first {
			minX, minY, maxX, maxY = p.Pos.X, p.Pos.Y, p.Pos.X, p.Pos.Y
			first = false
			continue
		}
		if p.Pos.X < minX {
			minX = p.Pos.X
		}
		if p.Pos.Y < minY {
			minY = p.Pos.Y
		}
		if p.Pos.X > maxX {
			maxX = p.Pos.X
		}
		if p.Pos.Y > maxY {
			maxY = p.Pos.Y
		}
	}
	if first {
		return
	}
	spanX, spanY := maxX-minX, maxY-minY
	if spanX < 1 {
		spanX = 1
	}
	if spanY < 1 {
		spanY = 1
	}
	var rs []render.AtlasSprite
	for _, p := range e.Particles() {
		if !p.Alive() {
			continue
		}
		c := e.ColorOf(p)
		if c.A <= 0 {
			continue
		}
		x := ax + 20 + (p.Pos.X-minX)/spanX*(w-40)
		y := ay + 20 + (p.Pos.Y-minY)/spanY*(h-40)
		rs = append(rs, render.AtlasSprite{
			SrcX: 0, SrcY: 0, SrcW: 8, SrcH: 8,
			DstX: x - size/2, DstY: y - size/2, DstW: size, DstH: size,
			Opacity: c.A, Tint: c.ToRender(), Filter: render.InterpNearest,
		})
	}
	_, _ = pc.DC.DrawAtlasEx(atlasBuf, rs, render.AtlasDrawOptions{})
}

type liveSim struct {
	app    *embedder.PipelineApp
	shell  *wrkit.ShellChrome
	phase  *wrkit.PhaseClock
	box    *rendering.RenderBox
	statL  *rendering.RenderText
	noteL  *rendering.RenderText
	em     *particle.Emitter
	eff    Effect
	rate   float64
	alive  int
	spawns int
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
	if s.em != nil {
		s.em.Update(core.SecondsFloat(dt))
		s.alive = s.em.Alive()
		s.spawns = s.em.Spawned()
	}
	if s.box != nil {
		s.box.MarkNeedsPaint()
	}
	if s.statL != nil {
		s.statL.SetText(fmt.Sprintf("alive %d spawned %d rate %.0f", s.alive, s.spawns, s.rate))
	}
	phase := s.phase.Advance(dt)
	gateOK := s.alive > 0
	s.shell.NoteHUDTick(dt)
	s.shell.UpdateHUD("particle-preview", phase, s.app, gateOK,
		fmt.Sprintf("alive=%d spawned=%d", s.alive, s.spawns),
		fmt.Sprintf("rate=%.0f shape=%s", s.rate, s.eff.ShapeKind))
	s.app.ScheduleFrame()
	return true
}

type manualSummary struct {
	Pointer, Key, Resize int
	Timed                bool
	Note                 string
}

func resetEmitter(sim *liveSim, rate float64) {
	if sim == nil {
		return
	}
	em, err := sim.eff.BuildEmitter()
	if err != nil {
		return
	}
	cfg := em.Config()
	cfg.Rate = rate
	em2, err := particle.NewEmitter(cfg, sim.eff.Seed)
	if err != nil {
		return
	}
	sim.em, sim.rate = em2, rate
	if sim.statL != nil {
		sim.statL.SetText(fmt.Sprintf("alive %d spawned %d rate %.0f", 0, 0, rate))
	}
}

func main() {
	project := flag.String("project", "tools/particleprev/testdata", "particle project dir")
	effect := flag.String("effect", "fire", "effect name (file effect_<name>.json)")
	autoOnly := flag.Bool("auto-only", false, "probes + short window, JSON gate on stdout")
	manualSeconds := flag.Int("manual-seconds", 0, "manual phase seconds (0 = until close)")
	flag.Parse()

	wrkit.EnsureUIFace()
	atlasBuf = buildParticleAtlas()

	probe, eff := runProbes(*project, *effect)
	fmt.Fprintf(os.Stderr, "particleprev: probes ok=%v effect=%s replay=%d/%d/%d shape=%s empty=%v bad=%v parity=%v pix=%v golden=%v(wrote=%v changed=%d) %s\n",
		probe.OK, *effect, probe.Spawned, probe.Alive, probe.Child, probe.Shape,
		probe.EmptyOK, probe.BadOK, probe.ParityOK,
		probe.PixOK, probe.GoldenOK, probe.GoldenWrote, probe.GoldenChanged, probe.PixDetail)
	if !probe.OK {
		if *autoOnly {
			failJSON(probe)
		} else {
			fmt.Fprintln(os.Stderr, "particleprev: selftest FAIL, not opening window")
		}
		os.Exit(1)
	}

	em, err := eff.BuildEmitter()
	if err != nil {
		// Missing effect placeholder: the probes already gated the canonical
		// fire effect, so open an empty live emitter instead of crashing.
		fmt.Fprintln(os.Stderr, "particleprev: placeholder effect, empty live emitter:", err)
		em, _ = eff.BuildEmitter()
		if em == nil {
			fire, ferr := OpenEffect(*project, "fire")
			if ferr != nil {
				fmt.Fprintln(os.Stderr, "FAIL: build emitter:", err)
				os.Exit(1)
			}
			em, err = fire.BuildEmitter()
			if err != nil {
				fmt.Fprintln(os.Stderr, "FAIL: build emitter:", err)
				os.Exit(1)
			}
		}
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

	shell := wrkit.NewShell(winW, winH, "particleprev — 17.1 粒子预览 (particle-preview)", []string{
		"火/烟走真引擎数",
		"+/- 减半/加倍即看",
		"0 回 filed 速率",
		"颜色=引擎 ColorOf",
		"底栏 alive/spawned",
		"JSON见 ability_extra",
	})

	sim := &liveSim{em: em, eff: eff, rate: eff.Rate, shell: shell}
	if secs > 0 {
		sim.phase = wrkit.NewPhaseClock(float64(secs)*0.5, float64(secs)*0.8)
	} else {
		sim.phase = wrkit.NewPhaseClock(0, 0)
	}

	sim.box = rendering.NewRenderBox()
	sim.box.FixedWidth, sim.box.FixedHeight = 880, 560
	live := sim
	sim.box.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		paintLiveCard(pc, size.Width, size.Height, live.em, 8)
	}
	shell.Body.Place(sim.box, 16, 44)
	shell.Body.Place(wrkit.Label("EFFECT 特效区", 13, 0.55, 0.75, 0.95), 16, 20)
	sim.statL = wrkit.Label("alive 0", 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.statL, 16, 620)
	sim.noteL = wrkit.Label("+/-调速率即看, 0回 filed 值; 关窗出汇总 JSON", 12, 0.70, 0.78, 0.88)
	shell.Body.Place(sim.noteL, 16, 648)

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "particleprev", Decorations: true})
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
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose, platform.EventCloseRequested:
				fmt.Fprintf(os.Stderr, "particleprev: close (%s)\n", win.Backend())
				return
			case platform.EventPointer:
				summary.Pointer++
				if manualMode {
					fmt.Fprintf(os.Stderr, "particleprev: pointer %s (%.0f,%.0f) n=%d\n",
						ev.Pointer, ev.X, ev.Y, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("particleprev events=%d", summary.Pointer+summary.Key+summary.Resize))
					}
				}
				return
			case platform.EventKey:
				if ev.Pressed {
					summary.Key++
					switch ev.Rune {
					case '+', '=':
						resetEmitter(live, live.rate*2)
					case '-', '_':
						resetEmitter(live, live.rate/2)
					case '0':
						resetEmitter(live, live.eff.Rate)
					}
					if manualMode {
						fmt.Fprintf(os.Stderr, "particleprev: key n=%d rate=%.0f\n", summary.Pointer+summary.Key+summary.Resize, live.rate)
						if ctl != nil {
							ctl.SetTitle(fmt.Sprintf("particleprev events=%d rate=%.0f", summary.Pointer+summary.Key+summary.Resize, live.rate))
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
					fmt.Fprintf(os.Stderr, "particleprev: resize %dx%d n=%d\n", ev.Width, ev.Height, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("particleprev events=%d", summary.Pointer+summary.Key+summary.Resize))
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
		"effect":   *effect,
		"alive":    sim.alive,
		"spawned":  sim.spawns,
		"rate":     sim.rate,
		"probe_ok": probeOK,
		"case":     "particle",
	}

	if *autoOnly {
		report := wrgate.BuildReport(wrgate.BuildInput{
			AbilityID:     abilityID,
			Scenario:      scenario,
			Snap:          snap,
			PresentCount:  presents,
			ElapsedSec:    elapsed,
			SurfaceAreaPx: winW * winH,
			Extra:         extra,
		})
		raw, _ := json.Marshal(report)
		fmt.Println(string(raw))
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{MinPresents: 1}); err != nil {
			fmt.Fprintln(os.Stderr, "FAIL:", err)
			os.Exit(1)
		}
		if presents < 1 || sim.alive <= 0 {
			fmt.Fprintf(os.Stderr, "FAIL: presents=%d alive=%d (want >=1, >0)\n", presents, sim.alive)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "particleprev: OK presents=%d alive=%d spawned=%d elapsed=%.1fs\n",
			presents, sim.alive, sim.spawns, elapsed)
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
		"alive":       sim.alive,
		"spawned":     sim.spawns,
		"rate":        sim.rate,
		"probe_ok":    probeOK,
		"timed":       summary.Timed,
		"note":        summary.Note,
	})
	fmt.Println(string(b))
	fmt.Fprintf(os.Stderr, "particleprev: backend=%s presents=%d alive=%d spawned=%d elapsed=%.1fs\n",
		win.Backend(), presents, sim.alive, sim.spawns, elapsed)
}
