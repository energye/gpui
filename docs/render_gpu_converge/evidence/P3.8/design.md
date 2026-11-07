# P3.8 正式方案（D1–D7 已拍板 2026-11-07，全按推荐档）

- 上游：`assessment.md`（缺口清单 G1–G14、码面实锤、影响面）。
- 总口径（D1）：**完整静态 SVG 画对**。语义锚 = Skia modules/svg（SkSVGDOM）行为；resvg 支持表当核对清单；手写解析器扩展，不引第三方 Go SVG 库；render 层缺底座在 render 层补，render/svg 只做解析与映射。
- 显式不做（D2，具名拒绝+登记）：SMIL 动画、script、foreignObject、跨文件 `<use>` 外部引用、`<a>` 按链接语义处理（按 g 渲染）。
- 批次（D3/D7）：六批按序 1→6，每批一扇独立真窗、该批内收口；批 6 可并行。
- 彩色字形边界（D4）：A 档——P3.8 四格式静态全包（批 5），P4.4 = fvar/avar 变量轴 deltas + CPAL 调色板切换/变体。
- K5（D5）：穿链，GlyphEntry 10B→14B（+int32 Cluster），批 6 同批全量切换；K3 维持在册。
- 嵌套 Picture（D6）：批 6 `Recorder.DrawPicture` 通用命令 + 深度/环护栏；`CmdSceneReplay` 保持不动。

## 批 1：SVG 文档语义地基（本批范围与设计）

### 1.1 能力定义（每点对齐 SkSVGDOM 行为）

**F1 preserveAspectRatio（含 G1 + G13 嵌套视口共用）**

- 属性 `preserveAspectRatio="<align> <meetOrSlice>"`，align 九值（xMinYMin…xMaxYMax，缺省 xMidYMid），meetOrSlice ∈ {meet(缺省), slice}。
- viewBox→视口换算统一为：`scale = meet ? min(sx,sy) : max(sx,sy)`；平移按 align 分量（min→0、mid→(视口−内容·scale)/2、max→视口−内容·scale）。
- **根视口裁剪**：根 `<svg>` 按视口矩形裁剪内容（overflow 默认 hidden；`overflow="visible"` 显式声明才放行）——slice 内容出界必须被裁，这是与 meet 配套的语义。当前实现无任何视口裁剪，本批补上。
- 嵌套 `<svg>`（G13）：创建新视口，同样吃 pAR/overflow 语义，嵌套深度不限（文档树深度天然有界）。
- 语义问题 4 条中 pAR 条（assessment §1.1-1）在本批收口；url() 黑块条（§1.1-2）由 F4 收口。

**F2 完整单位系统（G2）**

- 绝对单位：px=1、pt=96/72、pc=16、in=96、cm=96/2.54、mm=96/25.4、q=96/254（CSS 96dpi 口径，同 SkSVGAttributeParser）。
- 字号相对：em=当前 font-size、ex=font-size×0.5（x-height 近似，Skia 同近似）；font-size 属性进继承状态（初值 16px，CSS initial medium 口径），`font-size` 表现属性本批只挂状态供单位用，文字渲染在批 3。
- 百分比：按引用轴解析——x/宽度类属性相对视口宽、y/高度类相对视口高、r 类（半径）相对 `sqrt((w²+h²)/2)`（SVG 1.1 口径）；替换现「子元素 % 具名报错」策略（报错策略只保留给解析不了的值）。
- 长度解析产物：`Length{value, unit, isPercent}`，解析期不定值，绑定视口期定值（视口可嵌套）。

**F3 继承模型完整化（G3）**

- 属性继承表（SVG 1.1 inherited 属性、本批相关者）：fill、fill-opacity、fill-rule、stroke、stroke-opacity、stroke-width、stroke-linecap、stroke-linejoin、stroke-miterlimit、stroke-dasharray、stroke-dashoffset、color、font-size、clip-rule、visibility。opacity 不继承（元素级）。
- `style` 属性：CSS 声明串解析（`prop:value;…`），优先级高于同名表现属性（CSS cascade：style attr > presentation attr）。
- `class` 选择器最小集：`<style>` 元素里的 `.cls{...}` 单类选择器（element 级选择器 tag{...} 顺带支持）；复杂选择器（后代/伪类/@media）本批不解析、`<style>` 内容里遇到不支持选择器整体具名忽略（不静默半解析）。注意 `<style>` 在 defs 里也要收集。
- `currentColor`：fill/stroke 值为 currentColor 时取继承的 `color`（初值 black）。
- stroke-dasharray/dashoffset/miterlimit：映射 `Context.SetDash/SetDashOffset/SetMiterLimit`（底座已备 render/context_style.go:279,327,351）；`stroke-dasharray="none"` 或奇数个值按规范复制成偶数序列。
- clip-rule 现状「借用 fill-rule」（renderer.go:259-262）本批保留（真裁剪语义批 2 归位），不改行为。

**F4 引用机制与具名拒绝（G4/G5）**

- 文档级 `#id` 查表：解析期收集全部带 id 元素（含 defs 内），`url(#id)` / `href` / `xlink:href` 统一走查表。
- `<defs>`：收集不渲染；`<symbol>`：同 defs 收集（视口语义随 use 实例化时生效）；`<use>`：实例化渲染（x/y 平移；引用 `<symbol>` 且 use 带 width/height 时按 symbol viewBox 走 pAR 新视口；引用普通元素直接展开）。**use 嵌套 use 支持，深度上限 64**（超限具名拒绝该实例，防环）。SVG 2 裸 `href` 与 `xlink:href` 都收。
- **fail-closed 具名拒绝策略**（替代黑块）：
  - 解析期：引用不存在的 id → 该元素按「fill/stroke 均无效」处理（SVG 规范：无效 paint = 不画该笔），并在 Parse 返回值携带 `Warnings []ParseWarning`（元素名/属性/原因/字节偏移）；
  - url() 指向批 2 才支持的类型（gradient 等）→ 本批先按「未注册引用」同策略不画该笔 + Warning（批 2 接上后自然变成真渐变）；
  - Parse 本身不因 Warning 失败（容错渲染，resvg/Skia 同口径）；显式想做严格校验的调用方读 Warnings 自行判断。
- 公开口新增（见 1.2）：`ParseWarning`/`Document.Warnings`。

**F5 语义修复与文档收口**

- `render/svg/doc.go` 重写支持面/不做面两节（transform 六函数补记、icon subset 定位改完整静态子集、pAR/单位/继承/引用语义写明）。
- 行为变化点唯一清单（A/B 归因对象）：pAR 缺省 meet 改变非等比 viewBox 的落点、根视口裁剪可能裁掉此前越界出画的部分、% 单位从报错变有效、url() 引用从黑块变不画/解析。

### 1.2 公开 API 增量（批 1，catalog §13 P3.8 行转正式的批 1 部分）

- `render/svg` 新增公开符号（预估 +10~14）：
  - 类型：`Length`、`LengthUnit`（含常量 LengthPx/Pt/Pc/In/Cm/Mm/Q/Em/Ex/Percent）、`ParseWarning`、`UseElement`、`DefsElement`、`SymbolElement`、`SVGElement`（嵌套视口，替换现"仅根"假设）；
  - 函数/字段：`Document.Warnings []ParseWarning`、`Document.ElementByID(id string) Element`（查表查询口）；
  - 哨兵：无新 panic 面，Warning 携带原因字符串（不定义 error 哨兵族，Parse 仍只对真失败返 error）。
- 存量签名零改动（接口全量切换制不触发：无改名无改签名）；`Attrs` 加字段为结构体增量（非公开签名变化）。
- apidoc/catalog：批 1 交付跑 `go run ./scripts/apidoc` rc=0；catalog svg 节补批 1 行。

### 1.3 实现方式（单后端无关；双后端验证方式）

- render/svg 纯 CPU 侧映射：解析（parser.go 白名单扩 defs/use/symbol/svg/style）→ 文档模型（新元素类型 + 引用表 + 继承求值器 inheritance.go 新文件）→ 渲染（renderer.go 每元素分支调 render.Context）。无 GPU 腿新增——svg 渲染进 Context 后由既有 GPU/CPU 双路径承接；真窗里 SVG 出图经 `ImageBufFromImage`+`DrawImageEx` 走两后端各验一遍（ui_render_base_svg 同款）。
- pAR 换算位置：`renderInternal` 的 scale/translate 段重写（svg.go:71-101），抽 `viewportTransform(viewport, viewBox, pAR) (render.Matrix, ok)` 纯函数便于钉值级测试；根与嵌套 svg 共用。
- 视口裁剪：`dc.ClipRect` 视口矩形（根在 renderInternal Push 后、嵌套 svg 在其 Push 后）。

### 1.4 验证方案

- **能力单测**（新增 `render/svg/p38_doc_test.go`，先红后绿；SVG 样本落 `render/svg/testdata/p38/`，禁循环生成标准数据——样本手写入库）：
  - pAR 九值×{meet,slice} 18 格 viewportTransform 值级钉（对照手算矩阵）；
  - 根视口裁剪：越界内容出图面积=0；
  - 单位换算 18 值钉（7 绝对 + em/ex×font-size 继承 + %×三轴）；
  - 继承：八属性沿 g 树逐层钉（style 属性覆盖表现属性、class 选择器命中、currentColor 解析、stroke-dasharray 奇数补偶）；
  - 引用：use 展开（含 symbol+width/height+pAR、嵌套 use、深度 64 拒绝）、悬空引用不画+Warning、url() 未注册引用不画+Warning、Warnings 内容逐字段钉。
- **真窗** `examples/ui_wr_p38_svg_doc`（新窗，满窗即内容）：面板化逐能力（pAR 九宫格 meet/slice、单位标尺、继承树、use/symbol 引用、悬空引用拒绝牌、视口裁剪），逻辑探针 + 语义像素探针 + 金图；`-auto-only` 自终止 + 常驻人工两段式；跑窗环境变量按台账（`WGPU_NATIVE_PATH`/`LD_LIBRARY_PATH`/`DISPLAY`，人工强制 X11）。
- **金图**：窗 `golden/*.png` 容差 0；播种后复跑 0.0000%。
- **双后端**：webgpu(native) 与 gwgpu-gles 各两轮（seed/verify），逐位 0.0000%；gwgpu-metal 不可验（darwin 交叉编译）。
- **负向对照**：摘 pAR 换算（退回独立拉伸）→ 九宫格钉红；摘视口裁剪 → slice 出界钉红；摘继承求值器（退回 parentFill/parentStroke 两属性）→ 继承树钉红；摘 use 展开 → 引用元素消失钉红；摘 Warning 收集 → 悬空引用钉红。每条红后还原复绿。
- **存量回归**：`render/svg/svg_test.go` 存量测试全绿（pAR 修正可能动其中非等比用例的期望值——按新语义改写并在测试里注明依据）；真窗 `ui_render_base_svg` 金图 A/B（16×16 方形 meet 恒等，预期 0.0000%，若有差逐像素归因）。

### 1.5 批 1 门禁清单（合入前）

build/vet 触及包 rc=0 → 定向单测（新文件+存量 svg 包）逐文件 → apidoc rc=0 → conformance 双后端 → `-race` 触及包 → 真窗双后端 seed/verify 0.0000% → 负向对照红→绿 → 存量金图 A/B 归因 → catalog+apidoc 同步。

## 批 2–6 骨架（随各批开工细化）

- 批 2 svg_paint：linear/radial 渐变（stops/gradientUnits/gradientTransform/spreadMethod——**spreadMethod 为 render 底座增量，落 render 渐变层**；radial 焦点 fx/fy 同核）、clipPath（clip-rule 归位）、mask、image、pattern（CustomPatternBrush 映射）；窗 `ui_wr_p38_svg_paint`。
- 批 3 svg_text：text/tspan（字体解析回退、anchor、letter/word-spacing、dx/dy/rotate；import 环解法定稿）；窗 `ui_wr_p38_svg_text`。
- 批 4 svg_filter：fe 子集映射 P3.4 滤镜家族；映射不到的原语具名登记不做；窗 `ui_wr_p38_svg_filter`。
- 批 5 color_font：sbix 接线、SVG 表→render/svg、COLRv1 paint graph 静态求值、colr.go:177 假注释与 draw_emoji.go:70-73 双判断收口；前置=dc.Image() regionBytes 提升；窗 `ui_wr_p38_color_font`。
- 批 6 picture：`Recorder.DrawPicture`+深度/环护栏、K5 穿链（GlyphEntry 10B→14B 全量切换+对拍基线处理）；窗 `ui_wr_p38_picture`。

---

## 批 2 细化：SVG paint 完整化（2026-11-07 开工登记）

### 2.0 底座盘点结论（全 file:line，**render 层零增量**）

| 能力 | 底座现状 | SVG 映射 |
|---|---|---|
| 渐变 spreadMethod | `render.ExtendMode` 三态全备：CPU `gradient.go:24-29,153`（colorAtOffset 处理 Repeat/Reflect）、GPU `brush_native.go:342 gradientExtend` + `linear_ramp_mask.go:44`（extend uniform 0/1/2） | `spreadMethod` → `SetExtend` 直映 |
| 渐变焦点 fx/fy | `RadialGradientBrush.Focus/SetFocus` + `StartRadius/EndRadius`（`gradient_radial.go:33-39`），focal 走 ray-circle（computeTFocal） | cx/cy→Center、r→EndRadius、fx/fy→SetFocus |
| clipPath | `Context.Clip` 路径裁剪 + P3.5 `Path.Op` 保曲线布尔（并集） | clipPath 子形状 union → Clip |
| mask | `Mask` + `NewLuminanceMask`（`mask.go:182`，CSS Masking Level 1）+ `MaskCanvas.PushMaskLayer/SetMask` | luminance 语义直配 |
| image | `Context.DrawImage/DrawImageEx` + render 解码器族 | data: URI → Decode → DrawImage |
| pattern | `CustomBrush`（`brush_custom.go:34` ColorFunc）+ GPU 路径（`brush_native.go:206,538` customBrushFieldSeed） | tile 离屏渲 → ColorFunc 平铺采样 |

### 2.1 拆步与语义

- **2a 渐变**：`linearGradient/radialGradient`（x1/y1/x2/y2、cx/cy/r/fx/fy、`gradientUnits` userSpaceOnUse|objectBoundingBox、`gradientTransform`、`spreadMethod`、`stop` offset/stop-color/stop-opacity、渐变 `href` 继承 stops）；`fill`/`stroke="url(#g)"` 转正（validatePaints 对渐变目标不再 invalid，渲染期 resolve 成 Brush：渐变→`SetFillBrush`，颜色→`SetColor`）；objectBoundingBox = Translate(bx,by)∘Scale(bw,bh)∘gradientTransform∘渐变坐标。**前置重构**：`parseTransformList(s) (render.Matrix, error)` 纯函数（M=M1·…·Mn 右乘链），`applyTransform(dc)` 改薄壳。
- **2b clipPath**：`clipPath`（`clipPathUnits`、子形状并集——`Path.Op` 保曲线、clip-rule 归位到 clipPath 内容求值）+ `clip-path="url(#c)"`。
- **2c mask**：`mask`（maskUnits/maskContentUnits、x/y/w/h 默认 -10%~120% bbox、luminance）+ `mask="url(#m)"`。实现=**用户空间三离屏组合**（mask 内容离屏渲→LuminanceMask→被裁内容离屏渲→DestinationIn→组合图 DrawImage 回主 dc）——CTM 天然正确（Skia saveLayer/Flutter drawDisplayList+paint 同形），规避 MaskCanvas 设备空间 1:1 限制，双后端同路。
- **2d image**：`<image>` href=**data: URI only**（base64/percent 两形），外部文件引用具名拒绝（D2 边界）；`preserveAspectRatio` 对图像视口（默认 xMidYMid meet，viewport=图像固有尺寸）。
- **2e pattern**：`pattern`（patternUnits/patternContentUnits 默认 userSpaceOnUse、patternTransform、viewBox、href 继承内容）；tile=离屏 Context 渲内容→`CustomBrush` 平铺 ColorFunc。

### 2.2 提交节奏

子步提交（每枚提交前 question）：①2a 渐变（含 transform 纯函数化）；②2b+2c clipPath+mask；③2d+2e image+pattern；④真窗 `ui_wr_p38_svg_paint` + 批级门禁（五目标/64 包 sweep/conformance/双后端金图）收口批 2。

### 2.3 验证方案（随步红测）

- 2a：stops 排序/越界 offset 钉、units 两档 bbox 换算钉、gradientTransform 矩阵合成钉（对照手算）、spreadMethod 三态 ColorAt 像素钉、focal 偏移像素钉、href 继承 stops、悬空 url 回归（批 1 行为保持）。
- 2b：clip 并集/挖洞（evenodd clip-rule）像素钉、objectBoundingBox units、clip-rule 归位（被裁元素自身 fill 不受影响）。
- 2c：luminance（白=显/黑=隐/灰=半）、mask 区域默认 bbox 扩 10%、mask 内容继承树。
- 2d：data URI base64 PNG 画位钉、pAR meet/slice 图像视口、外部 href 拒绝+Warning。
- 2e：tile 平铺周期像素钉、patternTransform、href 继承。
- 负向对照：摘 spreadMethod→三态钉红 / 摘 clip 并集→挖洞钉红 / 摘 luminance→灰阶钉红 / 摘 tile 周期→重复钉红。

---

## 批 3 细化：SVG 文本（2026-11-07 开工登记）

### 3.0 import 环解法（设计稿第一题，定案）

实测 `render/text` **不 import render 主包**（grep 全包零命中）⇒ `render/svg → render/text` 无环，批 3 直接依赖。批 5 的反向需求（SVG 表字形：text → svg）届时走**接口倒置**（text 定义渲染接口、svg 实现、高层接线），批 3 不预留代码。

### 3.1 字体解析口径（fail-closed）

- svg 包公开钩子 `SetFontResolver(fn)`：fn(family, weight, italic) → *text.FontSource（nil=无匹配）。
- 无 resolver 或无匹配 → **具名 Warning + 文本不渲染**（fail-closed；SkSVGDOM 用系统默认，本仓纪律优先）。
- 字体匹配：FontSource 的 FullName/Family 包含匹配（大小写不敏感）+ weight/italic 近似（FontFamily.pick 同款）。
- 测试/真窗用 **AHEM 字体**（render/text/testdata/ahem.ttf → 复制入 svg testdata/p38/，22KB）：每字符=满 em 高实心方块 ⇒ 锚点/间距/逐字符定位全部精确断言。

### 3.2 语义范围（对齐 SkSVGDOM 静态文本）

- `<text>`：x/y（**多值=逐字符定位**）、text-anchor（start/middle/end，整行对齐）、font-family/size/weight/style、letter-spacing、word-spacing（词间隔）、dx/dy/rotate（逐字符相对偏移/旋转）、fill/stroke（走形状同一条 paint 管线——渐变/clip/mask 自动生效）。
- `<tspan>`：绝对 x/y 或相对 dx/dy + rotate + 样式覆盖（继承 text）。
- **不做（具名拒绝+登记）**：textPath（沿路径）、textLength 强制、writing-mode 竖排、系统字体扫描（归 ui/平台层——svg 只认 resolver 钩子）。
- 渲染形态：**glyph 轮廓→render.Path→fillAndStroke**（与形状同管线；文字可渐变填充/描边/裁剪/遮罩；不走 GPU 文字管线——svg.Render 是一次性出图，合理）。

### 3.3 布局与渲染

- 逐段（text 直文本 + tspan 序列）shape（`text.Shape`）→ glyph 定位（x 多值逐字符/anchor/dx/dy/letter-spacing）→ 每 glyph outline（face.Source().Parsed().GlyphOutline）→ outlineToPath（FlipY：字体 Y 向上、屏幕 Y 向下；rotate 逐字符矩阵拼合）→ 拼进该元素的渲染 Path → fillAndStroke。
- 公开面：`TextElement/TspanElement/SetFontResolver`（+text.FontWeight 引用）。

### 3.4 验证方案

- AHEM 样本四枚：text_basic（anchor 三态×字号）、text_tspan（dx/dy/rotate+样式覆盖）、text_paint（渐变填充+clipPath 裁剪文字——paint 管线红利）、text_missing_font（无匹配不画+Warning）。
- 像素断言：AHEM 方块的角/边采样（x/y/宽度精确）；金图真窗逐位。
- 负向：摘 anchor 换算→中点/末端钉红；摘 rotate→旋转字符钉红；摘 resolver→缺字体钉红。
