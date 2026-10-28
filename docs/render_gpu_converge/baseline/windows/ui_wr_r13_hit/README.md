# ui_wr_r13_hit

- 来源：`windows.md` §3.2（ui_wr）
- 目录：`examples/ui_wr_r13_hit`
- 运行：`A/5s`（mode=A，seconds=5，batch=False）
- 金图：`testdata/showcase_r13_hit_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：命中/交互 · Hit ≡ 绘

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 | 0.0000% | 19.80 | 25.10 | 57.74 | 1 | 1 | p99=25.10; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 | 0.0000% | 18.54 | 19.71 | 57.93 | 1 | 1 | p99=19.71; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

