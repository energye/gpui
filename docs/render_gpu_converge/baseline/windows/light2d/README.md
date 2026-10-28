# light2d

- 来源：`windows.md` §2（engine）
- 目录：`examples/engine/light2d`
- 运行：`A/8s`（mode=A，seconds=8，batch=False）
- 金图：`testdata/soft_{none,pcf5,pcf13}_golden.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：2.5D · 2D 光/软阴影三档

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| shadow | native | run | FAIL | 1 |  | 19.35 | 21.90 | 57.45 | 1 | 4 | exit code 1; p99=21.90; vsync_source=true (not a source name) |
| shadow | go | run | NO_JSON | 1 |  |  |  |  | 0 | 4 | no parseable JSON on stdout |

## 失败原因（原始 stderr 摘录）

- `shadow` / `native` / `run`：exit code 1; p99=21.90; vsync_source=true (not a source name); game_light2d: grade=PCF13 taps=13 mirror=true moved=252 gain=0.8007 lee=0.000000 golden=0.0000%(first=false) pixel=true; game_light2d: probes ok=true logic=true pixel=true golden=true off_is_night=true; game_light2d: golden light2d_final.png: diff=99.7670% over 916800 px; FAIL: win_golden_diff=99.7670 want 0.0 over 916800 px
- `shadow` / `go` / `run`：no parseable JSON on stdout; 2026/10/28 20:23:57 ERROR gles: AdapterContext.Lock MakeCurrent failed err="eglMakeCurrent failed: error 0x3002"; FAIL: open: render: ConfigureFromCapabilities: gles: failed to bind GL context for swapchain framebuffer: gles: AdapterContext: MakeCurrent failed: eglMakeCurrent failed: error 0x3002

## 视觉证据说明

引擎窗以冻结金图 + window/offscreen 金图 diff 计数为视觉证据；窗自身写出 `*_final.png`/`*_last.png` 的已归档到 `snapshots/`。本轮未生成黑屏 X11 抓图（rootless Xwayland 的 x11grab 为黑，已禁用）。

