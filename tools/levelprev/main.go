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
	"github.com/energye/gpui/game/tilemap"
	"github.com/energye/gpui/game/world"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH = 1200, 800
	abilityID  = "level-preview"
	scenario   = "tools/levelprev"

	goldenPath = "tools/levelprev/testdata/level_preview_golden.png"
	lastPath   = "tools/levelprev/testdata/level_preview_last.png"

	probePixelTol = 8
)

const (
	bgR, bgG, bgB = 0.08, 0.09, 0.11
)

const (
	offW, offH = 480, 270
)

type probeResult struct {
	SmallTiles, SmallEntities int
	BigTiles, BigEntities     int
	BigOpenMs                 float64
	EmptyOK                   bool
	BadMapKeptScene           bool
	BadSceneKeptMap           bool
	ParityOK                  bool
	PixOK                     bool
	PixDetail                 string
	GoldenOK                  bool
	GoldenChanged             int
	GoldenWrote               bool
	OK                        bool
}

func kindColor(k tilemap.LayerKind) (float64, float64, float64) {
	switch k {
	case tilemap.KindDecoration:
		return 0.55, 0.55, 0.25
	case tilemap.KindCollision:
		return 0.70, 0.25, 0.25
	case tilemap.KindNavigation:
		return 0.25, 0.45, 0.75
	case tilemap.KindOcclusion:
		return 0.45, 0.45, 0.48
	default:
		return 0.25, 0.60, 0.30
	}
}

func paintPreviewFrame(dc *render.Context, p Preview) {
	dc.ClearWithColor(render.RGBA{R: bgR, G: bgG, B: bgB, A: 1})
	m, ok := p.Tilemap()
	if ok && m.W() > 0 && m.H() > 0 {
		cw := float64(offW) / float64(m.W())
		ch := float64(offH) / float64(m.H())
		if cw > ch {
			cw = ch
		} else {
			ch = cw
		}
		if cw < 1 {
			cw, ch = 1, 1
		}
		ox := (float64(offW) - cw*float64(m.W())) / 2
		oy := (float64(offH) - ch*float64(m.H())) / 2
		for li, l := range m.Layers() {
			r, g, b := kindColor(l.Kind())
			gids := l.GIDs()
			for row := 0; row < m.H(); row++ {
				for col := 0; col < m.W(); col++ {
					if gids[row*m.W()+col] == 0 {
						continue
					}
					x := ox + float64(col)*cw
					y := oy + float64(row)*ch
					shade := 1.0 - 0.08*float64(li%3)
					dc.SetRGB(r*shade, g*shade, b*shade)
					dc.DrawRectangle(x, y, cw-0.5, ch-0.5)
					_ = dc.Fill()
				}
			}
		}
		for _, o := range m.Objects() {
			b := o.Bounds()
			sx := ox + b.X/float64(m.W()*32)*float64(offW)*0.9
			sy := oy + b.Y/float64(m.H()*32)*float64(offH)*0.9
			dc.SetRGB(0.95, 0.85, 0.25)
			dc.DrawRectangle(sx, sy, 8, 8)
			_ = dc.Fill()
		}
	}
	if sf, ok := p.Scene(); ok {
		sc, ids, err := sf.Open()
		if err == nil {
			n := 0
			for _, id := range ids {
				wt, err := sc.World().WorldOf(id)
				if err != nil {
					continue
				}
				x := 20 + wt.Pos.X*1.5
				y := 20 + wt.Pos.Y*1.5
				if x < 0 {
					x = 0
				}
				if y < 0 {
					y = 0
				}
				if x > offW-6 {
					x = offW - 6
				}
				if y > offH-6 {
					y = offH - 6
				}
				dc.SetRGB(0.95, 0.25, 0.25)
				dc.DrawRectangle(x, y, 5, 5)
				_ = dc.Fill()
				n++
				if n >= 400 {
					break
				}
			}
		}
		_ = sc
	}
	var _ = world.IdentityTransform
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

func probePixels(p Preview) (bool, string) {
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
	paintPreviewFrame(dc, p)
	img := dc.Image()
	_ = dc.Close()

	br, bg, bb := sample8(img, 4, 4)
	okBG := closeEnough(br, want8(bgR)) && closeEnough(bg, want8(bgG)) && closeEnough(bb, want8(bgB))
	// Cell (0,0) of proj_small is a filled ground tile; its center must not be background.
	cr, cg, cb := sample8(img, 105, 34)
	cellDrawn := !(closeEnough(cr, want8(bgR)) && closeEnough(cg, want8(bgG)) && closeEnough(cb, want8(bgB)))
	detail := fmt.Sprintf("corner=(%d,%d,%d) cell00=(%d,%d,%d) tol=%d", br, bg, bb, cr, cg, cb, probePixelTol)
	return okBG && cellDrawn, detail
}

func probeGolden(p Preview) (ok bool, changed int, wrote bool) {
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
	paintPreviewFrame(dc, p)
	img := dc.Image()
	_ = dc.Close()

	f, err := os.Open(goldenPath)
	if err != nil {
		if err := os.MkdirAll("tools/levelprev/testdata", 0o755); err != nil {
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

func runProbes(project string) (probeResult, Preview) {
	var r probeResult
	// Display side: the requested project, placeholders when files miss or
	// tear, so empty and bad projects still open instead of crashing.
	disp, _ := OpenProject(project)
	// Gate side: the canonical small project always carries logic, pixel,
	// and golden evidence, independent of which project is displayed.
	small, err := OpenProject("tools/levelprev/testdata/proj_small")
	if err != nil {
		return r, disp
	}
	r.SmallTiles, r.SmallEntities = small.TileCount, small.EntityCount
	m, hasMap := small.Tilemap()
	sf, hasScene := small.Scene()
	if hasMap && hasScene {
		r.ParityOK = VerifyCounts(small, m, true, sf, true) == nil
	}
	r.PixOK, r.PixDetail = probePixels(small)
	r.GoldenOK, r.GoldenChanged, r.GoldenWrote = probeGolden(small)

	empty, err := OpenProject("tools/levelprev/testdata/proj_empty")
	r.EmptyOK = err == nil && !empty.HasMap && !empty.HasScene && len(empty.Notes) == 2

	badm, _ := OpenProject("tools/levelprev/testdata/proj_badmap")
	r.BadMapKeptScene = !badm.HasMap && badm.HasScene && badm.EntityCount == 3 && badm.MapErr != ""
	bads, _ := OpenProject("tools/levelprev/testdata/proj_badscene")
	r.BadSceneKeptMap = !bads.HasScene && bads.HasMap && bads.TileCount == 17 && bads.SceneErr != ""

	t0 := time.Now()
	big, err := OpenProject("tools/levelprev/testdata/proj_big")
	r.BigOpenMs = float64(time.Since(t0).Microseconds()) / 1000.0
	if err == nil {
		r.BigTiles, r.BigEntities = big.TileCount, big.EntityCount
		again, err := OpenProject("tools/levelprev/testdata/proj_big")
		bigOK := err == nil && again.TileCount == big.TileCount && again.EntityCount == big.EntityCount
		r.OK = r.ParityOK && r.PixOK && r.GoldenOK && r.EmptyOK &&
			r.BadMapKeptScene && r.BadSceneKeptMap && bigOK &&
			big.TileCount == 14627 && big.EntityCount == 2000
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

func paintLiveCard(pc *rendering.PaintContext, w, h float64, p Preview, scale float64) {
	if pc == nil || pc.DC == nil {
		return
	}
	ax, ay := pc.Abs(0, 0)
	pc.DC.SetRGB(bgR, bgG, bgB)
	pc.DC.DrawRectangle(ax, ay, w, h)
	_ = pc.DC.Fill()
	m, ok := p.Tilemap()
	if !ok || m.W() <= 0 || m.H() <= 0 {
		pc.DC.SetRGB(0.6, 0.65, 0.75)
		dc := pc.DC
		_ = dc
		return
	}
	cw := w / float64(m.W())
	ch := h / float64(m.H())
	if cw > ch {
		cw = ch
	} else {
		ch = cw
	}
	cw, ch = cw*scale, ch*scale
	if cw < 1 {
		cw, ch = 1, 1
	}
	if cw*float64(m.W()) > w {
		cw = w / float64(m.W())
		ch = cw
	}
	if ch*float64(m.H()) > h {
		ch = h / float64(m.H())
		cw = ch
	}
	ox := ax + (w-cw*float64(m.W()))/2
	oy := ay + (h-ch*float64(m.H()))/2
	for li, l := range m.Layers() {
		r, g, b := kindColor(l.Kind())
		gids := l.GIDs()
		shade := 1.0 - 0.08*float64(li%3)
		pc.DC.SetRGB(r*shade, g*shade, b*shade)
		for row := 0; row < m.H(); row++ {
			for col := 0; col < m.W(); col++ {
				if gids[row*m.W()+col] == 0 {
					continue
				}
				pc.DC.DrawRectangle(ox+float64(col)*cw, oy+float64(row)*ch, cw-0.5, ch-0.5)
				_ = pc.DC.Fill()
			}
		}
	}
	pc.DC.SetRGB(0.95, 0.85, 0.25)
	for _, o := range m.Objects() {
		b := o.Bounds()
		sx := ox + b.X/32*cw
		sy := oy + b.Y/32*ch
		pc.DC.DrawRectangle(sx, sy, cw, ch)
		_ = pc.DC.Fill()
	}
	if sf, ok := p.Scene(); ok {
		if sc, ids, err := sf.Open(); err == nil {
			pc.DC.SetRGB(0.95, 0.25, 0.25)
			n := 0
			for _, id := range ids {
				wt, err := sc.World().WorldOf(id)
				if err != nil {
					continue
				}
				ex := ox + wt.Pos.X/32*cw
				ey := oy + wt.Pos.Y/32*ch
				if ex < ox {
					ex = ox
				}
				if ey < oy {
					ey = oy
				}
				if ex > ox+w-6 {
					ex = ox + w - 6
				}
				if ey > oy+h-6 {
					ey = oy + h - 6
				}
				pc.DC.DrawRectangle(ex, ey, 5, 5)
				_ = pc.DC.Fill()
				n++
				if n >= 600 {
					break
				}
			}
		}
	}
}

type liveSim struct {
	app   *embedder.PipelineApp
	shell *wrkit.ShellChrome
	phase *wrkit.PhaseClock
	box   *rendering.RenderBox
	statL *rendering.RenderText
	noteL *rendering.RenderText
	prev  Preview
	proj  string
	scale float64
	bigMs float64
	opens int
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
	if s.box != nil {
		s.box.MarkNeedsPaint()
	}
	phase := s.phase.Advance(dt)
	gateOK := s.prev.HasMap || s.prev.HasScene
	s.shell.NoteHUDTick(dt)
	s.shell.UpdateHUD("level-preview", phase, s.app, gateOK,
		fmt.Sprintf("tiles=%d ent=%d big=%.0fms", s.prev.TileCount, s.prev.EntityCount, s.bigMs),
		fmt.Sprintf("opens=%d scale=%.2f", s.opens, s.scale))
	s.app.ScheduleFrame()
	return true
}

type manualSummary struct {
	Pointer, Key, Resize int
	Timed                bool
	Note                 string
}

func main() {
	project := flag.String("project", "tools/levelprev/testdata/proj_small", "level project dir")
	autoOnly := flag.Bool("auto-only", false, "probes + short window, JSON gate on stdout")
	manualSeconds := flag.Int("manual-seconds", 0, "manual phase seconds (0 = until close)")
	flag.Parse()

	wrkit.EnsureUIFace()

	probe, p := runProbes(*project)
	fmt.Fprintf(os.Stderr, "levelprev: probes ok=%v small=%d/%d big=%d/%d bigMs=%.1f empty=%v badmap=%v badscene=%v parity=%v pix=%v golden=%v(wrote=%v changed=%d) %s\n",
		probe.OK, probe.SmallTiles, probe.SmallEntities, probe.BigTiles, probe.BigEntities, probe.BigOpenMs,
		probe.EmptyOK, probe.BadMapKeptScene, probe.BadSceneKeptMap, probe.ParityOK,
		probe.PixOK, probe.GoldenOK, probe.GoldenWrote, probe.GoldenChanged, probe.PixDetail)
	if !probe.OK {
		if *autoOnly {
			failJSON(probe)
		} else {
			fmt.Fprintln(os.Stderr, "levelprev: selftest FAIL, not opening window")
		}
		os.Exit(1)
	}

	if p.MapErr != "" || p.SceneErr != "" {
		fmt.Fprintf(os.Stderr, "levelprev: open project with placeholder: map=%q scene=%q\n", p.MapErr, p.SceneErr)
	}
	bigT0 := time.Now()
	big, _ := OpenProject("tools/levelprev/testdata/proj_big")
	bigMs := float64(time.Since(bigT0).Microseconds()) / 1000.0
	_ = big

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

	shell := wrkit.NewShell(winW, winH, "levelprev — 17.1 关卡预览 (level-preview)", []string{
		"绿块 地面/装饰",
		"红洗 碰撞 黄箱怪",
		"红点 实体世界位",
		"+/- 缩放 重开计时",
		"底栏 瓦片/实体/大关",
		"JSON见 ability_extra",
	})

	sim := &liveSim{prev: p, proj: *project, shell: shell, scale: 1.0, bigMs: bigMs, opens: 1}
	if secs > 0 {
		sim.phase = wrkit.NewPhaseClock(float64(secs)*0.5, float64(secs)*0.8)
	} else {
		sim.phase = wrkit.NewPhaseClock(0, 0)
	}

	sim.box = rendering.NewRenderBox()
	sim.box.FixedWidth, sim.box.FixedHeight = 880, 560
	live := sim
	sim.box.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		paintLiveCard(pc, size.Width, size.Height, live.prev, live.scale)
	}
	shell.Body.Place(sim.box, 16, 44)
	shell.Body.Place(wrkit.Label("LEVEL 关卡区", 13, 0.55, 0.75, 0.95), 16, 20)
	sim.statL = wrkit.Label(fmt.Sprintf("tiles %d entities %d big %.0fms", p.TileCount, p.EntityCount, bigMs), 12, 0.92, 0.94, 0.98)
	shell.Body.Place(sim.statL, 16, 620)
	sim.noteL = wrkit.Label("火星文:绿=路沿自接样,黄=摆怪,红点=实体; +/-缩放, R重开", 12, 0.70, 0.78, 0.88)
	shell.Body.Place(sim.noteL, 16, 648)

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "levelprev", Decorations: true})
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
				fmt.Fprintf(os.Stderr, "levelprev: close (%s)\n", win.Backend())
				return
			case platform.EventPointer:
				summary.Pointer++
				if manualMode {
					fmt.Fprintf(os.Stderr, "levelprev: pointer %s (%.0f,%.0f) n=%d\n",
						ev.Pointer, ev.X, ev.Y, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("levelprev events=%d", summary.Pointer+summary.Key+summary.Resize))
					}
				}
				return
			case platform.EventKey:
				if ev.Pressed {
					summary.Key++
					if manualMode {
						fmt.Fprintf(os.Stderr, "levelprev: key n=%d\n", summary.Pointer+summary.Key+summary.Resize)
						if ctl != nil {
							ctl.SetTitle(fmt.Sprintf("levelprev events=%d", summary.Pointer+summary.Key+summary.Resize))
						}
					}
				}
				t0 := time.Now()
				np, oerr := OpenProject(live.proj)
				ms := float64(time.Since(t0).Microseconds()) / 1000.0
				if oerr == nil {
					err = nil
				}
				if ev.Pressed {
					live.prev, live.bigMs, live.opens = np, ms, live.opens+1
					if live.statL != nil {
						live.statL.SetText(fmt.Sprintf("tiles %d entities %d reopen %.0fms", np.TileCount, np.EntityCount, ms))
					}
				}
				return
			case platform.EventResize:
				summary.Resize++
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
				if manualMode {
					fmt.Fprintf(os.Stderr, "levelprev: resize %dx%d n=%d\n", ev.Width, ev.Height, summary.Pointer+summary.Key+summary.Resize)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("levelprev events=%d", summary.Pointer+summary.Key+summary.Resize))
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
		"tiles":       sim.prev.TileCount,
		"entities":    sim.prev.EntityCount,
		"big_open_ms": sim.bigMs,
		"reopens":     sim.opens,
		"empty_ok":    probe.EmptyOK,
		"parity_ok":   probeOK,
		"case":        "level",
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
		if presents < 1 {
			fmt.Fprintf(os.Stderr, "FAIL: presents=%d (want >=1)\n", presents)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "levelprev: OK presents=%d tiles=%d entities=%d bigMs=%.1f elapsed=%.1fs\n",
			presents, sim.prev.TileCount, sim.prev.EntityCount, sim.bigMs, elapsed)
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
		"tiles":       sim.prev.TileCount,
		"entities":    sim.prev.EntityCount,
		"big_open_ms": sim.bigMs,
		"reopens":     sim.opens,
		"probe_ok":    probeOK,
		"timed":       summary.Timed,
		"note":        summary.Note,
	})
	fmt.Println(string(b))
	fmt.Fprintf(os.Stderr, "levelprev: backend=%s presents=%d tiles=%d entities=%d bigMs=%.1f elapsed=%.1fs\n",
		win.Backend(), presents, sim.prev.TileCount, sim.prev.EntityCount, sim.bigMs, elapsed)
}
