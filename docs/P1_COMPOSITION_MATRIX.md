# 阶段 A — 任意组合维度矩阵（Composition Matrix）

> 版本：1.11 | 日期：2026-07-15  
> 主线：[`MAINLINE_PLAN.md`](./MAINLINE_PLAN.md) v1.45+  
> 能力表：[`SKIA_2D_CAPABILITY_MATRIX.md`](./SKIA_2D_CAPABILITY_MATRIX.md)  
> 形态密度（旁证）：[`P1_COMPLEX_UI_MATRIX.md`](./P1_COMPLEX_UI_MATRIX.md)  
> 架构：`render → gpu/webgpu → gpu/rwgpu → libwgpu_native`

## 定位

| 是 | 不是 |
|----|------|
| 验证 **任意组合** 图元/状态交叉后像素与 GPU 链路仍正确 | Ant Design / 控件层实现 |
| 用维度轴覆盖场景空间（可扩展、可组合） | 固定产品 UI 清单打勾即完工 |
| S4 之前的正确性/覆盖度门禁 | 性能数字门槛 |

复杂 UI Tier A–U 是 **形态密度采样**；本矩阵是 **组合完备性** 主轴。目标：render 层 primitive + convenience API 足以支撑任意场景拼装，而不是只覆盖 antd 命名场景。

## 硬规则

1. `WGPU_NATIVE_PATH` 真库；`GPUOps > 0`  
2. 无 silent CPU-only 冒充 GPU 完成  
3. 关键结构像素 / 内外区域检查  
4. 性能数字 **不** 作为 A 关闭条件（留给 S4.0）  
5. 发现 ABI/facade 缺口：回 S1/S2 补，再继续 A  

## 维度轴（可增不可假关闭）

| 轴 | 代表 API / 语义 |
|----|-----------------|
| **clip** | `ClipRect` / `ClipRoundRect` / path `Clip` 嵌套 |
| **layer** | `PushLayer` / `PopLayer` 透明度与嵌套 |
| **blend** | `SetBlendMode` / layer blend（SrcOver/Plus/Multiply…） |
| **text** | `DrawString` / 图集路径 |
| **image** | `DrawImage` / `DrawImageQuad` / 可选 `DrawGPUTexture` |
| **transform** | `Translate` / `Scale` / `Rotate` + CTM 下 clip/fill |
| **HiDPI** | `WithDeviceScale` + hairline / 文本 |
| **backdrop/damage** | `PushBackdropLayer`；多区域重绘（脏区语义预热，S4.4 再优化） |
| **mask** | `SetMask` / `PushMaskLayer` |
| **mesh/atlas** | `DrawVertices` / `DrawMesh` / `DrawAtlas` |
| **path effects** | `WithCorners` / `Discrete` / `Trim` / dash / caps/joins |
| **gradient/pattern** | `SetFillBrush` 渐变 / `CreateImagePattern` |
| **filter** | `ApplyDropShadow` / `ApplyBlur` / `SetDither` |
| **external tex** | `CreateOffscreenTexture` + `DrawGPUTexture` / `WithOpacity` |
| **filter graph** | `ApplyImageFilterGraph` / gray / invert / BlurXY / ColorMatrix |
| **write pixels** | `WritePixels` 局部保留更新 |
| **advanced blend** | Overlay / SoftLight / Hue / Difference / ColorBurn / Exclusion |
| **rich text** | `DrawStringWrapped` / `Anchored` / `StrokeString` |
| **text mode** | `TextModeGlyphMask` / `TextModeVector` / `DrawShapedGlyphs` |
| **multi-context** | 多 `Context` 离屏纹理回合成 |
| **frame damage** | `FrameDamage` / `PresentFrameDamage` |
| **resize** | 运行中 `Resize` 后重绘 |

组合写法：`Dxx = 轴1 × 轴2 × …`，每条探针至少 **3 轴交叉**。复杂场景可叠加 5+ 轴。

## 探针状态

> 共 **105** 条；真 GPU 门禁；非控件层。

| ID | 交叉 | 场景意图 | 门禁 | 状态 |
|----|------|----------|------|------|
| D01 | clip × layer × text | 嵌套矩形 clip + 半透明层 + 标签 | `TestP1_Comp_D01_ClipLayerText` | ✅ |
| D02 | clip × image × blend | 圆角 clip 内图像 + Plus 叠色 | `TestP1_Comp_D02_ClipImageBlend` | ✅ |
| D03 | clipPath × layer × fill | 多边形 path clip + 层内填充 | `TestP1_Comp_D03_ClipPathLayerFill` | ✅ |
| D04 | HiDPI × hairline × text | DPR=2 下 hairline 与文字 | `TestP1_Comp_D04_HiDPIHairlineText` | ✅ |
| D05 | layer × blend × clip | 外 clip + Multiply 层叠 | `TestP1_Comp_D05_LayerBlendClip` | ✅ |
| D06 | image × text × clip × backdrop | 内容 + 字 + backdrop | `TestP1_Comp_D06_ImageTextClipBackdrop` | ✅ |
| D07 | transform × clip × fill | Translate/Scale 下 clip 填充 | `TestP1_Comp_D07_TransformClipFill` | ✅ |
| D08 | multi-region redraw | 多脏区局部重绘 | `TestP1_Comp_D08_MultiRegionRedraw` | ✅ |
| D09 | dash × clip × text | 虚线描边 + 嵌套 clip | `TestP1_Comp_D09_DashStrokeClipText` | ✅ |
| D10 | gradient × clipRRect × layer | 多 stop 渐变 + 圆角 clip | `TestP1_Comp_D10_GradientClipLayer` | ✅ |
| D11 | evenOdd × layer × blend | 孔洞填充 + Multiply | `TestP1_Comp_D11_EvenOddLayerBlend` | ✅ |
| D12 | mask × fill × image | alpha mask 调制 + 底图 | `TestP1_Comp_D12_MaskFillImage` | ✅ |
| D13 | maskLayer × text × clip | PushMaskLayer + 文字 | `TestP1_Comp_D13_MaskLayerTextClip` | ✅ |
| D14 | vertices × clip × blend | 彩色 mesh + Plus | `TestP1_Comp_D14_VerticesClipBlend` | ✅ |
| D15 | atlas × HiDPI × clip | DrawAtlas 精灵 + DPR | `TestP1_Comp_D15_AtlasHiDPIClip` | ✅ |
| D16 | mesh × transform × layer | 索引 mesh + CTM + 层 | `TestP1_Comp_D16_MeshTransformLayer` | ✅ |
| D17 | imageQuad × clipPath × text | 任意四边形图 + path clip | `TestP1_Comp_D17_ImageQuadClipText` | ✅ |
| D18 | pathEffects × stroke × clip | corners/discrete/trim | `TestP1_Comp_D18_PathEffectsClipStroke` | ✅ |
| D19 | deepNest clip×layer×image×text | 三层 clip + 宫格图文 | `TestP1_Comp_D19_DeepNestClipLayerImageText` | ✅ |
| D20 | multiBlend × clip × image | Multiply/Screen/Plus 栈 | `TestP1_Comp_D20_MultiBlendClipImage` | ✅ |
| D21 | externalTex × clip × backdrop | 离屏纹理 + 灯箱 | `TestP1_Comp_D21_ExternalTexClipBackdropText` | ✅ |
| D22 | shadow × blur × dense | DropShadow + Blur 卡片 | `TestP1_Comp_D22_ShadowBlurDenseScene` | ✅ |
| D23 | dither × gradient × HiDPI | 有序抖动 + 渐变 + DPR | `TestP1_Comp_D23_DitherGradientHiDPI` | ✅ |
| D24 | scrollClip × translate × text | 视口 clip + 平移列表 | `TestP1_Comp_D24_ScrollClipTranslateText` | ✅ |
| D25 | nestedLayer × multiBlend | 多层 + Multiply/Screen | `TestP1_Comp_D25_DeepNestedBlendLayers` | ✅ |
| D26 | caps/joins × dash × pathClip | 端点/拐角/虚线 | `TestP1_Comp_D26_CapsJoinsDashPathClip` | ✅ |
| D27 | imageRounded × mask × layer | 圆角/圆形图 + mask | `TestP1_Comp_D27_ImageRoundedMaskLayer` | ✅ |
| D28 | damage × gradient × text × image | 多脏区 + 渐变头 | `TestP1_Comp_D28_DamageGradientTextImage` | ✅ |
| D29 | rotate × clip × image × stroke | 旋转 CTM + clip | `TestP1_Comp_D29_RotateClipImageStroke` | ✅ |
| D30 | virtualList primitives | 列表密度组合 | `TestP1_Comp_D30_VirtualListPrimitiveDensity` | ✅ |
| D31 | lattice stress blend×clip | 80 格交替 blend | `TestP1_Comp_D31_LatticeStressBlendClip` | ✅ |
| D32 | pattern × transform × clip | 图像 pattern 填充 | `TestP1_Comp_D32_PatternTransformClip` | ✅ |
| D33 | editor multi-pane | 树+代码+gutter+selection | `TestP1_Comp_D33_EditorMultiPaneComposition` | ✅ |
| D34 | chart primitives | 网格+柱+折线+图例 | `TestP1_Comp_D34_ChartPrimitiveComposition` | ✅ |
| D35 | calendar grid | 多格+高亮+事件条 | `TestP1_Comp_D35_CalendarGridComposition` | ✅ |
| D36 | kitchen-sink max mix | 近全轴同场景 | `TestP1_Comp_D36_KitchenSinkMaxMix` | ✅ |
| D37 | filterGraph gray×invert | 灰度+反相滤波图 | `TestP1_Comp_D37_FilterGraphColorOpsClip` | ✅ |
| D38 | DrawImageEx × clip × text | srcRect/opacity/blend 图像 | `TestP1_Comp_D38_DrawImageExClipText` | ✅ |
| D39 | radial+sweep gradient panels | 径向/锥形渐变面板 | `TestP1_Comp_D39_RadialSweepGradientPanels` | ✅ |
| D40 | Overlay/SoftLight/Hue blend | 高级混合 × 图 × 字 | `TestP1_Comp_D40_AdvancedBlendImageText` | ✅ |
| D41 | fill+stroke pattern × CTM | 填充/描边 pattern | `TestP1_Comp_D41_FillStrokePatternTransform` | ✅ |
| D42 | BlurXY+DropShadow graph | 各向异性模糊+阴影图 | `TestP1_Comp_D42_BlurXYShadowGraphDense` | ✅ |
| D43 | InvertMask × layer × text | 反掩码 + 层 | `TestP1_Comp_D43_InvertMaskLayerText` | ✅ |
| D44 | external opacity × damage | 离屏透明度+多脏区 | `TestP1_Comp_D44_ExternalOpacityDamageRects` | ✅ |
| D45 | WritePixels retained panels | 像素写徽章+局部更新 | `TestP1_Comp_D45_WritePixelsRetainedPanels` | ✅ |
| D46 | filter multi-node scene | blur+colorMatrix 图 | `TestP1_Comp_D46_FilterGraphMultiNodeScene` | ✅ |
| D47 | kanban density | 多列卡片徽章层 | `TestP1_Comp_D47_KanbanPrimitiveDensity` | ✅ |
| D48 | scroll+sticky+modal | 嵌套滚动+粘性+模态 | `TestP1_Comp_D48_NestedScrollStickyModal` | ✅ |
| D49 | HiDPI app chrome | DPR=2 应用壳密度 | `TestP1_Comp_D49_HiDPIAppChromeDensity` | ✅ |
| D50 | multi-CTM mesh×text×clip | 旋转缩放平移 mesh | `TestP1_Comp_D50_MultiCTMMeshTextClip` | ✅ |
| D51 | spreadsheet grid | 冻结窗格+选区+网格 | `TestP1_Comp_D51_SpreadsheetGridComposition` | ✅ |
| D52 | media timeline | 胶片+波形+playhead | `TestP1_Comp_D52_MediaTimelineComposition` | ✅ |
| D53 | form wizard | 步骤条+表单+popover | `TestP1_Comp_D53_FormWizardComposition` | ✅ |
| D54 | tree split view | 树+预览+查找条 | `TestP1_Comp_D54_TreeSplitViewComposition` | ✅ |
| D55 | cascader columns | 多列级联面板 | `TestP1_Comp_D55_CascaderColumnsComposition` | ✅ |
| D56 | notification stack | toast 层叠+角标 mesh | `TestP1_Comp_D56_NotificationStackComposition` | ✅ |
| D57 | transfer dual list | 双列表穿梭 | `TestP1_Comp_D57_TransferDualListComposition` | ✅ |
| D58 | color picker | SV 方+色相条+预览 | `TestP1_Comp_D58_ColorPickerComposition` | ✅ |
| D59 | rich text × clip × layer | wrap/anchor/stroke 文本+卡片 | `TestP1_Comp_D59_RichTextClipLayerStack` | ✅ |
| D60 | Difference/Burn/Exclusion | 高级 blend 三连 × image | `TestP1_Comp_D60_DiffBurnExclusionBlendStack` | ✅ |
| D61 | AA × dashOffset × miter | 抗锯齿开关+虚线相位+尖角 | `TestP1_Comp_D61_AADashOffsetMiterClip` | ✅ |
| D62 | Resize × recompose | 中途 Resize 后多面板重绘 | `TestP1_Comp_D62_ResizeRecomposePanels` | ✅ |
| D63 | FrameDamage × PresentDamage | 帧脏区累计+单 rect present | `TestP1_Comp_D63_FrameDamageSingleRectPresent` | ✅ |
| D64 | MaskFromAlpha × layer | 快照 alpha 掩码+图文 | `TestP1_Comp_D64_MaskFromAlphaLayerImageText` | ✅ |
| D65 | infinite canvas pan/zoom | 平移缩放网格+节点连线 | `TestP1_Comp_D65_InfiniteCanvasPanZoom` | ✅ |
| D66 | chat bubbles | 气泡+头像+composer | `TestP1_Comp_D66_ChatBubbleComposition` | ✅ |
| D67 | gantt chart | 任务条+冻结列+today 线 | `TestP1_Comp_D67_GanttChartComposition` | ✅ |
| D68 | heatmap × tooltip | 热力格+色条+提示层 | `TestP1_Comp_D68_HeatmapTooltipComposition` | ✅ |
| D69 | multi-modal stack | 双 backdrop 嵌套模态 | `TestP1_Comp_D69_MultiModalStackComposition` | ✅ |
| D70 | map tiles × route | 瓦片+路径+POI 弹层 | `TestP1_Comp_D70_MapTilesRoutePopup` | ✅ |
| D71 | code diff | 增减行着色+gutter | `TestP1_Comp_D71_CodeDiffComposition` | ✅ |
| D72 | bicubic × path clip × pattern | 双三次缩放+pattern 描边 | `TestP1_Comp_D72_BicubicImagePathClipPattern` | ✅ |
| D73 | IDE dock layout | 活动栏+侧栏+编辑器+终端 | `TestP1_Comp_D73_IDEDockLayoutComposition` | ✅ |
| D74 | filter×mask×blend×text mega | 多轴同场景滤波 | `TestP1_Comp_D74_FilterMaskBlendTextMega` | ✅ |
| D75 | dashboard KPI/sparkline/table | KPI+折线+表+LIVE 脏更 | `TestP1_Comp_D75_DashboardKPISparklineTable` | ✅ |
| D76 | shaped glyphs × clip × glyphmask | Shape+DrawShapedGlyphs+层 | `TestP1_Comp_D76_ShapedGlyphsClipLayerMode` | ✅ |
| D77 | vector text × stroke × gradient | TextModeVector+描边字 | `TestP1_Comp_D77_VectorTextStrokeGradientClip` | ✅ |
| D78 | carousel stage | ImageQuad 轮播+caption | `TestP1_Comp_D78_CarouselStageComposition` | ✅ |
| D79 | video player chrome | 进度/控件/scrub 预览 | `TestP1_Comp_D79_VideoPlayerChromeComposition` | ✅ |
| D80 | org chart | 节点连线+选中层 | `TestP1_Comp_D80_OrgChartComposition` | ✅ |
| D81 | mindmap radial | 径向分支 Discrete 路径 | `TestP1_Comp_D81_MindmapRadialComposition` | ✅ |
| D82 | candlestick chart | K线+成交量+MA+十字 | `TestP1_Comp_D82_CandlestickChartComposition` | ✅ |
| D83 | isometric tiles | 等距体块+高亮层 | `TestP1_Comp_D83_IsometricTileComposition` | ✅ |
| D84 | watermark × mask badge | 水印层+机密徽章 | `TestP1_Comp_D84_WatermarkMaskBadgeComposition` | ✅ |
| D85 | multi-context textures | 多 Context 离屏合成 | `TestP1_Comp_D85_MultiContextTextureComposite` | ✅ |
| D86 | settings dense form | Tab+开关+滑条 | `TestP1_Comp_D86_SettingsDenseFormComposition` | ✅ |
| D87 | particle field HiDPI | 大量点/圆+Plus+DPR | `TestP1_Comp_D87_ParticleFieldHiDPIComposition` | ✅ |
| D88 | nested EvenOdd × pattern | 双孔 EvenOdd+pattern | `TestP1_Comp_D88_NestedEvenOddPatternStroke` | ✅ |
| D89 | split editor/terminal | 分栏+sash+焦点环 | `TestP1_Comp_D89_SplitEditorTerminalComposition` | ✅ |
| D90 | kitchen-sink v2 | mesh/atlas/text/blend/filter 再混合 | `TestP1_Comp_D90_KitchenSinkV2MaxMix` | ✅ |
| D91 | ClipPreserve fill+stroke | 同 path 裁剪+填充+描边+层 | `TestP1_Comp_D91_ClipPreserveFillStrokeText` | ✅ |
| D92 | grayscale × colorMatrix | 对比矩阵后灰度 dense UI | `TestP1_Comp_D92_GrayscaleColorMatrixDense` | ✅ |
| D93 | GPUTextureBase × opacity | Base/opacity 离屏纹理叠层 | `TestP1_Comp_D93_GPUTextureBaseOpacityClip` | ✅ |
| D94 | PresentFrame complex | 复杂场景 PresentFrame e2e | `TestP1_Comp_D94_PresentFrameComplexScene` | ✅ |
| D95 | text mode mix | GlyphMask/Vector/MSDF/Bitmap | `TestP1_Comp_D95_TextModeMixClipLayer` | ✅ |
| D96 | file manager density | 树+图标网格+路径条 | `TestP1_Comp_D96_FileManagerDensityComposition` | ✅ |
| D97 | email compose | chips+wrap body+附件+Send | `TestP1_Comp_D97_EmailComposeComposition` | ✅ |
| D98 | swimlane board | 泳道卡片+拖拽幽灵 | `TestP1_Comp_D98_SwimlaneBoardComposition` | ✅ |
| D99 | radar chart | 雷达网+填充+图例 clip | `TestP1_Comp_D99_RadarChartComposition` | ✅ |
| D100 | picture-in-picture | 主舞台+浮窗+resize handle | `TestP1_Comp_D100_PictureInPictureComposition` | ✅ |
| D101 | double scroll sticky chips | 双滚动+粘性 chips+FAB | `TestP1_Comp_D101_DoubleScrollStickyChipsFAB` | ✅ |
| D102 | multi-resize recompose | 多次 Resize 后重组 | `TestP1_Comp_D102_MultiResizeRecomposeStress` | ✅ |
| D103 | hybrid damage updates | WritePixels+脏区+外纹理 | `TestP1_Comp_D103_HybridDamageWritePixelsTexture` | ✅ |
| D104 | design canvas annotations | 参考线+选区手柄 | `TestP1_Comp_D104_DesignCanvasAnnotations` | ✅ |
| D105 | kitchen-sink v3 stress | 列布局+纹理+filter+脏徽章 | `TestP1_Comp_D105_KitchenSinkV3Stress` | ✅ |
| D106 | music player | 封面+频谱+进度+队列 clip | `TestP1_Comp_D106_MusicPlayerComposition` | ✅ |
| D107 | three-pane shell | 导航/列表/详情三栏 | `TestP1_Comp_D107_ThreePaneShellComposition` | ✅ |
| D108 | PR review | diff hunks+批注+状态 | `TestP1_Comp_D108_PRReviewComposition` | ✅ |
| D109 | week calendar | 周视图+事件块+弹层 | `TestP1_Comp_D109_WeekCalendarComposition` | ✅ |
| D110 | network graph | 节点边+选中+小地图 | `TestP1_Comp_D110_NetworkGraphComposition` | ✅ |
| D111 | image editor | 画布+工具条+直方图 | `TestP1_Comp_D111_ImageEditorComposition` | ✅ |
| D112 | checkout density | 表单密排+汇总卡 | `TestP1_Comp_D112_CheckoutDensityComposition` | ✅ |
| D113 | notification drawer | 抽屉+未读点+遮罩 | `TestP1_Comp_D113_NotificationDrawerComposition` | ✅ |
| D114 | multi-tab terminal | tabs×buffer×selection | `TestP1_Comp_D114_MultiTabTerminalComposition` | ✅ |
| D115 | markdown split | 编辑/预览+code fence | `TestP1_Comp_D115_MarkdownSplitPreviewComposition` | ✅ |
| D116 | datagrid multi-select | 冻结列+多选 | `TestP1_Comp_D116_DataGridFrozenMultiSelect` | ✅ |
| D117 | floating toolbar | 选区+悬浮工具条 | `TestP1_Comp_D117_FloatingToolbarSelection` | ✅ |
| D118 | rotate mask blur shadow | 变换×mask×blur×shadow | `TestP1_Comp_D118_RotateMaskBlurShadowStack` | ✅ |
| D119 | fan mesh overlay | mesh扇区+环形图案 | `TestP1_Comp_D119_FanMeshOverlayCircularPattern` | ✅ |
| D120 | stress lattice | 12×16 clip/layer/blend/text | `TestP1_Comp_D120_StressLatticeNestedAxes` | ✅ |
| D121 | modal stack | scrim×叠对话框×焦点环 | `TestP1_Comp_D121_ModalStackComposition` | ✅ |
| D122 | multi-column article | 分栏×drop-cap×引文 | `TestP1_Comp_D122_MultiColumnArticleComposition` | ✅ |
| D123 | CAD blueprint | 网格×虚线尺寸×标注 | `TestP1_Comp_D123_CADBlueprintComposition` | ✅ |
| D124 | video call mosaic | 多画面×发言边框×控制条 | `TestP1_Comp_D124_VideoCallMosaicComposition` | ✅ |
| D125 | spreadsheet multi-range | 公式栏×冻结×多选区 | `TestP1_Comp_D125_SpreadsheetFrozenMultiRange` | ✅ |
| D126 | timeline scrubber | 轨道×关键帧×波形×playhead | `TestP1_Comp_D126_TimelineScrubberComposition` | ✅ |
| D127 | nested scroll sticky | 粘性头/列×浮动选区 | `TestP1_Comp_D127_NestedScrollStickySelection` | ✅ |
| D128 | color picker panel | 色相条×SV×色板×alpha | `TestP1_Comp_D128_ColorPickerPanelComposition` | ✅ |
| D129 | isometric board | 等距层×连接线×标签 | `TestP1_Comp_D129_IsometricBoardComposition` | ✅ |
| D130 | multi-doc IDE | tabs×分栏×minimap×problems | `TestP1_Comp_D130_MultiDocIDEComposition` | ✅ |
| D131 | deep transform chain | rotate×scale×translate×clip×text | `TestP1_Comp_D131_DeepTransformChainClipText` | ✅ |
| D132 | advanced blend cascade | 多 blend 条带叠底图 | `TestP1_Comp_D132_AdvancedBlendCascadeStrip` | ✅ |
| D133 | filter graph chain | blur×color matrix 后叠徽章 | `TestP1_Comp_D133_FilterGraphChainComposition` | ✅ |
| D134 | masked gradient particles | mask×渐变×粒子×label | `TestP1_Comp_D134_MaskedGradientParticlesLabel` | ✅ |
| D135 | infinite canvas frames | 多 frame×连接×手柄 | `TestP1_Comp_D135_InfiniteCanvasFramesComposition` | ✅ |
| D136 | HiDPI switch mid | Scale 中途切换×重绘 | `TestP1_Comp_D136_HiDPISwitchMidComposition` | ✅ |
| D137 | multi-pass damage | WritePixels 戳记×overlay | `TestP1_Comp_D137_MultiPassDamageStampComposition` | ✅ |
| D138 | mixed text modes clip | 多 TextMode×layer×path clip | `TestP1_Comp_D138_MixedTextModesLayerClip` | ✅ |
| D139 | pattern dash image clip | hatch×dash×image×text plate | `TestP1_Comp_D139_PatternDashImageClipText` | ✅ |
| D140 | kitchen-sink v4 stress | 导航+lattice+inspector+toast+filter | `TestP1_Comp_D140_KitchenSinkV4Stress` | ✅ |


| D141 | command palette workspace | scrim×命令列表×快捷键×选中 | `TestP1_Comp_D141_CommandPaletteOverWorkspace` | ✅ |
| D142 | nested popover cascade | 菜单×子菜单×tip | `TestP1_Comp_D142_NestedPopoverCascade` | ✅ |
| D143 | dual viewport mock | 双视口独立 clip×共享 chrome | `TestP1_Comp_D143_DualViewportMockComposition` | ✅ |
| D144 | sankey flow density | 流带×节点×图例 | `TestP1_Comp_D144_SankeyFlowDensityComposition` | ✅ |
| D145 | map tiles route markers | 瓦片×路线×标记×罗盘 | `TestP1_Comp_D145_MapTilesRouteMarkersComposition` | ✅ |
| D146 | trading terminal | 盘口×曲线×成交×徽章 | `TestP1_Comp_D146_TradingTerminalComposition` | ✅ |
| D147 | coverage heatmap overlay | 代码行×覆盖条×热力层 | `TestP1_Comp_D147_CoverageHeatmapCodeOverlay` | ✅ |
| D148 | recursive card stack | 深度偏移卡片栈×选中 | `TestP1_Comp_D148_RecursiveCardStackDepth` | ✅ |
| D149 | before/after filter compare | 对比条×grayscale×分割线 | `TestP1_Comp_D149_BeforeAfterFilterCompare` | ✅ |
| D150 | dense atlas toolbar | DrawAtlas 图标条×overflow | `TestP1_Comp_D150_DenseAtlasToolbarOverflow` | ✅ |
| D151 | mesh terrain contours | fan mesh×等高线×标签 | `TestP1_Comp_D151_MeshTerrainContourLabels` | ✅ |
| D152 | PresentFrame multi damage | 多脏区 PresentFrameDamageRects | `TestP1_Comp_D152_PresentFrameDamageMultiRect` | ✅ |
| D153 | nested backdrop panels | PushBackdrop×半透明叠层 | `TestP1_Comp_D153_NestedBackdropTranslucentPanels` | ✅ |
| D154 | path dash join gallery | 多 dash 样式×圆角 clip | `TestP1_Comp_D154_PathDashJoinGalleryClip` | ✅ |
| D155 | image quad warp clip | DrawImageQuad×path clip×stroke text | `TestP1_Comp_D155_ImageQuadWarpClipStrokeText` | ✅ |
| D156 | radial sweep gradient stack | 径向/扫描渐变×layer | `TestP1_Comp_D156_RadialSweepGradientLayerStack` | ✅ |
| D157 | window tiling manager | 多 pane×splitter×active | `TestP1_Comp_D157_WindowTilingManagerMock` | ✅ |
| D158 | circular progress dashboard | 环形进度×磁贴 | `TestP1_Comp_D158_CircularProgressDashboard` | ✅ |
| D159 | multi-context snapshot merge | 多 Context 合成 merge | `TestP1_Comp_D159_MultiContextSnapshotMerge` | ✅ |
| D160 | kitchen-sink v5 ultimate | mesh+blend+atlas+filter 终极应力 | `TestP1_Comp_D160_KitchenSinkV5UltimateStress` | ✅ |


| D161 | kanban WIP ghost | 泳道×WIP×拖拽幽灵层 | `TestP1_Comp_D161_KanbanWIPGhostComposition` | ✅ |
| D162 | tree inspector multi-select | 树×属性×面包屑×多选 | `TestP1_Comp_D162_TreeInspectorBreadcrumbMultiSelect` | ✅ |
| D163 | three-way diff gutter | 三方 diff×gutter×toggle | `TestP1_Comp_D163_ThreeWayDiffGutterComposition` | ✅ |
| D164 | candles volume MA tooltip | K线×成交量×均线×十字线 | `TestP1_Comp_D164_CandlesVolumeMACrosshairTooltip` | ✅ |
| D165 | wizard form sticky footer | 步骤条×校验×粘性底栏 | `TestP1_Comp_D165_WizardFormValidationStickyFooter` | ✅ |
| D166 | particle mask additive HUD | mask×Plus 粒子×HUD | `TestP1_Comp_D166_ParticleFieldMaskAdditiveHUD` | ✅ |
| D167 | z-index layers occlusion | 多层遮挡×选中描边 | `TestP1_Comp_D167_ZIndexLayersReorderOcclusion` | ✅ |
| D168 | rich tooltip caret actions | tip×caret×action chips | `TestP1_Comp_D168_RichTooltipStackCaretActions` | ✅ |
| D169 | CSS grid hero span | hero 跨列×卡片网格 | `TestP1_Comp_D169_CSSGridDenseCardsHeroSpan` | ✅ |
| D170 | responsive breakpoint reflow | 多宽度 reflow 模拟 | `TestP1_Comp_D170_ResponsiveBreakpointReflowMock` | ✅ |
| D171 | image annotation freehand | 图注×箭头×框×手绘 | `TestP1_Comp_D171_ImageAnnotationArrowsBoxesFreehand` | ✅ |
| D172 | sidebar rail flyout | 图标轨×flyout×active | `TestP1_Comp_D172_SidebarRailFlyoutActiveRoute` | ✅ |
| D173 | nested clip damage islands | 多岛 clip×WritePixels | `TestP1_Comp_D173_NestedClipDamageIslands` | ✅ |
| D174 | perspective fan cards | rotate×scale 扇形卡片 | `TestP1_Comp_D174_PerspectiveFanCardRotateScale` | ✅ |
| D175 | audio mixer faders | 推子×电平×mute | `TestP1_Comp_D175_AudioMixerFadersMeters` | ✅ |
| D176 | deep clip layer recursion | 5 层 clip×layer×blend | `TestP1_Comp_D176_DeepClipLayerBlendRecursion` | ✅ |
| D177 | type hierarchy shadow cards | 字号层级×阴影卡 | `TestP1_Comp_D177_TypeHierarchyShadowCards` | ✅ |
| D178 | constellation clusters | 星团×hull×连边 | `TestP1_Comp_D178_ConstellationClustersHullLabels` | ✅ |
| D179 | multipage preview strip | 多页预览×active | `TestP1_Comp_D179_MultipagePreviewStripActive` | ✅ |
| D180 | kitchen-sink v6 mega | atlas+mesh+filter+damage | `TestP1_Comp_D180_KitchenSinkV6MegaStress` | ✅ |


| D181 | chat thread reactions | 会话气泡×反应×composer | `TestP1_Comp_D181_ChatThreadReactionsComposer` | ✅ |
| D182 | inbox bulk actions | 侧栏×多选×批量栏 | `TestP1_Comp_D182_InboxFiltersBulkActions` | ✅ |
| D183 | settings search sections | 搜索×锚点分区 | `TestP1_Comp_D183_SettingsSearchAnchoredSections` | ✅ |
| D184 | board filters collapsed lanes | chips×折叠泳道 | `TestP1_Comp_D184_BoardFiltersSwimLanesCollapsed` | ✅ |
| D185 | pivot heat cells | 透视热力格 | `TestP1_Comp_D185_PivotTableHeatCells` | ✅ |
| D186 | gantt deps today | 依赖箭头×今日线 | `TestP1_Comp_D186_GanttDependencyArrowsToday` | ✅ |
| D187 | carousel peek dots | 窥视页×圆点×进度 | `TestP1_Comp_D187_CarouselPeekDotsProgress` | ✅ |
| D188 | org chart collapse | 组织树×连线 | `TestP1_Comp_D188_OrgChartConnectorsCollapse` | ✅ |
| D189 | mindmap radial | 放射脑图分支 | `TestP1_Comp_D189_MindmapRadialBranches` | ✅ |
| D190 | video editor timeline | 预览×多轨×playhead | `TestP1_Comp_D190_VideoEditorTimelineTracks` | ✅ |
| D191 | ER diagram | 表卡×关系线 | `TestP1_Comp_D191_SchemaERDiagramCrowFoot` | ✅ |
| D192 | log stream severity | 级别 chip×流 | `TestP1_Comp_D192_LogStreamSeverityFilters` | ✅ |
| D193 | sitemap hierarchy | 站点地图连线 | `TestP1_Comp_D193_SitemapHierarchyConnectors` | ✅ |
| D194 | notebook cells fold | 代码格×折叠输出 | `TestP1_Comp_D194_NotebookCellsOutputFold` | ✅ |
| D195 | CMS block nest | 块编辑嵌套 | `TestP1_Comp_D195_CMSBlockEditorNest` | ✅ |
| D196 | shop gallery variant | 主图×缩略×规格 | `TestP1_Comp_D196_ShopProductGalleryVariant` | ✅ |
| D197 | admin CRUD table | 表行操作 | `TestP1_Comp_D197_AdminCRUDTableRowActions` | ✅ |
| D198 | onboarding coachmarks | 遮罩×聚光×引导 | `TestP1_Comp_D198_OnboardingCoachmarksOverlay` | ✅ |
| D199 | status page incidents | 状态条×组件×事件 | `TestP1_Comp_D199_StatusPageIncidentsTimeline` | ✅ |
| D200 | kitchen-sink v7 blast | 密面板×blend×mesh×filter | `TestP1_Comp_D200_KitchenSinkV7CompositionBlast` | ✅ |






## A 关闭清单

- [x] D01–D08 首批主交叉轴  
- [x] D09–D36 高密度复杂组合  
- [x] D37–D58 极端轴 + 应用形态组合  
- [x] D59–D75 mega 形态密度  
- [x] D76–D90 ultra 形态  
- [x] D91–D105 hyper：ClipPreserve、灰度/矩阵滤波、TextureBase、PresentFrame、多 TextMode、文件管理器/邮件/泳道/雷达/PiP/双滚动、多次 Resize、混合脏更、设计标注、kitchen-sink v3  
- [x] D106–D120 omega：播放器/三栏/PR/日历/图网络/图像编辑/结账/抽屉/终端/markdown/表格多选/悬浮条/变换滤镜/mesh/应力格  
- [x] D121–D140 sigma：模态栈/分栏文/CAD/视频通话/表格多区/时间线/粘性滚动/拾色器/等距板/IDE/深度变换/blend 级联/filter 链/mask 粒子/无限画布/HiDPI/damage/多 TextMode/pattern/kitchen-sink v4  
- [x] D141–D160 tau：命令面板/嵌套弹出/双视口/sankey/地图/交易终端/覆盖率热力/卡片栈/前后对比/atlas 工具条/mesh 地形/多脏区 Present/backdrop/dash 画廊/quad warp/径向扫描渐变/平铺窗口/环形仪表/多 Context/kitchen-sink v5  
- [x] D161–D180 phi：看板/树检视/三方diff/K线/向导表单/粒子/ z 层/富 tip/网格 hero/响应式 reflow/图注/侧栏 flyout/损伤岛/扇形卡/混音台/深递归/字体层级/星团/多页预览/kitchen-sink v6  
- [x] D181–D200 chi：会话/收件箱/设置/看板过滤/透视热力/Gantt/轮播/组织图/脑图/视频时间线/ER/日志流/站点地图/笔记本/CMS/商城/Admin CRUD/引导/状态页/kitchen-sink v7  
- [x] 主交叉轴 + 呈现路径/文本模式全套/混合脏更新 + 更深应用形态组合  
- [x] **停扩**：组合探针止于 **D200**  
- [x] 关闭 A 前全量 `TestP1_*`（形态 Tier A–U）再确认  
- [x] **阶段 A 关闭**；主线焦点切到 **S4.0 基线**  

## 验证命令

```bash
export WGPU_NATIVE_PATH=/home/yanghy/app/projects/gogpu/gpui/lib/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache
export LD_LIBRARY_PATH=/home/yanghy/app/projects/gogpu/gpui/lib:$LD_LIBRARY_PATH

go test -count=1 ./render -run 'TestP1_Comp_' -timeout 250s
```

## 与后续 S4 的关系

- A 场景将作为 S4.0 **测量输入**（尤其 D05/D06/D08 与既有 P1 压力场景）  
- S4 优化不得删减本矩阵断言；只能在基线文档中记录加速比  

## 修订

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-15 | 1.0 | 建立阶段 A；首批 D01–D08 |
| 2026-07-15 | 1.1 | D01–D08 真 GPU 门禁全绿（gpu_ops>0, cpu_fallback=0） |
| 2026-07-15 | 1.2 | D09–D36 高密度复杂组合扩展；编辑器/图表/日历/kitchen-sink |
| 2026-07-15 | 1.3 | D37–D58 极端组合：filter graph/ImageEx/高级 blend/WritePixels/应用形态密度 |
| 2026-07-15 | 1.4 | D59–D75 mega：富文本/FrameDamage/canvas/chat/gantt/heatmap/map/diff/IDE/dashboard |
| 2026-07-15 | 1.5 | D76–D90 ultra：shaped/vector text、carousel/video、org/mindmap/K线/isometric/多Context/粒子 |
| 2026-07-15 | 1.11 | D181–D200 chi 完成；**阶段 A 关闭（停 D200）**；下一焦点 S4.0 |
| 2026-07-15 | 1.10 | D161–D180 phi：看板/三方diff/K线/粒子/混音台/深递归/kitchen-sink v6 |
| 2026-07-15 | 1.9 | D141–D160 tau：命令面板/双视口/sankey/地图/交易/热力/atlas/mesh/PresentDamage/backdrop/渐变/平铺/kitchen-sink v5 |
| 2026-07-15 | 1.8 | D121–D140 sigma：模态/CAD/视频/表格/时间线/拾色器/IDE/深度变换/blend/filter/mask/canvas/HiDPI/damage/kitchen-sink v4；**停 D140** |
| 2026-07-15 | 1.7 | D106–D120 omega：播放器/三栏/PR/日历/图网络/图像编辑/结账/抽屉/终端/markdown/多选/悬浮条/应力格 |
| 2026-07-15 | 1.6 | D91–D105 hyper：ClipPreserve/PresentFrame/多TextMode/文件管理/邮件/泳道/雷达/PiP/混合脏更 |
