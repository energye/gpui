# 渲染层审计修复方案（RENDER_AUDIT_FIX_PLAN）

日期：2026-10-25 ｜ 来源：四域源码审计（GPU 后端/渲染核心/文本/性能，结论见会话审计）＋ 逐项源码复核
性质：**修复方案文档（未动手）**。每一项都满足「动手修门槛」两件套：真实问题（源码 file:line 复核钉死）＋ 对齐标准方案（Skia/Flutter/HarfBuzz/OpenGL ES 规范的具体出处）。

> 纪律：每项修复前先落红灯测试（值级或像素级，证明洞存在），修后转绿 + 所属域回归 + 相关真窗 Golden 复跑；基线漂移须归因后经批准重出。本文档只登记方案，不改代码。

## 0. 总览与优先级

| # | 问题 | 域 | 严重度 | 标准依据 | 预估工作量 |
|---|---|---|---|---|---|
| A1 | render 层默认文本路径 cmap-only 不整形 | 文本 | 高（正确性） | Skia SkShaper / Flutter txt | 中 |
| A2 | RTL + MultiFace 整段丢字 | 文本 | 高（正确性） | Flutter txt run itemize | 中 |
| A3 | GlyphMaskKey 缺 hinting 位（键互踩） | 文本 | 高（正确性） | Skia SkDescriptor 全状态键 | 小 |
| B1 | 旋转 CTM 下 ClipRect 取两角 AABB | 几何 | 中（正确性） | Skia 精确四边形裁剪 | 小 |
| B2 | dash 相位每子路径重置 + 缩放只放大不缩小 | 几何 | 中（正确性） | WHATWG canvas / Skia SkDashPath | 小 |
| C1 | AA 矩形渐变 ColorAt 场每帧 CPU 重建 | 性能 | 中（已登记 §6 性能子项） | Skia ramp 表 + shader 求值 | 中 |
| C2 | 彩色字形图集无淘汰 + 前景色进键 | 资源 | 中 | Skia GrStrikeCache LRU / 绘制期着色 | 小-中 |
| D1 | Windows/WGL 上传缺 UNPACK_ROW_LENGTH | 后端 | 中（Windows 正确性） | OpenGL ES 3.0 规范 | 小 |
| D2 | HbShaper 无锁改共享 hbFont scale | 线程 | 高（数据竞争候选） | HarfBuzz 对象线程语义 | 小 |

---

## A1. render 层默认文本路径 cmap-only（不整形）

### 现象
直接调 `render.Context` 的 `DrawString`（以及 TextMode 位图/MSDF 路径）时，文本按 cmap 逐字符排：**无 GSUB/GPOS/kern/连字**——Arabic/Indic 呈孤立字形，同一字符串在不同路径下形状不一致。UI 组件产线不受影响（`ui/rendering` 走 `text.Shape`→`DrawShapedGlyphs`，`ui/rendering/text.go:1014,1078`），但 2.5D/视频等直接调 render 的消费方拿到的是未整形文本。

### 源码证据（逐行复核）
- GPU 主路：`render/internal/gpu/gpu_render_context.go:1778,1814,1852`（DrawString→`engine.LayoutText/LayoutTextAliased`）
- 引擎默认：`render/internal/gpu/glyph_mask_engine.go:221,228,286`（`shaped := text.LayoutGlyphs(face, s)`）
- `LayoutGlyphs` 的实现是 cmap 迭代：`render/text/shape_result_cache.go:405-419`（`for glyph := range face.Glyphs(s)`）→ `render/text/face.go:245` `iterGlyphs`（cmap+hmtx，无 HarfBuzz）
- MSDF 路径同样：`render/internal/gpu/gpu_text.go:196`（`for glyph := range face.Glyphs(s)`）
- 对照：vector 路径 `render/text_outline.go:132` 与 `LayoutShapedGlyphs`（`glyph_mask_engine.go:290` 起）走 `text.Shape`（HarfBuzz）——四路里只有这条整形。

### 标准依据
- **Skia**：`SkCanvas` 不做字符级 cmap 排版——现代 Skia 只接受 glyph ID（SkTextBlob/SkGlyphRun），字符串→glyph 一律经 `SkShaper`（HarfBuzz+ICU）整形；legacy `SkCanvas::drawText` 已废弃。Skia 的口径：**绘制端要么拿到整形结果，要么自己调 Shaper，不存在"画字符串但跳过整形"的默认路径**。
- **Flutter**：engine `txt/`（SkParagraph）所有文本先 `shapeFull`（HarfBuzz）再进 raster，无 cmap-only 分支。

### 对齐方案
1. `GlyphMaskEngine.LayoutText/LayoutTextAliased` 把 `text.LayoutGlyphs(face, s)` 换成 `text.Shape(s, face)`（HarfBuzz，已有缓存 `shape_result_cache`，键已含方向/变体/语言）；`LayoutGlyphs` 降级为「shaper 不可用（无 HB 构建/face 不支持）」的显式回退，并在回退时打点计数。
   **前置依赖（A2 先行的原因）**：`text.Shape` 对 MultiFace 返回 nil（`shaper_hb.go:70-72`，`Source()==nil`）。A1 落地时必须同时给默认路径接 **per-face 拆段**（复用 A2 的 `mf.Runs` 拆段机具，抽成 text 包共享助手），否则 MultiFace 消费方从「cmap 可画」退化为整段空白——那是引入回归，不是修洞。
2. MSDF 路径 `gpu_text.go:196` 同批切换 `text.Shape`。
3. CPU 位图路径（`text.Draw*` 派发）同口径。
4. 高频 HUD 字符串（`IsHighChurnLabel`）不跳过整形（Latin 数字整形开销可忽略，缓存兜底），只保留不进 layout 模板缓存的现状。
5. 接口不动（`DrawString` 签名不变），行为对齐「SkShaper 一站式」。

### 验证口径
- 红灯：值级测试——Arabic 连写字串（如 "لمّا"）经 `LayoutText` 产出的 GID 序列 ≠ cmap 序列（修前相同=cmap-only 实证；修后不同=整形生效）；kern 对（"AV"）宽度对比 `text.Measure`。
- 绿后回归：`render/internal/gpu` 文本测试逐文件 + `ui_render_base_text/textlow/textspacing/textstroke` 四窗 Golden（Latin 部分应零差——shaped Latin 与 cmap Latin 同形；若 AA 边有差须归因）。

### 风险
Latin/数字路径字形序列改变（ kern 进来）会使部分既有 Golden 的文本宽度变化 → 属于「修正确」的基线漂移，按纪律归因重出（预期只影响含 kerning 对的文本窗）。

---

## A2. RTL + MultiFace 整段丢字

### 现象
RTL 方向的段落遇到 MultiFace（多字体回退链）时**整段没有字形**（空白）。

### 源码证据
- `render/text/layout.go:307`：`if mf, ok := face.(*MultiFace); ok && seg.Direction != DirectionRTL` ——按 face 拆段的逻辑**显式跳过 RTL**；
- `render/text/shaper_hb.go:70-72`：`source := face.Source(); if source == nil { return nil }` ——MultiFace 的 `Source()` 为 nil，HarfBuzz 整形直接返回空 → 整段 0 字形。

### 标准依据
**Flutter txt/ParagraphImpl::shapeFull**：按 script **和** 字体回退双向 itemize 成 runs（`FontCollection::matchForFallback`），**每个 run 用解析出的单字体整形**，LTR/RTL 同一套规则，不存在"RTL 不拆字体"的分支。

### 对齐方案
1. 去掉 `Direction != RTL` 守卫：RTL 段同样走 `mf.Runs(seg.Text)` 拆成 per-face 连续段（LTR 拆段机具已在 `layout.go:308-320` 现成）。
2. 每段用 `facePart.face`（单 face，`Source()` 非 nil）整形，方向传 RTL。
3. 段序：RTL 的 parts 在行内按**视觉序**（与 LTR 相反）拼接；cluster 偏移已由 `seg.Start+part.start` 支持。
4. `MultiFace.Source()` 保持 nil 语义（它不是可整形字体），但在 `HbShaper.Shape` 入口对 MultiFace 显式报错而不是静默 nil（防再次静默丢字）。

### 验证口径
- 红灯：`مرحبا` + emoji 混排（emoji 需要第二 face）经 RTL 段整形，修前 0 字形、修后非零且含两 face 的 GID；`TestLayoutText_MultiFace` 扩 RTL 用例。
- 回归：`ui_render_base_textspacing`（B 卡 RTL 标尺）Golden；m5_anycase A 区。

---

## A3. GlyphMaskKey 缺 hinting 位（缓存互踩）

### 现象
字形掩码的 alpha 内容随 hinting 变（提取轮廓时传入），但缓存键不含 hinting → 同字体同字号不同 hinting 配置的两个 face **共用同一掩码条目**，后到的配置读 到先到的掩码。

### 源码证据
- 键结构：`render/text/glyph_mask_atlas.go:27-46`——`{FontID, GlyphID, SizeQ4, SubpixelXQ2, SubpixelYQ2, Flags(Aliased/LCD/BGR/Bold)}`，**无 hint 位**；
- 掩码内容吃 hinting：`render/internal/gpu/glyph_mask_engine.go:837` `ExtractOutlineHintedVar(parsed, gid, size, hinting, variations)`（非变体走 `glyph_outline.go:326 ExtractOutlineHinted(parsed, gid, size, hinting)`）；
- 自相矛盾的证据：引擎自己的 layout 模板键**含** hint 位（`glyph_mask_engine.go:92` 注释 `flags // lcd/aliased/hint bits`）——同引擎两套键一套记得一套忘。
- 触发面：UI 把某些 face 钉 `HintingVertical`，默认 face 是 Full——同源同字号两配置并存即互踩。

### 标准依据
**Skia 字形缓存键 = SkDescriptor**：把**所有影响 scaler 输出的状态**（字号、矩阵、flags、**hinting**、LCD 序）全部纳入描述符，"任何影响生成掩码的状态都必须进键" 是 SkGlyphCache 的铁律（skia/src/core/SkDescriptor.h / SkScalerContext）。

### 对齐方案
1. `GlyphMaskKey.Flags` 增加 2 位 hinting（None/Vertical/Full 三态，`text.Hinting` 就 3 值；Flags 目前用 4 位、宽度足够）。
2. `MakeGlyphMaskKey/…Bucketed` 加 hinting 参数；引擎侧构键点（`glyph_mask_engine.go:609-622` 等）传入已算好的 `hinting`。
3. 布局模板键与掩码键从此同口径。

### 验证口径
- 红灯：同 fontID+GID+size，hinting=Full 与 None 各光栅一次，断言得到**不同**的图集条目（修前同一条目）；`glyph_mask_hinting_test.go` 扩用例。
- 回归：`glyphmask_lcd_mix_test`、三文本窗 Golden。

---

## B1. 旋转 CTM 下 ClipRect 取两角 AABB 当裁剪框

### 现象
`ClipRectOp`/`ClipRoundRect` 在带旋转/斜切的 CTM 下，把变换后的**两个对角点**取 min/max 当轴对齐矩形——实际是外接框，**该裁掉的四个角区域没裁掉**（多画）。

### 源码证据
- `render/clip_op.go:36-41`：`p1 := tm.TransformPoint(...); p2 := tm.TransformPoint(...); devX := min(p1.X,p2.X) …` ——只变两角；
- `render/context_clip.go:96-118`（ClipRoundRect 同构问题）。

### 标准依据
**Skia**：`SkCanvas::clipRect` 把 rect 经 CTM 变换后按**精确四边形**裁剪（raster 端解析扫变换四边形的边），绝不取外接框；`clipRRect` 同理（变换后作 path 裁剪）。

### 对齐方案
1. 检测 `totalMatrix` 非轴对齐（`B != 0 || C != 0`，引擎已有同款判定 `glyph_mask_engine.go:1248`）：
   - `ClipRectOp`：把四角变换成 `Path`（MoveTo+3×LineTo+Close），走**既有路径裁剪**（clipStack 支持 path + AA + fillRule——`internal/clip` 已具备）。
   - `ClipRoundRect`：变换非轴对齐时同样降级为 path（圆角矩形轮廓 path）。
2. 轴对齐快路径保持不变（scissor/矩形栈零成本）。
3. GPU 深度裁剪侧（`gpuClipPath`）沿用 path 管线，无需新机制。

### 验证口径
- 红灯：45° 旋转 CTM 下 ClipRect(100×100)，角落点（变换后落在原矩形外、AABB 内）修前画出、修后不画；`render/clip_ops_semantics_test.go` 加用例（复用既有 oracle 框架）。
- 回归：`ui_render_base_cliprrect`/`clipop` 两窗 Golden。

---

## B2. dash 相位每子路径重置（+ 缩放只放大不缩小）

### 现象
多轮廓路径的虚线，每个子路径都从**同一个初始相位**重新起跑——轮廓间虚线节距错位；另外虚线周期只在放大 CTM（scale>1）下按比例缩放，缩小 CTM 下周期偏长。

### 源码证据
- `render/software_stroke.go:200-203`（MoveTo 分支）：注释明写 "Reset pattern state for new subpath"，`patternIdx, patternPos, inDash = dashStateAtOffset(pattern, offset)` 重置回原始 offset；
- `render/software_stroke.go:40-42`：`if transformScale > 1.0 { dash = dash.Scale(transformScale) }` ——缩小不缩放；对照 T.03 路径 `stroke_user_expand.go:53-64` 是两边都缩（口径不一）。

### 标准依据
- **WHATWG HTML canvas 规范**（lineDashOffset/stroke）：dash 模式**跨子路径连续**（"the dash pattern continues across sub-paths"）；
- **Skia SkDashPath**：同一 path 的各 contour 相位连续推进；dash 长度按**设备空间**（CTM 全量）缩放。

### 对齐方案
1. MoveTo 分支不再重置：`patternIdx/patternPos/inDash` 沿用上一子路径结束时的状态（即删掉那次 `dashStateAtOffset` 重置；`offset` 只在路径起点应用一次）。
2. dash 缩放对齐 T.03 口径：`transformScale != 1.0` 一律 `dash.Scale(transformScale)`（双向）。
3. GPU 侧 stroke 展开若独立实现 dash（`internal/stroke/expander`），核对同口径。

### 验证口径
- 红灯：两段共线等距子路径（间隔 10px）虚线 period=20，修后第二段相位应接续（+10 即半个 period），修前重置——点采样断言 on/off 相位；
- 回归：`pathshape` 窗 C/D 卡（Discrete 不受影响）、`render/pathops_subpath_test.go`。

---

## C1. AA 矩形渐变 ColorAt 场每帧 CPU 重建（已登记 §6 性能子项，本方案细化）

### 现象
带 AA 的矩形面 + 自定义/扫掠/径向渐变走「CPU 逐像素采样 ColorAt 场」路径：**每次调用** `make` ≤512² 的 RGBA 数组并逐像素调 `ColorAt`（≤26 万次/帧），动画层每帧重录即每帧中招（~1-3ms/帧 + 1MB 分配/帧）。genID 只省 GPU 重复上传，不省 CPU 采样。

### 源码证据
- `render/internal/gpu/brush_fill.go:158-318`（`fillColorAtFieldMaskedGPU`）：`:229` `field := make([]byte, nw*nh*4)` + 逐像素 `colorAt(...)` 循环；`:305` genID 组合只用于 `QueueImageDraw` 跳过重复上传；
- 调用点：`:476`（linear）、`:773,776,950,1060`（radial/sweep 族）——AA 矩形优先走此路（既有 §6 登记的 `brush_native` 场路径的 GPU 化后裔，采样仍在 CPU）。

### 标准依据
- **Skia**：渐变从不建 CPU 颜色场——`SkGradientShader` 把色标预处理成**一份缓存的区间表**（fCached，随 shader 生命周期），像素值在 shader/GPU 端求值（GPU：1D ramp 纹理 + 片元着色器按坐标采样/数学）；
- **Impeller/Flutter**：渐变参数进 uniform，片元着色器求值，CPU 零逐像素工作。

### 对齐方案（两刀，按标准终态设计）
1. **终态**：AA 矩形 sweep/radial/自定义 ColorAt 全面改走**既有 1D ramp + textured cover 管线**（圆/非矩形已经在用，`brush_fill` 的 textured cover 路径现成）——色标进 ramp 纹理一次、片元着色器按 UV 求值，CPU 场路径仅保留给「无法表达为 ramp 的真自定义 brush」（用户闭包 ColorAt），且该路径加 `render.Canvas` 级显式标注（性能口径如实）。
2. **过渡刀（同批可落）**：`fillColorAtFieldMaskedGPU` 的场按 `(brush genID, 几何 genID, 场分辨率)` 记忆化——同帧/同 brush 复用上一场，只重算 mask；genID 失效（色标变）才重采样。动画期 brush 不变时 CPU 成本归零。
3. 落地时同步把覆盖总账 §6 的对应性能子项标「已修/部分」。

### 验证口径
- 基准：`go test -bench` 对 AA 矩形 sweep 填充（新增 bench：固定 brush 重复 fill 100 帧）修前/修后 CPU 时间与 allocs（预期 allocs/帧 1MB→0、时间 1-3ms→<50µs 过渡刀 / →0 终态）；
- 正确性：`ui_render_base_sweepgradient` B 卡像素探针 + Golden（同参数 ColorAt oracle 12/255 容差既有）不回归。

---

## C2. 彩色字形图集无淘汰 + 前景色进键

### 现象
彩色字形（CBDT/COLR）图集写满 maxPages 后**永久** `ErrColorAtlasFull`→该字形永远回退 CPU 路径；且**前景色进缓存键**——每种文字颜色都占一份图集空间，加速耗尽。

### 源码证据
- `render/internal/gpu/glyph_color_atlas.go:22-24,107`：满页返回 `ErrColorAtlasFull`，entries map 无任何 evict/LRU（grep 零命中）；
- 键：`ColorGlyphKey{FontID, GlyphID, PPEM, Palette, FG}`（`:27-33`），`FG color.RGBA` 进键——引擎构键点 `glyph_color_engine.go:111`。

### 标准依据
- **Skia GrStrikeCache/GrAtlasProvider**：图集满时按 **LRU 逐出**最久未用字形并回收页空间（purge when full），绝不永久拒绝；
- **着色位置**：Skia 的 glyph 掩码/彩色位图**不带画笔前景色**——tint/currentColor 在**绘制期**用 SkPaint/着色器应用，缓存键只有影响位图内容的状态（COLR 的 currentColor 经 CPAL 调色板索引表达，不把任意 FG 塞进键）。

### 对齐方案
1. **LRU 逐出**：`ColorGlyphAtlas` 记 lastUse 戳；满页时先逐出最久未用条目（整页回收重建或条目级 shelf 重排——对齐 Skia「页级回收」的最小实现：淘汰最冷整页并从该页迁出仍热的条目）。
2. **FG 出键**：核查 `FG` 在光栅里的实际用途——CBDT 位图自带颜色（FG 应无影响，直接删）；COLR 若用 FG 当 currentColor 调色板，改为「中性调色板光栅 + 绘制期 tint」（着色器乘 FG），`Palette` 字段保留。若确有 FG 影响像素的用例，量化 FG（如 4bit/通道）缩小键空间并登记限界。
3. 满页行为改为「逐出后重试一次，仍满才回退 CPU」。

### 验证口径
- 红灯：灌满图集（>maxPages×条目）后请求新字形，修前 ErrColorAtlasFull、修后成功且最冷条目被逐出（Get 旧冷键 miss）；
- FG：同字形两个前景色绘制，修后共享图集条目且两色均正确（tint 后像素断言）；
- 回归：`ui_render_base_emoji` A/B/D 卡 Golden。

---

## D1. Windows/WGL 纹理上传缺 UNPACK_ROW_LENGTH

### 现象
带行距（BytesPerRow > 紧宽）的上传（如字形图集脏矩形）在 Windows 端会**逐行错位**——Linux 端设了 `UNPACK_ROW_LENGTH`，Windows 端没设。

### 源码证据
- Windows：`gpu/gwgpu/gles/queue.go:165-184` 只设 `UNPACK_ALIGNMENT`（R8 时），**无 UNPACK_ROW_LENGTH**（注释自认 "Windows 真机待复验"）；
- Linux 对照：`gpu/gwgpu/gles/queue_linux.go:178-191` 按 `layout.BytesPerRow/pixelBytes` 设 `UNPACK_ROW_LENGTH` 并 defer 复位。

### 标准依据
**OpenGL ES 3.0 / OpenGL 规范**（glPixelStorei）：`GL_UNPACK_ROW_LENGTH` 指定源图像行距（像素），未设时行距=图像宽——padded 行必须显式告知，双端一致是规范要求，不是平台风格差异。

### 对齐方案
把 Linux 的行距块原样移植到 `queue.go`（含 `defer` 复位与 pixelBytes 换算）；顺手核查该文件其余 `TexSubImage2D`/`TexImage2D` 调用点同口径。

### 验证口径
- 单测：构造 BytesPerRow=width+pad 的上传，读回断言像素无错位（无 Windows 真机时以 GL 语义单测 + 代码对齐为准，真机验证登记为验收项）；
- 回归：`gles` 包逐文件测试。

---

## D2. HbShaper 无锁改共享 hbFont 的 scale（数据竞争候选）

### 现象
`HbShaper.Shape` 每次整形都改**共享缓存**的 hbFont 实例的 `XScale/YScale`（按当前字号），且不持锁——两个 goroutine 用**同字体不同字号**并发整形时，互相覆盖 scale，字形位置/advance 错乱。

### 源码证据
- 共享缓存：`render/text/shaper_hb.go:134-141` `getOrCreateHbFont` 按 `(source, variations)` 缓存 `*hbFont`；
- 无锁突变：`shaper_hb.go:82-84` `hbf.hbf.XScale = int32(...); hbf.hbf.YScale = hbf.hbf.XScale`——发生在 `Shape`（`:66`）体内，**不在** `s.mu` 临界区（锁只用于 ClearCache/RemoveSource/字体表读写 `:116-141`）。

### 标准依据
**HarfBuzz**：`hb_font_t` 单个调用内部线程安全，但**修改属性（set scale）与使用是两次调用**，并发改属性行为未定义——需要外部同步或按状态分实例（go-text/typesetting 的做法：每 (font, size) 独立 font 实例或整体加锁）。

### 对齐方案（二选一，推荐 1）
1. **锁扩大**：`Shape` 全程持 `s.mu`（整形已被 `globalShapeResultCache` 前置缓存挡住，临界区冲突低；改动最小）。
2. **实例分键**：`hbFontKey` 加 size，按 `(source, variations, size)` 各建实例，无共享突变（内存略增，无锁竞争）。

### 验证口径
- 红灯：并发整形测试——两个 goroutine 同字体分别 12px/48px 循环 `Shape`，`-race` 修前报 DATA RACE（或断言结果串号），修后 0；
- 回归：`render/text` 逐文件 + `-race`；`ui_text_m5_anycase` Golden。

---

## 0.1 第一批修复落地记录（2026-10-25）

| 项 | 状态 | 证据 |
|---|---|---|
| A3 | ✅ 已修 | 红灯：`TestGlyphMaskHintingSeparatesAtlasEntries` 修前 entries 停在 1（碰撞实锤）→ 修复后分离 → 变异反证（hint 位清零再红）。键位 Flags 4-5 编码 hinting，构造器三件套加参全量切换，引擎手工旗标改 `\|=` 防冲。回归：render/text + render/internal/gpu Mask/Hint/LCD 族绿。真窗：`ui_text_m5_anycase` 两连 0.0000%；`ui_render_base_textlow` 出现 0.0883% **纠错性漂移**（混 hinting 配置修前共享掩码，Full/None/Vertical 轮廓实测互异 + 两跑逐位一致 + 抓屏 0.0000%）——**经用户批准重出**，两连 0.0000%。连带：`ui_render_base_textspacing` 同机制 0.5958% 漂移，同批准归因重出，两连 0.0000%（`-race` 档） |
| D2 | ✅ 已修 | 红灯：`TestHbShaperConcurrentSizes`（新文件 `shaper_hb_race_test.go`）`-race` 修前 11 条 DATA RACE 命中 `shaper_hb.go:82` → `Shape` 全程持 `s.mu`、`getOrCreateHbFont` 改持锁前置（`system_font.go` 预热线同批适配）→ 0 条且并发结果与单线程基准逐位一致 → 变异反证（撤锁再红）。回归：render/text 全包 + Shape/Layout/Wrap 族 `-race` 绿。真窗：`textspacing -race` 两连 0.0000%、`m5_anycase -race` OK |
| D1 | ✅ 已修（代码级） | `gles/queue.go` 补 `UNPACK_ROW_LENGTH`（与 `queue_linux.go` 逐行同构，含 defer 复位与 pixelBytes 换算）；`GOOS=windows` 交叉编译过 + Linux `gles` 包全绿。**Windows 真机验收登记待做**（无真机不假绿） |
| B2 | ✅ 已修 | 红灯：`TestDashPhaseContinuesAcrossSubpaths`（新文件 `software_stroke_dash_test.go`）修前第二子路径首 dash=[60,80]（每子路径重置相位）→ oracle [60,70]（WHATWG 跨子路径连续）→ 修复后绿 → 变异反证（恢复重置再红）。附带：dash 缩放 `>1.0` 改 `!=1.0` 双向（对齐 T.03）。回归：render 的 Stroke/Dash/PathEffects 族绿。真窗：`ui_render_base_strokepreset` 两连 0.0000%（单轮廓虚线窗，无基线漂移符合预期） |

遗留观察（已定性为存量）：`render/text` 全包单跑默认 600s 超时系 B1 全字集扫描聚合慢（`zz_b1_ttf_scan_test.go`，2026-09-30 存量：WqyFull 216s + Indic 173s + Latin 58s + Myanmar 28s，B1 族单独 ~475s，全包需 >600s）；单跑 `-timeout=900s` 全 PASS（B1 过滤组 `ok 702.785s`），逐文件/过滤跑绿，非本批回归。

## 0.2 第二批进度（2026-10-25）

- **A2 ✅ 已修（已提交）**：`layout.go` 去掉 RTL 拆段守卫 + RTL parts 视觉序反转。红灯（RTL+MultiFace 0 字形/无行）→绿→变异反证→`Layout|Wrap|Segment|Multi|Shape` 族回归绿（含 `-race`）→真窗 `textspacing`/`m5_anycase` 两连 0.0000% 零漂移。
- **A1 ✅ 已修（提交待落）**：`text.ShapeForDraw`（整形优先+MultiFace 拆段+cmap 显式回退计数）落地，引擎 `LayoutText/LayoutTextAliased` 与 MSDF 路径已切换；fi 连字红灯→绿→变异反证过；`render/internal/gpu` 文本族回归绿。四步隔离实证定性「textstroke 17.15% 屏差」为 **HEAD 存量**（干净 HEAD 逐位同值；A1 还原后仍复现）——与 A1 无关，见 §0.3 跨线归位。mask 路位置对拍 text.Shape 一致（quad 起点=整形 X+bounds）；基线处置经用户批准：`base_text` 7.19% / `textlow` 0.02% 为 kerning 重排漂移，重出后各两连 0.0000%；`textspacing`/`m5_anycase`/`recording`/`miscdraw` 零漂移两连过。CPU 位图路径（`text/draw.go` drawGlyphs hinted-advance 机具）登记为 A1 后续子项。

## 0.3 存量登记（2026-10-25，跨线归位）

- **`ui_render_base_textstroke` 抓屏 vs 快照 17.1543% 恒定差（HEAD 存量，非 A1）**：四步隔离实证——A1 工作区复现 → 临时还原 LayoutGlyphs 复现 → **干净 HEAD（3524f68e）逐位同值 17.1543%** → 背景/前景采样点全同但文字行错位 ±11px 级。金图 0.0000%（快照重绘路自洽），差异只在「屏上活帧」——指向抓屏时机抓到合成器过渡帧或窗内容局部刷新未完成（该窗 C 卡 TextPath 粉/橙元素屏上错位）。归 presentation/抓屏域跟进（与 R10 斜率同类存量），不归 A1；A1 按自身验收口径继续。

## 0.4 第三批进度（2026-10-25）

- **B1 ✅ 已修**：`ClipRectOp`/`ClipRoundRect` 非轴对齐 CTM（`tm.B|C != 0`）改走 `deviceQuadClipPath`（四角精确四边形→既有 path 裁剪，镜像 ClipPathOp 的设备空间语义），轴对齐快路径不变。红灯（45° CTM 角点 (110,110) 泄漏红）→绿→变异反证（禁用分支→中心都不裁剪，双证）。回归：`Clip|Mask` 过滤族红项与干净 HEAD 一致（存量）；真窗 `cliprrect`/`clipop` 各两连 0.0000% 零漂移。连带修正 A3 过期的 `TestGlyphMaskKey_AliasedFlag` 期望（hint 位在 Flags 4-5 是新语义）。

- **C2 ✅ 已修**：① 图集页级 LRU——满时逐最冷页重试一次（`evictColdestPageLocked`，lastUse 戳随 Get/Put 刷新，页重索引），`ErrColorAtlasFull` 从「永久失败」变「可回收」；红灯（填满后 Put 永久失败）→绿→变异反证（禁逐出→红）。② CBDT 键出 FG——位图不吃前景色（`rasterizeColorBitmap` 无 fg 参数），同字形每种文字色一份条目纯属碎片化；COLR 保留 FG（currentColor 烘进光栅）。连带发现并登记：上游 `ColorRasterCache`（`color_raster.go:53`）键同样含 fg 且对 CBDT 也进键（同类碎片化 + 满 maxSize 随机驱逐），属后续子项。真窗 `emoji` 两连 0.0000%（0.0548% legend 文本 A1 重排漂移，同批 A1 批准归因）。

- **C1 ✅ 过渡刀已修（终态 ramp 化留后续）**：`fillColorAtFieldMaskedGPU` 的 ColorAt 场按 `(brush seed, 场几何, 分辨率)` 记忆化（`brush_field_cache.go` fieldCache，容量 8 drop-oldest，挂 GPUShared），稳态 brush 复用已建场、覆盖仍每帧从 mask 现算。**基准（同 brush 连填，512×320）**：修前 58.8ms/op、1.56MB/op、140 allocs/op → 修后 **5.9ms/op（10×）、1.23MB/op**（剩余分配在 staging 上传/覆盖路径，登记后续）；变异反证（禁 memo→57.7ms）过。真窗：`sweepgradient`（B 卡 ColorAt oracle）与 `geometry` 各两连 0.0000% 零漂移；`render/internal/gpu` 全包绿。终态（AA 矩形 sweep/radial 全面 ramp 化+shader 求值，CPU 场仅留真自定义 brush）按 §C1.1 留独立刀。

## 3. 后续开线清单（2026-10-25 定版，一期一议）

9 项审计修复全部收口后的遗留线，按建议顺序排；每条线的源码位置、方案与验收口径都已在前文对应章节。

| # | 线 | 范围 | 依据章节 | 验收口径 | 预估 |
|---|---|---|---|---|---|
| 1 | **C1 终态：AA 矩形渐变 ramp 化** | AA 矩形 sweep/radial 全面走 1D ramp 纹理 + 片元/调制 shader 求值（复用圆卡既有 textured-cover 管线，maskR8Modulate 扩 ramp 采样），CPU 场路径仅留真自定义 ColorAt brush 且加显式标注；顺带消掉过渡刀剩余的 1.23MB/op staging 分配 | §C1 方案第 1 条 | bench（过渡刀同款）稳态 ≈0 分配/op；`sweepgradient` B 卡 oracle 探针 + Golden 两连 0.0000%；`geometry` 两连 | 大 |
| 2 | **CPU 位图路径整形（A1 后续）** | `render/text/draw.go` 的 `drawGlyphs`（cmap 迭代 + hinted-advance/TT bytecode 吸附机具）切 `ShapeForDraw`；hinted advance 需从整形结果取（HVAR/hint pen 口径，同 VF 修复先例） | §0.2 A1 行 | 红灯：CPU 路径 "fi" 连字（与 A1 红灯同构）；`text.Draw*` 族回归；CPU 位图真窗（`miscdraw` E 卡等）两连 | 中 |
| 3 | **ColorRasterCache fg 键出键（C2 后续）** | `render/text/color_raster.go:53` 的 `colorRasterKey` 对 CBDT 位图同样带 fg（位图不吃 fg）→ 同字形每文字色一份条目 + 满 maxSize 随机驱逐；按 C2 同口径：CBDT 键清 fg、COLR 保留 | §0.4 C2 行 | 红灯：同字形双色两键→修后一键；`emoji` 真窗两连 | 小 |
| 4 | **D1 Windows 真机验收** | 行距上传在真机 WGL 上复验（字形图集脏矩形）；需 Windows 真机环境 | §0.1 D1 行 + §2 验收表 | 真机上传读回无错位 | 小（待机器） |
| 5 | **textstroke 抓屏 17.15% 存量** | 活帧 vs 快照重绘路差异（四步隔离实证与 A1 无关）；疑抓屏时机抓到合成器过渡帧或局部刷新未完成，归 presentation/抓屏域 | §0.3 | 定位根因后按所属域口径关账（本方案只登记归属） | 中 |

## 0.5 后续线 #1 进度（2026-10-25，清单 §3.1）

- **C1 终态第一刀 ✅**：`brush_native.go` radial/sweep 分支重排——GPU ramp×mask 提升为**所有形状（含 AA 矩形）首选**（原顺序把矩形优先派给 CPU 场，即 §6 登记的性能项真凶），场降为 ramp 拒绝形状后的兜底。**归因与批准**：`sweepgradient`（B 卡矩形 sweep）金图漂移 2.2565%、`geometry`（Row4 径向卡）0.3783%，全部为 ±1-2/255 ramp 量化差（1D ramp n≤2048 步进 vs 精确 ColorAt；Skia `SkGradientShader` 位图表同行为），像素探针 12/255 全过、两跑逐位确定——**经用户批准两窗重出**，各两连 0.0000%；`imagepattern` 零漂移两连过；`Brush|Sweep|Radial|Gradient|Glyph` 族回归绿。live 路径每帧 CPU 场重建就此消除（ramp 命中时不建场）。剩余：linear 对角 2D 场的 ramp 化（同构，另刀）。

- **C1 终态第二刀 ✅**：linear 分支重排——对角/任意方向 linear 的 GPU ramp masked 提到 2D 场前（H/V span 快路径不变）。值级新增 `linear_diagonal_ramp_test.go`（对角矩形 ramp vs ColorAt oracle ±2/255 + 背景 不越界）——修前无此覆盖（对角只有动画窗 M5 无 golden）；真窗：`dither` 7.48%/`miscdraw` 0.99% 漂移为 ramp 量化 ±1/255 被 Bayer 抖动放大（dither 窗语义即输入量化可视化探针；探针全过、两跑确定），同批归因经批准重出各两连 0.0000%；`geometry`/`recording` 零漂移。live 路径 2D ColorAt 场就此全部退出常规渐变（仅剩 ramp 拒绝的形状与真自定义 brush）。

- **#3 ColorRasterCache fg 出键 ✅**：`color_raster.go` 的 `colorRasterKey` 按 `GlyphTypeBitmap` 清 fg（CBDT 位图自着色，rasterizeColorBitmap 本就不吃 fg；COLR 保留 currentColor）。红灯（同字形红/绿两色两份缓存条目）→绿→变异反证→`ColorRaster` 族回归绿 + `render/text` 排除 zz_* 慢族后全包 503s 绿（zz_* 慢扫描族聚合超时为既有存量口径，与 B1 同族登记）+ `emoji` 真窗两连 0.0000% 零漂移。位图同字形各文字色共享一份光栅，缓存碎片化与随机驱逐面消除。

- **#2 CPU 位图整形 ✅（A1 收尾）**：`drawGlyphs` 弃 cmap 迭代+hinted-advance 手动走笔，切 `ShapeForDraw` 整形 run（位置取整形 X、hinting 只管轮廓栅格、hinted 字形逐个整数吸附对齐 GPU glyphPlacement snapX；弃 ttCache 定位机具——它正是 kerning 被 GPOS 覆盖、与 GPU 分叉的根源）。红灯：镜像 oracle（同栅格化器按 shaped GID+位置合成参考图，与 `Draw("ffi")` 逐像素比，修前差 77px=ffi 连字缺位）→绿→变异反证（换回 LayoutGlyphs→红）。回归：Draw/Measure/Wrap 族绿；真窗 `miscdraw`/`emoji`/`recording` 零漂移（门禁走 GPU 路，CPU 位图路只在 Aliased/回退触发）、`textlow` 0.0371%（C 卡 Aliased 行整形移位，A1 收尾同类归因）重出两连 0.0000%。顺带澄清：`render/text` 全包 FAIL 系 zz_* 慢扫描家族（wqy/b2/ftgrays/tt_full）聚合超时，排除后 503s 全绿（既有存量口径）。

## 1. 执行顺序建议

1. **第一批（小改高价值，先红后绿）**：A3 → D2 → D1 → B2（互独立，可并行开线）。D2 必须在 A1/A2 之前——两者都会加大过 `HbShaper` 的整形流量，先锁再放量。
2. **第二批（文本线，一条线做完）**：**A2 先行 → A1 随后**。依赖方向：A1 要把默认路径切到 `text.Shape`，而 `text.Shape` 对 MultiFace 返回 nil——A2 落的 per-face 拆段机具（抽成 text 包共享助手）正是 A1 支持 MultiFace 的前置；A1 若先行会让 MultiFace 消费方从 cmap 可画退化成整段空白。两刀同批回归、文本窗 Golden 只重定一次（kerning 宽度变化 + RTL 拆段的变化合并归因）。
3. **第三批**：B1 → C2（互独立；与文本线无交集，各自窗 Golden）。
4. **第四批（性能刀，带基准）**：C1 过渡刀 → 终态（ramp 化）。

顺序要点复核（2026-10-25）：A3 先于 A1——两者都改 `glyph_mask_engine.go`，键修正先行避免同文件冲突；A1 的基线重出放在 B1/C2/C1 之前——后续批次在新基线上跑，避免二次重定；C1 收尾——影响面是像素路径本身，需带基准数据且与其它项零耦合。每项修完：所属域总账（`RENDER_FEATURE_COVERAGE.md` §6/§11、`RENDER_API_CATALOG.md` 如有公开 API 变化）+ 相关真窗 README 同步；本文档对应条目标 ✅ 并记提交号。

## 2. 每项验收标准（销账判据，2026-10-25 定版）

每项销账 = ① 红灯测试（修前必红、期望值来自独立 oracle 非 implementation 自身输出）+ ② 修后转绿 + ③ 变异反证（撤修复必再红）+ ④ 指定包逐文件回归 + ⑤ 指定真窗 `-auto-only` 两连 0.0000%。缺一不算关。

| 项 | ① 红灯测试落点 | ② 绿灯判据 | ④ 包回归 | ⑤ 真窗（两连 0.0000%） | 特别说明 |
|---|---|---|---|---|---|
| A3 | `render/internal/gpu/glyph_mask_hinting_test.go` 扩例：同源两 face 仅 `WithHinting` 不同→引擎 `AtlasStats().entries` 必须新增条目 | 两 hinting 各自条目数正确；同 hinting 仍复用（不爆图集） | `render/text`、`render/internal/gpu` 逐文件 | `ui_render_base_textlow`（六 TextMode）、`ui_text_m5_anycase`（多 DPR） | Latin 掩码不变，**基线不应漂移**（漂了=修错） |
| D2 | 新 `render/text/shaper_hb_race_test.go`：双 goroutine 同源 12/48px 并发 Shape ×200，`-race` | 修前 race detector 报 DATA RACE；修后 0 条且两字号 advance 与单线程基准一致 | `render/text` 逐文件 + 全包 `-race` | `ui_render_base_textspacing -race`、`ui_text_m5_anycase -race` | 锁方案先于 A1/A2 放量 |
| D1 | 无 Windows 真机：交叉编译（`GOOS=windows`）+ 与 `queue_linux.go` **逐行对齐**审查；真机验收**登记待做** | 双端上传代码同构（ROW_LENGTH+复位）；Linux 全绿 | `gpu/gwgpu/gles` 逐文件 | Windows 真机（待验收，不假绿） | 诚实口径：代码级验证≠真机验证 |
| B2 | 新 dash 连续性测试（`render` 包）：双共线子路径间隔 10px、period {20,20}，oracle=累计距离公式断言第二子路径首 dash 相位=10 | 修前相位=0（重置）必红；修后=10；dash 缩放双向对齐 T.03 口径 | `render` 的 `Stroke\|Dash\|Subpixel` 过滤 + `pathops_subpath_test` | `ui_render_base_strokepreset`（D 卡 Dashed/Dotted） | Discrete/Trim 不受影响照跑 |
| A2 | `render/text/layout_test.go` 的 `TestLayoutText_MultiFace` 扩 RTL：阿语+emoji 混排→字形数>0 且含两 face 的 GID | 修前 0 字形必红；修后非零且视觉序正确（段序反转断言） | `render/text` 逐文件 + `-race` | `ui_render_base_textspacing`（B 卡 RTL）、`m5_anycase`（A 区） | 与 A1 同批回归、Golden 只重定一次 |
| A1 | 新测试（`render/internal/gpu`）：`LayoutText("لمّا")` GID 序列≠cmap 序列；`AV` 宽度==`text.Shape` 基准 | 序列含 GSUB 连字 GID；宽度逐位等于整形基准 | `render/internal/gpu` 文本测试逐文件 | `ui_render_base_text/textlow/textspacing/textstroke` + `m5_anycase` | **预期 kerning 基线漂移→归因后经用户批准重出**；MultiFace 走 A2 拆段不退化成空白 |
| B1 | `render/clip_ops_semantics_test.go` 扩例：45° CTM `ClipRect`，oracle=旋转四边形解析覆盖；AABB 内、四边形外的角点必须背景 | 修前角点画出必红；修后纯背景；轴对齐快路径行为不变 | `render` clip 族逐文件 | `ui_render_base_cliprrect`、`clipop` | 旋转矩形→path 裁剪带 AA |
| C2 | `render/internal/gpu/glyph_color_atlas_test.go` 扩例：灌满 maxPages→新 Put 成功且最冷条目被逐出；同字形双前景色→一条目 | 修前 `ErrColorAtlasFull` 必红；修后 LRU 生效；两色像素各自正确（tint 后断言） | `render/internal/gpu` 逐文件 | `ui_render_base_emoji`（A/B/D 卡） | FG 出键前先核查 COLR currentColor 用途（方案 C2.2） |
| C1 | 新 bench（`render/internal/gpu`）：同 brush 连填 100 帧，记 allocs/耗时 | 过渡刀：稳态 0 分配/帧、CPU<50µs；终态：场路径仅剩真自定义 brush 且有显式标注 | `render/internal/gpu` 逐文件 | `ui_render_base_sweepgradient`（B 卡 oracle 12/255）、`ui_render_base_geometry` | 基准数据入文档；总账 §6 对应性能子项同批销账 |

