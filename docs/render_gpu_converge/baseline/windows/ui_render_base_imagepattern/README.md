# ui_render_base_imagepattern

- 来源：`windows.md` §3.1（render）
- 目录：`examples/ui_render_base_imagepattern`
- 运行：`A/5s`（mode=A，seconds=5，batch=False）
- 金图：`testdata/showcase_imagepattern_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：图片 · 旧 Pattern/画笔

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 | 0.0000% | 20.69 | 26.40 | 57.62 | 1 | 1 | p99=26.40; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 | 0.0000% | 18.44 | 19.99 | 57.76 | 1 | 1 | p99=19.99; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

