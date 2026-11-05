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
	blurApply           func(src, dst *Pixmap, radius float64)
	blurXYApply         func(src, dst *Pixmap, radiusX, radiusY float64)
	dropShadowApply     func(src, dst *Pixmap, offsetX, offsetY, blur float64, color RGBA)
	colorMatrixApply    func(src, dst *Pixmap, matrix [20]float32)
	grayscaleApply      func(src, dst *Pixmap)
	invertApply         func(src, dst *Pixmap)
	erodeApply          func(src, dst *Pixmap, radius int)
	dilateApply         func(src, dst *Pixmap, radius int)
	offsetApply         func(src, dst *Pixmap, dx, dy int)
	alphaThresholdApply func(src, dst *Pixmap, threshold uint8)
	lightingApply       func(src, dst *Pixmap, lightPos [3]float32, color RGBA, kd, surfaceScale float32)
	tileApply           func(src, dst *Pixmap, srcRect [4]float32)
	displacementApply   func(src, dst *Pixmap, xSel, ySel, scale int, disp *ImageBuf)
	bloomApply          func(src, dst *Pixmap, p BloomParams)
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

// FilterOpImpls is the CPU-side implementation set for render's image filters.
// Named fields since P3.7 batch 5: the registrar used to take thirteen
// positional closures whose signatures differed only by one scalar, so
// swapping any two of them still compiled.
type FilterOpImpls struct {
	Blur           func(src, dst *Pixmap, radius float64)
	BlurXY         func(src, dst *Pixmap, radiusX, radiusY float64)
	DropShadow     func(src, dst *Pixmap, offsetX, offsetY, blur float64, color RGBA)
	ColorMatrix    func(src, dst *Pixmap, matrix [20]float32)
	Grayscale      func(src, dst *Pixmap)
	Invert         func(src, dst *Pixmap)
	Erode          func(src, dst *Pixmap, radius int)
	Dilate         func(src, dst *Pixmap, radius int)
	Offset         func(src, dst *Pixmap, dx, dy int)
	AlphaThreshold func(src, dst *Pixmap, threshold uint8)
	Lighting       func(src, dst *Pixmap, lightPos [3]float32, color RGBA, kd, surfaceScale float32)
	Tile           func(src, dst *Pixmap, srcRect [4]float32)
	Displacement   func(src, dst *Pixmap, xSel, ySel, scale int, disp *ImageBuf)
	Bloom          func(src, dst *Pixmap, p BloomParams)
}

// snapshotFilterOps reads back the current registration (save/restore seam
// for in-process re-registration tests).
func snapshotFilterOps() FilterOpImpls {
	return FilterOpImpls{
		Blur: blurApply, BlurXY: blurXYApply, DropShadow: dropShadowApply,
		ColorMatrix: colorMatrixApply, Grayscale: grayscaleApply, Invert: invertApply,
		Erode: erodeApply, Dilate: dilateApply, Offset: offsetApply,
		AlphaThreshold: alphaThresholdApply, Lighting: lightingApply, Tile: tileApply,
		Displacement: displacementApply, Bloom: bloomApply,
	}
}

// GPUFilterGraphHooks exposes the registered GPU filter-graph entry points
// (the same hooks ApplyImageFilterGraph uses) for internal filtered-paint
// paths (P3.4 paint-level filters). Nil members mean the graph is not
// registered.
type GPUFilterGraphHooks struct {
	Apply func(src []byte, w, h int, nodes []ImageFilterNode) ([]byte, error)
}

// RegisteredGPUFilterGraphHooks returns the current hook set.
func RegisteredGPUFilterGraphHooks() GPUFilterGraphHooks {
	return GPUFilterGraphHooks{Apply: gpuFilterGraphApply}
}
