package ffishim

import (
	"testing"
	"unsafe"
)

// strlen via libc validates the SyscallN path end to end:
// pointer argument in, integer return out.
func TestCallFunctionStrlen(t *testing.T) {
	lib, err := LoadLibrary("libc.so.6")
	if err != nil {
		t.Skipf("libc not available: %v", err)
	}
	sym, err := GetSymbol(lib, "strlen")
	if err != nil {
		t.Fatalf("strlen missing: %v", err)
	}
	var cif CallInterface
	if err := PrepareCallInterface(&cif, DefaultCall,
		UInt64TypeDescriptor,
		[]*TypeDescriptor{PointerTypeDescriptor}); err != nil {
		t.Fatal(err)
	}
	s := append([]byte("hello"), 0)
	var n uintptr
	arg := unsafe.Pointer(&s[0])
	if _, err := CallFunction(&cif, sym, unsafe.Pointer(&n),
		[]unsafe.Pointer{unsafe.Pointer(&arg)}); err != nil {
		t.Fatal(err)
	}
	if n != 5 {
		t.Fatalf("strlen = %d, want 5", n)
	}
}

// Unknown float signatures must error, not crash.
func TestCallFunctionFloatReturnUnsupported(t *testing.T) {
	var cif CallInterface
	if err := PrepareCallInterface(&cif, DefaultCall,
		FloatTypeDescriptor, []*TypeDescriptor{}); err != nil {
		t.Fatal(err)
	}
	var out float32
	if _, err := CallFunction(&cif, ptrOf(1),
		unsafe.Pointer(&out), nil); err == nil {
		t.Fatal("float return must error")
	}
}
