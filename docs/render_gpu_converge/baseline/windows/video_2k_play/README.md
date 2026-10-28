# video_2k_play

- 来源：`windows.md` §3.5（video）
- 目录：`examples/video/video_2k_play`
- 运行：`R=30s`（mode=R，seconds=30，batch=False）
- 金图：—
- 容差：`—`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：视频 · 2K 直传

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | NO_JSON | 1 |  |  |  |  | 0 | 0 | no parseable JSON on stdout |
| default | go | run | NO_JSON | 1 |  |  |  |  | 0 | 0 | no parseable JSON on stdout |

## 失败原因（原始 stderr 摘录）

- `default` / `native` / `run`：no parseable JSON on stdout; FAIL: 缺片 p3_2k_2560_1440_30fps.mp4：先跑 bash video/testdata/gen_p3_2k4k.sh（大二进制不进仓库，本地与 CI 现场生成）
- `default` / `go` / `run`：no parseable JSON on stdout; FAIL: 缺片 p3_2k_2560_1440_30fps.mp4：先跑 bash video/testdata/gen_p3_2k4k.sh（大二进制不进仓库，本地与 CI 现场生成）

## 为什么无金图

该窗按 `windows.md` 无固定金图（能力标签：视频 · 2K 直传）。替代判定是窗口自身的逻辑探针、像素断言、性能门禁或人工探针 JSON；动态视频/整窗动画/纯性能窗做固定点金图会假绿。逐轮 JSON 与 stderr 已归档。

