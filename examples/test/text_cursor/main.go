// GPU Test: Text Cursor
package main

import (
	"github.com/energye/gpui/gpui"
	"github.com/energye/gpui/internal/gg"
	"github.com/energye/lcl/api/libname"
)

func main() {
	libname.UseWS = "gtk3"
	app := gpui.NewApplication()
	win := gpui.NewWindow(gpui.WindowConfig{Title: "Text Cursor", Width: 800, Height: 600})
	win.OnInit(func(ctrl *gpui.TGPUControl) {
		ctrl.SetOnRender(func(ctx *gg.Context) {
			ctx.ClearWithColor(gg.RGBA{R: 1, G: 1, B: 1, A: 1})
			ctx.SetRGBA(0, 0, 0, 1)
			ctx.DrawString("Text with cursor|", 100, 100)
			ctx.DrawString("Blinking cursor position", 100, 200)
			ctx.SetRGBA(0, 0, 0, 0.5)
			ctx.SetLineWidth(1)
			ctx.MoveTo(280, 190)
			ctx.LineTo(280, 210)
			ctx.Stroke()
			ctx.SetRGBA(0, 0, 0, 1)
			ctx.DrawString("Multiple cursor positions:", 100, 300)
			ctx.SetRGBA(1, 0, 0, 1)
			ctx.SetLineWidth(2)
			ctx.MoveTo(380, 290)
			ctx.LineTo(380, 310)
			ctx.Stroke()
			ctx.SetRGBA(0, 0, 1, 1)
			ctx.MoveTo(450, 290)
			ctx.LineTo(450, 310)
			ctx.Stroke()
			ctx.SavePNG("gpu_text_cursor.png")
		})
	})
	app.AddWindow(win)
	app.Run()
}