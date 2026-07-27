# 07 · 文本 / Font / Paragraph

> **状态（组级）：** C 为主（RO 子集可用） · **Wave：** P0–P2 / 远期 IME · 修订：2026-07-28  
> 拆自 `ENGINE_UI_RENDER_BASE`；状态码见 [00_meta](./00_meta.md)  
> **施工真源分册** — 改状态先改本文件，再同步 [README](./README.md)

## 1. 目标 / 非目标

**目标：** 可测可绘文本；默认系统 UI 字体；上层可函数配置字体。  
**非目标：** 完整 Paragraph 富文本；IME 选区；库默认打进 CJK 全量字体包。  

**已落地字体策略（2026-07-28）：**  
- 库默认：`text.LoadDefaultFace` → **一个** 平台常用 UI 字体（`FontRoleUI`）  
- 跨平台候选：`system_font_linux.go` / `_windows.go` / `_darwin.go`  
- 配置（**无环境变量**）：`SetDefaultFontPath` · `SetSystemFontPaths` · `FontResolver.SetFontFile`/`SetPaths`  
- 多语可选：`LoadMultiFace` / `FontResolver.SetChain`（示例 `ui_render_base_text` 使用）  
- UI 薄封装：`rendering.TryLoadDefaultFace` / `TryLoadDefaultFaceWith`  

**组内顺序：** Measure→RO → Wrap → Font 策略 ✅ → Paragraph → ellipsis/maxLines → BiDi UI → IME。

## 2. 依赖（交叉关联）

| 方向 | 分册 / 说明 |
|------|-------------|
| **前置** | [02_paint_context](./02_paint_context.md) |
| **同级可并行** | 见 [91_wave_order](./91_wave_order.md) |
| **解锁** | 正文/列表/富文本编辑（后） |

## 3. 能力表（母表全文）

## §8 文本 / Paragraph / TextPainter 母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 | Wave |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|------|
| FT-PARAGRAPH-BUILD | 构建段落 | ParagraphBuilder | 富文本 | — | 无 ParagraphBuilder | RenderText 单串 | D | — | 富文本基座缺口 | P1 |
| FT-PARAGRAPH-LAYOUT | 段落布局 | Paragraph.layout | 换行高度 | 近似宽高 | MeasureString/Multiline | RenderText.Layout 启发式 | C/B | text.go · render/text.go | 接通 Measure 到 RO=P0 | P0 |
| FT-PARAGRAPH-PAINT | 绘段落 | drawParagraph | 正文 | DrawTextColored | DrawString* | RenderText.Paint | C | rendering/text.go | — | P0 |
| FT-PAINTER | TextPainter | TextPainter | 通用测量绘 | — | Measure+Draw 组合 | — | B | render/text.go | 可做 ui 门面 | P0 |
| FT-STYLE-SIZE | 字号 | TextStyle.fontSize | 层级 | FontSize 字段 | LoadFontFace points | RenderText | C | text.go | 未可靠设到 DC 字体 | P0 |
| FT-STYLE-FAMILY | 字体族 | fontFamily | 品牌/CJK | — | SetFont/LoadFontFace | — | B | render/text.go | RO 未接通 | P0 |
| FT-STYLE-WEIGHT | 字重 | fontWeight | 强调 | — | Face/variations 部分 | — | B | LoadFontFaceWithVariations | — | P1 |
| FT-STYLE-STYLE | italic | fontStyle | 斜体 | — | — | — | C/D | — | 依赖字体文件 | P1 |
| FT-LETTER-SPACING | 字距 | letterSpacing | 标题微调 | — | — | — | D | — | — | P2 |
| FT-WORD-SPACING | 词距 | wordSpacing | 英文 | — | — | — | D | — | — | P2 |
| FT-HEIGHT | 行高 | height | 多行节奏 | *1.25 近似 | lineSpacing 参数 | — | C | DrawStringWrapped | — | P1 |
| FT-STRUT | StrutStyle | StrutStyle | 多行稳定行盒 | — | — | — | D | — | — | P2 |
| FT-ALIGN | 对齐 | TextAlign | 标题居中 | — | Align @ Wrapped | — | B | DrawStringWrapped | 单行 RO 无 | P1 |
| FT-DIRECTION | 文本方向 | TextDirection | RTL | — | BiDi shaping 路径 | — | B/C | render/text | 未 UI 文档化 | P1 |
| FT-MAXLINES | 最大行数 | maxLines | 列表副标题 | — | — | — | D | — | — | P1 |
| FT-OVERFLOW | 溢出省略 | TextOverflow.ellipsis | 长文案 | — | — | — | D | — | 高频列表需求 | P1 |
| FT-LOCALE | locale | locale | 断行/字体 | — | — | — | D | — | — | P2 |
| FT-DECORATION | 下划线等 | TextDecoration | 链接 | — | text decoration 若有 | — | B/D | render | 需核对 | P1 |
| FT-SHADOW | 文字阴影 | shadows | 标题质感 | — | — | — | D | — | 滤镜近似 | P2 |
| FT-FG-BG | 前景/背景 Paint | foreground/background | 镂空字 | — | StrokeString / 底 rect | — | B | StrokeString | — | P1 |
| FT-BIDI | 双向文本 | BiDi | 阿语/混排 | — | shaper BiDi | — | B | render/text | — | P1 |
| FT-SEL-BOXES | 选区盒 | getBoxesForSelection | 输入选区 | — | — | — | D | — | 编辑器后置 | 远期 |
| FT-POS-FOR-OFFSET | 点击定位 | getPositionForOffset | 光标 | — | — | — | D | — | IME 后置 | 远期 |
| FT-LINE-METRICS | 行度量 | computeLineMetrics | 排版调试 | — | MeasureMultiline 部分 | — | B/C | render/text.go | — | P1 |
| FT-CJK | CJK 与回退 | 字体 fallback | 中日韩 | DrawText 可绘 | isCJKText/GPU/MultiFace | RenderText | C | render/text.go | 系统字体策略 | P0 |
| FT-WRAP | 自动换行 | softWrap | 段落 | — | WordWrap/DrawStringWrapped | — | B | render/text.go | RO 未用 | P0 |
| FT-SHAPED | 已整形 glyph | shaped glyphs | 性能/复杂文种 | — | DrawShapedGlyphs | — | B | render/text.go | — | P1 |
| FT-STROKE-TEXT | 描边字 | foreground stroke | 描边标题 | — | StrokeString* | — | B | render/text.go | — | P1 |
| FT-ANCHOR | 锚点绘制 | 对齐锚 | 居中标签 | — | DrawStringAnchored | — | B | render/text.go | — | P1 |
| FT-MEASURE | 测量宽高 | TextPainter.width/height | layout 真值 | — | MeasureString | RenderText 启发式 | B | render/text.go | P0 接通 RO | P0 |

---



## 4. 代码落点

| 层 | 路径 |
|----|------|
| RO | `ui/rendering/text.go`（Face/MaxWidth/Align/SetColor paint-only） |
| 默认字体 UI | `ui/rendering/default_font.go` |
| 系统字体 | `render/text/system_font.go` + `system_font_{linux,windows,darwin,other}.go` |
| shaper/MultiFace | `render/text/*` |
| 窗测 | `examples/ui_render_base_text`（多语 Multiface **示例侧**） |
| 测 | `system_font_test.go` · `font_multiface_test.go` · text measure 测 |

## 5. 指标挂钩

| 指标 | 说明 |
|------|------|
| M-LAYOUT-COUNT | SetColor 不 layout 风暴 |
| M-ATLAS-HIT/MISS | 上浮后（P1–P3） |
| M-FPS | text 轴 |

完整定义 → [90_metrics](./90_metrics.md)。**无 baseline 的优化无效**（[00_meta](./00_meta.md)）。

## 6. 验收 DoD

- [x] 单系统默认字体 + 函数配置  
- [x] Text 轴多语示例（LoadMultiFace）  
- [x] RenderText MaxWidth wrap  
- [ ] ParagraphBuilder（D/P1）  
- [ ] ellipsis / maxLines（D/P1）  
- [ ] 完整 BiDi 排版 UI  
- [ ] 选区/IME（远期）

## 7. 风险与非宣称

库**不**读 `GPUI_FONT*` env。阿文/希伯来字形可显示，排版可能仍 LTR。

## 8. 相关分册

- [总集 README](./README.md) · [实施顺序 91](./91_wave_order.md) · [指标 90](./90_metrics.md) · [F 交叉 92](./92_f_crosswalk.md)
