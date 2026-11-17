# P6 细 damage 第一卡：脏区合成 + 持久合成面

日期：2026-11-16 ｜ 仓库：`/home/yanghy/app/projects/gogpu/gpui` ｜ 分支：`feat/UI-组件`
状态：**P0（观测）、P1（GL 持久面 + damage 剪刀，默认开启）、P2（场景层树脏区裁剪，默认开启）已落地；P3–P4 未开**。上屏合同不变。
真源：`ENGINE_FRAME_PRESENT_STANDARD.md`（上屏合同）、`RENDER_BASE_RESOURCE_MATRIX.md`（资源账）、
成熟实现：`/home/yanghy/app/projects/gogpu/flutter`（`flow/compositor_context.cc` / `flow/diff_context.cc` / `flow/surface_frame.h`）。

## 0. 一句话

现在“脏区”只走到了上屏：OS 增量提交已经生效，但每帧仍把整棵 retained 层树重新合成一遍。
本卡把 Flutter 的标准做法补到合成侧——**damage 参与层树 Pre-roll/裁剪，只合成脏区碰到的层；其余像素留在持久合成面里**——
目标是把 icon 稳态的 raster p50 2.25ms / CPU 22.60% 单核降下来，上屏协议与门禁一条不改。

**关联卡**：`ENGINE_NODE_RECORD_COVERAGE_PLAN.md`（节点录制覆盖 4/14→14/14 + 接口化——paint 层完备性前置；滚动/拖拽光放成本归本卡合成侧，两卡合并验收 divider 拖拽场景）。

**先查再动（2026-11-16 审计）**：上屏 damage、GL 持久 FBO、层纹理缓存、GPU group scissor 都已有实现；本卡不新造 damage 系统，也不新建 GL 合成面——只补“合成目标持久性没告诉 render 层”和“合成行走没消费 damage”两处断点，另加 wgpu 持久面与计数，逐项见 §3.4。

## 1. 问题与实测（可复现）

环境：桌面 Xorg `:0` + NVIDIA 940MX（驱动 580），4 核机，HEAD `b41ff479`，GL 后端。
上一轮复核已确认：EGL 三件齐备（`swap_with_damage` + `partial_update` + `buffer_age`），
§2.4 严格门直接生效，`spin_min` `os_present_partial=891 / full=5`，icon 稳态 `partial=1121 / full=71`（94%）。
**上屏已经是增量，CPU 仍高**：

| 窗 | 稳态 CPU（单核） | 折四核 | partial / full | damage | raster p50 | flush p50 |
|---|---:|---:|---|---:|---:|---:|
| `spin_min`（64px 旋转） | **14.64%** | 3.66% | 891 / 5 | 7225px² | 16.51ms（其中等 vblank 16.03ms） | 0.38ms |
| `kit/icon` 稳态（只转图标） | **22.60%** | 5.65% | 1121 / 71（94%） | ratio 0.000551（≈529px²） | **2.25ms** | **1.49ms** |
| `kit/icon` 稳态（默认 wgpu，全量） | **24.20%** | 6.05% | 无计数 | — | 2.03ms | 1.05ms |
| `kit/icon` 原样带自动巡览 | 37.67%（整跑） | 9.42% | 108 / 782 | 滚动大损伤 | 3.54ms | 1.86ms |

三条判据把责任钉在合成侧：

1. **上屏不是瓶颈**：GL 94% 帧走 partial present，present wait p50 只有 0.25ms；spin 的 raster p50 16.51ms 里 16.03ms 是在 FIFO 等 vblank（睡，不烧 CPU）。
2. **交换后端不是瓶颈**：默认 wgpu（全量、无损伤提交）24.20% 与 GL（partial 生效）22.60% 同量级。
3. **每帧仍走完整层树**：icon 场景实测 52 个边界层、195 个 picture ops；稳态探针 `boundary_skip 105994 / 1195 帧 ≈ 88.7 次 blit/帧`。529px² 的脏区，实际合成了整窗 52 层。

代码证据（全量合成的直接原因）：

- `render/internal/gpu/render_session.go:1181-1189`：每帧 `BeginFrame` 清空 `renderedViews`，只把 `persistentContentViews`（离屏目标）重新登记；swapchain surface view 不登记（`1381-1388` 注释明确“surface sessions must NOT”）。
- `render/internal/gpu/render_session.go:7114`：`fullSurface := !s.viewRendered(view)` —— surface view 每帧都被当成 fresh，**每帧都走全窗 blit/合成**；damage 剪刀只在 `viewRendered` 为真时才用（`7115-7125`）。
- `ui/scene/textured.go:1463` `CompositeFramePacketTextured`：签名没有 damage 参数；phase 2（`1637-1711`）对每个 PictureLayer 调 `tex.blit` / vector 回放，全部层都进绘制命令；damage 只是走完后产出的 `TexturesStats.DamageRects`。
- `ui/embedder/pipeline_app.go:2124-2196`：`presentPacketTextured` 先 `CompositeFramePacketTextured` 全量合成，再把 `st.DamageRects` 交给 `TrackDamageRect` 和 `PresentWithAuto`——damage 产生在合成之后，无法反过来裁剪合成。
- GPU pass 侧确实有 group 级 damage 跳过（`render_session.go:6816-6825` `applyGroupScissorWithDamageRects`），但那是最后一段：CPU 已经把整棵树走完、draw 命令和 group 资源已经建好，跳过的只是 GPU 绘制。

## 2. 标准做法（Flutter/Skia 证据）

Flutter 的 partial repaint 不是“只把 damage 交给合成器”，而是**damage 同时参与绘制裁剪**：

| 标准件 | Flutter 实现 | 对 gpui 的含义 |
|---|---|---|
| damage 计算 | `flow/diff_context.cc:104-130` `ComputeDamage`：previous layer tree diff + `existing_damage` + readback 扩张；`frame_damage` / `buffer_damage` 分开 | gpui 已有 layer dirty bounds + `PrevBounds`；缺“合成前先算 damage”这一步 |
| 合成裁剪 | `flow/compositor_context.cc:114-153`：`clip_rect = frame_damage->ComputeClipRect(...)`，`layer_tree.Preroll(clip_rect ? *clip_rect : kGiantRect)`，`PaintLayerTreeSkia/Impeller` 里 `canvas()->ClipRect(clip_rect)`（`157-196`） | 本卡核心：把 damage clip 传给层树 Pre-roll，并裁剪 canvas；不在 damage 里的子树直接不画 |
| 全量回退 | `compositor_context.cc:210-226` `ShouldPerformPartialRepaint`：damage 覆盖比例过大（Impeller `kImpellerRepaintRatio = 0.7`）就不做 partial | gpui 已有 0.85 全量门（`render/frame.go:61-62`），复用并实测 |
| 持久内容 | swapchain 后缓冲由驱动保留；`flow/surface_frame.h:41,49-60` `existing_damage` 由 embedder 按 back buffer 记账 | GL 已有持久 FBO（见 §3）；wgpu 需要自建离屏合成面 |
| 上屏 | `SurfaceFrame::SubmitInfo` 的 `frame_damage` / `buffer_damage` → `eglSwapBuffersWithDamage` / `eglSetDamageRegionKHR`；GLFW DRM embedder 是完整样例 | gpui gles 已按此实现（`gpu/gwgpu/gles/queue_linux.go:236-352`），本卡不改 |

**标准边界（必须一起读）**：

- Flutter Linux GTK embedder 目前**没有**接 `existing_damage`（上轮源码核查），Android 也因 issue #123353 关掉了 partial repaint；所以不能拿“Linux 上 Flutter 的 CPU”当本卡的“标准下限”。标准是引擎协议和绘制模型，不是某个 embedder 的现状。
- Flutter 的 `Preroll(clip_rect)` 会把 damage 作为 cull rect：不在 damage 内的层整棵跳过；在 damage 内的层全部绘制（不是只画 dirty 层）。**透明层叠、saveLayer、clip、opacity 的正确性靠“画所有与 damage 相交的层”保证**，不是靠“只画 dirty 层”。本卡照此语义，不允许只画 dirty 层。
- Flutter 的 damage clip 是**单个 union rect**（`buffer_damage`），GPU 侧再按多 rect 做 scissor；gpui 也照此：场景裁剪用 union，上屏用多 rect。

## 3. 现状架构与差距

### 3.1 GL 后端其实已经有持久合成面

- `gpu/gwgpu/gles/surface.go:51-100`：`allocateSwapchainFBO` 分配持久 FBO + color texture（Flutter `fl_framebuffer` 同款，off-screen texture，不是 renderbuffer）。
- `gpu/gwgpu/gles/resource_linux.go:370-390`：`AcquireTexture` 返回的是同一个 FBO 的 `SurfaceTexture` 包装；`gpu/gwgpu/gles/swapchain.go:280-295` 每帧 `CreateTextureView` 生成新 view handle。
- 因此 GL 的 FBO 内容天然跨帧保留，`Queue.Present` 也已经按 buffer age 做 partial present（`queue_linux.go:236-352`，历史 4 帧、85% 门在 `surface_damage_linux.go:20-28`）。

**差距**：render 层不知道这件事。`renderedViews` 按 view handle 记账，而 view 每帧都是新 handle；surface session 又被明确排除在持久登记之外（`render_session.go:1381-1388`）。所以 `fullSurface` 恒为 true，持久 FBO 被每帧全量重写。

### 3.2 wgpu 原生后端没有“稳态”持久面

- `gpu/webgpu/queue.go:298`：`Present(..., _ []image.Rectangle)` 有意忽略 damage；`Surface.GetCurrentTexture` 每帧轮转图像。
- `frameScratch`（`gpu_render_context.go:172-178`）是全屏持久 offscreen，但只在 advanced-pop 时启用并强制清（`3394-3412`），不是稳态合成面；能不能提升它见 §3.5/P3。
- 没有稳态持久面，就不能把合成裁剪到 damage（fresh image 上会留下空洞）。要么继续全量合成，要么提升 `frameScratch`/自建一张离屏合成纹理，再把整张拷到 swapchain。

### 3.3 场景层不知道 damage（但不是没有“走树”）

`CompositeFramePacketTextured` 是“先走树、后得 damage”：phase 1（`textured.go:1514-1625`）已经在走整棵树、判断 dirty/重录，phase 2（`1637-1711`）再走一遍画；damage 只是 phase 2 的副产物。按 Flutter 的 Pre-roll 语义，**不需要再新增第三次 bounds 预走**——把 damage 汇总挂进已有的 phase 1（它已经知道 dirty、cache bounds、PrevBounds、filter bounds），phase 2 拿这个 union 做裁剪即可。已有 `RasterizeDirty`（`rasterize.go:31-96`，`pipeline_app.go:1710` 生产调用）是 stats/NeedsRaster 清理走法，不是 damage 源；本卡不复制它。

注意：现有 `setGPUClipRect`/`applyGroupScissorWithDamageRects` 只给**已入队**的命令加剪刀；`prepareGPUTextureDraw`/`QueueGPUTextureDrawQuad`（`render/context_image_gpu.go:200-247`、`gpu_render_context.go:1885-1901`）没有“完全在 clip 外就不入队”的 CPU 跳过。所以只在根节点加一个 damage `ClipRect` 并不能省掉 89 次入队/资源构建——必须在场景 walk 层跳过。

### 3.4 已有实现核对（2026-11-16，先查再动，避免重复）

| 立项任务项 | 现有实现（文件:行） | 结论 | 对分期的影响 |
|---|---|---|---|
| OS 级 damage 上屏 | `render/frame.go:144-228` `PlanFramePresent`；`render/context_damage.go:21-88` `FrameDamage`/`TrackDamageRect`；`render_session.go:6816-6825` group scissor；`gles/queue_linux.go:236-352` buffer-age partial present | **已有，直接用** | 不改上屏合同；新卡只把 damage 喂给合成 |
| GL 持久合成面 | `gles/surface.go:51-100` 持久 FBO + color texture；`gles/resource_linux.go:370-390` 每帧返回同一 FBO；`gles/swapchain.go:280-295` 每帧新 view handle | **已有，缺“告知 render 层”** | P1 只补能力上报/view 登记，不新建 FBO |
| 层纹理缓存 / 脏层重录 | `ui/scene/textured.go:74` `PictureTextureCache`；`Bounds`/`PrevBounds`/`RefusedBounds`/`RecordedThisFrame`（`1254/1270/1369/1403`）；`ui/rendering/boundary_cache.go` 边界显示列表缓存（entry 有 `ox,oy,w,h`，但 `BoundaryLayer` 只有 DX/DY，未带 bounds） | **picture bounds 已有；boundary bounds 未接线** | P2 复用 picture bounds；boundary bounds 用子 recording union（含绘制几何）或 build 时带回，不另建缓存 |
| 脏层行走 | `ui/scene/rasterize.go:31-96` `RasterizeDirty`（`pipeline_app.go:1710` 生产调用）；`textured.go:1514-1625` phase 1 重录 | **已有两套行走** | **不新增第三次**；P2 扩 phase 1 汇总 damage，phase 2 消费 |
| 过滤子树 bounds/脏检测 | `ui/scene/filter_cache.go:181-210` `filterSubtreeBounds`；`textured.go:1780-1830` filter cache | **已有，限非 transform** | 复用；transform/嵌套效果保守全量 |
| GPU pass 的 damage 消费 | `render/accelerator.go:71-84` `GPURenderTarget.DamageRects`；`render_session.go` 各 encode 路径（7147/7272/7361/7437） | **已有，最后一段** | 保留；本卡补它之前的 CPU 行走 |
| damage 类型/历史 | `render.Context` frame damage（`context_damage.go`）；`ui/scene.DirtyLayer`（`dirty.go:13-30,145`，冻结的动态世界/雪碧图层）；`render/scene.DamageTracker`（`damage.go:28-80`，软件 scene 对象 diff） | **已有三套，别再新增** | UI retained 路径统一用 `render.Context`；DirtyLayer/DamageTracker 不动 |
| 持久内容登记 | `render_session.go:717-723/1181-1189/1398` `persistentContentViews`/`MarkPersistentContent`（目前只给离屏） | **已有机制，面未覆盖** | P1 扩到 surface，不另起一套 |
| 全量回退 | `render/frame.go:61-62` 0.85；`gles/surface_damage_linux.go:20-28` 4 帧/0.85 | **已有** | 复用，不新增阈值 |
| wgpu 稳态持久面 | `gpu/webgpu/queue.go:298` 忽略 damage；`GetCurrentTexture` 轮转；`gpu_render_context.go:172-178` 的 `frameScratch` 是全屏持久 offscreen，但仅 advanced-pop 用且强制清 | **近似已有，稳态缺** | P3 先评估复用 `frameScratch`，不适合再新建 |
| 合成裁剪 / cull | `textured.go:1637-1711` phase 2 全量走；无 damage 参数/无 cull | **真缺口** | P2 主收益 |
| 计数/测试 | `TexturedStats`（`textured.go:1429-1452`）有 raster/skip/filter 计数，无 cull/full-reason；无 cull/persistent 测试 | **真缺口** | P0 仪 table + 新测试 |

已有测试（不要重复造断言）：`render/internal/gpu/damage_scissor_test.go`、`empty_clip_skip_test.go`、`gpu/gwgpu/gles/surface_damage_linux_test.go`、`present_damage_x11_test.go`、`ui/scene/shrink_damage_test.go`、`textured_admission_internal_test.go`。它们锁的是上屏 scissor、损伤历史、缩小文本 previous ∪ current、纹理准入；**没有**层树 cull 与持久面失效的测试，这是 P0/P2 要补的。

### 3.5 新计划任务源码级复核（P0–P4，2026-11-16）

| 计划任务 | 源码现有实现 | 判定 | 处理 |
|---|---|---|---|
| P0 计数 `layers_visited/culled/blits_issued/full_reason` | `ui/scene.TexturedStats`（`textured.go:1429-1452`）已有 raster/skip/filter/texture/damage 计数；`ui/scheduler.MetricsStore` 有 `raster_layer_count`/`boundary_rerecord`/`boundary_skip`（`metrics.go:112/202-203`）；`wrgate.Report` 有 boundary 两字段（`report.go:108-109`） | 部分已有；**P0 已补** cull/blits/full-reason | 已落地：扩 `TexturedStats`/`MetricsStore`/`wrgate`，不新建 telemetry；cull 裁剪本身仍属 P2 |
| P0 hal 持久面能力 | `hal.Surface` 可选能力模式已有 `PixelPresenter`/`PixelWriter`/`PixelReader`（`hal/resource.go:181-211`）；`SurfaceCapabilities` 只有 Formats/PresentModes/AlphaModes（`descriptor.go:133-145`） | 缺口；**P0 已补** | 已落地：`PersistentBackBuffer` + `SurfacePersistentBackBuffer`；GLES=true / wgpu=false，核心接口不动 |
| P1-1 GL surface 持久登记 | FBO/同纹理已有（`gles/surface.go:51-100`、`resource_linux.go:370-390`）；离屏持久登记已有（`render_session.go:717-723/1398`），但 surface 被排除（`1381-1388`）且 `renderedViews` 每帧清（`1181-1189`） | 机制已有，surface 未接 | 扩 `MarkPersistentContent`/新增 surface 标志；不新建 FBO |
| P1-2 damage 剪刀合成 | `encodeBlitOnlyPass:7114-7125` 已有 damage 剪刀；grouped/blit encoder 各路径均接 `applyGroupScissorWithDamageRects`（7147/7272/7361/7437）；`GPURenderTarget.DamageRects` 已有（`accelerator.go:71-84`） | 已有 | P1 把 surface 的**颜色**登记为跨帧已有（`viewRendered` 稳态为真），**stencil 仍每帧 Clear**（`surfaceCarryViews`/isSurfaceCarry） |
| P1-3 失效处理 | GL `reconfigureSwapchainFBOWith`/`borrowOldFrameOrErr`（`gles/surface.go:283-320`）；present 失败 `resetPresentDamageHistory`（`queue_linux.go:244-247`）；离屏 `ForgetPersistentContent`（`render_session.go:1420`）；`forceFull`/`postResizeFull=3` | 部分已有 | 把新 surface 标志接到这些失效点，不新造失效机制 |
| P2-1 phase 1 汇总 damage | phase 1 已走树并重录（`textured.go:1514-1625`）；picture bounds 已有（`pictureRecordBounds:938`、`PictureTextureCache.Bounds/PrevBounds/RefusedBounds:1254/1270/1369`）；texture damage 已有（`composite.go:296`）；无全局 union | 部分已有，缺 union | 在 phase 1 加累加器；damage 规则抽成共享 helper，phase 1/phase 2 共用，避免两处规则漂移 |
| P2-2 phase 2 裁剪 | phase 2 全量走（`textured.go:1637-1711`）；`ClipRect` 只处理 ClipRectLayer（`1898/1907`）；`DrawGPUTexture`/`QueueGPUTextureDrawQuad` 不做 clip 外不入队（`context_image_gpu.go:200-247`、`gpu_render_context.go:1885-1901`） | 缺口 | 加 damage clip + 子树 skip；相交层全画 |
| P2-3 boundary/container bounds | `BoundaryLayer` 只有 DX/DY（`layer.go:383-391`）；`BoundaryCache` entry 有 ox/oy/w/h（`boundary_cache.go:113-114`）；`RenderObject.Size()` 可用（`node.go:33`）但无 PaintBounds；`filterSubtreeBounds` 只覆盖 filter 子树（`filter_cache.go:181-210`） | 部分/缺口 | 子 recording union 或 build 时带 bounds；不另建缓存 |
| P2-4 效果 bounds | `filterSubtreeBounds` 拒绝 transform/嵌套 filter；`subtreeHasTransform`（`filter_cache.go:149`）；`BackdropFilterLayer` 走全屏 backdrop；blur cache pad 16 | 部分，限界明确 | transform 四角映射（`transformBounds:1965`）；backdrop 全量；blur 保守扩张/全量，不猜 |
| P2-5 全量回退 | `PlanFramePresent` 0.85（`frame.go:144-228`）、`DamageFullCoverageThreshold`（`61-62`）、gles `presentDamageFullCoverage`（`surface_damage_linux.go:25-28`）、`forceFull`/`postResizeFull` | 已有 | 复用；不新增阈值 |
| P3-1 wgpu 持久合成面 | `frameScratch` 已是全屏持久 offscreen（`gpu_render_context.go:172-178`），`ensureFrameScratch` 复用/重建（`3693-3745`），只在 advanced-pop 时启用并强制清（`3394-3412`），关闭释放（`4545-4557`） | **已有近似实现** | P3 先评估复用/提升 `frameScratch`（改清/拥有语义）；不先新建；不适合再新开一张 |
| P3-2 全量拷贝 | `frameScratch → surface` 已有 blit 路径（`3477-3512`）；`QueueGPUTextureDraw`/`copyGPUTextureToTarget` 可复用 | 已有 | 复用，不写新 blit |
| P3-3 资源生命周期 | `frameScratch` 走 `RetireTexture`/Destroy/尺寸变化（`3693-3745/4545-4557`）；`CreateOffscreenTexture` 池/账本（`4273+`） | 已有模式 | 复用；若新建接 VRAM 账本 |
| P4-1 硬化窗/测试 | 已有 `ui_base_multiwin_cap`、R7、R4b、C7、R17 等窗；`damage_scissor_test.go`、`surface_damage_linux_test.go`、`present_damage_x11_test.go`、`shrink_damage_test.go`、`textured_admission_internal_test.go` | 部分 | 复用窗；补 cull/persistent/失效测试，不重复现有断言 |
| P4-2 回归/真源 | `tools/per-file-tests.sh`、golden/pixel/wrgate 现有门禁 | 已有 | 按现有门禁跑，不新建管线 |

## 4. 目标形态（设计）

### 4.1 两块正交能力

**A. 持久合成面（persistent composite surface）**

合同：这一帧的合成目标在下一帧仍保有上一帧的完整内容；只有 configure/resize/context loss/借旧 FBO 等事件会使它失效。

- GL：把现有 swapchain FBO 的持久性通过 hal 可选能力上报（照 `hal.PixelPresenter` 的可选接口先例），render session 每帧把当前 surface view 登记为 persistent；失效事件由 backend 发信号，或由现有 `forceFull` / `postResizeFull=3` 覆盖。
- wgpu：优先评估复用/提升现有 `frameScratch`（`gpu_render_context.go:172-178`，已全屏持久、可 resize/关闭释放；现只在 advanced-pop 用且强制清），把它变成稳态合成面的前提是改“清/拥有”语义；若不适合复用，再新建一张屏幕大小的离屏纹理（render 层持有），每帧只更新 damage 区，最后整张拷到 swapchain image（没有 incremental present，这一拷省不掉）。
- 资源：wgpu 的离屏纹理进 VRAM 账本；resize 重建、关窗释放、池复用要清账（照 `ENGINE_GPU_RESOURCE_LIFECYCLE.md`）。

**B. 脏区合成（damage-aware composition）**

- damage 汇总：扩已有 phase 1（不新增第三次走树），产出本帧 damage union + 多 rect；规则沿用现有 `DamageRects` 语义（重录层 previous ∪ current、纹理层每次绘制都脏、vector 回放/拒录层按当前 bounds、空壳保旧范围）。bounds 来源优先复用 `PictureTextureCache.Bounds/PrevBounds/RefusedBounds` 与 `filterSubtreeBounds`；拿不到可靠 bounds 的 transform/嵌套效果按全量保守处理。
- boundary/container bounds：`BoundaryLayer` 目前只有 DX/DY（`layer.go:383-391`）；`RenderObject.Size()`（`node.go:33`）不含阴影/滤镜溢出。优先用子 recording 的 `Bounds()` union（`Recording.Bounds()` 含绘制几何），拿不到就保守全量；不另建边界缓存。
- 合成裁剪：把 damage union 作为 `dc.ClipRect`，只绘制与 damage 相交的层/子树；整棵不相交的子树跳过。**相交的层全部画**，不区分 dirty/clean。
- 全量回退：damage 并集 ≥85% 表面（`DamageFullCoverageThreshold`）、首帧、resize/recovery、持久面失效、影响范围无法证明（滤镜/模糊/无界效果）时，走全量合成——与 Flutter 的 `ShouldPerformPartialRepaint` 同义。
- 上屏：GL 继续走现有三件门 + buffer-age partial present；wgpu 全量拷 + 现有全量 present。**上屏合同一条不改**。

### 4.2 正确性不变量（违反即打回）

1. **画所有与 damage 相交的层**，不是只画 dirty 层；透明层叠、saveLayer、opacity、clip 的混合结果才能对。
2. **previous ∪ current**：重录层、空壳层、缩小文本的旧范围必须继续进 damage（`ui/scene/shrink_damage_test.go` 已锁）。
3. **效果扩张**：模糊/阴影/backdrop 的 paint bounds 必须含扩张；拿不到可靠 bounds 的效果按“无界→全量/不裁剪”保守处理。
4. **坐标**：damage 预走用逻辑坐标，相交判断按 deviceScale 向外取整到物理像素；裁剪到表面。
5. **持久面失效**：configure/resize/context loss/借旧 FBO/池复用 → 强制全量并清 damage 历史；不得靠“看起来没变”猜。
6. **多窗/单 GL 线程**：持久面、damage 历史、renderedViews 全部按窗隔离；共享设备不共享这些状态。
7. **无隐藏全量**：每帧必须能报出“为什么全量”（first/resize/coverage/effect-unbounded/unknown-persist）；稳态出现无理由全量 = 假绿。

### 4.3 分层归属

| 层 | 做什么 | 不做什么 |
|---|---|---|
| `ui/scene` | damage 预走、层树裁剪、canvas clip、相交层绘制 | 不碰 swapchain/FBO/后端 |
| `render/internal/gpu` | 持久目标登记、LoadOp/scissor、全量回退、合成计数器 | 不认识组件/层类型 |
| `gpu/hal` + `gles`/`webgpu` | 持久面能力上报、失效信号、资源生命周期 | 不做层树裁剪 |
| `ui/embedder` | 把 forceFull/resize/occlusion 状态传下来；示例计数接 JSON | 不写合成算法 |
| `examples` | 压力窗（透明叠层/缩小文本/滚动/嵌套 clip+opacity/saveLayer/resize/遮挡）只做原子验证 | 不实现裁剪/合成行为 |

## 5. 分期与验收

**P0 仪 table + 能力探测（✅ 2026-11-16 落地，不改行为）**

- 开工前先按 §3.4 表逐项签字：已有项只复用不改，缺口项才动；任何新增实现必须写明“为什么不能复用”。
- 加每帧计数：`layers_visited` / `layers_culled` / `blits_issued` / `composite_full_reason`；加 hal 可选能力探测（GL=持久、wgpu=非持久），先用测试锁住。
- 验收：画面/golden/pixel 零变化；现有窗 JSON 计数自洽（`visited = culled + live-walked`，容器也计入 live；纹理 blit 单独看 `blits_issued`）；基线 A/B 三跑中位落表。数字没钉死前不准改行为。
- **落地证据**：`scene.TexturedStats` + `PipelineApp.LastCompositeStats` + `FrameMetrics`/wrgate 四字段；`hal.PersistentBackBuffer` + `SurfacePersistentBackBuffer`（GLES=true / wgpu=false）；真窗计数 spin `8/0/2/fresh_target`、icon `297/0/120/fresh_target`（icon golden 0.0000%、pixel 4/4）；targeted tests + `go run ./scripts/apidoc` 绿。全包唯一红 `TestS69_HeavyBudget_TierGates` 属项目存量红名单（S69 过期基线），非本批新增。
- **A/B（2026-11-16，用户授权当前桌面单窗；一窗一轮、示例自退、逐轮查 Xorg，无超时/无 SIGTERM）**：icon 3 轮中位基线 **24.98%** 单核 vs P0 **24.06%** 单核（P0 −0.9pt，噪声内）；spin 3 轮中位 17.64% vs 15.25%，但 `present_wait` 在 FIFO/非阻塞间混跑（base 2/3 FIFO，P0 1/3 FIFO），同模式看无一致回归（FIFO base ≈17.7 vs P0 15.9；非阻塞 base 13.36 vs P0 ≈14.85）。计数稳定：spin `8/0/2`、icon `293/0/118`、`fresh_target`；OS partial ≈1193/5、1185/6。结论：P0 计数未造成可见回归；绝对数受 pacing 影响，性能结论以 A/B 相对值为准。
- **遗留**：全量 render/embedder GPU 回归未在当前桌面重跑（全包曾卡死；S69 为存量过期基线红）；如需全量，走隔离环境。

**P1 GL 持久面登记（2026-11-18 悬停路径根因修复，默认开启；只改“敢不敢用 damage 剪刀”）**

- 只补能力上报/view 登记，不新建 FBO（FBO 已在 `gles/surface.go:51`）；`fullSurface` 在稳态为 false；仍保留全树合成（暂不裁剪层）。
- **实现**：`PresentTarget` 构建时 `hal.SurfacePersistentBackBuffer(surf)` → `Context.setSurfacePersistent`（GL=true / wgpu=false）；
  `present` 维护 `surfaceContentValid`（forceFull / postResizeFull / storm / 设备恢复 / present 失败清零，成功 Full/Damage 后置位，Idle 不动）；
  `flushGPUWithViewCoreImpl` 在非离屏且 persistent+valid 时调 `GPURenderContext.MarkSurfaceContent(view)`；
  session 把**每帧新 view handle**登记进 `renderedViews`/`colorContentViews`（**不进** `persistentContentViews`，避免每帧泄漏一个句柄），
  首 pass 颜色 LoadOpLoad、**stencil 必须 Clear**、合成按 damage 剪刀。窗口路径用 per-pass `target.View`（`SetSurfaceTarget` 未被窗口调用），登记点就在实际 flush 前。
- **stencil 跨帧加载修复（2026-11-17，根因）**：`MarkSurfaceContent` 只登记颜色；另记 `surfaceCarryViews`，
  三个 surface pass 的 LoadOp 判定改成 `viewRendered(view) && !isSurfaceCarry(view)` 才 Load stencil。
  **stencil 是每帧瞬态**（`StencilStoreOp=Store` 只在帧内有效）：把上一帧的 stencil 载入后，stencil-then-cover 的 cover 阶段
  读到脏 winding，整条 path 按包围盒填实——活面上所有矢量图标变成“描边留白、其余填满”的实心块，文字（MSDF）不受影响。
  去掉登记会连脏区剪刀一起关掉（`fullSurface=true`），所以不是修法；正确修法是颜色跨帧、stencil 每帧清。
- **单测**：`TestSurfaceContentFlags`、`TestMarkSurfaceContentIsPerFrame`（含 `isSurfaceCarry` 与幂等）、
  `TestCompositeFullReasonSurfaceContent`（MSAA 仍返回 msaa）。
- **落地观测（单窗、正常自关、逐轮查 Xorg）**：spin_min 10s `composite_full_reason=""`（P0 为 fresh_target）、
  8/0/2、OS 595/5、60.16fps；kit/icon 10s `""`、297/0/120、60.02fps、golden 0.0000%（1024px）。
- **活面证据口径（重要）**：`SnapshotPath` 每轮覆写同一文件，跨轮对比无效；必须在同一轮内比 presented surface 与
  `SnapshotPath`（或 `RetainedSnapshot`）。`-auto-only` 的 golden 是整树重绘快照，不能单独证明活面。
- **隔离证明（2026-11-17，ANIM=20 固定步进、逐帧存 GL FBO 活面）**：
  1. P1-off：帧 0 与同轮快照一致；帧 1–3 仅目录卡名文字瞬时差 1682px；帧 5 起回基线（224px）。
  2. P1-on（修复前）：帧 0 一致；帧 4（首个非 force 帧）起所有图标行变实心块，之后到 180 帧完全不变（16886px，目录区 10688px）。
  3. 只保留颜色登记（去掉 `renderedViews`）：图标恢复，但 `fullSurface=true`，脏区剪刀被一起关掉——不是修法。
  4. 精确修复（颜色跨帧 + stencil 每帧清）：帧 4 起活面 vs 同轮快照 **202px**，bbox `(8,50)-(458,467)`，
     与 P1-off 基线 **199px** 同域同量；`composite_full_reason=""`，剪刀真实生效。
  - **旧判断更正**：此前把 17119px 归因为“`ui/scene` damage 范围不足（目录行/旋转层漏报）”是**错误归因**；
    那 17119px 就是本节的 stencil 脏读。候选 A（cacheless 矢量回放全部报当前 bounds）已试并回退，无变化。
- **悬停路径回归（2026-11-17，用户报告）**：鼠标划过目录卡片后图标变样/消失（左→右消失、右→左部分不对，点击也不对）；rwgpu 正常。
  自动扫掠复现（ANIM 固定步进 + 合成 hover 序列）：P1-on 6604px、P1-off 452px 基线；缺失区域正是被悬停后**回落到 idle** 的卡片图标与名字。
- **取证链（逐项排除）**：
  1. 只滚动、不悬停：215px（基线），悬停才是触发点。
  2. 全矢量回放（`WR_NOCACHE`）：目录区 807px（恢复正常）→ 问题在层纹理路径。
  3. 层纹理探针：新图标层 `ops>0 / rec=true / blit=true`，回放 `pending==ops`，纹理不是空录制。
  4. 把同一纹理再画一次到卡片 damage 内的另一位置：**字形完整出现**；同位置重复画一次：**第一笔不生效、第二笔正常**。
  5. 结论：纹理内容正确，**多组 damage 分组帧里该组的第一笔纹理 draw 丢失/被覆盖**（同位置二次 blit 可复现），属于 GPU 分组/顶点批次路径，不是 scene damage 范围。
- **处置**：P1 默认关闭（`GPUI_SURFACE_PERSISTENT=1` 才开）；保留 stencil 跨帧加载修复（真实正确性修复，P1 关闭时无行为影响）。
- **重开条件**：先修“分组帧组内第一笔纹理 draw 丢失”，用同位置二次 blit 探针与悬停扫掠（P1-on diff ≤ 基线）双证据验收，再开默认。
- **未跑**：wgpu 原生（P3 负责）；桌面未跑整包 GPU 测试（按 AGENTS 走隔离环境）。
- **✅ 2026-11-18 根因修复并默认开启（上一条“组内第一笔纹理 draw 丢失”归因作废）**：
  1. **真根因**：P1 把 `MarkSurfaceContent` 放在 `flushGPUWithViewCoreImpl`（泛化 flush 核心），`offscreen := view == t.View` 对 recordLocal 的层 view flush 恒为 false（层无隔离栈时 `t.View` 为 nil）→ **每个层纹理 view 被登记成 surface carry**（进 `renderedViews`/`colorContentViews`/`surfaceCarryViews`）。稳态下（`surfaceContentValid=true`）每一条层录制 pass 的 colorLoadOp 变成 **Load**：把池化层 RT 里上一个主人的像素垫在新录制下面。图标洞区（Outlined 环的内部）与池化换主（别的卡的 Filled 图标）处显形——WR_PASSDBG 证据：hover 重录 pass `colorOp=Load vr=true carry=true`（39x39 层 view 带 carry 标记）。之前“组内第一笔 draw 丢失”“路径分类/convex 覆盖”均为错误归因：record pass 分类与表面回放完全一致（fill 规则/凸性判定 target 无关），且 worktree 实验“stencil 一律不 Load”后仍复现，证明与 stencil loadOp 无关。
  2. **修法**：登记点挪到唯一持有持久面合同的地方——`PresentTarget.present` 在 acquire 后、flush 前调 `dc.markSurfaceContentForPresent(view)`（`render/present_frame.go`，门控 persistent+valid 在 `render/present_surface_content.go`）；`flushGPUWithViewCoreImpl` 不再标记。层录制 pass 回到 Clear/Clear，surface pass 保持“颜色跨帧 Load、stencil 每帧 Clear”。
  3. **同批落地（worktree 甄别结论）**：逐矩形 damage 剪刀（`damageRectsForGroup`，四条 grouped/blit 路径 + base 层，AABB 会涂掉远距矩形间未损坏区域）；evictForNew 逐出前把 lastBlit 计入 damage（旧字形在 LoadOpLoad 下永久残留）；GL pass 开始 ResetScissor（GL scissor 是全局态，damage 剪刀泄漏进后续离屏录制 pass 会把层 RT 画空——WebGPU 语义对齐）；离屏 clip 栈隔离（`BeginOffscreenPass` 暂存调用方 clip 栈，录制回放自建裁剪）。DIAG“stencil 一律不 Load”**回退**（同帧重渲染的 stencil 是本帧有效内容，Load 正确；跨帧 carry 由 isSurfaceCarry 排除）。
  4. **验收（单窗、自退、逐轮查 Xorg；同轮 surface vs SnapshotPath 逐像素）**：悬停扫掠（ANIM=64 固定步进 + hover 序列）**P1-on surface vs P1-off surface = 0px**（1200×800 逐位一致）；两模式 surface vs 同轮快照同 1798px（快照矢量回放 vs 纹理 blit 的 AA 固有差异，与模式无关）。icon `-auto-only` golden **0.0000%**、scripted **4/4**；spin_min 10s `reason=""`、OS 590/5、raster p50 16.52ms（基线 16.51）；icon 默认开启后 `reason=""`、OS 343/5（98.6% 增量）。单测：`TestDamageRectsForGroup`、`TestMarkSurfaceContentForPresentGate`、`TestSurfaceContentFlags`、`TestMarkSurfaceContentIsPerFrame`、`TestCompositeFullReasonSurfaceContent`、record coverage、gles damage、scene admission/shrink 全绿。
  5. **A/B 公平性教训**：scripted hover（elapsed 时间触发）与 hover sweep（tick 步进）竞速导致两轮最终悬停卡不确定，曾把“最终状态不同”误读成 P1 差异；sweep 模式现已禁用 scripted hover（`examples/kit/icon/main.go`）。
  6. **未跑**：wgpu 原生（P3 负责，capability=false 不受影响）；桌面未跑整包 GPU 测试（按 AGENTS 走隔离环境）。

**P2 场景层树脏区裁剪（✅ 2026-11-18 落地，GPUI_COMPOSITE_CULL=0 退出）**

- 实现与验收记录（细节见修订表 2026-11-18 P2 行）：phase 1 损伤预测（`ui/scene/composite_damage.go` 共享 helper，镜像 pushCompositeCTM 的纯矩阵跟踪）+ 裁剪门（`CompositeClipSafe` × 0.85 覆盖 × unbounded）+ phase 2 `ClipRect(union)` + 逐层/子树跳过（`LayersCulled` 落账）+ `WR_P2CHK` 超集自证。
- 验收实测：icon 稳态冻结帧 `blits_issued` **95→2**、`culled=33/visited=74`（巡览帧大 damage 自动回全量门）；悬停扫掠/滚动证明 **P2-on vs P2-off surface 逐位 0px**；icon golden 0.0000%/scripted 4/4；WR_P2CHK 扫掠全程零违例；spin_min 不回归（reason=""/OS 591-5）；性能 A/B 三跑中位 CPU 24.34%→21.46%（−11.8%）、raster p50 1.78→1.11ms（−37.6%）。
- 单测：`ui/scene/composite_damage_test.go`（变换镜像/累加器/门/逐层跳过/重录壳永不跳/矩阵序），纯 CPU 无 GPU。
- `-race` 多窗与 60s+ soak 按 P1 先例记 P4 硬化期（隔离显示环境）；本批无新增共享状态（每窗独立 session/cache）。
- 顺带发现两个存量问题（非本卡引入，归属对应线，如实记录）：① `transformBounds`（textured.go）对 `TransformPoint` 结果再除 deviceScale，但 `c.matrix` 只含用户变换不含 device scale（`SetDeviceScale` 明确不触碰用户矩阵）——scale≠1 的窗口 damage 矩形会缩小 1/scale，现网真窗均 scale=1 故潜伏；② 7 个 ui_render_base 场景窗（clipop/cliprrect/backdrop/filterops/framepresent/blendmodes/canvascontract）`-auto-only` 存量红（2.9%~7.0%），在 P1 提交与其父提交 eebfde98 上逐窗对照数字完全一致，与 P1/P2 均无关，golden 基线过期归属基线线。

**P3 wgpu 持久合成面**

- 先评估复用 `frameScratch`（见 §3.5）；不适合复用再新建；全量拷贝复用现有 `frameScratch → surface` blit 路径；复用 P2 的裁剪；资源账本/resize/关窗/池复用。
- 验收：wgpu 稳态 CPU/raster 双降；无 partial 计数时如实报“全量 present”；VRAM 进出可查；关窗/重开无泄漏。

**P4 硬化与收口**

- resize 风暴、遮挡/最小化/恢复、4/8 窗、60s+ soak、context loss/恢复、借旧 FBO、MSAA 路径（现有 damage 在 sampleCount>1 会丢弃，必须保持）。
- 回归：spin_min / icon / R4 / C2 / C4 / R7 / 多窗逐项；真源回填 §5.11 同款实测表。

## 6. 门禁（合入前必跑）

1. 受影响包逐文件回归；触及共享数据的改动跑 `-race`（多窗）。
2. 画面三证据：现有 icon/spin 门禁 `golden 0.0000%` + pixel 全过；新增压力窗逐项像素断言 + Golden。
3. 计数门禁：稳态不得出现无理由全量；`blits_issued` 必须与“相交层”账对得上；`damage` 不扩大（不能靠多画掩盖漏画）。
4. CPU/帧：同机、同窗、同时长、三跑中位；单核百分比与四核合计分开写；steady 窗口剔预热。
5. 资源：VRAM 账本、关窗/重开、resize 重建；只功能绿不看资源 = 假绿。
6. 收敛性：damage union ≥85% / 滚动/巡览/风暴必须回全量；不得把全量换成“多 rect 小剪刀”导致更慢。
7. 重复实现检查：新增类型/缓存/持久面/树走必须对照 §3.4 表说明复用关系；不能说明就不合入。

## 7. 风险与开放问题

1. **效果 bounds 不可靠** → 漏画留残影。先做 bounds 审计，拿不到就保守全量；不得用“大概”定裁剪。
2. **持久面被驱动丢弃**：GL FBO 是 offscreen texture，理论跨帧有效；仍需 backend 失效信号兜底，不能假设永远有效。
3. **buffer age 与持久面混淆**：持久面保存最新内容，上屏到轮转缓冲必须继续用 age 并集；两者不能合并成一个 damage。
4. **多窗单 GL 线程**：每窗独立 FBO/历史/view 记账；共享设备只共享 device，不共享窗口状态。
5. **saveLayer/filter/backdrop**：clip 语义与 Flutter 有差异时，先保守全量，再逐类放开；不允许“先裁了再说”。
6. **滚动/巡览/大损伤**：这些场景本就不该省；验收必须包含“没有变慢”。
7. **wgpu 全量拷贝成本**：1200×800 约 3.84MB/帧，GPU 带宽小但命令/FFI 有固定成本；P3 实测，不达标就保留全量合成，不硬上。
8. **MSAA**：现有 damage 在 sampleCount>1 会被丢弃（`render_session.go:2190-2207`）；持久面/裁剪只能在 1x blit 路径启用，MSAA 路径保持全量。
9. **阈值**：场景裁剪用 0.85（现有）还是 0.7（Flutter Impeller）；P0/P2 实测定，不引入第二套“拍脑袋”阈值。
10. **标准边界**：Flutter Linux GTK 未接 existing_damage；本卡是“按引擎标准补合成侧”，不是“追平 Linux Flutter 现状”。
11. **重复实现风险**：上屏 damage、GL FBO、层纹理缓存、group scissor、`DirtyLayer`/`DamageTracker` 都已存在；P0 必须先按 §3.4 表逐项确认再动手，任何“新建一套 damage/持久面”的方案直接打回。

## 8. 复现与证据

```bash
# 基线（2026-11-16，HEAD b41ff479，桌面 Xorg :0 + NVIDIA，4 核）
go build -o /tmp/gpui_spin_min ./examples/kit/spin_min
go build -o /tmp/gpui_icon ./examples/kit/icon
DISPLAY=:0 GPUI_BACKEND=go RUN_SECONDS=15 /tmp/gpui_spin_min   # partial 891/5, steady 14.64% single-core
DISPLAY=:0 GPUI_BACKEND=go RUN_SECONDS=15 /tmp/gpui_icon        # 巡览版 37.67% single-core, partial 108/782
# icon 稳态：复制 examples/kit/icon 强制 secs=20、secsSet=false（关自动巡览/定时快照）的临时探针；
# 探针未入库，仓库示例的定时跑法会开巡览。
# 逐线程 CPU 用 top -H 风格 14s 窗；四核合计 = 单核 ÷ 4。
```

证据归属：本轮数字与计数见 `RENDER_BASE_RESOURCE_MATRIX.md` §5.11；上屏合同见 `ENGINE_FRAME_PRESENT_STANDARD.md` §2.4。
本卡 P0 开工前先把基线 JSON 与 `boundary_skip`/`blits_issued` 计数落盘，后续每期只对同一口径做 A/B。

## 9. 修订表

| 日期 | 内容 |
|---|---|
| 2026-11-16 | 立项：把“脏区合成 + 持久合成面”立为 P6 细 damage 第一卡。依据=2026-11-16 真 Xorg/NVIDIA 复核（partial present 已生效但 icon 稳态仍 22.60% 单核、raster p50 2.25ms、≈89 blit/帧）+ Flutter `compositor_context.cc` 的 `Preroll(clip_rect)`/`canvas.ClipRect` 标准。结论：GL 已有持久 FBO 但 render 层不认；wgpu 无持久面；场景层无 damage 裁剪。本卡只改合成侧，不改上屏合同。产品代码零改动。 |
| 2026-11-16 | **P0 落地（观测，渲染行为零改动）**：`scene.TexturedStats` + `PipelineApp.LastCompositeStats`；`FrameMetrics`/示例 JSON/wrgate `Report` 四字段（`composite_layers_visited`/`culled`/`blits_issued`/`full_reason`，进 RequiredSchemaKeys）；`hal.PersistentBackBuffer` 可选接口 + `SurfacePersistentBackBuffer` 助手（GLES 持久 FBO=true、wgpu-native=false）；`PresentTarget.LastCompositeFullReason` 归类 `force`/`coverage`/`fresh_target`/`msaa`/`no_damage`/`""`。真窗：spin `8/0/2/fresh_target`、icon `297/0/120/fresh_target`（golden 0.0000%、pixel 4/4）；targeted tests + apidoc 绿；全包 S69 红为存量过期基线。P1 待开。 |
| 2026-11-16 | **已有实现审计（用户要求“先确定是否已有实现不要重复”）**：确认上屏 damage（`PlanFramePresent`/`TrackDamageRect`/group scissor/gles partial present）、GL 持久 FBO（`gles/surface.go:51`）、层纹理缓存（`PictureTextureCache`/`BoundaryCache`）、filter bounds（`filterSubtreeBounds`）均已有；真缺口=合成目标持久性未告知 render、phase 2 未消费 damage、wgpu 无持久面、无 cull 计数。据此把 P2 从“新增 bounds 预走”改为“扩已有 phase 1 汇总 damage”，把 P1 从“新建持久面”改为“GL 能力上报/view 登记”；不新增 damage 类型、不新增第三次树走、不新建 GL FBO。详见 §3.4/§3.5。 |
| 2026-11-17 | **P1 落地后回退（GL 持久面登记）**：能力上报 + `surfaceContentValid` 失效门 + 每帧 surface view 登记 + flush 前 MarkSurfaceContent；首 pass LoadOpLoad、合成 damage 剪刀。观测：spin reason="" / 8-0-2 / OS 595-5 / 60.16fps；icon reason="" / 297-0-120 / 60.02fps / golden 0.0000%。**但活面回归**：icon 活窗静态图标黑块/残块（xwd P1-on vs P1-off 直抓对比），golden 快照看不到；GL 是单持久 FBO，问题收窄到 damage 范围不足（不是 FBO 轮转）。已整体回退（patch 存 `tmp/p1_surface_persistent.patch`），待 P2 逐像素证明 damage 后再从 patch 重放。 |
| 2026-11-17 | **P1 重新落地 + 根因更正（stencil 跨帧加载）**：逐帧活面隔离（ANIM=20，帧 0 正确、帧 4 首个非 force 帧起图标全变实心块、P1-off 帧 5 自愈、P1-on 不恢复）证明旧归因“damage 范围不足”错误；真因是 `MarkSurfaceContent` 把 surface view 登记进 `renderedViews`，surface pass 走 `stencilLoadOp=LoadOpLoad`，把上一帧已结束的 stencil 载入，stencil-then-cover 的 cover 按包围盒整块填充。修法：新增 `surfaceCarryViews`/`isSurfaceCarry`，颜色跨帧 Load、**stencil 每帧 Clear**；三处 surface pass 判定统一。活面 vs 同轮快照：帧 4 起 **202px**（P1-off 基线 199px，同 bbox），`composite_full_reason=""`；icon -`auto-only` golden 0.0000%、scripted 4/4、resize/tour 绿。修复后活面回到基线，但随后发现悬停路径回归（见下条），P1 保持默认关闭。 |
| 2026-11-17 | **P1 悬停路径回归（默认关闭）**：鼠标划过/离开目录卡片后图标消失或变样（rwgpu 正常）。自动 hover 扫掠：P1-on 6604px vs P1-off 452px。逐项隔离——只滚动 215px（悬停才触发）；全矢量回放恢复；层纹理 ops/rec/blit/pending 均正常；**同一纹理重复画一次时第一笔不生效、第二笔正常**；纹理拷贝到卡片 damage 内另一位置则字形完整。结论：多组 damage 帧里组内第一笔纹理 draw 丢失，属 GPU 分组/顶点批次路径，不是 scene damage 范围。处置：P1 默认关闭（`GPUI_SURFACE_PERSISTENT=1` 诊断开），保留 stencil 跨帧加载修复；重开需先修该 draw 丢失并过悬停扫掠。 |
| 2026-11-18 | **P1 悬停路径根因修复 + 默认开启（上一条“组内第一笔 draw 丢失”归因更正）**：真根因是 `MarkSurfaceContent` 放在 `flushGPUWithViewCoreImpl`，把 recordLocal 的层 view 误标成 surface carry（`view == t.View` 对层 flush 恒 false），稳态每条层录制 pass colorLoadOp=Load，池化层 RT 上一个主人的像素垫进新录制（图标洞区/换主处显形；WR_PASSDBG：39x39 层 view `colorOp=Load vr=true carry=true`）。修复：登记点挪到 `PresentTarget.present`（`markSurfaceContentForPresent`，唯一持有持久面合同处），层录制回到 Clear/Clear。同批：逐矩形 damage 剪刀（四条路径+base）、evict lastBlit 计 damage、GL pass 开始 ResetScissor、离屏 clip 栈隔离；DIAG stencil-never-Load 回退（与根因无关）。验收：悬停扫掠 P1-on vs P1-off surface **0px**（逐位一致），icon golden 0.0000%/scripted 4/4，spin/icon `reason=""`、OS partial 98%+；单测全绿（新增 `TestDamageRectsForGroup`/`TestMarkSurfaceContentForPresentGate`）。P1 默认开启，`GPUI_SURFACE_PERSISTENT=0` 退出。 |
| 2026-11-18 | **P2 场景层树脏区裁剪落地（本卡主收益）**：phase 1 走树时预测本帧 damage union（`ui/scene/composite_damage.go` 共享 helper；镜像 pushCompositeCTM 的纯矩阵跟踪 compM——phase 1 dc 走 record CTM，旋转/缩放子树的 phase-2 quad 经 compM 四角精确预测）；裁剪门 = `render.Context.CompositeClipSafe()`（P1 持久面+validity，新增导出口）× union 覆盖 <0.85（与 PlanFramePresent 同口径）× 无 unbounded 效果（backdrop/投影/未知界滤镜 miss/未知层种类）；phase 2 整体 `ClipRect(union)` + 逐层跳过（Picture quad/Texture rect/ClipRect 矩形/滤镜有界输出；重录壳永不跳——欠 prev∪current 发射）；最终 damage 发射点一条未动，`WR_P2CHK` 自证超集。过程中修掉三个预测缺陷：①rotDepth==0 的 move 相等必须用与 blit() 同款设备吸附 quad（未吸附 quad 逐位不等 → 冻结帧每层误报移动，95 矩形全窗）；②旋转 move 的 q 是层局部坐标必须过 compM 再并 lb（坐标域混用丢前缘 1px）；③矢量回放的 Fill/Text 经 dc **自记 damage**（context_draw.go Fill → trackDamage），回放层（refused/record 失败/CacheKey==0/noCache）须整层 footprint 预测而非只镜像 1809 门。验收：icon 稳态冻结帧 blits **95→2**/culled=33（巡览滚动帧大 damage 自动回全量）；悬停扫掠+滚动证明 P2-on vs P2-off（`GPUI_COMPOSITE_CULL=0`）surface **逐位 0px**；golden 0.0000%/scripted 4/4；WR_P2CHK 零违例；spin_min 不回归；A/B 三跑中位 CPU −11.8%、raster p50 −37.6%。顺带发现两个存量问题（归属对应线）：`transformBounds` 在 scale≠1 时 damage 缩小 1/scale（现网均 scale=1 潜伏）；7 个 ui_render_base 场景窗 golden 存量红（P1 前后对照同值）。单测 `ui/scene/composite_damage_test.go` 纯 CPU 全绿。 |
