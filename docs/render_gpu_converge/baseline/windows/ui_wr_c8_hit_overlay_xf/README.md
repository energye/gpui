# ui_wr_c8_hit_overlay_xf

- 来源：`windows.md` §3.2（ui_wr）
- 目录：`examples/ui_wr_c8_hit_overlay_xf`
- 运行：`A/15s`（mode=A，seconds=15，batch=False）
- 金图：`testdata/c8_recover_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：命中/混合 · R13+R6+R8

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 |  | 19.23 | 24.51 | 56.95 | 2 | 1 | p99=24.51; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 |  | 18.66 | 20.64 | 57.24 | 2 | 1 | p99=20.64; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

