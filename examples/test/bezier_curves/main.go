// GPU Test: Bezier Curves
package main

import (
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
			ctx.SetLineWidth(2)
			// Cubic bezier curves
			curves := []struct {
				x1, y1, x2, y2, x3, y3, x4, y4 float64
			}{
				{100, 300, 200, 100, 300, 500, 400, 300},
				{400, 300, 500, 500, 600, 100, 700, 300},
			}
			ctx.SetRGBA(0, 0, 0, 1)
			for _, c := range curves {
				ctx.MoveTo(c.x1, c.y1)
				ctx.CubicTo(c.x2, c.y2, c.x3, c.y3, c.x4, c.y4)
				ctx.Stroke()
			}
			// Control points
			ctx.SetRGBA(1, 0, 0, 0.5)
			for _, c := range curves {
				ctx.DrawCircle(c.x2, c.y2, 3)
				ctx.DrawCircle(c.x3, c.y3, 3)
				ctx.Fill()
			}
			ctx.SavePNG("gpu_bezier_curves.png")
		})
	})
	app.AddWindow(win)
	app.Run()
}