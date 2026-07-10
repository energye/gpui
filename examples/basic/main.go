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

	// Main window: blue circle
	mainWin := gpui.NewWindow(gpui.WindowConfig{
		Title:  "GPUI - Main Window",
		Width:  800,
		Height: 600,
	})
	mainWin.OnInit(func(ctrl *gpui.TGPUControl) {
		ctrl.SetOnRender(func(ctx *gg.Context) {
			ctx.ClearWithColor(gg.RGBA{R: 0.95, G: 0.95, B: 0.98, A: 1})
			ctx.SetRGBA(0.2, 0.4, 0.8, 1.0)
			ctx.DrawCircle(400, 300, 200)
			ctx.Fill()
			ctx.SetRGBA(0, 0, 0, 1)
			ctx.DrawString("Main Window", 350, 320)
		})
	})

	// Second window: red rectangle
	secondWin := gpui.NewWindow(gpui.WindowConfig{
		Title:  "GPUI - Second Window",
		Width:  400,
		Height: 300,
	})
	secondWin.OnInit(func(ctrl *gpui.TGPUControl) {
		ctrl.SetOnRender(func(ctx *gg.Context) {
			ctx.ClearWithColor(gg.RGBA{R: 1, G: 1, B: 0.95, A: 1})
			ctx.SetRGBA(0.8, 0.2, 0.2, 1.0)
			ctx.DrawRectangle(50, 50, 300, 200)
			ctx.Fill()
			ctx.SetRGBA(0, 0, 0, 1)
			ctx.DrawString("Second Window", 120, 160)
		})
	})

	// Third window: green triangle
	thirdWin := gpui.NewWindow(gpui.WindowConfig{
		Title:  "GPUI - Third Window",
		Width:  400,
		Height: 300,
	})
	thirdWin.OnInit(func(ctrl *gpui.TGPUControl) {
		ctrl.SetOnRender(func(ctx *gg.Context) {
			ctx.ClearWithColor(gg.RGBA{R: 0.95, G: 1, B: 0.95, A: 1})
			ctx.SetRGBA(0.2, 0.8, 0.2, 1.0)
			ctx.MoveTo(200, 40)
			ctx.LineTo(350, 260)
			ctx.LineTo(50, 260)
			ctx.ClosePath()
			ctx.Fill()
			ctx.SetRGBA(0, 0, 0, 1)
			ctx.DrawString("Third Window", 150, 160)
		})
	})

	app.AddWindow(mainWin)
	app.AddWindow(secondWin)
	app.AddWindow(thirdWin)
	app.Run()
}