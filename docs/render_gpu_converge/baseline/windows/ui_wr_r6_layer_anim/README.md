# ui_wr_r6_layer_anim

- 来源：`windows.md` §3.2（ui_wr）
- 目录：`examples/ui_wr_r6_layer_anim`
- 运行：`R=30s`（mode=R，seconds=30，batch=False）
- 金图：`golden/baseline_igpu.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：混合 · Opacity/Transform/Clip 层动画

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 | 0.0000% | 19.62 | 20.50 | 57.29 | 2 | 1 | p99=20.50; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 | 0.0000% | 18.64 | 19.56 | 57.58 | 2 | 1 | p99=19.56; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 金图真源不一致（P1.4 发现）

`windows.md` 标的是 `golden/baseline_igpu.png`，但当前 `main.go` 的 `evaluateGolden` 读的是 `R6_SNAP_DIR`（默认 `/tmp/r6_layer_anim`）下的 `r6_final_base.png`；仓内 `baseline_igpu.png` 没有被读取。首跑只播种 `/tmp`，所以本轮记 `SEED`，不标 PASS；P2 需把真源与代码对齐（或把该窗正式列入运行期金图）。

