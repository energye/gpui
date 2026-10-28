# ui_text_m5_anycase

- 来源：`windows.md` §3.3（base）
- 目录：`examples/ui_text_m5_anycase`
- 运行：`A/30s`（mode=A，seconds=30，batch=False）
- 金图：`testdata/showcase_text_m5_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：X11/native

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | FAIL | 1 | 3.1630% | 25.06 | 29.08 | 59.98 | 1 | 1 | exit code 1; p95=25.06 > 22 (P1.7 metric); p99=29.08; vsync_source=true (not a source name) |
| default | go | run | FAIL | 1 | 3.1630% | 24.97 | 29.86 | 59.96 | 1 | 1 | exit code 1; p95=24.97 > 22 (P1.7 metric); p99=29.86; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 失败原因（原始 stderr 摘录）

- `default` / `native` / `run`：exit code 1; p95=25.06 > 22 (P1.7 metric); p99=29.08; vsync_source=true (not a source name); golden diff 3.1630%; hitch_rate_per_min=48.06 > 5; ui_text_m5_anycase: golden showcase_text_m5.png: diff=3.1630% over 821184 px; FAIL: pixel_golden_diff_pct=3.1630 want 0 over 821184 px
- `default` / `go` / `run`：exit code 1; p95=24.97 > 22 (P1.7 metric); p99=29.86; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 3.1630%; hitch_rate_per_min=26.06 > 5; ui_text_m5_anycase: golden showcase_text_m5.png: diff=3.1630% over 821184 px; FAIL: pixel_golden_diff_pct=3.1630 want 0 over 821184 px

