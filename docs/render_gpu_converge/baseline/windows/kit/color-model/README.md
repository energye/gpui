# kit/color-model

- 来源：`windows.md` §3.4（kit）
- 目录：`examples/kit/color-model`
- 运行：`A/5s`（mode=A，seconds=5，batch=False）
- 金图：`testdata/showcase_color_model_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：组件 · 颜色值

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | FAIL | 1 | 0.0000% | 25.63 | 27.10 | 59.02 | 1 | 1 | exit code 1; p95=25.63 > 22 (P1.7 metric); p99=27.10; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 | 0.0000% | 18.74 | 21.15 | 57.90 | 1 | 1 | p99=21.15; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 失败原因（原始 stderr 摘录）

- `default` / `native` / `run`：exit code 1; p95=25.63 > 22 (P1.7 metric); p99=27.10; vsync_source=true (not a source name); hitch_rate_per_min=11.96 > 5; golden showcase_color_model.png: diff=0.0000% over 353300 px; FAIL: FAIL: interval_p95_ms=25.63 > 22.00

