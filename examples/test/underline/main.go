// GPU Test: Underline
package main

import (
	"github.com/energye/gpui/gpui"
	"github.com/energye/gpui/internal/gg"
	"github.com/energye/lcl/api/libname"
)

func main() {
	libname.UseWS = "gtk3"
	app := gpui.NewApplication()
	win := gpui.NewWindow(gpui.WindowConfig{Title: "Underline", Width: 800, Height: 600})
	win.OnInit(func(ctrl *gpui.TGPUControl) {
		ctrl.SetOnRender(func(ctx *gg.Context) {
			ctx.ClearWithColor(gg.RGBA{R: 1, G: 1, B: 1, A: 1})
			ctx.SetRGBA(0, 0, 0, 1)
			ctx.DrawString("Underlined text", 100, 100)
			ctx.SetLineWidth(1)
			ctx.MoveTo(100, 106)
			ctx.LineTo(280, 106)
			ctx.Stroke()
			ctx.SetRGBA(0, 0, 1, 1)
			ctx.DrawString("Blue underlined text", 100, 200)
			ctx.SetRGBA(0, 0, 1, 1)
			ctx.MoveTo(100, 206)
			ctx.LineTo(330, 206)
			ctx.Stroke()
			ctx.SavePNG("gpu_underline.png")
		})
	})
	app.AddWindow(win)
	app.Run()
}