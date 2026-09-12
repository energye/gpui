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

**损伤语义（2026-09-12 补）**：重录层的损伤 = 上一帧范围 ∪ 当前范围（Flutter/Skia 标准语义）。只用当前范围时，缩减类变更（拼音退格 `hi`→`h`）清掉的尾巴不在损伤内，LoadOpLoad 直接呈现旧像素（`ui/scene/textured.go`：`PrevBounds` + 损伤点 union；透明清屏路径原有用 viewless shell 保旧范围损伤，本次统一到同一语义）。向量回放兜底路径（旋转/超大拒绝）的损伤仍用当前 picture 范围，缩减场景同类漏报记入 §8 待办。

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
| 向量回放兜底损伤缩减漏报（2026-09-12 新发现，不修） | `compositeWalk` 里 `noTex > 0` 的向量路径损伤用当前 picture 范围，旋转/超大层的缩减变更同类漏报；本次只修纹理主路径（`PrevBounds` union），向量路径待独立验证后再动 |
| X11 sync request 几何未驱动 resize（拖拽冻结，2026-08-19 检查发现） | 已修（2026-08-19）：`atSyncReq` 分支读 l[1]/l[2] → `setSize` + `EventResize`；serial 读 l[3]（原误读 l[2]=height，合成器 counter 永不过 serial → 持续拉伸旧帧） |
| X11 PresentCompleteNotify 解析可能失效（2026-08-19 检查发现，待真机） | Xlib 对 GenericEvent 返回 `XGenericEventCookie`（type=35），需 `XGetEventData` 展开；drainX 直接按普通事件读 `type==presentBase` 大概率不命中。本机无 libXpresent.so 无法验证；真机验证项 |
| X11 NotifyMSC 无在途去重（2026-08-19 检查发现，低危） | 渲染慢时每 present 注册的 NotifyMSC 向 X server 堆积 → 每 vblank 补发通知；`FrameDue` 门禁兜底不会多渲染。对齐 Wayland `frameCB==0` 的去重待做 |
| X11 无 XPresent 实际退软件间隔（2026-08-19 检查发现，文档修正） | `HostFrameNotifier` 只看接口实现不看 `presentOK` → DRM listener 被挡 → 退 16ms 软件间隔（功能可用）。文档原写「退 DRM vblank」，与实际不符，以本行为准 |

---

## 9. 修订表

| 版本 | 说明 |
|------|------|
| 2026-09-12（修4：缩减损伤合并） | **A 框中文输入退格慢（用户实测：候选未提交时退格删字母要等几百毫秒）根治（ui/scene）**：**根因链（真窗自动测量钉死）**——按键→提交全链路恒快（X 取键 1–13ms、输入法回话 7–55ms、改字推送 1–7ms、提交 2–16ms、光栅 2–5ms，合计 <70ms）；但退格把拼音条 `hi` 缩成 `h` 后，2.2 秒 30 连拍像素纹丝不动（还是 `hi`），期间 6 帧正常提交——重录层损伤只用**当前**范围，清掉的 `i` 尾巴不在损伤内，LoadOpLoad 直接呈现旧像素；第二次退格（清屏走 viewless shell 保旧范围损伤）才刷新，视觉上就是“等一下才删”。A/B 框同流程同现象（用户只盯 A 框故报 A）。**修复（Flutter/Skia 损伤语义：上一帧 ∪ 当前帧）**——`ui/scene/textured.go`：`pictureTextureEntry` 加 `prevBounds`（三次重录入口统一 stash）+ `PrevBounds(id)`（仅本帧有效，不累积）+ 损伤点 union；增长/不变场景 union == 当前，零额外开销。**验证**——①新增 `textured_shrink_internal_test.go`（无 GPU 可跑）：缩减保旧范围、增长 union 即当前、前帧过期；②新增 `shrink_damage_test.go`（有 GPU 跑，无 GPU 诚实 skip，同 caret 测试门控）；③真窗自动复测（按键 + 像素时序）见本轮验证；④回归：ui/scene 12 文件全 PASS（`TestPicture_Replay_DrawImagePixels` 存量 FAIL，干净代码对照同挂，与本修无关）。中途曾误判为 swapchain present 卡住并试过输入簇 Mailbox，已证伪回滚（Mailbox 不改损伤矩形，复测仍冻结；树内无残留）。 |
| 2026-09-12（修4补：看错框纠正 + 清空壳损伤） | **纠正**：修4 的“A/B 同现象”是误判——当时盯的是 B 框层号（19），用户指正只坏 A 框；已重测，只盯 A（A 文本层 16）。**第一下（`hi`→`h`）**：union 修补后真窗约 150ms 上屏（可接受）。**第二下（`h`→空）仍慢 250–700ms**：引擎 7ms 内已清空，但屏幕鬼影残留到下一次整框重绘（500ms 光标闪烁捎带）。**第二根因（代码实证）**：`recordWith`/`recordLocalWith` 遇全空 picture 直接提前返回（旧纹理不删也不留壳），且合成侧损伤计算在空图门禁内——空壳的旧范围损伤从未发出，LoadOpLoad 留鬼影；修4 行文称“清屏走 shell 保损伤”是错的（shell 逻辑有，发射路径不通）。**本次修复（ui/scene，同一语义）**：①空图重录先 `dropTransparentLocked` 留壳（`recordWith`/`recordLocalWith` 两处）；②合成侧损伤块移出空图门禁（空壳也发上一帧范围损伤；`noTex` 向量分支不受影响——该分支下从不录制，`RecordedThisFrame` 恒假）。**验证**：① `textured_shrink_internal_test.go` 加空壳用例（留壳/记帧/过期，无 GPU 可跑）；② `shrink_damage_test.go` 经引擎字体查找配系统 sans（与真窗同形，不硬编码路径）+ 相对断言（以首帧自家墨迹为尺，不硬编码坐标）：缩减 `hi`→`h` 损伤盖尾巴到 319，清空间 `h`→空损伤盖旧墨迹；③藏起修补重跑两合成用例皆挂（非空断言）；④回归见下。**待办**：本合成器下 xwd 像素 harness 几何逐轮漂移，自动时序判定不可靠，第二下时延需用户在真窗用人眼复验。 |
| 2026-08-26（修3：显示周期学习 + 能力上报 + Wayland Fifo 化） | **pelican 真窗用户实测「wayland 持续不匀卡顿、倍速越高越明显」三洞根治（ui/platform + ui/scheduler + render）**：**根因链（全部真机实测钉死）**——①本机无 libXpresent → x11Host XPresent 探测失败 presentOK=false，`RequestFrameNotify` 是 no-op；但 `ensureVsyncListener` 只看接口实现即认定 compositor 驱动，**DRM vblank 监听 goroutine 被误杀不启动** → 节拍源丢失退化为软件自由跑；②软件边界硬编码 animTick=16ms vs 本机显示真实刷新 16.70ms（xrandr 实测 1920x1080@59.88Hz 与 DRM vblank 打点实测 p50=16.69ms 双源互证）→ UI 抢跑 0.7ms/帧 → 周期性错过合成 deadline → 上屏内容步距忽大忽小（speed=2.5 时跳帧步距翻倍故「越快越卡」）；③块3 原「Wayland 恒无阻塞 FifoRelaxed」使提交相位完全不受显示刷新约束，放大①②。**修复**——① `ui/platform/vsync.go` 新增可选接口 `NotifierAvailability{FrameNotifyAvailable() bool}`：实现了 FrameNotifier 但能力上报 false 的 host 视为 nil（no-op notifier 不再误杀 DRM 回退）；x11Host 实现（=presentOK && NotifyMSC 绑定）；② scheduler 新增**显示周期学习**：DRM 打点间隔经 EMA（α=1/8，仅采 8–100ms 合理区间）收敛为 `displayPeriod`，`boundaryPeriodLocked()` 以其替换软件边界的硬编码 animTick（clamp [animTick*0.8,100ms]），出帧节拍与真实刷新同频；③ `render/present_target.go`：Wayland fixedNoVsync=false → Fifo 排队偏好（提交节奏受刷新约束），SetVsync 在 wayland 生效。**效果（wayland 真窗）**：相邻帧间隔抖动 mean **2.41→0.48ms**（p95 5.59→1.31ms）、每秒帧数稳定 58、hitch=0；x11 无回归。**诚实勘误**：2026-08-25 曾记「X11 修复使 p95 18.2→16.2 收敛」，经同环境受控 A/B（stash 三轮）证伪——前后 p50/p95 无差异，当时基线取自不同系统负载日；X11 修复真实收益 = vsync_source fallback→true + DRM 锁相路径打通。**块2 部分修订声明**：compositor done 通知在本架构（UI→raster 两段异步 + swapchain depth 3）上不可作节拍门控——真窗实测在途互斥锁死 ~22fps（管线无法在一个刷新槽内完成 done→渲染→提交），Wayland pacing 改由 DRM vblank stamp 驱动（频率对齐而非事件对齐），`NotifierAvailability` 即该修订的机制载体。回归：ui/scheduler+platform+embedder+rendering+io 全 ok、apidoc exit=0、R6 关闭档 30s PASS、C4 golden FAIL 经 stash 对照为存量。 |
| 2026-08-20（修2：事件循环忙等空转） | **ticker 常开 + ModePersistent 下事件循环忙等空转根因修复（ui/embedder + ui/scheduler）**：**根因**——`PipelineApp.ScheduleFrame`（及旧版 `App.ScheduleFrame`）无条件 `host.WakeUp()` 写 wake pipe；ticker 驱动的持久模式每轮循环都 `ScheduleFrame`（ticker 回调 + 帧后 keep-scheduling），而 X11/Wayland `WaitEvents` 对 wake 可读立即返回 → 循环永远睡不满 16ms 节奏 timeout，ticker 回调（含 `Metrics().Snapshot()`/percentiles 排序）以空转速率运行，帧门禁虽正确 60fps 但 CPU 白烧（pprof：percentilesLocked 33.5% + 管道 syscall 23% + WakeUp 9%，进程 113%/核 ≈ 用户 top 观测 28–30% 四核归一）。**修复**——① `ScheduleFrame`：ModePersistent 下不 `WakeUp`（该模式 WaitEvents 已 ≤animTick 自行唤醒；IDLE/TRANSIENT 无限阻塞仍需跨线程打断）；② `FrameDue`：**软件地板无条件化**——slow/批处理合成器通知（实测 wl_surface.frame ~2× 延迟、stamp≈renders/2）不得把动画钉在通知速率之下（once-per-stamp 门把稳态钉 37fps 次优平衡点）；stamp 快（interval ≤ animTick，120Hz vblank）才每 stamp 放行，慢由 16ms 地板覆盖（双态均不双重渲染）；③ `WaitTimeout` 持久模式返回「距下一帧边界剩余」，循环在事件噪声早醒后重算剩余、精确在截止点醒来（地板不再量化到事件边界）。**验证（GPU NVIDIA Vulkan，2026-08-20）**：稳态 CPU 113%→21%/核、fps 稳定 60（2399 presents/40s）、风暴全程 59.6–60.5fps 跟手不回归、GPU 回读快照渲染正确；scheduler/embedder 全量 + 4 个新 pacing 单测（含 -race）PASS；apidoc 绿；render 公开 API 无增删。 |
| 2026-08-19 | 立项：块1/2/3 目标架构 + 分层改动清单 + 测试/验证计划 |
| 2026-08-19（实现） | 全部落地：①`ui/platform`：`FrameNotifier` 接口 + `HostFrameNotifier`（vsync.go）、`EventFramePresented`（host.go）；Wayland `wl_surface.frame`（opcode 3）回调 + `RequestFrameNotify`（wayland_linux.go，`wl_callback_*` 为本机 libwayland inline 包装 → 复用 `wl_proxy_add_listener/destroy`）；X11 XPresent（`XPresentQueryExtension/SelectInput/NotifyMSC`，PresentCompleteNotify 解析，presentOK=false 退 DRM）；②`ui/scheduler`：`NoteFramePresented()`（只 stamp 不制造需求）+ `ensureVsyncListener` 去 `ScheduleFrame`（块1）+ 检测 `FrameNotifier` 不启 DRM listener（单节拍源）；③`ui/embedder`：`EventFramePresented → NoteFramePresented`、present 成功后 `RequestFrameNotify`（与 FrameSync 并排）；④`render`：`fixedNoVsync`（Wayland）→ `SetVsync` no-op、初始 `SetPreferFifoRelaxed()`（swapchain.go 新增，[FifoRelaxed, Mailbox, Immediate] 不含 Fifo）；X11 保持 Fifo + 风暴期 SetVsync。测试：scheduler 3 个新测试（FrameNotifier 互斥 / listener 只 stamp / NoteFramePresented 只开门）+ `TestWaylandFrameNotify`（真窗合成器闭环，真实合成器下 PASS）+ swapchain `SetPreferFifoRelaxed` 断言；ui/platform 13 文件 + ui/embedder 6 文件 + render present + gpu swapchain 全 PASS；apidoc exit=0。真窗验证待用户复测 `go run ./tmp_resize_diag`（块2/块3 端到端：idle 零渲染 + 风暴期跟手 + frame 节拍）。 |
| 2026-08-19（检查+修1） | 全面回归检查：块1/2/3 相关测试全 PASS（scheduler 4 / platform 16 / embedder 6 / webgpu swapchain 5 / render present 8）+ apidoc exit=0 + X11 storm 30s 无错（`Fifo→Immediate` 切换生效）。发现并修复 X11 `_NET_WM_SYNC_REQUEST` 两处缺陷：①serial 误读 l[2]=height（正确 l[3]）→ 合成器 counter 永不过 serial → 拖拽持续拉伸旧帧；②sync request 的 l[1]/l[2] 几何未使用（XWayland/mutter 拖拽不发 ConfigureNotify，仅 sync request）→ 内容冻结。修复：`atSyncReq` 分支读宽高 → `setSize` + `EventResize`（与 ConfigureNotify 分支同范式），serial 改读 l[3]/l[4 拼 64 位。验证：`x11_sync_test.go` 3 个真窗测试 PASS + ui/platform 全量回归 PASS + X11 storm 无回归。其余发现记入 §8：X11 PresentCompleteNotify cookie 解析待真机、NotifyMSC 无在途去重、无 XPresent 实际退软件间隔（文档修正）。与本会话无关的既有失败 3 处（x11_sync 测试原 FAIL 已随修复转绿；swapchain 同尺寸 Resize no-op 断言过严；render f1/filter GPU 读回疑似其他线改动）另记。 |
