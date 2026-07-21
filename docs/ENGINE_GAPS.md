# 引擎必有缺口（Engine-scoped gaps）

> 版本：1.2 | 日期：2026-07-21 | **活文档 · 缺口唯一真源**  
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
| 引擎是否零缺口 | **否** — 主缺口 **G1–G3** |
| 控件层能否开工 | **能**（见 `S5_WIDGET_ENTRY`）；G1–G3 影响深度/性能/生产稳，**不是**缺一整块绘制 API |
| 当前工程跟进 | **P0 = G1–G3 + 稳**（MAINLINE §3） |

---

## 1. 主缺口（必跟）

### G1 — 文本栈深度与正确性

| 子项 | 现状（代码） | 影响 | 证据 | 修好标准 |
|------|--------------|------|------|----------|
| **G1.a 长文 / Input 路径** | shape + atlas + `DrawString` 可用；密集编辑 / 长列表下 reshape、cache、atlas **未做产品级长 soak** | Input、表格、虚拟列表滚动卡顿或内存爬升 | `render/text/*` · 矩阵 X.* 有门禁但非编辑器 soak | Input 形态长 soak：atlas 上传有界；RSS/VRAM 斜率≈0（对齐 `MEM_LEAK_*`） |
| **G1.b CFF / CFF2 轮廓** | own parser **仅 TrueType `glyf`**；CFF 无 glyf → **零框 / 不出字** | 系统 OTF、多数 Noto CJK 桌面字体 | `font_parser_own.go`：`CFF fonts are not yet supported` · `glyf_parser.go` | 指定桌面/CJK **OTF(CFF)** 可测出字 + 度量；parser 不再对有轮廓字返回 zero box |
| **G1.c 复杂 OT shaping** | **GSUB** Type **1–4、7** ✅；**5/6/8**（contextual / chaining / reverse）❌。**GPOS** Type **1–2**（+ Type 9 extension）✅；**3–8**（cursive / mark / context…）❌ | 阿拉伯、印地等；mark 附着、复杂连写 | `render/text/gsub.go` · `gpos.go` · `Script.RequiresComplexShaping` | 目标 script 集合有像素/advance 门禁；文档写明 Type 覆盖表并与代码一致 |

**排期建议：** G1.b（出字面）→ G1.c（目标 script）→ G1.a（产品 soak）。中文 antd 桌面若可绑 TTF/glyf 可先绕开 G1.b，但系统字体路径仍会撞上。

**相关矩阵行：** X.01（Face/CFF）· X.03（shaping）

---

### G2 — 矢量路径下脏区 / 合成效率

| 子项 | 现状（代码） | 影响 | 证据 | 修好标准 / 契约 |
|------|--------------|------|------|-----------------|
| **G2.a MSAA 矢量帧** | 帧含 Fill/Stroke → MSAA 路径 **恒 `LoadOpClear`**；`damageRect` **不保留**旧像素 | 「只脏几像素」无法在矢量全帧上省 fill | `render/context.go` `FlushGPUWithViewDamage` 注释 | **契约保留**：有 API，**不承诺**任意矢量脏区=局部重画；若做增量，需可测分层/路径说明 |
| **G2.b blit-only 路径** | 仅 `DrawGPUTexture*`（无矢量）时 **`LoadOpLoad` + scissor** | 控件层缓存 RT / 分层合成时有效 | 同上 · 对齐 Chrome/Flutter 分层实践 | 单测/示例证明：纯 blit 帧 damage 只更新脏区；混矢量则走 G2.a |
| **G2.c OS Present damage** | `Surface.PresentWithDamage` **忽略 rect**（wgpu-native 不支持） | 无 OS 多矩形 present 省电收益 | `gpu/webgpu/surface.go` | 后端支持后再接线；此前文档与实现一致（忽略）即可 |

**产品预期（写进对外/控件约定）：**

- 引擎 **提供** damage API + scissor  
- **不承诺**「任意矢量脏区 = 只重画脏矩形像素」  
- 稳 60fps 依赖：**轻脏 UI / 分层 RT / 少全屏滤镜**；重全帧矢量 **不保证** 60  

**相关矩阵行：** S.09（API ✅，效率有界）

---

### G3 — 重层 + 滤镜 + 多 RT 稳定性（lifecycle / VRAM）

| 子项 | 现状（代码） | 影响 | 证据 | 修好标准 |
|------|--------------|------|------|----------|
| **G3.a 多离屏 / filter 图** | API 齐；重场景预算与 soak **需持续绿** | Modal / Drawer / 毛玻璃 / 多路由 RT | `examples/api_coverage_app` · particle · `scripts/run_mem_guard.sh` | 重场景 mem guard 持续绿；无 RSS/VRAM 斜率爆炸 |
| **G3.b Device lost / recover** | sticky lost · AutoRecover · `ForceRecoverHealthy` · Context 注册表 abandon · CB/pool/pipeline 回收 **已落地** | 遮挡/TDR 后 OOM、双 Device 堆 | [`GPU_修复_device_lost.md`](./GPU_修复_device_lost.md) | force-lost + recover 路径矩阵绿；恢复后无双 Device / 泄漏爬升 |
| **G3.c Surface lifecycle** | tier **Normal / Purge / Recreate** 已实现；**`auto` 默认 = Purge**（含 dGPU）；仅 `GPUI_LIFECYCLE=normal` → Normal；`NoteTextureOOM` → 升 **Recreate** | 最小化/恢复跨硬件显存与恢复行为 | [`SURFACE_LIFECYCLE_SKIA_FLUTTER.md`](./SURFACE_LIFECYCLE_SKIA_FLUTTER.md) · `render/gpu/lifecycle_policy.go` · `exboot` | lifecycle matrix 绿；OOM 后自适应 Recreate 可测 |
| **G3.d VRAM 基线** | 独显 Vulkan 设备基线偏高（本机约 **300MiB** 级）；可 low-power / adapter 策略 | 弱显存机 | [`VRAM_BUDGET.md`](./VRAM_BUDGET.md) | 预算文档与实测同量级；策略开关行为与文档一致 |

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
| G1 | 本文 · 矩阵 X.* | `render/text/`（`font_parser_own.go` · `gsub.go` · `gpos.go` · `glyf_parser.go`） |
| G2 | 本文 · 矩阵 S.09 | `render/context.go` · `gpu/webgpu/surface.go` |
| G3 | SURFACE · device_lost · VRAM · MEM_LEAK_* | `render/gpu/lifecycle_policy.go` · `context_gpu_registry.go` · `gpu/webgpu/swapchain.go` |
| 原则 | `GPU_FIRST_ROUTING` | ensureGPU / fallback 观测 |
| 优化执行 | `PERF_ENGINE_FORWARD` · `CODE_CONVERGENCE` | 热路径改动任务卡 |

---

## 6. 修订

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-21 | 1.0 | 首版：从源码与 antd 引擎职责评估收敛 |
| 2026-07-21 | 1.1 | 源码对照：GSUB/GPOS 类型边界；lifecycle auto=Purge；去掉错误 G4 |
| 2026-07-21 | 1.2 | 可执行化：每子项补「修好标准」；用法/落档流程；**修正 G3.c**（auto 默认 Purge，含 dGPU，非「非离散=Normal」）；GPOS 注明 Type 9 extension |

---

## 7. 源码对照检查（多轮）

| 轮次 | 检查项 | 结果 |
|------|--------|------|
| R1 | G1.b `font_parser_own.go` CFF 注释；`glyf_parser.go` 无 glyf 报错 | ✅ |
| R1 | G1.c `gsub.go` 实现 1–4、7；`gpos.go` 实现 1–2、9；无 5/6/8 与 GPOS 3–8 | ✅ |
| R1 | G2.a `FlushGPUWithViewDamage`：矢量 MSAA → LoadOpClear | ✅ |
| R1 | G2.c `PresentWithDamage` 忽略 rect | ✅ |
| R2 | G3 `NoteTextureOOM` / `ForceRecoverHealthy` / registry abandon | ✅ |
| R2 | lifecycle **`auto`→Purge（含 dGPU）**；仅 env `normal`→Normal；OOM→Recreate | ✅（1.2 纠正表文案） |
| R3 | 活文档路径无悬空 | ✅ |
| R4 | SurfaceHost：Purge/Recreate 才 Unconfigure+DropGPU | ✅（见 SURFACE 文） |
| R5 | VRAM adapter 策略文档链存在 | ✅ |
| R6 | 2026-07-21 再扫：上文符号仍在；G3.c 与 `ResolveSurfaceLifecycle` 一致 | ✅ |
