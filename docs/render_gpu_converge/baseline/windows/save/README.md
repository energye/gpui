# save

- 来源：`windows.md` §2（engine）
- 目录：`examples/engine/save`
- 运行：`A`（mode=A，seconds=None，batch=False）
- 金图：`testdata/save_q123_golden.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：2.5D · 存档读写

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 | 0 px changed | 19.61 | 23.24 | 57.40 | 0 | 1 | p99=23.24; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 | 0 px changed | 19.74 | 22.78 | 57.63 | 0 | 1 | p99=22.78; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 视觉证据说明

引擎窗以冻结金图 + window/offscreen 金图 diff 计数为视觉证据；窗自身写出 `*_final.png`/`*_last.png` 的已归档到 `snapshots/`。本轮未生成黑屏 X11 抓图（rootless Xwayland 的 x11grab 为黑，已禁用）。

