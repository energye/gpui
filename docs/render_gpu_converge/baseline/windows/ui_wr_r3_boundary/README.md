# ui_wr_r3_boundary

- 来源：`windows.md` §3.2（ui_wr）
- 目录：`examples/ui_wr_r3_boundary`
- 运行：`A/10s`（mode=A，seconds=10，batch=False）
- 金图：`testdata/showcase_r3_boundary_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：缓存/边界 · 嵌套 Boundary

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 | 0.0000% | 19.43 | 20.52 | 57.43 | 1 | 1 | p99=20.52; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 | 0.0000% | 18.52 | 20.06 | 57.67 | 1 | 1 | p99=20.06; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

