# 05 · Transform / CTM / TransformLayer

> **状态（组级）：** A/B/C（TransformLayer + RO 已接） · **Wave：** P1 部分 ✅ · 修订：2026-07-28  
> 拆自 `ENGINE_UI_RENDER_BASE`；状态码见 [00_meta](./00_meta.md)  
> **施工真源分册** — 改状态先改本文件，再同步 [README](./README.md)

## 1. 目标 / 非目标

**目标：** 场景 `TransformLayer` + `RenderTransform` 旋转/缩放；paint-only 更新。  
**非目标：** Matrix4/透视；逆 CTM 命中；P6 层纹理复用。  
**已落地（2026-07-28）：**  
- `ui/scene.TransformLayer` · `PushTransform` · `MutSetTransform`（compositor-only 分类）  
- `ui/rendering.RenderTransform`（SetRotation/SetScale → MarkNeedsPaint）  
- `BuildLayerTree` 识别 Transform  
- 单测 + `ui_render_base_cliplayer` 旋转面板

## 2. 依赖（交叉关联）

| 方向 | 分册 / 说明 |
|------|-------------|
| **前置** | [01_pipeline_dirty](./01_pipeline_dirty.md) · [02_paint_context](./02_paint_context.md) · [08_scene_picture](./08_scene_picture.md) |
| **同级可并行** | 见 [91_wave_order](./91_wave_order.md) |
| **解锁** | 动画翻牌/图标缩放 · ClipLayer/PerfSoak 轴 |

## 3. 能力表（母表全文）

## §6 Transform / CTM / 变换层母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 | Wave |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|------|
| FX-OFFSET-LAYER | 位移层 | OffsetLayer | 滚动、定位 | — | Translate | OffsetLayer **A** | A | ui/scene/layer.go | — | — |
| FX-TRANSFORM-LAYER | 变换层 | TransformLayer | 旋转/缩放 | RenderTransform | CTM B | **TransformLayer A/C** | A/C | scene · transform.go | 命中 AABB；非 P6 | P1 |
| FX-OPACITY-LAYER | 透明度层 | OpacityLayer | 淡入淡出 | AnimatedOpacity 分类 | 层 opacity | OpacityLayer **A** | A/C | animation/implicit.go | 分类 compositor；Present 仍全画 | P1 |
| FX-CTM-FULL | 完整 CTM 栈 | Canvas transform 族 | 自定义绘制 | WithOrigin 仅平移语义 | 完整 CTM B | — | B | render | — | P0 |

---



## 4. 代码落点

| 层 | 路径 |
|----|------|
| scene | `layer.go` TransformLayer · `build.go` PushTransform · `packet.go` MutSetTransform · `compositing.go` |
| RO | `ui/rendering/transform.go` |
| layer_build | `RenderTransform` → PushBoundary? + PushTransform |
| render CTM | Translate / Rotate / Scale / Transform |
| 窗测 | `examples/ui_render_base_cliplayer` · `ui_render_base_perfsoak` |
| 测 | `transform_test.go` · `TestTransformLayer_Builder` · `TestBuildLayerTree_TransformLayer` |

## 5. 指标挂钩

| 指标 | 通过方向 |
|------|----------|
| M-LAYOUT-COUNT | SetRotation/SetScale **不得** MarkNeedsLayout |
| M-RASTER-LAYER | boundary 下局部 |
| M-FPS / hitch | cliplayer/soak |

完整定义 → [90_metrics](./90_metrics.md)。**无 baseline 的优化无效**（[00_meta](./00_meta.md)）。

## 6. 验收 DoD

- [x] scene TransformLayer + builder  
- [x] RenderTransform paint-only  
- [x] BuildLayerTree 含 transform  
- [x] cliplayer 旋转  
- [ ] 逆 CTM HitTest  
- [ ] Opacity+Transform 真接到 Present 合成（仍全画）

## 7. 风险与非宣称

HitTest 仍为 **AABB**。Present 全量时「子树不重绘」的 compositor 收益有限 — 勿宣称 P6。

## 8. 相关分册

- [总集 README](./README.md) · [实施顺序 91](./91_wave_order.md) · [指标 90](./90_metrics.md) · [F 交叉 92](./92_f_crosswalk.md)
