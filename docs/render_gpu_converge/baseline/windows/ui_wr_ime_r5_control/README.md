# ui_wr_ime_r5_control

- 来源：`windows.md` §3.2（ui_wr）
- 目录：`examples/ui_wr_ime_r5_control`
- 运行：`A`（mode=A，seconds=None，batch=False）
- 金图：`testdata/r5_control_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：IME/文本 · 控件与滚动

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 |  | 19.18 | 19.46 | 57.60 | 1 | 1 | p99=19.46; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 |  | 18.68 | 19.28 | 57.68 | 1 | 1 | p99=19.28; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

