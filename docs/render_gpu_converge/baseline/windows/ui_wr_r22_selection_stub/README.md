# ui_wr_r22_selection_stub

- 来源：`windows.md` §3.2（ui_wr）
- 目录：`examples/ui_wr_r22_selection_stub`
- 运行：`R=5s`（mode=R，seconds=5，batch=False）
- 金图：运行期 `/tmp/r22_selection_stub/r22_final_base.png`（`R22_SNAP_DIR`）
- 容差：`0*`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：缓存/边界/文本 · 选区光标局部脏
- 运行期基线：`R22_SNAP_DIR`；归档在 `docs/render_gpu_converge/baseline/windows/ui_wr_r22_selection_stub/default/native/snapshots/r22_final_base.png`

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | seed | SEED | 1 | 0.0000% | 18.04 | 24.36 | 57.97 | 4 | 1 | exit code 1; p99=24.36; vsync_source=true (not a source name) |
| default | native | confirm | FAIL | 1 | 0.0000% | 18.02 | 32.29 | 57.81 | 4 | 1 | exit code 1; p99=32.29; vsync_source=true (not a source name) |
| default | go | confirm | FAIL | 1 | 0.0000% | 18.96 | 22.87 | 57.67 | 4 | 1 | exit code 1; p99=22.87; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 失败原因（原始 stderr 摘录）

- `default` / `native` / `confirm`：exit code 1; p99=32.29; vsync_source=true (not a source name); hitch_rate_per_min=48.18 > 5; golden r22_final.png: diff=0.0000% over 318160 px; FAIL: hitch_rate_per_min=48.18 > 5
- `default` / `go` / `confirm`：exit code 1; p99=22.87; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); hitch_rate_per_min=23.86 > 5; golden r22_final.png: diff=0.0000% over 318160 px; FAIL: hitch_rate_per_min=23.86 > 5

## 运行期基线

首遍 `GPUI_ACCEPT_GOLDEN=1` 播种，次遍逐位对比；基线已归档到 `docs/render_gpu_converge/baseline/windows/ui_wr_r22_selection_stub/default/native/snapshots/r22_final_base.png`。`windows.md` 的容差应从 `0*` 回填为 `0`。

