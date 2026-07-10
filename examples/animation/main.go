//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License 2.0
//
//----------------------------------------

package main

import (
	"math"

	"github.com/energye/gpui/gpui"
	"github.com/energye/gpui/internal/gg"
	"github.com/energye/lcl/api/libname"
)

func main() {
	libname.UseWS = "gtk3"

	app := gpui.NewApplication()

	win := gpui.NewWindow(gpui.WindowConfig{
		Title:  "GPUI - Animation",
		Width:  800,
		Height: 600,
	})

	var angle float64

	win.OnInit(func(ctrl *gpui.TGPUControl) {
		ctrl.SetOnRender(func(ctx *gg.Context) {
			ctx.ClearWithColor(gg.RGBA{R: 0.95, G: 0.95, B: 0.98, A: 1})

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

			angle += 0.02
		})
		ctrl.StartAnimation()
	})

	app.AddWindow(win)
	app.Run()
}