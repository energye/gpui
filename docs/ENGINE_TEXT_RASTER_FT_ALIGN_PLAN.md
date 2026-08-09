# 文本渲染 FT 全对齐计划（Raster-FT-ALIGN）

**状态**: **计划中**（2026-08-09 立）
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

### 阶段 B：8–72px 全字集全字体矩阵

**B1 轮廓级矩阵**（快，先铺）
- 字号：8–72 **每 1px 一档**（65 档）。
- 字集/字体（复用现有 testdata + 系统字体）：
  - CJK：cjk3000.txt × Noto Sans CJK TTC / wqy-microhei-nohint.ttf / wqy-microhei.ttf(bytecode)
  - 韩：kr_all.txt × Noto Sans KR；泰：th_all.txt × NotoSansThai
  - Latin：latin_all.txt × FreeSans / DejaVuSans
  - Indic：deva/beng/taml × Samyak/Mukti/Samyak-Tamil
  - 脚本：arab/ethi/mymr/gujr/fallback × KacstOne/AbyssinicaSIL/Padauk/Samyak-Gujarati
  - VF：SourceSans3VF × 4 wght；Noto Sans SC VF × 4 wght
- 对照维度升级：**坐标 + 点数 + on/off 标记 + contours 分组**（补 1.3 盲区）。
- 允许差清单：唯一已知 = gujr px12 U+0A91 +0.2px（文档记录）。

**B2 位图级矩阵**（逐字节）
- 字号：8/10/12/14/16/18/20/24/28/32/40/48/56/64/72 阶梯（15 档）× 全字集。
- 逐字节 bad=0（新光栅器）。

**B 验收**: B1/B2 全矩阵 bad=0（含 on/off、分组维度）；允许差仅 gujr px12 U+0A91。

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
