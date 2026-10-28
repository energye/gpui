# ui_l1_render_matrix

- 来源：`windows.md` §3.3（base）
- 目录：`examples/ui_l1_render_matrix`
- 运行：`A/15s`（mode=A，seconds=15，batch=False）
- 金图：`testdata/showcase_l1_render_matrix_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：X11/native

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | FAIL | 1 |  |  |  |  | 1 | 1 | exit code 1; vsync_source=true (not a source name); missing A fields: fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_rate_per_min,target_hz |
| default | go | run | FAIL | 1 |  |  |  |  | 1 | 1 | exit code 1; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); missing A fields: fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,target_hz |

## 失败原因（原始 stderr 摘录）

- `default` / `native` / `run`：exit code 1; vsync_source=true (not a source name); missing A fields: fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_rate_per_min,target_hz; ui_l1_render_matrix: avg=17.39ms max=27.64ms p50=17.38 p99=20.10 hitches=0 vsync=true; ui_l1_render_matrix: golden diff=1.5271% over 288000 px; FAIL: pixel_golden_diff_pct=1.5271 want 0 over 288000 px
- `default` / `go` / `run`：exit code 1; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); missing A fields: fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,target_hz; ui_l1_render_matrix: avg=17.29ms max=38.24ms p50=17.47 p99=21.24 hitches=1 vsync=true; ui_l1_render_matrix: golden diff=1.5271% over 288000 px; FAIL: pixel_golden_diff_pct=1.5271 want 0 over 288000 px

