# video_vr3_color

- 来源：`windows.md` §3.5（video）
- 目录：`examples/video/video_vr3_color`
- 运行：`R=5s`（mode=R，seconds=5，batch=False）
- 金图：—
- 容差：`—`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：视频 · 颜色转换

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | FAIL | 1 | 0.0000% | 20.70 |  |  | 0 | 0 | exit code 1; vsync_source=true (not a source name) |
| default | go | run | FAIL | 1 | 0.0000% | 20.31 |  |  | 0 | 0 | exit code 1; vsync_source=true (not a source name) |

## 失败原因（原始 stderr 摘录）

- `default` / `native` / `run`：exit code 1; vsync_source=true (not a source name); hitch_rate_per_min=12.08 > 5; FAIL: 转色失败: vr2_b_intra.mp4 播出1帧(要5)
- `default` / `go` / `run`：exit code 1; vsync_source=true (not a source name); hitch_rate_per_min=12.08 > 5; FAIL: 转色失败: vr2_b_intra.mp4 播出1帧(要5)

## 为什么无金图

该窗按 `windows.md` 无固定金图（能力标签：视频 · 颜色转换）。替代判定是窗口自身的逻辑探针、像素断言、性能门禁或人工探针 JSON；动态视频/整窗动画/纯性能窗做固定点金图会假绿。逐轮 JSON 与 stderr 已归档。

