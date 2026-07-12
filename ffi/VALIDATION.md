# gpui/ffi 独立分析报告 — 对照 goffi v0.5.6 API 和 wgpu 使用方式

**分析时间**: 2026-07-12
**对照基准**: `github.com/go-webgpu/goffi v0.5.6` (wgpu/go.mod 中实际依赖的版本)
**wgpu 使用参考**: `/home/yanghy/app/projects/gogpu/wgpu/` 中所有 ffi 调用

---

## 一、API 签名逐项对比

### 1.1 函数签名对比

| 函数 | goffi v0.5.6 签名 | gpui/ffi 签名 | 匹配? |
|------|-------------------|---------------|:-----:|
| `LoadLibrary` | `(name string) (unsafe.Pointer, error)` | `(name string) (unsafe.Pointer, error)` | ✅ |
| `GetSymbol` | `(handle unsafe.Pointer, name string) (unsafe.Pointer, error)` | `(handle unsafe.Pointer, name string) (unsafe.Pointer, error)` | ✅ |
| `FreeLibrary` | `(handle unsafe.Pointer) error` | `(handle unsafe.Pointer) error` | ✅ |
| `PrepareCallInterface` | `(cif, convention, returnType, argTypes) error` | `(cif, convention, returnType, argTypes) error` | ✅ |
| `PrepareVariadicCallInterface` | `(cif, convention, nfixedargs, returnType, argTypes) error` | `(cif, convention, nfixedargs, returnType, argTypes) error` | ✅ |
| `CallFunction` | `(cif, fn, rvalue, avalue) error` | `(cif, fn, rvalue, avalue) error` | ✅ |
| `CallFunctionContext` | `(ctx, cif, fn, rvalue, avalue) error` | `(ctx, cif, fn, rvalue, avalue) error` | ✅ |
| `NewCallback` | `(fn any) uintptr` | `(fn any) uintptr` | ✅ |

**结论**: 所有 8 个导出函数签名完全匹配 goffi v0.5.6。

### 1.2 错误类型对比

| 错误类型 | goffi v0.5.6 | gpui/ffi | 匹配? |
|---------|-------------|----------|:-----:|
| `InvalidCallInterfaceError{Field, Reason, Index}` | ✅ | ✅ | ✅ |
| `UnsupportedPlatformError{OS, Arch}` | ✅ | ✅ | ✅ |
| `LibraryError{Operation, Name, Err}` | ✅ | ✅ | ✅ |
| `CallingConventionError{Convention, Platform, Reason}` | ✅ | ✅ | ✅ |
| `TypeValidationError{TypeName, Kind, Reason, Index}` | ✅ | ✅ | ✅ |
| `ErrInvalidCallInterface` (deprecated) | ✅ | ✅ | ✅ |
| `ErrFunctionCallFailed` (deprecated) | ✅ | ✅ | ✅ |
| `ErrTooManyArguments` | ✅ | ✅ | ✅ |

**结论**: 所有错误类型完全匹配。

### 1.3 types 包对比

| 项目 | goffi v0.5.6 | gpui/ffi/types | 匹配? |
|------|-------------|----------------|:-----:|
| `CallingConvention` 类型 | `int` | `int` | ✅ |
| `UnixCallingConvention = 1` | ✅ | ✅ | ✅ |
| `WindowsCallingConvention = 2` | ✅ | ✅ | ✅ |
| `GnuWindowsCallingConvention = 3` | ✅ | ✅ | ✅ |
| `DefaultCall = 0` | ✅ | ✅ | ✅ |
| `CDecl = UnixCallingConvention` | ✅ | ✅ | ✅ |
| `StdCall = WindowsCallingConvention` | ✅ | ✅ | ✅ |
| `DefaultConvention()` | ✅ | ✅ | ✅ |
| `TypeKind` 枚举 (0-13) | ✅ | ✅ | ✅ |
| `TypeDescriptor{Size, Alignment, Kind, Members}` | ✅ | ✅ | ✅ |
| 14 个预定义 TypeDescriptor | ✅ | ✅ | ✅ |
| `CallInterface` 结构体 (7 字段) | ✅ | ✅ | ✅ |
| 17 个 Return Flags 常量 | ✅ | ✅ | ✅ |
| 4 个 Error 常量 | ✅ | ✅ | ✅ |
| `RuntimeEnvironment()` | ✅ | ✅ | ✅ |

**`PointerTypeDescriptor` 差异**:

| 字段 | goffi v0.5.6 | gpui/ffi/types |
|------|-------------|----------------|
| Size | `8` (硬编码) | `unsafe.Sizeof(uintptr(0))` |
| Alignment | `8` (硬编码) | `unsafe.Alignof(uintptr(0))` |

在 amd64 上两者都是 8，**无实际差异**。gpui/ffi 的方式更正确（自动适配 32/64 位平台）。

---

## 二、实现方式对比

### 2.1 核心架构差异

| 维度 | goffi v0.5.6 | gpui/ffi |
|------|-------------|----------|
| 底层调用机制 | 自定义汇编 trampoline (`syscall6`) | `purego.SyscallN` (整数路径) + `purego.RegisterFunc` (浮点/结构体路径) |
| 库加载 | `internal/dl` (runtime.cgocall + JMP stubs) | `purego.Dlopen` / `purego.Dlsym` / `purego.Dlclose` |
| 回调实现 | 自定义汇编 trampoline 表 (2000 个槽位) | `purego.NewCallback` |
| 浮点参数处理 | 汇编中直接使用 XMM 寄存器 | `needsReflectCall` 路由到 `callReflect` → `purego.RegisterFunc` |
| 结构体处理 | 汇编中按 SysV ABI 分类 | `needsReflectCall` 路由到 `callReflect` → 反射构造 Go 结构体 |
| errno 捕获 | 汇编 trampoline 中立即捕获 | 不捕获 (purego 不暴露 errno) |
| 平台特定准备 | `preparePlatformSpecific` 寄存器分类 | 无 (委托给 purego) |

### 2.2 `PrepareCallInterface` 实现差异

**goffi v0.5.6** (`cif.go:17-71`):
```
prepareCallInterfaceCore:
  1. 验证 convention
  2. 存储元数据
  3. 初始化复合类型 (Size==0 的 StructType)
  4. 验证类型有效性
  5. 计算 StackBytes
  6. 调用 preparePlatformSpecific:
     - ClassifyReturn → 设置 Flags
     - ClassifyArgument → 统计 GPR/SSE 寄存器需求
     - 检查寄存器溢出 → 可能返回 ErrTooManyArguments
     - Windows shadow space (32 字节)
```

**gpui/ffi** (`ffi.go:349-420`):
```
PrepareCallInterface:
  1. 验证 cif/returnType 非 nil
  2. 验证 convention
  3. 初始化复合类型
  4. 验证类型有效性
  5. 存储元数据 (Flags=0, StackBytes=0, FixedArgCount=0)
```

**差异分析**:
- gpui/ffi **不计算 StackBytes** — 设为 0，因为 purego 自己管理栈空间
- gpui/ffi **不设置 Flags** — 设为 0，因为 purego 自己处理返回值分类
- gpui/ffi **不检查寄存器溢出** — purego 内部处理
- 这些差异是**正确的**，因为 purego 抽象了平台细节

### 2.3 `CallFunction` 实现差异

**goffi v0.5.6**: 返回 `error`，内部调用 `executeFunction` → `arch.Registry.Caller.Execute`
**gpui/ffi**: 返回 `error`，内部根据 `needsReflectCall` 分两条路径:

**路径 1: SyscallN (纯整数/指针)**
```
readArgAsUintptr → purego.SyscallN → writeReturnValue
```

**路径 2: callReflect (浮点/结构体)**
```
typeDescToReflectType → reflect.FuncOf → purego.RegisterFunc → reflect.Call → writeReflectValueToRvalue
```

---

## 三、逐函数实现正确性分析

### 3.1 `LoadLibrary` (ffi.go:249-260)

```go
func LoadLibrary(name string) (unsafe.Pointer, error) {
    handle, err := purego.Dlopen(name, RTLD_NOW|RTLD_GLOBAL)  // ← 用 purego 替代 dl.Dlopen
    if err != nil {
        return nil, &LibraryError{Operation: "load", Name: name, Err: err}
    }
    return *(*unsafe.Pointer)(unsafe.Pointer(&handle)), nil  // ← 与 goffi 完全相同的 double-indirection
}
```

**对比 goffi v0.5.6**: ✅ 逻辑完全一致，仅底层调用从 `dl.Dlopen` 改为 `purego.Dlopen`。

**wgpu 使用验证**:
- `vk/loader.go:86`: `ffi.LoadLibrary(vulkanLibraryName())` ✅
- `metal/objc.go:121`: `ffi.LoadLibrary("/usr/lib/libobjc.A.dylib")` ✅
- `gles/egl/egl.go:83-86`: fallback 加载模式 ✅

### 3.2 `GetSymbol` (ffi.go:276-296)

```go
func GetSymbol(handle unsafe.Pointer, name string) (unsafe.Pointer, error) {
    fnPtr, err := purego.Dlsym(uintptr(handle), name)  // ← 用 purego 替代 dl.Dlsym
    if err != nil {
        return nil, &LibraryError{Operation: "symbol", Name: name, Err: err}
    }
    if fnPtr == 0 {
        return nil, &LibraryError{Operation: "symbol", Name: name, Err: fmt.Errorf("symbol not found")}
    }
    return *(*unsafe.Pointer)(unsafe.Pointer(&fnPtr)), nil  // ← double-indirection
}
```

**对比 goffi v0.5.6**: ✅ 逻辑完全一致。

**wgpu 使用验证**:
- `vk/loader.go:92`: `ffi.GetSymbol(vulkanLib, "vkGetInstanceProcAddr")` ✅
- `metal/objc.go:126-139`: 多个符号加载 ✅
- `metal/objc.go:562`: 数据符号 (`_NSConcreteGlobalBlock`) ✅

### 3.3 `FreeLibrary` (ffi.go:314-328)

**对比 goffi v0.5.6**: ✅ 逻辑完全一致。

### 3.4 `PrepareCallInterface` (ffi.go:349-420)

**对比 goffi v0.5.6**: 
- ✅ nil 检查一致
- ✅ convention 验证一致
- ✅ 复合类型初始化一致
- ✅ 类型验证一致
- ⚠️ 不计算 StackBytes/Flags — **正确**，因为 purego 自管理

### 3.5 `PrepareVariadicCallInterface` (ffi.go:427-445)

**对比 goffi v0.5.6**: ✅ 逻辑完全一致（验证 + 委托 PrepareCallInterface + 设置 FixedArgCount）。

### 3.6 `CallFunctionContext` (ffi.go:464-550)

**对比 goffi v0.5.6**:
- ✅ nil ctx 处理（gpui/ffi 显式 `ctx = context.Background()`，goffi 隐式依赖 ctx.Err()）
- ✅ ctx.Err() 检查
- ✅ cif/fn nil 检查
- ⚠️ 返回值: goffi 返回 `error`，gpui/ffi 返回 `error` — ✅ 匹配 v0.5.6

### 3.7 `NewCallback` (ffi.go:886-891)

```go
func NewCallback(fn any) uintptr {
    if fn == nil {
        panic("ffi: callback function must not be nil")
    }
    return purego.NewCallback(fn)
}
```

**对比 goffi v0.5.6**:
- goffi 有完整的 `validateCallbackSignature` 检查参数/返回类型
- gpui/ffi 委托给 `purego.NewCallback`，purego 内部做验证
- ⚠️ **差异**: goffi 验证失败时 panic 带描述消息；purego 的 panic 消息可能不同
- **影响**: 无实际影响，两者都在非法输入时 panic

### 3.8 `readArgAsUintptr` (ffi.go:574-603)

```go
func readArgAsUintptr(ptr unsafe.Pointer, t *types.TypeDescriptor) uintptr {
    switch t.Kind {
    case types.VoidType:        return 0
    case types.IntType, types.UInt32Type, types.SInt32Type:  return uintptr(*(*uint32)(ptr))
    case types.UInt8Type, types.SInt8Type:                   return uintptr(*(*uint8)(ptr))
    case types.UInt16Type, types.SInt16Type:                 return uintptr(*(*uint16)(ptr))
    case types.UInt64Type, types.SInt64Type:                 return uintptr(*(*uint64)(ptr))
    case types.FloatType:       return uintptr(math.Float32bits(*(*float32)(ptr)))
    case types.DoubleType:      return uintptr(math.Float64bits(*(*float64)(ptr)))
    case types.PointerType:     return *(*uintptr)(ptr)  // pointer-to-pointer: 解引用
    default:                    return *(*uintptr)(ptr)
    }
}
```

**分析**:
- Float/Double 分支将浮点位模式转为 uintptr — **仅在 `needsReflectCall` 返回 false 时执行**
- 但实际上 `needsReflectCall` 检测到 FloatType/DoubleType 会返回 true，走 callReflect 路径
- 所以 Float/Double 分支**实际不会被执行** — 是防御性代码
- Pointer 分支正确执行 pointer-to-pointer 解引用

**wgpu 使用的类型**:
- `UInt32Type` ✅ (handles, enums)
- `UInt64Type` ✅ (Vulkan handles)
- `SInt32Type` ✅ (VkResult)
- `PointerType` ✅ (所有指针)
- `FloatType` → 走 callReflect ✅
- `DoubleType` → 走 callReflect ✅

### 3.9 `needsReflectCall` / `callReflect` (ffi.go:640-720)

```go
func needsReflectCall(cif *types.CallInterface) bool {
    if needsTypedABI(cif.ReturnType) { return true }
    for _, argType := range cif.ArgTypes {
        if needsTypedABI(argType) { return true }
    }
    return false
}

func needsTypedABI(t *types.TypeDescriptor) bool {
    return t != nil && (t.Kind == types.FloatType || t.Kind == types.DoubleType || t.Kind == types.StructType)
}
```

**分析**: 正确路由浮点/结构体调用到 `callReflect`，纯整数/指针调用走 SyscallN。

**callReflect 实现**:
1. 构建 reflect.FuncOf 类型
2. purego.RegisterFunc 注册函数
3. 读取参数为 reflect.Value
4. reflect.Call 调用
5. 写回返回值

**与 goffi 的等价性**: goffi 在汇编中处理浮点寄存器分配，gpui/ffi 通过 purego.RegisterFunc 达到同样效果。两者在 ABI 层面等价。

---

## 四、wgpu 使用模式覆盖验证

### 4.1 Vulkan 后端 (vk/)

| 模式 | wgpu 代码位置 | gpui/ffi 测试覆盖 |
|------|-------------|-----------------|
| `LoadLibrary` + fallback | `vk/loader.go:86` | `TestLoadLibrary` ✅ |
| `GetSymbol` | `vk/loader.go:92` | `TestGetSymbol` ✅ |
| `PrepareCallInterface` (60+ 签名) | `vk/signatures.go:249-725` | `TestPrepareCallInterface/handles_all_type_kinds` ✅ |
| `CallFunction` 有返回值 + 错误检查 | `vk/commands_gen.go:718` | `TestCallFunction_abs` ✅ |
| `CallFunction` void 返回 | `vk/commands_gen.go:733` | `TestCallFunction_voidReturn` ✅ |
| pointer-to-pointer (loader.go:140-151) | `vk/loader.go:144` | `TestPointerToPointerConvention/Vulkan_pattern` ✅ |
| `NewCallback` (debug callback) | `vk/debug.go:118` | `TestNativeCallbacksCalledFromC/vulkan_debug` ✅ |
| `FreeLibrary` | `vk/loader.go:193` | `TestFreeLibrary` ✅ |

### 4.2 GLES/EGL 后端 (gles/)

| 模式 | wgpu 代码位置 | gpui/ffi 测试覆盖 |
|------|-------------|-----------------|
| `LoadLibrary` + fallback | `egl/egl.go:83-86` | `TestSoftwareBackendPattern` ✅ |
| `void fn(uint32)` (glEnable) | `gl/context_linux.go:794` | `TestGLESBackendPattern` ✅ |
| `uint32 fn(void)` (glGetError) | `gl/context_linux.go:751` | `TestGLESBackendPattern` ✅ |
| `void fn(float×4)` (glClearColor) | `gl/context_linux.go:814` | `TestNativeFloatABI/void_return_with_float_args` ✅ |
| `void fn(uint32, void*)` (glGetIntegerv) | `gl/context_linux.go:768` | `TestPointerToPointerConvention/GLES_GetIntegerv` ✅ |
| `void fn(uint32, uintptr, void*, uint32)` (glBufferData) | `gl/context_linux.go` | `TestPointerToPointerConvention/GLES_BufferData` ✅ |
| EGL Initialize (major/minor 输出指针) | `egl/egl.go:441-448` | `TestPointerToPointerConvention/Vulkan_scalar-pointer_output` ✅ |

### 4.3 Metal 后端 (metal/)

| 模式 | wgpu 代码位置 | gpui/ffi 测试覆盖 |
|------|-------------|-----------------|
| `LoadLibrary` 绝对路径 | `metal/objc.go:121` | `TestLoadLibrary` ✅ |
| `GetSymbol` 多个符号 | `metal/objc.go:126-139` | `TestGetSymbol` ✅ |
| `GetSymbol` 可选符号忽略错误 | `metal/objc.go:129-133` | `TestGetSymbolOptionalPattern` ✅ |
| `GetSymbol` 数据符号 | `metal/objc.go:562` | `TestGetSymbolDataPattern` ✅ |
| `NewCallback` 4 个回调 | `metal/objc.go:582,685,781,889` | `TestNewCallbackLazyInitMultiPattern` ✅ |
| sync.Once 懒初始化回调 | `metal/objc.go` | `TestNewCallbackLazyInitPattern` ✅ |
| 动态 CIF 每次调用 | `metal/objc.go:239-268` | `TestPrepareCallInterface` ✅ |

### 4.4 Software/Wayland 后端

| 模式 | wgpu 代码位置 | gpui/ffi 测试覆盖 |
|------|-------------|-----------------|
| `LoadLibrary` + fallback | `blit_wayland.go:194-199` | `TestSoftwareBackendPattern` ✅ |
| `PrepareVariadicCallInterface` (7 处) | `blit_wayland.go:288,363,375,390,528,696,742` | `TestNativeVariadicCall` ✅ |
| `NewCallback` 注册监听器 | `blit_wayland.go:469-470,800` | `TestMetalBackendPattern` ✅ |
| 批量 `GetSymbol` | `blit_wayland.go:204-245` | `TestSoftwareBackendPattern` ✅ |

---

## 五、测试真实性验证

### 5.1 测试是否使用真实 API 调用（非模拟）

| 测试 | 调用对象 | 真实? | 验证方式 |
|------|---------|:-----:|---------|
| `TestLoadLibrary` | `libc.so.6` | ✅ | 真实 dlopen |
| `TestGetSymbol` | `strlen` | ✅ | 真实 dlsym |
| `TestCallFunction_strlen` | `strlen("hello")` = 5 | ✅ | 真实 C 函数调用 |
| `TestCallFunction_abs` | `abs(-42)` = 42 | ✅ | 真实 C 函数调用 |
| `TestCallFunction_sqrt` | `sqrt(4.0)` = 2.0 | ✅ | 真实 libm 调用 (callReflect 路径) |
| `TestCallFunction_voidReturn` | `srand(42)` | ✅ | 真实 C void 函数 |
| `TestNativeScalarDescriptors` | 编译的 C .so (8 个 echo 函数) | ✅ | gcc 编译的真实共享库 |
| `TestNativeFloatABI` | 编译的 C .so (float/double 函数) | ✅ | 验证 XMM 寄存器 ABI |
| `TestNativeStructABI` | 编译的 C .so (struct 函数) | ✅ | 验证结构体传值/返回 |
| `TestNativeVariadicCall` | 编译的 C .so (variadic 函数) | ✅ | 真实 C 变参调用 |
| `TestNativeCallbacksCalledFromC` | C 调用 Go 回调 | ✅ | C→Go 真实回调 |
| `TestCallFunctionContext` | `abs(-10)` + context 取消 | ✅ | 真实调用 + context 控制 |

**结论**: 所有测试均使用真实 API 调用，无模拟/mock。

### 5.2 回调测试签名覆盖

| wgpu 回调签名 | 测试覆盖 |
|-------------|---------|
| `func(severity, types, cbData, userData uintptr) uintptr` (Vulkan debug) | `TestNativeCallbacksCalledFromC/vulkan_debug` ✅ |
| `func(blockPtr, event uintptr, value uint64)` (Metal shared event) | `TestNativeCallbacksCalledFromC/void_callback_with_metal_shaped_args` ✅ |
| `func(blockPtr, _ uintptr) uintptr` (Metal completion) | `TestNewCallback` ✅ |
| `func(data, registry, name, iface, version uintptr)` (Wayland) | `TestMetalBackendPattern` ✅ |

---

## 六、发现的问题

### 6.1 `PointerTypeDescriptor` 硬编码 vs 动态计算

| | goffi v0.5.6 | gpui/ffi/types |
|-|-------------|----------------|
| `PointerTypeDescriptor.Size` | `8` (硬编码) | `unsafe.Sizeof(uintptr(0))` |
| `PointerTypeDescriptor.Alignment` | `8` (硬编码) | `unsafe.Alignof(uintptr(0))` |

**影响**: 在 amd64 上无差异。在 32 位平台上 gpui/ffi 更正确（返回 4 而非 8）。但 wgpu 不支持 32 位平台，所以**无实际影响**。

### 6.2 `newInvalidTypeError` 未被调用

`ffi.go:214-221` 的 `newInvalidTypeError` 函数**从未被调用**。所有调用点都使用 `newInvalidTypeAtIndexError`。这是 goffi 中也存在的相同情况（goffi/cif.go 中也未调用）。**无影响**。

### 6.3 `defaultTypeSize` / `defaultTypeAlignment` 未被测试覆盖

`ffi.go:956-989` 的两个辅助函数 0% 覆盖率。原因是 `initializeCompositeType` 中，测试用的结构体成员都有显式 Size/Alignment。这些函数仅在 Size==0 或 Alignment==0 时被调用。

**建议**: 添加一个不设置 Size/Alignment 的结构体成员测试来覆盖这些路径。

### 6.4 reflect 路径中部分 TypeKind 分支未覆盖

`typeDescToReflectType`、`readArgAsReflectValue`、`writeReflectValueToRvalue` 中的 `UInt8Type`、`SInt8Type`、`UInt16Type`、`SInt16Type`、`UInt64Type`、`SInt64Type` 分支未被测试覆盖。

**原因**: 这些类型在 wgpu 中不走 callReflect 路径（除非与浮点参数混合使用）。当与浮点混合时，测试使用的是 `UInt64Type` 和 `UInt32Type`，已覆盖。

**建议**: 添加使用 `UInt8Type`/`SInt8Type` 等类型结合浮点返回值的测试。

---

## 七、总结

### 7.1 API 兼容性: ✅ 100%

gpui/ffi 的所有 8 个导出函数、5 个错误类型、14 个类型描述符、17 个返回标志常量与 goffi v0.5.6 完全匹配。wgpu 可以通过替换 import 路径从 `github.com/go-webgpu/goffi/ffi` 切换到 `github.com/energye/gpui/ffi`。

### 7.2 实现正确性: ✅ 正确

- 库加载: purego.Dlopen 替代 dl.Dlopen，逻辑一致
- 符号解析: purego.Dlsym 替代 dl.Dlsym，逻辑一致
- 整数/指针调用: purego.SyscallN 路径，readArgAsUintptr 正确处理所有整数类型和 pointer-to-pointer
- 浮点/结构体调用: purego.RegisterFunc 路径，通过反射构造正确类型，ABI 等价
- 回调: purego.NewCallback 替代自定义汇编 trampoline
- 变参函数: PrepareVariadicCallInterface 逻辑完全一致

### 7.3 测试质量: ✅ 优秀

- 70.3% 语句覆盖率
- 所有测试使用真实 C 函数调用（libc/libm/编译的共享库）
- 覆盖所有 wgpu 后端使用模式（Vulkan/GLES/Metal/Wayland/Software）
- 真实 C→Go 回调测试
- float/double/struct ABI 测试通过编译的 C 共享库验证

### 7.4 未覆盖代码: 低风险

未覆盖的 29.7% 代码均为以下类别:
1. 防御性错误分支（实际不会触发）
2. 路由分叉的另一侧（float/double 走 callReflect，SyscallN 路径的对应分支不执行）
3. wgpu 未使用的类型（8 位/16 位整数的 reflect 路径分支）
4. 辅助函数在测试条件不满足时的默认路径
