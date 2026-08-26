# 文本编辑 + IME 完整功能规范（对标 Flutter TextPainter / SkParagraph）

> **性质**：真源（用户已定方向：对齐成熟框架、可推翻重写、不打补丁）。本文取代
> `ENGINE_INPUT_IME_PLAN.md` 中光标/文本几何相关结论；IME 协议层仍归
> `ENGINE_IME_MODERN_STANDARD.md`。
> **背景**：光标条定位问题多轮修补未根治（v1.9/v2.0/v2.4–v2.6）。根因不在某个公式，
> 而在架构：测量与绘制是两条独立路径。本规范以成熟框架为基准重立架构与功能全集，
> 分期实现，每期带像素级验收门禁。

---

## 0. 对标框架与结论来源

| 框架 | 关键机制 | 本引擎对应 |
|---|---|---|
| Flutter TextPainter | `layout()` 一次产出全部字形位置（TextLine/GlyphInfo）；`getOffsetForCaret`/`getBoxesForRange` 只读布局结果，不重新 shape | A1 TextLayout 单源 |
| Skia SkParagraph | SkShaper shape 一次 → TextBox 数组供查询；绘制走 drawTextBlob 消费同一份 shape 结果 | 引擎已有等价物：`text.Shape → []ShapedGlyph` + `DC.DrawShapedGlyphs`（ADR-022 shape-once），**但 RenderText 未接入** |
| Chromium Blink | ShapeResult 缓存；caret affine 位置从 shape 结果来；缓存随字体/尺寸失效 | ShapeResultCache 已有（S6.5）；补 RenderText 失效纪律 |
| VSCode / GTK | 窄字距下光标允许压字形，靠对比色而非几何避让 | 放弃「几何避让」类补丁 |

### 根因记录（为什么此前反复失败 · 审计实证）

- 绘制路径：`RenderText.Paint → DC.DrawString(s)`——DrawString 内部**自行 shape**
  （MultiFace 分 run → GPU 字形掩码管线，含 hinting/设备缩放各自的取整）；
- 测量路径：`RenderText.measureLine → Face.Advance 逐字符累加`（纯浮点求和）；
- 两条路径任何一环不一致，误差随字符数线性累积 → 「越靠后光标越偏」；
- 此前离屏验证假绿：离屏 harness 与度量路径恰好同源（同 scale、同管线分支），
  不代表真窗 GPU 管线；真值必须取自真窗实际绘制的像素；
- 一切「条宽/公式/避让」补丁都在错误的层面上打转，全部废弃。

### 引擎既有基建盘点（S1 直接复用）

| 基建 | 位置 | 说明 |
|---|---|---|
| `text.Shape(text, face) []ShapedGlyph` | render/text/shaper.go | shape 迭代器：每字形 GID/X/XAdvance/**Cluster(rune 索引)**，进程级缓存 |
| `DC.DrawShapedGlyphs(glyphs, face, x, y)` | render/text.go:213 | 预定位字形批量渲染（GPU 掩码优先，vector 回退）——ADR-022 shape-once 渲染出口 |
| scene 层先例 | render/scene/gpu_renderer.go resolveText | 已按「Shape→存 ShapedGlyph→DrawShapedGlyphs」运行，证明管线可行 |
| `WrapText(text, face, maxW, mode) []WrapResult{Start,End}` | render/text/wrap.go | 行字节区间（含 \n 归并），换行切分复用 |
| ComposedView / MoveCaretVertically / 点击命中 | ui/textinput、ui/rendering | 语义正确，仅数据源切换为 TextLayout |

---

## 1. 架构决策（审定后为硬约束）

### A1 单一文本布局源（最高优先级）

新增 `ui/rendering/text_layout.go`：

```go
// GlyphCaret 是一个可放置光标的边界（Flutter GlyphInfo+cursor 边界合成）。
type GlyphCaret struct {
    ByteOff int     // 在 Text 中的字节偏移（rune 边界）
    X       float64 // 该边界的笔位（行内逻辑 px）
}

type TextLayoutLine struct {
    StartByte int         // 行首字节偏移
    EndByte   int         // 行尾内容字节偏移（不含 '\n'）
    Carets    []GlyphCaret // len = rune 数 + 1（含行尾边界）
    Width     float64      // 末边界 X
}

type TextLayout struct {
    Lines    []TextLayoutLine
    FontSize float64
}
```

生成（唯一合法方式）：
1. `WrapText(text, face, maxW, WrapWord)` 切行（复用现有 UAX#14 换行）；
2. 每行 `text.Shape(lineText, face)` 得到 `[]ShapedGlyph`；
3. 由 ShapedGlyph 的 `Cluster`（rune 索引，经 line 内 rune→byte 映射转字节偏移）
   与 `X/XAdvance` 组装出 Carets 数组；行宽 = 最后字形 X+XAdvance。

消费纪律：
- **绘制**：`Paint` 改为逐行 `DC.DrawShapedGlyphs(shapedOfLine, face, 0, baseline)`
  ——与查询读同一份 shape 结果，双路径从结构上消灭；
- **光标矩形 / 点击命中 / IME 锚点**：全部改为查 Carets（二分或线性）；
- **删除**：`measureLine` 在 caret/hit 路径上的所有调用（普通测量如 measureSize 可保留）；
- **失效**：SetText/SetFace/SetFontSize/SetMaxWidth/SetMaxLines/LineSpacing 任一变更
  → layout 缓存置空，下次访问重建（挂接现有 MarkNeedsLayout 链）。

### A2 光标模型（标准语义）

- 光标位置 = Text 字节偏移（rune 边界）；
- 光标矩形：x=该边界 Caret.X，y=该行的 `[baseline−ascent, baseline+descent]`
  （baseline = fs + lineIdx×lineHeight，ascent/descent 取自 face.Metrics()）；
- 宽度固定 1–2px，**允许压字形**（窄字距字体下的业界常态），靠颜色对比保证可读；
  废弃一切几何避让特判；
- 点击命中：点在字形 advance 中点左侧归左边界、右侧归右边界；数据源=TextLayout。

### A3 图素簇（grapheme）

编辑移动按图素簇吸附；本轮 rune 实现，API 命名预留簇升级位（沿用主设计非目标声明）。

---

## 2. 功能需求全集

### F-A 光标与定位
| # | 需求 | 验收要点 |
|---|---|---|
| F-A1 | 光标渲染：竖条、行盒高、闪烁 ~500ms、失焦隐藏 | 像素断言条盒=[baseline−ascent .. baseline+descent] |
| F-A2 | 偏移→光标矩形（任意偏移、行尾/空行/超长串） | 以真窗**实际绘制字形**为真值：条列落在相邻墨迹间隙内（允许压字的窄缝除外） |
| F-A3 | 点击→偏移（中点规则、换行感知） | 点 m 串第 k 字中部 → 落 k/k+1 边界；深部（30+ 字符）无漂移 |
| F-A4 | 左右键按簇移动 | CJK 不劈半字 |
| F-A5 | 上下键粘滞列移动 | 已实现，S1 迁移数据源 |
| F-A6 | Home/End/PageUp/PageDown/Ctrl+←→ 词跳 | ⬜ S3 |

### F-B 选区
| # | 需求 | 状态 |
|---|---|---|
| F-B1 | Shift+方向键扩展选区 | ⬜ S2 |
| F-B2 | 鼠标拖选 | ⬜ S2 |
| F-B3 | 双击选词/三击选段 | ⬜ S2 |
| F-B4 | 选区高亮（行盒矩形并集，来自 TextLayout） | ⬜ S2 |
| F-B5 | Copy/Cut/Paste 接线剪贴板 | 核心 ✅ / UI ⬜ S2 |

### F-C 编辑操作
| # | 需求 | 状态 |
|---|---|---|
| F-C1 | 插入/前后删（rune 吸附） | ✅ |
| F-C2 | Undo/Redo（一次组合=一步，主设计 §4.4） | ⬜ S3 |
| F-C3 | 词删除 Ctrl+Backspace/Delete | ⬜ S3 |

### F-D IME（协议层归主设计；此处只列与光标几何交叉项）
| # | 需求 | 状态 |
|---|---|---|
| F-D1 | preedit overlay 渲染（下划线/分段样式） | ⬜ S4 |
| F-D2 | 组合光标骑 preedit 内部 | ✅ |
| F-D3 | 候选锚点 = 光标矩形（同源） | ✅ 单源化后改读 TextLayout |
| F-D4~D7 | surrounding/purpose/delete_surrounding/多字段 | ✅ |
| F-D8 | 三平台矩阵 | Wayland ✅ / X11 🔶(I3 复测) / Win ⬜ M2 / mac ⬜ M3 |

### F-E 滚动与裁剪
| # | 需求 | 状态 |
|---|---|---|
| F-E1 | 输入/移动时自动滚动露出光标 | ⬜ S5 |
| F-E2 | 单行字段水平滚动 | ⬜ S5 |
| F-E3 | MaxLines/Ellipsis 与编辑态共存 | ⬜ S5 |

---

## 3. 分期实施（推翻式重写，不打补丁）

### S1 布局单源化（当前期 · 地基）

**重写范围**（可推翻现有代码）：
- 新建 `ui/rendering/text_layout.go`：TextLayout 结构 + 构建（WrapText+Shape 组装）+ 查询 API：
  - `(t *RenderText) layout() *TextLayout`（惰性构建+失效）
  - `(l *TextLayout) CaretForOffset(byteOff int) (lineIdx int, x float64, ok bool)`
  - `(l *TextLayout) OffsetAtPoint(x, y float64, lineHeight float64) int`
- 重写 `RenderText.Paint`：单串路径逐行 DrawShapedGlyphs（runs 路径暂保留旧制，另立迁移项）；
- 重写 `RenderText.CaretColumn / ByteOffsetAtPoint`：改为查 TextLayout（签名不变，调用方无感）；
- 示例 `caretAnchor()` 不变（已单源），底层自动获得精确几何；
- 删除：caret/hit 路径上的一切 Advance 累加。

**出口门禁（全过才算完）**：
1. **真窗像素终验（用户场景复现）**：36×'m' 串，SetCaret 至 {3,10,20,30,35} 各帧快照；
   断言条中心列落在相邻两个**实际绘制字形**墨迹之间的空隙列内——以扫描出的墨迹列区间
   为真值，禁止用度量值自证；
2. 点击链多点回归（点 m 中部 → 相邻边界，byte {2..35} 抽样 ≥8 点）；
3. CJK/混合/composing 五场景快照不回归；
4. 全 ui 包测试绿；apidoc 门禁绿（新公开类型/API 入 RENDER_API_CATALOG）。

**明确不做**：不改条宽/颜色/避让策略；不新增测量 API；不碰 IME 协议层。

### S2 选区体系
F-B1–B5；高亮矩形直接由 TextLayout 的行盒+边界 X 合成。门禁：拖选/双击/剪贴板互通真窗断言。

### S3 编辑补全
F-C2/C3、F-A6。门禁：undo 跨组合分组单测 + 词跳单测。

### S4 IME 收口
F-D1 preedit 样式渲染；P2/P3/P8 真机销账；X11 I3 复测；M2/M3 盲写按 R-BLIND 纪律。

### S5 滚动露出
F-E1–E3。门禁：长文输入光标始终可见的真窗断言。

---

## 4. 验证纪律（防再犯）

1. **真值为绘制结果**：光标类像素断言以「实际画出来的字形墨迹」为参照物，禁止度量值自证
   （教训：离屏 harness 与度量路径同源导致假绿）；
2. **深部位置必测**：任何光标修复必须覆盖 ≥30 字符的深部边界；
3. **提交纪律**：每批改动停下报告，用户确认后才 git add/commit（AGENTS.md 硬约束）；
4. **单一来源审计**：CI/grep 层面禁止 caret/hit 路径出现 `MeasureWidth(`、`Advance(` 累加。

---

## 5. 待用户定夺（已并入 §3 分期建议）

1. S1 立即开工（建议：是）；
2. F-A6/F-C3 并入 S3（建议：是）；
3. 图素簇维持非目标（建议：是，CJK 不受影响）。
