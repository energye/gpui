# ui_wr_ime_r1_editor

- 来源：`windows.md` §3.2（ui_wr）
- 目录：`examples/ui_wr_ime_r1_editor`
- 运行：`A/5s`（mode=A，seconds=5，batch=False）
- 金图：`testdata/showcase_ime_r1_editor_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：IME/文本 · 编辑器四元组

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | FAIL | 1 | 0.0847% | 19.36 | 20.19 | 57.69 | 1 | 1 | exit code 1; p99=20.19; vsync_source=true (not a source name) |
| default | go | run | FAIL | 1 | 0.0847% | 18.69 | 21.17 | 57.53 | 1 | 1 | exit code 1; p99=21.17; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 失败原因（原始 stderr 摘录）

- `default` / `native` / `run`：exit code 1; p99=20.19; vsync_source=true (not a source name); golden diff 0.0847%; hitch_rate_per_min=24.12 > 5; ui_wr_ime_r1_editor: golden showcase_ime_r1_editor.png: diff=0.0847% over 813312 px; FAIL: pixel_golden_diff_pct=0.0847 want 0 over 813312 px
- `default` / `go` / `run`：exit code 1; p99=21.17; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 0.0847%; hitch_rate_per_min=12.15 > 5; ui_wr_ime_r1_editor: golden showcase_ime_r1_editor.png: diff=0.0847% over 813312 px; FAIL: pixel_golden_diff_pct=0.0847 want 0 over 813312 px

