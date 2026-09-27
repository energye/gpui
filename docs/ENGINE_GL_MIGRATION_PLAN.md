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
- 工作区纪律：`gpu`/`render` 已恢复干净（2026-09-26，`video` 不动，改动备份在 `/tmp/opencode`；2026-09-27 片1–7重来前全量备份在 `/tmp/slice-revert-backup`），重做从干净底子开始；代码提交只认人工指令（人工说交才交，不主动交），文档按需提交。

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
- P3 双后端并存（未开始）：默认不动，`SelectBackend` 函数切换（`BackendNative` 默认，`BackendPureGo` 切纯 Go），不支持自动回退。`SelectBackend` 源码尚无此名（现仅 `hal.SelectBestBackend` 注册表帮手，非后端切换），待 H3 定。Windows 同样走一遍 P0-P2。
- H 无感知收敛（2026-09-26 已重置按新顺序重做，旧 H1/H2/H3 记录转历史，备份在 `/tmp/opencode`；每批格式+vet+单测+T5/T12 逐位零差，差一像素就停）：
  - H1 `hal` 对齐 `webgpu`（已完，2026-09-26 门禁收：`gofmt` 零输出+`go vet ./gpu/hal/... ./gpu/gwgpu/...` 绿+`hal/noop/gles/types` 整包绿+`go build ./gpu/... ./render/...` 绿+T5/T12 离屏 `md5` 对文档指纹（T5 `58315c52…`、T12 `840ffe6d…`，`render` 未动故画面无影响）；`webgpu` 包带库（`WGPU_NATIVE_PATH=lib/libwgpu_native.so` 指到文件）59 过 1 败，败的是 `TestS68_Swapchain_X11_MultiFramePresent`（重跑复现，`gpu/webgpu` 零改动，与本批无关，H2 前需另看），`metal` 包本机无测试文件跳过；对照表 16 行行行有下落）：
  - H2 `webgpu` 实现新 `hal` 口（已完，参数侧收官，返回侧移 H3）：H2-a 资源 12 件套＋H2-b1–b19（描述符别名、围栏、队列、拷贝链、断言），明细与门禁见 §7 同名行；`*HAL` 残留仅浏览器占位与注释；`Device/CommandEncoder/Surface/Adapter/Instance` 5 断言缺口属返回侧，移分片片7。
  - H3 `render` 全换 `hal` 口（进行中：7a–7d已完，7e–7f待做，见分片表）：只存 `hal.xxx` 接口，先接上去跑通；创建入口与门面钩子除外（`CreateInstance` 入口形态另议，门面类留 `webgpu`，见分片不做行）。
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

## 附：render→webgpu 引用全量分片清单（2026-09-26 拉取，做完一片勾一片）

> `render` 里写了 `webgpu.` 的地方一共 97 个不同符号，拉取命令见附账本，基线 `38e97481`（即当前 HEAD，2026-09-27 干净树重跑一字不差），`hal` 有无／别名有无逐个现查源码。规矩（四步、铁律、别名与换口同批）见§S，这里不重复；H2-b 已加的别名是过渡，下面各片里"hal＋别名已做完"的只剩 render 侧。新会话按片号从片1往后做，一片四步（现数→点头→动手→验证→勾状态），一次只做一片。
>
> 门禁基线B0（每片通用，下表状态格只写差异，不复述）：`go build ./gpu/... ./render/...` 绿＋`vet`（webgpu/hal）绿＋本批文件 `gofmt` 净＋`hal/noop/types/gles` 基测全绿＋H2B 7过＋带库 66 过 1 败（已知 `TestS68 X11` 用例，同基线）＋T5 `58315c52…`/T12 `840ffe6d…` 逐位一致＋类型断言仍 6 处零新增＋代码未提交。片2起加 `render/gpu` 整包；片5起加 R70；片5/片6加 wasm vet；片5加 darwin metal 门（片6复验）。超出 B0 的（如带库 67 过）记在该片状态格。

| 片 | 内容 | 规模（render 引用处，约） | 状态 |
|---|---|---|---|
| 片1 | 纯数据结构 7 个（`Extent3D` 123、`Origin3D` 14、`ImageDataLayout` 53、`ImageCopyTexture` 57、`BufferTextureCopy` 18、`TextureCopy` 2、`StencilFaceState` 38） | 约 305 | 已完（2026-09-27：`render` 46文件305处换 `hal` 口＋7别名删＋`webgpu`包内5文件跟改；B0全绿：构建/vet/gofmt、`hal/noop/types/gles`、H2B 7过、带库66过1败（已知`TestS68`同基线）、T5离屏200轮过、CPU Golden 6过、断言6处、账本90行2024处；收敛优化：2注释行号跟正＋4格式漂移清零；代码已交本地`88d26090`（人工指令，未推）） |
| 片2 | 枚举常量（住 `types`，`hal` 无；`PollWait`/`MapMode` 除外，见片4；2026-09-27 现数：`MapModeRead` 17处因 `types.MapMode` 与 `webgpu.MapMode` 各自另定（`gpu/types/buffer.go:92` vs `gpu/webgpu/map_types.go:8`）直换编译不过，归片4；片2实做171处21符号：`StencilOperation*` 119（含类型1）／`PowerPreference*` 18（含类型1）／`Backends*` 11／`TextureFormatRGBA8Unorm` 4／`BufferUsage*` 8／`TextureUsage*` 6／`TextureDimension2D` 2／`DefaultLimits` 3） | 约 171 | 已完（2026-09-27：`render` 20文件171处换 `types` 口＋`webgpu`别名删（`descriptor.go`去`Stencil`组＋`types.go`去`Backends`6/`BufferUsage`10/`TextureUsage`5/`Dimension`3/`Format`6/`Power`3＋类型/`DefaultLimits`，`descriptor_browser`同步，`hal`不动）＋`webgpu`包内跟改（`device/adapter/buffer_browser`＋自家测试3＋外部测试4）；B0全绿：构建/vet/gofmt净、`hal/noop/types/gles`绿、H2B 7过、`render/gpu`整包绿、带库66过1败（已知`TestS68 X11`同基线，绝对路径）、T5离屏200轮过（`GPUI_MEM_STRESS=1`＋带库，200/200 rss稳）、Golden 5过、断言6处零新增、账本69行1853处；`render`整包1败`TestS69`系干净树同败（`U05` 7.35 vs 6.58）与本批无关；T12仓内无命令同片1；收敛优化：4注释行号跟正（`types.go:150→101`、`descriptor.go:118→102`、`descriptor_browser.go:180→172`/`270→254`，`map_types.go:26`不动）＋触及文件`gofmt`净＋B0复验全绿（带库仍66过1败同基线）；代码已交本地`a9455c73`（人工指令，未推）） |
| 片3a | 纯别名描述符14个（`BufferDescriptor` 71／`TextureDescriptor` 73／`TextureViewDescriptor` 62／`ShaderModuleDescriptor` 34／`CommandEncoderDescriptor` 34／`SamplerDescriptor` 20／`BindGroupLayoutDescriptor` 36／`ComputePassDescriptor` 2／`RenderPassDescriptor` 29／`DepthStencilState` 21／`RenderPassColorAttachment` 38／`RenderPassDepthStencilAttachment` 10／`TextureBarrier/UsageTransition` 各 9；2026-09-27现数纠正：原`CommandEncoder` 20实为`CommandEncoderDescriptor` 34） | 约 448 | 已完（2026-09-27：`render` 64文件448处换 `hal` 口＋14别名删＋`RenderPipelineDescriptor.DepthStencil` 跟改 `hal`＋`webgpu`包内裸用4文件加前缀＋自家测试5文件＋外部测试1处＋11无用引用清＋触及文件`gofmt`净（含2存量格式收敛）；B0全绿：构建/vet（`hal/webgpu`净，`render/gpu`仅3存量提示同干净树）/`hal/noop/types/gles`绿、H2B 7过、`render/gpu`整包1败（`TestOpt21`干净树同败与本批无关）、带库66过1败（已知`TestS68 X11`同基线）、T5离屏200轮过（rss稳）、Golden 5过、断言6处、账本55行1405处；T12同片1仓内无命令；`BindGroupDescriptor` 33处不在本片（形态对不上，留片5）；收敛优化：3过期注释跟正＋1行号跟正（`hal/descriptor.go:380` 指 `descriptor.go:61`）＋1数字口径（自家测试5文件）；代码已交本地`775323d5`（人工指令，未推）） |
| 片3b | 自有结构体 5 个（`PipelineLayout` 29／`RenderPipeline` 48／`Vertex` 48／`Fragment` 48／`ComputePipeline` 10）：单指针字段赋值兼容，`BindGroupLayouts []*BGL` 切片逐元重建，嵌套 `Vertex/Fragment` 跟换前缀；子描述符经 `types` 同一包不动 | 约 183 | 已完（2026-09-27：`render` 22文件183处换 `hal` 口＋`BindGroupLayouts` 切片29处跟改 `[]hal`（含3变量声明）＋`webgpu` 5旧结构体删＋`device.go` 3个Create收 `hal` 描述符＋6转换函数跟改＋自家测试4文件跟改；`browser` 零改动（独立structs＋`hal`在wasm下不可用，同片3a先例；`browser-compute`示例2处不动）；B0全绿：构建/vet（`hal/webgpu`净）/触及文件`gofmt`净/`hal/noop/types/gles`绿、H2B 7过、转换单测9过、带库66过1败（已知`TestS68 X11`同基线，绝对路径）、T5 stress过、Golden 5过、断言6处零新增、账本50行1193处；`render/gpu`整包绿、`render`整包6败与干净树一字相同与本批无关；T12同片1仓内无命令；收敛优化：1行号注释跟正（`hal/descriptor.go:379`）＋1过期注释跟正（`pipeline_cache_core.go:646`）；收敛2（2026-09-27：3注释跟正——`pipeline_cache_core.go:706`漏网、`device.go:781` our→hal、片3a遗留`texture.go:548`顺手收敛——＋1行号`:170→:172`＋B0复验绿）；代码已交本地`04c4ccee`（人工指令，未推）） |
| 片4 | 两边重复定义 5 个（以 `webgpu` 版为准改 `hal`）：`MapMode`（`gpu/types/buffer.go:92` vs `gpu/webgpu/map_types.go:8`，含 `MapModeRead` 17处调用）／`PollType`+`PollWait`（`gpu/hal/api.go:141` vs `gpu/webgpu/map_types.go:26`，含 `PollWait` 11处）／`RequestAdapterOptions`（`gpu/types/adapter.go:155` vs `gpu/hal/descriptor.go:33` vs `gpu/webgpu/types.go:150`，`CompatibleSurface` 形态三分）／`DeviceDescriptor`（`gpu/types/adapter.go:187` vs `gpu/hal/descriptor.go:45` vs `gpu/webgpu/adapter.go:14`）／`InstanceDescriptor`（`gpu/types/adapter.go:268` vs `gpu/hal/descriptor.go:17` vs `gpu/webgpu/instance.go:16`）；2026-09-27 由片2现数落位 `MapModeRead` | 5 处定义＋`MapModeRead` 17＋`PollWait` 11＋其余调用点 | 已完·形状对齐（2026-09-27：三处问答定方向——①5组都按文档办全换 `hal` 口②`CompatibleSurface` 跟 `webgpu` 但 `hal` 内仍留接口形（`webgpu` 指 `hal` 会循环引用，待片7创建入口统）③`MemoryHints` 删、`XlibDisplay/Screen` 合并进 `hal`——＋动手前发现真耦合：51处调用点挂的全是片7返回侧方法签名，直换等于提前做片7，故本片只做形状对齐、调用点留片7；改动：`types.MapMode` 去 `None` 成员（`buffer.go:95-96` 删，注释注零值非法）、`types.DeviceDescriptor` 去 `MemoryHints` 全套（`adapter.go:164-203` 类型＋常量＋方法＋字段＋默认值，`RequiredFeatures []Feature` 改 `Features` 位掩码）、`hal.InstanceDescriptor` 加 `XlibDisplay uintptr`＋`XlibScreen int32`（`descriptor.go:17` 超集，注释注X11专用）；`PollType` 两边逐字一致不动、`RequestAdapterOptions/DeviceDescriptor` 主字段已同构不动；`render` 64处调用点不动（`MapModeRead` 17＋`PollWait` 11＋`RequestAdapterOptions` 11＋`DeviceDescriptor` 12＋`InstanceDescriptor` 13，片7随方法签名一起换）；B0全绿：构建/vet净、触及文件`gofmt`净、`types/hal/noop`绿、H2B 7过、`gles`绿、`render/gpu`整包绿、带库66过1败（已知`TestS68 X11`同基线，绝对路径）、断言仍6处零新增；T5/T12仓内无命令（同片1–3b）；代码未提交（人工指令前不交）；收敛优化（2026-09-27：6行号跟正（`api.go` 405→421／456→475／529→548／468→487／418→436／426→442）＋`hal.DeviceDescriptor` 行号15→14＋3注释补口径（`hal.InstanceDescriptor` 补Wayland/Win/Metal走`SurfaceTarget`（`surface_target.go:19`）故实例无字段、`types.RequestAdapterOptions` 注uintptr裸柄、`types.InstanceDescriptor` 注X11连接在hal/webgpu、`types.MapMode` 补`map_types.go:8`）＋触及文件`gofmt`净＋B0复验全绿（带库仍66过1败同基线）） |
| 片5 | 绑定模型分歧（最难） | Entry 45＋Descriptor 33（`BindGroup` 63／`BindGroupLayout` 67／`PipelineLayout` 30 三类具体指针留片7随返回改接口一起换） | 已完·描述符先行（2026-09-27：问答定方向——hal 向 webgpu 看齐，以 hal 接口重述 webgpu 扁平形；改动：`hal` 新增 `BindGroupEntry` 接口形（`descriptor.go`，`Buffer/Sampler/TextureView` 收 hal 接口＋`Offset/Size`，`BindGroupDescriptor.Entries` 改 `[]BindGroupEntry`，gputypes 柄形注 H4 收敛）＋`webgpu` 删自家 `BindGroupDescriptor/Entry`（`descriptor.go` 留注释指 hal）＋`Device.CreateBindGroup` 改收 `*hal.BindGroupDescriptor`（`device.go:258`，`Layout` 内部解包＋`convertBindGroupEntry` 改收 hal 形内部解包具体柄）＋`R70` 跟改 `hal` 形＋`noop` 2测试跟改空 Entry＋`gles`（`resource.go/device.go/device_linux.go`）/`metal`（`resource.go/device.go`）加 `convertBindGroupEntries` 保绿垫片（hal 扁平经 `NativeHandle` 转内部柄形，runtime 不动，H4 收敛时移除）＋`render` 13文件78处换口（`Entry` 45＋`Descriptor` 33，另 `session_textured_cover.go` 去空 `webgpu` 引用1）；`browser` 零改动（独立 structs＋hal 在 wasm 下不可用，同片3a/3b先例；`browser-compute` 示例2处不动；`descriptor_browser.go:94/:101` 不动）；B0全绿：构建绿、vet（hal/webgpu）净、触及文件 `gofmt` 净、`hal/noop/types/gles` 绿、H2B 7过、R70 3过（`convertBindGroupEntry` 零申请保持）、`render/gpu` 整包绿、带库66过1败（已知 `TestS68 X11` 同基线；66过含 R70/H2B 在内）、断言仍6处零新增（render `(*webgpu.Texture)` 6处；gpu 内新增内部解包断言走既有 internal unpack 模式，不计入红线2）、账本48行1115处（基线1193－78＝1115对上）；加门：wasm vet（`GOOS=js GOARCH=wasm go vet ./gpu/webgpu/` 同干净树失败，`purego struct_other.go:21 syscallArgs`，与本批无关；`render/gpu` 在 wasm 下不可构建因 `hal` 无 wasm 文件，同前片先例）＋darwin metal 门（本机无 clang，`GOOS=darwin go build` 在 `runtime/cgo` 即止，与本批无关；metal 两文件改动经 `gofmt` 净＋同包 gles 逻辑对称，片6复验）；T5/T12仓内无命令同前片；代码未提交（等人工指令）） |
| 片6 | 错误哨兵统一 | 极少（`ErrorFilterValidation` 1＋`ErrDeviceLost` 1＋收敛3哨兵＋`GPUError/ErrorFilter`；终态零别名） | 已完（2026-09-27：问答定方向A（hal 为准统一）＋改动：`webgpu.ErrDeviceLost` 改 `hal.ErrDeviceLost` 别名（`error.go`，`browser` 侧不动，wasm 下 hal 不可用同前片先例）＋`render` 2处换口（`render_session.go:4150` 改 `hal.ErrorFilterValidation`、`shared_recovery.go:166` 改 `hal.ErrDeviceLost`＋补 `hal` 引用）；B0全绿：构建绿、vet（hal/webgpu）净、触及文件 `gofmt` 净、`hal/noop/types/gles` 绿、H2B 7过、R70 3过、`render/gpu` 整包绿、带库66过1败（已知 `TestS68 X11` 同基线）、断言仍6处零新增（render `(*webgpu.Texture)` 6处）、账本46行1113处（片5基线48行1115处－2行2处对上，两哨兵行清零）；加门：wasm vet（`purego struct_other.go:21 syscallArgs` 同干净树失败，与本批无关）＋darwin metal 门复验（本机无 clang，`runtime/cgo` 即止，与本批无关，本批 metal 未动）；错误链专项：`gate/surface/mapRWGPUErr/AutoRecover` 相关单测全过，无字符串断言（`errors.Is` 走通，唯一 `wgpu: device lost` 字面剩 `error_browser.go:19`）；T5/T12仓内无命令同前片（本批不碰管线，画面零影响）；收敛（2026-09-27）：`ErrSurfaceLost/SurfaceOutdated/Timeout` 3个改 `hal` 别名（`surface.go` 经 `errors.Is` 使用，无字符串断言，`surface_error_test` 全过；`browser` 不动；`ErrOutOfMemory` 名分两边不同暂不动；门面私有哨兵不动）；去别名（2026-09-27）：4哨兵别名定义删（`error.go` 留指向注释），`webgpu` 包内 `gate/surface/swapchain/device`＋自家2测试、`webgpu_test` 外部1测试共8文件直写 `hal.` 前缀，全仓 `webgpu.ErrDeviceLost/SurfaceLost/SurfaceOutdated/Timeout` 引用清零；复验全绿（构建/vet/gofmt净、`hal/noop/types/gles`绿、H2B 7过、R70 3过、`render/gpu`整包绿、带库66过1败同基线）；`GPUError/ErrorFilter` 别名（H2旧物）已去（2026-09-27，人工指令连签名一起换）：`error.go` 类型＋常量别名删（留指向注释，`hal` 引用摘除）、`device.go` 的 `convertErrorType` 签名改 `hal.ErrorFilter`＋`&hal.GPUError` 构造＋注释跟正；wasm 侧（`error_browser/device_browser`）独立不动；复验全绿（构建/vet/gofmt净、带库66过1败同基线）；代码未提交（等人工指令）） |
| 片7a | 参数侧先行（`MapMode→types`、`PollType→hal`、`DeviceDescriptor→hal`、`InstanceDescriptor→hal`别名＋`Device.Poll`收`hal.PollType`、`Buffer.Map/MapAsync`收`types.MapMode`、`Adapter.RequestDevice`收`hal.DeviceDescriptor`、`CreateInstance`收`hal.InstanceDescriptor`；`RequestAdapterOptions`留7f（`CompatibleSurface`需`Surface`先实现`hal.Surface`）；`Device/CommandEncoder`断言需`Create*`返回一起改留后片） | 53处（`MapModeRead`17＋`PollWait`11＋`DeviceDescriptor`12＋`InstanceDescriptor`13；`render`25文件，动手前复核） | 已完（2026-09-27：`webgpu`4别名＋4签名＋`render`25文件53处换口（9文件补`hal`引用）＋`browser`零改动；B0全绿：构建/vet/触及文件`gofmt`净、`hal/noop/types/gles`绿、H2B 7过、R70 3过、`render/gpu`整包绿、带库66过1败（已知`TestS68 X11`同基线）、断言6处零新增、账本42行1060处（基线46行1113－53对上，4行清零）；加门wasm/darwin同干净树失败与本批无关；T5/T12仓内无命令同片4-6；代码已交本地`0252e316`（人工指令，未推；收敛见§7收敛行，收敛代码未提交） |
| 片7b | 队列先行（根上先行）：`Device.Queue()`返回改`hal.Queue`＋`Backend.Queue()`及`render`封装`queue`字段/参数改`hal`形＋`render`跟改（`Device`157处不动，移7f随`Create*`返回一起换）；`Create*`返回本片不动（留7c–7e）；`var _ hal.Device`留7f（`Create*`未改齐前加断言编译不过，本片不加）；另补`Device`缺口桩（`hal.Device`多出约9方法：`CreateQuerySet/DestroyQuerySet/CreateRenderBundleEncoder/DestroyRenderBundle/AS`系5，`noop/gles`已有对称实现、`webgpu`缺，本片补桩：`CreateQuerySet`返`ErrTimestampsNotSupported`，其余返错/无操作零值，照`metal/gles`对称（`hal`无通用`ErrUnsupported`），否则7f断言不过；四家空实现按H1注记同批保绿） | 约76处＋改名＋9桩（`Queue`76约33文件；`Device`157约51文件不动，移7f随`Create*`返回一起换（`*webgpu.Device`尚不满足`hal.Device`，硬换编不过，问答定队列先行）；`Queue`特有方法`render`零调用已核：`OnSubmittedWorkDone`仅注释1处、`queue.Release`零调用，`Submit/WriteBuffer/WriteTexture/Poll/LastSubmissionIndex/Present`全是`hal.Queue`已有；`FreeCommandBuffer`5处（`hal`已有）不动） | 已完·队列先行（2026-09-27：问答定队列先行设备留后（`*webgpu.Queue`已有`var _ hal.Queue`断言可独立换，`*webgpu.Device`的`Create*`仍返具体指针不满足`hal.Device`硬换编不过）＋改动：`hal.Device.Destroy`改名`Release`（`api.go`＋`noop/gles×2/metal`同步＋`webgpu`删`Destroy`只留`Release`＋`noop/metal/hal`测试`Device.Destroy`跟改`Release`（`noop_test/swapchain_suppressed/bench/bench_cross/error`＋`metal texture_copy/surface/encoder`））＋`webgpu`补9缺口桩（`CreateQuerySet`返`ErrTimestampsNotSupported`＋`CreateRenderBundleEncoder/ CreateAccelerationStructure`返错＋`Destroy/尺寸/地址/字节`无操作零值，照`metal/gles`对称）＋`webgpu.Device.Queue()`返`hal.Queue`＋`device_provider.QueueToHandle`内部解包（`gpu`内既有unpack模式，不计红线2）＋`s2Device`返`hal.Queue`＋`render`33文件76处` *webgpu.Queue`换`hal.Queue`（2文件补`hal`引用：`commands/sparse_strips`）＋`Backend.Queue/GPUShared.Queue/getDeviceQueue`返`hal.Queue`；`browser`零改动（`queue_browser`独立不动）；B0全绿：构建绿、vet（hal/webgpu）净、触及文件`gofmt`净、`hal/noop/types/gles`绿、H2B 7过、R70 3过、`render/gpu`整包绿、带库66过1败（已知`TestS68 X11`同基线，绝对路径）、断言6处零新增（render `(*webgpu.Texture)` 6处）、账本41行984处（基线42行1060－`Queue`行76对上，`Device`157留7f）；加门wasm vet（`purego struct_other.go:21 syscallArgs`同干净树失败）＋darwin（本机无clang，`runtime/cgo`即止）与本批无关；T5/T12仓内无命令同前片；代码未提交（等人工指令）） |
| 片7c | 缓冲与采样器＋`Map`系：`CreateBuffer/CreateSampler`返回改`hal`接口＋补`var _ hal.Buffer`（`Sampler`断言H2已在；`Buffer.Size/Usage/Label/NativeHandle/Destroy`已齐，`Sampler/Shader`同理只嵌`Resource`故断言零方法改动）＋`Raw()`等封装跟改＋`Map`系56处（非测试12文件：`gpu_texture/vello/pattern_mask/mask_r8/linear_ramp/dual_tex/render_session/textured_linear/filter_graph/sdf_render/textured_pattern/stencil_renderer`＋测试3文件`buffer_test/damage_blit/p1_closers`，动手前现数复核）按H1对照表改`Device.MapBuffer/UnmapBuffer`（`Buffer.Map/MappedRange/Unmap/MapState/MapAsync`：`hal.Buffer`无此三法，`hal`保留`Map/Unmap`，删门面帮手，`render`直调`hal.Device`方法）；`Write/Copy`已`hal`形不动 | 约142处类型（`Buffer`116约18文件＋`Sampler`26约14文件）＋`Map`系56处（上段文件，动手前现数复核） | 已完（2026-09-27：问答定切法（类型142＋Map系56同批，`Map`经`device.MapBuffer/UnmapBuffer`＋`unsafe.Slice`拷，`webgpu`内部同实现对得上）＋改动：`webgpu`的`CreateBuffer/CreateSampler`返回改`hal.Buffer/hal.Sampler`（入参片3a已`hal`形；`var _ hal.Buffer/Sampler`已在零方法改动）＋`render`的`Buffer`包装（`gpuBuffer`字段/`NewBuffer`/`Raw()`）改`hal.Buffer`＋全仓类型142处换口（`*webgpu.Buffer→hal.Buffer`（含`vello_compute`的`bufSpec.target **hal.Buffer`＋`filter/dual-tex`的BG缓存key `ubuf`改存`hal.Buffer`接口值）／`*webgpu.Sampler→hal.Sampler`）＋`Release→Destroy`跟改（`hal`资源口只有`Destroy`，`webgpu`的`Destroy`即`Release`同义）＋`Map`系12非测试文件14站点改`device.MapBuffer/UnmapBuffer`（`MappedRange.Bytes()`拷改`unsafe.Slice`拷，`mapped.Release()`删，`device.Poll`等待保留，`context/types.MapModeRead`引用摘除）＋测试跟改（`s2_ae_smoke`3处`Map` trio＋`Release→Destroy`；`damage_blit`1站；`p1_closers`2站（`indBuf/staging Destroy`）；`pass_bind_ledger_test`换`noop.Buffer`；`submission_gate_test`换`Destroy`记录桩（`noop.Buffer`无`Released`）；`tex_shrink/opt27/depth_clip_test`换`noop.Buffer/Destroy`）；`browser`零改动；B0全绿：构建绿、vet（hal/webgpu）净（`render`整包vet仅存量`res/view_test.go unsafe.Pointer`提示）、触及文件`gofmt`净、`hal/noop/types/gles`绿、H2B 7过、R70 3过、`render/gpu`整包绿、带库66过1败（已知`TestS68 X11`同基线，绝对路径）、断言6处零新增（仍`(*webgpu.Texture)` 6处，归7d）、账本39行842处（基线41行984－`Buffer`116－`Sampler`26对上，两类型行清零）；加门wasm vet（`purego struct_other.go:21 syscallArgs`同干净树失败）＋darwin（未跑，同前片口径）与本批无关；T5/T12仓内无命令同前片（`MapBuffer`内部即`Map+MappedRange`同语义，CPU Golden子集2过）；代码未提交（等人工指令）） |
| 片7d | 纹理与视图：`CreateTexture/CreateTextureView`返回改`hal`接口（`CreateTextureView`入参改`hal.Texture`）＋补`var _ hal.Texture/TextureView`（`Format/CurrentUsage/PendingRef/NativeHandle/Texture()`已齐，断言零方法改动）＋`SurfaceTexture`3方法返回改`hal`（`surface.go:365 AsTexture/368 CreateView/396 Texture()`返具体，`SurfaceTexture`断言H2已在）＋`render`跟改＋红线2收口（6处`.(*webgpu.Texture)`清零：`layer_gpu_io.go:93`、`gpu_render_context.go:3020/3245/3296`、`dual_tex_blend.go:1715/1836`，以动手前现查为准） | 约240处（`Texture`86约32文件＋`TextureView`154约39文件（两类并集40文件，动手前现数复核） | 已完（2026-09-27：问答定标准切法一次清零＋`webgpu`的`CreateTexture/CreateTextureView`返回改`hal`（`CreateTextureView`入参改`hal.Texture`内部解包，`swapchain`内部解包＋`Destroy`跟改）＋`SurfaceTexture`3方法（`AsTexture/CreateView/Texture`）返回改`hal`＋`render`核心类型全换`hal`（`texture/gpu_textures/gpu_texture/command_encoder/gpu_types/render_session/gpu_render_context/layer_gpu_io/dual_tex/filter_graph/brush_advanced/gpu_shared/depth_clip/mask/text/image/glyph/sdf/stencil/textured/linear/pattern/cover/present_target`）＋`view_handle.go`新建（`packView/unpackView/unpackRawView`，Pointer透传改装hal holder）＋`dualTexBGKey/filterBGKey`改存`hal.TextureView`值（同片7c BG缓存存hal值先例）＋6断言清零＋`Release→Destroy`跟改（`cmdBuf/bg/pipeline`等7e/7f范围不动）＋测试跟改（`testTexture/testTextureView`记录桩（`submission_gate`，同片7c记录桩先例）＋6测试文件`packView`＋`retire/attack`过期引用跟改；`browser`零改动；B0全绿：构建/vet（hal/webgpu/gpu净，`render`整包仅3存量提示）/触及文件`gofmt`净、基测全绿、H2B 7过、R70 3过、`render/gpu`整包绿、带库66过1败（已知`TestS68 X11`同基线，绝对路径）、断言零残留（`.(*webgpu`全仓清零）、账本37行602处（基线39行842－`Texture`86－`TextureView`154对上，两类型行清零）；加门wasm vet（`purego struct_other.go:21 syscallArgs`同干净树失败）＋darwin（本机无clang，`runtime/cgo`即止）与本批无关；T5/T12仓内无命令同前片；代码未提交（等人工指令）） |
| 片7e | 管线与绑定：`CreateShaderModule/CreateRenderPipeline/CreateComputePipeline/CreateBindGroupLayout/CreatePipelineLayout`返回改`hal`接口（`CreateBindGroup`描述符片5已`hal`形，返回改`hal`）＋补6断言`var _ hal.ShaderModule/RenderPipeline/ComputePipeline/BindGroup/BindGroupLayout/PipelineLayout`（6家只嵌`Resource`（`Destroy`已有），断言零方法改动）＋`render`跟改 | 约292处（`ShaderModule`35约20文件＋`RenderPipeline`83约19文件＋`ComputePipeline`14约7文件＋`BindGroup`63约29文件＋`BindGroupLayout`67约22文件＋`PipelineLayout`30约19文件，动手前现数复核） | 已完（2026-09-27：问答定标准切法一次清零＋`webgpu`的6个Create返回改`hal`（`device.go:165/192/223/261/300/322`；6断言`shader.go:32`单行＋`bind.go:84-86`/`pipeline.go:61-62` var块，H2/片5已在零方法改动）＋`render`36文件292处换口（非测试30＋测试6：`depth_clip/opt27/render_session/bg_cache/glyphmask/pass_bind_ledger`；`*webgpu→hal`，3文件补`hal`引用：`compute_pass/pipeline_cache_core/render_pass`；`pass_bind_ledger`字段/签名全`hal`形；本地包装器`compute_pass.ComputePipeline`/`render_pass.RenderPipeline/BindGroup`/`pipeline_cache_core.ShaderModule`/`shader_helper.GPUResources`字段与`Raw()`同步改`hal`形）＋`Release→Destroy`跟改415处（`git diff`增删对上；非测试30文件＋`s2_ae_smoke`2处＋测试`depth_clip/opt27/render_session`6处`layout/bg`）＋测试跟改（`pass_bind_ledger_test`换`noop.Buffer`双实例（`noop.Resource`同地址复用致管线切换不断言失败，同片7c记录桩先例）＋`tex_shrink`换`noop.Buffer`＋`bg_cache/opt27`去`webgpu`空引用；`browser`零改动；B0全绿：构建绿、vet（hal/webgpu）净、触及文件`gofmt`净、`hal/noop/types/gles`绿、H2B 7过、R70 3过、`render/gpu`整包绿、带库66过1败（已知`TestS68 X11`同基线，绝对路径）、断言零残留（`.(*webgpu`全仓清零）、账本31行310处（基线37行602－6行292对上，6类型行清零；`RenderPipeline/BindGroup`无残留，前稿“各2处”系无边界子串误数，以现查为准）；加门wasm vet（`purego struct_other.go:21 syscallArgs`同干净树失败）与本批无关；T5/T12仓内无命令同前片（本批不碰管线描述符只换持有类型，画面零影响）；收敛（2026-09-27：8注释`underlying WebGPU→hal`口径（`compute_pass`3＋`render_pass`5）＋3占位注释改包装器口径（`compute_pass.ComputePipeline`＋`render_pass.RenderPipeline/BindGroup`）＋触及文件`gofmt`净＋B0复验全绿（带库仍66过1败同基线）；收敛代码未提交）；代码未提交（等人工指令）） |
| 片7f | 编码器/通道/创建入口收尾：`BeginRenderPass/BeginComputePass`返回改`hal`接口＋补`var _ hal.CommandEncoder`（`CommandBuffer/RenderPass/ComputePass`断言H2已在）＋`Finish`去向动手前定（`hal`无`Finish`只有`EndEncoding`（`RenderBundle.Finish`是另一物），`render`调`Finish`40处非测试（`command_encoder/gpu_texture/vello/pattern/mask/renderer/linear/commands/filter_flush/gpu_render_context/dual_tex`等，动手前现数复核），二选一（`hal`补`Finish` OR `render`改`EndEncoding`，按§S（`hal`以`webgpu`为准）＋H1编码器行（`Finish`按`webgpu`补进`hal`）倾向`hal`补，定完全量改）＋`Surface`形状重写（`hal`：`Configure(device,config)/Unconfigure(device)/AcquireTexture(fence)/DiscardTexture(texture)/ActualExtent`；`webgpu`：`Configure(*Device)/Unconfigure()/GetCurrentTexture/Present/PresentWithDamage/WindowSize/SetPrepareFrame`＋`SurfaceTexture`3方法已在7d；`GetCurrentTexture/Present/PresentWithDamage/WindowSize/SetPrepareFrame`注明去向（留门面/删/转接`Acquire+Queue.Present`，`render`经`Swapchain`调用链（`present_target/adapter_policy`）跟改，动手前定））＋`Adapter`补桩（`Open/TextureFormatCapabilities/Destroy`，`hal`有`webgpu`无，`noop/gles`对称，补`ErrUnsupported`桩）＋`Instance`补桩（`CreateSurface/EnumerateAdapters/Destroy`，同上）＋`Adapter.GetSurfaceCapabilities`入参加`hal`＋`Instance.RequestAdapter`收`hal.RequestAdapterOptions`返`hal.Adapter`＋补`var _ hal.Device/Surface/Adapter/Instance`（7b桩＋7c–7e返回齐后本片补）＋`Device`157处约51文件换`hal`形（7b移入：`Backend.Device/GPUShared.Device/createDevice/requestDeviceWithRetry/getDeviceQueue`及`render`封装`device`字段/参数随`Create*`返回一起换）＋`render`跟改＋收尾（7a的4个别名删（`MapMode/PollType/DeviceDescriptor/InstanceDescriptor`）、`RequestAdapterOptions`11处换口、账本清零、无感知验收：`render`无`webgpu.`具体类型（创建入口除外）、断言零残留、别名零残留） | 约261处＋收尾（`Device`157约51文件（7b移入）＋`CommandEncoder`14约5文件＋`CommandBuffer`21约8文件＋`RenderPassEncoder`23约12文件＋`ComputePassEncoder`1＋`Adapter`21约6文件＋`Instance`10约12文件＋`RequestAdapterOptions`11约9文件＋`Surface`3约3文件＋4个别名删，动手前现数复核；`CreateInstance`函数13处不做，入口形态H3/H4另议；总量验算：7b76＋7c142＋7d240＋7e292＋7f261＝1011对上） | 未开始（最后做；前置7b–7e；保绿同7b；B0同基线＋收尾验收；代码不交，等人工指令） |
| — | 不进 hal（留 `webgpu`，不动）：`SimpleDeviceProvider` 6、`NewSwapchain` 6、`Swapchain` 2、`DeviceFromHandle` 2、`AdapterFromHandle` 1、`AdapterInfo` 1、`Vram*` 9（`Budget`3＋`Live`2＋`TestReset/Add/Peak/LiveCount`各1）、`SetLogger` 1、`BeforeDeviceRecover` 1、`AfterSurfaceUnconfigure` 1、`SurfaceBackend*` 5（4平台＋类型各1）、`CreateInstance` 13（入口形态 H3/H4 另议）、`TestSwapchain_WindowPresentE2E` 1 | 49处（`CreateInstance`13＋`Simple`6＋`NewSwapchain`6＋`Swapchain`2＋`Vram`9＋`DeviceFrom`2＋`AdapterFrom/Info`各1＋`SetLogger/Before/After`各1＋`SurfaceBackend`5＋`Test`1；`Surface`3归7f，不在此列） | 不做 |

### 两条红线（出处＋现状，2026-09-26 记清）

- 红线1·代码提交只认人工指令：出自§S第18行（2026-09-27 起 "代码永远不提交" 作废，改为 "代码提交只认人工指令（人工说交才交，不主动交），文档按需提交"）＋AGENTS.md git 提交纪律（只交本次文件、提交/推送先确认）；历史：2026-09-26 前代码零改动（HEAD `38e97481`，工作区干净）；现状：片1–片3b代码均已按人工指令交本地（片1 `88d26090`、片2 `a9455c73`、片3a `775323d5`、片3b文档 `b3a18d93`＋代码 `04c4ccee`），未推；此后仍只认人工指令，不主动交、不主动推。
- 红线2·类型断言零残留：出自§S第10行（"不许类型猜测"）＋§5第211行无感知验收（"`hal` 口相关类型猜测零残留"）；现状：过渡 `.(*webgpu.Texture)` 现查 6 处——`layer_gpu_io.go:92`、`gpu_render_context.go:3019/3244/3295`、`dual_tex_blend.go:1714/1835`（H2-a 行记的"5 处"是当时数，现查 6 处，以现查为准），片7收官时清。注：第 207 行"像素断言"是测试断言画面对，必须有，与此无关。

### 附账本：97 行机器输出原文（数以此为准，分组只是索引）

> 拉取原文，一字未改，共 97 行 2329 处；数以此为准，分组只是索引（约数已对账本验算：片1约305／片2约190／片3约650／不做约50）。
>
> 拉取命令（干净树可复现）：`grep -rho "webgpu\.[A-Za-z0-9_]*" render/ --include="*.go" | sort | uniq -c | sort -rn`。2026-09-27 在干净树重跑，97 行 2329 处与下表一字不差。

```
    157 webgpu.Device
    154 webgpu.TextureView
    123 webgpu.Extent3D
    116 webgpu.Buffer
     96 webgpu.BindGroupLayout
     86 webgpu.Texture
     83 webgpu.StencilOperationKeep
     83 webgpu.RenderPipeline
     76 webgpu.Queue
     73 webgpu.TextureDescriptor
     71 webgpu.BufferDescriptor
     63 webgpu.BindGroup
     62 webgpu.TextureViewDescriptor
     57 webgpu.ImageCopyTexture
     53 webgpu.ImageDataLayout
     48 webgpu.VertexState
     48 webgpu.RenderPipelineDescriptor
     48 webgpu.FragmentState
     45 webgpu.BindGroupEntry
     38 webgpu.StencilFaceState
     38 webgpu.RenderPassColorAttachment
     36 webgpu.BindGroupLayoutDescriptor
     35 webgpu.ShaderModule
     34 webgpu.ShaderModuleDescriptor
     34 webgpu.CommandEncoderDescriptor
     33 webgpu.BindGroupDescriptor
     30 webgpu.PipelineLayout
     29 webgpu.RenderPassDescriptor
     29 webgpu.PipelineLayoutDescriptor
     26 webgpu.Sampler
     23 webgpu.RenderPassEncoder
     21 webgpu.DepthStencilState
     21 webgpu.CommandBuffer
     21 webgpu.Adapter
     20 webgpu.SamplerDescriptor
     18 webgpu.BufferTextureCopy
     17 webgpu.StencilOperationZero
     17 webgpu.MapModeRead
     14 webgpu.Origin3D
     14 webgpu.ComputePipeline
     14 webgpu.CommandEncoder
     13 webgpu.InstanceDescriptor
     13 webgpu.CreateInstance
     12 webgpu.DeviceDescriptor
     11 webgpu.StencilOperationIncrementWrap
     11 webgpu.RequestAdapterOptions
     11 webgpu.PollWait
     10 webgpu.RenderPassDepthStencilAttachment
     10 webgpu.Instance
     10 webgpu.ComputePipelineDescriptor
      9 webgpu.TextureUsageTransition
      9 webgpu.TextureBarrier
      8 webgpu.PowerPreferenceHighPerformance
      8 webgpu.BackendsPrimary
      6 webgpu.SimpleDeviceProvider
      6 webgpu.PowerPreferenceLowPower
      6 webgpu.NewSwapchain
      5 webgpu.StencilOperationDecrementWrap
      4 webgpu.TextureFormatRGBA8Unorm
      4 webgpu.BufferUsageCopyDst
      3 webgpu.VramBudgetMB
      3 webgpu.TextureUsageRenderAttachment
      3 webgpu.TextureUsageCopySrc
      3 webgpu.Surface
      3 webgpu.PowerPreferenceNone
      3 webgpu.DefaultLimits
      3 webgpu.BufferUsageMapRead
      2 webgpu.VramLiveBytes
      2 webgpu.TextureDimension2D
      2 webgpu.TextureCopy
      2 webgpu.Swapchain
      2 webgpu.StencilOperationReplace
      2 webgpu.DeviceFromHandle
      2 webgpu.ComputePassDescriptor
      2 webgpu.BackendsVulkan
      1 webgpu.VramTestReset
      1 webgpu.VramTestAdd
      1 webgpu.VramPeakBytes
      1 webgpu.VramLiveCount
      1 webgpu.TestSwapchain_WindowPresentE2E
      1 webgpu.SurfaceBackendXlib
      1 webgpu.SurfaceBackendWin32
      1 webgpu.SurfaceBackendWayland
      1 webgpu.SurfaceBackendMetal
      1 webgpu.SurfaceBackend
      1 webgpu.StencilOperation
      1 webgpu.SetLogger
      1 webgpu.PowerPreference
      1 webgpu.ErrorFilterValidation
      1 webgpu.ErrDeviceLost
      1 webgpu.ComputePassEncoder
      1 webgpu.BufferUsageIndirect
      1 webgpu.BeforeDeviceRecover
      1 webgpu.BackendsMetal
      1 webgpu.AfterSurfaceUnconfigure
      1 webgpu.AdapterInfo
      1 webgpu.AdapterFromHandle
```

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
| 2026-09-26 | 历史合账：立项→P0（断根三条，EGL 过）→方向重置（hal 以 render 用的 webgpu 为准，四步 hal→webgpu→render→gwgpu）→H1（hal 对齐：Device/Queue/Encoder/资源查询/创建入口/描述符，四家空实现同批保绿）→H2（webgpu 参数侧收官：H2-a 资源 12 件套，H2-b1–b19 别名/围栏/队列/拷贝链，H2B 7 单测；改判：返回侧移 H3 即分片片7）→分片建档（97 符号 2329 处，7 片，见附表；账本命令与验算见附账本；红线见两条红线节；片3 拆 3a/3b）。门禁基线：带库 59 过 1 败（`TestS68 X11` 存量用例）＋T5/T12 零差。明细见备份（`/tmp/opencode`、`git 历史`）。 |
| 2026-09-27 | 片1关闭：7纯数据结构 `render` 305处换 `hal` 口＋7别名删（附带 `webgpu` 包内裸用5文件＋自家测试1处）。门禁见片1状态格（带库66过1败同基线；T5离屏200轮过；T12仓内无对应命令，待后片定位离屏md5做法）。代码已交本地`88d26090`（人工指令，未推）。 |
| 2026-09-27 | 片2关闭：21枚举 `render` 20文件171处换 `types` 口＋`webgpu`别名删（`Stencil`全组＋`Backends`6/`BufferUsage`10/`TextureUsage`5/`Dimension`3/`Format`6/`Power`组＋`DefaultLimits`，`hal`不动）＋`webgpu`包内跟改（浏览器3＋自家3＋外部4）；`MapModeRead` 17处落位片4（`types`与`webgpu`各自另定，直换不过）。门禁见片2状态格（带库66过1败同基线；T5离屏200轮过；`render`整包`TestS69`系干净树同败与本批无关；T12同片1无命令；账本69行1853处）。收敛优化：4注释行号跟正＋触及文件`gofmt`净＋B0复验绿。代码已交本地`a9455c73`（人工指令，未推）。 |
| 2026-09-27 | 片3a关闭：14纯别名描述符 `render` 64文件448处换 `hal` 口＋14别名删。门禁见片3a状态格（带库66过1败同基线；T5离屏200轮过；`render/gpu`整包`TestOpt21`系干净树同败与本批无关；T12同片1无命令；账本55行1405处）。数量纠正：`CommandEncoder` 20→`CommandEncoderDescriptor` 34，约434→约448。代码已交本地`775323d5`（人工指令，未推）。 |
| 2026-09-27 | 片3b关闭：5自有结构体 `render` 22文件183处换 `hal` 口＋`BindGroupLayouts` 29处跟改＋`webgpu` 5旧结构体删＋3个Create收 `hal` 描述符＋6转换函数跟改＋自家测试4文件跟改（`browser`零改动，`browser-compute`示例2处不动）。门禁见片3b状态格（带库66过1败同基线；T5 stress过；Golden 5过；`render/gpu`整包绿；`render`整包6败与干净树一字相同与本批无关；T12同片1无命令；账本50行1193处）。代码已交本地`04c4ccee`（人工指令，未推）。 |
| 2026-09-27 | 片3b收敛优化：3注释跟正（`pipeline_cache_core.go:706`漏网、`device.go:781` our→hal、片3a遗留`texture.go:548`顺手收敛）＋1行号（`hal/descriptor.go:381` :170→:172字段行）；复验：构建绿、vet（hal/webgpu）净、触及文件gofmt净、基测全绿、H2B/转换单测过、`render/gpu`整包绿；其余口径（账本50行1193处、断言6处）不变。代码已交本地`04c4ccee`（人工指令，未推）。 |
| 2026-09-27 | 片4关闭·形状对齐（调用点留片7）：三处问答定方向（全换 `hal` 口／`CompatibleSurface` 跟 `webgpu` 但 `hal` 留接口形待片7／`MemoryHints` 删＋`Xlib` 合并进 `hal`）＋真耦合（51处调用点挂片7返回侧签名，直换等于提前做片7，故只对齐形状）；改动：`types.MapMode` 去 `None`、`types.DeviceDescriptor` 去 `MemoryHints`（`RequiredFeatures` 改位掩码）、`hal.InstanceDescriptor` 加 `XlibDisplay/Screen`；门禁见片4状态格（构建/vet/gofmt净、`types/hal/noop/gles`绿、H2B 7过、`render/gpu`整包绿、带库66过1败同基线、断言6处；T5/T12仓内无命令同前片）。代码未提交（等人工指令）。 |
| 2026-09-27 | 片4收敛优化：6行号跟正（`hal/api.go` 405→421／456→475／529→548／468→487／418→436／426→442、`hal/descriptor.go` DeviceDescriptor 15→14）＋3注释补口径（`hal.InstanceDescriptor` 补Wayland/Win/Metal走 `SurfaceTarget`（`surface_target.go:19`）故实例无字段、`types.RequestAdapterOptions/InstanceDescriptor` 注三形态分工、`types.MapMode` 补 `map_types.go:8`）；复验：构建绿、vet净、触及文件`gofmt`净、基测全绿、H2B 7过、`gles`绿、`render/gpu`整包绿、带库66过1败同基线、断言6处、账本64处（5符号）不变。代码未提交（等人工指令）。 |
| 2026-09-27 | 片7单子补齐：片4转入64处（`MapModeRead` 17＋`PollWait` 11＋`RequestAdapterOptions` 11＋`DeviceDescriptor` 12＋`InstanceDescriptor` 13）连文件单子与先改签名顺序一并写进片7状态格，动手前按单子现数复核。只改文档，代码不动，未提交。 |
| 2026-09-27 | 片5关闭·描述符先行（`BindGroup` 63／`BindGroupLayout` 67／`PipelineLayout` 30 三类具体指针留片7，见片5状态格）：`hal` 新增 `BindGroupEntry` 接口形＋`webgpu` 删自家定义＋`CreateBindGroup` 改收 hal 形＋`render` 13文件78处换口＋`gles/metal` 保绿垫片；门禁见片5状态格（构建/vet/gofmt净、`hal/noop/types/gles`绿、H2B 7过、R70 3过、`render/gpu`整包绿、带库66过1败同基线、断言6处（render `(*webgpu.Texture)` 6处）、账本48行1115处；wasm vet 与 darwin metal 门因本机工具链缺失同干净树失败，片6复验；T5/T12仓内无命令同前片）。代码未提交（等人工指令）。 |
| 2026-09-27 | 片5收敛优化：1悬空注释跟正（`hal/descriptor.go:295` Mirrors 已删的 `webgpu BindGroupEntry`→改 canonical 口径（片5以旧 webgpu 扁平形为准、类型提取为 hal 接口，贴合 hal 以 webgpu 为准目标））＋2数字口径（账本50行→48行（1115处不变，删 `Entry/Descriptor` 2行对上）＋`descriptor_browser.go:101`→`:94/:101`（`Descriptor:94/Entry:101`））＋1断言口径（6处限定 render `(*webgpu.Texture)`，gpu 内新增内部解包走既有 internal unpack 模式不计入红线2）；复验：构建绿、vet（hal/webgpu）净、触及文件`gofmt`净、基测全绿、H2B 7过、R70 3过、`render/gpu`整包绿、带库66过1败同基线；其余口径不变。代码未提交（等人工指令，源码注释1处同批改，待交）。 |
| 2026-09-27 | 片6关闭·错误哨兵统一（`ErrorFilterValidation` 1＋`ErrDeviceLost` 1，问答定方向A：hal 为准）：`webgpu.ErrDeviceLost` 改 `hal` 别名＋`render` 2处换 `hal` 口；门禁见片6状态格（构建/vet/gofmt净、`hal/noop/types/gles`绿、H2B 7过、R70 3过、`render/gpu`整包绿、带库66过1败同基线、断言6处、账本46行1113处；wasm vet 与 darwin metal 门因本机工具链缺失同干净树失败；T5/T12仓内无命令同前片）。代码未提交（等人工指令）。 |
| 2026-09-27 | 片6收敛·通用哨兵追加（问答定只收敛重名通用哨兵）：`webgpu` 的 `ErrSurfaceLost/SurfaceOutdated/Timeout` 3个改 `hal` 别名（`surface.go` 全经 `errors.Is` 使用，无字符串断言，`surface_error_test`＋带库66过1败同基线全过；`browser` 侧不动；`ErrOutOfMemory` 两边名字不同暂不动；帧配对/提交/绘制校验等门面私有哨兵不动）。代码未提交（等人工指令）。 |
| 2026-09-27 | 片6去别名（人工指令：不用别名、直接 `包.符号`）：4哨兵别名定义删（`error.go` 留3条指向注释），`gate/surface/swapchain/device`＋`gate/surface_error` 自家测试＋`auto_recover` 外部测试共8文件直写 `hal.` 前缀（含 `hal` 引用3处补），全仓 `webgpu.ErrDeviceLost/SurfaceLost/SurfaceOutdated/Timeout` 引用清零；复验：构建绿、vet（hal/webgpu）净、触及文件`gofmt`净、基测全绿、错误链专项过、H2B 7过、R70 3过、`render/gpu`整包绿、带库66过1败同基线；`GPUError/ErrorFilter` 别名（H2旧物，`PushErrorScope` 签名牵连＋wasm 另定，动则单独立项）未动。代码未提交（等人工指令）。 |
| 2026-09-27 | 片6去别名2（人工指令连签名一起换）：`GPUError/ErrorFilter`（含3常量）别名删（`error.go` 留指向注释＋摘 `hal` 引用），`device.go` 的 `convertErrorType` 签名/返回加 `hal.` 前缀＋`&hal.GPUError` 构造＋3注释跟正；`PushErrorScope/PopErrorScope` 签名本就是 `hal` 形，零改动；wasm 侧独立不动；全仓 `webgpu.GPUError/ErrorFilter` 引用清零；复验：构建/vet/gofmt净、基测全绿、专项18过、带库66过1败同基线。代码未提交（等人工指令）。 |
| 2026-09-27 | 片6收敛优化：1注释口径（`gate.go:50` to webgpu public errors→to hal public errors）＋1行号（`shared_recovery.go:165→166`，补 `hal` 引用占一行）＋1数量口径（状态格“自家3测试”→“自家2测试”（`gate/surface_error` 自家＋`auto_recover` 外部，§7行原本就对）＋规模格改终态口径（零别名）；扫残留：`encoder/queue` 的 aliased 字眼系描述符旧别名（非错误哨兵，不属本片）、全仓 `webgpu.Err*/Error*` 零残留（门面私有哨兵除外）、非计划文档零现行引用；复验：构建/vet/触及文件`gofmt`净、基测全绿、`render/gpu`整包绿、带库66过1败同基线、账本46行1113处不变。代码未提交（等人工指令）。 |
| 2026-09-27 | 片7a关闭·参数侧先行：`webgpu`的`MapMode/PollType/DeviceDescriptor/InstanceDescriptor`改`types/hal`别名＋4个方法签名收`hal/types`形＋`render`25文件53处换口（`MapModeRead`17、`PollWait`11、`DeviceDescriptor`12、`InstanceDescriptor`13；`RequestAdapterOptions`11处留7f）。门禁见片7状态格（构建/vet/gofmt净、基测全绿、H2B 7过、R70 3过、`render/gpu`整包绿、带库66过1败同基线、断言6处、账本42行1060处；wasm/darwin两加门同干净树失败与本批无关；T5/T12仓内无命令同片4-6）。代码未提交（等人工指令）。 |
| 2026-09-27 | 片7细化：单片7拆7a–7f（7a已完53处见片7a状态格；余量1011处＝7b设备队列233＋7c缓冲采样142＋7d纹理视图240（含6断言清零）＋7e管线绑定292＋7f编码通道创建入口104＋收尾；H3改进行中；顺序7b→7c→7d→7e→7f，每片保绿（`webgpu`签名＋`render`跟改同批）、`browser`零改动、B0同基线；`CreateInstance`函数13处仍不做；只改文档，代码不动，未提交）。 |
| 2026-09-27 | 片7完整性复核（只改文档，代码不动，未提交）：①量级：`render`现数42行1060处＝7b233＋7c142＋7d240＋7e292＋7f104（＝1011）＋不做49（`CreateInstance`13＋`Simple`6＋`NewSwap`6＋`Swapchain`2＋`Vram`9＋`DeviceFrom`2＋`AdapterFrom/Info`各1＋`SetLogger/Before/After`各1＋`SurfaceBackend`5＋`Test`1；不做行修正：去`Surface`3（归7f）、`Vram*`7→9）；②7b补`Device`缺口桩（`QuerySet/RenderBundle/AS`约9方法，`noop/gles`已有，`webgpu`缺）＋`var _ hal.Device`移7f（`Create*`未齐前加断言不过）＋`Queue/Device`特有方法`render`零调用已核（仅`FreeCommandBuffer`5处不动）；③7c重写（原“`Map`不动”错）：`Map`系56处非测试12文件＋测试3文件按H1对照表改`Device.MapBuffer/UnmapBuffer`，为主工作；④7d加`SurfaceTexture`3方法（`surface.go:365/368/396`返具体改`hal`）；⑤7e注6断言零方法改动；⑥7f重写（`Finish`40处去向动手前定：`hal`无`Finish`只有`EndEncoding`，二选一；`Surface`形状对照重写（`hal`有`Acquire`无`GetCurrent/Present`，5个多余方法注明去向，`render`经`Swapchain`链跟改）；`Adapter/Instance`补`Open/Caps/Destroy`与`CreateSurface/Enumerate/Destroy`桩）；顺序7b→7c→7d→7e→7f不变。 |
| 2026-09-27 | 片7a收敛优化：4注释跟正（`map_types.go`2处＋`adapter.go`/`instance.go`各1处：`7c收尾→7f收尾`，别名删归7f见片7f状态格）＋自家测试4文件9处直写`hal`（`auto_recover_depth_test`2＋`recover_vram_test`3＋`swapchain_test`1＋`swapchain_x11`3；`swapchain_test/swapchain_x11`补`hal`引用各1，`browser-compute`示例1处不动（wasm下`hal`不可用））＋文档7a状态补已交`0252e316`；复验：构建/vet/触及文件`gofmt`净、基测全绿、H2B 7过、R70 3过、`render/gpu`整包绿、带库66过1败同基线、断言6处、账本42行1060处不变。代码未提交（等人工指令）。 |
| 2026-09-27 | 片7b关闭·队列先行（问答定切法：`*webgpu.Queue`已有`var _ hal.Queue`可独立换，`*webgpu.Device`的`Create*`仍返具体指针不满足`hal.Device`硬换编不过，故`Device`157处约51文件移7f随返回一起换；总量验算76＋142＋240＋292＋261＝1011对上）：改动见片7b状态格（`Destroy→Release`改名＋9桩＋`Queue()`返`hal.Queue`＋`render`33文件76处换口）；门禁见片7b状态格（B0全绿，带库66过1败同基线，断言6处，账本41行984处；wasm/darwin加门同干净树失败与本批无关；T5/T12仓内无命令）。代码未提交（等人工指令）。 |
| 2026-09-27 | 片7b收敛优化：1测试改名（`TestNoopDeviceDestroy→TestNoopDeviceRelease`，注释7b已改名函数名漏网）＋1遗留注释（`gles device_linux.go:851` Destroy→Release，`device.go`同批已改）＋2共享注释（`gpu_shared.go:154/338` Device.Destroy→Release，触及文件顺手收敛；`recover_vram_test/safety`两处系历史叙事不动）＋11行号跟正（`hal/api.go`：`device.go` 7处（接入27,32,37→30,35,40、`WaitForFence`421→426、`Poll`475→481、`IsLost`548→531、`FlushCallbacks`487→493、`PushErrorScope`436→441、`PopErrorScope`442→450，`Destroy`删除移位所致）＋`queue.go` 3处（Submit 40→44、Poll 122→138、LastSubmission 195→222）＋`map_types.go`26→37（7a别名化所致））＋文档内容格2处（标题设备与队列→队列先行、`Backend.Device()`去名、`ErrUnsupported`改实际口径（`hal`无通用错，`QuerySet`返`ErrTimestampsNotSupported`其余返错/零值））＋1括号嵌套简化；复验：构建/vet/触及文件`gofmt`净、基测全绿、H2B 7过、R70 3过、`render/gpu`整包绿、带库66过1败同基线、断言6处、账本41行984处不变；遗留1处非本批旧行号（`api.go:423` queue.go:620-634，真目标待查，留后片认领）。代码未提交（等人工指令）。 |
| 2026-09-27 | 片7c关闭·缓冲采样＋Map系（问答定类型142＋Map56同批）：`webgpu`的`CreateBuffer/CreateSampler`返回改`hal`＋`render`的`Buffer`包装改`hal.Buffer`＋全仓142处换口＋`Release→Destroy`跟改＋`Map`系14站点改`device.MapBuffer/UnmapBuffer`＋测试8文件跟改；门禁见片7c状态格（构建/vet/gofmt净、基测全绿、H2B 7过、R70 3过、`render/gpu`整包绿、带库66过1败同基线、断言6处、账本39行842处；wasm/darwin加门同干净树失败与本批无关；T5/T12仓内无命令同前片）。代码未提交（等人工指令）。 |
| 2026-09-27 | 片7c收敛优化：2过时注释跟正（`render/buffer.go:130` wraps `*wgpu.Buffer`→`hal.Buffer`、`buffer_test.go:15`同口径）＋7b遗留认领（`hal/api.go:423`的`queue.go:620-634`不存在：`queue.go`全文件263行、`WriteTexture`（`:167`）内无save/restore、`VK-004`全仓仅此一处，改指向`webgpu Queue.WriteTexture`不带编号行号）＋残留口径（全仓`webgpu.Buffer/Sampler`剩14处，全在`browser-compute/browser-test`示例（wasm下`hal`不可用），同片3a/3b/5先例不动）＋数量复核（`Buffer`116＝18文件分项加总对上、`Sampler`26＝14文件分项加总对上）；复验：构建/vet（hal/webgpu）净、触及文件`gofmt`净、`render/gpu`整包绿、H2B 7过、R70 3过、带库66过1败同基线、断言6处、账本39行842处不变。代码未提交（等人工指令）。 |
| 2026-09-27 | 片7d关闭·纹理视图（问答定标准切法一次清零）：`webgpu`的`CreateTexture/CreateTextureView`返回改`hal`（`CreateTextureView`入参改`hal.Texture`内部解包，`swapchain`内部解包＋`Destroy`跟改）＋`SurfaceTexture`3方法（`surface.go:365 AsTexture/368 CreateView/396 Texture`）返回改`hal`（`var _ hal.Texture/TextureView/SurfaceTexture`已在，零方法改动）＋`render`核心类型全换`hal`（`texture/gpu_textures/gpu_texture/command_encoder/gpu_types/render_session/gpu_render_context/layer_gpu_io/dual_tex/filter_graph/brush_advanced/gpu_shared/depth_clip/mask/text/image/glyph/sdf/stencil/textured/linear/pattern/cover/present_target`，`Device/CommandEncoder`等7e/7f范围不动）＋`view_handle.go`新建（`packView/unpackView/unpackRawView`，Pointer透传改装hal holder，`res.View.Raw`零改动）＋`dualTexBGKey/filterBGKey`改存`hal.TextureView`值（同片7c BG缓存存hal值先例）＋红线2收口（6处`.(*webgpu.Texture)`清零，全仓`.(*webgpu`零残留）＋`Release→Destroy`跟改（`cmdBuf/bg/pipeline/shader`等7e/7f范围不动）＋测试跟改（`testTexture/testTextureView`记录桩（`submission_gate`，同片7c记录桩先例）＋6测试文件`packView`＋`retire/attack/offscreen/opt41/opt44`过期引用跟改；`browser`零改动）；门禁B0全绿：构建绿、vet（`gpu/...`净，`render`整包仅3存量提示：2不可达＋1测试helper，同基线）、触及文件`gofmt`净、基测（`hal/noop/types/gles`）绿、H2B 7过、R70 3过、`render/gpu`整包绿、带库66过1败（已知`TestS68 X11`同基线，绝对路径）、账本37行602处（基线39行842－`Texture`86－`TextureView`154对上，两类型行清零）；加门wasm vet（`purego struct_other.go:21 syscallArgs`同干净树失败）＋darwin（本机无clang，`runtime/cgo`即止）与本批无关；T5/T12仓内无命令同前片。代码未提交（等人工指令）。 |
| 2026-09-27 | 片7d收敛优化：3残留命名跟正（`dual_tex/filter/layer`的`wgpuView→halView`、`gpu_render_context`的`srcWGPU→srcHalView`＋`wgpuView→halView`＋`texRaw`间接塌缩为`tex`直取）＋3过期注释跟正（`texture.go:39` wraps `*wgpu.Texture`→`hal.Texture`、`text_pipeline.go:933` atlas注释→hal、`gpu_texture.go`两处`Texture()/View()`注释→hal口径）＋1数量口径（规模格“`Texture`86约40文件”→“约32文件（两类并集40文件）”，基线实数：`Texture`32文件／`TextureView`39文件／并集40文件）＋10处死`//nolint:gosec`摘除（`packView`行已无unsafe调用，真抑制只留`view_handle.go`一处）＋触及文件`gofmt`净；复验：构建绿、vet（`gpu/...`净，`render`整包仅3存量提示同基线）、基测全绿、H2B 7过、R70 3过、`render/gpu`整包绿、带库66过1败同基线、断言零残留、账本37行602处不变。代码未提交（等人工指令）。 |
