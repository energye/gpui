# ENGINE_TEXT_SCALE_PLAN — 任意规模文本编辑排版架构（O(1) 击键 · 分期施工）

**状态**: **立项（未开工）** — 本文档为设计与施工总纲，实施按 `M0 → M1 → M2 → M3 → M4 → M5` 分期触发，每期独立验收、独立可回滚。`M3.5` 为条件触发（非必经），`M6` 为贯穿全程的验收主窗（非施工期）。
**日期**: 2026-09-02（立项；过程见 `docs/ENGINE_TEXT_SCALE_CHANGELOG.md`）
**触发**: `ui_wr_ime_r*` 真窗实测「文本越多输入越卡」。实测定位：卡顿 100% 来自**排版与绘制层**，与 IME D-Bus 通道无关（详见 §2）。当前模型是**每次击键整串重排**，复杂度 O(n²)，靠优化常数无法解决量级增长。
**定位约束**: 本仓库是**跨平台 wgpu 渲染框架 + GUI 组件库底层**，不是单一应用。因此本方案**不接受任何场景取舍**——不得出现「这个 case 我们不支持」「长文本走简化路径」这类分支。所有取舍必须是**复杂度上界的取舍**，不是**功能子集的取舍**。
**问题范围**: 共 **4 个面**（详见 §3.6），非仅 `TextLayout.Lines` 一处：面1 字段暴露 / 面2 方法内部 O(n²) / 面3 `Generation` 语义 / 面4 绘制侧平行实现（**单源纪律现已破损**）。

> **实证状态**：§2 根因、§3.5 三条约束、§3.6 四个面的关键论断均已在当前代码上跑探针复验（探针已删，仓库零改动）。当前值：量宽 ~430ns/字、面 4 为 2v12/11v1、混排切 6 run。过程明细见变更记录。
**开工前必读**: `docs/ENGINE_TEXT_SESSION_BRIEF.md` —— **新会话开工话术 + 工作纪律 + 纪律速查卡**，每期开工前先读这份（含可直接复制的角色设定与证据纪律）。

**关联**:
- `docs/ENGINE_TEXT_SESSION_BRIEF.md`（**新会话开工简报，开工先读**）
- `docs/ENGINE_TEXT_X11_IME_REQUIREMENT.md`（IME 通道规范，本期不动）
- `docs/ENGINE_TEXT_SHAPING_PLAN.md`（shaping 层已切 HarfBuzz Go 移植，A 类完成）
- `docs/ENGINE_UI_WIDGET_RENDER.md`（真窗/指标族/分期真源）
- `docs/UI_PIXEL_ASSERTION_STANDARD.md`（像素断言 F0–F9 规范）
- `docs/RENDER_API_CATALOG.md`（render 公开 API 总账，改公开面必同步）
- `AGENTS.md`（分层纪律 · 引擎层禁止硬编码 · 禁止示例层绕引擎洞）

### 读法（新会话从这里开始）

> **🚀 新会话：先打开 `docs/ENGINE_TEXT_SESSION_BRIEF.md`**，那里有可直接复制的**开工话术**（角色设定 + 证据纪律 + 交付要求）和**纪律速查卡**。把话术复制给新会话，比让它自己读本文档更可靠。

**先选你的读法档位**（全文 1370+ 行，不必全读）：

| 档位 | 适用 | 读哪些 | 量 |
|---|---|---|---|
| **⚡ 最小开工**（推荐，要动手写代码时） | 只需知道这期改什么、怎么算做完 | §0 硬目标 → §1 术语 → **§3.2.1 不变量 I1–I11** → **§6.3.1 真窗生命周期表** → **你要做的那一期条目** | **约 120–180 行**（≈10–13%） |
| **🔍 完整开工**（本期开工前想理解决策依据） | 需要理解为什么这么做、背景是什么 | 最小开工 + §2 根因 + §3.5 三条约束 + §3.6 四个面 + §9.1 决策 | 约 400 行（≈29%） |
| **🧭 全盘理解**（项目负责人/复核者） | 要评估方案合理性 | 通读全文 | 全部 |

**按「最小开工」档的完整步骤**：
1. 读 **§0 硬目标 G1–G8**（22 行）与 **§1 术语**（21 行）；
2. 读 **§3.2.1 跨期不变量 I1–I11**（红线，违反即回退）；
3. **查 §6.3.1 真窗生命周期表**——确认本期要用的真窗是否存在，不存在则按「建于」列先建；
4. **只读你要做的那一期条目**（M0–M5），每期含九项规范（§9.2），自洽可开工；
5. 验收标准看 **§6**，最终交付看 **§5.6 M6 验收主窗**。

> **§3「目标架构」整节约 260 行，开工不必通读**——其中的关键结论已提炼为 §3.2.1 不变量表（I1–I11），读那张表即可；需要论证细节时再按条目内引用回查。

> **🔴 开工前必读（否则会卡住）**
> - **M0 的第一件事不是改代码，是 M0-pre**：验收主窗 `ui_text_edit_accept` **目前不存在**，但 M0–M5 每期都要跑它。必须在动代码**之前**建好并跑出基线，否则发散基线再也取不到。见 **M0 第 ② 项**。
> - **面 4 不能靠「删绘制侧」实现**：`TextLayout` 当前**缺** `MaxLines` 截断、`Ellipsis`、无 Face 估算回绕三样能力（绘制侧都有）。见 **M0「面 4 落地要点」**。

---

## 0. 硬目标（不可协商）

| # | 目标 | 度量方式 |
|---|------|----------|
| **G1** | **击键代价与文档长度无关** | 单次击键耗时 T(N)，N ∈ {1e3, 1e4, 1e5, 1e6}。分档：**M1 收口 ≤ 5**；**M5 总收口 ≤ 1.5**（硬目标）。全程由 `BenchmarkKeystroke` 的比值门禁守（§6.2），禁止口头宣称 |
| **G2** | **任意脚本正确** | 拉丁/CJK/阿拉伯/希伯来/印度系/泰/缅/高棉/emoji，字形序列与 HarfBuzz 参考一致 |
| **G3** | **任意长度可用** | 1 字 ~ 1e6 字、1 行 ~ 1e6 行，均不崩、不卡、内存有界 |
| **G4** | **不破坏现有真窗** | 全部 `ui_wr_*` / `ui_wr_ime_r*` 真窗保持绿，行为逐像素等价 |
| **G5** | **每期独立可回滚** | 每期有独立开关或独立包，失败可退回上期，不阻塞主线 |
| **G6** | **复杂度门禁自动化** | 每期的复杂度上界必须有 `testing.B` 基准断言，进 CI，不允许口头宣称 |
| **G7** | **新会话可独立接手** | 每一期条目必须自洽到「新开会话只读该期即可开工」，详见 §9.2 九项规范 |
| **G8** | **最终验收是真实输入框** | `ui_text_edit_accept`（§5.6）——**单行 + 多行**两个真实可交互输入框，人工 + 自动双轨验收 |

### 0.1 非目标（本期不做）

- IME 协议层改动（D-Bus/IBus/fcitx5 通道已可用，且实测非瓶颈）
- 像素层改动（hinting / 光栅化 / MSDF / autohint — 自研独有资产，不动）
- 自绘候选窗、拼写检查、富文本编辑器 UI
- 协同编辑 / CRDT

---

## 1. 术语

| 词 | 定义 |
|---|---|
| **n** | 文档总字符数 |
| **L** | 被编辑行的长度（通常 < 100） |
| **V** | 视口内可见字形数（通常 100~2000） |
| **run** | 同一 (字体, 字号, 脚本, 方向, 颜色) 的连续字符段，整形的最小单位 |
| **itemizer** | 把文本切成 run 的组件（含字体回退决策） |
| **簇 / 字素簇 (grapheme cluster)** | 用户可感知的最小编辑单位（组合音标、ZWJ emoji 序列 = 1 簇），按 **UAX #29** 定义。⚠️ 现有 `snapToGraphemeBoundary`（`text_layout.go:470`）**名不副实**——仅做 UTF-8 续字节归位，非 UAX#29 |
| **段 (paragraph)** | 以 `\n` 分隔的文本块。回绕模式下是**缓存与失效的粒度单位**（一个段可回绕出多个视觉行） |
| **视觉行 vs 段** | 视觉行 = 屏幕上看到的一行；段 = 逻辑文本块。不回绕时二者 1:1；回绕时一段可含多行 |
| **前缀和** | 累积宽度数组，支持偏移→X 的 O(1) 与 X→偏移的 O(log n) |
| **piece tree** | 以「片段+来源」表示的文本缓冲，编辑 O(log n)、快照 O(1) |
| **懒测量** | 未显示过的行不实测高度，先用估算值，滚动到时再实测并增量修正总高 |
| **复杂度门禁** | 用 `testing.B` 在多个 N 上测同一操作，断言耗时比值 ≤ 阈值 |
| **单源纪律** | `TextLayout` 是布局与绘制**唯一**几何真源；绘制侧不得独立计算行或坐标（不变量 I1/I7） |
| **A1 / A5** | 本文档沿用 `docs/ENGINE_UI_WIDGET_RENDER.md` 的既有硬约束编号：**A1 = 单源纪律**（布局与绘制必须同源，不得各算一套）、**A5 = `TextLayout` 对外唯一几何源**。本文档的 I1/I7 是对 A1/A5 在本项目内的具体化 |

---

## 2. 现状与根因（实测，非推测）

### 2.1 实测数据

**A · 击键路径（本机，5000 字 / 9444 字节，每次击键新字符串以绕过缓存）**

> **🔴 口径声明**：下表数值为 **`MultiFace` 口径**（`ui_wr_ime_r*` 真窗的实际路径，经 `wrkit.FaceAt()` 装载 MultiFace）。
> **但 `singleFace` 同语料是 34.06 ms，比 MultiFace 慢 14.9 倍**（本轮实测，见下表 A-2）。原因是两条路径走的是不同分支——**见下方「为什么单 face 反而更慢」**。
> **不标注这个口径，会让新会话严重低估 M0 的收益空间**（以为排版就 2ms，实际修通整形后会先跳到 34ms 再降下来）。

| 环节 | 200 字 | 1000 字 | 2000 字 | 5000 字 |
|---|---|---|---|---|
| 排版重建 `BuildTextLayout` | 0.15 ms | 0.70 ms | 1.05 ms | **2.35 ms** |
| `face.Advance`（整串量宽） | 0.08 ms | 0.59 ms | 0.69 ms | **1.95 ms** |
| `text.Shape`（整串整形） | 0.002 ms | 0.004 ms | 0.004 ms | 0.010 ms |
| `InputBox.Sync`（一次击键全链路） | 0.18 ms | 0.85 ms | 1.23 ms | **2.68 ms** |
| 光标查询（查表） | ~0 ms | ~0 ms | ~0 ms | 0.002 ms |
| 4000 字节 surrounding 截断 | 0.002 ms | 0.007 ms | 0.011 ms | 0.028 ms |

**A-2 · 单 face vs MultiFace 对照（本轮实测，同一 5000 rune 中英混排语料）**

| 环节 | `MultiFace` | `singleFace` | 倍数 |
|---|---|---|---|
| `BuildTextLayout` 5000 字 | **2.290 ms** | **34.057 ms** | **14.9×** |
| `face.Advance` 5000 字 | 1.58 ms | 0.30 ms | 0.19× |
| `text.Shape` 5000 字 | 0.0088 ms | 0.0246 ms | 2.8× |

**为什么单 face 反而更慢（关键机制）**：

| 路径 | `Shape` 结果 | caret 构建分支 | 复杂度 |
|---|---|---|---|
| **`MultiFace`** | 返回 **0 glyph**（`Source()` 为 nil） | 走 `:190-206` **逐字量宽兜底** | **O(n)** |
| **`singleFace`** | 成功返回 glyph | 走 `:222-231`，每个 rune 调 `CaretXForCluster` **扫全表** | **O(n²)** |

实测 `CaretXForCluster` 全表耗时 **36.x ms**，占 `BuildTextLayout` 总耗时 34.06 ms 的 **98%**——**即单 face 的慢，98% 来自这处 O(n²) 扫表**。

> **🔴 这条实测直接印证 M0 的「先慢后快」熔断**：M0 修通 `MultiFace` 整形后，代码会从 **O(n) 兜底切到 O(n²) 成功路径**，5000 字耗时将从 2.29 ms 跳到 ~34 ms（**慢 15 倍**）。
> **这不是纸上风险，是实测确认的**。因此 M0 第 8 项必须与 `CaretXForCluster` 扫表消除**同时做**，否则 M0 会造成一次**真实的 15 倍性能回退**。

**B · 单字量宽成本（稳态实测）**

稳态实测（冷启动首字偏高，不代表稳态）：

| 场景 | 实测 | 说明 |
|---|---|---|
| 单 face（`sourceFace`） | **427 ns/字** | 1000 字混排（`"a世界b"`×200）取均值 |
| `MultiFace` | **449 ns/字** | 同上 |
| `MultiFace.Advance(1000 字整串)` | 0.275 ms（≈275 ns/字） | 整串路径有内部缓存 |

> **注**：冷启动首字含首次解析开销，稳态约 **430 ns/字**。
> **但这不改变结论**：5000 字兜底逐字量宽 ≈ 5000 × 430 ns ≈ **2.15 ms**，与 A 表 `face.Advance` 1.95 ms、整串排版 2.35 ms 互相印证——**量宽仍是击键耗时的主要构成**。M0 的 advance 缓存目标改为「消除重复解析的**尾部抖动**与 leftover 解析开销」，基准门禁由「≤ 200 ns」放宽为 **「≤ 250 ns 且 P99 不劣化」**（见 M0 第 ⑥ 项）。

CPU profile（200 次击键采样，cum 列为含被调用方的累计占比）：

```
flat   flat%    cum%     cum        symbol
 20ms   3.39%  45.76%  510ms 86.44%  ui/rendering.buildCaretsForLine   ← 排版，累计占 86%
 10ms   1.69%  74.58%  320ms 54.24%  text.(*MultiFace).glyphForRune
 10ms   1.69%  76.27%  450ms 76.27%  text.RuneAdvance                  ← 单字量宽，累计占 76%
 50ms   8.47%  18.64%  240ms 40.68%  text.(*sourceFace).glyphForRune
 60ms  10.17%  10.17%   60ms 10.17%  runtime.duffcopy                  ← 大对象拷贝
```

读法：`buildCaretsForLine` 累计占 **86.44%**，其中经 `RuneAdvance` 链（累计 76.27%）的部分是单字量宽。

> **🔴 归因**：`duffcopy` 的调用方是 **`RuneAdvance` / `glyphForRune`（量宽链）**，**不是 undo**。实测：
> - `pprof -peek duffcopy` 显示其调用方是 **`RuneAdvance` / `glyphForRune`（量宽链）**，**不是 undo**；
> - **对照实验（决定性）**：只跑 `Editor` 编辑 3000 次、**完全不触排版**，profile 中 `duffcopy`/`memmove` **恒为 0**。
> - `duffcopy` 实测占比 **6.90%**（另一轮 5.17%）。
>
> **影响**：根因⑥ 的证据用快照常驻内存实测，不引用 duffcopy。

### 2.2 根因链（从现象到代码）

**① `MultiFace` 使整条整形链路失效**
`render/text/multi.go:164` `MultiFace.Source()` 硬编码返回 `nil` → 三个 shaper（Own/Hb/Builtin）进门即 `return nil`。实测 `text.Shape(5000字, MultiFace)` 返回 **0 个 glyph**。而 `wrkit.FaceAt()` 给所有 `ui_wr_ime_r*` 装的正是一个 MultiFace → **全员中招**。

**② 返回 0 glyph → 退化逐字量宽**
`ui/rendering/text_layout.go:190-206` 兜底分支（`len(glyphs)==0` 时）对每个字调 `text.RuneAdvance`。稳态实测 ≈ **430 ns/字**，5000 字 ≈ 2.15 ms，这就是那 76%。

**③ 兜底分支内藏 O(n²)**
`buildCaretsForLine` 对每个 rune 调 `CaretXForCluster`（`text_layout.go:222-231` 循环），该函数每次**从头线性扫一遍 glyph 数组**（`render/text/shaped.go:125-160`）。建 n 个 caret 扫 n 次 = **O(n²)**。5000 字 = 1250 万次比较。

> **注意**：第 ③ 条的 O(n²) 只在 **`Shape` 成功返回 glyph**（即单 face 路径）时才成立；在 `MultiFace` 路径下 `Shape` 返回 0 glyph，走的是第 ② 条兜底分支，**只有 O(n) 的逐字量宽**。两条路径的瓶颈不同，M0 修复后**反而会进入第 ③ 条的 O(n²)**（因为 glyph 不再为空）。因此 M0 修完 ①② 后**必须立即复测 caret 构建耗时**，否则会「修完更慢」（见 M0 第 ⑦ 熔断条件）。

**④ 一次击键触发多轮全量重排**
`InputBox.sync()` 内 `SetText` 清缓存后，`TextLayout()` / `syncSelectionHighlight()` / `layoutCaret()` / `caretAnchor()` 各自再摸 layout；密码框额外 `MeasureWidth`。

**⑤ 绘制逐字提交，裁剪形同虚设**
`ui/rendering/text.go:895-921`（`RenderText.Paint` 内 `face.Source()==nil` 分支）因 glyph 为空 → 逐字 `DrawString`，且逐字路径里对每个 rune 还要**线性扫 carets 找 X**（`text.go:906-913`），是第二处 O(n²)。
`cullGlyphs`（`text.go:945`）的调用条件为 `t.hasViewportHint && t.MaxWidth == 0 && len(lay.Lines) == 1 && len(glyphs) > 200`——**四条件与的关系**：`glyphs` 在 `MultiFace` 下恒为 0 使 `len(glyphs) > 200` 恒 false，且要求**恰好单行**才生效。实测该裁剪**从未生效**；多行（`MaxWidth > 0`）与横向长行未被覆盖。

**⑥ undo 存整篇快照**
`editor.go:144` `pushHistory` 存全文 ×100 份。**实测：100 份互异快照常驻 18.54 MB**（理论 18.01 MB，语料为中英混排 1e5 字）。
> **口径说明**：纯 ASCII 约 9.54MB，中英混排（约 1.9 字节/字）实测 **18.5 MB**。
> `pushHistory` 是纯 append，实测 **0.10–0.12 µs 且与文档长度无关**（Go string 只拷 header，底层数组共享）。真正的成本是**内存常驻**（100 份全文），不是编辑时的拷贝耗时。

**⑦ 省略号拟合**
`ellipsizeToWidth` 每次试探都是新字符串 → cache miss → 整行重量。
> **复杂度为 O(n)**。实测 `ellipsize 总耗时 / 一次 Measure 耗时`在 n=1e3→1e6 恒为 **8.0–8.7×**，**不随 n 增长**。clip 对照组同为 8.2–8.6×，印证**瓶颈是整行 measure 而非二分**。
> **影响**：M5 改为基于前缀和的 O(log n) 后，直接从 O(n) 到 O(log n)。

### 2.3 已排除（不是瓶颈）

`TruncateSurrounding`（0.028 ms）、D-Bus `SetSurroundingText`（有 300ms 超时 + `lastSurr` 去重）、`ensureReprobe`（有 500ms 节流 + `icOwnerSame` 早退）、`ProcessKeyEventAsync`（已异步化）。**卡顿纯在渲染层。**

---

## 3. 目标架构

### 3.1 分层

```
层   职责            内容                              代价        建立于
─────────────────────────────────────────────────────────────────────
L7  GPU 提交        裁剪 + 批处理 + 图集分页              O(V)        M2
L6  视口/虚拟化     行窗口 + 懒测量 + damage 只报变动行/段  O(可见行)   M4
L5  布局缓存        行缓存（不回绕）/ 段缓存（回绕）         O(L)/O(段长) M1
L4  前缀和 caret    累积宽度表 → 偏移↔坐标 O(1)/O(log n)   O(L)        M1
L3  整行整形        bidi → run 切分 → 每 run 整形           O(L)        M0
L2  断行/回绕       在整形结果上回绕（先整形后回绕）           O(L)        M1
L1  行索引          行数 / 行起始偏移                      O(log n)    M1
L0  字体度量        advance 缓存 + 回退链                  O(1) 摊还    M0

旁挂（不参与主链）
  P1 文本缓冲       piece tree（**M3.5 条件触发，非必经**）  O(log n)    M3.5
  P2 增量 undo      undo 历史只存增量                      O(1)        M3
```

> 分层编号 **由上至下为调用方向**（L0 是地基）。`P1/P2` 是旁挂层：piece tree 押后为条件触发（Q3 决策），增量 undo 独立改造，二者均不影响 L0–L7 主链。

### 3.2 复杂度契约（硬）

| 操作 | 当前 | 目标 | 门禁阶段 |
|---|---|---|---|
| 光标处插入/删除 | O(n) | **O(log n)** | M3.5 |
| 取第 k 行 | O(n) | **O(log n)** | M1 |
| 偏移 → (行号, X) | O(n) | **O(log n)** | M1 |
| 点击 (x,y) → 偏移 | O(n) | **O(log n)** | M1 |
| 击键后重排（不回绕） | **O(n²)** | **O(L)** | M1 |
| 击键后重排（回绕） | **O(n²)** | **O(段长)** | M1 |
| **`CaretForOffset`** | **O(n²)**(2万行 375ms) | **O(log n)** | **M1（面2）** |
| **`BoxesForRange`(全文)** | **O(n²)**(2万行 365ms) | **O(输出行数)** | **M1（面2）** |
| **`LineTop`** | O(n) | **O(1)**/O(log n) | **M1（面2）** |
| 击键后重绘 | O(n) | **O(V)** | M2 |
| undo 快照 | O(n) 拷贝 ×100 | **O(1)** | M3 |
| 文档总高（定行高） | O(n) | **O(1)** | M4 |
| 文档总高（变行高） | O(n) 全测 | **O(log n)** 摊还 | M4 |

**关键：击键代价里不出现 n。**

> **加粗三行（面 2，见 §3.6）**：查询 API 的 O(n²) 退化——实测 2 万行下单次调用耗时达 s 级，是比字段暴露（面 1）严重得多的问题。

### 3.2.1 跨期不变量（新会话必读 · 违反即回退）

以下不变量由各期建立，**后续任何改动都不得违反**：

| # | 不变量 | 建立于 | 违反后果 |
|---|---|---|---|
| **I1** | `glyph.X` 是布局与绘制的**唯一位置真源**，二者不得各自累加 | M0 | 约束 ① 复发（5000 字偏 976.6px） |
| **I2** | 布局缓存失效粒度 = **不回绕按行 / 回绕按段** | M1 | 回绕下退化为 O(n²) |
| **I3** | 裁剪只影响**提交范围**，不得影响坐标计算 | M2 | 可见内容被裁错 |
| **I4** | undo 历史**只存增量**，不得存全文 | M3 | 内存回到 O(n)×100 |
| **I5** | 未显示行的高度是**估算值**，依赖精确总高的逻辑不得直接使用 | M4 | 滚动条跳变 |
| **I6** | caret/hit 路径**禁止**逐字 `RuneAdvance` 累加（守 A1 单源） | M0 | O(n²) 复发 |
| **I7** | `RenderText.DisplayLines()` 等绘制侧 API **必须从 `TextLayout` 派生**，禁止独立行计算 | M0 | 绘制行与布局行不一致（实测 2v12 / 11v1） |
| **I8** | `CaretForOffset` / `BoxesForRange` / `LineTop` 必须走行索引 + 前缀和，**禁止全表遍历** | M1 | O(n²) 复发（2 万行 365.9ms） |
| **I9** | **按行/段失效标记独立于 `Generation`**：`Generation` 保持全局自增语义不动（有 2 处消费者），按行失效**另用**字段表达 | M1 | 改 `Generation` 语义会打破 `TestTextLayout_Generation` → M1-d2 熔断 |
| **I10** | run 切分 = **字体回退（`MultiFace.Runs`）+ script/bidi 分段（`segment.go`）叠加**；仅按字形有无切分不够 | M0 | 阿拉伯/印度系等有字形但需整形的脚本**不整形**（`Shape` 返 0 glyph）→ 显示错误 |
| **I11** | `MaxLines` / `Ellipsis` 等**截断语义必须在布局侧与绘制侧同时生效**，不得只在一侧 | M0 | 绘制 2 行 vs 布局 12 行 |

### 3.3 四条硬分界（API 设计约束）

1. **缓冲层不进 `render/text`**。文本缓冲（若 M3.5 引入）属编辑层，排版层只认「给我第 k 行/段的文本」。静态标签不必背缓冲层成本。
2. **排版必须懒，但 API 要藏住懒**。调用方调 `LineMetrics(k)` 时，不得因文档有 1e6 行就触发全量测量。暴露「懒测量提供者」接口，由**控件层**决定物化多少。
3. **`TextLayout` 对外仍是唯一几何源**（守 A5/A1 硬约束），内部从「一张整表」变为「按行/段缓存的视图」。Paint 与查询读同一份结果，不破坏单源纪律。
4. **回绕与不回绕是两套缓存结构，不是开关**。不回绕按行缓存、回绕按段缓存（约束 ② / Q2 决策）。二者 key 与失效粒度均不同，**不得用同一结构加条件分支糊过去**——否则回绕下会静默退化为 O(n²)。

### 3.4 不按窗口整形

早期讨论曾提出「超长单行只整形可见窗口 ±2000 字」。**本方案否决该做法**：

- 阿拉伯字母连接形式（isol/init/medi/fina）取决于**相邻字符**
- 印度系文字要做**重排序**（reph 后置等），重排跨度可很大
- bidi 是**段落级**算法（UAX #9），不是窗口级

**正确做法**：整行整形（`\n` 天然定界），**只在 GPU 提交时裁剪**。整形保证正确，裁剪只管性能，两者解耦——这也正是 Skia/SkParagraph 的做法。代价可接受，因为行级缓存让「整行整形」只在编辑那一行时发生。

### 3.5 三条实测约束（决定方案可行性）

> 以下三条是直接决定方案能否成立的硬约束（探针已删，仓库零改动）。

#### 约束 ①：绘制位置与布局位置会发散（hinting 取整累加）

`render/text/draw.go:113`（`drawGlyphs`）、`:229`（`drawGlyphsVariable`）、`:311` 绘制用 `snapPen += math.Round(adv)`——**每字取整后累加**；而布局（`BuildTextLayout`）用**原始浮点 advance 累加**。两者随长度线性发散。

**实测（DejaVuSans 16pt，`"a"`×N）**：

| 文本长度 | 布局末尾 X | 绘制 snapPen 累加 | 偏差 | 每字平均 |
|---|---|---|---|---|
| 100 字 | 980.469 | 1000.000 | **19.5 px** | 0.195 px |
| 1000 字 | 9804.688 | 10000.000 | **195.3 px** | 0.195 px |
| 5000 字 | 49023.438 | 50000.000 | **976.6 px** | 0.195 px |

> **注**：发散是**确定性线性函数**（每字恒定 0.195px），5000 字下约 **61 个字符宽**的偏移，肉眼与点击定位均可感知。对策 C 必须做。

**影响**：前缀和只能给出**布局坐标**；若仍逐字 `DrawString`，第 5000 字会偏离 **976.6px**（约 61 个字符宽）。

**对策选型（已拍板）**
- ~~A：绘制改用前缀和坐标（放弃逐字取整）~~ —— 需验证 hinting crisp stems 是否退化，未验证
- ~~B：布局侧同步取整（前缀和存整值）~~ —— O(1) 不变，但布局精度降低
- ✅ **C：按 run 批量整形 + 批量绘制** —— 绘制不再逐字推进，而用整形结果里的 `glyph.X`（与布局同源），发散消失

**决策（Q1）**：选 **C**。理由：唯一同时保住**画质**与 **O(1)** 的做法，且与 M0 打通 run 整形的目标一致。

**连带影响（硬）**：选 C 后，M2 的绘制批量化从「性能优化」升级为「**正确性前置**」——执行顺序锁定 `M0 → M1 → M2`，**不可调换**。若 M0 未完成就做 M2，批量提交会把近千 px 的发散**固化成批量错误**，比逐字错误更难排查。

#### 约束 ②：回绕模式下，编辑会级联作废后续所有行

实测（`MaxWidth=300`，首行插入一个词）：改首行后，**所有后续行的字节区间全部平移**，行内容重新断行：

```
改前 L0:[0,35)  "The quick brown fox jumps over..."   L1:[35,71) "lazy dog and then keeps..."
改后 L0:[0,30)  "INSERTED_WORD The quick brown "     L1:[30,62) "fox jumps over the lazy..."
```

**影响**：`MaxWidth == 0`（不回绕）时编辑第 K 行只作废第 K 行；回绕模式下必须作废 K 及其之后**所有行**，最坏退化为 O(n)。
**对策**：
- **不回绕（单行/代码编辑器）**：K 行隔离成立，O(L) 达标。
- **回绕（段落文本）**：引入**段落级缓存**——以 `\n` 分段，缓存粒度是「段」而非「行」；段落内回绕结果整体缓存。编辑只作废所在段。段长通常 << 文档长，O(段长) 达标。
- M1 的复杂度契约须按两种模式**分别书写**，不得用不回绕的数据冒充回绕达标。

#### 约束 ③：`MultiFace.Runs()` 已存在，itemizer 不必自研

**实测**：

| 输入 | `Runs()` 结果 | 判定 |
|---|---|---|
| `"Hello世界abc你好"`（DejaVuSans，无 CJK） | **6 个 run**：`"Hello"` / `"世"` / `"界"` / `"abc"` / `"你"` / `"好"` | ✅ **按缺字正确切分** |
| `"مرحبا بالعالم hello"`（阿拉伯+拉丁） | **1 个 run**（整串） | ⚠️ **阿拉伯文未被识别为缺字** |

另有 `globalMultiFaceRunsCache` 软缓存（`multi.go:329-332`）。

**影响与对策**：
- M0 的「itemizer」由「自研」**降级为「接线 + 验证」**——切分能力已具备。
- **但存在真缺口**：`Runs()` 仅按「字形是否存在于该 face」切分，**不做 script/bidi 分段**。阿拉伯文在 DejaVuSans 下有字形（故不触发回退），但**未经整形**（`Shape` 返回 0 glyph，见根因①）→ 阿拉伯文本**连字与方向均错误**。
- **因此 M0 不能只「接线」**：必须在 `MultiFace.Runs` 之上**叠加 `segment.go` 的 script/bidi 分段**，保证「同 face 但不同脚本/方向」的文本也切成独立 run 并各自整形。补单测锁：阿拉伯+拉丁混排须切 ≥2 run 且阿拉伯段 `Shape` 返回非 0 glyph。

### 3.6 全量问题清单（四个面，非仅 `Lines`）

> 懒加载受影响的是四个面，且面 2 比面 1 严重得多。本节是完整清单，后续分期按此覆盖。

#### 面 1 · 字段暴露 —— 外部可直接翻行

`TextLayout.Lines` / `.Carets` / `.Glyphs` / `Lines[].Width` 全量暴露。

**统计口径**：以下计数**仅限 `TextLayout.Lines`**（即 `ui/rendering` + `ui/textinput` + 相关示例/测试），**不含** `render/` 下同名但无关的 `Line` 类型（`render/internal/gpu/`、`render/text/layout.go` 等另有 155 处 `.Lines` 匹配，与本面无关，已从计数中剔除）。

**按改动难度分**（排除 `text_layout.go` 自身）：

| 类别 | 数量 | 迁移难度 |
|---|---|---|
| 真下标 `Lines[i]` | **47** | 🔴 硬 |
| 仅判空/取长 `len(.Lines)` | 80 | 🟢 易 |
| 遍历 `range .Lines` | 19 | 🟡 中 |
| **合计（去重后）** | **139** | |

> **计数口径**：三类相加为 146 > 总数 139，差 **7 行重复计数**——这些行形如 `for _, c := range lay.Lines[0].Carets`，同时命中「真下标」与「遍历」两类。7 行的确切归属：
> `ui/rendering/text.go:909`、`examples/ui_wr_ime_r1_editor/main.go:494`、`ui/rendering/text_layout_r2_test.go:32/58/129/249/270`。
> **去重后总引用 139、真下标 47 准确无误**（已独立复算两遍）。
> 「139 / 47」为**上述范围内的总量**（含测试与示例）；本文档统一用该口径。

**按归属分层**（决定各期工作量，实测统计）：

| 归属 | 文件 | `.Lines` 引用 | 其中真下标 | 处理阶段 |
|---|---|---|---|---|
| **生产代码（引擎层）** | `ui/textinput/input_box.go` | 43 | **11** | M1-d1 |
| | `ui/textinput/viewport_input.go` | 14 | **4** | M1-d1 |
| | `ui/textinput/visual_move.go` | 10 | **4** | M1-d1 |
| | `ui/rendering/text.go` | 15 | **3** | M1-d1 |
| | **小计** | — | **22** | **M1-d1 全部处理** |
| **测试文件** | `ui/rendering/text_layout_r2_test.go` | 24 | 11 | M1-d3（**先当裁判 d2，再改**） |
| | `ui/textinput/r1_selftest_visual_test.go` | 8 | 4 | M1-d3 |
| | `ui/textinput/r5_control_test.go` | 2 | 0 | M1-d3 |
| | **小计** | **34** | 15 | **M1-d3** |
| **示例探针** | `ui_wr_ime_r1_editor` | 8 | 3 | M2 |
| | `ui_wr_ime_r2_textlayout` | 6 | 3 | M2 |
| | `ui_wr_ime_r5_control` | 4 | 2 | M2 |
| | `ui_textinput_ime` | 4 | 2 | M2 |
| | `ui_wr_ime_r4_selection` | 1 | 0 | M2 |
| | **小计** | **23** | 10 | **M2** |

**主路径**：生产代码中 **`input_box.go` 11 + `visual_move.go` 4 = 15 处**，是打字与上下键移光标的核心，优先级最高。

**问题**：外部直接翻行 → 实现被迫全量物化 → 懒加载白做。

**处理口径（重要）**：**全部在 M1 内处理完**——生产代码 22 处（d1）；测试 34 处先当裁判（d2 不改）再改写法（d3）；示例 23 处在 d4 迁移，**必须排在 d5 删字段之前**（否则示例立即编译失败）。M1 结束时 `.Lines` 已删除，M2 及之后不得再引用。

#### 面 2 · 方法内部全表遍历 —— 🔴 比面 1 严重 100 倍

**实测（2 万行文档，单次调用耗时）**：

| API | 200 行 | 2000 行 | 20000 行 | 复杂度 |
|---|---|---|---|---|
| **`CaretForOffset`(末尾)** | 0.034 ms | 3.55 ms | **374.8 ms** | 🔴 **O(n²)** |
| **`BoxesForRange`(全文)** | 0.057 ms | 3.73 ms | **365.3 ms** | 🔴 **O(n²)** |
| `LineTop`(末行) | 0.0003 | 0.003 | 0.050 | 🟡 O(n) |
| `GetOffsetForCaret`(末) | 0.001 | 0.011 | 0.125 | 🟡 O(n) |
| `GetPositionForOffset` | 0.001 | 0.014 | 0.139 | 🟡 O(n) |
| `BoxesForRange`(末小段) | 0.001 | 0.015 | 0.126 | 🟡 O(n) |

**实测（DejaVuSans 16pt，每行 ≈36 字节，10 次调用取均值）**：

| API | 201 行 | 2001 行 | 20001 行 | 增长倍数（10× 行 → ?× 耗时） |
|---|---|---|---|---|
| `CaretForOffset`(末尾) | 0.031 ms | **3.14 ms** | **365.9 ms** | 200→2000：**102×** ／ 2000→20000：**116×** |
| `BoxesForRange`(全文) | 0.073 ms | **3.78 ms** | **377.6 ms** | 200→2000：**52×** ／ 2000→20000：**100×** |

> **结论**：O(n²) **完全确认**——行数 ×10，耗时 ×100（理论 O(n²) 特征）。2 万行下单次调用 **≈366 ms**，用户感知为「点一下卡死近 0.4 秒」。

**根因**：
- `CaretForOffset`（`text_layout.go:241-260`）：先调 `GetOffsetForCaret` 拿到 y，再 `for i := range l.Lines` 逐行调 `LineTop(i)`（本身 O(i)）反查行号 → **O(n²)**。2 万行 = 2 亿次累加。
- `BoxesForRange`（`text_layout.go:396-453`）：`for i, ln := range l.Lines` 扫全表，每行内嵌 `for _, c := range ln.Carets` 线性找 X → **O(n²)**。
- **补充**：`RenderText.Paint` 的逐字绘制分支（`text.go:906-913`）对每个 rune 线性扫 `ln.Carets` 找 X，是**第三处** O(n²)，面 2 清单需覆盖（`text.go` 而非 `text_layout.go`）。

**关键认知**：这两个是**公开方法内部的实现问题**，与外部怎么调**无关**。就算面 1 全部迁移到新 API，内部不改照样 O(n²)。

#### 面 3 · `Generation` 语义不足

`Generation` 是**全局自增计数器**（`textLayoutGen++`，`text_layout.go:55` 声明、:80/:133/:277/:386/:389 五处递增）。

**问题**：它只能表达「整体变了」，无法表达「只有第 5 行变了」。懒加载 + 行/段缓存后，需要**按行/段的失效标记**，否则缓存只能全量作废 → 缓存等于没做。

**🔴 实测有 2 处消费者**：

| 消费者 | 位置 | 断言内容 | 后果 |
|---|---|---|---|
| `TestTextLayout_Generation` | `ui/rendering/text_layout_r2_test.go:211-237` | 同参两次 `BuildTextLayout` 返回的 `Generation` **必须递增**；不同 `MaxWidth` 的两个 layout 也须递增 | 若把 `Generation` 改为「按行失效标记」而不再全局自增 → **此测试立即红** |
| `ui_wr_ime_r2_textlayout` | `examples/ui_wr_ime_r2_textlayout/main.go:307` | 比较 `lay36.Generation` 与 `tFallback.TextLayout().Generation` | 同上，真窗断言失败 |

> **注**：`render/` 下另有大量 `GenerationID()`（Pixmap/Atlas 的缓存键），与 `TextLayout.Generation` **是不同东西**，不受影响。

**这改变了面 3 的改造方式（硬约束）**：
- ❌ **不可行**：直接把 `Generation` 语义改成按行失效标记（会同时打破 1 个老测试 + 1 个真窗断言；而 `TestTextLayout_Generation` 正是 M1-d2「13 个老测试一行不改全绿」的裁判之一 → **d2 会熔断**）。
- ✅ **可行做法**：**保留 `TextLayout.Generation` 为全局自增计数器（语义不变）**，**另增**一个按行/段的失效标记字段（如 `LineGeneration []uint64` 或行缓存内部自带版本号）。
- **理由**：d2 的硬约束是「d1 与 d2 之间不得修改任何测试文件」，故 M1 内**不允许**为适配改造而改测试。保留旧字段 + 新增字段是唯一不违反 d2 的路径。

#### 面 4 · 绘制侧的平行实现 —— 🔴 单源纪律现已破损（真实 bug）

`RenderText` 有**另一套** line 概念：`DisplayLines()` / `DisplayText()` / `MeasureWidth()` / `CaretColumn()`，走 `wrapLines()` / `estimateWrapLines()` / `layoutRunLines()`，**与 `TextLayout` 不是同一条路径**。

**实测确认的两个真实 bug**：

| 场景 | 绘制侧 `DisplayLines()` | 布局侧 `TextLayout.Lines` | 后果 |
|---|---|---|---|
| **MaxLines=2 + Ellipsis** | **2 行** | **12 行** | caret 查询可能落到**已被裁剪不可见**的行；`MaxLines` 对布局侧**完全未生效** |
| **无 Face（估算路径）** | **11 行** | **1 行** | 绘制 11 行、布局只认 1 行；点击第 5 行 → 光标全落第 1 行 |

对照：`MaxWidth=300` 有 face（回绕）→ 4v4 一致；不回绕单行 → 1v1 一致。

> **现状**：`MaxLines` 截断**只在绘制侧 `DisplayLines()` 生效**（`text.go:631-634`，`if maxL > 0 && len(lines) > maxL { lines = lines[:maxL] }`）；`BuildRenderTextLayout` 的**多 run 路径有一句 mirror 注释但只截断不处理省略号**（`text_layout.go:380-383`），**单串路径根本没有对应截断**。即：**「限制 2 行」这个语义，绘制遵守、布局无视**，二者行数差 6 倍。这不是「偶尔不一致」，是**两条路径对同一份配置的理解不同**。

**结论**：A1 单源纪律**当前即已破损**，不是懒加载引入的，是存量缺陷。M0 必须修复，否则后续所有坐标优化都建立在裂缝上。

#### 四个面的处理顺序（依赖决定）

```
面4（单源修复）→ 面2（方法去O(n²)）→ 面1（字段封装）→ 面3（失效标记）
   M0              M1                M1            M1
```

**理由**：
- **面 4 最先**：两条路径算出不同的行，任何坐标优化都是空中楼阁。
- **面 2 次之**：它是纯实现问题，不依赖外部调用改造，且危害最大（375ms）。
- **面 1 再次**：外部调用改造，量大但难度低于面 2。
- **面 3 最后**：失效标记需在缓存结构确定后才有意义。

---

## 4. 「任意场景」场景矩阵（验收覆盖率要求）

每个场景必须在 **M5 收口**时有**独立单测 + 至少一次真窗实测**，并在 **M6 验收主窗（§5.6）**上有对应场景区。

| 维度 | 覆盖要求 | 当前状态 | 负责阶段 | M6 对应区 |
|---|---|---|---|---|
| **脚本** | 拉丁 / CJK / 阿拉伯 / 希伯来 / 天城文 / 泰 / 缅 / 高棉 / 藏 | bidi 有（UAX#9）；阿拉伯连接 + 印度系重排 GSUB 有；**泰/缅/高棉/藏未验（系统无字体，需引入测试字体）** | M5 | A/D |
| **组合字符** | 组合音标、ZWJ emoji 序列、代理对 | ⚠️ **已有 `snapToGraphemeBoundary`（`text_layout.go:470`）但名不副实**——它只做 UTF-8 续字节（0x80–0xBF）归位，**不识别组合音标/ZWJ 序列**；且**仅在 `GetOffsetForCaret` 一处调用**，`GetPositionForOffset`（点击定位）与上下键移动**均未调用** | **M1**（并入，范围扩大） | A |
| **彩色字形** | COLR/CBDT emoji | ⚠️ 有 `render_color_emoji` 示例，与排版 run 级集成待确认 | M5 | A |
| **富文本** | 段内多字号/多字体/多颜色 | ⚠️ `layoutRunLines`（`paragraph.go:255`）有雏形，caret 精度待验 | M5 | D |
| **长度** | 1 字 ~ 1e6 字；1 行 ~ 1e6 行 | ❌ 5000 字 2.35 ms/次；**2 万行 `CaretForOffset` 375ms**；**单 face 5000 字 34 ms**（见 §2.1 A-2） | **M1（重排 + 面2 去 O(n²)）/ M4（行数）/ M3.5（编辑，条件触发）** | B/C/E |
| **行数规模** | 200 / 2000 / 20000 行的查询不退化 | ❌ **O(n²)**，实测 2 万行单次查询 **≈366 ms** | **M1（面 2）** | E |
| **单源一致性** | 绘制行 == 布局行 | ❌ **已破损**（实测 MaxLines **2v12** / 无 Face **11v1**） | **M0（面 4）** | C/F |
| **DPR** | 1x / 1.25x / 2x / 3x，亚像素定位 | ✅ 量化**已实现**（`glyph_mask_atlas.go:63` 的 `SubpixelXQ2/YQ2`，1/4 px）→ **本项为验证，非新做** | M5（验证） | A |
| **回绕** | 定宽回绕 / 不回绕 / 截断 / 省略号 | ⚠️ 省略号二分 O(n log n)；回绕下编辑级联作废（约束 ②） | M1（缓存）/ M5（省略号） | F |
| **滚动** | 定行高 / 变行高 | ❌ 无行虚拟化 | M4 | E |
| **GPU 约束** | 顶点数 / 批次数 / 图集容量 | ❌ 逐字提交；`cullGlyphs` 条件恒 false，**实测至今一次未生效** | M2 | B/C |

**📊 「卡不卡」的量化分级**

> 立项时写「5000 字已卡」，该定性**偏强**。以一帧 16.7 ms 为基准实测分级：

| 字数 | `InputBox.Sync` 全链路 | 占一帧 | 判定 |
|---|---|---|---|
| 5000 字 | **2.64 ms** | **15.8%** | ⚠️ 有感知余量，**尚不构成"卡"** |
| 20000 字 | ~11.3 ms | **68%** | 🔴 明显掉帧 |
| 50000 字 | **29.09 ms** | **174%（超一帧）** | 🔴 **确定卡死** |

> **结论**：本方案的立项依据**依然成立**，但**真正的痛点在 2 万字以上**（M6 验收主窗的 B 框 5000 字 / C 框 50000 字设计正好覆盖这两个档位，见 §5.6.2）。
> **不要再用「5000 字已卡」当主要论据**——它只占一帧 16%，容易被人反驳。改用「**50000 字超一帧 1.7 倍**」或「**单 face 5000 字 34 ms（超一帧 2 倍）**」这两个更硬的数。

**「长度」维度的阶段分工**：
- **M1** 解决**排版**复杂度（O(n²) → O(L)/O(段长)）——这是 5000 字卡顿的主因
- **M4** 解决**行数**维度（1e6 行的首屏与滚动）
- **M3.5**（条件触发）解决**编辑时文本拷贝**——仅在实测证明拷贝成为瓶颈时才做

> 注：M3 只做增量 undo（Q3 决策），不解决长度问题。

---

## 5. 分期施工

> **通用规则**（每期均适用）：
> 1. 每期开工前先补**失败的单测/基准**，红灯起步；
> 2. 每期结束必须**三证据合证**：单测全绿 + 基准门禁达标 + 真窗 A–J 全族；
> 3. 任一期验收不过，**不得进入下一期**，且**禁止放宽阈值通过**；
> 4. 改动 render 公开 API 必须同步 `docs/RENDER_API_CATALOG.md` 并跑 `go run ./scripts/apidoc`（exit 0）；
> 5. 每期留独立回滚点（独立包或编译开关），失败可退回上期。

---

### M0 · 单源修复 + 度量地基（面4 绘制收敛 · advance 缓存 · run 切分 · 绘制同源）

**① 本期目标**：修复面 4（绘制侧与布局侧单源收敛）+ 修通整形链路 + 消除布局/绘制发散，单字量宽 O(1) 摊还。不改任何公开 API 语义（新增公开 API 除外）。

**② 前置依赖**
- **文档**：需已读 §0 硬目标、§1 术语、§2 根因、§3.5 约束①②③、§3.6 面 4。
- **🔴 前置任务 M0-pre：先建验收主窗**（**必做，不得跳过**）

  > **为什么必须先建**：M0–M5 **每一期**的验收清单都要求跑 `ui_text_edit_accept`（§5.6）记录趋势值，但该窗**当前不存在**（`examples/` 下无此目录）。若不先建，M0 收口时无窗可跑 → 验收卡死。

  | 步骤 | 做什么 | 判据 |
  |---|---|---|
  | pre-1 | 新建 `examples/ui_text_edit_accept/`，按 §5.6.2 摆 6 个场景区（A–F），控件**只用** `ui/textinput` 现成的 `NewInputBox` / `NewViewportInputBox` / `NewMultiLineInputBox`（三者均**已存在**），**禁止在 `examples/` 里实现输入框功能**（`AGENTS.md` 硬纪律） | 编译通过、6 个区可见 |
  | pre-2 | 接 §5.6.3 的自动探针，至少输出 `keystroke_p99_ms` / `caret_vs_paint_max_delta_px` / `layout_ms_p99` / `paint_ms_p99` / `scroll_fps` | 出 JSON |
  | pre-2b | **预填文本落 `testdata/`**：B 框 5000 字、C 框 50000 字、E 框 1e5 字（~2000 行）的预填内容**一律存文件**（`examples/ui_text_edit_accept/testdata/`），或复用已有字表（如 `render/text/hint/testdata/cjk3000.txt`）。**禁止在 `main.go` 里用 range 循环生成**——守 `AGENTS.md`「测试数据一律入 testdata、禁硬编码字表」 | `testdata/` 下有文件；`main.go` 无生成循环 |
  | pre-3 | **跑一次，记录全部基线值**（此时预期很差：`caret_vs_paint_max_delta_px` 应 ≈ 数百 px，正是约束 ① 的发散） | 基线入 README 趋势表 |
  | pre-4 | 补 §5.6.5 像素断言（F0+F6+F7）与 Golden 基线 | Golden 落盘 |

  **规模提示**：pre-1~pre-4 是**摆控件 + 接探针**，不含引擎改动，预计独立可完成；若探针项过多，可**先只接 `caret_vs_paint_max_delta_px` 一项**（M0 熔断只依赖它），其余随后续各期补齐。

  **⚠️ 顺序约束**：pre-3 的基线**必须在改任何代码之前**记录——一旦开始改 M0，发散值就会变，基线再也取不到。

**③ 背景与根因**（引用实测，非推测）
- **§2.2 根因链 ①②**：`MultiFace.Source()` 返回 `nil`（`render/text/multi.go:164`）→ shaper 进门即 `return nil`（`shaper_hb.go:55-57`）→ `Shape(MultiFace)` 返回 **0 glyph**（单 face 同串返回 12）→ 布局退化逐字 `RuneAdvance`（占 profile 76%）。
- **§3.5 约束 ①**：绘制 `snapPen` 逐字取整累加 vs 布局浮点累加，5000 字发散 **976.6px**（每字 0.195px，确定性线性）。
- **§3.5 约束 ③**：`MultiFace.Runs()` 已存在（`multi.go:355`）且带缓存。注：其切分能力可用（混排切 6 run），但**不做 script/bidi 分段**（阿拉伯+拉丁只切 1 run 且不整形）→ M0 须**叠加 `segment.go`**，非纯「接线」。

**③ 补充背景：面 4（单源纪律现已破损）**
- **§3.6 面 4 实测**：`RenderText.DisplayLines()`（绘制用）与 `TextLayout.Lines`（查询用）**不是同一条路径**，两个真实 bug——
  - `MaxLines=2 + Ellipsis`：绘制 **2 行** vs 布局 **12 行** → `MaxLines` 对布局侧完全未生效
  - 无 Face（估算路径）：绘制 **11 行** vs 布局 **1 行** → 点击第 5 行光标全落第 1 行
- 这是**存量缺陷**，不是懒加载引入的。**必须最先修**，否则后续所有坐标优化都建在裂缝上。

**④ 改动清单（精确）**

| # | 面 | 文件:行号 | 改什么 |
|---|---|---|---|
| 1 | 面4 | `ui/rendering/text.go:603` `DisplayLines` / `:665` `DisplayText` / `:526` `MeasureWidth` / `:562` `CaretColumn` + `ui/rendering/text_layout.go`（`TextLayout` 新增 `MaxLines` + `Overflow/Ellipsis` 字段） | **单源收敛**：先给 `TextLayout` 补截断能力（单串与多 run 两个构建路径统一算截断宽度；省略号在布局侧生成替换字形并同步排除 caret；无 Face 估算回绕也在布局侧补齐），再把这四个 API 改为**从 `TextLayout` 派生**，删除 `wrapLines()` / `estimateWrapLines()` 的独立行计算。**实现路径见下方「面 4 落地要点」**（`TextLayout` 当前**缺 `MaxLines` 截断能力**，需先补，否则无法派生） |
| 2 | 面4 | `ui/rendering/paragraph.go:255` `layoutRunLines` | 多 run 路径也收敛到 `TextLayout`（当前已有独立行计算） |
| 3 | 面4 | `ui/rendering/text.go` 新增 | 补 `TestDisplayLinesMatchLayout_*`：MaxLines+Ellipsis、无 Face 估算、多 run、回绕 四种场景下 `DisplayLines()` 与 `TextLayout.Lines` **行数与内容必须一致** |
| 4 | — | `render/text/` 新建 `advance_cache.go` | 新增 `(fontID, glyphID, sizeQ8, hinting, variations)` → advance 缓存（key 必须含 hinting/variations，缺一项即撞车）；容量上限 + LRU，范式照 `shape_result_cache.go` |
| 5 | — | `render/text/face.go:118` `sourceFace.Advance` | 接入 advance 缓存（替代逐字 `parsed.GlyphAdvance`） |
| 6 | — | `render/text/draw.go:435` `runAdvance` / `:475` `advanceFromGlyphs` | 两处统一走 advance 缓存 |
| 7 | — | `render/text/face.go:70` `RuneAdvance` | 接入同一缓存（稳态 **~430 ns/字**；目标降至 ≤ 250 ns 且消抖动） |
| 8 | — | `ui/rendering/text_layout.go:166` `buildCaretsForLine` | 改用 **`MultiFace.Runs()` + `segment.go` script/bidi 分段**切 run（约束③：仅 `Runs()` 不足以处理阿拉伯文）→ 每 run 整段 `text.Shape` → 拼接 caret；**仅**消除 `:190-206` 的「有 face 但 Shape 返 0」兜底分支。**⚠️ `:170-188` 的 `face==nil` 估算路径必须保留，见下方要点**。同时必须消除 `:222-231` 的 O(n²) 扫表（见 §2.2 ③）。**🔒 同提交硬锁：通整形与扫表消除是第 8 项内部的两个半步，必须在同一提交合入、同一基准验证（`BenchmarkCaretBuild`），不得分两次提交** |
| 9 | — | `render/text/draw.go:113` `drawGlyphs`、`:229` `drawGlyphsVariable`、`:309-312` `rasterizeHintedGlyph` | **对策 C**：绘制位置统一取整形结果的 `glyph.X`（与布局同源），消除发散（976.6px/5000 字）。**⚠️ 路径澄清见下方要点** |
| 10 | — | `ui/rendering/text.go:855` `RenderText.Paint` | 绘制路径同步改用 `glyph.X`（与第 9 项同源） |

#### 第 8 项路径要点（必读 · 删错分支会拆掉 M1 的裁判）

> **问题**：第 8 项字面写「消除逐字 fallback 分支」，但 `buildCaretsForLine` 里有**两条**兜底路径，只有一条该删。

| 分支 | 位置 | 触发条件 | 该不该删 |
|---|---|---|---|
| **`face == nil` 估算路径** | `text_layout.go:170-188`（adv 固定 10.0/rune） | 调用方传 `nil` face | ❌ **必须保留** |
| **有 face 但 `Shape` 返 0** | `text_layout.go:190-206`（逐字 `RuneAdvance`） | `MultiFace` 等 `Source()` 为 nil 的 face | ✅ 本期消除（修通整形后不再触发） |

**为什么 `face == nil` 路径不能删**：M1-d2 依赖的 13 个老测试中，**`TestTextLayout_36Deep_Roundtrip`、`TestTextLayout_StickyFallback`、`TestTextLayout_Affinity_Newline` 三个都用 `BuildTextLayout(txt, nil, ...)` 调用**。删掉该路径 → 这 3 个测试立即崩（返回空 layout）→ **M1-d2 的裁判体系在 M0 阶段就被拆掉**，且崩因隐藏（要等到 M1 才暴露）。

**正确做法**：只消除 `:190-206`；`:170-188` 原样保留。M0 验收不因保留它而受影响。

**叠加算法（第 8 项必读 · 否则阿拉伯文仍不整形）**：`MultiFace.Runs()` 按字形有无切出的区间，与 `segment.go` 按脚本/方向切出的区间**取相交**得最终 run 边界；每个最终 run 独立 `text.Shape`；RTL 段按视觉序重排后再拼 caret。只做其中一层 = 切不干净。

#### 第 9–10 项路径要点（必读 · 否则改错地方）

> **问题**：第 9 项字面写「改 `snapPen`」，但 **`snapPen` 不在 M0 验收要量的那条路径上**。照字面改会发现偏差纹丝不动，误判为「对策 C 失效」。

**实测两条绘制路径**：

| 路径 | 触发条件 | 位置 | 用不用 `snapPen` | 与约束 ① 的关系 |
|---|---|---|---|---|
| **GPU 批量路径**（主路径） | `face.Source() != nil` 且 glyph 有效 | `text.go:905` `pc.DC.DrawShapedGlyphs(glyphs, face, ax, ay)` → `render/text.go:212` → GPU 加速（`DrawShapedGlyphMaskText`）或 `drawShapedGlyphsAsOutlines` | ❌ **完全不经 `drawGlyphs`/`snapPen`** | 坐标直接取 `glyph.X`，**本身已与布局同源** |
| **CPU 逐字兜底** | `face.Source() == nil`（当前 `MultiFace` 正落这里，因 `Shape` 返 0 glyph） | `text.go:906-921` 逐字 `pc.DC.DrawString` | ✅ 经 `snapPen` 取整累加 | **发散的真实来源** |

**因此第 9–10 项的正确理解是**：
1. **主目标**不是"改 `snapPen`"，而是**让 `MultiFace` 场景也走通 GPU 批量路径**（当前因 `Shape` 返 0 glyph 而落到 CPU 兜底分支）。修通后坐标自动取 `glyph.X`，发散消失——**这才是对策 C 的实质**。
2. `draw.go` 的 `snapPen` 三处（`:113`/`:229`/`:309-312`）属于 **CPU 光栅兜底**，只有在无 GPU 加速时才生效。**可作为一致性清理，但改它不会让 `TestPaintUsesShapedX_*` 的 5000 字偏差数值变化**。
3. **验收判据不变**：`caret_vs_paint_max_delta_px` 须从 M0-pre 基线降至 ≤ 2px——但**达标靠的是"让批量路径跑通"（第 8 项 + 第 10 项），不是靠改 `snapPen`**。

**执行顺序**：**先 1–3（面 4 单源修复），再 4–10**。面 4 是地基，未修复前不要动坐标相关改动。

**⑤ 复杂度契约**

| 操作 | 改前 | 改后 |
|---|---|---|
| 整串量宽 | O(n) × 单字解析（稳态 ~430 ns） | O(唯一 rune 数)，命中 O(1)/rune |
| 布局 caret 构建 | O(n²)（成功路径线性扫表）／O(n)（MultiFace 兜底逐字） | **O(n)**（run 批量整形 + 单次累积） |
| 布局 vs 绘制偏差 | 5000 字 **976.6px** | **≤ 1px** |
| **绘制行 vs 布局行** | **不一致（2v12 / 11v1）** | **完全一致** |

> **⚠️ M0 的「先慢后快」风险**：当前 `MultiFace` 路径因 `Shape` 返回 0 glyph 而走**兜底 O(n)** 分支；M0 修通整形后会**进入此前从未执行过的成功路径**，而该路径藏着 **O(n²) 的 `CaretXForCluster` 扫表**（§2.2 ③）。因此 **M0 第 8 项必须与 O(n²) 消除同时做**，不得拆成两步——否则会出现「修完击键反而更慢」的回退。

#### 面 4 落地要点（第 1 项必读 · 否则无从下手）

> **问题**：第 1 项要求「`DisplayLines()` 从 `TextLayout` 派生」，但**当前 `TextLayout` 根本没有截断能力**——直接派生会把「限制 2 行 + 省略号」这个语义**整个丢掉**。新会话照字面做会卡住。

**现状实测**：

| 能力 | 绘制侧 `DisplayLines()` | 布局侧 `TextLayout` |
|---|---|---|
| `MaxLines` 截断 | ✅ 有（`text.go:631-634`，`lines = lines[:maxL]`） | ⚠️ **仅多 run 路径有**（`text_layout.go:380-383`）；**单串路径完全没有** |
| `Ellipsis` 省略号 | ✅ 有（`text.go:641-651`，`ellipsizeToWidth`） | ❌ **无**（多 run 路径注释明说「caret 不需要」而跳过） |
| 无 Face 估算回绕 | ✅ 有（`estimateWrapLines`，`text.go:669`） | ❌ **无**（该路径下布局恒 1 行） |

**结论：`TextLayout` 不是「已有全部能力、绘制侧重复实现」，而是「绘制侧有三样布局侧没有的能力」。单纯删除绘制侧实现 = 功能倒退。**

**正确做法（三选一，按推荐度排序）**：

| 方案 | 做法 | 代价 |
|---|---|---|
| **✅ 推荐：能力上移** | 把 `MaxLines` 截断、`Ellipsis`、无 Face 估算**三样能力补进 `TextLayout` 构建流程**（`TextLayout` 增 `MaxLines` 字段 + `Overflow`，单串/多 run 两条构建路径统一算截断宽度，省略号在布局侧落字形并排除 caret），然后 `DisplayLines()` 改为纯粹从 `TextLayout` 读结果 | 改动最大，但**一步到位符合 I7**，且修复 2v12/11v1 两个 bug |
| 备选：派生 + 后处理 | `DisplayLines()` 从 `TextLayout` 取行，截断/省略号**仍在绘制侧做** | 行**内容**同源了，但行数仍可能不等 → **不满足 I11**，仅作临时台阶 |
| ❌ 不可取：直接删 | 删掉 `wrapLines()`/`estimateWrapLines()`，直接读 `TextLayout` | 丢掉 `MaxLines`/`Ellipsis`/无 Face 三项能力 → **功能倒退**，禁止 |

**验收判据（写死，防偷懒）**：`TestDisplayLinesMatchLayout_*` 四场景必须**行数与内容均一致**。若只做到「内容一致、行数不等」，视为**未通过**。

**⑥ 验收清单**
- 单测 `TestDisplayLinesMatchLayout_*`（**面 4 核心**）：MaxLines+Ellipsis / 无 Face 估算 / 多 run / 回绕 四场景下 `DisplayLines()` 与 `TextLayout.Lines` **行数与内容逐字节一致**
- 单测 `TestAdvanceCache_*`：命中 / 淘汰 / 容量上限 / 换字体失效 / 换字号失效
- 单测 `TestItemizeRuns_*`：拉丁+CJK 混排切成多 run（6 run ✅）；**阿拉伯+拉丁必须切成 ≥2 run 且阿拉伯段 `Shape` 返非 0 glyph**（当前只切 1 run 且不整形，此项为**回归锁**）；缺字场景正确切分
- 单测 `TestIntersectRuns_*`（叠加合并器）：`Runs` 区间与 script/bidi 区间相交合并正确；RTL 段视觉序重排正确；单测失败即第 8 项未完成
- 单测 `TestPaintUsesShapedX_*`（约束①）：绘制坐标取自 `glyph.X`，与布局坐标偏差 **≤ 1px**（5000 字规模验证发散消失）
- 单测 `TestShapeNonEmpty_MultiFace_*`（根因① 回归锁）：`Shape(MultiFace, 5000字)` 返回 **非 0** glyph
- 基准 `BenchmarkShapeLine`：N ∈ {1e3, 1e4}，整形耗时下降 **≥ 5×**
- 基准 `BenchmarkRuneAdvance`：单次 **≤ 250 ns**（稳态 ~430 ns；同时断言 **P99 不劣化**，防缓存引入抖动；缓存 key 必须含 hinting/variations，换设置必须失效）
- 基准 `BenchmarkCaretBuild`（防「修完更慢」）：N ∈ {1e3, 1e4, 1e5}，caret 构建耗时比值 **≤ 1.5**，确保 M0 未把 O(n) 兜底换成 O(n²) 成功路径
- 回归：`render/text` 全量 + `ui/rendering` 全量 + `ui/textinput` 全量绿
- 像素：`render/text` 现有 golden **逐位一致**
- 真窗：`ui_text_edit_accept`（§5.6）跑一次，记录 `caret_vs_paint_max_delta_px` 基线值

**⑦ 熔断条件**
- **面 4 修复后 `DisplayLines` 与 `TextLayout.Lines` 仍不一致 → 停**。这是地基，宁可多花时间。（注意：`MaxLines` 截断目前**仅绘制侧生效**，修复时须保证布局侧同步截断，见 §3.6 面 4 说明。）
- `MultiFace` 公开行为有任何变化 → **停**，重新设计接口后再动。
- **像素 golden 出现任何差异 → 停**。整形通路切换是高风险改动，宁可退回逐字 fallback 也不接受静默画质变化。
- `caret_vs_paint_max_delta_px` 未下降 → 停，对策 C 未生效。
- **🔴 `BenchmarkCaretBuild` 未达标（修完击键反而更慢）→ 停**。这是「O(n) 兜底换成 O(n²) 成功路径」的信号，第 8 项必须连同 `CaretXForCluster` 扫表一起改，**且在同一提交内**，不得只通整形。
  > **⚠️ 这是实测确认的必现风险**：同 5000 rune 语料下，`MultiFace`（当前路径，走 O(n) 兜底）**2.290 ms**，而 `singleFace`（**M0 修通后的成功路径**，走 O(n²) 扫表）**34.057 ms**，**慢 14.9 倍**（其中 98% 来自 `CaretXForCluster` 全表扫描）。
  > **即：M0 若只通整形而不消除扫表，击键耗时将从 2.29 ms 跳到 ~34 ms**。见 §2.1 A-2 与变更记录 §11.0 第 13 项。

**⑧ 回滚方式**
- 第 1–3 项（面 4）：`DisplayLines` 等四处恢复独立行计算。**注意：这会让两个已确认 bug 复现**，仅在无法推进时作为临时退路，且必须在文档记录。
- 第 4–7 项（advance 缓存）：`advance_cache.go` 整文件删除 + 三处调用点 revert。纯加法，回滚无副作用。
- 第 8–10 项（run 切分 + 对策 C）：**三项耦合**，须整体回滚。只回滚 9–10 而保留 8，发散会以新形式出现。**第 8 项内部两半（通整形 + 消扫表）同样耦合，只回滚其中一半禁止合入**。

**⑨ 交接说明（给 M1）**
- **新不变量**：
  - **I1**：`glyph.X` 是布局与绘制的唯一位置真源，二者不得各自累加。
  - **I7（新增）**：`RenderText.DisplayLines()` 等绘制侧 API **必须从 `TextLayout` 派生**，禁止独立行计算。
- **新 API**：`render/text` 新增 advance 缓存（内部，非公开 API，无需进 `RENDER_API_CATALOG.md`）。
- **新增禁区**：禁止在 caret/hit 路径重新引入逐字 `RuneAdvance` 累加（违反 A1 单源纪律）。
- **给 M1 的前置**：M1 的前缀和必须建在 **`glyph.X` 同源的坐标**上，否则约束 ① 复发。

---

### M1 · 去 O(n²) + 布局内核（面2 查询加速 · 行索引 · 前缀和 · 面3 失效标记 · 面1 迁移）

**① 本期目标**：消除面 2 的 O(n²) 查询（`CaretForOffset`/`BoxesForRange`/`LineTop`）；把击键重排从 O(n²) 降到 O(行长)/O(段长)；建立面 3 的按行失效标记；完成面 1 的字段迁移并拆掉老门。**这是收益最大的一期。**

**② 前置依赖（硬）**
- **M0 必须已完成**，尤其 **M0 第 8 项**（run 批量整形，让 `Shape` 返非 0 glyph）与 **第 9–10 项**（绘制用 `glyph.X`，坐标同源）。M1 的前缀和必须建在 `glyph.X` 同源坐标上，否则约束 ① 复发。
  > ⚠️ **注意**：**M0 第 5–7 项是 advance 缓存**（性能项，非 M1 的正确性前置）；**run 批量整形是第 8 项、绘制用 `glyph.X` 是第 9–10 项**。照错误编号验收会漏掉真正的前置。
- 需已读 §0、§1、§2.2 根因③、§3.5 约束①②、§9.1 Q1/Q2/Q4/Q5 决策（M1-c 开工前 Q5 必须已定，否则不得动手）。

**③ 背景与根因**
- **§2.2 根因链 ③**：`buildCaretsForLine`（`text_layout.go:222-231`）对每个 rune 调 `CaretXForCluster`，该函数每次从头线性扫 glyph 数组（`render/text/shaped.go:125-160`）→ 建 n 个 caret 扫 n 次 = **O(n²)**，5000 字 = 1250 万次比较。
- **§3.5 约束 ②**：回绕模式（`MaxWidth > 0`）下编辑会级联作废后续所有行（改首行后 L1 由 `[35,71) "lazy dog and then keeps running far "` 变 `[30,62) "fox jumps over the lazy dog and "`，L2 由 `[71,75)` 变 `[62,89)`）→ 回绕下「只作废第 K 行」不成立。
- **实测依据**：不回绕下单独排版第 2 行与全文内第 2 行宽度**差值 0.000000**（170.593750 vs 170.593750）→ 行隔离在不回绕模式成立。

**③ 补充背景：面 2（方法内部 O(n²)，实测）**
- **§3.6 面 2 实测**：2 万行文档下单次调用 `CaretForOffset` = **365.9 ms**、`BoxesForRange`(全文) = **377.6 ms**；行数 ×10 → 耗时 ×100，**O(n²) 确认**。
- `CaretForOffset`（`text_layout.go:241-260`）外层 `for i := range l.Lines` 内调 `LineTop(i)`（自身 O(i)）→ O(n²)。
- `BoxesForRange`（`text_layout.go:396-453`）扫全表 + 每行内嵌 `for _, c := range ln.Carets` 线性找 X → O(n²)。
- **第三处**：`RenderText.Paint` 逐字分支（`text.go:906-913`）每 rune 线性扫 `ln.Carets` 找 X → O(n²)。
- **这是纯实现问题，与外部调用无关**——面 1 全迁移也救不了。

**④ 改动清单（精确）**

| # | 面 | 文件:行号 | 改什么 |
|---|---|---|---|
| **1** | **面2** | `ui/rendering/text_layout.go:241-260` `CaretForOffset` | **去 O(n²)**：去掉循环内 `LineTop(i)`，改为行索引二分定位 + 行高前缀和 O(1) |
| **2** | **面2** | `ui/rendering/text_layout.go:396-453` `BoxesForRange` | **去 O(n²)**：改为行索引定位区间 + 行内前缀和二分定位 X，不再扫全表 |
| **3** | **面2** | `ui/rendering/text_layout.go:138` `LineTop` / `:154` `LineHeight` | 加**行高前缀和**（定行高时 O(1)，变行高前缀和 O(log n)） |
| 4 | 面2 | `render/text/shaped.go:125` `CaretXForCluster` | 保留公开签名（有外部调用：`bidi_reorder_test.go`、`text_layout.go:223`），**内部改为前缀和查表** |
| 5 | — | `ui/rendering/text_layout.go` 新增 `line_index.go` | 行起始偏移数组（增量维护），支持 O(log n) 的「偏移→行号」「行号→偏移」 |
| 6 | — | `ui/rendering/text_layout.go:220-237` `buildCaretsForLine` | caret 构建改为**累积宽度前缀和数组**（一次遍历） |
| 7 | — | `ui/rendering/text_layout.go:500` `GetOffsetForCaret` | 前缀和 O(1) 取 X + 行索引 O(log n) |
| 8 | — | `ui/rendering/text_layout.go:597` `GetPositionForOffset` | 行索引 O(log n) + 行内前缀和二分 O(log L) |
| 9 | — | `ui/rendering/` 新增 `layout_cache.go` | **双模式布局缓存**：不回绕→行缓存；回绕→**段缓存**。key = 内容哈希 + face + size + maxWidth + hinting/variations（缺一项即撞车） |
| 10 | 面3 | `ui/rendering/text_layout.go:34` `Generation` | **保留 `Generation` 全局自增语义不动**（有 2 处消费者，见 §3.6 面 3），**另增**按行/段失效标记字段供缓存使用。**禁止**把 `Generation` 本身改语义——会打破 `TestTextLayout_Generation`（M1-d2 的裁判之一） |
| 11 | — | `ui/rendering/text_layout.go:470` + `:597` + `ui/textinput/editor.go:937` `SetCaretWithAffinity` + `ui/textinput/visual_move.go` | **簇边界**：现有 `snapToGraphemeBoundary` **名不副实**（只做 UTF-8 续字节归位，不认组合音标/ZWJ），须升级为 **UAX#29 字素簇**。**⚠️ 依赖见 §9.1 Q5（默认 A，M1-c 开工前最终确认），`go.mod` 单独提交，见下方要点** |
| 11-b | — | 同上 | **范围补全**：簇吸附只在 `text_layout.go:508` 一处被调用；`editor.go:937` `SetCaretWithAffinity` 有**同名独立实现**（同样只做续字节归位），点击定位（`GetPositionForOffset`）与上下键（`visual_move.go`）**均未吸附**。四处须一并改，只改一处会「点击修好了但 IME 置位没修好」 |
| 12 | 面1 | `ui/rendering/text_layout.go:28-37` + 22 处生产代码 + 23 处示例探针 | **Q4 决策落地（选项 4 · 先绿后拆）**：新增惰性 API + 生产代码换新 + 老测试当裁判 + 示例迁移 + 拆老门（详见下方执行顺序） |
| **13** | **面2** | `ui/rendering/text.go:906-913`（`Paint` 逐字分支每 rune 扫 `ln.Carets` 找 X） | **去 O(n²)（第三处）**：改为一次构建 byteOff→X 索引后 O(1) 查表 |

#### 第 11 项依赖决策（开工前必须先定 · 否则卡死）

> **问题**：要求「升级为 UAX#29 字素簇」，但**仓库当前没有任何字素簇实现**，文档原未提及依赖问题。新会话做到这里才发现要引第三方库或手写状态机，会直接卡住。

**现状实测**：
- `grep -rn "uniseg\|GraphemeCluster" --include=*.go .` → **空**，无实现
- `go.mod` 仅有 `golang.org/x/text`（含 bidi，**不含** grapheme 包）与 `golang.org/x/image`
- 现有 `snapToGraphemeBoundary`（`text_layout.go:470`）只做 UTF-8 续字节（`0x80–0xBF`）归位，**不是 UAX#29**

**三个选项（开工前必须先定，已记为 §9.1 Q5）**：

| 选项 | 做法 | 代价 | 风险 |
|---|---|---|---|
| **A · 引入 `rivo/uniseg`**（推荐） | UAX#29 参考实现，业界标准（Go 生态主流） | 新增 1 个第三方依赖 | 需确认许可与二进制体积影响；须记入 §9.1 决策表 |
| B · 自研最小集 | 只实现组合音标（ combining marks）+ ZWJ 序列 + 代理对三类规则 | 无新依赖 | 工作量大且易漏规则（如 emoji 修饰符、地区指示符、印度系 virama） |
| C · 降级到 M1.5 | 本期只做「续字节归位覆盖全部 4 条路径」，完整 UAX#29 押后 | 本期不彻底 | 组合音标仍会被劈开，但**不比现状差** |

**推荐 A**，理由：字素簇是文本编辑的基础能力，自研易错；引入成熟实现符合项目「参考成熟框架」的一贯做法。

**若选 A，须同步**：① §9.1 Q5 已记，`go.mod` 变更单独一个提交；② §8.1 家底表补一行（合入时把待引入状态翻为已存在）。

**执行顺序（M1 内部四步，不可乱序）**

```
M1-a  面2 去 O(n²)（第 1–4 项 + 第 13 项） ← 先做，危害最大且不依赖外部改造
M1-b  行索引+前缀和+缓存（第 5–10 项）  ← 建缓存结构与失效标记
M1-c  簇边界（第 11 项）               ← 可独立，出问题拆为 M1.5
M1-d  面1 迁移（第 12 项）· **五小步 d1–d5**：
        d1 新增惰性 API（LineCount()/Line(i)/CaretAt(...)）
           生产代码 22 处换用新 API，老字段 .Lines 留作临时脚手架
        d2 13 个老测试（10 个 text_layout_r2_test.go + 3 个 r1_selftest_visual_test.go）
           【一行不改】跑一遍 → 全绿 = 新实现与老实现等价
        d3 老测试改用新 API 读数据（断言不动），再跑一遍必须仍全绿
        d4 示例探针 23 处迁移到新 API（ui_wr_ime_r1/r2/r4/r5 + ui_textinput_ime）
           ⚠ 必须在 d5 之前——d5 删字段会让这些示例立即编译失败
        d5 删除 .Lines 字段，老门拆掉 —— 全部在 M1 内完成，不留兼容期
```

> **顺序约束（硬）**：原计划把示例迁移排在 M2，但 **d5 删除 `.Lines` 会让 23 处示例探针立即编译失败**。故示例迁移（d4）**必须并入 M1**，排在 d5 之前、d2/d3 之后（等裁判验完再动其余调用方）。

**为什么 d2 不能省**：这 13 个老测试（**10 个** `text_layout_r2_test.go` + **3 个** `r1_selftest_visual_test.go`）验证的是**行为契约**——caret X 往返差 **≤0.5px**（`TestTextLayout_36Deep_Roundtrip`）、**混排不劈簇**（`TestTextLayout_StickyFallback`）、**HiDPI 对齐**（`TestTextLayout_HiDPI_Snap`）、**省略号不进 caret**（`TestTextLayout_Ellipsis_NotInCarets`）——而不是 API 形态。若 d1 与测试改写同时进行，运动员与裁判同时换人 → 失去判断对错的能力。d2 让老测试当**一次独立裁判**。

> **⚠️ 裁判自身的缺口**：`TestTextLayout_StickyFallback` 的注释写明「Carets 数应为 **rune 数+1**（即使 nil face 固定 adv=10 也**不劈**）」——即该测试断言的是「按 rune 切分、不劈开**字形**」，**并非 UAX#29 字素簇**。它无法发现「组合音标被劈开」这类错误。
> **对策**：d2 阶段该测试仍照跑（它是有效的回归锁，只是覆盖范围有限）；**簇边界的正确性由 M1-c 新增的 `TestClusterBoundary_*` 独立把关**，不得指望 d2 的老测试兜底。

**⑤ 复杂度契约**（按模式**分别**书写，不得混用）

| 模式 | 操作 | 改前 | 改后 |
|---|---|---|---|
| 不回绕 | 击键重排 | O(n²) | **O(L)**（L = 行长） |
| 回绕 | 击键重排 | O(n²) | **O(段长)**（段长 << 文档长） |
| 二者 | `CaretForOffset` | **O(n²)**（2 万行 365.9ms） | **O(log n)** |
| 二者 | `BoxesForRange`(全文) | **O(n²)**（2 万行 377.6ms） | **O(输出行数)** |
| 二者 | `LineTop` | O(n) | **O(1)**（定行高）/ O(log n)（变行高） |
| 二者 | 偏移↔坐标 | O(n) | **O(log n)** |
| 二者 | caret 表构建 | O(n²) | **O(n)** |
| 二者 | **逐字绘制 X 查表**（第 13 项） | **O(n²)**（每 rune 扫 carets） | **O(1)**/rune |

二者均须满足：`BenchmarkKeystroke` 的 `T(1e5)/T(1e3) ≤ 5`。**段长分布写死**：回绕模式必须分别测短段（≤200 字/段）与长段（≥5000 字/段），1e6 字按多短段 + 少数长段的混合分布测，不得只用单一长段冒充达标。

**⑥ 验收清单**
- 单测 `TestCaretForOffset_NoQuadratic_*`（**面2 核心**）：N ∈ {2e2, 2e3, 2e4} 行，单行查询耗时比值 ≤ 3（当前 10× 行数 → 100× 耗时：0.031→3.14→365.9 ms）
- 单测 `TestBoxesForRange_NoQuadratic_*`（**面2 核心**）：同上（当前 0.073→3.78→377.6 ms）
- 单测 `TestPaintPerRuneX_NoQuadratic_*`（**第 13 项**）：逐字绘制路径的 X 查表不得随行长二次增长
- 单测 `TestLineTop_PrefixSum_*`：定行高 O(1)；变行高累计正确
- 单测 `TestLineIndex_*`：增删行后索引正确、越界钳制、CRLF、末尾无换行
- 单测 `TestLayoutCache_NoWrap_*`（不回绕）：改第 K 行**只作废 K**
- 单测 `TestLayoutCache_Wrap_*`（回绕，**约束 ② 回归锁**）：**改首段不得作废后续段**
- 单测 `TestGeneration_PerLine_*`（面3）：改第 K 行只让 K 的行/段失效标记变化，其余行标记不变；`Generation` 仍全局自增（以 §3.2.1 I9 为准，禁止断言 `Generation` 按行不变）
- 单测 `TestCaretPrefixSum_*`：与旧实现**逐字节对照**
- 单测 `TestClusterBoundary_*`："e"+U+0301 不劈开；👨‍👩‍👧 ZWJ 整体跳过；代理对移动 2 单元
- 单测 `TestClusterBoundary_ClickAndArrow_*`：**点击定位**（`GetPositionForOffset`）与**上下键移动**（`visual_move.go`）同样不得劈簇——现有实现仅 `GetOffsetForCaret` 一处吸附，另两条路径是缺口
- **单测 `TestLegacyLinesContract_*`（d2 门禁）**：M1-d2 阶段，`Lines` 下标访问对全部 i/j 与旧实现逐字节一致
- 单测 `TestCaretMatchesPaint_*`（**约束① 回归锁**）：前缀和坐标与绘制位置偏差 **≤ 1px**
- 门禁 `grep -rn "RuneAdvance\|MeasureWidth" ui/rendering/ --include="*.go"`：caret/hit 路径出现逐字累加循环即红（守 I6/I8，防偷偷退化）
- 基准 `BenchmarkKeystroke`：**不回绕与回绕各跑一遍**，N ∈ {1e3,1e4,1e5}，均断言 `T(1e5)/T(1e3) ≤ 5`
- 基准 `BenchmarkCaretQuery`：N ∈ {1e3,1e5}，偏移↔坐标查询耗时比值 ≤ 2
- 回归：`ui/textinput` + `ui/rendering` 全量绿；`ui_wr_ime_r1/r2/r4/r5` 行为不变
- 真窗：`ui_text_edit_accept`（§5.6）跑一次，记录 `keystroke_p99_ms`

**⑦ 熔断条件**
- **面 2 的 `CaretForOffset` / `BoxesForRange` 未降到 O(log n) → 停**。这是本期最大收益点，也是 M4 虚拟化的前提。
- 簇边界若影响现有 caret 单测 → 拆为 M1.5 单独做。
- **d2 阶段老测试未全绿 → 停**。不要改测试去迁就实现。
- 回绕模式下出现「改一段导致全文档重排」→ 停，段缓存设计有误。

**⑧ 回滚方式**
- 第 1–4 项（面 2）：四处 revert 回线性扫。**注意：第 4 项 `CaretXForCluster` 是公开 API，revert 时须保持签名不变。**
- 第 5–10 项：`line_index.go` / `layout_cache.go` 整文件删除，`BuildTextLayout` 恢复全量构建。纯加法，回滚无副作用。
- 第 11 项（簇边界）：独立 revert，`visual_move.go` 恢复 rune 吸附。
- 第 12 项（面 1）：d1–d4 整体 revert。**d4 拆门后无法单独回退 d1**，须整体还原。

**⑨ 交接说明（给 M2）**
- **新不变量**：
  - **I2**：布局缓存失效粒度 = 不回绕按行 / 回绕按段
  - **I8（新增）**：`CaretForOffset` / `BoxesForRange` / `LineTop` 必须走行索引 + 前缀和，**禁止全表遍历**
  - **I9（新增，以 §3.2.1 为准）**：`Generation` 保持全局自增语义不动，按行/段失效**另用新字段**表达（禁止改 `Generation` 语义，否则 `TestTextLayout_Generation` 变红 → M1-d2 熔断）
- **新 API**：`TextLayout.LineCount()` / `Line(i)` / `CaretAt(...)`（**公开 API，须进 `RENDER_API_CATALOG.md` 并跑 `scripts/apidoc`**）
- **`.Lines` 已删除**：M2 及之后不得再引用该字段。
- **给 M2 的前置**：M2 的裁剪范围计算**必须基于 M1 的前缀和**，不得重新遍历。

---

### M2 · 绘制与脏区（批量提交 + 真裁剪 + damage）

**① 本期目标**：GPU 顶点数从 O(n) 降到 O(V)（可见字形数），并让 damage 矩形只覆盖变动行/段。

**② 前置依赖（硬）**
- **M0 第 9–10 项**（绘制改用 `glyph.X`）必须完成。否则批量提交会把近千 px 发散**固化成批量错误**，比逐字错误更难排查。
- **M0 第 1–3 项**（面 4 单源修复）必须完成，否则绘制行与布局行仍不一致。
- **M1 第 5–8 项**（行索引 + 前缀和）必须完成，裁剪范围计算依赖它。
- 执行顺序锁定 `M0 → M1 → M2`，**不可调换**。

**③ 背景与根因**
- **§2.2 根因链 ⑤**：因 glyph 为空，`RenderText.Paint` 走 `face.Source()==nil` 分支逐字 `DrawString`；`cullGlyphs` 的条件 `len(glyphs) > 200` 恒为 false → **实测至今一次未生效**。
- **§3.5 约束 ①**：绘制与布局发散 **976.6px/5000 字**（每字 0.195px，确定性线性），靠 M0 对策 C 已消除，M2 是其批量化落地。

**④ 改动清单（精确）**

| # | 文件:行号 | 改什么 |
|---|---|---|
| 1 | `ui/rendering/text.go:895-921`（`RenderText.Paint`，逐字分支） | 逐字 `DrawString` 改为 `DrawShapedGlyphs` 批量提交（约束① 对策 C 落地） |
| 2 | `ui/rendering/text.go:945` `cullGlyphs` | 现有调用条件为 `hasViewportHint && MaxWidth==0 && len(Lines)==1 && len(glyphs)>200` **四条件与**（`text.go:901`）——实测从未生效。改为**基于 M1 前缀和算可见区间**（两次 O(log n) 二分）；**同时覆盖横向（不回绕）与纵向（MaxWidth>0 多行）**，并解除「必须单行」限制 |
| 3 | `ui/rendering/text.go:902` 裁剪调用点（`glyphs = t.cullGlyphs(glyphs)`） | 同上，按 M1 前缀和计算范围，不得重新遍历 |
| 4 | `ui/rendering/` damage 上报 | 编辑只让被改行/段进 damage 矩形（回绕模式为被改段及其位移影响范围） |
| 5 | `render/` GPU 提交层 | 按 (图集页, 颜色) 分组实例化 quad，控制批次数 |

**⑤ 复杂度契约**

| 操作 | 改前 | 改后 |
|---|---|---|
| 击键重绘 | O(n) 次 `DrawString` | **O(V)** 批量提交 |
| 可见区间计算 | 全量遍历 | **O(log n)** 两次二分 |
| damage 面积 | 整框 | **O(变动行/段)** |

**⑥ 验收清单**
- 单测 `TestCullRange_*`：可见区间计算正确；边界含/不含；全不可见返回空；**横向与纵向两种形态各测**
- 单测 `TestDamageRows_*`：改第 K 行只报 K；改换行报 K 及之后；**回绕模式只报被改段**
- 基准 `BenchmarkPaintLine`：N ∈ {1e3,1e5}，绘制耗时比值 ≤ 2
- 真窗：`ui_wr_ime_r5_control`（5000 横滚）`gpu_ops` 与 `cpu_fallback_ops` 不劣化；新增 `vertex_count` 观测字段
- 真窗：`ui_text_edit_accept`（§5.6）跑一次，记录 `paint_ms_p99` 与 `scroll_fps`
- A–J 全族：`cpu_fallback_ops == 0`（F 族硬）

**⑦ 熔断条件**
- `gpu_ops` 或 `cpu_fallback_ops` 劣化 → **停**，查裁剪是否误伤可见内容。
- 出现「可见字符被裁掉」→ 停，裁剪区间计算有误（此类 bug 肉眼可见但难定位，须靠 `TestCullRange_*` 覆盖）。
- `caret_vs_paint_max_delta_px` 回升 → 停，M0 对策 C 在批量路径下未生效。

**⑧ 回滚方式**
- 第 1–3 项：`RenderText.Paint` 与 `cullGlyphs` revert 回逐字 `DrawString`。**注意：revert 后回到 M0 之前的性能水平（5000 字逐字绘制），但功能正确。**
- 第 4 项：damage 上报 revert 回整框。
- 第 5 项：GPU 分组 revert（纯性能，无正确性影响）。
- **整体回滚安全**：M2 不改变任何坐标语义（坐标由 M0 保证），revert 只影响性能。

**⑨ 交接说明（给 M3）**
- **新不变量**：裁剪只影响**提交范围**，不得影响坐标计算。
- **新增禁区**：禁止在裁剪分支里重新计算 glyph 位置（违反 M0 的 `glyph.X` 单源）。
- **给 M3 的前置**：M3 的增量 undo 改动 `Editor`，与绘制层无耦合，可独立进行。

---

### M3 · 文本缓冲（仅增量 undo）⚠️ 范围已收窄

**① 本期目标**：undo 快照内存从 O(n)×100 降到 O(1)。

**② 前置依赖**：M0–M2 已完成（M3 改动 `Editor`，与绘制层无耦合，技术上可独立；但复杂度门禁需在 M0–M2 之后测才有意义）。

**③ 背景与根因**
- **§2.2 根因链 ⑥**：`pushHistory`（`ui/textinput/editor.go:144`）存**整篇文本快照 ×100 份** → 中英混排 1e5 字约 18.5MB 常驻。
- **Q3 决策**：**piece tree 押后为 M3.5**，本期只做增量 undo。
  > **注**：字符串拼接（`s[:off]+"x"+s[off:]`，即 `Editor` 插入的真实成本）实测 1e5 字 **~70 µs**、1e6 字 **~240 µs**；相对当前排版 2350 µs 仅占 **~10%**，远未到 M3.5 的 **30% 触发线** → piece tree 押后成立。
  >
  > ⚠️ **未验证项诚实标注**：§5.6.4 中有 **5 项属真窗/GPU 运行时指标**——`interval_p95_ms`、`fps_interval`、`hitch_rate_per_min`、`rss_slope_kb_per_min`、`caret_vs_paint_max_delta_px`——需 `ui_text_edit_accept` 窗（**尚未创建**，见 M0-pre）+ GPU 环境才能产出，纯 CPU 探针测不了。**未用估算值冒充实测**，待 M0-pre 建窗后补测。

**④ 改动清单（精确）**

| # | 文件:行号 | 改什么 |
|---|---|---|
| 1 | `ui/textinput/editor.go:41-47` `editSnapshot` | 改为增量记录：`offset` + `length` + `replacedText` + selection/composing（不存全文） |
| 2 | `ui/textinput/editor.go:144` `pushHistory` | 只记录增量，不存整篇文本 |
| 3 | `ui/textinput/editor.go:174` `Undo` / `:193` `Redo` | 反向/正向应用增量记录 |

**硬约束**：`Editor` 对外字符串 API 语义**完全不变**，仅内部历史记录结构变化。

**⑤ 复杂度契约**

| 操作 | 改前 | 改后 |
|---|---|---|
| undo 快照内存 | O(文档长) × 100 份 | **O(编辑量)** |
| undo 快照耗时 | O(n) 拷贝 | **O(1)** |

（插入/删除仍为 O(n) 拷贝，M3.5 处理）

**⑥ 验收清单**
- 单测 `TestUndoDelta_*`：100 次编辑后逐步撤销回到初态；redo 对称
- 单测 `TestUndoDelta_MemoryCeiling_*`（**本期核心**）：1e5 字文档编辑 100 次后，`history` 占用 ≤ **编辑内容总量**，而非 100× 文档长
- 基准 `BenchmarkPushHistory`：N ∈ {1e3,1e5}，单次耗时比值 ≤ 2
- 内存门禁：`rss_slope_kb_per_min` 在 1e5 字连续编辑 60s 下 ≤ 30000（E 族）
- 回归：`ui/textinput` 全量绿（Editor 对外语义不变）
- 真窗：`ui_text_edit_accept`（§5.6）跑一次

**⑦ 熔断条件**：撤销/重做任何一处与旧实现行为不一致 → **停**。

**⑧ 回滚方式**：`editSnapshot` / `pushHistory` / `Undo` / `Redo` 四处 revert 回全文快照。**纯内部改动，无外部依赖，回滚零风险。**

**⑨ 交接说明（给 M4）**
- **新不变量**：undo 历史只存增量，不得存全文（写入 §7.1）。
- **给 M3.5 的触发数据**：本期结束时须给出 `BenchmarkKeystroke` 中**文本拷贝占比**的实测值，供 M3.5 判断是否触发。

---

### M3.5 · piece tree（条件触发 · 非必经）

**① 本期目标**：编辑 O(log n)、快照 O(1)。

**② 前置依赖 / 触发条件**（满足任一才做）
1. M0–M2 完成后，`BenchmarkKeystroke` 显示**文本拷贝**占击键耗时 **> 30%**；或
2. 有明确需求支持 ≥ 1e6 字的**编辑**（非只读查看）。

**③ 背景与根因**：同上 Q3 决策。若拷贝未成为瓶颈，本期的复杂度是为伪问题买单。

**④ 改动清单**
| # | 位置 | 改什么 |
|---|---|---|
| 1 | 新建 `ui/textbuffer` 包 | `Insert/Delete/Slice/LineAt/Snapshot`，编辑 O(log n)、快照 O(1) |
| 2 | `ui/textinput/editor.go` | `Editor` 内部持有 buffer，对外字符串 API 语义不变 |

**独立包，不塞进 `ui/textinput`**——日志查看器、代码编辑器都要用。小文本（< 64KB）走直串 + 内容哈希缓存，**分层降级，不是二选一**。

**⑤ 复杂度契约**：插入/删除 O(n) → **O(log n)**；快照 O(n) → **O(1)**。

**⑥ 验收清单**
- 单测 `TestPieceTree_*`：随机 10k 次插入/删除，与 `string` 参考实现**逐字节比对**
- 单测 `TestBufferLineAt_*`：行边界、CRLF、空行、末尾无换行
- 基准 `BenchmarkInsertAtOffset`：N ∈ {1e4,1e5,1e6}，单次耗时比值 ≤ 3

**⑦ 熔断条件**：随机对拍出现**任何一处**不一致 → 停。

**⑧ 回滚方式**：`ui/textbuffer` 整包删除 + `Editor` 恢复直串。**独立包，回滚零风险。**

**⑨ 交接说明**：新包 `ui/textbuffer` 为**公开包**，须进 `RENDER_API_CATALOG.md`？——**否**（属 `ui` 层，不在 render 目录口径内，无需进该目录文档；但需在 `docs/README.md` 登记）。

---

### M3.8 · 冷排版提速专项（M4 前置 · 2026-09-04 插入）

> 编号沿 M3.5 先例（插入期不占整号）。起因：M3/M3.5 合入时 13 个老裁判中 `TestTextLayout_LongBuild_RealFace`（`ui/rendering/text_layout_r2_test.go:162`，5000 字真字体冷排版 <100ms）在本机三测 225–289ms，连红两期。M3/M3.5 均未碰排版层，系环境门禁未达，非功能回归。本专项先扫清该门禁，否则 M4/M5 在本机同样无法走正常链条。

**① 本期目标**：该测试本机连续 3 次 <100ms，13 个老裁判全绿。

**② 前置依赖**：M0–M3.5 已合入；**不改对外 API，不改裁判测试**（纪律硬线，改测试迁就实现即熔断）。

**③ 背景与根因（待 profiling 落定，不许凭印象下手）**：候选热点为真字体逐字量宽/整形、caret 构建、回绕路径（本测试宽 600 带回绕）。M0/M1 已消除三处 O(n²)，剩余热点须先跑 profile 定位再动手。

**④ 改动方向**：对齐 Skia/Flutter（批量整形、复用既有缓存），只动 `ui/rendering` + `render/text` 内部实现，对外语义不变。

**⑤ 验收清单**
- `TestTextLayout_LongBuild_RealFace` 本机连续 3 次 <100ms（贴原始输出）
- M0–M3.5 各期验收逐期重跑，全绿（新专项不得破坏旧阶段）
- 13 个老裁判全绿

**⑥ 熔断条件**：提速引起任何行为不一致或旧阶段变红 → **停**；若 profiling 证明瓶颈是纯算力墙、软件优化已到头 → **停**，转“门禁机器档位”文档方案（写明该门禁适用的机器档位，不改阈值）。

**⑦ 回滚方式**：`ui/rendering` / `render/text` 改动 revert。纯内部性能改动，行为不变，回滚零风险。

**⑧ 交接说明（给 M4）**：M4 的 `BenchmarkOpenDocument`（1e6 行首屏 ≤100ms）与本门禁同源，本专项的 profile 结论一并移交。

**完成结论（2026-09-04）**：⑤验收三条全绿 + 用户真窗人验（验收主窗 + 五扇输入法窗）无明显问题 → M3.8 完成，M4 可开工。

---

### M4 · 虚拟化（纵向行虚拟化 + 懒测量）

**① 本期目标**：解锁 1e6 行——首屏与滚动代价与总行数无关。

**② 前置依赖（硬）**：M1 的布局缓存（按行/段）已完成；**M3.8 已完成（13 个老裁判全绿）**。虚拟化是「懒加载」在行维度的放大。

**③ 背景与根因**：当前无行虚拟化，1e6 行需全量排版 → 首屏 O(n)、内存 O(n)。

**④ 改动清单（精确）**

| # | 文件:行号 | 改什么 |
|---|---|---|
| 1 | 新建 `ui/rendering/virtual_text_lines.go` | 纵向行虚拟化：**直接复用 `ui/rendering/virtual_list.go` 的 `VirtualList` 范式**（含变高行的前缀和 `prefix[i]`），不加新概念 |
| 2 | 同上 | **懒测量**：未显示过的行先用估算高度，滚动到时实测，增量修正总高与滚动条 |
| 3 | 同上 | **定行高 O(1)**：`总高 = 行数 × 行高`，不测任何行 |
| 4 | 同上 | **变行高前缀和**：懒实测 + 增量修正，支持 `ScrollToIndex` 与滚动条定位 |

**⑤ 复杂度契约**

| 操作 | 改前 | 改后 |
|---|---|---|
| 首屏 | O(n) | **O(可见行)** |
| 总高（定行高） | O(n) | **O(1)** |
| 总高（变行高） | O(n) 全测 | **O(log n)** 摊还 |

**⑥ 验收清单**
- 单测 `TestVirtualLines_*`：1e6 行只物化视口 + cache；滚动后窗口正确滑动
- 单测 `TestLazyMeasure_*`：未测行用估算；实测后总高增量修正**且不跳变**
- 基准 `BenchmarkOpenDocument`：1e6 行，首屏时间 ≤ 100ms
- 真窗：新建 `ui_text_m4_virtual`（1e6 行日志场景），A–J 全族
- 门禁：A 族 `interval_p95_ms ≤ 22`、`fps_interval ≥ 55`；E 族 `rss_slope` 达标

**⑦ 熔断条件**：1e6 行首屏 > 500ms 或滚动掉帧 → **停**。

**⑧ 回滚方式**：虚拟化接入点 revert（控件层不接入即退回全量排版）；`virtual_text_lines.go` 整文件删除。

**⑨ 交接说明（给 M5）**
- **新不变量**：未显示行的高度是**估算值**，任何依赖「精确总高」的逻辑都不得直接使用（写入 §7.1）。
- **给 M5 的前置**：M5 的富文本/彩色字形需在虚拟化的行上工作，注意懒测量与 run 高度的交互。

---

### M5 · 任意场景收口

**① 本期目标**：把「任意场景」从设计意图变成**有测试覆盖的事实**；G1 硬目标总收口。

**② 前置依赖**：M0–M4 全部完成。

**③ 背景与根因**：§4 场景矩阵中 9 个维度，当前有 3 项未覆盖（组合字符已并入 M1、泰/缅/高棉缺字体、DPR 未验证），需逐项补齐测试。

**④ 改动清单（精确）**

| # | 文件:行号 | 改什么 |
|---|---|---|
| 1 | `ui/rendering/` run 结构 | **彩色字形**：run 打 `IsColor` 标记，走独立图集通道（不塞 mask 图集） |
| 2 | `ui/rendering/paragraph.go:255` `layoutRunLines` | **富文本 run**：段内多字号/多字体/多颜色；行盒按该行**最大 ascent/descent** 撑开（Flutter SkParagraph 语义） |
| 3 | `render/text/testdata/` | **复杂脚本**：**先盘点复用**已有资产，再补缺口（**守测试数据纪律，禁硬编码字表**）。`render/text/hint/testdata/` **已存在** `NotoSansThai-Regular.otf`（泰文字体）、`th_all.txt`（泰文 128 码位）、`mymr_sample.txt`（缅文）、`cjk3000.txt`、`kr_all.txt`、`latin_all.txt`、`arab_sample.txt`、`deva_sample.txt`、`beng_sample.txt`、`gujr_sample.txt`、`taml_sample.txt`、`ethi_sample.txt` 等。**真正缺的只有高棉（Khmer）与藏文（Tibetan）** |
| 4 | `render/text/glyph_mask_atlas.go:63` | **DPR 量化验证**：确认已实现的 1/4px 量化（`SubpixelXQ2/YQ2`）在多 DPR 下图集不爆炸；**本项是验证，不是新做** |
| 5 | `ui/rendering/text.go:740` `ellipsizeToWidth` | 省略号拟合改为基于前缀和的 **O(log n)** 定位，消除 O(n log n) 二分 |
| 6 | `ui/embedder/input_router.go:413` `pushSurroundingForEditor` | 去重键从「拼 4000 字节字符串」改为比对 `epoch`（`Editor` 已有单调计数） |
| 7 | 全局 | **全场景矩阵回归**：§4 矩阵每项一个单测 + 一次真窗实测 |

**⑤ 复杂度契约**：省略号 O(n log n) → **O(log n)**；IME 去重 O(n) → **O(1)**。

**⑥ 验收清单**
- 单测：§4 矩阵**每项至少一个**独立测试（脚本/组合字符/彩色字形/富文本/长度/DPR/回绕/滚动/GPU）
- 像素断言：按 `UI_PIXEL_ASSERTION_STANDARD.md` F6（文字）+ F0（静态）+ F7（细线）选型，Golden **逐位回归**
- 真窗：**新建** `ui_text_m5_anycase`（多脚本 + emoji + 富文本 + 多 DPR 同窗），A–J 全族
- 真窗：`ui_text_edit_accept`（§5.6）**全部门禁达标**
- **复杂度门禁总收口**：`BenchmarkKeystroke` N ∈ {1e3,1e4,1e5,1e6}，断言 **`T(1e6)/T(1e3) ≤ 1.5`**（G1 硬目标；段长分布沿用 M1 第 ⑤ 项：短段/长段分别测 + 混合分布，不得单一段长冒充）
- **Q4 迁移收口**：47 处 `Lines` 真下标访问已随 **M1-d5 删除字段**一并完成迁移。本期只做**核验**：确认全仓库无残留 `.Lines` 引用（`grep -rn "\.Lines" ui/ examples/ --include=*.go` 排除无关同名后应为 0），而非再做一次迁移

**⑦ 熔断条件**
- §4 矩阵任一项无独立测试 → **停**，不得标记完成（这是「任意场景」的核心承诺）。
- G1 总收口不达标 → 停，退回分析哪一期的复杂度契约未兑现。

**⑧ 回滚方式**：本期为**收敛与补测**，主体改动可独立 revert；测试文件保留（测试不回滚）。

**⑨ 交接说明（项目收口）**
- 产出：`ui_text_edit_accept` 全绿 = **项目完成**。
- 遗留清单写入 §9 风险与遗留，供后续立项参考。
- 趋势曲线（M0–M5 每期的 `keystroke_p99_ms` / `caret_vs_paint_max_delta_px`）写入该窗 README，作为「逐期改善」的诚实证据。

---

### 5.6 M6 · 验收主窗（真实输入框 · 单行 + 多行）

> **用户 2026-09-02 指定**：最终验收必须是一个**真实可交互的输入框真窗**，同时覆盖**单行**与**多行**两种形态。
> 本窗是**本项目对外的最终交付证据**——前面所有 M0–M5 的收益，最终都要在这个窗里被肉眼与指标同时验证。

### 5.6.1 定位与纪律

- **窗名**：`examples/ui_text_edit_accept`
- **性质**：**测试窗，不是示例**——按 `AGENTS.md` 硬纪律，输入框实现必须在 `ui/` 引擎层（`ui/textinput`），本窗**只摆控件、走输入、跑探针/像素/Golden**，禁止在 `examples/` 里实现输入框功能。
- **手动关闭**：`RunFor = 0` 无限运行，人工点 X 关闭（与 `ui_wr_ime_r*` 一致）。`RUN_SECONDS` 仅为门禁最小观察时长。
- **命名与标准归属（声明）**：本窗族（`ui_text_edit_accept` / `ui_text_m4_virtual` / `ui_text_m5_anycase`）**不是** `ENGINE_UI_WIDGET_RENDER.md` §2 主表的 R/C 能力窗，故不套 `ui_wr_r*` / `ui_wr_c*` 命名义务（U5/U6）。命名沿用仓库既有先例 `examples/ui_textinput_ime`（同族 `ui_text*` 前缀）。
  **但：本窗族自愿按 `ui_wr_*` 全套标准执行**——U12（A–J 全族）、U15（1200×800）、U16（`RUN_SECONDS ≥ 5`）、U21（像素断言 F0/F1/F6/F7/F9 + Golden），依据为同款先例 `ENGINE_TEXT_X11_IME_REQUIREMENT.md`。
- **与 `ui_textinput_ime` 的职责边界**：`ui_textinput_ime` 验证 **IME 输入通道**（D-Bus/IBus/fcitx5 协议、候选窗、surrounding text）；本窗 `ui_text_edit_accept` 验证 **排版与绘制性能**（击键延迟、坐标一致性、滚动、回绕）。二者场景有交集但**验收目标不同**，不得互相替代。
- **最小观察时长**：**30s**（需覆盖连续击键 + 滚动 + 组合输入，短于 30s 不得判定通过）。

### 5.6.2 场景（单行 + 多行，两套并行）

| 区域 | 控件 | 内容 | 验证点 |
|---|---|---|---|
| **A · 单行短** | `textinput.NewInputBox` | 空框起步，手工键入 | 获焦/占位/光标/闪烁/IME 候选锚点 |
| **B · 单行超长** | `textinput.NewViewportInputBox` | **5000 字**预填（中英混排） | 横滚、视口裁剪、末尾击键不卡 |
| **C · 单行超长×10** | `textinput.NewViewportInputBox` | **50000 字**预填 | 量级放大后击键仍不卡（G1 主证据） |
| **D · 多行短** | `textinput.NewMultiLineInputBox` | 3 行，手工键入 | Enter 换行、上下移动、粘滞列 |
| **E · 多行长文** | `textinput.NewMultiLineInputBox` | **1e5 字 / ~2000 行**预填 | 纵向滚动、懒测量、任意位置击键 |
| **F · 多行回绕** | `textinput.NewMultiLineInputBox` + `MaxWidth` | 长段落自动回绕 | 回绕模式下编辑只作废所在段（约束 ②） |

> **🔴 硬约束（护栏，防止引擎层被污染）**：上表全部文案、预填内容、场景设定**只能在 `examples/ui_text_edit_accept/` 内出现，禁止进入 `ui/`（`ui/rendering` / `ui/textinput` / `ui/embedder`）引擎层**。
>
> **特别提醒 M0 面 4**：补 `Ellipsis` 能力进 `TextLayout` 时，**必须沿用既有合规模式**——`ui/rendering/text.go:22` 已有 `const textEllipsis = "…"` 且配 `SetOverflow()` 公开 API（内部默认值 + 可配置），**照此办理，禁止新增硬编码**。
>
> **合入前必跑回归检查**：`grep -rn "（" ui/ --include="*.go"` 与 `grep -rn "●" ui/ --include="*.go"`。
> ⚠️ **现状提示**：这两个检查**当前已非零**（`ui/` 下 `（` 有 200 处、`●` 有 3 处，均为历史存量）。因此本条的执行口径是「**本方案改动不得新增**」，存量清理不在本方案范围——但 M0/M1 改动文件若顺手碰到，应清掉。

### 5.6.3 必测交互（人工 + 自动双轨）

**人工**（README 写明步骤，验收者照做）：
1. 点 A 框 → 键入中英混排 → 观察光标跟随、IME 候选框位置
2. 点 B 框 → 光标移到末尾 → 连续快速击键 30 次 → **观察是否掉帧/迟滞**
3. 点 C 框 → 重复 2 → **对比 B，确认 10× 量级下无感知差异**
4. 点 E 框 → 滚到底部 → 在末行击键 → **观察滚动与输入是否流畅**
5. 点 F 框 → 在段落开头插入文字 → **观察后续行重排是否正确（不得串行/错位）**

**自动**（每帧 ticker 探针，输出 JSON）：
- `keystroke_p99_ms`：在 B/C/E 三框各注入合成击键，测 P99 延迟
- `layout_ms_p99` / `paint_ms_p99`：分层耗时
- `caret_vs_paint_max_delta_px`：**约束 ① 回归锁**——光标布局坐标与实际绘制墨迹的最大偏差，门禁 **≤ 2px**（5000 字规模下）
- `ime_anchor_delta_px`：IME 候选框锚点与光标位置偏差
- `scroll_fps`：E/F 框滚动时的有效帧率

### 5.6.4 门禁（A–J 全族 + 专属八项）

**A–J 全族**按 `ENGINE_UI_WIDGET_RENDER.md §2.2` 出全族 JSON，缺失即 FAIL。

**专属硬门禁**：

> **族归属说明（对齐 `ENGINE_UI_WIDGET_RENDER.md` §2.2.1）**：A–J 是**既有指标族**，本窗另有 3 个**自定义指标**（`caret_vs_paint_max_delta_px`、`keystroke_p99_ms`、`ime_anchor_delta_px`），它们是**文本排版专项**，不属于任何既有族，**门禁阈值写在本窗 README**（符合该文档「阈值写在窗 README」的惯例）。特别说明：**J 族（正确性）在真源里是"构建/CI 防假绿"检查**（`ui` 不 import gpu、无 cgo、不降画质），**不含数值字段**。

| 指标 | 阈值 | 族 |
|---|---|---|
| `caret_vs_paint_max_delta_px` | **≤ 2** | **自定义**（非 A–J 既有族，见下方说明） |
| `keystroke_p99_ms` @ C 框（50000 字） | **≤ 16**（一帧内） | **自定义**（当前实测 **29.09 ms**，需降至 16 以下） |
| `T(C框) / T(B框)` 击键耗时比 | **≤ 1.5**（50000 vs 5000，10× 量级） | **G1 主证据**（当前实测 **11.04**，需降 7.4 倍） |
| `interval_p95_ms`（持续击键+滚动 30s） | **≤ 22** | **A** |
| `fps_interval` | **≥ 55** | **A** |
| `cpu_fallback_ops` | **== 0** | F |
| `rss_slope_kb_per_min` | **≤ 30000** | E |
| `hitch_rate_per_min` | **≤ 5** | A |

> **阈值自洽性说明**：
> - `keystroke_p99_ms ≤ 16` 与 `T(C)/T(B) ≤ 1.5` **合理且可达**——当前 29.09 ms / 11.04 是 O(n) 未优化的必然结果（5000→50000 字，字数 ×10 耗时 ×11，典型线性），**G1 达标后两个阈值自动满足**。这组阈值照着判**不会误判**。
> - `interval_p95 ≤ 22` 与 `fps_interval ≥ 55` **互为补集、逻辑自洽**（1000/55 ≈ 18.2 ms，p95 放宽到 22 ms 是留长尾余量）。
> - **§0 G1 的 `T(1e6)/T(1e3) ≤ 1.5` 与本表 `T(C)/T(B) ≤ 1.5` 是同一目标的两次抽样**（C/B = 5e4/5e3，是 1e6/1e3 区间的子集），**非两个独立门禁**——新会话不要当成两件事。
> - `rss_slope ≤ 30000`（≈30 MB/min）相对 M3 实测的「1e5 字编辑 100 次 = 18.5 MB」**偏松**，真跑起来可能永不触发；但需真窗数据才能定论，**本轮不下断言**。

### 5.6.5 像素断言（**F0 + F1 + F6 + F7 + F9**）

按 `UI_PIXEL_ASSERTION_STANDARD.md`：
- **F6（文字）**：每个输入框的文字包围盒内「非背景色像素数 ≥ 阈值」+ 主色占比；Golden 基线对比
- **F0（静态）**：框体边框、背景精确点采样（容差 ≤8/通道）
- **F7（细线）**：1.5px 光标条 —— 沿线段多点采样，「至少 N 点显著偏离背景」
- **F1（平移动画 / 滚动）**：E/F 框含**滚动**形态，必须补。做法：从引擎 `scrollX/scrollY` **实时取 offset** 再采中心点，**禁止固定坐标采样**（固定坐标在滚动后会采到别的内容 → 假绿）
- **F9（异步稳定）**：F 框「插入文字后重排」属异步稳定形态。做法：重排后**等待 ≥3 帧**再采样；且**前后双态各断言一次**（插入前/后都验）

> **为什么补 F1/F9**：`UI_PIXEL_ASSERTION_STANDARD.md §7` 末句明写「本对照为最低集；**窗口实际包含超出此表的形态时，按 §2 矩阵补齐**」。本窗 E/F 区确实含滚动与回绕重排，属实打实的形态缺口——不补会在关窗时被 `metrics-audit` 判 FAIL。

**容差声明（标准 §4：未声明 tolerance 视为零容差）**：
- F0 理论色：**≤ 8/通道**（显式声明）
- Golden 逐位回归：**同引擎默认零容差**（第二次运行起生效）
- F6 主色占比、F7 偏离点数：阈值写在本窗 README

**关键断言（约束 ① 回归锁）**：在 B 框（5000 字）横向滚动到最右，**对末尾字符做区域采样**，断言其实际墨迹位置与 `TextLayout` 报告的 caret X 偏差 ≤ 2px。这是「绘制与布局不发散」的像素级证据，必须与逻辑探针 `caret_vs_paint_max_delta_px` 双证据合证。

### 5.6.6 与 M0–M5 的关系

- M0–M5 每期结束时**都要跑一次本窗**（哪怕当期未完全达标，记录当期数值形成趋势曲线）
- **M6 不是独立施工期**，是贯穿全程的验收载体；M5 收口时本窗全部门禁达标 = 项目完成
- 趋势曲线写入 README，作为「逐期改善」的诚实证据（禁止只报最终值）

---

## 6. 验收体系

### 6.1 三层门禁（缺一不可）

| 层 | 工具 | 判据 |
|---|---|---|
| **正确性** | `go test` | 每期单测全绿；随机对拍（`TestPieceTree_*`）与参考实现逐字节一致 |
| **复杂度** | `testing.B` | 多 N 上测同一操作，断言耗时比值 ≤ 阈值（**进 CI，禁止口头宣称**） |
| **端到端** | 真窗 + A–J 全族 | 按 `ENGINE_UI_WIDGET_RENDER.md §2.2` 出全族 JSON；缺族即 FAIL |

### 6.2 复杂度门禁规范（硬）

每个基准必须**至少 3 个 N 档**（推荐 {1e3, 1e4, 1e5}，M5 加 1e6），输出形如：

```
BenchmarkKeystroke/1e3    T = 0.08 ms
BenchmarkKeystroke/1e4    T = 0.09 ms
BenchmarkKeystroke/1e5    T = 0.10 ms
→ T(1e5)/T(1e3) = 1.25   (门禁 ≤ 5)  PASS
```

**禁止**：只测单一 N 就宣称「已优化」；用绝对值阈值代替比值阈值（绝对值随机器变，比值才是复杂度）。

### 6.3 真窗要求

#### 6.3.1 真窗生命周期表（新会话先看这里）

> **状态基准：2026-09-02**。新会话开工前先查此表，确认本期要用的窗**是否已存在**；不存在则按「建于」列找到该期去建，不要指望它自己出现。

| 真窗 | 当前状态 | 建于 | 被哪些期使用 |
|---|---|---|---|
| `ui_wr_ime_r1_editor` | ✅ **已存在**（有 main.go） | — | M0–M5 每期回归（须保持绿） |
| `ui_wr_ime_r2_textlayout` | ✅ **已存在** | — | M1、M2 回归 |
| `ui_wr_ime_r3_channel` | ✅ **已存在** | — | 回归 |
| `ui_wr_ime_r4_selection` | ✅ **已存在** | — | 回归 |
| `ui_wr_ime_r5_control` | ✅ **已存在** | — | M2 回归（`gpu_ops`/`cpu_fallback_ops`） |
| **`ui_text_edit_accept`**（§5.6 M6） | ❌ **不存在** | **M0-pre**（见 M0 第 ② 项） | **M0–M5 每期都要跑**，记录趋势曲线 |
| `ui_text_m4_virtual` | ❌ 不存在 | **M4** | M4 |
| `ui_text_m5_anycase` | ❌ 不存在 | **M5** | M5 |

> **🔴 最关键的一条**：`ui_text_edit_accept` 被 **M0 到 M5 每一期**的验收清单引用，但**尚未创建**。它必须在 **M0 改任何代码之前**建好并跑出基线（见 M0 第 ② 项 M0-pre）——否则 M0 无窗可跑，且一旦开始改代码，发散基线就再也取不到。

#### 6.3.2 通用要求

- 现有 `ui_wr_ime_r1/r2/r3/r4/r5` 每期结束**必须保持绿**，且行为逐像素等价（Golden 对比）。
- **验收主窗（§5.6 M6）**：`ui_text_edit_accept` —— **真实单行 + 真实多行输入框**，是本项目对外的最终验收窗。
- 全部真窗 1200×800，`RUN_SECONDS ≥ 5`，出 A–J 全族 JSON（G3 硬）。
- 像素断言按 `UI_PIXEL_ASSERTION_STANDARD.md` 选型，本方案主要涉及 **F6（文字）+ F0（静态）+ F7（细线/边框）**，容差显式声明。

### 6.4 每期收口清单

标记本期完成前，以下**全部**必须为真（缺一不可）：

- [ ] 红灯起步的失败单测/基准已转为绿灯
- [ ] 单测全量绿（含历史阶段回归，按文件逐个跑 `-run`，禁止 `./...` 全量并发）
- [ ] 复杂度门禁比值达标（**不回绕与回绕两种模式各跑一遍**，见 M1 第 ⑥ 项）
- [ ] 真窗 A–J 全族 JSON 已出且达标
- [ ] **M6 验收主窗（§5.6）已跑，数值记入趋势曲线**
- [ ] 跨期不变量 **I1–I11**（§3.2.1）未被违反
- [ ] `go run ./scripts/apidoc` exit 0（若改了 render 公开面）
- [ ] `docs/RENDER_API_CATALOG.md` 已同步（若改了 render 公开面）
- [ ] `docs/ENGINE_UI_WIDGET_RENDER.md §10` 修订表已追加一行
- [ ] 回滚点已验证（按本期第 ⑧ 项实际执行一遍，确认上期功能仍完整）
- [ ] 退化门禁已跑：`grep -rn "\.Lines" ui/ examples/ --include=*.go`（M1-d5 后应为 0）与 `grep -rn "RuneAdvance\|MeasureWidth" ui/rendering/ --include="*.go"`（caret/hit 路径逐字累加即红），两项均为红即 FAIL

---

## 7. 高可用

### 7.1 降级链（任一层失败不得崩、不得退化到不能输入）

| 失效 | 降级行为 | 守不变量 |
|---|---|---|
| itemizer / run 切分不可用 | 退回单字体整串整形（当前行为），功能完整，仅性能回退 | I6 |
| 布局缓存未命中 | 现场重排该行/段，结果入缓存；不得抛错 | I2 |
| **回绕模式段缓存失效** | 退回「整段重排」；**禁止**退回「全文档重排」 | **I2**（否则 O(n²) 复发） |
| piece tree 不可用 | 退回直串实现（小文本本就走这条），功能完整 | — |
| 懒测量估算偏差 | 滚动到时实测修正，总高单调修正，不得跳变/不得崩滚动条 | I5 |
| 图集满 | LRU 分页淘汰 + 必要时降级为 CPU 软路径（记 `cpu_fallback_ops`，F 族可解释） | — |
| 裁剪计算异常 | 退回全量提交（性能回退，画面正确优先） | **I3** |
| **绘制行与布局行不一致** | **立即退回 M0 之前路径**（`DisplayLines` 独立计算），并记录 bug 已复现 | **I7** |
| **绘制坐标与 layout 不一致** | **立即退回 M0 之前路径**（逐字 snapPen），画面正确优先 | **I1** |
| **行索引/前缀和失效** | 退回线性扫（性能回退），但**必须在指标里如实上报**，不得静默 | **I8** |
| **按行失效标记失效** | 退回全局作废（缓存命中率下降但正确） | **I9** |
| **script/bidi 分段不可用** | 退回「仅按 `MultiFace.Runs` 切分」（当前行为），功能完整；阿拉伯等复杂脚本**维持现状不整形**（不会更差，但也未修好），须在指标里如实上报 | **I10** |
| **截断（MaxLines/Ellipsis）两侧不一致** | **立即退回 M0 之前路径**，以绘制侧为准（画面正确优先），并记录 bug 已复现 | **I11** |
| **增量 undo 历史不可用** | 退回全文快照（M3 之前路径，内存回到 O(n)×100，**仅临时退路**，必须记录） | **I4** |

**原则**：**画面正确 > 性能**。任何优化在不确定时退到正确路径，并在指标里如实上报，禁止静默降级装绿。

### 7.2 熔断

- 任一期验收不过 → **停止进入下一期**，修到达标为止。
- **禁止放宽阈值通过**；阈值调整必须在本文档修订表记录理由。
- 真窗 `cpu_fallback_ops` 无故暴涨、`rss_slope` 超预算、A 族掉帧 → 当期 FAIL。

### 7.3 回滚

- **M0**：advance 缓存是纯加法（新文件 + 3 处调用点），revert 无副作用；绘制改用 `glyph.X` 与 run 切分**耦合**，须整体回滚（见 M0 第 ⑧ 项）。
- **M1**：行索引 + 布局缓存是纯加法（新文件）；前缀和三处 revert 时须保持 `CaretXForCluster` 公开签名不变（见 M1 第 ⑧ 项）。
- **M2**：不改变坐标语义（由 M0 保证），revert 只影响性能，**整体回滚安全**。
- **M3**：`editSnapshot` / `pushHistory` / `Undo` / `Redo` 四处 revert 回全文快照，**纯内部改动，回滚零风险**。
- **M3.5**：`ui/textbuffer` 为**独立新包** → 整包删除 + `Editor` 恢复直串，**回滚零风险**。
- **M4**：虚拟化走**控件层接入** → 不接入即退回全量排版；`virtual_text_lines.go` 整文件删除。

### 7.4 并发与线程

沿用现有契约（IME 需求文档 §8 C1/C4）：
- **C1**：Editor 状态只在**事件循环线程**修改；
- **C4**：`dispatch` 独占事件循环线程；
- 本期新增：M4 的**懒测量**若引入后台 goroutine，结果必须经队列回灌 UI 线程，**禁止跨线程直接改 Editor 或布局缓存**。行/段布局缓存若被多线程访问，必须加锁或采用线程封闭。

---

## 8. 家底盘点（避免重复造轮子）

### 8.1 已有可复用

| 能力 | 位置 |
|---|---|
| bidi（UAX #9） | `render/text/segment.go`（基于 `x/text/bidi`） |
| HarfBuzz Go 移植（默认 shaper） | `render/text/shaper_hb.go`，`defaultShaper = NewHbShaper()` |
| 阿拉伯连接 / 印度系重排 / GSUB-GPOS | `gsub.go` / `indic_shaping.go` / `arabic_joining.go` |
| 字形 mask 图集 + 批量取 | `render/text/msdf/atlas.go` 的 `Get/GetBatch` |
| 整形结果软 LRU 缓存 | `shape_result_cache.go`（容量范式可直接抄） |
| 列表虚拟化 + 变高前缀和 | `ui/rendering/virtual_list.go` |
| Damage 矩形上报 | `ui/scene/textured.go` → `pipeline_app.go` `TrackDamageRect` |
| 断词/簇分段器 | `render/text/segment.go` |
| **字体回退 run 切分** | `MultiFace.Runs()`（`render/text/multi.go:355`，带 `globalMultiFaceRunsCache`）— 已存在，M0 不必自研 |
| **字素簇分段（M1-c 已落地，无新依赖）** | `render/text/grapheme.go`（`ClusterStarts`/`SnapCluster`，复用 `third_party/go-text/typesetting/segmenter` 的 UAX#29 实现；见 §9.1 Q5-D） |
| **亚像素量化（1/4 px）** | `render/text/glyph_mask_atlas.go` `MakeGlyphMaskKey`（`SubpixelXQ2/YQ2`）— 已实现，M5 只做验证 |

### 8.2 真正缺失（本方案要补的）

| # | 缺失项 | 阶段 | 备注 |
|---|---|---|---|
| 1 | advance 缓存 | M0 | 新建，范式照 `shape_result_cache.go` |
| 2 | 行/段布局缓存 | M1 | 新建，**两套结构**（不回绕按行 / 回绕按段） |
| 3 | 前缀和 caret | M1 | 替换 `CaretXForCluster` 的 O(n²) 线性扫 |
| 4 | 增量 undo | M3 | 替换全文快照 |
| 5 | 纵向行虚拟化 + 懒测量 | M4 | 复用 `VirtualList` 范式 |
| 6 | 簇边界 caret 吸附 | M1 | **部分已有**：`text_layout.go:470` `snapToGraphemeBoundary` 仅处理 UTF-8 续字节归位。M1 需**升级为 UAX#29 字素簇**并**扩展到全部 caret 路径**（`GetPositionForOffset`/上下键/`visual_move.go`），非「复用 `segment.go`」即可（`segment.go` 只做 script/bidi 分段，无字素簇能力） |
| 7 | **script/bidi 分段叠加** | M0 | `MultiFace.Runs()` 只按「字形是否存在」切分，**不做 script/bidi 分段** → 阿拉伯文在 DejaVuSans 下有字形故不回退、但**不整形**（`Shape` 返 0 glyph）。须在 `Runs()` 之上叠加 `segment.go` 分段 |
| 8 | **`MaxLines` 布局侧截断** | M0 | 现状：`MaxLines` **仅绘制侧生效**（`text.go:631-634`），布局侧无截断 → 2v12。属面 4，随单源修复一并解决 |
| — | ~~字体回退 itemizer~~ | — | **降级为「接线 + 验证」**（`MultiFace.Runs` 已存在，约束 ③；但验证发现**需叠加 script/bidi 分段**，见上第 7 项） |
| — | ~~DPR 亚像素量化~~ | — | **降级为「验证」**（图集 key 已含 1/4px 量化） |
| — | ~~piece tree~~ | M3.5 | **降级为条件触发**（Q3 决策，实测拷贝非瓶颈） |

**结论：这是补 20% 再组装，不是重写。** 2 件已有能力覆盖（run 切分、DPR 量化），1 件条件触发（piece tree）；必修 **8 件**（含 script/bidi 分段叠加、`MaxLines` 布局侧截断）。

### 8.3 修订过程记录（已搬出正文）

> 本节原对照表已搬到 `docs/ENGINE_TEXT_SCALE_CHANGELOG.md`（修订明细 + 8.3 原表）。正文只留当前结论，见 §8.2。

---

## 9. 风险与遗留

| 风险 | 影响 | 缓解 |
|---|---|---|
| `MultiFace` 通路改动影响面大 | M0 阻塞 | 两种取法择优；公开行为零变化为硬约束，否则停 |
| **像素 golden 变化** | M0 阻塞 | 整形通路切换是高风险改动；宁可退回逐字 fallback 也不接受静默画质变化 |
| **面 1：`Lines` 139 处引用（47 真下标）迁移** | M1 工作量最大 | 拆四小步（d1–d4），d2 用老测试当独立裁判；d4 后拆门，不留兼容期 |
| **🔴 M0「先慢后快」** | **修完击键反而更慢** | 当前 `MultiFace` 走 O(n) 兜底；修通整形后会进入**含 O(n²) 扫表的成功路径**（此前从未执行）。**对策**：M0 第 8 项与 `CaretXForCluster` 扫表消除**必须同时做**，新增 `BenchmarkCaretBuild` 门禁 + 熔断 |
| **阿拉伯/复杂脚本不整形** | 阿拉伯、印度系等**显示错误**（`Shape` 返 0 glyph） | `MultiFace.Runs()` 只按字形有无切分、不做 script/bidi 分段 → 有字形的脚本不回退也不整形。M0 须叠加 `segment.go` 分段 |
| **面 2：`CaretForOffset`/`BoxesForRange` O(n²)** | **2 万行卡死（375ms/次）** | M1-a 专项去 O(n²)；`TestCaretForOffset_NoQuadratic_*` 等基准锁 |
| **面 4：绘制侧与布局侧行不一致（存量 bug）** | **A1 单源纪律已破损** | M0 最先修；`TestDisplayLinesMatchLayout_*` 四场景锁 |
| 簇边界改动影响现有 caret 单测 | M1 延期 | 可拆为 M1.5 独立做 |
| **回绕模式段缓存设计不当 → O(n²) 复发** | M1/M4 返工 | M1 一次性把两种模式的缓存结构都设计好（§3.3 第 4 条）；复杂度门禁分模式各跑一遍 |
| **M1-d2 老测试未全绿却改测试迁就实现** | 失去判错能力，光标错位难发现 | **硬约束**：d1 与 d2 之间禁止改任何测试文件；未全绿即熔断 |
| 缺**高棉/藏文**字体（泰/缅已有 `hint/testdata` 资产可复用） | M5 该两项无法验证 | **先盘点复用** `render/text/hint/testdata/`（已有泰文字体+字表、缅文字表、CJK/韩/拉丁/阿拉伯/梵文等），**仅高棉/藏文需新增**；新增字表须先落 `testdata/` 文件再写测试，禁止 range 生成 |
| piece tree 引入新包增加维护面 | 长期 | **已押后为 M3.5 条件触发**（Q3）；未触发则不引入 |
| 亚像素量化引入视觉抖动 | M5 | **量化已实现**，本项为验证；需 Golden 逐位对比 + 多 DPR 真窗目检 |
| **M6 验收主窗自身性能不达标** | 项目无法收口 | M0–M5 每期都跑一次该窗记录趋势，早暴露而非最后才发现 |
| **新会话误读分期导致返工** | 任一期内耗 | §9.2 九项规范 + §3.2.1 跨期不变量；每期改动清单精确到文件:行号 |
| **C 框单行整形 O(L)（M5 遗留，归 M1；已定位，需引擎活）** | 单行整形复用 | **核验结论（2026-09-04，源码+实测）**：生产增量路径单行 5 万字中位数 **71ms**（`ui/rendering/m5_singleline_test.go` 真字体常驻锁，红灯复现后转 200ms 防退化绊线；先前 72µs 系 nil face 估算口径，已纠正）。根因链（逐行核对）：`input_box.go:467` 走 `SetTextSpan` → `layout_update.go:75 updateSpan`（`checkSpan` 仅 32 字节抽查，O(1)）→ `applyPatch`/`patchRows` 粒度是整行 → 单行文档整行重整 → 整形缓存键是整串 `textHash`（`shape_result_cache.go:23`）必然 miss → O(L) 整形（§3.4 整行整形设计的直接后果）。**修法**：行内整形复用（只重整变更 run + 拼接，`ui/rendering` 内部，中）或前后缀复用缓存；M3.5 不碰（代价是整形非拷贝）。改门禁数值本身需审批，禁单方面。**已落地（2026-09-04，feat/ime-x11）**：行内复用（`ui/rendering/row_reuse.go`，只重整变更run+拼接；另补`effectiveFace`记忆化，否则增量因face身份抖动永不生效）。实测单行5万字击键中位数72→24ms，单次Shape分发50000→1；余量约24ms为整行`SegmentText`（UAX#9在`render/text`层，另立项）。200ms绊线与13裁判全绿 |
| **完整彩色图集 GPU 通道未建（M5 遗留，归 M5 后续）** | emoji 画质/性能 | M5 只做到 mask 旁路 + 字符串彩色路。**CBDT 实测（2026-09-04，真机）**：NotoColorEmoji 进链先崩（`DrawWithEmoji` 对无 Source 脸无 guard，已修 + 回归锁），修完仍整段回退（19 万 fallback、hitch、CPU 127%）且空白——CPU 逐帧兜底此路不通，已回退。**层位**：`render/`（CPU 位图小字号无像素）+ `gpu/`（mask 管线无位图感知、无显式拒绝）。**家底盘点（逐文件核对）**：COLR 解析器（`emoji/colr.go`）+ CBDT 提取器（`emoji/cbdt_extractor.go`，含 StrikeBestFit）+ emoji 检测分段 + `IsColor` 标记旁路 + 空 guard 均存在；**但 `ColorFont` 接口（`render/text/color_font.go:12`）零实现者**——解析器与绘制之间缺适配器，`DrawWithEmoji` 永走 outline 回退（`DetectGlyphType` 恒返 outline），此为 CBDT 空白的直接根因（非 strike 选择问题）。`gpu/` 内无 CBDT/CBLC/strike 任一分支（mask 管线纯轮廓）；`scripts/cap_compare_golden.py` 系 capability_matrix 的 RMSE/SSIM 门，与 U21 逐位口径无关，不可复用。**缺口与三级台阶**：① 补 `ParsedFont`→`ColorFont` 适配器（绑 CBDT 提取 + COLR 解析，小–中，接口与解析器都已存在）；② `gpu` 层 mask 管线加显式拒绝（有色/位图走干净回退，学 `resolveGlyphMaskParams:310` 的 nil-Source 模式，小）；③ 完整 GPU 颜色图集（大，动 `render/` 公开面 + apidoc）。禁一步到位，先走①② |
| **M5 GPU 真窗已补，余两小项（M1–M4 欠账，M5 代记）** | 收口完整性 | **已补（2026-09-04，真机）**：m5 窗三跑 fps≈58.8/p95≈17.4/hitch 0/零回退，Golden 两跑 0px，F6 四区有墨、F0 一致；accept 60s 除 C 框 `keystroke_p99`（存量设计）外全绿。**剩余**：① accept Golden 相对 M0 基线差 1.0%（散布字形抗锯齿级，基线过期，待专立刷新项——以 M5 截图为新基线归档，须独立评审）；② B 区 emoji 端到端彩色像素（缺链内彩色字体，随颜色管线项走） |

**遗留（本期不做）**：协同编辑/CRDT、RTL 特殊布局（DeleteSurrounding 按逻辑序已处理）、IME 自绘候选窗。

---

## 9.1 已拍板决策（用户确认 · 2026-09-02）

| # | 决策 | 已选 | 落地位置 |
|---|---|---|---|
| **Q1** | 约束 ① 对策 | **C · 按 run 批量整形 + 批量绘制**，绘制用 `glyph.X`（与布局同源） | **M0 第 8–10 项**（run 切分 + 绘制用 `glyph.X`）+ **M2**（批量化落地） |
| **Q2** | 回绕缓存粒度 | **段缓存**（以 `\n` 分段，编辑只作废所在段） | **M1 第 9 项**（双模式布局缓存：不回绕行缓存 / 回绕段缓存） |
| **Q3** | piece tree | **押后**——M3 只做增量 undo，piece tree 降为 **M3.5 条件触发** | M3 / M3.5 |
| **Q4** | `Lines` 公开面 | **加惰性 API + 拆旧字段**（分 d1–d5 五小步，不留兼容期） | **M1 第 12 项**（面 1 迁移，含 d1–d5） |
| **Q5** | 字素簇依赖（M1 第 11 项开工前必须先定；默认 A，待用户最终确认） | **D · 复用内置 `segmenter`（2026-09-03 M1-c 落定）**：`third_party/go-text/typesetting/segmenter` 已实现 UAX#29（含 GB11 表情 ZWJ 序列、GB12/GB13 地区指示符、Extend/ZWJ 不切分，代码已核验），且 `render/text/wrap.go` 已依赖该包——**零新依赖，`go.mod` 无需改动**。原 A（引入 `rivo/uniseg`）/ B（自研）/ C（降级 M1.5）不再采用 | **M1 第 11 项**（M1-c：`render/text/grapheme.go` `ClusterStarts`/`SnapCluster` + 4 条路径吸附）。`go.mod` 零变更，故无单独依赖提交 |

> **⚠️ 编号提示**：Q1/Q2/Q4 的落地位置曾写错过，**已按 M0/M1 实际改动清单校准**。**若后续调整 M0/M1 改动项编号，必须同步回来改这张表**——这是跨期引用的高危点。Q5 落地 M1 第 11 项，同样受本条约束。

### Q1-C 的连带影响（必须遵守）
选 C 后 **M2 不能再独立于 M0 评估**。执行顺序锁定为 `M0 → M1 → M2`：
- M0 打通「绘制用 `glyph.X`」是 M2 的**正确性前置**；
- 若 M0 未完成就做 M2，批量提交会把近千 px 的发散**固化成批量错误**，比逐字错误更难排查。

### Q4 迁移路径（已升级为「选项 4 · 先绿后拆」）

> **用户 2026-09-02 质疑**：开发阶段框架、推翻重写，为何还要留兼容门？
> **答**：不需要长期兼容。老测试的价值不是「兼容」，而是当**一次独立裁判**——验证新实现与老实现行为等价。用完即拆。

```
M1-d1  生产代码 22 处换新 API（input_box 11 + visual_move 4 + viewport_input 4 + text 3）
       老字段 .Lines 暂留作【临时脚手架】
M1-d2  13 个老测试【一行不改】直接跑 → 全绿 = 新实现与老实现等价 ✅
       （10 个 text_layout_r2_test.go + 3 个 r1_selftest_visual_test.go）
M1-d3  老测试改用新 API 读数据（断言不动），再跑一遍，必须仍全绿
M1-d4  示例探针 23 处迁移到新 API（ui_wr_ime_r1/r2/r4/r5 + ui_textinput_ime）
       ⚠ 必须在 d5 之前——d5 删字段会让这些示例立即编译失败
M1-d5  删除 .Lines 字段，老门拆掉
       —— 全部在 M1 内完成，不跨期、不留兼容期
```

> **编号说明**：M1-d 为 **d1–d5 五小步**（d4 = 示例迁移、d5 = 删字段）。本文档一律用 d1–d5。

**为什么 d2 不能省**：这 13 个测试（10 个 `text_layout_r2_test.go` + 3 个 `r1_selftest_visual_test.go`）验证的是**行为契约**（caret X 往返差 ≤0.5px、混排不劈簇、HiDPI 对齐、省略号不进 caret），不是 API 形态。若 d1 与测试改写同时进行，**运动员（实现）与裁判（测试）同时换人**，比分失去意义——光标偏 2px 这类错在 5000 字框里肉眼根本看不出来。

**硬约束**：d1 与 d2 之间**不得修改任何测试文件**。d2 未全绿 → 熔断。

**注**：原计划的 `TestTextLayout_LinesIndexable_*`（长期双路径一致性锁）**已不需要**——d2 的全绿已证明等价性，且 d4 后不存在双路径。改为 `TestLegacyLinesContract_*`，仅在 M1-d2 阶段有效，d4 后随 `.Lines` 一并删除。

---

## 9.2 新会话接手规范（硬 · 每期必须满足）

> 背景：本方案跨 M0–M5 多期，每期可能由**全新会话**执行。新会话**没有前序上下文**，只能读文档。
> 因此每一期必须做到：**只读本文档该期条目即可开工，不需要读代码历史、不需要问人。**

### 每期条目必须包含的九项（缺一即视为设计未完成）

| # | 项 | 说明 |
|---|---|---|
| 1 | **本期目标** | 一句话，可判定达成与否 |
| 2 | **前置依赖** | 依赖哪几期、哪几个必须已完成的能力；未满足则不得开工 |
| 3 | **背景与根因** | 为什么做这期；引用 §2/§3.5 的实测编号，不写"优化一下" |
| 4 | **改动清单** | **精确到文件:行号 + 改什么**，不接受"重构排版层"这类模糊描述 |
| 5 | **复杂度契约** | 改前 vs 改后，按模式分列 |
| 6 | **验收清单** | 单测名 / 基准名 / 真窗名 / 门禁阈值，**全部可执行** |
| 7 | **熔断条件** | 出现什么现象立刻停、向谁报告 |
| 8 | **回滚方式** | 具体怎么退（revert 哪些文件 / 关哪个开关），不是"可以回滚" |
| 9 | **交接说明** | 本期产出给下一期留下什么（新 API / 新不变量 / 新增的禁区） |

### 新会话开工五步法（写进每期开头）

1. **读本文档 §0 硬目标 + §1 术语 + 本期条目**，不要读代码历史推断意图；
2. **先跑一遍基线**：`go test ./ui/... ./render/... -count=1` + 本期涉及的真窗，记录当前绿灯状态；
3. **红灯起步**：先写本期要加的失败单测/基准，确认它确实红；
4. **改代码**，每改完一个文件跑一次相关测试，不要攒着最后跑；
5. **收口**：按 §6.4 清单逐项打勾，缺一项不得标记本期完成。

### 文档可自洽性要求

- 每期的**所有引用**（文件:行号、API 名、测试名）必须**在文档内可查或可从代码直接定位**，不依赖"上一期说过"；
- 新引入的**不变量**必须写进 §3.2 复杂度契约表或 §7.1 降级链，不得只写在某期条目里；
- 每期的**熔断与回滚**不得写"见上期"，必须本期完整重写一遍。

---

## 10. 修订

| 版本 | 说明 |
|---|---|
| 立项 | 2026-09-02 · 文本越长输入越卡，排版占 86%；定 M0–M5 六期与 G1–G8；整行整形 + 提交裁剪。 |
| 重审 | 2026-09-02 · 补三条硬约束（发散 976.6px/5000 字、回绕级联作废、`Runs` 已存在但缺 script/bidi 分段）与 Q1–Q4 决策；M2 转为正确性前置。 |
| 决策落定 + 分期细化 | 2026-09-02 · Q1 选 C、Q2 段缓存、Q3 piece tree 押后 M3.5、Q4 惰性 API + 拆旧字段；新增 M6 验收主窗、§9.2 接手规范、I1–I6、G7/G8；M0–M5 按九项规范重写。 |
| 全量问题纳入 + 分期重排 | 2026-09-02 · 四个面全纳入：M0 先修面 4；M1 分 a/b/c/d 四步；新增 I7/I8/I9（I9 以 §3.2.1 为准：`Generation` 保持全局自增）；Q4 升级为先绿后拆。 |
| 终审收敛 | 2026-09-02 · 通读校准行号、G1 分档、§7.3 回滚写实等 10 处；§9 补两项风险。 |
| **实证复核 · 收敛** | 2026-09-02 · 关键论断全部重跑探针复验，主干无一被推翻；当前值：量宽 ~430ns/字、发散 976.6px/5000 字、面 4 为 2v12/11v1、面 2 两处 O(n²) 确认、新增 M0 先慢后快风险。明细见变更记录。 |
| **可开工性校验 · 新会话视角** | 2026-09-02 · 新会话视角走查：补 M0-pre 前置建窗任务、§6.3.1 生命周期表、面 4 落地要点。 |
| **五 agent 并行分类核验 · 合成** | 2026-09-02 · 五维度并行核验：`Generation` 保留语义另增字段、M0 第 8/9 项路径要点、M1 第 11 项补依赖决策（Q5）、前置编号校准、数值维度复验 22 项。明细见变更记录。 |
| **七坑收敛** | 2026-09-03 · 落 7 处实现尾巴：① M1 ⑨ I9 改回以 §3.2.1 为准（`Generation` 保持全局自增，另用新字段；同步修正 `TestGeneration_PerLine_*` 断言口径）；② §9.1 新增 Q5（字素簇依赖默认 A · `rivo/uniseg`，`go.mod` 单独提交，M1-c 开工前最终确认；同步补 §8.1 待引入行）；③ M0 第 8 项加同提交硬锁（通整形 + 消扫表同一提交合入、同一基准验证，⑦熔断 + ⑧回滚同步耦合）；④ 补相交合并规则（`Runs` 区间 × script/bidi 区间取相交，RTL 视觉序重排）+ `TestIntersectRuns_*`；⑤ M0 第 1 项落点写死（`TextLayout` 新增 `MaxLines` + `Overflow/Ellipsis`，单串/多 run 统一算截断、布局侧落省略号字形并排除 caret）；⑥ 门禁补段长分布（短段 ≤200 字 / 长段 ≥5000 字 + 混合分布）与缓存 key 规则（advance 与布局缓存 key 必须含 hinting/variations）；⑦ 加退化门禁（`grep RuneAdvance\|MeasureWidth` 即红，同步进 M1 ⑥ 与 §6.4 收口清单）。主干（四面认定、M0→M5 分期、G1–G8、Q1–Q4）不动。 |
| **M5 收口** | 2026-09-04 · 七项落地（CPU 侧）：① `LineGlyphRun`/`TextRun`/`displaySpan` 增 `IsColor`，`emoji.Segment` 检测，`paintCompositeRuns` 彩色分区绕开 mask 批量（`SubmittedMaskGlyphEstimate` 观测）；② 富文本行盒核实已是最大语义，零改动加锁；③ `khmer_all.txt`（128）+`tibetan_all.txt`（256）字表与整形单测；④ DPR 量化验证（16 键不爆炸）；⑤ `ellipsizeWithPrefix`（一次前缀构建+O(log n) 定位+常数次校验）；⑥ 去重键改 `Editor.Epoch` 比对（`lastSurr` 删除）；⑦ 矩阵 9 行单测齐。G1 `T(1e6)/T(1e3)`=0.92/1.19/1.25 ✅。真窗 `ui_text_m5_anycase` 新建并取改前基线。遗留见 §9（整形路径长文击键超线性、GPU 窗未验、彩色图集通道未建）。 |

---

## 11. 校验记录（明细见变更记录）

> 过程证据已搬到 `docs/ENGINE_TEXT_SCALE_CHANGELOG.md`（修订明细、复核对照表、探针原始输出）。结论一句话：关键论断全部重跑复验，主干无一被推翻；动手只看正文各节的当前值。
