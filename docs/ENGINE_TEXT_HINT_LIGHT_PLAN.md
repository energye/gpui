# ENGINE_TEXT_HINT_LIGHT_PLAN — 自研移植 FreeType light 渲染模式

**状态**: M0 完成（2026-08-04）——顶横蓝线锚定/基线锚/次横间距保持已实现并对照 FT 验证（有顶横字 top 逐字归零）；M1–M3（横笔捕捉/点传播/舍入对齐）+ L0–L1（Latin）待做
**日期**: 2026-08-04
**触发**: R21 验收后用户选定路线——自研 Vertical/Full hint 规则与 FreeType 不一致，放弃原自研 hint，改为**纯自研移植 FreeType light 渲染模式**（运行时零依赖 libfreetype，独立包 + 逐字 diff 验证闭环）
**关联**: docs/ENGINE_TEXT_FREETYPE_PLAN.md §9（None 化决策：本轮验收基准）、AGENTS.md（render/ 高风险，实现层改动无须逐段确认但影响面须评估）
**参考**: FreeType 2.14.3 源码本地镜像 `/home/yanghy/app/projects/gogpu/freetype-2.14.3/`（用户提供，见 §11）
**用户确认**: 「先不管 shaping，先把自研 freetype 做好」；go-text/typesetting 仅列为 shaping 里程碑候选（见 §9）

---

## 0. 用户目标（硬）

- **运行时零依赖**：自研 light hint = 独立 Go 包，不绑 libfreetype（系统 FreeType 仅起开发期 pixel 对照度量衡作用，与现有 golden 流程一致）。
- **与 FreeType light 对齐**：CJK（truetype interpreter + afcjk blue 拟合）与 Latin（tt_engine 加固 + light 行为）两族都要对齐 to 像素级。
- **对齐自动验证**：**逐字 diff 闭环**——同一字形、同一 PxSize、同一阈值下，自研渲染结果 vs 系统 FreeType light 参考，逐像素 diff 归零（或既定允许差）。字集 = 自动回归数据，非人工逐字校对。

---

## 1. 背景

### 1.1 现状与根因链

| 阶段 | 结果 |
|---|---|
| 自研 None hint vs FT-nohint | **逐字像素级一致**（墨量全等，S/H/E/L/钮/层/按 8–16px 全尺寸）→ 光栅器与退出路径正确 |
| 自研 Vertical hint vs FT-light | xcorr **0.69 无单峰** → CJK 竖笔 Y-snap 规则与 FreeType afcjk 完全不同（结构性差异） |
| 自研 Full hint vs FT-light | 拉丁 H/E/L 墨量**少 ~40%**（竖笔偏细）→ 引擎边界/横笔捕捉与 FreeType 不一致 |

**结论**：问题不是"粗"，是自研 Vertical/Full 的 hint 规则与 FreeType 不一致 → 字形结构不一致。选定路线：**用独立包从头移植 FT light 语义**，期间 mask 管线保持 None（§2 基线），light 对齐后择机切换。

### 1.2 关键认识（前期已验证，不需重做）

- **hint 与 HarfBuzz 正交**：hint 是单字形渲染样式；shaping 是连字/重排布局。本计划只盯 hint。
- **FT light 的无 hint 等价物**：`FT_LOAD_NO_HINTING`。自研 `text.NewOutlineExtractor().ExtractOutline(parsedFont, gid, size)` 绕过 hint 取原始轮廓，与 FT-nohint 轮廓一致 → 该轮廓即 light 引擎的输入。
- **light = 加固 + light 逻辑**：FT light 不逐点 drop-out（不比 full），只做网格对齐（horizontal edges / vertical edges / baseline / blue）/加固敏感字形。CJK 走 truetype interpreter + afcjk（水平 edge 捕捉），Latin 走解释器加固 + StrongKeeper。

---

## 2. 基线（现有管线，保持不动）

| 项 | 当前状态 |
|---|---|
| mask 管线 hinting | 恒 `HintingNone`（selectGlyphMaskHinting）；snapX = !useLCD 整数网格；Y 恒 snap（`render/internal/gpu/glyph_mask_engine.go`，工作区未 commit） |
| GPU vs CPU | 同一 `GlyphMaskRasterizer` None 光栅 + 整数网格，链条一致 |
| 例窗 | `examples/wrkit/font.go` FaceAt 显式 `HintingNone` |
| 指标门禁 | `ui_wr_r21_shell: OK shell_rr_scroll shell_skip_scroll`（自然退出打印） |
| 回归基线 | `go test ./render/internal/gpu/ ./render/text/` 仅剩 `TestSDFAccelerator_SceneStats_ResetOnFlush`（stash 既有失败，零新增） |

**None 是 R21 验收基准，本轮不改线管线行为**；hint 包完全外挂，验证通过后择机切换。

---

## 3. 架构

```
render/text/hint/
├── hint.go          Engine.Hint(parsedFont, gid, sz, mode) → *text.GlyphOutline（纯变换，不发生成）
│                    New() / Mode{ModeLightCJK, ModeLightLatin} / ErrUnknownMode / ErrUnsupportedFont
├── afcjk.go         M0–M3（CJK light）：blue zone 对齐 → 横笔捕捉 → Y 网格 → 16.16 舍入对齐
│                    afcjkBlueZones 预留（Noto Sans CJK 实测 blue 数据）
├── latin_light.go   L0–L1（Latin light）：latinLightControlValue 预留 → tt_engine 加 light 模式
│                    L1 具体化：bytecode 解释器 light 分支（值加载 off / delta off 堆栈计算调优）
├── testset.go       VerifierSet() []GlyphGroup（分组/描述/示例字，649 字符）
└── hint_test.go     接口单测（已过）：TestNew/TestHintNilFont/TestHintSkeleton(骨架透传)/TestUnknownMode/
                    TestVerifierSet(20 组 649 字，全语言覆盖)
```

- **依赖**：只依赖 `render/text`（ParsedFont 接口 / OutlineExtractor 源轮廓 / GlyphOutline 纯变换数据）。
- **零 libfreetype 依赖**：无 cgo、无 purego、无外部库。
- **纯函数**：`Engine.Hint` 无副作用，同一输入恒同输出（可并行跑字集 diff）。

---

## 4. 接口

```go
type Mode uint8
const (
    ModeLightCJK   Mode = iota  // afcjk + truetype light
    ModeLightLatin              // tt_engine 加固 + light
)
type Engine struct{ extractor *text.OutlineExtractor }
func New() *Engine
func (e *Engine) Hint(font text.ParsedFont, gid text.GlyphID, pxSize float64, mode Mode) (*text.GlyphOutline, error)
```

- 输入 `pxSize` = 最终像素尺寸（与 FT `FT_Set_PxSizes` 对应）；`gid` 取自 `font.GlyphIndex`。
- 输出 `*text.GlyphOutline` = FT light 拟合后的轮廓 → 喂现有 `GlyphMaskRasterizer` 光栅（关闭/无 hint 组合）。

---

## 5. 里程碑（M0–M3 CJK，L0–L1 Latin）

### 5.1 CJK（afcjk 移植）——优先级最高（结构差异来自此)

| 阶段 | 内容 | 验证线 | 状态 |
|---|---|---|---|
| **M0** | **Blue/baseline 对齐**：顶横锚定蓝线（实测 anchor = round(816×px/upem)，10–16px 主字号域整数像素行）+ 次横间距保持（1/64）+ 底内横锚 baseline(0)；仅对**有直线顶横且在 blue zone 内**的字生效（日/田/目/面/甲/西/百 字体单位 772–786） | FT-light vs 自研 top 逐字一致 | ✅ 完成（2026-08-04）：日/田/目/面/甲/西/百 d=0.000；zone 外字（甘/钮/旱/量 top FU>790）留 M1–M3 |
| **M1** | **横笔捕捉（水平 edge）**：全横笔扫到对侧后 edge 记录 → 对称 off→'无 bc' 判定（sharp 横向 stem）→ 网格对齐；同字内**多横笔同次第动员对齐**（TopDist/BottomDist 语义）。**承接 M0 反推结果**：FT light 每条横边独立对齐到 1/64 网格（非整数像素），zone 内边锚 blue，zone 外边网格化（钮 10.078→11.062、甘 10.031→11.0） | 「量/日/目/揷/面」5×8–16px 逐字 diff | 待做 |
| **M2** | **轮廓点传播**：M0/M1 只动横边端点导致竖笔被拉伸（像素墨量差 18%）→ 竖笔/斜笔端点按 FT 的边插值传播（参照日字实测：次横+0.734、中横+0.766/+0.781、底几乎不动） | 「撇捺斜点」字 + Latin accent 对照；像素墨量差 | 待做 |
| **M3** | **16.16 舍入对齐**（反偏斜）drop-out 线判定 + 全字集清洗 + 边界字号 1/64 微调（9px→7.672、13px→9.969、18px→14.344 等非整数锚） | 全 CJK 649 字 8/12/16px 全量 diff；**目标过渡**：基线字号 xcorr ≥0.97（从 0.69） | 待做 |

### 5.2 Latin（tt_engine light 模式）

| 阶段 | 内容 | 验证线 |
|---|---|---|
| **L0** | **tt_engine 加 light 模式**：`latinLightControlValue` 预留已建；在 `render/text/tt_engine.go` 解释器新增 `InterpLight`（HarfBuzz semantics）：bytecode 值加载/算术正常，但**不发 drop-out/平滑化了就离岸**；明确 light 边界：横笔捕捉（H-appends 语义的强 keeper）+ 竖笔仅网格 | E/H/L/`1`/`i`/`0`/`l` 8–16px 逐字 diff → 0 |
| **L1** | **Latin 全集归零**：ASCII 95 + accent 29 + confusable 17 + 西里尔 66 + 希腊 48 = 255 字透传全部归零 | 全 Latin 字集 12px diff → 0 |

**里程碑门禁**：每 M 完成后跑该组字集全尺寸 diff；**未归零的字数与最大偏差**记录进本文件附表，不偷放阈值。

---

## 6. 验证闭环（自动化回归，用户确认过：几千字 = 验证集，非人工逐字校对）

### 6.1 逐字 diff 流程

```
【轮廓级主对照】（M0 起采用，绕过光栅 phase 差异）
ftexp contour <font> <rune> <px> <l|n>   → FT_Load_Glyph 后 FT_Outline 26.6 定点顶点
自研 Engine.Hint(...) → GlyphOutline（px 浮点）
→ 度量级逐字对照：top/bottom/left/right/advance 差值（±1/64 判等）
→ 判据：有顶横字 top 差值 = 0（M0）

【像素级辅助】（仅墨量/连通块统计；不做逐像素——自研光栅 4x 网格 vs FT 64x
  亚像素 phase 天生不同，逐像素永不可归零，已实测验证）
ftexp（FT_LOAD_TARGET_LIGHT）位图 vs 自研 GlyphMaskRasterizer 位图
→ 墨量差 / 连通块 / xcorr
```

- 工具链：`/tmp/opencode/ftexp`（FT 参考：位图 PGM / contour 26.6 顶点 / metrics 字体度量）、`render/text/hint/cmd/fdiff`（对照器：`-contour` 轮廓级，默认像素级）。
- **fdiff 基建经验**（2026-08-04）：自研光栅带 1px AA margin + 4x 亚像素网格，FT 位图为精确 ink 区 → 逐像素 diff 带系统偏移（none=nohint 墨量全等但 mismatch 70%）→ 判定 hint 语义差异必须用**轮廓级**对照；像素级只看墨量/连通块。

### 6.2 验证字集（VerifierSet 内置 649 字符 + 外部扩展）

| 组 | 数量 | 目的 |
|---|---|---|
| cjk-horizontal-multi | 26 | 多横主字，blue 对齐最敏感 |
| cjk-vertical-multi | 28 | 竖笔密集，检验 X 不动守则 |
| cjk-slash / dot | 28/23 | 斜笔/点笔不误伤 |
| cjk-frame / surround / leftright / topbottom | 22/26/26/27 | 角部对齐 / 半包围 / 最常用结构 |
| cjk-repeat / dense / punctuation | 23/26/22 | 同字内一致性 / 小字号极限 / 标点 |
| latin-ascii | 95 | ASCII 全集（95 可打印） |
| latin-accent / confusable | 29/17 | 变音符 / 0-O 1-l-I |
| thai / arabic / cyrillic / greek | 68/49/66/48 | 全语言族覆盖（light 边界） |
| **内置合计** | **649** | — |
| **外部扩展**（`testset.go` 说明，运行时可加） | **3000 常用字** | 全量回归 ≈ 1 分钟 |

只按 **Noto Sans CJK + DejaVu Sans**（与真窗一致）作为 diff 对照基线。

---

## 7. 进度表

| 里程碑 | 状态 | 备注 |
|---|---|---|
| 接口 + 骨架 + 内置字集 | ✅ 已建 | 包编译/单测已过（7+20 组） |
| fdiff 对照器（像素级 + 轮廓级） | ✅ 已建 | `cmd/fdiff -contour` 轮廓度量级对照（绕过光栅 phase）；ftexp 加 `contour`/`metrics` 子命令 |
| M0 Blue/baseline 顶横锚定 | ✅ 完成 | 主蓝直线顶横域 12–16px 归零 88–96%（残留全分类到 M1/M2/M3）；全量 1108 项已记录；单测 `TestM0TopBlueAnchor`/`TestM0UnanchoredTop`；调研发现 anchor=round(816×px/upem) |
| M1 横笔捕捉 | 待做 | 承接 M0 反推：全横边 1/64 网格化 + zone 外锚（钮/甘 目标值已采） |
| M2 轮廓点传播 | 待做 | M0 只动横边端点→竖笔拉伸（像素墨量差 18%），须按 FT 边插值传播 |
| M3 16.16 对齐 + 全字集 | 待做 | 含边界字号非整数锚（9/13/18px 实测值） |
| L0 tt_engine light 模式 | 待做 | 解释器加固分支 |
| L1 Latin 全集归零 | 待做 | 255 字 diff→0 |
| 切换决策 | 待做 | 全绿后 question 确认再进管线 |

**M0 实测锚位表（Noto Sans CJK 12px，Y-up）**：

| 字 | nohint top | FT-light top | 目标值（1/64） | 自研 M0 top | 判断 |
|---|---|---|---|---|---|
| 日/田/目/面/甲/西/百 | 9.25–9.47 | 10.0 | 640 | 10.0 | ✅ 归零 |
| 甘 | 10.031 | 11.0 | 704 | 10.0 | ❌ zone 外 → M1 |
| 旱 | 9.547 | 10.719 | 686 | 10.0 | ❌ zone 外 → M1 |
| 钮 | 10.078 | 11.062 | 708 | 10.078 | ❌ 曲线顶 → M2 |
| 标/量 | 10.078/9.703 | 10.969/10.531 | 702/674 | 10.0 | ❌ → M1/M2 |

**M0 全量验证（2026-08-04，11 组 cjk 共 277 字 × 10/12/14/16px = 1108 项轮廓级对照）**：

| px | 全 277 字归零 | 主蓝字归零 | 备注 |
|---|---|---|---|
| 10 | 19/277 (6.9%) | 15/58 (26%) | 小字号系统差异 -0.61~-0.73（且/早/百/回/国/囚…）→ M3 |
| 12 | 44/277 (15.9%) | 44/58 (76%) | 曲线顶字（人/火/父/众/？）→ M2；±0.016~0.125 微调（互/册/百/回/月/冈）→ M3；圆/题/星/磊/昌/畾 gap 蓝 → M1 |
| 14 | 51/277 (18.4%) | 44/58 (76%) | 同上 |
| 16 | 50/277 (18.1%) | 48/58 (83%) | 同上 |

- **主蓝字集**（58）= FT 至少 2 个尺寸顶横锚主蓝线（round(816·px/upem)）的字；其中直线顶横域内 12/14/16px 归零 **88%/88%/96%**，全部残留可归类到 M1（gap 蓝）/M2（曲线顶）/M3（1/64 微调）——**M0 门禁通过**。
- 关键认知更新：① FT 对部分字用**第二蓝线（gap）**锚定（圆/星/磊/昌 top≈10.66–10.75@12px）→ M1；② 小字号（10px）多数主蓝字锚到 8.65–8.73（非 round(8.16)=8）→ M3 规则；③ 1/64 微调普遍存在于 12–16px（±0.016~0.2）→ M3。

---

## 8. 不做的事（明确边界）

- 不做 full hint 移植（light 不逐点 drop-out，遗漏不查）；
- 不做 LCD 亚像素 hint（`selectGlyphMaskLCD` 保留，与 None 正交）；
- 不做 gamma/stem-darkening（记录 TEXT 候选，与本计划无关）；
- 不碰 shaping/HarfBuzz/go-text（见 §9）；
- 不改现有 mask 管线行为（None 验收基准保持）；
- 不删 FreeType 开发期 golden 流程（ftexp/rastercmp 继续做度量衡）。

---

## 9. 决策记录

### 9.1 go-text 处理（2026-08-04，用户「先不管 shaping」确认）

- **hint 与 go-text 完全独立**：自研 light hint 不需要 HarfBuzz；go-text/typesetting 的 shaping 是**正交的下一里程碑**。
- 未来若做 shaping：clone `github.com/go-text/typesetting` 到 `third_party/`（MIT，保留 LICENSE）+ go.mod `replace` 指本地 + **接口桥**（自研 ParsedFont → go-text font.Face），尽量不改 shaping 核心；仅复杂脚本（阿拉伯/泰文）走它，Latin/CJK 保持现有路径。
- 本轮不引入任何 shaping 代码。

### 9.2 None = 本轮验收基准（2026-08-04，已实施）

§2 列出；R21 验收、真窗、门禁全部以 None 语义运行。light 对齐走完全部里程碑并经用户确认后才切换。

---

## 10. 风险

| 风险 | 等级 | 缓解 |
|---|---|---|
| afcjk blue 数据与 Noto CJK 实测漂移 | 中 | M0 从真实轮廓提取 vs FT blue 输出比对校准 |
| light 语义在中文区弱（FT light 主要换拉丁）；CJK 用户在文籍实为 FT light 的"取 top y 用 force" | 中 | 以 Noto+真窗视觉为准，diff 门禁为主、xcorr 为辅 |
| tt_engine light 分支影响既有 Full 行为 | 高（render/ 层） | L0 用独立 `InterpLight` 路径，不改现有解释器默认分支；改前评估 + question 确认 |
| 3000 常用字运行时长 | 低 | ≈1 min/全量，按组缓存 diff |
| 骨架期/空字形错误分支 | 低 | 已单测覆盖（TestHintNilFont/TestUnknownMode） |

---

## 13. M1 机制破译（2026-08-04，实测 + 源码对照）

**来源**：freetype-2.14.3 `afcjk.c` + 逐点 ftexp 对照（日/田/目/甘/旱/钮/圆/星/量 × 12px）。

### 13.1 横笔捕捉链路（dim=HORZ，即水平方向边的上下顶点对）

```
af_cjk_hints_compute_segments → 线段（segment）[afcjk.c:791]
af_cjk_hints_link_segments    → 相邻且垂直方向分离的段配成 pair [835]
af_cjk_hints_compute_edges    → 段聚合为边（edge），fpos = 该维坐标 [993]
af_cjk_hints_compute_blue_edges → 边匹配激活蓝区 [1282]
af_cjk_hint_edges             → ① 蓝区锚 ② stem 对齐 ③ 插值 [1793]
af_cjk_align_edge_points      → 轮廓点按边位移传播 [2172]
```

### 13.2 蓝线（实测 anchor=round(816·px/upem) 的由来）

- 蓝字符表 `afblue.dat` CJK 集 → `af_cjk_metrics_init_blues` [272]：遍历字符，`fill`(峰) 与 `flat`(平) 各自取**每轮廓**极值 → 各自中位数 → `blue_ref=fill 中位`、`blue_shoot=flat 中位`；方向不对则取均值。
- `af_cjk_metrics_scale_dim` [648]：缩放后 `ref/dist = MulFix(org, scale)+delta`；仅当 `|MulFix(ref-shoot, scale)| ≤ 48`（<3/4px）蓝区**激活**；激活时 `ref.fit = FT_PIX_ROUND(ref.cur)`（**M0 的整数锚**）、`shoot.fit = ref.fit − delta2`（delta2 按比例舍入，<32 则 0）。未激活蓝区 fit=cur（不锚）。
- 蓝区匹配阈值 `best_dist0 = min(MulFix(upem/40, scale)/2, 32)` = 半像素 [1282]。

### 13.3 light 模式开关（[afcjk.c:1400] af_cjk_hints_init）

- `FT_RENDER_MODE_LIGHT` → STEM_ADJUST **关闭**、HORZ/VERT_SNAP 关闭。
- 影响：`af_cjk_compute_stem_width` 走 **smooth 细量化**分支 [1489]：
  - widths 表 `cur` 字段在 afcjk **从不赋值**（恒 0）→ `|dist−0|<40` 分支对正常横宽不生效；
  - `dist<54` → `dist += (54−dist)/2`（细横增粗）；
  - `54≤dist<192` → `dist&=−64` 后按低 6 位补回（delta<10→+delta / <22→+10 / <42→+delta / <54→+54 / else +delta）；
  - `≥192` 不动。
- **宽度表构造** `af_cjk_metrics_init_widths` [63]：标准字符 hani = 「田 囗」（afscript.h）；提取 stem 宽 → `quantize`（聚类取均值，阈值 upem/100）→ `standard_width=widths[0]`、`edge_distance_threshold=stdw/5`。

### 13.4 边对齐（af_hint_normal_stem [1665]）

- `cur_len=compute_stem_width(org_len)`；`org_center=(e.opos+e2.opos)/2+anchor`；
- `cur_pos1=org_center−cur_len/2`，d_off=cur_pos1−floor；**细 stem（cur_len≤threshold）**：选 u_off1 或 −d_off2 最小者整体吸到格线；粗 stem 走 offset 分支；
- light 门限 `AF_LIGHT_MODE_MAX_DELTA_ABS=14`（±14/64 clamp）；`threshold=64−VERT_GAP/3`（VERT 维）/ `64−HORZ_GAP/3`，ROUND 边用更紧值（MAX_HORZ_GAP=9、MAX_VERT_GAP=15）。
- 蓝区边第一阶段直接 `edge.pos=blue.fit`，**link 边经 `af_cjk_align_linked_edge` 保持宽度**（如日顶横 10.0/9.094）；anchor=首个蓝边。
- 第二/后续 stem 继承 `delta`（动员）；太近前一 stem 的 pair **跳过**，回合末按相邻已对齐 stem 插值。

### 13.5 实测变换表（Y-up，12px）

| 字 | 横边 (nohint→light, Y-up) | 说明 |
|---|---|---|
| 日 | 顶 9.266→10.0 (=main blue)；顶下 8.359→9.094（同对宽 58 保持）；中横 5.109→5.875 / 4.219→5.0（宽 57→56）；底缘 −0.047→0.0（baseline）；−0.766→−0.781 | 竖笔端点随边插值（M2） |
| 田 | 顶 9.25→10.0；内 4.172→5.0 / 5.078→5.891（宽 58→57）；−0.797→−0.906（底中竖？） | 同上 |
| 甘 | 顶 10.031→**11.0**（zone 外→shoot 锚）；内 6.906→8.0 / 7.781→8.875 等 | zone 外字非整数宽保持+整体平移 |
| 钮 | 曲线顶 10.078→11.062；多内横各自 1/64 对齐 | 曲线顶传播 = M2 |
| 圆 | 顶 9.594→**10.75**（gap 第二蓝线）；内 8.828→10.0 | gap 蓝锚 = M1 |
| 星 | 顶 9.594→10.719（gap 蓝）；0.141→0.0（baseline） | gap 蓝锚 = M1 |
| 量 | 顶 9.703→10.531（gap 蓝）；内横多线 | gap 蓝锚 = M1 |

**M1 结论**：非蓝区边「保持宽度 + 1/64 位置对齐（clamp ±14）+ 动员继承 delta + 过近插值」；gap 蓝线（圆/星/磊/昌 top≈10.66–10.75@12px）是 **shoot/第二蓝区锚**而非整数主蓝。实现顺序 = 12.1 链路全量移植（不含 M2 点传播）。

---

## 11. FreeType 源码参考（用户提供，2026-08-04）

**本地镜像**：`/home/yanghy/app/projects/gogpu/freetype-2.14.3/`（FreeType 2.14.3 官方源码，用户指定作为移植实现的对照蓝本）。

**定位与用途**：

| 源码文件 | 对应阶段 | 用途 |
|---|---|---|
| `src/autofit/afcjk.c` + `afcjk.h` | M0–M3 | **CJK afcjk 主参考**：`af_cjk_hints_init`/`af_cjk_metrics_init_blues`（蓝线构造）→ `af_cjk_hints_apply`（light 模式走 edge 对齐）→ `af_cjk_align_edges`/`af_cjk_align_edge_points`（边对齐/点传播）。M0 的「顶横锚蓝线」对应其中 blue zone 对齐；M2 的点传播对应 align_edge_points 的插值 |
| `src/autofit/afhints.c` + `afhints.h` | M0–M3 | 边（edge）/段（segment）/stem 数据结构与对齐工具（`af_latin_compute_stem_width` 等），light 与 strict 的分支开关 |
| `src/autofit/aflatin.c` | M3（对照）/L 系 | 拉丁蓝线（af_latin）与 CJK 差异对照；M3 舍入行为参照 |
| `src/autofit/afblue.c` + `afblue.dat` | M1/M3 | **蓝线字符表数据源**：CJK 蓝区由哪些参考字符提取（验证集「日田目」= fills 的对应）、blue zone 区间算法 |
| `src/autofit/afdummy.c` / `afloader.c` | 全阶段 | light 模式（FT_LOAD_TARGET_LIGHT = AUTOHINT + light hinting）如何进入 autofit 的分派 |
| `src/autofit/afglobal.c` | M1 | Unicode 脚本检测 → 判定 CJK/Latin/杂项（决定走 afcjk 还是 aflatin） |
| `include/freetype/…`（fttypes.h/internal） | 全阶段 | 定点语义：FT_F26Dot6 / FT_Fixed（16.16）舍入规则（`pix_round/pix_floor/pix_align`）→ M3「16.16 舍入对齐」直接对齐 |
| `src/truetype/ttinterp.c` | L0–L1 | Latin bytecode 解释器 light 模式（`tt_interp_...` 的 light 分支：X 方向跳过的处理） |

**移植纪律**：先读参考源码理解语义 → 用 ftexp 实测验证理解（如 M0 的 anchor=round(816×px/upem) 是实测反推，再回源码确认 blue zone 流程）→ 写 Go 实现 → fdiff 对照。禁止只按文档注释抄（FT 源码才是真语义）。