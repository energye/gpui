# ui_base_multiwin_cap

- 来源：`windows.md` §3.3（base）
- 目录：`examples/ui_base_multiwin_cap`
- 运行：`批量(8 窗) A/75s`（mode=A，seconds=75，batch=True）
- 金图：`testdata/multiwin_win{1..8}_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：X11/native

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 | 0.0000% | 26.85 | 29.03 | 53.42 | 8 | 8 | p95=26.85 > 22 (P1.7 metric); p99=29.03; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 | 0.0000% | 21.97 | 26.35 | 56.94 | 8 | 8 | p99=26.35; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

