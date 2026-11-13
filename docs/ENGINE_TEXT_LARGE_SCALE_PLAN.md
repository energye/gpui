# ENGINE_TEXT_LARGE_SCALE_PLAN — 超大规模文本架构立项书

**状态**: 立项审查中（2026-11-13）——方向与档位待用户审定；成熟方案已调研（§1.4），**D1–D5 已于 2026-11-13 确认生效**；审查记录见 §9，结论为**有条件通过**。**V0 已完成**（报告 `docs/ENGINE_TEXT_LARGE_SCALE_V0.md`）：1e6 唯一行全量约 5s/3GB、1e6 字单行真实击键 1.80s、5e6 字 10.41s、M4 hitch 5.007 贴门未过。**V1 已收口（a/b-1/b-2/b-3）**：行定位 118.6×→1.05×、UTF-16 偏移 102.2×→1.44×，Editor AddText/Backspace/行导航/IME 改前 97/101/173/141× → 改后 0.06–0.17×；端到端 `GetText`/UI 同步仍 O(n)，归 V2。**V2a 已完成**：按行视图 `TextViewport`（1e6/1e4 首屏 0.80×、缓存 O(可见行)、几何与全量布局逐项一致）；**V2b-1 已完成**：生产 Editor buffer 接成 `LineSource`，真实 1e6 行 Editor 首屏 120µs（1e4 459µs，0.26×），视图支持失效；**V2b-2a 已完成**：`RenderText` 增加虚拟绘制模式（只物化/绘制视口行，无窗口不画），`Paint` 逐行绘制抽成 `paintTextLine`；**V2b-2b-1 已完成**：多行框（不回绕、折叠选区、非 IME/密码、≥64KB）共享 view 走虚拟 sync/光标/绘制/命中/拖拽/上下键，1e6 行首同步 125µs 且不物化全串；非折叠选区/IME 暂时回退全量（V2b-2b-2 收口）。
**关联**: `docs/ENGINE_TEXT_SCALE_PLAN.md`（M0–M5 主体已收口，本文件是其规模延伸）、`docs/ENGINE_TEXT_SCALE_CHANGELOG.md`、`AGENTS.md`。
**定位**: 架构级立项，不是 M5 遗留补丁；目标是把文本编辑的规模上界从「1e6 字/1e6 行可用」推进到「几百万行、单行几百万字仍不退化」。

---

## 0 一句话目标

在 **300 万行、单行 500 万字**的文档上，首屏、滚动、击键、内存仍与文档规模解耦；小文档行为保持不变。

---

## 1 立项依据（现状证据）

### 1.1 布局路径仍是整篇构建

- 全量布局实测：2k 行 38.3ms（估算）/ 232.9ms（真字体），20k 行 349.7ms（10× 行数 → 9.1× 耗时，O(n) 确认）；1e6 行**未直接跑**，M4 README 按线性外推约 17s（估算路径），只能作排序依据，不能当验收数（`examples/ui_text_m4_virtual/README.md`）。
- 单行整形：5 万字真字体击键 **17.29ms**（2026-11-13 实测，`ui/rendering/m5_singleline_test.go:58`）；1e6/5e6 字未测。
- 行表是扁平数组：`layoutLive.rows []TextLayoutLine` + `lineIndex.starts/ends/tops`，内存 O(行数)；行数变化编辑 O(n)（1e6 行约 1ms，绝对门禁 2ms，`ui/rendering/layout_update.go:381`）。

### 1.2 文本真源仍是整串字符串

- `Editor.text string` 是主存储，所有编辑走整串 splice（如 `ui/textinput/editor.go:903`）。
- `ui/textbuffer` 已有 rope（隐式键 treap，4KB 不可变块；`LineCount` O(1)，`LineAt` 当前按块遍历 O(#chunks)，treap 的 `nl` 计数尚未用于行定位，`ui/textbuffer/buffer.go`），但当前只是**镜像/快照**，不是编辑主存储（`ui/textinput/editor_buffer.go`）。
- 布局同步每次击键 `disp := ed.GetText()` 物化整串（`ui/textinput/edit_scroll.go:196`），再整串传给 `SetTextSpan`。即使文本层换成 rope，这一句仍会保持 O(n)。

### 1.3 行虚拟化原语已存在，但只用到「绘制裁剪」，没用到「布局/度量」

- `VirtualTextLines`（定高 O(1)、变高 Fenwick + 懒测量，`ui/rendering/virtual_text_lines.go:22`）已有 1e6 行真窗：首屏 7ms、窗口 42 行、变高模式堆 +17MB（M4 README）。
- 输入框的**绘制/重录**裁剪已接：单行 `SetViewportHint`（`ui/textinput/edit_scroll.go:118/185`），多行 `SetViewportRect`（`edit_scroll.go:112/179`；`ui/rendering/text.go:618`）。
- 但**布局/度量仍是整篇**：`RenderText.ensureLayout()` 全量构建行表；多行框算最大宽逐行扫全表（`ui/textinput/box.go:857`）；`VirtualTextLines` 没有接进输入框，总高/行定位仍来自全量 `TextLayout`。
- M4 1e6 行真窗 `hitch_rate_per_min` 8.0 / 9.0 / 12.5，门禁 ≤5，**既有红未收口**（M4 README 表）。

### 1.4 成熟方案调研（2026-11-13，按源码/官方文档核对）

| 系统 | 文本层 | 行高/布局 | 超长行 | 对本项目的含义 |
|---|---|---|---|---|
| VS Code | piece tree：64KB 块，节点缓存子树字符数 + 换行数，行/偏移查找 O(log N)；内存≈文件大小 | 视口渲染；旧「每行一个对象」的 line array 在 35MB/13.7M 行文件上约 600MB（20×） | `editor.stopRenderingLineAfter` 默认 **10000** 字符，`editor.maxTokenizationLineLength` 默认 **20000** | 文本必须用「块 + 子树计数」的树，禁止每行对象；超长行要有渲染上限 |
| CodeMirror 6 | rope（Text 树） | `HeightMap` 树：`HeightMapGap` 覆盖纯文本区间按行/字符估算，`HeightMapText` 只给有 widget/折叠的行，`HeightMapBranch` 汇总；实测值经 `MeasuredHeights` 稀疏更新；编辑只重建变化区间 | 视口测量；回绕是可选能力 | 变高高度模型应是「区间估算 + 稀疏实测节点」，不是每行一个 float/bool |
| Flutter | `TextPainter` 是整段 `ui.Paragraph`，每次文本变化整段重建 | `SliverChildDelegate.estimateMaxScrollOffset` 用已构建子项估算未构建子项，滚动时修正 | 无超长行专用机制 | 百万行/百万字单行不能照 Flutter；参考其「估算 + 修正」，不参考整段 Paragraph |
| 本仓库现状 | `Editor.text string` 是真源；`textbuffer` treap 只是镜像；`GetText()` 每击键物化整串 | `TextLayout` 全表；`VirtualTextLines` 每行 17B；多行布局全篇 | 无上限（50k 已 17.29ms） | 与成熟方案的四点偏差见 §3 |

结论：要「规模不变」，成熟方案的四条硬做法是 **树形文本真源、区间高度图、视口物化、超长行上限**；本立项按这四条调整（§3/§7）。

---

## 2 目标与门禁（草案，待审查定稿）

### 2.1 档位

| 档 | 行数 | 单行字数 | 用途 |
|---|---|---|---|
| L1 | 1e6 | 1e6 | 现状上界 |
| L2 | 3e6 | 3e6 | 用户「几百万」主档 |
| L3 | 3e6 | 5e6 | 极值档 |
| S | 1e7 | 定高 | 定高压力档（不做变高） |

### 2.2 硬门禁（每档全过才算完成）

- 首屏 ≤100ms（定高/估算口径，外部计时）。
- 滚动 `interval_p95_ms ≤22`、`fps_interval ≥55`、`hitch_rate_per_min ≤5`。
- 字符击键 p95 ≤16ms（含 5e6 字单行）。
- caret 移动/命中 p95 ≤4ms。
- 行数变化编辑 p95 ≤16ms（3e6 行）。
- 内存：RSS ≤ 文本字节 ×1.5 + 64MB（不含字体/图集）；变高行高度表 ≤1KB/千行。
- **击键路径不得物化整串**：编辑与布局按块/增量读写，`GetText()` 全串拷贝不得出现在击键路径（硬约束）。
- **测量口径**：所有布局/击键门禁必须用「唯一行」与「每击键改不同位置」；重复行、交替同位置编辑视为无效测量（V0 已证实会假快 800×）。
- **多行同步**：全表最大宽扫描（`box.go:857` 同款）≤4ms @1e6 行；超过即视为 O(行数) 回归。

### 2.3 非目标

- 百万字单行的**全量精确回绕**：只做可见窗口断行 + 远端估算。
- 全选/复制/查找全部时走视觉行全量布局：走文本层，不走视觉行。
- 跨块 bidi/复杂脚本的完整重排：保留 line-level 回退。
- 小文档行为变更（<64KB 沿用现路径）。

---

## 3 架构（四层 + 一条硬约束）

1. **文本层（唯一真源，所有尺寸）**：`ui/textbuffer` 的 rope 升级为主存储；4KB 不可变块 + 子树 `len/nl` 计数；编辑 O(log n)，行/偏移查找 O(log n)（当前 `LineAt` 仍是块遍历，V1 补下降）；小文档可用单叶字符串，但仍是同一个 Buffer 真源，`Editor.text` 不再做第二真源。
2. **行/高度层（区间高度图）**：定高 O(1)；变高照 CodeMirror `HeightMap`：纯文本区间用 `HeightMapGap` 按行/字符估算，只有 widget/折叠/已测行存 `HeightMapText`，分支汇总；禁止每行 `float64+bool` 数组（当前 17B/行 → 3e6 51MB、1e7 170MB）。
3. **视口层**：只物化可见行 + overscan；对外提供 `viewport`/`visibleRows` 式视图（对齐 CodeMirror `EditorView.viewport`）；`LineCount/Line(i)` 大文档走虚拟视图，小文档可物化全表作为兼容视图；命中/选区走高度图。
4. **超长行层（按 VS Code 口径）**：单行设渲染上限（默认 10000 字符，可配）与分词/整形上限（默认 20000）；上限内按字素分块（2–4KB）整形 + 前缀宽度，只整形可见水平窗口；超过上限按 VS Code 的做法截断显示并给出标记，**不承诺百万字全量回绕**。
5. **硬约束**：击键路径从 buffer 取增量与可见块，不再 `GetText()` 整串、不再把整串传给布局；`GetText()` 仅用于导出/全选等明确 O(n) 场景。

复用：`VirtualTextLines`（高度图）、`SetViewportRect`（纵向裁剪）、现有段缓存与 `rowShift`（视口内偏移）。

---

## 4 分期（每期独立可回滚，九项规范沿用文本总纲 §9.2）

| 期 | 目标 | 关键交付 | 验收/熔断 |
|---|---|---|---|
| V0 | 基线量化（只读） | 1e6/3e6 行全量首屏；50k/1e6/5e6 字单行击键；多行框滚动；M4 hitch 复测 | **已完成（2026-11-13，见 `ENGINE_TEXT_LARGE_SCALE_V0.md`）**；结论：现有门禁全部未达，且现有基准会因缓存假快 |
| V1 | rope 主存储 | **已完成（2026-11-13，V1a/b-1/b-2/b-3）**：`LineStart/LineIndex/LineAt` O(log n)（118.6×→1.05×）；节点 `u16/nr` + `UTF16Len`/`ByteOffsetForUTF16`/`UTF16ToRuneIndex`/`RuneIndexToUTF16`/`UTF16OffsetAt`/`StepUTF16`；Buffer 为真源、`e.text` 惰性缓存；AddText/Backspace/DeleteSelected/Undo/Redo/IME/选区/行导航/词导航全部走 buffer。Editor 四条路径改前 97/101/173/141× → 改后 0.06–0.17×。**遗留到 V2**：`GetText()`/UI 同步与 `Delta()` 导出仍整串物化 | 编辑 O(log n) 基准；13 裁判 + textinput 全绿；崩溃即回滚 |
| V2 | 视口行模型 | **V2a 已完成**：`ui/rendering/text_viewport.go` 新增 `TextViewport`（`LineSource` + 懒物化 + 有界缓存 + `VirtualTextLines` 高度）；首屏 1e6/1e4 = 0.80×，几何与全量布局逐项一致，缓存 O(可见行)；V2a 覆盖不回绕模型。**V2b-1 已完成**：`editorLineSource` 把生产 buffer 接成 `LineSource`，真实 1e6 行 Editor 首屏 120µs（1e4 459µs，0.26×），`Invalidate/InvalidateLine` 可用。**V2b-2a 已完成**：`RenderText` 虚拟绘制（只绘视口行，无窗口不画），逐行绘制抽成 `paintTextLine`。**V2b-2b-1 已完成**：多行框虚拟 sync/光标/命中/拖拽/上下键，1e6 行首同步 **125µs**，`b.txt.Text` 保持空（不 GetText）；非折叠选区/IME 暂时回退全量。**V2b-2b-2 待做**：虚拟选区高亮/IME 候选框/彻底去掉回退降级 | 1e6 行首屏/滚动/hitch 达标；hitch 红未消不得标绿 |
| V3 | 变高/回绕估算 | 区间/间隙高度图（gap 估算 + 稀疏实测）；编辑局部失效；滚动修正 | 3e6 行变高内存 ≤1KB/千行；锚点不跳变 |
| V4 | 超长行分块整形 | 块级 shape cache + 前缀宽度；横向视口 | 5e6 字单行击键 ≤16ms；跨块 bidi 回退可用 |
| V5 | 收敛 | 小文档阈值降级定稿；大文档移除扁平 rows；全回归 | 全档门禁 + 真窗 + 内存 + Golden |

---

## 5 验收与证据

- 数据：`testdata` 种子 + 循环索引（沿用 `ui_text_m4_virtual` 的 40 条日志复用），禁止测试内循环造标准数据。
- 基准：`BenchmarkOpenDocument/Scroll/Keystroke/Caret`，门禁用绝对上限（大文档 O(n) 路径不适用 1.5 比值）。
- 真窗：扩展 `ui_text_m4_virtual` 或新建 `ui_text_large_scale`，逻辑探针 + 像素断言 + Golden 三证据。
- 内存：外部 probe（RSS/gctrace/smaps），判据按 O3 的 120s 口径。
- 回归：13 裁判 + `ui/rendering`/`ui/textinput`/`ui/textbuffer` 整包 + 真窗。

---

## 6 风险

| 风险 | 说明 | 对策 |
|---|---|---|
| 双轨期行为漂移 | 小文档旧路径 vs 大文档虚拟 | 同文档查询逐项等价测试；阈值降级 |
| 变高内存 | 当前 17B/行 | 区间高度图 + 稀疏实测；1e7 只定高 |
| M4 hitch 既有红 | 8–12.5/min >5 | V2 先复测基线，红不继承；必要时单独立项 |
| 跨块 bidi/字素 | 断行/整形错误 | 块边界带上下文；复杂脚本 line-level 回退 |
| API 语义 | `LineCount/Line(i)` 全表假设 | 新视图接口 + 小文档兼容；13 裁判锁行为 |
| 范围过大 | rope+视口+分块整形 | 分期可回滚；V1 不接 UI 先验证 |

---

## 7 决策（2026-11-13 用户确认，D1–D5 生效）

- **D1 超长行**：按 VS Code 口径，单行默认渲染上限 **10000 字符**、分词/整形上限 **20000 字符**；超过上限截断显示并给标记，**不承诺百万字全量回绕**；回绕只对上限内/可见窗口生效；上限可配置（-1 表示不限制，但明确卡顿风险）。依据：`editor.stopRenderingLineAfter=10000`、`editor.maxTokenizationLineLength=20000`。
- **D2 文本真源**：Buffer/rope 是所有尺寸的唯一真源；`Editor.text` 降级为可选的物化缓存，不再做第二真源；小文档可以用单叶字符串，但仍在同一 Buffer API 内。布局统一走虚拟模型；`TextLayout` 全表只作为小文档/兼容视图，不作为第二套真源。依据：VS Code piece tree、CodeMirror rope 都只有一套文本结构。
- **D3 变高**：采用 CodeMirror 式**区间高度图**（纯文本 gap 估算 + 稀疏实测节点 + 分支汇总），不采用每行数组；验收档 3e6 变高，1e7 定高（这是测试成本/风险取舍，不是架构上限；1e7 变高沿用同一 gap 模型）。
- **D4 视图 API**：新增 `viewport`/`visibleRows` 式公开视图（对齐 CodeMirror `EditorView.viewport`），保留现有 `TextLayout` 全表作为小文档/兼容视图；13 裁判锁行为。
- **D5 内存预算**：RSS ≤ 文本×1.5 + 64MB（不含字体/图集）；禁止每行对象/数组；高度图只存 gap 节点 + 实测节点，实测节点数可配上限（默认 ≤1e5）。依据：VS Code piece tree 内存≈文件大小；旧 line array 35MB/13.7M 行约 600MB（20×）是反例。

---

## 8 修订记录

| 日期 | 说明 |
|---|---|
| 2026-11-13 | 立项草案：四层架构 + V0–V5 + 门禁/Q1–Q5 + 审查 §9。 |
| 2026-11-13 | 成熟方案调研（VS Code piece tree/长行上限、CodeMirror HeightMap、Flutter Sliver 估算）；Q1–Q5 改为 D1–D5；补 §1.4/§10，审查条件 7→9。 |
| 2026-11-13 | V0 完成：唯一行 3e5 1.57s/898MB、1e6 字单行 1.80s、5e6 10.41s、M4 hitch 5.007；门禁补「唯一行/唯一编辑」与最大宽扫描；报告 `ENGINE_TEXT_LARGE_SCALE_V0.md`。 |
| 2026-11-13 | D1–D5 用户确认；V1a 完成：`textbuffer` 行定位 O(log n)（先红 118.6×→绿 1.05×，1e6 LineAt≈3.0µs），`BenchmarkInsertAtOffset` 1e4/1e5/1e6=2.5/4.1/6.7µs，textbuffer/textinput 整包与 13 裁判全绿；V1b Editor 主存储化待做。 |
| 2026-11-13 | V1b-1 完成：treap 节点加 UTF-16 计数，`UTF16Len` O(1)、`ByteOffsetForUTF16` 按子树下降（先红 102.2×→绿 1.44×，1e6≈7.7µs）；保持 Editor 现有「代理对中间偏移→串尾」语义；textbuffer/textinput/13 裁判全绿；V1b-2（Editor 主存储化）待做。 |
| 2026-11-13 | V1b-2 完成：Buffer 为唯一真源、`e.text` 惰性缓存；AddText/Delete/DeleteSelected/Undo/Redo/SetComposing/EndComposing/SetText/ApplyDelta 走 buffer；64+ 处 `e.text` 读改 `Text()`；`TextRange` 用 `Buffer.UTF16Len` O(1)。先红 HEAD 工作树 117.5× → 改后 0.06×，1e6 追加 1.7µs；textinput 全包 + R1 + TextLayout 裁判绿。M3.5 收缩用例改为「buffer 保留、树降级 string」（D2 契约）。V1b-3 待做。 |
| 2026-11-13 | V1b-3 完成（V1 收口）：buffer 加 rune 计数与 `UTF16ToRuneIndex/RuneIndexToUTF16/UTF16OffsetAt/StepUTF16/RuneAtUTF16`；DeleteSurrounding、IME 合成、SetSelection、行/词导航全部迁移，`ApplyIME/ApplyText` 改 docVersion 比对。HEAD 红灯 AddText 97.27×/Backspace 101.39×/行导航 173.42×/IME 140.80× → 改后 0.06–0.17×；textbuffer/textinput 整包与 13 裁判绿（一次整包跑偶发失败，随后两轮 + 新门禁 5 连跑均绿）。遗留 `GetText`/`Delta()` 物化归 V2。 |
| 2026-11-13 | V2a 完成：新增 `TextViewport` 按行视图（LineSource + 懒物化 + 有界缓存 + VirtualTextLines 高度）；1e6/1e4 首屏 0.80×（≤3 门禁），几何与全量布局逐项一致，缓存 O(可见行)；`ui/rendering` 整包绿。V2b（多行框/RenderText 接线、布局消费 buffer、GetText 退出击键路径）待做。 |
| 2026-11-13 | V2b-1 完成：`editorLineSource` 将生产 Editor buffer 接成 `rendering.LineSource`；真实 1e6 行 Editor 首屏 120µs（1e4 459µs，0.26×）；`TextViewport.Invalidate/InvalidateLine` 支持编辑后失效；textinput/rendering 整包绿。V2b-2（虚拟绘制 + 多行框度量 + GetText 退出击键路径）待做。 |
| 2026-11-13 | V2b-2a 完成：`RenderText` 虚拟绘制模式（`SetVirtualSource/ClearVirtualSource/Virtual` + `paintVirtual`），只物化/绘制 `SetViewportRect` 窗口内的行，无窗口不画；逐行绘制抽成 `paintTextLine` 供全量/虚拟共用。测试：1e6 行源绘制在 50 万行窗口只物化 ≤200 行、无窗口 0 行；`ui/rendering` 整包绿。V2b-2b（多行框度量/选区/命中接线、GetText 退出击键路径）待做。 |
| 2026-11-13 | V2b-2b-1 完成：多行框（不回绕/折叠选区/非 IME/密码/≥64KB）共享 `TextViewport`，虚拟 sync 度量、光标锚点、绘制、鼠标命中、拖拽 maxX/totalH、上下键；1e6 行首同步 125µs，`b.txt.Text` 保持空；非折叠选区/IME 回退全量。textinput/rendering 整包绿。V2b-2b-2（虚拟选区/IME、去回退）待做。 |

---

## 9 立项审查（2026-11-13，只读，不改代码）

### 9.1 结论：**有条件通过**

方向成立（四层架构 + 长行分块 + rope 主存储），但以下 9 条必须先落，否则需求不可执行。

### 9.2 逐项判定

| 需求 | 依据 | 判定 |
|---|---|---|
| L2/L3 档位（3e6 行 / 5e6 字单行） | 现状无任何实测；1e6 行全量只是外推 | **需补 V0 实测后定稿**（条件 1） |
| 变高 3e6/1e7 行内存 | `VirtualTextLines` 变高 17B/行 → 3e6 51MB、1e7 170MB，远超「≤1KB/千行」 | **必须改区间/间隙高度图**（条件 2） |
| M4 hitch ≤5 | 1e6 行真窗实测 8–12.5，既有红 | **V2 先复测；红未消不得标绿**（条件 3） |
| 多行框布局虚拟化 | 绘制裁剪已接（`edit_scroll.go:112/179`）；但布局/度量仍全篇（`ensureLayout`、`box.go:857`） | 需求改为「布局按视口物化」，可执行 |
| rope 行定位 O(log n) | `LineAt` 当前 O(#chunks)，`nl` 计数未用于下降 | **V1 需补 O(log n) 行定位**（条件 7） |
| 击键路径不物化整串 | 当前 `GetText()` + 整串 `SetTextSpan`；rope 也只是镜像 | **必须升级为硬约束并列入 V1/V2 验收**（条件 4） |
| `TextView`/虚拟 `Line(i)` | 现有 `LineCount/Line` 按整表语义（`text_layout.go:292/300`） | D4 采用：新增 viewport 视图；保留 `TextLayout` 兼容（条件 5） |
| 文本单一真源 | `Editor.text` 仍是编辑真源，Buffer 只是镜像 | D2 采用；V1 验收（条件 9） |
| 超长行上限 | VS Code `stopRenderingLineAfter=10000`、`maxTokenizationLineLength=20000` | D1 采用；需定义截断 UI 行为（条件 8） |
| 规模测试数据 | 仓库规则禁止测试内造标准数据 | **沿用 testdata 种子 + 循环索引**（条件 6，已写入 §5） |
| 百万字单行回绕 | 全量断行 O(L)；VS Code 超长行上限 10000 | D1 已定：不承诺全量回绕，只做上限内/可见窗口 |
| rope 改造工作量 | `ui/textbuffer` 已存在，但主存储化会影响所有编辑 API | V1 前补工作量评估；分期可回滚 |

### 9.3 审查发现并已修订

- 把「1e6 行全量 17s」明确标为外推，不作为验收数。
- 把变高内存从隐含改为显式预算 + 区间高度图。
- 把 M4 hitch 红写入风险，不作为已完成能力。
- 门禁改为绝对上限（大文档 O(n) 路径不适用 1.5 比值）。
- 新增「击键路径不得物化整串」硬约束（原草案漏了 `GetText()` 这一条）。
- 明确小文档阈值降级（D2）。
- **审查中修正两处事实错误**：① 初稿称「多行纵向视口未接」，实测 `edit_scroll.go:112/179` 已接 `SetViewportRect`，真正缺的是**布局/度量虚拟化**；② `textbuffer.LineAt` 不是 O(log n)，当前按块遍历，V1 需补行定位。
- **Q1–Q5 已按成熟方案改为 D1–D5**（§7）：超长行上限、单一文本真源、区间高度图、viewport 视图、内存预算；其中 D1 的截断 UI 行为和 D2 的真源切换列为条件 8/9。

### 9.4 审查未覆盖 / 未验证

- 3e6 全量未跑；5e6 单行只测 1 次唯一编辑；1e6 唯一行全量未跑（外推 5s/3GB）。V1b-3 已落地（Editor 编辑/选区/IME/导航原语 O(log n)）；V2a/V2b-1/V2b-2a/V2b-2b-1 已落地（视图、生产 buffer 适配、RenderText 虚拟绘制、多行框虚拟 sync/光标/命中）；遗留：非折叠选区/IME 仍回退全量（V2b-2b-2），`Delta()` 导出仍整串物化，maxW 暂按可见行近似。
- hitch 根因未定位（M4 只有现象数据）。
- 跨块 bidi/字素边界的失败模式未做探针。
- `TextView` 虚拟视图与 rope 行定位（O(log n)）未做原型验证。
- 超长行截断的 UI 行为（省略标记、横向滚动、复制范围）未定义；单一真源切换未做原型。

### 9.5 建议

V0 已完成（见 `docs/ENGINE_TEXT_LARGE_SCALE_V0.md`）；下一步确认 D1–D5 后开 V1（rope 主存储 + O(log n) 行定位），V1 不接 UI 先验证。

---

## 10 参考（2026-11-13 调研）

- VS Code 文本缓冲（piece tree、64KB 块、子树长度/换行计数、内存≈文件大小）：https://code.visualstudio.com/blogs/2018/03/23/text-buffer-reimplementation
- VS Code 超长行：`stopRenderingLineAfter` 默认 10000、`maxTokenizationLineLength` 默认 20000（`src/vs/editor/common/config/editorOptions.ts`；Monaco typedoc 同项）。
- CodeMirror 6 高度图（gap 估算 + 稀疏实测 + 分支汇总）：https://raw.githubusercontent.com/codemirror/view/master/src/heightmap.ts
- CodeMirror 6 屏外高度估算说明：https://discuss.codemirror.net/t/dynamic-line-height/3257
- Flutter 未构建子项估算：`packages/flutter/lib/src/widgets/scroll_delegate.dart` 的 `estimateMaxScrollOffset`；`TextPainter` 整段 Paragraph（本机 `flutter/packages/flutter/lib/src/painting/text_painter.dart:781-789、1218-1271`）。
