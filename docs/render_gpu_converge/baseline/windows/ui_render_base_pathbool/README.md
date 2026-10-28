# ui_render_base_pathbool

- 来源：`windows.md` §3.1（render）
- 目录：`examples/ui_render_base_pathbool`
- 运行：`A/5s`（mode=A，seconds=5，batch=False）
- 金图：`testdata/showcase_pathbool_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：矢量 · Path.Op 布尔

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | FAIL | 1 | 0.0000% | 19.93 | 25.03 | 57.72 | 1 | 1 | exit code 1; p99=25.03; vsync_source=true (not a source name) |
| default | go | run | FAIL | 1 | 0.0000% | 18.61 | 22.66 | 57.84 | 1 | 1 | exit code 1; p99=22.66; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 失败原因（原始 stderr 摘录）

- `default` / `native` / `run`：exit code 1; p99=25.03; vsync_source=true (not a source name); hitch_rate_per_min=11.98 > 5; golden showcase_pathbool.png: diff=0.0000% over 627200 px; FAIL: screen check: diff=100.0000% over 627200 px > 1.00% (grab vs snapshot)
- `default` / `go` / `run`：exit code 1; p99=22.66; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); hitch_rate_per_min=11.97 > 5; golden showcase_pathbool.png: diff=0.0000% over 627200 px; FAIL: screen check: diff=100.0000% over 627200 px > 1.00% (grab vs snapshot)

