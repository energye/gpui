# ENGINE_TEXT_FREETYPE_PLAN — SELF-RASTERIZER HARDENING（已转向自研优化）

**状态**: 实施完成（2026-08-04）——方向已从「purego 绑 libfreetype 运行时」转为「自研光栅优化 + FreeType 仅作开发期像素对照度量衡」（用户确认：运行时零捆绑依赖）
**日期**: 2026-08-04
**触发**: R21 视觉 bug 轮2（Bug2 文字断笔/笔划粗）
**关联**: docs/ENGINE_UI_RENDER_BASE.md §0.6（①单测 ②指标接线 ③窗测）、AGENTS.md 分层规则（render/ 高风险，改前须确认）
**本文件保留原 purego 方案的探针与偏移速查（§1.4、§5、§6.2 对自研修复合用）；运行时依赖结论见 §8。**

---

## 0. 最终结论（2026-08-04 更新）

- **运行时保持零依赖**（不捆绑 libfreetype）：Bug2 的断笔已在 `render/internal/gpu/glyph_mask_engine.go` + `render/text/draw.go` 修复（rsnapshot 整数 X 定位，见 §8）。
- **libfreetype 仅作开发期度量衡**：`/tmp/opencode/ftexp` 在开发/CI 机用系统 freetype 生成同字形像素 golden 与连通块基线，用于对照验证；不进入运行时。
- 像素对照工具：`/tmp/opencode/rastercmp`（自研侧 dump PGM + 连通块），两侧同参数 diff。

## 1. 背景与目标

### 1.1 Bug2 现象与根因

自研光栅管线（`render/text/glyph_mask_rasterizer.go` + AnalyticFiller）小字号（10–16px）下 CJK 断笔、笔划偏粗。量化证据（连通块统计，4 连通）：

| 字 | size | 自研 no-hint 连通块 | FreeType light-hint 连通块 |
|---|---|---|---|
| 标 | 12 | 5 | — |
| 标 | 16 | — | **2** |
| 钮 | 16 | 5 | **1** |
| 文 | 16 | — | 1 |

差距来源（Skia/Chrome/Flutter 调研结论）：
- Chrome 桌面小字号正文 = FreeType coverage 光栅 + slight/light hinting，从不走 MSDF；
- Flutter/Impeller = at-scale 位图 glyph atlas（SDF 仍在开发）；
- 双方都带 gamma 校正 + stem darkening；
- 差距主因在 coverage 光栅算法本身（边缘判定/AA 权重），不在 hinting。

### 1.2 用户约束（硬）

- 框架最终跨平台（Linux/macOS/Windows），**不以 Linux 为唯一目标**；
- **不使用 cgo**（`CGO_ENABLED=0` 必须保持）；
- MSDF 保留，仅服务大字号/缩放文本。

### 1.3 选定方案

**purego 动态绑定 libfreetype**（与 exhost 绑 X11 同模式）：

- Linux：系统 `libfreetype.so.6`（无需捆绑）；
- macOS：系统 `libfreetype.6.dylib`（或捆绑）；
- Windows：捆绑 `freetype.dll`（随产品分发）；
- 库不可用时：**优雅降级回自研光栅器**（不报错、只记录指标）。

### 1.4 探针验证（已完成，2026-08-04）

`/tmp/opencode/ftexp/`（module ftexp，github.com/ebitengine/purego v0.10.2）已证明可行：

- 绑定 6 个符号：FT_Init_FreeType / FT_New_Memory_Face / FT_Set_Char_Size /
  FT_Get_Char_Index / FT_Load_Glyph / FT_Render_Glyph，全部调用成功；
- 8 个字形（标/钮/文/ i/g/e/A/l，10–16px）光栅成功，连通块统计合理；
- **偏移陷阱**（已踩坑并修正，写入 §5 防回归）：
  - FT_FaceRec.glyph 指针在 **offset 152**（FT_BBox = 4×FT_Pos = 32 字节，非 16）；
  - FT_GlyphSlotRec.bitmap 在 slot+152；FT_Bitmap.buffer 相对 **16**（非 24）；
  - bitmap_left/top 在 bitmap 之后（slot+192/+196，FT_Bitmap 共 40 字节）。
- 探针用 `FT_Set_Char_Size(face, px*64, px*64, 0, 0)`（72dpi 默认 → 1px=1px）+
  `FT_LOAD_TARGET_LIGHT`；`-buildvcs=false` 构建（VCS error）。

---

## 2. 架构设计

### 2.1 总览

```
                    ┌─────────────────────────────────────────┐
                    │          GlyphMaskRasterizer             │
                    │  (入口不变，draw.go 零改动)                │
                    └──────────────┬──────────────────────────┘
                        RasterizeHinted / RasterizeAliased / RasterizeOutline
                                   │ 后端分发
                    ┌──────────────┴──────────────┐
                    ▼                             ▼
        ┌─────────────────────┐       ┌──────────────────────┐
        │ FreetypeGlyphRaster  │       │  自研路径（现状）       │
        │  purego → libfreetype│       │  OutlineExtractor +   │
        │  FT_Load_Glyph +     │       │  AnalyticFiller       │
        │  FT_Render_Glyph     │       │  (降级目标)            │
        └─────────────────────┘       └──────────────────────┘
```

- 新增文件：`render/text/freetype_rasterizer.go`（后端）+ `freetype_platform.go`（平台库查找）
- `GlyphMaskRasterizer` 增加字段 `ft *FreetypeGlyphRasterizer`（懒初始化）；
  `RasterizeHinted/RasterizeAliased/RasterizeOutline` 入口分发：FT 可用且命中支持范围 → FT；否则自研。
- 调用方（`draw.go` 的 drawGlyphs / GlyphMaskAtlas.GetOrRasterize）**零改动**；
- RasterizeOutline(Outline) 系列保留自研（输入是已提取 outline，无 FT 必要）；
- `RasterizeLCD` **不在替换范围**（自研 LCD pipeline，FT_LCD 的 RGB 子像素布局不同）；
- MSDF/描边/变体不涉及。

### 2.2 FT 会话与并发

- 每个 `FreetypeGlyphRasterizer` 实例：独立 `FT_Library`（FT_Init_FreeType 一次）+ 自己的 face 缓存；
  GlyphMaskRasterizer 本就文档声明「非并发安全，每 goroutine 一实例」——延续该约束；
- face 缓存 key：`(RawFontData 切片首地址, collectionIndex)`；
  缓存的 `FT_Face` 生命周期 = rasterizer 生命周期（不调 FT_Done_Face，进程退出 OS 回收；
  若需显式释放，提供 Close()——首版不做，文档注明）。
- FT_Set_Char_Size 每 (face, ppem) 一次，缓存 face 同时缓存 char size 生效状态；
  子像素是 load 期参数（§2.4），不缓存。

### 2.3 数据来源映射

| 需求 | 来源 |
|---|---|
| 字体字节 | `ParsedFont.(RawFontDataProvider).RawFontData()`（ownParsedFont 已实现，autohint.go:130） |
| 集合索引 | **新增可选接口** `FontCollectionIndexProvider { CollectionIndex() int }`，`ownParsedFont` 实现（字段已存在：font_parser_own.go:74） |
| gid | `GlyphID`（uint16）→ FT_Get_Char_Index 不需要，直接传 gid 给 FT_Load_Glyph（我们已有 gid） |
| ppem | `size`（float64 px）→ FT_Set_Char_Size(face, size*64, size*64, 0, 0) |
| hinting | 映射表见 §2.5 |
| 子像素 | §2.4 |

ParsedFont 不支持 RawFontDataProvider（第三方 parser）→ 直接走自研，不尝试 FT。

### 2.4 子像素定位

- FT 方案：`FT_Set_Transform(face, nil, &FT_Vector{dx<<6, dy<<6})` 在 load 前施加平移
  （dx/dy = subpixelX/subpixelY，Y 向上取负）；
- 输出处理：mask 会整体平移子像素量，`bitmap_left/top` 相应变化；
  **需要回归校准**：同一 glyph 在 subpixelX=0 与 0.5 时 mask 右/下边界 + 与 atlas
  tight-bbox 的一致性（GlyphMaskRegion 期待整像素 bbox）。
- 阶段 2 扩展探针验证该路径后再实现（列入 §6 步骤）。

### 2.5 hinting 映射（对齐 Chrome/Skia 策略）

| 现有 Hinting | FT load flags |
|---|---|
| HintingNone | FT_LOAD_NO_HINTING（纯 coverage，光栅算法优势仍在） |
| HintingVertical | FT_LOAD_TARGET_LIGHT（autohinter 只做纵向网格拟合；Chrome Linux 默认） |
| HintingFull | FT_LOAD_TARGET_NORMAL（全 hinting） |

- 默认走 light（§2.6 说明为什么 light 而非 none）；
- CJK 字形 FreeType 天然不打 hint，改进来自光栅算法本身；
- FT_LOAD_NO_HINTING + 大字号（>48px）可不走 FT 直接自研（减少面开销）——
  首版不做阈值，全尺寸走 FT。

### 2.6 输出映射

FT_Render_Glyph(FT_RENDER_MODE_NORMAL) 输出 8-bit gray `FT_Bitmap`：

| GlyphMaskResult 字段 | 来源 |
|---|---|
| Mask / Width / Height | bitmap.buffer / bitmap.width / bitmap.rows（按 pitch 拷贝） |
| BearingX | `bitmap_left`（slot+192，float32） |
| BearingY | `bitmap_top`（slot+196，float32，正 = 基线之上） |

- bitmap_left/top 语义与现有 GlyphMaskResult 一致（自研来自 outline bbox 计算）；
- 空字形（space 等）：FT_Load_Glyph 成功但 bitmap 0x0 → 返回 `(nil, nil)`，
  与自研语义一致（nil = 空字形，非错误）；
- **gamma/stem darkening**：首版不做（FT 无开关，Skia 在 shader 侧做）；
  记录为后续 TEXT 候选（不在本次范围）。

### 2.7 平台库查找（freetype_platform.go）

| 平台 | 查找顺序 |
|---|---|
| linux | `libfreetype.so.6` → 次选 `libfreetype.so` |
| darwin | `/usr/lib/libfreetype.dylib` → `libfreetype.6.dylib` |
| windows | `freetype.dll`（与可执行文件同目录捆绑）→ 系统目录 |

实现：`freetype_platform.go` 内 `findFreetypeLib() []string`，每平台一个
build-tag 文件（system_font_*.go 同模式，见 render/text/system_font_linux.go）；
purego.Dlopen 失败 → 后端标记不可用，全量降级自研。

### 2.8 降级与观测（②指标接线）

- 降级是**永久优雅降级**（非 per-glyph 重试）：dlopen 失败 / 符号缺失 / FT_Init 失败
  → 本次进程内后端标记 disabled；
- per-glyph 失败（某 glyph 因字体/格式 FT_Load 报错）→ 该 glyph 单次回退自研；
- 观测指标（text 层原子计数，暴露到 metrics 族）：
  - `ft_glyphs`：FT 成功光栅数
  - `ft_fallback_glyphs`：单 glyph 回退自研数
  - `ft_disabled`：1/0（后端禁用原因位掩码：dlopen/符号/init）
- 真窗指标：示例窗展示三计数，确认正文路径命中 FT。

---

## 3. 兼容性与风险

| 风险 | 评估 | 缓解 |
|---|---|---|
| 偏移手算错误 | 探针已验证 152/152+40/192/196 | §5 防回归测试锁定 |
| TTC 多面 | collectionIndex 已存 | CollectionIndexProvider 接口 + FT_New_Memory_Face(index) |
| 子像素平移坐标差 | 未验证 | §6 阶段 2 探针扩展先验证 |
| 跨平台库差异 | macOS/Windows 未实测 | 平台层隔离 + 降级保证任何平台不坏 |
| 性能（每 glyph 一次 FT_Load_Glyph） | 与自研 ExtractOutline 同量级；atlas 缓存不变 | 阶段 4 benchmark 对照 |
| 线程安全 | 延续每实例一 library | 文档约束不变 |
| 与 TT 引擎共存 | 只换 mask 光栅，shape/度量不动 | 回归跑全套 text 单测 |

---

## 4. 测试计划（对齐 RENDER_BASE §0.6）

### ① 单测（单元层）

- `render/text/freetype_rasterizer_test.go`：
  - **连通块断言**（防回归核心）：`标/钮/文`@12–16px 走 FT 后 comps ≤ 自研基线
    （标 12px：5 → ≤3；钮 16px：5 → ≤2）；用测试字体（testdata 内系统字体可依
    赖性 vs 内置字体，优先内置 TTF 构造）；
  - hinting 映射表：三种 Hinting × FT flags 断言；
  - 子像素：subpixelX=0/0.5 输出尺寸、bearing 单调性；
  - 空字形：space gid → nil,nil；
  - 降级：`WithFreetypeForced(false)` 强制自研 → 结果与现有 golden 一致；
  - TTC：双面集合 index 0/1 输出不同（若 testdata 有 TTC）；
  - 输入字节等价：RawFontData 缺失（mock ParsedFont 不实现 provider）→ 走自研。
- `glyph_mask_rasterizer_test.go` 补：后端分发单元（mock 后端开关）。

### ② 指标接线

- text 层原子计数 + 单测断言（强制 FT 时 ft_glyphs>0；强制禁用时 ft_disabled=1）。

### ③ 窗测（真窗回归）

- `examples/ui_wr_r21_shell` 回归：15s 无交互 + 30s 温和 resize（Bug1 门禁不回退）；
- 视觉目检：标题/按钮 12–16px CJK 断笔消失（连通块证据 + 截图对比）；
- 若 render/ 3 个基线 FAIL（dash/hairline/AA）仍为改动前后一致 → 维持基线标注。

---

## 5. 防回归：FT 结构偏移速查（写进代码注释）

已验证（x86_64, freetype 2.13，LP64）：

```
FT_FaceRec:  ... bbox(4×FT_Pos=32B) + 8×FT_Short(16B) → glyph @ 152, size @ 160
FT_GlyphSlotRec: library/face/next/glyph_index/generic/metrics(64B)/
                 linearH/V/advance/format → bitmap @ 152
FT_Bitmap(40B): rows@0, width@4, pitch@8, buffer@16, num_grays@24,
                pixel_mode@26, palette_mode@27, palette@32
bitmap_left @ slot+192, bitmap_top @ slot+196
```

任何偏移改动必须经 `freetype_rasterizer_test.go` 连通块断言 + 探针回归。

---

## 6. 实施步骤（每阶段完成停下报告）

| 阶段 | 内容 | 产物/验证 |
|---|---|---|
| 1 | 平台层 + FT 会话/face 缓存骨架 + 空实现占位 | freetype_platform.go 各平台查找；dlopen 失败降级路径单测 |
| 2 | 探针扩展：子像素平移 + TTC index + 输出坐标校准 | ftexp 扩展跑通，校准结论写回 §2.4 |
| 3 | FT 后端光栅实现 + 分发接入 GlyphMaskRasterizer | ①单测全绿（连通块/映射/降级/TTC） |
| 4 | 指标接线 + benchmark | ②③：text 层计数、性能对照 |
| 5 | 真窗回归 + 目检 | ui_wr_r21_shell 15s + 30s resize + 截图 |
| 6 | 收尾 | docs §2 主表/§10 修订记录（Bug2 修复）；如需 R21 重关走 wr-close 模式 3 |

---

## 7. 不做的事（明确边界）

- 不做 MSDF 替换（保留大字号路径）；
- 不做 LCD 路径替换（FT_LCD 布局不同）；
- 不做 gamma 校正/stem darkening（记录 TEXT 候选）；
- 不做 cgo / 不做 purego 绑 libfreetype **运行时**路径（用户确认零捆绑；libfreetype 仅开发期 golden）；
- 不替换 tt_engine 解释器 / shaper / 度量（只换 mask 光栅）。

## 8. 自研优化实施结果（2026-08-04，替代 6 阶段计划）

### 8.1 根因

- **光栅器本身无断笔**：NO_HINTING + 整数定位下，自研与 FreeType 同轮廓连通块一致（12px `标`=2、`钮`=1、`文`=1；16px `标`=2）。历史"自研 5 连通块"与当前代码不符。
- **断笔在 placement 层**：hinted 文本若带小数 X 定位进光栅，1px 垂直 CJK 笔划被劈成两个半覆盖列——**FreeType 自身在 fracX=0.5 时同样断笔**（ftexp 实测 comps 2→3）。Skia 对任何 hinted 字形强制整数设备放置。
- **引擎漏洞**：`glyph_mask_engine.go` 的 `snapX := hinting == HintingFull && !useLCD`——CJK 走 `HintingVertical` 时 X 不 snap，fracX 小数泄漏进光栅 → 真窗 CJK 标题/按钮断笔、时粗时细。

### 8.2 修复（render/ 高风险，经用户确认自研优化方向）

- `render/internal/gpu/glyph_mask_engine.go`：`snapX := hinting != text.HintingNone && !useLCD`——CJK Vertical 也走整数 X 放置（advance 网格）；LCD 保留 RGB 相位；HintingNone（大字号/高 DPI 旋转）保留小数。
- `render/text/draw.go`（CPU 软件路径，两条 drawGlyphs/drawGlyphsVariable 等同修复）：hinted 时 `subpixelX=0`、`subpixelY=0`（Y 本就 grid-fit，不再重平移）、X 用 round-advance 网格。

### 8.3 验证

| 项 | 结果 |
|---|---|
| 对照（FreeType vs 自研，同轮廓） | NO_HINTING 连通块全一致；fracX=0.5 时双方同时 comps 2→3（断笔机制共证） |
| 单测（新） | `render/internal/gpu/glyph_placement_test.go` 5 项：CJK-V snapX / Full snapX / None 保留小数 / LCD 保留 X 相位 / snapXGrid 单调整数 |
| 回归 | `go test ./render/text/` PASS；`./render/...` 失败集 = stash 基线完全相同（**零新增**）；`pipeline_wiring_test`/SDF/S69/AA/dash/hairline 等均为既有基线 |
| 真窗 | `ui_wr_r21_shell` 15s：`OK shell_rr_scroll=0 shell_rr_total=1 shell_skip=896 presents=900 fps_interval=59.93 hitch=0` 全绿 |

### 8.4 追责与后续

- 后续阶段（细分步长/覆盖率 vs FT 像素级 diff）暂缓——当前结构一致已达标（用户目标：运行时零依赖）。
- libfreetype 开发期对比流程保留（§1.4 探针 + §5 偏移速查），供未来算法级优化对照。

## 9. R21 验收追加：mask 管线 hinting 全面 None 化（2026-08-04）

### 9.1 背景

用户验收 R21 多字号样张时反馈：真窗文字「笔划很粗 / 显示效果完全不一样」。逐级定位：

1. 自研 None hint 与 FT-nohint **逐字像素级一致**（墨量全等，S/H/E/L/钮/层/按 8–16px 全尺寸）；
2. 自研 Vertical hint 与 FT-light **xcorr 0.69 无单峰**（结构级差异：CJK 竖笔 Y-snap 规则与 FreeType afcjk 完全不同）；
3. 自研 Full hint 拉丁 H/E/L 墨量比 FT-light **少 ~40%**（竖笔偏细）；
4. GPU atlas 层排除（nearest 采样 + 整数 UV）；字体混淆排除（全 Noto 版 match 不变）；
5. **结论：问题不是"粗"，是自研 Vertical/Full 的 hint 规则与 FreeType 不同 → 字形结构不一致**。

用户认可 FT-light/FT-nohint 参考图。**选中方案：CJK+Latin 一起验 → mask 管线统一 None**。

### 9.2 改动（本决策）

| 文件 | 改动 |
|---|---|
| `render/internal/gpu/glyph_mask_engine.go` | `selectGlyphMaskHinting` 恒返回 `HintingNone`（CJK/Latin 全字号；旋转/大字号本就 None）。`snapX := !useLCD`：None 也走整数 X 网格（round-advance 网格）+ Y 恒 snap（FT 整数网格基线）→ 无间距抖动、与 FT-nohint 整数网格光栅一致 |
| `render/text/glyph_mask_rasterizer.go` | 未改（None 光栅 = FT_LOAD_NO_HINTING 对齐，前轮已验证） |
| `examples/wrkit/font.go` | `FaceAt` → `src.Face(points, text.WithHinting(text.HintingNone))`，CPU fallback 与 GPU mask 一致 |
| 测试 | `glyph_mask_engine_test.go` / `gpu_text_cjk_test.go` / `glyph_mask_hinting_test.go` / `glyph_placement_test.go` 更新为 None 契约（Y 恒 snap 新增 `TestGlyphPlacementNoneSnapsX`） |

### 9.3 关键证据（本决策）

- **OLD vs NEW 真窗标题区**（同文本 Vertical/Full vs None）：xcorr 0.9573；墨量 OLD 6872 < NEW 7137（旧版更细 = Vertical/Full 墨量少于 None，与离屏逐字结论方向一致）。
- **样张 5 档**（8/10/12/14/16px）在真窗全部可见（xwd 抓屏存在 +769 循环偏移，左移恢复后确认，用户真实视角无偏移）。
- **GPU=CPU 一致性**：同一 `GlyphMaskRasterizer` None 光栅 + 整数网格（GPU `snapXGrid` = CPU round-advance 网格）→ 字形光栅化代码链路同一。
- **回归**：`go test ./render/internal/gpu/ ./render/text/` 仅剩 `TestSDFAccelerator_SceneStats_ResetOnFlush` = stash 基线既有失败（零新增）。

### 9.4 后续里程碑（非本次范围）

- advance 精度（16.16 定点 / 小数 advance 保留）；
- Latin 字节码字体兼容面（Noto Sans/Ubuntu 等 hint 程序差异）；
- LCD 亚像素渲染（`selectGlyphMaskLCD` 保留，与 None 正交）；
- 阿拉伯语/泰文等复杂 shaping（HarfBuzz 对齐）。

### 9.6 autohint 脚本蓝区（2026-08-05 追加，非本次 9.x 范围）

2026-08-05 已导入 skrifa 全量脚本（`autohint_scripts_gen.go`，51 脚本 + t2b 字段），
并修复两个引擎/生成器 bug（LONG 检测条件、union 链首 flags 丢失）。探针比对
skrifa 数值：Thai/Bengali/Tamil 完全一致、Gujarati 大部分一致。已知差异：

- **复合 cluster（已修复，2026-08-05 晚）**：skrifa 蓝区测量在 ShaperMode::Nominal
  下**拒绝多码元 cluster**（shape.rs 引用 FreeType afshaper.c:639）——并非走 GSUB
  整形。Go `computeDefaultBlues`/`computeCJKBlues` 已同步跳过多码元字段
  （`len(rune) != 1`），无需 GSUB 接入。修复后探针验证：Gujarati（Kalapi）5/5
  zones、Kannada（Gubbi）2/2 zones 与 skrifa 全一致（pos/over/asc/desc/flags，
  含 Gujarati zone2=1166）。Khmer/Malayalam/Sinhala/Mongolian/Chakma/Kayah Li
  本机无字体，未实测（同一代码路径，逻辑等价）。
- **hint_top_to_bottom**：Bengali/Devanagari/Gothic/Gurmukhi/Mongolian 5 个脚本
  的 t2b 数据已生成（scriptClass.hintTopToBottom），但 autohint_edges.go 的
  蓝区匹配算法未接入（需对照 skrifa topo/edges.rs 实现）。
- 探针产物：`/tmp/opencode/skrifa-0.31.1`（Rust 源 + 蓝区探针测试）。

### 9.5 遗留注意

- 真窗抓屏：`xwd -id` 在 GNOME 合成器下存在 **+769px 循环 x 偏移**（OLD/NEW 一致，非渲染问题）；分析时左移恢复；用户实际视角无偏移。
- 真窗 build 需长 run（>120s）再抓屏，滚动期间早抓会拿到未完成帧（大面积黑）。
