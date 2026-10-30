# P2.4d 第六段：shared 两池下沉设备级并收敛 `res.Cache`（filter 乒乓 / dualtex snap）

> 2026-10-30。基线 HEAD `ac2e58b3`（P2.4c 收口）。方向按用户批准的 D1（六处全收）、D3（本步给 hal 加完成 token，直提纳入 `res` 闸门）、D4（每域一个 Cache + SizeFn/MaxPerKey/PurgeIdle/按域统计）、D5（新增 `BudgetFilterGraph`）。
> 前五段见 `hal_token.md` / `registry_device.md` / `cache_caps.md` / `remove_texturepool.md` / `surface_pools.md`。

## 1 迁移前的两处形态（评估 §2.1 #5 / #6）

| 管理器 | 位置（HEAD） | key | 容量 | 归还/销毁 | 问题 |
|---|---|---|---|---|---|
| #5 `filterGPUCache` 乒乓池 | `filter_gpu_graph.go:176`，`ensurePool:473`，`acquirePublish:706`，`releasePublish:738` | **单尺寸** `poolW/poolH` | 3 张（A/B/H）+ `publishFree ≤4` | 尺寸/设备一变即 `releasePoolUnlocked:236` **立即 Destroy** | 换尺寸把所有 RT 全销毁重建；两套自由表（乒乓 + publish）各管各的上限；直提无 fence |
| #6 `dualTexBlendCache.outPool` | `dual_tex_blend.go:270`，`getOutBGRA:1108`，`putOutBGRA:1138` | 64 对齐 `[w,h]` | 8/桶 + 32MiB | `putOutBGRA` 回桶；`releasePooledVRAM:1177` 全放；超预算 `enforceOutPoolBudgetLocked:1198` 立即 Destroy | 手写桶 + 手写 LRU + 手写预算三套算术；直提同样无 fence |

两处的共同洞：命令缓冲是自己 `queue.Submit` 的，落在任何 session 帧之外，`res.Submission`（每 session 一个在飞窗口）盖不住；归还时机全靠调用方自觉（dualtex 在安全点归还、filter 的 publish 槽由持有方上下文延迟释放）。P2.4c 之前这个洞只能靠约定，本段按批准的 D3 补上真闸门。

## 2 落位

### 2.1 dualtex snap → 域 `dualtex-snap`

- `dual_tex_blend.go:1163` `outCacheLocked(device)`：设备变了就 `ReleaseAll()` 重建 Cache（create 闭包捕获 device），**不在此处分配 native**；`c.resOwner.IsNil()` 时返回 nil，调用方保留直建直毁的兜底路径。
- key：`dualTexSnapKey`（`:1148`）= 量化桶尺寸 × `TextureFormatBGRA8Unorm` × Samples=1 × `dualTexSnapUsageBits` × `RoleFrameScratch`。请求尺寸是逻辑损伤尺寸、**分配按量化桶**，所以只做左上角拷贝的调用方必须把精确尺寸传给拷贝而不是 key（注释钉在 `getOutBGRA:1189`）。
- 预算：`dualTexOutPoolBudgetBytes` / `dualTexOutPoolBucketCap`（`:1288`）直接取自 `render.DefaultBudget(BudgetBlendDualTex)`（32MiB / 8 条），`SizeFn = W*H*4`。原 `enforceOutPoolBudgetLocked` 的手写逐出算术删除，逐出交 `Cache.trimLocked`。
- 归还：`recycleOut(o, track)`（`:1230`）——先 `track(ref)` 再 `cache.Release(ref)`；`track` 为 nil 时是裸归还。ref 为 nil 表示这块 snap 是池外直建的，走 Destroy。
- 统计/清空：`stats()`（`:1296`）与 `releasePooledVRAM()`（`:1309`）都变成 `res.Cache` 的薄壳，且 `stats()` **绝不建池**（未用过的域报空快照，不能顺手造出一个池来上报 0）。

### 2.2 filter 乒乓 → 域 `filter-graph`

- `filter_gpu_graph.go:527` `rtCacheLocked(device)`：同上，设备切换 `ReleaseAll()` 重建；`rtDropGen` 在重建时重置。
- key：`filterRTKey`（`:467`）= w×h × RGBA8 × Samples=1 × `filterRTUsage()` × `RoleFilterRT`。
- 预算：`render.BudgetFilterGraph`（新域，8 条/尺寸 + 128MiB，见 `cache_caps.md` 与 API 目录 §3.7 行）——数字是收敛前「3 张乒乓 + publish 自由表 ≤4」的等价上限，零行为（不放宽也不收紧到能观测的程度）。
- **本段唯一的可见行为差异**：换尺寸不再把所有 RT 立即 Destroy，改由预算/每键上限逐出。原 `poolW/poolH` 单尺寸字段与 `ensurePool`/`promotePoolResultToPublish` 删除（整词 grep 0 命中，见 `grep-residual.txt` §1）。
- 管线、绑定组布局、uniform slab、sampler、dummy view **留在原位**：只有纹理桶进 Cache（评估 §5.2 #5 的口径——图形算法进 `res`，后端对象编排不动）。
- publish 槽仍叫 `filterPublishSlot`，但现在带 `ref`：`acquirePublish:814` 从 Cache 取，`releasePublish:827` 归还并把 ref 交给调用方的 `track`。

### 2.3 直提闸门 `directSubmit`（D3 的实现体）

新文件 `render/internal/gpu/direct_submit.go`（164 行），设备级一份，`GPUShared.directSubmitLocked()`（`gpu_shared.go:794`）建，`dualTexBlendLocked:807` / `filterGPULocked:818` 通过 `bindRes(owner, gate)` 拿同一实例。它把两样东西配成一对：

- `res.InflightGate`（`res/gate.go`，`Submission` 的多在飞对应物）：命令缓冲碰到的每个 ref 都在飞期间被 hold，池不能把它再借给别的窗，retired 条目也不能在被读写时销毁。
- `hal.CompletionToken`（上一段 `hal_token.md`）：`Submit` 返回后**立刻**取 token，hold 的结束点就是那批工作的结束点——不压到下一帧边界，也不阻塞等待。

规则细节（都写进了 `direct_submit_test.go`）：

| 情况 | 行为 | 为什么 |
|---|---|---|
| 正常提交 | hold 挂在 token 上，`pump()` 非阻塞扫已完成项 | 帧率不能被 fence 等待拖住 |
| `Submit` 返回错误 | 当场 `hold.Resolve()` | 什么都没进 GPU，挂着就是永久泄漏 |
| 后端给不出 token | 保持 held，直到真实 `drain()`（`WaitIdle` 证明 GPU 空闲）或设备丢失 | 猜「应该完了」会把竞态重新打开 |
| 设备丢失 | `invalidate()`：清 hold 但**不**减在飞计数 | fence 永不来，native 拆除归 abandon 流程 |

pump 的挂接点：`submit` 自己开头 pump 一次（任何共享池的下次提交顺带清账），加上 session 已有的 GPU 完成点 `endSubmission()` → `s.direct.pump()`（`render_session.go:781`）。锁序 cache → gate → registry，handle 的 resolve 一律在 gate 锁外做（`res/gate.go:58` 注释即此约束）。

### 2.4 顺手补的假绿洞：`Cache.Stats().Idle` 口径

写 `direct_submit_test.go` 时发现：`Idle` 原先按 registry 的 `idleEntry`（refs==0 && inflight==0 && !retire）扫 `keyOf` 统计，而 `Acquire` 只能从 **key 桶**里借、`PurgeIdle` 只能从 `lru` 队列里清。设备丢失路径 `clearInflight` 只清标记、不会触发 idle 回调，于是同一条记录：registry 说「可复用」、Cache 说「不在桶里」——**探针把一条永远借不出去的条目报成可复用**。`res/cache.go:290` 已改为「桶里且 registry 判空闲」，与可借出/可 purge 的集合严格一致；`res/gate_test.go` 的 `TestGate_InvalidateLeavesEntryUnpoolable` 把这条钉住（Idle=0、下一次 Acquire 必是 miss、但 OOM purge 仍能把它收回）。

## 3 测试

| 文件 | 覆盖 | 结果 |
|---|---|---|
| `render/internal/gpu/direct_submit_test.go`（新，253 行，5 条） | hold 未解 → 不回桶、不可再借、第二次 Acquire 必须拿到另一块 native；token 完成 → 回桶且下次命中拿到同一块；submit 失败 → hold 当场关闭；无 token → 只有 drain 能收；invalidate → 条目永不复活、也不重复减计数；无 gate 时 `submitGated` = 裸 `Queue.Submit` | PASS（noop 设备，无 GPU 不 skip） |
| `render/internal/gpu/res/gate_test.go`（新 7+1 条） | 闸门原语：阻塞再借/阻塞销毁、retire 等 resolve、invalidate 不减计数、双 hold 独立、nil/外来 ref 跳过、settleAll、并发 hold/resolve | PASS |
| `render/internal/gpu/budget_evict_test.go`（改写） | `TestDualTexOutPool_ByteBudgetEviction` / `_PerBucketCap` / `_PurgeKeepsCheckedOut` 三条从手写桶改为对 `res.Cache` 域断言 | PASS |
| `render/internal/gpu/pool_purge_lock_test.go`（新） | 见 `surface_pools.md` §4 与本段：purge 不拿 `s.mu`、两池都在 OOM 链上 | PASS |
| `render/internal/gpu/opt32/opt36/opt37/opt40/r7_3/pipeline_wiring` 等既有测试 | 调用点随接口改（refs/track），断言不变 | PASS |

整包口径（不是「ok」，是**与基线逐同名、零新增红**）：`render/internal/gpu` 逐文件 114 PASS / 18 FAIL、`render` 9 FAIL、`res` 0 FAIL、`image` 0 FAIL，合计 27 条与 P2.4c 收口时的 27 条同名同因（`per-file-tests.txt` + `per-file-tests-notes.md` §1）；本段涉及的文件（`dual_tex_blend.go`/`filter_gpu_graph.go`/`direct_submit.go`/`res/*`）单跑全绿。

## 4 留给 P2.4e 的偏差（如实记，不含糊）

1. `filter_flush_coalesce.go:118` 的本窗 flush 口仍走 `rc.session.trackRef`（session 帧的 Submission），不是设备级 gate——它的提交本来就在 session 帧内，按 D3 属正确归属；但这条路径的在飞窗口粒度是「整帧」，比 gate 的「单批命令缓冲」粗。
2. 该口的 `srcView` 记账沿用 P2.4c 之前的形态（视图本身不经 gate hold），因为它是上游 session 已持有的资源、不是池借出的条目。P2.4e 做混合枚举收敛时统一。
3. `directSubmit` 只在 `GPUShared` 存在时装配；裸 `res.Cache` 字面量（测试缝）与 `resOwner` 为 nil 的路径继续走「直建直毁」兜底。
4. 换尺寸不再全销毁 RT 之后，`BudgetFilterGraph` 的 128MiB 是新的显存上界来源；真窗若出现显存抬升，应调预算而不是回退行为（本段按「数字=现状」取等价上限，实测见 `summary.md` 真窗段）。
