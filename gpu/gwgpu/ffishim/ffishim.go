// Package ffishim is gpui's self-maintained replacement for the external
// goffi C-ABI call layer used by the ported GLES backend (gpu/gwgpu).
//
// What it is: the same four operations the ported code needs — LoadLibrary,
// GetSymbol, PrepareCallInterface, CallFunction — plus the type-descriptor
// vocabulary, implemented on top of the purego FFI already used by
// gpu/rwgpu. No libffi, no external goffi dependency.
//
// How calls run: integer/pointer signatures go through purego.SyscallN.
// Signatures with float32 arguments (glClearColor, glClearDepthf,
// glDepthRangef, glSamplerParameterf) go through cached purego.RegisterFunc
// trampolines, because SysV passes floats in XMM registers, not integer
// registers. Returns are never float in the GLES call set; that case errors.
package ffishim

import (
	"fmt"
	"math"
	"runtime"
	"sync"
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
)

// TypeDescriptor describes one C ABI type (size/alignment/kind).
type TypeDescriptor struct {
	Size      int
	Alignment int
	Kind      TypeKind
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

	mu    sync.Mutex
	float map[unsafe.Pointer]any // fn -> RegisterFunc trampoline (float sigs)
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
	if hasFloat(cif) {
		return 0, callFloat(cif, fn, avalue)
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

func hasFloat(cif *CallInterface) bool {
	for _, t := range cif.ArgTypes {
		if t.Kind == FloatType || t.Kind == DoubleType {
			return true
		}
	}
	return cif.ReturnType.Kind == FloatType || cif.ReturnType.Kind == DoubleType
}

func loadArg(t *TypeDescriptor, p unsafe.Pointer) uintptr {
	switch t.Kind {
	case UInt32Type, SInt32Type, IntType, FloatType:
		return uintptr(*(*uint32)(p))
	case UInt8Type, SInt8Type:
		return uintptr(*(*uint8)(p))
	case UInt16Type, SInt16Type:
		return uintptr(*(*uint16)(p))
	case UInt64Type, SInt64Type, PointerType, DoubleType:
		return *(*uintptr)(p)
	default:
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
	case UInt32Type, SInt32Type, IntType, FloatType:
		*(*uint32)(rvalue) = uint32(r1)
	case UInt8Type, SInt8Type:
		*(*uint8)(rvalue) = uint8(r1)
	case UInt16Type, SInt16Type:
		*(*uint16)(rvalue) = uint16(r1)
	default: // UInt64, SInt64, Pointer, Double: full width
		*(*uintptr)(rvalue) = r1
	}
}

// callFloat handles the four float-argument signatures in the GLES call set
// via cached RegisterFunc trampolines (all void-returning):
// (f32), (f32,f32), (f32,f32,f32,f32), (u32,u32,f32).
func callFloat(cif *CallInterface, fn unsafe.Pointer, avalue []unsafe.Pointer) error {
	if cif.ReturnType.Kind != VoidType {
		return fmt.Errorf("ffishim: float return unsupported")
	}
	key := sigKey(cif.ArgTypes)
	cif.mu.Lock()
	if cif.float == nil {
		cif.float = make(map[unsafe.Pointer]any)
	}
	t, ok := cif.float[fn]
	if !ok {
		var err error
		t, err = makeFloatTrampoline(key, fn)
		if err != nil {
			cif.mu.Unlock()
			return err
		}
		cif.float[fn] = t
	}
	cif.mu.Unlock()
	switch key {
	case "f":
		t.(func(float32))(
			math.Float32frombits(*(*uint32)(avalue[0])))
	case "ff":
		t.(func(float32, float32))(
			math.Float32frombits(*(*uint32)(avalue[0])),
			math.Float32frombits(*(*uint32)(avalue[1])))
	case "ffff":
		t.(func(float32, float32, float32, float32))(
			math.Float32frombits(*(*uint32)(avalue[0])),
			math.Float32frombits(*(*uint32)(avalue[1])),
			math.Float32frombits(*(*uint32)(avalue[2])),
			math.Float32frombits(*(*uint32)(avalue[3])))
	case "uuf":
		t.(func(uint32, uint32, float32))(
			*(*uint32)(avalue[0]),
			*(*uint32)(avalue[1]),
			math.Float32frombits(*(*uint32)(avalue[2])))
	default:
		return fmt.Errorf("ffishim: float signature %q unsupported", key)
	}
	runtime.KeepAlive(avalue)
	return nil
}

func sigKey(args []*TypeDescriptor) string {
	var b []byte
	for _, t := range args {
		switch t.Kind {
		case FloatType:
			b = append(b, 'f')
		case UInt32Type, SInt32Type, IntType:
			b = append(b, 'u')
		case PointerType, UInt64Type, SInt64Type, DoubleType:
			b = append(b, 'p')
		default:
			b = append(b, '?')
		}
	}
	return string(b)
}

func makeFloatTrampoline(key string, fn unsafe.Pointer) (any, error) {
	switch key {
	case "f":
		var f func(float32)
		purego.RegisterFunc(&f, uintptr(fn))
		return f, nil
	case "ff":
		var f func(float32, float32)
		purego.RegisterFunc(&f, uintptr(fn))
		return f, nil
	case "ffff":
		var f func(float32, float32, float32, float32)
		purego.RegisterFunc(&f, uintptr(fn))
		return f, nil
	case "uuf":
		var f func(uint32, uint32, float32)
		purego.RegisterFunc(&f, uintptr(fn))
		return f, nil
	default:
		return nil, fmt.Errorf("ffishim: float signature %q unsupported", key)
	}
}
