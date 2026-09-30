//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package gles

import "unsafe"

// u2p converts a raw address or byte offset to unsafe.Pointer
// (GL PBO-offset idiom and FFI boundary only).
func u2p(u uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&u))
}
