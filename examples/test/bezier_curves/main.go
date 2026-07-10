// GPU Test: Bezier Curves - 贝塞尔曲线渲染验证
//
// 预期效果:
// =========
// 1. S形曲线: (100,200)-(400,200), 控制点(200,50)(300,350), 黑色线宽2px
// 2. 弧形曲线: (400,200)-(700,200), 控制点(500,50)(600,50), 黑色线宽2px
// 3. 回环曲线: (100,400)-(400,400), 控制点(200,550)(300,250), 黑色线宽2px
// 4. 尖角曲线: (400,400)-(700,400), 控制点(500,550)(550,550), 黑色线宽2px
// 5. 控制点: 红色圆点(r=4), 灰色虚线连接线
// 6. 起点终点: 绿色圆点(r=3)
// 7. 文字标注: 显示曲线名称和坐标范围
//
// 验证标准:
// - 曲线平滑, 无锯齿
// - 曲线通过起点和终点
// - 控制点影响曲线形状正确
// - 无自交现象(回环曲线除外)
package main

import (
	"fmt"
	"github.com/energye/gpui/gpui"
	"github.com/energye/gpui/internal/gg"
	"github.com/energye/lcl/api/libname"
)

func main() {
	libname.UseWS = "gtk3"
	app := gpui.NewApplication()
	win := gpui.NewWindow(gpui.WindowConfig{Title: "Bezier Test", Width: 800, Height: 600})

	win.OnInit(func(ctrl *gpui.TGPUControl) {
		ctrl.SetOnRender(func(ctx *gg.Context) {
			ctx.ClearWithColor(gg.RGBA{R: 1, G: 1, B: 1, A: 1})
			ctx.LoadFontFace("../NotoSansCJK-Regular.ttc", 13)

			curves := []struct{x1,y1,x2,y2,x3,y3,x4,y4 float64; name string}{
				{100,200,200,50,300,350,400,200,"S-Curve"},
				{400,200,500,50,600,50,700,200,"Arc"},
				{100,400,200,550,300,250,400,400,"Loop"},
				{400,400,500,550,550,550,700,400,"Sharp"},
			}
			for _, c := range curves {
				ctx.SetDash(5,3); ctx.SetLineWidth(1)
				ctx.SetRGBA(0.7,0.7,0.7,0.5)
				ctx.MoveTo(c.x1,c.y1); ctx.LineTo(c.x2,c.y2); ctx.LineTo(c.x3,c.y3); ctx.LineTo(c.x4,c.y4)
				ctx.Stroke(); ctx.SetDash()
				ctx.SetRGBA(0,0,0,1); ctx.SetLineWidth(2)
				ctx.MoveTo(c.x1,c.y1); ctx.CubicTo(c.x2,c.y2,c.x3,c.y3,c.x4,c.y4); ctx.Stroke()
				ctx.SetRGBA(1,0,0,0.8)
				ctx.DrawCircle(c.x2,c.y2,4); ctx.DrawCircle(c.x3,c.y3,4); ctx.Fill()
				ctx.SetRGBA(0,0.5,0,1)
				ctx.DrawCircle(c.x1,c.y1,3); ctx.DrawCircle(c.x4,c.y4,3); ctx.Fill()
				ctx.SetRGBA(0,0,0,1)
				ctx.DrawString(fmt.Sprintf("%s: (%.0f,%.0f)-(%.0f,%.0f)",c.name,c.x1,c.y1,c.x4,c.y4),c.x1,c.y1-20)
			}
			ctx.SavePNG("gpu_bezier_curves.png")
		})
	})
	app.AddWindow(win)
	app.Run()
}