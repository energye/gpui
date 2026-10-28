# ui_wr_ime_r2_textlayout

- 来源：`windows.md` §3.2（ui_wr）
- 目录：`examples/ui_wr_ime_r2_textlayout`
- 运行：`A`（mode=A，seconds=None，batch=False）
- 金图：`testdata/r2_textlayout_base.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：IME/文本 · 文本布局单源

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 |  | 20.01 | 21.48 | 57.55 | 1 | 1 | p99=21.48; vsync_source=true (not a source name) |
| default | go | run | NO_JSON | 1 |  |  |  |  | 0 | 1 | no parseable JSON on stdout |

## 失败原因（原始 stderr 摘录）

- `default` / `go` / `run`：no parseable JSON on stdout; FAIL: open: render: CreateSurface(xlib): gles: failed to create shared X11 EGL context: eglCreateContext failed: error 0x3000

