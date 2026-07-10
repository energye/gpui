// GPU Test: Arrows
package main

import (
	"github.com/energye/gpui/gpui"
	"github.com/energye/gpui/internal/gg"
	"github.com/energye/lcl/api/libname"
)

func main() {
	libname.UseWS = "gtk3"
	app := gpui.NewApplication()
	win := gpui.NewWindow(gpui.WindowConfig{Title: "Arrow Test", Width: 800, Height: 600})
	win.OnInit(func(ctrl *gpui.TGPUControl) {
		ctrl.SetOnRender(func(ctx *gg.Context) {
			ctx.ClearWithColor(gg.RGBA{R: 1, G: 1, B: 1, A: 1})
			ctx.SetLineWidth(2)
			// Draw arrows in 4 directions
			colors := []gg.RGBA{{R: 1, G: 0, B: 0, A: 1}, {R: 0, G: 1, B: 0, A: 1}, {R: 0, G: 0, B: 1, A: 1}, {R: 1, G: 1, B: 0, A: 1}}
			for i, col := range colors {
				ctx.SetRGBA(col.R, col.G, col.B, col.A)
				cx := 200.0 + float64(i)*150
				cy := 300.0
				ctx.MoveTo(cx-50, cy)
				ctx.LineTo(cx+50, cy)
				ctx.LineTo(cx+30, cy-20)
				ctx.MoveTo(cx+50, cy)
				ctx.LineTo(cx+30, cy+20)
				ctx.Stroke()
			}
			ctx.SavePNG("gpu_arrow.png")
		})
	})
	app.AddWindow(win)
	app.Run()
}