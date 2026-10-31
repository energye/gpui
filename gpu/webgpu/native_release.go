//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package webgpu

import (
	"github.com/energye/gpui/gpu/hal"
	rwgpu "github.com/energye/gpui/gpu/rwgpu"
)

// The skip-on-lost release semantics are implemented in the rwgpu binding
// layer; wiring them into hal here lets render-side recover paths flip the
// switch through hal instead of importing a concrete backend (P2.5 D2).
func init() {
	hal.SetForceNativeReleaseOnLostHook(rwgpu.WithForceNativeReleaseOnLost)
}
