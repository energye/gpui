# ENGINE_FRAME_PRESENT_STANDARD.md — 帧节奏与呈现模式真源

> **状态**：已实现（2026-08-19）。块1/2/3 全部落地：按需渲染 + 合成器通知驱动（Wayland wl_surface.frame / X11 XPresent）+ present 无阻塞（Wayland 恒 FifoRelaxed、X11 稳态 Fifo）。验证见 §6；实现细节见 §3–§4。
>
> **背景**：拖拽调整窗口大小时内容与标题栏不同步、窗口越大越明显（内容帧跟不上鼠标）；同时用户要求垂直同步由系统（合成器）保证、客户端不自行算帧率，X11 与 Wayland 两平台行为统一。

---

## 0. 三十秒

- **块1 按需渲染**：没内容要画就一帧都不画（现在 idle 也 60fps 持续渲染）。
- **块2 合成器通知驱动**：帧节奏源从「客户端读 DRM vblank」换成「合成器/显示服务器通知帧已显示」；两平台把硬件节拍抽象封装统一（`FrameNotifier` + 事件 `EventFramePresented`），上层（scheduler / embedder）统一使用。
- **块3 present 无阻塞**：Wayland 恒不阻塞（合成器 vblank 保证不撕裂）；X11 稳态 Fifo 防撕裂、风暴期 Mailbox（沿用现有 `SetVsync`）。

依赖：`ui` → `render` → `gpu` 单向分层不变；本改造不新增 render 公开 API（present 策略用现有 `SetVsync`/present mode 机制组合）。

---

## 1. 现状与问题

### 1.1 现状（2026-08-19 实测/代码核实）

| 项 | 现状 | 证据 |
|---|---|---|
| 帧节奏源 | `x11Host.WaitVSync` / `wlHost.WaitVSync` 都返回 `WaitDRMVBlank()`（客户端读 `/dev/dri` 的 vblank 信号） | ui/platform/x11_linux.go:665、wayland_linux.go:1373 |
| 渲染需求 | scheduler listener 每收到 vblank **stamp + `ScheduleFrame()`** → idle 时也持续渲染 | ui/scheduler/scheduler.go:74-78 |
| present 模式 | 稳态 Fifo（每帧 present 阻塞 ~16.5ms 等 vblank）；风暴期 `SetVsync(false)` 切 Mailbox→FifoRelaxed→Immediate，平静 200ms 回 Fifo | render/present_target.go `SetVsync`、gpu/webgpu/swapchain.go `PresentModeForVsync`（v1.8） |
| 风暴期重绘 | 每帧全量重绘，栅格 30–390ms（峰值 2.9s），帧率 3–15fps；标题栏独立 subsurface 成本近零 → 跟手，内容滞后 | docs/ENGINE_UI_WIDGET_RENDER.md §10（resize拖拽延迟行）、ENGINE_WAYLAND_WINDOW_STANDARD.md §8 v1.8 |

### 1.2 问题

- **P1（浪费）**：idle 时每 vblank 渲染一帧（内容无变化也画），CPU/GPU 空转。来源 = vsync listener 无条件 `ScheduleFrame()`。
- **P2（不跟手）**：风暴期 Fifo 每帧 present 阻塞 ~16.5ms，叠加全量重绘成本，内容帧跟不上鼠标；窗口越大重绘越贵越明显。
- **P3（切换成本）**：Fifo↔Mailbox 运行时切换 = surface 重配 + 全写预算（postResizeFull=3），每次风暴都付。
- **P4（语义）**：客户端自读 DRM vblank 是「客户端侧等硬件信号」，非合成器通知；用户诉求是垂直同步交系统、客户端不算/不等帧率，两平台统一。

### 1.3 对齐基准

- **Wayland 帧节奏**：Gio（`wl_surface.frame` 回调驱动 + EGL swap 关闭 vsync）、Chromium、GTK 的正统做法——present 不阻塞，帧节奏由合成器「帧已显示」通知驱动，idle 不渲染。
- **X11 帧节奏**：Chromium 用 XPresent extension（`XPresentNotifyMSC` → `PresentCompleteNotify`）作为显示完成通知；无 XPresent 时退 DRM vblank。
- **按需渲染**：Flutter VsyncWaiter 语义——vsync 回调只做节拍 stamp，渲染由「需求」（事件/动画）触发。

---

## 2. 目标架构（三块）

### 2.1 块1：按需渲染

**原则：节拍源只 stamp，不制造需求；需求决定渲染；渲染后请求通知。**

- scheduler vsync listener（含 DRM 退路）**不再 `ScheduleFrame()`**，只更新 `lastVSync` 时间戳（帧门禁 `FrameDue()` 依赖新鲜度放行）。
- 渲染需求唯一来源：事件（resize/expose/pointer/key/…）与动画 ticker（`pipeline_app` 现有 `ScheduleFrame()` 调用点）。
- idle（无 pending、无 ticker）：不渲染、不提交、不注册通知 → CPU/GPU 零开销。

### 2.2 块2：合成器通知驱动（两平台统一抽象）

**platform 层统一封装硬件节拍源，上层统一使用：**

```go
// ui/platform/vsync.go（新增）
// FrameNotifier 是合成器「帧已显示」通知抽象：Host 可选实现。
// Wayland：wl_surface.frame 回调；X11：XPresent PresentCompleteNotify；
// 退路：无 FrameNotifier → scheduler 用 DRM vblank（只 stamp）。
type FrameNotifier interface {
    // RequestFrameNotify 请求下一个「帧已显示」通知（渲染提交完成后调用）。
    // 通知到达时 platform 投递 EventFramePresented。
    RequestFrameNotify()
}
func HostFrameNotifier(h Host) FrameNotifier
```

- **统一事件**：`EventFramePresented`（ui/platform/host.go 新增 EventType 值）——合成器确认一帧已显示。embedder 事件循环收到 → `sched.NoteFramePresented()`。
- **统一节拍入口**：`scheduler.NoteFramePresented()`——等价 vsync stamp（只更新 `lastVSync`，不 `ScheduleFrame`；渲染需求仍由事件/ticker 决定，保证按需闭环不空转）。
- **双源互斥**：`ensureVsyncListener` 检测到 Host 实现 `FrameNotifier` 时**不启动** DRM listener（避免双节拍源 + 持续渲染）；无 FrameNotifier（或实现不可用）才走 DRM vblank 退路（只 stamp）。

### 2.3 块3：present 无阻塞（按平台策略）

| 平台 | 稳态 | 风暴期 | 依据 |
|---|---|---|---|
| Wayland | 恒无阻塞（FifoRelaxed 优先，Immediate 次之） | 同左（事件驱动立即渲染，提交不阻塞） | 合成器 vblank 原子换帧，客户端提交不撕裂（Gio `EnableVSync(false)` 同款）；无需 Fifo↔Mailbox 切换 |
| X11 | Fifo（阻塞等 vblank，防直接扫描出撕裂） | Mailbox（现有 `SetVsync(false)`，不阻塞不撕裂），平静回 Fifo | X11 无 Wayland 式换帧保证；`PresentCompleteNotify` 驱动节拍，Fifo 提交点等待仍在预算内 |

**实现**：render 层 present 模式默认值按平台分流（Wayland → 无阻塞；X11 → Fifo）。复用现有 `SetVsync`/`SetPresentModeForce`，不新增公开 API。

---

## 3. 分层改动清单（文件级，已实现）

| 文件 | 改动（已落地） |
|---|---|
| `ui/platform/host.go` | EventType 新增 `EventFramePresented`（追加在 EventHidden 后）+ String() 分支 |
| `ui/platform/vsync.go` | 新增 `FrameNotifier` 接口 + `HostFrameNotifier(h)` helper |
| `ui/platform/wayland_linux.go` | ① `wlSurfaceFrame = 3` opcode 常量 + `ifaceCallback`（`wl_callback_interface`）；② `wlWin` 加 `frameCB atomic.Uintptr`/`frameDone atomic.Bool`/`frameListener [1]uintptr`；③ `wlFrameDone` 回调：`proxyDestroy(cb)` → `frameCB=0` → `frameDone=true` → `WakeUp`；④ `RequestFrameNotify()`（raster 线程）：`frameCB==0` 时 `wl_surface.frame(surface)`（proxyMarshalArrayCtor）+ `proxyAddListener` 挂 frame done；⑤ poll 读 `frameDone` 生成 `EventFramePresented`。**实现修正（vs 计划）**：本机 libwayland 把 `wl_callback_add_listener/destroy` 做成 inline 包装不导出符号（`nm -D` 证实仅 `wl_callback_interface` 数据符号）→ 复用底层 `wl_proxy_add_listener`/`wl_proxy_destroy` |
| `ui/platform/x11_linux.go` | ① x11Create var 块注册 `XPresentQueryExtension`/`XPresentSelectInput`/`XPresentNotifyMSC`；② 窗口创建后查询 XPresent（event_base/error_base），支持则 `XPresentSelectInput(win, 1)`（PresentCompleteNotifyMask）；③ `x11State` 加 `presentBase int`/`presentOK bool`/`xPresentNotifyMSC func`；④ `RequestFrameNotify()`：`XPresentNotifyMSC(dpy, win, 0, 1, 0)`（下一个 vblank 通知一次，仅 presentOK 时）；⑤ drainX `case st.presentBase` 解析 PresentCompleteNotify（window@offset 32 过滤）→ `EventFramePresented` |
| `ui/scheduler/scheduler.go` | ① 新增 `NoteFramePresented()`（只 stamp `lastVSync`，不 ScheduleFrame）；② `ensureVsyncListener`：`HostFrameNotifier(host) != nil` 则不启 DRM listener（单节拍源）；③ listener 循环去掉 `s.ScheduleFrame()`（块1：只 stamp + 错误计数） |
| `ui/embedder/pipeline_app.go` | ① 事件 switch 加 `case platform.EventFramePresented: a.sched.NoteFramePresented()`；② 帧 job `Run` 里 `err == nil`（present 成功）分支：`HostFrameNotifier(a.host).RequestFrameNotify()`（与现有 `FrameSync.NotifyFrameDrawn` 并排） |
| `render/present_target.go` + `gpu/webgpu/swapchain.go` | ① swapchain 新增 `SetPreferFifoRelaxed()`（[FifoRelaxed, Mailbox, Immediate]，不含阻塞 Fifo）；② `PresentTarget` 加 `fixedNoVsync bool`：`ns.Platform == PresentPlatformWayland` → `SetPreferFifoRelaxed()` + `vsyncOn=false` + `SetVsync` no-op（块3：Wayland 恒无阻塞）；X11 保持 `SetPreferVSync()` + 风暴期 SetVsync 切换 |

**明确不改**：`ui/platform/vsync_drm_linux.go`（保留为退路，只改 scheduler 侧调用语义）；CSD 标题栏 subsurface 更新路径（独立即时更新，不进帧节流）。

---

## 4. 帧闭环时序

### 4.1 稳态 idle（零渲染）

```
无事件无动画 → 无 ScheduleFrame → 不渲染 → 无通知注册 → 合成器不发通知 → 完全闲着
```

### 4.2 动画/事件驱动（60fps 节拍）

```
动画 ticker / 事件 → ScheduleFrame → FrameDue（通知或软件间隔开门）→ 渲染一帧
  → present（Wayland 无阻塞 / X11 Fifo）→ RequestFrameNotify（注册下一个显示完成通知）
  → 合成器 vblank 显示 → 「帧已显示」通知（Wayland wl_callback.done / X11 PresentCompleteNotify）
  → EventFramePresented → NoteFramePresented（stamp）→ 有需求则下一帧，无需求收工
```

### 4.3 风暴期（resize 拖拽）

```
EventResize（连续）→ pendingResize=true + ScheduleFrame（帧门禁旁路，现有逻辑）
  → 立即渲染新尺寸 → present 不阻塞 → 提交跟手；平静 200ms 无新 resize → 恢复稳态节拍
```

---

## 5. 测试结果（已实现，全部 PASS）

| 测试 | 位置 | 结果 |
|---|---|---|
| scheduler：listener 不制造需求（块1） | ui/scheduler/scheduler_test.go `TestFramePace_ListenerStampsButDoesNotSchedule` | ✅ PASS：DRM waiter 只 stamp，`Pending()` 不被置位 |
| scheduler：FrameNotifier 互斥（单节拍源） | ui/scheduler/scheduler_test.go `TestFramePace_FrameNotifierSkipsDRMListener` | ✅ PASS：MissedVSync==0、vsync_source="fallback"（DRM listener 未启动） |
| scheduler：通知只开门不制造需求 | ui/scheduler/scheduler_test.go `TestNoteFramePresented_StampsPacing` | ✅ PASS：`NoteFramePresented` 后 `FrameDue()` 开、`Pending()` 关 |
| wayland：frame 通知闭环 | ui/platform/wayland_frame_notify_test.go `TestWaylandFrameNotify` | ✅ PASS（真实合成器）：RequestFrameNotify → 提交 64×64 shm 帧 → `EventFramePresented` 到达 → frameCB 归零可再请求 |
| swapchain：FifoRelaxed 偏好（块3） | gpu/webgpu/s6_8_swapchain_test.go `TestS68_Swapchain_PreferPresentModes` | ✅ PASS：`SetPreferFifoRelaxed` → 首候选 FifoRelaxed 且不含 Fifo |
| 回归 | ui/platform 13 测试文件 + ui/embedder 6 文件 + render present/resize + gpu swapchain | ✅ 全部 PASS（逐个文件跑，禁一次全量）；apidoc exit=0 |
| x11：XPresent 探测 | （无 XPresent 自动化测试；`presentOK=false` 退 DRM 路径由真窗验证覆盖） | ⏳ 待真机 X11 环境确认 |

---

## 6. 验证计划（真窗 + GPU，状态标注）

1. `go run ./tmp_resize_diag`（Wayland）：拖拽 resize 内容跟手；DIAG_STORM=1 风暴 fps 不回退；`WR_RESIZE_DBG=1` 观察帧节拍。⏳ 待用户复测。
2. **idle 零渲染验证**：窗口静止时 CPU/GPU 占用显著下降（对比改造前 idle 60fps 渲染）；`WR_RESIZE_DBG=1` 无多余帧。⏳ 待用户复测。
3. `go run ./examples/ui_wr_r4_composite`（R4 组合窗）：稳态 60fps、fps≈59.9 基线不回归。✅ 二进制重建成功（2026-08-19），运行复测待用户。
4. X11（若可测）：Xwayland 下跑 `tmp_resize_diag`，XPresent 路径生效（`EventFramePresented` 到达），撕裂无回归。⏳ 待 X11 环境。

**已自动验证**（无人工观测部分）：`TestWaylandFrameNotify` 真窗闭环（真实合成器，2026-08-19 PASS）；ui/platform 13 文件 + ui/embedder 6 文件 + render present + gpu swapchain 全 PASS；apidoc exit=0。

---

## 7. 文档同步（已全部完成）

- ✅ `docs/RENDER_API_CATALOG.md`：§3.7 / §7.1 / §8 的 `SetVsync` 描述补充「Wayland 平台 fixedNoVsync 恒无阻塞 = no-op（块3，2026-08-19）」。
- ✅ `docs/ENGINE_UI_WIDGET_RENDER.md` §10 修订表：追加「帧节奏正统化（块1/2/3）」行（版本列 `API目录同步`）。
- ✅ `docs/ENGINE_WAYLAND_WINDOW_STANDARD.md` §8：追加 v1.13 行（合成器通知驱动 + 按需渲染 + Wayland 无阻塞 present）。
- ✅ 本文档 §0 状态「已实现」+ §9 修订表「实现」行。
- ✅ 机器校验：`go run ./scripts/apidoc` 退出码 0（render 公开 API 无增删）。

---

## 8. 风险与退路

| 风险 | 退路 |
|---|---|
| X11 X server 无 XPresent extension | 不实现 `FrameNotifier` → scheduler 走 DRM vblank 按需模式（仍满足块1：只 stamp 不制造需求） |
| Wayland 合成器不响应 frame 回调 | 回调永不到达 → 通知不驱动，但事件/ticker 仍 `ScheduleFrame` + 软件间隔兜底（`FrameDue` 走 animTick），功能不冻结 |
| X11 无阻塞 present 撕裂 | X11 稳态保持 Fifo（本计划默认）；Immediate 仅风暴期经 `SetVsync` 使用 |
| 节拍源双驱（通知 + DRM 同时 stamp） | `ensureVsyncListener` 检测 `FrameNotifier` 后不启动 DRM listener，单源 |
| frame 回调堆积（每帧注册未销毁） | `wlFrameDone` 销毁回调 + `frameCB` 归零；`RequestFrameNotify` 仅在 `frameCB==0` 时注册 |
| X11 sync request 几何未驱动 resize（拖拽冻结，2026-08-19 检查发现） | 已修（2026-08-19）：`atSyncReq` 分支读 l[1]/l[2] → `setSize` + `EventResize`；serial 读 l[3]（原误读 l[2]=height，合成器 counter 永不过 serial → 持续拉伸旧帧） |
| X11 PresentCompleteNotify 解析可能失效（2026-08-19 检查发现，待真机） | Xlib 对 GenericEvent 返回 `XGenericEventCookie`（type=35），需 `XGetEventData` 展开；drainX 直接按普通事件读 `type==presentBase` 大概率不命中。本机无 libXpresent.so 无法验证；真机验证项 |
| X11 NotifyMSC 无在途去重（2026-08-19 检查发现，低危） | 渲染慢时每 present 注册的 NotifyMSC 向 X server 堆积 → 每 vblank 补发通知；`FrameDue` 门禁兜底不会多渲染。对齐 Wayland `frameCB==0` 的去重待做 |
| X11 无 XPresent 实际退软件间隔（2026-08-19 检查发现，文档修正） | `HostFrameNotifier` 只看接口实现不看 `presentOK` → DRM listener 被挡 → 退 16ms 软件间隔（功能可用）。文档原写「退 DRM vblank」，与实际不符，以本行为准 |

---

## 9. 修订表

| 版本 | 说明 |
|------|------|
| 2026-08-19 | 立项：块1/2/3 目标架构 + 分层改动清单 + 测试/验证计划 |
| 2026-08-19（实现） | 全部落地：①`ui/platform`：`FrameNotifier` 接口 + `HostFrameNotifier`（vsync.go）、`EventFramePresented`（host.go）；Wayland `wl_surface.frame`（opcode 3）回调 + `RequestFrameNotify`（wayland_linux.go，`wl_callback_*` 为本机 libwayland inline 包装 → 复用 `wl_proxy_add_listener/destroy`）；X11 XPresent（`XPresentQueryExtension/SelectInput/NotifyMSC`，PresentCompleteNotify 解析，presentOK=false 退 DRM）；②`ui/scheduler`：`NoteFramePresented()`（只 stamp 不制造需求）+ `ensureVsyncListener` 去 `ScheduleFrame`（块1）+ 检测 `FrameNotifier` 不启 DRM listener（单节拍源）；③`ui/embedder`：`EventFramePresented → NoteFramePresented`、present 成功后 `RequestFrameNotify`（与 FrameSync 并排）；④`render`：`fixedNoVsync`（Wayland）→ `SetVsync` no-op、初始 `SetPreferFifoRelaxed()`（swapchain.go 新增，[FifoRelaxed, Mailbox, Immediate] 不含 Fifo）；X11 保持 Fifo + 风暴期 SetVsync。测试：scheduler 3 个新测试（FrameNotifier 互斥 / listener 只 stamp / NoteFramePresented 只开门）+ `TestWaylandFrameNotify`（真窗合成器闭环，真实合成器下 PASS）+ swapchain `SetPreferFifoRelaxed` 断言；ui/platform 13 文件 + ui/embedder 6 文件 + render present + gpu swapchain 全 PASS；apidoc exit=0。真窗验证待用户复测 `go run ./tmp_resize_diag`（块2/块3 端到端：idle 零渲染 + 风暴期跟手 + frame 节拍）。 |
| 2026-08-19（检查+修1） | 全面回归检查：块1/2/3 相关测试全 PASS（scheduler 4 / platform 16 / embedder 6 / webgpu swapchain 5 / render present 8）+ apidoc exit=0 + X11 storm 30s 无错（`Fifo→Immediate` 切换生效）。发现并修复 X11 `_NET_WM_SYNC_REQUEST` 两处缺陷：①serial 误读 l[2]=height（正确 l[3]）→ 合成器 counter 永不过 serial → 拖拽持续拉伸旧帧；②sync request 的 l[1]/l[2] 几何未使用（XWayland/mutter 拖拽不发 ConfigureNotify，仅 sync request）→ 内容冻结。修复：`atSyncReq` 分支读宽高 → `setSize` + `EventResize`（与 ConfigureNotify 分支同范式），serial 改读 l[3]/l[4 拼 64 位。验证：`x11_sync_test.go` 3 个真窗测试 PASS + ui/platform 全量回归 PASS + X11 storm 无回归。其余发现记入 §8：X11 PresentCompleteNotify cookie 解析待真机、NotifyMSC 无在途去重、无 XPresent 实际退软件间隔（文档修正）。与本会话无关的既有失败 3 处（x11_sync 测试原 FAIL 已随修复转绿；swapchain 同尺寸 Resize no-op 断言过严；render f1/filter GPU 读回疑似其他线改动）另记。 |
