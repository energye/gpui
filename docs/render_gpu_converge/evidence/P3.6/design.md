# P3.6 正式方案：图片格式（GIF/APNG/动图 WebP/EXIF）+ GPU mipmap（2026-11-02 拍板后落盘）

依据：`evidence/P3.6/assessment.md`（现状实测）+ 用户拍板 D1–D6（D1 剔除 AVIF/HEIF / D2 mesh gradient 单列 P3.9 / D3 CPU 统一产链 + 逐层上传 / D4 `MipmapMode` 枚举换掉 `UseMipmaps` bool / D5 解码注册表收敛单一真源 + 动图用 `*AnimatedImage` / D6 按需 + 幂等 + 4/3 预算随批 2 修）。本文是 PLAN §3「P3.1–P3.8 每项先补文档（能力定义、公开 API、双后端实现方式、验证方案），再实现」的那份文档。

## 1 能力定义与语义口径

### 1.1 帧模型（对齐 Flutter `ImageGenerator::FrameInfo`，逐字段）
- 每帧元数据五字段：`Duration`（ms，0=只显示一帧）、`RequiredFrame`（解码这一帧必须先有哪一帧，无则「任意/无需」）、`Blend`、`Disposal`、`DisposalRect`。
- **枚举取值逐值对齐上游，不自创**：`Blend ∈ {Src, SrcOver}`；`Disposal ∈ {Keep, RestoreBGColor, RestorePrevious}`（依据 `flutter/engine/lib/ui/painting/image_generator_apng.cc:446-473` 的逐值映射；旧名 `kDiscard`/`kNoBlend` **不用**；单帧默认 `Keep`，见 `image_generator.cc:60`）。
- `RepetitionCount`：**负数=无限循环**，非负=额外重复次数（`image_generator.cc:118-121`）。总帧数与循环的播放节奏**不放进引擎**（Flutter 也放在 framework 侧，`image_stream.dart:1041-1101`），引擎只交付「帧 i 的像素 + 该帧的 duration/disposal/blend/required」。
- 畸形数据按上游既有口径处理并写进测试：APNG `delay_den==0` → 按 100 处理（`image_generator_apng.cc:476-480`）。
- 帧合成语义必须在引擎内做对：`RestoreBGColor` = 处置矩形清成透明（`multi_frame_codec.cc:143-147`），`RestorePrevious` = 保留该矩形上一帧内容（`:120-137`），`RequiredFrame` 缺失时按「blank slate」降级（`:78-115`）。**「能播」不算绿，逐格合成对拍才算绿。**

### 1.2 各格式实现路径（全部纯 Go、`CGO_ENABLED=0`/`!nogpu` 均可编译）
- **GIF**：stdlib `image/gif`（`DecodeAll` 给 `Frames[]`，含 `DelayTime`/`DisposalMethod`/`Transparent`/`HasTransparency`）；stdlib 的 disposal 0..3 → 我方 `Keep/RestoreBGColor/RestorePrevious` 映射（0 未指定按 `Keep`）。
- **APNG**：自研容器层——`acTL` 取 `num_frames/num_plays`、`fcTL` 取偏移/尺寸/`delay_num/den`/`dispose_op/blend_op`、`fdAT` 剥 4 字节的 `sequence_number` 改写成 `IDAT` 后**复用单帧 PNG 解码**（照 Flutter 的结构先例 `image_generator_apng.{h:73-74,99-113}`、`cc:499-500,576-580`）。
- **动图 WebP**：自研容器层——RIFF `VP8X.flags` 的 `animationBit` + `ANIM`（`bg_color`/`loop_count`）+ 每帧 `ANMF`（x/y 偏移、w/h、`duration`、flags 的 dispose/blend 两位），逐帧 payload 内仍是 `VP8 `/`VP8L`/`ALPH` chunk，直接调 `golang.org/x/image/vp8`（`NewDecoder/Init/DecodeFrameHeader/DecodeFrame`）与 `x/image/vp8l`（`Decode`）+ 既有 ALPH 反滤波路径（静态解码器 `webp/decode.go:76-99,146,222` 同调法）。**批 1 第一步先做规范核对**：ANMF 内 VP8 chunk 送进 `vp8.Decoder` 时是否需要按 WebP 规范处理 `scale` 字段与 1/2 尺寸标志（`vp8.FrameHeader` 含 `HScale/VScale`），核对结论写进证据后再定偏移。
- **EXIF 方向**：自研 APP1/TIFF-IFD 解析（`Orientation` tag=0x0112），12 值档枚举含 `Unspecified`。**解码期应用**，不是绘制期加矩阵（`image_generator.cc:144-189`：解到 temp 再 Orient）；且 `Size()`/`DecodeConfig` 返回的尺寸**必须已按方向交换宽高**（`image_generator.cc:86-95` 用 `SkEncodedOriginSwapsWidthHeight`）。默认应用；需保留原始像素时走显式 `DecodeOptions{ApplyOrientation:false}`（对齐 Skia「报告 origin、由调用方决定」与 Flutter「默认应用」的合体：应用是默认，不是唯一）。
- **AVIF/HEIF**：本项不做（D1）。仅做 ISOBMFF box 级探测（识别 `ftyp` brand `avif/heix/hevc/avis`）+ 返回明确哨兵错 `ErrUnsupportedFormat`（顺带激活 `render/internal/image/io.go:31` 的死码），不产像素。真解码登记为远期专项，触发条件=有可用的 AV1 熵解码通路。
- **格式识别收敛为嗅探优先 + 扩展名仅作提示**：现状 render 侧扩展名优先（`io.go:73-81`）且嗅探丢弃 format（`:108`）；改为「魔数嗅探定格式、扩展名只做提示/兜底」，并把嗅探返回的 format 真正用起来（对外可查、测试可钉）。对齐上游注册表轮询（`image_generator_registry.cc:30-42` 的注册顺序即优先级）。

### 1.3 解码注册表收敛（D5）
- 真源=`render/internal/image`：单一格式注册表（魔数嗅探 + 帧元数据契约），GIF/APNG/WebP-anim/EXIF 全在此落地。
- `ui/io/decode.go` 的 `Decoder`/`RegisterDecoder`/`SupportedDecoders`/扩展名表（`:62-133,651-687`）改为**委托 render**；公开口 `DecodeFile/DecodeBytes/…WithContext/Default/Pool`（`:172,155,474-489`）**签名与异步语义零改动**，`Result` 增加帧元数据字段（不动既有字段）。
- 验收钉：两边支持表逐项相同（同一列表断言），杜绝「一边能开 GIF 一边不能」。

### 1.4 动图载体：`*AnimatedImage`，不动 `ImageBuf`（D5）
> **2026-11-02 批 1c 拍板 D7=(B) 修订本节机制口径**：「不动 `ImageBuf` 结构、复用同一缓冲」保留，但**代际恒定 + `MarkPixelsDirty` 原地重写这条路推翻**，改成 Skia 口径——像素变了就换图号，缓存靠**图号变更监听器精确逐出** + **尺寸分桶空闲池**回收复用；「连播 N 帧条目与显存不增长」目标不降级，手段改变。评估实测（含顺带抓出的 `Pixmap`/`ImageBuf` 键空间完全重叠第四洞）见 `batch1c-assessment.md`，机制与钉见 `batch1c-design.md`。本节下方 `MarkPixelsDirty`/「必须钉 N 帧推进后 imageCache 条目数不变」的**手段描述**按 `batch1c-design.md` §2 执行。
- 理由（硬约束）：`ImageBuf` 的代际即 GPU 纹理缓存键（`image_cache.go:221-253` + 批合并谓词 `image_pipeline.go:1160`），给 `ImageBuf` 加帧字段会污染全部图片缓存路径；且帧推进若每帧 `NotifyPixelsChanged`（`buf.go:272`）会**每帧新建纹理条目**、显存单调增长。
- 正解：`*AnimatedImage` 持容器元数据 + **复用同一 `ImageBuf`**，逐帧 `DecodeFrame` 写入后 `MarkPixelsDirty()`（`buf.go:296` 原地重写、保留 genID）。批 1 必须钉「N 帧推进后 imageCache 条目数不变」。
- 分层与上游一致：`AnimatedImage` ↔ Flutter `Codec`/`MultiFrameCodec`（解码器侧对象），单帧 `ImageBuf` ↔ `ui.Image`。

### 1.5 GPU mipmap 语义（D3/D4/D6）
- **采样档与生成解耦，且「生成 ≠ 消费」**：`MipmapMode` 决定消费；产链只在「该图这次要用 mip」时发生。上游同样把解码内部一律钉 `kNone`（`image_decoder_skia.cc:59`、`image_decoder_impeller.cc:246`）。
- `MipmapMode{None,Nearest,Linear}` 三档，语义逐一对齐 `impeller/core/formats.h:602-615 MipFilter{kBase,kNearest,kLinear}`：`None`→采样器 `MipmapFilterModeNearest` + 纹理单级（只吃 level0）；`Nearest`→`MipmapNearest`；`Linear`→`MipmapLinear`。与 `InterpolationMode{Nearest,Bilinear,Bicubic}`（filter 档/质量档）**正交**，不折成 Flutter 的四值档（D4 选 (A) 而非 (C)）。
- 层数：~~`min(floor(log2 w), floor(log2 h)) + 1`~~ **2026-11-02 批 2 开工实测纠正为 max**：`NumLevels = 1 + floor(log2(max(w,h)))`（`engine/tex/mipmap.go:214-226` 与 `render/internal/image/mipmap.go:42-43` 两处既有实现同为 max，逐级 `max(1, prev/2)` 收敛到 1×1；Impeller 实际口径也是 max，本行旧写法与上游和仓内实现均不符——`impeller/geometry/size.h:137-143`），非 2 的幂按 floor，**写明尺寸不整除时的截断口径**。
- 生成：CPU 盒式链复用 `render/internal/image/mipmap.go:25-96`，滤波空间维持 sRGB（与 P3.1b 一起谈，本批不偷跑混合空间）；上传走 hal 已有的逐层口（`gpu/hal/command.go:73-296`，`ImageCopyTexture.MipLevel`）。
- 幂等：纹理记 `mipmapGenerated` 位（对齐 `impeller/core/texture.cc:71-73`），同一 `ImageBuf` 重上传不重复产链。
- 预算：`image_cache.go:392,426-428` 的 `need`/`usedBytes` 改按逐层求和（等价 4/3，精确式对齐 `impeller/core/texture_descriptor.h:63-75`），与 hal 侧 `vram_ledger.go:81-91` 已乘 4/3 的账对齐。
- 后端分工（跨平台标记一次写齐）：**webgpu(native)** ✅ 本批真做（创建透传已存在 `gpu/rwgpu/texture.go:264-268,288-297`，绑定形已可 mipmap `image_pipeline.go:846-860`）；**gles** ✅ 真做（多级创建已实现 `device_linux.go:284-294`、`TEXTURE_MAX_LEVEL` `:314-325`，Win 孪生 `device.go:193-307`；本批只做「收 CPU 链字节」，`glGenerateMipmap` 留作后续优化项，`gl/context.go:675` 零调用现状登记）；**metal** ⚠️ 逻辑已备（`gpu/gwgpu/metal/device.go:236-241`、`conv.go:205-211`）但 darwin 交叉编译，**本机不可验**；**web 档（browser）** ⚠️ 复核 `gpu/webgpu/device_browser.go:87`，结论按实际写。
- **双后端逐位一致的根据**：链在 CPU 同一处产出 → 两档上传的是同一份字节，金图容差可按 P0.1 默认 0；若各后端 GPU 原生生成则该前提不成立（这是 D3 选 CPU 产链的核心理由，wgpu-native 绑定面还零生成入口，实测符号清单见 assessment §1.7）。

## 2 公开 API 面（批 1 + 批 2，实现时逐符号转正式登记并跑 apidoc）
```
// 批 1 解码
type FrameBlend int      // FrameBlendSrc / FrameBlendSrcOver
type FrameDisposal int   // FrameDisposalKeep / RestoreBGColor / RestorePrevious
type ImageOrientation int // 12 值：Unspecified + EXIF 8 档（TopLeft/TopRight/BottomRight/BottomLeft/LeftTop/RightTop/RightBottom/LeftBottom）
type FrameInfo struct { Duration time.Duration; RequiredFrame int; Blend FrameBlend; Disposal FrameDisposal; DisposalRect Rectangle }
type DecodeOptions struct { ApplyOrientation bool; ... }   // 零值=默认应用方向
type AnimatedImage struct{ ... }  // FrameCount() int / RepetitionCount() int / FrameInfo(i) FrameInfo
                                 // DecodeFrame(i int, prior *ImageBuf) error / Image() *ImageBuf（当前帧复用缓冲）/ Orientation() ImageOrientation / Dispose()
func LoadAnimatedImage(path string) (*AnimatedImage, error)
func LoadAnimatedImageFromBytes(data []byte, opts DecodeOptions) (*AnimatedImage, error)
func LoadImageFromBytes(data []byte, opts DecodeOptions) (*ImageBuf, error)   // 现有 io.go:94 的再导出，签名按 opts 定
func ImageOrientationOf(data []byte) (ImageOrientation, error)                 // 不解码只报方向
var ErrUnsupportedFormat                                                       // 现有死码激活（io.go:31）
// 批 2 mipmap
type MipmapMode int        // MipmapNone / MipmapNearest / MipmapLinear
// DrawImageOptions.UseMipmaps bool → Mipmap MipmapMode（全量切换，不留 shim）
// QuadDrawOptions + 批绘 sprite 选项同挂 Mipmap
```
计数影响（实测基线 144 类型/184 顶层函数/219 Context/109 Canvas/172 常量/26 变量）：批 1 预计 +6 类型、+7 顶层函数、+11 常量（blend/disposal/orientation）、+~8 方法（`AnimatedImage` 上，不进 Context）；批 2 预计 +1 类型、+3 常量、**净 -0**（`UseMipmaps` 字段改类型不改签名数量）。最终以 `go/ast` 实测为准写进 catalog §0。

## 3 验证方案（三证据 + 双后端 + 负向，逐批）
- **能力单测**（先红后绿，`testdata/` 落样本，禁在测试里循环造标准数据，缺数据 `t.Skipf` 不静默假绿）：
  - 组 1 容器解析：GIF/APNG/动图 WebP 的 `FrameCount`/`RepetitionCount`/逐帧 `Duration`/`Disposal`/`Blend`/`DisposalRect`/`RequiredFrame` 对 authored 元数据逐值钉；`delay_den==0→100` 畸形钉；fdAT 序号乱序/跨 chunk 钉；ANMF 偏移与 flags 两位钉。
  - 组 2 帧合成：同一序列在「`Src` vs `SrcOver`」「`Keep` vs `RestoreBGColor` vs `RestorePrevious`」下逐像素输出与独立参考实现（手算合成序）对拍，像素全等不留容差。
  - 组 3 EXIF：12 档方向逐一对拍「解码出口尺寸已交换 + 像素旋镜正确」，与 `ImageBuf`→绘制链一致；`ApplyOrientation:false` 对照。
  - 组 4 缓存不涨：动图连播 N 帧后 `imageCache` 条目数与显存记账不增长（钉 assessment §1.2 的硬约束）。
  - 组 5 注册表收敛：render 与 ui/io 支持表逐项相同 + 动图经 ui/io 异步池路径同样可解。
  - 组 6 mipmap：层数公式（含非 2 的幂、1×N 退化）、CPU 链与 `image_cache` 上传字节逐层一致、幂等位（重复用不重产）、`MipmapMode` 三档采样结果单调（缩样方差不增）、4/3 预算对账（render 自计 vs hal 真账相等）。
- **真窗三扇**（AGENTS「每主能力独立真窗，禁组合窗」，按 §4.2 模板登记；**2026-11-02 批 1c 拍板 D9 由两扇改三扇**）：
  - `examples/ui_wr_p36_imgfmt`：EXIF 八档方向对拍卡、GIF/APNG/动图 WebP 帧序列与 dispose/blend 合成卡、嗅探优先于扩展名卡。
  - `examples/ui_wr_p36_animplay`（**D9 新增，批 1c 播放与消费链的独立真窗**）：连播期间 `imageCache` 条目数/`usedBytes`/`hal.VramLiveBytes`/`VramLiveCount` 四路对账卡、空闲池命中（`CreateTexture` 次数）卡、同一动图两消费者（窗口 + 离屏 target）不冻结对拍卡、图案填充采样场逐帧跟手卡。原 §3 把「帧推进期间显存与条目数对账卡」错放在 mipmap 窗，现改归本窗（帧推进与 mipmap 无技术交集）。
  - `examples/ui_wr_p36_mipmap`：同一图按 `None/Nearest/Linear` 三档缩样并排（摩尔/混叠可见差）、CPU/GPU 双档同源。
- **金图**：新窗新金图，容差按 P0.1 默认 0；**存量金图零变化**为批 1 验收项；批 2 若改变既有图片缩放像素，必须逐窗 A/B 归因并显式声明（不允许靠调大容差蒙）。
- **双后端**：批 1 CPU 链双后端同源必跑；批 2 三档 × {native webgpu, gles} 像素对拍 + `TestConformanceDualBackend`；metal darwin 交叉过编译，本机不可验按标记写明。
- **负向对照（每批一路，红→还原绿）**：批 1=摘 fdAT→IDAT 改写 / 摘 EXIF Orient / 摘 `RestorePrevious` 保留逻辑；批 2=摘逐层上传（只传 level0 却把采样器设成 Linear）/ 摘采样器 mip 接线（证明公开档真驱动）/ 摘 4/3 预算（证明显存账被断言覆盖）。
- **门禁全套**：`scripts/apidoc` rc=0、`go vet` rc=0、定向 `-count=1` 全绿、`CGO_ENABLED=1 -race -count=2`（动图帧状态跨线程 + 缓存）、conformance 双后端 PASS、五目标构建 5/5 各 {默认,nogpu} 2/2、逐文件分进程回归只减不增（新增红必须 clean HEAD A/B 归因）。
- **性能与资源**：批 2 实测并登记「开 mip 后 128 条/64 MiB 预算能装多少张常用缩略图」与产链耗时（`budget.go:78`），超红线不合入；关窗显存/条目回收按 `docs/ENGINE_GPU_RESOURCE_LIFECYCLE.md` 口径核。

## 4 批次与提交点（每批独立提交，提交前与用户确认，不推送除非另行确认）
- **批 1a 解码半边·格式与方向 ✅ 已交付（2026-11-02）**：容器嗅探（`Encoding`/`SniffEncoding`，扩展名降为提示）+ EXIF 方向（`ImageOrientation` 9 档，解码期应用、返回尺寸已交换宽高，含 PNG `eXIf`/WebP `EXIF`）+ `DecodeOptions{PreserveOrientation}` 两扇门 + AVIF/HEIF 品牌识别与具名 `ErrUnsupportedFormat`（原死码激活）+ testdata 样本与 Pillow `exif_transpose` 参照网格。证据 `evidence/P3.6/batch1a.md`、`batch1-containers-probe.md`。
- **批 1b 解码半边·动图三容器**（原批 1 剩余）：GIF（stdlib `image/gif`：透明=调色板 alpha=0 项 `reader.go:396-408`、`LoopCount` 0/-1/N 映射 `:577-582`、帧 `Bounds()` 即 `DisposalRect`）+ APNG（fcTL 26 字节、**dispose/blend 单字节**、fdAT→IDAT 复用单帧解码、`den==0→100` 采上游口径）+ 动图 WebP（ANMF **16 字节头无补白**、**X/Y 单位 2 像素**、W/H 为像素减一、VP8/VP8L 子 chunk 原样喂 `x/image`）+ `*AnimatedImage`/`FrameInfo` 公开面（复用同一 `ImageBuf` + `MarkPixelsDirty`，硬钉连播不涨缓存条目）+ 组 1–5 单测 + `ui/io` 注册表改委托（D5 另一半）。字段依据 `batch1-containers-probe.md`。
- **批 1c 播放与消费链（图号=内容版本）✅ 已交付（2026-11-02）**：D7=(B) 落地——单一图号分配器（修 `Pixmap`/`ImageBuf` 键空间重叠第四洞）+ 原地改写换号 + 图号变更监听精确逐出 + 尺寸分桶空闲池 + `MarkPixelsDirty`/`TakeGPUDirty`/`ContentDirty`/`rewriteImage` 通道退役（`AnimatedImage`、`ExportImageBuf`、`video_bridge` 三用户全量切换）+ 钉 P1–P10。方案与实测 `batch1c-design.md`、评估 `batch1c-assessment.md`。
- **批 1d 生产接线（D8 拆出）**：`ui/rendering.RenderImage` 动图入口 + `ui/kit` 控制动词，节奏走 `ui/scheduler.Ticker`（引擎不持节奏）；交付前动图按「已验证待接线」记，不得标生产。
‑ **批 2 mipmap 半边 ✅ 已交付（2026-11-02，已提交 `e0684738` 未推送；评估/方案/交付三件齐 `batch2-assessment.md`/`batch2-design.md`/`batch2.md`）**：1.5（`MipmapMode` 全量切换 + 采样器生产接线打通 `SetPerImageFilter` 零调用者问题 + `image_cache.go:362,377` 多级 + 逐层上传 + 幂等 + 4/3 预算 + 组 6 单测 + 旧钉同步：`r1_residual_gpu_test.go:106-145` 反向钉改语义、`quad_cpu_test.go:303-379` 容差重声明、`internal/image/draw.go:235-258` 的 `Release` 泄漏副修）。
- **批 3 真窗三扇** + 文档收口（catalog §13 → 正式登记、台账状态、evidence 汇总）。

## 5 显式不做与登记（不留口头）
- AVIF/HEIF 像素解码（D1，远期专项，条件=有可用 AV1 通路）。
- mesh gradient（D2，已单列 **P3.9**）。
- `glGenerateMipmap` 后端原生产链（D3 后续性能优化项，登记不混做）。
- mipmap 盒式滤波改线性空间（归 P3.1b / 色彩管理线，本批不动混合空间）。
- iCCP/gAMA/白点等 ICC 色彩管理（P3.1b 线，P3.6 只碰方向不碰色域）。
- 动图播放节奏与 timer（Flutter 同放 framework；引擎只交付帧与元数据，节奏归 2.5D/组件线消费）。
- 压缩纹理的 mip（上游明确强制单级豁免 `image_decoder_impeller.cc:637`，我方暂无该格式路径）。
- 动图 WebP 的 ALPH/blend/dispose/loop 语义 = **批 1b 交成熟纯 Go 库 `SeriousBug/webp-go-pure` 承担**（容器规范复核判出我方手写三处语义反了，库输出与参照帧零字节差），本包不再自研该算法；APNG 容器解析同理交 `kettek/apng`。见 `evidence/P3.6/batch1b.md` §1–§2。

## 6 风险（按可能性排序）
1. **动图语义面大**（blend×disposal×required-frame×循环×畸形 delay）：靠组 1–2 的逐值与逐像素对拍压住，缺一项就是假绿。
2. **动图 WebP 容器细节**：ANMF 内 VP8 payload 与 `vp8.Decoder` 期望字节流的偏移/`scale` 标志需先做规范核对（批 1 第一步），错一位则整帧错位。
3. **采样档转正撞存量像素**：`quad_cpu_test.go:373` 的 1% 门与预滤波必然冲突（1.10），必须重新声明容差并写明理由；`UseMipmaps→Mipmap` 全量切换涉及所有调用点与录制面，改一半就是行为分叉。
4. **显存 +4/3**：64 MiB/128 条硬门（`budget.go:78`）下缩略图墙类场景可能提前逐出，批 2 要实测数字并登记，别把问题留给 P3.7。
5. **metal/web 档不可本机验**：按跨平台标记纪律写明状态，不留空白格。
