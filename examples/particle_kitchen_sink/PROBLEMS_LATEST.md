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
