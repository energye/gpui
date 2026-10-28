# ui_render_base_svg

- 来源：`windows.md` §3.1（render）
- 目录：`examples/ui_render_base_svg`
- 运行：`A/5s`（mode=A，seconds=5，batch=False）
- 金图：`testdata/showcase_svg_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：矢量 · SVG 图标栅格化

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | FAIL | 1 | 0.0000% | 19.33 | 19.82 | 57.69 | 1 | 1 | exit code 1; p99=19.82; vsync_source=true (not a source name) |
| default | go | run | FAIL | 1 | 0.0000% | 18.47 | 19.59 | 57.88 | 1 | 1 | exit code 1; p99=19.59; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 失败原因（原始 stderr 摘录）

- `default` / `native` / `run`：exit code 1; p99=19.82; vsync_source=true (not a source name); golden showcase_svg.png: diff=0.0000% over 627200 px; FAIL: screen check: diff=100.0000% over 627200 px > 1.00% (grab vs snapshot)
- `default` / `go` / `run`：exit code 1; p99=19.59; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); hitch_rate_per_min=11.97 > 5; golden showcase_svg.png: diff=0.0000% over 627200 px; FAIL: screen check: diff=100.0000% over 627200 px > 1.00% (grab vs snapshot)

