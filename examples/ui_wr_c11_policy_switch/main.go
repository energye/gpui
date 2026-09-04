// Command ui_wr_c11_policy_switch is the W6 C11 combined real window:
// R0→R4 策略切换 — full_paint↔retained 热切换 + 静态内容不丢失。
//
// §3 C11 thesis: 策略热切换正确 — Steady(full_paint 0–5s) → Spike(retained
// 5–10s) → Recover(full_paint 10–15s)，两次切换后静态面板像素不变、policy
// 字段如实跟随、retained 相损伤远小于全屏、切换无闪烁（fps 不塌）。
//
// Gates (§3 C11 行 + §2.5 层 Present/多 damage 档): policy_switches==2,
// retained 相 damage_ratio_avg≤0.35 且小于 full 相均值, full/damage 两态
// present_mode 均观测到, fps_interval≥55, p95≤22ms, hitch≤5/min, CPU 非双 0,
// 像素断言 3/3, Golden 静态掩码零容差（次跑起）。
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=15 go run ./examples/ui_wr_c11_policy_switch
//
// Window: 1200x800. RUN_SECONDS>=5 (U16); closing requires 15 (§2.5 层
// Present 档). No RUN_SECONDS → interactive loop (pixel evidence off).
package main

import (
	"encoding/json"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
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
	winW, winH   = 1200, 800
	closeSeconds = 15 // §2.5 层 Present 档：C11 = 15s

	hitchBudget = 5.0
	damageCap   = 0.35 // retained 相损伤上限（远小于全屏）
)

// Palette shared by scene builder AND pixel assertions (single source of truth).
var (
	clearBG   = [3]float64{0.08, 0.09, 0.11}
	denseBase = [3]float64{0.11, 0.12, 0.18}
	denseCell = [3]float64{0.37, 0.50, 0.72}
)

type probePoint struct {
	x, y float64
	want [3]float64
	tol  float64
}

type rect struct{ x, y, w, h float64 }

type pixelCheck struct {
	pt   *probePoint
	text *rect // F6: text density is a REGION property
	base [3]float64
	desc string
}

var pixelResult = map[string]bool{}

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "C11")
	} else {
		secs = closeSeconds
	}
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: "gpui ui_wr_c11_policy_switch — R0→R4 策略切换", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "C11 R0→R4 策略切换 — 切后静态不丢", []string{
		"Steady  full_paint：全树重画 基线",
		"Spike   retained：只画脏区 损伤≪全屏",
		"Recover full_paint：切回 静态仍在",
		"DENSE   4×4 静态色格 + 8 标签（跨切换探针）",
		"HOT     每帧变色动态热点（切换无闪烁）",
		"policy 字段跟随切换 + 切换计数",
		"损伤分相统计：retained 相 < full 相",
		"切换后静态像素不变（Golden 掩码）",
	})

	body := shell.Body

	// ===== DENSE: static panel probed across both switches ================
	dense := rendering.NewAbsoluteBox(300, 430)
	dense.Background = &rendering.Color{R: denseBase[0], G: denseBase[1], B: denseBase[2], A: 1}
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			c := rendering.NewRenderColorBox(64, 48,
				denseCell[0]*(1-0.08*float64(i)), denseCell[1]*(0.8+0.1*float64(j)), denseCell[2], 1)
			c.SetRepaintBoundary(true)
			dense.Place(c, 12+float64(i)*70, 12+float64(j)*56)
		}
	}
	for i := 0; i < 8; i++ {
		row, col := i/4, i%4
		lbl := wrkit.Label(fmt.Sprintf("c11-%d", i), 10, 0.62, 0.72, 0.85)
		dense.Place(lbl, 12+float64(col)*70, 248+float64(row)*20)
	}
	denseNote := wrkit.Label("静态缓存：切换前后像素不变", 11, 0.55, 0.75, 0.95)
	dense.Place(denseNote, 12, 300)
	body.Box.Place(dense, 40, 30)

	// ===== HOT spot: self-dirtying animation (switch must not flicker) =====
	// Own boundary: without it the per-frame recolor dirties the root layer
	// and every retained frame degrades to full-surface damage (the §2.1
	// contrast this window proves would collapse).
	hot := rendering.NewRenderColorBox(30, 30, 0.95, 0.3, 0.25, 1)
	hot.SetRepaintBoundary(true)
	body.Box.Place(hot, 420, 120)
	hotLbl := wrkit.Label("HOT 每帧变色（切换无闪烁）", 11, 0.95, 0.7, 0.6)
	body.Box.Place(hotLbl, 460, 124)

	// ===== Policy banner (HUD-visible switch counter) =====================
	// Own boundary for the same reason as hot: 2Hz re-text must stay a small
	// damage rect, never a root-layer full-surface invalidation.
	policyBanner := wrkit.Label("POLICY: full_paint switches=0", 13, 0.95, 0.90, 0.55)
	policyBanner.SetRepaintBoundary(true)
	body.Box.Place(policyBanner, 420, 200)
	phaseLabel := wrkit.Label("PHASE: STEADY", 13, 0.95, 0.95, 0.4)
	phaseLabel.SetRepaintBoundary(true)
	body.Box.Place(phaseLabel, 420, 240)
	staticNote := wrkit.Label("切换只改提交路径 不改内容：静区零重录", 11, 0.55, 0.75, 0.95)
	body.Box.Place(staticNote, 420, 280)

	// ===== Probe geometry (resolved from the layout chain; §2.7 rule 2) ====
	ptDenseCell := probePoint{
		want: [3]float64{denseCell[0], denseCell[1] * 0.8, denseCell[2]}, tol: 8.0 / 255,
	}
	capTextBox := rect{}
	var goldenRects []rect
	resolveGeometry := func() {
		bodyX, bodyY := body.Box.Offset().X, body.Box.Offset().Y
		dx := bodyX + dense.Offset().X
		ptDenseCell.x = dx + 12 + 32
		ptDenseCell.y = bodyY + dense.Offset().Y + 12 + 24
		capTextBox = rect{x: dx + 12, y: bodyY + dense.Offset().Y + 300, w: 270, h: 28}
		// Golden static mask: time-invariant BY DESIGN. Excluded: HUD band,
		// policy banner (switch counter text), phase label, HOT spot.
		goldenRects = []rect{
			{x: 0, y: 0, w: winW, h: 48},
			{shell.Legend.X, shell.Legend.Y, shell.Legend.W, shell.Legend.H},
			{dx - 2, bodyY + dense.Offset().Y - 2, 306, 436},
			{x: dx + 340, y: bodyY + 300, w: 220, h: 130}, // empty body-bg strip
		}
	}

	snapDir := os.Getenv("C11_SNAP_DIR")
	if snapDir == "" {
		snapDir = "/tmp/c11_policy_switch"
	}
	os.MkdirAll(snapDir, 0o755)

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: clearBG[0], ClearG: clearBG[1], ClearB: clearBG[2], ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		SnapshotPath: func() string {
			if secsSet {
				return filepath.Join(snapDir, "c11_final.png")
			}
			return ""
		}(),
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintf(os.Stderr, "ui_wr_c11_policy_switch: close (%s)\n", win.Backend())
			}
			if ev.Type == platform.EventResize && ev.Width > 0 && ev.Height > 0 {
				shell.Resize(float64(ev.Width), float64(ev.Height))
			}
		},
	})
	// C11 starts on full_paint (R0 posture) and switches per phase below.
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)

	clock := wrkit.NewPhaseClock(5, 10) // Steady 5s → Spike 5s → Recover
	var hotHue float64
	var bannerAccum float64
	lastPhase := ""
	switches := 0
	// Per-phase damage accounting: DamageStats is cumulative, so latch the
	// counters at each switch boundary to derive per-phase averages.
	var fullSum, fullN, retSum, retN int64
	latchSum, latchN := int64(0), int64(0)
	seenFullMode := false
	seenDamageMode := false

	latch := func() {
		sum, _, n, _ := app.DamageStats()
		latchSum, latchN = sum, n
	}
	accumulate := func(policy string) {
		sum, _, n, _ := app.DamageStats()
		if policy == scheduler.PresentPolicyRetained {
			retSum += sum - latchSum
			retN += n - latchN
		} else {
			fullSum += sum - latchSum
			fullN += n - latchN
		}
		latch()
	}

	app.Scheduler().Tickers().Add(&tickerT{on: func(dt float64) {
		phase := clock.Advance(dt)
		wantPolicy := scheduler.PresentPolicyFullPaint
		if phase == wrkit.PhaseSpike {
			wantPolicy = scheduler.PresentPolicyRetained
		}
		if phase != lastPhase {
			if lastPhase != "" {
				// Phase boundary = policy switch point: close out the old
				// phase's damage account, then switch.
				accumulate(app.Metrics().Snapshot().PresentPolicy)
				switches++
			} else {
				latch()
			}
			lastPhase = phase
			app.SetPresentPolicy(wantPolicy)
			switch phase {
			case wrkit.PhaseSteady:
				phaseLabel.SetText("PHASE: STEADY")
			case wrkit.PhaseSpike:
				phaseLabel.SetText("PHASE: SPIKE")
			default:
				phaseLabel.SetText("PHASE: RECOVER")
			}
			phaseLabel.MarkNeedsPaint()
		}
		if m := app.LastPresentMode(); m == "full" {
			seenFullMode = true
		} else if m == "damage_union" || m == "damage_multi" {
			seenDamageMode = true
		}
		hotHue = math.Mod(hotHue+dt*0.6, 1.0)
		hot.R = 0.85 + 0.1*hotHue
		hot.G = 0.25 + 0.4*(1-hotHue)
		hot.B = 0.3 + 0.5*hotHue
		hot.MarkNeedsPaint()

		bannerAccum += dt
		if bannerAccum >= 0.5 {
			bannerAccum = 0
			policyBanner.SetText(fmt.Sprintf("POLICY: %s switches=%d", app.Metrics().Snapshot().PresentPolicy, switches))
			policyBanner.MarkNeedsPaint()
		}

		pcSnap := app.Metrics().Snapshot()
		gateOK := pcSnap.BoundarySkip > 0 && switches <= 2
		shell.NoteHUDTick(dt)
		shell.UpdateHUD("C11", phase, app, gateOK,
			fmt.Sprintf("pol=%s sw=%d", pcSnap.PresentPolicy, switches),
			fmt.Sprintf("mode=%s skip=%d", app.LastPresentMode(), pcSnap.BoundarySkip))
		app.ScheduleFrame()
		proc.Sample()
	}})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: open: %v\n", err)
		os.Exit(1)
	}
	resolveGeometry()
	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: run: %v\n", err)
		os.Exit(1)
	}
	// Close out the final (Recover) phase account.
	accumulate(app.Metrics().Snapshot().PresentPolicy)
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	elapsedSec := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	wrkit.MergeBoundaryCache(app, &snap)

	var fullAvg, retAvg float64
	if fullN > 0 {
		fullAvg = float64(fullSum) / float64(fullN) / float64(winW*winH)
	}
	if retN > 0 {
		retAvg = float64(retSum) / float64(retN) / float64(winW*winH)
	}
	// ---- Final-frame pixel assertions (§2.7/U21) --------------------------
	// A switch that loses static content leaves it lost: clean boundaries
	// never re-record in retained mode, so the final snapshot + golden catch
	// either switch direction going wrong (absolute colors, not deltas).
	finalChecks := []pixelCheck{
		{pt: &ptDenseCell, desc: "static_cell_survives_switches"},
		{text: &capTextBox, base: denseBase, desc: "static_note_text_density"},
	}
	runPixelChecks(loadImage(filepath.Join(snapDir, "c11_final.png")), finalChecks, pixelResult)
	scriptedOK, scriptedTotal := 0, len(pixelResult)
	for _, ok := range pixelResult {
		if ok {
			scriptedOK++
		}
	}

	goldenDiffPct, goldenTotalPx, goldenFirstRun := evaluateGolden(snapDir, goldenRects)

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "C11",
		Scenario:      "ui_wr_c11_policy_switch",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"covers":                    []string{"R0", "R4"},
			"policy_switches":           switches,
			"full_phase_damage_avg":     fullAvg,
			"retained_phase_damage_avg": retAvg,
			"seen_full_mode":            seenFullMode,
			"seen_damage_mode":          seenDamageMode,
			"hitch_budget_per_min":      hitchBudget,
			"scripted_ok":               scriptedOK,
			"scripted_total":            scriptedTotal,
			"pixel_golden_diff_pct":     goldenDiffPct,
			"pixel_golden_total_px":     goldenTotalPx,
			"pixel_golden_first_run":    goldenFirstRun,
			"covered":                   "R0 full_paint baseline + R4 retained damage + hot policy switch both ways",
			"impl_correctness":          "switching only changes the submit path — the static panel replays its cache in both policies, so pixels never move (final probes + golden)",
			"impl_dirty":                "retained phase re-records only hot + banner; full phases repaint the tree; per-phase damage accounts prove the contrast",
			"impl_cache":                "texture/boundary caches persist across switches — no cold re-record storm after switching back",
			"impl_interaction":          "the same frame loop drives R0 full-paint correctness and R4 retained damage; the policy flag is the only variable, so any static loss isolates to the switch itself",
			"impl_edge":                 "switches happen exactly at phase boundaries; final Recover phase closes its own damage account before sampling",
			"impl_fail":                 "lost static (drift/golden), missing switch, retained damage ≥ full damage, or fps collapse all trip gates → FAIL exit 1",
			"impl_visible":              "HUD shows live policy/switch/mode; banner tracks the switch count; dense panel looks perfectly still through both switches",
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	// ---- Gates (§3 C11 行 + §2.5 层 Present 档 + §2.2 全族；硬，不许放) ----
	if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
		MinPresents:          1,
		RequirePersistentFPS: true,
		MinFPSWall:           55,
		MaxP95Ms:             22,
		MinBoundarySkip:      1,
		MinBoundaryRerecord:  1,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: %v\n", err)
		os.Exit(1)
	}
	if snap.VSyncSource == "" {
		fmt.Fprintln(os.Stderr, "FAIL: vsync_source missing")
		os.Exit(1)
	}
	if secsSet && secs < closeSeconds {
		fmt.Fprintf(os.Stderr, "FAIL: RUN_SECONDS=%d want >=%d (§2.5 层 Present 档关闭用时长)\n", secs, closeSeconds)
		os.Exit(1)
	}
	// 能力专用①：两次切换必须真实发生。
	if switches != 2 {
		fmt.Fprintf(os.Stderr, "FAIL: policy_switches=%d want 2 (Steady→Spike→Recover 各一次切换)", switches)
		os.Exit(1)
	}
	// 能力专用②：两态提交路径均观测到。
	if !seenFullMode || !seenDamageMode {
		fmt.Fprintf(os.Stderr, "FAIL: seen_full=%v seen_damage=%v want both true (两态路径缺一不可)", seenFullMode, seenDamageMode)
		os.Exit(1)
	}
	// 能力专用③：retained 相损伤远小于全屏且小于 full 相。
	if retN == 0 || retAvg > damageCap {
		fmt.Fprintf(os.Stderr, "FAIL: retained_damage_avg=%.3f (n=%d) want ≤%.2f", retAvg, retN, damageCap)
		os.Exit(1)
	}
	if fullN > 0 && !(retAvg < fullAvg) {
		fmt.Fprintf(os.Stderr, "FAIL: retained_avg=%.3f not < full_avg=%.3f (切换未改变提交行为)", retAvg, fullAvg)
		os.Exit(1)
	}
	// 能力专用④：切换后静态不丢（快照绝对色探针 + Golden，见上）。
	// 族 A：持续 tick 60fps 档。
	if fpsOf(snap) < 55 {
		fmt.Fprintf(os.Stderr, "FAIL: fps_interval=%.1f < 55 (持续 tick C11)", fpsOf(snap))
		os.Exit(1)
	}
	if snap.HitchRatePerMin > hitchBudget {
		fmt.Fprintf(os.Stderr, "FAIL: hitch_rate_per_min=%.2f > %.0f\n", snap.HitchRatePerMin, hitchBudget)
		os.Exit(1)
	}
	// 族 D：CPU 非双 0（15s 窗只必采不断言上限，见 §2.2.3）。
	if snap.CPUPctAvg <= 0 || (snap.CPUUIPct <= 0 && snap.CPURasterPct <= 0) {
		fmt.Fprintln(os.Stderr, "FAIL: cpu 双 0 装绿嫌疑")
		os.Exit(1)
	}
	// 像素断言全过。
	if scriptedTotal != 2 || scriptedOK != scriptedTotal {
		fmt.Fprintf(os.Stderr, "FAIL: scripted=%d/%d want 2/2 (像素断言)\n", scriptedOK, scriptedTotal)
		os.Exit(1)
	}
	// Golden 零容差（首跑产基线；次跑起逐位一致）。
	if !goldenFirstRun && goldenDiffPct != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: pixel_golden_diff_pct=%.4f want 0 over %d px\n", goldenDiffPct, goldenTotalPx)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "ui_wr_c11_policy_switch: OK presents=%d fps=%.1f p95=%.1f hitch=%.2f/min sw=%d full_dmg=%.3f ret_dmg=%.3f skip=%d scripted=%d/%d golden=%.4f%%(%dpx first=%v) vsync=%s elapsed=%.1fs\n",
		app.PresentCount(), fpsOf(snap), snap.P95FrameIntervalMs, snap.HitchRatePerMin,
		switches, fullAvg, retAvg,
		snap.BoundarySkip, scriptedOK, scriptedTotal, goldenDiffPct, goldenTotalPx, goldenFirstRun,
		snap.VSyncSource, elapsedSec)
}

// --- assertion helpers -----------------------------------------------------

func runPixelChecks(img image.Image, checks []pixelCheck, result map[string]bool) {
	dpr := 1.0
	if img != nil {
		dpr = float64(img.Bounds().Dx()) / winW
	}
	for _, c := range checks {
		var ok bool
		if c.pt != nil {
			r, g, b, valid := sampleLogical(img, dpr, c.pt.x, c.pt.y)
			ok = valid && nearC(r, g, b, c.pt.want, c.pt.tol)
			fmt.Fprintf(os.Stderr, "ui_wr_c11_policy_switch: pixel %-32s @(%4.0f,%4.0f) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) ok=%v\n",
				c.desc, c.pt.x, c.pt.y, r, g, b, c.pt.want[0], c.pt.want[1], c.pt.want[2], ok)
		} else if c.text != nil {
			n := textPixels(img, dpr, *c.text, c.base)
			ok = img != nil && n >= 120
			fmt.Fprintf(os.Stderr, "ui_wr_c11_policy_switch: pixel %-32s text_px=%d (want >=120) ok=%v\n", c.desc, n, ok)
		}
		result[c.desc] = ok
	}
}

func loadImage(path string) image.Image {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pixel checks: %v\n", err)
		return nil
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pixel checks: decode %s: %v\n", path, err)
		return nil
	}
	return img
}

func sampleLogical(img image.Image, dpr float64, lx, ly float64) (r, g, b float64, valid bool) {
	if img == nil {
		return 0, 0, 0, false
	}
	px, py := int(lx*dpr), int(ly*dpr)
	if px < 0 || py < 0 || px >= img.Bounds().Dx() || py >= img.Bounds().Dy() {
		return 0, 0, 0, false
	}
	r32, g32, b32, _ := img.At(px, py).RGBA()
	return float64(r32>>8) / 255, float64(g32>>8) / 255, float64(b32>>8) / 255, true
}

func nearC(r, g, b float64, want [3]float64, tol float64) bool {
	return math.Abs(r-want[0]) <= tol && math.Abs(g-want[1]) <= tol && math.Abs(b-want[2]) <= tol
}

func textPixels(img image.Image, dpr float64, box rect, base [3]float64) int {
	if img == nil {
		return 0
	}
	x0, y0 := int(box.x*dpr), int(box.y*dpr)
	x1, y1 := int((box.x+box.w)*dpr), int((box.y+box.h)*dpr)
	n := 0
	for py := y0; py < y1 && py < img.Bounds().Dy(); py++ {
		for px := x0; px < x1 && px < img.Bounds().Dx(); px++ {
			r32, g32, b32, _ := img.At(px, py).RGBA()
			r, g, b := float64(r32>>8)/255, float64(g32>>8)/255, float64(b32>>8)/255
			if math.Abs(r-base[0]) > 24.0/255 || math.Abs(g-base[1]) > 24.0/255 || math.Abs(b-base[2]) > 24.0/255 {
				n++
			}
		}
	}
	return n
}

func evaluateGolden(snapDir string, rects []rect) (diffPct float64, totalPx int64, firstRun bool) {
	cur := filepath.Join(snapDir, "c11_final.png")
	base := filepath.Join(snapDir, "c11_final_base.png")
	if _, err := os.Stat(base); err != nil {
		data, err := os.ReadFile(cur)
		if err != nil {
			fmt.Fprintf(os.Stderr, "golden: current snapshot %s missing (%v)\n", cur, err)
			return 100, 0, false
		}
		if err := os.WriteFile(base, data, 0o644); err == nil {
			fmt.Fprintf(os.Stderr, "golden baseline stored: %s\n", base)
			return 0, 0, true
		}
		return 100, 0, false
	}
	diff, total, err := comparePNG(base, cur, rects)
	if err != nil {
		fmt.Fprintf(os.Stderr, "golden compare: %v\n", err)
		return 100, 0, false
	}
	fmt.Fprintf(os.Stderr, "golden %s: diff=%.4f%% over %d px\n", filepath.Base(cur), diff, total)
	return diff, total, false
}

func comparePNG(basePath, curPath string, rects []rect) (pct float64, total int64, err error) {
	a, b := loadImage(basePath), loadImage(curPath)
	if a == nil || b == nil {
		return 0, 0, fmt.Errorf("decode failed")
	}
	if a.Bounds() != b.Bounds() {
		return 0, 0, fmt.Errorf("size mismatch %v vs %v", a.Bounds(), b.Bounds())
	}
	dpr := float64(a.Bounds().Dx()) / winW
	var diff int64
	for _, r := range rects {
		x0, y0 := int(r.x*dpr), int(r.y*dpr)
		x1, y1 := int((r.x+r.w)*dpr), int((r.y+r.h)*dpr)
		for py := y0; py < y1 && py < a.Bounds().Dy(); py++ {
			for px := x0; px < x1 && px < a.Bounds().Dx(); px++ {
				ar, ag, ab, _ := a.At(px, py).RGBA()
				br, bg, bb, _ := b.At(px, py).RGBA()
				if ar != br || ag != bg || ab != bb {
					diff++
				}
				total++
			}
		}
	}
	if total == 0 {
		return 0, 0, nil
	}
	return float64(diff) / float64(total) * 100, total, nil
}

func fpsOf(snap scheduler.FrameMetrics) float64 {
	if snap.AvgFrameIntervalMs > 1e-6 {
		return 1000.0 / snap.AvgFrameIntervalMs
	}
	return 0
}

type tickerT struct{ on func(dt float64) }

func (t *tickerT) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
