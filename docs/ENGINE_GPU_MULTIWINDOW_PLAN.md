# 多窗高可用 GPU 方案需求（低占用后端 + 任意数量程序共存）

> 地位：设计与施工总纲。分两块独立交付，每块内按阶段验收、独立可回滚。
> 日期：2026-09-05。
> 触发：同机开第二个真窗黑屏；静态文本窗单窗掉到约 4fps。
> 目标：同机运行任意数量程序，每个窗口都正常渲染；后端可配置（含 OpenGL 路线）；对齐成熟框架（Chrome 单 GPU 进程思想、Flutter 降级不崩、Skia 空帧不碰交换链）。
>
> **新会话开工法**：指着本文档说“开 1.1”（或任一阶段号）即可开工，不用读别的：
>
> 1. 只读 §0 根因表 + 你要做的那一阶段整节，不要通读全文。
> 2. 先跑基线（本阶段“验收”里的第一条命令），确认当前是红（复现）还是绿。
> 3. 红灯起步：先写失败断言（单测或真窗指标），确认它红，再改代码。
> 4. 改完跑本阶段“验收”全部条目，全绿才算完。
> 5. 收尾走“合入生产线的硬门槛”逐项打勾；任一熔断命中就停下报告，不自决跳过。
>
> 环境（ judging 口径一致）：`export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so LD_LIBRARY_PATH=$PWD/lib DISPLAY=:0 GPUI_SURFACE_SAMPLE_COUNT=1`，Go 用仓库默认工具链，`GOWORK=off`。窗口一律 1200×800（accept/R6/m5 均已是）。

## 0. 根因实测（立项依据，下面的每一条都是真机量出来的）

| # | 事实 | 口径 |
|---|---|---|
| R1 | 裸设备（建实例+卡+设备，零纹理零窗口）占 194MB | `nvidia-smi`，单进程 |
| R2 | 配好交换链、零帧，占 193MB | 同上，surface 已配置 |
| R3 | 新建一个 1×1 纹理（1 字节），从 193MB 跳到 321MB | 同上，+128MB 堆块预留 |
| R4 | 空白真窗稳态 321MB，其中可追踪的纹理+缓冲只有 0.6MB | 自研临时统计 + `nvidia-smi` |
| R5 | 双窗同开（R6 321MB + accept 193~321MB）+ 桌面约 330MB，顶爆 1GB 卡，第二窗 depth 建不出 → 黑屏 | 复现：第二窗快照全黑 960000px，日志 `CreateTexture OOM` |
| R6 | 静态窗 acquire 250ms 超时 + 每次约 1s 重建，约 4fps；R6 同机 58fps | `WR_RESIZE_DBG` 日志 |
| R7 | 闲帧跳过 acquire 后，accept 回到 57.6fps/p95 17.6/1 hitch | 临时补丁验证 |
| R8 | `GPUI_BACKEND=gl` 能枚举到 GL 卡，但接 X11 窗口报不兼容 | 实测报错原文 |
| R9 | 核显（HD 520）单窗 R6/accept 均为 58fps；双窗都出画面；软渲染空白窗约 84fps | 真机三跑 |

结论：单窗固定成本约 322MB（194 设备 + 128 首分配堆块），与内容无关；Vulkan 多进程在 1GB 卡上物理上限约 2 窗；静态窗卡死是“拿图不交还”把交换链耗尽。

---

## 第一块：代码层高可用（不换后端，多窗先跑起来）

### 阶段 1.1 闲帧不碰交换链

- **目标**：静态窗不再拿图，单窗帧率回到门禁线以上。
- **前置**：无。本阶段是后面所有阶段的地基（不修它，任何窗的指标都量不准）。
- **背景与根因**：R6/R7。`present()` 先拿图后发现空帧再丢，驱动不回收未送显的图，交换链耗尽 → acquire 250ms 超时 + 约 1s 重建的死亡循环。
- **改动清单（精确）**：
  - `render/present_target.go` 的 `present()`：在 `FrameDamageUnion()` 取到空 damage 之后、`sc.BeginFrame()` 之前加早退——`!forceFull && damage==0 && postResizeFull<=0 && !inResizeStormLocked()` 时直接返回 `PresentOutcome{Mode: PresentModeIdle, Idle: true}` 并记 `lastOutcome`，不碰交换链。
  - 其余逻辑（强制整帧、resize 风暴、postResize 欠帧、错误路的 Discard）一行不动。
- **行为契约**：有 damage 的帧与改前逐字节同路；空帧不产生 acquire/present/discard 计数；动画窗（R6，常年有 damage）行为零变化。
- **验收（全部可执行）**：
  1. 复现基线：`GPUI_ACCEPT_RUN_SECONDS=15 go run ./examples/ui_text_edit_accept`（改前约 3~4fps，`hitch_count` 约 15）——确认红。
  2. 改后同命令：`fps_interval ≥ 55`、`interval_p95_ms ≤ 22`、`hitch_rate_per_min ≤ 5`、`cpu_fallback_ops == 0`。
  3. `RUN_SECONDS=15 go run ./examples/ui_wr_r6_layer_anim` 保持绿（`fps_interval ≥ 55`；Golden 与改前一致，允许既有 4.89% 基线漂移，不新增）。
  4. `RUN_SECONDS=15 go run ./examples/ui_text_m5_anycase` 同 accept 门禁。
  5. 快照目检：accept 六区出字正常（`GPUI_ACCEPT_SNAP=/tmp/x.png`）。
- **熔断**：任一窗帧率回退或画面不一致 → 回退本阶段改动，先查 damage 口径（`FrameDamageUnion` 是否漏了 HUD/overlay 脏）。
- **回滚**：单个函数早退块，revert 即回。
- **交接（给 1.2/1.3）**：此后帧率问题先看 acquire 相关日志（`WR_RESIZE_DBG=1`），不再怀疑 damage；`PresentOutcome.Idle` 语义不变。

### 阶段 1.2 建窗走适配器策略（混搭默认核显）

- **目标**：混搭机器默认用核显（内存与系统共用），独显不再被两个窗顶爆；`GPUI_POWER=high` 可要回独显；单显卡机器行为零变化。
- **前置**：1.1 已绿（否则帧率基线不可比）。
- **改动清单（精确）**：
  - 新建 `render/adapter_policy.go`（包 `render`）：把既有 `render/gpu/adapter_policy.go` 整文件搬入，函数名与行为一字不动（`AdapterPolicy/PolicyDefault/PolicyHigh/PolicyLow/ResolveAdapterPolicy/RequestAdapterWithPolicy/DeviceDescriptorForAdapter`）；`DeviceDescriptorLowVRAM/DeviceDescriptor` 若在 `render/gpu/device.go`，一并搬入（查引用：仅本策略使用）；删掉 `render/gpu` 下的原文件；`render/gpu/adapter_policy_test.go` 随之搬入改包名。
  - `render/present_target.go` 的 `NewPresentTarget`：选卡一段（现写死 `PowerPreferenceHighPerformance`）改为 `RequestAdapterWithPolicy(inst, surf, ResolveAdapterPolicy())`；建设备改用 `DeviceDescriptorForAdapter("ui-l1-present", adapter)`；`forceFallback==true` 时如实记日志（哪级、为什么）。
  - `ui/embedder` 两处调用方（`app.go`/`pipeline_app.go` 的 `OpenPresentTarget`）零改动。
- **行为契约**：单显卡机器选出的卡与改前同一张；`GPUI_POWER` 各取值行为与策略文件注释一致；`forceFallback` 只上报不改画。
- **验收（全部可执行）**：
  1. 混搭机默认：`RUN_SECONDS=10 go run ./examples/ui_l1_blank`，`nvidia-smi` 看不到该进程，且窗口正常出色（清屏色）。
  2. `GPUI_POWER=high RUN_SECONDS=10 go run ./examples/ui_l1_blank`，进程落在独显。
  3. 默认分别跑 R6 与 accept 15s：门禁同 1.1（`fps_interval ≥ 55` 等）。
  4. 默认双窗同开（R6 40s + accept 15s 带 SNAP）：两个快照都出画面（用像素数判定非全黑/非全一色），R6 不退回 4fps 档。
  5. 核显 Golden 存 `examples/<窗>/golden/baseline_igpu.png`，与独显基线的差异经评审确认为光栅级（参考值：R6 约 9.9%，无移位无缺字）。
- **熔断**：核显出现渲染错误（缺字、错位、色块错）→ 默认切回独显，本阶段只保留变量可配（`GPUI_POWER`），结论回写本节。
- **回滚**：选卡两处 revert + 搬包还原，默认行为回到独显。
- **交接（给 1.3）**：留下“当前实际后端”上报口（JSON 字段名、取值独显/核显/软渲染/GL），1.3 的降级链复用它；`RENDER_API_CATALOG.md` 的搬包路径由本阶段同步。

### 阶段 1.3 显存压力降级链 + 永不黑屏崩溃

- **目标**：显存真不够时自动降级，还能开；降无可降时一句人话报错退出，永远不出现黑窗、不崩溃。
- **前置**：1.1、1.2 已绿（降级链复用 1.2 的选卡与后端上报口）。
- **改动清单（精确）**：
  - `render/present_target.go` 的 `NewPresentTarget`：建卡/建设备/建交换链任一步报 OOM（含 `not enough memory`/`out of memory` 大小写全匹配）时，按“独显→核显→软渲染”顺序换卡重试（复用 `RequestAdapterWithPolicy` 的 fallback 语义）；每级重试前释放本级已建资源（instance/surface/adapter/device 按 `Close()` 逆序）；`requestPresentDeviceWithRetry` 的重试保留。
  - OOM 可淘汰缓存：建纹理 OOM 处（`render/internal/gpu/gpu_textures.go` 的 `createTextureRetryOOM` 已有 flush+重试）在仍失败时，调一轮 purge（字形图集页、图片纹理缓存、层池的可淘汰部分，需各包提供 `PurgeEvictable()` 接口，缺口新加、只做加法）再试一次。
  - 黑屏根除：任一必需纹理（depth/resolve）两次均失败 → 返回干净错误（写明哪一级、哪个尺寸、建议动作如关窗或换卡），调用方直接退出，不送黑帧；修此前 OOM 后继续画图导致的 SIGSEGV（失败后置失效位，后续调用直接返回错）。
  - JSON 增加 `gpu_backend`（取值 `discrete/integrated/software/gl`）与 `gpu_fallbacks`（降级次数）。
- **行为契约**：资源充足时与改前逐字节同路（重试只在 OOM 时触发）； purge 只清可重建缓存，不动当帧必需资源。
- **验收（全部可执行）**：
  1. 正常单窗：回归 1.1（accept 15s）+ 1.2（默认落卡）验收，零变化。
  2. 压力：先开 R6 占住独显（`RUN_SECONDS=60` 后台），再开 accept——期望自动降级成功（`gpu_backend` 非 discrete、画面快照非黑、退出码 0）或带原因报错退出；两种都不允许：黑屏快照、无日志刷屏（OOM 日志每秒不超过 3 行）、崩溃（退出码须为 0 或 1 的干净错误，禁 SIGSEGV）。
  3. 纹理级 OOM 注入：单测构造小堆设备（如有）或以缺卡 CI 跳过并注明（禁静默假绿）。
- **熔断**：降级后画面错 → 停用该级降级，只保留报错退出，结论回写本节。
- **回滚**：重试与报错改动 revert（`gpu_backend` 字段保留，值为 discrete 不影响旧判读）。
- **交接（给出块）**：降级链即高可用的最后一道；此后新后端（第二块 GL）即插即用——只需实现“选卡+建窗”两步并上报 `gpu_backend=gl`。

---

## 第二块：OpenGL 后端扶正（独立立项，与第一块并行排期）

> 前提结论（已实测）：开关（`GPUI_BACKEND=gl`）与 GL 卡存在，X11 的 EGL 地基通，但 wgpu 的 GLES 后端不认我们建的 X11 窗口；compute 管线在 GL 下建得出来（已实测 trivial compute 建链成功），真跑逐个验；另有已知 EGL abort 坑。故这是移植项目，不是配置项。
>
> 引擎对后端的硬需求清单（先对着它做 2.0 门检，缺一即停）：
>
> | # | 需求 | 来源 | GL 现状 |
> |---|---|---|---|
> | F1 | 每阶段 ≥9 storage buffer（Vello coarse） | `render/gpu/device.go:7` | 待 2.0 实测（看卡规格，低于 9 即熔断） |
> | F2 | compute 着色器可建链可调度（flatten/fine/clear/prepare） | `render/internal/gpu` 四处建链点 | 建链已通（trivial 实测），调度逐管线验 |
> | F3 | write-only storage texture（1 处） | `render/internal/gpu/pipeline.go:185` | 待 2.2 实测 |
> | F4 | Depth24PlusStencil8 + MSAA | session 纹理 | GLES 基础能力，预过 |
> | F5 | BGRA8/RGBA8/R8 三格式 | 全引擎用量统计（94/65/22 处） | GLES 基础能力，预过 |
> | F6 | 时间戳查询 | 仅绑定层有，引擎运行时零使用 | 非问题，不验 |
> | F7 | EGL 下 Fifo/Mailbox present |  steadily vsync 语义 | 待 2.1 实测 |

### 阶段 2.0 能力门检（新增，不写功能代码）

- **目标**：拿着上表 F1–F7 逐项出“过/不过”，任一硬需求不过则第二块整体暂停，不进入 2.1。
- **前置**：无（可与第一块并行）。
- **改动清单（精确）**：零功能改动。允许加临时探针（`examples/tmp_*/`，用完即删，禁合入）：
  - 读 GL 卡规格：`GPUI_BACKEND=gl go run ./gpu/rwgpu/examples/adapter_info`，核对 F1（≥9）与 compute 相关规格。
  - trivial compute 建链+调度冒烟（建、调度、读回各一步），核对 F2。
  - EGL 初始化与 present mode 枚举，核对 F7。
- **验收**：上表 F1–F7 每行填“过/不过 + 实测数”，写入本节；F1/F2/F3 任一不过 → 熔断（第二块暂停，结论回写）。
- **熔断**：见上。门检本身不改代码，无需回滚。
- **交接（给 2.1）**：过的清单即 2.1/2.2 的验证基线；不过的项即不支持清单的初稿。

### 阶段 2.1 EGL 窗面打通

- **目标**：`GPUI_BACKEND=gl` 的窗口能弹出来、配好交换链、present 空帧。
- **前置**：2.0 全过（F1–F7 门检绿灯）。
- **背景与根因**：R8。GL 卡可枚举但不认 X11 窗（`gl not compatible with provided surface`）；X11 的 EGL 地基通（NVIDIA EGL 1.5）。
- **架构事实（已按源码核定）**：后端在 Go 侧只有三处——建实例位掩码（`gpu/rwgpu/instance.go`，`GPUI_BACKEND` 进位）、选卡过滤（本策略）、上报名字（`convertBackendType`）；建设备之后引擎零后端分支（`render/`/`ui/` 无任何 `if GL`），WGSL 翻译与送显全在 wgpu 库内。注：仓内另有自研 `gpu/shader/glsl` 翻译包，但未接入窗口路径（仅自带测试），线上仍走库内翻译。故本阶段只修“进门两件”，不碰引擎。
- **改动清单（精确）**：
  - `gpu/webgpu/surface_linux.go`：GL 后端走 EGL 路径建 surface（Xlib 窗配 EGLConfig 兼容 visual；Wayland 走既有 Wayland 路）。
  - 选卡：`RequestAdapterWithPolicy` 收 surface 兼容 + 特性需求两个条件（附 §2），GL 不满足直接下一级。
  - 默认后端保持 Vulkan 不动；GL 只走 `GPUI_BACKEND=gl`。
  - F7 验证：EGL 下 Fifo 一定要有（稳态 vsync 语义就靠它），Mailbox 有最好，没有如实记、不强求。
- **行为契约**：Vulkan 路径一行不动；GL 路径失败只影响 GL 窗。
- **验收（全部可执行）**：
  1. `GPUI_BACKEND=gl RUN_SECONDS=15 go run ./examples/ui_l1_blank`：present ≥ 100 帧（读 stderr 汇总），无崩溃，退出码 0。
  2. `nvidia-smi` 记录 GL 空白窗占用，写入 §0 表续行（与 Vulkan 321MB 并列）。
  3. 同条件在 Intel 机 / 单显卡机各跑一次（有条件才跑，缺环境 `t.Skip` 注明）。
- **熔断**：EGL 初始化在目标机器环境普遍失败 → 本路线暂停，结论回写本节。
- **回滚**：surface 改动 revert，Vulkan 路径零触碰。
- **交接（给 2.2）**：留下 GL 窗的最小可跑基线与占用数；着色器逐个验的清单从 2.2 起。

### 阶段 2.2 着色器与管线过筛

- **目标**：全量管线在 GLES 下建得出来、跑得对；compute 缺口逐个有替代或明确不支持清单。
- **前置**：2.1 已绿（GL 窗可开可刷）。
- **背景与根因**：R8 + 2.0 门检结论 + 已知坑（purego 下 EGL abort、并发选卡不安全，见 `gpu/rwgpu/thread_safety_test.go` 注释）。风险点收敛为：F3 storage 纹理写、复杂 compute 调度的真实行为（建链已通不等于跑得对）。
- **改动清单（精确）**：逐管线在 GL 真机建链+跑帧+读回（清单：`render/internal/gpu` 下所有 `CreateRenderPipeline/CreateComputePipeline` 调用点，先列后验）；缺口改写法（禁止降画质通过）；改动一律进引擎层（`render/`/`gpu/`），示例层只做验证；EGL 调用串行化（沿用测试注释的结论，加锁不加并发）。F4/F5 格式已预过，只做回归对照不重验；F6 时间戳跳过（引擎零使用）。
- **行为契约**：Vulkan 下所有管线行为零变化（改写法必须双后端跑对照）。
- **验收（全部可执行）**：
  1. `GPUI_BACKEND=gl` 分别跑 R6/m5/accept（时长同 1.1）：画面与 Vulkan 一致（像素断言过；Golden 存独立 `baseline_gl.png`）。
  2. 不支持清单写入本文档（哪个特性、影响哪个窗、替代方案或降级行为）。
  3. `go test ./gpu/...` 全绿（含既有 GLES 串行测试）。
- **熔断**：核心管线（文本/遮罩/合成任一）在 GLES 无合理替代 → 本路线暂停，结论回写本节。
- **回滚**：按管线 revert，不影响 Vulkan。
- **交接（给 2.3）**：GL 全绿的窗矩阵 + 不支持清单；2.3 只量占用与多窗。

### 阶段 2.3 多窗占用达标

- **目标**：GL 单窗占用显著低于 Vulkan（不足一半，以 2.1 基线为准），同机多窗数量翻倍以上且全绿。
- **前置**：2.2 已绿（GL 画面已对，只比占用与数量）。
- **改动清单（精确）**：本阶段原则上不写功能代码，只做测量与调优（图集页上限、层池上限等旋钮若需动，走小步+单测+回归）；`GPUI_BACKEND` 矩阵文档化（Linux/Vulkan/GL 何时默认，见附 §1）。
- **行为契约**：Vulkan 默认不动；GL 只走变量，直到本阶段验收全绿才谈默认。
- **验收（全部可执行）**：
  1. GL 下 R6 + accept + m5 三窗同开（R6 40s 后台 + 另两窗 15s 带 SNAP）：个个出画面（非黑判读），`fps_interval ≥ 55`（弱卡如实记录，`gpu_backend=gl` 可查），零黑屏零崩溃。
  2. 每窗占用数（`nvidia-smi` 单进程）写入 §0 表格续行；与 Vulkan 同窗数并列对比。
  3. 缺环境（无 GL/无多卡）用 `t.Skip` 注明，禁静默假绿。
- **熔断**：占用未达标 → 只保留 GL 可配，不宣传多窗收益，结论回写本节。
- **回滚**：默认后端仍是 Vulkan，GL 走变量；旋钮改动 revert。
- **交接（给出块）**：GL 默认 gate（附 §1 的 Linux 默认行）由用户单独确认后才翻；翻之前本文档保持“GL 可配、Vulkan 默认”。

---

## 合入生产线的硬门槛（两块通用）

1. 调研期的临时代码全删（统计钩子、孤儿回收、探针目录），合入 diff 里不得残留。
2. `go run ./scripts/apidoc` 绿；`docs/RENDER_API_CATALOG.md` 同步（搬包的函数改路径）；本文档的修订记录逐阶段追加。
3. 每阶段回归：改动单测 + `go test ./ui/...` + 验收窗真机（R6/accept/m5，按阶段要求）。
4. 禁止示例层绕引擎洞；禁止降画质装绿；`ui → render → gpu` 单向依赖；禁 CGO。
5. 每个真窗的 JSON 必须如实上报实际后端（独显/核显/软渲染/GL），跨卡 Golden 各存各的基线，不得混用。
6. 任一阶段熔断 → 停下报告用户定方向，不自决跳过。

## 修订

| 版本 | 说明 |
|---|---|
| 立项 | 2026-09-05 · 双窗黑屏 + 静态窗 4fps 根因实测建档；分两块六阶段； OpenGL 独立为第二块。 |
| 新会话可开工 | 2026-09-05 · 每阶段补齐前置/精确改动清单/行为契约/可执行验收/交接（对标 §9.2 九项规范）；新会话指文档说阶段号即可开工。 |
| 第二块补门检 | 2026-09-05 · 新增 2.0 能力门检（F1–F7 硬需求表）；compute 建链实测通过；时间戳/格式预过不再重验；storage-9 与 EGL present 为待测门项。 |
| 后端矩阵 | 2026-09-05 · 明确各 OS 可用后端与默认（见 §1）；macOS 无 GL（苹果已废弃，wgpu 不提供）；Linux 默认 GL 必须等 2.3 变绿；选型按能力驱动（见 §2），为 2.5D/3D 预留。 |
| 1.1 关闭 | 2026-09-05 · 闲帧不碰交换链：`render/present_target.go` 的 `present()` 在空 damage 时早退 `PresentModeIdle`，不做 acquire/present/discard；accept 真机 3.75fps→57.8fps（p95 17.6/hitch 4.0/cpu_fallback 0），R6 58.0fps 且 Golden 漂移与改前同为 4.8887%（零新增），m5 同 accept 门禁，快照六区出字正常。 |
| 1.2 关闭 | 2026-09-05 · 建窗走适配器策略：`render/gpu/adapter_policy.go` + `device.go` 整文件搬入 `render/adapter_policy.go`（函数名与行为一字不动；新文件无构建标签，保持 `go build -tags nogpu ./render/` 绿），`NewPresentTarget` 改走 `RequestAdapterWithPolicy(inst, surf, ResolveAdapterPolicy())` + `DeviceDescriptorForAdapter`，forceFallback 时 stderr 如实记日志，新增 `PresentTarget.GPUBackend()`（discrete/integrated/software）供 1.3 复用；混搭机默认核显（blank 窗 `nvidia-smi` 不见进程，~60fps，DBG=Intel IntegratedGPU），`GPUI_POWER=high` 回独显（`nvidia-smi` 可见，DBG=NVIDIA DiscreteGPU）；默认 R6 58.3fps/accept 57.5fps/m5 58.4fps 全绿，双窗同开（R6 40s + accept 15s）双快照出画面（全黑 0%），三窗核显基线 `baseline_igpu.png` 已存（R6 与独显基线差 9.93%≈参考值 9.9%，目检无移位无缺字；accept 差 1.52%），熔断未触发。 |

## 附 §1 各 OS 后端矩阵（以本仓 pin 的 wgpu 为准）

> 库内定义（`gpu/types/adapter.go`）：主后端 = Vulkan / Metal / D3D12；备后端 = GL（仅）；实例默认只要主后端。

| OS | 可用后端 | 默认（本框架） | 说明 |
|---|---|---|---|
| Linux | Vulkan（主）、GLES/EGL（备） | Vulkan + 混搭默认核显；GL 要等第二块 2.3 变绿才可当默认 | 现状 GL 接不上 X11 窗（已实测），先核显顶多窗 |
| Windows | D3D12（主）、Vulkan（看驱动）、GL 经 ANGLE（备） | D3D12 | 与 Chrome/Flutter 一致；GL 只做备选 |
| macOS | Metal（唯一） | Metal | 苹果 10.14 起废弃 OpenGL，wgpu 不提供 GL，不可配 GL |
| 软兜底（全平台） | lavapipe / 系统软渲染 | 降级链末级 | 已实测空白窗约 84fps；只保能开，不保性能 |

## 附 §2 选型按能力驱动（给 2.5D/3D 预留扩展性）

后端选择 = 环境变量 > 窗口能力需求 > OS 默认 > 降级链，禁止按 OS 写死：

- 窗口声明需要的特性（如 compute、storage 纹理、时间戳），策略只从满足特性的后端里选。2D 小窗要轻量，3D 窗要功能，各取所需。
- 2.1/2.2 落地时，选卡函数必须同时收 surface 兼容 + 特性需求两个条件，不满足直接走下一级，不黑屏。
- 将来 2.5D/3D 只需在窗口侧声明特性，策略层不用改。
