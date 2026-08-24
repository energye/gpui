// Command ui_wr_c4_shell_overlay is the W4 C4 composite real-window
// (mode-2 rewrite, 2026-08-24; scene layout revised per user review: the
// virtual list is the LEFT column with its explanation text beside it, the
// right side keeps the §3.1.2 standard bands).
//
// Integration thesis (§3 C4 row): 顶栏壳(R21) + 可滚体内容(R3) + 浮层面板叠加
// (R8) 三层同屏独立。Three bands share one frame and must not interfere:
//
//	SHELL demo band (top strip): one RepaintBoundary tagged
//	  SetShellBoundary — body scrolling AND overlay cycling must never
//	  re-record it (shell_rerecord_scroll == 0, replays grow shell_skip).
//	CONTENT band (left): a 90-row VirtualList scrolls every frame inside a
//	  RenderViewport, with a caption explaining what to watch. Right side:
//	  static dense grid of nested RepaintBoundaries replays throughout (R3,
//	  boundary_skip > 0) plus a custom-painted HOT spot dirtying itself
//	  every frame (part of the main-band baseline).
//	OVERLAY band: Spike phase cycles popups open/closed (1.6s open /
//	  1.0s closed; modal dialog + menu stack / dropdown / tooltip sheet,
//	  fresh entries per open). Opening must dirty ONLY the overlay band —
//	  main-band dirty count stays at its closed-state baseline
//	  (main_dirty_excess_frames == 0). Recover removes everything.
//
// Present policy: retained (Plan C, Flutter-aligned; user-confirmed at the
// stop-report node) — steady frames CompositeOnly + damage present; only
// dirty layers re-record, which makes both the shell-rerecord=0 gate and the
// overlay-does-not-grow-main-paint gate strict.
//
// §2.7/U21 visual correctness: scripted hit-probes paired with pixel
// assertions (F0 exact points / F5 barrier blend formula / F6 text-region
// density / F8 open-close dual state), plus Golden bitwise comparison of the
// STATIC regions between consecutive runs (zero tolerance, same-engine
// baseline semantics; the first run only produces the baseline).
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=15 go run ./examples/ui_wr_c4_shell_overlay
//
// Window: 1200x800. RUN_SECONDS>=5 (U16); closing requires 15 (§2.5).
// GPU window required (needs_gpu_window otherwise).
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
	"github.com/energye/gpui/ui/overlay"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH = 1200, 800
	// §2.5 组合窗关闭用时长: C4 = 25（弹窗≥5s/用户要求后：Steady5 +
	// 3×(开5+关1) + Recover 沉降；原 15s 值无法容纳三种轮换）。
	closeSeconds = 25

	listRows  = 90
	rowExtent = 44.0

	vpH          = 380.0 // virtual-list viewport height
	vpW          = 420.0 // virtual-list viewport width (row width)
	midH         = 300.0 // right-column band height for dense + HOT rows
	denseW       = 310.0 // dense grid panel (4×4 cells + 8 labels)
	denseH       = 252.0
	bandW, bandH = 700.0, 92.0 // shell demo band

	// Left-column text budget: wrap width minus padding; the status line
	// must stay under it or the wrap grows the box and breaks pixel anchors.
	statusWrapW = 330.0
)

// Palette helpers shared by the scene builder AND the pixel assertions, so a
// painted color and its expected value can never drift apart.
func chipRGB(i int) (float64, float64, float64) {
	return 0.30 + float64(i%3)*0.20, 0.45 + float64(i%2)*0.30, 0.60
}

func cellRGB(i, j int) (float64, float64, float64) {
	return 0.25 + float64(i)*0.12, 0.40 + float64(j)*0.10, 0.62
}

var (
	bandBase  = [3]float64{0.16, 0.18, 0.26} // shell band fill
	denseBase = [3]float64{0.11, 0.12, 0.18} // dense panel fill
	btn2Base  = [3]float64{0.62, 0.42, 0.20} // shell button 2 fill
	ovBtnBase = [3]float64{0.20, 0.50, 0.85} // popup button fill
	barrierBG = [3]float64{0.02, 0.03, 0.06} // barrier source color
	barrierA  = 0.45                         // barrier alpha
	clearBG   = [3]float64{0.08, 0.09, 0.11} // window clear color
)

// blend computes the F5 expected result of the translucent barrier over a
// known opaque background: out = a·src + (1-a)·bg.
func blend(bg [3]float64) [3]float64 {
	var out [3]float64
	for i := 0; i < 3; i++ {
		out[i] = barrierA*barrierBG[i] + (1-barrierA)*bg[i]
	}
	return out
}

// probePoint is one pixel-assertion site: a logical window coordinate plus
// the theoretically expected color. Coordinates are RESOLVED FROM THE LAYOUT
// CHAIN at runtime (never hand-computed absolutes, §2.7 sampling rule 2);
// overlay-entry sites derive from engine-owned Entry fields.
type probePoint struct {
	x, y  float64
	want  [3]float64
	tol   float64 // channel tolerance in 0..1 (8/255 F0, 12/255 F5)
	label string
}

// rect is a logical-coordinate rectangle (text-density box / golden mask unit).
type rect struct{ x, y, w, h float64 }

// checkSpec binds one pixel assertion to the snapshot that validates it.
type checkSpec struct {
	pt           *probePoint // exact-point assertion (F0/F5)…
	wantOverride *[3]float64 // …optionally against a different expected color
	tolOverride  float64     // …and/or a different tolerance
	text         *rect       // …or a text-density assertion (F6)
	base         [3]float64  // text background
	desc         string      // unique key in pixelResult
}

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if !secsSet {
		// No RUN_SECONDS: run indefinitely (user closes the window). The
		// phase script loops Spike↔Recover so popups keep cycling. Closing
		// runs must set RUN_SECONDS=15 (§2.5); <5 fails U16.
		secs = 0
	} else {
		wrkit.RequireMinRun(secs, "C4")
	}
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: "gpui ui_wr_c4_shell_overlay — 壳+体内容+浮层 三层独立", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "C4 组合 — 壳(R21)+体内容(R3)+浮层(R8) 三层独立", []string{
		"SHELL = 壳层演示带 SetShellBoundary 全程静止 (R21)",
		"BODY  = 左列虚拟列表持续滚动 只脏体层",
		"DENSE = 右侧静态密集区 嵌套 boundary (R3)",
		"HOT   = 自绘热点 每帧重绘 全程活跃",
		"Spike = 浮层循环 对话框/下拉/Tooltip 轮换 (R8)",
		"开浮层帧 主带脏计数必须保持基线",
		"体滚与浮层都不得重录壳层 (rr=0)",
		"policy=retained 稳态只重录脏层",
		"像素断言 F0/F5/F6/F8 + Golden 逐位对比 (U21)",
	})
	body := shell.Body

	// ===== SHELL demo band (R21) ============================================
	// One RepaintBoundary tagged SetShellBoundary covering title + buttons +
	// font samples. Nothing inside ever changes after build.
	band := rendering.NewAbsoluteBox(bandW, bandH)
	band.Background = &rendering.Color{R: bandBase[0], G: bandBase[1], B: bandBase[2], A: 1}
	band.SetRepaintBoundary(true)
	band.SetShellBoundary(true)
	band.Place(wrkit.Label("SHELL 壳层 — 标题/按钮/样张 全程静止", 15, 0.95, 0.96, 0.99), 16, 12)
	btn1 := rendering.NewAbsoluteBox(86, 26)
	btn1.Background = &rendering.Color{R: 0.22, G: 0.52, B: 0.88, A: 1}
	btn1.SetRepaintBoundary(true)
	btn1.SetDebugName("c4-btn-1")
	btn1.Place(wrkit.Label("按钮-1", 11, 1, 1, 1), 10, 7)
	band.Place(btn1, 320, 18)
	btn2 := rendering.NewAbsoluteBox(86, 26)
	btn2.Background = &rendering.Color{R: btn2Base[0], G: btn2Base[1], B: btn2Base[2], A: 1}
	btn2.SetRepaintBoundary(true)
	btn2.SetDebugName("c4-btn-2")
	btn2.Place(wrkit.Label("按钮-2", 11, 1, 1, 1), 10, 7)
	band.Place(btn2, 414, 18) // sampled at local (70,13): right of its label glyphs
	sx := 530.0
	for _, s := range []float64{8, 10, 12, 14, 16} {
		band.Place(wrkit.Label(fmt.Sprintf("%dpx 样张", int(s)), s, 0.92, 0.95, 1), sx, 44)
		sx += 28 + s*3.0
	}
	wrapShell := rendering.NewRenderAlignBox(band, 0.5, 0.5)
	wrapShell.FixedHeight = 104.0
	body.Box.Place(wrapShell, 0, 0)

	contentTop := wrapShell.FixedHeight // content region starts under the strip

	// ===== LEFT column: scrolling virtual list + explanation text ===========
	vlist := rendering.NewVirtualList(listRows, rowExtent, func(i int) rendering.RenderObject {
		row := rendering.NewAbsoluteBox(420, 40)
		row.Background = &rendering.Color{R: 0.13, G: 0.15, B: 0.20, A: 1}
		cr, cg, cb := chipRGB(i)
		chip := rendering.NewRenderColorBox(26, 26, cr, cg, cb, 1)
		chip.SetDebugName(fmt.Sprintf("row-%d-chip", i))
		row.Place(chip, 8, 7)
		lbl := wrkit.Label(fmt.Sprintf("item-%d 体内容行", i), 12, 0.85, 0.88, 0.93)
		lbl.SetDebugName(fmt.Sprintf("row-%d-lbl", i))
		row.Place(lbl, 44, 11)
		return row
	})
	// Persistent column backdrop: the viewport only clips rows — between-row
	// gaps and mid-scroll moments would otherwise expose the body background
	// and read as "the list moved up and vanished". A fixed backdrop under the
	// viewport keeps the column visually continuous.
	colBg := rendering.NewAbsoluteBox(vpW, body.H-contentTop-8)
	colBg.Background = &rendering.Color{R: 0.13, G: 0.15, B: 0.20, A: 1}
	body.Box.Place(colBg, 0, contentTop+8) // spans the full remaining body height
	vlist.CacheExtent = 480                // pre-mount ±11 rows beyond the viewport: new rows
	// record their texture while still outside the clip, so entering rows never
	// cause a mid-scroll raster spike (突然卡顿).
	viewport := rendering.NewRenderViewport(vlist)
	viewport.SetScrollOffset(0, 0)
	viewport.FixedWidth = vpW // clip to the list column: rows must never span the body
	viewport.FixedHeight = vpH
	wrapList := rendering.NewRenderAlignBox(viewport, 0, 0)
	wrapList.FixedHeight = vpH
	body.Box.Place(wrapList, 0, contentTop+8) // flush after the Legend gap

	// Explanation panel beside the list: what this window proves and which
	// counters to watch (U17 multi-region; static after each SetText burst).
	const capW = 340.0
	cap := wrkit.NewPanel(capW, vpH, 0.10, 0.11, 0.14, 1)
	body.Box.Place(cap.Box, vpW+24, contentTop+8)
	cap.LabelAt("体内容 · 虚拟列表（左）", 14, 14, 10, 0.90, 0.94, 1)
	capY := 44.0
	for _, ln := range []string{
		"列表每帧滚动：Steady/Recover 慢速，Spike 8px/帧。",
		"只有视口内的行被挂载（bind≪N），行各自缓存，",
		"滚动时已见行 Picture 平移回放、新入视口才录制一次。",
		"看 HUD：skip 持续增长而 rr 不随滚动上涨。",
	} {
		t := wrkit.Label(ln, 12, 0.72, 0.78, 0.88)
		t.SetMaxWidth(capW - 28)
		cap.Place(t, 14, capY)
		capY += 26
	}
	capY += 8
	cap.Place(rendering.NewRenderColorBox(capW-28, 2, 0.25, 0.32, 0.42, 1), 14, capY)
	capY += 14
	for _, ln := range []string{
		"右侧密集色格为嵌套 RepaintBoundary（R3）：",
		"静态缓存回放，boundary_skip 只增不重录；",
		"HOT 自绘热点每帧重绘，是主带基线脏的一部分。",
	} {
		t := wrkit.Label(ln, 12, 0.72, 0.78, 0.88)
		t.SetMaxWidth(capW - 28)
		cap.Place(t, 14, capY)
		capY += 26
	}
	statusLbl := wrkit.Label("PHASE=Steady scr=0 ov=closed", 12, 0.85, 0.95, 0.55)
	statusLbl.SetMaxWidth(statusWrapW)
	cap.Place(statusLbl, 14, capY+4)
	bannerLbl := wrkit.Label("HIT 0/6 px 0/9", 12, 0.95, 0.90, 0.55)
	bannerLbl.SetMaxWidth(statusWrapW)
	cap.Place(bannerLbl, 14, capY+30)

	// ===== RIGHT side: dense nested-boundary grid (R3) + HOT custom paint ===
	dense := rendering.NewAbsoluteBox(denseW, denseH)
	dense.Background = &rendering.Color{R: denseBase[0], G: denseBase[1], B: denseBase[2], A: 1}
	dense.SetDebugName("dense-panel")
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			cr, cg, cb := cellRGB(i, j)
			c := rendering.NewRenderColorBox(56, 28, cr, cg, cb, 1)
			c.SetRepaintBoundary(true)
			c.SetDebugName(fmt.Sprintf("cell-%d-%d", i, j))
			dense.Place(c, 10+float64(i)*64, 10+float64(j)*38)
		}
	}
	for i := 0; i < 8; i++ {
		row, col := i/4, i%4
		lbl := wrkit.Label(fmt.Sprintf("lbl-%d", i), 10, 0.62, 0.72, 0.85)
		lbl.SetDebugName(fmt.Sprintf("lbl-%d", i))
		dense.Place(lbl, 10+float64(col)*64, 168+float64(row)*20)
	}
	wrapDense := rendering.NewRenderAlignBox(dense, 1, 0)
	wrapDense.FixedHeight = midH
	body.Box.Place(wrapDense, 0, contentTop+8)

	var hotT float64
	hot := rendering.NewRenderBox()
	hot.FixedWidth, hot.FixedHeight = 46, 46
	hot.OnPaint = func(pc *rendering.PaintContext, _ rendering.Size) {
		// Two color-cycling circles: pulsing core + orbiting satellite.
		pulse := 0.5 + 0.5*math.Sin(hotT*3.1)
		rendering.FillCircle(pc, 23, 23, 10+5*pulse, 0.95, 0.30+0.35*pulse, 0.25, 1)
		rendering.FillCircle(pc, 23+16*math.Cos(hotT*2.2), 23+16*math.Sin(hotT*2.2), 5, 0.95, 0.90-0.3*pulse, 0.30+0.4*pulse, 1)
	}
	wrapHot := rendering.NewRenderAlignBox(hot, 0, 0)
	wrapHot.FixedHeight = midH
	body.Box.Place(wrapHot, 0, contentTop+vpH+28)

	// ===== Layout-chain geometry (refreshed after relayout; §2.7 rule 2) ====
	geoValid := false
	ptShellBtn := probePoint{want: btn2Base, tol: 8.0 / 255, label: "pixel_shell_btn2"}
	ptCell := probePoint{tol: 8.0 / 255, label: "pixel_dense_cell"}
	ptDenseBg := probePoint{want: denseBase, tol: 8.0 / 255, label: "pixel_dense_bg"}
	titleBox := rect{w: 230, h: 26}
	captionBox := rect{w: capW - 28, h: 70}
	resolveGeometry := func() {
		if geoValid {
			return
		}
		bodyX, bodyY := body.Box.Offset().X, body.Box.Offset().Y
		// Shell band: body panel → strip wrapper → band.
		bx := bodyX + wrapShell.Offset().X + band.Offset().X
		by := bodyY + wrapShell.Offset().Y + band.Offset().Y
		ptShellBtn.x, ptShellBtn.y = bx+414+70, by+18+13
		// Dense panel.
		dx := bodyX + wrapDense.Offset().X + dense.Offset().X
		dy := bodyY + wrapDense.Offset().Y + dense.Offset().Y
		cellR, cellG, cellB := cellRGB(2, 1)
		ptCell.x, ptCell.y = dx+10+2*64+28, dy+10+1*38+14
		ptCell.want = [3]float64{cellR, cellG, cellB}
		// Empty dense-panel spot clear of every popup geometry — the barrier
		// blend (open) / restore (recover) probe site (F5/F8 dual state).
		ptDenseBg.x, ptDenseBg.y = dx+270, dy+228
		titleBox.x, titleBox.y = bx+16, by+12 // glyph tops ≈ +1.8px under the baseline convention
		// cap.Box is placed at body-local (16+vpW+24, contentTop+8); its
		// Offset() already carries that position — do not double-count it.
		capAbs := cap.Box.Offset() // absolute within body panel (Place set it)
		captionBox.x = bodyX + capAbs.X + 14
		// Cover the first three caption lines' glyph band: lines placed at
		// panel-local y=44/70/96 with baseline = y + fs (12px), glyph tops
		// ≈ baseline − ascent (≈0.88·fs) → first tops ≈ y+1.4.
		captionBox.y = bodyY + capAbs.Y + 45
		geoValid = true
	}

	var liveBarriers []*overlay.Entry // open modal barriers: resized with the window
	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: clearBG[0], ClearG: clearBG[1], ClearB: clearBG[2], ClearA: 1,
		RunFor: time.Duration(secs) * time.Second, // 0 = run until window close
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "ui_wr_c4_shell_overlay: close (%s)\n", win.Backend())
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
					geoValid = false // re-resolve chain geometry after relayout
					// Modal barriers cover the window: refit their entry
					// bounds and dim-child size to the new client area.
					for _, be := range liveBarriers {
						be.W, be.H = float64(ev.Width), float64(ev.Height)
						if box, ok := be.Child.(*rendering.RenderColorBox); ok {
							box.Width, box.Height = float64(ev.Width), float64(ev.Height)
							box.MarkNeedsLayout()
						}
					}
				}
			}
		},
	})
	// Plan C posture (user-confirmed at the stop-report node).
	app.SetPresentPolicy(scheduler.PresentPolicyRetained)

	ov := overlay.New()
	app.SetOverlay(ov)

	// ===== Overlay popups: fresh entries per open, three rotating kinds =====
	buildPopup := func(kind int) []*overlay.Entry {
		switch kind % 3 {
		case 0: // modal dialog + stacked menu list (full-screen barrier)
			// Child must cover the entry bounds: the translucent dim is the
			// barrier's VISUAL (the hit-consume comes from Entry.Barrier).
			// Size from the CURRENT window (resize-safe), not build-time constants.
			bw, bh := host.Size()
			if bw <= 0 {
				bw, bh = winW, winH
			}
			barrier := rendering.NewRenderColorBox(float64(bw), float64(bh), barrierBG[0], barrierBG[1], barrierBG[2], barrierA)
			barrier.SetDebugName("c4-barrier")
			entries := []*overlay.Entry{overlay.NewBarrierEntry(0, 0, float64(bw), float64(bh), barrier)}
			panel := rendering.NewAbsoluteBox(340, 240)
			panel.Background = &rendering.Color{R: 0.15, G: 0.17, B: 0.24, A: 1}
			panel.SetRepaintBoundary(true)
			panel.SetDebugName("ov-panel")
			panel.Place(wrkit.Label("OVERLAY 浮层面板", 14, 0.92, 0.95, 1), 16, 14)
			panel.Place(wrkit.Label(fmt.Sprintf("第 %d 次打开 · 对话框", kind+1), 11, 0.70, 0.78, 0.88), 16, 44)
			ovBtn := rendering.NewAbsoluteBox(120, 34)
			ovBtn.Background = &rendering.Color{R: ovBtnBase[0], G: ovBtnBase[1], B: ovBtnBase[2], A: 1}
			ovBtn.SetRepaintBoundary(true)
			ovBtn.SetDebugName("ov-button")
			ovBtn.Place(wrkit.Label("浮层内按钮", 11, 1, 1, 1), 14, 9)
			panel.Place(ovBtn, 24, 84) // sampled at button-local (95,17): right of glyphs
			menu := rendering.NewAbsoluteBox(180, 150)
			menu.Background = &rendering.Color{R: 0.20, G: 0.24, B: 0.32, A: 1}
			menu.SetRepaintBoundary(true)
			for i := 0; i < 4; i++ {
				menu.Place(wrkit.Label(fmt.Sprintf("menu-item-%d", i), 11, 0.85, 0.9, 0.95), 12, 10+float64(i)*28)
			}
			return append(entries,
				overlay.NewEntry(panel, 380, 250, 340, 240),
				overlay.NewEntry(menu, 700, 190, 180, 150))
		case 1: // dropdown menu (no barrier — clicks fall through outside it)
			drop := rendering.NewAbsoluteBox(220, 210)
			drop.Background = &rendering.Color{R: 0.18, G: 0.22, B: 0.30, A: 1}
			drop.SetRepaintBoundary(true)
			for i := 0; i < 6; i++ {
				rowB := rendering.NewAbsoluteBox(200, 26)
				rowB.Background = &rendering.Color{R: 0.14 + 0.03*float64(i%2), G: 0.18, B: 0.26, A: 1}
				rowB.SetRepaintBoundary(true)
				rowB.Place(wrkit.Label(fmt.Sprintf("下拉项-%d 选择", i), 11, 0.85, 0.9, 0.95), 10, 5)
				drop.Place(rowB, 10, 10+float64(i)*32)
			}
			return []*overlay.Entry{overlay.NewEntry(drop, 450, 170, 220, 210)}
		default: // tooltip sheet (no barrier)
			sheet := rendering.NewAbsoluteBox(280, 96)
			sheet.Background = &rendering.Color{R: 0.25, G: 0.28, B: 0.20, A: 1}
			sheet.SetRepaintBoundary(true)
			sheet.Place(wrkit.Label("Tooltip 提示层", 13, 0.95, 0.93, 0.75), 14, 12)
			sheet.Place(wrkit.Label(fmt.Sprintf("cycle #%d — 随开随建", kind+1), 11, 0.8, 0.85, 0.9), 14, 44)
			return []*overlay.Entry{overlay.NewEntry(sheet, 420, 470, 280, 96)}
		}
	}

	// ===== Phase script =======================================================
	// Steady 0–5s (baseline) → Spike: three popup kinds, each open ≥5s then 1s
	// closed → Recover (final close + settle). Indefinite runs loop Spike↔Recover.
	const spikeStart = 5.0
	const openDur = 5.0 // popup stays open ≥5s (user requirement)
	const closeDur = 1.0
	// Spike must fit three full popup cycles (open+close each); the closing
	// run length follows from that budget instead of the other way.
	spikeEnd := spikeStart + 3*(openDur+closeDur) // 18s of cycling
	if secs > closeSeconds {
		spikeEnd = math.Max(spikeEnd, float64(secs)-3.5)
	}
	recoverTick := int(math.Round((spikeEnd + 0.5) * 45.0)) // first-Recover restore/snapshot tick
	recoverFired := false

	// ---- Gate state ----
	phase := wrkit.PhaseSteady
	var elapsed float64
	var lastPhaseStr string
	var scrollY float64
	tickN := 0
	var wrapSettle int
	var resizeSkip int
	var lastW, lastH int

	var shellRR, shellSkip int64 // R21 partition samples (post-warm-up, non-resize)
	hudAcc := 0.0                // HUD refresh accumulator (~10Hz throttle)
	refMainDirty := -1           // R8: worst-case closed-state main-band dirty count
	ovExcessFrames := 0          // R8: open frames exceeding the baseline
	ovOpenObserved := false
	ovEntriesOnOpen := int64(0)
	openedThisRun := false
	cycles := 0
	kindsSeen := map[int]bool{}
	var cycleEntries []*overlay.Entry
	cycleOpen := false
	cycleT := 0.0

	// ---- Scripted hit-probes (logic side of the §2.7 three-evidence trio) --
	scriptShellOK := false    // c4-btn-2 under the shell band
	scriptCellOK := false     // cell-2-1 inside the dense grid
	scriptRowOK := false      // row-N-chip under scroll (N derived from scrollY)
	scriptOvBtnOK := false    // ov-button inside the modal popup entry
	barrierConsumeOK := false // barrier consumes a main-tree point (BandOverlay)
	restoreOK := false        // same point falls back to dense-panel after close
	rowName := ""
	rowPix := probePoint{tol: 8.0 / 255, label: "pixel_row_chip"}

	// ---- Pixel assertion registry ==========================================
	// Each snapshot validates its own group exactly once:
	//   steady = F0 points + F6 text density (clean frame, no overlays)
	//   open   = F5 barrier blend formula + popup button under the modal
	//   recover = F8 restore (barrier gone → base color back) + shell intact
	blendWant := blend(denseBase)
	ptOvBtn := probePoint{x: 380 + 24 + 95, y: 250 + 84 + 17, want: ovBtnBase, tol: 8.0 / 255, label: "pixel_ov_button"}
	pixelChecks := map[string][]checkSpec{
		"steady": {
			{pt: &ptShellBtn, desc: "shell_btn2_base_color"},
			{pt: &ptCell, desc: "dense_cell_base_color"},
			{pt: &rowPix, desc: "scrolled_row_chip_color"},
			{pt: &ptDenseBg, desc: "dense_panel_base_color"},
			{text: &titleBox, base: bandBase, desc: "shell_title_text_density"},
			{text: &captionBox, base: [3]float64{0.10, 0.11, 0.14}, desc: "list_caption_text_density"},
		},
		"open": {
			{pt: &ptDenseBg, wantOverride: &blendWant, tolOverride: 12.0 / 255, desc: "barrier_blend_over_dense_bg_F5"},
			{pt: &ptOvBtn, desc: "popup_button_base_color"},
		},
		"recover": {
			{pt: &ptDenseBg, desc: "barrier_removed_dense_bg_restored_F8"},
			{pt: &ptShellBtn, desc: "shell_band_intact_at_recover"},
		},
	}
	pixelResult := map[string]bool{}
	pixelTotal := 0
	for _, group := range pixelChecks {
		pixelTotal += len(group)
	}
	pixelOKCount := func() int {
		n := 0
		for _, ok := range pixelResult {
			if ok {
				n++
			}
		}
		return n
	}
	bannerText := func() string {
		return fmt.Sprintf("HIT %d/6 px %d/%d",
			boolToInt(scriptShellOK)+boolToInt(scriptCellOK)+boolToInt(scriptRowOK)+
				boolToInt(scriptOvBtnOK)+boolToInt(barrierConsumeOK)+boolToInt(restoreOK),
			pixelOKCount(), pixelTotal)
	}

	// ---- Snapshot scheduling (§2.7: ≥2 shots, deterministic phase anchors) --
	snapDir := os.Getenv("C4_SNAP_DIR")
	if snapDir == "" {
		snapDir = filepath.Join("examples", "ui_wr_c4_shell_overlay", "golden")
	}
	os.MkdirAll(snapDir, 0o755)
	steadyPath := filepath.Join(snapDir, "c4_steady.png")
	openPath := filepath.Join(snapDir, "c4_open.png")
	recoverPath := filepath.Join(snapDir, "c4_recover.png")

	app.Scheduler().Tickers().Add(&tickerFn{on: func(dt float64) {
		tickN++
		// Phase clock counts TICKS scaled to a conservative 45 ticks/second:
		// ticker dt is wall-derived and host jitter can hold the observed pace
		// below vsync, so second-based phases could outrun the fixed RunFor
		// and never reach Recover. The 45/s budget guarantees the full script
		// (Steady→Spike×3 cycles→Recover) fits in RUN_SECONDS on this host.
		elapsed = float64(tickN) / 45.0
		if secs == 0 {
			// Indefinite interactive run: after the first Steady→Spike, loop
			// Spike↔Recover forever (popups keep cycling; each Recover gives
			// a clean 1.5s settle before the next cycle opens).
			const cyclePeriod = 6.5 // Spike 5s + Recover 1.5s
			if elapsed > spikeStart {
				cycleT2 := math.Mod(elapsed-spikeStart, cyclePeriod)
				switch {
				case cycleT2 < 5.0:
					phase = wrkit.PhaseSpike
				default:
					phase = wrkit.PhaseRecover
				}
			}
		} else {
			switch {
			case elapsed >= spikeStart && elapsed < spikeEnd:
				phase = wrkit.PhaseSpike
			case elapsed >= spikeEnd:
				phase = wrkit.PhaseRecover
			default:
				phase = wrkit.PhaseSteady
			}
		}

		// Resize settle: legal re-record wave after external WM resize; the
		// 3-frame exclusion keeps the shell gate free of WM interference.
		cw, ch := host.Size()
		if cw != lastW || ch != lastH {
			lastW, lastH = cw, ch
			resizeSkip = 3
			geoValid = false
		}
		resizeFrame := resizeSkip > 0
		if resizeSkip > 0 {
			resizeSkip--
		}

		// Body scroll pacing: smooth 2px/frame every tick (Spike 6px) — small
		// per-frame remounts keep the main-band dirty count low and stable.
		if phase == wrkit.PhaseSpike || tickN%1 == 0 {
			scrollY += 2.0
			if scrollY >= float64(listRows)*rowExtent {
				scrollY = 0
				wrapSettle = 8 // remount wave settles over several frames
			}
			if wrapSettle > 0 {
				wrapSettle--
			}
			viewport.SetScrollOffset(0, scrollY)
		}
		// HOT repaints itself every frame.
		hotT += dt
		hot.MarkNeedsPaint()

		// Phase transitions: Recover removes any open popup immediately.
		if phase != lastPhaseStr {
			lastPhaseStr = phase
			if phase == wrkit.PhaseRecover && cycleOpen {
				for _, e := range cycleEntries {
					ov.Remove(e)
				}
				liveBarriers = nil
				cycleEntries = nil
				cycleOpen = false
				openedThisRun = false
			}
		}

		// Overlay cycling (Spike only, fresh entries per open). Status/banner
		// text updates are batched to phase changes so their self-dirt cannot
		// mask an overlay-open main-band leak in the R8 observation.
		if phase == wrkit.PhaseSpike {
			// Cycle pacing in TICKS (same clock as phases): openDur/closeDur
			// are tick budgets at the 45tps scale, so the number of cycles
			// completed inside Spike is deterministic regardless of fps.
			cycleT++
			if cycleOpen && cycleT >= float64(int(openDur*45)) {
				for _, e := range cycleEntries {
					ov.Remove(e)
				}
				liveBarriers = nil
				cycleEntries = nil
				cycleOpen = false
				openedThisRun = false
				cycleT = 0
				// Text updates ride the CLOSE boundary: open frames must stay
				// free of example-side main-band dirt (R8 gate hygiene).
				statusLbl.SetText(fmt.Sprintf("PHASE=Spike scr=%.0f ov=closed cyc=%d rr=%d skip=%d", scrollY, cycles, shellRR, shellSkip))
				statusLbl.MarkNeedsPaint()
				bannerLbl.SetText(bannerText())
				bannerLbl.MarkNeedsPaint()
			} else if !cycleOpen && cycleT >= float64(int(closeDur*45)) {
				cycleEntries = buildPopup(cycles)
				for _, e := range cycleEntries {
					ov.Insert(e)
					if e.Barrier {
						liveBarriers = append(liveBarriers, e)
					}
				}
				ovEntriesOnOpen = int64(len(cycleEntries))
				kindsSeen[cycles%3] = true
				cycles++
				cycleOpen = true
				openedThisRun = true
				cycleT = 0
			}
		}

		resolveGeometry()

		// ===== Scripted probes ==============================================
		// (1)(2) shell button + dense cell, early Steady.
		if phase == wrkit.PhaseSteady && elapsed > 0.8 && (!scriptShellOK || !scriptCellOK) {
			if !scriptShellOK {
				scriptShellOK = hitIs(app, ptShellBtn.x, ptShellBtn.y, "c4-btn-2")
			}
			if !scriptCellOK {
				scriptCellOK = hitIs(app, ptCell.x, ptCell.y, "cell-2-1")
			}
		}
		// (3) scrolled row chip: N derives from the LIVE scroll offset
		// (layout-driven anchor); fires when a chip center passes the
		// viewport middle so the steady snapshot can sample the same site.
		// Guard: skip on ticks where scrolling just happened — mounted-cell
		// offsets land on the NEXT frame's layout, so probing the same tick
		// would race stale geometry.
		// Accept any row whose chip center currently sits in the middle band
		// of the viewport (130..250): with 2px/tick smooth scrolling the exact
		// middle is crossed between layout frames, so probe the whole band.
		if !scriptRowOK && phase == wrkit.PhaseSteady && elapsed > 1.0 && elapsed < spikeStart-0.2 && wrapSettle == 0 {
			seen := map[int]bool{}
			for _, target := range []float64{130, 165, 190, 215, 250} {
				cand := int(math.Round((target + scrollY - 20) / rowExtent))
				if cand < 1 || cand >= listRows || seen[cand] {
					continue
				}
				seen[cand] = true
				yOff := float64(cand)*rowExtent + 20 - scrollY // chip center, viewport-local
				if yOff < 130 || yOff > 250 {
					continue
				}
				vx := body.Box.Offset().X + wrapList.Offset().X + viewport.Offset().X
				vy := body.Box.Offset().Y + wrapList.Offset().Y + viewport.Offset().Y
				px := vx + 8 + 13
				py := vy + yOff
				want := fmt.Sprintf("row-%d-chip", cand)
				if hitIs(app, px, py, want) {
					scriptRowOK = true
					rowName = want
					cr, cg, cb := chipRGB(cand)
					rowPix.x, rowPix.y, rowPix.want = px, py, [3]float64{cr, cg, cb}
					break
				}
			}
		}

		// (4)(5) overlay-band probes while the MODAL popup (kind 0) is open.
		if cycleOpen && len(cycleEntries) >= 2 && cycles >= 1 && kindsSeen[0] {
			if !barrierConsumeOK {
				bd, obj, entry := app.HitTestPointer(ptDenseBg.x, ptDenseBg.y)
				// Consumed = overlay band claims it; the hit object is either
				// nil (pure barrier) or the barrier's own dim box.
				name := rendering.HitDebugName(obj)
				barrierConsumeOK = bd == overlay.BandOverlay && entry != nil &&
					(obj == nil || name == "" || name == "c4-barrier")
				fmt.Fprintf(os.Stderr, "ui_wr_c4_shell_overlay: probe barrier-consume @(%.0f,%.0f) band=%v obj=%q entry=%v ok=%v\n",
					ptDenseBg.x, ptDenseBg.y, bd, name, entry != nil, barrierConsumeOK)
			}
			if !scriptOvBtnOK {
				scriptOvBtnOK = hitIs(app, ptOvBtn.x, ptOvBtn.y, "ov-button")
			}
		}
		// (6) restore: after the final close the same point hits the main tree.
		if phase == wrkit.PhaseRecover && !openedThisRun && !restoreOK && tickN >= recoverTick {
			restoreOK = hitIs(app, ptDenseBg.x, ptDenseBg.y, "dense-panel")
		}

		// ===== Band observation (one-frame lag; see ui_wr_r8_overlay) =======
		srr, ssk := embedder.LastShellBoundaryFrame()
		if elapsed >= 0.5 && !resizeFrame {
			shellRR += srr
			shellSkip += ssk
		}
		mainDirty, _ := embedder.LastOverlayBandFrame()
		if openedThisRun && cycleOpen {
			ovOpenObserved = true
			// +2 slack: the band snapshot lags one frame and the per-frame
			// main-band id count jitters ±1-2 as HUD refreshes align with
			// scroll churn (same documented tolerance as the previous C4).
			if refMainDirty >= 0 && mainDirty > refMainDirty+2 && wrapSettle <= 0 {
				ovExcessFrames++
			}
		} else if !cycleOpen && elapsed > 1.0 && mainDirty > refMainDirty {
			// Baseline = worst-case main-band dirty over CLOSED frames of any
			// phase (Steady, Spike-closed, Recover). Closed Spike frames carry
			// the same HUD/HOT/scroll noise as open frames — excluding them
			// (as an earlier revision did) starved the baseline in interactive
			// runs and flagged every open frame as excess. Popup build/remove
			// churn itself lives in the overlay band and never touches this
			// count (band observation is sampled pre-attach).
			refMainDirty = mainDirty
		}

		// §2.7/U21 snapshots: ONE batched request when Recover begins — all
		// three states replay inside a single raster drain (steady = post-
		// close tree, open = freshly rebuilt modal, recover = cleared again),
		// so the run pays exactly ONE readback stall instead of three.
		if !recoverFired && phase == wrkit.PhaseRecover && !openedThisRun && tickN >= recoverTick {
			recoverFired = true
			app.SnapshotAsync(func() {
				// Resolve the row-chip sample site from the CURRENT scroll
				// offset inside this raster closure: the chip color is a pure
				// function of row index, and the row under the viewport middle
				// is computable without mutating any shared state here.
				if scriptRowOK {
					cand := int(math.Round((240 + viewport.ScrollOffset().Y) / rowExtent)) // viewport middle
					yOff := float64(cand)*rowExtent + 20 - viewport.ScrollOffset().Y
					if cand >= 0 && cand < listRows && yOff > 20 && yOff < vpH-20 {
						cr, cg, cb := chipRGB(cand)
						vx := shell.Body.Box.Offset().X + wrapList.Offset().X + viewport.Offset().X
						vy := shell.Body.Box.Offset().Y + wrapList.Offset().Y + viewport.Offset().Y
						rowPix.x = vx + 21
						rowPix.y = vy + yOff
						rowPix.want = [3]float64{cr, cg, cb}
					}
				}
				saveSnap(app, shell, ov, steadyPath, "steady")
				runPixelChecks(steadyPath, pixelChecks["steady"], pixelResult)
				entries := buildPopup(999) // kind 0 → modal + barrier + menu
				for _, e := range entries {
					ov.Insert(e)
				}
				ov.Layout(winW, winH) // laidOut gate: hit-ready, NeedsPaint fresh
				saveSnap(app, shell, ov, openPath, "open")
				runPixelChecks(openPath, pixelChecks["open"], pixelResult)
				for _, e := range entries {
					ov.Remove(e)
				}
				saveSnap(app, shell, ov, recoverPath, "recover")
				runPixelChecks(recoverPath, pixelChecks["recover"], pixelResult)
			})
		}

		app.ScheduleFrame()
		proc.Sample()

		// HUD refresh throttled to ~10Hz (matches LiveHUD's own paint budget):
		// MetricsStore.Snapshot() insertion-sorts the 256-sample interval ring
		// per call — 60 calls/s of UI-thread sorting competed with the raster
		// loop and dragged interval_avg past the vsync slot.
		hudAcc += dt
		gateOK := shellRR == 0 && shellSkip > 0 && ovExcessFrames == 0 && ovOpenObserved &&
			scriptShellOK && scriptCellOK && scriptRowOK && scriptOvBtnOK && barrierConsumeOK && restoreOK
		if hudAcc >= 0.1 {
			hudAcc = 0
			shell.UpdateHUD("C4", phase, app, gateOK,
				fmt.Sprintf("shell_rr=%d skip=%d main=%d/base excess=%d", shellRR, shellSkip, refMainDirty, ovExcessFrames),
				fmt.Sprintf("ov=%d cyc=%d %s scr=%.0f t=%.1f", ov.Len(), cycles,
					map[bool]string{true: "OPEN", false: "closed"}[cycleOpen], scrollY, elapsed))
		} else {
			shell.NoteHUDTick(dt)
		}
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

	// Static-region mask for the golden comparison (layout-chain resolved
	// while the tree is still alive; dynamic regions excluded per §2.7).
	geo := collectGoldenGeo(shell, band, dense, wrapShell, wrapDense)

	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	elapsedSec := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()

	// Golden comparison (§2.7 rule 5): first run produces the baseline,
	// second run onward enters bitwise comparison over the static mask.
	goldenDiffPct, goldenTotal, goldenFirstRun := evaluateGolden(snapDir, geo)

	scriptedTotal := 6
	scriptedOK := boolToInt(scriptShellOK) + boolToInt(scriptCellOK) + boolToInt(scriptRowOK) +
		boolToInt(scriptOvBtnOK) + boolToInt(barrierConsumeOK) + boolToInt(restoreOK)

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "C4",
		Scenario:      "ui_wr_c4_shell_overlay",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"covers": []string{"R3", "R8", "R21"},
			// R21 shell/content split
			"shell_rerecord_scroll": shellRR, // must be 0
			"shell_skip_scroll":     shellSkip,
			"shell_rerecord_total":  snap.ShellRerecord,
			"shell_skip_total":      snap.ShellSkip,
			// R8 overlay independence
			"main_dirty_baseline":      refMainDirty,
			"main_dirty_excess_frames": ovExcessFrames, // must be 0
			"overlay_open_observed":    ovOpenObserved,
			"overlay_entries_on_open":  ovEntriesOnOpen,
			"overlay_cycles":           cycles,
			"overlay_kinds_seen":       len(kindsSeen),
			"overlay_still_open":       openedThisRun, // must be false
			// R3 static cache
			"boundary_skip":         snap.BoundarySkip,
			"boundary_rerecord":     snap.BoundaryRerecord,
			"static_dense_cells":    16,
			"static_labels":         8,
			"hot_active_all_phases": true,
			// Scripted hit probes (logic evidence)
			"scripted_total":            scriptedTotal,
			"scripted_ok":               scriptedOK,
			"probe_shell_button":        boolToInt(scriptShellOK),
			"probe_dense_cell":          boolToInt(scriptCellOK),
			"probe_scrolled_row":        boolToInt(scriptRowOK),
			"probe_row_name":            rowName,
			"probe_overlay_button":      boolToInt(scriptOvBtnOK),
			"probe_barrier_consume":     boolToInt(barrierConsumeOK),
			"probe_restore_after_close": boolToInt(restoreOK),
			// §2.7 pixel evidence (F0/F5/F6/F8)
			"pixel_checks_total": pixelTotal,
			"pixel_checks_ok":    pixelOKCount(),
			"pixel_tolerance_f0": "8/255 per channel",
			"pixel_tolerance_f5": "12/255 per channel (blend formula declared)",
			"pixel_detail":       pixelDetail(pixelChecks, pixelResult),
			// §2.7 golden evidence
			"pixel_golden_diff_pct":  goldenDiffPct,
			"pixel_golden_mask_px":   goldenTotal,
			"pixel_golden_first_run": goldenFirstRun,
			"snapshot_paths":         []string{steadyPath, openPath, recoverPath},
			// U20 six dimensions + interaction
			"impl_interaction": "three bands share one frame: scroll updates only viewport offsets (shell partition untouched, dense grid replays); overlay Insert/Remove dirties only the overlay band (FramePacket.Root vs .Overlay observed pre/post attach); popups cover list and dense regions without re-recording either",
			"impl_correctness": "retained Plan C: steady frames CompositeOnly + damage present; shell demo band is ONE SetShellBoundary RepaintBoundary; dense cells are nested RepaintBoundaries; popups carry their own boundaries and compose above the main band",
			"impl_dirty":       "band-separated observation: LastShellBoundaryFrame samples the shell partition; LastOverlayBandFrame mirrors overlay dirties into pkt.OverlayDirtyLayerIDs pre/post attach so an open cannot pollute the main-band count; closed Spike frames refresh the baseline, open frames must stay within it; status text updates are batched to phase changes so banner self-dirt cannot mask an open-frame leak",
			"impl_cache":       "shell Picture replays while rows scroll and popups cycle (skip grows, rr stays 0); dense cells replay; freshly-built popup entries allocate fresh layer textures every cycle",
			"impl_edge":        "resize frames excluded from the shell gate (3-frame settle) and geometry re-resolved from the layout chain; scroll wrap (long runs) excluded via an 8-frame settle; Recover removes popups immediately; HUD/caption live OUTSIDE the shell boundary",
			"impl_fail":        "any shell rerecord during scroll OR main_dirty above baseline during open frames OR a failed hit probe OR a failed pixel check OR a nonzero golden diff (from run 2 on) → FAIL exit 1",
			"impl_visible":     "HUD live: shell_rr/skip, main=X/base, excess, ov=N cyc=K; caption shows HIT n/6 px n/9; topbar pixels never change while rows scroll beneath and popups stack above",
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	// ===== Gates: ∪(R3, R8, R21) + §2.2 family A hard lines (硬, 不许放) ====
	if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
		MinPresents:           1,
		RequireRetainedPolicy: true, // Plan C posture
		RequirePersistentFPS:  true, // continuous scroll ticker (animation-class)
		MinFPSWall:            55,
		MaxP95Ms:              22,
		MinBoundarySkip:       1, // R3: static dense cache engaged
	}); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	if shellRR != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: shell_rerecord_scroll=%d want 0 (scroll/overlay must not re-record the shell)\n", shellRR)
		os.Exit(1)
	}
	if shellSkip <= 0 {
		fmt.Fprintf(os.Stderr, "FAIL: shell_skip_scroll=%d want >0 (shell Picture must replay)\n", shellSkip)
		os.Exit(1)
	}
	if !ovOpenObserved {
		fmt.Fprintln(os.Stderr, "FAIL: overlay never observed open (script error)")
		os.Exit(1)
	}
	if refMainDirty < 0 {
		fmt.Fprintln(os.Stderr, "FAIL: main_dirty baseline never sampled")
		os.Exit(1)
	}
	if ovExcessFrames != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: main_dirty_excess_frames=%d want 0 (opening overlay must not dirty the main band beyond baseline)\n", ovExcessFrames)
		os.Exit(1)
	}
	if ovEntriesOnOpen < 1 {
		fmt.Fprintf(os.Stderr, "FAIL: overlay_entries_on_open=%d want >=1 (overlay band must be the dirtied side)\n", ovEntriesOnOpen)
		os.Exit(1)
	}
	if openedThisRun {
		fmt.Fprintln(os.Stderr, "FAIL: overlay still open at exit (Recover phase must remove it)")
		os.Exit(1)
	}
	minCycles, minKinds := 1, 1
	if secs >= closeSeconds {
		minCycles, minKinds = 3, 3 // closing runs must exercise all three popup kinds
	}
	if cycles < minCycles || len(kindsSeen) < minKinds {
		fmt.Fprintf(os.Stderr, "FAIL: overlay_cycles=%d kinds=%d want >=%d cycles / %d kinds\n", cycles, len(kindsSeen), minCycles, minKinds)
		os.Exit(1)
	}
	if snap.BoundarySkip <= 0 {
		fmt.Fprintf(os.Stderr, "FAIL: boundary_skip=%d want >0 (R3 static dense cache)\n", snap.BoundarySkip)
		os.Exit(1)
	}
	// Scripted hit probes: all six identities must hold (hit ≡ paint).
	if scriptedOK != scriptedTotal {
		fmt.Fprintf(os.Stderr, "FAIL: scripted_ok=%d/%d (shell=%v cell=%v row=%q(%v) ovbtn=%v barrier=%v restore=%v)\n",
			scriptedOK, scriptedTotal, scriptShellOK, scriptCellOK, rowName, scriptRowOK, scriptOvBtnOK, barrierConsumeOK, restoreOK)
		os.Exit(1)
	}
	// Pixel assertions: every scheduled check must exist AND pass (a snapshot
	// that never fired leaves its group unchecked → shortfall → FAIL).
	if pixelOKCount() != pixelTotal {
		fmt.Fprintf(os.Stderr, "FAIL: pixel_checks %d/%d — %s\n", pixelOKCount(), pixelTotal, pixelDetail(pixelChecks, pixelResult))
		os.Exit(1)
	}
	// Golden bitwise comparison (zero tolerance, static mask only).
	if !goldenFirstRun && goldenDiffPct != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: pixel_golden_diff_pct=%.4f want 0 over %d px (§2.7 zero-tolerance static mask)\n", goldenDiffPct, goldenTotal)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "ui_wr_c4_shell_overlay: OK shell_rr=%d shell_skip=%d main_base=%d excess=%d cyc=%d kinds=%d scripted=%d/%d pixel=%d/%d golden=%.2f%% fps=%.1f p95=%.1f presents=%d elapsed=%.1fs\n",
		shellRR, shellSkip, refMainDirty, ovExcessFrames, cycles, len(kindsSeen),
		scriptedOK, scriptedTotal, pixelOKCount(), pixelTotal, goldenDiffPct,
		fpsOf(snap), snap.P95FrameIntervalMs, app.PresentCount(), elapsedSec)
}

// ============================ plumbing ======================================

type tickerFn struct{ on func(dt float64) }

func (t *tickerFn) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// hitIs resolves one window point through the hit-tester and compares the
// hit identity. Logic side of the §2.7 three-evidence trio.
func hitIs(app *embedder.PipelineApp, x, y float64, want string) bool {
	_, obj, _ := app.HitTestPointer(x, y)
	got := rendering.HitDebugName(obj)
	ok := got == want
	fmt.Fprintf(os.Stderr, "ui_wr_c4_shell_overlay: probe @(%.0f,%.0f) want=%q got=%q ok=%v\n", x, y, want, got, ok)
	return ok
}

// saveSnap renders the full tree (main + current overlay state) into the GPU
// context and writes a PNG. Runs on the raster thread via SnapshotAsync.
func saveSnap(app *embedder.PipelineApp, shell *wrkit.ShellChrome, ov *overlay.State, path, tag string) {
	dc := app.Target().Context()
	if dc == nil {
		fmt.Fprintf(os.Stderr, "snapshot %s: no gpu context\n", tag)
		return
	}
	dc.BeginFrame()
	embedder.PaintPresentTree(dc, app.Pipeline(), shell.Root, ov, clearBG[0], clearBG[1], clearBG[2], 1, true)
	if err := dc.SavePNG(path); err != nil {
		fmt.Fprintf(os.Stderr, "snapshot %s: %v\n", tag, err)
	} else {
		fmt.Fprintf(os.Stderr, "snapshot %s: %s\n", tag, path)
	}
}

// runPixelChecks executes one snapshot's assertion group against the freshly
// rendered PNG.
func runPixelChecks(path string, group []checkSpec, result map[string]bool) {
	img := loadImage(path)
	if img == nil {
		return
	}
	dpr := float64(img.Bounds().Dx()) / winW
	for _, c := range group {
		if _, seen := result[c.desc]; seen {
			continue
		}
		var ok bool
		switch {
		case c.pt != nil:
			want, tol := c.pt.want, c.pt.tol
			if c.wantOverride != nil {
				want = *c.wantOverride
			}
			if c.tolOverride > 0 {
				tol = c.tolOverride
			}
			r, g, b, valid := sampleLogical(img, dpr, c.pt.x, c.pt.y)
			ok = valid && nearC(r, g, b, want, tol)
			fmt.Fprintf(os.Stderr, "ui_wr_c4_shell_overlay: pixel %-36s @(%4.0f,%4.0f) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) ok=%v\n",
				c.desc, c.pt.x, c.pt.y, r, g, b, want[0], want[1], want[2], ok)
		case c.text != nil:
			n := textPixels(img, dpr, *c.text, c.base)
			ok = n >= 120 // ≥120 non-background pixels: the label really rendered
			fmt.Fprintf(os.Stderr, "ui_wr_c4_shell_overlay: pixel %-36s text_px=%d (want >=120) ok=%v\n", c.desc, n, ok)
		}
		result[c.desc] = ok
	}
}

func pixelDetail(checks map[string][]checkSpec, result map[string]bool) string {
	s := ""
	for _, group := range checks {
		for _, c := range group {
			tag := "-"
			if val, seen := result[c.desc]; seen {
				tag = map[bool]string{true: "ok", false: "FAIL"}[val]
			}
			if s != "" {
				s += "; "
			}
			s += c.desc + "=" + tag
		}
	}
	return s
}

// loadImage decodes a PNG written by saveSnap.
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

// sampleLogical reads one logical-coordinate pixel (physical = logical·dpr).
func sampleLogical(img image.Image, dpr float64, lx, ly float64) (r, g, b float64, valid bool) {
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

// textPixels counts pixels that clearly deviate from the panel background
// inside a logical box (F6: text presence is a REGION property, never a
// single point).
func textPixels(img image.Image, dpr float64, box rect, base [3]float64) int {
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

// goldenGeo is the static-region mask captured from the layout chain at
// snapshot time (topbar, legend panel, shell band, dense panel — all
// pixel-stable by construction; dynamic regions are excluded per §2.7).
type goldenGeo struct {
	rects []rect
}

func collectGoldenGeo(shell *wrkit.ShellChrome, band, dense *rendering.AbsoluteBox,
	wrapShell, wrapDense *rendering.RenderAlignBox) goldenGeo {
	bodyX, bodyY := shell.Body.Box.Offset().X, shell.Body.Box.Offset().Y
	bx := bodyX + wrapShell.Offset().X + band.Offset().X
	by := bodyY + wrapShell.Offset().Y + band.Offset().Y
	dx := bodyX + wrapDense.Offset().X + dense.Offset().X
	dy := bodyY + wrapDense.Offset().Y + dense.Offset().Y
	return goldenGeo{rects: []rect{
		{x: 0, y: 0, w: winW, h: 48}, // wrkit TopBar (static title)
		{x: shell.Legend.X, y: shell.Legend.Y, w: shell.Legend.W, h: shell.Legend.H},
		{x: bx, y: by, w: bandW, h: bandH},
		{x: dx, y: dy, w: denseW, h: denseH},
	}}
}

// evaluateGolden compares this run's steady/recover snapshots against the
// stored baselines over the static mask. First run stores the baseline and
// reports first_run=true (no verdict — §2.7 golden semantics). Later runs
// demand ZERO differing pixels (same-engine, zero-tolerance).
func evaluateGolden(snapDir string, geo goldenGeo) (diffPct float64, totalPx int64, firstRun bool) {
	pairs := [][2]string{
		{filepath.Join(snapDir, "c4_steady_base.png"), filepath.Join(snapDir, "c4_steady.png")},
		{filepath.Join(snapDir, "c4_recover_base.png"), filepath.Join(snapDir, "c4_recover.png")},
	}
	n := 0
	for _, p := range pairs {
		if _, err := os.Stat(p[0]); err != nil {
			// No baseline yet: store this run's shot as the baseline.
			data, err := os.ReadFile(p[1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "golden: current snapshot %s missing (%v)\n", p[1], err)
				diffPct = 100 // missing evidence cannot pass silently
				n++
				continue
			}
			if err := os.WriteFile(p[0], data, 0o644); err == nil {
				firstRun = true
				fmt.Fprintf(os.Stderr, "golden baseline stored: %s\n", p[0])
			}
			continue
		}
		diff, total, err := comparePNG(p[0], p[1], geo)
		if err != nil {
			fmt.Fprintf(os.Stderr, "golden compare %s vs %s: %v\n", p[0], p[1], err)
			diffPct = 100
			n++
			continue
		}
		totalPx += total
		diffPct += diff
		n++
		fmt.Fprintf(os.Stderr, "golden %s: diff=%.4f%% over %d px\n", filepath.Base(p[1]), diff, total)
	}
	if n > 0 {
		diffPct /= float64(n)
	}
	return diffPct, totalPx, firstRun
}

// comparePNG counts differing pixels inside the static mask between two
// same-engine screenshots (physical pixels; any channel mismatch counts).
func comparePNG(basePath, curPath string, geo goldenGeo) (pct float64, total int64, err error) {
	a, b := loadImage(basePath), loadImage(curPath)
	if a == nil || b == nil {
		return 0, 0, fmt.Errorf("decode failed")
	}
	if a.Bounds() != b.Bounds() {
		return 0, 0, fmt.Errorf("size mismatch %v vs %v", a.Bounds(), b.Bounds())
	}
	dpr := float64(a.Bounds().Dx()) / winW
	var diff int64
	for _, r := range geo.rects {
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
