# ui_render_base_dither

- 来源：`windows.md` §3.1（render）
- 目录：`examples/ui_render_base_dither`
- 运行：`A/5s`（mode=A，seconds=5，batch=False）
- 金图：`testdata/showcase_dither_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：混合 · SetDither

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | FAIL | 1 | 0.0000% | 19.33 | 20.82 | 57.75 | 1 | 1 | exit code 1; p99=20.82; vsync_source=true (not a source name) |
| default | go | run | FAIL | 1 | 0.0000% | 19.12 | 20.98 | 57.94 | 1 | 1 | exit code 1; p99=20.98; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 失败原因（原始 stderr 摘录）

- `default` / `native` / `run`：exit code 1; p99=20.82; vsync_source=true (not a source name); hitch_rate_per_min=23.98 > 5; golden showcase_dither.png: diff=0.0000% over 627200 px; FAIL: screen check: diff=100.0000% over 627200 px > 1.00% (grab vs snapshot)
- `default` / `go` / `run`：exit code 1; p99=20.98; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden showcase_dither.png: diff=0.0000% over 627200 px; FAIL: screen check: diff=100.0000% over 627200 px > 1.00% (grab vs snapshot)

