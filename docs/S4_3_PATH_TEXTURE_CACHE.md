# S4.3 Path / Texture Cache — 几何与纹理复用

> 版本：1.0 | 日期：2026-07-15  
> 状态：**S4.3 本切片关闭**  
> 依赖：S4.0 基线、`docs/S4_1_BATCH.md`、`docs/S4_2_GLYPH_ATLAS.md`  
> 架构：`render → gpu/webgpu → gpu/rwgpu → libwgpu_native`

---

## 1. 目标

在**不改像素语义**的前提下，复用：

1. **Path 三角化结果**（stencil-then-cover fan vertices + cover quad）  
2. **Stroke 扩展结果**（stroke → filled outline path）  
3. **Image 纹理上传**（既有 `ImageCache`，补齐 hit/miss/upload 统计）

服务对象：列表滚动、表单重绘、retained Context 多帧（为 S4.4 铺路）。

---

## 2. 实现

### 2.1 `PathGeometryCache`（`render/internal/gpu/path_geometry_cache.go`）

| 项 | 说明 |
|----|------|
| Key | FNV path verbs/coords + `FillRule` + `aaOff` |
| Value | fan `[]float32` + cover quad |
| Budget | 256 entries，LRU-by-gen 淘汰 |
| API | `GetOrTessellate` / `Stats` / `Clear` |
| 挂载 | `GPUShared.PathGeomCache()` 懒创建，跨 `GPURenderContext` 共享 |

`FillPath` 在 convex 快路径未命中时走 stencil-then-cover，优先读 cache；miss 时 tessellate 并写入。

### 2.2 `StrokeGeometryCache`

| 项 | 说明 |
|----|------|
| Key | path hash + width/cap/join/miter + dash hash + `aaOff` |
| Value | 扩展后 path 的 verbs/coords（克隆回放） |
| Budget | 128 entries |
| 挂载 | `GPUShared.StrokeGeomCache()` |

`StrokePath`：dash/snap 后查 cache → miss 则 `StrokeExpander.Expand` → `Put` → 再 `FillPath`（EvenOdd）。

### 2.3 `ImageCache` 统计

`ImageCacheStats` 增加：

- `Hits` / `Misses` / `Uploads`（连同既有 `Entries` / `Budget` / `Generations`）

上传键仍为 `Pixmap.GenerationID()`；`GenerationID==0` 不入缓存。

---

## 3. 调用链（真实 GPU 主路径）

```
Context.Stroke/Fill
  → GPURenderContext.StrokePath / FillPath
    → StrokeGeomCache.Get/Put          (stroke only)
    → PathGeomCache.GetOrTessellate    (stencil path)
    → QueueStencil / QueueConvex
  → FlushGPU → RenderFrameGrouped → webgpu → rwgpu → libwgpu_native

Image draw
  → ImageCache.GetOrUpload → QueueWriteTexture (miss only)
```

**硬规则**：cache 只省 CPU tess/expand/upload；submit 仍走真 native；`GPUOps>0` 不变。

---

## 4. 测试

| 测试 | 作用 |
|------|------|
| `TestS43_PathGeometryCache_HitMiss` | tess miss→hit，fill-rule 分槽 |
| `TestS43_StrokeGeometryCache_HitMiss` | stroke expand 存取 |
| `TestS43_ImageCache_Stats` | 真 device 上传 + hit 计数 |
| `TestS44_SharedPathCacheAcrossContexts` | GPUShared 单例 cache + retained 命中 |
| `TestS44_StrokeCache_SharedRetainedHit` | shared stroke cache retained |
| Harness | `B10` / `B12` / `B14_RetainedPathText` |

```bash
export WGPU_NATIVE_PATH=/home/yanghy/app/projects/gogpu/gpui/lib/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache
export LD_LIBRARY_PATH=/home/yanghy/app/projects/gogpu/gpui/lib:$LD_LIBRARY_PATH

go test -count=1 ./render/internal/gpu -run 'TestS43_|TestS44_' -timeout 120s
```

---

## 5. 退出条件

| 条件 | 状态 |
|------|------|
| path 几何复用（fill tess） | ✅ |
| stroke 扩展复用 | ✅ |
| image 纹理 cache 统计 | ✅ |
| 真 native + 回归绿 | ✅ |
| 不改像素语义 / 无 silent CPU | ✅ |

**S4.3 关闭。** 下一焦点见 `docs/S4_4_DAMAGE_RETAINED.md`。

---

## 6. 修订记录

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-15 | 1.0 | Path/Stroke geometry cache + ImageCache stats；关闭 S4.3 |
