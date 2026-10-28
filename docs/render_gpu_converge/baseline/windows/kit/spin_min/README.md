# kit/spin_min

- 来源：`windows.md` §3.4（kit）
- 目录：`examples/kit/spin_min`
- 运行：`A/60s`（mode=A，seconds=60，batch=False）
- 金图：—
- 容差：`—`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：组件/帧时 · 旋转图标（无金图：只出 stderr 指标+JSON）

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 |  |  |  |  | 0 | 0 | vsync_source=true (not a source name); missing A fields: fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,hitch_rate_per_min,target_hz |
| default | go | run | PASS | 0 |  |  |  |  | 0 | 0 | vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); missing A fields: fps_wall,interval_avg_ms,interval_p50_ms,interval_p95_ms,interval_p99_ms,target_hz |

## 为什么无金图

该窗按 `windows.md` 无固定金图（能力标签：组件/帧时 · 旋转图标（无金图：只出 stderr 指标+JSON））。替代判定是窗口自身的逻辑探针、像素断言、性能门禁或人工探针 JSON；动态视频/整窗动画/纯性能窗做固定点金图会假绿。逐轮 JSON 与 stderr 已归档。

