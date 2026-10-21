// Command ui_render_graphics/basic is the raster primitives window for
// render_graphics.md §1 (等级A): Point / LineSegment / Triangle.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 SNAP_PNG=/tmp/basic.png go run ./examples/ui_render_graphics/basic
//
// Six-cell canvas (3x2), each cell exercises one primitive family:
//
//	① Point 多半径（2/4/8/14px） ② Point 点阵网格
//	③ LineSegment 角度（0/90/30/45/60°） ④ LineSegment 线宽（1/2/4/6/8px）
//	⑤ Triangle fill（等边/直角/倒） ⑥ Triangle stroke（细长/描边）
//
// Triangle has no dedicated API: drawn via MoveTo+LineTo+ClosePath path.
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/energye/gpui/examples/wrevidence"
	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/ui/platform"
	//_ "github.com/energye/gpui/render/gpu"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"
)

const winW, winH = 1200.0, 800.0

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate run: snapshot + pixel + Golden gate (default 5s)")
	manualSeconds := flag.Int("manual-seconds", 0, "manual run seconds (0 = until close; manual runs do not seed Golden)")
	flag.Parse()

	secs, secsSet := wrkit.RunSecondsOpt()
	gateMode := *autoOnly
	if *manualSeconds > 0 {
		secs = *manualSeconds
		gateMode = false
	} else if gateMode {
		if !secsSet {
			secs = 5
		}
	}
	if gateMode {
		wrkit.RequireMinRun(secs, "GRX")
	}
	wrkit.EnsureUIFace()

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui graphics/basic — Point/Line/Triangle", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "基础光栅图元（Point/LineSegment/Triangle）", []string{
		"① Point 多半径  ② Point 点阵  ③ Line 角度",
		"④ Line 线宽  ⑤ Triangle fill  ⑥ Triangle stroke",
		"Triangle = MoveTo+LineTo×2+ClosePath",
		"SNAP_PNG 帧内快照；RUN_SECONDS 可调",
		"截图扫点形状/线宽/边缘填充",
	})

	const cellW, cellH, pad = 290.0, 245.0, 17.0
	canvas := rendering.NewRenderBox()
	canvas.FixedWidth, canvas.FixedHeight = 3*cellW+2*pad, 2*cellH+2*pad
	canvas.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		dc := pc.DC
		scale := pc.Scale
		if scale <= 0 {
			scale = 1
		}
		cell := func(c, r int) (float64, float64) {
			return float64(c)*(cellW+pad) + pad, float64(r)*(cellH+pad) + pad
		}
		for row := 0; row < 2; row++ {
			for col := 0; col < 3; col++ {
				ax, ay := cell(col, row)
				ax, ay = pc.Abs(ax, ay)
				if (row+col)%2 == 0 {
					dc.SetRGBA(0.135, 0.15, 0.19, 1)
				} else {
					dc.SetRGBA(0.11, 0.12, 0.15, 1)
				}
				dc.DrawRectangle(ax, ay, cellW, cellH)
				_ = dc.Fill()
			}
		}

		// ① Point 多半径：r=2/4/8/14，同一中线排列。
		ox, oy := cell(0, 0)
		mx, my := pc.Abs(ox+30, oy+cellH/2)
		radii := []float64{2, 4, 8, 14}
		cols := [][4]float64{{1, 0.45, 0.4}, {0.95, 0.78, 0.25}, {0.4, 0.85, 0.5}, {0.4, 0.75, 0.95}}
		for i, r := range radii {
			dc.SetRGBA(cols[i][0], cols[i][1], cols[i][2], 1)
			dc.DrawPoint(mx+float64(i)*62, my, r)
			_ = dc.Fill() // DrawPoint 只构建圆路径，必须 Fill 才出实心点
		}

		// ② Point 点阵网格：5x5，r=3，验证点覆盖与中心对齐。
		ox, oy = cell(1, 0)
		dc.SetRGBA(0.6, 0.62, 0.68, 1)
		for gx := 0; gx < 5; gx++ {
			for gy := 0; gy < 5; gy++ {
				ax, ay := pc.Abs(ox+cellW/2+(float64(gx)-2)*32, oy+cellH/2+(float64(gy)-2)*32)
				dc.DrawPoint(ax, ay, 3)
			}
		}
		_ = dc.Fill() // 25 点同色，一个路径一次填
		ax, ay := pc.Abs(ox+cellW/2, oy+cellH/2)
		dc.SetRGBA(0.95, 0.5, 0.2, 1)
		dc.DrawPoint(ax, ay, 8) // 中心点放大，检验网格对齐
		_ = dc.Fill()

		// ③ LineSegment 角度：水平/垂直/30/45/60°。
		ox, oy = cell(2, 0)
		cx, cy := pc.Abs(ox+cellW/2, oy+cellH/2)
		dc.SetLineWidth(2 / scale)
		dc.SetRGBA(0.95, 0.55, 0.5, 1)
		dc.DrawLine(cx-80, cy, cx+80, cy) // 0°
		_ = dc.Stroke()
		dc.SetRGBA(0.55, 0.85, 0.6, 1)
		dc.DrawLine(cx, cy-80, cx, cy+80) // 90°
		_ = dc.Stroke()
		for i, deg := range []float64{30, 45, 60} {
			rad := deg * math.Pi / 180
			cols := [][4]float64{{0.95, 0.78, 0.25}, {0.4, 0.75, 0.95}, {0.9, 0.45, 0.85}}
			dc.SetRGBA(cols[i][0], cols[i][1], cols[i][2], 1)
			dc.DrawLine(cx+90*math.Cos(rad)+40, cy+90*math.Sin(rad)+42, cx-90*math.Cos(rad)+40, cy-90*math.Sin(rad)+42)
			_ = dc.Stroke()
		}

		// ④ LineSegment 线宽：1/2/4/6/8px 横线。
		ox, oy = cell(0, 1)
		widths := []float64{1, 2, 4, 6, 8}
		for i, w := range widths {
			a1x, a1y := pc.Abs(ox+40+float64(i)*44+15, oy+55)
			a2x, a2y := pc.Abs(ox+40+float64(i)*44+48, oy+55)
			dc.SetLineWidth(w / scale)
			dc.SetRGBA(0.55+0.1*float64(i), 0.75-0.06*float64(i), 0.95, 1)
			dc.DrawLine(a1x, a1y, a2x, a2y)
			_ = dc.Stroke()
		}

		// ⑤ Triangle fill：等边/直角/倒三角。
		ox, oy = cell(1, 1)
		cx, cy = pc.Abs(ox+60, oy+cellH/2)
		dc.SetRGBA(0.25, 0.8, 0.7, 1)
		dc.MoveTo(cx, cy-55)
		dc.LineTo(cx+48, cy+40)
		dc.LineTo(cx-48, cy+40)
		dc.ClosePath()
		_ = dc.Fill()
		cx, cy = pc.Abs(ox+160, oy+27)
		dc.SetRGBA(0.85, 0.6, 0.3, 1)
		dc.MoveTo(cx, cy)
		dc.LineTo(cx+80, cy)
		dc.LineTo(cx, cy+110)
		dc.ClosePath()
		_ = dc.Fill()
		cx, cy = pc.Abs(ox+245, oy+cellH/2)
		dc.SetRGBA(0.55, 0.5, 0.9, 1)
		dc.MoveTo(cx, cy+55)
		dc.LineTo(cx+40, cy-40)
		dc.LineTo(cx-40, cy-40)
		dc.ClosePath()
		_ = dc.Fill()

		// ⑥ Triangle stroke：细长三角 + 描边三角。
		ox, oy = cell(2, 1)
		cx, cy = pc.Abs(ox+65, oy+cellH-35)
		dc.SetRGBA(0.45, 0.85, 0.5, 1)
		dc.MoveTo(cx, cy)
		dc.LineTo(cx+95, cy-175)
		dc.LineTo(cx+30, cy)
		dc.ClosePath()
		_ = dc.Fill()
		cx, cy = pc.Abs(ox+210, oy+cellH/2)
		dc.SetLineWidth(3 / scale)
		dc.SetRGBA(0.95, 0.4, 0.55, 1)
		dc.MoveTo(cx, cy-60)
		dc.LineTo(cx+52, cy+52)
		dc.LineTo(cx-52, cy+52)
		dc.ClosePath()
		_ = dc.Stroke()

		fmt.Fprintf(os.Stderr, "basic.OnPaint: scale=%.2f points=%d lines=%d tris=%d\n",
			scale, 9, 8, 5)
	}

	shell.Body.Place(canvas, 0, 0)
	shell.Body.Place(wrkit.Label("basic — §1 光栅图元：Point / LineSegment / Triangle", 13, 0.9, 0.92, 0.97), 4, 2*cellH+2*pad+8)

	snapPath := os.Getenv("SNAP_PNG")
	if snapPath == "" && gateMode {
		snapPath = filepath.Join("examples", "ui_render_graphics", "basic", "testdata", "showcase_basic.png")
	}
	if snapPath != "" {
		if err := os.MkdirAll(filepath.Dir(snapPath), 0o755); err != nil {
			fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
			os.Exit(1)
		}
	}

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor:       time.Duration(secs) * time.Second,
		WarmUp:       true,
		SnapshotPath: snapPath,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "basic: close (%s)\n", win.Backend())
			case platform.EventResize:
				fmt.Fprintf(os.Stderr, "basic: resize ev=%dx%d (ignored)\n", ev.Width, ev.Height)
			}
		},
	})

	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		app.ScheduleFrame()
		proc.Sample()
		shell.NoteHUDTick(dt)
		shell.UpdateHUD("GRX", "basic", app, true, "Point/Line/Triangle", "")
	}})
	app.Scheduler().SetMode(scheduler.ModePersistent)
	if gateMode {
		app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)
	}

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

	presents := app.PresentCount()
	if presents < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: no frames")
		os.Exit(1)
	}
	if !gateMode {
		fmt.Fprintf(os.Stderr, "basic: OK presents=%d\n", presents)
		return
	}

	elapsed := time.Since(t0).Seconds()
	if elapsed < 0.001 {
		elapsed = 0.001
	}
	m := app.Metrics().Snapshot()
	img := wrevidence.Load(snapPath)
	if img == nil {
		os.Exit(1)
	}
	bx, by := shell.Body.X, shell.Body.Y
	checks := []wrevidence.PixelCheck{
		{Desc: "cell0_bg", X: bx + 27, Y: by + 27, Want: [3]float64{0.135, 0.15, 0.19}, Tol: 0.05, Kind: wrevidence.KindBG},
		{Desc: "cell1_bg", X: bx + 334, Y: by + 27, Want: [3]float64{0.11, 0.12, 0.15}, Tol: 0.05, Kind: wrevidence.KindBG},
		{Desc: "point_r2", X: bx + 47, Y: by + 139, Want: [3]float64{1.0, 0.45, 0.40}, Tol: 0.06, Kind: wrevidence.KindBG},
		{Desc: "point_r14", X: bx + 233, Y: by + 139, Want: [3]float64{0.40, 0.75, 0.95}, Tol: 0.06, Kind: wrevidence.KindBG},
		{Desc: "grid_center", X: bx + 469, Y: by + 139, Want: [3]float64{0.95, 0.50, 0.20}, Tol: 0.06, Kind: wrevidence.KindBG},
		{Desc: "tri_fill", X: bx + 384, Y: by + 384, Want: [3]float64{0.25, 0.80, 0.70}, Tol: 0.06, Kind: wrevidence.KindBG},
		{Desc: "line_w6", X: bx + 220, Y: by + 334, Want: [3]float64{0.85, 0.57, 0.95}, Tol: 0.08, Kind: wrevidence.KindBG},
		{Desc: "top_bg", X: 100, Y: 24, Want: [3]float64{0.12, 0.14, 0.18}, Tol: 0.05, Kind: wrevidence.KindBG},
		{Desc: "legend_bg", X: shell.Legend.X + 20, Y: shell.Legend.Y + shell.Legend.H - 20, Want: [3]float64{0.11, 0.12, 0.15}, Tol: 0.05, Kind: wrevidence.KindBG},
	}
	pixelResult := wrevidence.CheckPixels(img, checks, "basic")
	pixelOK := 0
	for _, ok := range pixelResult {
		if ok {
			pixelOK++
		}
	}
	goldenDiffPct, goldenTotalPx, goldenFirstRun := wrevidence.Golden(
		snapPath, filepath.Join("examples", "ui_render_graphics", "basic", "testdata", "showcase_basic_base.png"),
		[]wrevidence.Rect{
			{X: 0, Y: 0, W: winW, H: 48},
			{X: shell.Legend.X, Y: shell.Legend.Y, W: shell.Legend.W, H: shell.Legend.H},
			{X: shell.Body.X, Y: shell.Body.Y, W: shell.Body.W, H: shell.Body.H},
		}, "basic")

	logic := map[string]bool{
		"presents_positive": presents >= 1,
		"paint_positive":    m.PaintCount > 0,
		"gpu_ops_positive":  m.GPUOps > 0 || m.PictureOpCount > 0,
		"fallback_zero":     m.CPUFallbackOps == 0,
		"picture_ops":       m.PictureOpCount > 0,
	}
	logicPass := 0
	for _, ok := range logic {
		if ok {
			logicPass++
		}
	}

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: "GRX-BASIC", Scenario: "ui_render_graphics_basic",
		Backend: win.Backend().String(), Snap: m, PresentCount: presents,
		ElapsedSec: elapsed, SurfaceAreaPx: winW * winH, Warmup: true,
		Extra: map[string]any{
			"logic_probes_total":     len(logic),
			"logic_probes_ok":        logicPass,
			"pixel_probes_total":     len(checks),
			"pixel_probes_ok":        pixelOK,
			"pixel_golden_diff_pct":  goldenDiffPct,
			"pixel_golden_total_px":  goldenTotalPx,
			"pixel_golden_first_run": goldenFirstRun,
		},
	})
	if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
		MinPresents: 1, RequireFullPaintPolicy: true,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if b, err := wrgate.Marshal(report); err == nil {
		fmt.Println(string(b))
	}
	if logicPass != len(logic) {
		fmt.Fprintf(os.Stderr, "FAIL: logic probes %d/%d\n", logicPass, len(logic))
		os.Exit(1)
	}
	if pixelOK != len(checks) {
		fmt.Fprintf(os.Stderr, "FAIL: pixel probes %d/%d\n", pixelOK, len(checks))
		os.Exit(1)
	}
	if !goldenFirstRun && goldenDiffPct != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: golden diff %.4f%% over %d px\n", goldenDiffPct, goldenTotalPx)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "basic: OK presents=%d logic=%d/%d pixel=%d/%d golden=%.4f%% elapsed=%.1fs\n",
		presents, logicPass, len(logic), pixelOK, len(checks), goldenDiffPct, elapsed)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
