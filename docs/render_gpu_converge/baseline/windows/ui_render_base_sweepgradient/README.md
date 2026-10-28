# ui_render_base_sweepgradient

- 来源：`windows.md` §3.1（render）
- 目录：`examples/ui_render_base_sweepgradient`
- 运行：`A/5s`（mode=A，seconds=5，batch=False）
- 金图：`testdata/showcase_sweepgradient_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：混合 · 锥形渐变

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 | 0.0000% | 20.01 | 26.02 | 57.68 | 1 | 1 | p99=26.02; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 | 0.0000% | 18.60 | 22.28 | 57.53 | 1 | 1 | p99=22.28; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

