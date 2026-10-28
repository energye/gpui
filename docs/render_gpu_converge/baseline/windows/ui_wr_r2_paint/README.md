# ui_wr_r2_paint

- 来源：`windows.md` §3.2（ui_wr）
- 目录：`examples/ui_wr_r2_paint`
- 运行：`A/5s`（mode=A，seconds=5，batch=False）
- 金图：`testdata/showcase_r2_paint_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：缓存/边界 · 局部 NeedsPaint

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 | 0.0000% | 19.18 | 20.48 | 57.72 | 1 | 1 | p99=20.48; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 | 0.0000% | 18.31 | 18.99 | 57.85 | 1 | 1 | p99=18.99; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

