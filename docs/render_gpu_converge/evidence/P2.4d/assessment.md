# P2.4d 开工评估：同尺寸纹理分配收敛（六处 → `res/cache`）

> 2026-10-30 开工登记。只写代码里读到的；行号为当前 HEAD（`ac2e58b3`）。
> 状态：**评估完成，方向待用户拍板**（D1–D6）。未获确认前不动代码。
> 前置：P2.4c 已收口（`00dc07d4`），registry / inflight / idle 回池已在位。

## 0 一句话

六处“同尺寸纹理管家”里，**一处是空的（TexturePool 生产零调用）、四处是真池（stencil / offscreen / filter / dualtex）、一处是所有者自持的固定套装（textureSet）**；它们分属 session 与进程级两种归属，释放安全有三种不同保证，而 `res.Cache` 至今零生产调用且缺三样能力（真实字节、每桶上限、主动 purge）。所以本步的第一件事不是搬代码，是先定 **池归属（session 下沉 vs registry 升设备级）** 和 **直接提交池的 in-flight 闸门怎么办**。

## 1 本步目标与判定（台账口径）

- 目标：六处同尺寸管理器 → `res/cache`；**复用命中率有探针**（`ENGINE_RENDER_GPU_CONVERGE_TASKS.md:304`）。
- 完成标准：能力单测 + 真窗 + 三证据；门禁同 P2.4c；行为零变化（P2 阶段）。
- 依赖：P2.4c 的 `Registry` 槽位状态机（`res/registry.go:318 removeIfReady`）与 idle 回池（`res/registry.go:227 SetIdleCallback`）。

## 2 六处现状盘点

### 2.1 总表

| # | 管理器 | 位置 | key | 容量 / 预算 | 归还 / 销毁 | 生产调用 | 现有测试 | 尺寸变化 |
|---|---|---|---|---|---|---|---|---|
| 1 | `TexturePool` | `texture_pool.go:44`；建 `gpu_shared.go:196` | (w,h,samples) | 128MiB；每 key 留 2 套（`:124-131`） | `DestroyAll` 直接销毁 | **仅** `DestroyAll`/`SetBudget`/`Stats`（`gpu_shared.go:399,470,511,589,918`；`sdf_gpu.go:528`） | `pipeline_wiring_test.go:358,386`、`alloc_bench_test.go:47` | `EndFrame`（生产无调用） |
| 2 | `textureSet` | `gpu_textures.go:33` | 自带 w/h（无 map） | 固定三件（MSAA/depth/resolve），无池预算 | `destroyTextures`（`:560`）→ `releaseOrRetire`（`:600`） | 四个 owner，见 §2.2 | `stencil_pool_prune_test.go` | 尺寸变即 `destroyTextures` + 重建 |
| 3 | `stencilPool` | `gpu_textures.go:58`；`takePooledStencil:135`；`pruneStencilPoolExcept:499` | (w,h) | `BudgetStencilScratch` = 8 条（`:130`） | LRU 逐出 → `releaseOrRetire` | **只有 `session.textures`**（`render_session.go:1352,1395` 走 `ensureSurfaceTextures`） | `budget_evict_test.go:210`、`stencil_pool_prune_test.go` | `pruneStencilPoolExcept` 清旧尺寸 |
| 4 | `offscreenPool` | `gpu_render_context.go:163`；`CreateOffscreenTexture:3696`；`makePoolRelease:3807` | [w,h] | 4/桶 + 32MiB（`:167-173`） | 释放闭包回桶；满则 Destroy；`drainOffscreenPool:3905` | 5 处（`context_layer_pass.go:28`、`context_layer_stack.go:146,573`、`context_image_gpu.go:494`、`filter_ops_gpu.go:785`、`ui/scene/textured.go:444`） | `offscreen_pool_r6_test.go:16` | 无（按 w,h 分桶） |
| 5 | `filterGPUCache` 乒乓池 | `filter_gpu_graph.go:176`；`ensurePool:473`；`acquirePublish:706`；`releasePublish:738` | 单尺寸 `poolW/poolH` | 3 张（A/B/H）+ publishFree ≤4（`:745`） | 尺寸/设备变即 `releasePoolUnlocked` **立即 Destroy**（`:236`） | 全局注册口 `gpu_shared.go:1108,1119,1130`；本窗 flush 口 `filter_flush_coalesce.go:61,113` | `opt44_filter_pass_rp_reuse_test.go` | 立即全销毁重建 |
| 6 | `dualTexBlendCache.outPool` | `dual_tex_blend.go:270`；`getOutBGRA:1108`；`putOutBGRA:1138` | 64 对齐 [w,h]（`:1094` 附近） | 8/桶 + 32MiB（`:1170-1173`） | `putOutBGRA` 回桶；`releasePooledVRAM:1177` 全放；超预算 `enforceOutPoolBudgetLocked:1198` 立即 Destroy | `brush_advanced.go:114`、`gpu_render_context.go:3312` | `budget_evict_test.go:130,178` | 按桶自然增长，超预算淘汰 |

### 2.2 `textureSet` 的四个 owner（同尺寸重复的来源）

| owner | 位置 | 是否跨窗共享 | retireFn | 实际用途 |
|---|---|---|---|---|
| `session.textures` | `render_session.go:278`；`retireFn` 在 `:710` | 否（每窗一个） | **有**（`RetireTexture` → registry 延迟释放） | surface 模式（`ensureSurfaceTextures:342`），唯一使用 `stencilPool` 的地方 |
| `StencilRenderer.textures` | `stencil_renderer.go:58` | 主窗用 GPUShared 注入的共享实例（`gpu_render_context.go:2728`）；effect 窗自建（`preferSampleCount1`） | 无 | `ensureTextures:194`；共享实例在**不同尺寸窗口间互相销毁重建** |
| `texturedStencilLinearCache.tex` | `textured_stencil_linear.go:216` | 是（GPUShared:142） | 无 | `ensureTextures`（`:609`） |
| `texturedStencilPatternCache.tex` | `textured_stencil_pattern.go:131` | 是（GPUShared:143） | 无 | `ensureTextures`（`:530`） |

> 更正 `03_overlap_pools_retry.md` 的两处过期结论：① `:62` “offscreenPool 当前只分配不复用”已不成立（R6-3 已重开，`gpu_render_context.go:3729`）；② `:23` 的 `res.Cache.pending`/`OnSubmissionFinished` 已在 P2.4c 删除。本步收口时一并修文档。

## 3 交叉发现（决定设计的事实）

1. **TexturePool 是空壳**：`Acquire/Release/EndFrame` 生产零调用 → `pool` 恒空；`DestroyAll`（`gpu_shared.go:399,511,589`）与 `Stats`（`:918`、`sdf_gpu.go:528`）都是空操作；对应预算域 `BudgetTexturePool`（`render/budget.go:39-42,131-134`）也是空账。删除是零行为，但会移除一个**公开常量**（`BudgetTexturePool`），预算 pin 测试（`budget_default_test.go:31,60`）与 API 快照需同步。
2. **stencilPool 实际只有一份在生产**（session），但 `textureSet` 结构上给每个 owner 都留了 map 字段；四个 owner 各自持同尺寸 MSAA/depth/resolve 套装，真正的重复是**四套 trio**，不是四个 stencil 池。
3. **释放安全三种模型并存**：
   - registry 闸门（session trio / stencil pool，`retireFn`）；
   - 上游延迟 ≥2 帧（offscreen，`makePoolRelease:3804` 注释与 scene 释放路径）；
   - 调用方持有到安全点（dualtex `putOutBGRA` 注释 `:1135`；filter `releasePublish` 在 submit 后即可回桶，`filter_gpu_graph.go:1269` 的 `release := func(){ cache.releasePublish(slot) }`，**当前没有 fence 保证**）。
4. **filter / dualtex 是进程级（GPUShared）对象，registry 在 session（每窗）**；且两者的 `queue.Submit` 是函数内直提（`filter_gpu_graph.go:1259,1311`；`dual_tex_blend.go:794,1076,1550,1690,1769,1884`），**不进 session 的 submission 记账**。`hal.Queue` 目前只有 `WaitIdle`（`gpu/hal/api.go:295`），无提交完成 token；`webgpu.Queue.OnSubmittedWorkDone` 未进 hal。
5. **purge / OOM 链断裂**：`createTextureRetryOOM`（`gpu_textures.go:615`）第二步只调 `render.PurgeEvictables()`（`:648`，注册方仅 image-cache `image_cache.go:129` 与 picture-texture `ui/scene/textured.go:247`），第三步 `PurgeAcceleratorSurfaceResources()`（`:663`）覆盖 session surface + offscreen + texturePool；**filter / dualtex / 两个 mask textureSet / shared stencilRenderer 不在任何链上**（dualtex 只有 glyph atlas 失败时单点 `releasePooledVRAM`，`gpu_render_context.go:4228`）。
6. **`res.Cache` 缺口**（`res/cache.go`）：
   - `estimatedBytes()` 恒为 `len(keyOf)*4096`（`:186-193`），**不是真实字节**；
   - 无每桶上限（`MaxPerKey`）、无 TTL、无主动 `Purge/PurgeIdle`；
   - 单实例单预算标量，与“按 Budget 域分账”不对齐；
   - 命中/未命中/淘汰计数已在（`HitRate:123`、`Evictions:131`），但没有汇总出口；
   - 线程契约未写明（`res` 包自述“非线程安全”，`types.go:27-28`）；共享池（filter/dualtex）多窗串行依赖各自 `mu`/`runMu`，接 Cache 时锁序要重述。
7. **没有 budget 域的池**：filter 乒乓池目前完全无上限（固定 3 张，换尺寸全销毁）；若收进 Cache 需要给它一个域名或复用 `BudgetLayerOffscreen`（后者语义不符，需要拍板）。

## 4 归属约束

| 资源 | 现状 owner | registry 可用性 | 结论 |
|---|---|---|---|
| session trio + stencilPool | `GPURenderSession` | 有（`resReg`，`render_session.go:282,698`） | 可直接用 Cache |
| offscreenPool | `GPURenderContext`（每窗一个 context，session 在它下面） | 有（`rc.session.resReg`） | 可在 session 内建 Cache；注意 context 自有 `offscreenPoolMu` 锁序 |
| filter / dualtex 纹理池 | `GPUShared`（进程级） | 无（多个 session 的 registry 都在） | **必须二选一**：池下沉 session（每窗一份）或 registry 升设备级 |
| 四个 textureSet | 2 个在 session、2 个在 GPUShared | 混合 | 同 filter/dualtex |

**关键取舍**：把 GPUShared 的纹理池下沉到 session，管线/着色器仍留 GPUShared，是改动最小且更符合“资源谁建谁放”的方案；代价是多窗各有一份纹理池（今天共享池在不同尺寸窗口间本来就会全销毁重建，下沉反而更稳）。把 registry 升为设备级（Skia GrContext 形态）理论更纯，但会推翻 P2.4c 的 session 归属，还要给 `Destroy` 的 `ReleaseAll`（`render_session.go:2134`）加按窗分账，风险高。

## 5 收敛设计（候选）

### 5.1 `res.Cache` 能力补齐（两种归属方案都需要）

| 能力 | 现状 | 补齐后 |
|---|---|---|
| 真实字节 | 固定 4096/条 | `SizeFn func(CacheKey) uint64`（由调用方按格式/样本算，复用 `hal.VramTextureBytes` 口径） |
| 每桶上限 | 无 | `MaxPerKey int`（stencil 8 / offscreen 4 / dualtex 8 / filter 3） |
| 主动释放 | 无 | `PurgeIdle() (freed int)`：只销毁 idle（`refs==0 && inflight==0`）条目；在途条目等 SubmitDone（沿用 `removeIfReady`） |
| 多域预算 | 单标量 | 每域一个 Cache 实例，预算来自 `render.DefaultBudget(domain)` |
| 统计 | hits/misses/evictions | 增加 `Pooled/Bytes/PeakBytes` 与汇总出口（单测 + 真窗探针） |
| 线程契约 | 未写 | 明确：session 内串行；GPUShared 实例由原 `mu`/`runMu` 保护 |

### 5.2 六处的落位

- **#1 TexturePool**：删文件 + 删 `BudgetTexturePool` 域 + 删 `GPUMemoryStats.TexturePool*` 字段（`gpu_shared.go:905-921`）/`ResourceUploadStats.Texture`；测试改写。
- **#3 stencilPool**：由 session Cache（domain `BudgetStencilScratch`）承载；`textureSet` 保留容器字段名，但 acquire/release 走 Cache。
- **#4 offscreenPool**：由 context→session Cache（domain `BudgetLayerOffscreen`）承载；保留“上游 ≥2 帧释放”契约（归还即 `Cache.Release`，registry idle 回池在当前调用点是安全的）。
- **#5 filter 乒乓**：纹理桶进 Cache（管线/绑定组/暂存 buffer 留原位）；换尺寸不再全销毁，交给预算/上限淘汰。
- **#6 dualtex outPool**：进 Cache（domain `BudgetBlendDualTex`）；`releasePooledVRAM` → `Cache.PurgeIdle`。
- **#2 textureSet**：见 D1 —— 建议保留“所有者自持套装”，但**分配与重建走同一个 Cache**（以 Role 区分 MSAA/depth/resolve）；是否把四个 owner 的套装合并成同一个按尺寸共享的 Cache 是 D1 的分歧点。

### 5.3 in-flight 闸门

- session 三处：`Cache.Acquire` 返回 `Ref`，调用方使用期间持 ref；归还即 `Cache.Release`，registry 在 `refs==0 && inflight==0` 后回池。session 的 `SubmitDone` 点已在位（`render_session.go:930,1014,2051,2130,5641`）。
- filter/dualtex：直提路径无 fence。三选一（D3）：
  - **A（推荐）**：沿用现有安全契约（dualtex 调用方在安全点归还；filter 的 publish 槽由持有方上下文延迟释放），Cache 只做 key/预算/上限/统计；把“hal 队列完成 token + 这两池的 fence 级闸门”登记为 P2.4e 子项。
  - **B**：本步给 `hal.Queue` 增加完成 token（wgpu-native `OnSubmittedWorkDone` + GL fence），两后端同步实现，再把直提纳入 `res.Submission`；工作量大、需要双后端真机验证。
  - **C**：不收敛这两处，留在 P2.4e（与 B 的跟踪改造一起做）。

### 5.4 purge / OOM 链

- 推荐把新 Cache 统一实现 `PurgeEvictable` 并注册（`render.RegisterPurgeEvictable`），让 `createTextureRetryOOM` 的第二步真正释放可重建池；同时把 filter/dualtex/shared textureSet 也纳入 `PurgeAcceleratorSurfaceResources` 的覆盖清单。
- 需要可重入/锁序验证：OOM 发生在持有 `filterGPU.mu` / `dualTexBlend.mu` 的路径上时，purge 不能再抢同一把锁（现有 `releasePooledVRAM` 就是“调用时持锁”的形态，接链前必须理清）。

### 5.5 命中率探针

- 单测：同一 key 连续 Acquire/Release → 第二次 hit；关池（`GPUI_NO_OFFSCREEN_POOL=1` 或上限 0）→ miss；预算淘汰后 hit 归零。
- 汇总出口：参考 `render.Context.LayerPoolStats()`（`context_layer_stack.go:27`）与 `FilterPoolStats()`（`filter_ops_cpu.go:17`）的既有形态，暴露每域 `hits/misses/pooled/bytes/peak`。
- 真窗：l1_blank / multiwin 的 JSON 探针增加 pool 段（命中率、池化字节、峰值），作为“复用真的发生”的实证；负向对照 = 把某一域上限改 0 → 该域 hits=0 且 misses 增长。

## 6 决策点

| 编号 | 决策 | 选项 | 推荐 | 理由 |
|---|---|---|---|---|
| D1 | 收敛范围 | A 六处全收（含 textureSet 合并）；B **五池 + 保留 owned set**；C 只删 TexturePool | **B** | textureSet 是“所有者持有的固定套装”而非池；强行池化改动大、收益低；先收四处真池 + 统一释放出口 |
| D2 | 池归属 | A **GPUShared 纹理池下沉 session（管线留 shared）**；B registry 升设备级；C shared 另建设备级 registry | **A** | 复用 P2.4c 的 session registry；避免推翻 `Destroy` 的 `ReleaseAll` 语义；多窗不同尺寸时今天本来就会全销毁重建 |
| D3 | 直提池闸门 | A **沿用上游安全 + Cache 记账，fence 级闸门归 P2.4e**；B 本步加 hal 完成 token；C 这两池留到 P2.4e | **A** | 本步行为零变化；不新增无法验证的双后端能力 |
| D4 | Cache 预算/上限 | A **每域一个 Cache 实例（预算来自 DefaultBudget）+ MaxPerKey + SizeFn**；B 单 Cache + Role 分账 | **A** | 与 P2.4b 的按域声明一一对应，统计也按域 |
| D5 | filter 池预算域 | A **新增 `BudgetFilterGraph`（3 张单尺寸）**；B 复用 `BudgetLayerOffscreen` | **A** | 语义独立；预算表是唯一声明点 |
| D6 | OOM/purge 链 | A **本步把所有新 Cache 注册进 purge 链并补锁序测试**；B 留到 P2.4e | **A** | 否则“池化”会放大 OOM 下的驻留；P2.4c 的教训是链断了没人知道 |
| D7 | 命中率探针出口 | A **单测 + 每域统计 + 真窗 JSON pool 段**；B 只做单测 | **A** | 逻辑探针 + 真窗实证，避免“只绿单测”的假绿 |

## 7 风险

| 风险 | 触发 | 缓解 | 兜底 |
|---|---|---|---|
| 下沉 session 后多窗各有池，显存峰值上升 | 多窗同时开滤镜/混合 | 每域预算按窗公平份额（`render.FairSplit`）；multiwin 峰值对账 P2.4c 的 348064KB | 超限即回到 shared 方案或调小上限 |
| offscreen 归还提前 | 上游延迟契约被 Cache 的 Release 时序改写 | 保持“调用方在 ≥2 帧后 release”契约不变；回归 `offscreen_pool_r6_test.go` | 重开 `GPUI_NO_OFFSCREEN_POOL=1` 验证 |
| 换尺寸不再全销毁导致旧尺寸驻留 | filter/dualtex 入 Cache 后 | MaxPerKey + 字节预算淘汰；`PurgeIdle` 在 purge 链上 | 记录 pooled bytes，真窗峰值门禁 |
| 锁序死锁 | purge 链回调与 `filterGPU.mu`/`dualTexBlend.mu`/`offscreenPoolMu` 交叉 | 先定锁序并写单测；purge 只拿 Cache 自己的锁 | 负向测试固定顺序 |
| `BudgetTexturePool` 公开常量删除 | 外部调用方 | 仓库内 grep 为零；按“接口全量切换制”一次改完，总账同步 | API 快照差异白名单 |
| 命中率探针只反映单测 | 真窗未接 | D7=A 要求真窗 JSON 有 pool 段 | 无数字不标绿 |

## 8 门禁计划（对齐 P2.4c）

- 构建：五目标（linux amd64 / linux 386 / windows / darwin×2，含 `-tags nogpu`）。
- `go vet ./...` 逐行对比 P2.4c 的 119 条。
- 逐测试文件（带 `WGPU_NATIVE_PATH`）PASS/SKIP/FAIL，与 P2.4c 的 27 红逐条对账。
- `-race`：改动面 + 共享池路径。
- conformance 双后端：native + go，资源上界保持 **0/0**。
- 真窗（隔离嵌套 Wayland）：`ui_l1_blank`（native，JSON pool 段）、`ui_base_multiwin_cap`（native `-auto-only` 8 窗）、`ui_base_vram_cap`（native `-auto-only` 300s 峰值对 P1.6 的 372MB 档）；go 真窗撞既有 C7，如实登记。
- 负向对照：① 关掉一个域的池化 → 该域 hits=0/misses 增；② 去掉 `MaxPerKey` → 上限单测红；③ 去掉一次 `PurgeIdle` 生效 → purge 单测红；均需红→绿零残留。
- API 快照 diff（`render` 公开面：`BudgetTexturePool` 删除、`BudgetFilterGraph` 新增）+ 文档同步（`budget.go`、`03_overlap_pools_retry.md`、`ENGINE_GPU_RESOURCE_LIFECYCLE.md` 的池/释放段）。
- 证据目录：`evidence/P2.4d/`（assessment / summary / build / vet-full / per-file-tests / race / conformance / window-gates / hit-rate / negative / api / grep-residual / tools）。

## 9 决策记录（2026-10-30 用户拍板）

| 编号 | 决策 | 结论 |
|---|---|---|
| D1 | 收敛范围 | **六处全收（含 textureSet 合并）**：四个 owner 的同尺寸 trio 合并为按尺寸共享的 Cache；Context/Cache 用 strong ref 独占，避免同帧两个 owner 抢同一张纹理 |
| D2 | 池归属 | **registry 升为设备级（Skia GrContext 形态）**：registry/submission 挂到 GPUShared（每 device 一份），session 持自己的 submission 记账；`Destroy` 只释放本窗归属（slot 增加 owner 标记） |
| D3 | 直提池闸门 | **本步给 hal 加完成 token**：native（wgpu-native `OnSubmittedWorkDone` + 非阻塞查完成）与 gles（提交序号 + `Queue.Poll`）双实现；filter/dualtex 直提纳入 `res` 闸门 |
| D4 | Cache 形态 | **每域一个 Cache 实例 + 全套能力**（`SizeFn` 真实字节、`MaxPerKey`、`PurgeIdle`、按域统计） |
| D5 | filter 池预算域 | **新增 `BudgetFilterGraph`**；surface trio 复用原 128MiB 档，`BudgetTexturePool` 改为语义正确的 `BudgetSurfacePool`（旧名退场） |
| D6 | OOM/purge 链 | **本步把所有新 Cache 注册进 purge 链并补锁序测试** |
| D7 | 命中率探针 | **单测 + 每域统计 + 真窗 JSON pool 段** |

实施顺序（每段单独跑该段单测，最后统一走全量门禁）：

1. hal 完成 token（native + gles + noop/metal/browser 同步实现）→ `res` 侧可挂异步完成。
2. registry 设备级化 + owner 标记 + 按 owner 释放；session submission 保持每窗一份。
3. `res.Cache` 能力补齐（SizeFn/MaxPerKey/PurgeIdle/统计/线程契约）。
4. 六处逐处迁入 Cache（surface trio → stencil → offscreen → filter → dualtex），每处保持行为等价。
5. purge/OOM 链 + 命中率探针 + 全量门禁 + 证据。

## 10 待确认

已无。按 §9 决策执行；每段完成更新本文件与台账。