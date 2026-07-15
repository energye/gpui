# rwgpu × Skia 2D WebGPU 子集检查表（S1 工作底稿）

> 版本：0.1 | 日期：2026-07-15  
> 来源：[`SKIA_2D_CAPABILITY_MATRIX.md`](./SKIA_2D_CAPABILITY_MATRIX.md) §2  
> 主线：[`MAINLINE_PLAN.md`](./MAINLINE_PLAN.md) S1  
> 状态图例：✅ 有绑定 | 🔄 有但待 header 严格审计 | ⬜ 缺失/不可用 | ❓ 未核实

---

## 审计方法

1. 以 `lib/webgpu.h` 为权威。  
2. 对照 `gpu/rwgpu` 的 `NewProc("wgpu…")`、Go API、`convert.go` wire。  
3. 每项关闭条件：映射正确 + 测试（单元或 native 烟测）+ 备注。  
4. **M0–M2 依赖项不得长期 ⬜。**

---

## A. 实例 / 适配器 / 设备 / 队列（M0）

| 项 | wgpu 符号（代表） | Go 侧 | 状态 | 测试 | 备注 |
|----|-------------------|-------|------|------|------|
| A1 | wgpuCreateInstance | CreateInstance | 🔄 | instance 测试 | 审计 descriptor flags |
| A2 | RequestAdapter | Instance.RequestAdapter | 🔄 | adapter 测试 | |
| A3 | RequestDevice | Adapter.RequestDevice | 🔄 | device 测试 | limits/features |
| A4 | DeviceGetQueue | Device.Queue | ✅ | | |
| A5 | DevicePoll | Device.Poll | 🔄 | | native 扩展 |
| A6 | ErrorScope push/pop | Push/PopErrorScope | 🔄 | errors 测试 | |
| A7 | Adapter/Device limits/features | Limits/Features/HasFeature | 🔄 | | |
| A8 | Release 全对象 | Release | 🔄 | leak debug | 生命周期 |

## B. Buffer（M0–M2）

| 项 | 内容 | 状态 | 测试 | 备注 |
|----|------|------|------|------|
| B1 | CreateBuffer + usage 全集 | 🔄 | buffer 测试 | MapRead/Write/Storage/… |
| B2 | QueueWriteBuffer | ✅ | | |
| B3 | MapAsync + GetMappedRange + Unmap | 🔄 | | v29 mapped range API |
| B4 | Destroy/Release | 🔄 | | |
| B5 | usage/size/mapState 查询 | 🔄 | | |

## C. Texture / View / Sampler / Copy（M0–M2）

| 项 | 内容 | 状态 | 测试 | 备注 |
|----|------|------|------|------|
| C1 | CreateTexture | 🔄 | | format 对齐 header |
| C2 | CreateView | 🔄 | | |
| C3 | QueueWriteTexture | 🔄 | render 侧 pitch 已测 | bytesPerRow=256 |
| C4 | CopyTextureToBuffer/ToTexture/B2T/B2B | 🔄 | | readback 关键 |
| C5 | CreateSampler | 🔄 | | address/filter |
| C6 | 格式 RGBA8/BGRA8/R8/Depth24Stencil8 | 🔄 | | sRGB/F16 → M3 |
| C7 | MSAA texture + resolve | 🔄 | | Q.01 |

## D. Shader / Bind / Pipeline（M0–M2）

| 项 | 内容 | 状态 | 测试 | 备注 |
|----|------|------|------|------|
| D1 | CreateShaderModule WGSL | 🔄 | | |
| D2 | BindGroupLayout/BindGroup | 🔄 | bindgroup 测试 | |
| D3 | PipelineLayout | 🔄 | | |
| D4 | CreateRenderPipeline | 🔄 | pipeline 测试 | **topology/frontFace/cull 已修一轮** |
| D5 | DepthStencilState + StencilOp | 🔄 | conversion 测试部分 | clip/path 关键 |
| D6 | BlendState factors/ops | 🔄 | | B.* 能力 |
| D7 | MultisampleState | 🔄 | | |
| D8 | CreateComputePipeline | 🔄 | | K.01 可选但已有 |

## E. Command / Pass / Draw（M0–M2）

| 项 | 内容 | 状态 | 测试 | 备注 |
|----|------|------|------|------|
| E1 | CreateCommandEncoder / Finish | 🔄 | command 测试 | |
| E2 | BeginRenderPass attachments load/store | 🔄 | clear_pass | |
| E3 | SetPipeline/BindGroup/Vertex/Index | 🔄 | | |
| E4 | Draw / DrawIndexed | 🔄 | | |
| E5 | DrawIndirect* | 🔄 | | M3/M4 |
| E6 | SetViewport/Scissor/StencilRef/BlendConstant | 🔄 | | |
| E7 | BeginComputePass / Dispatch | 🔄 | | |
| E8 | QueueSubmit | ✅ | | |
| E9 | Debug marker/group | 🔄 | | 非阻塞 |

## F. Surface / Present（M3，须具备绑定）

| 项 | 内容 | 状态 | 测试 | 备注 |
|----|------|------|------|------|
| F1 | 平台 CreateSurface | 🔄 | surface_* | linux/darwin/windows 文件存在 |
| F2 | Configure / GetCurrentTexture / Present | 🔄 | | render 尚未完整消费 |
| F3 | GetCapabilities | 🔄 | | |

## G. Enum / Struct 严格性（贯穿 S1）

| 项 | 内容 | 状态 | 备注 |
|----|------|------|------|
| G1 | PrimitiveTopology 映射 | 🔄 已修首轮 | 持续回归 |
| G2 | FrontFace / CullMode | 🔄 已修首轮 | |
| G3 | TextureFormat 全表 | ❓ | 对 header |
| G4 | BlendFactor/BlendOp | ❓ | |
| G5 | StencilOperation/CompareFunction | 🔄 部分 | |
| G6 | LoadOp/StoreOp | ❓ | |
| G7 | VertexFormat/IndexFormat | ❓ | |
| G8 | 关键 wire struct size/offset | 🔄 abi_test | 扩展覆盖 |

---

## S1 退出检查

- [ ] A–E 无 ⬜（F 可为 🔄 但 API 存在）  
- [ ] G 中 M0–M2 相关 ❓ 清零  
- [ ] `go test ./gpu/rwgpu` 全绿  
- [ ] 能力表 §2 回写绑定状态  

---

## 修订

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-15 | 0.1 | 初稿：按能力表反推 + 仓库现状粗标 |
