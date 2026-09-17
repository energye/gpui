# 03 池/缓存/延迟释放/预算/重试对照

> 只写代码里读到的，不猜。头注释引用原文见表。

## 1 池与缓存清单

### 表面/会话纹理

- `texture_pool.go:32 TexturePool`：跨 Context 复用 MSAA/stencil 套装；头注释自述 Deprecated（P5）：生产只用 DestroyAll/SetBudget/Stats，分配路已迁走（session 经 textureSet + pendingTexRetire，长期归 res.Cache）。
- `texture_pool.go:69 Acquire` / `:88 Release` / `:105 EndFrame`（每 key 留 2 套 + 全局 budgetMB）/ `:153 DestroyAll` / `:58 SetBudget`（默认 128MB，`~5 concurrent 1080p MSAA4x`）。
- `gpu_textures.go:24 textureSet`：MSAA color + depth/stencil + resolve 三件套；`:105 stencilPoolKey{w,h}`、`:110 pooledStencil`、`44 stencilPool`（sc==1 surface depth 缓存，cap 8，LRU + retireFn）；`:563 releaseOrRetire`（有 retireFn 进 deferred，否则直接 Release）；`:524 destroyTextures`（Pool 条目故意 surviving，经 LRU + ClearPool 排空）；`:184 ensureTextures`（经 createTextureRetryOOM）。
- `gpu_render_context.go:133 offscreenPool map[[2]int][]offscreenPooled`（layer RT 避每层分配）；`:3364 CreateOffscreenTexture`（复用分支已注释禁用，COLOR_TARGET vs RESOURCE 同 usage scope 冲突，release 闭包直接放）；`:3510 drainOffscreenPool`（全放）；`:3455 PurgeSurfaceResources`（session.PurgeSurfaceTextures + drainOffscreenPool + releaseBrushCoverResults）；`:256 drainLayerReleaseHold`。
- `gpu_shared.go:343 PurgeAllSurfaceResources`（逐 context Purge + texturePool.DestroyAll，`:360-363`）；`sdf_gpu.go:290 SDFAccelerator.PurgeSurfaceResources`（转调）。

### 延迟释放三条平行队列（规则同字，集合独立）

- `render_session.go:92 pendingTexRetire`（resize 旧视图仍被排队命令引用）/ `:98 Add` / `:109 Drain`（GPU 做完才放）；`:134 pendingBufRetire`（grow 旧 buffer，P6）/ `:148 Drain`；`:818 RetireTexture`；`:690 RetireBuffer`；`:776 notePendingViewRetire`；`:786 retirePendingViews`（GPU 做完才放）；`:1805 PurgeSurfaceTextures`（尾部双 Drain，`:1833-1834`）。
- `res/registry.go:107 Release`（refs==0 && inflight==0 && retire 才放）/ `:123 Retire` / `:227 ReleaseAll`（健康 teardown）/ `:212 InvalidateAll`（丢失路不调 native）；`res/submission.go:30 Track` / `:40 SubmitDone` / `:54 Invalidate`。
- `image_cache.go:pending + 197 ReleaseEphemeral`（submit 完成后放，禁 mid-encode）/ `:388 removeEntry`（摘索引推 pending）。

### 长期池归宿

- `res/cache.go:25 Cache`（GrResourceCache 式：key 复用 + 字节预算 LRU + in-flight 不外借）；`:4 CacheKey{W,H,Format,Samples,Usage,Role}`；`:52 Acquire`；`:86 Release`（in-flight 进 pending 等 OnSubmissionFinished）；`:115 OnSubmissionFinished`；`:159 InvalidateAll`；`:173 trimLocked`。

### 专用缓存

- `image_cache.go:47 ImageCache`（key Pixmap.GenerationID，对标 Skia/Vello）；双预算 128 条 + 64MiB（`:16-20`）；`:284 PurgeEvictable`；`:405 evictIfNeeded`；`:415 evictOldest`；`:82 imageStagingPool sync.Pool`（CPU 打包缓冲）。
- `filter_gpu_graph.go:162 filterGPUCache`（texA/texB/texH 乒乓 + publishFree + bgCache + passUniformSlab）；`:216 release`；`:725 releasePublish`。
- `dual_tex_blend.go:244 dualTexBlendCache`（outPool map[[2]int] + bgCache）；`:1168 releasePooledVRAM`（VRAM 紧先放池留管线）；`:1189 enforceOutPoolBudgetLocked`（32MiB，`:1163`）；`:285 release`。
- `render_session.go:195 gpuTexBGSlotCache`（view 指针 key，重建自然失效，4 槽轮转，无字节预算；`:3842 getOrCreate`、`:3882 releaseAll`）。
- `pipeline.go:20 PipelineCache`（老 stub） vs `pipeline_cache_core.go:50 PipelineCacheCore`（描述符哈希；`:237 Clear`、`:251 DestroyAll`）。
- CPU 四几何：`path_geometry_cache.go:112 Path`、`:308 Stroke`、`:420 Dash`、`:553 ConvexPath`（各 Get/Put/Clear/evictOldestLocked）；`tile.go:296 TilePool`（gpu 版 `[]*Tile` 栈，Put 上限 4096）；`sparse_strips.go:315 SparseStripsPool`；mask 五小件（maskR8、linearRamp、patternMask、texturedStencilLinear/Pattern）。
- 通用 CPU：`internal/cache/cache.go:10 Cache`（泛型 LRU，超限删 25%）；`:sharded.go:67 ShardedCache`；`internal/image/pool.go:13 Pool`（ImageBuf 复用）；`internal/parallel/tile_pool.go:11 TilePool`（尺寸分桶 + sync.Pool）；`parallel/pool.go:16 WorkerPool`（goroutine 池，drainQueue 与 GPU 无关）。

### 预算六口径（碎）

128MB（TexturePool）/ 128 条+64MB（ImageCache）/ 256MB+80%（MemoryManager `memory.go:28-34`，`AllocTexture :142` 内 evict，`FreeTexture :182`）/ 768MB 环境变量（vram_ledger，调用方仅 texture.go + buffer.go，管线/采样器/swapchain 不进账）/ 32MB（dualTex）/ 8 条（stencilPoolCap）。CPU 池另有 softLimit/maxPerBucket/4096 各自为政。

## 2 重叠点（只写读到的）

1. 表面尺寸纹理六处同管：TexturePool(w,h,samples) / textureSet(会话尺寸) / stencilPool(w,h) / offscreenPool([w,h]) / filterGPUCache(poolW/H) / dualTex.outPool([w,h])。TexturePool 头注释自认生产分配已迁走，但 purge 仍 DestroyAll 它（废分配 + 未废销毁并存）。
2. 延迟释放三集合独立：pendingTex/Buf.Drain / ImageCache.pending + ReleaseEphemeral / res.Cache.pending + OnSubmissionFinished，三处注释同字“提交完成前不能放”。
3. purge 四条，OOM 重试只调一条：`createTextureRetryOOM (:605-616)` 第二次失败只调 `render.PurgeEvictables()`；ReleaseEphemeral、pending.Drain、PurgeSurface(All) 均不在该函数。且 `RegisterPurgeEvictable` 生产注册只搜到 `image_cache.go:116` 一处，glyph/filter/dualTex 不在链上。
4. 不重叠：internal/cache、image.Pool、parallel.TilePool、路径几何四件套管 CPU，不持 webgpu.Texture；`parallel.WorkerPool.drainQueue` 与资源释放无关。

## 3 重试入口（触发→动作）

- `createTextureRetryOOM gpu_textures.go:578`：OOM → FlushCallbacks+WaitIdle 重试 → PurgeEvictables 再试 → SampleCount>1 降 1 再试。
- depth 1x1 回退 `:253-272`：全尺寸链仍 OOM → 建 1x1 + latchDegraded。
- `degradedServe/degradedReprobeFrames :78,70`（120 帧窗）：锁存期同尺寸直接成功，窗满放行一次重探。
- `latch/clearDegraded :94/100`；`failedFast/failTerminal/clearFailed :494/507/517`（双败锁存，换尺寸/丢设备清锁，错带角色尺寸建议动作）。
- `requestDeviceWithRetry device.go:93`：RequestDevice OOM → 最多 3 次 + 1s 退避。
- `oom_hook.go:15/17/28 SetTextureOOMHook/noteTextureOOM/oomLogThrottled`（1 秒一条）。
- `oom_purge.go:57 PurgeEvictables / :77 IsGPUOutOfMemory`（三短语大小写不敏感）。
- `swapchain.go:645 tryRecoverDeviceLocked`（丢设备 + 1s cooldown → abandon → 老 surface Release（不清 Unconfigure，会 SIGSEGV）→ 老 device 只 Release 不 Destroy（Destroy 会永久 OOM）→ pendingReconfigure）；`:529 ForceRecoverHealthy`（健康 teardown 全套）；`:778 takeRecoverGrace`（settling 帧跳过）；`:1008 acquireHungLocked`（锁存跳 acquire，500ms 节流重配）。
- `render_session.go:702 InvalidateForDeviceLoss`（只清记账不清原生）；`gpu_shared.go:368 AbandonExternalDevice`（WithForce 下 Close 全 live context）。
- `render.ErrFallbackToCPU`（如 brush_advanced.go:26）：GPU 快捷不支持 → CPU 回退，非重试。
- `ResolveCommandView render_session.go:744 + res/types.go:81`：SourceKey 解析不到 → 注册 transient ref 再解，失败跳 draw。

## 4 证据缺口（确认没有）

- glyph atlas / filter / dualTex 未注册 purge 链；offscreenPool 当前只分配不复用；TexturePool Acquire/Release 生产无调用。
