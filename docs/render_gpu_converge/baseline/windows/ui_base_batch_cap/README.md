# ui_base_batch_cap

- 来源：`windows.md` §3.3（base）
- 目录：`examples/ui_base_batch_cap`
- 运行：`A/200s`（mode=A，seconds=200，batch=False）
- 金图：`testdata/batch_cap_{prims,vector}_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：X11/native

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | FAIL | 1 | 0.0000% | 53.11 | 56.38 | 47.20 | 2 | 2 | exit code 1; p95=53.11 > 22 (P1.7 metric); p99=56.38; vsync_source=true (not a source name) |
| default | go | run | FAIL | 1 | 0.0169% | 63.15 | 70.18 | 47.08 | 2 | 2 | exit code 1; p95=63.15 > 22 (P1.7 metric); p99=70.18; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 失败原因（原始 stderr 摘录）

- `default` / `native` / `run`：exit code 1; p95=53.11 > 22 (P1.7 metric); p99=56.38; vsync_source=true (not a source name); hitch_rate_per_min=244.72 > 5; ui_base_batch_cap: golden examples/ui_base_batch_cap/testdata/batch_cap_prims_base.png: diff=0.0000% over 883200 px; ui_base_batch_cap: golden examples/ui_base_batch_cap/testdata/batch_cap_vector_base.png: diff=0.0000% over 883200 px; FAIL: vector floor failed: max_60fps_paths=0 want >=100 (steps=prims/1000:57fps:19ms, prims/5000:57fps:20ms, prims/10000:57fps:22ms, vector/100:57fps:23ms, vector/300:34fps:33ms, vector/600:21fps:54ms)
- `default` / `go` / `run`：exit code 1; p95=63.15 > 22 (P1.7 metric); p99=70.18; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 0.0169%; hitch_rate_per_min=239.91 > 5; ui_base_batch_cap: golden examples/ui_base_batch_cap/testdata/batch_cap_prims_base.png: diff=0.0000% over 883200 px; ui_base_batch_cap: golden examples/ui_base_batch_cap/testdata/batch_cap_vector_base.png: diff=0.0169% over 883200 px; FAIL: vector/100 meets 60fps but cpu=90.3% > 85% health line

