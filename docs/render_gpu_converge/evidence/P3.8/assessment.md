# P3.8 开工影响面评估（SVG 完整语义、彩色字形、嵌套 Picture 引用）

- 日期：2026-11-07（开工登记，零代码；本轮全部 file:line 实测）
- 任务行：任务总表 §2 P3.8「SVG 完整语义、彩色字形、嵌套 Picture 引用」，依赖 P3.0/P3.3/P3.4/P3.5 全 ✅
- 性质：**先评估后拍板再动手**。本文产出五件：①SVG 缺口清单与参考口径选型；②彩色字形与 P4.4 边界；③嵌套 Picture 语义定义；④批次拆分提案；⑤影响面。拍板前不写任何代码。
- 上游定语（不改口）：PLAN §5.1 行 156 明列 SVG 完整语义 = defs/use/渐变/clipPath/text/filter；依赖图（`evidence/P3.0/dependency-graph.md` 行 21-23、48、70、74）登记 P3.4→P3.8（`<filter>` 子项）、P3.3→P3.8（text/tspan、COLRv1 完整层）、P3.5→P3.8；catalog §13 行 962 规划登记 = `render/svg` 解析扩展 + COLRv1 paint graph 完整解析 + `recording` 嵌套引用命令（DrawRecording + Playback 递归）。

---

## §1 码面现状实测

### 1.1 render/svg：icon 子集（八文件 2259 行，含 754 行测试）

支持面（`render/svg/doc.go:17-35`、`parser.go:195-220`）：

- 元素 8 种：svg/path/circle/rect/ellipse/line/polygon/polyline/g；`parseElement`（parser.go:196-220）白名单分派，未知名走 `decoder.Skip()` **静默跳过**。
- 属性：fill/fill-rule/fill-opacity/stroke/stroke-width/stroke-linecap/stroke-linejoin/stroke-opacity/opacity/transform/clip-rule（parser.go:222-265）。
- transform 六函数全：translate/rotate(含 cx,cy)/scale/matrix/skewX/skewY（transform.go:89-187，doc.go:33 只列了四个是文档旧账）。
- 根：viewBox/width/height，根级 % 按 viewBox 折算（parser.go:127-146）；子元素几何 % **具名报错**（parser.go:152-156，ui_render_base_svg 修过的洞）。
- 单位：只剥 px/pt/em 后当用户单位（parser.go:138-140），无 pt/pc 换算、无 mm/cm/in/q、无 em/ex 字号相对、无全树 % 相对语义。
- 继承：只有 fill/stroke 沿 `<g>` 继承两层字符串（renderer.go:235-252 renderState.parentFill/parentStroke）；fill-opacity/stroke-width/opacity 等其余表现属性不继承，与 SVG 继承模型不同。
- clip-rule 被解析但只当 fill-rule 的兜底别名（renderer.go:259-262），没有裁剪语义。

**语义级问题 4 条（与 SVG 规范/Skia 行为相悖，不只是缺功能）**：

1. **preserveAspectRatio 未实现且行为反规范**：viewBox→目标按 sx、sy 独立拉伸（svg.go:84-86），SVG 默认 `xMidYMid meet` 应等比缩放居中（letterbox）。当前任何非等比 viewBox/目标组合都画变形。这是行为变化点，修了要 A/B 归因（现有真窗 viewBox 16×16 → 方形目标，meet 恒等，理论零变化，须实测确认）。
2. **url(...) 漆黑静默**：`fill="url(#x)"` 解析失败落默认黑（doc.go:49 明文），违反 fail-closed——不认识的引用应该具名拒绝（跳过或报错），不该画黑块。
3. **clip-rule 借用**：只有 fill 语义，没有 clipPath；等批 2 落 clipPath 时归位。
4. **doc.go 注释与实现漂移**：doc.go:33 说 transform 支持 4 函数（实为 6）；doc.go:47-49 的「不做清单」在 P3.8 完成后整段要重写。

### 1.2 彩色字形（render/text + render/text/emoji）

- **接口面齐但四缺二**：`ColorFont` 接口（color_font.go:22-39）声明四格式（CBDT/sbix/COLR/SVG），`ColorFontInfo.HasSbix/HasSVG` 字段在（color_font.go:46,50）；适配层 `ownParsedFont.loadColor`（color_font_adapter.go:22-44）**只装 CBDT 和 COLR 两路**——sbix 解析器存在（`emoji/bitmap.go:125-` SBIXParser）但零消费者（PLAN §5.2 行 167 已登记），SVG 表完全无解析代码。
- **COLR 纯 v0，v1 字体静默退轮廓**：`emoji/colr.go:177-179` 头部收 version≤1（注释写「We support COLRv0 and basic COLRv1」——**该注释与实现不符**），但 `parseBaseGlyphs`(:197-216)/`parseLayers`(:219-237) 只按 v0 记录格式（6B/4B 定长）解析。真 COLRv1 表的 BaseGlyphList 是 paint 偏移结构，v0 头字段 numBaseGlyphRecords 在 v1 表里为 0 ⇒ `HasGlyph` 恒假 ⇒ v1 字形当普通轮廓画黑字。PLAN §6 行 196 已登记「比只取基础层更糟」。
- **v0 求值链（可保留作底座）**：`ColorRasterCache`（color_raster.go:52-209）按 (font,glyph,ppem,palette,fg) 缓存，CBDT 解码缩放、COLRv0 逐层 mask 合成（:150-203，纯色层 draw.DrawMask）；前景色层 IsForeground→fg（:205-210）。
- **消费两端都在**：CPU 腿 `render/text_dispatch.go:387,391` → `DrawWithEmoji`（draw_emoji.go:31-82；**COLR 臂 :70-73 现走轮廓兜底+TODO 注释，与 ColorRasterCache 里的 v0 扁平化并存=两套判断不一致**）；GPU 腿 `render/internal/gpu/glyph_color_engine.go:35-`（ColorGlyphEngine，RGBA 色彩图集页 + 独立 quad 批）。
- **GlyphType 枚举四值齐**（glyph_type.go:19-32），GlyphTypeSVG 有值无人产出。
- ui 侧生产接线点：`ui/rendering/text.go:1152` → `DrawShapedColorGlyphs`（CBDT+COLRv0 生产链，PLAN 行 167 确认）。

### 1.3 嵌套 Picture（render/recording + render/scene）

- **H21 先例在**：`CmdSceneReplay` 命令 + `ScenePayload` 接口（recording/scene_replay.go:30-59）+ Backend 契约 `RenderScene(p ScenePayload)`（backend.go:90-93）+ Recorder 分发（recorder.go:275-277）。payload 语义：录制期取值、回放只读、Push/Pop 包夹防泄漏；目标为 *Recorder 时折叠进外层录制、为 *render.Context 时就地回放（scene_replay.go:65-80）。**payload 形态限定 scene.Scene**（ScenePayload 只有一法 RenderSceneInto），不是任意 *Recorder。
- **通用嵌套 Picture 命令无**：recording 命令表（command.go:52-124）无 CmdDrawRecording 类命令；PLAN §13 行 962 规划的「DrawRecording + Playback 递归」未做。
- **折叠机制已备**：CanvasBackend 明文支持回放到 *Recorder 折叠进外层（backend_canvas.go:29-33「one display list can be reused inside another」）——通用命令只是把这件事变成命令流里的一等公民（可被 Hash/编码/回放统一处理），不需要新机制。
- render/scene 编码无版本字段（encoding.go 全文无 version），进程内二进制契约，GlyphEntry 改动走同仓全量切换即可，无跨版本兼容负担。

### 1.4 在册项 K3/K5 联动

- **K3**（§2.2 行 377）：cluster 兜底全零三处——`render/text/shape_result_cache.go:405-419`（layoutGlyphsUncached 不填 Cluster，实测确认只填 GID/X/Y/IsCJK）、`ui/rendering/text_layout.go:635,679`（无 face/不可整形兜底 GID:0,Cluster:0）。真源面 ShapedGlyph.Cluster 定义在 shaped.go:20-25（rune 下标，shaper 四实现都填：face.go:289,336、shaper_builtin.go:76、shaper_hb.go:291、shaper_own.go:618,654）。登记口径：「接线 cluster 消费面前须先补全」——P3.8 三件（SVG text 无 hit-test 语义、彩色字形无 cluster 需求、Picture 无关）**都不是 cluster 消费面**，K3 不因 P3.8 自动销案。
- **K5**（§2.2 行 379）：`render/scene` 编码 GlyphEntry 10 字节只有 GlyphID/X/Y（encoding.go:404-412），回放重建的 RenderedGlyph.Cluster 恒 0；直连路径 text.go:178,228 从 ShapedGlyph 带了 Cluster。**RenderedGlyph.Cluster 生产读者为零**（全仓 grep 仅 scene/text_test.go:145-158 断言直连路径）。登记口径「随 P3.8 定夺：穿链或删面」→ 本评估给提案（§4/D5）。

---

## §2 产出一：SVG 完整语义缺口清单与参考口径（D1/D2）

### 2.1 缺口清单（现有子集 → 目标语义，逐项）

| # | 缺口 | 现状 | render 底座是否已备 | 归批 |
|---|---|---|---|---|
| G1 | preserveAspectRatio（meetOrSlice、X/Y 对齐九值） | 不实现且**独立拉伸反规范**（svg.go:84-86） | 无需底座，纯文档级换算 | 批 1 |
| G2 | 完整单位系统（pt/pc/mm/cm/in/q 换算、em/ex 字号相对、全树 %） | 只剥 px/pt/em 当用户单位 | 无需底座 | 批 1 |
| G3 | 继承模型完整化（全部可继承表现属性 + `style` 属性 + class 选择器最小集 + currentColor） | 只 fill/stroke 两属性沿 g 继承 | 无需底座 | 批 1 |
| G4 | url() 引用解析机制 + #id 文档查表 + fail-closed 具名拒绝（替代黑块） | url(...) 落黑块（doc.go:49） | 无需底座 | 批 1 |
| G5 | defs / use / symbol（含 x/y/width/height 视口换算、嵌套 use） | 静默跳过 | 无需底座 | 批 1 |
| G6 | linearGradient / radialGradient（stops、gradientUnits 两档、gradientTransform、spreadMethod） | 静默跳过 | **已备**：NewLinearGradientBrush/NewRadialGradientBrush/NewSweepGradientBrush（render/gradient_linear.go:40、gradient_radial.go:53、gradient_sweep.go:51）+ CustomBrush 三便捷口 | 批 2 |
| G7 | clipPath（clipPathUnits 两档、clip-rule 归位） | 静默跳过 | **已备**：Context.Clip 路径裁剪 | 批 2 |
| G8 | mask | 静默跳过 | **已备**：render 掩码/离屏链（P3.4 滤镜图、scene alpha mask 同底座） | 批 2 |
| G9 | image（内嵌栅格） | 静默跳过 | **已备**：Context.DrawImage 族 | 批 2 |
| G10 | pattern（tile 引用） | 静默跳过 | **已备**：CustomPatternBrush 先例（recording/backend_canvas.go 已有承接） | 批 2（重，可砍，见 D2） |
| G11 | text / tspan（font-family/size/weight/style、text-anchor、letter-spacing、tspan dx/dy/rotate、多字体回退） | 静默跳过 | **已备**：render/text shaping + P3.3 逐 run 样式（TextStyle/DrawParagraphStyled）；SVG text 是绝对定位 run 集，复用 shaping/字体解析/样式，不复用 Paragraph 流式布局 | 批 3 |
| G12 | filter（fe 子集） | 静默跳过 | **已备**：P3.4 滤镜家族（ImageFilter 高斯/偏移/混合/颜色矩阵/丢阴影等）+ 滤镜图离屏链；SVG filter 是 primitive 链映射 | 批 4 |
| G13 | 嵌套 svg 元素（child <svg> 新视口） | 静默跳过 | 无需底座 | 批 1（视口语义与 use/symbol 共用） |
| G14 | marker / switch / a / foreignObject / animation(SMIL) / script | 静默跳过 | marker 可选做；a=g 即可；switch 少见；**foreignObject/SMIL/script 显式不做**（登记具名拒绝） | 批 1 定拒绝面（D2） |

### 2.2 参考口径选型（建议，D1 拍板）

- **语义主锚 = Skia modules/svg（SkSVGDOM）**。依据：AGENTS「渲染层新功能实现，统一对齐 Skia/Flutter 标准实现方案」。Flutter 无第一方 SVG DOM（**本树实证**：`flutter/engine/src/flutter/skia/modules/` 只 vendor canvaskit/skcms/skparagraph/skshaper/skunicode 五模块，无 svg；display_list 无 SVG 节点），Skia modules/svg 是两参考框架里唯一的 SVG 语义实现，取它为锚不违背规则。
- **逐项核对清单 = resvg 支持表**（业界事实参考实现，Rust，仅做语义对照文档，不引代码不引依赖）。
- **不引第三方 Go SVG 库**（oksvg 等）：Go 生态 SVG 渲染库能力面弱于本仓 render 底座（无 filter/mask 完整链、无渐变 spreadMethod 全档），引入只会多一层映射与语义偏差；P3.6 引库先例的成立条件是「自研语义全反」，SVG 这里问题是覆盖面缺失而非语义反向，手写扩展解析器（现有 467 行 parser 继续长）+ 映射 render 底座是正确形态。
- **COLRv1 口径** = OpenType 1.9 COLRv1 paint graph 规范 + Skia SkTypeface COLRv1 scaler 语义（paint 全图求值：transform/translate/scale/rotate/skew、linear/radial/sweep 渐变、glyph 层、clip box、composite 24 模式）；HarfBuzz hb-colr / FreeType 作交叉参照。
- **嵌套 Picture 口径** = Skia `SkCanvas::drawPicture` / Flutter `DlCanvas::drawDisplayList`（H21 scene_replay.go:26-29 已按此立据，通用命令照旧）。

---

## §3 产出二：彩色字形与 P4.4 边界提案（D4）

P4.4 任务行：「COLRv1 完整层（若 P3.8 未覆盖）」，依赖 P3.3+P3.8。边界两档：

- **A 档（推荐）：P3.8 = 四格式静态语义补全；P4.4 = 变量与动态层**。
  - P3.8 做：① sbix 接线（解析器已在，进 loadColor 第二路 + BitmapGlyph 分流，零新机制）；② SVG 表字形（解析 SVG 表 → 每字形一份 SVG 文档 → 走批 1-4 完成后的 render/svg 渲到 RGBA → 进色彩图集；GlyphTypeSVG 从「有值无人产出」转正）；③ **COLRv1 paint graph 完整解析与静态求值**（32 种 paint 格式 + ClipList + composite 24 模式，静态值求值，fvar/VarIndexMap deltas 不动即按默认轴值取）；④ 顺带修 doc 假注释（colr.go:177）与 draw_emoji.go:70-73 的双判断不一致（COLR 臂落地走 ColorRasterCache 扁平化，删 TODO）。
  - P4.4 做：fvar/avar 变量轴 → VarIndexMap deltas（COLRv1 可变 paint 值）、CPAL 调色板切换/变体、变量字体与 COLRv1 的联动收口。理由：变量轴求值需要 fvar/avar 基座且独立可测，塞进 P3.8 会把批 5 变成两批体量。
- **B 档：P3.8 只修 v0 误读（v1 命名拒绝）+ sbix + SVG 表；COLRv1 全量留 P4.4**。代价：P3.8 任务行「彩色字形」交付不完整（COLRv1 是 emoji 字体主流方向，Noto Color Emoji 等大字体是 v1），P4.4 体量暴涨且其依赖注记「若 P3.8 未覆盖」形同虚设。不推荐。

推荐 A 档。边界一句话：**P3.8 管「静态调出来是对的」（四格式全部能画对），P4.4 管「随轴动、随盘换」（变量与调色板动态）**。

---

## §4 产出三：嵌套 Picture 语义定义提案（D6）

- **语义**：`Recorder.DrawPicture(pic *Recorder)`（命名随设计稿定，对齐 Skia drawPicture）——把一份已完成录制的 *Recorder 作为 payload 按引用嵌入命令流（CmdDrawRecording）；回放时目标为 *render.Context 就地回放（Push/Pop 包夹，同 H21 纪律）、为 *Recorder 折叠进外层（复用 CanvasBackend 既有折叠机制 backend_canvas.go:29-33）。payload 录制期取值、回放只读，与 ScenePayload/Stroke.PathEffect/材质尾参同一条「引用携带」纪律。
- **递归护栏**：嵌套深度上限（具名哨兵 ErrRecordingDepthExceeded 或等价）+ 自引用环检测（回放期 payload 身份栈，Skia 不设防会栈溢出，本仓按 fail-closed 设防）。
- **与 H21 的关系**：CmdSceneReplay 保持不动（scene payload 专用、已收口）；DrawPicture 是通用面，CmdSceneReplay 的 ScenePayload 实现恰好也是它的一个消费者形态，两者并存不合并（合并=改已收口命令，收益为零风险为正）。
- **K5 定夺提案（D5）**：**穿链**——GlyphEntry 10B→14B（+int32 Cluster），Encoding/Decoder 同批切换（接口全量切换制），RenderedGlyph.Cluster 在编码路径从此不再恒 0。理由按能力存留两问：①不是重复实现（ShapedGlyph.Cluster 是 shaping 真源，RenderedGlyph.Cluster 是 scene 渲染结果面上的携带位，直连路径已在带）；②属于三支柱内文本能力的正常中间态（scene 富文本命中检测/光标将来需要）；现状「字段在、编码路恒 0」是潜伏错值面，比没有字段更糟。穿链后 K5 可销案；K3（shaping 侧兜底全零）仍维持「接线 cluster 消费面前须补全」，不因穿链自动了结。编码变化会动 scene Hash/录制对拍基线，按拍板重采或证明等价（登记为批 6 验收项）。

---

## §5 产出四：批次拆分提案（D3，六批，对齐 P3.7 节奏）

| 批 | 内容 | 独立收口物 | 真窗（新窗，一能力一窗） |
|---|---|---|---|
| 1 | **SVG 文档语义地基**：pAR 修正（A/B 归因）+ 单位系统 + 继承完整化 + url()/#id 引用机制与具名拒绝 + defs/use/symbol + 嵌套 svg 视口 | 红测先行的能力单测组 + 批交付 | `examples/ui_wr_p38_svg_doc` |
| 2 | **SVG paint 完整化**：linear/radial 渐变（stops/units/transform/spreadMethod）+ clipPath + mask + image（pattern 随 D2） | 同上 | `examples/ui_wr_p38_svg_paint` |
| 3 | **SVG 文本**：text/tspan（字体解析、anchor、spacing、dx/dy/rotate、多字体回退；接 P3.3） | 同上 | `examples/ui_wr_p38_svg_text` |
| 4 | **SVG 滤镜**：fe 子集映射 P3.4 家族；映射不到的原语具名登记不做 | 同上 | `examples/ui_wr_p38_svg_filter` |
| 5 | **彩色字形补全**：sbix 接线 + SVG 表→render/svg + COLRv1 paint graph 静态求值 + draw_emoji COLR 臂落地 | 同上 | `examples/ui_wr_p38_color_font` |
| 6 | **嵌套 Picture**：recording DrawPicture 命令 + 递归护栏 + K5 穿链（随 D5） | 同上 | `examples/ui_wr_p38_picture` |

- 顺序理由：批 1 是文档语义地基（pAR/单位/继承/引用查表），批 2-4 都要吃它；批 5 的 SVG 表字形吃到批 1-4 全部成果（含 text/filter），必须殿后；批 6 独立无前置依赖，可与批 2-5 任一并行，排最后是因为 K5 穿链动 scene 编码需要对拍收尾。
- 每批照 P3.7 口径：先文档（能力定义/公开 API/双后端实现方式/验证方案）→ 红测 → 实现 → 真窗三证据 → 双后端 → 门禁 → 单枚提交（每枚提交前 question 确认）。
- 每批独立可收口：批内能力在该批真窗出金图收口，✅ 不延后（无 P3.7 批 6 式集中收口安排）。

---

## §6 产出五：影响面

### 6.1 公开 API 增量预估（catalog §13 行 962 转正式登记的底稿）

- `render/svg`（现公开面 17 符号：Render/RenderWithColor/Parse/Document 五法/ViewBox/Attrs/Element+8 元素类型）：+15~25 符号——新元素类型族（Defs/Use/Symbol/LinearGradient/RadialGradient/GradientStop/Pattern/ClipPath/Mask/Text/Tspan/Filter+原语/Image）、Parse 选项、引用解析哨兵族（ErrUnresolvedRef 等）。解析器内部结构（引用查表、继承求值器）不公开。
- `render/text`：COLRGlyph 数据面升级（v0 Layers 保留 + v1 paint graph 类型族 ColorPaint*）+ ColorFont 接口可能扩方法（SVG 表/sbix 取形），接口全量切换制一次改完不留垫片：+5~10 符号。
- `render/recording`：+CmdDrawRecording + Recorder/Backend 方法 + 深度哨兵：+3~4 符号。
- `render/scene`：GlyphEntry 10B→14B（若 D5 穿链），同批改 Encoding/Decoder/写入点 text.go:505 与全部读者；编码变化动 scene Hash（验收项）。
- 每批交付跑 `go run ./scripts/apidoc`（rc=0）+ catalog 同步（硬）。

### 6.2 真窗清单与金图

- 新窗六扇（§5 表），全部走 `-auto-only` 自终止自检 + 常驻人工窗两段式；**人工看窗强制 X11**（`env -u WAYLAND_DISPLAY -u XDG_SESSION_TYPE`，2026-11-07 台账登记）。
- 金图：每扇新金图，容差按 P0.1 默认 0；**存量金图零变化**为每批验收项——批 1 的 pAR 修正是唯一预判会「改变渲染行为」的点，A/B 归因（现窗 viewBox 16×16→方形 meet 恒等，理论零变化，实测确认；若实测有差逐像素归因后重采）。
- 双后端：webgpu(native) + gwgpu/gles 逐窗必跑；gwgpu-metal 本机不可验（darwin 交叉编译，按标记写明）。

### 6.3 风险与坑（提前挂账）

1. **dc.Image() 逐像素读回病**（2026-11-07 工程登记 B）：`svg.Render` 每次渲调 `dc.Image()`（svg/svg.go:47）整帧 flush+sync；批 5 SVG 表字形每字形一渲高频触发，**探针分钟级自旋风险实锤**。批 5 动工前先做已登记的 regionBytes 一行提升（④⑥ 同款待办），或在 P3.8 窗内改用 region 读回。
2. colr.go:177 假注释与 draw_emoji.go:70-73 双判断不一致，批 5 一并收口，不许留假语义。
3. GlyphEntry 穿链动 scene 编码 → recording/scene 对拍基线按拍板重采或证明等价。
4. SVG <text> 与 P3.3 的边界：复用 shaping/字体解析/逐 run 样式，**不复用 Paragraph 流式布局**（SVG text 是绝对定位 run 集）；render/text → render/svg 的 SVG 表字形路径与 render/svg → render/text 的 SVG <text> 路径**存在 import 环风险**，设计稿须定解（接口/回调单向化），这是批 3/批 5 设计稿的第一道题。
5. 负向对照随批列：摘 pAR 修正→变形钉红、摘 spreadMethod→重复/反射/填充三态钉红、摘 use 引用→引用元素消失钉红、摘 COLRv1 paint 求值→v1 字形退轮廓钉红、摘 DrawPicture 折叠→外层缺内容钉红、超深嵌套→哨兵红（不许栈溢出）。

---

## §7 待拍板清单（2026-11-07 用户拍板：D1–D7 全按推荐）

| # | 问题 | 拍板结果 |
|---|---|---|
| D1 | SVG 参考口径 | **定档「完整静态 SVG 画对」**：Skia modules/svg 语义锚 + resvg 核对清单 + 手写扩展不引第三方 Go SVG 库；render 层缺底座（如渐变 spreadMethod）在 render 层补，render/svg 只做解析与映射 |
| D2 | SVG 范围边界 | G1-G9/G11/G12/G13 全做；pattern（G10）做；marker/switch 做、a=g；**foreignObject/SMIL/script/跨文件 use 引用显式不做**（具名拒绝+登记） |
| D3 | 批次表 | **六批按序 1→6**（§5 表），批 6 可并行 |
| D4 | 彩色字形 vs P4.4 边界 | **A 档**：P3.8=四格式静态完整（COLRv1 静态求值在内，含两处假语义收口）；P4.4=变量轴 deltas+调色板动态 |
| D5 | K5 定夺 | **穿链**（GlyphEntry +4B Cluster，同批全量切换）；K3 维持在册不自动销案 |
| D6 | 嵌套 Picture 形态 | **Recorder.DrawPicture 通用命令 + 深度/环护栏**；CmdSceneReplay 保持不动 |
| D7 | 真窗方案 | **六扇新窗**（§5 表），每批一扇、该批收口 |

## §8 验证载体（登记 §2.1 用草案，随拍板定稿）

- 能力单测：随批新增（批 1 `render/svg/p38_*_test.go`…批 6 `render/recording/p38_*_test.go`，先红后绿；彩色字形批 `render/text/p38_*_test.go` + `render/text/emoji/*_test.go` 扩展；测试数据一律 `testdata/`，禁循环生成标准数据）。
- 真窗：六扇（§5 表），`-auto-only` 自终止 + 人工常驻两段式。
- 金图：各窗 `golden/*.png`，容差 0（P0.1 口径）；批 1 pAR 修正 A/B 归因条款见 §6.2。
- 双后端：webgpu(native) 逐窗实测；gwgpu-gles 逐窗实测；gwgpu-metal 不可验（darwin 交叉编译，标记写明）。
- 负向对照：§6.3 条 5 随批落地。
- 证据：`evidence/P3.8/`（本评估 + 随批 design/交付/门禁日志）。
