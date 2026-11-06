H23 修复账（2026-11-06 · 后端拒收的模块：具名链 + 帧存活 + 按设备负面缓存）
==========================================================================

方案与判据正本：`h23-assessment.md`（拍板 A→B 改道见下 §二）；复现与三条量正本：
`gap3-backend-reject-probe.txt`。

一、修的是什么（一句话）
----------------------
一枚**前端 `NewShaderEffect` 放行、后端 naga 拒收**的用户材质模块（本机真触发子
=uniform 地址空间数组 stride 非 16 倍数），挂在**图像口/quad 口**上时：不具名、不计数、
直接把整帧 flush 打断；且失败不记档 ⇒ 每帧重打一次设备、每帧刷一整段 stderr。
填充腿当时是对的（具名 + 计数 + CPU 回退），本修把三条腿统一到同一条契约，
并给填充腿补上同一套缓存。

二、方案改道（如实记，不写成一次就选对）
--------------------------------------
- 评估里的 **P-1 A 档**（「把管线创建提到 uniform 缓冲之前，让错误落在已包具名错的那一步」）
  **被实测证伪**：诊断跑（临时把 `CreatePipelineLayout`/`CreateRenderPipeline` 提到建缓冲之前，
  逐步打错误）实测 `pipeline 步：<nil>`、`buffer 步：CreateBuffer: Validation Error`
  ⇒ wgpu 的校验错误是**排队**的，换顺序只是换个受害者调用背锅，归位靠顺序不成立。
- 改走 **B 档 = hal 错误域**，同一次诊断实测成立：
  `模块步 err=<nil> 域内=GPU Validation error: Validation Error`，且 pop 之后
  `缓冲步 err=<nil>` ⇒ 错误域既**同步归位**又**就地吃掉**（不再污染下一次调用）。
- hal 面：`gpu/webgpu/device_convert.go:106-129`（native，`PopErrorScopeAsync`）与
  `gpu/rwgpu/errors.go:36/124` 是真实现；`gpu/gwgpu/gles`、`gpu/gwgpu/metal` 是 stub
  （no-op / 恒 nil）⇒ 这两端拿不到域，但它们的 WGSL 前端本来就不拒这枚模块（③ 臂 G 实测
  纯 Go GL 接受并出图），故不构成本修的验收缺口。

三、落点（文件:行号，改后）
--------------------------
1. `render/internal/gpu/material_pipeline.go`
   - `materialCache.rejected map[*render.ShaderEffect]error`（H23 负面缓存），
     `ensureLocked` 建、`destroyLocked` 清、`evictEffect` 随 effect 关闭撤判
     （防地址复用误拒）；
   - `ensureMaterialPipeline` 进函数先查缓存 ⇒ 命中即零设备调用直接回具名错；
   - `CreateShaderModule` 包 `PushErrorScope(ErrorFilterValidation)` / `PopErrorScope()`，
     域内取到的错与同步错同路；
   - 新增 `rejectMaterialEffect(...)`：**首次**登记才 `s.matRefusalFails++`，
     模块步与管线步两条失败都走它（登记后即不再重试）；
2. `render/internal/gpu/render_session.go`
   - `buildImageResources` 材质循环：`ensureMaterialPipeline` 返回
     `errors.Is(err, render.ErrShaderBackendCompile)` ⇒ **这笔摘掉**（`refused[i]=true`、
     `noteMaterialRefusal()`、槽位仍推进以保持 `firstVertex` 对齐），其余画面照常；
     非具名错仍照旧上抛（不掩盖别的病）；
   - 出图段：`if cmd.Material != nil { if refused[i] { i = j; continue } }`——同一 effect
     才会成段（`canMergeImageDraw` 按 `a.Material != b.Material` 断段），所以整段跳是安全的；
   - `refused` 用会话级 grow-only scratch（`materialRefusedScratch`），且只在
     `matCount > 0` 时铺——实测教训：无条件 `make([]bool, len(cmds))` 每帧多一次堆分配，
     把 S6.9 重预算计时门一起拖红（7.71→7.98ms p50）；
   - 三个可观察计数：`matRefusalSkips`（每笔）、`matRefusalFails`（每 effect 首次）、
     `matRejectAttempts`（真正打到设备的模块创建次数，含成功）。
3. `render/internal/gpu/user_shader.go` + `user_shader_fill.go`（填充腿）
   - `userShaderCache.rejected` 同款缓存（生命周期同上三处）；`CreateShaderModule`
     也包进验证错误域；
   - `runShaderFillAsImage` 在尝试前记 `knownRejected`：首次 ⇒ 打一次 stderr +
     双 `%w`（批 3d 链不变）；重复帧 ⇒ 只回 `ErrFallbackToCPU`（不重复计
     `ShaderBackendFails`、不重复刷日志，但 `CPUFallbackOps` 照每帧记——那一帧的像素确实来自 CPU 腿）。
4. `render/internal/gpu/gpu_render_context.go` `TakeMaterialRefusals()` /
   `userShaderKnownRejected()`；`render/context_damage.go` 内部接口 `gpuContextOps` 加
   `TakeMaterialRefusals()`（unexported 接口 ⇒ 公开 API 面零变化，apidoc 实测绿）；
   `render/context_diag.go` `RenderPathStats()` 读时取走并折算进
   `pathStats.MaterialSkips` / `pathStats.ShaderBackendFails`（取走即清，重复读不累加）。

四、钉与变异（先红后修，红态留档）
--------------------------------
新增 `render/p37_h23_backend_reject_test.go` 四枚 + `render/internal/gpu/p37_h23_negative_cache_test.go` 一枚：

| 钉 | 断言 | 修前（红态原样） | 修后 |
|---|---|---|---|
| `TestH23ImageLegDoesNotBreakFrame` | flush==nil、`ShaderBackendFails>=1`、`MaterialSkips>=1`、材质区=裸底、同帧无材质对照仍在 | **FAIL**：`build image resources: material: uniform buffer: wgpu: failed to create buffer: CreateBuffer: Validation Error` | PASS |
| `TestH23QuadLegDoesNotBreakFrame` | 同上（quad 口） | **FAIL**：同一句 | PASS |
| `TestH23RejectionIsRememberedPerFrame` | 帧 1 计数、帧 2 `fails` 零增量但 `skips` 照记 | **FAIL**：帧 1 就被掀 | PASS |
| `TestH23FillLegUnchanged` | 填充腿现状不许变 + 重复帧不重复打设备（`fails` 增量 0、`cpu_fb` 照记） | 修前即 PASS（这条腿本来就对） | PASS |
| `TestP37H23NegativeCacheStopsDeviceRetries` | 3 帧 ⇒ `MaterialRejectAttempts()==1`；合法模块照常真建一次（防连带误拒） | —（新增） | PASS |

变异三条（各自还原）：

| 变异 | 结果 |
|---|---|
| N1 摘掉验证错误域（不 push/不 pop） | 三枚材质腿钉全红——错误又漂回 `CreateBuffer`，`ErrShaderBackendCompile` 判不出来 ⇒ 帧再被掀 |
| N2 摘掉出图段的 `refused` 跳段 | **只被图像腿那枚抓到**（`材质腿落墨了`）；quad 腿那枚当时没红 ⇒ 如实记为**部分有效变异**，不假装三条都咬住 |
| N3 摘掉 `ensureMaterialPipeline` 顶部的缓存查问 | 计数钉红：`3 帧打了 3 次设备编译（want 1）`、`合法模块 attempts=4 want 2` |

五、真窗复验（单窗一轮、WM_DELETE 自然停机、每轮查 Xwayland 存活）
----------------------------------------------------------------
| 窗 | 后端 | 结果 |
|---|---|---|
| ③ `p37_material` | native | `probes=20/20 golden=0.0000%`，`arm_g_frame_check=拒收端同形（底色 + 计数≥1）`，**`window_shader_backend_fails` 239→1**，stderr `pass1 err` **243→2 行**（首帧 + 离屏探针各一次） |
| ③ `p37_material` | go | `probes=20/20 golden=0.0000%`，`arm_g_frame_check=接受端同形（shader 解析色 + 计数 0）` |
| ⑥ `ui_wr_p37_scene` | native | `15/15 golden=0.0000%`、`skips=0` |
| ① `p37_persp_batch` | native | `15/15 golden=0.0000%` |

金图零变化 = 本修只改「拒收时怎么办」，不改任何可用材质的画面。

**归因更正（把我今天先前写下的错话改回来）**：③ 臂 G 那格每帧的 p95 开销
**不是**每帧重编译。修后 native p95=50.48ms（修前 51.13ms），而 go 端（该模块被接受、
无拒收）p95=27.87ms ⇒ 差额来自**每帧走 CPU 回退腿画那一格**本身，与编译无关。
负面缓存真正省下的是 238 次设备模块创建 + 241 段 stderr 洪水。`RUN_SECONDS=20`
这条仍然要（8 秒跑不满定帧 240），但理由要按本段改。

六、门禁账
----------
- 触及两包**逐文件** sweep（正本已抄进本目录 `h23-sweep.txt`；411 行 =
  逐状态实测 **PASS 386 / FAIL 6 / NOTEST 6**（余 13 行是 FAIL 的子测试明细与收尾标记）：
  `./render` + `./render/internal/gpu` 共 6 个文件红——`context_misc_draw_gpu`、
  `filter_present_coherence`、`rasterizer_tile_value`、`text_transform_gpu`、
  `video_direct_upload`、`s6_9_heavy_budget`；前五枚在 HEAD worktree **同口径隔离复跑同样红**
  （`rc=1`，FAIL 数一致）⇒ 存量族，**本修新增红 0**；`internal/gpu` 整包零红。
- 第 6 枚是 `TestS69_Contract_FromJSON`：它读的是 `tmp/s6_9_heavy_budget.json`，
  而同一文件里的 `TestS69_HeavyBudget_TierGates` 会在整包跑时**重写这份基线**——
  我这次整包跑在机器忙的时刻写下 p50=7.98/0.34，随后 Contract 拿它当门必然假红
  （删掉自生文件后 HEAD 与本树同为 SKIP）。登记为测试工装洞（见 §七）。
- `-race`：`./render/internal/gpu` rc=0、`--- FAIL` 0、`DATA RACE` **0**；
  `./render` 整包 `-race`：`DATA RACE` **0**、`--- FAIL` 29（含子测试；与不挂 race 的整包跑
  同源，全在下条的在册族里）⇒ 竞态门过。
- `go build -tags nogpu ./...` rc=0；`go vet ./render/...` rc=0；gofmt 净（存量三枚脏文件除外，非本线）。
- `go run ./scripts/apidoc` rc=0：公开符号面零变化（新加的 `TakeMaterialRefusals` 挂在
  **unexported** 接口 `gpuContextOps` 上）。

七、顺带盘出的两条存量（不归本修，按归位纪律登记）
------------------------------------------------
1. **nogpu 测试面断口**：`go vet -tags nogpu ./render/...` 在
   `render/internal/gpu/shared_layouts_test.go:36` 报 `undefined: createClipBindGroupLayout`
   —— 该文件缺 `//go:build !nogpu`（HEAD 同样复现，非本修带入；本修的新测试文件已带标签）。
   属批 6 收官时修掉的那类「nogpu 断口」漏网。**✅ 已收口（2026-11-07，提交 `ee514b42`）**：实修时顺藤盘出同族共 **5 枚**（shared_layouts/3b/3c/3d/h23_backend_reject），各补一行 `//go:build !nogpu`，vet -tags nogpu ./render/... rc=1→0。
2. **自重写基线的计时门**：`render/s6_9_heavy_budget_test.go` 一个文件里既有测量者
   （`TestS69_HeavyBudget_TierGates` 写 `tmp/s6_9_heavy_budget.json`）又有契约校验者
   （`TestS69_Contract_FromJSON` 读它当门）⇒ 任何整包跑都会让 Contract 用「本次机器状态」
   判红。与台账在册的 `TestCullBudgetLarge` 同族（浮动计时门不作收口证据），
   建议要么把基线文件钉成 `testdata/` 里的受版本控正本，要么把 Contract 判据降为只报数。**✅ 已收口·结构修（2026-11-07，提交 `4b33185c`，账 `pit2-s69-contract-fix.md`）**：writer→TempDir + reader→testdata 冻结正本（暂缺具名 Skip）；四轮实测证明 all-PASS 冻结对在现有门限下取不到（U05 相邻两轮抖 11% vs 门容 +0.3ms），正本待性能门重定标另线落定后 cp 即转真。
