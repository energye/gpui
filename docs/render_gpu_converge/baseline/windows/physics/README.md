# physics

- 来源：`windows.md` §2（engine）
- 目录：`examples/engine/physics`
- 运行：`A/8s`（mode=A，seconds=8，batch=False）
- 金图：`testdata/physics_{hit,jump,large}_golden.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：2.5D · 射线/碰撞/跳跃

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| hit | native | run | PASS | 0 | 0 px changed | 19.13 | 20.38 | 57.38 | 0 | 3 | p99=20.38; vsync_source=true (not a source name) |
| hit | go | run | PASS | 0 | 0 px changed | 18.36 | 18.93 | 57.77 | 0 | 3 | p99=18.93; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |
| jump | native | run | PASS | 0 | 0 px changed | 19.27 | 20.94 | 57.41 | 0 | 3 | p99=20.94; vsync_source=true (not a source name) |
| jump | go | run | PASS | 0 | 0 px changed | 18.40 | 19.75 | 57.89 | 0 | 3 | p99=19.75; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |
| large | native | run | PASS | 0 | 0 px changed | 19.91 | 22.60 | 57.47 | 0 | 3 | p99=22.60; vsync_source=true (not a source name) |
| large | go | run | PASS | 0 | 0 px changed | 19.25 | 24.07 | 57.69 | 0 | 3 | p99=24.07; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 视觉证据说明

引擎窗以冻结金图 + window/offscreen 金图 diff 计数为视觉证据；窗自身写出 `*_final.png`/`*_last.png` 的已归档到 `snapshots/`。本轮未生成黑屏 X11 抓图（rootless Xwayland 的 x11grab 为黑，已禁用）。

