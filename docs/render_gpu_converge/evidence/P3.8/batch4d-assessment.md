# P3.8 批 4d：showcase 全面压测窗人工对照归因（currentColor / 刷子 opacity / 滤镜链 / 文字基线）

日期：2026-11-08。前序：批 4c（batch4c-assessment.md）交付后，用户对
`ui_wr_p38_svg_showcase` 人工对照两栏渲染报差异位置（编号 + 现象），本轮
逐位置像素归因、修复、复跑收口。

## 1. 归因方法

窗口左栏 = 本引擎 `svg.Document.Render` 烘焙（1024×640 1:1），右栏 = resvg
0.42 参照（同文件 `--use-font-file ahem.ttf` 钉死）。归因一律以**逐位置像素
探针**为准（Read 直看 PNG 对深色/透明图不可靠，4c 已两次误导）：临时探针
测试（跑完即删）输出 ours/ref 色值 + 逐行墨迹剖面 + 虚线 48 步相位图。

## 2. 用户报告逐条 → 实体 → 结论

| 用户报告 | 实体（文档坐标） | 结论 |
|---|---|---|
| ⑬ 砖上两个小方块全是黑的 | `<rect fill="currentColor" color=…>` ×2 @ (760/790, 600) | **洞 A**：currentColor 解析错序（§3.1） |
| ③ 星上红块没有后面边的蓝投影 | glowChain 滤镜的 rect（90..170, 400..480，压在星上） | **洞 C**：滤镜链两断点（§3.3） |
| ⑮..㉑ "都没有 / 窗口下面部分全是空的" | switch 内文字、正文 tspan 行、textPath、页脚 | **洞 E**：文字整篇静默丢弃（§3.5）——底部的"空"是文字行全缺 + 反射带无 opacity 的合成观感 |
| ⑳ 反射带（先前自确认） | `<rect fill=url(#reflectBand) opacity=0.55>` y 560..640 | **洞 B**：刷子填充丢元素 opacity（§3.2） |
| "横着的宽虚线，有渐变/灰白/黄/蓝" | **resvg 参照里的文字行**——Ahem 每个字母是实心方块，一行字=一条宽虚线；颜色=文字填充（标题渐变、标签灰白、tspan 黄/蓝、textPath 黄） | 非缺陷：参照系里文字本来长这样；本方修后同样呈现 |
| （探针自查）圆虚线 s≈74..88 缺口被填 | stroke-linecap=round 虚线圆 (540,520) r46 | 随 C/E 修复后**在全文档复测相位与 resvg 逐位一致**；隔离复现（含 round cap）本就正确 → 归入 K6 状态依赖族观察项，c4d 钉测锁相位 |
| ⑰ 虚线框 / ⑫ pattern / switch 块本体 / textPath | — | 与参照一致（textPath 被 starClip 裁剪双方一致），非缺陷 |

## 3. 五个引擎洞

### A. currentColor 在 fill/stroke 声明应用时就地解析（用 parent.color）

CSS 语义：currentColor 指向**本元素**计算后的 color。旧实现 applyDecl 在
级联遍历中就地解析 fill/stroke，`resolveColorToken(v, parent)` 查的是
parent.color —— 元素自己同带的 `color="#e24a4a"` 声明还没轮到 → 黑。
修复（inheritance.go）：evalProps 两段式 —— 第一段应用除 fill/stroke 外全部
声明（后写者胜、inline 压过表现属性），第二段用**本元素计算后的 color**（p
本身就是 parent.clone，未声明即继承值）解析 fill/stroke。红测
`TestP38C4DFillStrokeCurrentColor`（4 象限：自有 color ×2 + 继承 color 描边
+ 无 color 回黑）。

### B. 刷子填充/描边丢弃元素 opacity

`applyFillAttrs`/`applyStrokeAttrs` 实色分支经 setColorWithOpacity 折
`p.opacity`，**paint-server 分支直接 SetFillBrush 返回** —— 渐变/图案填充
+ opacity 全按不透明画。修复：新增公开口 `render.BrushWithAlpha(b, α)`
（包 opacityMulBrush，ColorAt 逐采样乘 α；与 F1 层机制同款），svg 层刷子
分支折算后传入。语义 = 单一填充时与组 opacity 等价（src-over 数学），
与既有"形状把 opacity 折进涂料"的设计注释同构。红测
`TestP38C4DBrushElementOpacity`（实心对照 + 渐变填充 + 图案填充 + 渐变
描边四行 alpha=127；pattern 行采样点避开第 4 块 stroke-width=20 的合法
叠涂带）。已知边界登记：fill+stroke 都有且互相重叠 + 元素 opacity 的
组合成语义（resvg 走组层）与逐涂料折算有理论差，本批不扩面（与既有实色
路径行为一致），留待 opacity 统一收口批。

### C1. feOffset dx/dy 错位

`parseFilterPrimitive` 的 feOffset/feDropShadow 专用分支**未提前 return**，
落入通用数字尾部又收集一遍 dx/dy —— `Numbers=[专用dy, 通用dx, 通用dy]`，
offsetDeltas 取前两个 = (dy, dx)。dx="14" dy="0" 实测渲染成 (0,+14)。
修复：专用分支改为按名读取（in/result/x/y/width/height/dx/dy/
stdDeviation）+ 显式 Skip + 早退；规范槽位 Number=dx、Numbers[0]=dy，
offsetDeltas/dropShadowParams 同契约（feDropShadow 的 dx 同病同修）。

### C2. feMerge 隐式首节点被丢

`<feMergeNode/>`（无 in）= 上一步结果（SVG 规范 / resvg resolve_input）。
旧解析只收集带 in 属性的节点 → 隐式首节点静默消失 → glowChain 的
泛光分支整体蒸发（梯次探针：flood→composite→offset→blur 全通，加
feMerge 后 blue=0）。修复：解析端保留 "" 哨兵，渲染端走与 fp.In 相同的
缺省规则（`__last__` 优先、否则 SourceGraphic）；feMerge 补读 result。

### E. 文字双重翻转 + 继承 font-family 断链（两洞叠加）

**E1 双重翻转（主犯）**：`ExtractOutline`（glyph_outline.go:713）早已把
轮廓转 **Y-DOWN**（"to match sfnt"，墨迹在基线上方 = 负 y；
color_glyph_render.go:358 注释即此契约），而 svg 层 `outlineToPathM` 按
过期的 Y-up 契约再翻一次（`fy := -py`）—— 双重翻转 = 没翻 = 全部文字
墨迹落到基线下方（resvg 为上方）。**此前从未被抓**：p38 文字钉测当年按
我们自己的输出采样（把反摆钉成了正解）；conformance 48×48 网格分级对
文字位移不敏感（实测 simple-case 移位 70px 仍 grade 5.06=close；修复后
同用例 1.57）。修复：svg 层撤二次翻转（消费 Y-DOWN，与 render 其余
消费者一致）。
**E2 继承 font-family 断链**：props 无 fontFamily 字段，flatten 只沿
text→tspan 链继承 —— 根 `<svg font-family="Ahem">` 到不了 span（渲染与
解析期校验两条路都断）→ 文字静默不渲染且无警告。修复：props + 级联
（applyDecl "font-family"）+ renderTextElement 对无家族 span 补级联值 +
解析期校验 walk 沿祖先线程传家族（presentation attr 或物化 style 里的
声明），无 resolver 匹配 → 具名警告（warnUnresolvedRef）+ fail-closed。
红测 `TestP38C4DInheritedFontFamily`（根家族渲染 / g 中间层 / 具名警告 /
不匹配不渲染）+ `TestP38C4DFilterWithTransform`（C1/C2）。

## 4. 窗口与验证

- **showcase 窗**：ensureBaked 挂 Ahem 解析器（svg 不扫系统字体，宿主
  接线，与 text 窗同口径）；探针 5→10（新增 chips×2 / 反射带上段 /
  标题渐变字 / 正文行）；logic 4/4 + pixel **10/10**；金图重种后
  native 验证与 go/GL 验证均 **0.0000%**。
- **text 窗**：8 个探针采样点从旧反摆盒重标定到标准盒（避 Ahem 中线
  镂空 y=B−0.55em..B−0.45em；渐变字探针改"蓝主导"语义断言）；logic
  6/6 + pixel **11/11**；金图重种（_base 是比对锚，重种须删旧 base 再
  种——本轮踩坑记录）native+go 均 **0.0000%**。
- **structure 窗**：logic 6/6 + pixel 20/20 + golden 0.0000%（无文字，
  零变化，交叉确认本批不伤 4c 能力面）。
- **conformance 全量（-font 真口径）**：**94.1%**（1225 id + 361 close +
  99 partial，零 far/blank/error）——**text 组 2 partial → 0（100%）**，
  其余组持平（filters 53 / masking 19 / painting 23 / shapes 4）。
- **svg 包全量绿 + -race 绿**；render 包回归：本批触及面零新增失败
  （TestLoadWebP_LosslessPixels / TestTextTransformAutoRoutesOutlines /
  video bridge 三者为干净树同败的先在项，归各自线；GPU 类用例需
  WGPU_NATIVE_PATH 环境）。
- 钉测：`render/svg/c4d_test.go` 5 枚（currentColor / 刷子 opacity /
  滤镜+transform / 继承家族+警告 / 圆虚线相位 48 步含 round-cap）。
- apidoc rc=0（catalog 新增 BrushWithAlpha 行）。

## 5. 遗留登记

1. fill+stroke 重叠 + 元素 opacity 的组层语义（§3.B 边界）→ opacity
   统一收口批。
2. 圆虚线 round-cap 缺口在 4d 修复前的全文档渲染中出现过一次、修复后
   不可复现 → 并入 K6 状态依赖观察项（同族：状态相关 mask 渲染）。
3. conformance 99 partial（filters 53/masking 19/painting 23/shapes 4）
   → 后续 4e 归因批。
4. pattern 内容独立 cascade 的根 font-family 继承（pattern tile 用
   rootProps 起底）——无消费者，登记不修。
