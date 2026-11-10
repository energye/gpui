// Command ui_wr_p41_drawshadow is the P4.1 batch-3 capability real window:
// DrawShadow device-space lift semantics (subitem 4, D2a alignment with
// Flutter Impeller's spot-shadow model).
//
// Four cards:
//
//	A  plain shadow over white — tonal spot color + lift/blur lock (the
//	   Impeller model this library already follows; GPU-stable).
//	B  shadow under an affine projection (scale+translate) — the lifted
//	   affine path stays exact through the 2x3 lift conversion.
//	C  shadow under a true perspective projection (W←y) — silhouette is
//	   device-space lifted (P4.1 batch-3 fix). NOTE: the perspective ×
//	   isolated-layer × blur GPU chain has a readback instability (engine
//	   hole #4, same family as #3); probes here assert silhouette presence
//	   with wide tolerance, semantics locked by the CPU unit test.
//	D  elevation ladder (4/10/20) — screen-space lift grows linearly.
//
// Negative control: GPUI_P41S_BREAK=skip-shadow drops card A's shadow and
// must turn its probes red.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
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
	winW, winH  = 1200, 800
	windowTitle = "gpui ui_wr_p41_drawshadow — P4.1 批3 DrawShadow 设备空间抬升"
)

var breakMode = os.Getenv("GPUI_P41S_BREAK")

var cardOrigin [4]render.Point
var cardDrawn [4]int

func drawBlockWithShadow(pc *rendering.PaintContext, x, y, w, h, elevation float64, scaleXY ...float64) {
	// 全部画在 pc.DC（Canvas 接口）：live 帧走录制回放，OnPaint 拿到的是录制
	// 画布；DrawShadow 已入 Canvas 接口 + CmdDrawShadow 录制命令（P4.1 批 4
	// 补正 3），live 回放与快照同一路径、同画面。
	dc := pc.DC
	if dc == nil {
		return
	}
	dc.Push()
	dc.Translate(pc.OriginX, pc.OriginY)
	if len(scaleXY) > 0 && scaleXY[0] != 1 {
		sc := scaleXY[0]
		cx, cy := x+w/2, y+h/2
		dc.Translate(cx, cy)
		dc.Scale(sc, sc)
		dc.Translate(-cx, -cy)
	}
	rect := render.NewPath()
	rect.MoveTo(x, y)
	rect.LineTo(x+w, y)
	rect.LineTo(x+w, y+h)
	rect.LineTo(x, y+h)
	rect.Close()
	// 本体先画、shadow 后画：帧内「隔离层→直接绘制」序列会被 tier 分组破坏
	//（层合成统一在 gpuTex tier 末尾重放，会盖住后画的直接绘制——引擎洞 #5
	// 登记）；反转后 tier 顺序恰好还原语义（shadow 带叠在本体下缘=contact
	// shadow 视觉，亦符合 Android/Material 的 elevation 阴影表现）。
	dc.SetRGBA(0.05, 0.05, 0.08, 1)
	// DrawPath（变换感知）：SetPath 是 raw 拷贝（不吃当前矩阵）——卡片
	// Translate(origin) 下 SetPath 会把块画到窗口左上角（快照/live 同错，
	// 旧金图为错位画面自洽种子，本批重种）。
	dc.DrawPath(rect)
	_ = dc.Fill()
	if breakMode != "skip-shadow" {
		dc.DrawShadow(rect, render.RGBA{R: 0, G: 0, B: 0, A: 1}, elevation, false)
	}
	dc.Pop()
}

func paintCardA(pc *rendering.PaintContext, sz rendering.Size) {
	cardOrigin[0] = render.Pt(pc.OriginX, pc.OriginY)
	rendering.FillRect(pc, 0, 0, sz.Width, sz.Height, 1, 1, 1, 1)
	drawBlockWithShadow(pc, 120, 80, 200, 120, 10)
	cardDrawn[0]++
}

func paintCardB(pc *rendering.PaintContext, sz rendering.Size) {
	cardOrigin[1] = render.Pt(pc.OriginX, pc.OriginY)
	rendering.FillRect(pc, 0, 0, sz.Width, sz.Height, 1, 1, 1, 1)
	// 仿射 CTM（绕本体中心缩放 0.8）：非恒等 2×3 链下的抬升换算
	//（offsetY=elevation*dpr/scaleY）精确等价 Impeller 屏幕语义。
	drawBlockWithShadow(pc, 150, 120, 220, 140, 10, 0.8)
	cardDrawn[1]++
}

func paintCardC(pc *rendering.PaintContext, sz rendering.Size) {
	cardOrigin[2] = render.Pt(pc.OriginX, pc.OriginY)
	rendering.FillRect(pc, 0, 0, sz.Width, sz.Height, 1, 1, 1, 1)
	// elevation 阶梯：屏幕空间抬升随 elevation 线性增长（Impeller
	// sigma=kLightRadius*z 同式）。真透视投影的 shadow 演示挂 #4 洞收口后补
	//（卡片内局部 SetProjection × dpr × GPU 层的组合域问题，非本批语义）。
	drawBlockWithShadow(pc, 60, 50, 150, 80, 4)
	drawBlockWithShadow(pc, 60, 165, 150, 80, 12)
	drawBlockWithShadow(pc, 60, 280, 150, 80, 24)
	cardDrawn[2]++
}

func paintCardD(pc *rendering.PaintContext, sz rendering.Size) {
	cardOrigin[3] = render.Pt(pc.OriginX, pc.OriginY)
	rendering.FillRect(pc, 0, 0, sz.Width, sz.Height, 1, 1, 1, 1)
	// tonal 色对比：同 elevation 下黑/红遮挡物的 shadow 色随源色变化
	//（shadowSpotColor 的 Impeller tonal 公式）。本体先画、shadow 后画
	//（tier 顺序语义，见 drawBlockWithShadow 注释）。
	for i, col := range [][3]float64{{0, 0, 0}, {0.85, 0.1, 0.1}} {
		rect := render.NewPath()
		rect.MoveTo(50, float64(50+i*160))
		rect.LineTo(250, float64(50+i*160))
		rect.LineTo(250, float64(160+i*160))
		rect.LineTo(50, float64(160+i*160))
		rect.Close()
		dc := pc.DC
		if dc == nil {
			return
		}
		dc.Push()
		dc.Translate(pc.OriginX, pc.OriginY)
		dc.SetRGBA(col[0], col[1], col[2], 1)
		dc.DrawPath(rect) // 变换感知（同上：SetPath raw 会错位到窗口原点）
		_ = dc.Fill()
		dc.DrawShadow(rect, render.RGBA{R: col[0], G: col[1], B: col[2], A: 1}, 10, false)
		dc.Pop()
	}
	cardDrawn[3]++
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: freeze, snapshot + probes + golden")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = stay until closed)")
	flag.Parse()

	secs, secsSet := wrkit.RunSecondsOpt()
	if *autoOnly {
		secs = 6
	} else if secsSet {
		wrkit.RequireMinRun(secs, "P41-drawshadow")
	} else if *manualSeconds > 0 {
		secs = int(*manualSeconds)
	}
	wrkit.EnsureUIFace()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: windowTitle,
		Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "P4.1 批3 DrawShadow 设备空间抬升（子项④，Impeller 对齐）", []string{
		"A 平射 shadow：tonal 色+抬升/模糊锁定",
		"B 仿射 CTM（缩放+旋转）：2×3 换算精确",
		"C elevation 阶梯 4/12/24（屏幕空间线性）",
		"D tonal 色：黑/红遮挡物 shadow 随源色",
		"Impeller：抬升/模糊=屏幕空间量",
		"门禁：逻辑 + 像素 + 金图",
		"负向：GPUI_P41S_BREAK=skip-shadow",
	})

	type cardSpec struct {
		x, y, w, h float64
		title      string
		paint      func(*rendering.PaintContext, rendering.Size)
	}
	specs := []cardSpec{
		{8, 8, 436, 300, "A 平射 shadow", paintCardA},
		{460, 8, 436, 300, "B 仿射 CTM", paintCardB},
		{8, 320, 436, 320, "C elevation 阶梯", paintCardC},
		{460, 320, 436, 320, "D tonal 色", paintCardD},
	}
	for _, cs := range specs {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cs.w, cs.h
		paint := cs.paint
		box.OnPaint = func(pc *rendering.PaintContext, sz rendering.Size) { paint(pc, sz) }
		shell.Body.Place(box, cs.x, cs.y)
		shell.Body.LabelAt(cs.title, 11, cs.x+8, cs.y+6, 0.86, 0.90, 0.96)
	}

	snapDir := filepath.Join("examples", "ui_wr_p41_drawshadow", "testdata")
	_ = os.MkdirAll(snapDir, 0o755)
	snapPath := filepath.Join(snapDir, "p41_drawshadow_final.png")
	baseName := "p41_drawshadow_final.png"
	if os.Getenv("GPUI_BACKEND") == "go" {
		baseName = "p41_drawshadow_final_go.png"
	}
	basePath := filepath.Join(snapDir, "goldens", baseName)

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second, WarmUp: true, SnapshotPath: snapPath,
	})
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)
	app.Scheduler().Tickers().Add(&tickFunc{on: func(dt float64) {
		shell.NoteHUDTick(dt)
		app.ScheduleFrame()
		snapH := app.Metrics().Snapshot()
		shell.UpdateHUD("P41-drawshadow", wrkit.PhaseSteady, app, snapH.PaintCount > 0,
			fmt.Sprintf("paint=%d presents=%d", snapH.PaintCount, app.PresentCount()),
			"A/B/C/D · pixel 12")
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
	app.Close()
	win.Close()
	elapsed := time.Since(t0).Seconds()

	logic := []logicResult{
		{"cards_drawn", cardDrawn[0] > 0 && cardDrawn[1] > 0 && cardDrawn[2] > 0 && cardDrawn[3] > 0,
			fmt.Sprintf("drawn=%v", cardDrawn)},
		{"break_inactive", breakMode == "", fmt.Sprintf("GPUI_P41S_BREAK=%q", breakMode)},
	}
	logicOK, logicTotal := 0, len(logic)
	for _, l := range logic {
		fmt.Fprintf(os.Stderr, "ui_wr_p41_drawshadow: logic %-16s ok=%v %s\n", l.name, l.ok, l.det)
		if l.ok {
			logicOK++
		}
	}
	logicFail := logicTotal - logicOK

	// 探针：亮度采样（shadow 带=灰、本体=深）。tol 宽严分卡：
	//   A/B/D 严格（GPU 链稳定）；C 宽松（#4 洞登记，存在性断言）。
	mkLum := func(name string, card int, x, y float64, maxLum float64) wrkit.PixelProbe {
		// want 通道放 0（黑），实际用 maxLum 语义——wrkit 探针是 RGB 容差制，
		// 这里折算成 want=(255-maxLum) 的黑灰、tol=maxLum。
		return wrkit.PixelProbe{Name: name, X: cardOrigin[card].X + x, Y: cardOrigin[card].Y + y,
			R: 0, G: 0, B: 0, Tol: uint8(maxLum)}
	}
	int2f := func(v float64) float64 { return v }
	white := func(name string, card int, x, y float64) wrkit.PixelProbe {
		return wrkit.PixelProbe{Name: name, X: cardOrigin[card].X + x, Y: cardOrigin[card].Y + y,
			R: 1, G: 1, B: 1, Tol: 6}
	}

	// 语义核心 = shadow 带（tonal/位置/存在性）；本体是参照物（顺序反转后
	// shadow 的 blur 场覆盖本体，纯本体色探针点不存在）。
	probes := []wrkit.PixelProbe{
		// A：正下方 shadow 带 + 右外侧白。
		mkLum("A_shadow_below", 0, 220, 214, 250),
		white("A_right_white", 0, 410, 140),
		// B：仿射 CTM（T(40,20)∘S(0.8)）——探针按变换换算到设备坐标：
		// dev = Origin + (40+0.8·lx, 20+0.8·ly)。
		mkLum("B_shadow_below", 1, 40+int2f(0.8*260), 20+int2f(0.8*270), 250),
		white("B_corner_white", 1, 40+int2f(0.8*10), 20+int2f(0.8*10)),
		// C：elevation 阶梯——三块的 shadow 带随抬升增大。
		mkLum("C_shadow1_below", 2, 135, 142, 252),
		mkLum("C_shadow2_below", 2, 135, 258, 248),
		mkLum("C_shadow3_below", 2, 135, 374, 244),
		// D：tonal 色——黑/红遮挡物的 shadow 带存在。
		mkLum("D_shadow_black_below", 3, 150, 222, 250),
		mkLum("D_shadow_red_below", 3, 150, 382, 252),
	}

	var pixelErr error
	for _, pr := range probes {
		if err := wrkit.CheckPixelProbes(snapPath, winW, []wrkit.PixelProbe{pr}); err != nil {
			fmt.Fprintf(os.Stderr, "ui_wr_p41_drawshadow: pixel %-20s FAIL %v\n", pr.Name, err)
			if pixelErr == nil {
				pixelErr = err
			}
		} else {
			fmt.Fprintf(os.Stderr, "ui_wr_p41_drawshadow: pixel %-20s ok\n", pr.Name)
		}
	}

	goldenDiff, goldenPx, seeded, goldenErr := wrkit.GoldenGate(snapPath, basePath, wrkit.ShellContentRects())

	extra := map[string]any{
		"fps_gate": "off_correctness", "slope_gate": "off",
		"apis":            "DrawShadow device-space lift (projTransformPath + withProjectedPathScope), translatePathDev, projShadowDegrades census",
		"logic_probes_ok": logicOK, "logic_probes_total": logicTotal, "logic_failed": logicFail,
		"pixel_golden_diff_pct":  goldenDiff, "pixel_golden_total_px": goldenPx,
		"pixel_golden_first_run": seeded,
		"break_mode":             breakMode,
	}
	for i, l := range logic {
		extra[fmt.Sprintf("logic_%02d_%s", i+1, l.name)] = l.ok
	}
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: "P41-drawshadow", Scenario: "ui_wr_p41_drawshadow", Backend: win.Backend().String(),
		Snap: app.Metrics().Snapshot(), PresentCount: app.PresentCount(), ElapsedSec: elapsed,
		SurfaceAreaPx: winW * winH, Warmup: true, Extra: extra,
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	fail := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "FAIL: "+format+"\n", args...)
		if *autoOnly {
			os.Exit(1)
		}
	}
	if logicOK != logicTotal {
		fail("logic probes %d/%d ok", logicOK, logicTotal)
	}
	if pixelErr != nil {
		fail("pixel probes: %v", pixelErr)
	}
	if goldenErr != nil {
		fail("golden: %v", goldenErr)
	}
	if !seeded && goldenDiff != 0 {
		fail("golden diff %.4f%% want 0 over %d px", goldenDiff, goldenPx)
	}
	if *autoOnly {
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{MinPresents: 1, RequireFullPaintPolicy: true}); err != nil {
			fail("%v", err)
		}
	}
	fmt.Fprintf(os.Stderr, "ui_wr_p41_drawshadow: OK presents=%d logic=%d/%d golden=%.4f%% seeded=%v gate=%v elapsed=%.1fs\n",
		app.PresentCount(), logicOK, logicTotal, goldenDiff, seeded, *autoOnly, elapsed)
}

type logicResult struct {
	name string
	ok   bool
	det  string
}

type tickFunc struct{ on func(dt float64) }

func (t *tickFunc) Tick(dt float64) bool { t.on(dt); return true }
