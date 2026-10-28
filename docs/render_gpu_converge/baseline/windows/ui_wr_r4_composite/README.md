# ui_wr_r4_composite

- 来源：`windows.md` §3.2（ui_wr）
- 目录：`examples/ui_wr_r4_composite`
- 运行：`A/15s`（mode=A，seconds=15，batch=False）
- 金图：`testdata/showcase_r4_composite_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：缓存/边界 · 层合成分带

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 | 0.0000% | 19.40 | 20.71 | 57.28 | 1 | 1 | p99=20.71; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 | 0.0000% | 18.56 | 19.87 | 57.67 | 1 | 1 | p99=19.87; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

