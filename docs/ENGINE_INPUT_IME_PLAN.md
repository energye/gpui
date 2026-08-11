# 跨平台输入 / IME 分层方案（草案 v0.3）

> **性质：DRAFT** — 方向收敛中，待确认后转真源。不参与 §R/§W 关闭。  
> **并读：** [`ENGINE_FLUTTER_SKIA_ARCH.md`](./ENGINE_FLUTTER_SKIA_ARCH.md)（§3 Host 注入）· [`ENGINE_UI_WIDGET_RENDER.md`](./ENGINE_UI_WIDGET_RENDER.md)（§0.3 U3 IME 暂缓可预留 SPI）  
> **v0.2 修正（用户定，推翻 exhost 演进思路）：**  
> ① 平台窗口 + 原生事件统一在 **`ui/platform`** 管理（后端重写，非迁移 exhost）；  
> ② 事件抽象出独立层 **`ui/input`** —— 各平台事件类型归一为跨平台一致的统一事件给上层；  
> ③ 多窗口：入口 **`application.New()`** + **`app.NewWindow()`**；kit 只做控件不建窗口；  
> ④ 控件层由 **框架统一事件绑定管理**（InputRouter），自定义控件实现统一回调接口即自动接线。  
> **v0.3（方案 A 第 1 步落地）：** `ui/input` 包已实现（Event/Key 逻辑键表/PointerEvent/Touch/Text/IME/FromPlatform），单测 16 项全绿，ui 层编译零回归。

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

## 1. 应用模型（多窗口）

```go
// ui/application（新包）—— 建议入口
app := application.New(application.Config{
    Name:    "myapp",
    Backend: platform.DisplayAuto,   // x11/wayland/auto；win32/appkit 后置
})

winA, err := app.NewWindow(application.WindowOptions{Title: "A", Width: 1200, Height: 800})
winB, err := app.NewWindow(application.WindowOptions{Title: "B", Width: 800, Height: 600})

winA.SetRoot(kit.App(/* 控件树，含 Input / ListView … */)) // kit 内容根
app.Run() // 运行全部窗口事件循环；主窗关闭 → 退出
```

| API | 职责 | 说明 |
|---|---|---|
| `application.New(cfg)` | 应用级初始化 | 平台后端注册、GPU device 共享策略、全局配置（多窗口共享） |
| `app.NewWindow(opts)` | **引擎窗口** | 建原生窗 → PresentTarget → RO 根 → InputRouter 自动接线 |
| `win.SetRoot(root)` | 挂控件树 | kit 控件树作为窗口内容 |
| `app.Run()` / `app.Quit()` | 多窗口事件泵 | 各窗口 WaitEvents → FromPlatform 归一 → 各窗 InputRouter |

**决策：kit 不提供 `kit.NewWindow`。** 理由：窗口 = 平台 + 引擎事务（句柄、PresentTarget、事件泵）；kit 若建窗必然 import `platform`，违反 `kit 永不碰原生` 纪律。kit 提供 `kit.App` 组件作窗口内控件根（对齐 `docs/antd/app.md` 的 message/notification 上下文语义）。

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

### 2.3 统一指针（鼠标 + 触摸归一，v0.3 已实现）

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

### 3.3 后端重写落点

| 文件 | 内容 |
|---|---|
| `ui/platform/window.go` | 新增：`Window`/`Open`/`Adopt`/注册表 |
| `ui/platform/x11_linux.go` | **重写**（非搬 exhost）：建窗与读事件分离、事件解析独立函数、XIM 能力探测 |
| `ui/platform/wayland_linux.go` | **重写**：wl 事件循环 + `zwp_text_input_v3` 绑定 |
| `ui/platform/win32_windows.go` | stub（接口齐） |
| `ui/platform/appkit_darwin.go` | stub（接口齐） |
| `ui/platform/ime.go` | 能力接口定义 |
| `ui/platform/stub_host.go` | 保留（测试宿主） |

`examples/exhost` 作废（删除或退化薄转发壳，22 窗迁 `application.New`，方式待定）。

---

## 4. L1：控件层框架统一事件绑定（InputRouter）

```go
// ui/rendering（或 ui/input）：自定义控件的统一交互回调接口 —— 实现即自动接线
type EventTarget interface {
    OnPointer(ev input.PointerEvent)
    OnKey(ev input.KeyEvent)
    OnText(ev input.TextEvent)
    OnIME(ev input.IMEEvent)
}

// ui/embedder/input.go —— 每窗口一个，PipelineApp 生命周期内自动运行
type InputRouter struct {
    hit   func(x, y float64) (overlay.Band, rendering.RenderObject, *overlay.Entry)
    focus *focus.Manager
    ime   platform.IME // nil = 无 IME
}

func (r *InputRouter) Route(ev input.Event) // 统一分发：
//   Pointer/Touch → HitTest → gestures 竞技场 → 命中 RO.OnPointer
//   Key → focus.HandleKey → 聚焦 RO.OnKey
//   Text/IME → textinput（聚焦控件）→ RO.OnText/OnIME
```

- `PipelineApp.Run` 不再把事件丢给示例 `OnEvent` 手写分发；改为：`platform.WaitEvents → input.FromPlatform → InputRouter.Route`。
- IME 生命周期自动：焦点进文本框 `ime.EnableIME(rect)`；预编辑 `SetComposing`；上屏 `Commit`；失焦 `DisableIME`。

---

## 5. L2 / L3 消费端改动

| 层 | 现状 | 改后 |
|---|---|---|
| `ui/gestures` | `FromPlatform(platform.Event)` | 直接消费 `input.PointerEvent`；`FromPlatform` 删除；多指并行 |
| `ui/focus` | `keys.go` 4 个硬编码键 | 键位表迁 `ui/input`；`KeyEvent` 带 `Modifiers` |
| `ui/textinput`（新增） | — | 编辑缓冲 + 光标/选区 + IME 预编辑 + 剪贴板 |
| `ui/kit`（未建，P7） | — | 控件实现 `EventTarget` 或依赖 kit 包装；`kit.App` 窗口内容根 |

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
| 2 | `ui/platform` 重写：`Window`/`Open`/`Adopt`/后端注册表（x11/wayland 重写，三分离） | 中（碰原生） | 22 窗跑通即可回退 | ⬜ |
| 3 | `ui/application` + `app.NewWindow` 多窗口 | 中 | 双窗同跑无串扰 | ⬜ |
| 4 | `embedder.InputRouter`：自动路由，示例删手写 HandleEvent | 中 | L2 shell 行为等价 | ⬜ |
| 5 | `ui/gestures` 多点 + `Event` 触摸归一 | 中 | 双指并行单测 | ⬜ |
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