# S6.1 — 帧模型强制

> 版本：1.0 | 日期：2026-07-15  
> 状态：**S6.1 关闭**  
> 依赖：S5.2 `docs/S5_FRAME_MODEL.md` + S6.0 `docs/S6_PERF_BASELINE.md`  
> 架构：`render.BeginFrame / Invalidate / PresentFrameAuto` → `PresentFrame*` → webgpu → rwgpu → libwgpu_native

---

## 1. 目标

把 S5.2「可以 damage present」升级为 **默认强制帧模型**：

1. **Idle 帧**：无脏区 → 不 FlushGPU、不调 present 回调  
2. **局部 invalidation**：稳态帧只画脏区 + damage present  
3. **禁止无意义全清屏**：应用默认走 `PresentFrameAuto`，不要每帧 `PresentFrame`  
4. **全量 redraw 有明确 API**：`PresentFrameFull` / 高覆盖率自动 promote  
5. **Damage 合并策略 + multi vs union 选择**写进代码与测试  

**非目标**：控件层、改 batch 算法、改 path/text 内核（那些是 S6.2+）。

---

## 2. 默认 UI 帧（强制）

```text
Bootstrap / resize / mode-switch:
  NewContext → 全量绘制 → PresentFrameFull(view)

Steady frame:
  dc.BeginFrame()                  // = ResetFrameDamage
  // 只绘制脏控件；Fill/Stroke 自动 TrackDamage
  // 或 compositor: dc.Invalidate(logicalRect)
  outcome, err := dc.PresentFrameAuto(view, W, H, present)

Idle (无输入、无动画、无脏区):
  dc.BeginFrame()
  // 不绘制
  PresentFrameAuto → outcome.Idle == true（跳过 GPU 与 present）
```

### API 表

| API | 角色 |
|-----|------|
| `BeginFrame` | 稳态帧入口：清空 damage |
| `Invalidate` | 逻辑脏区（HiDPI→物理）；`TrackDamageRect` 别名 |
| `MarkFullRedraw` | 整窗逻辑脏；高覆盖时常 promote 到 Full |
| `PlanPresent` / `PlanFramePresent` | pure 决策：idle / multi / union / full |
| `CoalesceDamageRects` | 相交/贴边合并；超 cap 塌缩为 union |
| `PresentFrameAuto` | **默认 present**（按 plan 分派） |
| `PresentFrameFull` | **显式全量** redraw（= `PresentFrame`） |
| `PresentFrameDamage*` | 底层单/多 rect（Auto 内部使用） |

### 硬规则

1. 新应用代码 / examples 稳态路径 **优先** `PresentFrameAuto`。  
2. 宣称 60fps 仍只允许 **present-only**（S5/S6 硬规则）。  
3. HiDPI：`Invalidate` / 自动 track 输出 **物理像素** damage。  
4. MSAA：仍 ADR-021（scissor∩damage；不在 MSAA 上赌 LoadOpLoad）。  
5. 不为刷分改像素门禁或 silent CPU。

---

## 3. Damage 合并与 multi / union / full

| 条件 | Mode | Present 路径 |
|------|------|----------------|
| 无非空脏区 | `idle` | 跳过 Flush + present |
| 脏像素估计（Σ rect 面积，或单 rect / 簇集 union）≥ **85%** 表面 | `full` | `PresentFrameFull` |
| 仅 1 个 rect | `damage_union` | `PresentFrameDamage` |
| ≥2 rect 且 union/sum(areas) ≥ **1.35**（空隙大） | `damage_multi` | `PresentFrameDamageRects` |
| ≥2 rect 且簇集/重叠 | `damage_union` | 单 union rect |
| track 中 rect 数 > **16** | 累加时已塌缩为 1 个 bbox | — |

常量（`render/frame.go`）：

- `MaxTrackedDamageRects = 16`  
- `DamageFullCoverageThreshold = 0.85`  
- `DamageMultiWasteRatio = 1.35`  

`CoalesceDamageRects`：相交或 **贴边（1px 膨胀）** 的 rect 合并，避免碎 rect 风暴。

---

## 4. 全量 redraw 的合法路径

| 场景 | API |
|------|-----|
| 冷启动 bootstrap | `PresentFrameFull` |
| 窗口 resize / scale 变化后首帧 | `PresentFrameFull` |
| 故意全窗重绘对照（H01） | `PresentFrameFull` |
| 脏区已覆盖 ≥85% 表面 | `PresentFrameAuto` → `full` |
| 需要 LoadOpLoad 的「整窗 damage」且不要 clear | 显式 `PresentFrameDamage(fullRect)`（罕见） |

---

## 5. 测试

| 测试 | 作用 |
|------|------|
| `TestPlanFramePresent_*` / `TestCoalesce*` / `TestBeginFrame*` / `TestInvalidate_HiDPI*` | 策略纯函数 + API（可无 GPU） |
| `TestS61_PresentAuto_IdleSkipsGPU` | idle 零 GPU / 不调 present |
| `TestS61_PresentAuto_LocalDamageMulti` | 局部脏区非 full |
| `TestS61_PresentFull_ExplicitPath` | 显式 full 路径 |
| `TestS61_U01like_DamageVs_H01like_Full` | damage vs full 对照计量 |
| `TestS61_PlanPresent_MatchesFrameDamagePhysical` | HiDPI 物理 plan |
| `TestS61_L0_MainPathHelpersStillGreen` | Auto 主路径 p50 软门禁 ≤16.7ms |

回归（S6 契约 L0/L1 抽样）：

```bash
export WGPU_NATIVE_PATH=.../lib/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache
export LD_LIBRARY_PATH=.../lib:$LD_LIBRARY_PATH

go test -count=1 ./render -run 'TestPlanFramePresent_|TestCoalesce|TestBeginFrame_|TestInvalidate_HiDPI|TestMarkFullRedraw|TestS61_' -timeout 180s
go test -count=1 ./render -run 'TestS6_L0_|TestS52_|TestS53_' -timeout 300s
go test -count=1 ./render -run 'TestP1_Comp_(D01|D06|D08|D36|D63|D152)_' -timeout 300s
```

---

## 6. 退出条件

| 条件 | 状态 |
|------|------|
| 默认 API：`BeginFrame` / `Invalidate` / `PresentFrameAuto` / `PresentFrameFull` | ✅ |
| multi vs union vs full 策略代码化 + 单测 | ✅ |
| HiDPI 物理脏区 | ✅（既有 track + 本切片 Invalidate） |
| 全量 redraw 明确路径 | ✅ `PresentFrameFull` |
| Idle 跳过无意义 GPU | ✅ |
| L0/L1 抽样绿；U01–U04 不因本切片回退 | ✅（本切片强制 helpers + 既有 S6/S5 门禁） |
| 无控件层 | ✅ |

**S6.1 关闭。** 下一：**S6.2 录制/提交 CPU 路径**。
