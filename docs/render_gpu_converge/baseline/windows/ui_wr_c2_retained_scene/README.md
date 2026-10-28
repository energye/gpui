# ui_wr_c2_retained_scene

- 来源：`windows.md` §3.2（ui_wr）
- 目录：`examples/ui_wr_c2_retained_scene`
- 运行：`A/15s`（mode=A，seconds=15，batch=False）
- 金图：`testdata/showcase_c2_retained_scene_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：缓存/录制 · R3+R4+R4b+R5

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 | 0.0000% | 18.07 | 18.51 | 57.29 | 1 | 1 | p99=18.51; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 | 0.0000% | 18.49 | 19.74 | 57.72 | 1 | 1 | p99=19.74; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

