// Command ui_wr_p41_evenodd_msaa is the P4.1 batch-4 capability real window:
// MSAA>1 even-odd path clipping (subitem 1) on top of the go4x MSAA fixes.
//
// Four cards (all at 4x MSAA — SetMSAASampleCount before window open):
//
//	A  even-odd ring clip (two concentric circles, same direction) — the
//	   inner disk is an even-odd hole: it must show the card background,
//	   not the clipped fill (the old winding stencil kept it filled).
//	B  winding lock — the same two circles under the non-zero rule stay a
//	   full disk (winding=2): behavior must not change with the fix.
//	C  even-odd self-intersecting star — the center pentagon is a hole.
//	D  even-odd ring over a striped underlay — the hole exposes the stripes,
//	   proving the hole is really carved (not just background-colored).
//
// Negative control: GPUI_P41E_BREAK=nonzero makes the even-odd cards use the
// non-zero rule (the old observable behavior: no holes) and must turn the
// hole probes red. Engine-level negatives (eo-to-wind pipeline, depth
// clear/range revert) live in render/clip_evenodd_msaa_test.go.
//
// Gate mode (-auto-only) asserts logic probes, pixel probes and a
// zero-tolerance golden; the default run stays open for manual inspection.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
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
	windowTitle = "gpui ui_wr_p41_evenodd_msaa — P4.1 批4 MSAA even-odd 裁剪"
)

var breakMode = os.Getenv("GPUI_P41E_BREAK")

// autoGate mirrors the -auto-only flag (the gate frame must be fully static).
var autoGate bool

var cardOrigin [4]render.Point
var cardDrawn [4]int

// Card-local clip geometry: concentric circles for A/B/D, star for C.
var (
	ringOuterR = 120.0
	ringInnerR = 60.0
	starR      = 110.0
	starInnerR = 44.0
)

// fillRuleFor returns the fill rule a card should clip with. The negative
// control ("nonzero") forces the non-zero rule on the even-odd cards — the
// old observable behavior (holes stay filled) — so the hole probes must turn
// red, proving they have teeth.
func fillRuleFor(evenOdd bool) render.FillRule {
	if evenOdd && breakMode != "nonzero" {
		return render.FillRuleEvenOdd
	}
	return render.FillRuleNonZero
}

// clipAndFill builds clipPath, clips with rule, fills the whole card.
// 画在 pc.DC（render.Canvas）上而不是 LiveContext()：live 帧走
// recordFullFrame→presentRecordedFrame（录制回放），OnPaint 拿到的是录制画布，
// LiveContext 断言 *render.Context 失败会静默跳过整卡内容（白卡根因）；
// Canvas 接口在 live/录制两种画布上都可用，回放后引擎流与快照一致。
func clipAndFill(pc *rendering.PaintContext, clipPath *render.Path, rule render.FillRule, rr, gg, bb float64) {
	dc := pc.DC
	dc.Push()
	dc.Translate(pc.OriginX, pc.OriginY)
	if breakMode != "noclip" { // 负向诊断：跳过 clip 看 fill 本体是否画
		// DrawPath = 变换感知（SetPath 是 raw 拷贝，不吃当前矩阵——
		// 卡片内 translate 下必须用 DrawPath，否则 clip 落到窗口原点）。
		dc.DrawPath(clipPath)
		dc.SetFillRule(rule)
		dc.Clip()
	}
	dc.SetRGBA(rr, gg, bb, 1)
	dc.DrawRectangle(-2, -2, 520, 380) // oversized: only the clip shows
	_ = dc.Fill()
	dc.Pop()
}

func ringPath(cx, cy float64) *render.Path {
	p := render.NewPath()
	p.Circle(cx, cy, ringOuterR)
	p.Circle(cx, cy, ringInnerR)
	return p
}

func paintCardA(pc *rendering.PaintContext, sz rendering.Size) {
	cardOrigin[0] = render.Pt(pc.OriginX, pc.OriginY)
	rendering.FillRect(pc, 0, 0, sz.Width, sz.Height, 1, 1, 1, 1)
	clipAndFill(pc, ringPath(160, 150), fillRuleFor(true), 0.9, 0.15, 0.15)
	cardDrawn[0]++
}

func paintCardB(pc *rendering.PaintContext, sz rendering.Size) {
	cardOrigin[1] = render.Pt(pc.OriginX, pc.OriginY)
	rendering.FillRect(pc, 0, 0, sz.Width, sz.Height, 1, 1, 1, 1)
	clipAndFill(pc, ringPath(160, 150), fillRuleFor(false), 0.15, 0.35, 0.85)
	cardDrawn[1]++
}

// starPath builds a {5/2} pentagram (vertices connected in skip-1 order).
// The edges self-intersect, so under even-odd the central pentagon is covered
// an even number of times = hole; under the non-zero rule it stays filled.
// (A 10-point alternating outline is a SIMPLE polygon — no hole under either
// rule; the first draft's expectation was wrong, not the engine.)
func starPath(cx, cy float64) *render.Path {
	pts := make([]render.Point, 5)
	for i := 0; i < 5; i++ {
		ang := -math.Pi/2 + float64(i)*2*math.Pi/5
		pts[i] = render.Pt(cx+starR*math.Cos(ang), cy+starR*math.Sin(ang))
	}
	order := [5]int{0, 2, 4, 1, 3}
	p := render.NewPath()
	for i, idx := range order {
		if i == 0 {
			p.MoveTo(pts[idx].X, pts[idx].Y)
		} else {
			p.LineTo(pts[idx].X, pts[idx].Y)
		}
	}
	p.Close()
	return p
}

func paintCardC(pc *rendering.PaintContext, sz rendering.Size) {
	cardOrigin[2] = render.Pt(pc.OriginX, pc.OriginY)
	rendering.FillRect(pc, 0, 0, sz.Width, sz.Height, 1, 1, 1, 1)
	clipAndFill(pc, starPath(160, 150), fillRuleFor(true), 0.3, 0.75, 0.4)
	cardDrawn[2]++
}

func paintCardD(pc *rendering.PaintContext, sz rendering.Size) {
	cardOrigin[3] = render.Pt(pc.OriginX, pc.OriginY)
	// Striped underlay: the even-odd hole must expose it.
	rendering.FillRect(pc, 0, 0, sz.Width, sz.Height, 1, 1, 1, 1)
	dc := pc.DC
	dc.Push()
	dc.Translate(pc.OriginX, pc.OriginY)
	for y := 0.0; y < 300; y += 24 {
		dc.SetRGBA(0.25, 0.45, 0.85, 1)
		dc.DrawRectangle(0, y, 320, 12)
		_ = dc.Fill()
	}
	dc.Pop()
	clipAndFill(pc, ringPath(160, 150), fillRuleFor(true), 0.85, 0.5, 0.1)
	cardDrawn[3]++
}

type logicResult struct {
	name string
	ok   bool
	det  string
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: freeze, snapshot + probes + golden")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = stay until closed)")
	flag.Parse()
	autoGate = *autoOnly

	secs, secsSet := wrkit.RunSecondsOpt()
	if *autoOnly {
		secs = 6
	} else if secsSet {
		wrkit.RequireMinRun(secs, "P41-evenodd-msaa")
	} else if *manualSeconds > 0 {
		secs = int(*manualSeconds)
	}
	wrkit.EnsureUIFace()

	// MSAA 4x must be set BEFORE the window opens (session sample count is
	// locked at first GPU init — the go4x diagnosis lesson). GPUI_P41E_MSAA
	// allows a 1x diagnostic run (default 4).
	msaaReq := 4
	if v := os.Getenv("GPUI_P41E_MSAA"); v == "1" {
		msaaReq = 1
	}
	render.SetMSAASampleCount(uint32(msaaReq))
	defer render.SetMSAASampleCount(0)

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: windowTitle,
		Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "P4.1 批4 MSAA even-odd 裁剪（子项①）", []string{
		"A even-odd 环孔（双同心圆）→ 孔洞露背景",
		"B winding 锁定（非零规则整盘照画）",
		"C even-odd 星形自交 → 中心五边形孔",
		"D even-odd 孔洞透条纹下层",
		"Skia：MSAA even-odd=stencil INVERT",
		"门禁：逻辑 + 像素 + 金图（4x MSAA）",
		"负向：GPUI_P41E_BREAK=nonzero",
	})

	type cardSpec struct {
		x, y, w, h float64
		title      string
		paint      func(*rendering.PaintContext, rendering.Size)
	}
	specs := []cardSpec{
		{8, 8, 320, 300, "A even-odd 环孔", paintCardA},
		{336, 8, 320, 300, "B winding 锁定", paintCardB},
		{8, 320, 320, 300, "C even-odd 星形", paintCardC},
		{336, 320, 320, 300, "D 孔洞透下层", paintCardD},
	}
	for _, cs := range specs {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cs.w, cs.h
		paint := cs.paint
		box.OnPaint = func(pc *rendering.PaintContext, sz rendering.Size) { paint(pc, sz) }
		shell.Body.Place(box, cs.x, cs.y)
		shell.Body.LabelAt(cs.title, 11, cs.x+8, cs.y+6, 0.86, 0.90, 0.96)
	}

	snapDir := filepath.Join("examples", "ui_wr_p41_evenodd_msaa", "testdata")
	_ = os.MkdirAll(snapDir, 0o755)
	snapPath := filepath.Join(snapDir, "p41_evenodd_msaa_final.png")
	baseName := "p41_evenodd_msaa_final.png"
	if os.Getenv("GPUI_BACKEND") == "go" {
		baseName = "p41_evenodd_msaa_final_go.png"
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
		shell.UpdateHUD("P41-evenodd-msaa", wrkit.PhaseSteady, app, snapH.PaintCount > 0,
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
		{"msaa_setting", int(render.MSAASampleCount()) == msaaReq,
			fmt.Sprintf("msaa=%d want=%d", render.MSAASampleCount(), msaaReq)},
		{"break_inactive", breakMode == "", fmt.Sprintf("GPUI_P41E_BREAK=%q", breakMode)},
	}
	logicOK, logicTotal := 0, len(logic)
	for _, l := range logic {
		fmt.Fprintf(os.Stderr, "ui_wr_p41_evenodd_msaa: logic %-16s ok=%v %s\n", l.name, l.ok, l.det)
		if l.ok {
			logicOK++
		}
	}
	logicFail := logicTotal - logicOK

	// Probe colors in 0..1 channels (PixelProbe R/G/B are 0..1 floats).
	white := [3]float64{1, 1, 1}
	red := [3]float64{0.9, 0.15, 0.15}
	blue := [3]float64{0.15, 0.35, 0.85}
	green := [3]float64{0.3, 0.75, 0.4}
	orange := [3]float64{0.85, 0.5, 0.1}
	stripe := [3]float64{0.25, 0.45, 0.85}

	mkProbe := func(name string, card int, x, y float64, c [3]float64, tol uint8) wrkit.PixelProbe {
		return wrkit.PixelProbe{
			Name: name,
			X:    cardOrigin[card].X + x,
			Y:    cardOrigin[card].Y + y,
			R:    c[0], G: c[1], B: c[2], Tol: tol,
		}
	}

	probes := []wrkit.PixelProbe{
		// A: even-odd ring — hole (r<60) shows background, ring (60<r<120) red.
		mkProbe("A_hole_center", 0, 160, 150, white, 6),
		mkProbe("A_hole_inner", 0, 160, 110, white, 6),
		mkProbe("A_ring", 0, 160, 45, red, 12),      // r=105: ring band
		mkProbe("A_outside", 0, 160, 285, white, 6), // r=135: outside outer circle
		// B: winding lock — inner disk stays filled (non-zero rule).
		mkProbe("B_center_filled", 1, 160, 150, blue, 12),
		mkProbe("B_ring", 1, 160, 45, blue, 12),
		mkProbe("B_outside", 1, 160, 285, white, 6),
		// C: even-odd star — center pentagon is a hole, star points filled.
		mkProbe("C_center_hole", 2, 160, 150, white, 6),
		mkProbe("C_point_top", 2, 160, 70, green, 14),  // top arm, r=80 from center
		// D: even-odd hole exposes the striped underlay.
		mkProbe("D_hole_stripe", 3, 160, 150, stripe, 10),
		mkProbe("D_ring", 3, 160, 45, orange, 12),
	}

	var pixelErr error
	for _, pr := range probes {
		if err := wrkit.CheckPixelProbes(snapPath, winW, []wrkit.PixelProbe{pr}); err != nil {
			fmt.Fprintf(os.Stderr, "ui_wr_p41_evenodd_msaa: pixel %-20s FAIL %v\n", pr.Name, err)
			if pixelErr == nil {
				pixelErr = err
			}
		} else {
			fmt.Fprintf(os.Stderr, "ui_wr_p41_evenodd_msaa: pixel %-20s ok\n", pr.Name)
		}
	}

	goldenDiff, goldenPx, seeded, goldenErr := wrkit.GoldenGate(snapPath, basePath, wrkit.ShellContentRects())

	extra := map[string]any{
		"fps_gate": "off_correctness", "slope_gate": "off",
		"apis":           "Clip path SetFillRule(EvenOdd) → depth_clip eoStencilFillPipeline (stencil INVERT, Skia gEOStencilPass) + desktop-GL depth clear/range fix",
		"msaa_requested": 4, "msaa_setting": int(render.MSAASampleCount()),
		"logic_probes_ok": logicOK, "logic_probes_total": logicTotal, "logic_failed": logicFail,
		"pixel_golden_diff_pct": goldenDiff, "pixel_golden_total_px": goldenPx,
		"pixel_golden_first_run": seeded,
		"break_mode":             breakMode,
	}
	for i, l := range logic {
		extra[fmt.Sprintf("logic_%02d_%s", i+1, l.name)] = l.ok
	}
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: "P41-evenodd-msaa", Scenario: "ui_wr_p41_evenodd_msaa", Backend: win.Backend().String(),
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
	fmt.Fprintf(os.Stderr, "ui_wr_p41_evenodd_msaa: OK presents=%d logic=%d/%d golden=%.4f%% seeded=%v gate=%v elapsed=%.1fs\n",
		app.PresentCount(), logicOK, logicTotal, goldenDiff, seeded, *autoOnly, elapsed)
}

type tickFunc struct{ on func(dt float64) }

func (t *tickFunc) Tick(dt float64) bool { t.on(dt); return true }
