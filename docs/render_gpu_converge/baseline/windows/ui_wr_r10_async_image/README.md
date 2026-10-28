# ui_wr_r10_async_image

- 来源：`windows.md` §3.2（ui_wr）
- 目录：`examples/ui_wr_r10_async_image`
- 运行：`A/30s`（mode=A，seconds=30，batch=False）
- 金图：`testdata/showcase_r10_async_image_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：图片 · 异步图→局部脏

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 | 0.0000% | 19.55 | 20.42 | 57.10 | 1 | 1 | p99=20.42; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 | 0.0000% | 18.58 | 19.19 | 57.68 | 1 | 1 | p99=19.19; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

