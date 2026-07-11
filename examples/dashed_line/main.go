// GPU Test: Dashed Line - 虚线渲染验证
//
// 预期效果:
// =========
// 1. 实线: 线宽1px 黑色, 连续无间断
// 2. 红色虚线: 线宽2px 8px实+4px空
// 3. 绿色宽虚线: 线宽3px 12px实+6px空
// 4. 蓝色点线: 线宽1px 4px实+4px空
// 5. 黄色长虚线: 线宽2px 20px实+10px空
// 6. 紫色细虚线: 线宽1.5px 6px实+3px空
// 7. 文字标注: 每种虚线样式左侧有名称标签
//
// 验证标准:
// - 虚线间距均匀
// - 线端平滑
// - 不同线宽显示正确
package main

import (
	"github.com/energye/gpui/examples"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui"
	"github.com/energye/lcl/api/libname"
	"github.com/energye/lcl/tool/exec"
	"log"
	"path/filepath"
)

func main() {
	libname.UseWS = "gtk3"
	app := ui.NewApplication()
	win := ui.NewWindow(ui.WindowConfig{Title: "Dashed Line", Width: 800, Height: 600})

	win.OnInit(func(ctrl *ui.TGPUControl) {
		src, err := text.NewFontSource(examples.Font)
		if err != nil {
			log.Fatalf("Font load error: %v", err)
		}
		face14 := src.Face(14)

		ctrl.SetOnRender(func(ctx *render.Context) {
			ctx.ClearWithColor(render.RGBA{R: 1, G: 1, B: 1, A: 1})
			ctx.SetFont(face14)

			patterns := []struct {
				dash, gap, width float64
				color            render.RGBA
				name             string
			}{
				{0, 0, 1, render.RGBA{R: 0, G: 0, B: 0, A: 1}, "Solid"},
				{8, 4, 2, render.RGBA{R: 1, G: 0, B: 0, A: 1}, "Red Dash"},
				{12, 6, 3, render.RGBA{R: 0, G: 1, B: 0, A: 1}, "Green Wide"},
				{4, 4, 1, render.RGBA{R: 0, G: 0, B: 1, A: 1}, "Blue Dotted"},
				{20, 10, 2, render.RGBA{R: 1, G: 1, B: 0, A: 1}, "Yellow Long"},
				{6, 3, 1.5, render.RGBA{R: 0.8, G: 0, B: 0.8, A: 1}, "Purple Fine"},
			}
			for i, p := range patterns {
				y := 60.0 + float64(i)*80
				ctx.SetDash(p.dash, p.gap)
				ctx.SetLineWidth(p.width)
				ctx.SetRGBA(p.color.R, p.color.G, p.color.B, p.color.A)
				ctx.MoveTo(100, y)
				ctx.LineTo(700, y)
				ctx.Stroke()
				ctx.SetRGBA(0, 0, 0, 1)
				ctx.DrawString(p.name, 100, y-20)
			}
			ctx.SetDash()
			ctx.SavePNG(filepath.Join(exec.CurrentDir, "examples/dashed_line/gpu_dashed_line.png"))
		})
	})
	app.AddWindow(win)
	app.Run()
}
