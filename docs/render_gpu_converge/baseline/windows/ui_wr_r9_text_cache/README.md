# ui_wr_r9_text_cache

- 来源：`windows.md` §3.2（ui_wr）
- 目录：`examples/ui_wr_r9_text_cache`
- 运行：`A/5s`（mode=A，seconds=5，batch=False）
- 金图：`testdata/showcase_r9_text_cache_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：文本/缓存 · measure 缓存

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 | 0.0000% | 19.54 | 20.47 | 57.36 | 1 | 1 | p99=20.47; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 | 0.0000% | 18.72 | 21.36 | 57.84 | 1 | 1 | p99=21.36; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

