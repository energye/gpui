# P3.8 批 4c 影响面评估（2026-11-08，零代码）

> 立项：4b 四批收口后剩余窟的逐用例归因（总账 §2.1 批 4b-4 行「4c 待续」）。
> 范围：paint-servers 29 partial + structure 70 partial + 1 far + 5 error = **105 枚**。
> 方法：conformance -per-case 逐枚清单（/tmp/ps_perscase.txt、/tmp/st_perscase.txt）+ 全量语料自评分（限定名消歧，同名用例 basename 歧义已排除）+ 差异热力图 + 色值探针 + 源码逐枚钉因。探针数据在案（本批临时测试，用完即删）。
> 注意：Read 工具直看 PNG 对大面积透明图不可靠（本批两次误导），归因一律以像素探针为准。

## 1. 基线（4b-4 交付后真实口径，-font 带字体）

| 组 | total | id | close | partial | far | blank | error | pass% |
|---|---|---|---|---|---|---|---|---|
| paint-servers | 149 | 32 | 88 | 29 | 0 | 0 | 0 | 80.5% |
| structure | 247 | 134 | 37 | 70 | 1 | 0 | 5 | 69.2% |

## 2. structure 归因（10 洞 76 枚）

| 洞 | 根因（代码位置） | resvg 依据 | 用例数 | 用例（delta） |
|---|---|---|---|---|
| H1 未知元素整树丢弃 + `<a>` 容器缺失 | `parser.go:285` default 分支 `dec.Skip()` 丢整子树；usvg 对未知元素**不建节点但继续解析 children**（random 内 rect 被 use 引用渲染成功实锤）；`<a>` 是 usvg 已知容器（Group+链接元数据） | usvg parser mod.rs / links | 3 | a/on-shape 61.45、use/xlink-to-a-child-of-a-non-SVG-element 61.45、…-invalid-element 61.45（三枚我们 frame-only=该画没画/引用落空） |
| H2 switch/systemLanguage 条件处理未实现 | render/svg 全文无 switch/systemLanguage/requiredFeatures 命中；switch 整树不渲染；root svg 上 systemLanguage 不匹配也不隐藏 | usvg `parser/switch.rs`（语言 BCP47 匹配、直接子元素逐个验、第一个通过者渲染其余丢弃） | 14 | switch/* 12 枚全 61.45 frame-only + systemLanguage/on-svg 62.67（该隐藏没藏）+ systemLanguage/ru-Ru 61.45（该隐藏没藏） |
| H3 CSS 缺口 | `inheritance.go:50 supportedSelector` 只认 tag+单 class；`#id`/`[attr]`/组合选择器全拒；无 `!important`；无 `<style type>` 校验；`style` 属性/CSS 的 `transform` 属性不进 cascade | usvg `style.rs`（simplecss 全语义：id/attr/组合选择器、!important、type=text/css 校验、attribute presentation cascade） | 7 | style/attribute-selector 52.08（[x] 拒→fill 缺省黑）、combined-selectors 52.08、important 40.91（!important 拒→黑）、invalid-type 61.45（type=no-a-css 未校验→样式误应用红）、style/transform 53.15 + style-attribute/transform 53.15（CSS transform 属性不生效）、use/cSS-rules 61.45（#id 拒→继承红） |
| H4 image 数据源家族 | `parser.go:1001` 非 data: URI 具名拒绝（外链全死）；`parser.go:1058` image/svg+xml 具名拒绝（SVG 图像源死）；mime 表无 image/jpg 别名/无 mime 嗅探；width/height 缺省与 auto（SVG2 固有尺寸）未实现；image 上 preserveAspectRatio 对 SVG 源未装配 | usvg `parser/image.rs`（相对路径解析、SVG 嵌套文档、auto=intrinsic、pAR） | 28 | 外链栅格 3（external-png/jpeg/gif ≈100）+ SVG 图像源 6（embedded-svg/svgz/without-mime、external-svg/svgz/with-transform ≈100）+ mime 变体 2（embedded-jpeg-as-image-jpg/without-mime ≈100）+ 缺省尺寸/auto 10（no-width/no-height/no-width-and-height ×2 组 + width-and-height-set-to-auto 100.37 + no-height-non-square 29.84 + float-size 98.87 + raster-odd 93.77 + far: no-width-and-height-on-svg 131.53）+ pAR-on-svg 7（24~51，依赖 SVG 源先通） |
| H5 嵌套 svg/symbol 尺寸与 overflow | 嵌套 svg 缺省 x/y=0、w/h=100% 未实现；% 按父 viewport 解析未实现；overflow=auto 裁剪未实现；symbol overflow=visible 不裁未实现 | usvg units.rs / svg 转换器 | 6 | nested-svg-with-viewBox 29.76、-with-relative-width-and-height 33.94、-with-viewBox-and-percent-values 42.23、-with-overflow-auto 53.67、svg/no-size 59.64（根 svg 无尺寸缺省 100%→按画布拉伸）、symbol/with-overflow-visible 46.18 |
| H6 use/symbol 家族 | use/symbol 上的 opacity 未生效（group opacity 应用于 use 实例）；symbol viewBox 建坐标系未实现（1:1 直画）；use 引用 `<svg>` 元素（当 symbol 处理：use w/h 做 viewport + svg viewBox 缩放/裁剪）；display:none on use 不隐藏 shadow tree | usvg `use_node.rs` / symbol 转换 | 8 | opacity-on-use 30.65、opacity-on-symbol 30.65、opacity-on-use-and-symbol 46.05、opacity-on-symbol-with-viewBox 29.66、symbol/with-viewBox 31.63、xlink-to-svg-element-with-viewBox 62.79、-with-width-height-on-use 27.34、use/display-inheritance 61.45 |
| H7 transform-origin 未实现 | render/svg 全文零命中；rotate 绕原点而非指定点。全族 14 枚靠元素小落在 close 容差内（3~23），4 枚超界 | SVG2 transform-origin（usvg 支持：center/left/top/length/%，按元素 bbox） | 4（另 10 枚 close 可提升） | on-image 24.89（image 旋转出画布→空）、on-clippath-objectBoundingBox 43.64、on-pattern-object-bounding-box 40.43、on-pattern-user-space-on-use 35.36 |
| H8 XML 命名空间 | 只按 Name.Local 匹配；`xmlns=` 默认 ns 重绑定后无前缀子元素应按非 SVG ns 丢弃（rect2 不画）、带前缀 s:rect 应归属 SVG ns 渲染 | usvg svgtree 命名空间校验 | 1 | svg/xmlns-validation 61.45（我们把非 SVG ns 的 rect2 画了红） |
| H9 DOCTYPE 内部实体 | Go encoding/xml 不解析 DTD 内部子集：`<!ENTITY Rect "...">` + `&Rect;`（元素级）与 `&fill_value;`（属性级）→ Parse 整文档 error | XML 1.0 内部实体替换 | 4 error | elements-via-ENTITY-reference-1/2/3、attribute-value-via-ENTITY-reference |
| H10 非 UTF-8 编码 | xml.Decoder.CharsetReader nil：`encoding="Windows-1251"` → error。x/text v0.40.0 已在 go.mod | XML encoding 声明 | 1 error | svg/not-UTF-8-encoding |

## 3. paint-servers 归因（7 洞 29 枚）

| 洞 | 根因 | resvg 依据 | 用例数 | 用例（delta） |
|---|---|---|---|---|
| P1 stop offset 缺省与钳制 | offset 缺省=0；**递减钳制**（offset<前一 stop → 取前一值，spec 语义）未实现 | usvg paint_server.rs stop 合法化 | 4 | missing-offset-2 24.61/-4 42.98/-7 41.56、stop-with-smaller-offset 33.68 |
| P2 stop-color/stop-opacity inherit | `stop-color="inherit"` 取 gradient 元素上的 stop-color（非继承属性一层查找，同 4c-0 flood-color inherit 修复模式） | usvg parse.rs resolve_inherit | 5 | stop-color-with-inherit-1~5（30.65~40.91） |
| P3 userSpaceOnUse 百分比 | gradientUnits=userSpaceOnUse 下缺省/百分比坐标按 viewBox 尺寸解析（linear 缺省 x1=0%→x2=100% 即 (0,0)→(200,0)）；我们按 oBB 或零处理 | usvg units.rs convert_user_space | 4 | linearGradient/gradientUnits=userSpaceOnUse 61.37、radialGradient/同 33.28、linearGradient/-with-percent 44.31（150×150 画布）、radialGradient/-with-percent 33.80 |
| P4 奇异 gradientTransform/patternTransform | `matrix(0 0 0 0 0 0)` 奇异矩阵：我们整个渐变不画（frame-only）；resvg 仍渲染（行为待实现时对源码确认：塌缩单色或跳过变换） | usvg converter + resvg 渲染 | 3 | radialGradient/invalid-gradientTransform 88.13、linearGradient/同 112.55、pattern/invalid-patternTransform 39.84 |
| P5 pattern 家族 | ①patternUnits/patternContentUnits **无效值回退缺省 oBB**（parser.go:1027 现映射无效→userSpaceOnUse，错）；②pattern x/y 平铺相位未实现；③patternContentUnits=oBB 内容按 fill bbox 缩放未实现；④嵌套画刷（pattern 内容里 url(#patt1)/url(#lg1)）未解析；⑤userSpaceOnUse 下 % 尺寸 | usvg paint_server.rs pattern 转换 | 6 | invalid-patternUnits-and-patternContentUnits 41.17、with-x-and-y 28.60、patternContentUnits=objectBoundingBox 25.39、pattern-on-child 27.15、nested-objectBoundingBox 24.97、patternUnits=userSpaceOnUse-with-percent 40.01 |
| P6 径向渐变语义 | r=0 → 渲染最后 stop 单色；fx/fy 超圆 → 焦点投射到圆上（focal correction）；spreadMethod=reflect 径向外推 | spec + usvg/resvg 渲染器 | 3 | zero-r 30.64、focal-point-correction 29.96、spreadMethod=reflect 27.66 |
| P7 xlink:href 渐变属性继承链 | `<linearGradient xlink:href="#other">` 未显式属性从被引用 gradient 逐属性继承（含跨类型 linear↔radial、复杂顺序） | usvg paint_server.rs href 合并 | 4 | linearGradient/attributes-via-xlink-href-complex-order 25.24、-from-radialGradient 24.01、radialGradient/-complex-order 27.82、-from-linearGradient 25.02 |

## 4. 分批方案（建议，待拍板）

| 批 | 内容 | 洞 | 枚数 | 依赖/风险 |
|---|---|---|---|---|
| 4c-1 | 解析器鲁棒：ENTITY 内部子集 + Windows-1251 + 命名空间校验 + 未知元素子树保留 + `<a>` 容器 | H9 H10 H8 H1 | 9（5 error + 4 partial） | 无渲染交互，纯解析层；未知元素语义变更需过全量回归防误伤 |
| 4c-2 | CSS 补全：#id/[attr]/组合选择器 + !important + type 校验 + CSS transform 属性 | H3 | 7 | simplecss 语义对齐 usvg style.rs；cascade 优先级表扩展 |
| 4c-3 | switch/条件处理：switch + systemLanguage/requiredFeatures/requiredExtensions + 语言匹配 | H2 | 14 | 独立；usvg switch.rs 逐字 |
| 4c-4 | paint-servers 语义：stop 钳制 + inherit 关键字 + userSpaceOnUse % + 奇异 transform + 径向三语义 + xlink 继承链 | P1 P2 P3 P4 P6 P7 | 22 | 渐变渲染路径改动，金图必须复跑 |
| 4c-5 | pattern 补全：无效值回退 + x/y 相位 + ContentUnits=oBB + 嵌套画刷 + % 尺寸 | P5 | 6 | pattern 渲染路径 |
| 4c-6 | image 数据源：**外链解除具名拒绝（需拍板）** + SVG 图像源递归渲染 + mime 变体 + auto/缺省尺寸 + image pAR | H4 | 28（27p+1far） | **拍板点**：D2 具名拒绝解除要加 Parse base-dir 概念（API 面）；SVG 图像源=递归渲染装配，工作量最大 |
| 4c-7 | 嵌套 svg/symbol/use 家族：缺省尺寸/% 解析/overflow 裁剪 + opacity on use/symbol + symbol viewBox + use→svg + display:none on use + transform-origin | H5 H6 H7 | 18 | use/symbol 渲染路径；transform-origin 全族顺带提升 |

预期：paint-servers 80.5%→~100%、structure 69.2%→~95%、全量 87.6%→**~94%**（+110 枚 id/close）。

## 5. 验证载体

- 红测先行：每洞至少 1 枚语料样本钉测（入 `render/svg/testdata/p38/`，命名 `c4-<簇>-<case>.svg` + 期望 PNG 从语料拷贝）+ 逻辑探针；负向变异每批至少 1 枚。
- conformance：`scripts/svgconformance` 单组逐批复跑（-per-case），按批回填数字。
- 金图：ui_render_base_svg / p38_svg_filter / p34_filters 复跑（渐变/pattern/CSS 路径被触及）。
- apidoc：若 4c-6 拍板加 base-dir API → catalog 同步 + rc=0。
- 临时诊断测试 5 枚（tmp_c4diag/tmp_c4map/tmp_c4probe/tmp_c4color/tmp_c4selfgrade + 残留 tmp_tp2）收口前全删 + grep 插桩清零。

## 6. 风险与拍板点

1. **H4a 外链解除具名拒绝（4c-6）**：D2 历史决策（跨文件引用拒绝）。解除需要 Parse 侧加 base-dir（相对路径基准）与文件读取边界（只读本地相对路径）；API 面变更走 catalog。**用户拍板**。
2. H1 未知元素子树保留改变既有拒绝行为（foreignObject 等具名拒绝面）——实现时逐 usvg 对齐哪些元素递归哪些整体丢，全量回归防误伤。
3. H4b SVG 图像源是递归渲染（文档嵌文档），有环引用风险（resvg 怎么断环实现时查）——fail-closed 跳过 + 计数。
4. CSS transform 属性进 cascade 会触及 transform 解析顺序（style 属性 vs transform 属性 vs transform-origin 的应用顺序），resvg usvg 的应用次序照抄。
5. P4 奇异矩阵的 resvg 具体行为（塌缩单色 vs 跳过变换）实现时以源码为准，禁猜。

## 7. 批 4c 交付记录（2026-11-08，七批一次交付即收口）

用户拍板：7 批全做、批间不停等；外链解除具名拒绝（D2 修订，加 base-dir API）。逐批交付：

- **4c-1 解析器鲁棒（9 枚）**：DOCTYPE 内部子集实体（引号感知扫描+实体表+递归解析+替换，encoding.go）、Windows-1251/1252/latin1 CharsetReader（x/text charmap）、XML 命名空间校验（非 SVG ns 元素=惰性容器：不渲染、继承穿透、id 可被 use 引用——usvg id_map 重解析语义的树内等价物）、未知 SVG 元素同惰性容器、`<a>`=组（usvg "Treat links as groups"）、walkElements 补 ClipPath/Mask/Pattern 遍历（use 穿透引用 clipPath 内形状）。structure 69.2→72.9（error 5→0）。
- **4c-2 CSS 补全（7 枚）**：选择器解析器/匹配器（type/universal/#id/.class/[attr] 全操作符/:first-child/四组合器，css.go）、解析期物化（usvg parse_svg_element 同构：命中声明 transform 覆盖 transform 属性、其余并 style 属性 CSS 先原 style 后——三级级联自然成立）、**specificity 稳定排序**（simplecss lib.rs:219 sort_by_cached_key——rule-specificity 参照实证）、!important 剥离（usvg 忽略标志，参照红实证）、`<style type>` 校验、Attrs.Raw 属性快照供 [attr]。72.9→75.3。
- **4c-3 switch/条件处理（14 枚）**：FEATURES 白名单 30 项逐字、requiredExtensions 存在即败、systemLanguage 精确+前缀匹配（用户语言 ["en"]=resvg 缺省）、switch=取第一个过条件子元素（条件与 display 分离：选中 display:none 渲染为空**不回退**）、条件失败连子树隐藏且 use 不可达、根 svg 条件失败整文档隐藏。75.3→80.6。
- **4c-4 paint-servers 语义（22 枚）**：stop offset 缺省=前值/异单位回退/三修后处理逐字（convert_stops）、stop-color/opacity inherit 一层、<2 stops 单色、r≤0 末 stop 单色、奇异/无效 gradientTransform→identity（参照实证）、href 链属性继承（坐标限同类型/其余任意渐变/stops 走链）、linear x2 缺省 100%（userSpaceOnUse 按 viewBox）、**render 层两洞**：①渐变 stop 插值 linear→**premultiplied sRGB**（SVG 缺省 color-interpolation + tiny-skia 同口径，全部渐变受益，render 包测试/conformance 台 golden 同步重录）；②径向 focal 重写为 **Skia 两点 conical**（tiny-skia Focal stages 逐条：inside=唯一正根/outside=greater 根/无实根与 t≤0 透明掩码；svg 层撤 focal 预修正——resvg 不做）。98.7%。
- **4c-5 pattern（6 枚）**：patternUnits 缺省 oBB/patternContentUnits 缺省 userSpaceOnUse（两缺省不同）、无效值回缺省、tile 内容 1:1 无位移（resvg render_pattern_pixmap）、oBB 内容按 **fill bbox 尺寸**缩放无平移（to_user_coordinates "no need to shift patterns"）、x/y 锚定铺贴相位、嵌套画刷、%尺寸按 viewBox。**paint-servers 100%（127 id+22 close）**。
- **4c-6 image 数据源（28 枚）**：ParseWithConfig/ParseConfig{ResourcesDir}（catalog §3.22、apidoc rc=0）、外链解除具名拒绝（相对路径+魔数嗅探、URL scheme 仍拒绝）、data URI mime 别名 image/jpg、无 mime/text/plain 嗅探、SVG(Z) 子文档递归渲染（深度 ≤8，renderInternal 复用两步映射=图像 pAR fit+子文档 viewBox）、base64 空白剥离、SVG2 auto/固有尺寸缺省+单边等比、显式 0 尺寸跳过（撤渲染回退）。structure →95.1。
- **4c-7 嵌套 svg/symbol/use/display/transform-origin（12 枚）**：use/symbol/svg 的 opacity 组合成层（render.PushLayer 通道）、symbol 自身属性进级联、use→svg 符号语义（use w/h **设了才覆盖** svg 自身尺寸、svg x/y 偏移视口）、overflow 语义（visible/auto 不裁、缺省裁、viewBox-only 无 w/h 不裁——usvg get_clip_rect 逐条，corpus 实证缺省裁）、根 svg 无尺寸无 viewBox 按内容包围盒重定（calculate_svg_bbox）、**SVG2 transform-origin 落地**（T(o)·T·T(−o)，percent 按 viewBox——usvg resolve_transform 逐字；元素 transform/patternTransform/clipPath transform 三应用点；此前误判 resvg 不实现，源码实证后落地，on-image/on-clippath/on-pattern 全翻绿）。**structure 100%（231 id+16 close）**。

## 8. 终值与验证账

- **全量 conformance（-font 真口径）：87.6% → 94.0%**（1145 id + 439 close + 101 partial，**零 far、零 blank、零 error**）；4c 范围两组合计 **100%**（paint-servers 149=128+21、structure 247=231+16）；连带收益 painting 92.5%、shapes 96.9%、text 99.4% 保持。
- 钉测：4c-1 10 枚 + 4c-2 8 枚 + 4c-3 7 枚 + 4c-4 9 枚 + 4c-5 5 枚 + 4c-6 4 组 16 枚 + 4c-7 7 组 15 枚（语料+参照 PNG 入 testdata/p38/c4/，含 resources/），负向锚点 6 枚（未定义实体/不支持编码/未知元素不渲染/use→未知目标/条件失败不可达/无 ResourcesDir 外链）。
- 回归：svg 全包 rc=0、-race rc=0、render 包触及面（Conformance 台/COLRv1/Opt26）绿、internal/gpu、internal/filter、emoji 绿；render 包全量存在的 K6 干扰集合外零新增（干净树对照排除）。
- 金图：ui_render_base_svg 627200px / p38_svg_filter 912640px（logic 6/6 pixel 10/10）/ p34_filters 273600px 复跑全部 **0.0000%**（screen check 环境性沿用 4b-1 登记口径）。
- apidoc：catalog §3.22 补 ParseWithConfig/ParseConfig，rc=0。
- 登记遗留：text/masking/painting/filters 组 101 枚 partial 归后续批（4c 范围外）；用户语言固定 ["en"]（resvg 缺省同，可配置化挂后续）；focal+StartRadius≠0+off-center 组合未验证（现无消费者）；§6 风险 5（flood-opacity inherit）仍为登记 gap。
