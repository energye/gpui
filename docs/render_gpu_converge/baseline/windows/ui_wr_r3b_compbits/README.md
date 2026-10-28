# ui_wr_r3b_compbits

- 来源：`windows.md` §3.2（ui_wr）
- 目录：`examples/ui_wr_r3b_compbits`
- 运行：`A/8s`（mode=A，seconds=8，batch=False）
- 金图：`testdata/showcase_r3b_compbits_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：缓存/边界 · 合成位/边界发现

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 | 0.0000% | 19.38 | 20.48 | 57.33 | 1 | 1 | p99=20.48; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 | 0.0000% | 19.16 | 20.48 | 57.81 | 1 | 1 | p99=20.48; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

