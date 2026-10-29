# C7 解剖报告：GL 第 N 窗 `eglMakeCurrent 0x3002`（2026-10-30，含 Wayland 复现修正）

> 任务：P2.4a 影响面评估 §5 约定的解剖产出。性质：只诊断定位，不改代码。
> 台账原文（§2.2 C7）：「`ui_base_vram_cap` ladder 的 go 档在 GL 第 4 窗起 `eglMakeCurrent 0x3002`，8 窗阶梯跑不了；归 P2.4a/P2.4e（gles 多窗上下文）」。
> 结论归属：**根因在 gles 多窗上下文的跨线程绑定管理（呈现/GL 线程模型），不在设备创建入口——修复归 P2.4e**；P2.4a 的建设备收敛与本根因无交集。
>
> **2026-10-30 修正（P2.4a 生命周期复跑补充）**：嵌套 Wayland 隔离环境上 `ui_base_multiwin_cap` go 档也在**第 8 窗**复现同错误（`evidence/P2.4a/multiwin_go.stderr`，rc=1）。Wayland 建面是 context-per-surface（`api_linux.go:127-141` Path B，每窗自有 EGL context），X11 的「共享实例 context × keep 租约」机制不直接适用——**Wayland 的具体触发路径是第二个未解剖机制**（同错误码、同「第 N 窗打开」形态、窗号不确定），P2.4e 修 C7 时需一并定向解剖（候选：hub 线程对某共用资源的绑定租约 / EGL display 侧冲突 / msw 交互）。本报告的 X11 机制链不受影响。

## 0 结论（一句话）

**所有 GL 工作集中在进程唯一 GL 线程（`ui/raster/hub`），而「开窗」的 EGL 绑定型操作（surface 创建、adapter 探测、设备打开、swapchain configure）跑在控制器线程**；共享 EGL context 在 hub 的 Submit→Present 期间被 keep 在 hub 线程上（性能优化 `UnlockKeepBinding`），此时任何其它线程 `eglMakeCurrent` 同一 context 都被 EGL 单绑定规则拒绝（EGL_BAD_ACCESS 0x3002）。ladder 每窗带 160 张静态卡连续重绘，窗越多 hub 忙碌占比越高，第 4 窗的打开动作撞进 hub 忙碌期 → 三次绑定尝试全失败 → `FAIL: app 4 open`。这是**竞态**，非确定性窗号（2/3 窗躲过了忙碌间隙，第 4 窗没躲过）。

## 1 现象（P1.6 原始证据）

`baseline/vram/ui_base_vram_cap/ladder/go/stderr.log:3-5`：

```
2026/10/29 07:20:25 ERROR gles: AdapterContext.Lock MakeCurrent failed err="eglMakeCurrent failed: error 0x3002"
2026/10/29 07:20:26 ERROR gles: AdapterContext.Lock MakeCurrent failed err="eglMakeCurrent failed: error 0x3002"
FAIL: app 4 open: render: ConfigureFromCapabilities: gles: failed to bind GL context for swapchain framebuffer: gles: AdapterContext: MakeCurrent failed: eglMakeCurrent failed: error 0x3002
```

之前一行是 `phase n=2 shared=2 fps_min=54.4`——前两扇窗双后端正常呈现（54 fps），说明「多窗共享一个 EGL context + keep 租约」在帧路径上是自洽的，问题只在**开窗**动作上。

## 2 机制链（file:line）

| # | 环节 | 位置 | 事实 |
|---|---|---|---|
| 1 | 进程唯一 GL 线程模型 | `ui/raster/hub.go:16-24` | 「every Loop in the process runs its frame jobs on this one OS thread」；注释原文写明：per-window 线程会让共享 context 被别窗线程持有 → `eglMakeCurrent` 失败 0x3002 → 该窗所有 GL 调用静默 no-op、画面冻结 |
| 2 | X11 下所有窗共享一个 EGL context | `gpu/gwgpu/gles/api_linux.go:234-267` | `createX11Surface` 首窗建 `i.x11` 共享 context，后续窗 `Surface.ctx = i.x11.ctx`（`ownsContext=false`） |
| 3 | 共享 context 流进每窗的 Device/Queue | `gpu/gwgpu/gles/adapter_linux.go:52`（Open 锁 `a.ctx`）、`:88`（`Device.ctx = a.ctx`）、`:92`（`Queue.ctx = a.ctx`） | 全部窗的 Submit/Present 挤同一个 AdapterContext |
| 4 | Submit 结束把绑定**留**在 GL 线程 | `gpu/gwgpu/gles/queue_linux.go:41-45`（`defer q.ctx.UnlockKeepBinding()`） | keep 是性能优化（省 UnmakeCurrent FFI，pprof 170ms/1.87s）；租约语义在 `adapter_context_linux.go:172-192`（keepTid，外部线程自动 unmake 兜底） |
| 5 | Present 结束才 unmake | `gpu/gwgpu/gles/queue_linux.go:257-268` | `TryLockForSurface` → 绘制 → `defer Unlock()`（unmake）。**Submit 返回到 Present 结束之间，context 绑定在 hub 线程** |
| 6 | 开窗在控制器线程做绑定操作 | `examples/ui_base_vram_cap/main.go:345-368`（openBatch：控制器线程 `app.Open()`，每窗 Run 在独立 goroutine）；`gpu/gwgpu/gles/api_linux.go:254`（createX11Surface `Lock`，失败仅记 ERROR）、`resource_linux.go:86`（GetAdapterInfo `Lock`，失败仅记 ERROR）、`adapter_linux.go:52`（Open `Lock`，失败仅记 ERROR）、`resource_linux.go:179`（ConfigureFromCapabilities `TryLock`，**失败即开窗失败**） | stderr 的 2 行 ERROR = 开窗路径的 Lock 站点，第 5 行 FAIL = TryLock 站点 |
| 7 | EGL 单绑定规则 | `adapter_context_linux.go:36-40`（「EGL admits a single binding thread per context」） | 跨线程 MakeCurrent 返回 0x3002，本机 NVIDIA/EGL 实测一致 |

**时序还原**：第 4 窗打开时，hub 线程正处于某窗的 Submit（keep）→Present 之间（每窗 160 张卡连续重绘，hub 忙碌占比随窗数上升）；控制器线程连续三次 MakeCurrent（2 次 Lock 记 ERROR、1 次 TryLock 致命）全部落在这个窗口期 → 0x3002 → 开窗失败退出。

## 3 为什么窗 2/3 没失败（X11）

帧间（Present 的 Unlock 之后）context 是 unbound 的——开窗若落在帧间隙就能成功。窗越多、hub 忙碌占比越高，开窗落在忙碌期的概率越大：2/3 窗打开时躲过（间隙命中），第 4 窗命中。**这与「第 4 窗起必挂」的台账表述需要修正：它是竞态概率现象，不是第 4 窗的确定性阈值**（P2.4a 复跑在 X11 ladder 撞第 4 窗、嵌套 Wayland multiwin 撞第 8 窗，窗号随负载浮动）。

## 4 修复方向候选（归 P2.4e，本步不动）

| 方案 | 思路 | 代价 |
|---|---|---|
| (a) 开窗 GL 步骤投递 hub 线程执行（推荐） | `hub.go:38-40` 已有 `jobs chan FrameJob`（注释明确「GL-thread work that belongs to no window loop：handoff releases, snapshot readback, teardown destroys」）——把 surface 创建/adapter 探测/swapchain configure 作为 FrameJob 排进 hub，与帧路径天然互斥 | 需要把开窗改成异步/half-sync（`app.Open` 等 job 完成）；改动集中在 gles surface/configure 的调用侧 |
| (b) 打开前 hub 屏障 | 开窗前等 hub 完成当前帧、确认 unbound 再放行 | 引入新的同步原语；hub 忙碌时开窗延迟不可控 |
| (c) 放弃 keep 优化 | 全部 Unlock（unmake） | 直接回退已量化的性能优化（170ms/1.87s），不可取 |

验收闭环（P2.4e 做）：修后 `ui_base_vram_cap` ladder go 档 8 窗跑通（隔离显示环境），即 C7 关闭条件。

## 5 与 P2.4a 的关系

- C7 的碰撞面是「开窗的 EGL 绑定操作 × hub keep 窗口期」，**不在建设备/选卡路径**：`buildPresentTarget` 的 shareOpenMu 只串行化了打开，没协调 GL 绑定；设备创建收敛（本步）不触碰 EGL context 管理。
- 本步唯一交集：开窗路径上的 `RequestAdapter`/`RequestDevice` 会经 `GetAdapterInfo`/`Adapter.Open` 里的共享 context `Lock()`——它们在绑定失败时只记 ERROR 继续（GL 查询拿空值），**本来就带病运行**（`adapter_linux.go:52` 在 unbound context 上查询 maxTexUnits/vendor 得到空值时用保守默认兜底）；这属于 C7 的伴生现象（开窗时探测数据可能为空），随 P2.4e 方案 (a) 一并消除。

## 6 复现要点（P2.4e 修后验收用）

```bash
# 隔离显示环境（按 AGENTS GPU 真窗安全条，勿在当前桌面批量跑）
GPUI_BACKEND=go RUN_SECONDS=300 examples/ui_base_vram_cap -mode ladder -auto-only
# 预期（修后）：8 窗全开、无 0x3002、账本峰值 < 0.9 水线；当前基线：第 4 窗 FAIL
```
