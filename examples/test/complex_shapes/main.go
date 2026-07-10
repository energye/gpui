// GPU Test: Complex Shapes
package main

import (
	"github.com/energye/gpui/gpui"
	"github.com/energye/gpui/internal/gg"
	"github.com/energye/lcl/api/libname"
)

func main() {
	libname.UseWS = "gtk3"
	app := gpui.NewApplication()
	win := gpui.NewWindow(gpui.WindowConfig{Title: "Complex Shapes", Width: 800, Height: 600})
	win.OnInit(func(ctrl *gpui.TGPUControl) {
		ctrl.SetOnRender(func(ctx *gg.Context) {
			ctx.ClearWithColor(gg.RGBA{R: 1, G: 1, B: 1, A: 1})
			// Star
			ctx.SetRGBA(1, 0.7, 0.1, 1)
			drawStar(ctx, 200, 150, 60, 30, 5)
			ctx.Fill()
			// Concentric circles
			ctx.SetRGBA(0.2, 0.4, 0.8, 0.7)
			for i := 5; i > 0; i-- {
				ctx.DrawCircle(500, 200, float64(i*20))
				ctx.Stroke()
			}
			// Overlapping shapes
			ctx.SetRGBA(1, 0, 0, 0.3)
			ctx.DrawCircle(300, 400, 80)
			ctx.Fill()
			ctx.SetRGBA(0, 1, 0, 0.3)
			ctx.DrawCircle(370, 400, 80)
			ctx.Fill()
			ctx.SetRGBA(0, 0, 1, 0.3)
			ctx.DrawCircle(335, 340, 80)
			ctx.Fill()
			ctx.SavePNG("gpu_complex_shapes.png")
		})
	})
	app.AddWindow(win)
	app.Run()
}
func drawStar(ctx *gg.Context, cx, cy, outerR, innerR float64, points int) {
	for i := 0; i < points*2; i++ {
		r := outerR
		if i%2 == 1 { r = innerR }
		a := float64(i)*3.14159/float64(points) - 3.14159/2
		x := cx + r*cos(a)
		y := cy + r*sin(a)
		if i == 0 { ctx.MoveTo(x, y) } else { ctx.LineTo(x, y) }
	}
	ctx.ClosePath()
}
func cos(x float64) float64 { return 1 - x*x/2 + x*x*x*x/24 }
func sin(x float64) float64 { return x - x*x*x/6 + x*x*x*x*x/120 }