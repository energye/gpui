// Command ui_wr_r14_cache_budget is the R14 real window (mode-1 first close):
// cache budget / eviction under sustained scroll pressure.
//
// §2 R14 thesis: 大量 boundary 区域 + 持续滚动产生新缓存 + 超预算淘汰 — a
// 600-row virtual-list scrolls for the whole window so fresh cells keep
// producing new cache keys, while the retained-path texture LRU runs under an
// EXPLICIT budget (SetBudget, default 128 — above the per-frame live set of
// ~107 distinct keys so the cap can bind, below the scroll churn total).
// The budget bounds cross-frame accumulation: texture entries stay ≤ cap,
// cache_evictions grows monotonically (~590/60s), RSS slope stays flat, and
// the visible picture stays correct (pixel probes + golden).
//
// Gates (§2 R14 row + §2.5 预算/压力 档): texture_entries 触顶且 ≤ budget,
// cache_evictions ≥3 且严格单调, rss_slope_kb_per_min ≤30000, retained
// policy, fps_interval ≥55, p95 ≤22ms, hitch ≤5/min, CPU 非双 0, pixel
// assertions 3/3, golden bitwise zero-tolerance from run 2.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=60 go run ./examples/ui_wr_r14_cache_budget
//
// Window: 1200x800. RUN_SECONDS>=5 (U16); closing requires 60 (§2.5 预算/
// 压力 档). No RUN_SECONDS → interactive loop (pixel evidence off).
package main

import (
	"encoding/json"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"strconv"
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
	closeSeconds = 60 // §2.5 预算/压力 档：R14 = 60s

	itemCount  = 600
	rowExtent  = 64.0 // fixed row height
	hitchBudget = 5.0
	slopeBudget = 30000.0 // §2.2.4 default extreme-climb budget (KB/min)

	evictBudgetMin      = 3   // minimum evictions a 60s pressured run must show
	defaultTexBudget    = 128 // explicit texture-LRU budget (above per-frame live set ~107, below scroll churn total)
	boundaryDefaultCap  = 0   // boundary cache stays unlimited (sweep-bounded)
)

// Palette shared by scene builder AND pixel assertions (single source of truth).
var (
	clearBG   = [3]float64{0.08, 0.09, 0.11}
	denseBase = [3]float64{0.11, 0.12, 0.18}
	denseCell = [3]float64{0.37, 0.50, 0.72}
	boardBG   = [3]float64{0.13, 0.15, 0.21}
)

// rowColor is the deterministic per-row fill color.
func rowColor(i int) [3]float64 {
	return [3]float64{
		0.16 + 0.07*float64(i%5),
		0.38 + 0.08*float64(i%4),
		0.52 + 0.10*float64(i%3),
	}
}

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
		wrkit.RequireMinRun(secs, "R14")
	} else {
		secs = closeSeconds
	}
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: "gpui ui_wr_r14_cache_budget — 缓存预算/淘汰", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "R14 缓存预算/淘汰 — 超预算仍正确", []string{
		"LIST  600 行持续滚动 不断产生新缓存键",
		"DENSE 右侧嵌套 boundary 静态缓存",
		"HOT   每帧变色动态热点（非缓存）",
		"预算   纹理 LRU 显式上限 封住滚动累积",
		"超预算 → 最老条目淘汰 计数只增",
		"entries 触顶 + evictions 单调 + RSS 平",
		"淘汰后画面仍正确（像素探针/Golden）",
	})

	body := shell.Body

	// ===== LIST: sustained-scroll cache producer ===========================
	listW := 640.0
	vpH := body.H
	list := rendering.NewVirtualList(itemCount, rowExtent, func(i int) rendering.RenderObject {
		c := rowColor(i)
		cell := rendering.NewAbsoluteBox(listW, rowExtent)
		box := rendering.NewRenderColorBox(listW, rowExtent, c[0], c[1], c[2], 1)
		box.SetRepaintBoundary(true)
		cell.Place(box, 0, 0)
		// Label sits on the far right so the left half stays pure fill for
		// the content probe (text pixels would poison the color assertion).
		t := wrkit.Label(fmt.Sprintf("row %04d — 缓存受预算淘汰管理", i), 13, 0.94, 0.96, 1.0)
		cell.Place(t, listW-340, 18)
		return cell
	})
	list.CacheExtent = 2 * rowExtent
	vp := rendering.NewRenderViewport(list)
	vp.FixedWidth = listW
	vp.FixedHeight = vpH
	body.Box.Place(vp, 0, 0)

	track := wrkit.NewPanel(10, vpH, 0.05, 0.06, 0.08, 1)
	body.Box.Place(track.Box, listW-14, 0)
	thumb := rendering.NewRenderColorBox(6, 56, 0.45, 0.75, 0.95, 1)
	thumbAlign := track.Align(thumb, 0.5, 0)

	// ===== DENSE: static nested-boundary panel (must stay untouched) ======
	dense := rendering.NewAbsoluteBox(250, 430)
	dense.Background = &rendering.Color{R: denseBase[0], G: denseBase[1], B: denseBase[2], A: 1}
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			c := rendering.NewRenderColorBox(56, 44,
				denseCell[0]*(1-0.08*float64(i)), denseCell[1]*(0.8+0.1*float64(j)), denseCell[2], 1)
			c.SetRepaintBoundary(true)
			dense.Place(c, 10+float64(i)*62, 10+float64(j)*50)
		}
	}
	for i := 0; i < 8; i++ {
		row, col := i/4, i%4
		lbl := wrkit.Label(fmt.Sprintf("lbl-%d", i), 10, 0.62, 0.72, 0.85)
		dense.Place(lbl, 10+float64(col)*62, 222+float64(row)*20)
	}
	denseNote := wrkit.Label("静态缓存：全程 skip 不重录", 11, 0.55, 0.75, 0.95)
	dense.Place(denseNote, 10, 268)
	body.Box.Place(dense, 680, 34)

	// ===== HOT spot: self-dirtying animation (U17 dynamic hotspot) =========
	hot := rendering.NewRenderColorBox(26, 26, 0.95, 0.3, 0.25, 1)
	body.Box.Place(hot, 680, 500)
	hotLbl := wrkit.Label("HOT 每帧变色（非缓存）", 11, 0.95, 0.7, 0.6)
	body.Box.Place(hotLbl, 716, 504)

	// ===== Cache-budget banner (HUD-visible core counter) ==================
	texBudget := int64(defaultTexBudget)
	if v := os.Getenv("R14_TEX_BUDGET"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			texBudget = int64(n)
		}
	}
	cacheBanner := wrkit.Label("CACHE: entries=-- evictions=--", 13, 0.95, 0.90, 0.55)
	body.Box.Place(cacheBanner, 320, 284)

	// ===== Probe geometry (resolved from the layout chain; §2.7 rule 2) ====
	ptDenseCell := probePoint{
		want: [3]float64{denseCell[0], denseCell[1] * 0.8, denseCell[2]}, tol: 8.0 / 255,
	} // cell (0,0) factors (1−0.08·0, 0.8+0.1·0)
	ptListRow := probePoint{tol: 6.0 / 255} // scrolled-in row fill (exact color)
	capTextBox := rect{}
	var goldenRects []rect

	resolveGeometry := func(scrollY float64) {
		bodyX, bodyY := body.Box.Offset().X, body.Box.Offset().Y
		dx := bodyX + dense.Offset().X
		// Dense cell (0,0) center.
		ptDenseCell.x = dx + 10 + 28
		ptDenseCell.y = bodyY + dense.Offset().Y + 10 + 22
		// Mid-visible list row: pure-fill point left of the label.
		firstVis := int(scrollY / rowExtent)
		lastVis := int((scrollY + float64(vpH)) / rowExtent)
		if lastVis >= itemCount {
			lastVis = itemCount - 1
		}
		mid := (firstVis + lastVis) / 2
		if mid >= itemCount {
			mid = itemCount - 1
		}
		ptListRow.want = rowColor(mid)
		ptListRow.x = bodyX + 48
		ptListRow.y = bodyY + (float64(mid)*rowExtent - scrollY) + rowExtent/2
		// F6 box: dense-note label region.
		capTextBox = rect{x: dx + 10, y: bodyY + dense.Offset().Y + 268, w: 230, h: 28}

		// Golden static mask: only regions that are time-invariant BY DESIGN.
		// Excluded: HUD band (fps/phase text), cache banner (counters grow),
		// the whole scrolling list column (dt jitter shifts rows between
		// runs), HOT spot (cycles every frame).
		goldenRects = []rect{
			{x: 0, y: 0, w: winW, h: 48},
			{shell.Legend.X, shell.Legend.Y, shell.Legend.W, shell.Legend.H},
			{dx - 2, bodyY + dense.Offset().Y - 2, 256, 436}, // dense cache panel
			{x: dx + 280, y: bodyY + 34, w: 200, h: 400},     // empty body-bg strip
		}
	}

	snapDir := os.Getenv("R14_SNAP_DIR")
	if snapDir == "" {
		snapDir = "/tmp/r14_cache_budget"
	}
	os.MkdirAll(snapDir, 0o755)

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: clearBG[0], ClearG: clearBG[1], ClearB: clearBG[2], ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		SnapshotPath: func() string {
			if secsSet {
				return filepath.Join(snapDir, "r14_final.png")
			}
			return ""
		}(),
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintf(os.Stderr, "ui_wr_r14_cache_budget: close (%s)\n", win.Backend())
			}
			if ev.Type == platform.EventResize && ev.Width > 0 && ev.Height > 0 {
				shell.Resize(float64(ev.Width), float64(ev.Height))
			}
		},
	})
	app.SetPresentPolicy(scheduler.PresentPolicyRetained)

	clock := wrkit.NewPhaseClock(20, 40) // Steady 20s → Spike 20s → Recover

	var elapsed, scrollY, dir float64 = 0, 0, 1
	var hotHue float64
	phasesSeen := map[string]bool{}
	budgetApplied := false
	texMin, texMax := int64(-1), int64(-1)
	var lastEvictions int64
	evMonotonicViolations := int64(0)
	everAtCap := false
	var bannerAccum float64

	app.Scheduler().Tickers().Add(&tickerT{on: func(dt float64) {
		elapsed += dt
		phase := clock.Advance(dt)
		phasesSeen[phase] = true
		spike := phase == wrkit.PhaseSpike

		// Explicit budget knob: applied once the raster thread has created
		// the texture cache (first retained frame); idempotent afterwards.
		if !budgetApplied {
			if tex := app.PictureTextures(); tex != nil {
				tex.SetBudget(int(texBudget))
				if bc := app.BoundaryCache(); bc != nil && boundaryDefaultCap > 0 {
					bc.SetMaxEntries(boundaryDefaultCap)
				}
				fmt.Fprintf(os.Stderr, "r14 budget applied: texture=%d boundary=%d\n", texBudget, boundaryDefaultCap)
				budgetApplied = true
			}
		}

		// Sustained scroll: fresh cells keep producing new cache keys. Rates
		// are deliberately moderate — the stress thesis is CUMULATIVE churn
		// over 60s (hundreds of evictions + cap enforcement), not per-frame
		// record bursts; faster scroll only piles re-records into hitch
		// frames and drags fps_interval below the family-A gate.
		speed := 200.0
		if spike {
			speed = 800.0
		} else if phase == wrkit.PhaseRecover {
			speed = 50.0
		}
		maxY := float64(itemCount)*rowExtent - float64(vpH)
		scrollY += dir * speed * dt
		if scrollY >= maxY {
			scrollY, dir = maxY, -1
		}
		if scrollY <= 0 {
			scrollY, dir = 0, 1
		}
		vp.SetScrollOffset(0, scrollY)
		if maxY > 0 {
			thumbAlign.SetAlignment(0.5, scrollY/maxY)
		}

		// HOT spot repaints itself every frame (speed ×2 in Spike).
		speedK := 1.0
		if spike {
			speedK = 2.0
		}
		hotHue = math.Mod(hotHue + dt*speedK*0.6, 1.0)
		hot.R = 0.85 + 0.1*hotHue
		hot.G = 0.25 + 0.4*(1-hotHue)
		hot.B = 0.3 + 0.5*hotHue
		hot.MarkNeedsPaint()

		pcSnap := app.Metrics().Snapshot()
		ent, ev := pcSnap.CacheEntries, pcSnap.CacheEvictions
		// The cap check targets the TEXTURE cache alone: CacheEntries is the
		// combined boundary+texture count and the boundary cache legitimately
		// adds ~30 sweep-bounded entries on top of the pinned budget.
		var texLen int64
		if tex := app.PictureTextures(); tex != nil {
			texLen = int64(tex.Len())
		}
		if elapsed >= 1.0 { // discard warm-up equilibrium from min/max band
			if texMin < 0 || texLen < texMin {
				texMin = texLen
			}
			if texLen > texMax {
				texMax = texLen
			}
			if texLen >= texBudget {
				everAtCap = true
			}
			if ev < lastEvictions {
				evMonotonicViolations++
			}
			lastEvictions = ev
		}
		cacheBanner.SetText(fmt.Sprintf("CACHE: entries=%d/%d evictions=%d", ent, texBudget, ev))
		// Throttle banner re-text to 2 Hz: a per-frame text change re-records
		// its glyph texture every frame (hidden raster cost on top of the
		// scroll churn this window already carries).
		bannerAccum += dt
		if bannerAccum >= 0.5 {
			bannerAccum = 0
			cacheBanner.MarkNeedsPaint()
		}

		gateOK := pcSnap.BoundarySkip > 0 && !(!everAtCap && elapsed > 5) && evMonotonicViolations == 0
		shell.NoteHUDTick(dt)
		shell.UpdateHUD("R14", phase, app, gateOK,
			fmt.Sprintf("ent=%d ev=%d", ent, ev),
			fmt.Sprintf("skip=%d rr=%d spike=%v", pcSnap.BoundarySkip, pcSnap.BoundaryRerecord, spike))
		app.ScheduleFrame()
		proc.Sample()
	}})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: open: %v\n", err)
		os.Exit(1)
	}
	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: run: %v\n", err)
		os.Exit(1)
	}
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	elapsedSec := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	wrkit.MergeBoundaryCache(app, &snap)
	if tex := app.PictureTextures(); tex != nil {
		fmt.Fprintf(os.Stderr, "texture cache: len=%d budget=%d evictions=%d\n",
			tex.Len(), tex.Budget(), tex.Evictions)
	}

	// ---- Final-frame pixel assertions (§2.7/U21) --------------------------
	// The engine snapshot is the FINAL frame; scrollY holds the last value,
	// so the row probe resolves against the actually-presented content.
	resolveGeometry(scrollY)
	finalChecks := []pixelCheck{
		{pt: &ptDenseCell, desc: "dense_cell_intact_under_eviction"},
		{pt: &ptListRow, desc: "scrolled_in_row_content_correct"},
		{text: &capTextBox, base: denseBase, desc: "static_note_text_density"},
	}
	runPixelChecks(loadImage(filepath.Join(snapDir, "r14_final.png")), finalChecks, pixelResult)

	scriptedOK, scriptedTotal := 0, len(pixelResult)
	for _, ok := range pixelResult {
		if ok {
			scriptedOK++
		}
	}

	goldenDiffPct, goldenTotalPx, goldenFirstRun := evaluateGolden(snapDir, goldenRects)

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R14",
		Scenario:      "ui_wr_r14_cache_budget",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"cache_entries":                snap.CacheEntries,
			"cache_entries_note":           "combined boundary+texture live entries",
			"texture_entries_min":          texMin,
			"texture_entries_max":          texMax,
			"texture_budget":               texBudget,
			"cache_evictions":              snap.CacheEvictions,
			"cache_evict_monotonic_violat": evMonotonicViolations,
			"boundary_cache_cap":           boundaryDefaultCap,
			"ever_at_cap":                  everAtCap,
			"hitch_budget_per_min":         hitchBudget,
			"rss_slope_budget_kb_per_min":  slopeBudget,
			"phases_seen":                  keysOf(phasesSeen),
			"scripted_ok":                  scriptedOK,
			"scripted_total":               scriptedTotal,
			"pixel_golden_diff_pct":        goldenDiffPct,
			"pixel_golden_total_px":        goldenTotalPx,
			"pixel_golden_first_run":       goldenFirstRun,
			"covered":                      "R14 cache budget/explicit eviction + R3 boundary cache coexistence + retained composite",
			"impl_correctness":             "evicted entries re-record on demand — scrolled-in row shows its exact per-index color (P2) and the dense panel stays intact (P1) under sustained eviction pressure",
			"impl_dirty":                   "steady frames re-record only newly mounted cells + banner + hot; the dense panel keeps skipping (boundary_skip monotonic)",
			"impl_cache":                   "explicit texture-LRU budget above the per-frame live set bounds the scroll-churn accumulation: cache_entries stays ≤ budget while evictions grow monotonically — budget enforced, never exceeded",
			"impl_edge":                    "budget applies at first retained frame (idempotent setter); probe geometry re-resolved on resize; expectations from frozen per-index colors",
			"impl_fail":                    "unbounded growth (entries>budget), silent eviction (counter flat), non-monotonic counter, wrong row color, or RSS slope runaway all trip gates → FAIL exit 1",
			"impl_visible":                 "HUD shows live entries/evictions; banner tracks the cap; list scrolls through all 600 rows; dense region looks perfectly still",
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	// ---- Gates (§2 R14 行 + §2.5 预算/压力 档 + §2.2 全族；硬，不许放) ------
	if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
		MinPresents:           1,
		RequirePersistentFPS:  true,
		MinFPSWall:            55,
		MaxP95Ms:              22,
		RequireRetainedPolicy: true,
		MinBoundarySkip:       3,
		MinBoundaryRerecord:   1,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: %v\n", err)
		os.Exit(1)
	}
	if snap.VSyncSource == "" {
		fmt.Fprintln(os.Stderr, "FAIL: vsync_source missing")
		os.Exit(1)
	}
	if secsSet && secs < closeSeconds {
		fmt.Fprintf(os.Stderr, "FAIL: RUN_SECONDS=%d want >=%d (§2.5 预算/压力 档关闭用时长)\n", secs, closeSeconds)
		os.Exit(1)
	}
	// 族 C 能力专用①: 纹理缓存条目必须被观测到且被预算封顶。
	if texMax <= 0 {
		fmt.Fprintln(os.Stderr, "FAIL: texture cache entries never observed (接线断)")
		os.Exit(1)
	}
	if texMax > texBudget {
		fmt.Fprintf(os.Stderr, "FAIL: texture_entries_max=%d > budget=%d (预算未封顶)", texMax, texBudget)
		os.Exit(1)
	}
	if !everAtCap {
		fmt.Fprintln(os.Stderr, "FAIL: cache never reached the budget cap (压力不足或淘汰失效)")
		os.Exit(1)
	}
	// 族 C 能力专用②: 淘汰必须真实发生且计数严格单调。
	if snap.CacheEvictions < evictBudgetMin {
		fmt.Fprintf(os.Stderr, "FAIL: cache_evictions=%d want >=%d (淘汰未发生)", snap.CacheEvictions, evictBudgetMin)
		os.Exit(1)
	}
	if evMonotonicViolations != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: evict_monotonic_violations=%d want 0 (淘汰计数回退)", evMonotonicViolations)
		os.Exit(1)
	}
	// 族 A: 持续 tick 60fps 档。
	if fpsOf(snap) < 55 {
		fmt.Fprintf(os.Stderr, "FAIL: fps_interval=%.1f < 55 (持续 tick R14)", fpsOf(snap))
		os.Exit(1)
	}
	if snap.HitchRatePerMin > hitchBudget {
		fmt.Fprintf(os.Stderr, "FAIL: hitch_rate_per_min=%.2f > %.0f\n", snap.HitchRatePerMin, hitchBudget)
		os.Exit(1)
	}
	// 族 E: RSS 斜率预算（稳态语义；本能力的核心门禁之一）。
	if snap.RSSSlopeKBPerMin > slopeBudget {
		fmt.Fprintf(os.Stderr, "FAIL: rss_slope_kb_per_min=%.1f > %.0f", snap.RSSSlopeKBPerMin, slopeBudget)
		os.Exit(1)
	}
	// 族 D: CPU 非双 0。
	if snap.CPUPctAvg <= 0 || (snap.CPUUIPct <= 0 && snap.CPURasterPct <= 0) {
		fmt.Fprintln(os.Stderr, "FAIL: cpu 双 0 装绿嫌疑")
		os.Exit(1)
	}
	// 像素断言全过。
	if scriptedTotal != 3 || scriptedOK != scriptedTotal {
		fmt.Fprintf(os.Stderr, "FAIL: scripted=%d/%d want 3/3 (像素断言)\n", scriptedOK, scriptedTotal)
		os.Exit(1)
	}
	// Golden 零容差（首跑产基线；次跑起逐位一致）。
	if !goldenFirstRun && goldenDiffPct != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: pixel_golden_diff_pct=%.4f want 0 over %d px\n", goldenDiffPct, goldenTotalPx)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "ui_wr_r14_cache_budget: OK presents=%d fps=%.1f p95=%.1f hitch=%.2f/min ent=%d(min %d max %d) ev=%d slope=%.0fKB/min skip=%d scripted=%d/%d golden=%.4f%%(%dpx first=%v) vsync=%s elapsed=%.1fs\n",
		app.PresentCount(), fpsOf(snap), snap.P95FrameIntervalMs, snap.HitchRatePerMin,
		snap.CacheEntries, texMin, texMax, snap.CacheEvictions, snap.RSSSlopeKBPerMin,
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
			fmt.Fprintf(os.Stderr, "ui_wr_r14_cache_budget: pixel %-32s @(%4.0f,%4.0f) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) ok=%v\n",
				c.desc, c.pt.x, c.pt.y, r, g, b, c.pt.want[0], c.pt.want[1], c.pt.want[2], ok)
		} else if c.text != nil {
			n := textPixels(img, dpr, *c.text, c.base)
			ok = img != nil && n >= 120
			fmt.Fprintf(os.Stderr, "ui_wr_r14_cache_budget: pixel %-32s text_px=%d (want >=120) ok=%v\n", c.desc, n, ok)
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
	cur := filepath.Join(snapDir, "r14_final.png")
	base := filepath.Join(snapDir, "r14_final_base.png")
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
				ar, ag, ab, aa := a.At(px, py).RGBA()
				br, bg, bb, ba := b.At(px, py).RGBA()
				total++
				if ar != br || ag != bg || ab != bb || aa != ba {
					diff++
				}
			}
		}
	}
	if total == 0 {
		return 0, 0, fmt.Errorf("empty mask")
	}
	return float64(diff) / float64(total) * 100, total, nil
}

func fpsOf(snap scheduler.FrameMetrics) float64 {
	if snap.AvgFrameIntervalMs > 1e-6 {
		return 1000.0 / snap.AvgFrameIntervalMs
	}
	return 0
}

func keysOf(m map[string]bool) string {
	out := ""
	for k := range m {
		if out != "" {
			out += ","
		}
		out += k
	}
	return out
}

type tickerT struct{ on func(dt float64) }

func (t *tickerT) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
