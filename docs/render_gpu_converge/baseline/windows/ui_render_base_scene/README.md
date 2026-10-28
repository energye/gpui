# ui_render_base_scene

- 来源：`windows.md` §3.1（render）
- 目录：`examples/ui_render_base_scene`
- 运行：`A/5s`（mode=A，seconds=5，batch=False）
- 金图：`testdata/showcase_scene_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：图片 · CPU tile/GPU 场景双路

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 | 0.0000% | 19.53 | 24.73 | 57.61 | 1 | 1 | p99=24.73; vsync_source=true (not a source name) |
| default | go | run | FAIL | 1 | 0.0032% | 18.52 | 21.37 | 58.07 | 1 | 1 | exit code 1; p99=21.37; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 失败原因（原始 stderr 摘录）

- `default` / `go` / `run`：exit code 1; p99=21.37; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 0.0032%; golden showcase_scene.png: diff=0.0032% over 627200 px; FAIL: pixel_golden_diff_pct=0.0032 want 0 over 627200 px

