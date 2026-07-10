// GPU Test: Gradient Effects
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
			// Linear gradient brush
			g := gg.NewLinearGradientBrush(100, 100, 300, 300)
			g.AddColorStop(0, gg.RGBA{R: 1, G: 0, B: 0, A: 1})
			g.AddColorStop(0.5, gg.RGBA{R: 0, G: 1, B: 0, A: 1})
			g.AddColorStop(1, gg.RGBA{R: 0, G: 0, B: 1, A: 1})
			ctx.SetFillBrush(g)
			ctx.DrawRectangle(100, 100, 200, 200)
			ctx.Fill()
			// Radial gradient brush
			g2 := gg.NewRadialGradientBrush(500, 200, 10, 100)
			g2.AddColorStop(0, gg.RGBA{R: 1, G: 1, B: 0, A: 1})
			g2.AddColorStop(1, gg.RGBA{R: 0.8, G: 0, B: 0.8, A: 1})
			ctx.SetFillBrush(g2)
			ctx.DrawCircle(500, 200, 100)
			ctx.Fill()
			ctx.SavePNG("gpu_gradient_effects.png")
		})
	})
	app.AddWindow(win)
	app.Run()
}