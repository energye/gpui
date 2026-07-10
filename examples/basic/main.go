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
		Title:  "GPUI - gg Rendering",
		Width:  800,
		Height: 600,
	})
	win.OnInit(func(ctrl *gpui.TGPUControl) {
		ctrl.SetOnRender(func(ctx *gg.Context) {
			// Draw a circle
			ctx.SetRGBA(0.2, 0.4, 0.8, 1.0)
			ctx.DrawCircle(400, 300, 200)
			ctx.Fill()

			// Draw a rectangle
			ctx.SetRGBA(0.8, 0.2, 0.2, 0.7)
			ctx.DrawRectangle(100, 100, 200, 150)
			ctx.Fill()

			// Draw text
			ctx.SetRGBA(0, 0, 0, 1)
			ctx.DrawString("Hello GPUI!", 300, 350)
		})
	})

	app.AddWindow(win)
	app.Run()
}
