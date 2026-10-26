# UI 渲染基座 — Flutter 母表 × gpui

> **版本：1.31** | 日期：2026-09-13  
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
| Layer | `ui/scene` | 常用层 A/C；Filter D；Texture A/C（C4 已接视频，待三窗证据） |
| Scheduler / Vsync | `ui/scheduler` | 接口 A；DRM 真 VSync A/C |
| 帧/资源指标 | `MetricsStore` + 示例 JSON | 分位/hitch/RSS/**slope**/GPU path/baseline **A/C（D8）** |
| dirty-rect Present | render + **PipelineApp 稳态 Auto** | **A/C（序13）** 首帧/resize 仍 full；OS damage 可忽略 |
| Picture 显示列表 | scene PictureRecorder/Replay | **A/C（序13/D7）** rect/path/circle/oval/arc/roundrect/line/gradient/text/image；≠ Flutter 全量；≠ partial Present |

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
| FC-CLIP-BOUNDS-LOCAL | 本地裁剪界 | getLocalClipBounds | 优化/命中 | — | **ClipBounds()**（反 CTM 回本地） | — | **B** | context_misc_draw.go · internal/clip/stack.go · ui_render_base_miscdraw | render 1:1；路径裁剪界按路径 AABB（不再报全画布）；UI 未暴露 |
| FC-CLIP-BOUNDS-DEST | 设备裁剪界 | getDestinationClipBounds | damage 对齐 | — | — | — | D | — | 可与序13 damage 联动 |
| FC-DRAW-COLOR | 整幅着色 | Canvas.drawColor | 清屏/遮罩 | 清色在 Present 路径 | **DrawColor(col, mode)** / Clear/ClearWithColor | — | **B** | context_misc_draw.go · ui_render_base_miscdraw | render 1:1 填当前 clip；高级混合走隔离层；UI 仍用清屏 |
| FC-DRAW-PAINT | 用 Paint 填当前 clip | Canvas.drawPaint | 全 clip 填着色器 | — | **DrawPaint()** / SetFillBrush+大 rect Fill | — | **B** | context_misc_draw.go · ui_render_base_miscdraw | render 1:1，保留当前路径；UI 未暴露 |
| FC-DITHER | 抖动后处理 | SkPaint.setDither（Flutter 无公开对等） | 低色深渐变去带 | —（无 UI 门面） | **SetDither/Dither** | — | **A** | m4_dither.go · dither_contract_test.go · ui_render_base_dither | 读回路径 pixmap Bayer 后处理；真窗 2026-10-20 关闭（16 逻辑 + 29 像素 + golden + 抓屏）；整张 pixmap / 重复读回累加 / 不影响 Present 主路三限界 |
| FC-DRAW-RECT | 矩形 | Canvas.drawRect | 色块/背景 | FillRect / StrokeRect | DrawRectangle+Fill/Stroke | ColorBox | **A** | rendering/draw.go | fill+stroke；无持久 Paint 对象 |
| FC-DRAW-RRECT | 圆角矩形 | Canvas.drawRRect | 按钮/卡片 | FillRoundRect / StrokeRoundRect | DrawRoundedRectangle* | — | **A** | draw.go · geometry 轴 | 均匀圆角；XY 不等圆角仍 B |
| FC-DRAW-DRRECT | 双 RRect 环 | Canvas.drawDRRect | 环形进度/边框环 | **FillDRRect** | EvenOdd 双 RRect path | — | **A/C** | draw.go · draw_test | 均匀圆角 even-odd 近似；非 per-corner |
| FC-DRAW-OVAL | 椭圆 | Canvas.drawOval | 头像底/高亮 | FillOval / StrokeOval | DrawEllipse | — | **A** | draw.go | — |
| FC-DRAW-CIRCLE | 圆 | Canvas.drawCircle | 头像/点/涟漪 | FillCircle / StrokeCircle | DrawCircle | Spinner 仍可用 rect 点 | **A** | draw.go | — |
| FC-DRAW-ARC | 弧/扇形 | Canvas.drawArc | 进度环 | FillArc / StrokeArc | DrawArc/DrawEllipticalArc | — | **A** | draw.go | 椭圆弧 API 仍可加深 |
| FC-DRAW-PATH | 任意路径 | Canvas.drawPath | 图标/波浪/自定义 | NewPath+**PathAdd***+FillPath · **SetFillRule** | Path/Fill | — | **A** | draw.go | PathAddRect/RRect/Oval；fillType UI |
| FC-DRAW-LINE | 线段 | Canvas.drawLine | 分割线/刻度 | StrokeLine | DrawLine | — | **A** | draw.go | — |
| FC-DRAW-POINTS | 点列 | Canvas.drawPoints/RawPoints | sparklines | **DrawPoints** | DrawPoint | — | **A** | draw.go · draw_test | 圆点半径；非 PointMode 枚举全量 |
| FC-DRAW-VERTICES | 顶点网格 | Canvas.drawVertices | 扭曲图/网格渐变 | **DrawVertices(Ex)/DrawMesh(Ex)** | DrawVertices | — | **A/C** | vertices_mesh.go · vertices_mesh_contract_test.go · vertices_mesh_gpu_test.go · ui_render_base_verticesmesh | Triangles/Fan/indexed；GPU 逐点色，CPU 真 Gouraud；真窗 2026-10-20 关闭（19 逻辑 + 29 像素 + golden + 抓屏）；逐点色忽略 Paint alpha/blend |
| FC-DRAW-ATLAS | 图集精灵 | Canvas.drawAtlas | 图标合批、粒子 | **DrawAtlas** | DrawAtlas | — | **A** | image_draw.go · vertices.go | UI 门面 |
| FC-DRAW-SHADOW | 路径阴影 | Canvas.drawShadow | Material 海拔阴影 | **ApplyDropShadow** | **DrawShadow(path,col,elevation,...)** / ApplyDropShadow | — | **A/C** | context_misc_draw.go · filter_ops.go · ui_render_base_miscdraw | render 按 Impeller spot 模型做 path-bound elevation；UI 仍用全幅 alpha 阴影；transparentOccluder 暂忽略 |
| FC-DRAW-IMAGE | 绘图像 | Canvas.drawImage | 图标/位图 | **DrawImageBuf** | DrawImage | RenderImage | A | image_draw.go · RenderImage | 公开 UI 门面 |
| FC-DRAW-IMAGE-RECT | 源/目标矩形 | Canvas.drawImageRect | 裁剪缩放 | **DrawImageRect** / DrawImageBuf | DrawImageEx | RenderImage | **A** | image_draw.go | |
| FC-DRAW-IMAGE-NINE | 九宫格 | Canvas.drawImageNine | 可拉伸边框 | **DrawImageNine** | DrawImageNine | — | **A** | image_draw.go · image_draw_test | 均匀 center 矩形；Atlas UI 仍开 |
| FC-DRAW-PARAGRAPH | 绘段落 | Canvas.drawParagraph | 所有正式文本 | DrawText/Colored 子集 | **DrawParagraph(*text.Layout,x,y)** / DrawString* | RenderText | **B/C** | context_misc_draw.go · text/layout.go · ui_render_base_miscdraw | render 可按布局逐行/run 画（MultiFace 已分段）；UI 仍非 Paragraph 模型；单色 |
| FC-DRAW-PICTURE | 回放 Picture | Canvas.drawPicture | 层缓存回放 | Picture.Replay | Context 绘 | PictureLayer+Ops | **A/C** | scene/picture.go · picture_test · picture_geo_test | rect/path/circle/oval/arc/roundrect/line/gradient/text/image 回放；**非** dirty-rect Present；非 GPU picture 缓存 |

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
| FP-IMAGE-FILTER | 图像滤镜 | Paint.imageFilter | 模糊背景 | **ApplyBlur** | ApplyImageFilterGraph/Blur | ImageFilterLayer | **A/B** | filter_draw.go · filter_ops · filter_gpu_graph.go · ui_render_base_filterops | UI 全画布 blur；图滤镜 DAG 由 render 直调真窗原子验证（2026-10-20），UI 侧仍非产品 |
| FP-FILTER-QUALITY | 采样质量 | Paint.filterQuality | 缩放图锐利度 | — | DrawImageEx 选项部分 | — | C | context_image.go | 对齐不完全 |
| FP-INVERT | 反色 | Paint.invertColors | 无障碍高对比 | — | ApplyInvert 类 | — | B/D | filter_ops.go · filter_gpu_graph.go · ui_render_base_filterops | 实现核对完成：GPU 反色按 `alpha−premul` 修正；2026-10-20 真窗原子验证；UI 未暴露 |
| FS-LINEAR | 线性渐变 | Gradient.linear | AppBar/按钮 | FillLinearGradient | NewLinearGradientBrush | — | **A** | draw.go | 两 stop 便利 API |
| FS-RADIAL | 径向渐变 | Gradient.radial | 光晕 | FillRadialGradient | NewRadialGradientBrush | — | **A** | draw.go | 两 stop 便利 API |
| FS-SWEEP | 扫掠渐变 | Gradient.sweep | 圆锥/色轮 | FillSweepGradient | NewSweepGradientBrush | — | **A** | draw.go | 两 stop 便利 API |
| FS-IMAGE | 图像着色 | ImageShader | 图案填充 | — | NewImagePattern/SetFillPattern（2026-10-23 S2b C2b：原 `Context.CreateImagePattern` 改包级函数，图样要能进显示列表） | — | B | context_image.go | — |
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
| FPath-COMBINE | 路径布尔 | Path.combine/op | 镂空合并 | — | Path.Op 布尔 | — | B | path_boolean.go | Clipper 扫描线（2026-10-19 重写）：精确几何 / 2 轮廓 10 verbs / 无截断；2026-10-20 真窗 `ui_render_base_pathbool`（Union/Intersect/Difference/Xor + NonZero·EvenOdd + 远坐标，逻辑 10/10、像素 18/18、Golden 627200px 零差、抓屏 0.0000%）；同批修半透明 stencil path 重复预乘。输出为折线（输入曲线按 0.1px 展平） |
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
| FImg-WEBP | WebP 解码 | Image codec（WebP） | 位图/贴图 | — | **LoadWebP/LoadImage** | RenderImage | A | context_image_buf.go · internal/image/io.go · webp_contract_test.go · ui_render_base_webp | `golang.org/x/image/webp` 静态 VP8/VP8L；无损对 PNG 四通道零差（含 Alpha）；有损跨解码器非逐位；动画未覆盖；真窗 2026-10-20 关闭（19 逻辑 + 24 像素 + golden + 抓屏） |
| FImg-DISPOSE | 释放图像 | image.dispose | 防泄漏 | SetImage 所有权 | ImageBuf.Dispose | RenderImage | A | buf · image_dispose_test | 幂等 |
| FImg-TO-BYTES | 读回像素 | toByteData | 截图/测试 | — | ExportImageBuf/Image/SavePNG | — | B | context_image.go | — |
| FImg-GPU-TEX | 外部纹理 | Texture | 视频帧 | — | DrawGPUTexture* | **TextureLayer**（C4） | B/C | context_image.go · scene/layer.go · composite.go | FL-TEXTURE 已接视频；三窗已实跑（非 -race）retained 像素绿；4K/VR8 p95 22.99/22.19ms 超 22、-race 被 #18 挡、无 Golden |
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
| FT-STYLE-FAMILY | 字体族 | fontFamily | 品牌/CJK | **SetFontFamily** / LoadFaceByFamily | LoadFontFace | RenderText | **A** | default_font.go · paragraph_style_test · render/text/family_test.go | sans/serif/mono + 路径；`SetFontFamily` 仍按单 face 路径解析，静态字重族由 `FontFamily` 单独提供（见 FT-STYLE-WEIGHT） |
| FT-STYLE-WEIGHT | 字重 | fontWeight | 强调 | **SetFontWeight** · DeriveFace | WithWeight/FaceEmbolden/FontRef/FontFamily/NewFontFamily/LoadFontFamily/EmboldenOutline/GlyphMaskFlagBold | RenderText | **A/C** | render/text/family_test.go · embolden_test.go · weight_test.go · ui/rendering/weight_test.go | 6b：FontFamily 按 CSS 权重匹配真 Regular/SemiBold/Bold 切（name ID 16 组族）；可变 wght 走真轮廓；无粗体兄弟的独立静态字体才走 `ppem/24` 轮廓兜底（advance 不变），不再栅格后 mask 膨胀；模板/图集/边界三处分键 |
| FT-STYLE-STYLE | italic | fontStyle | 斜体 | — | — | — | C/D | — | 依赖字体文件 |
| FT-LETTER-SPACING | 字距 | letterSpacing | 标题微调 | SetLetterSpacing · BuildTextLayoutExWithSpacing | ApplySpacing/LayoutText/WrapTextWithSpacing（数学委托第三方 `shaping.AddSpacing`） | RenderText | **A/C** | spacing_test.go · ui/rendering/spacing_test.go · ui_render_base_textspacing | 簇间隙n-1/末簇无拖尾/负值收紧/RTL不断形；富文本Runs随后（6c）；真窗 2026-10-20 关闭（16 逻辑 + 37 像素 + golden + 抓屏） |
| FT-WORD-SPACING | 词距 | wordSpacing | 英文 | SetWordSpacing · 同上 | 同上（第三方 CSS 分隔符集，tab 不计） | RenderText | **A/C** | spacing_test.go · ui/rendering/spacing_test.go | 同上；本轮 A10 只开字距真窗，词距未单独建窗 |
| FT-HEIGHT | 行高 | height | 多行节奏 | **lineSpacing 参数** | 可视块高度 = `(n-1)*lineHeight*lineSpacing + ascent + descent` | DrawStringWrapped | B/C | render/text.go · ui_render_base_textlow | 绘制与 `MeasureMultilineString` 同口径（2026-10-20 统一） |
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
| FT-LINE-METRICS | 行度量 | computeLineMetrics | 排版调试 | **MeasureMultilineString** | MeasureMultiline 部分 | — | B/C | render/text.go · ui_render_base_textlow | 多行可视块高度与绘制同口径（2026-10-20） |
| FT-CJK | CJK 与回退 | 字体 fallback | 中日韩 | DrawText 可绘 | isCJKText/GPU/MultiFace | RenderText | C | render/text.go | 系统字体策略 |
| FT-WRAP | 自动换行 | softWrap | 段落 | **WordWrap** | WordWrap/DrawStringWrapped | — | B | render/text.go · text_entry.go · ui_render_base_textlow | 行尾分隔空白裁剪、CRLF/CR 保留原始偏移；RO 未用 |
| FT-SHAPED | 已整形 glyph | shaped glyphs | 性能/复杂文种 | — | DrawShapedGlyphs | — | B | render/text.go | — |
| FT-STROKE-TEXT | 描边字 | foreground stroke | 描边标题 | — | StrokeString* | — | B | render/text.go | — |
| FT-ANCHOR | 锚点绘制 | 对齐锚 | 居中标签 | — | DrawStringAnchored | — | B | render/text.go | — |
| FT-MEASURE | 测量宽高 | TextPainter.width/height | layout 真值 | **effectiveFace+Measure** | MeasureString | RenderText | **A/B** | text.go · text_font_ro_test | 有 Face→真测；无 Face 仍启发式。零字距仍走未整形 `Face.Advance`（kerning/RTL 与 shaped layout 有差，见 `RENDER_FEATURE_COVERAGE` §6 独立子项）；字距≠0 已走 shaped builder |

---

## §9 Picture / 录制回放母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|
| FPic-RECORDER | Picture 录制 | PictureRecorder+Canvas | 层缓存 | **PictureRecorder** | — | Picture.Ops | **A/C** | scene/picture.go · picture_test · picture_geo_test | rect/path/circle/oval/arc/roundrect/line/gradient/text/image；非全量 Canvas recorder（无 transform/clip/saveLayer/points/vertices/atlas 进列表） |
| FPic-PLAYBACK | 回放 | drawPicture | 静态层复用 | **Picture.Replay** · RasterizeDirtyToContext | Context | NeedsRaster+Ops | **A/C** | picture.go · rasterize.go · picture_test | CPU 回放 path/text/image+脏跳过；非 GPU 纹理缓存；非 dirty Present |
| FPic-TO-IMAGE | 栅格化 | Picture.toImage | 截图 | — | Export/Image | — | B | context | — |
| FPic-DISPOSE | 释放 | dispose | 防泄漏 | — | — | — | C | — | 规范待补 |

---

## §10 阴影 / 滤镜 / Blend / Mask 母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|
| FF-SHADOW-PATH | 路径阴影 | Canvas.drawShadow | 卡片海拔 | — | **DrawShadow**（spot 偏移+模糊）/ ApplyDropShadow 近似 | — | **B** | context_misc_draw.go · filter_ops.go · ui_render_base_miscdraw | render 已有 path-bound elevation（2026-10-20 真窗）；UI 未接线 |
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
| FPC-LAYER | pushLayer/addLayer | pushLayer | 自定义层 | `BuildFramePacketWithSaveLayer(Stats)` | — | LayerBuilder 有限 | C | scene/build.go | `Stats` 版顺手出操作数（`BuildPacket` 收尾走包树收脏图层时加，E2 已修 2026-09-15；包结构不动，生产消费者 `ui/embedder/pipeline_app.go:1149`） |
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
| FL-TEXTURE | 纹理层 | TextureLayer | 视频 | — | DrawGPUTexture* | **TextureLayer** | **A/C** | scene/layer.go · composite.go · texture_layer_test · examples/video/video_* | C4（2026-10-24）：id 化、永不缓存、每帧重画；三窗已实跑（非 -race）retained 像素绿；4K/VR8 p95 22.99/22.19ms 超 22、-race 被 #18 挡、无 Golden；V-3 已修（单测） |
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
| FRO-LAYER-FIELD | 层句柄 | layer / updateCompositedLayer | 合成对象 | BuildLayerTree(+Stats) | — | layer_build.go | C | — | 非每节点持久 Layer 句柄；`BuildLayerTreeStats`/`FrameBuildStats` 一次建包顺手出边界数·深度·字命中（E2 已修 2026-09-15，生产消费者 `ui/embedder/pipeline_app.go:1149`，实测见 §22.2 序 2） |
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
> **自定义控件真窗硬门禁（FPS/CPU/RSS + 全族必采）：** [`ENGINE_UI_WIDGET_RENDER.md`](./ENGINE_UI_WIDGET_RENDER.md) **§0 U12–U16 · §2.2 · §2.5**（对齐本 §20.0 族 A–J）。  
> **真窗几何与时长：** 客户区 **1200×800**；**`RUN_SECONDS ≥ 5`**；按主能力加长（Boundary 8–10s、动画 30s、列表/压力 60s、soak 300s 等，见 WIDGET_RENDER §2.5 全表）。

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
| E | RSS 起止峰/after_close **已接**；**rss_slope_kb_per_min 原始 ✅；rss_slope_steady_kb_per_min / rss_slope_valid / rss_warmup_sec 已接** |
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
| M-RSS-SLOPE | 原始斜率 KB/min | 趋势 | **rss_slope_kb_per_min** · ProcessTracker.Apply | KB/min | **A** | 短跑含启动爬坡，不作泄漏结论；非 Linux stub=0 |
| M-RSS-SLOPE-STEADY | 稳态斜率 KB/min | 泄漏 | **rss_slope_steady_kb_per_min** · ProcessTracker.RSSSlopeSteadyKBPerMin | KB/min | **A** | 预热 max(25%,15s)，≥30s 才有效；60s soak ≈0 |
| M-RSS-WARMUP | 稳态预热秒数 | 口径 | **rss_warmup_sec** · SetRSSSlopeSteady | s | **A** | 短跑也报，解释 valid=false |
| M-RSS-VALID | 稳态斜率是否可判漏 | 诚实 | **rss_slope_valid** · SetRSSSlopeSteady | bool | **A** | false=跑太短/无 RSS，不能判漏 |
| M-RSS-AFTER-CLOSE | 关闭后 RSS | 释放 | ProcessTracker | KB | **A** 观察 | 可解释下降 |
| M-HEAP | HeapAlloc | runtime | 未进 UI JSON | B | C | soak |
| M-NUM-GC | GC 次数 | — | **无** | count | D | 稳定 |
| M-GOROUTINES | 协程数 | 泄漏 | **无** | count | D | 稳定 |
| M-VRAM | 显存/驱动池 | — | **无** | MB | D | 不爬升 |

> **RSS 判漏口径（2026-10-21 复核）：** `rss_slope_kb_per_min` 是原始趋势，5s/15s 窗包含设备初始化、字体/图片缓存和 Go 堆启动爬坡；**判漏只看 `rss_slope_steady_kb_per_min` 且 `rss_slope_valid=true`**。稳态预热取 `max(25% elapsed, 15s)`，再加至少 15s 采样，所以最小可判漏时长是 30s，推荐 60s soak。`rss_slope_valid=false` 只表示“不能下结论”，不是“没漏”。

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
| 2/13 | ✅ | E1 层树全量重建（2026-09-15 证伪、撤销不修，同 B1）：每帧整树重建是认账的现状（对齐 Flutter：搭树便宜、脏层重录 + 干净贴图；本引擎送屏模型要求每帧重现全部可见内容，B1 跳过干净层已黑屏撤回）。稀疏包修法（干净层扔出包）致静按钮丢失，已退回作废；`e-isolation-baseline` 分支与 `TestEVerify_E1` 残留待清 | —（不再排期；CPU 走 E2 + 使用层） |
| 2 | ✅ | E2 一帧多遍整树 walk（已修，2026-09-15 实测）：`BuildFramePacketWithSaveLayerStats` 一次建包顺手出边界数/深度（`appendNode` 记）+ 操作数（`BuildPacket` 收尾本来就要走包树收脏图层时加）+ 字命中（建包走到 `RenderText` 顺手读，与 `TreeMeasureCacheStats` 同一数同加锁）；真帧不再调 `CountRepaintBoundaries`/`CountPictureOps`/`TreeMeasureCacheStats`（老函数留着给别处）。`ConsumeNeedsPaint` 留原行原位。120 节点只脏 1 叶：老序列 `Children` 603 次 → 新序列 361 次（根 2 + 叶 359）；数诚实（边界 120/深 1、操作数、字命中与老单独数逐位等，`TestE2Merged_CountsHonestAndPacketEqual`）；包相等（同树同脏态新老层序列与脏号顺序全等） | 高（E2 已闭环；E1 已撤销，CPU 剩使用层） |
| 14 | ✅ | **几何管线性能结构（动画帧预算 · F1/F2 已交付 2026-09-13）** | — |
| T（立项，未开工） | ⬜ | **三线重设计（唯一文档 `ENGINE_THREAD_T_PLAN.md`，此处只留指针）** | 高 |
| 指标 | ✅ D8 | build/raster p99、VRAM、atlas 命中、CI 基线库 | 可观测加深 |

### 22.2a 几何管线性能结构（动画帧预算 · 序 14）

> **立项：** 2026-09-13。鹈鹕真窗（`examples/ui_render_pelican`）偶发卡顿（此前已存在）：
> 同二进制同机对照，机器安静 30s 卡顿 0 次（维持 ~59fps、p99 < 20ms），
> 日常桌面负载 4–46 次。垃圾回收（最大停顿 < 0.3ms）、CPU 变频、
> VSync 源（JSON `vsync_source=true`）均已排除；帧内分段计时（`HITCH_DIAG=1`）
> 定锤：**每次卡顿的 33–75ms 全部花在光栅线程的提交函数里，
> 界面线程构建耗时恒 < 1ms**。病根在**提交路径和几何管线内部**，
> **不是** 某个图形功能（天空/云/山/鸟/腿/轮）的绘制问题：
> 同一提交路径服务所有真窗，任何每帧重画的矢量场景都会踩同一条路，
> 鹈鹕只是因为每帧全屏动画把它暴露得最明显。
> 画像最大三块：purego 调 wgpu 原生库过门费约四成、四几何缓存命中不了仍交
> 全表扫描淘汰税、顶点/uniform 逐次上传。
> 本库开发中无用户，**不保留双路/开关，直接重写**。
>
> **公开接口：** 两期均不动公开 API（内部实现替换）。**跨平台：** 纯软件侧改动，
> 不新增系统调用，X11/Wayland/Win/mac 一次生效；像素以离屏逐字节对比为准。

| 期 | 目标 | 改动落点 | 状态 | ① 单测 | ② 指标（§20） | ③ 窗测 |
|----|------|----------|------|--------|---------------|--------|
| F1（中档） | 淘汰常数化 + 上传合并：预期单帧 CPU 下降、hitch 回落（以同机基线对比为准，不预设具体 ms 数） | `render/internal/gpu/path_geometry_cache.go`（四缓存淘汰：Path/Stroke/Dash/Convex，全表扫描→链表常数步；键/哈希/命中语义不变） · `render/internal/gpu/stencil_renderer.go` + `render_session.go` 提交路径（模版/覆盖顶点逐 path WriteBuffer → 整板一次上传；uniform 沿用 slab；着色器/管线布局不动） | ✅ 已交付（2026-09-13；单测/像素/真窗见验收） | 缓存命中率单测（`path_geometry_cache_test.go` 加淘汰回归） | 同机 30s：单帧 CPU ms、hitch_count、p95/p99 不回退 | `ui_render_pelican` RUN_SECONDS=30 + `ui_render_base_geometry` 像素对比 |
| F2（大档） | 几何存用户空间原形、变换随 uniform 下发：动画帧几何命中（病根：变换在 `render.Context` 动词级 baked 进路径坐标 → 逐帧内容哈希永变；用户空间原形 + 设备矩阵只在提交时应用，动画帧只变矩阵不换形） | 缓存键拆（形状/变换分家；坐标量化 1e-6 + 负零归一 + 尺度相对量化，旋转平移哈希稳定） · 四缓存键加尺度位 + 键控查询（命中零分配） · 模版填充/AA 带着色器加 64B 变换 uniform（视口+仿射行+颜色；AA 带距离按相似比缩放）管线布局不动 · 描边快路径（相似变换：用户空间膨胀 + 宽度除尺度，用户空间缓存；遮罩/虚线/发丝/非相似走烘焙老路） · 快路径复用 FillPath 建联检查（GPU 未就绪/目标绑定失败 → CPU 兜底，与改前一致） | ✅ 已交付（2026-09-13；单测/像素/真窗见验收） | F2 专项 6 项（相似门/往返哈希/uniform 布局/尺度键/描边键一致/容差尺度） | 同机 30s：hitch 17→2、p99 21.8→18.4ms（负载同约 1.9） | `ui_render_pelican` RUN_SECONDS=30 + 离屏双时刻（>4 差异 ≤9/84 万像素，max 29，落点为旋转辐条 AA 边缘） |

**F1 验收门（硬）：** ① 缓存单测绿 ② 同机同二进制基线对比（单帧 CPU、hitch、p95/p99）
③ 离屏像素一致（除既有 CPU 兜底退化外）④ `go vet` + 上表回归不回退。
**F1 验收结论（2026-09-13）：** ① `render/internal/gpu` 全绿（`render` 包 6 个失败经 stash 对照为改前已存在：AA 探针/CPU 回退、虚线、描边文字、混合、布尔、S6.9 合同）② 同机 30s：光栅指示 15.87→11.50ms、hitch 44→13、p99 54.4→34.2ms（负载相近 1.8~2.2）③ 离屏双时刻逐字节一致 ④ vet 仅老告警（gpu_render_context 不可达 + attack 测试 unsafe，与改前一致）。
**F2 开工门：** F1 四门全绿（已满足，可开）。**F2 验收结论（2026-09-13）：** ① `render/internal/gpu` 全绿（含新增 F2 专项 6 项）；`render` 包失败集与改前逐项一致（6 个老失败：AA 探针、虚线、混合、布尔、S6.9 合同、描边文字；中间曾引入 9 个描边/裁剪/文字失败，根因为快路径跳过建联检查与遮罩路由，已修并复验）② 同机 30s：hitch 17→2、p99 21.8→18.4ms、58.1fps（负载同约 1.9；另有两轮 4~5 hitch 佐证）③ 离屏双时刻：>4 差异 ≤9/84 万像素、max 29（旋转辐条 AA 边缘亚像素，无结构差异）④ `ui/...` 回归：仅剩 2 个改前已失败（TestUIDoesNotImportGPU、TestPicture_Replay_DrawImagePixels；另 3 个疑似失败经安静重跑确认为并发负载污染）⑤ vet 仅老告警。**退出门：** 两期交付后本节状态 → ✅，修订记 1.31。

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
| **指标 D8（§20）** | `rss_slope_kb_per_min` + `rss_slope_steady_kb_per_min`/`rss_slope_valid` · `gpu_ops`/`cpu_fallback_ops` · `CompareToBaseline` · baseline_test · perfsoak | 非 VRAM/atlas；baseline 容差松；GPU ops 为 Context 累计；短跑 raw 不作泄漏结论 |
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
| `ui_render_base_geometry` | 5–6 | 几何/Path + 逻辑/像素/整窗 Golden（409200px） |
| `ui_render_base_text` | 10 | 文本 + 逻辑/像素/整窗 Golden（576000px） |
| `ui_render_base_image` | 9 | 图/异步/RSS + 逻辑/像素/整窗 Golden（499200px） |
| `ui_render_base_cliplayer` | 7–8 | Clip/Transform + 逻辑/像素/整窗 Golden（403200px） |
| `ui_render_base_pathlow` | A 组#1 | 路径低频三 API（FillPreserve/StrokePreserve/GetCurrentPoint）+ 像素/Golden |
| `ui_render_base_pathshape` | A 组#2 | 路径造型三 API（Trim/WithCorners/Discrete）+ 像素/Golden |
| `ui_render_base_strokepreset` | A 组#3 | 描边预设族（8 预设 + SetStroke）+ 像素/Golden |
| `ui_render_base_sweepgradient` | A 组#4 | Sweep 渐变族（7 色标整圈/起始角/负向/局部 × Pad·Repeat·Reflect + 门面 `FillSweepGradient`）+ 像素/Golden |
| `ui_render_base_clipop` | A 组#5 | 裁剪运算（ClipRectOp/ClipPathOp difference·replace：整数/分数边/多子路径/开放路径 + ClipRect/ResetClip/SetFillRule）+ 像素/Golden |
| `ui_render_base_maskops` | A 组#6 | 掩码写路径族（SetMask/ClearMask/InvertMask/ApplyMask/PushMaskLayer 圆掩码图层 + Mask 构造）+ 逻辑/像素/Golden/**抓屏 vs 快照**。**2026-10-26 活窗复核转红（跨线，待收）**：常驻真窗屏幕 `diff=63.2419%`（`screen=false`），四张掩码卡全部不显示；快照/golden/像素仍全绿。计数实测 `liveOnlySkips=2872`/360 presents；回退当日 IME 修复后逐位同值，证明基线即红。根因：掩码后处理挂在 `RequireLiveContext()` 上，retained 录制通道（Recorder）拒绝后整卡丢失；归属线程计划 S2b/C3「可录制掩码后处理」，未收前本行不得按抓屏绿计。详见 `RENDER_BASE_RESOURCE_MATRIX.md` §5.4.1 |
| `ui_render_base_backdrop` | A 组#7 | Backdrop 层族（PushBackdropLayer 四卡：基面压暗 / 隔离卡毛玻璃 / 裁剪面板 / 嵌套背板）+ 逻辑/像素/Golden/**抓屏 vs 快照**；1x/4x 双模式，4x 专属 Golden |
| `ui_render_base_textstroke` | A 组#8 | 文本描边/沿路径族（StrokeString 线宽 4/8/16 / StrokeStringAnchored 三锚点 / TextPath→Fill·Stroke·DrawString 对照 / 90° 旋转 + 20.2·20.8 分数字号）+ 逻辑/像素/Golden/**抓屏 vs 快照** |
| `ui_render_base_textlow` | A 组#9 | 文本低频四 API（MeasureMultilineString 多行度量+DrawStringWrapped 三锚点 / WordWrap 行尾空白与 CRLF·CR / 六种 TextMode / LCDLayout None·RGB·BGR 同帧交错）+ 逻辑/像素/Golden/**抓屏 vs 快照** |
| `ui_render_base_textspacing` | A 组#10 | 文本字距 SetLetterSpacing（A 单行 -4/0/+5/+10 实测标尺与端 tick / B CJK·RTL 簇间隙 / C 同宽 130px 回绕 3→5 行 / D MeasureWidth=TextLayout=绘制 + 缓存失效）+ 逻辑/像素/Golden/**抓屏 vs 快照** |
| `ui_render_base_verticesmesh` | A 组#11 | 顶点/网格 DrawVertices(Ex)/DrawMesh(Ex)（A Gouraud+纯色回退+Ex 诊断 / B TriangleFan+25° CTM / C indexed=expanded+3 索引单三角 / D 短索引·颜色不匹配·坏索引）+ 逻辑/像素/Golden/**抓屏 vs 快照** |
| `ui_render_base_dither` | A 组#12 | 抖动 SetDither/Dither（A 线性 off/on+4×+差异×64 / B 径向 off/on / C 扫掠 off/on / D 纯色 off/on+4×+差异×64）+ 逻辑/像素/Golden/**抓屏 vs 快照** |
| `ui_render_base_webp` | A 组#13 | WebP 加载/解码（A 无损+Alpha 2×/4×/PNG 参考 / B 有损 VP8 q80 1×/2× / C LoadWebP·LoadImage(.webp)·LoadImage(.bin)·PNG 四路径 / D 缺失·非 WebP·空文件三错误）+ 逻辑/像素/Golden/**抓屏 vs 快照** |
| `ui_render_base_filterops` | A 组#14 | 内容滤镜 ApplyInvert/ApplyBlurXY/ApplyImageFilterGraph/GPUFilterTexture（A 反色 / B 纵糊横锐 / C 三节点图链 / D 发布视图直绘并保 CPU 底）+ 逻辑/像素/Golden/**抓屏 vs 快照** |
| `ui_render_base_miscdraw` | A 组#15 | 杂项绘制 DrawShadow/DrawColor/DrawPaint/ClipBounds/DrawParagraph（A 路径海拔阴影 / B 裁剪+Multiply·Screen 顺序混合+圆角裁剪 / C 渐变填 clip 保路径 / D 裁剪界四态 / E 多行 MultiFace 段落）+ 逻辑/像素/Golden/**抓屏 vs 快照** |
| `ui_render_base_pathbool` | C 组#5 | 布尔路径 Path.Op（A Difference NonZero·EvenOdd 同路径同洞 / B Union α=0.72 交叠只混一次 / C 菱形∩圆精确交点 / D x≈3000 远坐标 Xor）+ 逻辑/像素/Golden/**抓屏 vs 快照**；同批修半透明 stencil path 重复预乘 |
| `ui_render_base_cliprrect` | C 组#6 | 圆角裁剪消费路径（A `DrawGPUTexture`·`textured_quad` / B `DrawImageEx` / C `DrawString` 字形 / D `Push`/`Pop` 恢复）+ 逻辑/像素/Golden/**抓屏 vs 快照**；同批修 CPU 位图/缩放/别名文本绕过 `ClipStack` |
| `ui_render_base_svg` | C 组 SVG | SVG 图标四卡（填充圆 / 根 `fill="none"` 描边圆 / 嵌套 `<g transform>` / `RenderWithColor` 主题色）+ 逻辑/像素/Golden/**抓屏 vs 快照**；同批修 `fill="none"` 继承、根百分比、非法尺寸静默吞 0 |
| `ui_render_base_imagepattern` | B 组 图案/画笔 | 旧 Pattern/ImagePattern/SetFillPattern/SetStrokePattern/SolidPattern/Paint 四卡（裁片平铺 / anchor·scale·opacity·clamp+旋转 / 非矩形圆环描边 / 自定义 Pattern）+ 逻辑/像素/Golden；同批修非实色描边被实色快速路径吃掉 |
| `ui_render_base_variablefont` | B 组 VF 变体 | VF 四卡（A `wght` 200/400/600/900 墨量递增 / B fvar 四命名实例 / C 默认=显式默认 / D 边界钳位+未知轴）+ 逻辑/背景点/墨量/Golden；同批修 MSDF/glyph-mask font ID、默认 master 光栅化、静态 HVAR advance、HarfBuzz/TextPath 实例缓存 4 处断点 |
| `ui_render_base_recording` | B 组 Recording | Recorder→raster Playback 四卡（A 填充/渐变/虚线/变换 / B Clip·ResetClip·Save/Restore·ClipRoundRect / C DrawString+DrawImage / D 重放·资源池·StrokeRect）+ 逻辑/像素/墨量/Golden；同批修 Playback 漏 StrokeRect、raster 裁剪空实现、DrawImage 只画未填充矩形、DrawText face 丢失、非零原点子图源矩形偏移 5 处 |
| `ui_render_base_scene` | B 组 Scene | 同一 `scene.Scene` 双后端四卡（A GPU `NewGPUSceneRenderer.RenderScene`：multiply/50% alpha 图层·旋转·圆裁剪·矩形裁剪内圆角矩形 / B 同一场景 CPU 64×64 分块→`DrawImageEx` 上屏并与 A 做容差 A/B / C `Encoding.Hash`+`LayerCache` 两色缩略图 / D 文本+图像+`RenderWithDamage` 移动矩形增量）+ 逻辑/像素/墨量/Golden；同批修 CPU 图层 alpha/blend、`TagFillRoundRect` 裁剪与变换、Hash 漏 brushes、`TaggedBounds` 未展开、append/clone 丢 commandBounds、`TagTransform` 状态泄漏 6 处 |
| `ui_l1_scroll` | 12 | 滚动/虚拟列表 |
| `ui_render_base_perfsoak` | 每项建议 | §20 回归 |

### 23.1 自定义控件主能力真窗（`ui_wr_*`）

> 完整 SoT：[`ENGINE_UI_WIDGET_RENDER.md`](./ENGINE_UI_WIDGET_RENDER.md) **§2 / §3 / §2.5**。  
> 本表只钉 **几何与时长**，避免与 WIDGET 主能力表漂移。

| 项 | 硬规格 |
|----|--------|
| **客户区** | **1200 × 800** 逻辑像素（所有 `ui_wr_r*` / `ui_wr_c*`） |
| **最小运行** | **`RUN_SECONDS ≥ 5`**（&lt;5 不得关闭 R/W/C；实现侧 FAIL） |
| **按能力加长** | 关闭用时长见 WIDGET §2 主表与 §2.5：正确性 5s · Boundary 8–10s · 层/DPR 15s · 动画 30s · 列表/压力 60s · soak **300s** |
| **指标** | 结束输出 §20.0 族 A–J + 能力专用字段；不达标 `FAIL:` + `exit 1` |
| **路径** | `examples/ui_wr_<id>/` only（禁止放进 `ui/`） |

### 23.2 P1 首批 render-axis 真窗（`ui_render_graphics/*`）

| 程序 | 画面 | 三证据 |
|------|------|--------|
| `examples/ui_render_graphics/basic` | Point / LineSegment / Triangle 六格（静态） | 逻辑 5/5、像素 9/9、shell 掩码 821184px 两连 0.0000%；full_paint；静态窗不设 FPS 门禁（presents≈15） |
| `examples/ui_render_graphics/vector` | QuadTo / CubicTo / Arc / EllipticalArc 六格（静态） | 逻辑 5/5、像素 8/8、shell 掩码 821184px 两连 0.0000%；full_paint；静态窗不设 FPS 门禁（presents=300 仅观测） |

六扇共用的像素/Golden 辅助在 `examples/wrevidence/evidence.go`；gate 档统一 `-auto-only` 5s，manual 档 `-manual-seconds N` 且不写快照。

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
2. Picture 显示列表 = rect/path/circle/oval/arc/roundrect/line/gradient/text/image 子集；无列表内 transform/clip/saveLayer；无 GPU 纹理缓存。
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
| **1.62（2026-10-26）** | **IME 光标/选区不显示修复（通用渲染层）+ 六扇 IME 真机人工验收 + maskops 活窗红登记**：① 现象：IME 真窗焦点切换正常、字形正常，光标与选区纯色块不显示。根因：祖先 mask/difference 裁剪被提升进离屏录制通道——mask 平面按整面坐标存放，录制通道按录制局部坐标绘制，提升后采样恒 0，纯色块 Fill 被静默丢弃（字形走另一路径，故不可见）。修法（对齐 Skia saveLayer-with-mask：掩码在合成时生效）：`render/context_clip_mask_gpu.go` 新增 `maskClipPromotionAllowed()`（`offscreenPassSuspended()` 内禁止提升），`render/context_draw.go` 两处 `forceCPUClip`（Fill/Stroke）改用它，`applyMaskToPaint` 在离屏子通道内直接返回以录制完整画面；新增永久护栏 `render/offscreen_record_mask_test.go` `TestOffscreenPassSkipsAncestorClipMask`（子通道外允许 / `offscreenPassDepth=1` 禁止 / `MaskCoverage` 内 nil 外非 nil，PASS）。② 六扇 IME 窗常驻人工档真实输入验收：r2 81.7s/59.8fps/hitch 2/fallback 0/RSS 7350KB/min；r3 33.6s/59.7fps/hitch 0；r4 106.4s/59.8fps/拖选 2621px/双击 1568px/跨行 4968px；r5 51.0s/59.8fps/两段选区 1956+2780px；`ui_textinput_ime` 逐字输入 + 光标 24px 呼吸；r1 修复后复验光标 38px、选区 4056px。③ 门禁盲区：`-auto-only` 快照是 live-context 合成重绘，走不到 retained 录制通道，此类缺陷只能由常驻人工档捕获。④ 登记跨线红：`ui_render_base_maskops` 活窗 `diff=63.2419%`、四卡不显示、`liveOnlySkips=2872`，基线同值，归 S2b/C3 可录制掩码后处理。观察项：密码框选区高亮按设计抑制（`ui/textinput/box.go:934`）；`ui_textinput_ime` 示例空区点击命中内嵌 RenderBox 不触发 OnPointer（示例层，不影响生产 `textinput.Box`）。**公开 API 无签名变更**。真源同步：`RENDER_BASE_RESOURCE_MATRIX.md` §5.4.1/§5.4.2/§9、本文件 A 组#6 行/§24。**性质=引擎正确性回归 + 真机人工验收** |
| **1.61（2026-10-24）** | **C4 纹理层接视频（线程计划 §18.1 #17 第六笔，`e00e6dbc`）**：`ui/scene/layer.go` 新增 `TextureLayer`（只带 `TextureID/X/Y/W/H/DrawIndex`，不带活指针；永不进光栅缓存、每帧重画）+ `NewTextureLayer`/`LayerID`/`Children`/`Kind`/`Rect`；`ui/rendering/layer_build.go:460` 在普通 picture 叶子前询问 `TextureLeafProvider`，`addTextureLeaves`（`:509`）把 `TextureLeaf` 转成 `scene.TextureLayer`，`RenderVideo.TextureLeaves`（`ui/rendering/video_direct_node.go:171`）按 `bridge.ID()` 交主视图 + PiP 两片；`render/texture_source.go` 新增 `TextureSource` 契约 + `TexturePaintOptions` + `RegisterTextureSource`/`UnregisterTextureSource`/`GetTextureSource`/`LiveTextureSources`，`VideoBridge` 实现 `PaintTexture`/`ID`/`PaintStats` 并在 `NewVideoBridge`/`Close` 自注册/自注销；光栅合成 `ui/scene/composite.go:111` 按 id 解析，源不在/无帧返回 `ok=false` 记 miss。C0 复现 `ui/rendering/layer_build_video_retained_test.go` 从 `t.Skipf` 改回 `t.Fatalf` 并通过；三窗 `video_player`/`video_4k_play`/`video_vr8_embed` 各加 retained 封包两帧像素断言。**2026-10-24 三窗实跑（非 -race）**：retained 像素全绿；**未达**：4K/VR8 p95 22.99/22.19ms 超 22、-race 被 #18 挡、无 Golden；V-3 已修（单测，真窗负向未跑）。**P0/P1 已修（2026-10-24）**：`PaintStats` 增 `ViewLastOK`、`VideoNodeStats` 增 `DrewLast`/`SmallDrewLast`，4K/2K 门禁改断最近一帧；VR8 去掉 `paintsB > paintsA` 自证门；纹理层 miss 仅曾出画后补损伤（`TextureLayer.painted`；从未出画层不污染）。`RENDER_API_CATALOG.md` 已同步（+3 类型/+4 函数/+3 方法），`go run ./scripts/apidoc` 绿。**后续（`172eadc0`）**：4K 异步双缓冲上传后 p95 19.31ms / fps 58.95 / uploads=frames=104 已过，VR8 21.85ms 已过；C4 仍未关只剩视频 Golden 与 4K 解码吞吐（视频线 V-5）。**2026-10-24 收口**：新增确定性 Golden 窗 `examples/video/video_retained_golden`（固定 904×656 NV12 素材 1:1 走 `RenderVideo → TextureLayer → retained 合成`，只上传一次冻结；颜色探针 4/4、两次回放稳定 0、Golden 0 覆盖 589908px，零容差，两次门禁 + `-race` 全绿）；4K/2K 报告的 `color_diff_per_channel`/`pixel_golden_diff_pct` 改为未测 `null`，不再硬编码 0。C4 渲染口径证据链齐、已关；视频线只剩 V-5 解码吞吐（非渲染）。 |
| **1.60（2026-10-21）** | **P1 首批六扇 render-axis 真窗三证据收口（Text / Image / Geometry / ClipLayer / Graphics basic / Graphics vector）**：① `ui_render_base_text` 逻辑 5/5、像素 5/5、整窗 Golden **576000px** 两连 `0.0000%`，gate 5s 两连 exit 0（presents≈300），gate 冻结 ColorPulse/DCPanel phase，manual 2s 保留脉动；② `ui_render_base_image` 逻辑 8/8、像素 8/8、整窗 Golden **499200px** 两连 `0.0000%`，保留异步解码（`async_decode_callbacks=1`、`swaps=1`、`status=ready`），gate 5s 两连 exit 0；首帧解码 hitch 使 p95 曾达 23.34ms，本窗明确不设 p95 硬门禁；③ `ui_render_base_geometry` 逻辑 6/6、像素 9/9、整窗 Golden **409200px** 两连 `0.0000%`，gate 5s 两连 exit 0；gate 冻结三个脉动面板；顶部标签未设 face、实际不显示，像素/Golden 只按实际画面采样，不声称不可见文字已断言；④ `ui_render_base_cliplayer` 逻辑 6/6、像素 9/9、整窗 Golden **403200px** 两连 `0.0000%`，gate 5s 两连 exit 0；gate 冻结 phase/clipOff/rotation/radius；⑤ `ui_render_graphics/basic` 逻辑 5/5、像素 9/9、shell 掩码 Golden **821184px** 两连 `0.0000%`，gate 5s 两连 exit 0，静态画面 `presents≈15`，不设 FPS 门禁；⑥ `ui_render_graphics/vector` 逻辑 5/5、像素 8/8、shell 掩码 Golden **821184px** 两连 `0.0000%`，gate 5s 两连 exit 0，静态画面不设 FPS 门禁。**共同实现**：新增 `examples/wrevidence` 共享像素/Golden 辅助并实际接线；六窗统一 `-auto-only`（默认 5s，gate 档冻结动画/相位）与 `-manual-seconds`（manual 不写快照），gate 档 stdout 输出 §2.2 报告，字段统一为 `logic_probes_ok` / `pixel_probes_ok` / `pixel_golden_*`；四个 base 窗整窗 Golden，两扇 graphics 沿用 header+legend+body 掩码（HUD 不入 Golden）。**公开 API 无变更**。限界：本批为原子验证，无组件/2.5D/视频生产消费者；graphics 两窗是静态画面，不作持续 FPS 结论；image 短窗 p95 不作性能门禁；geometry 标签文字不显示。真源同步：本文件 §23/§24、`RENDER_FEATURE_COVERAGE.md` §1/§11、`docs/review/WINDOW_EVIDENCE_AUDIT_2026-10-21.md` §3.3/§5/§6/附录 A.2、`ENGINE_UI_WIDGET_RENDER.md` §10、六窗 README。**性质=原子验证**，不算生产接线 |
| **1.59（2026-10-21）** | **资源债复核：B 组四窗 RSS 启动爬坡非泄漏 + M-RSS 指标口径修正**：四窗 + c0 基线 5s/15s 斜率均由启动爬坡主导（c0 5s 回放 ≈699k KB/min），60s 实测 1.9k–8.8k KB/min、180s Scene ≈2.0k，均低于 30k；`smaps` 增长只在 Go 堆，`WR_MEMDIG` live 1.5–2.2MiB / peak 9–34MiB。新增 `rss_slope_steady_kb_per_min` / `rss_slope_valid` / `rss_warmup_sec`；原 `rss_slope_kb_per_min` 保留兼容并明确不作泄漏结论。 |
| **1.58（2026-10-21）** | **B 组 Scene 真窗 `ui_render_base_scene` + CPU 分块渲染/编码状态六处修复**：1200×800 四卡（A 同一场景 GPU 场景渲染器 / B 同一场景 CPU 64×64 tile 后上屏 A/B 对比 / C `Encoding.Hash`+`LayerCache` / D 文本+图像+损伤增量）。逻辑 21/21、像素 17/17、墨量 1/1、Golden 627200px `0.0000%`（seed 后两次复跑零差）；A/B `mismatch=1.4376%`（tol 10，门限 2%）、`high(>96)=0.0259%`（门限 0.1%）、`worst=111`（AA 边缘，只报告不设门）；auto `presents=300`/`fps_interval=60.02`/`p95=18.71ms`/`hitch=0`/`gpu_ops=15003`/`cpu_fallback_ops=0`/`gpu_fallbacks=0`/`present_policy=full_paint`/`vsync_source=true`，manual 15s `presents=900`/`fps_interval=59.95`/`p95=19.16ms`/`hitch=1`/`gpu_ops=45003`/`cpu_fallback_ops=0`。**引擎修复**：①CPU tile `TagPushLayer` 丢 alpha/blend → 逐层 pixmap + 按 `BlendMode+opacity` 合成（嵌套/安全清栈）；②`TagFillRoundRect` 绕过裁剪且忽略 `Path.Transform` 返回值 → 变换路径后填 `activePM`（多 tile 不再从场景原点重画）；③`Encoding.Hash` 漏 brushes → 纳入 Kind+RGBA；④`Scene.TaggedBounds` 未展开 root layer → `DamageTracker` 零对象，改先 `flattenLayers()`；⑤`Append/AppendWithTranslation/Clone` 丢 `commandBounds` → 搬运/克隆；⑥`TagTransform` 状态泄漏 → `EncodeDrawTransform` 跟踪流内状态并补 identity 重置，append 边界同补。永久回归 `render/scene/paint_correctness_test.go` 八例（修前红→修后绿）；`go test ./render/scene/ -count=1` 全绿。**公开 API 变更**：新增 `Encoding.EncodeDrawTransform(Affine)`（只增方法，顶层计数不变）。限界：原子验证无生产消费者、A/B 不覆盖设备/驱动差异、损伤链只验 CPU 分块、RSS 资源债（5s `82.2MB→287–291MB`、`slope≈130,000–248,000 KB/min`；15s `82.2MB→298.3MB`、`slope≈73,656 KB/min`）、未做抓屏 vs 快照、LayerCache 不验淘汰/并发、文本只验一条 Latin run。真源同步：`RENDER_FEATURE_COVERAGE.md` §1 计数/§2/§6/§11、本文件 §23/§24、`RENDER_API_CATALOG.md` §2/§6.2、本窗 README。**性质=原子验证**，不算生产接线 |
| **1.57（2026-10-21）** | **B 组 Recording 真窗 `ui_render_base_recording` + raster 回放五处缺陷修复**：1200×800 四卡（A 渐变·`FillRectangle`·圆·圆角描边·虚线·`Push/Translate/Rotate` / B `Clip`·`ResetClip`·`Save/Restore`·`ClipRoundRect` / C `DrawString`+`DrawImageScaled` / D 多次 `FillRect`·旋转圆·`StrokeRectangle`+同一 Recording 重放两次），四卡均由 `Recorder` 录制、`Recording.Playback` 到内置 raster 后端、再 `DrawImageEx` 上屏。逻辑 19/19（22 种命令类型覆盖、contract 23 条命令）、像素 17/17、墨量 1/1、Golden 表头+图例+四卡 627200px `0.0000%`；auto `presents=301`/`fps_interval=60.24`/`p95=17.32ms`/`hitch=0`/`gpu_ops=9936`/`cpu_fallback_ops=0`/`gpu_fallbacks=0`/`present_policy=full_paint`/`vsync_source=true`，manual 15s `presents=898`/`fps_interval=59.92`/`p95=17.15ms`/`hitch=0`/`gpu_ops=29637`/`cpu_fallback_ops=0`。**引擎修复**：①`Playback` 漏 `StrokeRectCommand` → 转矩形路径走 `StrokePath`；②raster `SetClip` 只设路径未 `Clip()`、`ClearClip` 空实现 → 走 `render.Context` 真实裁剪栈 `Clip()/ResetClip()`；③raster `DrawImage` 只发未填充矩形 → `ImageBufFromImage`+`DrawImageEx`（源裁片/目标/插值/alpha）；④`DrawTextCommand`/`StrokeTextCommand` 不带字体、Playback 传 nil → 新增公开字段 `Font FontRef`，录制时 `AddFont`、回放解析 `text.Face`，raster `SetFont`+`DrawString`；⑤`ImageBufFromImage` 归一化子图到 `0,0`，旧回放源矩形又加 `Bounds().Min`，非零原点子图采样错位 → 源矩形改按图像相对坐标裁剪。永久回归 `render/recording/backends/raster/backend_paint_test.go` 七例（含非零原点子图；修前红→修后绿）；`go test ./render/recording/...` 全绿。**公开 API 变更**：`DrawTextCommand.Font`、`StrokeTextCommand.Font` 字段（只增字段，不改签名）。限界：raster 非全量 Canvas（StrokeText 填充近似/SetTransform 已烘焙/SetAntiAlias 无开关/DrawImage 轴对齐/ClipRoundRect 未烘变换）、RSS 资源债（5s `194.8MB→262.2MB`、`slope≈421,273 KB/min`；15s `194.2MB→287.1MB`、`slope≈177,691 KB/min`，留 §7/§8）、未做抓屏 vs 快照、原子验证无生产消费者、仅 raster 后端（PDF/SVG 仓外）。真源同步：`RENDER_FEATURE_COVERAGE.md` §1 计数/§2/§3.2/§6/§11、本文件 §23/§24、`RENDER_API_CATALOG.md` §2/§6.3/§7.2、本窗 README。**性质=原子验证**，不算生产接线 |
| **1.56（2026-10-21）** | **B 组 VF 变体真窗 `ui_render_base_variablefont` + 变体不到 GPU 文本管线修复**：四卡 A `wght` 200/400/600/900 / B fvar 四命名实例 / C 默认=显式默认 / D 边界钳位+未知轴；逻辑 18/18、背景点 4/4、墨量 17/17、Golden 627200px `0.0000%`；auto `presents=300`/`fps_interval=60.03`/`p95=17.31ms`/`hitch=0`/`gpu_ops=17703`/`cpu_fallback_ops=0`，manual 15s `presents=899`/`p95=17.19ms`/`hitch=0`。**引擎修复**：①MSDF/glyph-mask font ID 组合 `variationFontID`；②glyph-mask 新增 `rasterizeVariationGlyph`，按 `face.Variations()` 抽轮廓后复用 outline 光栅器；③hint pen 改用 shaped X 差（HVAR），不再用静态 hmtx advance；④HarfBuzz face 缓存键改为 `(FontSource, variation hash)` 并建私有 `font.Face`，`TextPath`/CPU outline 缓存组合同一变体哈希。永久回归 `render/internal/gpu/varfont_atlas_test.go`、`render/text/varfont_shape_test.go`、`render/text_variable_path_test.go`（修前红→修后绿）。限界：RSS 资源债（5s `rss_start≈78.4MB→rss_end≈255.2MB`、`slope≈528,846 KB/min`，留 §7/§8）、未做抓屏 vs 快照、原子验证无生产消费者、只覆盖 `wght`、复杂脚本 VF 未独立建窗。**公开 API 无签名变更**。真源同步：`RENDER_FEATURE_COVERAGE.md` §1 #31/§3.2/§6/§11、本文件 §23/§24、本窗 README。**性质=原子验证**，不算生产接线 |
| **1.50（2026-10-20）** | **C 组#6 真窗 `ui_render_base_cliprrect`（ClipRoundRect 图片/文字/GpuTexture）+ CPU 文本裁剪修复**：四卡 A `DrawGPUTexture`（200×160 GPU 发布视图，`textured_quad` RRect 覆盖）/ B `DrawImageEx` / C 三行 `DrawString`（起点在框外、字符串越右边界）/ D `Push`→圆角裁图→`Pop` 后未裁剪对照。逻辑 11/11、像素 15/15（容差 8/255 + 通道带）、Golden 表头+图例+四卡 627200px `0.0000%`、抓屏 vs 快照同 627200px `0.0000%`（tol ±2/255）；`gpu_ops=25803`/`cpu_fallback_ops=0`/`presents=300`，纹理视图 `200×160`/live paints=301、纹理源 `gpu_ops=1`/`cpu_fallback_ops=0`。**引擎缺陷**：CPU `drawStringBitmap`/`drawStringScaled`/`drawStringCPUAliased` 直接 `text.Draw*` 写 pixmap，绕过 scissor 与 `ClipStack`；修前 CPU 探针 C 卡框外泄漏 1191 个亮像素。修法：`render/text_clip_cpu.go:tryDrawTextWithClip` 有裁剪时先画透明 scratch，再用 `ClipStack.Coverage` 生成 alpha 掩码合成；无裁剪保持原快路径；三入口同批接入。永久回归五例（CPU 位图/别名/2× 缩放三入口框外 0，旧 dispatch 实测泄漏 1191/1200/4662px；字形包围盒包住实际墨迹；GPU 纹理与字形裁剪）。限界：CPU 文本优先按 run 字形包围盒 + 字体度量余量分配临时图/掩码，异常越界回退整块裁剪重画（窗口 GPU 主路不受影响；同机全窗 1200×800 基准 98.7→12.8ms/次）、A 卡需 GPU 滤镜发布视图；本窗为原子验证。**公开 API 无签名变更**（仅未导出方法）。真源同步：`RENDER_FEATURE_COVERAGE.md` §1 #12/§3.3/§6/§11、本文件 §23/§24、本窗 README。**性质=原子验证**（直调 render 接口一次验一族），不算生产接线 |
| 1.51（2026-10-21） | **C 组 API 收敛：删除旧两参 `Context.DrawImageQuad`，并把 `DrawImageQuadEx` 正名为 `DrawImageQuad(img,corners,opts)`**：旧两参入口只剩兼容包装，生产调用为零，仓库内仅三个测试调用；按开发阶段「接口全量切换制」删除旧方法与未调用 `drawImageQuadCPU`，带选项入口去掉非标准 `Ex` 后缀，测试/示例同批改调 `DrawImageQuad`（显式 `QuadDrawOptions{}`，退化四边形仍返回 `ErrQuadDegenerate` 且不污染画布）。FImg-QUAD 母表入口同步改为 `DrawImageQuad`。`Context` 导出方法仍 200（删一留一）；`RENDER_API_CATALOG.md` §0/§3.5/§3.10/§8、`RENDER_FEATURE_COVERAGE.md` §1 #19/#20/§3.3/§9/§11、`ENGINE_UI_WIDGET_RENDER.md` §10 同步。验证：`go test ./render -run '^TestQuad' -count=3`、`TestQuad* -race`、`TestP1_Capability_T04_ImageQuadGPU`、`TestP1_S2_/T1_/U1_`、`go run ./scripts/apidoc` 均绿；`go build ./render/...` 与 `examples/engine/quad` 绿。**性质=API 收敛，无新增能力**。 |
| 1.52（2026-10-21） | **C 组子包裁撤：删除 `render/raster`**：该包只有 35 行 init，注册 `gpuimpl.AdaptiveFiller`；全仓非测试零 import，且同一 filler 已由 `render/gpu` 的 init 注册。删除后 CPU 复杂路径分块光栅仍由 `render/gpu` 启用，不需要第二个开关。同步清理 `render/gpu` 文档注释、`render/internal/gpu/transformed_mask_text.go` 旧引用、`scripts/apidoc` 跳过说明及 API 总账/覆盖表。验证：`go build ./render/...`、`go test ./render -run '^TestQuad'`、`go run ./scripts/apidoc` 绿。**性质=死包清理，无能力变化**。 |
| 1.53（2026-10-21） | **C 组子包裁撤：删除 `render/surface`**：整包 53 个公开 API，全仓非测试零 import；`ImageSurface` 与 render 主包 CPU 渲染重复，`GPUSurface` 只是外部 backend 壳，自带的 Path/Stroke/Brush 又是一套劣化实现。按「render 只做通用难事」与成熟库优先原则裁撤，第三方后端未来统一走 render/hal。同步删除 `scripts/apidoc` 的 surface 扫描，更新 API 总账 §0/§6.4/§7.2/附录与覆盖表 §2/§3.3/§9。验证：`go build ./render/...`、`go test ./render -run '^TestQuad'`、`go run ./scripts/apidoc` 绿。**性质=死包清理，无能力变化**。 |
| 1.54（2026-10-21） | **C 组 SVG 真窗 + `fill="none"`/根百分比修复**：新增 `examples/ui_render_base_svg`（1200×800 四卡：填充圆 / 根 `fill="none"` 描边圆 / 嵌套 `<g transform>` / `RenderWithColor` 主题色；图标经 256×256 SVG raster → `ImageBufFromImage` → GPU `DrawImageEx`）。逻辑 7/7、像素 9/9、Golden 表头+图例+四卡 627200px `0.0000%`、抓屏 vs 快照同 627200px `0.0000%`（tol ±2/255）；`gpu_ops=26703`/`cpu_fallback_ops=0`/`gpu_fallbacks=0`/`presents=300`；`present_policy=full_paint`/`vsync_source=true`；`fps_gate=off_correctness`；人工档 `-manual-seconds 8` 退出 0（`presents=480`、`gpu_ops=42723`、`cpu_fallback_ops=0`、抓屏 0）。**引擎修复**：`fillAndStroke` 默认黑填充兜底改看有效值 `resolveFill/resolveStroke`（祖先 `fill="none"` 不再被黑填充覆盖）；`parseSVGRoot` 先读 viewBox 再按参考轴解析根 width/height 百分比；`<circle>/<rect>/<ellipse>/<line>` 几何改用 `parseLength`，非法值/子元素百分比明确报错，不再静默吞 0。永久回归 `render/svg/svg_test.go` 四例。**公开 API 无签名变更**。限界：仍为图标子集（defs/use/渐变/clipPath/preserveAspectRatio 未支持）、子元素百分比报错、原子验证无生产消费者。真源同步：`RENDER_FEATURE_COVERAGE.md` §2/§3.3/§9/§11、`RENDER_API_CATALOG.md` §6.4、本文件 §23/§24、本窗 README。**性质=原子验证**，不算生产接线。 |
| **1.55（2026-10-21）** | **B 组旧 Pattern/画笔真窗 `ui_render_base_imagepattern` + 非实色描边快速路径修复**：四卡 A 32×16 图集左侧 16×16 四象限裁片平铺 / B anchor+scale 2+opacity .5、clamp、旋转+缩放 2 的 ImagePattern / C `SetStrokePattern` 非矩形圆环 / D 自定义棋盘 Pattern、`NewSolidPattern`、Paint brush/pattern 互转。逻辑 17/17、像素 25/25、Golden 表头+图例+四卡 627200px `0.0000%`；`presents=300`/`fps_interval=60.06`/`interval_p95=18.37ms`/`hitch=0`/`gpu_ops=15003`/`cpu_fallback_ops=0`/`present_policy=full_paint`/`vsync_source=true`；人工档 `-manual-seconds 15` 退出 0（`presents=899`、`p95=18.35ms`、`hitch=0`、`gpu_ops=44953`、`cpu_fallback_ops=0`）。**引擎缺陷**：C 环被 `StrokePath` 的实色快速尾部 `fillPathUser` 画成 `getColorFromPaint` 的单色（修前实测 `(0.949,0.800,0.102)`，期望红 `(0.902,0.102,0.102)`）；按 Skia/Flutter 口径新增 `strokeSolidFastPathEligible`，仅 `isGPUSolidPaint` 可走实色尾部，非实色 ImagePattern/渐变/自定义 brush 一律走 `FillPath`；`fillPathUser` 加防御性早退。永久回归 `render/internal/gpu/stroke_fast_path_test.go` 两例；`go test ./render/internal/gpu -count=1` 绿。**公开 API 无签名变更**。限界：旧 Pattern API 无生产消费者（原子验证）、RSS slope 约 489,753 KB/min 资源债留 §7/§8、未做抓屏 vs 快照、B3 仅 ImagePattern 旋转。真源同步：`RENDER_FEATURE_COVERAGE.md` §1 #10/§3.2/§11、本文件 §23/§24、本窗 README。**性质=原子验证**，不算生产接线 |
| **1.49（2026-10-20）** | **4x MSAA backdrop Blur 收口 + 采样数归 RenderTarget 的第一步**：根因不是模糊算法，而是**目标采样域错配**——backdrop 层 RT 是 1x 纹理，却由 base 4x 会话绘制；`ApplyBlur` 的 CPU 种子只写进 1x resolve，随后 15% tint 的首个 4x MSAA pass 以 `LoadOpClear` 清掉种子，最终只剩锐利父条纹 + tint（无 tint 时模糊正常，1x 同窗正常）。修法：`Context` 拆 `baseGpuCtx`（窗口，保持配置的 4x）与 `layerGpuCtx`（离屏层，固定 1x）；Push/Pop GPU 层只切 active 指针，不销毁 base 会话、不全局改采样；嵌套层共用 1x 层上下文，最外层 Pop 后回 base 合成。`GPURenderContext` 新增 `seededViews`：会话尚未创建时 `MarkViewSeeded` 先记账，会话创建后重放；`BeginFrame`/设备重建清空，`PopLayer` 回收时 `UnmarkViewSeeded`，避免池化视图继承陈旧 LoadOp。`tryApplyFilterGraphGPU` 的 mid-frame fold 仅在 `!hasLayer` 时启用。真窗：1x/4x `ui_render_base_backdrop` 均 logic 14/14、pixel 21/21，4x B 黑/白中心 0.588/0.561（期望 0.575，容差内），4x 专属 Golden 627200px `0.0000%`、screen vs snapshot `0.0000%`，`gpu_ops≈1.07 万`/`cpu_fallback_ops=0`；4x `ui_ant_verify` 仍 `mode=4x MSAA` 通过。永久回归 `render/backdrop_layer_msaa_test.go` 两例（4x 嵌套 filter 种子 / 4x filter 后 tint 保种子）+ 原 `render/backdrop_layer_present_test.go` 四例；`go test ./render` 与干净 HEAD 失败集合逐项一致，`ui/embedder` 绿，`ui/rendering`/`ui/scene` 与 HEAD 基线一致，`-race` 三例绿，GPU 上下文生命周期用例绿。**公开 API 无签名变更**。真源同步：`RENDER_FEATURE_COVERAGE.md` §1 #16/§3.3/§6/§11、本文件 §23/§24、本窗 README。**性质=引擎正确性回归 + 原子真窗验证；不是生产接线** |
| **1.48（2026-10-20）** | **4x MSAA 中帧 LoadOpLoad 未定义行为修复**：`render/internal/gpu/render_session.go:colorAttachment` 的 resolve 分支原固定 `StoreOpDiscard`，但同帧 `encodeToEncoder` 在 `viewRendered`/backdrop seed 时会以 `LoadOpLoad` 重入同一 MSAA 附件（`render_session.go:6487`）；WebGPU 对 load discarded attachment 为 UB。修法：MSAA resolve 分支改 `StoreOpStore`，1x/无 resolve 分支不变；ADR-021 的 discard 只适用于确定单 pass 的帧，encode 时无法预知最后一遍，统一 Store。永久回归 `render/internal/gpu/msaa_load_store_test.go` 两例（描述符 + 4x 两 pass GPU 读回保内容）。回归：internal/gpu 失败集合与 HEAD 干净 worktree 逐项一致（7 个存量）；ui/embedder 绿、ui/rendering/ui/scene 与 HEAD 基线一致；`TestS3b_M2_MSAAResolve` 绿。4x 真窗 `ui_ant_verify`（新增 `GPUI_MSAA=4` 控制）：`mode=4x MSAA`、presents=596、1200×800 快照、抓屏 vs 快照 diff=0.674%（tol ±2）。同一控制跑 `ui_render_base_backdrop` 暴露 B 卡 Blur 未生效的存量子项（1x 21/21，干净 HEAD 同现象，见 `RENDER_FEATURE_COVERAGE` §6），不在本次修复范围。代价：4x 每 pass 多一次 store；默认 1x 不受影响。`RENDER_FEATURE_COVERAGE.md` §3.3/§6/§11 同步。**公开 API 无签名变更**。**性质=引擎正确性回归；不是生产接线** |
| **1.47（2026-10-20）** | **C 组#5 真窗 `ui_render_base_pathbool`（Path.Op 布尔四运算）+ 半透明 stencil path 重复预乘修复**：四卡 A Difference（同路径 NonZero/EvenOdd 双规则，整数外框+分数洞）/ B Union（两圆 α=0.72，交叠必须一次 source-over 无深色缝）/ C 菱形∩圆（精确交点）/ D 两圆 Xor 在 x≈3000 运算后平移到卡内（无 2048 截断）。逻辑 10/10（精确轮廓/verbs、分数顶点、Union 单轮廓、Intersect 精确交点、Xor 两瓣、远坐标、双填充规则一致、曲线展平、空操作数、自交蝴蝶结）；像素 18/18（oracle=调色板/解析 source-over，容差 8/255 + 通道带）；golden 表头+图例+四卡 627200px 零差（播种后复跑 0）；抓屏 vs 快照同 627200px `0.0000%`（tol ±2/255）；`gpu_ops=29703`/`cpu_fallback_ops=0`/presents=300/`present_policy=full_paint`/`vsync_source=true`；`-auto-only`/RUN_SECONDS 退出 0。**引擎缺陷与修法**：`StencilPathCommand.Color` 已是预乘色，但 `render_session.go` slab uniform、`stencil_renderer.go:updateRenderBuffersSticky`、`makeCoverUniform` 又各乘一次 alpha → 半透明 path RGB=`color*α²`（B 卡实测 (0.188,0.322,0.537) vs 期望 (0.250,0.434,0.730)，alpha 不变；A/C/D 不透明故不受影响）。统一 stencil 内部预乘色语义并去掉三处重复乘法；独立 `StencilRenderer.RenderPath` 公开入口在边界预乘一次。永久回归 `TestMakeCoverUniformDoesNotPremultiplyTwice`（值级）+ `TestRenderSessionStencilPremultipliedColor`（GPU α=0.5 采样，重复乘会减半）。限界：布尔输出为折线（输入曲线按 0.1px 展平，不保曲线），记 `RENDER_FEATURE_COVERAGE` §6。真源同步：§3 FPath-COMBINE + §23 登记；`RENDER_FEATURE_COVERAGE.md` §1 #5 ⛔→✅、§3.3、§6、§11；`RENDER_API_CATALOG.md` §2；`ENGINE_UI_WIDGET_RENDER.md` API目录同步。**公开 API 无签名变更**（内部颜色契约修正）。**性质=原子验证**（直调 render 接口一次验一族），不算生产接线 |
| **1.46（2026-10-20）** | **A 组#15 真窗 `ui_render_base_miscdraw`（杂项绘制五 API）+ 六个缺陷修复**：新增 `Context.DrawShadow/DrawColor/DrawPaint/ClipBounds/DrawParagraph`（`render/context_misc_draw.go`）。① 连续两次高级混合 `DrawColor` 时第一次合成消失——第二次图层自 flush 把上一次待解层当成本层目标；按 Skia saveLayer 目的地语义给 `pendingAdvancedLayer` 加 `dstView`，离屏自 flush 只解目标为当前层 RT 的 pop，父表面 pop 留到父/呈现 flush；父隔离层退出时若自身无待画命令但仍有子层指向它，用 `HasPendingAdvancedFor` 先 flush 父 RT 再释放；效果发布 RT 通过 `FlushOffscreenBase` 同样接收基面 pop（否则纯高级着色的 effect surface 不发布纹理）；并禁止离屏 flush 切 frameScratch（否则递归）。② `MultiFace.Source()==nil` 致段落 HarfBuzz 整形 0 字形；`shapeSegments` 先按 `MultiFace.Runs` 切逐 face 段再整形，cluster 偏移 `seg.Start+part.start`，RTL 仍走旧单 face。③ `ClipBounds` 对路径裁剪返回全画布；`clip.Stack.PushPath` 改并集路径坐标 AABB（控制点含在内），mask 仍全画布。④ HiDPI 图层 RT 用逻辑宽高致右侧/下方裁切；`pushLayerSurface` 改物理 `pixmap.Width/Height`。⑤ `ApplyBlur/ApplyBlurXY/ApplyDropShadow` 未标全量合成，模糊/投影尾部被层 blit 裁掉；补 `markLayerFullComposite()`。 ⑥ `GOGPU_RENDER_MODE=cpu` 下 `ensureGPUCtx` 仍创建 GPU 上下文，CPU 契约直读 pixmap 看不到离屏层/结构化字形/滤镜结果；在 `ensureGPUCtx` 最前按 `CPUOnlyMode()` 返回。五卡 A 阴影 / B DrawColor 高级混合顺序+圆角 / C DrawPaint 渐变保路径 / D ClipBounds 四态 / E 多行 MultiFace 段落；逻辑 14/14、像素 17/17、golden 表头+图例+五卡 737760px 零差、抓屏 vs 快照同 737760px `0.0000%`（tol ±2/255）；`gpu_ops=39003`/`cpu_fallback_ops=0`/`gpu_fallbacks=0`/presents=300（五卡路由 13/0、7/0、3/0、20/0、6/0）；`present_policy=full_paint`/`vsync_source=true`；门禁 `-auto-only`/RUN_SECONDS（≥5s）+ §2.2 A–J JSON（`fps_gate=off_correctness`）。永久回归 `render/context_misc_draw_test.go`（CPU 契约）+ `render/context_misc_draw_gpu_test.go`（五 API/连续高混/隔离父层内高混/效果面高级着色）+ `render/text/layout_test.go:TestLayoutText_MultiFace`。3 项限界（`transparentOccluder` 忽略 / DrawParagraph 单色且 RTL+MultiFace 回退 / 高混每次一层）见 `RENDER_FEATURE_COVERAGE` §6。**公开 API 变更**：`Context` +5 方法；`internal/gpu.GPURenderContext.QueueAdvancedLayerComposite` 加 `dstView` 参数（internal 包，唯一调用点同批切换）。真源同步：§2 FC-CLIP-BOUNDS-LOCAL/FC-DRAW-COLOR/FC-DRAW-PAINT/FC-DRAW-SHADOW/FC-DRAW-PARAGRAPH/FF-SHADOW-PATH + §23 登记；`RENDER_FEATURE_COVERAGE.md` §1 #42 ⛔→✅、§3.1、§6 3 行、真窗计数 145→146、§11；`RENDER_API_CATALOG.md` §0/§3.12/§6.1/§7.2/§8；`ENGINE_UI_WIDGET_RENDER.md` §10。**性质=原子验证**（直调 render 公开接口一次验五 API），不算生产接线 |
| **1.45（2026-10-20）** | **A 组#14 真窗 `ui_render_base_filterops`（内容滤镜：ApplyInvert/ApplyBlurXY/ApplyImageFilterGraph/GPUFilterTexture）+ 两个 GPU 滤镜缺陷修复**：此前四 API 全仓生产零调用、无直调真窗。① GPU 反色 shader 对预乘存储做 `1−c`，半透明像素吐出大于 alpha 的通道；按 Skia/Flutter 颜色滤镜直通 alpha 口径改 `premul' = alpha − premul`（`render/internal/gpu/filter_gpu_graph.go`）。② GPU 零读回滤镜/特效发布 RT 只重放待画命令，`ClearWithColor`/CPU 回退绘制的底被首 pass Clear 丢掉；新增 `Context.pixmapBaseContent` + `GPURenderTarget.SeedFromData` + `seedViewFromRGBA`，重放前上传 CPU 底并 `MarkColorContent` 走 Load，`FlushAndFilterFromView` 加 `seed` 参数；纯 GPU 透明清屏/glow 帧保持零读回。四卡 A 反色 / B `ApplyBlurXY(0,4)` 纵糊横锐 / C `BlurY→Grayscale→Invert` 图链 / D `ApplyBlurXY(0,6)` 后 `GPUFilterTexture` 发布 199×130 视图、每帧 `DrawGPUTexture` 直绘（CPU 参考仅 4×/差异）；逻辑 16/16、像素 26/26（A 整卡 25870px 最大误差 1/255；D 对 CPU 参考最大 6/255、均值 ≤1.5/255）、golden 表头+图例+四卡 627200px 零差、抓屏 vs 快照同 627200px `0.0000%`（tol ±2/255；`xwd -id` 优先，避免遮挡）；`gpu_ops=67803`/`cpu_fallback_ops=0`/`gpu_fallbacks=0`/presents=300（四卡各 `gpu_ops=1`/`cpu_fallback_ops=0`；D 纹理 `199×130`/before=false/live paints=301；manual 16s 3 条 pointer 事件、presents=959/fps_wall≈58.31/screen=0）；`present_policy=full_paint`/`vsync_source=true`；门禁 `-auto-only`/RUN_SECONDS（≥5s）+ §2.2 A–J JSON（`fps_gate=off_correctness`）。永久回归 `render/filter_ops_contract_test.go` 3 例 + `render/filter_ops_gpu_test.go` 5 例。3 项限界（GPUFilterTexture 仅 GPU、GPU/CPU blur 非逐位、CPU 底整面上传）见 `RENDER_FEATURE_COVERAGE` §6 与本窗 README。**公开 API 变更**：`GPURenderTarget` 新增 `SeedFromData bool`；`internal/gpu.GPURenderContext.FlushAndFilterFromView` 加 `seed []byte`（internal 包签名，唯一调用点同批切换）。真源同步：§3 FP-IMAGE-FILTER/FP-INVERT + §23 登记；`RENDER_FEATURE_COVERAGE.md` §1 #40 ⛔→✅、§3.1、§6 3 行、真窗计数 144→145、§11；`RENDER_API_CATALOG.md` §1/§3.6/§7.2；`ENGINE_UI_WIDGET_RENDER.md` §10。**性质=原子验证**（直调 render 公开接口一次验一族），不算生产接线 |
| **1.44（2026-10-20）** | **A 组#13 真窗 `ui_render_base_webp`（WebP 加载/解码）+ 永久回归**：`LoadWebP` 此前全仓零消费者、无直调真窗。四卡 A 无损+Alpha（2× / 4× 裁片 / 1× PNG 参考）/ B 有损 VP8 q80（1× / 2×）/ C 四条加载路径（LoadWebP / LoadImage .webp / LoadImage .bin 内容识别 / PNG）/ D 失败模式（缺失 / 非 WebP / 空文件）。逻辑 19/19（无损 vs PNG 四通道零差、Alpha 0/部分/255=379/64/5701、四路径逐位相同、有损均值差有界、三错误）；像素 24/24（A 红绿蓝 + Alpha 128 合成 + Alpha 0 透棋盘 + PNG 参考 + 4× 裁片 / B 渐变·亮圆·暗块 + 2× 裁片 / C 四路径 + 四缩略图两两比对 3×6144=18432px 零差 / D 红绿徽章 + 卡间隙；容差 2–4/255）；golden 表头+图例+四卡 627200px 零差；抓屏 vs 快照同 627200px `0.0000%`（tol ±2/255）；`gpu_ops=105303`/`cpu_fallback_ops=0`/presents=300（manual 15s presents=899, fps_wall≈58.28, hitch=0）；`present_policy=full_paint`/`vsync_source=true`；门禁 `-auto-only`/RUN_SECONDS（≥5s）+ §2.2 A–J JSON（`fps_gate=off_correctness`）。永久回归 `render/webp_contract_test.go` 5 例；数据 `render/testdata/webp/`。2 项限界（有损跨解码器非逐位、动画未覆盖）见 `RENDER_FEATURE_COVERAGE` §6。真源同步：§7 FImg-WEBP + §23 登记；`RENDER_FEATURE_COVERAGE.md` §1 #22 ⛔→✅、§3.1、§6 2 行、真窗计数 143→144、§11；`RENDER_API_CATALOG.md` §3.5。**公开 API 无签名变更**。**性质=原子验证**（直调 render 公开接口一次验一项），不算生产接线 |
| **1.43（2026-10-20）** | **A 组#12 真窗 `ui_render_base_dither`（SetDither/Dither）+ 永久回归**：此前全仓零调用、无直调真窗。按用户口径「先补窗，限界入档」，**不改引擎语义**：实现是 `Image()`/`FlushGPU()` 读回后的整张 pixmap Bayer 后处理，窗口呈现主路 `FlushGPUWithView`/`PresentWith` 不经过它。四卡 A 线性 off/on+4×+差异×64 / B 径向 off/on / C 扫掠 off/on / D 纯色 off/on+4×+差异×64；逻辑 16/16、像素 29/29、golden 627200px 零差、抓屏 vs 快照 0.0000%（tol ±2/255）；`gpu_ops=54303`/`cpu_fallback_ops=0`/`dither_gpu_ops=12`/`dither_cpu_fallback_ops=0`/presents=300（manual 15s presents=898, fps_wall≈57.43, hitch=0）；`present_policy=full_paint`/`vsync_source=true`；门禁 `-auto-only`/RUN_SECONDS（≥5s）+ §2.2 A–J JSON（`fps_gate=off_correctness`）。永久回归 `render/dither_contract_test.go` 4 例 CPU+GPU 双绿。三限界见 `RENDER_FEATURE_COVERAGE` §6。真源同步：§1 #34 ⛔→✅、§3.1、§6 3 行、真窗计数 142→143、§11；`RENDER_API_CATALOG.md` §3.10/§7.2。**公开 API 无签名变更**。**性质=原子验证**（直调 render 接口一次验一项），不算生产接线 |
| **1.42（2026-10-20）** | **A 组#11 真窗 `ui_render_base_verticesmesh`（DrawVertices(Ex)/DrawMesh(Ex)）+ 三缺陷修复**：此前四 API 全仓生产零调用、无直调真窗。① 非空短索引旧路径回退三角列表误画 990px → 恒走索引路径，完整组不足即 skip；② 越界索引串读邻网格 3696px → `DrawMeshEx` 全量校验返回 `ErrVertsBadIndex`，旧 `DrawMesh` 跳过坏组，坏索引不再上传；③ 单三角 6B 索引上传 `COPY_BUFFER_ALIGNMENT` → 零填到 4B 倍数，`indexCount` 保真。永久 pin 7 CPU + 5 GPU（无库 skip）。四卡 A Gouraud+纯色 / B Fan×25°CTM / C indexed=expanded+单三角 / D 短索引·颜色不匹配·坏索引；逻辑 19/19、像素 29/29（17 点+12 区域，C 卡左右逐位 13144px，D 空槽 97%）、golden 627200px 零差、抓屏 vs 快照 0.0000%（tol ±2/255）；`gpu_ops=44103`/`cpu_fallback_ops=0`/presents=300；manual 15s presents=899。公开 API 无签名变更。真源同步：`RENDER_FEATURE_COVERAGE.md` §1 #32、§3.1、§6 两限界、真窗计数 141→142、§11；`RENDER_API_CATALOG.md` §3.9/§7.2。**性质=原子验证** |
| **1.41（2026-10-20）** | **A 组#10 真窗 `ui_render_base_textspacing`（`SetLetterSpacing`）+ 永久回归**：此前全仓生产零调用、无直调真窗。侦察结论：字距数学正确，**不改引擎**——最初怀疑 RTL「只加 2 个 gap」是拿零字距未整形 `MeasureWidth` 作基线的口径错误；`BuildTextLayoutExWithSpacing` 按簇加满 n-1 个 gap，RTL 标尺取 shaped layout 宽度。配套真窗四卡：A 单行 `HHHHHH` -4/0/+5/+10 实测标尺/端 tick / B CJK `间距测试`+RTL `مرحبا` 的 0/+10 间隙 / C 同宽 130px 回绕 3→5 行 / D 预热零字距缓存后 `SetLetterSpacing(8)` 的 `MeasureWidth`=TextLayout 双标尺同端 + 负字距 -5。逻辑 16/16（子进程 CPU 离屏值级：bump/单簇/回绕/负值/高度/measure·layout·cache·generation/paint/边界缓存/预热）/ 像素 37/37（GPU 快照点+区域，oracle=布局实测+调色板，容差 23/255 规则线、8/255 字形）/ golden 表头+图例+四卡 627200px 逐位零差 / **抓屏 vs 快照同 627200px `diff=0.0000%`（tol ±2/255）**；`gpu_ops=53280`/`cpu_fallback_ops=0`/presents=301、`present_policy=full_paint`/`vsync_source=true`；门禁 `-auto-only`/RUN_SECONDS（≥5s）+ §2.2 A–J JSON（`fps_gate=off_correctness`）。永久回归 `ui/rendering/spacing_test.go:TestRenderText_SpacingClusterCountsAndWrap`（六簇 +6/-4 精确 bump、单簇无尾隙、预热 measure cache、130px 回绕行数单调（正距更多行/负距更少行）、多行高度不变）；既有 measure/layout、paint、parent-cache 共 4 例绿。范围：仅单字符串路径，`Runs` 富文本留 6c。已知限界：零字距 `MeasureWidth` 仍走未整形 `Face.Advance`（kerning/RTL 与 shaped layout 有差），记 `RENDER_FEATURE_COVERAGE` §6 独立子项，本窗避开该对比。真源同步：`RENDER_FEATURE_COVERAGE.md` §1 #41 ⛔→✅、§3.1 关闭、§6 新增 1 行、真窗计数 140→141、§11 本行；本文件 §8 FT-LETTER-SPACING/FT-MEASURE + §23 登记。**公开 API 无签名变更**。**性质=原子验证**（直调 ui/rendering 门面一次验一项），不算生产接线 |
| **1.40（2026-10-20）** | **文本低频四 API 五缺陷修复 + A 组#9 真窗 `ui_render_base_textlow`**：`MeasureMultilineString`/`WordWrap`/`SetTextMode`/`SetLCDLayout` 此前无直调真窗。修复：① MSDF 轮廓方向按子路径各自判定，CFF/OTF 孔洞方向与 TrueType 相反 → 字形填成黑块；改为整字按主轮廓统一方向（`render/text/msdf/contour.go` + `generator.go`，值级回归 `TestGenerateOrientationIndependent`）。② `WordWrap` 行尾保留分隔空白、CRLF/CR 原始偏移错位；改为裁掉行尾空格且保留原始字节偏移（`render/text/wrap.go`、`render/text_entry.go`、`render/recording/recorder_path_draw.go` 三处同口径）。③ `MeasureMultilineString` 与 `DrawStringWrapped` 高度不一致；统一为 `(n-1)*lineHeight*lineSpacing + ascent + descent`（n≥1 可视块高度）。④ `GlyphMaskEngine.SetLCDLayout` 清空整个字形图集——图集按 `GlyphMaskKey.Flags` 已区分灰度/LCD-RGB/LCD-BGR，同帧混用后先前字形被覆盖成花屏；改为只清布局模板缓存、不动图集（`render/internal/gpu/glyph_mask_engine.go`；永久单测 `glyph_mask_lcd_layout_cache_test.go` 已验证加回 clear 转红）。⑤ Aliased GPU 文本未记损伤、被损伤裁剪吃掉；补 `trackTextDamage`（`render/text_gpu.go`）。永久 GPU 像素回归 `render/glyphmask_lcd_mix_test.go`（Gray→LCD 与 LCD→Gray 两序）。**公开 API 无签名变更**。配套真窗四卡：A `MeasureMultilineString` 多行度量 + `DrawStringWrapped` 三锚点 / B `WordWrap` 行宽·行尾空格·CRLF·CR / C 六种 `TextMode`（Auto/MSDF/Vector/Bitmap/GlyphMask/Aliased）/ D `LCDLayout` None·RGB·BGR 同上下文交错（花屏回归钉）；逻辑 16/16（子进程 CPU 离屏值级）、像素 28/28（GPU 快照，oracle=布局实测 + 调色板，容差 8/255 + 通道带）、golden 表头+图例+四卡 627200px 逐位零差、**抓屏 vs 快照同 627200px `diff=0.0000%`（tol ±2/255）**、`gpu_ops=29455`/`cpu_fallback_ops=0`/presents=199、`present_policy=full_paint`/`vsync_source=true`；门禁 `-auto-only`/RUN_SECONDS（≥5s）+ §2.2 A–J JSON（`fps_gate=off_correctness`）。真源同步：`RENDER_FEATURE_COVERAGE.md` §1 #30 ⛔→✅、§3.1 关闭、真窗计数 139→140、§11 本行；`RENDER_API_CATALOG.md` §2/§3.8/§7.2；本文件 §8 FT-WRAP/FT-HEIGHT/FT-LINE-METRICS + §23 登记 |
| **1.39（2026-10-19）** | **文本描边/沿路径两缺陷修复 + A 组#8 真窗 `ui_render_base_textstroke`**：`StrokeString`/`StrokeStringAnchored`/`TextPath` 此前全仓非测试零调用。侦察取证：① `Stroke()`/`StrokeString()` 只按 `path.Bounds()` 记损伤——24×24 矩形 + 线宽 12 的真实覆盖 14..50，`FrameDamageUnion=(20,20)-(44,44)`，隔离层合成只 blit damage+pad(2) → 外缘 (14,32)/(32,14) 透明丢失；按 Skia `SkStrokeRec::applyToPath`（描边 bounds = 描边转填充后的路径 bounds，含 caps/miter/dash）新增 `render/context_damage.go:trackStrokeOutlineDamage`（展开轮廓按逆 CTM 映回用户空间 AABB），`render/context_draw.go` 的 `doStroke` 在 GPU 成功分支与 CPU 分支各记一次，`Stroke()`/`StrokeString()` 撤旧记账。② `OutlineCacheKey.Size=int16(fontSize)` 截断分数——face 20.2/20.8 共用 key、20.8 轮廓 `maxX` 仍 30.2（应 ≈30.8）；改 26.6 定点 `int32` + 新增 `text.FixedSize(size float64) int32`（`render/text/glyph_cache.go`），四处 key 构建点一次切齐（`render/text_outline.go` / `render/scene/text.go` / `render/text/glyph_run.go` ×2 / `render/text/glyph_renderer.go`），删 `sizeToInt16`/`computeSizeKey`；**公开 API 变更**：`OutlineCacheKey.Size` 字段类型 int16→int32 + 新增 1 个导出函数 `FixedSize`（`scripts/apidoc` exit 0）。永久钉子 `render/stroke_damage_bounds_test.go`：`TestStrokeDamageCoversLineWidth`（损伤并集覆盖 14..50、隔离层四外缘+四外部像素保留）/ `TestTextOutlineCacheKeepsFractionalSizes`（20.2 vs 20.8 轮廓跨度 ≈30.2/≈30.8）。配套真窗四卡：A `StrokeString` 线宽 4/8/16（只描边、字形中空）/ B `StrokeStringAnchored` 锚点 (0,0)(0.5,0.5)(1,1)+十字 / C `TextPath`→`FillPath`/`StrokePath`+`DrawString(Vector)` 对照 / D `TextPath` 90° 旋转 + 20.2/20.8 两行；逻辑 14/14（子进程 CPU 离屏值级，含 `stroke_damage_covers_width`/`textpath_fractional_sizes_distinct` 两条窗级缺陷探针）、像素 21/21（GPU 快照，oracle=布局实测字形几何 + 调色板，容差 8/255 + 通道带）、golden 表头+图例+四卡 627200px 逐位零差（seed 后复跑两次 0）、**抓屏 vs 快照同 627200px `diff=0.0000%`（tol ±2/255）**、`gpu_ops≈4.1 万`/`cpu_fallback_ops=0`、`present_policy=full_paint`/`vsync_source=true`；门禁 `-auto-only`/RUN_SECONDS（≥5s）+ §2.2 A–J JSON（`fps_gate=off_correctness`）；字体硬前提：`TextPath`/`StrokeString` 在 `MultiFace` 上返回 nil，窗用 `TryLoadDefaultFace`+`NewFontSourceFromFile` 单 face。真源同步：`RENDER_FEATURE_COVERAGE.md` §1 #29 ⛔→✅、§3.1 关闭、§6 两子项改已修、真窗计数 137→139（含补记 backdrop）、§11 本行；`RENDER_API_CATALOG.md` §3.8/§6.1/§7.2 |
| **1.38（2026-10-19）** | **Backdrop 层呈现修复 + A 组#7 真窗 `ui_render_base_backdrop`**：`PushBackdropLayer` 此前无直调真窗（ui/rendering/filter_draw.go、ui/scene/composite.go 在用）。人工验窗取证：客户区**除面板外整窗黑**、快照四卡正常（像素/golden 绿=假绿）。**根因**：① 层 RT 首 pass `LoadOpClear` 清掉 CPU 上传的 seed——会话无「视图已有色彩内容」记账（快照对/实屏黑的直接原因）；② 图层内父内容只存在于父 RT，其 pixmap 是未播种池化缓冲（陈旧内容 → 整面贴回盖掉父层）；③ 滤镜在整层内容上跑，模糊越过层边界糊掉父内容。**修法**（对齐 Skia saveLayer-with-backdrop + Flutter BackdropFilterLayer）：① `render/internal/gpu/render_session.go` 新增 `MarkColorContent`/`colorContentViews`/`renderedViews`（逐帧复位），seed 视图首 pass 改 Load、嵌套返回不清屏；② `PushBackdropLayer` 抓父 RT 走 **RT→RT 快照**（新增包内 `copyGPUTextureToTarget`：忽略裁剪与层组 alpha；父无 GPU 视图时保留 pixmap seed 路径）；③ 新增 `cropFilterOutputToLayerBounds`——滤镜结果落层 pixmap 后、RT 重播种前，从父 pixmap 还原层边界外两带（Skia：saveLayer 滤镜输出裁剪到层边界）。**Pop 合成保持整面**（`drawGPUTextureWithOpacityFullSurface`）：窗口呈现只重放帧尾操作，整面贴回是把帧中物化内容送上屏的承重动作——收窄合成（scissor/UV 限界）实测客户区大面积黑（`diff=78.4313%` 确定性），恢复整面并用 `markLayerBackdrop(bounds, bounded)` 的 bounds 只裁滤镜输出。**公开 API 无签名变更**（新增助手均包内小写：`copyGPUTextureToTarget` / `clipDeviceBounds` / `cropFilterOutputToLayerBounds`；`markLayerBackdrop` 内部签名加 `(bounds image.Rectangle, bounded bool)`）。配套真窗四卡：A 基面直达压暗 / B 隔离卡毛玻璃 `Blur(6)`+15% 白 / C 裁剪面板（`ClipRect→Push→压暗→Pop→ResetClip`）/ D 隔离层内嵌套背板；逻辑 14/14、像素 21/21（GPU 快照解析 oracle，容差 8/255 + 通道带）、golden 表头+图例+四卡 627200px 逐位零差、**抓屏 vs 快照同 627200px `diff=0.0000%`（tol ±2/255）**、`gpu_ops=10776`/`cpu_fallback_ops=0`、`present_policy=full_paint`/`vsync_source=true`；门禁 `-auto-only`/RUN_SECONDS（≥5s）+ §2.2 A–J JSON（`fps_gate=off_correctness`）。永久钉子 `render/backdrop_layer_present_test.go` 4 例（seed 存活 / 裁剪背板整面合成 / 嵌套保父内容 / 边界外裁剪不越界，末例经 kill-switch 验证鉴别力）。回归：逐测试文件设备模式全绿（layer_rt_clear/blend_porterduff/context_layer/f1/f14/clip_ops_semantics/g2_damage_contract/damage_scale/mask/registry_retire/memory/ui/scene composite + `TestP1_Capability_L05_BackdropLayerGPU`）；既有红与基线一致（`TestPresentDamage_ImageMatchesView`、internal/gpu 7 例 SDF mask layout、`go vet present_frame.go:310`）。真源同步：`RENDER_FEATURE_COVERAGE.md` §1 #16 ⛔→✅、§3.1 关闭、§6 性能子项 1 项、真窗计数 137→138、§11 本行；`RENDER_API_CATALOG.md` §3.4 |
| **1.37（2026-10-19）** | **掩码 pop 呈现断裂修复 + A 组#6 真窗 `ui_render_base_maskops`**：`SetMask`/`ClearMask`/`InvertMask`/`ApplyMask`/`PushMaskLayer` 此前无直调真窗，用户 API 全仓零调用（内部仅 clip 差集上传复用 R8 管线）。人工验窗取证：客户区除指标与文字外**整窗黑**，而快照四卡正常 → 像素/golden 全绿属假绿。根因链：掩码 pop 先做 nil-view `FlushGPU()`（把本帧此前待画操作物化回 CPU pixmap）→ `render/internal/gpu/layer_gpu_io.go` CPU 掩码合成写 pixmap → `gpu_render_context.go` 只重放读回后新排操作，本帧早先排队内容再也进不了交换链（`SnapshotPath` 走收尾独立重绘，故快照不黑）。修法（对齐 Skia saveLayer-with-mask：掩码是合成覆盖、不在帧中回读）：① `ModulateMaskedLayerGPU`（R8 调制 pass `maskR8ModulateViewToTexture`）+ `queueMaskedLayerGPU` 把掩码 pop 改成 GPU 可重放合成（无读回 / 无 nil-view flush，CPU 回落保留，掩码面按源全尺寸构建越界填 0）；② 调制 pass 采样图层 RT 前先 `FlushLeadingSubmitsOnly()`（opt21 延迟提交队列，同 RenderFrameGrouped C5 drain 契约）；③ `encodeBlitOnlyPass` 原对任何目标用不透明黑清屏（A=1），嵌套隔离层（纯 `PushLayerIsolated` 亦可复现）把父层 RT 首帧清黑、blit 只覆盖局部 → 父层全域黑；现由 `FlushOffscreen`/`flushLayerViewGPU`/`gpuFlushOffscreenTarget` 标记离屏目标（`SetOffscreenBlitTarget`，internal）清 A=0（Skia saveLayer 透明底），**窗口表面仍清不透明黑**；④ 永久钉子 `render/layer_rt_clear_test.go` 两例（嵌套隔离层透明底 / 嵌套掩码层只留交集）。**公开 API 无签名变更**（`GPURenderTarget` 未加字段，新入口均在 `render/internal/gpu` 与包内助手）。配套真窗：四卡 A `ApplyMask` 后处理 ramp 255/170/85/0（quad-bounded，DstIn）/ B `SetMask`×`ClearMask` 复原 / C `InvertMask` 原地反转 / D `PushMaskLayer` 圆掩码图层；逻辑 14/14（子进程 CPU 离屏值级）、像素 18/18（GPU 快照解析 oracle，容差 8/255 + 通道带）、golden 表头+图例+四卡 627200px 逐位零差、**抓屏 vs 快照同 627200px `diff=0.0000%`（tol ±2/255；`xwininfo` 客户区 + `ffmpeg x11grab`，门禁强制，抓屏失败/尺寸不符/遮挡即红）**、`gpu_ops=11267`/`cpu_fallback_ops=0`、`present_policy=full_paint`/`vsync_source=true`；门禁 `-auto-only`/RUN_SECONDS（≥5s）+ §2.2 A–J JSON；人工常驻窗 `-manual-seconds` 复验 screen=true。Golden 重播种（评审记录）：D 卡圆边 1105px 全为 \|Δ\|=1/255（GPU 合成 vs 旧 CPU 整数 source-over 舍入）+ 图例新增 screen 行，重播两次 `diff=0.0000%` 后接受。回归：逐测试文件设备模式全绿（mask/context_layer/layer_rt_clear/p01/p12/s6_4×2/f1/f14/blend_porterduff/p03/p13/present_record/present_resize/pass_scratch/opt18/r7_4/r7_2/context_image/images_gpu_visual/y_orientation/mem_complex_offscreen/p1_gpu_fixed_pixel/s6_1-3）、`ui_render_base_clipop` golden 0.0000%/627200px 不回归；既有红与 stash 基线逐条一致（`TestPresentDamage_ImageMatchesView`、internal/gpu 7 例 SDF mask layout、S69/S3c/ClipCircle/StencilRenderer/clipping 严格档 2 区）。真源同步：`RENDER_FEATURE_COVERAGE.md` §1 #14 ⛔→✅、§3.1 关闭、§6 性能子项 1 项、真窗计数 136→137、§11 本行；`RENDER_API_CATALOG.md` §3.3/§3.4/§7.2（掩码族按口径降级 🔌 + 真窗原子验证证据） |
| **1.36（2026-10-19）** | **裁剪运算四缺陷修复 + A 组#5 真窗 `ui_render_base_clipop`**：`ClipRectOp`/`ClipPathOp` 的 difference/replace 此前全仓非测试零调用。CPU/GPU 探针程序三路复现确认：① 多子路径裁剪 mask 损坏（同向嵌套路径差集应实心扣除、实吐环状——`render/internal/clip/mask.go` 把每个子路径当独立 mask 叠加）② 裁剪路径 fill rule 恒 even-odd（`Clip()`/`ClipPathOp` 不吃 `SetFillRule`，NonZero 嵌套同向路径中心被误挖）③ 矩形差集整数取整无 AA（分数边 hard edge）。修法：重写 `mask.go` 多子路径联合光栅化（winding 累积 + even-odd 解析覆盖）、`Clip()/ClipPreserve()/ClipRectOp/ClipPathOp` 透传 `c.paint.FillRule`（新增 `Context.gpuClipFillRule`；**签名变更** `PathClipAware.SetClipPath(path, fillRule)` 同批原子切换，GPU 深度裁剪 fill rule 一并透传）、`render/internal/clip/difference.go` 矩形差集改解析面积覆盖。第四个缺陷在验收实测中暴露：分数覆盖被应用两次（值呈平方 64→16、191→112）——GPU 走图路线 `fillMaskedAsImage` 把 `ClipCoverage` 烘焙进 src 后被 R8 mask 平面再乘一次（修：src 只栅格形状，裁剪由 GPU 侧 scissor/clipBG/深度裁剪/mask 平面执行）；CPU 回退路线 `installGPUClipMask` 写入的合成 mask 与软渲染器 `ClipCoverage` 再乘一次（修：CPU 回退前 `MaskCoverage` 归一化回用户 mask、`applyMaskToPaint` 每次重置防嵌套残留）。新增回归 `render/clip_ops_semantics_test.go`（8 项，含 CPU 回退变体）+ `render/internal/clip/mask_fillrule_test.go`（4 项值级）。配套真窗三证据：逻辑 14/14、像素 23/23（GPU 路线 `gpu_ops=4018 cpu_fallback_ops=0`）、golden 表头+图例+四卡 627200px 逐位零差（seed 后复跑 `diff=0.0000%`）；门禁 `-auto-only`/RUN_SECONDS（≥5s）+ §2.2 A–J JSON（`present_policy=full_paint`、`vsync_source=true`）。已知限界 2 项（深度裁剪 even-odd 在 sampleCount>1 退化 NonZero；分数坐标 intersect 矩形裁剪仍走 scissor）记 `RENDER_FEATURE_COVERAGE.md` §6 与本窗 README |
| **1.35（2026-10-19）** | **Sweep tile Extend 修正 + A 组#4 真窗 `ui_render_base_sweepgradient`**：非 Pad `Extend` 在部分扫掠（end≠2π）下范围外区域被钳成 Pad——D 卡 Repeat/Reflect 圆在 112.5°/157.5° 实测与 Pad 相同（钳到末端蓝），CPU oracle（同参数 `ColorAt`）给出 frac/triangle 值；永久测试复现 `repeat@112.5°` 读 (5,0,254) vs 模型 (99,0,240)。根因：1D ramp 覆盖路径（`render/internal/gpu/brush_fill.go:fillSweepGradientFieldMasked`）的 `tMin/tMax` 角域收窄对所有 Extend 生效，shader `(t-tMin)*invSpan` 再钳 [0,1] → 范围外 raw t 恒落 ramp 边缘 = Pad 行为。修法（Skia tile-on-t 口径）：非 Pad 时 ramp 固定整周期 [0,1]，cover FS 对 raw t 折叠（repeat `t-floor(t)`、reflect 三角波）；Pad 原角域收窄保留、整圈折叠恒等 → Pad 与整圈输出逐位不变；linear/radial 调用点补 extend=0 不受影响。改动 `brush_fill.go` / `session_textured_cover.go` / `render_session.go`（`TexExtend`）+ 三份 WGSL（`shaders/cover_textured_linear.wgsl`、`textured_stencil_linear.go`、`linear_ramp_mask.go` 均加 `sweep_tile()`）。新增永久回归 `render/p1_capability_matrix_closers_test.go:TestP1_Capability_D03_SweepGradientTileExtendGPU`（GPU 判别采样 tol 12/255）。配套真窗三证据：逻辑 14/14、像素 22/22、golden 627200px 逐位零差（seed 后第 2/3/4 跑 `diff=0.0000%`）；门禁 `-auto-only`/RUN_SECONDS（≥5s）+ §2.2 A–J JSON（`present_policy=full_paint`、`vsync_source=true`、`cpu_fallback_ops=0`）+ 人工常驻窗实屏截图（`close-requested` 关窗汇总 presents=3651）；性能子项 1 项（AA 矩形 sweep/radial 每帧 CPU 重建 ColorAt 场，`brush_native.go:155→593→614`）记 `RENDER_FEATURE_COVERAGE.md` §6 与本窗 README |
| **1.34（2026-10-19）** | **描边 miter 拐角符号修正 + A 组#3 真窗 `ui_render_base_strokepreset`**：`render/internal/stroke/expander.go:applyOuterMiterJoin` 的 miter 交点分母用无符号 `crossAbs`，丢掉转向符号 → 「下→右」一类拐角的尖点镜像到错误一侧（探针实测尖点应为两外偏移线交点 `(56,145)`、实吐 `(65,145)`），轮廓自交后直角被一条角度不对的斜边切掉（真窗像素同位置纯背景 + 邻格 81% 覆盖，与轮廓打印互证）；按 Skia 带符号 cross 求交点口径改 `t := Δ.Cross(cd)/ab.Cross(cd)`、`miterPt := fpLast + ab*t`，公开签名未变、另一转向输出逐点不变。新增正反转向值级回归 `TestStrokeExpander_MiterJoinTipBothDirections`（修前红→修后绿）；配套真窗三证据（逻辑 12/12、像素 28/28、golden 627200px 逐位零差）并登记 §23；`ui_render_base_pathshape` golden 因本修复重播（0.0002%/1px），`ui_render_base_pathlow` 不受影响 |
| **1.33（三线 T 立项 2026-09-16，未开工）** | **三线重设计立项（唯一文档 `ENGINE_THREAD_T_PLAN.md`，此处只留指针）** |
| **1.32（E2 已修 2026-09-15；E1 已撤销）** | **动画帧 CPU 结构问题 E2 闭环、E1 撤销**：E2 一帧多遍 walk 已修——一次建包顺手出四个数（边界数/深度、操作数、字命中），`ConsumeNeedsPaint` 原位不动，包结构不动。实测 120 节点只脏 1 叶老序列 `Children` 603 次 → 新序列 361 次；数诚实 + 包相等由 `TestE2Merged_CountsHonestAndPacketEqual` 锁死，老证据 `TestEVerify_E2_OldEvidence` 仍通过（只调老代码）。E1 证伪撤销：全量重建是对齐 Flutter 的认账现状（同 B1），稀疏包修法丢静按钮已退回作废。E4（任务槽多执行一次）经 20 万次 hammer + 存取时序推演证伪，不修 |
| **1.31** | **序 14 收口**：F1（淘汰链表化 + 模版粘性上传）与 F2（用户空间几何 + 变换 uniform + 描边快路径）已交付；验收数见 §22.2a；公开 API 未动 |
| **1.30** | **序 14 立项**：§22.2a 几何管线性能结构（F1/F2 两期，未开工；鹈鹕偶发卡顿诊断结论与证据落位；无用户故直接重写不留双路） |
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
