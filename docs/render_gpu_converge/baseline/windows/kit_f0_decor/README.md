# kit_f0_decor

- 来源：`windows.md` §3.4（kit）
- 目录：`examples/kit_f0_decor`
- 运行：`A/5s`（mode=A，seconds=5，batch=False）
- 金图：`testdata/showcase_decor_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：组件/混合 · 装饰/透明度公式

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 | 0.0000% | 25.15 | 30.76 | 58.57 | 1 | 1 | p95=25.15 > 22 (P1.7 metric); p99=30.76; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 | 0.0000% | 19.71 | 23.82 | 57.92 | 1 | 1 | p99=23.82; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

