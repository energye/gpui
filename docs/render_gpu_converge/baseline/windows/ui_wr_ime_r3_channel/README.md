# ui_wr_ime_r3_channel

- 来源：`windows.md` §3.2（ui_wr）
- 目录：`examples/ui_wr_ime_r3_channel`
- 运行：`A`（mode=A，seconds=None，batch=False）
- 金图：`testdata/r3_channel_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：IME/窗口 · 通道与平台

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 |  | 19.41 | 20.32 | 57.55 | 1 | 1 | p99=20.32; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 |  | 18.57 | 18.97 | 57.77 | 1 | 1 | p99=18.97; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

