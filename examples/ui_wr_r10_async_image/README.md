# ui_wr_r10_async_image — R10 async image → local damage

**Ability:** R10 (W3)
**Window:** **1200×800**
**Close duration:** **`RUN_SECONDS=30`** (§2.5 关闭用 30s — 逐格加载观察)

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=30 go run ./examples/ui_wr_r10_async_image
```

`RUN_SECONDS<5` → `FAIL:` + `exit 1` (U16)。

## Visible effect

| Region | Effect |
|--------|--------|
| Body 主区 6×4 网格 @ (284,60) | 24 格 RenderImage 占位符·每格 140×140 间隔 8·占位色暗蓝灰 chrome |
| 加载完成 | Spike 期逐格 SetImage 翻成解码色块（左上→右下·每 0.8s 一格·色随 idx 变）·**只该格闪变** |
| 静格不重绘 | 加载某格时其他 23 格缓存命中 skip（Picture 不变） |
| Recover | 25s+ 全 Clear 回占位 chrome |
| LiveHUD band @ y=728 | R10 + phase + fps + p95 + `loaded=X/24 rr/img=1 skip=Y` |
| Legend @ (12,60) 8 行 | async image / local dirty / 占位 / SetImage / phase / HUD 说明 |

**相位脚本（PhaseClock 30s 窗）：**
- Steady 0–5s：24 格全 SetLoading（占位 chrome 建立）
- Spike 5–25s：逐格 SetImage 每 0.8s（加载完成·局部脏一格·左上→右下顺序）
- Recover 25s+：全 Clear 回占位

## Gates (§2.2 全族 + R10 专用)

**族 A 帧时**（加载类 · §2.2.2）：
- `fps_interval ≥ 55` 或 `interval_p95_ms ≤ 22`（Elapse≥5s 后硬判）
- 必须输出 `vsync_source`（fallback 禁止宣称锁 60Hz）

**族 C 脏区**（R10 专用）：
- `max_rr_per_img_load ≤ 1`（单次 SetImage 只脏一格 boundary·非全树重绘）
- `load_events ≥ 10`（Spike 20s 必须驱动 ≥10 次 SetImage 压测局部脏）
- `loaded_peak ≥ 10`（至少 10 格达 ImageReady）
- `boundary_skip ≥ 1`（非加载格缓存命中 skip）
- FullPaint 下 `damage_ratio` 可近 1（不得用其冒充 Retained）

**族 D CPU**：`cpu_pct_avg` 长窗（≥30s）`>85%` 且持续 → FAIL；`cpu_ui_pct`/`cpu_raster_pct` 必采（禁双 0 装降画质过 fps）

**族 E 内存**（R10 长跑 ≥30s · 同 R7/R7b 诚实化）：
- `rss_start/end/peak_kb` 必采（Linux）
- 真泄漏门禁：`rss_peak_kb > 200MB` 且 `rss_slope_kb_per_min > 300000` → FAIL
- 本机基线标注：R3/R4/R7/R7b/R11 已绿真窗同机基线·peak≈130MB·after_close==peak，R10 cold/warm peak 稳定非渐进泄漏

**族 F GPU**：热路径 `cpu_fallback_ops` 无故暴涨 → FAIL

**族 G 图/文**：`g_metrics=skipped` + 原因（R10 是图加载→局部脏能力非 R9 文本度量缓存；解码 buffer 经 SetImage 测非 measure cache）

**族 H 启动**：`warmup=true`；首帧有内容

**族 J 正确性**：`ui` 无 import gpu；无 cgo；本窗不降画质

## U20 实现点六维

| 维度 | R10 说明 |
|------|---------|
| 正确性 | 每 RenderImage cell 独立 RepaintBoundary；SetImage/SetLoading/SetError/Clear 只 MarkNeedsPaint 自身（不向上泡整树脏）；SetImage 取 ImageBuf 所有权（Dispose 旧）；加载完成只脏一格 boundary |
| 脏区 | SetImage → MarkNeedsPaint(self) 只脏自身；BoundaryCache invalidate 该格 entry → rerecord=1；其他 23 格缓存命中 skip（Picture 未变）；Clear 路径同理（只局部脏） |
| 缓存 | 每 cell 缓存为独立 RepaintBoundary entry；加载完成只 invalidate 该格 entry；非加载格缓存整 30s 窗保持有效 |
| 边界条件 | 24 cell 6×4 网格；SetImage nil → SetError；Dispose'd buffer 拒收（SetError）；Recover 清回占位；加载序左上→右下；idle/loading/ready/error 四态均历 |
| 失败模式 | max_rr_per_img_load>1 = FAIL（加载只脏一格非全树）；fps<55 加载期 = FAIL；RSS slope 真泄漏门禁（peak>200MB + slope>300000） |
| 窗内如何看出 | 24 占位 chrome cell；Spike 期逐格翻解码色块（左上→右下）；只新加载格闪变其他 23 静；HUD 显 loaded=X/24 rr/img=1 skip=Y |
