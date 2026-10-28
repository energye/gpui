# ui_wr_r21_shell

- 来源：`windows.md` §3.2（ui_wr）
- 目录：`examples/ui_wr_r21_shell`
- 运行：`A/15s`（mode=A，seconds=15，batch=False）
- 金图：`testdata/r21_shell_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：缓存/边界 · 壳/内容分层

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 |  | 18.82 | 24.52 | 57.35 | 1 | 1 | p99=24.52; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 |  | 20.15 | 22.81 | 57.62 | 1 | 1 | p99=22.81; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

