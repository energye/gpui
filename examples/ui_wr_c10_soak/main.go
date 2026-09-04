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
	"github.com/energye/gpui/examples/wrsoak"
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

	// Body-local layout (logical px). resolveGeometry derives probe sites
	// from the same constants — no hand-computed absolutes.
	listW = 420.0
	nestX, nestY     = 440.0, 10.0
	denseX, denseY   = 440.0, 320.0
	hotX, hotY       = 680.0, 40.0
	hotLblY          = 70.0
	ovLblX, ovLblY   = 660.0, 100.0
	bannerX, bannerY = 440.0, 540.0
	ovWinX, ovWinY   = 950.0, 570.0
	ovW, ovH         = 200.0, 150.0
	imgX, imgY       = 660.0, 140.0
	imgTitleY        = 118.0
	imgCellW, imgCellH = 104.0, 64.0
	imgW, imgH         = 96.0, 56.0
	imgStepX, imgStepY = 110.0, 70.0
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
	body.Box.Place(nestOuter, nestX, nestY)

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
	body.Box.Place(dense, denseX, denseY)

	// ===== HOT spot (non-cached animation) =================================
	hot := rendering.NewRenderColorBox(24, 24, 0.95, 0.3, 0.25, 1)
	body.Box.Place(hot, hotX, hotY)
	hotLbl := wrkit.Label("HOT 每帧变色", 11, 0.95, 0.7, 0.6)
	body.Box.Place(hotLbl, hotX, hotLblY)
	ovLbl := wrkit.Label("OVERLAY 每轮 Spike 开 8s（右下）", 11, 0.70, 0.85, 0.95)
	body.Box.Place(ovLbl, ovLblX, ovLblY)

	// ===== IMG: 4 decoded image cells (图片; decoded once at startup) ======
	// Startup-only file decode path (R10-style): PNGs are generated + decoded
	// BEFORE the window opens, so the 300s run itself performs zero decodes —
	// the soak proves decoded image textures composite stably, not that the
	// decoder is fast. Cell 0 is a solid fill (F0 pixel probe target).
	imgTitle := wrkit.Label("IMG 启动一次解码·全程零解码", 11, 0.65, 0.80, 0.95)
	body.Box.Place(imgTitle, imgX, imgTitleY)
	imgCellBase := [][3]float64{
		{0.30, 0.55, 0.80},
		{0.75, 0.45, 0.25},
		{0.35, 0.65, 0.40},
		{0.70, 0.35, 0.55},
	}
	imgLoaded := 0
	for i := 0; i < 4; i++ {
		box := rendering.NewAbsoluteBox(imgCellW, imgCellH)
		box.Background = &rendering.Color{R: 0.13, G: 0.14, B: 0.18, A: 1}
		box.SetRepaintBoundary(true)
		if im := decodeStripedCell(i, imgCellBase[i]); im != nil {
			ri := rendering.NewRenderImage(imgW, imgH)
			ri.SetImage(im)
			box.Place(ri, 4, 4)
			imgLoaded++
		}
		body.Box.Place(box, imgX+float64(i%2)*imgStepX, imgY+float64(i/2)*imgStepY)
	}
	if imgLoaded != 4 {
		fmt.Fprintf(os.Stderr, "FAIL: img_loaded=%d want 4 (图片启动解码失败)\n", imgLoaded)
		os.Exit(1)
	}

	// ===== Soak banner ======================================================
	soakBanner := wrkit.Label("SOAK: bind=-- skip=-- ov=--", 12, 0.95, 0.90, 0.55)
	body.Box.Place(soakBanner, bannerX, bannerY)

	// ===== Probe geometry ==================================================
	ptNestInner := wrsoak.ProbePoint{Want: nestInner, Tol: 8.0 / 255}
	ptListRow := wrsoak.ProbePoint{Tol: 6.0 / 255}
	ptOverlayClosed := wrsoak.ProbePoint{Want: bodyBGColor, Tol: 8.0 / 255}
	ptImgCell := wrsoak.ProbePoint{Want: imgCellBase[0], Tol: 8.0 / 255}
	capTextBox := wrsoak.Rect{}
	var goldenRects []wrsoak.Rect

	resolveGeometry := func(scrollY float64) {
		bodyX, bodyY := body.Box.Offset().X, body.Box.Offset().Y
		nx := bodyX + nestOuter.Offset().X + 15 + 15
		ny := bodyY + nestOuter.Offset().Y + 25 + 40
		ptNestInner.X = nx + 100
		ptNestInner.Y = ny + 120
		firstVis := int(scrollY / rowExtent)
		lastVis := int((scrollY + float64(vpH)) / rowExtent)
		if lastVis >= itemCount {
			lastVis = itemCount - 1
		}
		mid := (firstVis + lastVis) / 2
		if mid >= itemCount {
			mid = itemCount - 1
		}
		ptListRow.Want = rowColor(mid)
		ptListRow.X = bodyX + 48
		ptListRow.Y = bodyY + (float64(mid)*rowExtent - scrollY) + rowExtent/2
		dx := bodyX + dense.Offset().X
		capTextBox = wrsoak.Rect{X: dx + 10, Y: bodyY + dense.Offset().Y + 172, W: 180, H: 24}
		// Overlay closed-state probe: panel zone center shows body bg when
		// the overlay is closed (final frame is always closed — cycleT=0).
		ptOverlayClosed.X = ovWinX + 100
		ptOverlayClosed.Y = ovWinY + 75
		// Image cell 0 center: solid fill, decoded once at startup.
		ptImgCell.X = bodyX + imgX + 4 + 48
		ptImgCell.Y = bodyY + imgY + 4 + 28
		// Golden static mask: time-invariant BY DESIGN. Excluded: HUD band,
		// soak banner, scrolling list column, HOT spot, overlay corner zone.
		goldenRects = []wrsoak.Rect{
			{X: 0, Y: 0, W: winW, H: 48},
			{X: shell.Legend.X, Y: shell.Legend.Y, W: shell.Legend.W, H: shell.Legend.H},
			{X: dx - 2, Y: bodyY + dense.Offset().Y - 2, W: 206, H: 206},
			{X: nx - 2, Y: ny - 2, W: 146, H: 176},
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
		ovEntry = overlay.NewEntry(panel, ovWinX, ovWinY, ovW, ovH)
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

	app.Scheduler().Tickers().Add(&wrsoak.Ticker{On: func(dt float64) {
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
	finalChecks := []wrsoak.PixelCheck{
		{Pt: &ptNestInner, Desc: "nested_inner_intact_over_soak"},
		{Pt: &ptListRow, Desc: "scrolled_in_row_content_correct"},
		{Pt: &ptOverlayClosed, Desc: "overlay_zone_closed_no_residue"},
		{Pt: &ptImgCell, Desc: "image_cell0_stable_over_soak"},
		{Text: &capTextBox, Base: denseBase, Desc: "static_note_text_density"},
	}
	wrsoak.RunPixelChecks("ui_wr_c10_soak", winW, wrsoak.LoadImage(filepath.Join(snapDir, "c10_final.png")), finalChecks, pixelResult)

	scriptedOK, scriptedTotal := 0, len(pixelResult)
	for _, ok := range pixelResult {
		if ok {
			scriptedOK++
		}
	}

	goldenDiffPct, goldenTotalPx, goldenFirstRun := wrsoak.EvaluateGolden("ui_wr_c10_soak", snapDir, "c10_final.png", "c10_final_base.png", goldenRects, winW)

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
			"phases_seen":                 wrsoak.SortedKeys(phasesSeen),
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
	if wrsoak.FpsOf(snap.AvgFrameIntervalMs) < 55 {
		fmt.Fprintf(os.Stderr, "FAIL: fps_interval=%.1f < 55 (持续 tick C10)", wrsoak.FpsOf(snap.AvgFrameIntervalMs))
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
		app.PresentCount(), wrsoak.FpsOf(snap.AvgFrameIntervalMs), snap.P95FrameIntervalMs, snap.HitchRatePerMin,
		snap.BindCount, snap.ItemCount, snap.ScrollRerecord, overlayOpens,
		snap.RSSSlopeKBPerMin, snap.CPUPctAvg,
		snap.BoundarySkip, scriptedOK, scriptedTotal, goldenDiffPct, goldenTotalPx, goldenFirstRun,
		snap.VSyncSource, elapsedSec)
}

// Assertion plumbing (pixel/Golden/tick) is shared with R15 in examples/wrsoak.
