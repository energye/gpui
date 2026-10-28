# video_s8_index

- 来源：`windows.md` §3.5（video）
- 目录：`examples/video/video_s8_index`
- 运行：`A/15s`（mode=A，seconds=15，batch=False）
- 金图：—
- 容差：`—`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：视频 · 跳转索引

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | FAIL | 1 |  |  |  |  | 0 | 0 | JSON pass/ok=false |
| default | go | run | FAIL | 1 |  |  |  |  | 0 | 0 | JSON pass/ok=false |

## 失败原因（原始 stderr 摘录）

- `default` / `native` / `run`：JSON pass/ok=false; video_s8_index: probes passed=16/18 failed=2 forwardMax=1 err=seek:vr_stream_long.mp4/100000:跳后黑屏: 落点100000没播出来
- `default` / `go` / `run`：JSON pass/ok=false; video_s8_index: probes passed=16/18 failed=2 forwardMax=1 err=seek:vr_stream_long.mp4/100000:跳后黑屏: 落点100000没播出来

## 为什么无金图

该窗按 `windows.md` 无固定金图（能力标签：视频 · 跳转索引）。替代判定是窗口自身的逻辑探针、像素断言、性能门禁或人工探针 JSON；动态视频/整窗动画/纯性能窗做固定点金图会假绿。逐轮 JSON 与 stderr 已归档。

