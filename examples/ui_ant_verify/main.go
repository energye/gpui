// Command ui_ant_verify is a dedicated AA verification window: 圆角/斜边/细边框
// 在 4x MSAA vs 1x 几何AA（fringe coverage + SDF smoothstep）下的边缘平滑度对比。
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_ant_verify   # 默认 4x MSAA（设备探测）
//	1x 对比：main() 前调 render.SetMSAASampleCount(render.MSAASampleCount1)
//
// Window: 1200x800. 用 xwd 截图后对比六宫格边缘过渡带像素：
//
//	① 大圆 fill  ② 大圆角矩形 fill  ③ 45° 斜边（菱形+三角）
//	④ 1px 细边框（圆+圆角矩形 stroke） ⑤ 斜 1px 细线（30/45/60°+水平）
//	⑥ 弧线 stroke（圆弧+椭圆弧）
//
// 每个图形都画在深色底上、用高对比亮色，便于扫边缘渐变宽度。
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/platform"
	//_ "github.com/energye/gpui/render/gpu"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"
)

const winW, winH = 1200.0, 800.0

func modeName() (name, hint string) {
	switch render.MSAASampleCount() {
	case render.MSAASampleCount1:
		return "1x 几何AA", "render.SetMSAASampleCount(render.MSAASampleCount1)"
	case render.MSAASampleCount4:
		return "4x MSAA", "render.SetMSAASampleCount(render.MSAASampleCount4)"
	default:
		return "4x MSAA (默认)", "未设置 → 设备探测/默认 4x"
	}
}

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "ANT")
	}
	wrkit.EnsureUIFace()

	mode, hint := modeName()

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui ui_ant_verify — " + mode + " 圆角/斜边/细边框", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "AA验证 — "+mode+"（圆角/斜边/细边框）", []string{
		"① 大圆 fill（曲率边缘）",
		"② 大圆角矩形 fill（r=65）",
		"③ 45° 斜边：菱形+三角",
		"④ 1px 细边框 stroke",
		"⑤ 斜 1px 细线 30/45/60°",
		"⑥ 弧线 stroke（圆+椭圆弧）",
		hint + " — 换模式重启示例",
		"截图扫边缘过渡带对比",
	})

	// 六宫格画布：3 列 × 2 行，每格 290x245。
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
		fmt.Fprintf(os.Stderr, "canvas.OnPaint: origin=(%.1f,%.1f) scale=%.2f size=(%.0f,%.0f)\n",
			pc.OriginX, pc.OriginY, scale, size.Width, size.Height)
		// 画布四角定位方块（调试用）：白=左上 黄=右上 红=左下 蓝=右下。
		dc.SetRGBA(1, 1, 1, 1)
		ax, ay := pc.Abs(0, 0)
		dc.DrawRectangle(ax, ay, 30, 30)
		_ = dc.Fill()
		dc.SetRGBA(1, 1, 0.2, 1)
		ax, ay = pc.Abs(3*cellW+2*pad-30, 0)
		dc.DrawRectangle(ax, ay, 30, 30)
		_ = dc.Fill()
		dc.SetRGBA(1, 0.3, 0.3, 1)
		ax, ay = pc.Abs(0, 2*cellH+2*pad-30)
		dc.DrawRectangle(ax, ay, 30, 30)
		_ = dc.Fill()
		dc.SetRGBA(0.3, 0.4, 1, 1)
		ax, ay = pc.Abs(3*cellW+2*pad-30, 2*cellH+2*pad-30)
		dc.DrawRectangle(ax, ay, 30, 30)
		_ = dc.Fill()
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

		// ① 大圆 fill（曲率边缘）。
		ox, oy := cell(0, 0)
		ax, ay = pc.Abs(ox+cellW/2, oy+cellH/2)
		dc.SetRGBA(0.2, 0.85, 0.95, 1)
		dc.DrawCircle(ax, ay, 100)
		_ = dc.Fill()

		// ② 大圆角矩形 fill（r=65）。
		ox, oy = cell(1, 0)
		ax, ay = pc.Abs(ox+pad, oy+pad)
		dc.SetRGBA(0.95, 0.78, 0.25, 1)
		dc.DrawRoundedRectangle(ax, ay, cellW-2*pad, cellH-2*pad, 65)
		_ = dc.Fill()

		// ③ 45° 斜边：菱形 + 直角三角。
		ox, oy = cell(2, 0)
		cx, cy := pc.Abs(ox+cellW/2, oy+cellH/2)
		dc.SetRGBA(0.9, 0.32, 0.75, 1)
		dc.MoveTo(cx, cy-80)
		dc.LineTo(cx+80, cy)
		dc.LineTo(cx, cy+80)
		dc.LineTo(cx-80, cy)
		dc.ClosePath()
		_ = dc.Fill()
		ax, ay = pc.Abs(ox+27, oy+cellH-30)
		dc.SetRGBA(0.35, 0.9, 0.4, 1)
		dc.MoveTo(ax, ay)
		dc.LineTo(ax+110, ay)
		dc.LineTo(ax, ay-120)
		dc.ClosePath()
		_ = dc.Fill()

		// ④ 1px 细边框 stroke（圆 + 圆角矩形）。
		ox, oy = cell(0, 1)
		dc.SetLineWidth(1 / scale)
		ax, ay = pc.Abs(ox+32, oy+32)
		dc.SetRGBA(0.96, 0.96, 0.98, 1)
		dc.DrawRoundedRectangle(ax, ay, 145, cellH-64, 45)
		_ = dc.Stroke()
		ax, ay = pc.Abs(ox+cellW-pad-80, oy+cellH/2)
		dc.DrawCircle(ax, ay, 78)
		_ = dc.Stroke()

		// ⑤ 斜 1px 细线 30/45/60° + 水平（端点带小数，逼出部分覆盖）。
		ox, oy = cell(1, 1)
		dc.SetLineWidth(1 / scale)
		x0, y0 := pc.Abs(ox+37.5, oy+cellH-pad-30.5)
		x1, y1 := pc.Abs(ox+cellW-pad-20.5, oy+45.5)
		dc.SetRGBA(0.95, 0.45, 0.4, 1)
		dc.DrawLine(x0, y0, x1, y1)
		_ = dc.Stroke()
		x0, y0 = pc.Abs(ox+42.7, oy+cellH-pad-15.3)
		x1, y1 = pc.Abs(ox+cellW-pad-25.7, oy+pad+15.3)
		dc.SetRGBA(0.45, 0.85, 0.5, 1)
		dc.DrawLine(x0, y0, x1, y1)
		_ = dc.Stroke()
		x0, y0 = pc.Abs(ox+27.2, oy+cellH-pad-25.1)
		x1, y1 = pc.Abs(ox+cellW-pad-10.2, oy+pad+25.1)
		dc.SetRGBA(0.5, 0.6, 0.95, 1)
		dc.DrawLine(x0, y0, x1, y1)
		_ = dc.Stroke()
		x0, y0 = pc.Abs(ox+37, oy+cellH-pad-8.5)
		x1, y1 = pc.Abs(ox+cellW-pad-37, oy+cellH-pad-8.5)
		dc.SetRGBA(0.9, 0.85, 0.4, 1)
		dc.DrawLine(x0, y0, x1, y1)
		_ = dc.Stroke()

		// ⑥ 弧线 stroke（圆弧 + 椭圆弧）。
		ox, oy = cell(2, 1)
		cx, cy = pc.Abs(ox+cellW/2, oy+cellH/2)
		dc.SetLineWidth(2 / scale)
		dc.SetRGBA(0.9, 0.4, 0.5, 1)
		dc.DrawArc(cx, cy, 100, 0.3, 2.6)
		_ = dc.Stroke()
		dc.SetRGBA(0.4, 0.75, 0.95, 1)
		dc.DrawEllipticalArc(cx, cy, 90, 55, 3.4, 5.8)
		_ = dc.Stroke()
	}

	shell.Body.Place(canvas, 0, 0)
	shell.Body.Place(wrkit.Label(mode+" — 上六宫格：圆角/斜边/细边框/细线/弧线", 13, 0.9, 0.92, 0.97), 4, 2*cellH+2*pad+8)

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor:       time.Duration(secs) * time.Second,
		WarmUp:       true,
		SnapshotPath: os.Getenv("SNAP_PNG"),
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "ui_ant_verify: close (%s)\n", win.Backend())
			case platform.EventResize:
				// NOTE: 不响应 resize —— exhost/x11 下 shell.Resize 会触发
				// 窗口↔surface 尺寸反馈振荡（1200↔1435 循环）。验证示例固定
				// 1200x800 布局即可。
				fmt.Fprintf(os.Stderr, "ui_ant_verify: resize ev=%dx%d (ignored)\n", ev.Width, ev.Height)
			}
		},
	})

	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		app.ScheduleFrame()
		proc.Sample()
		shell.NoteHUDTick(dt)
		shell.UpdateHUD("ANT", "steady", app, true, mode, "")
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

	fmt.Fprintf(os.Stderr, "ui_ant_verify: OK mode=%s presents=%d\n",
		mode, app.PresentCount())
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
