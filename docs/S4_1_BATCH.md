# S4.1 Batch — 同类 draw 合并

> 版本：1.0 | 日期：2026-07-15  
> 状态：**S4.1 本切片关闭**  
> 依赖：S4.0 基线 `docs/S4_PERF_BASELINE.md`  
> 架构：`render → gpu/webgpu → gpu/rwgpu → libwgpu_native`

---

## 1. 目标

在**不改像素语义**的前提下，合并同类 GPU draw，减少 bind group / Draw 次数。

本切片聚焦：**Image Tier 3 multi-quad coalescing**（同纹理 + 同 opacity/filter/viewport 的连续 quad）。

已有、本切片不重复建设：

| 路径 | 既有 batch |
|------|------------|
| SDF shapes | 单 pass 多 shape 顶点打包 + 1× Draw |
| Text / GlyphMask | ADR-031 `CanMerge` 相邻 batch 合并 |
| Convex | 按 blend mode range 合并 |

---

## 2. 实现

### 2.1 合并键（`canMergeImageDraw`）

相邻 `ImageDrawCommand` 可合并当且仅当：

- `GenerationID != 0` 且相同（Pixmap 内容身份）
- `Nearest` / `Opacity` / viewport / `ImgWidth` / `ImgHeight` 相同

### 2.2 Scissor seal

`RenderFrameGrouped` 在每个 scissor group 的 image 起始 index 置 `batchSeal[i]=true`，**禁止跨 scissor 合并**（否则 slice 后 scissor 错位）。

### 2.3 资源构建（`buildImageResources`）

- 顶点：一次分配 `N×6×stride`，整块 `WriteBuffer`
- 连续可合并 run → **1 bind group + 1 Draw(vertexCount=6×quads)**
- `sliceImageResources` 按 **顶点范围** 选取完全落入 group 的 drawCalls

### 2.4 调用路径

- `TexturedQuadPipeline.RecordDraws` / `RecordBlitDraws` 使用 `imageDrawVertexCount(dc)`
- 统计：`GPURenderSession.LastImageBatchStats()` → `(drawCalls, quads)`

### 2.5 测试

| 测试 | 作用 |
|------|------|
| `TestS41_CanMergeImageDraw` | 合并键边界 |
| `TestS41_SliceImageResourcesByVertexRange` | scissor 子范围 |
| `TestS41_BatchSealPreventsCrossGroupMergeLogic` | seal 切 run |
| `TestS41_MergeRunCounts64SameTexture` | 64 同纹理 → 1 run |
| 回归 D01–D08 / D36 / P1.2 / S3* | 像素与 `GPUOps>0` |

Harness 场景 **B13_ImageBatchNoClip**：64 同 `ImageBuf` 无 per-tile clip（S4.1 主输入）。B10 仍带 per-tile clip，正确保持 per-group 1 image draw。

---

## 3. 对比基线（定性）

| 场景 | S4.1 预期 | 说明 |
|------|-----------|------|
| B13 64 images no-clip | GPU Draw：64→**1**（同 group） | `gpu_ops` 仍计 64 次 Queue（路径计数 ≠ Draw 次数） |
| B10 per-tile clip | Draw 不跨组合并 | 正确性优先 |
| B02 200 rects | 无新增（SDF 已 1 Draw） | 留给 S4.x instance 顶点瘦身 |
| B01/B03 flush 主导 | wall-time 变化有限 | 读回/present 非本切片 |

本机回归后 wall-time 波动受机器负载影响大；**正确性门禁全绿** + **逻辑证明 64→1 run** 作为本切片退出条件。绝对 FPS 提升留给 present 无读回路径与 S4.2+。

---

## 4. 复现

```bash
export WGPU_NATIVE_PATH=/home/yanghy/app/projects/gogpu/gpui/lib/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache
export LD_LIBRARY_PATH=/home/yanghy/app/projects/gogpu/gpui/lib:$LD_LIBRARY_PATH

go test -count=1 ./render/internal/gpu -run 'TestS41_' -timeout 30s
go test -count=1 ./render -run 'TestP1_Comp_D0|TestP1_Comp_D36|TestP12GPUFixedPixel|TestS3a_|TestS4_PerfBaseline' -timeout 180s
```

---

## 5. 退出条件

| 条件 | 状态 |
|------|------|
| 同类 image draw 合并（同 scissor） | ✅ |
| 不跨 scissor / 像素回归绿 | ✅ |
| 单元测试覆盖 merge/seal/slice | ✅ |
| 基线对比文档 | ✅ 本文 |
| 禁止 silent CPU | ✅ |

**S4.1 本切片关闭。** 下一焦点：**S4.2 glyph/atlas**。

---

## 6. 修订

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-15 | 1.0 | Image multi-quad coalescing + B13 harness + S41 tests |
