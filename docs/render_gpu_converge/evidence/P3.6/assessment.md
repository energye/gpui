# P3.6 图片格式 + GPU mipmap 影响面评估（2026-11-02）

直话：P3.6 分两半，难度与风险完全不对称。**解码半边**（GIF/APNG/动图 WebP/EXIF）纯 Go 可行、上游有可对齐的标准语义，但要先解决「仓里有两套并行解码注册表」这个重复实现问题；**mipmap 半边**接口面其实已经齐（hal 字段、采样器、WGSL 绑定形、公开 `UseMipmaps` 都在），缺的是「没人生产」+「纹理只有一级」+「webgpu 后端没有 GPU 生成入口」三件事，真正的工作量在确定生成路线与把 4/3 显存账补齐。AVIF/HEIF 经实测在本机 Go 生态无解（无可用的 AV1 熵解码器，且参考框架 Flutter 本树根本没编进这两种格式），属 PLAN 承诺与环境现实的冲突，必须拍板处置。

- 范围来源：PLAN §7.4「P3.6 图片格式（GIF/APNG/动图 WebP/AVIF/HEIF/EXIF）+ GPU mipmap」`docs/ENGINE_RENDER_GPU_CONVERGE_PLAN.md:262`；§5.1 两行缺口 `:152`（GPU mipmap 链）`:153`（图片解码）；台账 §2.2 行 `docs/ENGINE_RENDER_GPU_CONVERGE_TASKS.md:296`；依赖 `P3.0` ✅；catalog §13 规划行 `docs/RENDER_API_CATALOG.md:895`；依赖图节点 `docs/render_gpu_converge/evidence/P3.0/dependency-graph.md:34`。
- 取证方式：三路只读调查（解码链现状 / GPU mipmap 现状 / 上游标准对齐）+ 本机直接复核；基线 HEAD `b37c21ed`。
- 本文只盘点与给推荐，正式文档与代码在 D1–D6 拍板后落地。

## 1 关键现状（全部实测 file:line）

### 1.1 解码入口与格式面
- render 公开口只有 4 个：`LoadImage(path)` `render/context_image_buf.go:111`、`LoadWebP(path)` `:117`、`NewImageBuf(w,h,format)` `:123`、`ImageBufFromImage(image.Image)` `:129`；`type ImageBuf = intImage.ImageBuf` `render/context_image.go:22`。
- 转发链全在 `render/internal/image/io.go`：`LoadImage:72` → `LoadPNG:38`/`LoadJPEG:49`/`LoadWebP:60` → `DecodePNG:114`/`DecodeJPEG:123`/`DecodeWebP:132`；兜底 `Decode:102` →  stdlib `image.Decode:103` → `FromStdImage:197`。
- 格式识别＝**扩展名优先** `io.go:73-81`（.png/.jpg/.jpeg/.webp），未知扩展才落魔数嗅探 `io.go:89`，且嗅探返回的 format 字符串被丢弃 `io.go:108`（`_ = format`）。不可识别时**报错不静默**（`io.go:105`）。
- PLAN `:153` 写的「仅 PNG/JPEG/WebP 静态」**仍是事实**，但行号已漂移：`io.go:20-25` 现为 `:25` 的 `golang.org/x/image/webp` 导入行，`102-132` 实为 `102-138`。
- 缺：GIF、APNG、动图 WebP、AVIF、HEIF、BMP、TIFF 全无；EXIF 全无（见 1.3）。
- 死码：`ErrUnsupportedFormat` `io.go:31` 全仓零返回点（只 `ErrEmptyData:96` 有用）。

### 1.2 ImageBuf 数据模型 = 单帧，且「帧推进」有缓存硬约束
- `render/internal/image/buf.go:53-82`：`data:62` `width/height:63-64` `stride:65` `format:66` `genID:67` `gpuDirty:72` `disposed:76` + 懒预乘缓存 `:79-81`。**没有任何帧/时长/重复/dispose-blend 字段**；全仓 `dispose` 命中全部指 `Dispose()`（`buf.go:91`，释放 CPU backing），与 GIF 的 dispose 语义无关。
- 代际口：`GenerationID():260`、换代 `NotifyPixelsChanged():272`、一次性 `MarkEphemeral():284`（gen=0）、保代+脏 `MarkPixelsDirty():296`、消费清旗 `TakeGPUDirty():309`。
- 缓存键＝代际：`image_cache.go:62` `map[uint64]*imageCacheEntry`，`GetOrUpload:221`/`key=cmd.GenerationID:226`；`key==0` 上传后立即 `retireEntry:229-236`；命中且 `ContentDirty:243-247` 原地重写；尺寸不符则 `removeEntry:251-253` 重建。批处理合并谓词同样以代际为键 `image_pipeline.go:1160`（声明 `:72`）。
- **结论（决定动图接口形态）**：若每帧调 `NotifyPixelsChanged`，帧推进会**每帧新建纹理条目**（显存单调增长，正是 `render/context_image_buf.go:84-85` 注释警告的场景）。正确姿势只有两条：复用同一 `ImageBuf` + `MarkPixelsDirty`（原地重写），或每帧 `MarkEphemeral`（一次性上传不入缓存）。
- 引擎层唯一既有的帧时序模型：`engine/sprite/flipbook.go:47-50`（`Frames []int` + `FrameDur` + `Loop`，输入是**已解码**图像 id，与解码器解耦）。

### 1.3 EXIF / 方向 / 色彩元数据 = 零
- 全仓 `exif` Go 侧 0 命中；`orientation` 命中全属 tilemap `engine/tilemap/tilemap.go:23` 与字体轮廓方向 `render/text/autohint.go:941`；`dpi`/`density`/`icc`/`gAMA`/`cHRM`/`iCCP` 均 0 命中。
- PNG 走 `png.Decode` `io.go:115`（stdlib 不暴露 chunk 元数据）；JPEG 走 `jpeg.Decode` `:124`（APP1 直接忽略）。
- WebP：模块 `x/image@v0.44.0/webp/decode.go:164-168` 定义了 `animationBit`/`exifMetadataBit`/`iccProfileBit` 三个常量但**从未使用**——上游也没解析 ANIM/ANMF/EXIF/ICC。
- P3.1 交付的是 `BlendSpace` 线性混合，**没有**吃 ICC/gAMA（`render/context_image.go:168-171`；`evidence/P3.1/assessment.md:76-78`），sRGB 目标格式整体推到 P3.1b（同文 `:66`）。→ P3.6 若读 iCCP/gAMA，要与 P3.1b 的边界拍清，不在本批偷跑色彩管理。

### 1.4 仓里有两套并行解码注册表（重复实现，触发 AGENTS「收敛只动实现」）
- render 侧：`render/internal/image/io.go`（扩展名表 + stdlib 注册表）。
- ui 侧另起一套：`ui/io/decode.go:62` `Decoder{Decode,Config}` 契约、`:75` `RegisterDecoder`、`:85` `SupportedDecoders`、`:106-117` 扩展名映射、`:121-133` 按序 `Config` 嗅探、`:651-687` init 注册 png/jpeg/webp；公开口 `DecodeFile:474`、`DecodeBytes:479`、`…WithContext:484/489`、`Default:172`、异步池 `Pool:155`。
- 后果：给 P3.6 加格式若只加一边，两套支持表立刻分叉（一边能开 GIF 一边不能）。这是「同一能力的重复副本」，按纪律应收敛成单一真源。

### 1.5 消费者现状（判断「做出来谁用」）
- 组件库：`ui/kit/internal/prim/content.go:316-319` → `rendering.NewRenderImage` `ui/rendering/image.go:47`，装图口 `SetImage:71`/`SetImageShared:105`。
- 真窗/示例走 **ui/io**：`examples/ui_wr_c3_list_scroll/main.go:278`、`ui_wr_r10_async_image/main.go:244`、`ui_wr_c10_soak/main.go:133`、`ui_wr_t_stress/main.go:448`、`ui_render_base_image/scene.go:254`。
- render 侧唯一程序化消费：`render/internal/conformance/run.go:78`。视频线不解码：`examples/video/video_vr3_color/main.go:108-126` 只 `SetImageShared` 合成图。彩色字形 CBDT 自研表解析，与 image 解码无关 `render/text/color_font_adapter.go:28-29`。

### 1.6 GPU mipmap：接口面早已齐，缺的是生产者
- hal 纹理描述**已有** mip 字段：`gpu/hal/descriptor.go:199-224`（`MipLevelCount:207`、`SampleCount:210`、`Format:216`、`Usage:219`、`ViewFormats:223`）；view `:227-253`（`BaseMipLevel:243`、`MipLevelCount:246`，0=全层）；blit 拷贝**已带 `MipLevel`**：`gpu/hal/command.go:73-74`、`:248-252`、`:296-297`。`gpu/hal/resource.go:57-77` Texture 接口无 mip 查询口。
- 后端透传现状：webgpu `gpu/webgpu/device_create.go:70-86`（`:79` 透传）→ CGO/purego `gpu/rwgpu/texture.go:264-268`（0→1）、`:288-297` wire、`:314,:342` 记账；浏览器 `gpu/webgpu/device_browser.go:87`。gles **真支持多级**：`gpu/gwgpu/gles/device_linux.go:204-348`（`:284-294` 逐层 `TexImage2D`；`:314-325` 设 `TEXTURE_MAX_LEVEL=MipLevelCount-1`，注释 `:316-319` 明说单级不设会 texture-incomplete 采出全零），Win 孪生 `device.go:193-307`；上传按 `dst.MipLevel` `queue_linux.go:201`/`queue.go:191`。metal 在仓内且逻辑已备 `gpu/gwgpu/metal/device.go:236-241`（`setMipmapLevelCount`）、`:334` view、`:417`+`conv.go:205-211`（`setMipFilter`）——**仅 darwin，本机不可验**。
- **PLAN `:132`/`:152` 的两处引用有误**：`gpu_texture.go` 这个文件名不存在（实名 `gpu/rwgpu/texture.go:254,270`→实测 `:264-268`）；「等 15 处」严重低计——实测非测试代码 `MipLevelCount: 1` **84 处 / 28 文件**（`grep -rn "MipLevelCount:\s*1" --include=*.go . | grep -v _test.go | wc -l`）。
- 其中**真正该开多级的只有图片缓存一处**：`render/internal/gpu/image_cache.go:362,377`。其余按性质保持单级，禁全域改动：滤镜中间纹理 `filter_gpu_graph.go:493,505,620,629,699,708`；RT/离屏 `gpu_render_context.go:2621,2635,3308,3322,3961,3975,4357,4372`；MSAA/depth/resolve `surface_cache.go:322,337,354,376`；mask 系 `mask_r8_modulate.go:216,224,262,271`、`linear_ramp_mask.go:399,409,435,445,474,484`、`pattern_mask_sample.go:333,343,372,382,410,420`；SDF `sdf_render.go:244,260,272,289`；字形/图集 `glyph_mask_engine.go:958,974`、`glyph_color_engine.go:223,239`；视频 `video_texpool.go:72,86`、`video_planepool.go:144`。
- **采样器面也已齐**：`gpu/hal/descriptor.go:256-289` 有 `MagFilter:270`/`MinFilter:273`/`MipmapFilter:276`/`LodMinClamp:279`/`LodMaxClamp:282`/`Anisotropy:288`（无独立 mipLevelBias）。render 侧契约纯函数 `image_pipeline.go:1110-1143` `SamplerDescriptorForImageFilter` 已把 `MipmapLinear→MipmapFilterModeLinear`（`:1123-1127`），惰性实例化 `SamplerForFilter:1408-1436`（`:1419` 只有 mip/aniso 键才新建）。PLAN `:152` 引用的「`image_pipeline.go:950-985` 采样器已备空转」行号已漂到别的函数（`:926-1005` 现为 `destroyPipeline`；真采样器在 `:809-820`/`:822-831`）。
- **「空转」的真正双因**（这才是要修的点）：① `ImageDrawCommand.FilterMipmapLinear` `image_pipeline.go:118` 的唯一写手 `SetPerImageFilter:1096` **全仓零生产调用者**（仅 `engine/tex/doc.go:61` 与本文件注释提及）；② 纹理只有一级。公开开关 `render/context_image.go:160-162` `DrawImageOptions.UseMipmaps` 在 GPU 路**被忽略**——`render/context_image_gpu.go:47-133` 通篇不读它，只读 Interpolation/BlendMode（`:100-101`），仅 CPU 兜底透传 `render/context_image_draw.go:157`，且 `:129` 注释自认「UseMipmaps uses GPU bilinear (approx」。**即公开 API 现在是在承诺一个不存在的能力**。
- WGSL：图片类全部 `texture_2d<f32>` + 独立 filtering sampler，绑定形已可 mipmap（`image_pipeline.go:846-860`，`SamplerBindingType=Filtering:859`）——**WGSL 类型不用改**。自动 LOD 天然生效的口：`textured_quad.wgsl:36,96`、`textured_quad_persp.wgsl:38,98`、`video_yuv.wgsl:40-41,98-99`、`glyph_mask.wgsl:29,79`、`glyph_color.wgsl:28,63`。钉死 level0、要吃 mip 必须显式改 LOD 的口：`textured_quad_bicubic.wgsl:36,126`、`cover_textured_linear.wgsl:19,36,69,177`、`cover_textured_pattern.wgsl:15,32,65,122`、`sdf_render.wgsl:68,111`。WGSL 份数实测 26（`render/internal/gpu/shaders/` 16 + `tilecompute/shaders/` 10），PLAN `:143` 写 25 已漂移。
- 跨后端一份 WGSL 的既有约定：单份 `go:embed`（`image_pipeline.go:26,29,32…`）直喂 `hal.CreateShaderModule`（`gpu/hal/api.go:236`）；webgpu 原样用，gles 建管线时 naga 翻译（`device_linux.go:523,1062` → `compileWGSLToGLSL` `shader.go:127-139,168`；映射 `gpu/shader/glsl/internal/codegen/expressions.go:1127-1146`：`textureSample→texture()`、`SampleLevelExact→textureLod:1132`；msl 侧 `:2754-2776`）。

### 1.7 mipmap 生成入口：webgpu 后端没有，gles 有但没人调（本机实测）
- wgpu-native 绑定面**不存在任何 mipmap 生成入口**：`gpu/rwgpu/` 里 mip 相关只有采样器枚举（`sampler.go:28,52,82,118,132`、`gputypes_aliases.go:71`）与只读查询 `wgpuTextureGetMipLevelCount`，纹理级符号仅 `wgpuTextureCreateView/Destroy/GetWidth/GetHeight/GetFormat/GetUsage/GetMipLevelCount/GetSampleCount/GetDepthOrArrayLayers/GetTextureBindingViewDimension/Release`（本机实测符号清单）。
- gles 有 `glGenerateMipmap` 且已加载但**零生产调用**：`gpu/gwgpu/gles/gl/context.go:98,268,675-676`、`gl/context_linux.go:489,688,1315-1317`。
- 推论（重要）：想「双后端金图逐位一致」，生成必须在**同一处产链**——各后端各自 GPU 生成会让 webgpu 必须自写逐级降采样管线（wgpu core 无 hook），gles 用 `glGenerateMipmap`，两条链的滤波核不同，逐位对齐不成立。统一 CPU 产链（已有实现 `render/internal/image/mipmap.go:25-96`）+ 逐层上传（hal blit 已带 `MipLevel`，`gpu/hal/command.go:73-74`）是唯一能让双端出同一份字节的路。

### 1.8 CPU 侧真链与两处既有洞
- `render/internal/image/mipmap.go:25` `MipmapChain`、`:36-58` `GenerateMipmaps`（层数 `:43`）、`:62-96` `downsample` 2×2 盒式（**非线性空间**，逐像素 `GetRGBA`）、`:100` `Level`、`:109` `NumLevels`、`:128-149` `LevelForScale`、`:156-168` `Release`。
- 唯一消费者 `render/internal/image/draw.go:235-258`：每次 `scale<0.99` **现场生成整链、只取一级、且从不调 `Release`** → 池归还丢失（可登记为 P3.6 副修）。
- `engine/tex/mipmap.go` 另有独立 mip 数学（`MipMode:60-91`、`Filter:104-117`、`SamplerParams:202-207`、`NumLevels:215-234`）+ 数据契约 `engine/tex/testdata/mipmap_cases.json`，**只有测试与示例消费**（`examples/engine/tex/main.go:359,378`），未接 render 生产；`image_pipeline.go:1109` 注释把它写成 `game/tex/...`（漂移）。

### 1.9 显存预算：开 mip 后 render 侧会低估 4/3
- 条目/字节预算 `image_cache.go:29-34` ← `render/budget.go:78`（128 条 / 64 MiB / 公平地板 16 / 8 MiB）；进程 768 MB `gpu/hal/vram_ledger.go:38-58`（`GPUI_VRAM_BUDGET_MB`）；淘汰 `evictIfNeeded:455-463`、`evictOldest:466-483`、`PurgeEvictable:340-349`、`SetBudgets:163-175`。
- hal 账本**已经**按 mips 乘 4/3：`gpu/hal/vram_ledger.go:81-91`；后端记账点 `gpu/rwgpu/texture.go:314,342`、`gles/device_linux.go:208,335`（Win `device.go:200,307`）、退账含 mipLevels `gles/resource.go:149-174`。
- **洞**：render 侧 `image_cache.go:392` `need = w*h*4` 与 `:426-428` 自计 `usedBytes` **只算 level0** → 开 mip 后 64 MiB 门低估 4/3，render 自计与 hal 真账分叉，实际显存先到 hal 768 MB 上限而 render 以为没满。

### 1.10 现有钉与必改测试（语义一转就红的那两处）
- 无任何 GPU mip 链测试、无 mipmap 真窗证据（`docs/render_gpu_converge/evidence/**` 里 mipmap 只出现在 P3.0 台账/依赖图与 api-diff 快照）。
- `render/r1_residual_gpu_test.go:106-145` 是**反向钉**：断言 `UseMipmaps` 走 GPU 且 `CPUFallbackOps==0`（`:141,:144`）——它现在钉的是「GPU 忽略该开关所以不落 CPU」这个假语义，语义转正后必改。
- `render/quad_cpu_test.go:303-379` 是 GPU/CPU 图片缩放唯一像素钉，`:373` 的「changed<1%」门与 mip 预滤波直接冲突（预滤波必然改变大量像素值）→ 需按新语义重新声明容差。
- `render/internal/image/mipmap_test.go:8-220` 纯 CPU 链；`gpu/rwgpu/command_new_test.go:149,174,202` 是全仓唯一 `MipLevelCount:3` 的用法（未接 render）；`gpu/hal/noop/*` 需随 hal 任何接口新增同步。

### 1.11 mesh gradient：真源口径自相矛盾
- PLAN §5.1 `:151` 把它单列一行「mesh gradient | render | 两后端 | 全仓零命中确认缺 | 真窗+金图」，但 §7.4 `:256-264` 的 P3.1–P3.8 八行**没有任何一行包含它**；依赖图 `:34` 的 P3.6 节点范围却写着「§5.1 六种格式+EXIF+**mesh gradient**」；台账 §2.2 P3.6 行 `:296` 与 catalog §13 `:895` 都没写它。
- 代码侧「零命中」仍成立：严格搜 `mesh ?gradient|gradient ?mesh|渐变网格|PatchVertex|GrMesh` 只命中 7 处文档（PLAN `:151`、TASKS `:128`、`evidence/P3.0/assessment.md:10,82`、`dependency-graph.md:34,78`、`baseline/windows.md:322`）。既有 `render.Mesh`（`render/vertices_atlas.go:475-486`，Positions/Colors/Indices + `DrawMesh/DrawMeshEx`，`render/canvas.go:147-149`）是**逐顶点色三角形（Gouraud）**，不是 Skia 的 2×2/3×3 patch，不能当已有实现。
- 若并入 P3.6 需新增：patch 类型（12/16 控制点 + corner role + style 枚举 bicubic/coons）、绘制口、`recording` 命令、新 WGSL 管线（连带第 15 份 `rrect_clip_coverage` 内联副本）、CPU 孪生、正式 API 登记、独立真窗——**与图片解码/mipmap 无任何技术交集**。

## 2 上游标准对齐依据（本地 flutter 树实证；Skia 源码本机未 checkout）
> `flutter/engine/src/flutter/third_party/skia` 本机不存在（DEPS `flutter/engine/DEPS:474-475` 声明、`skia_revision` `:19`，目录仅剩 BUILD.gn + 空 modules），故 Skia 枚举原文标「需回上游核对」，以下取 Flutter 的**真实调用点**=标准在参考框架里的落地方式。

**A 解码**
- 格式识别＝**注册表轮询**，不是 switch：`lib/ui/painting/image_generator_registry.cc:30-42`（依次注册 PNG/JPEG/WebP/GIF/BMP/WBMP/ICO，注释明说保持 Skia 原顺序），入口 `SkCodec::MakeFromData`（`image_generator.cc:106,193`）。
- 帧元数据字段（`lib/ui/painting/image_generator.h:35-52` `FrameInfo`）：`required_frame: optional<uint>`、`duration`（ms，0=只显示一帧）、`disposal_method`、`disposal_rect`、`blend_mode`。
- repetition：`image_generator.cc:118-121`——`getRepetitionCount()<0 → kInfinitePlayCount`，否则 `+1`（`image_generator.h:30-31`）。
- 帧依赖：新版用 `FrameInfo::fRequiredFrame`（`:123-133`，`SkCodec::kNoFrame → nullopt`），并把上一帧作 `Options::fPriorFrame` 回传（`:148-152`）；`getRequiredFrame()` 已废弃、本地零调用。
- **准确枚举名**（逐值映射见 `image_generator_apng.cc:446-473`）：Blend = `kSrc`/`kSrcOver`；Disposal = `kKeep`/`kRestoreBGColor`/`kRestorePrevious`（**没有 `kDiscard`、没有 `kNoBlend`**，那是旧名）；单帧默认 `kKeep`（`image_generator.cc:60`）。
- APNG：Flutter **不用 Skia 的 APNG**，自研 `lib/ui/painting/image_generator_apng.{h,cc}`——`h:73-74` fcTL/fdAT 常量、`h:99-113` `acTL.num_frames` 与 fcTL 的 `x/y_offset,width,height,delay_num/den,dispose_op,blend_op`、`cc:499-500` **fdAT 剥 4 字节序号改写成 IDAT** 后 `cc:576-580` 复用单帧 PNG 解码、`cc:476-480` delay 分母为 0 按 100、`:313` num_plays；硬纪律 `cc:626`「解码前必须先调一次 getInfo」。→ 我方 APNG 有可直接照抄的容器层设计先例。
- EXIF orientation：`image_generator.cc:86-95` `getInfoIncludingExif` + `SkEncodedOriginSwapsWidthHeight(codec->getOrigin())` **交换宽高**；`:144-189` 在**解码期**应用（解到 temp buffer 再 `SkPixmapUtils::Orient`），不是绘制期矩阵；回归测试 `image_decoder_unittests.cc:951`。`SkEncodedOrigin` 12 值枚举原文需回上游 `include/codec/SkEncodedOrigin.h` 核对。
- 高层：Flutter 主链路不走 `SkAnimatedImage`（只有 skwasm 用：`skwasm/animated_image.cc:34-56,66-67`），主链路是 `ImageGenerator` + `MultiFrameCodec`（`multi_frame_codec.cc:78-115` required 帧缺失降级 blank slate、`:120-137` keep/restorePrevious 缓存策略、`:143-147` `kRestoreBGColor` 用 `erase(TRANSPARENT, rect)`、`:258 nextFrameIndex_=(i+1)%frameCount_`、`:260-266` 把 (image, duration) 回投 UI 线程）；Dart 面 `lib/ui/painting.dart:2347-2372 abstract class Codec{frameCount, repetitionCount, getNextFrame, dispose}` + `:2319 FrameInfo`；**播放节奏在 framework 不在引擎**（`packages/flutter/lib/src/painting/image_stream.dart:1041-1101`）。
- AVIF/HEIF：Flutter 本树**完全没有**（`grep -ri "heif|avif|SkHeif|aom|dav1d"` 覆盖 lib/ui、display_list、impeller 仅命中一张 README 图片 URL），`RegisterSkiaCodecs` 无此两项 → 「对齐 Skia/Flutter」不构成实现 AVIF/HEIF 的依据。GIF 走 Skia vendored gif（无外部库），动图 WebP 依赖 libwebp（DEPS:486-487）。

**B mipmap**
- 层数公式 `impeller/geometry/size.h:137-143` `MipCount = min(log2(w),log2(h))`，下限 1。
- 生成侧：`lib/ui/painting/image_decoder_impeller.cc:463` `mip_count = size.MipCount()`；`:465-469` Metal+需 resize 时降为 1；`:501-503,528-530` **仅 `mip_count>1` 才** `blit_pass->GenerateMipmap`；`:637` **压缩纹理强制 1 级**（色彩格式限制）；`:481-503` 上传后**同一 command buffer 内立即生成**；幂等守卫 `impeller/core/texture.cc:71-73` `NeedsMipmapGeneration() = !mipmap_generated_ && mip_count>1`；接口 `renderer/blit_pass.cc:170-179` + 虚口 `blit_pass.h:143,182`（各后端各实现一次）。
- 采样侧四层枚举（gpui 公开采样面对齐目标）：`display_list/dl_sampling_options.h:17-23` `DlImageSampling{kNearestNeighbor, kLinear, kMipmapLinear, kCubic}`；换算 `skia/dl_sk_conversions.h:63-75`（`kMipmapLinear→SkSamplingOptions(kLinear, SkMipmapMode::kLinear)`、`kCubic→SkCubicResampler{1/3,1/3}`）；`impeller/display_list/skia_conversions.cc:26-46`（`kNearest/kLinear→mip=kBase`、`kCubic/kMipmapLinear→kLinear`）；`impeller/core/formats.h:602-615 MipFilter{kBase,kNearest,kLinear}`；字段 `impeller/core/sampler_descriptor.h:15-17 {min_filter,mag_filter,mip_filter}`。
- **「生成 ≠ 消费」**：解码内部一律显式 `SkMipmapMode::kNone`（`image_decoder_skia.cc:59`、`image_decoder_impeller.cc:246`）——生成了链也不代表采样去吃它，消费由采样档决定。这条要写进 gpui 语义，否则 `UseMipmaps` 与采样档会再次分叉。
- 预算：`hasMipmaps()? 4/3 : 1` 两处一致——`lib/ui/painting/display_list_image_gpu.cc:53-54`、`display_list/image/dl_image_skia.cc:66`（`computeMinByteSize()*kMipmapOverhead`）；逐层精确求和 `impeller/core/texture_descriptor.h:63-75 GetByteSizeOfAllMipLevels`。→ 印证 1.9 的 render 侧低估是真洞。
- 上游**未发现**「尺寸阈值/仅被 GPU 采样才生成」这类工程豁免（仅压缩格式与 resize 路径两条豁免）。

## 3 本机实测对上游结论的两处修正
1. **动图 WebP 在纯 Go 生态是可行的**，「必须依赖 libwebp」的判断不成立：`x/image@v0.44.0` 已随依赖在盘上导出 `vp8`（`vp8.NewDecoder()` / `Init(io.Reader,int)` / `DecodeFrameHeader()` / `DecodeFrame() *image.YCbCr`，实测 `vp8/decode.go:150,155,160,364`）、`vp8l`（`Decode`/`DecodeConfig`，`vp8l/vp8l.go:543,556`）、`riff`（chunk 遍历）三个公开包；静态 WebP 解码器本就是这么调的（`webp/decode.go:76-99` `fccVP8` 把 chunk 数据**原样**喂 `vp8.Decoder`，`:146` `vp8l.Decode`，`:58,222` `fccALPH`+`readAlpha`+`unfilterAlpha`）。动图只是外面多一层 RIFF `ANIM`/`ANMF` 容器与帧合成，逐帧 payload 仍是同一个 VP8/VP8L/ALPH chunk。→ 剩余风险从「无解码器」降为「容器解析 + 帧合成语义正确性」，与 APNG 同一量级。**待核对项**：ANMF 内 VP8 chunk 是否需按 WebP 规范做尺寸/裁剪对齐（`x/image/vp8` 的 `FrameHeader` 含 `HScale`，需按规范核对后再定偏移）。
2. **单级纹理创建点是 84 处/28 文件，不是 PLAN 写的 15 处**；且 PLAN `:132` 引用的文件名 `gpu_texture.go` 不存在（实名 `gpu/rwgpu/texture.go`）。影响：影响面清单必须按「只有 `image_cache.go:362,377` 该开多级」来定，不能被「15 处」暗示的全域改法带走——那会同时动到 MSAA/depth/mask/SDF/字形图集，是纯风险无收益。

## 4 顺带发现的洞（本批处置归属待拍板，不口头略过）
- `image_cache.go:392,426-428` 预算只算 level0 → 开 mip 低估 4/3（hal 侧已乘，两边分叉）。属 P3.6 直接影响面，建议随批 2 同修。
- `render/internal/image/draw.go:235-258` 现场产链从不 `Release` → 池归还丢失。属 P3.6 影响面，建议随批 2 同修。
- `render/internal/image/io.go:31` `ErrUnsupportedFormat` 零使用死码。批 1 接线时激活（嗅探失败统一返回它）或删除，二选一，不留悬空。
- `render/r1_residual_gpu_test.go:106-145` 现钉的是「GPU 忽略 UseMipmaps」的假语义 → 批 2 语义转正必须同步改钉（负向对照用同一条）。
- PLAN 四处文档漂移：`:132`（文件名 + 15 处）、`:143`（WGSL 25→26 份）、`:152`（`image_pipeline.go:950-985` 行号漂移）、`:153`（`io.go:20-25,102-132` 漂移）；另有 §7.4 未覆盖 §5.1 的 mesh gradient 行（1.11）。→ 建议随 P3.6 开工登记一次刷新，并在 §7.4 明写 AVIF/HEIF 的处置结论（D1）。
- 非本线：`go test ./render` 单进程整包在 `Context.Close→wgpuDeviceCreatePipelineLayout` 确定性 native abort（P3.5 批 4 已登记，归 `docs/ENGINE_GPU_RESOURCE_LIFECYCLE.md`）；P3.1 遗留 raster 坏包 6 枚。

## 5 拍板项（2026-11-02 用户确认：**D1–D4 全按推荐**；D5/D6 按本文推荐形态执行）
**拍板结果**：**D1** 批 1 做 GIF+APNG+动图 WebP+EXIF 四项，**AVIF/HEIF 从 P3.6 剔除并同步修订 PLAN §7.4/台账 §2.2/catalog §13**，登记为远期专项（容器解析+能力探测+哨兵错作剔除的免费副产品，不占验收）；**D2** mesh gradient **剔出 P3.6 单列 P3.9**，PLAN §7.4 补行、依赖图节点范围同步修正；**D3** GPU mipmap **CPU 统一产链 + hal 逐层上传**（`glGenerateMipmap` 留作后续性能优化项登记，本批不混做；盒式核维持 sRGB 现状，滤波空间归 P3.1b）；**D4** 公开采样面**新增 `MipmapMode{None,Nearest,Linear}` 枚举替换 `DrawImageOptions.UseMipmaps` bool**，DrawImage/Ex + `QuadDrawOptions` + 批绘 sprite 三处同挂，按接口全量切换制不留 shim，同提交改完所有调用点与旧钉；**D5** 两套解码注册表**收敛为单一真源**（真源在 `render/internal/image`，`ui/io` 公开口与异步池语义零改动改委托），动图载体用**新类型 `*AnimatedImage`**（不给 `ImageBuf` 加帧字段）；**D6** 生成**按需 + 幂等**（采样档要 mip 才产链、纹理记 `mipmapGenerated` 位、层数 `min(log2 w, log2 h)`），4/3 预算低估随批 2 修，并实测开 mip 后 128 条/64 MiB 门能装多少张登记进证据。
以下为拍板依据原文（保留存档）。
- **D1 格式范围**：推荐 **批 1 做 GIF + APNG + 动图 WebP + EXIF 方向**（四者都有标准语义可对齐：`FrameInfo` 五字段 `image_generator.h:35-52`、准确枚举名 `image_generator_apng.cc:446-473`、APNG 容器先例 `cc:499-500`、EXIF 解码期应用 `image_generator.cc:144-189`；纯 Go 可行性已实测 §3.1）。**AVIF/HEIF 处置三选一**：(a) 本批剔除并同步修订 PLAN §7.4 `:262` + 台账 `:296` + catalog `:895`，登记为远期专项（依据：参考框架本树无此能力 §2A、Go 生态无 AV1 熵解码器）；(b) 只做 ISOBMFF 容器解析 + `avif/heif` 能力探测 + 清晰哨兵错（不产像素，为后端解码留接口）；(c) 真解码 = purego dlopen 外部解码库（本仓已有先例 `gpu/gwgpu/ffishim/libload_unix.go:23`、go.mod 已含 purego、视频线 `video/ffmpeg` 也是 purego 绑定），代价是引入系统库依赖与「render 层禁后端特有类型」`AGENTS.md:81` 的分层争议。**推荐 (a)**，(b) 可作 (a) 的免费副产品。
- **D2 mesh gradient 归属**：推荐**剔出 P3.6**，按 PLAN §5.1 `:151` 与 §7.4 补一条独立编号项（或并入 P3.8 的「渐变/Picture」簇）。依据：与图片解码/mipmap 零技术交集（1.11），并入会把「解码 + mipmap」两批变成「+ 新渲染管线 + 第 15 份 WGSL 副本 + CPU 孪生 + recording 命令 + 独立真窗」，违背「每批独立提交点」的收敛节奏；同时必须修订依赖图 `:34` 的节点范围表述（口径矛盾不能留着）。
- **D3 mipmap 生成路线**：推荐 **CPU 统一产链 + hal 逐层上传**（复用 `internal/image/mipmap.go:25-96` 盒式降采样，经 `gpu/hal/command.go:73-296` 的 `MipLevel` 逐层传），而不是「各后端 GPU 原生生成」。依据：① wgpu-native 绑定面无任何生成入口（1.7，本机实测符号清单），走原生等于自写逐级 WGSL 降采样管线，而 gles 侧核不同 → 双后端金图逐位一致直接不成立（P0.1 起本线验收硬标准）；② 已有 gles `glGenerateMipmap` 已加载未用（`gl/context.go:675`），保留为后续性能优化项登记，不在本批混做；③ 上游是「各后端各实现一次」的分工（`blit_pass.h:143,182`），我方 webgpu 缺该原语属环境事实，须在 §4 双后端矩阵与台账按跨平台标记纪律写明哪端真做、哪端为何不做。**盒式滤波核要不要改线性空间**（现 `downsample:62-96` 在 sRGB 逐像素平均）另附 D3 子问：推荐维持 sRGB 现状，与 P3.1b 的 sRGB 目标格式一起谈，避免本批悄悄改混合空间。
- **D4 采样面公开 API 形态**：三选一。(A) **新增 `MipmapMode{None,Nearest,Linear}` 枚举，替换 `DrawImageOptions.UseMipmaps bool`**（`context_image.go:160-162`），三处选项（DrawImage/Ex、`QuadDrawOptions` `m4_quadimage.go:23`、批绘 sprite）同挂——最贴上游 `MipFilter{kBase,kNearest,kLinear}`（`impeller/core/formats.h:602-615`），按 `AGENTS.md` 接口全量切换制不留过渡 shim。**推荐 (A)**。(B) 保留 bool 只把语义转正（改动最小，但 bool 无法表达 mip=Nearest 与 Linear 的差别，标准语义残缺）。(C) 整体换成 Flutter 的 `DlImageSampling` 四值档（把 cubic 也折进去），一次性动所有采样口与 26 份 WGSL 的档位映射，收益是档位语义单一，代价是全量切换波及面最大。**同时必须把「生成≠消费」写进文档与测试**（上游解码内部一律 `kNone`：`image_decoder_skia.cc:59`、`image_decoder_impeller.cc:246`）。
- **D5 两套解码注册表 + 动图接口载体**：推荐**收敛为单一真源**——格式真源在 `render/internal/image`，`ui/io/decode.go:62-133` 的 `Decoder`/`RegisterDecoder`/注册表改为委托 render（公开口 `DecodeFile/DecodeBytes/Pool` 与异步语义零改动）。依据：AGENTS「收敛只动实现、不留重复副本」；不收敛则每加一个格式都要两边同步、支持表必然分叉（1.5 显示真窗全走 ui/io，render 侧只有 conformance 直调）。动图载体**推荐新增 `*AnimatedImage` 类型**（持 `FrameInfo`/`RepetitionCount` + 复用同一 `ImageBuf` 逐帧 `MarkPixelsDirty`），**不给 `ImageBuf` 加帧字段**：依据 1.2 的代际/缓存硬约束（帧字段进 ImageBuf 会污染全部图片缓存路径），且与 Flutter 分层一致（`Codec`/`ImageFrame` `lib/ui/painting.dart:2319,2347-2372` vs 单帧 `ui.Image`；`MultiFrameCodec` 也是解码器侧对象 `multi_frame_codec.cc:78-115`）。
- **D6 生成触发策略与预算修正**：推荐「**按需 + 幂等**」——只有采样档要求 mip 时才产链并上传，纹理记 `mipmapGenerated` 幂等位（对齐 `impeller/core/texture.cc:71-73`），层数按 `MipCount=min(log2 w, log2 h)`（`size.h:137-143`），非 2 的幂尺寸按 floor 语义写明；预算随批 2 修 `image_cache.go:392,426-428` 的 4/3 低估（对齐 `vram_ledger.go:81-91` 与 `texture_descriptor.h:63-75`）。上游无尺寸阈值豁免（§2B），故不引入私有阈值；但 gpui 图片缓存有 128 条/64 MiB 硬门（`render/budget.go:78`），须实测开 mip 后同一预算能装多少张，并把结论写进证据（这是性能红线相关，不能只算不改）。

## 6 分批与验收草案（拍板后细化）
- **批 1 解码半边**：格式注册表收敛 + GIF/APNG/动图 WebP + EXIF 方向 + `AnimatedImage`/`FrameInfo` 公开面 + testdata 落样本（GIF/APNG/WebP-anim/带 EXIF 的 JPEG，小尺寸同 `442B/82B` 量级）+ 单测先红后绿。验收口径按 PLAN `:153`「测试数据入 testdata」。
- **批 2 mipmap 半边**：`MipmapMode` 公开面转正 + 采样器生产接线（`SetPerImageFilter:1096` 零调用者 → 从 `opts` 打通 `tryGPUDrawImage`）+ `image_cache.go:362,377` 多级 + 逐层上传 + 幂等位 + 4/3 预算修正 + 必改两处旧钉（1.10）。
- **批 3 真窗两扇**（AGENTS「每主能力独立真窗，禁止组合窗」）：`examples/ui_wr_p36_imgfmt`（EXIF 八档方向对拍、GIF/APNG/WebP-anim 帧序列与 dispose/blend 合成、帧推进不涨纹理条目）+ `examples/ui_wr_p36_mipmap`（缩样 LOD 对比、CPU/GPU 双后端逐位或声明容差、显存账对账）。每扇三证据（逻辑探针 + 像素断言 + 金图），门禁全套照 `evidence/P3.5/windows-close.md`。
- **负向对照设计（每批一路，红→还原绿）**：摘 APNG 的 fdAT→IDAT 改写 / 摘 EXIF Orient 应用 / 摘帧合成（blend/dispose）/ 摘采样器 mip 接线（证明公开档真驱动）/ 摘 4/3 预算（证明显存账被断言覆盖）。
- **双后端**：批 2 必须 native webgpu + gles 两档像素对拍同字节；metal darwin 交叉编译过，本机不可验按标记写明；browser/web 档复核 `device_browser.go:87`。
- **回归**：逐文件分进程（`tools/per-file-tests.sh`），只减不增；`-race -count=2`；`TestConformanceDualBackend`；五目标构建 5/5×2/2；`scripts/apidoc` rc=0（批 1/2 都动公开面，计数与分类表必须同步）；catalog §13 规划行转正式登记。

## 7 风险
- AVIF/HEIF：PLAN `:262` 明写了、环境（Go 生态 + 参考框架）给不出实现路径，是唯一「承诺与可得性冲突」项 → D1 必须显式拍，不能沉默跳过（沉默=文档与实现分叉）。
- 动图语义正确性面大：blend/dispose × required-frame × 循环 × 时长折算（含 `delay_den==0` 这类畸形，APNG 先例 `cc:476-480`），必须有逐格合成对拍钉，否则「能播」是假绿。
- 采样档一旦转正，`quad_cpu_test.go:373` 的 1% 门与预滤波冲突（1.10）：容差必须重新声明并写明理由，不能靠调大数字蒙过。
- 显存：mip 链 +4/3，图片缓存硬门 128 条/64 MiB（`budget.go:78`），缩略图墙/滚动列表一类场景可能提前触发逐出 → 批 2 需实测并登记数字，别等 P3.7 2.5D 消费时爆。
- metal 端不可本机验（仅 darwin 交叉），按跨平台标记纪律写「本机不可验」，不留空白。
