# ui_wr_c3_list_scroll

- 来源：`windows.md` §3.2（ui_wr）
- 目录：`examples/ui_wr_c3_list_scroll`
- 运行：`A/60s`（mode=A，seconds=60，batch=False）
- 金图：`testdata/showcase_c3_list_scroll_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：缓存/图片 · 虚拟列表+滚复用+异步图

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | FAIL | 1 | 0.0000% | 26.43 | 32.04 | 54.66 | 1 | 1 | exit code 1; p95=26.43 > 22 (P1.7 metric); p99=32.04; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 | 0.0000% | 18.95 | 20.87 | 57.57 | 1 | 1 | p99=20.87; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 失败原因（原始 stderr 摘录）

- `default` / `native` / `run`：exit code 1; p95=26.43 > 22 (P1.7 metric); p99=32.04; vsync_source=true (not a source name); hitch_rate_per_min=52.00 > 5; ui_wr_c3_list_scroll: golden showcase_c3_list_scroll.png: diff=0.0000% over 821184 px; FAIL: logic probes 8/11 bad=[fps_interval_ge_55 p95_le_22 hitch_le_budget]

