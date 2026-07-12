# GPUI 开发任务计划

> 由 AI 会话维护，新会话读取此文档了解当前进度。
> 最后更新: 2026-07-12

---

## 开发环境

| 项目 | 值 |
|------|-----|
| 操作系统 | Linux 6.8.0-124-generic x86_64 (Ubuntu 22.04) |
| Go 版本 | go1.25.11 linux/amd64 |
| GPU | Intel HD Graphics 520 (Skylake GT2) |
| Vulkan | ✅ 可用（libvulkan.so.1，含 Intel 驱动） |
| GLES | ✅ 可用（libEGL.so.1 + libGLESv2.so.2） |
| Software | ✅ 纯 CPU，无硬件要求 |
| X11 | ✅ 可用（libX11.so.6） |
| Wayland | ✅ 可用（libwayland-client.so.0） |
| DX12 | ❌ 仅 Windows |
| Metal | ❌ 仅 macOS |

**当前可测试的后端：** Vulkan ✅ | GLES ✅ | Software ✅ | X11 blit ✅ | Wayland blit ✅

---

## 阶段一：代码迁移与基础清理 ✅

| 阶段项 | 状态 | 完成的事情 |
|--------|:----:|-----------|
| 迁移核心库 | ✅ 完成 | 将 gg, wgpu, naga, gputypes, gpucontext 从 gogpu 生态迁移到 `github.com/energye/gpui` |
| 排除无关库 | ✅ 完成 | 排除 gogpu(窗口/平台), ui(GUI工具包), webgpu(Rust FFI), goffi(待替换) |
| 清理 wgpu 冗余代码 | ✅ 完成 | 删除 cmd/, examples/, *_rust*.go, *_browser*.go, hal/noop/ |
| 清理 gg 冗余代码 | ✅ 完成 | 删除 cmd/, 依赖 gogpu 的示例(10个) |
| 清理 naga 冗余代码 | ✅ 完成 | 删除 cmd/, snapshot/ |
| 合并单模块 | ✅ 完成 | 删除子模块 go.mod, 合并为 `github.com/energye/gpui` 单模块 |
| 替换导入路径 | ✅ 完成 | `github.com/gogpu/*` → `github.com/energye/gpui/*` (1141处替换) |
| 编译验证 | ✅ 完成 | `go build ./...` 通过 |
| 保留示例程序 | ✅ 完成 | 恢复不依赖 gogpu 的 gg 示例(19个) + wgpu 示例(7个) |

---

## 阶段二：goffi 替换为 purego（中间层方案）🔄

### 方案说明

**不直接修改代码调用方式，而是创建 `gpui/ffi` 中间层包。**

`gpui/ffi` 导出与 `goffi` 完全相同的 API，内部用 `purego` 实现。
现有代码只需改 import 路径，调用方式**零改动**。

```
替换前:  import "github.com/go-webgpu/goffi/ffi"     → 代码中 ffi.LoadLibrary(...)
         import "github.com/go-webgpu/goffi/types"   → 代码中 types.UInt32TypeDescriptor

替换后:  import "github.com/energye/gpui/ffi"        → 代码中 ffi.LoadLibrary(...)  不变
         import "github.com/energye/gpui/ffi/types"  → 代码中 types.UInt32TypeDescriptor  不变
```

**优势：**
- 上游代码更新时，直接拉新代码 → 改 import 路径 → 编译通过
- 无需逐行替换 `CallFunction` → `SyscallN`，中间层统一处理
- 性能损失极小（~5-15 ns/次，占 GPU 驱动调用的 <1.5%）

### 中间层 API 映射

| goffi API | 中间层实现（内部用 purego） |
|-----------|---------------------------|
| `ffi.LoadLibrary(path)` | `purego.Dlopen(path, RTLD_LAZY\|RTLD_GLOBAL)` |
| `ffi.GetSymbol(lib, name)` | `purego.Dlsym(lib, name)` |
| `ffi.FreeLibrary(lib)` | `purego.Dlclose(lib)` |
| `ffi.PrepareCallInterface(&cif, ct, ret, args)` | 仅存储类型信息，无实际准备 |
| `ffi.CallFunction(&cif, fn, result, args)` | 读取 args → `purego.SyscallN` → 写回 result |
| `ffi.NewCallback(fn)` | `purego.NewCallback(fn)` |

### 中间层文件结构

```
gpui/ffi/
├── ffi.go          ← LoadLibrary, GetSymbol, FreeLibrary, PrepareCallInterface, CallFunction, NewCallback
└── types/
    └── types.go    ← CallInterface, TypeDescriptor, 所有 TypeDescriptor 变量
```

### 替换清单

| 阶段项 | 状态 | 完成的事情 |
|--------|:----:|-----------|
| **创建 `gpui/ffi/types` 包** | ⏳ 待开始 | TypeDescriptor 定义 + 所有类型描述符常量 |
| **创建 `gpui/ffi/ffi.go`** | ⏳ 待开始 | 6 个函数，内部用 purego 实现 |
| **替换 import 路径** | ⏳ 待开始 | 15 个文件：`go-webgpu/goffi/ffi` → `gpui/ffi` |
| **清理 goffi 依赖** | ⏳ 待开始 | 从 go.mod 移除 `github.com/go-webgpu/goffi` |
| **编译验证** | ⏳ 待开始 | `go build ./...` 通过 |
| **Vulkan 后端测试** | ⏳ 待开始 | `go test ./wgpu/hal/vulkan/...` |
| **GLES 后端测试** | ⏳ 待开始 | `go test ./wgpu/hal/gles/...` |
| **Software 后端测试** | ⏳ 待开始 | `go test ./wgpu/hal/software/...` |
| **Metal 后端编译验证** | ⏳ 待开始 | `go build ./wgpu/hal/metal/...`（仅 macOS） |

**覆盖范围：**

| 后端 | 文件数 | CallFunction | NewCallback | LoadLibrary | GetSymbol |
|------|:-----:|:------------:|:-----------:|:-----------:|:---------:|
| Vulkan | 6 | 571 | 2 | 1 | 1 |
| Metal | 2 | 5 | 13 | 3 | 8 |
| GLES | 4 | 133 | 0 | 8 | 28 |
| Software | 3 | 49 | 3 | 7 | 21 |
| **总计** | **15** | **758** | **18** | **19** | **58** |

**无需替换的 GPU 后端：**
- `hal/dx12/` — 全部用 `syscall.Syscall`，零 goffi ✅
- `hal/gles/gl/context.go` — Windows GL 用 `syscall.SyscallN` (103次) ✅
- `hal/software/blit_windows.go` — Windows 用 `syscall` ✅

---

## 阶段三：验证测试 🔬

### 验证策略

**5 层验证，逐层递进：**

| 层级 | 验证方式 | 捕获的问题 |
|:----:|---------|-----------|
| 1️⃣ 编译期 | `go build ./...` | 导入路径、语法、类型错误 |
| 2️⃣ 静态分析 | `go vet` | unsafe.Pointer 误用 |
| 3️⃣ 单元测试 | `go test` 各包 | 参数传递、返回值、逻辑错误 |
| 4️⃣ 集成测试 | `go test -count=1 ./...` | 端到端 FFI 调用链 |
| 5️⃣ 运行时断言 | 返回值判零 + 日志 | 静默失败（handle=0 不崩溃但后续全错） |

### 此环境可执行的测试

| 后端 | 可测试 | 测试文件 | 说明 |
|------|:-----:|---------|------|
| Vulkan | ✅ | `wgpu/hal/vulkan/*_test.go` (11个) | 真实 GPU 调用 |
| GLES | ✅ | `wgpu/hal/gles/*_test.go` (9个) | 真实 EGL/GL 调用 |
| Software | ✅ | `wgpu/hal/software/*_test.go` (14个) | 纯 CPU，无硬件要求 |
| X11 blit | ✅ | `wgpu/hal/software/draw_test.go` | 需要 X11 显示 |
| 核心逻辑 | ✅ | `wgpu/*_test.go` (21个) | 不依赖具体后端 |
| Metal | ❌ | 仅 macOS | 本环境不可用 |
| DX12 | ❌ | 仅 Windows | 本环境不可用 |

### 验证执行计划

| 步骤 | 操作 | 验证方式 | 预期 |
|:----:|------|---------|------|
| 0 | 替换前基准测试 | `go test -count=1 ./wgpu/hal/... 2>&1` | 记录基线 |
| 1 | 替换 `loader.go`（LoadLibrary/GetSymbol/Dlclose） | `go build ./wgpu/...` | 编译通过 |
| 2 | 替换 `signatures.go`（删除 PrepareCallInterface） | `go build ./wgpu/...` | 编译通过 |
| 3 | 替换 `commands_gen.go` 中 1 个函数 | `go test ./wgpu/hal/vulkan/...` | 单函数调用正确 |
| 4 | 批量替换 `commands_gen.go` 全部 565 次 | `go test ./wgpu/hal/vulkan/...` | 全量 Vulkan 测试通过 |
| 5 | 替换 `commands_manual.go` + `adapter.go` + `debug.go` | `go test ./wgpu/hal/vulkan/...` | 全部 Vulkan 测试通过 |
| 6 | 替换 GLES 后端（egl + gl context_linux） | `go test ./wgpu/hal/gles/...` | GLES 测试通过 |
| 7 | 替换 Software 后端（blit linux/wayland） | `go test ./wgpu/hal/software/...` | Software 测试通过 |
| 8 | 替换 Metal 后端（仅语法替换，无法运行测试） | `go build ./wgpu/...` | 编译通过 |
| 9 | 移除 goffi 依赖 | `go mod tidy; go build ./...` | 编译通过，无 goffi 引用 |
| 10 | 全量回归测试 | `go test -count=1 ./...` | 全部测试通过 |

### 各后端的测试入口

**Vulkan 测试（11 个测试文件）：**
```
wgpu/hal/vulkan/api_test.go
wgpu/hal/vulkan/bench_descriptor_test.go
wgpu/hal/vulkan/bench_hot_path_test.go
wgpu/hal/vulkan/command_nullguard_test.go
wgpu/hal/vulkan/compute_integration_test.go
wgpu/hal/vulkan/compute_test.go
wgpu/hal/vulkan/convert_test.go
wgpu/hal/vulkan/descriptor_test.go
wgpu/hal/vulkan/memory/buddy_test.go
wgpu/hal/vulkan/memory/types_test.go
wgpu/hal/vulkan/relay_test.go
wgpu/hal/vulkan/resource_test.go
```

**GLES 测试（9 个测试文件）：**
```
wgpu/hal/gles/adapter_open_test.go
wgpu/hal/gles/binding_test.go
wgpu/hal/gles/capabilities_test.go
wgpu/hal/gles/command_test.go
wgpu/hal/gles/compute_test.go
wgpu/hal/gles/convert_test.go
wgpu/hal/gles/integration_test.go  ← 关键：专门测试 FFI 调用正确性
wgpu/hal/gles/sampler_test.go
wgpu/hal/gles/version_test.go
```

**Software 测试（14 个测试文件）：**
```
wgpu/hal/software/compute_test.go
wgpu/hal/software/damage_test.go
wgpu/hal/software/draw_test.go
wgpu/hal/software/software_test.go
wgpu/hal/software/stats_test.go
wgpu/hal/software/raster/*_test.go (6个)
wgpu/hal/software/shader/*_test.go (8个)
```

---

## 阶段四：代码清理与优化

| 阶段项 | 状态 | 完成的事情 |
|--------|:----:|-----------|
| 清理测试文件 | ⏳ 待开始 | 处理引用已删除代码的测试文件 |
| 清理无用外部依赖 | ⏳ 待开始 | `go mod tidy` 清理 |
| 目录重命名 | ⏳ 待开始 | `gg/`, `wgpu/`, `naga/` → 更合适的名称 |
| 代码审查与优化 | ⏳ 待开始 | 检查 go vet 警告 |

---

## 阶段五：框架集成

| 阶段项 | 状态 | 完成的事情 |
|--------|:----:|-----------|
| 验证示例程序运行 | ⏳ 待开始 | 确保 gg/wgpu 示例可编译运行 |
| 编写 GPUI 封装层 | ⏳ 待开始 | 提供简洁的渲染接口 |
| 跨平台测试 | ⏳ 待开始 | Windows(DX12/Vulkan) + macOS(Metal) 测试 |

---

## 附录 A：purego 源码分析

### purego 核心 API

```go
// 动态库加载
func Dlopen(path string, mode int) (uintptr, error)     // 加载动态库
func Dlsym(handle uintptr, name string) (uintptr, error) // 获取函数指针
func Dlclose(handle uintptr) error                       // 关闭动态库

// 函数调用
func SyscallN(fn uintptr, args ...uintptr) (r1, r2, err uintptr)  // 低阶直接调用（最快）
func RegisterFunc(fptr any, cfn uintptr)                           // 高阶注册（反射，类型安全）
func RegisterLibFunc(fptr any, handle uintptr, name string)        // 注册+加载一步完成

// 回调
func NewCallback(fn any) uintptr  // 创建 C 回调函数指针

// ObjC（内置子包）
import "github.com/ebitengine/purego/objc"
objc.GetClass("NSView")
objc.RegisterName("alloc")
objc.Send[objc.ID](cls, sel)
```

### goffi vs purego 调用模式对比

**goffi（当前代码）：**
```go
// 每次调用需要：准备 CIF + 传参数数组
var cif types.CallInterface
ffi.PrepareCallInterface(&cif, types.DefaultCall, returnType, argTypes)
var result uint32
args := []unsafe.Pointer{unsafe.Pointer(&a), unsafe.Pointer(&b)}
_ = ffi.CallFunction(&cif, fn, unsafe.Pointer(&result), args)
```

**purego SyscallN（替换方案，性能最优）：**
```go
// 直接传 uintptr，无需准备 CIF
r1, _, _ := purego.SyscallN(fn, uintptr(a), uintptr(b))
result := uint32(r1)
```

**purego RegisterFunc（类型安全，有反射开销）：**
```go
var myFunc func(a uint32, b uint64) uint32
purego.RegisterFunc(&myFunc, fn)
result := myFunc(a, b)
```

---

## 附录 B：关键文件索引

### 需要替换 goffi 的文件列表

**Vulkan 后端（6 个文件，使用 `SyscallN`）：**
- `wgpu/hal/vulkan/vk/commands_gen.go` — 565 次 CallFunction（自动生成，改 `vk-gen` 代码生成器）
- `wgpu/hal/vulkan/vk/signatures.go` — 0 次 CallFunction（67 次 PrepareCallInterface，删除）
- `wgpu/hal/vulkan/vk/loader.go` — 2 次 CallFunction
- `wgpu/hal/vulkan/vk/commands_manual.go` — 3 次 CallFunction
- `wgpu/hal/vulkan/adapter.go` — 1 次 CallFunction
- `wgpu/hal/vulkan/debug.go` — 2 次 NewCallback

**Metal 后端（2 个文件，使用 `RegisterFunc` + `purego/objc`）：**
- `wgpu/hal/metal/objc.go` — 3 次 CallFunction + 13 次 NewCallback
- `wgpu/hal/metal/metal.go` — 2 次 CallFunction

**GLES 后端（4 个文件，混合 `SyscallN` + `RegisterFunc`）：**
- `wgpu/hal/gles/egl/egl.go` — 22 次 CallFunction
- `wgpu/hal/gles/egl/display.go` — 4 次 CallFunction
- `wgpu/hal/gles/egl/wayland_egl.go` — 3 次 CallFunction
- `wgpu/hal/gles/gl/context_linux.go` — 104 次 CallFunction（部分含 float 参数需 `RegisterFunc`）

**Software 后端（3 个文件，使用 `SyscallN`）：**
- `wgpu/hal/software/blit_linux.go` — 9 次 CallFunction
- `wgpu/hal/software/blit_darwin.go` — 10 次 CallFunction
- `wgpu/hal/software/blit_wayland.go` — 30 次 CallFunction + 3 次 NewCallback