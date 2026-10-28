# camera

- 来源：`windows.md` §2（engine）
- 目录：`examples/engine/camera`
- 运行：`A/8s`（mode=A，seconds=8，batch=False）
- 金图：`testdata/{camera_follow,voices}_golden.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：2.5D · 相机跟随/限位/远带

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| follow | native | run | PASS | 0 | 0 px changed | 19.42 | 22.16 | 57.53 | 0 | 2 | p99=22.16; vsync_source=true (not a source name) |
| follow | go | run | PASS | 0 | 0 px changed | 18.72 | 20.64 | 57.83 | 0 | 2 | p99=20.64; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |
| voices | native | run | PASS | 0 | 0 px changed | 19.80 | 22.73 | 57.39 | 1 | 2 | p99=22.73; vsync_source=true (not a source name) |
| voices | go | run | PASS | 0 | 0 px changed | 18.73 | 20.19 | 57.72 | 1 | 2 | p99=20.19; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 视觉证据说明

引擎窗以冻结金图 + window/offscreen 金图 diff 计数为视觉证据；窗自身写出 `*_final.png`/`*_last.png` 的已归档到 `snapshots/`。本轮未生成黑屏 X11 抓图（rootless Xwayland 的 x11grab 为黑，已禁用）。

