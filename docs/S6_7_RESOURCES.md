# S6.7 — 上传 / 资源 / 内存

> 版本：1.0 | 日期：2026-07-15  
> 状态：**S6.7 关闭**  
> 依赖：S4.3 ImageCache、TexturePool、S4.2 glyph upload  
> 架构：`render → gpu/webgpu → gpu/rwgpu → libwgpu_native`

---

## 1. 目标

生产级 **资源驻留与上传诊断**：

1. ImageCache **entry + 字节预算** + 上传字节统计  
2. **Generation 失效** 正确分槽（ADR-014）  
3. **gen=0 临时纹理** 帧末 `ReleaseEphemeral`（防泄漏）  
4. 非紧 stride 上传 **staging scratch 池**  
5. TexturePool **hit/miss + EndFrame 全局 budgetMB**  
6. 聚合 `ResourceUploadStats` / 扩展 `GPUMemoryStats`  

**非目标**：控件层；大改 swapchain；PDF/YUV。

---

## 2. 实现摘要

| 组件 | 改动 |
|------|------|
| `ImageCache` | budget 128 entries / 64MiB；`UploadBytes`；LRU 字节驱逐；`ReleaseEphemeral` |
| staging pool | `sync.Pool` 打包非紧 stride |
| `TexturePool` | Hits/Misses/Releases；EndFrame 压预算 |
| `RenderSession` Flush | defer `ReleaseEphemeral` |
| `SDFAccelerator` | `ResourceUploadStats` / `ImageCacheStatsFromDefault` |

### 诊断

```go
st := accel.ResourceUploadStats()
// st.Image.Hits / UploadBytes / UsedBytes / Evictions
// st.Texture.Hits / Misses / UsageMB
// st.Memory.TexturePoolMB
```

---

## 3. 验证（真 GPU）

| 测试 | 结果 |
|------|------|
| `TestS67_ImageCache_HitUploadBytes` | ✅ |
| `TestS67_ImageCache_GenerationInvalidation` | ✅ |
| `TestS67_ImageCache_ByteBudgetEvicts` | ✅ |
| `TestS67_ImageCache_EphemeralReleased` | ✅ |
| `TestS67_ImageCache_StrideStaging` | ✅ |
| `TestS67_PresentImageGrid_NoRegress` | ✅ **p50≈2.85ms** |
| `TestS43_ImageCache_Stats` | ✅ |

---

## 4. 复现

```bash
export WGPU_NATIVE_PATH=.../lib/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache
export LD_LIBRARY_PATH=.../lib:$LD_LIBRARY_PATH

go test -count=1 ./render/internal/gpu -run 'TestS67_|TestS43_Image' -timeout 120s
go test -count=1 ./render -run 'TestS67_|TestS6_L0_' -timeout 300s
```

---

## 5. 退出条件

| 条件 | 状态 |
|------|------|
| 上传字节/次数诊断 | ✅ |
| generation 失效 | ✅ |
| 无泄漏烟测（ephemeral） | ✅ |
| 纹理池预算 + stats | ✅ |
| L0/L1 抽样绿 | ✅ |

**S6.7 关闭。** 下一：**S6.8 真窗口 Present 管线**。

---

## 6. 修订

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-15 | 1.0 | ImageCache 字节预算+ephemeral；TexturePool stats；TestS67_* |
