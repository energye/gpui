# 09 · 调度 · Vsync · 帧管道

> **状态（组级）：** A/C · **Wave：** — / P1 · 修订：2026-07-28  
> 拆自 `ENGINE_UI_RENDER_BASE`；状态码见 [00_meta](./00_meta.md)  
> **施工真源分册** — 改状态先改本文件，再同步 [README](./README.md)

## 1. 目标 / 非目标

**目标：** IDLE/TRANSIENT/PERSISTENT、Ticker、WarmUp、帧指标可观测。  
**非目标：** 无真 VSync 时宣称锁屏 60Hz；PerformanceOverlay（P6）。  
**已落地：** FrameScheduler 模式；PipelineApp WarmUp/JSON；p50/p99；vsync_source；ProcessTracker RSS/CPU。

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
| FSch-VSYNC | 垂直同步 | SchedulerBinding/vsync | 稳帧 | VSyncWaiter 接口 | — | Host 可选 | C | platform/host.go · vsync.go | exhost 多无 WaitVSync；fallback 16ms | P1 |
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
| platform | `VSyncWaiter` · exhost |
| metrics | `ui/scheduler/metrics.go` · ProcessTracker |

## 5. 指标挂钩

| 指标 | 说明 |
|------|------|
| M-INTERVAL-P50/P99 | 环 256 |
| M-VSYNC-SOURCE | true\|fallback |
| M-HITCH* | jank |
| M-PIPE-DEPTH | ≤2 |
| M-CPU-PROCESS / M-RSS-* | ProcessTracker |

完整定义 → [90_metrics](./90_metrics.md)。**无 baseline 的优化无效**（[00_meta](./00_meta.md)）。

## 6. 验收 DoD

- [x] 三模式 + Ticker  
- [x] 示例 JSON 指标  
- [ ] exhost **真** WaitVSync（C→A）  
- [ ] p95 / hitch_rate 字段（P1）  
- [ ] UI/Raster CPU 分轨（P1）

## 7. 风险与非宣称

`vsync_source=fallback` 时软件 ~16ms，**禁止**锁显示 60Hz 宣称。

## 8. 相关分册

- [总集 README](./README.md) · [实施顺序 91](./91_wave_order.md) · [指标 90](./90_metrics.md) · [F 交叉 92](./92_f_crosswalk.md)
