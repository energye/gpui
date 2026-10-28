# ui_wr_r5_picture

- 来源：`windows.md` §3.2（ui_wr）
- 目录：`examples/ui_wr_r5_picture`
- 运行：`A/5s`（mode=A，seconds=5，batch=False）
- 金图：`testdata/showcase_r5_picture_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：录制 · Picture 录/回放

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | FAIL | 1 | 0.0217% | 19.65 | 22.24 | 57.50 | 1 | 1 | exit code 1; p99=22.24; vsync_source=true (not a source name) |
| default | go | run | FAIL | 1 | 0.0217% | 19.57 | 21.33 | 57.71 | 1 | 1 | exit code 1; p99=21.33; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 失败原因（原始 stderr 摘录）

- `default` / `native` / `run`：exit code 1; p99=22.24; vsync_source=true (not a source name); golden diff 0.0217%; hitch_rate_per_min=12.02 > 5; golden showcase_r5_picture.png: diff=0.0217% over 821184 px; FAIL: logic probes 5/6 bad=[direct_replay_eq]
- `default` / `go` / `run`：exit code 1; p99=21.33; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 0.0217%; hitch_rate_per_min=11.98 > 5; golden showcase_r5_picture.png: diff=0.0217% over 821184 px; FAIL: logic probes 5/6 bad=[direct_replay_eq]

