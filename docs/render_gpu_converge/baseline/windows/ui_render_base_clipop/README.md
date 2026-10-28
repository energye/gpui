# ui_render_base_clipop

- 来源：`windows.md` §3.1（render）
- 目录：`examples/ui_render_base_clipop`
- 运行：`A/5s`（mode=A，seconds=5，batch=False）
- 金图：`testdata/showcase_clipop_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：裁剪 · difference/replace

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 | 0.0000% | 29.90 | 35.24 | 47.28 | 1 | 1 | p95=29.90 > 22 (P1.7 metric); p99=35.24; vsync_source=true (not a source name) |
| default | go | run | FAIL | 1 | 0.0078% | 31.93 | 39.76 | 44.33 | 1 | 1 | exit code 1; p95=31.93 > 22 (P1.7 metric); p99=39.76; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 失败原因（原始 stderr 摘录）

- `default` / `go` / `run`：exit code 1; p95=31.93 > 22 (P1.7 metric); p99=39.76; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 0.0078%; hitch_rate_per_min=108.31 > 5; golden showcase_clipop.png: diff=0.0078% over 627200 px; FAIL: pixel_golden_diff_pct=0.0078 want 0 over 627200 px

