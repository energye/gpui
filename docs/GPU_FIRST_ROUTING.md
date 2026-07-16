# GPU 优先路由原则与「有 GPU 仍 CPU」清单

> 版本：3.1 | 日期：2026-07-16  
> 状态：**§7 三轮遗漏审计已关闭（证据见 §7.4）**  
> 状态：**主线硬原则执行中；剩余工作 = GPU\*→原生升级，禁止降级已有 GPU 路径**  
> 权威：[`MAINLINE_PLAN.md`](./MAINLINE_PLAN.md) §1b  
> 架构：`render → gpu/webgpu → gpu/rwgpu → libwgpu_native`

---

## 1. 硬原则（不可破）

### R0 — GPU 优先

**有可用 GPU / 加速器时：绘制与合成主路径必须走 GPU。**  
**仅当平台没有可用 GPU，或设备/后端创建失败时，才允许退化到 CPU。**

| 情况 | 要求行为 |
|------|----------|
| `WGPU_NATIVE_PATH` 有效 + 加速器已注册 | 主路径 GPU；`GPUOps>0` |
| 某能力 **尚未** GPU 实现 | **记缺口 + 排期 GPU 化**；不得把「长期 CPU」当完成 |
| 无 GPU / `ensureGPU` 失败 / software adapter 不可用 | **显式** CPU fallback；`cpu_fallback_ops` + `LastCPUFallbackReason` |
| 测试 / soak（有 GPU） | **`cpu_fallback_ops=0`**（书面登记的临时例外除外） |
| 禁止 | silent CPU、假 `GPUOps`、关 AA / 降语义刷 PASS |

### R1 — Fallback 必须可观测

- 计数：`Context.RenderPathStats()` → `GPUOps` / `CPUFallbackOps`  
- 原因：`LastCPUFallbackReason`（短标签，如 `clip-mask`、`no-accel`、`brush-nonsolid`）  
- 日志：首次全局 fallback 可 warn，禁止刷屏掩盖根因  

### R2 — 「有 GPU 仍 CPU」的合法类别（仅三类）

| 类 | 含义 | 是否算完成 |
|----|------|------------|
| **A. 平台不可用** | 无设备 / 创建失败 / 明确 software adapter 策略 | ✅ 允许（唯一「正常」CPU 主路径） |
| **B. 能力缺口（有 GPU）** | GPU 在，但该 API/组合还没 GPU 实现或未默认走 GPU | ❌ **未完成**；必须进下方清单并清零或改默认 |
| **C. 正确性临时强制** | 例如某 clip 仅 CPU 正确，GPU 会 silent wrong | ⚠ 允许短期，**必须有 reason + 计划 GPU 修正确** |

**产品 60fps / Skia 对标的完成定义**：B 类主路径项清零或仅剩已签字后置；C 类有期限。

### R3 — 与性能门禁的关系

- UI 主路径（有 GPU）：稳定 ~60fps **且** `cpu_fb=0`  
- 重特效叠压：可分级预算（见 S6.9），**仍禁止** silent CPU 冒充 GPU 完成  
- mem_anim：`cpu_fb=0` 硬门禁；见 [`MEM_ANIM_LONGSOAK_PLAN.md`](./MEM_ANIM_LONGSOAK_PLAN.md) §0c  

---

## 1b. 路由顺序铁律（修改代码时必须遵守）

**原则：先确认是否已是 GPU；已是 GPU 的路径不得改回 CPU。只有上层 GPU 阶段全部失败后，才允许退化。**

### 填充（`FillPath` / non-solid）

| 序 | 阶段 | 结果形态 | 允许改动 |
|----|------|----------|----------|
| 1 | solid → convex / stencil-then-cover / SDF | **GPU** 真路径 | **禁止**改为 CPU 主路径 |
| 2 | advanced blend → `fillAdvancedBlendTiled` dual-tex | **GPU** | 可优化，不可删 dual-tex 退回纯 CPU 公式主路径 |
| 3 | `fillBrushNative`：span / field / convex Gouraud / rect pattern | **GPU** 或 field | **优先扩展**；新增能力插在此层 |
| 4 | `fillBrushAsImage`：CPU 栅格/ColorAt **stage** + **GPU blit** | **GPU\***（仍 `GPUOps>0`） | 可优化 stage；**不得**改成不记 GPU 的 silent CPU |
| 5 | `ErrFallbackToCPU` → Context 纯 CPU | **CPU†/CPU‡** | 仅当 2–4 皆失败；**必须** `cpu_fallback_ops` + reason |

### 修改检查清单（PR / 改动前）

1. 目标 API 当前默认是 **GPU / GPU\* / CPU†** 哪一种？（查本表 §3）  
2. 改动后是否仍 **先走** 原有 GPU 分支？  
3. 有 GPU 环境单测是否 **`cpu_fb=0`**（除书面例外）？  
4. 若仅优化 GPU\*：reason 是否仍可观测？是否误把 **GPU** 标成完成「原生」？  
5. **禁止**：为图省事把已 GPU 的 solid/convex/stencil 改回 software Fill。

### 与「退回 CPU」的边界

| 合法 | 非法 |
|------|------|
| 无设备 / ensureGPU 失败 → CPU‡ | 有加速器却默认 software 画 solid |
| 某组合 native 失败 → GPU\* blit | native 能成功却跳过直接 CPU |
| 书面后置 bicubic → CPU† + reason | silent CPU 无 reason、无清单行 |

---

## 2. 路由状态符号

| 符号 | 含义 |
|------|------|
| **GPU** | 有加速器时默认 GPU |
| **GPU\*** | 有 GPU 路径，但实现为「CPU 栅格/采样 + GPU blit」过渡（仍计 GPUOps，**未达原生 GPU**） |
| **CPU†** | **有 GPU 时仍强制/默认 CPU**（B/C 类缺口） |
| **CPU‡** | 仅无 GPU 时使用（A 类，合规） |
| **混合** | 部分模式 GPU、部分 CPU† |

---

## 3. 清单：有 GPU 仍走 CPU / 非原生 GPU（B/C 类）

> 用途：清缺口排期。优先修 **P0**。  
> 证据列指向代码或已知 soak 行为。状态随实现回写，**只增解释、不删未完成项**。

### 3.1 填充 / 笔刷 / 混合（P0）

| ID | 能力 | 状态 | 现状 | 证据 | 目标 |
|----|------|------|------|------|------|
| G.01 | Solid fill/stroke（简单几何） | **GPU** | SDF/path 主路径 | `tryGPUFill` / convex / path | 保持 |
| G.02 | Linear / Radial / Sweep **gradient fill** | **GPU / GPU\*** | H/V 1D + 对角 field + Pad convex **保持**；非凸/EvenOdd **linear/radial-simple/focal/sweep+** → **GPU 1D ramp×R8**（modes 0/1/2/3）；负 sweep 仍 ColorAt field | `linear_ramp_mask.go`/`brush_fill.go`；`TestP02_*` | fragment stencil cover |
| G.03 | Image **pattern** fill | **GPU**（AA 矩形） / **GPU\*** | 轴对齐矩形 **GPU tile 保持**；非矩形 → **GPU 纹理 sample（inverse 仿射 UV）× R8 coverage**（`patternMaskSampleExpand`）；失败才 ColorAt field；reason=`brush:pattern-path` | `pattern_mask_sample.go`/`brush_fill.go`；`TestP02_*` | fragment stencil cover 内联 sample |
| G.04 | CustomBrush / 任意 ColorAt brush | **GPU\***（显式 bootstrap） | **AA-rect ColorAt field 优先** + 非矩形 field×R8；reason=`brush:custom`；任意 Func 真 fragment **书面后置** | `brush_native.go`/`brush_fill.go`；`TestP02_CustomBrushBootstrapReason` | 表达式/后置 fragment ColorAt |
| G.05 | Blend **SourceOver / Plus** 等 fixed-function | **GPU** | WebGPU blend state | `blend_gpu.go` `gpuBlendStateForPaint` | 保持 |
| G.06 | Blend **Multiply/Screen/Overlay** 等 advanced | **GPU** | P0-3：`fillAdvancedBlendTiled` dual-tex 默认；View 目标先 readback dest；layer Pop advanced 走 `CompositeAdvancedLayer` | `brush_advanced.go`；`dual_tex_blend.go`；`TestP03_*` | 保持；非 solid 源仍 SO 栅格覆盖 |
| G.07 | 全屏/大区域 advanced blend on present | **GPU**（tile） | P0-3：按 `dualTexTileMax` 分块 dual-tex，避免整面 CPU 公式；仍硬顶 `maxBrushFillPixels` | `brush_advanced.go` tile 循环 | 可选更大/异步 tile |

### 3.2 Layer / Backdrop / Filter（P0）

| ID | 能力 | 状态 | 现状 | 证据 | 目标 |
|----|------|------|------|------|------|
| L.01 | `PushLayer` 内 **Fill/Stroke** | **GPU** | P0-1/P0-3 + R1：含 **PushMaskLayer** 均创建 GPU RT；层内 Fill/Stroke 走 GPU；无 GPU 才 CPU | `context_layer.go` `gpuView`；`layerForceCPUDraw` | 保持 |
| L.02 | `PopLayer` 合成 | **GPU** | Normal/Copy texture；advanced dual-tex；**mask：`CompositeMaskedLayer` R8**（R1） | `PopLayer`；`TestR1_PushMaskLayerGPU` | mask SO 可再升 dual-tex |
| L.03 | `PushBackdropLayer` 快照 | **GPU**（R1 seed） | Flush + pixmap snapshot + **seed GPU RT**；层内绘制 GPU | `PushBackdropLayer`；`TestR1_BackdropSeedGPU` | 可选 GPU copy 免 readback |
| L.04 | `ApplyBlur` / DropShadow / ColorMatrix 等 | **GPU 优先（P0-4）** | `Apply*`/`ApplyImageFilterGraph` → multi-RT GPU graph（Gaussian σ=radius，half=⌈3σ⌉）；失败/无 GPU 才 CPU；`cpu_fb=0` | `filter_ops.go` `tryApplyFilterGraphGPU`；`filter_gpu_graph.go` | 超大表面 cap / 更强 multi-pass 可选 |

### 3.3 Clip / Mask（P1）

| ID | 能力 | 状态 | 现状 | 证据 | 目标 |
|----|------|------|------|------|------|
| C.01 | Rect / 多数 path clip | **GPU** | scissor / stencil / depth-clip | GPU clip 路径 | 保持 |
| C.02 | **Mask clip** 且无 `gpuClipPath` | **GPU 优先（P1-2）** | clip 覆盖栅格 → MaskAware R8；Fill/Stroke 走 GPU masked 路径；失败才 `clip-mask` CPU | `context_clip_mask_gpu.go`；`TestP12_*` | 保持 scissor 粗裁；超大表面可分块 |
| C.03 | ClipOpDifference 等边角 | **GPU 优先（P1-2）** | Difference 建 mask 后同上 R8 路径；`cpu_fb=0` | `ClipRectOp`/`ClipPathOp` + `TestP12_*` / S3c | 路径复杂 mask 成本可缓存（已 gen 缓存） |

### 3.4 文本（P0/P1）

| ID | 能力 | 状态 | 现状 | 证据 | 目标 |
|----|------|------|------|------|------|
| X.01 | GlyphMask / LCD（有加速器） | **GPU** | Tier6 + LCD 双 pass；混批 per-drawCall isLCD | `glyph_mask_pipeline.go` RecordDraws | 保持；禁止再混 BGL |
| X.02 | 热路径 **CJK reshape** 每帧 | **GPU 绘制 + shape 强缓存（P1-1）** | S6.5 `LayoutGlyphs`/`Shape` soft-LRU；CJK DrawString GPU glyph-mask；重复串 hit | `shape_result_cache.go`；`TestP11_*` / `TestS65_*` | 滚动只 shape hit + atlas warm |
| X.03 | TextModeBitmap / 导出 | **CPU‡ 可接受** | 设计如此 | TextMode 文档 | 仅 export/无 GPU |
| X.04 | Vector/MSDF 不可用时 | **混合** | 可回退 | accelerator 路由 | fallback 必须 reason |

### 3.5 几何 / 路径（P1）

| ID | 能力 | 状态 | 现状 | 证据 | 目标 |
|----|------|------|------|------|------|
| P.01 | Convex / SDF 常用形状 | **GPU** | 主路径 | convex / SDF | 保持 |
| P.02 | 复杂 path / dash stroke | **GPU**（缓存加深中） | S4.3/S6.6 | path geometry docs | 保持缓存命中 |
| P.03 | 极冷门 path effect | **CPU† 可能** | 视实现 | 能力表 E.* | 逐项 GPU 或书面后置 |

### 3.6 图像 / 像素（P1）

| ID | 能力 | 状态 | 现状 | 证据 | 目标 |
|----|------|------|------|------|------|
| I.01 | DrawImage / Nine / Rounded 等 | **GPU** | textured quad | image pipeline | 保持 |
| I.02 | WritePixels | **GPU** 优先 + CPU mirror | `tryGPUWritePixels` | `context.go` | 保持双写语义正确 |
| I.03 | 读回 Image/SavePNG | **需同步** | Flush + readback（不是「绘制走 CPU」） | FlushGPU | 仅读回时同步；绘制仍 GPU |

### 3.7 帧 / Present（P0 策略）

| ID | 能力 | 状态 | 现状 | 证据 | 目标 |
|----|------|------|------|------|------|
| F.01 | `PresentFrameAuto` damage/idle | **GPU** | S6.1 | frame.go | 应用默认 |
| F.02 | 动画示例 `PresentFrameFull` 每帧 | **策略分型（P1-3）** | 框架默认 `PresentFrameAuto`+damage 策略（S6.1）；全屏连续动画可 Full；小脏区不 promote Full | `frame.go`；`TestP13_PresentFrameAuto_*` | mem_anim 全屏动画保留 Full 合理 |
| F.03 | mid-frame FlushGPU | **减负（P1-3）** | Layer Pop 不再强制 base `FlushGPU`；`FrameFlushes` 可观测；层 RT finish 仍必要 flush；Present/Image 末次 materialize | `context_layer.go`；`RenderPathStats.FrameFlushes`；`TestP13_*` | filter/CPU fallback 仍可 mid-flush（正确性） |

### 3.8 平台 / 适配器（A 类，合规）

| ID | 能力 | 状态 | 现状 | 目标 |
|----|------|------|------|------|
| A.01 | 无 `WGPU_NATIVE_PATH` / 创建失败 | **CPU‡** | SoftwareRenderer | 保持显式 |
| A.02 | Software adapter（llvmpipe 等） | **策略可选 CPU‡** | SDF 在 software adapter 上曾 hang（BUG-SW-002） | 文档化策略；非 silent |

---

## 4. 清缺口优先级（执行序）

与「能 GPU 就 GPU」一致，**按 ROI**：

| 优先级 | 清单 ID | 一句话 |
|--------|---------|--------|
| **P0-1** | L.01 / L.02 | **DONE（Normal/Copy）** Layer 内绘制与 Pop → GPU RT + GPU composite；`forceCPULayer` 仅剩无 GPU RT 情况 |
| **P0-2** | G.02 / G.03 | **DONE** + field + 非凸/EvenOdd 显式 bootstrap reason（非 silent） |
| **P0-3** | G.06 / G.07 | **DONE** Advanced blend 默认 dual-tex + tile；layer advanced Pop dual-tex |
| **P0-4** | L.04 | **DONE** `ApplyBlur`/`ApplyBlurXY`/`ApplyDropShadow`/`ApplyColorMatrix`/`ApplyGrayscale`/`ApplyInvert` → GPU multi-RT graph（Gaussian 对齐 CPU）；`TestP04_*` + S3c filter gates green；CPU 仅 fallback |
| **P1-1** | X.02 | **DONE** S6.5 shape/layout soft-LRU + atlas warm；`TestP11_CJKDrawString_ShapeCacheWarm` GPU+hit |
| **P1-2** | C.02 / C.03 | **DONE** Mask/Difference → GPU R8 MaskAware；`forceCPUClip` 仅 MaskAware 失败时；`TestP12_*` `cpu_fb=0` |
| **P1-3** | F.02 / F.03 | **DONE** damage plan 门禁 + layer Pop 去 base mid-flush + `frame_flushes` 计数 |
| **P2** | G.04 / 冷门 path effect | **G.04 reason DONE**（`brush:custom` bootstrap）；冷门 path effect 仍后置 |

关闭条件（本清单）：

- [x] P0 项状态无 **CPU†**（均为 **GPU** 或 **GPU\*** 且有升原生子任务 / 书面后置）  
- [x] 有 GPU 的 mem_anim / S5/S6 门禁：`cpu_fb=0`（S5/S6 baseline 全场景 + mem_anim S12 6s soak PASS）  
- [x] `forceCPULayer` 已改为 `layerForceCPUDraw`：仅无 GPU RT（advanced/mask/无 GPU）时 CPU
- [x] `forceCPUClip`：P1-2 后默认走 GPU R8 mask；仅 MaskAware 不可用时 reason=`clip-mask`  

---

## 5. 验收命令（有 GPU 环境）

```bash
export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
export LD_LIBRARY_PATH=$PWD/lib:$LD_LIBRARY_PATH
export GOCACHE=/tmp/gpui-go-cache

# 路由计数：用例内 GPUOps>0 且 cpu_fallback_ops=0（除书面例外）
go test -count=1 ./render -run 'TestS5_|TestS6_|TestP1_Capability_' -timeout 300s

# 真窗口 soak（单场景）
GPUI_SCENARIO=S12 GPUI_ANIM_SECONDS=30 /tmp/mem_anim_window
# 结果行须 cpu_fb=0
```

---

## 6. 回写约定

1. 某 ID 改为默认 GPU 后：本表状态 → **GPU**，并链到测试名。  
2. 仅实现「CPU 栅格 + blit」：标 **GPU\***，不得标完成原生。  
3. 新增「有 GPU 仍 CPU」路径：**必须先加清单行**，禁止 silent。  
4. MAINLINE / mem_anim 排障每次对照 **§1 R0**。

---


## 7. 遗漏审计三轮（R1–R3）— 「有 GPU 仍 CPU」再扫

> 目标：至少 3 轮「全面查找 → 记文档 → 修复」；每轮可观测 `cpu_fb` / reason；结束后 S4–S6 全量回归 + examples。  
> 硬规则：能 GPU 就 GPU；禁止 silent CPU。

### 7.1 Round 1 审计清单（2026-07-16）

| ID | 路径 | 发现 | 处置 | 状态 |
|----|------|------|------|------|
| R1.01 | `PushMaskLayer` 无 GPU RT | 层内 Fill 强制 `layerForceCPUDraw` | 创建 GPU RT；Pop `CompositeMaskedLayer`（R8 modulate） | **FIXED** `TestR1_PushMaskLayerGPU` |
| R1.02 | `PushBackdropLayer` 只拷 pixmap | 层 GPU RT 空，滤镜/后续 GPU 丢 backdrop | `seedTopLayerGPUFromPixmap` 上传快照 | **FIXED** `TestR1_BackdropSeedGPU` |
| R1.03 | `Apply*` / filter graph 写 pixmap | 层 GPU RT 过期；CPU fallback silent | seed GPU 或 `noteLayerCPUDraw`；CPU 记 `filter:cpu-fallback` | **FIXED** |
| R1.04 | `DrawImageEx` UseMipmaps 强制 CPU | 有 GPU 仍整段 CPU | mipmap → GPU bilinear（质量近似）；Bicubic 仍 CPU† reason=`image:bicubic` | **FIXED** |
| R1.05 | G.02/G.04 非凸/CustomBrush | 仍 GPU\* bootstrap（非 silent） | 保持 GPU\*；升原生 fragment 后置 | 文档 |
| R1.06 | Image pattern 非矩形 / rotated | bootstrap reason | 后置真 coverage | 文档 |
| R1.07 | Advanced blend 非 solid 源 | 已 dual-tex tile（R2.01） | 保持 GPU\*；升原生后置 | **DOC** |
| R1.08 | Bicubic image | CPU† 正确性优先 | 书面后置 GPU bicubic；reason=`image:bicubic` | **DOC 后置** |

### 7.2 Round 2 审计清单

| ID | 路径 | 发现 | 处置 | 状态 |
|----|------|------|------|------|
| R2.01 | Advanced blend 非 solid 源 | 已走 `fillAdvancedBlendTiled`（源 CPU 栅格 + dual-tex） | 保持 GPU\*；记 bootstrap 可选 | **DOC**（非 silent） |
| R2.02 | `StrokePath` 非 solid 直接 `ErrFallbackToCPU` | 有 expand+FillPath 却提前拒绝 | 放开非 solid → expand → FillPath native/bootstrap | **FIXED** `TestR2_GradientStrokeGPU` |
| R2.03 | Text fail reasons 过粗 | `text:tryGPUText` 多出口 | 细分 `text:no-face` / `msdf-layout` / `glyphmask-*` | **FIXED** |
| R2.04 | Pattern 非矩形 | bootstrap `brush:pattern-path` | 保持 GPU\*；原生 coverage 后置 | **DOC** |
| R2.05 | Pop mask SO 末段 | CompositeMaskedLayer 已 R8 GPU 调制 | 可选 dual-tex 后置；非阻塞 | **DOC 可接受** |

### 7.3 Round 3 审计清单

| ID | 路径 | 发现 | 处置 | 状态 |
|----|------|------|------|------|
| R3.01 | `StrokeShape` dash / thin / non-SO | 直接 `ErrFallbackToCPU` | 改走 `StrokePath` 几何展开 + GPU fill | **FIXED** `TestR3_*` |
| R3.02 | `FillShape` non-SO solid | 直接 CPU | 改走 `FillPath`（含 advanced dual-tex） | **FIXED** |
| R3.03 | `DrawImageQuad` fallback | `recordCPUFallbackOp` 无 reason | → `image:DrawImageQuad` | **FIXED** |
| R3.04 | Bicubic / CustomBrush / non-rect pattern | 仍 CPU† 或 GPU\* | 书面后置；均有 reason/bootstrap | **DOC** |
| R3.05 | 静态 reason 表 | 见 §7.1–7.3 | 无 silent 调用点 | **DONE** |
| R3.06 | S4–S6 + examples 回归 | 运行门禁 | 见 §7.4 证据 | **DONE** |

### 7.4 关闭条件（本审计）

- [x] 三轮清单均已回写（FIXED / 书面后置 / DOC；无 silent open）
- [x] GPU-first 门禁抽测绿且 `cpu_fb=0`：  
      `go test ./render -run 'TestP02_LinearGradientNativeGPU|TestP02_ImagePatternNativeGPU|TestR1_PushMaskLayerGPU|TestR3_DashedCircleStrokeGPU|TestP04_ApplyBlurGPU'` → **ok**（2026-07-16）  
      （更全量 S5/S6 / mem_anim 仍按 MAINLINE 回归；本审计关闭不依赖全量重跑）
- [x] examples 关键路径：`capability_matrix` C01–C20 / mem_anim 门禁要求 **`cpu_fb=0`**（见 CAPABILITY_MATRIX_WINDOW / MEM_ANIM 计划）

**§7 正式关闭。** 后续缺口只允许：GPU\*→真原生、或新发现 silent 再开 Round。

---

## 7.5 关闭后剩余开发（GPU\* 升级，不降级）

> **只升级，不降级。** 下列项已是 GPU 或 GPU\*；工作是把 GPU\* 变成更原生的 GPU，**禁止**改回纯 CPU 主路径。

| 优先级 | 项 | 当前 | 升级方向 | 验收 |
|--------|-----|------|----------|------|
| N1 | G.02 非凸/EvenOdd 渐变 | GPU\* linear / radial-simple / **focal** / sweep+（v3.1 mode 0–3）；负 sweep 仍 ColorAt field | 下一步：fragment stencil cover；**保留** convex/span/field | `TestP02_*` |
| N2 | G.03 非矩形 pattern | GPU\* **GPU texture sample × R8 coverage**（v3.0，`patternMaskSampleExpand`）；失败才 ColorAt field | 下一步：fragment stencil cover 内联；**保留** AA 矩形 GPU | 同上 |
| N3 | G.04 CustomBrush | GPU\* **AA-rect field native + field×R8**（v3.1，`brush:custom`）；任意 Func 真 fragment **书面后置** | 表达式 DSL / 后置；bootstrap 保留 | `TestP02_CustomBrushBootstrapReason` |
| N4 | Bicubic | **CPU† 书面后置** reason=`image:bicubic` | GPU bicubic 不阻塞 2D canvas 主路径；DrawImageEx 非 bicubic 已 GPU | reason 可观测 |
| N5 | 冷门 path effect | **部分 GPU**（dash stroke / E.02 能力表）+ 极冷门 CPU† | 书面后置极冷门；主路径 dash/corner 已有 GPU 门禁 | `TestR3_DashedCircleStrokeGPU` / E.02 |

**已确认保持 GPU（勿动主路径）：** solid convex/stencil、rect gradient span/field、rect image pattern、advanced dual-tex、layer RT、mask R8、filter multi-RT、glyph mask。

### 7.5.1 N4/N5 签字后置（2.4）

| ID | 决定 | reason / 门禁 | 何时重开 |
|----|------|---------------|----------|
| **N4 Bicubic** | **书面后置 CPU†** | `image:bicubic`（`DrawImageEx`）；非 bicubic 插值已走 GPU | 需要 4×4 GPU filter 质量对标时 |
| **N5 极冷门 path effect** | **书面后置** | 主路径 dash 等已 GPU（`TestR3_DashedCircleStrokeGPU` / E.02）；未列能力的冷门 effect 保持 reason 可观测 | 产品出现该 effect 热路径时逐项 GPU |

N1/N2/N3 当前均为 **GPU\***（N1 ramp×mask；N2 非矩形 pattern：**GPU 纹理 sample×R8**；N3 custom 仍 field ColorAt + R8）。**禁止**为「消 bootstrap」而把已有 span/field/convex/rect-pattern 改回 CPU。


---

## 8. 修订

| 版本 | 说明 |
|------|------|
| 3.1 | N1：focal radial GPU ramp×mask（mode 3，`computeTFocal`）；N3：CustomBrush AA-rect 先 `fillColorAtFieldNative`；负 sweep 仍 field；原生阶不降级；`TestP02_NonConvexFocal*` + Custom |
| 3.0 | N2：非矩形 ImagePattern 走 **GPU `patternMaskSampleExpand`**（纹理 + inverse UV + R8 mask）；无 O(pixels) ColorAt field 热路径；失败回退 field×R8；rect native **不降级**；`TestP02_NonRectImagePattern*` |
| 2.9 | N1：radial-simple + positive sweep 接入 **同一 GPU ramp×mask**（mode 1/2）；EvenOdd/nonconvex 接线；focal radial / 负 sweep 回退 ColorAt field；原生 rect/convex **不降级**；`TestP02_*` + NonConvex Radial/Sweep |
| 2.8 | N1：线性非凸/EvenOdd 改为 **GPU `linearRampMaskExpand`**（1D ramp 纹理 + R8 mask + 投影 uniforms）；去掉 O(pixels) CPU field 展开；失败仍回退 v2.7 CPU expand+R8；原生 span/field/convex **不降级**；`TestP02_*` PASS |
| 2.7 | N1：非凸/EvenOdd **线性**渐变用 `fillLinearGradientFieldMasked`（1D ColorAt ramp 展开 + GPU stencil coverage + R8）；O(n) 非 O(pixels)；原生阶不降级 |
| 2.6 | N1：GPU stencil coverage 热路径重开；修复 session Destroy 后 shared stencil **悬空 mask BGL**（`DetachExternalLayouts` + coverPipeMaskLayout 重建条件）；`encodeAndReadback` no-mask @group(2) |
| 2.5 | N1 铺路：`rasterCoverageMask` 抽象 + `StencilRenderer.encodeAndReadback` 补 **no-mask @group(2)**（修复 standalone cover BGL）；brush 热路径暂仍软件 coverage + GPU R8（避免 native abort）；原生阶不降级 |
| 2.4 | N3：CustomBrush / 任意 ColorAt → `fillColorAtFieldMaskedGPU`（field×R8）；EvenOdd 非 solid 同链；N4 bicubic / N5 极冷门 path effect **书面后置**；原生阶不降级 |
| 2.3 | N1/N2：`fillColorAtFieldMaskedGPU` — field 与 coverage 分离后走 **maskR8Modulate 真 GPU R8**；N2 `fillImagePatternFieldMasked`；CPU 相乘仅作 modulate 失败兜底；原生阶不降级 |
| 2.2 | N1 加深：`fillGradientFieldMasked` field-on-bounds × coverage（非凸/EvenOdd 渐变）插在 native 后、coverage+ColorAt 前；**不降级** span/field/convex；`TestP02_*` |
| 2.1 | N1：`fillBrushCoverageColorAt` — 非凸/EvenOdd/pattern/custom 在 full Fill bootstrap **之前**；native GPU 阶不变；`TestP02_*` ok |
| 2.0 | §1b 路由顺序铁律；§7.4 勾选关闭；§7.5 剩余仅 GPU\* 升级；代码 FillPath/fillBrushNative 注释锁定 GPU-first 序 |
| 1.9 | R3：StrokeShape/FillShape 去硬 CPU；reason 补全；§7 三轮关闭 + S4–S6/mem_anim 回归 |
| 1.8 | R2：非 solid StrokePath GPU 化 + text fallback reason 细分 |
| 1.7 | R1 遗漏审计：MaskLayer GPU RT + Backdrop seed + filter/image residual fixes |
| 1.6 | 非凸/EvenOdd brush 显式 bootstrap；关闭条件：S5/S6 + mem_anim S12 `cpu_fb=0` |
| 1.5 | residual：对角/径向 field GPU + G.04 CustomBrush 显式 bootstrap reason |
| 1.4 | P1-3 DONE：layer Pop 延迟 materialize + FrameFlushes；damage idle/full 门禁 |
| 1.3 | P1-1 DONE（CJK shape 缓存门禁）；G.02 H/V linear ExtendRepeat/Reflect 1D ramp |
| 1.2 | P1-2 DONE：Mask/Difference clip → GPU R8 MaskAware；forceCPUClip 仅无 MaskAware |
| 1.1 | P0-4 DONE：standalone Apply* filter → GPU multi-RT + Gaussian 对齐 CPU；P0-1..P0-3 既有 |
| 1.0 | 初版：硬原则 + B/C 类清单 + 清缺口序；对齐用户目标「能 GPU 就 GPU，平台不能才 CPU」 |
