//go:build linux || darwin

package rwgpu

import (
	"unsafe"

	"github.com/ebitengine/purego"
)

func callBufferMapAsync(buffer uintptr, mode MapMode, offset, size uint64, callbackInfo *BufferMapCallbackInfo) (Future, error) {
	proc, ok := procBufferMapAsync.(*unixProc)
	if !ok {
		future, _, err := procBufferMapAsync.Call(
			buffer,
			uintptr(mode),
			uintptr(offset),
			uintptr(size),
			uintptr(unsafe.Pointer(callbackInfo)),
		)
		return Future{ID: uint64(future)}, err
	}
	if proc.fnPtr == 0 {
		return Future{}, &WGPUError{Op: "Buffer.MapAsync", Message: "wgpuBufferMapAsync symbol is missing"}
	}

	var mapAsync func(uintptr, MapMode, uint64, uint64, BufferMapCallbackInfo) Future
	purego.RegisterFunc(&mapAsync, proc.fnPtr)
	return mapAsync(buffer, mode, offset, size, *callbackInfo), nil
}
