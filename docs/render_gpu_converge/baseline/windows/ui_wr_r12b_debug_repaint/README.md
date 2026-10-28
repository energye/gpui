# ui_wr_r12b_debug_repaint

- 来源：`windows.md` §3.2（ui_wr）
- 目录：`examples/ui_wr_r12b_debug_repaint`
- 运行：`A/8s`（mode=A，seconds=8，batch=False）
- 金图：`testdata/showcase_r12b_debug_{on,off}_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：缓存/边界 · 重绘调试 on/off

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| on | native | run | PASS | 0 | 0.0000% | 19.68 | 22.42 | 57.35 | 1 | 2 | p99=22.42; vsync_source=true (not a source name) |
| on | go | run | PASS | 0 | 0.0000% | 18.47 | 20.41 | 57.69 | 1 | 2 | p99=20.41; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |
| off | native | run | PASS | 0 | 0.0000% | 19.88 | 24.76 | 57.07 | 2 | 2 | p99=24.76; vsync_source=true (not a source name) |
| off | go | run | PASS | 0 | 0.0000% | 19.15 | 20.91 | 57.84 | 2 | 2 | p99=20.91; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

