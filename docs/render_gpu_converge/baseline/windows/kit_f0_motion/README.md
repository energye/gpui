# kit_f0_motion

- 来源：`windows.md` §3.4（kit）
- 目录：`examples/kit_f0_motion`
- 运行：`A/30s`（mode=A，seconds=30，batch=False）
- 金图：`testdata/showcase_motion_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：组件/帧时 · 动效

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 | 0.0000% | 19.38 | 20.69 | 57.73 | 1 | 1 | p99=20.69; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 | 0.0000% | 18.25 | 19.06 | 58.13 | 1 | 1 | p99=19.06; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

