# world

- 来源：`windows.md` §2（engine）
- 目录：`examples/engine/world`
- 运行：`A/8s`（mode=A，seconds=8，batch=False）
- 金图：`testdata/world_{open,large}_golden.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：2.5D · 场景 JSON/多视口/挂点

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| open | native | run | PASS | 0 | 0 px changed | 19.72 | 20.48 | 57.44 | 0 | 2 | p99=20.48; vsync_source=true (not a source name) |
| open | go | run | PASS | 0 | 0 px changed | 18.42 | 19.97 | 57.80 | 0 | 2 | p99=19.97; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |
| large | native | run | PASS | 0 | 0 px changed | 21.52 | 26.65 | 57.20 | 0 | 2 | p99=26.65; vsync_source=true (not a source name) |
| large | go | run | NO_JSON | 1 |  |  |  |  | 0 | 2 | no parseable JSON on stdout |

## 失败原因（原始 stderr 摘录）

- `large` / `go` / `run`：no parseable JSON on stdout; game_world-large: probes ok=true logic=true pix=true golden=true(wrote=false changed=0) active=1655 total=20008 entities=20008 home_active=1655 attach=500 markers=8 / layer0@(28,28) marker@(420,246) bg@(100,52) tol=4; FAIL: open: render: CreateSurface(xlib): gles: failed to create shared X11 EGL context: eglCreateContext failed: error 0x3000

## 视觉证据说明

引擎窗以冻结金图 + window/offscreen 金图 diff 计数为视觉证据；窗自身写出 `*_final.png`/`*_last.png` 的已归档到 `snapshots/`。本轮未生成黑屏 X11 抓图（rootless Xwayland 的 x11grab 为黑，已禁用）。

