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
  - H1 `hal` 对齐 `webgpu`（未开始）：`hal` 只定接口，`render` 调了啥就对应上啥；先拉对照表，以源码为准。
  - H2 `webgpu` 实现新 `hal` 口（未开始）：各类型实现接口，`var _ hal.Xxx` 全过。
  - H3 `render` 全换 `hal` 口（未开始）：只存 `hal.xxx` 接口，先接上去跑通。
  - H4 `gwgpu` 补实现并收官（未开始）：`render` 跑通后再做，对照新 `hal` 改、对照 `webgpu` 功能补；三端构建+全量单测+鹈鹕单窗真上屏（像素零差+显存对照）。
- 历史留档：2026-09-25 前的 H1/H2/H3 交货记录已回退干净（`gpu` 58 改回+18 新增删，`render` 100 改回+2 新增删，备份在 `/tmp/opencode`），待按新 H1-H4 重做，明细见 git 历史与备份 patch。

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
