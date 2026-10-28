# kit_f0_layout

- 来源：`windows.md` §3.4（kit）
- 目录：`examples/kit_f0_layout`
- 运行：`A/5s`（mode=A，seconds=5，batch=False）
- 金图：`testdata/showcase_layout_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：组件 · 行/叠放/换行布局

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | FAIL | 1 | 0.0000% | 25.28 | 34.69 | 58.70 | 1 | 1 | exit code 1; p95=25.28 > 22 (P1.7 metric); p99=34.69; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 | 0.0000% | 19.72 | 23.67 | 57.68 | 1 | 1 | p99=23.67; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 失败原因（原始 stderr 摘录）

- `default` / `native` / `run`：exit code 1; p95=25.28 > 22 (P1.7 metric); p99=34.69; vsync_source=true (not a source name); hitch_rate_per_min=59.90 > 5; golden showcase_layout.png: diff=0.0000% over 255040 px; FAIL: FAIL: interval_p95_ms=25.28 > 22.00

