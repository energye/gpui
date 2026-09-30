//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package renderconv

import (
	"github.com/energye/gpui/engine/core"
	"github.com/energye/gpui/render"
)

// Vec2ToRenderPoint converts to render.Point at the render boundary.
func Vec2ToRenderPoint(v core.Vec2) render.Point { return render.Point{X: v.X, Y: v.Y} }

// Vec2FromRenderPoint converts a render.Point back to game units.
func Vec2FromRenderPoint(p render.Point) core.Vec2 { return core.Vec2{X: p.X, Y: p.Y} }

// ColorToRender converts to render.RGBA at the render boundary.
func ColorToRender(c core.Color) render.RGBA {
	return render.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}
}

// ColorFromRender converts a render.RGBA back to game units.
func ColorFromRender(rc render.RGBA) core.Color {
	return core.Color{R: rc.R, G: rc.G, B: rc.B, A: rc.A}
}

// Mat2DToRenderMatrix converts to render.Matrix at the render boundary.
func Mat2DToRenderMatrix(m core.Mat2D) render.Matrix {
	return render.Matrix{A: m.A, B: m.B, C: m.C, D: m.D, E: m.E, F: m.F}
}

// Mat2DFromRenderMatrix converts a render.Matrix back to game units.
func Mat2DFromRenderMatrix(m render.Matrix) core.Mat2D {
	return core.Mat2D{A: m.A, B: m.B, C: m.C, D: m.D, E: m.E, F: m.F}
}
