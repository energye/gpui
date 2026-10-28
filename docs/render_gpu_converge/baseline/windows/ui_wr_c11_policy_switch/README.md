# ui_wr_c11_policy_switch

- 来源：`windows.md` §3.2（ui_wr）
- 目录：`examples/ui_wr_c11_policy_switch`
- 运行：`R=15s`（mode=R，seconds=15，batch=False）
- 金图：—
- 容差：`—`（0=逐位零容差；0*=首跑播种、次跑零容差；—=无金图）
- 能力标签：缓存/帧时 · full↔retained 热切换（无金图：策略窗）

## 采集结果

| 变体 | 后端 | 轮次 | 状态 | rc | 金图 diff | p95 ms | p99 ms | fps_interval | 快照 | 金图归档 | 备注 |
|---|---|---|---|---:|---|---:|---:|---:|---:|---:|---|
| default | native | run | PASS | 0 | 0.0000% | 19.01 | 19.84 | 57.32 | 2 | 0 | p99=19.84; vsync_source=true (not a source name) |
| default | go | run | FAIL | 1 | 0.0000% | 18.51 | 20.59 | 57.67 | 2 | 0 | exit code 1; p99=20.59; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch) |

## 失败原因（原始 stderr 摘录）

- `default` / `go` / `run`：exit code 1; p99=20.59; vsync_source=true (not a source name); GPUI_BACKEND=go: gpu_backend still integrated (present-only switch); hitch_rate_per_min=7.99 > 5; golden c11_final.png: diff=0.0000% over 390176 px; FAIL: hitch_rate_per_min=7.99 > 5

## 为什么无金图

该窗按 `windows.md` 无固定金图（能力标签：缓存/帧时 · full↔retained 热切换（无金图：策略窗））。替代判定是窗口自身的逻辑探针、像素断言、性能门禁或人工探针 JSON；动态视频/整窗动画/纯性能窗做固定点金图会假绿。逐轮 JSON 与 stderr 已归档。

## 无固定金图的替代判定

该窗 `windows.md` 标无金图；代码仍在 `C11_SNAP_DIR`（默认 `/tmp/c11_policy_switch`）做首跑播种/次跑比较。本轮只登记 `SEED`/原始 JSON，不把它当固定金图 PASS。

