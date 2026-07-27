# L1 UI 引擎收口说明（P0–P3）

> **版本：1.4** | 日期：2026-07-27  
> **状态：✅ 已收口 · P0–P3 已实现 · 可验收**  
> **范围：** L1（`ui/` + `render.PresentTarget`）  
> **不在本收口：** P4 滚动/IO、L2 手势焦点、L3 Ant 控件  

**单测（收口当日）：** `go test ./ui/... -count=1` 全绿。

---

## 1. 结论

```text
已完成：Flutter 式 L1 主路径（调度 · RO · Layer/Boundary · 异步 Raster · 指标 · 真窗示例）

可以宣称：
  · 引擎级「原生手感」主门禁达标
  · 脏区 ∝ 动画区域；Spinner 不带动全树 layout
  · UI 不因 Present 同步阻塞；帧可 JSON 观测

不可以宣称：
  · 长列表/文本/IO 已完成（P4）
  · 控件库完成（L3）
  · 数学全局最优 CPU/FPS
```

---

## 2. 阶段

| 阶段 | 内容 | 状态 |
|------|------|------|
| P0 | 句柄 · Scheduler · Raster 线程 · 清色 Present | ✅ |
| P1 | RO · Constraints · 坐标/Y-down · PaintingContext | ✅ |
| P2 | Layer · RepaintBoundary · FramePacket COW · CompositeOnly | ✅ |
| P3 | Animation · Spinner · 脏层统计 · SubmitLatest · PipelineApp | ✅ |
| P4 | 滚动/虚拟列表/IO | ✅ [`ENGINE_PHASE_P4.md`](./ENGINE_PHASE_P4.md) |

任务细节：P0–P3 [`ENGINE_PHASE_P0_P3.md`](./ENGINE_PHASE_P0_P3.md) · P4 [`ENGINE_PHASE_P4.md`](./ENGINE_PHASE_P4.md)

---

## 3. 代码地图

```text
ui/
  platform/    Host · NativeSurface · Event · StubHost · VSyncWaiter
  scheduler/   Mode · ScheduleFrame · Ticker · FrameMetrics(JSON)
  raster/      Loop depth=2 · SubmitLatest · pending 覆盖
  embedder/    App(清色) · PipelineApp(树+异步 present)
  rendering/   RO · Box · ColorBox · Spinner · PipelineOwner · BuildLayerTree
  painting/    Context（逻辑 px · Y-down · CompositeOnly）
  scene/       Layer · FramePacket COW · RasterizeDirty · Overlay band
  animation/   Controller（完成自动卸 Ticker）
  depcheck_test.go

render/present_target.go   ← ui 唯一 GPU 入口（内部调 gpu）

examples/ui_l1_blank | ui_l1_spinner
```

```text
ui → render → gpu     （禁止 ui → gpu）
```

---

## 4. 验收命令

```bash
# 单测
go test ./ui/... -count=1

# 真窗（需 DISPLAY）
export LD_LIBRARY_PATH=$PWD/lib
export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
go run ./examples/ui_l1_blank
go run ./examples/ui_l1_spinner
```

| 示例 | 期望 |
|------|------|
| blank | 可见清色窗；stdout JSON；presents>0 |
| spinner | 可见旋转；`layout_flushes` 远小于 presents；`raster_layer_count` 小 |

建议把 JSON 存为 **baseline**，供以后优化对比。

---

## 5. 门禁摘要

| 项 | 状态 |
|----|------|
| IDLE 可阻塞 | ✅ |
| Boundary + CompositeOnly | ✅ |
| Spinner 不刷 layout | ✅ |
| SubmitLatest 不堵 UI | ✅ |
| FrameMetrics JSON | ✅ |
| 逻辑/物理/Y-down | ✅ |
| 滚动 1k / IO 解码 | ✅ P4（S5/S6 测 + ui_l1_scroll） |
| 真 vsync（非 fallback） | ⚠ 接口有，多 fallback |
| per-layer GPU RT 池 | ⚠ 语义有，可再加深 |

---

## 6. 已知限制

1. 虚拟列表 MVP 为 **固定行高**；可变高 / 完整 sliver 未做  
2. 文本为引擎级最小 DrawString，非 IME 编辑器；atlas 深度仍靠 render  
3. Present 以整帧 `PresentWith` 为主；脏层 **计数/skip** 已有  
4. `presents` 计的是「已提交」，可能比 GPU 完成早约 1 帧  
5. Win/mac 真句柄未接  
6. 手势竞技 / 焦点 / Overlay 产品机制属 **P5**  

---

## 7. 文档职责（避免重复改）

| 文档 | 改什么时候 |
|------|------------|
| **本文** | L1 状态、验收、限制（**日常入口**） |
| `ENGINE_CODING_RULES` | **禁止 CGO / purego / 全局纪律** |
| `ENGINE_FLUTTER_SKIA_ARCH` | 架构条款变更 |
| `ENGINE_ARCH_OVERVIEW` | 图示/白话 |
| `ENGINE_PHASE_P0_P3` | 已完成任务考古 |
| `ENGINE_PHASE_P4` | P4 执行细卡（已实现） |
| `ENGINE_PHASE_P5` | **P5 执行细卡（L2 手势/焦点/Overlay）** |
| `ENGINE_PHASE_P4_P7_OUTLINE` | P4–P7 大纲 |

**L1 冻结：** 除非修 bug 或 P4 回头改契约，否则以本文状态为准，不再扩 L1 范围冒充完成。

---

## 8. 下一步

| 顺序 | 内容 |
|------|------|
| 1 | （可选）存 blank/spinner JSON baseline |
| 2 | P4 ✅ 已实现 |
| 3 | P5 L2 框架壳 ✅ — [`ENGINE_PHASE_P5.md`](./ENGINE_PHASE_P5.md) · `examples/ui_l2_shell` |
| 4 | **P7** Ant（后置，P5 A+C+D 已可用） |

---

## 9. 修订

| 版本 | 日期 | 说明 |
|------|------|------|
| 1.0 | 2026-07-27 | 首版收口 |
| 1.1 | 2026-07-27 | 文档收尾定稿：职责表、冻结说明、单测确认 |
| 1.2 | 2026-07-27 | 挂链 ENGINE_PHASE_P4 细卡 |
| 1.3 | 2026-07-27 | P4 实现完成：限制与下一步更新 |
| 1.4 | 2026-07-27 | 挂链 ENGINE_PHASE_P5 细卡 |
