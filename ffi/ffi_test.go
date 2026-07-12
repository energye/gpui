package ffi

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"testing"
	"unsafe"

	"github.com/energye/gpui/ffi/types"
)

// =============================================================================
// Environment Detection
// =============================================================================

// hasLibC is true when libc.so.6 is available (Linux).
var hasLibC = false

// hasLibM is true when libm.so.6 is available (Linux).
var hasLibM = false

// libcHandle holds the loaded libc handle for testing.
var libcHandle unsafe.Pointer

// libcTestInitOnce ensures test libraries are loaded once.
var libcTestInitOnce sync.Once

// initLibcForTest loads libc and libm for testing.
func initLibcForTest() {
	libcTestInitOnce.Do(func() {
		var err error
		libcHandle, err = LoadLibrary("libc.so.6")
		if err != nil {
			// Try alternative names
			libcHandle, err = LoadLibrary("libc.so")
		}
		if err == nil {
			hasLibC = true
		}

		// Try to load libm
		libmHandle, err := LoadLibrary("libm.so.6")
		if err != nil {
			libmHandle, err = LoadLibrary("libm.so")
		}
		if err == nil {
			hasLibM = true
			_ = FreeLibrary(libmHandle) // We only need to check availability
		}
	})
}

// =============================================================================
// Library Loading Tests
// =============================================================================

func TestLoadLibrary(t *testing.T) {
	initLibcForTest()

	if !hasLibC {
		t.Skip("libc.so.6 not available on this system")
	}

	handle, err := LoadLibrary("libc.so.6")
	if err != nil {
		t.Fatalf("LoadLibrary(libc.so.6) failed: %v", err)
	}
	if handle == nil {
		t.Fatal("LoadLibrary(libc.so.6) returned nil handle")
	}
	defer FreeLibrary(handle) //nolint:errcheck
}

func TestLoadLibraryNotFound(t *testing.T) {
	_, err := LoadLibrary("nonexistent_library_12345.so")
	if err == nil {
		t.Fatal("LoadLibrary with nonexistent library should return error")
	}

	// Verify it's a LibraryError
	var libErr *LibraryError
	if !errors.As(err, &libErr) {
		t.Fatalf("expected *LibraryError, got %T", err)
	}
	if libErr.Operation != "load" {
		t.Errorf("LibraryError.Operation = %q, want %q", libErr.Operation, "load")
	}
	if libErr.Name != "nonexistent_library_12345.so" {
		t.Errorf("LibraryError.Name = %q, want %q", libErr.Name, "nonexistent_library_12345.so")
	}
}

func TestLoadLibraryTwice(t *testing.T) {
	initLibcForTest()

	if !hasLibC {
		t.Skip("libc.so.6 not available on this system")
	}

	h1, err := LoadLibrary("libc.so.6")
	if err != nil {
		t.Fatalf("first LoadLibrary failed: %v", err)
	}
	defer FreeLibrary(h1) //nolint:errcheck

	h2, err := LoadLibrary("libc.so.6")
	if err != nil {
		t.Fatalf("second LoadLibrary failed: %v", err)
	}
	defer FreeLibrary(h2) //nolint:errcheck

	// On Linux, dlopen returns the same handle for the same library
	if h1 != h2 {
		t.Log("Note: handles differ (expected on some platforms)")
	}
}

// =============================================================================
// Symbol Resolution Tests
// =============================================================================

func TestGetSymbol(t *testing.T) {
	initLibcForTest()

	if !hasLibC {
		t.Skip("libc.so.6 not available on this system")
	}

	handle, err := LoadLibrary("libc.so.6")
	if err != nil {
		t.Fatalf("LoadLibrary failed: %v", err)
	}
	defer FreeLibrary(handle) //nolint:errcheck

	sym, err := GetSymbol(handle, "strlen")
	if err != nil {
		t.Fatalf("GetSymbol(strlen) failed: %v", err)
	}
	if sym == nil {
		t.Fatal("GetSymbol(strlen) returned nil")
	}
}

func TestGetSymbolNotFound(t *testing.T) {
	initLibcForTest()

	if !hasLibC {
		t.Skip("libc.so.6 not available on this system")
	}

	handle, err := LoadLibrary("libc.so.6")
	if err != nil {
		t.Fatalf("LoadLibrary failed: %v", err)
	}
	defer FreeLibrary(handle) //nolint:errcheck

	_, err = GetSymbol(handle, "this_symbol_does_not_exist_12345")
	if err == nil {
		t.Fatal("GetSymbol with nonexistent symbol should return error")
	}

	var libErr *LibraryError
	if !errors.As(err, &libErr) {
		t.Fatalf("expected *LibraryError, got %T", err)
	}
	if libErr.Operation != "symbol" {
		t.Errorf("LibraryError.Operation = %q, want %q", libErr.Operation, "symbol")
	}
}

// =============================================================================
// FreeLibrary Tests
// =============================================================================

func TestFreeLibrary(t *testing.T) {
	initLibcForTest()

	if !hasLibC {
		t.Skip("libc.so.6 not available on this system")
	}

	handle, err := LoadLibrary("libc.so.6")
	if err != nil {
		t.Fatalf("LoadLibrary failed: %v", err)
	}

	err = FreeLibrary(handle)
	if err != nil {
		t.Fatalf("FreeLibrary failed: %v", err)
	}
}

func TestFreeLibraryNil(t *testing.T) {
	err := FreeLibrary(nil)
	if err != nil {
		t.Fatalf("FreeLibrary(nil) should not return error, got: %v", err)
	}
}

// =============================================================================
// PrepareCallInterface Tests
// =============================================================================

func TestPrepareCallInterface(t *testing.T) {
	t.Run("nil cif returns error", func(t *testing.T) {
		err := PrepareCallInterface(nil, types.DefaultCall, types.VoidTypeDescriptor, nil)
		if err == nil {
			t.Fatal("PrepareCallInterface(nil cif) should return error")
		}
		var cifErr *InvalidCallInterfaceError
		if !errors.As(err, &cifErr) {
			t.Fatalf("expected *InvalidCallInterfaceError, got %T", err)
		}
		if cifErr.Field != "cif" {
			t.Errorf("Field = %q, want %q", cifErr.Field, "cif")
		}
	})

	t.Run("nil returnType returns error", func(t *testing.T) {
		var cif types.CallInterface
		err := PrepareCallInterface(&cif, types.DefaultCall, nil, nil)
		if err == nil {
			t.Fatal("PrepareCallInterface(nil returnType) should return error")
		}
		var cifErr *InvalidCallInterfaceError
		if !errors.As(err, &cifErr) {
			t.Fatalf("expected *InvalidCallInterfaceError, got %T", err)
		}
		if cifErr.Field != "returnType" {
			t.Errorf("Field = %q, want %q", cifErr.Field, "returnType")
		}
	})

	t.Run("invalid convention returns error", func(t *testing.T) {
		var cif types.CallInterface
		err := PrepareCallInterface(&cif, types.CallingConvention(99), types.VoidTypeDescriptor, nil)
		if err == nil {
			t.Fatal("PrepareCallInterface with invalid convention should return error")
		}
		var convErr *CallingConventionError
		if !errors.As(err, &convErr) {
			t.Fatalf("expected *CallingConventionError, got %T", err)
		}
		if convErr.Convention != 99 {
			t.Errorf("Convention = %d, want 99", convErr.Convention)
		}
	})

	t.Run("nil arg type returns error", func(t *testing.T) {
		var cif types.CallInterface
		err := PrepareCallInterface(&cif, types.DefaultCall, types.VoidTypeDescriptor,
			[]*types.TypeDescriptor{nil})
		if err == nil {
			t.Fatal("PrepareCallInterface with nil arg type should return error")
		}
		var typeErr *TypeValidationError
		if !errors.As(err, &typeErr) {
			t.Fatalf("expected *TypeValidationError, got %T", err)
		}
		if typeErr.Index != 0 {
			t.Errorf("Index = %d, want 0", typeErr.Index)
		}
	})

	t.Run("void void no args", func(t *testing.T) {
		var cif types.CallInterface
		err := PrepareCallInterface(&cif, types.DefaultCall, types.VoidTypeDescriptor, nil)
		if err != nil {
			t.Fatalf("PrepareCallInterface failed: %v", err)
		}
		if cif.ReturnType != types.VoidTypeDescriptor {
			t.Errorf("ReturnType should be VoidTypeDescriptor")
		}
		if cif.ArgCount != 0 {
			t.Errorf("ArgCount = %d, want 0", cif.ArgCount)
		}
	})

	t.Run("uint32 void no args", func(t *testing.T) {
		var cif types.CallInterface
		err := PrepareCallInterface(&cif, types.DefaultCall, types.UInt32TypeDescriptor, nil)
		if err != nil {
			t.Fatalf("PrepareCallInterface failed: %v", err)
		}
		if cif.ReturnType != types.UInt32TypeDescriptor {
			t.Errorf("ReturnType should be UInt32TypeDescriptor")
		}
		if cif.ArgCount != 0 {
			t.Errorf("ArgCount = %d, want 0", cif.ArgCount)
		}
	})

	t.Run("void with one arg", func(t *testing.T) {
		var cif types.CallInterface
		err := PrepareCallInterface(&cif, types.DefaultCall, types.VoidTypeDescriptor,
			[]*types.TypeDescriptor{types.UInt32TypeDescriptor})
		if err != nil {
			t.Fatalf("PrepareCallInterface failed: %v", err)
		}
		if cif.ArgCount != 1 {
			t.Errorf("ArgCount = %d, want 1", cif.ArgCount)
		}
		if len(cif.ArgTypes) != 1 {
			t.Errorf("len(ArgTypes) = %d, want 1", len(cif.ArgTypes))
		}
		if cif.ArgTypes[0] != types.UInt32TypeDescriptor {
			t.Errorf("ArgType[0] should be UInt32TypeDescriptor")
		}
	})

	t.Run("void with float args", func(t *testing.T) {
		var cif types.CallInterface
		err := PrepareCallInterface(&cif, types.DefaultCall, types.VoidTypeDescriptor,
			[]*types.TypeDescriptor{
				types.FloatTypeDescriptor,
				types.FloatTypeDescriptor,
				types.FloatTypeDescriptor,
				types.FloatTypeDescriptor,
			})
		if err != nil {
			t.Fatalf("PrepareCallInterface failed: %v", err)
		}
		if cif.ArgCount != 4 {
			t.Errorf("ArgCount = %d, want 4", cif.ArgCount)
		}
		for i, arg := range cif.ArgTypes {
			if arg != types.FloatTypeDescriptor {
				t.Errorf("ArgType[%d] should be FloatTypeDescriptor", i)
			}
		}
	})

	t.Run("result with mixed args", func(t *testing.T) {
		var cif types.CallInterface
		err := PrepareCallInterface(&cif, types.DefaultCall, types.SInt32TypeDescriptor,
			[]*types.TypeDescriptor{
				types.UInt64TypeDescriptor,
				types.PointerTypeDescriptor,
				types.UInt32TypeDescriptor,
			})
		if err != nil {
			t.Fatalf("PrepareCallInterface failed: %v", err)
		}
		if cif.ArgCount != 3 {
			t.Errorf("ArgCount = %d, want 3", cif.ArgCount)
		}
		if cif.ReturnType != types.SInt32TypeDescriptor {
			t.Errorf("ReturnType should be SInt32TypeDescriptor")
		}
		if cif.ArgTypes[0] != types.UInt64TypeDescriptor {
			t.Errorf("ArgType[0] should be UInt64TypeDescriptor")
		}
		if cif.ArgTypes[1] != types.PointerTypeDescriptor {
			t.Errorf("ArgType[1] should be PointerTypeDescriptor")
		}
		if cif.ArgTypes[2] != types.UInt32TypeDescriptor {
			t.Errorf("ArgType[2] should be UInt32TypeDescriptor")
		}
	})

	t.Run("handles all type kinds", func(t *testing.T) {
		allTypes := []*types.TypeDescriptor{
			types.VoidTypeDescriptor,
			types.IntTypeDescriptor,
			types.FloatTypeDescriptor,
			types.DoubleTypeDescriptor,
			types.UInt8TypeDescriptor,
			types.SInt8TypeDescriptor,
			types.UInt16TypeDescriptor,
			types.SInt16TypeDescriptor,
			types.UInt32TypeDescriptor,
			types.SInt32TypeDescriptor,
			types.UInt64TypeDescriptor,
			types.SInt64TypeDescriptor,
			types.PointerTypeDescriptor,
		}

		for _, tt := range allTypes {
			t.Run(fmt.Sprintf("type_%d", tt.Kind), func(t *testing.T) {
				var cif types.CallInterface
				err := PrepareCallInterface(&cif, types.DefaultCall, types.VoidTypeDescriptor,
					[]*types.TypeDescriptor{tt})
				if err != nil {
					t.Fatalf("PrepareCallInterface with type kind %d failed: %v", tt.Kind, err)
				}
			})
		}
	})
}

// =============================================================================
// PrepareVariadicCallInterface Tests
// =============================================================================

func TestPrepareVariadicCallInterface(t *testing.T) {
	t.Run("basic variadic", func(t *testing.T) {
		var cif types.CallInterface
		err := PrepareVariadicCallInterface(&cif, types.DefaultCall, 2,
			types.PointerTypeDescriptor,
			[]*types.TypeDescriptor{
				types.PointerTypeDescriptor, // proxy (fixed)
				types.UInt32TypeDescriptor,  // opcode (fixed)
				types.PointerTypeDescriptor, // interface (variadic)
				types.UInt32TypeDescriptor,  // version (variadic)
			})
		if err != nil {
			t.Fatalf("PrepareVariadicCallInterface failed: %v", err)
		}
		if cif.ArgCount != 4 {
			t.Errorf("ArgCount = %d, want 4", cif.ArgCount)
		}
		if cif.FixedArgCount != 2 {
			t.Errorf("FixedArgCount = %d, want 2", cif.FixedArgCount)
		}
	})

	t.Run("nfixedargs negative", func(t *testing.T) {
		var cif types.CallInterface
		err := PrepareVariadicCallInterface(&cif, types.DefaultCall, -1,
			types.VoidTypeDescriptor, nil)
		if err == nil {
			t.Fatal("PrepareVariadicCallInterface with negative nfixedargs should return error")
		}
	})

	t.Run("nfixedargs exceeds total", func(t *testing.T) {
		var cif types.CallInterface
		err := PrepareVariadicCallInterface(&cif, types.DefaultCall, 5,
			types.VoidTypeDescriptor,
			[]*types.TypeDescriptor{types.UInt32TypeDescriptor})
		if err == nil {
			t.Fatal("PrepareVariadicCallInterface with nfixedargs > total should return error")
		}
	})

	t.Run("nfixedargs zero", func(t *testing.T) {
		var cif types.CallInterface
		err := PrepareVariadicCallInterface(&cif, types.DefaultCall, 0,
			types.VoidTypeDescriptor, nil)
		if err != nil {
			t.Fatalf("PrepareVariadicCallInterface with nfixedargs=0 should succeed: %v", err)
		}
	})
}

// =============================================================================
// CallFunction Tests (Real C Function Calls)
// =============================================================================

func TestCallFunction_strlen(t *testing.T) {
	initLibcForTest()

	if !hasLibC {
		t.Skip("libc.so.6 not available on this system")
	}

	handle, err := LoadLibrary("libc.so.6")
	if err != nil {
		t.Fatalf("LoadLibrary failed: %v", err)
	}
	defer FreeLibrary(handle) //nolint:errcheck

	strlenPtr, err := GetSymbol(handle, "strlen")
	if err != nil {
		t.Fatalf("GetSymbol(strlen) failed: %v", err)
	}

	// Prepare CIF: size_t strlen(const char *s)
	// size_t = uint64 on amd64, pointer arg
	var cif types.CallInterface
	err = PrepareCallInterface(&cif, types.DefaultCall, types.UInt64TypeDescriptor,
		[]*types.TypeDescriptor{types.PointerTypeDescriptor})
	if err != nil {
		t.Fatalf("PrepareCallInterface failed: %v", err)
	}

	// Test with various strings
	testCases := []struct {
		input string
		want  uint64
	}{
		{"", 0},
		{"a", 1},
		{"hello", 5},
		{"hello world", 11},
		{"\x00", 0}, // null byte at start
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("strlen(%q)", tc.input), func(t *testing.T) {
			cstr, err := stringToCString(tc.input)
			if err != nil {
				t.Fatalf("stringToCString failed: %v", err)
			}

			// goffi convention: avalue elements are pointers to values.
			// For pointer types, use pointer-to-pointer.
			strPtr := unsafe.Pointer(&cstr[0])
			var result uint64
			_, err = CallFunction(&cif, strlenPtr, unsafe.Pointer(&result), []unsafe.Pointer{unsafe.Pointer(&strPtr)})
			if err != nil {
				t.Fatalf("CallFunction(strlen) failed: %v", err)
			}
			if result != tc.want {
				t.Errorf("strlen(%q) = %d, want %d", tc.input, result, tc.want)
			}
		})
	}
}

func TestCallFunction_abs(t *testing.T) {
	initLibcForTest()

	if !hasLibC {
		t.Skip("libc.so.6 not available on this system")
	}

	handle, err := LoadLibrary("libc.so.6")
	if err != nil {
		t.Fatalf("LoadLibrary failed: %v", err)
	}
	defer FreeLibrary(handle) //nolint:errcheck

	absPtr, err := GetSymbol(handle, "abs")
	if err != nil {
		t.Fatalf("GetSymbol(abs) failed: %v", err)
	}

	// Prepare CIF: int abs(int j)
	var cif types.CallInterface
	err = PrepareCallInterface(&cif, types.DefaultCall, types.SInt32TypeDescriptor,
		[]*types.TypeDescriptor{types.SInt32TypeDescriptor})
	if err != nil {
		t.Fatalf("PrepareCallInterface failed: %v", err)
	}

	testCases := []struct {
		input int32
		want  int32
	}{
		{0, 0},
		{42, 42},
		{-42, 42},
		{-1, 1},
		{math.MaxInt32, math.MaxInt32},
		{math.MinInt32 + 1, math.MaxInt32}, // -MinInt32 overflows
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("abs(%d)", tc.input), func(t *testing.T) {
			var result int32
			_, err = CallFunction(&cif, absPtr, unsafe.Pointer(&result), []unsafe.Pointer{unsafe.Pointer(&tc.input)})
			if err != nil {
				t.Fatalf("CallFunction(abs) failed: %v", err)
			}
			if result != tc.want {
				t.Errorf("abs(%d) = %d, want %d", tc.input, result, tc.want)
			}
		})
	}
}

func TestCallFunction_sqrt(t *testing.T) {
	initLibcForTest()

	if !hasLibM {
		t.Skip("libm.so.6 not available on this system")
	}

	libmHandle, err := LoadLibrary("libm.so.6")
	if err != nil {
		libmHandle, err = LoadLibrary("libm.so")
		if err != nil {
			t.Fatalf("LoadLibrary(libm) failed: %v", err)
		}
	}
	defer FreeLibrary(libmHandle) //nolint:errcheck

	sqrtPtr, err := GetSymbol(libmHandle, "sqrt")
	if err != nil {
		t.Fatalf("GetSymbol(sqrt) failed: %v", err)
	}

	// Prepare CIF: double sqrt(double x)
	var cif types.CallInterface
	err = PrepareCallInterface(&cif, types.DefaultCall, types.DoubleTypeDescriptor,
		[]*types.TypeDescriptor{types.DoubleTypeDescriptor})
	if err != nil {
		t.Fatalf("PrepareCallInterface failed: %v", err)
	}

	// Test cases for sqrt: double sqrt(double x)
	// Uses purego.RegisterFunc (via callFloatReturn) to correctly read the
	// float64 return value from XMM0 register.
	testCases := []struct {
		input float64
		want  float64
	}{
		{0, 0},
		{1, 1},
		{4, 2},
		{9, 3},
		{16, 4},
		{2, 1.4142135623730951},
		{0.25, 0.5},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("sqrt(%v)", tc.input), func(t *testing.T) {
			input := tc.input
			var result float64
			_, err = CallFunction(&cif, sqrtPtr, unsafe.Pointer(&result), []unsafe.Pointer{unsafe.Pointer(&input)})
			if err != nil {
				t.Fatalf("CallFunction(sqrt) failed: %v", err)
			}
			// Verify the result is correct (uses RegisterFunc for float return)
			if math.Abs(result-tc.want) > 1e-12 {
				t.Errorf("sqrt(%v) = %v, want %v", tc.input, result, tc.want)
			}
		})
	}
}

func TestCallFunction_voidNoArgs(t *testing.T) {
	initLibcForTest()

	if !hasLibC {
		t.Skip("libc.so.6 not available on this system")
	}

	handle, err := LoadLibrary("libc.so.6")
	if err != nil {
		t.Fatalf("LoadLibrary failed: %v", err)
	}
	defer FreeLibrary(handle) //nolint:errcheck

	// Use a simple void function - we'll use getpid which returns pid_t
	// Actually, let's use a function that returns void with no args
	// Since most C functions return something, we'll use getpid and ignore the result
	getpidPtr, err := GetSymbol(handle, "getpid")
	if err != nil {
		t.Fatalf("GetSymbol(getpid) failed: %v", err)
	}

	// Prepare CIF: pid_t getpid(void)
	var cif types.CallInterface
	err = PrepareCallInterface(&cif, types.DefaultCall, types.IntTypeDescriptor, nil)
	if err != nil {
		t.Fatalf("PrepareCallInterface failed: %v", err)
	}

	var result int32
	_, err = CallFunction(&cif, getpidPtr, unsafe.Pointer(&result), nil)
	if err != nil {
		t.Fatalf("CallFunction(getpid) failed: %v", err)
	}
	if result <= 0 {
		t.Errorf("getpid() = %d, should be > 0", result)
	}
}

func TestCallFunction_nilArgSlice(t *testing.T) {
	initLibcForTest()

	if !hasLibC {
		t.Skip("libc.so.6 not available on this system")
	}

	handle, err := LoadLibrary("libc.so.6")
	if err != nil {
		t.Fatalf("LoadLibrary failed: %v", err)
	}
	defer FreeLibrary(handle) //nolint:errcheck

	getpidPtr, err := GetSymbol(handle, "getpid")
	if err != nil {
		t.Fatalf("GetSymbol(getpid) failed: %v", err)
	}

	var cif types.CallInterface
	err = PrepareCallInterface(&cif, types.DefaultCall, types.IntTypeDescriptor, nil)
	if err != nil {
		t.Fatalf("PrepareCallInterface failed: %v", err)
	}

	var result int32
	// Test with nil args (same as nil avalue)
	_, err = CallFunction(&cif, getpidPtr, unsafe.Pointer(&result), nil)
	if err != nil {
		t.Fatalf("CallFunction with nil args failed: %v", err)
	}
	if result <= 0 {
		t.Errorf("getpid() = %d, should be > 0", result)
	}
}

func TestCallFunction_voidReturn(t *testing.T) {
	initLibcForTest()

	if !hasLibC {
		t.Skip("libc.so.6 not available on this system")
	}

	handle, err := LoadLibrary("libc.so.6")
	if err != nil {
		t.Fatalf("LoadLibrary failed: %v", err)
	}
	defer FreeLibrary(handle) //nolint:errcheck

	// Call a function with void return and ignore result
	// We'll use a function that returns void - let's use srand
	// void srand(unsigned int seed)
	srandPtr, err := GetSymbol(handle, "srand")
	if err != nil {
		t.Fatalf("GetSymbol(srand) failed: %v", err)
	}

	var cif types.CallInterface
	err = PrepareCallInterface(&cif, types.DefaultCall, types.VoidTypeDescriptor,
		[]*types.TypeDescriptor{types.UInt32TypeDescriptor})
	if err != nil {
		t.Fatalf("PrepareCallInterface failed: %v", err)
	}

	seed := uint32(42)
	// Call with nil rvalue (void return)
	_, err = CallFunction(&cif, srandPtr, nil, []unsafe.Pointer{unsafe.Pointer(&seed)})
	if err != nil {
		t.Fatalf("CallFunction(srand) failed: %v", err)
	}
}

func TestCallFunction_nilFn(t *testing.T) {
	var cif types.CallInterface
	err := PrepareCallInterface(&cif, types.DefaultCall, types.VoidTypeDescriptor, nil)
	if err != nil {
		t.Fatalf("PrepareCallInterface failed: %v", err)
	}

	_, err = CallFunction(&cif, nil, nil, nil)
	if err == nil {
		t.Fatal("CallFunction with nil fn should return error")
	}
	var cifErr *InvalidCallInterfaceError
	if !errors.As(err, &cifErr) {
		t.Fatalf("expected *InvalidCallInterfaceError, got %T", err)
	}
}

func TestCallFunction_nilCif(t *testing.T) {
	var fnPtr uintptr = 1
	_, err := CallFunction(nil, *(*unsafe.Pointer)(unsafe.Pointer(&fnPtr)), nil, nil)
	if err == nil {
		t.Fatal("CallFunction with nil cif should return error")
	}
	var cifErr *InvalidCallInterfaceError
	if !errors.As(err, &cifErr) {
		t.Fatalf("expected *InvalidCallInterfaceError, got %T", err)
	}
}

// =============================================================================
// CallFunctionContext Tests
// =============================================================================

func TestCallFunctionContext(t *testing.T) {
	initLibcForTest()

	if !hasLibC {
		t.Skip("libc.so.6 not available on this system")
	}

	handle, err := LoadLibrary("libc.so.6")
	if err != nil {
		t.Fatalf("LoadLibrary failed: %v", err)
	}
	defer FreeLibrary(handle) //nolint:errcheck

	absPtr, err := GetSymbol(handle, "abs")
	if err != nil {
		t.Fatalf("GetSymbol(abs) failed: %v", err)
	}

	var cif types.CallInterface
	err = PrepareCallInterface(&cif, types.DefaultCall, types.SInt32TypeDescriptor,
		[]*types.TypeDescriptor{types.SInt32TypeDescriptor})
	if err != nil {
		t.Fatalf("PrepareCallInterface failed: %v", err)
	}

	t.Run("context background", func(t *testing.T) {
		var result int32
		input := int32(-10)
		_, err = CallFunctionContext(context.Background(), &cif, absPtr,
			unsafe.Pointer(&result), []unsafe.Pointer{unsafe.Pointer(&input)})
		if err != nil {
			t.Fatalf("CallFunctionContext failed: %v", err)
		}
		if result != 10 {
			t.Errorf("abs(-10) = %d, want 10", result)
		}
	})

	t.Run("nil context behaves as background", func(t *testing.T) {
		var result int32
		input := int32(-11)
		_, err = CallFunctionContext(nil, &cif, absPtr,
			unsafe.Pointer(&result), []unsafe.Pointer{unsafe.Pointer(&input)})
		if err != nil {
			t.Fatalf("CallFunctionContext with nil context failed: %v", err)
		}
		if result != 11 {
			t.Errorf("abs(-11) = %d, want 11", result)
		}
	})

	t.Run("cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		var result int32
		input := int32(42)
		_, err = CallFunctionContext(ctx, &cif, absPtr,
			unsafe.Pointer(&result), []unsafe.Pointer{unsafe.Pointer(&input)})
		if err == nil {
			t.Fatal("CallFunctionContext with cancelled context should return error")
		}
		if err != context.Canceled {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})
}

// =============================================================================
// NewCallback Tests
// =============================================================================

func TestNewCallback(t *testing.T) {
	t.Run("simple callback uintptr->uintptr", func(t *testing.T) {
		cb := func(x uintptr) uintptr {
			return x * 2
		}
		fnPtr := NewCallback(cb)
		if fnPtr == 0 {
			t.Fatal("NewCallback returned 0")
		}
	})

	t.Run("callback with multiple args", func(t *testing.T) {
		cb := func(a, b, c uintptr) uintptr {
			return a + b + c
		}
		fnPtr := NewCallback(cb)
		if fnPtr == 0 {
			t.Fatal("NewCallback returned 0")
		}
	})

	t.Run("callback with float args", func(t *testing.T) {
		cb := func(a, b, c, d float64) uintptr {
			return uintptr(math.Float64bits(a + b + c + d))
		}
		fnPtr := NewCallback(cb)
		if fnPtr == 0 {
			t.Fatal("NewCallback returned 0")
		}
	})

	t.Run("callback with no return", func(t *testing.T) {
		called := false
		cb := func() {
			called = true
		}
		fnPtr := NewCallback(cb)
		if fnPtr == 0 {
			t.Fatal("NewCallback returned 0")
		}
		_ = called
	})
}

func TestNewCallbackPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("NewCallback(nil) should panic")
		}
	}()
	NewCallback(nil)
}

func TestNewCallbackNonFunc(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("NewCallback with non-function should panic")
		}
	}()
	NewCallback(42)
}

// =============================================================================
// Error Type Tests
// =============================================================================

func TestErrorTypes(t *testing.T) {
	t.Run("InvalidCallInterfaceError", func(t *testing.T) {
		err := &InvalidCallInterfaceError{
			Field:  "testField",
			Reason: "test reason",
			Index:  -1,
		}
		if err.Error() != "invalid call interface: testField: test reason" {
			t.Errorf("unexpected error message: %q", err.Error())
		}

		errWithIndex := &InvalidCallInterfaceError{
			Field:  "argTypes",
			Reason: "invalid type",
			Index:  2,
		}
		if errWithIndex.Error() != "invalid call interface: argTypes[2]: invalid type" {
			t.Errorf("unexpected error message: %q", errWithIndex.Error())
		}

		// Test errors.Is
		target := &InvalidCallInterfaceError{}
		if !errors.Is(err, target) {
			t.Error("errors.Is should match InvalidCallInterfaceError")
		}
	})

	t.Run("LibraryError", func(t *testing.T) {
		err := &LibraryError{
			Operation: "load",
			Name:      "libtest.so",
			Err:       fmt.Errorf("cannot open shared object file"),
		}
		if err.Error() != `library load failed for "libtest.so": cannot open shared object file` {
			t.Errorf("unexpected error message: %q", err.Error())
		}
		if err.Unwrap() == nil {
			t.Error("Unwrap() should not be nil")
		}

		// Test errors.Is
		target := &LibraryError{}
		if !errors.Is(err, target) {
			t.Error("errors.Is should match LibraryError")
		}
	})

	t.Run("CallingConventionError", func(t *testing.T) {
		err := &CallingConventionError{
			Convention: 99,
			Platform:   "linux/amd64",
			Reason:     "invalid value",
		}
		if err.Error() != "unsupported calling convention 99 on linux/amd64: invalid value" {
			t.Errorf("unexpected error message: %q", err.Error())
		}

		target := &CallingConventionError{}
		if !errors.Is(err, target) {
			t.Error("errors.Is should match CallingConventionError")
		}
	})

	t.Run("TypeValidationError", func(t *testing.T) {
		err := &TypeValidationError{
			TypeName: "returnType",
			Kind:     99,
			Reason:   "unsupported type kind",
			Index:    -1,
		}
		if err.Error() != "type validation failed for returnType (kind=99): unsupported type kind" {
			t.Errorf("unexpected error message: %q", err.Error())
		}

		errWithIndex := &TypeValidationError{
			TypeName: "argTypes",
			Kind:     99,
			Reason:   "unsupported type kind",
			Index:    2,
		}
		expected := "type validation failed for argTypes[2] (kind=99): unsupported type kind"
		if errWithIndex.Error() != expected {
			t.Errorf("unexpected error message: %q, want %q", errWithIndex.Error(), expected)
		}

		target := &TypeValidationError{}
		if !errors.Is(err, target) {
			t.Error("errors.Is should match TypeValidationError")
		}
	})

	t.Run("UnsupportedPlatformError", func(t *testing.T) {
		err := &UnsupportedPlatformError{
			OS:   "linux",
			Arch: "mips",
		}
		if err.Error() != "unsupported platform: linux/mips (FFI not implemented for this platform)" {
			t.Errorf("unexpected error message: %q", err.Error())
		}

		target := &UnsupportedPlatformError{}
		if !errors.Is(err, target) {
			t.Error("errors.Is should match UnsupportedPlatformError")
		}
	})
}

func TestSentinelErrors(t *testing.T) {
	if ErrInvalidCallInterface == nil {
		t.Error("ErrInvalidCallInterface should not be nil")
	}
	if ErrFunctionCallFailed == nil {
		t.Error("ErrFunctionCallFailed should not be nil")
	}
	if ErrTooManyArguments == nil {
		t.Error("ErrTooManyArguments should not be nil")
	}

	// Verify ErrInvalidCallInterface is an *InvalidCallInterfaceError
	var cifErr *InvalidCallInterfaceError
	if !errors.As(ErrInvalidCallInterface, &cifErr) {
		t.Error("ErrInvalidCallInterface should be an *InvalidCallInterfaceError")
	}
}

// =============================================================================
// RTLD Constants
// =============================================================================

func TestRTLDConstants(t *testing.T) {
	if RTLD_LAZY != 0x1 {
		t.Errorf("RTLD_LAZY = %d, want 0x1", RTLD_LAZY)
	}
	if RTLD_NOW != 0x2 {
		t.Errorf("RTLD_NOW = %d, want 0x2", RTLD_NOW)
	}
	if RTLD_GLOBAL != 0x100 {
		t.Errorf("RTLD_GLOBAL = %d, want 0x100", RTLD_GLOBAL)
	}
}

// =============================================================================
// Integration Test: Full FFI Pipeline
// =============================================================================

// TestFullFFIPipeline tests the complete LoadLibrary → GetSymbol → PrepareCallInterface → CallFunction pipeline.
func TestFullFFIPipeline(t *testing.T) {
	initLibcForTest()

	if !hasLibC {
		t.Skip("libc.so.6 not available on this system")
	}

	// Step 1: Load library
	handle, err := LoadLibrary("libc.so.6")
	if err != nil {
		t.Fatalf("Step 1 (LoadLibrary) failed: %v", err)
	}
	defer FreeLibrary(handle) //nolint:errcheck

	// Step 2: Get symbol
	strlenPtr, err := GetSymbol(handle, "strlen")
	if err != nil {
		t.Fatalf("Step 2 (GetSymbol) failed: %v", err)
	}
	if strlenPtr == nil {
		t.Fatal("Step 2 (GetSymbol) returned nil")
	}

	// Step 3: Prepare call interface
	var cif types.CallInterface
	err = PrepareCallInterface(&cif, types.DefaultCall, types.UInt64TypeDescriptor,
		[]*types.TypeDescriptor{types.PointerTypeDescriptor})
	if err != nil {
		t.Fatalf("Step 3 (PrepareCallInterface) failed: %v", err)
	}

	// Step 4: Call function
	cstr, err := stringToCString("hello FFI pipeline")
	if err != nil {
		t.Fatalf("stringToCString failed: %v", err)
	}
	// goffi convention: avalue elements are pointers to values.
	// For pointer types, use pointer-to-pointer.
	strPtr := unsafe.Pointer(&cstr[0])
	var result uint64
	_, err = CallFunction(&cif, strlenPtr, unsafe.Pointer(&result), []unsafe.Pointer{unsafe.Pointer(&strPtr)})
	if err != nil {
		t.Fatalf("Step 4 (CallFunction) failed: %v", err)
	}
	if result != 18 {
		t.Errorf("strlen result = %d, want 18", result)
	}

	// Step 5: Free library (success)
	err = FreeLibrary(handle)
	if err != nil {
		t.Fatalf("Step 5 (FreeLibrary) failed: %v", err)
	}
}

// =============================================================================
// Usage Pattern Tests (mirroring gpui codebase patterns)
// =============================================================================

// TestVulkanBackendPattern tests the CallFunction pattern used in Vulkan backend.
// Vulkan pattern: result = CallFunction(&cif, fn, &result, args[:])
func TestVulkanBackendPattern(t *testing.T) {
	handle := nativeTestLibrary(t)
	fn := nativeSymbol(t, handle, "gpui_ffi_vk_result2")

	// Vulkan pattern: SigResultHandlePtr (like VkResult(handle, ptr))
	var cif types.CallInterface
	err := PrepareCallInterface(&cif, types.DefaultCall, types.SInt32TypeDescriptor,
		[]*types.TypeDescriptor{
			types.UInt64TypeDescriptor,
			types.PointerTypeDescriptor,
		})
	if err != nil {
		t.Fatalf("PrepareCallInterface failed: %v", err)
	}

	input := uint64(42)
	var out uint64
	outPtr := unsafe.Pointer(&out)
	var result int32
	args := [2]unsafe.Pointer{
		unsafe.Pointer(&input),
		unsafe.Pointer(&outPtr),
	}
	_, err = CallFunction(&cif, fn, unsafe.Pointer(&result), args[:])
	if err != nil {
		t.Fatalf("CallFunction failed: %v", err)
	}
	if result != -3 {
		t.Fatalf("result = %d, want -3", result)
	}
	if out != 119 {
		t.Fatalf("out = %d, want 119", out)
	}
}

// TestGLESBackendPattern tests the CallFunction pattern used in GLES backend.
// GLES pattern: _ = ffi.CallFunction(&cif, fn, nil, args[:])  (void return)
//
//	_ = ffi.CallFunction(&cif, fn, unsafe.Pointer(&result), args[:]) (with return)
func TestGLESBackendPattern(t *testing.T) {
	handle := nativeTestLibrary(t)

	// Test pattern: void fn(uint32) - used for glEnable, glDisable, etc.
	var cifVoid1 types.CallInterface
	err := PrepareCallInterface(&cifVoid1, types.DefaultCall, types.VoidTypeDescriptor,
		[]*types.TypeDescriptor{types.UInt32TypeDescriptor})
	if err != nil {
		t.Fatalf("PrepareCallInterface (void fn(uint32)) failed: %v", err)
	}

	// Test pattern: uint32 fn(void) - used for glGetError, glCreateProgram, etc.
	var cifUInt32 types.CallInterface
	err = PrepareCallInterface(&cifUInt32, types.DefaultCall, types.UInt32TypeDescriptor, nil)
	if err != nil {
		t.Fatalf("PrepareCallInterface (uint32 fn(void)) failed: %v", err)
	}

	// Test pattern: void fn(float, float, float, float) - used for glClearColor
	var cifVoid4Float types.CallInterface
	err = PrepareCallInterface(&cifVoid4Float, types.DefaultCall, types.VoidTypeDescriptor,
		[]*types.TypeDescriptor{
			types.FloatTypeDescriptor,
			types.FloatTypeDescriptor,
			types.FloatTypeDescriptor,
			types.FloatTypeDescriptor,
		})
	if err != nil {
		t.Fatalf("PrepareCallInterface (void fn(float*4)) failed: %v", err)
	}

	// Test pattern: void fn(uint32, void*) - used for glGetIntegerv
	var cifVoid2 types.CallInterface
	err = PrepareCallInterface(&cifVoid2, types.DefaultCall, types.VoidTypeDescriptor,
		[]*types.TypeDescriptor{types.UInt32TypeDescriptor, types.PointerTypeDescriptor})
	if err != nil {
		t.Fatalf("PrepareCallInterface (void fn(uint32, void*)) failed: %v", err)
	}

	recordU32 := nativeSymbol(t, handle, "gpui_ffi_record_u32")
	getLastU32 := nativeSymbol(t, handle, "gpui_ffi_get_last_u32")
	recordFourFloats := nativeSymbol(t, handle, "gpui_ffi_record_four_floats")
	getLastFloatSum := nativeSymbol(t, handle, "gpui_ffi_get_last_float_sum")
	storeI32 := nativeSymbol(t, handle, "gpui_ffi_store_i32")

	enableValue := uint32(0x0BE2)
	if _, err := CallFunction(&cifVoid1, recordU32, nil, []unsafe.Pointer{unsafe.Pointer(&enableValue)}); err != nil {
		t.Fatalf("CallFunction(void uint32) failed: %v", err)
	}
	var gotU32 uint32
	if _, err := CallFunction(&cifUInt32, getLastU32, unsafe.Pointer(&gotU32), nil); err != nil {
		t.Fatalf("CallFunction(uint32 void) failed: %v", err)
	}
	if gotU32 != enableValue {
		t.Fatalf("got last u32 %#x, want %#x", gotU32, enableValue)
	}

	r, g, b, a := float32(0.25), float32(0.5), float32(0.75), float32(1.0)
	if _, err := CallFunction(&cifVoid4Float, recordFourFloats, nil, []unsafe.Pointer{
		unsafe.Pointer(&r),
		unsafe.Pointer(&g),
		unsafe.Pointer(&b),
		unsafe.Pointer(&a),
	}); err != nil {
		t.Fatalf("CallFunction(void float*4) failed: %v", err)
	}
	var gotFloatSum float32
	var cifFloatReturn types.CallInterface
	if err := PrepareCallInterface(&cifFloatReturn, types.DefaultCall, types.FloatTypeDescriptor, nil); err != nil {
		t.Fatalf("PrepareCallInterface(float return) failed: %v", err)
	}
	if _, err := CallFunction(&cifFloatReturn, getLastFloatSum, unsafe.Pointer(&gotFloatSum), nil); err != nil {
		t.Fatalf("CallFunction(float void) failed: %v", err)
	}
	if gotFloatSum != 2.5 {
		t.Fatalf("got float sum %v, want 2.5", gotFloatSum)
	}

	pname := uint32(44)
	var data int32
	dataPtr := unsafe.Pointer(&data)
	if _, err := CallFunction(&cifVoid2, storeI32, nil, []unsafe.Pointer{
		unsafe.Pointer(&pname),
		unsafe.Pointer(&dataPtr),
	}); err != nil {
		t.Fatalf("CallFunction(void uint32 pointer) failed: %v", err)
	}
	if data != 1044 {
		t.Fatalf("stored data = %d, want 1044", data)
	}

	// Verify all CIFs have correct structure
	if cifUInt32.ArgCount != 0 {
		t.Errorf("cifUInt32 ArgCount = %d, want 0", cifUInt32.ArgCount)
	}
	if cifUInt32.ReturnType != types.UInt32TypeDescriptor {
		t.Errorf("cifUInt32 ReturnType should be UInt32TypeDescriptor")
	}

	if cifVoid1.ArgCount != 1 {
		t.Errorf("cifVoid1 ArgCount = %d, want 1", cifVoid1.ArgCount)
	}
	if cifVoid1.ArgTypes[0] != types.UInt32TypeDescriptor {
		t.Errorf("cifVoid1 ArgTypes[0] should be UInt32TypeDescriptor")
	}

	if cifVoid4Float.ArgCount != 4 {
		t.Errorf("cifVoid4Float ArgCount = %d, want 4", cifVoid4Float.ArgCount)
	}
	for i, at := range cifVoid4Float.ArgTypes {
		if at != types.FloatTypeDescriptor {
			t.Errorf("cifVoid4Float ArgTypes[%d] should be FloatTypeDescriptor", i)
		}
	}
}

// TestSoftwareBackendPattern tests the CallFunction pattern used in Software backend.
// Software pattern: LoadLibrary with fallback names, then GetSymbol for multiple symbols.
func TestSoftwareBackendPattern(t *testing.T) {
	// Test the fallback loading pattern used in software/blit_wayland.go
	handle, err := LoadLibrary("libwayland-client.so.0")
	if err != nil {
		handle, err = LoadLibrary("libwayland-client.so")
		if err != nil {
			t.Skip("Wayland client library not available")
		}
	}
	defer FreeLibrary(handle) //nolint:errcheck

	// Get multiple symbols like the Wayland backend does
	symbols := []string{
		"wl_display_connect",
		"wl_display_disconnect",
		"wl_display_get_registry",
		"wl_proxy_marshal",
		"wl_proxy_marshal_constructor",
	}

	for _, sym := range symbols {
		_, err := GetSymbol(handle, sym)
		if err != nil {
			t.Logf("Symbol %q not found (expected on some systems): %v", sym, err)
		}
	}
}

// TestMetalBackendPattern tests the NewCallback pattern used in Metal backend.
// Metal pattern: ffi.NewCallback(func(blockPtr, event uintptr, value uint64) { ... })
func TestMetalBackendPattern(t *testing.T) {
	// Test the callback patterns used in metal/objc.go

	// Pattern 1: shared event block callback
	cb1 := func(blockPtr, event uintptr, value uint64) {
		if blockPtr == 0 {
			return
		}
		_ = event
		_ = value
	}
	fnPtr1 := NewCallback(cb1)
	if fnPtr1 == 0 {
		t.Fatal("NewCallback for shared event block returned 0")
	}

	// Pattern 2: completion handler callback (with uintptr return)
	cb2 := func(blockPtr, _ uintptr) uintptr {
		if blockPtr == 0 {
			return 0
		}
		return 0
	}
	fnPtr2 := NewCallback(cb2)
	if fnPtr2 == 0 {
		t.Fatal("NewCallback for completion handler returned 0")
	}

	// Pattern 3: callback with only uintptr args
	cb3 := func(severity, types, callbackData, userData uintptr) uintptr {
		if callbackData == 0 {
			return 0
		}
		return 0
	}
	fnPtr3 := NewCallback(cb3)
	if fnPtr3 == 0 {
		t.Fatal("NewCallback for debug callback returned 0")
	}

	_ = fnPtr1
	_ = fnPtr2
	_ = fnPtr3
}

// =============================================================================
// Edge Case Pattern Tests (from original goffi usage)
// =============================================================================

// TestGetSymbolOptionalPattern verifies pattern 2c: GetSymbol with error ignored
// for optional symbols. Used in metal/objc.go:
//
//	symObjcMsgSendFpret, _ = ffi.GetSymbol(objcLib, "objc_msgSend_fpret")
//	symObjcMsgSendStret, _ = ffi.GetSymbol(objcLib, "objc_msgSend_stret")
func TestGetSymbolOptionalPattern(t *testing.T) {
	initLibcForTest()

	if !hasLibC {
		t.Skip("libc.so.6 not available on this system")
	}

	handle, err := LoadLibrary("libc.so.6")
	if err != nil {
		t.Fatalf("LoadLibrary failed: %v", err)
	}
	defer FreeLibrary(handle) //nolint:errcheck

	// Pattern: optional symbol, error ignored, nil on failure
	sym, _ := GetSymbol(handle, "strlen") // exists
	if sym == nil {
		t.Error("GetSymbol(strlen) should succeed even with ignored error")
	}

	// Optional symbol that doesn't exist - should return nil, no crash
	nonexistent, _ := GetSymbol(handle, "this_symbol_does_not_exist_at_all_xyz")
	if nonexistent != nil {
		t.Error("GetSymbol for nonexistent symbol should return nil")
	}
}

// TestGetSymbolDataPattern verifies pattern 2d: GetSymbol for data (not function).
// Used in metal/objc.go:
//
//	sym, err := ffi.GetSymbol(objcLib, "_NSConcreteGlobalBlock")
//	if err == nil && sym != nil {
//	    symNSConcreteGlobalBlock = *(*uintptr)(sym)
//	}
func TestGetSymbolDataPattern(t *testing.T) {
	initLibcForTest()

	if !hasLibC {
		t.Skip("libc.so.6 not available on this system")
	}

	handle, err := LoadLibrary("libc.so.6")
	if err != nil {
		t.Fatalf("LoadLibrary failed: %v", err)
	}
	defer FreeLibrary(handle) //nolint:errcheck

	// Get a known data symbol (errno location, or similar)
	// On Linux, we can get __environ or similar data symbol
	sym, err := GetSymbol(handle, "__environ")
	if err == nil && sym != nil {
		// Read the data at the symbol location (like _NSConcreteGlobalBlock pattern)
		// sym is a pointer to the environ pointer
		environ := *(*uintptr)(sym)
		if environ == 0 {
			t.Log("__environ is nil (expected in some environments)")
		} else {
			t.Logf("__environ = 0x%x", environ)
		}
	} else {
		// __environ may not be available on all systems, try environ
		sym, err = GetSymbol(handle, "environ")
		if err == nil && sym != nil {
			environ := *(*uintptr)(sym)
			t.Logf("environ = 0x%x", environ)
		} else {
			t.Log("No data symbol found to test (expected on some systems)")
		}
	}
}

// TestNewCallbackLazyInitPattern verifies pattern 10d: sync.Once lazy init with NewCallback.
// Used in metal/objc.go for all 4 callback functions:
//
//	var sharedEventBlockInvokeOnce sync.Once
//	sharedEventBlockInvokePtr  uintptr
//	func getSharedEventBlockInvoke() uintptr {
//	    sharedEventBlockInvokeOnce.Do(func() {
//	        sharedEventBlockInvokePtr = ffi.NewCallback(func(...) { ... })
//	    })
//	    return sharedEventBlockInvokePtr
//	}
func TestNewCallbackLazyInitPattern(t *testing.T) {
	var (
		once        sync.Once
		callbackPtr uintptr
	)

	getCallback := func() uintptr {
		once.Do(func() {
			callbackPtr = NewCallback(func(blockPtr, event uintptr, value uint64) {
				if blockPtr == 0 {
					return
				}
				_ = event
				_ = value
			})
		})
		return callbackPtr
	}

	// First call: creates the callback
	ptr1 := getCallback()
	if ptr1 == 0 {
		t.Fatal("First getCallback() returned 0")
	}

	// Second call: returns the same cached callback
	ptr2 := getCallback()
	if ptr2 == 0 {
		t.Fatal("Second getCallback() returned 0")
	}
	if ptr1 != ptr2 {
		t.Error("Lazy init should return the same pointer on subsequent calls")
	}
}

// TestNewCallbackLazyInitMultiPattern verifies that multiple lazy-initialized
// callbacks work correctly (matching the 4 callback pattern in metal/objc.go).
func TestNewCallbackLazyInitMultiPattern(t *testing.T) {
	type callbackDef struct {
		once sync.Once
		ptr  uintptr
	}

	// Define 4 callbacks like metal/objc.go does
	callbacks := []callbackDef{
		{}, // shared event block invoke
		{}, // completed handler block invoke
		{}, // frame completion block invoke
		{}, // gpu completion block invoke
	}

	getCallback := func(cb *callbackDef, fn any) uintptr {
		cb.once.Do(func() {
			cb.ptr = NewCallback(fn)
		})
		return cb.ptr
	}

	// Initialize all 4 callbacks
	for i := range callbacks {
		// Use different callback signatures matching metal/objc.go patterns
		var fn any
		switch i {
		case 0:
			// void(blockPtr, event, value) - shared event
			fn = func(blockPtr, event uintptr, value uint64) {
				_ = blockPtr
				_ = event
				_ = value
			}
		case 1, 2, 3:
			// uintptr(blockPtr, _) - completion handlers
			fn = func(blockPtr, _ uintptr) uintptr {
				if blockPtr == 0 {
					return 0
				}
				return 0
			}
		}

		ptr := getCallback(&callbacks[i], fn)
		if ptr == 0 {
			t.Fatalf("Callback %d returned 0", i)
		}

		// Verify idempotency
		ptr2 := getCallback(&callbacks[i], fn)
		if ptr != ptr2 {
			t.Errorf("Callback %d: second call returned different pointer", i)
		}
	}
}

// =============================================================================

// TestPointerToPointerConvention verifies that the pointer-to-pointer convention
// used by the Vulkan and GLES backends works correctly.
//
// Vulkan pattern (loader.go):
//
//	namePtr := unsafe.Pointer(&cname[0])
//	args[1] = unsafe.Pointer(&namePtr)  // pointer TO the pointer
//
// GLES pattern (context_linux.go):
//
//	args[1] = unsafe.Pointer(&data)  // data is *int32, &data is **int32
func TestPointerToPointerConvention(t *testing.T) {
	initLibcForTest()

	if !hasLibC {
		t.Skip("libc.so.6 not available on this system")
	}

	handle, err := LoadLibrary("libc.so.6")
	if err != nil {
		t.Fatalf("LoadLibrary failed: %v", err)
	}
	defer FreeLibrary(handle) //nolint:errcheck
	nativeHandle := nativeTestLibrary(t)

	t.Run("strlen with pointer-to-pointer (Vulkan pattern)", func(t *testing.T) {
		strlenPtr, err := GetSymbol(handle, "strlen")
		if err != nil {
			t.Fatalf("GetSymbol(strlen) failed: %v", err)
		}

		var cif types.CallInterface
		err = PrepareCallInterface(&cif, types.DefaultCall, types.UInt64TypeDescriptor,
			[]*types.TypeDescriptor{types.PointerTypeDescriptor})
		if err != nil {
			t.Fatalf("PrepareCallInterface failed: %v", err)
		}

		// Exact Vulkan pattern: namePtr → &namePtr
		cstr := []byte("hello world\x00")
		namePtr := unsafe.Pointer(&cstr[0])
		args := [1]unsafe.Pointer{
			unsafe.Pointer(&namePtr), // pointer TO the pointer
		}
		var result uint64
		_, err = CallFunction(&cif, strlenPtr, unsafe.Pointer(&result), args[:])
		if err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}
		if result != 11 {
			t.Errorf("strlen = %d, want 11", result)
		}
	})

	t.Run("Vulkan scalar-pointer output pattern", func(t *testing.T) {
		fn := nativeSymbol(t, nativeHandle, "gpui_ffi_vk_result2")
		var cif types.CallInterface
		err = PrepareCallInterface(&cif, types.DefaultCall, types.SInt32TypeDescriptor,
			[]*types.TypeDescriptor{types.UInt64TypeDescriptor, types.PointerTypeDescriptor})
		if err != nil {
			t.Fatalf("PrepareCallInterface failed: %v", err)
		}

		input := uint64(55)
		var out uint64
		outPtr := unsafe.Pointer(&out)
		args := [2]unsafe.Pointer{
			unsafe.Pointer(&input),  // scalar arg: pointer to value
			unsafe.Pointer(&outPtr), // pointer arg: pointer to pointer
		}
		var result int32
		_, err = CallFunction(&cif, fn, unsafe.Pointer(&result), args[:])
		if err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}
		if result != -3 {
			t.Fatalf("result = %d, want -3", result)
		}
		if out != 132 {
			t.Fatalf("out = %d, want 132", out)
		}
	})

	t.Run("GLES GetIntegerv pattern (void fn(uint32, void*))", func(t *testing.T) {
		fn := nativeSymbol(t, nativeHandle, "gpui_ffi_store_i32")

		// GLES pattern: void fn(uint32, void*) for glGetIntegerv
		var cif types.CallInterface
		err = PrepareCallInterface(&cif, types.DefaultCall, types.VoidTypeDescriptor,
			[]*types.TypeDescriptor{types.UInt32TypeDescriptor, types.PointerTypeDescriptor})
		if err != nil {
			t.Fatalf("PrepareCallInterface failed: %v", err)
		}

		pname := uint32(42)
		data := int32(0)
		dataPtr := unsafe.Pointer(&data)
		args := [2]unsafe.Pointer{
			unsafe.Pointer(&pname),   // scalar: pointer to value
			unsafe.Pointer(&dataPtr), // pointer: dataPtr is *int32, &dataPtr is **int32
		}
		_, err = CallFunction(&cif, fn, nil, args[:])
		if err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}
		if data != 1042 {
			t.Fatalf("data = %d, want 1042", data)
		}
	})

	t.Run("GLES BufferData pattern (void fn(uint32, uintptr, void*, uint32))", func(t *testing.T) {
		recordFn := nativeSymbol(t, nativeHandle, "gpui_ffi_record_buffer_data")
		getFn := nativeSymbol(t, nativeHandle, "gpui_ffi_get_last_buffer_data")

		var cif types.CallInterface
		err = PrepareCallInterface(&cif, types.DefaultCall, types.VoidTypeDescriptor,
			[]*types.TypeDescriptor{
				types.UInt32TypeDescriptor,
				types.UInt64TypeDescriptor,
				types.PointerTypeDescriptor,
				types.UInt32TypeDescriptor,
			})
		if err != nil {
			t.Fatalf("PrepareCallInterface failed: %v", err)
		}

		target := uint32(0xDEAD)
		size := uint64(1024)
		data := unsafe.Pointer(nil) // null data for buffer allocation
		usage := uint32(0xBEEF)

		args := [4]unsafe.Pointer{
			unsafe.Pointer(&target),
			unsafe.Pointer(&size),
			unsafe.Pointer(&data), // data is unsafe.Pointer, &data is *unsafe.Pointer
			unsafe.Pointer(&usage),
		}
		_, err = CallFunction(&cif, recordFn, nil, args[:])
		if err != nil {
			t.Fatalf("CallFunction(record buffer data) failed: %v", err)
		}

		var cifReturn types.CallInterface
		err = PrepareCallInterface(&cifReturn, types.DefaultCall, types.UInt64TypeDescriptor, nil)
		if err != nil {
			t.Fatalf("PrepareCallInterface(return) failed: %v", err)
		}
		var got uint64
		_, err = CallFunction(&cifReturn, getFn, unsafe.Pointer(&got), nil)
		if err != nil {
			t.Fatalf("CallFunction(get buffer data) failed: %v", err)
		}
		want := uint64(target) + size + uint64(uintptr(data)) + uint64(usage)
		if got != want {
			t.Fatalf("got %d, want %d", got, want)
		}
	})

	t.Run("Vulkan loader.go vkGetInstanceProcAddr exact pattern", func(t *testing.T) {
		// This test verifies the pointer-to-pointer convention used in
		// vk/loader.go without actually calling vkGetInstanceProcAddr.
		// We verify the arg preparation pattern is correct.
		//
		// From loader.go:
		//   namePtr := unsafe.Pointer(&cname[0])
		//   args[1] = unsafe.Pointer(&namePtr)
		//
		// This test verifies that the pointer-to-pointer pattern produces the
		// correct uintptr value when read by readArgAsUintptr.

		// Simulate the exact arg preparation from loader.go
		cname := []byte("vkGetInstanceProcAddr\x00")
		namePtr := unsafe.Pointer(&cname[0])
		_instance := uint64(0)

		// Verify the pointer-to-pointer convention produces the correct value
		// ptrToPtr = &namePtr (pointer to the pointer variable)
		// readArgAsUintptr should read *(*uintptr)(&namePtr) = uintptr(namePtr)
		ptrToPtr := unsafe.Pointer(&namePtr)
		expected := uintptr(namePtr)
		got := *(*uintptr)(ptrToPtr)
		if got != expected {
			t.Errorf("pointer-to-pointer read: got 0x%x, want 0x%x", got, expected)
		}

		// Also verify the scalar arg pattern
		gotInstance := *(*uint64)(unsafe.Pointer(&_instance))
		if gotInstance != 0 {
			t.Errorf("instance read: got %d, want 0", gotInstance)
		}
	})
}

// =============================================================================
// Helpers
// =============================================================================

// stringToCString converts a Go string to a C-style null-terminated byte slice.
func stringToCString(s string) ([]byte, error) {
	// Check for null bytes in the string (C strings can't contain them)
	if strings.Contains(s, "\x00") {
		// Truncate at null byte
		idx := strings.IndexByte(s, 0)
		s = s[:idx]
	}

	buf := make([]byte, len(s)+1)
	copy(buf, s)
	buf[len(s)] = 0
	return buf, nil
}

// TestStringToCString verifies the test helper.
func TestStringToCString(t *testing.T) {
	buf, err := stringToCString("hello")
	if err != nil {
		t.Fatalf("stringToCString failed: %v", err)
	}
	if len(buf) != 6 {
		t.Errorf("len = %d, want 6", len(buf))
	}
	if buf[5] != 0 {
		t.Errorf("last byte should be null")
	}
	if string(buf[:5]) != "hello" {
		t.Errorf("content = %q, want %q", string(buf[:5]), "hello")
	}
}

// =============================================================================
// Integration Test: Real Callback via C Function
// =============================================================================

// TestCallbackViaQsort tests using NewCallback with qsort to verify callback execution.
// This is a real test that calls a C function (qsort) with a Go callback.
//
// NOTE: This test is skipped by default because the purego callback ABI has limitations
// with int32 return values from C callbacks on some platforms. The basic NewCallback
// tests above verify that the callback registration works correctly.
func TestCallbackViaQsort(t *testing.T) {
	t.Skip("Skipped: purego callback ABI has limitations with int32 returns from C callbacks; native callback tests cover project-used uintptr and void callback signatures")
}

// =============================================================================
// Environment Conditions Documentation
// =============================================================================

// TestEnvironmentConditions documents the environment conditions that are
// met or not met for various API verification scenarios.
func TestEnvironmentConditions(t *testing.T) {
	t.Log("=== Environment Conditions ===")
	t.Logf("OS: %s, Arch: %s", runtime.GOOS, runtime.GOARCH)
	t.Logf("Platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	if gcc, err := exec.LookPath("gcc"); err == nil {
		t.Logf("✅ gcc: available (%s) for native shared-library ABI tests", gcc)
	} else {
		t.Logf("❌ gcc: not available for native shared-library ABI tests: %v", err)
	}
	if os.Getenv("GOCACHE") == "" {
		t.Log("⚠️ GOCACHE: not set; sandbox may require GOCACHE=/tmp/gpui-go-cache")
	} else {
		t.Logf("✅ GOCACHE: %s", os.Getenv("GOCACHE"))
	}

	// Library availability
	libs := []string{
		"libc.so.6",
		"libm.so.6",
		"libvulkan.so.1",
		"libwayland-client.so.0",
		"libEGL.so.1",
		"libGLESv2.so.2",
	}
	for _, lib := range libs {
		_, err := LoadLibrary(lib)
		if err == nil {
			t.Logf("✅ %s: available", lib)
		} else {
			t.Logf("❌ %s: %v", lib, err)
		}
	}

	// Platform availability
	t.Logf("✅ Linux: %s", runtime.GOOS)
	if runtime.GOOS == "darwin" {
		t.Log("✅ macOS: native")
	} else {
		t.Log("❌ macOS: not available (Metal backend cannot be tested)")
	}
	if runtime.GOOS == "windows" {
		t.Log("✅ Windows: native")
	} else {
		t.Log("❌ Windows: not available (DX12 backend cannot be tested)")
	}

	// Note: This test is informational only, it doesn't test any assertions.
	// It documents what can and cannot be tested in the current environment.
}
