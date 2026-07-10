// GPU Test: Gradient Effects - 渐变渲染验证
//
// 预期效果:
// =========
// 1. 水平线性渐变: (100,100)-(300,100) 红→绿→蓝 三色渐变
// 2. 垂直线性渐变: (400,100)-(400,180) 黄→紫→青 三色渐变
// 3. 对角线渐变: (100,250)-(300,400) 红→蓝 双色渐变
// 4. 径向渐变: 中心(600,250) 内径10px 外径80px 白→橙→红
// 5. 偏移径向渐变: 中心(150,500) 内径5px 外径60px 白→蓝
// 6. 文字标注: 每种渐变类型下方有名称标签
//
// 验证标准:
// - 渐变过渡平滑
// - 颜色过渡无条纹
// - 径向渐变圆形完美
// - 文字标注清晰
package main

import (
	"github.com/energye/gpui/gpui"
	"github.com/energye/gpui/internal/gg"
	"github.com/energye/lcl/api/libname"
)

func main() {
	libname.UseWS = "gtk3"
	app := gpui.NewApplication()
	win := gpui.NewWindow(gpui.WindowConfig{Title: "Gradient Test", Width: 800, Height: 600})

	win.OnInit(func(ctrl *gpui.TGPUControl) {
		ctrl.SetOnRender(func(ctx *gg.Context) {
			ctx.ClearWithColor(gg.RGBA{R: 1, G: 1, B: 1, A: 1})
			ctx.LoadFontFace("../NotoSansCJK-Regular.ttc", 14)

			g1 := gg.NewLinearGradientBrush(100,100,300,100)
			g1.AddColorStop(0, gg.RGBA{R:1,G:0,B:0,A:1}); g1.AddColorStop(0.5, gg.RGBA{R:0,G:1,B:0,A:1}); g1.AddColorStop(1, gg.RGBA{R:0,G:0,B:1,A:1})
			ctx.SetFillBrush(g1); ctx.DrawRectangle(100,100,200,80); ctx.Fill()
			ctx.SetRGBA(0,0,0,1); ctx.DrawString("Horizontal Linear", 100, 90)

			g2 := gg.NewLinearGradientBrush(400,100,400,180)
			g2.AddColorStop(0, gg.RGBA{R:1,G:1,B:0,A:1}); g2.AddColorStop(0.5, gg.RGBA{R:0.8,G:0,B:0.8,A:1}); g2.AddColorStop(1, gg.RGBA{R:0,G:1,B:1,A:1})
			ctx.SetFillBrush(g2); ctx.DrawRectangle(400,100,200,80); ctx.Fill()
			ctx.SetRGBA(0,0,0,1); ctx.DrawString("Vertical Linear", 400, 90)

			g3 := gg.NewLinearGradientBrush(100,250,300,400)
			g3.AddColorStop(0, gg.RGBA{R:1,G:0,B:0,A:1}); g3.AddColorStop(1, gg.RGBA{R:0,G:0,B:1,A:1})
			ctx.SetFillBrush(g3); ctx.DrawRectangle(100,250,200,150); ctx.Fill()
			ctx.SetRGBA(0,0,0,1); ctx.DrawString("Diagonal Linear", 100, 240)

			g4 := gg.NewRadialGradientBrush(600,250,10,80)
			g4.AddColorStop(0, gg.RGBA{R:1,G:1,B:1,A:1}); g4.AddColorStop(0.5, gg.RGBA{R:1,G:0.5,B:0,A:1}); g4.AddColorStop(1, gg.RGBA{R:1,G:0,B:0,A:1})
			ctx.SetFillBrush(g4); ctx.DrawCircle(600,250,80); ctx.Fill()
			ctx.SetRGBA(0,0,0,1); ctx.DrawString("Radial Gradient", 560, 160)

			g5 := gg.NewRadialGradientBrush(150,500,5,60)
			g5.AddColorStop(0, gg.RGBA{R:1,G:1,B:1,A:1}); g5.AddColorStop(1, gg.RGBA{R:0.2,G:0.4,B:0.8,A:1})
			ctx.SetFillBrush(g5); ctx.DrawCircle(150,500,60); ctx.Fill()
			ctx.SetRGBA(0,0,0,1); ctx.DrawString("Offset Radial", 110, 440)

			ctx.SavePNG("gpu_gradient_effects.png")
		})
	})
	app.AddWindow(win)
	app.Run()
}