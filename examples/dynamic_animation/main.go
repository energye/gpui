// GPU Test: Dynamic Animation - 动态动画渲染验证
//
// 预期效果:
// =========
// 1. 旋转臂: 蓝色大圆(r=80) 中心(400,300), 红色臂从中心向外旋转
// 2. 轨道圆点: 金色小圆(r=10) 沿轨道旋转, 速度是臂的两倍
// 3. 脉冲球: 绿色半透明圆 半径在10-50px之间脉动
// 4. 角度显示: 显示当前旋转角度(度)
//
// 验证标准:
// - 动画流畅无卡顿
// - 旋转速度均匀
// - 脉冲效果平滑
// - 所有形状抗锯齿
package main

import (
	"fmt"
	"github.com/energye/gpui/examples"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui"
	"github.com/energye/lcl/api/libname"
	"github.com/energye/lcl/tool/exec"
	"log"
	"math"
	"path/filepath"
)

func main() {
	libname.UseWS = "gtk3"
	app := ui.NewApplication()
	win := ui.NewWindow(ui.WindowConfig{Title: "Animation", Width: 800, Height: 600})

	src, err := text.NewFontSource(examples.Font)
	if err != nil {
		log.Fatalf("Font load error: %v", err)
	}
	face14 := src.Face(14)

	var angle float64 = 0
	win.OnInit(func(ctrl *ui.TGPUControl) {
		ctrl.SetOnRender(func(ctx *render.Context) {
			ctx.ClearWithColor(render.RGBA{R: 0.95, G: 0.95, B: 0.98, A: 1})
			ctx.SetFont(face14)

			ctx.SetFillBrush(render.Solid(render.RGBA{R: 0.2, G: 0.4, B: 0.8, A: 1}))
			ctx.DrawCircle(400, 300, 80)
			ctx.Fill()
			ctx.SetStrokeBrush(render.Solid(render.RGBA{R: 1, G: 0, B: 0, A: 1}))
			ctx.SetLineWidth(3)
			ctx.MoveTo(400, 300)
			ctx.LineTo(400+120*math.Cos(angle), 300+120*math.Sin(angle))
			ctx.Stroke()
			ctx.SetFillBrush(render.Solid(render.RGBA{R: 1, G: 0.8, B: 0, A: 1}))
			ctx.DrawCircle(400+80*math.Cos(angle*2), 300+80*math.Sin(angle*2), 10)
			ctx.Fill()
			pulse := 30 + 20*math.Sin(angle)
			ctx.SetFillBrush(render.Solid(render.RGBA{R: 0.2, G: 0.8, B: 0.2, A: 0.5}))
			ctx.DrawCircle(400, 300, pulse)
			ctx.Fill()

			ctx.SetRGBA(0, 0, 0, 1)
			ctx.DrawString("Rotating Animation", 350, 450)
			ctx.DrawString(fmt.Sprintf("Angle: %d°", int(angle*180/math.Pi)), 350, 470)
			angle += 0.03
			if angle > 2*math.Pi {
				angle -= 2 * math.Pi
			}
			ctx.SavePNG(filepath.Join(exec.CurrentDir, "examples/dynamic_animation/gpu_dynamic_animation.png"))
		})
		ctrl.StartAnimation()
	})
	app.AddWindow(win)
	app.Run()
}
