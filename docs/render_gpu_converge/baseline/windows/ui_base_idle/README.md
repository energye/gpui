# ui_base_idle

- 来源：`windows.md` §3.3（base）
- 目录：`examples/ui_base_idle`
- 运行：`A/60s`（mode=A，seconds=60，batch=False）
- 金图：`testdata/showcase_idle_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：X11/native

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 | 0.0000% | 0 | 0 |  | 1 | 1 | p99=0.00 |
| default | go | run | PASS | 0 | 0.0000% | 0 | 0 |  | 1 | 1 | p99=0.00; GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

