# ui_wr_r20_filter

- 来源：`windows.md` §3.2（ui_wr）
- 目录：`examples/ui_wr_r20_filter`
- 运行：`R=10s`（mode=R，seconds=10，batch=False）
- 金图：运行期 `/tmp/r20_filter/r20_final_base.png`（`R20_SNAP_DIR`）
- 容差：`0*`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：滤镜 · 模糊/灰度滤镜子树
- 运行期基线：`R20_SNAP_DIR`；归档在 `docs/render_gpu_converge/baseline/windows/ui_wr_r20_filter/default/native/snapshots/r20_final_base.png`

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | seed | SEED | 1 | 0.0000% | 18.32 | 19.19 | 57.49 | 4 | 1 | exit code 1; p99=19.19; vsync_source=true (not a source name) |
| default | native | confirm | FAIL | 1 | 0.0000% | 18.36 | 18.60 | 57.44 | 4 | 1 | exit code 1; p99=18.60; vsync_source=true (not a source name) |
| default | go | confirm | FAIL | 1 | 0.0000% | 18.63 | 20.11 | 57.74 | 4 | 1 | exit code 1; p99=20.11; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 失败原因（原始 stderr 摘录）

- `default` / `native` / `confirm`：exit code 1; p99=18.60; vsync_source=true (not a source name); hitch_rate_per_min=17.98 > 5; golden r20_final.png: diff=0.0000% over 306280 px; FAIL: scripted=4/5 want 5/5 (像素断言)
- `default` / `go` / `confirm`：exit code 1; p99=20.11; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); hitch_rate_per_min=12.03 > 5; golden r20_final.png: diff=0.0000% over 306280 px; FAIL: scripted=3/5 want 5/5 (像素断言)

## 运行期基线

首遍 `GPUI_ACCEPT_GOLDEN=1` 播种，次遍逐位对比；基线已归档到 `docs/render_gpu_converge/baseline/windows/ui_wr_r20_filter/default/native/snapshots/r20_final_base.png`。`windows.md` 的容差应从 `0*` 回填为 `0`。

