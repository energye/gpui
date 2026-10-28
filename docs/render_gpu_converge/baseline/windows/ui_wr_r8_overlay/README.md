# ui_wr_r8_overlay

- 来源：`windows.md` §3.2（ui_wr）
- 目录：`examples/ui_wr_r8_overlay`
- 运行：`A/15s`（mode=A，seconds=15，batch=False）
- 金图：`testdata/showcase_r8_overlay_{open,closed}_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：混合 · 浮层独立合成双态

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| open | native | run | PASS | 0 | 0.0000% | 20.75 | 24.94 | 59.93 | 1 | 2 | p99=24.94; vsync_source=true (not a source name) |
| open | go | run | FAIL | 1 | 0.0000% | 22.24 | 25.14 | 60.04 | 1 | 2 | exit code 1; p95=22.24 > 22 (P1.7 metric); p99=25.14; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |
| closed | native | run | FAIL | 1 | 0.0000% | 22.07 | 25.19 | 60.02 | 2 | 2 | exit code 1; p95=22.07 > 22 (P1.7 metric); p99=25.19; vsync_source=true (not a source name) |
| closed | go | run | PASS | 0 | 0.0000% | 19.84 | 22.32 | 60.00 | 2 | 2 | p99=22.32; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 失败原因（原始 stderr 摘录）

- `open` / `go` / `run`：exit code 1; p95=22.24 > 22 (P1.7 metric); p99=25.14; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); hitch_rate_per_min=8.04 > 5; ui_wr_r8_overlay: golden showcase_r8_overlay_open.png: diff=0.0000% over 821184 px; FAIL: FAIL: interval_p95_ms=22.24 > 22.00
- `closed` / `native` / `run`：exit code 1; p95=22.07 > 22 (P1.7 metric); p99=25.19; vsync_source=true (not a source name); ui_wr_r8_overlay: golden showcase_r8_overlay_closed.png: diff=0.0000% over 821184 px; FAIL: FAIL: interval_p95_ms=22.07 > 22.00

