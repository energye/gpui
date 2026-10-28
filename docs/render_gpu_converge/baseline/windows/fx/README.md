# fx

- 来源：`windows.md` §2（engine）
- 目录：`examples/engine/fx`
- 运行：`A/8s`（mode=A，seconds=8，batch=False）
- 金图：`testdata/{fx_grade,fx_custom}_golden.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：2.5D · 水波扭曲/后期调色

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| all | native | run | FAIL | 1 | 100.0000% | 19.09 | 19.58 | 57.58 | 1 | 4 | exit code 1; p99=19.58; vsync_source=true (not a source name) |
| all | go | run | FAIL | 1 | 100.0000% | 18.40 | 19.67 | 57.72 | 1 | 4 | exit code 1; p99=19.67; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 失败原因（原始 stderr 摘录）

- `all` / `native` / `run`：exit code 1; p99=19.58; vsync_source=true (not a source name); golden diff 100.0000%; game_fx: golden fx_final.png: diff=100.0000% over 921600 px; FAIL: win_golden_diff_pct=100.0000 want 0.0 over 921600 px
- `all` / `go` / `run`：exit code 1; p99=19.67; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 100.0000%; game_fx: golden fx_final.png: diff=100.0000% over 921600 px; FAIL: win_golden_diff_pct=100.0000 want 0.0 over 921600 px

## 视觉证据说明

引擎窗以冻结金图 + window/offscreen 金图 diff 计数为视觉证据；窗自身写出 `*_final.png`/`*_last.png` 的已归档到 `snapshots/`。本轮未生成黑屏 X11 抓图（rootless Xwayland 的 x11grab 为黑，已禁用）。

