# GL 后端迁移计划（把 gogpu 的 GL 接进 gpui）

> 一句话：现在 gpui 只会走 Rust 编的 WebGPU 那套，一个窗口吃几百 M，1G 卡开几个窗就爆。gogpu 那边有现成的纯 Go 的 GL 路，一个窗口几十 M。把那条路搬过来，Linux 和 Windows 一起接，macOS 不接。
> 只走标准做法，非标准的不要。示例只做验证，不绕引擎。

## 0. 为啥干

- 现状：gpui 经 `gpu/rwgpu`（Rust 库，purego 调）再包一层 `gpu/webgpu`，`render/present_target.go` 只会调这条。开关 `GPUI_BACKEND=gl` 只能让它去找 GL 卡，窗口那步报不兼容，画面送不上去。
- 量到的数：独显 940MX（1G）上鹈鹕单窗驱动侧 655M，整卡 663/1024M，退出回 3M；账本只记 30M，剩下 600M 是驱动和库内部吃掉的。空白窗也要 300M 打底。Skia 的 GL 路单窗 30M 级别。
- 结论：WebGPU 吃显存是天生的，三端都这样。1G 卡上多开窗口，只能走 GL。

## 1. 目标和不目标

- 目标：`GPUI_BACKEND=gl` 在 Linux 和 Windows 上真能上屏，画面跟现在一模一样，三窗（图标/鹈鹕/列表）都过，单窗驱动侧显存降一个数量级。
- 不目标：不动画面，不动上层接口；macOS 不接 GL（那边只有 Metal 的实现，苹果也没有真 GL）；视频链路不动；不在示例里绕。

## 2. 范围（三端一次写清）

| 平台 | 窗口 | 状态 | 走哪个系统接口 |
|---|---|---|---|
| Linux X11 | 接 | 要做 | EGL + Xlib 窗（`hal.SurfaceTargetXlibWindow`） |
| Linux Wayland | 接 | 要做 | EGL + `wl_egl_window` 包装（`hal.SurfaceTargetWaylandSurface`，经 libwayland-egl） |
| Windows | 接 | 要做 | EGL + HWND（`hal.SurfaceTargetWindowsHWND`，已有注册 `hal/gles`） |
| macOS | 不接 | 原因：gogpu 那边 macOS 只有 Metal 的文件，无 GL 实现；苹果无真 GL | 继续走现有 Metal/Vulkan 路 |
| Android | 不接 | 原因：gogpu 的 GL 没接 `AndroidNativeWindow` | 以后再说 |

## 3. 搬啥（接点）

- 搬的代码：`hal/gles` 本体（约 1.1 万行）+ `egl` + `gl` 两个子包（约 5800 行），测试文件一起搬。落到 `gpu/gwgpu/` 下按原样摆三块，`gles/`、`egl/`、`gl/`。
- 断根三条：不加 `goffi`（调用换 gpui 现成的 purego，自研 `ffishim` 垫片）、不加 `gputypes`（换 gpui 自己的 `gpu/types`+补 7 类小类型）、naga 也自己养（最小闭包 `wgsl+ir+glsl+internal+根门面`，spirv/msl/hlsl/dxil 不搬）；go.mod 无新增外部依赖。以后这三块咱们自己修，不跟那边合。
- gpui 侧接线：`gpu/webgpu` 还是统一的口，内部按 `GPUI_BACKEND` 二选一，默认不动，不支持自动回来；`render/present_target.go` 的建面和同步按后端二选一。
- 着色器：现在 WGSL 下发，GL 要转 GLSL（naga 那套）。转完逐个对像素，差了就停。

## 4. 分期

- P0 验证（已完，未提交）：34 个实现文件 + 21 个单测文件搬进 `gpu/gwgpu/`（hal/hal-noop/gles/egl/gl/wgl 原样摆）；断根三条全落实（goffi→自研 ffishim 走 purego，gputypes→`gpu/types`+补 7 类小类型，naga 留依赖 v0.19.0）；格式/vet 全干净；单测全绿；真机 EGL 1.5 初始化一次过（Mesa，surfaceless）。
- P1 单窗打通：鹈鹕单窗 GL 上屏，像素零差，帧数和同步正常。
- P2 三窗回归：图标/鹈鹕/列表各按现有时长重跑，像素加帧间隔加人工看，三证据齐。
- P3 双后端并存：默认不动（还是现在这套），`GPUI_BACKEND=gl` 切换，不支持自动回退。Windows 同样走一遍 P0-P2。

## 5. 门禁

- 像素：两时刻 Golden 逐位对，容差写明；逻辑探针加像素断言加 Golden，缺一不可。
- 显存：`nvidia-smi` 量单窗和三窗同开，对比现在这套，写明降了多少。
- 回归：按测试文件逐个跑，不一次全量；长跑给足时间；缺数据用 `t.Skipf`，不假绿。
- 跨平台：上表一次标齐，不留空白格；版本号加修订一起走。

## 6. 风险

- 两个 GPU 实现并存（Rust 原生和纯 Go），依赖和构建变复杂。做法：GL 独立分支，默认不动，出问题一键回退。
- GL 驱动差异（Mesa 新版本和旧卡行为不同）。做法：先在现有真机（HD 520/940MX）验，再谈别的卡。
- 着色器转译差一像素。做法：像素门禁不过就停，不硬上。

## 7. 修订

| 日期 | 说明 |
|---|---|
| 2026-09-24 | 立项：Linux+Windows 一起接，macOS 明确不接；P0 先验显存再往下走 |
| 2026-09-24 | P0 搬完：断根（ffishim/purego、无 goffi/gputypes 新依赖）；hal 单测 8 个文件、gles 单测 13 个全绿；真机 EGL 1.5 初始化过；naga v0.19.0 留依赖（着色器转译用，vendoring 不现实） |
| 2026-09-24 | naga 自养：最小闭包搬进 `gpu/gwgpu/naga`（wgsl+ir+glsl+internal+根，spirv 等不搬，门面裁到 Parse/Lower/Validate）；go.mod 零新增外部依赖；单测 wgsl/ir/glsl-codegen/internal 全绿 |
