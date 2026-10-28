# stage_chase

- 来源：`windows.md` §2（engine）
- 目录：`examples/engine/stage_chase`
- 运行：`A/120s`（mode=A，seconds=120，batch=False）
- 金图：`testdata/stage_chase_golden.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：2.5D · 相机/排序/粒子/瓦片/流式组合

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | FAIL | 1 | 99.1429% | 19.65 | 21.85 | 57.14 | 2 | 2 | exit code 1; p99=21.85; vsync_source=true (not a source name) |
| default | go | run | FAIL | 1 | 99.1429% | 18.46 | 19.31 | 57.62 | 2 | 2 | exit code 1; p99=19.31; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 失败原因（原始 stderr 摘录）

- `default` / `native` / `run`：exit code 1; p99=21.85; vsync_source=true (not a source name); golden diff 99.1429%; game_stage_chase: probes ok=true logic=true(cam=true chunk=true sort=true trail=true dirty=true pool=true detail=cam=(60,48) chunks=8x5 needed=2 sorted=far,near,fx,ui trail=12->2 dirty=1/full pool alive) pix=true parity=0.0000% golden=true(wrote=false changed=0) chunkA=(40,56,45) chunkB=(51,45,35) tree=(51,153,63) car=(229,38,30) trail=(255,165,38) tol=8; game_stage_chase: golden chase_final.png: diff=99.1429% over 240000 px; FAIL: p95=19.65 want <=16 (§5)
- `default` / `go` / `run`：exit code 1; p99=19.31; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 99.1429%; game_stage_chase: probes ok=true logic=true(cam=true chunk=true sort=true trail=true dirty=true pool=true detail=cam=(60,48) chunks=8x5 needed=2 sorted=far,near,fx,ui trail=12->2 dirty=1/full pool alive) pix=true parity=0.0000% golden=true(wrote=false changed=0) chunkA=(40,56,45) chunkB=(51,45,35) tree=(51,153,63) car=(229,38,30) trail=(255,165,38) tol=8; game_stage_chase: golden chase_final.png: diff=99.1429% over 240000 px; FAIL: p95=18.46 want <=16 (§5)

## 视觉证据说明

引擎窗以冻结金图 + window/offscreen 金图 diff 计数为视觉证据；窗自身写出 `*_final.png`/`*_last.png` 的已归档到 `snapshots/`。本轮未生成黑屏 X11 抓图（rootless Xwayland 的 x11grab 为黑，已禁用）。

