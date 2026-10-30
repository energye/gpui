# P2.4d 同尺寸纹理分配收敛（六处 → `res.Cache`）收口

- 完成日期：2026-10-30
- 依据：台账 §2.1 P2.4d 载体、`assessment.md`（六处盘点 + D1–D7）
- 一句话结论：**六处同尺寸纹理管理器全部收敛到 `res.Cache`（四个生产域：`device-surface` / `layer-offscreen` / `filter-graph` / `dualtex-snap`，加测试缝 `session-surface` 共五域），配套 registry 设备级化 + `hal.CompletionToken` 直提闸门 + `SizeFn/MaxPerKey/PurgeIdle` 能力 + purge/OOM 链闭合 + 命中率探针三层出口；门禁全部跑过，真窗数字见 §5。**

## 1 六处 → 五域

| # | 原管理器（HEAD `ac2e58b3`） | 现在 | 归属 | 证据 |
|---|---|---|---|---|
| 1 | `TexturePool`（`texture_pool.go`，生产零调用） | 整文件删除；`BudgetTexturePool` → `BudgetSurfacePool`（128MiB 原值） | — | `remove_texturepool.md` |
| 2 | 四个 `textureSet` owner 各自自持的 MSAA/depth/resolve 套装 | 设备级 `surfaceCache`（域 `device-surface`，owner = `DeviceOwner`）；owner 侧字段形状保留，生命周期走 `res.Ref` | `GPUShared` | `surface_pools.md` |
| 3 | `stencilPool`（`gpu_textures.go`，size-keyed cap 8 手写 LRU） | 合并进同一 `surfaceCache`；每 key 上限取 `BudgetStencilScratch.MaxEntries` | `GPUShared` | `surface_pools.md` |
| 4 | `offscreenPool`（`gpu_render_context.go`，桶 4 / 32MiB 手写） | 每 context 一个 `res.Cache`（域 `layer-offscreen`），Close→`ReleaseAll`+`ReleaseOwned` | 每 `GPURenderContext` | `surface_pools.md` |
| 5 | `filterGPUCache` 乒乓（单尺寸、换尺寸全销毁、直提无 fence） | 设备级 `res.Cache`（域 `filter-graph`，`BudgetFilterGraph` 8 条/尺寸 + 128MiB），publish 槽带 `ref` | `GPUShared` | `filter_dualtex.md` |
| 6 | `dualTexBlendCache.outPool`（64 对齐手写桶 + LRU + 预算） | 设备级 `res.Cache`（域 `dualtex-snap`，`BudgetBlendDualTex` 32MiB / 8 条） | `GPUShared` | `filter_dualtex.md` |

直提路径（filter/dualtex 自己 `Queue.Submit`，不进 session 帧）由 `directSubmit` + `res.InflightGate` + `hal.CompletionToken` 接管：提交拿 token，hold 到 token 完成才允许回池/销毁；设备丢失走 `invalidate()`（不减在飞计数、不猜完成）。

## 2 设施补齐（D2–D7）

| 决策 | 落地 | 证据 |
|---|---|---|
| D2 registry 设备级 | `Registry` 挂 `GPUShared`（每 device 一份、加锁）；`Owner` 句柄 + `slot/roleKey.owner` 分账；session `Destroy`→`ReleaseOwned`、设备丢失→`InvalidateOwned` | `registry_device.md`、`res/owner_test.go`、`session_registry_owner_test.go` |
| D3 hal 完成 token | `hal.CompletionToken{Done,Wait}` + `Queue.OnSubmittedWorkDone`；native/gles/metal/noop 五端；顺带修 v29 回调 ABI 三处 + 空 future，T1 转绿 | `hal_token.md` |
| D4 Cache 能力 | `Role` 进键、`SizeFn` 真实字节（零值兜底 4096）、`MaxPerKey`、`PurgeIdle`、`CacheStats` 按域；锁序 cache→registry；`CreateFn` 锁外跑 | `cache_caps.md`、`res/cache_size_test.go` |
| D5 预算域 | `BudgetTexturePool`→`BudgetSurfacePool`；新增 `BudgetFilterGraph`（等价上限 8 条/尺寸 + 128MiB） | `render/budget.go`、`budget_default_test.go` |
| D6 purge/OOM 链 | `gpu-shared-pools`（dualtex + filter）注册 `RegisterPurgeEvictable`；surface 走 session `PurgeSurfaceTextures`→`surfaceCache.purgeIdle`；context offscreen 走自身 `PurgeIdle`；`pool_purge_lock_test.go` 钉住 purge 不拿 `GPUShared.mu` | `surface_pools.md`、`filter_dualtex.md` |
| D7 命中率探针 | `Cache.Stats()` → 域访问器（nil 安全、不建池）→ `GPUShared.poolStats()` → `render.GPUPoolStats()`；wrgate `pool` 段进必检 schema + 四判据（`MinPoolHits`/`MinPoolMisses`/`RequirePoolDomains`/`RequirePoolHitDomains`） | `hit_rate.md` |

## 3 能力单测（全部 noop 设备，无 GPU 不 skip）

| 文件 | 覆盖 | 结果 |
|---|---|---|
| `res/cache_size_test.go`（5） | 真实字节预算 / 每键上限 / PurgeIdle / 按域统计 / 并发 | PASS |
| `res/gate_test.go`（8） | hold 阻塞再借与销毁、retire 等 resolve、invalidate 不减计数、双 hold、nil/外来 ref、settleAll、并发 | PASS |
| `res/owner_test.go` + `session_registry_owner_test.go` | 设备级 registry 按窗分账：双窗共 registry 只关本窗、失效/回调重入/并发 | PASS |
| `render/internal/gpu/surface_cache_test.go`（12） | 两 owner 同尺寸不同原生、模式/样本进 key、degraded 即销毁、每键上限、预算逐出、purge 只收 idle、ReleaseAll/InvalidateAll、探针、生产限制、owner 路由 | PASS |
| `direct_submit_test.go`（5） | hold 未解不回池、token 完成回池命中、submit 失败当场关闭、无 token 只 drain、invalidate 永久失效、无 gate 裸提交 | PASS |
| `pool_purge_lock_test.go` | 注册→purge→释放；purge 不拿 `GPUShared.mu`；两域都在 OOM 链上 | PASS |
| `pool_stats_probe_test.go` | 四域真数字上报、`layer-offscreen` 未用不建池、Close 后 label 消失 | PASS |
| `render/gpu_pool_stats_test.go` | 注册/注销幂等、nil 忽略、`HitRate` 表驱动（0/0→0） | PASS |
| `offscreen_pool_r6_test.go`、`budget_evict_test.go`、`retire_queue_test.go` 等 | 改写为 `res.Cache` 域断言；调用点随 refs/track 改 | PASS |
| `examples/wrgate/report_test.go` | `pool` 段由探针进 JSON + `EvaluatePoolGates` 八组判定（含 `MinPoolMisses` 与 presence-only 域） | PASS |

## 4 门禁结果

| 门禁 | 结果 | 证据 |
|---|---|---|
| 五目标构建 | ✅ linux×2 321/317、windows 327/323、darwin×2 319/315；段 3/4 全 exit 0；raw 红仅 2 个已知破损包（`ui_wr_t_stress`、`ui/scene/internal/gpupixel`） | `build/*.txt` |
| vet | ✅ 默认 61 / nogpu 58，与 P1.2 基线 **119 条逐条同名同文案、零新增零消失**；仅 2 条 `unreachable` 行号 +47 漂移 | `vet-full.txt`、`vet-diff.txt` |
| 逐测试文件（9 包） | ✅ 421 文件：375 PASS / 17 仅编译 / 1 标签排除 / 28 FAIL；28 = P2.4c 27 条同名 + `gpu/webgpu/swapchain_x11_linux_test.go`（干净树 `ac2e58b3` 同断言复现，判既有 I2） | `per-file-tests.txt`、`per-file-tests-ab-clean.txt`、`per-file-tests-notes.md` |
| `-race` | ✅ 4 包 exit 0；脚本先 `-list` 打印入选 116 条，防正则拼错沉默通过 | `race.txt` |
| conformance 双后端 | ✅ 真跑：3 场景 × native/go 全 `status=ok`、6 帧与 P0.4 基线逐字节一致、资源上界 0/0；无 SKIP | `conformance/summary.md`、`rerun-after-pool-convergence.txt`、`conformance/reports/` |
| 负向对照 | ✅ N1 上限 / N2 预算逐出 / N3 OOM 链 / N4 探针漏填，四条红→绿、残留 0（N2 第一轮注错位置如实登记后改注执行点） | `negative/pool-negative.txt` |
| API 快照 | ✅ `render` 仅预期变化（`BudgetTexturePool`→`BudgetSurfacePool`、新增 `BudgetFilterGraph`、池探针四符号）；`gpu/hal` 仅新增 token；res/internal 面按 P2.4d 设计变化；apidoc rc=0 | `api-diff.txt`、`api/`、`apidoc-check.txt` |
| 残留 grep | ✅ 六处旧符号整词 0（仅注释历史名）、按尺寸裸 map 0、`res.NewCache` 4 个生产调用点、render 后端特有类型 0 | `grep-residual.txt` |
| 真窗 | ✅ 见 §5（headless 1600x1000 五窗 + R20 X11 复核；R20 窗自身既有红 / I1 未复现均如实登记） | `window-gates.txt` |

## 5 真窗与生命周期（隔离 headless Wayland 1600x1000@60 + 桌面 X11 单窗复核，标准关闭）

**环境**：隔离 `gnome-shell --headless --wayland-display=gpui-p24d --virtual-monitor 1600x1000@60 --no-x11`（gbm 走真 GPU，不设 `LIBGL_ALWAYS_SOFTWARE`）。每轮跑前 `connect()` 验活 Wayland socket + 查桌面 X `:1`，跑后再查 X `:1`；全部窗口靠 `RUN_SECONDS` / `-auto-only` 自己退出，未发任何 kill 信号，最终一轮没有 rc=124。第一轮用 nested Wayland（stage 固定 800x600）把 1200x800 的窗压成 748x541，multiwin 金图 diff 99.79%、R20 像素探针错位、ladder 8 窗 rc=124；该轮完整保留在 `window-gates-run1-bad-env.txt`，只作环境反证，不作为数字。最终跑：`window-gates.txt`（2026-10-30 18:32–18:40，HEAD `ac2e58b3`）。

| 窗 | 运行 | rc | 关键结果 | `pool` 段（hits/misses） | 判定 |
|---|---|---|---|---|---|
| `ui_l1_blank` | native 60s | 0 | 3497 presents、58.3 fps、正常关闭 | `device-surface` 0/1，idle=1，614400 B（峰值同），预算 128MiB | 纯分配对照：池建过、分配过、0 命中；Close 前采样（探针时序已修） |
| `ui_base_multiwin_cap` | native 75s `-auto-only` | 0 | 8 窗、12559 presents、`max_windows_60fps=4`、16/16 金图 diff=0、RSS 307→312MB | `device-surface` 4/32，evict=8，purge=24，峰值 20326528 B（19.4MiB）/128MiB | 池门禁 `MinPoolMisses:8` + `device-surface` 命中 → PASS |
| `ui_wr_r20_filter` seed | native 30s（headless） | 1 | hitch 98.12/min；scripted 4/5（既有红）；播种 `r20_final_base.png` | `device-surface` 543/46；`filter-graph` 6/15（12 idle，31740096 B）；`layer-offscreen` 0/1 ×3 | 池三域 presence + filter/surface 命中 → PASS；窗自身红见下 |
| `ui_wr_r20_filter` confirm | native 30s（headless） | 1 | 金图 diff=0.0000% / 306280 px；hitch 92.06/min；scripted 4/5 | `device-surface` 531/46；`filter-graph` 6/15（12 idle，30.3MiB）；`layer-offscreen` 0/1 ×3 | 同上；headless 的 hitch 是环境性（见 X11 复核） |
| `ui_wr_r20_filter` confirm（X11 复核） | native 30s（桌面 `:1`） | 1 | hitch 6.0/min（低于 30 门禁）；金图 diff=0；只剩 scripted 4/5 红 | `device-surface` 567/46；`filter-graph` 6/15；`layer-offscreen` 0/1 ×3 | 同一池数字跨环境稳定；把「环境 hitch」和「既有像素回归」拆开 |
| `ui_render_base_scene` | native 10s `-auto-only` | 0 | 601 presents、58.7 fps、金图 diff=0 / 627200 px、parity 1.4376% valid | `device-surface` 0/3（3 idle，15360000 B）；`dualtex-snap` 0/1（1 idle，65536 B）/32MiB | dualtex 域唯一现有真窗负载只走到「建池 + 分配」，没有第二次同尺寸复用；0 命中如实登记，不据此标复用绿 |
| `ui_base_vram_cap` | native 300s `-auto-only` | 0 | 8 窗、32834 presents、`ledger_peak=372.2MB`、16/16 金图 diff=0；n=8 相位 `fps_min=13.3`、live 359.3MB、hold 斜率 975KB/min | `device-surface` 28/27，evict=3，purge=24，峰值 30640464 B（29.2MiB）/128MiB | PASS；峰值与 P1.6 的 372MB 档对上 |

X11 复核的命令、原始 rc/hitch/池数字见 `window-gates-x11-crosscheck.txt`。

**R20 窗自身仍红，两条都不是 P2.4d 引入**：

1. `gray_red_band_matrix_exact @(408,274) got=(0.898,0.200,0.149) want=(0.404,0.404,0.404)`：P1.4 基线 native seed/confirm 的 stderr 是同一行、同一 got/want（`baseline/windows/ui_wr_r20_filter/default/native/{seed,confirm}/stderr.log:5`），P2.4d 干净树之外就已存在。R20 关闭于 2026-08-25（`6a752a5d`），P1.4 采集前只有 `b7deb056`（2026-10-23，hand layers immutable recordings）动过 layer recording，是当前疑点；**没有做该提交前后的 A/B，不下因果结论**，解剖归 §2.2 I3。
2. headless 下 `hitch_rate_per_min=92.06 > 30`：同一二进制在桌面 X11 `:1` 跑 30s 是 6.0/min（P1.4 基线 17.98/min），金图同样 diff=0，说明 hitch 是 headless 合成器环境性；不退门禁、不改门禁。
3. `r20_steady.png` / `r20_recover.png` 全黑（5585 B，1200x800 全 0）：P1.4 归档的基线两张图就是同样的全黑文件，本轮 stderr 另有 `embedder: snapshot request timed out`；登记 §2.2 I4，归 SnapshotAsync 既有问题，不作为 P2.4d 回归。

**I1 说明**：最终 headless 环境里 ladder 8 窗正常退出（rc=0、n=8 `fps_min=13.3`、live 359.3MB、峰值 372.2MB），**没有复现** nested Wayland 的 `present` 卡死（run1：n=8 `fps_min=0.0`、live 199.7MB、rc=124）。这只说明换到 headless 后该路径不复现，不能当成 I1 已修；I1 仍按 §2.2 开在 P2.4e。

## 6 留给 P2.4e 的偏差（如实记）

1. `filter_flush_coalesce.go` 的本窗 flush 口仍走 `rc.session.trackRef`（整帧粒度的 submission），不是设备级 gate 的单批粒度；提交本来就在 session 帧内，按 D3 归属正确。
2. 该口的 `srcView` 记账沿用 P2.4c 之前形态（视图本身不经 gate hold），因为它是上游 session 已持有的资源、不是池借出的条目。
3. `directSubmit` 只在 `GPUShared` 存在时装配；裸 `res.Cache` 字面量（测试缝）与 `resOwner` 为 nil 的路径继续走「直建直毁」兜底。
4. 换尺寸不再全销毁 filter RT 之后，`BudgetFilterGraph` 的 128MiB 成为新的显存上界来源；真窗若出现抬升应调预算，不回退行为。

## 7 改动清单（仅本线）

- 引擎：`gpu/hal/api.go`、`gpu/hal/noop/queue.go`、`gpu/webgpu/queue.go` + `queue_token.go`、`gpu/rwgpu/queue_workdone.go`、`gpu/gwgpu/gles/queue_token.go`、`gpu/gwgpu/metal/queue.go`；`render/budget.go`、`render/gpu_pool_stats.go`；`render/internal/gpu/{res/*,surface_cache,direct_submit,gpu_textures,render_session,gpu_shared,gpu_render_context,filter_gpu_graph,filter_flush_coalesce,dual_tex_blend,stencil_renderer,textured_stencil_*}.go`。
- 删除：`render/internal/gpu/texture_pool.go`、`stencil_pool_prune_test.go`。
- 测试：新增 `res/{cache_size,gate,owner}_test.go`、`surface_cache_test.go`、`direct_submit_test.go`、`pool_purge_lock_test.go`、`pool_stats_probe_test.go`、`session_registry_owner_test.go`、`render/gpu_pool_stats_test.go`、`gpu/*/queue_token_test.go`；改写 `budget_evict_test.go`、`offscreen_pool_r6_test.go`、`retire_queue_test.go`、`report_test.go` 等。
- 示例：`examples/wrgate/report.go`（pool 段 + 门禁）、`ui_l1_blank` / `ui_base_multiwin_cap` / `ui_wr_r20_filter`（探针接线与门禁）。
- 文档：`docs/RENDER_API_CATALOG.md`、`docs/ENGINE_RENDER_GPU_CONVERGE_TASKS.md`、`docs/render_gpu_converge/{02_api_gpu,03_overlap_pools_retry}.md`。
- 证据：`evidence/P2.4d/`（assessment / summary / hal_token / registry_device / cache_caps / remove_texturepool / surface_pools / filter_dualtex / hit_rate / build / vet-full + vet-diff / per-file-tests(+ab-clean+notes) / race / conformance / negative / window-gates(+run1-bad-env + x11-crosscheck) / r20_snapshots + r20_snapshots_x11 / api(+diff) / apidoc-check / grep-residual / tools）。

## 8 下一步

- P2.4e：I1（多表面 present 卡死）、C7（gles 多窗上下文）、I2（swapchain 同尺寸 Resize 语义）、本步四条偏差与 G2 入口收敛。
- P2.6 前诊断：I3（R20 灰度矩阵既有红，先做 `b7deb056` 前后 A/B）、I4（R20 steady/recover 黑图，SnapshotAsync 既有问题）。
