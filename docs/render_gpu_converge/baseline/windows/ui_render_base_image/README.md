# ui_render_base_image

- 来源：`windows.md` §3.1（render）
- 目录：`examples/ui_render_base_image`
- 运行：`A/15s`（mode=A，seconds=15，batch=False）
- 金图：`testdata/showcase_image_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：图片 · Image 轴

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 | 0.0000% | 19.48 | 21.65 | 57.38 | 1 | 1 | p99=21.65; vsync_source=true (not a source name) |
| default | go | run | FAIL | 1 | 0.2877% | 18.64 | 19.60 | 57.71 | 1 | 1 | exit code 1; p99=19.60; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 失败原因（原始 stderr 摘录）

- `default` / `go` / `run`：exit code 1; p99=19.60; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 0.2877%; ui_render_base_image: avg=17.33 p50=17.35 p99=19.60 hitches=0 vsync=true; ui_render_base_image: golden showcase_image.png: diff=0.2877% over 499200 px; FAIL: golden diff 0.2877% over 499200 px

