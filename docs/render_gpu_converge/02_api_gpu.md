# 02 gpu 导出与调用关系

> 口径：`^func [A-Z]` / `^type [A-Z]`，`!*_test.go` 为生产。
> 局限：`descriptor_browser.go` 等 build tag 双实现 grep 会 double count，编译只留一份；`gpu/shader` 子树 200+ 文件只给顶层精确数。
> 结论先行：`render/internal/gpu`、`render/*.go`、顶层 `examples/` 都不直调 `rwgpu.Create*`，只调 `webgpu.*`；`webgpu` 是唯一转发层；唯一直调 rwgpu 的是 `gpu/rwgpu/examples/`。

## 1 包规模

- `gpu/types`（16 go：15 生产 + 1 测试）：生产 func 20（`geometry.go:18 NewExtent2D`、`:27 NewExtent3D`、`limits.go:80 DefaultLimits`、`:121 DownlevelLimits`、`color.go:19 NewColor` 等）+ type 94。
- `gpu/context`（23 go：14 生产 + 9 测试）：生产 func 9（`registry.go:36 WithPriority`、`:43 NewRegistry`、`webgpu_types.go:39 NewDevice`、`:52,65,78,91 NewQueue/Adapter/Surface/Instance`、`handle.go:26 NewTextureView`、`:47 NewCommandEncoder`）+ type 53。
- `gpu/rwgpu`（主包 85 go + examples 14 main）：生产顶层 func 30（`safety.go:17 LockGPU`、`:20 UnlockGPU`、`:77 ClassifyError`，`adapter.go:18 EmptyStringView`，`device.go:107 LastUncapturedError`、`:119 PeekLastUncapturedError`，`wgpu.go:206 Init`，`vram_ledger.go:140 VramLiveBytes`、`:147 VramLiveCount`，`bindgroup.go:394 BufferBindingEntry` 等，`math.go` 8 个 Mat4，`debug.go` 3 个，`command.go:579 CmdBufLive`，`instance.go:103 CreateInstance`）+ type 主包约 178 + examples 27。
- `gpu/webgpu`（71 go 含 internal/browser、internal/thread）：生产 func 99（含子包；本包如 `gpucontext_helpers.go:14 DeviceFromHandle` 等 12 个，`wrap.go:9 NewDeviceFromHAL` 与 `wrap_browser.go:9` 同名双实现，`instance.go:35 CreateInstance` 与 `instance_browser.go:30` 同名双实现，`swapchain.go:153 NewSwapchain`；browser 子包 20+ `*ToJS` + `Build*`；thread 2 个）+ type 208（含 browser 双实现 double count）。
- `gpu/shader`（顶层 3 go + 8 子树 200+ go）：顶层生产 func 8（`naga.go:57,69,80,121,143,151,168,176 DefaultOptions/Compile/.../GenerateSPIRV`）+ type 1（`naga.go:45 CompileOptions`）；子树递归 `^func` 889+ 行被截断，未断言总数。
- `gpu/common`：目录存在但全树 0 个 go 文件，无代码可盘。
- `gpu/webgpu/internal/browser 25`、`internal/thread 3`（go 文件数）：已含在 webgpu 71 计数内（browser `*ToJS`/`Build*`、thread `New`/`NewRenderLoop`）。

## 2 rwgpu Create/Release/Destroy/Request → 上层

| rwgpu 函数 | 上层直调 | 证据 |
|---|---|---|
| `CreateInstance instance.go:103` | 经 webgpu | 转发 `webgpu/instance.go:51 rwgpu.CreateInstance`；上层 `backend.go:69`、`gpu_shared.go:650`、`present_target.go:396 webgpu.CreateInstance` |
| `(Instance) RequestAdapter adapter.go:115` | 经 webgpu | 转发 `webgpu/instance.go:77`；上层 `backend.go:76`、`gpu_shared.go:658`、`present_target.go:414` |
| `(Adapter) RequestDevice device.go:392` | 经 webgpu | 上层 `internal/gpu/device.go:100`、`present_target.go:29`；render 内 `grep rwgpu\.` 仅 `gpu_shared.go:347,373 WithForce...` |
| `(Device) CreateBuffer buffer.go:121` | 经 webgpu | 转发 `webgpu/device.go:49`；上层 `stencil_renderer.go:575,600,708,802,871`、`textured_stencil_pattern.go:551...`、`sdf_render.go:637,689` 等 |
| `(Device) CreateTexture texture.go:236` | 经 webgpu | 转发 `webgpu/device.go:70`；上层 `stencil_renderer.go:667`、`sdf_render.go:219,247`、`filter_gpu_graph.go:411...`、`swapchain.go:497`（恢复探针） |
| `(Texture) CreateView texture.go:72` | 经 webgpu | 转发 `webgpu/device.go:121`；上层 `stencil_renderer.go:677` 等 |
| `CreateSampler/Linear/Nearest sampler.go:50,96,110` | 经 webgpu | 转发 `webgpu/device.go:151`；render 无直调 |
| `CreateShaderModule* shader.go:21,64,89,100,119` | 经 webgpu | 转发 `webgpu/device.go:172,174` |
| `CreateBindGroup/Layout* bindgroup.go:254,296,318,372` | 经 webgpu | 转发 `webgpu/device.go:208,239,276` |
| `CreatePipelineLayout/Simple、CreateComputePipeline/Simple pipeline.go:77,127,149,201` | 经 webgpu | 转发 `webgpu/device.go:239,325` |
| `CreateRenderPipeline/Simple render_pipeline.go:216,461` | 经 webgpu | 转发 `webgpu/device.go:297` |
| `CreateCommandEncoder command.go:51`、`CreateRenderBundleEncoder/Simple render_bundle.go:42,91`、`CreateQuerySet queryset.go:21` | 经 webgpu 或无上层命中 | 同上转发；后两者上层无命中 |
| `CreateDepthTextureErr/Device.go:801`、`CreateDepthTexture :821` | 0 生产调用（见下） | 仅测试 + 示例 |
| `Release/Destroy` 全系（Device/Queue/Instance/Adapter/Buffer/Texture/View/Sampler/Module/QuerySet/Encoder/Pass/Bundle/Pipeline/BindGroup 20+） | 无直调 | 转发如 `webgpu/device.go:563 d.r.Release()`；上层全是 webgpu 对象方法（如 `gpu_shared.go:661 instance.Release()`），接收者不同 |

唯一直调 rwgpu 的是 `gpu/rwgpu/examples/`（rotating-triangle、colored-triangle、mrt、textured-quad、render_debug_markers、cube），顶层 `examples/` 下 `grep rwgpu\.` 0 命中。

## 3 特别核对

- **Depth 双子**：`device.go:801 CreateDepthTextureErr` 是带错版（固定 RenderAttachment/2D/1mip/1sample 套壳 `CreateTexture`）；`:821 CreateDepthTexture` 是 `t, _ := ...Err` 吞错版。生产 0 调用；仅 `texture_test.go:123`、`null_guard_test.go:106`、`examples/cube/main.go:343`。
- **Texture vs Buffer 回调**：大框同（vramCheck 快拒 → 原生 Call → 查 LastUncapturedError → 释半成品回 WGPUError → 空句柄报错 → track+vramAdd）。三处不同：Texture 补默认 Mip/Sample=1、算 layers/samples、转 ViewFormats、打 est_mib（`texture.go:255-273,294-302,243-252`），Buffer 直接用 Size、打 BUF_CREATE（`buffer.go:137-139`）；记账 Texture 用 trackResourceLabel（`:330-331`），Buffer 用 trackResource + mapState（`:164-170`）；Buffer 显式 KeepAlive（`:148-149`）。
- **OnSubmittedWorkDone 唯一实现**：`queue_workdone.go:83 (Queue) OnSubmittedWorkDone`；`queue_workdone_unix.go:13` / `_other.go:9` 是同名 C 调 helper（build tag 二选一）；`wgpu.go:116,352` 只是 Proc 符号；转发唯一 `webgpu/queue.go:24`；render 仅 `res/submission.go:6` 注释提及。
