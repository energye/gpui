//go:build !linux && !darwin

package rwgpu

import "unsafe"

func callHandleStringView(proc Proc, handle uintptr, label *StringView) {
	proc.Call( //nolint:errcheck
		handle,
		uintptr(unsafe.Pointer(label)),
	)
}
