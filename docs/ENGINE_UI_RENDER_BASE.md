# UI 渲染基座能力总表 — Flutter 母表 × gpui 映射

> **版本：1.0** | 日期：2026-07-27  
> **性质：能力真源（盘点）** — 以 **Flutter UI 渲染基座** 为母表；gpui **不支持也必须保留行**  
> **范围：** Canvas / Paint / Path / Text / Picture / PaintingContext / Layer / RenderObject / Scheduler / 滚动协议 / **正向性能指标**  
> **非范围：** P6 dirty-rect Present 冒充完成、Ant 控件皮肤（P7）、完整 a11y 生态  
> **交叉：** [`ENGINE_FLUTTER_SKIA_ARCH.md`](./ENGINE_FLUTTER_SKIA_ARCH.md) · [`ENGINE_CODING_RULES.md`](./ENGINE_CODING_RULES.md) · [`ENGINE_L1_CLOSEOUT.md`](./ENGINE_L1_CLOSEOUT.md) · [`ENGINE_PHASE_P4_P7_OUTLINE.md`](./ENGINE_PHASE_P4_P7_OUTLINE.md)

---

## §0 元信息 · 读法 · 非宣称

### 0.1 状态码（每行必填其一）

| 码 | 含义 |
|----|------|
| **A** | **UI 场景路径已可用**：经 `ui/painting` 或 RO/scene 已接，可测 |
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

---

## §1 Flutter ↔ gpui 架构对照

| Flutter 概念 | gpui 路径 | 状态摘要 |
|--------------|-----------|----------|
| `dart:ui.Canvas` | `render.Context` + 薄 `ui/painting.Context` | render 富 / ui 薄 |
| `dart:ui.Paint` | `SetRGBA` / Stroke / Brush / Blend… | 多在 render（B） |
| `PaintingContext` | `ui/painting.Context` | **C** 子集 |
| `RenderObject` | `ui/rendering` | **A** 子集 |
| `Layer` 树 | `ui/scene` | Offset/Opacity/ClipRect/Picture/Boundary **A**；Transform 等 **D** |
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
## §2 `dart:ui.Canvas` 全量能力母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 | Wave |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|------|
| FC-SAVE | 画布状态压栈 | Canvas.save | 嵌套 clip/transform | —（经 PushClip 间接） | Push | — | B | render/context.go | UI 无通用 Save | P0 |
| FC-RESTORE | 画布状态出栈 | Canvas.restore | 与 save 配对 | PopClip 仅 clip 对 | Pop | — | B | render/context.go | 通用 restore 未暴露 | P0 |
| FC-RESTORE-N | 恢复到指定深度 | Canvas.restoreToCount | 错误恢复/多层一次性弹出 | — | — | — | D | — | Flutter 有；gpui 无对等 API | P2 |
| FC-SAVECOUNT | 查询 save 深度 | Canvas.getSaveCount | 调试/断言 | — | clipStackDepth 内部 | — | C | context_clip*.go | 未作 UI 公开 API | P2 |
| FC-SAVELAYER | 离屏层+Paint | Canvas.saveLayer(bounds,paint) | 半透明组、滤镜、阴影组 | SaveLayerBudget 仅预算 | PushLayer/PopLayer | — | C+B | ui/painting · context_layer.go | 预算 C；真 saveLayer 在 render B | P1 |
| FC-SAVELAYER-NULL | 全画布 saveLayer | saveLayer(null,…) | 极重；默认应禁 | 预算 MaxArea 可拦 | PushLayer 全幅 | — | C | SaveLayerBudget | F16 纪律 | P1 |
| FC-TRANSLATE | 平移 CTM | Canvas.translate | 局部坐标系 | WithOrigin 绝对偏移 | Translate | OffsetLayer | A/B | painting.Context · scene | UI 用 origin 模型非完整 CTM | P1 |
| FC-SCALE | 缩放 CTM | Canvas.scale | 缩放动画/适配 | — | Scale | 无 TransformLayer | B/D | render/context.go | scene 无 TransformLayer=D | P1 |
| FC-ROTATE | 旋转 CTM | Canvas.rotate | 旋转图标/翻牌 | — | Rotate/RotateAbout | 无 TransformLayer | B/D | render/context.go | 需 TransformLayer | P1 |
| FC-SKEW | 错切 CTM | Canvas.skew | 少见 UI 效果 | — | Shear | — | B | render/context.go | — | P2 |
| FC-TRANSFORM | 任意 Matrix4 | Canvas.transform | 通用仿射 | — | Transform/SetTransform | — | B | render/context.go | UI 未暴露 | P1 |
| FC-GET-TRANSFORM | 读取 CTM | Canvas.getTransform | 命中反变换/调试 | — | GetTransform | — | B | render/context.go | — | P2 |
| FC-CLIP-RECT | 矩形裁剪 | Canvas.clipRect+ClipOp | 列表视口、溢出隐藏 | PushClipRect/PopClip | ClipRect/ClipRectOp | ClipRectLayer | A | ui/painting/clip_text.go | ClipOp 差异在 render | — |
| FC-CLIP-RRECT | 圆角裁剪 | Canvas.clipRRect | 卡片/头像裁切 | — | ClipRoundRect | 无 ClipRRectLayer | B/D | render/context_clip.go | 高频 UI 缺口 | P0 |
| FC-CLIP-PATH | 路径裁剪 | Canvas.clipPath | 异形遮罩 | — | Clip/ClipPathOp | 无 ClipPathLayer | B/D | render/context_clip.go | — | P1 |
| FC-CLIP-BOUNDS-LOCAL | 本地裁剪界 | getLocalClipBounds | 优化/命中 | — | 内部 clip 栈 | — | C | context_clip*.go | 未 UI 暴露 | P2 |
| FC-CLIP-BOUNDS-DEST | 设备裁剪界 | getDestinationClipBounds | damage 对齐 | — | — | — | D | — | 可与 P6 damage 联动 | P6 |
| FC-DRAW-COLOR | 整幅着色 | Canvas.drawColor | 清屏/遮罩 | 清色在 Present 路径 | Clear/ClearWithColor | — | B | render/context.go | PipelineApp 清屏 | — |
| FC-DRAW-PAINT | 用 Paint 填当前 clip | Canvas.drawPaint | 全 clip 填着色器 | — | SetFillBrush+大 rect Fill | — | B | render/context.go | 无 1:1 API | P1 |
| FC-DRAW-RECT | 矩形 | Canvas.drawRect | 色块/背景 | FillRect（fill） | DrawRectangle+Fill/Stroke | ColorBox | A/B | ui/painting/context.go | 描边 rect 仅 B | P0 |
| FC-DRAW-RRECT | 圆角矩形 | Canvas.drawRRect | 按钮/卡片 | FillRoundRect | DrawRoundedRectangle* | — | A/B | ui/painting/shapes.go | stroke rrect=B；XY 圆角=B | P0 |
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
| FC-DRAW-IMAGE | 绘图像 | Canvas.drawImage | 图标/位图 | DrawImageBuf | DrawImage | RenderImage | A | ui/painting/clip_text.go | — | — |
| FC-DRAW-IMAGE-RECT | 源/目标矩形 | Canvas.drawImageRect | 裁剪缩放 | DrawImageBuf dst | DrawImageEx | RenderImage | A/B | context_image.go | 精细 src 矩形部分 B | P1 |
| FC-DRAW-IMAGE-NINE | 九宫格 | Canvas.drawImageNine | 可拉伸边框 | — | DrawImageNine | — | B | render/nine_patch.go | — | P1 |
| FC-DRAW-PARAGRAPH | 绘段落 | Canvas.drawParagraph | 所有正式文本 | DrawText/Colored 子集 | DrawString* | RenderText | C | ui/rendering/text.go | 非 Paragraph 模型 | P0 |
| FC-DRAW-PICTURE | 回放 Picture | Canvas.drawPicture | 层缓存回放 | — | 无 UI Picture 显示列表 | PictureLayer 仅 Valid | C/D | ui/scene/picture.go | P6 显示列表 | P6 |

---

## §3 `dart:ui.Paint` / Shader 母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 | Wave |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|------|
| FP-COLOR | 纯色 | Paint.color | 控件填色 | Fill* 参数 RGBA | SetRGBA/SetColor/SetHexColor | — | A/B | painting + render | 无持久 Paint 对象 | P1 |
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
| FS-LINEAR | 线性渐变 | Gradient.linear | AppBar/按钮 | FillLinearGradient* | NewLinearGradientBrush | — | A | ui/painting/gradient.go | — | — |
| FS-RADIAL | 径向渐变 | Gradient.radial | 光晕 | — | NewRadialGradientBrush | — | B | gradient_radial.go | — | P0 |
| FS-SWEEP | 扫掠渐变 | Gradient.sweep | 圆锥/色轮 | — | NewSweepGradientBrush | — | B | gradient_sweep.go | — | P1 |
| FS-IMAGE | 图像着色 | ImageShader | 图案填充 | — | CreateImagePattern/SetFillPattern | — | B | context_image.go | — | P1 |
| FS-FRAGMENT | 片元着色器 | FragmentShader | 自定义特效 | — | — | — | D | — | Impeller/SkSL 级 | 远期 |

---

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
| FClip-SAVE-PAIR | save/restore 裁剪栈 | 与 Canvas save | 嵌套裁剪 | PushClipRect/PopClip | Push/Pop+Clip* | — | A/B | clip_text.go | 仅 rect 友好封装 | P0 |
| FClip-OP-INTERSECT | ClipOp.intersect | ClipOp | 默认相交 | — | ClipOpIntersect | — | B | clip_op.go | — | P1 |
| FClip-OP-DIFFERENCE | ClipOp.difference | ClipOp | 挖洞 | — | ClipOpDifference | — | B | clip_op.go | — | P1 |
| FClip-NEST-DEPTH | 深层嵌套 clip | 实现限制 | 复杂卡片 | 有限 | clip 栈+测试 | — | C | context_clip_depth_test.go | 需文档化上限 | P1 |

---

## §6 Transform / CTM / 变换层母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 | Wave |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|------|
| FX-OFFSET-LAYER | 位移层 | OffsetLayer | 滚动、定位 | — | Translate | OffsetLayer **A** | A | ui/scene/layer.go | — | — |
| FX-TRANSFORM-LAYER | 变换层 | TransformLayer | 旋转/缩放不重绘子树 | — | CTM B | **无 TransformLayer** | D | — | 场景树关键缺口 | P1 |
| FX-OPACITY-LAYER | 透明度层 | OpacityLayer | 淡入淡出 | AnimatedOpacity 分类 | 层 opacity | OpacityLayer **A** | A/C | animation/implicit.go | 分类 compositor；Present 仍全画 | P1 |
| FX-CTM-FULL | 完整 CTM 栈 | Canvas transform 族 | 自定义绘制 | WithOrigin 仅平移语义 | 完整 CTM B | — | B | render | — | P0 |

---

## §7 图像 / Atlas / 纹理母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 | Wave |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|------|
| FImg-DRAW | 绘 Image | drawImage | 位图 | DrawImageBuf | DrawImage | RenderImage | A | image.go | — | — |
| FImg-RECT | 源矩形采样 | drawImageRect | 精灵/裁剪 | 部分 via Ex | DrawImageEx | — | A/B | context_image.go | — | P1 |
| FImg-NINE | 九宫格 | drawImageNine | 气泡边框 | — | DrawImageNine | — | B | nine_patch.go | — | P1 |
| FImg-ROUND | 圆角图 | ClipRRect+image | 头像 | — | DrawImageRounded | — | B | context_image.go | — | P0 |
| FImg-CIRCLE | 圆形图 | DecorationImage | 头像 | — | DrawImageCircular | — | B | context_image.go | — | P0 |
| FImg-QUAD | 四角映射 | 自定义 | 透视广告 | — | DrawImageQuad | — | B | m4_extensions.go | — | P2 |
| FImg-CODEC | 解码 | instantiateImageCodec | 加载图 | ui/io.Pool DecodeFile | ImageBuf | RenderImage 状态机 | A | ui/io/decode.go | F12；多帧 GIF 弱 | P1 |
| FImg-DISPOSE | 释放图像 | image.dispose | 防泄漏 | — | ImageBuf 生命周期 | — | C | — | 需规范 Release | P1 |
| FImg-TO-BYTES | 读回像素 | toByteData | 截图/测试 | — | ExportImageBuf/Image/SavePNG | — | B | context_image.go | — | P2 |
| FImg-GPU-TEX | 外部纹理 | Texture | 视频帧 | — | DrawGPUTexture* | 无 TextureLayer | B/D | context_image.go | FL-TEXTURE 关联 | P2 |
| FImg-ATLAS | 图集 | drawAtlas | 合批图标 | — | DrawAtlas | — | B | vertices.go | — | P2 |

---

## §8 文本 / Paragraph / TextPainter 母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 | Wave |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|------|
| FT-PARAGRAPH-BUILD | 构建段落 | ParagraphBuilder | 富文本 | — | 无 ParagraphBuilder | RenderText 单串 | D | — | 富文本基座缺口 | P1 |
| FT-PARAGRAPH-LAYOUT | 段落布局 | Paragraph.layout | 换行高度 | 近似宽高 | MeasureString/Multiline | RenderText.Layout 启发式 | C/B | text.go · render/text.go | 接通 Measure 到 RO=P0 | P0 |
| FT-PARAGRAPH-PAINT | 绘段落 | drawParagraph | 正文 | DrawTextColored | DrawString* | RenderText.Paint | C | rendering/text.go | — | P0 |
| FT-PAINTER | TextPainter | TextPainter | 通用测量绘 | — | Measure+Draw 组合 | — | B | render/text.go | 可做 ui 门面 | P0 |
| FT-STYLE-SIZE | 字号 | TextStyle.fontSize | 层级 | FontSize 字段 | LoadFontFace points | RenderText | C | text.go | 未可靠设到 DC 字体 | P0 |
| FT-STYLE-FAMILY | 字体族 | fontFamily | 品牌/CJK | — | SetFont/LoadFontFace | — | B | render/text.go | RO 未接通 | P0 |
| FT-STYLE-WEIGHT | 字重 | fontWeight | 强调 | — | Face/variations 部分 | — | B | LoadFontFaceWithVariations | — | P1 |
| FT-STYLE-STYLE | italic | fontStyle | 斜体 | — | — | — | C/D | — | 依赖字体文件 | P1 |
| FT-LETTER-SPACING | 字距 | letterSpacing | 标题微调 | — | — | — | D | — | — | P2 |
| FT-WORD-SPACING | 词距 | wordSpacing | 英文 | — | — | — | D | — | — | P2 |
| FT-HEIGHT | 行高 | height | 多行节奏 | *1.25 近似 | lineSpacing 参数 | — | C | DrawStringWrapped | — | P1 |
| FT-STRUT | StrutStyle | StrutStyle | 多行稳定行盒 | — | — | — | D | — | — | P2 |
| FT-ALIGN | 对齐 | TextAlign | 标题居中 | — | Align @ Wrapped | — | B | DrawStringWrapped | 单行 RO 无 | P1 |
| FT-DIRECTION | 文本方向 | TextDirection | RTL | — | BiDi shaping 路径 | — | B/C | render/text | 未 UI 文档化 | P1 |
| FT-MAXLINES | 最大行数 | maxLines | 列表副标题 | — | — | — | D | — | — | P1 |
| FT-OVERFLOW | 溢出省略 | TextOverflow.ellipsis | 长文案 | — | — | — | D | — | 高频列表需求 | P1 |
| FT-LOCALE | locale | locale | 断行/字体 | — | — | — | D | — | — | P2 |
| FT-DECORATION | 下划线等 | TextDecoration | 链接 | — | text decoration 若有 | — | B/D | render | 需核对 | P1 |
| FT-SHADOW | 文字阴影 | shadows | 标题质感 | — | — | — | D | — | 滤镜近似 | P2 |
| FT-FG-BG | 前景/背景 Paint | foreground/background | 镂空字 | — | StrokeString / 底 rect | — | B | StrokeString | — | P1 |
| FT-BIDI | 双向文本 | BiDi | 阿语/混排 | — | shaper BiDi | — | B | render/text | — | P1 |
| FT-SEL-BOXES | 选区盒 | getBoxesForSelection | 输入选区 | — | — | — | D | — | 编辑器后置 | 远期 |
| FT-POS-FOR-OFFSET | 点击定位 | getPositionForOffset | 光标 | — | — | — | D | — | IME 后置 | 远期 |
| FT-LINE-METRICS | 行度量 | computeLineMetrics | 排版调试 | — | MeasureMultiline 部分 | — | B/C | render/text.go | — | P1 |
| FT-CJK | CJK 与回退 | 字体 fallback | 中日韩 | DrawText 可绘 | isCJKText/GPU/MultiFace | RenderText | C | render/text.go | 系统字体策略 | P0 |
| FT-WRAP | 自动换行 | softWrap | 段落 | — | WordWrap/DrawStringWrapped | — | B | render/text.go | RO 未用 | P0 |
| FT-SHAPED | 已整形 glyph | shaped glyphs | 性能/复杂文种 | — | DrawShapedGlyphs | — | B | render/text.go | — | P1 |
| FT-STROKE-TEXT | 描边字 | foreground stroke | 描边标题 | — | StrokeString* | — | B | render/text.go | — | P1 |
| FT-ANCHOR | 锚点绘制 | 对齐锚 | 居中标签 | — | DrawStringAnchored | — | B | render/text.go | — | P1 |
| FT-MEASURE | 测量宽高 | TextPainter.width/height | layout 真值 | — | MeasureString | RenderText 启发式 | B | render/text.go | P0 接通 RO | P0 |

---

## §9 Picture / 录制回放母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 | Wave |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|------|
| FPic-RECORDER | Picture 录制 | PictureRecorder+Canvas | 层缓存 | — | 无完整 recorder 对 UI | Picture Valid 旗 | C/D | ui/scene/picture.go | 显示列表 P6 | P6 |
| FPic-PLAYBACK | 回放 | drawPicture | 静态层复用 | RasterizeDirty 统计复用 | — | NeedsRaster 旗 | C | scene/rasterize.go | 非 GPU 纹理复用证明 | P6 |
| FPic-TO-IMAGE | 栅格化 | Picture.toImage | 截图 | — | Export/Image | — | B | context | — | P2 |
| FPic-DISPOSE | 释放 | dispose | 防泄漏 | — | — | — | C | — | 规范待补 | P1 |

---

## §10 阴影 / 滤镜 / Blend / Mask 母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 | Wave |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|------|
| FF-SHADOW-PATH | 路径阴影 | Canvas.drawShadow | 卡片海拔 | — | ApplyDropShadow 近似 | — | B | filter_ops.go | — | P1 |
| FF-BLUR | 高斯模糊 | ImageFilter.blur | 毛玻璃 | — | ApplyBlur/ApplyBlurXY | 无 Backdrop Layer 场景 | B | filter_ops.go | — | P2 |
| FF-COLOR-MATRIX | 颜色矩阵 | ColorFilter.matrix | 置灰 | — | ApplyColorMatrix | — | B | filter_ops.go | — | P2 |
| FF-BLEND-LAYER | 混合合成 | saveLayer+blend | 叠色 | — | SetBlendMode/PushLayer | — | B | context_layer.go | — | P1 |
| FF-MASK | 遮罩层 | ShaderMask/saveLayer mask | 渐变淡出 | — | PushMaskLayer | — | B | context_layer.go | — | P2 |
| FF-BACKDROP | 背景滤镜 | BackdropFilter | 毛玻璃导航 | — | PushBackdropLayer | 无 FL-BACKDROP 场景层 | B/D | m4_extensions.go | — | P2 |
| FF-BUDGET | saveLayer 预算 | 工程纪律 | 防滥用 | SaveLayerBudget | — | — | C | clip_text.go | F16 | P1 |

---

## §11 `PaintingContext` 母表（Flutter rendering）

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 | Wave |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|------|
| FPC-CANVAS | 取 Canvas | PaintingContext.canvas | 底层绘制 | DC *render.Context | Context | — | A | painting/context.go | — | — |
| FPC-PAINT-CHILD | 绘子节点 | paintChild | 树遍历 | 子 Paint+WithOrigin | — | Box/Absolute 遍历 | A | box.go | — | — |
| FPC-CLIP-RECT | pushClipRect | PaintingContext.pushClipRect | 视口 | PushClipRect | ClipRect | ClipRectLayer | A | clip_text.go | — | — |
| FPC-CLIP-RRECT | pushClipRRect | pushClipRRect | 圆角裁子树 | — | ClipRoundRect | 无层 | B/D | — | 高频缺口 | P0 |
| FPC-CLIP-PATH | pushClipPath | pushClipPath | 异形 | — | Clip path | — | B/D | — | — | P1 |
| FPC-COLOR-FILTER | pushColorFilter | pushColorFilter | 子树滤镜 | — | 滤镜 API | — | B/D | — | — | P2 |
| FPC-OPACITY | pushOpacity | pushOpacity | 子树透明 | — | Opacity 层/CTM | OpacityLayer | A/C | scene | Present 全画 | P1 |
| FPC-TRANSFORM | pushTransform | pushTransform | 子树变换 | — | CTM | 无 TransformLayer | B/D | — | — | P1 |
| FPC-LAYER | pushLayer/addLayer | pushLayer | 自定义层 | — | — | LayerBuilder 有限 | C | scene/build.go | — | P1 |
| FPC-COMPLEX-HINT | setIsComplexHint | setIsComplexHint | 光栅缓存提示 | — | — | — | D | — | RasterCache P6 | P6 |
| FPC-WILL-CHANGE | setWillChangeHint | setWillChangeHint | 动画提示 | — | — | — | D | — | P6 | P6 |
| FPC-COMPOSITE-ONLY | retained 跳过 | 实现细节 | 脏区局部 paint | CompositeOnly+PaintVisits | — | FlushPaint(false) | A | pipeline.go | 真窗 force 全画 | — |

---

## §12 Layer 类型全量母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 | Wave |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|------|
| FL-CONTAINER | 容器层 | ContainerLayer | 树节点 | — | — | ContainerLayer | A | scene/layer.go | — | — |
| FL-OFFSET | 位移层 | OffsetLayer | 滚动 | — | — | OffsetLayer | A | layer.go | — | — |
| FL-TRANSFORM | 变换层 | TransformLayer | 旋转缩放合成 | — | CTM only | **无** | D | — | 关键 | P1 |
| FL-OPACITY | 透明层 | OpacityLayer | 淡入 | MutSetOpacity | — | OpacityLayer | A/C | compositing.go | 真合成未接到 Present | P1 |
| FL-CLIP-RECT | 裁剪层 | ClipRectLayer | 溢出 | — | — | ClipRectLayer | A | layer.go | Build 使用有限 | — |
| FL-CLIP-RRECT | 圆角裁剪层 | ClipRRectLayer | 卡片 | — | — | **无** | D | — | — | P1 |
| FL-CLIP-PATH | 路径裁剪层 | ClipPathLayer | 异形 | — | — | **无** | D | — | — | P2 |
| FL-PICTURE | 图层层 | PictureLayer | 录制内容 | — | — | PictureLayer+NeedsRaster | C | picture.go | 无 op 缓冲 | P6 |
| FL-TEXTURE | 纹理层 | TextureLayer | 视频 | — | DrawGPUTexture | **无** | D | — | — | P2 |
| FL-PLATFORM-VIEW | 平台视图 | PlatformViewLayer | WebView/地图 | — | — | **无** | D | — | — | 远期 |
| FL-SHADER-MASK | 着色遮罩 | ShaderMaskLayer | 渐变淡出列表 | — | Mask 近似 | **无** | D | — | — | P2 |
| FL-BACKDROP | 背景滤镜层 | BackdropFilterLayer | 毛玻璃 | — | PushBackdropLayer | **无** | D | — | — | P2 |
| FL-COLOR-FILTER | 颜色滤镜层 | ColorFilterLayer | 子树置灰 | — | ApplyColorMatrix | **无** | D | — | — | P2 |
| FL-IMAGE-FILTER | 图像滤镜层 | ImageFilterLayer | 模糊子树 | — | ApplyBlur | **无** | D | — | — | P2 |
| FL-LEADER | LeaderLayer | LeaderLayer | 跟随定位锚点 | — | — | **无** | D | — | Overlay 高级 | P2 |
| FL-FOLLOWER | FollowerLayer | FollowerLayer | Tooltip 跟随 | — | — | **无** | D | — | — | P2 |
| FL-ANNOTATED | 注解层 | AnnotatedRegionLayer | 语义/系统 UI | semantics 薄 | — | — | C | ui/semantics | 非 Layer 实现 | P2 |
| FL-PERF | 性能叠加层 | PerformanceOverlayLayer | HUD | — | — | **无** | D | — | P6 HUD | P6 |
| FL-BOUNDARY | 重绘边界 | RepaintBoundary→层 | 脏区隔离 | SetRepaintBoundary | — | BoundaryLayer | A | node.go · layer.go | — | — |
| FL-OVERLAY-BAND | 覆盖带 | Overlay | 弹层 | ui/overlay | — | FramePacket.Overlay | A | overlay · scene | F13 | — |

---

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

## §14 调度 · Vsync · 帧管道母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 | Wave |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|------|
| FSch-VSYNC | 垂直同步 | SchedulerBinding/vsync | 稳帧 | VSyncWaiter 接口 | — | Host 可选 | C | platform/host.go · vsync.go | exhost 多无 WaitVSync；fallback 16ms | P1 |
| FSch-TRANSIENT | 短暂帧回调 | scheduleFrameCallback | 动画 | Mode TRANSIENT | — | FrameScheduler | A | scheduler.go | — | — |
| FSch-PERSISTENT | 每帧回调 | persistent frame callbacks | 连续动画 | Mode PERSISTENT + Ticker | — | — | A | scheduler.go | — | — |
| FSch-IDLE | 空闲阻塞 | 无事不转 | 省电 | Mode IDLE + WaitEvents | — | — | A | scheduler.go | S0 | — |
| FSch-WARMUP | 热身帧 | warm-up frame | 减首帧 jank | PipelineOptions.WarmUp | — | — | A | pipeline_app.go | F11 | — |
| FSch-TIMINGS | FrameTiming | FrameTiming | 分轨耗时 | frame_build_ms/raster_ms | — | MetricsStore | C | metrics.go | 多为 last；缺直方图 | P0 |
| FSch-TIMINGS-CB | timings callback | addTimingsCallback | 自定义采集 | — | — | — | D | — | 可扩展 Metrics | P1 |
| FSch-TICKER | Ticker | Ticker/AnimationController | 动画驱动 | animation.Controller+TickerRegistry | — | — | A | animation · ticker.go | F17 | — |
| FPerf-OVERLAY | 性能 HUD | PerformanceOverlay | 开发态 | — | — | — | D | — | P6 | P6 |
| FPerf-DEVTOOLS | CPU/内存工具 | DevTools | 优化 | 无内建 | render mem harness | 示例 JSON 部分 | C/D | mem_harness_test.go | ui 需接入 | P0 |

---

## §15 滚动 / 视口 / 虚拟化母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 | Wave |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|------|
| FScroll-OFFSET | 滚动偏移 | Viewport offset | 列表 | SetScrollOffset/ScrollBy | — | Viewport+Offset | A | viewport.go | 默认只 MarkNeedsPaint | — |
| FScroll-CLIP | 视口裁剪 | clip canvas | 离屏不画 | PushClip 于 Viewport.Paint | — | — | A | viewport.go | — | — |
| FScroll-VIRTUAL | 虚拟化 | SliverChild | 长列表 | VirtualList 固定行高 | — | — | A | virtual_list.go | 可变高 D | P2 |
| FScroll-CACHE | cacheExtent | cacheExtent | 预创建窗外 | CacheExtent | — | VirtualList | A | virtual_list.go | — | — |
| FScroll-NEST | 嵌套滚动 | NestedScroll/竞技 | 父子列表 | Scrollable.Parent 移交 | — | gestures+scrollable | A | nested_scroll_test.go | P5 | — |
| FScroll-VAR-EXTENT | 可变行高 | Sliver 可变 | 聊天列表 | — | — | — | D | — | — | P2 |
| FScroll-PHYSICS | 物理回弹 | ScrollPhysics | iOS/Android 感 | — | — | — | D | — | 产品感后置 | P2 |

---

## §16 坐标 · DPR · HitTest 母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 | Wave |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|------|
| FCoord-LOGICAL | 逻辑像素 | logical pixels / dp | 布局点击 | 全程逻辑 px | DeviceScale | Host.Size | A | 架构文 §4 | F07 | — |
| FCoord-PHYSICAL | 物理像素 | device pixels | 纹理 | — | PixelWidth/Height | Swapchain | A | render | — | — |
| FCoord-DPR | devicePixelRatio | MediaQuery.devicePixelRatio | HiDPI | Context.Scale | WithDeviceScale | Host.ScaleFactor | A | — | — | — |
| FCoord-YDOWN | Y 向下 | Flutter 布局 | 一致 | Y-down | 2D UI 惯例 | — | A | — | — | — |
| FCoord-HIT | HitTest | HitTestResult | 指针 | HitTest | — | 各 RO + Overlay 先 | A | embedder | — | — |

---


## §17 gpui 现状快照 — `ui/painting` 实际 API

| API | 文件 | 对应 Flutter | 状态 |
|-----|------|--------------|------|
| `Context` / `New` / `WithOrigin` / `Scale` / `DC` | context.go | PaintingContext 子集 | A |
| `CompositeOnly` / `PaintVisits` / `NotePaintVisit` | context.go | retained paint | A |
| `FillRect` / `FillRectCPU` | context.go | drawRect fill | A |
| `FillRoundRect` | shapes.go | drawRRect fill | A |
| `FillLinearGradient` / `FillLinearGradient2` / `GradientStop` | gradient.go | Gradient.linear | A |
| `PushClipRect` / `PopClip` | clip_text.go | pushClipRect | A |
| `DrawText` / `DrawTextColored` | clip_text.go | 弱版 drawParagraph | C |
| `DrawImageBuf` | clip_text.go | drawImageRect 简化 | A |
| `SaveLayerBudget` | clip_text.go | saveLayer 纪律 | C |

**未暴露（见 §2–§10 中 B/D）：** Stroke*、Path、Circle/Oval/Arc、ClipRRect/Path、Radial/Sweep、MeasureText、PushLayer、Transform、Blur…

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
| Container / Offset / Opacity / ClipRect / Picture / Boundary | A 或 C(Picture) |
| Transform / ClipRRect / ClipPath / Texture / Backdrop / Filter / Leader… | **D** |

### 18.3 Present 诚实条款

| 项 | 状态 |
|----|------|
| `render.PresentFrameDamage*` | B（render 有） |
| `PipelineApp` 每帧 force 全清+全 paint | **C 缺口**（注释写明 P6） |
| 真 dirty-rect Present | **D / P6** |

---

## §19 F01–F18 交叉表

| F | 内容 | 状态 | 关联能力 ID | 残余 |
|---|------|------|-------------|------|
| F01 | Layer COW | A | FL-* FramePacket | — |
| F02 | 静态层复用/脏 re-raster | A/C | FL-PICTURE RasterizeDirty | 非 GPU RT |
| F03 | RelayoutBoundary | A | FRO-RELAYOUT-B | — |
| F04 | compositing 传播 | A/C | FRO-MARK-COMP | 加深 |
| F05 | 滚动协议 | A | FScroll-* | 可变高 D |
| F06 | 真 vsync | C | FSch-VSYNC | exhost |
| F07 | 逻辑/物理/Y-down | A | FCoord-* | — |
| F08 | 输入不等 Present | A | SubmitLatest | — |
| F09 | 动画 compositor-only | C | FX-OPACITY FL-OPACITY | Present 全画 |
| F10 | GPU/render 热路径 | A/B | render 丰富 ui 薄 | 暴露 P0 |
| F11 | warm-up | A | FSch-WARMUP | — |
| F12 | IO 解码 | A | FImg-CODEC | — |
| F13 | Overlay band | A | FL-OVERLAY-BAND | — |
| F14 | 帧计时 JSON | A/C | §20 | 分位/CPU/RSS |
| F15 | 遮挡停帧 | C | — | 加深 |
| F16 | saveLayer 预算 | C | FF-BUDGET FC-SAVELAYER | 真层 P1 |
| F17 | Ticker/Animation | A | FSch-TICKER | — |
| F18 | Present 策略可文档化 | C | §18.3 | 本文 + P6 |

---

## §20 正向性能与资源指标全表

### 20.1 对照 Flutter / 工程

| 指标族 | Flutter / 工程 | gpui 现状 |
|--------|----------------|-----------|
| 帧时 / FPS | FrameTiming、60Hz | avg/max/last；无 p50/p99 |
| Jank | jank 帧 | hitch_count @ 33.4ms |
| UI vs Raster | build/raster span | frame_build_ms / frame_raster_ms（常 last） |
| 管道 | pipeline | depth/max=2 |
| 局部性 | RepaintBoundary | raster_layer_count、PaintVisits、S2/S4 |
| layout/paint 计数 | 调试 | JSON 字段常 **未接线** → C |
| CPU | DevTools | **ui 无** → D |
| 内存 | Observatory / RSS | render `VmRSS` harness；**ui 示例无** |
| Vsync miss | vsync | 接口有；无真 waiter 时无意义 |
| 关闭释放 | dispose 后内存 | **未门禁** |

### 20.2 指标 ID 全表

| ID | 指标 | 对照 | 来源现状 | 单位 | 状态 | 正向门禁方向 | Wave |
|----|------|------|----------|------|------|--------------|------|
| M-FRAME | frame_count | 帧计数 | MetricsStore | count | A | 随 schedule 增 | — |
| M-PRESENT-SUBMIT | 提交 present 数 | — | PipelineApp.PresentCount | count | A | 与 complete 区分 | — |
| M-PRESENT-COMPLETE | 完成 present | — | NotePresent JSON | count | A | ≤ submit | — |
| M-INTERVAL-LAST | 上次帧间隔 | FrameTiming | metrics | ms | A | ~16.7 @60 | — |
| M-INTERVAL-AVG | 平均间隔 | — | metrics | ms | A | ~16.7 | — |
| M-INTERVAL-MAX | 最大间隔 | — | metrics | ms | A | 观察尖峰 | — |
| M-INTERVAL-P50 | 分位 p50 | DevTools | **无** | ms | D | ≈16.7 | P0 |
| M-INTERVAL-P95 | 分位 p95 | — | **无** | ms | D | 预算 | P0 |
| M-INTERVAL-P99 | 分位 p99 | — | **无** | ms | D | ≤20~33 轻场景 | P0 |
| M-FPS-WALL | wall fps | — | presents/elapsed | Hz | A 示例算 | ≥55 动画 | — |
| M-FPS-COMPLETE | 完成帧 fps | — | complete/elapsed | Hz | C | 诚实 FPS | P0 |
| M-TARGET-HZ | 目标刷新 | 60/120 | 文档+DefaultAnimTick | Hz | C | 标明 60 | — |
| M-HITCH | hitch_count | jank>2frame | >33.4ms | count | A | soak 低速率 | — |
| M-HITCH-RATE | hitches/min | — | 派生 | /min | D | SLO | P0 |
| M-JANK-33 | >33.4ms 次数 | — | =hitch | count | A | — | — |
| M-JANK-50 | >50ms | 严重卡顿 | **无** | count | D | 0 理想 | P0 |
| M-MISSED-VSYNC | missed_vsync | vsync miss | 仅 Wait 错误时 | count | C | 真 vsync 后才有意义 | P1 |
| M-VSYNC-SOURCE | true/fallback | — | **无字段** | enum | D | JSON 标明 | P0 |
| M-BUILD-MS | frame_build_ms | UI build | NoteBuildMs | ms | A | p99 预算 | P0 分位 |
| M-RASTER-MS | frame_raster_ms | raster | NoteRasterMs | ms | A | p99 预算 | P0 分位 |
| M-PIPE-DEPTH | pipeline_depth | 在途 | SetPipeline | int | A | ≤2 | — |
| M-PIPE-MAX | pipeline_max | 配置/峰值 | 常为配置 2 | int | C | 记录观测峰值 | P0 |
| M-LAYOUT-COUNT | layout_count | layout 范围 | 字段常 0；LayoutFlushCount 有 | count | C | S2=0 | P0 接线 |
| M-PAINT-COUNT | paint_count | paint 范围 | **未接线** | count | C | ∝脏 | P0 |
| M-PAINT-VISITS | PaintVisits | retained walk | 测试用 | count | A 测 | M 矩阵 | — |
| M-RASTER-LAYER | raster_layer_count | 脏层 | SetRasterLayerCount | count | A | S2 小 | — |
| M-SKIP-LAYER | skipped layers | 静态复用统计 | RasterStats | count | A 内部 | S4 | — |
| M-COMPOSITE-LAYER | composite_layer_count | 合成 | 占位 | count | C | — | P1 |
| M-BIND | virtual bind | sliver children | VirtualList.BindCount | count | A | ≪ itemCount | — |
| M-CPU-PROCESS | 进程 CPU% | DevTools | **无** | % | D | S0≈0 | P0 |
| M-CPU-UI | UI 线程 CPU | — | **无** | % | D | S2 上界 | P1 |
| M-CPU-RASTER | Raster CPU | — | **无** | % | D | — | P1 |
| M-CPU-IDLE | 空闲 CPU | — | **无** | % | D | S0 | P0 |
| M-RSS-START | 起始 VmRSS | 内存 | render harness only | KB | C | — | P0 ui |
| M-RSS-END | 结束 VmRSS | — | — | KB | C | — | P0 |
| M-RSS-PEAK | 峰值 RSS | — | — | KB | D | — | P0 |
| M-RSS-SLOPE | 斜率 KB/min | 泄漏 | — | KB/min | D | ≈0 soak | P0 |
| M-RSS-AFTER-CLOSE | 关闭后 RSS | 释放 | **无** | KB | D | 下降可解释 | P0 |
| M-HEAP | HeapAlloc | runtime | render 测有 | B | C | — | P0 |
| M-NUM-GC | GC 次数 | — | — | count | D | soak | P1 |
| M-GOROUTINES | 协程数 | 泄漏 | — | count | D | 稳定 | P1 |
| M-GPU-FALLBACK | CPU 回退原因 | — | LastCPUFallbackReason | string | B | 诊断 | P1 |
| M-LAYER-POOL | 层池命中 | saveLayer | LayerPoolStats | — | B | — | P1 |
| M-ATLAS-HIT | 字形 atlas | 文本 | 内部 | — | B | — | P1 |
| M-DAMAGE-AREA | 脏像素面积 | partial present | **P6** | px | D | **不本阶段门禁** | P6 |

### 20.3 采集建议

- Linux：`/proc/self/status` VmRSS（`render/mem_harness_test.go`）
- CPU：`/proc/self/stat` 或定时采样
- 帧分位：`MetricsStore` 环形直方图
- 示例 stdout 扩展 JSON；**无 baseline 不谈优化**
- 无真 vsync 时写「软件 pace ~16ms」，不得宣称「锁显示 60Hz」

---

## §21 场景门禁矩阵

| 场景 | 内容 | 关键指标 | 通过方向 |
|------|------|----------|----------|
| S0 | 空闲 | M-CPU-IDLE | CPU≈0 |
| S2 | 单 Spinner | M-LAYOUT, M-RASTER-LAYER, M-FPS* | layout≈0；~60fps |
| S4 | 静态+1 动画 | M-SKIP-LAYER | 静态 skip |
| S5 | 虚拟列表 | M-BIND | bind 上界 |
| S6 | 拖拽滚动 | M-LAYOUT | 无 layout 风暴 |
| Soak | 5–10min | M-RSS-SLOPE, M-HITCH-RATE | 斜率≈0 |
| Close | 关窗 | M-RSS-AFTER-CLOSE | 释放 |
| 60fps+ | Persistent | M-INTERVAL-P50/P99 | p50≈16.7；hitch 低 |
| M1–M5 | render_matrix | 见 examples | 契约+视觉 |

真窗 Present 仍全量时，**不可**用 present 路径 PaintVisits 证明 P6。

---

## §22 缺口注册表（按优先级）

| 优先级 | 缺口 | 状态 | Wave |
|--------|------|------|------|
| P0 | Stroke* + line style → painting | B | P0 |
| P0 | MeasureText → RenderText | B/C | P0 |
| P0 | FillCircle；ClipRRect UI | B | P0 |
| P0 | layout/paint 计数接线；分位；CPU/RSS | C/D | P0 |
| P1 | Radial/Sweep；Wrap 文本；Font；TransformLayer；saveLayer UI；真 VSync | B/C/D | P1 |
| P2 | Path metrics；可变行高；ellipsis；Backdrop 场景层 | D/B | P2 |
| P6 | Picture 显示列表；dirty-rect Present；层 RT；HUD | D/C | **P6 单列** |
| 远期 | PlatformView；FragmentShader；IME 选区 | D | 远期 |

---

## §23 补齐 Wave

**P0：已收尾（2026-07-27）**

已实现：
- `ui/painting`：Stroke*/Circle/ClipRoundRect、MeasureText、EstimateTextSize、`TryLoadDefaultFace`（`GPUI_UI_FONT`）
- `RenderText`：rune 估算 + optional Face
- `FrameMetrics`：layout/paint 计数、p50/p99、`vsync_source`
- **进程资源：** `ProcessTracker` + `ReadRSSKB` → JSON `rss_start/end/peak/after_close_kb`、`cpu_pct_avg`
- 示例接线：`ui_l1_spinner` / `ui_l1_scroll` / `ui_l1_render_matrix`（含默认字体尝试）

说明：`rss_after_close` 可能因驱动/分配器延迟仍接近 peak，作观察字段而非硬释放证明。

**P1：** 径向/扫掠渐变、DrawTextWrapped、Font 体系、TransformLayer、预算内 PushLayer、exhost 真 VSync、Path 封装。  
**P2：** ellipsis/maxLines、path metrics、可变高、Backdrop/Filter 层。  
**P3：** 窗测矩阵扩展、soak 入库。  
**P6（单列）：** dirty-rect Present、Picture 录制、层 RT、HUD。

## §24 窗口专项测试程序设计

**路径：** `examples/ui_render_base_gallery/` 或扩展 `ui_l1_render_matrix`（**禁止** `ui/` demo 包）

| 轴 | 覆盖 | 验证 |
|----|------|------|
| Geometry | FC-DRAW-* | 视觉 |
| Text | FT-* | 布局+字形 |
| Image | FImg-* | 异步不堵 |
| ClipLayer | Clip/Opacity/Boundary | 脏局部 |
| Scroll | FScroll-* | bind 上界 |
| PerfSoak | §20 | 60fps+、CPU、RSS、hitch、close |

环境：`RUN_SECONDS`、`AXIS=`、`GPUI_DISPLAY`；stdout JSON + 可选 baseline。

---

## §25 文档完备性清单

```text
[x] Flutter Canvas / Paint / Path / Text / Picture / PaintingContext / Layer / RO / Scheduler 母表
[x] 不支持项保留为 D/B/C
[x] 指标含 CPU/RSS/泄漏/释放/60fps/hitch/分位
[x] Present force 与 P6 诚实
[x] F01–F18 · Wave · 窗测
[x] README 挂链
```

---

## §26 修订历史

| 版本 | 日期 | 说明 |
|------|------|------|
| 1.0 | 2026-07-27 | 首版：Flutter 母表全量 × gpui 映射 + 正向指标 + Wave + 窗测设计 |

---

## 参考

- [Canvas (dart:ui)](https://api.flutter.dev/flutter/dart-ui/Canvas-class.html)
- [Paint](https://api.flutter.dev/flutter/dart-ui/Paint-class.html)
- [Path](https://api.flutter.dev/flutter/dart-ui/Path-class.html)
- [Paragraph](https://api.flutter.dev/flutter/dart-ui/Paragraph-class.html) / [TextPainter](https://api.flutter.dev/flutter/painting/TextPainter-class.html)
- [PaintingContext](https://api.flutter.dev/flutter/rendering/PaintingContext-class.html)
- [RenderObject](https://api.flutter.dev/flutter/rendering/RenderObject-class.html)
- [Layer](https://api.flutter.dev/flutter/rendering/Layer-class.html)
- 本仓库：`ui/painting` · `ui/rendering` · `ui/scene` · `render/*` · `docs/ENGINE_FLUTTER_SKIA_ARCH.md`
