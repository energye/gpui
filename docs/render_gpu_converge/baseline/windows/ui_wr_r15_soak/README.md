# ui_wr_r15_soak

- 来源：`windows.md` §3.2（ui_wr）
- 目录：`examples/ui_wr_r15_soak`
- 运行：`R=300s`（mode=R，seconds=300，batch=False）
- 金图：运行期 `/tmp/r15_soak/r15_final_base.png`（`R15_SNAP_DIR`）
- 容差：`0*`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：帧时/调度 · 全主路径长跑
- 运行期基线：`R15_SNAP_DIR`；归档在 `docs/render_gpu_converge/baseline/windows/ui_wr_r15_soak/default/native/snapshots/r15_final_base.png`

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | seed | SEED | 0 | 0.0000% | 18.29 | 18.56 | 57.24 | 2 | 1 | runtime golden seeded; p99=18.56; vsync_source=true (not a source name) |
| default | native | confirm | PASS | 0 | 0.0000% | 18.27 | 19.57 | 57.24 | 2 | 1 | p99=19.57; vsync_source=true (not a source name) |
| default | go | confirm | NO_JSON | 1 |  |  |  |  | 1 | 1 | no parseable JSON on stdout |

## 失败原因（原始 stderr 摘录）

- `default` / `go` / `confirm`：no parseable JSON on stdout; FAIL: open: render: CreateSurface(xlib): gles: failed to create shared X11 EGL context: eglCreateContext failed: error 0x3000

## 运行期基线

首遍 `GPUI_ACCEPT_GOLDEN=1` 播种，次遍逐位对比；基线已归档到 `docs/render_gpu_converge/baseline/windows/ui_wr_r15_soak/default/native/snapshots/r15_final_base.png`。`windows.md` 的容差应从 `0*` 回填为 `0`。

