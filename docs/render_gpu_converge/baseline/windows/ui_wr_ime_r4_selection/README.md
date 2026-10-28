# ui_wr_ime_r4_selection

- 来源：`windows.md` §3.2（ui_wr）
- 目录：`examples/ui_wr_ime_r4_selection`
- 运行：`A`（mode=A，seconds=None，batch=False）
- 金图：`testdata/r4_selection_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：IME/文本 · 选区与编辑

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 |  | 19.38 | 20.83 | 57.59 | 1 | 1 | p99=20.83; vsync_source=true (not a source name) |
| default | go | run | NO_JSON | 1 |  |  |  |  | 0 | 1 | no parseable JSON on stdout |

## 失败原因（原始 stderr 摘录）

- `default` / `go` / `run`：no parseable JSON on stdout; 2026/10/28 20:56:56 ERROR gles: AdapterContext.Lock MakeCurrent failed err="eglMakeCurrent failed: error 0x3002"; FAIL: open: render: ConfigureFromCapabilities: gles: failed to bind GL context for swapchain framebuffer: gles: AdapterContext: MakeCurrent failed: eglMakeCurrent failed: error 0x3002

