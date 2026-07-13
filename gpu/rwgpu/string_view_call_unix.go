//go:build linux || darwin

package rwgpu

import (
	"unsafe"

	"github.com/energye/gpui/ffi"
	ffitypes "github.com/energye/gpui/ffi/types"
)

var stringViewValueType = &ffitypes.TypeDescriptor{
	Kind:      ffitypes.StructType,
	Size:      16,
	Alignment: 8,
	Members: []*ffitypes.TypeDescriptor{
		ffitypes.PointerTypeDescriptor, // data
		ffitypes.UInt64TypeDescriptor,  // length
	},
}

func callHandleStringView(proc Proc, handle uintptr, label *StringView) {
	unix, ok := proc.(*unixProc)
	if !ok || unix.fnPtr == nil {
		return
	}

	var cif ffitypes.CallInterface
	if err := ffi.PrepareCallInterface(
		&cif,
		ffitypes.UnixCallingConvention,
		ffitypes.VoidTypeDescriptor,
		[]*ffitypes.TypeDescriptor{
			ffitypes.PointerTypeDescriptor,
			stringViewValueType,
		},
	); err != nil {
		return
	}

	args := []unsafe.Pointer{
		unsafe.Pointer(&handle),
		unsafe.Pointer(label),
	}
	_, _ = ffi.CallFunction(&cif, unix.fnPtr, nil, args)
}
