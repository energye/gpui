# render/gpu 收敛方案（2026-10-28）

> 依据：用户 7 条口径——①重复的删一套、保留标准实现；②能合并的以标准实现为基础合并；③死代码按「任意图形 / UI 组件库 / 2.5D 引擎」三根支柱的支撑需求删除；④双后端 webgpu 与 gwgpu 在各平台实现对应功能；⑤缺失能力先落文档再实现；⑥半成品先落文档再实现；⑦最终包组织结构更规范。
>
> 性质：决策与排期文档，本文件不含代码改动。证据沿用 01–06 与 2026-10-28 只读复核。

## 0 「标准」的判定口径

三条同时满足才算标准：**对齐上游（naga / Skia / wgpu）设计**、**当前生产在用**、**双后端可实现且可验证**。

## 1 标准选型结论

### 1.1 shader 翻译器

| 项 | `gpu/gwgpu/naga`（11.6 万行） | `gpu/shader`（23.9 万行） |
|---|---|---|
| 后端 | WGSL → IR → GLSL / MSL | WGSL → IR → GLSL / MSL / SPIR-V / HLSL / DXIL |
| 在用情况 | gles 与 metal 的唯一翻译器 | 仅 `render/internal/gpu/shader_helper.go:19`，且只走测试调用链 |
| 与对方的差异 | `lower.go` 比对方**多 598 行、少 13 行**；`parser.go` 多 40/少 12；`validate.go` 多 51/少 6；msl codegen 多 205/少 31 | 旧版；glsl codegen 24/24 持平 |
| 公开 API | `glsl.Compile` / `msl.Compile` 与对方**逐行一致**（连行号都对得上） | 同左 |

**结论：naga 是标准实现，保留；`gpu/shader` 的旧副本删除。** 依据：naga 是净增的新版且在用；`gpu/shader` 的三个额外后端（SPIR-V/HLSL/DXIL）对应的是 Vulkan/D3D12，不在「webgpu + gwgpu」双后端目标内，属于按第③条应删的死代码。

**合并动作**：把 naga 的实现迁到顶层 `gpu/shader`（保持 `gpu/shader/{ir,wgsl,glsl,msl}` 结构），删除旧副本与 `spirv/`、`hlsl/`、`dxil/`、`internal/backend`、`internal/dxcvalidator`。公开 API 不变，调用方只换 import。

迁移面：非测试 8 个文件（`gpu/gwgpu/gles/{capabilities,device,device_linux,resource,shader}.go`、`gpu/gwgpu/metal/{device,resource}.go`、`gpu/gwgpu/metal/mslmap/mslmap.go`）+ 7 个测试文件。`ir/capabilities.go` 需随迁（naga 独有）。

**风险**：Metal 无法在本机（Linux）验证。执行顺序先在 GL/Linux 真窗验 GLSL 路径，Metal 按跨平台标记纪律标「待验证」，不得标绿。

### 1.2 渲染主路

**标准 = `render.Context` + `render/gpu`（SDFAccelerator + AdaptiveFiller）。**

非标准副本：

- `render/render`（18 文件 3968 行）：doc.go 仍写 “gg / gogpu.App”，Scene/Renderer/Target/Layers 与现架构重叠，零生产消费。
- `render/internal/gpu` 老一代场景渲染器（约 6000 行闭合簇）：`backend.go`、`renderer.go`、`pipeline.go`、`pipeline_cache_core.go`、`memory.go`、`shaders.go`、`sparse_strips_gpu.go`、`gpu_coarse.go`、`gpu_fine.go`、`gpu_flatten.go`。仅 `_test.go` 调用；`backend.go` 自行 `webgpu.CreateInstance` 建第二个设备，违反「创建入口唯一」。

### 1.3 其它领域

| 领域 | 标准 | 非标准 |
|---|---|---|
| 设备创建 / 选卡 | `adapter_policy.go` 定策略 + `present_target.go` 唯一建链 | `internal/gpu/device.go` 镜像、`present_target.go` 第三份重试循环 |
| 混合枚举 | `render.BlendMode` | `blend.Mode`、`image.BlendMode` |
| 呈现决策 | `frame.PlanFramePresent` | `PresentWith(Auto)`、`Context.PresentFrame(Damage)` 各自决策 |
| 资源记账 | `hal.Vram*`（原生 + GL 已接线）+ `res/cache` | `MemoryManager` 第二套预算 |
| 延迟释放 | `res/registry` 一套 | `pendingTex/Buf`、`ImageCache.pending`、`res.Cache.pending` 三套 |
| 裁剪栈 | render clipStack | `scene.ClipState/Stack` 第三套 |

## 2 删除 / 合并清单

### 2.1 整套删除（重复实现）

| 编号 | 删除对象 | 保留 | 批次 |
|---|---|---|---|
| A1 | `gpu/shader` 旧 ir/wgsl/glsl/msl 副本 + `spirv/` `hlsl/` `dxil/` `internal/backend` `internal/dxcvalidator` | naga 实现（迁到 `gpu/shader`） | B2 |
| A2 | `render/render` 15 个旧文件（device/layers/target/renderer/software/scene/gpu_renderer 及旧测试与 testdata） | 现架构；`float_target.go` 迁出 | B1 |
| A3 | `render/internal/gpu` 老一代渲染器 10 个文件（见 §1.2） | 主路径；`segment.go`/`flatten.go`/`fine.go`/`coarse.go`/`sparse_strips.go`/`tilecompute_filler.go` 等活文件保留 | B3 |
| A4 | `pipeline_cache_core.go`（随 A3） | 主路径 `ensurePipelines` | B3 |

### 2.2 合并（同职责多入口）

| 编号 | 合并项 | 做法 | 批次 |
|---|---|---|---|
| B1 | 建设备 / 选卡三份 | `internal/gpu/device.go` 改为委托 `adapter_policy.go`；删 `present_target.go` 独立重试循环 | B4 |
| B2 | 裁剪第三套 | `scene.ClipState/Stack` 改为调 render clipStack | B4 |
| B3 | 混合枚举三套 | 统一到 `render.BlendMode`，另两处改别名/边界转换 | B4 |
| B4 | 呈现三套 | `frame.PlanFramePresent` 唯一决策，另两处委托 | B4 |
| B5 | 已废弃别名（纯色/渐变/形状） | 按「接口全量切换制」一次改完调用方后删除 | B1 |
| B6 | 预算六口径 | 统一 Budget 类型，按域分账；删除 `MemoryManager` 预算逻辑 | B4 |
| B7 | 延迟释放三队列 | 统一到 `res/registry` | B4 |
| B8 | 同尺寸纹理六处分配 | 收敛到 `res/cache` | B4 |

### 2.3 单点死代码

| 项 | 处理 | 批次 |
|---|---|---|
| `TexturePool.Acquire/Release/EndFrame`（半废：分配不用、销毁仍在） | 删分配路，保留或改掉销毁路 | B4 |
| `offscreenPool`（复用分支禁用） | 恢复复用或删池 | B4 |
| `render/gpu.SetDeviceProvider/AbandonDevice`（生产零调） | 接进 embedder 或标实验 | B4 |
| `gpu/shader/tmp/*.dxil`（gitignore 本地产物） | 删本地目录 | B1 |
| `render/internal/gpu/doc.go` 老一代说明 | 随 A3 重写 | B3 |

## 3 包组织目标结构

```
gpu/
  hal/           接口（唯一抽象）+ noop/（无 GPU 后端，测试缝与降级用）
  types/         类型与常量
  context/       设备桥
  webgpu/        native 后端门面（Windows/Linux/macOS/Web；internal/{browser,thread} 组成）
  rwgpu/         wgpu-native 的 purego FFI 绑定层（包名 wgpu；webgpu 门面之下，Web 路径不经）
  gwgpu/
    gles/        GL/EGL/WGL（egl/ gl/ wgl/ 组成）
    metal/       Metal（mslmap/ 组成）
    indirect/    间接绘制跨度算术（metal 专用）
    ffishim/     ffi 垫片（purego C-ABI）
  shader/        唯一 shader 翻译器：ir / wgsl / glsl / msl（各带 internal 实现子包）
render/
  *.go           门面：Context / Canvas / Path / Paint / Text / Image ...
  gpu/           GPU 注册包（!nogpu/nogpu 双形态）
  filters/       CPU 图像滤镜注册
  integration/   ggcanvas 等对接桥
  scene/         保留场景
  recording/     录制回放（含 backends/raster）
  text/          排版与字形（cache/ emoji/ hint/ msdf/ 组成）
  svg/           SVG 图标子集
  hdr/           FloatTarget + 色调映射（从 render/render 迁出）
  internal/
    gpu/         主路径：SDF、AdaptiveFiller、session、texture、text、mask、filter
                 + res/（资源生命周期，P2.4c）+ tilecompute/（Vello CPU 移植）
    image/ color/ blend/ filter/ raster/ cache/ parallel/
    clip/ stroke/ wide/            通用图形件
    conformance/ testutil/         测试设施（P0.4 测试台 / imagediff）
```

三条硬规则：后端目录只放后端实现；通用图形能力在 `render`；不存在 `render/render` 这种同名子包。

> 修订（2026-10-31 P2.5 go list 对账，证据 `evidence/P2.5/golist-compare.md`）：目标树补列 `gpu/rwgpu` 层、`ffishim` 实名、`render/{gpu,filters,integration}`、internal 补列 `clip/stroke/wide/conformance/testutil` 与 `gpu/{res,tilecompute}`；逐包登记理由见对比文档。已登记豁免：`gpu/rwgpu/examples`、`gpu/webgpu/examples` 示例混居后端目录（迁出为可选后续小步）。

## 4 双后端 × 平台能力矩阵

平台覆盖：Windows = webgpu + gles；Linux = webgpu + gles；macOS = webgpu + metal；Web = webgpu。

| 能力 | webgpu | gles | metal | 处理 |
|---|---|---|---|---|
| RenderBundle | ✅ | ❌ `hal_assert_shared.go:18` | ❌ `hal_assert.go:18` | 真源显式标「不支持」并保留明确错误；不阻塞 |
| QuerySet | ✅ | ✅ | ❌ `hal_assert.go:18` | Metal 实现或显式标不支持 |
| AccelerationStructure | ✅ | ❌、`:19` | ✅ `hal_assert.go:42` | GL 显式标不支持 |
| 损伤呈现 | ❌ 忽略（`surface.go:390-392`） | EGL/X11 ✅；WGL ❌（`queue.go:217-218`） | — | 标「仅 X11」 |
| sRGB 交换链 | ❌ 优先非 sRGB（`swapchain_configure.go:153-159`） | 未见 | 未见 | 三端都按 gamma 混合，列入缺口 |
| mipmap 链 | ❌ 全后端纹理单级（实测非测试代码 `MipLevelCount: 1` **84 处 / 28 文件**；创建透传在 `gpu/rwgpu/texture.go:264-268,288-297`，原文引用的 `gpu_texture.go` 文件名不存在）；**生成侧 webgpu 绑定面零入口**（无 `GenerateMipmap` 符号，仅采样器 mip 档 + `wgpuTextureGetMipLevelCount`），gles `glGenerateMipmap` 已加载零调用 | ❌ 同左（gles `device_linux.go:284-294,314-325` 已真支持多级纹理，只缺产链） | 逻辑已备 `gpu/gwgpu/metal/device.go:236-241`，darwin 交叉本机不可验 | P3.6 缺口，路线拍板 D3=CPU 统一产链 + hal 逐层上传（`evidence/P3.6/assessment.md` §5） |
| 自定义 shader | ❌ 无 API | ❌ | ❌ | 三端都缺，列入缺口 |

## 5 缺失能力登记（先文档，后实现）

### 5.1 任意图形（2D 矢量）

> 现状列＝P3.0 依赖图定稿实测（2026-11-01，HEAD `da9168a5`，逐条 file:line 详证 `docs/render_gpu_converge/evidence/P3.0/assessment.md` §1）。

| 能力 | 归属层 | 双后端要求 | 代码现状（2026-11-01 实测） | 验收 |
|---|---|---|---|---|
| 自定义 / 运行时着色器 | render（新 API） | 两后端各一条路径 | 公开层零 shader API；hal `CreateShaderModule(WGSL)` 现成（`gpu/hal/api.go:235`）；内部 25 个 WGSL go:embed 直喂 hal（`render/internal/gpu/shader_helper.go:15`）；翻译器 `gpu/shader` 仅 gles/metal 后端在用 | 逻辑探针 + 像素断言 + 金图 |
| 色彩管理：HDR / 广色域 / ICC | render/hdr + gpu | 两后端交换链与混合 | 公开层零 ColorSpace/sRGB API；主路 GPU/CPU 都在 sRGB 空间混合（`blend_gpu.go:55`、`software_blend.go:32`）；`render/hdr` FloatTarget 纯 CPU 零消费者；双后端 Float RT 格式能力已备未用（`gles/capabilities.go:800-810`）；sRGB 交换链三端缺（§4） | HDR 真窗 + 金图 |
| MaskFilter | render | 两后端 | 零命中；现有 mask 能力服务掩码合成与字形/裁剪采样，无 Skia 式 MaskFilter 概念 | 真窗 + 金图 |
| ColorFilter 家族（blend/lighting/compose/table） | render | 两后端 | 无 ColorFilter 抽象；ColorMatrix(4x5) 已有双链路+9 预设（`internal/filter/colormatrix.go:30-177`）；缺四族与 LUT | 真窗 + 金图 |
| ImageFilter 家族（erode/dilate/lighting/offset/tile/merge/displacement/alpha-threshold） | render | 两后端 | 已有 blur/blurXY/dropshadow 双链路（`internal/filter/blur.go:24,42`、`shadow.go:23`）+ Context 方法族接入；缺的正是本行 8 族；Paint 无 ImageFilter 字段 | 真窗 + 金图 |
| conic 路径段 | render/path | 与后端无关 | verb 全集无 Conic（`path_verb.go:35-44`）；Arc 入口级逼近（`path_build.go:179`） | 单测 + 金图 |
| 四角独立圆角 | render/path | 与后端无关 | `ClipRoundRect` 单半径（`context_clip.go:86`）；`internal/clip.RRectClip` uniform（`stack.go:22-26`） | 单测 + 金图 |
| 4×4 透视矩阵 | render/matrix | 与后端无关 | `Matrix{A..F}` 纯 2x3 仿射（`matrix.go:25`）；CPU 栅格 2D（`raster/edge_builder.go:49-56`） | 单测 + 金图 |
| mesh gradient（**2026-11-02 拍板 D2：单列 P3.9，不并入 P3.6**；**2026-11-09 标准落地修正**：Skia `SkCanvas::drawPatch` 是单 patch Coons 原语，`SkPatchUtils::MakeVertices`→`drawVertices` 先细分再走彩色顶点网格，**复用现有 mesh 管线、不需要新 WGSL**；PDF type 7 TensorProduct 为 bicubic 家族） | render | 两后端 | 全仓零命中确认缺（实测严格搜仅命中 7 处文档；既有 `render.Mesh` 实际在 `render/vertices_atlas.go:739`，是逐顶点色三角形 Gouraud，不是 Skia patch，不算已有实现） | 真窗 + 金图 |
| GPU mipmap 链 | render/gpu | 两后端 | **2026-11-02 开工复核修正**（原登记「`image_cache.go:362,377` 等 15 处」低计，实测 84 处/28 文件，其中**该开多级的只有 `image_cache.go:362,377` 一处**，余为 MSAA/depth/mask/SDF/字形图集应保持单级）；**接口面其实已齐**——hal `descriptor.go:199-224` 有 `MipLevelCount`、blit `command.go:73-296` 带 `MipLevel`、采样器契约 `image_pipeline.go:1110-1143/1408-1436` 已能把 `MipmapLinear→MipmapFilterModeLinear`、WGSL 绑定形 `image_pipeline.go:846-860` 已可 mipmap；缺的是生产者：`SetPerImageFilter`（`:1096`）**全仓零生产调用者**、公开 `DrawImageOptions.UseMipmaps`（`render/context_image.go:160-162`）在 GPU 路被整段忽略（`context_image_gpu.go:47-133` 不读）——公开 API 正在承诺不存在的能力；真链只有 CPU（`internal/image/mipmap.go:25-96`，且唯一消费者 `draw.go:235-258` 从不 `Release`）；原登记行号 `image_pipeline.go:950-985` 已漂到 `destroyPipeline` | 真窗 + 金图 |
| 图片解码 GIF/APNG/动图 WebP/AVIF/HEIF/EXIF（**2026-11-02 拍板 D1：AVIF/HEIF 剔除本项，改登记为远期专项**） | render/internal/image | 与后端无关 | 仅 PNG/JPEG/WebP 静态（实测行号 `internal/image/io.go:25,72-138`，原登记 `20-25,102-132` 已漂）；`ImageBuf` **无任何帧/时长/重复/dispose-blend 字段**（`buf.go:53-82`），动图/EXIF 零支持；**仓里有两套并行解码注册表**（render 侧 `io.go` + `ui/io/decode.go:62-133,651-687`），加格式必先收敛单一真源（D5）；**动图 WebP 纯 Go 可行**（`x/image` 已导出 `vp8`/`vp8l`/`riff` 公开包，静态解码器 `webp/decode.go:76-99,146` 本就这么调，动图只多一层 `ANIM/ANMF` 容器） | 测试数据入 testdata |
| 路径效果 1D/2D/Sum/Compose | render/path | 与后端无关 | dash 完整（`dash.go:18/40/69`）；`WithCorners`/`Discrete` 两个 1D 子集已有（`pathops_subpath.go:391,493`）；无 PathEffect 抽象与 2D/Sum/Compose | 单测 + 金图 |
| 布尔运算保留曲线 | render/path | 与后端无关 | 布尔已有（Clipper 扫描线，`path_boolean.go:22-98`）但曲线 flatten 0.1px 折线输出；缺「保留曲线」版 | 单测 + 金图 |
| SVG 完整语义（defs/use/渐变/clipPath/text/filter） | render/svg | 与后端无关 | 现支持 8 元素基础属性（`svg/parser.go:197-260`）；defs/use/渐变/clipPath/text skip（`svg/doc.go:46-49`）；render 渐变/裁剪底座已有，缺解析映射 | 单测 + 真窗 |

### 5.2 UI 组件库

> 现状列＝P3.0 依赖图定稿实测（2026-11-01，详证 `evidence/P3.0/assessment.md` §1.3/§1.8）。

| 能力 | 归属层 | 代码现状（2026-11-01 实测） | 验收 |
|---|---|---|---|
| render 层逐段富文本样式（颜色/装饰） | render/text | 无 Span/RunStyle/Paragraph/ParagraphStyle 类型；`ShapedRun` 是 shaping 产物（script/direction/face 切分）非样式容器；DrawParagraph 单色自述 `context_misc_draw.go:270-276` | 真窗 + 金图 |
| 两端对齐 Justify | render/text | 常量在（`layout.go:30`）实现缺（`applyAlignment:617` 直接 return 退化左对齐） | 单测 + 金图 |
| ellipsis / maxLines 下沉到 render | render/text | 零命中；现有只有 MaxWidth+WrapMode（`layout.go:50-79`） | 单测 + 金图 |
| SVG 表彩色字形 / COLRv1 完整 | render/text | COLR 实际只解析 v0 表、v1 字体被当 v0 误读（`colr.go:197,219` 无 Paint/ClipList；CPAL v1 按基础色提取 `:248`）——比 §6 原登记「只取基础层」更糟；CBDT+COLRv0 生产链在（`color_font_adapter.go:18-34`→`ui/rendering/text.go:1152`）；sbix 解析器零消费者 | 真窗 + 金图 |
| 可复用嵌套 Picture 引用 | render/recording | 无 DrawRecording/嵌套引用命令（`recording/command.go:52-124`）；`Playback` 单层（`backend_canvas.go:25`） | 单测 + 真窗 |

### 5.3 2.5D 引擎

> 现状列＝P3.0 依赖图定稿实测（2026-11-01，详证 `evidence/P3.0/assessment.md` §1.7）。

| 能力 | 归属层 | 代码现状（2026-11-01 实测） | 验收 |
|---|---|---|---|
| 透视相机与投影矩阵 | render/matrix + 模块库 | 引擎层 Projector 已有（`engine/camera/project.go:22` DepthToScale/Project/ProjectQuad/Unproject）；缺 render/matrix 4×4 透视（`matrix.go:25` 仿射）与 render 层透视绘制口 | 真窗三证据 |
| 世界高度 / 精灵 z 序排序 | 2.5D 模块库 | 引擎层 YSort25D/Sprite.DepthSort 在用（`ysort25d.go:60`）；render 层 `DepthSprite/SortDepthSprites/DrawDepthSprites` 拍平批绘已有（`depth_r5.go:43,99,132`） | 真窗三证据 |
| 自定义材质 | 依赖自定义 shader | 等待 P3.2（依赖图定稿：P3.7 硬依赖 P3.2，台账已补记） | 真窗三证据 |
| HDR / 泛光 | 依赖 render/hdr | 等待 P3.1 地基（线性混合 + hdr GPU 面） | 真窗三证据 |
| GPU mipmap | 依赖 render/gpu | 由 P3.6 交付、2.5D 消费（依赖图定稿：软依赖） | 真窗三证据 |

## 6 半成品登记（先文档，后实现）

| 能力 | 现状 | 位置 | 修法 | 验收 |
|---|---|---|---|---|
| 线性空间混合 | helper 写好未接线（2026-11-01 复核：`GetBlendFuncLinear/BlendLinear` 全仓 0 生产调用，仅 linear_test.go；GPU/CPU 主路均在 sRGB 空间混合，`blend_gpu.go:55`、`software_blend.go:32`） | `render/internal/blend/linear.go` | 合并为带 linear 参数的单一入口并接主路 | 单测 + 金图 |
| MSAA>1 even-odd 裁剪 | 退化跳过 | `internal/gpu/depth_clip.go:596-618` | 补 sampleCount>1 路径 | 真窗 + 金图 |
| 非整数裁剪矩形 | 整数 scissor 无 AA | `internal/clip/stack.go:421` | 走路径裁剪或分数 scissor | 金图 |
| 顶点网格 paint alpha/blend | 忽略，GPU 硬边 | `vertices_mesh.go:18-95`、`gpu_render_context.go:1058-1183` | 透传 blend/alpha，补 AA 或声明 | 真窗 + 金图 |
| DrawShadow | 无环境光/透视剪影 | `context_misc_draw.go:184-195` | 按 Skia 模型补 | 金图 |
| GPU/CPU 模糊一致 | 两套实现靠容差 | — | 对齐容差或统一算法 | 金图逐位 |
| mask 弹层临时纹理 | 每次 2–3 张 | `mask_r8_modulate.go:305-350` | 池化复用 | 预算/帧时 |
| backdrop 全表面快照 | RT→RT 全量 + 全量合成 | `m4_quadimage.go:251-301` | 裁剪到损伤区 | 预算/帧时 |
| 抖动整图 + 回读 | 全图抖动 | `m4_dither.go:33-90` | 仅输出路径、避免回读 | 帧时 |
| 覆盖率在 CPU | Vello compute 未接主路 | `render/gpu/gpu.go:36-56` | 作为独立性能项目重做（注入设备） | 真窗 + 帧时 |
| COLRv1 基础层 | **口径修正（2026-11-01 复核）**：实际只解析 v0 表，v1 字体被当 v0 误读——`parseBaseGlyphs`(:197)/`parseLayers`(:219) 无 Paint/ClipList/VarIndex 解析，CPAL v1 按基础色提取(:248)；比原记「只取基础层」更糟 | `text/emoji/colr.go:177-250` | 补 paint graph | 真窗 + 金图 |
| DrawParagraph 单色 | 单 paint（2026-11-01 复核：`:276`，`:270-272` 注释自述 per-run 颜色/样式是 render 层未有的 ParagraphBuilder concern） | `context_misc_draw.go:276` | 逐 run 样式 | 真窗 + 金图 |

## 7 任务分解、可行性与门禁（2026-10-28 修订）

### 7.0 可行性判断

**总体可行**，但有三个前提：基线可复现、P2.4 必须拆步、跨平台运行时边界必须如实标注。

最大不确定性：P2.4 的资源生命周期收敛（释放时序、预算淘汰语义）；Metal 与 Windows 运行时无法在本机验证。

原 7 项任务方向正确，但缺 4 类步骤：**治理准备（P0）**、**双后端一致性测试台**、**P2.4 拆步**、**前置半成品前移**。本次一并补入。

### 7.1 P0 准备（不改代码，新增）

| 步骤 | 内容 | 产出 |
|---|---|---|
| P0.1 | 验收阈值与环境清单：首帧、滚动帧率、p95/p99、显存上限、金图容差写死数字并指明真源 | `baseline/thresholds.md` |
| P0.2 | 真窗与金图清单：窗口名 → 金图 → 平台 → 容差 → 单窗/批量 | `baseline/windows.md` |
| P0.3 | 归档与回滚演练：打 tag/建分支，实际回退一次证明可回 | 回滚记录 |
| P0.4 | 双后端一致性测试台：同一场景在 webgpu/gles/metal 跑逻辑断言（不比像素，只比语义） | 一致性用例集 |

### 7.2 P1 基线冻结（不改代码）

目的：把“当前所有测试和真窗都正确”变成可对照的证据，作为 P2 的唯一参照。

- P1.1 六目标交叉编译：`linux/amd64`、`linux/arm64`、`windows/amd64`、`darwin/amd64`、`darwin/arm64`、`js/wasm`；另跑 `-tags nogpu`。（2026-10-28 更新：js/wasm 因 `purego` 无 wasm 实现，登记为不支持，基线按五真机目标验收，见台账 P1.1b。）
- P1.2 `go vet ./...`。
- P1.3 逐测试文件运行，记录每文件 PASS/SKIP/FAIL；SKIP 必须带原因。
- P1.4 全部真窗：JSON 探针 + PNG + 金图 diff 计数 + 容差声明。
- P1.5 每包公开 API 快照（`go doc -all`）+ 测试计数快照。
- P1.6 开窗/关窗/压测后的显存账本快照。
- P1.7 首帧、滚动帧率、p95/p99 性能基线。
- P1.8 负向对照：人为改 1px / 少画一次 / 漏释放一次，确认门禁会红。

### 7.3 P2 框架代码调整与整合（对应第 1–4 条）

| 子步 | 内容 | 对应 |
|---|---|---|
| P2.1 | shader 合并：naga 实现迁到顶层 `gpu/shader`（统一包名），删旧副本与 `spirv/` `hlsl/` `dxil/` `internal/backend` `internal/dxcvalidator`；迁移前先对 GLSL/MSL 输出做全量快照，迁移后逐字节对比 | 第 2、3 条 |
| P2.2 | 删 `render/render` 旧原型；`float_target.go` 迁到 `render/hdr` 并保留/迁移其单测与 testdata | 第 2、3 条 |
| P2.3 | 删 `render/internal/gpu` 老一代渲染器 10 个文件及其测试；`doc.go` 重写 | 第 2、3、4 条 |
| P2.4a | 设备创建 / 选卡收敛（三份 → 一份） | 第 1、3 条 |
| P2.4b | 预算统一（六口径 → 一套 Budget，按域分账） | 第 1、3 条 |
| P2.4c | 延迟释放统一（三队列 → `res/registry`） | 第 1、3 条 |
| P2.4d | 同尺寸纹理分配收敛（六处 → `res/cache`） | 第 1、3 条 |
| P2.4e | 裁剪 / 混合枚举 / 呈现入口收敛 | 第 1、3 条 |
| P2.5 | 包组织收口：`go list` 对比、分层约束测试、索引与总账同步 | 第 1 条 |
| P2.6 | 行为零变化总验收：P1 基线全量重放 | 第 1–4 条 |

**每个子步的门禁**：全平台编译 + `go vet` + 逐测试文件 + 残留 grep + API 快照 diff（只允许 §7.7 白名单内的删除）。

**P2.4 每条子步的附加门禁**：生命周期测试台（开窗/关窗/切场景 → 显存账本归零、堆无增长）+ `-race` + 真窗三证据。一次只动一类，出问题整步回退。

**阶段总门禁**：所有单测与真窗结果与 P1 基线一致；金图逐位差为 0（非确定性路径按 P0.1 声明容差）；API/测试计数差异逐条对上 §7.7 白名单。

**回退**：每个子步独立分支与提交点。

### 7.4 P3 缺失能力（对应第 5 条）

- P3.0 依赖图 + 文档更新（本文 §5 + `RENDER_API_CATALOG.md`），再实现。
- P3.1 色彩管理地基（含线性混合接线——原属半成品，**前移**，否则 HDR 无地基）。
- P3.2 自定义 shader（依赖 P2.1 的翻译器 + 新公开 API）。
- P3.3 富文本（含 `DrawParagraph` 逐 run 样式——原属半成品，**前移**，否则 UI 组件库缺基础）。
- P3.4 MaskFilter / ColorFilter / ImageFilter 家族。
- P3.5 路径与几何：conic、四角圆角、Matrix4 透视、路径效果、布尔保曲线。
- P3.6 图片格式（GIF/APNG/动图 WebP/EXIF 方向）+ GPU mipmap。**2026-11-02 开工拍板修订范围**：AVIF/HEIF 剔除本项（Flutter 本树未编进、Go 生态无可用 AV1 熵解码器，登记为远期专项，容器解析+能力探测为副产品）；mesh gradient 不并入本项（见 P3.9）。
- P3.7 2.5D 透视相机、世界高度/精灵 z 序、自定义材质、HDR/泛光。
- P3.8 SVG 完整语义、彩色字形、嵌套 Picture 引用。
- **P3.9（2026-11-02 拍板 D2 新增）** mesh gradient：Skia 口径 2×2/3×3 patch 网格（角角色 + bicubic/coons 风格），与 P3.6 零技术交集，从 §5.1 单列成本项；建议排在 P3.8 之后，依赖 P3.0。**2026-11-09 标准落地修正**：Coons=Skia `drawPatch` 12 控制点（`include/core/SkCanvas.h:2154`），TensorProduct/bicubic=PDF shading type 7 的 16 控制点（`src/pdf/SkPDFGradientShader.cpp:755-762`）；栅格落地复用 `SkPatchUtils::MakeVertices`→`drawVertices` 的彩色 mesh，本仓对应 `DrawMesh`，**无新 WGSL 管线**。
- **P3.9a（2026-11-09 新增，阶段 1）** mesh gradient 高层网格：rows×columns 个 patch、(rows+1)×(columns+1) 顶点、每顶点位置/颜色/可选四切向量、自动 C1 推断（逐字对齐 AndroidX Compose `MeshGradientConfig.inferBezierControlPointsIfRequired`）、Bilinear/CatmullRom 颜色模型；高层默认 Coons 12 点，TensorProduct 16 点继续用低层 `MeshPatch`；一次绘制合成一个 indexed `Mesh`。**不并入本项**：纹理坐标/图片变形（阶段 3 按需另立通用 textured mesh）、paint alpha/blend（归 P4.1 接线）、AA/颜色精度/合批/缓存（阶段 4 先验证/基准，不先写代码）、kit/PDF/SVG/HDR/Spine renderer。依赖 P3.9。
- **P3.9b（2026-11-09 新增，测量补充）** 1x/4x MSAA 真实窗口动画性能基准：为回答「4x 占多少显存/掉多少帧」补一份有效对照。本机 940MX 仅 1024MiB，字面 4K(3840×2160)+4x 会 OOM 降级回 1x 测不到；按拍板跑满屏 1920×1080（真 GPU 有效数据），代码 `GPUI_P39B_W/H` 可配并留 3840×2160 档给大显存机器、本机不跑。新建 `examples/ui_wr_p39b_msaa_anim`：满窗混合通用动画（文字/圆角/曲线/渐变/图片/半透明层/旋转缩放/粒子 + mesh gradient 网格一起每帧动），MSAA 可配（开窗前 `SetMSAASampleCount`），三模式（冻结相位门禁 / 动画测量 / 常驻人工窗）；1x/4x × native/go 四轮串行测显存/首帧/帧时/CPU。**零引擎改动、零新增公开 API、catalog 不改**。依赖 P3.9a。 **2026-11-09 测量收口 ✅**：三有效组合金图 0.0000%/logic 6/6、18s 测量有效；native 1x→4x 显存峰值 2.35×/fps −24.9%/p95 破线；**抓到并落位 go/GL+4x 整窗全黑既有引擎洞**（用户拍板不修/登记不可验/不标绿），证据 `evidence/P3.9b/`。

**P3.0 定稿（2026-11-01，用户拍板 D1–D4）**：依赖图正本在 `docs/render_gpu_converge/evidence/P3.0/dependency-graph.md`（现状证据 `assessment.md`）。依赖修订 5 条——P3.3 解除对 P3.1 的依赖、P3.4 对 P3.1 降为软依赖、P3.7 补硬依赖 P3.2 与软依赖 P3.6、P3.8 补硬依赖 P3.4（台账 §2 依赖列已同步）。实现顺序定稿（串行主序）：**P3.1 → P3.2 → P3.3 → P3.4 → P3.5 → P3.6 → P3.7 → P3.8**；P3.1/P3.3 前移项位置满足「不得挪回 P4」。§5 三表现状列、§6 半成品表三行已按 2026-11-01 实测刷新；P3 各项计划公开 API 的规划登记在 `RENDER_API_CATALOG.md` §13，各项实现时转正式登记。

每项：逻辑探针 + 像素断言 + 金图三证据；双后端分别验；不支持的端显式标注。

### 7.5 P4 半成品完整实现（对应第 6 条）

在 P3 完成后启动，只处理非前置项：

- P4.1 正确性：MSAA>1 even-odd 裁剪、分数裁剪 AA、顶点网格 alpha/blend、DrawShadow。
- P4.2 一致性：GPU/CPU 模糊对齐。
- P4.3 性能：mask 临时纹理池化、backdrop 损伤区合成、抖动路径优化。
- P4.4 文本：COLRv1 完整层（若 P3 未覆盖）。

说明：P4.1 的 MSAA/分数裁剪属正确性缺陷，若在 P3 的真窗里被阻塞，允许前移到 P3 对应项，并在 §7.7 记录调整。

### 7.6 任务同步规则

- **可执行任务清单与状态台账在 `docs/ENGINE_RENDER_GPU_CONVERGE_TASKS.md`，该文件是任务状态的唯一真源**；外部任务列表是它的镜像。本文 §7 只定方案口径与门禁，不再重复维护状态。
- 每步完成：更新 TASKS 台账状态 + 证据落盘（JSON/PNG/diff 计数）+ 若口径有变回写本文。
- 计划内删除的 API 与测试登记在 §7.7；基线对账只允许白名单内差异。
- 新会话开工顺序：`AGENTS.md` → `docs/render_gpu_converge/00_README.md` → `docs/ENGINE_RENDER_GPU_CONVERGE_PLAN.md` → `docs/ENGINE_RENDER_GPU_CONVERGE_TASKS.md`，按台账取第一个未阻塞任务。

### 7.7 计划内删除登记（基线对账白名单）

| 对象 | 删除内容 |
|---|---|
| `gpu/shader` | 旧 ir/wgsl/glsl/msl 副本；spirv/hlsl/dxil/internal |
| `gpu/gwgpu/naga` | 目录整体（实现迁到 `gpu/shader`） |
| `render/render` | 15 个旧文件及其测试/testdata；FloatTarget 迁出 |
| `render/internal/gpu` | backend/renderer/pipeline/pipeline_cache_core/memory/shaders/sparse_strips_gpu/gpu_coarse/gpu_fine/gpu_flatten 及其测试 |
| 别名 | 已废弃的纯色/渐变/形状旧接口 |

### 7.8 本次补充项（原计划不完整之处）

1. **P0 治理**：没有阈值、环境清单、金图清单、回滚演练，基线无法复现。
2. **双后端一致性测试台**：现有测试按后端各写各的，缺“同语义跨后端”验证，无法证明 webgpu 与 gwgpu 对等功能。
3. **P2.4 拆步**：设备/预算/延迟释放/纹理分配/入口收敛混在一起不可控，必须一次一类。
4. **前置半成品前移**：线性混合是 HDR 地基、逐 run 样式是富文本地基；按“6 在 5 之后”会返工。
5. **CI / 矩阵自动化**：六目标编译与真窗需要可重复执行；没有 CI 就要有固定 runner 协议。
6. **测试计数对账**：删除会减少测试数，必须区分“计划内减少”和“误删覆盖”。
7. **构建标签回归**：`nogpu`、`js/wasm`、`windows`、`darwin` 必须逐个编译，不能只跑默认标签。（js/wasm 当前登记为不支持：`purego` 无 wasm 实现，见台账 P1.1b；其余标签照跑。）
8. **文档同步持续门禁**：API 同步不能只在 P2.5 做一次，每次公开 API 改动都要同步总账。

### 7.9 每任务能力验证（硬，2026-10-28 补）

每个任务都必须有**对应的能力单测 + 对应的真窗**，两类缺一即未完成：

- **开工前登记**：能力单测文件与测试名、真窗路径与运行方式、金图与容差数值、双后端覆盖，写进台账 §2.1；四项没登记不得开工。
- **删除/合并/重构（P2）**：动手前先补齐能力单测并跑绿；没有测试的能力不许重构；删除的能力用同一组断言证明等价；真窗按基线逐位重放。
- **新能力（P3/P4）**：先写会红的能力单测，再实现；真窗必须新增或扩展并出新金图；没有真窗的能力不得标 ✅。
- **完成证据**：单测逐文件 PASS、真窗 JSON 探针、PNG、金图 diff 计数、双后端结果、一个针对本能力的负向对照（必须红过再还原）。
- **正确的定义**：单测绿 + 三证据齐 + diff 在容差内 + 双后端一致（或差异有书面依据）+ 负向对照红过；缺一项只能标 🟡。
- **唯一例外**：零生产消费者的死代码删除，确实无真窗可跑；须给全仓无引用 grep、迁移前后单测一致、P2.6 基线重放无差异三样替代证据，并在台账 §7 流水写明。

操作细则（登记模板、按任务类型的落地方式）见 `docs/ENGINE_RENDER_GPU_CONVERGE_TASKS.md` §4.1/§4.2。

## 8 已确认事项（2026-10-28）

1. **shader**：naga 实现是标准（净增新版、在用），保留并迁到顶层 `gpu/shader`；`gpu/shader` 旧副本与 SPIR-V/HLSL/DXIL 删除（不在 webgpu+gwgpu 双后端目标内）。
2. **老一代 GPU 渲染器**：按冗余实现删除；GPU 覆盖率列为未来性能项，不在本轮保留。
3. **`render/render`**：删旧原型；`FloatTarget` 迁到 `render/hdr`。
4. **包组织**：按 §3 目标树收口；跨平台与双后端按 §4 矩阵逐格标注。
5. **顺序**：P1 → P2（1–4）→ P3（5）→ P4（6）。

## 9 尚未具备的验证条件（诚实标注）

- Windows / macOS 运行时：本机只有 Linux，只能交叉编译；这两端运行时标「待验证」。
- Metal 真窗：同 Linux 限制，标「待验证」。
- 跨平台金图：文字类需按平台分开基线，不能共用。
