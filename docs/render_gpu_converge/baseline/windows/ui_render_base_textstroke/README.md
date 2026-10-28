# ui_render_base_textstroke

- 来源：`windows.md` §3.1（render）
- 目录：`examples/ui_render_base_textstroke`
- 运行：`A/5s`（mode=A，seconds=5，batch=False）
- 金图：`testdata/showcase_textstroke_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：文本 · 描边/沿路径

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | FAIL | 1 | 0.0000% | 19.68 | 20.77 | 57.49 | 1 | 1 | exit code 1; p99=20.77; vsync_source=true (not a source name) |
| default | go | run | NO_JSON | 1 |  |  |  |  | 0 | 1 | no parseable JSON on stdout |

## 失败原因（原始 stderr 摘录）

- `default` / `native` / `run`：exit code 1; p99=20.77; vsync_source=true (not a source name); hitch_rate_per_min=23.96 > 5; golden showcase_textstroke.png: diff=0.0000% over 627200 px; FAIL: screen check: diff=100.0000% over 627200 px > 1.00% (grab vs snapshot)
- `default` / `go` / `run`：no parseable JSON on stdout; 2026/10/28 20:37:33 ERROR gles: AdapterContext.Lock MakeCurrent failed err="eglMakeCurrent failed: error 0x3002"; FAIL: open: render: ConfigureFromCapabilities: gles: failed to bind GL context for swapchain framebuffer: gles: AdapterContext: MakeCurrent failed: eglMakeCurrent failed: error 0x3002

