# ui_render_base_variablefont

- 来源：`windows.md` §3.1（render）
- 目录：`examples/ui_render_base_variablefont`
- 运行：`A/5s`（mode=A，seconds=5，batch=False）
- 金图：`testdata/showcase_variablefont_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：文本 · 可变字体

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | FAIL | 1 | 0.9517% | 19.43 | 20.08 | 57.71 | 1 | 1 | exit code 1; p99=20.08; vsync_source=true (not a source name) |
| default | go | run | FAIL | 1 | 0.9517% | 18.90 | 21.19 | 58.01 | 1 | 1 | exit code 1; p99=21.19; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 失败原因（原始 stderr 摘录）

- `default` / `native` / `run`：exit code 1; p99=20.08; vsync_source=true (not a source name); golden diff 0.9517%; hitch_rate_per_min=11.98 > 5; golden showcase_variablefont.png: diff=0.9517% over 627200 px; FAIL: pixel_golden_diff_pct=0.9517 want 0 over 627200 px
- `default` / `go` / `run`：exit code 1; p99=21.19; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 0.9517%; hitch_rate_per_min=12.00 > 5; golden showcase_variablefont.png: diff=0.9517% over 627200 px; FAIL: pixel_golden_diff_pct=0.9517 want 0 over 627200 px

