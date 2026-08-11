# 跨平台输入 / IME 分层方案（草案 v0.7）

> **性质：DRAFT** — 方向收敛中，待确认后转真源。不参与 §R/§W 关闭。  
> **并读：** [`ENGINE_FLUTTER_SKIA_ARCH.md`](./ENGINE_FLUTTER_SKIA_ARCH.md)（§3 Host 注入）· [`ENGINE_UI_WIDGET_RENDER.md`](./ENGINE_UI_WIDGET_RENDER.md)（§0.3 U3 IME 暂缓可预留 SPI）  
> **v0.2 修正（用户定，推翻 exhost 演进思路）：**  
> ① 平台窗口 + 原生事件统一在 **`ui/platform`** 管理（后端重写，非迁移 exhost）；  
> ② 事件抽象出独立层 **`ui/input`** —— 各平台事件类型归一为跨平台一致的统一事件给上层；  
> ③ 多窗口：入口 **`application.New()`** + **`app.NewWindow()`**；kit 只做控件不建窗口；  
> ④ 控件层由 **框架统一事件绑定管理**（InputRouter），自定义控件实现统一回调接口即自动接线。  
> **v0.3–v0.6（方案 A 第 1–4 步落地）：** `ui/input` · `ui/platform` · `ui/application` · `embedder.InputRouter` 均已实现。  
> **v0.7（方案 A 第 5 步落地，方案 B：删 FromPlatform）：** `ui/gestures` 完整迁移到 `input` 类型——`PointerEvent = input.PointerEvent`（别名），`FromPlatform`/`EffectivePointerID`/`PrimaryPointerID=1` 删除，`PrimaryPointerID=0`（鼠标主指针）对齐 input；新增 `FromInput(input.Event)` 统一入口（Pointer/Scroll 直映射，Touch 携带 ID/X/Y）；多指并行 = 每 PointerID 一个独立 Arena（ID 0=鼠标，ID≥1=触摸槽）；`ui/rendering/Scrollable` 同步迁移；L2 shell 适配（2 处常量 + FromInput）。新增双指并行单测（双指独立 Tap / 一指 Tap 一指 Pan 并行不干扰）。

---

## 0. 目标模式（对齐框架总结构）

```text
L3  kit（ui/kit）       控件消费统一事件；kit.App 作窗口内控件根 —— 永不碰原生
L2  抽象                 指针归一(鼠标+触摸) / 键位 / 文本+IME：gestures · focus · textinput
L1  app 壳               application（多窗口） + embedder.InputRouter（统一事件绑定分发）
L0  platform             平台窗口 + 原生事件采集 + vsync + 能力接口（统一管理，唯一原生接触点）
    render / gpu：渲染底座（不动）
```

依赖纪律（照抄现有）：`kit → app/embedder → input → platform → render`；`ui` 永不 import `gpu`；`kit` 永不 import `platform`。

---

## 1. 应用模型（多窗口，v0.5 已实现）

```go
// ui/application（已实现）
app := application.New(application.Config{
    Name:    "myapp",
    Backend: platform.DisplayAuto,   // x11/wayland/auto；win32/appkit 后置
})

winA, err := app.NewWindow(application.WindowOptions{Title: "A", Width: 1200, Height: 800})
winB, err := app.NewWindow(application.WindowOptions{Title: "B", Width: 800, Height: 600})

winA.SetRoot(kit.App(/* 控件树，含 Input / ListView … */)) // kit 内容根
app.Run() // 每窗独立事件泵；主窗（首个）关闭 → 全部退出
```

| API | 职责 | 说明 |
|---|---|---|
| `application.New(cfg)` | 应用级初始化 | 后端注册（platform init 已自动）、Clear 色/WarmUp/事件回调等全局配置 |
| `app.NewWindow(opts)` | **引擎窗口** | platform.Open 建原生窗（或 `Config.NewHost` 注入 host，测试/嵌入）→ 注册 → 首个为主窗 |
| `win.SetRoot(root)` | 挂控件树 | 构建 embedder.PipelineApp（渲染管线）；Run 前必调 |
| `win.Host()` / `IME()` / `Clipboard()` | 能力透传 | 经 platform.Window 暴露 |
| `app.Run()` / `app.Quit()` / `app.Close()` | 多窗口生命周期 | 每窗独立 goroutine 事件泵；主窗关闭 → Quit 全部；Close 幂等 |

**决策：kit 不提供 `kit.NewWindow`。** 窗口 = 平台 + 引擎事务；kit 提供 `kit.App` 组件作窗口内控件根（对齐 `docs/antd/app.md`）。

---

## 2. 事件抽象层 `ui/input`（v0.2 新增，核心）

> 目标：**每个平台的事件类型 → 归一为跨平台一致的统一事件**，上层只认 `ui/input`。
> **v0.3 已实现**：包 `ui/input`（`event.go` / `keys.go` / `pointer.go` / `ime.go` / `fromplatform.go`）+ 单测 16 项全绿（`TestFromPlatform_*` / `TestFromTouch` / `TestFromText` / `TestFromIME` / `TestKeyTableStable` / `TestKeyString` 等）。`CGO_ENABLED=0 go build ./ui/...` + `go test ./ui/input/` 通过，ui 层零回归。

### 2.1 统一事件类型（已实现）

```go
// ui/input/event.go —— 上层唯一消费的输入事件
type Event struct {
    Kind      Kind        // close/resize/key/pointer/touch/scroll/text/ime/wake
    WindowID  int         // 多窗口路由（application 填入）
    Modifiers Modifiers   // Shift / Ctrl / Alt / Meta（跨平台一致布尔）
    Width, Height, Scale  // Kind=Resize
    Key       KeyEvent    // 逻辑键（Key + Rune + Pressed + Repeat）
    Pointer   PointerEvent// 鼠标统一样本（ID 0 = 主指针）
    Touch     TouchEvent  // 多点触摸（ID ≥ 1）
    Text      TextEvent   // 字符输入（含 IME 上屏文本）
    IME       IMEEvent    // 预编辑/光标（Compose/Commit/CaretMove）
}

// FromPlatform 是消除平台差异的唯一转换点：
// 各平台后端产出 platform.Event（原生近邻）→ input.FromPlatform(ev, mods) → 统一语义
func FromPlatform(ev platform.Event, mods Modifiers) Event

// 触摸/文本/IME 的独立入口（供后端直接构造统一事件）：
func FromTouch(ev TouchEvent, mods Modifiers) Event
func FromText(s string, mods Modifiers) Event
func FromIME(ev IMEEvent, mods Modifiers) Event
```

### 2.2 逻辑键（跨平台一致键位表，v0.3 已实现）

```go
// ui/input/keys.go —— 所有平台后端把原生键码映射到此表（append-only，禁改编号）
type Key uint32
const (
    KeyNone Key = iota
    KeyA … KeyZ          // 逻辑字母
    Key0 … Key9          // 数字
    KeyF1 … KeyF24       // 功能键
    KeyEnter / KeyTab / KeySpace / KeyBackspace / KeyDelete / KeyEscape
    KeyHome / KeyEnd / KeyPageUp / KeyPageDown / KeyInsert / KeyPrintScreen
    KeyCapsLock / KeyNumLock / KeyScrollLock / KeyPause / KeyMenu
    KeyArrowUp / KeyArrowDown / KeyArrowLeft / KeyArrowRight
    KeyShift / KeyControl / KeyAlt / KeyMeta
    KeyMinus / KeyEqual / KeyBracketLeft/Right / KeyBackslash
    KeySemicolon / KeyQuote / KeyGrave / KeyComma / KeyPeriod / KeySlash
    KeyPad0…KeyPad9 / KeyPadDecimal/Divide/Multiply/Subtract/Add/Enter
    KeyIMECompose / KeyIMECandidatePrev/Next/Select
)

type KeyEvent struct {
    Key     Key    // 逻辑键
    Rune    rune   // 可打印字符（0 = 无）
    Pressed bool
    Repeat  bool   // OS 自动重复
}
```

现状 `ui/focus/keys.go` 只有 4 个硬编码键 → 后续迁移为 `ui/input` 的逻辑键位表；`ui/focus.KeyEvent` 消费 `input.KeyEvent` 并携带 `Modifiers`。

### 2.3 统一指针（鼠标 + 触摸归一，v0.3 类型 + v0.7 gestures 消费）

```go
// ui/input/pointer.go —— 鼠标 = ID 0 的单指触摸；触摸 = TouchID 多槽
type PointerKind int
const ( PointerMove / PointerDown / PointerUp / PointerCancel / PointerScroll )

type PointerEvent struct {
    Kind    PointerKind
    ID      int     // 0 = 主指针(鼠标)；触摸 = 触摸槽
    X, Y    float64 // 逻辑像素
    Button  int     // 0/1/2/3 = none/left/middle/right
    ScrollX, ScrollY float64
}
type TouchEvent struct { Kind; ID; X, Y float64 } // 多点触摸样本
```

**消费端（v0.7）：** `ui/gestures` 直接消费 `input.PointerEvent`（`type PointerEvent = input.PointerEvent` 别名），`FromInput(input.Event)` 统一入口；多指并行 = 每 PointerID 一个独立 GestureArena。

`ui/gestures` 只消费 `input.PointerEvent`（竞技场一套，多指并行）——现状 `gestures.FromPlatform` 直接删，改为吃 `input.Event`。

### 2.4 IME / 文本（v0.3 类型已实现，能力接口后置）

```go
// ui/input/ime.go
type IMEKind int
const ( IMECompose / IMECommit / IMECaretMove )

type IMEEvent struct {
    Kind   IMEKind // Compose（预编辑变化）/ Commit（上屏）/ CaretMove（光标/选区）
    Text   string  // Compose 的拼音串 / Commit 的上屏串
    Start, End int // 编辑区位置（bytes；End=-1 = 全缓冲）
}
type TextEvent struct{ Text string }

// 能力接口在 L0，转换在 input：
// platform.IME.EnableIME(rect) → 系统回事件 → 后端产 platform.Event(IME) → input.Event(IME)
// → textinput 更新预编辑区 → 光标边界局部脏（对齐 W4 R22 选区预留）
```

---

## 3. L0：`ui/platform` 统一管理层（重写，非迁移 exhost）

### 3.1 三分离（推翻 exhost 的"一个 openX11 干所有"）

```go
// ① 后端注册表
type Backend interface {
    Kind() PlatformKind
    Create(opts Options) (*Window, error)   // 建窗
    Adopt(ns NativeSurface) (*Window, error) // 注入已有句柄（嵌入场景）
}
func Register(kind PlatformKind, b Backend)

// ② 窗口 = 句柄 + 事件泵 + 能力
type Window struct {
    host  Host          // NativeSurface / Size / ScaleFactor / WaitEvents / WakeUp
    ime   IME           // nil = 不支持，静默降级
    clip  Clipboard
    close func()
}
func Open(opts Options) (*Window, error)   // Auto 检测 → 查注册表
func Adopt(ns NativeSurface) (*Window, error)

// ③ 能力接口（重建 XIM/zwp_text_input_v3/TSF/InputMethod 统一姿态）
type IME interface {
    EnableIME(rect Rect)
    SetComposing(text string, cursor int)
    Commit(text string)
    DisableIME()
}
type Clipboard interface {
    Get(kind string) (string, error)
    Set(kind, data string) error
}
```

### 3.2 平台事件（原生近邻，差异保留在此层）

```go
// platform.Event 保持"接近原生"：keysym/虚拟键码/原始触摸 —— 由 input.FromPlatform 归一
// 关键：platform 层不定义逻辑键语义；逻辑键只在 ui/input
type Event struct {
    Type     EventType // Close/Resize/Expose/Pointer/Key/Wake + IME
    …
}
```

### 3.3 后端重写落点（v0.4 已实现）

| 文件 | 内容 | 状态 |
|---|---|---|
| `ui/platform/window.go` | `Window`/`Open`/`Adopt`（默认尺寸、Close 幂等、能力 getter） | ✅ |
| `ui/platform/backend.go` | `Backend` 接口 + `Register`/`backendFor`/`registeredKinds` + `toPlatformKind` | ✅ |
| `ui/platform/x11_linux.go` | X11 后端重写：`x11Backend.Create/Adopt` + `x11Host` 事件泵 + 事件解析独立函数，`init()` 自注册 | ✅ |
| `ui/platform/wayland_linux.go` | Wayland 后端重写：`waylandBackend.Create`（Adopt 明确不支持，嵌入里程碑）+ `wlHost` 事件泵，`init()` 自注册 | ✅ |
| `ui/platform/win32_windows.go` | stub 注册（Create/Adopt 报 not implemented） | ✅ |
| `ui/platform/appkit_darwin.go` | stub 注册（Create/Adopt 报 not implemented） | ✅ |
| `ui/platform/ime.go` | `IME`/`Clipboard`/`Rect` 能力接口 | ✅ |
| `ui/platform/host.go` | `PlatformNone=-1` 独立 const（不扰动 iota；`PlatformX11==0` ABI 锁定） | ✅ |
| `ui/platform/stub_host.go` | 保留（测试宿主） | ✅ |

`examples/exhost` **未动**（用户指示示例保持原样）；作废/迁移留到 application 里程碑。

**验证：** `TestPlatformKindValuesStable` 锁 PlatformX11==0 等 ABI；`TestRegister*`/`TestOpen*`/`TestAdopt*`/`TestWindow*` 覆盖注册表路由、Open/Adopt 分发、Close 幂等、能力 nil 降级。ui 层 14 包回归全绿。

---

## 4. L1：控件层框架统一事件绑定（InputRouter，v0.6 已实现）

```go
// ui/input/eventtarget.go —— 自定义控件的统一交互回调接口，实现即自动接线
type PointerHandler interface { OnPointer(ev input.PointerEvent) }
type KeyHandler     interface { OnKey(ev input.KeyEvent) }
type TextHandler    interface { OnText(ev input.TextEvent) }
type IMEHandler     interface { OnIME(ev input.IMEEvent) }
type EventTarget    = interface { PointerHandler; KeyHandler; TextHandler; IMEHandler }

// ui/embedder/input_router.go —— 每窗口一个，PipelineApp 生命周期内自动运行
type InputRouter struct {
    hit      HitTestFunc // = PipelineApp.HitTestPointer（构造时自动接线）
    focus    *focus.FocusManager
    OnPointer func(ev input.PointerEvent, target rendering.RenderObject)
    OnKey     func(ev input.KeyEvent)
    OnText    func(ev input.TextEvent)
    OnIME     func(ev input.IMEEvent)
}
func NewInputRouter(hit HitTestFunc, f *focus.FocusManager) *InputRouter
func (r *InputRouter) RoutePlatform(ev platform.Event) // 归一 + 分发
func (r *InputRouter) Route(ev input.Event)

// PipelineOptions.Input *InputRouter —— 挂载后 Run 自动路由：
//   Pointer/Touch → HitTest → 命中 RO 实现 PointerHandler 即 OnPointer
//   Key → 修饰键跨事件跟踪 → OnKey + focus 桥（Tab/Enter/激活）
//   Text/IME → OnText / OnIME（textinput 里程碑消费）
// 未挂载 Input 的窗口保持原 OnEvent 路径字节不变（示例零影响）
```

- `PipelineApp.Run` 不再把事件丢给示例 `OnEvent` 手写分发；改为：`platform.WaitEvents → input.FromPlatform → InputRouter.Route`。
- IME 生命周期自动：焦点进文本框 `ime.EnableIME(rect)`；预编辑 `SetComposing`；上屏 `Commit`；失焦 `DisableIME`。

---

## 5. L2 / L3 消费端改动

| 层 | 现状 | 改后 | 状态 |
|---|---|---|---|
| `ui/gestures` | `FromPlatform(platform.Event)` | 直接消费 `input.PointerEvent`（alias）；`FromInput` 入口；多指并行 Arena | ✅ v0.7 |
| `ui/focus` | `keys.go` 4 个硬编码键 | 键位表迁 `ui/input`；`KeyEvent` 带 `Modifiers` | 🔄 键位表迁移待做（focus 已经 InputRouter 桥接消费 input） |
| `ui/textinput`（新增） | — | 编辑缓冲 + 光标/选区 + IME 预编辑 + 剪贴板 | ⬜ |
| `ui/rendering/Scrollable` | `gestures.FromPlatform` | `FromInput` + `input` 常量 | ✅ v0.7 |
| `ui/kit`（未建，P7） | — | 控件实现 `EventTarget` 或依赖 kit 包装；`kit.App` 窗口内容根 | ⬜ |

---

## 6. 依赖与纪律（硬）

```text
kit          ↛ platform  ↛ 控件层禁止出现 keysym/HWND/NSView
app/embedder → input      （InputRouter 只认统一事件）
input        → platform   （FromPlatform 是唯一归一转换点）
platform     → render     （建 PresentTarget 经 render；ui 不 import gpu）
examples     → application 入口；禁止在示例里写原生事件解析（exhost 作废）
```

---

## 7. 实施顺序

| 步 | 内容 | 风险 | 判定 | 状态 |
|---|---|---|---|---|
| 1 | `ui/input` 包：统一 Event/逻辑键表/PointerEvent/FromPlatform | 低（纯新类型+转换，不动渲染） | 单测：FromPlatform 各平台→同一语义 | ✅ v0.3 |
| 2 | `ui/platform` 重写：`Window`/`Open`/`Adopt`/后端注册表（x11/wayland 三分离，win32/appkit stub） | 中（碰原生） | 单测路由全绿；示例未动仍跑 exhost | ✅ v0.4 |
| 3 | `ui/application` + `app.NewWindow` 多窗口 | 中 | 生命周期/主窗/guard 单测绿；GPU 双窗真窗留真窗里程碑 | ✅ v0.5 |
| 4 | `embedder.InputRouter`：自动路由，示例删手写 HandleEvent | 中 | 路由/modifier/hit 单测绿；未挂载窗口字节不变（示例零影响） | ✅ v0.6 |
| 5 | `ui/gestures` 多点 + `Event` 触摸归一（方案 B：删 FromPlatform） | 中 | 双指并行单测（双指独立 Tap / Tap+Pan 并行） | ✅ v0.7 |
| 6 | `ui/textinput` + `platform.IME` + Wayland `zwp_text_input_v3` 实证 | 高（真 IME 才算数） | Linux 真跑通 IME | ⬜ |
| 7 | Win/mac 后端 + TSF/InputMethod（接口已预留） | 高 | 后置 | ⬜ |

第 6 步 Linux 真跑通 IME = 方案成败证明；X11 XIM 为备选。

---

## 8. 与现有文档关系

- `ENGINE_UI_WIDGET_RENDER.md` §0.3 **U3**：完整 IME 暂缓、**SPI 可预留** → 第 6 步前均为"预留 SPI"，不冲突。
- §2.4 非主能力：IME / 剪贴板 / 拖放不做进 §R 关闭 → 本线独立管理，不并入 W 矩阵。
- `ui/input` 是架构新增层，需在确认后回写 `ENGINE_ARCH_OVERVIEW.md` 分层图与依赖条款。

---

## 9. 修订

| 版本 | 说明 |
|---|---|
| v0.1 | DRAFT：`ui/platform` 统一管窗口+事件；控件层框架统一事件绑定（InputRouter）；四类输入分层；IME 能力接口。 |
| **v0.2** | **推翻 exhost 演进思路**：① 新增事件抽象层 `ui/input`（平台事件统一跨平台一致给上层）；② 多窗口应用模型 `application.New()` + `app.NewWindow()`，kit 不建窗口只出 `kit.App` 内容根；③ `ui/platform` 持后端起**重写**为注册表 + 三分离（建窗/事件/能力）；④ `ui/gestures`/`ui/focus` 改消费 `input` 统一类型。 |
| **v0.3** | **方案 A 第 1 步落地**：`ui/input` 包实现（`event.go`/`keys.go`/`pointer.go`/`ime.go`/`fromplatform.go`），单测 16 项全绿，`go build ./ui/...` + 相关包回归零失败。下一步：第 2 步 `ui/platform` 重写。 |
| **v0.4** | **方案 A 第 2 步落地**：`ui/platform` 后端重写——`Window`/`Open`/`Adopt`（`window.go`）+ 后端注册表（`backend.go`）+ x11/wayland 三分离后端（`init()` 自注册）+ win32/appkit stub 注册 + `IME`/`Clipboard` 能力接口（`ime.go`）。`PlatformNone=-1` 独立 const 不扰动 iota，`TestPlatformKindValuesStable` 锁 ABI。示例保持原样（exhost 未动）。ui 层 14 包回归全绿。下一步：第 3 步 `ui/application` 多窗口。 |
| **v0.5** | **方案 A 第 3 步落地**：`ui/application` 多窗口应用层——`New(Config)`/`NewWindow(opts)`/`SetRoot(root)`/`Run()`/`Quit()`/`Close()`；每窗 = `platform.Window` + `embedder.PipelineApp` 独立 goroutine 事件泵；主窗关闭 → 全部退出；`Config.NewHost` 注入点（测试/嵌入）。`platform.WrapHost` 供宿主复用。生命周期/主窗/guard 单测全绿，ui 层 14 包回归全绿。GPU 双窗真窗验证留真窗里程碑。下一步：第 4 步 `embedder.InputRouter`。 |
| **v0.6** | **方案 A 第 4 步落地**：`embedder.InputRouter` 统一事件绑定——`input.EventTarget` 接口族（PointerHandler/KeyHandler/TextHandler/IMEHandler）；`PipelineOptions.Input` 可选挂载，Run 中 pointer/key 自动 `input.FromPlatform` 归一 + 路由（命中控件实现 handler 即自动接线）；修饰键跨事件跟踪（事件时语义：shift 按下事件本身 Mods.Shift=false）；focus 桥接（mapFocusKeyCode 兼容 focus 键码空间）；`input.KeyEvent` 增 `Mods` 字段。未挂载 Input 的窗口字节不变（示例零影响）。单测 7 项全绿，ui 层 14 包回归全绿。下一步：第 5 步 `ui/gestures` 多点 + 触摸归一。 |
| **v0.7** | **方案 A 第 5 步落地（方案 B：删 FromPlatform）**：`ui/gestures` 完整迁移到 `input` 类型——`PointerEvent = input.PointerEvent`（别名）、`FromPlatform`/`EffectivePointerID` 删除、`PrimaryPointerID=0`（鼠标主指针）对齐 input；新增 `FromInput(input.Event)`（Pointer/Scroll 直映射、Touch 携 ID/X/Y）；多指并行 = 每 PointerID 独立 Arena（ID 0=鼠标，≥1=触摸槽）；`ui/rendering/Scrollable` 同步迁移；L2 shell 适配（FromInput + 2 处 input 常量）。新增双指并行单测（双指独立 Tap / 一指 Tap 一指 Pan 并行不干扰）+ FromInput 单测。gestures 8 项 + 全 ui 15 包 + L2 shell 回归全绿。下一步：第 6 步 `ui/textinput` + `platform.IME` + Wayland `zwp_text_input_v3`。 |