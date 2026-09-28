// Copyright 2026 The gogpu Authors
// SPDX-License-Identifier: MIT

//go:build !(windows || linux) || (js && wasm)

package render

import (
	"fmt"

	"github.com/energye/gpui/gpu/hal"
)

func newGLHalSwapchain(_ hal.Surface, _ hal.Device, _, _ uint32) (hal.Swapchain, error) {
	return nil, fmt.Errorf("render: P1 GL swapchain unsupported on this platform")
}
