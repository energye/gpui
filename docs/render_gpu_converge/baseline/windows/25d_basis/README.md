# 25d_basis

- 来源：`windows.md` §2（engine）
- 目录：`examples/engine/25d_basis`
- 运行：`A/10s`（mode=A，seconds=10，batch=False）
- 金图：`testdata/basis_view_golden.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：2.5D · 六视角基向量/排序/影子

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 | 0 px changed | 19.29 | 20.17 | 57.39 | 0 | 1 | p99=20.17; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 | 0 px changed | 19.05 | 21.25 | 57.66 | 0 | 1 | p99=21.25; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 视觉证据说明

引擎窗以冻结金图 + window/offscreen 金图 diff 计数为视觉证据；窗自身写出 `*_final.png`/`*_last.png` 的已归档到 `snapshots/`。本轮未生成黑屏 X11 抓图（rootless Xwayland 的 x11grab 为黑，已禁用）。

