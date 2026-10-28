# ui_wr_shadermask

- 来源：`windows.md` §3.2（ui_wr）
- 目录：`examples/ui_wr_shadermask`
- 运行：`A/5s`（mode=A，seconds=5，batch=False）
- 金图：`testdata/showcase_shadermask_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：着色器 · ShaderMask 用户空间

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | FAIL | 1 | 4.2822% | 24.34 | 27.31 | 54.58 | 1 | 1 | exit code 1; p95=24.34 > 22 (P1.7 metric); p99=27.31; vsync_source=true (not a source name) |
| default | go | run | NO_JSON | 1 |  |  |  |  | 0 | 1 | no parseable JSON on stdout |

## 失败原因（原始 stderr 摘录）

- `default` / `native` / `run`：exit code 1; p95=24.34 > 22 (P1.7 metric); p99=27.31; vsync_source=true (not a source name); golden diff 4.2822%; hitch_rate_per_min=11.99 > 5; ui_wr_shadermask: logic mask_skips_zero          pass=true; ui_wr_shadermask: logic live_only_skips_zero     pass=true; ui_wr_shadermask: logic golden_zero              pass=false; FAIL: logic probes 10/11 bad=[golden_zero]
- `default` / `go` / `run`：no parseable JSON on stdout; 2026/10/28 20:57:06 ERROR gles: AdapterContext.Lock MakeCurrent failed err="eglMakeCurrent failed: error 0x3002"; FAIL: open: render: ConfigureFromCapabilities: gles: failed to bind GL context for swapchain framebuffer: gles: AdapterContext: MakeCurrent failed: eglMakeCurrent failed: error 0x3002

