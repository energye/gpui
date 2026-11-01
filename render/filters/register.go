//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// Package filters registers CPU image-filter implementations on render.Context.
//
// Import for side effects:
//
//	import _ "github.com/energye/gpui/render/filters"
package filters

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/internal/filter"
	"github.com/energye/gpui/render/scene"
)

func init() {
	render.RegisterFilterOps(
		func(src, dst *render.Pixmap, radius float64) {
			filter.NewBlurFilter(radius).Apply(src, dst, full(src))
		},
		func(src, dst *render.Pixmap, radiusX, radiusY float64) {
			filter.NewBlurFilterXY(radiusX, radiusY).Apply(src, dst, full(src))
		},
		func(src, dst *render.Pixmap, offsetX, offsetY, blur float64, color render.RGBA) {
			filter.NewDropShadowFilter(offsetX, offsetY, blur, color).Apply(src, dst, full(src))
		},
		func(src, dst *render.Pixmap, matrix [20]float32) {
			filter.NewColorMatrixFilter(matrix).Apply(src, dst, full(src))
		},
		func(src, dst *render.Pixmap) {
			filter.NewGrayscaleFilter().Apply(src, dst, full(src))
		},
		func(src, dst *render.Pixmap) {
			filter.NewInvertFilter().Apply(src, dst, full(src))
		},
		func(src, dst *render.Pixmap, radius int) {
			filter.NewErodeFilter(radius, radius).Apply(src, dst, full(src))
		},
		func(src, dst *render.Pixmap, radius int) {
			filter.NewDilateFilter(radius, radius).Apply(src, dst, full(src))
		},
		func(src, dst *render.Pixmap, dx, dy int) {
			filter.NewOffsetFilter(dx, dy).Apply(src, dst, full(src))
		},
		func(src, dst *render.Pixmap, threshold uint8) {
			filter.NewAlphaThresholdFilter(threshold).Apply(src, dst, full(src))
		},
		func(src, dst *render.Pixmap, lightPos [3]float32, color render.RGBA, kd, surfaceScale float32) {
			filter.NewPointDiffuseFilter(lightPos, color, kd, surfaceScale).Apply(src, dst, full(src))
		},
		func(src, dst *render.Pixmap, srcRect [4]float32) {
			filter.NewTileFilter(srcRect).Apply(src, dst, full(src))
		},
		func(src, dst *render.Pixmap, xSel, ySel, scale int, disp *render.ImageBuf) {
			filter.NewDisplacementFilter(xSel, ySel, scale, disp).Apply(src, dst, full(src))
		},
	)
}

func full(pm *render.Pixmap) scene.Rect {
	if pm == nil {
		return scene.Rect{}
	}
	return scene.Rect{
		MinX: 0,
		MinY: 0,
		MaxX: float32(pm.Width()),
		MaxY: float32(pm.Height()),
	}
}
