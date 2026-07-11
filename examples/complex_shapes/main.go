// GPU Test: Complex Shapes - 复杂图形渲染验证
//
// 预期效果:
// =========
// 1. 正六边形: 中心(200,200) 半径60px 蓝色填充 黑色边框
// 2. 五角星: 中心(450,200) 外径60px 内径25px 金色填充
// 3. 菱形: 中心(700,200) 60x60 紫色填充
// 4. 圆环: 中心(900,200) 外径60px 内径40px 绿色填充
// 5. 十字形: 中心(1100,200) 60x60 红色填充
// 6. 齿轮: 中心(200,450) 半径60px 12齿 蓝色填充
// 7. 心形: 中心(450,450) 40px 红色填充
// 8. 文字标注: 每个图形下方有名称标签
//
// 验证标准:
// - 所有形状边缘平滑无锯齿
// - 填充完整无遗漏
// - 形状正确无变形
package main

import (
	"github.com/energye/gpui/examples"
	"github.com/energye/lcl/tool/exec"
	"math"
	"path/filepath"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui"
	"github.com/energye/lcl/api/libname"
)

func main() {
	libname.UseWS = "gtk3"
	app := ui.NewApplication()
	win := ui.NewWindow(ui.WindowConfig{Title: "Complex Shapes", Width: 800, Height: 600})

	win.OnInit(func(ctrl *ui.TGPUControl) {

		face := examples.Face

		ctrl.SetOnRender(func(ctx *render.Context) {
			ctx.ClearWithColor(render.RGBA{R: 1, G: 1, B: 1, A: 1})
			ctx.SetFont(face(14))

			// 1. 正六边形
			p := render.NewPath()
			for i := 0; i < 6; i++ {
				a := float64(i) * math.Pi / 3
				if i == 0 {
					p.MoveTo(200+60*math.Cos(a), 200+60*math.Sin(a))
				} else {
					p.LineTo(200+60*math.Cos(a), 200+60*math.Sin(a))
				}
			}
			p.Close()
			ctx.SetFillRule(render.FillRuleNonZero)
			ctx.SetFillBrush(render.Solid(render.RGBA{R: 0.2, G: 0.6, B: 1, A: 1}))
			ctx.FillPath(p)
			ctx.SetStrokeBrush(render.Solid(render.RGBA{R: 0, G: 0, B: 0, A: 0.3}))
			ctx.SetLineWidth(2)
			ctx.StrokePath(p)
			ctx.SetRGBA(0, 0, 0, 1)
			ctx.DrawString("Hexagon", 180, 280)

			// 2. 五角星
			ctx.SetFillRule(render.FillRuleEvenOdd)
			p = render.NewPath()
			for i := 0; i < 10; i++ {
				a := float64(i)*math.Pi/5 - math.Pi/2
				r := 60.0
				if i%2 == 1 {
					r = 25
				}
				if i == 0 {
					p.MoveTo(450+r*math.Cos(a), 200+r*math.Sin(a))
				} else {
					p.LineTo(450+r*math.Cos(a), 200+r*math.Sin(a))
				}
			}
			p.Close()
			ctx.SetFillBrush(render.Solid(render.RGBA{R: 1, G: 0.8, B: 0, A: 1}))
			ctx.FillPath(p)
			ctx.SetStrokeBrush(render.Solid(render.RGBA{R: 0.8, G: 0.6, B: 0, A: 1}))
			ctx.SetLineWidth(1.5)
			ctx.StrokePath(p)
			ctx.SetRGBA(0, 0, 0, 1)
			ctx.DrawString("Star", 430, 280)

			// 3. 菱形
			p = render.NewPath()
			p.MoveTo(700, 140)
			p.LineTo(760, 200)
			p.LineTo(700, 260)
			p.LineTo(640, 200)
			p.Close()
			ctx.SetFillBrush(render.Solid(render.RGBA{R: 0.8, G: 0.2, B: 0.8, A: 1}))
			ctx.FillPath(p)
			ctx.SetRGBA(0, 0, 0, 1)
			ctx.DrawString("Diamond", 680, 280)

			// 4. 圆环
			ctx.SetFillRule(render.FillRuleNonZero)
			ctx.SetFillBrush(render.Solid(render.RGBA{R: 0.2, G: 0.8, B: 0.4, A: 1}))
			ctx.DrawCircle(900, 200, 60)
			ctx.Fill()
			ctx.SetFillBrush(render.Solid(render.RGBA{R: 1, G: 1, B: 1, A: 1}))
			ctx.DrawCircle(900, 200, 40)
			ctx.Fill()
			ctx.SetRGBA(0, 0, 0, 1)
			ctx.DrawString("Donut", 880, 280)

			// 5. 十字形
			p = render.NewPath()
			p.MoveTo(1100, 160)
			p.LineTo(1120, 160)
			p.LineTo(1120, 180)
			p.LineTo(1140, 180)
			p.LineTo(1140, 220)
			p.LineTo(1120, 220)
			p.LineTo(1120, 240)
			p.LineTo(1100, 240)
			p.LineTo(1100, 220)
			p.LineTo(1080, 220)
			p.LineTo(1080, 180)
			p.LineTo(1100, 180)
			p.Close()
			ctx.SetFillBrush(render.Solid(render.RGBA{R: 0.8, G: 0.2, B: 0.2, A: 1}))
			ctx.FillPath(p)
			ctx.SetRGBA(0, 0, 0, 1)
			ctx.DrawString("Cross", 1080, 260)

			// 6. 齿轮
			p = render.NewPath()
			for i := 0; i < 24; i++ {
				a := float64(i) * math.Pi / 12
				r := 60.0
				if i%2 == 0 {
					r = 55
				}
				if i == 0 {
					p.MoveTo(200+r*math.Cos(a), 450+r*math.Sin(a))
				} else {
					p.LineTo(200+r*math.Cos(a), 450+r*math.Sin(a))
				}
			}
			p.Close()
			ctx.SetFillBrush(render.Solid(render.RGBA{R: 0.2, G: 0.4, B: 0.8, A: 1}))
			ctx.FillPath(p)
			ctx.SetStrokeBrush(render.Solid(render.RGBA{R: 0, G: 0, B: 0, A: 0.3}))
			ctx.SetLineWidth(1)
			ctx.StrokePath(p)
			ctx.SetRGBA(0, 0, 0, 1)
			ctx.DrawString("Gear", 180, 530)

			// 7. 心形
			p = render.NewPath()
			p.MoveTo(450, 450)
			p.CubicTo(450, 420, 410, 400, 410, 440)
			p.CubicTo(410, 470, 450, 490, 450, 490)
			p.CubicTo(450, 490, 490, 470, 490, 440)
			p.CubicTo(490, 400, 450, 420, 450, 450)
			p.Close()
			ctx.SetFillBrush(render.Solid(render.RGBA{R: 1, G: 0.2, B: 0.2, A: 1}))
			ctx.FillPath(p)
			ctx.SetRGBA(0, 0, 0, 1)
			ctx.DrawString("Heart", 430, 510)

			ctx.SavePNG(filepath.Join(exec.CurrentDir, "examples/complex_shapes/gpu_complex_shapes.png"))
		})
	})
	app.AddWindow(win)
	app.Run()
}
