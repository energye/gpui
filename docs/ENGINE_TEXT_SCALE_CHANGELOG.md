# ENGINE_TEXT_SCALE_CHANGELOG — 变更过程存档

> 本文件只存过程证据：每次修订改了什么、旧数错在哪、探针原始输出。**要动手只读主文档**（`ENGINE_TEXT_SCALE_PLAN.md`），那里只写现在是什么。

## 修订明细（原 §10 全量，2026-09-02 – 2026-09-03）

| 版本 | 说明 |
|---|---|
| 立项 | 2026-09-02 · 基于 `ui_wr_ime_r*` 实测卡顿（排版占 86%、其中 76% 为单字量宽）立项；定 M0–M5 六期，硬目标 G1–G6；否决「按可见窗口整形」方案（正确性优先，改为整行整形+提交裁剪） |
| 重审 | 2026-09-02 · 补做三组探针实测（临时代码已删，仓库零改动），新增 §3.5 三条硬约束 + §9.1 四项待沟通决策。**约束①** 绘制 `snapPen` 逐字取整累加 vs 布局浮点累加，5000 字发散 **173px** → M2 由「性能优化」升级为「正确性前置」，必须在 M0 打通批量整形后实施。**约束②** 回绕模式下编辑会级联作废后续所有行（实测改首行后 L1 由 `[35,71)` 变 `[30,62)`）→ M1 原「只作废第 K 行」表述不成立，改为**按模式分：不回绕=行缓存，回绕=段缓存**，复杂度门禁分模式各跑。**约束③** `MultiFace.Runs()` 已存在且带软缓存 → M0 的 itemizer 由「自研」降为「接线+验证」，但实测 `Runs("Hello世界abc你好")` 只切 1 run，需补缺字场景回归锁。另发现 `glyph_mask_atlas.go` 已实现 1/4px 亚像素量化 → M5 的 DPR 量化降为验证项。**新增最大风险项**：`TextLayout.Lines` 被 29 处直接下标访问，懒视图改造是 M1 最大回归风险，已加 `TestTextLayout_LinesIndexable_*` 逐字节对照锁 + 全量物化兼容开关降级路径。缺失项由 7 件降为 6 件。 |
| 决策落定 + 分期细化 | 2026-09-02 · **四项决策用户确认**：Q1 选 **C**（按 run 批量整形+批量绘制，用 `glyph.X`）、Q2 选**段缓存**、Q3 **piece tree 押后为 M3.5 条件触发**（M3 只做增量 undo）、Q4 选**惰性 API + 旧字段 deprecated 分阶段迁移**。据此新增：① **§5.6 M6 验收主窗 `ui_text_edit_accept`**——真实**单行 + 多行**输入框，6 个场景区（A 单行短/B 单行 5000/C 单行 50000/D 多行短/E 多行 1e5 字 2000 行/F 多行回绕），人工 5 步 + 自动探针双轨，专属硬门禁 8 项（含 `caret_vs_paint_max_delta_px ≤ 2`、`T(C框)/T(B框) ≤ 1.5` 作为 **G1 主证据**），F0+F6+F7 像素断言；② **§9.2 新会话接手规范**——每期条目必须含九项（目标/前置依赖/背景根因/**精确到文件:行号的改动清单**/复杂度契约/验收清单/熔断/回滚/交接说明）+ 开工五步法 + 文档自洽性要求，确保**每期可由全新会话独立接手**；③ **§3.2.1 跨期不变量 I1–I6**（`glyph.X` 单源、缓存失效粒度、裁剪只影响提交范围、undo 只存增量、懒测量估算值、禁逐字 RuneAdvance），降级链标注对应不变量；④ **§0 新增硬目标 G7（新会话可接手）、G8（最终验收是真实输入框）**；⑤ M0–M5 各期条目全部按九项规范重写，改动清单精确到文件:行号。M3 范围收窄为「仅增量 undo」，复杂度契约表同步修正（插入/删除 O(log n) 移至 M3.5）。 |
| 全量问题纳入 + 分期重排 | 2026-09-02 · **用户质疑「只提 Lines，其它呢」→ 补测确认受影响的是四个面，非仅一个**。新增 **§3.6 全量问题清单**：**面1** 字段暴露（139 处引用，47 真下标 / 80 判空 / 19 遍历，其中 15 处主路径）；**面2** 方法内部全表遍历（**实测 2 万行 `CaretForOffset` 374.8ms、`BoxesForRange` 365.3ms，均 O(n²)**——根因 `CaretForOffset` 外层 `for i:=range l.Lines` 内调 `LineTop(i)`(O(i))、`BoxesForRange` 扫全表+行内线性找 X；**纯实现问题，与外部调用无关，此前方案严重低估**）；**面3** `Generation` 全局自增无法表达按行失效；**面4** 绘制侧平行实现（`DisplayLines()` 走 `wrapLines()/estimateWrapLines()`，与 `TextLayout` 非同一路径）——**实测确认两个存量 bug：MaxLines+Ellipsis 下绘制 2 行 vs 布局 4 行、无 Face 估算路径下绘制 10 行 vs 布局 1 行（点击第 5 行光标全落第 1 行），A1 单源纪律当前即已破损**。据此重排分期：**M0 增加面 4 单源修复为最先任务**（改动清单扩为 10 项，执行顺序「先 1–3 面4，再 4–10」）；**M1 重排为 M1-a/b/c/d 四步**——a 面2 去 O(n²)（第 1–4 项）、b 行索引+前缀和+缓存（第 5–10 项）、c 簇边界（第 11 项）、**d 面1 迁移四小步（d1 生产换新留脚手架 → d2 13 个老测试一行不改写跑一遍当独立裁判 → d3 老测试改新 API 断言不动 → d4 删 `.Lines` 拆门，全部在 M1 内完成不留兼容期）**。新增不变量 **I7**（绘制侧必须从 TextLayout 派生）、**I8**（CaretForOffset/BoxesForRange/LineTop 禁全表遍历）、**I9**（Generation 为按行失效标记）。复杂度契约表补三行 O(n²)→O(log n)。§4 场景矩阵补「行数规模」「单源一致性」两维。§7.1 降级链补三条。§9 风险表补五项。Q4 由「选项 3 分阶段兼容」升级为「**选项 4 · 先绿后拆**」（老测试当一次独立裁判，用完即拆，不留兼容期）；原 `TestTextLayout_LinesIndexable_*` 改为 `TestLegacyLinesContract_*`（仅 M1-d2 阶段有效，d4 后随字段删除）。 |
| 终审收敛 | 2026-09-02 · 全文档通读校验，修正 10 处不一致：① **§2.1 CPU profile 数值缺失**（原始粘贴丢列）→ 补全 flat/flat%/cum%/cum 四列并加读法说明；② **§3.5 约束① 仍写「三选一待拍板」** → 改为已决策状态（Q1 选 C，划掉 A/B）；③ **§3.1 分层图**仍写 `piece tree`/`itemize`（均已变更）→ 重画为 L0–L7 + 旁挂 P1/P2，标注建立阶段与调用方向；④ **§3.3 硬分界**由三条增为四条，新增「回绕与不回绕是两套缓存结构，不是开关」；⑤ **§0 G1** 阶段标注（`M2 收口 ≤3`）与实际门禁不符 → 改为分档 **M1 收口 ≤5 / M5 总收口 ≤1.5**；⑥ **§4 场景矩阵**补充「M6 对应区」列 + DPR 改为「已实现·验证项」+ 新增「长度维度阶段分工」说明（原表误列 M3）；⑦ **§7.3 回滚**逐期写实（原只写 3 行且与 M3.5 决策不符）；⑧ **§7.4 并发**补充引用来源（IME 需求文档 §8 C1/C4）+ 懒测量的跨线程约束；⑨ **§8.2 缺失项**改表格并标注 2 件已覆盖/1 件条件触发；⑩ **§6.4 收口清单**位置错放（孤立于 §5.6 与 §6 之间）→ 移入 §6 下并补全（新增 M6 趋势记录、I1–I6 校验、按第⑧项实跑回滚）。另 §9 风险表新增「M6 自身不达标」「新会话误读」两项，§9.1 决策落地位置改为指向改动清单编号。 |
| **实证复核 · 收敛** | 2026-09-02 · **把文档每条关键论断拿回当前代码重跑探针复验**（探针已删，仓库零改动）。共修正 **12 处**。逐条明细见 **§11.0**；此处只记结论与影响：<br>**结论：方案主干（四个面的认定、`M0→M5` 分期、G1–G8 硬目标、Q1–Q4 决策）全部成立，无一条被推翻。** 被修正的是**数值精度**（2 项）、**行号**（4 处）、**能力现状**（3 项）；**新增发现 3 处缺口**（script/bidi 分段、`MaxLines` 布局侧截断、面2 第三处 O(n²)）；**新增 1 项风险**（M0「先慢后快」）。<br>**对施工的影响**：M0 基准门禁放宽（≤250ns 且 P99 不劣化）、M0 第 8 项须与 O(n²) 消除同时做、M0 新增 `TestShapeNonEmpty_MultiFace_*` 与 `BenchmarkCaretBuild`、M0 需叠加 `segment.go` 分段、M1 新增第 13 项（面2 第三处 O(n²)）与簇边界范围扩大、§8.2 缺失项 6→8 件。 |
| **可开工性校验 · 新会话视角** | 2026-09-02 · **以「新会话只读当期条目」的视角逐期走查**，补齐 5 处会让人卡住的地方：<br>**① 🔴 阻塞项：`ui_text_edit_accept` 不存在**，却被 M0–M5 每期引用 → 已新增 **M0-pre 前置任务**（M0 第 ② 项）。<br>**② 新增 §6.3.1 真窗生命周期表**（8 个窗的状态/建于/被谁用）。<br>**③ 新增「面 4 落地要点」**：`TextLayout` 缺 `MaxLines`/`Ellipsis`/无 Face 回绕三样能力（绘制侧都有）→ 不能靠删绘制侧实现，给出三方案选型 + 写死判据。<br>**④ 修正 M0 第 1 项描述**。<br>**⑤ M5 的 `ui_text_m5_anycase` 补标「新建」**。<br>校验结果：7 期九项规范全齐；行号 56 处中 55 通过；M0 的 8 个新测试均不存在 = 符合红灯起步；编译与基线绿灯。 |
| **五 agent 并行分类核验 · 合成** | 2026-09-02 · **开 5 个子 agent 分维度并行全量核验**（事实 / 数值 / 可开工性 / 纪律合规 / 结构收敛），结果统一合成。共修正 **26 处**，其中 **5 处为阻塞级**（不修就做错或卡死）：<br>**🔴 阻塞级 5 处**：(a) **`Generation`「零消费者」结论被推翻**——实测有 2 处消费者（`TestTextLayout_Generation` 断言全局递增、`ui_wr_ime_r2_textlayout:307`），原方案「直接改语义」会**让 M1-d2 的裁判测试变红 → d2 熔断，且崩因是文档给错情报**。已改为「保留 `Generation` 自增语义不动，**另增**按行失效字段」，I9 同步更正。(b) **M0 第 9 项改错地方**：`snapPen` 属 CPU 光栅兜底，**不在验收路径上**（主路径 `text.go:905` 已走 `DrawShapedGlyphs` → GPU，直接取 `glyph.X`）；对策 C 的实质是「让 MultiFace 也走通批量路径」，而非改 `snapPen`。已加路径要点。(c) **M0 第 8 项删错分支**：`buildCaretsForLine` 有两条兜底，`:170-188`（`face==nil`）**必须保留**——M1-d2 的 3 个老测试正是用 nil face 跑的，删了会**在 M0 阶段就拆掉 M1 的裁判体系**。已加要点区分。(d) **M1 第 11 项依赖未声明**：升级 UAX#29 字素簇**仓库无实现、需引入 `rivo/uniseg` 或自研**，原文档只字未提 → 新会话做到这里必卡死。已补三选项决策表（推荐引入 `rivo/uniseg`，建议记为 Q5）。(e) **M1 前置依赖指向错误编号**：原写「M0 第 5–7 项」（实为 advance 缓存），真正前置是**第 8、9–10 项** → 照错编号验收会漏掉正确性前置。<br>**🟡 重要修正 8 处**：Q1/Q2/Q4 落地位置编号**三个全错**（Q1 应 M0 第 8–10 项、Q2 应 M1 第 9 项、Q4 应 M1 第 12 项）；M1-d 步骤数统一为 **d1–d5 五小步**（§9.1 曾写四小步）；老测试括号 **11+3→10+3**；新增 **A1/A5 术语定义**（原悬空未定义）；M5「字段进入移除评估」改为「核验已随 M1-d5 删除」；行号 3 处落到注释行上（`input_box.go:494` 应为 `ui_wr_ime_r1_editor:494`、`text.go:897-899` 应为 901、`text.go:899` 应为 902）；§9.1 的 11+3 与 M1 主条目 10+3 打架。<br>**🟢 纪律与规范 6 处**：像素断言**补选 F1（滚动）+ F9（异步稳定）**（原只 F0+F6+F7，漏形态，会判 FAIL）+ 补 Golden/理论色容差声明；场景文案加「禁进 `ui/` 引擎层」护栏 + 提醒 M0 补 Ellipsis 须沿用 `textEllipsis` 常量 + `SetOverflow` 模式；**M5 字体盘点更正**（泰/缅字表字体**已存在**于 `hint/testdata`，真正缺的只有高棉+藏文）；真窗命名加定位声明（非 R/C 窗但自愿按 `ui_wr_*` 全套标准）+ 与 `ui_textinput_ime` 划清职责边界；表头「专属四项」→「专属八项」；`caret_vs_paint_max_delta_px` 等 3 个自定义指标加族归属说明（真源 J 族无数值字段）。<br>**📐 结构优化 1 处**：新增**「读法三档」**（最小开工 / 完整开工 / 全盘理解）——实测新会话按原读法开工 M2 需读 462 行（34%），最小档压到 **120–180 行（≈10–13%）**，§3 目标架构 260 行改为按需回查。<br>**核验规模**：事实 agent 全量核 **89 处行号 + 33 个符号 + 55 个路径 + 38 个测试名**；纪律 agent 逐条比对 **A–J 全族 8 项阈值**（与真源逐字一致，0 错误）；可开工 agent **7 期 × 6 维度**全走查；**数值 agent（补跑完成）复验 22 项数值论断**。<br>**🔬 数值维度 7 处修正（最后一个维度，补跑后合成）**：(a) **🔴 §2.1 A 表缺口径标注 → 单 face 慢 14.9 倍**：同 5000 rune 中英混排语料，`MultiFace` 2.290 ms 而 **`singleFace` 34.057 ms**（我已独立复现：2.290 / 34.057）。机制：`MultiFace` 因 `Shape` 返 0 glyph 走 **O(n) 兜底**，`singleFace` 走成功路径的 `CaretXForCluster` **O(n²) 扫表**（实测占其总耗时 98%）。**这直接印证 M0「先慢后快」熔断不是纸上风险——修通整形后 5000 字会从 2.29 ms 跳到 ~34 ms（慢 15 倍）**。已补 A-2 对照表 + 机制说明。(b) **🔴 `duffcopy` 归因错误（证据撤回）**：原称「undo 全量快照拷贝」，实测 `pprof -peek` 显示调用方是 `RuneAdvance`/`glyphForRune`（量宽链）；**对照实验决定性**——只跑 Editor 编辑 3000 次不触排版，profile 中 duffcopy/memmove **恒为 0**。根因⑥ 该证据撤回，改用「100 份快照常驻 18.54 MB」实测。(c) **根因⑥「每次编辑全量拷贝」表述不准确**：`pushHistory` 是纯 append，实测 **0.10–0.12 µs 且与文档长度无关**（Go string 只拷 header）；真成本是**内存常驻**而非编辑耗时。(d) **Q3 数值偏快 2 倍**：1e5/1e6 拼接实测 **70 µs / 240 µs**（原记 10/100 µs）。**决策保住**（240 µs 相对排版 2350 µs 仅占 10%，远未到 M3.5 的 30% 触发线），但数字改准以防将来误判触发时机。(e) **`ellipsizeToWidth` 实为 O(n) 非 O(n log n)**：`ell/一次Measure` 比值在 n=1e3→1e6 恒为 8.0–8.7× 不随 n 增长；M5 改前缀和后收益**比原写的更大**。(f) **「5000 字已卡」论据偏强**：2.64 ms 仅占一帧 16.7 ms 的 **15.8%**，不足以支撑"卡"。已补量化分级表——真正卡的是 **20000 字 68%** / **50000 字 174%（超一帧）**；**建议改用「50000 字超一帧 1.7 倍」或「单 face 5000 字 34 ms」这两个更硬的数当论据**。(g) **`100k 字 = 10MB` 仅在纯 ASCII 口径成立**（9.54 MB）；中英混排实测 **18.5 MB**，已注明口径。<br>**✅ 数值维度验证通过项（给人信心）**：`InputBox.Sync` 5000 字 2.68 ms → 实测 **2.64 ms**（高度吻合，A 表最准的数）；profile 两占比（`buildCaretsForLine` 86.44%→实测 87.93%、`RuneAdvance` 链 76.27%→实测 72.41%）**双双成立**；§3.2 契约表三行 O(n) 标注**全部准确**（×10 行数 → 耗时 ×10~12）；`CaretForOffset` 374.9 ms / `BoxesForRange` 380.x ms **第三轮独立复现**；M6 的 `≤16ms` 与 `T(C)/T(B)≤1.5` 两阈值**自洽且可达**（当前 29.09 / 11.04，G1 达标后自动满足，照着判不会误判）。<br>**⚠️ 明确未验证的 5 项**：`interval_p95_ms`、`fps_interval`、`hitch_rate_per_min`、`rss_slope_kb_per_min`、`caret_vs_paint_max_delta_px` —— 均需 `ui_text_edit_accept` 窗（**尚未创建**）+ GPU 环境，**未用估算值冒充实测**，待 M0-pre 建窗后补测。<br>**最终校验**：行号 69 处中 68 通过（唯一"越界"为修订表历史对照，应保留）；7 期九项规范 0 缺失；`ui/`+`render/` 编译通过、基线测试绿灯；**代码零改动**，探针已全部清理。 |

## 8.3 重审修正记录原表（2026-09-02，已搬出正文）

### 8.3 重审修正记录（2026-09-02）

| 原表述 | 修正后 | 依据 |
|---|---|---|
| M0「itemizer 自研」 | 改为「接线 + 验证」，`MultiFace.Runs` 已存在 | 约束 ③ 实测 |
| M1「编辑第 K 行只作废第 K 行」 | 区分模式：不回绕=行缓存，回绕=**段缓存** | 约束 ② 实测 |
| M1 未提及 `TextLayout.Lines` 公开面 | 补充：47 处直接下标访问，是最大回归风险 | 代码扫描（原记 29 为生产代码子集，口径不统一，已修正） |
| M2 定位为「性能优化」 | 升级为**正确性前置**（依赖 M0 消除发散） | 约束 ① 实测 |
| M5「DPR 量化」 | 降级为验证（图集 key 已量化 1/4px） | 代码核对 |
| 缺失项 7 件 | 降为 6 件 | 上述合并 |
| **单字量宽 8288 ns** | **稳态 ~430 ns**（复核：427/449 ns） | 复核重测，立项值为冷启动采样 |
| **发散 173px/5000 字** | **976.6px/5000 字**（每字 0.195px） | 复核重测（DejaVuSans 16pt） |
| **面4 2v4 / 10v1** | **2v12 / 11v1** | 复核重测 |
| **约束③「只切 1 run」** | **混排切 6 run；但阿拉伯+拉丁只切 1 run** | 复核重测 |
| **簇边界「按 rune 吸附」** | **已有 `snapToGraphemeBoundary` 但名不副实**（仅 UTF-8 续字节归位，且只在 1/3 条路径调用） | 代码核对 |
| **面3 `Generation` 11 处引用** | **五处递增、零消费者读取** → 无兼容负担，纯设计风险 | 代码核对 |

## 校验记录（原 §11 全量，2026-09-02）


### 11.0 实证复核（2026-09-02 · 本文档最重要的一次校验）

> 与 §11.1 的「文档自洽性校验」不同，本节记录的是**把文档里的每条关键论断拿回当前代码重新跑一遍**的结果。所有探针为临时代码，**已全部删除，仓库零改动**（`git status` 仅本文档）。

| # | 论断 | 立项/重审记录 | 复核实测 | 判定 |
|---|---|---|---|---|
| 1 | `MultiFace.Source()` 返 nil → `Shape` 返 0 glyph | 是 | **确认**：`Shape(MultiFace)=0`，`Shape(单face)=12` | ✅ 成立 |
| 2 | 单字量宽 `RuneAdvance` 成本 | 8288 ns | **~430 ns**（单 face 427 / MultiFace 449） | ❌ **修正**（差 19×，立项为冷启动值） |
| 3 | 绘制/布局位置发散（5000 字） | 173 px | **976.6 px**（每字 0.195px，线性） | ❌ **修正**（实测量级更严重 5.6×） |
| 4 | 回绕下编辑级联作废后续行 | L1 `[35,71)`→`[30,62)` | **逐字复现** | ✅ 成立 |
| 5 | 不回绕下行隔离成立 | 差值 0.000000 | **确认**：170.593750 vs 170.593750 | ✅ 成立 |
| 6 | `CaretForOffset` O(n²)（2 万行） | 374.8 ms | **365.9 ms**；行数×10 → 耗时×100 | ✅ 成立 |
| 7 | `BoxesForRange` O(n²)（2 万行） | 365.3 ms | **377.6 ms**；同上 | ✅ 成立 |
| 8 | 面4 单源破损 | 2v4 / 10v1 | **2v12 / 11v1**（更严重） | ❌ **修正**（数值）+ ✅ 成立 |
| 9 | 约束③ `Runs()` 只切 1 run | 1 run | **6 run**（混排）；**1 run**（阿拉伯+拉丁） | ❌ **修正**（混排切分正常，阿拉伯才是缺口） |
| 10 | caret 按 rune 吸附（无簇） | 无簇支持 | **部分有**：`snapToGraphemeBoundary` 仅 UTF-8 续字节归位，且只在 1/3 路径调用 | ⚠️ **状态修正** |
| 11 | `Generation` 被广泛依赖 | 11 处引用 | **5 处递增、生产代码 0 处消费；但有 2 处测试/真窗消费**（`TestTextLayout_Generation`、`ui_wr_ime_r2_textlayout:307`） | ⚠️ **性质修正**：早期判为「零消费者、无兼容负担」**错误**，第三轮已推翻，见 §3.6 面 3 |
| 12 | 面1 `.Lines` 引用规模 | 139 / 47（另处写 29） | **139 / 47**（剔除 `render/` 无关同名 155 处） | ⚠️ **口径统一** |
| 13 | **§2.1 A 表口径**（新增维度） | 未标注 | **MultiFace 2.290 ms / singleFace 34.057 ms（慢 14.9×）**；机制：MultiFace 走 O(n) 兜底、singleFace 走 O(n²) 扫表（占其 98%） | ❌ **补口径**（不标会严重低估 M0 收益） |
| 14 | `InputBox.Sync` 5000 字 | 2.68 ms | **2.64 ms** | ✅ **高度吻合**（A 表最准的数） |
| 15 | profile：`buildCaretsForLine` cum | 86.44% | **87.93%** | ✅ 成立 |
| 16 | profile：`RuneAdvance` 链 cum | 76.27% | **72.41%** | ✅ 成立 |
| 17 | `duffcopy` 来源 | undo 全量快照 | **量宽链**（`RuneAdvance`/`glyphForRune`）；只跑 Editor 不触排版时 duffcopy **恒为 0** | ❌ **归因错误，证据撤回** |
| 18 | Q3：1e5 / 1e6 拷贝 | ~10 µs / ~100 µs | **~70 µs / ~240 µs**（偏快 2 倍+） | ❌ **修正**（但决策保住：240µs 仅占排版 10%，未到 30% 触发线） |
| 19 | `ellipsizeToWidth` 复杂度 | O(n log n) | **O(n)**（`ell/一次Measure` 恒 8.0–8.7× 不随 n 增长） | ❌ **修正**（M5 收益比原写更大） |
| 20 | 「5000 字已卡」 | 定性"卡" | 2.64 ms = 一帧 16.7 ms 的 **15.8%**，**不足以支撑"卡"**；20000 字 68%、50000 字 **174%** | ⚠️ **论据强度校准** |
| 21 | undo 内存：100k 字 | 10 MB | 纯 ASCII 9.54 MB ✅；中英混排 **18.5 MB** | ⚠️ **补口径**（结论不变且偏保守） |
| 22 | §3.2 契约表 `LineTop`/`GetOffsetForCaret`/`GetPositionForOffset` 标 O(n) | O(n) | ×10 行数 → 耗时 **×9.5~12.2**，标准线性 | ✅ **标注准确** |
| 23 | M6 阈值 `keystroke_p99≤16` / `T(C)/T(B)≤1.5` | 阈值合理性 | 当前 29.09 ms / 11.04（O(n) 未优化的必然），**G1 达标后自动满足** | ✅ **合理可达，照判不误判** |
| 24 | `interval_p95`/`fps_interval`/`hitch_rate`/`rss_slope`/`caret_vs_paint_max_delta_px` | 5 项阈值 | **未验证**——需 `ui_text_edit_accept` 窗（尚未建）+ GPU 环境 | ⚠️ **未用估算冒充实测** |

**复核总结论**：方案主干（四个面的认定、`M0→M1→M2→M3(/M3.5)→M4→M5` 分期、G1–G8 硬目标、Q1–Q4 决策）**全部成立，无一条被推翻**。被修正的是**数值精度**（5 项）、**若干行号**（4 处）、**能力现状**（3 项）与**证据归因**（1 项）；**新增发现 3 处缺口**（script/bidi 分段、`MaxLines` 布局侧截断、面2 第三处 O(n²)）。

> **📌 唯一由数值复核"反向印证"的施工风险（最重要的一条）**：第 13 项实测——**`singleFace`（即 M0 修通后的成功路径）5000 字已达 34 ms，是 `MultiFace`（当前路径）2.29 ms 的 14.9 倍**。这意味着 M0 修通 `MultiFace` 整形后，代码会从 O(n) 兜底切到 O(n²) 成功路径，**造成一次真实的 15 倍性能回退**。
> **这不是推测，是实测**。因此 M0 第 8 项（通整形）与 `CaretXForCluster` 扫表消除**必须同时做**，且 `BenchmarkCaretBuild` 熔断**必须设**。详见 M0 第 ⑤ 项与 ⑦ 熔断。

### 11.0.1 可开工性校验（新会话视角 · 2026-09-02，第三轮由 5 个 agent 并行重跑）

> 校验问题：**一个没有任何上下文的新会话，只读当期条目，能不能直接开工？**

**第三轮（5 agent 并行）结论：可以开工，但必须先看三处新增的"要点"提示**——M0 第 8 项、第 9–10 项、M1 第 11 项，这三处照字面做都会做错或卡死，已各自加了实现路径说明。

| 校验项 | 结果 |
|---|---|
| **各期九项规范齐全** | ✅ 机器核验：M0/M1/M2/M3/M3.5/M4/M5 共 **7 期，全部 9/9 齐全** |
| **各期改动清单精确到文件:行号** | ✅ 无「重构排版层」类模糊描述 |
| **各期熔断/回滚本期完整重写** | ✅ 无「见上期」 |
| **本期要用的真窗是否存在** | ⚠️ **发现阻塞**：`ui_text_edit_accept` 被 M0–M5 每期引用但**不存在** → 已补 **M0-pre** 前置任务 + §6.3.1 生命周期表 |
| **改动清单是否可执行（不隐藏坑）** | ⚠️ **发现坑**：M0 第 1 项「从 `TextLayout` 派生」隐含 `TextLayout` 缺 3 项能力 → 已补「面 4 落地要点」+ 三方案选型 |
| **红灯起步条件** | ✅ M0 要求的 8 个新测试/基准**均不存在**，符合「先写失败测试」 |
| **基线可跑** | ✅ `go test ./ui/rendering/ -count=1` 绿灯；`ui/`+`render/`+`examples/` 编译通过 |

### 11.1 文档自洽性校验（复核后重跑）

| 校验项 | 结果 |
|---|---|
| 章节顺序（§0→§11） | ✅ 连续无跳号、无重复 |
| 分期条目九项规范 | ✅ M0/M1/M2/M3/M3.5/M4/M5 共 **7 期**齐全（7 个「①本期目标」+ 7 个「⑨交接说明」） |
| 引用的代码行号 | ✅ **复核后全部校准**。已修正：`draw.go:452`→435、`text.go:908`→855(Paint)、`text.go:946`→945(cullGlyphs)、`input_router.go:424`→413、`text_layout.go` 区间 `241-258`→241-260 / `396-450`→396-453 / `501`→500 / `599`→597。核对无误：`multi.go:164/355`、`shaped.go:125`、`editor.go:144`、`text.go:603/665/740`、`text_layout.go:138/154`、`paragraph.go:255` |
| 决策一致性 | ✅ Q1–Q4 在 §3.5 / §9.1 / 各期条目中表述一致，无残留「待拍板」 |
| 阶段编号一致性 | ✅ M3.5 条件触发、M6 非施工期，在 §0/§3.2/§4/§5/§7.3 中表述统一 |
| **四个面覆盖** | ✅ 面1(M1-d) / 面2(M1-a + 第13项) / 面3(M1 第10项) / 面4(M0 第1–3项) **全部纳入分期** |
| **数值口径统一** | ✅ `.Lines` 全文档统一为 139/47（已剔除 `render/` 下无关同名 155 处）；发散/量宽/单源/复杂度均统一为复核值 |
| 代码改动 | ✅ **零改动**（本轮仅文档；`git status` 仅本文档） |
| 探针清理 | ✅ 全部临时探针已删（`zz_verify/` 包已移除，仓库无残留） |

### 11.2 补测数据存档（2026-09-02，临时探针已删）

**第一轮（立项/重审阶段）**

| 探针 | 结论 | 落入 |
|---|---|---|
| 绘制 vs 布局位置发散 | 5000 字偏 **173px**（逐字 `snapPen` 取整累加 vs 浮点累加） | §3.5 约束①（**复核修正为 976.6px**） |
| 回绕下编辑级联作废 | 改首行后 L1 由 `[35,71)` 变 `[30,62)` | §3.5 约束② |
| `MultiFace.Runs()` 能力 | 已存在；"Hello世界abc你好" 只切 1 run | §3.5 约束③（**复核修正为 6 run**） |
| 各 API 复杂度（2 万行） | `CaretForOffset` **374.8ms**、`BoxesForRange` **365.3ms**；其余 O(n) | §3.6 面2 |
| 单源一致性 | MaxLines 2v4、无 Face 10v1 **不一致**；回绕/多 run 一致 | §3.6 面4（**复核修正为 2v12 / 11v1**） |

**第二轮（实证复核阶段 · 原始输出）**

```
# MultiFace 整形通路（DejaVuSans 16pt）
MultiFace.Source()==nil ? true
Shape(MultiFace) glyphs = 0          ← 根因① 确认
Shape(singleFace) glyphs = 12
MultiFace.Runs("Hello世界abc你好") = 6
  run0 "Hello"  run1 "世"  run2 "界"  run3 "abc"  run4 "你"  run5 "好"
MultiFace.Runs("مرحبا بالعالم hello") = 1   ← script/bidi 分段缺口
Shape(arabic) glyphs = 0

# 单字量宽（1000 字混排 "a世界b"×200，稳态）
RuneAdvance[singleFace]: 427.0 ns/call
RuneAdvance[MultiFace]:  448.6 ns/call
MultiFace.Advance(1000 字整串): 0.2746 ms

# 绘制/布局发散（"a"×N，DejaVuSans 16pt）
n=  100  layoutEnd=  980.469  snapPen= 1000.000  delta= 19.531  perChar=0.1953
n= 1000  layoutEnd= 9804.688  snapPen=10000.000  delta=195.312  perChar=0.1953
n= 5000  layoutEnd=49023.438  snapPen=50000.000  delta=976.562  perChar=0.1953

# 复杂度（每行 ≈36 字节，10 次调用均值）
CaretForOffset n=  201  0.0306 ms   lines=201
CaretForOffset n= 2001  3.1425 ms   lines=2001
CaretForOffset n=20001  365.9292 ms lines=20001
BoxesForRange  n=  201  0.0728 ms
BoxesForRange  n= 2001  3.7848 ms
BoxesForRange  n=20001  377.6219 ms

# 回绕级联（MaxWidth=300）
BEFORE L0[0,35) "The quick brown fox jumps over the "
       L1[35,71) "lazy dog and then keeps running far "
       L2[71,75) "away"
AFTER  L0[0,30) "INSERTED_WORD The quick brown "
       L1[30,62) "fox jumps over the lazy dog and "
       L2[62,89) "then keeps running far away"

# 不回绕行隔离
full L1 width=170.593750  solo width=170.593750  delta=0.000000

# 单源一致性（147 字符长句）
[MaxLines=2+Ellipsis] DisplayLines=2   TextLayout.Lines=12   *** MISMATCH ***
[NoFace estimate]     DisplayLines=11  TextLayout.Lines=1    *** MISMATCH ***
[Wrap MaxWidth=300]   DisplayLines=4   TextLayout.Lines=4    content match OK
[NoWrap single]       DisplayLines=1   TextLayout.Lines=1    content match OK
```

## M3 合入记录（2026-09-04 · 代码合入，老裁判 1 红另立案，M3 不标完成）

- 合入内容：`ui/textinput/editor.go` 增量 undo（`pushDelta` 存量做 `strings.Clone` 不钉原文数组、历史上限 100 组带槽位清零、空组合不占步数）+ `editor_undo_m3_test.go` 新增边界/零文档引用/耗时比值单测 + 新文件 `editor_retention_paths_m3_test.go`（DeleteSurrounding/覆盖式输入/IME 三条路径不断言漏网）+ 验收窗 README 补 M3 补丁与 60 秒稳态复测记录。
- M3 自身证据（本机实测）：`TestUndoDelta_*` 7 个全绿、`TestPushHistory_NoQuadratic_M3` 比值 1.00、`BenchmarkPushHistory_M3` 多轮约 1.0–1.3（门禁 ≤2）、`go test ./ui/textinput` 整包绿。
- 未消红灯（另立案跟，不归 M3）：13 个老裁判中 `TestTextLayout_LongBuild_RealFace` 红——5000 字真字体排版门禁 100ms，本机（i5-6200U）两次实测约 289ms / 225ms。M3 未碰排版层（改动文件清单无 `ui/rendering`），系慢机器跑不动门禁线，老问题在 M3 期暴露。
- 待补诚实项：M3.5 触发数据现用“拼接 vs 空字体排版”口径并带“估算”字样，需换 `BenchmarkKeystroke` 口径重测；60 秒稳态复测只有结论缺原始输出附件。

## M3.5 合入记录（2026-09-04 · 代码合入，触发条件1未满足按任务指派执行）

- 触发诚实项（2026-09-04 复测，`BenchmarkKeystroke` 口径）：条件1（拷贝占击键 >30%）旧结论“未满足（2.7%）”的分母用错了——27ms 是冷启动整篇 `BuildTextLayout`，M1 之后生产击键走增量 `updateSpan` 不付这笔钱。同机 warm-incremental 口径重测：编辑侧 `BenchmarkEditorSpliceShare_M3/splice_100000`（`AddText` 含拼接+`utf16Len`/定位全串扫描+undo+镜像，5 轮 719–779µs）vs 排版侧 `BenchmarkKeystroke/NoWrap/N=100000`（1.0–1.6µs）——编辑器串工作约占 warm 击键 CPU 的 99.8%，条件1按此口径已远超触发线。诚实边界：M3.5 镜像不降低该占比（`Editor` 仍全串拼接+逐键全串扫描；镜像暂无生产读者，是 M4 脚手架）；镜像单键成本约 2–5µs（`BenchmarkInsertAtOffset` 1e5/1e6 档）+ 大文档约双倍内存。旧 2.7% 口径（冷排版分母）与纯拼接探针（/tmp，已清理）数字保留备查，不再作为触发判据。
- 修补（2026-09-04 · M3.5 挑刺修复）：① 降级开关虚标——`bufForceString` 此前只有测试能直接改字段，生产无入口；新增 `Editor.SetBufferFallback(bool)`（`editor_buffer.go`，对活跃镜像立即生效，双向各一次 O(n) 转换，小文档无影响）+ `TestEditorBufferFallback_M35` 锁定。② `ForceString(false)` 解钉后大文档永久停在直串模式（`normalize` 只降级不升级）：改为解钉即按尺寸重建树 + `TestForceStringUnpinRestoresTree` 锁定（先红后绿）。③ `BufferRestore` 注释补成本诚实项：`Editor` 仍持有直串，树快照恢复需一次 O(n) 物化+镜像重建，O(1) 仅对 `Buffer.Snapshot`/`Buffer.Restore` 成立。
- 合入内容：新建 `ui/textbuffer` 包（持久化隐式 treap：编辑 O(log n)、快照 O(1；<64KB 直串 + 32KB 滞后回落；`ForceString` 降级开关）+ 新文件 `ui/textinput/editor_buffer.go`（镜像持有、阈值激活、O(1) 快照；恢复需一次物化，见修补③；`SetBufferFallback` 生产降级入口，见修补①）+ `editor.go` 9 处一位接线（8 个变更点 + 3 个字段）+ `delta.go` 1 处（`ApplyDelta` 整串替换后重建镜像）+ 新测试 `ui/textbuffer/buffer_test.go`、`bench_test.go`、`ui/textinput/editor_buffer_m35_test.go`。
- M3.5 自身证据（本机实测）：`TestPieceTree_RandomAgainstString`（10k 随机对拍 × auto/forced-tree 双模式）绿、`TestBufferLineAt_*`（空/边界/CRLF 跨块/越界钳制）绿、`TestInsertComplexityRatio` 比值 T(1e6)/T(1e4)=1.90（门禁 ≤3）、`BenchmarkInsertAtOffset` 2266/3951/5663 ns（1e4/1e5/1e6）、`BenchmarkSnapshot` 约 1ns（指针拷贝）、`TestEditorBufferMirror_M35`（2000 次公开 API 随机编辑镜像逐字节一致 × 正常/降级双模式）绿；`ui/textinput` 逐文件全绿、`ui/textbuffer` 整包绿；回滚演练（移走新文件 + 恢复两处旧文件，构建与 M3 烟囱测试绿）通过。
- 未验证项：M3.5 验收清单本身无真窗/GPU 项；`ui_text_edit_accept` 无头探针已跑（`GPUI_ACCEPT_SELFTEST=1`：`keystroke_p99_ms` 3.868、`layout_ms_p99` 5.88、`caret_vs_paint_max_delta_px` 821.47，同 M0 基线量级，M3.5 未动排版/绘制符合预期；`paint_ms_p99`/`scroll_fps` 需交互真窗，工具自报 unmeasured），60 秒真窗空转无崩溃。

## M3.8 完成记录（2026-09-04 · 验收通过，用户已标完成）

- 根因（画像+探针实锤）：`TestTextLayout_LongBuild_RealFace` 计时的 280ms 里约 235ms 是一次性 HbFont 构造（字体字节解析 + CFF 表），热态整形仅约 0.02ms/次；缓存机制本身正常，错的是构造时机（第一次排版时才懒构造，账记进了排版）。
- 改动：`render/text/system_font.go` 加 28 行 `warmHbFonts`（`assembleFaces` 返回前对各源预建 HbFont，失败忽略则 Shape 照旧懒建）+ 白盒测试 `render/text/hb_warm_m38_test.go`（先红后绿锁定）。不改接口、不改行为、不改裁判。
- 验收证据（本机实测）：老测试连续 3 次绿；同条件探针排版耗时 11.3 / 9.7 / 8.2ms（门禁 100ms）；13 个老裁判全绿。
- M0–M3.5 回归（单测）：`render/text` 整形/簇边界/版式/分段/缓存/加载组全绿、`ui/rendering` 整包两遍绿、`ui/textinput` 整包绿、`ui/textbuffer` 整包绿；验收窗无头探针无崩溃（`keystroke_p99_ms` 7.1、`layout_ms_p99` 7.06，`caret` 差值 821.47 同基线量级）。
- M0–M3.5 回归（基准，本轮补跑）：`BenchmarkCaretBuild` 每字 450/499/618ns（比值约 1.37，门禁 1.5）；`BenchmarkKeystroke` 不回绕三档约 0.7–0.9µs 基本不随规模涨；`BenchmarkPushHistory_M3` 三轮比值约 1.0–1.1（门禁 ≤2）；`BenchmarkInsertAtOffset` 中位数约 2.3/2.9/5.4µs（比值约 2.4，门禁 ≤3）；`BenchmarkSnapshot` 约 1ns（指针拷贝）。
- 诚实项：① `TestCaretBuild_NoQuadratic` 在整包+并行压测下误报红过一次（CPU 被我自己的并行任务占满），单跑三遍全绿，定性为测量方法问题；计时类回归以后一次只跑一个。② `render/text` 约 90 个微调/光栅/扫描类测试文件未逐个跑（改动碰不到这些路径）。③ `BenchmarkInsertAtOffset` 单轮样本抖动大（最坏单轮比值约 6.5），中位数与 `TestInsertComplexityRatio` 均达标，机器抖动已注明。
- 完成确认：用户真窗人验（验收主窗 60 秒 + 五扇输入法窗逐扇操作）无明显问题 → M3.8 标完成；M4 前置（M1 缓存 + M3.8 完成）已齐，M4 可开工。

## G1 重确认记录（2026-11-13 · 门禁口径复核，未改代码）

前置：按 `ENGINE_TEXT_SCALE_PLAN.md` §9 G1 行的「重确认要求（硬）」执行——重跑门禁 + 差分 + 画像。探针全部放 `/tmp`，经 `go test -overlay` 注入，仓库测试文件未改。

### 1. 门禁现状（原样跑）

`go test ./ui/rendering -run '^TestKeystrokeRatio_M5$' -count=1 -v` 共 4 次单跑（失败即止，故只记录到首个红模式）：

| 跑次 | NoWrap 比值 | WrapShort 比值 | 结论 |
|---|---|---|---|
| 1 | 1.33 | 1.90 | FAIL（WrapShort） |
| 2 | 1.93 | 未跑到 | FAIL（NoWrap） |
| 3 | 2.68 | 未跑到 | FAIL（NoWrap） |
| 4 | 1.86 | 未跑到 | FAIL（NoWrap） |

### 2. 同场景加预热（复刻门禁：5 轮中位数 × 每轮 50 次，另加 200 次预热后计时）

| 模式 | base | 1e6 | 比值 |
|---|---|---|---|
| NoWrap | 901ns | 607ns | 0.67 |
| WrapShort | 569ns | 625ns | 1.10 |
| WrapLong(5k 基准) | 44.55µs | 24.48µs | 0.55 |

专用基准 `WrapShort@1e6` 连续 2 万次 = **845 ns/op**；`BenchmarkKeystroke@1e5` 同模式 943 ns/op。**等长替换路径不随文档长度增长**，门禁红来自 1e6 档冷启动/GC 被算进 50 次均值（量具缺陷）。2026-09-16 记的「回绕是真超线性」作废。

### 3. 真实打字（插入/删除，delta=±1）差分（2000 次均值）

| 模式 | base | 1e6 | 比值 |
|---|---|---|---|
| NoWrap | 1.05µs | 286.7µs | 254.9 |
| WrapShort | 0.89µs | 303.5µs | 341.4 |
| WrapLong(5k 基准) | 40.07µs | 315.5µs | 7.9 |

正确性口径：1e6 下 200 次插/删 `spansHit+=200、spansMiss+=0`，无全量回退。

### 4. 画像（1e6@WrapShort 插入，3000 次，`-cpuprofile`）

- `patchRows` flat 66.7%（cum 86.7%）：热点 `layout_update.go:331-334` 逐行平移 `StartByte/EndByte`，`:333` 单行 480ms/1050ms。
- `patchIndex` flat 20.0%：热点 `layout_update.go:396-400` 索引 `starts/ends/tops` 逐行平移。
- `partLines` cum 仅 4.8%；门禁测试自身的 profile（503ms）主要被 5 次 1e6 全量首建占据，不能用于击键归因。
- 结论：O(rows) 成本 = 行/索引的**绝对偏移逐行改**（约 11ns/row × 2.7 万行），与整形/回绕无直接关系。

### 5. 诚实项

- 未实测真字体（`face!=nil`）插入路径；tail 平移与 face 无关，但没跑。
- 未测 1e6 以上；`BenchmarkKeystroke` 本身仍只到 1e5。
- 修法（后缀惰性平移，Fenwick/分段增量）与门禁补插入臂均未动手，按计划「另排期」待拍板；阈值 1.5 未动。
- 探针文件（`/tmp/g1_probe*.go`、`/tmp/g1_*.out`、overlay json）仅本地，仓库无新增临时 `.go`。

## G1 门禁补插入/删除臂（2026-11-13 · 用户批准，阈值 1.5 未动）

前置：2026-11-13 复核确认旧门禁只测等长替换（delta=0，走 `patchRows` 的 `same` 快径），真实打字的插入/删除（delta=±1）未进裁判；插入路径实测 1e6 为 O(rows)，旧门禁看不见。

### 改动

- 文件：`ui/rendering/m5_keystroke_g1_test.go`（唯一改动，生产代码零改动）。
- 两臂：`等长替换`（原口径）与 `插入删除`（真实打字：插一个字符再删回）。
- 量具：每臂预热 200 次 → `runtime.GC()` 压回堆基线 → 计时 200 次；基线与 1e6 交替执行、每轮成对取比值，5 轮比值取中位数（门禁 ≤1.5 不变）。
- 失败改为 `t.Errorf` 逐臂报告，保证三模式两臂全部测完再判红。

### 3 连跑证据（本机，当前树）

| 跑次 | 等长 NoWrap | 等长 WrapShort | 等长 WrapLong | 插入 NoWrap | 插入 WrapShort | 插入 WrapLong |
|---|---|---|---|---|---|---|
| 1 | 1.02 | 0.93 | 0.80 | 300.9 | 209.6 | 8.55 |
| 2 | 0.60 | 0.98 | 1.22 | 314.9 | 238.1 | 8.49 |
| 3 | 1.05 | 0.90 | 0.78 | 289.1 | 284.5 | 8.21 |

- 等长臂全绿（0.60–1.22），旧门禁的 1e6 冷启动抖动已消除。
- 插入臂稳定红（NoWrap/WrapShort ×210–315，WrapLong ×8.2–8.6），与画像热点 `layout_update.go:331-334`（逐行偏移改写）+ `:396-400`（索引逐行平移）一致。
- `TestKeystrokeRatio_M1`（N 到 1e5、阈值 5）同时复跑 PASS，未受共用 helper 影响。

### 诚实项

- 红灯是**预期**（红灯起步）：修复（行/索引偏移后缀惰性平移）未开工，修复落地前该测试保持红。
- 插入臂仍用 nil face（门禁原配置）；真字体插入路径未实测。
- `runtime.GC()` 只作用于测量前，不改变生产行为。

## G1 修复记录（2026-11-13 · 按 Flutter 版面语义重构，用户批准）

### 标准对齐依据（本机 Flutter 源码）

- `TextPainter.markNeedsLayout()` 直接丢弃整块版面缓存（`packages/flutter/lib/src/painting/text_painter.dart:781-789`），`layout()` 重建段落并调用 `paragraph.layout(constraints)`（同文件 `:1218-1271`）。
- 行度量由段落派生：`computeLineMetrics()` 从 `layout.lineMetrics` 取（`:1794-1808`），`lineMetrics` getter 缓存整份派生结果（`:524`）。
- 结论：版面结果不可变、行度量按需派生，**不逐行回写偏移**。本仓库此前的偏离点是：把各硬段缓存摊平成全局绝对偏移行表，并在每次长度变化编辑时逐行改写 tail 行与索引。

### 改动（生产代码，`ui/rendering`）

- 新增 `line_shift.go`：`rowShift`（Fenwick 差分树）记录“从某行起整体平移”的后缀增量；行内只存 base 偏移，绝对偏移 = base + `at(i)`。
- `layout_update.go`：行数不变的编辑只替换本段行内容并把新行转回 base，随后 `addSuffix(hi+1, delta)` 记一笔增量；删除对 tail 行的逐行 `StartByte/EndByte += delta`（原 `:331-334` 热点）与 tail 索引逐行平移（原 `:396-400`）。行数变化或替换区间含旧增量事件时走 `compactOffsets()` 烘焙（O(rows)，结构性编辑）。
- `layout_update.go`：新增「上次编辑行」O(1) 快路（击键局部性），命中时免两次 O(log n) 二分。
- `text_layout.go`：`TextLayout` 增 `shift`，`Line`、`LineCarets`、`CaretAt`、`LineClusterStarts`、`BoxesForRange`、`CaretForOffset`、`GetOffsetForCaret`、`GetPositionForOffset` 统一走 `lineStart/lineEnd` 派生；行内簇缓存改存相对偏移（行基址平移后仍有效）。
- `line_index.go`：`rowForOffset/rowRangeForSpan` 增加 shift 参数；无增量的全量构建传 nil，零开销。

### 证据

- 插入/删除臂（`TestKeystrokeRatio_M5`，阈值 1.5 未动）：
  - 1e6 单次击键：修复前 287–315µs → 修复后 **0.72–1.20µs**。
  - 量具收紧：每轮样本 200→1000、轮数 5→7（原配置在 1e6 档偶发 1.52）。收紧后 **6 连跑全绿**，可见比值：插入臂 NoWrap 0.72/WrapShort 1.02/WrapLong 0.77；等长臂 NoWrap 1.16/WrapShort 0.90/WrapLong 0.69（阈值 1.5 未动）。
- 规模阶梯（NoWrap 插入 2000 次均值）：修复前 0.8µs→1.98µs→16.1µs→268.8µs（1k→1e6）；修复后门禁实测 1e6 不再随行数增长。
- 新增 `m5_line_shift_test.go`：
  - `TestLineShift_EquivQueries_M5`：三种宽度、8 步编辑，每步与 `BuildTextLayout` 逐行对照 `Line/簇起点/CaretForOffset/GetOffsetForCaret/BoxesForRange/GetPositionForOffset`。
  - `TestLineShift_Compaction_M5`：100 行文档覆盖行内插字（增量路径）、删换行触发烘焙、插换行/跨行删（行数变化）、前缀插入，逐步对照全量构建。
- 回归：`TestTextLayout_*` 全部 PASS；`TestR1_*` 全部 PASS；`ui/rendering` 整包、`ui/textinput` 整包、`ui/textbuffer` 整包 PASS；`TestKeystrokeRatio_M1` PASS。

### 诚实项

- 行数变化编辑（回车/跨行删）仍 O(rows)（烘焙约 0.3ms@1e6），绝对可接受但不是 O(log n)；本次门禁不覆盖该路径。
- 真字体（face!=nil）插入路径未单独测量；本次测量沿用门禁的 nil-face 估算配置。
- `-race` 未跑：仓库禁 CGO，`go test -race` 直接拒绝。
- accept 真窗本轮未复跑；改动集中在布局增量层，13 裁判 + 三个整包 + 查询口等价性已覆盖。

## 行数变化编辑（回车/跨行删）收口（2026-11-13 · 同按 Flutter 口径）

### 标准对齐

- Flutter 对任意文本变化整段重建版面（`markNeedsLayout` → `layout()` → `paragraph.layout`，text_painter.dart:781-789、1218-1271），**没有** rope/间隙式增量行表。
- 因此本项不引入 Flutter 没有的结构，保持 O(n) 整段重建口径，只把常量压小、并用绝对上限守门。

### 改动

- `line_shift.go`：`rowShift` 增 `diff` 事件数组；新增 `drain()`（O(n) 前缀烘焙）、`reshape()`（把旧事件映射到新行数：删除区间、插入 f 行、tail 整体 +delta）、`rebuild()`（O(n) 重建 Fenwick）。
- `layout_update.go` 行数变化路径：删除 `make + 两次 append` 整表拼装与 tail 逐行平移，改为**就地 splice**（单次 memmove，删除时 `clear` 尾部引用）；偏移交给 `reshape`；`patchIndex` 改为就地插入/删除 starts/ends/tops，只有行高变化才平移 tail 行顶。
- `compactOffsets` 改用 `drain()` 前缀烘焙，只作为“替换区含旧事件”的兜底。
- 测试：`TestRowShift_Reshape` 三个字面用例锁映射；`TestRowCountEditBound_M5` 绝对上限门禁；`TestPatchIndex_InPlace_RV` 的“行数变化必须换索引对象”改为值一致断言（就地插入是本次有意口径）。

### 证据

- 行数变化（插入换行再删回，2000 次均值）：1e3 12.6µs→2.6µs；1e4 34.1µs→16.0µs；1e5 405µs→70.9µs；1e6 **3.32ms→1.08ms**（Wrap 模式 3.15ms→1.07ms）。
- 门禁：`TestRowCountEditBound_M5` 五轮中位数 1e6 **0.98–1.10ms**（上限 2ms）、1e5 **59–112µs**（上限 500µs），PASS。
- 字符编辑门禁未退化：`TestKeystrokeRatio_M5` 3 连跑全绿。
- 等价性：`TestLineShift_Compaction_M5`（含删换行触发事件边界）、`TestLayoutUpdate_*`、`TestLayoutCache_Equiv_M1` 全绿；13 老裁判、`ui/rendering`/`ui/textinput`/`ui/textbuffer` 整包全绿。

### 诚实项

- 仍是 O(n)：1e6 约 1ms，剩余 54% 是行表尾部 memmove（含 GC 写屏障）；要再降需要 Flutter 没有的块/间隙行表，本轮不做。
- 1e5 整包跑时 `TestCaretBuild_NoQuadratic` 曾误报一次（已知负载敏感，单跑 PASS；与本次改动路径无关）。
- accept 真窗未复跑；真字体行数变化路径未单独测。

## V3c：Box 放开回绕 + 滚动锚点（2026-11-13）

### 标准对齐

- 立项书 `docs/ENGINE_TEXT_LARGE_SCALE_PLAN.md` §4 V3 行：V3c = 「Box 放开 wrap + 锚点」，验收「锚点不跳变」；§1.4 成熟方案（CodeMirror `HeightMap` 编辑只重建变化区间、VS Code 视口锚点）。
- §2.2 硬约束「击键路径不得物化整串」；`docs/ENGINE_TEXT_SESSION_BRIEF.md` §1「红灯起步：先写失败单测确认红，再改代码」。
- 接手时 Qoder 会话留下未提交半成品（编译绿、无测试、被登录过期打断）。按纪律先存补丁回 V3b 基线，不带着半成品继续。

### 改动

- `ui/rendering/text_viewport.go`：新增 `AnchorAt(y)`（用当前高度图取顶部行 + 行内偏移，不做源行数同步）与 `InvalidateLayouts()`（丢布局缓存、保留实测高度）。
- `ui/textinput/box.go`：
  - `canVirtual` 去掉 `b.wrap` 拦截；新增 `wrapWidth()`，`ensureVirtual` 改用 `NewWrappedTextViewport` 并在 wrap 宽度变化时重建共享视图；`Layout` 的 wrap 分支按 `vtext.MaxWidth()` 判断变化。
  - 新增 `rebaseVirtual`：同数编辑只 `InvalidateLayouts`；行数变化编辑先 `AnchorAt` 取锚点，按 `ConsumeEditSpan` 的编辑行平移锚点，再用新高度图 `LineTop(anchorLine)+anchorOff` 重定 `scrollY`。
  - `virtualVertical`（视觉行上下键 + sticky column）接到 `OnKey` 的 ArrowUp/Down 虚拟分支与 `extendVertical` 虚拟分支；选区映射改用 `Buffer.UTF16OffsetAt`，不再走 `extentForBytes`/`extendExtentBytes`（内部 `Editor.Text()` 会物化整串）。
  - 修正 `TextRange().Affinity`（返回全文档范围）为 `SelectionRange().Affinity`；清理 `virtualVertical` 中重复的 `if extend` 赋值。
  - 虚拟 `syncHighlightVirtual`/`imeRectVirtual` 改用 `Prepare` 而非 `VisibleRange`，保证回绕行几何与同帧绘制一致；`syncVirtual` 的 `maxW`/`totalH` 计算顺序对齐回绕语义。
- `ui/textinput/box_v3c_test.go`（新增 4 条）：wrap 进虚拟且不物化整串、按宽度重建视图、回车后锚点不跳、上下键/Shift 扩选按视觉行且留在同一逻辑行。

### 证据

- 红灯（V3b 基线 `b6f1f2c9`）：4 条 `TestBoxV3c_*` 全 FAIL，失败原因均为 `virtualOn=false`（wrap 大文档进不了虚拟模式）。
- 转绿（应用实现后）：4 条全 PASS；其中 `TestBoxV3c_VisualRowArrowKeys` 在只应用半成品、未接线时精确红：`ArrowDown jumped to logical line 801 (want 800)`。
- 反证（防假绿）：临时废掉 `rebaseVirtual` 的锚点重定，`TestBoxV3c_AnchorStableAcrossLineCountEdit` 精确红 `top line 799 -> 774 after Enter at line 800 (scrollY 16796.0 -> 16262.0)`；恢复后绿。
- 回归：`ui/rendering`/`ui/textinput`/`ui/textbuffer` 共 119 个 `*_test.go` 按文件逐个跑，119 PASS / 0 FAIL；`go build ./...`、`go vet ./ui/textinput ./ui/rendering`、`gofmt -l` 全干净。

### 诚实项

- `rebaseVirtual` 借用消费式 `ConsumeEditSpan` 定位编辑行（`box.go` 与 `edit_scroll.go` 各有一处消费点）。虚拟分支不走 `syncBoxText`，当前无冲突；但这是一次性通道的语义借用，未复现到实际故障，本轮未改，已记入立项书 §8 诚实项。
- `wrapWidth()` 用 `FixedWidth - 2*Padding()`，未覆盖 DPR 缩放与 `MaxLines` 交互；GPU 真窗 wrap 场景另轮验收。
- 真字体（face!=nil）回绕上下键未单独测；4 条测试沿用 nil-face 估算排版口径。
- `-race` 未跑：仓库禁 CGO（沿用既有口径）。
- **真窗曾 FAIL、P1/P2 已修（2026-11-13，隔离 `:3`）**：`examples/ui_text_large_scale -wrap`（1e6 行、130MB、每行回绕成 2 视觉行）首轮 60s `fps 39.67` / `hitch 390.36` 双双红，`anchor_top_line_stable` 漂移 1 行；CPU 单测口径未覆盖「`Invalidate()` 丢实测高度 + `Prepare` 重测」。按 §1.4 CodeMirror 口径与 CPU profile 定位后修复（详见下节与立项书 §8）。修复后 60s 回绕窗口两轮 `fps 58.93/59.01`、`p95 20.29/20.26`、`hitch 2.00/4.00` 全达标；Golden 零容差比对 `gate_pass=true`、diff=0（763584 px）、像素探针 68960、5 条逻辑探针全绿。60s 不回绕对照 `p95 22.95–23.03` 略超 22（当前机器基线，batch 前同窗 26.75 更差），非 V3c 引入。
- 新增公开查询口 `Box.TopVisibleLine()`（虚拟视口顶部逻辑行号，替换「原始 `scrollY` 数值」这种错误锚点判据）与 `TextViewport.RebaseCount(editLine, delta)`（行数变化只重建变化区间、保留实测高度）。
- **人工二合一**：常驻窗已开、用户已看画面确认 OK（Golden 基线随之认可）；补跑 `-wrap -manual-seconds 20` 标准关闭窗口，exit 0、`evidence_mode=manual`、fps 58.58、正常出汇总 JSON。
- **人工窗关闭异常 + 未验证项（诚实）**：首次常驻窗挂 1h30m 后被外部 SIGKILL（exit 137），未走标准关闭、无汇总 JSON；`:3`/`:0` 显示均健康、无残留进程、dmesg 无 OOM/段错误，非脚本主动硬杀。已按纪律补跑标准关闭窗口。`OnEvent` 的 `EventKey` 收真键打日志/改标题分支未实测：隔离环境 rootless Xwayland 的窗口树对注入工具不可见（xdotool 未安装、python-xlib 遍历 root 得 0 children），代码路径已核（`EventKey`/`KeyCode`/`WindowController.SetTitle` 均存在），记为**未验证**，不标绿。

### P1/P2 修复记录（2026-11-13，红灯起步 + CPU profile）

- **P1 红灯** `TestBoxV3c_RebaseKeepsMeasuredHeights`：行数变化编辑后 `MeasuredCount 26→1`（`Invalidate()` 整张丢实测高度），修复后 5 条 V3c 测试全绿、119 个测试文件回归 0 FAIL。
- **P1 修法**：`TextViewport.RebaseCount` 用 `EachMeasured` 导出实测节点、按行号平移（编辑行回落估算），`rebaseVirtual` 不再 `Invalidate()`；编辑点改用光标行定位，不再借用消费式 `ConsumeEditSpan`（该通道归 `syncBoxText`）。真窗自检同时修正判据场景：`-auto-only` 强制静止视口、编辑点放在视口内（原先与滚动脚本并发且把光标扔到文档中部，`ensureVisible` 合法跟走造成假红）。
- **P2 定位**（`go tool pprof`，60s 回绕窗口）：`main.(*ticker).Tick` cum 68%；`Editor.MoveCursorDown` → `SetSelection→changed→OnChange→Box.sync→syncVirtual→TextViewport.Line→BuildTextLayout` cum 59%；滚动脚本每帧最多 4000 次移动 = 4000 次整行排版，回绕后每行 122 字符 / 2 视觉行使成本翻倍。
- **P2 修法**：示例滚动脚本用引擎已有的 `BeginBatchEdit/EndBatchEdit` 把整批光标移动合并为一次 `OnChange`（能力在引擎已存在，属示例用错 API，未在示例绕洞）。修后 60s 回绕两轮全达标。
- **证据文件**：`/tmp/v3c_wrap_cpu.prof`（CPU profile）、`/tmp/v3c_auto6.json`（5 探针全绿）、`/tmp/v3c_r4_gate.json`（Golden gate PASS）、`/tmp/v3c_r5_wrap60.json` 与 `/tmp/v3c_60s_batch.json`（60s 性能）、`examples/ui_text_large_scale/testdata/v3c_wrap_golden.png`（待人工确认的种子基线）。

---

### V4 阶段 1：分块物化（2026-11-13，按 D1 上限）

- **红灯**：`ui/rendering/v4_longline_test.go`（testdata 种子 + 循环索引、每击键改不同位置、生产 `SetTextSpan+TextLayout`）；两轮实测 1e6 字 724–784ms、5e6 字 3.95–4.08s（门禁 ≤16ms）。
- **成熟框架核查**：Skia `skparagraph`（整段 `ParagraphImpl::layout`，只有 `maxLines`/`ellipsis`；`Run.cpp:356 prefixWidth` 是省略号按宽度比例分配，非文本前缀宽度）、Flutter `text_painter.dart`（`maxLines`/`ellipsis`，无视口/分块）**均无超长行分块与横向视口原生能力**，按 D1/VS Code 口径自研，不手写库里已有的算法。
- **CPU profile**：`ui/rendering/layout_update.go:167 patchRows` cum 4.07s、`text_layout.go:662 buildShapedCarets` cum 5.98s、`memmove` flat 26%，`render/text.Shape` 仅 20.53% → 瓶颈是整行 caret/簇表全量重建，不是整形。
- **修法**：`RenderText` 加 D1 可配置上限（`DefaultMaxSingleLineBytes=10000`、`MaxSingleLineBytes` 字段 / `SetMaxSingleLineBytes` / `LongLineWindow` 公开），`ensureLayout` 只把窗口文本交给 `layoutCache`，全文留 `RenderText.Text` 供复制/全选；`TextLayout.LongLine` 记窗口/全文长度，越界偏移经既有 `clampOffset` 钳制。新增公开面：`LongLineInfo`、`TextLayout.LongLine`。
- **证据**：1e6 字**窗口内真实打字 3.98ms**（不是远端快路假绿）、5e6 字远端编辑 532µs；`TestV4_LongLineWindow` 锁全文保留/窗口内编辑真被布局/上限可配/-1 禁用/越界钳制不崩；`m5_singleline_test.go` 显式 `SetMaxSingleLineBytes(-1)` 保持其 50k 全量口径；全仓 `go build ./...` + vet + gofmt + 120 个测试文件逐个回归 0 FAIL。
- **诚实项**：阶段 2/3 未做——省略标记尚未绘制、窗口外光标钳到窗口末尾（横向滚动待阶段 3）、`longLineWindow` 每次 `strings.IndexByte` 只读扫全文（未物化但规模未完全解耦，待阶段 3 改用 Editor 行信息）、跨块 bidi/字素边界失败模式未探针、真窗三证据待阶段 3。**V4 整体不得标绿**。

---

### V4 阶段 2/3（部分收口，2026-11-13）

- **用户拍板**：块级 shape cache + 前缀宽度按冗余处理；继续做完含真窗。冗余依据：D1 窗口仅 10000 字节，滚动重整形一次 ~4ms（门禁 ≤16ms 已达标），独立块缓存无收益；横向位置按平均字宽估算（D1 远端估算），`render/text` 不新增底层公开 API。
- **横向视口**：`RenderText.SetLongLineWindow(start)` + `LongLineInfo.WindowStart`；`ensureLayout` 把行表偏移改为全文绝对、`l.Text` 保留全文，所有查询口继续吃/吐全文偏移。红灯 `TestV4_LongLineHorizontalViewport` 的 caret **位置断言**先逼出假绿 `xIn=xEnd=71269.6`（只查 `ok` 会漏），修后转绿。`Box.sync` 光标跟随滑窗；`canVirtual` 加 `LineCount()>1`，超长单行不再被 V2 `TextViewport` 绕过 D1 窗口（`TestV4_LongLineStaysOnRenderTextWindow`）。
- **真窗**：新建 `examples/ui_text_v4_longline`（独立主能力窗）。`-auto-only` 三证据：3 逻辑探针全绿（d1_window / not_virtual / horizontal_viewport 窗口 0→2,495,000）、像素探针 3486、Golden 零容差 diff=0（763584 px）；`GPUI_ACCEPT_GOLDEN=1` 种子轮按契约 `seeded=true` 不算绿；60s 人工窗二合一（收真键打日志改标题、标准关闭）。15s 自测 fps 59.36–59.68 / p95 18.91–19.27 / hitch 4.01–8.03（短跑波动）。
- **公开 API 新增**：`RenderText.SetLongLineWindow`、`LongLineInfo.WindowStart`、`Box.LongLineWindow()`。
- **证据**：全仓 `go build ./...`、vet、gofmt、`scripts/apidoc`、`scripts/check_tasks_ledger.py` 全绿；121 个测试文件逐个回归 0 FAIL（含新增 `box_v4_test.go`、V4 四条）。
- **诚实项（未收口，V4 整体不得标绿）**：① 截断省略标记三种做法均失败——`paintTextLine` 单独 `DrawString` 被 GPU 批量字形延迟提交覆盖（分支探针确认执行）、窗口文本等长替换 + `displayText` 覆盖 `DisplayLines()` 后 GPU 回读快照仍无标记（composite face 的 `paintCompositeRuns` 绕过字符串级标记），按「搞不定如实回退、不交付无效改动」全部回退，需后续专项重构绘制路径；② 独立滚动条拖动未实现（现靠光标跟随滑窗）；③ `longLineWindow` 每次只读扫全文 O(n)；④ 跨块 bidi/字素边界失败模式未探针；⑤ 人工窗已跑（隔离 :3、60s 标准关闭，fps 59.67 / p95 19.22 / hitch 0.00 达标），但 `v4_manual_keys=0` 真键事件通道未实测、`close_req=false` 为超时关闭，人工画面确认待用户。


---

### V4 第四层重做：分块窗口 + 截断标记收口（2026-11-13）

- **背景**：用户审查判定阶段 1「固定 10000 字节窗口」偏离 §3 第 4 层（渲染上限≠窗口大小、缺字素分块/前缀宽度/可见窗口），拍板按立项书重做；上轮「块缓存冗余」结论（只在简化方案下成立）撤销。
- **A 段（重做第四层）**：`RenderCap`(10000 字符)/`ShapeCap`(20000 字符) 拆开；`ui/rendering/long_line.go` 新增 2–4KB 块模型（FNV 内容哈希块 shape cache 提到 `RenderText` 级、跨编辑保留并有 `TestV4_BlockShapeCacheSurvivesEdit` 钉住；块宽前缀和；`ByteAtX`/`XAtByte` 像素↔字节映射）；窗口按可见像素带宽 + 512B overscan、钳制在 RenderCap 内（真窗 `window=(0,631)`）。复用 `text.LayoutText(...).Width`，`render/text` 未新增底层公开 API。旧 API（`MaxSingleLineBytes`/`SetMaxSingleLineBytes`/`SetLongLineWindow`/`DefaultMaxSingleLineBytes`）按接口全量切换制一次删净。
- **B 段第四次（截断省略标记，本轮收口）**：前三次失败的根因用隔离探针逐个钉死——① 独立 `DrawString` 被 GPU 批量字形延迟提交覆盖；② `DisplayLines` 字符串改动被 `paintCompositeRuns` 读 `LineGlyphs` 绕过；③ 按**字节偏移**等长替换 `…` 落在多字节汉字中间，窗口文本变非法 UTF-8（`decorated[105:118]="56789…\x96界\xe6" validUTF8=false`），`emoji.Segment` 切分越界 panic，标记根本没合法进字形。rune 边界替换对照探针证明 `LineGlyphs` 可含 `…`（GID 2825）。**实现**对齐 Skia `TextLine::createEllipsis` / Flutter `TextOverflow.ellipsis`：不动窗口文本，在 `adaptLongLineLayout` 裁掉可见带右端之后的字形，把 `…` 用**行尾具体子 face**（`*text.sourceFace`；`*text.MultiFace` 不能直接 `DrawShapedGlyphs`）整形为**独立 `LineGlyphRun`** 挂到布局字形表，走原生 `DrawShapedGlyphs` 链。GPU 回读快照右端确认 `…` 可见。
- **横向滚动坐标系洞（本轮一并修）**：`hCullWindow` 用全文 `scrollX` 裁剪，而长行窗口 glyph X 是窗口相对（0 起），滚动后显示内容错位（隔离探针 `scrollX=5000` 时 `cull=(4800,6104)` 落在窗口文本 `[0,8203]` 内却对应错误字节段）。`adaptLongLineLayout` 按 `XAtByte(WindowStart)` 把 glyph/caret X 平移成全文绝对、清空懒算 clusters 缓存；`TestV4_HorizontalCullCoordinates` 钉 `first X=0.0`→`1334.2`。`row.Width` 保持 A 段原值（不缩小，避免钳死 `m.maxW` 鼠标边缘滚动上限）。
- **客观判据（三证据合证，不用 mean）**：3 条 CPU 红灯（`…` 字形 GID+可见带右端位置、窗口文本 UTF-8 合法、cull 坐标系）+ 1 条真窗并排像素差分全绿；`marker_ink_diff=16`（`…` 三点墨迹，右端窄条逐像素差分），反证把对照图换成有标记图→`0` 且门禁明确报 `marker pixel probe`；`GPUI_V4_NO_MARKER=1` 生成无标记对照图存 `testdata/v4_longline_no_marker.png`（同一份代码只切 `SetLongLineMarker` 开关，差分可完全归因于标记）。
- **真窗验收**：Golden 重种后零容差 `diff=0 / 763584px`（种子轮按契约不算绿）、1px 负向 `0.000131%` 红→还原 `diff=0` 绿、4 逻辑探针全绿（`d1_window`/`not_virtual`/`horizontal_viewport`/`marker_glyph MarkerX=8505.0`）、`ink=3439`；60s 人工窗 `fps 59.75 / p95 19.30 / hitch 0.00`、标准关闭 exit 0、`:3`/`:0` 健康、无残留、无异常日志。
- **公开 API 新增**：`RenderText.RenderCap`/`ShapeCap`/`DisableLongLineMarker`、`SetRenderCap`/`SetShapeCap`/`SetLongLineMarker`/`SetViewportHint`/`LongLineWindow()`；`LongLineInfo.MarkerX`；`Box.SetLongLineMarker()`/`LongLineMarkerX()`/`LongLineWindow()`。
- **回归**：121 个测试文件逐个 0 FAIL；`go build ./...`、vet、gofmt、`scripts/apidoc`、`scripts/check_tasks_ledger.py` 全绿。击键 `13.93ms@1e6 / 11.34ms@5e6`（≤16ms）。
- **诚实项（V4 整体仍不标绿）**：击键贴近 16ms 门禁、余量不足；`longLineWindow` 的 `strings.IndexByte` + 全串比较仍 O(n) 未消除；跨块 bidi/字素边界未探针（§2.3 非目标 line-level 回退）；§2.2 的 L1/L2/L3/S 四档全量门禁本轮未重跑（引用 V0–V3 既有证据）；独立滚动条拖动未实现（现靠光标跟随）；真键事件通道未实测（`keys=0`、`close_req=false` 为超时关闭）；60s 短窗 RSS 斜率判 inconclusive；人工画面确认待用户。余项已在立项书 §8 最新修订与 §8.1 平台覆盖表归位。

---

### V4 标绿前置 · CPU 侧全档门禁 + 跨块 bidi 探针（2026-11-13）

- **用户拍板**：V4 标绿前须补齐 §2.2 全档门禁与跨块 bidi 探针；本轮先做 CPU 侧与 bidi，四档真窗串行重跑下一轮。
- **新增 `ui/textinput/v4_gate_test.go`**（§2.2 准确口径，唯一行 + 每轮改不同位置 + 中位数 + 防假绿有效性断言）：
  - 1e6 行首屏 `17.1µs`（门禁 ≤100ms）——断言进入虚拟模式，避免测空 Box；
  - caret 移动/命中 `9.5µs`（≤4ms）——走生产 `Box.caretAnchor()`；初版误用 `TextLayout.CaretForOffset(全文偏移)` 假红（`lookup failed at 2000000`），定位为 V2 虚拟模式只物化可见行、全文偏移本不适用（生产路径 `box.go:817` 用 TextViewport 行内相对偏移），纠正测试口径而非改引擎；
  - 多行同步 `4.4µs`（≤4ms）——V0 报告的 `19.27ms` 红已由 V2 视口模型（不扫全表）修复；断言虚拟模式 + 行数稳定；
  - 3e6 行行数变化编辑 `36.7µs`（≤16ms）——断言 `Insert("\n")` 真 +1 行、`Undo` 真恢复。
- **新增 `ui/rendering/v4_bidi_test.go` + `testdata/v4_bidi_seed.txt`**：长行窗口下 RTL（阿拉伯语/希伯来语）+ LTR + CJK 混合探针，断言不崩（`emoji.Segment` 不再越界）、窗口文本合法 UTF-8、字形非空、`…` 标记存在、caret 可查。按 §2.3 只保证 line-level 回退可用，不承诺跨块视觉重排。
- **既有 CPU 门禁引用**：`TestHeightMap_Memory3M`（3M 行高度图 ≤3MB，合 §2.2「≤1KB/千行」）、`TestRowCountEditBound_M5`（1e6 行 ≤2ms）、`TestKeystrokeRatio_M5`（击键比值 ≤1.5）、`TestV4_SingleLineKeystroke_*`（≤16ms）——均随 123 文件逐个回归全绿。
- **回归**：123 个测试文件逐个 0 FAIL；`go build ./...`、vet、gofmt、`scripts/apidoc`、`scripts/check_tasks_ledger.py` 全绿。
- **剩余（V4 仍不标绿）**：L1(1e6/1e6)/L2(3e6/3e6)/L3(3e6/5e6)/S(1e7 定高) 四档真窗串行重跑（隔离 `:3`、标准关闭、每轮健康检查）、滚动 fps/p95/hitch 与首屏/RSS 真窗口径、独立滚动条拖动、真键事件通道。

## V4 任务 20：四档真窗串行重跑 + 两处 O(n) 洞修绿（2026-11-14 · 整体仍不标绿，剩两项待拍板）

- **示例参数化（演示数据控制动词，示例只用组件公开 API）**：`ui_text_large_scale` 加 `-lines`（1e6/3e6/1e7）与 `-burst` 计时探针；`ui_text_v4_longline` 加 `-bytes`（1e6/3e6/5e6）、`-burst`、`-cpuprofile`，并补 `proc.Sample()`（此前静态轮无 RSS 样本、verdict 恒 inconclusive）。burst 探针带防假绿有效性断言（编辑真实发生 / 虚拟模式保持 / 长行窗口真滑动），口径=唯一行+每击键不同位置+中位数；观察值不替代 CPU 权威门禁。L3 多行部分复用 L2 证据（同为 3e6 行文档）。
- **两处 O(n) 洞（红灯起步→转绿，均四档实测暴露）**：
  1. `Editor.SetSelectionBytes`（`ui/textinput/editor.go`）：旧实现 `len(e.Text())` 钳制 ×2（tree 模式每次物化整串）+ 两个全前缀 range 扫描数 UTF-16 单元——20MB 文档实测 21.08ms/次、53MB 60.3ms/次（真窗 burst caret 相 60,325µs 中位），超 §2.2 caret ≤4ms 门 5 倍；既有 CPU 门禁只测 `caretAnchor`（9.5µs）未覆盖此入口（V0 §7.2 假快陷阱同款）。修 = `Buffer.Len()`（O(1)）+ `Buffer.UTF16OffsetAt`（V1b-3 已建 O(log n)，块边界不劈 rune、`utf16Count` 与旧扫描逐 rune 语义一致；`box.go:2183`/`editor.go:1533` 同款惯例）。`20.84ms→5.59µs`，真窗 caret `60.3ms→150µs`。锁：`TestV4Gate_SetSelectionBytes{NoMaterialize,Multibyte,CaretBound}`（修前红/修后绿）。
  2. `capBytes`（`ui/rendering/long_line.go`）：守卫 `nChars >= utf8.RuneCountInString(s)` 为判断「上限是否超过全文」每次把整行 rune 数完，`longLineIndex.Window` 每窗口查询都调它（CPU 画像 50.25% cum，`Window` 占 85.8%）——Box 全路径 5e6 单行击键真窗 `28.8ms`（1e6=3.5 / 3e6=17.5 / 5e6=28.8，线性 ≈5.7µs/KB，超 16ms 门）。修 = 守卫融进既有步进循环（走满 nChars 步或到行尾即停，`O(L)→O(min(cap,L))`，逐案语义等价）。`capBytes 5.34ms→85.9µs`、Box 路径 `13.38→2.55ms`（余量 1.2×→6.3×）、真窗击键修后 `1.30/3.86/5.47ms`（p95 ≤11.7ms 全档过 16ms 门）。锁：`TestV4CapBytes_{Equivalence,CappedWalk}` + `TestV4Gate_LongLineBoxKeystroke5M`（修前 Box 全路径 13.38ms 贴门、修后 2.55ms）。
- **四档实测**（隔离 `:3`、串行、每轮 `xdpyinfo` 健康检查、RUN_SECONDS 标准关闭）：
  - 多行窗：L1 1e6 行 65MB `fps 57.46 / p95 19.47 / hitch 3.00 / 首同步 3µs / burst 257/138/122µs`；L2 3e6 行 197MB `fps 57.53–57.79 / p95 19.27–21.99 / hitch 6.50/4.00/6.00/3.00（4 轮中位 5.0 骑线）/ burst 194/134/135µs`；S 1e7 行 658MB 定高 `fps 57.64–57.75 / p95 19.13@120s、23.29@240s / hitch 5.50@120s、4.25@240s / 构造 5.86s（观察值）/ burst 186/142/134µs`（burst 与档位无关=多行路径规模解耦）。
  - 单行窗：L1 1e6 `fps 59.75 / p95 19.54 / hitch 0.00`；L3 5e6 `fps 59.75 / p95 19.82 / hitch 0.00`；L2 3e6 修后击键 `3.86ms`（p95 8.6）/ caret `1.33ms`。
- **内存 D5（沿 V2 收口口径：110s 堆快照、去 75.4MB 字体文件缓存）**：L1 ls `112.9≤166MB` / L1 v4 `47.2≤65.5MB` / L2 `246.2≤373MB` / L3 v4 `60.1≤71.5MB` / S `713.3≤1099MB` 全过；S 堆 60s→110s diff `1.5MB/786MB`（0.19%，零增长）；rope 叶零拷贝共享原串（文本域 = 1× 文本）。raw RSS（L1 525MB / L2 835MB / S 1951MB）含 GC 头 room 与驱动侧映射，如实上报、不作判定口径。
- **GC 归因证据（L2 hitch 骑线）**：`GOGC=50` 判别实验 GC 轮 40→95、hitch `3.00→10.50` 反向钉死 GC 周期与慢帧因果；慢帧（34–47ms）与 GC 并发标记起点部分重合（8 帧中 2–3 帧）；堆快照稳态零增长。修法方向 = 滚动布局路径分配削减（`growslice` 5.4% + `mallocgc` 11%），引擎级工作、待拍板。
- **S 档 RSS 判定口径事实**：120s 轮稳态斜率 `40715KB/min fail`，240s 轮 `844KB/min pass`、终值同平台（1951/1957MB）——120s 的 fail 是 GC 头 room 爬坡尾巴（爬坡约 150s 到位），非泄漏；同轮 240s p95 `23.29` 超 22 而 120s 口径 `19.13` 过（长窗统计更严）。判定时长口径待定。
- **既有门禁复跑未破坏**：v4 `-auto-only` 三证据逐值复现（`diff=0/763584px`、`marker_diff=16`、`MarkerX=8505.0`、window=(0,631)）；V3c `-wrap -auto-only` 5 探针 + Golden `diff=0` 全绿。
- **回归**：128 个测试文件逐个 `126 PASS / 0 FAIL / 2 SKIP`（SKIP 为无 Test 函数的基准文件）；`go build ./...`、scoped vet（`ui/platform` 的 unsafe.Pointer 警告为存量、非本轮文件）、gofmt（本轮文件干净）、`scripts/apidoc`、`scripts/check_tasks_ledger.py` 全绿。
- **用户拍板（2026-11-14）**：① L2 多行 hitch 骑线→**开新专项修**（滚动布局路径分配削减，动手前先做影响面评估并停下报告；专项完成并复测后再谈 V4 标绿）；② 判定时长口径→**分指标口径**：p95/fps/hitch 用 120s（与 V2/V3c 基线同口径可比），RSS 稳态斜率用能到平台的时长（S 档 240s）——按此口径 S 档 p95 19.13 / RSS 844KB/min 均绿；③ 本轮 9 文件按清单提交（两洞修复+新测试+示例参数化+台账），全部本地未推送。
- **剩余（V4 仍不标绿）**：hitch 分配削减专项（见上）完成并复测后再谈标绿；IndexByte 单行判定残留 O(L) ≈1ms/op@5e6 留诚实项（多行判定语义涉及虚拟模式交互，需单独评估）；独立滚动条拖动、真键事件通道未实测（沿前）。

## hitch 分配削减专项第一阶段收口 + V4 标绿（2026-11-14 · 带注记）

- **分配削减三处落地（红灯起步→绿）**：① `HbShaper` 持 hb Buffer 成员 `Clear()` 复用 + runes scratch（Skia `SkShaper_harfbuzz.cpp:721` `HBBuffer fBuffer` 同款；go-text `Buffer.Clear` 保留容量；Shape 全程持 `s.mu` 锁内使用无裸共享，`shaper_hb_race_test` 兜底）；② `buildShapedCarets` 容量预估（每可见行省 2-3 次 growslice）；③ `rebaseGlyphs` 精确预分配。`AllocsPerRun` **51.0→8.0 allocs/shape**（`render/text/v4_shape_alloc_test.go` 红灯 51.0 超 40 界→修后绿）；画像 growslice 5.4%→跌出 top12、mallocgc 11%→5.2%。
- **行为无损**：byte-exact 全家 + 并发竞态 + 分段复用全绿；131 文件（render/text + ui/rendering）逐个回归 **0 真实 FAIL**——2 个外部工具链长跑（`zz_b1_ttf_scan`/`zz_b2_bitmap_scan`，对照 TTF hinting 轮廓/位图扫描，不经过 Shape 路径）在长 timeout 下单独验证 PASS（1183s/1787s，远超回归脚本 600s 上限）；2 个比值型门禁（height_map/m1_paint）在负载互扰下假红、干净复跑绿。
- **L2 复测五轮 `4.00/5.50/4.50/6.50/7.01`（中位 5.0），hitch 未动——GC 主源假设证伪**（GC 相关 CPU 减半，hitch 分布不变）。单帧分解基准（`ui/textinput` 临时探针，已删）钉死：spike 帧（caret 跳 66 行 + sync + 物化）CPU 仅 **50-55µs**，虚拟模式只物化 caret 行、绘制时才逐行布局——CPU 排版彻底排除。慢帧 34-47ms 为 spike 相位 **0.4% 偶发**，Go 画像穷尽后剩 cgocall（GPU present/驱动，21% CPU）。环境漂移实锤：未改路径的 L1 从 3.0 漂到 4.5。
- **归位（新问题归位纪律）**：剩余两股归**渲染域**台账（`ENGINE_RENDER_GPU_CONVERGE_TASKS.md` §2.1 余留登记）：① 字形 atlas 脏区上传（快滚帧 ~2600 新字形/帧小块上传驱动开销，`glyph_mask_atlas.go` DirtyUploads → gpu flush 链）；② present/驱动 stall（940MX + Present/DRI3 偶发，同 AGENTS 事故基线栈）。hal 层上传批量化/present 策略另立专项。
- **V4 标绿（带注记）**：能力面（§3 第 4 层）+ 真窗三证据（4 探针 / Golden diff=0 / marker 差分 / 负向对照 / 人工确认）+ §2.2 CPU 门禁 + 四档真窗（分指标口径：p95/fps/hitch 120s、RSS 到平台时长）+ 两处 O(n) 洞修复 + 分配削减专项第一阶段全部收口。注记 = 本机 spike 口径下 L2 hitch 4-7/min 骑线，定性渲染域问题（文本引擎侧已穷尽），hal 层批量化专项完成后可复测。诚实项保留：IndexByte 单行判定残留（≈1ms/op@5e6）、独立滚动条拖动、真键事件通道、跨块 bidi 仅 line-level（§2.3）。
- **提交**：专项 4 文件（shaper_hb.go / text.go / text_layout.go / v4_shape_alloc_test.go）+ 3 处台账（渲染域任务表 / 立项书 §4+§8 / 本文件），全部本地未推送。

## hitch 专项第二阶段：帧分段埋点 + MAILBOX 实验（2026-11-14 · 机制钉死，骑线定性测量环境）

- **帧分段直方图埋点（纯观测，行为零变化）**：`ui/scheduler` 加 `stageDist` 环形收集器（8192 帧样本，Snapshot 时算分位），五段全接——build/raster/flush/present_wait/acquire_wait 各 p50/p95/max 进 `FrameMetrics` 并由 wrgate 透传（present_wait/acquire_wait 原本只有 Note* 无分布）。scheduler/wrgate 测试绿。
- **机制铁证（L2 滚动 120s）**：慢帧归属=raster 段（p95 13.35/max 361ms），HITCH_DIAG 逐帧归因 22 帧 `ui→submit=0、submit→done 5-28ms`——**raster 偶发超 16.7ms 帧预算（acquire 背压 p95 8-9/max 17 + flush 偶发 max 22-32）→ 错 vsync → interval 33-59ms**；present_wait p50 0.15ms 干净（vsync 节流在 X compositor）。build p95 5.6 干净、drainSnapshots 无快照空转排除。
- **atlas 上传假设消解**：链路已是 Skia 模式（脏区按页合并 + 256KB/帧预算 Deferred + 部分上传行对齐，J5/P2.4e 已做）；字形集 ~40 字符×变体 ≪16384，滚动初期饱和后全命中零上传；静态视口对照 hitch 0.50/min vs 滚动 4-7/min——慢帧严格随滚动负载。
- **MAILBOX 实验不可判，已回退**：`SetPreferMailboxSteady`（Mailbox→FifoRelaxed→Fifo 无 tearing）+ `GPUI_PRESENT_MAILBOX=1` env 开关，四轮 28-34/min vs 回退 Fifo 基线 34/min 无显著差异——测量时段 load average **3.53-4.39**（4 核满载边缘：ZCode renderer 38.5% CPU 等常驻负载），对比被环境漂移淹没。开关与方法按「不交付无效改动」回退，行为零变化。
- **最终定性**：hitch 骑线 = 4 核满载边缘机器上的**测量环境负载依赖**（同一份代码：凌晨空闲 3.0-4.5/min → 高负载时段 9-34/min）+ GPU 消化偶发超帧预算（wgpu/驱动内部，需 GPU capture 工具深挖，同「misc_draw wgpu 执行态」先例挂账）。引擎侧四路全验证穷尽（CPU/GC/atlas/present 策略）。**复测口径**：干净时段（load <1）跑一轮分段直方图滚动轮即可复核，无需改代码；明细归位 `ENGINE_RENDER_GPU_CONVERGE_TASKS.md` §2.1。
- **V4 标绿注记维持**，如干净时段复测 hitch 回到 ≤5 带内，注记可摘。

## hitch 终判：注记摘除 + p95 先例判机器基线（2026-11-14 · 用户终判）

- **用户空闲环境终判两轮**（codebuddy 关闭、ZCode/agent 会话闲置）：L2 hitch **1.00 / 2.00 per min**（门 ≤5，大幅达标）、fps 58.20/58.19、virtual=True、24.2 万行真实滚动——**V4 标绿的 hitch 注记摘除**。
- **p95 23.71/26.15 稳定超 22**：同环境 L1 对照 26.21/hitch 31.51——L1/L2 同波动带（19-27，跨两天多轮），**非 3e6 规模回归、非本轮引入**；援引 V3c 收口先例（22.95-23.03 判「当前机器基线」）判为机器基线；空闲下分段 raster_p95 5.8/acquire_p95 2.7 全干净，尾巴=环境（桌面负载/显示栈/GPU 频率档）。
- **测量环境事实**：hitch 同代码 3-34/min 随环境强波动（agent 会话活跃时 6-34、凌晨空闲 3-7、用户空闲 1-2）；940MX utilization 全程 0% + glxinfo renderer=Intel HD 520（渲染在核显，940MX 135MHz 归因修正）；940MX 笔记本卡锁频不支持（`-lgc` not supported 实测）。
- **复测口径沉淀**：终判须 agent 会话闲置+桌面无重负载；stdout 单独重定向（2>&1 混入启动日志会破坏 JSON 解析）。
