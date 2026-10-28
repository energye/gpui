# ui_render_base_geometry

- 来源：`windows.md` §3.1（render）
- 目录：`examples/ui_render_base_geometry`
- 运行：`A/15s`（mode=A，seconds=15，batch=False）
- 金图：`testdata/showcase_geometry_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：矢量 · Rect/RRect/线/圆/渐变/Path

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 | 0.0000% | 17.44 | 18.49 | 59.94 | 1 | 1 | p99=18.49; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 | 0.0000% | 17.16 | 18.03 | 59.95 | 1 | 1 | p99=18.03; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

