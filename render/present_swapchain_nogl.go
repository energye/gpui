//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !(windows || linux) || (js && wasm)

package render

import (
	"fmt"

	"github.com/energye/gpui/gpu/hal"
)

func newGLHalSwapchain(_ hal.Surface, _ hal.Device, _, _ uint32) (hal.Swapchain, error) {
	return nil, fmt.Errorf("render: P1 GL swapchain unsupported on this platform")
}
