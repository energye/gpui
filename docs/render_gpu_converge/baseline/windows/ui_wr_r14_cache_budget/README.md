# ui_wr_r14_cache_budget

- 来源：`windows.md` §3.2（ui_wr）
- 目录：`examples/ui_wr_r14_cache_budget`
- 运行：`R=60s`（mode=R，seconds=60，batch=False）
- 金图：运行期 `/tmp/r14_cache_budget/r14_final_base.png`（`R14_SNAP_DIR`）
- 容差：`0*`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：缓存/边界 · 预算/淘汰
- 运行期基线：`R14_SNAP_DIR`；归档在 `docs/render_gpu_converge/baseline/windows/ui_wr_r14_cache_budget/default/native/snapshots/r14_final_base.png`

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | seed | SEED | 0 | 0.0000% | 18.16 | 20.48 | 57.12 | 2 | 1 | runtime golden seeded; p99=20.48; vsync_source=true (not a source name) |
| default | native | confirm | PASS | 0 | 0.0000% | 18.20 | 19.83 | 57.35 | 2 | 1 | p99=19.83; vsync_source=true (not a source name) |
| default | go | confirm | PASS | 0 | 0.0000% | 18.61 | 20.78 | 57.55 | 2 | 1 | p99=20.78; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 运行期基线

首遍 `GPUI_ACCEPT_GOLDEN=1` 播种，次遍逐位对比；基线已归档到 `docs/render_gpu_converge/baseline/windows/ui_wr_r14_cache_budget/default/native/snapshots/r14_final_base.png`。`windows.md` 的容差应从 `0*` 回填为 `0`。

