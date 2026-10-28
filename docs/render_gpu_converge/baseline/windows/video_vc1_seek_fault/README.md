# video_vc1_seek_fault

- 来源：`windows.md` §3.5（video）
- 目录：`examples/video/video_vc1_seek_fault`
- 运行：`R=30s`（mode=R，seconds=30，batch=False）
- 金图：—
- 容差：`—`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：视频 · 边播边跳+坏文件

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | FAIL | 1 | 0.0000% | 19.29 |  |  | 0 | 0 | exit code 1; vsync_source=true (not a source name) |
| default | go | run | FAIL | 1 | 0.0000% | 18.60 |  |  | 0 | 0 | exit code 1; vsync_source=true (not a source name) |

## 失败原因（原始 stderr 摘录）

- `default` / `native` / `run`：exit code 1; vsync_source=true (not a source name); [h264 @ 0x70a1d004a8c0] Error splitting the input into NAL units.; [h264 @ 0x70a1d076d0c0] Error splitting the input into NAL units.; [h264 @ 0x70a1d0087b00] Error splitting the input into NAL units.; FAIL: 跳进度门禁: 跳第二GOP：跳后黑屏: 目标1800 落点1800 没立刻播出来
- `default` / `go` / `run`：exit code 1; vsync_source=true (not a source name); [h264 @ 0x3039de00] Error splitting the input into NAL units.; 2026/10/28 21:15:48 ERROR gles: AdapterContext.Lock MakeCurrent failed err="eglMakeCurrent failed: error 0x3002"; FAIL: 跳进度门禁: 跳第二GOP：跳后黑屏: 目标1800 落点1800 没立刻播出来

## 为什么无金图

该窗按 `windows.md` 无固定金图（能力标签：视频 · 边播边跳+坏文件）。替代判定是窗口自身的逻辑探针、像素断言、性能门禁或人工探针 JSON；动态视频/整窗动画/纯性能窗做固定点金图会假绿。逐轮 JSON 与 stderr 已归档。

