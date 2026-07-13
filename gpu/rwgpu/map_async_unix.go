//go:build linux || darwin

package rwgpu

import (
	"unsafe"

	"github.com/energye/gpui/ffi"
	ffitypes "github.com/energye/gpui/ffi/types"
)

var bufferMapCallbackInfoType = &ffitypes.TypeDescriptor{
	Kind:      ffitypes.StructType,
	Size:      40,
	Alignment: 8,
	Members: []*ffitypes.TypeDescriptor{
		ffitypes.PointerTypeDescriptor, // nextInChain
		ffitypes.UInt32TypeDescriptor,  // mode
		ffitypes.PointerTypeDescriptor, // callback
		ffitypes.PointerTypeDescriptor, // userdata1
		ffitypes.PointerTypeDescriptor, // userdata2
	},
}

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
	if proc.fnPtr == nil {
		return Future{}, &WGPUError{Op: "Buffer.MapAsync", Message: "wgpuBufferMapAsync symbol is missing"}
	}

	var cif ffitypes.CallInterface
	if err := ffi.PrepareCallInterface(
		&cif,
		ffitypes.UnixCallingConvention,
		ffitypes.UInt64TypeDescriptor,
		[]*ffitypes.TypeDescriptor{
			ffitypes.PointerTypeDescriptor,
			ffitypes.UInt64TypeDescriptor,
			ffitypes.UInt64TypeDescriptor,
			ffitypes.UInt64TypeDescriptor,
			bufferMapCallbackInfoType,
		},
	); err != nil {
		return Future{}, &WGPUError{Op: "Buffer.MapAsync", Message: err.Error()}
	}

	modeArg := uint64(mode)
	var future uint64
	args := []unsafe.Pointer{
		unsafe.Pointer(&buffer),
		unsafe.Pointer(&modeArg),
		unsafe.Pointer(&offset),
		unsafe.Pointer(&size),
		unsafe.Pointer(callbackInfo),
	}
	if _, err := ffi.CallFunction(&cif, proc.fnPtr, unsafe.Pointer(&future), args); err != nil {
		return Future{}, &WGPUError{Op: "Buffer.MapAsync", Message: err.Error()}
	}
	return Future{ID: future}, nil
}
