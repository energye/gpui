# 09 · 调度 · Vsync · 帧管道

> **状态（组级）：** A/C · 真 VSync 宿主路径 ✅ · **Wave：** — / P1 · 修订：2026-07-28  
> 拆自 `ENGINE_UI_RENDER_BASE`；状态码见 [00_meta](./00_meta.md)  
> **施工真源分册** — 改状态先改本文件，再同步 [README](./README.md)

## 1. 目标 / 非目标

**目标：** IDLE/TRANSIENT/PERSISTENT、Ticker、WarmUp、帧指标可观测。  
**非目标：** 无真 VSync 时宣称锁屏 60Hz；PerformanceOverlay（P6）。  
**已落地：** FrameScheduler 模式；PipelineApp WarmUp/JSON；**p50/p95/p99**；**hitch_rate_per_min**；vsync_source；ProcessTracker RSS/CPU。  
**已落地真 VSync（FSch-VSYNC / F06）：**  
- `examples/exhost` X11/Wayland `Host.WaitVSync` → `platform.WaitDRMVBlank`（libdrm `drmWaitVBlank`，purego）  
- `FrameScheduler.WaitFramePace`：成功 → `vsync_source=true`；失败/无 waiter → `fallback` +（失败时）`missed_vsync++`  
- 单测：假 waiter 优先；错误回退；DRM API 不 panic

## 2. 依赖（交叉关联）

| 方向 | 分册 / 说明 |
|------|-------------|
| **前置** | [00_meta](./00_meta.md) |
| **同级可并行** | 见 [91_wave_order](./91_wave_order.md) |
| **解锁** | 全部窗测与动画 · [90_metrics](./90_metrics.md) |

## 3. 能力表（母表全文）

## §14 调度 · Vsync · 帧管道母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 | Wave |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|------|
| FSch-VSYNC | 垂直同步 | SchedulerBinding/vsync | 稳帧 | **VSyncWaiter + DRM Wait** | — | Host 可选 | A/C | vsync_drm_linux · exhost · WaitFramePace | 无 DRM 时诚实 fallback；禁锁 60Hz 宣称 | — |
| FSch-TRANSIENT | 短暂帧回调 | scheduleFrameCallback | 动画 | Mode TRANSIENT | — | FrameScheduler | A | scheduler.go | — | — |
| FSch-PERSISTENT | 每帧回调 | persistent frame callbacks | 连续动画 | Mode PERSISTENT + Ticker | — | — | A | scheduler.go | — | — |
| FSch-IDLE | 空闲阻塞 | 无事不转 | 省电 | Mode IDLE + WaitEvents | — | — | A | scheduler.go | S0 | — |
| FSch-WARMUP | 热身帧 | warm-up frame | 减首帧 jank | PipelineOptions.WarmUp | — | — | A | pipeline_app.go | F11 | — |
| FSch-TIMINGS | FrameTiming | FrameTiming | 分轨耗时 | frame_build_ms/raster_ms | — | MetricsStore | C | metrics.go | 多为 last；缺直方图 | P0 |
| FSch-TIMINGS-CB | timings callback | addTimingsCallback | 自定义采集 | — | — | — | D | — | 可扩展 Metrics | P1 |
| FSch-TICKER | Ticker | Ticker/AnimationController | 动画驱动 | animation.Controller+TickerRegistry | — | — | A | animation · ticker.go | F17 | — |
| FPerf-OVERLAY | 性能 HUD | PerformanceOverlay | 开发态 | — | — | — | D | — | P6 | P6 |
| FPerf-DEVTOOLS | CPU/内存工具 | DevTools | 优化 | 无内建 | render mem harness | 示例 JSON 部分 | C/D | mem_harness_test.go | ui 需接入 | P0 |

---



## 4. 代码落点

| 层 | 路径 |
|----|------|
| scheduler | `ui/scheduler/*.go` |
| embedder | `pipeline_app.go` |
| platform | `VSyncWaiter` · **`WaitDRMVBlank`**（linux）· exhost X11/WL `WaitVSync` |
| metrics | `ui/scheduler/metrics.go` · ProcessTracker |

## 5. 指标挂钩

| 指标 | 说明 |
|------|------|
| M-INTERVAL-P50/P95/P99 | 环 256 |
| M-VSYNC-SOURCE | true\|fallback |
| M-HITCH* / M-HITCH-RATE | jank 计数 + **hitches/min** |
| M-PIPE-DEPTH | ≤2 |
| M-CPU-PROCESS / M-RSS-* | ProcessTracker |

完整定义 → [90_metrics](./90_metrics.md)。**无 baseline 的优化无效**（[00_meta](./00_meta.md)）。

## 6. 验收 DoD

- [x] 三模式 + Ticker  
- [x] 示例 JSON 指标  
- [x] exhost **真** WaitVSync（DRM vblank；失败 → fallback + 诚实 `vsync_source`）  
- [x] **p95 / hitch_rate_per_min** 字段（`frame_interval_p95_ms` · JSON；测绿）  
- [ ] UI/Raster CPU 分轨（P1）

## 7. 风险与非宣称

`vsync_source=fallback` 时软件 ~16ms，**禁止**锁显示 60Hz 宣称。

## 8. 相关分册

- [总集 README](./README.md) · [实施顺序 91](./91_wave_order.md) · [指标 90](./90_metrics.md) · [F 交叉 92](./92_f_crosswalk.md)
