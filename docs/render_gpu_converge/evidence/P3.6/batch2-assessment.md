# P3.6 批 2 影响面评估：GPU mipmap（2026-11-02 开工登记）

直话：批 2 不是"从零写 mipmap"。仓里**三样东西都已存在但互不相连**——① CPU 产链（`render/internal/image/mipmap.go`，盒子 2×2，NPOT 与 1px 退化都对，且**有单元测试**）；② 逐 per-image 的采样器机制（`SamplerForFilter` + `filterSamplers` + `ImageDrawCommand.SetPerImageFilter`，能产出 `MipmapFilterModeLinear`，**全仓零调用者**）；③ hal 的逐层上传口（`ImageCopyTexture.MipLevel`，gles/metal/rwgpu/浏览器四端都已实现）。缺的是**把它们接起来 + 纹理开多级 + 预算按层算**。同时有一处对外撒谎的公开口要关：`DrawImageOptions.UseMipmaps` 在 GPU 路整段不被读取。

一手实测与行号（子代理报告 + 本机逐条复核）：

## 1 现状（file:line）

- **公开口撒谎**：`DrawImageOptions.UseMipmaps bool` 在 `render/context_image.go:162`；GPU 路 `render/context_image_gpu.go:47-129` 只读 `Interpolation`(`:100-101`) 与 `BlendMode`(`:102`)，**从不读它**。唯一被读处是 CPU 兜底：`render/context_image_draw.go:157`（只在 `InterpBicubic` 分支里置位）→ `render/internal/image/draw.go:235`。全仓生产代码没有任何一处把它设成真。
- **CPU 产链的两个真问题**（`render/internal/image/draw.go:232-250`）：
  1. **每次缩小绘制都重建整条链**（`GenerateMipmaps(src)` 在 draw 里，无缓存）；
  2. **从不 `Release()`**（`mipmap.go:156` 是归还口，链级走 `GetFromDefault`/`pool.go:111` 的 8/bucket 池）→ 每次绘制把 1..N 级泄给 GC 并饿着池子。正确用法见 `mipmap_test.go:101,137,171,244`（`defer` 归还）。
- **采样器死代码**：`image_pipeline.go:1103 SetPerImageFilter`、`:1117 SamplerDescriptorForImageFilter`（`:1133` 会发 `MipmapFilterModeLinear`）、`:1415 SamplerForFilter`（`:1432-1443` 按需建并缓存进 `filterSamplers:246`）——**零调用者**（全仓 grep 只剩定义本身、`engine/tex/doc.go:60-61` 两条注释、`:133` 注释）。活采样器只有两个且硬编 `MipmapFilter: MipmapFilterModeNearest`：`:816-828`（linear）、`:829-841`（nearest）；绑定处 `render_session.go:4472,5295,5300` 只走 `SamplerFor(nearest)`。
- **合并谓词已经能按采样档分家**：`ImageSamplerKey{Nearest,Bicubic,MipmapLinear,Aniso}`（`:1060-1065`）+ `EffectiveImageSamplerKey`（`:1083`，nearest/bicubic 屏蔽 mip 档）+ `FilterKey()`（`:1092`）已在 `canMergeImageDraw` 比对（`:1173`）——接线时**不需要**再动合并逻辑。
- **纹理级数全部钉死 1**：图像缓存的两处创建 `image_cache.go:436`（CreateTexture）与 `:451`（CreateTextureView）都是 `MipLevelCount: 1`；上传只写 level 0（`:480` `MipLevel: 0`）。其余 80 处非测试 `MipLevelCount: 1` 属蒙版/图集/RT/字形等，**不该动**。
- **四端都能收多级**（无需新增后端能力）：gles 全链分配并设 `TEXTURE_MAX_LEVEL`（`gpu/gwgpu/gles/device.go:262-271,294`；Linux 变体 `device_linux.go:284,322`），上传按 `dst.MipLevel`（`queue.go:191`）；metal `setMipmapLevelCount`（`gpu/gwgpu/metal/device.go:236-239`）+ `queue.go:359,425`；rwgpu 把 0 夹成 1、其余透传（`gpu/rwgpu/texture.go:265-267,294`）；webgpu-native 转交（`gpu/webgpu/device_create.go:79`）、浏览器仅在 `>1` 时写 `mipLevelCount`（`gpu/webgpu/internal/browser/convert_resources.go:75-76`）。**hal 无 GPU 侧生成入口**：`glGenerateMipmap` 只是 GL 绑定（`gpu/gwgpu/gles/gl/context.go:675`、`context_linux.go:1315`），无 hal 包装、零调用。
- **预算两头都不对**：hal 侧 `VramTextureBytes` 收 `mips` 参数但只按 `mips>1 → n*4/3` 估上限（`gpu/hal/vram_ledger.go:80-91`，记账点 `gpu/rwgpu/texture.go:314,342`）；render 侧条目字节只算 level 0（`image_cache.go:466 need := w*h*4`、`:500 nbytes`、`:509 entry.bytes`、`:285 evictIfNeeded(w*h*4+1)`）→ 开了 mip 后 render 自计比 hal 真账小，128 条/64 MiB 硬门（`render/budget.go:135`）会误判容量。
- **两处旧钉要改语义**：`render/r1_residual_gpu_test.go:106-146 TestR1_ImageMipmapUsesGPU` 现在断言的是"这口被忽略且留在 GPU"（`:140-141` `GPUOps==0` 即红、`:143-144` `CPUFallbackOps!=0` 即红）——**它把假语义钉住了**；`render/quad_cpu_test.go:369-377` 是 CPU/GPU 一致性门（变化像素 >1.0% 即红，`:375` nearest 逐位、`:378` bilinear MeanAbs>1.0 即红），预滤波 mipmap 与未预滤波的 CPU 双线性缩小必然超 1%，**容差必须显式重声明并给理由**。（次要：`s3c_m3_residual_gate_test.go:281-291` 只查 alpha≥200，真 mip 不破它。）

## 2 真源自身要纠正的两处

1. **层数公式**：design.md §1.5 写「`NumLevels = min(floor(log2 w), floor(log2 h)) + 1`」——**与上游和仓内实现都不符**。Impeller `Size::MipLevels()` 与仓内两份实现都是 **max**：`engine/tex/mipmap.go:214-226 NumLevels = 1 + floor(log2(max(w,h)))`、`render/internal/image/mipmap.go:42-43` 同式；逐级 `max(1, prev/2)`（`mipmap.go:64-65`）保证 NPOT 与 1px 退化收敛到 1×1。批 2 按 max 口径落，并订正 design.md 该行。
2. **第二份三档枚举**：`engine/tex/mipmap.go:56-70` 已有 `MipMode{MipUnknown,MipNearest,MipLinear}`（`Unknown` 为零值→映射成 Nearest，**没有"不用 mip"这一档**）。design.md D4 要的是 `render.MipmapMode{None,Nearest,Linear}`。`engine/tex` 在 `render` 之上（模块库），`render` 反向 import 它是分层倒置，所以不能直接复用；但两份并存就是收敛计划要清的重复面。→ 待拍板 D11。

## 3 接线形态（不改变现状的部分写清，避免顺手扩大战场）

- 采样档进命令：`ImageDrawCommand` 现成有 `MipmapLinear bool`/`Aniso`/`FilterKey()`；批 2 用**三档 mode** 替掉那个 bool（D4 全量切换），`EffectiveImageSamplerKey` 的 nearest/bicubic 掩码规则保持（bicubic 是手工抽 taps，`textured_quad_bicubic.wgsl:126` 钉 `textureSampleLevel(...,0.0)`，不受 mip 影响）。
- 绑定切到 `SamplerForFilter(cmd.FilterKey())`（三处：`render_session.go:4472,5295,5300`），死代码转正；同一批合并组必须同 key（已由 `:1173` 保证）。
- 纹理级数：`None` → 1 级（现状不变，零风险）；`Nearest/Linear` → `NumLevels(w,h)` 级，**CPU 产一次链、逐层上传同一份字节**（这是 D3 拍板的"双后端金图可逐位"前提）。缓存条目键必须带级数（否则同一张图被两种档请求会互相顶掉）→ 待拍板 D13。
- 幂等：链只在条目首次需要 mip 时生成，条目记 `mipmapGenerated` 位（对齐 `impeller/core/texture.cc:71-73`），同缓冲重上传不重复产链。
- 着色器不动：`textured_quad.wgsl:96` 与 persp `:98` 用导数 `textureSample`（LOD-capable，有多级时自动选层），这正是我们要的；bicubic 显式钉 level 0 保持。

## 4 风险与验证口径

- **存量金图必须零变化**：`None` 档保持单级 + `MipmapFilterModeNearest`，行为与今日逐位相同；只有显式请求 mip 的绘制才变。批 2 交付要给出「哪些窗/测试因真 mip 改了像素」的逐窗 A/B 归因，禁止靠放大容差蒙。
- **性能红线**：产链耗时、开 mip 后 128 条/64 MiB 预算实际能装多少张常用缩略图，必须实测并登记（`budget.go:135`），超线不合入。
- **跨平台标记一次写齐**：gles/rwgpu/webgpu-native/浏览器=可实现（本机可验 gles+native webgpu）；**metal 本机（Linux）不可验**，按标记纪律写"逻辑已备、darwin 交叉编译过、像素本机不可验"，不留空白格。
- 三证据 + 双后端 + 负向：负向至少两路——摘逐层上传（只传 level0 却把采样器设 Linear）应红、摘采样器接线（应红且证明公开档真驱动）；预算一路（摘按层求和应红）。

## 5 待拍板（下一条消息问）

- **D11** 枚举真源归位：`render.MipmapMode{None,Nearest,Linear}` 新增并把 `engine/tex.MipMode` 改成别名（消灭第二份，代价是 `Mip*` 常量改名、tex 侧 `MipUnknown` 语义并入）；还是两份并存（tex 那份是模块库自有口径）。
- **D12** 显存/预算口径：hal 从 `4/3` 上限改成**精确逐层求和**（对齐 Impeller `texture_descriptor.h` 的按 mip_count 计算，双端同源）；还是 render 侧也统一用 `4/3` 上限（简单但 NPOT 高估）。
- **D13** 缓存条目键：`(genID, 级数)` 双维键（同一图两档各一条，正确但可能两份显存）；还是"要 mip 就永远带链"（一条条目，`None` 也付 4/3 字节）。
- **D14** 采样器接线范围：三处绑定全切 `SamplerForFilter`（死代码转正）；还是先只在 mip 档非 None 时切（改动面更小，但留双路分支）。
- **D15** CPU 兜底那条链：本批一并改成"从同一份缓存链取、并修 `Release` 泄漏"（一份真源，风险是 CPU 侧像素变化）；还是只修泄漏、CPU 语义不动（存量金图零变化更稳）。
- **D16** `quad_cpu_test.go:369-377` 的 1% 门怎么重声明（数值 + 理由 + 覆盖范围），以及 `r1_residual_gpu_test.go:106-146` 反向钉改成正向钉的具体断言。
