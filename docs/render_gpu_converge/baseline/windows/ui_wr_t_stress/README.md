# ui_wr_t_stress

- 来源：`windows.md` §3.2（ui_wr）
- 目录：`examples/ui_wr_t_stress`
- 运行：`A/8s`（mode=A，seconds=8，batch=False）
- 金图：—
- 容差：`—`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：文本/帧时 · 三线压力（无金图：压力窗）

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | BUILD_FAIL |  |  |  |  |  | 0 | 0 |  |
| default | go | run | BUILD_FAIL |  |  |  |  |  | 0 | 0 |  |

## 失败原因（原始 stderr 摘录）

- `default` / `native` / `run`：binary missing: /tmp/p14/bin/ui_wr_t_stress.bin
- `default` / `go` / `run`：binary missing: /tmp/p14/bin/ui_wr_t_stress.bin

## 为什么无金图

该窗按 `windows.md` 无固定金图（能力标签：文本/帧时 · 三线压力（无金图：压力窗））。替代判定是窗口自身的逻辑探针、像素断言、性能门禁或人工探针 JSON；动态视频/整窗动画/纯性能窗做固定点金图会假绿。逐轮 JSON 与 stderr 已归档。

## 失败说明

`go build` 失败：`ui_wr_t_stress` 导入 `ui/kit/button`、`ui/kit/icon`，当前 HEAD 没有这两个包；P1.4 只登记，修复留给 P2。

