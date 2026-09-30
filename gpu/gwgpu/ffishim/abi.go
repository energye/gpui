//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// ABI trampolines for float and small-struct signatures.
//
// Integer/pointer-only signatures go through purego.SyscallN (see
// CallFunction). Anything containing a float, double, or struct needs a
// real ABI call: floats ride in SIMD registers, small structs split across
// registers, large structs pass indirectly. Those go through cached
// purego.RegisterFunc trampolines built per exact signature.
//
// The trampoline cache is process-global (keyed by signature + function
// address) because some callers — notably the Metal ObjC bridge's msgSend —
// build a fresh CallInterface per call; caching per cif would mint a new
// trampoline every call.
//
// Supported signatures are exactly the ones the ported backends emit:
//   - GLES: (f32), (f32,f32), (f32*4), (u32,u32,f32), all void.
//   - Metal ObjC: setClearDepth (doubles), setBlendColor (4 floats),
//     and the fixed set of small-struct messages (viewports, scissor
//     rects, clear colors, origins, sizes, ranges, regions), plus the
//     single struct-returning query (acceleration-structure sizes) and
//     the two struct-returning test helpers.
//
// Anything else errors loudly so a new call shape can be added deliberately.
package ffishim

import (
	"fmt"
	"math"
	"runtime"
	"sync"
	"unsafe"

	purego "github.com/ebitengine/purego"
)

// Go mirrors of the fixed C struct shapes used by the ported Metal backend.
// Layouts match the C definitions field for field (all members 8 bytes,
// naturally aligned, no padding).
type structU2 struct{ A, B uint64 }             // NSRange
type structF2 struct{ A, B float64 }            // CGSize
type structU3 struct{ A, B, C uint64 }          // MTLOrigin, MTLSize
type structU4 struct{ A, B, C, D uint64 }       // MTLScissorRect
type structF4 struct{ A, B, C, D float64 }      // MTLClearColor
type structU6 struct{ A, B, C, D, E, F uint64 } // MTLRegion
type structF6 struct {
	A, B, C, D, E, F float64
} // MTLViewport

// abiTramp holds process-global RegisterFunc trampolines keyed by
// signature + function address.
var abiTramp sync.Map // string -> func([]unsafe.Pointer, unsafe.Pointer) error

// needsABI reports whether the signature needs a trampoline (any float,
// double, or struct in arguments or return).
func needsABI(cif *CallInterface) bool {
	for _, t := range cif.ArgTypes {
		if t == nil {
			continue
		}
		if t.Kind == FloatType || t.Kind == DoubleType || t.Kind == StructType {
			return true
		}
	}
	if cif.ReturnType != nil {
		k := cif.ReturnType.Kind
		return k == FloatType || k == DoubleType || k == StructType
	}
	return false
}

// abiCode encodes one scalar kind for signature keys.
func abiCode(t *TypeDescriptor) byte {
	switch t.Kind {
	case PointerType:
		return 'P'
	case UInt64Type, SInt64Type:
		return 'U'
	case UInt32Type, SInt32Type, IntType:
		return 'u'
	case UInt8Type, SInt8Type, UInt16Type, SInt16Type:
		return 'b'
	case FloatType:
		return 'F'
	case DoubleType:
		return 'D'
	case VoidType:
		return 'V'
	default:
		return '?'
	}
}

// abiKey builds the exact signature key: return + "|" + args. Structs
// encode as S[member-codes] with u = 8-byte int, d = float64.
func abiKey(cif *CallInterface) string {
	var b []byte
	b = append(b, abiTypeCode(cif.ReturnType)...)
	b = append(b, '|')
	for _, t := range cif.ArgTypes {
		b = append(b, abiTypeCode(t)...)
	}
	return string(b)
}

func abiTypeCode(t *TypeDescriptor) string {
	if t == nil {
		return "?"
	}
	if t.Kind != StructType {
		return string([]byte{abiCode(t)})
	}
	var b []byte
	b = append(b, 'S', '[')
	for _, m := range t.Members {
		switch m.Kind {
		case DoubleType:
			b = append(b, 'd')
		default:
			b = append(b, 'u')
		}
	}
	b = append(b, ']')
	return string(b)
}

// callABI dispatches one call through the cached trampoline for its exact
// signature. rvalue points at the return buffer (nil for void).
func callABI(cif *CallInterface, fn unsafe.Pointer, rvalue unsafe.Pointer, avalue []unsafe.Pointer) error {
	if cif.ReturnType != nil {
		switch cif.ReturnType.Kind {
		case FloatType, DoubleType:
			return fmt.Errorf("ffishim: float return unsupported")
		}
	}
	key := abiKey(cif)
	ck := fmt.Sprintf("%s@%x", key, uintptr(fn))
	if v, ok := abiTramp.Load(ck); ok {
		return v.(func([]unsafe.Pointer, unsafe.Pointer) error)(avalue, rvalue)
	}
	inv, err := makeABIInvoker(key, fn)
	if err != nil {
		return err
	}
	actual, _ := abiTramp.LoadOrStore(ck, inv)
	return actual.(func([]unsafe.Pointer, unsafe.Pointer) error)(avalue, rvalue)
}

// Scalar readers: each avalue element points at the argument value.
func rdU64(p unsafe.Pointer) uint64   { return *(*uint64)(p) }
func rdU32(p unsafe.Pointer) uint32   { return *(*uint32)(p) }
func rdU8(p unsafe.Pointer) uint8     { return *(*uint8)(p) }
func rdPtr(p unsafe.Pointer) uintptr  { return *(*uintptr)(p) }
func rdF32(p unsafe.Pointer) float32  { return math.Float32frombits(*(*uint32)(p)) }
func rdF64(p unsafe.Pointer) float64  { return math.Float64frombits(*(*uint64)(p)) }
func rdSU2(p unsafe.Pointer) structU2 { return structU2{rdU64(p), rdU64(at(p, 8))} }
func rdSF2(p unsafe.Pointer) structF2 { return structF2{rdF64(p), rdF64(at(p, 8))} }
func rdSU3(p unsafe.Pointer) structU3 { return structU3{rdU64(p), rdU64(at(p, 8)), rdU64(at(p, 16))} }
func rdSU4(p unsafe.Pointer) structU4 {
	return structU4{rdU64(p), rdU64(at(p, 8)), rdU64(at(p, 16)), rdU64(at(p, 24))}
}
func rdSF4(p unsafe.Pointer) structF4 {
	return structF4{rdF64(p), rdF64(at(p, 8)), rdF64(at(p, 16)), rdF64(at(p, 24))}
}
func rdSU6(p unsafe.Pointer) structU6 {
	return structU6{rdU64(p), rdU64(at(p, 8)), rdU64(at(p, 16)), rdU64(at(p, 24)), rdU64(at(p, 32)), rdU64(at(p, 40))}
}
func rdSF6(p unsafe.Pointer) structF6 {
	return structF6{rdF64(p), rdF64(at(p, 8)), rdF64(at(p, 16)), rdF64(at(p, 24)), rdF64(at(p, 32)), rdF64(at(p, 40))}
}

func at(p unsafe.Pointer, off uintptr) unsafe.Pointer {
	return unsafe.Pointer(uintptr(p) + off)
}

func wrU64(p unsafe.Pointer, v uint64)  { *(*uint64)(p) = v }
func wrPtr(p unsafe.Pointer, v uintptr) { *(*uintptr)(p) = v }

// makeABIInvoker builds the trampoline + materializer for one exact
// signature key. rvalue is written for non-void returns.
func makeABIInvoker(key string, fn unsafe.Pointer) (func([]unsafe.Pointer, unsafe.Pointer) error, error) {
	switch key {
	// -- GLES float signatures (all void) --
	case "V|F":
		var f func(float32)
		purego.RegisterFunc(&f, uintptr(fn))
		return func(a []unsafe.Pointer, _ unsafe.Pointer) error {
			f(rdF32(a[0]))
			runtime.KeepAlive(a)
			return nil
		}, nil
	case "V|FF":
		var f func(float32, float32)
		purego.RegisterFunc(&f, uintptr(fn))
		return func(a []unsafe.Pointer, _ unsafe.Pointer) error {
			f(rdF32(a[0]), rdF32(a[1]))
			runtime.KeepAlive(a)
			return nil
		}, nil
	case "V|FFFF":
		var f func(float32, float32, float32, float32)
		purego.RegisterFunc(&f, uintptr(fn))
		return func(a []unsafe.Pointer, _ unsafe.Pointer) error {
			f(rdF32(a[0]), rdF32(a[1]), rdF32(a[2]), rdF32(a[3]))
			runtime.KeepAlive(a)
			return nil
		}, nil
	case "V|uuF":
		var f func(uint32, uint32, float32)
		purego.RegisterFunc(&f, uintptr(fn))
		return func(a []unsafe.Pointer, _ unsafe.Pointer) error {
			f(rdU32(a[0]), rdU32(a[1]), rdF32(a[2]))
			runtime.KeepAlive(a)
			return nil
		}, nil
	// -- EGL device enumeration (int/pointer-only, but routed here
	// deliberately): libglvnd dispatch for eglQueryDevicesEXT answers
	// through the RegisterFunc trampoline (ok=1) while the same bytes
	// through the SyscallN/cgocall path come back EGL_FALSE or fault —
	// verified 2026-09 on NVIDIA 580 + Mesa 25 (suspected al/XMM-state
	// sensitivity in loader dispatch forwarding; normal callees ignore
	// it, dispatch forwarders apparently do not). Callers must use
	// CallTrampoline, never CallFunction, for these two shapes.
	case "u|uPP": // EGLBoolean (u32, ptr, ptr): eglQueryDevicesEXT
		var f func(uint32, uintptr, uintptr) uint32
		purego.RegisterFunc(&f, uintptr(fn))
		return func(a []unsafe.Pointer, r unsafe.Pointer) error {
			v := f(rdU32(a[0]), rdPtr(a[1]), rdPtr(a[2]))
			if r != nil {
				*(*uint32)(r) = v
			}
			runtime.KeepAlive(a)
			return nil
		}, nil
	case "P|Pu": // const char * (device, u32): eglQueryDeviceStringEXT
		var f func(uintptr, uint32) uintptr
		purego.RegisterFunc(&f, uintptr(fn))
		return func(a []unsafe.Pointer, r unsafe.Pointer) error {
			v := f(rdPtr(a[0]), rdU32(a[1]))
			if r != nil {
				wrPtr(r, v)
			}
			runtime.KeepAlive(a)
			return nil
		}, nil
	// -- Metal scalar-float signatures (all void) --
	case "V|PPD": // setClearDepth:
		var f func(uintptr, uintptr, float64)
		purego.RegisterFunc(&f, uintptr(fn))
		return func(a []unsafe.Pointer, _ unsafe.Pointer) error {
			f(rdPtr(a[0]), rdPtr(a[1]), rdF64(a[2]))
			runtime.KeepAlive(a)
			return nil
		}, nil
	case "V|PPFFFF": // setBlendColorRed:green:blue:alpha:
		var f func(uintptr, uintptr, float32, float32, float32, float32)
		purego.RegisterFunc(&f, uintptr(fn))
		return func(a []unsafe.Pointer, _ unsafe.Pointer) error {
			f(rdPtr(a[0]), rdPtr(a[1]), rdF32(a[2]), rdF32(a[3]), rdF32(a[4]), rdF32(a[5]))
			runtime.KeepAlive(a)
			return nil
		}, nil
	// -- Metal struct signatures (self, sel first) --
	case "P|PPUUS[uu]S[uu]": // newTextureViewWithPixelFormat:... -> ID
		var f func(uintptr, uintptr, uint64, uint64, structU2, structU2) uintptr
		purego.RegisterFunc(&f, uintptr(fn))
		return func(a []unsafe.Pointer, r unsafe.Pointer) error {
			v := f(rdPtr(a[0]), rdPtr(a[1]), rdU64(a[2]), rdU64(a[3]), rdSU2(a[4]), rdSU2(a[5]))
			if r != nil {
				wrPtr(r, v)
			}
			runtime.KeepAlive(a)
			return nil
		}, nil
	case "V|PPPUUUS[uuu]PUUS[uuu]": // copyBufferToTexture:...
		var f func(uintptr, uintptr, uintptr, uint64, uint64, uint64, structU3, uintptr, uint64, uint64, structU3)
		purego.RegisterFunc(&f, uintptr(fn))
		return func(a []unsafe.Pointer, _ unsafe.Pointer) error {
			f(rdPtr(a[0]), rdPtr(a[1]), rdPtr(a[2]), rdU64(a[3]), rdU64(a[4]), rdU64(a[5]), rdSU3(a[6]), rdPtr(a[7]), rdU64(a[8]), rdU64(a[9]), rdSU3(a[10]))
			runtime.KeepAlive(a)
			return nil
		}, nil
	case "V|PPPUUS[uuu]S[uuu]PUUU": // copyTextureToBuffer:...
		var f func(uintptr, uintptr, uintptr, uint64, uint64, structU3, structU3, uintptr, uint64, uint64, uint64)
		purego.RegisterFunc(&f, uintptr(fn))
		return func(a []unsafe.Pointer, _ unsafe.Pointer) error {
			f(rdPtr(a[0]), rdPtr(a[1]), rdPtr(a[2]), rdU64(a[3]), rdU64(a[4]), rdSU3(a[5]), rdSU3(a[6]), rdPtr(a[7]), rdU64(a[8]), rdU64(a[9]), rdU64(a[10]))
			runtime.KeepAlive(a)
			return nil
		}, nil
	case "V|PPPUUS[uuu]S[uuu]PUUS[uuu]": // copyTextureToTexture:...
		var f func(uintptr, uintptr, uintptr, uint64, uint64, structU3, structU3, uintptr, uint64, uint64, structU3)
		purego.RegisterFunc(&f, uintptr(fn))
		return func(a []unsafe.Pointer, _ unsafe.Pointer) error {
			f(rdPtr(a[0]), rdPtr(a[1]), rdPtr(a[2]), rdU64(a[3]), rdU64(a[4]), rdSU3(a[5]), rdSU3(a[6]), rdPtr(a[7]), rdU64(a[8]), rdU64(a[9]), rdSU3(a[10]))
			runtime.KeepAlive(a)
			return nil
		}, nil
	case "V|PPS[uuuuuu]UUPUUU": // replaceRegion:...
		var f func(uintptr, uintptr, structU6, uint64, uint64, uintptr, uint64, uint64)
		purego.RegisterFunc(&f, uintptr(fn))
		return func(a []unsafe.Pointer, _ unsafe.Pointer) error {
			f(rdPtr(a[0]), rdPtr(a[1]), rdSU6(a[2]), rdU64(a[3]), rdU64(a[4]), rdPtr(a[5]), rdU64(a[6]), rdU64(a[7]))
			runtime.KeepAlive(a)
			return nil
		}, nil
	case "V|PPS[ffffff]": // setViewport:
		var f func(uintptr, uintptr, structF6)
		purego.RegisterFunc(&f, uintptr(fn))
		return func(a []unsafe.Pointer, _ unsafe.Pointer) error {
			f(rdPtr(a[0]), rdPtr(a[1]), rdSF6(a[2]))
			runtime.KeepAlive(a)
			return nil
		}, nil
	case "V|PPS[uuuu]": // setScissorRect:
		var f func(uintptr, uintptr, structU4)
		purego.RegisterFunc(&f, uintptr(fn))
		return func(a []unsafe.Pointer, _ unsafe.Pointer) error {
			f(rdPtr(a[0]), rdPtr(a[1]), rdSU4(a[2]))
			runtime.KeepAlive(a)
			return nil
		}, nil
	case "V|PPS[ffff]": // setClearColor:
		var f func(uintptr, uintptr, structF4)
		purego.RegisterFunc(&f, uintptr(fn))
		return func(a []unsafe.Pointer, _ unsafe.Pointer) error {
			f(rdPtr(a[0]), rdPtr(a[1]), rdSF4(a[2]))
			runtime.KeepAlive(a)
			return nil
		}, nil
	case "V|PPS[uuu]S[uuu]": // dispatchThreadgroups:...
		var f func(uintptr, uintptr, structU3, structU3)
		purego.RegisterFunc(&f, uintptr(fn))
		return func(a []unsafe.Pointer, _ unsafe.Pointer) error {
			f(rdPtr(a[0]), rdPtr(a[1]), rdSU3(a[2]), rdSU3(a[3]))
			runtime.KeepAlive(a)
			return nil
		}, nil
	case "V|PPPUS[uuu]": // dispatchThreadgroupsWithIndirectBuffer:...
		var f func(uintptr, uintptr, uintptr, uint64, structU3)
		purego.RegisterFunc(&f, uintptr(fn))
		return func(a []unsafe.Pointer, _ unsafe.Pointer) error {
			f(rdPtr(a[0]), rdPtr(a[1]), rdPtr(a[2]), rdU64(a[3]), rdSU3(a[4]))
			runtime.KeepAlive(a)
			return nil
		}, nil
	case "V|PPS[ff]": // CGSize-taking setters (test + surface sizing)
		var f func(uintptr, uintptr, structF2)
		purego.RegisterFunc(&f, uintptr(fn))
		return func(a []unsafe.Pointer, _ unsafe.Pointer) error {
			f(rdPtr(a[0]), rdPtr(a[1]), rdSF2(a[2]))
			runtime.KeepAlive(a)
			return nil
		}, nil
	case "V|PPS[uu]": // resetWithRange:
		var f func(uintptr, uintptr, structU2)
		purego.RegisterFunc(&f, uintptr(fn))
		return func(a []unsafe.Pointer, _ unsafe.Pointer) error {
			f(rdPtr(a[0]), rdPtr(a[1]), rdSU2(a[2]))
			runtime.KeepAlive(a)
			return nil
		}, nil
	case "V|PPPS[uu]": // executeCommandsInBuffer:withRange:
		var f func(uintptr, uintptr, uintptr, structU2)
		purego.RegisterFunc(&f, uintptr(fn))
		return func(a []unsafe.Pointer, _ unsafe.Pointer) error {
			f(rdPtr(a[0]), rdPtr(a[1]), rdPtr(a[2]), rdSU2(a[3]))
			runtime.KeepAlive(a)
			return nil
		}, nil
	case "S[uuu]|PPP": // accelerationStructureSizesWithDescriptor: -> 3xU64
		if runtime.GOARCH == "amd64" {
			// x86_64 uses the stret entry: hidden result pointer first.
			var f func(uintptr, uintptr, uintptr, uintptr)
			purego.RegisterFunc(&f, uintptr(fn))
			return func(a []unsafe.Pointer, r unsafe.Pointer) error {
				f(uintptr(r), rdPtr(a[0]), rdPtr(a[1]), rdPtr(a[2]))
				runtime.KeepAlive(a)
				if r != nil {
					runtime.KeepAlive(r)
				}
				return nil
			}, nil
		}
		var f func(uintptr, uintptr, uintptr) structU3
		purego.RegisterFunc(&f, uintptr(fn))
		return func(a []unsafe.Pointer, r unsafe.Pointer) error {
			v := f(rdPtr(a[0]), rdPtr(a[1]), rdPtr(a[2]))
			if r != nil {
				wrU64(r, v.A)
				wrU64(at(r, 8), v.B)
				wrU64(at(r, 16), v.C)
			}
			runtime.KeepAlive(a)
			return nil
		}, nil
	case "S[ffff]|PP": // clearColor (test helper) -> MTLClearColor
		if runtime.GOARCH == "amd64" {
			var f func(uintptr, uintptr, uintptr)
			purego.RegisterFunc(&f, uintptr(fn))
			return func(a []unsafe.Pointer, r unsafe.Pointer) error {
				f(uintptr(r), rdPtr(a[0]), rdPtr(a[1]))
				runtime.KeepAlive(a)
				if r != nil {
					runtime.KeepAlive(r)
				}
				return nil
			}, nil
		}
		var f func(uintptr, uintptr) structF4
		purego.RegisterFunc(&f, uintptr(fn))
		return func(a []unsafe.Pointer, r unsafe.Pointer) error {
			v := f(rdPtr(a[0]), rdPtr(a[1]))
			if r != nil {
				*(*structF4)(r) = v
			}
			runtime.KeepAlive(a)
			return nil
		}, nil
	case "S[ff]|PP": // CGSize-returning getters (test helper)
		var f func(uintptr, uintptr) structF2
		purego.RegisterFunc(&f, uintptr(fn))
		return func(a []unsafe.Pointer, r unsafe.Pointer) error {
			v := f(rdPtr(a[0]), rdPtr(a[1]))
			if r != nil {
				*(*structF2)(r) = v
			}
			runtime.KeepAlive(a)
			return nil
		}, nil
	default:
		return nil, fmt.Errorf("ffishim: signature %q unsupported (add a trampoline deliberately)", key)
	}
}
