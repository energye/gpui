# kit_f0_theme

- 来源：`windows.md` §3.4（kit）
- 目录：`examples/kit_f0_theme`
- 运行：`A/10s`（mode=A，seconds=10，batch=False）
- 金图：`testdata/showcase_theme_{default,skinned,compact}_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：组件 · 三主题

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 | 0.0000% | 19.17 | 20.18 | 58.18 | 1 | 3 | p99=20.18; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 | 0.0000% | 18.33 | 19.10 | 57.74 | 1 | 3 | p99=19.10; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

