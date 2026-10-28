# ui_render_graphics/vector

- 来源：`windows.md` §3.1（render）
- 目录：`examples/ui_render_graphics/vector`
- 运行：`A/5s`（mode=A，seconds=5，batch=False）
- 金图：`testdata/showcase_vector_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：矢量 · Quad/Cubic/Arc

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | FAIL | 1 | 0.0005% | 19.71 | 21.09 | 57.58 | 1 | 1 | exit code 1; p99=21.09; vsync_source=true (not a source name) |
| default | go | run | FAIL | 1 | 0.0005% | 18.70 | 22.77 | 57.73 | 1 | 1 | exit code 1; p99=22.77; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 失败原因（原始 stderr 摘录）

- `default` / `native` / `run`：exit code 1; p99=21.09; vsync_source=true (not a source name); golden diff 0.0005%; vector: golden showcase_vector.png: diff=0.0005% over 821184 px; FAIL: golden diff 0.0005% over 821184 px
- `default` / `go` / `run`：exit code 1; p99=22.77; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 0.0005%; vector: golden showcase_vector.png: diff=0.0005% over 821184 px; FAIL: golden diff 0.0005% over 821184 px

