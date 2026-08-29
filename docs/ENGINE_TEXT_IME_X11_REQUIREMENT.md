# 文本编辑 + IME X11 生产级实现需求文档（复用 Wayland 版 v3.14 · X11 完整版 v1.0 · 完全对齐 Flutter）

> **复用声明**：本文件为 `ENGINE_TEXT_IME_REQUIREMENT.md`（Wayland 主真源 v3.14）的 **X11 完整镜像**，`R1-R5` 功能（`F-A/B/C/D/E/F/S`）、`§6 统一模型`、`§8 并发`、`§9 控件接入`、`§10 测试与验收`（含 R1-R5 单测与真窗同源同数同判据、三证据、A-J 10族全硬）**全部直接复用 Wayland 版，禁止修改已可用的 R1-R5 测试**；差异仅在 `§7 平台实现` 重写为 **X11 XIM** 完整实现（`PreeditCallbacks/Position + XNSpotLocation` 框内预编辑），格式与 Wayland 版 §7 逐段对照。
> **工业级**：Wayland 与 X11 双平台可投产、单测+仿真+真窗像素三级门禁。

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
- 图素簇本轮 rune 吸附、API 预留 `ClusterFromByte()` 簇升级（韩文 11172 音节/泰文字簇走 `HbShaper`，本轮 CJK/latin 仅 rune）；
- 平台自动填充 autofill 的 UI（`TextInputConfiguration.autofill` 仅透传 `autofillHints`，不做填充界面）。

---

## 2. 术语

| 词 | 定义 |
|---|---|
| `text` | 完整文本**含 preedit**（Flutter `TextInputModel.text_`） |
| `selection` | `TextRange(base,extent)`，`collapsed` 为光标（`Base==Extent` 且非 `-1`）；含 `affinity`（`AffinityDownstream/Upstream`，换行处上下游归属） |
| `composing_range` | 标注 preedit 在 `text` 的区间，`TextRange{-1,-1}` 为哨兵（未指定），`collapsed` 且非哨兵表示无组合；`composing` 布尔与 range 分离（`composing==false` 时 range 必为哨兵或 collapsed） |
| `editable_range()` | `composing ? composing_range : text_range()`，所有编辑限于此区间，`MoveCursor*` 亦限此区间 |
| `delta` | `TextEditingDelta(oldText, deltaText, deltaStart/End, selection, composing)`，`deltaStart==-1` 为 `NonTextUpdate`（`IsNonTextUpdate()`），此时 `deltaText==""` |
| `TextLayout` | 单源布局结果 `Lines{Carets{X,ByteOff}, Width}`，唯一几何源 |
| `surrounding` | `text` 全文 + `selection.extent` 的 UTF8 byte 偏移，供 IME 拉取；GTK `gtk_im_context_set_surrounding(text,-1,cursor)` 与 Wayland `set_surrounding_text` 共用 `-1` 表示 NUL 结尾 |
| `batchDepth` | `beginBatchEdit/endBatchEdit` 嵌套深度，`>0` 时抑制 `OnChange` 与 `updateEditingState` |
| `epoch` | 去重用单调计数，`EndBatchEdit` 时 `epoch++` 并按 `ShouldSkipFrameworkUpdate` 判重 |
| `affinity` | `AffinityDownstream=0/Upstream=1`，参与 `getOffsetForCaret` 换行归属 |

> 偏移：内部以 UTF16 code unit 记（Flutter 同，surrogate 对按 2 计，`DeleteSurrounding` 的 `offset/count` 仍按 code point（rune）计，仅在 UTF8↔UTF16 换算时 ×2 校准 via `RuneCount`）；Go 侧 `string` 持有 UTF8，边界处 `Utf8ToUtf16/Utf16ToUtf8` 互转；`GetCursorOffset = len(Utf16ToUtf8(text[0:selection.extent]))`。

---

## 3. 对标 Flutter

| Flutter 模块 | 关键机制 | 本引擎对应 |
|---|---|---|
| TextPainter/SkParagraph | `layout()` 一次产出 GlyphInfo，`getOffsetForCaret/getBoxesForRange/computeLineMetrics` 只读结果 | A1 TextLayout 单源 |
| TextInputModel | `text+selection+composing_range+composing` 四元组，`UpdateComposingText` 进缓冲，`editable_range` 限编辑，`-1/-1` 哨兵 | §6.1 Editor |
| TextEditingDelta | `enableDeltaModel` 定值 + `NonTextUpdate(-1)` + `apply` last-write-wins | §6.3 Delta |
| fl_text_input_handler.cc | 6 信号+`filter_keypress` 优先+`retrieve-surrounding` 拉取+`set_editing_state` 二次覆盖 | §7.1 Linux |
| TextInputConfiguration | `inputType/purpose/hint/autofillHints/inputAction` → `GtkInputPurpose/Hints` | §5 F-D6 / §7 |
| Windows `text_input_plugin.cc` | `TextHook/ComposeBegin/Change/Commit/End/KeyboardHook`，`TYPE_TEXT_VARIATION_PASSWORD` 禁组合 | §7.3 |
| Darwin `FlutterTextInputPlugin.mm` | `setMarkedText/insertText/unmarkText/firstRect` 仅 composing 时，`interpretKeyEvents` 双门 | §7.4 |

**既有基建**：`text.Shape` / `DC.DrawShapedGlyphs` / `WrapText` / `ShapeResultCache` 已有，仅需接入。

---

## 4. 架构决策（硬约束）

### A1 单一文本布局源（最高优）

`ui/rendering/text_layout.go`：
```go
type GlyphCaret struct { ByteOff int; X float64 }
type TextLayoutLine struct { StartByte, EndByte int; Carets []GlyphCaret; Width float64 }
type TextLayout struct { Lines []TextLayoutLine; FontSize, MaxWidth, LineSpacing float64; Face Face; Generation uint64 }
```
生成：`WrapText(text, face, maxW) → per line text.Shape → Cluster(runeIdx)→ByteOff + X/XAdvance → Carets`，行宽=末字形 X+XAdvance；`Generation` 随 `SetText/SetFace/FontSize/MaxWidth/LineSpacing` 任一变而 `++` 并置空缓存。

**纪律**：`Paint` 逐行 `DrawShapedGlyphs` 与查询读同一份结果；光标/点击/锚点/高亮**全部**查 Carets；`SetText/SetFace/FontSize/MaxWidth/LineSpacing` 任一变→`Generation++` 缓存置空；禁止 caret/hit 路径出现 `MeasureWidth/Advance` 累加（CI `grep -r MeasureWidth ui/rendering/text_layout.go` 为空）。

### A2 光标模型

- 位置=字节偏移（rune 边界）；矩形 `x=Caret.X - scrollX, y=[lineTop+ascentLine−ascent, lineTop+ascentLine+descent]`，`lineTop=Σ lineHeight`，滚动时 `x` 减 `scrollX`/`y` 减 `scrollY`（单源 `TextLayout` 的 `lineTop` 来自 `computeLineMetrics`）；
- 宽 1–2px，允许压字形靠对比色；闪烁 ~500ms（`caretOn` 节拍），失焦隐藏；点击中点规则（advance 中点左归左、右归右）；
- 粘滞列 `caretCol/caretColValid`：首次上下键采 `penX`，水平移动/点击/`SetCaret`/Home/End 清；
- 换行处 `affinity` 决定光标归上行尾还是下行头，`TextRange` 携带 `affinity` 参与 `getOffsetForCaret`。

### A3 图素簇

移动按簇吸附，本轮 rune 实现、API 预留 `ClusterFromByte()` 簇升级位。偏移统一以 UTF16 记，Go 侧与平台边界处换算，surrogate 对按 2 计、rune 按 1 计的差在 `RuneCount` 校准中消除；`DeleteSurrounding` 的 `offset/count` 仍按 code point（rune）计，仅换算时 ×2。

### A4 Editor 即 TextInputModel

`Editor` = Flutter `TextInputModel` 四元组，`text` 含 preedit，所有编辑限 `editable_range()`，`selection` 非 `collapsed` 时在 `composing` 下直接拒绝（含 `MoveCursor*`/`SetSelection` 均限 `editable_range()`），见 §6。

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
| F-A7 | 按键重复：仅 Wayland purego 无 GTK 时在 `ui/input.KeyRepeater` 自合成（`rate≈30Hz/delay≈400ms`，修饰键不武装，失焦取消）；X11/Win/mac 委托系统/GDK，不归一 | 确定时钟单测（`TestKeyRepeater_Tick`） |

### F-B 选区

| # | 需求 | 验收 |
|---|---|---|
| F-B1 | Shift+方向键扩展选区，`composing` 时非 collapsed 直接失败 | `SetSelection` 失败单测 |
| F-B2 | 鼠标拖选，限 `editable_range()` | 拖选真窗 |
| F-B3 | 双击选词/三击选段，限 `editable_range()` | 词/段边界单测 |
| F-B4 | 选区高亮（行盒并集，来自 TextLayout） | 真窗矩形并集像素 |
| F-B5 | Copy/Cut/Paste 接剪贴板（`PurposePassword` 时 Copy 仅 `●`，限 `editable_range()`） | 真窗 + 剪贴板单测 |

### F-C 编辑与形态

| # | 需求 | 验收 |
|---|---|---|
| F-C1 | 插入/前后删（本轮 rune 吸附、API 预留簇，限 `editable_range()`，Backspace/Delete 按 surrogate 对计 1，`AddCodePoint(rune)` 为原子插入） | 单测 |
| F-C2 | Undo/Redo（一次组合 `Begin→Update*→Commit` = 一步，组合中 `Backspace` 不另起步） | 分组单测 |
| F-C3 | 词删除 Ctrl+Backspace/Delete，限 `editable_range()` | 词边界单测 |
| F-C4 | 只读态：编辑全拒，IME 可开但 `AddText`/`UpdateComposingText` 均拒绝写入；`input_type==NONE` 时 `hide` 直接 `focus_out` | 只读+NONE 单测 |
| F-C5 | 单行/多行：单行回车不换行、水平滚动、拒 '\n'；多行反之（`MaxLines/Ellipsis` 裁剪不影响 `editable_range()`，见 F-F3） | 单多行真窗 |
| F-C6 | 提交动作：Enter 触发 `onSubmitted` + `TextInputAction(done/go/search/send…)`，仅 `filter_keypress` 未命中且 `Return` 时触发 `performAction` | 回调单测 |
| F-C7 | 组合期回车分层：单行组合中回车=提交原文 commit as-is，非组合=onSubmitted；多行=换行 | 单测 |
| F-S1 | 组合起点：有选区时 `DeleteSelected()` 先删选中段再 `erase(composingRange)`，无选区从 caret 起 | 选中段拼音替换真窗 |
| F-S2 | 组合期选区钳制：任何 `SetSelection` 非 collapsed 在 `composing` 下失败；`MoveCursor*` 限 `editable_range()` | 钳制单测 |
| F-S3 | commit 替换语义：`AddText`/`AddCodePoint` 原子替换 `composing_range` 或 `selection`，`delta replace_range = was_composing?composing_before:selection_before` | 单测锁定 |

### F-D IME（完全对齐 Flutter）

| # | 需求 | 验收 |
|---|---|---|
| F-D1 | preedit 进缓冲：`UpdateComposingText(text, selection)` 替换 `collapsed?selection:composingRange`，`text` 含 preedit；`""` 且 `collapsed` 时 no-op，结束由 `EndComposing` | 单测 |
| F-D2 | 组合光标：`SetComposingRange(range, cursorOffset)` 需 `composing==true`，`selection=range.start+cursorOffset` | 真窗 |
| F-D3 | 锚点：`composing` 时必报 `set_cursor_rectangle`（`composing_rect*transform+width/height` 经 `translate_coordinates`），非 `composing` 时仅预热 `composing_rect/transform` 不报 | 日志断言：非组合期 0 上报，组合期 1 次/变更 |
| F-D4 | surrounding 拉取：仅 `retrieve-surrounding` 时 `gtk_im_context_set_surrounding(text,-1,GetCursorOffset)`，`-1` 为 NUL 结尾；Wayland `set_surrounding_text` 限 4000 bytes 含 NUL，以光标为中心截断 | 拉取单测，截断单测 |
| F-D5 | deleteSurrounding：`offset/count` 按 code point（surrogate 对计 1，Go 侧 rune 计 1，在 UTF16↔UTF8 换算中统一），`selection` 仅 `offset<=0` 时移至 `start`，`composingRange.end-=len` | 单测 |
| F-D6 | purpose/hint 完整：`inputAction/enableDeltaModel/inputType→GtkInputPurpose/Hints`，`TextInputConfiguration` 含 `autofillHints` 透传（UI 不做，仅透传） | 映射表单测 |
| F-D7 | 多字段/多窗口：每窗口一 handler，`show/hide` 按 `input_type==NONE` 切 `focus_in/out`，首焦 `focus_out` 预热（`fl_text_input_handler_new:454`） | 双窗互不影响，首焦即弹 |
| F-D8 | `set_editing_state` 覆盖：`SetText(text)` 一次→`selection -1/-1→0,0`→`SetText` 二次覆盖→`SetSelection`→`composing -1/-1→EndComposing else SetComposingRange(range, selectionBase-composingStart)` | 覆盖单测 + `TestDelta_NonTextUpdate` |
| F-D9 | Hint 位透传（Completion/Spellcheck/HiddenText 等） | 协议日志 |
| F-D10 | 锚点自动闭环：`OnChange→RefreshIMEAnchor`，程序化 `SetText` 也刷新（仅 composing 时实报） | 单测 |
| F-D12 | 三端矩阵：Linux Wayland+X11 本期 ✅（P12 双引擎 ibus/fcitx 均过 P1–P9），Win IMM32/TSF 与 mac NSTextInputClient 为 M2/M3 分窗复测、本期复用同逻辑 | P12 双引擎 |

> F-D11（IME 主动改选区）已删除：Flutter 无此通道，选区唯一来源为 `set_editing_state` 与本地 `editable_range` 操作。

### F-E 控件与外观

| # | 需求 | 验收 |
|---|---|---|
| F-E0a | `BaseEditable` 四件套 `Editor/IMERect/ContentType/DrawPreedit`，≤15 行接入（`grep -c` 审计，含 `BaseEditable` 嵌入） | 样板窗 |
| F-E0b | 占位符（空+未聚焦 hint，不进缓冲） | 真窗 |
| F-E0c | 密码掩码（PurposePassword 圆点+关预测，`composing` 禁止） | 掩码像素+日志 |
| F-E0d | 只读/禁用样式 | 真窗 |

### F-F 滚动与裁剪

| # | 需求 | 验收 |
|---|---|---|
| F-F1 | 输入/移动自动露出光标（`ensureCaretVisible` 调 `scrollX/scrollY`） | 长文真窗 |
| F-F2 | 单行水平滚动（`scrollX + SetOffset(-scrollX)`，`IMERect` 需 `x-scrollX`） | 真窗 |
| F-F3 | MaxLines/Ellipsis 与编辑态共存（`…` 不进 `Carets`/`editable_range`，仅裁剪显示） | 真窗 |

---

## 6. 统一模型

### 6.1 TextRange & Editor（= TextInputModel）

const (
AffinityDownstream = 0 // 下游：换行处归上行尾
AffinityUpstream   = 1 // 上游：换行处归下行头
)
type TextRange struct { Base, Extent int; Affinity int } // start=min, end=max, collapsed/Contains/length; {-1,-1} 为哨兵
type Editor struct {
text string; selection, composingRange TextRange; composing bool
batchDepth int; lastFrameworkText string; lastFrameworkSel, lastFrameworkComp TextRange
epoch uint64; caretCol float64; caretColValid bool; OnChange func()
contentType ContentType // purpose/hint/inputType/inputAction 缓存
}
func (e *Editor) SetClient(cfg TextInputConfiguration) // 定 enableDeltaModel（运行中不可变）+ contentType
func (e *Editor) SetText(text string, sel, comp TextRange, affinity int) bool
func (e *Editor) SetSelection(TextRange) bool // composing && !collapsed → false
func (e *Editor) SetComposingRange(TextRange, cursorOffset int) bool
func (e *Editor) BeginComposing()
func (e *Editor) UpdateComposingText(text string, sel TextRange) bool // 只读态下直接 false
func (e *Editor) CommitComposing()
func (e *Editor) EndComposing()
func (e *Editor) BeginBatchEdit(); EndBatchEdit() // batchDepth 嵌套，>0 抑制 OnChange/通知，末层 epoch++ 去重
func (e *Editor) DeleteSelected() bool // 删 selection，若 composing 则同时清 composingRange
func (e *Editor) AddText(text string) bool; AddCodePoint(r rune) bool // 原子替换 composing_range 或 selection
func (e *Editor) DeleteSurrounding(offset,count int) bool // offset/count 按 code point，surrogate 计 1
func (e *Editor) Backspace() bool; Delete() bool // 按簇，Backspace 限 editable_range
func (e *Editor) MoveCursorToBeginning() bool; MoveCursorToEnd() bool
func (e *Editor) MoveCursorBack() bool; MoveCursorForward() bool // 按簇，限 editable_range，更新 caretColValid
func (e *Editor) MoveCursorUp() bool; MoveCursorDown() bool // 粘滞列 caretCol
func (e *Editor) MoveCursorByWord(forward bool) bool // Ctrl+←→ 词跳
func (e *Editor) GetText() string; GetCursorOffset() int // UTF8 byte 偏移
func (e *Editor) TextRange() TextRange; EditableRange() TextRange
func (e *Editor) ShouldSkipFrameworkUpdate(text string, sel, comp TextRange) bool // 全相等跳过
func (e *Editor) IsComposing() bool
```

### 6.2 TextLayout

见 A1，仅读 `Editor.text`（已含 preedit），`Paint` 与查询（`CaretForOffset`/`ByteOffsetAtPoint`/`BoxesForRange`）同一份 shape；`Generation` 失效时整表重建。

### 6.3 增量通道

```go
type TextEditingDelta struct { OldText, DeltaText string; DeltaStart, DeltaEnd int; Selection, Composing TextRange; Affinity int }
func (d TextEditingDelta) IsNonTextUpdate() bool { return d.DeltaStart == -1 } // deltaText=="" 且 OldText==text 时
type TextInputConfiguration struct { InputType, InputAction string; EnableDeltaModel bool; AutofillHints []string }
```

`EnableDeltaModel` 在 `SetClient` 时定、运行中不可变；`IsNonTextUpdate` 时 `apply` 以 `OldText` 为基底 last-write-wins；`TextRange` 携带 `affinity`。

---

## 7. 平台实现（Wayland 与 X11 区分）

> **共用层（平台无关）**：`ui/textinput.Editor`（四元组/affinity/batch/epoch 去重）、`ui/rendering.TextLayout` 单源缝表、`ui/embedder.InputRouter`（`filter_keypress`/`surrounding`/`二次覆盖`/`batch/done`）与 `F-B/F-C/F-D/F-E/F-F` 定义对两平台完全复用。本章仅区分**平台适配**的实现差异。

### 7.1 Linux Wayland（`wayland_textinput_linux.go` · `zwp_text_input_v3`）【主路径·已落地】

- **协议**：`zwp_text_input_manager_v3` → `zwp_text_input_v3`（`enable/disable/set_surrounding_text/set_content_type/set_cursor_rectangle/commit` + 事件 `enter/leave/preedit_string/commit_string/delete_surrounding_text/done`），`purego` 直绑，缺省时 `Window.IME()==nil` 静默退化。
- **使能**：`EnableIME(rect)` 队列 `enable + rect + contentType` 一次 `commit`；`SetContentType(purpose)` 立即 `set_content_type(None,purpose)`；`UpdateCursorRect` 按 `ScaleFactor` 转物理像素、去重相同矩形、`hasRect` 缓存供 `refreshTextInput` 重发，`composing==false` 时仅预热、`true` 时实报 `composing_rect`（`BoxesForRange` 并集，经 `translate_coordinates`）。
- **预编辑**：`preedit_string(text,commit,index)` → `IMEKind=0 compose`（`IMEStart==IMEEnd==index`，`-1` 表尾），`commit_string(text)` → `IMEKind=1 commit`，`delete_surrounding_text(before,after)` → `IMEKind=3`（`Start=-before/End=after`），`done(serial)` 仅唤醒 `WakeUp` 与 `batchDepth` 互斥。
- **环绕文本**：`SetComposing(text,cursor)` 按 4000 字节居中截断（`budget=3999`，按 UTF8 边界吸附，`cursor` 同步回退），`pushSurrounding` 走 `TruncateSurrounding` 单源。
- **焦点**：`enter` 触发 `refreshTextInput`（`enable+content+rect+commit` 重发，补 `mutter` focus 前 `commit` 丢失），`leave` 发 `disable+commit`；`tiRecheckDelay 400ms` 的 `disable→enable` 延迟重激活仅 `ibus/mutter` 首活兜底，有真实 `preedit/commit` 即 `cancelRecheck`。
- **与上层对接**：`preedit-start→Editor.BeginComposing`；`preedit-changed→UpdateComposingText+SetSelection+delta`（`enableDeltaModel` 分支）；`commit→AddText/CommitComposing+delta`；`delete_surrounding→DeleteSurrounding+delta`；`filter_keypress` 走 `InputRouter` 优先拦截。

### 7.2 Linux X11（`x11_xim_linux.go` · `XIM`）【极简已落地·预编辑待补】

- **当前已落地（PreeditNothing 极简）**：`XSetLocaleModifiers("") → XOpenIM → XCreateIC(XNInputStyle=PreeditNothing|StatusNothing, XNClientWindow=win)`，`XFilterEvent` 优先（`handled==true` 即组合期吞键），否则 `Xutf8LookupString` 取 `UTF8` → `AddText+CommitComposing`；`XSetICFocus/UnsetICFocus` 绑定 `EnableIME/DisableIME`，`SetComposing/UpdateCursorRect/SetContentType` 均空实现（注释 `XNSpotLocation would need XIMPreeditPosition`），`Window.IME()==nil` 时同 Wayland 静默退化。已满足“能打中文、候选框由系统弹”可用态，对应 `R5` 禁用/占位/滚动验证已过但预编辑不高亮。
- **待补完整（PreeditCallbacks/Position）**：切 `XIMPreeditCallbacks`（注册 `XIMPreeditStartCallback/DrawCallback/DoneCallback/CaretCallback`）或 `XIMPreeditPosition + XNSpotLocation`，将 `preeditDraw(text, caret)` 映射为 `Begin/UpdateComposingText`，把 `IMERect`（`CaretForOffset-scrollX` 经 `ScaleFactor`）以 `XPoint spot` 回传 XIM 使候选框跟光标，`PurposePassword` 禁组合同 Wayland，`KeyRepeater` 仍委托系统（X11 无需 `purego` 重复）。
- **与 Wayland 的差异点**：无 `surrounding/done/content_type/cursor_rectangle` 协议，靠 `XIM` 回调驱动；无 `tiRecheckDelay`，靠 `XSetICFocus` 重激活。

### 7.3 Windows

`text_input_plugin.cc` 的 `TextHook/ComposeBegin/Change/Commit/End/KeyboardHook` 同 Linux；`enableDeltaModel` 分支同（delta vs 全量），`TYPE_TEXT_VARIATION_PASSWORD` 禁组合，`firstRect` 仅 composing 时。

### 7.4 macOS/iOS

`FlutterTextInputPlugin.mm`：`setMarkedText→Begin+Update`，`insertText→AddText+Commit+End`，`unmarkText→Commit+End`，`firstRectForCharacterRange` 仅 `composing==true` 时返回 composing 框（含 `translate_coordinates`），`interpretKeyEvents` 双门模型（`filter_keypress` 等价）。

### 7.5 平台映射对照（Wayland ↔ X11）

| 能力 | Wayland (`zwp_text_input_v3`) | X11 (`XIM`) | 复用层 |
|---|---|---|---|
| 使能/失能 | `enable(surface)/disable(surface)+commit` | `XSetICFocus/UnsetICFocus` | `Editor.OnAnchor / InputRouter.syncSession` |
| 光标锚点 | `set_cursor_rectangle(x,y,w,h)*ScaleFactor` | `XNSpotLocation (XPoint)`（待补） | `IMERect()` |
| 环绕文本 | `set_surrounding_text(text,cursor,anchor) -1 NUL + 4000 截断` | `XIM` 不上报（待补时由回调拉取，不经过此） | `TruncateSurrounding` |
| 预编辑 | `preedit_string` | `PreeditDrawCallback`（待补） | `Begin/UpdateComposingText` |
| 提交 | `commit_string` | `Xutf8LookupString` | `AddText+CommitComposing` |
| 删除环绕 | `delete_surrounding_text` | `XIM` 删除回调（待补） | `DeleteSurrounding` |
| 内容类型 | `set_content_type(hint,purpose)` | 无（待补时忽略 `PurposePassword` 仍禁组合） | `ContentType` |
| 批结束 | `done(serial)` 唤醒 + `batchDepth` 互斥 | `XFilterEvent` 同步，无 `done` | `batchDepth` |

---

## 8. 并发契约（硬）

C1 状态与 Editor 仅事件循环线程；C2 Timer 仅投队列+WakeUp；C3 adapter 标注线程语义；C4 Wayland `zwp_text_input_v3.done` 批处理队列 `dispatch` 独占，与 `batchDepth>0` 互斥（`done` 进队列后不立即 `updateEditingState`，待 `EndBatchEdit` 的 `epoch++` + `ShouldSkipFrameworkUpdate` 去重后一次性发送）。`batchDepth>0` 时抑制 `OnChange` 与 `updateEditingState`，`EndBatchEdit` 时一次性 `epoch++` 并按 `ShouldSkipFrameworkUpdate` 去重。

---

## 9. 控件接入

```go
type TextEditTarget interface { Editor()*Editor; IMERect() Rect; ContentType() ContentType }
type BaseEditable struct { ed *Editor }
func (b *BaseEditable) DrawPreedit(pc *PaintContext, text string, composing TextRange, segs []Segment)
```
≤15 行接入（`grep -c` 含嵌入计行），`IMERect` 取 `TextLayout.CaretForOffset(selection.extent)` 的 `X - scrollX`（单行横滚补偿），`affinity` 参与换行处归属，`caretOn` 节拍 ~500ms。

> **硬纪律（R 示例只测不实现）**：`IME` 输入框控件的实现**必须**落在 `ui/` 引擎层（`ui/textinput` 的 `BaseEditable` / `InputBox` + `ui/rendering` 单源排版 + `ui/embedder` 会话），`examples/ui_wr_ime_r*` 真窗**只做测试**（摆控件、走输入、跑探针/像素/Golden），**禁止**在 `examples/` 里实现输入框功能（在示例层写输入框即视为绕引擎洞，按 `AGENTS.md` 禁止）。

---

## 10. 测试与验收（R1–R5 指标族全覆盖 · 硬）

> **一句话**：每个 R 都要有指标族测试，单测和真窗用同一套数据、同一把尺子、同一个判据，场景必须复杂真实，禁止用一点点简单用例装绿。

### 10.0 通用硬约束（所有 R1–R5 必守，缺一即 FAIL）

| # | 规则 | 说明 | 违者 |
|---|---|---|---|
| G1 | **同源同数同判据** | 单测与真窗的输入、断言数、阈值必须同源：同一段文本、同一批采样点、同一门限值。R2 的 36×m 深部 5 位置+8 点、R1/R3 的 4000 中心截断 4 锚点（0/1/4/3/4/末尾）等，单测与真窗同源同阈值。禁止单测简单、真窗另起一套。 | 判 **假绿**，该 R 不得标 ✅ |
| G2 | **复杂真实场景** | 每个 R 的测试必须覆盖真实输入法会遇到的复杂情况：中英文混排、长串/换行、边界粘滞、快速连击、焦点切换、多引擎、surrogate/emoji、HiDPI、ellipsis。禁止只测 `a/b/1` 三字符或空串就过。字表走 `testdata/*.txt`（`cjk3000.txt`/`latin_all.txt`/`kr_all.txt` 等），禁止硬编码 `for r:=range` 生成。 | 场景不足即 FAIL |
| G3 | **指标族 A–J 全族全硬** | 每个 R 的真窗结束必须输出 **A–J 全族 10 族 JSON**（见 `ENGINE_UI_WIDGET_RENDER.md §2.2` / `ENGINE_UI_RENDER_BASE.md §20`），**每族必有阈值**，无数据填 `null`+显式原因并经 `metrics-audit` 认可，禁止默默省略或默默置 0。短窗（<15s）允许 `rss_slope` 写 `slope_gate=off` 但必须在 README 显式声明且 `metrics-audit` 复审；其余 10 族一律硬 FAIL，不存在“只采样”。各 R 的 10 族阈值见 §10.4.1–§10.4.5。 | 缺族/缺阈值即 FAIL |
| G4 | **禁止假绿/降画质装绿** | 逻辑探针绿≠画面对。每个 R 必须三证据：① 逻辑探针 JSON（全族）② 像素断言（以实际绘制墨迹为真值，F0–F9 选型+容差显式）③ Golden 逐位对比。真窗必须 `Present` 真跑 GPU（`present_policy` 与 `damage_ratio` 如实上报），禁止 `CompositeOnly+全幅Clear` 丢静态或降分辨率/关抗锯齿保 fps。 | `metrics-audit` 审出假值直接打回 |
| G5 | **全对齐 Flutter 真实现** | 每个 R 的行为与门限必须逐项对齐 Flutter 源码（`text_input_model.cc`/`text_editing_delta.cc`/`fl_text_input_handler.cc`/`linux/TextInputPlugin`/`darwin/FlutterTextInputPlugin.mm`/`TextPainter/SkParagraph`）及 `text_input_model_unittest.cc` / `fl_text_input_handler` 测试集。对齐项见 §10.4 各 R 行。 | 未对齐即 FAIL |
| G6 | **多轮检查 ≥3 轮** | 关 R 前必跑 3 轮：① 单测全量（含历史 R 回归，按文件逐个 `go test -run`）② 真窗 GPU 实测（取 §10.4 推荐 `RUN_SECONDS`，1200×800，打全族 JSON）③ `gpui-metrics-audit` 横切审（字段完备/诚实/阈值防偷放/观测一致/降画质）。任一轮 FAIL 不得关 R；审计报告贴 JSON 链接。 | 未跑满 3 轮不得标 ✅ |

### 10.1 单测（按 R 分层，禁止简单用例）

- **通用**：所有单测读 `testdata` 真字表，字体缺失 `t.Skipf` 不静默绿；长跑用例单给足 `-timeout`，按文件逐个跑 `go test -run '^(用例)$'`，禁止 `go test ./...` 全量并发卡死。
- **R1/R2/R3/R4/R5 细项**见 §10.4 各 R 行「单测场景」列。

### 10.2 真窗矩阵（P1–P15 为基座，R 真窗在其上加严）

| # | 用例 | 判据 | 关联 R |
|---|---|---|---|
| P1 | 英文 10 键 | 恰一次插入 | R1,R3 |
| P2 | 拼音→候选→上屏 | preedit 高亮，原子替换 | R1,R3 |
| P3 | 组合中退格 | 缩拼音不删缓冲外 | R1,R3 |
| P4 | Esc 取消 | composing 清 | R1,R3 |
| P5 | 点击定位 | 最近边界，深部无漂移，affinity 正确 | R2 |
| P6 | 粘滞列 | 上下保持 | R2 |
| P7 | 密码 | 日志+掩码，无候选 | R4 |
| P8 | 切引擎 | 首焦即弹 | R3 |
| P9 | 单键 ≤2 preedit | 风暴锁 | R3 |
| P10 | 多字段切换 | 无残留 | R3,R5 |
| P11 | 死键 | 重音正确 | R3 |
| P12 | ibus/fcitx | 均过 P1–P9 | R3 |
| P13 | 守护重启 | 恢复或降级 | R3 |
| P14 | 深部 36×m | 墨迹间隙，Carets 查表无漂移 | R2 |
| P15 | 4000 截断 | 以光标为中心 4 锚点，不崩 | R3 |
| P16 | 密码/只读 | 掩码像素 `●` + 日志无候选，只读可移不可写 | R4 |
| P17 | 单行/多行 | 单行拒 `\n` 横滚，多行允 `\n` + `MaxLines/Ellipsis` 不进 `editable_range` | R5 |

- 证据：逻辑探针 JSON（A–J 全族）+ 像素（以实际绘制墨迹为真值，F0–F9，闪烁 500ms 节拍采样）+ `GPUI_IME_DEBUG` 日志。每个 R 真窗的 JSON 外壳同 `ENGINE_UI_WIDGET_RENDER.md §2.2.5`。

### 10.3 真窗硬性几何与时长（继承 `ENGINE_UI_WIDGET_RENDER.md §2.5`）

- 客户区 **1200×800** 逻辑像素，`RUN_SECONDS≥5` 最小观察时长硬底线（**所有 `ui_wr_ime_r*` 真窗均为手动关闭，无自动关闭**：`RunFor=0` 无限运行，需人工点窗口 `X` 关闭；`RUN_SECONDS` 仅为门禁的最小观察时长，不触发自动退出）；各 R 最小观察时长见 §10.4。
- 窗口标题含 `ability_id`（如 `ime_r2_textlayout`），结束打全族 JSON，不达标 `FAIL:`+`exit 1`。

### 10.4 R×指标族×场景×门禁矩阵（每个 R 独立真窗，禁止复用）

> **读法**：每行一个 R，对应一个独立 `examples/ui_wr_ime_r*` 真窗（1200×800，`RUN_SECONDS` 为最小观察时长；**所有真窗均为手动关闭，无自动关闭**）；**A–J 10 族全硬**（阈值见 10.4.1–10.4.5），单测与真窗同源同数同判据。

#### 10.4.0 总览（5 窗总表，硬阈值以 10.4.1–10.4.5 细表为准）

| R | 能力（F 覆盖） | 10 族门禁（A–J 全采，细表为硬阈值真源） | 关键场景数 | 单测入口 | 真窗名 | 最小观察时长 RUN_SECONDS（手动关闭） | 证据 |
|---|---|---|---|---|---|---|---|
| **R1 Editor** | F-D1/D2/D4/D5/D8, F-S1–S3（四元组+affinity+batch+surrogate+4000 居中） | 全采，硬阈值见 10.4.1（A 仅告警，其余硬） | 8 场景（含空/单字/surrogate/4 锚点/嵌套 batch） | `TestEditor_*` | `ui_wr_ime_r1_editor` | **5** | 单测全绿+全族 JSON+四元组探针 |
| **R2 TextLayout 单源** | F-A2/A3/A5, F-B4（Carets 单源） | 全采，硬阈值见 10.4.2（长串 15s 时 G 硬） | 7 场景（含 36×m/affinity/长串/Fallback/HiDPI） | `TestTextLayout_*` | `ui_wr_ime_r2_textlayout` | **5**（长串 15） | 探针+像素+Golden |
| **R3 通道与平台** | F-D1–D10, F-C6/C7（Delta 定值/6 信号/filter/surrounding/二次覆盖） | **10 族全硬**（风暴/回退/泄露零容忍，细表为准） | 9 场景（含风暴/死键/autofill/双引擎） | `TestDelta_*/TestChannel_*/TestSurrounding_*` | `ui_wr_ime_r3_channel` | **10** | 仿真+日志+全族 JSON |
| **R4 选区与编辑** | F-B1–B5, F-C1–C7, F-E0c, F-S2/S3（钳制/密码/Undo/锚点） | 全采，硬阈值见 10.4.4（A/C/D/E/J 硬） | 8 场景（含双/三击/拖选/密码/只读/撤销） | `TestSelection_*/TestPassword_*/TestAnchor_*` | `ui_wr_ime_r4_selection` | **10** | 拖选像素+hint 日志 |
| **R5 控件与滚动** | F-E0a/E0b/E0d, F-F1–F3, F-C5（BaseEditable/占位/滚动） | **10 族全硬**（滚动 hitch/泄露零容忍，细表为准） | 7 场景（含 5000 字横滚/组合期滚动/ellipsis/首帧） | `TestScroll_*/TestBaseEditable_*` | `ui_wr_ime_r5_control` | **15**（滚动 30） | 样板窗+滚动像素+全族 JSON |

#### 10.4.1 R1 Editor — 四元组（F-D1/D2/D4/D5/D8, F-S1–S3）

**全族 10 族阈值（硬）：**

| 族 | 字段 | 阈值（R1，5s 窗） | 说明 |
|---|---|---|---|
| **A 帧时** | `fps_wall`/`interval_p95_ms`/`hitch_rate_per_min` | 必采，`hitch_rate` 仅告警（非动画窗） | 帧时不卡但要如实上报 |
| **B 管线** | `frame_build_ms`/`frame_raster_ms`/`pipeline_depth` | `build_ms p95 <5ms`，`depth` 不持续 >上限 | 四元组操作不堵 Present |
| **C 脏区** | `damage_ratio`/`paint_count` | 允许≈1（R1 初期 `full_paint`），`paint_count` 可解释 | 纯逻辑窗脏区不硬卡面积 |
| **D CPU** | `cpu_pct_avg`/`cpu_ui_pct`/`cpu_raster_pct` | `cpu_pct_avg <40%`（5s 短窗） | surrogate 换算不飙 CPU |
| **E 内存** | `rss_slope_kb_per_min` | `slope_gate=off`（<15s 允许显式声明），`rss_end-peak` 不泄漏 | 短窗不硬卡 slope，需显式 |
| **F GPU** | `cpu_fallback_ops` | `==0` | 无 GPU 回退 |
| **G 图文** | `measure_cache_hit` | 必采（可 `skipped` 显式） | R1 无布局，允许跳过 |
| **H 启动** | `time_to_first_present_ms` | `<1000ms` | 首帧有内容 |
| **I 回归** | `BASELINE_JSON` delta | 偏差 <10%（若开） | 可选开，发布建议开 |
| **J 正确性** | `go vet` + 四元组探针 | `vet==0` 且探针 `text/selection/composingRange/composing` 全等 | 硬 |

**复杂真实场景（单测与真窗同源，8 项）：**
1. 空串/单字/纯 surrogate `😀𝄞` 的 `-1/-1` 二次覆盖（`SetText(s,{-1,-1},{-1,-1})→{0,0}`）
2. 中英混排 `你好Hello世界` 50+字（`cjk3000.txt` 抽 200 字）含 `你好Hello` 粘贴
3. surrogate 计 1：`DeleteSurrounding(-1,1)` 删 `𝄞`/`😀` 不劈半，`RuneCount` 校准
4. 4000 中心截断 4 锚点：光标 at 0 / 1/4 / 3/4 / 末尾，`surrounding` 截断后含光标
5. `batchDepth` 嵌套 3 层 `Begin/EndBatch` + 中途 `SetComposingRange`，`OnChange` 仅末层触发 1 次，去重 `ShouldSkipFrameworkUpdate`
6. `GetCursorOffset` UTF16↔UTF8 往返：`你好` 6 bytes ↔ 2 units，多语言 `kr_all.txt` 抽样
7. `UpdateComposingText("", collapsed)` no-op，`EndComposing` 才清
8. `editable_range` 越界写拒绝：`selection` 非 `collapsed` 时 `SetSelection` 直接失败

**单测阈值：** `TestEditor_Sentinel` 双 SetText、`TestEditor_Surrogate`、`TestEditor_SurroundingCenter4`、`TestEditor_BatchNest3`、`TestEditor_Utf16Roundtrip` 全绿；`go vet ./ui/rendering` 0。

**真窗阈值：** `ui_wr_ime_r1_editor` 10 键+Esc+Backspace 序列，探针四元组每步可复现，全族 JSON 按上表门禁。

#### 10.4.2 R2 TextLayout — 单源（F-A2/A3/A5, F-B4）

**全族 10 族阈值（硬）：**

| 族 | 字段 | 阈值（R2） | 说明 |
|---|---|---|---|
| **A 帧时** | `fps_interval`/`interval_p95_ms`/`hitch_rate_per_min` | `fps_interval≥55`（持续 tick 时），`p95≤22ms`，`hitch≤2/min` | 长串横滚不丢帧 |
| **B 管线** | `frame_build_ms`/`frame_raster_ms` | `build p95 <3ms`，`raster p95 <8ms` | shape 不堵 |
| **C 脏区** | `damage_ratio`/`paint_count` | `paint_count` 逐行可解释，`damage_ratio` 与行盒面积正相关 | R2 初期 `full_paint` 允许≈1 |
| **D CPU** | `cpu_pct_avg` | `<50%`（5s），`<65%`（15s 长串） | 逐字 `Measure` 不飙 |
| **E 内存** | `rss_slope_kb_per_min` | 短窗 `slope_gate=off` 显式，长串 15s 窗 `<20000` | 5000 字不泄露 |
| **F GPU** | `cpu_fallback_ops` | `==0` | 文排不回退 |
| **G 图文** | `measure_cache_hit`/`shape_cache` | 必采，hit 可解释 | 单源缓存命中 |
| **H 启动** | `time_to_first_present_ms` | `<800ms` | 首帧有字 |
| **I 回归** | baseline delta | <10% | 可选开 |
| **J 正确性** | CI grep | `grep -r MeasureWidth ui/rendering/text_layout.go` 为空 | 禁止 Carets 路径累加 |

**复杂真实场景（7 项）：**
1. 36×m 深部 5 位置×8 点（中点规则），`CaretForOffset`↔`ByteOffsetAtPoint` 往返误差 <0.5px，`X` 单调
2. 换行 `你好\n世界\nFlutter` 的 affinity 上/下游（行尾 `Downstream` 归上行尾，`Upstream` 归下行头）+ 空行 `"\n"` 盒高
3. 粘滞列 `caretCol`：上下键深部穿越列保持，`Home/End` 清 `caretColValid`
4. Fallback 混排 `中文+😀+مرحبا` 的 Carets 不劈簇，`MultiFace` 按行逐字 `DrawString`
5. HiDPI `scale 1.25/2.0` 下 1px 对齐采样（`UI_PIXEL_ASSERTION_STANDARD.md` F0–F9）
6. `MaxLines/Ellipsis` 与编辑共存：`…` 不进 `Carets`，`ByteOff` 不跳变
7. 长串 5000 字（`cjk3000.txt` 重复）`Build <100ms` 且横滚 `scrollX` 与 `Caret.X` 联动

**单测/真窗**同源同阈值，真窗加像素：墨迹间隙 F0–F9 选型+Golden 容差显式。

#### 10.4.3 R3 通道与平台 — Delta/信号（F-D1–D10, F-C6/C7）

**全族 10 族阈值（10 族全硬，零容忍）：**

| 族 | 字段 | 阈值（R3，10s） | 说明 |
|---|---|---|---|
| **A 帧时** | `fps_interval≥55`，`p95≤22ms`，`hitch_rate_per_min≤5` | 风暴期不卡 |
| **B 管线** | `frame_build_ms p95<5ms`，`pipeline_depth` 不超限 | 6 信号不堵 |
| **C 脏区** | `damage_ratio` 可≈1（full_paint）但 `paint_count` 可解释 | 通道窗允许全脏 |
| **D CPU** | `cpu_pct_avg<60%` | 风暴锁后不飙 |
| **E 内存** | `rss_slope<15000`，`rss_after_close` 不高于 peak | surrounding 拉取不泄露 |
| **F GPU** | `cpu_fallback_ops==0`，`gpu_ops` 稳定 | 无回退 |
| **G 图文** | `measure_cache_hit` 必采 | 通道不偷算 |
| **H 启动** | `time_to_first_present<1000ms` | 首焦即弹 |
| **I 回归** | baseline delta <10%（**必开**） | 通道必回归 |
| **J 正确性** | `vet==0` + `filter_keypress` 日志 | 命中即拦截 |

**复杂真实场景（9 项，含新增 3）：**
1. 6 信号序列 `preedit-start→change(+get_preedit_string)→commit→end` + `retrieve-surrounding` + `delete-surrounding` 仿真全通
2. 风暴锁：单键 ≤2 preedit（10ms 内连击 20 键，`UpdateComposingText` 去抖）
3. `filter_keypress` 命中即 `return TRUE` 拦截，Home/End/PageUp/Down/Return 分流（`Return` 仅 `MULTILINE+newline` 才 `AddCodePoint('\n')+performAction`）
4.  `set_editing_state` 二次覆盖：`SetText→{-1,-1}→0,0→SetText 二次→SetSelection→composing` 往返，`NonTextUpdate(deltaStart==-1)` 仅 `oldText==text` 触发
5.  `enableDeltaModel` 定值不可变 + `apply` last-write-wins（`text_editing_delta_unittest.cc` 同）
6.  surrounding 4000 居中 4 锚点 + `-1` NUL 结尾（Wayland `set_surrounding_text` / GTK `set_surrounding`）
7.  **新增** `autofillHints` 透传 + `inputAction/done/go/search/send` 映射 `GtkInputPurpose/Hints`
8.  **新增** 死键 `´+e=é` 重音正确（`Xutf8LookupString` 后 `AddText`）
9.  双引擎 ibus/fcitx 均过 P1–P9 + 守护重启恢复/降级（P12/P13）

#### 10.4.4 R4 选区与编辑 — 钳制/密码/Undo/锚点（F-B1–B5, F-C1–C7, F-E0c, F-S2/S3）

**全族 10 族阈值：**

| 族 | 字段 | 阈值（R4，10s） | 说明 |
|---|---|---|---|
| **A 帧时** | `fps_interval≥55`，`p95≤22ms` | 拖选不卡 |
| **B 管线** | `build p95<4ms` | 选区盒不堵 |
| **C 脏区** | `damage_area_px ∝ 选区并集`，`damage_ratio` 与面积正相关 | 硬 |
| **D CPU** | `cpu_pct_avg<55%` | 拖选不飙 |
| **E 内存** | `rss_slope<15000` | 不泄露 |
| **F GPU** | `cpu_fallback_ops==0` | 不回退 |
| **G 图文** | `measure_cache_hit` 必采 | 词跳命中 |
| **H 启动** | `time_to_first_present<1000ms` | 首帧有选区 |
| **I 回归** | baseline <10%（可选开） |  |
| **J 正确性** | `composing && !collapsed → false` 硬 + `vet==0` | 钳制 |

**复杂真实场景（8 项，含新增 2）：**
1. 双击选词/三击选段（`SelectWordAt` 词边界 `Ctrl+←→`，`cjk3000.txt` + `latin_all.txt` 抽样）
2. Shift+方向扩展 + 鼠标拖选跨行（行盒并集 `getBoxesForRange` 来自 TextLayout）
3. 密码 `PurposePassword` 圆点掩码像素 + 关预测，`BeginComposing` 直接拒绝（日志无候选）
4. 只读/`input_type==NONE` 禁写但 `MoveCursor` 仍可在 `editable_range`，`hide` 即 `focus_out`
5. 撤销分组：一次组合（`Begin→Update*→Commit`）=1 步，组合中 `Backspace` 不另起步（F-C2）
6. 词删除 `Ctrl+Backspace/Delete` 限 `editable_range`
7. 锚点 `set_cursor_rectangle` 仅 `composing` 实报 1 次/变更，非 composing 0 上报，`OnChange→RefreshIMEAnchor` 闭环（含程序化 `SetText`）
8. `AddText` 原子替换 `was_composing?composing_before:selection_before`（F-S3）

#### 10.4.5 R5 控件与滚动 — BaseEditable/滚动（F-E0a/E0b/E0d, F-F1–F3, F-C5）

**全族 10 族阈值（10 族全硬）：**

| 族 | 字段 | 阈值（R5，15s/30s） | 说明 |
|---|---|---|---|
| **A 帧时** | `fps_interval≥55`，`p95≤22ms`，`hitch_rate≤5/min` | 滚动 60Hz |
| **B 管线** | `build p95<4ms`，`raster p95<10ms` | 滚动不堵 |
| **C 脏区** | `damage_area_px ≪全屏`（`damage_ratio<0.3` 30s 窗） | 横滚仅脏条 |
| **D CPU** | `cpu_pct_avg<65%`（30s 5000 字横滚） |  |
| **E 内存** | `rss_slope<15000`（30s 必硬，15s 可告警） | 长串不泄露 |
| **F GPU** | `cpu_fallback_ops==0` |  |
| **G 图文** | `measure_cache_hit` 必采 |  |
| **H 启动** | `time_to_first_present<800ms` 且首帧有内容 | 硬 |
| **I 回归** | baseline <10%（必开） |  |
| **J 正确性** | `vet==0` + `BaseEditable ≤15 行` 审计 |  |

**复杂真实场景（7 项，含新增 2）：**
1. 单行 5000 字横滚 `scrollX + SetOffset(-scrollX)`，点击 `localX+scrollX` 命中 `ByteOffsetAtPoint`
2. **新增** 组合期横滚：preedit `拼音|` + `scrollX` 仍露出候选光标，`IMERect` 实报 composing 框
3. 粘性 `caretCol` 在滚动后仍保持
4. 多行 `MaxLines/Ellipsis` 与编辑共存：`…` 像素 + `…` 不进 `editable_range`
5. 占位符 `hint`（空+未聚焦才显，获焦或有字即隐）+ 禁用态样式像素
6. **新增** 单行拒 `\n`、多行允 `\n`，`MaxLines` 裁剪不影响 `TextRange`
7. 首帧 `warmup:true` + 滚动首帧有内容（`ENGINE_UI_WIDGET_RENDER.md §2.2 H`）

> **门禁执行**：每个 R 关闭时必须至少观察上表「最小观察时长」值（可更长不可更短，**手动关闭，无自动关闭**）后点 `X` 关闭并输出 **A–J 10 族 JSON**，按本节阈值 `metrics-audit` 审，三证据（探针+像素 F0–F9+Golden）齐才可回写 `ENGINE_UI_WIDGET_RENDER.md §2/§3` 或本文 §11 状态；任一族 FAIL → 走 `gpui-wr-debug` 定位分层 → `gpui-wr-engine` 或 `gpui-metrics-audit` 修复后重跑。

#### 10.4.6 R 真窗示例规范（人工可难度 + 指标族全覆盖 · 硬）

> **本节为 `ui_wr_ime_r*` 真窗的“怎么做窗、怎么验输入、怎么算过”硬规范**，与 `ENGINE_UI_WIDGET_RENDER.md §2.6` 对位，所有 `R1–R5` 真窗必须同时满足；缺一项即 `metrics-audit` 判 FAIL。

| 项 | 要求 | 说明 | 违者 |
|---|---|---|---|
| **W1 窗体与壳** | `1200×800` 逻辑像素 + `wrkit.NewShell`（TopBar + Legend≥5行色块 + Body + LiveHUD）+ `PhaseClock`（Steady→Spike→Recover）+ `EnsureUIFace` | 每个 `ui_wr_ime_r*` 必须有完整壳与 HUD，禁止裸 `RenderBox` 就当窗；LiveHUD 实时显 `fps/p95/hitch/policy/paint/presents/cpu + 探针` | 判 **假窗** |
| **W2 真实可输** | 每个真窗至少 **3 个可获焦 `InputBox`/`MultiLineInputBox`**（`ui/textinput` 引擎，`NewInputBox`/`NewMultiLineInputBox` + `SetFace` + `FocusManager` + `InputRouter` + `Clipboard` + `IME`，`RunFor=0` 手动关闭） | 字号/形态必须拉开：`10/12/16/20px` 单行各一 + `14px` 多行 `MaxWidth=w-16/56h` + 混排 `Aa@10+你好@16+Hello@12`；点击获焦、打字、退格、方向键、粘贴、拼音预编辑均走 `TextLayout` 单源缝表，禁止在 `examples/` 里自绘输入框绕引擎 | 无可输框即 FAIL |
| **W3 人工可难度** | 8–9 项 **手打必现** 场景（与单测同源同阈）：空/单字/surrogate `😀𝄞`、中英 `你好Hello` 混排 200 字、`DeleteSurrounding` 计 1、4000 截断 4 锚点、嵌套 batch、UTF16 往返、组合中退格/Esc、点击深部 36×m 第 k 字中部、换行 `affinity`、粘滞列 `caretCol`、Fallback `中文+😀+مرحبا`、HiDPI 1px、5000 字横滚 | 每项必须配 **手打路径**：点不同字号框获焦→输 `你好Hello`→`←→` 跨字→`↑↓` 跨行→拖选→`Ctrl+C/V`→拼音 `nihao`→`Esc`，深部用 `ByteOffsetAtPoint` 中点规则手点验证；禁止只跑 `a/b` 三字节就过 | 场景不足即 FAIL |
| **W4 指标族全硬** | 每个真窗结束必须打 **A–J 10 族全量 JSON**（同 `WIDGET_RENDER §2.2`），**每族必有阈值**（见 10.4.1–10.4.5），`cpu_fallback==0`、`measure_cache_hit` 必采、`paint_count` 可解释、`damage_ratio` 如实（`full_paint` 允许≈1）、`time_to_first<800/1000` | 无数据填 `null`+显式原因并经 `metrics-audit` 认可，禁止默默省略；短窗 `<15s` 允许 `rss_slope` 写 `slope_gate=off` 但须 README 显式 | 缺族即 FAIL |
| **W5 画面对** | `F0–F9` 选型 + 容差显式（`F6` 文字区域非背景像素≥阈值 + `F0` 色块中心点容差≤8）+ `Golden` 掩码逐位对比（静态区 `diff=0`）+ `SnapshotAsync` 双张（稳态+恢复） | 逻辑探针绿≠画面对；快照必须 `raster` 线程重画完整路径，禁止 `CompositeOnly` 丢静态 | 假绿即 FAIL |
| **W6 关闭方式** | `RunFor=0` 无限运行，**点 `X` 手动关闭**，`RUN_SECONDS` 仅校验最小观察时长（`<5 → FAIL`） | 所有 `ui_wr_ime_r*` 禁止 `RunFor>0` 自动关闭；自检 `GPUI_R*_SELFTEST` 例外（3s 定时仅用于 CI 探针，不作关闭证据） | 自动关闭即 FAIL |

**各 R 窗口必含清单（在 W1–W6 之上）：**
- **R1**：4 字号单行 + 1 多行 + 1 行内混排 `10+16+12`（同缝表验证）+ `probe/surrounding/epoch` 实时标签 + 右侧动态异形边框（证 `paint_count`）
- **R2**：`36×m` 单行 + `你好\n世界\nFlutter` 换行 + `0123456789×3` 多行粘滞 + `中文+😀+مرحبا` Fallback + `HiDPI` 标签 + `MaxLines=1 Ellipsis` + `5000` 横滚（200 字可见 + 5000 离屏 `Build <100ms`）+ 实时 `probe{deep/affinity/sticky/fallback/ellipsis/long/boxes/generation}`
- **R3–R5**：在 R1/R2 基座上叠加 `preedit 高亮/风暴锁/死键/autofill/密码掩码/拖选` 等，窗内必须保留 **至少 2 个可输框** 供手打验证，后续按本节 W1–W6 逐项加严。

---

## 11. 分期（推翻重写）

| 期 | 内容 | 门禁（A–J 10 族全采 + 三证据 + 3 轮，硬阈值见 §10.4.1–§10.4.5） |
|---|---|---|
| R1 Editor 重构 ✅已落 2026-08-29 | 四元组+affinity+batch+去重，按 UTF16 | 单测全绿 + `ui_wr_ime_r1_editor` 10 族全采（A 仅告警，其余硬） + 四元组探针 + 4 锚点 + 5000 字 |
| R2 TextLayout 单源 ✅已落 2026-08-29 | 新建 `text_layout.go`，Paint 改 DrawShapedGlyphs，Carets 查表 | `ui_wr_ime_r2_textlayout` 10 族全采（长串 15s 时 G 硬） + P14/P5 三证据（探针+像素+Golden）+ 5000 字 <100ms + HiDPI/ellipsis |
| R3 通道与平台 ✅已落 2026-08-29 | Delta+enableDeltaModel 定值，6 信号+surrounding 拉取+filter_keypress 命中拦截+二次覆盖 | 仿真全绿 + `ui_wr_ime_r3_channel` **10 族全硬** + P2/P9/P15/死键/autofill 日志 + ≥3 轮 `metrics-audit` |
| R4 选区与编辑 ✅已落 2026-08-29 | F-B/F-C4 密码禁组合/F-C2-6/F-S/锚点预热+仅 composing 报 | `ui_wr_ime_r4_selection` 10 族全采（A/C/D/E/J 硬） + 拖选/双三击/密码像素+hint 日志 |
| R5 控件与滚动 ✅已落 2026-08-29 | BaseEditable+F-E+F-F | `ui_wr_ime_r5_control` **10 族全硬** + 样板窗 + 5000 字横滚（含组合期）+ 首帧有内容 |
| M2/M3 | Win IMM32/TSF、mac objc 子类，复用同逻辑 | 双引擎 P12 + 10 族全采（硬阈值见细表） |

---

## 12. 待补实现清单（R1–R5 复盘 · 2026-08-29）

> 复盘范围：`ui/textinput/editor.go` / `delta.go` / `ui/rendering/text_layout.go` / `text.go` / `ui/embedder/input_router.go` / `ui/platform/wayland_*` / `ui/textinput/input_box.go` / `viewport_input.go` / `padding.go` / `examples/ui_wr_ime_r*` 对照 §4–§10 真源。已落地 `padding` 可扩展与 Wayland 5-mime 互通（`6d959f8/3011ae5`），余下按 **P0 硬拦 / P1 门禁 / P2 小缺** 分级。

| R | 级别 | 待补项（按规范） | 现状与证据 | 影响 |
|---|---|---|---|---|
| **R1 ✅已落 2026-08-29** | P1 ✅ | `SetClient` 透传 `contentType/InputType/autofillHints→GtkInputPurpose`（§6.1） | `929f53f` 已补：`purposeFromInputType` 映射 + `deltaModelLocked` 卫栏 + `autofillHints` 缓存 | 已落 |
| R1 ✅ | P1 ✅ | `enableDeltaModel` 定值不可变卫栏（§6.3） | `SetClient` 首次锁定，后续翻转忽略 | 已落 |
| R1 ✅ | P2 ✅ | `lastFramework*/epoch` 自动维护（§6.1/§8 C4） | `SetText/ApplyDelta` 末尾自动写 `lastFramework*`，`ShouldSkip` 去重闭环 | 已落 |
| R1 ✅ | P2 ✅ | F-S1 有选区先 `DeleteSelected` 再 `erase(composingRange)` 原子性 | `BeginComposing` 批内先删选区再起 `composing`（`editor.go:350`） | 已落 |
| R1 ✅ | P2 ✅ | `ApplyDelta` 以 `OldText` 为底一致性（§6.3） | `delta.go:73` 纯 `OldText` 基底 + RuneStart 吸附 + `NonTextUpdate` 严格 `OldText==e.text` | 已落 |
| R1 ✅ | P2 ✅ | 单测溯源 G2 字表 `testdata/cjk3000.txt` | `testdata/cjk3000.txt` 3000 字落地，`TestR1_MixedCJK200` 改读文件，新增 `TestEditor_*` 别名 | 已落 |
| **R2 ✅已落 2026-08-29** | **P0 ✅** | `TextLayout.Face` 字段溯源（§4 A1） | `dce5ace` 已补：`Face text.Face` 字段存于所有 `Build*` 路径 | 已落 |
| R2 ✅ | **P0 ✅** | HiDPI 1.25/2.0 1px 对齐 F0–F9 采样（§10.4.2-5） | `SnapPixel/SnappedX` 已落地，`TestTextLayout_HiDPI_Snap` 验 1.25/2.0 | 已落 |
| R2 ✅ | **P0 ✅** | 5000 字真面形 `<100ms` 压测（§10.4.2-7） | `TestTextLayout_LongBuild_RealFace` 以 `LoadMultiFace(14)` 真面形 <100ms + 存 `Face` | 已落 |
| R2 ✅ | P1 ✅ | `Generation` 属性驱动（§4 A1） | `TestTextLayout_Generation` 追加 `FontSize/LineSpacing/Face` 变 `Generation` 递增 | 已落 |
| R2 ✅ | P1 ✅ | `CaretColumn/ByteOffsetAtPoint` 全查 Carets（§4 A1） | `text_layout.go==0` 已过，`text.go:588` 仅 `lay==nil` 回退（有布局则查表），密码 `input_box:323` 为掩码宽度保留 | 已落 |
| R2 ✅ | P1 ✅ | `MaxLines/Ellipsis` 行宽一致性 | `BuildRenderTextLayout` 截行保留 `…不进Carets`，`TestTextLayout_EllipsisWidth` 验 `Text` 不含 `…` 且 caret 不越界 | 已落 |
| R2 ✅ | P2 ✅ | 粘滞列/ fallback 混排单测 | `TestTextLayout_StickyFallback` 验 `中文+😀+مرحبا` 不劈簇 + 粘滞 X | 已落 |
| **R3 ✅已落 2026-08-29** | **P0 ✅** | `enableDeltaModel` 定值 + 通道真分发 `delta vs 全量`（§6.3/§7.1） | `da44676` 已补：`deltaModelLocked` + `OnDelta` 真通道 `pushDelta` | 已落 |
| R3 ✅ | **P0 ✅** | `filter_keypress` 路由层优先拦截（§7.1） | `input_router.go:362` 先判 `IsComposing && isComposingFilterKey` 再 `Insert` | 已落 |
| R3 ✅ | **P0 ✅** | 6 信号 `retrieve-surrounding -1 NUL` 按拉取推（§7.1 F-D4） | `TruncateSurrounding` 预留 NUL + `tiPendingQueue` 附 `0`，`SurroundingUpdates` 按需推 | 已落 |
| R3 ✅ | **P0 ✅** | `set_editing_state` 二次覆盖四步原子（F-D8） | 新增 `ApplyFrameworkState` 合并哨兵→显式→Sel→Composing | 已落 |
| R3 ✅ | P1 ✅ | `batchDepth/done(serial)` 互斥（§8 C4） | `wlTiDone` 唤醒 + `IsInBatch` 抑制 `pushDelta/pushSurrounding` | 已落 |
| R3 ✅ | P1 ✅ | 锚点双流 `translate_coordinates`（§7.1） | `UpdateCursorRect` 按 `ScaleFactor` 换算 `rect*scale` | 已落 |
| R3 ✅ | P2 ✅ | `NONE/show/hide/首焦预热`、`4000` 单源去重、风暴/死键双引擎守护 | `IsNone()` 跳过 `Enable` + 首焦预热 `Disable`，`4000` 单源复用 | 已落 |
| **R4 ✅已落 2026-08-29** | **P0 ✅** | Undo/Redo 历史栈 `Ctrl+Z/Y` 一次组合=1 步（F-C2） | 已补：`Editor.Undo/Redo` 历史栈 100 步 + `composingSnapshot` 分组，`InputBox/Viewport/Multi Ctrl+Z/Y` | 已落 |
| R4 ✅ | **P0 ✅** | `MoveVisualUp/Down` 走 `SetSelection` 钳制（F-S2） | `visual_move.go:142` 改走 `SetSelection` + 粘滞恢复 | 已落 |
| R4 ✅ | **P0 ✅** | Viewport 密码/锚点/高亮三漏（F-E0c/F-D3） | `viewport_input.go` 已补：`sync` 密码掩码 + `caretAnchor` 密码 + `IMERect` composing 分支 + `syncHighlight` 密码判空 | 已落 |
| R4 ✅ | P1 ✅ | 拖选自滚动 timer + 多行自滚（F-B2/F-F1） | `input_box` 单行/`viewport` 单行/`multi` 双向 50ms `AutoScroll` 计时器 + `PointerMove` 28px/14px 步进 | 已落 |
| R4 ✅ | P1 ✅ | 三击语义统一 `SelectLineAt`（F-B3） | Viewport 三击改 `SelectLineAt` 与他处一致 | 已落 |
| R4 ✅ | P2 ✅ | `OnChange→RefreshIMEAnchor` 程序化 `SetText` 闭环（F-D10） | `Editor.OnAnchor` + `InputRouter.syncSession` 设置 `OnAnchor=afterEdit`，`changed`/`EndBatchEdit` 调 `OnAnchor` | 已落 |
| **R5 ✅已落 2026-08-29** | P1 ✅ | 禁用态键盘拦截（F-E0d） | 已补：`InputBox/Viewport/Multi` 全 `Disabled()` 早退 + `BaseEditable.SetDisabled→readOnly` 联动 + `Node.Enabled` 切换 + `InputRouter` 禁用感知（`currentTarget/syncSession/Route` 跳过禁用） | 已落 |
| R5 ✅ | P2 ✅ | 单行 `SetMaxLines` 显式忽略（F-C5） | 已补：`InputBox.SetMaxLines` 改 `return` 显式忽略（单行不设），注释与实现对齐 | 已落 |
| R5 ✅ | P2 ✅ | `warmup` 观测诚实性（§10.4.5-7/W1–W6） | 已补：`r5/main.go:370` 去硬写、`ticker` 同步 `Snap.Warmup`、`report.go:153` 去覆盖仅认 `Snap.Warmup` | 已落 |

> 已落地（不计入待补）：`padding.go` 可扩展（`SetDefaultPadding/SetPadding/ClearPadding` 默认 8，对齐 Flutter `contentPadding`）与 `viewport` 清空后空文本光标、`\u` 解码、`Ctrl+V` 剪贴板；Wayland 5-mime 互通与 `selMimes/offerMimes` 跟踪（`3011ae5`）；**R1 6 项已落 `929f53f`、R2 7 项已落 `dce5ace`、R3 7 项已落 `da44676`、R4 6 项已落（本提交）、R5 3 项已落（禁用/warmup/MaxLines）**。

## 13. 修订

| 版本 | 说明 |
|---|---|
| v3.1 2026-08-27 | 统一文本+IME，清理非 Flutter 引用 |
| **v3.2 2026-08-27** | 补齐 Flutter 缺失：TextRange affinity、Delta NonTextUpdate 定值、set_editing_state 二次覆盖与 -1 哨兵、GetCursorOffset/-1 与 4000 中心截断、filter_keypress 命中拦截、密码/NONE 禁组合、batchDepth 去重；修正锚点预热/实报、surrogate 换算、KeyRepeater 仅 Wayland purego、删除 F-D11 |
| **v3.3 2026-08-27** | 新增 §10 G1–G5 与 §10.4 矩阵：R1–R5 独立 `ui_wr_ime_r*` 真窗、A–J 全采、单测与真窗同源同数同判据、复杂场景（cjk3000/latin、36×m、5000 字、surrogate、风暴、双引擎）、三证据+3 轮审查；§11 回写全族 JSON |
| **v3.4 2026-08-27** | 补全覆盖：G3 收紧为 10 族全采且每族有阈值、新增 G6 多轮；§10.4 拆 10.4.0+10.4.1–10.4.5 分 R 10 族阈值表，补 12 漏场景（4 锚点/嵌套 batch/HiDPI/ellipsis/Fallback/autofill/死键/只读移动/撤销分组/组合期横滚）；§11 同步 |
| **v3.5 2026-08-27** | 全量复核 34 项：标题 v3.2→v3.5；§1 非目标 autofill 改透传；§2 哨兵/epoch/affinity；§3 补 Delta/Win/mac；§4 A1 Generation/A2 y-=scroll/A3 code point；§5 补闪烁500ms/剪贴板密码/簇口径/MaxLines；§6 补 SetClient/AddCodePoint/IsNonTextUpdate；§7 补 Delta 分支/死键；§8 Wayland done 互斥；§9 IMERect 减 scrollX；§10 修 G1 按 R/补 P16/P17/对齐总览与细表口径；§11 门禁对齐 |
| **v3.6 2026-08-28** | 补硬纪律：`R` 真窗只测不实现——`IME` 输入框实现必须落 `ui/`（`ui/textinput` + `ui/rendering` 单源 + `ui/embedder`），`examples/ui_wr_ime_r*` 只做 ≤15 行接入与真窗验证，禁止在示例层实现输入框（绕引擎洞） |
| **v3.7 2026-08-28** | 去自动关闭：所有 `ui_wr_ime_r*` 真窗改为**手动关闭（无自动关闭，`RunFor=0` 无限运行，点 X 关闭）**；`RUN_SECONDS` 仅为最小观察时长，见 §10.3/§10.4 与 `ENGINE_UI_WIDGET_RENDER.md §2.5` |
| **v3.8 2026-08-28** | 补 `R` 真窗示例规范 §10.4.6：`W1–W6` 窗体/壳/真实可输/人工可难度/指标族全硬/画面对/手动关闭 + 各 `R1–R5` 窗口必含清单（多字号/多行/混排/Fallback/5000 等），对齐 `WIDGET_RENDER §2.6` 硬度 |
| **v3.9 2026-08-29** | 新增 §12 待补实现清单：R1–R5 复盘 23 项（P0 硬拦 9 / P1 门禁 9 / P2 小缺 5），含 R2 Face/HiDPI/5000 真面形、R3 Delta 定值+6 信号+filter 优先+二次覆盖+done 组批、R4 Undo/钳制绕过/Viewport 3 漏/拖滚、R5 禁用拦截/warmup 诚实；已落地 padding 5-mime 不计入 |
| **v3.10 2026-08-29** | R1 已落：§12 中 R1 6 项标 ✅（`SetClient` 透传/定值锁、`lastFramework` 自动、`F-S1` 先删、`ApplyDelta` OldText 基底、`DeleteSurrounding` 吸附、`cjk3000.txt` 溯源），§11 R1 行标 ✅，对应实现 `929f53f` |
| **v3.11 2026-08-29** | R2 已落：§12 中 R2 7 项标 ✅（`Face` 溯源、`HiDPI` SnapPixel/`SnappedX` 1.25/2.0、`5000` 真面形 `LoadMultiFace` <100ms、`Generation` 属性化、`Caret` 单源、`Ellipsis` 一致、粘滞/回退），§11 R2 行标 ✅，对应实现 `dce5ace` |
| **v3.12 2026-08-29** | R3 已落：§12 中 R3 7 项标 ✅（`delta` 真通道 `OnDelta` + 定值锁、`filter_keypress` 路由优先、`-1` NUL、`二次覆盖` `ApplyFrameworkState`、`batch/done` 唤醒、`translate_coordinates` Scale、`NONE` 跳过），§11 R3 行标 ✅，对应实现 `da44676` |
| **v3.13 2026-08-29** | R4 已落：§12 中 R4 6 项标 ✅（`Undo/Redo` 历史栈+分组、`MoveVisualUp/Down` 钳制、`Viewport` 密码/锚点/高亮、`拖选 timer` 双向、`三击` 统一、`OnChange→Anchor` 闭环），§11 R4 行标 ✅，同时修 `BaseEditable.SetDisabled→readOnly` 联动与 `Viewport` 获焦 `Disabled` 拦截 |
| **v3.14 2026-08-29** | R5 已落：§12 中 R5 3 项标 ✅（禁用全拦截 `InputBox/Viewport/Multi`+`InputRouter` 感知、`SetMaxLines` 单行忽略、`warmup` 诚实化 `r5/main.go`+`wrgate/report.go`），§11 R5 行标 ✅；`metrics-audit` A-J 10族全审 + 三证据（探针+像素+Golden）通过 |

## 附录：偏移与截断

- `TextRange` 以 UTF16 记（`{-1,-1}` 哨兵），Go 边界 `Utf8ToUtf16/Utf16ToUtf8` 互转，surrogate 对按 2 计仅换算时 ×2；
- `surrounding` 4000 bytes 超长以光标为中心 4 锚点截断，末尾含 NUL，GTK/Wayland 共用 `-1`；
- XIM `PreeditNothing` 接受无样式；
- 闪烁 `caretOn` 节拍 ~500ms，`BaseEditable` ≤15 行 `grep -c` 审计。
