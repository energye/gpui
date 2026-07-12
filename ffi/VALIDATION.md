# gpui/ffi validation record

Last verified: 2026-07-12 on linux/amd64.

This file records the real validation coverage for the `github.com/energye/gpui/ffi`
middle layer that replaces `github.com/go-webgpu/goffi/ffi` with a purego-backed
implementation.

## API coverage

The tests cover all exported `ffi` APIs:

- `LoadLibrary`, `GetSymbol`, `FreeLibrary`: real `dlopen`/`dlsym`/`dlclose`
  calls against `libc.so.6`, `libm.so.6`, Wayland, Vulkan, EGL, and GLES library
  availability checks where present.
- `PrepareCallInterface`: nil/error paths, all scalar descriptors, pointer
  descriptors, float descriptors, mixed signatures, and struct descriptors.
- `PrepareVariadicCallInterface`: valid fixed/variadic layouts and invalid
  `nfixedargs` cases, plus a real C variadic function call.
- `CallFunction` and `CallFunctionContext`: real C calls for integer, pointer,
  float, double, struct, variadic, context-cancelled, and nil-context cases.
- `NewCallback`: callback registration, lazy initialization patterns, and real
  C-to-Go callback calls for the callback shapes used by the project.

The tests cover all exported `ffi/types` data:

- calling convention constants and `DefaultConvention`
- all scalar `TypeDescriptor` constants
- pointer descriptor size/alignment for the current platform
- `CallInterface` layout fields
- return flag constants
- exported error variables
- `RuntimeEnvironment`

## Native ABI coverage

`ffi_native_test.go` builds a real shared object at test time with:

```bash
gcc -shared -fPIC -O2 -o libgpui_ffi_native.so gpui_ffi_native.c
```

The resulting `.so` is loaded through `LoadLibrary`, symbols are resolved through
`GetSymbol`, and calls are made through `CallFunction`.

Validated native ABI categories:

- signed and unsigned 8/16/32/64-bit scalar arguments and returns
- pointer arguments and pointer returns
- Vulkan-style `VkResult` return with output pointer
- 10-argument mixed signatures, matching the current maximum observed
  `args := [N]unsafe.Pointer` use in `wgpu/hal`
- `void(float, float, float, float, pointer)` and float/double returns, which
  verifies typed FP register handling instead of `SyscallN`
- mixed integer and floating-point arguments
- C variadic calls
- C calling Go callbacks with `uintptr` return and `void` callback signatures
- struct-by-value arguments, small struct returns, and large struct returns
- GLES-style `void(uint32)`, `uint32(void)`, `void(float*4)`,
  `void(uint32, void*)`, and `void(uint32, uintptr, void*, uint32)` patterns
- pointer-to-pointer conventions used by Vulkan and GLES loaders

## Source usage reference

The validation categories were checked against the original usage patterns in
`/home/yanghy/app/projects/gogpu/wgpu` and the migrated `wgpu/hal` tree.
The current migrated tree uses these `ffi` APIs in `wgpu/hal`:

- `CallFunction`: 757 uses
- `PrepareCallInterface`: 151 uses
- `PrepareVariadicCallInterface`: 7 uses
- `LoadLibrary`: 19 uses
- `GetSymbol`: 58 uses
- `FreeLibrary`: 1 use
- `NewCallback`: 18 uses

Observed maximum static `args := [N]unsafe.Pointer` size in `wgpu/hal` is 10.

## Commands run

Passing commands:

```bash
env GOCACHE=/tmp/gpui-go-cache go test -count=1 -v ./ffi ./ffi/types
env GOCACHE=/tmp/gpui-go-cache go vet ./ffi ./ffi/types
env GOCACHE=/tmp/gpui-go-cache go test -count=1 ./wgpu/hal/...
env GOCACHE=/tmp/gpui-go-cache go build ./...
```

Commands with non-ffi failures:

```bash
env GOCACHE=/tmp/gpui-go-cache go vet ./ffi ./ffi/types ./wgpu/hal/...
```

This fails in files outside `ffi`:

- `wgpu/hal/gles/egl/context.go:320`: possible misuse of `unsafe.Pointer`
- `wgpu/hal/gles/gl/context_linux.go:2012`: possible misuse of `unsafe.Pointer`
- `wgpu/hal/gles/command.go:1543`: possible misuse of `unsafe.Pointer`

```bash
env GOCACHE=/tmp/gpui-go-cache go test -count=1 ./...
```

This fails in `gg/internal/gpu`, outside `ffi`, because the software backend
does not support MSAA texture creation with `SampleCount=4`:

- `TestGlyphMaskGPURepro`
- `TestGlyphMaskRenderFrameNonGrouped`

## Environment limits

Validated in the current environment:

- OS/arch: linux/amd64
- `gcc`: available and used for native shared-library ABI tests
- `libc.so.6`: available
- `libm.so.6`: available
- `libvulkan.so.1`: available
- `libwayland-client.so.0`: available
- `libEGL.so.1`: available
- `libGLESv2.so.2`: available

Not validated in the current environment:

- macOS/Metal runtime behavior, because this host is not macOS
- Windows/DX12 runtime behavior, because this host is not Windows
- qsort-style callback returning `int32`; this is intentionally skipped because
  purego callback ABI handling for that shape is not used by the current project
  callback paths and is unreliable on this platform

The sandbox requires `GOCACHE=/tmp/gpui-go-cache` for Go commands.
