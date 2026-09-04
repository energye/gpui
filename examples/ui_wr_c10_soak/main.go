// Command ui_wr_c10_soak is the C10 combined soak real window:
// R15+全主路径 — 全能力组合长跑（静态+动画+滚动+浮层+图片）300s。
//
// §3 C10 thesis: 全能力长跑不泄漏不崩溃 — 在 R15 全主路径 soak 场景上叠加
// 图片条带（启动时一次解码，300s 全程零解码）：滚动 churn、浮层开合、图片
// 纹理同帧合成，相互不拖累。结束时判无崩 + hitch_rate 合理 + CPU/RSS 稳定 +
// 无渐进退化（图片像素 300s 后仍精确）。
//
// Gates (§3 C10 行 + §2.5 Soak 长跑 档 + 覆盖能力门禁并集；硬，不许放):
// R15 全部门禁 + img_loaded==4（4 格图片启动时全部就位）+ 像素断言 5/5
// （多第 5 项图片格色）+ Golden 静态掩码零容差（次跑起）。运行中无逐帧
// 读回：像素采样只读终帧 PNG 文件，不碰 GPU backbuffer。
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=300 go run ./examples/ui_wr_c10_soak
//
// Window: 1200x800. RUN_SECONDS>=5 (U16); closing requires 300 (§2.5 Soak
// 长跑 档). No RUN_SECONDS → runs the 300s closing duration. GPU window
// required (needs_gpu_window otherwise).
package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/io"
	"github.com/energye/gpui/ui/overlay"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH   = 1200, 800
	closeSeconds = 300 // §2.5 Soak 长跑 档：C10 = 300s

	itemCount  = 600
	rowExtent  = 56.0 // fixed row height
	hitchBudget = 5.0
	slopeBudget = 30000.0 // §2.2.4 default extreme-climb budget (KB/min)
	cpuBudget   = 85.0    // §2.2.3 长窗单核折算预算
	bindCap     = 64      // bind≪item_count (600 行只绑 ≤64)

	cyclePeriod  = 60.0 // soak 相位周期：Steady 20s → Spike 20s → Recover 20s，300s 跑满 5 轮
	steadyEnd    = 20.0
	spikeEnd     = 40.0
	overlayOpen  = 20.0 // 每轮 Spike 起浮层打开…
	overlayClose = 28.0 // …8s 后关闭（5 轮共 5 次开合）
	overlayMinOpens = 3 // 300s 关闭跑至少见到 3 次开合
)

var bodyBGColor = [3]float64{0.10, 0.11, 0.13}

// Palette shared by scene builder AND pixel assertions (single source of truth).
var (
	clearBG   = [3]float64{0.08, 0.09, 0.11}
	denseBase = [3]float64{0.10, 0.11, 0.13}
	denseCell = [3]float64{0.37, 0.50, 0.72}
	nestInner = [3]float64{0.62, 0.38, 0.28} // nested inner frozen cell (probe)
	ovPanel   = [3]float64{0.16, 0.30, 0.42} // overlay panel fill (open-state evidence)
)

// rowColor is the deterministic per-row fill color.
func rowColor(i int) [3]float64 {
	return [3]float64{
		0.16 + 0.07*float64(i%5),
		0.38 + 0.08*float64(i%4),
		0.52 + 0.10*float64(i%3),
	}
}

// decodeStripedCell generates a 96x56 striped PNG to a process-temp file and
// decodes it through the engine image pool (R10-style file decode path),
// blocking until the decode completes — all 4 decodes happen BEFORE the
// window opens, so the 300s run itself performs zero decodes. Cell 0 is a
// solid fill (F0 pixel probe target); cells 1-3 are striped for richness.
func decodeStripedCell(idx int, base [3]float64) *render.ImageBuf {
	m := image.NewRGBA(image.Rect(0, 0, 96, 56))
	solid := color.RGBA{R: uint8(base[0] * 255), G: uint8(base[1] * 255), B: uint8(base[2] * 255), A: 255}
	alt := color.RGBA{R: uint8(base[0] * 255 * 0.45), G: uint8(base[1] * 255 * 0.45), B: uint8(base[2] * 255 * 0.45), A: 255}
	for y := 0; y < 56; y++ {
		for x := 0; x < 96; x++ {
			c := solid
			if idx != 0 && ((x+y)/8+idx)%2 == 0 {
				c = alt
			}
			m.Set(x, y, c)
		}
	}
	f, err := os.CreateTemp("", "c10_cell_*.png")
	if err != nil {
		return nil
	}
	name := f.Name()
	if err := png.Encode(f, m); err != nil {
		f.Close()
		os.Remove(name)
		return nil
	}
	f.Close()
	done := make(chan io.Result, 1)
	io.DecodeFile(name, func(res io.Result) { done <- res })
	select {
	case res := <-done:
		os.Remove(name)
		if res.Err != nil || res.Img == nil {
			fmt.Fprintf(os.Stderr, "decode cell %d: %v\n", idx, res.Err)
			return nil
		}
		return res.Img
	case <-time.After(10 * time.Second):
		os.Remove(name)
		fmt.Fprintf(os.Stderr, "decode cell %d: timeout\n", idx)
		return nil
	}
}

// cyclicPhase maps elapsed seconds into a repeating 60s Steady→Spike→Recover.
func cyclicPhase(elapsed float64) (string, float64) {	cycleT := math.Mod(elapsed, cyclePeriod)
	switch {
	case cycleT < steadyEnd:
		return wrkit.PhaseSteady, cycleT
	case cycleT < spikeEnd:
		return wrkit.PhaseSpike, cycleT
	default:
		return wrkit.PhaseRecover, cycleT
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
		wrkit.RequireMinRun(secs, "C10")
	} else {
		secs = closeSeconds
	}
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: "gpui ui_wr_c10_soak — 全主路径 300s 长跑", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "C10 全能力组合长跑 — 300s 不泄漏不退化", []string{
		"LIST  600 行持续滚动 相位变速 5 轮",
		"NEST  3 层嵌套 boundary（内脏不外溢）",
		"DENSE 静态色格面板（全程 skip）",
		"HOT   每帧变色动态热点（非缓存）",
		"OVERLAY 浮层每轮 Spike 开 8s 关",
		"IMG   4 格解码图片 300s 零解码同屏",
		"相位 60s 周期 Steady→Spike→Recover",
		"hitch/CPU/RSS 长窗口径判（见 README）",
		"终帧像素探针 + Golden 静态掩码",
		"HUD 实时 fps/p95/skip/bind/img/ov",
	})

	body := shell.Body

	// ===== LIST: sustained-scroll producer (滚动) ==========================
	listW := 420.0
	vpH := body.H
	list := rendering.NewVirtualList(itemCount, rowExtent, func(i int) rendering.RenderObject {
		c := rowColor(i)
		cell := rendering.NewAbsoluteBox(listW, rowExtent)
		box := rendering.NewRenderColorBox(listW, rowExtent, c[0], c[1], c[2], 1)
		box.SetRepaintBoundary(true)
		cell.Place(box, 0, 0)
		t := wrkit.Label(fmt.Sprintf("row %04d — 长跑滚动内容正确", i), 13, 0.94, 0.96, 1.0)
		cell.Place(t, listW-330, 16)
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

	// ===== NEST: 3-layer nested boundary (静态隔离) ========================
	nestOuter := rendering.NewAbsoluteBox(200, 300)
	nestOuter.Background = &rendering.Color{R: 0.14, G: 0.15, B: 0.20, A: 1}
	nestOuter.SetRepaintBoundary(true)
	nestMid := rendering.NewAbsoluteBox(170, 250)
	nestMid.Background = &rendering.Color{R: 0.16, G: 0.18, B: 0.24, A: 1}
	nestMid.SetRepaintBoundary(true)
	nestOuter.Place(nestMid, 15, 25)
	nestInnerBox := rendering.NewAbsoluteBox(140, 170)
	nestInnerBox.Background = &rendering.Color{R: nestInner[0], G: nestInner[1], B: nestInner[2], A: 1}
	nestInnerBox.SetRepaintBoundary(true)
	nestMid.Place(nestInnerBox, 15, 40)
	// Inner phase chip: recolors only on phase flips → inner rerecord ≥1,
	// outer/mid keep skipping (nesting isolation proof).
	nestChip := rendering.NewRenderColorBox(120, 40, 0.85, 0.55, 0.25, 1)
	nestInnerBox.Place(nestChip, 10, 10)
	nestLbl := wrkit.Label("nest inner 静区", 10, 0.9, 0.85, 0.7)
	nestInnerBox.Place(nestLbl, 10, 60)
	body.Box.Place(nestOuter, 440, 10)

	// ===== DENSE: static panel (must stay untouched) =======================
	dense := rendering.NewAbsoluteBox(200, 200)
	dense.Background = &rendering.Color{R: denseBase[0], G: denseBase[1], B: denseBase[2], A: 1}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			c := rendering.NewRenderColorBox(58, 48,
				denseCell[0]*(1-0.08*float64(i)), denseCell[1]*(0.8+0.1*float64(j)), denseCell[2], 1)
			c.SetRepaintBoundary(true)
			dense.Place(c, 10+float64(i)*64, 10+float64(j)*54)
		}
	}
	// Static note: the F6 text-density probe target (must exist — an empty
	// box scores 0 text pixels and trips the pixel gate).
	denseNote := wrkit.Label("静态缓存：全程 skip 不重录", 11, 0.55, 0.75, 0.95)
	dense.Place(denseNote, 10, 172)
	body.Box.Place(dense, 440, 320)

	// ===== HOT spot (non-cached animation) =================================
	hot := rendering.NewRenderColorBox(24, 24, 0.95, 0.3, 0.25, 1)
	body.Box.Place(hot, 680, 40)
	hotLbl := wrkit.Label("HOT 每帧变色", 11, 0.95, 0.7, 0.6)
	body.Box.Place(hotLbl, 680, 70)
	ovLbl := wrkit.Label("OVERLAY 每轮 Spike 开 8s（右下）", 11, 0.70, 0.85, 0.95)
	body.Box.Place(ovLbl, 660, 100)

	// ===== IMG: 4 decoded image cells (图片; decoded once at startup) ======
	// Startup-only file decode path (R10-style): PNGs are generated + decoded
	// BEFORE the window opens, so the 300s run itself performs zero decodes —
	// the soak proves decoded image textures composite stably, not that the
	// decoder is fast. Cell 0 is a solid fill (F0 pixel probe target).
	imgTitle := wrkit.Label("IMG 启动一次解码·全程零解码", 11, 0.65, 0.80, 0.95)
	body.Box.Place(imgTitle, 660, 118)
	imgCellBase := [][3]float64{
		{0.30, 0.55, 0.80},
		{0.75, 0.45, 0.25},
		{0.35, 0.65, 0.40},
		{0.70, 0.35, 0.55},
	}
	imgLoaded := 0
	for i := 0; i < 4; i++ {
		box := rendering.NewAbsoluteBox(104, 64)
		box.Background = &rendering.Color{R: 0.13, G: 0.14, B: 0.18, A: 1}
		box.SetRepaintBoundary(true)
		if im := decodeStripedCell(i, imgCellBase[i]); im != nil {
			ri := rendering.NewRenderImage(96, 56)
			ri.SetImage(im)
			box.Place(ri, 4, 4)
			imgLoaded++
		}
		body.Box.Place(box, 660+float64(i%2)*110, 140+float64(i/2)*70)
	}
	if imgLoaded != 4 {
		fmt.Fprintf(os.Stderr, "FAIL: img_loaded=%d want 4 (图片启动解码失败)\n", imgLoaded)
		os.Exit(1)
	}

	// ===== Soak banner ======================================================
	soakBanner := wrkit.Label("SOAK: bind=-- skip=-- ov=--", 12, 0.95, 0.90, 0.55)
	body.Box.Place(soakBanner, 440, 540)

	// ===== Probe geometry ==================================================
	ptNestInner := probePoint{want: nestInner, tol: 8.0 / 255}
	ptListRow := probePoint{tol: 6.0 / 255}
	ptOverlayClosed := probePoint{want: bodyBGColor, tol: 8.0 / 255}
	ptImgCell := probePoint{want: imgCellBase[0], tol: 8.0 / 255}
	capTextBox := rect{}
	var goldenRects []rect

	resolveGeometry := func(scrollY float64) {
		bodyX, bodyY := body.Box.Offset().X, body.Box.Offset().Y
		nx := bodyX + nestOuter.Offset().X + 15 + 15
		ny := bodyY + nestOuter.Offset().Y + 25 + 40
		ptNestInner.x = nx + 100
		ptNestInner.y = ny + 120
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
		dx := bodyX + dense.Offset().X
		capTextBox = rect{x: dx + 10, y: bodyY + dense.Offset().Y + 172, w: 180, h: 24}
		// Overlay closed-state probe: panel zone center shows body bg when
		// the overlay is closed (final frame is always closed — cycleT=0).
		ptOverlayClosed.x = 950 + 100
		ptOverlayClosed.y = 570 + 75
		// Image cell 0 center: solid fill, decoded once at startup.
		ptImgCell.x = bodyX + 660 + 4 + 48
		ptImgCell.y = bodyY + 140 + 4 + 28
		// Golden static mask: time-invariant BY DESIGN. Excluded: HUD band,
		// soak banner, scrolling list column, HOT spot, overlay corner zone.
		goldenRects = []rect{
			{x: 0, y: 0, w: winW, h: 48},
			{shell.Legend.X, shell.Legend.Y, shell.Legend.W, shell.Legend.H},
			{dx - 2, bodyY + dense.Offset().Y - 2, 206, 206},
			{nx - 2, ny - 2, 146, 176},
		}
	}

	snapDir := os.Getenv("C10_SNAP_DIR")
	if snapDir == "" {
		snapDir = "/tmp/c10_soak"
	}
	os.MkdirAll(snapDir, 0o755)

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: clearBG[0], ClearG: clearBG[1], ClearB: clearBG[2], ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		SnapshotPath: func() string {
			if secsSet {
				return filepath.Join(snapDir, "c10_final.png")
			}
			return ""
		}(),
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintf(os.Stderr, "ui_wr_c10_soak: close (%s)\n", win.Backend())
			}
			if ev.Type == platform.EventResize && ev.Width > 0 && ev.Height > 0 {
				shell.Resize(float64(ev.Width), float64(ev.Height))
			}
		},
	})

	// NOTE: no explicit SetPresentPolicy — C10 runs the W6 retained default.
	ov := overlay.New()
	app.SetOverlay(ov)

	var elapsed, scrollY, dir float64 = 0, 0, 1
	var hotHue float64
	phasesSeen := map[string]bool{}
	var bannerAccum float64
	lastPhase := ""
	overlayOpens := 0
	var ovEntry *overlay.Entry
	ovIsOpen := false

	openOverlay := func() {
		n := overlayOpens + 1
		panel := rendering.NewAbsoluteBox(200, 150)
		panel.Background = &rendering.Color{R: ovPanel[0], G: ovPanel[1], B: ovPanel[2], A: 1}
		panel.SetRepaintBoundary(true)
		panel.Place(wrkit.Label(fmt.Sprintf("浮层面板 open #%d", n), 12, 0.92, 0.95, 1.0), 12, 12)
		panel.Place(wrkit.Label("Spike 期开 8s 后关", 11, 0.75, 0.85, 0.95), 12, 40)
		panel.Place(wrkit.Label("主带不受浮层拖累", 11, 0.75, 0.85, 0.95), 12, 64)
		ovEntry = overlay.NewEntry(panel, 950, 570, 200, 150)
		ov.Insert(ovEntry)
		ovIsOpen = true
		overlayOpens = n
	}
	closeOverlay := func() {
		if ovEntry != nil {
			ov.Remove(ovEntry)
			ovEntry = nil
		}
		ovIsOpen = false
	}

	app.Scheduler().Tickers().Add(&tickerT{on: func(dt float64) {
		elapsed += dt
		phase, cycleT := cyclicPhase(elapsed)
		phasesSeen[phase] = true

		if phase != lastPhase {
			lastPhase = phase
			// Phase flips recolor only the nest inner chip: inner rerecords,
			// outer/mid keep skipping (nesting isolation inside the soak).
			switch phase {
			case wrkit.PhaseSteady:
				nestChip.R, nestChip.G, nestChip.B = 0.85, 0.55, 0.25
			case wrkit.PhaseSpike:
				nestChip.R, nestChip.G, nestChip.B = 0.25, 0.70, 0.85
			default:
				nestChip.R, nestChip.G, nestChip.B = 0.55, 0.85, 0.35
			}
			nestChip.MarkNeedsPaint()
		}

		// Overlay opens during [overlayOpen, overlayClose) of each Spike.
		wantOpen := phase == wrkit.PhaseSpike && cycleT >= overlayOpen && cycleT < overlayClose
		if wantOpen && !ovIsOpen {
			openOverlay()
		} else if !wantOpen && ovIsOpen {
			closeOverlay()
		}

		speed := 150.0
		if phase == wrkit.PhaseSpike {
			speed = 600.0
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

		speedK := 1.0
		if phase == wrkit.PhaseSpike {
			speedK = 2.0
		}
		hotHue = math.Mod(hotHue+dt*speedK*0.6, 1.0)
		hot.R = 0.85 + 0.1*hotHue
		hot.G = 0.25 + 0.4*(1-hotHue)
		hot.B = 0.3 + 0.5*hotHue
		hot.MarkNeedsPaint()

		pcSnap := app.Metrics().Snapshot()
		bannerAccum += dt
		if bannerAccum >= 0.5 {
			bannerAccum = 0
			soakBanner.SetText(fmt.Sprintf("SOAK: bind=%d skip=%d ov=%d t=%.0fs",
				pcSnap.BindCount, pcSnap.BoundarySkip, overlayOpens, elapsed))
			soakBanner.MarkNeedsPaint()
		}

		gateOK := pcSnap.BoundarySkip > 0
		ovState := "closed"
		if ovIsOpen {
			ovState = fmt.Sprintf("open#%d", overlayOpens)
		}
		shell.NoteHUDTick(dt)
		shell.UpdateHUD("C10", phase, app, gateOK,
			fmt.Sprintf("bind=%d ov=%s", pcSnap.BindCount, ovState),
			fmt.Sprintf("skip=%d rr=%d t=%.0fs", pcSnap.BoundarySkip, pcSnap.BoundaryRerecord, elapsed))
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

	// ---- Final-frame pixel assertions (§2.7/U21) --------------------------
	// Final frame is always overlay-closed (300s = 5 full 60s cycles,
	// cycleT=0 → Steady): deterministic, never a transition frame.
	resolveGeometry(scrollY)
	finalChecks := []pixelCheck{
		{pt: &ptNestInner, desc: "nested_inner_intact_over_soak"},
		{pt: &ptListRow, desc: "scrolled_in_row_content_correct"},
		{pt: &ptOverlayClosed, desc: "overlay_zone_closed_no_residue"},
		{pt: &ptImgCell, desc: "image_cell0_stable_over_soak"},
		{text: &capTextBox, base: denseBase, desc: "static_note_text_density"},
	}
	runPixelChecks(loadImage(filepath.Join(snapDir, "c10_final.png")), finalChecks, pixelResult)

	scriptedOK, scriptedTotal := 0, len(pixelResult)
	for _, ok := range pixelResult {
		if ok {
			scriptedOK++
		}
	}

	goldenDiffPct, goldenTotalPx, goldenFirstRun := evaluateGolden(snapDir, goldenRects)

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "C10",
		Scenario:      "ui_wr_c10_soak",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"covers":                      []string{"R15", "全主路径+图片"},
			"item_count":                  snap.ItemCount,
			"bind_count":                  snap.BindCount,
			"scroll_rerecord":             snap.ScrollRerecord,
			"overlay_opens":               overlayOpens,
			"img_loaded":                  imgLoaded,
			"overlay_open_per_cycle_sec":  overlayClose - overlayOpen,
			"cycle_period_sec":            cyclePeriod,
			"hitch_budget_per_min":        hitchBudget,
			"rss_slope_budget_kb_per_min": slopeBudget,
			"cpu_budget_pct":              cpuBudget,
			"phases_seen":                 keysOf(phasesSeen),
			"scripted_ok":                 scriptedOK,
			"scripted_total":              scriptedTotal,
			"pixel_golden_diff_pct":       goldenDiffPct,
			"pixel_golden_total_px":       goldenTotalPx,
			"pixel_golden_first_run":      goldenFirstRun,
			"covered":                     "R15 全主路径 soak + 4 格解码图片同屏 300s",
			"impl_correctness":            "终帧探针：嵌套内层色不变、滚入行按索引色精确、浮层区关闭无残留、图片格 0 色稳定、静区文字密度达标",
			"impl_dirty":                  "稳态每帧只重录新挂载 cell + banner + hot + 相位翻转 nest chip + 浮层开合；dense 与 nest 外/中层持续 skip",
			"impl_cache":                  "600 行只绑一小窗；相位翻转只脏内层 chip；图片启动一次解码后常驻纹理",
			"impl_interaction":            "滚动 churn、浮层开合、图片纹理同帧合成：图片格全程静止证明合成不污染静态纹理，浮层开合不抬升主带重录",
			"impl_edge":                   "300s 整 5 个 60s 周期，终帧 cycleT=0 浮层必关，探针避开过渡帧",
			"impl_fail":                   "崩溃、hitch/CPU/RSS 出预算、bind 爆炸、浮层开合不足、像素/Golden 任一失败 → FAIL exit 1",
			"impl_visible":                "HUD 实时 fps/p95/bind/skip/overlay 开合态；列表 5 轮变速滚动；浮层每轮 Spike 出现 8s",
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	// ---- Gates (§3 C10 行 + §2.5 Soak 长跑 档 + §2.2 全族；硬，不许放) ----
	if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
		MinPresents:          1,
		RequirePersistentFPS: true,
		MinFPSWall:           55,
		MaxP95Ms:             22,
		MinBoundarySkip:      3,
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
		fmt.Fprintf(os.Stderr, "FAIL: RUN_SECONDS=%d want >=%d (§2.5 Soak 长跑 档关闭用时长)\n", secs, closeSeconds)
		os.Exit(1)
	}
	if snap.ItemCount != itemCount {
		fmt.Fprintf(os.Stderr, "FAIL: item_count=%d want %d (列表规模不对)", snap.ItemCount, itemCount)
		os.Exit(1)
	}
	if snap.BindCount <= 0 || snap.BindCount > bindCap {
		fmt.Fprintf(os.Stderr, "FAIL: bind_count=%d want 1..%d (bind 爆炸或未绑定)", snap.BindCount, bindCap)
		os.Exit(1)
	}
	if snap.ScrollRerecord <= 0 {
		fmt.Fprintln(os.Stderr, "FAIL: scroll_rerecord=0 (列表没滚起来)")
		os.Exit(1)
	}
	if overlayOpens < overlayMinOpens {
		fmt.Fprintf(os.Stderr, "FAIL: overlay_opens=%d want >=%d (浮层没按周期开合)", overlayOpens, overlayMinOpens)
		os.Exit(1)
	}
	// 族 A: 持续 tick 60fps 档。
	if fpsOf(snap) < 55 {
		fmt.Fprintf(os.Stderr, "FAIL: fps_interval=%.1f < 55 (持续 tick C10)", fpsOf(snap))
		os.Exit(1)
	}
	if snap.HitchRatePerMin > hitchBudget {
		fmt.Fprintf(os.Stderr, "FAIL: hitch_rate_per_min=%.2f > %.0f\n", snap.HitchRatePerMin, hitchBudget)
		os.Exit(1)
	}
	// 族 D: 300s 长窗 CPU 上限（§2.2.3）+ 非双 0。
	if snap.CPUPctAvg > cpuBudget {
		fmt.Fprintf(os.Stderr, "FAIL: cpu_pct_avg=%.1f > %.0f (长窗 CPU 超预算)", snap.CPUPctAvg, cpuBudget)
		os.Exit(1)
	}
	if snap.CPUPctAvg <= 0 || (snap.CPUUIPct <= 0 && snap.CPURasterPct <= 0) {
		fmt.Fprintln(os.Stderr, "FAIL: cpu 双 0 装绿嫌疑")
		os.Exit(1)
	}
	// 族 E: RSS 斜率预算。
	if snap.RSSSlopeKBPerMin > slopeBudget {
		fmt.Fprintf(os.Stderr, "FAIL: rss_slope_kb_per_min=%.1f > %.0f", snap.RSSSlopeKBPerMin, slopeBudget)
		os.Exit(1)
	}
	// 像素断言全过。
	if scriptedTotal != 5 || scriptedOK != scriptedTotal {
		fmt.Fprintf(os.Stderr, "FAIL: scripted=%d/%d want 5/5 (像素断言)\n", scriptedOK, scriptedTotal)
		os.Exit(1)
	}
	// Golden 零容差（首跑产基线；次跑起逐位一致）。
	if !goldenFirstRun && goldenDiffPct != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: pixel_golden_diff_pct=%.4f want 0 over %d px\n", goldenDiffPct, goldenTotalPx)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "ui_wr_c10_soak: OK presents=%d fps=%.1f p95=%.1f hitch=%.2f/min bind=%d/%d scroll_rr=%d ov_opens=%d slope=%.0fKB/min cpu=%.1f skip=%d scripted=%d/%d golden=%.4f%%(%dpx first=%v) vsync=%s elapsed=%.1fs\n",
		app.PresentCount(), fpsOf(snap), snap.P95FrameIntervalMs, snap.HitchRatePerMin,
		snap.BindCount, snap.ItemCount, snap.ScrollRerecord, overlayOpens,
		snap.RSSSlopeKBPerMin, snap.CPUPctAvg,
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
			fmt.Fprintf(os.Stderr, "ui_wr_c10_soak: pixel %-32s @(%4.0f,%4.0f) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) ok=%v\n",
				c.desc, c.pt.x, c.pt.y, r, g, b, c.pt.want[0], c.pt.want[1], c.pt.want[2], ok)
		} else if c.text != nil {
			n := textPixels(img, dpr, *c.text, c.base)
			ok = img != nil && n >= 120
			fmt.Fprintf(os.Stderr, "ui_wr_c10_soak: pixel %-32s text_px=%d (want >=120) ok=%v\n", c.desc, n, ok)
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
	cur := filepath.Join(snapDir, "c10_final.png")
	base := filepath.Join(snapDir, "c10_final_base.png")
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
