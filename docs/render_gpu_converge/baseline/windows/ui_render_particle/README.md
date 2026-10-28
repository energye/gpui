# ui_render_particle

- 来源：`windows.md` §3.6（demo）
- 目录：`examples/ui_render_particle`
- 运行：`A/15s`（mode=A，seconds=15，batch=False）
- 金图：`testdata/particle_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：混合/帧时 · 粒子特效

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 |  |  |  |  | 1 | 1 | vsync_source=true (not a source name) |
| default | go | run | PASS | 0 |  |  |  |  | 1 | 1 | vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

