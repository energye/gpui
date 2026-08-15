// Copyright 2026 The gogpu Authors
// SPDX-License-Identifier: MIT

//go:build !nogpu

// scene_auto.go provides an automatic CPU/GPU rendering entry for clip-aware
// scenes. Callers describe the scene once as []SceneElement and the backend is
// selected automatically: the Vello GPU compute pipeline when available,
// the tilecompute CPU reference otherwise.

package gpu

import (
	"fmt"
	"image"
	"time"

	"github.com/energye/gpui/render/internal/gpu/tilecompute"
)

// SceneRenderBackend identifies which implementation actually rendered a scene.
type SceneRenderBackend string

const (
	// SceneRenderBackendGPU means the Vello compute pipeline produced the image.
	SceneRenderBackendGPU SceneRenderBackend = "gpu-compute"
	// SceneRenderBackendCPU means the tilecompute CPU reference produced the image.
	SceneRenderBackendCPU SceneRenderBackend = "cpu-reference"
)

// SceneRenderResult is the outcome of an automatic scene render.
type SceneRenderResult struct {
	// Img is the rendered RGBA image (straight alpha), never nil on success.
	Img *image.RGBA

	// Backend reports which implementation rendered the image.
	Backend SceneRenderBackend

	// Duration is the actual rendering time of the selected backend.
	Duration time.Duration

	// GPUError is non-nil only when the GPU backend was attempted and failed
	// (in which case the CPU reference was used as the fallback).
	GPUError error
}

// RenderSceneAuto renders a clip-aware scene (SceneElements with optional
// BeginClip/EndClip layers) through the best available backend.
//
// It prefers the Vello GPU compute pipeline (RenderSceneComputeDef) and
// automatically falls back to the CPU reference
// (tilecompute.Rasterizer.RasterizeSceneDefPTCL) when the GPU device or
// compute pipeline is unavailable. Both backends run the same Vello algorithm
// and produce pixel-identical output, so callers do not need to branch on the
// backend — SceneRenderResult.Backend only reports which one was used.
//
// The GPU accelerator instance is created and destroyed inside this call;
// it does not interfere with any externally held GPU device.
func RenderSceneAuto(
	width, height int,
	bgColor [4]uint8,
	elements []tilecompute.SceneElement,
) SceneRenderResult {
	// CPU fallback: render once through the tilecompute reference and time it.
	cpuRender := func() SceneRenderResult {
		rast := tilecompute.NewRasterizer(width, height)
		start := time.Now()
		img := rast.RasterizeSceneDefPTCL(bgColor, elements)
		return SceneRenderResult{
			Img:      img,
			Backend:  SceneRenderBackendCPU,
			Duration: time.Since(start),
		}
	}

	// GPU first: standalone Vello compute pipeline.
	accel := &VelloAccelerator{}
	if err := accel.InitStandalone(); err != nil {
		res := cpuRender()
		res.GPUError = err
		return res
	}
	defer accel.Close()

	if !accel.CanCompute() {
		res := cpuRender()
		res.GPUError = fmt.Errorf("compute pipeline not available")
		return res
	}

	start := time.Now()
	img, rErr := accel.RenderSceneComputeDef(width, height, bgColor, elements)
	dur := time.Since(start)
	if rErr == nil && img != nil {
		return SceneRenderResult{
			Img:      img,
			Backend:  SceneRenderBackendGPU,
			Duration: dur,
		}
	}

	res := cpuRender()
	res.GPUError = rErr
	return res
}