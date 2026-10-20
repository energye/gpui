# R1 单能力窗关闭（已关，2026-10-20）

> 顺序 4。**现状：`examples/ui_wr_r1_layout` 已建并关闭（1200×800·30s，两跑 PASS，次跑 Golden `0.0000%/421120px`）**。§2 主表 R22/R15/R1 均已 ✅，R 能力 27/27。R1 表下限 5s；因 U21 steady/recover 两次快照读回在 5s 窗折合 24/min 会误触 `hitch≤5/min`，关闭证据加长到 30s（2/30s=4/min），门禁阈值未放大。

## 问题

R1（局部 NeedsLayout）此前是 §2 唯一 ⬜：改一个子树的高度只脏该 relayout boundary 的约定没有真窗锁着，改坏布局隔离没人拦。R22（选区/光标局部脏预留）与 R15（UI/raster 300s 长跑）已在此前关闭。

## 验收

- 独立包 `examples/ui_wr_r1_layout`（1200×800），与 §2 R1 一行一一对应。
- `RUN_SECONDS=30` 关闭；`<30` 只做调试，不作关闭证据。
- U21 三证据：逻辑探针（`layout_count`/`NeedsLayout`/offset）+ 像素断言 10/10 + Golden 静态掩码 421120px 逐位零容差；另加 steady↔recover 同静态掩码 0。
- 门禁（详见窗 README）：`layout_delta==height_changes==4`、`layout_isolation_violations=0`、neighbor/frame/chip shift=0、target=120/marker y=36、fps_interval≥55、p95≤22ms、hitch≤5/min、CPU 非双 0、`vsync_source=true`、`gpu_fallbacks=0`、两张快照非空白。
- `present_policy=full_paint`：R1 测布局隔离，不冒充 retained 局部 damage；30s 的 RSS slope 只作观测，泄漏结论归 R15/C10 300s soak。

## 关闭证据（2026-10-20）

| 项 | 第 1 跑（基线） | 第 2 跑（Golden 对比） |
|----|-----|-----|
| `layout_count` / `layout_delta` | 1→5 / 4 | 1→5 / 4 |
| `layout_isolation_violations` | 0 | 0 |
| shifts（neighbor/frame/chip） | 0/0/0 | 0/0/0 |
| target / marker y | 120 / 36 | 120 / 36 |
| `fps_interval` / `p95` | 59.69 / 17.06ms | 59.72 / 17.05ms |
| `hitch_rate_per_min` | 4.01（2 次快照） | 4.01（2 次快照） |
| 像素断言 | 10/10 | 10/10 |
| Golden | 首跑存基线 | `0.0000%/421120px` |
| steady↔recover 静区 | `0.0000%/421120px` | `0.0000%/421120px` |

两跑均 `exit=0`、`presents=1790/1791`、`vsync_source=true`、`gpu_fallbacks=0`。

## 关键修复 / 口径

- **boundary 位置**：引擎 `MarkNeedsLayout` 标父链、遇父 boundary 停（`pipeline_test.go:TestRelayoutBoundary_StopsBubble`），所以 `targetFrame` 设 `SetRelayoutBoundary(true)`，不是给会变高的 `target` 设；四次变化只允许 target + boundary frame 脏。
- **快照空白**：`SnapshotAsync` 内直接 `Context().Image()` 取到 present 后的白图；改为 `PaintPresentTree` 重画 + `SavePNG`（与 C4 U21 批量快照同入口）；快照期间 ticker 暂停树变更，raster 读稳定树，steady 蓝灯/recover 绿灯像素锁死。
- **CPU fallback**：`cpu_fallback_ops≈1/frame` 为静态标签 `text:color-layout` 既有路径，`gpu_fallbacks=0`，不是 R1 布局回调。
- **内存**：30s 窗 `rss_start≈74MB→286–287MB`、slope≈66kKB/min 属启动爬坡观测，不标泄漏结论。

## 优化收敛

已做：探针与门禁去重（10 项像素覆盖 target/marker/slot/sibling/neighbor/stripe/hot/双灯/文本）；Golden 与 steady↔recover 共用同一静态掩码；快照只读 PNG，不逐帧碰 GPU backbuffer；相位与高度变化错开，终帧稳定落 Recover。

## 原文档同步清单

- `ENGINE_UI_WIDGET_RENDER.md` §2 R1 ⬜→✅、§2.5/§5 关闭时长 5→30、§10 加行；R 能力 27/27。
- `RENDER_FEATURE_COVERAGE.md` §5 目录计数 46/45 与 R 能力 27/27、§11 加行。
- `LEGACY_00_M0_M5_BATCH.md` 顺序 4 状态更新。
- `docs/antd/F0_FOUNDATION_PLAN.md` 两处旧状态（R1 未关）改为已关。
- W6 已于 2026-10-20 关闭：C4/C5 Golden 已重定并回写，不属于 R1。
