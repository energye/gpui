# GPUI - LCL OpenGL + gg 渲染引擎实现方案

## 1. 架构确认

### 最终架构

```
┌─────────────────────────────────────────────────────────────────────┐
│  GPUI                                                                │
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────────┐ │
│  │  GPU 加速模式 (可选)                                             │ │
│  │  import _ "github.com/energye/gpui/gpu"                         │ │
│  │                                                                  │ │
│  │  wgpu Instance (headless, 无 Surface)                            │ │
│  │    → Adapter → Device → Queue                                    │ │
│  │    → GPUShared.SetDeviceProvider(provider)                        │ │
│  │    → SDF Render Pipeline (GPU Compute Shader)   ✅ GPU 计算      │ │
│  │    → Coverage AdaptiveFiller (GPU Compute Shader) ✅ GPU 计算     │ │
│  └─────────────────────────────────────────────────────────────────┘ │
│                                    ↓                                 │
│  ┌─────────────────────────────────────────────────────────────────┐ │
│  │  gg 渲染引擎 (CPU 合成)                                         │ │
│  │                                                                  │ │
│  │  gg.Context                                                      │ │
│  │    ├── 路径绘制 (MoveTo/LineTo/CubicTo/Arc)    ↕ CPU/GPU         │ │
│  │    ├── SDF 形状 (圆/圆角矩形)                   ↕ 走 GPU 或 CPU  │ │
│  │    ├── 复杂路径填充 (Coverage)                  ↕ 走 GPU 或 CPU  │ │
│  │    ├── 渐变生成 (Linear/Radial/Sweep)           ✅ CPU            │ │
│  │    ├── 文字渲染 (TrueType/OpenType)             ✅ CPU            │ │
│  │    ├── 像素合成 (Blend)                         ✅ CPU            │ │
│  │    ├── 图层合成 (Layer)                         ✅ CPU            │ │
│  │    └── 裁剪 (Clip)                              ✅ CPU            │ │
│  │                                                                  │ │
│  │  输出: Pixmap.Data() → []uint8 (RGBA premul, 4bpp)              │ │
│  └─────────────────────────────────────────────────────────────────┘ │
│                                    ↓                                 │
│  ┌─────────────────────────────────────────────────────────────────┐ │
│  │  LCL OpenGL 控件 (GPU 显示)                                      │ │
│  │                                                                  │ │
│  │  TCustomOpenGLControl                                            │ │
│  │    ├── MakeCurrent()                → GL 上下文绑定              │ │
│  │    ├── gl.TexImage2D(Pixmap.Data()) → DMA 上传 (GPU, 不阻塞 CPU) │ │
│  │    ├── 全屏 Quad 纹理绘制           → GPU 片元着色器             │ │
│  │    └── SwapBuffers()                → 双缓冲交换 → 屏幕         │ │
│  └─────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────┘
```

### 两种模式源代码级确认

#### 模式 A: 纯 CPU 模式 (不导入 GPU 加速包)

**源码路径验证：**

```
用户代码:
  dc := gg.NewContext(800, 600)
  dc.DrawCircle(400, 300, 100)
  dc.Fill()
```

**gg 内部调用链：**
```
gg/context.go:117  NewContext() → 创建 SoftwareRenderer (CPU 渲染器)
gg/context.go:876  Fill()
  → doFill()                               (context.go:1704)
    → tryGPUFillWithMode(RasterizerAuto)   (context.go:1941)
      → tryGPUFill()                       (context.go:1574)
        → Accelerator() == nil             (accelerator.go:282)
        → return ErrFallbackToCPU          (context.go:1584-1585)
      → return false                       (context.go:1956)
    → c.renderer.Fill(c.pixmap, ...)       (context.go:1750) ← CPU 软件渲染
```

**关键源文件：**
- `gg/accelerator.go:282-285` — `Accelerator()` 返回 nil（无 GPU 包导入时）
- `gg/context.go:1583-1585` — `a == nil` → `ErrFallbackToCPU`
- `gg/context.go:1740-1750` — 透明回退到 CPU 渲染器

**结论：✅ 不导入 GPU 包时，gg 纯 CPU 工作，零 GPU 依赖。**

#### 模式 B: GPU 加速模式 (导入 GPU 加速包)

**源码路径验证：**

```
用户代码:
  import _ "github.com/energye/gpui/gpu"  // 启用 GPU 加速
  dc := gg.NewContext(800, 600)
  dc.DrawCircle(400, 300, 100)
  dc.Fill()
```

**gg 内部调用链：**
```
gg/gpu/gpu.go:28-38  init() → RegisterAccelerator(SDFAccelerator)
                                → RegisterCoverageFiller(AdaptiveFiller)

gg/context.go:117  NewContext() → 创建 SoftwareRenderer
gg/context.go:876  Fill()
  → doFill()                               (context.go:1704)
    → tryGPUFillWithMode(RasterizerAuto)   (context.go:1941)
      → tryGPUFill()                       (context.go:1574)
        → Accelerator() == SDFAccelerator  (accelerator.go:282)
        → tryGPUOp(a, a.FillShape, ...)    (context.go:1588)
          → a.FillShape(target, shape, paint)
            → SDF GPU Compute Shader ✅    (internal/gpu/sdf_gpu.go)
            → 写入 target.Data (Pixmap)    (context.go:1539-1546)
```

**关键源文件：**
- `gg/gpu/gpu.go:28-38` — 注册加速器
- `gg/internal/gpu/gpu_shared.go:183-261` — `SetDeviceProvider()` 注入 Device
- `gg/internal/gpu/gpu_shared.go:246-248` — 创建 SDF/Convex/Stencil 管线
- `gg/context.go:1539-1546` — `gpuRenderTarget()` 返回 `Pixmap.Data()` 作为目标缓冲区

**结论：✅ 导入 GPU 包后，SDF + Coverage 自动走 GPU Compute Shader，透明回退 CPU。**

---

## 2. 项目结构

```
gpui/
├── go.mod                          # 模块定义
├── go.sum
├── PLAN.md                         # 本文件
├── README.md                       # 项目说明
│
├── gpui/                           # GPUI 核心包
│   ├── control.go                  #   TGPUControl (LCL OpenGL + gg 封装)
│   ├── render.go                   #   渲染循环 (OnPaint 驱动)
│   ├── provider.go                 #   DeviceProvider (wgpu headless)
│   ├── options.go                  #   配置选项
│   └── doc.go                      #   包文档
│
├── gpu/                            # GPU 加速器 (可选导入)
│   ├── gpu.go                      #   init() 注册 SDFAccelerator + AdaptiveFiller
│   └── provider.go                 #   wgpu Instance/Device 初始化
│
├── internal/
│   └── gg/                         # gg 渲染库的内嵌副本 (迁移后)
│       ├── context.go              #   gg.Context 核心
│       ├── pixmap.go               #   Pixmap 像素缓冲
│       ├── path.go                 #   路径操作
│       ├── text.go                 #   文本渲染
│       ├── accelerator.go          #   加速器接口
│       ├── ...                     #   其他 gg 源文件
│       └── internal/               #   gg 内部实现
│           ├── gpu/                #   GPU 加速器实现 (SDF, Coverage)
│           └── ...
│
└── examples/
    ├── basic/                      # 基础示例: 纯 CPU 模式
    │   └── main.go
    ├── gpu_accel/                  # GPU 加速示例
    │   └── main.go
    └── text/                       # 文字渲染示例
        └── main.go
```

---

## 3. 实现阶段

### 阶段 0: 项目骨架搭建

**目标：** 可编译的空项目，依赖全部就绪

**状态变更：** ❌ 空项目 → ✅ 编译通过

**具体任务：**

| # | 任务 | 涉及文件 | 验证方式 |
|---|------|---------|---------|
| 0.1 | 更新 go.mod，添加 gg/wgpu/gpucontext/gputypes 依赖 | `go.mod` | `go mod tidy` 成功 |
| 0.2 | 确认 gg 的 go 版本要求 (1.25) 与 LCL (1.20) 的兼容性，调整 go.mod | `go.mod` | `go build ./...` 成功 |
| 0.3 | 创建目录结构 | — | `ls -R` 目录完整 |
| 0.4 | 创建 `gpui/doc.go` 包文档 | `gpui/doc.go` | `go vet ./gpui/...` |

**依赖配置：**
```
require (
    github.com/energye/lcl v1.0.9
    github.com/gogpu/gg v0.50.4
    github.com/gogpu/wgpu v0.30.10
    github.com/gogpu/gpucontext v0.21.0
    github.com/gogpu/gputypes v0.5.1
)
```

**完成标志：** `go build ./...` 零错误

---

### 阶段 1: LCL OpenGL 控件封装

**目标：** 创建 `TGPUControl`，能在 LCL 窗口中显示 OpenGL 内容（纯色清除）

**状态变更：** 📦 空控件 → 🖥️ 显示 OpenGL 清屏颜色

**具体任务：**

| # | 任务 | 涉及文件 | 源码级验证 |
|---|------|---------|-----------|
| 1.1 | 实现 `TGPUControl` 结构体，嵌入 LCL 的 `TCustomOpenGLControl` | `gpui/control.go` | LCL 的 `TCustomOpenGLControl` 提供 `MakeCurrent()`、`SwapBuffers()`、`Handle()`、`SetOnPaint()` |
| 1.2 | 实现 `OnPaint` 事件处理：清屏 + SwapBuffers | `gpui/control.go` | `MakeCurrent()` → `gl.Clear(GL_COLOR_BUFFER_BIT)` → `SwapBuffers()` |
| 1.3 | 实现 `Resize` 处理：更新视口 | `gpui/control.go` | `glViewport()` 在每次 `OnPaint` 时根据控件大小设置 |
| 1.4 | 封装 OpenGL 函数加载（purego 加载 OpenGL 函数表） | `gpui/gl/` | 参考 `ebitengine/purego` 或 `gogpu/wgpu/hal/gles/gl` 的加载方式 |
| 1.5 | 创建基本示例，验证控件在 Form 中显示 | `examples/basic/main.go` | 运行后看到彩色清屏窗口 |

**关键源码参考：**
- LCL: `lcl/customopenglcontrol.go` 第 18-67 行 — `ICustomOpenGLControl` 接口
- LCL: `lcl/customopenglcontrol.go` 第 346-349 行 — `NewCustomOpenGLControl()` 构造函数
- wgpu: `hal/gles/gl/context.go` — OpenGL 函数表的 purego 加载方式

**完成标志：** 运行示例，窗口显示纯色背景（OpenGL 清屏）

---

### 阶段 2: gg 渲染引擎迁移

**目标：** 将 gg 的 CPU 渲染引擎迁移到 `internal/gg/`，并集成到 `TGPUControl`

**状态变更：** 🖥️ OpenGL 清屏 → 🎨 显示 gg 绘制的 2D 图形

**具体任务：**

| # | 任务 | 涉及文件 | 源码级验证 |
|---|------|---------|-----------|
| 2.1 | 将 gg 的 CPU 渲染核心源文件复制到 `internal/gg/` | `internal/gg/*.go` | 排除 `gg/gpu.go` 和 `internal/gpu/`（GPU 加速器独立） |
| 2.2 | 调整 import 路径，从 `github.com/gogpu/gg` 改为 `github.com/energye/gpui/internal/gg` | `internal/gg/*.go` | `go vet ./internal/gg/...` 通过 |
| 2.3 | 确认 `gg.NewContext()` 创建纯 CPU 渲染管线 | `internal/gg/context.go:117` | `NewContext()` → `NewPixmap()` + `NewSoftwareRenderer()` |
| 2.4 | 实现 `TGPUControl.SetGGContext()` 设置 gg 渲染上下文 | `gpui/control.go` | 用户在 `OnPaint` 中使用 gg API 绘制 |
| 2.5 | 实现 Pixmap → OpenGL 纹理的显示管线 | `gpui/render.go` | `MakeCurrent()` → `gl.TexImage2D(pixmap.Data())` → Quad → `SwapBuffers()` |
| 2.6 | 更新示例：在 `OnPaint` 中用 gg 绘制圆形 | `examples/basic/main.go` | 运行后看到 gg 绘制的圆形 |

**Pixmap → OpenGL 纹理管线验证：**
```go
// gpui/render.go — 核心显示管线
func (c *TGPUControl) present() {
    c.MakeCurrent(true)
    defer c.RestoreOldOpenGLControl()

    // 获取 gg 渲染结果
    data := c.ggCtx.Pixmap().Data()  // []uint8, RGBA premul, 4bpp
    w := c.ggCtx.Width()
    h := c.ggCtx.Height()

    // GPU 上传 + 显示 (全部在 GPU 上)
    gl.BindTexture(gl.TEXTURE_2D, c.texture)
    gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, w, h, 0, gl.RGBA, gl.UNSIGNED_BYTE, data)
    // → DMA 引擎异步上传，不阻塞 CPU
    // → 绘制全屏 Quad
    gl.DrawArrays(gl.TRIANGLE_STRIP, 0, 4)
    // → SwapBuffers → 屏幕
    c.SwapBuffers()
}
```

**gg 源文件迁移清单 (CPU 核心)：**
```
context.go, pixmap.go, path.go, path_builder.go, path_ops.go, path_svg.go
paint.go, painter.go, brush.go, brush_custom.go
color.go, matrix.go, point.go, vec.go
curve.go, stroke.go, dash.go, solver.go
gradient.go, gradient_linear.go, gradient_radial.go, gradient_sweep.go
pattern.go, mask.go
text.go, text_mode.go
accelerator.go, coverage_filler.go, renderer.go, software.go
sdf.go, sdf_accelerator.go, shapes.go, shape_detect.go
shapes.go, pipeline_mode.go, rasterizer_mode.go
options.go, logger.go, doc.go
```

**完成标志：** 运行示例，窗口显示 gg 绘制的彩色圆形

---

### 阶段 3: GPU 加速模式

**目标：** 实现 `gpu/gpu.go` 包，导入后自动启用 GPU 加速

**状态变更：** 🎨 CPU 渲染 → ⚡ GPU 加速的 SDF + Coverage

**具体任务：**

| # | 任务 | 涉及文件 | 源码级验证 |
|---|------|---------|-----------|
| 3.1 | 实现 `gpu/provider.go`：创建 wgpu Instance/Adapter/Device (headless) | `gpu/provider.go` | `wgpu.CreateInstance(nil)` → `RequestAdapter(nil)` → `RequestDevice(nil)` |
| 3.2 | 实现 `DeviceProvider` 接口 (约 30 行) | `gpu/provider.go` | 实现 `Device()` / `Queue()` / `SurfaceFormat()` / `Adapter()` / `AdapterInfo()` |
| 3.3 | 实现 `gpu/gpu.go`：init() 中调用 `gg.SetDeviceProvider` | `gpu/gpu.go` | 参考 `gg/gpu/gpu.go:28-38` 的注册机制 |
| 3.4 | 迁移 gg 的 GPU 加速器源文件到 `internal/gg/internal/gpu/` | `internal/gg/internal/gpu/*.go` | 排除与 wgpu 窗口相关的代码，只保留 headless compute 部分 |
| 3.5 | 创建 GPU 加速示例 | `examples/gpu_accel/main.go` | 运行后看到 GPU 加速的复杂路径渲染 |

**关键源码参考：**
```go
// gpu/provider.go — DeviceProvider 实现
type gpuiDeviceProvider struct {
    device *wgpu.Device
    queue  *wgpu.Queue
}

func (p *gpuiDeviceProvider) Device() gpucontext.Device {
    return wgpu.DeviceToHandle(p.device)
}
func (p *gpuiDeviceProvider) Queue() gpucontext.Queue {
    return wgpu.QueueToHandle(p.queue)
}
func (p *gpuiDeviceProvider) SurfaceFormat() gputypes.TextureFormat {
    return gputypes.TextureFormatUndefined  // headless 模式
}
func (p *gpuiDeviceProvider) Adapter() gpucontext.Adapter {
    return gpucontext.Adapter{}  // 可选的，传 nil
}
func (p *gpuiDeviceProvider) AdapterInfo() gputypes.AdapterInfo {
    info := p.device.Adapter().Info()
    return gputypes.AdapterInfo{
        Name:       info.Name,
        DeviceType: info.DeviceType,
        Backend:    info.Backend,
    }
}

// gpu/gpu.go — GPU 加速器注册
func init() {
    // 创建 wgpu headless 设备
    instance, _ := wgpu.CreateInstance(nil)
    adapter, _ := instance.RequestAdapter(nil)
    device, _ := adapter.RequestDevice(nil)
    queue := device.Queue()

    // 注入 DeviceProvider
    provider := &gpuiDeviceProvider{device: device, queue: queue}
    gg.SetDeviceProvider(provider)
}
```

**完成标志：** GPU 加速示例中，SDF 形状 (圆/圆角矩形) 和复杂路径填充使用 GPU Compute Shader

---

### 阶段 4: 文本渲染

**目标：** 完整集成 gg 的文本渲染引擎，支持 CPU 文字渲染和 GPU MSDF 文字渲染

**状态变更：** 🎨 图形渲染 → 📝 图形 + 文字渲染

**具体任务：**

| # | 任务 | 涉及文件 | 源码级验证 |
|---|------|---------|-----------|
| 4.1 | 迁移 gg 文本引擎源文件到 `internal/gg/text/` | `internal/gg/text/*.go` | 包括 TrueType 解析、OpenType 布局、字形渲染 |
| 4.2 | 实现 `TGPUControl.SetFont()` 设置字体 | `gpui/control.go` | 调用 `gg.Context.SetFont()` |
| 4.3 | 验证 CPU 文字渲染路径 | `internal/gg/text/draw.go:13` | `Draw()` → `drawSourceFace()` → `drawGlyphs()` → `rasterize()` → `draw.DrawMask()` |
| 4.4 | 验证 GPU MSDF 文字渲染 (可选) | `internal/gg/text.go:101` | `tryGPUText()` → `GPUTextAccelerator` 接口 |
| 4.5 | 创建文字渲染示例 | `examples/text/main.go` | 显示各种字体、大小、颜色的文字 |

**gg 文本引擎源文件迁移清单：**
```
text/*.go — 全部
text/msdf/*.go — MSDF 生成器
text/cache/*.go — 缓存
text/emoji/*.go — Emoji 支持
```

**完成标志：** 文字渲染示例显示各种文字，CPU 路径完全工作

---

### 阶段 5: 动画 + 事件集成

**目标：** 实现动画帧驱动、窗口 Resize 处理、帧率控制

**状态变更：** 🖼️ 静态渲染 → 🎬 动画渲染

**具体任务：**

| # | 任务 | 涉及文件 | 源码级验证 |
|---|------|---------|-----------|
| 5.1 | 实现 `SetOnRender(fn)` 回调接口 | `gpui/control.go` | 用户提供 `func(*gg.Context)` 渲染回调 |
| 5.2 | 实现 LCL `TTimer` 驱动动画帧 | `gpui/render.go` | 定时器触发 → `MakeCurrent()` → 用户回调 → `present()` → `SwapBuffers()` |
| 5.3 | 实现 `OnResize` 事件：重建 gg.Context | `gpui/control.go` | LCL 的 `SetOnResize()` → `gg.NewContext(newW, newH)` |
| 5.4 | 实现帧率控制 (FPS 限制) | `gpui/render.go` | 可选 V-Sync 或固定帧率 |
| 5.5 | 更新示例：动画旋转的图形 | `examples/basic/main.go` | 运行后看到旋转的动画 |

**关键源码参考：**
```go
// gpui/render.go — 动画帧驱动
func (c *TGPUControl) startAnimation() {
    c.timer = lcl.NewTimer(c)
    c.timer.SetInterval(16)  // ~60fps
    c.timer.SetOnTimer(func(sender lcl.IObject) {
        if c.onRender != nil {
            c.ggCtx.Clear()
            c.onRender(c.ggCtx)     // 用户绘制
            c.present()              // 显示
        }
    })
    c.timer.SetEnabled(true)
}
```

**完成标志：** 动画示例流畅运行 60fps

---

### 阶段 6: 优化 + 完善

**目标：** 性能优化、纹理缓存、增量更新、文档完善

**状态变更：** 🎬 动画 → ⚡ 优化完善

**具体任务：**

| # | 任务 | 涉及文件 | 说明 |
|---|------|---------|------|
| 6.1 | OpenGL 纹理缓存（避免每帧重新创建纹理） | `gpui/render.go` | 只在尺寸变化时重建纹理 |
| 6.2 | 增量更新（仅重绘脏区域） | `gpui/render.go` | 配合 gg 的 damage tracking |
| 6.3 | 纹理池（重用纹理对象） | `gpui/render.go` | 减少内存分配 |
| 6.4 | HiDPI 支持 | `gpui/control.go` | 通过 `deviceScale` 参数 |
| 6.5 | 编写 README 和 API 文档 | `README.md`, `gpui/doc.go` | 完整的使用说明 |
| 6.6 | 编写单元测试 | `gpui/*_test.go` | 核心功能测试 |

**完成标志：** 所有示例运行流畅，文档完整

---

## 4. 阶段依赖关系

```
阶段 0: 项目骨架
  │
  ▼
阶段 1: LCL OpenGL 控件封装  ← 必须先有 LCL 控件骨架
  │
  ▼
阶段 2: gg 渲染引擎迁移  ← 必须有 Pixmap 才能显示
  │
  ├────────────────────┐
  ▼                    ▼
阶段 3: GPU 加速      阶段 4: 文本渲染
  │                    │
  └────────┬───────────┘
           ▼
      阶段 5: 动画 + 事件
           │
           ▼
      阶段 6: 优化完善
```

- 阶段 1 和 2 可以并行（但 2 需要 1 来显示）
- 阶段 3 和 4 可以并行
- 阶段 5 需要 3 和 4 都完成
- 阶段 6 在所有之后

---

## 5. 核心接口设计

### TGPUControl 接口

```go
type TGPUControl struct {
    lcl.TCustomOpenGLControl  // 嵌入 LCL OpenGL 控件

    // gg 渲染引擎
    ggCtx      *gg.Context
    pixmap     *gg.Pixmap
    width      int
    height     int

    // OpenGL 显示资源
    texture    uint32       // OpenGL 纹理 ID
    vao        uint32       // VAO (OpenGL Core Profile)
    vbo        uint32       // 全屏 Quad 顶点缓冲

    // 动画
    timer      lcl.ITimer

    // 用户回调
    onRender   func(*gg.Context)
}

func NewGPUControl(owner lcl.IComponent) *TGPUControl
func (c *TGPUControl) SetOnRender(fn func(*gg.Context))
func (c *TGPUControl) Resize(width, height int)
func (c *TGPUControl) StartAnimation()
func (c *TGPUControl) StopAnimation()
func (c *TGPUControl) Present()  // 手动触发一帧
```

### GPU 加速器包

```go
// gpu/gpu.go — 可选导入，启用 GPU 加速
package gpu

func init() {
    // 1. 创建 wgpu Instance (headless)
    // 2. 创建 Adapter → Device
    // 3. 创建 GPUShared
    // 4. 调用 gg.SetDeviceProvider(provider)
    // 5. 注册 SDFAccelerator + AdaptiveFiller
}
```

---

## 6. 技术风险与应对

| 风险 | 影响 | 应对方案 |
|------|------|---------|
| gg 的 go 版本要求 1.25，LCL 是 1.20 | 编译失败 | 统一 go.mod 中的 go 版本为 1.25 |
| wgpu 需要 Vulkan 驱动 | 无 GPU 加速 | 自动降级到纯 CPU 模式 |
| OpenGL 函数表加载 | 无法渲染 | 参考 wgpu 的 GLES backend 的 purego 加载方式 |
| gg 的 import 路径大规模重写 | 工作量大 | 使用 `sed` 批量替换，逐一验证编译 |
| wgpu GLES backend 的 compute shader 支持度 | 部分 GPU 降级 | 自动检测 `supportsCompute`，不支持则走 CPU |

---

## 7. 阶段完成标志汇总

| 阶段 | 完成标志 | 预计代码量 |
|------|---------|-----------|
| 0 | `go build ./...` 零错误 | ~50 行 |
| 1 | 运行示例显示纯色 OpenGL 窗口 | ~200 行 |
| 2 | 运行示例显示 gg 绘制的圆形 | ~500 行 (迁移) + ~200 行 (胶水代码) |
| 3 | GPU 加速示例中 SDF 和 Coverage 走 GPU | ~300 行 |
| 4 | 文字渲染示例显示各种文字 | ~100 行 (迁移) + ~100 行 (胶水代码) |
| 5 | 动画示例 60fps 流畅运行 | ~200 行 |
| 6 | 所有示例运行流畅，文档完整 | ~300 行 |
| **总计** | | **~1950 行** |