# ui_wr_c0_smoke

- 来源：`windows.md` §3.2（ui_wr）
- 目录：`examples/ui_wr_c0_smoke`
- 运行：`A/5s`（mode=A，seconds=5，batch=False）
- 金图：`testdata/showcase_c0_smoke_{pre,post}_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：缓存/边界/帧时 · R0+R12+R16 集成

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 | 0.0000% | 18.02 | 19.50 | 57.82 | 1 | 2 | p99=19.50; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 | 0.0000% | 18.53 | 20.57 | 57.91 | 1 | 2 | p99=20.57; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

