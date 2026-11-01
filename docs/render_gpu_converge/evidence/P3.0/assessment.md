# P3.0 影响面评估（2026-11-01）

> 任务：P3.0 依赖图 + 文档更新（PLAN §5 + `RENDER_API_CATALOG.md`），再按图排 P3.1–P3.8 实现顺序；**文档没写完不写代码**（PLAN §7.4、台账 §3）。
> 前置：P2.6 ✅（`evidence/P2.6/summary.md`，P2 全阶段行为零变化成立）；P3.1（线性混合前移）/P3.3（逐 run 样式前移）为固定前移项，不得挪回 P4（台账 §3）。
> 性质：纯文档任务，不改任何代码；本评估为开工登记的依据，方向待用户拍板后才动手写正式文档。

## 0. 口径与方法

- 依据文档链：AGENTS.md → `docs/render_gpu_converge/00_README.md` → `ENGINE_RENDER_GPU_CONVERGE_PLAN.md` §5/§6/§7.4 → `ENGINE_RENDER_GPU_CONVERGE_TASKS.md` §2/§3 → `evidence/P2.6/summary.md`。
- 调查方法：三路只读扫描（色彩/文本、shader/路径/图片、滤镜/2.5D/SVG/录制）+ 本机补查（mesh gradient/EXIF/渐变家族/apidoc），全部结论带 file:line；调查于 2026-11-01 在 HEAD `da9168a5` 工作区完成。
- P3.0 产出范围（按台账 §3 口径）：依赖图、PLAN §5 缺失能力清单刷新、`RENDER_API_CATALOG.md` 更新、P3.1–P3.8 实现顺序排定。**各项能力的详细文档（能力定义/公开 API/双后端实现方式/验证方案）留各 P3.x 自己的开工登记做**（台账 §3「P3.1–P3.8：每项先补文档……再实现」），P3.0 不越位写八份能力定义书。

## 1. 逐能力现状盘点（PLAN §5 对照代码实测）

> 每行给出 PLAN 原登记 → 实测现状 → 结论（成立/需精确化/需修正）。行号以 HEAD `da9168a5` 为准。

### 1.1 P3.1 色彩管理地基（线性混合接线，前移项）

| 项 | PLAN 原登记 | 实测现状 |
|---|---|---|
| 线性混合 helper | §6：helper 写好未接线（`render/internal/blend/linear.go`） | **成立**。`GetBlendFuncLinear(mode, linear)`（linear.go:37）sRGB↔linear 转换后混合再转回；便捷封装 `BlendLinear`（:126）；包装类型 `BlendModeLinear`（:25）。全仓生产调用 **0**（仅 linear_test.go） |
| 主路混合空间 | — | GPU 主路=固定功能混合、在 sRGB 编码值上直接混（`render/internal/gpu/blend_gpu.go:55,73-177`，目标 `BGRA8Unorm` 非 sRGB，`render/present_construct.go:63`）；CPU 主路同样字节直接混（`render/software_blend.go:32`）。两条主路都非线性混合 |
| 公开色彩 API | §5.1：色彩管理 HDR/广色域/ICC → render/hdr + gpu | render 公开层 **无** 任何 ColorSpace/sRGB/linear 开关（唯一命中是未导出的渐变 stop 插值 `render/gradient.go:108`；`render/color.go:165` Lerp 是字节直插） |
| `render/hdr` | §7.5 依赖 | FloatTarget 纯 CPU staging（RGBA16Float 标记、`TextureView()` 恒 nil），全仓零消费者（catalog §7.2 已登记） |
| sRGB 交换链 | §4 缺口行：三端都按 gamma 混合，列入缺口 | **成立**。webgpu 显式偏好非 Srgb（`gpu/webgpu/swapchain_configure.go:152-159`）；gles 声明了 `EGL_GL_COLORSPACE_SRGB_KHR` 未使用（`gpu/gwgpu/gles/egl/types.go:210`）；metal 有 Srgb→MTLPixelFormat 映射但默认仍非 Srgb（`gpu/gwgpu/metal/conv.go:56-67`、`surface.go:84-85`） |
| Float RT 硬件面 | — | 双后端格式管道已备：gles RGBA16F/32F 渲染目标（`gles/capabilities.go:728-745,800-810`）、metal RGBA16Float/32Float（`metal/types.go:114-148`）；render 层离屏/层纹理一律 RGBA8Unorm（`gpu_render_context.go:2511,2519`），零使用 |

**结论**：P3.1 地基=①`GetBlendFuncLinear` 接进主路（GPU 管线/CPU 合成）+ ②公开色彩空间 API 面 + ③（范围待 P3.1 开工拍板）sRGB 交换链/离屏 RT 格式。线性混合是 HDR 的地基（PLAN §7.8-4），先做成立。

### 1.2 P3.2 自定义 shader

| 项 | PLAN 原登记 | 实测现状 |
|---|---|---|
| 公开 API | §4 矩阵：三端都缺，列入缺口 | **成立**。render 公开层零 shader API；最接近的三物都不是：`DrawShaderMask`（render/draw_shader_mask.go:20，Brush 掩码）、`CustomBrush/ColorFunc`（brush_custom.go:14，CPU 颜色函数）、`vertices_mesh.go:453` 注释明确 "Full custom fragment shaders are deferred" |
| hal 层 | — | 接口已备：`hal.Device.CreateShaderModule(WGSL)`+`DestroyShaderModule`（`gpu/hal/api.go:235-239`、`descriptor.go:365-366`、`resource.go:96-98`） |
| 内部主路 | — | 15 个渲染 WGSL `go:embed` 直喂 hal，不经翻译器（`render/internal/gpu/shader_helper.go:15`；shaders/ 与 tilecompute/shaders/ 共 25 个 .wgsl） |
| 翻译器 | P2.1 ✅ | `gpu/shader`（naga）公开面 Parse/Lower/LowerWithCapabilities/Validate + glsl.Compile/msl.Compile；**唯一使用方是 gles/metal 后端内部**（`gles/shader.go:155-168`、`metal/device.go:552-628`），render/internal 不 import gpu/shader |

**结论**：P3.2 依赖 P2.1（✅ 已满足）+ P3.0；hal 口现成、render 新 API 是主工作量。标准对齐目标=Skia `SkRuntimeEffect` / Flutter `FragmentProgram`（写进 P3.2 自己的能力定义）。

### 1.3 P3.3 富文本（逐 run 样式，前移项）

| 项 | PLAN 原登记 | 实测现状 |
|---|---|---|
| DrawParagraph 逐 run | §6：单色（context_misc_draw.go:265-285） | **成立**。`DrawParagraph(layout, x, y)`（`render/context_misc_draw.go:276`），:270-272 注释自述 "per-run colors/styles are a ParagraphBuilder concern that the render layer does not own yet"；:288 循环逐 run 调 `DrawShapedGlyphs`，而后者在 `text_dispatch.go:167` 统一取单一 `FromColor`——一个颜色套所有 run |
| 逐段富文本样式 | §5.2：render 层逐段富文本样式（颜色/装饰）缺 | **成立**。render/text 无 Span/RunStyle/TextStyle/Paragraph/ParagraphStyle 任何类型；`ShapedRun`（shaped_run.go:15-36）是 shaping 产物（按 script/direction/face 切分），非样式容器 |
| Justify | §5.2：缺 | **成立（半）**。有常量无实现：`AlignJustify`（layout.go:30）、`applyAlignment` 对 Justify 直接 return 退化左对齐（layout.go:617-620） |
| ellipsis / maxLines | §5.2：缺（下沉到 render） | **成立**。全 render/ 零命中；现有只有 MaxWidth+WrapMode（layout.go:50-79） |
| 公开文本面 | — | DrawString 族（text_entry.go:27-352）+ DrawShapedGlyphs/DrawShapedColorGlyphs（text_dispatch.go:147/206）+ DrawParagraph + TextPath/描边；无 `render.Text` 类型 |
| COLRv1（§5.2 相关行） | §6：只取基础层（colr.go:177,248） | **需修正口径，比登记的更糟**：colr.go:177-180 自称支持 v0+basic v1，但实际只解析 v0 表（`parseBaseGlyphs`:197/`parseLayers`:219，无 Paint/ClipList/VarIndex 解析），**v1 字体会被当 v0 误读**；CPAL v1 同样按基础色提取（:248）。彩色字形生产链=CBDT+COLR v0 双解析器（color_font_adapter.go:18-34）→ ui/rendering（text.go:1152、text_picture.go:55）；sbix 解析器已实现但零消费者（bitmap.go:127） |

**结论**：P3.3 前移项成立；逐 run 样式 + Justify + ellipsis/maxLines 都在 render/text，公开 API 面是 ParagraphBuilder 式（对齐 Flutter `ParagraphBuilder`/Skia `SkShaper::StyledRuns`，细节 P3.3 开工定）。

### 1.4 P3.4 滤镜家族

| 项 | PLAN 原登记 | 实测现状 |
|---|---|---|
| MaskFilter | §5.1：缺 | **成立**。grep 零命中；现有 mask 能力服务掩码合成（mask.go:15）与字形/裁剪采样（internal/gpu/mask*.go），无 Skia 式 MaskFilter（对内容做模糊/羽化）概念 |
| ColorFilter 家族 | §5.1：blend/lighting/compose/table 缺 | **需精确化**：无独立 ColorFilter 抽象，但 **ColorMatrix（4x5）已有双链路**——CPU `Context.ApplyColorMatrix`（filter_ops_cpu.go:134）+ 预设 9 个（brightness/contrast/saturation/grayscale/sepia/invert/hueRotate/opacity/tint，internal/filter/colormatrix.go:30-177）+ GPU 图节点（filter_ops_gpu.go:439，连续节点合并 :546-560）；缺的是 blend/lighting/compose/table 四族与 LUT |
| ImageFilter 家族 | §5.1：erode/dilate/lighting/offset/tile/merge/displacement/alpha-threshold 缺 | **需精确化**：已有 blur/blurXY（filter/blur.go:24,42）、DropShadow（shadow.go:23）、高斯/Box kernel 缓存（kernel.go:25,66）；缺的正是 PLAN 列的 8 族。接入方式=Context 方法族（ApplyBlur/ApplyColorMatrix/ApplyImageFilterGraph，filter_ops_gpu.go:487）+ `render/filters/register.go` 副作用注册 + GPU 图注入（gpu_shared.go:1264-1286）；Paint 上无 ImageFilter 字段 |
| scene.FilterChain | — | 存在（scene/filter.go:25,80）但 render/ui 回放链路零消费 |

**结论**：P3.4 成立；PLAN §5.1 该行要补「已有 blur/colormatrix/dropshadow 双链路」的现状脚注，缺口按 PLAN 原清单。

### 1.5 P3.5 路径与几何

| 项 | PLAN 原登记 | 实测现状 |
|---|---|---|
| conic 路径段 | §5.1：缺 | **成立**。verb 全集 MoveTo/LineTo/QuadTo/CubicTo/Close（path_verb.go:35-44），无 Conic；Arc 是入口级逼近（path_build.go:179、context_path.go:388-471） |
| 四角独立圆角 | §5.1：缺 | **成立**。`ClipRoundRect` 单半径（context_clip.go:86）；internal/clip `RRectClip` 单半径且注释明确 uniform（stack.go:22-26）；scene.RoundRectShape 是 X/Y 椭圆角非逐角（shape.go:195-199） |
| 4×4 透视矩阵 | §5.1：缺 | **成立**。`Matrix{A..F}` 纯 2x3 仿射（matrix.go:25）；CPU 栅格 TransformPoint 2D（raster/edge_builder.go:49-56），perspective 仅 GPU 注释提及（m4_dither.go:90） |
| 路径效果 1D/2D/Sum/Compose | §5.1：缺 | **需精确化**：dash 完整（dash.go:18/40/69 + canvas.go:94-96）；`Path.WithCorners`（CornerPathEffect 子集，pathops_subpath.go:391）与 `Path.Discrete`（DiscretePathEffect 子集）已有；缺 PathEffect 抽象与 2D/Sum/Compose |
| 布尔运算保留曲线 | §5.1：缺 | **需精确化**：布尔运算**已有**（Clipper Vatti 扫描线，PathOpUnion/Intersect/Difference/Xor，path_boolean.go:22-98），但曲线 flatten 0.1px 折线输出——缺的是「保留曲线」版本，PLAN 行成立但口径要写清 |

**结论**：P3.5 成立，两行加现状脚注（dash/布尔已有基座）。

### 1.6 P3.6 图片格式 + GPU mipmap

| 项 | PLAN 原登记 | 实测现状 |
|---|---|---|
| GIF/APNG/动图 WebP/AVIF/HEIF/EXIF | §5.1：缺 | **成立**。解码仅 PNG/JPEG/WebP 静态（internal/image/io.go:20-25,102-132）；动图零支持；EXIF 零命中；公开入口 `Context.LoadImage/LoadWebP/NewImageBuf`（context_image_buf.go:111-123） |
| mesh gradient | §5.1：缺 | **成立**（本机补查 grep 零命中；渐变家族现有 Linear/Radial/Sweep 数据式+函数式） |
| GPU mipmap 链 | §4/§5.1：三端都缺 | **成立（含现状）**：所有 CreateTexture 写死 `MipLevelCount: 1`（image_cache.go:362,377 等 15 处）；采样器 MipmapFilter 能力已在但空转（hal/descriptor.go:275-276、image_pipeline.go:950-985 sampler_contract 冻结）；`UseMipmaps` 公开选项 GPU 路径只是 bilinear 近似（context_image_draw.go:129）；真 mip 链只有 CPU（image/mipmap.go:25 box filter） |

**结论**：P3.6 成立；mipmap 交付物=纹理侧 MipLevelCount + 上传/生成链路，采样器侧已备。

### 1.7 P3.7 2.5D

| 项 | PLAN 原登记 | 实测现状 |
|---|---|---|
| 透视相机与投影矩阵 | §5.3：render/matrix + 模块库 | **需精确化**：2.5D 模块库=`engine/`（仓库根单 module，doc.go:11 接口冻结 2.5D-IF-1.0），**Projector 已在引擎层**（engine/camera/project.go:22 DepthToScale/Project/ProjectQuad/Unproject）+ YSort25D/ShadowMath25D（ysort25d.go:92,97）+ Shadow25D/parallax；缺的是 render/matrix 的 4×4 透视（P3.5 交付）与 render 层接收透视参数的绘制口 |
| 世界高度/精灵 z 序 | §5.3：2.5D 模块库 | **部分已有**：引擎层 YSort25D/Sprite.DepthSort 在用（stage_chase main.go:598 构建期排序、:899-910 校验）；render 层 `DepthSprite/SortDepthSprites/DrawDepthSprites` 拍平批绘已有（depth_r5.go:43/99/132）但 stage_chase 未用 |
| 自定义材质 | §5.3：依赖自定义 shader | **成立**，即依赖 P3.2（台账依赖列漏记，见 §2） |
| HDR/泛光 | §5.3：依赖 render/hdr | **成立**，即依赖 P3.1 |
| GPU mipmap | §5.3：依赖 render/gpu | 由 P3.6 交付，2.5D 消费 |

**结论**：P3.7 的四个子项有三个已在 PLAN §5.3 标了依赖；依赖列要按此修订（§2）。

### 1.8 P3.8 SVG 完整语义 / 彩色字形 / 嵌套 Picture

| 项 | PLAN 原登记 | 实测现状 |
|---|---|---|
| SVG 完整语义 | §5.1：defs/use/渐变/clipPath/text/filter 缺 | **成立**。现支持 path/circle/rect/ellipse/line/polygon/polyline/g + fill/stroke/opacity/transform（parser.go:197-260）；defs/use/渐变/clipPath/text/preserveAspectRatio 直接 skip（doc.go:46-49、parser.go:214），url(...) paint 回退黑。注：render 主包渐变家族（Linear/Radial/Sweep）与裁剪栈已有，SVG 侧是解析映射缺口非底层能力缺口 |
| SVG 表彩色字形 / COLRv1 完整 | §5.2 | 见 1.3 COLRv1 行（v0 误读 v1 需修正） |
| 嵌套 Picture 引用 | §5.2：缺 | **成立**。recording 命令全集无 DrawRecording/嵌套引用（command.go:52-124）；`Recording.Playback(backend)` 单层回放（recorder.go:141、backend_canvas.go:25）；ui 侧 PictureLayer 持 Recording+CacheKey（ui/scene/layer.go:292） |

**结论**：P3.8 成立；SVG filter 语义依赖 P3.4（依赖列漏记，见 §2）；SVG text/tspan 逐段样式与 P3.3 同源。

## 2. 依赖图草案（P3.0 核心产出）

### 2.1 图

```
P3.0（本文档） ──┬→ P3.1 色彩管理地基（线性混合接线）【前移项，HDR 地基】
                 ├→ P3.5 路径与几何（conic/四角RRect/Matrix4/PathEffect/布尔保曲线）
                 └→ P3.6 图片格式 + GPU mipmap
P3.1 ──┬→ P3.3 富文本逐 run 样式【前移项】   ← 依赖存疑，建议解除（D1）
       ├→ P3.4 滤镜家族                      ← 建议降为软依赖（D1）
       └→ P3.7 2.5D（HDR/泛光、材质、透视接线）
P3.2 自定义 shader（P2.1 ✅ + P3.0）──→ P3.7（自定义材质）【台账漏记，建议补（D1）】
P3.4 ──→ P3.8 SVG 完整语义（<filter>）【台账漏记，建议补（D1）】
P3.3 ──→ P3.8（SVG text/tspan 逐段样式、COLRv1 完整层）
P3.5 ──→ P3.7（Matrix4 透视）／→ P3.8（维持现记）
P3.6 ──→ P3.7（GPU mipmap，软依赖，消费关系）
```

### 2.2 对台账 §2 依赖列的修订建议（拍板项 D1）

| # | 台账现记 | 建议 | 依据 |
|---|---|---|---|
| 1 | P3.3 依赖 P3.0、**P3.1** | 解除 P3.1 | 逐 run 颜色/装饰样式不经过混合空间；linear.go/色彩 API 与富文本无技术交集（§1.1/§1.3 实测） |
| 2 | P3.4 依赖 P3.0、**P3.1** | P3.1 降为软依赖（推荐先做 P3.1） | offset/erode/tile/merge 等几何滤镜不依赖色彩空间；blend/lighting 族滤镜可在线性空间受益，但属可选增强 |
| 3 | P3.7 依赖 P3.0、P3.1、P3.5 | **补 P3.2**（硬） | 自定义材质依赖自定义 shader，PLAN §5.3 明写；台账漏记 |
| 4 | P3.8 依赖 P3.0、P3.3、P3.5 | **补 P3.4**（硬） | SVG 完整语义含 `<filter>`（PLAN §5.1 明列 feXxx 语义需滤镜家族） |
| 5 | P3.7 无 P3.6 | 补 P3.6（软，消费关系） | GPU mipmap 由 P3.6 交付、2.5D 消费（PLAN §5.3 末行） |

> 若用户对 1/2 有既定理由（如富文本颜色要做线性混合渲染、滤镜家族整体按色彩空间重做），维持现记并在依赖图上注明理由——拍板时说明即可。

## 3. 实现顺序草案（拍板项 D2）

**推荐主序（串行）**：

```
P3.1 → P3.2 → P3.3 → P3.4 → P3.5 → P3.6 → P3.7 → P3.8
```

理由：
1. P3.1 第一：前移项 + HDR/滤镜/2.5D 三家的共同地基；
2. P3.2 第二：hal 口现成、只依赖 P2.1（✅），独立性强可早期见效；
3. P3.3 第三：前移项，且 P3.8（SVG text/tspan）依赖它；
4. P3.4 第四：P3.8 依赖它；
5. P3.5/P3.6：技术上最独立（只依赖 P3.0），放在中后是因为它们不被 P3.1–P3.4 依赖、不阻塞任何人；若要提前可整体前移；
6. P3.7/P3.8 收尾：消费方最多（P3.7 吃 P3.1+P3.2+P3.5[+P3.6]，P3.8 吃 P3.3+P3.4+P3.5）。

**可并行说明**：台账 §2 并行段写「P3.2–P3.8 在 P3.0（及各自前置）完成后可分批并行」。技术上 P3.5 与 P3.6 可与 P3.2/P3.3 并行（互不依赖）；但每项都要独立分支+全门禁+双后端真窗（隔离 :3 串行排期），GPU 真窗排期是实际瓶颈，**推荐串行**，并行留给用户明确要求时。

**前移项硬约束核对**：P3.1 在第 1 位、P3.3 在第 3 位，均未落入 P4——满足「不得挪回 P4」（台账 §3）。

## 4. 文档更新方案（拍板项 D3/D4）

### 4.1 PLAN §5 更新（D4，推荐「加现状列+修差异行」）

- §5.1/§5.2/§5.3 三张表各加一列「代码现状（2026-11-01 实测）」，内容取本评估 §1 的 file:line；
- 口径修正行：布尔运算（已有 Clipper 版、缺保留曲线）、路径效果（已有 dash+2 个 1D 子集）、ImageFilter/ColorFilter（已有 blur/colormatrix/dropshadow 子面）、2.5D（引擎库已有 Projector/YSort25D，缺 render 层透视口）；
- §6 半成品表同步刷三行：线性混合（0 生产调用现状）、DrawParagraph（行号 265-285→276，附 :270-272 自述）、COLRv1（「只取基础层」→「实际只解析 v0 表、v1 字体被当 v0 误读，比原登记更糟」）；
- §7.4 回写：依赖图结论 + 实现顺序定稿（按 D1/D2 拍板结果）。

### 4.2 RENDER_API_CATALOG.md 更新（D3，推荐「新增规划登记节」）

- 新增一节（建议 §13）「P3 计划新增公开 API（规划登记）」：按 P3.1–P3.8 逐项列计划 API 面（草稿级、标「规划」状态），每项实现时把规划行转正式登记并跑 apidoc；
- 现有各节状态刷新仅限本次盘点涉及的描述（如 §7.2 render/hdr 行维持、COLRv1 相关描述随 P3.8 实现再改）——**P3.0 不改现有 API 的状态标注**（它们没变）；
- 改完跑 `go run ./scripts/apidoc`（当前 rc=0，改完必须仍是 0；apidoc 只机检符号覆盖，新增规划节预期无冲突，落盘留档）。

### 4.3 台账更新（按 §6 同步规则，P3.0 开工即做）

- §2.1 当前焦点切 P3.0 + 载体登记（见 §5）；§2 表 P3.0 ⬜→🟡；§7 流水追加；
- 依赖列修订（§2.2 表）在拍板后随文档更新同一次提交。

## 5. P3.0 验证载体（台账 §4.2 模板）

```
- 任务：P3.0 依赖图 + 文档更新（PLAN §5 + RENDER_API_CATALOG.md + 实现顺序）
- 能力单测：不适用（纯文档任务，零代码变更；替代自校验=①apidoc rc=0 复跑留档
  ②PLAN §5 现状列每条 file:line 附可复现 grep/读点清单 ③依赖图与台账 §2 依赖列一致性核对）
- 真窗：不适用（不开窗、不占 :3）
- 金图：不适用
- 双后端：不适用（无行为变更）
- 负向对照：不适用（文档任务无门禁可红；替代=apidoc 机器校验对 catalog 的一致性）
- 证据：evidence/P3.0/（assessment.md、dependency-graph.md、apidoc-p30.txt）
```

## 6. 工作区与非本线事项

- 未跟踪改动 4 处，均非本线，**只报告不动**：`.atomcode/artifacts/`、`.qoder/`、`.zcode/`、`examples/ui_wr_c5_anim_over_static/golden/c5_final.png`。
- 台账在册尾巴（I3/I4/C4/T2、ui 5 既有红、gles DeviceLost/X11KeyRepeat、misc_draw wgpu 执行态、SeatCapCB、B1–C5/E1/E3/F1/F2/G3/I1 等待办池项）：均不阻塞 P3.0（文档任务），依赖图中作不阻塞注记；I1（ladder Wayland 呈现卡死）与 P3.7 真窗排期有关，P3.7 开工时知悉。
- 分支 `feat/p2-收敛整合`，HEAD `da9168a5`，47 个本地提交未推送（用户要求推送前确认，本任务不涉及）。

## 7. P3.0 完成标准草案（拍板后按此执行）

1. `evidence/P3.0/dependency-graph.md` 落盘：节点/边/每条边的依据（file:line）/修订对照；
2. PLAN §5 三表加现状列 + 差异行修正；§6 三行刷新；§7.4 回写顺序与依赖定稿；
3. `RENDER_API_CATALOG.md` 新增 P3 规划登记节（按 D3 拍板口径）；
4. 台账 §2 依赖列按 D1 拍板修订 + §2.1/§7 同步；
5. `go run ./scripts/apidoc` rc=0 复跑留档 `evidence/P3.0/apidoc-p30.txt`；
6. 以上全部为文档提交，零代码变更；构建/vet/逐文件门禁不受影响（无代码），提交前照常 `git status` 对照只提交本线文件。
