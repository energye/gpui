# UI 渲染基座 — Flutter 母表 × gpui

> **版本：1.29** | 日期：2026-07-28  
> **范围：** 渲染基座（画 / 排 / 合 / 滚 / 调度 + 帧与资源指标）。**不是** 整站 Flutter、不是 Ant 控件。  
> **唯一文档：** 本文件。  
> **交叉：** [`ENGINE_CODING_RULES.md`](./ENGINE_CODING_RULES.md) · [`ENGINE_FLUTTER_SKIA_ARCH.md`](./ENGINE_FLUTTER_SKIA_ARCH.md)  
> **收口：** §22 主路径 + D1–D8 **已收口**（见 **§25**）。全表仍有 B/C/D → **不得** 宣称「已对齐 Flutter 全量渲染」。  
> **下一层（自定义控件渲染基座 · 统一真源）：** [`ENGINE_UI_WIDGET_RENDER.md`](./ENGINE_UI_WIDGET_RENDER.md)（§R · 域/L0 地图 · W0–W6；**非** 本表画 API 清零）。

### 闭环（流程已闭合 · §22 主路径已收口 · 全表对照未清零）

```text
§2–§16 能力清单  →  §22 依赖序实现  →  §0.6 三项验收
                                      ├ ① 单元测试
                                      ├ ② 正向指标（§20）
                                      └ ③ 窗口测试
                 ← 状态回写本表 ←┘
```

| | 含义 |
|--|------|
| **流程闭环** | 有清单、有顺序、有验收、有回写 |
| **§22 主路径收口** | 序 1–13 主项 + 指标 D8 已达「可画 / 可滚 / 可调度 / 可观测」；细项 A/C 边界见各 § |
| **全表未清零** | 母表仍有 B/C/D（Flutter 对照全集）；**清 D ≠ 基座目标**；下一刀看产品场景或 §25 残项 |

### 施工（收口后）

```text
1. 新产品缺口 → 对照母表 ID → 单独立项（勿无场景清 D）
2. 仍按 §0.6 三项验收；只改状态/证据/§22.2 残项
3. 加深项优先：产品会踩到的 A/C 边界，而非 PlatformView/IME/SkSL
```

---

## §0 元信息 · 读法 · 非宣称

### 0.1 状态码（每行必填其一）

| 码 | 含义 |
|----|------|
| **A** | **UI 场景路径已可用**：经 `PaintContext` / RO / scene 已接，可测 |
| **B** | **仅 `render/` 具备**，UI 未暴露给控件/场景 paint |
| **C** | **半成品**：接口/字段/预算有，语义不全、未接线、或仅统计无 GPU 证明 |
| **D** | **相对 Flutter 缺失**（母表有、gpui 无；或明确远期/序13 未做） |

### 0.2 允许 / 禁止宣称

```text
允许：L1 retained-paint 契约（Boundary/CompositeOnly/DirtyLayerIDs）；render Skia 式 2D 能力丰富；
      帧间隔/hitch/RSS slope/GPU path/管道可观测；虚拟列表 bind 上界等已测门禁；
      §22 主路径收口（§25）— 仍须带诚实边界。

禁止：宣称「已对齐 Flutter 全量渲染」；Picture/Canvas/Paragraph 与 Flutter 对等；
      在无 damage 证据时宣称 dirty-RECT 局部 Present；锁显示 60Hz（vsync_source≠true）；
      无同机 baseline 的「最优/更顺」；把母表 D 行当施工失败。
允许（有证据）：PipelineApp 稳态 force=false + PresentFrameAuto + FrameDamage 面积 ≪ 全屏（§18）；
      BASELINE_JSON 对比（容差内）谈回归。
```

### 0.3 列定义

能力表列：ID · Flutter 能力/参考 · UI 用途 · gpui.ui / render / scene·RO · **状态** · 证据 · 缺口。**实现顺序只看 §22。**

### 0.4 工程纪律

```text
ui → render → gpu    禁止 ui→gpu    禁止 CGO（purego）
架构在 ui/ · 示例在 examples/    见 ENGINE_CODING_RULES
```

### 0.5 指标何时采

**每实现一项能力就采**，不要等基座全做完。指标清单与门禁方向见 **§20**。  
无同机、同场景 baseline 的「变快/更顺」无效。

### 0.6 每项能力验收（硬 · 三项全过才算完成）

| # | 验收 | 必须 | 说明 |
|---|------|------|------|
| **1** | **单元测试** | `go test` 相关包绿 | 新行为至少 1 条测；可复用 S2/S4/S5 等门禁 |
| **2** | **正向性能指标** | 采到 §20 相关 M-\*，不无故回退 | 通用：帧间隔/hitch、CPU、RSS；专项见 §22 该行「指标」列 |
| **3** | **窗口测试** | 对应 `examples/ui_render_base_*` 或滚动/门禁场景可跑 | 无轴则为本项补最小窗测后再标完成；禁止只合 API |

同时：母表 **状态列诚实**（仅 render→B；UI 可测→A；半成品→C；未做→D）。

```text
禁止：缺任一项验收就标完成。
禁止：用 MVP 改写 Flutter 目标；进度只改状态/证据/§22。
```

**常用命令：**

```bash
go test ./ui/... -count=1
go test ./ui/rendering -run 'TestS2_|TestS4_|TestS5_|TestS6_|TestM' -count=1
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
go run ./examples/ui_render_base_geometry   # 几何
go run ./examples/ui_render_base_text       # 文本
go run ./examples/ui_render_base_image       # 图像
go run ./examples/ui_render_base_cliplayer  # Clip/Layer
go run ./examples/ui_render_base_perfsoak   # 指标回归（每项建议）
go run ./examples/ui_l1_scroll              # 滚动
```

---

## §1 架构对照（现状）

| Flutter | gpui | 摘要 |
|---------|------|------|
| Canvas / Paint | `render.Context` + `PaintContext.DC` | render 多；UI 暴露仍有 B |
| PaintingContext | `ui/rendering.PaintContext` | 子集 |
| RenderObject / Pipeline | `ui/rendering` | 主路径 A |
| Layer | `ui/scene` | 常用层 A/C；Filter/Texture D |
| Scheduler / Vsync | `ui/scheduler` | 接口 A；DRM 真 VSync A/C |
| 帧/资源指标 | `MetricsStore` + 示例 JSON | 分位/hitch/RSS/**slope**/GPU path/baseline **A/C（D8）** |
| dirty-rect Present | render + **PipelineApp 稳态 Auto** | **A/C（序13）** 首帧/resize 仍 full；OS damage 可忽略 |
| Picture 显示列表 | scene PictureRecorder/Replay | **A/C（序13/D7）** rect/path/text/image；≠ Flutter 全量；≠ partial Present |

```text
一帧：ScheduleFrame → layout → paint → FramePacket → RasterizeDirty
    → SubmitLatest → Present（稳态 damage/Auto；首帧/resize 全清）
```

---

## §2 `dart:ui.Canvas` 全量能力母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|
| FC-SAVE | 画布状态压栈 | Canvas.save | 嵌套 clip/transform | **Save** | Push | — | **A** | paint_context.go · transform_ctm_test | CTM+clip 栈 |
| FC-RESTORE | 画布状态出栈 | Canvas.restore | 与 save 配对 | **RestoreCanvas** / PopTransform | Pop | — | **A** | paint_context.go | 与 SaveLayer.Restore 分离 |
| FC-RESTORE-N | 恢复到指定深度 | Canvas.restoreToCount | 错误恢复/多层一次性弹出 | — | — | — | D | — | Flutter 有；gpui 无对等 API |
| FC-SAVECOUNT | 查询 save 深度 | Canvas.getSaveCount | 调试/断言 | — | clipStackDepth 内部 | — | C | context_clip*.go | 未作 UI 公开 API |
| FC-SAVELAYER | 离屏层+Paint | Canvas.saveLayer(bounds,paint) | 半透明组、滤镜、阴影组 | **SaveLayer/Restore** + Budget | **PushLayerIsolated** | — | **A/C** | paint_context · save_layer_test · context_layer | 全幅隔离 RT；bounds 仅预算；非 paint 全参 |
| FC-SAVELAYER-NULL | 全画布 saveLayer | saveLayer(null,…) | 极重；默认应禁 | 预算 MaxArea 可拦 | PushLayer 全幅 | — | C | SaveLayerBudget | F16 纪律 |
| FC-TRANSLATE | 平移 CTM | Canvas.translate | 局部坐标系 | **Translate** + WithOrigin | Translate | OffsetLayer | **A** | paint_context · transform_ctm_test | 本地空间经 origin 共轭 |
| FC-SCALE | 缩放 CTM | Canvas.scale | 缩放动画/适配 | **ScaleXY** · RenderTransform | Scale | TransformLayer | **A** | paint_context · transform | 2D；非透视 |
| FC-ROTATE | 旋转 CTM | Canvas.rotate | 旋转图标/翻牌 | **Rotate/RotateAbout** · RenderTransform | Rotate* | TransformLayer | **A** | paint_context · transform_hit_test | 2D；非 Matrix4 |
| FC-SKEW | 错切 CTM | Canvas.skew | 少见 UI 效果 | **Shear** | Shear | — | **A** | paint_context · transform_ctm_test | |
| FC-TRANSFORM | 任意 Matrix4 | Canvas.transform | 通用仿射 | **Concat/PushTransform** | Transform | — | **A/C** | paint_context · transform_ctm_test | **2D 仿射**；非透视 Matrix4 |
| FC-GET-TRANSFORM | 读取 CTM | Canvas.getTransform | 命中反变换/调试 | **GetTransform** | GetTransform | — | **A** | paint_context.go | 返回 user matrix 副本 |
| FC-CLIP-RECT | 矩形裁剪 | Canvas.clipRect+ClipOp | 列表视口、溢出隐藏 | PushClipRect/PopClip | ClipRect/ClipRectOp | ClipRectLayer | A | rendering/paint_context.go | ClipOp 差异在 render |
| FC-CLIP-RRECT | 圆角裁剪 | Canvas.clipRRect | 卡片/头像裁切 | PushClipRRect · **RenderClipRRect** | ClipRoundRect | ClipRRectLayer RO→Build | **A** | paint_context · clip_rrect · layer_build | 均匀圆角；Present 仍全画 |
| FC-CLIP-PATH | 路径裁剪 | Canvas.clipPath | 异形遮罩 | **PushClipPath** | Clip | 无 ClipPathLayer 仍 D | **A/B** | paint_context · save_layer_test | UI paint 可裁；场景层仍无 |
| FC-CLIP-BOUNDS-LOCAL | 本地裁剪界 | getLocalClipBounds | 优化/命中 | — | 内部 clip 栈 | — | C | context_clip*.go | 未 UI 暴露 |
| FC-CLIP-BOUNDS-DEST | 设备裁剪界 | getDestinationClipBounds | damage 对齐 | — | — | — | D | — | 可与序13 damage 联动 |
| FC-DRAW-COLOR | 整幅着色 | Canvas.drawColor | 清屏/遮罩 | 清色在 Present 路径 | Clear/ClearWithColor | — | B | render/context.go | PipelineApp 清屏 |
| FC-DRAW-PAINT | 用 Paint 填当前 clip | Canvas.drawPaint | 全 clip 填着色器 | — | SetFillBrush+大 rect Fill | — | B | render/context.go | 无 1:1 API |
| FC-DRAW-RECT | 矩形 | Canvas.drawRect | 色块/背景 | FillRect / StrokeRect | DrawRectangle+Fill/Stroke | ColorBox | **A** | rendering/draw.go | fill+stroke；无持久 Paint 对象 |
| FC-DRAW-RRECT | 圆角矩形 | Canvas.drawRRect | 按钮/卡片 | FillRoundRect / StrokeRoundRect | DrawRoundedRectangle* | — | **A** | draw.go · geometry 轴 | 均匀圆角；XY 不等圆角仍 B |
| FC-DRAW-DRRECT | 双 RRect 环 | Canvas.drawDRRect | 环形进度/边框环 | **FillDRRect** | EvenOdd 双 RRect path | — | **A/C** | draw.go · draw_test | 均匀圆角 even-odd 近似；非 per-corner |
| FC-DRAW-OVAL | 椭圆 | Canvas.drawOval | 头像底/高亮 | FillOval / StrokeOval | DrawEllipse | — | **A** | draw.go | — |
| FC-DRAW-CIRCLE | 圆 | Canvas.drawCircle | 头像/点/涟漪 | FillCircle / StrokeCircle | DrawCircle | Spinner 仍可用 rect 点 | **A** | draw.go | — |
| FC-DRAW-ARC | 弧/扇形 | Canvas.drawArc | 进度环 | FillArc / StrokeArc | DrawArc/DrawEllipticalArc | — | **A** | draw.go | 椭圆弧 API 仍可加深 |
| FC-DRAW-PATH | 任意路径 | Canvas.drawPath | 图标/波浪/自定义 | NewPath+**PathAdd***+FillPath · **SetFillRule** | Path/Fill | — | **A** | draw.go | PathAddRect/RRect/Oval；fillType UI |
| FC-DRAW-LINE | 线段 | Canvas.drawLine | 分割线/刻度 | StrokeLine | DrawLine | — | **A** | draw.go | — |
| FC-DRAW-POINTS | 点列 | Canvas.drawPoints/RawPoints | sparklines | **DrawPoints** | DrawPoint | — | **A** | draw.go · draw_test | 圆点半径；非 PointMode 枚举全量 |
| FC-DRAW-VERTICES | 顶点网格 | Canvas.drawVertices | 扭曲图/网格渐变 | **DrawVertices** | DrawVertices | — | **A/C** | draw.go · vertices.go · draw_test | Triangles/Fan；CPU 均色 |
| FC-DRAW-ATLAS | 图集精灵 | Canvas.drawAtlas | 图标合批、粒子 | **DrawAtlas** | DrawAtlas | — | **A** | image_draw.go · vertices.go | UI 门面 |
| FC-DRAW-SHADOW | 路径阴影 | Canvas.drawShadow | Material 海拔阴影 | **ApplyDropShadow** | ApplyDropShadow | — | **A/C** | filter_draw.go · filter_draw_test | 全幅 alpha 阴影；非 path-bound elevation |
| FC-DRAW-IMAGE | 绘图像 | Canvas.drawImage | 图标/位图 | **DrawImageBuf** | DrawImage | RenderImage | A | image_draw.go · RenderImage | 公开 UI 门面 |
| FC-DRAW-IMAGE-RECT | 源/目标矩形 | Canvas.drawImageRect | 裁剪缩放 | **DrawImageRect** / DrawImageBuf | DrawImageEx | RenderImage | **A** | image_draw.go | |
| FC-DRAW-IMAGE-NINE | 九宫格 | Canvas.drawImageNine | 可拉伸边框 | **DrawImageNine** | DrawImageNine | — | **A** | image_draw.go · image_draw_test | 均匀 center 矩形；Atlas UI 仍开 |
| FC-DRAW-PARAGRAPH | 绘段落 | Canvas.drawParagraph | 所有正式文本 | DrawText/Colored 子集 | DrawString* | RenderText | C | ui/rendering/text.go | 非 Paragraph 模型 |
| FC-DRAW-PICTURE | 回放 Picture | Canvas.drawPicture | 层缓存回放 | Picture.Replay | Context 绘 | PictureLayer+Ops | **A/C** | scene/picture.go · picture_test | rect/path/text/image 回放；**非** dirty-rect Present；非 GPU picture 缓存 |

---

## §3 `dart:ui.Paint` / Shader 母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|
| FP-COLOR | 纯色 | Paint.color | 控件填色 | Fill*/Stroke* 参数 RGBA | SetRGBA/SetColor/SetHexColor | — | **A** | draw.go | 无持久 Paint 对象 |
| FP-STYLE | fill/stroke | Paint.style | 描边按钮 | Fill* / Stroke* 分函数 | Fill vs Stroke 路径 | — | **A** | draw.go | 非单一 Paint.style 枚举 |
| FP-STROKE-W | 描边宽 | Paint.strokeWidth | 分割线粗细 | lineWidth 参数 / SetStrokeStyle | SetLineWidth | — | **A** | draw.go | — |
| FP-STROKE-CAP | 线帽 | Paint.strokeCap | 圆角线端 | SetStrokeStyle | SetLineCap | — | **A** | draw.go | — |
| FP-STROKE-JOIN | 线接 | Paint.strokeJoin | 折线拐角 | SetStrokeStyle | SetLineJoin | — | **A** | draw.go | — |
| FP-STROKE-MITER | 斜接限制 | Paint.strokeMiterLimit | 尖角 | — | Miter 相关 | — | B | render/stroke | — |
| FP-AA | 抗锯齿 | Paint.isAntiAlias | 边缘质量 | — | SetAntiAlias | — | B | render/context.go | — |
| FP-SHADER | 着色器 | Paint.shader | 渐变/图着色 | FillLinear/Radial/Sweep | SetFillBrush 等 | — | **A**/B | draw.go | 图着色 ImageShader 仍 B |
| FP-BLEND | 混合模式 | Paint.blendMode | 叠加/遮罩 | — | SetBlendMode / PushLayer blend | — | B | context_layer.go | UI 未暴露 |
| FP-MASK-FILTER | 遮罩模糊 | Paint.maskFilter | 软阴影感 | — | ApplyBlur 等 | — | B | filter_ops.go | 非 Paint 绑定模型 |
| FP-COLOR-FILTER | 颜色滤镜 | Paint.colorFilter | 置灰禁用图标 | **ApplyGrayscale/ColorMatrix** | ApplyColorMatrix 等 | ColorFilterLayer | **A/B** | filter_draw.go · filter_ops | UI 全画布 apply；非 Paint 对象绑定 |
| FP-IMAGE-FILTER | 图像滤镜 | Paint.imageFilter | 模糊背景 | **ApplyBlur** | ApplyImageFilterGraph/Blur | ImageFilterLayer | **A/B** | filter_draw.go · filter_ops | UI 全画布 blur；非图滤镜图 DAG 产品 |
| FP-FILTER-QUALITY | 采样质量 | Paint.filterQuality | 缩放图锐利度 | — | DrawImageEx 选项部分 | — | C | context_image.go | 对齐不完全 |
| FP-INVERT | 反色 | Paint.invertColors | 无障碍高对比 | — | ApplyInvert 类 | — | B/D | filter_ops.go | 核对实现 |
| FS-LINEAR | 线性渐变 | Gradient.linear | AppBar/按钮 | FillLinearGradient | NewLinearGradientBrush | — | **A** | draw.go | 两 stop 便利 API |
| FS-RADIAL | 径向渐变 | Gradient.radial | 光晕 | FillRadialGradient | NewRadialGradientBrush | — | **A** | draw.go | 两 stop 便利 API |
| FS-SWEEP | 扫掠渐变 | Gradient.sweep | 圆锥/色轮 | FillSweepGradient | NewSweepGradientBrush | — | **A** | draw.go | 两 stop 便利 API |
| FS-IMAGE | 图像着色 | ImageShader | 图案填充 | — | CreateImagePattern/SetFillPattern | — | B | context_image.go | — |
| FS-FRAGMENT | 片元着色器 | FragmentShader | 自定义特效 | — | — | — | D | — | Impeller/SkSL 级 |

---

## §4 `dart:ui.Path` 几何构建母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|
| FPath-MOVE | MoveTo | Path.moveTo | 子路径起点 | — | Path.MoveTo / PathBuilder | — | B | render/path.go | — |
| FPath-LINE | LineTo | Path.lineTo | 折线 | — | LineTo | — | B | path.go | — |
| FPath-QUAD | 二次贝塞尔 | Path.quadraticBezierTo | 曲线 | — | QuadraticTo/QuadTo | — | B | path.go | — |
| FPath-CUBIC | 三次贝塞尔 | Path.cubicTo | 平滑曲线 | — | CubicTo | — | B | path.go | — |
| FPath-CONIC | 圆锥曲线 | Path.conicTo | 精确圆弧权重 | — | — | — | D | — | Flutter 有 conic |
| FPath-ARC-TO | 弧加入路径 | Path.arcTo/arcToPoint | 圆角路径 | — | Path.Arc / DrawArc | — | B | path · context | API 形态不完全同 |
| FPath-ADD-RECT | addRect | Path.addRect | — | — | Rectangle() | — | B | path.go | — |
| FPath-ADD-RRECT | addRRect | Path.addRRect | — | — | RoundedRectangle | — | B | path.go | — |
| FPath-ADD-OVAL | addOval | Path.addOval | — | — | Ellipse/Circle | — | B | path.go | — |
| FPath-ADD-PATH | addPath | Path.addPath | 组合图标 | — | Append | — | B | path.go | — |
| FPath-CLOSE | close | Path.close | 闭合填充 | — | Close | — | B | path.go | — |
| FPath-FILLTYPE | fillType | Path.fillType | evenOdd 镂空 | — | SetFillRule | — | B | render | UI 未暴露 |
| FPath-TRANSFORM | 路径变换 | Path.transform | 图标旋转缩放 | — | 矩阵作用于 path/CTM | — | B | — | — |
| FPath-METRICS | 路径度量 | Path.computeMetrics | 虚线动画/沿路径字 | ComputePathMetrics / PathPositionAt / PathTangentAt | ComputeMetrics · TotalLength · PositionAt · TangentAt | — | **A** | path_metrics.go · path_metrics_test | 每轮廓；Close 计入；conic 仍无；非全量 getSegment |
| FPath-COMBINE | 路径布尔 | Path.combine/op | 镂空合并 | — | Path.Op 布尔 | — | B | path_boolean.go | — |
| FPath-BOUNDS | 包围盒 | Path.getBounds | layout/damage | — | Bounds() | — | B | path.go | — |
| FPath-RESET | reset/clear | Path.reset | 复用 path | — | Clear/Reset | — | B | path.go | — |

---

## §5 Clip / save / saveLayer（补充母表）

> Canvas 级 clip/save 见 §2。本表强调 **UI 场景裁剪策略** 与 **ClipOp**。

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|
| FClip-SAVE-PAIR | save/restore 裁剪栈 | 与 Canvas save | 嵌套裁剪 | PushClip*/PopClip · **SaveLayer/Restore** | Push/Pop+Clip*+Layer | — | **A/B** | paint_context.go | clip+layer 对；通用任意 CTM save 仍 B |
| FClip-OP-INTERSECT | ClipOp.intersect | ClipOp | 默认相交 | — | ClipOpIntersect | — | B | clip_op.go | — |
| FClip-OP-DIFFERENCE | ClipOp.difference | ClipOp | 挖洞 | — | ClipOpDifference | — | B | clip_op.go | — |
| FClip-NEST-DEPTH | 深层嵌套 clip | 实现限制 | 复杂卡片 | 有限 | clip 栈+测试 | — | C | context_clip_depth_test.go | 需文档化上限 |

---

## §6 Transform / CTM / 变换层母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|
| FX-OFFSET-LAYER | 位移层 | OffsetLayer | 滚动、定位 | — | Translate | OffsetLayer **A** | A | ui/scene/layer.go | — |
| FX-TRANSFORM-LAYER | 变换层 | TransformLayer | 旋转/缩放 | RenderTransform | CTM B | **TransformLayer A/C** | A/C | scene · transform.go · transform_hit_test | **逆 CTM hit**（rotate+scale）；通用 pushTransform/Present 层合成仍开 |
| FX-OPACITY-LAYER | 透明度层 | OpacityLayer | 淡入淡出 | AnimatedOpacity 分类 | 层 opacity | OpacityLayer **A** | A/C | animation/implicit.go | 分类 compositor；Present 仍全画 |
| FX-CTM-FULL | 完整 CTM 栈 | Canvas transform 族 | 自定义绘制 | WithOrigin 仅平移语义 | 完整 CTM B | — | B | render | — |

---

## §7 图像 / Atlas / 纹理母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|
| FImg-DRAW | 绘 Image | drawImage | 位图 | **DrawImageBuf** | DrawImage | RenderImage | A | image_draw.go · image.go | — |
| FImg-RECT | 源矩形采样 | drawImageRect | 精灵/裁剪 | **DrawImageRect** | DrawImageEx | — | **A** | image_draw.go · image_draw_test | SrcRect 门面 |
| FImg-NINE | 九宫格 | drawImageNine | 气泡边框 | **DrawImageNine** | DrawImageNine | — | **A** | image_draw.go · image_draw_test · image 轴 | center 拉伸语义单测 |
| FImg-ROUND | 圆角图 | ClipRRect+image | 头像 | **DrawImageRounded** | DrawImageRounded | — | **A** | image_draw.go · image_draw_test | 均匀圆角；per-corner 仍开 |
| FImg-CIRCLE | 圆形图 | DecorationImage | 头像 | **DrawImageCircular** | DrawImageCircular | — | **A** | image_draw.go · image_draw_test | |
| FImg-QUAD | 四角映射 | 自定义 | 透视广告 | — | DrawImageQuad | — | B | m4_extensions.go | — |
| FImg-CODEC | 解码 | instantiateImageCodec | 加载图 | ui/io.Pool DecodeFile | ImageBuf | RenderImage 状态机 | A | ui/io/decode.go | F12；多帧 GIF 弱 |
| FImg-DISPOSE | 释放图像 | image.dispose | 防泄漏 | SetImage 所有权 | ImageBuf.Dispose | RenderImage | A | buf · image_dispose_test | 幂等 |
| FImg-TO-BYTES | 读回像素 | toByteData | 截图/测试 | — | ExportImageBuf/Image/SavePNG | — | B | context_image.go | — |
| FImg-GPU-TEX | 外部纹理 | Texture | 视频帧 | — | DrawGPUTexture* | 无 TextureLayer | B/D | context_image.go | FL-TEXTURE 关联 |
| FImg-ATLAS | 图集 | drawAtlas | 合批图标 | **DrawAtlas** | DrawAtlas | — | **A** | image_draw.go · image_draw_test · vertices.go | UI 门面；原点偏移 sprites |

---

## §8 文本 / Paragraph / TextPainter 母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|
| FT-PARAGRAPH-BUILD | 构建段落 | ParagraphBuilder | 富文本 | **ParagraphBuilder**+PushStyle | — | RenderText.Runs | **A/C** | paragraph.go · paragraph_style_test | 多 span+样式栈；无 locale/strut/placeholder |
| FT-PARAGRAPH-LAYOUT | 段落布局 | Paragraph.layout | 换行高度 | Face.Measure 当有 Face | MeasureString/Multiline | RenderText.Layout | **A/C** | text.go · text_font_ro_test | 有 Face→真测；无 Face 仍启发式；全量 Paragraph 仍 C |
| FT-PARAGRAPH-PAINT | 绘段落 | drawParagraph | 正文 | SetFont+DrawTextColored | DrawString* | RenderText.Paint | **A/C** | text.go · text_font_ro_test | 有 Face 时 DC.SetFont(size-synced)；非 drawParagraph 全量 |
| FT-PAINTER | TextPainter | TextPainter | 通用测量绘 | — | Measure+Draw 组合 | — | B | render/text.go | 可做 ui 门面 |
| FT-STYLE-SIZE | 字号 | TextStyle.fontSize | 层级 | **SetFontSize** · effectiveFace | Face/Source.Face(pts) | RenderText | **A** | text.go · text_font_ro_test | FontSize 与 Face 尺寸同步；改 size 重测 |
| FT-STYLE-FAMILY | 字体族 | fontFamily | 品牌/CJK | **SetFontFamily** / LoadFaceByFamily | LoadFontFace | RenderText | **A** | default_font.go · paragraph_style_test | sans/serif/mono + 路径；非完整 CSS 族匹配 |
| FT-STYLE-WEIGHT | 字重 | fontWeight | 强调 | — | Face/variations 部分 | — | B | LoadFontFaceWithVariations | — |
| FT-STYLE-STYLE | italic | fontStyle | 斜体 | — | — | — | C/D | — | 依赖字体文件 |
| FT-LETTER-SPACING | 字距 | letterSpacing | 标题微调 | — | — | — | D | — | — |
| FT-WORD-SPACING | 词距 | wordSpacing | 英文 | — | — | — | D | — | — |
| FT-HEIGHT | 行高 | height | 多行节奏 | *1.25 近似 | lineSpacing 参数 | — | C | DrawStringWrapped | — |
| FT-STRUT | StrutStyle | StrutStyle | 多行稳定行盒 | — | — | — | D | — | — |
| FT-ALIGN | 对齐 | TextAlign | 标题居中 | — | Align @ Wrapped | — | B | DrawStringWrapped | 单行 RO 无 |
| FT-DIRECTION | 文本方向 | TextDirection | RTL | — | BiDi shaping 路径 | — | B/C | render/text | 未 UI 文档化 |
| FT-MAXLINES | 最大行数 | maxLines | 列表副标题 | MaxLines/SetMaxLines | — | RenderText | A | text.go | 0=不限 |
| FT-OVERFLOW | 溢出省略 | TextOverflow.ellipsis | 长文案 | Overflow Ellipsis/Clip | — | RenderText | A | DisplayLines | “…” |
| FT-LOCALE | locale | locale | 断行/字体 | — | — | — | D | — | — |
| FT-DECORATION | 下划线等 | TextDecoration | 链接 | **SetDecoration** | SetTextDecoration | RenderText | **A/C** | text.go · render/text_decoration · paragraph_style_test | underline/lineThrough/overline；无 thickness 样式 |
| FT-SHADOW | 文字阴影 | shadows | 标题质感 | — | — | — | D | — | 滤镜近似 |
| FT-FG-BG | 前景/背景 Paint | foreground/background | 镂空字 | — | StrokeString / 底 rect | — | B | StrokeString | — |
| FT-BIDI | 双向文本 | BiDi | 阿语/混排 | — | shaper BiDi | — | B | render/text | — |
| FT-SEL-BOXES | 选区盒 | getBoxesForSelection | 输入选区 | — | — | — | D | — | 编辑器后置 |
| FT-POS-FOR-OFFSET | 点击定位 | getPositionForOffset | 光标 | — | — | — | D | — | IME 后置 |
| FT-LINE-METRICS | 行度量 | computeLineMetrics | 排版调试 | — | MeasureMultiline 部分 | — | B/C | render/text.go | — |
| FT-CJK | CJK 与回退 | 字体 fallback | 中日韩 | DrawText 可绘 | isCJKText/GPU/MultiFace | RenderText | C | render/text.go | 系统字体策略 |
| FT-WRAP | 自动换行 | softWrap | 段落 | — | WordWrap/DrawStringWrapped | — | B | render/text.go | RO 未用 |
| FT-SHAPED | 已整形 glyph | shaped glyphs | 性能/复杂文种 | — | DrawShapedGlyphs | — | B | render/text.go | — |
| FT-STROKE-TEXT | 描边字 | foreground stroke | 描边标题 | — | StrokeString* | — | B | render/text.go | — |
| FT-ANCHOR | 锚点绘制 | 对齐锚 | 居中标签 | — | DrawStringAnchored | — | B | render/text.go | — |
| FT-MEASURE | 测量宽高 | TextPainter.width/height | layout 真值 | **effectiveFace+Measure** | MeasureString | RenderText | **A/B** | text.go · text_font_ro_test | 有 Face→真测；无 Face 仍启发式 |

---

## §9 Picture / 录制回放母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|
| FPic-RECORDER | Picture 录制 | PictureRecorder+Canvas | 层缓存 | **PictureRecorder** | — | Picture.Ops | **A/C** | scene/picture.go · picture_test | rect+path+text+image；非全量 Canvas recorder（无 saveLayer/clip 进列表） |
| FPic-PLAYBACK | 回放 | drawPicture | 静态层复用 | **Picture.Replay** · RasterizeDirtyToContext | Context | NeedsRaster+Ops | **A/C** | picture.go · rasterize.go · picture_test | CPU 回放 path/text/image+脏跳过；非 GPU 纹理缓存；非 dirty Present |
| FPic-TO-IMAGE | 栅格化 | Picture.toImage | 截图 | — | Export/Image | — | B | context | — |
| FPic-DISPOSE | 释放 | dispose | 防泄漏 | — | — | — | C | — | 规范待补 |

---

## §10 阴影 / 滤镜 / Blend / Mask 母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|
| FF-SHADOW-PATH | 路径阴影 | Canvas.drawShadow | 卡片海拔 | — | ApplyDropShadow 近似 | — | B | filter_ops.go | — |
| FF-BLUR | 高斯模糊 | ImageFilter.blur | 毛玻璃 | **ApplyBlur** | ApplyBlur/ApplyBlurXY | **ImageFilterLayer** | **A/C** | filter_draw · scene/layer · filter_*_test | 场景层+UI apply；Backdrop 产品仍开；Present 全画 |
| FF-COLOR-MATRIX | 颜色矩阵 | ColorFilter.matrix | 置灰 | **ApplyGrayscale/ColorMatrix** | ApplyColorMatrix | **ColorFilterLayer** | **A/C** | filter_draw · scene · filter_*_test | 场景层+UI apply；Present 合成仍开 |
| FF-BLEND-LAYER | 混合合成 | saveLayer+blend | 叠色 | — | SetBlendMode/PushLayer | — | B | context_layer.go | — |
| FF-MASK | 遮罩层 | ShaderMask/saveLayer mask | 渐变淡出 | — | PushMaskLayer | — | B | context_layer.go | — |
| FF-BACKDROP | 背景滤镜 | BackdropFilter | 毛玻璃导航 | **PushBackdrop/PopBackdrop** | PushBackdropLayer | **BackdropFilterLayer** | **A/C** | filter_draw · scene · composite_test | 全幅快照+blur；非 clip 局部产品 |
| FF-BUDGET | saveLayer 预算 | 工程纪律 | 防滥用 | **SaveLayerBudget+Allow** | — | — | **A** | paint_context · save_layer_test | MaxOps/MaxArea 门禁 |

---

## §11 `PaintingContext` 母表（Flutter rendering）

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|
| FPC-CANVAS | 取 Canvas | PaintingContext.canvas | 底层绘制 | DC *render.Context | Context | — | A | rendering/paint_context.go | — |
| FPC-PAINT-CHILD | 绘子节点 | paintChild | 树遍历 | 子 Paint+WithOrigin | — | Box/Absolute 遍历 | A | box.go | — |
| FPC-CLIP-RECT | pushClipRect | PaintingContext.pushClipRect | 视口 | PushClipRect | ClipRect | ClipRectLayer | A | paint_context.go | 层接线有限 |
| FPC-CLIP-RRECT | pushClipRRect | pushClipRRect | 圆角裁子树 | PushClipRRect · RenderClipRRect | ClipRoundRect | ClipRRectLayer RO 接线 | **A** | clip_rrect.go · layer_build · clip_rrect_layer_test | 均匀圆角；Present 仍全画 |
| FPC-CLIP-PATH | pushClipPath | pushClipPath | 异形 | — | Clip path | — | B/D | — | — |
| FPC-COLOR-FILTER | pushColorFilter | pushColorFilter | 子树滤镜 | Apply* 全画布 | 滤镜 API | ColorFilterLayer Builder | **C** | scene/build · filter_draw | 层类型有；RO push 子树隔离仍 C |
| FPC-OPACITY | pushOpacity | pushOpacity | 子树透明 | — | Opacity 层/CTM | OpacityLayer | A/C | scene | Present 全画 |
| FPC-TRANSFORM | pushTransform | pushTransform | 子树变换 | **PushTransform** · RenderTransform | CTM | TransformLayer | **A/C** | paint_context · transform · layer_build | 2D PushTransform；非 Matrix4 |
| FPC-LAYER | pushLayer/addLayer | pushLayer | 自定义层 | — | — | LayerBuilder 有限 | C | scene/build.go | — |
| FPC-COMPLEX-HINT | setIsComplexHint | setIsComplexHint | 光栅缓存提示 | — | — | — | D | — | 光栅缓存（序13） |
| FPC-WILL-CHANGE | setWillChangeHint | setWillChangeHint | 动画提示 | — | — | — | D | — | 序13 |
| FPC-COMPOSITE-ONLY | retained 跳过 | 实现细节 | 脏区局部 paint | CompositeOnly+PaintVisits | — | FlushPaint(false) | **A** | pipeline.go · present_damage_test | 稳态 force=false；首帧/resize full |

---

## §12 Layer 类型全量母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|
| FL-CONTAINER | 容器层 | ContainerLayer | 树节点 | — | — | ContainerLayer | A | scene/layer.go | — |
| FL-OFFSET | 位移层 | OffsetLayer | 滚动 | — | — | OffsetLayer | A | layer.go | — |
| FL-TRANSFORM | 变换层 | TransformLayer | 旋转缩放合成 | RenderTransform | CTM | **TransformLayer** | **A/C** | transform · **composite.go** · composite_test | 逆 hit + **CompositeToContext walk**；PipelineApp 仍 RO paint |
| FL-OPACITY | 透明层 | OpacityLayer | 淡入 | MutSetOpacity | PushLayer | OpacityLayer | **A/C** | compositing · **composite.go** | CompositeToContext PushLayer；PipelineApp 未默走层路径 |
| FL-CLIP-RECT | 裁剪层 | ClipRectLayer | 溢出 | — | — | ClipRectLayer | A | layer.go | Build 使用有限 |
| FL-CLIP-RRECT | 圆角裁剪层 | ClipRRectLayer | 卡片 | **RenderClipRRect** | — | **ClipRRectLayer** | **A/C** | clip_rrect · layer_build · **composite_test** | RO→层 + **Composite walk**；PipelineApp 仍 RO paint |
| FL-CLIP-PATH | 路径裁剪层 | ClipPathLayer | 异形 | — | — | **无** | D | — | — |
| FL-PICTURE | 图层层 | PictureLayer | 录制内容 | Record/SetPicture | Replay | **PictureLayer+Ops** | **A/C** | picture · **composite.go** · composite_test | Composite Replay；PipelineApp 仍 RO 至 Record 接线 |
| FL-TEXTURE | 纹理层 | TextureLayer | 视频 | — | DrawGPUTexture | **无** | D | — | — |
| FL-PLATFORM-VIEW | 平台视图 | PlatformViewLayer | WebView/地图 | — | — | **无** | D | — | — |
| FL-SHADER-MASK | 着色遮罩 | ShaderMaskLayer | 渐变淡出列表 | — | Mask 近似 | **无** | D | — | — |
| FL-BACKDROP | 背景滤镜层 | BackdropFilterLayer | 毛玻璃 | PushBackdrop | PushBackdropLayer | **BackdropFilterLayer** | **A/C** | layer.go · build · composite · layer_test | Composite walk；全幅快照 |
| FL-COLOR-FILTER | 颜色滤镜层 | ColorFilterLayer | 子树置灰 | ApplyGrayscale | ApplyColorMatrix | **ColorFilterLayer** | **A/C** | layer · **composite.go** | Composite 子树 PushLayer+Apply；PipelineApp 未默走 |
| FL-IMAGE-FILTER | 图像滤镜层 | ImageFilterLayer | 模糊子树 | ApplyBlur | ApplyBlur | **ImageFilterLayer** | **A/C** | layer · **composite.go** | Composite 子树 blur；非 Backdrop |
| FL-LEADER | LeaderLayer | LeaderLayer | 跟随定位锚点 | — | — | **无** | D | — | Overlay 高级 |
| FL-FOLLOWER | FollowerLayer | FollowerLayer | Tooltip 跟随 | — | — | **无** | D | — | — |
| FL-ANNOTATED | 注解层 | AnnotatedRegionLayer | 语义/系统 UI | semantics 薄 | — | — | C | ui/semantics | 非 Layer 实现 |
| FL-PERF | 性能叠加层 | PerformanceOverlayLayer | HUD | — | — | **无** | D | — | 序13 HUD |
| FL-BOUNDARY | 重绘边界 | RepaintBoundary→层 | 脏区隔离 | SetRepaintBoundary | — | BoundaryLayer | A | node.go · layer.go | — |
| FL-OVERLAY-BAND | 覆盖带 | Overlay | 弹层 | ui/overlay | — | FramePacket.Overlay | A | overlay · scene | F13 |

---

## §13 RenderObject / Pipeline 脏区母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|
| FRO-MARK-LAYOUT | 标记需布局 | markNeedsLayout | 内容/约束变 | MarkNeedsLayout | — | Base | A | node.go | — |
| FRO-MARK-PAINT | 标记需绘制 | markNeedsPaint | 外观变 | MarkNeedsPaint | — | Base | A | node.go | — |
| FRO-MARK-COMP | 合成位更新 | markNeedsCompositingBitsUpdate | 层结构变 | compositing 传播 | — | scene compositing | A/C | compositing.go | 深度弱于 Flutter |
| FRO-RELAYOUT-B | RelayoutBoundary | relayoutBoundary | 截断 layout 冒泡 | SetRelayoutBoundary | — | Viewport 默认 | A | node.go · viewport.go | — |
| FRO-REPAINT-B | RepaintBoundary | isRepaintBoundary | 截断 paint 冒泡 | SetRepaintBoundary | — | BoundaryLayer | A | node.go | — |
| FRO-SIZED-BY-PARENT | 父决定尺寸 | sizedByParent | 紧约束子 | Fixed 模式部分 | — | Box/ColorBox | C | box.go | 无完整标志模型 |
| FRO-CONSTRAINTS | 约束 | BoxConstraints | layout 输入 | Constraints | — | pipeline | A | constraints.go | — |
| FRO-LAYOUT | performLayout | performLayout | 测布局 | Layout() | — | 各 RO | A | rendering/* | — |
| FRO-PAINT | paint | paint | 录制/绘制 | Paint(pc) | 经 DC | — | A | — | — |
| FRO-HIT | 命中测试 | hitTest* | 指针 | HitTest | — | 各 RO | A | — | — |
| FRO-LAYER-FIELD | 层句柄 | layer / updateCompositedLayer | 合成对象 | BuildLayerTree | — | layer_build.go | C | — | 非每节点持久 Layer 句柄 |
| FRO-ATTACH | attach/detach | attach/detach | 元素挂载 | AddChild 等 | — | Base | C | node.go | 生命周期弱于 Element 树 |
| FRO-SEMANTICS | 语义脏 | markNeedsSemanticsUpdate | 无障碍 | ui/semantics 骨架 | — | — | C | semantics | 非完整 |
| FRO-PIPELINE | PipelineOwner | flushLayout/Paint/Compositing | 帧驱动 | PipelineOwner | — | embedder | A | pipeline.go | flushCompositing 弱 |
| FRO-BUILD-OWNER | BuildOwner | 构建脏元件 | Widget→Element | 无 Widget 层 | — | — | D | — | 命令式 RO 模型 |

---

## §14 调度 · Vsync · 帧管道母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|
| FSch-VSYNC | 垂直同步 | SchedulerBinding/vsync | 稳帧 | VSyncWaiter + DRM | — | Host 可选 | A/C | vsync_drm_linux · exhost | 无 DRM→fallback；禁锁 60Hz |
| FSch-TRANSIENT | 短暂帧回调 | scheduleFrameCallback | 动画 | Mode TRANSIENT | — | FrameScheduler | A | scheduler.go | — |
| FSch-PERSISTENT | 每帧回调 | persistent frame callbacks | 连续动画 | Mode PERSISTENT + Ticker | — | — | A | scheduler.go | — |
| FSch-IDLE | 空闲阻塞 | 无事不转 | 省电 | Mode IDLE + WaitEvents | — | — | A | scheduler.go | S0 |
| FSch-WARMUP | 热身帧 | warm-up frame | 减首帧 jank | PipelineOptions.WarmUp | — | — | A | pipeline_app.go | F11 |
| FSch-TIMINGS | FrameTiming | FrameTiming | 分轨耗时 | frame_build_ms/raster_ms | — | MetricsStore | C | metrics.go | 多为 last；缺直方图 |
| FSch-TIMINGS-CB | timings callback | addTimingsCallback | 自定义采集 | — | — | — | D | — | 可扩展 Metrics |
| FSch-TICKER | Ticker | Ticker/AnimationController | 动画驱动 | animation.Controller+TickerRegistry | — | — | A | animation · ticker.go | F17 |
| FPerf-OVERLAY | 性能 HUD | PerformanceOverlay | 开发态 | — | — | — | D | — | 序13 |
| FPerf-DEVTOOLS | CPU/内存工具 | DevTools | 优化 | 无内建 | render mem harness | 示例 JSON 部分 | C/D | mem_harness_test.go | ui 需接入 |

---

## §15 滚动 / 视口 / 虚拟化母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|
| FScroll-OFFSET | 滚动偏移 | Viewport offset | 列表 | SetScrollOffset/ScrollBy | — | Viewport+Offset | A | viewport.go | 默认只 MarkNeedsPaint |
| FScroll-CLIP | 视口裁剪 | clip canvas | 离屏不画 | PushClip 于 Viewport.Paint | — | — | A | viewport.go | — |
| FScroll-VIRTUAL | 虚拟化 | SliverChild | 长列表 | VirtualList 固定行高 | — | — | A | virtual_list.go | 固定行高 A；可变高见 FScroll-VAR-EXTENT |
| FScroll-CACHE | cacheExtent | cacheExtent | 预创建窗外 | CacheExtent | — | VirtualList | A | virtual_list.go | — |
| FScroll-NEST | 嵌套滚动 | NestedScroll/竞技 | 父子列表 | Scrollable.Parent 移交 | — | gestures+scrollable | A | nested_scroll_test.go | — |
| FScroll-VAR-EXTENT | 可变行高 | Sliver 可变 | 聊天列表 | ItemExtentAt · **OffsetOfIndex/IndexAtOffset/ScrollToIndex** | — | VirtualList | **A/C** | virtual_list.go · virtual_list_scroll_test | 前缀和 index↔offset + 跳转；**非**完整 multi-sliver；Physics 仍 D |
| FScroll-PHYSICS | 物理回弹 | ScrollPhysics | iOS/Android 感 | **ClampingScrollPhysics** · Fling/TickPhysics | — | Viewport | **A/C** | scroll_physics.go · scroll_physics_test | clamp+摩擦 fling；**无** bounce/glow |

---

## §16 坐标 · DPR · HitTest 母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|
| FCoord-LOGICAL | 逻辑像素 | logical pixels / dp | 布局点击 | 全程逻辑 px | DeviceScale | Host.Size | A | 架构文 §4 | F07 |
| FCoord-PHYSICAL | 物理像素 | device pixels | 纹理 | — | PixelWidth/Height | Swapchain | A | render | — |
| FCoord-DPR | devicePixelRatio | MediaQuery.devicePixelRatio | HiDPI | Context.Scale | WithDeviceScale | Host.ScaleFactor | A | — | — |
| FCoord-YDOWN | Y 向下 | Flutter 布局 | 一致 | Y-down | 2D UI 惯例 | — | A | — | — |
| FCoord-HIT | HitTest | HitTestResult | 指针 | HitTest | — | 各 RO + Overlay 先 | A | embedder | — |

---


## §17 代码落点（简）

| 入口 | 路径 |
|------|------|
| 绘制游标 | `ui/rendering.PaintContext`（`DC *render.Context`） |
| 画布实现 | `render.Context` |
| RO / Pipeline | `ui/rendering` |
| 层树 | `ui/scene` |
| 调度 / 指标 | `ui/scheduler` |
| 窗测 | `examples/ui_render_base_*`、`examples/ui_l1_scroll` |

依赖：`ui → render → gpu`（禁止 `ui→gpu`、禁止 CGO）。

## §18 Present 诚实条款

| 项 | 状态 |
|----|------|
| `render.PresentFrameDamage*` | B（render 有） |
| UI `PipelineApp` 稳态 force=false + PresentFrameAuto | **A/C** | 首帧/resize/warm-up 仍 full |
| 真 dirty-rect Present（UI 稳态） | **A/C** PresentWithAuto + force=false + damage 指标；首帧/resize full |
| Picture 显示列表 record/replay | **A/C（序13 主项）** 见 FPic-* |

细能力状态以 §2–§16 为准，不在此重复 RO/Layer 清单。

---

## §20 正向性能与资源指标

> **每实现一项能力就采**（§0.6 第 2 项）。定义全表如下；合入时取**相关** M-\* + 通用帧/CPU/RSS，相对 baseline 不无故回退。  
> **自定义控件真窗硬门禁（FPS/CPU/RSS + 全族必采）：** [`ENGINE_UI_WIDGET_RENDER.md`](./ENGINE_UI_WIDGET_RENDER.md) **§0 U12–U14 · §2.2**（对齐本 §20.0 族 A–J）。

### 20.0 指标族

| 族 | 回答什么 | 典型 ID / 字段 |
|----|----------|----------------|
| **A 帧时** | 是否稳、卡在哪 | interval p50/p95/p99、hitch、hitch_rate、vsync_source、FPS |
| **B 管线** | 是否堵在 Present/排队 | build/raster ms、pipeline depth、输入延迟 |
| **C 脏区** | 成本是否 ∝ 脏 | layout/paint count、raster/skip layer、bind_count；damage（序13） |
| **D CPU** | 谁吃 CPU | 进程 CPU%；ui/raster 路径 proxy |
| **E 内存** | 是否漏、关是否放 | RSS start/end/peak/slope、after_close |
| **F GPU** | 提交/回退 | submit/batch/fallback（需上浮到 UI JSON） |
| **G 图/文资源** | 缓存是否命中 | atlas hit、解码是否离 UI |
| **H 启动** | 首帧 | warm-up、至首 Present |
| **I 回归** | 是否变差 | 同机同场景 baseline delta |
| **J 正确性** | 是否假绿 | ui↛gpu、无 cgo、禁止降画质装绿 |

### 20.1 接线现状（摘要）

| 族 | 现状 |
|----|------|
| A | p50/p95/p99、hitch_rate、vsync_source **已接** |
| B | build/raster **last**；分位仍缺 |
| C | layout/paint/raster_layer **已接**；**damage_area_px 已接** |
| D | 进程 CPU + **ui/raster 路径 proxy 已接** |
| E | RSS 起止峰/after_close **已接**；**rss_slope_kb_per_min 自动字段 ✅** |
| F | **gpu_ops / cpu_fallback_ops / last_cpu_fallback 进 UI JSON ✅**（PipelineApp Present 后采样）；atlas/VRAM 仍开 |
| G/H | 部分在 render/示例；首帧统一字段仍开 |
| I | **CompareToBaseline + Load/SaveBaseline ✅**；CI 机器基线仍手配 |
| J | depcheck + 门禁测 |

### 20.2 指标 ID 全表

> 状态随实现更新；已 A 的不得标成「无」。

#### A. 帧时与流畅

| ID | 指标 | 对照 | 来源现状 | 单位 | 状态 | 正向门禁方向 |
|----|------|------|----------|------|------|--------------|
| M-FRAME | frame_count | 帧计数 | MetricsStore | count | A | 随 schedule 增 |
| M-PRESENT-SUBMIT | 提交 present | — | PipelineApp.PresentCount | count | A | 与 complete 区分解读 |
| M-PRESENT-COMPLETE | 完成 present | — | NotePresent JSON present_count | count | A | ≤ submit |
| M-PRESENT-COALESCE | 合并/丢弃次数 | 背压 | **无独立字段** | count | D | 可解释 |
| M-INTERVAL-LAST | 上次帧间隔 | FrameTiming | metrics | ms | A | ~16.7 @60 |
| M-INTERVAL-AVG | 平均间隔 | — | metrics | ms | A | ~16.7 |
| M-INTERVAL-MAX | 最大间隔 | — | metrics | ms | A | 观察尖峰 |
| M-INTERVAL-P50 | 分位 p50 | DevTools | 环形 256 | ms | **A** | ≈16.7 |
| M-INTERVAL-P95 | 分位 p95 | — | 环形 256 | ms | **A** | 预算 |
| M-INTERVAL-P99 | 分位 p99 | — | 环形 256 | ms | **A** | 轻场景 < hitch 阈 |
| M-FPS-WALL | wall fps | — | presents/elapsed 示例 | Hz | A | 动画 ≥55 |
| M-FPS-COMPLETE | 完成帧 fps | — | complete/elapsed | Hz | C | 诚实 FPS |
| M-TARGET-HZ | 目标刷新 | 60/120 | 文档 + DefaultAnimTick | Hz | C | JSON 标明 |
| M-HITCH | hitch_count | jank>≈2frame | >33.4ms | count | A | soak 低 |
| M-HITCH-RATE | hitches/min | SLO | hitch_count÷墙钟分钟 | /min | **A** | soak 低 |
| M-JANK-33 | >33.4ms | =hitch | hitch | count | A | — |
| M-JANK-50 | >50ms | 严重卡顿 | **无** | count | D | 趋 0 |
| M-MISSED-VSYNC | missed_vsync | vsync miss | Wait 错误时 | count | C | 真 vsync 后 |
| M-VSYNC-SOURCE | true / fallback | — | JSON vsync_source | enum | **A** | 诚实 |

#### B. 管线与跟手

| ID | 指标 | 对照 | 来源现状 | 单位 | 状态 | 正向门禁方向 |
|----|------|------|----------|------|------|--------------|
| M-BUILD-MS | frame_build_ms | UI build | NoteBuildMs | ms | A | 控制 p99 |
| M-RASTER-MS | frame_raster_ms | raster | NoteRasterMs | ms | A | 控制 p99 |
| M-BUILD-P99 | build p99 | — | **无** | ms | D | 预算 |
| M-RASTER-P99 | raster p99 | — | **无** | ms | D | 预算 |
| M-PIPE-DEPTH | pipeline_depth | 在途 | SetPipeline | int | A | ≤2 健康 |
| M-PIPE-MAX | pipeline_max | 配置/峰值 | 常为配置 2 | int | C | 记观测峰值 |
| M-QUEUE-WAIT | 提交前等待 | 排队 | **无** | ms | D | 低 |
| M-INPUT-LAG | 输入到状态 | ≤1 帧 | 测/注入 | frame | C | 慢 Present 仍跟手 |

#### C. 脏区与局部性

| ID | 指标 | 对照 | 来源现状 | 单位 | 状态 | 正向门禁方向 |
|----|------|------|----------|------|------|--------------|
| M-LAYOUT-COUNT | layout_count | layout 范围 | PipelineApp 接线 | count | **A** | S2 动画期不涨 |
| M-PAINT-COUNT | paint_count | paint flush | PipelineApp 接线 | count | **A** | 可解释 |
| M-PAINT-VISITS | PaintVisits | retained walk | 测试 Context | count | A 测 | ≪ 全树 |
| M-RASTER-LAYER | raster_layer_count | 脏层 | SetRasterLayerCount | count | A | S2 小 |
| M-SKIP-LAYER | skipped layers | 静态复用统计 | RasterStats | count | A 内部 | S4 >0 |
| M-COMPOSITE-LAYER | composite_layer_count | 合成-only | 占位 | count | C | opacity 动画 |
| M-BIND | virtual bind | sliver children | VirtualList.BindCount | count | A | ≪ itemCount |
| M-DAMAGE-AREA | 脏像素面积 | partial present | **damage_area_px** | px | **A/C** | present_damage_test；随脏降 |
| M-UPLOAD-BYTES | 上传字节/帧 | — | **无** | B | D | 随脏降 |

#### D. CPU

| ID | 指标 | 对照 | 来源现状 | 单位 | 状态 | 正向门禁方向 |
|----|------|------|----------|------|------|--------------|
| M-CPU-PROCESS | 进程 CPU% | DevTools | ProcessTracker cpu_pct_avg | % | **A** 示例 | S0 低；动画有上界 |
| M-CPU-PROCESS-P95 | 进程 CPU p95 | — | **无** | % | D | 尖峰 |
| M-CPU-UI | UI 路径 %（proxy） | — | build 份额 cpu_ui_pct | % | **C**/A | 非 OS 线程 |
| M-CPU-RASTER | Raster 路径 %（proxy） | — | raster 份额 cpu_raster_pct | % | **C**/A | 非 OS 线程 |
| M-CPU-IDLE | 空闲 CPU | — | 派生/采样 | % | D | S0 ≈0 |

#### E. 内存与释放

| ID | 指标 | 对照 | 来源现状 | 单位 | 状态 | 正向门禁方向 |
|----|------|------|----------|------|------|--------------|
| M-RSS-START | 起始 VmRSS | 内存 | ProcessTracker | KB | **A** | — |
| M-RSS-END | 结束 VmRSS | — | ProcessTracker | KB | **A** | — |
| M-RSS-PEAK | 峰值 RSS | — | ProcessTracker | KB | **A** | 有解释 |
| M-RSS-SLOPE | 斜率 KB/min | 泄漏 | **rss_slope_kb_per_min** · ProcessTracker.Apply | KB/min | **A** | soak ≈0；非 Linux stub=0 |
| M-RSS-AFTER-CLOSE | 关闭后 RSS | 释放 | ProcessTracker | KB | **A** 观察 | 可解释下降 |
| M-HEAP | HeapAlloc | runtime | 未进 UI JSON | B | C | soak |
| M-NUM-GC | GC 次数 | — | **无** | count | D | 稳定 |
| M-GOROUTINES | 协程数 | 泄漏 | **无** | count | D | 稳定 |
| M-VRAM | 显存/驱动池 | — | **无** | MB | D | 不爬升 |

#### F. GPU / 提交 / 回退

| ID | 指标 | 对照 | 来源现状 | 单位 | 状态 | 正向门禁方向 |
|----|------|------|----------|------|------|--------------|
| M-GPU-SUBMIT | 提交/GPU 路径 ops | batching | **gpu_ops** ← RenderPathStats（累计） | count | **A/C** | 累计非每帧；勿碎提交 |
| M-GPU-FALLBACK | CPU 回退原因 | — | **last_cpu_fallback** | string | **A** | 可诊断 |
| M-GPU-FALLBACK-N | 回退次数 | — | **cpu_fallback_ops** | count | **A** | 热路径趋 0 |
| M-LAYER-POOL | 层池命中 | saveLayer | LayerPoolStats | — | B | 命中率 |
| M-TEX-CREATE | 纹理创建次数 | — | **无** | count | D | soak 稳定 |

#### G. 文本 / 图像资源

| ID | 指标 | 对照 | 来源现状 | 单位 | 状态 | 正向门禁方向 |
|----|------|------|----------|------|------|--------------|
| M-ATLAS-HIT | 字形 atlas 命中 | 文本热路径 | render 内部 | ratio | B | 高命中 |
| M-ATLAS-MISS | atlas 未命中 | — | 内部 | count | B | 可控 |
| M-SHAPE-CACHE | shape 缓存 | — | 内部 | — | B | — |
| M-IMG-DECODE-MS | 解码耗时（worker） | F12 | ui/io | ms | C | 不进 UI 线程 |
| M-IMG-UPLOAD-N | 图上传次数 | — | **无** | count | D | 滚动不爆 |

#### H. 启动与首帧

| ID | 指标 | 对照 | 来源现状 | 单位 | 状态 | 正向门禁方向 |
|----|------|------|----------|------|------|--------------|
| M-TIME-TO-FIRST-PRESENT | 至首 Present | 冷启动 | **无统一字段** | ms | D | 预算 |
| M-WARMUP | warm-up 已跑 | F11 | PipelineOptions.WarmUp | bool | A | 默认开 |
| M-FIRST-ANIM-FRAME | 热身后首动画帧 | — | **无** | ms | D | 无巨大尖峰 |

#### I. 回归元数据（每份 JSON 建议带）

| ID | 指标 | 说明 | 状态 |
|----|------|------|------|
| M-META-SCENARIO | scenario | 如 geometry / spinner / s2 | C 示例名 |
| M-META-BACKEND | backend | x11/wayland | A stderr |
| M-META-SCALE | dpr | Host.ScaleFactor | C |
| M-META-MACHINE | 机器/GPU 备注 | 文档/手记 | C |
| M-BASELINE-DELTA | 相对 baseline | **CompareToBaseline** · Load/SaveBaseline · perfsoak `BASELINE_JSON` | **A/C** |

#### J. 正确性相邻（与性能并列，防止假绿）

| ID | 检查 | 状态 |
|----|------|------|
| M-DEP-NO-GPU | ui 无 import gpu | A depcheck |
| M-DEP-NO-CGO | 无 import C | A 纪律 |
| M-GATE-S2/S4/S5 | 既有单测门禁 | A |
| M-NO-QUALITY-CHEAT | 禁止降画质装门禁 | 流程 |

### 20.3 采集注意

- 帧分位 / hitch_rate：`MetricsStore`；RSS/CPU：示例 `ProcessTracker`
- `vsync_source=fallback` 时 **禁止** 宣称锁显示 60Hz
- 无同机同场景 baseline 的「优化」无效
- 真窗仍全量 Present 时，**不可**用 Present 路径证明 dirty-rect（序13）

---

## §21 场景门禁（单测常用）

| 场景 | 关键指标 | 通过方向 |
|------|----------|----------|
| S0 空闲 | CPU idle | ≈0 |
| S2 Spinner | layout、raster_layer、fps | 动画期 layout≈0 |
| S4 静+动 | skip_layer | 静态可 skip |
| S5 虚拟列表 | bind | ≪ itemCount |
| S6 拖拽滚 | layout | 无风暴 |
| Soak | hitch_rate、RSS slope | 长跑不烂 |
| Close | rss_after_close | 可释放 |

---

## §22 依赖序与施工表（真源）

> 下面每一行 = 一批可一起推进的能力（对应 §2–§16 的 ID）。  
> **前驱未完成不要跳。** 同层可并行。细行状态以各 § 表为准。  
> **完成定义：** 该行相关母表 ID 达目标状态 **且** §0.6 三项验收全过。

### 22.0 依赖（谁先谁后）

```text
① 坐标/Hit
② Pipeline/脏区/RO  ── ③ PaintContext ──┬─ ⑤ 几何+Paint/Shader ─ ⑥ Path
④ 调度/VSync        ─────────────────────┤
                                         ├─ ⑦ Clip/saveLayer ─ ⑧ Transform
                                         ├─ ⑨ 图像
                                         ├─ ⑩ 文本
                                         ├─ ⑪ 滤镜/阴影层
                                         └─ ⑫ 滚动/视口
指标采集（§20）∥ 每一项都做，不是最后再做
⑬ Picture/局部 Present ← 基座主路径之后、有数据再加深（仍属渲染基座）
```

### 22.1 施工表（顺序 · 前驱 · 三项验收）

| 序 | 能力（母表） | 前驱 | 状态 | ① 单测 | ② 指标（§20） | ③ 窗测 |
|----|--------------|------|------|--------|---------------|--------|
| 1 | 坐标/DPR/Hit §16 | — | ✅ | Hit/embedder | 不破坏 S2/S4 | — |
| 2 | Pipeline/脏区/RO §13 | 1 | ✅/🔄 | `TestS2/S4/M*` | layout/paint/raster/skip | — / matrix |
| 3 | PaintContext 基础 §11 | 2 | ✅ | RO/PaintContext | 同 2；PaintVisits | — |
| 4 | 调度/VSync §14 | 2 | ✅/🔄 | scheduler/vsync | interval·hitch·vsync_source·cpu 分轨 | PerfSoak |
| 5 | 几何+Paint/Shader §2§3 | 3 | ✅ 主路径 | `TestDraw_*` | geometry/PerfSoak 帧指标 | **geometry** |
| 6 | Path §4 | 5 | ✅ 主路径 | `TestPathMetrics_*` · FillPath | geometry 帧指标 | **geometry** |
| 7 | Clip/saveLayer §5 | 3,6 | ✅/🔄 | ClipRRect RO→层；clip_rrect_test | raster/skip；无层泄漏 | **cliplayer** |
| 8 | Transform 层 §6 | 3,7 | ✅/🔄 | 逆 CTM hit；TransformLayer | 同 7；旋转 hitch 可解释 | **cliplayer** |
| 9 | 图像 §7 | 3,7 | ✅/🔄 | Nine/Round UI；dispose | RSS/Dispose；解码不堵 UI | **image** |
| 10 | 文本/Paragraph §8 | 3,5 | ✅/🔄 | Font→RO；maxLines/ellipsis | 文本帧时；(有则) atlas | **text** |
| 11 | 滤镜/阴影层 §10§12 | 7,8 | ✅/🔄 | Color/ImageFilter 层+Apply | CPU/RSS；saveLayer 预算 | **cliplayer** |
| 12 | 滚动/视口 §15 | 3,7 | ✅/🔄 | index↔offset；ScrollToIndex；S5 | **bind** 上界；layout 不风暴 | **scroll** |
| 13 | Picture/局部 Present §9§18.3 | 2,8,11 | ✅/🔄 | 显示列表 + **dirty Present 稳态** | damage_area_px；禁全清冒充 | 专用/PerfSoak |
| — | 指标骨架本身 §20 | — | ✅/🔄 | slope·GPU JSON·baseline 对比 | 全表逐步接线；p99/VRAM 仍开 | **perfsoak** |

**不在本表、不挡渲染基座收口：** Ant 控件、完整 IME 编辑器、PlatformView 产品态等（母表可留 D 行，标远期）。

### 22.2 收口后残项（非阻塞 · 产品驱动再开）

> §22 主路径已收口。下表是 **加深/产品** 残项，**不** 表示基座「还没做完主项」。

| 序 | 主项 | 残项（摘要） | 优先级建议 |
|----|------|--------------|------------|
| 5 | ✅ | ImageShader、不等圆角 RRect、真 Gouraud | 场景需要再开 |
| 6 | ✅ | conic；Path 动词 UI 文档化 | 低 |
| 7 | ✅ | ClipPath **场景层**；任意 CTM save 产品化 | 中（异形 UI） |
| 8 | ✅ | 透视 Matrix4；**PipelineApp 默走层 Present** | 中（合成架构） |
| 9 | ✅ | per-corner 圆角图；GPU atlas 产品化 | 低–中 |
| 10 | ✅ | strut/locale/选区/placeholder；全量 TextStyle | 编辑器后置 |
| 11 | ✅ | clip 局部毛玻璃；层 Present 默走 | 中（质感） |
| 12 | ✅ | multi-sliver；BouncingPhysics | 产品滚动手感 |
| 13 | ✅ | GPU picture 缓存；列表内 clip/saveLayer；层树 Present 合成 | 中（性能结构） |
| 指标 | ✅ D8 | build/raster p99、VRAM、atlas 命中、CI 基线库 | 可观测加深 |

### 22.3 已落地（勿回退）

| 能力 | 证据 | 诚实边界 |
|------|------|----------|
| PaintContext + DC | `paint_context.go` | 无独立 painting 包 |
| **几何 draw UI（序5/D2）** | `draw.go` · `draw_test` · Points/Vertices/DRRect/PathAdd | ImageShader/不等圆角仍开；Vertices CPU 均色 |
| **Path metrics（序6）** | `render/path_metrics.go` · `ui/rendering/path_metrics*` | conic 仍 D；非全量 PathMetric.getSegment |
| ClipRRect **paint + 层** | `PushClipRRect` · **RenderClipRRect** · `clip_rrect_layer_test` · cliplayer | **FL-CLIP-RRECT A/C**（RO→Build 主路径）；Present 全画；ClipPath/saveLayer 仍开 |
| TransformLayer + RenderTransform + **逆 CTM hit** | transform.go · transform_hit_test · layer_build · cliplayer | 中心 rotate+scale 逆 hit；**非** Matrix4/通用 pushTransform；Present 全画 |
| Text maxLines/ellipsis；最小多 span | text · paragraph | ≠ 全量 Paragraph |
| **Font 接通 RenderText（序10）** | `SetFace` · `SetFontSize` · `effectiveFace` · `text_font_ro_test` | 有 Face 真测+DC.SetFont |
| **Paragraph 加深（序10/D3）** | `LoadFaceByFamily` · `SetFontFamily` · `SetDecoration` · `PushStyle/PopStyle` · paragraph_style_test | 非 strut/locale/IME；非全量 Flutter Paragraph |
| **Filter 场景层+UI Apply（序11）** | `ColorFilterLayer` · `ImageFilterLayer` · `ApplyGrayscale/Blur` · filter_*_test | Backdrop/Present 合成/saveLayer 隔离仍开 |
| Image Dispose 所有权 | image_dispose_test | — |
| **Image Nine/Round/Circular/SrcRect/Atlas UI（序9/D1）** | `image_draw.go` · `image_draw_test` · image 轴 | per-corner 仍开 |
| 真 VSync（DRM）+ vsync_source | platform · exhost · WaitFramePace | 无 DRM→fallback；禁锁 60Hz |
| p95、hitch_rate、cpu_ui/raster **proxy** | MetricsStore | proxy≠OS 线程 CPU |
| **指标 D8（§20）** | `rss_slope_kb_per_min` · `gpu_ops`/`cpu_fallback_ops` · `CompareToBaseline` · baseline_test · perfsoak | 非 VRAM/atlas；baseline 容差松；GPU ops 为 Context 累计 |
| VirtualList 可变高前缀和 + **index↔offset/ScrollToIndex** | virtual_list · virtual_list_scroll_test · viewport.ScrollToIndex | **A/C**；完整 multi-sliver 仍开 |
| **ScrollPhysics clamp+fling（序12/D4）** | `ClampingScrollPhysics` · `Viewport.Fling/TickPhysics` · Scrollable pan end · scroll_physics_test | 无 bounce/glow；需 App 每帧 TickPhysics |
| **Picture 显示列表（序13/D7）** | `PictureRecorder` Fill/StrokeRect+Path · DrawString · DrawImage · `Replay` · picture_test | path clone + image 引用 + text Face；非全量 Canvas recorder；非 GPU picture 缓存 |
| **dirty-rect Present 稳态（序13）** | `PresentWithAuto` · `PaintPresentTree` · `present_damage_test` · `damage_area_px` | 首帧/resize full clear；**W0：稳态全树 paint**（防 GPU Clear 丢静态）；CompositeOnly 仅测试 API；真 retained 见 WIDGET_RENDER |
| **层 Composite walk（序8/11/13）** | `scene.CompositeToContext` · `composite_test` · Offset/Clip/Transform/Picture/Opacity/Filter | 单测像素；**PipelineApp 仍 RO paint**（待 Record 接线） |
| **SaveLayer + ClipPath paint（序7）** | `SaveLayer/Restore` · `PushClipPath` · `PushLayerIsolated` · save_layer_test | 预算门禁；全幅隔离；无 ClipPathLayer |
| **DropShadow + Backdrop（序11/D5）** | `ApplyDropShadow` · `PushBackdrop` · `BackdropFilterLayer` · filter_draw_test · composite_test | 全幅 alpha 阴影；全幅 backdrop 快照 |
| **2D CTM PushTransform（序8/D6）** | `Save`/`PushTransform`/`Concat`/`ScaleXY`/`Rotate`/`Shear` · transform_ctm_test | 非透视 Matrix4 |
| 窗测轴 | `examples/ui_render_base_*` | — |

---

## §23 窗口测试程序

路径：`examples/ui_render_base_*`（禁止放进 `ui/`）。与 §22「③ 窗测」列对应。

| 程序 | 施工序 | 作用 |
|------|--------|------|
| `ui_render_base_geometry` | 5–6 | 几何/Path + 指标 JSON |
| `ui_render_base_text` | 10 | 文本 + JSON |
| `ui_render_base_image` | 9 | 图/异步/RSS |
| `ui_render_base_cliplayer` | 7–8 | Clip/Transform |
| `ui_l1_scroll` | 12 | 滚动/虚拟列表 |
| `ui_render_base_perfsoak` | 每项建议 | §20 回归 |

---

## §25 基座收口终审（阶段 E · v1.29）

> **日期：** 2026-07-28  
> **范围：** 仅 UI **渲染基座**（§22 序 1–13 + §20 指标主骨架）。不含 Ant、完整 IME、PlatformView、SkSL。  
> **抽查：** `go test ./ui/...` 绿（终审当日）。

### 25.1 结论（可对外说的话）

| 可宣称 | 不可宣称 |
|--------|----------|
| §22 **主路径已收口**：画 / 排 / 合（能力）/ 滚 / 调度 / dirty Present / Picture 子集 / 指标主字段可测 | **已对齐 Flutter 全量渲染基座** |
| L1 retained-paint 契约可用（Boundary / CompositeOnly / DirtyLayerIDs） | Picture / Canvas / Paragraph **与 Flutter 对等** |
| 稳态 dirty-rect Present **有证据路径**（force=false + Auto + damage_area_px） | 无 DRM 时 **锁显示 60Hz** |
| 轻量 2D UI **可以**做到流畅（场景/环境依赖） | **已达原生全场景丝滑** / 无 baseline 的「更顺」 |
| 指标：p50/p95/p99、hitch、RSS slope、GPU path、baseline 对比 **骨架可用** | 指标全表 M-\* 已 A；VRAM/atlas/首帧统一字段已完备 |

### 25.2 §22 主路径判定

| 序 | 判定 | 三项验收（摘要） |
|----|------|------------------|
| 1–4 | **收口** | Hit/Pipeline/PaintContext/VSync 单测 + 指标 +（4）PerfSoak |
| 5–6 | **收口（主路径）** | draw/path 单测 + geometry 轴；ImageShader/conic 等残 |
| 7–8 | **收口（主路径）** | ClipRRect/SaveLayer/CTM/Composite 单测 + cliplayer；场景 ClipPath 层 / 默走层 Present 残 |
| 9–11 | **收口（主路径）** | image/text/filter 单测 + 对应窗测；编辑器/局部毛玻璃残 |
| 12 | **收口（主路径）** | VirtualList+Physics 单测 + scroll 轴；bounce/multi-sliver 残 |
| 13 | **收口（主路径）** | Picture D7 + dirty Present + damage；GPU picture 缓存 / 默走 Composite 残 |
| 指标 | **收口（D8）** | slope + GPU JSON + baseline 骨架；p99/VRAM/CI 库残 |

### 25.3 母表状态分布（约，终审快照）

| 码 | 约数 | 读法 |
|----|------|------|
| A / A/C / A/B | ~140+ | 主路径与大量 UI 可测项 |
| B | ~50 | render 有、UI 未暴露或未接场景 |
| C | ~25 | 半成品 / 内部有未公开 |
| D | ~37 | **故意远期** 或 Flutter 对照缺失（非施工失败） |

**D 的主要桶（勿当欠债清零）：**  
PlatformView / Texture 视频层 / Leader-Follower / BuildOwner / FragmentShader / 编辑选区与 IME 定位 / conic / 部分指标字段（jank50、VRAM、GC…）/ PerformanceOverlay。

### 25.4 架构诚实点（最易误读）

```text
1. PipelineApp 默认仍 RO 直绘 Present（PaintPresentTree）
   → CompositeToContext / Picture 能力在，但是「可选/测试路径」，非默认 compositor。
2. Picture 显示列表 = rect+path+text+image 子集；无列表内 clip/saveLayer；无 GPU 纹理缓存。
3. Backdrop / DropShadow / 部分 SaveLayer = 全幅或大区域成本，产品慎用。
4. VSync：有 DRM 可为 true；否则 fallback — JSON vsync_source 为准。
5. cpu_ui/raster = 路径 proxy，≠ OS 线程 DevTools %。
6. gpu_ops = Present Context 累计路由计数，≠ 严格「每帧 GPU submit 次数」。
```

### 25.5 建议的下一步（收口之后）

| 优先级 | 方向 | 说明 |
|--------|------|------|
| **1 产品** | 真实 App / 控件场景 | 用场景驱动开残项，禁止无目标清 D |
| **2 可选架构** | Record→Composite 双路径（flag） | 要层缓存/静态跳过再做；不默认替换 |
| **3 可观测** | 同机 baseline 入库 + soak 门禁收紧 | D8 骨架已有；CI 机器绑定 |
| **4 质感** | 局部 backdrop、bounce 滚动 | 纯产品体验 |

### 25.6 终审签字栏（工程）

| 项 | 状态 |
|----|------|
| §22 主路径 + D1–D8 | **收口** |
| 母表诚实（A/C 边界、禁止宣称） | **已回写本 §** |
| `go test ./ui/...` | **绿（终审抽查）** |
| 全表 B/C/D 清零 | **不做**（非目标） |
| 「对齐 Flutter 渲染」对外口径 | **仅主路径子集 + 本 § 边界** |

---

## §24 修订

| 版本 | 说明 |
|------|------|
| **1.29** | **阶段 E 终审**：§25 基座收口声明；§22.2 改为收口后残项；文首闭环口径更新；抽查 `go test ./ui/...`；链出 **ENGINE_UI_WIDGET_RENDER**（控件工业级渲染设计） |
| 1.28 | **阶段 D8** 指标：rss_slope_kb_per_min；gpu_ops/cpu_fallback 进 UI JSON；CompareToBaseline+Load/Save；perfsoak BASELINE_JSON |
| 1.27 | **阶段 D7** Picture 显示列表扩展：Fill/StrokePath · DrawString · DrawImage + picture_test 像素回放；非 GPU picture 缓存 |
| 1.26 | **阶段 D6** PaintContext Save/Concat/PushTransform/Translate/ScaleXY/Rotate/Shear/GetTransform + transform_ctm_test |
| 1.25 | **阶段 D5** ApplyDropShadow + PushBackdrop/BackdropFilterLayer + composite walk + tests |
| 1.24 | **阶段 D4** ClampingScrollPhysics + Viewport Fling/TickPhysics + Scrollable pan-end fling + scroll_physics_test |
| 1.23 | **阶段 D3** LoadFaceByFamily/SetFontFamily、TextDecoration、ParagraphBuilder PushStyle + paragraph_style_test |
| 1.22 | **阶段 D2** DrawPoints/DrawVertices/FillDRRect/PathAdd*/SetFillRule + draw_test |
| 1.21 | **阶段 D1** DrawImageCircular/DrawImageRect/DrawAtlas UI 门面 + image_draw_test + image 轴接线 |
| 1.20 | **阶段 C** SaveLayer/Restore+Budget + PushLayerIsolated + PushClipPath + save_layer_test；ClipPath 场景层仍开 |
| 1.19 | **阶段 B** CompositeToContext：Offset/Clip/Transform/Picture/Opacity/Filter 层 walk + composite_test；PipelineApp 仍 RO paint |
| 1.18 | **序13** dirty-rect Present 稳态：PresentWithAuto + force=false CompositeOnly + damage_area_px 指标 + present_damage_test；首帧/resize full |
| 1.17 | **序13** Picture 显示列表：PictureRecorder Fill/StrokeRect + Replay + RasterizeDirtyToContext；dirty-rect Present 仍开 |
| 1.16 | **序12** VirtualList OffsetOfIndex/IndexAtOffset/ScrollToIndex（前缀和）+ virtual_list_scroll_test；Physics/完整 multi-sliver 仍开 |
| 1.15 | **序11** ColorFilterLayer/ImageFilterLayer + LayerBuilder；UI ApplyGrayscale/Blur/ColorMatrix + 像素单测；Backdrop/Present 合成仍开 |
| 1.14 | **序10** Font 接通 RenderText：SetFontSize + faceForSize/effectiveFace 测绘同路径 + text_font_ro_test；全量 Paragraph 仍开 |
| 1.13 | **序9** DrawImageRounded/DrawImageNine/DrawImageBuf UI 门面 + image_draw_test + image 轴接线；Atlas 仍开 |
| 1.12 | **序8** RenderTransform 逆 CTM hit（中心 rotate+scale）+ transform_hit_test；通用 pushTransform/Present 合成仍开 |
| 1.11 | **序7** ClipRRectLayer RO→BuildLayerTree：RenderClipRRect + layer_build + 单测 + cliplayer；FL-CLIP-RRECT→A/C；ClipPath/saveLayer 仍开 |
| 1.10 | **序6** FPath-METRICS：ComputeMetrics/PositionAt/TangentAt + UI 门面 + 单测 |
| 1.9 | 核查已落地；FL-CLIP-RRECT→C；**序5** draw.go 几何主路径 |
| 1.8 | 流程闭环/实现未闭环；删 Wave/分册叙事 |
| ≤1.7 | 母表三项验收；分册已删 |
