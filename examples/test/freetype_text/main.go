// GPU Test: Freetype Text
package main

import (
	"github.com/energye/gpui/gpui"
	"github.com/energye/gpui/internal/gg"
	"github.com/energye/lcl/api/libname"
)

func main() {
	libname.UseWS = "gtk3"
	app := gpui.NewApplication()
	win := gpui.NewWindow(gpui.WindowConfig{Title: "Text Test", Width: 800, Height: 600})
	win.OnInit(func(ctrl *gpui.TGPUControl) {
		ctrl.SetOnRender(func(ctx *gg.Context) {
			ctx.ClearWithColor(gg.RGBA{R: 1, G: 1, B: 1, A: 1})
			ctx.SetRGBA(0, 0, 0, 1)
			// Draw text at various positions
			ctx.DrawString("Hello World 你好世界", 100, 100)
			ctx.DrawString("GPU Text Rendering", 100, 150)
			ctx.DrawString("Anti-aliased Text", 100, 200)
			ctx.DrawString("ABCDEFGHIJKLMNOPQRSTUVWXYZ", 100, 250)
			ctx.DrawString("abcdefghijklmnopqrstuvwxyz", 100, 300)
			ctx.DrawString("0123456789!@#$%^&*()", 100, 350)
			ctx.SavePNG("gpu_freetype_text.png")
		})
	})
	app.AddWindow(win)
	app.Run()
}