//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build linux && !(js && wasm)

package gles

import "github.com/energye/gpui/gpu/hal"

// init registers the OpenGL ES backend with the HAL registry.
func init() {
	hal.RegisterBackend(Backend{})
}
