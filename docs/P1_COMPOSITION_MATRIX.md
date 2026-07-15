# 阶段 A — 任意组合维度矩阵（Composition Matrix）

> 版本：1.2 | 日期：2026-07-15  
> 主线：[`MAINLINE_PLAN.md`](./MAINLINE_PLAN.md) v1.35+  
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
| **external tex** | `CreateOffscreenTexture` + `DrawGPUTexture` |

组合写法：`Dxx = 轴1 × 轴2 × …`，每条探针至少 **3 轴交叉**。复杂场景可叠加 5+ 轴。

## 探针状态

| ID | 交叉 | 场景意图 | 门禁 | 状态 |
|----|------|----------|------|------|
| D01 | clip × layer × text | 嵌套矩形 clip + 半透明层 + 标签 | `TestP1_Comp_D01_ClipLayerText` | ✅ |
| D02 | clip × image × blend | 圆角 clip 内图像 + Plus 叠色 | `TestP1_Comp_D02_ClipImageBlend` | ✅ |
| D03 | clipPath × layer × fill | 多边形 path clip + 层内填充 | `TestP1_Comp_D03_ClipPathLayerFill` | ✅ |
| D04 | HiDPI × hairline × text | DPR=2 下 hairline 与文字共存 | `TestP1_Comp_D04_HiDPIHairlineText` | ✅ |
| D05 | layer × blend × clip | 外 clip + Multiply 层叠 | `TestP1_Comp_D05_LayerBlendClip` | ✅ |
| D06 | image × text × clip × backdrop | 内容区 + 文字 + backdrop dim | `TestP1_Comp_D06_ImageTextClipBackdrop` | ✅ |
| D07 | transform × clip × fill | Translate/Scale 下 clip 与填充 | `TestP1_Comp_D07_TransformClipFill` | ✅ |
| D08 | multi-region redraw | 全量底图后多脏区局部重绘 | `TestP1_Comp_D08_MultiRegionRedraw` | ✅ |
| D09 | dash × clip × text | 虚线描边 + 嵌套 clip + 标签 | `TestP1_Comp_D09_DashStrokeClipText` | ✅ |
| D10 | gradient × clipRRect × layer | 多 stop 渐变 + 圆角 clip + 半透明层 | `TestP1_Comp_D10_GradientClipLayer` | ✅ |
| D11 | evenOdd × layer × blend | 孔洞填充 + 层 + Multiply | `TestP1_Comp_D11_EvenOddLayerBlend` | ✅ |
| D12 | mask × fill × image | alpha mask 调制 + 底图 | `TestP1_Comp_D12_MaskFillImage` | ✅ |
| D13 | maskLayer × text × clip | PushMaskLayer + 文字 + clip | `TestP1_Comp_D13_MaskLayerTextClip` | ✅ |
| D14 | vertices × clip × blend | 彩色 mesh + clip + Plus | `TestP1_Comp_D14_VerticesClipBlend` | ✅ |
| D15 | atlas × HiDPI × clip | DrawAtlas 精灵 + DPR + clip | `TestP1_Comp_D15_AtlasHiDPIClip` | ✅ |
| D16 | mesh × transform × layer | 索引 mesh + CTM + 层透明度 | `TestP1_Comp_D16_MeshTransformLayer` | ✅ |
| D17 | imageQuad × clipPath × text | 任意四边形图 + path clip + 字 | `TestP1_Comp_D17_ImageQuadClipText` | ✅ |
| D18 | pathEffects × stroke × clip | corners/discrete/trim 组合描边 | `TestP1_Comp_D18_PathEffectsClipStroke` | ✅ |
| D19 | deepNest clip×layer×image×text | 三层 clip + 双层 + 宫格图文 | `TestP1_Comp_D19_DeepNestClipLayerImageText` | ✅ |
| D20 | multiBlend × clip × image | Multiply/Screen/Plus 栈 | `TestP1_Comp_D20_MultiBlendClipImage` | ✅ |
| D21 | externalTex × clip × backdrop | 离屏纹理磁贴 + 灯箱 | `TestP1_Comp_D21_ExternalTexClipBackdropText` | ✅ |
| D22 | shadow × blur × dense | DropShadow + Blur + 卡片密度 | `TestP1_Comp_D22_ShadowBlurDenseScene` | ✅ |
| D23 | dither × gradient × HiDPI | 有序抖动 + 渐变 + DPR | `TestP1_Comp_D23_DitherGradientHiDPI` | ✅ |
| D24 | scrollClip × translate × text | 视口 clip + 平移列表 | `TestP1_Comp_D24_ScrollClipTranslateText` | ✅ |
| D25 | nestedLayer × multiBlend | 多层 + Multiply/Screen 叠色 | `TestP1_Comp_D25_DeepNestedBlendLayers` | ✅ |
| D26 | caps/joins × dash × pathClip | 端点/拐角/虚线/path clip | `TestP1_Comp_D26_CapsJoinsDashPathClip` | ✅ |
| D27 | imageRounded × mask × layer | 圆角/圆形图 + mask 洗色 | `TestP1_Comp_D27_ImageRoundedMaskLayer` | ✅ |
| D28 | damage × gradient × text × image | 多脏区 + 渐变头 + 图文 | `TestP1_Comp_D28_DamageGradientTextImage` | ✅ |
| D29 | rotate × clip × image × stroke | 旋转 CTM + clip + 图 + 描边 | `TestP1_Comp_D29_RotateClipImageStroke` | ✅ |
| D30 | virtualList primitives | 列表密度：clip×image×text×badge×layer | `TestP1_Comp_D30_VirtualListPrimitiveDensity` | ✅ |
| D31 | lattice stress blend×clip | 80 格 × 交替 blend × clip | `TestP1_Comp_D31_LatticeStressBlendClip` | ✅ |
| D32 | pattern × transform × clip | 图像 pattern 填充 + 缩放 clip | `TestP1_Comp_D32_PatternTransformClip` | ✅ |
| D33 | editor multi-pane | 树+代码+gutter+selection+minimap | `TestP1_Comp_D33_EditorMultiPaneComposition` | ✅ |
| D34 | chart primitives | 网格+渐变柱+折线+图例 clip | `TestP1_Comp_D34_ChartPrimitiveComposition` | ✅ |
| D35 | calendar grid | 多格+高亮层+事件条+clip | `TestP1_Comp_D35_CalendarGridComposition` | ✅ |
| D36 | kitchen-sink max mix | 近全轴同场景交叉压力 | `TestP1_Comp_D36_KitchenSinkMaxMix` | ✅ |


## A 关闭清单

- [x] D01–D08 真 GPU 绿（首批主交叉轴）  
- [x] D09–D36 高密度 / 复杂组合扩展（真 GPU 绿）  
- [x] 主交叉轴覆盖：clip/layer/blend/text/image/transform/HiDPI/mask/mesh/atlas/pathEffect/gradient/pattern/backdrop/damage/filter  
- [ ] 可选再增极端压力 D37+（按回归成本决定，不阻塞 S4.0）  
- [ ] 关闭 A 前全量 `TestP1_*`（形态 Tier A–U）再确认  
- [ ] 主线焦点切到 **S4.0 基线**  

## 验证命令

```bash
export WGPU_NATIVE_PATH=/home/yanghy/app/projects/gogpu/gpui/lib/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache
export LD_LIBRARY_PATH=/home/yanghy/app/projects/gogpu/gpui/lib:$LD_LIBRARY_PATH

go test -count=1 ./render -run 'TestP1_Comp_' -timeout 120s
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
