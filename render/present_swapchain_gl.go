//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build (windows || linux) && !(js && wasm)

package render

import (
	"fmt"

	"github.com/energye/gpui/gpu/gwgpu/gles"
	"github.com/energye/gpui/gpu/hal"
)

// newGLHalSwapchain builds the pure-Go GL hal.Swapchain.
// Creation is the only place that names the backend; afterwards render
// talks hal.Swapchain only.
func newGLHalSwapchain(surf hal.Surface, dev hal.Device, w, h uint32) (hal.Swapchain, error) {
	if surf == nil || dev == nil {
		return nil, fmt.Errorf("render: GL swapchain needs surface+device")
	}
	return gles.NewSwapchain(surf, dev, w, h).NewHalSwapchain(), nil
}
