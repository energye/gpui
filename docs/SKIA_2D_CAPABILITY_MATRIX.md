# Skia 级 2D 渲染能力表（GPUI 主线验收基准）

> 版本：1.4 | 日期：2026-07-15  
> 用途：定义 render 对标 **Skia 2D 渲染能力** 的全面清单，并反推 `rwgpu` / `gpu/webgpu` 必绑 WebGPU 子集。  
> 范围：**仅渲染栈** `render → gpu/webgpu → gpu/rwgpu → libwgpu_native`。不含控件层、不含过早性能优化。  
> 维护：能力缺口只允许“新增行”，不允许静默缩小已列必选项。

---

## 0. 使用说明

### 0.1 对标含义

| 对标 | 含义 | 不对标 |
|------|------|--------|
| Skia **2D 能力** | Canvas/Paint/Path/Image/Text/Layer 等语义与可测结果 | 1:1 抄 Skia 源码结构 |
| WebGPU **API 形状** | 用 wgpu-native 实现 GPU 后端 | 假设 Skia Ganesh(GL/Vulkan) 每个 call 都有 WebGPU 等价物 |
| 正确性优先 | 像素/语义门禁 | 初期 FPS/批处理 |

### 0.2 状态列图例

| 符号 | 含义 |
|------|------|
| ✅ | 已具备可用实现 + 基本测试 |
| 🔄 | 部分实现 / 仅 CPU 或仅 GPU / 语义不全 |
| ⬜ | 未实现或不可用 |
| N/A | 本阶段不要求 GPU 绑定（纯 CPU 可接受） |

**Layer 列**：`rwgpu` = native ABI 绑定；`webgpu` = facade；`render` = 2D API/语义。

### 0.3 优先级

| 级 | 含义 |
|----|------|
| **M0** | 骨架：能建设备、清屏、上传、提交、readback |
| **M1** | 基础 2D：path fill/stroke、AA、transform、solid color、clip rect |
| **M2** | UI 高频：rrect、blend/premul、image、text、layer opacity、dash、gradient |
| **M3** | 完整 2D：高级 clip/path effect、filter、vertices、color space、surface present |
| **M4** | 增强：完整 image filter 图、PDF/SVG 后端、录制优化等（可后置） |

---

## 1. 能力总表（Skia 2D → 实现层）

### 1.1 Surface / Canvas 生命周期

| ID | 能力 | Skia 参考 | WebGPU 需求（摘要） | rwgpu | webgpu | render | Pri |
|----|------|-----------|---------------------|-------|--------|--------|-----|
| S.01 | 离屏像素表面 | `SkSurface::MakeRaster` | Buffer/Texture + readback | ✅ | ✅ | ✅ Pixmap/Context | M0 |
| S.02 | GPU 离屏 RT | `SkSurface::MakeRenderTarget` | Texture RenderAttachment + resolve | ✅ | ✅ | ✅ Context+FlushGPU | M0 |
| S.03 | 窗口 Surface/Swapchain | `MakeFromBackendRenderTarget` / Window | Surface/Swapchain/Present/配置 | ✅ 绑定+CreateSurface | ✅ Swapchain API | ✅ PresentFrame + 同 device 注入；X11 draw `window_present` / `-tags gpui_x11_present` | M3 |
| S.04 | 清屏/clear 色 | `canvas->clear` | LoadOpClear + clearValue | ✅ | ✅ | ✅ Clear/ClearWithColor | M0 |
| S.05 | 尺寸/resize | surface 重建 | 重建 texture/pipeline 依赖 | ✅ | ✅ | ✅ `Resize` + GPU 重绘 `TestP1_Capability_S05_ResizeGPU` | M1 |
| S.06 | 读回像素 | `peekPixels` / `readPixels` | copyTextureToBuffer + map | ✅ | ✅ | ✅ Image/SavePNG/FlushGPU | M0 |
| S.07 | 写像素/上传 | `writePixels` | queue.writeTexture/writeBuffer | ✅ | ✅ | ✅ `WritePixels` GPU textured upload `TestP1_Capability_S07_WritePixelsGPU` | M0 |
| S.08 | DPR / 逻辑像素 | surface props scale | 视口/纹理物理尺寸 | N/A | N/A | ✅ deviceScale + HiDPI hairline `TestP1_Capability_S08_HiDPIHairline` | M1 |
| S.09 | 部分更新/damage | dirty rect present | scissor + LoadOpLoad | ✅ | ✅ | ✅ FlushGPUWithViewDamage `TestS3c` | M3 |

### 1.2 变换 (Matrix)

| ID | 能力 | Skia 参考 | WebGPU 需求 | rwgpu | webgpu | render | Pri |
|----|------|-----------|-------------|-------|--------|--------|-----|
| T.01 | 2D 仿射 Translate/Scale/Rotate/Skew | `concat`/`setMatrix` | Uniform / vertex xform | N/A | N/A | ✅ | M1 |
| T.02 | Save/Restore CTM | `save`/`restore` | 栈在 CPU | N/A | N/A | ✅ | M1 |
| T.03 | 非均匀缩放 stroke | stroke 受 CTM | 管线或 CPU 展平 | N/A | N/A | ✅ user-space expand + CTM `TestP1_Capability_T03` | M2 |
| T.04 | 透视/非仿射（可选） | `SkMatrix` persp | 网格细分/homography | N/A | N/A | ✅ `DrawImageQuad` 任意四边形 `TestP1_Capability_T04_ImageQuadGPU` | M4 |

### 1.3 Paint：颜色、样式、描边

| ID | 能力 | Skia 参考 | WebGPU 需求 | rwgpu | webgpu | render | Pri |
|----|------|-----------|-------------|-------|--------|--------|-----|
| P.01 | Solid color RGB/A | `setColor` | premul blend 输入 | N/A | N/A | ✅ | M0 |
| P.02 | Style Fill/Stroke/StrokeAndFill | `setStyle` | 几何生成 | N/A | N/A | ✅ | M1 |
| P.03 | Stroke width | `setStrokeWidth` | 展平/SDF | N/A | N/A | ✅ | M1 |
| P.04 | Hairline（设备 1px） | width=0 语义 | 像素对齐 stroke | N/A | N/A | ✅ GPU ink `TestP1_Capability_P04_HairlineGPU` / `TestS3a_M1_Hairline` | M1 |
| P.05 | Cap Butt/Round/Square | `setStrokeCap` | 几何 | N/A | N/A | ✅ GPU pixels `TestP1_Capability_P05_StrokeCapsGPU` | M1 |
| P.06 | Join Miter/Round/Bevel | `setStrokeJoin` | 几何 | N/A | N/A | ✅ GPU pixels `TestP1_Capability_P06_StrokeJoinsGPU` | M1 |
| P.07 | Miter limit | `setStrokeMiter` | 几何 | N/A | N/A | ✅ `TestS3b_M2_MiterLimit` | M2 |
| P.08 | Anti-alias 开关 | `setAntiAlias` | MSAA 或 coverage AA | ✅ MSAA | ✅ | ✅ SetAntiAlias | M1 |
| P.09 | Dither（可选） | `setDither` | shader | N/A | N/A | ✅ ordered Bayer `SetDither` `TestP1_Capability_P09_DitherGPU` | M4 |

### 1.4 Blend / Alpha / Premul

| ID | 能力 | Skia 参考 | WebGPU 需求 | rwgpu | webgpu | render | Pri |
|----|------|-----------|-------------|-------|--------|--------|-----|
| B.01 | SrcOver（默认） | `kSrcOver` | BlendState premul | ✅ enum | ✅ | ✅ `TestP12GPUFixedPixel_SourceOverPremul` | M1 |
| B.02 | Clear/Src/Dst/… Porter-Duff | `SkBlendMode` PD | blend factor 或 shader blend | ✅ enum+factors | ✅ BlendState | ✅ GPU fixed PD: Clear/Copy/Plus/SrcOver/DstOut/SrcAtop/Xor/DstOver/SrcIn/SrcOut/DstIn/DstAtop `TestP12GPUFixedPixel_Blend*` | M2 |
| B.03 | Multiply/Screen/Overlay… | separable modes | shader blend 常见 | ✅ shader | ✅ | ✅ dual-tex Mul/Screen/Overlay + Darken/Lighten/Dodge/Burn/Hard/Soft/Diff/Exclusion `TestP1_Capability_B03_*` | M2 |
| B.04 | HSL 模式 Hue/… | non-separable | shader | ✅ | ✅ | ✅ dual-tex HSL Hue/Sat/Color/Lum GPU `TestP1_Capability_B04_*` / `TestS3c_M3_Blend*` | M3 |
| B.05 | Premul 约定贯穿 | premul pipeline | texture/blend 一致 | ✅ | ✅ | ✅ solid+image+layer+text premul `TestP1_Capability_B05_*` | M1 |
| B.06 | 全局 alpha | paint alpha | uniform/premul | N/A | N/A | ✅ solid/image/layer/text premul `TestP1_Capability_B06_PaintAlpha` + `...MultiPath` | M1 |
| B.07 | Plus/Modulate 等 | `kPlus`/`kModulate` | blend factors | ✅ Plus + Modulate | ✅ | ✅ GPU Plus + Modulate `TestP12GPUFixedPixel_BlendPlus` / `...BlendModulate` | M2 |

### 1.5 Path 构建与填充

| ID | 能力 | Skia 参考 | WebGPU 需求 | rwgpu | webgpu | render | Pri |
|----|------|-----------|-------------|-------|--------|--------|-----|
| H.01 | Move/Line/Quad/Cubic/Close | `SkPath` | CPU path → GPU mesh/stencil | N/A | N/A | ✅ | M1 |
| H.02 | Arc/圆角路径 | `arcTo` | 细分 | N/A | N/A | ✅ | M1 |
| H.03 | Fill rule NonZero/EvenOdd | `setFillType` | stencil 或 winding | ✅ stencil | ✅ | ✅ EvenOdd hole vs NonZero fill `TestP1_Capability_H03_EvenOddGPU` | M1 |
| H.04 | Path 布尔（可选） | `Op` | CPU | N/A | N/A | ✅ BooleanPath `TestS3c_M3_PathBoolean*` | M3 |
| H.05 | Path measure/长度 | `SkPathMeasure` | CPU | N/A | N/A | ✅ Path.Length `TestS3c` | M3 |
| H.06 | 复杂 path GPU 光栅 | GrPathRenderer 类 | stencil-then-cover / tess / compute | ✅ | ✅ | ✅ stencil-then-cover（shapes/clip STRICT） | M2 |
| H.07 | Convex path 快路径 | convex | 专用 pipeline | ✅ | ✅ | ✅ convex renderer 路径（S3a/S3b 场景） | M2 |

### 1.6 图元（可归约为 Path）

| ID | 能力 | Skia 参考 | WebGPU 需求 | rwgpu | webgpu | render | Pri |
|----|------|-----------|-------------|-------|--------|--------|-----|
| G.01 | Rect | `drawRect` | fill/stroke | N/A | N/A | ✅ | M1 |
| G.02 | Oval/Circle | `drawOval`/`drawCircle` | SDF 或 path | N/A | N/A | ✅ + SDF GPU | M1 |
| G.03 | RRect | `drawRRect` | SDF/path | N/A | N/A | ✅ + SDF | M1 |
| G.04 | Line / 折线 | `drawLine`/`drawPoints` | stroke | N/A | N/A | ✅ | M1 |
| G.05 | Arc | `drawArc` | path | N/A | N/A | ✅ | M1 |
| G.06 | RoundRect 变体 XY 半径 | `SkRRect` | path | N/A | N/A | ✅ `DrawRoundedRectangleXY` + GPU `TestP1_Capability_G06_*` | M2 |

### 1.7 Clip

| ID | 能力 | Skia 参考 | WebGPU 需求 | rwgpu | webgpu | render | Pri |
|----|------|-----------|-------------|-------|--------|--------|-----|
| C.01 | Clip rect | `clipRect` | scissor 或 stencil | ✅ scissor | ✅ | ✅ | M1 |
| C.02 | Clip rrect | `clipRRect` | stencil/SDF mask | ✅ | ✅ | ✅ ClipRoundRect `TestS3b` | M2 |
| C.03 | Clip path | `clipPath` | stencil | ✅ | ✅ | ✅ Clip+GPU fill `TestS3b_M2_ClipPath` | M2 |
| C.04 | Clip 栈 / 交 | clip stack | stencil ref / mask | ✅ | ✅ | ✅ 栈 | M1 |
| C.05 | Clip AA | aa clip | coverage/stencil MSAA | ✅ | ✅ | ✅ rrect SDF soft edge + path clip flag `TestP1_Capability_C05_*` | M2 |
| C.06 | Replace/Difference clip op | `SkClipOp` | stencil ops / mask | ✅ | ✅ | ✅ ClipRectOp `TestS3c_M3_ClipRect*` | M3 |

### 1.8 Layer / SaveLayer

| ID | 能力 | Skia 参考 | WebGPU 需求 | rwgpu | webgpu | render | Pri |
|----|------|-----------|-------------|-------|--------|--------|-----|
| L.01 | Save/Restore | `save`/`restore` | CPU 栈 | N/A | N/A | ✅ | M1 |
| L.02 | SaveLayer 离屏 | `saveLayer` | 离屏 RT + composite | ✅ | ✅ | ✅ PushLayer（内容可 GPU）`TestS3b` | M2 |
| L.03 | Layer opacity | saveLayer alpha | premul composite | ✅ | ✅ | ✅ 层 GPU 内容+CPU composite `TestS3b` | M2 |
| L.04 | Layer blend mode | saveLayer blend | blend/shader | ✅ | ✅ | ✅ Multiply/Screen 层 `TestS3b` | M2 |
| L.05 | Layer + backdrop（可选） | backdrop filter | 采样背景 | N/A | N/A | ✅ `PushBackdropLayer` 快照父画布 `TestP1_Capability_L05_BackdropLayerGPU` | M4 |
| L.06 | Mask layer | mask filter/clip mask | R8 mask texture | ✅ R8 MaskAware | ✅ | ✅ MaskAware + convex/SDF/stencil cover-inline R8 `TestP1_Capability_L06_*` | M2 |

### 1.9 Shader / Gradient / Pattern

| ID | 能力 | Skia 参考 | WebGPU 需求 | rwgpu | webgpu | render | Pri |
|----|------|-----------|-------------|-------|--------|--------|-----|
| D.01 | Linear gradient | `SkGradientShader::MakeLinear` | 1D tex 或 shader | ✅ | ✅ | ✅ GPU staging+quad `TestS3b` | M2 |
| D.02 | Radial gradient | `MakeRadial` | shader/tex | ✅ | ✅ | ✅ GPU staging+quad `TestS3b` | M2 |
| D.03 | Sweep/conic | `MakeSweep` | shader | ✅ native shader | ✅ | ✅ GPU staging blit `TestP1_Capability_D03_SweepGradientGPU`（与 linear 同 bootstrap） | M2 |
| D.04 | 多 stop / tile mode | clamp/repeat/mirror | sampler address | ✅ sampler | ✅ | ✅ multi-stop + ExtendRepeat/Reflect GPU staging `TestP1_Capability_D04_*` | M2 |
| D.05 | Image shader/pattern | `SkImage::makeShader` | texture sample | ✅ | ✅ | ✅ ImagePattern fill GPU staging `TestP1_Capability_D05_*` | M2 |
| D.06 | Local matrix on shader | localMatrix | uniform | N/A | N/A | ✅ ImagePattern SetScale/SetTransform GPU `TestP1_Capability_D06_*` | M2 |

### 1.10 Image / Bitmap

| ID | 能力 | Skia 参考 | WebGPU 需求 | rwgpu | webgpu | render | Pri |
|----|------|-----------|-------------|-------|--------|--------|-----|
| I.01 | DrawImage | `drawImage` | textured quad | ✅ | ✅ | ✅ GPU quad `TestS3b` | M2 |
| I.02 | DrawImageRect/src-dst | `drawImageRect` | UV rect | ✅ | ✅ | ✅ scale dst `TestS3b` | M2 |
| I.03 | 过滤 Nearest/Linear | sampling | FilterMode | ✅ | ✅ | ✅ Nearest/Linear GPU `TestP1_Capability_I03` | M2 |
| I.04 | Cubic/mipmap（可选） | cubic sampling | mip / custom | ✅ | ✅ | ✅ InterpBicubic+UseMipmaps `TestS3c_M3_ImageBicubicAndMipmap` | M3 |
| I.05 | Opacity / alpha image | paint alpha | premul | ✅ | ✅ | ✅ DrawImageEx opacity `TestS3b` | M2 |
| I.06 | 旋转/CTM 下图像 | concat + drawImage | 任意 quad | ✅ | ✅ | ✅ 四角点 | M2 |
| I.07 | 九宫格（可选） | lattice | 多 quad / DrawAtlas | N/A | N/A | ✅ DrawImageNine `TestS3c_M3_DrawImageNine` | M3 |
| I.08 | YUV/外部纹理（可选） | external | multiplanar | ✅ view bind | ✅ | ✅ offscreen external `DrawGPUTexture` `TestP1_Capability_I08_ExternalTextureGPU`（真 multiplanar YUV 后置） | M4 |

### 1.11 Text / Font / Glyph

| ID | 能力 | Skia 参考 | WebGPU 需求 | rwgpu | webgpu | render | Pri |
|----|------|-----------|-------------|-------|--------|--------|-----|
| X.01 | 字体加载/Face | SkTypeface | CPU | N/A | N/A | ✅ | M1 |
| X.02 | DrawString baseline | `drawString` | glyph atlas tex | ✅ R8 atlas | ✅ | ✅ GPU `TestS3b_M2_DrawString` | M2 |
| X.03 | Glyph 位置 shaping | shape + pos | CPU shape | N/A | N/A | ✅ OwnShaper GSUB/GPOS + DrawShapedGlyphs GPU `TestP1_Capability_X03` | M2 |
| X.04 | Subpixel positioning | subpixel | atlas/frac | N/A | N/A | ✅ 1/4 px glyph mask frac GPU `TestP1_Capability_X04` | M2 |
| X.05 | Edging: alias/anti-alias/subpixel LCD | edging | RGB mask / blend | ✅ | ✅ | ✅ LCD two-pass darken+add（白底+彩底）`TestP1_Capability_X05` / `X05_LCDTextOnColoredDestGPU` | M2 |
| X.06 | CJK / fallback 字体 | fallback | 同 text | N/A | N/A | ✅ MultiFace Runs + GPU glyph mask `TestP1_Capability_X06` | M2 |
| X.07 | 路径化文本 | text → path | path 管线 | N/A | N/A | ✅ outline | M2 |
| X.08 | 文本装饰 underline 等 | decorations | 几何 | N/A | N/A | ✅ SetTextDecoration `TestS3c_M3_TextUnderline` | M3 |
| X.09 | 变体字体 variable | variations | CPU | N/A | N/A | ✅ LoadFontFaceWithVariations `TestS3c_M3_VariableFontWeight` | M3 |
| X.10 | Emoji/彩色字体 | CBDT/SBIX/… | RGBA atlas | ✅ | ✅ | ✅ DrawWithEmoji 接入 DrawString `TestS3c_M3_DrawWithEmojiAPI` | M3 |
| X.11 | Glyph atlas 管理 | strike cache | texture atlas + upload | ✅ | ✅ | ✅ R8 atlas put/reuse under GPU text `TestP1_Capability_X11` | M2 |

### 1.12 Path Effect

| ID | 能力 | Skia 参考 | WebGPU 需求 | rwgpu | webgpu | render | Pri |
|----|------|-----------|-------------|-------|--------|--------|-----|
| E.01 | Dash | `SkDashPathEffect` | CPU 展平 stroke | N/A | N/A | ✅ GPU ApplyDash+expand | M2 |
| E.02 | Corner/1D/2D path effect | path effects | CPU | N/A | N/A | ✅ `WithCorners`/`Discrete` `TestP1_Capability_E02_PathEffectsGPU` | M4 |
| E.03 | Trim path（可选） | trim | CPU | N/A | N/A | ✅ `Path.Trim` + GPU stroke `TestP1_Capability_E03_TrimPathGPU` | M4 |

### 1.13 MaskFilter / ImageFilter（质量项）

| ID | 能力 | Skia 参考 | WebGPU 需求 | rwgpu | webgpu | render | Pri |
|----|------|-----------|-------------|-------|--------|--------|-----|
| F.01 | Blur mask filter | `SkMaskFilter::MakeBlur` | 离屏 + blur pass | ✅ | ✅ | ✅ ApplyBlur `TestS3c` | M3 |
| F.02 | Drop shadow | image filter | multi-pass | ✅ | ✅ | ✅ ApplyDropShadow + GPU graph node `TestS3c` / `TestP1_Capability_F03_GPUColorMatrixDropShadow` | M3 |
| F.03 | 通用 image filter 图 | `SkImageFilter` DAG | 多 RT ping-pong | ✅ | ✅ | ✅ ApplyImageFilterGraph + **GPU multi-RT** blur/gray/invert/CM/DropShadow `TestP1_Capability_F03_*` | M3/M4 |
| F.04 | Color filter | `SkColorFilter` | shader/LUT | ✅ | ✅ | ✅ ApplyGrayscale/matrix + GPU graph CM `TestS3c` / `F03_GPUColorMatrix*` | M3 |

### 1.14 Vertices / 网格 / Atlas 精灵

| ID | 能力 | Skia 参考 | WebGPU 需求 | rwgpu | webgpu | render | Pri |
|----|------|-----------|-------------|-------|--------|--------|-----|
| V.01 | DrawVertices | `drawVertices` | vertex color/uv pipeline | ✅ buffer | ✅ | ✅ GPU `TestS3c_M3_DrawVertices` | M3 |
| V.02 | DrawAtlas | `drawAtlas` | multi QueueImageDraw | ✅ | ✅ | ✅ GPU `TestS3c_M3_DrawAtlas` | M3 |
| V.03 | Mesh（可选） | `drawMesh` | 自定义 shader / indexed mesh | ✅ | ✅ | ✅ indexed+Gouraud `DrawMesh` `TestP1_Capability_V03_DrawMeshIndexedGPU` | M4 |

### 1.15 MSAA / 质量 / 像素惯例

| ID | 能力 | Skia 参考 | WebGPU 需求 | rwgpu | webgpu | render | Pri |
|----|------|-----------|-------------|-------|--------|--------|-----|
| Q.01 | MSAA 4x + resolve | samples | multisample texture + resolve | ✅ | ✅ | ✅ sampleCount=4 `TestS3b_M2_MSAAResolve` | M2 |
| Q.02 | Coverage AA 无 MSAA | analytic AA | 软件/计算覆盖 | N/A | N/A | ✅ AA on/off GPU fringe/interior `TestP1_Capability_Q02_CoverageAAGPU` / `TestS3a_M1_AntiAliasToggle` | M1 |
| Q.03 | 像素对齐/近像素规则 | device pixel snap | CPU/GPU 一致 | N/A | N/A | ✅ AA-off path snap GPU `TestP1_Capability_Q03` | M2 |
| Q.04 | 半透明边缘与 premul | premul AA | blend | ✅ | ✅ | ✅ 半透明 AA 粉边无 fringe 爆色 `TestP1_Capability_Q04_PremulAAEdgeGPU` | M1 |

### 1.16 颜色空间 / 位深（可分阶段）

| ID | 能力 | Skia 参考 | WebGPU 需求 | rwgpu | webgpu | render | Pri |
|----|------|-----------|-------------|-------|--------|--------|-----|
| CS.01 | sRGB 默认 | color space | sRGB texture format 可选 | ✅ | ✅ | ✅ 8bit mid-gray `TestS3c` | M3 |
| CS.02 | F16 / 宽色域（可选） | F16 surface | rgba16float | ✅ | ✅ | ✅ RGBA16Float RT clear+readback `TestP1_Capability_CS02_RGBA16FloatSurfaceGPU`（render Context 仍 8-bit） | M4 |
| CS.03 | 线性混合 vs sRGB 混合 | linear blending | 格式/shader | ✅ | ✅ | ✅ black→white linear mid≈0.735 `TestP1_Capability_CS03_LinearBlendMidGPU` | M4 |

### 1.17 录制 / 回放 / 文档后端（后置）

| ID | 能力 | Skia 参考 | WebGPU 需求 | rwgpu | webgpu | render | Pri |
|----|------|-----------|-------------|-------|--------|--------|-----|
| R.01 | Picture 录制回放 | `SkPicture` | 无（命令表） | N/A | N/A | ✅ recording Playback `TestS3c` | M3 |
| R.02 | PDF/SVG 后端 | document | 无 GPU | N/A | N/A | ⬜ **画布 100% 排除**（见 `CAPABILITY_MATRIX_WINDOW.md` §0；document 专项 DOC.1） | M4 |

### 1.18 计算路径 / 高级 GPU（可选增强）

| ID | 能力 | Skia/行业参考 | WebGPU 需求 | rwgpu | webgpu | render | Pri |
|----|------|---------------|-------------|-------|--------|--------|-----|
| K.01 | Compute 粗光栅/分片 | Vello/Pathfinder 类 | compute pipeline/storage | ✅ | ✅ | ✅ Context `PipelineModeCompute` multi-path `TestP1_Capability_K01_VelloComputePathGPU` + complex `TestP1_P2_*` | M3 |
| K.02 | 间接绘制 | multi-draw | indirect buffer | ✅ | ✅ | ✅ DrawIndirect 真链路读回 `TestP1_Capability_K02_DrawIndirectGPU`（高层 scene multi-draw 后置） | M4 |

---

## 2. 由能力表反推的 WebGPU / rwgpu 必绑子集

> 原则：绑定 **Skia 级 2D 所需 WebGPU 能力** 全面，而不是 WebGPU 全文。

### 2.1 核心对象与入口（M0）

| API 组 | 关键入口 | 能力依赖 |
|--------|----------|----------|
| Instance/Adapter/Device/Queue | CreateInstance, RequestAdapter, RequestDevice, Queue, Poll | 全部 GPU |
| Error scope / 日志 | Push/Pop error scope | 可调试绑定 |
| Limits/Features | 查询 limits/features | MSAA、format 决策 |

### 2.2 资源（M0–M2）

| API 组 | 关键入口 | 能力依赖 |
|--------|----------|----------|
| Buffer | create, map async, unmap, writeBuffer, usages | 顶点/索引/uniform/storage/readback |
| Texture | create, createView, writeTexture, copy* | RT、atlas、图片、mask |
| Sampler | create | image/gradient |
| 格式 | RGBA8, BGRA8, R8, depth24+stencil8, 可选 sRGB/F16 | 表面、mask、深度模板 |
| Usage | CopySrc/Dst, TextureBinding, RenderAttachment, storage… | 全管线 |

### 2.3 管线与绑定（M0–M2）

| API 组 | 关键入口 | 能力依赖 |
|--------|----------|----------|
| ShaderModule | WGSL create | 全 GPU 绘制 |
| BindGroupLayout/BindGroup/PipelineLayout | create* | 全管线 |
| RenderPipeline | vertex/fragment/depthstencil/multisample/primitive/blend | path/text/image |
| ComputePipeline | create + compute pass | K.01 可选但建议保留 |
| Blend/DepthStencil 状态 | factor/op/writeMask/stencil ops | B.* C.* Q.* |

### 2.4 编码与提交（M0–M2）

| API 组 | 关键入口 | 能力依赖 |
|--------|----------|----------|
| CommandEncoder | begin render/compute, copy, finish | 全帧 |
| RenderPass | setPipeline/bind/vertex/index, draw/drawIndexed, scissor/viewport, stencilRef | 绘制 |
| ComputePass | setPipeline/bind, dispatch | 可选高级 path |
| Queue.submit | submit | 全帧 |
| Copy 路径 | B2T T2B T2T B2B | 上传/readback/mip |

### 2.5 Surface 呈现（M3）

| API 组 | 关键入口 | 能力依赖 |
|--------|----------|----------|
| Surface/Swapchain | create surface, configure, getCurrentTexture, present | S.03 窗口 |
| 与外部 view 互操作 | 接受外部 TextureView | 嵌入宿主 |

### 2.7 S1 绑定状态（2026-07-15）

- **Enum 严格性（G）**：✅ `s1_skia_subset_enum_test.go`
- **函数 NewProc（A–F）**：约 132 项，子集无函数级 ⬜
- **A–E 真链路烟测**：✅ `s1_ae_smoke_test.go`（Write/Map、WriteTexture/Copy、Draw/DrawIndexed 像素读回）
- **S1**：✅ 关闭（Skia 2D 子集）
- **S2**：✅ 关闭（`gpu/webgpu` facade + `TestS2*`）；**下一步 S3a** render

### 2.6 明确可后置（非 Skia 2D 阻塞）

- Ray tracing、video、WebGPU 实验扩展  
- 完整 query 性能计数（可要可不要）  
- 多队列高级同步（除非 present 需要）

---

## 3. 里程碑切分（能力 → 交付）

| 里程碑 | 能力 ID 焦点 | 绑定焦点 | Render 焦点 |
|--------|--------------|----------|-------------|
| **S0** 表与基线 | 本文档 | 清单冻结 | 现状标记 |
| **S1** rwgpu ABI | 支撑 M0–M2 的 2.x 子集 | header 对齐 + 测试 | 不扩 feature |
| **S2** webgpu facade | 同 S1 | 转换/生命周期完整 | 不扩 feature |
| **S3a** render M0–M1 | S/T/P/H/G/C 基础 | 缺口随用补 | path/clip/AA/transform 门禁 |
| **S3b** render M2 | B/I/X/D/L/E/Q | 缺口随用补 | UI 级 2D 门禁 |
| **S3c** render M3 | F/V/CS/S.03/R | surface 等 | 完整 2D + 窗口 |
| **S4** 性能 | 全表正确后 | 无新 ABI 也可 | batch/atlas/cache |

---

## 4. 与当前仓库的粗粒度差距（2026-07-15）

| 区域 | 现状摘要 | 风险 |
|------|----------|------|
| rwgpu | **S1 ✅**：enum header-lock + A–E native 烟测（`TestS1*`/`TestS1AE*`）；~132 NewProc | 非子集扩展未绑；Surface/MSAA resolve 深度在 S3 |
| webgpu | **S2 ✅**：子集 facade + 转换/烟测；render 不直连 rwgpu | Surface 完整消费在 S3c |
| render CPU | path/text/image/clip/layer 较全 | GPU 不一致 |
| render GPU | **S3a/S3b/S3c ✅**（M0–M3 主能力）；M4 可选后置 | K.01/Q.02 已门禁；窗口 multi-rect Present e2e 可选深化 |
| Surface | 外部 view 思路；完整 swapchain 弱 | 窗口路径 |
| Blend 模式 | CPU 包全；Context.SetBlendMode 弱；GPU 固定像素仅 SourceOver 首轮 | UI 叠加 |
| Filter/Shadow | 弱 | 视觉效果缺口 |
| Gradient GPU | 弱/回退 | 按钮/进度类 |

---

## 5. 验收与补充规则

1. **关闭 S3b（UI 级 2D）前**，M0–M2 中标记为必选的能力不得以“以后再说”删除。  
2. 新发现的 Skia 2D 能力只许 **新增行** 并标 Pri。  
3. 每项 M1/M2 能力关闭时至少具备：  
   - 真实 `WGPU_NATIVE_PATH` 路径（若声称 GPU）  
   - 像素或语义测试  
   - 所属层（rwgpu/webgpu/render）说明  
4. 性能数字不得替代本表能力关闭条件。

---

## 6. 修订记录

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-15 | 1.8 | P1：T.03 non-uniform stroke、X.06 MultiFace GPU、X.11 atlas |
| 2026-07-15 | 1.7 | P1：X.03/X.04/Q.03/L.06 GPU 门禁（shape/subpixel/snap/mask staging） |
| 2026-07-15 | 1.6 | P1：B.02 全 PD fixed GPU + D.04–D.06 pattern/tile/localMatrix GPU 门禁；Tier C 复杂 UI |
| 2026-07-15 | 1.22 | H.03 EvenOdd + L.06 PushMaskLayer + P.04 + Tier F Cascader/VirtualList |
| 2026-07-15 | 1.21 | B.05 layer/text + Q.04 premul AA + B.03 Overlay + Tier E DatePicker/Transfer |
| 2026-07-15 | 1.20 | L.06 R8 modulate + P.05/P.06 + B.05 + Tier D 复杂 UI |
| 2026-07-15 | 1.19 | X.05 彩底 two-pass LCD + B.03 dual-tex GPU 合成 |
| 2026-07-15 | 1.5 | P1 closers：I.03 Nearest/Linear、C.05 clip AA、X.05 LCD GPU 门禁 |
| 2026-07-15 | 1.4 | S3a 关闭：M0–M1 GPU 门禁 |
| 2026-07-15 | 1.3 | S2 关闭：webgpu facade |
| 2026-07-15 | 1.3 | P1 复杂 UI 矩阵 A1–A8 + S.05/S.08/B.06 门禁；S.03 真窗口 draw |
| 2026-07-15 | 1.2 | S1 关闭：A–E 烟测；§2.7/§4 更新 |
| 2026-07-15 | 1.1 | §4 rwgpu：S1 枚举 header-lock 已落地 |
| 2026-07-15 | 1.0 | 首版：全面 Skia 2D 能力表 + WebGPU 反推子集 + 里程碑 |

| 2026-07-15 | M3 residual | B.04/C.06/H.04/I.04/I.07/X.08–X.10 实现 + S3c residual 门禁 |
| 2026-07-15 | — | matrix rwgpu/webgpu 🔄→✅ align (render-proven M0–M3) + B.03 Soft/Hard/Dodge gates + Tier N |
| 2026-07-15 | — | B.03 full separable advanced GPU + Tier M |
| 2026-07-15 | — | S.07 WritePixels GPU + Tier L |
| 2026-07-15 | — | B.04 HSL dual-tex GPU + F.03 CM/DropShadow GPU + Tier K |
| 2026-07-15 | — | L.06 stencil cover-inline R8 + F.03 GPU multi-RT + Tier J |
| 2026-07-15 | — | L.06 SDF cover-inline R8 + mask lifetime fix + Tier I dashboard/modal |
| 2026-07-15 | — | L.06 convex cover-inline R8 + Tier H large virtual/transfer |
| 2026-07-15 | — | F.03 filter graph + L.06 MaskAware native upload + Tier G TreeSelect/Carousel |
| 2026-07-15 | — | K.01 Context Compute + Q.02 Coverage AA gates; B.03 ColorBurn/Exclusion; Tier O/P complex UI |
| 2026-07-15 | — | V.03/K.02/CS.02/CS.03 M4 gates + Tier Q/R complex UI |
| 2026-07-16 | — | R.02：2D 画布 100% **不含** PDF/SVG；计划见 CAPABILITY_MATRIX_WINDOW §0/§8/§9 |
| 2026-07-15 | — | E.03 Trim + P.09 Dither + T.04 ImageQuad + L.05 Backdrop + Tier S/T |
| 2026-07-15 | — | E.02 PathEffects + I.08 ExternalTexture + Tier U complex UI |


---

## 3. P1 复杂 UI 场景矩阵（2026-07-15）

> 非控件实现；模拟 Ant Design 级 UI 的 **绘制组合** 压测。

| ID | 场景 | 门禁 | GPUOps |
|----|------|------|--------|
| A1 | Button states | `TestP1_A1_UIButtonStates` | >0 |
| A2 | Input field | `TestP1_A2_UIInputField` | >0 |
| A3 | Menu overlay | `TestP1_A3_UIMenuOverlay` | >0 |
| A4 | Modal mask | `TestP1_A4_UIModalMask` | >0 |
| A5 | Table cells | `TestP1_A5_UITableCells` | >0 |
| A6 | Tabs/badge/tag | `TestP1_A6_UITabsBadge` | >0 |
| A7 | Icon+text mix | `TestP1_A7_UIIconTextMix` | >0 |
| A8 | Scroll clip | `TestP1_A8_UIScrollClip` | >0 |
| B1 | Many rrects | `TestP1_B1_ManyRRectsCorrectness` | >0 |

```bash
export WGPU_NATIVE_PATH=.../lib/libwgpu_native.so
go test -count=1 ./render -run 'TestP1_'
```
| 2026-07-16 | 1.4 | B.07 GPU `BlendModulate`；B.06 multi-path premul 门禁 |
