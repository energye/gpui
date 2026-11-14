# ENGINE_TEXT_LARGE_SCALE_PLAN — 超大规模文本架构立项书

**状态**: 已确认（2026-11-13）——D1–D5 生效，V0–V2 已收口，V3c 自动验收 PASS + 用户人工确认画面 OK（事件通道未实测，见 §8 诚实项）；成熟方案见 §1.4，审查记录见 §9（有条件通过）。**V0 已完成**（报告 `docs/ENGINE_TEXT_LARGE_SCALE_V0.md`）：1e6 唯一行全量约 5s/3GB、1e6 字单行真实击键 1.80s、5e6 字 10.41s、M4 hitch 5.007 贴门未过。**V1 已收口（a/b-1/b-2/b-3）**：行定位 118.6×→1.05×、UTF-16 偏移 102.2×→1.44×，Editor AddText/Backspace/行导航/IME 改前 97/101/173/141× → 改后 0.06–0.17×；端到端 `GetText`/UI 同步仍 O(n)，归 V2。**V2a 已完成**：按行视图 `TextViewport`（1e6/1e4 首屏 0.80×、缓存 O(可见行)、几何与全量布局逐项一致）；**V2b-1 已完成**：生产 Editor buffer 接成 `LineSource`，真实 1e6 行 Editor 首屏 120µs（1e4 459µs，0.26×），视图支持失效；**V2b-2a 已完成**：`RenderText` 增加虚拟绘制模式（只物化/绘制视口行，无窗口不画），`Paint` 逐行绘制抽成 `paintTextLine`；**V2b-2b-1 已完成**：多行框（不回绕、非密码、≥64KB）共享 view 走虚拟 sync/光标/绘制/命中/拖拽/上下键；**V2b-2b-2 已完成**：虚拟选区高亮、IME 候选框，非折叠选区/IME 不再回退全量，1e6 行首同步 125–223µs 且 `b.txt.Text` 保持空。**V2 已收口（2026-11-13 真窗）**：`ui_text_large_scale` 1e6 行 120s 在隔离 :3 跑通，fps 58.54、p95 18.92ms、hitch 1.0/min、RSS 稳态 21542KB/min（valid/pass）；虚拟模式 true，滚动 24.2 万行无崩溃。 **V3a 已完成**：新增稀疏区间高度图 `HeightMap`（估算 + 实测覆盖 treap）；3e6 行/32768 实测节点堆 **2.37MB**（旧 `VirtualTextLines` 51.01MB），查询 3M/1e4 墙钟比 2.12。**V3b 已完成**：`HeightMap` 接入 `TextViewport`（行数随源同步、编辑失效重置实测高度、`Prepare` 量测后重定窗口并按实测覆盖视口带）；虚拟绘制/录制按回绕视觉行逐行画（新增 `NewWrappedTextViewport`、`ViewportLine.Rows`、`MeasuredCount`、`LineRowText`、`HeightMap.Reset`）；**V3c 自动验收 PASS + 用户人工确认画面 OK（2026-11-13）**：多行框放开回绕（`canVirtual` 不再拦 `wrap`）、按宽度重建共享视图、行数变化编辑按锚点重定 `scrollY`、虚拟上下键与 Shift 扩选按视觉行移动且不物化整串；新增 `TextViewport.AnchorAt` / `InvalidateLayouts` / `RebaseCount` / `Box.TopVisibleLine` 四个公开方法；CPU 侧 5 条红灯（V3b 基线全红）→ 转绿 5 条、119 个测试文件逐个回归全绿。隔离 `:3` 真窗曾暴露 P1 锚点漂移 1 行与 P2 回绕性能退化（fps 39.67 / hitch 390.36），均已在当前线修好：**P1** 按 §1.4 CodeMirror「只重建变化区间」新增 `RebaseCount` 保留实测高度、用 `TopVisibleLine()` 做客观判据，自动自检 5 条逻辑探针全绿；**P2** CPU profile 定位到滚动脚本每帧 4000 次 `MoveCursorDown` 各触发一次 `Box.sync→BuildTextLayout`，改用引擎已有的 `BeginBatchEdit/EndBatchEdit` 合并为一次，60s 回绕窗口两轮复测 `fps 58.93–59.01`、`p95 20.26–20.29`、`hitch 2.00–4.00` 全达标；**P3** Golden 已按零容差比对 PASS（763584 px，diff=0，像素探针 68960 个文字像素）。人工常驻窗已开、用户已看画面确认 OK，Golden 基线随之认可；标准关闭窗口已补跑（exit 0、出汇总 JSON）。**诚实项**：首次常驻窗被外部 SIGKILL（exit 137，无汇总 JSON，:3/:0 均健康、无残留、无 OOM/段错误）；`OnEvent` 的 `EventKey` 打日志/改标题分支因隔离环境窗口树对注入工具不可见而未实测，不标绿。
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
| V2 | 视口行模型 | **V2a 已完成**：`ui/rendering/text_viewport.go` 新增 `TextViewport`（`LineSource` + 懒物化 + 有界缓存 + `VirtualTextLines` 高度）；首屏 1e6/1e4 = 0.80×，几何与全量布局逐项一致，缓存 O(可见行)；V2a 覆盖不回绕模型。**V2b-1 已完成**：`editorLineSource` 把生产 buffer 接成 `LineSource`，真实 1e6 行 Editor 首屏 120µs（1e4 459µs，0.26×），`Invalidate/InvalidateLine` 可用。**V2b-2a 已完成**：`RenderText` 虚拟绘制（只绘视口行，无窗口不画），逐行绘制抽成 `paintTextLine`。**V2b-2b-1 已完成**：多行框虚拟 sync/光标/命中/拖拽/上下键，1e6 行首同步 125µs，`b.txt.Text` 保持空（不 GetText）。**V2b-2b-2 已完成**：虚拟选区高亮（按可见行 `BoxesForRange`）与 IME 候选框；非折叠选区/IME 不再回退全量。**V2 已收口（2026-11-13 真窗）**：1e6 行 120s fps 58.54 / p95 18.92 / hitch 1.0 / RSS 稳态 21542KB/min pass | 1e6 行首屏/滚动/hitch 达标；hitch 红未消不得标绿 |
| V3 | 变高/回绕估算 | **V3a 已完成（2026-11-13）**：`ui/rendering/height_map.go` 稀疏高度图（每行估算 + 实测覆盖 treap，FIFO 上限 32768）；3e6 行/32768 实测堆 2.37MB，Offset/Index O(log n·log m)，查询 3M/1e4 墙钟比 2.12；旧 `VirtualTextLines` 3e6 实测 51.01MB。**V3b 已完成（2026-11-13）**：`TextViewport` 接高度图（`syncCount` 行数随源同步、`Invalidate` 重置实测高度、`InvalidateLine` 单行回落估算、`Prepare` 量测后按实测重定窗口直到覆盖视口带）；回绕视觉行映射（`ViewportLine.Rows`、`LineRowText`）与虚拟绘制/录制逐视觉行提交；测试：回绕逐行几何与整篇布局一致、估算偏大时窗口仍盖满视口带、1e6/1e4 回绕首屏 1.02×、200 窗滚动实测/缓存有界、录制行数=窗口视觉行数；先红证据（V2 旧 `Prepare` 带底短 474px、只画首行时 21 行仅录 3 行、HEAD 行数增长后 `LineCount` 仍 100）。**V3c 已完成（2026-11-13）**：`canVirtual` 去掉 `b.wrap` 拦截；`wrapWidth()` + `NewWrappedTextViewport` 按宽度重建共享视图；`rebaseVirtual` 用 `AnchorAt` + `ConsumeEditSpan` 在行数变化编辑后按锚点重定 `scrollY`（同数编辑只 `InvalidateLayouts`）；`virtualVertical` 接到 `OnKey` 上下键与 `extendVertical` 虚拟分支，按视觉行移动，选区映射走 `Buffer.UTF16OffsetAt` 不物化整串（§2.2）；公开 API `TextViewport.AnchorAt` / `InvalidateLayouts`。红灯 4 条 V3b 全红 → 转绿 4 条；废掉锚点重定的反证红灯 `top line 799→774`；新增 P1 红灯 `MeasuredCount 26→1`（`Invalidate()` 丢实测高度）→ 用 `RebaseCount` 转绿；119 个测试文件逐个回归 0 FAIL。真窗 P1/P2/P3 收口证据见 §8：5 条逻辑探针全绿、Golden 零容差 diff=0（763584 px）、60s 回绕窗口两轮 `fps 58.93–59.01 / p95 20.26–20.29 / hitch 2.00–4.00` 全达标 | 3e6 行变高内存 ≤1KB/千行；锚点不跳变（真窗 `anchor_top_line_stable` 绿）；滚动 fps ≥55 / hitch ≤5（60s 回绕两轮达标） |
| V4 | 超长行分块整形 | **阶段 1（分块物化）+ 阶段 2/3（横向视口/真窗）+ 第四层重做（A/B/C）均落地（2026-11-13）**：① `RenderCap`(10000 字符)/`ShapeCap`(20000 字符) 两个独立可配上限（`SetRenderCap`/`SetShapeCap`），`SetViewportHint(scrollX,visW)` 驱动窗口；② `ui/rendering/long_line.go` 2–4KB 块模型（FNV 内容哈希块 shape cache 提到 `RenderText` 级跨编辑保留、`TestV4_BlockShapeCacheSurvivesEdit` 钉、块宽前缀和、`ByteAtX`/`XAtByte` 像素↔字节映射），窗口按**可见像素带宽** + 512B overscan、钳制在 RenderCap 内（真窗实测 `window=(0,631)`，非 10000）；③ `adaptLongLineLayout` 把 glyph/caret X 从窗口相对平移成**全文绝对**（修 `hCullWindow` 用全局 `scrollX` 裁剪、长行窗口 glyph X 却从 0 起导致的横向滚动内容错位，`TestV4_HorizontalCullCoordinates` 钉 `first X=0.0`→`1334.2`）；④ 截断省略标记按 Skia `TextLine::createEllipsis` / Flutter `TextOverflow.ellipsis` 做成**独立 GlyphRun 挂在布局字形表**：不动窗口文本，用行尾具体子 face 整形 `…` 并走 `DrawShapedGlyphs` 原生链（前三次 `DrawString`/字符串替换/字节偏移替换均失败，根因见 §8）；`TestV4_TruncationMarkerInGlyphs` + 真窗 `marker_ink_diff=16` 并排像素差分（反证换基线→`0` 判红）双证据；⑤ 独立真窗 `examples/ui_text_v4_longline`：4 逻辑探针全绿、Golden 零容差 `diff=0/763584px`、1px 负向 `0.000131%` 红→还原绿、60s 人工窗 `fps 59.75 / p95 19.30 / hitch 0.00` 达标、标准关闭无残留；121 个测试文件逐个回归 0 FAIL，全仓 build/vet/gofmt/apidoc/ledger 全绿。**诚实项**：跨块 bidi 探针（§2.3 line-level 回退）与 §2.2 CPU 侧门禁（首屏/caret/多行同步/3e6 行数编辑）已补并全绿（见 §8）；L1/L2/L3/S 四档真窗串行重跑已完成（2026-11-14，见 §8 最新修订），两处击键/caret 路径 O(n) 洞（`SetSelectionBytes` 物化整串、`capBytes` 整行 rune 计数）红灯起步修绿，修后 5e6 单行 Box 全路径击键 2.55ms（余量 6.3×）、真窗 5.47ms；剩余诚实项：L2 多行 hitch 骑线（4 轮 3.0–6.5/min 中位 5.0，GC 归因证据齐、修法待拍板）、`SetViewportHint`/`longLineWindow` 单行判定 IndexByte 残留 O(L) ≈1ms/op@5e6（多行判定语义涉及虚拟模式交互，未动）、独立滚动条拖动未实现（现靠光标跟随）、真键事件通道未实测（`keys=0`）。**V4 标绿（2026-11-14，带注记）**：能力面 + 真窗三证据 + §2.2 CPU 门禁 + 四档真窗（分指标口径）+ 两处 O(n) 洞修复 + hitch 分配削减专项第一阶段全部收口（§8 2026-11-14 两条修订）；注记=本机 spike 口径下 L2 hitch 4-7/min 骑线，慢帧主源定性渲染域（atlas 脏区上传/present stall，已归位 `ENGINE_RENDER_GPU_CONVERGE_TASKS.md` §2.1，hal 层批量化另立专项），文本引擎侧已穷尽（spike 帧 CPU 50-55µs） | 5e6 字单行击键 ≤16ms（达标，余量 6.3×）；跨块 bidi 回退可用（§2.3 非目标 line-level 回退） |
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
| 2026-11-13 | V2b-2b-2 完成：`canVirtual` 放开非折叠选区/IME；`syncHighlight` 虚拟分支按可见行拼 `BoxesForRange`，`IMERect` 虚拟分支按可见行求并集；测试断言选区有高亮框、IME 框 W/H>0、`b.txt.Text` 仍为空。1e6 行首同步 125–223µs；textinput 整包绿（rendering 一次已知计时门禁偶发，复跑绿）。V2 待真窗收口。 |
| 2026-11-13 | V2 真窗收口：新建 `examples/ui_text_large_scale`（生产多行框 1e6 行）；隔离 :3、120s、标准关闭。fps_interval 58.54、p95 18.92ms、hitch 1.0/min、present 7024、virtual_mode=true、lines_moved 242362；RSS 稳态 21542KB/min（<30000，valid=true，pass），原始 start/end 190/505MB；110s 堆 173MB（字体 76MB + 文本 58MB，去字体约 97MB，符合 D5 预算）。wall first-present 327ms 为启动/字体/61MB 文本构造，布局首同步 125µs。 |
| 2026-11-13 | V3a 完成：`HeightMap` 稀疏区间高度图（估算基线 + 实测 treap + FIFO 32768 上限）；旧 `VirtualTextLines` 3e6 变高 51.01MB → 新结构 2.37MB（≤3MB），Offset/Index 查询 3M/1e4 比 2.12；参考模型/上限/边界测试全绿，rendering 整包绿；V3b 回绕映射待做。 |
| 2026-11-13 | V3b 完成：`HeightMap` 接入 `TextViewport`——`syncCount` 跟随 LineSource 行数（先红：HEAD 增长到 150 行后 `LineCount` 仍 100）、`Invalidate` 清实测高度、`InvalidateLine` 单行回落估算、`Prepare` 每轮量测后重定窗口（先红：V2 旧循环「估计偏大」带底短 474px 的空白带）；回绕视觉行映射 `ViewportLine.Rows`/`LineRowText`，`paintVirtual` 与 `recordRenderText` 逐视觉行提交（先红：只画首行时窗口 21 行仅录 3 行、缺 18 个续行）。门禁：回绕逐行几何=整篇布局、1e6/1e4 回绕首屏 1.02×、200 窗滚动实测 ≤ cap 且缓存 O(可见)、异步录制行集合与窗口视觉行一致；rendering/textinput 按文件回归见提交记录。V3c（Box 放开 wrap + 锚点）另见下一条。 |
| 2026-11-13 | V3c 完成（Box 放开 wrap + 锚点）：先存补丁回到 V3b 基线写 4 条红灯 `TestBoxV3c_*`（wrap 进虚拟、按宽度重建视图、锚点不跳、视觉行上下键/Shift 扩选、不物化整串），V3b 上 4 条全红；再应用实现逐个转绿，补齐 `virtualVertical` 接线（`OnKey` 上下键 + `extendVertical` 虚拟分支），把 `extentForBytes`/`extendExtentBytes`（内部 `Editor.Text()` 会物化整串）换成 `Buffer.UTF16OffsetAt` 满足 §2.2 硬约束，并修正 `TextRange().Affinity` → `SelectionRange().Affinity`；新增公开 API `TextViewport.AnchorAt` / `InvalidateLayouts`。证据：废掉锚点重定后反证红灯 `top line 799→774`；`go build ./...`、`go vet`、`gofmt -l` 全干净；`ui/rendering`、`ui/textinput`、`ui/textbuffer` 共 **119 个测试文件逐个 PASS，0 FAIL**。诚实项：`rebaseVirtual` 借消费式 `ConsumeEditSpan` 定位编辑行，虚拟分支不走 `syncBoxText` 故当前无冲突，但语义脆弱（未复现故障，本轮不改）；GPU 真窗 wrap 场景另轮验收。 |
| 2026-11-13 | **V3c 真窗验收 FAIL（未收口）**：隔离显示 `:3` 跑 `examples/ui_text_large_scale -wrap`（1e6 行、130MB、每行回绕成 2 视觉行）。**客观证据**：① 回绕确实生效（GPU 回读快照可见同一行号内容折到下一行；自检 `visual_row_down` caret +113 < 逻辑行长 122，未跨逻辑行）；② 逻辑探针 5 条中 `anchor_top_line_stable` **FAIL**：插入换行后顶部逻辑行 `499963→499964`（漂移 1 行），定位 `rebaseVirtual` 用 `v.Invalidate()` 全量重置高度图（丢实测高度）后重定 `scrollY`，随后 `Prepare` 重测可见行又改变高度图 → 顶部行漂移；CPU 单测「同一高度图下行号自洽」口径未覆盖此路径；③ 60s 稳态 `fps_interval 39.67`（门禁 ≥55）、`hitch_rate_per_min 390.36`（门禁 ≤5）双双 FAIL，`interval_p95_ms 21.26` 达标，V2 不回绕基线 58.54/18.92/1.0；RSS 278896→671560KB（60s 口径 inconclusive）；④ 15s 自动自检 Golden 缺失按 `wrkit.GoldenGate` 契约诚实判 FAIL（未种基线，禁止假绿）。**问题归位**：`P1 锚点漂移 1 行`（现象：顶部行 499963→499964；位置：`ui/textinput/box.go rebaseVirtual`；与真值差异：应 0 行漂移；归属：V3c 能力点；状态：待修，方向按 §1.4 CodeMirror「编辑只重建变化区间」保留实测高度）；`P2 回绕性能退化`（现象：fps 39.67/hitch 390.36；位置：待画像；与真值差异：V2 基线 58.54/1.0；归属：V3c 性能红线；状态：待定位/优化）；`P3 Golden 基线未种`（状态：待人工确认截图后种入并复跑 0 容差比对）。**状态**：V3c 不得标绿；CPU 部分（4 红灯/4 绿灯/119 文件回归）有效，真窗部分未收口。 |
| 2026-11-13 | **V3c P1/P2 修复 + 真窗自动验收 PASS**：P1 按 §1.4 CodeMirror「编辑只重建变化区间」新增 `TextViewport.RebaseCount(editLine, delta)`（导出实测节点、按行号平移、只让编辑行回落估算），`rebaseVirtual` 不再 `Invalidate()` 全量重置，并改用光标行定位编辑点（不再借用消费式 `ConsumeEditSpan`，该通道归 `syncBoxText`）；新增 `Box.TopVisibleLine()` 公开查询口，把真窗锚点判据从「原始 `scrollY` 数值」换成「顶部逻辑行号」（高度图重建会合法改变数值，数值变化≠视口跳变）；自检脚本改用视口内编辑且 `-auto-only` 强制静止视口（原先与滚动脚本并发、又把光标扔到文档中部让 `ensureVisible` 合法跟走，是判据场景错误，非功能 bug）。红灯 `TestBoxV3c_RebaseKeepsMeasuredHeights`：`MeasuredCount 26→1` → 修后 5 条 V3c 测试全绿、119 文件回归 0 FAIL。P2 用 CPU profile 定位：`main.(*ticker).Tick` cum 68%，`Editor.MoveCursorDown` 每次 → `SetSelection→changed→OnChange→Box.sync→syncVirtual→TextViewport.Line→BuildTextLayout`（cum 59%），滚动脚本每帧最多 4000 次移动即 4000 次整行排版；回绕后每行 122 字符 / 2 视觉行使成本翻倍 → `fps 39.67`、`hitch 390.36`。修法是示例脚本改用引擎已有的 `BeginBatchEdit/EndBatchEdit` 把整批移动合并为一次 `OnChange`（能力在引擎已存在，属示例用错 API，未在示例绕洞）。60s 回绕窗口两轮复测 `fps 58.93/59.01`、`p95 20.29/20.26`、`hitch 2.00/4.00` 全达标；60s 不回绕对照 `fps 57.81–58.03`、`hitch 0–1`，`p95 22.95–23.03` 略超 22（当前机器基线，batch 前同窗 26.75 更差，非 V3c 引入）。P3 Golden：`GPUI_ACCEPT_GOLDEN=1` 种子轮按契约 `seeded=true` 明确不算绿，随后零容差比对轮 `gate_pass=true`、diff=0、763584 px、像素探针 68960，5 条逻辑探针全绿。**人工二合一**：常驻窗已开、用户已看画面确认 OK，Golden 基线随之认可；补跑 `-wrap -manual-seconds 20` 标准关闭窗口（exit 0、`evidence_mode=manual`、fps 58.58、出汇总 JSON）。**诚实项**：首次常驻窗挂 1h30m 后被外部 SIGKILL（exit 137），未走标准关闭、无汇总 JSON（:3/:0 显示健康、无残留、无 OOM/段错误，非主动硬杀）；`OnEvent` 的 `EventKey` 打日志/改标题分支因 rootless Xwayland 窗口树对 xdotool/python-xlib 不可见而未实测（代码路径已核：`EventKey`/`KeyCode`/`SetTitle` 均存在），记为未验证、不标绿。 |
| 2026-11-13 | **V4 开工（红灯起步）+ 两处台账销账**：按用户确认，先对齐滞后文字（本文件头部 V3c 段首「人工二合一待确认」→「用户人工确认画面 OK」；`ENGINE_RENDER_GPU_CONVERGE_TASKS.md:52` 当前焦点 O7 → 一行收口索引）。V4 红灯 `ui/rendering/v4_longline_test.go`（testdata 种子 + 循环索引、每击键改不同位置、生产 `SetTextSpan+TextLayout`）：1e6 字击键中位数 **724–784ms**（两轮实测，门禁 ≤16ms，45–49×）、5e6 字 **3.95–4.08s**（247–255×）；两轮均 FAIL，1e6→5e6 约 5.2–5.5× 线性增长，与 V0 §2.1 的 O(L) 结论一致。绝对值低于 V0 的 1.804s/10.412s 是语料差异（本例 ASCII 种子、V0 为「a世」混合，CJK 簇整形更贵），两端口径不同、不混用。红灯确认后停在「停下报告」节点待方向确认，**V4 未实现**。 |
| 2026-11-13 | **V4 开工前调研（停下报告）**：① 成熟框架核查：Skia `modules/skparagraph`（`ParagraphImpl::layout(rawWidth)` 整段布局，只有 `maxLines`/`ellipsis` 截断；`Run.cpp:356 prefixWidth` 是省略号按比例分配宽度，非文本前缀宽度，不可复用）、Flutter `text_painter.dart`（`maxLines`/`ellipsis` 齐，无视口/分块/前缀宽度）**均没有超长行分块整形与横向视口原生能力**，按 D1/VS Code 口径自研，不重复手写库里已有的算法。② CPU profile（5e6 字单行击键，8.33s / 10.18s samples）把 O(L) 瓶颈定位到行号：`ui/rendering/layout_update.go:167 patchRows` cum 4.07s（40%）、`ui/rendering/text_layout.go:662 buildShapedCarets` cum 5.98s（59%）、`:661 itemizeRuns` 2.10s、`runtime.memmove` flat 26% + `growslice` cum 29%；`render/text.Shape` 仅 20.53% → **主要成本是整行 caret/簇表全量重建与整表拷贝，不是整形**。③ 影响面：`ui/rendering`（layoutCache / TextLayout 主战场）、`render/text`（块级 shape cache 支撑横向滚动）、`ui/textinput`（横向视口 / 截断标记 / 复制范围 / 上限可配置 API）；公开 API 新增、真窗横向视口三证据。**问题归位**：§4 V4 原写「块级 shape cache + 前缀宽度」偏重整形，与 profile 证据不符，实施顺序待用户拍板；状态：V4 未实现，停在「停下报告」节点。 |
| 2026-11-13 | **V4 阶段 2/3（部分收口，整体不标绿）**：用户拍板「块缓存按冗余处理」+「继续做完含真窗」。① 冗余依据：D1 窗口仅 10000 字节，滚动时重新整形一次 ~4ms（门禁 ≤16ms 已达标），独立块级 shape cache / 前缀宽度数组无收益；横向位置按平均字宽估算（D1 远端估算），`render/text` 不新增底层公开 API，符合「不过度工程」「只服务一家不进 render」。② 已落地：`RenderText.SetLongLineWindow(start)` + `LongLineInfo.WindowStart`；`ensureLayout` 把行表偏移改为全文绝对、`l.Text` 保留全文，所有查询口（Line/CaretForOffset/BoxesForRange/GetPositionForOffset）继续吃/吐全文偏移（红灯 `TestV4_LongLineHorizontalViewport` 先以 caret 位置断言逼出 `xIn=xEnd=71269.6` 假绿，修后转绿）；`Box.sync` 光标跟随滑窗、`canVirtual` 加 `LineCount()>1` 让超长单行留在 RenderText 窗口路径（红灯 `TestV4_LongLineStaysOnRenderTextWindow`）；`Box.LongLineWindow()` 公开查询口；独立真窗 `examples/ui_text_v4_longline`（`-auto-only` 三证据：3 逻辑探针全绿 d1_window/not_virtual/horizontal_viewport、像素 3486、Golden 零容差 diff=0/763584px；`GPUI_ACCEPT_GOLDEN=1` 种子轮按契约不算绿；60s 人工窗二合一）。全仓 build + 121 个测试文件逐个回归 0 FAIL、vet/gofmt/apidoc/ledger check 全绿。③ **诚实项（未收口）**：截断省略标记三种做法均失败——`paintTextLine` 单独 `DrawString` 被 GPU 批量字形延迟提交覆盖（分支探针确认执行）、窗口文本等长替换 + `displayText` 覆盖 `DisplayLines()` 后 GPU 回读快照仍无标记（composite face 的 `paintCompositeRuns` 绕过字符串级标记），按「搞不定如实回退、不交付无效改动」全部回退，V4 整体不得标绿；独立滚动条拖动未实现（现靠光标跟随滑窗）；`longLineWindow` 每次 `strings.IndexByte` 只读扫全文 O(n)（未物化但规模未完全解耦）；跨块 bidi/字素边界失败模式未探针；人工窗已跑（隔离 :3、60s 标准关闭，fps 59.67 / p95 19.22 / hitch 0.00 达标；keys=0 真键事件通道未实测、close_req=false 为超时关闭），人工画面确认待用户。 |
| 2026-11-13 | **V4 阶段 1 完成（分块物化，不标整体绿）**：按用户拍板「先分块物化」，`ui/rendering/text.go` 加 D1 可配置上限（`DefaultMaxSingleLineBytes=10000`、`MaxSingleLineBytes`/`SetMaxSingleLineBytes` 公开、`LongLineWindow` 查询口），`ensureLayout` 只把窗口文本交给 `layoutCache`（全文仍留 `RenderText.Text` 供复制/全选），窗口外/跨窗口编辑丢 span 走同文快路；`ui/rendering/text_layout.go` 加 `LongLineInfo`/`TextLayout.LongLine`，越界偏移由既有 `clampOffset` 钳制。红灯两轮曾 724–784ms / 3.95–4.08s，阶段 1 后 1e6 字**窗口内真实打字 3.98ms**、5e6 字远端编辑 **532µs**（门禁 ≤16ms）；`TestV4_LongLineWindow` 锁全文保留/窗口内编辑真被布局/上限可配/-1 禁用/越界钳制不崩；`m5_singleline_test.go` 显式 `SetMaxSingleLineBytes(-1)` 保持其 50k 全量口径。全仓 `go build ./...`、vet、gofmt、120 个测试文件逐个回归 0 FAIL。**诚实项**：阶段 2/3 未做——省略标记尚未绘制、窗口外光标钳到窗口末尾（横向滚动待阶段 3）、`longLineWindow` 每次 `strings.IndexByte` 只读扫描全文（未物化但规模未完全解耦，待阶段 3 改用 Editor 行信息）、跨块 bidi/字素边界失败模式未探针、真窗三证据待阶段 3。**V4 整体不得标绿**。 |

| 2026-11-13 | **V4 第四层重做（A 段完成、B 段失败回退、C 段 Golden 重种；整体不标绿）**：用户审查判定「固定 10000 字节窗口」偏离 §3 第 4 层（渲染上限≠窗口大小、缺块整形/前缀宽度/可见窗口），拍板按立项书重做 + 拆 `RenderCap`/`ShapeCap`、按接口全量切换制改完 7 个调用方。① **A 段落地**：`ui/rendering/long_line.go` 新增 2–4KB 块模型（FNV 内容哈希块 shape cache 提到 `RenderText` 级跨编辑保留，`TestV4_BlockShapeCacheSurvivesEdit` 钉）、块宽前缀和、`ByteAtX` 把 scrollX 映射成字节窗口；窗口按**可见像素带宽** + 512B overscan、钳制在 RenderCap 内（真窗实测 `window=(0,631) size=631`，非 10000），复用 `text.LayoutText(...).Width`，`render/text` 不新增底层 API。两条第四层红灯（200px 窗口按可见宽度 / scrollX 滑窗）转绿，8 条 V4 单测 + 121 文件逐个回归 0 FAIL、全仓 build/vet/gofmt/apidoc/ledger 全绿。击键 12–13ms 达标但**贴近 16ms 门禁、余量不足**（O(n) 字符串比较/换行扫描未消除，诚实项）。② **B 段截断标记**：按用户要求「参考成熟框架」调研 Skia `TextLine::createEllipsis`（`…` 作为独立 Run 挂 TextLine）/ Flutter `TextOverflow.ellipsis`，复用本仓库 `TextOverflowEllipsis`/`ellipsizeToWidth`。三种做法（`DisplayLines` 分支、layoutText 尾部等长标记、可见右端标记 + 全量构建）**均未在 GPU 回读快照观察到标记**：根因是 `paintCompositeRuns` 读 `LineGlyphs` 而非 `DisplayLines` 字符串，且标记曾在窗口末尾（屏幕外）。按「搞不定如实回退、不交付无效代码」全部回退，需后续专项重排绘制路径，B 段未收口。③ **C 段**：Golden 重种（备份旧基线→种子轮 `seeded=true` 按契约不算绿→比对轮 `gate_pass=true` diff=0/763584px）、60s 人工窗二合一。负向对照、§2.2 全档门禁、跨平台格、跨块 bidi 仍未做。**V4 整体不得标绿**。 |


| 2026-11-13 | **V4 第四层 B 段收口 + 横向滚动坐标系修复（整体仍不标绿）**：按用户「一起修、做到标绿条件」继续。① **B 段第四次根因钉死**：隔离探针（走真实生产路径）逐个定位前三次失败——独立 `DrawString` 被 GPU 批量字形延迟提交覆盖；`DisplayLines` 字符串改动被 `paintCompositeRuns` 读 `LineGlyphs` 绕过；按**字节偏移**等长替换 `…` 落在多字节汉字中间 → `validUTF8=false` → `emoji.Segment` 切分越界 panic（探针复现 `decorated[105:118]="56789…\x96界\xe6"`）。rune 边界替换对照探针证明 `LineGlyphs` 能含 `…`（GID 2825）。② **实现**：对齐 Skia `TextLine::createEllipsis`（`shapeEllipsis` 把 `…` 做独立 Run 挂 TextLine），`adaptLongLineLayout` 裁掉可见带右端之后的字形、把 `…` 用**行尾具体子 face**（`*text.sourceFace`；`*text.MultiFace` 不能直接 `DrawShapedGlyphs`）整形为独立 `LineGlyphRun` 追加；窗口文本不改写、UTF-8 与 cluster/byte 偏移不动。③ **坐标系洞一并修**：`hCullWindow` 用全文 `scrollX` 而长行窗口 glyph X 窗口相对 → 滚动内容错位；适配层按 `XAtByte(WindowStart)` 把 glyph/caret X 平移成全文绝对。④ **证据**：3 条 CPU 红灯（`…` 字形 GID+位置、UTF-8 合法、cull 坐标）+ 1 条真窗并排像素差分全绿；`marker_ink_diff=16`（`…` 三点墨迹），反证把对照图换成有标记图→`0` 判红；Golden 重种 `diff=0/763584px`；1px 负向 `0.000131%` 红→还原绿；60s 人工窗 `fps 59.75 / p95 19.30 / hitch 0.00`、标准关闭、`:3`/`:0` 健康无残留；121 文件逐个回归 0 FAIL、build/vet/gofmt/apidoc/ledger 全绿。⑤ **诚实项**：击键 13.93/11.34ms 贴 16ms 门、O(n) 扫描未消除、跨块 bidi 与 L1/L2/L3/S 全档门禁未重跑、独立滚动条未做、真键通道未实测、人工画面确认待用户。V4 整体不标绿。 |

| 2026-11-13 | **用户拍板 V4 标绿口径**：真窗人工画面确认 OK（二合一闭合）；标绿须先补齐 §2.2 的 L1/L2/L3/S 四档全量门禁（首屏 ≤100ms / caret ≤4ms / 行数变化编辑 ≤16ms@3e6 行 / 内存预算 / 多行同步 ≤4ms@1e6 行）与跨块 bidi 字素边界探针，全绿再谈。本轮 12 文件按清单先提交，**V4 整体仍不标绿**；全档门禁与 bidi 探针为 V4 剩余子项，下一阶段执行。 |

| 2026-11-13 | **V4 标绿前置 · CPU 侧门禁 + bidi 探针完成（用户拍板「补齐全档门禁再标绿」的第 1 步）**：按 §2.2 准确口径补 `ui/textinput/v4_gate_test.go` 四项客观测量（唯一行、每轮改不同位置、中位数、防假绿有效性断言：虚拟模式真进入 / caret 视口真滑动 ≥5 行 / 行数编辑真 ±1 行）：1e6 行首屏 `17.1µs`（≤100ms）、caret 移动/命中 `9.5µs`（≤4ms，走生产 `caretAnchor`，纠正了误用 `TextLayout.CaretForOffset` 全文偏移的假红）、多行同步 `4.4µs`（≤4ms，V0 的 19.27ms 红已由 V2 视口模型修复）、3e6 行行数变化编辑 `36.7µs`（≤16ms）；`ui/rendering/v4_bidi_test.go` + `testdata/v4_bidi_seed.txt` 补长行窗口 RTL(阿拉伯/希伯来)+LTR+CJK 混合探针：不崩、UTF-8 合法、字形非空、标记存在、caret 可查（不承诺跨块重排，§2.3）。123 个测试文件逐个回归 0 FAIL、build/vet/gofmt/apidoc/ledger 全绿。**剩余**：L1/L2/L3/S 四档真窗串行重跑（隔离 :3、标准关闭）与 RSS/首屏真窗口径，下一轮执行；V4 整体仍不标绿。 |
| 2026-11-14 | **V4 任务 20 · 四档真窗串行重跑完成 + 两处击键/caret 路径 O(n) 洞修绿（整体仍不标绿，剩两项待拍板）**：示例参数化（演示数据控制动词，合规）：`ui_text_large_scale` 加 `-lines`（1e6/3e6/1e7）、`ui_text_v4_longline` 加 `-bytes`（1e6/3e6/5e6）+ burst 计时探针（唯一行+不同位置、中位数、防假绿有效性断言：编辑真实发生/虚拟模式保持/窗口真滑动）+ v4 窗补 `proc.Sample()`（此前静态轮无样本致 RSS verdict 恒 inconclusive）与 `-cpuprofile`；L3 多行部分复用 L2 证据（同为 3e6 行）。**① 多行窗实测**（:3 隔离、串行、每轮健康检查、标准关闭）：L1 1e6 行 65MB `fps 57.46 / p95 19.47 / hitch 3.00 / 首同步 3µs / burst 257/138/122µs`；L2 3e6 行 197MB `fps 57.53–57.79 / p95 19.27–21.99 / hitch 6.50/4.00/6.00/3.00（4 轮中位 5.0 骑线）/ burst 194/134/135µs`；S 1e7 行 658MB 定高 `fps 57.64–57.75 / p95 19.13@120s、23.29@240s / hitch 5.50@120s、4.25@240s / 构造 5.86s（观察值）/ burst 186/142/134µs`。**② 单行窗实测**：L1 1e6 `fps 59.75 / p95 19.54 / hitch 0.00`、L3 5e6 `fps 59.75 / p95 19.82 / hitch 0.00`；**③ 两处 O(n) 洞（红灯起步→绿）**：(a) `SetSelectionBytes`（editor.go）旧实现 `len(e.Text())` 钳制 ×2 物化整串 + 两个全前缀 range 扫 UTF-16 计数，20MB 实测 21.08ms/次（53MB 60.3ms），超 §2.2 caret ≤4ms 5 倍——CPU 门禁只测了 `caretAnchor`（9.5µs）没测这个入口，正是 V0 §7.2 假快陷阱；修=buffer `Len()`+`UTF16OffsetAt`（V1b-3 已建 O(log n)，块边界不劈 rune 语义逐 rune 等价），`20.84ms→5.59µs`、真窗 caret `60.3ms→150µs`，锁 `TestV4Gate_SetSelectionBytes{NoMaterialize,Multibyte,CaretBound}`；(b) `capBytes`（long_line.go）守卫 `nChars>=RuneCountInString(整行)` 使每次窗口查询全行数 rune（画像 50.25% cum、`Window` 占 85.8%），Box 全路径 5e6 单行击键真窗 `28.8ms`（1e6=3.5/3e6=17.5/5e6=28.8 线性，超 16ms 门）——修=守卫融进步进 `O(min(cap,L))` 语义等价，`capBytes 5.34ms→85.9µs`、Box 路径 `13.38→2.55ms`、真窗击键修后 `1.30/3.86/5.47ms`（p95 ≤11.7ms 全过），锁 `TestV4CapBytes_{Equivalence,CappedWalk}`+`TestV4Gate_LongLineBoxKeystroke5M`。**④ 内存 D5（V2 口径：110s 堆快照去 75.4MB 字体文件缓存）**：L1 ls `112.9≤166MB` / L1 v4 `47.2≤65.5MB` / L2 `246.2≤373MB` / L3 v4 `60.1≤71.5MB` / S `713.3≤1099MB` 全过；S 堆 60s→110s diff `1.5MB/786MB` 零增长；rope 叶零拷贝共享原串（文本域=1× 文本）。**⑤ 既有门禁复跑未破坏**：v4 `-auto-only` 三证据逐值复现（`diff=0/763584px、marker_diff=16、MarkerX=8505.0`）、V3c `-wrap -auto-only` 5 探针+Golden 全绿；128 测试文件逐个回归 `126 PASS / 0 FAIL / 2 SKIP（无 Test 函数）`，build/vet（scoped；`ui/platform` 为存量 unsafe.Pointer 警告非本轮文件）/gofmt/apidoc/ledger 全绿。**用户拍板（2026-11-14，随本轮停下报告）**：(a) L2 多行 hitch 骑线→**开新专项修**（滚动布局路径分配削减，growslice 5.4%+mallocgc 11% 画像归因；专项动手前先做影响面评估并停下报告）；(b) 判定时长口径→**分指标口径**：p95/fps/hitch 用 120s（与 V2/V3c 基线同口径可比），RSS 稳态斜率用能到平台的时长（S 档 240s——C5 泄漏判定本质需要曲线到稳态；120s 的 40715KB/min fail 是 GC 头 room 爬坡尾巴），按此口径 S 档 p95 19.13 与 RSS 844KB/min 均绿；(c) IndexByte 单行判定残留（O(L) ≈1ms/op@5e6）留作诚实项（多行判定语义涉及虚拟模式交互，需单独评估）；raw RSS（L1 525MB/L2 835MB/S 1951MB）按堆口径判定、raw 如实上报。**V4 整体仍不标绿：hitch 分配削减专项完成并复测后再谈标绿**。 |

| 2026-11-14 | **用户拍板（任务 20 收尾）**：① L2 多行 hitch 骑线（4 轮 6.50/4.00/6.00/3.00 中位 5.0，GC 归因证据齐）→ **开新专项修**：滚动布局路径分配削减（画像归因 growslice 5.4% + mallocgc 11%），专项动手前先做影响面评估并停下报告；专项完成并复测后再谈 V4 标绿。② S 档判定时长口径 → **分指标口径**：p95/fps/hitch 用 120s（与 V2/V3c 基线同口径可比），RSS 稳态斜率用能到平台的时长（S 档 240s；120s fail 为 GC 头 room 爬坡尾巴，堆快照稳态零增长佐证非泄漏）——按此口径 S 档 p95 19.13 / RSS 844KB/min 均绿。③ 本轮 9 文件按清单提交（两洞修复+新测试+示例参数化+台账），全部本地未推送。④ IndexByte 单行判定残留（≈1ms/op@5e6）留诚实项。 |

| 2026-11-14 | **hitch 分配削减专项第一阶段收口 + V4 标绿（带注记）**：① 分配削减三处落地（红灯起步→绿）：`HbShaper` 持 hb Buffer 成员 Clear() 复用（Skia `HBBuffer fBuffer` 同款）+ runes scratch、`buildShapedCarets`/`rebaseGlyphs` 容量预估——`AllocsPerRun` **51.0→8.0 allocs/shape**（`TestShapeAlloc_ScrollTrafficBound` 红灯 51.0 超 40 界→修后绿），画像 growslice 5.4%→跌出 top12、mallocgc 11%→5.2%，byte-exact 全家+并发竞态全绿；② L2 复测五轮 `4.00/5.50/4.50/6.50/7.01`（中位 5.0）hitch 未动——**GC 主源假设证伪**（GC CPU 减半 hitch 不动）；③ 单帧分解钉死：spike 帧（caret 跳 66 行+sync+物化）CPU 仅 **50-55µs**，慢帧 34-47ms 为 spike 相位 0.4% 偶发，Go 画像穷尽后剩 cgocall（GPU present/驱动，21% CPU）；④ 环境漂移实锤：未改路径的 L1 从 3.0 漂到 4.5；⑤ 回归 131 文件 **0 真实 FAIL**（2 个外部工具链长跑 `zz_b1/zz_b2` 单独验证 PASS——1183s/1787s 远超回归脚本 600s 上限、2 个比值门禁互扰复验绿）。**用户拍板链**：骑线处置→「继续压 hitch」→ 单帧分解证实 CPU 侧穷尽→剩余两股（字形 atlas 脏区上传小块开销、940MX present/驱动 stall）**归位渲染域台账**（`ENGINE_RENDER_GPU_CONVERGE_TASKS.md` §2.1 余留登记，hal 层上传批量化/present 策略另立专项）。**V4 标绿（带注记）**：能力面（§3 第 4 层）+ 真窗三证据（4 探针/Golden diff=0/marker 差分/负向对照/人工确认）+ §2.2 CPU 门禁 + 四档真窗（分指标口径）+ 两处 O(n) 洞修复 + 分配削减全部收口；注记=本机 spike 口径下 L2 hitch 4-7/min 骑线，定性渲染域问题（见上），文本引擎侧无进一步手段。诚实项保留：IndexByte 单行判定残留、独立滚动条、真键事件通道、跨块 bidi 仅 line-level（§2.3）。 |

### 8.1 V4 平台覆盖（同功能多端一次标齐）

按本文件状态口径逐端填写；每端不留空白，未做的写明原因。V4 为纯 Go + `render/text` 整形 + `hal` 后端抽象，不直接触碰平台 GPU API。

| 平台 / 后端 | 分块窗口 + 截断标记 + 横向视口 | 证据 / 原因 |
| --- | --- | --- |
| Linux X11（隔离 `:3` XWayland） | 已实测达标 | 真窗 `examples/ui_text_v4_longline -auto-only`：4 逻辑探针全绿、Golden `diff=0`、1px 负向红→还原绿、`marker_ink_diff=16`；60s 人工窗 `fps 59.75 / p95 19.30 / hitch 0.00`、标准关闭无残留；2026-11-14 四档串行重跑复验同结果（L1/L3 单行 `fps 59.75 / hitch 0.00`，多行 L1/L2/S 见 §8 修订） |
| Linux Wayland（原生） | 未做 | GPU 真窗安全纪律要求隔离会话；原生 Wayland 走 `/gpui-wayland-nested`，本轮未开该会话 |
| Windows | 未做 | 本机无该环境；CPU 单测、Golden 与像素差分脚本不依赖平台，迁移后可直接复跑 |
| macOS / Metal | 未做 | 本机无该环境；同上，后端经 `hal` 抽象，V4 无平台专属分支 |

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

- 3e6 全量未跑；5e6 单行只测 1 次唯一编辑；1e6 唯一行全量未跑（外推 5s/3GB）。V1b-3 已落地（Editor 编辑/选区/IME/导航原语 O(log n)）；V2a/V2b-1/V2b-2a/V2b-2b-1 已落地（视图、生产 buffer 适配、RenderText 虚拟绘制、多行框虚拟 sync/光标/命中）；遗留：`Delta()` 导出仍整串物化，maxW 暂按可见行近似；V2 真窗已跑通（1e6 行 120s，fps/p95/hitch/RSS 稳态 pass），wall first-present 含启动/字体/61MB 文本构造。
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
