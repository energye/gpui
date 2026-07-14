//go:build linux || darwin

package rwgpu

import (
	"unsafe"

	"github.com/ebitengine/purego"
)

func callDevicePopErrorScope(device uintptr, callbackInfo *popErrorScopeCallbackInfo) (Future, error) {
	proc, ok := procDevicePopErrorScope.(*unixProc)
	if !ok {
		future, _, err := procDevicePopErrorScope.Call(
			device,
			uintptr(unsafe.Pointer(callbackInfo)),
		)
		return Future{ID: uint64(future)}, err
	}
	if proc.fnPtr == 0 {
		return Future{}, &WGPUError{Op: "PopErrorScopeAsync", Message: "wgpuDevicePopErrorScope symbol is missing"}
	}

	var popErrorScope func(uintptr, popErrorScopeCallbackInfo) Future
	purego.RegisterFunc(&popErrorScope, proc.fnPtr)
	return popErrorScope(device, *callbackInfo), nil
}
