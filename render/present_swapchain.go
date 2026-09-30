// Copyright 2026 The gogpu Authors
// SPDX-License-Identifier: MIT

package render

import (
	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/webgpu"
)

// newWebgpuHalSwapchain builds the default hal.Swapchain (Rust path).
func newWebgpuHalSwapchain(surf hal.Surface, dev hal.Device, w, h uint32) hal.Swapchain {
	return webgpu.NewSwapchain(surf, dev, w, h).NewHalSwapchain()
}

// newPresentSwapchain picks the backend once (creation sentence) and
// returns its hal.Swapchain; render talks hal afterwards. Callers own
// cleanup on error (their Surface/Device lifetimes differ).
func newPresentSwapchain(surf hal.Surface, dev hal.Device, w, h uint32) (hal.Swapchain, error) {
	want, err := ResolveBackend()
	if err != nil {
		return nil, err
	}
	if want == BackendGo {
		return newGLHalSwapchain(surf, dev, w, h)
	}
	return newWebgpuHalSwapchain(surf, dev, w, h), nil
}
