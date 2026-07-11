//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License 2.0
//
//----------------------------------------

package main

import (
	"github.com/energye/gpui/examples"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui"
	"github.com/energye/lcl/api/libname"
	"math"
)

func main() {
	libname.UseWS = "gtk3"

	app := ui.NewApplication()

	win := ui.NewWindow(ui.WindowConfig{
		Title:  "GPUI - Animation",
		Width:  800,
		Height: 600,
	})

	var face14 text.Face
	src, err := text.NewFontSource(examples.Font)
	if err == nil {
		face14 = src.Face(14)
	}

	var angle float64

	win.OnInit(func(ctrl *ui.TGPUControl) {
		ctrl.SetOnRender(func(ctx *render.Context) {
			ctx.ClearWithColor(render.RGBA{R: 0.95, G: 0.95, B: 0.98, A: 1})

			// Rotating circle
			ctx.SetRGBA(0.2, 0.4, 0.8, 1.0)
			ctx.DrawCircle(400, 300, 150)
			ctx.Fill()

			// Rotating line
			ctx.SetRGBA(1, 0, 0, 1)
			ctx.SetLineWidth(3)
			ctx.MoveTo(400, 300)
			ctx.LineTo(400+150*math.Cos(angle), 300+150*math.Sin(angle))
			ctx.Stroke()

			if face14 != nil {
				ctx.SetFont(face14)
				ctx.SetRGBA(0, 0, 0, 1)
				ctx.DrawString("GPUI Animation", 350, 500)
			}

			angle += 0.02
		})
		ctrl.StartAnimation()
	})

	app.AddWindow(win)
	app.Run()
}
