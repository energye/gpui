//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build nogpu

package main

// resetGPUAccelerator is a no-op in nogpu builds: the probe runs CPU-only, so
// there is no accelerator device to drop before the window opens.
func resetGPUAccelerator() error {
	return nil
}
