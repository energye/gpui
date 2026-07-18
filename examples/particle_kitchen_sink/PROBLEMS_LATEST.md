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
- opt27/opt28 landed after HUD UV fix (see below)


## opt27 gpu-tex multi-view BG slot cache (2026-07-17)

### pprof / diagnosis (post-opt26 + HUD UV fix)
- Glow / filter publish **ping-pongs** `TextureView`s from a free-list (cap 4)
- Single last-view BG cache was a **guaranteed miss** every alternate frame → `CreateBindGroup` on GPU-tex path
- Dual-tex already had multi-slot cache (opt26); gpu-tex path lagged

### Engine fixes (class A equivalence)
1. **`gpuTexBGSlotCache`** — 4-slot view→BG ring per uniform `poolIdx` (matches publish free-list cadence)
2. Miss: create BG, retire displaced BG into `pendingBindGroupRelease` (post-Submit)
3. Hit: reuse native BG; no semantic/draw change
4. `Stats().GPUTexBindGroups` counts live cached BGs

Same pixels/draws/bind content; only BG handle reuse across publish view swap.

### Unit gates
| test | result |
|------|--------|
| `TestOpt27_GPUTexBGSlotCache_ReusesView` | PASS |
| `TestOpt27_BuildGPUTextureResources_MultiViewBGCache` | PASS |
| `TestF1_AdvancedLayerPresentView*` | PASS |
| `TestOpt26_DualTex*` / `TestR75_*` (sample) | PASS |

### PKS present (`tmp/pks/cpu_opt/opt27_*.json`, 8s, DISPLAY=:1)

| probe | opt26 cpu | **opt27 cpu** | fps_ema | status | cpu_fb |
|-------|-----------|---------------|---------|--------|--------|
| P_SOLID | 10 | **9** | 58.4 | PASS | 0 |
| P_GLOW | 14 | **13** | 57.8 | PASS | 0 |
| P_BLEND_GLOW | 20 | **19** | 59.1 | PASS | 0 |
| P_L3 | 24 | 25 | 60.3 | PASS | 0 |
| P_LAYER | 10 | 11 | 58.5 | PASS | 0 |

Glow/solid **−1pp CPU**; L3/LAYER within noise. Correctness: all PASS, `cpu_fb=0`, content/pixel true.

### Policy
- **Keep** multi-view gpu-tex BG ring (measured + unit-proven; no content cut)
- Revert if F1/glow pixels regress (stale BG / early Release)
- HUD UV gen+TouchPage (prior fix) remains required with templates

### Still open
1. Animated mesh **vertex WriteBuffer volume** (compact format / GPU particle — class B if format changes)
2. single-CB frame still disabled for pixel correctness
3. HUD digit amortize for changing FPS strings
4. RSS long-soak
5. Image atlas vertex WriteBuffer still material

### Next knife (class A preferred)
- pprof dig: WriteBuffer / mesh pack / image upload volume without pixel downgrade
- Or class-B compact mesh verts **only with** pixel gates

### Evidence
- `tmp/pks/cpu_opt/opt27_*.json`, `pl3_opt27.pprof` (if present)
- Tests: `render/internal/gpu/opt27_gputex_bg_cache_test.go`


## opt28 sticky image/gpu-tex vertex WriteBuffer (2026-07-17)

### pprof / diagnosis (post-opt27)
- `queueWriteBuffer` ~23% cum on P_L3: **buildImageResources ~47%**, buildConvex ~40%, gpu-tex small
- Atlas sprites move every frame, but **mid-frame flushes** and glow present rects often re-upload identical packed quads
- Uniforms already sticky (opt24); vertex path always `WriteBuffer`

### Engine fixes (class A equivalence)
1. **Image verts**: fingerprint packed staging (`indexBytesFingerprint`); skip WriteBuffer when hash+len match live buffer
2. **GPU-tex verts**: same for overlay + base buffers separately
3. Invalidate sticky on buffer recreate / Destroy

Same GPU buffer contents on hit; miss path identical upload.

### Unit gates
| test | result |
|------|--------|
| `TestOpt28_ImageVertSticky_SkipsRepeatWrite` | PASS |
| `TestOpt28_GPUTexVertSticky_SkipsRepeatWrite` | PASS |
| `TestOpt27_*` / `TestOpt24_ImageUniform*` | PASS |
| `TestF1_AdvancedLayerPresentView*` | PASS |

### PKS present (`tmp/pks/cpu_opt/opt28_*.json`, 8s, DISPLAY=:1)

| probe | opt27 cpu | **opt28 cpu** | fps_ema | status | cpu_fb |
|-------|-----------|---------------|---------|--------|--------|
| P_SOLID | 9 | 10 | 58.3 | PASS | 0 |
| P_GLOW | 13 | **12** | 58.2 | PASS | 0 |
| P_BLEND_GLOW | 19 | 19 | 59.4 | PASS | 0 |
| P_L3 | 25 | **22** | 58.4 | PASS | 0 |
| P_LAYER | 11 | **9** | 58.1 | PASS | 0 |

L3 **−3pp CPU**; LAYER/GLOW improved; SOLID noise. All PASS, `cpu_fb=0`.

### Policy
- **Keep** sticky image/gpu-tex verts (measured + unit-proven)
- Revert if pixel/F1 regress (false sticky hit) — fingerprint covers full packed bytes
- Fingerprint cost acceptable vs avoided cgocall WriteBuffer

### Still open
1. Animated mesh **vertex WriteBuffer volume** (particles always change — needs compact format / GPU update, class B)
2. single-CB frame still disabled for pixel correctness
3. HUD digit amortize for changing FPS strings
4. RSS long-soak
5. CommandEncoder.Finish multi-encode cost

### Next knife
- Mesh compact verts with pixel gates (class B), or Finish/submit structure only if pixel-safe
- Do not full-hash every animated convex mesh (would pay cost with low hit rate)

### Evidence
- `tmp/pks/cpu_opt/opt28_*.json`
- Tests: `render/internal/gpu/opt28_sticky_image_gputex_verts_test.go`


## opt29 image uniform slab — one WriteBuffer for N opacities (2026-07-17)

### pprof / diagnosis (post-opt28)
- `buildImageResources` ~9.6% cum; **160ms of that was per-slot uniform `WriteBuffer`**
- Atlas sprites use **unique animated opacities** → `canMergeImageDraw` fails → N pool slots
- opt24 sticky only helps when opacity/viewport stable; PKS atlas invalidates every frame
- Each slot was an 80B WriteBuffer → **cgo call tax**, not bandwidth

### Engine fixes (class A equivalence)
1. **`imageUniformSlab`**: single buffer, `imageUniformSlotStride=256` (minUniformBufferOffsetAlignment)
2. Pack all slot uniforms → **one** `queueWriteBuffer` when any slot dirty / slab recreated
3. BindGroup entries use `Offset: slot*256, Size: 80`
4. Sticky skip when all slot opacity/viewport keys match (no pack/write)
5. `putImageUniform` helper (payload-only clear for slab slots)

Same projection/opacity/bind semantics; fewer Queue.WriteBuffer calls.

### Unit gates
| test | result |
|------|--------|
| `TestOpt29_ImageUniformSlab_OneWriteForManyOpacities` | PASS |
| `TestOpt29_ImageUniformSlotStride_Aligned` | PASS |
| `TestOpt24_ImageUniform*` / `TestOpt28_*` | PASS |
| `TestF1_AdvancedLayerPresentView*` | PASS |

### PKS present (`tmp/pks/cpu_opt/opt29_*.json`, 8s, DISPLAY=:1)

| probe | opt28 cpu | **opt29 cpu** | fps_ema | status | cpu_fb |
|-------|-----------|---------------|---------|--------|--------|
| P_SOLID | 10 | **9** | 58.1 | PASS | 0 |
| P_GLOW | 12 | 12 | 58.0 | PASS | 0 |
| P_BLEND_GLOW | 19 | 19 | 59.6 | PASS | 0 |
| P_L3 | 22 | **21** | 58.8 | PASS | 0 |
| P_LAYER | 9 | 10 | 58.2 | PASS | 0 |

### pprof structure (P_L3 8s)
| metric | opt28 | **opt29** |
|--------|-------|-----------|
| buildImageResources cum | 200ms (9.6%) | **40ms (2.1%)** |
| queueWriteBuffer total | 0.42s (20%) | **0.27s (14%)** |
| WriteBuffer from image | 0.19s | **0.04s** |
| WriteBuffer from convex | 0.17s | 0.20s (now dominant) |

### Policy
- **Keep** image uniform slab (large structural win, unit+matrix green)
- Revert if bind offset/pixel issues on multi-opacity atlas
- Next hotspot: **convex mesh vertex WriteBuffer** (~0.20s) — class B compact format with pixel gates, or GPU particle update

### Evidence
- `tmp/pks/cpu_opt/opt29_*.json`, `pl3_opt29.pprof`
- Tests: `render/internal/gpu/opt29_image_uniform_slab_test.go`


## opt30 compact convex color unorm8x4 (class B, 2026-07-17)

### pprof / diagnosis (post-opt29)
- `buildConvexResources` WriteBuffer ~0.20s (74% of remaining WriteBuffer)
- Animated mesh verts change every frame → sticky useless
- Vertex was **28B**: pos2(8)+cov(4)+color f32x4(16); mesh coverage always 1.0 but cov kept for shared AA pipeline

### Engine change (class B — color quantize)
1. `convexVertexStride` **28 → 16**: color `float32x4` → **`unorm8x4`** (GPU expands to vec4 in same shader)
2. `writeConvexVertex` / `packMeshVertsCoverage1` pack LE unorm8
3. Vertex layout attribute format `VertexFormatUnorm8x4` at offset 12
4. Same WGSL `VertexInput` (coverage + color as f32)

**Semantic:** solid colors that are k/255 exact are unchanged; continuous gradients may differ ≤1/255.

### Unit / pixel gates
| test | result |
|------|--------|
| `TestOpt30_*` | PASS |
| `TestWriteConvexVertex` / `TestBuildConvex*` | PASS |
| `TestOpt19/22/23/25` mesh pack | PASS |
| `TestF1_AdvancedLayerPresentView*` | PASS |
| PKS content/pixel | true |

### PKS present (`tmp/pks/cpu_opt/opt30_*.json`, 8s)

| probe | opt29 cpu | **opt30 cpu** | status | cpu_fb |
|-------|-----------|---------------|--------|--------|
| P_SOLID | 9 | 9 | PASS | 0 |
| P_GLOW | 12 | 13 | PASS | 0 |
| P_BLEND_GLOW | 19 | **18** | PASS | 0 |
| P_L3 | 21 | **20** | PASS | 0 |
| P_LAYER | 10 | **9** | PASS | 0 |

### Policy
- **Keep** (measured L3 −1pp; mesh bandwidth −43% bytes; F1/PKS pixel gates green)
- Revert if gradient banding visible in product UI
- Next: Finish/submit multi-encode (dual-tex / surface), or further mesh (drop coverage attribute for SkipAA-only pipeline)

### Evidence
- `tmp/pks/cpu_opt/opt30_*.json`, `pl3_opt30.pprof`
- Tests: `render/internal/gpu/opt30_compact_convex_color_test.go`

## opt31 fast mesh unorm pack + stencil create audit (2026-07-17)

### Diagnosis (post-opt30 pprof `pl3_opt30b.pprof`)
1. **`packMeshVertsCoverage1` / `packColorUnorm8x4` / `clampUnorm8` ~70ms cum (3.7%)** on VC mesh path — per-vertex `[4]float32` temp + helper calls after opt30 color quantize.
2. **`ensurePipelines` / `StencilRenderer.createPipelines` ~70–120ms** — profiled as **one-shot cold/probe**, not every-frame thrash. Main↔effect sampleCount ownership already prevents BGL mismatch rebuild (see `preferSampleCount1` comment in `gpu_render_context.go`). No class-A free win from "stop warm recreate".

### Engine change (class A pure-equivalent)
1. `packColorUnorm8x4` → `packColorUnorm8x4RGBA` + `quantUnorm8` (scalar, same clamp/round bits).
2. `packMeshVertsCoverage1` VC loop: **fully inline** channel quantize (no helper call, no temp array); solid path uses scalar pack once.
3. `clampUnorm8` retained as thin wrapper over `quantUnorm8` for existing call sites/tests.

### Unit gates
| test | result |
|------|--------|
| `TestOpt31_*` | PASS |
| `TestOpt30_*` / mesh pack / `TestWriteConvexVertex` | PASS |
| `TestF1_AdvancedLayerPresentView*` | PASS |

### PKS present (`tmp/pks/cpu_opt/opt31_*.json` = opt31b inline, 8s)

| probe | opt30 cpu | **opt31 cpu** | fps_ema | status | cpu_fb | pixel |
|-------|-----------|---------------|---------|--------|--------|-------|
| P_SOLID | 9.3 | **9.2** | 58+ | PASS | 0 | true |
| P_GLOW | 12.9 | 13.2 | 57+ | PASS | 0 | true |
| P_BLEND_GLOW | 18.3 | 18.4 | 59+ | PASS | 0 | true |
| P_L3 | 20.4 | **20.8** | 59.0 | PASS | 0 | true |
| P_LAYER | 9.1 | 9.3 | 58+ | PASS | 0 | true |

L3 repeats: opt30 20.3–20.4; opt31a/b 20.7–20.9 → **wall CPU within noise**.

### pprof structure (P_L3 8s)
| metric | opt30b | **opt31b** |
|--------|--------|------------|
| packMeshVertsCoverage1 cum | 70ms (3.7%) | **10ms (0.5%)** |
| packColorUnorm8x4 / clamp chain | 50ms | **gone from top (inlined)** |
| Stencil createPipelines | ~70ms one-shot | ~60ms one-shot (still cold only) |

### Policy
- **Keep** opt31 as class-A cleanup (bit-identical quantize; pprof pack tax collapsed).
- Wall L3 not improved beyond noise; do **not** chase further micro-pack without higher mesh pressure.
- Stencil warm recreate was a **false lead** on this build — do not spend next knife there without a new thrash repro.
- Next knife: **Finish/submit multi-encode** (`encodeSubmitSurfaceGrouped` / mid-frame encode count) with F1+PKS pixel gates, or class-B SkipAA mesh pipeline (drop coverage attr) only with pixel gates.

### Evidence
- `tmp/pks/cpu_opt/opt31_*.json`, `opt31a/b_P_L3.json`, `pl3_opt31.pprof`, `pl3_opt31b.pprof`
- Tests: `render/internal/gpu/opt31_fast_unorm_pack_test.go`

## opt32 dual-tex multi + composite blit one Finish (2026-07-17)

### Diagnosis (post-opt31 `pl3_opt31b.pprof`)
- `CommandEncoder.Finish` **~380ms (20%)** of L3 samples
- Present advanced-blend path did **two Finishes**: dual-tex multi CB + blit CB (R7.3 only coalesced **Submit**)
- `encodeBlitToEncoder` LoadOp ignored `frameRendered` (Clear risk on shared-encoder composite onto scratch/HUD)

### Engine change (class A)
1. **`dualTexAdvancedBlendViewsMultiIntoEncoder`**: record multi dual-tex passes into caller encoder (no Finish/Submit)
2. **`resolvePendingAdvancedLayersEnc`**: one `dual_tex_composite_enc` → dual-tex multi + `sharedEncoder` blit Flush → **one Finish** + `submitWithLeading` (layers still lead CBs)
3. **`encodeBlitToEncoder` LoadOp** matches `encodeBlitOnlyPass` (`frameRendered || damage` → Load) so base+HUD survive composite
4. Fallback: R7.3 multiBundle lead CB, then per-op region path

Semantics unchanged: same passes, same order in one Submit; fewer Finish cgo calls.

### Unit / pixel gates
| test | result |
|------|--------|
| `TestOpt32_*` | PASS |
| `TestR73_*` | PASS |
| `TestOpt21_*` / `TestP03_*` | PASS |
| `TestF1_AdvancedLayerPresentView*` (`./render`) | PASS |
| PKS content/pixel | true |

### PKS present (`tmp/pks/cpu_opt/opt32_*.json`, 8s)

| probe | opt31 cpu | **opt32 cpu** | fps_ema | status | cpu_fb | pixel |
|-------|-----------|---------------|---------|--------|--------|-------|
| P_SOLID | 9.2 | 10.0 | 58.8 | PASS | 0 | true |
| P_GLOW | 13.2 | **11.9** | 57.5 | PASS | 0 | true |
| P_BLEND_GLOW | 18.4 | **17.4** | 59.2 | PASS | 0 | true |
| P_L3 | 20.8 | **19.6** | 59.0 | PASS | 0 | true |
| P_LAYER | 9.3 | 10.2 | 58.3 | PASS | 0 | true |
| P_BLEND_LAYER | — | 15.7 | 58.6 | PASS | 0 | true |

### pprof structure (P_L3 8s)
| metric | opt31b | **opt32** |
|--------|--------|-----------|
| CommandEncoder.Finish cum | **380ms (20%)** | **180ms (9.4%)** |
| dual/blit-focused slice | 0.62s | 0.55s |

### Policy
- **Keep** (measured L3 −1.2pp; Finish tax ~half; F1/PKS pixel green)
- Revert if dual-tex composite ordering/HUD LoadOp regresses
- SOLID/LAYER ±1pp noise (no dual-tex composite benefit)
- Next: filter/glow encode coalesce, or class-B SkipAA mesh drop coverage attr with pixel gates; avoid re-opening single full-frame CB without dest-correct dual-tex proof

### Evidence
- `tmp/pks/cpu_opt/opt32_*.json`, `pl3_opt32.pprof`
- Tests: `render/internal/gpu/opt32_dual_tex_composite_encoder_test.go`

## opt33 SkipAA mesh 12B verts (drop constant coverage) (2026-07-17)

### Diagnosis (post-opt32 `pl3_opt32.pprof`)
- `buildConvexResources` / vertex `queueWriteBuffer` **~120ms** still largest WriteBuffer slice
- SkipAA mesh coverage is always **1.0** after opt30 color compact — paying 4B/vert for a constant
- Pure mesh batches dominate PKS particle/mesh flushes

### Engine change (class B — layout + shader)
1. **`convexMeshVertexStride=12`**: `pos2 + unorm8x4 color` (no coverage channel)
2. **`packMeshVertsCoverage1` / `writeConvexMeshVertex`** pack 12B; `vs_mesh` hardcodes coverage=1
3. **`meshPipelineWithStencil` / depth-clip mesh** via `vs_mesh` + mesh vertex layout
4. **`allConvexCommandsMeshCompact`**: pure mesh + BlendNormal → 12B WriteBuffer + mesh pipeline
5. Mixed frames: expand 12B→16B into AA layout (coverage=1) so existing convex pipeline still works

Semantic: SkipAA mesh coverage was already constant 1.0; solid k/255 colors unchanged; mixed AA path unchanged.

### Unit / pixel gates
| test | result |
|------|--------|
| `TestOpt33_*` | PASS |
| `TestOpt19/22/23/25/30/31` mesh pack | PASS |
| `TestF1_AdvancedLayerPresentView*` | PASS |
| PKS content/pixel | true |

### PKS present (`tmp/pks/cpu_opt/opt33_*.json`, 8s)

| probe | opt32 cpu | **opt33 cpu** | fps_ema | status | cpu_fb | pixel |
|-------|-----------|---------------|---------|--------|--------|-------|
| P_SOLID | 10.0 | **9.1** | 58.6 | PASS | 0 | true |
| P_GLOW | 11.9 | 11.6 | 58.1 | PASS | 0 | true |
| P_BLEND_GLOW | 17.4 | 17.3 | 58.5 | PASS | 0 | true |
| P_L3 | 19.6 | **19.6** | 58.7 | PASS | 0 | true |
| P_LAYER | 10.2 | 10.2 | 58.2 | PASS | 0 | true |

### pprof structure (P_L3 8s)
| metric | opt32 | **opt33** |
|--------|-------|-----------|
| buildConvexResources cum | 130ms (6.8%) | **70ms (3.8%)** |
| convex vertex WriteBuffer | ~120ms | **~70ms** |
| mesh vert bytes | 16B | **12B (−25%)** |

### Policy
- **Keep** (structure win on WriteBuffer; SOLID −0.9pp; F1/PKS pixel green)
- L3 wall CPU flat vs opt32 (other costs dominate); still worth Keep as mesh bandwidth cut
- Revert if AA/mesh mixed frames pixel-regress (expand path)
- Next: filter pass-uniform slab / remaining Submit tax; avoid further mesh micro without new pressure

### Evidence
- `tmp/pks/cpu_opt/opt33_*.json`, `pl3_opt33.pprof`
- Tests: `render/internal/gpu/opt33_mesh_compact_stride_test.go`
- Shader: `render/internal/gpu/shaders/convex.wgsl` `vs_mesh`

## opt34 Skip FlushLeading under sharedEncoder (2026-07-17)

### Diagnosis (post-opt33 `pl3_opt33.pprof`)
- `FlushLeadingSubmitsOnly` ~8% / `Queue.Submit` ~13.5% still high on L3
- Root cause: opt32 composite sets `sharedEncoder` but `RenderFrameGrouped` drained
  `leadSubmitCBs` whenever `!deferSurfaceSubmit`, forcing a mid-frame Submit of
  layer fills alone; dual-tex + blit then submitted separately
- Intended ADR-017 / opt32 path: layers + dual + blit = **one** `submitWithLeading`

### Engine change (class A — pure submit ordering)
1. `RenderFrameGrouped`: FlushLeading only when `sharedEncoder == nil`
   (`!defer && sharedEncoder == nil && leads > 0`)
2. Shared-encoder path leaves leads queued; caller Finish + `submitWithLeading`
3. No sharedEncoder present path still drains (opt21 MSAA attachment safety)
4. `encodeSubmitSurfaceGrouped` uses static `sessionSurfaceEncoderDesc` +
   `EncodersCreated++` after successful create

Semantic: same command buffers / same content; fewer mid-frame Queue.Submit only.

### Unit / pixel gates
| test | result |
|------|--------|
| `TestOpt34_SharedEncoder_DoesNotFlushLeading` | PASS (leads kept; CoalescedCBs=3) |
| `TestOpt34_NoSharedEncoder_StillFlushesLeading` | PASS (leads drained; surface CoalescedCBs=1) |
| `TestOpt21_*` / `TestOpt32_*` / `TestR73_*` | PASS |
| `TestF1_AdvancedLayerPresentView*` | PASS |
| PKS content/pixel | true |

### PKS present (`tmp/pks/cpu_opt/opt34_*.json`, 8s)

| probe | opt33 cpu | **opt34 cpu** | fps_ema | status | cpu_fb | pixel |
|-------|-----------|---------------|---------|--------|--------|-------|
| P_SOLID | 9.1 | 10.2 | 58.7 | PASS | 0 | true |
| P_GLOW | 11.6 | 12.1 | 58.8 | PASS | 0 | true |
| P_BLEND_GLOW | 17.3 | 17.5 | 58.7 | PASS | 0 | true |
| P_L3 | 19.6 | **19.8** | 59.3 | PASS | 0 | true |
| P_LAYER | 10.2 | **9.4** | 59.0 | PASS | 0 | true |

### pprof structure (P_L3 8s)
| metric | opt33 | **opt34** |
|--------|-------|-----------|
| FlushLeadingSubmitsOnly cum | **150ms (8.1%)** | **~0 (gone from top)** |
| Queue.Submit cum | 250ms (13.5%) | **140ms (7.7%)** |
| submitWithLeading cum | 230ms (12.4%) | **140ms (7.7%)** |
| RenderFrameGrouped cum | 810ms (43.8%) | **650ms (35.9%)** |
| wall samples | 1.85s | 1.81s |

### Policy
- **Keep** (structure: mid-frame FlushLeading on composite path removed; Submit tax cut; F1/PKS pixel green)
- Wall CPU flat/noise vs opt33 (±1pp) — same class as opt33 Keep-as-structure
- Revert if advanced-layer composite ordering / present content regresses
- Next: filter pass-uniform **slab** (2–4 WriteBuffers/blur → one); enlarge convex ring only if `allocConvexVertSlot` still forces FlushLeading; avoid mesh micro without new pressure

### Evidence
- `tmp/pks/cpu_opt/opt34_*.json`, `pl3_opt34.pprof`
- Tests: `render/internal/gpu/opt34_shared_encoder_no_flush_leading_test.go`
- Code: `render/internal/gpu/render_session.go` (`RenderFrameGrouped` opt34 gate)

## opt35 filter pass-uniform slab (2026-07-17)

### Diagnosis (post-opt34)
- Blur/glow multi-pass wrote **one uniform buffer per pass** (`queue.WriteBuffer` × 2–4+)
- Same cgo tax pattern as pre-opt29 image uniforms; payload is only 128B/pass
- L3 `ApplyBlur` ~6% / total WriteBuffer ~10%; filter graph itself small but every glow frame pays N uniform uploads

### Engine change (class A — pure upload coalesce)
1. **`filterPassUniformSlotStride=256`** (WebGPU minUniformBufferOffsetAlignment)
2. Single **`passUniformSlab`** on `filterGPUCache`; pack all pass `Params` into CPU scratch
3. Bind groups use **fixed Offset** into the slab (opt29 style; no dynamic-offset BGL change)
4. **One** `WriteBuffer` for used slots after encode, before `Finish`/`Submit`
5. `filterBGKey` includes slab offset; slab recreate clears BG cache
6. Pre-size estimate `len(nodes)*4+2` (min 16 slots) so steady-state glow does not reallocate mid-run

Semantic: same Params layout/shader; same per-pass bind content; fewer Queue.WriteBuffer calls only.

### Unit / pixel gates
| test | result |
|------|--------|
| `TestOpt35_FilterPassUniformSlotStride_Aligned` | PASS |
| `TestOpt35_FilterPassUniformSlab_OneWriteForMultiPassBlur` | PASS (slots≥2, WB=1) |
| `TestOpt35_FilterPassUniformSlab_ShadowMultiPass` | PASS |
| `TestP04_ApplyBlurGPU` / `TestP04_ApplyDropShadowGPU` | PASS |
| `TestF1_AdvancedLayerPresentView*` | PASS |
| PKS content/pixel | true |

### PKS present (`tmp/pks/cpu_opt/opt35_*.json`, 8s)

| probe | opt34 cpu | **opt35 cpu** | fps_ema | status | cpu_fb | pixel |
|-------|-----------|---------------|---------|--------|--------|-------|
| P_SOLID | 10.2 | 10.2 | 58.0 | PASS | 0 | true |
| P_GLOW | 12.1 | **12.0** | 58.7 | PASS | 0 | true |
| P_BLEND_GLOW | 17.5 | 17.7 | 57.8 | PASS | 0 | true |
| P_L3 | 19.8 | **19.6** | 58.8 | PASS | 0 | true |
| P_LAYER | 9.4 | **9.0** | 58.5 | PASS | 0 | true |

### pprof structure (P_L3 8s)
| metric | opt34 | **opt35** |
|--------|-------|-----------|
| ApplyBlur cum | 110ms (6.1%) | **80ms (4.2%)** |
| Queue.WriteBuffer cum | 180ms (9.9%) | **140–150ms (~7.9%)** |
| runGPUFilterGraphEx cum | 40ms | 50ms (noise) |
| pass uniform WB / blur | N (H+V+…) | **1** (unit-gated) |

### Policy
- **Keep** (class A structure: N filter uniform WriteBuffers → 1; F1/P04/PKS pixel green)
- Wall CPU flat/noise vs opt34 — expected for small payload; still worth Keep as glow-path cgo cut
- Revert if multi-pass filter pixel/content regresses (wrong slot offset / mid-run slab grow)
- Next: remaining WriteBuffer (convex/mesh already opt33); Submit/encode still dominate L3 — avoid mesh micro; only enlarge convex ring if `allocConvexVertSlot` forces FlushLeading again

### Evidence
- `tmp/pks/cpu_opt/opt35_*.json`, `pl3_opt35.pprof`
- Tests: `render/internal/gpu/opt35_filter_pass_uniform_slab_test.go`
- Code: `render/internal/gpu/filter_gpu_graph.go` (opt35 slab)

## opt36 mesh-seed + filter one encoder Finish (2026-07-17)

### Diagnosis (post-opt35 `pl3_opt35.pprof`)
- `CommandEncoder.Finish` still **~250ms (13–14%)** on L3
- Glow path (`FlushAndFilterFromView` / opt18): mesh seed **Finish** + filter graph **Finish**, then one `Queue.Submit([mesh, filter])`
- Same GPU work; the second Finish is pure encode tax (class A target)

### Engine change (class A — pure encoder coalesce)
1. `runGPUFilterGraphEx(..., sharedEnc)` continues filter passes on an open encoder
2. `runGPUFilterGraphFromViewIntoEncoder` — mesh seed already recorded on `sharedEnc`
3. `FlushAndFilterFromView`: encode mesh via `sharedEncoder`, **do not Finish**, continue filter on same encoder → **one Finish** + one Submit
4. Failure recovery: Finish+Submit open encoder (applies mesh seed), then standalone `FromView`
5. Static encoder descriptors: `filterSeedMeshEncoderDesc` / `filterGPUBatchEncoderDesc` / `filterGPUReadEncoderDesc`
6. Diagnostics: `lastGraphFinishes`, `lastUsedSharedEnc` (unit-gated)

Semantic: same mesh seed draws + same filter graph; fewer Finish only (opt18 already single Submit).

### Unit / pixel gates
| test | result |
|------|--------|
| `TestOpt36_FilterSeedSharedEncoder_OneFinish` | PASS (sharedEnc, Finishes=1, slots≥2) |
| `TestOpt36_FilterFromView_StillOneFinish` | PASS |
| `TestOpt36_FilterSeedSharedEncoder_StaticDescs` | PASS |
| `TestOpt35_*` (slab still 1 WB) | PASS |
| `TestOpt18_ApplyBlur_MeshSeedGPUFilter` | PASS |
| `TestP04_ApplyBlurGPU` / DropShadow | PASS |
| `TestF1_AdvancedLayerPresentView*` | PASS |
| PKS content/pixel | true |

### PKS present (`tmp/pks/cpu_opt/opt36_*.json`, 8s)

| probe | opt35 cpu | **opt36 cpu** | fps_ema | status | cpu_fb | pixel |
|-------|-----------|---------------|---------|--------|--------|-------|
| P_SOLID | 10.2 | **9.1** | 58.3 | PASS | 0 | true |
| P_GLOW | 12.0 | **11.4** | 58.3 | PASS | 0 | true |
| P_BLEND_GLOW | 17.7 | **17.3** | 59.7 | PASS | 0 | true |
| P_L3 | 19.6 | 19.9 | 58.4 | PASS | 0 | true |
| P_LAYER | 9.0 | 9.9 | 58.1 | PASS | 0 | true |

### pprof structure (P_L3 8s)
| metric | opt35 | **opt36** |
|--------|-------|-----------|
| CommandEncoder.Finish cum | **250–260ms (~13.5%)** | **180–190ms (~10.3%)** |
| ApplyBlur / FlushAndFilter | 80ms | 80ms |
| filter path symbol | `FromViewWithLeading` | **`FromViewIntoEncoder`** (shared path live) |
| wall samples | 1.90s | 1.85s |

### Policy
- **Keep** (class A: glow mesh+filter Finish count 2→1; Finish tax −~3pp; F1/P04/PKS pixel green)
- L3 wall CPU flat/noise; GLOW/BLEND_GLOW modest drop
- Revert if ApplyBlur content wrong / recovery path leaves empty seed RT
- Next: encode/Submit still dominate non-glow; avoid mesh micro; do not re-open full-frame singleSubmit without dest-correct dual-tex proof; convex ring 4→8 only if `allocConvexVertSlot` FlushLeading returns as material tax

### Evidence
- `tmp/pks/cpu_opt/opt36_*.json`, `pl3_opt36.pprof`
- Tests: `render/internal/gpu/opt36_filter_seed_shared_encoder_test.go`
- Code: `filter_flush_coalesce.go`, `filter_gpu_graph.go` (opt36 sharedEnc)

## opt37 dual-tex multi uniform slab (2026-07-17)

### Diagnosis (post-opt36 `pl3_opt36.pprof`)
- Advanced-blend multi path (`dualTexAdvancedBlendViewsMultiIntoEncoder`) used **per-op uniform buffers** (`uniformRing`) + `dualTexWriteParams` each → **N WriteBuffers** for N layers
- L3 has Screen+Multiply (2 ops); same cgo tax pattern as pre-opt35 filter uniforms
- `dualTexWriteParams` ~1% + MultiInto ~2.7%; total WriteBuffer ~11%

### Engine change (class A — pure upload coalesce)
1. Replace `uniformRing []*Buffer` with **`uniformSlab`** (stride `dualTexUniformSlotStride=256`)
2. `packDualTexParams` into CPU scratch for all ops → **one** `queue.WriteBuffer`
3. `multiBindGroup` takes **offset** into slab; `dualTexBGKey` includes offset
4. Slab recreate invalidates multiBG slots
5. Single-op paths still use `cache.uniform` + `dualTexWriteParams`
6. Diagnostics: `lastMultiUniformSlots` / `lastMultiUniformWB`

Semantic: same Params layout / same dual-tex passes; fewer WriteBuffer calls only.

### Unit / pixel gates
| test | result |
|------|--------|
| `TestOpt37_DualTexUniformSlotStride_Aligned` | PASS |
| `TestOpt37_DualTexMultiUniformSlab_OneWrite` | PASS (slots=2, WB=1) |
| `TestOpt26_*` multiBindGroup reuse | PASS |
| `TestOpt32_*` / `TestR73_*` / `TestOpt34_*` | PASS |
| `TestF1_AdvancedLayerPresentView*` | PASS |
| PKS content/pixel | true |

### PKS present (`tmp/pks/cpu_opt/opt37_*.json`, 8s)

| probe | opt36 cpu | **opt37 cpu** | fps_ema | status | cpu_fb | pixel |
|-------|-----------|---------------|---------|--------|--------|-------|
| P_SOLID | 9.1 | 9.0 | 58.3 | PASS | 0 | true |
| P_GLOW | 11.4 | 12.3 | 57.8 | PASS | 0 | true |
| P_BLEND_GLOW | 17.3 | **16.8** | 58.8 | PASS | 0 | true |
| P_L3 | 19.9 | **19.1** | 58.6 | PASS | 0 | true |
| P_LAYER | 9.9 | **9.3** | 58.1 | PASS | 0 | true |
| P_BLEND_LAYER | — | **14.5** | 59.3 | PASS | 0 | true |

### pprof structure (P_L3 8s)
| metric | opt36 | **opt37** |
|--------|-------|-----------|
| Queue.WriteBuffer cum | 210ms (11.4%) | **170ms (9.1%)** |
| dualTexWriteParams | 20ms (1.1%) | **gone from top** |
| MultiIntoEncoder cum | 50ms (2.7%) | **30ms (1.6%)** |
| resolvePendingAdvancedLayersEnc | 270ms (14.6%) | **190ms (10.2%)** |

### Policy
- **Keep** (class A structure: N dual-tex uniform WB → 1; L3 −0.8pp; blend probes green)
- GLOW ±1pp noise (no dual-tex multi benefit)
- Revert if advanced-blend pixel/content wrong (bad slab offset / BG cache key)
- Next: remaining WriteBuffer is mostly **animated mesh verts** (opt33 already compact); encode/Submit/Finish still dominate — avoid mesh micro; no full-frame singleSubmit without dest-correct dual-tex proof; convex ring only if ring FlushLeading material again

### Evidence
- `tmp/pks/cpu_opt/opt37_*.json`, `pl3_opt37.pprof`
- Tests: `render/internal/gpu/opt37_dualtex_uniform_slab_test.go`
- Code: `render/internal/gpu/dual_tex_blend.go` (opt37 slab)

## opt38 ensurePipelines warm fast-path (2026-07-17)

### Intent (class A)
`ensurePipelines` runs every `RenderFrameGrouped` (often multi-session/frame on L3 glow).
Add a warm fast-path when clip/mask layouts are stable and core pipelines already exist.

### Engine change
| item | detail |
|------|--------|
| `pipelinesReady` + clip/mask identity | skip body when ready |
| Invalidate | `Destroy`, `SetSDF/Convex/Stencil`, `MarkOwnsShapePipelines` |
| Counters | `ensurePipelinesFastN` / `ensurePipelinesFullN` + `lastEnsurePipelines` |

### Unit gates
| test | result |
|------|--------|
| `TestOpt38_EnsurePipelines_ReadyFastPath` | PASS (warm last=0, FastN≥1) |
| `TestOpt38_EnsurePipelines_WarmRenderFrame` | PASS |
| `TestOpt37_*` / `TestOpt34_*` / `TestOpt21_*` | PASS |
| `TestF1_AdvancedLayerPresentView*` | PASS |
| `TestP04_ApplyBlurGPU` | PASS |

### PKS present (`tmp/pks/cpu_opt/opt38c_*.json`, 8s)

| probe | opt37 cpu | **opt38c cpu** | fps_ema | status | cpu_fb | pixel |
|-------|-----------|----------------|---------|--------|--------|-------|
| P_SOLID | 9.0 | **8.6** | 58.6 | PASS | 0 | true |
| P_GLOW | 12.3 | **11.2** | 62.1 | PASS | 0 | true |
| P_BLEND_GLOW | 16.8 | **18.1** | 57.7 | PASS | 0 | true |
| P_L3 | 19.1 | **19.3** | 60.2 | PASS | 0 | true |
| P_LAYER | 9.3 | **8.7** | 59.2 | PASS | 0 | true |

### pprof structure (P_L3 8s, `pl3_opt38c.pprof`)
| metric | notes |
|--------|-------|
| `ensurePipelines` cum | ~60ms / ~3.3% — **cold multi-session create** (effect `preferSampleCount1` owns pipelines; stencil/convex first create samples) |
| Fast path | unit-proven sticky; no per-frame WGSL thrash (opt6 thrash already fixed by own pipelines) |
| L3 wall CPU | flat vs opt37 (noise) |

### Policy
- **Keep** (class A cheap guard; unit-gated; no pixel/content regression)
- Honest: **no L3 wall-CPU structure win** beyond noise; residual samples are cold full ensures, not warm-path miss thrash
- Revert only if clip/mask mismatch leaves stale pipelines (would break stencil/mask draws)
- Next knife: Finish/Submit still dominate L3 — avoid mesh micro; dual-tex into-dest WB cleanup optional (dead on MultiInto)

### Evidence
- `tmp/pks/cpu_opt/opt38c_*.json`, `pl3_opt38c.pprof`
- Tests: `render/internal/gpu/opt38_ensure_pipelines_ready_test.go`
- Code: `render/internal/gpu/render_session.go` (opt38 ensurePipelines)


## HUD text multi-page atlas fix (2026-07-17)

### Symptom
Top/bottom window HUD text correct at start, later garbled / not real glyphs.
(Prior layout-template × compact UV fix remains required; this is residual after page0 fills.)

### Root cause
`layoutGlyphs` hardcoded `AtlasPageIndex: 0` while the R8 atlas allows `MaxAtlases=4`.
After page 0 shelf fills (CJK + Latin + subpixel variants under particle/HUD load), new glyphs land on page ≥1 with **page-local UVs**, but the GPU bind group still sampled **page 0** → wrong masks.

### Fix (class A correctness)
| file | change |
|------|--------|
| `glyph_mask_pipeline.go` | `GlyphMaskQuad.Page`; `SplitGlyphMaskBatchByPage` (consecutive page runs) |
| `glyph_mask_engine.go` | set `Page`/`AtlasPageIndex` from `region.AtlasIndex`; template hit touches **all** pages |
| `gpu_render_context.go` | `queueGlyphMaskSplit` on DrawGlyphMask* paths |

### Gates
| test | result |
|------|--------|
| `TestSplitGlyphMaskBatchByPage_SingleAndMulti` | PASS |
| `TestLayoutText_SetsAtlasPageFromRegion` (64px atlas spill) | PASS |
| `TestR75_LayoutTemplate_*` / compact+Touch | PASS |
| `TestF1_AdvancedLayerPresentView*` | PASS |
| PKS `P_L3` 8s | PASS content/pixel `cpu_fb=0` |

### Policy
- Keep multi-page Page metadata + split-before-queue
- Keep generation + TouchPage (prior HUD UV fix)
- Do not hardcode page 0 again

### Evidence
- Tests: `render/internal/gpu/glyph_mask_multipage_test.go`
- Code: `glyph_mask_pipeline.go`, `glyph_mask_engine.go`, `gpu_render_context.go`

## opt39 classic convex vert sticky + singleSubmit note (2026-07-17)

### Intent
Post-opt38 L3 hotspots: Finish/Submit/WriteBuffer. Tried two class-A directions:

1. **Full-frame singleSubmit** (base + dual-tex MultiInto + blit, one Finish)
2. **Convex vertex sticky WriteBuffer** (opt28-style)

### 1) Full-frame singleSubmit — NOT enabled
- Code path exists (`f1_frame_enc` + `sharedEncoder`) but enabling regressed `TestF1_AdvancedLayerPresentView*`:
  - Multiply → white center
  - Screen → black
- Kept `singleSubmit := false`
- **Kept** resolve fix: when external `enc != nil`, composite out→scratch blit encodes into `enc` without nested Finish (needed if singleSubmit ever re-enabled with dest-correct proof)
- Finish/Submit end paths use `submitWithLeading` (coalesce opt21 layer leads) when singleSubmit is used later

### 2) Convex vertex sticky — Keep (narrow)
| change | detail |
|--------|--------|
| `allocConvexVertSlot` | when `deferredConvexUses==0` and no lead CBs → stay on **slot 0** |
| sticky WriteBuffer | **classic (non-meshCompact) only** — fingerprint skip when slot payload matches |
| meshCompact | always WriteBuffer (animated mesh always-miss; fingerprint would be O(n) tax on L3) |

### Unit gates
| test | result |
|------|--------|
| `TestOpt39_ConvexVertSticky_SkipsRepeatWrite` | PASS |
| `TestOpt39_ConvexVertSticky_Slot0WhenNoDeferred` | PASS |
| `TestOpt24_*` / `TestOpt28_*` / `TestOpt34_*` / `TestOpt37_*` | PASS |
| `TestF1_*` | PASS (singleSubmit still off) |

### PKS present (`tmp/pks/cpu_opt/opt39b_*.json`, 8s)

| probe | opt38c cpu | **opt39b cpu** | fps_ema | status | cpu_fb | pixel |
|-------|------------|----------------|---------|--------|--------|-------|
| P_SOLID | 8.6 | **9.5** | 58.5 | PASS | 0 | true |
| P_L3 | 19.3 | **19.6** / 19.3 prof | 58.3 | PASS | 0 | true |
| P_LAYER | 8.7 | **9.2** | 58.2 | PASS | 0 | true |
| P_BLEND_LAYER | — | **13.9** | 59.2 | PASS | 0 | true |

### pprof structure (P_L3)
| metric | opt38c | **opt39b** |
|--------|--------|------------|
| Queue.WriteBuffer cum | 160ms (8.9%) | **120ms (6.7%)** |
| buildConvexResources | 80ms (4.5%) | 80ms (4.4%) |

### Policy
- **Keep** slot0 + classic convex sticky (class A; unit-proven; no F1 regress)
- **Do not** fingerprint sticky meshCompact (L3 pure tax)
- **Do not** re-enable full-frame singleSubmit without dest-correct dual-tex proof + F1 green
- Wall L3 CPU noise; structure WB slightly down
- Next: Finish/Submit still dominate — only re-open singleSubmit with pixel proof; avoid mesh micro

### Evidence
- `tmp/pks/cpu_opt/opt39b_*.json`, `pl3_opt39b.pprof`
- Tests: `render/internal/gpu/opt39_convex_vert_sticky_test.go`
- Code: `render_session.go` (slot0 + sticky), `gpu_render_context.go` (external-enc resolve)

## opt40 gpu-tex uniform slab + drawCall scratch (2026-07-17)

### Intent
Mirror **opt29 image uniform slab** onto the GPU-texture (glow/filter publish) path:
1. Pack all coalesced gpu-tex opacity/viewport uniforms into **one grow-only slab** → single `Queue.WriteBuffer` per flush when any slot dirty
2. Reuse `gpuTexDrawCallScratch` for coalesced draw records (cut per-flush alloc)

Class A structure: same bind groups/draws/pixels; only upload/alloc path.

### Code
| change | detail |
|--------|--------|
| `render_session.go` | `gpuTexUniformSlab` / `gpuTexUniformSlabCap` / scratch; pack N slots → 1 WB |
| `getOrCreate(..., uniformOffset, ...)` | bind group uses slab + dynamic offset (or fixed offset layout) |
| `gpu_render_context.go` | packMesh uses `packColorUnorm8x4RGBA` (shared pack helper) |
| Legacy per-slot uniform buffers | remain nil after opt40; Destroy nils slice |

### Unit gates
| test | result |
|------|--------|
| `TestOpt40_GPUTexUniformSlab_*` | PASS — N slots → 1 slab WB; identical rebuild 0 WB; opacity change 1 slab WB |
| `TestOpt27_*` | PASS (updated for new `getOrCreate` signature) |
| `TestF1_AdvancedLayerPresentView*` | PASS |

### PKS present (`tmp/pks/cpu_opt/opt40_*.json`, 8s)

| probe | prior cpu (opt39b where avail) | **opt40 cpu** | fps_ema | status | cpu_fb | pixel |
|-------|--------------------------------|---------------|---------|--------|--------|-------|
| P_SOLID | 9.5 | **8.3** | 58.2 | PASS | 0 | true |
| P_GLOW | 11.8† | **11.9** | 57.7 | PASS | 0 | true |
| P_BLEND_GLOW | 17.1† | **17.0** | 59.1 | PASS | 0 | true |
| P_L3 | 19.6 | **19.3** / **18.8** prof | 60.6 / 58.9 | PASS | 0 | true |
| P_LAYER | 9.2 | **10.3** | 58.2 | PASS | 0 | true |
| P_BLEND_LAYER | 13.9 | **13.7** | 58.8 | PASS | 0 | true |

† prior from handoff when opt39b artifact missing for that probe.

### pprof structure (P_L3) — noisy vs opt39b
| metric | opt39b | **opt40** |
|--------|--------|-----------|
| WriteBuffer cum | 120ms (6.7%) | 210ms (11.9%) — sample noise / mix |
| buildGPUTextureResources | 30ms (1.7%) | 60ms (3.4%) |
| Finish | 270–290ms | **230ms (13%)** |
| packMeshVertsCoverage1 | 90ms (5%) | **60ms (3.4%)** |

Unit gate is the structure proof (N→1 slab WB). Wall L3 flat/slightly better; WriteBuffer % noisy.

### Policy
- **Keep** as class A structure (unit-proven; F1 green; PKS green; L3 wall flat/slightly better)
- Honest: L3 wall not a large structure win; pprof WriteBuffer % noisy
- **Revert** if gpu-tex pixels wrong (slab offset / BG cache)
- Do **not** open full-frame singleSubmit without dest-correct dual-tex + F1 green
- Next: Finish/Submit still dominate; remaining WB is animated mesh verts (avoid micro/fingerprint); optional dual-tex into-dest dead path cleanup

### Evidence
- `tmp/pks/cpu_opt/opt40_*.json`, `pl3_opt40.pprof`
- Tests: `render/internal/gpu/opt40_gputex_uniform_slab_test.go`
- Code: `render_session.go` (gpu-tex slab), `gpu_render_context.go` (pack helper)


## Filter/Present coherence (D105/D133/D140/D152) — 2026-07-17

### Symptom
D01–D200 static composition had 3 FAIL after opt40 Keep:
- `TestP1_Comp_D105_KitchenSinkV3Stress` — mid-frame content wiped by filter seed
- `TestP1_Comp_D133_FilterGraphChainComposition` — post-filter draws missing from Image
- `TestP1_Comp_D140_KitchenSinkV4Stress` — Multiply + filter blanked pre-blend surface
- `TestP1_Comp_D152_PresentFrameDamageMultiRect` — Present vs Image white false-green (asserts tightened)

### Root cause (render library, not PKS examples)
| case | cause |
|------|--------|
| D105 | Mid-frame `FlushGPU` then more draws then filter → path-1 Clear+pending-only seed |
| D133 | GPU filter publish left `pixmapFilterStale`; later draws never materialised into `Image()` |
| D140 | Advanced Multiply calls `rc.Flush(target)` (not Context.FlushGPU) → pixmap has pre-blend; remaining pending only |
| D152 | Present wrote view; `Image()/SavePNG` read white pixmap; weak asserts (`r>=100`) green on white |

### Fix (class A correctness)
| change | detail |
|--------|--------|
| B post-filter draws | `syncPublishedFilterBeforeDraw()` before fill/stroke/text/image/gpu-tex |
| A mid-frame seed | `midFrameNilFlush` + `GPURenderContext.flushedPendingToData`; mid-frame → fold pending + path-2 pixmap seed |
| hot path | **Removed** unconditional path-1 `seedFilterSrcFromPixmap` (avoids full-surface WriteTexture every glow frame) |
| C Present/Image | `markViewFlush` only from `PresentFrame*`; `syncViewFlushIntoPixmap` on Image/SavePNG/Export |
| D152 asserts | reject pure white |

### Gates
| gate | result |
|------|--------|
| `TestFilterGraph_*` / `TestPresentDamage_ImageMatchesView` | PASS |
| D105 / D133 / D140 / D152 | PASS |
| Full `TestP1_Comp_` (201) | **0 FAIL** (`tmp/pks/cpu_opt/d01_d200_verify_now.json`) |
| PKS after_fix P_SOLID/P_GLOW/P_L3 8s | PASS, `cpu_fb=0`; FPS/CPU within opt40 noise |

### Policy
- **Keep** correctness fix on top of opt40
- Do **not** re-enable always-on path-1 full-surface pixmap seed (perf regress on glow)
- Mid-frame / post-filter / Image readback costs only on rare correctness paths
- Optional later: delete unused `seedFilterSrcFromPixmap` helper

### Evidence
- Code: `render/filter_ops.go`, `render/context.go`, `render/present.go`, `render/internal/gpu/gpu_render_context.go`
- Tests: `render/filter_present_coherence_test.go`
- Logs: `tmp/pks/cpu_opt/fix_final.log`, `d01_d200_verify_now.json`, `after_fix_P_{SOLID,GLOW,L3}.json`

## Mem platformization (2026-07-18) — residual closed

### Fixes landed (engine)

1. **Swapchain surface texture**: `WGPUSurfaceTexture` is ReturnedWithOwnership — `Present`/`DiscardFrame` release texture (was leaking).
2. **Command buffer retain**: window path now calls `session.BeginFrame()` so `prevCmdBufs` drain; dig shows `prev_cb=1` stable.
3. **Dynamic HUD text residual** (after surface/CB fixed, solid+dynamic HUD still ~300 KB/s):
   - `text.IsHighChurnLabel` — skip shape/layout caches for FPS/RSS/frame telemetry strings
   - MSDF: atlas `Lookup` before `ExtractOutline`; layout template for stable strings; grow-only text staging
   - Glyph-mask: skip layoutTemplatePut for high-churn labels

### Gate

- Leak = **platformization only**: `rate = rss_steady_delta_kb / (seconds×0.8) ≤ GPUI_MEM_PLATEAU_RATE_KB_S` (default 256 KB/s; target ≈0)
- No absolute MiB climb gate (optional `GPUI_MEM_RSS_HARD_KB` OOM safety only)

### Tier soak evidence (`tmp/mem_tier_soak/`)

| Tier | Duration | Status | rate KB/s | FPS | CPU |
|------|----------|--------|-----------|-----|-----|
| P_MEM_SOAK | 60s | PASS | 41.6 | 58.4 | 18 |
| P_MEM_LONG | 180s | PASS | 6.4 | 57.4 | 18 |
| P_MEM_LONG | 600s | PASS | 3.5 | 57.2 | 18 |
| P_MEM_LONG | 900s | PASS | **2.13** | 58.6 | 18 |

600s offline segments early/mid/late ≈ 7.5 / 6.2 / 2.5 KB/s (late lowest → no delayed-release signature).

### Related digs

- solid + `GPUI_NO_HUD` / `GPUI_STATIC_HUD`: flat before text fix
- solid + dynamic HUD after fix: PASS
- baseline full L3 60s / blend 60s: PASS (`tmp/mem_hud_text_fix/`)

### Still out of scope / optional

- 15–30 min release soak (`GPUI_ANIM_SECONDS=900|1800`) for ultra-slow climb
- Duration honesty: **60s diagnoses fast leaks (≥~100–256 KB/s)**; does **not** alone prove no slow climb. Layer: 60 → 180 → 600 → 900+.
- Harness FPS honesty (2026-07-18): present-to-present only; skip interval after intermittent `stageContentSignature` dig; `fps_jitter` = **p95−p5** (raw min/max kept).

### 900s release soak (`tmp/mem_release_900/`) — 2026-07-18
- **PASS** rate=**2.13** KB/s delta=1534KB / 900s; early/mid/late=**3.69 / 1.36 / 1.38** (late≈mid → platform, no delayed-release)
- fps_ema≈58.6 cpu≈18.2; note: this binary predated harness jitter fix so `fps_jitter` whole-run max−min inflated (warn only)
- Release-tier mem platformization **complete** (optional 1800s only if ultra-slow still suspected)

### Harness FPS honesty verify (`tmp/jitter_honesty/`)
- present→present; skip dig-following interval; `fps_jitter`=p95−p5
- P_L3 20s: PASS jit=**6.2** min=46 max=80 low=0 warn=none cpu≈18
- P_SOLID 15s: PASS jit=**2.1** min=49 max=71 low=0 warn=none cpu≈8.6

- Precise VRAM accounting
- Glow/L3 hitch (perf, not leak) remains separate track


### Daily guard + glow baseline (same day)

- `run_mem_guard.sh daily` (SKIP_UNIT=1, COUNT=1): **DONE fail=0**
  - mem matrix pass=5; P_MEM_LONG 180s rate≈8.4 KB/s; P_SOLID/P_BLEND_LAYER 60s PASS
  - evidence: `tmp/mem_guard_daily_now/`
- Glow/L3 20s baseline (`tmp/perf_glow_next/`): all PASS, hitch low, CPU 8–18%
  - Next perf knife (optional): L3 jitter / filter bind reuse — not blocking mem track

## opt41 surface RP reuse + warm ensure (2026-07-18)

### Intent (class A — R8.2)
1. Reuse `GPURenderSession` surface render-pass descriptor/attachments (no per-encode `[]ColorAttachment` / DS alloc)
2. Warm `Flush` / `FlushAndFilterFromView`: skip `ensureGPU` body when device already live; only `registerFilterGraphIfNeeded` once

### Code
- `render/internal/gpu/render_session.go` — `surfaceRenderPassDesc`, used by surface encode paths
- `render/internal/gpu/gpu_render_context.go` — warm ensure
- `render/internal/gpu/filter_flush_coalesce.go` — same
- `render/internal/gpu/opt41_surface_rp_reuse_test.go` — warm allocs=0

### PKS 20s vs baseline (`tmp/perf_fwd_opt41/`)

| probe | base fps/cpu/hitch | **opt41** fps/cpu/hitch | jit | status |
|-------|--------------------|-------------------------|-----|--------|
| P_SOLID | 58.0 / 8.7 / 0.0019 | **58.0 / 9.0 / 0** | 1.9 | PASS |
| P_GLOW | 57.8 / 10.6 / 0 | 57.2 / 10.9 / 0 | 5.0 | PASS |
| P_BLEND_GLOW | 58.0 / 15.9 / 0.0009 | **58.1 / 15.4 / 0** | 5.8 | PASS |
| P_L3 | 57.2 / 18.5 / 0.0028 | **57.9 / 18.0 / 0.0009** | 5.9 | PASS |
| P_MEM_SOAK 60s | — | rate=43.9 KB/s PASS | 6.1 | PASS |

### Policy
- **Keep** — structure proof (0-alloc RP desc); L3 fps↑ cpu↓ hitch↓; no content gut; `cpu_fb=0`; mem platform guard green
- Wall win modest (noise band on SOLID/GLOW ±0.3pp CPU); main residual still **cgo Finish/Submit/WriteBuffer**
- Next knife: still from pprof — advanced resolve / filter encoder (not mesh micro)

### Evidence
- `tmp/perf_fwd_opt41/baseline/`, `tmp/perf_fwd_opt41/opt41/`
- Plan: `docs/PERF_CPU_FORWARD_OPT_PLAN.md`

## opt42 stringToStringView 0-alloc + dual-tex scratch (2026-07-18)

### Intent (class A — R8)
1. `gpu/rwgpu.stringToStringView`: stop `[]byte(s)` copy; use `unsafe.StringData` (label hot path every Begin/Create)
2. dual-tex resolve: reuse `dualTexOpsScratch` / `dualTexViewOpsScratch` on `GPURenderContext`

### Code
- `gpu/rwgpu/descriptors.go` + `opt42_string_view_test.go` (warm allocs=0)
- `render/internal/gpu/gpu_render_context.go` + `opt42_dualtex_scratch_test.go`

### PKS 20s vs baseline (`tmp/perf_fwd_opt41/`)

| probe | base fps/cpu | opt41 | **opt42** | hitch | status |
|-------|--------------|-------|-----------|-------|--------|
| P_SOLID | 58.0 / 8.7 | 58.0 / 9.0 | **58.1 / 7.9** | 0 | PASS |
| P_GLOW | 57.8 / 10.6 | 57.2 / 10.9 | **57.5 / 10.3** | 0 | PASS |
| P_BLEND_GLOW | 58.0 / 15.9 | 58.1 / 15.4 | **57.5 / 15.4** | 0 | PASS |
| P_L3 | 57.2 / 18.5 | 57.9 / 18.0 | **57.9 / 18.3** | 0 | PASS |
| P_MEM_SOAK 60s | — | rate 43.9 | **rate 37.6 PASS** | — | PASS |

All fps ≥ baseline×97%; CPU ≤ baseline (noise) or better. `cpu_fb=0`.

### Policy
- **Keep** — structure (0-alloc StringView + resolve scratch); SOLID −0.8pp CPU; mem guard green
- Residual still cgo Finish/Submit/WriteBuffer; no full-frame singleSubmit

### Evidence
- `tmp/perf_fwd_opt41/opt42/`

## opt43 hybrid app pace → stable ~60 (2026-07-18)

### Intent (class A — harness / present honesty)
X11+wgpu reports `present_mode=fifo` but **Present returns immediately** (no real vblank wait).
Pure `time.Sleep` budget → systematic **~58 fps**. Uncapped → 140–280 fps + huge jitter.

### Fix (app-side, default ON)
- `GPUI_APP_PACE` default **true** (`GPUI_APP_PACE=0` for dig/uncapped)
- Drift-free `nextFrameDeadline` + resync if >1 frame behind
- Hybrid wait: sleep bulk, **spin last ~1ms** (`runtime.Gosched`) to avoid sleep overshoot
- Intermittent stage sig every **90** frames (was 30) when enabled

### Code
- `examples/particle_kitchen_sink/main.go` — pace loop + log `app_pace` / `frame_budget`
- README: documents `GPUI_APP_PACE`

### PKS 20s clean matrix (`tmp/perf_fwd_opt41/opt43/`, `app_pace=true`)

| probe | opt42 pure-sleep fps/cpu | **opt43 hybrid** fps/cpu | jit | hitch | status |
|-------|--------------------------|--------------------------|-----|-------|--------|
| P_SOLID | 58.1 / 7.9 | **60.03 / 11.5** | 2.1 | 0 | PASS |
| P_GLOW | 57.5 / 10.3 | **60.12 / 12.3** | 3.8 | 0 | PASS |
| P_BLEND_GLOW | 57.5 / 15.4 | **60.53 / 18.3** | 5.4 | 0 | PASS |
| P_L3 | 57.9 / 18.3 | **59.94 / 19.5** | 5.8 | 0 | PASS |
| P_L3_sig | — | **60.12 / 21.2** | 5.5 | 0 | PASS |

`cpu_fb=0` all. Logs: `present_mode=fifo app_pace=true frame_budget=16.666666ms`.

### Policy
- **Keep** — core probes **locked ~60** (ema ≥59.9, avg≈59.95, hitch=0, low jitter)
- Spin costs a few CPU pp vs pure-sleep; **not engine work regression**. Fair engine-CPU compares must use **same** `GPUI_APP_PACE`.
- Invalid prior run (`opt43_mixed_bad/`, app_pace=false light probes → 200+ fps) discarded
- Next: engine knife (R8.3 glow/filter / WriteBuffer pprof) under **same pace** for apples-to-apples CPU

### Evidence
- `tmp/perf_fwd_opt41/opt43/` (+ `SUMMARY.md`)
- Plan: `docs/PERF_CPU_FORWARD_OPT_PLAN.md`

## opt43d step-budget + busy-spin → stable 60+ (2026-07-18)

### Intent
Lock present rate at **≥60** on this X11/wgpu stack where fifo Present does not wait for vblank.

### Fix
1. Default `GPUI_APP_PACE=true`
2. Deadline step = `frameBudget - 200µs` (~60.7 Hz) so measured rate clears 60
3. Sleep bulk + **busy-spin** last ~1ms (no `runtime.Gosched` — was causing ~59.9)

### PKS matrix 25s (`tmp/perf_fwd_opt41/opt43/`)

| probe | fps_ema | fps_avg | cpu | jit | hitch | status |
|-------|---------|---------|-----|-----|-------|--------|
| P_SOLID | **60.70** | **60.69** | 9.5% | 2.1 | 0 | PASS |
| P_GLOW | **61.16** | **60.69** | 12.1% | 3.3 | 0 | PASS |
| P_BLEND_GLOW | **60.74** | **60.69** | 16.8% | 5.0 | 0 | PASS |
| P_L3 | **60.91** | **60.69** | 19.6% | 5.5 | 0 | PASS |
| P_L3_sig | **60.82** | **60.69** | 19.9% | 5.9 | 0 | PASS |
| P_L3 uncapped | **145.8** | 87.4 | 47.6% | — | 0 | PASS (headroom) |

`cpu_fb=0` all. Logs: `app_pace=true frame_budget=16.666666ms`.

### Policy
- **Keep** — **GOAL stable 60+ verified** on core probes (ema & avg ≥60.7, hitch=0)
- Engine headroom uncapped L3 ≈146 fps
- Spin CPU is pace cost; engine knives compare under same `GPUI_APP_PACE`

### Evidence
- `tmp/perf_fwd_opt41/opt43/` (+ `SUMMARY.md`)
