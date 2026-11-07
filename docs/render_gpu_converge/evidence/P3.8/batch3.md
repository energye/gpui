# P3.8 批 3 交付：SVG 文本（text/tspan）+ 字体 resolver 钩子

- 日期：2026-11-07（交付即收口：真窗双后端金图逐位）
- 依据：`assessment.md`（D1–D7）+ `design.md` §批 3 细化（import 环解法定案）

## 1 交付物

| 件 | 内容 |
|---|---|
| 依赖方向 | **实测 `render/text` 不 import render 主包** ⇒ svg→text 无环，批 3 直接依赖；批 5 反向（SVG 表字形 text→svg）届时接口倒置，本批不预留 |
| `text.go` | TextElement/TspanElement 布局+渲染：`text.Shape` shaping → 逐 glyph `OutlineExtractor.ExtractOutline` → outlineToPath（**FlipY**、逐字符 rotate 矩阵、**子路径边界 Close**）→ **逐段 fillAndStroke**（tspan 样式独立；union 会把段样式压平成一种才改逐段） |
| 字体钩子 | `SetFontResolver(fn)`（atomic 存储）：fn(family,weight,italic)→*text.FontSource；**svg 不做系统扫描**（归 ui/平台层）；无匹配 → Parse 期 Warning + fontInvalid 渲染期跳过（fail-closed，不在渲染期改 doc 状态——并发安全） |
| 属性继承 | tspan 未设字段从 `<text>` 继承（inheritTextAttrs：font-*/text-anchor/letter-spacing/fill 系/stroke 系/color） |
| clip/mask on text | 布局后按 **glyph bbox** 解析（布局前 bbox 不存在——renderClipped 的前置 bbox 契约对 text 不适用，text 走快速路径自管）；mask 内容腿用 span 自己的 paint（bare black Fill 会丢填充色的洞已修） |
| x/y/dx/dy/rotate 列表 | **段内字符索引**（每元素列表对自己的首字符生效，SVG 1.1）——首版错用全局索引被探针抓回；anchor 平移叠加在绝对 x 上 |

公开面：svg 顶层 39→**43**（TextElement/TspanElement/SetFontResolver/FontResolverFunc；catalog §6.4 补行 + §0 口径；apidoc rc=0）。

## 2 过程中抓出的实现洞（全部探针/快照对照抓回，如实记）

1. **outlineToPath 缺 Close**：开放 MoveTo..LineTo 链在扫描线栅格化下**零墨**（五变体对照实验定位——DrawRectangle/DrawPath/手工闭合全有墨，唯独无 Close 的 glyph 链没有）；Close 必须落在**子路径边界**（首版误放每 seg 后 → 每点自成零面积子路径，同样零墨）。
2. **parseTextLike 无条件 return textElem**：tspan 分支返回 nil —— tspan 收集全丢（探针 children=0 实锤）。
3. **renderElement 的 text 快速路径传 parent props**：p.fill=black（SVG 默认）——文字全黑，元素 fill 丢失。
4. **mask 内容腿 bare black Fill**：renderTextMasked 的 cb 直接 Fill 丢 span 填充色（白区文字黑块）——改 fillAndStroke 走 span 自己的 paint。
5. **UnitsObjectBoundingBox=枚举 0 零值陷阱**（批 2 pattern 同款）：maskContentUnits/patternContentUnits 缺省必须显式 userSpaceOnUse。

## 3 验证账（门禁全绿）

- **能力单测**：`render/svg` 全包 PASS（新增 p38_text_test 六枚：anchor 三态/tspan dx+x,dy/letter-spacing/rotate 逐字符/paint 管线红利（渐变填充+clip 裁剪文字）/missing fail-closed；AHEM 字体=每字符满 em 实心方块，锚点全精确）+ 批 1/2 全量回归；`-race` 绿。
- **负向对照**：摘 anchor 换算（探点关系红）/摘 rotate（未旋转位置有墨红）/缺字体（不画+Warning）随实现验证；批 1/2 的 9 条负向照旧全红。
- **apidoc** rc=0（catalog §6.4 批 3 行 + §0 43/39/29/15 口径链）。
- **conformance 双后端** PASS（batch3-conf.txt）。
- **真窗** `examples/ui_wr_p38_svg_text`（四卡：anchor 三态 / tspan+spacing / paint 管线（渐变+clip+mask on text）/ rotate+spacing+缺字体拒绝牌；AHEM 经 SetFontResolver 注册）：**native seed→verify + gles verify 同一基线 0.0000%**（917440px）、logic 6/6、pixel 11/11；探针迭代中四轮探点/期望自误如实记（快照对照逐一定性为探针错而非渲染错）。
- **存量回归**：批 2 窗 `ui_wr_p38_svg_paint` 金图 **0.0000% 逐位零变化**、`render/internal/gpu` 包绿、批 2 GPU focal 钉照旧绿。
- **抓屏门**：同批 1/2 口径不设（P3.5/P3.7 先例 + H18 在册）。

## 4 已知边界（如实）

- textPath（沿路径文字）、textLength 强制、writing-mode 竖排：具名拒绝+登记（设计 §3.2）。
- 字体匹配为 resolver 钩子内实现（本批无内置 fontdb）；系统字体扫描归 ui/平台层。
- mask 区域外文字全隐（mask 内容未覆盖处=遮出）——CSS Masking 语义 ✓。
- 渐变 stop 中段 GPU ramp 插值域 vs CPU 线性域差异照旧属 P4.2。
