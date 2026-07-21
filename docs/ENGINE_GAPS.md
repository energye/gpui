# 引擎必有缺口（Engine-scoped gaps）

> 版本：1.26 | 日期：2026-07-21 | **活文档 · 缺口唯一真源**  
> 范围：**仅渲染引擎**（`render` / `gpu/webgpu` / `gpu/rwgpu`）  
> 真源：现网代码；与本文冲突时以代码为准  
> 非目标：布局 / HitTest / 焦点 / IME / 控件树（控件层）  
> 索引：[`README.md`](./README.md) · 主线：[`MAINLINE_PLAN.md`](./MAINLINE_PLAN.md) · 能力表：[`SKIA_2D_CAPABILITY_MATRIX.md`](./SKIA_2D_CAPABILITY_MATRIX.md)

---

## 怎么用本文（写缺口时照此）

| 规则 | 说明 |
|------|------|
| **只在这里写引擎缺口正文** | 矩阵只钉行（如 X.01→G1.b）；MAINLINE 只钉优先级；不另开缺口档案 |
| **一条 = 现状 + 影响 + 证据 + 验收** | 无代码路径 / 无可测标准的描述不进主缺口 |
| **先分档再编号** | 主缺口 G1–G3 → 次级 P* → 非引擎 §3；优先扩子项（G1.d），勿轻易新开 G4 |
| **先 `rg` 再改字** | 代码已修 → 降级/删除；仍缺 → 只更新现状与验收 |
| **API 缺行 vs 深度不够** | 缺 API 行 → 能力矩阵；有 API 但不深/不稳/不高效 → 本文 |

---

## 0. 结论（一屏）

| 判断 | 结论 |
|------|------|
| 画布 API 能否撑 antd 风格绘制 | **能**（矩阵除 R.02 PDF/SVG 外已齐） |
| 引擎是否零缺口 | **否** — G1 大部+CFF1+**CFF2 出字+轴 blend**+MarkFilteringSet+matra 类+多辅音 base/reph+**font pos（含 base 选择）**+**Khmer·Myanmar 轻量**+**cluster hit-test**+**G1.a 长 reshape soak** ✅；G2 契约+blit 局部像素 ✅；G3 TestMem ✅ + **mem_guard quick+daily+deep 主路径** ✅；**遮挡后 invalid handle 已按 hidden 处理**；仍开：full OT match-class rewrite |
| 控件层能否开工 | **能**（见 `S5_WIDGET_ENTRY`）；G1–G3 影响深度/性能/生产稳，**不是**缺一整块绘制 API |
| 当前工程跟进 | **P0 = G1–G3 + 稳**（MAINLINE §3） |

---

## 1. 主缺口（必跟）

### G1 — 文本栈深度与正确性

| 子项 | 现状（代码） | 影响 | 证据 | 修好标准 |
|------|--------------|------|------|----------|
| **G1.a 长文 / Input 路径** | shape + atlas + `DrawString` 可用；**产品级 CPU soak 门禁已落地**（`TestG1a_*`）。**mem_guard quick/daily/deep 主路径** ✅（deep：P_MEM_LONG 600s PASS）。**遮挡/切到后面窗口**：present `invalid handle` 根因 = 不可见仍 acquire/present；`particle_kitchen_sink` 已把 invalid handle 视为 obscured（pause + Unconfigure，露出后 reconfigure），不计入 steady present 门禁 | Input 密集编辑、表格滚动 | `g1a_soak_test.go` · `examples/particle_kitchen_sink/main.go`（`isSurfaceHandleInvalid`）· `tmp/mem_guard_*` | `TestG1a_*` 绿；mem_guard 主路径绿；遮挡恢复不 abort |
| **G1.b CFF / CFF2 轮廓** | **CFF 1 已出字**（`sfnt` Type2）。**CFF2 出字** ✅（go-text `ParseCFF2`/`LoadGlyph`；Y-up→Y-down）。**CFF2 轴 blend** ✅（`cff2VariationCoords`：fvar 归一化 + avar → `tables.Coord`；`ExtractOutlineHintedVar` / HVAR advance）。损坏/截断 CFF2 仍 `ErrCFF2Unsupported`。**仍开**：CFF 无 TT/auto-hint | CFF2 VF 可画可插值 | `cff_outline.go` · `TestCFF2_OutlineAndRaster_NotoVF` · `TestCFF2_VariationBlend` · `TestCFF2_DetectedAndRejected` | CFF1 绿；CFF2 出字+mask；轴 blend 几何可测；坏表拒绝 |
| **G1.c 复杂 OT shaping** | **GSUB/GPOS Type 1–9** ✅。**RTL visual** ✅。**GDEF** ✅。**Arabic joining** ✅。**Indic 特征分期** ✅。**Indic reordering** ✅。**Static below-base 辅音类** ✅。**Per-font positional coverage** ✅（`indic_font_pos.go`：blwf/pstf/**rkrf/vatu** Coverage 收割；**final 桶 + base 选择**均走 font pos；`ownShaperCache.indicFontPos` 缓存）。**Khmer·Myanmar 轻量** ✅。**Cluster hit-test** ✅。**仍开**：full OT match-class rewrite / 真实字体像素 golden | 极少数字体特有 class 仍可能不完美 | `indic_font_pos.go` · `TestFindBase_FontPosSkipsCoveredConsonant` · `TestReorderFinal_FontPosOverridesStatic` · `shaper_own.go` · `indic_reorder.go` · `shaped.go` | font pos base 跳过可测；full OT class 另专项 |

**排期建议：** G1/G2/G3 主路径门禁**已齐**（G2/G3 2026-07-21 源码+`TestG2_*`+`Lifecycle*` 再验）。剩余引擎深度：full OT match-class rewrite；工程：mem_guard 例行化。

**相关矩阵行：** X.01（Face/CFF）· X.03（shaping）

---

### G2 — 矢量路径下脏区 / 合成效率

| 子项 | 状态 | 现状（代码） | 影响 | 证据 | 修好标准 / 契约 |
|------|------|--------------|------|------|-----------------|
| **G2.a MSAA 矢量帧** | ✅ 契约 | 帧含 Fill/Stroke → MSAA 路径 **恒 `LoadOpClear`**；`damageRect` **不保留**旧像素 | 「只脏几像素」无法在矢量全帧上省 fill | `context.go` `FlushGPUWithViewDamage` · **`TestG2_DamageContract_API`**（再跑绿） | **契约可测** ✅；**不承诺**任意矢量脏区=局部重画（边界非未做） |
| **G2.b blit-only 路径** | ✅ | 仅 `DrawGPUTexture*`（无矢量）时 **`LoadOpLoad` + scissor** | 控件层缓存 RT / 分层合成时有效 | **`TestG2_BlitOnly_DamagePreservesOutsidePixels`**（域外白/域内红；再跑绿） | 纯 blit 帧 damage 保留域外像素 ✅；混矢量走 G2.a |
| **G2.c OS Present damage** | ✅ 边界钉死 | `Surface.PresentWithDamage` **忽略 rect**（wgpu-native 不支持） | 无 OS 多矩形 present 省电收益 | `gpu/webgpu/surface.go` 注释+实现 · **`TestG2_PresentWithDamage_IgnoresRect_Doc`** | 忽略行为可测 ✅；后端支持后再接线（**非漏实现**） |

**产品预期（写进对外/控件约定）：**

- 引擎 **提供** damage API + scissor  
- **不承诺**「任意矢量脏区 = 只重画脏矩形像素」  
- 稳 60fps 依赖：**轻脏 UI / 分层 RT / 少全屏滤镜**；重全帧矢量 **不保证** 60  

**G2 收口判断：** 三项 **主路径/契约均已落地且可测**；剩余是产品预期边界（矢量不全帧省 fill、OS present 无 multi-rect），不是「缺口没做」。

**相关矩阵行：** S.09（API ✅，效率有界）

---

### G3 — 重层 + 滤镜 + 多 RT 稳定性（lifecycle / VRAM）

| 子项 | 状态 | 现状（代码） | 影响 | 证据 | 修好标准 |
|------|------|--------------|------|------|----------|
| **G3.a 多离屏 / filter 图** | ✅ | API 齐；**`TestMem_T0–T4` 已绿**。`run_mem_guard.sh` 已钉 **GO_BIN≥1.25**。**遮挡策略** particle：fully obscured / invalid handle → 不 present | Modal / Drawer / 毛玻璃 / 多路由 RT | `mem_*_test.go` · `scripts/run_mem_*.sh` · mem_guard quick/daily/deep 主路径 | `TestMem_*` + mem_guard 绿 ✅；遮挡不 abort ✅ |
| **G3.b Device lost / recover** | ✅ | sticky lost · AutoRecover · `ForceRecoverHealthy` · Context 注册表 abandon · CB/pool/pipeline 回收 **已落地** | 遮挡/TDR 后 OOM、双 Device 堆 | [`GPU_修复_device_lost.md`](./GPU_修复_device_lost.md) · 源码钩子 | force-lost + recover 路径矩阵绿；恢复后无双 Device / 泄漏爬升 |
| **G3.c Surface lifecycle** | ✅ | tier **Normal / Purge / Recreate** 已实现；**`auto` 默认 = Purge**（含 dGPU）；仅 `GPUI_LIFECYCLE=normal` → Normal；`NoteTextureOOM` → 升 **Recreate** | 最小化/恢复跨硬件显存与恢复行为 | `lifecycle_policy.go` · **`TestResolveSurfaceLifecycle_*`**（再跑绿） · SURFACE 文 | lifecycle 可测 ✅；OOM 后自适应 Recreate 可测 |
| **G3.d VRAM 基线** | ✅ 策略 | 独显 Vulkan 设备基线偏高（本机约 **300MiB** 级）；可 low-power / adapter 策略 | 弱显存机 | [`VRAM_BUDGET.md`](./VRAM_BUDGET.md) · AdapterPolicy 测试 | 预算文档与策略开关可测 ✅；**非**「基线降到 0」 |

**G3 收口判断：** a–c **实现+门禁主路径已齐**；d 为**可测预算/策略**（硬件基线成本仍在）。日常靠 mem_guard 例行，不是再开 G3 功能大开发。

**验收命令（引擎侧常用）：**

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

go test -count=1 ./render/gpu -run 'Lifecycle|TextureOOM|AdapterPolicy' -timeout 60s
./scripts/run_mem_guard.sh
GPUI_COVERAGE_STRICT=1 GPUI_SELFTEST_LIFECYCLE=1 go run ./examples/api_coverage_app
# force-lost 见各 shell 示例 GPUI_FORCE_LOST_AFTER / ForceRecoverHealthy
```

**相关活文档：** SURFACE · device_lost · VRAM · MEM_LEAK_*

---

## 2. 次级 / 性能 polish（不挡控件开工）

| ID | 项 | 说明 | 去哪跟 |
|----|-----|------|--------|
| P1 | 嵌套 path-clip 恢复限制 | `depth_clip` 单 path/group；antd 多为 rect/rrect | 用到再加深 |
| P2 | Path boolean 质量 | UI soft path，非 CAD 级 | 视觉回归时再抬 |
| P3 | F16 / 宽色域全 Context 链 | RT 有；Context 仍 8-bit（矩阵 CS.02） | 色管产品需求 |
| P4 | N3–N5 冷路径 GPU 化 | CustomBrush fragment / Bicubic / 极冷 path effect | `GPU_FIRST_ROUTING` 后置 |
| P5 | Backdrop 无 readback 拷贝 | 可选性能 | `PERF_ENGINE_FORWARD` |
| P6 | COLR/SVG emoji 深度 | 部分 emoji 路径有 TODO | 文本/emoji 专项 |

P 级 **不**升主缺口，除非变成「无此则生产不可用」。

---

## 3. 明确非引擎缺口（勿写进 G）

| 项 | 归属 |
|----|------|
| Flex/Grid 布局、组件状态 | 控件层 |
| HitTest 树、焦点路由 | 控件层 |
| IME 组合态、按键语义 | OS + 控件层 |
| 滚动 offset / 虚拟列表策略 | 控件层 |
| 光标闪烁 / 选区**状态** | 控件层（引擎只画几何） |
| 无障碍树 | 非 GPU 画布 |
| PDF/SVG 文档后端 R.02 | 旁路（`DOC.1`，不挡画布 100%） |

---

## 4. 新缺口落档流程（给后续改文档的人）

1. **定性：** API 缺行？深度/效率/稳定？控件/OS？  
2. **对代码：** `rg` 符号/注释/未实现分支；无证据 → 不进 G。  
3. **落档：**  
   - 主问题 → **G1–G3 子项**（或极少情况新 Gx）  
   - polish → **P***  
   - 非引擎 → **§3**  
4. **双向钉：** 本文表行 + 能力矩阵对应行一句「见 Gx.y」  
5. **MAINLINE：** 仅当优先级变化时改 §3 一行  
6. **修订表：** 只记事实变更（边界、默认策略、删错号）

**不要：** 另开「缺口 v2」文档；把单次未复现 crash 直接升 G；把 PERF 优化抬成挡开工主缺口。

---

## 5. 文档 / 代码映射

| 缺口 | 活文档 | 关键代码 |
|------|--------|----------|
| G1 | 本文 · 矩阵 X.* | `render/text/`（G1.a–c：cff/gsub/gpos/bidi reorder · shape/atlas soak） |
| G2 | 本文 · 矩阵 S.09 | `render/context.go` · `gpu/webgpu/surface.go` |
| G3 | SURFACE · device_lost · VRAM · MEM_LEAK_* | `render/mem_window_x11_linux_test.go` · `lifecycle_policy.go` · `scripts/run_mem_*.sh` |
| 原则 | `GPU_FIRST_ROUTING` | ensureGPU / fallback 观测 |
| 优化执行 | `PERF_ENGINE_FORWARD` · `CODE_CONVERGENCE` | 热路径改动任务卡 |

---

## 6. 修订

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-21 | 1.0 | 首版：从源码与 antd 引擎职责评估收敛 |
| 2026-07-21 | 1.1 | 源码对照：GSUB/GPOS 类型边界；lifecycle auto=Purge；去掉错误 G4 |
| 2026-07-21 | 1.2 | 可执行化：每子项补「修好标准」；用法/落档流程；**修正 G3.c**（auto 默认 Purge，含 dGPU，非「非离散=Normal」）；GPOS 注明 Type 9 extension |
| 2026-07-21 | 1.3 | **G1.b CFF 出字落地**：`cff_outline.go` + `TestCFF_*`（CFFTest/Nimbus/NotoSansCJK）；CFF2 仍开；更新排期 |
| 2026-07-21 | 1.4 | **G1.c**：GSUB 5/6/8 + GPOS 3/4/6；默认 feature 扩 calt/locl/init…/mark/mkmk/curs；BiDi/GPOS 5/7/8 仍开 |
| 2026-07-21 | 1.5 | **G1.a**：`TestG1a_*` Input/列表/编辑 reshape soak；shape+atlas 有界门禁；真窗口仍开 |
| 2026-07-21 | 1.6 | **GPOS Type 5** mark-to-lig；mem 脚本 GO_BIN 钉死；**TestMem_T4** X11 cleanup SIGSEGV 修复；T0–T4 绿 |
| 2026-07-21 | 1.7 | **GPOS Type 7/8** context/chaining pos（`gpos_context.go` + `TestGPOS_Context*`）；GPOS Type 表 1–9 齐 |
| 2026-07-21 | 1.8 | **RTL visual reorder**：`ReorderRTLShapedGlyphs` + layout segment RTL；`TestReorderRTL*` / `TestOwnShaper_RTL*` |
| 2026-07-21 | 1.9 | **GDEF+IgnoreMarks**（连字/配对）；**G2 damage 契约测试** `TestG2_*` |
| 2026-07-21 | 1.10 | **Arabic joining** isol/init/medi/fina 分阶段；**MarkAttachmentType**；`TestArabicJoining_*` |
| 2026-07-21 | 1.11 | **Indic 特征分期** `indic_shaping.go` + 默认 rphf/half/vatu/pres…；`TestGSUB_StagedIndic_*` |
| 2026-07-21 | 1.12 | **Indic 轻量 reordering** reph/pre-base matra；`indic_reorder.go` · `TestIndic*` |
| 2026-07-21 | 1.13 | **CFF2 检测/拒绝** `ErrCFF2Unsupported`；**G2 blit-only 局部像素** `TestG2_BlitOnly_DamagePreservesOutsidePixels` |
| 2026-07-21 | 1.14 | **MarkFilteringSet**（GDEF MarkGlyphSets + LookupFlag bit 4 + lookup 解析）；**Indic matra 类** pre/above/below/post + peer pre-base；`TestGDEF_MarkFilteringSet` · `TestIndicCategory_MatraClasses` |
| 2026-07-21 | 1.15 | **Indic 多辅音 base/reph**：`findIndicBaseIndex`；final matra 桶序；`TestIndicFindBase_*` · `TestIndicInitial_RephAfterMultiConsonantBase` · `TestIndicFinal_MatraBuckets` |
| 2026-07-21 | 1.16 | **CFF2 默认实例出字**（go-text CFF2）；`TestCFF2_OutlineAndRaster_NotoVF`；坏表仍拒绝 |
| 2026-07-21 | 1.17 | **CFF2 轴 blend 接线**：fvar/avar → LoadGlyph coords；`ExtractOutlineHintedVar` CFF2 路径；`TestCFF2_VariationBlend` |
| 2026-07-21 | 1.18 | **Khmer·Myanmar 轻量 reordering**：coeng/kinzi 分类；pre-base；kinzi initial/final；`TestKhmer*` · `TestMyanmar*` |
| 2026-07-21 | 1.19 | **Cluster hit-test** visual X↔logical cluster（RTL 保 Cluster）；`HitTestCluster*` · `CaretXForCluster` · `TestHitTestCluster_*` |
| 2026-07-21 | 1.20 | **Static below-base 辅音类**（base 选择跳过 rakar 等；final below/post 桶）；**G1.a 长 reshape residual heap** 门禁 |
| 2026-07-21 | 1.21 | **Per-font blwf/pstf** coverage → below/post 类；`indic_font_pos.go`；**mem_guard quick** PASS（TestMem+ P_MEM_SOAK 60s） |
| 2026-07-21 | 1.22 | **rkrf/vatu** coverage + fontPos **缓存**；**mem_guard daily** PASS（P_MEM_LONG 180s + matrix + perf） |
| 2026-07-21 | 1.23 | **font pos 接入 base 选择**（`findIndicBaseIndexWithFont` in final）；`TestFindBase_FontPosSkipsCoveredConsonant`；修正 `gsub_context` GDEF 注释 |
| 2026-07-21 | 1.24 | **mem_guard deep**：P_MEM_LONG 600s PASS；P_BLEND_LAYER deep 套件偶发 present invalid handle（隔离 3×30s + 45s 复测全 PASS） |
| 2026-07-21 | 1.25 | **根因确认**：窗口切到后面/不可见仍 present → `invalid handle`；particle 将 invalid handle 当 obscured 暂停 present |
| 2026-07-21 | 1.26 | **G2/G3 逐项状态列**（与 G1 一致标 ✅）；源码再验 `TestG2_*` / `Lifecycle*` 绿；澄清 G2 契约边界 ≠ 未做 |

---

## 7. 源码对照检查（多轮）

| 轮次 | 检查项 | 结果 |
|------|--------|------|
| R1 | G1.b CFF 出字：`cff_outline.go` + `TestCFF_*`；无 glyf 走 sfnt；CFF2 仍未做 | ✅（1.3） |
| R7 | G1.c GSUB 5/6/8 · GPOS 3/4/6：`gsub_context.go` · `gpos_mark.go` · 单测绿 | ✅（1.4） |
| R1 | G1.c 历史对照行；以 R7/R9/R10 为准（GSUB 1–8 · GPOS 1–9） | ⚠ 已由后续轮取代 |
| R1 | G2.a `FlushGPUWithViewDamage`：矢量 MSAA → LoadOpClear | ✅ |
| R1 | G2.c `PresentWithDamage` 忽略 rect | ✅ |
| R2 | G3 `NoteTextureOOM` / `ForceRecoverHealthy` / registry abandon | ✅ |
| R2 | lifecycle **`auto`→Purge（含 dGPU）**；仅 env `normal`→Normal；OOM→Recreate | ✅（1.2 纠正表文案） |
| R3 | 活文档路径无悬空 | ✅ |
| R4 | SurfaceHost：Purge/Recreate 才 Unconfigure+DropGPU | ✅（见 SURFACE 文） |
| R5 | VRAM adapter 策略文档链存在 | ✅ |
| R6 | 2026-07-21 再扫：上文符号仍在；G3.c 与 `ResolveSurfaceLifecycle` 一致 | ✅ |
