# ui_wr_c0_smoke — W0 C0 R0+R12+R16 集成窗

## 运行

```bash
export DISPLAY=:0 LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=5 go run ./examples/ui_wr_c0_smoke
```

窗口 1200×800（U15），GPU 真窗必需，`RUN_SECONDS>=5`（U16，§2.5：C0 属正确性/schema 类 5s）。

## 集成内容（§3 组合表 C0）

> **C0 = R0+R12+R16 集成；只做集成，不能代替任何单窗。**

| 能力 | 集成内容 | 窗内呈现 |
|------|----------|----------|
| **R0** | FullPaint 静态存活 | 6×4 密集色格 + 12 静态文 + HOT 相位动块（静+动同屏，动画期间静态每帧存活） |
| **R12** | 指标 schema | SCHEMA 大字实时滚动字段名（`SCHEMA: 42 FIELDS · <key>`），结束时 42 键逐一核对 |
| **R16** | 首帧/WarmUp | 开机即见内容横幅 `FIRST FRAME CONTENT OK`；warmup 观测 + first_present_paint_count 证明首帧不黑 |

## 门禁（Gate）

| 门禁 | 阈值 | 类型 |
|------|------|------|
| policy | `present_policy == full_paint` | 硬 FAIL |
| presents | `present_count >= 1` | 硬 FAIL |
| 首帧内容不黑（R16） | `warmup == true` 且 `first_present_paint_count > 0` | 硬 FAIL |
| 静态每帧存活（R0） | `paint_count >= presents*0.9`（full_paint 全树重画） | 硬 FAIL |
| 指标全族字段齐（R12） | wrgate 30 键 + §2.2.1 A–J 全族 42 键全存在 | 硬 FAIL |
| 持续 tick FPS | `fps_interval >= 55`（§2.2.2 正确性类，运行 ≥5s） | 硬 FAIL |

## 实现点（六维）

1. **窗口**：1200×800 + wrkit 骨架（TopBar/Legend/Body/HUD），U17 达标
2. **集成**：R0 静态网格+文+动块 / R12 SCHEMA 滚动大字 / R16 首帧横幅 + WarmUp 观测——三能力同窗共存
3. **指标**：§2.2 全族经 wrgate.BuildReport；H 族观测由 PipelineApp 首帧发布
4. **schema**：`checkAllKeys`（42 键）+ `wrgate.CheckSchema`（30 键）双重核对
5. **诚实**：字段值全部来自观测；集成窗不伪造任何单窗门禁结果（单窗各自仍有独立真窗）
6. **时长**：`RUN_SECONDS>=5`（U16），§2.5 正确性/schema 类 5s
