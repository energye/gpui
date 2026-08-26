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
| F-A1 | 光标渲染：竖条、行盒高、闪烁 ~500ms、失焦隐藏（S1） | 像素断言条盒=[baseline−ascent .. baseline+descent] |
| F-A2 | 偏移→光标矩形（任意偏移、行尾/空行/超长串）（S1） | 以真窗**实际绘制字形**为真值：条列落在相邻墨迹间隙内（允许压字的窄缝除外） |
| F-A3 | 点击→偏移（中点规则、换行感知）（S1） | 点 m 串第 k 字中部 → 落 k/k+1 边界；深部（30+ 字符）无漂移 |
| F-A4 | 左右键按簇移动（S1 复验） | CJK 不劈半字 |
| F-A5 | 上下键粘滞列移动（S1 迁移数据源） | 深部上下穿越列保持 |
| F-A6 | Home/End/PageUp/PageDown/Ctrl+←→ 词跳 | ⬜ S3 |
| F-A7 | **按键重复归一**：KeyRepeater 从 wayland_keyboard 迁 ui/input（主设计 D6 兑现），X11 服务端重复评估统一 | ⬜ S3（R5 补） |

### F-B 选区
| # | 需求 | 状态 |
|---|---|---|
| F-B1 | Shift+方向键扩展选区 | ⬜ S2 |
| F-B2 | 鼠标拖选 | ⬜ S2 |
| F-B3 | 双击选词/三击选段 | ⬜ S2 |
| F-B4 | 选区高亮（行盒矩形并集，来自 TextLayout） | ⬜ S2 |
| F-B5 | Copy/Cut/Paste 接线剪贴板 | 核心 ✅ / UI ⬜ S2 |

### F-C 编辑操作与编辑器形态
| # | 需求 | 状态 |
|---|---|---|
| F-C1 | 插入/前后删（rune 吸附） | ✅ |
| F-C2 | Undo/Redo（一次组合=一步，主设计 §4.4） | ⬜ S3 |
| F-C3 | 词删除 Ctrl+Backspace/Delete（对齐 Gio/Flutter 词边界） | ⬜ S3 |
| F-C4 | **只读态 ReadOnly**：Editor 开关 + 编辑操作全拒 + 控件只读样式；IME 会话照常可开但 commit 拒绝写入（对齐 Gio ReadOnly / Flutter readOnly） | ⬜ S3（N4） |
| F-C5 | **单行/多行模式建模**：单行=回车不换行、水平滚动、'
' 输入被拒；多行反之（对齐 Gio SingleLine / Flutter maxLines） | ⬜ S3（N5） |
| F-C6 | **提交动作**：Enter 触发 onSubmitted 回调 + TextInputAction 语义（done/go/search/send…映射 zwp keymap 无关的抽象动作），单行模式默认启用 | ⬜ S3（N6） |
| F-C7 | **IME 组合期回车语义分层**：单行模式组合中回车=提交组合原文（commit as-is，Flutter AddText 同径），非组合回车=触发提交动作；多行模式组合中回车=换行插入。须成文+单测（对照 Flutter engine AddCodePoint/EndComposing 分工） | ⬜ S1.5（R2 补） |

### F-D2 组合与选区交互语义（N1–N3 · 对齐 Flutter TextInputModel）
| # | 需求 | 现状→目标 |
|---|---|---|
| F-S1 | **组合起点语义**：开始组合时若存在选区 → 组合 span 从选区头（sel[0]）开始并覆盖选中段；无选区 → 从 caret 起。当前实现固定 sel[1]，属未定义行为，须改 | ❌→S1.5 |
| F-S2 | **组合期选区钳制**：组合期间选区（含点击/键盘造成的移动）钳制在组合范围内，不得跨出 | ❌→S1.5 |
| F-S3 | **commit 替换既有选区语义成文**：commit 原子替换「选中文本或组合 span」——当前 Insert 的实现恰好如此但属巧合，须写成 Editor 契约 + 单测锁定 | 🔶→S1.5 |

### F-D IME（协议层归主设计；此处只列与光标几何交叉项）
| # | 需求 | 状态 |
|---|---|---|
| F-D1 | preedit overlay 渲染（下划线/分段样式） | ⬜ S4 |
| F-D2 | 组合光标骑 preedit 内部 | ✅ |
| F-D3 | 候选锚点 = 光标矩形（同源） | ✅ 单源化后改读 TextLayout |
| F-D4~D7 | surrounding/purpose/delete_surrounding/多字段 | ✅ |
| F-D9 | **ContentType.Hint 建模落地（兑现已拖期的主设计 D8）**：Hint{Any,Text,Numeric,Email,URL,Telephone,Password…} 对齐 zwp content_hint / Gio InputHint；接口、平台透传（wl set_content_type hint 位）、purpose→hint 联动默认值 | ❌ S1.5（N7） |
| F-D10 | **锚点刷新自动闭环**：Editor.OnChange → InputRouter.RefreshIMEAnchor 自动接线，消灭「程序化改动（SetText 等）绕过路由导致锚点过期」隐患 | ❌ S1 随手做（N9） |
| F-D11 | **IME 主动改选区**（对齐 Gio SelectionEvent）：输入法可请求设置选区（如候选确认后的范围圈定）；入站词表补 IMESetSelection 或复用 IMEEvent 载荷 | ❌ S1.5（R2 补） |
| F-D12 | 三平台矩阵 | Wayland ✅ / X11 🔶(I3 复测) / Win ⬜ M2 / mac ⬜ M3 |

### F-E 控件层与外观（kit BaseEditable，主设计 §6.1 兑现）
| # | 需求 | 状态 |
|---|---|---|
| F-E0a | **BaseEditable 内嵌类型**：包办 Editor/IMERect/ContentPurpose/DrawPreedit 四件套，自定义控件 ≈15 行接入（主设计 §6.1 承诺） | ⬜ S2.5（N8） |
| F-E0b | 占位符渲染（空+未聚焦时显示 hint 文本，不进缓冲） | ⬜ S2.5 |
| F-E0c | 密码掩码渲染（PurposePassword 时圆点替换 + 关预测） | ⬜ S2.5 |
| F-E0d | 只读/禁用视觉样式 | ⬜ S2.5 |

### F-F 滚动与裁剪
| # | 需求 | 状态 |
|---|---|---|
| F-F1 | 输入/移动时自动滚动露出光标 | ⬜ S5 |
| F-F2 | 单行字段水平滚动 | ⬜ S5 |
| F-F3 | MaxLines/Ellipsis 与编辑态共存 | ⬜ S5 |

---

## 3. 分期实施（推翻式重写，不打补丁）

### S1 布局单源化（地基 · 含 N9 锚点闭环）

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

**随 S1 顺手做（N9）**：Editor.OnChange → RefreshIMEAnchor 自动接线（一行胶水 + 单测）。

**明确不做**：不改条宽/颜色/避让策略；不新增测量 API；不碰 IME 协议层。

### S1.5 组合×选区交互语义（N1–N3 语义正确性 · 紧跟 S1）
F-S1 组合起点=选区头并覆盖选中段；F-S2 组合期选区钳制；F-S3 commit 替换选区契约成文+单测锁定；
F-C7 组合期回车语义分层；F-D9 ContentType.Hint 建模落地（接口改传完整 ContentType，含 Hints 位）；
F-D11 IME 主动改选区入站事件。
门禁：选中一段文字直接打拼音→拼音替换选中段（真窗断言）；组合中点击钳制单测；
单行组合回车 commit-as-is 单测；hint 位协议日志断言。

### S2 选区体系
F-B1–B4；高亮矩形直接由 TextLayout 的行盒+边界 X 合成。门禁：拖选/双击/剪贴板互通真窗断言。

### S2.5 控件层 BaseEditable（N8 · 兑现主设计 §6.1）
F-E0a–d：BaseEditable 四件套 + 占位符 + 密码掩码 + 只读样式。
门禁：自定义控件接入 ≤15 行的示例编译+运行断言；密码框 purpose 日志 + 掩码像素断言。

### S3 编辑补全
F-C2/C3/C4/C5/C6、F-A6。门禁：undo 跨组合分组单测 + 词跳单测 + 只读态拒绝写入单测 +
单行回车触发 onSubmitted 断言。

### S4 IME 收口
F-D1 preedit 样式渲染；P2/P3/P8 真机销账；X11 I3 复测；M2/M3 盲写按 R-BLIND 纪律。

### S5 滚动露出
F-F1–F3。门禁：长文输入光标始终可见的真窗断言。

---

## 4. 验证纪律（防再犯）

1. **真值为绘制结果**：光标类像素断言以「实际画出来的字形墨迹」为参照物，禁止度量值自证
   （教训：离屏 harness 与度量路径同源导致假绿）；
2. **深部位置必测**：任何光标修复必须覆盖 ≥30 字符的深部边界；
3. **提交纪律**：每批改动停下报告，用户确认后才 git add/commit（AGENTS.md 硬约束）；
4. **单一来源审计**：CI/grep 层面禁止 caret/hit 路径出现 `MeasureWidth(`、`Advance(` 累加。

### 语义补充（R7 · 成熟 IME 行为对照后的明确化）

| # | 语义 | 定案 |
|---|---|---|
| S-1 | 失焦时活组合的处理 | 取消组合、缓冲不变（DetachEditor 现行为 ✓，成文锁定） |
| S-2 | 候选翻页/选词 | 合成器侧职责（zwp 架构），客户端只收 commit——非自绘候选窗的必然推论 |
| S-3 | preedit 样式映射 | Segments{Attr}→下划线/粗下划线/背景色三档客户端自绘（zwp 无样式位的协议天花板，见 R3） |
| S-4 | 多窗口实例粒度 | 每顶层窗口一个 adapter + 一个 ImeSession（主设计 §9 已定案 ✓） |
| S-5 | 组合中程序化 SetText/SetCaret | 终止组合再应用（与 Esc 同径），杜绝 overlay 与新偏移叠加的未定义态 |

### 非目标（本规范边界外）

- 平台自动填充框架（autofill/password autofill）——属系统凭据管理域，非输入法通道职责；
- 自绘候选窗（沿用主设计 §0）；
- IME 引擎本身/手写语音（沿用主设计 §0）。

---

## 5. 待用户复核（2026-08-26 补缺讨论后）

1. N1–N9 九项缺失是否仍有遗漏（本轮已对照 Flutter TextInputModel/Gio Editor/zwp 协议面盘点，多轮复查继续）；
2. F-S1–S3 组合×选区语义的 Flutter 对齐口径；
3. Hint 枚举值域（对齐 zwp content_hint 位 + Gio InputHint 的并集裁剪）。

---

## 6. 已定夺（2026-08-26 用户确认）

1. **九项缺失（N1–N9）成立**，按本文分期归属补入；
2. **分期与实现路线同意**：S1 布局单源化 → S1.5 组合×选区语义 → S2 选区 → S2.5 控件层 BaseEditable → S3 编辑补全 → S4 IME 收口 → S5 滚动；
3. **可推翻重写**：S1 范围内 RenderText 绘制/查询路径允许推翻式重写，不做兼容性补丁；
4. **多轮复查**：文档补入后继续逐组对照成熟框架复查，新缺失随查随补（复查轮记录见下）。

### 复查轮记录

| 轮 | 对照物 | 结论 |
|---|---|---|
| R1 | Flutter engine TextInputModel / Gio Editor / zwp_text_input_v3 协议面 | 发现 N1–N9 九项缺失，已补入 §2/§3 |
| R2 | Flutter TextInputPlugin（composing_rect_ + editabletext_transform_ 锚点机制）/ Gio InputHint+SelectionEvent / platform.ime.go 现状核对 | 发现：① ContentType.Hints 字段已存在但全链路断线（接口只传 Purpose）→ F-D9 细化；② Gio SelectionEvent「IME 主动改选区」缺失 → F-D11；③ 组合期回车语义未定义（单行=commit as-is vs 多行=换行）→ F-C7；④ Flutter 的 composing_rect+transform 锚点两件套已被 caretAnchor+IMERect 覆盖 ✓；⑤ autofill/password-autofill 属平台自动填充框架，超出输入法模块边界 → 记为非目标 |
| R3 | zwp_text_input_v3 全请求/事件面 vs 引擎 L0 覆盖；GTK4 GtkIMContext 方法面 | ① zwp 七请求（enable/disable/set_surrounding_text/set_text_change_cause/set_content_type/set_cursor_rectangle/commit）引擎全实现 ✓；② set_text_change_cause 目前写死 input-method——本地编辑时应报 other，属协议礼貌性偏差，挂 I 系列低优先级（不阻塞）；③ GtkIMContext 的 get_preedit_string/set_cursor_location/get_surrounding 拉取模式与我们的推模式（D2 受控供给）为同构两面，已覆盖 ✓；④ 协议天花板：zwp v3 无 preedit 样式位（样式只能客户端自绘），F-D1 实现时按 Segments 自绘下划线/着色，不依赖协议位 ✓ 可行 |
| R4 | Windows TSF/IMM32、mac NSTextInputClient 接口面（盲写前瞻） | TSF InputScope ≈ Purpose+Hints 并集 ✓ 建模可映射；mac firstRectForCharacterRange ← IMERect ✓；mac setMarkedText attributes → Segments ✓ 词表已预留。结论：现有抽象三端可承载，无需返工 |
| R5 | 快捷键面（全选/复制/剪切/粘贴/撤销重做键位）+ 焦点丢失时组合处理 + KeyRepeater 归一 | ① Ctrl+A/Z/Y/X/C/V 键位路由未成文——并入 S2/S3 各自门禁；② 失焦取消组合已实现（DetachEditor/Detach 清 comp）✓；③ D6 KeyRepeater 迁 ui/input 未做 → 补 F-A7（S3）；④ zwp set_text_change_cause 本地编辑应报 other——R3 ②的协议偏差在 S4 一并修 |
| R6 | 光标几何消费方全量清单 + Editor 焦点期数据同步 | 消费方=可见条/IME 锚点/点击命中/上下移动/未来选区高亮，五处全部声明走 TextLayout ✓；Gio Editor 有 OnFocusChange 时同步 Selection/Snippet 的语义——我们 AttachEditor 已带 field snapshot ✓；结论：无新增缺失，A1 单源化后自然收敛 |

