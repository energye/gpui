# ui_render_base_pathlow

- 来源：`windows.md` §3.1（render）
- 目录：`examples/ui_render_base_pathlow`
- 运行：`A/5s`（mode=A，seconds=5，batch=False）
- 金图：`testdata/showcase_pathlow_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：矢量 · 路径低频三 API

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 | 0.0000% | 19.34 | 23.92 | 57.53 | 1 | 1 | p99=23.92; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 | 0.0000% | 18.55 | 20.79 | 58.03 | 1 | 1 | p99=20.79; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

