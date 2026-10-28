//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !nogpu

package main

import rendergpu "github.com/energye/gpui/render/gpu"

// resetGPUAccelerator drops the probe's standalone accelerator device before
// the window opens (see the call site in main.go).
func resetGPUAccelerator() error {
	return rendergpu.ResetAccelerator()
}
