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

---

## 批 4 细化：SVG 滤镜（2026-11-07 开工登记）

### 4.0 底座盘点（file:line，**render 层零增量**）

| SVG 原语 | render 底座 | 映射 |
|---|---|---|
| feGaussianBlur | `Context.ApplyBlur/ApplyBlurXY`（filter_ops_cpu.go:82,100） | stdDeviation → radius（Skia 口径 radius=2×σ？——**SVG stdDeviation 直接用 σ**，blur radius 口径按底座语义对齐） |
| feColorMatrix | `Context.ApplyColorMatrix([20]f32)`（:139）；saturate/hueRotate/luminanceToAlpha 为标准矩阵公式（SVG 规范给出） | type=matrix 直接 values；其余按规范构造 20 值 |
| feDropShadow | `Context.ApplyDropShadow(dx,dy,blur,color)`（:118） | 直映 |
| feBlend | BlendMode 枚举全（context_image.go:92-150）：normal/multiply/screen/darken/lighten | 离屏 B 上 DrawImage with BlendMode |
| feComposite | Porter-Duff 全：over=Normal、in=SourceIn、out=SourceOut、xor=Xor（B.02 全套） | 同上；**arithmetic 具名拒绝**（无 k 系数接口） |
| feOffset | 离屏图偏移贴图（DrawImage dx,dy） | 直映 |
| feFlood / feMerge | 纯色填充 / 依序叠加 | 直映 |

### 4.1 范围与架构

- **解析**：`<filter>`（filterUnits/p primitiveUnits、x/y/w/h 默认 -10%~120% bbox）+ 原语 feGaussianBlur/feOffset/feFlood/feColorMatrix/feBlend/feComposite(4 运算)/feMerge/feDropShadow；in/in2/result 引用图（SourceGraphic/SourceAlpha/前序 result）。
- **求值**：全离屏——内容腿（SourceGraphic/SourceAlpha）→ 按 DAG 逐原语在独立离屏 Context 上执行（Apply* 自动走 GPU 滤镜路）→ 末原语输出 DrawImage 回主 dc。与 mask 三离屏同族。
- **具名拒绝（登记不做）**：feComposite arithmetic、feTile、feImage、feTurbulence、feDisplacementMap、feComponentTransfer、feConvolveMatrix、feMorphology、feSpecularLighting、feDiffuseLighting、lighting 全系、textPath 同批已拒——解析遇之 → Warning + 该 filter 具名拒绝（整个 filter 不应用 = 原样绘制，CSS Masking 口径）。
- filter 引用悬空/无 <filter> → 原样绘制 + Warning（CSS 口径：引用失败不过滤）。

### 4.2 验证方案

- 样本五枚：filter_blur（高斯）、filter_offset_merge（feOffset+feMerge 投影链）、filter_colormatrix（saturate 0 → 灰）、filter_dropshadow、filter_blend（multiply 两图）+ 缺原语拒绝牌。
- 红测：blur 后边缘模糊度（中心/边缘采样差）、colorMatrix saturate(0) 全通道相等、dropShadow 双影（原图色+影色分位采样）、blend multiply 值、悬空/不支持 filter 原样绘制+Warning。
- 负向：摘 blur→边缘锐度钉红；摘 saturate 矩阵→彩色钉红；摘 dropShadow 影→单影钉红。

---

## 批 5 细化：彩色字形四格式静态完整（2026-11-07 开工登记）

### 5.0 码面盘点（全 file:line 复核，与 assessment §1.2 一致 + 两处修正）

| 项 | 现状 | 批 5 动作 |
|---|---|---|
| sbix 解析器 | `emoji/bitmap.go:125-` SBIXParser 全套 API（NewSBIXParser/HasGlyph(gid,strike)/GetGlyph/BestStrikeForPPEM/NumStrikes），**零消费者** | wire 进 `color_font_adapter.go`：colorBackends 加 sbix 字段、loadColor 查 `"sbix"`、GlyphType 加分支、BitmapGlyph 按 strike 取形 |
| `"SVG "` 表 | `font_tables.go:73-104` 读**全表目录无白名单**——表数据已在 `f.tables`（assessment「没读标签」修正为「有数据无消费者」）；`GlyphTypeSVG` 有值无人产出（glyph_type.go:31） | `emoji/svg_table.go` 新解析器（DocumentList 遍历 + gzip 0x1f8b 透明解压）→ GlyphTypeSVG 转正 |
| COLRv1 | `emoji/colr.go` 收 version≤1 但 parseBaseGlyphs(:197)/parseLayers(:219) 纯 v0 定长格式，真 v1 表 numBaseGlyphRecords=0 → HasGlyph 恒假 → 静默退轮廓；colr.go:177 注释「We support COLRv0 and basic COLRv1」为假 | `emoji/colrv1.go` 新文件：v1 头五偏移 + BaseGlyphList/LayerList/ClipList + Paint 格式 1-32 解析与静态求值（Var 变体 deltas 按 0=默认轴值，归 P4.4）；colr.go:177 注释改真 |
| draw_emoji COLR 臂 | draw_emoji.go:70-73 TODO + 走轮廓兜底，与 ColorRasterCache v0 扁平化双判断不一致 | COLR 臂落地走 ColorRasterCache 扁平化（包级缓存），删 TODO |
| GPU 图集键 | glyph_color_engine.go:103-117 C2 键规则：Bitmap 自色键零、COLR 键 fg | ColorRasterImage 加 `UsesFG bool`（COLR 恒真；SVG 按文档含 currentColor/var( 判定；Bitmap 假），引擎按它定键 |

### 5.1 架构（接口倒置，2026-11-07 三项拍板：D5-1 渲路=render.Context 离屏；D5-2 读回前置=窗内一次读帧切片；D5-3 节奏=单枚收口）

- **依赖方向**（批 3 §3.0 定案 render/svg→render/text 单向）⇒ `render/text` 不能反向 import `render/svg`/`render`，SVG 表字形与 COLRv1 求值的像素渲染走**接口倒置**：
  - `render/text` 定义两个原子钩子（`SetFontResolver` 同款先例）：`SetSVGTableRenderer(fn)`（SVG 文档字节+size+fg → RGBA+origin）、`SetCOLRv1Renderer(fn)`（ParsedFont+COLRv1 绘制树+ppem+palette+fg → RGBA+origin）；钩子未设 fail-closed 退轮廓（已验证待接线是正常中间态）。
  - `render` 包实现两钩子（`color_glyph_render.go` 新文件）：SVG 走 `render/svg.Parse`+表字形专用入口离屏；COLRv1 绘制树→`render.Context` 离屏（Solid/三渐变→SetFillBrush、PaintGlyph→Clip(outline)、Transform 族→dc.Transform、Composite→双离屏+BlendMode、ClipList→ClipRect）——**render 层零增量**（outline→Path 转换器按收敛纪律从 svg 批 3 实现中归位 render，svg 改调）。
  - `render` 包 init 自接线：任何 import render 的二进制自动获得钩子（生产接线零动作；text 单测不含 render 时走 fail-closed 路径，天然可测）。
- **SVG 表语义（OT 1.9.1 spec 钉值，2026-11-07 spec 原文核对）**：表头 version=0 + svgDocumentListOffset（表首相对）+ reserved；DocumentList: numEntries + records{startGlyphID,endGlyphID,svgDocOffset(**相对 DocumentList 起点**),svgDocLength}，records 按 startGlyphID 递增；文档 UTF-8 或 gzip（0x1F 0x8B 0x08）二选一；**坐标=SVG 天然 y-down、默认单位=字体设计单位、原点=设计网格原点（基线）、初始视口=em 方格但禁止裁剪**（svg 根 overflow:visible 强制）；glyph N 渲染=文档内 `id="glyphN"` 元素按 **use 语义**实例化（吃批 1 defs/use/#id 机制）；currentColor=宿主前景色、`var(--colorN, fallback)`=CPAL 调色 0 引用（强推荐支持，本批落 palette+fallback 静态档；CPAL 独立解析抽出复用）；advance 走 hmtx 与轮廓字形同值。禁元素（text/foreignObject/switch/script/a/view）按 spec 忽略。
- **消费侧零改动实证**：`GlyphsHaveColor`（color_font.go:96）判 `GlyphType != Outline` 即走色彩路；CPU 腿 text_dispatch.go:387,391、GPU 腿 glyph_color_engine.go 两端中间全是 `ColorRasterCache.Image() → RGBA`——`ColorRasterCache.rasterize`（color_raster.go:111）加 GlyphTypeSVG/COLRv1 两分支即双端全通。
- **读回病前置（D5-2）**：钩子路每字形只在 ColorRasterCache 未命中时读回一次（小画布），真窗探针按 mat_persp 先例一次读帧切片 regionBytes；引擎侧 regionBytes 提升维持另线登记不进本批。
- **测试字体（测试数据入 testdata 纪律）**：解析层单测沿用**表拼接**既有模式（color_font_adapter_test.go:33-107：goregular tables map + 合成表 blob）；真窗载体=Go 生成器（`render/text/testdata/mkcolorfont/`）产四枚最小字体（head/hhea/maxp/cmap/glyf/loca/hmtx 骨架 + sbix/SVG /COLRv1+CPAL/CBDT+CBLC），产物 .ttf 提交 testdata，生成器入库可重建。
- **语义锚**：COLRv1 = OpenType 1.9 paint graph 规范 + Skia COLRv1 scaler 语义（paletteIndex 0xFFFF=前景色、F2Dot14/Fixed 解码、Var 变体按默认实例）；SVG 表坐标语义见上（spec 原文钉值，红测钉）。P4.4 边界不动：fvar/avar deltas + CPAL 切换/变体留 P4.4。

### 5.2 拆步（红测先行，批内一气呵成，单枚提交）

1. sbix 接线（adapter 三处 + 表拼接红测）。
2. SVG 表：emoji 解析器 + GlyphTypeSVG 产出 + 钩子 + render 实现 + UsesFG。
3. COLRv1：emoji 解析器/静态求值 + 钩子 + render 实现（outline→Path 归位）。
4. 收口：draw_emoji COLR 臂 + colr.go:177 假注释 + 引擎 UsesFG 键。
5. 生成器四字体 + 真窗 `ui_wr_p38_color_font`（四格式四卡 + 负向卡，满窗即内容）+ 批级门禁。

### 5.3 验证方案

- 单测（先红后绿）：sbix strike 选择/origin 定位钉；SVG 表 gzip 两形/越界 glyph 区间/悬空区间拒绝；COLRv1 头五偏移/Paint 格式 1-32 逐格式解码钉/ClipList/Var 变体默认值=基础值/0xFFFF 前景色；静态求值端到端（表拼接 v1 字体 → 钩子出 RGBA → 渐变色采样钉）。
- 真窗四卡：CBDT（Noto 系形）/sbix（红方块蓝圆）/SVG（gzip 文档渐变形）/COLRv1（层+渐变+composite+clip box）；logic 探针（GlyphType 四值+钩子已接线）+ 像素探针（分位采样）+ 金图容差 0；双后端 seed/verify 0.0000%。
- 负向：摘 sbix 分流→BitmapGlyph 错走 CBDT 钉红；摘 SVG 钩子→SVG 字形退轮廓钉红；摘 COLRv1 求值→v1 退轮廓钉红；摘 ClipList→越界内容钉红；摘 Composite→混合色错钉红。
- 存量回归：color_raster/draw_emoji/colr/adapter 各测 + svg 包全量（outline→Path 归位若触）+ 批 1-4 真窗金图不动。

---

## 批 6 细化：嵌套 Picture + K5 穿链（2026-11-07 开工登记）

### 6.0 码面盘点（file:line 实测）

| 项 | 现状 | 批 6 动作 |
|---|---|---|
| H21 CmdSceneReplay | ScenePayload 单方法（RenderSceneInto(dc)）只承载 scene.Scene；CanvasBackend.RenderScene 双目标（Context 直绘 Push/Pop 包夹、Recorder 折叠）scene_replay.go:65-81 | **保持不动**（D6） |
| CanvasBackend 折叠 | NewCanvasBackend(*Recorder) 把命令折叠进外层（backend_canvas.go:29-40 明文契约），Begin/End 状态包夹 | 复用为 DrawPicture 的双目标回放机 |
| 通用嵌套命令 | 命令表 command.go:51-124 无 CmdDrawPicture | +CmdDrawPicture + DrawPictureCommand{Pic *Recorder} + Recorder.DrawPicture |
| Canvas 接口 | 实现者仅 *Context 与 *Recorder（recorder_canvas_ops.go:35 断言；Context 侧同款） | +`Canvas.DrawPicture(pic *Recorder)`（全量切换，两实现者同笔）：Context=就地回放（同 Push/Pop 纪律）、Recorder=记命令 |
| 回放护栏 | 无（H21 payload 场景无环可能——ScenePayload 回放不递归） | 回放期身份栈 + 深度 64：CanvasBackend 回放 DrawPicture 时压栈 pic 指针，栈内重复=ErrPictureCycle、深度超限=ErrPictureDepth（Skia 不设防会栈溢出，fail-closed 设防）；哨兵新面 +2 |
| K5 GlyphEntry | 10B（GID u16+X f32+Y f32）encoding.go:404-412；写入 scene/text.go:505（不带 Cluster）；回放重建 ShapedGlyph 两腿均丢 Cluster（gpu_renderer.go:382、renderer.go:1271 renderTextOnTile） | GlyphEntry +`Cluster int32`→14B；EncodeText/Decoder.Text 同笔扩字段；写入点补 `Cluster: g.Cluster`；两腿重建补 Cluster；命令流内进程内契约无版本字段（encoding.go 全文无 version），同仓全量切换零兼容负担 |
| bounds walker | bounds.go contentWalk 不展开 SceneReplay payload（H21 同款） | DrawPicture 同策略：不展开 pic（折叠后命令逐条走 walker；直绘时 damage 由实际绘制产生）——登记口径 |
| 对拍基线 | Hash 是关系断言为主（同内容相等/平铺等价/异内容不等，encoding_test:407、layer_test:671、tagtext_test:235）；无具体 hash 数字钉 | 14B 切换后关系断言自然成立；若有字节级/尺寸钉随步 2 更新并注明依据 |

### 6.1 架构

- **语义**（Skia SkCanvas::drawPicture / Flutter DlCanvas::drawDisplayList 同位物）：`DrawPicture(pic)` 把一份**已完成录制**的 display list 按引用嵌入当前流；回放只读（录制期取值纪律，同 ScenePayload/PathEffect/材质尾参）；调用方在 record→replay 间不得改 pic 内容。
- **回放双目标**（复用 CanvasBackend 既有机制，零新机制）：
  - 目标=*render.Context：`NewCanvasBackend(ctx)` 回放 pic，外层 Push/Pop 包夹（pic 的显式状态不泄漏）；
  - 目标=*Recorder：pic 命令**折叠**进外层流（backend_canvas.go:29-40 明文契约）——嵌套展开后 walker/Hash/后续回放全部自洽。
  - 递归：pic 内可再 DrawPicture——CanvasBackend 回放 pic 时走同一条 Playback→case DrawPictureCommand 路径，身份栈天然覆盖递归。
- **护栏**：CanvasBackend 增 pictureStack []*Recorder（回放状态字段）；Playback 入口清栈。环：压栈前查同指针→ErrPictureCycle；深度 64→ErrPictureDepth。录期 nil pic 跳过。
- **K5 穿链语义**：ShapedGlyph.Cluster（rune 下标，shaping 四实现都填）沿 编码→解码→回放 全链到达 RenderedGlyph/ShapedGlyph 重建点；CPU/GPU 两腿 DrawShapedGlyphs 拿到真 Cluster（为后续富文本命中/光标消费铺路——本批只穿链，不接消费面，K3 维持在册）。

### 6.2 拆步（红测先行，批内一气呵成，单枚提交）

1. DrawPicture：命令+Canvas 接口+双实现+回放+护栏+单测（折叠/直绘/嵌套/环红/深度红/nil 跳过/状态包夹）。
2. K5 穿链：struct/encode/decode/write/两腿重建 + 编解码往返钉测 + 基线适配。
3. 真窗 `ui_wr_p38_picture` + 批级门禁。

### 6.3 验证方案

- 单测（先红后绿）：折叠等价（pic 折叠进外层 vs 直接录同命令，命令序列相等）；直绘像素钉（pic 内容+外层内容共存、CTM 组合正确）；嵌套三层像素钉；环→ErrPictureCycle 红、深度 65→ErrPictureDepth 红；nil 跳过；Push/Pop 包夹（pic 内 Save 不泄漏）。
- K5：编解码往返（Cluster 值逐个钉）；两腿重建 ShapedGlyph.Cluster 断言；Hash 关系断言复跑绿。
- 真窗四卡：卡1 折叠复用（一 picture 三处摆位，同内容像素一致）卡2 嵌套三层 卡3 环拒绝牌（哨兵错误显示）卡4 K5 中英混排 cluster 穿链（logic 钉 Cluster 值+像素钉混排位置）；金图容差 0；双后端各自基线 0.0000%（批 5 同款口径）。
- 负向：摘 Push/Pop 包夹→pic 内状态泄漏钉红；摘身份栈→自引用栈溢出（改深度护栏为 panic 对照——只做存在性验证不真跑溢出，用深度 65 触发）；摘 Cluster 穿链→两腿重建 Cluster 恒 0 钉红。
- 门禁：build/vet/触及包单测逐文件/-race 触及包/conformance 双后端/apidoc rc=0（catalog +CmdDrawPicture/DrawPictureCommand/DrawPicture×2/ErrPictureCycle/ErrPictureDepth）/存量窗金图 A/B（ui_render_base_svg + ui_wr_p38_color_font + 场景窗）/catalog 同步。
