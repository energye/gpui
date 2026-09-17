# 01 render 导出与多入口嫌疑

> 口径：`glob=!*_test.go` + `^func [A-Z]` / `^type [A-Z]` / `^(var|const) [A-Z]`，非测试顶层声明。
> 局限：`^(var|const)` 块式成员会被漏计（如 `ErrNoBackendAvailable` 在 `var()` 块内）；`render` 主包直接 grep 会混入子包，主包数以 `RENDER_API_CATALOG.md` 快照为准。
> 证据：文件:行号均为只读实测行。

## 1 包规模

| 包 | .go 文件数 | 导出数（本批口径） | 目录对照 |
|---|---|---|---|
| `render` 主包 | 204（顶层） | 类型 99·函数 123·Context 方法 184·常量 131·变量 16（目录 2026-08-15 快照 + 09-05/09-15 增量） | 🔗生产主链路 |
| `render/internal/gpu` | 213（顶层；子树 244 含 tilecompute/res） | `^func` 123 + `^type` 224（含子包；var/const 单行式极少，块式漏计） | 引擎资源大本营 |
| `render/text` | 220（顶层；子树 277） | `^func` 188（子包约 58：msdf25+emoji27+hint2+cache4，顶层约 130）；目录 go doc 口径 241 | 🔗字形子系统 |
| `render/scene` | 40 总数 / 16 非测试 | func67 + type64 + var1 = 132；var 仅 `pool.go:60 DefaultPool` | render 内部吃 Scene |
| `render/recording` | 12 总数（顶层 11 + backends/raster 1） | func25 + type54；var/const 单行式 0（`InvalidRef` 在块内，漏计） | 🔌仅测试/示例 |
| `render/surface` | 7 非测试 | func19 + type28；var/const 单行式 0（`ErrNoBackendAvailable` 在 `registry.go:260-262 var()` 块内） | 🔌无消费者 |
| `render/svg` | 8 总数 / 7 非测试 | func3 + type12（`svg/svg.go:13 Render`、`：24 RenderWithColor`、`parser.go:16 Parse`） | 🔌无消费者 |
| `render/filters` | 1（`register.go` 仅 init） | 0/0/0 | 经 render/gpu 副作用注册 |
| `render/raster` | 1（仅 init） | 0/0/0 | 🔌 |
| `render/gpu` | 2（`gpu.go/lifecycle_policy.go`） | func8 + type1 = 9（目录 go doc 12，差 3 是块式常量漏计） | 真窗全量接线 |
| `render/render` 同名子包 | 8 | func9（抽查：`scene.go:95 NewScene`、`layers.go:63 NewLayeredPixmapTarget` 等） | 待核对 |

## 2 多入口嫌疑（只列头注释+签名，不定逻辑）

### 建纹理（render 主包 1 + internal/gpu 6）

- `render/context_image.go:902 CreateOffscreenTexture`：离屏纹理，可经 FlushGPUWithView 渲染、DrawGPUTexture 合成，GPU 不可用返 nil。
- `render/internal/gpu/gpu_texture.go:200 CreateTexture`：建空纹理待 UploadPixmap，nil backend 走 stub。
- `render/internal/gpu/gpu_texture.go:279 CreateTextureFromPixmap`：建纹理并立即上传。
- `render/internal/gpu/texture.go:610 CreateCoreTexture`：四种失败返回，缺省补 1。
- `render/internal/gpu/texture.go:690 CreateCoreTextureSimple`：2D 单层单 mip 便捷版。
- `render/internal/gpu/texture.go:117 NewTexture`：已有 GPU 纹理的包装构造。
- `render/internal/gpu/atlas.go:297 NewTextureAtlas`：图集分配器。
- `render/internal/gpu/gpu_render_context.go:3364 GPURenderContext.CreateOffscreenTexture`：按 (w,h) 复用池化纹理（但复用分支已注释禁用，见 03）。

### 建缓冲（3 + 1 包装）

- `render/internal/gpu/buffer.go:569 CreateBuffer`：完整校验 + 4 字节对齐。
- `render/internal/gpu/buffer.go:633 CreateBufferSimple`：简化版。
- `render/internal/gpu/buffer.go:663 CreateStagingBuffer`：中转缓冲。
- `render/internal/gpu/buffer.go:206 NewBuffer`：已有 buffer 包装。

### 建设备/选卡（主包 5 + internal 镜像 2）

- `render/adapter_policy.go:102 RequestAdapterWithPolicy`：按策略选卡，只剩软光栅置 forceFallback。
- `render/adapter_policy.go:179 DeviceDescriptor`：从 DefaultLimits 只抬 storage buffers。
- `render/adapter_policy.go:197 DeviceDescriptorLowVRAM`：1–2G 卡收紧 64M/32M/4k。
- `render/adapter_policy.go:224 DeviceDescriptorForAdapter`：核显/CPU 自动 LowVRAM，`GPUI_LOW_VRAM=1` 强制。
- `render/present_target.go:22 requestPresentDeviceWithRetry`（非导出）：present 建设备 OOM 3 次重试。
- `render/present_target.go:339 NewPresentTarget`：建窗呈现目标（含风暴 resize 状态机）。
- `render/internal/gpu/device.go:16 renderDeviceDescriptor`（非导出）：与主包 DeviceDescriptor 同义（storage 抬到 9）。
- `render/internal/gpu/device.go:93 requestDeviceWithRetry`（非导出）：iGPU 瞬时 OOM 3 次 + 1s 退避。
- 调用点：`gpu_shared.go:676`（gg-shared）、`vello_accelerator.go:1345`（gg-vello）。

### 放资源/purge（主包 6，internal 见 03）

- `render/oom_purge.go:29 RegisterPurgeEvictable` / `:40 UnregisterPurgeEvictable` / `:57 PurgeEvictables` / `:77 IsGPUOutOfMemory`。
- `render/accelerator.go:394 PurgeAcceleratorSurfaceResources`：只清表面显存不弃设备。
- `render/accelerator.go:404 AbandonAcceleratorDevice`：Skia abandon 语义。

### 缓存/池（主包 2 + internal 见 03）

- `render/filter_ops.go:31,36 FilterPoolStats/ResetFilterPoolStats`：滤镜池计数。
- `render/pixmap_pool.go:43,47 newPixmapPool/newPixmapPoolBudget`（非导出）：像素缓冲池。
- internal 几何四件套：`path_geometry_cache.go:123,319,431,564 NewPath/Stroke/Dash/ConvexPathCache`；管线两件套：`pipeline.go:85 NewPipelineCache`（老 stub）、`pipeline_cache_core.go:70 NewPipelineCacheCore`；通用：`res/cache.go:40 NewCache`。

## 3 未列入口的包（已核对，非遗漏）

- `render/internal/blend 22`、`clip 8`、`color 6`、`filter 10`、`image 18`、`parallel 12`、`raster 32`、`stroke 4`、`wide 10`、`cache 6`（go 文件数，顶层）：CPU 侧能力包，无 GPU 分配入口，本批只计数；池语义见 03 §1.5。
- `render/internal/testutil`：仅 `imagediff/imagediff.go` 1 个 go 文件（金图比对帮手，容差见 05 §4）。
- `render/text/emoji 11`、`hint 25`、`msdf 13`、`cache 4`：已含在 text 子包约 58 计数内。
- `render/integration`、`examples`、`tests`、`tmp`、`testdata`：目录存在但顶层 0 个 go 文件（支撑/用例目录，非包），无导出可盘。

## 4 本文件未覆盖

- `render/text` 220 文件未逐函数比行为，只计数。
- 调用方“谁调谁”见 02（gpu 侧）与 04（死代码侧），本文件只列入口。
