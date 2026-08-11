# GPU 资源生命周期真源 — 对齐 Skia 资源管理规范

> **版本：1.0（待批准）** | 日期：2026-08-11
> **范围：** `render/internal/gpu/`（引擎层资源）· `gpu/rwgpu/`（FFI 防御）· `gpu/webgpu/`（facade 校验）
> **约束：** 禁止 CGO（只经 purego FFI）；`CGO_ENABLED=0` 可构建；依赖方向 ui → render → gpu 不变
> **背景：** 对 `render/internal/gpu` 底层资源生命周期的全局能力改造，对齐 Skia（GrSurfaceProxy / GrResourceCache / command-buffer refs）。补齐底层缺失能力（资源悬垂、提交前释放、grow 过早释放、FFI 零防御、池化不可复用），收益对象为**全部上层 R/C 真窗与生产路径**；`ui_wr_r18_savelayer` 的 resize panic 只是最先显形的症状，不是目标。
> **验收口径：** 单窗不崩不足以验收；以 §6 全局回归矩阵全绿为准（`Resolve miss=0`、error scope=0、usage 冲突=0、显存≤预算、设备丢失可恢复）。

---

## §1 背景与现状缺陷

### 1.1 现状缺陷（全局，非单窗）

| # | 缺陷 | 证据 | 影响面 |
|---|---|---|---|
| D1 | 命令持有裸句柄：`GPUTextureDrawCommand.View` 是 `unsafe.Pointer`，入队到消费全程无存活校验 | `gpu_types.go:14`；入队 `gpu_render_context.go:1149`；消费 `render_session.go:3609` | 所有 GPU 纹理合成路径（图层/混合/滤镜/离屏） |
| D2 | 重建即销毁：尺寸变化立即 `Release` 旧视图并清零句柄 | `gpu_textures.go:41,301`；`gpu/rwgpu/safety.go:261` | 所有依赖离屏 MSAA/resolve 的路径 |
| D3 | 同一帧竞态：paint 入队 → flush 重建 → 用已释放视图建 bind group | `render/present_target.go:380`；`render_session.go:996,1178` | 所有 resize / 视口变化帧 |
| D4 | ImageCache 提交前释放：evict 在编码阶段立即 Release；`ReleaseEphemeral` 在未提交的 shared-encoder 路径先于 Submit 释放 | `image_cache.go:347-348,184-187`；`render_session.go:1337` | 图像内容触发预算 evict 的场景 |
| D5 | grow 旧缓冲过早释放：build* 6 处容量增长时立即 Release 旧 buffer，可能被上一帧在途 CB 引用 | `render_session.go:2297,2418,2718,2970,3558,3829` | 内容量突增（文本/HUD/网格增长）的帧 |
| D6 | FFI 零防御：`CreateBindGroup` 对 entry 0 句柄不拦截，0 直接穿到 wgpu → 非 unwind panic → SIGABRT | `gpu/rwgpu/bindgroup.go:292` | 任何一次悬垂句柄=进程级崩溃 |
| D7 | 无兜底：生产路径不用 error scope；uncaptured callback 已注册但无结构性处理 | `gpu/rwgpu/device.go:425-435` | GPU 侧验证/提交错误无法归因上报 |

### 1.2 对齐范围判定（进/不进 res 体系）

> 判定三问：**是否跨帧存活 / 是否 resize 重建 / 是否被已提交 CB 引用**。任一为是 → 进 res 体系；全否 → 帧内临时，保持现状。

## §2 需求（对齐 Skia 机制）

| 需求 | Skia 机制 | 内容 |
|---|---|---|
| **R1 命令与纹理解耦** | M1 GrSurfaceProxy 间接层 | 命令存 `SourceKey`（逻辑描述）或 `Ref`（强引用），不存具体视图快照；`SourceKey` 在 flush 时解析当前活动实例；`Ref` 资源 ref>0 绝不销毁 |
| **R2 资源缓存统一** | M2 GrResourceCache | 按 `(w,h,format,samples,usage,role)` key 缓存 idle 资源；预算 + LRU；in-flight 项绝不借出 |
| **R3 提交期延迟销毁** | M3 command-buffer refs / fence | 被提交 CB 引用的资源保留到该提交完成（`OnSubmittedWorkDone` / readback 同步点）后才回收；与现有 `prevCmdBufs` 并存不替代 |
| **R4 缓存按解析身份失效 + 错误可捕获** | M4 bind-group key + validation error | bind group / 状态缓存 key = 解析结果身份，纹理重建自动 miss；GPU 错误是可捕获 error，不 abort |
| **R5 设备丢失衔接** | GrGpu device-lost 处理 | `Registry/Cache/Submission` 三 `Invalidate*`，native 释放保持 `abandonDeviceOwnedLocked` 单出口 |
| **R6 FFI 防御与错误上报** | GrGpu backend validation | `CreateBindGroup` entry 0 句柄 → 返回 `WGPUError`；生产路径 `PushErrorScope`+`PopErrorScope`（同步）+ `LastUncapturedError` 归因 |

## §3 目标架构：`render/internal/gpu/res` 包

> 新包 `res`（internal），只依赖 `gpu/webgpu` 与 `gpu/types`。命名对齐：`SourceKey`+`Ref`≈`GrSurfaceProxy`、`Registry`≈proxy→资源映射、`Cache`≈`GrResourceCache`、`Submission`≈command-buffer refs。

### 3.1 核心类型

```go
type Kind uint8 // KindTextureView / KindTexture / KindBuffer / KindSampler / KindBindGroup

// SourceKey：命令里的逻辑纹理引用，flush 时解析，绝不快照（GrSurfaceProxy 实例化时机）。
// Role：session_resolve / session_msaa / session_stencil（textureSet 三件套）
//       frame_scratch / layer_rt / cover_result / atlas_page / external（外部借用）
type SourceKey struct { Kind Kind; Role Role; Index uint32 } // Index: 0=当前活动实例；atlas 页=页号

// Ref：一次使用引用（GrGpuResource::ref）。ref>0 的资源绝不销毁。
type Ref struct { reg *Registry; id uint32 }

// Registry：id → 资源 + 引用计数。id 单调递增不复用（uint64，无 ABA）。
//   Register(res) Ref / Resolve(k) (res,bool) / Acquire(id) Ref / Release(ref)
//   InvalidateAll()  // 设备丢失：全部槽位置废，不调 native（§3.3）
type Registry struct{ ... }

// Cache：GrResourceCache。Key{W,H,Format,Samples,Usage,Role}；预算+LRU；in-flight 不借出。
//   Acquire(key) (Ref,error) / Release(ref)（归零回池/淘汰）/ InvalidateAll()
type Cache struct{ ... }

// Submission：一次 Queue.Submit 的生命周期。
//   Begin() / Track(ref) / SubmitDone()（OnSubmittedWorkDone 回调 或 readback 同步点）/ Invalidate()
type Submission struct{ ... }
```

### 3.2 关键语义

- **为何不用世代验证**：Skia 无世代号。安全来自 (a) 命令持 proxy、flush 时实例化 → 重建后解析到当前实例，**不丢帧**；(b) `Ref` 强引用保活，**无悬垂**。世代验证是"命令持快照"反模式的补丁，epoch 不符会跳过 draw = 把崩溃换成丢帧，属次优。epoch 最多保留为 **debug 断言**（开发期检测，对齐 `GR_DEBUG`；release 零开销，可整体删）。
- **flush 解析**：session 活动视图（resolve/msaa/stencil）→ `SourceKey` flush 时解析；rc 临时纹理（layer RT/offscreen/frameScratch/brush cover）→ `Ref` 保活。
- **提交点全景（M3 挂接）**：
  - 主漏斗 `submitWithLeading`（`render_session.go:3275`）；
  - 直连 `queue.Submit`（必须一并覆盖）：`gpu_render_context.go:366`、`render_session.go:4270`、`filter_gpu_graph.go:1240/1292`、`dual_tex_blend.go:785/1072/1542/1682/1761/1881`、`mask_r8_modulate.go:390`、`pattern_mask_sample.go:490`、`linear_ramp_mask.go:531`、`textured_stencil_linear.go:795/835`、`textured_stencil_pattern.go:735/773`、`stencil_renderer.go:783`、`sdf_render.go:662`、`gpu_texture.go:511`、`vello_accelerator.go:809`、`vello_compute.go:1219`、`renderer.go:816`。
  - `WGPUSubmissionIndex` 现成可用（`gpu/rwgpu/command.go:520-561`），但 `submitWithLeading:3275` 丢弃索引——P6 保留并按 index 对齐。
- **提交完成点**：纯 surface 模式 = `Queue.OnSubmittedWorkDone` 回调（**P6 前置：`gpu/rwgpu` 现无此 FFI，须新建**）；readback 模式 = 同步 `Map(MapModeRead)` 成功即完成（`gpu_texture.go:515` 等）。
- **与 `prevCmdBufs` 并存**：`render_session.go:449-466,611-623` 的跨帧 CB 保留 + `lastSubmitUsedSurface` 已是 M3 手工等价实现（VSync/WaitIdle 作屏障）。Submission 只新增"资源回收"挂架，不改 CB 保留机制；`SubmitDone` 触发点与 `BeginFrame` 释放点对齐。
- **grow 旧缓冲回收**：build* 6 处（`render_session.go:2297,2418,2718,2970,3558,3829`）容量增长时 Release 旧 buffer → 改为进 Submission 待回收队列（提交后 Release，与 `pendingBindGroupRelease` 同机制）。

### 3.3 设备丢失规则（R5）

现状：`gpu_shared.go:451` `abandonDeviceOwnedLocked` 全量清场（`WaitIdle` 兜底 + `WithForceNativeReleaseOnLost`（`:356,382`）AutoRecover 翻转）。

1. `Registry.InvalidateAll()` / `Cache.InvalidateAll()` —— 槽位/条目全部作废，**不调 native**（native 统一交 abandon 流程，保持单出口）。
2. `Submission.Invalidate()` —— 强制解除全部 in-flight（设备丢失后 fence 不会到达），后续 `Track` 短路。
3. 接入点：`abandonDeviceOwnedLocked` 之前，顺序 Submission → Registry/Cache（先断 fence 依赖再清资源表）。

## §4 迁移映射表（现状 → 目标）

| 现状 | 位置 | 目标 |
|---|---|---|
| `GPUTextureDrawCommand.View gpucontext.TextureView`（裸指针） | `gpu_types.go:14` | `View res.SourceKey`（session 视图/atlas 页）或 `res.Ref`（rc 临时/外部借用） |
| 入队点 9 处（`QueueGPUTextureDraw`/baseLayer/blit/stash/advanced） | `gpu_render_context.go:1077,1149,1171,1783,2023,2125,2403,2590,2618,2731` | 存 `SourceKey` 或 `Registry.Acquire` 得 `Ref` |
| 消费点裸指针强转 | `render_session.go:3609`；`image_pipeline.go:532`（指针比较） | `Resolve(SourceKey)` / `Ref` 取资源；身份比较 |
| BG 缓存按指针/uintptr 命中：`gpuTexBGSlotCache`、`imageBindGroups`、`textBindGroups`、`glyphMaskBindGroups`、`filterGPUCache.bgCache` | `render_session.go:3334,3120,359-390,2857`；`filter_gpu_graph.go:725` | key = 解析结果身份（`SourceKey`+资源 id）；纹理重建自动 miss |
| `textureSet`×4（session / stencil / textured pattern×2）+ SDF 独立纹理 | `render_session.go:233`；`stencil_renderer.go:38`；`textured_stencil_*.go:121,184`；`sdf_render.go:100-103` | `Cache`（Role=session_*）；resize=换 key+in-flight |
| `frameScratch` | `gpu_render_context.go:2282-2330` | `Cache`（Role=scratch） |
| `offscreenPool`（复用被禁用） | `gpu_render_context.go:2773-2816` | `Cache`（Role=layer_rt）；in-flight 后安全复用（P6 解禁） |
| `TexturePool`（生产空转，仅 DestroyAll/Stats 被调） | `texture_pool.go` | 并入 `Cache` 或废弃；`Stats`（预算观测）保留 |
| `ImageCache` 纹理（evict 编码阶段立即 Release） | `image_cache.go:262-275,347-348,184-187` | entry view 持 `Ref`；evict/`ReleaseEphemeral` 只标记，提交后释放 |
| glyph / MSDF atlas 页（固定尺寸，grow-only 至 Destroy） | `glyph_mask_engine.go:43-44,637`；`text_pipeline.go:933-934` | 保持现状；可选预算淘汰（GrAtlasManager 语义，非必须） |
| filter ping-pong + publishFree | `filter_gpu_graph.go:162-209,725` | `Cache`（Role=filter）；free-list cap 4 语义并入 |
| noMask 全套（session 固定资源） | `render_session.go:500-501,2174-2265`；`stencil_renderer.go:60-62` | 注册为 session 持有 `Ref` |
| 外部借用视图（surfaceView/textAtlasView/filter 视图/lastFlushedView） | `render_session.go:259,2879`；`render/context.go:108,116,124` | `SourceKey{Role: external}`：只跟踪不销毁，owner 在外部 |
| 帧内临时（convex ring / uniform slab / staging / stencilBufPool / clipBindPool / brushCover / LCD dest） | `render_session.go:276-292,3563,3630`；`gpu_texture.go:515`；`render_session.go:435,510`；`gpu_render_context.go:1083,1718` | 保持现状（时点已对齐提交；convex ring 已是手工 M3 语义） |
| `abandonDeviceOwnedLocked`（设备丢失） | `gpu_shared.go:451` | 前置 `Submission/Registry/Cache.Invalidate*`（§3.3） |
| `drainQueue()`（WaitIdle）垫底 | `render_session.go:537-579,756-790` | 保留为慢路径兜底，不再依赖它保纹理安全 |
| `CreateBindGroup` entry 0 句柄无防御 | `gpu/rwgpu/bindgroup.go:292-341` | entry 校验：任一 handle==0 → `WGPUError`（不 panic） |
| 生产路径无 error scope | `render_session.go` | `PushErrorScope`+`PopErrorScope`（同步）+ `LastUncapturedError` 归因（`gpu/rwgpu/errors.go:112`、`command.go:544`、`device.go:668`）；**禁用 `PopErrorScopeAsync`**（其内部 `waitForFuture` 1ms 轮询，`errors.go:132`+`future.go:27`，每帧调用拖帧） |
| `Queue.OnSubmittedWorkDone` 缺失 | `gpu/rwgpu/queue.go`（无） | P6 新建 FFI 绑定 + purego callback |

## §5 实施阶段（P1–P6）

> **防假绿总则**：单测用 fake native + 计数器断言"时机与次数"（非"没崩"）；真窗用 JSON/HUD 硬断言；`t.Skip` 必须带原因；JSON 门禁绿 ≠ 洞已修；每阶段验证 §6 全局矩阵（非单窗）。每阶段结束跑 `go test ./render/text/...` 按文件回归（M0–M3 全字集不回退）。
>
> **实施顺序注记（P3 前置分析定稿）**：入队点全景核实后确认——gpuTex 命令的视图来源都是"same-flush 内创建/使用/销毁"或"框架受控重建时序"（`textured_stencil_*.go` 为 ensure→用 函数内序、dual-tex 经 hold list 不入队、blit 在重建后入队），**没有直接快照 session 视图的入队点**。因此 resize 崩溃的直接修复是 **P4（textureSet/frameScratch 重建即销毁 → Cache + Ref 保活）**：旧视图 ref>0 活到提交后，命令快照自然有效。**P3（SourceKey 延迟解析）降级为语义优化**（消除"画旧尺寸一帧"），实施顺序改为 **P4 先于 P3**（在 §5 表中标注 P3↔P4 可交换，P4 优先）。

| 阶段 | 文件级改动 | 验收（防假绿） |
|---|---|---|
| **P1 res 骨架** | 新建 `res/res.go`（Kind/SourceKey/Ref/Registry）、`res/cache.go`、`res/submission.go` + 3 个测试文件（fake native + Release 计数器） | ① ref 归零+retire+无 in-flight 齐全才 `Release`（断言次数=1、时机正确）；② `Resolve` 重建后返回新实例；③ Cache 命中/未命中计数器；④ in-flight 项不借出；⑤ 预算淘汰等无 in-flight |
| **P2 FFI 防御** | `gpu/rwgpu/bindgroup.go`（entry 0 句柄→`WGPUError`）；生产路径 `PushErrorScope`+`PopErrorScope`（同步）+`LastUncapturedError` 归因，错误入 `slogger`+S6.x 指标 | 单测：0 句柄 entry→返回 error 不 panic；注入 validation error→被捕获进指标；**同步 Pop 无 1ms 轮询（计时断言）**。全局：矩阵全窗不 abort |
| **P3 命令句柄化** | `gpu_types.go`（View 改 `SourceKey`/`Ref` 联合视图）；入队 9 处；消费 2 处；`gpuTexBGSlotCache`；`image_pipeline.go:532`；改 ~10 个测试文件 | 单测：`Resolve` 时机（重建后解析到新实例，用 fake 断言身份）。全局：矩阵全窗 resize + 常规，`Resolve miss=0`、不崩溃不丢帧 |
| **P4 纹理集迁移** | `gpu_textures.go`、`gpu_render_context.go`（frameScratch/offscreenPool）、`sdf_render.go:100-103`、textureSet 使用方 3 处、`image_cache.go`（entry view 持 Ref） | 单测：换 key 后旧 ref 交 in-flight，提交前不回收/提交后回收；**shared-encoder 路径 evict 的 Release 延迟到提交后（v0.4 用例）**。全局：图像类窗（预算 evict）+图层类窗连续 resize，`Resolve miss=0`、显存≤预算、无静默丢帧 |
| **P5 缓存按解析身份失效+收敛** | `render_session.go`（imageBindGroups/textBindGroups/glyphMaskBindGroups key）；`filter_gpu_graph.go`（bgCache）；glyph/MSDF 页保持现状；`TexturePool` 并入或废弃 | 单测：纹理重建→BG miss→重建（新 BG 创建、旧 BG 进 pendingRelease）；atlas 换代→旧 BG 释放、新 BG 绑定新 atlas。全局：文本/字形类窗 + 滤镜/混合类窗全绿；`go test ./render/internal/gpu/...` 按文件回归 |
| **P6 Submission** | 新建 `gpu/rwgpu/queue.go` `OnSubmittedWorkDone` FFI + 测试；保留 `WGPUSubmissionIndex`（`submitWithLeading` 现丢弃）；挂接主漏斗+全部直连点；`prevCmdBufs` 机制保持不动；grow 旧 buffer 待回收；`abandonDeviceOwnedLocked` 前置 `Invalidate*`；offscreenPool 复用解禁 | 单测：FFI 回调；Submission 生命周期（SubmitDone 前不回收/后回收）；设备丢失 `Invalidate`（fake fence 永不回调，in-flight 强制解除、不调 native）；**grow 旧 buffer：prevCmdBufs 未完成时 grow→旧 buffer 不立即 Release（v0.4 用例）**。全局：resize 风暴+多图层+图像预算压力，显存≤预算、error scope=0、readback 类窗无过早释放 |

## §6 验收与回归（全局口径）

### 6.1 全局回归矩阵（每阶段真窗验收的唯一口径）

| 底层路径 | 代表真窗 | 关键断言 |
|---|---|---|
| 离屏 MSAA/resolve 纹理集重建 | `ui_wr_r18_savelayer`、`ui_wr_r4_composite`、`ui_wr_c0_smoke` | resize 时 `Resolve miss=0`；无 abort |
| 帧级中间纹理（frameScratch/dual-tex） | `ui_wr_r4_composite`、`ui_wr_r4b_multidamage` | usage 冲突提交错误=0；`Resolve miss=0` |
| 图像缓存（上传/evict） | `ui_wr_r5_picture`、`ui_render_base_image`、`ui_wr_r3b_compbits` | 预算压力下 evict Release 延迟到提交后；错误捕获=0 |
| 滤镜/效果（filter ping-pong/publish） | `ui_wr_r5_picture`、`ui_wr_r4b_multidamage` | 尺寸变化重建后 `Resolve miss=0`；publish free-list 无悬垂 |
| 文本/字形 atlas（页+BG 失效） | `ui_wr_r9_text_cache`、`ui_wr_r16_warmup`、`ui_wr_r0_fullpaint` | atlas 换代旧 BG 释放、新 BG 绑定新 atlas |
| 离屏 readback / 离屏渲染 | `ui_render_base_perfsoak`、`ui_wr_c1_boundary_nest` | buffer grow 无过早释放；readback 后 `SubmitDone` 正常 |
| 图层/合成边界（SaveLayer 预算） | `ui_wr_r18_savelayer`、`ui_wr_r2_paint`、`ui_wr_r21_shell` | 多图层 + resize 显存≤预算；error scope=0 |
| 设备丢失 / AutoRecover | 手动 TDR/驱动重置 + `ui_wr_r12_metrics` 观测 | `Invalidate*` 后无 native 误放；恢复后继续渲染（无 `resource already released`） |

### 6.2 指标硬断言与纪律

- **指标**（每窗 JSON/HUD）：`Resolve miss=0`（P3 后）、error scope 捕获=0（P2 后，设备丢失除外）、usage 冲突提交错误=0（P6 解禁后）、显存峰值≤预算（P6 后）。
- **诚实性**：计数接真实代码路径（`Resolve` miss 在 `Registry.Resolve` 自增；error scope 在 `PopErrorScope` 后自增），禁止测试写死；禁用 `PopErrorScopeAsync` 计帧。
- **单元**：`go test ./render/internal/gpu/...` 与 `./render/internal/gpu/res/...` 按文件逐个跑（禁止一次全量，长测单独 `-timeout`）。
- **回归**：每阶段 `go test ./render/text/...` 按文件全回归（M0–M3 全字集，坏字数不回退）。
- **纪律**：JSON 门禁绿 ≠ 洞已修；`t.Skip` 必须带原因；禁止降画质装绿。

## §7 修订

| 版本 | 关键变更 |
|---|---|
| 0.1 | 设计稿初版 |
| 0.2 | 去世代验证：改 `SourceKey`（flush 解析）+ `Ref`（引用计数保活）；epoch 降级 debug 断言 |
| 0.3 | 补 6 处遗漏（设备丢失、SDF 独立纹理、Buffer/slab、atlas 页、外部借用、帧内临时边界）+ `OnSubmittedWorkDone` FFI 缺失；阶段细化到文件级 |
| 0.4 | 再补 5 处实测遗漏（ImageCache 提交前释放、grow 旧 buffer、textBindGroups/glyphMaskBindGroups、直连 submit 点、`WGPUSubmissionIndex` 被丢弃）+ 2 处事实修正（TexturePool 空转、glyph/MSDF 页不随 LRU 释放）+ 1 处方案修正（禁 `PopErrorScopeAsync`） |
| 0.5 | 全局视角：定位为底层全局能力改造，验收改全局回归矩阵，单窗不崩不足验收 |
| 1.0 | 全文标准化重排（编号需求 R1–R6 对齐 M1–M4、§3.4/§4 合并、头部变更说明并入修订表、去除冗余） |
| 1.0+ | **实施登记**：P1 ✅ res 包骨架落地（`res/types.go`、`res/registry.go`、`res/cache.go`、`res/submission.go` + 3 测试文件 13 用例）；验收：test/vet/race 全绿，`go build ./render/...` 无破坏；实现含 Cache pending 队列（Release 时 in-flight → `OnSubmissionFinished` 回池）。P2 ✅ FFI 防御 + error scope：`CreateBindGroup` entry 级 0 句柄校验（`validateBindGroupEntries`，3 单测）；`SubmitPathStats.ErrorScopeErrors` 指标；`withSubmitErrorScope` 接线 `submitWithLeading`/`copySubmitAndReadback`（双提交点 Push/Pop 同步 error scope，错误入 slogger + 统计）；`go build ./gpu/... ./render/...` 无破坏，submit 路径既有测试绿（`TestMultipleAdapterRequests` 等需真实 wgpu native 的测试为环境依赖既有失败）。P4 ✅ 重建不销毁（resize 崩溃直接修复）：`textureSet` 加 `retireFn`（`gpu_textures.go`），session 在 `NewGPURenderSession` 注入延迟退役队列 `pendingTexRetire`，`destroyTextures` 改为"移交或立即释放"；`BeginFrame`/`SetSurfaceTarget`/`PurgeSurfaceTextures`/`Destroy` 四个安全点 `Drain`（对齐 prevCmdBufs 语义）；`frameScratch` 重建走 `session.RetireTexture`；`ImageCache.replaceRemove` 的 evict/替换纹理改 pending 队列，`ReleaseEphemeral` 释放点从帧末 defer 移到提交后（`submitWithLeading.runClean` + `copySubmitAndReadback` 提交后）；webgpu 加 `Released()` 诊断方法；P4 单测 4 用例 + 既有回归绿；`render/text` 与本次改动零代码耦合（仅依赖 internal/raster|cache），build 无破坏。P3 ✅ 命令视图句柄化：`GPUTextureDrawCommand.View` 从裸 `gpucontext.TextureView` 改为 `res.View`（SourceKey deferred / Ref direct 联合，对齐 GrSurfaceProxy 两态）；`res.View.Equals/RefID` + `Registry.ReleaseAll`（会话销毁兜底）；入队点（`QueueBaseLayer`/`QueueGPUTextureDraw`/`QueueGPUTextureDrawUV`/LCD base）经 `rc.viewToResView` 注册 `texViewNative` 到 session `resReg` 保活；消费点 `ResolveCommandView`（deferred 按 Role 解析当前活动实例 + 临时 ref 用完即放；direct 直接解包；miss → 安全跳过不送 wgpu）；`buildGPUTextureResources` defer 释放命令 Ref；**缓存 key 决策**：`gpuTexBGSlotCache` 保持"解析后原生视图身份"（指针）——若改用 Ref id 则同视图每帧新注册 id 不同、缓存永不命中，故回滚；测试适配 4 文件（s6_3/opt27/opt28/opt40 用 `Reg().Bind(Key, Register(texViewNative))` + `ViewFromKey`，循环 build 安全）。回归：`go build ./...` 全绿；全包测试仅 1 个既有环境失败（`TestSDFAccelerator_SceneStats_ResetOnFlush`——无 device 时 `Flush` 经 `ensureGPU` 失败提前返回不重置 stats，`pipeline_wiring_test.go` 未改动、非本次回归）。**真窗矩阵（§6）需 GPU 机执行**（本环境无 libwgpu_native）。P5 ✅ BG 缓存核对 + TexturePool 废弃：核对结论——`imageBindGroups`（texView+nearest）、`glyphMaskBindGroups`（atlasView+isLCD）、`textBindGroups`（atlas 换代经 `invalidateTextBindGroups` 重建）、`filterGPUCache.bgCache`（view/buffer 指针 key，pool 重建 `clearBGCacheUnlocked`）四类缓存的 key **均已满足"解析后原生视图身份"语义**（P3/P4 后自然成立）；旧 BG 统一进 `pendingBindGroupRelease`（提交后释放）；`TexturePool` 标注 Deprecated（生产 Acquire/Release 空转，`Stats` 保留给 S6.x 显存指标）；新增 `bg_cache_semantics_test.go`（glyphMask 同视图复用 / 换视图重建 + 旧 BG 延迟释放断言，GPU 环境跑、无 native 带原因 SKIP）。回归：全包仅既有环境失败 `TestSDFAccelerator_SceneStats_ResetOnFlush`。**待做：P6** |