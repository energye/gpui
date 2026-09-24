// Package ffishim is gpui's self-maintained replacement for the external
// goffi C-ABI call layer used by the ported backends (gpu/gwgpu/gles,
// gpu/gwgpu/metal).
//
// What it is: the same operations the ported code needs — LoadLibrary,
// GetSymbol, PrepareCallInterface, CallFunction, NewCallback — plus the
// type-descriptor vocabulary (including struct descriptors), implemented on
// top of the purego FFI already used by gpu/rwgpu. No libffi, no external
// goffi dependency.
//
// How calls run: integer/pointer signatures go through purego.SyscallN.
// Signatures containing floats, doubles, or structs go through cached
// purego.RegisterFunc trampolines built per exact signature (see abi.go),
// because those ride in SIMD registers or split across registers. Only the
// signatures the ported backends actually emit are supported; anything else
// errors loudly so it can be added deliberately.
package ffishim

import (
	"fmt"
	"runtime"
	"unsafe"

	purego "github.com/ebitengine/purego"
)

// CallingConvention selects the platform native call convention.
// Only DefaultCall is used by the ported code.
type CallingConvention int

const (
	// DefaultCall resolves to the platform native convention (SysV on
	// Linux, Win64 on Windows) — same meaning as goffi's DefaultCall.
	DefaultCall CallingConvention = 0
)

// TypeKind identifies one scalar C ABI type.
type TypeKind int

const (
	VoidType TypeKind = iota
	IntType
	UInt8Type
	SInt8Type
	UInt16Type
	SInt16Type
	UInt32Type
	SInt32Type
	UInt64Type
	SInt64Type
	FloatType
	DoubleType
	PointerType
	// StructType is a small fixed-layout C struct (Metal ObjC bridge:
	// CGSize, MTLOrigin/Size/Region, viewports, scissor rects, ranges).
	// Members holds the member descriptors in order; all members used
	// by the ported code are 8 bytes (uint64 or double), naturally
	// aligned, without padding.
	StructType
)

// TypeDescriptor describes one C ABI type (size/alignment/kind).
// For StructType, Members holds the member descriptors in order.
// Size/Alignment are uintptr, matching the upstream goffi vocabulary.
type TypeDescriptor struct {
	Size      uintptr
	Alignment uintptr
	Kind      TypeKind
	Members   []*TypeDescriptor
}

var (
	VoidTypeDescriptor    = &TypeDescriptor{Size: 1, Alignment: 1, Kind: VoidType}
	IntTypeDescriptor     = &TypeDescriptor{Size: 4, Alignment: 4, Kind: IntType}
	FloatTypeDescriptor   = &TypeDescriptor{Size: 4, Alignment: 4, Kind: FloatType}
	DoubleTypeDescriptor  = &TypeDescriptor{Size: 8, Alignment: 8, Kind: DoubleType}
	UInt8TypeDescriptor   = &TypeDescriptor{Size: 1, Alignment: 1, Kind: UInt8Type}
	SInt8TypeDescriptor   = &TypeDescriptor{Size: 1, Alignment: 1, Kind: SInt8Type}
	UInt16TypeDescriptor  = &TypeDescriptor{Size: 2, Alignment: 2, Kind: UInt16Type}
	SInt16TypeDescriptor  = &TypeDescriptor{Size: 2, Alignment: 2, Kind: SInt16Type}
	UInt32TypeDescriptor  = &TypeDescriptor{Size: 4, Alignment: 4, Kind: UInt32Type}
	SInt32TypeDescriptor  = &TypeDescriptor{Size: 4, Alignment: 4, Kind: SInt32Type}
	UInt64TypeDescriptor  = &TypeDescriptor{Size: 8, Alignment: 8, Kind: UInt64Type}
	SInt64TypeDescriptor  = &TypeDescriptor{Size: 8, Alignment: 8, Kind: SInt64Type}
	PointerTypeDescriptor = &TypeDescriptor{Size: 8, Alignment: 8, Kind: PointerType}
)

// CallInterface is a prepared call signature: return type plus argument
// types. Prepared once by PrepareCallInterface, reused per call.
type CallInterface struct {
	ArgTypes   []*TypeDescriptor
	ReturnType *TypeDescriptor
}

// ptrOf converts a uintptr handle to unsafe.Pointer without tripping
// vet's unsafeptr check (same bit pattern, no arithmetic).
func ptrOf(u uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&u))
}

// LoadLibrary opens a shared library (RTLD_NOW|RTLD_GLOBAL, same as rwgpu).
func LoadLibrary(name string) (unsafe.Pointer, error) {
	h, err := purego.Dlopen(name, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return nil, err
	}
	return ptrOf(h), nil
}

// GetSymbol resolves a symbol from an opened library.
func GetSymbol(handle unsafe.Pointer, name string) (unsafe.Pointer, error) {
	ptr, err := purego.Dlsym(uintptr(handle), name)
	if err != nil {
		return nil, err
	}
	return ptrOf(ptr), nil
}

// FreeLibrary closes an opened library.
func FreeLibrary(handle unsafe.Pointer) error {
	return purego.Dlclose(uintptr(handle))
}

// PrepareCallInterface records the signature into cif. Only scalar and
// pointer kinds are supported — the GLES call set needs nothing else.
func PrepareCallInterface(
	cif *CallInterface,
	convention CallingConvention,
	returnType *TypeDescriptor,
	argTypes []*TypeDescriptor,
) error {
	if cif == nil {
		return fmt.Errorf("ffishim: cif must not be nil")
	}
	if returnType == nil {
		return fmt.Errorf("ffishim: returnType must not be nil")
	}
	cif.ArgTypes = argTypes
	cif.ReturnType = returnType
	return nil
}

// CallFunction invokes fn with the prepared signature. Each avalue element
// is a pointer to the argument value; rvalue points at the return buffer
// (nil for void). Returns the C errno (always 0 here) and error.
func CallFunction(
	cif *CallInterface,
	fn unsafe.Pointer,
	rvalue unsafe.Pointer,
	avalue []unsafe.Pointer,
) (uintptr, error) {
	if cif == nil {
		return 0, fmt.Errorf("ffishim: cif must not be nil")
	}
	if fn == nil {
		return 0, fmt.Errorf("ffishim: fn must not be nil")
	}
	if len(avalue) != len(cif.ArgTypes) {
		return 0, fmt.Errorf("ffishim: got %d args, signature wants %d", len(avalue), len(cif.ArgTypes))
	}
	if needsABI(cif) {
		return 0, callABI(cif, fn, rvalue, avalue)
	}
	vals := make([]uintptr, len(avalue))
	for i, t := range cif.ArgTypes {
		vals[i] = loadArg(t, avalue[i])
	}
	r1, _, _ := purego.SyscallN(uintptr(fn), vals...)
	runtime.KeepAlive(avalue)
	storeRet(cif.ReturnType, rvalue, r1)
	if rvalue != nil {
		runtime.KeepAlive(rvalue)
	}
	return 0, nil
}

// loadArg reads one integer/pointer argument value for the SyscallN path.
// Float, double, and struct arguments never reach here (needsABI routes
// them to trampolines).
func loadArg(t *TypeDescriptor, p unsafe.Pointer) uintptr {
	switch t.Kind {
	case UInt32Type, SInt32Type, IntType:
		return uintptr(*(*uint32)(p))
	case UInt8Type, SInt8Type:
		return uintptr(*(*uint8)(p))
	case UInt16Type, SInt16Type:
		return uintptr(*(*uint16)(p))
	default: // UInt64, SInt64, Pointer: full width
		return *(*uintptr)(p)
	}
}

func storeRet(t *TypeDescriptor, rvalue unsafe.Pointer, r1 uintptr) {
	if rvalue == nil {
		return
	}
	switch t.Kind {
	case VoidType:
		return
	case UInt32Type, SInt32Type, IntType:
		*(*uint32)(rvalue) = uint32(r1)
	case UInt8Type, SInt8Type:
		*(*uint8)(rvalue) = uint8(r1)
	case UInt16Type, SInt16Type:
		*(*uint16)(rvalue) = uint16(r1)
	default: // UInt64, SInt64, Pointer: full width
		*(*uintptr)(rvalue) = r1
	}
}

// NewCallback converts a Go function to a C function pointer (ObjC block
// trampolines in the ported Metal backend). Backed by purego.NewCallback:
// the function must take uintptr-sized arguments with zero or one
// uintptr-sized result. At least 1024 callbacks can always be created;
// memory for created callbacks is never released.
func NewCallback(fn any) uintptr {
	return purego.NewCallback(fn)
}
