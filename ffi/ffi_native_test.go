package ffi

import (
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"unsafe"

	"github.com/energye/gpui/ffi/types"
)

var (
	nativeTestOnce sync.Once
	nativeTestSO   string
	nativeTestErr  error
)

const nativeTestSource = `
#include <stdarg.h>
#include <stddef.h>
#include <stdint.h>

uint8_t gpui_ffi_echo_u8(uint8_t v) { return v; }
int8_t gpui_ffi_echo_s8(int8_t v) { return v; }
uint16_t gpui_ffi_echo_u16(uint16_t v) { return v; }
int16_t gpui_ffi_echo_s16(int16_t v) { return v; }
uint32_t gpui_ffi_echo_u32(uint32_t v) { return v; }
int32_t gpui_ffi_echo_s32(int32_t v) { return v; }
uint64_t gpui_ffi_echo_u64(uint64_t v) { return v; }
int64_t gpui_ffi_echo_s64(int64_t v) { return v; }

void *gpui_ffi_identity_ptr(void *p) { return p; }

static uint32_t gpui_ffi_last_u32;
static float gpui_ffi_last_float_sum;
static uint64_t gpui_ffi_last_buffer_data;

void gpui_ffi_record_u32(uint32_t v) { gpui_ffi_last_u32 = v; }
uint32_t gpui_ffi_get_last_u32(void) { return gpui_ffi_last_u32; }

void gpui_ffi_record_four_floats(float a, float b, float c, float d) {
	gpui_ffi_last_float_sum = a + b + c + d;
}
float gpui_ffi_get_last_float_sum(void) { return gpui_ffi_last_float_sum; }

void gpui_ffi_store_i32(uint32_t pname, int32_t *out) {
	if (out != NULL) {
		*out = (int32_t)(pname + 1000);
	}
}

void gpui_ffi_record_buffer_data(uint32_t target, uintptr_t size, void *data, uint32_t usage) {
	gpui_ffi_last_buffer_data = (uint64_t)target + (uint64_t)size + (uint64_t)(uintptr_t)data + (uint64_t)usage;
}
uint64_t gpui_ffi_get_last_buffer_data(void) { return gpui_ffi_last_buffer_data; }

int32_t gpui_ffi_vk_result2(uint64_t handle, uint64_t *out) {
	if (out != NULL) {
		*out = handle + 77;
	}
	return -3;
}

int32_t gpui_ffi_vk_result(uint64_t handle, uint32_t a, uint32_t b, uint64_t *out) {
	if (out != NULL) {
		*out = handle + a + b;
	}
	return -7;
}

uint64_t gpui_ffi_sum10(uint64_t a, uint32_t b, uint32_t c, uint32_t d, int32_t e,
	uint64_t f, void *p, uint32_t g, void *q, uint64_t h) {
	return a + b + c + d + (uint32_t)e + f + (uintptr_t)p + g + (uintptr_t)q + h;
}

void gpui_ffi_store_floats(float a, float b, float c, float d, float *out) {
	out[0] = a + b;
	out[1] = c + d;
	out[2] = a * d;
}

float gpui_ffi_add_float(float a, float b) { return a + b; }
double gpui_ffi_add_double(double a, double b) { return a + b; }

double gpui_ffi_mixed_fp(uint64_t a, float b, double c, uint32_t d) {
	return (double)a + (double)b + c + (double)d;
}

uint64_t gpui_ffi_sum_variadic(uint32_t n, ...) {
	va_list ap;
	uint64_t sum = n;
	va_start(ap, n);
	for (uint32_t i = 0; i < n; i++) {
		sum += va_arg(ap, unsigned int);
	}
	va_end(ap);
	return sum;
}

uintptr_t gpui_ffi_call_cb_uintptr(uintptr_t (*cb)(uintptr_t, uintptr_t), uintptr_t a, uintptr_t b) {
	return cb(a, b);
}

void gpui_ffi_call_cb_void3(void (*cb)(uintptr_t, uintptr_t, uint64_t),
	uintptr_t a, uintptr_t b, uint64_t c) {
	cb(a, b, c);
}

uintptr_t gpui_ffi_call_cb_vulkan(uintptr_t (*cb)(uintptr_t, uintptr_t, uintptr_t, uintptr_t),
	uintptr_t a, uintptr_t b, uintptr_t c, uintptr_t d) {
	return cb(a, b, c, d);
}

struct gpui_ffi_pair_f64 {
	double x;
	double y;
};

struct gpui_ffi_six_f64 {
	double a;
	double b;
	double c;
	double d;
	double e;
	double f;
};

double gpui_ffi_sum_pair_f64(struct gpui_ffi_pair_f64 p) {
	return p.x + p.y;
}

struct gpui_ffi_pair_f64 gpui_ffi_make_pair_f64(double x, double y) {
	struct gpui_ffi_pair_f64 p = {x, y};
	return p;
}

double gpui_ffi_sum_six_f64(struct gpui_ffi_six_f64 v) {
	return v.a + v.b + v.c + v.d + v.e + v.f;
}

struct gpui_ffi_six_f64 gpui_ffi_make_six_f64(double base) {
	struct gpui_ffi_six_f64 v = {base, base + 1.0, base + 2.0, base + 3.0, base + 4.0, base + 5.0};
	return v;
}
`

func nativeTestLibrary(t *testing.T) unsafe.Pointer {
	t.Helper()

	nativeTestOnce.Do(func() {
		if runtime.GOOS != "linux" {
			nativeTestErr = errors.New("native ffi validation currently builds a Linux shared object")
			return
		}
		gcc, err := exec.LookPath("gcc")
		if err != nil {
			nativeTestErr = errors.New("gcc not available for native ffi validation")
			return
		}

		dir, err := os.MkdirTemp("", "gpui-ffi-native-*")
		if err != nil {
			nativeTestErr = err
			return
		}
		src := filepath.Join(dir, "gpui_ffi_native.c")
		so := filepath.Join(dir, "libgpui_ffi_native.so")
		if err := os.WriteFile(src, []byte(nativeTestSource), 0o600); err != nil {
			nativeTestErr = err
			return
		}
		cmd := exec.Command(gcc, "-shared", "-fPIC", "-O2", "-o", so, src)
		if output, err := cmd.CombinedOutput(); err != nil {
			nativeTestErr = errors.New(string(output))
			return
		}
		nativeTestSO = so
	})

	if nativeTestErr != nil {
		t.Skipf("native ffi validation unavailable: %v", nativeTestErr)
	}
	handle, err := LoadLibrary(nativeTestSO)
	if err != nil {
		t.Fatalf("LoadLibrary(%s) failed: %v", nativeTestSO, err)
	}
	t.Cleanup(func() {
		if err := FreeLibrary(handle); err != nil {
			t.Fatalf("FreeLibrary(native test library) failed: %v", err)
		}
	})
	return handle
}

func nativeSymbol(t *testing.T, handle unsafe.Pointer, name string) unsafe.Pointer {
	t.Helper()
	sym, err := GetSymbol(handle, name)
	if err != nil {
		t.Fatalf("GetSymbol(%s) failed: %v", name, err)
	}
	return sym
}

func prepareNativeCIF(t *testing.T, ret *types.TypeDescriptor, args ...*types.TypeDescriptor) types.CallInterface {
	t.Helper()
	var cif types.CallInterface
	if err := PrepareCallInterface(&cif, types.DefaultCall, ret, args); err != nil {
		t.Fatalf("PrepareCallInterface failed: %v", err)
	}
	return cif
}

func TestNativeScalarDescriptors(t *testing.T) {
	handle := nativeTestLibrary(t)

	t.Run("uint8", func(t *testing.T) {
		fn := nativeSymbol(t, handle, "gpui_ffi_echo_u8")
		cif := prepareNativeCIF(t, types.UInt8TypeDescriptor, types.UInt8TypeDescriptor)
		in := uint8(250)
		var got uint8
		if _, err := CallFunction(&cif, fn, unsafe.Pointer(&got), []unsafe.Pointer{unsafe.Pointer(&in)}); err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}
		if got != in {
			t.Fatalf("got %d, want %d", got, in)
		}
	})

	t.Run("sint8", func(t *testing.T) {
		fn := nativeSymbol(t, handle, "gpui_ffi_echo_s8")
		cif := prepareNativeCIF(t, types.SInt8TypeDescriptor, types.SInt8TypeDescriptor)
		in := int8(-12)
		var got int8
		if _, err := CallFunction(&cif, fn, unsafe.Pointer(&got), []unsafe.Pointer{unsafe.Pointer(&in)}); err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}
		if got != in {
			t.Fatalf("got %d, want %d", got, in)
		}
	})

	t.Run("uint16", func(t *testing.T) {
		fn := nativeSymbol(t, handle, "gpui_ffi_echo_u16")
		cif := prepareNativeCIF(t, types.UInt16TypeDescriptor, types.UInt16TypeDescriptor)
		in := uint16(0xfeed)
		var got uint16
		if _, err := CallFunction(&cif, fn, unsafe.Pointer(&got), []unsafe.Pointer{unsafe.Pointer(&in)}); err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}
		if got != in {
			t.Fatalf("got %#x, want %#x", got, in)
		}
	})

	t.Run("sint16", func(t *testing.T) {
		fn := nativeSymbol(t, handle, "gpui_ffi_echo_s16")
		cif := prepareNativeCIF(t, types.SInt16TypeDescriptor, types.SInt16TypeDescriptor)
		in := int16(-1234)
		var got int16
		if _, err := CallFunction(&cif, fn, unsafe.Pointer(&got), []unsafe.Pointer{unsafe.Pointer(&in)}); err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}
		if got != in {
			t.Fatalf("got %d, want %d", got, in)
		}
	})

	t.Run("uint32", func(t *testing.T) {
		fn := nativeSymbol(t, handle, "gpui_ffi_echo_u32")
		cif := prepareNativeCIF(t, types.UInt32TypeDescriptor, types.UInt32TypeDescriptor)
		in := uint32(0xfedcba98)
		var got uint32
		if _, err := CallFunction(&cif, fn, unsafe.Pointer(&got), []unsafe.Pointer{unsafe.Pointer(&in)}); err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}
		if got != in {
			t.Fatalf("got %#x, want %#x", got, in)
		}
	})

	t.Run("sint32", func(t *testing.T) {
		fn := nativeSymbol(t, handle, "gpui_ffi_echo_s32")
		cif := prepareNativeCIF(t, types.SInt32TypeDescriptor, types.SInt32TypeDescriptor)
		in := int32(-123456)
		var got int32
		if _, err := CallFunction(&cif, fn, unsafe.Pointer(&got), []unsafe.Pointer{unsafe.Pointer(&in)}); err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}
		if got != in {
			t.Fatalf("got %d, want %d", got, in)
		}
	})

	t.Run("uint64", func(t *testing.T) {
		fn := nativeSymbol(t, handle, "gpui_ffi_echo_u64")
		cif := prepareNativeCIF(t, types.UInt64TypeDescriptor, types.UInt64TypeDescriptor)
		in := uint64(0xfeedfacecafebeef)
		var got uint64
		if _, err := CallFunction(&cif, fn, unsafe.Pointer(&got), []unsafe.Pointer{unsafe.Pointer(&in)}); err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}
		if got != in {
			t.Fatalf("got %#x, want %#x", got, in)
		}
	})

	t.Run("sint64", func(t *testing.T) {
		fn := nativeSymbol(t, handle, "gpui_ffi_echo_s64")
		cif := prepareNativeCIF(t, types.SInt64TypeDescriptor, types.SInt64TypeDescriptor)
		in := int64(-0x123456789)
		var got int64
		if _, err := CallFunction(&cif, fn, unsafe.Pointer(&got), []unsafe.Pointer{unsafe.Pointer(&in)}); err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}
		if got != in {
			t.Fatalf("got %d, want %d", got, in)
		}
	})
}

func TestNativePointerAndVulkanLikePatterns(t *testing.T) {
	handle := nativeTestLibrary(t)

	t.Run("pointer return", func(t *testing.T) {
		fn := nativeSymbol(t, handle, "gpui_ffi_identity_ptr")
		cif := prepareNativeCIF(t, types.PointerTypeDescriptor, types.PointerTypeDescriptor)
		target := byte(42)
		ptr := unsafe.Pointer(&target)
		var got unsafe.Pointer
		if _, err := CallFunction(&cif, fn, unsafe.Pointer(&got), []unsafe.Pointer{unsafe.Pointer(&ptr)}); err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}
		if got != ptr {
			t.Fatalf("got %p, want %p", got, ptr)
		}
	})

	t.Run("vk result with output pointer", func(t *testing.T) {
		fn := nativeSymbol(t, handle, "gpui_ffi_vk_result")
		cif := prepareNativeCIF(t, types.SInt32TypeDescriptor,
			types.UInt64TypeDescriptor,
			types.UInt32TypeDescriptor,
			types.UInt32TypeDescriptor,
			types.PointerTypeDescriptor,
		)
		handleArg := uint64(100)
		a := uint32(20)
		b := uint32(3)
		var out uint64
		outPtr := unsafe.Pointer(&out)
		var result int32
		args := []unsafe.Pointer{
			unsafe.Pointer(&handleArg),
			unsafe.Pointer(&a),
			unsafe.Pointer(&b),
			unsafe.Pointer(&outPtr),
		}
		if _, err := CallFunction(&cif, fn, unsafe.Pointer(&result), args); err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}
		if result != -7 {
			t.Fatalf("result = %d, want -7", result)
		}
		if out != 123 {
			t.Fatalf("out = %d, want 123", out)
		}
	})

	t.Run("ten argument mixed signature", func(t *testing.T) {
		fn := nativeSymbol(t, handle, "gpui_ffi_sum10")
		cif := prepareNativeCIF(t, types.UInt64TypeDescriptor,
			types.UInt64TypeDescriptor,
			types.UInt32TypeDescriptor,
			types.UInt32TypeDescriptor,
			types.UInt32TypeDescriptor,
			types.SInt32TypeDescriptor,
			types.UInt64TypeDescriptor,
			types.PointerTypeDescriptor,
			types.UInt32TypeDescriptor,
			types.PointerTypeDescriptor,
			types.UInt64TypeDescriptor,
		)
		a := uint64(1)
		b := uint32(2)
		c := uint32(3)
		d := uint32(4)
		e := int32(-5)
		f := uint64(6)
		pTarget := byte(7)
		p := unsafe.Pointer(&pTarget)
		g := uint32(8)
		qTarget := byte(9)
		q := unsafe.Pointer(&qTarget)
		h := uint64(10)
		var got uint64
		args := []unsafe.Pointer{
			unsafe.Pointer(&a),
			unsafe.Pointer(&b),
			unsafe.Pointer(&c),
			unsafe.Pointer(&d),
			unsafe.Pointer(&e),
			unsafe.Pointer(&f),
			unsafe.Pointer(&p),
			unsafe.Pointer(&g),
			unsafe.Pointer(&q),
			unsafe.Pointer(&h),
		}
		if _, err := CallFunction(&cif, fn, unsafe.Pointer(&got), args); err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}
		want := a + uint64(b) + uint64(c) + uint64(d) + uint64(uint32(e)) + f + uint64(uintptr(p)) + uint64(g) + uint64(uintptr(q)) + h
		if got != want {
			t.Fatalf("got %d, want %d", got, want)
		}
	})
}

func TestNativeFloatABI(t *testing.T) {
	handle := nativeTestLibrary(t)

	t.Run("void return with float args", func(t *testing.T) {
		fn := nativeSymbol(t, handle, "gpui_ffi_store_floats")
		cif := prepareNativeCIF(t, types.VoidTypeDescriptor,
			types.FloatTypeDescriptor,
			types.FloatTypeDescriptor,
			types.FloatTypeDescriptor,
			types.FloatTypeDescriptor,
			types.PointerTypeDescriptor,
		)
		a := float32(1.25)
		b := float32(2.5)
		c := float32(3.75)
		d := float32(4.0)
		out := [3]float32{}
		outPtr := unsafe.Pointer(&out[0])
		args := []unsafe.Pointer{
			unsafe.Pointer(&a),
			unsafe.Pointer(&b),
			unsafe.Pointer(&c),
			unsafe.Pointer(&d),
			unsafe.Pointer(&outPtr),
		}
		if _, err := CallFunction(&cif, fn, nil, args); err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}
		if out[0] != 3.75 || out[1] != 7.75 || out[2] != 5.0 {
			t.Fatalf("unexpected output: %#v", out)
		}
	})

	t.Run("float return with float args", func(t *testing.T) {
		fn := nativeSymbol(t, handle, "gpui_ffi_add_float")
		cif := prepareNativeCIF(t, types.FloatTypeDescriptor,
			types.FloatTypeDescriptor,
			types.FloatTypeDescriptor,
		)
		a := float32(10.5)
		b := float32(0.25)
		var got float32
		if _, err := CallFunction(&cif, fn, unsafe.Pointer(&got), []unsafe.Pointer{unsafe.Pointer(&a), unsafe.Pointer(&b)}); err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}
		if got != 10.75 {
			t.Fatalf("got %v, want 10.75", got)
		}
	})

	t.Run("double return with double args", func(t *testing.T) {
		fn := nativeSymbol(t, handle, "gpui_ffi_add_double")
		cif := prepareNativeCIF(t, types.DoubleTypeDescriptor,
			types.DoubleTypeDescriptor,
			types.DoubleTypeDescriptor,
		)
		a := float64(0.125)
		b := float64(10.0)
		var got float64
		if _, err := CallFunction(&cif, fn, unsafe.Pointer(&got), []unsafe.Pointer{unsafe.Pointer(&a), unsafe.Pointer(&b)}); err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}
		if got != 10.125 {
			t.Fatalf("got %v, want 10.125", got)
		}
	})

	t.Run("mixed integer and fp args", func(t *testing.T) {
		fn := nativeSymbol(t, handle, "gpui_ffi_mixed_fp")
		cif := prepareNativeCIF(t, types.DoubleTypeDescriptor,
			types.UInt64TypeDescriptor,
			types.FloatTypeDescriptor,
			types.DoubleTypeDescriptor,
			types.UInt32TypeDescriptor,
		)
		a := uint64(10)
		b := float32(1.5)
		c := float64(2.25)
		d := uint32(3)
		var got float64
		if _, err := CallFunction(&cif, fn, unsafe.Pointer(&got), []unsafe.Pointer{
			unsafe.Pointer(&a),
			unsafe.Pointer(&b),
			unsafe.Pointer(&c),
			unsafe.Pointer(&d),
		}); err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}
		if math.Abs(got-16.75) > 1e-12 {
			t.Fatalf("got %v, want 16.75", got)
		}
	})
}

func TestNativeStructABI(t *testing.T) {
	handle := nativeTestLibrary(t)

	pairType := &types.TypeDescriptor{
		Kind: types.StructType,
		Members: []*types.TypeDescriptor{
			types.DoubleTypeDescriptor,
			types.DoubleTypeDescriptor,
		},
	}
	sixType := &types.TypeDescriptor{
		Kind: types.StructType,
		Members: []*types.TypeDescriptor{
			types.DoubleTypeDescriptor,
			types.DoubleTypeDescriptor,
			types.DoubleTypeDescriptor,
			types.DoubleTypeDescriptor,
			types.DoubleTypeDescriptor,
			types.DoubleTypeDescriptor,
		},
	}

	type pairF64 struct {
		X float64
		Y float64
	}
	type sixF64 struct {
		A float64
		B float64
		C float64
		D float64
		E float64
		F float64
	}

	t.Run("struct argument", func(t *testing.T) {
		fn := nativeSymbol(t, handle, "gpui_ffi_sum_pair_f64")
		cif := prepareNativeCIF(t, types.DoubleTypeDescriptor, pairType)
		in := pairF64{X: 3.25, Y: 4.5}
		var got float64
		if _, err := CallFunction(&cif, fn, unsafe.Pointer(&got), []unsafe.Pointer{unsafe.Pointer(&in)}); err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}
		if got != 7.75 {
			t.Fatalf("got %v, want 7.75", got)
		}
		if pairType.Size != unsafe.Sizeof(in) {
			t.Fatalf("pair descriptor size = %d, want %d", pairType.Size, unsafe.Sizeof(in))
		}
	})

	t.Run("small struct return", func(t *testing.T) {
		fn := nativeSymbol(t, handle, "gpui_ffi_make_pair_f64")
		cif := prepareNativeCIF(t, pairType, types.DoubleTypeDescriptor, types.DoubleTypeDescriptor)
		x := float64(8.5)
		y := float64(9.25)
		var got pairF64
		if _, err := CallFunction(&cif, fn, unsafe.Pointer(&got), []unsafe.Pointer{
			unsafe.Pointer(&x),
			unsafe.Pointer(&y),
		}); err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}
		if got != (pairF64{X: 8.5, Y: 9.25}) {
			t.Fatalf("got %#v, want {8.5 9.25}", got)
		}
	})

	t.Run("large struct argument and return", func(t *testing.T) {
		makeFn := nativeSymbol(t, handle, "gpui_ffi_make_six_f64")
		makeCIF := prepareNativeCIF(t, sixType, types.DoubleTypeDescriptor)
		base := float64(10)
		var made sixF64
		if _, err := CallFunction(&makeCIF, makeFn, unsafe.Pointer(&made), []unsafe.Pointer{unsafe.Pointer(&base)}); err != nil {
			t.Fatalf("CallFunction(make_six) failed: %v", err)
		}
		wantMade := sixF64{A: 10, B: 11, C: 12, D: 13, E: 14, F: 15}
		if made != wantMade {
			t.Fatalf("made = %#v, want %#v", made, wantMade)
		}

		sumFn := nativeSymbol(t, handle, "gpui_ffi_sum_six_f64")
		sumCIF := prepareNativeCIF(t, types.DoubleTypeDescriptor, sixType)
		var got float64
		if _, err := CallFunction(&sumCIF, sumFn, unsafe.Pointer(&got), []unsafe.Pointer{unsafe.Pointer(&made)}); err != nil {
			t.Fatalf("CallFunction(sum_six) failed: %v", err)
		}
		if got != 75 {
			t.Fatalf("got %v, want 75", got)
		}
		if sixType.Size != unsafe.Sizeof(made) {
			t.Fatalf("six descriptor size = %d, want %d", sixType.Size, unsafe.Sizeof(made))
		}
	})
}

func TestNativeVariadicCall(t *testing.T) {
	handle := nativeTestLibrary(t)
	fn := nativeSymbol(t, handle, "gpui_ffi_sum_variadic")

	var cif types.CallInterface
	if err := PrepareVariadicCallInterface(&cif, types.DefaultCall, 1, types.UInt64TypeDescriptor,
		[]*types.TypeDescriptor{
			types.UInt32TypeDescriptor,
			types.UInt32TypeDescriptor,
			types.UInt32TypeDescriptor,
			types.UInt32TypeDescriptor,
			types.UInt32TypeDescriptor,
		}); err != nil {
		t.Fatalf("PrepareVariadicCallInterface failed: %v", err)
	}

	n := uint32(4)
	a := uint32(10)
	b := uint32(20)
	c := uint32(30)
	d := uint32(40)
	var got uint64
	if _, err := CallFunction(&cif, fn, unsafe.Pointer(&got), []unsafe.Pointer{
		unsafe.Pointer(&n),
		unsafe.Pointer(&a),
		unsafe.Pointer(&b),
		unsafe.Pointer(&c),
		unsafe.Pointer(&d),
	}); err != nil {
		t.Fatalf("CallFunction failed: %v", err)
	}
	if got != 104 {
		t.Fatalf("got %d, want 104", got)
	}
}

func TestNativeCallbacksCalledFromC(t *testing.T) {
	handle := nativeTestLibrary(t)

	t.Run("uintptr callback return", func(t *testing.T) {
		fn := nativeSymbol(t, handle, "gpui_ffi_call_cb_uintptr")
		cif := prepareNativeCIF(t, types.UInt64TypeDescriptor,
			types.PointerTypeDescriptor,
			types.UInt64TypeDescriptor,
			types.UInt64TypeDescriptor,
		)
		cb := func(a, b uintptr) uintptr {
			return a*10 + b
		}
		cbPtr := NewCallback(cb)
		a := uint64(7)
		b := uint64(9)
		var got uint64
		if _, err := CallFunction(&cif, fn, unsafe.Pointer(&got), []unsafe.Pointer{
			unsafe.Pointer(&cbPtr),
			unsafe.Pointer(&a),
			unsafe.Pointer(&b),
		}); err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}
		runtime.KeepAlive(cb)
		if got != 79 {
			t.Fatalf("got %d, want 79", got)
		}
	})

	t.Run("void callback with metal shaped args", func(t *testing.T) {
		fn := nativeSymbol(t, handle, "gpui_ffi_call_cb_void3")
		cif := prepareNativeCIF(t, types.VoidTypeDescriptor,
			types.PointerTypeDescriptor,
			types.UInt64TypeDescriptor,
			types.UInt64TypeDescriptor,
			types.UInt64TypeDescriptor,
		)
		var gotA, gotB uintptr
		var gotC uint64
		cb := func(a, b uintptr, c uint64) {
			gotA = a
			gotB = b
			gotC = c
		}
		cbPtr := NewCallback(cb)
		a := uint64(11)
		b := uint64(12)
		c := uint64(13)
		if _, err := CallFunction(&cif, fn, nil, []unsafe.Pointer{
			unsafe.Pointer(&cbPtr),
			unsafe.Pointer(&a),
			unsafe.Pointer(&b),
			unsafe.Pointer(&c),
		}); err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}
		runtime.KeepAlive(cb)
		if gotA != 11 || gotB != 12 || gotC != 13 {
			t.Fatalf("callback got (%d, %d, %d), want (11, 12, 13)", gotA, gotB, gotC)
		}
	})

	t.Run("vulkan debug shaped callback", func(t *testing.T) {
		fn := nativeSymbol(t, handle, "gpui_ffi_call_cb_vulkan")
		cif := prepareNativeCIF(t, types.UInt64TypeDescriptor,
			types.PointerTypeDescriptor,
			types.UInt64TypeDescriptor,
			types.UInt64TypeDescriptor,
			types.UInt64TypeDescriptor,
			types.UInt64TypeDescriptor,
		)
		cb := func(severity, msgType, callbackData, userData uintptr) uintptr {
			return severity + msgType + callbackData + userData
		}
		cbPtr := NewCallback(cb)
		a := uint64(1)
		b := uint64(2)
		c := uint64(3)
		d := uint64(4)
		var got uint64
		if _, err := CallFunction(&cif, fn, unsafe.Pointer(&got), []unsafe.Pointer{
			unsafe.Pointer(&cbPtr),
			unsafe.Pointer(&a),
			unsafe.Pointer(&b),
			unsafe.Pointer(&c),
			unsafe.Pointer(&d),
		}); err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}
		runtime.KeepAlive(cb)
		if got != 10 {
			t.Fatalf("got %d, want 10", got)
		}
	})
}

func TestNativeArgumentCountValidation(t *testing.T) {
	handle := nativeTestLibrary(t)
	fn := nativeSymbol(t, handle, "gpui_ffi_echo_u32")
	cif := prepareNativeCIF(t, types.UInt32TypeDescriptor, types.UInt32TypeDescriptor)

	var got uint32
	_, err := CallFunction(&cif, fn, unsafe.Pointer(&got), nil)
	if err == nil {
		t.Fatal("CallFunction with too few args should fail")
	}
	var cifErr *InvalidCallInterfaceError
	if !errors.As(err, &cifErr) || cifErr.Field != "avalue" {
		t.Fatalf("got %T %[1]v, want InvalidCallInterfaceError for avalue", err)
	}

	a := uint32(1)
	b := uint32(2)
	_, err = CallFunction(&cif, fn, unsafe.Pointer(&got), []unsafe.Pointer{unsafe.Pointer(&a), unsafe.Pointer(&b)})
	if err == nil {
		t.Fatal("CallFunction with too many args should fail")
	}
	if !errors.As(err, &cifErr) || cifErr.Field != "avalue" {
		t.Fatalf("got %T %[1]v, want InvalidCallInterfaceError for avalue", err)
	}
}
