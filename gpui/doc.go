// Package gpui provides GPU-accelerated 2D vector graphics rendering for LCL
// applications.
//
// Architecture:
//
//	GUI Layer:  LCL OpenGL Control (TCustomOpenGLControl) — window management + display
//	Render:     gg Context (CPU/GPU hybrid) — 2D vector graphics, text, paths
//	GPU:        wgpu (headless compute) — optional SDF + Coverage acceleration
//	Display:    OpenGL texture upload + Quad → SwapBuffers
//
// Usage (CPU mode — no GPU acceleration):
//
//	import (
//	    "github.com/energye/gpui/gpui"
//	    "github.com/energye/lcl/lcl"
//	    "github.com/energye/lcl/lcl/application"
//	)
//
//	func main() {
//	    app := application.New()
//	    form := lcl.NewForm(nil)
//	    form.SetWidth(800)
//	    form.SetHeight(600)
//
//	    ctrl := gpui.NewGPUControl(form)
//	    ctrl.SetOnRender(func(ctx *gg.Context) {
//	        ctx.DrawCircle(400, 300, 100)
//	        ctx.Fill()
//	    })
//
//	    application.Run()
//	}
//
// Usage (GPU accelerated mode):
//
//	import _ "github.com/energye/gpui/gpu" // enables SDF + Coverage GPU compute
//
// The GPU accelerator is imported as a blank import. It registers itself in
// init() and transparently accelerates SDF shapes (circles, rounded rects)
// and path coverage filling via wgpu compute shaders.
package gpui