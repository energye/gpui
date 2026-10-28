# video_player

- 来源：`windows.md` §3.5（video）
- 目录：`examples/video/video_player`
- 运行：`R=15s`（mode=R，seconds=15，batch=False）
- 金图：—
- 容差：`—`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：视频 · 用法小样（非门禁）

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | NO_JSON | 0 |  |  |  |  | 2 | 0 | no parseable JSON on stdout |
| default | go | run | NO_JSON | 0 |  |  |  |  | 2 | 0 | no parseable JSON on stdout |

## 失败原因（原始 stderr 摘录）

- `default` / `native` / `run`：no parseable JSON on stdout
- `default` / `go` / `run`：no parseable JSON on stdout; 2026/10/28 21:14:27 ERROR gles: AdapterContext.Lock MakeCurrent failed err="eglMakeCurrent failed: error 0x3002"

## 为什么无金图

该窗按 `windows.md` 无固定金图（能力标签：视频 · 用法小样（非门禁））。替代判定是窗口自身的逻辑探针、像素断言、性能门禁或人工探针 JSON；动态视频/整窗动画/纯性能窗做固定点金图会假绿。逐轮 JSON 与 stderr 已归档。

