# 平台窗口统一 API — 设计真源

> **版本：v2.0** | 日期：2026-08-12  
> **地位：** `ui/platform` 窗口接口（Options / Window / Host / WindowController / Event）唯一设计真源，**口径：四平台统一语义**。
> **并读：** [`ENGINE_WAYLAND_WINDOW_STANDARD.md`](./ENGINE_WAYLAND_WINDOW_STANDARD.md)（Wayland 标准窗口 CSD）· [`ENGINE_ARCH_OVERVIEW.md`](./ENGINE_ARCH_OVERVIEW.md)（分层）· [`ENGINE_INPUT_IME_PLAN.md`](./ENGINE_INPUT_IME_PLAN.md)（IME）  
> **对标参考：** winit（Rust 窗口库）· GTK4（GtkWindow/GdkToplevel）· sctk（wayland-client 壳）· Zed gpui · Flutter（WindowOptions）。

---

## 0. 用户要求（验收与范围 · 不可删改义）

- P1：**对上层提供统一窗口接口**，语义对四平台完全一致：Linux X11 / Linux Wayland / Windows Win32 / macOS Cocoa。上层代码只写一次，跑四平台。
- P2：统一接口具备**窗口标准功能 + 属性控制 API**（标题、尺寸、约束、位置、最小化/最大化/全屏、显隐、焦点、置顶、光标等）。
- P3：**先实现 Linux 两端（X11 + Wayland）**；Win32 / AppKit 后端注册占位、返回明确错误。
- P4：平台做不到的操作返回 `ErrUnsupported`，上层静默降级，禁止假实现。
- P5（口径）：**接口语义以四平台统一目标定义**——接口形状由四平台能共同表达的最高语义决定，不由"Linux 目前能做多少"决定；某平台实现能力不足时在该平台列标 ⛔（如 Wayland 无客户端定位），不缩水接口语义（如 EventMove 在 Win32/macOS/X11 都是真事件，Wayland 是平台缺口）。
- P6（验收）：**每个平台原生实现必须有对应的真窗完整功能测试**——X11 / Wayland / Win32 / AppKit 各自独立 `examples/ui_pf_*` 真窗例程（同一共享驱动）+ 平台包内原生单测（build tag，直接断言原生行为）+ 上层抽象接口测试（`examples/pfkit/pfkit_test.go`，无窗口确定性断言）。三层缺一不可，见 §2.6。

---

## 1. 架构分层

```text
上层（ui/application 等）        ── 只认识统一 API（四平台无差别），不知道平台
        │
ui/platform                      ── Window（门面）+ Host（事件泵）+ WindowController（窗口控制 SPI）
        │                                 + Options（创建属性）+ Event（平台事件）
        ├── x11_linux.go      (Xlib, purego)        ✅ 已落地（Create/Adopt/Host/IME）
        ├── wayland_linux.go  (libwayland, purego)  ✅ 已落地（含 CSD 标准窗口）
        ├── win32_windows.go  (Win32 API)           ⬜ 占位（注册中，Create 返回"未实现"）
        └── appkit_darwin.go  (Cocoa/AppKit)        ⬜ 占位（注册中，Create 返回"未实现"）
```

- **门面模式**：`platform.Open(opts)` / `platform.Adopt(ns)` 返回 `*Window`，上层不 import 任何 `*_linux.go` / `*_windows.go` / `*_darwin.go`。
- **可选能力探测**：`Window.IME()` / `Window.Clipboard()` / `Window.Controls()` 可能为 nil（后端不支持时静默降级）。
- **禁止 CGO**：全部原生调用走 purego（`ENGINE_CODING_RULES.md`）。

### 1.1 分层硬规则（跨平台根基 · 不可违背）

1. **上层只认门面**：`ui/` 除 `platform` 门面包外，不得 import 任何平台实现文件（按 build tag 编译的隐藏层）。
2. **平台实现互不借用**：x11 / wayland / win32 / appkit 四个后端各自独立，内部类型（Display*/HWND/NSView* 等）不出对应 backend 文件集合。
3. **禁止反向依赖**：`ui/platform` 不得 import `ui/` 上层包；平台事件在上层消化，平台层不回调上层。
4. **接口无平台类型**：签名只出现 int / string / bool / `Cursor` / error；某平台特有的能力不进 `WindowController`，按 winit 模式用扩展接口提供。
5. **坐标/单位跨平台一致**：逻辑像素、Y 向下、客户区（不含装饰）为尺寸/命中基准；物理像素只在 GPU 侧（×dpr）。

---

## 2. 接口定稿（四平台统一语义）

> 实现列图例：✅ = 已实现；🔨 = 实现中；⬜ = 占位未实现（语义已定）；⛔ = 平台协议/API 不可做（返回 ErrUnsupported）。

### 2.1 创建属性 `Options`

| 字段 | 类型 | 语义（四平台一致） | X11 | Wayland | Win32 | AppKit |
|---|---|---|---|---|---|---|
| Width/Height | int | 初始客户区（逻辑像素）；<1 取默认 640×480 | ✅ | ✅ | ⬜ | ⬜ |
| Title | string | 窗口标题；空默认 "gpui" | ✅ `_NET_WM_NAME` | ✅ `set_title` | ⬜ `SetWindowTextW` | ⬜ `setTitle:` |
| Backend | DisplayBackend | 显式选平台（Auto/X11/Wayland/**Win32/AppKit**）；Auto=探测（非 Linux 恒为该平台唯一后端）；Win32/AppKit 值用于显式请求对应后端——非目标 OS 或无实现时返回明确错误 | ✅ | ✅ | ⬜ | ⬜ |
| Decorations | *bool | nil=true 标准装饰；&false 无框裸窗 | ⚠️ WM 决定 | ✅ CSD | ⬜ WS_CAPTION 族 | ⬜ styleMask |
| Min/Max*Size | int | 尺寸约束；0=不限 | ✅ XSizeHints | ✅ set_min/max_size | ⬜ WM_GETMINMAXINFO | ⬜ contentMin/MaxSize |
| Position | *Point | 初始位置（客户区左上，屏幕坐标）；nil=系统决定 | ✅ | ⛔ | ⬜ SetWindowPos | ⬜ setFrameOrigin: |
| Fullscreen | bool | 初始全屏 | ✅ EWMH | ✅ 创建后 set_fullscreen | ⬜ 屏幕铺满 | ⬜ toggleFullScreen: |
| Cursor | Cursor | 初始光标（9 形状） | ✅ XCreateFontCursor | ✅ enter 时应用 wl_cursor_theme | ⬜ LoadCursor | ⬜ NSCursor |
| Resizable | bool | 用户可否调整尺寸；false=固定（min==max） | ✅ | ✅ min==max 锁 | ⬜ WS_THICKFRAME | ⬜ styleMask Resizable |
| **Maximized** | bool | 初始最大化（新） | ✅ EWMH 初始 | ✅ 首 commit 前 set_maximized | ⬜ SW_MAXIMIZE | ⬜ zoom: |
| **Visible** | *bool | 初始可见（nil=默认 true；&false=初始隐藏）（新） | ✅ map 控制 | ⛔ 协议无控制→忽略（恒可见） | ⬜ SW_SHOW/SW_HIDE | ⬜ orderFront:/orderOut: |

### 2.2 窗口门面 `Window`

| 方法 | 语义（四平台一致） |
|---|---|
| `Open(opts) (*Window, error)` | 按指定/探测后端创建原生窗口，Host 就绪 |
| `Adopt(ns NativeSurface) (*Window, error)` | 绑定既有原生句柄（嵌入场景） |
| `Host() Host` | 事件泵 / 尺寸 / scale（永不为 nil） |
| `Kind() PlatformKind` | 平台识别 |
| `IME() / Clipboard() / Controls()` | 可选能力，可能为 nil（静默降级） |
| `Close() / Closed()` | 请求销毁（幂等） |
| `WrapHost(host)` | 测试/嵌入宿主包装 |

### 2.3 窗口控制 SPI `WindowController`

| 组 | 方法 | 语义（四平台一致） | X11 | Wayland | Win32 | AppKit |
|---|---|---|---|---|---|---|
| 标题 | `Title()/SetTitle(t)` | 标题读写 | ✅ | ✅ | ⬜ SetWindowTextW | ⬜ title/setTitle: |
| 尺寸 | `Size()/SetSize(w,h)` | 客户区读写（逻辑像素） | ✅ 立即 | ✅ min==max 钳位 | ⬜ GetClientRect/SetWindowPos | ⬜ contentView.frame/ setContentSize: |
| 约束 | `SetMinSize/SetMaxSize` | 0=不限 | ✅ | ✅ | ⬜ WM_GETMINMAXINFO | ⬜ contentMin/MaxSize |
| Resizable | `SetResizable(r)/IsResizable()` | 运行时切换可调性 | ✅ hints 锁 | ✅ min==max 锁 | ⬜ WS_THICKFRAME | ⬜ styleMask |
| 装饰 | `SetDecorations(dec) error/IsDecorated()` | 运行时切换装饰 | ✅ _MOTIF_WM_HINTS | ⛔ 创建期 CSD 不可重建 | ⬜ WS_CAPTION 增删 | ⬜ setStyleMask: |
| 位置 | `Position()/SetPosition(x,y) error` | 客户区左上屏幕坐标（inner 语义） | ✅ | ⛔ | ⬜ GetWindowRect 换算/SetWindowPos | ⬜ frame 换算/setFrameOrigin: |
| 最小化 | `Minimize()/IsMinimized()` | 图标化 + 查询（Win32/AppKit 真查询；X11 乐观跟踪；**Wayland 协议无 minimized 状态 → 乐观跟踪**） | ✅ XIconifyWindow + 跟踪 | ✅ set_minimized + 乐观跟踪（activated 回置） | ⬜ ShowWindow(SW_MINIMIZE) | ⬜ miniaturize:/isMiniaturized |
| 最大化 | `Maximize()/Unmaximize()/IsMaximized()` | 状态机：请求→configure 回传 | ✅ EWMH | ✅ | ⬜ SW_MAXIMIZE/IsZoomed | ⬜ zoom:/isZoomed |
| 全屏 | `SetFullscreen(fs)/IsFullscreen()` | 同 | ✅ EWMH | ✅ | ⬜ | ⬜ toggleFullScreen: |
| 显隐 | `Show()/Hide()/IsVisible()` | 映射/取消映射 | ✅ | ⛔ | ⬜ SW_SHOW/HIDE | ⬜ orderFront:/orderOut: |
| 焦点 | `Focus() error/IsFocused()` | 请求焦点 + 查询 | ✅ | ⛔（查=activated） | ⬜ SetForegroundWindow/GetForegroundWindow | ⬜ makeKeyAndOrderFront:/isKeyWindow |
| 置顶 | `SetAlwaysOnTop(on) error` | 置顶切换 | ✅ EWMH ABOVE | ⛔ | ⬜ HWND_TOPMOST | ⬜ setLevel: NSFloatingWindowLevel |
| 光标 | `SetCursor(c Cursor)` | 形状切换（9 形状） | ✅ | ✅ | ⬜ | ⬜ |
| 穿透热区 | `SetIgnoreCursorEvents(ignore) error` | 指针穿透（覆盖层/点击穿透） | ⛔ XShape 未落地 | ✅ set_input_region | ⬜ WS_EX_TRANSPARENT | ⬜ ignoresMouseEvents |
| 拖拽启动 | `RequestMove() error` | 按当前指针启动窗口拖动（无框窗必需） | ✅ _NET_WM_MOVERESIZE | ✅ xdg move | ⬜ WM_NCLBUTTONDOWN HTCAPTION | ⬜ performWindowDragWithEvent: |
| resize 启动 | `RequestResize(edge) error` | 按当前指针启动边缘 resize（无框窗必需） | ✅ _NET_WM_MOVERESIZE | ✅ xdg resize | ⬜ WM_NCLBUTTONDOWN HT* | ⬜ performWindowDragWithEvent: |

> `WindowEdge` 枚举（RequestResize 用）：None/Top/Bottom/Left/Right/TopLeft/TopRight/BottomLeft/BottomRight（9 值，对应各平台 resize 方向）。

**平台能力边界（写死）**：Wayland 协议无客户端定位/显隐/焦点/置顶/装饰切换 → ErrUnsupported。上层调用方 nil-check + 降级。

### 2.4 事件 `Event`（Host.WaitEvents 产出 · 四平台统一）

| 事件 | 携带 | 语义（四平台一致） | X11 | Wayland | Win32 | AppKit |
|---|---|---|---|---|---|---|
| eventCloseRequested | — | 请求关闭（✕/WM_DELETE/xdg close）。可拦截：应用不调 Close() 则窗口存活 | ✅ WM_DELETE_WINDOW | 🔨 xdg close/CSD ✕（S5 上报） | ⬜ WM_CLOSE | ⬜ windowShouldClose: |
| EventClose | — | 窗口已销毁。诚实注：主动 Close() 后（尤其 Wayland）不再有事件；主要用于外部强制销毁通知 | ✅ DestroyNotify | ✅ surface 销毁 | ⬜ WM_DESTROY | ⬜ windowWillClose: |
| EventResize | Width/Height/Scale | 客户区尺寸 + dpr | ✅ ConfigureNotify | ✅ top configure | ⬜ WM_SIZE | ⬜ didResize |
| EventMove | MoveX/MoveY | 客户区左上屏幕坐标变化 | ✅ ConfigureNotify x/y | ⛔ 协议无 | ⬜ WM_MOVE | ⬜ didMoveNotification |
| EventScale | Scale | dpr 独立变化（多屏拖动/系统缩放） | ⛔ 暂未监听（RandR 待补） | 🔨 output（解析未上报，S5） | ⬜ WM_DPICHANGED | ⬜ viewDidChangeBackingProperties |
| EventOccluded | Occluded bool | 完全遮挡/最小化（停渲染省电） | ✅ VisibilityNotify | 🔨 suspended 解析未上报（S5） | ⬜ WM_SHOWWINDOW | ⬜ occlusionState |
| EventPointer（Move/Down/Up/Scroll/**Enter/Leave**） | PointerKind/X/Y/Button/Scroll* | 指针全事件（含 enter/leave hover 判定） | ✅ enter/leave 已上报 | 🔨 enter/leave 已记录未上报（S5） | ⬜ WM_MOUSEMOVE/ENTER/LEAVE | ⬜ mouseEntered:/Exited: |
| EventKey | KeyCode/Rune/Pressed | 键盘（IME 已消费跳过） | ✅ | ✅ | ⬜ WM_KEYDOWN/UP | ⬜ keyDown/Up |
| EventIME | IMEKind/IMEText/IMEStart/IMEEnd | 输入法（compose/commit/caret） | ✅ XIM | ✅ text-input v3 | ⬜ TSF | ⬜ NSTextInputClient |
| EventFocus | Focused bool | 键盘焦点变化 | ✅ FocusIn/Out | 🔨 activated 解析未上报（S5） | ⬜ WM_SETFOCUS/KILLFOCUS | ⬜ didBecomeKey/didResignKey |
| EventWake | — | 跨线程唤醒 | ✅ | ✅ | ⬜ | ⬜ |

**消费方兼容约定**：ui/embedder 与 ui/application 主循环把 EventCloseRequested 与 EventClose 同等对待（quit）；要拦截关闭的实现监听 EventCloseRequested 后自行决定。

### 2.5 跨平台语义契约（实现与消费方共同遵守）

1. **Position = 客户区左上角屏幕坐标**（inner，逻辑像素，Y 向下）。外框位置留给平台扩展接口，禁止各平台习惯漂移。
2. **SetSize × Resizable 交互**：`SetSize` 在 Wayland 走 min==max 钳位 → 调后窗口锁定不可手动调；解除 = `SetMinSize/SetMaxSize` 放开约束 或 `SetResizable(true)`。X11/Win32/AppKit 上 SetSize 立即生效且不锁定。禁止"只改记忆不锁窗"的假实现。
3. **查询语义**：后端不能回答的查询返回零值（Wayland `Position()` ok=false）；零值≠"隐藏/在原点"；状态变化感知一律走事件。例外：能回答的真实值不降级——Wayland xdg 一旦 mapped 恒可见，`IsVisible()`=true（真值，非零值）；`IsMaximized()/IsFullscreen()/IsFocused()` 以 configure states 回传为准（真值）。
4. **错误契约**：非 nil error 只有两种含义——协议/API 不支持（`ErrUnsupported`）或原生调用失败；签名无 error 的方法（SetTitle/SetCursor 等）为 best-effort，失败静默记录。
5. **线程契约**：Controller 方法可跨 goroutine 并发调用（X11 有 XInitThreads；Wayland 走 libwayland proxy 锁），backend 内各自加锁；状态快照不需要与事件流严格同步（异步状态机）。

### 2.6 平台验收矩阵（真窗 + 原生测试 + 上层抽象测试 · P6 硬规则）

每个平台的**原生实现**（x11_linux.go / wayland_linux.go / win32_windows.go / appkit_darwin.go）必须配套三层测试，缺一不可；三层各自独立验证，互相不能代替：

| 层 | 形态 | 位置 | 验证什么 | 何时绿 |
|---|---|---|---|---|
| A 真窗例程 | 独立命令，开原生窗口跑全能力驱动 + 事件泵观察 + JSON 门禁 | `examples/ui_pf_x11` · `ui_pf_wayland` · `ui_pf_win32` · `ui_pf_appkit` | 统一 API 在该平台能真实驱动原生窗口的每一项能力 | 该平台 S 阶段落地时（win32/appkit 占位期只验「明确未实现错误」，S5 后同一例程自动转全能力） |
| A′ 专项真窗例程 | 能力专项真窗（非全能力驱动）：IME 管线、输入法等 | `examples/ui_textinput_ime`（Wayland zwp_text_input_v3 全链路：platform.Event → InputRouter → Editor + 预编辑显示）· `examples/ui_ime_probe`（X11 XIM 探针） | 专项能力的端到端真实行为（IME compose/commit/caret 只可能在真窗+真输入法下验证） | 与对应能力 S 阶段同生；无输入法/无法交互时人工验收（GUI 演示，不出 JSON 门禁） |
| B 原生单测 | 平台包内 build-tag 测试，直接断言原生行为（真假/去重/错误码） | `ui/platform/x11_window_linux_test.go` · `wayland_window_linux_test.go`（win32/appkit 随后端落地同步落，形态对齐 x11） | 每能力断言 + 事件流断言（EventCloseRequested≠EventClose、Focus/Occluded/Resize 到达等） | 有显示环境则必跑；无环境 `t.Skipf` 带原因（禁止静默假绿） |
| C 上层抽象测试 | 无 GPU 窗口：fake controller + `StubHost` 驱动同一共享驱动 | `examples/pfkit/pfkit_test.go` | 驱动确定性——调用序列/计数固定、ErrUnsupported 容忍为 ⛔、其他错误标 FAIL、Report 计数 | 恒绿（CI 无显示也跑） |

**规则（硬）**：
1. **共享驱动唯一**：四平台真窗例程与上层测试跑 `examples/pfkit` 的**同一份** `Drive()`/`Pump()` 驱动序列（平台无关，零平台代码）——C 绿 ≠ 平台绿（驱动没坏不代表后端对），A 绿 ≠ C 绿（后端对不代表驱动确定），两层分开断言。
2. **例程与平台同生**：某平台实现落地（S2 Wayland / S5 Win32+AppKit）的验收标准 = 对应 `ui_pf_*` 从占位态转全能力 PASS；禁止「实现先落地、真窗例程迟迟不建」。
3. **占位期诚实**：win32/appkit 占位期（Create 返回明确未实现错误）其真窗例程验的就是「错误明确、非 panic、不假 PASS」——S5 落地后同一文件自动升级为全能力验收，无需改例程代码。
4. **JSON 门禁**：真窗例程 stdout 统一输出 `{"backend":…,"rows":[…],"pass":…}` 供门禁工具消费；stderr 打人类可读表格。
5. **回归纪律**：真窗例程与平台的测试按文件跑（AGENTS.md），禁止一次全量。
6. **验证范围（P6 验收只验本需求）**：三层各自独立的验收命令——

   ```text
   C 层：go test ./examples/pfkit/ -count=1          # 6 例，恒绿（CI 无显示也跑）
   B 层：go test ./ui/platform/ -run '^(TestX11RealWindow|TestWaylandRealWindow)' -count=1
          # 前缀匹配（禁止加 $ 锚点，否则 0 测试假绿）；真窗；无 DISPLAY/WAYLAND_DISPLAY
          # → t.Skipf 带原因；WM 桌面（GNOME 等）→ Hide/UnmapNotify 断言诚实 Skip
          # （Xvfb/裸 X 严格断言）
A 层：go run ./examples/ui_pf_x11      # X11 全能力真窗（本环境 DISPLAY=:1 19 项 PASS）
          go run ./examples/ui_pf_wayland  # Wayland 全能力真窗（wlController S2 落地后 19 项 PASS）
         go run ./examples/ui_pf_win32    # 非 Windows = SKIP；Windows 上 S5 前 = placeholder
         go run ./examples/ui_pf_appkit   # 非 macOS = SKIP；macOS 上 S5 前 = placeholder
   A′ 层：export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so && \
           GPUI_DISPLAY=wayland go run ./examples/ui_textinput_ime   # IME 真窗（stage=ime-enabled）
           GPUI_DISPLAY=x11     go run ./examples/ui_ime_probe      # XIM 探针
   ```
   A′ 层为 GUI 交互式（人打字/Ibus 验证），无 JSON 门禁；启动到 `stage=ime-enabled` 即接线完整。

   P6 相关改动只跑这三层（平台其余测试/渲染层不在范围内）；完整平台回归（ui/platform 全部文件）留到各 S 阶段验收按文件跑。

---

## 3. 状态与实现（现状 · 2026-08-12）

| 阶段 | 内容 | 状态 |
|---|---|---|
| S0 接口定稿 | 四平台统一语义（v2.0 本文件） | ✅ |
| S1 X11 | x11Controller 全方法 + 事件上报（Move/CloseRequested/Occluded/Enter/Leave）+ Options 全量 + 真窗例程 `ui_pf_x11` + 原生单测 `x11_window_linux_test.go` | ✅ |
| S2 Wayland | wlController（xdg marshal/光标/热区/RequestMove/RequestResize）+ Options 全量 + 状态字段 + 事件上报（EventCloseRequested/EventFocus/EventOccluded/PointerEnter/Leave）——按 `ENGINE_WAYLAND_WINDOW_STANDARD.md` §2.4/§3.1/§3.2 执行；同步落真窗例程 `ui_pf_wayland` 全能力 + 原生单测 `wayland_window_linux_test.go`（§2.6 三层） | ✅ |
| S3 消费方兼容 + 测试 | embedder/application 兼容 EventCloseRequested；上层抽象测试 `pfkit_test.go`（C 层）落定；单测；回归按文件 | ✅ |
| S4 真窗验收 | 用户跑 examples：X11 + Wayland 全能力（§2.6 A 层）；真实显示环境跑通——`ui_pf_x11` 19 项全 PASS + `ui_pf_wayland` 19 项全 PASS（含 ⛔ 诚实降级），B 层真窗单测同机全 PASS | ✅ |
| S5 Win32/AppKit | 按 §2.3/§2.4 语义列落地（占位→实现）；同步落 `ui_pf_win32`/`ui_pf_appkit` 真窗例程 + 原生单测（§2.6 三层）；重跑能力矩阵 | ⬜ |

**落地纪律**：S1–S3 逐行对照 §2.3/§2.4 实现，不跳步；每阶段跑对应单测 + 回归（按文件，禁止一次全量）+ §2.6 三层验收同步落地（例程与平台同生）；S5 落地时四条语义契约（§2.5）逐条核对。

---

## 4. 修订

- v2.6（2026-08-13）：**S4 真窗验收完成**——A 层在真实显示环境跑通：`ui_pf_x11` 19 项全 PASS（DISPLAY=:1，IgnoreCursorEvents 按 XShape 未落地 ⛔ 诚实降级）；`ui_pf_wayland` 19 项全 PASS（Position/Show/Hide/Focus/AlwaysOnTop/Decorations 切换/RequestMove/RequestResize 按协议 ⛔ 诚实降级，其余真 PASS）；B 层 `TestX11RealWindow*` 7 例 + `TestWaylandRealWindow*` 3 例同机全 PASS（真窗非 Skip）；C 层 pfkit 6 例恒绿。S3 承上（v2.5）。
- v2.5（2026-08-12）：**S3 消费方兼容落定**——§2.4 消费方兼容约定落地：新增 `embedder.EventQuits(ev)`（EventCloseRequested 与 EventClose 同等退出主循环，单一判定点），embedder `app.go` 与 `pipeline_app.go` 两处主循环、application 主窗 quit（`embedder.EventQuits(ev) && w.main`）统一调用；`ui/input.FromPlatform` 把 EventCloseRequested 归为 KindClose；单测 `TestEventQuits`（embedder）+ fromplatform 生命周期加 EventCloseRequested→KindClose 断言；回归按文件全绿。
- v2.4（2026-08-12）：**S2 Wayland wlController 落地**——§2.3/§2.1 Wayland 列的 🔨 全部转 ✅：Min/Max/Fullscreen/Cursor/Resizable/Maximized Options 接线（waylandCreate 全量）、Minimize+IsMinimized 乐观跟踪（configure activated 回置）、RequestMove/RequestResize（xdg move/resize，无合法 enter serial 时诚实 ErrUnsupported）、SetIgnoreCursorEvents（wl_region 空 region/NULL 恢复）、SetCursor（wl_cursor_theme 名映射 + enter/motion 持续应用）；configure states 回传对抗乐观：IsMaximized/IsFullscreen/IsFocused 为真值。**§2.5.3 契约修正**：Wayland `IsVisible()` 改为恒 true（xdg 无隐藏态，真值而非零值，与标准文档 §3.2 对齐）；§2.6 A 层 ui_pf_wayland 19 项全 PASS、B 层 TestWaylandRealWindowControls 落地。
- v2.3（2026-08-12）：**P6 三层验收硬规则**——新增 §2.6 平台验收矩阵：每平台原生实现必须配套 A 真窗例程（`examples/ui_pf_x11/wayland/win32/appkit`，共享驱动）+ A′ 专项真窗例程（IME 专项：`ui_textinput_ime` Wayland zwp_text_input_v3 全链路 / `ui_ime_probe` X11 XIM，GUI 交互式无门禁）+ B 原生单测（build tag，无显示 `t.Skipf` 带原因）+ C 上层抽象测试（`pfkit_test.go`，fake controller + StubHost 恒绿）；§3 各 S 行把三层落地写成阶段验收标准（例程与平台同生，占位期验「明确未实现错误」）。
- v2.2（2026-08-12）：**Wayland 列诚实化**——审查发现 S1 前 Wayland 若干格假绿：EventCloseRequested/EventFocus/EventOccluded/EventScale 的 Wayland 列改回 🔨（代码仅解析未上报）；IsMinimized 明确「协议无 minimized 状态 → 乐观跟踪」；Options 的 Min/Max/Fullscreen/Cursor/Resizable/Maximized 改 🔨（waylandCreate 仅接 4 参）。联动 `ENGINE_WAYLAND_WINDOW_STANDARD.md` v1.1（S5 对齐）。
- v2.1（2026-08-12）：**S1 X11 controller 落地**——接口与 v2.0 完全对齐：Options.Maximized/Visible、IsMinimized、RequestMove/RequestResize（WindowEdge）+ _NET_WM_MOVERESIZE、EventOccluded（VisibilityNotify）、PointerEnter/Leave（Enter/LeaveNotify）、EventMove（ConfigureNotify 位置去重）；二维事件掩码补 VisibilityChange/EnterWindow/LeaveWindow；Create/Adopt 均接线 x11Controller；Create 全量应用 Options（Position/Fullscreen/Maximized/Cursor/Decorations/Visible/Resizable/min-max）。
- v2.0（2026-08-12）：**口径重写为四平台统一语义**——接口形状由四平台能共同表达的最高语义决定（P5）；删除旧版 Linux 主导表述与冗余章节；Options 增 Maximized/Visible；Controller 增 IsMinimized/RequestMove/RequestResize（WindowEdge）；事件增 EventScale/EventOccluded/PointerEnter/Leave；§2.3/§2.4 每行都给四平台实现列（Win32/AppKit 为占位语义，落地时不改接口）。
- v1.3（2026-08-12）：暴露面统一性审查——三张对齐表（属性 A1–A15 / 方法 M1–M21 / 事件 E1–E19，对照 winit/GTK4/gpui/Flutter），结论主干齐全、待补 8 项。
- v1.2：跨平台审查——§1.1 分层硬规则、§2.3 五条跨平台语义契约、EventClose 诚实注、Win32/AppKit 语义映射锚点。
- v1.1：P0 五项入接口（EventMove、Resizable/Decorations 切换、Close 拆分、穿透热区）。
- v1.0：初版统一窗口接口 + 能力矩阵 + 缺失分析。