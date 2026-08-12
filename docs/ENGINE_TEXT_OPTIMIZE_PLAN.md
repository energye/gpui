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

### P6. ownParsedFont 14 个 sync.Once 模式分散（可读性，低）
- 位置：`render/text/font_parser_own.go:83-140`
- 现象：每个表一个 Once+缓存字段+ensure 函数，重复样板；虽有收益（懒解析），
  但 14 份拷贝维护成本高。可收敛为「表加载器」辅助或统一 err/ok 模式。

### P7. string(r) 与逐字分配（性能小）
- `draw.go:348/394` `runeStr := string(r)` 每字分配；`runeToGlyphs` 等处同理。

### P8. glyph_cache.go / cache.go 是否有空表/退化路径
- 探索：GlyphCache 有 LRU+分片+上限（4096），Cache[K,V] 有 evict——先复核是否
  真的按 maxEntries 淘汰（验证时补内存测试）。

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
2. **P4**：抽 `iterGlyphs` 公共迭代器，Glyphs/AppendGlyphs 共用。
3. **P2**：autohintCache 加 maxEntries + 简单淘汰（如容量超限重置/清除最旧）。
4. **P3**：OwnShaper cache 加容量上限（如 64 源）+ 超限清空；保持 ClearCache 语义。
5. **P6**：ownParsedFont 懒解析收敛（低优先，若回归风险大则仅整理注释）。
6. **P7**：逐字 string(r) 改 range 直接传 rune（小改）。

## 四、约束
- 每次修改后跑对应测试窗回归；全量回归放最后。
- 不改变公共 API 语义；不破坏 M0–M7 + B/C/S1 已验证行为。
- 纯性能/可读性改动必须有「修改前后行为等价」的测试证据。
