# ui_render_base_filterops

- 来源：`windows.md` §3.1（render）
- 目录：`examples/ui_render_base_filterops`
- 运行：`A/5s`（mode=A，seconds=5，batch=False）
- 金图：`testdata/showcase_filterops_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：滤镜 · BlurXY/图链/发布视图

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | FAIL | 1 | 0.0000% | 20.03 | 25.80 | 57.64 | 1 | 1 | exit code 1; p99=25.80; vsync_source=true (not a source name) |
| default | go | run | FAIL | 1 | 9.4798% | 18.87 | 20.29 | 57.83 | 1 | 1 | exit code 1; p99=20.29; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 失败原因（原始 stderr 摘录）

- `default` / `native` / `run`：exit code 1; p99=25.80; vsync_source=true (not a source name); hitch_rate_per_min=23.93 > 5; ui_render_base_filterops: logic filter_graph_skips_noop              pass=true; golden showcase_filterops.png: diff=0.0000% over 627200 px; FAIL: screen check: diff=99.2653% over 627200 px > 1.00% (grab vs snapshot)
- `default` / `go` / `run`：exit code 1; p99=20.29; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 9.4798%; ui_render_base_filterops: logic filter_graph_skips_noop              pass=true; golden showcase_filterops.png: diff=9.4798% over 627200 px; FAIL: pixel probes 18/26 ok

