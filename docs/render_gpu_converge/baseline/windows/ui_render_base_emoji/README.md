# ui_render_base_emoji

- 来源：`windows.md` §3.1（render）
- 目录：`examples/ui_render_base_emoji`
- 运行：`A/5s`（mode=A，seconds=5，batch=False）
- 金图：`testdata/showcase_emoji_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：文本 · CBDT/CBLC + COLR 合成

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 | 0.0000% | 19.77 | 23.07 | 57.62 | 1 | 1 | p99=23.07; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 | 0.0000% | 18.43 | 19.42 | 57.91 | 1 | 1 | p99=19.42; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

