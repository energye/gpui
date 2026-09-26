# GL 后端迁移计划（把 gogpu 的 GL 接进 gpui）

> 一句话：现在 gpui 只会走 Rust 编的 WebGPU 那套，一个窗口吃几百 M，1G 卡开几个窗就爆。gogpu 那边有现成的纯 Go 的 GL 路，一个窗口几十 M。把那条路搬过来，Linux 和 Windows 走纯 Go GL，macOS 走纯 Go Metal。
> 只走标准做法，非标准的不要。示例只做验证，不绕引擎。

## S. 统一口规则（以本节为准，旧口径作废）

> 当前方向（2026-09-26 定稿）：`hal` 只定接口不写实现，形状以现在 `render` 用的 `gpu/webgpu` 方法为准——`render` 调了 `webgpu` 的哪些方法，`hal` 就对应上哪些，方法名/签名/描述符/枚举一一对应；`hal` 多出来的删，不一样的按 `webgpu` 改。顺序：① `hal` 对齐 `webgpu`；② `webgpu` 实现新 `hal` 口；③ `render` 全换 `hal` 口先跑通；④ `gwgpu` 放最后（`render` 跑通后再做），对照新 `hal` 口改，对照 `webgpu` 功能补实现。2026-09-25 及之前的 Rust 为准、facade 口终态一律作废。

- 终态：`render` 只存 `hal.xxx` 接口（Device/Queue/Buffer/Texture/View/Sampler/Shader/BindGroup/BindLayout/PipeLayout/RenderPipe/ComputePipe/Encoder/CommandBuffer/RenderPass/ComputePass/Surface/SurfaceTexture/Adapter/Instance/Fence/QuerySet），创建时 `SelectBackend` 装实现（默认 Rust 不动，不读环境变量、不自动回退），之后全走接口无感知；结构里不许双句柄+标记分支，分支只出现在创建那一句话；内部查表用普通 map，不许类型猜测。
- 对照表先行：H1 先拉 `hal` 方法×`webgpu` 方法×`render` 调用点对照表，以源码为准，不凭记忆；改的过程中发现的问题随时回写本文。
- 别名纪律：禁冗余别名，能直接用就直接用；`render` 直写 `hal.Xxx`/`types.Xxx`，删别名与换口同批做，不留过渡。
- 口径规则：`render` 调啥，`hal` 就得有啥；只是创建/配置/观测的留门面最小形态（交换链壳、显存账本只收数字）；跨包不透明句柄走 `gpucontext` 桥，`hal` 不收；各包日志自管。
- 行为清单（以 `webgpu` 为准）：提交排队还是直发、映射阻塞还是先报错、设备丢失报什么错、错误作用域怎么收、围栏时间戳怎么认、轮询与等闲语义。`hal` 按 `webgpu` 改，`gwgpu` 向 `webgpu` 看齐，差异逐项有单测。
- 只差名直接改（老调用处一起改）；真差才在对应源码文件贴过渡，老方法保留到搬完再删，不单独立文件/包。
- 节奏：一次一对，逐文件搬；每批格式+vet+单测+两时刻像素；默认 Rust 路像素逐位一致，差一像素就停。
- 跨平台：Linux X11/Wayland、Windows、macOS 三端一次标齐；版本号加修订一起走。
- 工作区纪律：`gpu`/`render` 已恢复干净（2026-09-26，`video` 不动，改动备份在 `/tmp/opencode`），重做从干净底子开始；代码永远不提交，文档按需提交。

## 0. 为啥干

- 现状：gpui 经 `gpu/rwgpu`（Rust 库，purego 调）再包一层 `gpu/webgpu`，`render/present_target.go` 只会调这条。开关 `GPUI_BACKEND=gl` 只能让它去找 GL 卡，窗口那步报不兼容，画面送不上去。
- 量到的数：独显 940MX（1G）上鹈鹕单窗驱动侧 655M，整卡 663/1024M，退出回 3M；账本只记 30M，剩下 600M 是驱动和库内部吃掉的。空白窗也要 300M 打底。Skia 的 GL 路单窗 30M 级别。
- 结论：WebGPU 吃显存是天生的，三端都这样。1G 卡上多开窗口，只能走 GL。

## 1. 目标和不目标

- 目标：Linux 和 Windows 走纯 Go GL、macOS 走纯 Go Metal 真能上屏，画面跟现在一模一样，三窗（图标/鹈鹕/列表）都过，单窗驱动侧显存降一个数量级。
- 不目标：不动画面，不动上层接口；视频链路不动；不在示例里绕。

## 2. 范围（三端一次写清）

| 平台 | 窗口 | 状态 | 走哪个系统接口 |
|---|---|---|---|
| Linux X11 | 接 | 要做 | EGL + Xlib 窗（`hal.SurfaceTargetXlibWindow`），纯 Go GL |
| Linux Wayland | 接 | 要做 | EGL + `wl_egl_window` 包装（`hal.SurfaceTargetWaylandSurface`，经 libwayland-egl），纯 Go GL |
| Windows | 接 | 要做 | EGL + HWND（`hal.SurfaceTargetWindowsHWND`，已有注册 `hal/gles`），纯 Go GL |
| macOS | 接 | 要做 | 纯 Go Metal（`hal/metal` 已搬，`SurfaceTargetMetalLayer`），不走 GL |
| Android | 不接 | 原因：纯 Go 套没接 `AndroidNativeWindow` | 以后再说 |

## 3. 搬啥（接点）

- 搬的代码：`hal/gles` 本体（约 1.1 万行）+ `egl` + `gl` 两个子包（约 5800 行），测试文件一起搬。落到 `gpu/gwgpu/` 下按原样摆三块，`gles/`、`egl/`、`gl/`。
- 断根三条：不加 `goffi`（调用换 gpui 现成的 purego，自研 `ffishim` 垫片）、不加 `gputypes`（换 gpui 自己的 `gpu/types`+补 7 类小类型）、naga 也自己养（最小闭包 `wgsl+ir+glsl+internal+根门面`，spirv/msl/hlsl/dxil 不搬）；go.mod 无新增外部依赖。以后这三块咱们自己修，不跟那边合。
- gpui 侧接线：`gpu/webgpu` 还是统一的口，默认 Rust 老路零改；`render/present_target.go` 的建面和同步按后端二选一。
- 着色器：现在 WGSL 下发，GL 要转 GLSL（naga 那套）。转完逐个对像素，差了就停。

## 4. 分期（含每步完成状态）

- P0 验证（已完）：34 实现 + 21 单测搬进 `gpu/gwgpu/`；断根三条落实；格式/vet 全干净；单测全绿；真机 EGL 1.5 初始化一次过（Mesa，surfaceless）。
- P1 三角验证（已完）：离屏 FBO 红三角蓝底一次过（中心 255,0,0、四角 0,0,255），WGSL→GLSL 经自养 naga（161 字节）；GL 上报 NVIDIA 940MX；驻留整卡 6/1024M（桌面待机 3M，GL 上下文约 3M），相对 WebGPU 空白窗 300M+、鹈鹕 655M 差两个数量级。验证程序放 /tmp 未进仓。
- P1 单窗打通（未开始）：鹈鹕单窗 GL 上屏，像素零差，帧数和同步正常。
- P2 三窗回归（未开始）：图标/鹈鹕/列表各按现有时长重跑，像素加帧间隔加人工看，三证据齐。
- P3 双后端并存（进行中）：默认不动，`SelectBackend` 函数切换（`BackendNative` 默认，`BackendPureGo` 切纯 Go），不支持自动回退。Windows 同样走一遍 P0-P2。
- H 无感知收敛（2026-09-26 已重置按新顺序重做，旧 H1/H2/H3 记录转历史，备份在 `/tmp/opencode`；每批格式+vet+单测+T5/T12 逐位零差，差一像素就停）：
  - H1 `hal` 对齐 `webgpu`（已完，2026-09-26 门禁收：`gofmt` 零输出+`go vet ./gpu/hal/... ./gpu/gwgpu/...` 绿+`hal/noop/gles/types` 整包绿+`go build ./gpu/... ./render/...` 绿+T5/T12 离屏 `md5` 对文档指纹（T5 `58315c52…`、T12 `840ffe6d…`，`render` 未动故画面无影响）；`webgpu` 包带库（`WGPU_NATIVE_PATH=lib/libwgpu_native.so` 指到文件）59 过 1 败，败的是 `TestS68_Swapchain_X11_MultiFramePresent`（重跑复现，`gpu/webgpu` 零改动，与本批无关，H2 前需另看），`metal` 包本机无测试文件跳过；对照表 16 行行行有下落）：
  - H2 `webgpu` 实现新 `hal` 口（进行中，2026-09-26 H2-a 资源已做：`Buffer/Texture/Sampler/Shader/BindLayout/PipeLayout/BindGroup/RenderPipe/ComputePipe/Fence/CommandBuffer/SurfaceTexture` 补 `Destroy/NativeHandle`（纹理另补 `CurrentUsage/PendingRef`，表面纹理 `Format` 转调底层）+ `var _ hal.Xxx` 全过；`TextureView.Texture()` 改回 `hal.Texture`，`render` 5 处直调加断言转回具体类型（行为不变）；`webgpu` 内部零 `.Texture()` 调用；格式+`vet`（`webgpu/hal`，`render` 存量 3 处无关）+`hal/noop/gles/types` 整包绿+`webgpu` 带库 59 过 1 败（同 H1 的 X11 用例，与本批无关）+T5/T12 零差；H2-b1 简单描述符已做：`gpu/webgpu/descriptor.go` 8 别名（`Extent3D/Origin3D/ImageDataLayout/Buffer/Texture/View/Sampler/CommandEncoder`）直指 `hal`，`ShaderModule` 形状不同（`hal` 是 `Source` 壳，`webgpu` 是扁平 `WGSL/SPIRV`）未动待 `hal` 按 `webgpu` 改；同 H1 三件套+T5/T12 零差+带库 59 过 1 败同上；H2-b2 着色器描述符已做：`hal.ShaderModuleDescriptor` 压扁对齐 `webgpu`（`Label/WGSL/SPIRV`，删 `ShaderSource` 壳），`gles/metal` 存 `wgsl/spirv` 双字段、`compileWGSLToGLSL` 改收 `wgsl string`（只用 WGSL，SPIRV 存着 H4 定），`noop` 测试同改，`webgpu` 别名到 `hal`；同 H1 三件套+T5/T12 零差+带库 59 过 1 败同上；H2-b3 围栏五件套已做：`webgpu.Device` 的 `CreateFence/DestroyFence/ResetFence/GetFenceStatus/WaitForFence` 换 `hal.Fence` 签名（内部拆包，行为不变），删 4 个 `*HAL` 过渡（`render` 零调用，`webgpu` 包内零调用）；同 H1 三件套+T5/T12 零差+带库 59 过 1 败同上；H2-b4 模板两件套已做：`gpu/webgpu/descriptor.go` 的 `StencilFaceState/DepthStencilState` 别名到 `hal`（字段逐个核过同构，枚举经 `types` 同一包，`render` 构造字段名不动，`webgpu` 内转换函数照用）；同 H1 三件套+T5/T12 零差+带库 59 过 1 败同上；H2-b5 表面配置已做：`hal.SurfaceConfiguration` 删 `EnableDamagePresent` 对齐 `webgpu`（源码数过全仓零读取，后端 `Configure` 皆未用，`render` 零构造），`webgpu` 别名到 `hal`（`surface.go/swapchain.go` 6 字段构造不动）；同 H1 三件套+T5/T12 零差+带库 59 过 1 败同上；H2-b6 绑定布局描述符已做：`gpu/webgpu/descriptor.go` 的 `BindGroupLayoutDescriptor` 别名到 `hal`（`Label+Entries` 同构，条目经 `types` 同一包，`device.go` 转换循环不动，`render` 约 36 处构造不动）；同 H1 三件套+T5/T12 零差+带库 59 过 1 败同上；H2-b7 计算通道描述符已做：`gpu/webgpu/descriptor.go` 的 `ComputePassDescriptor` 别名到 `hal`（多 `TimestampWrites`，`webgpu` 实现只读 `Label` 忽略余下，`render` 只设 `Label` 不动）；同 H1 三件套+T5/T12 零差+带库 59 过 1 败同上；H2-b8 纹理范围两件套已做：`gpu/webgpu/descriptor.go` 的 `TextureRange/TextureUsageTransition` 别名到 `hal`（无资源字段，枚举经 `types` 同一包，`render` 键名构造不动）；同 H1 三件套+T5/T12 零差+带库 59 过 1 败同上；H2-b9 计算间接派发已做：`gpu/webgpu/computepass.go` 的 `DispatchIndirect` 换 `hal.Buffer`（内部拆包，`render` 传具体指针照用不动），删 `DispatchIndirectHAL` 过渡（全仓零调用）；同 H1 三件套+T5/T12 零差+带库 59 过 1 败同上；H2-b10 渲染索引间接绘制已做：`gpu/webgpu/renderpass.go` 的 `DrawIndexedIndirect` 换 `hal.Buffer`（内部拆包，`render` 传具体指针照用不动），删 `DrawIndexedIndirectHAL` 过渡（计数版改直调本方法，`DrawIndirectCount` 原样保留）；同 H1 三件套+T5/T12 零差+带库 59 过 1 败同上；H2-b11 渲染直接间接绘制已做：`gpu/webgpu/renderpass.go` 的 `DrawIndirect` 换 `hal.Buffer`（内部拆包，`render` 传具体指针照用不动），删 `DrawIndirectHAL` 过渡（计数版改直调本方法）；同 H1 三件套+T5/T12 零差+带库 59 过 1 败同上；H2-b12 渲染顶点缓冲已做：`gpu/webgpu/renderpass.go` 的 `SetVertexBuffer` 换 `hal.Buffer`（内部拆包，`render` 传具体指针照用不动），删 `SetVertexBufferHAL` 过渡（全仓零调用）；同 H1 三件套+T5/T12 零差+带库 59 过 1 败同上；H2-b13 渲染索引缓冲已做：`gpu/webgpu/renderpass.go` 的 `SetIndexBuffer` 换 `hal.Buffer`（`IndexFormat` 经 `types` 同一包不动，内部拆包，`render` 传具体指针照用不动），删 `SetIndexBufferHAL` 过渡（全仓零调用）；同 H1 三件套+T5/T12 零差+带库 59 过 1 败同上；H2-b14 渲染管线设置已做：`gpu/webgpu/renderpass.go` 的 `SetPipeline` 换 `hal.RenderPipeline`（内部拆包，`render` 传具体指针照用不动），删 `SetPipelineHAL` 过渡（全仓零调用）；同 H1 三件套+T5/T12 零差+带库 59 过 1 败同上；H2-b15 计算管线设置已做：`gpu/webgpu/computepass.go` 的 `SetPipeline` 换 `hal.ComputePipeline`（内部拆包，`render` 传具体指针照用不动），删 `SetPipelineHAL` 过渡（全仓零调用）；同 H1 三件套+T5/T12 零差+带库 59 过 1 败同上；H2-b16 计算绑定组已做：`gpu/webgpu/computepass.go` 的 `SetBindGroup` 换 `hal.BindGroup`（`var _ hal.BindGroup` 已在，内部拆包，`render` 传具体指针照用不动），删 `SetBindGroupHAL` 过渡（全仓零调用）；同 H1 三件套+T5/T12 零差+带库 59 过 1 败同上；H2-b17 队列三件套+Present已做：`gpu/webgpu/queue.go` 的 `Submit` 改 `...hal.CommandBuffer` 变参（首遍拆包，错类型先于标记报错，R7.0 语义保留）、`WriteBuffer` 换 `hal.Buffer`、`WriteTexture` 换 `*hal.ImageCopyTexture/*hal.ImageDataLayout/*hal.Extent3D`（`.Texture` 拆包），新增 `Present`（转调 `Surface.Present`，damage 忽略）+`GetTimestampPeriod`（回 0）+`SupportsCommandBufferCopies`（回 true）适形 `hal.Queue`，加 `var _ hal.Queue`，删 `SubmitHAL/WriteBufferHAL` 过渡；`render` 两处 spread 适配（`render_session.go:submitWithLeading`、`filter_gpu_graph.go:runGPUFilterGraphEx`，逐元转 `[]hal.CommandBuffer`，元素原样行为不变）；H2-b18 错误别名+Device三件套已做：`gpu/webgpu/error.go` 的 `GPUError/ErrorFilter` 别名到 `hal`（删本地 struct/String/Error 方法），`device.go` 的 `PushErrorScope` 换 `hal.ErrorFilter`、`PopErrorScope` 回 `*hal.GPUError`、`FreeCommandBuffer` 收 `hal.CommandBuffer`，删 `FreeCommandBufferHAL/PushErrorScopeHAL/PopErrorScopeHAL` 过渡，同批补 `Device.Destroy` 系+`MapBuffer/UnmapBuffer` 适形 `hal.Device`（`Device` 断言仍缺：`Create*` 返回具体指针待改接口，留 H3）；H2-b19 编码器拷贝链+渲染描述符+断言收官已做：`encoder.go` 的 `CopyBufferToBuffer/ClearBuffer` 收 `hal.Buffer`，三拷贝收 `hal` 接口+`hal` region 切片（逐 region 拆包），`TransitionTextures` 收 `[]hal.TextureBarrier`，删 `CopyBufferToBufferHAL/TransitionTexturesHAL` 过渡；`descriptor.go` 新增 `RenderPassDescriptor` 三件套+`ImageCopyTexture/TextureCopy/BufferTextureCopy/TextureBarrier` 别名到 `hal`；`BeginRenderPass/BeginComputePass` 内 View 拆包改断言式；补 `hal` 适形填充（`CommandEncoder: Begin/EndEncoding` 等，`CommandBuffer.Destroy`，`RenderPass: IndirectCount/ExecuteBundle`）；加 `var _ hal.RenderPassEncoder/ComputePassEncoder/CommandBuffer`（连 b17 的 Queue 全过；未加 `Device/CommandEncoder/Surface/Adapter/Instance`，原因：`Create*` 返回改接口+表面/实例体系属 H3/H4）；新增 `h2b_hal_conformance_test.go` 7 个零 native 依赖单测全绿；`*HAL` 残留仅浏览器占位与注释；H2-b 参数侧收官（返回改接口留 H3）；三批门禁同 H1 三件套+T5/T12 零差+带库 66 过 1 败（59 基线+7 新单测，同 H1 X11 用例无关））：
  - H3 `render` 全换 `hal` 口（未开始）：只存 `hal.xxx` 接口，先接上去跑通。
  - H4 `gwgpu` 补实现并收官（未开始）：`render` 跑通后再做，对照新 `hal` 改、对照 `webgpu` 功能补；三端构建+全量单测+鹈鹕单窗真上屏（像素零差+显存对照）。
- H1 施工注记（以源码为准）：`hal.Device` 加口会打破 `noop/gles/metal` 的 `OpenDevice` 组装（`Device: &Device{}` 不再满足接口），所以每加一个口同批必须把四家空实现补上，否则构建红；这算保绿垫片，不算 H4 的功能实现。
- 历史留档：2026-09-25 前的 H1/H2/H3 交货记录已回退干净（`gpu` 58 改回+18 新增删，`render` 100 改回+2 新增删，备份在 `/tmp/opencode`），待按新 H1-H4 重做，明细见 git 历史与备份 patch。

## 附：H1 对照表（以源码为准，2026-09-26 拉取，改动中发现随时补）

> 读法：第一列是 `render` 现在调的 `webgpu` 方法（源码数过）；第二列是 `hal` 现状；第三列是 H1 对 `hal` 的动作。`hal` 只改接口不写实现，对不上直接改 `hal`，不改 `webgpu`。

| render 用的 webgpu 方法 | hal 现状 | H1 动作 |
|---|---|---|
| `Device.CreateBuffer/CreateTexture/CreateTextureView/CreateSampler/CreateShaderModule/CreateBindGroupLayout/CreatePipelineLayout/CreateBindGroup/CreateRenderPipeline/CreateComputePipeline/CreateCommandEncoder`（各约 20-70 处） | `hal.Device` 同名创建全有，返回 `hal` 接口（`webgpu` 返回具体指针） | `hal` 不动形状；H2 把 `webgpu` 返回改成 `hal` 接口 |
| `device.Queue()`（5 处）、`device.Features()/Limits()`（各约 20 处） | `hal.Device` 无 `Queue/Features/Limits`（`Adapter.Open` 一次给 `Device+Queue`） | `hal.Device` 补 `Queue()/Features()/Limits()`，按 `webgpu` 签名 |
| `device.Poll(PollWait)`（10 处）、`device.WaitIdle()`（8 处）、`device.IsLost()`（4 处）、`device.FlushCallbacks()`（2 处） | `hal.Device` 有 `WaitIdle`，无 `Poll/IsLost/FlushCallbacks` | `hal.Device` 补 `Poll/IsLost/FlushCallbacks`，按 `webgpu` 签名 |
| `device.PushErrorScope(ErrorFilterValidation)/PopErrorScope()`（各 1 处） | `hal` 无错误作用域口 | `hal.Device` 补 `PushErrorScope/PopErrorScope`，按 `webgpu` 签名 |
| `device.MapAsync/MapState/MappedRange/Map/Unmap`（`Buffer` 上各约 17 处，经 `MapHal` 三帮手） | `hal` 是 `Device.MapBuffer/UnmapBuffer`（同步阻塞，后端消化） | `hal` 保留 `Map/Unmap`，删门面三帮手，`render` 直调 `hal.Device` 方法 |
| `device.Destroy()/Release()/FreeCommandBuffer()` | `hal.Device` 有 `Destroy/FreeCommandBuffer`，另有 `DestroyBuffer` 等逐资源销毁 | `hal` 补 `Release` 语义说明（与 `Destroy` 二选一，按 `webgpu` 定一名），`render` 跟改 |
| `device.CreateFence/DestroyFence/ResetFence/GetFenceStatus/WaitForFence` | `hal.Device` 有 `CreateFence/DestroyFence/Wait/ResetFence/GetFenceStatus`（`Wait` 对 `WaitForFence`） | `hal` 把 `Wait` 改名对齐 `webgpu` 或注明对应关系，老调用一起改 |
| `queue.Submit(...)`（38 处，变参）、`queue.WriteBuffer`（42 处）、`queue.WriteTexture`（35 处） | `hal.Queue.Submit([]CommandBuffer)` 切片形，`WriteBuffer/WriteTexture` 收 `hal` 形 | `hal.Queue.Submit` 改变参对齐 `webgpu`；`Write` 系只换接口形，字段不动 |
| `queue.Poll()`、`queue.LastSubmissionIndex()`、`queue.OnSubmittedWorkDone()` | `hal.Queue` 是 `PollCompleted()`，无后两者 | `hal.Queue` 补 `Poll/LastSubmissionIndex` 按 `webgpu` 改名，`OnSubmittedWorkDone` 留门面（hal 不收 future） |
| `encoder.BeginRenderPass/BeginComputePass`（约 17 处，回 `(*Pass, error)`） | `hal` 回 `RenderPassEncoder/ComputePassEncoder` 无 error | `hal` 按 `webgpu` 加 error（或 `webgpu` 去 error，二选一，定完全量改） |
| `encoder.CopyBufferToBuffer(src,srcOff,dst,dstOff,size)` 扁平五参、`CopyBufferToTexture/CopyTextureToBuffer/CopyTextureToTexture/ClearBuffer/TransitionTextures/DiscardEncoding/Finish/Status` | `hal` 是 `regions` 数组形（`CopyBufferToBuffer(src,dst,regions)`），另有 `BeginEncoding/EndEncoding/ResetAll/ResolveQuerySet` 等 `webgpu` 没有的 | 拷贝按 `webgpu` 扁平签名改 `hal`；`hal` 多出来的 `Begin/EndEncoding` 等删或注明去向，`Finish/Status/Discard` 按 `webgpu` 补进 `hal` |
| `pass.SetPipeline/SetBindGroup/SetVertexBuffer/SetIndexBuffer/SetViewport/SetScissorRect/SetBlendConstant/SetStencilReference/Draw/DrawIndexed/DrawIndirect/DrawIndexedIndirect/End/Dispatch/DispatchIndirect` | `hal` 的 `Draw` 收结构体（`DrawArgs`）、`SetViewport/SetScissorRect` 收结构体、有 `DrawIndirectCount/ExecuteBundle`，`End()` 无 error | 画法与视口按 `webgpu` 扁平签名改 `hal`；`Count/ExecuteBundle` 留 `hal`（`webgpu` 照补或注明不用）；`End` 回错按 `webgpu` 加 error |
| `Buffer.Size/Usage/Label/Release/MapState`、`Texture.Format/Release`、`TextureView.Texture/Release`、`Sampler/ShaderModule/Adapter/Instance` 创建与查询系 | `hal` 资源口只有 `Release/CurrentUsage/PendingRef/NativeHandle`，无 `Size/Usage/Label/Format/Texture()` 查询 | `hal` 资源口按 `webgpu` 补查询方法（`Size/Usage/Label/Format/ParentTexture` 等），只读不改状态 |
| `CreateInstance/RequestAdapter/RequestDevice/DeviceDescriptor/RequestAdapterOptions/BackendsPrimary/PowerPreference*`（各约 10 处） | `hal.Instance` 是 `CreateSurface/EnumerateAdapters`，`hal.Adapter` 是 `Open`，层级与 `webgpu` 不一样 | `hal` 按 `webgpu` 补创建入口形态（`CreateInstance/RequestAdapter/RequestDevice`），或注明 `hal` 注册表如何对应，定完 `render` 创建入口一起换 |
| `NewSwapchain/DeviceFromHandle/AdapterFromHandle/NativeViewToHandle/SimpleDeviceProvider/VramLiveBytes/SetLogger/AfterSurfaceUnconfigure/BeforeDeviceRecover` | `hal` 无这批（创建/观测/桥接归门面） | 已定案（2026-09-26源码核过）：不进 `hal`。`NewSwapchain` 是 surface+device 编排（`gpu/webgpu/swapchain.go:153`）；句柄桥归 `gpucontext` 门面（`gpu/webgpu/gpucontext_helpers.go`，`NativeViewToHandle` 查无此名）；`SimpleDeviceProvider` 是 `gpucontext.DeviceProvider` 注入（`gpu/webgpu/device_provider.go`）；`Vram*` 读 `rwgpu` 账本（`gpu/webgpu/vram.go`）；`SetLogger` 两边各自独立（`gpu/hal/logger.go` vs `gpu/webgpu/logger.go`）；`After/Before` 是 `render` 注册的全局钩子（`gpu/webgpu/lifecycle_hooks.go`）；`ErrDeviceLost` 两边哨兵已齐（`gpu/hal/error.go:25` vs `gpu/webgpu/error.go:22`），接线待H2/H4 |
| 描述符/枚举（`Extent3D/TextureDescriptor/BufferDescriptor/RenderPassDescriptor/StencilOperation*/TextureFormat/BufferUsage` 等，约 60 类） | `hal` 描述符字段与 `webgpu` 基本同构，枚举多经 `types` | 逐字段对，差一字段就按 `webgpu` 改 `hal`；`render` 直写 `hal.Xxx`/`types.Xxx`，删冗余别名 |

## 5. 门禁

- 像素：T5/T12 两时刻 Golden 逐位对（T5 `58315c5278c968e2e57aa7f801ddbfde`、T12 `840ffe6d4f16baf1f16b26013e4de46e`，`cmp` 零差），容差写明；逻辑探针加像素断言加 Golden，缺一不可。
- 显存：`nvidia-smi` 量单窗和三窗同开，对比现在这套，写明降了多少。
- 回归：按测试文件逐个跑，不一次全量；长跑给足时间；缺数据用 `t.Skipf`，不假绿。
- 跨平台：上表一次标齐，不留空白格；版本号加修订一起走。
- 无感知验收（H 专用）：`render` 里搜不到 `webgpu.` 具体类型引用（创建入口除外）；`gpu/webgpu` 与 `gpu/gwgpu` 里无双字段标记分支；`hal` 口相关类型猜测零残留（自家测试白盒与 objc/msl 内部除外，注释写清）；冗余别名零残留（`render` 直写 `hal.Xxx`/`types.Xxx`）。

## 6. 风险

- 两个 GPU 实现并存（Rust 原生和纯 Go），依赖和构建变复杂。做法：GL 独立分支，默认不动，出问题一键回退。
- GL 驱动差异（Mesa 新版本和旧卡行为不同）。做法：先在现有真机（HD 520/940MX）验，再谈别的卡。
- 着色器转译差一像素。做法：像素门禁不过就停，不硬上。

## 7. 修订

| 日期 | 说明 |
|---|---|
| 2026-09-24 | 立项：Linux+Windows 接 GL，macOS 走 Metal；P0 先验显存再往下走 |
| 2026-09-24 | P0/Q1 搬完：断根三条落实，hal/gles/naga 单测全绿，真机 EGL 1.5 过 |
| 2026-09-25 | S/H 立项与修订：hal 统一口、facade 作废、Rust 为准、口径外与别名纪律（已作废，见下条） |
| 2026-09-26 | 方向重置：`hal` 以 `render` 用的 `webgpu` 为准，四步顺序（hal→webgpu→render→gwgpu 最后），旧记录转历史，工作区 `gpu`/`render` 恢复干净 |
| 2026-09-26 | 文档收敛：§S 只留现行四步，历史压成一行，门禁写死 T5/T12 指纹 |
| 2026-09-26 | H1 row2：`hal.Device` 补 `Queue/Features/Limits`，新增 `PollType/ErrorFilter/GPUError` 照抄 `webgpu`；`noop/gles/metal` 同批空实现保绿；`go build ./gpu/... ./render/...`+`vet`+`hal/noop` 单测绿，`render` 未动像素无影响 |
| 2026-09-26 | H1 row3/4：`hal.Device` 补 `Poll/IsLost/FlushCallbacks`+`PushErrorScope/PopErrorScope`，按 `webgpu` 签名；四家空实现同批保绿，同上三件套验证绿 |
| 2026-09-26 | H1 围栏：`hal.Device.Wait` 改名 `WaitForFence` 对齐 `webgpu`，四家实现与三处测试调用同改；同上三件套验证绿 |
| 2026-09-26 | H1 队列：`hal.Queue.Submit` 改变参、`PollCompleted` 改名 `Poll`、补 `LastSubmissionIndex`，四家实现与 `hal` 内测试调用同改；`OnSubmittedWorkDone` 留门面不进 `hal`；同上三件套验证绿 |
| 2026-09-26 | H1 编码器开端：`BeginRenderPass/BeginComputePass` 回 `error` 对齐 `webgpu`，`gles` 顺手加 `nil` 守卫、`metal` 空编码器改报错；四家测试调用同改；同上三件套验证绿 |
| 2026-09-26 | H1 拷贝：`CopyBufferToBuffer` 改扁平五参对齐 `webgpu`，删 `hal.BufferCopy`；三家实现与测试同改；同上三件套验证绿 |
| 2026-09-26 | H1 拷贝补：`hal.TextureCopy.SrcBase/DstBase` 改名 `Source/Destination` 对齐 `webgpu`，`gles/metal` 实现与 `metal` 测试同改；`CopyBufferToTexture/CopyTextureToBuffer/ClearBuffer/TransitionTextures/DiscardEncoding` 源码核过形状已一致不改；`Finish/EndEncoding` 与 `BeginEncoding/ResetAll/Destroy` 去向、`Status` 归属（`MapPending.Status`，非编码器口）待定案；同上三件套验证绿 |
| 2026-09-26 | H1 画法：`Draw/DrawIndexed/SetViewport/SetScissorRect/DrawIndirect/DrawIndexedIndirect` 改扁平对齐 `webgpu`，`RenderPass/ComputePass.End` 回 `error`，`metal` 多记录循环压成单记录、`IndirectCountRecorder` 同改；`Count/ExecuteBundle` 留 `hal`（`webgpu` 无对应口）；`hal` 内与 `gles/metal` 测试调用同改；同上三件套验证绿 |
| 2026-09-26 | H1 资源查询：`hal.Buffer` 补 `Size/Usage/Label`，`hal.Texture` 补 `Format`（含表面纹理），`hal.TextureView` 补 `Texture()`，按 `webgpu` 签名只读不改状态；`noop` 新增视图类型存父纹理，`gles/metal` 补字段与方法，测试 mock 同改；`MapState` 留 `render` 封装（`hal` 同步 `Map/Unmap` 不跟踪异步态），`Sampler/Shader/Adapter/Instance` 无查询口不改；`render` 未动，同上三件套验证绿 |
| 2026-09-26 | H1 创建入口：`hal.Instance` 补 `RequestAdapter/ProcessEvents`，`hal.Adapter` 补 `Info/Features/Limits/RequestDevice`，`SurfaceCapabilities` 改名 `GetSurfaceCapabilities`，新增 `RequestAdapterOptions/DeviceDescriptor` 照抄 `webgpu`；四家同批最小实现（首个枚举适配器直返，`RequestDevice` 转调 `Open`，`ProcessEvents` 空实现，`metal` 缓存枚举信息），测试 mock 同改；免费 `CreateInstance` 不进 `hal`（走注册表 `GetBackend`，`SelectBackend` H3 定）；`render` 未动，同上三件套验证绿 |
| 2026-09-26 | H1 门面定案：`NewSwapchain`/句柄桥/`SimpleDeviceProvider`/`Vram*`/`SetLogger`/`AfterSurfaceUnconfigure`/`BeforeDeviceRecover` 源码核过留门面不进 `hal`；`NativeViewToHandle` 查无此名；`ErrDeviceLost` 两边哨兵已齐接线待H2/H4；`hal` 代码零改，`render` 未动 |
| 2026-09-26 | H1 描述符（第一批）：`SamplerDescriptor.MipmapFilter` 改 `MipmapFilterMode` 对齐 `webgpu`，`ComputePipelineDescriptor` 压扁（删 `ComputeState` 壳，`Module/EntryPoint` 提上来，`Constants/ZeroInit` 随壳删，`render` 未用，浏览器 `Constants` H4 定）；三家实现与测试同改；`Extent3D/Origin3D/ImageDataLayout/ImageCopyTexture` 与 `webgpu` 字段已同构（`hal` 版注释多，`Size/Usage/Format` 等枚举经 `types` 一致），`Buffer/Texture/View/RenderPass/Bind/Pipeline` 等其余描述符仅注释与别名差，字段无差不改；同上三件套验证绿 |
| 2026-09-26 | H1 门禁收：`gofmt -l` 零输出+`vet`（`hal/gwgpu`）绿+`hal/noop/gles/types` 整包绿+全量构建绿+T5/T12 离屏 `md5` 对指纹一致（`render` 未动）；`webgpu` 带库（`WGPU_NATIVE_PATH=lib/libwgpu_native.so`）59 过 1 败，败的是 `TestS68_Swapchain_X11_MultiFramePresent`（单跑复现，`gpu/webgpu` 零改动，与本批无关）；`metal` 本机无测试文件；H1 关账，待 H2 |
| 2026-09-26 | H2-a 资源：12 资源+表面纹理 `var _ hal.Xxx` 全过（`TextureView.Texture()` 改回 `hal.Texture` 为唯一签名改动，`render` 5 处断言同改行为不变）；同 H1 三件套+T5/T12 零差；`Device/Queue/Encoder/Pass/Surface/Adapter/Instance` 缺口（描述符别名+返回改接口）待 H2-b |
| 2026-09-26 | H2-b1 简单描述符：`gpu/webgpu/descriptor.go` 8 别名到 `hal`（`Extent3D/Origin3D/ImageDataLayout/Buffer/Texture/View/Sampler/CommandEncoder`，字段源码核过同构，`Usage/Format` 等经 `types` 同一包）；`ShaderModule` 未动（`hal.Source` 壳 vs `webgpu` 扁平，待 `hal` 按 `webgpu` 改）；`gofmt` 零输出+`vet`（`hal/webgpu`）绿+`hal/noop/types/gles` 整包绿+`go build ./gpu/... ./render/...` 绿+T5/T12 对指纹一致+带库 59 过 1 败（同 H1 X11 用例无关）；浏览器 `descriptor_browser.go` 未动（`js/wasm` 独立） |
| 2026-09-26 | H2-b2 着色器描述符：`hal.ShaderModuleDescriptor` 压扁对齐 `webgpu`（`Label/WGSL/SPIRV`，删 `ShaderSource` 壳）；`gles`（`resource/shader/device/device_linux`）存 `wgsl/spirv` 双字段、`compileWGSLToGLSL` 改收 `wgsl string`，`metal`（`resource/device`）同改；`hal/noop` 测试与 `bench` 同改；`webgpu` 别名到 `hal`（`device.go` 的 `desc.WGSL/SPIRV` 不动，`render` 的 `WGSL:` 构造不动）；`gofmt` 零输出+`vet`（`hal/webgpu`）绿+`hal/noop/types/gles` 整包绿+全量构建绿+T5/T12 对指纹一致+带库 59 过 1 败（同 H1 X11 用例无关） |
| 2026-09-26 | H2-b3 围栏五件套：`gpu/webgpu/device.go` 的 `CreateFence/DestroyFence/ResetFence/GetFenceStatus/WaitForFence` 换 `hal.Fence` 签名（`Create` 回接口，余下收接口内部拆包，行为不变），删 4 个 `*HAL` 过渡（`Destroy/Reset/GetStatus/Wait`）；`render` 零调用（源码数过 `Create/Destroy/Reset/GetStatus/Wait` 皆 0），`webgpu` 包内零调用，测试零调用；`gofmt` 零输出+`vet`（`webgpu`）绿+全量构建绿+`hal/noop/types/gles` 整包绿+T5/T12 对指纹一致+带库 59 过 1 败（同 H1 X11 用例无关） |
| 2026-09-26 | H2-b4 模板两件套：`gpu/webgpu/descriptor.go` 的 `StencilFaceState/DepthStencilState` 别名到 `hal`（字段逐个核过同构，`CompareFunction/StencilOperation/TextureFormat` 经 `types` 同一包，`render` 构造字段名不动，`webgpu` 内 `convertDepthStencilStateInto` 照用）；`gofmt` 零输出+`vet`（`webgpu`）绿+全量构建绿+`hal/noop/types/gles` 整包绿+T5/T12 对指纹一致+带库 59 过 1 败（同 H1 X11 用例无关） |
| 2026-09-26 | H2-b5 表面配置：`hal.SurfaceConfiguration` 删 `EnableDamagePresent` 对齐 `webgpu`（源码数过全仓零读取，`noop/gles/metal` 的 `Configure` 皆未用，`render` 零构造，`hal` 测试只用 6 字段）；`webgpu` 别名到 `hal`（`surface.go/swapchain.go` 6 字段构造不动，`rwgpu` 转换不动）；`gofmt` 零输出+`vet`（`hal/webgpu`）绿+全量构建绿+`hal/noop/types/gles` 整包绿+T5/T12 对指纹一致+带库 59 过 1 败（同 H1 X11 用例无关） |
| 2026-09-26 | H2-b6 绑定布局描述符：`gpu/webgpu/descriptor.go` 的 `BindGroupLayoutDescriptor` 别名到 `hal`（`Label+Entries` 同构，条目经 `types` 同一包，`device.go` 转换循环不动，`render` 构造不动）；`gofmt` 零输出+`vet`（`webgpu/hal`）绿+全量构建绿+`hal/noop/types/gles` 整包绿+T5/T12 对指纹一致+带库 59 过 1 败（同 H1 X11 用例无关） |
| 2026-09-26 | H2-b7 计算通道描述符：`gpu/webgpu/descriptor.go` 的 `ComputePassDescriptor` 别名到 `hal`（多 `TimestampWrites`，`encoder.go` 只读 `Label` 忽略余下，`render` 只设 `Label` 不动，`gles` 的 `TimestampWrites` 读取走 `hal` 类型不变）；`gofmt` 零输出+`vet`（`webgpu/hal`）绿+全量构建绿+`hal/noop/types/gles` 整包绿+T5/T12 对指纹一致+带库 59 过 1 败（同 H1 X11 用例无关） |
| 2026-09-26 | H2-b8 纹理范围两件套：`gpu/webgpu/descriptor.go` 的 `TextureRange/TextureUsageTransition` 别名到 `hal`（无资源字段，枚举经 `types` 同一包，`render` 键名构造不动，`TextureBarrier.Range/Usage` 类型随别名走）；`gofmt` 零输出+`vet`（`webgpu/hal`）绿+全量构建绿+`hal/noop/types/gles` 整包绿+T5/T12 对指纹一致+带库 59 过 1 败（同 H1 X11 用例无关） |
| 2026-09-26 | H2-b9 计算间接派发：`gpu/webgpu/computepass.go` 的 `DispatchIndirect` 换 `hal.Buffer`（内部拆包行为不变，`render` 传具体指针照用不动），删 `DispatchIndirectHAL` 过渡（全仓零调用）；`gofmt` 零输出+`vet`（`webgpu`）绿+全量构建绿+`hal/noop/types/gles` 整包绿+T5/T12 对指纹一致+带库 59 过 1 败（同 H1 X11 用例无关） |
| 2026-09-26 | H2-b10 渲染索引间接绘制：`gpu/webgpu/renderpass.go` 的 `DrawIndexedIndirect` 换 `hal.Buffer`（内部拆包行为不变，`render` 传具体指针照用不动），删 `DrawIndexedIndirectHAL` 过渡（计数版改直调本方法，`DrawIndirectCount` 原样保留，全仓余下零调用）；`gofmt` 零输出+`vet`（`webgpu`）绿+全量构建绿+`hal/noop/types/gles` 整包绿+T5/T12 对指纹一致+带库 59 过 1 败（同 H1 X11 用例无关） |
| 2026-09-26 | H2-b11 渲染直接间接绘制：`gpu/webgpu/renderpass.go` 的 `DrawIndirect` 换 `hal.Buffer`（内部拆包行为不变，`render` 传具体指针照用不动），删 `DrawIndirectHAL` 过渡（计数版改直调本方法，全仓余下零调用）；`gofmt` 零输出+`vet`（`webgpu`）绿+全量构建绿+`hal/noop/types/gles` 整包绿+T5/T12 对指纹一致+带库 59 过 1 败（同 H1 X11 用例无关） |
| 2026-09-26 | H2-b12 渲染顶点缓冲：`gpu/webgpu/renderpass.go` 的 `SetVertexBuffer` 换 `hal.Buffer`（内部拆包行为不变，`render` 传具体指针照用不动），删 `SetVertexBufferHAL` 过渡（全仓零调用）；`gofmt` 零输出+`vet`（`webgpu`）绿+全量构建绿+`hal/noop/types/gles` 整包绿+T5/T12 对指纹一致+带库 59 过 1 败（同 H1 X11 用例无关） |
| 2026-09-26 | H2-b13 渲染索引缓冲：`gpu/webgpu/renderpass.go` 的 `SetIndexBuffer` 换 `hal.Buffer`（`IndexFormat` 经 `types` 同一包不动，内部拆包行为不变，`render` 传具体指针照用不动），删 `SetIndexBufferHAL` 过渡（全仓零调用）；`gofmt` 零输出+`vet`（`webgpu`）绿+全量构建绿+`hal/noop/types/gles` 整包绿+T5/T12 对指纹一致+带库 59 过 1 败（同 H1 X11 用例无关） |
| 2026-09-26 | H2-b14 渲染管线设置：`gpu/webgpu/renderpass.go` 的 `SetPipeline` 换 `hal.RenderPipeline`（内部拆包行为不变，`render` 传具体指针照用不动），删 `SetPipelineHAL` 过渡（全仓零调用）；`gofmt` 零输出+`vet`（`webgpu`）绿+全量构建绿+`hal/noop/types/gles` 整包绿+T5/T12 对指纹一致+带库 59 过 1 败（同 H1 X11 用例无关） |
| 2026-09-26 | H2-b15 计算管线设置：`gpu/webgpu/computepass.go` 的 `SetPipeline` 换 `hal.ComputePipeline`（内部拆包行为不变，`render` 传具体指针照用不动），删 `SetPipelineHAL` 过渡（全仓零调用）；`gofmt` 零输出+`vet`（`webgpu`）绿+全量构建绿+`hal/noop/types/gles` 整包绿+T5/T12 对指纹一致+带库 59 过 1 败（同 H1 X11 用例无关） |
| 2026-09-26 | H2-b16 计算绑定组：`gpu/webgpu/computepass.go` 的 `SetBindGroup` 换 `hal.BindGroup`（`bind.go` 底 `var _ hal.BindGroup` 已在，内部拆包行为不变，`render` 传具体指针照用不动），删 `SetBindGroupHAL` 过渡（全仓零调用）；`gofmt` 零输出+`vet`（`webgpu`）绿+全量构建绿+`hal/noop/types/gles` 整包绿+T5/T12 对指纹一致+带库 59 过 1 败（同 H1 X11 用例无关） |
| 2026-09-26 | H2-b17 队列三件套+Present：`gpu/webgpu/queue.go` 的 `Submit` 改 `...hal.CommandBuffer` 变参（首遍拆包，错类型先于标记报错，R7.0 语义保留）、`WriteBuffer` 换 `hal.Buffer`、`WriteTexture` 换 `*hal.ImageCopyTexture/*hal.ImageDataLayout/*hal.Extent3D`（`.Texture` 拆包），新增 `Present`（转调 `Surface.Present`，damage 忽略）+`GetTimestampPeriod`（回 0）+`SupportsCommandBufferCopies`（回 true）适形 `hal.Queue`，加 `var _ hal.Queue`；删 `SubmitHAL/WriteBufferHAL` 过渡（全仓零调用）；`render` 两处 spread 适配（`render_session.go:submitWithLeading`、`filter_gpu_graph.go:runGPUFilterGraphEx`，逐元转 `[]hal.CommandBuffer`，元素原样行为不变）；`gofmt`（本批文件）零输出+`vet`（`webgpu/hal`）绿+全量构建绿+`hal/noop/types/gles` 整包绿+T5/T12 对指纹一致+带库 66 过 1 败（59 基线+7 新单测，同 H1 X11 用例无关） |
| 2026-09-26 | H2-b18 错误别名+Device三件套：`gpu/webgpu/error.go` 的 `GPUError/ErrorFilter` 别名到 `hal`（删本地 struct/String/Error 方法，值与文案一致）；`device.go` 的 `PushErrorScope` 换 `hal.ErrorFilter`、`PopErrorScope` 回 `*hal.GPUError`、`FreeCommandBuffer` 收 `hal.CommandBuffer`（内部拆包行为不变），删 `FreeCommandBufferHAL/PushErrorScopeHAL/PopErrorScopeHAL` 过渡；同批补 `Device.Destroy` 系（Buffer/Texture/View/Sampler/BindLayout/BindGroup/PipeLayout/Shader/双管线）+`MapBuffer/UnmapBuffer` 适形 `hal.Device`（`Device` 断言仍缺：`Create*` 返回具体指针待改接口，留 H3）；门禁同 b17 |
| 2026-09-26 | H2-b19 编码器拷贝链+渲染描述符+断言收官：`encoder.go` 的 `CopyBufferToBuffer/ClearBuffer` 收 `hal.Buffer`，`CopyBufferToTexture/CopyTextureToBuffer/CopyTextureToTexture` 收 `hal` 接口+`hal` region 切片（逐 region 拆包，错类型 region 跳过），`TransitionTextures` 收 `[]hal.TextureBarrier`，删 `CopyBufferToBufferHAL/TransitionTexturesHAL` 过渡；`descriptor.go` 新增 `RenderPassDescriptor/RenderPassColorAttachment/RenderPassDepthStencilAttachment` 三件套别名到 `hal`（多 `TimestampWrites`，实现只读旧字段）+`ImageCopyTexture/TextureCopy/BufferTextureCopy/TextureBarrier` 别名到 `hal`（`.Texture` 为 `hal.Texture` 接口）；`BeginRenderPass/BeginComputePass` 与 `convertRenderPassDescriptorRust` 内 View 拆包改断言式；补 `hal` 适形填充（`CommandEncoder: Begin/EndEncoding/ResetAll/Destroy/TransitionBuffers/ResolveQuerySet`/加速结构四件套，`CommandBuffer.Destroy`，`RenderPass: DrawIndirectCount/DrawIndexedIndirectCount/ExecuteBundle`）；加 `var _ hal.RenderPassEncoder/ComputePassEncoder/CommandBuffer`（连 b17 的 Queue 全过；未加 `Device/CommandEncoder/Surface/Adapter/Instance`，原因：`Create*` 返回改接口+表面/实例体系属 H3/H4）；新增 `h2b_hal_conformance_test.go` 7 个零 native 依赖单测全绿；`*HAL` 残留仅浏览器占位与注释；H2-b 参数侧收官，门禁同 b17 |
| 2026-09-26 | H2-b 收官口径修订（验收 #2 改判）：`Device/CommandEncoder/Surface/Adapter/Instance` 的 `var _ hal.Xxx` 缺口经源码逐项核对，全部属返回侧改动——`Device.Create*` 11 个+`Queue()` 返回具体指针（`hal` 要接口）、`BeginRenderPass/BeginComputePass` 返回具体指针（试改即致 `render` 29 错，已回退）、`Surface.Configure/Unconfigure` 口径不同且缺 `AcquireTexture/Resource` 方法、`Adapter.RequestDevice/Open` 与 `Instance.CreateSurface/RequestAdapter` 返回具体指针且入口形态不同（`hal` 收 `SurfaceTarget`，`webgpu` 收双句柄）；强行加断言等于提前干 H3（`render` 全换 `hal` 口时改返回+调用点同批做），故验收标准正式修订为参数侧收官、返回侧移 H3；`*HAL` 零残留、已加 4 断言、T5/T12 零差结论不变，门禁重跑留新日志 |
