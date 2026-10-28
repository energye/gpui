# ui_ime_probe

- 来源：`windows.md` §3.3（base）
- 目录：`examples/ui_ime_probe`
- 运行：`M`（mode=M，seconds=None，batch=False）
- 金图：—
- 容差：`—`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：X11 或 Wayland/native

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PROBE | -15 |  |  |  |  | 0 | 0 | manual probe: WM_DELETE not honored; SIGTERM fallback used; process did not self-close; WM_DELETE/SIGTERM fallback used; manual probe: classified PROBE (no JSON gate) |
| default | go | run | PROBE | -15 |  |  |  |  | 0 | 0 | manual probe: WM_DELETE not honored; SIGTERM fallback used; process did not self-close; WM_DELETE/SIGTERM fallback used; manual probe: classified PROBE (no JSON gate) |

## 为什么无金图

该窗按 `windows.md` 无固定金图（能力标签：X11 或 Wayland/native）。替代判定是窗口自身的逻辑探针、像素断言、性能门禁或人工探针 JSON；动态视频/整窗动画/纯性能窗做固定点金图会假绿。逐轮 JSON 与 stderr 已归档。

## 人工探针

该窗是纯人工 IME 探针，无 JSON 门禁；采集时已抓取 stderr 探针行，WM_DELETE 未被响应，按 `AGENTS.md` 兜底 SIGTERM 结束；不标 PASS/FAIL。

