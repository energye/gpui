# input

- 来源：`windows.md` §2（engine）
- 目录：`examples/engine/input`
- 运行：`A/8s`（mode=A，seconds=8，batch=False）
- 金图：`testdata/input_{remap,combo}_golden.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：2.5D · 输入重映射/组合键

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| remap | native | run | PASS | 0 |  | 19.14 | 19.88 | 57.32 | 0 | 2 | p99=19.88; vsync_source=true (not a source name) |
| remap | go | run | PASS | 0 |  | 18.80 | 21.87 | 57.79 | 0 | 2 | p99=21.87; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |
| combo | native | run | PASS | 0 |  | 19.19 | 20.01 | 57.49 | 0 | 2 | p99=20.01; vsync_source=true (not a source name) |
| combo | go | run | PASS | 0 |  | 18.43 | 20.20 | 57.67 | 0 | 2 | p99=20.20; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 视觉证据说明

引擎窗以冻结金图 + window/offscreen 金图 diff 计数为视觉证据；窗自身写出 `*_final.png`/`*_last.png` 的已归档到 `snapshots/`。本轮未生成黑屏 X11 抓图（rootless Xwayland 的 x11grab 为黑，已禁用）。

