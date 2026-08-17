# Wayland 标准窗口（完整 CSD+交互）— 设计真源

> **版本：v1.0** | 日期：2026-08-12  
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
| Decorations | ✅ 现有（CSD / 无框） |
| Min/Max Size | ✅ set_min/max_size（waylandCreate 全量） |
| Resizable=false | ✅ min==max 锁（waylandCreate + SetResizable） |
| Position | ⛔ 协议无客户端定位 |
| Fullscreen | ✅ 创建后 set_fullscreen(NULL→当前输出) |
| Cursor | ✅ enter 时应用 wl_cursor_theme（win.cursor，9 形状映射） |
| Maximized | ✅ 首 commit 前 set_maximized |
| Visible | ⛔ 协议无控制 → 忽略（恒可见） |

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
- `IsVisible()`：xdg 无 unmapped 概念 → 恒 true（无隐藏），`Show/Hide` 返回 ErrUnsupported。
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

## 7. 分期与验收

| 阶段 | 内容 | 验收 |
|---|---|---|
| S1 | 协议层完整（接口表/shm/subcompositor/cursor） | ✅ 已落地 |
| S2 | CSD subsurface + painter（标题栏/边框/按钮/字体） | ✅ 已落地 |
| S3 | 交互（move/resize 8 向/三按钮/双击最大化/焦点态/光标）| ✅ 已落地 |
| S4 | 真窗验收（用户跑 ui_textinput_ime）| ✅ 已验收（窗口完整装饰 + 全行为）|
| S5 | **主文档 S2 对齐**：wlController（Options 全量 §2.4 + IsMinimized/RequestMove/RequestResize）+ 事件上报（§3.1/§3.2 闭环：EventCloseRequested/EventFocus/EventOccluded/PointerEnter/Leave）| ✅ 已落地（main doc S2 ✅；ui_pf_wayland 19 项 PASS + TestWaylandRealWindowControls） |

**回归纪律**：`go test ./ui/platform/` 按文件跑，禁止一次全量；真窗由用户执行。

---

## 8. 修订

- v1.2（2026-08-12）：**S5 落地（主文档 S2 ✅）**——§2.4 Options 表 🔨 全部转 ✅（waylandCreate 全量接线 + SetResizable 锁）；§3.2 状态查询口径实现闭环（IsMinimized 乐观跟踪 + activated 回置、IsVisible 恒 true、configure 真值查询）；§7 S5 ✅。
- v1.1（2026-08-12）：**对齐主文档 v2.1**——CSD ✕ 动作改 EventCloseRequested（§5）；补 WindowEdge↔xdg resize_edge 映射（§2.2）；补事件上报闭环（§3.1）、状态查询口径（§3.2）、Options 全量接线表（§2.4）；分期补 S5（主文档 S2 对齐）。
- v1.0（2026-08-12）：从"去边框"方案推翻重写，对齐 sctk/GTK/gogpu 标准 CSD。