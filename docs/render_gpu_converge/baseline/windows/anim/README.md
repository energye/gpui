# anim

- 来源：`windows.md` §2（engine）
- 目录：`examples/engine/anim`
- 运行：`A/8s`（mode=A，seconds=8，batch=False）
- 金图：`testdata/{fsm,lod,sk,tl}_golden.png`（4）
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：2.5D · FSM/LOD/骨骼/时间线

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| sk | native | run | FAIL | 1 | 74.9220% | 20.17 | 22.71 | 57.46 | 2 | 8 | exit code 1; p99=22.71; vsync_source=true (not a source name) |
| sk | go | run | FAIL | 1 | 74.9220% | 19.17 | 23.51 | 57.82 | 2 | 8 | exit code 1; p99=23.51; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |
| fsm | native | run | FAIL | 1 | 73.6752% | 19.26 | 20.66 | 57.43 | 3 | 8 | exit code 1; p99=20.66; vsync_source=true (not a source name) |
| fsm | go | run | FAIL | 1 | 73.6752% | 18.54 | 19.27 | 57.60 | 3 | 8 | exit code 1; p99=19.27; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |
| tl | native | run | FAIL | 1 | 25.7695% |  |  |  | 2 | 8 | JSON pass/ok=false; missing A fields: fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| tl | go | run | FAIL | 1 | 25.7695% |  |  |  | 2 | 8 | JSON pass/ok=false; missing A fields: fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz |
| lod | native | run | PASS | 0 | 0.0000% | 18.97 | 19.47 | 57.59 | 4 | 8 | p99=19.47; vsync_source=true (not a source name) |
| lod | go | run | PASS | 0 | 0.0000% | 18.63 | 20.28 | 57.77 | 4 | 8 | p99=20.28; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 失败原因（原始 stderr 摘录）

- `sk` / `native` / `run`：exit code 1; p99=22.71; vsync_source=true (not a source name); golden diff 74.9220%; hitch_rate_per_min=7.51 > 5; game_anim: selftest probe=true parity=true pixel=true golden=0.0000%(first=false) ok=true; game_anim: golden sk_final.png: diff=74.9220% over 921600 px; FAIL: win_golden_diff_pct=74.9220 want 0.0 over 921600 px
- `sk` / `go` / `run`：exit code 1; p99=23.51; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 74.9220%; game_anim: selftest probe=true parity=true pixel=true golden=0.0000%(first=false) ok=true; game_anim: golden sk_final.png: diff=74.9220% over 921600 px; FAIL: win_golden_diff_pct=74.9220 want 0.0 over 921600 px
- `fsm` / `native` / `run`：exit code 1; p99=20.66; vsync_source=true (not a source name); golden diff 73.6752%; game_anim: fsm selftest probe=true parity=true pixel=true golden=0.0000%(first=false) ok=true; game_anim: golden fsm_final.png: diff=73.6752% over 921600 px; FAIL: win_golden_diff_pct=73.6752 want 0.0 over 921600 px
- `fsm` / `go` / `run`：exit code 1; p99=19.27; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 73.6752%; game_anim: fsm selftest probe=true parity=true pixel=true golden=0.0000%(first=false) ok=true; game_anim: golden fsm_final.png: diff=73.6752% over 921600 px; FAIL: win_golden_diff_pct=73.6752 want 0.0 over 921600 px
- `tl` / `native` / `run`：JSON pass/ok=false; missing A fields: fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz; golden diff 25.7695%; game_anim: tl selftest probe=true parity=true pixel=true golden=25.7695%(first=false) ok=false; game_anim: tl selftest FAIL, not opening window
- `tl` / `go` / `run`：JSON pass/ok=false; missing A fields: fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_count,hitch_rate_per_min,vsync_source,target_hz; golden diff 25.7695%; game_anim: tl selftest probe=true parity=true pixel=true golden=25.7695%(first=false) ok=false; game_anim: tl selftest FAIL, not opening window

## 视觉证据说明

引擎窗以冻结金图 + window/offscreen 金图 diff 计数为视觉证据；窗自身写出 `*_final.png`/`*_last.png` 的已归档到 `snapshots/`。本轮未生成黑屏 X11 抓图（rootless Xwayland 的 x11grab 为黑，已禁用）。

