# tilemap

- 来源：`windows.md` §2（engine）
- 目录：`examples/engine/tilemap`
- 运行：`A`（mode=A，seconds=None，batch=False）
- 金图：`testdata/tilemap_{map,chunk,lod,large}_golden.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：2.5D · 瓦片/分区/LOD/大世界

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| map | native | run | PASS | 0 | 0 px changed | 19.03 | 20.64 | 57.51 | 0 | 4 | p99=20.64; vsync_source=true (not a source name) |
| map | go | run | NO_JSON | 1 |  |  |  |  | 0 | 4 | no parseable JSON on stdout |
| chunk | native | run | PASS | 0 | 0 px changed | 19.80 | 22.57 | 57.56 | 0 | 4 | p99=22.57; vsync_source=true (not a source name) |
| chunk | go | run | PASS | 0 | 0 px changed | 18.46 | 19.41 | 57.99 | 0 | 4 | p99=19.41; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |
| lod | native | run | PASS | 0 | 0 px changed | 19.07 | 19.55 | 57.55 | 0 | 4 | p99=19.55; vsync_source=true (not a source name) |
| lod | go | run | PASS | 0 | 0 px changed | 18.62 | 19.64 | 57.83 | 0 | 4 | p99=19.64; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |
| large | native | run | PASS | 0 | 0 px changed | 19.48 | 20.45 | 57.43 | 0 | 4 | p99=20.45; vsync_source=true (not a source name) |
| large | go | run | PASS | 0 | 0 px changed | 18.34 | 19.85 | 57.95 | 0 | 4 | p99=19.85; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 失败原因（原始 stderr 摘录）

- `map` / `go` / `run`：no parseable JSON on stdout; 2026/10/28 20:28:16 ERROR gles: AdapterContext.Lock MakeCurrent failed err="eglMakeCurrent failed: error 0x3002"; FAIL: open: render: ConfigureFromCapabilities: gles: failed to bind GL context for swapchain framebuffer: gles: AdapterContext: MakeCurrent failed: eglMakeCurrent failed: error 0x3002

## 视觉证据说明

引擎窗以冻结金图 + window/offscreen 金图 diff 计数为视觉证据；窗自身写出 `*_final.png`/`*_last.png` 的已归档到 `snapshots/`。本轮未生成黑屏 X11 抓图（rootless Xwayland 的 x11grab 为黑，已禁用）。

