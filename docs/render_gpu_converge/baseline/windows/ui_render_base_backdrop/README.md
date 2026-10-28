# ui_render_base_backdrop

- 来源：`windows.md` §3.1（render）
- 目录：`examples/ui_render_base_backdrop`
- 运行：`A/5s`（mode=A，seconds=5，batch=False）
- 金图：`testdata/showcase_backdrop_base.png`、`…_msaa4_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：滤镜 · Backdrop 模糊/4x MSAA

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| msaa1 | native | run | FAIL | 1 | 0.0000% | 137.18 | 146.14 | 8.49 | 1 | 2 | exit code 1; p95=137.18 > 22 (P1.7 metric); p99=146.14; vsync_source=true (not a source name) |
| msaa1 | go | run | FAIL | 1 | 1.2583% | 151.58 | 161.51 | 7.27 | 1 | 2 | exit code 1; p95=151.58 > 22 (P1.7 metric); p99=161.51; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |
| msaa4 | native | run | FAIL | 1 | 0.0000% | 132.60 | 143.28 | 8.42 | 2 | 2 | exit code 1; p95=132.60 > 22 (P1.7 metric); p99=143.28; vsync_source=true (not a source name) |
| msaa4 | go | run | FAIL | 1 | 52.7042% | 154.91 | 161.80 | 7.19 | 2 | 2 | exit code 1; p95=154.91 > 22 (P1.7 metric); p99=161.80; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 失败原因（原始 stderr 摘录）

- `msaa1` / `native` / `run`：exit code 1; p95=137.18 > 22 (P1.7 metric); p99=146.14; vsync_source=true (not a source name); hitch_rate_per_min=509.27 > 5; golden showcase_backdrop.png: diff=0.0000% over 627200 px; FAIL: screen check: diff=100.0000% over 627200 px > 1.00% (grab vs snapshot)
- `msaa1` / `go` / `run`：exit code 1; p95=151.58 > 22 (P1.7 metric); p99=161.51; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 1.2583%; hitch_rate_per_min=436.36 > 5; golden showcase_backdrop.png: diff=1.2583% over 627200 px; FAIL: pixel_golden_diff_pct=1.2583 want 0 over 627200 px
- `msaa4` / `native` / `run`：exit code 1; p95=132.60 > 22 (P1.7 metric); p99=143.28; vsync_source=true (not a source name); hitch_rate_per_min=504.91 > 5; golden showcase_backdrop_msaa4.png: diff=0.0000% over 627200 px; FAIL: screen check: diff=100.0000% over 627200 px > 1.00% (grab vs snapshot)
- `msaa4` / `go` / `run`：exit code 1; p95=154.91 > 22 (P1.7 metric); p99=161.80; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 52.7042%; hitch_rate_per_min=431.57 > 5; golden showcase_backdrop_msaa4.png: diff=52.7042% over 627200 px; FAIL: pixel probes 17/21 ok

