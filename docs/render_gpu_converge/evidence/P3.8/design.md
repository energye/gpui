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
