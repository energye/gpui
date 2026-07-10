// GPU Test: Shadow Effects
package main

import (
	"github.com/energye/gpui/gpui"
	"github.com/energye/gpui/internal/gg"
	"github.com/energye/lcl/api/libname"
)

func main() {
	libname.UseWS = "gtk3"
	app := gpui.NewApplication()
	win := gpui.NewWindow(gpui.WindowConfig{Title: "Shadow Test", Width: 800, Height: 600})
	win.OnInit(func(ctrl *gpui.TGPUControl) {
		ctrl.SetOnRender(func(ctx *gg.Context) {
			ctx.ClearWithColor(gg.RGBA{R: 1, G: 1, B: 1, A: 1})
			// Draw overlapping rectangles with transparency
			ctx.SetRGBA(0.2, 0.4, 0.8, 0.8)
			ctx.DrawRectangle(100, 100, 200, 200)
			ctx.Fill()
			ctx.SetRGBA(0.8, 0.2, 0.2, 0.8)
			ctx.DrawRectangle(200, 150, 200, 200)
			ctx.Fill()
			ctx.SetRGBA(0.2, 0.8, 0.2, 0.8)
			ctx.DrawRectangle(150, 200, 200, 200)
			ctx.Fill()
			ctx.SavePNG("gpu_shadow_effects.png")
		})
	})
	app.AddWindow(win)
	app.Run()
}