# asset

- 来源：`windows.md` §2（engine）
- 目录：`examples/engine/asset`
- 运行：`A/8s`（mode=A，seconds=8，batch=False）
- 金图：`testdata/asset_{large,reload}_golden.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：2.5D · 资源热重载/坏文件保旧

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| reload | native | run | PASS | 0 | 0 px changed | 19.27 | 19.85 | 57.55 | 0 | 2 | p99=19.85; vsync_source=true (not a source name) |
| reload | go | run | PASS | 0 | 0 px changed | 18.78 | 21.00 | 57.71 | 0 | 2 | p99=21.00; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |
| large | native | run | PASS | 0 | 0 px changed | 19.07 | 20.28 | 57.56 | 0 | 2 | p99=20.28; vsync_source=true (not a source name) |
| large | go | run | PASS | 0 | 0 px changed | 18.58 | 19.72 | 57.71 | 0 | 2 | p99=19.72; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 视觉证据说明

引擎窗以冻结金图 + window/offscreen 金图 diff 计数为视觉证据；窗自身写出 `*_final.png`/`*_last.png` 的已归档到 `snapshots/`。本轮未生成黑屏 X11 抓图（rootless Xwayland 的 x11grab 为黑，已禁用）。

