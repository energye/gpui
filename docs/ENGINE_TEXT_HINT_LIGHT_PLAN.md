# ENGINE_TEXT_HINT_LIGHT_PLAN — 自研移植 FreeType light 渲染模式

**状态**: M0 完成（2026-08-04）；**M1 机制破译完成（cf2 实锤）**——CJK CFF 字体的 light 模式走 **cf2（psaux）引擎**而非 afcjk/pshinter（afcjk 仅 TrueType glyf 的 CJK autohinter）；cf2 hintmap 算法已从源码+TRACE 完全解码并数值级复现（见 §13）；**M1 charstring 解释器完成（cffcs.go，2026-08-04 验证）**——日/田/目 12px 共 60 点 26.6 与 FT-nohint 逐点零误差。**M2 cf2Blues+cf2HintMap 完成（2026-08-05）**——含 X 轴 vstem；对照线 = ftexp `FT_LOAD_NO_STEM_DARKENING`（实证 FT 2.14.3 默认不暗化，darken=0）；单区字 25 个 × 10/12/14/16/18/20/22/24px 共 200 项逐点全绿。M3（hintmask 多区，明 1 字）+ L0–L1（Latin）待做
**日期**: 2026-08-04（M1 认知更新同日）
**触发**: R21 验收后用户选定路线——自研 Vertical/Full hint 规则与 FreeType 不一致，放弃原自研 hint，改为**纯自研移植 FreeType light 渲染模式**（运行时零依赖 libfreetype，独立包 + 逐字 diff 验证闭环）
**关联**: docs/ENGINE_TEXT_FREETYPE_PLAN.md §9（None 化决策：本轮验收基准）、AGENTS.md（render/ 高风险，实现层改动无须逐段确认但影响面须评估）
**参考**: FreeType 2.14.3 源码本地镜像 `/home/yanghy/app/projects/gogpu/freetype-2.14.3/`（用户提供，见 §11）
**用户确认**: 「先不管 shaping，先把自研 freetype 做好」；go-text/typesetting 仅列为 shaping 里程碑候选（见 §9）

---

## 0. 用户目标（硬）

- **运行时零依赖**：自研 light hint = 独立 Go 包，不绑 libfreetype（系统 FreeType 仅起开发期 pixel 对照度量衡作用，与现有 golden 流程一致）。
- **与 FreeType light 对齐**：**全世界的语言 + 现代字体库**（用户 2026-08-04 确认扩展）——CJK/CFF 一族与 Latin/glyf 一族都要对齐 to 像素级。FT light 实际分派：CFF/CFF2 → cf2（语言无关）；glyf+字节码 → TT 解释器 light；glyf 无字节码 → autofit light（latin/cjk/indic 三模块 + 其余脚本透传不 hint）。语言差异主要影响验证集，不改变引擎结构。
- **对齐自动验证**：**逐字 diff 闭环**——同一字形、同一 PxSize、同一阈值下，自研渲染结果 vs 系统 FreeType light 参考，逐像素 diff 归零（或既定允许差）。字集 = 自动回归数据，非人工逐字校对。
- **现代字体库覆盖**（本轮不做的列入 §8 边界）：可变字体（gvar ✓ 已有 / CFF2+blend 顺带）、TTC/OTC ✓、WOFF2 视解析层、彩色字体 ✓（非 hint 范畴）。

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
- **light = 网格对齐 + 蓝区锚（无 drop-out）**：FT light 不逐点 drop-out（不比 full）。**引擎按字体形态分派**：CFF/CFF2 → cf2（psaux，语言无关，本轮主线）；glyf+字节码 → TT 解释器 light；glyf 无字节码 → autofit light（afcjk/aflatin/afindic + 其余脚本透传）。

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
├── cff.go           CFF 表解析（已完成）：CFF INDEX / Top·Private DICT / FDSelect / FDArray /
│                    蓝区值（BlueValues/OtherBlues/StdHW/StdVW/blueScale·Shift·Fuzz）/ TTC 绝对偏移
│                    （M1 补 LanguageGroup 12 17 / Subrs 19）
├── cffcs.go         M1：Type 2 charstring 解释器（hstem/vstem/hstemhm/vstemhm/hintmask/cntrmask
│                    /callsubr/callgsubr/subrs/width 规则）→ 精确 cs hint 与 cs 轮廓
├── psh_light.go     M2：cf2 移植——cf2Blues（emBox 幽灵区 ICF±120/880 + 真实区 boost/capture）
│                    + cf2HintMap（初始图/插入/adjustHints 双向 1px 拟合/map 插值）→ 全轮廓映射
├── autohint_*.go    自研 autofit（Full 语义，现有）——M4 改 light 语义，不删
├── tt_engine.go     自研 TT 字节码解释器（现有）——L0 加 InterpLight 分支
├── testset.go       VerifierSet() []GlyphGroup（分组/描述/示例字，649 字符，M3 扩韩文/泰文 CFF）
└── hint_test.go     接口单测（已过）
```

- **依赖**：只依赖 `render/text`（ParsedFont 接口 / OutlineExtractor 源轮廓 / GlyphOutline 纯变换数据）。
- **零 libfreetype 依赖**：无 cgo、无 purego、无外部库。
- **纯函数**：`Engine.Hint` 无副作用，同一输入恒同输出（可并行跑字集 diff）。
- **引擎分派（按字体形态）**：CFF/CFF2 → cf2（psh_light.go）；glyf 有字节码 → tt_engine；glyf 无字节码 → autofit light。

---

## 4. 接口

```go
type Mode uint8
const (
    ModeLightCJK   Mode = iota  // cf2（CFF）主路径；glyf 无字节码 → autofit light
    ModeLightLatin              // tt_engine 加固 + light
)
type Engine struct{ extractor *text.OutlineExtractor }
func New() *Engine
func (e *Engine) Hint(font text.ParsedFont, gid text.GlyphID, pxSize float64, mode Mode) (*text.GlyphOutline, error)
```

- 输入 `pxSize` = 最终像素尺寸（与 FT `FT_Set_PxSizes` 对应）；`gid` 取自 `font.GlyphIndex`。
- 输出 `*text.GlyphOutline` = FT light 拟合后的轮廓 → 喂现有 `GlyphMaskRasterizer` 光栅（关闭/无 hint 组合）。

---

## 5. 里程碑

### 5.1 CJK/CFF（cf2 移植）——本轮主线（结构差异来自此）

| 阶段 | 移植内容 | 验证线 | 状态 |
|---|---|---|---|
| **M0** | **顶横蓝线锚定**（816 经验式，M0 全量基线 1108 项已记录） | 主蓝字 12–16px 88–96% 归零 | ✅ 完成（2026-08-04） |
| **M1** | **Type 2 charstring 解释器**（`cffcs.go`）：数值(1–5字节)/转义/路径操作符/hstem·vstem·hstemhm·vstemhm/hintmask·cntrmask/callsubr·callgsubr/subrs 递归/width 规则；**cff.go 补 LanguageGroup(12 17)/Subrs(19)/defaultWidthX(20)/nominalWidthX(21)**。产出：精确 cs hstem 对 + cs 轮廓（消除 26.6 反推的 ±1.3FU 误差） | 日/田/目 12px：解释器 cs 轮廓 = FT-nohint 26.6 反推 ±0 误差；hstem 对 = TRACE 数值一致 | ✅ 完成（2026-08-04，cffcs_test.go） |
| **M2** | **cf2Blues + cf2HintMap 完整移植**（`psh_light.go` 重写）：<br>① cf2Blues：语言组判定 → emBox 幽灵区启发式（ICF −120/880，MIN_COUNTER 0.5px，dummy 蓝区即启用）或真实区（zone/boost/suppressOvershoot/blueScale 修正/family blues）→ capture 三态（flat/overshoot/round）<br>② cf2HintMap：初始图（幽灵区优先+合成0）→ 逐区图（插入 midpoint+halfWidth/重叠丢弃/ghost 单边）→ adjustHints（1px 双向拟合+0.5px 防重叠+二遍上移）→ map 分段线性插值<br>③ **X 轴 vstem 同构移植**（用户确认 M2 含 X：日字 X 577 vs 578 的 1/64 对齐）<br>④ 全轮廓点映射（cs→ds，Y/X 独立） | 单区字（无 hintmask）全字集 10/12/14/16px 轮廓级 diff 归零（主蓝字 100%，全部字 ≥90%）；**对照线 = §13 已解码数值**（日 12px：-120→-1.5px、772→10.0、352→5.0 等 8 边） | ✅ 完成（2026-08-05） |
| **M3** | **hintmask 多区 + 收尾**：多 mask 分区逐区建图映射；16.16 舍入对齐；边界字号（9/13/18px）；验证集扩韩文/泰文 CFF（Noto Sans KR/TH 的 TTC）；3000 常用字全量回归 | 649 字（+韩/泰 CFF 扩集）全尺寸 fdiff；目标：基线字号 xcorr ≥0.97（从 0.69） | 待做 |
| **M5** | **CFF2 可变字体**：blend/vsindex 支持（cf2 语义顺带），可变 CFF 字体实例化 | 可选对照（Noto Sans CJK VF 默认实例） | 待做（本轮后） |

### 5.2 其他引擎（M4/L0–L1，本轮后）

| 阶段 | 内容 | 验证线 |
|---|---|---|
| **M4** | **glyf 无字节码 → autofit light 化**：现有自研 autohint_*.go（Full 语义）改 light 模式——afcjk light（旧 §13 破译有效）/ aflatin light / afindic；其余脚本（阿拉伯/希伯来/泰文等）light 下**透传不 hint** | 无字节码 TTF（Noto Sans TTF 版/DejaVu 无字节码样本）× 各脚本族 diff |
| **L0** | **tt_engine light 模式**：`InterpLight`（HarfBuzz semantics：网格对齐减弱、不发 drop-out、强 keeper）；不改现有解释器默认分支（render/ 高风险，改前 question） | E/H/L/`1`/`i`/`0`/`l` 8–16px 逐字 diff → 0 |
| **L1** | **Latin 全集归零**：ASCII 95 + accent 29 + confusable 17 + 西里尔 66 + 希腊 48 = 255 字 | 全 Latin 字集 12px diff → 0 |

### 5.3 skia/flutter 能力对齐（工业级缺口，2026-08-04 用户「工业级控件/对齐底层库能力」确认）

| 阶段 | 内容 | 验证线 |
|---|---|---|
| **M6** | **CJK 竖排**：vhea/vmtx 解析 + 竖排度量（vertical advance/bearing）+ vert/vrt2 特性（gpui 目前无任何 vhea/vmtx 支持） | 中文/日文竖排样本行排对照；CFF 竖排字形走 cf2 语义 |
| **M7** | **系统字体发现（FontMgr 等价）**：fontconfig/系统字体目录枚举 + 家族匹配 + MultiFace 自动回退链 | 系统字体列表与回退选择对照（fc-match 等价） |
| **S1** | **shaping 复杂脚本 HarfBuzz 语义 audit**：OwnShaper 已有 GSUB/GPOS 全类型 + arabic_joining + indic_reorder；对照 HarfBuzz 验证覆盖率（泰文重排/孟加拉 matra/emoji ZWJ 序列/默认 feature 集） | 各复杂脚本样本 shaping 结果对照 HarfBuzz（go-text 可作参考实现） |

**里程碑门禁**：每 M 完成后跑该组字集全尺寸 diff；**未归零的字数与最大偏差**记录进本文件附表，不偷放阈值。

**功能范围矩阵（本轮需移植/已移植功能清单，2026-08-04）**：

| # | 功能模块 | 引擎 | 内容 | 状态 |
|---|---|---|---|---|
| 1 | CFF 表解析 | cf2 | INDEX/DICT/FDSelect/FDArray/蓝区值/TTC 绝对偏移 | ✅ 完成 |
| 2 | LanguageGroup/Subrs | cf2 | Private DICT op 12 17 / 19 | ✅ 完成（M1，cff.go） |
| 3 | Type 2 charstring | cf2 | 解释器 + hint 收集 + subrs + width | ✅ 完成（M1，cffcs.go；M2 修 callgsubr/255 号语义 + INDEX 偏移） |
| 4 | emBox 幽灵区 | cf2 | ICF ±120/880 + MIN_COUNTER 0.5px + dummy 判定 | ✅ 完成（M2，psh_light.go） |
| 5 | 真实蓝区/capture | cf2 | boost/suppressOvershoot/blueScale/family/三态 capture | ✅ 完成（M2，psh_light.go） |
| 6 | hintmap 核心 | cf2 | 插入/adjustHints/map/二遍上移 | ✅ 完成（M2，psh_light.go） |
| 7 | X 轴 vstem | cf2 | 与 Y 同构 | ✅ 完成（M2，psh_light.go） |
| 8 | hintmask 多区 | cf2 | 多 mask 分区 | ❌ M3 |
| 9 | seac/非 1000-upem | cf2 | 顺带支持不验证（用户确认） | ❌ 随 M2 |
| 10 | CFF2 blend | cf2 | 可变字体 | ❌ M5 |
| 11 | autofit light | autofit | 自研 autohint 改 light（cjk/latin/indic） | ❌ M4 |
| 12 | TT 解释器 light | tt_engine | InterpLight | ❌ L0 |

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
| **M3 扩集**（2026-08-04 用户「全世界语言」确认） | — | **韩文谚文 / 泰文 CFF 版**（Noto Sans KR/TH TTC）：验证 cf2 语言无关性；后续 M4 扩无字节码 glyf 各脚本族（天城文/希伯来/孟加拉等） |
| **内置合计** | **649** | — |
| **外部扩展**（`testset.go` 说明，运行时可加） | **3000 常用字** | 全量回归 ≈ 1 分钟 |

只按 **Noto Sans CJK + DejaVu Sans**（与真窗一致）作为 diff 对照基线。

---

## 7. 进度表

| 里程碑 | 状态 | 备注 |
|---|---|---|
| 接口 + 骨架 + 内置字集 | ✅ 已建 | 包编译/单测已过（7+20 组） |
| fdiff 对照器（像素级 + 轮廓级） | ✅ 已建 | `cmd/fdiff -contour` 轮廓度量级对照（绕过光栅 phase）；ftexp 加 `contour`/`metrics` 子命令 |
| M0 Blue/baseline 顶横锚定 | ✅ 完成 | 主蓝直线顶横域 12–16px 归零 88–96%；全量 1108 项已记录；anchor=round(816×px/upem) 为 M0 经验式 |
| M1 机制破译（cf2 实锤） | ✅ 完成 | CFF light = cf2(psaux)，非 afcjk/pshinter；TRACE 单位解码（ds=printed×scale/65536）；adjustHints 全部数值复现（见 §13） |
| M1 charstring 解释器 | ✅ 完成（2026-08-04） | cffcs.go + LanguageGroup/Subrs/width；日/田/目 12px 60 点逐点零误差；26.6 转换链见 §13.6 |
| M2 cf2Blues+cf2HintMap（含 X 轴） | ✅ 完成（2026-08-05） | psh_light.go 重写完成；单区字 25 个 × 6 字号（10–24px）逐点 vs ftexp light（暗化实证关）全绿；暗化移植保留备用（psf_light.go 按 psfont.c 移植，当前 darken=0） |
| M3 hintmask 多区 + 收尾 + 验证集扩 | 待做 | 韩/泰 CFF 扩集；3000 常用字全量回归 |
| M4 autofit light 化（无字节码 glyf） | 待做 | 本轮后 |
| M5 CFF2 可变（blend） | 待做 | 本轮后；注：`extractCFF2Outline` 已有 fvar/avar/blend（cff_outline.go），M5 仅为 cf2 语义融合 |
| M6 CJK 竖排（vhea/vmtx/vrt2） | 待做 | 本轮后；gpui 现无竖排支持 |
| M7 系统字体发现（FontMgr 等价） | 待做 | 本轮后 |
| S1 shaping 复杂脚本 HarfBuzz audit | 待做 | 本轮后；OwnShaper 表驱动核心已就位 |
| L0 tt_engine light 模式 | 待做 | 解释器加固分支，改前 question（render/ 高风险） |
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
- 关键认知更新（cf2 视角）：① 「gap 蓝锚」（圆/星/磊/昌 top≈10.66–10.75@12px）= 这些字所在 **fd 有真实蓝区**（非 dummy），走 cf2 真实区 capture/幽灵区混合路径 → M2 移植后自然对齐；② 小字号（10px）多数主蓝字锚到 8.65–8.73（非 round(8.16)=8）→ cf2 boost/round 语义；③ 1/64 微调普遍存在于 12–16px（±0.016~0.2）→ adjustHints 语义。**M0 的 816 经验式为幽灵区+拟合的 12px 净效果（§13.5），M2 完成后不再需要。**

---

## 8. 不做的事（明确边界）

- 不做 full hint 移植（light 不逐点 drop-out，遗漏不查）；
- 不做 LCD 亚像素 hint（`selectGlyphMaskLCD` 保留，与 None 正交）；
- 不做 gamma/stem-darkening（记录 TEXT 候选，与本计划无关）；
- 不碰 shaping/HarfBuzz/go-text（见 §9）；
- 不改现有 mask 管线行为（None 验收基准保持）；
- 不删 FreeType 开发期 golden 流程（ftexp/rastercmp 继续做度量衡）；
- **M5 前不做 CFF2 blend/可变实例化**（已有 `extractCFF2Outline` fvar/avar/blend 兜底，cf2 移植不依赖；M5 仅为语义融合）；
- **本轮不做 WOFF2 解压层**（视 gpui 字体解析层现状，未发现 woff2 支持；列入后续评估）；
- **本轮不做彩色字体 hint**（COLR/CBDT/sbix 已有渲染，与 hint 正交）；
- **seac 复合字 / 非 1000-upem CFF**：顺带支持不验证（§9.3）。

---

## 9. 决策记录

### 9.1 go-text 处理（2026-08-04，用户「先不管 shaping」确认）

- **hint 与 go-text 完全独立**：自研 light hint 不需要 HarfBuzz；go-text/typesetting 的 shaping 是**正交的下一里程碑**。
- 未来若做 shaping：clone `github.com/go-text/typesetting` 到 `third_party/`（MIT，保留 LICENSE）+ go.mod `replace` 指本地 + **接口桥**（自研 ParsedFont → go-text font.Face），尽量不改 shaping 核心；仅复杂脚本（阿拉伯/泰文）走它，Latin/CJK 保持现有路径。
- 本轮不引入任何 shaping 代码。

### 9.2 None = 本轮验收基准（2026-08-04，已实施）

§2 列出；R21 验收、真窗、门禁全部以 None 语义运行。light 对齐走完全部里程碑并经用户确认后才切换。

### 9.3 范围扩展：全世界语言 + 现代字体库（2026-08-04，用户确认）

- **目标范围**：不只 CJK+Latin，覆盖全世界语言与现代字体库。FT light 的分派天然语言无关（cf2 / TT 解释器 / autofit 三模块 + dummy 透传），语言差异主要影响验证集。
- **本轮主线**（用户确认）：cf2 移植（M1–M3），验证集 M3 扩韩文/泰文 CFF（Noto Sans KR/TH TTC）。
- **本轮后**：M4 autofit light 化（无字节码 glyf）、M5 CFF2 可变（blend）、L0–L1 tt_engine light。
- **顺带支持不验证**（用户确认）：seac 复合字、非 1000-upem 的 CFF 字体。
- **M2 含 X 轴 vstem**（用户确认）：日字 X 的 1/64 对齐（577 vs 578）需要像素级归零。

---

## 10. 风险

| 风险 | 等级 | 缓解 |
|---|---|---|
| cf2 语义边界（hintmask 多区/暗化开关/非 1000-upem）与 Noto CJK 实测漂移 | 中 | M2 从 TRACE 数值级对照校准（§13 解码线）；hintmask 区 M3 单独验证 |
| autofit light 化（M4）影响既有自研 autohint Full 语义 | 高（render/ 层） | 独立 light 分支不改 Full 默认路径；改前评估 + question 确认 |
| tt_engine light 分支影响既有 Full 行为 | 高（render/ 层） | L0 用独立 `InterpLight` 路径，不改现有解释器默认分支；改前评估 + question 确认 |
| 3000 常用字运行时长 | 低 | ≈1 min/全量，按组缓存 diff |
| CFF2/go-text 默认实例与 FT cf2 的实例化差异 | 中 | M5 阶段单独对照，不混入 M1–M3 |
| 骨架期/空字形错误分支 | 低 | 已单测覆盖（TestHintNilFont/TestUnknownMode） |

---

## 13. M1 机制破译（cf2 实锤，2026-08-04，TRACE + 源码对照）

**来源**：freetype-2.14.3 `src/psaux/pshints.c`（cf2 hint map）+ `psblues.c`（蓝区）+ `psfixed.h`（定点宏）+ `FT_DEBUG_LEVEL_TRACE` 日志 `/tmp/opencode/ft12.log`（修复 ftcheck 后 12px 正确数据）。

### 13.1 引擎身份

- CFF 字体的 light 模式走 **cf2（psaux）**，非 pshinter/afcjk。TRACE 打印（"Got hint at"/"Inserting hint"/"Initial hintmap"/"Hints adjusted"）全部来自 `pshints.c`（行 649/778/1039/1059）。
- afcjk 仅用于 **TrueType glyf 无字节码**字体的 CJK autohint（M4 阶段）。

### 13.2 TRACE 单位解码（关键）

- `pshints.c:316`：`dsCoord/(scale*1.0)` → 打印值 = ds/scale，**真实 ds = 打印值×scale/65536（px）**。
- 验证：772 行打印 720.18×scale 910 → 10.000px 精确；-120 行 -116.06×847 → -1.500px。
- 幽灵区数值：bottom ghost ds = `round(cs×scale) − MIN_COUNTER(0.5px)`（-1.44→-1.0−0.5=-1.5）；top ghost = `round(cs×scale) + 0.5`（10.56→11.0+0.5=11.5）。

### 13.3 cf2_blues（psblues.c）

- 常量：`CF2_ICF_Bottom=-120 / CF2_ICF_Top=880`（psblues.h:105）；`CF2_MIN_COUNTER=0.5px`（psblues.h:114）；`cf2_fixedRound` = 就近 1px（psfixed.h:64）。
- **emBox 幽灵区启发式**（psblues.c:133-178）：LanguageGroup(Private DICT op 12 17)==1 且 BlueValues 为 dummy（0 项，或 4 项且 [0][1] < −120、[2][3] > 880）→ 幽灵区启用，**忽略真实蓝区**。Noto CJK 全部 subfont 的 [-250,-250,1100,1100] 均触发。
  - bottom edge：cs = −120−ε，ds = round(−120×scale)−0.5，GhostBottom|Locked|Synthetic
  - top edge：cs = 880+ε，ds = round(880×scale)+0.5，GhostTop|Locked|Synthetic
- **真实区路径**：BlueValues 首对=bottom 区（flat edge=顶值）、余=top 区（flat=底值）；OtherBlues 全 bottom；maxZoneHeight 钳制 blueScale；scale<blueScale → suppressOvershoot + boost = 0.6−0.6×scale/blueScale（≤0.5px）；dsFlatEdge = round(csFlat×scale ∓ boost)。
- **capture 三态**（psblues.c:452-568）：区+fuzz 内含边 → suppressOvershoot：ds=dsFlatEdge；否则 overshoot 距离≥blueShift：min/max(round(ds), dsFlatEdge∓1px)；否则 round(ds)。被捕获边全部 lock 并整体位移。

### 13.4 cf2_hintmap（pshints.c）

- **初始图**：幽灵区优先插入（:878-893）→ 被捕获/锁定 stem 插入（:897-941）→ 合成 0 点（若 edges 全在 0 一侧，:965-991）→ adjustHints。
- **逐区图**：幽灵区 + 全部激活 stem（mask 位），**未锁定 pair 经初始图定位**：midpoint=map_init(cs 中点)，halfWidth=(宽/2)×uniform_scale，两边缘 = midpoint∓halfWidth（:685-712）。
- **插入重叠丢弃**（:657-755）：cs 相同 / pair 跨既有边 / 插在 pair 中间 / ds 重叠 → 丢弃。
- **adjustHints**（:395-601）：未锁定 pair 计算四向移动（downMoveDown/upMoveDown/downMoveUp/upMoveUp）→ moveUp=min、moveDown=max → 有空间（邻边 ≥ds+move+0.5px）则取绝对值小者，否则单向/不动并 saveEdge → 二遍上移（:573-600）；**scale 重算**：处理边 i 时 scale[i-1]=DivFix(ds[i]−ds[i-1], cs[i]−cs[i-1])。
- **map(cs)**（:330-378）：cs<首边 → 均匀 scale 从首边外推；否则按边线性插值（用所在段下沿边的 scale）。

### 13.5 日 12px 全数值复现（对照线，fd12，emBox 幽灵区）

| hint 边 (cs) | 初始 ds | 调整后 ds | scale | 最终轮廓边 |
|---|---|---|---|---|
| -120 (gbLS) | -1.5 | -1.5 (锁) | 847 | 底栏下 -0.781 / 外 -0.844（插值） |
| -4 (pb) | 0.0455 | **0.0** | 786 | 底栏上 0.0 |
| 71 (pt) | 0.9455 | 0.8995 | 956 | 内底缘 0.891 |
| 352 (pb) | 4.673 | **5.0** | 786 | 中横下 5.0 |
| 426 (pt) | 5.561 | 5.888 | 777 | 中横上 5.875 |
| 697 (pb) | 9.1585 | **9.1** | 786 | 顶栏下 9.094 |
| 772 (pt) | 10.0585 | **10.0** | 910 | 顶栏上 10.0 |
| 880 (gtLS) | 11.5 | 11.5 (锁) | 786 | — |

- 全部 8 边与 `ftexp contour 日 12 l` 最终轮廓逐边吻合（±1/64 截断误差来自 16.16→26.6）。
- **误差源**：用 26.6 截断 px 反推 cs 有 ±1.3FU 误差（如 cs 352 反推 351.56 → ds 差 0.006px）→ M1 必须用 charstring 精确 cs（这正是 cffcs.go 的必要性）。
- **M0 816 经验式解释**：816 = 幽灵区+adjustHints+插值的 12px 净效果，非真实蓝区值（fd12 顶区在 1100FU）。

### 13.6 FT 定点链公式（M1 实测破译，2026-08-04，cffcs_test.go 逐点验证）

- **x_scale（16.16）** = `FT_DivFix(charWidth26_6, upem)`（ftcalc.h）：`q = (|a|<<16 + |b|/2) / |b|`。例：12px/upem 1000 → `FT_DivFix(768, 1000) = 50332`（非 786.432 类浮点值！）。
- **cs → 26.6**（unhinted：cf2 以 unity 渲染，cff_slot_load 后乘 x_scale，cffgload.c:707-713）= `FT_MulFix(cs, x_scale)`，**2.14.3 内联 64 位版**（ftcalc.h:90-100，FT_INT64 默认开）：
  ```
  ab = cs * scale
  ab += 0x8000 + (ab >> 63)   // 正数 +0x8000（half-up）；负数 +0x7FFF（向 -∞ 偏移）
  26_6 = ab >> 16             // C 算术右移：负数向下取整
  ```
- **负数值语义**：-69 × 50332 = -3,472,908 → +0x7FFF → -3,440,141 → >>16 = **-53**（round-half-up 给 -52 是错的，差 1/64px）。正数 752 → **578**（非中间分支的 577）。
- 自研换算（cffcs_test.go `csTo26_6`）：cs 为整数 → `ftMulFix(int64(round(cs)), ftDivFix(px*64, upem))`，60 点（含 12 个负 Y）全部零误差。
- **M2 用途**：hinted 时 scale = `(x_scale+32)/64`（psft.c:265-290）→ cf2 内以 16.16 运算、路径点 `x >> 10` 落 26.6（psobjs.c:2190-2213）；此公式是 M2 映射校验的精度基准。

### 13.7 移植纪律（延续）

先读参考源码理解语义 → 用 ftexp/TRACE 实测验证理解 → 写 Go 实现 → fdiff 对照。禁止只按文档注释抄（FT 源码才是真语义）。

---

## 11. FreeType 源码参考（用户提供，2026-08-04）

**本地镜像**：`/home/yanghy/app/projects/gogpu/freetype-2.14.3/`（FreeType 2.14.3 官方源码，用户指定作为移植实现的对照蓝本）。

**定位与用途**：

| 源码文件 | 对应阶段 | 用途 |
|---|---|---|
| `src/psaux/pshints.c` + `pshints.h` | M1–M3 | **cf2 hint map 主参考**：`cf2_hintmap_build`（初始图/逐区图/插入/重叠丢弃）/`cf2_hintmap_adjustHints`（1px 双向拟合+二遍上移）/`cf2_hintmap_map`（插值）/`cf2_hint_init`（stem→edge）/`cf2_doStemHints`。M1 机制破译已数值级复现（§13） |
| `src/psaux/psblues.c` + `psblues.h` | M1–M3 | **cf2 蓝区参考**：emBox 幽灵区启发式（ICF ±120/880）/真实区构造（boost/suppressOvershoot/blueScale 钳制/family blues）/`cf2_blues_capture` 三态 |
| `src/psaux/psfixed.h` | M1–M3 | 定点语义：`cf2_fixedRound`（就近 1px）/`CF2_MIN_COUNTER`(0.5px)/`CF2_FIXED_EPSILON` |
| `src/psaux/psft.c` | M1–M3 | `cf2_getLanguageGroup`（decoder→private_dict）/`cf2_getBlueMetrics` 等取值路径 |
| `src/psaux/cffdecode.c` + `cffgload.c` | M1 | Type 2 charstring 解释器：操作符语义/hintmask 掩码/callsubr·callgsubr 递归/width 规则（对照实现，不抄） |
| `src/cff/cffparse.c` | M1 | Private DICT 解析（LanguageGroup 0x111 / Subrs 0x13 / DELTA 累积） |
| `src/autofit/afcjk.c` + `aflatin.c` + `afindic.c` | M4 | **glyf 无字节码 autofit light 化**（本轮后）；旧 §13 afcjk light 破译移至 M4 参考 |
| `src/autofit/afdummy.c` + `afglobal.c` | M4 | 其余脚本（阿拉伯/希伯来/泰文等）light 下透传不 hint 的判定 |
| `src/truetype/ttinterp.c` | L0–L1 | Latin bytecode 解释器 light 模式（`tt_interp_...` 的 light 分支） |
| `include/freetype/…`（fttypes.h/internal） | 全阶段 | 定点语义：FT_F26Dot6 / FT_Fixed（16.16）舍入规则 |

**移植纪律**：先读参考源码理解语义 → 用 ftexp 实测验证理解（如 M0 的 anchor=round(816×px/upem) 是实测反推，再回源码确认 blue zone 流程）→ 写 Go 实现 → fdiff 对照。禁止只按文档注释抄（FT 源码才是真语义）。