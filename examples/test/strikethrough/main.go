// GPU Test: Strikethrough
package main

import (
	"github.com/energye/gpui/gpui"
	"github.com/energye/gpui/internal/gg"
	"github.com/energye/lcl/api/libname"
)

func main() {
	libname.UseWS = "gtk3"
	app := gpui.NewApplication()
	win := gpui.NewWindow(gpui.WindowConfig{Title: "Strikethrough", Width: 800, Height: 600})
	win.OnInit(func(ctrl *gpui.TGPUControl) {
		ctrl.SetOnRender(func(ctx *gg.Context) {
			ctx.ClearWithColor(gg.RGBA{R: 1, G: 1, B: 1, A: 1})
			ctx.SetRGBA(0, 0, 0, 1)
			ctx.DrawString("Text with strikethrough", 100, 100)
			ctx.SetLineWidth(1)
			ctx.MoveTo(100, 108)
			ctx.LineTo(350, 108)
			ctx.Stroke()
			ctx.DrawString("Another example", 100, 200)
			ctx.SetRGBA(1, 0, 0, 1)
			ctx.MoveTo(100, 208)
			ctx.LineTo(300, 208)
			ctx.Stroke()
			ctx.SavePNG("gpu_strikethrough.png")
		})
	})
	app.AddWindow(win)
	app.Run()
}