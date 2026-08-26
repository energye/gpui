# 跨平台输入 / IME 分层方案（草案 v1.0）

> **性质：DRAFT** — 方向收敛中，待确认后转真源。不参与 §R/§W 关闭。  
> **并读：** [`ENGINE_FLUTTER_SKIA_ARCH.md`](./ENGINE_FLUTTER_SKIA_ARCH.md)（§3 Host 注入）· [`ENGINE_UI_WIDGET_RENDER.md`](./ENGINE_UI_WIDGET_RENDER.md)（§0.3 U3 IME 暂缓可预留 SPI）  
> **v0.2 修正（用户定，推翻 exhost 演进思路）：**  
> ① 平台窗口 + 原生事件统一在 **`ui/platform`** 管理（后端重写，非迁移 exhost）；  
> ② 事件抽象出独立层 **`ui/input`** —— 各平台事件类型归一为跨平台一致的统一事件给上层；  
> ③ 多窗口：入口 **`application.New()`** + **`app.NewWindow()`**；kit 只做控件不建窗口；  
> ④ 控件层由 **框架统一事件绑定管理**（InputRouter），自定义控件实现统一回调接口即自动接线。  
> **v0.3–v0.9（方案 A 第 1–6 步 + XIM 反攻）：** `ui/input` · `ui/platform` · `ui/application` · `InputRouter` · `gestures` 多点 · `textinput` + Wayland `zwp_text_input_v3` + X11 `XIM`，均已实现。  
> **v1.0（反攻复测 1：真桌面候选交互探索 + 存量 bug 修复）：** 修 probe `XKeyEvent` 结构体布局（serial 为 8 字节 `unsigned long`，@24 display/@32 window 正确对齐）→ **合成按键终于到达窗口**；暴露并修复 **存量引擎 bug：`XKeycodeToKeysym` 注册缺 `dpy` 参数**（真实签名 3 参 `(Display*, keycode, index)`，注册成 2 参致 C 层把 keycode 当 display 指针解引用 → 真实键盘输入崩溃/错误解码）；`EnableIME` 后实证 **`XFilterEvent=1`（ibus 接管按键进入组合状态）**——XIM 引擎链路完整激活。诚实边界：headless Xvfb 无物理显示/候选窗交互，拼音候选→上屏 commit 无法自动化验证，需真桌面复测。  
> **v1.1（存量问题清单落账，2026-08-25）：** 对 IME 全链路静态排查（platform 绑定 / input 归一 / embedder.InputRouter / textinput.Editor / 示例），产出 A 类真 bug 3 项（I1–I3）+ B 类能力缺口 3 项（I4–I6）+ C 类规范细节 4 项（I7–I10），账本见 §10；修复方向待确认后回流 wr-engine / wr-implement，销账纪律见 §10.3。  
> **v1.2（A 类三项修复落地，2026-08-25）：** I1/I2 ✅（新枚举 IMEDeleteSurrounding + compose 光标载荷语义修正，单测+全 ui 回归绿）；I3 🔶 引擎侧已实现并回归绿，X11 真桌面双场景复测待真机执行后转 ✅。详见 §9 v1.2 行与 §10.1 状态列。  
> **v1.3（IME 测试窗修复，2026-08-25）：** ui_textinput_ime 三处渲染洞修复（根盒无尺寸 / DrawString 无字体静默跳过 / 绕过 SetText），新增自测模式（注入真实 IME 事件序 + 最终帧快照 + JSON 判定）；真 Wayland 窗像素级证实文本渲染，live ibus 组合事件到达编辑器。详见 §9 v1.3 行。  
> **v1.4（候选窗锚点 + 光标闪烁，2026-08-25）：** 引擎新增 `platform.IME.UpdateCursorRect`（焦点刷新重发光标矩形 + 光标移动实时跟随）与 `RenderText.MeasureWidth`；示例光标闪烁、键事件全量日志、模式提示行。详见 §9 v1.4 行。  
> **v1.5（组合态卡死 + 闪烁修复，2026-08-25）：** 空 preedit 按「清除预编辑」语义处理（此前把提交后的残留空 preedit 当新组合开始 → 组合态锁死、英文无法直打、需按一次删除）；光标闪烁改调度器 Ticker 驱动并真正重绘。详见 §9 v1.5 行。  
> **v1.6（按键连发，2026-08-25）：** Wayland 客户端自动重复落地——解析 repeat_info + 按住非修饰键合成重复事件（最后按下胜出/焦点丢失取消/修饰键不重复）。详见 §9 v1.6 行。  
> **v1.7（通用性审计 + 集成路径，2026-08-25）：** 契约与上层全平台无关（grep 零平台词），能力深度因协议而异（矩阵见 §9 v1.7 行）；对接框架三步 = §10.2 I4/I5/I6。  
> **v1.8（I4/I5/I6 落地：焦点驱动 IME 会话，2026-08-25）：** 对接框架能力完成——控件实现 TextEditTarget 即自动获得完整 IME 会话。详见 §9 v1.8 行。  
> **v1.9（光标交互修复，2026-08-25）：** 方向键移动可见化（光标按 Cursor 偏移内联渲染）+ 点击定位（RenderText.ByteOffsetAt 引擎能力）；自测含方向键断言。详见 §9 v1.9 行。  
> **v2.0（悬浮光标条 + 洪泛修复，2026-08-25）：** 光标改 2px 色条悬浮定位（排版零影响）；指针移动不再触发 IME 协议推送（引擎切换恢复响应）；surrounding 去重。详见 §9 v2.0 行。  
> **v2.1（居中 + surrounding 关断 + 调试日志 + 回收检查，2026-08-25）：** 光标条垂直居中；surrounding 上报默认关闭（SurroundingUpdates 可选开启）；GPUI_IME_DEBUG=1 全链路协议打点；v1.6 重复测试竞态修复；§10 账本盘点完成。详见 §9 v2.1 行。  
> **v2.2（首次激活时机修复，2026-08-25）：** 获焦后 400ms 延迟补发 disable→enable 激活轮（复刻有效的手动失焦-回焦循环）；leave 补 disable。详见 §9 v2.2 行。  
> **v2.3（preedit 回声风暴修复，2026-08-25）：** 锚点推送去重（路由层 hasAnchor/lastAnchor + 平台层同矩形拒发）斩断 commit_state↔preedit 回声反馈环；真实引擎输出到达即撤销挂起的补发轮（防打断进行中组合）。详见 §9 v2.3 行。

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
| `ui/platform/x11_linux.go` | X11 后端重写：`x11Backend.Create/Adopt` + `x11Host` 事件泵 + 事件解析独立函数，`init()` 自注册；`XFilterEvent`/`Xutf8LookupString` 走 XIM（v0.9） | ✅ |
| `ui/platform/x11_xim_linux.go` | **XIM 绑定**（v0.9）：`XOpenIM`（前置 `XSetLocaleModifiers`）+ `XCreateIC`（变参 ABI 模拟）+ `XFilterEvent`/`Xutf8LookupString` + `XSetICFocus`；`x11Ime` 实现 `platform.IME`；可选能力静默降级 | ✅ |
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
| 6 | `ui/textinput` + `platform.IME` + Wayland `zwp_text_input_v3` 实证 | 高（真 IME 才算数） | **协议握手真窗 PASS**（IME AVAILABLE + EnableIME 无错）；完整候选交互留真桌面 | ✅ v0.8（协议层） |
| 7 | Win/mac 后端 + TSF/InputMethod（接口已预留） | 高 | 后置 | ⬜ |
| 8 | **反攻 2：X11 XIM 绑定**（XOpenIM/XCreateIC/XFilterEvent/Xutf8LookupString，PreeditNothing 模式） | 中 | **真窗 PASS**（ibus+libpinyin：IME AVAILABLE + EnableIME 无错） | ✅ v0.9（协议层） |
| 9 | **反攻复测 1：完整候选交互探索**（修 XKeyEvent 布局 + **XKeycodeToKeysym dpy 存量 bug** + XFilterEvent=1 激活实证） | 中 | 引擎链路激活 PASS；候选→上屏需真桌面 | ✅ v1.0（引擎层，候选留真桌面） |

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
| **v0.8** | **方案 A 第 6 步落地（协议层实证）**：① `ui/textinput` 编辑控制器（`editor.go`）——UTF-8 字节偏移缓冲/光标/选区/剪贴板/IME 预编辑生命周期（BeginCompose/Update/Commit/Cancel），`ApplyText`/`ApplyIME` 消费归一事件，15 项单测全绿；② `platform.Event` 加 `EventIME` + `IMEKind/IMEText/IMEStart/IMEEnd` 载荷，`input.FromPlatform` 归一；③ **Wayland `zwp_text_input_v3` 真实绑定**（`wayland_textinput_linux.go`）——registry 识别 `zwp_text_input_manager_v3` + `wl_seat`，`get_text_input(id, seat)`（**修复：协议签名 "no" 需要 seat，初版漏参致 `invalid arguments`**），preedit_string/commit_string/delete_surrounding_text 事件 → `platform.Event{EventIME}` → `wlHost.poll` 队列；`wlIme` 实现 `platform.IME`（EnableIME/SetComposing/Commit/DisableIME）；④ `InputRouter.TextEditor` 自动路由 Text/IME → 聚焦编辑器；`PipelineApp.SetInputRouter` + `application.Window.SetInput` 接线；⑤ **实证**：`examples/ui_ime_probe` 真 Wayland 窗（GNOME）——`IME capability AVAILABLE`、`EnableIME sent`、无协议错误（修复前后对照：修复前 mutter 报 `invalid arguments for ...get_text_input`）。`examples/ui_textinput_ime` 完整示例（GPU 无头环境 surface 配置受限，输入链路已由 probe 验证）。全 ui 16 包回归全绿。下一步：第 7 步 Win/mac 后端 + TSF/InputMethod（接口已预留），或反攻：X11 XIM 绑定 + 真桌面候选交互实证。 |
| **v0.9** | **反攻强化 2：X11 XIM 绑定实证（协议层）**：`ui/platform/x11_xim_linux.go`——`XOpenIM`（**前置 `XSetLocaleModifiers("")`：Go 无默认 locale，缺它 XOpenIM 恒返回 0**）+ `XCreateIC`（**变参经 amd64 SysV ABI 固定参数模拟**：`XCreateIC(im, n1, v1, n2, v2, 0)` 恰 6 寄存器参数）+ `XFilterEvent`/`Xutf8LookupString`（组合时消费按键跳过 plain key、提交时产出 UTF-8 commit → `platform.Event{EventIME}`，可打印键同时发 Key+Rune 供快捷键与 textinput 双通道）+ `XSetICFocus`/`XUnsetICFocus`；`XIMPreeditNothing(0x0002)|XIMStatusNothing(0x0010)` 模式（**修复：初版误用 None 位 0x0001/0x0002 致 XCreateIC 返回 0**）；`x11Ime` 实现 `platform.IME`，`ximForX11` 静默降级。**实证（真 X11 窗 + ibus + libpinyin）：`probe: IME capability AVAILABLE (x11)`、`EnableIME sent`、无协议错误**。诚实边界：完整候选交互（拼音候选 → 上屏）需真桌面 + 交互式键盘，headless Xvfb 下 XTest/XSendEvent 合成事件投递不可靠（焦点已确认 OUR-WINDOW），未虚标。XIM 单测 4 项 + 全 ui 16 包回归全绿。下一步：真桌面候选交互实证 / Win+mac TSF+InputMethod / 或转入 kit 层。 |
| **v1.0** | **反攻复测 1：真桌面候选交互探索（headless 自动化到极限）**：① 修 `ui_ime_probe` 的 `XKeyEvent` 结构体布局（`serial` 是 8 字节 `unsigned long` @8，`display` @24、`window` @32——初版按 4 字节写 offset 16/24 全错，合成事件从未投递）→ **合成按键终于到达窗口**；② 修复**存量引擎 bug：`XKeycodeToKeysym` 注册缺 `dpy` 参数**（真实签名 3 参 `(Display*, keycode, index)`，`x11Lib.keycodeToKeysym`/`x11State` 字段/闭包/decodeKey 调用点全改，初版注册 2 参致 C 层把 keycode 当 display 指针解引用 → 真实键盘输入崩溃或错误 keysym；headless 之前无真实键事件未触发）；③ **XIM 引擎链路激活实证**：`EnableIME`（XSetICFocus）后 `XFilterEvent=1`——按键被 ibus 输入法接管进入组合状态，KEY 事件正确解码（`KEY code=97 rune='a'`）；④ 拼音序列 `nihao ` + 空格投递：headless Xvfb 无物理显示/候选窗交互，commit 未产出（诚实边界：完整候选→上屏需真桌面）。`XKeycodeToKeysym` 修复经全 ui 16 包回归验证零回归。下一步：真桌面（有显示+交互键盘）复测完整候选交互。 |
| **v1.1** | **存量问题清单落账（§10 新增）**：IME 全链路静态排查产出 10 项——A 类真 bug：I1 delete_surrounding_text 死路（编辑器未实现 Start<0 删除语义，绑定层注释失实）、I2 preedit 光标字段错位（End 装 commit 布尔被当字节偏移用，真实 index 无人消费）、I3 X11 双通道双插入（commit 事件 + 可打印 rune 双插，`ComposeActive` 防线挡不住英文直通与选字空格；未被踩中仅因无 X11 示例挂 TextEditor）；B 类缺口：I4 IME 生命周期无自动接线（DisableIME 零调用 / cursor rect 仅 Enable 发一次 / surrounding text 从不上报）、I5 router.TextEditor 静态单例不跟焦点、I6 content_type 写死；C 类细节：I7 done(serial) 无视（pending state 规范偏差）、I8 leave 组合残留、I9 平台命令侧 Commit 语义空转、I10 X11 PreeditNothing 下 rect 无处安放（接受后置）。证据全部 file:line 可溯源（§10.1），修复方向候选与销账纪律见 §10.2/§10.3。**同日用户逐项定夺**：I1 新增 `IMEDeleteSurrounding` 枚举、I2 丢弃 commit 标志、I3 平台层剥 Rune、本轮范围先修 A 类三项（I1+I2 一批 → I3），§10 头部与 §10.2 已按定夺收敛为唯一方案。 |
| **v1.2** | **A 类三项修复落地（I1/I2/I3，2026-08-25）**：① I1——`ui/input/ime.go` 新增第四枚举 `IMEDeleteSurrounding`（载荷 Start=-before / End=after 字节计数）；`editor.go` 新增 `DeleteSurrounding(before, after)`（以光标为中心删除、UTF-8 rune 边界外向吸附防劈字、跨越预编辑区取消组合否则平移组合偏移），`ApplyIME` 加分支；`fromplatform.go` kind 钳制边界放宽到新枚举（越界仍钳 Compose）；Wayland `wlTiDeleteSurr` 改发 kind 3 并改正失实注释。② I2——compose 事件载荷改 `Start=End=index`（-1=串尾），zwp commit 布尔不再透传（真正上屏必有 commit_string 跟随，对齐 Flutter composing 形态）；`UpdateCompose` 兑现"负 cursor=串尾"；`caretFromIME` 改读 Start。③ I3——`x11_linux.go` key case 增 `keyCommitted` 门控：XIM 已为该键产出上屏文本时 decodeKey 结果剥 Rune（文本只走 IME 上屏通道，Key 留给快捷键；无 XIM 时行为不变）。验证：新增单测 7 项（编辑器删除/光标/组合平移取消 + fromIME 映射边界）全绿；`go build ./ui/...` + 全 ui 18 包回归全绿。**诚实边界**：I3 的 X11 真桌面双场景复测（英文直通逐键不双插 / 拼音选字空格不多格）待真机执行，复测过 §10.1 I3 转 ✅。 |
| **v1.3** | **ui_textinput_ime 示例修复为可真窗测试（2026-08-25）**：该示例自 v0.8 起从未在真 GPU 窗渲染出文字（当时仅 headless 验证了输入链路）。三处叠加洞——① 根节点用无尺寸 `RenderBox`，布局塌零、整树不可见 → 改 `NewAbsoluteBox(winW, winH)` + `Place`（对齐 ui_l1/ui_wr 真窗模式）；② `RenderText` 未设 `Face`，`DrawString` 无字体**静默不画**（布局有尺寸但像素为零）→ 挂 `text.LoadMultiFace(20)` 多脚本链（DejaVu + Noto Sans CJK + …，上屏汉字有字形）；③ 文本更新绕过 `SetText` 直写 `Text` 字段、漏布局失效 → 改 `SetText`。示例同步升级为真窗直连模式（`platform.Open` + `NewPipelineApp`，InputRouter 接线与归一化路径不变），并新增**自测模式** `GPUI_IME_DEMO_SELFTEST=1`：预循环经 `RoutePlatform` 走生产归一化链注入真实 IME 事件序（compose "ni" → compose "nihao" → commit "你好" → 裸键 'a'），`RunFor` 限时退出时 `SnapshotPath` 存最终帧，JSON 判定缓冲区 == "你好a"。**像素证据**：框区 493 个非背景像素，bbox (40,62)-(96,84) 宽度与 "你好a"+光标测量值吻合；ASCII 像素可视化确认汉字字形。**附带实证**：交互冒烟 8s 内真实 ibus compose 事件到达编辑器（live 链路通）。运行需 `WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so` + `LD_LIBRARY_PATH=$PWD/lib`。 |
| **v1.4** | **测试窗四项体验修复 + IME 光标矩形跟踪落地（2026-08-25）**：① 候选窗位置错——根因 `refreshTextInput` 焦点刷新只重发 enable+content_type+commit，**丢了 pending set_cursor_rectangle**，mutter 侧候选窗锚到 surface 原点；且光标移动从不上报。引擎修：`platform.IME` 新增 `UpdateCursorRect(rect)`（wlIme 实现 set_cursor_rectangle+commit+flush；x11Ime PreeditNothing 下 no-op），`wlTIState` 存最近矩形、`refreshTextInput` 重发；新增 `rendering.RenderText.MeasureWidth(s)` 公开测量（caret x = 前缀宽）。示例 sync 时以「boxX+前缀宽」跟随更新锚点（去重发送）。② 光标不闪——示例加 530ms 闪烁 ticker（atomic 态 + ScheduleFrame，Run 返回即停）。③④ 英文/符号与退格——引擎链路已被自测证实（生产归一化路径注入 'a' 上屏；editor 单测覆盖删除）；中文模式下 ASCII 被输入法吃进组合属 OS 行为（GTK 同款），示例新增**全量 key-event 日志**与模式提示行，用户切英文态后若仍无 key-event 日志即为组合器层问题再回流 wr-engine。回归：全 ui 包构建通过、自测 JSON pass=true、像素化验 cyan(text)=281/gray(title)=707 不变。 |
| **v1.5** | **组合态卡死（空 preedit）+ 光标闪烁真修复（2026-08-25，用户实测反馈）**：① 「输完中文切英文态无法直打、宽光标（▌）、必须按一次删除才能输入」——根因：zwp_text_input_v3 `preedit_string` 空文本的协议语义是**清除预编辑**，`editor.ApplyIME` 却把它当新组合 `BeginCompose("")` → 上屏后残留空组合态，routeKey 的 ComposeActive 守卫锁死所有直打插入，示例的 ▌ 宽块标记即"很宽的光标"，一次退格触发 DeleteBackward→CancelCompose 才解锁——与用户描述逐字吻合。修：compose 事件 `Text==""` 一律走 CancelCompose（有活动组合则回滚预编辑，无活动则保持非活动），commit_string 无论先到后到均正确。② 光标不闪——v1.4 的后台 goroutine 只翻状态标志未重绘文本；改为调度器 Ticker（`app.Scheduler().Tickers().Add`，UI 线程每帧 Tick(dt 秒)，翻转 + sync 重绘），blink 计数进退出 JSON 作客观证据（3s 实测 5 次）。验证：自测新增回归步 commit→empty-preedit→'a' 全绿（step3 text="你好" compose=false）；textinput/input/embedder 单测绿；全 ui 回归绿。 |
| **v1.8** | **I4/I5/I6 落地：焦点驱动 IME 会话 + 动态目标解析 + 内容用途（2026-08-25，wr-implement 流程）**：① I5——`focus.FocusNode` 增 `Target any` 载荷（框架据此把焦点节点还原成控件）；`InputRouter.editorFor()` 动态解析：焦点目标的编辑器优先、静态 `TextEditor` 降为无焦点回退（旧用例零破坏）。② I4——embedder 新增 `TextEditTarget{Editor, IMERect, ContentPurpose}` 契约与 `AttachIME(ime)`：经 `FocusManager.AddFocusObserver`（新增管理器级观察者）在焦点迁移时自动开关会话——获焦：`SetContentType(purpose)` → `EnableIME(锚点矩形)` → 推送 surrounding text；失焦：`CancelCompose`（回滚活预编辑）→ `DisableIME`；每次文本/IME/按键事件应用后 `afterEdit()` 刷新候选锚点与 surrounding（surrounding **剔除预编辑区**并平移光标偏移）；程序化改动走公开 `RefreshIMEAnchor()`。接线：`PipelineOptions.IME` → NewPipelineApp 自动 AttachIME；application.SetRoot 透传 `w.plat.IME()`。③ I6——`platform.ContentPurpose` 枚举（值对齐 zwp content_purpose，稳定测试锁定）+ `IME.SetContentType`：wlIme 存档随每次状态提交发送、x11Ime no-op。④ 示例改造为纯消费者：inputBox 实现 TextEditTarget、焦点节点 Target 回指、点击=RequestFocus、删除全部手工 EnableIME/refocus/锚点代码。⑤ 测试：focus 观察者迁移序列 + Target 载荷、embedder 会话自动化（获焦三连调用/动态解析插入/组合态 surrounding 剔除断言/失焦禁用）+ 静态回退、platform purpose 线值锁 + nil 安全，全绿；真窗自测 `stage=focused (session auto-opened)` 证明自动会话走真实协议无错，注入流经焦点目标解析，JSON pass=true；全 ui 回归零失败。 |
| **v1.9** | **光标交互修复：方向键移动可见 + 点击定位（2026-08-25，用户实测反馈）**：① 方向键——事件链路本就通（keysym 0xff51–54 → KeyArrow* → MoveCaretRunes），但示例把光标渲染成**固定追加在文本末尾的 `\|`**，内部光标移了画面不动；修 `sync()`：光标字符插入到 `ed.Cursor()` 字节偏移处（组合态 ▌ 同理落在组合光标处）。② 点击定位——OnPointer 此前只 RequestFocus 不换算坐标；引擎新增 `RenderText.ByteOffsetAt(x)`（单行 x→最近 UTF-8 字节边界，rune 边界吸附防劈字，越界钳制），示例点击时 `SetCaret(ByteOffsetAt(ev.X - boxX))`；`routePointer` 尾部补 `afterEdit()` 使点击后的锚点/surrounding 同步刷新。③ 自测升级为方向键断言：commit→'a'→ArrowLeft→'b' ⇒ "你好ba"（b 插在 a 前 = 光标确实移动）。验证：ByteOffsetAt 单测（钳制/rune 边界/单调性/空文本）绿；真窗快照 ASCII 可视化确认光标画在 b|a 之间；全 ui 回归零失败。 |
| **v2.0** | **光标条悬浮化 + 指针移动洪泛修复（2026-08-25，用户实测反馈三问题）**：① 「输入法不能切换」——v1.9 把 afterEdit 挂在 routePointer 尾部且**不分事件种类**，鼠标每次移动都推 UpdateCursorRect+SetComposing(commit_state)，数百次/秒的协议洪泛把 ibus/mutter 喂到无法响应引擎切换；修：仅 PointerDown 刷新（motion/up 永不编辑）+ surrounding 推送去重（同态跳过，会话切换时强制重推）。② 「光标把文本拆开、显示时左右间距变大」——光标是插入字符串的字面 `\|` 字符，占真实字宽且随闪烁增删；修：光标改**悬浮色条**（2×26 RenderColorBox 定位在 MeasureWidth(前缀)+1 处，闪烁只切 alpha），文本串与编辑器缓冲恒等、排版零影响。③ 「点击不能定位」——根因即②：被拆开的串使点击映射失准；串对齐后 ByteOffsetAt 精确生效。回归测试新增 TestRouter_PointerMotionDoesNotSpamIME（50 次 move 零推送 / down 刷新锚点）；自测含方向键断言 "你好ba" 全绿；快照确认光标条立于 b|a 之间；除并行线在途的 ui/scene/textured.go vet 错误（非本线文件）外全 ui 包零失败。 |
| **v2.1** | **光标垂直居中 + surrounding 流量关断 + 调试日志（2026-08-25，用户实测反馈 + 回收检查）**：① 光标条 Y 定位从硬编码 6 改为按文本行盒垂直居中 `(textHeight - barHeight)/2`（负值钳 0）。② 「还是不能切换」——v2.0 去重未关断，surrounding 上报仍是每次编辑一条 commit_state 往返且为 v1.8 新增流量（出现即伴随切换失灵）；降级为 `InputRouter.SurroundingUpdates bool` **默认关闭**的可选项（需要上下文感知 IME 特性时显式开启），协议流量回到 v1.7 水平；embedder 会话套件测试显式开启以维持断言覆盖。③ 证据链基建：`GPUI_IME_DEBUG=1` 时 embedder 层（会话开/关、purpose/rect）与 Wayland 绑定层（enable/disable/commit 轮次、preedit/commit 事件）全量打点——下次复现有日志可依。④ 回收检查产出：v1.6 按键重复测试存在取消竞态（timer 协程与 cancel 竞争最后一推），改「清在途→断言静默」确定性写法后 count=3 全过；§10 账本盘点 I1/I2/I4/I5/I6 ✅、I3 🔶 待 X11 真桌面复测、I7–I9 ⬜ 低优先、I10 接受。验证：六核心包回归绿（ui/scene 为并行线 WIP vet 错误非本线）；自测 JSON pass=true；快照光标居中。 |
| **v2.2** | **首次激活时机修复（2026-08-25，用户实测日志定位）**：用户 GPUI_IME_DEBUG 日志显示——surrounding 关断后切换依旧失败，且 key-event 正常到达（键盘链路无恙）；关键线索：**失焦再回焦就能切**，而回焦路径 = refreshTextInput 的 enable+commit 轮次。结论：窗口刚显示时 ibus 引擎尚未绑定新 surface，首次激活轮落空；之后完整的焦点循环（leave→disable / enter→enable）才把引擎接上。修：① `refreshTextInput` 后 400ms 自动补发一轮 **disable→enable→rect/purpose→commit**（复刻有效的手动失焦-回焦序列；新一轮取消旧定时器不叠加洪泛）；② `wlTiLeave` 补发 disable+commit（此前空操作，开合循环缺前半段）；③ destroy 停挂起定时器。自测日志实证 `delayed re-activation round (disable+enable+commit)` 按时触发；四包回归全绿。待真机确认切换恢复后销账。 |
| **v2.3** | **preedit 回声风暴修复（2026-08-25，用户实测日志定位）**：现象——中文态按一个字母产生几十条重复 `preedit "f"` 事件、候选被扰乱上屏错字。根因（日志链）：组合器对**每条 commit_state 都回声重发当前 preedit**；v1.9 重构把示例层锚点去重丢进路由层时未实现去重 → 应用回声事件触发 afterEdit 无条件 UpdateCursorRect（=新 commit_state）→ 引发下一轮回声，反馈环直到组合结束。修三处：① 路由层 `afterEdit` 锚点去重（hasAnchor/lastAnchor，同矩形不发包）；② 平台层 `UpdateCursorRect` 同矩形拒发（双保险）；③ **真实引擎输出（preedit/commit 事件）到达即撤销挂起的延迟补发轮**——补发轮只为「首激活落空」兜底，引擎已活时 firing 只会打断进行中组合。验证：自测协议流量从风暴收敛为 6 条 cursor-rect（每真实移动各一条）、零回声；方向键断言 "你好ba" 全绿；四包回归全绿。真机复测项：切换响应、单字母单条 preedit、候选正常。 |
| **v2.4** | **光标几何标准化 + 垂直移动 + 候选锚点统一（2026-08-26，用户四问题反馈收敛）**：用户报 ①光标条不在两字中间/位置不当 ②中英混合中移动后条位错 ③方向键不能上下移行 ④左右移动在混合文本中落点错。按标准模型（Flutter TextPainter.getOffsetForCaret / Skia）结构性重做，废弃此前的「墨迹间隙居中」非标准方案（该方案逐字符扫笔画框，遇空格/全角标点把间隙算错位——②④的机制性根因）：① 引擎新增 `RenderText.CaretColumn(off)`（行号+行内笔位边界；条中心钉笔位=任意两字符的正中间，latin/CJK/混合通用）与 `RenderText.GlyphInkBounds`（字形墨迹框通用访问器）；② 引擎 `Editor.MoveCaretVertically`（粘滞列模型=Flutter desired-X：上下移动记忆目标列 x，水平移动/点击/SetCaret 重置；换行处边界归属下一行列0）+ `ResetCaretColumn`；示例 OnKey 接 KeyArrowUp/Down（经生产路由路径）；③ 示例新增 `caretAnchor()` 单一几何来源——可见条 layoutCaret 与 IME 候选锚点 IMERect 共用（修复候选窗 X 用全前缀宽测量致多行飞出、Y 恒盒顶不随行的双洞）；④ 验收场景扩到 6 个确定性真窗（multiline/composing/composing-mid/composing-cjk/mixed-nav/click），像素放大图逐场景断言条两侧零字形重叠；混合导航走真实按键路径 Right×3+Down 数值核对（cursor=29 行尾吸附 ✓ 条 bbox 与 penX+boxX 精确吻合）。测试：vertical_caret_test.go 6 项 + GlyphInkBounds 1 项 + ComposedView MapBufToView 补充断言；六场景快照回归 bbox 一致；SELFTEST 连跑 5 次 pass:true；全 ui 包回归绿。诚实边界：P2/P3/P8 真机交互复测仍待用户执行。 |

| **v1.6** | **Wayland 客户端按键重复落地（2026-08-25，用户实测反馈：按住不松不连发）**：协议事实——wl_keyboard 只发一次 `repeat_info(rate=keys/s, delay=ms)`，**重复按键由客户端合成**（无服务端 autorepeat）；原实现 `wlKbRepeatCB` 空函数、无任何定时器 → 按住永远单发。实现（`wayland_keyboard_linux.go`）：① `wlKeyboardState` 增重复态（rate/delay/heldKC/heldKS/timer，默认 25/s+400ms 兜底）；② `wlKbRepeatCB` 解析存档；③ `wlKbKeyCB` 非修饰键按下 armRepeat（最后按下的非修饰键胜出，替换旧 timer）、该键释放 cancelRepeatIf（其它键的释放不打断）、修饰键/未知 keysym 不武装（Shift 按住不刷 Shift）；④ `wlKbLeaveCB` 焦点丢失取消（防 alt-tab 后无限连发）；⑤ destroy 取消；⑥ 事件构造抽 `keysymEvent` 与 press 路径共享（Rune 推导一致）。X11 为服务端自动重复，无需改动。测试：新增 3 项（repeat_info 解析 / 按住连发+无关释放不断+本键释放停 / 修饰键不武装）全绿，全 ui 回归绿。 |
| **v1.7** | **跨平台通用性审计 + 对接框架集成路径定稿（2026-08-25）**：结论——**契约与上层完全通用，能力深度因协议而异**。证据：`ui/input`/`ui/textinput` 全包 grep 零平台词；`ui/platform/ime.go`（41 行）为唯一契约；两后端各自实现（zwp 488 行 / XIM 220 行）经 `Window.IME()`（application 层 :265 透传）统一暴露，nil=静默降级；win32/appkit stub 无 IME。能力矩阵：commit 两端✅；客户端预编辑仅 Wayland✅（X11 PreeditNothing 由 ibus 自绘候选窗）；cursor rect 仅 Wayland✅（v1.4，X11 需 PreeditPosition 另立里程碑）；delete-surrounding 仅 Wayland 有事件源；content_type 两端写死（I6）；按键重复两端✅（v1.6 客户端合成 / X11 服务端）。**对接框架三步**（即 §10.2 I4/I5/I6，无新增项）：① embedder 层 `TextEditTarget{Editor,IMERect,ContentPurpose}` + router 焦点回调自动 Enable/Disable/CancelCompose/UpdateCursorRect/surrounding；② router.TextEditor 改跟随焦点动态解析；③ content purpose 枚举。完成后 kit 控件实现接口即得完整 IME，多窗口天然支持（每窗独立 host 能力）；win/mac 后续以 TSF/InputMethod 实现同一接口，上层零改动。 |

---

## 10. 存量问题清单（v1.1 · 2026-08-25 排查）

> 性质：**待修账本**。来源 = IME 全链路静态走读（`ui/platform` 绑定 / `ui/input` 归一 / `ui/embedder.InputRouter` / `ui/textinput.Editor` / 两个示例）+ 全仓 grep 调用面核对；证据一律 file:line 可溯源。
> 分级：**A = 真 bug/死路**（引擎内行为错误或功能静默失效）；**B = 能力缺口**（方案已承诺、未落地）；**C = 规范/契约细节**（暂无实测危害）。
> **定夺记录（2026-08-25，用户逐项确认）**：I1 = 新增 `IMEDeleteSurrounding` 枚举载体；I2 = 丢弃协议 commit 布尔标志（End 改装光标 index）；I3 = 平台层剥 Rune（所有权切割）；**本轮范围 = A 类三项（I1+I2 一批 → I3），B/C 类留下一轮**。§10.2 各项已按定夺收敛为唯一方案。
> 边界：本轮不含真桌面交互验证（沿用 v1.0 诚实边界：候选→上屏需真桌面）；本线不在 §R/§W 关闭矩阵内，修复走 wr-engine/wr-implement 回流，修完在此表销账。

### 10.1 总表

| # | 级 | 问题 | 证据（file:line） | 状态 |
|---|---|------|--------------------|------|
| I1 | A | **delete_surrounding_text 死路**：Wayland 绑定把它转成 `IMEKind=2, Start=-before, End=after`，注释声称"编辑器把 Start<0 解释为删除请求"；实际 `ApplyIME` 对 CaretMove 要求 `Start>=0 && End>=0`，负数静默丢弃 → 输入法的删周围文本请求永远无效 | `wayland_textinput_linux.go:410-421`（失实注释）vs `editor.go:345-349` | ✅ v1.2（2026-08-25：新增 `IMEDeleteSurrounding` 枚举 + 编辑器 `DeleteSurrounding`（rune 边界吸附/组合区平移与取消）+ fromIME 钳制边界修正 + 绑定映射改 kind 3，单测绿） |
| I2 | A | **preedit 光标字段错位**：`wlTiPreedit` 把协议 `index`（组合串内真实光标字节偏移）装进 `IMEStart`、把 `commit` **布尔标志**装进 `IMEEnd`；`caretFromIME` 却取 `End` 当光标偏移 → commit=1 时（fcitx5 常见）光标钉在第 1 字节处（中文多字节正好劈半个字），真实 index 无人消费 | `wayland_textinput_linux.go:377-396`；`editor.go:352-358` | ✅ v1.2（2026-08-25：compose 载荷改 Start=End=index（-1=串尾）、commit 布尔不再透传、`UpdateCompose` 兑现"-1=串尾"、`caretFromIME` 改读 Start，单测绿） |
| I3 | A | **X11 双通道双插入**：XIM 上屏时同一按键同时产出 commit 事件与带 Rune 的 Key 事件；`InputRouter.routeKey` 在 `!ComposeActive()` 时再插一遍 rune → 英文直通模式每键双插（"aa"）、选字空格在汉字后多插一格。现未被踩中仅因 `ui_textinput_ime` 写死 Wayland、无 X11 示例挂 TextEditor | `x11_linux.go:1054-1080`（filter 后仍 decodeKey）+ `:1146-1169`（rune 赋值 1153-1155）；`input_router.go:158-176` | 🔶 v1.2 已实现（XIM 产出上屏文本的按键剥 Rune，keyCommitted 门控）+ 全 ui 回归绿；**X11 真桌面双场景复测待跑**（英文直通逐键 / 拼音选字空格），复测过再转 ✅ |
| I4 | B | **IME 生命周期无自动接线**：`DisableIME` 全仓零调用；`EnableIME` 仅示例手动调一次；`set_cursor_rectangle` 只在 Enable 时发一次（光标移动/滚动后候选窗不跟随）；surrounding text 引擎侧从不上报（`IME.SetComposing` 生产侧零调用） | `examples/ui_textinput_ime/main.go:163-171`；`wayland_textinput_linux.go:227-277`；全仓 grep | ✅ v1.8（embedder.TextEditTarget{Editor,IMERect,ContentPurpose}；InputRouter.AttachIME + FocusManager 观察者自动会话：获焦 SetContentType+EnableIME+surrounding、失焦 CancelCompose+DisableIME；afterEdit 逐编辑刷新锚点与上下文（surrounding 剔除预编辑区）；PipelineOptions.IME + application 透传；示例已改为纯焦点驱动消费） |
| I5 | B | **TextEditor 静态单例不跟焦点**：router 一窗只认一个编辑目标；`ui/focus` ↔ 编辑器 ↔ IME 无联动，多输入框无法正确路由，失焦也不取消组合 | `input_router.go:46-52,116-129` | ✅ v1.8（FocusNode.Target 载荷 + FocusManager.AddFocusObserver；router.editorFor 焦点动态解析，静态 TextEditor 降为无焦点时的回退） |
| I6 | B | **content_type 写死** `hint=None/purpose=Normal`：控件无法声明数字/邮箱/密码等用途，输入法无法切对应键盘布局 | `wayland_textinput_linux.go:62-66,246-248,318-320` | ✅ v1.8（platform.ContentPurpose 枚举对齐 zwp content_purpose 线值并加稳定测试；IME.SetContentType 接口方法——wlIme 存档随每次 commit 发送、x11Ime no-op） |
| I7 | C | **done(serial) 无视**：按 zwp_text_input_v3，preedit/commit 属 pending state、应等 `done` 才生效；现随到随应用。实践组合器紧随 done 未出事，乱序风险低但违反规范 | `wayland_textinput_linux.go:423-430`（`wlTiDone` 空函数） | ⬜ 记录偏差/择机 |
| I8 | C | **leave 组合残留**：`wlTiLeave` 空操作，IME 焦点丢失若正在组合，预编辑残留在缓冲区 | `wayland_textinput_linux.go:364-375` | ⬜ 待修（小） |
| I9 | C | **平台命令侧 Commit 语义空转**：`wlIme.Commit` 只是重发 surrounding text（非协议动作）；XIM 侧 `SetComposing/Commit` 双 no-op。接口契约与实现脱节 | `wayland_textinput_linux.go:275-277`；`x11_xim_linux.go:195-214` | ⬜ 待澄清 |
| I10 | C | **X11 rect 无处安放**：PreeditNothing 模式下 `EnableIME(rect)` 忽略坐标（候选窗定位不支持；模式选择的代价，注释已声明） | `x11_xim_linux.go:196-200` | ✅ 接受/后置 |

### 10.2 修复需求要点（I1–I3 已按定夺收敛为唯一方案；I4–I10 方向候选，实施前再确认）

- **I1【已定夺：新增枚举】**：`ui/input` 新增第四个 `IMEKind = IMEDeleteSurrounding`（载荷沿用 `Start=-before, End=after` 语义，实施时可改为正名字段），编辑器新增「以光标为中心前删 before / 后删 after 字节」方法并接入 `ApplyIME` 新分支；同步修两处连带——`fromplatform.go:38-45` 的未知 kind 钳制边界（现越界一律钳成 Compose，不改会把删除请求又变回组合）与 `wlTiDeleteSurr:410-421` 失实注释。验收：editor 删除语义单测 + fromIME 归一/钳制边界单测。
- **I2【已定夺：丢弃 commit 标志】**：compose 事件载荷改为 `Start=index、End=index`（-1=串尾，对齐 `IMEEvent.End` 既有约定"-1=end"）；协议 `commit` 布尔不再透传——真正上屏必有 commit_string 跟随，标志仅提示性（对齐 Flutter composing 通道只传 range 的形态）。须同步改 `wlTiPreedit:377-396` 映射与 `caretFromIME:352-358`（改读 Start）。验收：绑定层映射单测 + editor 组合光标位置单测。
- **I3【已定夺：平台层剥 Rune】**：`x11_linux.go:1054-1080` key case——当 `h.xim != nil && committed != ""`（XIM 已为该按键产出上屏文本）时，decodeKey 结果的 `ev.Rune` 清零再入队：文本只走 IME 上屏通道、Key 事件照发但无 rune（快捷键 OnKey/focus 不受影响）；`h.xim == nil`（无输入法服务器）rune 行为不变。与 Wayland「有 IME 时文本只走一条道」形态对齐。验收：单测 + **X11 真窗 probe 双场景复测**（英文直通逐键不双插 / 拼音选字空格不多格）。
- **I4+I5（合并为一项工作：焦点驱动 IME 会话）**：定义编辑目标接口（`Editor() / IMERect() / ContentType()` 形态），router 挂焦点回调——获焦文本目标 → `EnableIME(rect)`；失焦 → `CancelCompose + DisableIME`；光标移动/滚动逐帧合并更新 cursor rect；buffer 变更合并上报 surrounding text（cause=input-method）。依赖约束：接口放 embedder 层（`focus` 与 `textinput` 互不 import）。**【v1.4 部分落地】**光标矩形跟踪已进引擎（`platform.IME.UpdateCursorRect` + refreshTextInput 重发），示例以直接持有 IME 能力的方式先行消费；完整的焦点↔会话自动接线仍待做。
- **I6**：`platform.IME` 增内容类型声明（枚举对齐 zwp content_purpose 族），Wayland 直连 `set_content_type`，X11 no-op。
- **I7**：记录为已知规范偏差；若真桌面复测出现乱序再加 pending-state 队列（done(serial) 到达才 flush）。
- **I8**：leave 时由绑定层推一条显式取消事件（配合 I1 若新增 kind 则一并复用），编辑器收后 `CancelCompose`；不得伪造空 compose（历史注释已警告会破坏生命周期）。
- **I9**：澄清契约——Wayland/X11 下客户端不主动命令提交（提交权在输入法侧）：或文档说明 `Commit` 为保留接口，或从接口移除（涉 `platform.IME` 面，实施时评估 probe 兼容）。
- **I10**：接受现状；若将来需要 X11 候选窗定位，另立里程碑升 PreeditPosition 模式（XNSpotLocation + fontset，工作量另估）。

### 10.3 销账纪律

- 每项修完：相关包 `go test ./ui/...` 回归 + 更新本表状态列（⬜→✅ 并注明版本）+ §9 追加修订行；涉平台绑定行为变更的，probe 真窗复测并留日志证据。
- I1/I2/I3 属引擎洞，禁止在示例层绕过（AGENTS.md 硬约束）。