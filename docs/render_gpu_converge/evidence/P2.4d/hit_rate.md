# P2.4d 第七段：同尺寸复用命中率探针（D7「无数字不标绿」）

> 2026-10-30。目标（台账 `ENGINE_RENDER_GPU_CONVERGE_TASKS.md:304`）：六处同尺寸管理器 → `res/cache`，**且复用命中率有探针**。前五段把池收完了，本段负责把「有没有真的复用上一块」变成能打印、能门禁、能证伪的数字。

## 1 探针必须回答的问题

`res.Cache` 收敛本身不产生收益——收益是**同尺寸第二次分配拿到旧 native 而不是新建**。所以探针要同时给出：每域命中/未命中、当前条目/空闲条目、字节/峰值字节/预算、被销毁的条目数（逐出 + purge）。只有 hits 没有分母 = 假绿；只有分配数没有命中 = 看不出池白搭。

## 2 三层出口（按分层方向注册，不反向 import）

`render` 不 import `render/internal/gpu`（`grep-residual.txt` §6 = 0 命中）。探针方向与 P2.4c 的 purge 链一致：**内部把回调注册进公共包**。

| 层 | 位置 | 职责 |
|---|---|---|
| 池本身 | `res/cache.go:290` `Cache.Stats()` → `CacheStats{Name,Hits,Misses,Evictions,Purged,Entries,Idle,Bytes,PeakBytes,Budget}` | 原始计数；`Idle` 口径见 `filter_dualtex.md` §2.4 |
| 域访问器（nil 安全，**绝不建池**） | `surface_cache.go:287`、`dual_tex_blend.go:1296`、`filter_gpu_graph.go:582`、`gpu_render_context.go:3867` | 未用过的域报空快照（`Name` 为空），不顺手 create 一个池来上报 0 |
| 设备持有者 | `gpu_shared.go:244` `poolStats()` | 汇总本设备的 `device-surface` + `dualtex-snap` + `filter-graph` + 每个在飞 context 的 `layer-offscreen` |

生产建 session 的两条路径（`gpu_render_context.go:2717`、`filter_flush_coalesce.go:79`）都传 GPUShared 的 `surfaceCache`，所以设备级四域就是生产全量。`session-surface` 只在「session 自持一套」（`render_session.go:752`，走测试/独立入口 `NewGPURenderSession`）时存在，不进 `GPUShared.poolStats()`——它没有设备持有者可挂，登记为探针覆盖面的已知边界。
| 公共出口 | `render/gpu_pool_stats.go`（106 行） | `PoolDomainStats`（带 snake_case json tag）+ `HitRate()` + `GPUPoolStatsFunc` + `RegisterGPUPoolStatsCollector(owner any, fn)` / `Unregister…(owner)` + `GPUPoolStats()` |

- 注册表按 **owner 句柄**（`any`）分键，与 `render.RegisterPurgeEvictable` 同一形态：`NewGPUShared()` 构造时注册（`gpu_shared.go:236`）、`Close()` 注销（`:661`），多设备/多持有者互不覆盖。
- 每个持有者带 `poolLabel`（`device-N`，来自包级 `atomic.Uint64 deviceSerial`，`gpu_shared.go:168/216`）：多窗真窗里能分清哪条数字来自哪块设备，Close 之后该 label 必须从输出里消失（`pool_stats_probe_test.go` 钉住）。
- 锁序：`poolStats()` 先在 `s.mu` 下取指针快照，**放出 `s.mu` 之后**再逐个取池锁——与 purge 链「不拿 `s.mu`」的约束同一方向。

## 3 真窗接线（`examples/wrgate`）

| 位置 | 内容 |
|---|---|
| `report.go:100` | `Pool []render.PoolDomainStats \`json:"pool"\`` ——报告恒带该段 |
| `report.go:258` | `BuildReport` 填 `render.GPUPoolStats()`（负向 N3 证明漏填会红） |
| `report.go:315` | `"pool"` 进 `RequiredSchemaKeys`：**缺段 = schema 不过**，把沉默读成通过的路堵掉 |
| `report.go:379/385/391/396` | `GateOptions` 四个池判据：`MinPoolHits`（总命中下限）、`MinPoolMisses`（总未命中下限）、`RequirePoolDomains`（列出名的域必须出现在探针里，只证「建过池」）、`RequirePoolHitDomains`（列名域每域至少 1 次命中；**域整个缺席也判 FAIL**） |
| `report.go:432` `EvaluatePoolGates` | 四条判据的实现，`EvaluateGates`（`:543`）在其余门禁前调用 |

对应的窗：

- `examples/ui_wr_r20_filter/main.go:540`：`RequirePoolDomains: [filter-graph, device-surface, layer-offscreen]` + `RequirePoolHitDomains: [filter-graph, device-surface]`。**不设总命中下限**：干净跑（无 resize 抖动）filter-graph 只有 6 次命中，100 是被首轮 748x541 压缩窗的 resize 抖动抬出来的假数字（`window-gates-run1-bad-env.txt`）。
- `examples/ui_base_multiwin_cap/main.go:659`：8 窗共设备，`MinPoolMisses: 8` + 要求 `device-surface` 命中。8 个窗在收口时各自持有自己那套 surface 的强引用，可借出的空闲条目极少，命中数结构上不可能到 100；用「至少 8 次未命中」证明 8 个窗都真的从同一设备池里分配过，再用「至少 1 次命中」证明池确实回收过被释放的条目。
- `examples/ui_l1_blank/main.go`：控制窗不做池门禁，但 JSON 里 `pool` 段由 `withPoolSection()` 附上 `render.GPUPoolStats()`，作为「纯分配场景下命中应为低/零」的对照；输入畸形时原样透传（探针不许吃掉主证据）。

## 4 测试（三处）

| 文件 | 断言 |
|---|---|
| `render/gpu_pool_stats_test.go` | 注册→快照→注销的顺序与幂等；nil 回调被忽略；`HitRate()` 表驱动（含 0/0→0，不许 NaN） |
| `render/internal/gpu/pool_stats_probe_test.go` | 四个域都带**真数字**上报（驱动 hits>0、misses>0、预算=声明值），`layer-offscreen` 建了 context 未用时报零而不建池；`Close()` 后该 label 从 `render.GPUPoolStats()` 消失；label 按持有者唯一 |
| `examples/wrgate/report_test.go` | `TestBuildReport_PoolSectionFromProbe`（桩采集器 → 域/数字进 JSON、`CheckSchema` 过）与 `TestEvaluatePoolGates`（关/总命中下限/总未命中下限/列名域零命中/域缺席/presence-only 命中为 0/多窗形态组合/通过） |

`res/cache_size_test.go` 的 `TestCache_StatsPerDomain` 是域内计数的底层钉；`direct_submit_test.go` 的 `Idle` 与 hits/misses 断言（见 `filter_dualtex.md` §2.4）保证「在飞不算命中」。

## 5 负向对照

`tools/negative.sh` N4：把 `BuildReport` 的 `Pool` 赋值改成 `nil`，`TestBuildReport_PoolSectionFromProbe` 必须红（域与数字一起消失），复原后绿。另三条负向（N1 每键上限、N2 字节预算逐出、N3 purge/OOM 链）见 `negative/pool-negative.txt`。

## 6 真窗数字（2026-10-30，隔离 headless Wayland 1600x1000@60）

数字来自 `window-gates.txt` 与同目录 JSON；全部窗口自己退出、无 kill 信号。R20 另在桌面 X11 `:1` 复跑 30s，用来把 headless 的环境 hitch 与窗自身红拆开。

| 窗 | 域 | hits | misses | evict | purge | entries/idle | bytes / peak | 结论 |
|---|---|---|---:|---:|---:|---:|---|---|
| `ui_l1_blank` 60s | `device-surface` | 0 | 1 | 0 | 0 | 1/1 | 614400 / 614400 | 纯分配对照：池建过、分配过、无复用 |
| `ui_base_multiwin_cap` 75s | `device-surface` | 4 | 32 | 8 | 24 | 0/0 | 0 / 20326528 | 8 窗都从同一设备池分配（miss≥8），并回收过（hit≥1） |
| `ui_wr_r20_filter` 30s（headless confirm） | `device-surface` | 531 | 46 | 0 | 6 | 0/0 | 0 / 16113552 | 同尺寸 surface 跨帧复用 |
| 同上 | `filter-graph` | 6 | 15 | 0 | 0 | 15/12 | 31740096 / 31740096 | 首轮建 15 条、次轮命中 6 次；没有 resize 抖动 |
| 同上 | `layer-offscreen` ×3 context | 0 | 1 each | 0 | 0 | 1/0 each | 162816 / 132912 / 132912 | 每 context 只分配一次，窗内无第二次同尺寸层 |
| `ui_wr_r20_filter` 30s（X11 复核） | `device-surface` | 567 | 46 | 0 | 6 | 0/0 | 0 / 16113552 | 与 headless 同结构 |
| 同上 | `filter-graph` | 6 | 15 | 0 | 0 | 15/12 | 31740096 / 31740096 | 跨环境逐项一致 |
| `ui_render_base_scene` 10s | `device-surface` | 0 | 3 | 0 | 0 | 3/3 | 15360000 / 15360000 | 三种尺寸各建一次，无复用需求 |
| 同上 | `dualtex-snap` | 0 | 1 | 0 | 0 | 1/1 | 65536 / 65536 | 唯一现有真窗负载只分配一次；复用路径只有单测覆盖 |
| `ui_base_vram_cap` 300s | `device-surface` | 28 | 27 | 3 | 24 | 0/0 | 0 / 30640464 | 8 窗 + 预算逐出 + purge 全链走过 |

判断口径：

- **hits 必须有分母**：每行同时给 misses；`ui_l1_blank` / `ui_render_base_scene` 的 0 命中是「没有第二次同尺寸申请」的结构性 0，不是探针坏。
- **R20 的 6 次 filter 命中是干净数字**：没有 resize 抖动时两遍（headless seed/confirm）与 X11 复核都是 6/15；首轮 748x541 的 102/27 是压缩窗 resize 产生的假复用，已随坏环境证据一起隔离。
- **multiwin 的 4 次命中是收口时全部窗口仍持强引用的结果**：门禁因此用 `MinPoolMisses:8` 证「8 窗都分配过」，再用 `RequirePoolHitDomains` 证「池回收过」，不追一个结构上做不到的总命中数。
- 窗自身门禁红与池探针分开记：R20 的 `scripted=4/5`（P1.4 基线同红）与 headless hitch 都不影响上面池数字的有效性；但按 D7 口径，`dualtex-snap` 只有 0 命中，复用能力仍只能算单测覆盖，不能标「真负载复用绿」。
