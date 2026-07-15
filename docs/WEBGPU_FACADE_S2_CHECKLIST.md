# gpu/webgpu facade × Skia 2D 子集检查表（S2）

> 版本：1.0 | 日期：2026-07-15  
> 主线：[`MAINLINE_PLAN.md`](./MAINLINE_PLAN.md) S2  
> 上游：[`RWGPU_SKIA_SUBSET_CHECKLIST.md`](./RWGPU_SKIA_SUBSET_CHECKLIST.md) S1 ✅  
> 架构：`render → gpu/webgpu → gpu/rwgpu → libwgpu_native`

## 目标

1. render **只**依赖 `gpu/webgpu`（禁止直 import `rwgpu`）。  
2. 子集 API 有 facade 对象方法；descriptor 字段不丢。  
3. 高风险 enum 经 facade 传到 rwgpu；**native 数值转换在 rwgpu（S1）完成**。  
4. 真 native 烟测经 facade 读回像素/字节。

## 门禁

```bash
export WGPU_NATIVE_PATH=/path/to/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache
go test -count=1 ./gpu/webgpu
go test -count=1 ./gpu/rwgpu   # S1 回归
```

## 测试映射

| 类别 | 文件 / `-run` |
|------|----------------|
| 转换字段完整性 | `s2_facade_conversion_test.go` → `TestS2Convert*` |
| A–E 真链路（facade） | `s2_ae_smoke_test.go` → `TestS2AE*` |
| 既有 | `device_conversion_test.go`（stencil/keepAlive） |

## 子集 facade API（S2）

| 组 | 方法（代表） | 状态 | 证据 |
|----|--------------|------|------|
| A Instance/Device | CreateInstance, RequestAdapter, RequestDevice, Queue, Poll, ErrorScope | ✅ | `TestS2AE_*` |
| B Buffer | CreateBuffer, WriteBuffer, Map/MappedRange, CopyB2B | ✅ | `TestS2AE_BufferWriteCopyMap` |
| C Texture | CreateTexture/View, WriteTexture, CopyT2B, Sampler | ✅ | `TestS2AE_TextureWriteCopyMap` |
| D Pipeline | Shader WGSL, BindGroupLayout/Group, PipelineLayout, Render/ComputePipeline | ✅ | conversion + draw smoke |
| E Encode | CommandEncoder, RenderPass Load/Store, Draw, Viewport/Scissor | ✅ | `TestS2AE_DrawReadback` |
| F Surface | Configure/Present | 🔄 M3 | 绑定存在；render 完整消费 S3c |

## 转换审计（高风险）

| 项 | 行为 | 状态 |
|----|------|------|
| Topology/Front/Cull | gputypes 值传入 rwgpu；wire 在 rwgpu `toWGPU*` | ✅ + `TestS2ConvertRenderPipelinePrimitiveFields` |
| UnclippedDepth | facade 传递；rwgpu 写入 wire | ✅ 本轮修复 |
| VertexFormat/StepMode | 传入 rwgpu；FFI 转换 | ✅ keepAlive + S1 |
| Blend factors/ops | 字段完整；wire Order=Op/Src/Dst | ✅ `TestS2ConvertFragmentBlendFields` |
| DepthStencil/StencilOp | 完整字段 | ✅ |
| Binding types | 传入 rwgpu；+1 在 rwgpu | ✅ |
| TextureView counts 0 | → `UINT32_MAX`（header UNDEFINED） | ✅ 本轮修复 |
| MipmapFilter 类型 | `MipmapFilterMode`（非 FilterMode） | ✅ 本轮修复 |
| render → rwgpu import | 无 | ✅ |

## S2 退出检查

- [x] 子集 facade 方法齐全（A–E）  
- [x] conversion 测试覆盖 topology/blend/stencil/vertex/binding  
- [x] `go test ./gpu/webgpu` 全绿（含 `WGPU_NATIVE_PATH`）  
- [x] render 不直 import `rwgpu`  
- [x] 文档本表  

**S2：✅ 关闭（Skia 2D 子集 facade）**  

下一阶段：**S3a** render M0–M1 对标。

## 修订

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-15 | 1.0 | S2 关闭：转换测试 + facade 烟测 + UnclippedDepth/view count/mipmap 修复 |
