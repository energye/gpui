# tex

- 来源：`windows.md` §2（engine）
- 目录：`examples/engine/tex`
- 运行：`A/8s`（mode=A，seconds=8，batch=False）
- 金图：`testdata/tex_{far,stream}_golden.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：2.5D · mipmap/流式/缺图

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| far | native | run | PASS | 0 | 0 px changed | 25.13 | 30.41 | 56.35 | 0 | 2 | p95=25.13 > 22 (P1.7 metric); p99=30.41; vsync_source=true (not a source name) |
| far | go | run | PASS | 0 | 0 px changed | 25.31 | 33.48 | 57.00 | 0 | 2 | p95=25.31 > 22 (P1.7 metric); p99=33.48; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |
| stream | native | run | PASS | 0 | 0 px changed | 19.01 | 19.56 | 57.49 | 0 | 2 | p99=19.56; vsync_source=true (not a source name) |
| stream | go | run | PASS | 0 | 0 px changed | 18.44 | 18.99 | 57.78 | 0 | 2 | p99=18.99; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 视觉证据说明

引擎窗以冻结金图 + window/offscreen 金图 diff 计数为视觉证据；窗自身写出 `*_final.png`/`*_last.png` 的已归档到 `snapshots/`。本轮未生成黑屏 X11 抓图（rootless Xwayland 的 x11grab 为黑，已禁用）。

