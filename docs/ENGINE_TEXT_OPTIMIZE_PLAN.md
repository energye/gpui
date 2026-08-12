# render/text 正向优化清单（2026-08-12）

> 原则：**保证现有功能正确性**（全量回归不破坏），做正向优化——找潜在问题、
> 收敛代码、优化性能/内存、提高可读性。先列清单，再逐项验证真实性，最后逐步修改。

## 验证与修改进度

| 项 | 验证 | 修改 | 回归 |
|---|---|---|---|
| P1 竖排斜移（正确性） | ✅ 实锤（x-spans 94px→修复后单列 30px，3 字 3 垂直段） | ✅ drawGlyphs 竖排不累加 advanceX | ✅ 183 PASS 0 FAIL |
| P2 autohintCache 无上限 | ✅ 静态确认（无 maxEntries） | ✅ 加 256 上限 + 满时重置 | ✅ 221 PASS 0 FAIL |
| P3 OwnShaper cache 无上限 | ✅ 静态确认 | ✅ 加 64 上限 + 满时重置 | ✅ 221 PASS 0 FAIL |
| P4 Glyphs/AppendGlyphs 重复 | ✅ 静态确认（130 行重复） | ✅ 抽 iterGlyphs 公共迭代器 | ✅ 221 PASS 0 FAIL |
| P5 MultiFace/FilteredFace 逐字绘制 | ✅ 像素探针（见下） | ✅ 改按 FaceRun 连续 run 合并绘制 + 竖排修正 | ✅ Draw 路径 8 文件全绿 |
| P6 ownParsedFont 懒解析模式 | ✅ 静态确认（13 个 sync.Once + 哨兵字段） | ✅ 泛型 lazySlot[T] 收敛 + typed 值结构 | ✅ 摘要探针前后一致 + 24 窗 PASS |
| P7 逐字 string(r) 分配 | ✅ 探针实测（MultiFace.Advance 64→0 allocs） | ✅ glyphForRune 免分配单字 + MultiFace iterGlyphs 收敛 | ✅ 摘要探针前后一致 + 17 窗 PASS |
| P8 glyph_cache 上限复核 | ✅ 静态确认（淘汰已实现） | ✅ 补 4 个显式测试收口（分片上限/统计诚实/GetOrCreate/无退化） | ✅ glyph_cache 全窗 30 测试 PASS |

> **本轮收口（2026-08-12）**：P1–P8 全部完成。正确性：P1 竖排斜移（drawGlyphs +
> MultiFace 两处）+ P5 逐字光栅/线性扫 + P5 附带竖排 MultiFace 斜移；内存：
> P2/P3 两处无上限缓存 + P6 懒解析收敛；性能：P5 run 合并、P7 逐字分配清零；
> 可读性：P4/P6/P7 三处重复收敛。每项均以「修改前后行为等价」证据收口
> （像素/摘要探针 + 对应测试窗回归），全程未改公共 API 语义。

## 其它候选问题

### P1. drawMultiFace 逐 rune 单独绘制 + drawGlyphs 竖排斜移（正确性 bug + 性能 ⚠️⚠️）
- 位置：`render/text/draw.go:100-160`（drawGlyphs）+ `:343-381`（drawMultiFace）
- **已实锤正确性 bug**（2026-08-12 实测）：TTB 竖排 3 字 → x-spans `[50,144]`
  （94px 应为单列 ~34px）+ y 连续——`advanceX += adv` 无条件水平累加，而
  `face.Glyphs` 的 TTB 版已把 vmtx 高度放进 glyph.Y（正确），drawGlyphs 又把
  同一 vertical advance 误加进水平 X → **每字斜移一格**。此前只在 face 层验证
  Y、未直连 Draw 像素路径。
- 性能面：drawMultiFace 逐 rune 调 drawSourceFace（每字独立光栅+线性扫 faces）。
- 修复：drawGlyphs 判断方向——竖排不累加 advanceX（Y 由 glyph.Y 驱动）；
  drawMultiFace 改按 face 连续 run 合并绘制（已随 P5 完成，见下）。

### P2. autohintCache 无上限（内存 ⚠️ 已确认）
- `render/text/autohint.go:782` `map[autoHintMetricsKey]*unscaledStyleMetrics` 只增
  不减，仅手动 ClearAutoHintCache 清空。多字体×多脚本累积。
- 修复：加 maxEntries + 超限清空（或 LRU）。

### P3. OwnShaper cache 无上限（内存 ⚠️ 已确认）
- `render/text/shaper_own.go:195` `map[*FontSource]*ownShaperCache` 只增不减。
- 修复：加容量上限 + 超限逐出。

### P4. face.go Glyphs / AppendGlyphs 重复 130 行（可读性 ✅ 已确认）
- `render/text/face.go:151-280` 两函数核心循环几乎相同（yield vs append）。
- 修复：抽 `iterGlyphs` 公共迭代器。

### P5. drawMultiFace / drawFilteredFace 逐字线性扫 faces（性能 ✅ 已修，2026-08-12）
- 位置：`render/text/draw.go`（drawMultiFace / drawFilteredFace / runAdvance）
- 现象：每个 rune 对 `mf.faces` 线性 `HasGlyph` 扫描（faces 多时 O(n×m)）+ 每个
  rune 单独调 drawSourceFace（每字独立光栅）+ `string(r)` 分配。
- 验证（临时像素探针，已删）：① 单 face MultiFace 画 "Hello, World!" 与直接
  sourceFace 逐字节一致（hinting=Full 生效，说明 run 合并后 snapPen 与直绘收敛）；
  ② TTB MultiFace 竖排 3 字 "日月目" 墨 x=[51,69] 单列、与直接竖排逐字节一致
  （旧实现逐字 currentX += vmtx 高度 → 每字斜移一格，属 P1 同族 bug，现已修正）；
  ③ 混合脚本 "Hello世界" 两 run，run2 起点 = runAdvance("Hello") 累计，与手工
  分段直绘逐字节一致。
- 修复：
  - `drawMultiFace` 改用 `mf.Runs(text)`（FaceRun 复用，含 M3 系统回退；S6.5 缓存
    命中免重算），按 run 一次 dispatch 绘制；竖排时 run 沿 Y 堆叠（X 恒为原点，
    glyph.Y 驱动），横排沿 X 累计 `runAdvance`。
  - `drawFilteredFace` 把连续 in-range rune 用 strings.Builder 合并为段，滤掉的
    rune 断段且不推进（布局与旧逐字一致）。
  - `faceGlyphAdvance`/`unhintedGlyphAdvance`（逐字）收敛为 `runAdvance`/
    `advanceFromGlyphs`（整段）：横排 sourceFace 走 TT hinted 宽度（与 drawGlyphs
    内部推进一致，run 边界 = drawGlyphs 光标落点），竖排/其他 face 走 Glyphs 原值。
- 行为收敛说明：单 face 文本 MultiFace 绘制 = 直接 sourceFace 绘制（此前每字
  snapPen 重基导致相位漂移）；M3 系统字体回退现与 Glyphs/Advance 度量一致。
- 回归：draw_test / draw_aliased_test / multi_test / filtered_test /
  shape_result_cache_test / zz_m6_vertical_test / system_font_test / varfont_test
  / zz_dbg_png_test 全绿。

### P6. ownParsedFont 懒解析模式分散（可读性 ✅ 已修，2026-08-12）
- 位置：`render/text/font_parser_own.go` + `cff_outline.go`
- 现象：13 个表槽各自 `xxxOnce sync.Once` + 缓存字段 + 哨兵 bool（hmtxParsed /
  vheaOK 等）+ 10~20 行 ensure 函数，重复样板，维护成本高。
- 验证：写临时摘要探针（testdata 全字体 + DejaVu/NotoCJK-TTC/FreeSans 系统字体 ×
  最多 3 个 collection index），对全部 13 个槽位跑 Name/FullName/GlyphIndex/
  GlyphAdvance/GlyphVerticalAdvance/GlyphBounds/Metrics/GlyphAdvanceVar/hinted
  Advance/GlyfContours/applyVariations(gvar+avar)/extractCFF/extractCFF2(含变体)
  → sha256 摘要。重构前后摘要 **完全一致**（`718e59c0...d115`），行为逐字节等价。
- 修复：引入泛型 `lazySlot[T]{once, err, val}`（`load(init) (T, error)`，init 至多
  跑一次），13 个槽位全部改为 `xxx lazySlot[typed]`；多值槽位用 typed 结构
  （hmtxLazy/vmtxLazy/nameLazy/metricsLazy）替代分散哨兵字段；每个 ensure*/load*
  方法统一为「返回 typed 值」。语义逐一保持：
  - vheaOK 独立于 vmtx（vhea 有效即置位，vmtx 缺失/损坏不回退）——原实现如此；
  - cff/cff2 错误持久缓存（每次调用返回同一 err）；
  - cmap 缺失 → ensureCmap 返回 nil → GlyphIndex 保持既有 nil-panic（潜在洞，
    另行评估，本次不改行为）；
  - glyf 缓存建失败 → 回退 ParseGlyfContoursFromTables（原行为）。
- 回归：摘要探针（删前最后跑）一致 + 24 个测试窗（cff/cff2/gvar/hvar/avar/
  varfont/varfont_outline/source/font_tables/parser_collection/glyf_parser/tt_glyph/
  tt_engine/glyph_outline/draw/draw_aliased/multi/filtered/m6_vertical/shape_cache/
  cache/face/wrap_tab）PASS=24 FAIL=0。

### P7. 逐字 string(r) 分配（性能 ✅ 已修，2026-08-12）
- 原记录位置 `draw.go:348/394`（drawMultiFace/drawFilteredFace 逐字 `string(r)`）
  **已被 P5 消除**；`runeToGlyphs` 查证本就 rune 直传、无分配。P7 实际落点：
  `multi.go`（MultiFace.Advance/Glyphs/AppendGlyphs 每 rune `face.Glyphs(string(r))`
  + 嵌套迭代器）与 `filtered.go`（FilteredFace.Advance 同模式）。
- 验证（临时探针，已删）：testing.AllocsPerRun 于 17-rune 混合串（含 CJK），
  改前/改后（git stash push 我的文件对比）：

  | 路径 | 改前 | 改后 |
  |---|---|---|
  | MultiFace.Advance | 64 | **0** |
  | MultiFace.Glyphs | 67 | **0** |
  | FilteredFace.Advance | 144 | **5**（整串迭代固有开销）|
  | sourceFace.Advance | 0 | 0（本就无分配）|
- 修复：
  - 新增 `sourceFace.glyphForRune(r, byteIndex, cluster)`：单字位置中性 Glyph、
    免分配，是 iterGlyphs 逐字主体的等价物（iterGlyphs 本身不动，保热路径零风险）；
  - `MultiFace.Advance/Glyphs/AppendGlyphs` 收敛到共享 `MultiFace.iterGlyphs`
    （faceForRune + glyphForRune），消除每 rune `string(r)` + 嵌套 Glyphs 迭代器；
  - `FilteredFace.Advance` 改走 `f.Glyphs(text)` 整串迭代（过滤语义不变）；
  - 包级 `glyphForRune` 分派：sourceFace/FilteredFace/MultiFace 走免分配路径，
    未知 Face 实现（测试 mock/自定义）回退单字 Glyphs 迭代器，保持任意 Face
    实现可用（此兜底是回归时抓到的洞：首版分派对 mock 返回 0 glyphs）。
- 刻意不动：`multi.go` runsUncached 的 `face.Advance(string(r))`（仅缓存 miss
  热一次，且其 X 记账对 TTB face 的语义与 glyphForRune 不同，改则动 S6.5
  布局）；`wrap.go:545 measureRune` 的 `Shape(string(r), face)`（shaping 开销
  占绝对大头，string 分配可忽略）。
- 回归：摘要探针（Glyphs 全字段/Advance/AppendGlyphs/Measure × sourceFace/变体/
  MultiFace/FilteredFace/TTB，重构前后 sha256 一致 `143b7d6f...0fa`）+ 17 窗
  PASS=17 FAIL=0。

### P8. glyph_cache.go 上限复核（验证项 ✅ 已收口，2026-08-12）
- 复核结论：GlyphCache（默认 4096、16 分片）的容量淘汰真实生效——`Set`/
  `GetOrCreate` 在分片满时 LRU 淘汰 tail，`Maintain` 按 FrameLifetime 帧淘汰；
  `Cache[K,V]`（cache.go）同有 evict。**上限语义**：每片 ≤ shard.maxEntries
  （= ceil(MaxEntries/16)），总条目 ≤ Σ 各片上限（MaxEntries 非 16 倍数时
  可有向上取整 slack，如 MaxEntries=33 → 最多 48）——分片均摊的设计取舍，
  非泄漏；旧测试用 32（整除）恰好掩盖此语义。
- 收口（新增 4 个显式测试，glyph_cache_test.go，非临时探针）：
  - `TestGlyphCache_ShardCapacity`：分片层硬上限 + 灌 200× 容量后总条目受各片
    上限之和约束 + Evictions>0；
  - `TestGlyphCache_EvictionStatsMatch`：单线程无 Delete/Clear 时
    Len == insertions − evictions（淘汰计数诚实）；
  - `TestGlyphCache_GetOrCreate_Eviction`：GetOrCreate 路径同走容量淘汰；
  - `TestGlyphCache_AllShardsUsed`：确定性构造下 16 片全用（无退化）。
- 内存语义记录：entries map 总容量 O(MaxEntries) 有界，不随文本/字形数增长。
- 回归：glyph_cache_test.go 全窗（30 测试）PASS。

## 二、验证计划（每项如何证明「真实存在」）

| 项 | 验证方法 | 判定标准 |
|---|---|---|
| P1 | 写 CPU 路径基准：单 face 串 vs MultiFace 串，逐字 drawSourceFace 次数用临时计数 | 逐字调用次数 = 字符数（证明无批量） |
| P1 竖排 | 渲染 TTB MultiFace 一段文字到像素，测墨量 Y 分布 | 若 Y 恒 0 = 竖排 MultiFace 仍横排（真 bug） |
| P2 | 加载多字体跑 autohint，观察 cache map 长度（临时探针） | 加载 N 字体后 len(cache) 增长且无回收 |
| P3 | 动态 NewFontSource 多次，观察 ownShaperCache 计数 | 只增不减 |
| P4 | 代码差异 diff（静态） | Glyphs/AppendGlyphs 核心重复段落 ≥30 行 |
| P5 | MultiFace 链 8 faces × N 字，HasGlyph 调用计数 | 每字扫全链 |
| P7 | `go test -bench` 或临时探针计数 string(r) 分配 | 每字 1 次分配 |

## 三、修改方案（验证后按序实施，每步回归）

1. **P1/P5 合并修**（✅ 已完成）：drawMultiFace 改为「按 rune 用 Glyphs 迭代一次 + 同 face
   连续 run 合并绘制」（FaceRun 复用），消除逐字光栅与线性扫描；顺带用 glyph.Y 修正竖排。
2. **P4**（✅ 已完成）：抽 `iterGlyphs` 公共迭代器，Glyphs/AppendGlyphs 共用。
3. **P2**（✅ 已完成）：autohintCache 加 maxEntries + 简单淘汰（如容量超限重置/清除最旧）。
4. **P3**（✅ 已完成）：OwnShaper cache 加容量上限（如 64 源）+ 超限清空；保持 ClearCache 语义。
5. **P6**（✅ 已完成）：ownParsedFont 懒解析收敛为泛型 lazySlot[T]（摘要探针证明前后等价）。
6. **P7**（✅ 已完成）：新增 glyphForRune 免分配单字路径 + MultiFace iterGlyphs 收敛
   （分配探针 64/67/144 → 0/0/5）。

## 四、约束
- 每次修改后跑对应测试窗回归；全量回归放最后。
- 不改变公共 API 语义；不破坏 M0–M7 + B/C/S1 已验证行为。
- 纯性能/可读性改动必须有「修改前后行为等价」的测试证据。
