//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !linux && !darwin

package rwgpu

import "unsafe"

func callDevicePopErrorScope(device uintptr, callbackInfo *popErrorScopeCallbackInfo) (Future, error) {
	future, _, err := procDevicePopErrorScope.Call(
		device,
		uintptr(unsafe.Pointer(callbackInfo)),
	)
	return Future{ID: uint64(future)}, err
}
