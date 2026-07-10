// GPU Test: Anti-aliasing
package main

import (
	"github.com/energye/gpui/gpui"
	"github.com/energye/gpui/internal/gg"
	"github.com/energye/lcl/api/libname"
)

func main() {
	libname.UseWS = "gtk3"
	app := gpui.NewApplication()
	win := gpui.NewWindow(gpui.WindowConfig{Title: "AA Test", Width: 800, Height: 600})
	win.OnInit(func(ctrl *gpui.TGPUControl) {
		ctrl.SetOnRender(func(ctx *gg.Context) {
			ctx.ClearWithColor(gg.RGBA{R: 1, G: 1, B: 1, A: 1})
			// Small round rect
			ctx.SetRGBA(0.2, 0.5, 1, 1)
			ctx.DrawRoundedRectangle(100, 100, 80, 32, 4)
			ctx.Fill()
			// Medium round rect
			ctx.SetRGBA(0.9, 0.95, 1, 1)
			ctx.DrawRoundedRectangle(200, 100, 200, 40, 8)
			ctx.Fill()
			// Circle
			ctx.SetRGBA(1, 0.5, 0, 1)
			ctx.DrawCircle(600, 300, 60)
			ctx.Fill()
			// Lines at various angles
			ctx.SetRGBA(0, 0, 0, 1)
			for i := 0; i < 8; i++ {
				a := float64(i) * 3.14159 / 8
				ctx.MoveTo(400, 450)
				ctx.LineTo(400+150*cos(a), 450+150*sin(a))
				ctx.Stroke()
			}
			ctx.SavePNG("gpu_anti_aliasing.png")
		})
	})
	app.AddWindow(win)
	app.Run()
}
func cos(x float64) float64 { return 1 - x*x/2 + x*x*x*x/24 }
func sin(x float64) float64 { return x - x*x*x/6 + x*x*x*x*x/120 }