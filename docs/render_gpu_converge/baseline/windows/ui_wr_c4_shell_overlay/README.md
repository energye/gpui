# ui_wr_c4_shell_overlay

- 来源：`windows.md` §3.2（ui_wr）
- 目录：`examples/ui_wr_c4_shell_overlay`
- 运行：`R=25s`（mode=R，seconds=25，batch=False）
- 金图：`golden/c4_{steady,recover}_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：混合/缓存 · R3+R8+R21 壳浮层

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 | 0.0000% | 21.82 | 24.67 | 55.95 | 3 | 2 | p99=24.67; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 | 0.0000% | 19.66 | 27.68 | 56.52 | 3 | 2 | p99=27.68; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

