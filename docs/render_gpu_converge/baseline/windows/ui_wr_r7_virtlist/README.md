# ui_wr_r7_virtlist

- 来源：`windows.md` §3.2（ui_wr）
- 目录：`examples/ui_wr_r7_virtlist`
- 运行：`A/60s`（mode=A，seconds=60，batch=False）
- 金图：`testdata/r7_virtlist_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：缓存/边界 · 虚拟列表

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | FAIL | 1 |  | 24.89 | 25.18 | 59.94 | 1 | 1 | exit code 1; p95=24.89 > 22 (P1.7 metric); p99=25.18; vsync_source=true (not a source name) |
| default | go | run | FAIL | 1 |  | 22.66 | 27.36 | 59.95 | 1 | 1 | exit code 1; p95=22.66 > 22 (P1.7 metric); p99=27.36; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 失败原因（原始 stderr 摘录）

- `default` / `native` / `run`：exit code 1; p95=24.89 > 22 (P1.7 metric); p99=25.18; vsync_source=true (not a source name); hitch_rate_per_min=16.02 > 5; FAIL: interval_p95_ms=24.9 > 22
- `default` / `go` / `run`：exit code 1; p95=22.66 > 22 (P1.7 metric); p99=27.36; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); hitch_rate_per_min=15.02 > 5; FAIL: interval_p95_ms=22.7 > 22

