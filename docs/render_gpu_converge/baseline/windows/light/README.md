# light

- 来源：`windows.md` §2（engine）
- 目录：`examples/engine/light`
- 运行：`A/8s`（mode=A，seconds=8，batch=False）
- 金图：`testdata/{light_torch,normal_dome,shadow_wall}_golden.png`
- 容差：`0`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：2.5D · 点光/方向光/法线/阴影

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| torch | native | run | FAIL | 1 | 99.8429% | 19.44 | 20.56 | 57.57 | 1 | 6 | exit code 1; p99=20.56; vsync_source=true (not a source name) |
| torch | go | run | FAIL | 1 | 99.8429% | 18.40 | 18.93 | 57.60 | 1 | 6 | exit code 1; p99=18.93; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |
| shadow | native | run | FAIL | 1 | 99.8429% | 19.56 | 20.63 | 57.42 | 2 | 6 | exit code 1; p99=20.63; vsync_source=true (not a source name) |
| shadow | go | run | FAIL | 1 | 99.8429% | 18.30 | 18.93 | 57.94 | 2 | 6 | exit code 1; p99=18.93; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |
| normal | native | run | FAIL | 1 | 99.6451% | 19.11 | 20.00 | 57.39 | 2 | 6 | exit code 1; p99=20.00; vsync_source=true (not a source name) |
| normal | go | run | FAIL | 1 | 99.6451% | 18.59 | 20.78 | 57.65 | 2 | 6 | exit code 1; p99=20.78; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 失败原因（原始 stderr 摘录）

- `torch` / `native` / `run`：exit code 1; p99=20.56; vsync_source=true (not a source name); golden diff 99.8429%; hitch_rate_per_min=14.99 > 5; game_light: golden light_final.png: diff=99.8429% over 916800 px; FAIL: parity_changed_pct=1.1111 want <=1.00 (CPU/GPU diverge)
- `torch` / `go` / `run`：exit code 1; p99=18.93; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 99.8429%; game_light: golden light_final.png: diff=99.8429% over 916800 px; FAIL: parity_changed_pct=1.1111 want <=1.00 (CPU/GPU diverge)
- `shadow` / `native` / `run`：exit code 1; p99=20.63; vsync_source=true (not a source name); golden diff 99.8429%; game_light: golden shadow_final.png: diff=99.8429% over 916800 px; FAIL: parity_changed_pct=1.0069 want <=1.00 (CPU/GPU diverge)
- `shadow` / `go` / `run`：exit code 1; p99=18.93; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 99.8429%; game_light: golden shadow_final.png: diff=99.8429% over 916800 px; FAIL: parity_changed_pct=1.0069 want <=1.00 (CPU/GPU diverge)
- `normal` / `native` / `run`：exit code 1; p99=20.00; vsync_source=true (not a source name); golden diff 99.6451%; game_light: golden normal_final.png: diff=99.6451% over 916800 px; FAIL: parity_changed_pct=1.9618 want <=1.00 (CPU/GPU diverge)
- `normal` / `go` / `run`：exit code 1; p99=20.78; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); golden diff 99.6451%; game_light: golden normal_final.png: diff=99.6451% over 916800 px; FAIL: parity_changed_pct=1.9618 want <=1.00 (CPU/GPU diverge)

## 视觉证据说明

引擎窗以冻结金图 + window/offscreen 金图 diff 计数为视觉证据；窗自身写出 `*_final.png`/`*_last.png` 的已归档到 `snapshots/`。本轮未生成黑屏 X11 抓图（rootless Xwayland 的 x11grab 为黑，已禁用）。

