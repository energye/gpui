# ui_l1_scroll

- 来源：`windows.md` §3.3（base）
- 目录：`examples/ui_l1_scroll`
- 运行：`R=60s`（mode=R，seconds=60，batch=False）
- 金图：—
- 容差：`—`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：X11 或 Wayland/native

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 |  |  |  |  | 0 | 0 | vsync_source=true (not a source name); missing A fields: fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,target_hz |
| default | go | run | PASS | 0 |  |  |  |  | 0 | 0 | vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); missing A fields: fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_rate_per_min,target_hz |

## 为什么无金图

该窗按 `windows.md` 无固定金图（能力标签：X11 或 Wayland/native）。替代判定是窗口自身的逻辑探针、像素断言、性能门禁或人工探针 JSON；动态视频/整窗动画/纯性能窗做固定点金图会假绿。逐轮 JSON 与 stderr 已归档。

