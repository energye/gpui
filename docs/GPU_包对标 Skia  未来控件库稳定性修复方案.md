# GPU 包对标 Skia / 未来控件库稳定性修复方案

## 目标

本文基于当前 `gpu` 包代码审查，整理面向 Skia 级渲染库、未来 Ant Design 级控件库的 GPU 层修复方案。

目标不是补某一个示例的崩溃，而是把 `gpu/types -> gpu/rwgpu -> gpu/webgpu -> gpu/context` 建成可长期运行、可恢复、可诊断、可被控件树稳定依赖的基础设施。

本文是待修复方案和验收口径，不表示其中列出的 helper、测试名、presenter 类型已经存在。每个修复项落地后，需要把对应测试、命令和状态更新为真实证据。

## 对标标准

| 维度 | Skia / UI 框架要求 | 当前主要缺口 |
| --- | --- | --- |
| 资源生命周期 | context 可失效，资源 release/destroy 幂等，不因 device lost 崩溃 | `Release/Unconfigure` 多处直接 native 调用 |
| Device lost | 设备丢失是状态，不是进程崩溃 | 只依赖 DeviceLostCallback，uncaptured "Parent device is lost" 可能漏标 |
| Surface 状态机 | minimize/unmap/occluded/resize 由统一 presenter 管理 | 目前部分宿主/示例各自处理，部分示例缺失处理，策略分散 |
| 错误契约 | public API 返回结构化错误或 no-op，不能随机 panic | `webgpu` facade nil/released 检查不一致 |
| 控件资源缓存 | 跨帧缓存可识别旧 device generation，失效后可重建 | `gpu/context` opaque handle 只有 unsafe pointer，无 generation/liveness |
| 能力矩阵 | 功能、生命周期、性能、退化路径均可回归验证 | capability matrix 偏绘制能力，生命周期覆盖不足 |

## 当前已发现的问题

### P0. Device-lost 状态不闭合

当前 `rwgpu` 有 `ClassifyError`、`gateDevice`、`refuseIfLost` 等基础设施，但 device lost 状态主要由 `DeviceLostCallback` 写入。`uncapturedErrorHandler` 只记录错误，不会把 `"Parent device is lost"` 等 validation message 标记为 lost。

风险：

- native 已经进入 lost 状态，但 Go 侧仍认为 device 可用；
- 后续 `GetCurrentTexture/Configure/Unconfigure/Release` 可能触发 wgpu-native panic / SIGABRT；
- 上层控件树无法得到稳定的 `ErrDeviceLost` 契约。

修复方向：

- 对 uncaptured error message 做保守识别：包含 `"device lost"`、`"parent device is lost"`、`"device_removed"`、`"vk_error_device_lost"` 等时标记 lost；
- 保留当前 per-device lost 设计；是否增加进程级 fallback fuse 作为待决策项，仅用于 callback device pointer 无法可靠解析或 native 只返回 message 的兜底场景；
- 所有 native boundary 前统一检查 lost 状态。

### P0. Release / Destroy / Unconfigure 缺少 lost-safe 策略

当前多类资源释放仍直接调用 native release，例如 device、queue、surface、texture、buffer、pipeline、encoder 等。device lost 后 native release 本身也可能成为崩溃点。

风险：

- 程序主循环已安全退出，但 defer 清理阶段崩溃；
- 最小化、切后台、长时 soak 后出现无业务日志的 abort；
- 资源清理路径无法作为 UI 框架的可靠兜底。

修复方向：

- 在 `rwgpu` 增加统一 helper：
  - `releaseNativeHandle`
  - `destroyAndReleaseNativeHandle`
  - lost 后只清 Go handle / debug tracking，不调用危险 native release；
- 所有 `Release/Destroy/Unconfigure` 接入统一 helper；
- 释放函数全部保证 nil-safe、zero-handle-safe、idempotent；
- 对 release path 增加单元测试，使用假 handle + lost fuse 验证不会触发 native 调用。

### P0. public API 防御顺序不一致

典型问题：`Surface.Unconfigure()` 先 `mustInit()`，再判断 `s == nil` 或 `handle == 0`。这类顺序对基础库不合格。

风险：

- nil / released 对象也可能触发 native 初始化或 panic；
- 同类 API 行为不一致，控件库很难建立可靠异常处理；
- 测试覆盖不到的边界路径会成为偶现崩溃。

修复方向：

- 制定统一顺序：`nil -> released/zero handle -> lost -> checkInit -> native call`；
- `rwgpu` 和 `webgpu` 所有 public 方法按该顺序审计；
- 新增 null/released/lost gate 测试。

## 代码复查对齐记录

本节记录文档与当前代码的对齐状态，避免把“计划项”误读成“已实现项”。

| 项目 | 当前代码状态 | 文档处理 |
| --- | --- | --- |
| device lost 标记 | 当前是 per-device lost：`liveDevices` 路由 callback 到对应 `Device.lost`；`uncapturedErrorHandler` 不标记 lost | 文档改为：process-wide fuse 是待决策 fallback，不是当前实现 |
| release lost-safe helper | 当前没有 `releaseNativeHandle` / `destroyAndReleaseNativeHandle` | 文档保留为拟新增 helper 名称 |
| release path 覆盖 | 多数 `Release/Destroy/Unconfigure` 仍直接调用 native release/unconfigure | 文档将其列为 P0 待修复缺口 |
| `Surface.Unconfigure` 防御顺序 | 当前先 `mustInit()` 再判 nil/handle | 文档将其列为 P0 防御顺序问题 |
| `webgpu` facade gate | 当前部分方法检查 nil，部分方法直接访问 receiver 字段 | 文档将其列为 P0 facade gate 问题 |
| presenter/window 层 | 当前没有统一 `Presenter` / `WindowSurfaceState` / `SwapchainController` 类型 | 文档保留为 Phase 1 拟抽象 |
| `capability_matrix` 窗口状态 | 当前未发现统一 minimized/unmap/visibility 处理关键字 | 文档改为“部分示例处理、部分缺失” |
| `DeviceGeneration` / `ResourceEpoch` | 当前没有该类型 | 文档保留为 Phase 2 方案 |
| `TestDeviceLostReleasePathsSkipNative` | 当前没有该测试 | 文档中的该名称只作为修复后建议新增的证据示例 |

### P0. `webgpu` facade 缺少统一 gate

`webgpu.Device.CreateTexture()` 检查 nil device，但 `CreateBuffer()` 直接访问 `d.released`。类似差异在 facade 层多处存在。

风险：

- 上层拿到同一类错误时，有时是 error，有时是 panic；
- 控件库跨模块调用 GPU API 时很难判断是否可恢复；
- 未来自动 recovery / device recreate 难以统一接入。

修复方向：

- 在 `webgpu` 增加统一 gate：
  - `prepareDeviceCall`
  - `prepareQueueCall`
  - `prepareSurfaceCall`
  - `prepareResourceCall`
- facade 层所有创建、提交、present、release API 使用统一 gate；
- 文档明确：library API 默认返回 error/no-op，除非明确标记为 programmer panic。

### P1. Surface / Swapchain / Window visibility 策略分散

`webgpu.Swapchain` 已经有 frame pairing、reconfigure throttling、stats、auto recover hook，是正确方向。但窗口可见性策略仍留给宿主或示例处理。

风险：

- `mem_anim_window`、`capability_matrix`、未来窗口示例重复实现 X11/WM_STATE/VisibilityNotify；
- 控件库需要统一处理 minimize、unmap、occluded、DPI、resize、damage present；
- 每个 app 写自己的 pause/reconfigure 策略，容易出现同类崩溃。

修复方向：

- 新增统一 presenter/window-surface 层；
- host 只上报窗口状态：
  - mapped/unmapped
  - minimized/iconic
  - occluded
  - focused/unfocused
  - size/DPI
- presenter 负责：
  - minimized/unmapped 时 suspend swapchain；
  - mapped 后 reconfigure；
  - occluded/timeout 时 skip frame；
  - resize debounce；
  - device lost 后统一 recover 或上报 fatal state。

### P1. `gpu/context` opaque handle 缺少 generation / liveness

当前 `TextureView`、`CommandEncoder` 等 handle 是轻量 unsafe pointer。优点是零分配、解耦；缺点是不能表达资源是否属于旧 device。

风险：

- 控件树缓存 texture/view/layer/glyph atlas 后，device recreate 会留下旧 handle；
- 旧 handle 被新帧误用时，错误可能在很深的 GPU submit/present 阶段才出现；
- 无法做控件级资源自动失效和重建。

修复方向：

- 引入 `DeviceGeneration` 或 `ResourceEpoch`；
- handle 中记录 owner generation，或在资源 registry 中可查询；
- 控件层缓存资源时绑定 generation；
- device recreate 后统一 invalidate cache。

### P1. 资源 registry / cache purge 模型不足

对标 Skia，GPU context 应支持资源 purge、abandon、cache pressure、budget 等概念。当前 `debug` tracking 更偏调试，不是生产级资源治理。

修复方向：

- 区分 debug tracking 与 runtime resource registry；
- 维护资源 owner、type、bytes、generation、last-used frame；
- 支持：
  - device lost abandon；
  - memory pressure purge；
  - frame end transient cleanup；
  - glyph/image/layer atlas cache reset。

### P2. Shader pipeline 需要继续以 capability matrix 驱动

`gpu/shader/ir` 的方向正确：独立 IR、validate、多目标输出。但对标 Skia/Graphite，shader pipeline 必须靠能力矩阵持续证明。

修复方向：

- 扩展 shader feature matrix：
  - storage/uniform layout；
  - bind group compatibility；
  - texture sampling；
  - blend/filter/layer shader；
  - backend-specific limitations；
- shader compile error 统一结构化；
- pipeline cache key 标准化，避免控件高频创建 pipeline。

## 分阶段修复计划

### Phase 0：稳定性红线

目标：所有 GPU public API 不允许因常见生命周期异常导致进程崩溃。

任务：

- 补全 device-lost sticky fuse；
- release/destroy/unconfigure 全面 lost-safe；
- 所有 public 方法 nil/released/lost 检查顺序统一；
- facade gate 统一；
- 补单元测试覆盖 lost 后 release path、nil surface、nil device、released resource。

验收：

- `go test ./gpu/rwgpu -run 'DeviceLost|Null|Release|Surface'` 通过；
- 最小化/切后台/到时退出不出现 SIGABRT；
- device lost 后 API 返回 `ErrDeviceLost` 或 no-op，不 native abort。

### Phase 1：统一 Presenter

目标：窗口状态处理不再分散在示例中。

任务：

- 抽 `WindowSurfaceState`；
- 抽 `Presenter` 或 `SwapchainController`；
- X11/Wayland/Win32/Cocoa 状态由平台层适配；
- 示例只调用统一 presenter tick。

验收：

- `mem_anim_window`、`capability_matrix` 不再各自实现完整 minimize/unmap 策略；
- minimize/unmap 期间 presents 不增长，CPU 降低；
- restore 后自动 reconfigure 并继续渲染。

### Phase 2：控件资源生命周期

目标：控件树可安全缓存 GPU 资源，并在 device recreate 后自动失效。

任务：

- 引入 device/resource generation；
- handle 或 registry 支持 liveness 查询；
- render/cache 层接入 generation；
- glyph atlas、image atlas、layer cache 支持统一 reset。

验收：

- device recreate 后旧 cache 不被误用；
- 控件页面切换、主题切换、DPI 切换后无野 handle；
- 长时运行无单向资源增长。

### Phase 3：能力矩阵和回归体系

目标：把“能画”升级为“生命周期、性能、退化路径都可证明”。

任务：

- 扩展 `capability_matrix`：
  - minimize/restore；
  - resize debounce；
  - device lost injection；
  - OOM / resource pressure；
  - cache purge；
  - partial present；
- 建立稳定性门禁：
  - panic/SIGABRT 为 hard fail；
  - RSS/VRAM steady delta 阈值；
  - present/acquire/reconfigure 计数阈值；
  - lost/recover 事件日志。

验收：

- 代表场景 5/10/30 分钟 soak；
- 自动生成 summary；
- 所有 regression 有可追溯日志。

## 建议的代码边界

| 层 | 应负责 | 不应负责 |
| --- | --- | --- |
| `gpu/types` | WebGPU 类型、能力枚举、零依赖描述符 | native handle、窗口事件 |
| `gpu/rwgpu` | FFI、ABI、native handle、device lost gate、lost-safe release | 业务窗口策略、控件逻辑 |
| `gpu/webgpu` | 安全 facade、swapchain、frame pairing、recover hook | 具体 X11/Win32 事件解析 |
| `gpu/context` | 跨包窄接口、opaque handle、device provider | native backend 细节 |
| presenter/window 层 | minimize/unmap/resize/DPI/occluded 策略 | 底层 ABI / purego 调用 |
| render/control 层 | canvas、cache、控件绘制、主题、布局 | 直接操作 native surface |

## 优先级总表

| 优先级 | 项目 | 原因 |
| --- | --- | --- |
| P0 | device-lost sticky fuse | 防止 native abort |
| P0 | release/destroy lost-safe | 解决退出清理阶段崩溃 |
| P0 | public API gate 统一 | 给控件库稳定错误契约 |
| P0 | nil/released/lost 顺序审计 | 消除基础库随机 panic |
| P1 | presenter 抽象 | 消除示例重复窗口容错 |
| P1 | generation/liveness | 支撑控件缓存与 device recreate |
| P1 | resource registry | 支撑 purge、abandon、memory pressure |
| P2 | shader/capability matrix 扩展 | 证明功能完整性和跨 backend 一致性 |

## 最终验收标准

- 最小化、切后台、窗口遮挡、resize、关闭、到时退出均不触发 native abort；
- device lost 后所有 public API 可控返回，不 panic；
- release/destroy/unconfigure 幂等；
- 控件层不直接感知 native backend；
- device recreate 后旧资源不会被误用；
- capability matrix 覆盖功能、生命周期、性能和异常路径；
- 长时 soak 无持续 RSS/VRAM 单向增长；
- 文档、测试、日志能定位每一类失败。

## 单问题 / 单模块修复验证方法

每次只修一个问题或一个模块时，必须按“问题闭环”验证，而不是只跑全量测试。标准流程如下。

### 1. 定义问题边界

先写清楚本次修复对象：

- 问题 ID：例如 `GPU-P0-DEVICE-LOST-RELEASE`
- 影响模块：例如 `gpu/rwgpu`
- 触发条件：例如 device lost 后执行 `Surface.Unconfigure`
- 失败表现：例如 SIGABRT、panic、返回错误类型不正确、资源泄漏
- 期望行为：例如返回 `ErrDeviceLost`，或 release no-op 且 handle 清零

没有清晰问题边界，不允许进入修复。

### 2. 最小复现验证

修复前必须有最小复现方式。复现可以是：

- 单元测试复现；
- 小型测试程序复现；
- 示例程序固定命令复现；
- 人工窗口操作复现，但必须记录步骤和日志。

示例格式：

```text
复现步骤：
1. GPUI_SCENARIO=S12 GPUI_ANIM_SECONDS=120 go run ./examples/mem_anim_window
2. 启动后最小化窗口
3. 等待 60-180 秒

修复前结果：
- 进程 SIGABRT
- 日志最后停在 done/PASS 或 BeginFrame 附近

修复后期望：
- 不崩溃
- 退出 reason 可解释
- 若 device lost，返回 ErrDeviceLost 或 clean exit
```

### 3. 定向单元测试

每个底层修复必须有定向单测。单测只验证本问题，不依赖完整 GUI 环境。

| 修复类型 | 必须验证 |
| --- | --- |
| nil/released gate | nil receiver、zero handle、released resource 不 panic |
| device lost gate | lost 后不进入 native call，返回 `ErrDeviceLost` |
| release/destroy | 幂等、handle 清零、lost 后不 native release |
| surface acquire | occluded/outdated/timeout/device lost 分类正确 |
| swapchain frame pairing | BeginFrame/EndFrame/DiscardFrame 一一配对 |
| resource cache | device generation 变化后旧资源失效 |

示例命令：

```bash
env GOCACHE=/tmp/gpui-go-cache go test ./gpu/rwgpu -run 'TestDeviceLost|TestNull|TestRelease'
env GOCACHE=/tmp/gpui-go-cache go test ./gpu/webgpu -run 'TestSwapchain|TestSurface|TestFrame'
```

### 4. 模块级回归

修哪个模块，就跑该模块的回归；同时跑直接上游模块，避免 facade 破坏。

| 修改模块 | 必跑 |
| --- | --- |
| `gpu/types` | `go test ./gpu/types ./gpu/context ./gpu/webgpu` |
| `gpu/rwgpu` | `go test ./gpu/rwgpu` + `go test ./gpu/webgpu -run ...` |
| `gpu/webgpu` | `go test ./gpu/webgpu` + 相关示例 build |
| `gpu/context` | `go test ./gpu/context` + render/device provider 相关测试 |
| `gpu/shader` | `go test ./gpu/shader/...` + capability matrix 相关场景 |
| presenter/window 层 | 窗口示例最小化、还原、resize、关闭测试 |

如果环境无法跑完整 native 测试，必须记录原因，并至少跑不依赖 GUI/native 的定向测试和 build。

### 5. 示例级验证

底层修复不能只看单测。必须至少跑一个真实示例证明上层行为正常。

窗口 surface 相关修复：

```bash
env GOCACHE=/tmp/gpui-go-cache go build -o /tmp/mem_anim_window ./examples/mem_anim_window
GPUI_SCENARIO=S12 GPUI_ANIM_SECONDS=120 /tmp/mem_anim_window
```

能力矩阵相关修复：

```bash
env GOCACHE=/tmp/gpui-go-cache go build -o /tmp/capability_matrix ./examples/capability_matrix
GPUI_SCENARIO=C20 GPUI_ANIM_SECONDS=30 /tmp/capability_matrix
```

offscreen 资源释放相关修复：

```bash
env GOCACHE=/tmp/gpui-go-cache go build -o /tmp/mem_window_stress ./examples/mem_window_stress
GPUI_MEM_ITERS=500 /tmp/mem_window_stress
```

### 6. 扰动验证

涉及窗口、surface、swapchain、device lost 的修复，必须做扰动验证。

| 场景 | 验证点 |
| --- | --- |
| 最小化 1-3 分钟 | 不 acquire/present，不崩溃 |
| 最小化后还原 | 自动 reconfigure，继续渲染 |
| 切到其他窗口 | 不高 CPU，不 device lost abort |
| resize/maximize | 不 reconfigure thrash，不闪黑 |
| 到时退出 | defer cleanup 不崩溃 |
| 连续 20-50 次最小化/还原 | 状态机不乱，frame pairing 不破 |

### 7. 长时 soak 验证

P0 稳定性修复至少需要短 soak。进入发布前需要长 soak。

| 等级 | 时长 | 用途 |
| --- | --- | --- |
| smoke | 30-60 秒 | 快速确认不立即崩溃 |
| short soak | 2-5 分钟 | 验证延迟崩溃、后台状态 |
| long soak | 10-30 分钟 | 验证慢性泄漏、VRAM/RSS 稳定性 |
| release soak | 1 小时以上 | 发布前代表场景稳定性 |

必须记录：

- frames；
- presents；
- reconfigures；
- device lost 次数；
- RSS 起止和 steady delta；
- 退出原因；
- panic/SIGABRT/timeout 是否为 0。

### 8. 验收证据格式

每个修复完成后，提交或记录时必须包含：

```text
问题：
修复范围：
风险：
定向单测：
模块测试：
示例测试：
扰动/soak：
未覆盖项：
结论：
```

示例：

```text
问题：device lost 后 Surface.Release 触发 native abort
修复范围：gpu/rwgpu Surface/Device/Texture release path
风险：lost 后跳过 native release，进程内可能残留 native ref；只在 lost 状态生效
定向单测：新增 TestDeviceLostReleasePathsSkipNative；go test ./gpu/rwgpu -run TestDeviceLostReleasePathsSkipNative -count=1 PASS
模块测试：go test ./gpu/rwgpu -run 'DeviceLost|NullGuard|Surface' PASS
示例测试：mem_anim_window S12 120s minimized PASS（记录日志路径）
扰动/soak：20 次 minimize/restore PASS（记录日志路径）
未覆盖项：Wayland 未验证
结论：X11/wgpu-native 路径闭环，Wayland 待补证
```

### 9. 不同优先级的最低验证门槛

| 优先级 | 最低验证门槛 |
| --- | --- |
| P0 崩溃 / device lost / release path | 定向单测 + 模块测试 + 真实示例 + 扰动测试 |
| P1 presenter / cache / generation | 单测 + 两个窗口示例 + resize/minimize/restore |
| P2 shader / capability | shader 单测 + capability matrix 对应场景 + golden/capture |
| 文档 / README | 不需要运行测试，但要确认路径和命令准确 |

### 10. 失败处理规则

验证失败时不能扩大修复范围硬改。必须先分类：

- 复现不稳定：补日志和更小复现；
- 单测失败：先修单元层；
- 示例失败但单测过：检查 facade/presenter 契约；
- soak 失败：看最后日志、RSS、present/reconfigure/device lost 计数；
- 环境失败：记录 DISPLAY、WGPU_NATIVE_PATH、driver、XAUTHORITY、日志路径。

每次失败都要沉淀为新的定向测试或脚本检查，避免同类问题再次靠人工发现。
