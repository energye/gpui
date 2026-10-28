# kit/notice-queue

- 来源：`windows.md` §3.4（kit）
- 目录：`examples/kit/notice-queue`
- 运行：`A/5s`（mode=A，seconds=5，batch=False）
- 金图：`testdata/showcase_notice_queue_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：组件 · 通知队列

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | FAIL | 1 | 0.0000% | 25.94 | 32.05 | 58.55 | 1 | 1 | exit code 1; p95=25.94 > 22 (P1.7 metric); p99=32.05; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 | 0.0000% | 19.50 | 23.13 | 57.72 | 1 | 1 | p99=23.13; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 失败原因（原始 stderr 摘录）

- `default` / `native` / `run`：exit code 1; p95=25.94 > 22 (P1.7 metric); p99=32.05; vsync_source=true (not a source name); hitch_rate_per_min=47.80 > 5; golden showcase_notice_queue.png: diff=0.0000% over 281960 px; FAIL: FAIL: interval_p95_ms=25.94 > 22.00

