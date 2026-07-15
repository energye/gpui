# S6.3 — 绘制合并加深

> 版本：1.0 | 日期：2026-07-15  
> 状态：**S6.3 关闭**  
> 依赖：S4.1 image multi-quad + S6.2 submit path  
> 架构：`Queue*` coalesce → `RenderFrameGrouped` per-group deepen → `build*Resources` multi-draw

---

## 1. 目标

在 **不跨 clip / blend / scissor** 的前提下，加长同类 draw run：

| 层级 | S4.1 状态 | S6.3 |
|------|-----------|------|
| Image | multi-quad + seal | 保留；stats 并入 `BatchDrawStats` |
| GPUTexture overlay | 1 draw/cmd | **multi-quad + scissor seal** |
| Text / GlyphMask | Queue `CanMerge` | **per-scissor-group 二次 coalesce** |
| SDF | 已 1 draw/多 shape | 统计 `SDFDraws=1` |
| Convex | blend-range multi-draw | 统计 `ConvexDraws` |

**禁止**：跨 scissor 合并、跨不同 texture/view/opacity/color/transform 合并、silent CPU。

---

## 2. 关键改动

### 2.1 GPUTexture multi-quad

- `canMergeGPUTextureDraw`：同 `View` 指针 + opacity + viewport  
- `buildGPUTextureResources(..., batchSeal)`：与 image 相同 run 扩展  
- `RenderFrameGrouped` 在每个 group 的 `gpuTexStart` 置 seal  

### 2.2 Text / Glyph 组内加深

```text
for each ScissorGroup:
  TextBatches = coalesceTextBatches(TextBatches)
  GlyphMaskBatches = coalesceGlyphMaskBatches(GlyphMaskBatches)
```

- 仅 `CanMerge` 为真时合并 quads  
- **每 group 独立** → 天然不跨 scissor（相对全局 seal 更安全）  
- Quad 拷贝，避免 alias pending  

### 2.3 `BatchDrawStats`

```go
type BatchDrawStats struct {
  ImageDraws, ImageQuads int
  GPUTexDraws, GPUTexQuads int
  TextDraws, TextQuads int
  GlyphDraws, GlyphQuads int
  SDFDraws, SDFShapes int
  ConvexDraws, ConvexCmds int
}
// session.LastBatchDrawStats() / GPURenderContext.LastBatchDrawStats()
```

---

## 3. 测试

| 测试 | 作用 |
|------|------|
| `TestS63_CanMergeGPUTextureDraw` | merge 键 |
| `TestS63_GPUTexture_MultiQuadLogic` | seal 切 run |
| `TestS63_CoalesceTextBatches_*` / Glyph | 组内加深 |
| `TestS63_ImageSealStillRespected` | S4.1 不回退 |
| `TestS63_BatchStats_AfterFlush` | 真 GPU：5 SDF → 1 draw |
| `TestS63_PresentMainPath_NoRegress` | present p50 不回退 |

```bash
export WGPU_NATIVE_PATH=.../lib/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache
export LD_LIBRARY_PATH=.../lib:$LD_LIBRARY_PATH

go test -count=1 ./render/internal/gpu -run 'TestS63_|TestS41_|TestS62_' -timeout 180s
go test -count=1 ./render -run 'TestS63_|TestS62_|TestS61_|TestS6_L0_|TestS52_|TestS53_' -timeout 300s
go test -count=1 ./render -run 'TestP1_Comp_(D01|D06|D08|D36|D63|D152)_' -timeout 300s
```

---

## 4. 退出条件

| 条件 | 状态 |
|------|------|
| GPUTexture multi-quad + seal | ✅ |
| Text/Glyph 组内 deepen | ✅ |
| 不跨 scissor / 像素路径不降语义 | ✅ |
| BatchDrawStats 可观测 | ✅ |
| L0/L1 抽样绿；主路径 60fps 不回退 | ✅ |

**S6.3 关闭。** 下一：**S6.4 Layer / Backdrop / Filter**。
