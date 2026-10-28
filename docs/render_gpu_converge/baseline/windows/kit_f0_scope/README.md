# kit_f0_scope

- 来源：`windows.md` §3.4（kit）
- 目录：`examples/kit_f0_scope`
- 运行：`A/5s`（mode=A，seconds=5，batch=False）
- 金图：`testdata/showcase_scope_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：组件/混合 · 主题作用域

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | FAIL | 1 | 0.0000% | 25.06 | 30.62 | 58.94 | 1 | 1 | exit code 1; p95=25.06 > 22 (P1.7 metric); p99=30.62; vsync_source=true (not a source name) |
| default | go | run | NO_JSON | 1 |  |  |  |  | 0 | 1 | no parseable JSON on stdout |

## 失败原因（原始 stderr 摘录）

- `default` / `native` / `run`：exit code 1; p95=25.06 > 22 (P1.7 metric); p99=30.62; vsync_source=true (not a source name); hitch_rate_per_min=23.98 > 5; golden showcase_scope.png: diff=0.0000% over 311680 px; FAIL: FAIL: interval_p95_ms=25.06 > 22.00
- `default` / `go` / `run`：no parseable JSON on stdout; 2026/10/28 21:11:57 ERROR gles: AdapterContext.Lock MakeCurrent failed err="eglMakeCurrent failed: error 0x3002"; FAIL: open: render: ConfigureFromCapabilities: gles: failed to bind GL context for swapchain framebuffer: gles: AdapterContext: MakeCurrent failed: eglMakeCurrent failed: error 0x3002

