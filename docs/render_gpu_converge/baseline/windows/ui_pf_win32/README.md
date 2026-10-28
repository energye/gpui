# ui_pf_win32

- 来源：`windows.md` §3.3（base）
- 目录：`examples/ui_pf_win32`
- 运行：`M`（mode=M，seconds=None，batch=False）
- 金图：—
- 容差：`—`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：Windows/—

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | SKIP | 0 |  |  |  |  | 0 | 0 | window reports SKIP |
| default | go | run | SKIP | 0 |  |  |  |  | 0 | 0 | window reports SKIP |

## 为什么无金图

该窗按 `windows.md` 无固定金图（能力标签：Windows/—）。替代判定是窗口自身的逻辑探针、像素断言、性能门禁或人工探针 JSON；动态视频/整窗动画/纯性能窗做固定点金图会假绿。逐轮 JSON 与 stderr 已归档。

## 平台例外

本机不是 Windows/macOS；窗按设计输出 SKIP，验证的是“明确拒绝、不崩”，不得标平台绿。

