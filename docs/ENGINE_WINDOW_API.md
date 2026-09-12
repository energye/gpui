# 平台窗口统一 API — 设计真源

> **版本：v2.17** | 日期：2026-09-12  
> **地位：** `ui/platform` 窗口接口（Options / Window / Host / WindowController / Event）唯一设计真源，**口径：四平台统一语义**。
> **并读：** [`ENGINE_WAYLAND_WINDOW_STANDARD.md`](./ENGINE_WAYLAND_WINDOW_STANDARD.md)（Wayland 标准窗口 CSD）· [`ENGINE_ARCH_OVERVIEW.md`](./ENGINE_ARCH_OVERVIEW.md)（分层）· [`ENGINE_TEXT_WAYLAND_IME_REQUIREMENT.md`](./ENGINE_TEXT_WAYLAND_IME_REQUIREMENT.md) / [`ENGINE_TEXT_X11_IME_REQUIREMENT.md`](./ENGINE_TEXT_X11_IME_REQUIREMENT.md)（输入法需求；原 `ENGINE_INPUT_IME_PLAN.md` 不存在，入口改指这两份）  
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
| Backend | DisplayBackend | 显式选平台（Auto/X11/Wayland/**Win32/AppKit**）；Auto=探测（`WAYLAND_DISPLAY` 有则优先原生 Wayland，X11 只是 XWayland 有 resize/present 限制；仅 `DISPLAY` 才 X11；两空回 Auto；非 Linux 恒为该平台唯一后端）；Win32/AppKit 值用于显式请求对应后端——非目标 OS 或无实现时返回明确错误 | ✅ | ✅ | ⬜ | ⬜ |
| IconName | string | 应用图标名（任务栏图标匹配）；空=后端默认 | ⛔ 未接线（空=默认） | ✅ set_app_id | ⬜ | ⬜ |
| Decorations | bool | **true=标准装饰；false/缺省=无框裸窗**（手动开启） | ⚠️ WM 决定 | ✅ CSD | ⬜ WS_CAPTION 族 | ⬜ styleMask |
| Min/Max*Size | int | 尺寸约束；0=不限 | ✅ XSizeHints | ✅ set_min/max_size | ⬜ WM_GETMINMAXINFO | ⬜ contentMin/MaxSize |
| Position | *Point | 初始位置（客户区左上，屏幕坐标）；nil=系统决定 | ✅ | ⛔ | ⬜ SetWindowPos | ⬜ setFrameOrigin: |
| Fullscreen | bool | 初始全屏 | ✅ EWMH | ✅ 创建后 set_fullscreen | ⬜ 屏幕铺满 | ⬜ toggleFullScreen: |
| Cursor | Cursor | 初始光标（9 形状） | ✅ XCreateFontCursor | ✅ 首选 cursor-shape 协议（合成器自绘），无才回 wl_cursor_theme | ⬜ LoadCursor | ⬜ NSCursor |
| Resizable | bool | 用户可否调整尺寸；false=固定（min==max） | ✅ | ✅ min==max 锁 | ⬜ WS_THICKFRAME | ⬜ styleMask Resizable |
| **Maximized** | bool | 初始最大化（新） | ✅ EWMH 初始 | ✅ 首 commit 前 set_maximized | ⬜ SW_MAXIMIZE | ⬜ zoom: |
| **Visible** | *bool | 初始可见（nil=默认 true；&false=初始隐藏）（新） | ✅ map 控制 | ✅ destroy/重建栈 + EventHidden（原⛔已过期） | ⬜ SW_SHOW/SW_HIDE | ⬜ orderFront:/orderOut: |

### 2.2 窗口门面 `Window`

| 方法 | 语义（四平台一致） |
|---|---|
| `Open(opts) (*Window, error)` | 按指定/探测后端创建原生窗口，Host 就绪 |
| `Adopt(ns NativeSurface) (*Window, error)` | 绑定既有原生句柄（嵌入场景） |
| `Host() Host` | 事件泵 / 尺寸 / scale（永不为 nil） |
| `Kind() PlatformKind` | 平台识别 |
| `Backend() DisplayBackend` | 后端识别（窗口打开上报用，与 Kind 互补） |
| `IME() / Clipboard() / Controls()` | 可选能力，可能为 nil（静默降级）；`Adopt` 无显示给内部剪贴板，`Create` 才给真剪贴板；Wayland 无 data-device 时 Clipboard 为 nil |
| `Close() / Closed()` | 请求销毁（幂等） |
| `WrapHost(host)` | 测试/嵌入宿主包装 |

#### 2.2.1 IME / Clipboard 能力现状（代码有、原先漏记）

- 方向：`IME` 六方法（EnableIME/UpdateCursorRect/SetContentType/SetComposing/Commit/DisableIME）是应用推给输入法；`EventIME` 是输入法推回应用。`SetComposing` 实为推 surrounding+光标（不翻 composing 旗，旗只靠信号），密码目的跳过上报，surrounding 超 4000 字节居中截断（两端同口径）。
- 锚点：`Rect` 逻辑像素窗口相对；`ContentPurpose` 14 值（密码掩码+不预测等关键语义）；`FieldSnapshot` 是 Enable 快照包；X11 每改必推 surrounding，Wayland 按需取回。
- 剪贴板：`Clipboard.Get/Set(kind, data)`（如 text/plain）；X11 走 ICCCM（500ms 超时/INCR 大块/空与超时回落内部剪贴板）；Wayland 自读短路 + 同源缓存，offer/source 销毁 opcode 不可混。剪贴板与占位后端的做不到走 `fmt.Errorf` 明错（非 `ErrUnsupported`，见 §2.5）。
- 富预编辑：ibus/fcitx 下划线选中态只解析不转发（等分段感知事件），当前只留纯文本，非 bug。

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
| 显隐 | `Show()/Hide()/IsVisible()` | 映射/取消映射（Wayland 真实现：destroy/重建栈 + EventHidden） | ✅ | ✅ | ⬜ SW_SHOW/HIDE | ⬜ orderFront:/orderOut: |
| 焦点 | `Focus() error/IsFocused()` | 请求焦点 + 查询 | ✅ | ⛔（查=activated） | ⬜ SetForegroundWindow/GetForegroundWindow | ⬜ makeKeyAndOrderFront:/isKeyWindow |
| 置顶 | `SetAlwaysOnTop(on) error` | 置顶切换 | ✅ EWMH ABOVE | ⛔ | ⬜ HWND_TOPMOST | ⬜ setLevel: NSFloatingWindowLevel |
| 光标 | `SetCursor(c Cursor)` | 形状切换（9 形状） | ✅ | ✅ | ⬜ | ⬜ |
| 穿透热区 | `SetIgnoreCursorEvents(ignore) error` | 指针穿透（覆盖层/点击穿透） | ⛔ XShape 未落地 | ✅ set_input_region | ⬜ WS_EX_TRANSPARENT | ⬜ ignoresMouseEvents |
| 拖拽启动 | `RequestMove() error` | 按当前指针启动窗口拖动（无框窗必需；Wayland 无合法 enter serial 时诚实 ErrUnsupported） | ✅ _NET_WM_MOVERESIZE | ✅ xdg move | ⬜ WM_NCLBUTTONDOWN HTCAPTION | ⬜ performWindowDragWithEvent: |
| resize 启动 | `RequestResize(edge) error` | 按当前指针启动边缘 resize（无框窗必需；Wayland edge=None 或无 serial 时诚实 ErrUnsupported） | ✅ _NET_WM_MOVERESIZE | ✅ xdg resize | ⬜ WM_NCLBUTTONDOWN HT* | ⬜ performWindowDragWithEvent: |

> `WindowEdge` 枚举（RequestResize 用）：None/Top/Bottom/Left/Right/TopLeft/TopRight/BottomLeft/BottomRight（9 值，对应各平台 resize 方向）。

**平台能力边界（写死）**：Wayland 协议无客户端定位/焦点/置顶/装饰切换 → ErrUnsupported（显隐已真实现，不在此列）。上层调用方 nil-check + 降级。

### 2.4 事件 `Event`（Host.WaitEvents 产出 · 四平台统一）

| 事件 | 携带 | 语义（四平台一致） | X11 | Wayland | Win32 | AppKit |
|---|---|---|---|---|---|---|
| eventCloseRequested | — | 请求关闭（✕/WM_DELETE/xdg close）。可拦截：应用不调 Close() 则窗口存活 | ✅ WM_DELETE 拆分 CloseRequested/Close | ✅ xdg close/CSD ✕ | ⬜ WM_CLOSE | ⬜ windowShouldClose: |
| EventClose | — | 窗口已销毁。诚实注：主动 Close() 后（尤其 Wayland）不再有事件；主要用于外部强制销毁通知 | ✅ DestroyNotify | ✅ surface 销毁 | ⬜ WM_DESTROY | ⬜ windowWillClose: |
| EventResize | Width/Height/Scale | 客户区尺寸 + dpr | ✅ ConfigureNotify | ✅ top configure | ⬜ WM_SIZE | ⬜ didResize |
| EventMove | MoveX/MoveY | 客户区左上屏幕坐标变化 | ✅ ConfigureNotify x/y | ⛔ 协议无 | ⬜ WM_MOVE | ⬜ didMoveNotification |
| EventScale | Scale | dpr 独立变化（多屏拖动/系统缩放） | ✅ RandR 通知 + Xft.dpi 推导 | ✅ output 监听取 max | ⬜ WM_DPICHANGED | ⬜ viewDidChangeBackingProperties |
| EventOccluded | Occluded bool | 完全遮挡/最小化（停渲染省电） | ✅ VisibilityNotify | ✅ suspended 上报 | ⬜ WM_SHOWWINDOW | ⬜ occlusionState |
| EventPointer（Move/Down/Up/Scroll/**Enter/Leave**） | PointerKind/X/Y/Button/Scroll* | 指针全事件（含 enter/leave hover 判定；X11 6/7 号横滚键转 ScrollX） | ✅ enter/leave 带坐标 | ✅ enter/leave；Leave 补最近坐标 | ⬜ WM_MOUSEMOVE/ENTER/LEAVE | ⬜ mouseEntered:/Exited: |
| EventKey | KeyCode/Rune/Pressed/Repeat | 键盘（IME 已消费跳过；X11 XKB 连发/Wayland 客户端合成连发置 Repeat，异步 IME 保序） | ✅ | ✅ | ⬜ WM_KEYDOWN/UP | ⬜ keyDown/Up |
| EventIME | IMEKind/IMEText/IMEStart/IMEEnd | 输入法（compose/commit/caret/delete-surrounding；X11 走 D-Bus ibus/fcitx5，XIM 已移除；两端暂缺 caret；Wayland text-input 批量原子提交 + 同 rect 去重） | ✅ D-Bus | ✅ text-input v3 | ⬜ TSF | ⬜ NSTextInputClient |
| EventTouch | ID(≥1)/X/Y + 相位(Down/Move/Up/Cancel) | 多点触控槽采样 | ✅ XI2 探测 + 解析 + 门控 | ✅ wl_touch 绑定 + 解码 | ⬜ | ⬜ |
| EventFocus | Focused bool | 键盘焦点变化 | ✅ FocusIn/Out；IsFocused 真值回写 | ✅ activated 上报 | ⬜ WM_SETFOCUS/KILLFOCUS | ⬜ didBecomeKey/didResignKey |
| EventStateChanged | Minimized/Maximized/Fullscreen bool | 最小化/最大化/全屏三元组变化（非原生事件，由回传推导） | ✅ _NET_WM_STATE/WM_STATE 回读 | ✅ configure 推导 | ⬜ | ⬜ |
| EventWake | — | 跨线程唤醒 | ✅ | ✅ | ⬜ | ⬜ |
| EventExpose | — | 重绘提示。GPU 拥有像素，不进上层输入，X11 在事件泵过滤 | ✅ 发射后过滤 | ⛔ 无对应（frame 回调覆盖） | ⬜ WM_PAINT | ⬜ drawRect |
| EventResizeSync | — | 窗口系统请求同步帧（X11 _NET_WM_SYNC_REQUEST；排帧后推进 counter，否则拖动时只拉伸旧画面） | ✅ 请求解析 + NotifyFrameDrawn 推进 | ⛔ | ⛔ | ⛔ |
| EventDrop | Files/X/Y | 外部文件落入窗口（位置 + 路径；MIME 数据走 S6-P1；空拖放静默吞掉） | ❌ 无 XDND 接线（S6-P1） | ✅ text/uri-list | ⬜ | ⬜ |
| EventHidden | Hidden bool | 应用显隐（Wayland destroy/重建栈，GPU 先停） | ✅ 显隐发射 + 外部重映射通知 | ✅ 双向 + 真窗断言 | ⬜ | ⬜ |
| EventFramePresented | — | 合成器已显示一帧（帧 pacing 输入；通知与 DRM 二选一，有通知不用 DRM；Wayland 现恒不用通知、回 DRM 学周期） | ✅ XPresent（不可用回落 DRM） | ✅ frame 回调 | ⬜ | ⬜ |

**消费方兼容约定**：ui/embedder 与 ui/application 主循环把 EventCloseRequested 与 EventClose 同等对待（quit）；要拦截关闭的实现监听 EventCloseRequested 后自行决定。

### 2.5 跨平台语义契约（实现与消费方共同遵守）

1. **Position = 客户区左上角屏幕坐标**（inner，逻辑像素，Y 向下）。外框位置留给平台扩展接口，禁止各平台习惯漂移。
2. **SetSize × Resizable 交互**：`SetSize` 在 Wayland 走 min==max 钳位 → 调后窗口锁定不可手动调；解除 = `SetMinSize/SetMaxSize` 放开约束 或 `SetResizable(true)`。X11/Win32/AppKit 上 SetSize 立即生效且不锁定。禁止"只改记忆不锁窗"的假实现。
3. **查询语义**：后端不能回答的查询返回零值（Wayland `Position()` ok=false）；零值≠"隐藏/在原点"；状态变化感知一律走事件。例外：能回答的真实值不降级——Wayland xdg 一旦 mapped 恒可见，`IsVisible()`=true（真值，非零值）；`IsMaximized()/IsFullscreen()/IsFocused()` 以 configure states 回传为准（真值）。
4. **错误契约**：非 nil error 只有两种含义——协议/API 不支持（`ErrUnsupported`）或原生调用失败；签名无 error 的方法（SetTitle/SetCursor 等）为 best-effort，失败静默记录。例外：剪贴板读写与 Win32/AppKit 占位路径用 `fmt.Errorf` 明错（非 `ErrUnsupported`），调用方按错误串识别，P4 的 ErrUnsupported 只管窗口控制 SPI。
5. **线程契约**：Controller 方法可跨 goroutine 并发调用（X11 有 XInitThreads；Wayland 走 libwayland proxy 锁），backend 内各自加锁；状态快照不需要与事件流严格同步（异步状态机）。

### 2.6 平台验收矩阵（真窗 + 原生测试 + 上层抽象测试 · P6 硬规则）

每个平台的**原生实现**（x11_linux.go / wayland_linux.go / win32_windows.go / appkit_darwin.go）必须配套三层测试，缺一不可；三层各自独立验证，互相不能代替：

| 层 | 形态 | 位置 | 验证什么 | 何时绿 |
|---|---|---|---|---|
| A 真窗例程 | 独立命令，开原生窗口跑全能力驱动 + 事件泵观察 + JSON 门禁 | `examples/ui_pf_x11` · `ui_pf_wayland` · `ui_pf_win32` · `ui_pf_appkit` | 统一 API 在该平台能真实驱动原生窗口的每一项能力 | 该平台 S 阶段落地时（win32/appkit 占位期只验「明确未实现错误」，S5 后同一例程自动转全能力） |
| A′ 专项真窗例程 | 能力专项真窗（非全能力驱动）：IME 管线、输入法等 | `examples/ui_textinput_ime`（Wayland zwp_text_input_v3 全链路：platform.Event → InputRouter → Editor + 预编辑显示，自动探测后端）· `examples/ui_ime_probe`（D-Bus/X11 + text-input-v3 双栈探针，XIM 已移除） | 专项能力的端到端真实行为（IME compose/commit 只可能在真窗+真输入法下验证） | 与对应能力 S 阶段同生；无输入法/无法交互时人工验收（GUI 演示，不出 JSON 门禁） |
| B 原生单测 | 平台包内 build-tag 测试，直接断言原生行为（真假/去重/错误码） | `ui/platform/x11_window_linux_test.go` · `x11_sync_test.go`（3 例同步帧） · `wayland_window_linux_test.go`（win32/appkit 随后端落地同步落，形态对齐 x11） | 每能力断言 + 事件流断言（EventCloseRequested≠EventClose、Focus/Occluded/Resize 到达等） | 有显示环境则必跑；无环境 `t.Skipf` 带原因（禁止静默假绿） |
| C 上层抽象测试 | 无 GPU 窗口：fake controller + `StubHost` 驱动同一共享驱动 | `examples/pfkit/pfkit_test.go` | 驱动确定性——调用序列/计数固定、ErrUnsupported 容忍为 ⛔、其他错误标 FAIL、Report 计数 | 恒绿（CI 无显示也跑） |

**规则（硬）**：
1. **共享驱动唯一**：四平台真窗例程与上层测试跑 `examples/pfkit` 的**同一份** `Drive()`/`Pump()` 驱动序列（平台无关，零平台代码）——C 绿 ≠ 平台绿（驱动没坏不代表后端对），A 绿 ≠ C 绿（后端对不代表驱动确定），两层分开断言。
2. **例程与平台同生**：某平台实现落地（S2 Wayland / S5 Win32+AppKit）的验收标准 = 对应 `ui_pf_*` 从占位态转全能力 PASS；禁止「实现先落地、真窗例程迟迟不建」。
3. **占位期诚实**：win32/appkit 占位期（Create 返回明确未实现错误）其真窗例程验的就是「错误明确、非 panic、不假 PASS」——S5 落地后同一文件自动升级为全能力验收，无需改例程代码。
4. **JSON 门禁**：真窗例程 stdout 统一输出 `{"backend":…,"rows":[…],"pass":…}` 供门禁工具消费（win32/appkit 经 `finish()` 多带 `mode` 字段，门禁只读三键）；stderr 打人类可读表格。
5. **回归纪律**：真窗例程与平台的测试按文件跑（AGENTS.md），禁止一次全量。
6. **验证范围（P6 验收只验本需求）**：三层各自独立的验收命令——

```text
   C 层：go test ./examples/pfkit/ -count=1          # 6 例，恒绿（CI 无显示也跑）
   B 层：go test ./ui/platform/ -run '^(TestX11RealWindow|TestWaylandRealWindow|TestWaylandHideShow)' -count=1
          # 前缀匹配（禁止加 $ 锚点，否则 0 测试假绿）；真窗；无 DISPLAY/WAYLAND_DISPLAY
          # → t.Skipf 带原因；WM 桌面（GNOME 等）→ Hide/UnmapNotify 断言诚实 Skip
          # （Xvfb/裸 X 严格断言）
A 层：go run ./examples/ui_pf_x11      # X11 全能力真窗（本环境 DISPLAY=:1 19 行 PASS = 18 能力行 + 1 事件观察行，零事件也过）
          go run ./examples/ui_pf_wayland  # Wayland 全能力真窗（wlController S2 落地后 19 行 PASS，构成同上）
         go run ./examples/ui_pf_win32    # 非 Windows = SKIP；Windows 上 S5 前 = placeholder
         go run ./examples/ui_pf_appkit   # 非 macOS = SKIP；macOS 上 S5 前 = placeholder
   A′ 层：export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so && \
           go run ./examples/ui_textinput_ime   # IME 真窗（自动探测后端，Router+Editor 接线）
           go run ./examples/ui_ime_probe       # 双栈探针（D-Bus/X11 + text-input-v3，XIM 已移除）
```
   A′ 层为 GUI 交互式（人打字/Ibus 验证），无 JSON 门禁；能起窗打字即接线完整。

   P6 相关改动只跑这三层（平台其余测试/渲染层不在范围内）；完整平台回归（ui/platform 全部文件）留到各 S 阶段验收按文件跑。

---

## 3. 状态与实现（现状 · 2026-08-12）

| 阶段 | 内容 | 状态 |
|---|---|---|
| S0 接口定稿 | 四平台统一语义（v2.0 本文件） | ✅ |
| S1 X11 | x11Controller 全方法 + 事件上报（Move/Close/Occluded/Enter/Leave；CloseRequested 已从 Close 拆分）+ Options 全量 + 真窗例程 `ui_pf_x11` + 原生单测 `x11_window_linux_test.go` + `x11_sync_test.go`（同步帧 3 例） | ✅ |
| S2 Wayland | wlController（xdg marshal/光标/热区/RequestMove/RequestResize）+ Options 全量 + 状态字段 + 事件上报（EventCloseRequested/EventFocus/EventOccluded/PointerEnter/Leave）——按 `ENGINE_WAYLAND_WINDOW_STANDARD.md` §2.4/§3.1/§3.2 执行；同步落真窗例程 `ui_pf_wayland` 全能力 + 原生单测 `wayland_window_linux_test.go`（§2.6 三层） | ✅ |
| S3 消费方兼容 + 测试 | embedder/application 兼容 EventCloseRequested；上层抽象测试 `pfkit_test.go`（C 层）落定；单测；回归按文件 | ✅ |
| S4 真窗验收 | 用户跑 examples：X11 + Wayland 全能力（§2.6 A 层）；真实显示环境跑通——`ui_pf_x11` 19 行全 PASS（18 能力行 + 1 事件观察行，IgnoreCursorEvents 按 XShape 未落地 ⛔ 诚实降级） + `ui_pf_wayland` 19 行全 PASS（Position/Focus/AlwaysOnTop/Decorations 切换/RequestMove/RequestResize 按协议 ⛔ 诚实降级，Show/Hide 与 IgnoreCursorEvents 走真实现），B 层 `TestX11RealWindow*` 7 例 + `TestWaylandRealWindow*` 3 例 + `TestWaylandHideShow` 同机全 PASS | ✅ |
| S5 Win32/AppKit | 按 §2.3/§2.4 语义列落地（占位→实现）；同步落 `ui_pf_win32`/`ui_pf_appkit` 真窗例程 + 原生单测（§2.6 三层）；重跑能力矩阵 | ⬜ |
| S6-P0 上层统一必做 | §4.4 A–C + D 触控基础：新 Kind/载荷、Close 拆分、Enter/Leave/Cancel 独立、Repeat、横滚、触控产出、Wayland 三上报；消费方切换；§2.6 三层真窗（`ui_pf_x11`/`ui_pf_wayland` 重跑 + `pfkit` 恒绿） | ✅（Move/Scale/Focus/Drop/ResizeSync 补 FromPlatform 分支并进主循环路由；Touch/StateChanged 进主循环路由；Resize/ResizeSync/Occluded/Hidden/FramePresented 同步透传路由器观察） |
| S6-P1 顺手做 | §4.4 拖放四件套/触控板手势/主题/语言/设备插拔；专项真窗（拖放/Gesture 人工 + JSON 门禁能自动的自动） | ⬜ |
| S6-P2 占位 | 笔压感/显示器增减/智能放大接口占位 + Win32/AppKit 映射表；真窗验占位错误明确 | ⬜ |

**落地纪律**：S1–S3 逐行对照 §2.3/§2.4 实现，不跳步；每阶段跑对应单测 + 回归（按文件，禁止一次全量）+ §2.6 三层验收同步落地（例程与平台同生）；S5 落地时四条语义契约（§2.5）逐条核对。

---

## 4. 上层事件统一需求（S6 · 本次立项）

> 目标：§2.4 的窗口一堆事件 + 输入侧残缺口，对齐 winit / GTK4 / Flutter / W3C UI Events 语义，统一进上层 `ui/input` 事件，上层只认统一事件。

### 4.1 现状（新会话入口）

- 统一输入已落地：`ui/input/event.go`（Kind：Close/Resize/Key/Pointer/Touch/Scroll/Text/IME/Wake）+ `ui/input/fromplatform.go`（Close/Resize/Wake/Pointer/Key/IME 已转）+ `ui/embedder/input_router.go`（分发指针/按键/文本/IME）。
- 未进上层：Move / Scale / Occluded / Focus / Drop / Hidden / FramePresented / ResizeSync 其中 Occluded/Hidden/FramePresented/ResizeSync 由 `ui/embedder/pipeline_app.go` 直接读 `platform.Event`（见 EventOccluded / EventHidden 分支）；Move/Scale/Focus/Drop 无消费分支、直接穿过被忽略。
- 语义丢失点：`fromplatform.go:toPointerKind` 把 Enter/Leave 归并为 Move（Cancel 在平台层无来源）；X11 的 WM_DELETE 进 `EventClose` 与销毁通知坍缩；CloseRequested 与 Close 都归为 KindClose；触控只有 `FromTouch` 空接口，后端无产出；X11 横滚（6/7 号键）按普通按键上报未转 ScrollX。

### 4.2 范围与分期（口径 P5：接口按四平台最高语义一次定全，实现分期）

- P0（本次必做）：窗口 Move/Scale/Occluded/Focus/Hidden/FramePresented/ResizeSync/StateChanged 推导进上层；CloseRequested≠Close 拆分；Enter/Leave/Cancel 独立；按键 Repeat 标记；X11 横滚；触控后端产出；Wayland scale 上报（suspended/activated 已有）。
- P1（同文档顺手做）：拖放四件套（Enter/Over/Leave/Drop 带位置+MIME）、触控板缩放/旋转、主题切换、语言方向、设备插拔。
- P2（占位接口）：笔压感/倾斜/橡皮、显示器增减、智能放大。字段先占位为零值，后端未实现不报错。
- 手柄/摇杆/传感器另起接口，不进窗口事件。

### 4.3 非目标

- 不改 §2.3 窗口控制 SPI 形状；不动 GPU 渲染链路；Win32/AppKit 本次只定映射表 + 占位错误，不做真窗实现。

### 4.4 统一事件总表（非常全 · platform.Event → input.Event）

约定：坐标逻辑像素 Y 向下；指针 ID 0=鼠标，≥1=触控槽；⛔=协议天生无；🔨=S6 实现；Kind 只增不改号。
兼容：存量 `KindClose/Resize/Key/Pointer/Touch/Scroll/Text/IME/Wake` 全部保留；拆分只做加法（新增 `KindCloseRequested`，旧 `KindClose` 语义收窄为已销毁，`embedder.EventQuits` 同步改两侧都退）；指针/触控沿用存量 Kind（Down/Move/Up/Cancel 相位放载荷，不新增 Kind），笔新增一种 `KindStylus`（相位同样放载荷）；IME 复用 `IMEEvent.Kind`（compose/commit/caret/delete-surrounding），不拆 Kind；滚轮相位/笔倾斜/设备分类/MIME 载荷类型按 Gio/W3C 语义在 `ui/input` 内新增具名字段，零值=不支持。
存量保留：`KindWake`（跨线程唤醒）、`EventExpose`（底层重绘提示，不进上层，由 GPU 侧拥有像素）维持现状。

A 窗口生命周期/状态：

| 上层事件 | 携带 | X11 | Wayland | Win32 | AppKit | 备注 |
|---|---|---|---|---|---|---|
| KindCloseRequested | — | ✅ WM_DELETE 源头拆分 | ✅ | ⬜ | ⬜ | 可拦截；不调 Close 则存活 |
| KindClose | — | ✅ | ✅ | ⬜ | ⬜ | 已销毁；外部强杀通知 |
| KindResize | Width/Height/Scale | ✅ | ✅ | ⬜ | ⬜ | 客户区+dpr |
| KindMove | MoveX/MoveY | ✅ FromPlatform + 主循环路由 | ⛔ | ⬜ | ⬜ | Wayland 无事件即不支持 |
| KindScale | Scale | ✅ FromPlatform + 主循环路由（RandR + Xft.dpi） | ✅ FromPlatform + 主循环路由（output 监听取 max） | ⬜ | ⬜ | 多屏拖动/系统缩放 |
| KindOccluded | Occluded bool | ✅ VisibilityNotify | ✅ suspended 上报 | ⬜ | ⬜ | 全遮挡/最小化停渲染 |
| KindHidden | Hidden bool | ✅ 显隐发射 + 外部重映射通知 | ✅ | ⬜ | ⬜ | 应用显隐 |
| KindFocus | Focused bool | ✅ FromPlatform + 主循环路由；IsFocused 可查 | ✅ FromPlatform + 主循环路由（activated） | ⬜ | ⬜ | 键盘焦点 |
| KindStateChanged | Minimized/Maximized/Fullscreen bool | ✅ FromPlatform + 主循环路由（_NET_WM_STATE/WM_STATE 回读） | ✅ FromPlatform + 主循环路由（configure 推导） | ⬜ | ⬜ | 非原生事件，由配置回传推导；用户点按钮也被动通知 |
| KindThemeChanged | Dark bool | ⛔ | ⛔ | ⬜ | ⬜ | 深浅色；Linux 走设置守护，P1 polling/portal |
| KindFramePresented | — | ✅ XPresent | ✅ frame 回调 | ⬜ | ⬜ | pacing 用 |
| KindResizeSync | — | ✅ 统一 Kind + 原始排帧双通 | ⛔ | ⛔ | ⛔ | X11 同步 resize 专用 |
| KindMonitorChanged | Count/PrimaryScale | 🔨 RandR | 🔨 output 增减 | ⬜ | ⬜ | P2 显示器插拔 |

B 键盘/文本/输入法：

| 上层事件 | 携带 | X11 | Wayland | Win32 | AppKit | 备注 |
|---|---|---|---|---|---|---|
| KindKey | Key/Rune/Pressed/Repeat | ✅ XKB 连发检测置位（含异步 IME 保序） | ✅ 客户端合成连发置位 | ⬜ | ⬜ | Repeat 必带；存量 `fromplatform_test.go` 补 Repeat 断言 |
| KindModifiersChanged | Shift/Control/Alt/Meta | 🔨 状态变化即报 | 🔨 modifiers 事件即报 | ⬜ | ⬜ | 修饰键单独变化也报一次 |
| KindText | Text string | ✅ | ✅ | ⬜ | ⬜ | 提交文字（键字符/粘贴/IME 提交转正） |
| KindIMEPreedit | Text/Start/End | ✅ D-Bus（XIM 已移除） | ✅ text-input v3 | ⬜ | ⬜ | 复用 KindIME + IMECompose；行名是语义别名，不新增 Kind |
| KindIMECommit | Text | ✅ | ✅ | ⬜ | ⬜ | 复用 KindIME + IMECommit |
| KindIMEDeleteSurrounding | Start/End | ✅ | ✅ | ⬜ | ⬜ | 复用 KindIME + IMEDeleteSurrounding |
| KindLocaleChanged | Language/RTL bool | ⛔ | ⛔ | ⬜ | ⬜ | P1 系统语言方向 |

C 鼠标（含悬停/滚轮相位）：

| 上层事件 | 携带 | X11 | Wayland | Win32 | AppKit | 备注 |
|---|---|---|---|---|---|---|
| KindPointerMove | X/Y/Buttons | ✅ | ✅ | ⬜ | ⬜ | 悬停移动；复用 KindPointer + PointerMove |
| KindPointerDown/Up | X/Y/Button(1–5+前进后退) | ✅ 1–5 + 前进后退透传；6/7 转 ScrollX | ✅（含 8/9 侧键） | ⬜ | ⬜ | 5 键 + 横滚键区分；复用 KindPointer + PointerDown/Up |
| KindPointerEnter/Leave | X/Y | ✅ 带坐标 | ✅；Leave 补最近坐标 | ⬜ | ⬜ | 复用 KindPointer + PointerEnter/Leave；不再归并 Move |
| KindPointerCancel | — | 🔨 grab 中断上报 | 🔨 seat 丢失上报 | ⬜ | ⬜ | 复用 KindPointer + PointerCancel；手势被系统打断 |
| KindScroll | ScrollX/ScrollY + Phase(Started/Moved/Ended) | ✅ 4/5 竖滚 + 6/7 横滚 | ✅ axis 对齐 | ⬜ | ⬜ | 复用 KindScroll；触控板惯性走 Moved→Ended |

D 触控（多点）：

| 上层事件 | 携带 | X11 | Wayland | Win32 | AppKit | 备注 |
|---|---|---|---|---|---|---|
| KindTouchDown/Move/Up/Cancel | ID(≥1)/X/Y | ✅ FromPlatform + 主循环路由（XI2 探测 + 解析 + 门控） | ✅ FromPlatform + 主循环路由（wl_touch 绑定 + 解码） | ⬜ | ⬜ | 复用 KindTouch + TouchEvent.Kind；一槽一竞技场 |
| KindTouchForce | ID/Pressure 0–1 | ⛔多数屏 | ⛔ | ⬜ | ⬜ | 无则零值，不报错 |

E 笔/压感（P2 占位）：

| 上层事件 | 携带 | X11 | Wayland | Win32 | AppKit | 备注 |
|---|---|---|---|---|---|---|
| KindStylusDown/Move/Up | X/Y/Pressure/TiltX/TiltY/Eraser bool | 🔨 XInput2 | ⛔ tablet 协议二期 | ⬜ | ⬜ | 本表唯一新增 Kind 族（相位放载荷）；无压感设备 pressure=1 |

F 触控板手势（P1–P2）：

| 上层事件 | 携带 | X11 | Wayland | Win32 | AppKit | 备注 |
|---|---|---|---|---|---|---|
| KindPinch | ScaleDelta + Phase | ⛔ | 🔨 gesture 协议 | ⬜ 精密触控板 | ⬜ | 双指缩放 |
| KindRotate | AngleDelta + Phase | ⛔ | 🔨 gesture 协议 | ⬜ | ⬜ | 旋转 |
| KindSmartMagnify | — | ⛔ | ⛔ | ⛔ | ⬜ | mac 双击放大；他端恒无 |

G 拖放（对齐 Gio transfer：MIME + 文件）：

| 上层事件 | 携带 | X11 | Wayland | Win32 | AppKit | 备注 |
|---|---|---|---|---|---|---|
| KindDragEnter/Over | X/Y/MIMETypes | 🔨 XDND | 🔨 wl_data_device | ⬜ | ⬜ | 悬停高亮用；平台只有 Drop，需新接 XDND/data-device |
| KindDragLeave | — | 🔨 XDND leave | 🔨 data-device leave | ⬜ | ⬜ | 取消高亮；同上新接 |
| KindDrop | X/Y/Files/MIME/Data | ❌ 无 XDND 接线、永不产生（S6-P1） | ✅ FromPlatform + 主循环路由（text/uri-list） | ⬜ | ⬜ | 文件先行，MIME 数据二期 |

H 设备热插拔（P1）：

| 上层事件 | 携带 | X11 | Wayland | Win32 | AppKit | 备注 |
|---|---|---|---|---|---|---|
| KindDeviceAdded/Removed | DeviceClass(键/鼠/触/笔) + Name | 🔨 XI 层级变化 | 🔨 seat caps | ⬜ | ⬜ | 键鼠插拔通知 |

### 4.5 改动点（文件级，按 P0→P1→P2 顺序）

1. `ui/input/event.go`：增 §4.4 全部 Kind + 载荷（Kind 只增不改号；P2 字段先占位）。
2. `ui/platform/host.go`：`PointerKind` 加 Cancel、`input.PointerKind` 加 Enter/Leave、`Event` Key 段加 Repeat（先有位，后有值）。
3. `ui/input/fromplatform.go`：补全映射 + 拆 Close + 置 Repeat 位 + X11 横滚转 ScrollX。
4. `ui/embedder/input_router.go`：增对应 Route 分支（焦点/拖放/遮挡/主题/设备默认透传回调，不吞事件）。
5. `ui/embedder/pipeline_app.go` + `ui/application`：消费方从读 `platform.Event` 切到读统一事件（Occluded/Hidden/FramePresented 分支先切）。
6. 后端 P0：X11（Close 拆分源头/focused 回写/Enter-Leave 坐标/Hidden 发射/横滚/XI2 触控/Repeat 位/StateChanged 回读）+ Wayland（wl_touch 绑定 + scale 上报 + Leave 坐标）。
7. 后端 P1：XDND/wl_data_device 拖放四件套、触控板手势、主题/语言/设备插拔。
8. 按键映射表：X11 keysym / Wayland xkb 为准，Win32 虚键 / AppKit 键码占位映射，接口形状不变。

### 4.6 验收（§2.6 三层，命令以 §2.6 第 6 条为准不另抄）

- 跑 §2.6 三命令（C 恒绿、B 含 HideShow、A 两真窗 18+1 口径）；Win32/AppKit 验占位错误明确。
- 存量测试同步改：`ui/input/fromplatform_test.go`（Close 拆分 + Repeat 位 + Enter/Leave/Cancel 独立）、`ui/embedder` EventQuits 测试（两侧都退）。
- 回归纪律：按测试文件逐个跑，禁止一次全量（AGENTS.md）。

### 4.7 新会话开工顺序（P0 一天量级切分）

1. `ui/input/event.go` 加新 Kind + 载荷字段（只增不改号；笔是唯一新增 Kind 族）。
2. `ui/platform/host.go` 加三位（Cancel/Enter-Leave/Repeat）+ `ui/input/fromplatform.go` 补映射（拆 Close、置 Repeat、横滚）。
3. `ui/embedder/input_router.go` 加 Route 分支透传 + 单测。
4. `pipeline_app.go` + `application` 消费方切换（Occluded/Hidden/FramePresented 先切）。
5. X11/Wayland 后端 P0 上报 + §2.6 三层真窗验证。
6. P1/P2 按 §4.4 表逐行落地，每行补行内验收。

### 4.8 S6 阶段状态管理（每阶段真窗 + 状态双锁）

- 状态只认 §3 表：S6-P0/P1/P2 各自独立成行，完成一阶段改一阶段状态（✅/🔨/⬜），禁止跨阶段提前标绿。
- 每阶段真窗：P0 用存量 `ui_pf_x11`/`ui_pf_wayland` 全能力重跑（新增事件行全 PASS/⛔诚实降级）+ C 层 `pfkit` 恒绿；P1 拖放/手势/主题用专项真窗（有人机交互的按 A′ 人工验收）；P2（含 Win32/AppKit 占位）验明确错误非 panic。
- 每阶段收尾：跑 §4.6 三命令 + 存量同步改测试绿，才准改 §3 状态并在 §5 修订追加一行。

### 4.9 存量实现索引（防漏：代码有、新会话易重写或误判为缺失）

- 路由：祖先链冒泡（命中节点一路到根）+ 框外拖选补发 + 预编辑单发守卫去重 + composing 键过滤 + 失能吞字 + 锚点/环绕去重（`ui/embedder/input_router.go`）；`EventTarget` 四 Handler 接口（`ui/input/eventtarget.go`）；覆盖层先测 top→bottom（`ui/overlay/state.go`）。
- 手势：Tap/Pan/滚轮旁路/竞技场单赢家 + Up 收尾（`ui/gestures/`；Cancel 不结场，S6 修）。
- 焦点：窗口键盘焦点（`platform.Event.Focused`）≠ 控件焦点（`ui/focus` Manager/Node/Tab 排序/空格回车激活），两词勿混。
- 文本：Editor delta 模型/撤销 100 组/组合文状态机/双击选词三击选行（`ui/textinput/`）；IME 会话门面（`ui/embedder/ime_session.go`，Enable 丢 Type、SetPurpose 丢 Hints、PushSurrounding 令 anchor=cursor，现状）。
- 输入构造器：`FromTouch/FromText/FromIME`（`ui/input/fromplatform.go`，有函数体；Touch/Text 经管线到不了 Router，只能直调）；`Event.WindowID` 多窗 ID 现无人赋值；键表 F13–F24/小键盘/IME 键/符号键全在（`ui/input/keys.go`），映射子集见 `mapKey`。
- 平台：剪贴板双端（见 §2.2.1）、帧通知双源（见 §2.4 FramePresented 行）、CSD 双击/菜单/8 向/悬停（见 Wayland 标准）、D-Bus 双引擎与 text-input 队列（见 §2.2.1/§2.4 IME 行）。

## 5. 修订

- v2.17（2026-09-12）：**S6-P0 统一打通**——按文档修源码：`FromPlatform` 补 Move/Scale/Focus/Drop/ResizeSync 五分支（含 Scale 缺省回 1）；主循环路由放行 Touch/StateChanged/Move/Scale/Focus/Drop，Occluded/Hidden/FramePresented 在生命周期处理后同步透传路由器观察，Resize/ResizeSync 在原始排帧后同步透传；`fromplatform_test.go` 新增窗口事件断言；`ui/input` + `pfkit` + embedder 路由/退出测试绿。§4.4 七行与 §3 S6-P0 回 ✅。真机复验（X11 真 GPU 桌面 DISPLAY=:0）：B 层 7 例 + 同步帧 3 例 + S6-P0 文件 12 例全绿（整组首跑 1 例窗口管理器时机偶发失败、单跑与整组重跑均过），A 层 `ui_pf_x11` 19/19（事件泵实收 move/state-changed/occluded/hidden/focus）；Wayland 无 WAYLAND_DISPLAY 诚实 Skip。
- v2.16（2026-09-12）：**源码为准回齐**——以源码实现为真源、文档跟着走：§4.4 七行按源码降为 🔨（Move/Scale/Focus/Drop/ResizeSync 在 FromPlatform 无分支直接变 KindNone；Touch/StateChanged 转换有但主循环只放 Pointer/Key/IME、生命周期只认 Occluded/Hidden/FramePresented，到不了上层；ResizeSync 只走原始排帧）；§3 S6-P0 由 ✅ 改 🔨并注原因；S1 删“待拆分”旧话（两端拆分源码已做）；同步修三处注释（Visible 的 Wayland 忽略、Hidden 的 Wayland only、pfkit 的 Show 反例）+ `Window.Backend()` 补 AppKit 分支（源码内部对齐）。
- v2.15（2026-09-12）：**S6-P0 收尾**——P0 缺口补齐：X11 RandR 缩放监听（RRScreenChangeNotify + Xft.dpi 推导 + 根 RESOURCE_MANAGER 通知）与 Wayland StateChanged 推导（configure 三元组 + 控制器乐观写统一经 setStateTriple，变更才报）；X11 异步 IME 按键加链保序（连发释放/按下不再反转）；Wayland 客户端合成连发置 Repeat 位；输出表 nil-map 真机 panic 修掉。§4.6 三命令重跑：C 恒绿、B 两端真窗绿（含嵌套 Mutter 下 Wayland B 4 例 ×3 + A 19/19）、A X11 19/19；X11 S6-P0 文件 12 例 + Wayland 回调 6 例绿。§2.4 七格翻绿、Key 行补 Repeat 载荷、补 StateChanged/Touch 两漏记行，§4.4 十四格翻绿（指针 Cancel 双端无源头仍 🔨、修饰键/手势/拖放/P2 占位次阶段），§3 S6-P0 → ✅。
- v2.14（2026-09-11）：**开工前收敛**——消七处分歧重复：P0 补 StateChanged 推导、Wayland 去 suspended/activated 重复项；笔 Kind 口径统一（全表唯一新增 Kind 族）；Leave 补 Wayland 无坐标；§4.5 加平台加位项并重编号；§4.6 验收命令单源化指 §2.6；§2.6 A′ 双注释修正。开工线：§4.4 表 → §4.5 文件序 → §4.7 步骤 → §4.8 收尾。
- v2.13（2026-09-11）：**五路全量互查补漏**——发射点约43处/公开符号约80个/ErrUnsupported 约15处逐项对表：§0 修死链（IME 计划文档改指两份输入法需求）；§2.1 补 IconName 行、探测优先级、光标 shape 首选；§2.2 补 Backend() 行 + §2.2.1 IME/Clipboard 能力现状；§2.3 边界注删显隐、Move/Resize 加条件错注；§2.4 补空拖放过滤、两端缺 caret、Leave 无坐标、帧通知双源、剪贴板错误口径；§2.6 补 sync 测试文件、HideShow 正则、mode 字段、18+1 口径、A′ 双修正；§3 S1/S4 两处过期；新增 §4.9 存量实现索引。联动 Wayland 标准（光标 shape 首选、set_cursor opcode 0、axis 空实现、Leave 无坐标、v1.2 可见性过期）+ 三处代码注释。编译 + `ui/input` 测试绿。
- v2.12（2026-09-11）：**补已有实现漏记行**——§2.4 补五漏记事件行（Expose/ResizeSync/Drop/Hidden/FramePresented，代码早有、表里没有）：X11 同步帧全链路、两端帧显示、Wayland 拖放落下与显隐双向至此才有名分；同步修两处代码注释（`host.go` IME 补 3=delete-surrounding、`windowctl.go` 显隐 Wayland 真实现）；编译 + `ui/input` 测试绿。
- v2.11（2026-09-11）：**四路源码比对诚实化**——X11/Wayland/输入层/消费方四 agent 逐行审计 + 抽查复核（X11 零发射 CloseRequested、Wayland 三上报实锤、`ui/input`+`pfkit` 基线绿）：§2.4 翻绿 Wayland 四格（CloseRequested/Occluded/Enter-Leave/Focus）、X11 关闭标坍缩、Scale 注明无监听、IME 改 D-Bus 并注缺 caret、焦点注查询恒 false；§2.1/§2.3 Wayland 显隐 ⛔ 改真实现；§4.1 改直读/忽略与 Cancel 口径；§4.4 修正 CloseRequested/Hidden/StateChanged/Drop/Repeat/横滚/Enter-Leave 坐标格。代码未动，S6 按 §4.7 开工。
- v2.10（2026-09-11）：**S6 阶段管理补齐**——§3 增 S6-P0/P1/P2 状态行（⬜起步）；新增 §4.8 阶段状态管理（每阶段真窗 + 状态双锁 + 收尾改状态追加修订）。
- v2.9（2026-09-11）：**S6 通审补漏**——头部版本对齐 v2.9；§4.4 增兼容约定（存量 Kind 全保留、指针/触控/IME 用载荷相位不拆 Kind、Wake/Expose 维持现状）、修正 DragLeave/Cancel/StateChanged/Modifiers 标注、增存量测试同步改与回归纪律；新增 §4.7 开工顺序。新会话可直接按 §4.5→§4.7 开工。
- v2.8（2026-09-11）：**S6 非常全总表**——§4.4 展开为 A–H 全量统一事件总表（窗口/键盘/鼠标/触控/笔/触控板手势/拖放/设备/系统环境，含四平台⛔/🔨标注 + P0/P1/P2 分期），对照 winit/GTK4/Flutter/Gio/W3C；§4.5 改动点按分期重排。
- v2.7（2026-09-11）：**S6 立项**——新增 §4 上层事件统一需求（§4.1–§4.6 完整版，可直接开工）：§2.4 窗口事件 + 输入残缺口（Enter/Leave/Cancel、触控、横滚、关闭区分、四平台键表）统一进 `ui/input`，对齐 winit/GTK4/Flutter/W3C。
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