# 文本渲染 FT 全对齐计划（Raster-FT-ALIGN）

**状态**: **阶段 B 完成**（2026-08-09 阶段 A 完成；B1a CFF 扫描升级 → B1b autohint 26.6 直通 → B1c TTF 矩阵 ✅ → B2 位图矩阵 ✅，2026-08-11 B 验收全矩阵 bad=0）
**目标**: ①光栅器与 FT `smooth/ftgrays.c` **逐字节一致**；②8–72px 全字号 × 全字集 × 全字体验证矩阵；③M0–M5 实现方式源码级审计（不信文档）。
**对照基准**: 本地 FT 2.11.1 源码 `/home/yanghy/app/projects/gogpu/freetype-2.11.1/` + `ftexp` 二进制（purego 调系统 libfreetype 2.11.1）。

---

## 1. 现状差距（2026-08-09 实测，非文档宣称）

### 1.1 光栅器：算法不同，位图不可能逐字节一致

- 自研 `render/internal/raster`（`analytic_filler.go` 2358 行）＝解析式边缘覆盖，4×4 子像素网格（aaShift=2）→ 256 级。
- FT `src/smooth/ftgrays.c`（2228 行）＝**cell cover/area 记录 + sweep 累加**，`PIXEL_BITS=8`（256 级），26.6 定点输入。
- **实测差距**（`TestDbgRasterSource`，Noto Sans CJK，FT 轮廓 → 自研光栅 vs FT 位图，bestShift±3 对齐后逐点非 0 差数）：

| px | 静 | 每 | 合 |
|---|---|---|---|
| 12 | 73 | 77 | 62 |
| 16 | 133 | 110 | 90 |
| 24 | 215 | 165 | 162 |
| 32 | 283 | 235 | 209 |
| 48 | 428 | 356 | 315 |

结论：**mism 在光栅器本身**（FT 轮廓喂自研光栅与生产轮廓喂自研光栅 mism 相同），且随字号线性放大。

### 1.2 验证矩阵：字号/覆盖不足

- 现有扫描字号：M 系 10/12/14/16/20/24；wqy/Latin 10–16；afindic/scriptdispatch 12/16；bytecode 8/12/16。
- **8–72px 未铺满**（缺 8、18–24 中间档、28–72 大字号）。

### 1.3 M0–M5 扫描盲区（本次已实锤 2 例）

- 现有扫描只对照**坐标 + 点数**，不对照 on/off 标记与轮廓分组。
- 实锤：静 12px（on/off 错位）、合 16px（尾部 off 闭合漏段）——两者扫描全绿但渲染有缺陷。
- 单 mask（hintmaskCount≤1）字形不在 M3 扫描集（multi2 集合只含多 mask 字）。

---

## 2. 阶段划分（顺序执行）

### 阶段 A：光栅器逐字节对齐 FT（当前阶段）

**A1 对照基线**
- 扩 `ftexp`：单字形位图模式已具备（P2 PGM + bitmap_left/top + 亚像素 delta 偏移）。
- 建 `TestRasterFTByteExact`：同 26.6 轮廓 → 自研光栅 vs FT 位图，**逐字节（含 0 值）** 对比，bitmap_left/top 对齐，记录 bad 计数（现基线 ≈ 上表）。

**A2 移植 ftgrays.c 到 Go**（新文件，不动旧 raster 包，可并存切换）
- 移植范围（按 ftgrays.c 逐函数）：
  - cell 结构 `{x, cover, area}` + 链表池 `gray_set_cell`（line 572）
  - `gray_render_scanline`（line 636，含跨 cell DDA 细分 `delta/mod` 累进）
  - 竖线/水平线特判 `gray_line_to`（line 731）
  - conic（line 869）/ cubic（line 1044）De Casteljau 细分（`gray_render_conic`/`gray_render_cubic`，half-way 切分 + 直段近似）
  - 段到线近似 `gray_curve_to`/`gray_split_conic` 等（line 1237–1475）
  - `gray_sweep`（cover/area 累加 → 256 级输出，`FT_FILL_RULE` 非零/奇偶）
  - `gray_move_to`（line 1426）闭合语义
- 输入：26.6 定点段（与 FT `FT_Outline` 同构）；输出：灰度位图 + left/top。
- 坐标约定：FT 位图 origin 在**左下**，自研 mask origin 在**左上**——输出前对齐。
- 亚像素：FT 在 load 时 26.6 已含 subpixel（delta）；自研光栅另加 subpixelX/Y 平移等价。

**A3 接入生产路径**
- `GlyphMaskRasterizer` 光栅段由自研 raster 切到 ftgrays 移植（同 26.6 定点输入，避免 float 二次损失）。
- LCD 子像素路径（现 `lcd_filter.go`）保留，灰度主路径全走新光栅器。

**A4 逐字节回归**
- 对照矩阵：Noto Sans CJK 静/每/合/日/田 + wqy 抽样 × 8/12/16/24/32/48/72px，逐字节 bad=0。
- 全量 `go test ./render/text/...` 回归（不破坏 M0–M5 轮廓层对照）。

**A 验收**: 阶段 A 结束 = FT 轮廓喂新光栅器 vs FT 位图逐字节 bad=0（矩阵覆盖以上字号 × 抽样字符）。

**阶段 A 状态（2026-08-09）**：
- **A1 ✅** `TestDbgRasterByteExact` 基线已建（zz_dbg_raster_byteexact_test.go）。
- **A2 ✅** `render/text/ftgrays.go` 移植完成（新文件，旧 raster 包不动，可并存切换）：
  - `RasterizeFT26` 与 FT_Render_Glyph **逐字节一致**（bad=0），关键修复：cubic tag 解析（此前 tag=2 误判 conic 致 renderCubic 从未真跑）、UPSCALE 移到回调层（vMiddle 语义对齐）、splitCubic 7 元素视图、yShift 补 +64*rows（ftsmooth.c:480）、FT_UDIV 乘法移位近似、四方向 renderLine 带符号条件。
- **A4 ✅** 矩阵：Noto Sans CJK 静/每/合/日/田 × 8/12/16/24/32/48/72px **bad=0**；cjk3000 全字集 × 8/12/16/24/32px **15000 字全绿**；latin_all 254 / th_all 87 / kr_all 11172 **全绿**（`zz_dbg_ftgrays_*_test.go`）；wqy-microhei（TTF/conic 路径）17 字 × 7 字号 **119 组合 bad=0**（`zz_dbg_ftgrays_wqy_test.go`）。ftexp 新增 `bpgm` 批量位图模式（与 bcontour 对称，一次进程取全字集）。
- **A3 ✅ 2026-08-09 完成**：`GlyphMaskRasterizer.RasterizeHintedFT26` 直通方法——CFF light 引擎 `hint.LightHintVar` 的 26.6 定点输出直接喂 `RasterizeFT26`（跳过 float32 中转，方案 A）。关键修复：**CFF off 控制点必须映射 ftTagCubic**（CFF charstring 曲线是三次贝塞尔 curveto，FT_Outline 对 CFF 字体 off 点标 FT_CURVE_TAG_CUBIC；LightPt.On 只有 bool 丢失曲线类型，标 conic 会按二次渲染致错）。CFF2 变体坐标经 `cff2VariationCoords` 传入。验证：直通 vs FT-light 位图逐字节一致——48 组合（8 字 × 6 字号）+ cjk3000 × 12/16px = 6000 字全绿。生产接入：draw.go 在 hint 非 None + AA 模式优先走直通，不可用回退老 float32 路径。全量 `go test ./render/text/...` 回归绿。

### 阶段 B：8–72px 全字集全字体矩阵

**B 前置盘点（2026-08-09 实测，阶段 A 收口时补查）**
- CFF 扫描现状（hint 包 zz_scan_3000/kr/th）：仅 6 档（10/12/14/16/20/24），只对照**坐标+点数**；ftexp bcontour 已输出 tag（`x26 y26 tag`）但解析后丢弃——**on/off 标记与 contours 分组对照缺失**（1.3 盲区未补）。
- TTF 扫描现状（zz_wqy_scan / zz_light_bytecode_scan）：`matchPointSets` 贪心最近邻 ±64（0.5px）容差模糊匹配，**非逐点精确**——因 autohint 输出链 `26.6 定点 → contoursToOutline float32 → 重建 26.6` 存在固有精度损失（autohint 内部本来就是 26.6：`hintPoint.x/y`、`hintEdge.opos/pos`，丢精度只在 `contoursToOutline` float32 转换）。
- 引擎能力边界：`hint.LightHintVar`（自研 cf2 light）**只支持 CFF1/CFF2**；TTF/glyf 走 render/text 层 autohint（`autoHintContourPoints` → `GlyfContours` 26.6 Y-up，`pp1x` 平移后写回）。
- 字体资源：wqy-microhei-nohint.ttf / wqy-microhei.ttf（testdata）；系统有 DejaVuSans / FreeSans / Samyak-Devanagari / Samyak-Tamil / Samyak-Gujarati / Mukti / KacstOne / AbyssinicaSIL / PadaukBook；testdata 有 NotoSansThai-Regular.otf、NotoSansSC-VF.otf、source-sans/VF/（SourceSans3VF-Upright.otf CFF2）。**缺 Noto Sans KR/Thai 独立字体 → KR 用 NotoSansCJK TTC 的 KR face（m2Font Face(14)，已有先例）**。

**B 决策（2026-08-09 用户确认）**
1. **TTF 也逐点 26.6 精确对照**：改造 autohint 输出保留 26.6 定点（类比 CFF 的 lightPtsToFT26 直通思路），不再用 ±0.5px 模糊匹配。
2. **全矩阵 65 档所有字体全跑**：8–72 每 1px，CJK 全量、其余按文档字集全跑（接受 30–60 分钟长时）。

**B1 轮廓级矩阵（坐标 + 点数 + on/off 标记 + contours 分组，四维度）**
- **B1a CFF 扫描升级**（快，纯测试层）：
  - ftexp bcontour 解析保留 tag（已输出）与 contours 分组（`NCONTOURS` 头 + 每点 tag 已齐，需解析末点索引序列）；hint 侧 `LightPt.On` + `res.contours`（点数制）对齐 FT `ends`（末点制）。
  - 对照规则：CFF off 点 = FT tag2（cubic）；on 点 = tag1。点数、on/off 逐点、contours 分组逐轮廓，任一不符 = bad。
  - 档位：8–72 每 1px（65 档）× cjk3000 × Noto CJK TTC；kr_all 11172 × Noto CJK TTC KR face；th_all 128 × NotoSansThai。
  - VF：NotoSansSC-VF.otf × 4 wght（200/400/600/800）、SourceSans3VF-Upright.otf × 4 wght，抽样字集（CJK 300 常用 + latin_all）。

**B1a 状态（2026-08-09 收口）**:
- ✅ `zz_b1_scan_test.go`（hint 包）三测试全绿：`TestB1ScanCJK3000`（cjk3000 65 档）、`TestB1ScanKR`（kr_all 11172 × 65 档，11172×65 ≈ 72.6 万字形四维度对照）、`TestB1ScanThai`（th_all 128 × 65 档）——坐标 + 点数 + on/off + contours 分组四维度 bad=0。空块仅大规模 exec 偶发，小批量重试语义（禁止静默假绿）；长跑单文件给足超时。
- ✅ `zz_b1_vf_test.go`（render/text）两测试全绿：`TestB1VFNotoSansSC` / `TestB1VFSourceSans`（65 档 × 4 wght × CJK300 常用 + latin_all）——此前 SourceSans 矩阵累计 266 bad 集中于 66–72px 大字号，**坐标差 1（26.6）**，根因与修复见下：
  - **修复 ①（TestScanKR 偶发 FAIL 根因）**：ftexp（Go+purego+libfreetype）把 `os.ReadFile` 的字体字节 `&data[0]` 传 `FT_New_Memory_Face`——uintptr 不建 GC 引用，data 最后一次使用后底层数组被 GC 回收，FT 持有悬垂指针偶发读到被覆盖的字体内存 → 个别字形轮廓输出「混合」数据（np 漂移、坐标命中其他字形）。原版同矩阵 bad=49530，修复（各函数末尾 `runtime.KeepAlive(data)`）后 50/50 全一致。
  - **修复 ②（SourceSans 变体 266 bad 根因）**：`cff2Region.evaluate16`（cff2.go）区间插值用纯截断除法 `(v-start)<<16/(peak-start)`；FT 的 `cff_blend_build_vector`（cffload.c:1526-1530）用 `FT_DivFix` = `((a<<16)+(b>>1))/b`（**四舍五入**，ftcalc.c:266）。标量差 1 个 16.16 单位，经 delta 放大后偶发跨 26.6 边界 → 坐标差 1。CFF1 无 blend 故此前全对齐 —— 与「CFF1 全绿、CFF2 偶发差 1」现象自洽。修复后 SourceSans 全矩阵 bad=0（509s），NotoSansSC 588s 全绿。
- 遗留清理：`b1_dbg3_test.go` / `b1_dbg4_test.go`（单字形调试打印，无断言）已删除。
- **B1b autohint 26.6 输出改造**（引擎层，类比 A3 直通）：
  - `autoHintContourPoints` 已产出 26.6 定点（`GlyfContours.Points` 存 26.6 Y-up int16）；新增直通出口：hint 后不转 `contoursToOutline` float32，直接输出 26.6 点列 + on/off（`OnCurve`）+ 分组（`EndPts`）→ 喂 RasterizeFT26 或对照 ftexp。
  - 对照方：TTF 字体的 FT-light 轮廓（ftexp bcontour，tag0=conic off / tag1=on）；conic 隐含 on 中点规则与 FT 一致（glyf 双 off 拆中点，与 FT_Outline 输出点列相同）。

**B1b 状态（2026-08-10 收口）**:
- ✅ `zz_b1_ttf_scan_test.go`（render/text）`TestB1TTFWqyCJK` 全绿：wqy-microhei-nohint × cjk3000 × 10/12/16/24px，四维度（点数/坐标/on-off/分组）bad=0（26s）。此前 bad 2→1→0 的调试链：FT trace 补丁（ZZWEAK/ZZEDGE/ZZSEG/ZZSTAGE/ZZPOINTF/ZZCONTOUR/ZZSEGS，afcjk.c/afhints.c，stdbuf -o0 解 Go os.Exit 不 flush C stdout）逐阶段对照 weak 分类（一致）、IUP 公式（af_iup_interp 手算复现 p02=473/p05=82/p09=-55/p14=87/p17=288）、段级 link/serif（引擎与 FT 完全一致）、边级 serif 转换——**根因**：
- **修复（菊 12px d1 e4 根因）**：`computeEdges` 边级 serif 转换缺 FT `is_serif` 语义（afcjk.c:1248：`seg->serif && seg->serif->edge != edge`，serif 段指向**自己所在边**不算 serif）。菊 e4（fpos=936）的 s08 段 serif 指向同边 s06 段，引擎误把 e4.serif 设为**自环**（serif=4），pass1 对齐 `pos = serifEdge.pos + (opos-serifEdge.opos)` 对自环恒等 → e4 停在 351；FT 忽略 s08 后 e4.serif=e3（193+144=337）。连锁 IUP 锚点 p18=229 vs 226 → pt17 290 vs 288。修复后全矩阵 bad=0。
- B1b 期间附带修复（均回归全绿）：`computeEdgeDistThreshold` 补 FT_DivFix 四舍五入（幅24 清零）；`computeEdges` 段-边距离 tie-break 按 fpos（afcjk.c:1095）；边排序 major_dir 按字形轮廓方向（afhints.c:942-949，wqy 逆时针翻转 V 轴）；`alignStrongPoints` 边搜索精确二分（fpos 重复 linear=首条/binary=中条，afhints.c:1471-1537）；`computeStemWidthCJK` 无 STEM_ADJUST 原样返回（afcjk.c:1547）。
- FT 调试补丁已全部回退（freetype-2.11.1 干净重建），ftexp 走 testdata 源码在 TempDir 重建（链接当前库）。
- **B1c TTF 矩阵**：65 档 × 字集 × 字体——
  - wqy-microhei-nohint.ttf × cjk3000；wqy-microhei.ttf（字节码字体，light 仍走 autofit）× cjk3000 抽样 300；
  - latin_all 254 × FreeSans / DejaVuSans（SourceSans3VF 为 CFF2，归属 B1a VF 矩阵 `TestB1VFSourceSans`，不重复进 TTF 矩阵）；
  - deva_sample × Samyak-Devanagari；beng_sample × Mukti；taml_sample × Samyak-Tamil；
  - arab_sample × KacstOne；ethi_sample × AbyssinicaSIL；mymr_sample × PadaukBook；gujr_sample × Samyak-Gujarati。

**B1c 状态（2026-08-11 收口）**:
- ✅ 全部 11 个字体 65 档（8–72px 每 1px）四维度（点数/坐标/on-off/分组）bad=0：`TestB1TTFWqyCJKFull` / `TestB1TTFWqyMicroheiFull` / `TestB1TTFLatin` / `TestB1TTFMyanmar` / `TestB1TTFIndicScripts`（deva/beng/taml/gujr/arab/ethi，373s）。
- ✅ **6 脚本缺口转正**：deva/beng/taml/gujr/arab/ethi 此前为 gap 期待（仅统计不判红），2026-08-11 起撤 gap 进正式矩阵。根因修复：
  - **sameSign（autohint_segments.go）负数×0 象限判定**：FT pass2 用 C `(in^out)>=0`（负^0 符号位保留 → 判异象限），引擎旧实现把 (负,0) 判同象限 → arab `ف` pt11（in=(-203,360)、out=(0,0)）被误判 weak（FT 判 STRONG），连锁 8 个坐标差 1–11。改为 `b < 0` 后 arab/ethi/gujr probe 0 DIFF、全矩阵 bad=0。
  - **neutral 蓝区例外（autohint_edges.go computeBlueEdges）**：FT aflatin.c:2548-2558 对 `isTop ^ isMajorDir || isNeutral` 判候选，neutral 区无视方向恒为候选；旧实现漏掉 neutral 例外 → deva 基座区（ref 682, Top|Neutral）没吸到 fpos=673 的 serif 边，停在 opos 505（FT 蓝线吸附 512）。
  - **首 stem 豁免 bound check（autohint_edges.go alignStemEdges）**：FT aflatin.c:3356-3375 的越界回退只在非锚点分支（positionSubsequentStem）执行，首 stem（positionFirstStem）豁免；旧实现无条件执行 → ethi `ሠ` 12px 锚点边 1 落到 81（低于未 hint 的 serif 边 0 的 87）后被强行回退。
  - **空宽度表 zero-width 复刻（autohint_widths.go computeStandardWidths）**：FT afhints.c af_sort_and_quantize_widths 空表也把 count 提为 1、读首个清零条目 → stdw=0、edt=0；旧实现用 derivedConstant 兜底（edt=5）→ gujr `ટ` 竖直轴两条顶边被错误合并。复刻 zero 后边合并与 extra-light 标记（0*scale<0.625）均与 FT 一致。
  - **允许差清空**：此前唯一允许差 gujr px12 U+0A91 +0.2px 随 sameSign 修复消失（U+0A91 在 gujr_sample.txt，65 档实测 bad=0）。
- ✅ `TestB1TTFProbe` 转正为 12px 单档健康检查（11 字体全 match 判红）。
- ✅ 回归：`PASS=96 FAIL=0 SKIP=0`（render/text 逐文件，2026-08-11）。

**B2 位图级矩阵（逐字节）**
- 字号：8/10/12/14/16/18/20/24/28/32/40/48/56/64/72 阶梯（15 档）× 全字集。
- 逐字节 bad=0（新光栅器 RasterizeFT26；CFF 直通 + TTF 26.6 直通均喂同一光栅器）。
- CFF：cjk3000 × Noto CJK；kr_all × Noto CJK KR face；th_all × NotoSansThai。
- TTF：wqy-microhei-nohint × cjk3000 抽样 300（B2 全量 TTF 位图时长不可控，抽样 + latin_all × FreeSans/DejaVu 全量）。

**B2 状态（2026-08-11 收口）**: `zz_b2_bitmap_scan_test.go`，四组全矩阵逐字节 bad=0
- ✅ CJK：NotoSansCJK-Regular.ttc face0(JP) × cjk3000 × 15 档 45000 字逐字节 bad=0（267s）。
- ✅ TTF：wqy-microhei-nohint × cjk3000 抽样 300 + latin_all × FreeSans/DejaVuSans 全量 × 15 档 bad=0（61s）。
- ✅ Thai：hint/testdata/NotoSansThai-Regular.otf × th_all 128（有效 87）× 15 档 bad=0（7.3s，2026-08-11 修正字体路径假绿：原引用顶层 testdata 不存在走 t.Skipf，改为 hint/testdata 后真跑全绿）。
- ✅ KR：NotoSansCJK-Regular.ttc face1(KR) × kr_all 11172 × 15 档逐字节 bad=0（826s 重跑，含 faceIdx 对照；首跑 1345s）。
- ✅ 回归：`PASS=95 FAIL=0 SKIP=0`（render/text 逐文件，2026-08-11，含 zz_b2_bitmap_scan_test 全部 4 组；另清理 zz_dirrepro/zz_probe_gap 两个无断言探针，不计入）。
- ftexp 新增：bcontour/bpgm 可选 faceIdx（TTC face 序号，KR face = index 1，`ftexp faces` 枚举）；纯 CFF CFF2 位图对照沿用既有 bpgm 模式。

**B 验收（2026-08-11 达成）**: B1/B2 全矩阵 bad=0（含 on/off、分组维度），允许差已清空（原 gujr px12 U+0A91 随 sameSign 修复消失，2026-08-11）。

### 阶段 C：M0–M5 源码级审计

- C1：按 FT 2.11.1 源码逐文件审 hint 包实现方式：
  - `cffcs.go`（Type2 解释器）↔ `cf2intrp.c`/`cffgload.c`
  - `psh_light.go`（cf2Blues/cf2HintMap）↔ `cf2blues.c`/`cf2hints.c`
  - `autohint_*.go`（glyf autofit light）↔ `autofit/afcjk.c`/`aflatin.c`/`afindic.c`
  - `tt_*.go`（TT bytecode）↔ `ttinterp.c`/`ttgload.c`
  - `cff_light_bridge.go`/`light_api.go`（接线）↔ `ftobjs.c` light 路由
- C2：审计结论落 `docs/ENGINE_TEXT_HINT_LIGHT_PLAN.md` 修订表（含与文档不符的修正）。
- C3：扫描补单 mask 字形集（hintmaskCount≤1 也进扫描）。

**C 验收**: 审计逐项结论 + 修订表；单 mask 扫描窗 bad=0。

---

## 3. 门禁总则

- 每阶段完成必须 `go test ./render/text/...` 全量回归（含之前所有阶段）。
- 禁止为装绿改阈值/降采样；允许差必须显式声明并落文档。
- 测试长跑 + 外部字体缺失 → `t.Skipf` 提示，禁止静默假绿。
- 状态更新只写本计划文档 + `docs/ENGINE_TEXT_HINT_LIGHT_PLAN.md`（不碰 UI WR §2/§3 真源，除非涉及）。

## 4. 参考

- FT 光栅器：`/home/yanghy/app/projects/gogpu/freetype-2.11.1/src/smooth/ftgrays.c`
- ftexp 工具：`render/text/hint/testdata/ftexp/main.go`（位图模式 + contour 模式 + 亚像素 delta）
- 自研光栅现状：`render/internal/raster/`（analytic_filler.go）
- 生产接线：`render/text/glyph_mask_rasterizer.go`（rasterizeOutlineCore）
