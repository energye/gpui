# ui_render_base_webp

- 来源：`windows.md` §3.1（render）
- 目录：`examples/ui_render_base_webp`
- 运行：`A/5s`（mode=A，seconds=5，batch=False）
- 金图：`testdata/showcase_webp_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：图片 · WebP 有损/无损/缩放

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | FAIL | 1 | 0.0000% | 20.20 | 24.21 | 57.54 | 1 | 1 | exit code 1; p99=24.21; vsync_source=true (not a source name) |
| default | go | run | FAIL | 1 | 0.0000% | 19.27 | 22.27 | 57.89 | 1 | 1 | exit code 1; p99=22.27; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 失败原因（原始 stderr 摘录）

- `default` / `native` / `run`：exit code 1; p99=24.21; vsync_source=true (not a source name); hitch_rate_per_min=11.99 > 5; ui_render_base_webp: logic webp_empty_error                 pass=true; ui_render_base_webp: pixel pt_d_red_badge                 ok=true D red error badge got=(0.859,0.200,0.180) want=(0.860,0.200,0.180) Δ=0.0012 tol=0.0157; golden showcase_webp.png: diff=0.0000% over 627200 px; FAIL: screen check: diff=100.0000% over 627200 px > 1.00% (grab vs snapshot)
- `default` / `go` / `run`：exit code 1; p99=22.27; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); hitch_rate_per_min=12.02 > 5; ui_render_base_webp: logic webp_empty_error                 pass=true; ui_render_base_webp: pixel pt_d_red_badge                 ok=true D red error badge got=(0.859,0.200,0.180) want=(0.860,0.200,0.180) Δ=0.0012 tol=0.0157; golden showcase_webp.png: diff=0.0000% over 627200 px; FAIL: screen check: diff=100.0000% over 627200 px > 1.00% (grab vs snapshot)

