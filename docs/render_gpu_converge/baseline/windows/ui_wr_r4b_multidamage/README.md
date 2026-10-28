# ui_wr_r4b_multidamage

- 来源：`windows.md` §3.2（ui_wr）
- 目录：`examples/ui_wr_r4b_multidamage`
- 运行：`A/15s`（mode=A，seconds=15，batch=False）
- 金图：`testdata/r4b_multidamage_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：缓存/边界 · DirtyLayerID/多 damage

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 |  | 19.57 | 20.91 | 57.21 | 1 | 1 | p99=20.91; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 |  | 18.48 | 19.94 | 57.53 | 1 | 1 | p99=19.94; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

