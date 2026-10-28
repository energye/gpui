# video_a4_sink

- 来源：`windows.md` §3.5（video）
- 目录：`examples/video/video_a4_sink`
- 运行：`A/15s`（mode=A，seconds=15，batch=False）
- 金图：—
- 容差：`—`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：视频 · 宿主出声

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | FAIL | 1 |  |  |  |  | 0 | 0 | JSON pass/ok=false |
| default | go | run | FAIL | 1 |  |  |  |  | 0 | 0 | JSON pass/ok=false |

## 失败原因（原始 stderr 摘录）

- `default` / `native` / `run`：JSON pass/ok=false; video_a4_sink: probes passed=14/15 failed=1 backend=paplay-pulse available=true err=identity:本该静音却有声
- `default` / `go` / `run`：JSON pass/ok=false; video_a4_sink: probes passed=14/15 failed=1 backend=paplay-pulse available=true err=identity:本该静音却有声

## 为什么无金图

该窗按 `windows.md` 无固定金图（能力标签：视频 · 宿主出声）。替代判定是窗口自身的逻辑探针、像素断言、性能门禁或人工探针 JSON；动态视频/整窗动画/纯性能窗做固定点金图会假绿。逐轮 JSON 与 stderr 已归档。

