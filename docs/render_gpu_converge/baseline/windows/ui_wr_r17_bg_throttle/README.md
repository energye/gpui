# ui_wr_r17_bg_throttle

- 来源：`windows.md` §3.2（ui_wr）
- 目录：`examples/ui_wr_r17_bg_throttle`
- 运行：`A/30s`（mode=A，seconds=30，batch=False）
- 金图：`testdata/r17_bg_throttle_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：帧时/调度 · 不可见停帧

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 |  | 19.49 | 25.22 | 57.44 | 2 | 1 | p99=25.22; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 |  | 20.90 | 25.08 | 57.52 | 2 | 1 | p99=25.08; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

