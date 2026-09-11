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
- W6：光标首选 `wp_cursor_shape_manager_v1`（合成器自绘），缺席才回 `wl_cursor_theme`（libwayland-cursor）系统主题。

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

### 2.3 光标（首选 cursor-shape 协议，主题为兜底）
- 有 `zwp_cursor_shape_manager_v1` 时走 `set_shape`（合成器自绘，每次 enter/motion 发一个枚举，无 surface/shm/主题加载）；无才回 `wl_cursor_theme` 路径。
- `wl_cursor_theme_load(NULL, 24, shm)` → theme；`wl_cursor_theme_get_cursor(theme, name)` → cursor image（含 `wl_buffer`）。
- `wl_pointer.set_cursor(serial, surface, hotspot_x, hotspot_y)`（opcode 0，签名 `ouii`；`release`=1）。
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
| Cursor | ✅ 首选 cursor-shape 协议（§2.3），无才回主题（win.cursor，9 形状映射） |
| Maximized | ✅ 首 commit 前 set_maximized |
| Visible | ✅ 创建后隐藏：hideNative（§6.2；false=不可见启动） |

---

## 3. 窗口状态机（sctk 对齐）

```
WindowState: Maximized | Fullscreen | Resizing | Activated | TiledL | TiledR | TiledT | TiledB | Suspended
```

- `wlTopConfigure`：**先累积** states/尺寸 → 记 `w.resized` + `w.csd.setWindowState(...)` → 渲染层读 EventResize；`wlXdgConfigure` 侧 ack_configure（协议要求 configure 必须先 ack）。
- `activated`（focus）→ 标题栏聚焦/失焦底色切换，并上报 `EventFocus{Focused}`（值变化才发）。
- `maximized` → 最大化按钮图标 `□`→`❐`，且 maximized 无边框（标准行为：最大化时隐藏装饰边框）；**内容高 = configure 高 − 标题栏 32px**（`wlTopConfigure` 收缩，GTK4 对齐：工作区顶条让给标题栏，总窗 = 内容 + 装饰 = 工作区，标题栏不消失）。
- `suspended`（9）→ 上报 `EventOccluded{Occluded:true}`（停渲染省电）；解除 → `Occluded:false`。禁止把 suspended 当 minimized 处理。
- 关闭：CSD ✕ 与 xdg close（`wlTopClose`）→ `EventCloseRequested`（可拦截）；surface 实际销毁（`wl_surface.destroy` / `wl_registry.global_remove`）→ `EventClose`。

### 3.1 指针事件上报（wl_pointer → Event）

| 协议事件 | 上报 Event |
|---|---|
| enter | `EventPointer{Pointer: PointerEnter, X, Y}` |
| leave | `EventPointer{Pointer: PointerLeave}`（无坐标） |
| motion | `PointerMove`（现有） |
| button | `PointerDown/Up`（现有） |
| axis | `PointerScroll`（现有；`frame/axis_source/axis_stop/axis_discrete` 现为空实现，高精度滚动相位 S6-P0） |

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

- **标题栏宽 = 内容宽（无外伸）**，位于 (0, -32) 正好贴在内容上方：无左右加宽、无底边接缝线。**窗口四周无轮廓线**（GTK4 Adwaita 对齐：标准窗口没有 1px 边框线，边框 subsurface 全透明、仅作 resize 热区）。
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

**8 区缩放映射（用户要求，装饰可见时）**：标题栏左上角（y<8 且 x<8）→ NW；标题栏右上角（y<8 且 x≥宽−8，按钮区覆盖处由右/下边框上段补角）→ NE；窗口左下角（下边框左 8px / 左边框下 8px）→ SW；窗口右下角 → SE；窗口左边+标题栏左边（左边框全高，含标题栏段，标题栏 x≤4 条）→ W；标题栏上边（y<8）→ N；窗口右边+标题栏右边（右边框全高）→ E；窗口下边（下边框）→ S。左/右边框**全高 = 内容高 + 标题栏 32px**，保证标题栏高度的四角四边都在。

**装饰隐藏时（setVisible(false)，服务端装饰协商）**：内容边缘接管同 8 区映射——标题栏左上角/右上角/上边对应窗口（内容）自己的左上角/右上角/上边，其余边角同映射；内容中间仍归应用。

- 命中判定：`wl_pointer` enter/motion 携带 surface + surface-local 坐标；`button` 事件无坐标 → 用最近 motion 坐标（enter 也记坐标，避免进入即点击时 miss）。
- **enter serial 必须记录**：set_cursor 和 move/resize 都需要合法 serial。
- motion 每次更新光标（enterSerial + hitTest）。

---

## 6. resize 后 CSD 跟随

- `wlTopConfigure` 尺寸变化 → `w.resized=true` → `poll()` 里 `csd.resize(w,h)`：
  - 标题栏 subsurface：宽 = 内容宽（无外伸），高 32，位于 (0, -32)；
  - 左/右边框 subsurface：4×(ch+32)，位于 (-4,-32)/(cw,-32)（含标题栏段，四角四边热区完整）；下边框 (cw+8)×4 位于 (-4,ch)；
  - **只重建 shm buffer（保留 wl_surface/wl_subsurface 对象）**，交互式 resize 不打断 pointer grab；
  - detach 旧 buffer 必须传 `(nil, 0, 0)` 三参（attach 签名 `oii`）。

---

### 6.1 窗口几何（xdg window geometry）声明 — swapchain 同步

- **设计**：客户端几何 = 内容矩形（不含 CSD 装饰），对齐 GTK `gtk_window_set_geometry_hints` 内容矩形语义。`set_window_geometry(gx,gy,cw,ch)` 在**渲染器 swapchain 重配到新尺寸的同一 wire batch** 里发出（几何 → attach 同尺寸缓冲 → commit），保证合成器每笔提交看到「几何 ≤ surface 且几何 == 内容尺寸」。
- **最大化几何（GTK4 对齐，标题栏不消失）**：maximized、非 fullscreen 且 CSD 装饰可见（`visible`）时 `gy = -csdTitleBarHeight`（内容上缘上移 32px 声明）——合成器按几何锚定：surface 整体下移 32px，标题栏（subsurface 于 (0,-32)）占据工作区顶条，内容填满其余，总窗 = 内容 + 装饰 = 工作区；同时 `wlTopConfigure` 把内容高收缩 configure 高 − 32（`ctrl.Size()`/swapchain/EventResize 全走内容口径）。SSD 模式（装饰隐藏）不收缩、gy=0（合成器自绘边框，内容铺满）。恢复/取消全屏时 gy 回 0、尺寸回 configure（DIAG_STATE=1 验证还原精确 400×400）。
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

- v1.18（2026-08-20）：**标题栏中英文字号统一（字体链渲染，GTK/Flutter 对齐）**——v1.17 后标题栏 ASCII 走内嵌位图字体（字形内容仅 5-7px 高）、非 ASCII 走系统字体 13px em（汉字占满 12px），混排时中文比英文大 1.7 倍（实测 'e' 内容 5px / 'C' 7px / '层' 12px），视觉不协调。修复=标题栏文本整体走系统字体链（font fallback）：字体候选按优先级加载为多个 face（全能 CJK 字体自带 Latin，如 Noto Sans CJK → 纯 CJK：WQY/UMing → 纯拉丁兜底：DejaVu），每个 rune 用链上第一个含其字形的 face 栅格化（`GlyphBounds` 判定 fallback），字形缓存 key 改为 (face, rune)；所有字符同一字号同一度量，英文 cap 10px、中文 12px（1.2:1，GTK 标准方块字比例）；统一基线按链首 face 的 metrics 公式垂直居中（Pango 同款：baseline = 中心 − (ascent+descent)/2 + ascent，不用裸 hhea ascent——Noto CJK 13px 下 Ascent=16px 会把整行推到条底）；位图字体降级为「系统无任何字体」时的兜底（英文位图 + '?'）；字形缓存带防御性上限（>2048 条目清空，标题字符集实际远小于此）。验证（2026-08-20）：真窗 `ui_wr_r4_composite` dump 逐字形测量——'C' 内容 10px 高（y 12..21）、'层' 12px 高（y 11..22）、同一基线（descender 对齐）、文字整体 y 11..25 居中（条 32px 中心 16）；新增 `TestCSDSysGlyphLatin`（ASCII 也走系统抗锯齿灰度字形）；`TestDrawTitleTextMixed` 改为全系统字形断言；painter/behavior/hover/font 回归全 PASS。
- v1.17（2026-08-20）：**标题栏非 ASCII 乱码修复（中文等 → '?'）**——现象：Wayland 自绘标题栏（CSD）标题含中文等多语言字符时显示为 '?'（em dash 变成 '-'），如 `ui_wr_r4_composite` 的标题「— 层 Composite Present」实显「- ? Composite Present」。根因：`drawTitleText` 用内嵌位图字体 `csdFont`（仅 95 个可打印 ASCII 字形，6×13），`switch` 的 `default` 分支把一切非 ASCII（除 – — 映射 '-'）回退成 '?'。修复=`ui/platform` 新增 `wayland_csd_font.go`：非 ASCII rune 用系统字体（`golang.org/x/image/font/opentype`，纯 Go 无 cgo；候选 Noto Sans CJK ttc / WQY / AR PL UMing / 用户字体目录 OTF / DejaVu 兜底）栅格化灰度位图，按 rune 缓存（`csdGlyph`），alpha 混合进标题栏 ARGB8888 buffer（GTK parity：CSD 标题支持 UTF-8）；ASCII（含 – — 的 '-' 映射）继续走内嵌位图字体；宽度按 rune 累计（ASCII 7px + CJK ≈13px advance）保证 ASCII+CJK 混排居中；找不到系统字体时回退 '?'（不崩）。字形放置偏移取 `GlyphBounds`（baseline 相对 fixed 坐标）而非 `Glyph` 的 dr 矩形——dr 的整数坐标空间跨字体不稳定（实测 Noto CJK 13px 下 `dr.Min.Y=1013` 与 `Metrics.Ascent=16px` 不匹配）。验证（2026-08-20）：真窗 `ui_wr_r4_composite`（标题「gpui ui_wr_r4_composite — 层 Composite Present」）`GPUI_WL_DUMP_CSD` dump 逐字形解码——'层' 由 13×13 汉字笔画位图（非零 alpha ≥15 像素）替代原 '?' 占位，em dash 保持 '-'，ASCII 部分与位图字体一致；新增单测 `TestCSDSysGlyphCJK`/`TestCSDSysGlyphCache`/`TestBlendCSDGlyph`/`TestDrawTitleTextMixed`；ui/platform 相关回归（painter/behavior/hover/font 各文件）全 PASS，`go build ./ui/...` 绿。
- v1.16（2026-08-20）：**resize 时标题栏文本闪烁 + 位置窜动修复（先填充后提交）**——现象：拖拽调整窗口大小时标题栏文本（标题）闪烁、位置跳动，松手后恢复正常。根因=`wlCSD.resizeSurface` 的 buffer 交换顺序：**先** `attach(NULL)+commit`（desync 立即应用）→ 合成器把该 subsurface 置为无 buffer（标题栏区域变空/显示父表面 → 每步 resize 闪烁一次），**再** destroy 旧 buffer/pool/mmap、重建新 buffer，最后 `paint()` 才填充并提交 → 文本位置随"消失→恢复"跳动。修复=**先创建新 buffer → 立即填充正确内容（新增 `fillSurface`：top 画标题栏、border 清透明）→ set position → attach+commit（desync 立即生效且内容正确，无空窗/垃圾帧）→ 最后销毁旧 buffer/pool**；`createBufferWith` 失败时销毁已分配部分并恢复旧 buffer（仍 attach 在 surface 上继续显示）。验证（2026-08-20）：DIAG_STORM 风暴 `GPUI_WL_DUMP_CSD` 标题栏 dump 序列（400/760/1200/1868 宽）每帧文本位置正确（400 窄窗左对齐 x=12、760+ 居中 285/505/839，与 `(w-textW)/2` 计算一致）、无空白/垃圾帧、背景色正确；风暴 fps=60.3 无回归；新增白盒测试 `TestWaylandCSDResizeFillsBeforeCommit`（resize 后新 buffer 已含标题字形、旧 buffer 存活到提交后）；ui/platform 18 测试文件全 PASS。
- v1.15（2026-08-20）：**左下/右下角 resize 光标显示成右上/左上样式修复**——复现：窗口四角拖拽时左下角、右下角光标与左上角、右上角相同（用户报告"左下右下样式和左上右上一样"）。根因：v1.12 的回退候选只补了 `top_left_corner`/`top_right_corner` 两个**上**角经典名；经典主题（本机 default→DMZ-White，无 `nwse/nesw-resize` 现代名；Adwaita 亦无）下四角回退为：左上=top_left_corner、右上=top_right_corner、**左下=top_right_corner、右下=top_left_corner**——四个角只有两种图标，且左下/右下显示的是"头在右上/左上"的角图标（DMZ-White 的 classic 角名按"头在角"绘制：top_left 头在左上、top_right 头在右上、bottom_left 头在左下、bottom_right 头在右下）。修复=`cursorNamesForEdge` 按角返回完整候选：现代名（nwse/nesw-resize）→ **本角经典名**（top_left/top_right/bottom_left/bottom_right_corner）→ 同向对角经典名（Adwaita 缺 bottom_*_corner 时的最后回退）；`resizeBottomLeft`/`resizeBottomRight` 不再与上角共用候选。验证（2026-08-20）：本机 DMZ-White 实测四角分别解析 top_left/top_right/bottom_left/bottom_right_corner（ASCII 渲染确认四个角字形头位置各异、方向正确）；TestWaylandCSDCursorApply/Hotspot 断言更新（右下期望 bottom_right_corner，左下期望 bottom_left_corner 优先）；TestWaylandCSDCursorNamesResolve 候选表按角更新；ui/platform 18 测试文件全 PASS。
- v1.14（2026-08-20）：**最大化窗口切走/切回底部少 32px（=标题栏高）修复**——复现：`tmp_resize_diag` DIAG_STATE 最大化后经 XWayland 焦点窗口（自写 `xactivate` 工具，`_NET_ACTIVE_WINDOW` + `XSetInputFocus`）切走再切回，`GPUI_WL_TRACE_CFG=1` 显示 mutter 在激活/失焦翻转时交替发两种 configure 高度：**工作区全高 1053** 与 **内容高 1021**（后者是客户端声明的 window geometry 生效后的尺寸）。旧 `wlTopConfigure` 对任何 maximized configure 都无条件 `hi -= csdTitleBarHeight` → 内容高 configure（1021）再减一次 → 989 → 窗口底部少 32px（切走/切回各闪一次错误尺寸，最终尺寸取决于最后一次 configure 是哪一种）。修复=`wlWin.maxContentH` 锚定：配置高度 **高于锚点** = 工作区全高 configure（减 32 后更新锚点）；**等于锚点** = 已是内容高（不减，dedup 保持尺寸）；宽度变化（显示器/工作区切换）与退出最大化态重置锚点。验证（GPU NVIDIA Vulkan，2026-08-20）：修复后切走/切回 configure 1021 不再产生 989 EventResize（日志 0 次错误尺寸）、初始最大化 1053→1021 正确、Unmaximize 400×400 / Fullscreen 1920×1080 / Minimize 全路径无回归；新增白盒单测 `TestWlTopConfigure_MaximizedContentHeightAnchor`（含工作区切换重置）/`_MaximizeRestoreNoContentHeight`/`_MaximizedFullscreenNoShrink`；ui/platform 18 测试文件全 PASS。
- v1.14（2026-08-26）：**帧节拍修订（修3，详见 `docs/ENGINE_FRAME_PRESENT_STANDARD.md` §9）**——①`ui/platform/vsync.go` 新增可选接口 `NotifierAvailability`：实现了 FrameNotifier 但能力上报 false 的 host 视为 nil，no-op notifier 不再误杀 DRM vblank 回退；②scheduler 显示周期学习：DRM vblank 打点间隔 EMA 收敛为真实刷新周期（本机实测 16.70ms = xrandr 59.88Hz），软件边界从硬编码 animTick=16ms 改为学习值——消除 UI 抢跑导致的周期性跳帧（wayland 相邻帧抖动 2.41→0.48ms）；③Wayland present 从 FifoRelaxed 恒无阻塞改为 Fifo 排队偏好（提交节奏受刷新约束），SetVsync 两平台均生效。v1.13 的块2 compositor 驱动设计经真窗实测修订：done 通知在 UI→raster 异步管线上作节拍门控会锁死半帧率，改由 DRM stamp 驱动（频率对齐）。验证：wayland 真窗 57.7fps hitch=0、x11 无回归、五包测试全绿、apidoc exit=0。
- v1.13（2026-08-19）：**帧节奏正统化（块1/2/3，详见 `docs/ENGINE_FRAME_PRESENT_STANDARD.md`）**——①**按需渲染**：vsync listener 不再 `ScheduleFrame`（只 stamp 节拍时间戳），idle 零渲染零提交零通知（此前每 vblank 持续渲染一帧空画面）；②**合成器通知驱动**：帧节奏源从「客户端自读 DRM vblank」换为「显示服务器帧已显示通知」——Wayland 接 `wl_surface.frame` 回调（opcode 3，`wl_callback` 经 `wl_proxy_marshal_array_constructor_versioned` 创建、`wl_proxy_add_listener/destroy` 挂接——本机 libwayland 将 `wl_callback_*` 做成 inline 包装不导出符号），`RequestFrameNotify()` 于每帧 present 成功后请求，回调 done → `EventFramePresented` → `scheduler.NoteFramePresented()`（只 stamp）；scheduler 检测 `FrameNotifier` 后不启 DRM listener（单节拍源）；③**Wayland present 恒无阻塞**：`PresentTarget.fixedNoVsync`（Wayland）→ `SetVsync` 为 no-op、初始 `SetPreferFifoRelaxed()`（FifoRelaxed→Mailbox→Immediate，不含阻塞 Fifo）——合成器 vblank 换帧不撕裂，客户端排队等 vblank 是白等（Gio `EnableVSync(false)` 同款）；X11 稳态保持 Fifo 防撕裂 + 风暴期 SetVsync 切换。验证：`TestWaylandFrameNotify`（真窗合成器闭环：RequestFrameNotify → 提交 64×64 shm 帧 → EventFramePresented 到达 → 回调槽归零可再请求）PASS；scheduler 新增 FrameNotifier 互斥 / listener 只 stamp / NoteFramePresented 只开门三测试 PASS；ui/platform 13 测试文件 + ui/embedder 6 文件 + render present + gpu swapchain 全 PASS；apidoc exit=0。真窗复测（用户执行 `go run ./tmp_resize_diag`）：idle 零渲染、风暴期拖拽跟手待确认。
- v1.12（2026-08-19）：**四角光标改标准双箭头（nwse/nesw-resize 带回退）**——功能2 复测：四角显示的箭头"不是双箭头、不认得"。根因：GNOME 会话把 `XCURSOR_THEME=Yaru` 导进环境，应用经 `wl_cursor_theme_load(NULL,...)` 实际加载 **Yaru**；而 **Yaru 的 `top_left_corner`/`top_right_corner` 是单箭头**（顶部一个尖、尾端光杆），标准对角双箭头是 `nwse-resize`/`nesw-resize`（两端都有尖）——字形已逐像素 ASCII 渲染证实。之前测试环境无 XCURSOR_THEME → default→DMZ-White，其 top_left_corner 本身是双箭头，所以未暴露。修复=`cursorNameForEdge` 改为返回候选列表（角=先 `nwse-resize`/`nesw-resize`，主题缺失时回退 `top_left_corner`/`top_right_corner`——DMZ-White/Adwaita 无现代名，但其老名即双箭头），`setCursor` 按序尝试到成功、去重覆盖全部候选。验证：`TestWaylandCSDCursorHotspot` 增加角断言（curName ∈ {nwse-resize, top_left_corner} 且热点非零）；`TestWaylandCSDCursorNamesResolve` 改为逐边候选断言（每边至少一个可解析，防"主名缺失且无回退"的静默卡光标）；8 wayland 测试文件 PASS。真机复测待确认。
- v1.11（2026-08-19）：**按钮间 hover 跟随 + 右上角缩放握把**——① 功能1 按钮之间移入无反馈：`onHover` 重绘条件用「任一按钮 hover 是否变化」（any-vs-any）比较，最小化→最大化移动时两个状态都是 true → 不重绘 → 高亮卡在旧按钮（从窗外移入 prev=false 才触发，与现象完全吻合）。修复=逐按钮比较（Close/Max/Min 各自新旧状态），任一翻转即重绘。② 功能2 四角光标样式不对：唯一坏角是**右上角**——顶部表面 `hitTest` 按钮判定（x ≥ w-44）先于角握把（x ≥ w-8），右上角被关闭按钮盖死 → 永远显示默认箭头而非对角缩放箭头（其余三角正常；角名 `top_left_corner`/`top_right_corner` 经应用同路径实测可解析、字形为标准对角双箭头、热点非零）。修复=角握把在顶部条优先于按钮（GTK4：标题栏顶部条是缩放握把，按钮在其下；最大化/全屏/锁定仍禁用）。验证：新增 `TestWaylandCSDHoverBetweenButtonsMotion`（motion 路径 min→max→close 像素级验证高亮跟随）与 `TestWaylandCSDTopRightCornerResizeHit`（角=resizeTopRight、握把下方=Close、最大化=Close、左上角=resizeTopLeft）PASS；`TestWaylandCSDCursorNamesResolve` 断言五个光标名（含两角名）在应用加载的主题可解析（DMZ-White 缺 nwse/nesw-resize，故角名必须保持老名字）。真机复测待确认。
- v1.10（2026-08-19）：**光标崩溃修复（wl_cursor_image_get_buffer）**——v1.9 后真机跑 `tmp_resize_diag` 首次鼠标移入即崩：`wl_display@1: error 1: invalid arguments for wl_surface@58.attach`。根因：本机 libwayland 1.20（Ubuntu 补丁版）的 `wl_cursor_image` **只有 20 字节（width/height/hotspot_x/hotspot_y/delay），buffer 字段已私有化**（实际结构 theme 指针@0x18、buffer@0x20，惰性创建）；旧 `wlCursorImageC` 在 offset 20 假定 `Buffer uintptr`，读到**堆垃圾**当对象 ID 发 `wl_surface.attach` → 服务端判参数非法、杀连接。修复：`wlCursorLib` 注册 `wl_cursor_image_get_buffer`（nm 确认 1.20-1ubuntu0.1 存在该符号），`applyCursorImage` 用它取 `wl_buffer` 代理（随主题 shm 惰性创建），`wlCursorImageC` 去掉 Buffer 字段。验证：新增断言——`TestWaylandCSDCursorHotspot` 校验附加 buffer 的 wire id 为小奇数（对照已知可用 CSD shm buffer：同 interface 指针@+0、id@+16=13/35）；wm 全 wayland 测试 8 文件 PASS。真机复测（用户执行 `go run ./tmp_resize_diag`）：光标可见性、按钮直接移入反馈、无崩溃待确认。
- v1.9（2026-08-19）：**光标不可见根因修复（缺 damage + 零热点）**——① 功能1 鼠标移入窗口消失、无对应样式：上一版（v1.8①）修了「set_cursor(NULL) 隐藏指针」，但光标仍不可见。根因（mutter 42.9 `meta_wayland_cursor_surface_apply_state`）：**只有 commit 携带 damage（或首次 attach）时才刷新光标 sprite 纹理**；`set_hotspot` 也只在热点值变化时刷新。我们发的 `attach+commit` 无 damage、`set_cursor` 热点恒 (0,0) → sprite 纹理从未建立 → 光标渲染为空（GTK/Chromium 都发 damage + 真热点）。修复=`applyCursorImage` 每次 commit 前发全表面 `wl_surface.damage(0,0,w,h)` + `setCursor` 传 `wl_cursor_image` 真热点（新增 `cursorHotX/cursorHotY` 字段）。「拖拽位置按下出现」= 合成器 grab 自带光标，与客户端无关。② 功能2 按钮直接移入无反馈：与本问题同源——指针不可见无法瞄准按钮；光标可见后待复测确认。验证：新增 `TestWaylandCSDCursorHotspot`（resize 热区 setCursor 后 `curName=="sb_h_double_arrow"` 且热点非零）PASS；`ui_wr_r4_composite` 真窗重建于 2026-08-19 12:40。真机复测（用户执行 `go run ./tmp_resize_diag`）：装饰无、问题1/2 待确认。
- v1.8（2026-08-19）：**鼠标消失修复 + 极小窗崩溃修复 + 风暴期内容跟手（SetVsync）**——① **功能1 鼠标移入窗口消失**：根因=mutter 对 `set_cursor(NULL)`（空 surface）语义是**隐藏指针**，旧 `setCursor` 的「恢复默认」路径发送空 surface → 移入窗口即隐身。修复=`setCursor` 恢复默认显式重放主题 `left_ptr`（不再发 NULL；首次进入/离开窗口、内容 enter 全覆盖），resize 热区与 app 光标路径不变。② **功能4 小于 1px 崩溃**（`EventResize 8x1` + `panic: index out of range [-120]`）：根因=paintTitleBar 在窗口 < 3×按钮宽（132px）时右对齐按钮左缘为负（w=8 → minX=−124），`putPxA` 无负坐标守卫 → Go 切片负索引 panic。修复=`putPxA` 守卫 x<0/y<0/越界（所有像素写入的公共路径，putPx 委托它）+ `paintTitleBar` 在 `w < csdButtonW*3` 时跳过按钮绘制（热区命中不受影响）。③ **功能3 内容不跟手（Wayland/XWayland 同症状）**：帧门禁旁路（v1.7④）只解决「vsync 门卡帧」，Fifo present 仍每帧阻塞 ~16.5ms vblank → 内容帧被钉 60fps、标题栏（独立 subsurface 随 configure 即时更新）平滑，观感滞后。修复=`render.PresentTarget.SetVsync`（记录式，raster 线程 present 边界生效）：风暴首帧起切 **Mailbox/Immediate**（`gpu/webgpu` `PresentModeForVsync`/`SetPresentModeForce` 按 caps 快照选模式），平静 200ms 无 resize 回 Fifo。验证：cursor 测试更新为期望 `left_ptr`（TestWaylandCSDCursorApply/TestWaylandCSDHoverEnter PASS）；新增窄窗绘制回归测试（TestWaylandCSDNarrowPaint/TestWaylandCSDNarrowButtonsNotDrawn PASS，1/2/7/8/9/31/63/100/131/132 宽全不 panic）；S6.8 `TestS68_Swapchain_PresentModeForVsync` PASS。真机（GPU NVIDIA Vulkan，2026-08-19）：`tmp_tinydiag2` 直接开窗 8×1/1×1/4×4/16×1 全程无 panic EXIT=0；X11 风暴（DIAG_STORM=1）WR_RESIZE_DBG 证实 `DBG vsync Fifo -> Immediate`（风暴起）→ `Immediate -> Fifo`（平静 200ms 后），风暴 fps 11.4–23.9 与基线持平零回归、852 presents 无错；Wayland DIAG_STATE=1 Maximize→1868×1021、Unmaximize→400×400 精确。Wayland 真窗拖拽（含拖到 8×1）需人工复测确认。
- v1.7（2026-08-19）：**去轮廓线 + resize 光标修复 + 内容跟手 + 标题栏去抖**——① 删除窗口四周 1px 轮廓线（标题栏顶边、边框行列全去掉，边框 subsurface 全透明仅作热区；GTK4 Adwaita 对齐，标准窗口无线）；② 修复 resize 光标从不切换：`ensureCursorSurface` 无调用点（死代码）导致 `applyCursorImage` 恒 false，`setCursor` 惰性初始化 cursor surface + 主题（TestWaylandCSDCursorApply 真窗验证）；③ onHover 只在按钮 hover 状态实际变化时重绘标题栏（拖拽高频 motion 不再每事件提交标题栏 → 消除文本闪动）；④ embedder 帧门在 `pendingResize` 时绕过 vsync 门控立即渲染（CSD 标题栏同步跟指针，内容不再落后一个 vsync 帧——"标题栏宽与内容宽不同步"修复）。验证：wayland_csd_behavior 7 项 PASS（新增 TestWaylandCSDCursorApply）+ hover/features/interact/window 全过；tmp_tinydiag 程序化 SetSize 1×1 无崩溃（功能 4 待用户真窗复测）。
- v1.6（2026-08-19）：**装饰贴合 + 最大化标题栏 + 8 区缩放修复**——标题栏改内容宽（去 cw+8 外伸、去底边接缝轮廓行，轮廓只留顶边一行 + 边框外表列/外行）；左/右边框全高含标题栏段（四角四边热区完整）；最大化时内容高 −32 + 几何 gy=−32（标题栏占工作区顶条不消失，GTK4 对齐；还原精确 400×400）；装饰隐藏时内容边缘接管 8 区缩放（标题栏角/边映射窗口对应边角）。验证：wayland_csd_behavior 6 项 PASS（新增 HiddenChromeResizeZones）+ features/interact/window/keyboard 全过；tmp_resize_diag DIAG_STATE=1 EXIT=0（Maximize→1868×1021、Unmaximize→400×400 精确、Fullscreen→1920×1080）+ CSD 转储 400/1868/1920×32 全对（无轮廓行/列）。
- v1.5（2026-08-19）：**真隐藏（§6.2 重写）**——隐藏从「停帧+chrome 隐藏（wgpu WSI 不能外部 attach(NULL)）」升级为「销毁+重建」：embedder 收 EventHidden 后停帧 → 等 raster 空闲 → `PresentTarget.Close()`（先释放 wgpu WSI surface）→ `ApplyHiddenDetach()` 销毁 surface 栈（真 unmap，GTK4 同款）；Show 在泵线程外异步重建代理栈（不 dispatch），泵 ack 首个新 configure 后映射，poll 补发 EventHidden{false}，embedder 以新 wl_surface 重开 PresentTarget。抽出 `createSurfaceStack/applySurfaceConfig/destroySurfaceStack`；修复 listener 回调初始化在重建路径丢失（libwayland abort）。验证：wayland_features TestWaylandHideShow（detach 后 NativeSurface=0 → Show 后新 surface + 重映射）+ diag DIAG_FEATURES=1 EXIT=0 + WAYLAND_DEBUG 线序（隐藏点 `wl_surface@5.destroy()`，Show 重建 `wl_surface@56`/`xdg_toplevel@61` + title/app_id/min-max 重放）。
- v1.4（2026-08-19）：**§9 标准窗口需求落地**——§6.2 隐藏/恢复（Show/Hide/IsVisible/EventHidden + wgpu WSI attach(NULL) 限制）；§6.3 剪贴板读写 + 外部文件拖拽（wl_data_device 全套，offer/source 各自 destroy opcode、set_selection 焦点规则）；§6.4 图标名 set_app_id + zxdg_decoration 装饰协商；§2.4 Options 表 Visible/IconName 转 ✅、§3.2 IsVisible 改客户端跟踪；§7 加 S6。验证：wayland_features_linux_test（真窗 4 PASS + 1 预期 SKIP，8 连跑无崩溃）+ tmp_resize_diag DIAG_FEATURES=1 exit 0；并发安全：wlHost.destroying 防非事件线程 Close 的 SIGSEGV。
- v1.3（2026-08-19）：**窗口几何声明改为渲染器 swapchain 同步（§6.1）**——新增 `render.PresentTarget.SetOnSwapchainResized` + `platform.SurfacePresenter.OnSurfaceResized` 链路，几何与同尺寸缓冲同批到达合成器；修复 configure 钩子提前声明导致的负 frame extents（取消最大化恢复尺寸被翻成 1468×653）问题；验证：tmp_resize_diag DIAG_STATE=1 DIAG_SIZE=400/800 取消最大化/全屏恢复 configure 恒等于请求尺寸。
- v1.2（2026-08-12）：**S5 落地（主文档 S2 ✅）**——§2.4 Options 表 🔨 全部转 ✅（waylandCreate 全量接线 + SetResizable 锁）；§3.2 状态查询口径实现闭环（IsMinimized 乐观跟踪 + activated 回置、IsVisible 当时恒 true【已过期：v1.4 起改客户端跟踪真隐藏，见 §3.2 现行】、configure 真值查询）；§7 S5 ✅。
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