//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !(js && wasm)

package noop

import "github.com/energye/gpui/gpu/hal"

// init registers the noop backend with the HAL registry.
func init() {
	hal.RegisterBackend(Backend{})
}
