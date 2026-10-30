# P2.4c 影响面评估（开工前，2026-10-30）

> 性质：只评估，不动工作区（本文落盘 + 台账登记除外）。
> 依据：台账 §2.1 P2.4c（延迟释放统一：三队列 → `res/registry`；释放时序有单测）、PLAN §1.3/§2.2-B7/§7.3、`docs/render_gpu_converge/03_overlap_pools_retry.md` §1/§2、P0.4 已知差异 K1。
> 结论一句话：**今天生产里真正跑的是两条延迟释放队列（`pendingTex/Buf`、`ImageCache.pending+ephemeral`），第三条 `res.Cache.pending` 生产零实例；`res.Submission` 只有壳（`Track` 生产零调用）。收敛目标 = 把延后释放全部交给 registry 的 slot 状态机（refs/inflight/retire）+ 提交完成点，同时把 K1（conformance 每轮 +2 条 / +131072B）的少释放定位并修掉，修完把 conformance 上界收紧到 0。**

## 0 现场盘点（file:line 为实测）

### 0.1 三队列现状

| 队列 | 位置 | 存什么 | 释放点（实测） | 生产使用 |
|---|---|---|---|---|
| A `pendingTexRetire` / `pendingBufRetire` | `render/internal/gpu/render_session.go:102-141` / `:143-174` | 裸 `hal.Texture/TextureView/Buffer` 切片，`Add` 后等 `Drain()` 直接 `Destroy` | BeginFrame `:1068-1071`；离屏切换 `:985-986`；`PurgeSurfaceTextures` `:2107-2108`；`Destroy` `:2184-2185`；读回 `Map` 之后 `:5693-5694` | **在跑**：`retireFn`（`:789-791`）、`RetireTexture`（`:930-941`）、`RetireBuffer`（`:804-811`） |
| B `ImageCache.ephemeral` + `ImageCache.pending` | `render/internal/gpu/image_cache.go:81` / `:87`；`ReleaseEphemeral` `:251-273`；`removeEntry` `:441-455` | `*imageCacheEntry`（持有 hal texture+view） | **submit 返回后立即**（`submitWithLeading` 的 `runClean` `:4497-4502`，`queue.Submit` 是异步，**不等 GPU**）；读回 `Map` 之后 `:5695-5697`；`Destroy` `:329` | **在跑**（图片纹理） |
| C `res.Cache.pending` + `OnSubmissionFinished` | `render/internal/gpu/res/cache.go:40` / `:96-121` / `:125-145` | registry id | 由调用方在提交完成点手动调 `OnSubmissionFinished` | **生产零实例**：`res.NewCache` 仅 `res/cache_test.go`、`res/attack_test.go` 使用 |

### 0.2 registry / submission 现状

- 每 session 一份：`resReg: res.NewRegistry()` `render_session.go:777`；`sub: res.NewSubmission(resReg)` `:785`。
- `Registry` 状态机完整：`Register/RegisterBorrowed/Acquire/Release/Retire/Bind/Resolve/InvalidateAll/ReleaseAll`（`res/registry.go:75-289`），释放闸门 `removeIfReady`（refs==0 && inflight==0 && retire）。
- **`Submission.Track` 生产零调用**（全仓生产 grep 为空，只有 `res/registry_test.go`、`res/attack_test.go` 用）；因此 `s.sub.SubmitDone()`（`render_session.go:1073`、`:5699`）今天都是空转，`inflight` 恒 0。
- registry 的唯一生产用途是 **command view 借用记账**：`RegisterBorrowed`（`render_session.go:878`、`gpu_render_context.go:1471`）+ `retirePendingViews`（`:900-909`，同步点 `:1077`/`:5703`）。

### 0.3 时序对照（同一同步点、不同机制）

| 资源 | 今天何时真正 Destroy | 安全前提 |
|---|---|---|
| A 队列 | BeginFrame（present/vsync 屏障或 offscreen `drainQueue` 之后）/ 读回 `Map` 之后 / teardown `WaitIdle` 之后 | GPU 已完成 ✅ |
| B 队列（图片） | `queue.Submit` 返回后（**无屏障**）/ 读回 `Map` 之后 | native 依赖 wgpu Destroy 容错；GL 端 `glDeleteTextures` 与在飞提交存在竞争窗口 ⚠️ |
| C 队列 | 调用方提交完成点 | 设计正确，但无消费者 |
| command views | BeginFrame / 读回 Map 之后 | GPU 已完成 ✅ |

## 1 收敛设计（零行为，唯一语义变化 = K1 修复 + B 队列释放点后移）

1. **单机制**：所有延迟释放一律表达为 registry 槽位状态机——`Register(native)` → `Track(ref)`（提交期间 inflight+1）→ `Release(ref)`（refs→0）→ `Retire(ref)`；`SubmitDone()` 在既有同步点清 inflight，`removeIfReady` 统一 Destroy。**不再有裸切片队列。**
2. **A 队列**：`pendingTexRetire.Add` / `RetireBuffer` 改为「包一层 `res.Native`（view.Destroy+tex.Destroy）→ 上述四步」。`Drain()` 调用点全部删除，`SubmitDone()` 保留为唯一完成点（B 与 command views 也在同一处收口）。释放时机与今天相同（同一同步点），仅同一同步点内的顺序变成「transient views 先 Retire → SubmitDone 统一放」。
3. **B 队列（图片）**：`ImageCache` 增注入式 `retire func(view, tex)`（session 提供上面的 registry 退休口；测试构造时默认立即 Destroy，保持既有测试语义），`ephemeral`/`pending` 两个切片删除，`removeEntry`/gen==0 出队时直接退休。**释放点从「submit 返回后」后移到「下一同步点」**——更安全，净效果是图片纹理多活一帧（多窗账本峰值影响需在门禁里对照，见 §4）。
4. **C 队列**：`res.Cache.pending` 删除。registry 增加 **idle 回调**（槽位在 `refs==0 && inflight==0` 时回调一次，而不是 Destroy），`Cache.Release` 在 inflight>0 时只打标记，`SubmitDone` 触发回调自动回池；`OnSubmissionFinished` 随之删除。P2.4d 六处纹理收敛接线时直接复用该语义。`res` 是 internal 包，按「接口全量切换制」一次改完测试。
5. **诊断口径统一**：`PendingTexRetireCount` 改为 registry 里「已 Retire 未释放」计数；`Registry.Count()` 继续作为 live 条目数。conformance/生命周期测试直接读 registry 计数与 `hal.Vram*`，不再各读各的队列长度。

## 2 K1 定位与修复（本任务的硬产出）

- **已知事实**（P1.6 实测，native/go 逐位一致）：conformance 每轮（新建 `render.Context` → 画 → `Close`）后 `VramLiveCount +2`、`VramLiveBytes +131072`（= 2 × 65536）；`GPUContextCount` 恒 0（Go 侧 context 已销毁）；`clip_blend` 稳态不增长。
- **必须实测、禁止猜**：65536 恰好等于 `hal.VramPipelineBytes`（`gpu/hal/vram_ledger.go:100`），但也是 64KiB 缓冲/128×128 RGBA 纹理会出现的数；scene JSON 备注里的「brush/glyph/image textures」是旧假设，未证实。
- **定位步骤（实施第一步，先探后修）**：
  1. 给 `hal` 账本加**临时**env 门控追踪（`GPUI_VRAM_TRACE=1` 时 `VramAdd` 记录调用点 PC + need；`VramLiveEntries()` 输出 handle/bytes/调用点），跑 `TestConformanceDualBackend` 两轮以上，逐 handle 差分，列出第二轮新增且第一轮仍在的条目。
  2. 用调用点归位到创建方（pipeline / buffer / texture / sampler），核对它的拥有者与 `Destroy/Release` 路径；复现「创建→Close→未释放」的单测（无 GPU 也能用 fake device 断言 Destroy 调用次数）。
  3. 在**拥有者**修释放（不排除 pipeline 生命周期归属 session `Destroy` 的漏项；如果是跨文件小修，按「洞修了才能标绿」在本步一起修，不推后）。
  4. 修完把 `render/testdata/conformance/solid_gradient.json`、`text_image.json` 的 `max_steady_count_delta` / `max_steady_bytes_delta` 从 `2` / `131072` 收紧到 `0`，并作为本任务门禁；P2.6 基线重放按新值对账（台账 §2 已完成项注明 K1 已闭合）。
- 若探测发现 K1 根因不在延迟释放（例如设备级缓存），如实登记并拆到对应线，**不硬塞进本步**；但上界收紧必须由真实修复支撑，不许只改 JSON。

## 3 验证载体（开工前登记，硬）

- **能力单测**：
  - 新增 `render/internal/gpu/res/deferred_release_test.go`：① Track→Release→Retire 在 SubmitDone 前不 Destroy；② SubmitDone 恰好一次 Destroy（含重复 SubmitDone 幂等）；③ idle 回调只在 inflight 清零时回池一次、不过早；④ InvalidateAll/ReleaseAll 不误放/不漏放。
  - 新增 `render/internal/gpu/image_cache_retire_test.go`：注入 retire 钩子后，evict / 尺寸替换 / gen==0 三条路都不再直接 Destroy，而是交给钩子；无钩子（旧测试构造）仍直接 Destroy。
  - 沿用：`res/{registry,submission,cache,attack}_test.go`（语义等价，必要时按新接口改写断言）、`render/internal/gpu/s6_7_resources_test.go`、`budget_evict_test.go`。
- **真窗**：`examples/ui_l1_blank`（native，`RUN_SECONDS=60`，JSON 探针）；`examples/ui_base_multiwin_cap`（native `-auto-only`，8 窗生命周期 + 账本）；`examples/ui_base_vram_cap`（native `-auto-only`，`RUN_SECONDS=300`，峰值对账 P1.6 的 372MB 档）。全部在隔离嵌套 Wayland 上跑。
- **金图**：ui_l1_blank 设计内无金图（`baseline/windows.md` 记载），替代判定 = JSON 探针 + conformance 三场景 `frame.png` 与 P0.4 基线逐字节一致。
- **双后端**：conformance native + go 必跑（K1 收紧到 0 后两者都要绿）；真窗 native 必跑；go 真窗撞既有 C7（`eglMakeCurrent 0x3002`，P2.4e 修）如实登记，不阻塞本步。
- **负向对照**：conformance 上界收紧到 0 后，临时让一次 `Retire` 不生效（少退休一个图片条目）→ resource 断言红；还原复绿。另加 registry 单测级破坏：去掉 `inflight` 闸门 → 单测红。
- **证据**：`evidence/P2.4c/`（assessment / k1-trace / summary / build / vet / per-file-tests / race / conformance-rerun / lifecycle / window-gates / negative / api-diff / grep-residual）。

## 4 门禁

同 P2.4b 口径（五目标构建、vet 61/58 逐条不变、逐测试文件带 lib 且逐文件跑、API 快照 + 总账、残留 grep、负向对照、`-race`）+ 本步专项：

1. `res` 包单测逐文件 PASS（含新增时序测试）；
2. conformance 双后端 K1 上界 0/0 PASS；
3. 生命周期：multiwin native 8 窗三相位账本回落（对照 P2.4b 的 316MB 稳态）；vram ladder native 峰值对账 P1.6（372MB 档；ladder 8 窗卡死是既有 I1，不阻塞本步）；
4. ui_l1_blank native 正常开窗/关窗、账本在关窗后回落（`GPUShared` teardown 路径）；
5. B 队列释放点后移的显存峰值影响实测（ladder/ multiwin 峰值与 P2.4b 对照，差值逐条解释）。

## 5 预期 API 面

- `render/internal/gpu/res`（internal，不新增公开 API）：`Registry` 增 idle 回调用法（签名待实现细节定）、`Cache.OnSubmissionFinished` 删除；`Submission` 语义不变但开始被生产使用。
- `render/internal/gpu`：`pendingTexRetire`/`pendingBufRetire` 类型删除（或降为内部 wrapper），`ImageCache` 增 retire 注入口；`GPURenderSession` 内部口径调整。
- `render` 公开 API **零变化**（本步不导出新符号），`RENDER_API_CATALOG.md` 仅修订表加一行说明。

## 6 风险与对策

| # | 风险 | 对策 |
|---|---|---|
| 1 | 统一后释放时机提前/延后导致 use-after-free 或显存峰值升高 | 单测钉住「SubmitDone 前绝不 Destroy」；真窗账本峰值逐条对 P2.4b；必要时保留一帧延迟（与今天 A 队列一致） |
| 2 | `ImageCache` 释放点后移导致多窗峰值上升 | multiwin/ladder 峰值实测对照，差值写进 summary；若超门禁则回到「同一帧同步点」方案 |
| 3 | registry 引入 idle 回调复杂化状态机 | 只加一个「idle 时回调一次」分支，单测覆盖重复 SubmitDone/Invalidate 交叉 |
| 4 | K1 根因不在释放队列 | 先探后修；超出范围如实拆线，上界不硬改 |
| 5 | 逐测试文件轮时 GPU 用例假绿 | 沿用 P2.4a H1 纪律：`WGPU_NATIVE_PATH` 必带，逐文件记录 |
| 6 | go 档真窗 C7 阻塞 | 如实登记既有缺陷，本步不修（P2.4e） |

## 7 决策记录（2026-10-30 用户拍板，全部按推荐执行）

| # | 决策 | 采纳 |
|---|---|---|
| D1 | 收敛形态 | **registry 单一状态机**（A/B/C 全走 Register→Track→Release→Retire）；不做「三队列 + 公共 helper」的浅统一 |
| D2 | 图片纹理释放点 | **后移到提交完成同步点**（更安全，多活一帧）；若峰值超门禁再退回同帧 submit 后 |
| D3 | C 队列（res.Cache） | **本步直接删 pending 并加 idle 回池**（P2.4d 接线即用） |
| D4 | K1 探测工具 | **临时 env 追踪，定位后删除**（追踪数据落 evidence） |
| D5 | K1 修复范围 | **按根因就地修**（可跨 session destroy/pipeline 归属），跨文件修复逐条写进 summary |