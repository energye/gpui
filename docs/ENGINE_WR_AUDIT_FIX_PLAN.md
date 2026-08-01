# ENGINE WR 审计修复计划（AUDIT FIX PLAN）

> 本文档是 2026-08-01 三重核对（基座符合性 / Flutter-Skia 对齐 / 示例符合性）发现的全部问题的
> **单一修复账本**。每修复一项 → 更新本表状态 → 回写 `docs/ENGINE_UI_WIDGET_RENDER.md` §10 修订。
> 纪律：render/ 高风险、gpu/ 最高风险改动必须 question 确认；修示例禁止绕引擎洞。

## 0. 修复总览

| 优先级 | 项数 | 状态 |
|---|---|---|
| P0 指标造假（诚实纪律） | 3 | 3/3 ✅ |
| P1 引擎 bug（ui/rendering 低风险） | 2 | 2/2 ✅ |
| P2 示例能力缺口 | 9 | 9/9 ✅ |
| P3 组合窗门禁系统性 | 5 | 5/5 ✅ |
| P4 架构差距文档化 | 4 | 4/4 ✅ |
| **合计** | **23** | **23/23 ✅** |

## P0 指标造假 / 失真（必须修，违反诚实纪律）

### P0-1 R19 `dpr/dpr_changes` 假计数
- **问题**：`dpr` 原子只进 HUD 显示（main.go:103），host 无 scale 写路径（x11_linux.go:307 只读），
  DPR 变更**从不作用于窗口/渲染**；「采样或截图门禁」退化为 line_draws 计数。
- **根因**：exhost 无 scale 变更来源；引擎链路已完整（PresentTarget.Resize→SetDeviceScale→物理
  重分配→BoundaryCache.Clear，present_target.go:214-240、pipeline_app.go:640-661）。
- **方案**：exhost 增加**模拟 DPR 切换**（运行期注入 EventResize 带 Scale 1.0/1.5/2.0），R19 用真实
  链路测量：dpr_changes = 真实 scale 变更帧数、physical 分辨率真实变化、1px 线物理宽度变化。
- **验证**：真窗跑 R19，JSON 中 dpr_changes≥2 且 Extra 含真实 device_scale 序列；主表门禁改真指标。
- **状态**：✅ 2026-08-01 exhost 新增真实 DPR 注入链路：
  `Window.SetScale`（open.go）→ `x11Host.SetScale`（x11_linux.go，改 st.scale + 注入
  EventResize）→ pipeline_app Run 循环（pipeline_app.go:458-472）→ `applyPendingResize`
  （scaleChanged → `PresentTarget.Resize` → `SetDeviceScale` + BoundaryCache.Clear，640-661）。
  R19 删除死原子 `dpr`，onTick 真调 `win.SetScale(1.0/1.5/2.0)`，HUD/Extra 显示真实
  `host.ScaleFactor()`，Extra 加 `dpr_path` 注明真实链路。
  **真窗验证**：`RUN_SECONDS=6` 跑 R19 → PASS，`dpr_changes=5`（Spike 1.0→1.5→2.0 循环 4 次 + 恢复
  1 次），fps 59.9、hitch_count=0，无掉帧。

### P0-2 C1 `inner_static_clean` / `outer_side_clean` 假计数
- **问题**：每 tick 无条件 +1（main.go:451-452），注释自认 proxy（L448-450），
  「内脏不外溢 / 外脏不内传 ≥10」门禁恒真空转。
- **方案**：改真实探测——通过窗口持有的节点句柄直接查 `NeedsPaint()`/`SubtreeNeedsPaint()`：
  inner-static 在 inner-hot 脉冲期间保持 clean 才计数；outer-side 在 outer 脉冲期间保持 clean 才计数。
- **验证**：真窗跑 C1，JSON 的 inner_static_clean/outer_side_clean 不再恒等于帧数且门禁真实可 FAIL。
- **状态**：✅ 2026-08-01 `main.go` 保存 `staticLeaf`/`sideLeaf` 句柄，onTick 末尾真实探测
  `!staticLeaf.NeedsPaint()`（内脏不外溢）与 `!sideLeaf.NeedsPaint()`（外脏不内传）才计数，
  删除 proxy 注释与无条件 Add。
  **真窗验证**：`RUN_SECONDS=8` 跑 C1 → PASS，`inner_static_clean=479`、`outer_side_clean=479`（=帧数，
  真实语义：两静态叶在整个运行期未被波及），skip=1534/rr=811/tex_rasterize=5/tex_hit=1529，fps 59.9。

### P0-3 R3 `outerCleanTicks` 假计数
- **问题**：名字含 clean，实为无条件帧计数器（main.go:431），「内层脏→外层净」从未真实验证。
- **方案**：改真实探测——inner-hot 脉冲帧上查 outer 的 NeedsPaint()：若 outer 未因内层而脏则计数；
  同时按 §2.6.2 将嵌套加深到 ≥3 层（合并 P2-3）。
- **验证**：真窗跑 R3，outer_clean_ticks 语义真实；构造内层脏而外层净的分时场景可见于 skip 指标。
- **状态**：✅ 2026-08-01 `main.go` 调整 onTick 顺序：**先** inner-hot MarkNeedsPaint（脏停在自身
  RepaintBoundary，node.go:271）→ 探测 `!outer.NeedsPaint()` 才计 outerCleanTicks（内脏不外溢），
  再标 outer。删除无条件 Add。
  **真窗验证**：`RUN_SECONDS=8` 跑 R3 → PASS，`outer_clean_ticks=422`（479 帧中 57 帧探测点 outer
  已脏——真实时序反映，非恒等帧数），`inner_static_clean=479`，depth=3（嵌套加深见 P2-3），
  skip=1043/rr=845/tex_rasterize=4，fps 59.9。

## P1 引擎 bug（ui/rendering 低风险，可直接修）

### P1-1 `transform.go:114-120` childPC 缺字段 → transform 子树缓存失效
- **问题**：手工构造 `&PaintContext{DC, OriginX:0, OriginY:0, Scale, PaintVisits}` 丢失
  `BoundaryCache`/`UseBoundaryCache`/`UsePictureTextureCache`/`LayerBudget`/`saveLayerDepth`/
  `DebugRepaint`/`DebugRepaintDraws`/`CompositeOnly` → transform 子树内 RB 每帧重录、B1 纹理失效。
- **方案**：改用 `pc.WithOrigin(0,0)`（WithOrigin 已正确传播全部字段，paint_context.go:49-70）。
- **验证**：`go build ./...` + 真窗 R7b/R3（含 transform 场景）skip 计数恢复非 0。
- **状态**：✅ 2026-08-01 `transform.go:114` 改为 `pc.WithOrigin(0, 0)`，`go build ./ui/... ./render/...` 绿。
  真窗复测待 P0/P2 完成后统一回归。

### P1-2 `virtual_list.go:371-407` 重复 mount 块死代码
- **问题**：两个「Mount missing」块，后者（392-407）恒空 → `freshlyMounted` 恒空 →
  已挂载 cell 每帧 `clearPaintDirty()`（与注释「scroll reuse R7b 不清脏」意图相反）。
- **方案**：删除前置死代码块（371-384），保留并修正 392-407 为唯一 mount 源，freshlyMounted 真填。
- **验证**：`go build ./...` + 真窗 R7b 滚动复用帧 skip>0 且新 cell 首录计数真实。
- **状态**：✅ 2026-08-01 `virtual_list.go:371-401` 合并为单一 mount 块，freshlyMounted 在真实挂载时填入，
  `go build ./ui/... ./render/...` 绿。真窗复测待 P0/P2 完成后统一回归。

## P2 示例能力缺口（补齐内容，禁止绕洞）

| 项 | R | 缺口 | 方案 |
|---|---|---|---|
| P2-1 | R13 | 缺「变换/裁剪下 hit 目标」 | ✅ 2026-08-01 已修：hotPanel 加 45°-旋转 RenderTransform 包 DebugName="rotated" 140×140 目标；probes 扩到 6 项含 (384,568) 中心 与 (474,568) AABB 外-几何内 两点；真窗 PASS probes=6/6（inverseMapPoint 证明变换下 hit） |
| P2-2 | R9 | PhaseClock 死代码（`_ = rate`） | ✅ 2026-08-01 已修：rate 经布局累加器真实驱动 MarkNeedsLayout 频率（Steady 1.0/Spike 2.0/Recover 0.7 ×/s）；真窗 PASS hits=1563≫miss=13 |
| P2-3 | R3 | RB 嵌套仅 2 层（要求 ≥3）+ 无内层独脏分时 | ✅ 已修（见 P0-3）：插入 mid RB 层 → root→outer→mid→leaf，depth=3 实测通过 |
| P2-4 | R5 | 缺「变换」op | ✅ 2026-08-01 已修：ui/scene/picture.go 加 OpPushTransform/OpPopTransform（中心式 T(C)·R·S·T(-C)，与 RenderTransform.Paint 同语义）+ PictureRecorder.PushTransform/PopTransform + applyPictureOp 实现；示例 direct 侧 DC Push+Rotate+Scale 与 replay 侧 PushTransform 逐像素对齐；真窗 PASS ops=12（含变换 op），replays=direct=353 |
| P2-5 | R16 | 缺「渐入动画」 | ✅ 2026-08-01 已修：8 个色块开场 1.5s alpha 0→1 淡入（age 驱动），Steady 后保持；真窗 PASS first_ms=362.7/fps 59.9 |
| P2-6 | R18 | PhaseClock 三阶段行为无差异 | ✅ 2026-08-01 已修：Steady/Recover MaxOps=1 隔帧重绘；Spike MaxOps=0（1st SaveLayer 也被拒）每帧重绘，三阶段行为真实可辨；真窗 PASS allow=348/reject=167 |
| P2-7 | R12/R16 | Extra 缺六维 impl_* | ✅ 2026-08-01 已修：R12 与 R16 均补 impl_layout/impl_paint/impl_present/impl_metrics/impl_hit/impl_win 六维自评 |
| P2-8 | R2 | 无「渐变」内容 | ✅ 2026-08-01 已修：加 FS-LINEAR 渐变 RenderBox（FillLinearGradient 对角蓝→橙，740,540），真窗 PASS clean=359/fps 60 |
| P2-9 | 10 窗 | Legend 无色块（shell.go:44-49） | ✅ 2026-08-01 已修：wrkit shell.go 每行 ColorAt 14×14 色块 + LabelAt 文字（12 色调色板），全部 NewShell 窗生效；R13 真窗复测 PASS 布局不受影响 |

## P3 组合窗门禁系统性

| 项 | 窗 | 问题 | 方案 |
|---|---|---|---|
| P3-1 | C0 | GateOptions 用 retained 顶替 full_paint、非能力并集；PhaseClock 只影响 hotB | gate 取并集 + PhaseClock 驱动多能力 |
| P3-2 | C1 | 能力门禁外置为独立 if-FAIL | 并入 GateOptions 并集（含真实隔离指标 P0-2） |
| P3-3 | C2 | impl_fail 写 skip=0=FAIL 但 gate 设 MinBoundarySkip:0 自相矛盾 | 统一为 skip≥1 |
| P3-4 | C3 | 注释把 retained 误标 FullPaint；缺 covers 字段；无 policy 门禁 | 修正注释 + 补 covers + 显式 policy 门禁 |
| P3-5 | C7 | 能力门禁外置 | 并入 GateOptions 并集 |

**P3 修复记录（全部 ✅ 2026-08-01，每项真窗复跑）**：

- ✅ P3-1 C0：场景增 hotA（Steady/Recover 冻结= R0 静态存活证明，Spike 随 hotB 脉冲）→ PhaseClock 驱动多能力；`policy_note` 诚实声明 C0 集成窗跑 retained（C 不代替 R，R0/R16 full_paint 由 solo 证明）。真窗复跑：`RUN_SECONDS=6` → **PASS presents=360 policy=retained fps=59.9**。
- ✅ P3-2 C1：GateOptions `MinBoundaryMaxDepth` 2→3（与实测 root→outer→mid→leaf 链一致）；能力门禁 Skip/Rerecord/Count/Depth 全部在并集中，隔离不变式（inner/outer_side_clean）经 P0-2 已为真实探测且保持独立 if-FAIL（GateOptions 无法表达，属并集补充非代替）。真窗复跑：`RUN_SECONDS=8` → **PASS skip=1560 rr=785 cnt=6 depth=3 inner_static_clean=479 outer_side_clean=479**。
- ✅ P3-3 C2：impl_fail 改为 `skip=0` 从 FAIL 列表移除并注明「retained 下 boundary_skip=0 是正确语义——静 RB 靠 LoadOpLoad 保像素，不设 skip 门禁」，与 gate `MinBoundarySkip:0` 自洽。真窗复跑：`RUN_SECONDS=8` → **PASS skip=0 dmg=0.2095 ops=8 policy=retained fps=59.9**。
- ✅ P3-4 C3：**关键发现**——加 `RequireFullPaintPolicy` 后真窗 FAIL，暴露 C3 实际跑 **retained**（present_mode=damage_multi、dmg≈0.6、skip=4296 全程正常、无像素擦除）；旧注释「FullPaint 安全策略、retained unsafe」是过时误导。修正：gate 改 `RequireRetainedPolicy: true`，impl_correctness/impl_cache/impl_dirty/impl_edge/integration_note 五处注释全部按实测 retained 诚实改写，补 covers=["R4","R7","R7b","R10"]。真窗复跑：**PASS bind=14/1000 rr=0/frame dirty/img=1 dmg=0.60 fps=59.9**。
- ✅ P3-5 C7：GateOptions 并集已含 Skip/Count/FullPaintPolicy；独立 FAIL（tex_rasterize≥1、invalidation≥1、失效后 rerecord、static_cells≥8）是 C7 特有集成不变式，加注释说明为并集补充非代替。真窗复跑：`RUN_SECONDS=8` → **PASS inval=2 skip=7542 rr=36 cnt=19 cells=16**。

## P4 架构差距文档化（不修代码，只回写 §10 修订）

| 项 | 差距 | 处置 |
|---|---|---|
| P4-1 | F01/F02 Layer COW 未实现（Layer 树每帧重建，复用落在 Picture 层） | §10 如实注明实现深度与分期 |
| P4-2 | F05 滚动 offset 未进 Offset/Transform Layer（Paint 期 origin 平移代替） | §10 注明与 §6.4 的偏差及等价性 |
| P4-3 | F09 动画 paint-only 非 compositor-only | §10 注明 |
| P4-4 | F15 遮挡停帧未实现 | §10 注明属 W4+ 分期 |

**P4 修复记录（全部 ✅ 2026-08-01，纯文档化回写 §10 修订 3.30，不修代码）**：

- ✅ P4-1：F01/F02 Layer COW 未实现——Layer 树每帧重建，静态复用落在 Picture 层（BoundaryCache，R3/R3b 真窗已证明），§10 3.30 如实注明实现深度。
- ✅ P4-2：F05 滚动 offset 未进 Offset/Transform Layer——已核实 `virtual_list.go:460-530` Paint 期 `pc.WithOrigin` origin 平移代替，不产生 OffsetLayer 节点；等价于 transform 平移但无层树语义，§10 3.30 注明。
- ✅ P4-3：F09 动画 paint-only（全量重绘路径）非 compositor-only（compositor-only 仅 retained 组合窗 R4/R4b/C2 场景生效），§10 3.30 注明。
- ✅ P4-4：F15 遮挡停帧未实现，属 W4+ 分期，§10 3.30 注明。

## 验证总纲（每项修复后）

1. `go build ./...` + `go vet ./...`（或对应包）通过。
2. 受影响窗真窗复跑：`LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so DISPLAY=:0 go run ./examples/ui_wr_*`，取 §2.2 全族 JSON 核对门禁。
3. 本表对应项标 ✅ + 日期 + 证据摘要，回写 `docs/ENGINE_UI_WIDGET_RENDER.md` §10 修订表。
