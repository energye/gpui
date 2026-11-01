# P3.3 富文本逐 run 样式 影响面评估（2026-11-01）

> 任务：P3.3 富文本（逐 run 样式，前移项——不得挪回 P4）。PLAN §5.2 三行 + §6 DrawParagraph 行。依赖 P3.0 ✅。
> 对齐标准：Flutter `ParagraphBuilder`（PushStyle/AddText/Build）/ Skia `SkShaper::StyledRuns`（逐样式段 shaping、逐 run 提交渲染）。
> 调查：一路深查（排版结构/shaping 链/绘制分叉/ui 消费惯例/装饰字重/Justify/ellipsis 位置），HEAD 当前。

## 1. 关键现状

1. **样式模型在 ui 层已有雏形**：`ui/rendering/paragraph.go:21-49` 的 `TextRun{Text,Face,FontSize,RGBA,IsColor,Decoration}` + `ParagraphBuilder{PushStyle/PopStyle/AddText}`（自述非 Flutter 完整对齐）。P3.3=**样式模型下沉 render/text 层成为正源**，ui 后续切换消费。
2. **render/text 零样式**：`LayoutOptions`（layout.go:50-79）无样式字段；`ShapedRun/ShapedGlyph` 无颜色/装饰；Layout 输出 Lines/Glyphs/Advance 齐全（调用方可定位每 glyph）。
3. **逐 run 渲染改动面最小**：`DrawParagraph`（context_misc_draw.go:276）已是逐 run 循环调 `DrawShapedGlyphs`；颜色经 `c.currentColor()`（text_dispatch.go:167）per-call 捕获——**每 run 前 SetRGBA 即逐 run 颜色，GPU batch 的 color 本就是 per-call 参数**（SkParagraph 同为逐 run 提交）。不动 ShapedGlyph/accelerator 接口。
4. **Justify**：`AlignJustify` 退化点在 `applyAlignment`（layout.go:617-620）；空隙分配=改行内空格 glyph 的 XAdvance/X，**必须双写 `line.Glyphs` 与 `line.Runs[].Glyphs`**（applyAlignment:628-637 已示范双写模式）。
5. **ellipsis/maxLines**：LayoutOptions 无字段、wrapLinesWithMode 只贪心断行；最小改动=段循环末截行+末行「测宽收缩+省略号 Shape("…") 追加+整体右移」（ui 层 ellipsizeDisplayLine:449-503 可参照）。
6. **装饰**：`applyTextDecorations`（text_decoration.go:39-71）整串画线，只挂 DrawString 族；逐 run 装饰=DrawParagraph 按 run 的 Decoration+宽+基线画线（layout 输出有 run 宽/基线）。
7. **字重**：逐 run 走 `MultiFace.WithWeight(w)`（multi.go:364-384，wght 变轴/合成 embolden 已备）——每样式 run 用不同 face 实例，layout 的 face 切分天然接入。
8. **ui 消费惯例**：ui 文本主路（RenderText.paintRuns）目前逐 span `DrawString` 字符串 API，不走 Layout——**ui 迁移不在本步**（render 层能力先行+真窗验证，ui 切换另步）。
9. **测试基础**：layout_test.go 的 Alignment/MaxWidth/Bidi/GlyphPositions/Runs/MultiFace 钉可作等价基础；testdata 有 goregular/ahem/NotoSansSC-VF（变轴字重）/vazirmatn（RTL）。

## 2. 能力定义（草案）

**富文本逐 run 样式**：一个段落内逐段指定颜色/装饰/字重；支持两端对齐（Justify，空格分配）与 maxLines+ellipsis 截断；`DrawParagraph` 逐 run 渲染（颜色 per-run、装饰线 per-run、GPU/CPU 双路一致）。

- 第一版范围（拍板 D2/D3）：样式集=颜色+装饰（下划线/删除线/上划线）+字重；Justify=行内空格分配（最后一行不拉伸，Skia/Flutter 语义）；MaxLines/Overflow(ellipsis)。
- 不做（后续）：字号逐 run（face 尺寸派生复杂，第二版）、背景色、段落级 style（lineHeight/缩进）、ui paintRuns 迁移。

## 3. 公开 API 草案（拍板 D1）

```go
// render/text/paragraph.go（新）
type TextStyle struct {
    Color      render.RGBA   // a=0 视为未指定（继承 base）
    Decoration TextDecoration
    Weight     FontWeight    // 0=未指定（继承 base face）
}
type ParagraphBuilder struct{...}
func NewParagraphBuilder(face Face, base TextStyle) *ParagraphBuilder
func (b *ParagraphBuilder) PushStyle(s TextStyle)   // 栈式，Flutter 同构
func (b *ParagraphBuilder) PopStyle()
func (b *ParagraphBuilder) AddText(text string)     // 当前栈顶样式切分
func (b *ParagraphBuilder) Build(opts LayoutOptions) *Paragraph
type Paragraph struct {
    Layout *Layout            // 行/字形（现有结构）
    Styles []StyledSpan       // 逐 run：文本区间+face(带字重)+样式
}
```
- `Layout`/`Line`/`ShapedRun` 增加样式引用（Run.StyledIndex 或并行数组——实现细节倾向 ShapedRun 加 `Style int` 索引段）。
- `Context.DrawParagraph(layout, x, y)` **签名不变**：Layout 承载样式，逐 run `SetRGBA+DrawShapedGlyphs`+装饰线，结束恢复原 paint（注释已承诺不动 paint）。LayoutText 直造（无 Builder）的 Layout 样式索引=-1=当前 paint 色（零行为变化）。

## 4. 验证方案（草案）

- 能力单测（先红后绿）：①Builder 样式切分（Push/Pop/AddText 边界、跨段）②逐 run 颜色布局（run 索引/区间）③Justify（末行不拉伸+空格分配双写一致）④MaxLines/ellipsis（截断+省略号宽度）⑤DrawParagraph 恢复 paint（不漏色）⑥字重 run 的 face 派生（NotoSansSC-VF）。
- 真窗：`examples/ui_wr_p33_richtext`——三卡：多色多装饰段落（像素断言逐 run 颜色）、Justify 对齐（左右边缘齐行）、maxLines+ellipsis；双后端+金图+负向（摘 per-run SetRGBA→多色卡像素红→还原）。
- 双后端：glyph mask 管线（GPU）+vector/CPU 路（outlines）分别验；metal darwin 交叉。
- 回归：LayoutText 直造路径零行为变化（样式索引 -1）；per-file 改动面。

## 5. 拍板项

| # | 问题 | 推荐 |
|---|---|---|
| D1 | API 形态 | render/text 新 ParagraphBuilder/Paragraph（Flutter 同构，样式模型下沉正源）；DrawParagraph 签名不变由 Layout 承载样式 |
| D2 | 样式集第一版 | 颜色+装饰+字重；字号/背景/段落级样式后续 |
| D3 | Justify 语义 | 空格分配到非末行（Skia/Flutter 同）；双写两份 glyph 拷贝 |
| D4 | MaxLines/ellipsis | 进 LayoutOptions（MaxLines+Overflow 枚举 Clip/Ellipsis/Visible 对齐 Flutter） |
| D5 | ui 迁移 | 本步不做（ui paintRuns 切 DrawParagraph 另步）；render 层能力先行 |

## 6. 风险

- ShapedRun 加样式索引的兼容面（现有构造点多，零值=无样式向后兼容）。
- Justify 双写两份 glyph 拷贝的一致性（漏一份=显示与命中错位）——单测双断言。
- ellipsis 测宽与 shaper 一致性（Measure vs Shape 路径）——用 Shape("…")+Advance 累计而非 Measure 估算。
- 字重 run 每段落 face 解析成本——Builder 预解析缓存（同 weight 同 face）。
