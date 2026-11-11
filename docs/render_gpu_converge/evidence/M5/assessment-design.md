# M5 HDR 全链：评估与设计（2026-11-11，先文档后实现）

> 台账来源：§2.2 M5（P3.7 D5 派生）。本文件只做评估与设计，**未动代码**；
> 拍板后才开实现批，并按 §4.1 在 §2.1 登记载体。
>
> **2026-11-11 状态：M5 已全批收口（P0–P5 ✅），收口正本见同目录 closeout.md。**

## 0 结论摘要（一句话）

render 层现在只有 CPU 半边 HDR（render/hdr 有 float 缓冲与 tonemap，但 TextureView() 恒 nil）；
GPU 半边从目标格式、离屏 RT、tonemap pass 到呈现输出全缺。建议按标准形态做**格式参数化的
float 面**（RGBA16Float，1x），默认呈现仍是 tonemap→8bit；HDR 直出（10-bit/float swapchain +
色彩空间元数据）不在本项。

## 1 现状实测（file:line，2026-11-11 复核）

| 面 | 现状 | 位置 |
|---|---|---|
| CPU float 目标 | 有，[]float32 RGBA，域可 >1；TextureView()/Pixels() 显式 nil | render/hdr/float_target.go:74,77 |
| CPU tonemap | 曝光(0,16] → Reinhard x/(1+x) → sRGB 8bit | render/hdr/float_target.go:129-163 |
| CPU 生产消费者 | 仅 engine/fx/bloom.go:239,250（CPU 半边）；GPU 半边零 | 台账 M5 实测 |
| GPU 目标格式概念 | GPURenderTarget 无 Format 字段，格式由 View 隐含 | render/accelerator.go:71-99 |
| 离屏 RT | 固定 BGRA8Unorm，池键仅 (w,h,samples,role) | render/internal/gpu/gpu_render_context.go:4172-4187,4129-4131 |
| 管线颜色格式 | 22 个建管线文件、30+ 处硬编码 BGRA8Unorm ColorTargetState | render/internal/gpu/image_pipeline.go:318 等 |
| 呈现 | 就绪探针固定 BGRA8；swapchain 取 sc.GetFormat()（通常 BGRA8） | render/present_construct.go:63,546 |
| 8-bit 离屏 RT 覆盖面 | render/internal/gpu 184 处 RGBA8/BGRA8 格式引用 | 全目录 grep |
| 底座能力 | hal 有逐格式能力查询；GLES RGBA16Float = base+storage+halfFloatRender(含 blend/msaa) | gpu/hal/api.go:104、gpu/gwgpu/gles/capabilities.go:800-801 |
| native 能力查询 | webgpu.Adapter.TextureFormatCapabilities 是**空壳**（返回零值） | gpu/webgpu/adapter.go:114-116 |
| 目标池键 | CacheKey 已有 Format/Samples/Usage/Role 字段，离屏只用 Role | render/internal/gpu/res/cache.go:16-22 |

## 2 标准对齐（为何选 RGBA16Float + 格式参数化管线）

- **Skia**：kRGBA_F16_SkColorType 是一等色彩类型（include/core/SkColorType.h:98，半浮点、
  扩展范围、premul linear）；Skia 的 program/pipeline 键包含目标色彩类型与格式，即**同一绘制
  能力按目标格式出管线变体**，不是为 HDR 另做一套绘制器。
- **Flutter/Impeller**：PixelFormat::kR16G16B16A16Float（impeller/core/formats.h:109）在
  Vulkan/Metal/GLES 三端都有映射（同文件 :312,:665 与各后端 formats_*），浮点面按普通
  render target 使用。
- **本仓底座**：hal 的 TextureFormatCapabilities 已是现成能力口；GLES 半浮点能力按扩展探测；
  池键已留 Format 位。⇒ 缺的是**概念与管线变体**，不是驱动能力。

## 3 缺口分解

- **G1 目标格式概念**：GPURenderTarget.Format（零值=沿用 View 格式），显式表达 8bit/16F。
- **G2 管线颜色格式参数化**：22 个建管线文件把 BGRA8Unorm 换成「目标格式」入参，
  管线缓存键加格式；8bit 默认路径行为零变化。
- **G3 float 离屏 RT**：CreateOffscreenTexture 增加格式参数（池键带 Format + 显存按 8B/px
  记账）；能力不足 fail-closed（具名错 + 回退 8bit，禁止静默）。
- **G4 tonemap/曝光 pass**：RGBA16Float → 8bit（或中间 8bit RT），曲线与 hdr.Encode
  **同一组数字**（曝光→Reinhard→sRGB），GPU/CPU 逐像素 max ≤1。
- **G5 呈现输出**：默认 tonemap 后走现有 8bit 呈现（零 swapchain 改动）；HDR 直出另立。
- **G6 颜色空间语义**：float 面只存**线性 premul**；绘制进 float 面必须走线性混合腿
  （sRGB 解码输入、输出不编码），禁止把 sRGB 编码值写进 float 面。
- **G7 读取口**：float 面的 CPU 对齐口（Resolve(exposure) (*image.RGBA, error)）供金图/对拍；
  GPU 侧保留 view 供后续 pass（bloom/合成）。

## 4 设计决策（待拍板）

- **D1 面格式**：RGBA16Float（8B/px）。备选 RGBA32Float（16B/px，GLES 混合需扩展、
  显存/带宽翻倍）不推荐；RG11B10Ufloat 无 alpha、不适合通用面，仅可做 HDR 直出备选。
- **D2 管线范围**：**全格式参数化**（22 个建管线文件一致改造，标准形态；8bit 金图零变化），
  备选「只给图像/quad+tonemap 做窄 HDR 子集」（省工但形状/文字进不了 float 面，形态不完整）。
- **D3 呈现**：默认 tonemap→8bit（本项）；10-bit/float swapchain + 色彩空间元数据登记后续
  （当前仓无窗口系统 HDR 元数据链）。
- **D4 曲线/曝光**：复用 hdr.Encode 的「曝光 → Reinhard → sRGB」，曝光域 (0,16]；
  ACES/Filmic 仅作枚举可选项、默认 Reinhard（与 engine/fx.DefaultTonemap 一致，禁第二套数字）。
- **D5 MSAA**：float 面首版 **1x only**；MSAA+float 组合具名拒绝（或降 1x 并记账），
  4x resolve 到 16F 登记验证项。
- **D6 能力失败语义**：目标格式不可渲染时 ErrHDRUnsupported 具名错 + 回退 8bit（可观测计数），
  禁止静默黑屏/降级。

## 5 分批（每批先红后绿 + 门禁）

- **P0 底座探针（零 API）**：真 GPU 上建 RGBA16Float RT、单三角形/清屏、读回 >1 值、混合一次；
  native + GLES 双腿。红条件=建面/管线/读回任一失败。此步先证明「驱动地基」不是纸面结论。
- **P1 格式参数化（行为零变化）**：G1+G2；8bit 全量金图 0.0000%、双后端一致、API 仅内部面。
- **P2 float 离屏 RT**：G3+G7；池键/账本/OOM/能力回退；单测（分配/复用/释放/账本）。
- **P3 tonemap pass**：G4；shader 与 hdr.Encode 同式；单测逐像素 max ≤1（禁 mean）。
- **P4 呈现接线 + 真窗**：G5；新窗 ui_wr_m5_hdr（三档曝光：0.5/1/4，一半 HDR 一半 CPU 参考），
  逻辑探针 + 画面像素断言 + 金图（双后端各自基线）；SVG/既有窗金图零变化。
- **P5 收口**：双后端矩阵 + 降级负向 + catalog/apidoc + 台账/证据。

## 6 风险与实现前必须验证

1. native 端无格式能力查询（空壳）——只能靠创建/管线校验失败探测；P0 探针必须把
   「不可渲染」与「OOM」区分开（复用 K6 的错误分类）。
2. 全格式参数化触及 22 个管线文件；任何一处漏改在 float 面上会以管线校验失败暴露，
   P1 必须用「8bit 全量金图零变化 + float 面单点绘制」双门禁。
3. 线性语义：float 面必须走线性混合腿；现有 BlendSpaceSRGB 默认值是 8bit 历史行为，
   HDR 面需强制/校验为 Linear，否则 tonemap 结果会系统性偏亮。
4. 显存：1080p RGBA16Float ≈ 8.3MB/面；池化/预算按 DefaultBudget 域追加，不许绕过账本。
5. 与滤镜/图层的交互（float 面进 filter graph、layer 合成）首版不做，登记为后续。

## 7 待拍板

D1 面格式、D2 管线范围、D3 呈现边界、D4 曲线/曝光、D5 MSAA、D6 失败语义。

## 8 拍板结果（2026-11-11，用户确认）

| 决策 | 拍板 |
|---|---|
| D1 面格式 | **RGBA16Float**（8B/px） |
| D2 管线范围 | **全格式参数化（22 个建管线文件）**，8bit 默认行为零变化作门禁 |
| D3 呈现 | **tonemap→8bit 呈现**；10-bit/float swapchain + 色彩空间元数据另立 |
| D4 曲线/曝光 | **复用 hdr.Encode**：曝光(0,16]→Reinhard→sRGB，默认 Reinhard |
| D5 MSAA | **float 面首版 1x**；MSAA+float 具名拒绝/降级并记账，4x resolve 登记验证项 |
| D6 失败语义 | 不可渲染 ⇒ ErrHDRUnsupported 具名错 + 可观测回退 8bit（禁静默） |

## 9 P0 底座探针结果（2026-11-11，native 已过）

新增 render/internal/gpu/hdr_16f_probe_test.go::TestM5HDR16FRenderTargetProbe：
32×32 RGBA16Float 面，清黑 → 画 (3.0,0.5,0.25,1.0) → 第二 pass LoadOpLoad +
SrcAlpha/OneMinusSrcAlpha 混合 (2.0,0,0,0.5) → CopyTextureToBuffer 读回半浮点位型。

- 实测：center half bits got=4100/3400/3000/3a00 want=4100/3400/3000/3a00（PASS）。
- 结论：native 端 16F **可建、可渲、可混合、>1 值不钳**，驱动地基结论成立。
- GLES 腿：能力位（GL_EXT_color_buffer_half_float/float，capabilities.go:800）在案，
  真实绘制验收放 P4 真窗（gles 无 surfaceless 独立设备口径）；metal 本机不可验。
- 未动任何生产代码/API；P1 格式参数化未开。
