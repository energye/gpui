package gles

import "unsafe"

// u2p converts a raw address or byte offset to unsafe.Pointer
// (GL PBO-offset idiom and FFI boundary only).
func u2p(u uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&u))
}
