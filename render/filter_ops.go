//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import (
	gpucontext "github.com/energye/gpui/gpu/context"
)

// filterApplyFunc applies a filter from src into dst over the full pixmap.
type filterApplyFunc func(src, dst *Pixmap)

var (
	blurApply        func(src, dst *Pixmap, radius float64)
	blurXYApply      func(src, dst *Pixmap, radiusX, radiusY float64)
	dropShadowApply  func(src, dst *Pixmap, offsetX, offsetY, blur float64, color RGBA)
	colorMatrixApply func(src, dst *Pixmap, matrix [20]float32)
	grayscaleApply   func(src, dst *Pixmap)
	invertApply      func(src, dst *Pixmap)
	// gpuFilterGraphApply runs F.03 multi-RT GPU ping-pong (optional).
	// src is tight RGBA8 w*h*4; returns same layout result.
	gpuFilterGraphApply func(src []byte, w, h int, nodes []ImageFilterNode) ([]byte, error)
	// gpuFilterGraphApplyTexture publishes a GPU texture without CPU Map/readback.
	// Preferred for continuous effect RTs (glow/blur present path).
	gpuFilterGraphApplyTexture func(src []byte, w, h int, nodes []ImageFilterNode) (gpucontext.TextureView, func(), error)
	// gpuFilterGraphApplyFromView seeds from a GPU texture (no CPU upload/readback).
	gpuFilterGraphApplyFromView func(srcView gpucontext.TextureView, w, h int, nodes []ImageFilterNode) (gpucontext.TextureView, func(), error)

	filterPixmapPool = newPixmapPool(8)
)
