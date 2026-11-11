# I5 v2 设计：Wayland present 非阻塞化（根因修复，2026-11-12）

> 本文件取代 `../i5-robustify-design.md`（2026-11-11 旧设计：per-surface worker + watchdog + 每窗设备）。
> 旧设计的前提已被实测推翻：worker 隔离不了 wgpu-native 内部锁（2c），每窗设备需要拆全局 accelerator 且撞 FFI 指针问题（D1c），两者都已回滚。
> 本设计只解决已实锤的根因：**Mesa Wayland WSI 的阻塞式 present**。不引入设备/进程隔离，不拆 P2.4a 共享设备。

## 1 结论

- 卡死的不是「合成器坏了」，而是 **native Wayland 默认 `FIFO` present**：`vkQueuePresentKHR` 在 Mesa WSI 内用 `wl_display_dispatch_queue → poll(-1)` 等合成器 release；合成器不释放时 FFI 永不返回。
- 这个同步 FFI 在共享 raster hub 上持进程级 `gpuMu`，一扇窗卡住 → 全进程 GPU 冻结、关窗/退出都进不来。
- 修复：**native Wayland 的交换链默认用非阻塞 present（Mailbox 优先，Immediate 次选，FifoRelaxed/Fifo 兜底）**；X11/Windows/macOS 保持现有 FIFO。合成器负责上屏节流，客户端不再在 present 里等合成器。
- 这不是「换一个模式试试」：Vulkan 的 `MAILBOX` 语义就是替换待上屏图像、不等待 vblank/释放；`FIFO`/`FIFO_RELAXED` 都会在无空闲图像时等。实测 FifoRelaxed 与 Mailbox 的差异正好落在这里（见 §3 A/B）。

## 2 现场与证据

### 2.1 当前树复现（Fifo，级联 8 窗）

- 环境：嵌套 gnome-shell Wayland `gpui-i5diag`（`dbus-run-session -- gnome-shell --nested --wayland-display=gpui-i5diag --no-x11`），native / Intel Vulkan，8×640×480 级联摆放（`X=idx*56, Y=idx*56`，全部在嵌套屏内；I1 §5 已证摆放不是触发面）。
- 命令与产物：`diagnosis/fifo-cascade.stderr`、`fifo-cascade.rc`。
- 结果：

```
phase n=2 fps_min=44.4 refusals=+0
phase n=4 fps_min=21.9 refusals=+440
phase n=8 fps_min=0.0 refusals=+0        ← 冻结
SIGQUIT: quit                             ← 150s 仍未退出
```

- SIGQUIT 栈：hub 线程（locked to thread）`rwgpu.(*Surface).Present`（`gpu/rwgpu/surface.go:517`）→ `procSurfacePresent.Call` → purego → `wgpuSurfacePresent`。与 I1 gdb 原生栈（Mesa WSI `poll(-1)`）同一条链路。

### 2.2 A/B：同一现场、只换 present 模式（全窗 Mailbox）

- 所有 8 窗都走 Wayland native；`I5DIAG present mode=mailbox` × 8（诊断日志在 `diagnosis/mailbox-cascade.stderr`）。
- 结果（300s 跑满）：

```
phase n=2 fps_min=46.5 presents=[465,466]
phase n=4 fps_min=22.3 presents=[224,223,223,223]
phase n=8 fps_min=11.4 presents=[115,114,114,115,115,115,115,115]
rc=1（golden 17.5273% 预存红；进程正常退出）
```

- SIGQUIT/卡栈：无 `Surface.Present` 卡栈；8 窗在 n=8 全部继续 present；进程在 RUN_SECONDS 到期后正常退出。
- 账本：`ledger_peak_mb=355.07` < 768×0.9；hold 斜率 `-148.7KB/min`（无线）。
- 金图：`pixel_golden_per_window=[0,17.5273%×7]`，`pixel_probes_ok=16/16`。17.5273% 与上一轮 D1b 记录逐字相同（`I5 D1b A/B：... 17.5273% 为陈旧 golden`），属 P2.6 已登记的「22 窗金图漂移」族，不是本次模式切换引入；处理见 §6。

### 2.3 单窗节流对照（嵌套 Wayland，20s，非性能结论）

`ui_wr_c6_savelayer_group` 同一窗、同一环境：

| 模式 | fps_wall | p50 | p95 | p99 | hitch/min |
|---|---|---|---|---|---|
| Fifo | 58.31 | 16.56 | 19.99 | 21.02 | 9.01 |
| Mailbox | 58.59 | 16.76 | 21.76 | 23.44 | 6.01 |

结论：Mailbox 没有掉帧或明显抖动回归；嵌套环境本身 hitch 偏高（`ENGINE_WAYLAND_NESTED_TEST.md` 已登记嵌套+软件栈 jank 正常），本表只做同环境相对对照。

### 2.4 被否决的旧路线（不再走）

| 路线 | 反证 |
|---|---|
| 2b：per-surface worker + watchdog | 2c 真窗：卡死 present 持 wgpu-native 内部锁，hub 的 `ProcessEvents` 被连坐 7 分钟；worker 隔离不了这把锁（`historical/2c-analysis.md`） |
| D1：每窗独立 instance/device | D1c：4 窗判死 + `CreateCommandEncoder` 824GB 垃圾分配 SIGABRT；且需拆全局 `GPUShared`/`SetAcceleratorDeviceProvider`，改动面与 P2.4a 收敛成果冲突（`historical/d1c-analysis.md`） |
| 只加 gpuMu 拆锁 | 2a 已证必要但不充分：present 不持 `gpuMu` 后仍卡在 wgpu-native 内部锁 |

## 3 标准依据

- Vulkan `VK_PRESENT_MODE_MAILBOX_KHR`：呈现引擎保留一个待上屏图像，新图像到达时替换它；不等待 vblank，也不等待旧图像被消费。`FIFO`/`FIFO_RELAXED` 在队列满/无空闲图像时等待。Mesa Wayland WSI 的 `poll(-1)` 正是这条等待。
- Wayland 客户端模型：合成器是上屏节流权威；客户端用 `wl_surface.frame` 回调安排下一帧，不应在同步 present 里等合成器。本库的 raster hub 是所有窗口共享的 GL 线程，任何同步阻塞都会连坐其它窗，因此 native Wayland 必须选非阻塞 present。
- 与 Flutter/Chrome 的对应：Flutter 把 present 放在 raster 线程，引擎侧不等待合成器；Chrome 更进一步把 GPU 放独立进程。本设计取 Flutter 同层的「present 不阻塞」，不引入进程隔离。

## 4 实现

1. `gpu/webgpu` 新增 `(*Swapchain).SetPreferNonBlocking()`：优先级 `Mailbox → Immediate → FifoRelaxed → Fifo`，注释写明 Wayland 合成器节流语义与兜底顺序。保留 `SetPreferLowLatency`/`SetPreferFifoRelaxed` 原语义。
2. `render.newPresentSwapchain` 增加 `preferNonBlocking bool` 参数：Wayland native 传 true（真正设置 prefer list 的唯一位置）；其它平台 false（Fifo）。`buildPresentTarget`/`buildSharedSurface` 删除无条件 `SetPreferVSync()`/`SetPreferFifoRelaxed()`，避免覆盖。
3. Wayland native 的 `PresentTarget.fixedNoVsync=true`、`vsyncOn=false`：present 已非阻塞，`SetVsync` 的 Fifo↔低延迟切换在 Wayland 为 no-op；X11/Windows 行为不变。
4. 可观测：`PresentTarget.PresentModeName()` 暴露实际模式；`ui_base_vram_cap` 的 `ability_extra` 增加 `swapchain_present_mode`。Wayland 上若能力列表只剩 Fifo，启动时打一次 WARN（`I5 residual: ...`），不静默。
5. 不改 `hal` 接口；不改 FFI；不改共享设备；不改 raster hub 线程模型。

## 5 验收与载体

- 能力单测：
  - `gpu/webgpu/swapchain_configure_nonblocking_internal_test.go` :: `TestPickPresentMode_NonBlocking`（Mailbox 可用→Mailbox；Mailbox 不可用/Immediate 可用→Immediate；只剩 Fifo→Fifo）。
  - `render/present_mode_wayland_test.go` :: `TestPresentPreferNonBlocking`（Wayland→true，X11/Windows→false）。
- 真窗：`examples/ui_base_vram_cap` 级联 8 窗（`-auto-only`，`RUN_SECONDS=300`，嵌套 gnome-shell Wayland），记录 8 窗 `presents`、`swapchain_present_mode`、退出码、`ledger_peak_mb`、卡栈 grep。
- 金图：`examples/ui_base_vram_cap/testdata/vram_win{1..8}_base.png`，容差 0；17.5273% 属预存漂移，按 §6 处置。
- 双后端：native Wayland 本项验；go/gles 同窗归 C7（EGL present 阻塞另行立项），本项不混入，结果格写「C7 域，未验」。
- 负向对照：把 Wayland prefer 强制回 Fifo（同一补丁已在 `diagnosis/diagnosis-patch.diff` 与 Fifo 跑留档），预期 n=8 冻结 + 进程不退出；还原修复后同窗全 8 窗 presents>0 且正常退出。
- 回归：`gpu/webgpu`、`render` 相关逐文件单测；X11 `ui_base_vram_cap`（隔离 :3 或 Xvfb）确认 present 模式仍 Fifo 且门禁不回退；触及共享数据的改动跑 `-race`。
- 证据目录：`docs/render_gpu_converge/evidence/P2.4e/i5-v2/`。

## 6 风险与边界

1. **Mailbox 不可用**：按 `Mailbox → Immediate → FifoRelaxed → Fifo` 兜底；兜到 Fifo 时 WARN 并登记残余风险，不把「没冻」当保证。
2. **金图漂移**：`vram_win2..8` 的 17.5273% 只在 HUD 下方卡片区出现；先按 P2.6「22 窗金图漂移」逐窗判定「渲染洞 vs 基线过期」，确认卡片内容正确后再决定是否重种，禁止直接重种掩盖。
3. **GL/gles Wayland**：`eglSwapBuffers` 仍可能阻塞；归 C7，本项不修、不标绿，跨平台格写原因。
4. **真正卡死的驱动/合成器**（Mailbox 也不返回）：进程内无法打断 FFI；需要 Chrome 式 GPU 进程隔离，另立远期专项，不在 I5 承诺。
5. **帧节奏**：Mailbox 让客户端不等待，调度器仍按显示刷新 pacing；若某窗口输出快于显示，合成器丢旧帧是预期行为，不作为 jank 回归。

## 7 分批

| 批 | 内容 | 验收 |
|---|---|---|
| v2-0 | 本设计 + 当前树复现 + Fifo/Mailbox A/B（已完成） | 诊断证据落盘 |
| v2-1 | `SetPreferNonBlocking` + render 创建口 + `PresentModeName` + 单测 | ✅ 单测逐文件 PASS；catalog 已同步 |
| v2-2 | 真窗 8 窗 + 单窗节流 + X11 回归 + 负向对照 | ✅ 8 窗 presents>0、正常退出、无卡栈；负向 n=8 冻结；X11 无回退；go/gles 70s 未冻（C7 未跑满） |
| v2-3 | 金图漂移判定/处置 + catalog/台账 + P2.6 重放 | 🟡 漂移材料已落盘；用户 2026-11-12 拍板留 🟡 交 P2.6 逐窗判定，不直接重种 |
