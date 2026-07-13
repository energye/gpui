//go:build linux || darwin

package rwgpu

import (
	"unsafe"

	"github.com/energye/gpui/ffi"
	ffitypes "github.com/energye/gpui/ffi/types"
)

func callRenderPassEncoderSetViewport(handle uintptr, x, y, width, height, minDepth, maxDepth float32) {
	proc, ok := procRenderPassEncoderSetViewport.(*unixProc)
	if !ok || proc.fnPtr == nil {
		return
	}

	var cif ffitypes.CallInterface
	if err := ffi.PrepareCallInterface(
		&cif,
		ffitypes.UnixCallingConvention,
		ffitypes.VoidTypeDescriptor,
		[]*ffitypes.TypeDescriptor{
			ffitypes.PointerTypeDescriptor,
			ffitypes.FloatTypeDescriptor,
			ffitypes.FloatTypeDescriptor,
			ffitypes.FloatTypeDescriptor,
			ffitypes.FloatTypeDescriptor,
			ffitypes.FloatTypeDescriptor,
			ffitypes.FloatTypeDescriptor,
		},
	); err != nil {
		return
	}

	args := []unsafe.Pointer{
		unsafe.Pointer(&handle),
		unsafe.Pointer(&x),
		unsafe.Pointer(&y),
		unsafe.Pointer(&width),
		unsafe.Pointer(&height),
		unsafe.Pointer(&minDepth),
		unsafe.Pointer(&maxDepth),
	}
	_, _ = ffi.CallFunction(&cif, proc.fnPtr, nil, args)
}
