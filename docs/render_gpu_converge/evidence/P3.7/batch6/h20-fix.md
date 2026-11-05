H20 修复账（2026-11-06，批 6；评估与拍板见同目录 `h20-assessment.md`，用户拍板「根修 + closed 守卫一起做」）
====================================================================================

一、根因（评估文档 §2–§3 的结论，此处只留一句话）
------------------------------------------------
滤镜图全局钩子（`registerFilterGraphIfNeeded`，`gpu_shared.go`）的三个闭包捕获注册时的
那份 GPUShared 且包级标志永不复位；`TestMem_*ResetAccelerator` 一族关掉那份 shared
后，下一个滤镜 op（`TestP04_ApplyDropShadowGPU`）经钩子把它**复活**（在上面装新设备、
发布视图），读回却走当前 shared 的设备 ⇒ 跨 instance 句柄喂 `CopyTextureToBuffer`
⇒ wgpu-core `storage.rs:137 no longer alive` 非 unwind panic，整进程 abort。
探针实测链（发布设备 `0xc000186180` ∈ 复活的 shared `0xc0001d3508`，读回设备
`0xc000186000` ∈ 当前 shared `0xc00013aa88`）见评估文档 §2（tmp/h20-probe2.log，tmp 不入库）。

二、改动（两文件，56 行增 0 删 + 公开口 +1）
--------------------------------------------
1. `render/filter_ops_gpu.go`：新增 **`UnregisterGPUFilterGraphs()`**（清三个钩子）。
   目录 §3.6 滤镜注册行已登记（apidoc rc=0）。
2. `render/internal/gpu/gpu_shared.go`：
   - 结构体 +2 字段：`closed`（Close 置位）、`filterGraphHooksRegistered`（注册归属）；
   - `GPUShared.Close()`：若本 shared 注册过钩子 → `render.UnregisterGPUFilterGraphs()`
     + 复位 `gpuFilterGraphRegistered`（**与既有 Unregister 三件套同位同纪律**——
     Close 本来就注销 `UnregisterPurgeEvictable/GPUPoolStatsCollector/
     ShaderEffectEvictor`，唯独漏了滤镜钩子，本修补齐）+ `s.closed = true`；
   - closed 守卫三处：`ensureGPU` / `initGPU` / `registerFilterGraphIfNeeded` 入口
     ——**已关闭的 shared 禁止复活**（返回错误，上层滤镜 op 收 err 优雅退 CPU）。

行为变化（如实声明）：`CloseAccelerator/ResetAccelerator` 后滤镜 GPU 钩子被清空，
窗口期内滤镜退 CPU（`GPUFilterGraphRegistered()==false` 的既有判定挡着），活 shared
下一次 `ensureGPU` 重挂钩子（所有 ensureGPU 路径本来就调 `registerFilterGraphIfNeeded`，
无需新接线）。正常单 accelerator 应用无感。

三、三枚钉
----------
| 钉 | 位置 | 断什么 | 负向 |
|---|---|---|---|
| `TestP376H20DeviceResetRunsToCompletion` | `render/p37_6_h20_test.go` | **子进程**跑 T2+P04 最小对（失败模式 SIGABRT，同进程一 occurrence 带走整个测试二进制，只能外部断）：断两枚真 `--- PASS`、无 `no longer alive`/`non-unwinding panic`、有 SKIP 判空钉、`ok` 行 | 两条修复全摘 ⇒ 实测红（见 §四 N3） |
| `TestP376FilterHooksFollowSharedLifecycle` | `render/internal/gpu/h20_filter_hooks_test.go` | 钩子随 shared 生死：注册后归属标记置位 → `Close()` 后三钩子全 nil + 内外两标志复位 → **已关闭的 shared 不许再注册** → 新 shared 能重挂 | 摘 Close 注销块 ⇒ 实测红 |
| `TestP376ClosedSharedStaysClosed` | 同上 | closed 不复活：`Close()` 后 `ensureGPU`/`initGPU` 必须返回错误且 `s.device` 仍 nil、`gpuReady` 不置位、不许成为钩子归属者 | 摘三处 closed 守卫 ⇒ 实测红（ensureGPU 在关闭的 shared 上装出了设备） |

四、负向对照（三条变异逐个实测，取完即还原）
--------------------------------------------
| 变异 | 钉A（pair 子进程） | 钉B（钩子生死） | 钉C（closed） | 定性 |
|---|---|---|---|---|
| N1 只摘 Close 注销（守卫在） | **红**（优雅红：钩子仍指死 shared ⇒ 滤镜 op 报错退 CPU ⇒ P04 自己的 `must GPUOps` 断言抓红，`last_cpu_fb=filter:cpu-fallback`，**无 abort**） | **红** | 绿 | 钉B 咬住注销；pair 也咬住（断言红） |
| N2 只摘三处 closed 守卫（注销在） | 绿 | 绿 | **红**（ensureGPU 在已关闭的 shared 上装出了设备） | 钉C 咬住守卫；注销单条撑得住 pair |
| N3 两条全摘 | **红**（本轮两轮实测均为「退 CPU 断言红」；HEAD 修复前的同一命令是整进程 abort——`tmp/h20-repro.log`，探针轮实测。abort 与优雅红取决于跨设备句柄在 wgpu 哪道校验先被拒，两种红都咬住这条根因） | **红** | 红 | 钉A/钉B/钉C 全红 |

**纵深结论如实记**：只摘任一条，pair 钉都绿不了被另一条挡住或优雅退 CPU 被 P04
断言抓红——两条独立都能挡住 abort，但各自的完整语义只有合在一起才成立（注销管
「钩子不再指死 shared」，守卫管「死了的 shared 拿不到设备」），各自的专属钉
（钉B/钉C）缺一不可。

五、门禁（逐项实测，先落文件再取 rc）
------------------------------------
- `go build ./...` rc=0；`go vet ./render ./render/internal/gpu` rc=0；
  `gofmt -l` 触及 4 文件 = 0 行。
- 三枚钉全绿（内部两枚 + 子进程钉 3.37s）。
- 复现对直接跑（同进程）：`--- PASS: TestMem_T2_ResetAccelerator` + `--- PASS:
  TestP04_ApplyDropShadowGPU`，ok 1.559s——修复前整进程 abort，修复后两枚双 PASS。
- `CGO_ENABLED=1 go test -race ./render/internal/gpu` rc=0（ok 37.821s，含两枚新钉）。
- `go run ./scripts/apidoc` rc=0（新口入目录 §3.6 滤镜注册行，§0 计数 195→196）。
- 整包 `go test ./render`：见 §六。
- 触及两包逐文件 sweep：见 §六。

六、整包跑到底 + sweep（标志性验收）
------------------------------------
**整包 `go test -count=1 ./render` 首次跑到底**：35.188s 结束（rc=1=在册红名单的普通
失败），`no longer alive` / `non-unwinding` / `invalid bind group layout` 命中 **0**——
H20 这个最后一个已知中止点消失，整包不再被进程级 abort 掐断（此前：H19 前停第 624 枚，
H19 后停 ~1376 枚）。

整包 18 枚 FAIL 逐个定性（隔离复跑 `-run` 同组单进程 + 干净 HEAD worktree
`gpui-h20-ab` 同命令 A/B，两棵树都实测）：
- **9 枚在册红**（隔离也红，与基线 failures.tsv 逐名对应）：TestMiscDrawGPUPath、
  TestPresentDamage_ImageMatchesView、TestRasterizerValueMatchesAnalytic、
  TestS69_HeavyBudget_TierGates、TestS69_Contract_FromJSON、
  TestTextTransformAutoRoutesOutlines、TestVideoDrawFrameMarksDamage、
  TestVideoBridgeFallbackConverged、TestVideoBridgePlanesFallback。
- **1 枚名单外 = 存量顺序耦合，非本修引入**：`TestDrawShadowDrawsBlurredOffsetSilhouetteOnly`
  （CPU 路径，`context_misc_draw_test.go:189`）。单测隔离绿；18 枚同组隔离命令在
  **HEAD worktree 同样红**（HEAD 9 红 vs 修复树 10 红，唯一差异是 S69 一枚在册墙钟
  贴线族抖动）⇒ 顺序耦合早于本修。此前整包在 624/1376 被掐断、逐文件口径该文件
  独跑绿，两头都看不见它——是整包首次跑到底才**暴露**的存量顺序耦合，按「不许当
  回归也不许当收益」如实登记。
- **8 枚整包顺序耦合红（隔离全绿）**：TestS3c_M3_{RecordingPlayback,BlendHue,
  ClipRectDifference,ClipRectReplace,DrawWithEmojiAPI,PathBooleanIntersect,
  TextUnderline} + TestR72_ApplyBlur_GPUFilterTextureNoExportPrefer。同组 18 枚隔离
  命令在 HEAD 与修复树行为一致（两树都不含它们）；HEAD 整包因 H20 跑不到该区段，
  无同命令对照——判定为整包口径新暴露的存量顺序耦合候选，整包口径基线此前
  不存在（以前根本跑不到底），这 8 枚留整包基线建账，不当回归不当收益。

**触及两包逐文件 sweep**（`tools/per-file-tests.sh /tmp/h20.pkgs tmp/per-file-h20`，
正口径）：**396 文件 = 376 PASS / 6 FAIL / 14 SKIP_TAGGED**，六枚红与 H19 轮逐名
相同（全在册），零新增红，两枚新钉 PASS；全文见同目录 `h20-sweep.txt`。

七、整包竞态证据（H19/H20 连环堵上后首次可拿）
----------------------------------------------
`CGO_ENABLED=1 go test -race -count=1 ./render`：**122.319s 跑到底，DATA RACE 命中 0，
abort 痕迹 0**——整包竞态门自 H19 登记起一直「不可用（跑不到底）」，H20 修后首次恢复。
红名单 19 枚 = 非 race 整包的 18 枚逐名相同 + `TestPresentResize_StormWindowCoversBudgetGap`
（race 慢速墙钟/预算贴线族：隔离复跑 rc=0 绿；非竞态、非回归，如实记）。
