# P2.3 影响面评估（开工前，2026-10-29）

> 性质：**只评估、不动工作区**。所有结论来自临时 git worktree（HEAD = `6e913abf`，即 P2.2 提交点）里的真实删文件编译实验 + token 级引用扫描。
> 结论一句话：白名单那 10 个文件**不能照着名单直删**——删完有 6 处 `undefined`（3 类符号）；把这 3 类符号按下面 §3 处理掉之后，`go build ./...`、`go vet ./render/internal/gpu`、包内测试编译全过。

## 0 结论摘要

| 项 | 结论 |
|---|---|
| 10 文件直删 | ❌ 编译失败：`MemoryManager`、`Backend`、`StubPipelineID/StubComputePipelineID/StubBindGroupID/StubBufferID` 六处 undefined |
| 最小存活面 | 3 类符号，处理方式见 §3；补丁后全仓 build/vet/测试编译通过（唯一失败是既有 C2 的 `examples/ui_wr_t_stress`，与本任务无因果） |
| 规模 | 源 6515 行 + 测试 5635 行 + WGSL 2071 行 = **14221 行**；导出去重符号 **-131**，新增 0 |
| 测试计数 | `render/internal/gpu` 141/1008/109 → **131/852/79**（Test/Bench 逐项有账，见 §5） |
| 第二层老抽象 | `command_encoder.go`/`commands.go`/`atlas.go`/`gpu_texture.go` 里还有一批只在测试/示例可达的老类型；**不在 P2.3 范围**，登记后续（§6） |
| 能力是否都有新替代 | **8 项有、2 项特殊**：`PipelineCacheCore` 是从未工作的空壳（真实创建被注释），`MemoryManager` 功能完整但形态被新路径的分域预算取代——详见 §0.1 |
| 待用户拍板 | 3 项，见 §8 |

> 口径更正：§2 判定列里的「全死」「仅测试引用」只描述**可达性**（生产/测试/示例谁在调），**不作为删除依据**。本框架允许「先实现、暂未接线」的功能存在，删除的唯一依据是 §0.1 的能力替代核对。

## 0.1 能力替代核对（每个老实现，新路径拿什么接）

| # | 老文件 | 老能力 | 新载体 | 结论 |
|---|---|---|---|---|
| 1 | `backend.go` `Backend` | Instance/Device/Queue 生命周期 + 渲染器工厂 | `VelloAccelerator` 自持设备（`vello_accelerator.go:1330` 建 Instance）+ `render/backend.go:100`；渲染入口换成 `VelloAccelerator`/`RenderSceneAuto` | 已替代 |
| 2 | `renderer.go` `GPUSceneRenderer` | 场景渲染主循环、图层、图集、显存预算 | `vello_accelerator.go` + 各 `session_*` 渲染器 + `VelloAccelerator` 路径 | 已替代 |
| 3 | `sparse_strips_gpu.go` `HybridPipeline` | GPU 三阶段（flatten/coarse/fine）+ 稀疏条带 | `vello_compute.go` + `tilecompute/shaders/{pathtag_reduce,pathtag_scan,draw_reduce,draw_leaf,clip_leaf,path_count,backdrop,coarse,path_tiling,fine}.wgsl`（10 份全 embed） | 已替代（同一 vello 算法的新载体） |
| 4 | `gpu_flatten.go` | GPU 展平 | `tilecompute/flatten.go` + `path_tiling.wgsl`/`flatten` 相关管线 | 已替代 |
| 5 | `gpu_coarse.go` | GPU 粗光栅 | `tilecompute/coarse.go` + `compositor.go` + `tilecompute/shaders/coarse.wgsl` | 已替代 |
| 6 | `gpu_fine.go` | GPU 精细光栅 | `tilecompute/fine.go` + `tilecompute/shaders/fine.wgsl` | 已替代 |
| 7 | `shaders.go` `ShaderModules` + 29 混合模式 | 集中编译 WGSL、混合模式表 | 各子系统自编译（`stencil_pipeline.go`/`image_pipeline.go`/`glyph_mask_pipeline.go`/`text_pipeline.go`/`video_yuv_pipeline.go`/`sdf_render.go`/`convex_renderer.go`）+ `dual_tex_blend.go:818 dualTexModeU` 覆盖 15 种高级混合（Multiply…Exclusion，含 Hue/Saturation/Color/Luminosity）+ 内联 `dualTexBlendWGSL`（:33） | 已替代 |
| 8 | `pipeline.go` `PipelineCache`/`BindGroupBuilder` | 管线缓存、绑定组构建 | 各子系统建管线（无统一缓存层）；`hal` 提供描述符；`Stub*ID` 属命令编码层，搬 `commands.go` | 已替代 |
| 9 | `pipeline_cache_core.go` `PipelineCacheCore` | 哈希键管线缓存 + 命中统计 | **无，也不需要**：`createRenderPipeline`/`createComputePipeline` 里真实创建被注释（`pipeline_cache_core.go:682,723`），`655/715` 明写 `TODO: When pipeline creation is implemented`，返回的是占位管线 → **该能力从未工作过** | 特殊：不是「被替代」，是「从未存在」 |
| 10 | `memory.go` `MemoryManager` | 统一显存预算 + 80% 阈值 LRU 回收纹理 | **形态被取代、能力被分域承接**：`image_cache.go` 128 条/64MiB 硬上限 + LRU（:25-30, :457-467）、glyph 图集页 LRU（`glyph_color_atlas.go:157`）、`depth_clip.go` mask 缓存计数/字节预算（:800）、`texture_pool.go` 128MB（标 Deprecated(P5)，长期归 `res.Cache`）；统一预算计划在 P2.4b | 特殊：**功能完整但唯一实例化点在旧渲染器内部**（`renderer.go:128`），生产从未跑到；新架构没有「统一内存管理器」这个形态 |

两点要说清：

- 第 10 项不是「没用就删」：它的实现是真的（预算、阈值、LRU、Close 回收都能跑），只是**接在旧渲染器里、随旧渲染器一起停用**。新路径把同一件事拆成四个域各自的预算回收，统一预算在 P2.4b 做——届时是复用这块代码还是按域合并，要看 P2.4b 的设计，这正是 D1 要拍的点。
- 第 9 项也不是「没用就删」，而是它想做的事从来没做成：删它删的是空壳和「测哈希表命中数」的 34 个测试，没有能力损失。

## 1 方法（可复现）

```bash
# 1) 干净实验场：不动工作区
git worktree add /tmp/p23wt HEAD
ln -s <repo_parent>/purego /tmp/purego        # go.mod 里 replace ../purego 需要

# 2) 按白名单删 10 源 + 9 测试，让编译器报缺口
rm /tmp/p23wt/render/internal/gpu/{backend,renderer,pipeline,pipeline_cache_core,memory,shaders,sparse_strips_gpu,gpu_coarse,gpu_fine,gpu_flatten}.go
rm /tmp/p23wt/render/internal/gpu/{backend,renderer,pipeline_cache_core,memory,shaders,sparse_strips_gpu,gpu_coarse,gpu_fine,gpu_flatten}_test.go
cd /tmp/p23wt && go build ./render/...

# 3) token 级引用扫描（排除注释、排除选择器 Sel 位）
go run tools/scan_refs.go.txt <pkgdir> <repo> <candidate files>
```

引用扫描口径：同包文件里统计**裸标识符**（`x.Foo` 的 `Foo` 不算引用，避免把 `hal.BlendState`、结构体字段名误判成使用）；跨包只统计 `gpuimpl.X` / `gpu.X` 这类限定引用。结果落盘 `refs.tsv`（109 行）。

## 2 逐文件判定（10 文件）

| 文件 | 行 | 判定 | 依据 |
|---|---|---|---|
| `backend.go` | 345 | 部分存活 | `Backend` 类型被 2 个保留文件引用（§3.1）；`NewBackend/Init/NewRenderer/RenderScene/GPURenderer/newGPURenderer/GPUInfo()` 全死（生产 0 调用，测试 17 处） |
| `renderer.go` | 977 | 全死 | `GPUSceneRenderer`/`NewGPUSceneRenderer` 生产 0 引用；唯一"引用"是 `doc.go` 注释。真渲染走 `VelloAccelerator` |
| `pipeline.go` | 680 | 部分存活 | 4 个 `Stub*ID` 类型活在 `commands.go`（§3.2）；`PipelineCache`/`BindGroupBuilder` 生产 0 引用 |
| `pipeline_cache_core.go` | 739 | 空壳 | `PipelineCacheCore` 及 7 个描述符/`ShaderModule` 类型生产 0 引用（`hal.` 同名类型才是活的，扫描已排除选择器位）；`ShaderModule` 的唯一命中是 `shader_helper.go:56` 的**字段名** `ShaderModule hal.ShaderModule`；能力层面见 §0.1 第 9 项（真实创建被注释） |
| `memory.go` | 425 | 功能完整、接入在旧渲染器内 | `MemoryManager` 仅剩 `gpu_texture.go` 的可选钩子（§3.3）；`NewMemoryManager` 生产 0 调用，`MemoryStats` 的 2 处命中都是同名方法名（返回 `GPUMemoryStats`），非该类型；唯一实例化点 `renderer.go:128`（旧渲染器内部），能力替代情况见 §0.1 第 10 项 |
| `shaders.go` | 349 | 全死 | `ShaderModules`/`ShaderModuleID`/29 个 `ShaderBlend*`/`CompileShaders` 等生产 0 引用 |
| `sparse_strips_gpu.go` | 843 | 全死 | `HybridPipeline`/`HybridFineRasterizer`/`PipelineStage` 生产 0 引用；`vello_compute.go:88` 只有一句注释提到 |
| `gpu_coarse.go` | 690 | 全死 | `GPUCoarseRasterizer` 仅测试引用 |
| `gpu_fine.go` | 669 | 全死 | `GPUFineRasterizer`/`GPUSegment` 仅测试引用 |
| `gpu_flatten.go` | 798 | 全死 | `GPUFlattenRasterizer` 仅测试引用 |

> 名字陷阱（PLAN 已点名）：`gpu_coarse.go`/`gpu_fine.go`/`gpu_flatten.go`/`sparse_strips_gpu.go` 是**旧 GPU 三阶段簇**（删）；`coarse.go`/`fine.go`/`flatten.go`/`sparse_strips.go`/`segment.go`/`tilecompute_filler.go` 是**在用的 CPU 侧**（留）。逐个文件核对过，无语义重叠。

## 3 最小存活面（3 类符号，编译器实测）

### 3.1 `Backend`（backend.go）→ 精简后落 `device.go`

真实用途只有 `device/queue/initialized` 三个字段 + `IsInitialized()/Device()/Queue()/Close()` 四个方法：

- `command_encoder.go:107,129`（`*Backend` 字段与构造参数；读 `mu/initialized/device`）
- `gpu_texture.go:210,289`（`CreateTexture`/`CreateTextureFromPixmap` 形参；调 `IsInitialized/Device/Queue`）
- `atlas.go:307`（透传）
- 测试：`command_encoder_test.go`（3 处，测 `initialized` 语义）、`p0_format_readback_test.go`（4 处，构造 `&Backend{device,queue,initialized:true}`）

处置：把上面这层壳搬到 `device.go`（那里已有 `releaseDevice/releaseAdapter`）；`NewBackend`（唯一自行 `webgpu.CreateInstance` 的旧入口）、`Init`、`Name`、`GPUInfo`、`NewRenderer`、`RenderScene`、`GPURenderer` 随 `backend.go` 一起删。**保留字段名不变**，上列 7 处测试无需改。

备选（更彻底）：不要 `Backend`，把三个调用点改成收 `hal.Device`/`hal.Queue`。代价：`TestCoreCommandEncoder_CreateFromBackend` 的 `initialized=false` 用例语义要重写。

### 3.2 `Stub*ID`（pipeline.go）→ 搬到 `commands.go`

`commands.go` 本来就住着 `StubCommandEncoderID`/`StubCommandBufferID`，同类就近归位：

- `StubPipelineID`、`StubComputePipelineID`、`StubBindGroupID`（各 2~4 处，全在 `commands.go` 的 pass 方法签名里）
- `StubBufferID`（4 处，同上）
- `StubBindGroupLayoutID`：只活在 `renderer_test.go:499`（随测试一起删），**不迁**

### 3.3 `MemoryManager` 钩子（memory.go ↔ gpu_texture.go）→ 摘除

`GPUTexture.manager` 字段、`SetMemoryManager` 方法、`Close()` 里 `manager.unregisterTexture` 分支。唯一 setter 在 `memory.go:328`（`AllocTexture` 内），而 `NewMemoryManager` 生产 0 调用 → 该字段生产永远为 nil。摘除后 `Close()` 行为不变（少一次恒假分支）。

## 4 WGSL 逐个 embed 核对

`//go:embed` 精确路径核对（不是按文件名模糊匹配）：

| 删（仅被待删文件 embed） | 留（被保留文件 embed） |
|---|---|
| `shaders/blit.wgsl`（`shaders.go`） | `convex` `cover` `cover_aa` `cover_textured_linear` `cover_textured_pattern` `depth_clip` `glyph_color` `glyph_mask` `glyph_mask_lcd` `msdf_text` `sdf_render` `stencil_fill` `textured_quad` `textured_quad_bicubic` `video_yuv` |
| `shaders/blend.wgsl`（`shaders.go`） | |
| `shaders/strip.wgsl`（`shaders.go`） | |
| `shaders/composite.wgsl`（`shaders.go`） | |
| `shaders/coarse.wgsl`（`gpu_coarse.go`） | |
| `shaders/fine.wgsl`（`gpu_fine.go`） | |
| `shaders/flatten.wgsl`（`gpu_flatten.go`） | |

易混点：`vello_compute.go:57,63` 引的是 `tilecompute/shaders/coarse.wgsl`、`tilecompute/shaders/fine.wgsl`（另一目录，vello 在用），与 `shaders/coarse.wgsl`、`shaders/fine.wgsl` 无关——按路径核对过，删后者不影响前者。

删 7 个 = 2071 行；剩 15 个全部有 embed 出处。

## 5 测试计数与 API 快照账

计数口径与 `baseline/api` 完全一致（`FUNC_RE = ^func\s+(Test|Benchmark|Example|Fuzz)\w*\(`，逐 `_test.go` 静态数）。

| 包 | 基线（P1.5） | 删后（实验实测） | 差 |
|---|---|---|---|
| `render/internal/gpu` test_files | 141 | 131 | −10 |
| test_funcs | 1008 | 852 | −156 |
| bench_funcs | 109 | 79 | −30 |

拆解：

- 9 个簇测试文件：Test 147 / Bench 19（`backend_test` 10+1、`renderer_test` 23、`pipeline_cache_core_test` 29+5、`memory_test` 17、`shaders_test` 20、`sparse_strips_gpu_test` 17+3、`gpu_coarse_test` 10+2、`gpu_fine_test` 8+2、`gpu_flatten_test` 13+6）
- `integration_test.go`：9 Test（全部建立在 `NewBackend/RenderScene/NewMemoryManager/ShaderModules/NewPipelineCache` 上）
- 合计删 Test 156 / Bench 19；`benchmark_test.go` 另删 11 个 Benchmark（PipelineCreation、BlendPipelineCache、MemoryAllocation、MemoryTouch、BackendInit、Clear1080p、Rect100、Circle50、PathComplex、Layers4、BlendModes）+ 1 个 helper `runBackendComparison`；保留 `BenchmarkSceneCreation` → 总差 test −156 / bench −30，与实验实测一致

API 快照（`go doc -all ./render/internal/gpu`）：

|  | 行数 | `func/type/const/var` 声明行 |
|---|---|---|
| 删前 | 7800 | 1098 |
| 删后 | 6480 | 901 |

符号集合 diff：**新增 0、删除 131**（`Backend` 因搬家不计删除）。P2.6 对账时只允许这一条包级差异。

## 6 顺带发现（不在 P2.3 范围，需要归位）

1. **第二层老抽象**：`command_encoder.go`（`CoreCommandEncoder` + `RenderPassDescriptor`/`ImageCopyTexture` 等 wgpu 风格描述符）、`commands.go`（`CommandEncoder`/`RenderPass`/`CommandBuffer`）、`atlas.go` 的 `TextureAtlas`、`gpu_texture.go` 的 `GPUTexture` 这一套，生产可达性只在测试/示例；但 `RectAllocator`（atlas.go）被 `glyph_color_atlas.go` 在用、`IndexFormat` 被 `render_pass.go` 在用、`*RenderPass` 出现在 `text_pipeline.go` 两个签名里 → **不是整块死代码，拆解要逐符号判**。建议登记到 P2.4e（入口收敛）或单开 P2.3b，本任务不碰。
2. **C2**：`examples/ui_wr_t_stress` 引用早已不存在的 `ui/kit/button`、`ui/kit/icon`，`go build ./...` 在它这挂（非本任务引起，基线里就有）。台账把 C2 挂在 P2.3 名下，但它与 GPU 老渲染器无因果关系。当前 `ui/kit` 只有扁平文件（`color_model_*` 等），没有 button/icon 子包 → 这个 T 相门禁窗现在无法补齐。
3. `docs/render_gpu_converge/evidence/P2.2/api-post` 之类历史证据不动；需要同步的只有 `RENDER_API_CATALOG.md`（`render/internal/gpu` 内部包本就不在目录文档的公开面里，但 §7.2 有相关行）与 `doc.go` 重写。

## 7 `webgpu.CreateInstance` 白名单口径

删前 4 处（库内，不含 `gpu/webgpu/examples`）：`render/backend.go:100`、`render/internal/gpu/vello_accelerator.go:1330`、`render/internal/gpu/backend.go:80`（本次删）、`render/internal/gpu/gpu_shared.go:769`。

删后 3 处，登记为本阶段白名单（P2.4a 再收敛到一处）：`render/backend.go:100`、`vello_accelerator.go:1330`、`gpu_shared.go:769`。

## 8 待确认（停在这里）

按 §0.1 的核对结果，三个决定换了问法：

**D1｜`memory.go` + `pipeline_cache_core.go` 怎么处置**（其余 8 个能力均已替代，无争议）

- D1-A（全删 10 个）：P2.3 一次清掉。代价是统一内存管理器的现成实现没了，P2.4b 统一预算时按分域合并重做；收益是本阶段收尾干净、不留旧簇尾巴。`pipeline_cache_core` 无论选哪个都删，它从没工作过。
- D1-B（删 8 个，留 `memory.go` + `pipeline_cache_core.go`）：承认「先实现未接线」的现状，等 P2.4b 统一预算时再决定复用还是拆。代价是保留 slim `Backend`、`SetMemoryManager` 钩子、`GPUTexture.manager` 字段，以及 51 个 Test/5 个 Bench 及其依赖继续存在；本阶段「`render/internal/gpu` 收敛」的实际达成度下降。
- 我的建议：**D1-A**。分域预算已经落地并被生产路径使用，统一管理器要在 P2.4b 重新设计，留着的旧实现大概率不会原样复用，反而会让「已收敛」的账不实。但这条要你拍，因为它是「已实现但未使用」的边界情况。

**D2｜`examples/ui_wr_t_stress` 归属**（C2 登记在 P2.3 名下，实际与 GPU 老渲染器无因果）

它要的 `ui/kit/button`、`ui/kit/icon` 根本还不存在，是组件缺失，不属于 P2 零行为重构能解决的范围。建议把台账 C2 改挂到组件线（P3 能力补齐），P2.3 只做记录不修。若你要求 P2.3 内修，就得先建这两个组件包，那会把 P3 的活提前进 P2。

**D3｜`benchmark_test.go` 的去留**

11 个 bench 全部只测老簇（PipelineCreation/BlendPipelineCache/MemoryAllocation/MemoryTouch/BackendInit/Clear1080p/Rect100/Circle50/PathComplex/Layers4/BlendModes），老簇删了就没有被测对象。建议删 11 个 + helper，留 `BenchmarkSceneCreation`（测的是 `NewScene`，属保留代码）。若 D1 选 B，则 `PipelineCreation`/`BlendPipelineCache`/`MemoryAllocation`/`MemoryTouch` 4 个有对象可测，可一并保留。
