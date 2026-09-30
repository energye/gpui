//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build darwin && !(js && wasm)

package metal

import "github.com/energye/gpui/gpu/hal"

// init registers the Metal backend with the HAL registry.
// This is called automatically on package import.
//
// The Metal backend is only available on macOS and iOS.
func init() {
	hal.RegisterBackend(Backend{})
}
