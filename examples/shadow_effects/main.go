// GPU Test: Shadow Effects - 阴影效果渲染验证
//
// 预期效果:
// =========
// 1. 重叠矩形: 3个半透明矩形(蓝/红/绿)互相重叠, 透明度0.6
// 2. 重叠圆: 3个半透明圆(红/绿/蓝)互相重叠, 透明度0.3
// 3. 径向透明: 径向渐变从中心黑色到边缘透明
// 4. 文字标注: 说明每种效果类型
//
// 验证标准:
// - 透明度混合正确
// - 重叠区域颜色叠加效果正确
// - 渐变透明度过渡平滑
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
	win := ui.NewWindow(ui.WindowConfig{Title: "Shadow Test", Width: 800, Height: 600})

	win.OnInit(func(ctrl *ui.TGPUControl) {
		src, err := text.NewFontSource(examples.Font)
		if err != nil {
			log.Fatalf("Font load error: %v", err)
		}
		face14 := src.Face(14)

		ctrl.SetOnRender(func(ctx *render.Context) {
			ctx.ClearWithColor(render.RGBA{R: 0.95, G: 0.95, B: 0.95, A: 1})
			ctx.SetFont(face14)

			ctx.SetFillBrush(render.Solid(render.RGBA{R: 0.2, G: 0.4, B: 0.8, A: 0.6}))
			ctx.DrawRectangle(100, 100, 150, 150)
			ctx.Fill()
			ctx.SetFillBrush(render.Solid(render.RGBA{R: 0.8, G: 0.2, B: 0.2, A: 0.6}))
			ctx.DrawRectangle(180, 130, 150, 150)
			ctx.Fill()
			ctx.SetFillBrush(render.Solid(render.RGBA{R: 0.2, G: 0.8, B: 0.2, A: 0.6}))
			ctx.DrawRectangle(140, 170, 150, 150)
			ctx.Fill()
			ctx.SetRGBA(0, 0, 0, 1)
			ctx.DrawString("Alpha Blended Rectangles", 100, 80)

			ctx.SetFillBrush(render.Solid(render.RGBA{R: 1, G: 0, B: 0, A: 0.3}))
			ctx.DrawCircle(500, 150, 60)
			ctx.Fill()
			ctx.SetFillBrush(render.Solid(render.RGBA{R: 0, G: 1, B: 0, A: 0.3}))
			ctx.DrawCircle(550, 150, 60)
			ctx.Fill()
			ctx.SetFillBrush(render.Solid(render.RGBA{R: 0, G: 0, B: 1, A: 0.3}))
			ctx.DrawCircle(525, 120, 60)
			ctx.Fill()
			ctx.SetRGBA(0, 0, 0, 1)
			ctx.DrawString("Alpha Blended Circles", 400, 80)

			g := render.NewRadialGradientBrush(500, 400, 10, 100)
			g.AddColorStop(0, render.RGBA{R: 0, G: 0, B: 0, A: 0.8})
			g.AddColorStop(1, render.RGBA{R: 0, G: 0, B: 0, A: 0})
			ctx.SetFillBrush(g)
			ctx.DrawCircle(500, 400, 100)
			ctx.Fill()
			ctx.SetRGBA(0, 0, 0, 1)
			ctx.DrawString("Radial Transparency", 450, 520)

			ctx.SavePNG(filepath.Join(exec.CurrentDir, "examples/shadow_effects/gpu_shadow_effects.png"))
		})
	})
	app.AddWindow(win)
	app.Run()
}
