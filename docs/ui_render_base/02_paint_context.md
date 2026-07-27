# 02 · PaintContext 绘制游标

> **状态（组级）：** A · **Wave：** — · 修订：2026-07-28  
> 拆自 `ENGINE_UI_RENDER_BASE`；状态码见 [00_meta](./00_meta.md)  
> **施工真源分册** — 改状态先改本文件，再同步 [README](./README.md)

## 1. 目标 / 非目标

**目标：** UI 控制「怎么画」（Origin / CompositeOnly / 顺序）；`render.Context` 执行。  
**非目标：** 恢复独立 `ui/painting` 门面包；第二套 2D 引擎。  
**已落地（2026-07-27）：** 删除 `ui/painting`；`PaintContext`∈`ui/rendering`；示例/RO 经 `pc.DC.*`。

## 2. 依赖（交叉关联）

| 方向 | 分册 / 说明 |
|------|-------------|
| **前置** | [01_pipeline_dirty](./01_pipeline_dirty.md) |
| **同级可并行** | 见 [91_wave_order](./91_wave_order.md) |
| **解锁** | [03_draw_geometry](./03_draw_geometry.md) · [06_image](./06_image.md) · [07_text](./07_text.md) · 一切 OnPaint |

## 3. 能力表（母表全文）

## §11 `PaintingContext` 母表（Flutter rendering）

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 | Wave |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|------|
| FPC-CANVAS | 取 Canvas | PaintingContext.canvas | 底层绘制 | DC *render.Context | Context | — | A | paint_context.go | — | — |
| FPC-PAINT-CHILD | 绘子节点 | paintChild | 树遍历 | 子 Paint+WithOrigin | — | Box/Absolute 遍历 | A | box.go | — | — |
| FPC-CLIP-RECT | pushClipRect | PaintingContext.pushClipRect | 视口 | PushClipRect | ClipRect | ClipRectLayer | A | paint_context.go · viewport | — | — |
| FPC-CLIP-RRECT | pushClipRRect | pushClipRRect | 圆角裁子树 | **PushClipRRect** | ClipRoundRect | **ClipRRectLayer** | A | paint_context.go · clip_rrect_test | 均匀圆角 | — |
| FPC-CLIP-PATH | pushClipPath | pushClipPath | 异形 | — | Clip path | — | B/D | — | — | P1 |
| FPC-COLOR-FILTER | pushColorFilter | pushColorFilter | 子树滤镜 | — | 滤镜 API | — | B/D | — | — | P2 |
| FPC-OPACITY | pushOpacity | pushOpacity | 子树透明 | — | Opacity 层/CTM | OpacityLayer | A/C | scene | Present 全画 | P1 |
| FPC-TRANSFORM | pushTransform | pushTransform | 子树变换 | RenderTransform | CTM | TransformLayer | A/C | transform.go · layer_build | 非通用 pushTransform API | P1 |
| FPC-LAYER | pushLayer/addLayer | pushLayer | 自定义层 | — | — | LayerBuilder 有限 | C | scene/build.go | — | P1 |
| FPC-COMPLEX-HINT | setIsComplexHint | setIsComplexHint | 光栅缓存提示 | — | — | — | D | — | RasterCache P6 | P6 |
| FPC-WILL-CHANGE | setWillChangeHint | setWillChangeHint | 动画提示 | — | — | — | D | — | P6 | P6 |
| FPC-COMPOSITE-ONLY | retained 跳过 | 实现细节 | 脏区局部 paint | CompositeOnly+PaintVisits | — | FlushPaint(false) | A | pipeline.go | 真窗 force 全画 | — |

---
## §17 gpui 现状快照 — 绘制入口（无独立 painting 包）

> **2026-07-27：** `ui/painting` **已删除**。

| 入口 | 路径 | 说明 |
|------|------|------|
| 绘制游标 | `ui/rendering.PaintContext` | `DC *render.Context`、Origin、CompositeOnly、PaintVisits |
| 真画布 | `render.Context` | Fill/Stroke/Path/Text/Gradient/Clip/Layer… |
| 树 Paint | `RenderObject.Paint(*PaintContext)` | UI 控制怎么画；render 执行 |
| 字体辅助 | `rendering.TryLoadDefaultFace` | 示例用 |
| 文本估宽 | `rendering.EstimateTextSize` | 无 face 时 |
| F16 预算 | `rendering.SaveLayerBudget` | 可选 |

**禁止：** `ui → gpu`。**允许：** `ui → render`。



## 4. 代码落点

| 符号 | 路径 |
|------|------|
| PaintContext | `ui/rendering/paint_context.go` |
| 真画布 | `render.Context`（`pc.DC`） |
| RO.Paint | `Paint(*PaintContext)` |
| 默认字体薄封装 | `TryLoadDefaultFace` → `text.LoadDefaultFace` |
| 估宽 / F16 | `EstimateTextSize` · `SaveLayerBudget` |
| **已删除** | `ui/painting/**` |

## 5. 指标挂钩

| 指标 | 说明 |
|------|------|
| PaintVisits | 测内 CompositeOnly walk |
| M-PAINT-COUNT | Pipeline flush 次数 |

完整定义 → [90_metrics](./90_metrics.md)。**无 baseline 的优化无效**（[00_meta](./00_meta.md)）。

## 6. 验收 DoD

- [x] 无 `ui/painting` 包  
- [x] `go test ./ui/rendering`  
- [x] depcheck：ui ↛ gpu  
- [x] geometry/text/image/cliplayer 仅 `pc.DC`  
- [ ] 通用 Save/Restore UI 封装（仍 B，见 03/04）

## 7. 风险与非宣称

母表历史行若仍写 `painting/*`，**一律以本分册代码落点为准**。PaintContext 不是 Skia 替代实现。

## 8. 相关分册

- [总集 README](./README.md) · [实施顺序 91](./91_wave_order.md) · [指标 90](./90_metrics.md) · [F 交叉 92](./92_f_crosswalk.md)
