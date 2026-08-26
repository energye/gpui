# IME 现代标准设计（v1.1 · 经红队审查修订 · 待审定后转真源）

> **性质**：设计文档。**审定通过前不动代码**；通过后本文即 IME 线真源，取代 `ENGINE_INPUT_IME_PLAN.md` 中被本设计推翻的部分（该文档保留为历史账本：v1.x–v2.x 问题清单与销账记录仍有追溯价值）。
> **范围**：三端（Windows / macOS / Linux-Wayland / Linux-X11）原生平台实现 + 统一抽象规范 + 控件接入契约 + 测试验收体系。
> **方法论**：对标四家成熟实现（Chromium / Flutter Engine / Qt / GTK4）提取共性标准 → 与现状逐项差异及理由 → 分层重设计 → 平台逐端设计 → 测试矩阵 → 里程碑。
> **v1.1**：经独立红队审查修订——新增 §4.0 并发契约、§4.5 ComposedView、macOS 桥接路线改正（自建子类）、事件词汇表归一到 `ui/input`、适配器补拉取回调、编辑纪元、图素簇措辞；详见 §10。

---

## 0. 目标与非目标

**目标**

1. 生产级三端 IME：中文（拼音为主）/日文/韩文可用，英文直通零干扰；
2. **统一接口实例**（用户目标句的准确形态）：控件与编辑器只依赖一个门面实例 **`ImeSession`**——入站事件由它消化（它实现 `ImeEventHandler`）、出站动作由它暴露（普通方法）；平台侧两个接口（出站 `PlatformIMEAdapter` / 入站 `ImeEventHandler`）是它的两半，由宿主每窗口装配一次。**控件层永远不直接接触平台接口**（对齐 GTK GtkIMContext 单对象双通道形态）；
3. 现代标准语义：preedit 与缓冲区分离（S2）、surrounding 受控供给（S3）、会话状态机显式化（S4）；
4. 可验证：状态机全转移单测 + headless 协议仿真 + 真窗像素取证三级证据链。

**非目标**

- 自绘候选窗（跟随系统候选 UI）；
- 手写/语音输入；输入法引擎本身；
- RTL 文本特殊布局（删除的 before/after 按**逻辑序**解释，RTL 不支持——明文声明）；
- 图素簇级编辑边界：本轮实现 rune 粒度吸附，规范措辞已按图素簇表述，升级另立轮次（印地语系等复杂脚本受此影响，CJK 无感）。

---

## 1. 业界参考实现对照

| 维度 | Chromium | Flutter Engine | Qt 6 | GTK4 |
|---|---|---|---|---|
| 客户端接口 | `TextInputClient` | `TextInputClient` + `TextInputConfiguration` | `QInputMethodEvent` + `inputMethodQuery` | `GtkIMContext` 信号族 |
| preedit 存储 | 不进缓冲，range 标注 | 不进缓冲，`composing` span | 不进缓冲（event 携带） | 不进缓冲 |
| surrounding text | 拉取（ITextStoreACP/client 回调） | 推送（仅本地变更后 setEditingState） | 拉取（ImSurroundingText） | 拉取（retrieve-surrounding 信号） |
| 删除周围文本 | `ExtendSelectionAndDelete` 显式调用 | 全量状态同步 | IM 交互内 | `delete-surrounding` 信号 |
| 光标锚点 | composition info | editable region hint | cursorRectangle property | cursor-location set |
| Win 后端 | TSF 主 + IMM32 备 | IMM32/TSF-lite | QPA TSF+IMM32 | imm module |
| mac 后端 | NSTextInputClient 直通 | FlutterTextInputPlugin（NSView 子类） | QCocoaInputContext | — |
| Linux 后端 | ibus/zwp 双通道抽象 | GTK im module | ibus Qt 插件 | im module |

**五原则 S1–S5**（四家一致）：S1 宿主持有编辑状态真源；S2 preedit 不进缓冲区；S3 surrounding 受控通道（拉取或本地变更驱动推送）；S4 会话生命周期焦点驱动；S5 对文本的修改除 commit 外走显式 API。
现状 v1.x 违反 S2/S3/S5 的三处正是回声风暴、点击错位、删除死路的结构性根源。

---

## 2. 核心设计决策（与现状差异 · 逐条理由）

| # | 决策 | 现状 | 理由 |
|---|------|------|------|
| D1 | **preedit 移出缓冲区**：Editor 内部 `comp *Composition` 覆盖段；显示串渲染时拼装；提交原子插入缓冲 | preedit 直接 Insert 进缓冲 | S2；根除撤销污染/字数错/点击漂移/CancelCompose 字符串手术；日文分文节属性成为可能 |
| D2 | **surrounding 按需供给**：默认关闭推送；① 适配器可经 Provider 拉取；② 仅本地 commit/delete 后推一次。不变量：**处理入站事件的同步路径内禁止任何出站推送**；所有推送携带单调递增 editEpoch | afterEdit 无条件推（v1.8–v2.3 一路补丁） | S3；回声风暴的结构性根治；epoch 让 mac/IMM32 这类无 serial 平台也有可断言凭据 |
| D3 | **显式 DeleteSurrounding API**，废除负数载荷魔法数 | IMEDeleteSurrounding 用 Start=-before/End=after | S5；语义自描述 |
| D4 | **会话状态机显式化** + §4.0 并发契约 | 隐式状态散落 | v2.2/v2.3 时序 bug 均源于隐式状态 |
| D5 | **窄适配器接口**：`PlatformIMEAdapter{Enable/Disable/CaretMoved/SetPurpose/PushSurrounding/SetSurroundingProvider}` + `ImeEventHandler` 入站注入；命令侧 SetComposing/Commit 从契约移除（I9） | platform.IME 大接口含空转命令 | 提交权在输入法侧 |
| D6 | **KeyRepeater 归一接管按键重复**（Wayland 合成器迁入；X11 M4 评估迁移；win/mac 用系统自带） | Wayland 已有客户端合成 | 行为一致 + 确定时钟可测 |
| D7 | **done(serial) pending 队列**（Wayland），pending 状态归 dispatch 线程独占（见 §4.0） | 随到随应用 | zwp 规范要求；I7 正式修复 |
| D8 | **ContentType{Purpose, Hint} 完整建模**全端透传 | purpose 有、hint 未建 | 密码框禁候选是硬需求 |

---

## 3. 分层架构（按包表述）

```text
ui/platform   L0a PlatformIMEAdapter / ImeEventHandler / ContentType / FieldSnapshot（接口）
              L0b linux_wayland.go · linux_x11.go · windows_imm32.go(→windows_tsf.go) · darwin_textinput.go
ui/input      L1  ImeEvent 归一（扩展现有 IMEEvent：+Segments +Session，废负数载荷）· KeyRepeater
ui/textinput  L2  Editor（保名换内脏：缓冲真源 + comp overlay + ComposedView + editEpoch）
              L2  ImeSession（门面/状态机：实现 ImeEventHandler，向上暴露动作方法）
ui/embedder   L1' InputRouter 会话接线（TextEditTarget 契约与 FocusNode.Target 机制原样保留）
ui/kit        L3  EditableWidget + BaseEditable（默认实现）+ DrawPreedit 钩子
```

依赖纪律：向下单向；L0b 各实现互不可见；L1 以上零平台词（CI grep 门禁保留）。六个概念实际四个包——不再画成六层。

---

## 4. 统一规范层

### 4.0 并发契约（硬 · 红队 P0-1）

- C1 状态机转移与 Editor 全部变更**只发生在事件循环线程**（Wayland poll 排空线程 / 平台消息线程）；
- C2 一切 `time.Timer`/外部 goroutine **只允许向事件队列投递或触发 WakeUp，禁止直接 marshal 协议请求或改会话状态**（现状 refreshTextInput 兜底轮的 AfterFunc 直发列入本次重构改造范围）；
- C3 适配器公开方法标注线程语义：要么「任意 goroutine 可调（内部投递到循环线程）」，要么「仅循环线程」；
- C4 Wayland done 队列与 pending 状态归 dispatch 线程独占所有。

### 4.1 入站事件（平台 → 归一；词汇表归一到 `ui/input.IMEEvent`）

扩展现有类型（不造平行词表）：`IMEEvent` 增 `Segments []Segment{Start,End,Attr}` 与新 kind `IMESession`；废除负数 Start/End 载荷（D3）。映射：

| IMEEvent kind | 语义 | wl | TSF/IMM32 | mac |
|---|---|---|---|---|
| IMECompose(+Segments) | 组合串更新；""=清除终态 | preedit_string(done 后 flush) | GCS_COMPSTR | setMarkedText |
| IMECommit | 上屏原子插入 | commit_string | GCS_RESULTSTR/WM_IME_CHAR | insertText |
| IMEDeleteSurrounding{Before,After} | 显式删除（正数载荷） | delete_surrounding_text | TextStore 通知 | （range 替换） |
| IMESession(active bool) | 引擎侧会话开合 | enter/leave | focus change | 主窗状态 |

规则：R1 UTF-8 字节偏移、cursor<0=串尾、越界钳制；R2 空 preedit 只能终止组合（不得开启空组合——v1.5 教训）；R3 入站处理路径内禁止出站推送（S3 回声抑制不变量）。

### 4.2 出站动作（经 ImeSession 门面向下）

`Enable(field)`（焦点进入）、`Disable()`（焦点离开）、`CaretMoved(rect)`（本地编辑/**preedit 变化后**/滚动导致锚点位移——组合串宽度改变候选窗位置，必须重报，这是 v1.x 老 bug 的复发口，明文写入触发时机）、`SetPurpose(ct)`、`PushSurrounding(text,cursor)`（仅本地变更后，带 editEpoch）。

### 4.3 L0a 适配器接口（冻结前终版 · v1.2 分层修正）

> **v1.3 分层修正（实现期发现）**：`PlatformAdapter`/`ImeEventHandler` 接口若定义在 `ui/platform`，其签名引用 `input`/`textinput` 类型会构成 import 环（input→platform 是既有纪律）。故：**平台侧只保留纯原生绑定 + 旧式窄能力接口**；adapter 实现与 ImeEventHandler 转发放 **embedder 层**（它本就同时依赖 platform 与 textinput）。§3 图中 L0a 接口框归属 ui/embedder。

```go
// ui/platform —— 纯能力接口（无上层类型）
type IME interface { EnableIME(Rect); UpdateCursorRect(Rect); SetContentType(ContentPurpose); SetComposing(string,int); DisableIME() }

// ui/embedder —— adapter 与入站转发（依赖双方，无环）
type imeAdapter struct{ ime platform.IME; ed *textinput.Editor; field TextEditTarget }
// 实现 textinput.PlatformAdapter；ImeEventHandler 由 ImeSession 实现
```

所有权规定：adapter **每窗口一个**（NSView/zwp_text_input/HWND 天然 per-surface）；`Window.IME()` 为 nil 时降级路径保留（stub_host 模式不变）；宿主装配 = 创建 adapter + new ImeSession(adapter) + adapter 注入 handler。

### 4.4 Editor（L2，保名换内脏）

```go
type Editor struct {
    buf  string      // 已提交文本真源（不含 preedit）
    sel  [2]int
    comp *Composition // nil = 无组合 {Text; Cursor; Segs}
    epoch uint64     // 每次真实变更 +1，随 PushSurrounding 带出
}
```

要点：Undo 分组规则本轮定死——**一次完整组合 = 一个撤销步**（埋点即按此记录）；鼠标拖选跨组合边界时从边界钳制。

### 4.5 ComposedView —— 偏移换算的唯一出口（红队 P1-2）

preedit 出缓冲后，「缓冲偏移 ↔ 显示偏移」换算是高危区。规范：**除 `ComposedView` 外任何代码不得自行做该换算**（单测锁定）。

```go
type ComposedView struct {
    Display   string        // buf前段+comp.Text+buf后段
    CompStart, CompEnd int  // 显示坐标下的组合区间
}
func (e *Editor) View() ComposedView
func (v ComposedView) MapBufToView(off int) int          // 含钳制版本
func (v ComposedView) MapViewToBuf(off int) int
func (v ComposedView) AnchorRectInView(r [2]int, layout LayoutInfo) Rect // 日文分节/mac firstRect 需要组合段内子区间矩形；由布局结果回填几何
```

消费方（点击命中、选区高亮、锚点矩形、mac characterIndexForPoint、未来 Undo）一律经 View() 取视图再换算。

### 4.6 ImeSession 门面（L2）

```go
// 使用层唯一依赖。构造：NewImeSession(adapter PlatformIMEAdapter) *ImeSession
type ImeSession struct { /* 实现 ImeEventHandler；状态机所在 */ }
func (s *ImeSession) AttachEditor(ed *Editor)         // 焦点驱动
func (s *ImeSession) SetContentType(ct platform.ContentType)
func (s *ImeSession) CaretMoved(rect platform.Rect)   // 内部去重
func (s *ImeSession) PushSurroundingIfDirty()          // 仅本地变更后调用；epoch 判定
// 入站：ImeEventHandler 五方法 → 状态机转移 → Editor 变更 → OnChange 回调
```

状态机：`idle ⇄ active ⇄ composing →(commit/cancel)→ active`；非法转移 no-op 并计数（探针可见）。每条合法转移的单测断言副作用（出站调用次数/参数/epoch 不回退）。

---

## 5. 平台实现设计（L0b 逐端）

### 5.1 Linux · Wayland（zwp_text_input_v3）

保留 purego 绑定骨架与接口表；pending-state 队列（done(serial) flush，dispatch 线程独占——C4）；surrounding 按 D2；enter/leave → Session 事件 + leave 补 disable（v2.2 有效经验）；首激活兜底轮保留但**改为向事件队列投递的定时器事件**（C2 改造），存活仲裁沿用 v2.3（真实引擎输出即撤销兜底）。

**Adapter 粒度（2026-08-25 二次修正 · 实现期核实协议 XML 后定案）**：zwp_text_input_v3 的 `enable(1)[surface]`/`disable(2)[surface]` **自带 surface 参数**——「每窗口一个 ti 对象」即协议合法形态；此前引用的 SDL `set_input_surface` 是 SDL3 私有包装、非 zwp v3 标准，per-seat 改造项撤销。多窗口一致性由 enable(surface) 的参数化保证。~~对齐 SDL3/GTK4 per-seat 形态~~（保留记录供追溯）。

### 5.2 Linux · X11（XIM）

基线维持 PreeditNothing；commit 剥 Rune 不变量保持（v1.2）；PreeditPosition 升级在 M4 决策；服务端自动重复是否迁 KeyRepeater 见 §9.3。

### 5.3 Windows

分期：**M2 先 IMM32**——ImmGetContext/ImmProcessKey/ImmNotifyIME + WM_IME_STARTCOMPOSITION/COMPOSITION/CHAR 消息链（WndProc 分发表新增 case 组）+ ImmSetCompositionWindow 锚点 + GCS_COMPSTR/GCS_RESULTSTR 解析；**M2.1 TSF 增强**（ITfThreadMgr + ITextStoreACP：surrounding 拉取、InputScope 完整映射、属性范围）——纯 Go 手工 COM vtable 可行（仓库有 wl 接口表手工 ABI 先例），预算超支则 IMM32 版本即满足 P1–P10。purpose 映射：password/pin → 关候选预测；数字 → InputScope 建议。

### 5.4 macOS（红队 P0-2 路线改正）

- **桥接 = objc-runtime 自建子类**（非猴补丁）：`objc_allocateClassPair(NSView,…)` → `class_addMethod` 加 override → `objc_registerClassPair` → 窗口 contentView 换成该子类实例（FlutterTextInputPlugin 同形态；对进程内其它 NSView 零影响）。~~class_addMethod 替换 NSView 本体方法~~（addMethod 对已存在方法必失败；method_setImplementation 是全局猴补丁，hardened runtime 高危）；
- **重入护栏**：`interpretKeyEvents:` 会在 keyDown 栈内同步回调 setMarkedText/insertText —— Go trampoline 禁止 panic 穿透；主线程 LockOSThread；
- 方法映射：setMarkedText→PreeditChanged（attributes→Segments）、insertText→Committed、unmarkText→PreeditChanged("")、firstRectForCharacterRange← AnchorRectInView、selectedRange/markedRange/characterIndexForPoint ← View() 快照；
- **双门模型**：mac 组合期间原始按键仍达应用（经 interpretKeyEvents 正门后未消费才回落 key 通路），路由层需 shadowing 状态（组合中吞可打印键）；
- spike 失败降级：注入 NSTextView（代价=第二套编辑行为）；按子类路线 spike 成功率评估约九成。

---

## 6. 存量资产处置

| 资产 | 处置 |
|---|---|
| wayland 绑定骨架/接口表/回声·激活·存活仲裁经验 | 保留移植进新 adapter |
| xkb / wayland_keyboard（KeyRepeater） | 保留；repeater 抽到 ui/input |
| x11_xim | 基线保留，M4 决策升级 |
| textinput.Editor | **保名换内脏**（comp overlay + ComposedView + epoch）；kit 未建成=无下游破坏面，正是重构窗口期 |
| input_router 会话管理 | ImeSession 状态机化重写；TextEditTarget/FocusNode.Target 机制原样保留 |
| platform.ContentPurpose | 并入 ContentType，值域不变（稳定测试保留） |
| ui_textinput_ime | 改造为验收真窗（自测模式/快照取证/键日志保留；新增组合段渲染+多字段切换面板+BaseEditable 样板） |
| ui_ime_probe | 改造为 ImeEventHandler 仿真探针 |

## 6.1 控件接入成本（易用性承诺）

kit 提供 `BaseEditable` 内嵌类型包办四件套（Editor/IMERect 含 preedit 宽度测量/ContentPurpose/DrawPreedit 默认实现——签名传**已解析的像素几何+属性**而非逻辑 range）。自定义控件通常只需覆写 Editor 与 purpose 两处；从零接入 ≈ 15 行。

---

## 7. 测试与验收体系

### 7.1 单测（headless）

状态机全转移矩阵（每转移断言出站副作用与 epoch 不回退）；Editor overlay 边界（Insert/Delete/Snapshot/View/ByteOffsetAt：CJK 吸附/空串/组合段点击钳制）；**回声抑制不变量锁**（任意入站序列下「出站推送数 ≤ 本地变更数」且 epoch 单调）；done 队列乱序/double-done/serial 回绕；KeyRepeater（确定性时钟注入）；ComposedView 双向映射穷举。

### 7.2 真窗验收矩阵（每平台填实后才 ✅）

| # | 用例 | 通过判据 |
|---|---|---|
| P1 | 英文直打 10 键 | 每键恰一次插入；无双插 |
| P2 | 拼音组合→候选→上屏 | preedit 实时显示于光标处；原子替换 |
| P3 | 组合中退格 | 缩拼音不删已提交文本 |
| P4 | Esc 取消组合 | preedit 消失、缓冲不变 |
| P5 | 点击定位 | 光标落点=最近 rune 边界（含 CJK） |
| P6 | 方向键 | 光标可见移动、锚点跟随 |
| P7 | 密码框 purpose | 引擎禁候选（日志断言 SetContentType 发出） |
| P8 | 切输入法引擎 | 窗口显示后直接可切（无需失焦循环） |
| P9 | 单键事件数上限 | 一次按键入站 preedit ≤ 2（风暴回归锁） |
| P10 | 多字段切换 | 会话开合成对、无残留组合 |
| P11 | 死键/compose 序列（拉丁扩展键盘） | 重音字符正确产出 |
| P12 | ibus 与 fcitx5 双引擎对照 | 两引擎均过 P1–P9 |
| P13 | 引擎守护进程中途重启 | 会话自动恢复或干净降级 |

### 7.3 平台差异矩阵（实测后回填，禁止沿用笼统说法）

各平台「组合期间原始按键是否到达应用」：mac=到达（双门）；Wayland/X11=取决于合成器/引擎，M1 实测后写死进规范。

### 7.4 证据规范

沿 U21 三证据制：逻辑探针 JSON + 像素断言 + 协议日志（GPUI_IME_DEBUG）。

---

## 8. 里程碑（已定稿 · 2026-08-25 用户确认：Linux 全链先行，win/mac 后跟进）

> **环境约束（用户声明）**：当前只有 Linux 可实测；Windows/macOS 为「对齐标准盲写」——编译级验证，无真机验收。由此新增两条纪律：
> - **R-BLIND-1**：win/mac 交付时各附《盲写风险登记表》——逐条列出未验证假设（interpretKeyEvents 重入时机 / IMM32 消息顺序 / COM vtable 调用约定等），将来任一真机到手按表逐项核销；
> - **R-BLIND-2**:P1–P13 验收矩阵对 win/mac 标注为「欠账清单」，状态列区分「编译验证级」（本轮）与「验收级」（真机清账后），禁止虚标。
> - **顺序约束（用户确认）**：M0+M1 做完做扎实后才开 M2/M3——Linux 真窗先证明状态机与门面的行为契约，win/mac 只做同一契约的协议方言翻译。

| M | 内容 | 出口判据 |
|---|---|---|
| M0 ✅ 2026-08-25 | 本文定稿 + Editor/ComposedView/ImeSession 状态机落地（纯 Go 单测）+ RENDER_API_CATALOG 同步与 apidoc 门禁绿 | 状态机/Editor/View 单测全绿；接口冻结 |
| M1 ✅ 2026-08-25 | Wayland 迁移新架构：① D7 done 队列（`wayland_ti_queue_linux.go`：pending 动作队列 + flushCommit 原子提交 + postFlush 投递通道，poll 排水）；② C2 定时器改造（兜底轮 AfterFunc 只入队+post，marshal 全部归 dispatch 线程）；③ 全部发送路径改走队列（Enable/Disable/CaretMoved/SetPurpose/SetSurrounding 各自一次原子 commit）；④ ImeSession 门面接入（adapter 因 import 环移 embedder 层——§4.3 v1.3 分层修正）；⑤ 验收窗双链路自测（路由器腿 "你好ba" + 门面腿 "我"）。**per-seat 决策修正**：核实协议 XML 后确认 enable(1)[surface]/disable(2)[surface] 自带 surface 参数，「每窗一 ti」即协议合法形态；SDL set_input_surface 是 SDL3 私有包装非 zwp 标准——维持现状并记录，撤销 §5.1 的 per-seat 改造项 | P1–P13 Wayland 列：单测级全过；真机交互用例（P2/P3/P8）待用户复测；apidoc 门禁绿 |
| M2 | Windows IMM32 通道 + 消息链集成 | Windows 列过 |
| M2.1 | TSF 增强（surrounding 拉取/InputScope/属性范围） | 可选增强，超预算不阻塞 |
| M3 | macOS objc 子类 spike（一周盒装）→ NSTextInputClient 全套 | macOS 列过 |
| M4 | 收口：X11 PreeditPosition 决策、X11 重复迁移、三端矩阵归档、§10 账本迁移 | 三端全 ✅ |

风险：M3 为最大不确定点（约一成概率走 NSTextView 注入降级）；其余按存量移植路径可控。

---

## 9. 开放问题（审定时表态）

1. TSF full 是否本轮（建议：IMM32 先行，TSF=M2.1 增强）；
2. X11 PreeditPosition 升级做否（建议：M4 视余量）;
3. X11 服务端重复是否统一迁 KeyRepeater（建议：迁——行为一致+可测）；
4. Undo 本轮只埋点（分组规则已定：一组合一步）——确认；
5. 图素簇粒度本轮 rune 替代（已在非目标声明）——确认。

**已定案（2026-08-25 业界对照审查后并入正文）**：
- ~~多窗口 IME 实例粒度~~ → **已定**：业界主流为「每顶层窗口一个 adapter」（Chromium per-WindowTreeHost / Qt per-window / mac per-view / Windows HIMC per-thread），全应用单例无先例、按输入框建实例也无先例；唯一例外 Wayland 按协议本意为 **per-seat 共享 + 动态 input_surface**，已写入 §5.1 作为 M1 改造项。上层 ImeSession 门面维持每窗口一个。 |

## 10. 修订

| 版本 | 说明 |
|---|---|
| v1.2 定稿 | 2026-08-25 用户审定通过 + 环境约束落档：Linux 全链先行（M0+M1 做扎实后开 M2/M3）；R-BLIND-1 盲写风险登记表、R-BLIND-2 验收欠账清单两纪律入 §8。本文转 **IME 线真源**，接口自 M0 起冻结。 |
| v1.3 收敛检查 | 2026-08-26 按规划收敛核查：M0/M1 交付物与示例使用核对通过（ImeSession 门面、ComposedView 唯一换算出口、D2 surrounding 关断默认、C1–C4 并发契约未被新代码违反）；光标几何补齐标准实现（RenderText.CaretColumn 对标 getOffsetForCaret；Editor.MoveCaretVertically 粘滞列对标 desired-X）并经 6 确定性场景像素断言；示例 caretAnchor 单源化使可见条与 IMERect 锚点共用几何（§4.2 CaretMoved 触发时机兑现）。调试残留清理（dbg*_test.go ×9、编译产物 ×2）。P2/P3/P8 真机复测与 I3 X11 双场景仍为待办欠账。 |

| v1.1 | 红队审查修订：**P0-1** 新增 §4.0 并发契约（C1–C4；现状 AfterFunc 直发列入重构）；**P0-2** §5.4 mac 路线改正为自建子类+重入护栏+成功率重估；**P1-1** 目标句改写为 ImeSession 门面形态+所有权三条；**P1-2** 新增 §4.5 ComposedView 唯一换算出口+CaretMoved 触发时机补 preedit 变化+拖选钳制语义；**P1-3** 适配器补 SetSurroundingProvider；**P1-4** 词汇表归一到 input.IMEEvent（+Segments/+IMESession）；**P1-5** editEpoch 全局凭据；**P1-6** 图素簇措辞+rune 限制声明；**P2** 包表述替代六层、BaseEditable 易用性承诺、测试矩阵补 P11–P13+平台差异矩阵、API 目录同步义务入里程碑、RTL/Undo 分组声明。 |
