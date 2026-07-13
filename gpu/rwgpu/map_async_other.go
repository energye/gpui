//go:build !linux && !darwin

package rwgpu

import "unsafe"

func callBufferMapAsync(buffer uintptr, mode MapMode, offset, size uint64, callbackInfo *BufferMapCallbackInfo) (Future, error) {
	future, _, err := procBufferMapAsync.Call(
		buffer,
		uintptr(mode),
		uintptr(offset),
		uintptr(size),
		uintptr(unsafe.Pointer(callbackInfo)),
	)
	return Future{ID: uint64(future)}, err
}
