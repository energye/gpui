# ui_wr_r16_warmup — W0 R16 首帧/WarmUp/恢复

## 运行

```bash
export DISPLAY=:0 LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=5 go run ./examples/ui_wr_r16_warmup
```

窗口 1200×800（U15），GPU 真窗必需，`RUN_SECONDS>=5`（U16，§2.5：5s / 观察 15s）。

## 可见效果（U17）

| 区域 | 内容 |
|------|------|
| TopBar | 标题 `R16 首帧/WarmUp/恢复 — 首帧有内容·恢复不黑` |
| Legend | 5 行说明：WarmUp 首帧 / 首帧有内容 / t2f 观测 / 中段模拟恢复 / 恢复后 paint 继续增长 |
| Body 左 | 6×4 静态色格（RepaintBoundary）+ 12 条静态文本（**首帧即完整呈现**） |
| Body 中 | HOT 动块：相位驱动变位变色（Steady 红 / Spike 黄 / Recover 蓝） |
| HUD | t2f（time_to_first_present_ms）+ p1（first_present_paint_count）实时显示 |

运行中段（约 1s 后）：触发 `InvalidateBoundaryCache()`（全树 MarkNeedsPaint + force full，
即遮挡/Expose 恢复的重画路径），静态内容与动块必须继续每帧存活——**恢复不黑**。

## 门禁（Gate）

| 门禁 | 阈值 | 类型 |
|------|------|------|
| 真窗 Present | `present_count >= 1` | 硬 FAIL |
| 首帧 full | `warmup == true`（PipelineApp 观测：Open 时 presentSyncFull 真实执行） | 硬 FAIL |
| 首帧有内容 | `first_present_paint_count > 0`（首帧 paint 指令数） | 硬 FAIL |
| t2f 观测 | `time_to_first_present_ms > 0`（Open→首 Present 墙钟） | 硬 FAIL |
| policy | `present_policy == full_paint`（W0 默认） | 硬 FAIL |
| 持续 tick FPS | `fps_interval >= 55`（正确性+持续 tick，§2.2.2，运行 ≥5s） | 硬 FAIL |
| 恢复不黑 | `recovery_triggered` 且恢复后 `paint_count` 继续增长（`recovery_repaint_ok`） | 硬 FAIL |

## 实现点（六维）

1. **窗口**：1200×800 + wrkit 骨架（TopBar/Legend/Body/HUD），U17 达标
2. **指标**：H 族观测（`warmup` / `time_to_first_present_ms` / `first_present_paint_count`）由 PipelineApp `recordFirstPresent` 在首个 Present 一次性发布（非手动编造）；§2.2 全族经 wrgate.BuildReport
3. **首帧 full**：`WarmUp: true` → Open 时 `presentSyncFull`（全清 + 全树 paint）→ 观测 warmup=true
4. **首帧有内容**：首帧 paint 指令数 >0（静网+文+动块都在首帧），证明开机不是黑屏/空帧
5. **恢复不黑**：中段 `InvalidateBoundaryCache` 模拟遮挡恢复；恢复后全树重画继续，paint_count 增长、内容存活
6. **时长**：`RUN_SECONDS>=5`（U16），§2.5 推荐 5s / 观察 15s
