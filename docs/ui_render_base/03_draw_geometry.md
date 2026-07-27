# 03 · Canvas 几何绘制 + Paint/Shader 基础

> **状态（组级）：** A/B 混（主路径可用） · **Wave：** P0 ✅ / P1–P2 残余 · 修订：2026-07-28  
> 拆自 `ENGINE_UI_RENDER_BASE`；状态码见 [00_meta](./00_meta.md)  
> **施工真源分册** — 改状态先改本文件，再同步 [README](./README.md)

## 1. 目标 / 非目标

**目标：** 矩形/圆角/圆/线/路径/渐变等 UI 可画；Geometry 窗测可回归。  
**非目标：** Vertices/Atlas 产品化（P2）；P6 Picture 回放。  
**已落地：** P0/P1 经 `pc.DC`；`examples/ui_render_base_geometry`；径向/扫掠/path/oval/arc 示例行。  
**组内顺序：** Rect/RRect → Stroke → Circle/Oval → Path → Linear/Radial/Sweep → Arc。

## 2. 依赖（交叉关联）

| 方向 | 分册 / 说明 |
|------|-------------|
| **前置** | [02_paint_context](./02_paint_context.md) |
| **同级可并行** | 见 [91_wave_order](./91_wave_order.md) |
| **解锁** | CustomPaint · 控件底色 · [04](./04_path_clip_savelayer.md) |

## 3. 能力表（母表全文）

## §2 `dart:ui.Canvas` 全量能力母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 | Wave |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|------|
| FC-SAVE | 画布状态压栈 | Canvas.save | 嵌套 clip/transform | —（经 PushClip 间接） | Push | — | B | render/context.go | UI 无通用 Save | P0 |
| FC-RESTORE | 画布状态出栈 | Canvas.restore | 与 save 配对 | PopClip 仅 clip 对 | Pop | — | B | render/context.go | 通用 restore 未暴露 | P0 |
| FC-RESTORE-N | 恢复到指定深度 | Canvas.restoreToCount | 错误恢复/多层一次性弹出 | — | — | — | D | — | Flutter 有；gpui 无对等 API | P2 |
| FC-SAVECOUNT | 查询 save 深度 | Canvas.getSaveCount | 调试/断言 | — | clipStackDepth 内部 | — | C | context_clip*.go | 未作 UI 公开 API | P2 |
| FC-SAVELAYER | 离屏层+Paint | Canvas.saveLayer(bounds,paint) | 半透明组、滤镜、阴影组 | SaveLayerBudget 仅预算 | PushLayer/PopLayer | — | C+B | rendering.SaveLayerBudget · render PushLayer | 预算 C；真 saveLayer 在 render B | P1 |
| FC-SAVELAYER-NULL | 全画布 saveLayer | saveLayer(null,…) | 极重；默认应禁 | 预算 MaxArea 可拦 | PushLayer 全幅 | — | C | SaveLayerBudget | F16 纪律 | P1 |
| FC-TRANSLATE | 平移 CTM | Canvas.translate | 局部坐标系 | WithOrigin 绝对偏移 | Translate | OffsetLayer | A/B | PaintContext.WithOrigin · scene | UI 用 origin 模型非完整 CTM | P1 |
| FC-SCALE | 缩放 CTM | Canvas.scale | 缩放动画/适配 | RenderTransform.SetScale | Scale | TransformLayer | A/B | transform.go | RO 已接；任意 CTM 仍 B | P1 |
| FC-ROTATE | 旋转 CTM | Canvas.rotate | 旋转图标/翻牌 | RenderTransform.SetRotation | Rotate/RotateAbout | TransformLayer | A/B | transform.go | 命中 AABB | P1 |
| FC-SKEW | 错切 CTM | Canvas.skew | 少见 UI 效果 | — | Shear | — | B | render/context.go | — | P2 |
| FC-TRANSFORM | 任意 Matrix4 | Canvas.transform | 通用仿射 | — | Transform/SetTransform | — | B | render/context.go | UI 未暴露 | P1 |
| FC-GET-TRANSFORM | 读取 CTM | Canvas.getTransform | 命中反变换/调试 | — | GetTransform | — | B | render/context.go | — | P2 |
| FC-CLIP-RECT | 矩形裁剪 | Canvas.clipRect+ClipOp | 列表视口、溢出隐藏 | PushClipRect/PopClip | ClipRect/ClipRectOp | ClipRectLayer | A | paint_context.go / viewport | ClipOp 差异在 render | — |
| FC-CLIP-RRECT | 圆角裁剪 | Canvas.clipRRect | 卡片/头像裁切 | — | ClipRoundRect | 无 ClipRRectLayer | B/D | render/context_clip.go | 高频 UI 缺口 | P0 |
| FC-CLIP-PATH | 路径裁剪 | Canvas.clipPath | 异形遮罩 | — | Clip/ClipPathOp | 无 ClipPathLayer | B/D | render/context_clip.go | — | P1 |
| FC-CLIP-BOUNDS-LOCAL | 本地裁剪界 | getLocalClipBounds | 优化/命中 | — | 内部 clip 栈 | — | C | context_clip*.go | 未 UI 暴露 | P2 |
| FC-CLIP-BOUNDS-DEST | 设备裁剪界 | getDestinationClipBounds | damage 对齐 | — | — | — | D | — | 可与 P6 damage 联动 | P6 |
| FC-DRAW-COLOR | 整幅着色 | Canvas.drawColor | 清屏/遮罩 | 清色在 Present 路径 | Clear/ClearWithColor | — | B | render/context.go | PipelineApp 清屏 | — |
| FC-DRAW-PAINT | 用 Paint 填当前 clip | Canvas.drawPaint | 全 clip 填着色器 | — | SetFillBrush+大 rect Fill | — | B | render/context.go | 无 1:1 API | P1 |
| FC-DRAW-RECT | 矩形 | Canvas.drawRect | 色块/背景 | FillRect（fill） | DrawRectangle+Fill/Stroke | ColorBox | A/B | pc.DC · ColorBox/OnPaint | 描边 rect 仅 B | P0 |
| FC-DRAW-RRECT | 圆角矩形 | Canvas.drawRRect | 按钮/卡片 | FillRoundRect | DrawRoundedRectangle* | — | A/B | pc.DC · geometry 示例 | stroke rrect=B；XY 圆角=B | P0 |
| FC-DRAW-DRRECT | 双 RRect 环 | Canvas.drawDRRect | 环形进度/边框环 | — | 两 path 差或 stroke 近似 | — | D/B | — | 无 1:1 | P2 |
| FC-DRAW-OVAL | 椭圆 | Canvas.drawOval | 头像底/高亮 | — | DrawEllipse | — | B | render/context.go | — | P0 |
| FC-DRAW-CIRCLE | 圆 | Canvas.drawCircle | 头像/点/涟漪 | — | DrawCircle | Spinner 用 rect 点 | B | render/context.go | Spinner 可升级 | P0 |
| FC-DRAW-ARC | 弧/扇形 | Canvas.drawArc | 进度环 | — | DrawArc/DrawEllipticalArc | — | B | render/context.go | — | P1 |
| FC-DRAW-PATH | 任意路径 | Canvas.drawPath | 图标/波浪/自定义 | — | DrawPath/Fill/Stroke+Path | — | B | render/context.go · path.go | UI CustomPaint 急需 | P0 |
| FC-DRAW-LINE | 线段 | Canvas.drawLine | 分割线/刻度 | — | DrawLine | Divider 将来 | B | render/context.go | — | P0 |
| FC-DRAW-POINTS | 点列 | Canvas.drawPoints/RawPoints | sparklines | — | DrawPoint 等 | — | B | render/context.go | Raw 批量弱于 Flutter | P2 |
| FC-DRAW-VERTICES | 顶点网格 | Canvas.drawVertices | 扭曲图/网格渐变 | — | DrawVertices/DrawMesh | — | B | render/vertices.go | — | P2 |
| FC-DRAW-ATLAS | 图集精灵 | Canvas.drawAtlas | 图标合批、粒子 | — | DrawAtlas | — | B | render/vertices.go | — | P2 |
| FC-DRAW-SHADOW | 路径阴影 | Canvas.drawShadow | Material 海拔阴影 | — | ApplyDropShadow（全幅滤镜向） | — | B/C | render/filter_ops.go | 非 1:1 path shadow | P1 |
| FC-DRAW-IMAGE | 绘图像 | Canvas.drawImage | 图标/位图 | DrawImageBuf | DrawImage | RenderImage | A | paint_context.go / viewport | — | — |
| FC-DRAW-IMAGE-RECT | 源/目标矩形 | Canvas.drawImageRect | 裁剪缩放 | DrawImageBuf dst | DrawImageEx | RenderImage | A/B | context_image.go | 精细 src 矩形部分 B | P1 |
| FC-DRAW-IMAGE-NINE | 九宫格 | Canvas.drawImageNine | 可拉伸边框 | — | DrawImageNine | — | B | render/nine_patch.go | — | P1 |
| FC-DRAW-PARAGRAPH | 绘段落 | Canvas.drawParagraph | 所有正式文本 | DrawText/Colored 子集 | DrawString* | RenderText | C | ui/rendering/text.go | 非 Paragraph 模型 | P0 |
| FC-DRAW-PICTURE | 回放 Picture | Canvas.drawPicture | 层缓存回放 | — | 无 UI Picture 显示列表 | PictureLayer 仅 Valid | C/D | ui/scene/picture.go | P6 显示列表 | P6 |

---
## §3 `dart:ui.Paint` / Shader 母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 | Wave |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|------|
| FP-COLOR | 纯色 | Paint.color | 控件填色 | Fill* 参数 RGBA | SetRGBA/SetColor/SetHexColor | — | A/B | OnPaint RGBA + render | 无持久 Paint 对象 | P1 |
| FP-STYLE | fill/stroke | Paint.style | 描边按钮 | — | Fill vs Stroke 路径 | — | B | render | UI 无 Stroke* 封装 | P0 |
| FP-STROKE-W | 描边宽 | Paint.strokeWidth | 分割线粗细 | — | SetLineWidth | — | B | render | — | P0 |
| FP-STROKE-CAP | 线帽 | Paint.strokeCap | 圆角线端 | — | SetLineCap | — | B | render | — | P0 |
| FP-STROKE-JOIN | 线接 | Paint.strokeJoin | 折线拐角 | — | SetLineJoin | — | B | render | — | P0 |
| FP-STROKE-MITER | 斜接限制 | Paint.strokeMiterLimit | 尖角 | — | Miter 相关 | — | B | render/stroke | — | P1 |
| FP-AA | 抗锯齿 | Paint.isAntiAlias | 边缘质量 | — | SetAntiAlias | — | B | render/context.go | — | P1 |
| FP-SHADER | 着色器 | Paint.shader | 渐变/图着色 | 线性渐变 Fill | SetFillBrush 等 | — | A/B | gradient.go | 径向/扫掠/图=B | P0 |
| FP-BLEND | 混合模式 | Paint.blendMode | 叠加/遮罩 | — | SetBlendMode / PushLayer blend | — | B | context_layer.go | UI 未暴露 | P1 |
| FP-MASK-FILTER | 遮罩模糊 | Paint.maskFilter | 软阴影感 | — | ApplyBlur 等 | — | B | filter_ops.go | 非 Paint 绑定模型 | P2 |
| FP-COLOR-FILTER | 颜色滤镜 | Paint.colorFilter | 置灰禁用图标 | — | ApplyColorMatrix 等 | — | B | filter_ops.go | — | P2 |
| FP-IMAGE-FILTER | 图像滤镜 | Paint.imageFilter | 模糊背景 | — | ApplyImageFilterGraph/Blur | — | B | filter_ops.go | — | P2 |
| FP-FILTER-QUALITY | 采样质量 | Paint.filterQuality | 缩放图锐利度 | — | DrawImageEx 选项部分 | — | C | context_image.go | 对齐不完全 | P1 |
| FP-INVERT | 反色 | Paint.invertColors | 无障碍高对比 | — | ApplyInvert 类 | — | B/D | filter_ops.go | 核对实现 | P2 |
| FS-LINEAR | 线性渐变 | Gradient.linear | AppBar/按钮 | FillLinearGradient* | NewLinearGradientBrush | — | A | pc.DC 渐变 · geometry 示例 | — | — |
| FS-RADIAL | 径向渐变 | Gradient.radial | 光晕 | — | NewRadialGradientBrush | — | B | gradient_radial.go | — | P0 |
| FS-SWEEP | 扫掠渐变 | Gradient.sweep | 圆锥/色轮 | — | NewSweepGradientBrush | — | B | gradient_sweep.go | — | P1 |
| FS-IMAGE | 图像着色 | ImageShader | 图案填充 | — | CreateImagePattern/SetFillPattern | — | B | context_image.go | — | P1 |
| FS-FRAGMENT | 片元着色器 | FragmentShader | 自定义特效 | — | — | — | D | — | Impeller/SkSL 级 | 远期 |

---



## 4. 代码落点

| 层 | 路径 |
|----|------|
| render | `context.go` · gradient* · path · stroke |
| UI | OnPaint / ColorBox / geometry 示例 local helpers |
| 窗测 | `examples/ui_render_base_geometry` ✅ |
| 相关 | Transform 缩放旋转见 [05](./05_transform.md)（FC-SCALE/ROTATE 行已指向 RenderTransform） |

## 5. 指标挂钩

| 指标 | 场景 | 通过方向 |
|------|------|----------|
| M-FPS-WALL / p50/p99 | geometry Persistent | 动画 ≥55 量级 |
| M-LAYOUT-COUNT | 描边/色脉冲 | 不风暴 |
| M-HITCH | 短跑 | 低 |
| M-RSS-* | 短跑 | 观察 |

完整定义 → [90_metrics](./90_metrics.md)。**无 baseline 的优化无效**（[00_meta](./00_meta.md)）。

## 6. 验收 DoD

- [x] `RUN_SECONDS=10 go run ./examples/ui_render_base_geometry`  
- [x] layout 不因动画风暴  
- [ ] 补 UI 级 Stroke* 叙事若需要（render 已 B→可保持 B）  
- [ ] Vertices/Atlas 门禁（P2）  
- [ ] baseline JSON 入库（P3）

## 7. 风险与非宣称

证据列禁止再依赖已删 `ui/painting`。FC-DRAW-PICTURE / damage 属 [08](./08_scene_picture.md)/P6。

## 8. 相关分册

- [总集 README](./README.md) · [实施顺序 91](./91_wave_order.md) · [指标 90](./90_metrics.md) · [F 交叉 92](./92_f_crosswalk.md)
