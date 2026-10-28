# ui_render_base_verticesmesh

- 来源：`windows.md` §3.1（render）
- 目录：`examples/ui_render_base_verticesmesh`
- 运行：`A/5s`（mode=A，seconds=5，batch=False）
- 金图：`testdata/showcase_verticesmesh_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：矢量 · 顶点网格

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | FAIL | 1 | 0.0000% | 19.05 | 20.18 | 57.74 | 1 | 1 | exit code 1; p99=20.18; vsync_source=true (not a source name) |
| default | go | run | FAIL | 1 | 5.6751% | 18.69 | 21.73 | 58.04 | 1 | 1 | exit code 1; p99=21.73; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 失败原因（原始 stderr 摘录）

- `default` / `native` / `run`：exit code 1; p99=20.18; vsync_source=true (not a source name); ui_render_base_verticesmesh: logic mesh_short_indices_skip                pass=true; ui_render_base_verticesmesh: logic mesh_old_bad_index_skips               pass=true; golden showcase_verticesmesh.png: diff=0.0000% over 627200 px; FAIL: screen check: diff=100.0000% over 627200 px > 1.00% (grab vs snapshot)
- `default` / `go` / `run`：exit code 1; p99=21.73; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 5.6751%; hitch_rate_per_min=12.01 > 5; ui_render_base_verticesmesh: logic mesh_short_indices_skip                pass=true; ui_render_base_verticesmesh: logic mesh_old_bad_index_skips               pass=true; golden showcase_verticesmesh.png: diff=5.6751% over 627200 px; FAIL: pixel probes 20/29 ok

