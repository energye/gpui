# 90 · 正向性能与资源指标（真源）

> **与能力 Wave 并行**（[00_meta](./00_meta.md) §5）· 修订：2026-07-28  
> 能力分册 **只挂钩 ID**；本文件为指标定义全文。

## 使用

1. 实现/窗测 PR 必须带可跑命令 + JSON  
2. 相对同机 baseline **不无故回退**  
3. PerfSoak 贯穿：`examples/ui_render_base_perfsoak`  
4. P6 只加深 Present，不重新发明本表  

## 已接线（P0+ 示例常见）

| ID | 状态 |
|----|------|
| M-INTERVAL-P50/P99 · M-HITCH · M-VSYNC-SOURCE | A |
| M-LAYOUT-COUNT · M-PAINT-COUNT · M-RASTER-LAYER | A |
| M-RSS-START/END/PEAK/AFTER-CLOSE · M-CPU-PROCESS | A（示例 ProcessTracker） |
| M-PIPE-DEPTH · M-BUILD-MS · M-RASTER-MS | A/C |
| M-RSS-SLOPE 自动字段 | D（soak 可派生打印） |
| hitch_rate_per_min / p95 | **A**（MetricsStore Snapshot/JSON） |

## §20 正向性能与资源指标全表

> **时机（硬）：** 本表不是「能力全 A 之后的附录」。指标与 §23 能力 Wave **并行**（§0.5）。  
> 每波实现/窗测合入时更新采集与门禁；P3 做 soak **体系化**；P6 只加深 Present，不重新发明指标。  
> **范围说明：** 用户常提 CPU / 内存 / 60fps——**必要但不完整**。下列为渲染基座 **正向指标全景**（可测、可回归）；实现可分波，**清单不可删项装窄**。

### 20.0 指标族总览（补全清单）

| 族 | 回答什么问题 | 典型指标（摘要） | 最低何时要有 |
|----|--------------|------------------|--------------|
| **A 帧时与流畅** | 是否稳 60/120、卡在哪 | 间隔 last/avg/max/**p50/p95/p99**、hitch 率、jank>50ms、FPS wall/complete、目标 Hz、missed_vsync、vsync_source | **P0** 骨架；真 VSync→P1 |
| **B 管线与跟手** | 是否堵在 Present/排队 | pipeline depth/峰值、present 提交 vs 完成、build/raster ms（及分位）、输入→状态延迟、排队时长 | P0 部分；输入延迟门禁加深 P1 |
| **C 脏区局部性** | 成本是否 ∝ 脏而非全树 | layout/paint 计数、PaintVisits、raster/skip/composite layer、compositor-only vs re-raster、bind_count、**(P6) damage 面积/上传字节** | **P0** 计数；damage **P6** |
| **D CPU** | 谁吃 CPU、空闲是否 0 | 进程 CPU% avg；**路径 proxy** UI/Raster %；IDLE；可选 p95 | 进程 **P0**；路径分轨 **P1✅** |
| **E 内存与释放** | 是否漏、关是否放 | RSS start/end/**peak**/**slope KB/min**、after_close、HeapAlloc、NumGC、Goroutines、（可选）显存/驱动池 | RSS **P0**；斜率 soak P0–P3；VRAM 后波 |
| **F GPU / 提交 / 回退** | 是否碎提交、是否掉 CPU 路径 | gpu_submit 次数/帧、batch 大小、CPU fallback 原因与次数、layer pool hit、纹理创建/销毁 | 诊断 P1；门禁 P2–P6 |
| **G 文本 / 图资源** | 热路径是否打中缓存 | glyph **atlas hit/miss**、shape 缓存、图片解码是否离 UI、解码队列深度、上传次数 | 文本/图 Wave 同步；上浮 P1–P3 |
| **H 启动与首帧** | 冷启动是否可接受 | 到首 Present 耗时、warm-up 后首动画帧、字体/shader 首次编译 | 示例记录 P1；门禁 P3 |
| **I 多场景 / 回归** | 改动是否变差 | 同机 baseline JSON delta、场景 ID、机器/GPU/scale 元数据 | **每波合入**（§0.5） |
| **J 正确性相邻** | 「快」是否以错换绿 | 依赖 ui↛gpu、无 cgo、像素/命中抽检、禁止降画质装门禁绿 | 全程 |
| **K 可选平台** | 产品环境 | 电池/热、多窗串扰、遮挡停帧（F15） | 后置 / P6 工具 |

```text
用户常说的：
  CPU · 内存(占用/泄漏/释放) · 60fps+
文档补全后还必须覆盖：
  帧分位与 hitch · 管线/跟手 · 脏区局部 · GPU 提交与 fallback
  · 文本/图缓存 · 启动首帧 · baseline 回归 · 正确性纪律
```

### 20.1 对照 Flutter / 工程（族级）

| 指标族 | Flutter / 工程 | gpui 现状（P0 收尾后） |
|--------|----------------|------------------------|
| A 帧时 / 流畅 | FrameTiming、60Hz、jank | avg/max/last + **p50/p95/p99**；hitch_count + **hitch_rate_per_min**；vsync_source |
| B 管线 / 跟手 | pipeline、build/raster span | depth/max、build/raster **last**；present 提交/完成易混；输入延迟少示例门禁 |
| C 脏区局部 | RepaintBoundary、dev tools | layout/paint **已接线**；raster_layer；PaintVisits 测内；damage 面积 **P6** |
| D CPU | DevTools CPU | **进程** cpu_pct_avg + **路径 proxy** cpu_ui/raster_pct（build/raster 墙钟份额；非 OS 线程） |
| E 内存 | Observatory / RSS | rss_start/end/peak/after_close；缺 slope 自动字段、Heap/GC 未进 JSON |
| F GPU | 提交/缓存 | render 有 path stats / fallback / layer pool — **未系统进 UI JSON** |
| G 文本/图 | atlas、codec | render 内部；UI 示例未导出 hit 率 |
| H 启动 | 首帧 | WarmUp 有；无统一 TTFB 字段 |
| I 回归 | baseline | 手存 JSON；无自动对比工具 |
| J 正确性 | 测试/依赖 | depcheck、门禁测；与性能门禁并列 |
| K 平台 | 遮挡/多窗 | F15 浅；多窗 P6 |

### 20.2 指标 ID 全表（实现真源）

> 状态随实现更新。P0 已落地的不得再标成「无」。

#### A. 帧时与流畅

| ID | 指标 | 对照 | 来源现状 | 单位 | 状态 | 正向门禁方向 | Wave |
|----|------|------|----------|------|------|--------------|------|
| M-FRAME | frame_count | 帧计数 | MetricsStore | count | A | 随 schedule 增 | — |
| M-PRESENT-SUBMIT | 提交 present | — | PipelineApp.PresentCount | count | A | 与 complete 区分解读 | — |
| M-PRESENT-COMPLETE | 完成 present | — | NotePresent JSON present_count | count | A | ≤ submit | — |
| M-PRESENT-COALESCE | 合并/丢弃次数 | 背压 | **无独立字段** | count | D | 可解释 | P1 |
| M-INTERVAL-LAST | 上次帧间隔 | FrameTiming | metrics | ms | A | ~16.7 @60 | — |
| M-INTERVAL-AVG | 平均间隔 | — | metrics | ms | A | ~16.7 | — |
| M-INTERVAL-MAX | 最大间隔 | — | metrics | ms | A | 观察尖峰 | — |
| M-INTERVAL-P50 | 分位 p50 | DevTools | 环形 256 | ms | **A** | ≈16.7 | P0 ✅ |
| M-INTERVAL-P95 | 分位 p95 | — | 环形 256 | ms | **A** | 预算/尖峰 | P1 ✅ |
| M-INTERVAL-P99 | 分位 p99 | — | 环形 256 | ms | **A** | 轻场景 < hitch 阈 | P0 ✅ |
| M-FPS-WALL | wall fps | — | presents/elapsed 示例 | Hz | A | 动画 ≥55 | — |
| M-FPS-COMPLETE | 完成帧 fps | — | complete/elapsed | Hz | C | 诚实 FPS | P1 |
| M-TARGET-HZ | 目标刷新 | 60/120 | 文档 + DefaultAnimTick | Hz | C | JSON 标明 | P1 |
| M-HITCH | hitch_count | jank>≈2frame | >33.4ms | count | A | soak 低 | — |
| M-HITCH-RATE | hitches/min | SLO | hitch_count÷墙钟分钟 | /min | **A** | soak 低 | P1 ✅ |
| M-JANK-33 | >33.4ms | =hitch | hitch | count | A | — | — |
| M-JANK-50 | >50ms | 严重卡顿 | **无** | count | D | 趋 0 | P1 |
| M-MISSED-VSYNC | missed_vsync | vsync miss | Wait 错误时 | count | C | 真 vsync 后 | P1 |
| M-VSYNC-SOURCE | true\|fallback | — | JSON vsync_source | enum | **A** | 诚实 | P0 ✅ |

#### B. 管线与跟手

| ID | 指标 | 对照 | 来源现状 | 单位 | 状态 | 正向门禁方向 | Wave |
|----|------|------|----------|------|------|--------------|------|
| M-BUILD-MS | frame_build_ms | UI build | NoteBuildMs | ms | A | 控制 p99 | P1 分位 |
| M-RASTER-MS | frame_raster_ms | raster | NoteRasterMs | ms | A | 控制 p99 | P1 分位 |
| M-BUILD-P99 | build p99 | — | **无** | ms | D | 预算 | P1 |
| M-RASTER-P99 | raster p99 | — | **无** | ms | D | 预算 | P1 |
| M-PIPE-DEPTH | pipeline_depth | 在途 | SetPipeline | int | A | ≤2 健康 | — |
| M-PIPE-MAX | pipeline_max | 配置/峰值 | 常为配置 2 | int | C | 记观测峰值 | P1 |
| M-QUEUE-WAIT | 提交前等待 | 排队 | **无** | ms | D | 低 | P2 |
| M-INPUT-LAG | 输入到状态 | ≤1 帧 | 测/注入 | frame | C | 慢 Present 仍跟手 | P1 |

#### C. 脏区与局部性

| ID | 指标 | 对照 | 来源现状 | 单位 | 状态 | 正向门禁方向 | Wave |
|----|------|------|----------|------|------|--------------|------|
| M-LAYOUT-COUNT | layout_count | layout 范围 | PipelineApp 接线 | count | **A** | S2 动画期不涨 | P0 ✅ |
| M-PAINT-COUNT | paint_count | paint flush | PipelineApp 接线 | count | **A** | 可解释 | P0 ✅ |
| M-PAINT-VISITS | PaintVisits | retained walk | 测试 Context | count | A 测 | ≪ 全树 | — |
| M-RASTER-LAYER | raster_layer_count | 脏层 | SetRasterLayerCount | count | A | S2 小 | — |
| M-SKIP-LAYER | skipped layers | 静态复用统计 | RasterStats | count | A 内部 | S4 >0 | — |
| M-COMPOSITE-LAYER | composite_layer_count | 合成-only | 占位 | count | C | opacity 动画 | P1 |
| M-BIND | virtual bind | sliver children | VirtualList.BindCount | count | A | ≪ itemCount | — |
| M-DAMAGE-AREA | 脏像素面积 | partial present | **P6** | px | D | 随脏降 | **P6** |
| M-UPLOAD-BYTES | 上传字节/帧 | — | **无** | B | D | 随脏降 | P6 |

#### D. CPU

| ID | 指标 | 对照 | 来源现状 | 单位 | 状态 | 正向门禁方向 | Wave |
|----|------|------|----------|------|------|--------------|------|
| M-CPU-PROCESS | 进程 CPU% | DevTools | ProcessTracker cpu_pct_avg | % | **A** 示例 | S0 低；动画有上界 | P0 ✅ |
| M-CPU-PROCESS-P95 | 进程 CPU p95 | — | **无** | % | D | 尖峰 | P2 |
| M-CPU-UI | UI 路径 % | — | build_ms 份额 → `cpu_ui_pct` | % | **A** | S2 上界；**非** OS 线程 DevTools | P1 ✅ |
| M-CPU-RASTER | Raster 路径 % | — | raster_ms 份额 → `cpu_raster_pct` | % | **A** | 同上 proxy | P1 ✅ |
| M-CPU-IDLE | 空闲 CPU | — | 派生/采样 | % | D | S0 ≈0 | P1 |

#### E. 内存与释放

| ID | 指标 | 对照 | 来源现状 | 单位 | 状态 | 正向门禁方向 | Wave |
|----|------|------|----------|------|------|--------------|------|
| M-RSS-START | 起始 VmRSS | 内存 | ProcessTracker | KB | **A** | — | P0 ✅ |
| M-RSS-END | 结束 VmRSS | — | ProcessTracker | KB | **A** | — | P0 ✅ |
| M-RSS-PEAK | 峰值 RSS | — | ProcessTracker | KB | **A** | 有解释 | P0 ✅ |
| M-RSS-SLOPE | 斜率 KB/min | 泄漏 | **未自动字段** | KB/min | D | soak ≈0 | P1–P3 |
| M-RSS-AFTER-CLOSE | 关闭后 RSS | 释放 | ProcessTracker | KB | **A** 观察 | 可解释下降 | P0 ✅ |
| M-HEAP | HeapAlloc | runtime | 未进 UI JSON | B | C | soak | P1 |
| M-NUM-GC | GC 次数 | — | **无** | count | D | 稳定 | P1 |
| M-GOROUTINES | 协程数 | 泄漏 | **无** | count | D | 稳定 | P1 |
| M-VRAM | 显存/驱动池 | — | **无** | MB | D | 不爬升 | P2–P6 |

#### F. GPU / 提交 / 回退

| ID | 指标 | 对照 | 来源现状 | 单位 | 状态 | 正向门禁方向 | Wave |
|----|------|------|----------|------|------|--------------|------|
| M-GPU-SUBMIT | 提交次数/帧 | batching | render 统计未上浮 | count | B | 勿碎提交 | P1–P2 |
| M-GPU-FALLBACK | CPU 回退原因 | — | LastCPUFallbackReason | string | B | 可诊断 | P1 |
| M-GPU-FALLBACK-N | 回退次数 | — | **无** | count | D | 热路径趋 0 | P2 |
| M-LAYER-POOL | 层池命中 | saveLayer | LayerPoolStats | — | B | 命中率 | P1 |
| M-TEX-CREATE | 纹理创建次数 | — | **无** | count | D | soak 稳定 | P2 |

#### G. 文本 / 图像资源

| ID | 指标 | 对照 | 来源现状 | 单位 | 状态 | 正向门禁方向 | Wave |
|----|------|------|----------|------|------|--------------|------|
| M-ATLAS-HIT | 字形 atlas 命中 | 文本热路径 | render 内部 | ratio | B | 高命中 | P1–P3 |
| M-ATLAS-MISS | atlas 未命中 | — | 内部 | count | B | 可控 | P1–P3 |
| M-SHAPE-CACHE | shape 缓存 | — | 内部 | — | B | — | P2 |
| M-IMG-DECODE-MS | 解码耗时（worker） | F12 | ui/io | ms | C | 不进 UI 线程 | — |
| M-IMG-UPLOAD-N | 图上传次数 | — | **无** | count | D | 滚动不爆 | P2 |

#### H. 启动与首帧

| ID | 指标 | 对照 | 来源现状 | 单位 | 状态 | 正向门禁方向 | Wave |
|----|------|------|----------|------|------|--------------|------|
| M-TIME-TO-FIRST-PRESENT | 至首 Present | 冷启动 | **无统一字段** | ms | D | 预算 | P1 |
| M-WARMUP | warm-up 已跑 | F11 | PipelineOptions.WarmUp | bool | A | 默认开 | — |
| M-FIRST-ANIM-FRAME | 热身后首动画帧 | — | **无** | ms | D | 无巨大尖峰 | P2 |

#### I. 回归元数据（每份 JSON 建议带）

| ID | 指标 | 说明 | 状态 | Wave |
|----|------|------|------|------|
| M-META-SCENARIO | scenario | 如 geometry / spinner / s2 | C 示例名 | P1 字段化 |
| M-META-BACKEND | backend | x11/wayland | A stderr | P1 JSON |
| M-META-SCALE | dpr | Host.ScaleFactor | C | P1 |
| M-META-MACHINE | 机器/GPU 备注 | 文档/手记 | C | 流程 |
| M-BASELINE-DELTA | 相对 baseline | 工具 | D | P3 |

#### J. 正确性相邻（与性能并列，防止假绿）

| ID | 检查 | 状态 |
|----|------|------|
| M-DEP-NO-GPU | ui 无 import gpu | A depcheck |
| M-DEP-NO-CGO | 无 import C | A 纪律 |
| M-GATE-S2/S4/S5 | 既有单测门禁 | A |
| M-NO-QUALITY-CHEAT | 禁止降画质装门禁 | 流程 |

### 20.3 采集建议与合入纪律

- Linux RSS/CPU：`scheduler.ReadRSSKB` / `ProcessTracker` → 示例 JSON  
- 帧分位：MetricsStore 环 256 → **p50/p95/p99**；**hitch_rate_per_min** = hitch_count / (first→last NoteFrameInterval 分钟)；jank50 仍 P1  
- build/raster：**先 last，再补 p99**  
- GPU/atlas：从 render 统计 **上浮到 UI JSON**（P1–P3），避免只活在 render 单测  
- 无真 vsync：`vsync_source=fallback` + 软件 ~16ms；**禁止**宣称锁显示 60Hz  
- `rss_after_close`：观察字段（驱动延迟可能仍高），长 soak 用 **slope** 判泄漏  
- **合入检查（§0.5）：** 能力/窗测 PR → 可跑命令 + JSON；相对 baseline 不无故回退  
- **无 baseline 的「优化」无效**

### 20.4 与「用户口头三项」的映射

| 用户常说 | 落到本表 |
|----------|----------|
| CPU | D 族（进程 → 分轨） |
| 内存占用/泄漏/释放 | E 族（RSS/peak/slope/after_close + Heap/GC） |
| 60fps+ | A 族（p50/p99/hitch/FPS/目标 Hz/vsync） |
| （未说但必须） | B 管线跟手 · C 脏区 · F GPU · G 资源缓存 · H 启动 · I 回归 · J 正确性 |

---


## §21 场景门禁矩阵

| 场景 | 内容 | 关键指标 | 通过方向 |
|------|------|----------|----------|
| S0 | 空闲 | M-CPU-IDLE | CPU≈0 |
| S2 | 单 Spinner | M-LAYOUT, M-RASTER-LAYER, M-FPS* | layout≈0；~60fps |
| S4 | 静态+1 动画 | M-SKIP-LAYER | 静态 skip |
| S5 | 虚拟列表 | M-BIND | bind 上界 |
| S6 | 拖拽滚动 | M-LAYOUT | 无 layout 风暴 |
| Soak | 5–10min | M-RSS-SLOPE, M-HITCH-RATE | 斜率≈0 |
| Close | 关窗 | M-RSS-AFTER-CLOSE | 释放 |
| 60fps+ | Persistent | M-INTERVAL-P50/P99 | p50≈16.7；hitch 低 |
| M1–M5 | render_matrix | 见 examples | 契约+视觉 |

真窗 Present 仍全量时，**不可**用 present 路径 PaintVisits 证明 P6。

---


## 窗测命令

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=15 go run ./examples/ui_render_base_geometry
RUN_SECONDS=15 go run ./examples/ui_render_base_text
RUN_SECONDS=15 go run ./examples/ui_render_base_image
RUN_SECONDS=15 go run ./examples/ui_render_base_cliplayer
RUN_SECONDS=60 go run ./examples/ui_render_base_perfsoak
```

## 相关

- [91 实施顺序](./91_wave_order.md) · [00_meta](./00_meta.md)
