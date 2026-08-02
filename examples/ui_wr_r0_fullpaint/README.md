# ui_wr_r0_fullpaint — R0 FullPaint 正确性

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=5 go run ./examples/ui_wr_r0_fullpaint
```

## Window / Close duration

1200×800 逻辑像素（U15）。关闭用 RUN_SECONDS=5（§2.5），本地建议观察 15s。
`RUN_SECONDS<5` → FAIL（U16）。

## Visible effect

| Region | Expectation |
|--------|-------------|
| Body 左上 6×4 静色格 @ (304..852, 80..444) | 每帧重画（FullPaint），颜色稳定不闪 |
| Body 右侧 12 行 static text @ (884..1180, 80..200) | 每帧存活，Clear 后不丢 |
| HOT 动块 120×120 | Steady 红 @(884,200)；Spike 黄 @(884,320)；Recover 蓝 @(964,200)，位置随相位跳变 |
| 顶栏 | R0 标题 + 相位 + policy=full_paint |
| 底栏 LiveHUD | 实时 FPS / p95 / paint=… / dmg=… / PASS 预览色 |

## Gates

| # | Gate | FAIL 线 |
|---|------|---------|
| 1 | 持续 tick 帧率 | 运行 ≥5s 后 `fps_interval≥55`（§2.2.2 正确性+持续 tick 档） |
| 2 | FullPaint 全树 | `paint_count ≥ presents×0.9`（每帧全树重画，静态不丢） |
| 3 | 静/动同屏 | `static_visible≥8` 且 `hot_repaints>0`（§2 主表「静网+文+动」） |
| 4 | policy | `present_policy=full_paint`；`damage_ratio≈1` 属 FullPaint 语义，**不得**冒充 Retained（§2.2.1 族 C） |
| 5 | 全族 A–J | §2.2.1 必采字段全部出现；无数据填 null+原因，禁止默默省略 |
| 6 | 首帧 | `warmup:true`；首帧有内容（族 H） |
| 7 | vsync | `vsync_source` 必有；`fallback` 禁止宣称锁 60Hz |
| 8 | CPU/内存 | `cpu_pct_avg`/`cpu_ui_pct`/`cpu_raster_pct` 必采；`rss_start/end/peak_kb` 必采（短正确性窗 slope 默认告警） |
| 9 | 族 J | `ui` 无 import gpu；无 cgo；本窗不降画质 |

## 实现点六维（U20）

| 维度 | 回答 |
|------|------|
| 正确性 | FullPaint 语义 = 每帧全树重画；Clear（LoadOpClear）后静态内容每帧存活 |
| 脏区 | FullPaint 下 damage_ratio≈1 是正确语义（不是偷省绘）；JSON 标 `damage_semantic=full_paint_full_redraw_allowed` |
| 缓存 | BoundaryCache 在 full_paint 下每帧重录（`boundary_rerecord` 每帧）；本窗不测 skip |
| 边界条件 | 相位跳变（Spike/Recover）时动块改位/改色，静态区不抖动 |
| 失败模式 | 若 Clear 丢静态 → paint_count<frames 或 HUD 静态区闪黑；fps<55 → FAIL |
| 窗内如何看出 | LiveHUD 显示 paint/dmg 数字 + 相位；动块相位色；静态格颜色稳定 |
