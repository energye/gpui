# step

- 来源：`windows.md` §2（engine）
- 目录：`examples/engine/step`
- 运行：`A/8s`（mode=A，seconds=8，batch=False）
- 金图：`testdata/step_dirty_golden.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：2.5D · 脏区（只更脏区）

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 |  | 19.36 | 20.33 | 57.43 | 0 | 1 | p99=20.33; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 |  | 18.51 | 19.34 | 57.78 | 0 | 1 | p99=19.34; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 视觉证据说明

引擎窗以冻结金图 + window/offscreen 金图 diff 计数为视觉证据；窗自身写出 `*_final.png`/`*_last.png` 的已归档到 `snapshots/`。本轮未生成黑屏 X11 抓图（rootless Xwayland 的 x11grab 为黑，已禁用）。

