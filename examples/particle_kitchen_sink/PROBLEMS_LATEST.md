# Latest problem-suite snapshot (machine-local)

> 生成于本机 suite 跑分，**失败即信号**，不是刷分目标。

## 如何复现

```bash
# 全面挖问题（含 dig 墙）
GPUI_PROBLEM_CAP=critical scripts/run_problem_suite.sh
# 仅 dig
GPUI_PKS_FILTER=dig GPUI_ANIM_SECONDS=5 scripts/run_pks_matrix.sh
```

产物默认：`/tmp/problem_suite/PROBLEMS.md`（本快照对应 `/tmp/problem_suite3/`）。

## 本快照摘要（suite3, critical cap）

- evidence 6/6 PASS（pixel/stage/empty/flicker 装绿墙）
- **total_failures ≈ 19**（core+dig+combo+mem）
- capability critical C01/C07/C11 PASS
- `P_HIGH_N` PASS + 多 combo FAIL → 高级路径瓶颈，非粒子密度

### 新 dig 墙抓到的真问题

| probe | signal |
|-------|--------|
| P_GRAD_RT | `fps_hitch_ratio≈0.32`（渐变 RT export 尖刺） |
| P_FPS_JIT | `fps_jitter_high span≈65`（稳态抖动） |
| P_COMBO_UI | `fps≈25`（clip×grad×filter×blend 合成） |
| P_FILTER_FLICKER | `fps_hitch_ratio≈0.49`（滤镜+交替清屏） |

### 持续暴露的压力问题

| probe | signal |
|-------|--------|
| P_BLEND_GLOW / P_L2 | fps≈51（未在 F1 批次重跑） |
| P_L3 / P_L4 | fps≈25–31（未在 F1 批次重跑） |

### F1 收口（`/tmp/f1_batch5b`, 2026-07-16）

**内容修复（同日续）**：`P_BLEND_LAYER`/`P_BLEND_CPU` 画面无高级混合内容 — `dualTexBlendLayersIntoDest` 未写入 dest。  
已改为 `dualTexAdvancedBlendViewsRegionSized` → out RT → blit；View-nil `FlushGPU` 也可 resolve。  
证据：`TestF1_AdvancedLayerPresentView*`、`TestP03_AdvancedLayerDualTexGPU` PASS；`/tmp/f1_blend_fix`。


Present-deferred parent stash（`prepareTarget` + scissor re-base）落地后：

| probe | status | fps_avg | cpu_avg | gate |
|-------|--------|---------|---------|------|
| P_SOLID | PASS | 57.4 | 21.5 | — |
| P_MULTI_LAYER | PASS | 57.2 | 20.5 | ≥55 fps |
| P_LAYER_BLEND | PASS | 58.1 | 31.2 | ≥55 fps |
| P_BLEND_LAYER | PASS | 58.1 | 31.5 | ≥55 fps |
| P_L1 | PASS | 58.1 | 30.1 | ≥55 fps |
| **P_BLEND_CPU** | **PASS** | **58.1** | **31.8** | **cpu &lt; 80** |

此前：`P_MULTI_LAYER`/`P_LAYER_BLEND` ≈30–38fps；`P_BLEND_CPU` cpu≈102–110。  
改动要点：`render/internal/gpu/gpu_render_context.go` — 全部 Queue\* 走 `prepareTarget`；View-nil parent→layer 时 stash 而非 mid-frame full-scene Flush；Present 路径 unstash；stash merge 重基 scissor 计数。

### Bisect 示例

- `P_ALPHA_MESH` FAIL → `ENABLE_BLEND=0` 或 `ENABLE_MESH=0` 可恢复 → 组合路径问题

详见 `COVERAGE.md` 失败模式表。


### F1 HUD text (2026-07-16 续)

**问题**：`ClipRect` + `PushLayer(advanced)` 后 HUD/FPS `DrawString` 不可见。  
**根因**：`setGPUClipRect` 在 `Queue*→prepareTarget` 之前记录 `SetClip`；首笔 layer 绘制会把**未配对的 SetClip** stash 进 parent present-stash，present 时 HUD 文本落入 stage scissor。  
**修复**：`PrepareTarget` 先于 scissor 记录；base 编码后 drop pending 再 resolve；blit-only `LoadOpLoad` 保留 scratch。  
**证据**：`TestF1_AdvancedLayerPresentView_HUDText` PASS；`P_BLEND_LAYER`/`P_BLEND_CPU` PASS。


### F1 closeout (`/tmp/f1_closeout`, 2026-07-16 夜)

**F1 核心（层批 / 高级混合 present / HUD）已收口** — 证据目录 `/tmp/f1_closeout` + `TestF1_AdvancedLayerPresentView*`。

| probe | status | fps_ema | fps_avg | cpu | note |
|-------|--------|---------|---------|-----|------|
| P_SOLID | PASS | 59.5 | 57.2 | 21 | 基线 |
| P_MULTI_LAYER | PASS | 59.2 | 57.4 | 23 | present-stash + opacity-group |
| P_LAYER_BLEND | PASS | 58.8 | 57.5 | 33 | ≥55 |
| P_BLEND_LAYER | PASS | 59.6 | 57.6 | 32 | 高级混合内容+HUD |
| P_BLEND_CPU | PASS | 58.7 | 57.6 | 32 | cpu&lt;80 |
| P_L1 | PASS | 57.5 | 57.3 | 36 | gate |
| P_BLEND_GLOW | FAIL | 70.2 | 57.2 | 49 | hitch_ratio≈0.32（glow RT ApplyBlur+export） |
| P_L2 | FAIL | 72.6 | 57.2 | 51 | 同上 |
| P_L3 | FAIL | 76.2 | 56.1 | 61 | glow+mesh+text 组合 hitch |
| P_L4 | FAIL | 59.0 | 50.5 | 95 | 高密度+trails hitch/cpu |

**F1 交付项**
1. present-stash：View-nil parent 跨 layer RT 不 mid-frame full Flush  
2. advanced resolve：`viewsRegionSized → out → blit`（非 dead `dualTexBlendLayersIntoDest`）  
3. HUD/FPS 文本：`PrepareTarget` 先于 scissor；scratch blit `LoadOpLoad`  
4. 像素门：`TestF1_AdvancedLayerPresentView_HUDText`  

**非 F1 阻塞（下一挖）**：glow 滤镜 export 尖刺（`ApplyBlur`+`ExportImageBuf` 隔帧）— 修 GPU 常驻 glow RT / 减 readback，**不得**靠降内容刷 PASS。

### 门禁顺序（2026-07-16 夜，F1 后）

底层改动后：

1. **全量单元** `./scripts/run_full_unit_tests.sh` → `tmp/full_unit/summary.txt`  
2. **内存泄漏** `./scripts/run_mem_leak_tests.sh` → `tmp/gpui_mem_leak_tests.log`（process-local RSS + OOM 硬门）  
3. 再 dig glow hitch / 性能（不得砍内容刷分）

文档：`docs/MEM_LEAK_TEST_PLAN.md` §11、`docs/MAINLINE_PLAN.md` M 节。


### Engine filter zero-readback (2026-07-17)

**问题**：P_GLOW / P_BLEND_GLOW `fps_hitch_ratio≈0.33` + long soak RSS climb — 根因是
`ApplyBlur` GPU 路径每帧 CPU upload + Map readback，再 `ExportImageBuf` 二次读回。

**引擎修复**（内容密度不降：24 samples / blur=2 / stage-relative RT）：
1. `filter_gpu_graph`：池化 A/B/hold RT、staging、uniform；多 pass **单 submit**
2. `RegisterGPUFilterGraphTexture` / `FromView`：滤镜结果 **GPU 纹理发布**，无 Map
3. `tryApplyFilterGraphGPU`：pending draws → `FlushGPUWithView` 后 GPU→GPU 滤镜；pixmap 懒 materialize
4. `Context.GPUFilterTexture` + glowRT `DrawGPUTextureWithOpacity` 零拷贝合成
5. glow 连续刷新（避免 frame%3 造成 1/3 hitch 尖刺）

**证据** `tmp/pks/filter_final/`：

| probe | status | hitch | dRSS |
|-------|--------|-------|------|
| P_GLOW | PASS | ~0.05 | ~32MB |
| P_BLEND_GLOW | PASS | ~0.06 | ~38MB |
| P_L3 | PASS | ~0.06 | ~35MB |
| P_MEM_SOAK | PASS | ~0.07 | **0** |

单测：`TestP04_ApplyBlurGPU` / F03 multi-RT / ExportImageBuf dirty — PASS。

**残留**：连续 glow 刷新时 process CPU 偏高（~60–80% 单核口径）；下一步压 BindGroup/uniform 与多 circle batch，不砍内容。


### CPU batching pass (2026-07-17 续)

在零读回滤镜已 PASS 的基础上压 process CPU（**不砍内容密度**）：

1. glow 连续刷新 + **半分辨率 downsample-blur**（视觉 footprint 不变，blur 像素 ~1/4）
2. solid/blend 圆盘：`DrawCircle+Fill` ×N → **一次 DrawMesh disc fan**（同 sample 数/颜色/半径；`PathSubmitHeavy` 仍走 path 诊断）
3. filter 路径：scratch 复用 + publish 双缓冲

**对比** `tmp/pks/cpu_opt/` base → opt3：

| probe | base cpu | opt3 cpu | hitch | status |
|-------|----------|----------|-------|--------|
| P_SOLID | 19.5 | **15.3** | 0.03 | PASS |
| P_GLOW | 64.8 | **50.1** | 0.05 | PASS |
| P_BLEND_GLOW | 74.0 | **62.4** | 0.07 | PASS |
| P_L3 | ~85 | **69.2** | 0.07 | PASS |
| P_MEM_SOAK | — | **69.4** dRSS~16MB | 0.07 | PASS |
| P_BLEND_CPU | — | **23.9** | 0.06 | PASS |

**残留**：全开 glow/L3 场景 process CPU 仍偏高（单核口径 50–70%）；主因是每帧 effect RT Flush+filter BindGroup 与主场景 submit 叠加。下一刀：effect RT 轻量 Flush / filter bind 复用 / 主路径进一步合批。

## opt6–opt7 engine CPU (2026-07-17)

### Root causes (pprof, real present path)

1. **Shared StencilRenderer thrash (~39% CPU on P_GLOW)**  
   Main window Context + glow effect Context both injected the *same* shared stencil/sdf/convex pipelines. Each session owned distinct mask/clip BGL objects → `ensurePipelines` saw `coverPipeMaskLayout` mismatch every alternating flush → `createPipelines` recompiled WGSL every frame.

2. **GPU filter graph never registered on window path**  
   `SetDeviceProvider` set `device` without calling `registerFilterGraphIfNeeded`.  
   `Flush` only called `ensureGPU` when `device == nil`, so window apps never registered ApplyBlur GPU from-view/publish. Glow fell back to **CPU blur + readback** (`applyFilterInPlace` / `blurHorizontal`).

### Engine fixes (content density unchanged)

| Change | File |
|--------|------|
| Effect surfaces (`preferSampleCount1` / `SetEffectSurface`) own sampleCount-matched shape pipelines; do **not** inject shared stencil | `render/internal/gpu/gpu_render_context.go` |
| `MarkOwnsShapePipelines()` for effect sessions | `render/internal/gpu/render_session.go` |
| `SetDeviceProvider` + `initGPU` call `registerFilterGraphIfNeeded` | `render/internal/gpu/gpu_shared.go` |
| `Flush` always `ensureGPU` when device already set (registers filter graph once) | `render/internal/gpu/gpu_render_context.go` |
| Optional `GPUI_CPUPROFILE=` for kitchen-sink pprof | `examples/particle_kitchen_sink/main.go` |

### Measured matrix (process CPU % of one core, PASS gates, no content gutting)

| probe | opt3 | opt6 (thrash fix) | opt7 (+GPU filter register) |
|-------|------|-------------------|------------------------------|
| P_SOLID | 15.3 | 15.0 | **14.8** |
| P_GLOW | 50.1 | 23.5 | **18.4** |
| P_BLEND_GLOW | 62.4 | 40.6 | **27.1** |
| P_L3 | 69.2 | 48.6 | **35.2** |
| P_MEM_SOAK | 69.4 / dRSS 16MB | 48.6 / 39MB | **35.4 / 20MB** |
| P_MEM 45s | — | — | **36.1 / dRSS 30MB PASS** |

FPS steady ≈57–58, hitch_ratio ≤0.05, cpu_fb=0, content/pixel PASS.

### opt7 pprof (P_GLOW)

- `createPipelines` thrash: **gone**
- `ApplyBlur` → `tryApplyFilterGraphGPU` (GPU path), not CPU `blurHorizontal`
- Remaining: present Flush, HUD glyph raster (~text), filter GPU submit cgo

### Still open (engine, not content cuts)

1. **HUD / glyph mask first-frame raster** still material on CPU (~20% samples with Chinese HUD) — cache warm / atlas reuse deepen.
2. **RSS soak** still +20–30MB over 30–45s on L3 density — track publish double-buffer / offscreen pool / atlas growth (not particle N).
3. Glow still ~+4pp CPU vs solid (n=1000 vs 800 + effect flush + filter submit) — acceptable direction; further: lightweight mesh-only flush, dual-kawase for small radius.
4. Aspirational multi-core / ~5% process CPU not yet; need broader path (text, present) not content gutting.

### Policy reminder

- Do not reduce particle N / glow samples / RT footprint / AA to paint metrics green.
- Prefer GPU when available; CPU only when platform cannot.
- Full-scene Skia-class quality: fix root engine costs across all probes.

## opt8 glyph table cache (2026-07-17)

### Root cause
HUD `DrawString` path re-walked the full sfnt table directory on **every glyph raster miss**:
`extractFromOwn` → `ParseGlyfContours` → `parseFontTables`, then auto-hint re-parsed contours again.
Cold-start + any miss paid double parseFontTables cost (~22% of P_GLOW samples was drawHUD).

### Engine fix (quality unchanged)
| Change | File |
|--------|------|
| `ParseGlyfContoursFromTables` + `newCachedGlyfParserFromTables` | `render/text/glyf_parser.go` |
| `ownParsedFont.GlyfContours` lazy `cachedGlyfParser` over already-parsed tables | `render/text/font_parser_own.go` |
| `extractFromOwnWithContours` — single glyf parse shared with auto-hint | `render/text/glyph_outline.go` |
| `autoHintOutline` prefers `ownParsedFont.GlyfContours` | `render/text/autohint.go` |

Atlas mask cache still primary hit path; this cuts **miss / warm-up** cost and eliminates double table walk.

### Measured (content density unchanged)

| probe | opt3 | opt7 | **opt8** |
|-------|------|------|----------|
| P_SOLID | 15.3 | 14.8 | **14.4** |
| P_GLOW | 50.1 | 18.4 | **17.0** |
| P_BLEND_GLOW | 62.4 | 27.1 | **25.3** |
| P_L3 | 69.2 | 35.2 | **33.2** |
| P_MEM_SOAK 30s | 69.4 / 16MB | 35.4 / 20MB | **34.0 / 20MB** |
| P_MEM 45s | — | 36.1 / 30MB | **34.5 / 30MB** |

### opt8 pprof (P_GLOW)
- `drawHUD` **22.5% → 12.4%**
- `parseFontTables` / double `ParseGlyfContours` **gone from top**
- Remaining: present Flush, GPU filter submit (ApplyBlur tryApplyFilterGraphGPU ~22%), mesh WriteBuffer, HUD layout/cache hits

### Still open
1. RSS steady climb ~20–30MB / 30–45s on L3 density (publish pool stable; likely Go heap / wgpu / atlas growth — dig further)
2. Filter still copies A/B → publish slot every frame (could zero-copy last pass into publish RT)
3. HUD still pays per-frame layout for changing FPS/CPU strings (digit masks cached; layoutGlyphs ~7%)
4. Process CPU aspirational ~5–15% on heavy probes not yet full Skia-class


## opt9 filter promote publish (2026-07-17)

### Change
`filter_gpu_graph.promotePoolResultToPublish`: zero-copy A/B pool RT → publish free-list swap
(no per-frame `CopyTextureToTexture` when pool size matches).

### Measured vs opt8
| probe | opt8 | **opt9** |
|-------|------|----------|
| P_SOLID | 14.4 | **13.6** |
| P_GLOW | 17.0 | **16.8** |
| P_BLEND_GLOW | 25.3 | **25.2** |
| P_L3 | 33.2 | **33.9** |
| P_MEM_SOAK 30s | 34.0 / 20MB | **34.1 / 20MB** |
| MEM45 | 34.5 / 30MB | **34.4 / 29MB** |

**Conclusion**: promote is correctness/steady-state VRAM hygiene; CPU flat (submit/WriteBuffer still dominate).

## opt10 bind-group reuse + staging + dual-tex view pass (2026-07-17)

### Engine fixes (no content cut)
1. **Image / GPU-texture bind groups**: recreate only when texture view (or nearest sampler) changes; opacity-only updates WriteBuffer the uniform and reuse BG.
2. **Vertex + uniform staging reuse** for image and GPU-texture quads (`imageVertexStaging`, `gpuTexVertexStaging`, `makeImageUniformInto`).
3. **dual-tex advanced blend**: `dualTexAdvancedBlendViewsRegionSized` takes live views (layer `srcView` + dest view) — skip per-resolve `CreateTextureView` pair; texture wrapper remains for other callers.

### Measured (content density unchanged: n/samples/blur/RT footprint)

| probe | opt8 | opt9 | **opt10** |
|-------|------|------|-----------|
| P_SOLID | 14.4 | 13.6 | **13.2** |
| P_GLOW | 17.0 | 16.8 | **16.0** |
| P_BLEND_GLOW | 25.3 | 25.2 | **25.0** |
| P_L3 | 33.2 | 33.9 | **30.3** |
| P_MEM_SOAK 30s | 34.0 / 20MB | 34.1 / 20MB | **30.8 / 20MB** |
| MEM45 | 34.5 / 30MB | 34.4 / 29MB | **31.3 / 28MB** |

All PASS; fps≈57–58; `cpu_fb=0`; hitch low.

### Remaining (engine, no content gutting)
1. **RSS steady climb** ~20–28MB / 30–45s L3 density — likely native wgpu + Go heap mix; dig with `GPUI_MEMPROFILE` + feature bisect.
2. **Flush/cgocall** still ~50%+ of samples (purego WebGPU). Need fewer submits (effect RT light flush, multi-layer single-submit, filter fused into encode).
3. **PopLayer / dual-tex** still multi-submit per advanced layer.
4. **HUD layoutGlyphs** for changing FPS strings.
5. Target: heavy probes closer to solid-class CPU (~15%) where content allows; RSS slope → flat.


## opt11 present-stash prepend + mesh scratch (2026-07-17)

### Root cause (heap profile P_MEM_SOAK 20s)
| alloc | opt10 | note |
|-------|-------|------|
| `unstashPresentPending` | **451MB** | `append(append([]T{}, stash...), pending...)` **每帧双分配**整表命令 |
| `QueueColoredMesh` | **214MB** | cap 不够时 `make([]Point, need)` 丢弃可复用 scratch |
| total alloc_space | **~1010MB** | |

inuse_space 仅 ~22MB → GC 能回收，但分配风暴抬高 CPU/RSS 压力与碎片。

### Engine fix
1. `prependSlice[T]`: 复用 `dst` capacity 的 in-place shift；仅 cap 不足时单次 alloc
2. `unstashPresentPending` 全部队列改走 `prependSlice`（stash 仍 `[:0]` 保 capacity）
3. `QueueColoredMesh` **始终**写入 grow-only `convexMeshPts` / `convexMeshVCs`

### Measured (内容密度不降)

| probe | opt10 | **opt11** |
|-------|-------|-----------|
| P_SOLID | 13.2 | 13.6 |
| P_GLOW | 16.0 | 16.4 |
| P_BLEND_GLOW | 25.0 | 25.3 |
| P_L3 | 30.3 | **30.2** |
| P_MEM_SOAK 30s | 30.8 / 20MB | **30.7 / 22MB** |
| MEM45 | 31.3 / 28MB | **31.1 / 30MB** |
| heap alloc 20s | **~1010MB** | **~343MB (−66%)** |
| unstash / QueueColoredMesh in alloc top | yes | **gone** |

CPU 与 opt10 同级（cgocall/Flush 仍主导）；**堆分配压力显著下降**（引擎级，非砍内容）。

### Still open
1. RSS 稳态斜率 ~15–30MB/45s — Go heap inuse 小，**更像 native wgpu / 映射**；继续 feature 二分 + 资源池审计
2. `layoutGlyphs` 仍是剩余 alloc 大户（变化 HUD 字符串）
3. 少 Submit：effect 轻 Flush、advanced-layer 批 submit、filter 合入 encode
4. 目标：重探针 CPU 更近 solid 档；RSS 斜率趋 0

### Tooling
- `GPUI_MEMPROFILE=path`：退出时 `WriteHeapProfile`（`examples/particle_kitchen_sink/main.go`）


## opt12 glyph-mask staging + dual-tex multi-submit (2026-07-17)

### Engine fixes (no content cut)
1. **Glyph mask path**
   - `buildGlyphMaskVertexDataInto` / `IndexDataInto` / `makeGlyphMaskUniformInto` + session staging
   - Quad scratch reuse (`glyphMaskQuadScratch`)
   - **Index buffer grow-only**: skip `WriteBuffer` when `totalQuads <= glyphMaskIdxUploadedQuads`
   - Glyph mask bind group reuse when atlas view + LCD flag stable
2. **dual-tex advanced layers**
   - `dualTexAdvancedBlendViewsMulti`: multiple layers → **one Submit** (uniform ring)
   - `resolvePendingAdvancedLayersEnc` prefers multi; falls back to per-op on error
   - Opacity still applied at blit (same as single-op path)

### Measured

| probe | opt11 | **opt12** |
|-------|-------|-----------|
| P_SOLID | 13.6 | **13.0** |
| P_GLOW | 16.4 | **15.4** |
| P_BLEND_GLOW | 25.3 | 25.4 |
| P_L3 | 30.2 | **30.1** |
| P_MEM_SOAK 30s | 30.7 / 22MB | **30.7 / 20MB** |
| MEM45 | 31.1 / 30MB | **31.2 / 29MB** |
| heap alloc 20s | ~343MB | **~302MB** |

All PASS; fps≈57–58; `cpu_fb=0`.

### pprof notes (P_L3)
- Still dominated by `cgocall` / `Flush` / `Present` / `WriteBuffer` / `Submit`
- `PopLayer` ~20%; multi-submit helps only when **multiple** advanced layers share a resolve
- Remaining heap: `layoutGlyphs` → raster/hint (changing HUD + mask cold paths)

### Still open
1. RSS steady ~15–30MB / 30–45s (native-heavy; Go inuse small)
2. Heavy probe CPU ~30% vs solid ~13% — need fewer Flush/Submit on layer+glow+mesh stack
3. Optional next: effect RT light flush; HUD digit layout amortize; RSS feature-axis bisect


## opt13 LayoutText batch cache + owned glyph quads (2026-07-17)

### Engine fixes (no content cut)
1. **GlyphMaskEngine layout batch cache** (`layoutCache`, soft 256)
   - Key: text hash + font + size + quantized x/y (1/64 px) + color + LCD/aliased/hint
   - Only pure-translate matrices (HUD-style)
   - Sticky FPS/CPU strings hit without re-shaping/quad build
2. **layoutGlyphs** reuses `quadScratch`
3. **QueueGlyphMask** copies quads into `glyphMaskQuadStore` (grow-only, reset each flush)
   - Safe with engine scratch/cache aliases across multiple DrawString per frame

### Measured

| probe | opt12 | **opt13** |
|-------|-------|-----------|
| P_SOLID | 13.0 | **12.3** |
| P_GLOW | 15.4 | 15.5 |
| P_BLEND_GLOW | 25.4 | **24.6** |
| P_L3 | 30.1 | **29.5** |
| P_MEM_SOAK 30s | 30.7 / 20MB | **28.8 / 20MB** |
| MEM45 | 31.2 / 29MB | **29.2 / 29MB** |
| heap alloc 20s | ~302MB | **~179MB (−41%)** |

All PASS; fps≈57–58; hitch low; `cpu_fb=0`.

### pprof (P_L3)
- `DrawString` / `LayoutText` dropped out of top (~4% cum)
- `layoutGlyphs` **gone from alloc top** (was 168MB cum)
- Remaining: Flush/cgocall, WriteBuffer (convex mesh), PopLayer, Submit, ApplyBlur

### Still open
1. RSS steady ~14–30MB / 20–45s even on solid — native/driver; not fixed by heap wins
2. Heavy CPU still ~2× solid (WriteBuffer mesh + multi-submit stack)
3. Next: coalesceGlyphMaskBatches alloc, fmt.Errorf hot path, effect light Flush, fewer convex WriteBuffers

## opt14 errno false-positive + glyph/image scratch reuse (2026-07-17)

### Engine fixes (no content cut)
1. **rwgpu `unixProc.Call`**
   - wgpu C ABI does **not** report failure via `errno`
   - residual thread-local errno caused `fmt.Errorf` on nearly every present-path FFI call (~24.5MB / ~349k objects in opt13)
   - ignore errno; cache missing-symbol error once
   - real errors still via device uncaptured-error callback
2. **`coalesceGlyphMaskBatches` / `coalesceTextBatches`**
   - session grow-only out + quad pools (`coalesceGlyphOut/Quads`, `coalesceTextOut/Quads`)
   - pools accumulate per frame (`resetCoalesceScratch`); multi-group safe
3. **`buildImageVerticesInto`**
   - write 6-vert quads directly into `imageVertexStaging` / `gpuTexVertexStaging` (no per-quad `make`)
4. **image draw-call scratch**
   - `imageDrawCallScratch` for buildImageResources
   - `imageSliceDrawScratch` + `resetImageSliceScratch` for multi-group slice (no per-group `make`)

### Measured

| probe | opt13 | **opt14** |
|-------|-------|-----------|
| P_SOLID | 12.3 | **12.5** |
| P_GLOW | 15.5 | **15.2** |
| P_BLEND_GLOW | 24.6 | **23.2** |
| P_L3 | 29.5 | **28.1** |
| P_MEM_SOAK 30s | 28.8 / ~20MB dRSS | **28.1 / ~21MB** |
| MEM45 | 29.2 / ~29MB | **27.9 / ~28MB** |
| heap alloc 20s | ~179MB | **~118MB (−34%)** |

All PASS; fps≈57–58; `cpu_fb=0`; content/pixel OK.

### pprof (P_MEM_SOAK alloc_space)
- **`fmt.Errorf` gone** from top (was 24.5MB / 13.7%)
- **`coalesceGlyphMaskBatches` gone** (was 12MB)
- **`buildImageVertices` gone** (was 7MB)
- Remaining: `Queue.WriteBuffer` / `SetBindGroup` / `Draw` (purego `SyscallN` `//go:uintptrescapes` + variadic Call), text cold-path raster/hint, CreateBindGroup

### Still open
1. FFI hot path still allocates via purego `SyscallN` uintptrescapes + `Call(...uintptr)` — consider fixed-arity `syscall15X` wrappers (opt15)
2. RSS steady slope ~21–28MB / 30–45s — native/driver; not fixed by Go heap wins
3. Heavy probe CPU still ~2× solid (WriteBuffer mesh + multi-submit / PopLayer / blur stack)
4. Optional: effect RT light Flush; convex dirty-range WriteBuffer; fewer CreateBindGroup

### Evidence
- `tmp/pks/cpu_opt/opt14_*.json`, `opt14_MEM45.json`
- `tmp/pks/cpu_opt/mem_opt14.pprof`, `pl3_opt14.pprof`

## opt15 no-escape FFI + fixed-arity hot calls (2026-07-17)

### Engine fixes (no content cut)
1. **Local purego `SyscallNNoEscape`** (`../purego/syscall.go`)
   - same as `SyscallN` but **without** `//go:uintptrescapes`
   - avoids forcing present-path uintptr/variadic args onto the heap
   - pointer-passing callers use `runtime.KeepAlive`
2. **rwgpu `unixProc.Call`** routes through `SyscallNNoEscape` (errno still ignored; missing-symbol cached)
3. **Fixed-arity `call1`…`call6`** on unix hot path — bypasses `Proc.Call(...uintptr)` slice alloc
   - wired: SetPipeline / SetBindGroup / SetVertexBuffer / SetIndexBuffer / Draw / DrawIndexed /
     SetScissorRect / SetBlendConstant / SetStencilReference / End / Release /
     WriteBuffer / WriteBufferRaw / BeginRenderPass / Finish / Submit / CreateBindGroup*
4. **Queue.submitHandles** grow-only scratch (no per-Submit `make`)
5. KeepAlive on WriteBuffer/WriteTexture/CreateBindGroup/BeginRenderPass/Submit pointer args

### Measured (opt15b = fixed-arity final)

| probe | opt14 | **opt15b** |
|-------|-------|------------|
| P_SOLID | 12.5 | **11.8** |
| P_GLOW | 15.2 | **15.0** |
| P_BLEND_GLOW | 23.2 | **23.6** |
| P_L3 | 28.1 | **27.5** |
| P_MEM_SOAK 30s | 28.1 / ~21MB | **27.5 / ~20MB** |
| heap alloc 20s | ~118MB | **~102MB (−14%)** |
| alloc objects 20s | ~1.20M | **~0.75M (−37%)** |

All PASS; fps≈57–58; `cpu_fb=0`.

### pprof
- **`WriteBuffer` / `SetBindGroup` / `Draw` allocs eliminated** from top (were dominant after opt14)
- Remaining FFI-ish: `BeginRenderPass`, `callRenderPassEncoderSetViewport` (RegisterFunc/float path), `CreateBindGroup` cold
- Remaining engine: text raster/hint cold paths, layoutGlyphs uncached, pixmap/image

### Still open
1. **SetViewport** float RegisterFunc path still allocates — promote to fixed non-escape if feasible
2. **BeginRenderPass** descriptor packing allocs
3. RSS steady slope ~20MB / 30s — native/driver
4. Heavy CPU still ~2× solid (mesh WriteBuffer volume, PopLayer/blur, multi-submit) — next CPU pass
5. Optional: fewer CreateBindGroup; effect RT light Flush

### Evidence
- `tmp/pks/cpu_opt/opt15b_*.json`, `mem_opt15b.pprof`
- purego: `SyscallNNoEscape`; rwgpu: `call1`…`call6` in `loader_unix.go`

## opt16 viewport/uniform skip + mesh layer damage (2026-07-17)

### Engine fixes (no content cut)
1. **SetViewport skip-if-same** on `rwgpu.RenderPassEncoder` (within-pass; invalidated on End/Release)
2. **Convex/SDF uniform WriteBuffer skip** when viewport w/h/AA unchanged (`convexUniformValid` / `sdfUniformValid`)
3. **makeSDFRenderUniformInto** — no full-buffer zero loop; pad tail only
4. **DrawVertices → trackDamageDevicePoints** for isolation-layer Pop damage  
   - fast-path: skip O(n) AABB when no isolation layer / opacity-group / fullComposite
5. **PopLayer**: damage flush only when dirty area **&lt; 70%** of layer; near-full → full flush (avoids damage setup cost on particle-filled stages)

### Reverted / not kept
- Session `buildConvexBlendRangesScratch` — multi-group slice reuse risk; free `buildConvexBlendRanges` retained

### Measured (opt16d exclusive, vs opt15b)

| probe | opt15b | **opt16d** |
|-------|--------|------------|
| P_SOLID | 11.8 | **12.0** |
| P_GLOW | 15.0 | **15.3** |
| P_BLEND_GLOW | 23.6 | **24.5** |
| P_L3 | 27.5 | **27.6** |
| P_MEM_SOAK 30s | 27.5 / ~20MB | **28.0 / ~21MB** |
| heap alloc 20s | ~102MB | **~99MB** |
| alloc objects | ~0.75M | **~0.71M** |

All PASS; fps≈57–58; `cpu_fb=0`.

### pprof notes
- CPU still ~62% `cgocall` / Flush / WriteBuffer / PopLayer / Submit
- `callRenderPassEncoderSetViewport` still allocates once per new pass (cache helps multi-viewport only)
- Remaining: CreateCommandEncoder churn, text cold path, BeginRenderPass

### Still open
1. Heavy CPU ~2× solid: mesh vertex rebuild+WriteBuffer every frame, multi-submit glow/layer stack
2. SetViewport RegisterFunc alloc (one per pass)
3. BeginRenderPass / CreateCommandEncoder pooling
4. RSS slope ~20MB/30s native
5. Optional next: dirty-mesh upload, effect light Flush, encoder reuse

### Evidence
- `tmp/pks/cpu_opt/opt16d_*.json`, `mem_opt16d.pprof`

## opt17 BeginRenderPass pool (2026-07-17)

### Kept (engine, no content cut)
1. **`rwgpu.BeginRenderPass` single-attachment pool** — reuse native color-attachment slice for the common 1-RT path (`gpu/rwgpu/render.go`).

### Attempted then **reverted** (text correctness)
1. **HUD digit/mixed compose in `GlyphMaskEngine.LayoutText`** — char-by-char ASCII + CJK run stitch was tried to cut FPS/frame string shape thrash.
   - Broke real text: wrong advances / large or garbled glyphs on mixed HUD and digit pure-translate strings.
   - Root: compose is not equivalent to full `text.LayoutGlyphs` (face cmap, MultiFace runs, .notdef advance, kerning).
   - **Fully reverted** to opt13 path: always `text.LayoutGlyphs` + layout batch cache.
2. **Convex vertex FNV skip-if-same WriteBuffer** — extra full-buffer hash every animated mesh frame; net CPU regression on P_L3. **Reverted**.

### Gate evidence (post-revert)
- `TestGlyphMask*` / `TestS42*` / `TestGlyphMaskSkipsNotdef` PASS
- `TestF1_*` / `TestP04_ApplyBlurGPU` PASS
- Mem leak suite earlier this turn: `tmp/gpui_mem_leak_tests.log` **DONE fail=0** (count=2)
- Full unit (pre-opt17 morning): effective green (`tmp/full_unit/STATUS.md`)

### Still open
1. Heavy CPU ~2× solid (mesh WriteBuffer + multi-submit glow/layer stack)
2. RSS slope ~15–30MB / 30–45s native-heavy
3. HUD digit amortize needs a **correct** design (digit atlas / sticky template), not full-string compose
4. Optional: encoder reuse / effect light Flush / fewer PopLayer submits

### Evidence
- `tmp/pks/cpu_opt/opt17b_*.json` (pre-revert, do not treat as green text)
- Post-revert smoke: rebuild `pks_bin_opt17c` + glyph/F1 tests

## opt18 mesh+filter single Queue.Submit (2026-07-17)

### Goal
Cut glow multi-submit (mesh seed Flush + filter graph Submit) to **one** `Queue.Submit` without changing content density or pixel path.

### Engine fixes (pure equivalence / R8-class)
1. `runGPUFilterGraphFromViewWithLeading` — filter CB + optional leading seed CBs in one `Queue.Submit` (`filter_gpu_graph.go`)
2. `GPURenderContext.FlushAndFilterFromView` — ADR-017 shared-encoder mesh encode → Finish mesh CB → leading+filter submit (`filter_flush_coalesce.go`)
3. `tryApplyFilterGraphGPU` prefers combined path when pending draws + FromView available; recovery still submits mesh alone then FromView/readback
4. `FrameFlushes` still counted on successful combined path

### Gates (sandbox / offscreen; no X11 in agent bwrap)
| test | result |
|------|--------|
| `TestOpt18_ApplyBlur_MeshSeedGPUFilter` | PASS `cpu_fb=0` GPUFilterTexture |
| `TestP04_ApplyBlurGPU` | PASS |
| `TestR72_ApplyBlur_GPUFilterTextureNoExportPrefer` | PASS |
| `TestF1_AdvancedLayerPresentView_HUDText` | PASS |

### Host PKS verification (required — X11 present path)
Agent sandbox cannot `connect(/tmp/.X11-unix/X1)` (EPERM). On the host desktop:

```bash
cd examples/particle_kitchen_sink
export WGPU_NATIVE_PATH=$PWD/../../lib/libwgpu_native.so
export LD_LIBRARY_PATH=$PWD/../../lib:$LD_LIBRARY_PATH
export DISPLAY=:1
export GPUI_SURFACE_SAMPLE_COUNT=1
export GOCACHE=/tmp/gpui-go-cache

mkdir -p ../../tmp/pks/cpu_opt
for p in P_SOLID P_GLOW P_BLEND_GLOW P_L3 P_MEM_SOAK; do
  sec=8; [ "$p" = P_MEM_SOAK ] && sec=30
  GPUI_PROBE=$p GPUI_ANIM_SECONDS=$sec     GPUI_RESULT_FILE=../../tmp/pks/cpu_opt/opt18_${p}.json     go run . | tee ../../tmp/pks/cpu_opt/opt18_${p}.log
done
```

Compare cpu/hitch vs opt17b in `tmp/pks/cpu_opt/opt17b_*.json`. Expect glow/L3 process CPU ≤ prior; hitch_ratio not worse; `cpu_fallback_ops=0`.

### Still open
1. Heavy CPU ~2× solid residual (mesh WriteBuffer volume remains; only Submit coalesced)
2. RSS slope ~15–30MB / 30–45s
3. HUD digit amortize (correct design only)
4. Optional: true single-CB mesh+filter encode (harder barrier/lifecycle); encoder reuse

### Policy
- No content gutting
- Prefer GPU; no silent CPU
- Revert if PKS correctness/hitch regresses

## opt18+opt19 measured on host X11 (2026-07-17)

### Scope
- **opt18**: mesh seed + filter `Queue.Submit` coalesce (`FlushAndFilterFromView`)
- **opt19**: `QueueColoredMesh` pre-packs `PackedVerts`; flush memcpy only (no second Points→stride walk)

### PKS present matrix (`tmp/pks/cpu_opt/opt19_*.json`, 8s, DISPLAY=:1)

| probe | status | fps_ema | cpu_avg | low_fps_ratio | cpu_fb | vs opt17b cpu |
|-------|--------|---------|---------|---------------|--------|---------------|
| P_SOLID | PASS | 62.6 | 12.9 | 0.033 | 0 | +0.5pp (noise) |
| P_GLOW | PASS | 58.7 | 15.9 | 0.028 | 0 | +0.2pp |
| P_BLEND_GLOW | PASS | 58.5 | 24.3 | 0.062 | 0 | +0.1pp |
| P_L3 | PASS | 59.4 | 27.7 | 0.025 | 0 | **−1.0pp** |

All PASS, `cpu_fallback_ops=0`, no content gutting. L3 modest CPU win; glow path within noise of opt17b (submit already cheap after earlier knives).

### Unit gates
| test | result |
|------|--------|
| `TestOpt18_ApplyBlur_MeshSeedGPUFilter` | PASS |
| `TestOpt19_PackedVerts_MatchesTriangleListPack` | PASS |
| `TestP04_ApplyBlurGPU` / `TestR72_*` | PASS |

### Still open (next knives)
1. Mesh **WriteBuffer volume** still dominates cgocall on dense animated meshes (pack helps CPU pack, not PCIe/upload bytes)
2. RSS steady slope on long soak
3. HUD digit amortize (correct design only)
4. Optional: true single-CB mesh+filter encode; encoder reuse

### Policy
- Equivalence-only (class A); no AA/content cuts
- Prefer GPU; no silent CPU
- Keep if PKS green; revert pack path if pixel/mesh tests regress

## opt20 zero-copy packed mesh upload + dual convex VB (2026-07-17)

### pprof (P_L3 present, pre-opt20 `pl3_opt20base.pprof`)
- `cgocall` ~65%; `WriteBuffer` ~16% cum; `Finish`/`Submit` ~13–14%
- `buildConvexResources`: WriteBuffer 83% / `buildConvexVerticesReuse` 17% (pack already cheap after opt19)
- `memmove` still visible from staging copy of full mesh

### Engine fixes (class A equivalence)
1. **Zero-copy WriteBuffer** when a flush has a single `PackedVerts` TriangleList mesh — skip `convexVertexStaging` memcpy (`buildConvexResources`)
2. **Dual convex vertex buffers** (ping-pong) — avoid WriteBuffer competing with previous frame still sampling the same VB

### PKS present (8s, DISPLAY=:1, `tmp/pks/cpu_opt/opt20_*.json`)

| probe | opt17b cpu | opt19 cpu | **opt20 cpu** | opt20 status | notes |
|-------|------------|-----------|---------------|--------------|-------|
| P_SOLID | 12.4 | 12.9 | **12.2** | PASS | fb=0 |
| P_GLOW | 15.7 | 15.9 | **15.8** | PASS | fb=0 |
| P_BLEND_GLOW | 24.2 | 24.3 | **23.1** | PASS | low_fps 0.062→0.020 |
| P_L3 | 28.8 | 27.8 | **28.7** | PASS | within noise of baselines |

Post-opt20 pprof: `buildConvexVerticesReuse` **0.06s→0.02s** inside buildConvex; WriteBuffer volume remains (expected — particles animate).

### Unit gates
- `TestOpt19_PackedVerts_*` / `TestOpt18_*` / `TestP04_*` / `./render` targeted — PASS

### Still open
1. **WriteBuffer byte volume** for animated mesh (needs smaller format / GPU particle update — larger design)
2. PopLayer multi-flush stack (~23% cum) — frame-model coalesce
3. RSS long-soak slope
4. HUD digit amortize (correct design)

### Policy
- No content gutting; `cpu_fb=0` locked
- Keep zero-copy + dual VB (low risk); next knife elsewhere if L3 cpu plateaus


## opt21 PopLayer layer-RT defer-submit coalesce (2026-07-17)

### pprof (P_L3 present, pre=opt20 `pl3_opt20.pprof` → post=`pl3_opt21.pprof`)
- PopLayer cum **23.4% → 12.6%** (FlushGPUWithViewDamage branch)
- Queue.Submit still ~12% (expected — present + dual-tex remain; layers share one leading Submit)
- New path visible: `FlushLeadingSubmitsOnly` / `finishSurfaceSubmit` (~5%)

### Engine fix (class A equivalence)
**Problem:** Advanced `PushLayer(Screen|Multiply)` still mid-flushes layer RT content on every `PopLayer` (2× Finish+Submit/frame on P_L3 blend). Parent draws are already present-stashed (F1); only the layer fill paid an immediate `Queue.Submit`.

**opt21:**
1. When flushing a **layer self-target while `presentStash.active`**, encode the surface pass but **`EnqueueLeadingSubmit`** instead of immediate Submit (`SetDeferSurfaceSubmit`).
2. Before the next non-deferred `RenderFrameGrouped` (usually Present base / advanced path), **`FlushLeadingSubmitsOnly`** drains all deferred layer CBs in **one** `Queue.Submit` — avoids sharing MSAA attachments across unsubmitted CBs while still coalescing multi-layer fills.
3. Convex VB ring **2 → 4** so deferred layer fills do not `WriteBuffer` over an unsubmitted draw's vertex buffer; drain if ring exhausted.
4. `BeginFrame` also drains stranded lead CBs.

Pure equivalence: same encode, same draw order, only submit batching changes (same pattern as R7.3 / opt18 leading CBs).

### PKS present matrix (`tmp/pks/cpu_opt/opt21_*.json`, 8s, DISPLAY=:1)

| probe | opt20 cpu | **opt21 cpu** | opt21 fps_ema | status | cpu_fb |
|-------|-----------|---------------|---------------|--------|--------|
| P_SOLID | 12.2 | 12.8 | 62.5 | PASS | 0 |
| P_GLOW | 15.8 | 15.1 | 59.6 | PASS | 0 |
| P_BLEND_GLOW | 23.1 | 23.4 | 58.3 | PASS | 0 |
| P_L3 | 28.7 | **27.3** | 58.6 | PASS | 0 |
| P_LAYER | — | 12.6 | 59.0 | PASS | 0 |

L3 ~**−1.4pp CPU** (noisy host; pprof structure is the stronger signal). No content gutting; `cpu_fallback_ops=0`.

### Unit gates
| test | result |
|------|--------|
| `TestOpt21_DeferSurfaceSubmit_CoalescesLayerFills` | PASS |
| `TestF1_AdvancedLayerPresentViewMultiply/Screen/HUDText` | PASS |
| `TestOpt18_*` / `TestP04_ApplyBlurGPU` / `TestP03_Advanced*` / `TestS3b_M2_Layer*` | PASS |

### Still open
1. Animated mesh **WriteBuffer byte volume** (still ~18% WriteBuffer on L3)
2. True single-CB frame (base+layer+dual-tex) still disabled (`singleSubmit:=false` — pixel incorrect previously)
3. RSS long-soak slope
4. HUD digit amortize (correct design only)

### Policy
- Class A only; F1 present-path advanced blend must stay green
- Keep if PKS PASS + no F1 regression; revert defer path if hitch/pixel regresses
- Next knife: upload volume / GPU particle update — not more submit micro-batching unless pprof shows Submit >> WriteBuffer

## opt22 indexed DrawMesh + present-stash packed ownership (2026-07-17)

### pprof / diagnosis (post-opt21)
- `WriteBuffer` still ~18% on P_L3; `buildConvexResources` hot
- Root cause for disc meshes: `DrawMesh` **CPU-expanded** indices → triangle list (~2.7× verts for 10-seg discs)
- Secondary correctness: present-stash only shallow-copied `ConvexDrawCommand`; layer `QueueColoredMesh` could overwrite `rc.convexMeshPacked` under parent `PackedVerts` slices

### Engine fixes (class A equivalence)
1. **`QueueColoredMeshIndexed` + `DrawMesh` indexed GPU path** — unique verts + `uint16` indices → `DrawIndexed` (convex pipeline). Same triangles/pixels; less WriteBuffer for indexed meshes (discs, fans).
2. **Convex index buffer ring** + blend ranges with `baseVertex` (indexed ranges never merged).
3. **present-stash deep-copy** of `PackedVerts`/`Indices` into stash-owned storage; unstash relocates into rc scratch (opt22 correctness).

### PKS present (`tmp/pks/cpu_opt/opt22_*.json`, 8s, DISPLAY=:1)

| probe | opt21 cpu | **opt22 cpu** | fps_ema | status | cpu_fb |
|-------|-----------|---------------|---------|--------|--------|
| P_SOLID | 12.8 | **12.2** | 56.9 | PASS | 0 |
| P_GLOW | 15.1 | **14.8** | 58.2 | PASS | 0 |
| P_BLEND_GLOW | 23.4 | **22.2** | 59.2 | PASS | 0 |
| P_L3 | 27.3 | **26.6** | 58.6 | PASS | 0 |
| P_LAYER | 12.6 | **11.4** | 58.1 | PASS | 0 |

WriteBuffer **share** still ~18% (base 1600-tri mesh still unique verts — index path saves disc/layer fans more than base). Absolute PKS CPU improved; structure fix + disc upload cut are the main wins.

### Unit gates
| test | result |
|------|--------|
| `TestOpt22_IndexedMesh_FewerUploadBytes` | PASS |
| `TestOpt22_StashPreservesPackedVerts` | PASS |
| `TestF1_AdvancedLayerPresent*` | PASS |
| `TestOpt21_*` / `TestOpt19_*` | PASS |

### Still open
1. Base animated mesh **upload volume** (needs compact vertex format or GPU particle update — larger than class A)
2. True single-CB frame (`singleSubmit` still false)
3. RSS long-soak
4. HUD digit amortize (correct design)

### Policy
- Keep indexed path + stash deep-copy (correctness + disc upload)
- Do not gut particle N / AA / disc segs to green metrics
- Next: format/GPU sim only with pixel gates if tackling WriteBuffer further

## opt23 mesh pack hot-path + LE index zero-copy (2026-07-17)

### pprof (P_L3, opt22 → opt23)
| symbol | opt22 cum | opt23 cum |
|--------|-----------|-----------|
| DrawMesh | 5.83% | **2.62%** |
| QueueColoredMeshIndexed | 4.17% | **1.31%** |
| WriteBuffer | 18.33% | **14.85%** |
| buildConvexResources | 11.67% | **8.30%** |

### Engine fixes (class A equivalence)
1. **`packMeshVertsCoverage1`** — tight SkipAA mesh pack (coverage=1 fixed, direct f32 bit stores, no per-vert `writeConvexVertex`/`binary.LittleEndian` calls)
2. **Drop O(n) index pre-validation** on indexed queue (DrawMesh supplies in-range indices; expands only on CPU fallback path)
3. **Identity CTM fast path** in `DrawMesh` — queue user-space positions without transform copy
4. **LE index WriteBuffer zero-copy** — single indexed command uploads `[]uint16` as bytes via `unsafe.Slice` (no per-index LE encode loop)

Same verts/indices/pixels; only CPU pack/upload path.

### PKS present (`tmp/pks/cpu_opt/opt23_*.json`, 8s, DISPLAY=:1)

| probe | opt22 cpu | **opt23 cpu** | fps_ema | status | cpu_fb |
|-------|-----------|---------------|---------|--------|--------|
| P_SOLID | 12.2 | **11.6** | 58.6 | PASS | 0 |
| P_GLOW | 14.8 | **13.3** | 58.4 | PASS | 0 |
| P_BLEND_GLOW | 22.2 | **20.9** | 59.0 | PASS | 0 |
| P_L3 | 26.6 | **24.6** | 58.6 | PASS | 0 |
| P_LAYER | 11.4 | 11.4 | 58.1 | PASS | 0 |

L3 **−2.0pp CPU**; no content gutting.

### Unit gates
| test | result |
|------|--------|
| `TestOpt23_PackMeshVertsCoverage1_MatchesWriteConvexVertex` | PASS |
| `TestOpt22_*` | PASS |
| `TestF1_AdvancedLayerPresent*` | PASS |

### Still open
1. Remaining WriteBuffer volume on dense base mesh (compact format / GPU particle update)
2. single-CB frame (disabled for pixel correctness)
3. RSS long-soak
4. HUD digit amortize (correct design)

### Policy
- Keep pack/index zero-copy (measured win + byte-identical pack test)
- Revert if pack golden or F1 advanced layer regresses

## opt24 sticky index ring + layout/uniform skip (2026-07-17)

### pprof / diagnosis (post-opt23)
- `WriteBuffer` still ~15% on P_L3; index topology re-uploaded every flush even when verts-only animate
- Multi-topology thrash (base/layer/glow) made single sticky-snap a net loss → multi-slot hash reuse
- Static HUD strings still re-`LayoutGlyphs` before template hit (shaped unused on hit path)
- Image/gpu-tex uniforms rewritten every frame for fixed viewport/opacity

### Engine fixes (class A equivalence)
1. **Convex index multi-slot sticky** — FNV64 + len fingerprint per index ring slot; identical topology reuses buffer (no WriteBuffer)
2. **`LayoutText` / `LayoutTextAliased` template-before-shape** — try layout template first; only shape on miss
3. **Image + GPU-texture uniform skip** — skip WriteBuffer when viewport W/H + opacity unchanged for pool slot

Same triangles/pixels/text; only CPU pack/upload path.

### PKS present (`tmp/pks/cpu_opt/opt24_*.json`, 8s, DISPLAY=:1)

| probe | opt23 cpu | **opt24 cpu** | fps_ema | status | cpu_fb |
|-------|-----------|---------------|---------|--------|--------|
| P_SOLID | 11.6 | **10.8** | 58.6 | PASS | 0 |
| P_GLOW | 13.3 | 14.1 | 58.5 | PASS | 0 |
| P_BLEND_GLOW | 20.9 | **19.7** | 59.6 | PASS | 0 |
| P_L3 | 24.6 | **23.8** | 58.9 | PASS | 0 |
| P_LAYER | 11.4 | **10.9** | 58.5 | PASS | 0 |

L3 **−0.9pp CPU** (host-noisy; structure: index reuse + static text skip). No content gutting.

### Unit gates
| test | result |
|------|--------|
| `TestOpt24_StickyIndex_ReusesRingSlotOnSameTopology` | PASS |
| `TestOpt24_LayoutTemplate_HitBeforeShape` | PASS |
| `TestOpt24_ImageUniformSkip_SameViewportOpacity` | PASS |
| `TestOpt22_*` / `TestOpt23_*` / `TestR75_LayoutTemplate*` | PASS |
| `TestF1_AdvancedLayerPresent*` | PASS |

### Still open
1. Animated mesh **vertex WriteBuffer volume** (compact format / GPU particle update — class B territory)
2. single-CB frame (`singleSubmit` still false for pixel correctness)
3. RSS long-soak
4. HUD digit amortize for *changing* FPS strings (template misses every frame; needs correct digit design)

### Policy
- Keep multi-slot index sticky + template-before-shape + uniform skip
- Revert sticky if index hash collision ever observed (theoretical; FNV64+len)
- Next knife: filter/submit structure or compact verts with pixel gates — not more micro-batching unless Submit >> WriteBuffer

### Evidence
- `tmp/pks/cpu_opt/opt24_*.json`, `pl3_opt24.pprof`
- Tests: `render/internal/gpu/opt24_sticky_index_uniform_test.go`

## opt25 multi-cmd contiguous packed zero-copy (2026-07-17)

### pprof / diagnosis (post-opt24)
- `buildConvexResources` still ~10.5%; multi-DrawMesh flushes fell into `buildConvexVerticesReuse` staging `memmove` even though Queue packing already laid verts/indices contiguously in `convexMeshPacked` / `convexMeshIdx`
- Single-cmd opt20 zero-copy missed the common multi-mesh group case (base + discs + alpha)

### Engine fixes (class A equivalence)
1. **`packedMeshVertsContiguous`** — if all cmds are TriangleList+SkipAA+PackedVerts and slices are adjacent in one backing array, WriteBuffer the combined range (no staging memcpy)
2. **`packedMeshIndicesContiguous`** — same for `[]uint16` index payloads → LE byte view without `convexIndexStaging` concat
3. Warm frames (grow-only capacity) hit zero-copy; realloc mid-frame still falls back to copy (correct)

Same verts/indices/pixels/draws; only CPU packing path.

### PKS present (`tmp/pks/cpu_opt/opt25_*.json`, 8s, DISPLAY=:1)

| probe | opt24 cpu | **opt25 cpu** | fps_ema | status | cpu_fb |
|-------|-----------|---------------|---------|--------|--------|
| P_SOLID | 10.8 | **10.4** | 58.5 | PASS | 0 |
| P_GLOW | 14.1 | **13.6** | 58.3 | PASS | 0 |
| P_BLEND_GLOW | 19.7 | 20.1 | 59.9 | PASS | 0 |
| P_L3 | 23.8 | **23.4** | 58.7 | PASS | 0 |
| P_LAYER | 10.9 | **10.7** | 58.3 | PASS | 0 |

L3 **−0.3pp CPU**; pprof: `buildConvexResources` **0.23s→0.15s**, `buildConvexVerticesReuse` off hot path.

### Unit gates
| test | result |
|------|--------|
| `TestOpt25_PackedMeshVertsContiguous_MultiCmd` | PASS |
| `TestOpt25_PackedMeshIndicesContiguous_MultiCmd` | PASS |
| `TestOpt25_BuildConvexResources_MultiCmdZeroCopy` | PASS |
| `TestOpt24_*` / `TestOpt22_*` / `TestOpt23_*` | PASS |
| `TestF1_AdvancedLayerPresent*` | PASS |

### Still open
1. Animated mesh **vertex WriteBuffer volume** (compact format / GPU particle update)
2. Image atlas vertex WriteBuffer still material
3. single-CB frame disabled for pixel correctness
4. HUD digit amortize for changing FPS strings
5. RSS long-soak

### Policy
- Keep contiguous zero-copy (measured + unit-proven)
- Revert if multi-mesh pixels/ranges regress
- Next: image upload volume / filter-Finish structure / class-B compact verts with pixel gates

### Evidence
- `tmp/pks/cpu_opt/opt25_*.json`, `pl3_opt25.pprof`
- Tests: `render/internal/gpu/opt25_contiguous_pack_test.go`

## opt26 dual-tex BG slot cache + fontID/index fingerprint (2026-07-17)

### pprof / diagnosis (post-opt25)
- `CreateBindGroup` ~4.2%: dual-tex multi-bundle recreated BGs every advanced-layer resolve (and released after Submit) despite `dualTexBlendCache.bgCache` field existing unused
- `hash/fnv.(*sum64a).Write` ~0.9% on convex sticky index path (opt24)
- `computeGlyphMaskFontID` used `fmt.Fprintf` every `LayoutText` (incl. template-hit path)

### Engine fixes (class A equivalence)
1. **`dualTexBlendCache.multiBindGroup`** — per-op slot cache keyed by (dstView, srcView, uniformBuf); warm frames reuse native BG; ownership stays on cache (Cleanup no longer Releases)
2. **`GlyphMaskEngine.fontID`** — pointer-cached font identity; `computeGlyphMaskFontID` matches legacy `fmt.Fprintf("%s:%d")` FNV without fmt
3. **`indexBytesFingerprint`** — word-mix sticky index key (no `hash.Hash` interface)

Same pixels/bind content; only BG/handle reuse + CPU hash path.

### PKS present (`tmp/pks/cpu_opt/opt26_*.json`, 8s, DISPLAY=:1)

| probe | opt25 cpu | **opt26 cpu** | fps_ema | status | cpu_fb |
|-------|-----------|---------------|---------|--------|--------|
| P_SOLID | 10.4 | **9.7** | 58.4 | PASS | 0 |
| P_GLOW | 13.6 | 13.6 | 59.7 | PASS | 0 |
| P_BLEND_GLOW | 20.1 | **19.7** | 58.8 | PASS | 0 |
| P_L3 | 23.4 | 23.6 | 59.3 | PASS | 0 |
| P_LAYER | 10.7 | **10.2** | 57.9 | PASS | 0 |

L3 CPU flat/noise; pprof structure: dual-tex `CreateBindGroup` **0.06s→0.01s**, total CreateBindGroup **0.09s→0.06s**, sticky `fnv.Write` off hot path.

### Unit gates
| test | result |
|------|--------|
| `TestOpt26_DualTexMultiBindGroup_ReusesSlot` | PASS |
| `TestOpt26_ComputeGlyphMaskFontID_MatchesLegacyFmt` | PASS |
| `TestOpt26_IndexBytesFingerprint_Stable` | PASS |
| `TestR75_LayoutTemplate*` / `TestOpt24_*` / `TestOpt25_*` | PASS |
| `TestF1_AdvancedLayerPresent*` | PASS |

### Still open
1. Animated mesh **vertex WriteBuffer volume**
2. Image/atlas + GPU-texture BG recreate when filter publish swaps views (glow)
3. single-CB frame still disabled
4. HUD digit amortize for changing FPS strings
5. RSS long-soak

### Policy
- Keep multiBindGroup slot cache + fontID/index fingerprint
- Revert dual-tex BG cache if advanced blend pixels/F1 regress (stale BG / early Release)
- Next: glow GPU-tex BG stability across filter publish, or compact mesh verts with pixel gates

### Evidence
- `tmp/pks/cpu_opt/opt26_*.json`, `pl3_opt26.pprof`
- Tests: `render/internal/gpu/opt26_dualtex_bg_fontid_test.go`


## HUD text corruption fix — layout template × atlas compact (2026-07-17)

### Symptom
Top/bottom window HUD text correct at start, later garbled / not real glyphs.

### Root cause
R7.5 layout templates cache atlas UVs. Template hits **do not** call `atlas.Get`, so:
1. Page `lastUsedFrame` was only updated on **Put** (write), not read/template reuse
2. After ~32 frames, `AdvanceFrame` → `compact` **reset** pages still referenced by templates
3. Templates kept stale UVs into zeroed/recycled atlas → corrupted HUD

### Fix (class A correctness)
| file | change |
|------|--------|
| `render/text/glyph_mask_atlas.go` | `generation` bump on `resetPage`/`Clear`; `Generation()`; `TouchPage`; `Get` updates `lastUsedFrame` |
| `render/internal/gpu/glyph_mask_engine.go` | drop layout template cache when atlas generation advances; `TouchPage` on template hit |

### Gates
| test | result |
|------|--------|
| `TestR75_LayoutTemplate_AtlasCompactInvalidatesUVs` | PASS |
| `TestR75_LayoutTemplate_TouchKeepsPageAlive` | PASS |
| `TestGlyphMaskAtlas_Get_KeepsPageAlive` | PASS |
| `TestGlyphMaskAtlas_Generation_*` | PASS |
| `TestR75_*` / `TestOpt24_LayoutTemplate*` | PASS |
| PKS `P_L3` 8s present | PASS `cpu_fb=0` |

### Policy
- Keep generation + TouchPage (correctness)
- Do not return `quadScratch` from template get without deep copy
- Resume opt27 only after this stays green on soak
