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
)

func main() {
	libname.UseWS = "gtk3"

	app := ui.NewApplication()
	var face14 text.Face
	src, err := text.NewFontSource(examples.Font)
	if err == nil {
		face14 = src.Face(14)
	}

	win := ui.NewWindow(ui.WindowConfig{
		Title:  "GPUI - Text Rendering",
		Width:  800,
		Height: 600,
	})
	win.OnInit(func(ctrl *ui.TGPUControl) {
		ctrl.SetOnRender(func(ctx *render.Context) {
			ctx.ClearWithColor(render.RGBA{R: 1, G: 1, B: 1, A: 1})

			ctx.SetRGBA(0.2, 0.4, 0.8, 1.0)
			ctx.DrawCircle(400, 300, 200)
			ctx.Fill()

			if face14 != nil {
				ctx.SetFont(face14)
				ctx.SetRGBA(0, 0, 0, 1)
				ctx.DrawString("Hello GPUI!", 300, 350)
				ctx.DrawString("你好 GPUI!", 300, 380)
				ctx.DrawString("こんにちは GPUI!", 300, 410)
				ctx.DrawString("안녕하세요 GPUI!", 300, 440)
			}
		})
	})

	app.AddWindow(win)
	app.Run()
}
