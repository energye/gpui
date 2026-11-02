# P3.6 批 2 方案：GPU mipmap（2026-11-02 拍板后落盘）

拍板记录（用户 2026-11-02）：**D11** render 出真源、`engine/tex` 改别名 / **D12** hal 与 render 两侧都按级数精确求和 / **D13** 缓存条目键带级数（评估消息已声明为正确性强制，用户未反对） / **D14** 采样器三处绑定一次全切、不留双路分支（同上） / **D15** **CPU 兜底同批统一到同一份链**（用户选了非推荐项：接受 CPU 侧像素变化，要求逐窗 A/B 归因） / **D16** `quad_cpu_test.go` 旧 1% 门不动、显式钉在 `MipmapNone`，另开 mip 专测各自声明容差。

## 1 公开 API（全量切换，不留 shim）

```go
// render（真源，新增）
type MipmapMode int
const (
    MipmapNone     MipmapMode = iota // 零值：单级纹理，只吃 level0 —— 与今日 GPU 行为逐位相同
    MipmapNearest                     // 采样器 MipmapFilterModeNearest
    MipmapLinear                      // 采样器 MipmapFilterModeLinear
)
// DrawImageOptions.UseMipmaps bool  →  Mipmap MipmapMode（context_image.go:162）
// QuadDrawOptions（m4_quadclass.go:35-51，今日无 mip 字段）+ 批绘 sprite 选项同挂 Mipmap（design.md D4）
// image.DrawParams.UseMipmaps bool  →  Mipmap image.MipmapMode（internal/image/draw.go:199）
```

- `engine/tex`：`type MipMode = render.MipmapMode`（别名，同 recording 对 `render.Matrix` 的先例），`MipUnknown/MipNearest/MipLinear` 三常量跟着指向真源。**tex 自有默认不变**：`Filter.Normalized()` 继续把零值映射成 `MipNearest`（模块库自己定默认，不借枚举表达），故今日 tex 侧行为逐位不动；`validMip` 由「只许 Nearest/Linear」放开为三值合法（`None` 从此可显式表达）；`ParseMipMode` 加 `"none"` 分支——实测该函数**全仓零调用者**（只有自身定义与注释），所以"解析失败与显式 none 折叠为零值"无消费者受影响。
- 反向依赖检查：`render` 不 import `engine/tex`（分层单向，AGENTS「分层单向调用」）。
- 计数影响按 `go/ast` 实测写 catalog（新增 1 类型 + 3 常量 + 字段类型变更；`Canvas`/`Context` 方法数不动），`scripts/apidoc` rc=0 为合入项。
- 录制面自动携带：`recording.DrawImageCommand.Opts` 是 `render.DrawImageOptions` 值拷贝（`recording/command.go:368-373`、`recorder_canvas_ops.go:214-220` 深拷、`backend_canvas.go:297,313` 回放），字段换类型即随命令走，无需新增命令。

## 2 一份链真源（CPU 与 GPU 同源，D15）

现状两处缺陷（实测）：`internal/image/draw.go:243` 每次缩小绘制重建整条链、且**从不 `Release()`**（归还口 `mipmap.go:156`，级别走 `GetFromDefault`/`pool.go:111` 的 8/bucket 池）→ 泄给 GC 并饿着池。

方案：新增 `render/internal/image/mip_cache.go`——按 **(源缓冲, 内容世代)** 缓存 `*MipmapChain`：

- 取：`ChainFor(src *ImageBuf, levels int) (*MipmapChain, bool)`，命中即复用，未命中才 `GenerateMipmaps`（幂等，对齐 `impeller/core/texture.cc:71-73` 的 `mipmapGenerated` 思路）。
- 退役：**复用批 1c 的 `AddGenIDWatcher`**——源缓冲换代或 Dispose 时，精确归还该世代链的各级到 `PutToDefault` 并摘注册。于是动图连播不会堆链（每帧一条、旧帧即收），与 GPU 条目同一套失效纪律。
- 预算：链的 CPU 字节计入 `render/budget.go` 新增域 `BudgetMipChainCache`（沿用 P2.4b 的 22 域声明表口径，条数/字节上限 + LRU 逐出），并把「同一时刻驻留的链数、字节峰值、产链耗时」实测进证据——不设上限就是假绿。
- CPU 兜底（`internal/image/draw.go`）改从 `ChainFor` 取级（不再自产），GPU 上传也从 `ChainFor` 取级 → **两侧吃同一份字节**，这是双后端金图可逐位的前提（D3 原意）。

## 3 GPU 侧

- 级数：`NumLevels = 1 + floor(log2(max(w,h)))`（真源与 `engine/tex/mipmap.go:214-226`、`internal/image/mipmap.go:42-43` 一致；**design.md §1.5 原记的 min 公式已在开工登记时订正**），逐级 `max(1, prev/2)`；`MipmapNone` → 1 级。
- 条目键：`imageCache.entries` 由 `genID` 单维改 **`(genID, levels)`**（`image_cache.go:78`）。同一张图被 `None` 与 `Linear` 两档请求时各得一条，互不顶掉；链字节相同的部分不重复占 CPU 侧（走同一 `ChainFor` 缓存），GPU 侧多占的那份由精确预算（下条）如实计入。
- 创建与上传：`image_cache.go:436`（CreateTexture）与 `:451`（CreateTextureView）的 `MipLevelCount` 由钉死 1 改为条目级数；上传由只写 level0（`:480`）改为逐级 `WriteTexture{MipLevel:i}`，字节取自 `ChainFor`。hal 口 `ImageCopyTexture.MipLevel` 四端已实现（gles `queue.go:191`、metal `queue.go:359,425`、rwgpu/浏览器透传），**本批不加 GPU 侧生成入口**（`glGenerateMipmap` 仍零调用，登记为后续优化项，与 D3 一致）。
- 采样器：三处绑定 `render_session.go:4472,5295,5300` 由 `SamplerFor(nearest)` 改 `SamplerForFilter(cmd.FilterKey())`（`image_pipeline.go:1415`，死代码转正；`filterSamplers` 按需建并缓存，`destroyPipeline:990-995` 已有释放）。`ImageSamplerKey.MipmapLinear bool` 改为 `Mip render.MipmapMode`，`EffectiveImageSamplerKey:1083` 的 nearest/bicubic 掩码规则与 `canMergeImageDraw:1173` 的 `FilterKey()` 比对同步改；`SetPerImageFilter:1103` 由命令构造处填（转正）。着色器不动：`textured_quad.wgsl:96`/persp `:98` 用导数 `textureSample`（有多级自动选层），bicubic `:126` 显式 `textureSampleLevel(...,0)` 保持。
- 预算（D12 精确求和）：`gpu/hal/vram_ledger.go:80-91` 的 `mips>1 → n*4/3` 换成逐级求和（它已经收 `mips` 形参，无需改签名）；render 侧 `image_cache.go:466/500/509` 与 `:285 evictIfNeeded` 用**同一个求和函数**（放 hal，render 调 hal 口，避免第二份公式），使 render 自计与 `hal.VramAdd` 真账相等；128 条/64 MiB 硬门（`budget.go:135`）下的实际容量实测登记（D6 原要求）。

## 4 验证（三证据 + 双后端 + 负向）

- 组 6 能力钉（新增）：级数公式（NPOT、1×N、1×1 退化、2 的幂）；`ChainFor` 幂等（同世代重复取不重产、产链次数计数）；退役精确回收（换代/Dispose 后链数与池驻留归位）；逐级上传字节 == CPU 链字节（逐层比对，不是只比 level0）；三档采样单调（同一缩样序列 `None`→`Nearest`→`Linear` 方差不增、`Nearest` 与今日逐位相同）；预算对账（render 自计 == hal 真账）；`MipmapNone` 路径**零行为变化**（单级、采样器仍 `MipmapFilterModeNearest`）。
- D15 带来的 CPU 侧像素变化：逐窗 A/B 归因（哪些窗、多少像素、方向），存量金图**不重出**；变了的必须显式声明并写明"预滤波 mipmap 取代双线性"这一理由，禁止靠放大容差蒙。
- 旧门保持：`quad_cpu_test.go:369-377` 用例显式钉 `MipmapNone`（1% 门与 nearest 逐位门继续成立）；`r1_residual_gpu_test.go:106-146` 由反向钉改成正向钉（`MipmapLinear` 必须走 GPU 且真的用了多级：断言 GPUOps、级数、以及缩样结果与 `None` 档不同）。
- 负向至少四路（红→还原绿）：摘逐级上传（只传 level0 却设 Linear）→ 单调钉红；摘采样器接线（回 `SamplerFor`）→ 证明公开档真驱动的钉红；摘按级求和 → 预算对账钉红；摘 `ChainFor` 幂等/退役 → 产链次数与池驻留钉红。
- 性能红线（合入项）：产链耗时（按尺寸一档一档量）、开 mip 后 64 MiB/128 条能装几张常用缩略图、缩样滚动帧率；超线不合入。
- 跨平台一次写齐：gles / rwgpu / webgpu-native / 浏览器 = 可实现（本机验 gles + native webgpu 像素）；**metal 本机不可验**，按标记纪律写"逻辑已备（`metal/device.go:236-239`、`queue.go:359,425`）、darwin 交叉编译过、像素本机不可验"，不留空白格。
- 门禁全套沿用批 1a/1b/1c：apidoc、vet 触及包、定向、`-race -count=2`（新增链缓存跨线程访问必须进 race 组）、conformance 双后端、五目标×{默认,nogpu}、逐文件只减不增（新增红 clean-HEAD A/B 归因）、存量金图零变化或逐窗归因。

## 5 实施切分（每步可编译可测，禁止半新半旧）

1. 真源枚举 + tex 别名 + 公开口字段切换（含录制/回放/全部调用点与两处旧钉改语义）→ 编译与既有测试全绿（此时 mip 档仍不被消费，行为零变化）。
2. 链缓存 `ChainFor` + 预算域 + 退役回收 + `draw.go` 改取源（CPU 侧统一，含 D15 的像素影响与逐窗 A/B）。
3. 级数进条目键 + 多级创建 + 逐级上传 + 精确预算。
4. 采样器三处全切 + `ImageSamplerKey.Mip` 换类型 + `SetPerImageFilter` 转正。
5. 组 6 钉 + 负向四路 + 性能实测 + 文档收口（catalog §0/§3.5/§13、台账、`ENGINE_UI_WIDGET_RENDER.md` §10、design.md 状态、`batch2.md` 证据）+ 全套门禁。

每步独立可验证；批 2 一次交付（不再拆批，但提交前逐条过门禁并等你确认）。
