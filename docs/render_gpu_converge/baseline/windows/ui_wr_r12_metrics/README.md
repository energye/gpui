# ui_wr_r12_metrics

- 来源：`windows.md` §3.2（ui_wr）
- 目录：`examples/ui_wr_r12_metrics`
- 运行：`R=5s`（mode=R，seconds=5，batch=False）
- 金图：—
- 容差：`—`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：帧时/调度 · A–J 指标字段完备（无金图：指标窗）

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 |  | 19.27 | 23.34 | 57.63 | 0 | 0 | p99=23.34; vsync_source=true (not a source name) |
| default | go | run | PASS | 0 |  | 18.50 | 19.26 | 57.89 | 0 | 0 | p99=19.26; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 为什么无金图

该窗按 `windows.md` 无固定金图（能力标签：帧时/调度 · A–J 指标字段完备（无金图：指标窗））。替代判定是窗口自身的逻辑探针、像素断言、性能门禁或人工探针 JSON；动态视频/整窗动画/纯性能窗做固定点金图会假绿。逐轮 JSON 与 stderr 已归档。

