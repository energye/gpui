# P3.6 批 1b：动图三容器 + AnimatedImage + ui/io 委托（2026-11-02 交付，已提交 `4d4dffdf` 未推送）

直话：批 1b 的关键转折是**把动图 WebP 的解码与帧合成交给成熟纯 Go 库**。我先前手写的那份，对照 libwebp 容器规范原文逐条复核后发现三处语义全反；换库后同一批样本的逐帧像素与独立参照**零字节差**。APNG 用库做容器解析、合成仍由本包唯一的合成器承担（三容器共用一套 dispose/blend 语义）；GIF 继续走标准库。

## 1 依赖决策（用户拍板：按建议引入两库）

| 库 | 版本 | 用途 | 取证 |
|---|---|---|---|
| `github.com/SeriousBug/webp-go-pure` | v1.2.0 | 动图 WebP：`DecodeAnimation` 直接给**已合成整画布 RGBA8 帧** + `Duration`/`LoopCount`/`BackgroundColor`，含 ALPH（有损带透明）与 VP8L | MIT、go 1.22、自身零依赖；`CGO_ENABLED=0` 下 linux-amd64/arm64、windows-amd64、darwin-arm64、js/wasm 全部 `go build` rc=0；**逐帧与参照帧对拍 0/4800 字节差**（5 帧全零，含 duration 100/0/250 与 loop=2） |
| `github.com/kettek/apng` | v0.0.0-20250827… | APNG 容器与帧元数据解析（`XOffset/YOffset`、`DelayNumerator/Denominator`、`DisposeOp/BlendOp`、`IsDefault`、`LoopCount`） | MIT（派生自 Go 的 image/png）、无第三方依赖；合成仍在本包，避免第二套语义真值 |

GIF 保持标准库 `image/gif`（本就是成熟实现，其语义已逐条实测：`Delay` 单位 1/100 秒、透明表达成调色板 alpha=0 项 `reader.go:396-408`、`LoopCount` 0/-1/N 三态 `:577-582`）。

网络实况值得记一句：本机 `GOPROXY` 已配 `goproxy.cn, mirrors.aliyun.com, direct`；我先前手动覆盖成 `proxy.golang.org` 才报超时（该域名在本机解析到 IPv6 且不可达）。用默认代理，三个候选库都能取到并列出版本。

## 2 容器规范原文判死的三处（libwebp `doc/webp-container-spec.txt`）

1. **Blending 位反了**：原文 `0` = *Use alpha-blending*（把当前帧按 alpha 合成到上一画布，无 alpha 通道则视 alpha=255 即整块替换）、`1` = *Do not blend*（覆盖矩形）。我写成 `1→SrcOver、0→Src` → 动图 WebP 逐帧大面积错位（实测 860/1200 像素超容差）。
2. **Disposal=1 的目标错了**：原文「Dispose to the background color. Fill the rectangle covered by the current frame with the background color specified in the 'ANIM' Chunk」，且每轮循环开始画布先填该背景色。我 `backgroundColor()` 返回透明，把已解析到的 ANIM 背景色丢着不用。
3. **Loop Count 语义与 GIF 不同、与 APNG 相同**：原文「This is `Loop Count - 1` for non-zero `Loop Count` values or infinite if the `Loop Count` is zero」。我按「额外重复次数 = 原值」映射，多算一次。（同时订正 `batch1-containers-probe.md` §4 映射表里「WebP 是额外次数」这半句。）

顺带被原文坐实的两条：帧坐标确实是 `2 * Frame X / 2 * Frame Y`、`Frame Width/Height Minus One` 是 1-based（我此前由双编码器字节实测得出，现在有规范背书）；`Frame Duration == 0` 规范明写「由实现决定，许多工具/浏览器像 GIF 一样给最小值」→ 本包按原值交付，节奏策略留给消费者（P3.7/组件线），不悄悄改写。

`ALPH` 的坑也自动消失：规范里 ALPH 的两种压缩（裸数据 / VP8L 压缩、alpha 取 ARGB 绿通道）库都已实现，故我原先登记的「有损带透明的动图 WebP 不支持」这条删除。

## 3 交付内容（代码）

- `render/internal/image/anim.go`：`FrameBlend{Src,SrcOver}`、`FrameDisposal{Keep,RestoreBGColor,RestorePrevious}`、`FrameInfo{Duration,RequiredFrame,Blend,Disposal,DisposalRect}`、`NoRequiredFrame`、`AnimatedImage`（三容器共用的唯一合成器 + 复用同一 `ImageBuf`）。
  - 合成器修过一处真错：`color.Color.RGBA()` 返回**预乘**值，直接当直色会把半透明帧算暗（参照帧抓到，实测 115,40,80 而非 130,60,140）→ 改走 `color.NRGBAModel.Convert` 取非预乘 8 位分量。
  - 混合公式与容器规范 §alpha-blending 一致（`blend.A = src.A + dst.A*(1-src.A/255)`，A=0 时全保留，`Src` 与不透明源走快速路径）。
  - GIF 的「还原背景」回到逻辑屏底色、APNG 回到透明——这条差异是两个独立读端实测出来的，不是推测，注释里写明。
- `render/internal/image/anim_containers.go`：三容器适配层（GIF→stdlib，APNG→kettek 解析 + 本包合成，动图 WebP→库出帧并按 NRGBA 视图喂同一发布路径，避免 `image.RGBA` 的预乘误读）。
- `render/internal/image/anim_load.go`：`LoadAnimatedImage` / `LoadAnimatedImageFromBytes` 按嗅探分发；`DecodeConfig`（只报存储尺寸）。
- `render/internal/image/io.go`：`decodeByFormat` 加 GIF（海报帧=第 0 帧）与动图 WebP（先嗅 `VP8X` animation 位，再交库）；APNG 静态路仍是 `png.Decode` 取主图（标准 PNG 解码器天然忽略动画块）。
- `render/context_image_decode.go` / `context_image_buf.go`：公开别名与常量组 + 4 个新函数（`LoadAnimatedImage`、`LoadAnimatedImageFromBytes`、`DecodeImage`、`DecodeImageConfig`）。
- `ui/io/decode.go`（D5 收敛的另一半）：注册表 4 项（png/jpeg/gif/webp）的 `Decode`/`Config` 全部改调 render 公开口，**不再有第二套解码实现**；`sniffFormat` 改为**按内容定名**（委托后若仍按注册表轮询，任何格式都会被第一个名字 "gif" 接走——这是收敛时踩到并当场修掉的真实陷阱）；`Result.Format`/扩展名提示/异步池语义不变。
- `go.mod`/`go.sum`：新增两个直接依赖。

## 4 样本与参照（数据链本身就是被验证对象）

- `scripts/gen_p36_testdata.py`：GIF 与 APNG 都改为**自己按规范装配字节**，依据是实测到的两处写端缺陷——Pillow 多帧 GIF 每帧重排局部色板（透明帧被画成黑块、`dispose=background` 清出源图没有的颜色），Pillow 9.0.1 的 APNG 写端会给出整块全透明的 fdAT（该处明明有新颜色，Pillow 与 ffmpeg 两个读端都把这一帧读成「什么都没画」）。LZW 用「永不新增字典项」编码，且必须在解码器表长逼近码宽上限**之前**发清码（首次实现按 `2^width` 清码→读端直接判文件非法，GIF 报 `invalid graphic control extension block terminator`：GCE 负载恰好 4 字节，我把 dispose 多写成一字节）。
- 新增判别样本 `apng_keep_erase.png`：与 `apng_restoreprev.png` 帧内容、矩形完全相同，只差第 1 帧 dispose 位（0 vs 2）→ dispose 语义的差异只能在同几何对照上唯一判定（我原来的断言跨了两个不同矩形的样本，测的是我自己写错的期望）。
- `scripts/p36_anim_oracles.py`：参照帧来源分道——GIF/动图 WebP 用 Pillow（自带 GIF 解析器 / libwebp demux），**APNG 用 ffmpeg apng 解码器**，因为 alpha-over 那一格 Pillow 给 `alpha=191`，而容器规范公式与 ffmpeg 都给 `255`；两读端的 RGB 实测逐点一致，差异只在这条 alpha 通道上。中间产物走系统临时目录，不落在 testdata。
- 样本 15 个动图 + 8 个 EXIF + 对照共 30 文件，参照帧 + 元数据 54 文件，单文件 ≤3 KB。

## 5 钉（`render/image_anim_p36_test.go`）

- `TestAnimatedMetadataMatchesAuthoredValues`：15 样本逐帧数 / `RepetitionCount`（含 APNG `num_plays-1`、WebP `LoopCount-1`、GIF 三态）/ 逐帧 `Duration`（含 `den==0→1200ms` 与 0 时长合法）/ `DisposalRect` 非空 / `RequiredFrame ∈ {i-1, NoRequiredFrame}` / 越界返回错误。
- `TestAnimatedCompositingMatchesOracleFrames`：逐帧与参照**逐字节**比对（GIF/APNG/WebP-无损/WebP-有损全 0 差；有损路另设「最大单通道差 ≤40 且超阈像素 ≤15%」的容差，实测 **0**，容差是冗余护栏）。
- `TestDisposalVariantsActuallyDiffer`：`apng_keep_erase` vs `apng_restoreprev`、`apng_keep` vs `apng_restorebg`、GIF keep vs restore-bg 必须产出不同像素；并钉住「APNG 还原背景=透明、GIF 还原背景=逻辑屏底色」这条实测差异。
- `TestAnimatedFrameAdvanceKeepsOneBuffer`：三帧推进 `Image()` 同一指针、`GenerationID` 不变、每帧 `IsGPUDirty()` 为真；乱序回播帧 0 必须复现参照像素。
- `TestSingleFrameStaticPathForGIF`：GIF 走普通 `LoadImageFromBytes` 得海报帧并与参照对拍；单帧 PNG 不得从动图门通过。
- `TestUIIODecodeGoesThroughRenderSource`：`SupportedDecoders()` == {gif,jpeg,png,webp}，四类样本经 ui/io 异步池解出且 `Result.Format` **等于字节实际容器**（不是注册表第一个名字），垃圾字节必须报错。
- 两处收紧：样本或参照缺失从 `t.Skipf` 改 `t.Fatalf`——数据在仓库里，缺了就是仓库缺陷，静默跳过等于假绿。

## 6 门禁（全跑完，含一次真回归的修复与重跑）

| 项 | 结果 | 日志 |
|---|---|---|
| apidoc | rc=0（8 新符号逐名登记 §3.5「动图解码与帧合成」行；§0 计数 151/192/219/18/194/28，写明两库与依据） | 本文 |
| vet | 触及三包 rc=0（`./render/...` 全扫仍命中 P3.1 线 raster 存量坏包，HEAD 同红） | 本文 |
| 定向 | 两包 ok（`targeted.txt`） | `targeted.txt` |
| conformance 双后端 | PASS（`ok render 1.2s`） | `conformance.txt` |
| `-race -count=2` | `render/internal/image`、`ui/io`、`render/recording`、`render`(本批钉) 四组 ok | `race/race.txt` |
| 五目标构建 | linux-amd64/arm64、windows-amd64、darwin-amd64/arm64 × {默认, nogpu} = **10/10 exit 0**（324–332 包） | `build/*.txt` |
| 主逐文件 | 730 条：699 PASS / 21 SKIP / **10** FAIL / 1 NO_TESTS —— 比基线多出的正是 `ui/io/decode_t4_test.go`，**这是我改委托引入的真回归**（见 §6.1） | `per-file-tests/summary.tsv` |
| 补充分包 | 72 PASS / 2 SKIP / 6 FAIL，6 枚与基线逐名相同（P3.1 线 raster 存量坏包） | `per-file-tests-extra/summary.tsv` |
| 修复后重跑 | `ui/io` 三个测试文件全 PASS、render 6 枚 FAIL 全落基线名单、`image_anim_p36`/`image_decode_p36` PASS（203 PASS/6 FAIL/7 SKIP）；引用 ui/io 的 5 个 example 目录无测试文件，改跑受影响包五目标×{默认,nogpu} **10/10 rc=0** | `per-file-tests-recoat/summary.tsv`、`build-recoat/targets.txt` |

### 6.1 一次真回归的现场（不藏着）

主 sweep 里唯一的新增红是 `ui/io/decode_t4_test.go`：三条 `io: unsupported image format`。根因是我把 `sniffFormat` 改成「只按内容定名」后，把 `RegisterDecoder` 那条**既有契约**——测试可注册自定义/桩格式（`t4stub`/`t4slow`/`t4probe`，见 `ui/io/decode_t4_test.go:67,78,120-134`）——挤掉了，桩格式的字节再也进不了路由。修法不是回退委托，而是保留「内容定名为主 + 轮询注册表兜底」：内容能被引擎认出时按内容取名字（委托后必须，否则任何格式都会被排序第一个名字接走），认不出时回落到注册表轮询，自定义解码器重新可用。修复后 `ui/io` 整包绿，受影响包交叉编译 10/10 绿。

## 7 显式不做与归属（批 1c）

- **真动画播放链**（帧调度、时长到点换帧、暂停/seek、循环收尾）不在批 1b：引擎只交付「帧 i 的像素 + 该帧元数据」，节奏归消费者（与 Flutter 把播放节奏放 framework 一致）。归批 1c，需要一条 `ui/kit` 或 `ui/rendering` 侧的接线才算生产。
- **`DrawImage` 路径的动图消费**（把 `AnimatedImage.Image()` 画进 Context、每帧只重写同一张纹理的端到端像素/显存对账）归批 1c；批 1b 只钉到「缓冲与代际不变」这一层。
- 组件库/2.5D 侧的动图入口 = 三支柱生产接线，仍为零消费（按 AGENTS 不得当生产标绿）。
- 线性空间 alpha 混合（规范 SHOULD 项）与 P3.1b 的 sRGB 目标格式一并谈，本批不动混合空间——已在 §2 记录该偏差与出处。
