# P2.4d 段 5 证据：session 两池收敛到 `res.Cache`（surface trio / offscreen）

> 2026-10-30。行号以工作区当前内容为准（未提交）。门禁同 P2.4c 口径。
> 前置：段 1 hal 完成 token（`hal_token.md`）、段 2 registry 设备级（`registry_device.md`）、
> 段 3 Cache 能力（`cache_caps.md`）、段 4 删 TexturePool（`remove_texturepool.md`）。

## 1 这一段把什么收进来了

| 原管理器 | 现在 | 归属 |
|---|---|---|
| `textureSet` 四个 owner 各自自持的 MSAA/depth/resolve 套装 + `stencilPool`（size-keyed、cap 8、LRU 手写字典） | 一个设备级 `surfaceCache`（`res.Cache`，域 `BudgetSurfacePool`，每 key 上限取 `BudgetStencilScratch.MaxEntries`） | `GPUShared.surfaceCache`，owner = `resReg.DeviceOwner()` |
| `GPURenderContext.offscreenPool`（`map[[2]int][]offscreenPooled`，桶 4 / 32MiB / dev 隔离手写） | 每 context 一个 `res.Cache`（域 `BudgetLayerOffscreen`，owner = `resReg.NewOwner()`） | 随 context 生死，Close 走 `ReleaseAll` + `owner.ReleaseOwned` |

`textureSet` 保留 owner 侧字段形状（渲染路径 20+ 处直读 `msaaView/stencilView/resolveView`），生命周期交给 Ref：
`surfaceRef` + `deferRelease`（session 安装，把旧 trio 记进本帧 submission 后再归还）+ `retireOnRelease`
（在 `res.Submission` 之外提交命令的 owner：shared stencilRenderer、textured linear/pattern cover —— 保持“换尺寸即销毁”的旧契约）。

## 2 关键契约（代码里读到的，不是推测）

1. **同尺寸两个 owner 不得同时拿到同一张原生纹理**：`Cache.Acquire` 只回池 `refs==0 && inflight==0` 的条目（`res/registry.go acquireIfIdle`），D1 的 strong-ref 独占。
2. **模式进 key**：`surfaceUsageResolve` / `surfaceUsageDegraded` 是 key 的一部分（`surface_cache.go surfaceKey`）。surface（无 resolve）与 offscreen（有 resolve）是两套原生纹理；旧路径是就地丢 resolve，收进池后会读到 `resolveTex==nil` 的回读通道。
3. **degraded 1x1 回退永不进池**：`destroyTextures` 对 `degradedTrio` 走 `releaseRetire`，否则一次成功的全尺寸 re-probe 可能从池里拿回一张 1x1 depth。
4. **在途不归还**：session 侧重建把旧 Ref 先 `sub.Track` 再 `Cache.Release`（`render_session.go releaseSurfaceRef`），下一次 `SubmitDone` 才回池——这是 resize 崩溃的根因位。
5. **真实字节**：`surfaceTrioBytes` = MSAA(px·4·sc，仅 sc>1) + depth(px·4·sc，degraded 记 1 texel) + resolve(px·4)；`offscreenRTBytes` 同口径，对齐 `hal.VramTextureBytes`。
6. **CreateFn 在 cache 锁外跑**：`Cache.Acquire` 解锁后才 `create`，OOM 时 `createTextureRetryOOM → PurgeAcceleratorSurfaceResources → purgeIdle` 能重入同一 Cache 而不自锁。
7. **设备切换/丢失两条出口分开**：healthy → `releaseAll`（原生真放），device loss → `invalidateAll` + `owner.InvalidateOwned`（只丢账，原生归 abandon flow）。

## 3 单测（`render/internal/gpu/surface_cache_test.go`，全跑在 noop 设备：无 GPU、不 skip）

| 测试 | 钉住的行为 |
|---|---|
| `TestSurfaceCache_TwoOwnersSameSizeGetDistinctTrios` | 两 owner 同 key 拿到不同原生；A 归还后 C 命中拿回 A 那张（hits=1/misses=2） |
| `TestSurfaceCache_ResolveModeIsDistinctKey` | surface/offscreen 分键，跨模式必 miss，offscreen 带 resolve |
| `TestSurfaceCache_SampleCountIsDistinctKey` | 1x 与 4x 分键，1x 不分配 MSAA、4x 必分配 |
| `TestSurfaceCache_DegradedReleaseRetires` | degraded 归还即销毁，池 entries/bytes 归零，再取必是新 trio |
| `TestSurfaceCache_MaxPerKeyCapsIdleBucket` | 同 key 4 次 acquire/release、cap=2 → entries≤2、evictions=2 |
| `TestSurfaceCache_ByteBudgetEvictsLRU` | 预算只容一张：超额时归还即逐出，bytes 回到预算内 |
| `TestSurfaceCache_PurgeIdleKeepsCheckedOut` | OOM 出口只放 idle（freed 精确等于 idle trio 字节），held 不动；purged 计数对账 |
| `TestSurfaceCache_ReleaseAllDestroysEveryTrio` | 设备 teardown：pooled + checked-out 原生各放一次，字段清空，迟到的 Release 不再动 |
| `TestSurfaceCache_InvalidateAllLeavesNativesToAbandon` | 设备丢失：只丢账，原生仍活着（单一释放出口） |
| `TestSurfaceCache_StatsProbePerDomain` | D7 探针出口：name/hits/misses/entries/idle/bytes/peak/budget 逐项对账 |
| `TestSurfaceCache_ProductionLimitsMatchBudget` | 生产限制来自 `render.DefaultBudget`：预算=BudgetSurfacePool，字节算式三档逐一核对 |
| `TestTextureSet_ReleaseRoutingPoolsOrRetires` | owner 侧路由端到端：full→回池；degraded→销毁；`retireOnRelease`→取到池里的 trio 后销毁（hits 计数前进、idle 归零） |

`res` 包同期钉住的（段 3 已有 + 本段修）：`TestCache_SizeFnDrivesBudget`、`TestCache_MaxPerKeyCapsIdleBucket`、
`TestCache_PurgeIdle`、`TestCache_ReleaseRetire*`、`TestCache_ReleaseAll`、`TestCache_ConcurrentAcquireRelease`。

offscreen 侧改写：`TestOffscreenCache_ReuseAndAbandon`（同尺寸回用同一 view、尺寸翻转换 key、abandon 后不得再发旧原生）、
`TestOffscreenCache_NoPoolAllocatesFresh`（`GPUI_NO_OFFSCREEN_POOL=1`：必新分配、归还即 retire、entries 归零）。
`retire_queue_test.go` 四条改为钉 trio 路由（deferRelease 交接 / degraded retire / directTrio 就地放 / 手工字段 inline 放）。

## 4 负向对照（红→绿，逐条注入后原样还原）

| 注入 | 结果 |
|---|---|
| `Cache.repool` 的 `MaxPerKey` 判定短路 | 红：`TestCache_MaxPerKeyCapsIdleBucket` + `TestSurfaceCache_MaxPerKeyCapsIdleBucket` |
| `surfaceKey` 丢掉 resolve 位 | 红：`TestSurfaceCache_ResolveModeIsDistinctKey` |
| `Cache.trimLocked` 直接 return（预算失效） | 红：`TestSurfaceCache_ByteBudgetEvictsLRU` + `TestCache_SizeFnDrivesBudget` |
| `PurgeIdle` 不退池 | 红：`TestSurfaceCache_PurgeIdleKeepsCheckedOut` + `TestCache_PurgeIdle` |
| `destroyTextures` 忽略 `degraded` 位 | 红：`TestTextureSet_ReleaseRoutingPoolsOrRetires`（entries=2 want 1） |
| `destroyTextures` 忽略 `retireOnRelease` | 红：同上（entries=1 want 0） |

**负向抓到的真洞**：第一轮只测 `surfaceCache.releaseRetire` 原语，注入“owner 路由把 degraded 送回池”时全绿——
说明缺的是 owner 侧端到端钉测，遂补 `TestTextureSet_ReleaseRoutingPoolsOrRetires`，两条注入随之转红。

## 5 -race 与整包回归

- `go test -count=3 -race ./render/internal/gpu/res/` → ok。
  修掉一处真竞态：`Cache.Acquire` 现在在锁外调 `CreateFn`（契约如此，见 §2.6），测试替身
  `fakeNativeTracker.create` 裸 append 被 `-race` 抓到，已给替身加锁（生产代码无数据竞争）。
- `go test -race -run 'TestSurfaceCache_|TestTextureSet_|TestOffscreenCache_|TestSession|TestRegistry|TestRetire|TestStencilPool' ./render/internal/gpu/` → ok（带 `WGPU_NATIVE_PATH`）。
- 整包（带 native 库，`go test -count=1 -timeout 900s ./render/internal/gpu/`）：FAIL 7 条 =
  `TestBindGroupLayoutSwitch_StencilThenSDF`、`TestOpt21_DeferSurfaceSubmit_CoalescesLayerFills`、
  `TestOpt34_SharedEncoder_DoesNotFlushLeading`、`TestOpt34_NoSharedEncoder_StillFlushesLeading`、
  `TestOpt36_FilterSeedSharedEncoder_OneFinish`、`TestOpt38_EnsurePipelines_ReadyFastPath`、
  `TestOpt38_EnsurePipelines_WarmRenderFrame`。
  七条逐条核对在 `evidence/P2.4a/P2.4b/P2.4c` 的 per-file 基线里已红，本段零新增红。
- `go build ./render/...` 绿；`go vet ./render/internal/gpu/...` 剩两条 `unreachable code`
  （`gpu_render_context.go:4341/4389`）与 `view_raw_attack_test.go` 的 unsafe 提示，均为基线既有。

## 6 文件产出

新增：`render/internal/gpu/surface_cache.go`（trio + 设备级 Cache + 三档错误语义）、
`render/internal/gpu/surface_cache_test.go`（12 条 noop 单测）。
改：`gpu_textures.go`（textureSet 改为 cache 持有者，删手写 stencilPool，-436/+182 行）、
`render_session.go`（resOwner + deferRelease + purge/teardown 分账）、
`gpu_render_context.go`（offscreenPool → `res.Cache`，purge/abandon/Close 三出口）、
`gpu_shared.go`（设备级 `surfaceCache` 建/放/丢失三径 + shared stencilRenderer 绑定）、
`stencil_renderer.go` / `textured_stencil_linear.go` / `textured_stencil_pattern.go`（`setSurfaceCache`，幂等，绑定即 `retireOnRelease`）。
删：`stencil_pool_prune_test.go`（被 §3 的 key/cap/budget/purge 四条覆盖）。

## 7 过程记录（如实）

本段执行中一次误操作：负向对照脚本里带了 `git checkout render/internal/gpu/gpu_textures.go`，
把该文件未提交的重构（-436/+182）冲回 HEAD。已从本会话转录里同一文件的完整 Read 结果重建
（带行号版本与无行号版本互相核对一致，426 行），`go build ./render/...` 绿、整包 FAIL 仍为基线 7 条、
`git diff --numstat` 回到 182/436，与工作区事故前一致。教训：负向注入脚本禁止再出现任何 git 写操作。
