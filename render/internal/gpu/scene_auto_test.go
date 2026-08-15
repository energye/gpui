// Copyright 2026 The gogpu Authors
// SPDX-License-Identifier: MIT

//go:build !nogpu

// Tests for the automatic CPU/GPU scene render entry (scene_auto.go).
// The rendered image must match the tilecompute CPU reference pixel-for-pixel
// regardless of which backend is selected, and the result must report the
// backend that actually rendered it.

package gpu

import (
	"testing"

	"github.com/energye/gpui/render/internal/gpu/tilecompute"
)

// TestRenderSceneAuto_MatchesCPUReference renders a clip scene through
// RenderSceneAuto and asserts the output equals the CPU reference exactly.
// On a machine with an available GPU compute pipeline the GPU backend is
// exercised; otherwise the CPU fallback is exercised — both must agree.
func TestRenderSceneAuto_MatchesCPUReference(t *testing.T) {
	const size = 64
	bg := [4]uint8{255, 255, 255, 255}

	elements := []tilecompute.SceneElement{
		// Green background.
		{
			Type:     tilecompute.ElementDraw,
			Lines:    computeSquareLines(0, 0, size, size),
			Color:    [4]uint8{60, 180, 60, 255},
			FillRule: tilecompute.FillRuleNonZero,
		},
		// Clip: center rectangle.
		{
			Type:      tilecompute.ElementBeginClip,
			Lines:     computeSquareLines(16, 16, 48, 48),
			BlendMode: 0x8003,
			Alpha:     1.0,
		},
		// Red full-canvas rect (visually clipped to the center).
		{
			Type:     tilecompute.ElementDraw,
			Lines:    computeSquareLines(0, 0, size, size),
			Color:    [4]uint8{220, 40, 40, 220},
			FillRule: tilecompute.FillRuleNonZero,
		},
		{Type: tilecompute.ElementEndClip},
		// Even-odd star after EndClip (centers must stay hollow).
		{
			Type:     tilecompute.ElementDraw,
			Lines:    computeStarLines(),
			Color:    [4]uint8{230, 200, 0, 255},
			FillRule: tilecompute.FillRuleEvenOdd,
		},
	}

	res := RenderSceneAuto(size, size, bg, elements)
	if res.Img == nil {
		t.Fatal("RenderSceneAuto returned nil image")
	}

	// The rendered image must equal the CPU reference exactly.
	rast := tilecompute.NewRasterizer(size, size)
	cpuImg := rast.RasterizeSceneDefPTCL(bg, elements)
	diffPercent, diffCount := compareImages(res.Img, cpuImg)
	t.Logf("RenderSceneAuto: backend=%s diff=%d px (%.2f%%)",
		res.Backend, diffCount, diffPercent)
	if diffPercent > 0 {
		t.Errorf("RenderSceneAuto output differs from CPU reference: %.2f%% (%d px)",
			diffPercent, diffCount)
	}

	// Backend must be one of the two known values.
	switch res.Backend {
	case SceneRenderBackendGPU, SceneRenderBackendCPU:
	default:
		t.Errorf("unexpected backend %q", res.Backend)
	}

	// When the GPU backend was selected, GPUError must be nil; a CPU fallback
	// should report why the GPU path was not used.
	if res.Backend == SceneRenderBackendGPU && res.GPUError != nil {
		t.Errorf("GPU backend selected but GPUError = %v", res.GPUError)
	}
}

// TestRenderSceneAuto_EmptyElements asserts the empty-scene case produces the
// plain background and never fails.
func TestRenderSceneAuto_EmptyElements(t *testing.T) {
	bg := [4]uint8{10, 20, 30, 255}
	res := RenderSceneAuto(16, 16, bg, nil)
	if res.Img == nil {
		t.Fatal("RenderSceneAuto returned nil image for empty scene")
	}
	c := res.Img.RGBAAt(8, 8)
	if int(c.R) != 10 || int(c.G) != 20 || int(c.B) != 30 {
		t.Errorf("empty scene pixel = (%d,%d,%d), want background (10,20,30)",
			c.R, c.G, c.B)
	}
}

// TestRenderSceneAuto_ReportsBackend verifies the backend report reflects the
// actual render path: GPU when the compute pipeline is available, otherwise
// the CPU reference with a non-nil GPUError.
func TestRenderSceneAuto_ReportsBackend(t *testing.T) {
	bg := [4]uint8{255, 255, 255, 255}
	elements := []tilecompute.SceneElement{
		{
			Type:     tilecompute.ElementDraw,
			Lines:    computeSquareLines(2, 2, 14, 14),
			Color:    [4]uint8{0, 0, 255, 255},
			FillRule: tilecompute.FillRuleNonZero,
		},
	}

	res := RenderSceneAuto(16, 16, bg, elements)
	if res.Img == nil {
		t.Fatal("RenderSceneAuto returned nil image")
	}
	switch res.Backend {
	case SceneRenderBackendGPU:
		if res.GPUError != nil {
			t.Errorf("GPU backend with GPUError: %v", res.GPUError)
		}
	case SceneRenderBackendCPU:
		if res.GPUError == nil {
			t.Error("CPU fallback must report the GPU failure reason (GPUError)")
		}
		t.Logf("CPU fallback reason: %v", res.GPUError)
	default:
		t.Errorf("unexpected backend %q", res.Backend)
	}
}