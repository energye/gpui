# kit_f0_content

- 来源：`windows.md` §3.4（kit）
- 目录：`examples/kit_f0_content`
- 运行：`A/15s`（mode=A，seconds=15，batch=False）
- 金图：`testdata/showcase_content_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：组件/文本/图片 · 单/多行/省略/富文本/图标/图片三态

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 | 0.0000% | 19.16 | 19.80 | 57.90 | 1 | 1 | p99=19.80; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 | 0.0000% | 18.77 | 20.41 | 57.72 | 1 | 1 | p99=20.41; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

