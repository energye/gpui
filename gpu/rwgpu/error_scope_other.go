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
