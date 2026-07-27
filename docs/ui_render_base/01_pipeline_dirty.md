# 01 · RenderObject / Pipeline 脏区

> **状态（组级）：** A（核心）/ C（部分） · **Wave：** — / P1 · 修订：2026-07-28  
> 拆自 `ENGINE_UI_RENDER_BASE`；状态码见 [00_meta](./00_meta.md)  
> **施工真源分册** — 改状态先改本文件，再同步 [README](./README.md)

## 1. 目标 / 非目标

**目标：** markNeedsLayout/Paint、Relayout/RepaintBoundary、Constraints、PipelineOwner flush、CompositeOnly 契约可用。  
**非目标：** Widget/Element BuildOwner；完整 Semantics 树。  
**已落地：** Base dirty 位、Boundary、S2/S4/matrix 门禁、layout/paint 计数接线。

## 2. 依赖（交叉关联）

| 方向 | 分册 / 说明 |
|------|-------------|
| **前置** | [00_meta](./00_meta.md) |
| **同级可并行** | 见 [91_wave_order](./91_wave_order.md) |
| **解锁** | [02_paint_context](./02_paint_context.md) · 全部 RO |

## 3. 能力表（母表全文）

## §13 RenderObject / Pipeline 脏区母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 | Wave |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|------|
| FRO-MARK-LAYOUT | 标记需布局 | markNeedsLayout | 内容/约束变 | MarkNeedsLayout | — | Base | A | node.go | — | — |
| FRO-MARK-PAINT | 标记需绘制 | markNeedsPaint | 外观变 | MarkNeedsPaint | — | Base | A | node.go | — | — |
| FRO-MARK-COMP | 合成位更新 | markNeedsCompositingBitsUpdate | 层结构变 | compositing 传播 | — | scene compositing | A/C | compositing.go | 深度弱于 Flutter | P1 |
| FRO-RELAYOUT-B | RelayoutBoundary | relayoutBoundary | 截断 layout 冒泡 | SetRelayoutBoundary | — | Viewport 默认 | A | node.go · viewport.go | — | — |
| FRO-REPAINT-B | RepaintBoundary | isRepaintBoundary | 截断 paint 冒泡 | SetRepaintBoundary | — | BoundaryLayer | A | node.go | — | — |
| FRO-SIZED-BY-PARENT | 父决定尺寸 | sizedByParent | 紧约束子 | Fixed 模式部分 | — | Box/ColorBox | C | box.go | 无完整标志模型 | P1 |
| FRO-CONSTRAINTS | 约束 | BoxConstraints | layout 输入 | Constraints | — | pipeline | A | constraints.go | — | — |
| FRO-LAYOUT | performLayout | performLayout | 测布局 | Layout() | — | 各 RO | A | rendering/* | — | — |
| FRO-PAINT | paint | paint | 录制/绘制 | Paint(pc) | 经 DC | — | A | — | — | — |
| FRO-HIT | 命中测试 | hitTest* | 指针 | HitTest | — | 各 RO | A | — | — | — |
| FRO-LAYER-FIELD | 层句柄 | layer / updateCompositedLayer | 合成对象 | BuildLayerTree | — | layer_build.go | C | — | 非每节点持久 Layer 句柄 | P1 |
| FRO-ATTACH | attach/detach | attach/detach | 元素挂载 | AddChild 等 | — | Base | C | node.go | 生命周期弱于 Element 树 | P2 |
| FRO-SEMANTICS | 语义脏 | markNeedsSemanticsUpdate | 无障碍 | ui/semantics 骨架 | — | — | C | semantics | 非完整 | 远期 |
| FRO-PIPELINE | PipelineOwner | flushLayout/Paint/Compositing | 帧驱动 | PipelineOwner | — | embedder | A | pipeline.go | flushCompositing 弱 | P1 |
| FRO-BUILD-OWNER | BuildOwner | 构建脏元件 | Widget→Element | 无 Widget 层 | — | — | D | — | 命令式 RO 模型 | — |

---
## §18 gpui 现状 — RenderObject / scene

### 18.1 RenderObject

| 类型 | 角色 | 状态 |
|------|------|------|
| `Base` + dirty 位 | RO 核心 | A |
| `RenderBox` | 容器 | A |
| `RenderColorBox` | 色块 | A |
| `AbsoluteBox` | 绝对定位壳 | A |
| `RenderSpinner` | 动画测 | A |
| `RenderText` | 单行文本 MVP | C |
| `RenderImage` | 图+占位+异步 | A |
| `RenderViewport` | 裁剪+滚动 | A |
| `VirtualList` | 固定行高虚拟列表 | A |
| `Scrollable` | 指针/滚轮→滚动 | A |
| `PipelineOwner` | flush | A |

### 18.2 scene.Layer

| 类型 | 状态 |
|------|------|
| Container / Offset / Opacity / ClipRect / Picture / Boundary / **Transform** | A 或 C(Picture/Opacity 合成) |
| ClipRRect / ClipPath / Texture / Backdrop / Filter / Leader… | **D** |

### 18.3 Present 诚实条款

| 项 | 状态 |
|----|------|
| `render.PresentFrameDamage*` | B（render 有） |
| `PipelineApp` 每帧 force 全清+全 paint | **C 缺口**（注释写明 P6） |
| 真 dirty-rect Present | **D / P6** |

---



## 4. 代码落点

| 符号 | 路径 |
|------|------|
| Base / dirty | `ui/rendering/node.go` |
| PipelineOwner | `ui/rendering/pipeline.go` |
| Box/Absolute/Viewport | `box.go` · `absolute.go` · `viewport.go` |
| Layer build | `layer_build.go` |
| 测试 | `layer_test.go` · `render_matrix_test.go` · S2/S4/S5/S6 |
| 现状 RO 表 | 见本分册 §18.1（自母表） |

## 5. 指标挂钩

| 指标 ID | 场景 | 通过方向 |
|---------|------|----------|
| M-LAYOUT-COUNT | 动画/换色 | 不无故涨；S2≈0 额外 layout |
| M-PAINT-COUNT | flush | 可解释 |
| PaintVisits | CompositeOnly | ≪ 全树 |
| M-RASTER-LAYER | Spinner | 小 |
| M-BIND | VirtualList | ≪ itemCount |
| M-SKIP-LAYER | S4 静态+1 动画 | 静态 skip >0 |

完整定义 → [90_metrics](./90_metrics.md)。**无 baseline 的优化无效**（[00_meta](./00_meta.md)）。

## 6. 验收 DoD

- [x] Relayout/RepaintBoundary 单测  
- [x] CompositeOnly 跳过 clean boundary  
- [x] layout_count/paint_count 进 Metrics JSON  
- [ ] 加深 compositing bits / 每节点持久 Layer 句柄（P1 C 项）  
- [ ] 改 dirty 语义后重跑 matrix + spinner

## 7. 风险与非宣称

真窗 Present 仍 force 全清+全 paint 时，**不可**用 present 路径 PaintVisits 证明 **P6** dirty-rect。CompositeOnly 只证明 retained **契约**。

## 8. 相关分册

- [总集 README](./README.md) · [实施顺序 91](./91_wave_order.md) · [指标 90](./90_metrics.md) · [F 交叉 92](./92_f_crosswalk.md)
