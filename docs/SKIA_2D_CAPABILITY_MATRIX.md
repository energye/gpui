# Skia 级 2D 渲染能力表（GPUI 主线验收基准）

> 版本：1.0 | 日期：2026-07-15  
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
| S.02 | GPU 离屏 RT | `SkSurface::MakeRenderTarget` | Texture RenderAttachment + resolve | 🔄 | 🔄 | 🔄 | M0 |
| S.03 | 窗口 Surface/Swapchain | `MakeFromBackendRenderTarget` / Window | Surface/Swapchain/Present/配置 | 🔄/⬜ | 🔄/⬜ | ⬜ 外部 view | M3 |
| S.04 | 清屏/clear 色 | `canvas->clear` | LoadOpClear + clearValue | ✅ | ✅ | ✅ Clear/ClearWithColor | M0 |
| S.05 | 尺寸/resize | surface 重建 | 重建 texture/pipeline 依赖 | 🔄 | 🔄 | 🔄 | M1 |
| S.06 | 读回像素 | `peekPixels` / `readPixels` | copyTextureToBuffer + map | ✅ | ✅ | ✅ Image/SavePNG/FlushGPU | M0 |
| S.07 | 写像素/上传 | `writePixels` | queue.writeTexture/writeBuffer | ✅ | ✅ | 🔄 | M0 |
| S.08 | DPR / 逻辑像素 | surface props scale | 视口/纹理物理尺寸 | N/A | N/A | 🔄 deviceScale | M1 |
| S.09 | 部分更新/damage | dirty rect present | scissor + LoadOpLoad | 🔄 | 🔄 | 🔄 | M3 |

### 1.2 变换 (Matrix)

| ID | 能力 | Skia 参考 | WebGPU 需求 | rwgpu | webgpu | render | Pri |
|----|------|-----------|-------------|-------|--------|--------|-----|
| T.01 | 2D 仿射 Translate/Scale/Rotate/Skew | `concat`/`setMatrix` | Uniform / vertex xform | N/A | N/A | ✅ | M1 |
| T.02 | Save/Restore CTM | `save`/`restore` | 栈在 CPU | N/A | N/A | ✅ | M1 |
| T.03 | 非均匀缩放 stroke | stroke 受 CTM | 管线或 CPU 展平 | N/A | N/A | 🔄 | M2 |
| T.04 | 透视/非仿射（可选） | `SkMatrix` persp | 网格细分/homography | N/A | N/A | ⬜ | M4 |

### 1.3 Paint：颜色、样式、描边

| ID | 能力 | Skia 参考 | WebGPU 需求 | rwgpu | webgpu | render | Pri |
|----|------|-----------|-------------|-------|--------|--------|-----|
| P.01 | Solid color RGB/A | `setColor` | premul blend 输入 | N/A | N/A | ✅ | M0 |
| P.02 | Style Fill/Stroke/StrokeAndFill | `setStyle` | 几何生成 | N/A | N/A | ✅ | M1 |
| P.03 | Stroke width | `setStrokeWidth` | 展平/SDF | N/A | N/A | ✅ | M1 |
| P.04 | Hairline（设备 1px） | width=0 语义 | 像素对齐 stroke | N/A | N/A | 🔄 | M1 |
| P.05 | Cap Butt/Round/Square | `setStrokeCap` | 几何 | N/A | N/A | ✅/🔄 | M1 |
| P.06 | Join Miter/Round/Bevel | `setStrokeJoin` | 几何 | N/A | N/A | ✅/🔄 | M1 |
| P.07 | Miter limit | `setStrokeMiter` | 几何 | N/A | N/A | 🔄 | M2 |
| P.08 | Anti-alias 开关 | `setAntiAlias` | MSAA 或 coverage AA | 🔄 MSAA | 🔄 | ✅ SetAntiAlias | M1 |
| P.09 | Dither（可选） | `setDither` | shader | N/A | N/A | ⬜ | M4 |

### 1.4 Blend / Alpha / Premul

| ID | 能力 | Skia 参考 | WebGPU 需求 | rwgpu | webgpu | render | Pri |
|----|------|-----------|-------------|-------|--------|--------|-----|
| B.01 | SrcOver（默认） | `kSrcOver` | BlendState premul | ✅ enum | ✅ | 🔄 GPU 已验 SourceOver | M1 |
| B.02 | Clear/Src/Dst/… Porter-Duff | `SkBlendMode` PD | blend factor 或 shader blend | 🔄 | 🔄 | 🔄 CPU 多；SetBlendMode 弱 | M2 |
| B.03 | Multiply/Screen/Overlay… | separable modes | shader blend 常见 | 🔄 | 🔄 | 🔄 CPU blend 包 | M2 |
| B.04 | HSL 模式 Hue/… | non-separable | shader | 🔄 | 🔄 | 🔄 | M3 |
| B.05 | Premul 约定贯穿 | premul pipeline | texture/blend 一致 | 🔄 | 🔄 | 🔄 文档+部分测试 | M1 |
| B.06 | 全局 alpha | paint alpha | uniform/premul | N/A | N/A | 🔄 | M1 |
| B.07 | Plus/Modulate 等 | `kPlus`… | blend/shader | 🔄 | 🔄 | 🔄 | M2 |

### 1.5 Path 构建与填充

| ID | 能力 | Skia 参考 | WebGPU 需求 | rwgpu | webgpu | render | Pri |
|----|------|-----------|-------------|-------|--------|--------|-----|
| H.01 | Move/Line/Quad/Cubic/Close | `SkPath` | CPU path → GPU mesh/stencil | N/A | N/A | ✅ | M1 |
| H.02 | Arc/圆角路径 | `arcTo` | 细分 | N/A | N/A | ✅ | M1 |
| H.03 | Fill rule NonZero/EvenOdd | `setFillType` | stencil 或 winding | 🔄 stencil | 🔄 | ✅ | M1 |
| H.04 | Path 布尔（可选） | `Op` | CPU | N/A | N/A | 🔄/⬜ | M3 |
| H.05 | Path measure/长度 | `SkPathMeasure` | CPU | N/A | N/A | 🔄 | M3 |
| H.06 | 复杂 path GPU 光栅 | GrPathRenderer 类 | stencil-then-cover / tess / compute | 🔄 | 🔄 | 🔄 stencil/convex/vello | M2 |
| H.07 | Convex path 快路径 | convex | 专用 pipeline | 🔄 | 🔄 | 🔄 | M2 |

### 1.6 图元（可归约为 Path）

| ID | 能力 | Skia 参考 | WebGPU 需求 | rwgpu | webgpu | render | Pri |
|----|------|-----------|-------------|-------|--------|--------|-----|
| G.01 | Rect | `drawRect` | fill/stroke | N/A | N/A | ✅ | M1 |
| G.02 | Oval/Circle | `drawOval`/`drawCircle` | SDF 或 path | N/A | N/A | ✅ + SDF GPU | M1 |
| G.03 | RRect | `drawRRect` | SDF/path | N/A | N/A | ✅ + SDF | M1 |
| G.04 | Line / 折线 | `drawLine`/`drawPoints` | stroke | N/A | N/A | ✅ | M1 |
| G.05 | Arc | `drawArc` | path | N/A | N/A | ✅ | M1 |
| G.06 | RoundRect 变体 XY 半径 | `SkRRect` | path | N/A | N/A | 🔄 | M2 |

### 1.7 Clip

| ID | 能力 | Skia 参考 | WebGPU 需求 | rwgpu | webgpu | render | Pri |
|----|------|-----------|-------------|-------|--------|--------|-----|
| C.01 | Clip rect | `clipRect` | scissor 或 stencil | ✅ scissor | ✅ | ✅ | M1 |
| C.02 | Clip rrect | `clipRRect` | stencil/SDF mask | 🔄 | 🔄 | 🔄 | M2 |
| C.03 | Clip path | `clipPath` | stencil | 🔄 | 🔄 | 🔄 | M2 |
| C.04 | Clip 栈 / 交 | clip stack | stencil ref / mask | 🔄 | 🔄 | ✅ 栈 | M1 |
| C.05 | Clip AA | aa clip | coverage/stencil MSAA | 🔄 | 🔄 | 🔄 | M2 |
| C.06 | Replace/Difference clip op | `SkClipOp` | stencil ops | 🔄 | 🔄 | 🔄 | M3 |

### 1.8 Layer / SaveLayer

| ID | 能力 | Skia 参考 | WebGPU 需求 | rwgpu | webgpu | render | Pri |
|----|------|-----------|-------------|-------|--------|--------|-----|
| L.01 | Save/Restore | `save`/`restore` | CPU 栈 | N/A | N/A | ✅ | M1 |
| L.02 | SaveLayer 离屏 | `saveLayer` | 离屏 RT + composite | 🔄 | 🔄 | 🔄 PushLayer CPU 为主 | M2 |
| L.03 | Layer opacity | saveLayer alpha | premul composite | 🔄 | 🔄 | 🔄 | M2 |
| L.04 | Layer blend mode | saveLayer blend | blend/shader | 🔄 | 🔄 | 🔄 | M2 |
| L.05 | Layer + backdrop（可选） | backdrop filter | 采样背景 | ⬜ | ⬜ | ⬜ | M4 |
| L.06 | Mask layer | mask filter/clip mask | R8 mask texture | 🔄 R8 | 🔄 | 🔄 Mask API | M2 |

### 1.9 Shader / Gradient / Pattern

| ID | 能力 | Skia 参考 | WebGPU 需求 | rwgpu | webgpu | render | Pri |
|----|------|-----------|-------------|-------|--------|--------|-----|
| D.01 | Linear gradient | `SkGradientShader::MakeLinear` | 1D tex 或 shader | 🔄 | 🔄 | 🔄 Brush API | M2 |
| D.02 | Radial gradient | `MakeRadial` | shader/tex | 🔄 | 🔄 | 🔄 | M2 |
| D.03 | Sweep/conic | `MakeSweep` | shader | 🔄 | 🔄 | 🔄 | M2 |
| D.04 | 多 stop / tile mode | clamp/repeat/mirror | sampler address | ✅ sampler | ✅ | 🔄 | M2 |
| D.05 | Image shader/pattern | `SkImage::makeShader` | texture sample | ✅ | ✅ | 🔄 ImagePattern | M2 |
| D.06 | Local matrix on shader | localMatrix | uniform | N/A | N/A | 🔄 | M2 |

### 1.10 Image / Bitmap

| ID | 能力 | Skia 参考 | WebGPU 需求 | rwgpu | webgpu | render | Pri |
|----|------|-----------|-------------|-------|--------|--------|-----|
| I.01 | DrawImage | `drawImage` | textured quad | ✅ | ✅ | 🔄 GPU quad | M2 |
| I.02 | DrawImageRect/src-dst | `drawImageRect` | UV rect | ✅ | ✅ | ✅/🔄 | M2 |
| I.03 | 过滤 Nearest/Linear | sampling | FilterMode | ✅ | ✅ | 🔄 | M2 |
| I.04 | Cubic/mipmap（可选） | cubic sampling | mip / custom | 🔄 | 🔄 | ⬜ | M3 |
| I.05 | Opacity / alpha image | paint alpha | premul | 🔄 | 🔄 | 🔄 | M2 |
| I.06 | 旋转/CTM 下图像 | concat + drawImage | 任意 quad | ✅ | ✅ | ✅ 四角点 | M2 |
| I.07 | 九宫格（可选） | lattice | 多 quad | N/A | N/A | ⬜ | M3 |
| I.08 | YUV/外部纹理（可选） | external | multiplanar | ⬜ | ⬜ | ⬜ | M4 |

### 1.11 Text / Font / Glyph

| ID | 能力 | Skia 参考 | WebGPU 需求 | rwgpu | webgpu | render | Pri |
|----|------|-----------|-------------|-------|--------|--------|-----|
| X.01 | 字体加载/Face | SkTypeface | CPU | N/A | N/A | ✅ | M1 |
| X.02 | DrawString baseline | `drawString` | glyph atlas tex | 🔄 R8 atlas | 🔄 | 🔄 GPU/CPU | M2 |
| X.03 | Glyph 位置 shaping | shape + pos | CPU shape | N/A | N/A | 🔄 | M2 |
| X.04 | Subpixel positioning | subpixel | atlas/frac | N/A | N/A | 🔄 | M2 |
| X.05 | Edging: alias/anti-alias/subpixel LCD | edging | RGB mask / blend | 🔄 | 🔄 | 🔄 LCDLayout | M2 |
| X.06 | CJK / fallback 字体 | fallback | 同 text | N/A | N/A | 🔄 | M2 |
| X.07 | 路径化文本 | text → path | path 管线 | N/A | N/A | ✅ outline | M2 |
| X.08 | 文本装饰 underline 等 | decorations | 几何 | N/A | N/A | ⬜/🔄 | M3 |
| X.09 | 变体字体 variable | variations | CPU | N/A | N/A | 🔄 | M3 |
| X.10 | Emoji/彩色字体 | CBDT/SBIX/… | RGBA atlas | 🔄 | 🔄 | 🔄 | M3 |
| X.11 | Glyph atlas 管理 | strike cache | texture atlas + upload | 🔄 | 🔄 | 🔄 | M2 |

### 1.12 Path Effect

| ID | 能力 | Skia 参考 | WebGPU 需求 | rwgpu | webgpu | render | Pri |
|----|------|-----------|-------------|-------|--------|--------|-----|
| E.01 | Dash | `SkDashPathEffect` | CPU 展平 stroke | N/A | N/A | 🔄 | M2 |
| E.02 | Corner/1D/2D path effect | path effects | CPU | N/A | N/A | ⬜ | M4 |
| E.03 | Trim path（可选） | trim | CPU | N/A | N/A | ⬜ | M4 |

### 1.13 MaskFilter / ImageFilter（质量项）

| ID | 能力 | Skia 参考 | WebGPU 需求 | rwgpu | webgpu | render | Pri |
|----|------|-----------|-------------|-------|--------|--------|-----|
| F.01 | Blur mask filter | `SkMaskFilter::MakeBlur` | 离屏 + blur pass | 🔄 | 🔄 | ⬜/🔄 | M3 |
| F.02 | Drop shadow | image filter | multi-pass | 🔄 | 🔄 | ⬜ | M3 |
| F.03 | 通用 image filter 图 | `SkImageFilter` DAG | 多 RT ping-pong | 🔄 | 🔄 | ⬜ | M4 |
| F.04 | Color filter | `SkColorFilter` | shader/LUT | 🔄 | 🔄 | ⬜/🔄 | M3 |

### 1.14 Vertices / 网格 / Atlas 精灵

| ID | 能力 | Skia 参考 | WebGPU 需求 | rwgpu | webgpu | render | Pri |
|----|------|-----------|-------------|-------|--------|--------|-----|
| V.01 | DrawVertices | `drawVertices` | vertex color/uv pipeline | ✅ buffer | ✅ | ⬜/🔄 | M3 |
| V.02 | DrawAtlas | `drawAtlas` | instanced quads | ✅ | ✅ | 🔄 | M3 |
| V.03 | Mesh（可选） | `drawMesh` | 自定义 shader | 🔄 | 🔄 | ⬜ | M4 |

### 1.15 MSAA / 质量 / 像素惯例

| ID | 能力 | Skia 参考 | WebGPU 需求 | rwgpu | webgpu | render | Pri |
|----|------|-----------|-------------|-------|--------|--------|-----|
| Q.01 | MSAA 4x + resolve | samples | multisample texture + resolve | 🔄 | 🔄 | 🔄 | M2 |
| Q.02 | Coverage AA 无 MSAA | analytic AA | 软件/计算覆盖 | N/A | N/A | 🔄 | M1 |
| Q.03 | 像素对齐/近像素规则 | device pixel snap | CPU/GPU 一致 | N/A | N/A | 🔄 | M2 |
| Q.04 | 半透明边缘与 premul | premul AA | blend | 🔄 | 🔄 | 🔄 | M1 |

### 1.16 颜色空间 / 位深（可分阶段）

| ID | 能力 | Skia 参考 | WebGPU 需求 | rwgpu | webgpu | render | Pri |
|----|------|-----------|-------------|-------|--------|--------|-----|
| CS.01 | sRGB 默认 | color space | sRGB texture format 可选 | 🔄 | 🔄 | 🔄 默认 8bit | M3 |
| CS.02 | F16 / 宽色域（可选） | F16 surface | rgba16float | 🔄 | 🔄 | ⬜ | M4 |
| CS.03 | 线性混合 vs sRGB 混合 | linear blending | 格式/shader | 🔄 | 🔄 | 🔄 实验 | M4 |

### 1.17 录制 / 回放 / 文档后端（后置）

| ID | 能力 | Skia 参考 | WebGPU 需求 | rwgpu | webgpu | render | Pri |
|----|------|-----------|-------------|-------|--------|--------|-----|
| R.01 | Picture 录制回放 | `SkPicture` | 无（命令表） | N/A | N/A | 🔄 recording | M3 |
| R.02 | PDF/SVG 后端 | document | 无 GPU | N/A | N/A | ⬜ | M4 |

### 1.18 计算路径 / 高级 GPU（可选增强）

| ID | 能力 | Skia/行业参考 | WebGPU 需求 | rwgpu | webgpu | render | Pri |
|----|------|---------------|-------------|-------|--------|--------|-----|
| K.01 | Compute 粗光栅/分片 | Vello/Pathfinder 类 | compute pipeline/storage | ✅ 部分 | ✅ 部分 | 🔄 vello | M3 |
| K.02 | 间接绘制 | multi-draw | indirect buffer | 🔄 | 🔄 | ⬜ | M4 |

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
| rwgpu | 设备/缓冲/纹理/管线/pass/copy 大部分有；enum 仅部分严格对齐 header | 静默错拓扑/stencil/blend |
| webgpu | facade 已接 rwgpu；转换/lifetime 需按子集证明 | 半接/漏字段 |
| render CPU | path/text/image/clip/layer 较全 | GPU 不一致 |
| render GPU | SDF/stencil/image/text/atlas 混合；fallback 多 | 语义漂移 |
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
| 2026-07-15 | 1.0 | 首版：全面 Skia 2D 能力表 + WebGPU 反推子集 + 里程碑 |
