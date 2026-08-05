# ENGINE_TEXT_SHAPING_PLAN — HbShaper 接入（shaping 层替换为 typesetting/harfbuzz）

**状态**: **M0 完成（2026-08-05）**——`HbShaper`（shaper_hb.go）实现：用 typesetting `font.ParseTTF`（TTC 回退）从 FontSource 原始字节构 `*font.Face` → `harfbuzz.NewFont` → `buffer.Shape`；16.12 定点（`XScale=size<<12`）输出像素 `Position/4096`（误差 ≤0.0003px）；tab/控制字符/IsCJK/RTL 视觉序对齐自研语义；feature 集复用 `collectDesiredFeatures` 转 `hb.Feature`（含用户 Value:0 → 显式 disable）。**M0 验证**：Latin parity 0/1122（DejaVuSans/LiberationSans/NotoSansMono × 12/16/24px × 5 样本，glyph 序列 + advance 全一致）；CJK 冒烟中文/日文 0 差异、NotoSansCJK 韩文 1 glyph 差异（入 M1）；全套无新增回归。M1（默认切换 + A 类全量）+ M2–M4 待做。**M1 完成（2026-08-05）**——默认 shaper 切到 HbShaper（shaper.go `defaultShaper`）；render/text 全套绿（仅既存 Rust_FullParity）；render/ 与 ui/rendering・ui/embedder 无新增失败。实测三证：①NotoSansCJK space——FT `FT_Get_Char_Index(0x20)=1` 与 Hb 一致，**自研 cmap 的 TTC 解析有 bug（gid 63108 错），HbShaper 输出更正确**；②复杂脚本自研 vs Hb 差异量化（Tamil "மொழி" own3:hb4、Khmer "ភាសាខ្មែរ" own9:hb6 全错、Deva/Bengali 多处 gid 差异）——证实自研复杂脚本会排错，HbShaper 是工业正确实现；③性能基准（i5-6200U）：Latin Own 76µs vs Hb 99µs（自研快 30%）、**CJK Own 294µs vs Hb 61µs（Hb 快 4.8×）**、Indic Own 146µs vs Hb 231µs（Hb 慢 59%）→ M4 分派不能按 Latin/CJK 简单二分，需按脚本+数据决策。**M2 完成（2026-08-05）**——复杂脚本交叉验证：purego 绑定系统 `libharfbuzz.so`（2.7.4）作原生参考，scale 统一为 upem（font units 整数），12 字体 × 3–5 样本 × 128 glyphs 逐 glyph 对比。判据（用户裁定）：**只验证接入正确性，不审计移植库**——骨架（gid/cluster/xoffset/yoffset）必须与原生一致；advance 仅记录不判定。结果：**skeleton-mismatches=0**；advance 记录 1 处已知版本差异（sinhala GDEF-mark 0xDC6 的 advance：原生 2.7.4 保留 348、移植按通用规则清零——已实证该 glyph 在 GDEF 中 class=3 mark，移植行为符合 HarfBuzz 通用规则）。途中修复本测试代码 2 处：①struct 布局按 20B/项（`hb_glyph_info_t`/`hb_glyph_position_t` 含 var1/var2，非 16B）；②移植侧必须设 `Props.Script`（缺则 Indic 数量级差异 go=5 vs native=4）。已验证脚本：Devanagari/Bengali/Tamil/Gurmukhi/Gujarati/Kannada/Malayalam/Telugu/Odia/Sinhala/Khmer/Tibetan。缺字体未验：Thai/Arabic/Hebrew/Myanmar/Lao（系统无字体，`hb-shape` CLI 亦无）。**M3 完成（2026-08-05）**——C 类两项接入：①segmenter 断行：`wrap.go` 的 `findBreakOpportunities` Word 模式改用 typesetting/segmenter 完整 UAX#14（断点收集 `Line.Offset>0` + `IsMandatoryBreak` 行尾属性配对），WrapChar 保留逐字语义、WrapNone 不变；wrap_test 全绿，新增 zz_m3_segmenter_test.go（泰文 SA→AL 不拆字、标点簇实测行为 `,`/`.`/`+` 前后不断 `,`/ 后断、`\n` BK mandatory 在后、CJK/引号对回归）。②fontscan 缺字回退：新增 `fontscan_fallback.go`（懒初始化 `fontscan.SystemFonts(logger, cacheDir)` 落盘索引；`RuneSet.Contains` 按覆盖选字、aspectScore 偏向 regular；rune→path 与 (rune,size)→Face 双层缓存），`MultiFace.faceForRune` miss 时经全局 fallback 解析（faces 链保持不可变零锁）；验证：Deva/CJK 命中系统字体（Lohit/NotoSansCJK）、未分配码位降级 base face。回归：render/text 仅既存 Rust_FullParity；render/ 7 失败 stash 验证 6 既存 + 1 flaky（BlendHue）零新增；ui 层全绿。**M4 完成（2026-08-05）**——性能分派决策：**全走 HbShaper，不做双路径分派**（用户裁定：以准确为标准，性能非目标；shapeResultCache 兜底，后期有需要再引入分派）。自研 OwnShaper 保留为可选 shaper（SetShaper 能力），不再默认使用。**计划完成**：A 类（GSUB/GPOS/Indic/Arabic/变体/cmap）→ typesetting/harfbuzz；B 类（tt_*.go/autohint/栅格化/emoji/缓存）保持自研；C 类（segmenter 断行 + fontscan 回退）接入。遗留：Thai/Arabic/Hebrew/Myanmar/Lao 字体验证（系统无字体）、Rust_FullParity 既存失败清理、autohint 蓝区 18 脚本（见 §6）
**日期**: 2026-08-05
**触发**: 用户目标「跨平台 GUI 控件库，对齐 skia/flutter 工业架构」；盘点结论——自研 shaping 层（约 6,600 行）与 HarfBuzz 功能重复且覆盖不全（Indic 850 行 vs HB 2,000 行、无 USE/泰/缅/高棉/Hebrew shaper、无 NFC、无 AAT），像素层（hinting + 栅格化）为自研独有资产
**关联**: docs/ENGINE_TEXT_FREETYPE_PLAN.md §9.4（遗留项「复杂 shaping HarfBuzz 对齐」）、docs/ENGINE_TEXT_HINT_LIGHT_PLAN.md §9（typesetting 列为 shaping 里程碑候选）、AGENTS.md（render/ 高风险，实现前须评估影响面）
**参考**: FreeType 2.11.1 本地镜像、go-text/typesetting v0.3.4（go.mod 已有依赖，cff_outline.go 已用其 font/cff）
**用户确认**: 「重复的用 HarfBuzz，没有的用自研」；「像素层永远自研」；本计划只写文档，实施另行触发

---

## 0. 用户目标（硬）

- **分工原则**：shaping 层（A 类）替换为 typesetting/harfbuzz（HarfBuzz Go 移植）；像素层（B 类）永远自研，不动。
- **运行时零依赖不变**：typesetting 为纯 Go（无 CGO），符合 ADR-048 纯 Go 字体栈。
- **对齐工业架构**：Chromium/Skia/Flutter 均为「HarfBuzz 排字 + FreeType/Skia 像素层」——本计划即把自研栈拼成同一分工。
- **白得能力（C 类）**：AAT、NFC 归一化、USE/泰/缅/高棉/Hebrew/Hangul shaper、segmenter 断行、fontscan 字体匹配，接入即得，不额外实现。
- **验证闭环**：现有全部测试保持绿；复杂脚本新增离屏/逐字对照验证。

---

## 1. 背景：自研 vs typesetting 全量对照

### 1.1 A 类（重复 → 换用 typesetting，HarfBuzz 更全）

| 自研文件 | typesetting 对应 | 说明 |
|---|---|---|
| gsub.go / gsub_context.go | `harfbuzz/ot_layout_gsub.go` | GSUB 引擎（含上下文/链式） |
| gpos.go / gpos_context.go / gpos_mark.go | `harfbuzz/ot_layout_gpos.go` | GPOS 引擎（单/双/curs/mark 全家） |
| gdef.go | harfbuzz 内嵌 | 字形类 |
| kern.go | `harfbuzz/ot_kern.go` | kern 表 |
| script.go | `harfbuzz/ot_language.go` | 脚本/语言表 |
| indic_shaping.go / indic_reorder.go / indic_font_pos.go（1,160 行） | `ot_shape_indic*`（完整 11 脚本） | 自研只覆盖部分序列 |
| arabic_joining.go（211 行） | `ot_shape_arabic*` | 含 Stch/Tatweel，自研无 |
| gvar.go / gvar_deltas.go / hvar.go / avar.go / variation_coords.go / item_variation_store.go | `font/variations.go`（gvar/hvar/avar/fvar/mvar 全） | 可变字体 |
| font_cmap.go | `font/cmap.go` | 字形映射 |
| font_tables.go 等表解析 | `font/opentype/tables/`（生成式全表） | 表定义 |

### 1.2 B 类（自研独有 → 保留不动，HarfBuzz 无此层）

| 模块 | 功能 | 对应 FreeType |
|---|---|---|
| tt_*.go（30 文件） | TT 指令引擎（skrifa hint/engine 移植） | ttinterp.c |
| autohint_*.go（8 文件） | 自动 hint（FT light 逐点对齐进行中） | autofit/ |
| draw_aliased.go / lcd_filter.go / subpixel.go / msdf/ / glyph_mask_* | 光栅化 + 抗锯齿 + LCD + MSDF | ftraster/ftsmooth |
| draw_emoji.go / color_font.go / emoji/ | emoji/彩色字体渲染 | （FT 无对应完整层） |
| glyph_cache.go / shape_result_cache.go / mask atlas | 性能缓存层 | — |

### 1.3 C 类（typesetting 有而自研没有 → 接入即白得）

1. AAT（morx/kerx/trak——老 Mac 字体格式）
2. NFC 归一化 + USE/泰/缅/高棉/Hangul/Hebrew 专门 shaper
3. Unicode 属性大表（emoji 判定等）
4. segmenter（Unicode 断行/分词——泰语无空格断词）
5. fontscan（按语言自动匹配系统字体，与自研 system_font_* 互补）

---

## 2. 方案：HbShaper（实现现有 Shaper 接口）

### 2.1 架构

```
FontSource（原始表字节）
   ↓ typesetting/font.ParseTTF / ParseTTC   （已有 go.mod 依赖，cff_outline.go 同源）
font.Face（*tables 全表）
   ↓ harfbuzz.NewFont(face)
harfbuzz.Font（XScale/YScale = upem，像素 = fontUnit × scale / upem）
   ↓ buffer.SegmentProperties{script, language, direction, variations} + features
buffer.Shape(font, features)
   ↓ 转换
[]ShapedGlyph（自研结构，走现有渲染管线）
```

### 2.2 接线要点

- `HbShaper` 实现 `text.Shaper` 接口（shaper.go:10），`SetShaper(&HbShaper{})` 全量替换默认 OwnShaper。
- Face 构造：自研 `FontSource` 已有原始表字节（`getRawTables`），传给 `font.ParseTTF`/`ParseTTC`；解析结果按 FontSource 缓存（face 只读，可复用）。
- 尺寸：自研 face.Size()（px）→ `Font.XScale = YScale = size × 64`（26.6 精度，与渲染网格一致）。
- 特性集：自研 `FontFeature` → `harfbuzz.Feature`（tags + enabled）。
- 方向/脚本/语言：由文本段检测（复用自研 bidi/script 检测或 HB 自带）。
- 输出转换：`buffer.Info`（glyph ID + cluster）+ `buffer.Pos`（advance/offset，26.6）→ `ShapedGlyph`；cluster 用于下游文本定位/光标。
- 缓存：复用现有 shapeResultCache（key 不变，S6.5 契约不破坏）。
- **分派策略（决策点）**：先全量走 HbShaper；M4 按性能基准决定是否「复杂脚本 → HB、Latin/CJK → 自研」双 shaper 分派（自研轻量路径为拉丁/中文保留）。

### 2.3 明确不动的（B 类）

tt_*.go、autohint_*.go、光栅化全链、emoji/彩色字体、缓存层——HarfBuzz 无像素概念，本计划不触碰。

---

## 3. 验证

1. **回归**：`go test ./render/text/` 全绿（含 autofit 矩阵、golden、shape 相关测试；仅存的 Rust_FullParity 既存失败除外——由 scripts.go 遗留改动引起，另行处理）。
2. **字形序列对照**：Latin/中文/日文/韩文样本——HbShaper vs 自研 OwnShaper 的 glyph 序列 + advance 一致性（应等价；差异逐条评估）。
3. **复杂脚本新验证集**（自研无法覆盖，直接以 HarfBuzz 为标准）：Devanagari/孟加拉/泰/缅/老挝/阿拉伯/希伯来样本（Noto 系列系统字体），验证：连字、重排序列、mark 定位、RTL。
4. **像素层回归**：hint + 栅格化链路输入换 HbShaper 输出后，现有 light 对照矩阵（ftexp）不变。
5. **性能基准**：拉丁长文本 + 中文长文本——HbShaper vs OwnShaper 时间比；超阈值则触发 M4 分派决策。

---

## 4. 里程碑

- **M0** HbShaper adapter 最小可跑：Latin 文本走 HB 输出，现有测试无回归。
- **M1** A 类全量替换 + 全测试绿 + 字形序列对照通过。
- **M2** 复杂脚本验证集（Indic/泰/缅/老挝/阿拉伯/希伯来）逐字验证通过。
- **M3** C 类接入：segmenter 断行接入 wrap.go、fontscan 字体回退接入 MultiFace 链。
- **M4** 性能双路径决策：全 HB vs 分派（按基准数据定，需 question 确认）。

---

## 5. 风险与缓解

| 风险 | 缓解 |
|---|---|
| HB 全量引擎对拉丁/中文比自研轻量路径慢（预计 2-5×） | shapeResultCache 已兜底；M4 分派决策保留自研快路径 |
| 字体数据双解析（自研 + typesetting）内存/启动开销 | face 按 FontSource 缓存、lazy 构建；只解析 A 类所需表 |
| typesetting 依赖升级风险 | 锁 v0.3.4；引入前读 changelog |
| HB 输出 26.6 精度与自研渲染网格对齐问题 | XScale=size×64 与渲染层同精度；M0 即验证 |

---

## 6. 后续（不在本计划）

- autohint 蓝区补全 18 脚本（独立计划，质量层）
- Rust_FullParity 既存失败清理（scripts.go 遗留改动处理）
- typesetting 全量脚本表 vs 自研 script.go 的差异评估（切 HB 后 script.go 逐步退役）
