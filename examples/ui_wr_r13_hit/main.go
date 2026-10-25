// Command ui_wr_r13_hit is the W2 R13 real-window: hit testing ≡ paint
// identity. A scripted pointer probe set (8 probes) must hit exactly the
// drawn target under plain boxes, rounded clip, 30° rotation transform and
// overlap; out-of-clip and empty-area probes must not hit. A real X11 pointer
// click drives the same HitTestPointer path and highlights the target.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_wr_r13_hit -auto-only
//	go run ./examples/ui_wr_r13_hit                 # manual: animate until close
//	go run ./examples/ui_wr_r13_hit -manual-seconds 15
//
// Window: 1200x800. RUN_SECONDS>=5 (§2.5). GPU window required.
//
// Evidence (U21): logic probes (scripted hit set), color/ink probes on the GPU
// snapshot, and a bitwise Golden over header + legend + body (821184 logical
// px). Gate mode keeps the deterministic script (probes fire once, Spike glow
// then Recover restore) and only freezes the HOT hue (60-tick ramp then hold),
// so the snapshot shows the restored base colours bit-stably. Manual mode
// keeps the animated HOT spot and does not assert pixels.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH = 1200, 800
	closeSecs  = 5
)

// probe is one scripted click: a point relative to a laid-out (Align-placed)
// target, in window logical coordinates, with the expected hit debug name.
// expectHit=false asserts the point must NOT hit (empty area / beyond clip).
type probe struct {
	name      string
	align     *rendering.RenderAlignBox
	anc       rendering.RenderObject // clip/transform container (nil for plain boxes)
	target    rendering.RenderObject
	dx, dy    float64
	expectHit bool
	done      bool
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: freeze HOT, snapshot + pixel/Golden evidence, exit 1 on fail")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = stay until window close)")
	flag.Parse()

	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "R13")
	} else if *autoOnly {
		secs, secsSet = closeSecs, true
	}
	gateMode := secsSet
	_, _, errFace := wrkit.EnsureUIFace()
	if errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui ui_wr_r13_hit — Hit ≡ 绘", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "R13 Hit ≡ 绘 — 点哪高亮哪", []string{
		"A/B  = 色块目标 (探针中心)",
		"C    = 圆角裁剪目标",
		"D    = 旋转30°目标 (反变换)",
		"E    = 裁剪外拒绝探针",
		"F/G  = 重叠取最上层",
		"H    = 空白区 → 无命中",
		"HOT  = 每帧动画热点",
		"ptr  = 实时指针点击同链路",
	})

	bodyX := shell.Body.Box.Offset().X
	bodyY := shell.Body.Box.Offset().Y

	var (
		probes []*probe
		colors = map[string]*rendering.RenderColorBox{} // hit target color refs
	)

	mk := func(name string, box *rendering.RenderColorBox, ax, ay float64, dx, dy float64) {
		ab := shell.Body.Align(box, ax, ay)
		probes = append(probes, &probe{name: name, align: ab, target: box, dx: dx, dy: dy, expectHit: true})
		colors[name] = box
	}

	// A: plain color-box target.
	targetA := rendering.NewRenderColorBox(60, 60, 0.85, 0.30, 0.28, 1)
	targetA.SetDebugName("target-red")
	mk("target-red", targetA, 0.04, 0.05, 30, 30)

	// B: second plain color-box target.
	targetB := rendering.NewRenderColorBox(60, 60, 0.28, 0.45, 0.88, 1)
	targetB.SetDebugName("target-blue")
	mk("target-blue", targetB, 0.04, 0.30, 30, 30)

	// C: rounded-clip target — hit must land on the child inside the clip.
	innerC := rendering.NewRenderColorBox(50, 50, 0.30, 0.80, 0.45, 1)
	innerC.SetDebugName("target-round")
	clipC := rendering.NewRenderClipRRect(innerC)
	clipC.FixedWidth, clipC.FixedHeight = 50, 50
	clipC.SetRadius(12)
	abC := shell.Body.Align(clipC, 0.16, 0.05)
	probes = append(probes, &probe{name: "target-round", align: abC, anc: clipC, target: innerC, dx: 25, dy: 25, expectHit: true})
	colors["target-round"] = innerC

	// D: rotated target — 30° about center; inverse-mapped center still hits.
	innerD := rendering.NewRenderColorBox(60, 60, 0.72, 0.35, 0.82, 1)
	innerD.SetDebugName("target-xf")
	xfD := rendering.NewRenderTransform(innerD)
	xfD.SetRotation(30.0 * 3.141592653589793 / 180.0)
	abD := shell.Body.Align(xfD, 0.16, 0.30)
	probes = append(probes, &probe{name: "target-xf", align: abD, anc: xfD, target: innerD, dx: 30, dy: 30, expectHit: true})
	colors["target-xf"] = innerD

	// E: clip with overflowing child — inside clip hits, beyond clip must NOT.
	innerE := rendering.NewRenderColorBox(70, 70, 0.90, 0.75, 0.30, 1)
	innerE.SetDebugName("target-clipped")
	clipE := rendering.NewRenderClipRRect(innerE)
	clipE.FixedWidth, clipE.FixedHeight = 40, 40
	abE := shell.Body.Align(clipE, 0.28, 0.05)
	probes = append(probes,
		&probe{name: "target-clipped", align: abE, anc: clipE, target: innerE, dx: 20, dy: 20, expectHit: true},
		// (50,50) is inside the 70x70 child but outside the 40x40 clip.
		&probe{name: "beyond-clip", align: abE, anc: clipE, target: innerE, dx: 50, dy: 50, expectHit: false},
	)
	colors["target-clipped"] = innerE

	// F/G: overlap — both at the same spot, top added last must win.
	bottomF := rendering.NewRenderColorBox(60, 60, 0.35, 0.65, 0.55, 1)
	bottomF.SetDebugName("target-bottom")
	topG := rendering.NewRenderColorBox(60, 60, 0.95, 0.85, 0.35, 1)
	topG.SetDebugName("target-top")
	abF := shell.Body.Align(bottomF, 0.28, 0.32)
	abG := shell.Body.Align(topG, 0.28, 0.32)
	probes = append(probes,
		&probe{name: "target-top", align: abG, target: topG, dx: 30, dy: 30, expectHit: true})
	_ = abF
	colors["target-top"] = topG
	colors["target-bottom"] = bottomF

	// H: empty area anchor — tiny transparent box; the probe point is 3px off
	// its corner so the point itself must hit nothing (root has no debug name).
	blankH := rendering.NewRenderColorBox(2, 2, 0, 0, 0, 0)
	blankH.SetDebugName("blank-anchor")
	abH := shell.Body.Align(blankH, 0.80, 0.75)
	probes = append(probes, &probe{name: "empty-area", align: abH, target: blankH, dx: 3, dy: 3, expectHit: false})

	// Static dense content: 4x4 color grid + 8 labels (U17).
	denseD := rendering.NewAbsoluteBox(330, 240)
	denseD.Background = &rendering.Color{R: 0.13, G: 0.15, B: 0.21, A: 1}
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			c := rendering.NewRenderColorBox(58, 30, 0.25+float64(i)*0.12, 0.4+float64(j)*0.1, 0.62, 1)
			denseD.Place(c, 10+float64(i)*66, 10+float64(j)*40)
		}
	}
	for i := 0; i < 8; i++ {
		row, col := i/4, i%4
		denseD.Place(wrkit.Label(fmt.Sprintf("lbl-%d", i), 10, 0.62, 0.72, 0.85), 10+float64(col)*66, 178+float64(row)*22)
	}
	shell.Body.Align(denseD, 0.55, 0.05)

	// HOT spot: dirties itself every frame (live paint, non-boundary) — keeps
	// the window ticking so fps is measurable; contributes no rerecord noise.
	hot := rendering.NewRenderColorBox(26, 26, 0.95, 0.3, 0.25, 1)
	shell.Body.Align(hot, 0.88, 0.05)
	shell.Body.Align(wrkit.Label("HOT", 10, 0.95, 0.7, 0.6), 0.88, 0.16)

	// Live hit banner inside the body: "HIT: <name> ok/total" + last pointer hit.
	hitBanner := wrkit.Label("HIT: --  0/0   ptr: --", 13, 0.95, 0.90, 0.55)
	shell.Body.Align(hitBanner, 0.04, 0.55)

	var (
		elapsed       float64
		scriptedOK    int
		scriptedHit   int
		spikeDone     bool
		lastPtrHit    string
		lastScriptHit string
		hotTicks      int64
	)

	highlight := func(c *rendering.RenderColorBox, name string) {
		if c == nil {
			return
		}
		c.R, c.G, c.B = 1.0, 1.0, 0.55
		c.MarkNeedsPaint()
		lastPtrHit = name
	}

	// Gate snapshot + Golden base live under testdata/ (committed evidence).
	snapDir := filepath.Join("examples", "ui_wr_r13_hit", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}
	snapPath := filepath.Join(snapDir, snapshotPNG)
	// Manual mode animates, so its frame is not evidence. Keep the gate
	// snapshot path empty there: otherwise a later manual run would overwrite
	// the committed gate PNG that the Golden baseline was seeded from.
	snapshotPath := snapPath
	if !gateMode {
		snapshotPath = ""
	}
	runFor := time.Duration(secs) * time.Second
	if !gateMode && *manualSeconds > 0 {
		runFor = time.Duration(*manualSeconds * float64(time.Second))
	}

	var app *embedder.PipelineApp
	app = embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor:       runFor,
		WarmUp:       true,
		SnapshotPath: snapshotPath,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "ui_wr_r13_hit: close (%s)\n", win.Backend())
			case platform.EventResize:
				// Responsive layout: re-lay the shell to the new window size
				// (standard wiring, same as ui_wr_r4b_multidamage / r18).
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			}
			if ev.Type == platform.EventPointer && ev.Pointer == platform.PointerDown {
				_, obj, _ := app.HitTestPointer(ev.X, ev.Y)
				name := rendering.HitDebugName(obj)
				if h, ok := colors[name]; ok {
					highlight(h, name)
				}
				fmt.Fprintf(os.Stderr, "ui_wr_r13_hit: ptr click @(%.0f,%.0f) hit=%q\n", ev.X, ev.Y, name)
			}
		},
	})
	// W6: full_paint correctness window — pin policy explicitly (global default is retained).
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)

	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		elapsed += dt

		phase := wrkit.PhaseSteady
		switch {
		case elapsed >= 2.0 && elapsed < 3.5:
			phase = "Spike"
		case elapsed >= 3.5:
			phase = wrkit.PhaseRecover
		}

		// Scripted probe injection (idempotent): every probe runs exactly once.
		if scriptedOK+scriptedHit < len(probes) {
			for _, p := range probes {
				if p.done {
					continue
				}
				off := p.target.Offset()
				ab := p.align.Offset()
				wx := bodyX + ab.X + off.X + p.dx
				wy := bodyY + ab.Y + off.Y + p.dy
				if p.anc != nil {
					ao := p.anc.Offset()
					wx += ao.X
					wy += ao.Y
				}
				_, obj, _ := app.HitTestPointer(wx, wy)
				name := rendering.HitDebugName(obj)
				if p.expectHit {
					scriptedHit++
					if name == p.name {
						scriptedOK++
						lastScriptHit = p.name
						if c := colors[p.name]; c != nil {
							highlight(c, p.name)
						}
					}
				} else if name == "" {
					scriptedOK++ // empty area / beyond clip must miss
				}
				p.done = true
			}
		}

		// Spike phase: highlighted targets glow (human-visible "point where
		// you click = what highlights"). Recover restores base colors.
		if phase == "Spike" && !spikeDone {
			spikeDone = true
			for _, c := range colors {
				c.R, c.G, c.B = 1.0, 0.75, 0.30
				c.MarkNeedsPaint()
			}
		}
		if phase == wrkit.PhaseRecover {
			for name, c := range colors {
				switch name {
				case "target-red":
					c.R, c.G, c.B = 0.85, 0.30, 0.28
				case "target-blue":
					c.R, c.G, c.B = 0.28, 0.45, 0.88
				case "target-round":
					c.R, c.G, c.B = 0.30, 0.80, 0.45
				case "target-xf":
					c.R, c.G, c.B = 0.72, 0.35, 0.82
				case "target-clipped":
					c.R, c.G, c.B = 0.90, 0.75, 0.30
				case "target-top":
					c.R, c.G, c.B = 0.95, 0.85, 0.35
				case "target-bottom":
					c.R, c.G, c.B = 0.35, 0.65, 0.55
				}
				c.MarkNeedsPaint()
			}
		}

		// HOT spot repaints itself every frame. Gate mode ramps the hue once
		// over 60 ticks to its frozen final value so the end-of-run snapshot
		// is bit-stable.
		if gateMode {
			if n := hotTicks; n <= 60 {
				t := float64(n) / 60
				if t > 1 {
					t = 1
				}
				hotHue := 0.5 * t
				hot.R, hot.G, hot.B = 0.85+0.1*hotHue, 0.25+0.4*(1-hotHue), 0.3+0.5*hotHue
			}
			hotTicks++
		} else {
			hotHue := float64(int(elapsed*8)%16) / 16
			hot.R, hot.G, hot.B = 0.85+0.1*hotHue, 0.25+0.4*(1-hotHue), 0.3+0.5*hotHue
		}
		hot.MarkNeedsPaint()

		hitBanner.SetText(fmt.Sprintf("HIT: %s  %d/%d   ptr: %s", lastScriptHit, scriptedOK, len(probes), lastPtrHit))
		hitBanner.MarkNeedsPaint()

		app.ScheduleFrame()
		proc.Sample()

		shell.NoteHUDTick(dt)
		gateOK := scriptedOK == len(probes) && scriptedHit >= 4
		shell.UpdateHUD("R13", phase, app, gateOK,
			fmt.Sprintf("hit=%d/%d", scriptedOK, len(probes)),
			fmt.Sprintf("ptr=%s t=%.1f", lastPtrHit, elapsed))
	}})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}
	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: run:", err)
		os.Exit(1)
	}
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	elapsedSec := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	presents := app.PresentCount()
	fpsInterval := 0.0
	if snap.AvgFrameIntervalMs > 1e-6 {
		fpsInterval = 1000.0 / snap.AvgFrameIntervalMs
	}

	// Pixel probes are anchored to the body panel origin; Align offsets are
	// ax*(parent-child) fractions. The gate snapshot shows the Recover state:
	// every hit target restored to its base colour after the Spike glow.
	bx, by := shell.Body.X, shell.Body.Y
	pixelChecks := []pixelCheck{
		{desc: "target_red", x: bx + 64, y: by + 60, want: [3]float64{0.85, 0.30, 0.28}, tol: 0.06},
		{desc: "target_blue", x: bx + 64, y: by + 209, want: [3]float64{0.28, 0.45, 0.88}, tol: 0.06},
		{desc: "target_round", x: bx + 162, y: by + 55, want: [3]float64{0.30, 0.80, 0.45}, tol: 0.06},
		{desc: "target_xf", x: bx + 167, y: by + 209, want: [3]float64{0.72, 0.35, 0.82}, tol: 0.06},
		{desc: "target_clipped", x: bx + 259, y: by + 50, want: [3]float64{0.90, 0.75, 0.30}, tol: 0.06},
		{desc: "beyond_clip_bg", x: bx + 289, y: by + 80, want: [3]float64{0.10, 0.11, 0.13}, tol: 0.05, kind: kindBG},
		{desc: "target_top", x: bx + 269, y: by + 221, want: [3]float64{0.95, 0.85, 0.35}, tol: 0.06},
		{desc: "dense_bg", x: bx + 430, y: by + 210, want: [3]float64{0.13, 0.15, 0.21}, tol: 0.05, kind: kindBG},
		{desc: "dense_00", x: bx + 355, y: by + 46, want: [3]float64{0.25, 0.40, 0.62}, tol: 0.06},
		{desc: "dense_33", x: bx + 553, y: by + 166, want: [3]float64{0.61, 0.70, 0.62}, tol: 0.06},
		{desc: "hot_spot", x: bx + 786, y: by + 45, want: [3]float64{0.90, 0.45, 0.55}, tol: 0.06},
		{desc: "body_bg", x: bx + 450, y: by + 640, want: [3]float64{0.10, 0.11, 0.13}, tol: 0.05, kind: kindBG},
		{desc: "legend_bg", x: shell.Legend.X + 20, y: shell.Legend.Y + 620, want: [3]float64{0.11, 0.12, 0.15}, tol: 0.05, kind: kindBG},
		{desc: "top_bg", x: 1000, y: 24, want: [3]float64{0.12, 0.14, 0.18}, tol: 0.05, kind: kindBG},
	}
	inkRegions := []inkRegion{
		{desc: "hit_banner_ink", x: bx + 33, y: by + 350, w: 260, h: 16, bg: [3]float64{0.10, 0.11, 0.13}, tol: 0.04, minPx: 20},
		{desc: "dense_labels_ink", x: bx + 326, y: by + 199, w: 240, h: 36, bg: [3]float64{0.13, 0.15, 0.21}, tol: 0.04, minPx: 20},
	}

	// Pixel + Golden evidence only makes sense for the frozen gate phase.
	// Manual mode intentionally animates; its evidence is human observation.
	var pixelOK int
	pixelBad := make([]string, 0, len(pixelChecks)+len(inkRegions))
	var goldenDiffPct float64
	var goldenTotalPx int64
	var goldenFirstRun bool
	pixelTotal := 0
	if gateMode {
		img := loadImage(snapPath)
		pixelResult := make(map[string]bool, len(pixelChecks)+len(inkRegions))
		runPixelChecks(img, pixelChecks, pixelResult)
		runInkRegions(img, inkRegions, pixelResult)
		for desc, ok := range pixelResult {
			if ok {
				pixelOK++
			} else {
				pixelBad = append(pixelBad, desc)
			}
		}
		sort.Strings(pixelBad)
		pixelTotal = len(pixelChecks) + len(inkRegions)

		goldenRects := []rect{
			{x: 0, y: 0, w: winW, h: 48},
			{x: shell.Legend.X, y: shell.Legend.Y, w: shell.Legend.W, h: shell.Legend.H},
			{x: shell.Body.X, y: shell.Body.Y, w: shell.Body.W, h: shell.Body.H},
		}
		goldenDiffPct, goldenTotalPx, goldenFirstRun = evaluateGolden(snapDir, goldenRects)
	}
	pixelGoldenValue := any(goldenDiffPct)
	if !gateMode {
		pixelGoldenValue = nil
	}

	logicProbes := []string{
		"presents_positive",
		"fps_interval_ge_55",
		"present_policy_full",
		"cpu_fallback_zero",
		"vsync_source_present",
		"scripted_ok_all",
		"scripted_hit_ge_4",
	}
	logicFailed := make([]string, 0, len(logicProbes))
	if presents < 1 {
		logicFailed = append(logicFailed, "presents_positive")
	}
	if fpsInterval < 55 {
		logicFailed = append(logicFailed, "fps_interval_ge_55")
	}
	if snap.PresentPolicy != scheduler.PresentPolicyFullPaint {
		logicFailed = append(logicFailed, "present_policy_full")
	}
	if snap.CPUFallbackOps != 0 {
		logicFailed = append(logicFailed, "cpu_fallback_zero")
	}
	if snap.VSyncSource == "" {
		logicFailed = append(logicFailed, "vsync_source_present")
	}
	if scriptedOK != len(probes) {
		logicFailed = append(logicFailed, "scripted_ok_all")
	}
	if scriptedHit < 4 {
		logicFailed = append(logicFailed, "scripted_hit_ge_4")
	}
	logicPass := len(logicFailed) == 0

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R13",
		Scenario:      "ui_wr_r13_hit",
		Backend:       win.Backend().String(),
		Snap:          snap,
		PresentCount:  presents,
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"scripted_total":         len(probes),
			"scripted_ok":            scriptedOK,
			"scripted_hit":           scriptedHit,
			"scripted_miss":          len(probes) - scriptedOK,
			"probe_kinds":            "plain_box,plain_box,rounded_clip,rotated_xf,clip_inside,clip_outside,overlap_top,empty_area",
			"targets":                len(colors),
			"hit_via_engine":         "PipelineApp.HitTestPointer",
			"real_pointer_path":      "X11 EventPointer+PointerDown → HitTestPointer",
			"static_dense_cells":     16,
			"static_labels":          8,
			"phase_mode":             map[bool]string{true: "script-frozen-hot", false: "animated"}[gateMode],
			"evidence_mode":          map[bool]string{true: "gate", false: "manual"}[gateMode],
			"pixel_evidence":         map[bool]string{true: "gate_snapshot", false: "manual_not_asserted"}[gateMode],
			"fps_gate":               "persistent_tick_ge_55",
			"slope_gate":             "off_5s_correctness",
			"rss_slope_note":         "rss_slope_valid=false for 5s < rss_warmup_sec; slope_gate=off per §2.2.4",
			"golden_mask":            "header 1200x48 + legend 260x656 + body 904x656",
			"golden_tolerance_pct":   0,
			"logic_probes_ok":        len(logicProbes) - len(logicFailed),
			"logic_probes_total":     len(logicProbes),
			"logic_bad":              logicFailed,
			"pixel_probes_ok":        pixelOK,
			"pixel_probes_total":     pixelTotal,
			"pixel_bad":              pixelBad,
			"pixel_golden_diff_pct":  pixelGoldenValue,
			"pixel_golden_total_px":  goldenTotalPx,
			"pixel_golden_first_run": goldenFirstRun,
			"snapshot_png":           snapshotPath,
			"snapshot_note":          map[bool]string{true: "gate snapshot (GPU readback) used for pixel/Golden evidence", false: "manual mode does not write the gate snapshot; committed evidence is not overwritten"}[gateMode],
			"impl_correctness":       "scripted probes hit exactly the drawn target under plain boxes, rounded clip, 30° rotation and overlap; beyond-clip and empty-area probes miss",
			"impl_dirty":             "gate: same elapsed script (probes once, Spike glow 2–3.5s, Recover restore), only HOT hue frozen; manual: HOT hue cycles",
			"impl_cache":             "hit testing walks the same paint tree identity (HitDebugName); no cache involvement beyond paint identity",
			"impl_edge":              "8 probes across 6 target kinds + dense 4×4 grid + 8 labels; real pointer clicks share HitTestPointer",
			"impl_fail":              "scripted_ok != total (hit ≠ paint), scripted_hit < 4, pixel probe mismatch, golden diff!=0, or policy!=full_paint → FAIL",
			"impl_visible":           "click highlights the hit target (yellow flash); hit banner shows ok/total; gate pixel/Golden evidence over the restored state",
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	fail := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "FAIL: "+format+"\n", args...)
		if gateMode {
			os.Exit(1)
		}
	}
	if !logicPass {
		fail("logic probes %d/%d bad=%v", len(logicProbes)-len(logicFailed), len(logicProbes), logicFailed)
	}
	if gateMode {
		if pixelOK != pixelTotal {
			fail("pixel probes %d/%d ok bad=%v", pixelOK, pixelTotal, pixelBad)
		}
		if !goldenFirstRun && goldenDiffPct != 0 {
			fail("pixel_golden_diff_pct=%.4f want 0 over %d px", goldenDiffPct, goldenTotalPx)
		}
		// R13 gates (§2 主表: scripted_ok == scripted_total, all probes hit).
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
			MinPresents:            1,
			RequireFullPaintPolicy: true,
			RequirePersistentFPS:   true,
		}); err != nil {
			fail("%v", err)
		}
	}

	status := "OK"
	if !logicPass {
		status = "WARN"
	}
	if gateMode && goldenFirstRun {
		status = "SEEDED"
	}
	evidence := ""
	if gateMode {
		evidence = fmt.Sprintf(" pixel=%d/%d golden=%.4f%%", pixelOK, pixelTotal, goldenDiffPct)
	}
	fmt.Fprintf(os.Stderr,
		"ui_wr_r13_hit: %s probes=%d ok=%d hit=%d targets=%d logic=%d/%d%s mode=%s elapsed=%.1fs\n",
		status, len(probes), scriptedOK, scriptedHit, len(colors),
		len(logicProbes)-len(logicFailed), len(logicProbes), evidence,
		map[bool]string{true: "gate", false: "manual"}[gateMode], elapsedSec)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
