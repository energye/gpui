# Wayland 标准窗口（完整 CSD+交互）— 设计真源

> **版本：v1.5** | 日期：2026-08-19  
> **地位：** `ui/platform` Wayland 窗口标准实现唯一设计真源。  
> **并读：** [`ENGINE_ARCH_OVERVIEW.md`](./ENGINE_ARCH_OVERVIEW.md)（分层）· [`ENGINE_INPUT_IME_PLAN.md`](./ENGINE_INPUT_IME_PLAN.md)（输入/IME 已落地）  
> **参照实现：** sctk `shell/xdg/window`（状态机）· gogpu `internal/platform/wayland/csd*.go`（CSD 绘制）· GTK4 `gtkimcontextwayland.c`（输入）· winit wayland。

---

## 0. 用户要求（验收与范围 · 不可删改义）

> 以下为工程约束原文级固化；实现与文档冲突时 **以本节为准**。

- W1：完整可用的 **Wayland 标准窗口**：完整窗口外观装饰 + 完整窗口行为。
- W2：**不按用户早前"边框不要画"的临时说法做**——整体按成熟框架（sctk/winit/GTK/gogpu）的标准实现：标题栏 + 四边边框 + 8 方向 resize + 系统光标。
- W3：可配置（`Options.Decorations`）：**true=标准装饰；false/缺省=无框裸窗**（手动开启）。
- W4：不崩溃、可 resize、可拖动、按钮全功能、焦点态、光标态。
- W5：GNOME 42.9 无 SSD（`zxdg_decoration_manager_v1` server-side 是 KDE 特性）→ 标准 CSD。
- W6：GNOME 42.9 **无 wp_cursor_shape_manager_v1** → resize 光标用 `wl_cursor_theme`（libwayland-cursor）系统主题。

---

## 1. 架构分层

```
ui/platform/wayland_linux.go          — 连接/注册表/接口表/事件循环/wlWin
ui/platform/wayland_csd_linux.go      — CSD 管理器：subsurface 布局/状态/交互/光标
ui/platform/wayland_csd_painter.go    — CSD 绘制：标题栏/按钮/边框/位图字体
ui/platform/wayland_pointer_linux.go  — wl_pointer 事件 → hit-test → 动作
ui/platform/wayland_linux.go          — wlTopConfigure states→WindowState
```

- CSD 与内容 surface 分离：标题栏是 1 条 `wl_subsurface`（挂在内容 surface 上方，`set_desync` 独立提交）；四边边框是 3 条 subsurface（左/右/下）。**不画在内容 surface 内部**。
- 内容 surface 由 wgpu/render 层拥有（父 surface）；装饰 subsurface 由 CSD 拥有。

---

## 2. 协议要点（已核对权威 xml）

### 2.1 装饰 subsurface（wayland.xml 权威）
| 请求 | opcode | 签名 |
|---|---|---|
| wl_surface.attach | 1 | `oii`（buffer, x, y）⚠️ 必传 3 参，nil 会 SIGSEGV |
| wl_surface.damage | 2 | `iiii` |
| wl_surface.commit | 6 | — |
| wl_shm.create_pool | 0 | `nhi`（new_id, fd, size） |
| wl_shm_pool.create_buffer | 0 | `niiiiu`（new_id, offset, w, h, stride, format） |
| wl_subcompositor.get_subsurface | 1 | `noo`（new_id, surface, parent） |
| wl_subsurface.set_position | 1 | `ii` |
| wl_subsurface.set_sync / set_desync | 4 / 5 | — |

**装饰 subsurface 用 set_desync**：独立提交立即生效，不依赖父 surface（wgpu）commit 节奏（修 resize 错位）。

### 2.2 xdg_toplevel（xdg-shell.xml 权威，14 请求全表）
| 请求 | opcode | 签名 | 用途 |
|---|---|---|---|
| set_title | 2 | `s` | 标题 |
| set_app_id | 3 | `s` | 应用 ID |
| show_window_menu | 4 | `ouii` | 窗口菜单（标题栏右键） |
| move | 5 | `ou` | 拖动 |
| resize | 6 | `ouu` | resize（edges） |
| set_max_size / set_min_size | 7 / 8 | `ii` | 尺寸约束 |
| set_maximized | 9 | — | 最大化 |
| unset_maximized | 10 | — | 还原 |
| set_fullscreen | 11 | `?o` | 全屏（output 可空） |
| unset_fullscreen | 12 | — | 退出全屏 |
| set_minimized | 13 | — | 最小化 |
| destroy | 0 | — | 销毁 |

**必填 14 条方法表**（MethodCount=14 + types 数组），否则 libwayland marshal 越界 → 合成器 `invalid arguments` → 断窗。

**configure 事件 states 是 wl_array（uint32 数组），不是 bitfield**：
```
state 值：1=maximized 2=fullscreen 3=resizing 4=activated
         5..8=tiled_l/r/t/b 9=suspended
```
解析：`struct wl_array { size_t size; void *alloc; void *data; }`（size@+0, data@+16 amd64）逐 u32。

**resize_edge 枚举**：none=0 top=1 bottom=2 left=4 top_left=5 bottom_left=6 right=8 top_right=9 bottom_right=10。

**WindowEdge ↔ xdg resize_edge 映射**（wlController.RequestResize 用；主文档 §2.3 WindowEdge 枚举与 xdg 值不同，必须以本表为准）：
| WindowEdge | xdg resize_edge | 值 |
|---|---|---|
| None | none | 0 |
| Top | top | 1 |
| Bottom | bottom | 2 |
| Left | left | 4 |
| TopLeft | top_left | 5 |
| BottomLeft | bottom_left | 6 |
| Right | right | 8 |
| TopRight | top_right | 9 |
| BottomRight | bottom_right | 10 |
请求 `RequestResize(edge)` 传 xdg 值，禁止按 WindowEdge 的 iota 顺序直传。

### 2.3 光标（GNOME 42.9 无 cursor-shape）
- `wl_cursor_theme_load(NULL, 24, shm)` → theme；`wl_cursor_theme_get_cursor(theme, name)` → cursor image（含 `wl_buffer`）。
- `wl_pointer.set_cursor(serial, surface, hotspot_x, hotspot_y)`（opcode 1，签名 `ouii`）。
- 光标名：`sb_v_double_arrow`（N/S）、`sb_h_double_arrow`（E/W）、`top_left_corner`（NW/SE）、`top_right_corner`（NE/SW）、默认 `left_ptr`。
- 创建专用 cursor wl_surface，attach cursor buffer + commit。

### 2.4 Options 全量接线（与主文档 §2.1 对齐）

| Options 字段 | Wayland 处理 |
|---|---|
| Width/Height/Title/Backend | ✅ 现有（waylandCreate 已接） |
| Decorations | ✅ 现有（CSD / 无框）+ zxdg_decoration configure 协商（§6.4） |
| IconName | ✅ set_app_id（§6.4；X11/Win32/AppKit 未接，Options 注释注明） |
| Min/Max Size | ✅ set_min/max_size（waylandCreate 全量） |
| Resizable=false | ✅ min==max 锁（waylandCreate + SetResizable） |
| Position | ⛔ 协议无客户端定位 |
| Fullscreen | ✅ 创建后 set_fullscreen(NULL→当前输出) |
| Cursor | ✅ enter 时应用 wl_cursor_theme（win.cursor，9 形状映射） |
| Maximized | ✅ 首 commit 前 set_maximized |
| Visible | ✅ 创建后隐藏：hideNative（§6.2；false=不可见启动） |

---

## 3. 窗口状态机（sctk 对齐）

```
WindowState: Maximized | Fullscreen | Resizing | Activated | TiledL | TiledR | TiledT | TiledB | Suspended
```

- `wlTopConfigure`：**先累积** states/尺寸 → 记 `w.resized` + `w.csd.setWindowState(...)` → 渲染层读 EventResize；`wlXdgConfigure` 侧 ack_configure（协议要求 configure 必须先 ack）。
- `activated`（focus）→ 标题栏聚焦/失焦底色切换，并上报 `EventFocus{Focused}`（值变化才发）。
- `maximized` → 最大化按钮图标 `□`→`❐`，且 maximized 无边框（标准行为：最大化时隐藏装饰边框）。
- `suspended`（9）→ 上报 `EventOccluded{Occluded:true}`（停渲染省电）；解除 → `Occluded:false`。禁止把 suspended 当 minimized 处理。
- 关闭：CSD ✕ 与 xdg close（`wlTopClose`）→ `EventCloseRequested`（可拦截）；surface 实际销毁（`wl_surface.destroy` / `wl_registry.global_remove`）→ `EventClose`。

### 3.1 指针事件上报（wl_pointer → Event）

| 协议事件 | 上报 Event |
|---|---|
| enter | `EventPointer{Pointer: PointerEnter, X, Y}` |
| leave | `EventPointer{Pointer: PointerLeave}` |
| motion | `PointerMove`（现有） |
| button | `PointerDown/Up`（现有） |
| axis | `PointerScroll`（现有） |

> enter/leave 除驱动 CSD hover/光标外，必须同时上报给上层（hover 判定依赖）；CSD 消费与上报并存，不互斥。

### 3.2 状态查询口径（协议无查询 = 乐观跟踪）

- `IsMinimized()`：xdg configure **无 minimized 状态** → 乐观跟踪：`set_minimized` 后置 true；收到 `activated` 回传置 false；查询返回跟踪值（与 X11 同款注解）。
- `IsVisible()`：客户端跟踪（hideNative/showNative 置位），Hide → false、Show → true；非「合成器 unmapped」概念（§6.2）。
- `IsMaximized()/IsFullscreen()`：configure states 回传为准（真实值，非乐观）。

---

## 4. CSD 外观规范

| 项 | 值 |
|---|---|
| 标题栏高 | 32px |
| 边框宽 | 4px（可见边框 + resize 热区） |
| 按钮宽 | 44px（右对齐 3 个）|
| 标题对齐 | 居中（可配左/右）|
| 聚焦底色 | #2B2D30（深灰）|
| 失焦底色 | #202224（更深）|
| 按钮 hover | #3E4144 |
| 按钮 press | #333538 |
| 关闭 hover | #C42B1C（红）|
| 关闭 press | #B22A1A |
| 图标色 | #DFE1E5，关闭 hover 白 |
| resize 角 | 8px grip |

- 标题栏右侧按钮：`— 最小化`、`□/❐ 最大化/还原`、`✕ 关闭`。
- hover/press：pointer motion 更新按钮状态 → **只重绘标题栏 subsurface**（不碰内容）。
- 位图字体 5x7（ASCII + 数字，见 painter.go），标题截断到按钮左侧。

---

## 5. 交互行为

| 命 中 | 区域 | 动作 |
|---|---|---|
| CSDHitCaption | 标题栏非按钮区 | `xdg_toplevel.move(seat, serial)` |
| CSDHitClose | 右 44px | 请求关闭（EventCloseRequested，可拦截）|
| CSDHitMaximize | 右 88-44px | set/unset_maximized |
| CSDHitMinimize | 右 132-88px | set_minimized |
| CSDHitResizeN/S/E/W | 边框/边缘 | `xdg_toplevel.resize(seat, serial, edge)` |
| CSDHitResizeNW/NE/SW/SE | 角 | 同 |
| 双击 Caption | 标题栏 | 最大化/还原（GTK 行为）|

- 命中判定：`wl_pointer` enter/motion 携带 surface + surface-local 坐标；`button` 事件无坐标 → 用最近 motion 坐标（enter 也记坐标，避免进入即点击时 miss）。
- **enter serial 必须记录**：set_cursor 和 move/resize 都需要合法 serial。
- motion 每次更新光标（enterSerial + hitTest）。

---

## 6. resize 后 CSD 跟随

- `wlTopConfigure` 尺寸变化 → `w.resized=true` → `poll()` 里 `csd.resize(w,h)`：
  - 标题栏 subsurface：宽 = 内容宽，高 32，位于 (0, -32)；
  - 左/右/下边框 subsurface：按新内容尺寸 resize；
  - **只重建 shm buffer（保留 wl_surface/wl_subsurface 对象）**，交互式 resize 不打断 pointer grab；
  - detach 旧 buffer 必须传 `(nil, 0, 0)` 三参（attach 签名 `oii`）。

---

### 6.1 窗口几何（xdg window geometry）声明 — swapchain 同步

- **设计**：客户端几何 = 内容矩形（不含 CSD 装饰），对齐 GTK `gtk_window_set_geometry_hints` 内容矩形语义。`set_window_geometry(0,0,cw,ch)` 在**渲染器 swapchain 重配到新尺寸的同一 wire batch** 里发出（几何 → attach 同尺寸缓冲 → commit），保证合成器每笔提交看到「几何 ≤ surface 且几何 == 内容尺寸」。
- **链路**：`render.PresentTarget.SetOnSwapchainResized(fn)`（呈现临界区、raster 线程）→ `platform.SurfacePresenter.OnSurfaceResized(w,h)`（可选接口，`ui/platform/host.go`）→ `wlHost.OnSurfaceResized` → `wlCSD.setGeometryNoFlush`（不显式 flush，请求挂在 present 自身批上，与 UI 线程的 wl_display_flush 无竞态；同名 marshal 保留 flush 版 `setGeometry` 供首帧声明）。embedder `pipeline_app.go` Open 时接线。
- **反模式（已修，勿回退）**：在 `EventResize`/configure 钩子里同步声明几何 = 声明提前于同尺寸缓冲（swapchain 重配 ~90ms 滞后、期间旧尺寸缓冲继续 commit）。合成器（mutter 42.9）在「几何 > surface」的提交上缓存**负 frame extents**（`custom_frame_extents = surface − geometry`，且只在几何变化或有 ack configure 的提交重算，稳态提交永不刷新）→ 取消最大化时 size-hints 往返把恢复尺寸翻转成 `work-area − 内容`（实测 400×400 → 1468×653，连累后续取消全屏）。判定该反模式：`WAYLAND_DEBUG` 下几何请求落在首 configure（~ms 级）而 swapchain 切换在 ~90ms 后。
- **验收**：`tmp_resize_diag` DIAG_STATE=1（Maximize→Unmaximize→Fullscreen→Unfullscreen→SetTitle→Minimize，3s 步进）在 DIAG_SIZE=400/800 下取消最大化/全屏恢复 configure 恒等于请求尺寸；`WAYLAND_DEBUG` 确认恢复路径的 `set_window_geometry` 在对应的新尺寸 configure 之后、同批 commit 之前。

### 6.2 隐藏/恢复窗口（Show/Hide/IsVisible + EventHidden）

- **语义**：`WindowController.Hide()` → `hideNative()`：置 hidden 标志、隐藏 CSD 装饰、上报 `Event{EventHidden, Hidden:true}`；embedder 收到后**停帧 → 等 raster 线程清空在途 present → `PresentTarget.Close()`（释放 wgpu WSI surface）→ `HiddenSurface.ApplyHiddenDetach()` 销毁整个 surface 栈（wl_surface + xdg_surface + xdg_toplevel + deco + CSD subsurface）**——窗口从合成器真正消失（xdg 无 unmap 请求，GTK4 `gdk_wayland_window_hide` 同款「销毁+重建」方案）。`Show()` 对称重建：`showNative()` 在**事件泵线程之外只发请求不 dispatch**（泵是唯一事件源），重建代理栈 + 重放 title/app_id/装饰协商/min-max/fullscreen/CSD + 空 commit；泵线程收到首个新 configure 后 ack + commit（重新映射），poll 观察到 `recreated && configured` 才补发 `EventHidden{Hidden:false}`——embedder 此时用**新的 wl_surface** 重开 PresentTarget，帧恢复。
- **顺序硬约束**：wgpu 的 Wayland WSI surface 由内容 wl_surface 支撑，**必须先关 PresentTarget 再销毁 wl_surface**（present 与销毁竞争会挂死——早期 attach(NULL) 方案实测 `Present` 永久卡死，SIGQUIT 定位）。`Options.Visible=false` 走同一 hideNative 路径：窗口以未映射状态启动，首次泵排空 EventHidden 后完成拆除。
- **状态持久**：title/app_id/min-max/resizable/fullscreen/maximized/decorated 全部存于 wlWin 跟踪态，重建时逐项重放（WAYLAND_DEBUG 实测：重建的 toplevel 收到 set_title("…")、set_app_id、set_min/max_size）。
- **并发安全**：`wlHost.destroying` 标志 + wake pipe——非事件线程 Close 时，parked 在 poll 里的 WaitEvents 被唤醒、见标志即返回 nil，不在已销毁的 display 上 dispatch（曾触发 SIGSEGV）。

### 6.3 剪贴板与外部文件拖拽（wl_data_device 全套）

- **绑定**：registry 取 `wl_data_device_manager` → 座位闪光后 `bindDataDevice()` 建 manager + device（version 1）+ 6 个 device 回调 + offer/source listener；合成器无此 global 时 `clipFor` 返回 nil（剪贴板静默降级，示例层回退内部剪贴板）。
- **写（Set）**：`create_data_source`(v1) + `offer(text/plain)` + `set_selection(device, source, serial)`，payload 存本地，peer 拉取时经 `source.send(fd)` 写入。
- **读（Get）**：自读短路（ownSource 存活 → 返回本地副本，不走合成器）；外部选择走 `offer.receive(fd)` + 事件线程 dispatch 下阻塞读（≤3s）；空选择返回 error（不假空串）。
- **set_selection 焦点规则（mutter 42.9 实测）**：仅键盘焦点客户端可 set_selection，无焦点 → 合成器直接 `source.cancelled`（源生存期结束，读端报「clipboard is empty」；已在 mutter 源码 `meta-wayland-data-device.c` 核实：仅向 focus 客户端广播选择、无 serial 时效检查）。
- **DnD（EventDrop）**：device enter/motion/leave/drop 回调维护拖拽状态；drop 时若 mime 含 `text/uri-list` → receive 读入 + `parseURIList`（剥离 `file://`、host 前缀、unescape、跳过 `#` 注释行/空行）→ 上报 `Event{EventDrop, Files, X, Y}`，坐标用 `wl_fixed`（int32 24.8 定点 ÷256）。
- **代理生命周期**：offer/source 销毁一律 defer 到事件线程（`pendingDestroys` 队列在 `wlHost.poll` 的 `drainPendingDestroys` 排空），避免「Get/Set 任意线程 + 事件线程 dispatch」双重释放；**每条目携带各自 destroy opcode**（`wl_data_offer.destroy=2`、`wl_data_source.destroy=1`，共用同一 opcode 会在 source 上发成 set_actions → 协议错误断连，实测 `invalid method 2 (since 1 < 3)`）。

### 6.4 应用图标名与装饰协商

- **图标名**：`Options.IconName` → `xdg_toplevel.set_app_id("gpui.l1" 为缺省)`；合成器据 app_id 解析任务栏图标。协议无查询回读，验收 = 自定义值不破坏创建（tracer 见 set_app_id）。
- **装饰协商**：绑定 `zxdg_decoration_manager_v1`（version 1），创建时 `get_toplevel_decoration` + listener；`wlDecoConfigure` 收到模式后：decorated → `set_mode(SSD)`，frameless → `set_mode(NONE)`；**GNOME 42.9 无全局装饰选项**（server-side 是 KDE 特性）→ 合成器给 NONE → 客户端自绘 CSD；`decorated` 回传时隐藏 CSD surface（SSD 接管）。`Options.Decorations` 决定协商请求：true=decorated、false/缺省=裸窗（不设 set_mode）。

---

## 7. 分期与验收

| 阶段 | 内容 | 验收 |
|---|---|---|
| S1 | 协议层完整（接口表/shm/subcompositor/cursor） | ✅ 已落地 |
| S2 | CSD subsurface + painter（标题栏/边框/按钮/字体） | ✅ 已落地 |
| S3 | 交互（move/resize 8 向/三按钮/双击最大化/焦点态/光标）| ✅ 已落地 |
| S4 | 真窗验收（用户跑 ui_textinput_ime）| ✅ 已验收（窗口完整装饰 + 全行为）|
| S5 | **主文档 S2 对齐**：wlController（Options 全量 §2.4 + IsMinimized/RequestMove/RequestResize）+ 事件上报（§3.1/§3.2 闭环：EventCloseRequested/EventFocus/EventOccluded/PointerEnter/Leave）| ✅ 已落地（main doc S2 ✅；ui_pf_wayland 19 项 PASS + TestWaylandRealWindowControls） |
| S6 | **§9 标准窗口需求落地**：图标名 set_app_id（§6.4）+ 装饰协商（§6.4）+ 隐藏/恢复 Show/Hide/IsVisible/EventHidden（§6.2，销毁+重建真隐藏）+ 剪贴板读写 + 外部文件拖拽（§6.3）| ✅ 已落地（wayland_features_linux_test 真窗验证 + tmp_resize_diag DIAG_FEATURES=1：query/clipboard/hide-show 全过，exit 0；WAYLAND_DEBUG 实测隐藏点 `wl_surface.destroy` + Show 重建新 wl_surface + 标题/app_id/min-max 重放）|

**回归纪律**：`go test ./ui/platform/` 按文件跑，禁止一次全量；真窗由用户执行。

---

## 8. 修订

- v1.5（2026-08-19）：**真隐藏（§6.2 重写）**——隐藏从「停帧+chrome 隐藏（wgpu WSI 不能外部 attach(NULL)）」升级为「销毁+重建」：embedder 收 EventHidden 后停帧 → 等 raster 空闲 → `PresentTarget.Close()`（先释放 wgpu WSI surface）→ `ApplyHiddenDetach()` 销毁 surface 栈（真 unmap，GTK4 同款）；Show 在泵线程外异步重建代理栈（不 dispatch），泵 ack 首个新 configure 后映射，poll 补发 EventHidden{false}，embedder 以新 wl_surface 重开 PresentTarget。抽出 `createSurfaceStack/applySurfaceConfig/destroySurfaceStack`；修复 listener 回调初始化在重建路径丢失（libwayland abort）。验证：wayland_features TestWaylandHideShow（detach 后 NativeSurface=0 → Show 后新 surface + 重映射）+ diag DIAG_FEATURES=1 EXIT=0 + WAYLAND_DEBUG 线序（隐藏点 `wl_surface@5.destroy()`，Show 重建 `wl_surface@56`/`xdg_toplevel@61` + title/app_id/min-max 重放）。
- v1.4（2026-08-19）：**§9 标准窗口需求落地**——§6.2 隐藏/恢复（Show/Hide/IsVisible/EventHidden + wgpu WSI attach(NULL) 限制）；§6.3 剪贴板读写 + 外部文件拖拽（wl_data_device 全套，offer/source 各自 destroy opcode、set_selection 焦点规则）；§6.4 图标名 set_app_id + zxdg_decoration 装饰协商；§2.4 Options 表 Visible/IconName 转 ✅、§3.2 IsVisible 改客户端跟踪；§7 加 S6。验证：wayland_features_linux_test（真窗 4 PASS + 1 预期 SKIP，8 连跑无崩溃）+ tmp_resize_diag DIAG_FEATURES=1 exit 0；并发安全：wlHost.destroying 防非事件线程 Close 的 SIGSEGV。
- v1.3（2026-08-19）：**窗口几何声明改为渲染器 swapchain 同步（§6.1）**——新增 `render.PresentTarget.SetOnSwapchainResized` + `platform.SurfacePresenter.OnSurfaceResized` 链路，几何与同尺寸缓冲同批到达合成器；修复 configure 钩子提前声明导致的负 frame extents（取消最大化恢复尺寸被翻成 1468×653）问题；验证：tmp_resize_diag DIAG_STATE=1 DIAG_SIZE=400/800 取消最大化/全屏恢复 configure 恒等于请求尺寸。
- v1.2（2026-08-12）：**S5 落地（主文档 S2 ✅）**——§2.4 Options 表 🔨 全部转 ✅（waylandCreate 全量接线 + SetResizable 锁）；§3.2 状态查询口径实现闭环（IsMinimized 乐观跟踪 + activated 回置、IsVisible 恒 true、configure 真值查询）；§7 S5 ✅。
- v1.1（2026-08-12）：**对齐主文档 v2.1**——CSD ✕ 动作改 EventCloseRequested（§5）；补 WindowEdge↔xdg resize_edge 映射（§2.2）；补事件上报闭环（§3.1）、状态查询口径（§3.2）、Options 全量接线表（§2.4）；分期补 S5（主文档 S2 对齐）。
- v1.0（2026-08-12）：从"去边框"方案推翻重写，对齐 sctk/GTK/gogpu 标准 CSD。

## 9. 需求
一、窗口外观（基础）
设置窗口标题文本
设置窗口应用图标名称（icon name）
支持装饰协商：兼容 SSD 服务端装饰 / CSD 客户端自绘标题栏
CSD 基础实现：提供拖拽移动区域、最小 / 最大化 / 关闭按钮
设置窗口初始宽高
设置窗口最小、最大尺寸约束
二、窗口功能（基础）
2.1 窗口生命周期
创建单个 xdg_toplevel 顶层主窗口
显示激活窗口
隐藏窗口
关闭并销毁窗口
拦截关闭请求，支持自定义关闭逻辑
2.2 基础窗体交互
窗口拖拽移动
窗口边缘拖拽缩放
请求最小化
请求最大化 / 还原
请求全屏 / 退出全屏
2.3 基础系统集成
剪贴板读写
接收外部文件拖拽
三、窗口状态管理（基础）
3.1 客户端可发起请求
请求最大化 / 取消最大化
请求全屏 / 退出全屏
请求最小化
3.2 只读查询合成器推送状态
获取窗口状态：最大化、全屏、最小化、激活
获取窗口当前宽高
判断窗口是否启用装饰、是否可见
3.3 基础事件监听
close：合成器下发关闭请求回调
configure：接收窗口尺寸、状态配置并应用

### §9 落地状态（2026-08-19 · 实现对照，需求原文未改动）

| §9 需求项 | 实现位置 | 状态 |
|---|---|---|
| 设置窗口标题文本 | wayland_winctl_linux.go SetTitle → xdg set_title | ✅ 既有 |
| 设置窗口应用图标名称 | Options.IconName → set_app_id（§6.4） | ✅ 本次落地 |
| 装饰协商（SSD/CSD）| zxdg_decoration configure 监听 + set_mode（§6.4） | ✅ 本次落地（GNOME 42.9 → CSD）|
| CSD 基础（拖拽区/三按钮）| wayland_csd_*（§4/§5）| ✅ 既有 |
| 初始宽高 | waylandCreate + configure（§2.4）| ✅ 既有 |
| 最小/最大尺寸约束 | set_min/max_size（§2.4）| ✅ 既有 |
| 创建 xdg_toplevel 主窗口 | waylandCreate | ✅ 既有 |
| 显示激活窗口 / 隐藏窗口 | Show/Hide/IsVisible + EventHidden（§6.2，销毁+重建真隐藏）| ✅ 本次落地（真 unmap，WAYLAND_DEBUG 线序实测）|
| 关闭并销毁窗口 | EventCloseRequested 拦截 + Close → destroyNative | ✅ 既有 |
| 拖拽移动 / 边缘缩放 | xdg move/resize（§5）| ✅ 既有 |
| 请求最小化/最大化还原/全屏 | set_minimized/set_maximized/unset/fullscreen（§2.2）| ✅ 既有 |
| 剪贴板读写 | wl_data_device 全套（§6.3）| ✅ 本次落地（双窗广播路径需键盘焦点会话实测）|
| 接收外部文件拖拽 | data_device DnD + EventDrop + uri-list（§6.3）| ✅ 本次落地（真拖拽需人工实测）|
| 状态请求（最大化/全屏/最小化）| 3.1 wlController | ✅ 既有 |
| 状态查询（最大化/全屏/最小化/激活/宽高/装饰/可见）| 3.2 configure states + 客户端跟踪 | ✅ 既有 + IsVisible 本次改真实语义 |
| close / configure 事件监听 | wlTopClose/EventCloseRequested + wlTopConfigure（§3）| ✅ 既有 |