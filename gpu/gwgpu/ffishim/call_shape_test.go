//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package ffishim

import (
	"os"
	"testing"
	"unsafe"
)

// read(2) via libc validates the 3-arg (int, pointer, word) SyscallN
// shape eglQueryDevicesEXT needs: EGLint max_devices,
// EGLDeviceEXT *devices, EGLint *num_devices, EGLBoolean return.
func TestCallFunctionThreeArgIntPtrPtr(t *testing.T) {
	lib, err := LoadLibrary("libc.so.6")
	if err != nil {
		t.Skipf("libc not available: %v", err)
	}
	sym, err := GetSymbol(lib, "read")
	if err != nil {
		t.Fatalf("read missing: %v", err)
	}
	var cif CallInterface
	if err := PrepareCallInterface(&cif, DefaultCall,
		SInt64TypeDescriptor,
		[]*TypeDescriptor{IntTypeDescriptor, PointerTypeDescriptor, UInt64TypeDescriptor}); err != nil {
		t.Fatal(err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	msg := []byte("p21-shape")
	if _, err := w.Write(msg); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 32)
	var n int64
	fd := int32(r.Fd())
	count := uint64(len(buf))
	bufPtr := unsafe.Pointer(&buf[0])
	if _, err := CallFunction(&cif, sym, unsafe.Pointer(&n),
		[]unsafe.Pointer{unsafe.Pointer(&fd), unsafe.Pointer(&bufPtr), unsafe.Pointer(&count)}); err != nil {
		t.Fatal(err)
	}
	if n != int64(len(msg)) || string(buf[:n]) != string(msg) {
		t.Fatalf("read = (%d, %q), want (%d, %q)", n, buf[:n], len(msg), msg)
	}
}
