# ui_wr_c6_savelayer_group

- 来源：`windows.md` §3.2（ui_wr）
- 目录：`examples/ui_wr_c6_savelayer_group`
- 运行：`R=10s`（mode=R，seconds=10，batch=False）
- 金图：运行期 `/tmp/c6_savelayer_group/c6_final_base.png`（`C6_SNAP_DIR`）
- 容差：`0*`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：混合/显存 · R18+R3 离屏组拒批
- 运行期基线：`C6_SNAP_DIR`；归档在 `docs/render_gpu_converge/baseline/windows/ui_wr_c6_savelayer_group/default/native/snapshots/c6_final_base.png`

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | seed | SEED | 1 | 0.0000% | 19.13 | 19.86 | 57.19 | 2 | 1 | exit code 1; p99=19.86; vsync_source=true (not a source name) |
| default | native | confirm | FAIL | 1 | 0.0000% | 19.67 | 21.58 | 57.35 | 2 | 1 | exit code 1; p99=21.58; vsync_source=true (not a source name) |
| default | go | confirm | NO_JSON | 1 |  |  |  |  | 1 | 1 | no parseable JSON on stdout |

## 失败原因（原始 stderr 摘录）

- `default` / `native` / `confirm`：exit code 1; p99=21.58; vsync_source=true (not a source name); hitch_rate_per_min=12.01 > 5; golden c6_final.png: diff=0.0000% over 529400 px; FAIL: hitch_rate_per_min=12.01 > 5
- `default` / `go` / `confirm`：no parseable JSON on stdout; 2026/10/28 20:49:44 ERROR gles: AdapterContext.Lock MakeCurrent failed err="eglMakeCurrent failed: error 0x3002"; FAIL: open: render: ConfigureFromCapabilities: gles: failed to bind GL context for swapchain framebuffer: gles: AdapterContext: MakeCurrent failed: eglMakeCurrent failed: error 0x3002

## 运行期基线

首遍 `GPUI_ACCEPT_GOLDEN=1` 播种，次遍逐位对比；基线已归档到 `docs/render_gpu_converge/baseline/windows/ui_wr_c6_savelayer_group/default/native/snapshots/c6_final_base.png`。`windows.md` 的容差应从 `0*` 回填为 `0`。

