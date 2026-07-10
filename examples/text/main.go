//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License 2.0
//
//----------------------------------------

package main

import (
	"github.com/energye/gpui/gpui"
	"github.com/energye/gpui/internal/gg"
	"github.com/energye/lcl/api/libname"
)

func main() {
	libname.UseWS = "gtk3"

	app := gpui.NewApplication()

	win := gpui.NewWindow(gpui.WindowConfig{
		Title:  "GPUI - Text Rendering",
		Width:  800,
		Height: 600,
	})
	win.OnInit(func(ctrl *gpui.TGPUControl) {
		ctrl.SetOnRender(func(ctx *gg.Context) {
			ctx.ClearWithColor(gg.RGBA{R: 1, G: 1, B: 1, A: 1})

			ctx.SetRGBA(0.2, 0.4, 0.8, 1.0)
			ctx.DrawCircle(400, 300, 200)
			ctx.Fill()

			ctx.SetRGBA(0, 0, 0, 1)
			ctx.DrawString("Hello GPUI!", 300, 350)
		})
	})

	app.AddWindow(win)
	app.Run()
}