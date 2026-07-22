# UI App Shell 实现方案 — 按需帧 × 第三方窗口挂载 × 统一入口 × 多窗口

> 版本：1.1 | 日期：2026-07-21  
> 关联：[`UI_FRAMEWORK_MAP.md`](./UI_FRAMEWORK_MAP.md) · [`SURFACE_LIFECYCLE_SKIA_FLUTTER.md`](./SURFACE_LIFECYCLE_SKIA_FLUTTER.md) · [`S5_WIDGET_ENTRY.md`](./S5_WIDGET_ENTRY.md)  
> 状态：**Phase 0–1 落地中** — 按需帧对齐 gogpu（`ui/app` + `Tree` Ticker + `Host.WaitEvents`）

---

## 0. 一句话目标

提供 **唯一应用入口 `ui/app`**，使 gpui 控件树可：

1. **自建窗口**（内置 `platform` Host），或  
2. **挂载到任意第三方跨平台窗口**（只消费对方的句柄 + 事件 + 可选 Surface），  
3. **多窗口并行**（每窗一 `WindowSession`，进程级共享 Device/调度），  
4. **按需帧**（Flutter 向：无脏 / 无 Ticker / 无 expose → 不 Layout、不 Paint、不 Present）。

```text
core 管树与脏 · platform 管 SPI · app 管会话与调度 · render 只管像素
```

---

## 1. 背景与问题

### 1.1 现状

| 层 | 已有 | 缺口 |
|----|------|------|
| `ui/core` | `MarkNeedsLayout/Paint`、`Tree.Dirty`、`TickClock` | `Frame` 无条件 Layout+Paint；`TickClock` 无脑 MarkDirty；无 Ticker 注册表 |
| `ui/platform` | `Host`、`PumpEvents`、`RequestRedraw`、`EventRedraw`、`NewHost` | Pump 非阻塞忙等；`Dispatch` 忽略 Redraw；无第三方挂载 SPI；无多窗 |
| examples | 各自 for 循环 + `exboot` GPU 引导 | 每圈必 Present；GPU/loop 散落；单窗假设 |
| `gpu/rwgpu` | Xlib/HWND/Metal/Wayland `CreateSurfaceFrom*` | 未收成 app 层挂载 API |

### 1.2 需求合集

| ID | 需求 | 说明 |
|----|------|------|
| R1 | **按需帧** | 仅触发绘制时才 layout/paint/present；静态 idle 零刷屏 |
| R2 | **第三方窗口挂载** | 挂到任意三方跨平台窗口库（GLFW、fyne、wails、自研壳…） |
| R3 | **保留统一入口** | 业务只认一个 `app` 包 API，不复制 smoke 循环 |
| R4 | **多窗口** | 多 `WindowSession`；共享 GPU device 可选；独立树/脏/present |
| R5 | **边界不破** | core 不碰 OS；skin 不握 swapchain；仅 `PresentFrame*` |

### 1.3 非目标（本方案不做 / 后置）

- 任意矢量 **局部 damage 重绘**（文档已声明不承诺；可整窗 present）
- 首日真 Win32/AppKit 原生 Host 打磨（SPI 形状先齐）
- 多进程 / 跨 device 共享纹理
- 把第三方窗口库打进依赖（只适配，不捆绑）

---

## 2. 架构总图

```text
                         ┌──────────────────────────────────┐
                         │  L5  业务 App / 第三方壳           │
                         │  app.New() · Attach / OpenWindow  │
                         └───────────────┬──────────────────┘
                                         │ 唯一入口
                         ┌───────────────▼──────────────────┐
                         │  ui/app  Application + Scheduler  │
                         │  · WindowSession 表（多窗）        │
                         │  · 按需帧：Wait → Tick → Frame?   │
                         │  · Present 委托（自建 / 外置）     │
                         └───┬─────────────┬────────────┬───┘
                 自建窗口 SPI │             │ 树/脏/动画  │ Present 委托
         ┌───────────────────▼──┐   ┌──────▼──────┐  ┌──▼──────────────┐
         │ ui/platform          │   │ ui/core     │  │ PresentBridge   │
         │ Host / ExternalHost  │   │ Tree+Ticker │  │ → render+sc     │
         │ WaitEvents / Caps    │   │ NeedsFrame  │  │ 或 三方 swap    │
         └──────────────────────┘   └─────────────┘  └─────────────────┘
```

**依赖方向（硬）**

```text
业务 → ui/app → ui/kit|primitive → ui/core
              → ui/platform
              → render / gpu（仅 Present 路径；core 仍不 import OS）
```

禁止：`core → platform` 反向循环；第三方库 import 进 core/kit。

---

## 3. 核心概念

### 3.1 Application（进程级 · 单例入口）

| 职责 | 说明 |
|------|------|
| 生命周期 | `New` / `Run` / `Quit` |
| 窗口表 | `map[WindowID]*WindowSession` |
| 调度 | 全局 `Scheduler`：合并各窗 `NeedsFrame`、计算 wait timeout |
| 共享资源 | 可选共享 `webgpu.Instance` / `Device`（多窗同 GPU） |
| 线程约定 | **UI 单线程**（与现有 X11/wgpu 假设一致）；`runtime.LockOSThread` 由 Run 负责 |

```go
// 概念 API（落地时以源码为准）
app := app.New(app.Options{ /* Theme, SharedGPU, ... */ })
win, _ := app.OpenWindow(app.WindowOptions{Title: "...", Width: 800, Height: 600})
// 或
win, _ := app.AttachWindow(app.ExternalWindow{...})
app.Run() // 阻塞；按需帧主循环
```

### 3.2 WindowSession（每窗口一份）

| 字段 | 说明 |
|------|------|
| `ID` | 稳定窗口 ID |
| `Host` | `platform.Host`（自建）或 `ExternalHost`（挂载） |
| `Tree` | `*core.Tree` 根控件树 |
| `Theme` | 可覆盖 Application 默认 |
| `Viewport` | 逻辑尺寸 / scale |
| `Present` | `PresentFunc`：BeginFrame → tree.Frame → PresentFrame* / 回调 |
| `Surface` | 可选：本库创建的 swapchain；挂载模式可为 nil（外置 present） |
| `Dirty 来源` | Tree + host expose + resize + external RequestFrame |
| `Lifecycle` | presentable / hidden（对齐 `SURFACE_LIFECYCLE_*`） |

**一窗一树**；浮层 `OverlayHost` 挂在该窗 Tree 上，不跨窗。

### 3.3 Present 两种模式

| 模式 | 谁建窗口 | 谁建 Surface | 谁 Present |
|------|----------|--------------|------------|
| **A. Owned** | `platform.NewHost` | app 内 `CreateSurfaceFrom*` + swapchain | app 默认 `PresentBridge` |
| **B. External** | 第三方库 | 三方 **或** 本库用句柄建 Surface | `ExternalPresent` 回调 **或** 本库 PresentBridge |

挂载时最低要求只是：**逻辑尺寸 + 输入事件 + 一帧像素出口**；Surface 可由我方或对方提供。

### 3.4 按需帧语义（R1）

```text
NeedsFrame(window) =
    tree.Dirty()
 || tree.HasActiveTickers()
 || host.PendingRedraw()      // Expose / RequestRedraw
 || session.forceFrame        // resize 首帧、recover 后
 || session.presentable && session.neverPresented

Scheduler timeout =
    if any window HasActiveTickers → min(vsync, 16ms)
    else if any Dirty/force        → 0（立即跑一帧）
    else                           → 阻塞直到下一事件（-1）
```

帧内：

```text
for each window that NeedsFrame && presentable:
  TickActive(dt)           // 仅活跃 Ticker
  LayoutIfNeeded
  Paint + Present
  clear dirty / redraw flag
```

**不做** idle 60fps；**不做** 无脏仍 `BeginFrame`。

---

## 4. SPI 设计

### 4.1 保留并扩展 `platform.Host`

现有：

```go
Caps / Size / ScaleFactor / PumpEvents / RequestRedraw / Close
```

**扩展（增量，不破旧实现）**：

```go
// 可选接口：阻塞等待（实现则 Scheduler 优先用）
type Waiter interface {
    // timeout < 0: 直到有事件；0: 非阻塞（=Pump）；>0: 最多等 duration
    WaitEvents(timeout time.Duration) []Event
}

// 可选：外部/多窗区分
type WindowIDProvider interface {
    WindowID() uintptr // 或 string
}
```

未实现 `Waiter` 时：Scheduler 退化为 `PumpEvents` + 短 sleep（兼容 stub）。

### 4.2 第三方挂载：`ExternalHost` + `ExternalWindow`

**不要求**第三方实现完整 `Host`；提供适配器：

```go
// ui/platform/external.go（概念）
type ExternalWindowDesc struct {
    // 原生句柄（按平台填其一）
    Display uintptr // X11 Display* / Wayland display
    Window  uintptr // X11 Window / HWND / NSView* 等
    MetalLayer uintptr
    WaylandSurface uintptr
    HInstance uintptr // Win

    Width, Height int
    Scale         float64

    // 能力声明：无 IME 则降级
    Caps Caps
}

// ExternalHost：事件由三方推入，不 Pump OS
type ExternalHost struct {
    desc  ExternalWindowDesc
    queue []Event
    // ...
}

func (h *ExternalHost) Inject(ev Event)        // 三方把指针/键鼠翻译后注入
func (h *ExternalHost) InjectResize(w, h int)
func (h *ExternalHost) InjectRedraw()
func (h *ExternalHost) PumpEvents() []Event    // 只排空 queue
func (h *ExternalHost) RequestRedraw()         // 可回调到三方 schedule
func (h *ExternalHost) SetRequestFrame(fn func()) // 通知三方「请在下一 vsync 调 app.Pulse」
```

**挂载契约（第三方必须做的最少事）**

| 步骤 | 三方 | gpui |
|------|------|------|
| 1 | 创建 OS 窗口 | — |
| 2 | 调用 `app.AttachWindow(desc, rootNode)` | 建 Session + ExternalHost + Tree |
| 3 | 指针/键鼠/resize/expose → `host.Inject*` 或 `app.Inject(id, ev)` | 入队 |
| 4 | 主循环空闲 / vsync → `app.Pulse()` 或 `app.RunOnExternalLoop()` | 按需 Frame |
| 5 | （可选）提供 `PresentFunc`；否则 gpui 用句柄建 Surface 自 Present | Present |
| 6 | 关窗 → `app.DetachWindow(id)` | 释放 Session |

### 4.3 Present 委托

```go
// ui/app
type PresentContext struct {
    Session *WindowSession
    DC      *render.Context
    Width, Height int
    Scale   float64
    Theme   *core.Theme
}

// 返回 error；nil 表示已 present 或跳过
type PresentFunc func(pc PresentContext) error

// 默认 Owned：内部 swapchain BeginFrame → tree.Frame → PresentFrameAuto
// External 可注入：只 tree.Frame 到外置 RT，由三方 swap
```

### 4.4 统一入口 `ui/app`（R3）

| API | 作用 |
|-----|------|
| `app.New(opts)` | 进程级 Application |
| `app.OpenWindow(opts)` | 自建窗 + 默认 Present（模式 A） |
| `app.AttachWindow(desc, opts)` | 挂第三方窗（模式 B） |
| `app.Window(id)` / `Windows()` | 查询 |
| `app.DetachWindow(id)` / `CloseWindow(id)` | 拆除 |
| `app.Run()` | **拥有**主循环（内置 Host 场景） |
| `app.Pulse()` / `app.PulseWindow(id)` | **不拥有**主循环时由三方驱动一拍 |
| `app.Quit()` | 退出 Run |
| `session.SetRoot(node)` | 换根 |
| `session.MarkNeedsFrame()` | 外部强制一帧 |
| `session.SetPresent(fn)` | 覆盖 Present |

业务推荐只依赖 `ui/app` + `ui/kit|primitive`，**不要**再抄 `examples/*_smoke` 循环。

---

## 5. 多窗口模型（R4）

### 5.1 所有权

```text
Application
  ├── shared: Instance?, Device?, Scheduler, Theme default
  ├── WindowSession #1  Tree Host Present Surface?
  ├── WindowSession #2
  └── WindowSession #N
```

| 资源 | 共享策略 |
|------|----------|
| `webgpu.Instance` | 进程默认 1 个 |
| `Device` | 默认共享（同 adapter）；高级选项可每窗独立（后置） |
| `Swapchain` / `render.Context` | **每窗独立** |
| `Tree` / Focus / Overlay | **每窗独立** |
| 字体 / Theme tokens | 默认可共享只读 |

### 5.2 调度

- **单线程 Run**：一轮调度扫描所有 `NeedsFrame` 的窗，逐个 Present（顺序稳定：按 ID 或 z 注册序）。
- 一窗阻塞 Present 不应弄脏其它窗状态；error 隔离（单窗 recover，见 lifecycle）。
- 外部循环：`Pulse()` 扫描全部；`PulseWindow(id)` 只跑一窗。

### 5.3 输入路由

- Owned：每个 Host 的事件只进自己的 Session。
- External：`Inject(id, ev)` 必须带 WindowID（或 per-host 注入）。
- **禁止**跨窗焦点默认串联（除非业务显式 `FocusWindow`）。

### 5.4 与文档「多窗口后置」的关系

`UI_FRAMEWORK_MAP` 将「多窗口」标为后置能力 —— 本方案把 **壳层多窗** 提前为 **P0/P1 架构**，避免入口做成单窗死结构；**原生系统菜单 / 跨窗 DnD** 仍后置。

---

## 6. 按需帧详细设计（R1）

### 6.1 core 变更

| 项 | 行为 |
|----|------|
| `Tree.NeedsFrame()` | `dirty \|\| hasActiveTickers` |
| `Tree.Frame` | 可选：若 `!NeedsFrame` 直接 return；或由 app 层判断后不调用 |
| `LayoutIfNeeded` | 仅 root/subtree `needsLayout` 时 layout |
| `TickClock` | **删除「无脑 MarkDirty」**；改为 `TickActive(dt) bool` |
| `Ticker` 接口 | `Tick(dt) (still bool)`；`AddTicker` / `RemoveTicker` |
| 控件 | Spin/Skeleton/Motion/caret：激活注册，结束注销 |

### 6.2 platform 变更

| 项 | 行为 |
|----|------|
| `WaitEvents` | Linux：`XPending==0` 时 `XNextEvent` 阻塞或 `poll(ConnectionNumber)` + timeout |
| `EventRedraw` | Dispatcher / Scheduler → `session.MarkNeedsFrame()` |
| `RequestRedraw` | 入队 + 唤醒 Wait |

### 6.3 app Scheduler 伪代码

```text
func (a *Application) Run() {
  runtime.LockOSThread()
  for !a.quit {
    timeout := a.computeTimeout() // 见 §3.4
    // 合并：所有 Owned Host Wait；External 只排空 queue
    a.pumpAll(timeout)
    now := time.Now()
    dt := ...
    for _, s := range a.sessions {
      if !s.presentable { continue }
      if s.tree.HasActiveTickers() {
        s.tree.TickActive(dt)
      }
      if !s.NeedsFrame() { continue }
      if err := s.present(); err != nil {
        s.handlePresentError(err) // device_lost / resize
      }
    }
  }
}
```

External 模式：

```text
// 三方主循环
for {
  thirdParty.Poll()
  app.Pulse() // 内部同 pump+tick+present，但不阻塞 Wait
}
```

---

## 7. 第三方挂载路径（R2）详解

### 7.1 三种集成深度

| 深度 | 三方提供 | gpui 做 | 适用 |
|------|----------|---------|------|
| **D1 句柄 + 事件** | 窗口句柄、尺寸、输入、expose | 自建 Surface、自 Present、按需帧 | 最常见：GLFW/SDL 类 |
| **D2 句柄 + 外置 RT** | 句柄 + 自己的 framebuffer/纹理 | 只 Paint 到 `render.Context`，Present 回调给三方 | 引擎已握 swapchain |
| **D3 纯逻辑** | 仅事件 + 尺寸（Headless 类） | CPU/离屏 Context，无 GPU 窗 | 测试、录制 |

### 7.2 适配器包约定（可选，不进 core）

```text
ui/app                      # 入口
ui/platform                 # Host / ExternalHost SPI
// 官方可后补示例适配，不强制依赖：
examples/embed_glfw/        # 示范 D1
examples/embed_callback/    # 示范 D2
```

**原则**：gpui **不** import GLFW 等；示例适配器在 examples 或独立 module。

### 7.3 事件映射表（挂载方实现）

| 三方事件 | `platform.Event` |
|----------|------------------|
| cursor pos / button | `EventPointer` |
| key / text | `EventKey` / `EventText` |
| scroll | `EventScroll` |
| resize | `EventResize` |
| expose / damage / refresh | `EventRedraw` |
| focus | `EventFocus` |
| close | `EventClose` → Detach |
| IME（若有） | `EventIME`（无则 Caps 不声明） |

### 7.4 线程与唤醒

- 若三方事件在非 UI 线程：`Inject` 必须线程安全队列，由 `Pulse`/`Run` 在 UI 线程消费。
- `MarkNeedsPaint` → `ExternalHost.SetRequestFrame` → 三方 `post empty event` / `glfwPostEmptyEvent` 等价物，避免沉睡不醒。

---

## 8. 包与目录落地

```text
ui/
  app/                    # ★ 新：唯一入口
    app.go                # Application
    window.go             # WindowSession
    scheduler.go          # 按需帧
    present_owned.go      # 默认 GPU present（从 exboot 上收）
    present_external.go   # 委托 Present
    attach.go             # AttachWindow
    options.go
    app_test.go           # headless 多窗 + dirty 调度
  core/                   # 改：NeedsFrame / Ticker
  platform/               # 改：WaitEvents / ExternalHost / Dispatch Redraw
  kit/ primitive/         # 改：Sync 事件化；动画挂 Ticker
  ...
examples/
  ui_app_smoke/           # 新：OpenWindow 按需帧
  ui_app_multi/           # 新：双窗
  ui_app_attach_mock/     # 新：ExternalHost 模拟三方
```

`examples/exboot`：GPU 引导逻辑 **上收到** `ui/app` 内部（或 `ui/app/gpu` 子文件），examples 只调 `app.OpenWindow`；exboot 可薄包装兼容旧 example，避免双源。

---

## 9. 分阶段实施

### Phase 0 — 契约与单测骨架（0.5–1d）

- 文档定稿（本文）+ `UI_FRAMEWORK_MAP` 增补「C-Frame 按需 + App Shell」指针  
- `core`：`NeedsFrame`、修 `TickClock`、`Ticker` 最小接口 + 单测  
- **DoD**：无 Host 下 dirty 帧计数可测

### Phase 1 — 按需帧 + 单窗 Owned 入口（主路径）

- `platform.WaitEvents`（Linux + Headless）  
- `Dispatch`/`Scheduler` 处理 `EventRedraw`  
- `ui/app`：`New` / `OpenWindow` / `Run` + 默认 Present（收编 exboot 必要部分）  
- 一个 `ui_app_smoke`：静止 N 秒 present 次数 ≈ 1 + expose  
- **DoD**：Linux 真窗 idle 不空转；交互/resize 正常刷

### Phase 2 — 挂载入口 External

- `ExternalHost` + `AttachWindow` + `Pulse`  
- `PresentFunc` 可替换  
- `ui_app_attach_mock`：纯 Inject 模拟三方，无 X11 自建  
- **DoD**：文档级挂载清单 + mock 测试绿

### Phase 3 — 多窗口

- Session 表、共享 Device、双窗 smoke  
- 单窗 present 失败隔离  
- **DoD**：两窗独立 hover/按钮；关一窗另一窗仍在

### Phase 4 — 控件脏源收口

- Button `SyncState` 事件化  
- Spin/Skeleton/Motion/caret → Ticker  
- kit 文档注释去掉「每帧调用」  

### Phase 5 — 打磨（可并行后置）

- Wait 与 vsync 对齐、合并 expose  
- Win/mac External 句柄路径真测  
- 可选 damage present（非阻塞本方案）

---

## 10. API 使用示意

### 10.1 自建窗口（统一入口）

```go
a := app.New(app.Options{Title: "demo"})
w, _ := a.OpenWindow(app.WindowOptions{Width: 800, Height: 600})
w.SetRoot(kit.Column(
    kit.Button("OK", kit.OnClick(func() { /* MarkNeeds 自动 */ })),
))
a.Run()
```

### 10.2 挂载第三方窗口

```go
a := app.New(app.Options{ExternalLoop: true})
w, _ := a.AttachWindow(app.ExternalWindow{
    Desc: platform.ExternalWindowDesc{
        Display: disp, Window: win,
        Width: 800, Height: 600, Scale: 1,
        Caps: platform.CapPointer | platform.CapKeyboard | platform.CapPresent,
    },
    Root: myUI,
    // Present: nil → gpui 用句柄建 Surface
    // 或 Present: func(pc app.PresentContext) error { return third.Swap(pc.DC) },
})
w.Host().(*platform.ExternalHost).SetRequestFrame(func() {
    thirdParty.PostWake()
})

// 三方循环
for thirdParty.Running() {
    for _, e := range thirdParty.Poll() {
        a.Inject(w.ID(), mapEvent(e))
    }
    a.Pulse()
}
```

### 10.3 多窗口

```go
a := app.New(nil)
w1, _ := a.OpenWindow(app.WindowOptions{Title: "Main", Root: mainRoot})
w2, _ := a.OpenWindow(app.WindowOptions{Title: "Tool", Root: toolRoot})
a.Run()
```

---

## 11. 风险与决策

| 风险 | 缓解 |
|------|------|
| 共享 Device 多 swapchain 生命周期 | 每窗独立 SC；lost 时 Session 级 recover；参考 `SURFACE_LIFECYCLE_*` |
| External 不唤醒导致永不画 | 强制 `SetRequestFrame` 文档 + 无回调时 Pulse 忙等警告 |
| `TickClock` 兼容破坏 | 保留 API 但改语义；changelog 标明 |
| examples 大爆炸 | Phase1 只迁 1 个 smoke；其余渐进 |
| 多窗 vs 文档「后置」 | 壳层多窗做架构；系统级多窗特性仍后置 |
| Present 双路径复杂度 | 先 Owned 默认路径稳定，再 Attach |

**决策记录（默认）**

1. UI **单线程**；多窗轮转 Present，不做并行 paint。  
2. 默认 **共享 Device**。  
3. 按需帧默认 **整窗 present**（Phase1）；产品侧用 **RepaintBoundary + MarkNeedsPaint** 降 CPU；引擎 G2 下矢量 MSAA 仍常 LoadOpClear，**不**承诺 swapchain 任意矢量脏矩形保留。可选 damage present 见 Phase5 / `PresentFrameAuto`。  
4. 第三方 **零强制依赖**；只 SPI + 示例。  
5. 唯一入口包名：**`ui/app`**（不用 `ui/antd`，不把循环留在 kit）。

---

## 12. 验收标准（总）

| 场景 | 期望 |
|------|------|
| 单窗静态 5s | present 次数不随时间线性涨（仅首帧/expose） |
| 按钮 hover | 进入/离开各触发有限帧 |
| Spin 显示 | 期间连续帧；隐藏后回 idle |
| Attach mock | 仅 Inject 可驱动 UI |
| 双窗 | 交互互不串树；关 A 不影响 B |
| device lost（Owned） | 单窗 recover 后可再画 |
| API | 业务示例 ≤ 15 行到 `Run`（无手写 for Present） |

---

## 13. 与现有文档的挂钩

| 文档 | 挂钩点 |
|------|--------|
| `UI_FRAMEWORK_MAP` §C-Frame / 帧顺序 | 改为「按需」；增加 App Shell 层说明 |
| `UI_FRAMEWORK_MAP` §9.4 Host | 增 ExternalHost、WaitEvents |
| `SURFACE_LIFECYCLE_*` | Session.presentable ↔ OnUnpresentable |
| `S5_WIDGET_ENTRY` | 控件入口条件 + 按需帧不冲突 |
| `exboot` | 上收进 `ui/app` present_owned |

---

## 14. 建议执行顺序（开工清单）

```text
1. core: NeedsFrame + Ticker + 修 TickClock     ← 无 UI 可测
2. platform: WaitEvents + Redraw→dirty          ← Headless/Linux
3. app: Application + OpenWindow + Run          ← 替换一个 smoke
4. app: AttachWindow + Pulse + ExternalHost     ← 挂载
5. app: 多 WindowSession + 双窗 smoke           ← 多窗
6. kit: 去每帧 Sync；动画 Ticker                ← 脏源干净
```

**第一刀不碰** render 局部重绘、不绑第三方窗口库、不并行多窗 paint。

---

## 15. 落地进度（对齐 gogpu）

| 项 | 路径 | 状态 |
|----|------|------|
| `Tree.NeedsFrame` / `FrameIfNeeded` / `Ticker` | `ui/core/ticker.go` · `tree.go` | ✅ |
| `TickClock` 不再无脑 MarkDirty | `ui/core/motion.go` | ✅ |
| `Host.WaitEvents` / `WakeUp` | `ui/platform/host.go` + Headless/Linux/… | ✅ |
| `EventRedraw` → `MarkDirty` | `ui/platform/dispatch.go` | ✅ |
| 需求循环 IDLE/ANIMATING/CONTINUOUS | `ui/app` `Run`/`Pulse`/`RequestRedraw` | ✅ |
| Headless 单测（静止不刷、Ticker、Quit） | `ui/app/app_test.go` | ✅ |
| RenderLoop 双 OS 线程 | `ui/app/thread.go` + `renderloop.go`；Present 同步 hop | ✅ |
| `OpenWindow` GPU Owned Present | `ui/app` + exboot 上收 | 后置 |
| `AttachWindow` ExternalHost | Phase 2 | 后置 |
| 多 WindowSession | Phase 3 | 后置 |

**gogpu 对应关系**

| gogpu | gpui |
|-------|------|
| `Invalidator` + `RequestRedraw` | `app.RequestRedraw` + `pendingRedraw` + `Tree.SetOnDirty` |
| IDLE `WaitEvents` | `Host.WaitEvents(-1)` |
| ANIMATING `onUpdate` / OnDraw on dirty | `TickActive` + `OnUpdate`；paint only if `Dirty` |
| CONTINUOUS | `Options.ContinuousRender` |
| `StartAnimation` | `StartAnimation` / `StopAnimation` + `Tree.AddTicker` |
| `OnDraw` on render thread | 现：同线程 `Present`；RenderLoop 后置 |

## 16. 修订

| 版本 | 日期 | 说明 |
|------|------|------|
| 1.0 | 2026-07-21 | 初稿：按需帧 + 挂载 + 统一入口 + 多窗口 |
| 1.1 | 2026-07-21 | Phase 0–1 代码落地；对齐 gogpu 三模式 |
