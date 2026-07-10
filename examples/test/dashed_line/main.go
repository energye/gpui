// GPU Test: Dashed Line
package main

import (
	"github.com/energye/gpui/gpui"
	"github.com/energye/gpui/internal/gg"
	"github.com/energye/lcl/api/libname"
)

func main() {
	libname.UseWS = "gtk3"
	app := gpui.NewApplication()
	win := gpui.NewWindow(gpui.WindowConfig{Title: "Dashed Line", Width: 800, Height: 600})
	win.OnInit(func(ctrl *gpui.TGPUControl) {
		ctrl.SetOnRender(func(ctx *gg.Context) {
			ctx.ClearWithColor(gg.RGBA{R: 1, G: 1, B: 1, A: 1})
			ctx.SetLineWidth(2)
			dashes := []float64{10, 5, 20, 10, 5, 5}
			colors := []gg.RGBA{{R: 1, G: 0, B: 0, A: 1}, {R: 0, G: 1, B: 0, A: 1}, {R: 0, G: 0, B: 1, A: 1}, {R: 1, G: 1, B: 0, A: 1}}
			for i, col := range colors {
				ctx.SetRGBA(col.R, col.G, col.B, col.A)
				ctx.SetDash(dashes[i*2], dashes[i*2+1])
				ctx.MoveTo(100, 100+float64(i*100))
				ctx.LineTo(700, 100+float64(i*100))
				ctx.Stroke()
			}
			ctx.SetDash()
			ctx.SavePNG("gpu_dashed_line.png")
		})
	})
	app.AddWindow(win)
	app.Run()
}