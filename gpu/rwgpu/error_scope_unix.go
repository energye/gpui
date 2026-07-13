//go:build linux || darwin

package rwgpu

import (
	"unsafe"

	"github.com/energye/gpui/ffi"
	ffitypes "github.com/energye/gpui/ffi/types"
)

var popErrorScopeCallbackInfoType = &ffitypes.TypeDescriptor{
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

func callDevicePopErrorScope(device uintptr, callbackInfo *popErrorScopeCallbackInfo) (Future, error) {
	proc, ok := procDevicePopErrorScope.(*unixProc)
	if !ok {
		future, _, err := procDevicePopErrorScope.Call(
			device,
			uintptr(unsafe.Pointer(callbackInfo)),
		)
		return Future{ID: uint64(future)}, err
	}
	if proc.fnPtr == nil {
		return Future{}, &WGPUError{Op: "PopErrorScopeAsync", Message: "wgpuDevicePopErrorScope symbol is missing"}
	}

	var cif ffitypes.CallInterface
	if err := ffi.PrepareCallInterface(
		&cif,
		ffitypes.UnixCallingConvention,
		ffitypes.UInt64TypeDescriptor,
		[]*ffitypes.TypeDescriptor{
			ffitypes.PointerTypeDescriptor,
			popErrorScopeCallbackInfoType,
		},
	); err != nil {
		return Future{}, &WGPUError{Op: "PopErrorScopeAsync", Message: err.Error()}
	}

	var future uint64
	args := []unsafe.Pointer{
		unsafe.Pointer(&device),
		unsafe.Pointer(callbackInfo),
	}
	if _, err := ffi.CallFunction(&cif, proc.fnPtr, unsafe.Pointer(&future), args); err != nil {
		return Future{}, &WGPUError{Op: "PopErrorScopeAsync", Message: err.Error()}
	}
	return Future{ID: future}, nil
}
