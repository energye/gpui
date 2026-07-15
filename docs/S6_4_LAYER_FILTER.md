# S6.4 — Layer / Backdrop / Filter

> 版本：1.0 | 日期：2026-07-15  
> 状态：**S6.4 关闭**  
> 依赖：S6.3 batch deepen + S5 帧模型  
> 架构：`PushLayer/PushBackdropLayer` → pooled Pixmap → Pop composite；filter 中间 RT 池

---

## 1. 目标

降低 **每帧 layer / backdrop / filter** 中间表面成本：

1. **PushLayer**：不再每次 `NewPixmap` 全幅分配  
2. **PopLayer**：层表面回池复用  
3. **PushBackdropLayer**：跳过无用 Clear（整层被 snapshot 覆盖）  
4. **Filter 中间 RT**：`applyFilterInPlace` / CPU filter graph ping-pong 走共享池  
5. **滤镜图节点**：丢弃 no-op；合并连续 **ColorMatrix**（`composeColorMatrix4x5`）  
6. **禁止**为刷分改像素语义 / silent CPU  

**非目标**：控件层、把 layer 整段搬到 GPU（那是更深层架构，可后置）。

---

## 2. 关键改动

| 组件 | 改动 |
|------|------|
| `pixmapPool` | 按 (w,h) 复用；`Get` 清透明；`GetForOverwrite` 供 backdrop/filter |
| `layerStack.pool` | 从无用的 ImageBuf pool → **pixmapPool** |
| `PushLayer` / `PushMaskLayer` / `PopLayer` | acquire / put |
| `PushBackdropLayer` | `pushLayerSurface(..., clear=false)` + copy |
| `filterPixmapPool` | blur/shadow/graph 中间缓冲 |
| `coalesceImageFilterNodes` | matrix 折叠 |

### 诊断

```go
gets, puts, hits, misses := dc.LayerPoolStats()
gets, puts, hits, misses := render.FilterPoolStats()
```

---

## 3. 测试

| 测试 | 作用 |
|------|------|
| `TestS64_LayerPool_ReusesSurfaces` | 6× push/pop → hits≥4 |
| `TestS64_FilterPool_ReusesIntermediates` | 5× blur → hits≥3 |
| `TestS64_ComposeColorMatrix_*` / coalesce | 矩阵合成 |
| `TestS64_BackdropSnapshot_UsesPool` | backdrop 走池 |
| `TestS64_LayerPresent_NoRegress` | 真 GPU present 软预算 |
| 既有 `TestPushPopLayer` / nested | 语义不回退 |

```bash
export WGPU_NATIVE_PATH=.../lib/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache
export LD_LIBRARY_PATH=.../lib:$LD_LIBRARY_PATH

go test -count=1 ./render -run 'TestS64_|TestPushPopLayer|TestNestedLayers' -timeout 120s
go test -count=1 ./render -run 'TestS6_L0_|TestS52_|TestS53_|TestS61_|TestS62_|TestS63_' -timeout 300s
go test -count=1 ./render -run 'TestP1_Comp_(D01|D06|D08|D36|D63|D152)_' -timeout 300s
```

---

## 4. 退出条件

| 条件 | 状态 |
|------|------|
| Layer 表面池化 + Pop 回收 | ✅ |
| Backdrop 无无用 Clear | ✅ |
| Filter 中间 RT 池化 | ✅ |
| ColorMatrix 节点合并 | ✅ |
| L0/L1 抽样绿；layer 语义测试绿 | ✅ |
| 无 silent CPU / 无降像素门禁 | ✅ |

**S6.4 关闭。** 下一：**S6.5 Text / Shaping / Atlas**。
