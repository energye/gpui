# GPUI 开发计划

> 本文档记录开发阶段规划，随开发进展更新。
> 最后更新: 2026-07-11

---

## 冷启动知识 (AI/新人必读)

> 本章是**冷启动指南**，让 AI 或新开发者在无上下文情况下快速理解项目。
> 阅读本章后应能：理解架构、找到关键文件、编写代码、验证构建。

### 项目结构

```
gpui/
├── ui/                          # GUI 层 — 窗口、控件、帧调度
│   ├── control.go               # TGPUControl — OpenGL 控件封装 + 渲染管线
│   ├── frame_pump.go            # FramePump — 帧调度器 (goroutine + chan)
│   ├── window.go                # Window — LCL 窗口 + OpenGL 控件组合
│   ├── application.go           # Application — 应用入口
│   └── doc.go                   # 包文档
│
├── render/                      # 渲染层 — gg 2D 矢量图形引擎
│   ├── context.go               # Context — 主渲染 API (绘图、变换、裁剪)
│   ├── color.go                 # RGBA — 颜色类型 + Lerp 插值
│   ├── brush.go                 # Brush — SolidBrush, 渐变画刷
│   ├── scene/                   # 场景图 — 保留模式渲染
│   │   ├── builder.go           # SceneBuilder — 场景构建器
│   │   ├── shape.go             # Shape — Rect, Circle, Ellipse, RoundRect
│   │   ├── filter.go            # Filter — Blur, Shadow, ColorMatrix
│   │   └── layer.go             # Layer — 合成层 + ClipStack
│   ├── text/                    # 文字渲染 — OpenType 解析 + 字形缓存
│   ├── gl/                      # OpenGL 绑定 — purego 无 CGo
│   │   ├── gl.go                # GL 函数指针 + Init() + SwapInterval
│   │   └── swap_*.go            # (计划中) 平台特定 SwapInterval
│   ├── widget/                  # Widget 系统 — 占位包 (待实现)
│   │   └── widget.go            # Widget 接口占位
│   └── event/                   # Event 系统 — 占位包 (待实现)
│       └── event.go             # Event 接口占位
│
├── examples/                    # 示例程序
│   ├── dynamic_animation/       # 动态动画示例
│   ├── animation/               # 基础动画示例
│   ├── text/                    # 文字渲染示例
│   └── shadow_effects/          # 阴影效果示例
│
├── go.mod                       # 模块定义
├── DEVELOPMENT.md               # 本开发计划文档
└── README.md                    # 项目说明
```

### 核心依赖关系

```
gpui (本项目)
├── github.com/energye/lcl       # LCL 窗口系统 (Free Pascal 绑定)
│   └── 提供: 窗口、OpenGL 控件、事件回调、主线程调度
├── github.com/ebitengine/purego # 无 CGo FFI
│   └── 提供: Dlopen, Dlsym, RegisterLibFunc, RegisterFunc
├── github.com/gogpu/wgpu        # WebGPU 绑定
│   └── 提供: GPU 设备、命令编码、着色器编译
└── github.com/gogpu/gpucontext  # GPU 上下文
    └── 提供: 设备抽象、平台适配
```

**关键约束**:
- LCL 是 Free Pascal 绑定，事件回调在主线程执行
- purego 无 CGo，通过 Dlopen/Dlsym 加载原生库
- OpenGL context 绑定到创建它的线程，所有 GL 调用必须在同一线程

### 核心 API 速查

#### FramePump (帧调度器)

```go
// 文件: ui/frame_pump.go

// 创建
pump := NewFramePump(ctrl *TGPUControl) *FramePump

// 生命周期
pump.Start()                    // 启动 goroutine (幂等)
pump.Stop()                     // 停止 goroutine (幂等)

// 重绘请求 (任意 goroutine 安全)
pump.RequestRedraw()            // 请求一帧 (多次调用合并为一次)

// 动画管理
token := pump.StartAnimation()  // 引用计数 +1，返回生命周期句柄
token.Stop()                    // 引用计数 -1 (幂等)
pump.IsAnimating() bool         // 是否有活跃动画

// 内部机制
// - requestCh: chan struct{}(1) — 无锁合并
// - run(): goroutine 主循环 — select { <-requestCh → RunOnMainThreadSync(Invalidate) }
// - animCount: atomic.Int32 — 引用计数
```

#### TGPUControl (OpenGL 控件)

```go
// 文件: ui/control.go

// 创建
ctrl := NewGPUControl() *TGPUControl

// 绑定 LCL 控件 (自动启动 FramePump)
ctrl.Attach(glCtrl lcl.IOpenGLControl)

// 渲染回调
ctrl.SetOnRender(fn func(*render.Context))

// 帧请求 (委托给 FramePump)
ctrl.RequestRedraw()            // 等价于 ctrl.Pump().RequestRedraw()

// 动画
token := ctrl.StartAnimation()  // 等价于 ctrl.Pump().StartAnimation()
ctrl.StopAnimation()            // 停止所有动画

// 诊断
ctrl.FrameCount() uint64        // 帧计数
ctrl.LastFrameTime() time.Time  // 最近帧时间戳
ctrl.IsAnimating() bool         // 是否有活跃动画

// GL 操作
ctrl.MakeCurrent() bool         // 激活 GL 上下文
ctrl.ReleaseContext()           // 释放 GL 上下文
ctrl.SwapBuffers()              // 交换缓冲区

// 尺寸
ctrl.Width() int32              // 控件宽度
ctrl.Height() int32             // 控件高度
ctrl.SetSize(w, h int32)        // 设置尺寸
```

#### render.Context (渲染上下文)

```go
// 文件: render/context.go

ctx := render.NewContext(w, h int) *Context  // 创建
ctx.Close()                                   // 释放

// 绘图
ctx.Clear()
ctx.ClearWithColor(c RGBA)
ctx.DrawCircle(x, y, r float64)
ctx.DrawRect(x, y, w, h float64)
ctx.DrawRoundRect(x, y, w, h, rx, ry float64)
ctx.MoveTo(x, y float64)
ctx.LineTo(x, y float64)
ctx.Fill()
ctx.Stroke()

// 画刷
ctx.SetFillBrush(brush Brush)
ctx.SetStrokeBrush(brush Brush)
ctx.SetLineWidth(w float64)

// 文字
ctx.SetFont(face FontFace)
ctx.DrawString(s string, x, y float64)

// 变换
ctx.Push()                      // 保存状态
ctx.Pop()                       // 恢复状态
ctx.Translate(x, y float64)

// 输出
ctx.Pixmap() *Pixmap            // 获取像素数据
ctx.SavePNG(path string) error  // 保存为 PNG
```

### 代码约定

#### 命名规范

```
包名:      小写单词 (ui, render, gl, event)
导出类型:  PascalCase (TGPUControl, FramePump, AnimationToken)
导出方法:  PascalCase (RequestRedraw, StartAnimation, IsAnimating)
内部方法:  camelCase (requestRedraw, onPaint, doPresent)
常量:      GL_ 前缀 (GL_TEXTURE_2D, GL_RGBA)
文件名:    snake_case (frame_pump.go, control.go)
```

#### 文件组织

```
每个包一个目录
每个文件一个职责 (control.go = 控件逻辑, frame_pump.go = 帧调度)
平台特定代码用 build tag 或文件后缀 (_linux.go, _windows.go)
占位包用 doc.go 说明意图 (widget/, event/)
```

#### 错误处理

```go
// Init 返回 error，调用方必须检查
if err := gl.Init(); err != nil {
    return err
}

// FramePump 方法不返回 error (最佳努力)
pump.RequestRedraw()  // 无返回值

// AnimationToken.Stop() 幂等
token.Stop()  // 可多次调用
```

#### 线程安全约定

```
GL 操作:     必须在主线程 (MakeCurrent 后)
FramePump:   任意 goroutine 安全 (chan 合并)
LCL 回调:    在主线程执行
render.Context: 非线程安全，单线程使用
```

### 构建与运行

```bash
# 编译检查
go build ./ui/...                          # 编译 ui 包
go build ./render/...                      # 编译 render 包
go build ./examples/dynamic_animation/...  # 编译动画示例

# 运行示例
go run ./examples/dynamic_animation/...    # 运行动态动画
go run ./examples/animation/...            # 运行基础动画
go run ./examples/text/...                 # 运行文字渲染

# 全量编译
go build ./...                             # 编译所有包

# 注意: 运行需要图形环境 (X11/Wayland/Windows/macOS)
# 无图形环境只能编译，不能运行
```

### 关键设计模式

#### 1. FramePump 帧调度模式

```
onPaint (main thread)
  │
  └─ pump.RequestRedraw()  →  requestCh <- struct{}{}  (非阻塞)
                                  │
FramePump goroutine               │
  │                               │
  └─ <-requestCh  ←──────────────┘
       │
       └─ RunOnMainThreadSync(Invalidate)
              │
              └─ 主线程执行 Invalidate()  →  LCL 队列 OnPaint
                                              │
                                              └─ onPaint 运行
```

**关键**: onPaint **永远不直接调用 Invalidate**，所有重绘通过 FramePump goroutine 中转。

#### 2. 动画引用计数模式

```go
token1 := ctrl.StartAnimation()  // count=1, 开始渲染
token2 := ctrl.StartAnimation()  // count=2
token1.Stop()                    // count=1, 继续渲染
token2.Stop()                    // count=0, 停止渲染
```

多个动画独立启停，所有动画停止后自动停止渲染。

#### 3. 请求合并模式

```go
// 多个 goroutine 同时调用
go effectA.RequestRedraw()  // requestCh <- struct{}{} (成功)
go effectB.RequestRedraw()  // requestCh <- struct{}{} (channel 满，丢弃)
go effectC.RequestRedraw()  // requestCh <- struct{}{} (channel 满，丢弃)
// 结果: 只触发一次 Invalidate
```

### 已有资源清单

| 资源 | 位置 | 用途 |
|------|------|------|
| RGBA.Lerp() | render/color.go | 颜色线性插值 |
| SolidBrush.Lerp() | render/brush.go | 画刷颜色插值 |
| FilterBlur | render/scene/filter.go | 模糊滤镜 |
| FilterDropShadow | render/scene/filter.go | 阴影滤镜 |
| Shape.Contains() | render/scene/shape.go | 命中测试原语 |
| RunOnMainThreadSync | lcl/lcl/ | 主线程同步投递 |
| RunOnMainThreadAsync | lcl/lcl/ | 主线程异步投递 |
| OpenGLControl | lcl/lcl/ | OpenGL 控件接口 |
| SetSwapInterval | render/gl/gl.go | VSync 控制 |

### 常见陷阱

1. **onPaint 中调用 Invalidate** — 会死锁 (RunOnMainThreadSync 阻塞主线程)
2. **GL 操作不在主线程** — 会崩溃 (GL context 绑定线程)
3. **AnimationToken 忘记 Stop** — 动画永远不会停止，持续消耗 CPU
4. **dt 不 clamp** — 长时间 idle 后动画跳帧 (参考 gogpu 的 0.066s 上限)
5. **SavePNG 每帧调用** — 严重卡顿 (磁盘 IO 阻塞主线程)
6. **requestCh 不是 buffered(1)** — 请求丢失或阻塞

### 参考库关键文件

| 库 | 关键文件 | 参考点 |
|---|---------|--------|
| gogpu | app.go:runFrame() | 三模式帧循环 (IDLE/ANIMATING/CONTINUOUS) |
| gogpu | invalidator.go | chan struct{}(1) 合并模式 |
| gogpu | animation.go | AnimationController 引用计数 |
| gogpu | internal/thread/thread.go | 专用线程 + 消息队列 |
| gg | render/context.go | 即时模式绘图 API |
| gg | render/scene/builder.go | 场景图构建 |
| gg | render/text/ | 文字渲染管线 |
| LCL | lcl/customopenglcontrol.go | OpenGL 控件接口 |
| LCL | lcl/lcl_runonmain_thread.go | 主线程调度 API |

---

## 总体目标

基于 gg 渲染引擎 + LCL 窗口系统，构建跨平台 GPU 加速的 GUI 框架，对标 ant.design 级别的控件体系。

### 架构分层

```
┌─────────────────────────────────────────────────────┐
│  GUI Controls (Button, Input, Modal, Table, ...)    │  ← 目标
├─────────────────────────────────────────────────────┤
│  Effect System + State Machine + Event System       │  ← 待建
├─────────────────────────────────────────────────────┤
│  FramePump (帧调度基座)                              │  ← ✅ 已完成
├─────────────────────────────────────────────────────┤
│  gg 渲染引擎 (Context, Scene, Text, Filters)        │  ← ✅ 已有
├─────────────────────────────────────────────────────┤
│  LCL + OpenGL (窗口系统 + GL 上下文)                 │  ← ✅ 已有
└─────────────────────────────────────────────────────┘
```

### 设计原则

- **机制与策略分离**: FramePump 只提供帧调度机制，不管上面的 Effect/Widget 怎么实现
- **gogpu 模式移植**: Invalidator(chan 合并) + AnimationController(引用计数) + RunOnMainThreadSync(主线程投递)
- **onPaint 约束**: onPaint 永远不直接调用 Invalidate，所有重绘通过 FramePump goroutine 中转
- **纯 Go 无 CGo**: 通过 purego 调用原生 API

---

## 阶段规划

### Phase 1: FramePump 基座 ⬅️ 当前阶段

**状态**: ✅ 已完成

**目标**: 实现 goroutine 驱动的帧调度器，替代 atomic CAS 方案。

**子项**:

| # | 任务 | 状态 | 文件 |
|---|------|------|------|
| 1.1 | FramePump 核心 (chan 合并 + goroutine) | ✅ | `ui/frame_pump.go` |
| 1.2 | AnimationController (引用计数) | ✅ | `ui/frame_pump.go` |
| 1.3 | AnimationToken (生命周期句柄) | ✅ | `ui/frame_pump.go` |
| 1.4 | TGPUControl 集成 | ✅ | `ui/control.go` |
| 1.5 | Window 统一入口 | ✅ | `ui/window.go` |

**关键提醒**:
- FramePump goroutine 使用 `RunOnMainThreadSync` 投递 Invalidate，确保请求不丢失
- `requestCh` 是 buffered(1) channel，多次调用合并为一次
- `AnimationToken.Stop()` 是幂等的，安全多次调用
- `onPaint` 结尾通过 `pump.RequestRedraw()` 续帧，不直接调用 `Invalidate()`

---

### Phase 2: 帧时间与动画基础

**状态**: 📋 待开始

**目标**: 提供 delta time 和基础动画原语，为 Effect System 打基础。

**子项**:

| # | 任务 | 状态 | 说明 |
|---|------|------|------|
| 2.1 | Delta Time 计算 | 📋 | onPaint 中计算 dt，传递给 onUpdate 回调 |
| 2.2 | 帧率限制 (60fps cap) | 📋 | 可选的 minFrameDuration 节流 |
| 2.3 | Easing 函数库 | 📋 | ease-in, ease-out, ease-in-out, spring |
| 2.4 | Tween 引擎 | 📋 | 属性插值: float, RGBA, Vec2 |
| 2.5 | FPS 计数器 | 📋 | 诊断用，可选 |

**关键提醒**:
- 参考 gogpu 的 `onUpdate(dt float64)` 模式，dt 单位为秒
- dt 需要 clamp (参考 gogpu 的 0.066s 上限)，防止长时间 idle 后动画跳帧
- Easing 函数是 Effect System 的基础原语 (hover 渐变、focus ring 动画都依赖它)
- Tween 引擎需要与 FramePump 的帧节奏对齐

---

### Phase 3: Effect System

**状态**: 📋 待开始

**目标**: 实现可组合的视觉效果系统，支撑控件的 hover/press/focus 动画。

**子项**:

| # | 任务 | 状态 | 说明 |
|---|------|------|------|
| 3.1 | Effect 接口定义 | 📋 | `Update(dt) bool`, `Priority() int` |
| 3.2 | HoverEffect | 📋 | 颜色渐变、阴影变化 |
| 3.3 | PressEffect | 📋 | 缩放、颜色加深 |
| 3.4 | FocusEffect | 📋 | focus ring 渐变 |
| 3.5 | TransitionEffect | 📋 | 通用属性过渡 (opacity, color, position) |
| 3.6 | EffectChain | 📋 | 多个 Effect 组合、优先级排序 |

**关键提醒**:
- 每个 Effect 通过 `pump.RequestRedraw()` 请求帧，FramePump 自动合并
- Effect 需要持有控件的引用，以便修改控件状态 (hovered, pressed 等)
- `RGBA.Lerp()` 和 `SolidBrush.Lerp()` 已有，可直接用于颜色过渡
- Spring 物理效果可能需要独立的 dt 追踪 (不依赖帧率)

---

### Phase 4: Event System

**状态**: 📋 待开始

**目标**: 桥接 LCL 事件到控件树，实现事件分发和命中测试。

**子项**:

| # | 任务 | 状态 | 说明 |
|---|------|------|------|
| 4.1 | 事件类型定义 | 📋 | MouseEvent, KeyEvent, FocusEvent |
| 4.2 | LCL 事件桥接 | 📋 | 从 LCL 回调转换为内部事件 |
| 4.3 | 事件分发器 | 📋 | 控件树遍历、冒泡/捕获 |
| 4.4 | 命中测试 | 📋 | 基于 shape.Contains() 的控件树遍历 |
| 4.5 | 焦点管理 | 📋 | Tab 遍历、焦点环 |
| 4.6 | 手势识别 | 📋 | 长按、双击、拖拽 |

**关键提醒**:
- LCL 已有完整的事件回调 (OnMouseMove, OnMouseDown 等)，需要桥接
- `scene/shape.go` 已有 `Contains()` 方法 (Rect, Circle, Ellipse, RoundRect)
- 事件分发需要考虑 z-order 和裁剪区域
- 焦点管理需要与 Effect System 联动 (FocusEffect)

---

### Phase 5: State Machine

**状态**: 📋 待开始

**目标**: 为控件提供状态管理和状态转换驱动的动画。

**子项**:

| # | 任务 | 状态 | 说明 |
|---|------|------|------|
| 5.1 | ControlState 定义 | 📋 | hovered, pressed, focused, disabled, loading |
| 5.2 | 状态转换规则 | 📋 | 事件 → 状态变更 → Effect 触发 |
| 5.3 | 状态驱动渲染 | 📋 | 根据状态选择渲染路径 |
| 5.4 | 响应式状态 (可选) | 📋 | 信号/订阅模式 |

**关键提醒**:
- 状态转换是 Effect System 的触发源 (hover → HoverEffect.Start())
- 状态变更后需要 `pump.RequestRedraw()` 触发重绘
- 参考 React 的 useState / Flutter 的 StatefulWidget 模式

---

### Phase 6: Layout System

**状态**: 📋 待开始

**目标**: 实现 Flexbox 布局引擎，支持自动布局和响应式设计。

**子项**:

| # | 任务 | 状态 | 说明 |
|---|------|------|------|
| 6.1 | Box Model | 📋 | padding, margin, border |
| 6.2 | Flexbox 子集 | 📋 | flexDirection, justifyContent, alignItems |
| 6.3 | 约束求解器 | 📋 | min/max size, intrinsic size |
| 6.4 | 布局缓存 | 📋 | 脏标记、增量布局 |

**关键提醒**:
- 布局系统是纯 CPU 计算，不涉及渲染
- 参考 Yoga (Flexbox) / Taffy 的算法
- 需要与 Widget 树集成

---

### Phase 7: Theme System

**状态**: 📋 待开始

**目标**: 实现 Design Token 系统，支持主题切换和暗色模式。

**子项**:

| # | 任务 | 状态 | 说明 |
|---|------|------|------|
| 7.1 | Design Token 定义 | 📋 | colors, typography, spacing, shadows |
| 7.2 | Theme 结构体 | 📋 | Light/Dark 主题包 |
| 7.3 | 主题切换 | 📋 | 运行时切换、动画过渡 |
| 7.4 | 平台适配 | 📋 | DarkMode(), FontScale() 查询 |

**关键提醒**:
- gogpu 已有 `DarkMode()`, `FontScale()`, `HighContrast()` 等平台查询
- `render/color.go` 已有 `RGBA.Lerp()` 用于主题切换的颜色过渡
- Design Token 应该是静态的，不依赖运行时状态

---

### Phase 8: Widget System

**状态**: 📋 待开始

**目标**: 实现控件树和基础控件集。

**子项**:

| # | 任务 | 状态 | 说明 |
|---|------|------|------|
| 8.1 | Widget 接口 | 📋 | Render, Layout, HitTest, HandleEvent |
| 8.2 | Widget 树 | 📋 | 父子关系、遍历、脏传播 |
| 8.3 | Button 控件 | 📋 | 对标 ant.design Button |
| 8.4 | Input 控件 | 📋 | 对标 ant.design Input |
| 8.5 | Label 控件 | 📋 | 文本显示 |
| 8.6 | Container 控件 | 📋 | 布局容器 |
| 8.7 | 更多控件... | 📋 | Modal, Dropdown, Table, Tree 等 |

**关键提醒**:
- `widget/widget.go` 已有占位接口，需要扩展
- 控件需要组合 Phase 3-7 的所有能力
- 控件应该是可组合的 (Button 内嵌 Icon + Text + Spinner)

---

## 状态图例

| 符号 | 含义 |
|------|------|
| ✅ | 已完成 |
| 🔨 | 进行中 |
| 📋 | 待开始 |
| ⏸️ | 暂停 |
| ❌ | 已取消 |

---

## 依赖关系

```
Phase 1 (FramePump)
    ↓
Phase 2 (帧时间/动画基础)
    ↓
Phase 3 (Effect System) ← 依赖 Phase 2 的 Easing/Tween
    ↓
Phase 4 (Event System) ← 与 Phase 3 联动 (事件触发 Effect)
    ↓
Phase 5 (State Machine) ← 依赖 Phase 3 + 4
    ↓
Phase 6 (Layout System) ← 独立，但需要 Phase 5 的状态
    ↓
Phase 7 (Theme System) ← 独立
    ↓
Phase 8 (Widget System) ← 依赖所有前置 Phase
```

---

## 参考库

| 库 | 用途 | 参考点 |
|---|------|--------|
| gogpu | 平台层模式 | Invalidator, AnimationController, onUpdate(dt), EventSource |
| gg | 渲染引擎 | Context, Scene, Text, Filters, Color |
| ant.design | 控件设计 | 组件 API、交互规范、视觉标准 |
| Yoga | 布局算法 | Flexbox 约束求解 |
| Flutter | 架构参考 | Widget 树、状态管理、渲染管线 |

---

## 变更日志

| 日期 | 变更 |
|------|------|
| 2026-07-11 | 初始版本，Phase 1 已完成 |
