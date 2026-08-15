// Command ui_render_graphics/vector is the 2D vector path window for
// render_graphics.md §2 (等级B 2D 部分): 二次/三次贝塞尔、Arc、EllipticalArc.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 SNAP_PNG=/tmp/vector.png go run ./examples/ui_render_graphics/vector
//
// Six-cell canvas (3x2), each cell exercises one primitive family:
//
//	① QuadTo stroke（平缓/尖锐/反转） ② QuadTo 闭合 fill（泪滴+径向渐变/椭圆高光）
//	③ CubicTo stroke（S 形/回环/直线退化） ④ CubicTo 闭合 fill（花瓣）
//	⑤ Arc 角度范围（90/180/270/350°） ⑥ EllipticalArc（rx:ry 比例）
//
// 贝塞尔格内同时画控制顶点与多边形（细灰线），便于核对曲线形状。
package main

import (
	"fmt"
	"math"
	"os"
	"time"

	"github.com/energye/gpui/examples/exhost"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
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

	win, err := exhost.Open(exhost.Options{Width: winW, Height: winH, Title: "gpui graphics/vector — Bezier/Arc/EllipticalArc"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "2D 矢量路径（贝塞尔/圆弧/椭圆弧）", []string{
		"① Quad stroke  ② Quad fill  ③ Cubic stroke",
		"④ Cubic fill  ⑤ Arc 角度  ⑥ EllipticalArc 比例",
		"Controls 细灰线为控制多边形",
		"SNAP_PNG 帧内快照；RUN_SECONDS 可调",
		"截图扫曲线平滑度/端点/填充边界",
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

		ctrl := func(poly [][2]float64) {
			dc.SetRGBA(0.45, 0.47, 0.52, 1)
			dc.SetLineWidth(1 / scale)
			for _, p := range poly {
				dc.DrawPoint(p[0], p[1], 3)
				_ = dc.Fill() // 控制点：DrawPoint 需跟随 Fill 才出实心点
			}
			for i := 0; i+1 < len(poly); i++ {
				dc.DrawLine(poly[i][0], poly[i][1], poly[i+1][0], poly[i+1][1])
				_ = dc.Stroke()
			}
		}
		// absc = 格子的窗口绝对原点；之后所有坐标一律"绝对原点+格内偏移"，
		// 不再在 Bezier 控制点上用画布局部坐标。
		absc := func(c, r int) (float64, float64) { return pc.Abs(cell(c, r)) }

		// ① 二次贝塞尔 stroke：平缓 / 尖锐 / 反转三种控制点。
		ox, oy := absc(0, 0)
		cx, cy := ox+63, oy+cellH/2
		ctrl([][2]float64{{ox + 20, oy + cellH/2}, {ox + 67, oy + 18}, {ox + 107, oy + cellH/2}})
		dc.SetLineWidth(2.5 / scale)
		dc.SetRGBA(0.4, 0.85, 0.95, 1)
		dc.MoveTo(cx, cy)
		dc.QuadraticTo(ox+67, oy+18, ox+107, oy+cellH/2)
		_ = dc.Stroke()

		cx, cy = ox+168, oy+cellH/2
		ctrl([][2]float64{{ox + 122, oy + cellH/2}, {ox + 168, oy + 10}, {ox + 212, oy + cellH/2}})
		dc.SetRGBA(0.95, 0.78, 0.25, 1)
		dc.MoveTo(cx, cy)
		dc.QuadraticTo(ox+168, oy+10, ox+212, oy+cellH/2)
		_ = dc.Stroke()

		cx, cy = ox+258, oy+cellH/2
		ctrl([][2]float64{{ox + 222, oy + cellH/2}, {ox + 258, oy + 60}, {ox + 276, oy + 150}})
		dc.SetRGBA(0.9, 0.45, 0.75, 1)
		dc.MoveTo(cx, cy)
		dc.QuadraticTo(ox+258, oy+60, ox+276, oy+150)
		_ = dc.Stroke()

		// ② 二次贝塞尔闭合 fill：泪滴形（两段 QuadTo 闭合）。
		// 填充直接用径向渐变 brush 画在泪滴路径上，贴合轮廓无需裁剪；
		// 顶部偏左放小而靠内的椭圆高光（完全落在轮廓内），轮廓描边保持原样。
		ox, oy = absc(1, 0)
		lx, ly := cell(1, 0) // 局部坐标（FillOval 用）
		drop := render.BuildPath()
		drop.MoveTo(ox+cellW/2, oy+62)
		drop.QuadTo(ox+cellW-44, oy+cellH-62, ox+cellW/2, oy+cellH-52)
		drop.QuadTo(ox+44, oy+cellH-62, ox+cellW/2, oy+62)
		drop.Close()
		grad := render.NewRadialGradientBrush(ox+cellW/2-8, oy+95, 8, 105).
			AddColorStop(0, render.RGBA{R: 0.72, G: 1, B: 0.85, A: 1}).
			AddColorStop(1, render.RGBA{R: 0.03, G: 0.42, B: 0.28, A: 1}) // 光源偏上偏左：中心亮绿 → 轮廓深绿
		dc.SetFillBrush(grad)
		_ = dc.FillPath(drop.Path())
		rendering.FillOval(pc, lx+126, ly+94.5, 18, 11, 0.92, 1, 0.97, 0.85) // 椭圆高光（靠内，不越轮廓）
		dc.SetLineWidth(2 / scale)
		dc.SetLineJoin(render.LineJoinBevel) // 双尖 11° 锐角：miter 会拉出长刺，bevel 平切
		dc.SetRGBA(0.75, 1, 0.85, 1)
		_ = dc.StrokePath(drop.Path())

		// ③ 三次贝塞尔 stroke：S 形 / 回环 / 直线退化。
		ox, oy = absc(2, 0)
		sx, sy := ox+28, oy+cellH-52
		ctrlP := [][2]float64{{ox + 28, oy + cellH - 52}, {ox + 78, oy + 30}, {ox + 128, oy + cellH - 52}, {ox + 178, oy + 30}}
		ctrl(ctrlP)
		dc.SetLineWidth(2.5 / scale)
		dc.SetRGBA(0.4, 0.75, 0.95, 1)
		dc.MoveTo(sx, sy)
		dc.CubicTo(ox+78, oy+30, ox+128, oy+cellH-52, ox+178, oy+30)
		_ = dc.Stroke()

		ctrl([][2]float64{{ox + 196, oy + 155}, {ox + 238, oy + 42}, {ox + 258, oy + 200}, {ox + 262, oy + 46}})
		dc.SetLineWidth(2.5 / scale)
		dc.SetRGBA(0.95, 0.6, 0.3, 1)
		dc.MoveTo(ox+196, oy+155)
		dc.CubicTo(ox+238, oy+42, ox+258, oy+200, ox+262, oy+46)
		_ = dc.Stroke()

		ctrl([][2]float64{{ox + 66, oy + 190}, {ox + 96, oy + 130}, {ox + 126, oy + 70}, {ox + 156, oy + 10}})
		dc.SetLineWidth(2.5 / scale)
		dc.SetRGBA(0.85, 0.5, 0.55, 1)
		dc.MoveTo(ox+66, oy+190)
		dc.CubicTo(ox+96, oy+130, ox+126, oy+70, ox+156, oy+10)
		_ = dc.Stroke()

		// ④ 三次贝塞尔闭合 fill：四瓣花（凹点 r=55，瓣尖 r=55√2≈77.8）。
		// 半径比 √2 使凹点处两侧切向严格共线 → 全轮廓 C1 连续，凹口圆滑无折角；
		// 瓣尖为 cubic 自然拱顶（段内切线连续），任何角度无棱。
		ox, oy = absc(0, 1)
		pcx, pcy := ox+cellW/2, oy+cellH/2
		const rIn = 55.0
		rOut := rIn * math.Sqrt2
		const ext = 2.0 / 3.0
		dc.SetRGBA(0.75, 0.4, 0.9, 1)
		petal := render.BuildPath()
		for kk := 0; kk < 4; kk++ {
			a0 := float64(kk)*math.Pi/2 + math.Pi/4 // 凹点方向（45° 起始）
			p0x, p0y := pcx+rIn*math.Cos(a0), pcy-rIn*math.Sin(a0)
			p1x, p1y := pcx+rIn*math.Cos(a0+math.Pi/2), pcy-rIn*math.Sin(a0+math.Pi/2)
			t1x, t1y := pcx+rOut*math.Cos(a0+math.Pi/4), pcy-rOut*math.Sin(a0+math.Pi/4)
			if kk == 0 {
				petal.MoveTo(p0x, p0y)
			}
			petal.CubicTo(p0x+(t1x-p0x)*ext, p0y+(t1y-p0y)*ext, p1x+(t1x-p1x)*ext, p1y+(t1y-p1y)*ext, p1x, p1y)
		}
		petal.Close()
		dc.FillPath(petal.Path())

		// ⑤ Arc 角度范围：90°/180°/270°/350° 同心。
		ox, oy = absc(1, 1)
		acx, acy := ox+cellW/2, oy+cellH/2
		dc.SetLineWidth(2.5 / scale)
		arcs := []struct {
			a1, a2     float64
			r          float64
			rr, gg, bb float64
		}{
			{0, math.Pi / 2, 86, 0.95, 0.4, 0.5},
			{math.Pi / 2, 3 * math.Pi / 2, 66, 0.4, 0.85, 0.5},
			{math.Pi, 2*math.Pi + math.Pi/2, 46, 0.4, 0.65, 0.95},
			{math.Pi / 4, math.Pi/4 + 1.9106, 26, 0.95, 0.78, 0.25}, // 350°
		}
		for _, a := range arcs {
			dc.SetRGBA(a.rr, a.gg, a.bb, 1)
			dc.DrawArc(acx, acy, a.r, a.a1, a.a2)
			_ = dc.Stroke()
		}

		// ⑥ EllipticalArc：不同 rx:ry 比例与角度范围。
		ox, oy = absc(2, 1)
		ecx, ecy := ox+cellW/2, oy+cellH/2
		dc.SetLineWidth(2.5 / scale)
		dc.SetRGBA(0.95, 0.5, 0.35, 1)
		dc.DrawEllipticalArc(ecx, ecy, 100, 34, 0.4, 3.0)
		_ = dc.Stroke()
		dc.SetRGBA(0.45, 0.8, 0.6, 1)
		dc.DrawEllipticalArc(ecx, ecy, 56, 100, 3.2, 5.6)
		_ = dc.Stroke()
		dc.SetRGBA(0.7, 0.6, 0.95, 1)
		dc.DrawEllipticalArc(ecx, ecy, 84, 60, -0.7, 0.7) // 跨 0 轴小弧
		_ = dc.Stroke()

		fmt.Fprintf(os.Stderr, "vector.OnPaint: scale=%.2f quads=%d cubics=%d arcs=%d ellipticals=%d\n",
			scale, 5, 5, 4, 3)
	}

	shell.Body.Place(canvas, 0, 0)
	shell.Body.Place(wrkit.Label("vector — §2 矢量路径：QuadTo / CubicTo / Arc / EllipticalArc", 13, 0.9, 0.92, 0.97), 4, 2*cellH+2*pad+8)

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor:       time.Duration(secs) * time.Second,
		WarmUp:       true,
		SnapshotPath: os.Getenv("SNAP_PNG"),
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "vector: close (%s)\n", win.Backend())
			case platform.EventResize:
				fmt.Fprintf(os.Stderr, "vector: resize ev=%dx%d (ignored)\n", ev.Width, ev.Height)
			}
		},
	})

	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		app.ScheduleFrame()
		proc.Sample()
		shell.NoteHUDTick(dt)
		shell.UpdateHUD("GRX", "vector", app, true, "Bezier/Arc/EllipArc", "")
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

	fmt.Fprintf(os.Stderr, "vector: OK presents=%d\n", app.PresentCount())
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}