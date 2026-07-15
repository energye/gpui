# rwgpu × Skia 2D WebGPU 子集检查表（S1 工作底稿）

> 版本：0.3 | 日期：2026-07-15  
> 来源：[`SKIA_2D_CAPABILITY_MATRIX.md`](./SKIA_2D_CAPABILITY_MATRIX.md) §2  
> 主线：[`MAINLINE_PLAN.md`](./MAINLINE_PLAN.md) S1  
> 状态图例：✅ 绑定+测试锁定 | 🔄 有绑定/部分测 | ⬜ 缺失/不可用 | ❓ 未核实

---

## 审计方法

1. 以 `lib/webgpu.h` 为权威。  
2. 对照 `gpu/rwgpu` 的 `NewProc("wgpu…")`、Go API、`convert.go` wire。  
3. 每项关闭条件：映射正确 + 测试（单元或 native 烟测）+ 备注。  
4. **M0–M2 依赖项不得长期 ⬜。**

**测试入口：**

| 类别 | 文件 / `-run` |
|------|----------------|
| Enum header-lock（S1 核心） | `gpu/rwgpu/s1_skia_subset_enum_test.go` → `TestS1*` |
| A–E native 烟测（S1 收尾） | `gpu/rwgpu/s1_ae_smoke_test.go` → `TestS1AE*` |
| Struct size/offset / 既有 enum | `gpu/rwgpu/abi_test.go` |
| 包级回归 | `WGPU_NATIVE_PATH=... go test -count=1 ./gpu/rwgpu` |

**转换权威：** `gpu/rwgpu/convert.go`  
**Wire 使用点：** `render_pipeline.go`（topology/front/cull/vertex）、`bindgroup.go`（binding types）

---

## A. 实例 / 适配器 / 设备 / 队列（M0）

| 项 | wgpu 符号（代表） | Go 侧 | 状态 | 测试 | 备注 |
|----|-------------------|-------|------|------|------|
| A1 | wgpuCreateInstance | CreateInstance | ✅ | `TestS1AE_A_*` + instance 测试 | |
| A2 | RequestAdapter | Instance.RequestAdapter | ✅ | `TestS1AE_A_*` + `TestMultipleAdapterRequests` | 并发 RequestAdapter 在 GLES abort：默认 skip，需 `RWGPU_CONCURRENT_STRESS=1` |
| A3 | RequestDevice | Adapter.RequestDevice | ✅ | `TestS1AE_A_*` + device 测试 | limits/features |
| A4 | DeviceGetQueue | Device.Queue | ✅ | | |
| A5 | DevicePoll | Device.Poll | ✅ | `TestS1AE_*` Poll(true) | native 扩展 |
| A6 | ErrorScope push/pop | Push/PopErrorScope | ✅ | `TestS1AE_A_*` + errors 测试 | |
| A7 | Adapter/Device limits/features | Limits/Features/HasFeature | ✅ | `TestS1AE_A_*` | |
| A8 | Release 全对象 | Release | ✅ | 各 `TestS1AE_*` defer Release | 生命周期 |

## B. Buffer（M0–M2）

| 项 | 内容 | 状态 | 测试 | 备注 |
|----|------|------|------|------|
| B1 | CreateBuffer + usage 全集 | ✅ | `TestS1AE_B_*` + buffer + usage bitflags | |
| B2 | QueueWriteBuffer | ✅ | | |
| B3 | MapAsync + GetMappedRange + Unmap | ✅ | `TestS1AE_B_BufferWriteMap` Map/GetMappedRange/Unmap | MappedAtCreation + Map 读回 |
| B4 | Destroy/Release | ✅ | `TestS1AE_B_*` Destroy/Release | |
| B5 | usage/size/mapState 查询 | ✅ | `TestS1AE_B_*` Size/Usage | |

## C. Texture / View / Sampler / Copy（M0–M2）

| 项 | 内容 | 状态 | 测试 | 备注 |
|----|------|------|------|------|
| C1 | CreateTexture | ✅ | `TestS1AE_C_*` + texture | format 身份映射已锁 |
| C2 | CreateView | ✅ | `TestS1AE_C_*` | |
| C3 | QueueWriteTexture | ✅ | `TestS1AE_C_TextureWriteCopyMap` | bytesPerRow=256 对齐 |
| C4 | CopyTextureToBuffer/ToTexture/B2T/B2B | ✅ | `TestS1AE_C_*` T2B；`TestS1AE_B_*` B2B | T2T 有绑定+null_guard；M3 可加深 |
| C5 | CreateSampler | ✅ | `TestS1AE_C_*` + enum lock | |
| C6 | 格式 RGBA8/BGRA8/R8/Depth24Stencil8 | ✅ | `TestS1TextureFormatSkiaSubset` | 含 sRGB/F16/Depth32FloatStencil8 |
| C7 | MSAA texture + resolve | 🔄 豁免→S3 | | ABI 支持 SampleCount；resolve 语义在 render S3 验证 |

## D. Shader / Bind / Pipeline（M0–M2）

| 项 | 内容 | 状态 | 测试 | 备注 |
|----|------|------|------|------|
| D1 | CreateShaderModule WGSL | ✅ | `TestS1AE_DE_*` + shader 测试 | |
| D2 | BindGroupLayout/BindGroup | ✅ | `TestS1AE_D_*` + bindgroup | binding type 转换已锁 |
| D3 | PipelineLayout | ✅ | `TestS1AE_D_*` | |
| D4 | CreateRenderPipeline | ✅ | `TestS1AE_DE_*` 真 draw + enum | topology/front/cull wire 用 toWGPU* |
| D5 | DepthStencilState + StencilOp | ✅ | enum lock + `TestRenderPipelineWithDepth` | 深度管线创建已有；路径填充语义在 S3 |
| D6 | BlendState factors/ops | ✅ enum | `TestS1Identity*Blend*` | BlendFactor 0x00–0x0D / BlendOp 全表 |
| D7 | MultisampleState | ✅ | `TestS1AE_DE_*` Count=1 | MSAA>1 → S3 |
| D8 | CreateComputePipeline | ✅ | `TestCreateComputePipeline` / `TestFullComputeExample` | |

## E. Command / Pass / Draw（M0–M2）

| 项 | 内容 | 状态 | 测试 | 备注 |
|----|------|------|------|------|
| E1 | CreateCommandEncoder / Finish | ✅ | `TestS1AE_*` | |
| E2 | BeginRenderPass attachments load/store | ✅ | `TestS1AE_DE_*` LoadOpClear/StoreOpStore | |
| E3 | SetPipeline/BindGroup/Vertex/Index | ✅ | `TestS1AE_DE_*` / `TestS1AE_E_DrawIndexed` | |
| E4 | Draw / DrawIndexed | ✅ | `TestS1AE_DE_*` / `TestS1AE_E_DrawIndexed` 像素验证 | |
| E5 | DrawIndirect* | 🔄 豁免→M3 | `TestDrawIndirectArgs` 等 | 非 M0–M2 阻塞 |
| E6 | SetViewport/Scissor/StencilRef/BlendConstant | ✅ | `TestS1AE_DE_DrawReadback` | |
| E7 | BeginComputePass / Dispatch | ✅ | `TestComputePassDispatch` / `TestFullComputeExample` | |
| E8 | QueueSubmit | ✅ | | |
| E9 | Debug marker/group | ✅ | `TestCommandEncoderDebugMarkers` | 非阻塞 |

## F. Surface / Present（M3，须具备绑定）

| 项 | 内容 | 状态 | 测试 | 备注 |
|----|------|------|------|------|
| F1 | 平台 CreateSurface | 🔄 | surface_* | linux/darwin/windows 文件存在 |
| F2 | Configure / GetCurrentTexture / Present | 🔄 | | render 尚未完整消费 |
| F3 | GetCapabilities | 🔄 | | |

## G. Enum / Struct 严格性（贯穿 S1）— **本轮重点**

| 项 | 内容 | 状态 | 测试 / 备注 |
|----|------|------|-------------|
| G1 | PrimitiveTopology 映射 | ✅ | `toWGPUPrimitiveTopology` + `TestS1Converted*` + `TestS1ConvertedEnumsRejectSilentIdentity` |
| G2 | FrontFace / CullMode | ✅ | `toWGPUFrontFace` / `toWGPUCullMode` + 同上 |
| G3 | TextureFormat（Skia 子集） | ✅ | `TestS1TextureFormatSkiaSubset`（身份 cast） |
| G4 | BlendFactor/BlendOp | ✅ | `TestS1IdentityEnumsMatchWebGPUHeader` |
| G5 | StencilOperation/CompareFunction | ✅ | 同上 |
| G6 | LoadOp/StoreOp | ✅ | 同上 |
| G7 | VertexFormat / IndexFormat / VertexStepMode | ✅ | `toWGPUVertexFormat` / `toWGPUVertexStepMode` + Index 身份 |
| G8 | Binding types（Buffer/Sampler/TextureSample/StorageAccess） | ✅ | `toWGPU*` +1 映射 + `TestS1Converted*` |
| G9 | Filter/Mipmap/Address/TextureDim/Aspect/ColorWrite | ✅ | 身份映射 `TestS1Identity*` |
| G10 | 关键 wire struct size/offset | ✅ | `abi_test` wire/struct | 可随 header 升级扩展 |

### 转换 vs 身份（必须牢记）

| 类 | 枚举 | wire 策略 |
|----|------|-----------|
| **必须 convert** | PrimitiveTopology, FrontFace, CullMode, VertexFormat, VertexStepMode, BufferBindingType, SamplerBindingType, TextureSampleType, StorageTextureAccess | 仅 `toWGPU*` |
| **身份 + 表测锁定** | LoadOp, StoreOp, Blend*, Compare, StencilOp, Filter*, Address, IndexFormat, TextureFormat, TextureDim/View/Aspect, ColorWriteMask | `uint32(enum)` 允许，但禁止无测试 |

---

## NewProc 绑定覆盖（子集符号，粗计）

`gpu/rwgpu` 当前约 **132** 个 `NewProc("wgpu…")`，覆盖 CreateInstance/Adapter/Device/Queue、Buffer/Texture/Sampler、Shader/Bind/Pipeline、CommandEncoder、Render/Compute pass draw/dispatch、Copy*、Surface Configure/Present、ErrorScope、Poll 等。  
**A–E 无 ⬜ 函数级缺失**（功能/descriptor 深度审计仍为 🔄）。

---

## S1 退出检查

- [x] A–E 无 ⬜（F 可为 🔄 但 API 存在）  
- [x] G 中 M0–M2 相关 ❓ 清零（`TestS1*`）  
- [x] `go test ./gpu/rwgpu` 全绿（含 `WGPU_NATIVE_PATH`）  
- [x] 能力表 §2 回写绑定状态（§2.7 + §4）  
- [x] A–E 功能路径 native 烟测（`TestS1AE*` 读回像素/字节）  

**S1 整体：✅ 关闭（Skia 2D 子集）**  

门禁：

```bash
export WGPU_NATIVE_PATH=/path/to/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache
go test -count=1 ./gpu/rwgpu
```

书面豁免（不挡 M2 / 进入 S2）：

- F Surface present 完整消费 → M3 / S3c  
- E5 DrawIndirect* 深度 → M3  
- C7 MSAA resolve 语义 → S3  
- 同 Instance 并发 RequestAdapter（GLES abort）→ 默认 skip  

下一阶段：**S2** `gpu/webgpu` facade。

## 修订

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-15 | 0.3 | S1 收尾：`s1_ae_smoke_test.go` A–E 真链路烟测；S1 ✅ 关闭 |
| 2026-07-15 | 0.2 | S1 enum header-lock：`s1_skia_subset_enum_test.go`；G1–G9 ✅；转换路径审计 |
| 2026-07-15 | 0.1 | 初稿：按能力表反推 + 仓库现状粗标 |
