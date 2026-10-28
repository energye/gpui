# kit_f0_interact

- 来源：`windows.md` §3.4（kit）
- 目录：`examples/kit_f0_interact`
- 运行：`A/15s`（mode=A，seconds=15，batch=False）
- 金图：`testdata/showcase_interact_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：组件/命中 · hover/禁用/浮层

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 | 0.0000% | 18.99 | 19.92 | 57.80 | 1 | 1 | p99=19.92; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 | 0.0000% | 18.68 | 19.56 | 57.68 | 1 | 1 | p99=19.56; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

