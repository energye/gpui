# P4.3 ②③ 剩余根因：滤波源 RT 帧内跨阴影累积（2026-11-10）

> 性质：**新根因定位（文档，未动代码）**。用户指派的「GPU 层格式/premul 口径」假设
> 已被客观数据推翻；本文件记录真实根因、证据链、修法候选与影响面。
> 按「动手修门槛」两样齐后**停在「停下报告」等拍板**，确认前不改代码。

## 0 直话结论

1. **格式/premul 假设不成立。** 阴影颜色是纯黑 premul `(0,0,0,0.25)`，RGB=0；
   R↔B swizzle、premul/straight 转换对可见像素都没有影响——唯一能造成差异的是
   **alpha**。缓存 tile 在阴影带下缘实测 `191`（α=0.251），正好等于源 alpha 0.25，
   也就是 CPU 契约值（`render/context_misc_draw_test.go:189` 的 25% spot 断言）。
2. **不是「缓存偏亮」，是标准路把同一笔阴影叠加了多次。** 同一像素上标准路
   α 高达 0.79（值 53），是单次 0.25 的约 3 倍量级；且叠加次数随阴影顺序递减
   （A≈5–6、B≈5–6、C1≈3–4、D1≈1.8、D2=1）——正好等于「它之后还有几笔阴影」。
3. **真根因（通用层，非 P4.3 特例）**：`filterSrcView` 是 Context 级持久全画布
   滤波源 RT；`FlushAndFilterFromView` 把新剪影 flush 进去时，session 因
   `viewRendered(srcView)`（本帧已渲染过）选 **LoadOpLoad**，上一笔阴影的剪影被保留；
   于是每笔后续阴影的滤波输出都包含前面所有阴影的剪影，`PopLayer` 再把它们各自
   原位重画一遍 → 同一阴影在一帧内被重画 1..N 次。`renderedViews` 只在帧界清空，
   所以是**帧内累积、不跨帧**（1s 与 6s 标准路快照逐像素相同）。

## 1 格式契约逐项核查（结论：自洽，不是根因）

| 环节 | 契约 | 位置 |
|---|---|---|
| 离屏 RT（layer / 缓存 tile / filterSrc） | BGRA8Unorm | `render/internal/gpu/gpu_render_context.go:4139-4160` |
| 滤波图 ping-pong / 发布 RT | RGBA8Unorm | `render/internal/gpu/gpu_render_context.go:2752,2760` |
| `UploadRGBAToView` | RGBA→BGRA 只换 R/B，alpha 原样 | `render/internal/gpu/layer_gpu_io.go:75-135` |
| `ReadbackViewRGBA` / `...StraightRGBA` | 只做通道 swizzle，返回 premul | `render/internal/gpu/dual_tex_blend.go:1802,1917` |
| 纹理 quad shader | premul texel × opacity × tint，无 unpremul | `render/internal/gpu/shaders/textured_quad.wgsl` |
| 混合态 | premultiplied source-over | `render/internal/gpu/image_pipeline.go:306,346,399,487` |
| 层 pixmap → layer RT seed | 同一个 `UploadRGBAToView` 口 | `render/context_layer_blend.go:658-690` |

阴影是纯黑：R/B 交换、premul/straight 互换都只影响 RGB，不影响 alpha；premul 的
RGB 本来已是 0。**所以格式假设在数学上就不可能解释「整幅偏亮 1–12 级」。**
实测缓存 tile 的 alpha = 0.25 也证明上传/采样/混合链没有丢 alpha。

## 2 客观数据（零代码改动，全部来自遗留/新跑的单窗快照）

- 上会话遗留失败快照（13:37，缓存开启）vs golden：A/B 两卡差异 21135/48174 px，
  C/D 仅 2102/6160 px（max 5–6）——说明只有命中缓存的两笔变了，其余仍是标准路。
- A 卡本体下缘同一点：golden=53（α=0.792），缓存帧=191（α=0.251）。
- 按 `1-(1-0.25)^k` 拟合各阴影的等效叠加次数：A≈5.4–6、B≈5–6、C1≈3–4、
  D1≈1.8、D2≈1；与「每笔阴影在它之后的每个阴影层 Pop 时被重画一次」一致。
- 本会话 `-manual-seconds 1` 标准路单窗（6 presents，标准关闭）与 golden
  0.0000%，A 卡下缘同为 53；1s 与 6s 的快照逐像素一致 ⇒ **帧内累积**，非跨帧。
- native golden 与 `GPUI_BACKEND=go` golden 在这些点逐像素相同
  （53/100/154）⇒ 不是某个后端特例，是共享 session 路径。

## 3 代码链（file:line）

1. `render/context_misc_draw.go:222` `DrawShadow`：`PushLayerIsolated → 剪影 fill →
   ApplyBlur → PopLayer`。
2. `render/filter_ops_gpu.go:186,194`：layer 激活时走 path-1
   `ensureFilterSrcRT(w,h)` + `FlushAndFilterFromView(srcView, seed=nil, ...)`。
3. `render/filter_ops_gpu.go:933-956` `ensureFilterSrcRT`：同一 w,h 复用
   `c.filterSrcView`（全画布，跨阴影、跨帧持有，直到 resize/Close）。
4. `render/internal/gpu/filter_flush_coalesce.go:95-112`：`rc.Flush({View: srcView})`
   把 pending 剪影编码进 srcView。
5. `render/internal/gpu/render_session.go:7060-7067`（及 7158-7165、6432-6439、
   6914-6915）：`if s.viewRendered(view) { colorLoadOp = LoadOpLoad }` ⇒ 源 RT 保留
   上一笔内容。
6. `render/internal/gpu/render_session.go:1168,2209,2297`：`renderedViews` 只在
   BeginFrame/重建时清空 ⇒ 累积范围 = 一帧内的全部滤镜。
7. CPU 参照：`filter_ops_cpu.go` 的 blur 每次从 layer pixmap（干净剪影）起算，
   结果就是 0.25；`render/context_misc_draw_test.go:189` 把 25% 软阴影钉死。
   ⇒ 这是 **GPU/CPU 不一致**（P4.2 族），不是金图应有的样子。
8. 标准：Skia `saveLayer` / `SkImageFilter` 每次应用的源面都是隔离/清空的；
   持久 scratch 复用时必须先清（本项目 `FlushAndFilterFromView` 文档也写 `seed`
   非空才预置内容，隐含 seed=nil 应从透明开始）。

## 4 为什么上一版六种贴法全失败

缓存 tile 来自**单笔**阴影的滤波输出（干净 0.25），而标准路是 1..N 次叠加的累积结果；
差异是**叠加次数差**，不是贴法/UV/格式差。任何「单份贴一次」的方案都只能命中第一笔
阴影；把 tile 再包一层（#8）仍是一份。逐位对齐在不修根因前不可能成立。

## 5 修法候选与影响面（均未动代码，待拍板）

**A（推荐，治根、通用）**：每次滤镜应用前把 `filterSrcView` 置为「本 pass 必须 Clear」，
即 session 的 LoadOp 决策对滤镜源强制 Clear（或在 flush 前清该 view 的
rendered/colorContent 状态）。seed 非空时仍由 seed 上传定义内容。
- 效果：GPU 阴影回到 0.25，与 CPU 契约一致；`p41_drawshadow` 双后端金图会变
  → **必须重种 native+go**（与当前「金图 0.0000% 不重种」约束冲突，需用户明确批准）。
- 影响面：一帧内多次 `ApplyBlur`/滤镜的全部路径——DrawShadow、glow/effect、
  backdrop filter；属于正确性修复，与 P4.2 GPU/CPU 一致性同族。
- 风险控制：只清「滤镜源 scratch」，不动嵌套层/backdrop 的 LoadOp 语义；
  需要新增一个通用红测（同一 RT 连续两次滤镜，第二次不得含第一次内容）。

**B（不推荐）**：保留现状、②③ 不落地。per-shadow tile 无法复现「由后续阴影数量
决定的叠加次数」；除非改成整幅最终合成缓存（架构变更，失去紧致界裁剪收益）。

**C**：先只加探针（不改行为）在 filter 源 flush 前后 dump alpha 场，把 A 与
「layer RT 池复用」两个候选彻底分开，再决定。成本一轮，但结论更硬。

## 6 状态

- P4.3 仍 🟡；格式/premul 假设 ❌ 已推翻；新问题 = **滤波源 RT 帧内跨阴影累积**
  （通用层，非 P4.3 特例）。
- **2026-11-10 用户拍板 A**：修根因 + 重种金图。修法、红绿、重种数据、影响面
  A/B 与门禁见 `root-cause2-fix.md`；本文件保留定位过程。
- 定位阶段代码零改动；本会话只做了代码阅读 + 一次 1s 单窗标准关闭测量，
  runtime snapshot 已恢复原样。
