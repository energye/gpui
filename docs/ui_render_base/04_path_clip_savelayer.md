# 04 · Path 构建 · Clip · saveLayer

> **状态（组级）：** A/B/C（ClipRRect UI+层 ✅） · **Wave：** P0–P2 · 修订：2026-07-28  
> 拆自 `ENGINE_UI_RENDER_BASE`；状态码见 [00_meta](./00_meta.md)  
> **施工真源分册** — 改状态先改本文件，再同步 [README](./README.md)

## 1. 目标 / 非目标

**目标：** Path 原语、裁剪栈、saveLayer 预算；Clip 窗测可见。  
**非目标：** Path metrics 动画（P2）；全画布 saveLayer 滥用。  
**已落地：** render Path/ClipOp；`SaveLayerBudget`；cliplayer **ClipRect 溢出滚动** 面板；**`PaintContext.PushClipRRect` / `PopClip`**；**`scene.ClipRRectLayer` + `PushClipRRect`**；geometry 窗测走库 API。  
**组内顺序：** Path 原语 → Clip rect/op → nest depth → saveLayer 预算 → metrics/conic。

## 2. 依赖（交叉关联）

| 方向 | 分册 / 说明 |
|------|-------------|
| **前置** | [03_draw_geometry](./03_draw_geometry.md) · [02_paint_context](./02_paint_context.md) |
| **同级可并行** | 见 [91_wave_order](./91_wave_order.md) |
| **解锁** | 异形遮罩 · [12_filter_shadow](./12_filter_shadow.md) · [08](./08_scene_picture.md) Clip*Layer |

## 3. 能力表（母表全文）

## §4 `dart:ui.Path` 几何构建母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 | Wave |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|------|
| FPath-MOVE | MoveTo | Path.moveTo | 子路径起点 | — | Path.MoveTo / PathBuilder | — | B | render/path.go | — | P0 |
| FPath-LINE | LineTo | Path.lineTo | 折线 | — | LineTo | — | B | path.go | — | P0 |
| FPath-QUAD | 二次贝塞尔 | Path.quadraticBezierTo | 曲线 | — | QuadraticTo/QuadTo | — | B | path.go | — | P0 |
| FPath-CUBIC | 三次贝塞尔 | Path.cubicTo | 平滑曲线 | — | CubicTo | — | B | path.go | — | P0 |
| FPath-CONIC | 圆锥曲线 | Path.conicTo | 精确圆弧权重 | — | — | — | D | — | Flutter 有 conic | P2 |
| FPath-ARC-TO | 弧加入路径 | Path.arcTo/arcToPoint | 圆角路径 | — | Path.Arc / DrawArc | — | B | path · context | API 形态不完全同 | P1 |
| FPath-ADD-RECT | addRect | Path.addRect | — | — | Rectangle() | — | B | path.go | — | P0 |
| FPath-ADD-RRECT | addRRect | Path.addRRect | — | — | RoundedRectangle | — | B | path.go | — | P0 |
| FPath-ADD-OVAL | addOval | Path.addOval | — | — | Ellipse/Circle | — | B | path.go | — | P0 |
| FPath-ADD-PATH | addPath | Path.addPath | 组合图标 | — | Append | — | B | path.go | — | P1 |
| FPath-CLOSE | close | Path.close | 闭合填充 | — | Close | — | B | path.go | — | P0 |
| FPath-FILLTYPE | fillType | Path.fillType | evenOdd 镂空 | — | SetFillRule | — | B | render | UI 未暴露 | P1 |
| FPath-TRANSFORM | 路径变换 | Path.transform | 图标旋转缩放 | — | 矩阵作用于 path/CTM | — | B | — | — | P1 |
| FPath-METRICS | 路径度量 | Path.computeMetrics | 虚线动画/沿路径字 | — | — | — | D | — | 重要动效缺口 | P2 |
| FPath-COMBINE | 路径布尔 | Path.combine/op | 镂空合并 | — | Path.Op 布尔 | — | B | path_boolean.go | — | P2 |
| FPath-BOUNDS | 包围盒 | Path.getBounds | layout/damage | — | Bounds() | — | B | path.go | — | P1 |
| FPath-RESET | reset/clear | Path.reset | 复用 path | — | Clear/Reset | — | B | path.go | — | P0 |

---
## §5 Clip / save / saveLayer（补充母表）

> Canvas 级 clip/save 见 §2。本表强调 **UI 场景裁剪策略** 与 **ClipOp**。

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 | Wave |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|------|
| FClip-SAVE-PAIR | save/restore 裁剪栈 | 与 Canvas save | 嵌套裁剪 | PushClipRect/**PushClipRRect**/PopClip | Push/Pop+Clip* | ClipRect/ClipRRect Layer | A/B | paint_context.go · layer.go | rrect UI 已接 | — |
| FClip-OP-INTERSECT | ClipOp.intersect | ClipOp | 默认相交 | — | ClipOpIntersect | — | B | clip_op.go | — | P1 |
| FClip-OP-DIFFERENCE | ClipOp.difference | ClipOp | 挖洞 | — | ClipOpDifference | — | B | clip_op.go | — | P1 |
| FClip-NEST-DEPTH | 深层嵌套 clip | 实现限制 | 复杂卡片 | 有限 | clip 栈+测试 | — | C | context_clip_depth_test.go | 需文档化上限 | P1 |

---



## 4. 代码落点

| 层 | 路径 |
|----|------|
| Path | `render/path.go` · `path_boolean.go` |
| Clip | `render/context_clip*.go` · `paint_context.PushClipRect` / **`PushClipRRect`** / `PopClip` |
| saveLayer | render `PushLayer`/`PopLayer` · `SaveLayerBudget` |
| 窗测 | `examples/ui_render_base_cliplayer` clip 面板 · `ui_render_base_geometry` rrect |
| scene | `ClipRectLayer` · **`ClipRRectLayer`** · `PushClipRect` / **`PushClipRRect`** |
| 测 | `ui/rendering/clip_rrect_test.go` · `ui/scene` `TestClipRRectLayer_Builder` |

## 5. 指标挂钩

| 指标 | 说明 |
|------|------|
| PaintVisits | clip 嵌套 walk |
| M-LAYER-POOL | saveLayer 池（上浮后） |
| M-LAYOUT-COUNT | clip 滚动 paint-only |

完整定义 → [90_metrics](./90_metrics.md)。**无 baseline 的优化无效**（[00_meta](./00_meta.md)）。

## 6. 验收 DoD

- [x] Path/clip render 单测存在  
- [x] cliplayer clip 面板  
- [x] ClipRRect **UI/层** 友好封装（`PushClipRRect` + `ClipRRectLayer`；测绿）  
- [ ] Path.computeMetrics（D/P2）  
- [ ] 文档化 clip 嵌套上限

## 7. 风险与非宣称

F16：全幅 saveLayer 默认应拦。Clip 设备界与 P6 damage 联动见 [08](./08_scene_picture.md)。

## 8. 相关分册

- [总集 README](./README.md) · [实施顺序 91](./91_wave_order.md) · [指标 90](./90_metrics.md) · [F 交叉 92](./92_f_crosswalk.md)
