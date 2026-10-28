# ui_render_base_cliplayer

- 来源：`windows.md` §3.1（render）
- 目录：`examples/ui_render_base_cliplayer`
- 运行：`A/15s`（mode=A，seconds=15，batch=False）
- 金图：`testdata/showcase_cliplayer_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：裁剪 · Boundary/Clip/Opacity/Transform

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 | 0.0000% | 18.25 | 21.92 | 59.67 | 1 | 1 | p99=21.92; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 | 0.0000% | 17.13 | 18.24 | 60.05 | 1 | 1 | p99=18.24; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

