# ui_wr_c5_anim_over_static

- 来源：`windows.md` §3.2（ui_wr）
- 目录：`examples/ui_wr_c5_anim_over_static`
- 运行：`R=30s`（mode=R，seconds=30，batch=False）
- 金图：`golden/c5_final_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：混合/缓存 · R6+R3+R4 动画盖静态

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 | 0.0000% | 19.93 | 21.61 | 57.21 | 1 | 1 | p99=21.61; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 | 0.0000% | 18.57 | 20.66 | 57.68 | 1 | 1 | p99=20.66; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

