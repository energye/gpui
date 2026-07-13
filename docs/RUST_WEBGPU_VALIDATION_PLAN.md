# Rust WebGPU 验证计划

> 目标：验证 `gpu/rwgpu -> gpu/webgpu -> render` 的 Rust `wgpu-native` 路线在 GPUI 中可真实运行，并为后续接入 Lazarus LCL surface 提供固定基线。

## 背景

当前迁移后的分层为：

```text
render
  -> gpu/webgpu        # GPUI WebGPU API 门面，原 gogpu/wgpu
  -> gpu/rwgpu         # Rust wgpu-native FFI 绑定，原 go-webgpu/webgpu/wgpu
  -> ffi               # GPUI purego FFI 兼容层
  -> libwgpu_native.so
```

`go test -tags rust ./gpu/webgpu ./render` 已能通过编译层验证，但还缺少真实动态库运行验证、提交/readback 验证、float ABI 验证和 LCL surface 验证。

## 总体原则

1. 先验证 `gpu/rwgpu` 底层 FFI，再验证 `gpu/webgpu` 门面，最后验证 `render` 和 LCL 集成。
2. 每一步必须能独立失败并给出明确错误，不能用上层 fallback 掩盖底层问题。
3. Rust backend 测试不得依赖 `NewDeviceFromHAL`，必须走 `CreateInstance -> RequestAdapter -> RequestDevice`。
4. 所有 native 动态库路径必须显式记录，优先使用 `WGPU_NATIVE_PATH` 或项目根目录 `lib/`。
5. 涉及 float 参数的 API 必须单独验证，不能只靠普通 triangle/clear pass 间接推断。

## 阶段 1：放置并加载 `libwgpu_native.so`

### 目标

确认 `gpu/rwgpu.Init()` 能找到并加载 `wgpu-native` 动态库。

### 推荐放置位置

Linux:

```text
/home/yanghy/app/projects/gogpu/gpui/lib/libwgpu_native.so
```

或使用显式路径：

```bash
export WGPU_NATIVE_PATH=/absolute/path/to/libwgpu_native.so
```

当前加载顺序见 `gpu/rwgpu/wgpu.go`：

```text
1. WGPU_NATIVE_PATH
2. ./lib/libwgpu_native.so
3. ./libwgpu_native.so
4. 系统默认动态库搜索路径
```

### 建议新增测试

文件：

```text
gpu/rwgpu/native_load_test.go
```

Build tag：

```go
//go:build rustnative
```

测试内容：

```text
TestNativeLoad
  - 调用 rwgpu.Init()
  - 失败时输出 WGPU_NATIVE_PATH 和当前工作目录
```

### 验证命令

```bash
WGPU_NATIVE_PATH=/absolute/path/to/libwgpu_native.so \
env GOCACHE=/tmp/gpui-go-cache go test -tags rustnative ./gpu/rwgpu -run TestNativeLoad -v
```

### 完成标准

- `rwgpu.Init()` 返回 nil。
- 不再出现 `native library not loaded or failed to initialize`。
- 测试日志能明确显示实际使用的动态库路径。

## 阶段 2：最小创建 device 测试

### 目标

确认 Rust `wgpu-native` 能通过 GPUI FFI 创建 instance、adapter、device、queue。

### 建议新增测试

文件：

```text
gpu/rwgpu/native_device_test.go
```

Build tag：

```go
//go:build rustnative
```

测试内容：

```text
TestNativeCreateDevice
  - rwgpu.Init()
  - rwgpu.CreateInstance(nil)
  - instance.RequestAdapter(...)
  - adapter.RequestDevice(...)
  - device.Queue()
  - adapter.Info()
  - adapter.Limits()
  - device.Limits()
```

### 验证命令

```bash
WGPU_NATIVE_PATH=/absolute/path/to/libwgpu_native.so \
env GOCACHE=/tmp/gpui-go-cache go test -tags rustnative ./gpu/rwgpu -run TestNativeCreateDevice -v
```

### 完成标准

- device 和 queue 非 nil。
- adapter info 能输出 backend、device type、vendor/device id。
- limits/features 查询不 panic、不返回明显空结构。

## 阶段 3：clear pass + submit

### 目标

确认 command encoder、render pass、texture view、queue submit 的主渲染链路可用。

### 建议新增测试

文件：

```text
gpu/rwgpu/native_render_test.go
```

Build tag：

```go
//go:build rustnative
```

测试内容：

```text
TestNativeClearPassSubmit
  - 创建 16x16 RGBA8Unorm texture
  - usage: RenderAttachment | CopySrc
  - CreateView
  - CreateCommandEncoder
  - BeginRenderPass，LoadOpClear 清为固定颜色
  - End
  - Finish
  - Queue.Submit
```

如果当前 `gpu/rwgpu` readback 路径可用，继续：

```text
  - CopyTextureToBuffer
  - MapAsync/Map
  - 检查至少一个像素等于 clear color
```

### 验证命令

```bash
WGPU_NATIVE_PATH=/absolute/path/to/libwgpu_native.so \
env GOCACHE=/tmp/gpui-go-cache go test -tags rustnative ./gpu/rwgpu -run TestNativeClearPassSubmit -v
```

### 完成标准

- command buffer 能 finish。
- submit 返回无错误。
- 如启用 readback，像素值与 clear color 一致。
- 失败时能区分是 texture、render pass、submit、copy、map 哪一步失败。

## 阶段 4：验证 `SetViewport` float ABI

### 目标

确认 GPUI `ffi` 通过 purego 调用带直接 `float32` 参数的 C 函数时 ABI 正确。

重点 API：

```text
wgpuRenderPassEncoderSetViewport
```

当前调用点：

```text
gpu/rwgpu/render.go
```

### 风险

`SetViewport` 参数是 `float32`，不能只把 `math.Float32bits(x)` 当作整数参数就认定 ABI 正确。不同平台 ABI 对 float 参数可能使用浮点寄存器。

### 建议新增测试

文件：

```text
gpu/rwgpu/native_viewport_abi_test.go
```

Build tag：

```go
//go:build rustnative
```

测试内容：

```text
TestNativeSetViewportABI
  - 创建 32x16 render target
  - 设置 viewport 为右半区域，例如 x=16, y=0, width=16, height=16
  - clear 或绘制覆盖色
  - readback
  - 验证左半区域未受影响，右半区域受影响
```

如果 clear pass 无法受 viewport 限制，应使用最小 triangle/quad pipeline：

```text
  - viewport 设置为右半区域
  - draw full-screen quad/triangle
  - readback 左右半区差异
```

### 可能修复方向

如果测试失败，优先修 `gpu/rwgpu` 的 Proc 调用层：

```text
方案 A：为 SetViewport 增加专用 typed purego call
方案 B：扩展 ffi.CallFunction 支持真实 argTypes，并让 loader 为该符号声明 float 参数
方案 C：为所有已知 wgpu-native 函数生成 typed binding
```

短期建议先做方案 A，验证成本最低。

### 验证命令

```bash
WGPU_NATIVE_PATH=/absolute/path/to/libwgpu_native.so \
env GOCACHE=/tmp/gpui-go-cache go test -tags rustnative ./gpu/rwgpu -run TestNativeSetViewportABI -v
```

### 完成标准

- viewport 对渲染区域产生正确裁剪/映射效果。
- 测试能在 Linux amd64 上稳定通过。
- 如果修了 FFI，必须保留回归测试。

## 阶段 5：接 Lazarus LCL surface handle

### 目标

确认 Rust WebGPU backend 可以基于 Lazarus LCL 窗口句柄创建 surface，并 present 到真实控件窗口。

### 前置条件

- 阶段 1-4 已通过。
- LCL Go 绑定能提供平台窗口句柄。
- 明确当前平台：
  - Linux X11
  - Linux Wayland
  - Windows HWND
  - macOS NSView/CAMetalLayer

### 建议新增接口层

位置建议：

```text
gpu/context/window.go
gpu/webgpu/surface_lcl_*.go
```

或在 LCL 绑定库中实现 `gpu/context` 已有抽象，避免 `render` 直接依赖 LCL。

### 最小验证程序

建议新增 demo，而不是单元测试：

```text
examples/lcl_webgpu_clear/
```

流程：

```text
1. 创建 LCL 窗口/控件
2. 获取 native window handle
3. webgpu.CreateInstance
4. instance.CreateSurfaceFromLCLHandle(...)
5. RequestAdapter(CompatibleSurface)
6. RequestDevice
7. Configure surface
8. 每帧 GetCurrentTexture -> clear pass -> Submit -> Present
```

### 验证命令

根据 LCL 项目实际命令补充，例如：

```bash
WGPU_NATIVE_PATH=/absolute/path/to/libwgpu_native.so \
env GOCACHE=/tmp/gpui-go-cache go run ./examples/lcl_webgpu_clear
```

### 完成标准

- LCL 窗口能显示稳定 clear color。
- resize 后 surface 能重新 configure。
- 关闭窗口时 device/surface/texture 不 panic。
- X11/Wayland/Windows/macOS 至少明确支持矩阵和未支持原因。

## 推荐执行顺序

```text
1. TestNativeLoad
2. TestNativeCreateDevice
3. TestNativeClearPassSubmit
4. TestNativeSetViewportABI
5. lcl_webgpu_clear demo
6. render 接入 Rust backend 的 smoke
```

## 不在本阶段解决

- `render` 性能优化。
- 完整 Ant Design 控件渲染能力。
- GPU 资源 LRU。
- 文本 atlas 优化。
- compute/vello 完整质量修复。

这些应在 Rust WebGPU native 链路稳定后再进入 `OPTIMIZATION_PLAN.md` 的优化阶段。

## 固定架构前必须通过的门禁

```bash
# 不需要 native 动态库，只验证包图和 wrapper 编译
env GOCACHE=/tmp/gpui-go-cache go test -tags rust ./gpu/webgpu ./render

# 需要 native 动态库
WGPU_NATIVE_PATH=/absolute/path/to/libwgpu_native.so \
env GOCACHE=/tmp/gpui-go-cache go test -tags rustnative ./gpu/rwgpu -run 'TestNative(Load|CreateDevice|ClearPassSubmit|SetViewportABI)' -v
```

全部通过后，才能认为 Rust WebGPU backend 可以作为 GPUI 的固定渲染后端继续向 LCL surface 和 render 优化推进。
