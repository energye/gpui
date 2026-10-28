# ui_wr_r7b_scroll_reuse

- 来源：`windows.md` §3.2（ui_wr）
- 目录：`examples/ui_wr_r7b_scroll_reuse`
- 运行：`A/60s`（mode=A，seconds=60，batch=False）
- 金图：`testdata/showcase_r7b_scroll_reuse_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：缓存/边界 · 滚动 cell 复用

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | FAIL | 1 | 0.0000% | 22.42 | 28.12 | 57.76 | 1 | 1 | exit code 1; p95=22.42 > 22 (P1.7 metric); p99=28.12; vsync_source=true (not a source name) |
| default | go | run | NO_JSON | 1 |  |  |  |  | 0 | 1 | no parseable JSON on stdout |

## 失败原因（原始 stderr 摘录）

- `default` / `native` / `run`：exit code 1; p95=22.42 > 22 (P1.7 metric); p99=28.12; vsync_source=true (not a source name); ui_wr_r7b_scroll_reuse: golden showcase_r7b_scroll_reuse.png: diff=0.0000% over 821184 px; FAIL: logic probes 9/10 bad=[p95_le_22]
- `default` / `go` / `run`：no parseable JSON on stdout; 2026/10/28 20:42:48 ERROR gles: AdapterContext.Lock MakeCurrent failed err="eglMakeCurrent failed: error 0x3002"; FAIL: open: render: ConfigureFromCapabilities: gles: failed to bind GL context for swapchain framebuffer: gles: AdapterContext: MakeCurrent failed: eglMakeCurrent failed: error 0x3002

