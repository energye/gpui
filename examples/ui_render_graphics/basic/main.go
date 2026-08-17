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
	"fmt"
	"math"
	"os"
	"time"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/ui/platform"
	//_ "github.com/energye/gpui/render/gpu"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"
)

const winW, winH = 1200.0, 800.0

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
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

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor:       time.Duration(secs) * time.Second,
		WarmUp:       true,
		SnapshotPath: os.Getenv("SNAP_PNG"),
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

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: run:", err)
		os.Exit(1)
	}
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	fmt.Fprintf(os.Stderr, "basic: OK presents=%d\n", app.PresentCount())
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
