# kit/icon

- 来源：`windows.md` §3.4（kit）
- 目录：`examples/kit/icon`
- 运行：`A/5s`（mode=A，seconds=5，batch=False）
- 金图：`testdata/showcase_icon_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：组件 · 图标

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | FAIL | 1 | 0.0000% | 20.71 | 27.03 | 58.13 | 1 | 1 | exit code 1; p99=27.03; vsync_source=true (not a source name) |
| default | go | run | FAIL | 1 | 3.3203% | 18.91 | 21.76 | 57.83 | 1 | 1 | exit code 1; p99=21.76; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 失败原因（原始 stderr 摘录）

- `default` / `native` / `run`：exit code 1; p99=27.03; vsync_source=true (not a source name); hitch_rate_per_min=24.30 > 5; golden showcase_icon.png: diff=0.0000% over 1024 px; FAIL: FAIL: damage_ratio=1.0000 > max 0.3500 (retained must not full-screen damage)
- `default` / `go` / `run`：exit code 1; p99=21.76; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 3.3203%; hitch_rate_per_min=12.18 > 5; golden showcase_icon.png: diff=3.3203% over 1024 px; FAIL: FAIL: damage_ratio=1.0000 > max 0.3500 (retained must not full-screen damage)

