# 文本编辑 + IME X11 生产级实现需求文档（复用 Wayland 版 v3.5 · X11 完整版 v2.3 · 完全对齐 Flutter）

> **复用声明**：本文件为 `ENGINE_TEXT_WAYLAND_IME_REQUIREMENT.md`（Wayland 主真源 v3.5）的 **X11 完整镜像**，`R1-R5` 功能（`F-A/B/C/D/E/F/S`）、`§6 统一模型`、`§8 并发`、`§9 控件接入`、`§10 测试与验收`（含 R1-R5 单测与真窗同源同数同判据、三证据、A-J 10族全硬）**全部直接复用 Wayland 版，禁止修改已可用的 R1-R5 测试**；差异仅在 `§7 平台实现` 重写为 **X11 D-Bus** 完整实现（`org.freedesktop.IBus / org.fcitx.Fcitx5`），格式与 Wayland 版 §7 逐段对照。
> **纪律**：**仅使用 D-Bus ibus/fcitx5（`godbus/dbus/v5` 纯 Go）**。
> **工业级**：Wayland 与 X11 双平台可投产、单测+仿真+真窗像素三级门禁。
> **变更说明 v2.3（2026-08-30）**：于 `v2.2` 之上，将 `ibus↔fcitx5` 热切改为“脏标记→下次输入懒重探”（`NameOwnerChanged` 仅置 `imeDirty`，`EnableIME/ProcessKeyEvent` 入口重探），满足“切换后下次输入自动用当前生效输入法”；`X11` 走 `D-Bus ibus/fcitx5`，`Wayland` 走 `zwp_text_input_v3`。

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

## 7. 平台实现（X11 D-Bus 完整 · 框内预编辑）

> **复用声明**：本章为 **X11 专属完整实现**，功能层（`§4 架构 / §5 功能 / §6 模型 / §8 并发 / §9 控件 / §10 测试`）完全复用 `ENGINE_TEXT_WAYLAND_IME_REQUIREMENT.md`（Wayland 版），`R1-R5` 单测与真窗禁止修改。本章仅描述 X11 平台适配。

### 7.1 X11 D-Bus 框内预编辑（`x11_dbus_ime_linux.go` · `org.freedesktop.IBus / org.fcitx.Fcitx5`）【本文件主路径】

> **纪律**：**仅使用 D-Bus ibus/fcitx5（`github.com/godbus/dbus/v5` 纯 Go）**。参考 `GTK gtkimcontextibus.c` 的 `D-Bus ibus` 时序与信号归一。

- **依赖与会话总线（无 cgo）**：`go get github.com/godbus/dbus/v5`，`dbus.SessionBus()` / `dbus.SessionBusPrivate(opts…)` 自动解析 `DBUS_SESSION_BUS_ADDRESS`（回退 `unix:path=/run/user/<uid>/bus`），`Hello` 取 `unique name`，`BusObject.Call("org.freedesktop.DBus.AddMatch", "type='signal',sender='org.freedesktop.IBus'")` 与 `sender='org.fcitx.Fcitx5'` 分别订阅；有 `BUS` 无守护时静默退化 `Window.IME()==nil`（英文直通，与 `Wayland` 一致）；连接失败不阻塞建窗，`GPUI_IME_DEBUG=1` 打印 `dbus dial/hello/match`。
- **打开与探测（双引擎统一 · 官方签名）**：以 `godbus` `BusObject.Call` 同步探测，先 `ibus` 再 `fcitx5`，超时 `500ms`，`GPUI_IME_DEBUG` 记失败并退化——
  - **ibus（官方 `src/ibusbus.c:ibus_bus_create_input_context`）**：`service org.freedesktop.IBus`，`object /org/freedesktop/IBus`，`interface org.freedesktop.IBus`，`CreateInputContext(s client_name) → o`（`g_variant_new("(s)", client_name)`，`client_name="gpui:<进程名>"`，官方面 `s` 单参；老版双参 `ss` 仅作兼容探测），成功得如 `/org/freedesktop/IBus/InputContext_7`，接口 `org.freedesktop.IBus.InputContext`（`GDBusProxy` `service org.freedesktop.IBus`）；
  - **fcitx5（官方 `fcitx/fcitx5-dbusfrontend/src/dbus/dbusfrontend.cpp:DBusFrontend::createInputContext`）**：`service org.fcitx.Fcitx5`（兼容 `org.fcitx.Fcitx` 老名），`object /org/fcitx/Fcitx5/InputMethod`（老路径 `/org/fcitx/Fcitx/InputMethod`），`interface org.fcitx.Fcitx5.InputMethod`，`CreateInputContext(ss appname, s appid) → o`（`appname=argv[0]/"gpui"`, `appid="gpui"`；`godbus Call("CreateInputContext", appname, appid)`，空串兼容），成功得如 `/org/fcitx/Fcitx5/InputContext_2`，接口 `org.fcitx.Fcitx5.InputContext`；
  - 双探皆失败→ `nil IME`。每 `Window` 一 `InputContext`（非全局单例），`Window.Close` 时 `Destroy`（`ibus: proxy Destroy` / `fcitx5: DestroyIC`）并 `RemoveMatch`，避免泄漏。
- **InputContext 能力声明**：创建后立即 `SetCapabilities(uint32 caps)`（`godbus` `Call` `SetCapabilities` / `SetCapability` 名兼容）——`ibus: CAP_PREEDIT_TEXT(1<<0)|CAP_FOCUS(1<<2)|CAP_SURROUNDING_TEXT(1<<3)`，`fcitx5: CAPACITY_PREEDIT|SURROUNDING_TEXT`，再缓存 `engine`（`ibus`/`fcitx5`）以统一后续 `SetCursorLocation/SetSurroundingText/SetContentType` 的二态分发。
- **使能/失能**：`EnableIME(rect)` → `FocusIn()` + 缓存 `rect` 并 `SetCursorLocation(rect)` + `SetContentType(purpose)` + `SetSurroundingText(text, cursor, anchor)`；`DisableIME()` → `FocusOut()` 并 `EndComposing` 清理；`FocusIn/Out` 与 `Wayland` 的 `enable/disable` 语义一致，重复 `FocusIn` 幂等（`focused` 卫栏）。`godbus` 侧均为同步 `Call`（`FocusIn()` `()`，`FocusOut()` `()`，无返回值）。
- **光标锚点（候选跟随 · 官方名差异）**：`UpdateCursorRect(rect)` 取 `TextLayout.CaretForOffset(selection.extent)` 的 `X - scrollX`，按 `ScaleFactor` 转物理像素，`XTranslateCoordinates(window → root, x, y)` 得根坐标后分发——`ibus: SetCursorLocation(iiii)`（`src/ibusinputcontext.c:ibus_input_context_set_cursor_location`，`g_variant_new("(iiii)", x,y,w,h)`，`w=2, h=行高`）、`fcitx5: SetCursorRect(iiii)`（`fcitx5 InputContext SetCursorRect`，同参异名，`godbus Call("SetCursorRect", x,y,w,h)` 兼容 `SetCursorLocation`），候选窗钉在光标上（与 `Wayland` 的 `set_cursor_rectangle` 同源同参）；仅 `composing==true` 时实发，非 `composing` 仅缓存（`OnChange→RefreshIMEAnchor` 闭环同 `Wayland`）。
- **按键过滤（filter_keypress 等价 · 官方签名）**：`X11` 侧仍收 `XKeyPress`，先走 `ProcessKeyEvent` 再决定是否 `decodeKey→Editor.AddText`——
  - `ibus（官方 `src/ibusinputcontext.c:ibus_input_context_process_key_event`）: ProcessKeyEvent(uuu keyval, keycode, state) → (b handled)`（`g_variant_new("(uuu)", keyval, keycode, state)`，`keyval=XKeycodeToKeysym` 的 `keysym`，`keycode` 硬件码，`state` 修饰位；`godbus Call(...).Store(&handled)`）；
  - `fcitx5（官方 `DBusFrontend InputContext ProcessKeyEvent`）: ProcessKeyEvent(u keyval, u keycode, u state, u time, b isRelease) → (b handled)`（`time` 来自 `XKeyEvent.time`，`isRelease` 区分 Press/Release）；
  - `handled==true` 则拦截不再本地插入，与 `Wayland` 的 `filter_keypress` 命中即 `return TRUE` 一致；`handled==false` 再走本地 `Home/End/PageUp/Down/Return/Ctrl+←→`（`Return` 仅 `MULTILINE+newline→AddCodePoint('\n')+performAction`）与 `InputRouter.isComposingFilterKey`。`ProcessKeyEvent` 超时 `50ms`，超时按未处理放行（`godbus WithContext` 超时）。
- **预编辑/提交/删除（信号归一 · godbus 订阅）**：`godbus` `AddMatch` 后 `conn.Eavesdrop`/`Signal` 通道收——
  - `ibus` 信号：`UpdatePreeditText(variant Text, uint32 cursor_pos, bool visible)` / `CommitText(variant Text)` / `DeleteSurroundingText(int32 offset, uint32 n_chars)` / `HidePreeditText()`，`interface org.freedesktop.IBus.InputContext`；
  - `fcitx5` 信号：`UpdatePreedit(string text, int32 cursor)` / `CommitString(string text)` / `DeleteSurroundingText(int32 offset, uint32 n)`，`interface org.fcitx.Fcitx5.InputContext`（老 `org.fcitx.Fcitx` 同名）；
  - 归一后：`UpdatePreeditText(text, cursor, visible)` → `Editor.BeginComposing + UpdateComposingText(text, TextRange{start+cursor,start+cursor})`（`visible==false` 或 `HidePreeditText`→`EndComposing`）；`CommitText(text)` → `AddText(text)+CommitComposing`；`DeleteSurroundingText(offset,n)` → `DeleteSurrounding(offset,n)`（`offset/n` 按 `code point`，见 §6.1/附录）；与 `Wayland` 的 `preedit_string / commit_string / delete_surrounding_text` 同逻辑。`variant Text` 在 `godbus` 侧按 `dbus.Variant` 解 `string`。
- **环绕/内容类型（官方签名 · godbus 适配）**：`SetSurroundingText` 分引擎——`ibus（官方 `src/ibusinputcontext.c:ibus_input_context_set_surrounding_text`）: SetSurroundingText(v IBusText, u cursor_pos, u anchor_pos)`（`v` 为 `IBusText` 序列化 `variant`，`godbus dbus.MakeVariant(text)` + `ibus_serializable_serialize` 等价为含 `IBusText{text, attrs}` 的 `v`，`cursor/anchor` 为 `UTF8 byte` 偏移，`anchor==cursor` 无选区）、`fcitx5: SetSurroundingText(s text, u cursor, u anchor)`（`sii` 简式，`godbus Call("SetSurroundingText", text, cursor, anchor)`）；二者均限 `4000 bytes` 含 `NUL` 以光标为中心截断（复用 `textinput.TruncateSurrounding`，与 `Wayland` `set_surrounding_text` 同截断）；`SetContentType` 官方面为 `ibus: org.freedesktop.DBus.Properties.Set("ContentType", (uu) purpose, hints)`（`src/ibusinputcontext.c:ibus_input_context_set_content_type`，`Properties.Set` 带 `IBUS_INTERFACE_INPUT_CONTEXT/ContentType`），`fcitx5: SetCapacity/SetContentType(u)`；`purpose` 由 `ContentType.purposeFromInputType` 映射，`PurposePassword` 仍禁 `BeginComposing`。均 `godbus` 同步 `Call`。
- **与上层对接**：`UpdatePreeditText → UpdateComposingText+delta(OldText=composing_before)`、`CommitText → AddText+delta(replace_range=was_composing?composing_before:selection_before)`、`ForwardKeyEvent` 按 `filter` 拦截；`D-Bus` 无 `done` 批事件，`batchDepth` 仅用于 `Editor` 内部去重（`BeginBatchEdit/EndBatchEdit` 抑制 `OnChange`/`updateEditingState`，末层 `epoch++` 去重），与 `Wayland` 的 `done` 互斥模型对齐，见 §8 C4。
- **守护重启与热切（godbus NameOwnerChanged · 懒重连：下次输入自动用新引擎）**：`godbus` 同时订阅 `org.freedesktop.IBus` 与 `org.fcitx.Fcitx5`（兼容 `org.fcitx.Fcitx`）的 `NameOwnerChanged`（`sender='org.freedesktop.DBus',member='NameOwnerChanged'`，`ibus/src/ibusbus.c:_connection_dbus_signal_cb` 同源），**不立即重建**——`owner` 由有变空→ 标记 `imeDirty=true` 并 `FocusOut+EndComposing` 置 `IME==nil`（英文直通），由空变有→ 仅标记 `imeDirty=true` 并 `GPUI_IME_DEBUG` 记 `ime switch dirty`；**真正重建在下次输入时懒触发**：`EnableIME/FocusIn`、`UpdateCursorRect`、`ProcessKeyEvent`、`SetSurroundingText` 任一入口先判 `imeDirty||IME==nil`，则按“先 `ibus` 再 `fcitx5`”重探 `CreateInputContext`（`ibus s→o`，`fcitx5 ss→o`，`godbus Call` 超时 `500ms`），成功则清 `imeDirty` 并补 `FocusIn+SetCapabilities/SetCursorRect+SetSurroundingText`，失败保持 `nil` 仍直通。指数退避 `200ms→2s` 仅用于 `Bus` 断开（`conn.Signals` 关闭/`read error`）后的重拨，`P13` 守护重启与 **ibus↔fcitx5 热切**均走此“脏标记→下次输入重探”路径，满足“切换后下次输入自动用当前生效的输入法”。
- **线程与分发（godbus 纯 Go）**：`godbus` 信号在独立 `go` 协程 `range conn.Signals()`，回调内不直接改 `Editor`，而 `pushIME(Event{Type:EventImePreedit/Commit/Delete})` 入 `x11Host.imeMu` 队列并 `WakeUp()`，由 `WaitEvents` 所在事件循环线程统一 `dispatch`（与 `Wayland` 的 `dispatch` 独占同）；`SetCursorLocation/SetSurroundingText` 等 `Call` 可在事件线程同步发（`godbus` 线程安全），无需跨线程。禁止在 `D-Bus` 协程直接触 `Editor`。

### 7.2 Windows

`text_input_plugin.cc` 的 `TextHook/ComposeBegin/Change/Commit/End/KeyboardHook` 同 Linux；`enableDeltaModel` 分支同（delta vs 全量），`TYPE_TEXT_VARIATION_PASSWORD` 禁组合，`firstRect` 仅 composing 时。

### 7.3 macOS/iOS

`FlutterTextInputPlugin.mm`：`setMarkedText→Begin+Update`，`insertText→AddText+Commit+End`，`unmarkText→Commit+End`，`firstRectForCharacterRange` 仅 `composing==true` 时返回 composing 框（含 `translate_coordinates`），`interpretKeyEvents` 双门模型（`filter_keypress` 等价）。

---

## 8. 并发契约（硬）

C1 状态与 Editor 仅事件循环线程；C2 Timer 仅投队列+WakeUp；C3 adapter 标注线程语义；C4 Wayland `zwp_text_input_v3.done` 批处理队列 `dispatch` 独占，与 `batchDepth>0` 互斥（`done` 进队列后不立即 `updateEditingState`，待 `EndBatchEdit` 的 `epoch++` + `ShouldSkipFrameworkUpdate` 去重后一次性发送），**X11 D-Bus 无 `done`，`batchDepth>0` 仅抑制 `OnChange` 与 `pushSurrounding/pushDelta`，`D-Bus` 信号协程仅 `pushIME+WakeUp`，`dispatch` 仍独占事件循环线程**。`batchDepth>0` 时抑制 `OnChange` 与 `updateEditingState`，`EndBatchEdit` 时一次性 `epoch++` 并按 `ShouldSkipFrameworkUpdate` 去重。

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


## 13. 修订

| 版本 | 说明 |
|---|---|
| **v3.14 2026-08-29** | R5 已落：§12 中 R5 3 项标 ✅（禁用全拦截 `InputBox/Viewport/Multi`+`InputRouter` 感知、`SetMaxLines` 单行忽略、`warmup` 诚实化 `r5/main.go`+`wrgate/report.go`），§11 R5 行标 ✅；`metrics-audit` A-J 10族全审 + 三证据（探针+像素+Golden）通过 |

## 14. 六阶段可交付计划（按 §7 顺序 · X11 D-Bus 专用）

> **定位**：`R1-R5`（`F-A/B/C/D/E/F/S` + `§6/§8/§9/§10`）已在 Wayland 真源落盘，**本章仅拆 §7 平台实现**为 6 个可独立合入、独立验证的阶段。顺序即文档 §7 黑点顺序，前一阶段是后一阶段的地基；每阶段结束按 §10 P1-P15 + A-J 10族门禁验。**本表已合并 S7-S10 有用增量（AttrList/多显 DPI/连接复用/异步/探测顺序/序列号/极端用例），删去候选词列表/XIM/定时节流/全局状态重复项，S1-S6 仍为 6 阶段。**

| 阶段 | 对应 §7 黑点 + 合并增量 | 功用 | 改动文件 | 验收（P关联） |
|---|---|---|---|---|
| **S1 总线** | 依赖与会话总线 + **连接复用**（S8） | `godbus/dbus/v5` 纯 Go 接通会话总线，进程内**单 `dbus.Conn` 复用**多窗口 `InputContext`，`AddMatch` 订阅 `org.freedesktop.IBus / org.fcitx.Fcitx5`，无守护时 `Window.IME()==nil` 降级，`GPUI_IME_DEBUG` 打 `dial/hello/match` | `go.mod` + 新建 `ui/platform/x11_dbus_ime_linux.go`（`//go:build linux`，结构体 `x11Ime{conn, engine, objectPath, imeDirty, focused, mu}`，`conn` 为包级单例）+ `x11_linux.go:imeForX11` 去桩 | 建窗不阻塞；多窗口 `DBus` 连接数恒为 1；有/无守护均 `go vet` 0；`P8 首焦` 前的 `nil` 降级英文直通 |
| **S2 上下文** | 打开与探测 + 能力声明 + **探测顺序优化 & 异步化**（S8/S10） | 双探**优先读 `GTK_IM_MODULE/QT_IM_MODULE/XMODIFIERS`** 定顺序（`ibus`→先 `ibus`，`fcitx`→先 `fcitx5`，空值再先 `ibus` 后 `fcitx5`），各 `500ms` 超时；`ibus CreateInputContext(s)→o`（`src/ibusbus.c`）、`fcitx5 CreateInputContext(ss)→o`（`DBusFrontend.cpp`），成功缓存 `engine` 并 `SetCapabilities`（`ibus CAP_PREEDIT(1<<0)\|FOCUS(1<<3)\|SURROUNDING(1<<5)=41`【`ibustypes.h: IBUS_CAP_*`】 / `fcitx5 CAPACITY_PREEDIT\|SURROUNDING`）；**探测/建上下文全异步化**，后台协程完成前 `IME==nil` 直通不卡建窗；每窗口一 `InputContext`，`Window.Close` 时 `Destroy + RemoveMatch` | `x11_dbus_ime_linux.go:S2` | `P12 双引擎` 符合系统默认优先级；双窗路径不同；`Close` 不泄漏；`IBus 1.5.20+ / Fcitx5 5.0/5.1` 主流发行版全绿 |
| **S3 会话与锚点** | 使能/失能 + 锚点 + 环绕/内容 + **多显高 DPI**（S8） | `EnableIME(rect)→FocusIn幂等 + SetCursorLocation + SetContentType + SetSurroundingText`；`DisableIME→FocusOut+EndComposing`；`UpdateCursorRect` 取 `TextLayout.CaretForOffset - scrollX × ScaleFactor → XTranslateCoordinates到根窗口`，**结合 `RandR` 多显示器修正物理坐标**，分发 `ibus SetCursorLocation(iiii)/fcitx5 SetCursorRect(iiii)`（`w=2 h=行高`），仅 `composing==true` 实发（相同 `rect` 跳过防回声）；`SetSurroundingText` 复用 `textinput.TruncateSurrounding` 4000居中，`ibus variant(IBusText{attrs,text}) + u+u`（`attrs` 可空但类型必为 `v` 包 `IBusText`，`fcitx5 s+u+u` 明文）；`SetContentType` 映射 `purpose` | `x11_dbus_ime_linux.go:S3` + `x11OpenLib` 多绑 `XTranslateCoordinates` + `x11_linux.go` 坐标换算扩展 | 候选窗钉光标，跨显示器不漂移；组合期才实报（`OnChange→RefreshIMEAnchor` 闭环）；`P14 36×m` 不漂移；`P15 4000` 4锚点；密码 `PurposePassword` 禁组合 |
| **S4 按键过滤** | 按键过滤 `ProcessKeyEvent` | `XKeyPress` 先 `ProcessKeyEvent` 再本地插入：`ibus ProcessKeyEvent(uuu)→b / fcitx5 ProcessKeyEvent(uuuub, time/isRelease)→b`，`keyval = XKeycodeToKeysym(dpy,keycode, (state&ShiftMask)?1:0)`【`Xlib: index0=裸键, index1=Shift`】，`handled==true` 拦截，`false` 再走 `Home/End/PageUp/Down/Return(仅MULTILINE+newline→AddCodePoint)`；`client_id==Unset` 时直接放行；`50ms` 超时放行；死键兜底 `XLookupString→AddText` | `x11_linux.go:drainX` 前置过滤 + `x11_dbus_ime_linux.go:S4` | `P9 风暴锁` 单键≤2 preedit；组合中方向键不越 `editable_range`；`P11 死键` 正常 |
| **S5 通道归一** | 预编辑/提交/删除 + 与上层对接 + **AttrList**（S7） | `godbus Signal` 订阅归一：`ibus UpdatePreeditText(v,u,b)/CommitText(v)/DeleteSurroundingText(i,u)/HidePreeditText` + `fcitx5 UpdatePreedit(s,i)/CommitString(s)/DeleteSurroundingText(i,u)` → 归一 `Editor.Begin+UpdateComposingText / AddText+CommitComposing / DeleteSurrounding`（`offset/n` 按 code point，`variant` 按 `dbus.Variant` 解 string），`delta replace_range = was_composing?composing_before:selection_before`；**新增 `ime_format.go` 解析 `IBus AttrList/Fcitx5 格式化` → `Segment` 还原下划线/高亮/选中段**；`D-Bus` 无 `done`，`batchDepth` 仅抑 `OnChange`（`Wayland done` 互斥模型已由 `batchDepth` 复用，无需额外 `serial`） | `x11_dbus_ime_linux.go:S5` + 新增 `ui/platform/ime_format.go` + `x11Host.pushIME+WakeUp` 复用 Wayland `queue` 模型 | `P2 拼音→候选→上屏` 原子替换；`P16 预编辑高亮选中态` 还原准确；`P3 组合中退格`；`P4 Esc` 清；提交后组合状态强制清零 |
| **S6 稳定与分发** | 守护重启与热切 + 线程分发 + **极端用例**（S10） | 订阅 `NameOwnerChanged` 仅置 `imeDirty=true`（有→空 `FocusOut+EndComposing` 置 `nil`，空→有仅脏标记），下次 `Enable/UpdateCursorRect/ProcessKeyEvent/SetSurrounding` 入口懒重探（按 S2 顺序，`500ms` 超时，成功清脏并补 `FocusIn+SetCapabilities+SetCursorRect+Surrounding`）；`Bus` 断开才 `200ms→2s` 退避重拨；`godbus range Signals` 独立协程仅 `pushIME+WakeUp`，`dispatch` 独占事件循环线程；**极端用例：守护反复重启、焦点快速切换、连续开关窗 1000 次无泄漏无错乱** | `x11_dbus_ime_linux.go:S6` | `P13 守护重启` 恢复或降级；`ibus↔fcitx5 热切` 下次输入自动换；`P17 极端 1000 次` 无泄漏；A-J 10族 `metrics-audit` 全绿 |

> **落盘顺序**：`S1→S2` 连做（无 `InputContext` 后续无意义），随后 `S3→S4→S5→S6` 线性推进；每阶段 PR 以 `feat/ime-x11: S{n} ...` 为前缀，合入前 `go vet ./ui/platform ./ui/textinput` 0 且对应 P 子集仿真通，`S5` 通后跑 `ui_wr_ime_r3_channel` 真窗双引擎 `P1-P9`，`S6` 通后满足 `§10.4 R3` **10族全硬**方可关窗。**已删**：`S7 候选词列表/翻页`（撞 §1.2 非目标，自绘候选窗另起 R）、`S7 GlobalEngineChanged`（与 `NameOwnerChanged` 重复）、`S8 XIM 降级`（撞“仅 D-Bus”纪律）、`S9 16ms 定时节流`（改为相同 `rect` 跳过）。

### 14.1 分阶段“验什么 / 什么效果 / 输出什么”总表（真窗手打验收，每阶段必过）

> **怎么验**：统一开 `GPUI_IME_DEBUG=1 go run ./examples/ui_wr_ime_r3_channel`（1200×800，`RunFor=0` 手动点 X 关，`RUN_SECONDS` 仅校验最小观察时长）。**三证据**：① `GPUI_IME_DEBUG` 日志 ② 真窗画面（以实际绘制墨迹为真值，`F0-F9` 选型）③ 结束 `A-J 10族 JSON`。下表每行一个功能点，说明“验什么、看到什么效果、留下什么输出”。

| 阶段 | 功能点（§7 黑点） | 验什么（输入交互） | 预期效果（画面/行为） | 输出什么（日志/JSON/文件） |
|---|---|---|---|---|
| **S1 总线** | 会话总线接通 + 单 `dbus.Conn` 复用 | `go run` 开 2 个窗分别点获焦；`ss -x` / `dbus-monitor --session` 看连接数 | 两窗共享 1 个 `dbus.Conn`，建窗不阻塞，1 秒内现窗 | `stderr: dbus dial/hello/match` 各 1 次（`GPUI_IME_DEBUG=1`）；`go vet 0` |
| S1 | `AddMatch` 订阅 ibus/fcitx5 | 有守护与无守护各建窗一次 | 有守护正常，无守护 `Window.IME()==nil` 英文直通不崩 | 日志 `AddMatch org.freedesktop.IBus / org.fcitx.Fcitx5`；`P8 首焦前 nil 降级` PASS |
| S1 | `GPUI_IME_DEBUG` 打点 | `GPUI_IME_DEBUG=1` 建窗 | 仅调试态打点，默认静默 | 日志含 `dial/hello/match` 明文，无则 FAIL |
| **S2 上下文** | 探测顺序 `GTK_IM_MODULE/QT_IM_MODULE/XMODIFIERS` 定优 | `GTK_IM_MODULE=ibus` / `=fcitx` / 空 各跑一次，点框获焦 | `ibus→先 ibus`，`fcitx→先 fcitx5`，空值 `ibus→fcitx5`，符合系统默认 | 日志 `probe order: ibus / fcitx5` + `CreateInputContext(s)→o / (ss)→o` 各 500ms 超时记录 |
| S2 | `CreateInputContext` 官方签名 | 任一引擎下建窗获焦 | `ibus client_name="gpui:<进程名>"` 得 `InputContext_7`，`fcitx5 appname/appid="gpui"` 得 `InputContext_2`，双探失败则 `nil` | 日志 `engine=ibus/fcitx5 objectPath=/org/.../InputContext_*` |
| S2 | `SetCapabilities` | 获焦后紧接打字前 | `ibus CAP 1<<0|1<<2|1<<3=41` / `fcitx5 PREEDIT\|SURROUNDING` 已发 | 日志 `SetCapabilities 41 / SetCapacity` 各 1 次 |
| S2 | 全异步化不卡建窗 | 故意 `systemctl --user stop ibus` 让一端 500ms 超时，立即点框打 `hello` | 探测未完成前 `IME==nil` 直通，打字不卡 | `wrgate: time_to_first_present_ms <1000`（`wrgate/report.go` JSON H 族） |
| S2 | 每窗一 `InputContext` + `Destroy+RemoveMatch` | 开双窗再关一窗，`dbus-monitor` 观察 | 关窗路径消失，不串扰，另一窗仍可用 | 日志 `Destroy / RemoveMatch`；`lsof` fd 不涨，`-race` 0 |
| **S3 会话与锚点** | `EnableIME→FocusIn幂等` | 点框 A→框 B→回 A，快速来回点 | 重复 `FocusIn` 不刷屏，仅首次生效，画面不闪 | 日志 `FocusIn` 1 次/获焦，重复幂等 `focused guard` |
| S3 | `DisableIME→FocusOut+EndComposing` | 点外部失焦 / `Esc` | `composing` 清零，候选消失 | 日志 `FocusOut + EndComposing` |
| S3 | `UpdateCursorRect` 经 `TextLayout.CaretForOffset - scrollX × ScaleFactor → XTranslateCoordinates→根窗口` + `RandR` 多显修正 | 输 `nihao` 进 preedit，拖窗跨主/副屏，`scale 1.25/2.0`，单行横滚后看 | 候选窗死钉光标，跨屏/HiDPI 不漂移，`w=2 h=行高` | 日志 `SetCursorLocation(iiii) / SetCursorRect(iiii) x,y,w=2,h` 物理像素；`P14 36×m` 真窗像素 `F0-F9` 墨迹间隙 PASS |
| S3 | 仅 `composing==true` 实发 + 相同 `rect` 跳过 | 空闲移动光标 vs preedit 中移动光标 | 非组合期 0 上报，组合期 1 次/变更，相同 rect 不重发防回声 | 日志 `composing_rect real-report 1 / preheat 0 + skip same rect`；`OnChange→RefreshIMEAnchor` 闭环日志 |
| S3 | `SetSurroundingText` 4000 居中 + `-1` NUL | 贴 5000 字长文，光标放 0/1/4/3/4/末尾触发 `retrieve-surrounding` | 超长以光标为中心截断含 NUL，不崩不丢 | 日志 `SetSurroundingText len<=4000 centered`；`P15 4锚点` 单测 `TestSurroundingCenter4` 与真窗同阈值 PASS |
| S3 | `SetContentType` `purposeFromInputType` | 普通框 vs `PurposePassword` 框分别获焦 | 密码框关预测，`composing` 禁止 | 日志 `SetContentType purpose/password`；密码真窗掩码像素 `●` + 日志无候选 |
| S3 | `XTranslateCoordinates` 多绑 | `x11OpenLib` 符号检查 | 无 `undefined symbol` | `go vet 0` + `ldd` 无缺符号 |
| **S4 按键过滤** | `ProcessKeyEvent` 分流 `handled==true` 拦截 | preedit 中按 `← → Home End PageUp/Down` 再按字母 `a` | 被输入法消费的键不进 `Editor.AddText`，未消费才走本地 | 日志 `ProcessKeyEvent uuu/uuuub handled=true/false`；`P9 风暴锁` 单键≤2 preedit |
| S4 | `keyval = XKeycodeToKeysym(dpy,keycode, (state&ShiftMask)?1:0)` | 按 `a` vs `Shift+a`，`Ctrl+←→` | 大小写与词跳正确，`index0=裸键 index1=Shift` | 日志 `keyval/keysym` 与 `XLookupString` 一致 |
| S4 | `50ms` 超时放行 + `client_id==Unset` 放行 | 拔守护模拟超时；未建 `InputContext` 时直接打字 | 超时后本地插入不丢字，`Unset` 时直接放行 | 日志 `ProcessKeyEvent timeout 50ms → pass-through` |
| S4 | 特殊键分流 `Return(仅MULTILINE+newline→AddCodePoint)` | 单行按回车 vs 多行按回车 | 单行 `onSubmitted`，多行换行+`performAction` | 日志 `Return→AddCodePoint('\n')` 仅多行 |
| S4 | 死键兜底 `XLookupString→AddText` | 按 `´` 再按 `e` | 出 `é` 重音正确 | 真窗字符 `é` 像素 + 日志 `deadkey → AddText é`；`P11` PASS |
| **S5 通道归一** | `godbus Signal` 归一四信号 | `nihao→你好` 选词上屏；`Backspace` 缩拼音；`Esc` 清；`DeleteSurrounding` 触发 | `ibus UpdatePreedit(v,u,b)/Commit(v)/Delete(i,u)/Hide` 与 `fcitx5 UpdatePreedit(s,i)/CommitString(s)/Delete(i,u)` 均归一到 `Begin+UpdateComposing / AddText+Commit / DeleteSurrounding` | 日志 `UpdatePreeditText / CommitText / DeleteSurroundingText` 归一；`dbus.Variant→string` 解码日志 |
| S5 | `delta replace_range = was_composing?composing_before:selection_before` | 有选区时打拼音替换，提交看 | 原子替换选中段或 `composing_range`，不残留 | 日志 `delta OldText/composing_before/selection_before/DeltaStart`；单测 `TestDelta_NonTextUpdate` 同阈值 |
| S5 | `AttrList→Segment` 高亮还原 | 看 preedit 下划线/高亮/选中段（三段式） | 下划线/选中段颜色与 `IBus AttrList / Fcitx5 format` 一致 | 真窗像素 `F6` 选型 + `ime_format.go: Segment{underline/highlight/selected}` 日志；`P16` PASS |
| S5 | `batchDepth` 抑 `OnChange`（无 `done`） | 快速连击中看 | `D-Bus` 无 `done`，`batchDepth>0` 仅抑 `OnChange/pushSurrounding/pushDelta`，末层 `epoch++` 去重 | 日志 `batchDepth>0 suppress OnChange` + `epoch++ ShouldSkipFrameworkUpdate` |
| S5 | `variant Text` 解 `string` | `CommitText(v)` 含 `IBusText{attrs,text}` | `godbus dbus.Variant` 正确解 `string`，`attrs` 可空但类型为 `v` | 日志 `variant IBusText decoded` |
| **S6 稳定与分发** | `NameOwnerChanged` 仅置 `imeDirty=true` 懒重探 | 运行中 `killall ibus-daemon; fcitx5 &` 热切 `ibus↔fcitx5`，下次再打字 | 有→空 `FocusOut+EndComposing→nil` 英文直通；空→有仅脏标记，下次输入按 S2 顺序 500ms 重探成功清脏并补 `FocusIn+SetCapabilities+SetCursorRect+Surrounding` | 日志 `NameOwnerChanged owner有→空/空→有 imeDirty=true` + `lazy reprobe success/clear dirty`；`P13` PASS |
| S6 | `Bus` 断开 `200ms→2s` 退避 | `dbus-daemon` 重启 / `conn.Signals` 关闭 | 仅 Bus 断开走退避，普通 `NameOwnerChanged` 不退避 | 日志 `bus disconnect backoff 200ms→2s` |
| S6 | `godbus range Signals` 独立协程仅 `pushIME+WakeUp`，`dispatch` 独占 | preedit 中狂拖选、快速切焦点、连打 | `D-Bus` 协程不直接改 `Editor`，事件循环线程统一 `dispatch`，`-race` 0 | 日志 `pushIME EventImePreedit/Commit/Delete + WakeUp`；`x11Host.imeMu queue` 深度日志 |
| S6 | 极端用例 1000 次 | 脚本连续开关窗 1000 次、焦点快速来回切、守护反复重启 | 无泄漏无错乱无崩 | `P17` PASS；`rss_after_close <= rss_peak`；`go test -count 1000 -run TestX11ImeCloseLeak` |
| S6 | `A-J 10族全硬` 关窗 | 开窗 ≥10s（`RUN_SECONDS=10` 仅校验最小观察时长，`RunFor=0` 手动点 X 关） | `metrics-audit` 全绿才可回写 `§11` | `stderr JSON: A fps_interval≥55 p95≤22 hitch≤5, B build p95<5, C damage_ratio可≈1但paint可解释, D cpu<60%, E rss_slope<15000, F cpu_fallback==0, G measure_cache必采, H ttfp<1000, I baseline<10%必开, J vet==0+filter日志` + 像素 `F0-F9` + `Golden` 三证据 |

### 14.2 每阶段真窗手打验收清单（在 `ui_wr_ime_r3_channel` 里逐项点）

- **S1**：开 2 窗看单连接 → 无守护建窗打 `hello` → `GPUI_IME_DEBUG` 有 `dial/hello/match` → `go vet 0` → 点 X 看 `H ttfp<1000`
- **S2**：`GTK_IM_MODULE` 三态各开一次 → 双窗路径不同 → 关窗 `Destroy/RemoveMatch` → `P12` 探针 `engine` 分别为 `ibus/fcitx5` → `wrgate JSON engine` 字段可溯源
- **S3**：`nihao` preedit 跨屏拖窗 → `Scale 1.25/2.0` → 横滚后光标仍钉候选 → 空/末/1/4/3/4 四锚点各贴 5000 字 → 密码框输 → 程序化 `SetText` 看锚点闭环 → 结束 `C paint_count` 可解释
- **S4**：preedit 中 `←→HomeEnd` 不越界 → `10ms 内 20 键` 风暴 → `´+e=é` → 单/多行回车分流 → `client_id Unset` 放行 → 超时 50ms 放行 → `A hitch≤5`
- **S5**：`拼音→候选→上屏` 原子替换 → 选中段拼音替换 → 组合中退格/Esc → 提交后 `composing==false` → 三段高亮段像素 → `delta replace_range` 日志 → `B build p95<5`
- **S6**：`kill ibus/fcitx5` 热切下次输入自动换 → `Bus` 断开退避 → 快切焦点 → 1000 次开关窗 → ≥10s 后点 X → `A-J 10族全硬 + 三证据` → `metrics-audit` 审过才标 ✅

## 附录：偏移与截断

- `TextRange` 以 UTF16 记（`{-1,-1}` 哨兵），Go 边界 `Utf8ToUtf16/Utf16ToUtf8` 互转，surrogate 对按 2 计仅换算时 ×2；
- `surrounding` 4000 bytes 超长以光标为中心 4 锚点截断，末尾含 NUL，GTK/Wayland 共用 `-1`；`X11 D-Bus` 的 `cursor/anchor` 为 `UTF8 byte` 偏移（`anchor==cursor` 无选区），`offset/n` 按 `code point`，均复用 `textinput.TruncateSurrounding`，`godbus` 侧 `int32` 传参；
- `X11` 统一走 `SetCursorLocation/SetCursorRect(x,y,w=2,h)` 的 `2×行高` 光标矩形，经 `XTranslateCoordinates` 到根窗口，`ScaleFactor` 转物理像素；
- 闪烁 `caretOn` 节拍 ~500ms，`BaseEditable` ≤15 行 `grep -c` 审计；
- `D-Bus` 依赖 `github.com/godbus/dbus/v5` **纯 Go 无 cgo**，会话总线 `dbus.SessionBus()` 自解析 `DBUS_SESSION_BUS_ADDRESS`，`X11` 每窗口一 `InputContext` 对象路径，`Destroy` 于 `Window.Close`；参考 `GTK gtkimcontextibus.c`，实现库 `godbus/dbus/v5`。
