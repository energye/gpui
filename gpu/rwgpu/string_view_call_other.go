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

func callHandleStringView(proc Proc, handle uintptr, label *StringView) {
	proc.Call( //nolint:errcheck
		handle,
		uintptr(unsafe.Pointer(label)),
	)
}
