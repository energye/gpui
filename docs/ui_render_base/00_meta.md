# 00 · 元信息 · 读法 · 非宣称

> **全库状态码 / 非宣称 / 指标并行 / 依赖纪律真源** · 修订：2026-07-28

## §0 元信息 · 读法 · 非宣称

### 0.1 状态码（每行必填其一）

| 码 | 含义 |
|----|------|
| **A** | **UI 场景路径已可用**：经 RO/`PaintContext`/scene 已接，可测 |
| **B** | **仅 `render/` 具备**，UI 未暴露给控件/场景 paint |
| **C** | **半成品**：接口/字段/预算有，语义不全、未接线、或仅统计无 GPU 证明 |
| **D** | **相对 Flutter 缺失**（母表有、gpui 无；或明确 P6/P7/远期） |

### 0.2 允许 / 禁止宣称

```text
允许：L1 retained-paint 契约（Boundary/CompositeOnly/DirtyLayerIDs）；render Skia 式 2D 能力丰富；
      帧间隔/hitch/管道可观测；虚拟列表 bind 上界等已测门禁。

禁止：已完成 dirty-RECT 局部 Present；Picture 显示列表与 Flutter 对等；
      锁显示 60Hz（无真 VSync 时）；无 baseline 的「最优/更顺」。
```

### 0.3 列定义

每能力表统一 11 列。**Wave：** `—` 已 A；`P0`/`P1`/`P2`/`P3` 基座补齐；`P6` Present/层缓存；`P7` 控件；`远期` 进阶。

### 0.4 依赖纪律

```text
ui → render → gpu    禁止 ui→gpu    禁止 CGO（purego）
架构在 ui/ · 示例在 examples/    见 ENGINE_CODING_RULES
```

### 0.5 正向指标与能力补齐 **并行**（硬）

> 对应决策：**不要等「全部能力实现完」才处理正向优化指标。**

| 规则 | 说明 |
|------|------|
| **并行** | 能力 Wave（P0→P3）与帧时/hitch/layout/脏层/CPU/RSS 等指标 **同时推进** |
| **每波必带** | 合入一波能力或窗测轴时：stdout/JSON 可观测；相对同场景 baseline **不无故回退**（或可解释权衡） |
| **禁止后置** | **禁止** 等母表行全部变为 A 后，才开始采 60fps / CPU / 内存 / hitch |
| **P0 已起点** | 指标接线、p50/p99、RSS/CPU 采样、Geometry 轴门禁 **已在 P0 启动**（见 §23） |
| **PerfSoak 贯穿** | §24 `PerfSoak` 轴贯穿 P0–P3，不是「能力全做完后的最后一章」 |
| **P3 体系化** | 长 soak、多场景 baseline 入库、门禁矩阵收紧 — 在并行采集 **之上** 加深，不是第一次碰指标 |
| **P6 非起点** | dirty-rect Present / 层 RT 是 Present **极限**优化；**不是** 正向指标工作的起点 |
| **无 baseline 无效** | 没有同机器、同场景 JSON 对比的「优化」，不算正向优化（与大纲 §L1 一致） |

```text
能力 Wave（P1→P3）  ──并行──  每波指标不回退（§20 / §21）
        │
        ▼
   基座能力「可宣称够用」+ 测量体系已在跑
        │
        ▼
   P3 PerfSoak 体系化（长 soak / 基线库）
        │
        ▼
   P6 局部 Present / 层缓存（有瓶颈数据再拆）
```

交叉：[`ENGINE_PHASE_P4_P7_OUTLINE.md`](./ENGINE_PHASE_P4_P7_OUTLINE.md) §L1 验收（baseline 流程、持续正向优化）。

---


## §1 Flutter ↔ gpui 架构对照

| Flutter 概念 | gpui 路径 | 状态摘要 |
|--------------|-----------|----------|
| `dart:ui.Canvas` | `render.Context` + `PaintContext.DC` | render 富 / ui 薄 |
| `dart:ui.Paint` | `SetRGBA` / Stroke / Brush / Blend… | 多在 render（B） |
| `PaintingContext` | `ui/rendering.PaintContext` | **C** 子集 |
| `RenderObject` | `ui/rendering` | **A** 子集 |
| `Layer` 树 | `ui/scene` | Offset/Opacity/ClipRect/Boundary/Transform **A/C**；Picture **C**；Filter/Texture… **D** |
| `PipelineOwner` | `rendering.PipelineOwner` | **A** |
| `SchedulerBinding` / Vsync | `scheduler` + `VSyncWaiter` | 接口 **A**；真 VSync 多 **C** |
| Raster 线程 | `ui/raster.Loop` + `SubmitLatest` | **A**（F08） |
| Skia/Impeller present damage | `PresentFrameDamage*` | render **B**；PipelineApp **force 全清** → UI **D/C** |
| DevTools 性能 | `FrameMetrics` + 示例 stderr | 部分 **A**；CPU/RSS **D** |

**一帧（现状诚实）：**

```text
ScheduleFrame → layout(脏) → paint(CompositeOnly 可跳)
  → FramePacket(DirtyLayerIDs) → RasterizeDirty(统计)
  → SubmitLatest → PresentWith：全清 + force 全量 paint   ← 非 P6 damage
```

---


## 分册模板（每项必含）

1. 身份（状态/Wave）  
2. 目标/非目标（含**已落地**）  
3. 依赖表（前置/解锁）  
4. **能力母表全文**（本拆分不删 D 行）  
5. 代码落点  
6. 指标挂钩  
7. DoD  
8. 风险与非宣称  

## 相关

- [总集](./README.md) · [91 顺序](./91_wave_order.md) · [90 指标](./90_metrics.md)
