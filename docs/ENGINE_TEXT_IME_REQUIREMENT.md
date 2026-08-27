# 文本编辑 + IME 生产级实现需求文档（统一真源 v3.2 · 完全对齐 Flutter）

> **唯一真源**。按 Flutter 真实现对齐重构，行为与 `engine/src/flutter/shell/platform/common/text_input_model.{h,cc}` / `text_editing_delta.{h,cc}` / `linux/fl_text_input_handler.cc` / `android/TextInputPlugin.java` / `darwin/FlutterTextInputPlugin.mm` / `lib/src/services/text_input.dart + editable_text.dart` + `Flutter TextPainter` 一致。
> **工业级**：三端可投产、单测+仿真+真窗像素三级门禁。

---

## 1. 目标与非目标

### 1.1 目标
1. 文本编辑与 IME 一体可用：布局单源、光标精确、选区完整、编辑可撤销、中文/日文/韩文可用，英文直通；
2. 完全对齐 Flutter 模型：`TextInputModel` 四元组 + `TextEditingDelta` 增量 + `TextPainter` 单源布局 + 平台 handler 行为一致；
3. 每窗口一个 `Editor`+`TextLayout`+`ImeSession`，焦点驱动，`Window.IME==nil` 静默降级；
4. 可验证：单测全转移+仿真+真窗像素（以实际绘制墨迹为真值）。

### 1.2 非目标
- 自绘候选窗、手写/语音、输入法引擎本身；
- RTL 特殊布局（DeleteSurrounding 按逻辑序）；
- 图素簇本轮 rune 吸附（API 预留簇升级）；
- 平台自动填充 autofill。

---

## 2. 术语

| 词 | 定义 |
|---|---|
| `text` | 完整文本**含 preedit**（Flutter `TextInputModel.text_`） |
| `selection` | `TextRange(base,extent)`，`collapsed` 为光标；含 `affinity`（下游/上游，换行处上下游归属） |
| `composing_range` | 标注 preedit 在 `text` 的区间，`collapsed` 表示无组合；`composing` 布尔与 range 分离 |
| `editable_range()` | `composing ? composing_range : text_range()`，所有编辑限于此区间 |
| `delta` | `TextEditingDelta(oldText, deltaText, deltaStart/End, selection, composing)`，`deltaStart==-1` 为 `NonTextUpdate` |
| `TextLayout` | 单源布局结果 `Lines{Carets{X,ByteOff}, Width}`，唯一几何源 |
| `surrounding` | `text` 全文 + `selection.extent` 的 UTF8 byte 偏移，供 IME 拉取；Wayland `set_surrounding_text` 的 `-1` 表示 NUL 结尾 |
| `batchDepth` | `beginBatchEdit/endBatchEdit` 嵌套深度，`>0` 时抑制通知 |

> 偏移：内部以 UTF16 code unit 记（Flutter 同），Go 侧 `string` 持有 UTF8，边界处 `Utf8ToUtf16/Utf16ToUtf8` 互转；`GetCursorOffset = Utf16ToUtf8(text[0:selection.extent]).size()`。

---

## 3. 对标 Flutter

| Flutter 模块 | 关键机制 | 本引擎对应 |
|---|---|---|
| TextPainter | `layout()` 一次产出 GlyphInfo，`getOffsetForCaret/getBoxesForRange` 只读结果 | A1 TextLayout 单源 |
| TextInputModel | `text+selection+composing_range+composing` 四元组，`UpdateComposingText` 进缓冲，`editable_range` 限编辑 | §6 Editor |
| fl_text_input_handler.cc | 6 信号+filter_keypress 优先+retrieve-surrounding 拉取+set_editing_state 覆盖 | §7 平台 |

**既有基建**：`text.Shape` / `DC.DrawShapedGlyphs` / `WrapText` / `ShapeResultCache` 已有，仅需接入。

---

## 4. 架构决策（硬约束）

### A1 单一文本布局源（最高优）

`ui/rendering/text_layout.go`：
```go
type GlyphCaret struct { ByteOff int; X float64 }
type TextLayoutLine struct { StartByte, EndByte int; Carets []GlyphCaret; Width float64 }
type TextLayout struct { Lines []TextLayoutLine; FontSize float64 }
```
生成：`WrapText(text, face, maxW) → per line text.Shape → Cluster(runeIdx)→ByteOff + X/XAdvance → Carets`，行宽=末字形 X+XAdvance。

**纪律**：`Paint` 逐行 `DrawShapedGlyphs` 与查询读同一份结果；光标/点击/锚点/高亮**全部**查 Carets；`SetText/SetFace/FontSize/MaxWidth/LineSpacing` 任一变→缓存置空；禁止 caret/hit 路径出现 `MeasureWidth/Advance` 累加（CI grep）。

### A2 光标模型

- 位置=字节偏移（rune 边界）；矩形 `x=Caret.X, y=[baseline−ascent, baseline+descent]`，`baseline=fs+lineIdx*lineHeight`；
- 宽 1–2px，允许压字形靠对比色；点击中点规则（advance 中点左归左、右归右）；
- 粘滞列 `caretCol/caretColValid`：首次上下键采 `penX`，水平移动/点击/SetCaret 清；
- 换行处 `affinity` 决定光标归上行尾还是下行头，`TextRange` 携带 `affinity` 参与 `getOffsetForCaret`。

### A3 图素簇

移动按簇吸附，本轮 rune 实现，API 预留簇升级位。偏移统一以 UTF16 记，Go 侧与平台边界处换算，surrogate 对按 2 计、rune 按 1 计的差在 `RuneCount` 校准中消除。

### A4 Editor 即 TextInputModel

`Editor` = Flutter `TextInputModel` 四元组，`text` 含 preedit，所有编辑限 `editable_range()`，`selection` 非 `collapsed` 时在 `composing` 下直接拒绝，见 §6。

---

## 5. 功能需求全集（可测试验证）

> 每行均有验收要点，单测或真窗必达。

### F-A 光标与定位

| # | 需求 | 验收 |
|---|---|---|
| F-A1 | 竖条、行盒高、闪烁 ~500ms、失焦隐藏 | 像素断言条盒=[baseline−ascent..descent] |
| F-A2 | 偏移→矩形（任意偏移/行尾/空行/超长 36×m，含 affinity 下游/上游） | 条落在实际绘制墨迹间隙（窄缝允许压字），深部无漂移 |
| F-A3 | 点击→偏移（中点规则、换行感知，返回 affinity） | 点 m 串第 k 字中部→k/k+1 边界，byte 2..35 抽样 ≥8 点 |
| F-A4 | 左右键按簇移动，限 `editable_range()` | CJK 不劈半字，单测 |
| F-A5 | 上下键粘滞列，限 `editable_range()` | 深部上下穿越列保持，单测+真窗 |
| F-A6 | Home/End/PageUp/PageDown/Ctrl+←→ 词跳，限 `editable_range()` | 词边界单测 |
| F-A7 | 按键重复：仅 Wayland purego 无 GTK 时在 `ui/input.KeyRepeater` 自合成（rate/delay，修饰键不武装，失焦取消）；X11/Win/mac 委托系统/GDK，不归一 | 确定时钟单测 |

### F-B 选区

| # | 需求 | 验收 |
|---|---|---|
| F-B1 | Shift+方向键扩展选区，`composing` 时非 collapsed 直接失败 | `SetSelection` 失败单测 |
| F-B2 | 鼠标拖选，限 `editable_range()` | 拖选真窗 |
| F-B3 | 双击选词/三击选段，限 `editable_range()` | 词/段边界单测 |
| F-B4 | 选区高亮（行盒并集，来自 TextLayout） | 真窗矩形并集像素 |
| F-B5 | Copy/Cut/Paste 接剪贴板 | 真窗 |

### F-C 编辑与形态

| # | 需求 | 验收 |
|---|---|---|
| F-C1 | 插入/前后删（rune 吸附，限 `editable_range()`，Backspace/Delete 按 surrogate 对计） | 单测 |
| F-C2 | Undo/Redo（一次组合=一步） | 分组单测 |
| F-C3 | 词删除 Ctrl+Backspace/Delete，限 `editable_range()` | 词边界单测 |
| F-C4 | 只读态：编辑全拒，IME 可开但 `AddText` 拒绝写入；`input_type==NONE` 时 `hide` 直接 `focus_out` | 只读+NONE 单测 |
| F-C5 | 单行/多行：单行回车不换行、水平滚动、拒 '\n'；多行反之 | 单多行真窗 |
| F-C6 | 提交动作：Enter 触发 `onSubmitted` + `TextInputAction(done/go/search/send…)`，仅 `filter_keypress` 未命中且 `Return` 时触发 `performAction` | 回调单测 |
| F-C7 | 组合期回车分层：单行组合中回车=提交原文 commit as-is，非组合=onSubmitted；多行=换行 | 单测 |
| F-S1 | 组合起点：有选区时 `DeleteSelected()` 先删选中段再 `erase(composingRange)`，无选区从 caret 起 | 选中段拼音替换真窗 |
| F-S2 | 组合期选区钳制：任何 `SetSelection` 非 collapsed 在 `composing` 下失败；`MoveCursor*` 限 `editable_range()` | 钳制单测 |
| F-S3 | commit 替换语义：`AddText` 原子替换 `composing_range` 或 `selection`，`delta replace_range = was_composing?composing_before:selection_before` | 单测锁定 |

### F-D IME（完全对齐 Flutter）

| # | 需求 | 验收 |
|---|---|---|
| F-D1 | preedit 进缓冲：`UpdateComposingText(text, selection)` 替换 `collapsed?selection:composingRange`，`text` 含 preedit；`""` 且 `collapsed` 时 no-op，结束由 `EndComposing` | 单测 |
| F-D2 | 组合光标：`SetComposingRange(range, cursorOffset)` 需 `composing==true`，`selection=range.start+cursorOffset` | 真窗 |
| F-D3 | 锚点：`composing` 时必报 `set_cursor_rectangle`（`composing_rect*transform+width/height` 经 `translate_coordinates`），非 `composing` 时仅预热 `composing_rect/transform` 不报 | 日志断言：非组合期 0 上报，组合期 1 次/变更 |
| F-D4 | surrounding 拉取：仅 `retrieve-surrounding` 时 `gtk_im_context_set_surrounding(text,-1,GetCursorOffset)`，`-1` 为 NUL 结尾；Wayland `set_surrounding_text` 限 4000 bytes 含 NUL，以光标为中心截断 | 拉取单测，截断单测 |
| F-D5 | deleteSurrounding：`offset/count` 按 code point（surrogate 对计 1，Go 侧 rune 计 1，在 UTF16↔UTF8 换算中统一），`selection` 仅 `offset<=0` 时移至 `start`，`composingRange.end-=len` | 单测 |
| F-D6 | purpose/hint 完整：`inputAction/enableDeltaModel/inputType→GtkInputPurpose/Hints`，`TextInputConfiguration` 含 `autofill` | 映射表单测 |
| F-D7 | 多字段/多窗口：每窗口一 handler，`show/hide` 按 `input_type==NONE` 切 `focus_in/out`，首焦 `focus_out` 预热（`fl_text_input_handler_new:454`） | 双窗互不影响，首焦即弹 |
| F-D8 | `set_editing_state` 覆盖：`SetText(text)` 一次→`selection -1/-1→0,0`→`SetText` 二次覆盖→`SetSelection`→`composing -1/-1→EndComposing else SetComposingRange(range, selectionBase-composingStart)` | 覆盖单测 |
| F-D9 | Hint 位透传（Completion/Spellcheck/HiddenText 等） | 协议日志 |
| F-D10 | 锚点自动闭环：`OnChange→RefreshIMEAnchor`，程序化 `SetText` 也刷新（仅 composing 时实报） | 单测 |
| F-D12 | 三端矩阵：Wayland ✅ / X11 ✅ / Win M2 / mac M3 | P12 双引擎 |

> F-D11（IME 主动改选区）已删除：Flutter 无此通道，选区唯一来源为 `set_editing_state` 与本地 `editable_range` 操作。

### F-E 控件与外观

| # | 需求 | 验收 |
|---|---|---|
| F-E0a | `BaseEditable` 四件套 `Editor/IMERect/ContentType/DrawPreedit`，≤15 行接入 | 样板窗 |
| F-E0b | 占位符（空+未聚焦 hint，不进缓冲） | 真窗 |
| F-E0c | 密码掩码（PurposePassword 圆点+关预测，`composing` 禁止） | 掩码像素+日志 |
| F-E0d | 只读/禁用样式 | 真窗 |

### F-F 滚动与裁剪

| # | 需求 | 验收 |
|---|---|---|
| F-F1 | 输入/移动自动露出光标 | 长文真窗 |
| F-F2 | 单行水平滚动 | 真窗 |
| F-F3 | MaxLines/Ellipsis 与编辑态共存 | 真窗 |

---

## 6. 统一模型

### 6.1 TextRange & Editor（= TextInputModel）

```go
type TextRange struct { Base, Extent int; Affinity int } // start=min, end=max, collapsed/Contains/length
type Editor struct {
    text string; selection, composingRange TextRange; composing bool
    batchDepth int; lastFrameworkText string; lastFrameworkSel, lastFrameworkComp TextRange
    epoch uint64; caretCol float64; caretColValid bool; OnChange func()
}
func (e *Editor) SetText(text string, sel, comp TextRange, affinity int) bool
func (e *Editor) SetSelection(TextRange) bool // composing && !collapsed → false
func (e *Editor) SetComposingRange(TextRange, cursorOffset int) bool
func (e *Editor) BeginComposing()
func (e *Editor) UpdateComposingText(text string, sel TextRange)
func (e *Editor) CommitComposing()
func (e *Editor) EndComposing()
func (e *Editor) BeginBatchEdit(); EndBatchEdit() // batchDepth 嵌套，>0 抑制 OnChange/通知
func (e *Editor) DeleteSelected() bool
func (e *Editor) AddText(text string)
func (e *Editor) DeleteSurrounding(offset,count int) bool
func (e *Editor) Backspace() bool; Delete() bool; MoveCursorToBeginning/End/Back/Forward() bool
func (e *Editor) GetText() string; GetCursorOffset() int
func (e *Editor) TextRange() TextRange; EditableRange() TextRange
func (e *Editor) ShouldSkipFrameworkUpdate(text string, sel, comp TextRange) bool // 全相等跳过
```

### 6.2 TextLayout

见 A1，仅读 `Editor.text`（已含 preedit），`Paint` 与查询同一份 shape。

### 6.3 增量通道

```go
type TextEditingDelta struct { OldText, DeltaText string; DeltaStart, DeltaEnd int; Selection, Composing TextRange; Affinity int }
type TextInputConfiguration struct { InputType, InputAction string; EnableDeltaModel bool; AutofillHints []string }
```

`enableDeltaModel` 在 `SetClient` 时定、运行中不可变；`deltaStart==-1` 为 `NonTextUpdate`，`apply` 以 `oldText` 为基底 last-write-wins；`TextRange` 携带 `affinity`。

---

## 7. 平台实现

### 7.1 Linux（`fl_text_input_handler.cc`）

`preedit-start→Begin`；`preedit-changed→get_preedit_string→UpdateComposingText+SetSelection+delta(oldText, composing_before, newPreedit)`；`commit→AddText+CommitComposing+delta(replace_range=was_composing?composing_before:selection_before)`；`preedit-end→EndComposing+NonTextDelta`；`retrieve-surrounding→GetText+GetCursorOffset→set_surrounding(text,-1,byteOffset)`；`delete-surrounding→DeleteSurrounding+delta`；`filter_keypress` 命中即 `return TRUE` 拦截，否则才处理 Home/End/Return(MULTILINE+newline→AddCodePoint('\n')+performAction)；`set_editing_state` 按 F-D8 二次覆盖；`show/hide` 按 `input_type==NONE` 切 `focus_in/out`，`new` 时 `focus_out` 预热；`setEditableSizeAndTransform+setMarkedTextRect` 双流驱动 `update_im_cursor_position`（仅 composing 时报）。

### 7.2 X11

`XSetLocaleModifiers→XOpenIM→XCreateIC(PreeditNothing)`，`XFilterEvent+Xutf8LookupString` 取 commit 后走 `AddText+CommitComposing`，剥 Rune；仅 Wayland purego 需 KeyRepeater，X11 委托系统。

### 7.3 Windows

`text_input_plugin.cc` 的 `TextHook/ComposeBegin/Change/Commit/End/KeyboardHook` 同 Linux；`enableDeltaModel` 分支同；`TYPE_TEXT_VARIATION_PASSWORD` 禁组合。

### 7.4 macOS/iOS

`FlutterTextInputPlugin.mm`：`setMarkedText→Begin+Update`，`insertText→AddText+Commit+End`，`unmarkText→Commit+End`，`firstRect` 仅 composing 时，`interpretKeyEvents` 双门模型。

---

## 8. 并发契约（硬）

C1 状态与 Editor 仅事件循环线程；C2 Timer 仅投队列+WakeUp；C3 adapter 标注线程语义；C4 Wayland done 队列 dispatch 独占。`batchDepth>0` 时抑制 `OnChange` 与 `updateEditingState`，`EndBatchEdit` 时一次性 `epoch++` 并按 `ShouldSkipFrameworkUpdate` 去重。

---

## 9. 控件接入

```go
type TextEditTarget interface { Editor()*Editor; IMERect() Rect; ContentType() ContentType }
type BaseEditable struct { ed *Editor }
func (b *BaseEditable) DrawPreedit(pc *PaintContext, text string, composing TextRange, segs []Segment)
```
≤15 行接入，`IMERect` 取 `TextLayout.CaretForOffset(selection.extent)`，`affinity` 参与换行处归属。

---

## 10. 测试与验收

### 10.1 单测

- `Editor` 全方法（含 `-1/-1`、affinity、`batchDepth` 去重）；`TextEditingDelta` 往返与 `NonTextUpdate`；6 信号序列+filter_keypress 命中拦截+增量一致性；`TextLayout` 36×m 5 位置+8 点点击；密码禁组合；`GetCursorOffset` 字节换算；4000 截断以光标为中心。

### 10.2 真窗矩阵

| # | 用例 | 判据 |
|---|---|---|
| P1 | 英文 10 键 | 恰一次插入 |
| P2 | 拼音→候选→上屏 | preedit 高亮，原子替换 |
| P3 | 组合中退格 | 缩拼音不删缓冲外 |
| P4 | Esc 取消 | composing 清 |
| P5 | 点击定位 | 最近边界，深部无漂移，affinity 正确 |
| P6 | 粘滞列 | 上下保持 |
| P7 | 密码 | 日志+掩码，无候选 |
| P8 | 切引擎 | 首焦即弹 |
| P9 | 单键 ≤2 preedit | 风暴锁 |
| P10 | 多字段切换 | 无残留 |
| P11 | 死键 | 重音正确 |
| P12 | ibus/fcitx | 均过 P1–P9 |
| P13 | 守护重启 | 恢复或降级 |
| P14 | 深部 36×m | 墨迹间隙 |
| P15 | 4000 截断 | 以光标为中心，不崩 |

证据：逻辑探针 JSON + 像素（以实际绘制墨迹为真值）+ `GPUI_IME_DEBUG`。

---

## 11. 分期（推翻重写）

| 期 | 内容 | 门禁 |
|---|---|---|
| R1 Editor 重构 | 四元组+affinity+batch+去重，按 UTF16 | 单测全绿 |
| R2 TextLayout 单源 | 新建 `text_layout.go`，Paint 改 DrawShapedGlyphs，Carets 查表 | P14/P5 真窗像素 |
| R3 通道与平台 | Delta+enableDeltaModel 定值，6 信号+surrounding 拉取+filter_keypress 命中拦截+二次覆盖 | 仿真+真窗 P2 |
| R4 选区与编辑 | F-B/F-C4 密码禁组合/F-C2-6/F-S/锚点预热+仅 composing 报 | 真窗拖选+hint 日志 |
| R5 控件与滚动 | BaseEditable+F-E+F-F | 样板窗 |
| M2/M3 | Win IMM32/TSF、mac objc 子类，复用同逻辑 | 双引擎 P12 |

---

## 12. 修订

| 版本 | 说明 |
|---|---|
| v3.1 2026-08-27 | 统一文本+IME，清理非 Flutter 引用 |
| **v3.2 2026-08-27** | 补齐 Flutter 缺失：TextRange affinity、Delta NonTextUpdate 定值、set_editing_state 二次覆盖与 -1 哨兵、GetCursorOffset/-1 与 4000 中心截断、filter_keypress 命中拦截、密码/NONE 禁组合、batchDepth 去重；修正锚点预热/实报、surrogate 换算、KeyRepeater 仅 Wayland purego、删除 F-D11 |

## 附录：偏移与截断

`TextRange` 以 UTF16 记，Go 边界 `Utf8ToUtf16/Utf16ToUtf8` 互转；`surrounding` 4000 bytes 超长以光标为中心截断；XIM PreeditNothing 接受无样式。
