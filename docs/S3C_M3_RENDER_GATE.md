# S3c — Render M3 高级 2D / Present 门禁

> 版本：0.2 | 日期：2026-07-15  
> 主线：[`MAINLINE_PLAN.md`](./MAINLINE_PLAN.md) S3c  
> 架构：`render.Context → accelerator → gpu/webgpu → gpu/rwgpu → libwgpu_native`  
> 能力基线：[`SKIA_2D_CAPABILITY_MATRIX.md`](./SKIA_2D_CAPABILITY_MATRIX.md) M3

## 范围（M3）

| 能力 | 状态 | 门禁 |
|------|------|------|
| F.01 Blur | ✅ Context `ApplyBlur` + filter 注册 | `TestS3c_M3_ApplyBlur` |
| F.02 Drop shadow | ✅ `ApplyDropShadow` | `TestS3c_M3_ApplyDropShadow` |
| F.04 Color filter | ✅ `ApplyGrayscale` / matrix | `TestS3c_M3_ApplyGrayscale` |
| S.03 Offscreen present | ✅ `CreateOffscreenTexture` + `FlushGPUWithView` | `TestS3c_M3_OffscreenPresentPath` |
| S.09 Damage present | ✅ `FlushGPUWithViewDamage` | `TestS3c_M3_DamagePresentPath` |
| H.05 Path measure | ✅ `Path.Length` | `TestS3c_M3_PathLength` |
| R.01 Recording 回放 | ✅ recording + raster backend | `TestS3c_M3_RecordingPlayback` |
| CS.01 默认 8bit sRGB 表面 | ✅ mid-gray 往返 | `TestS3c_M3_DefaultSRGBSurface` |
| V.01 DrawVertices | ✅ Gouraud mesh via convex tier | `TestS3c_M3_DrawVertices` |
| V.02 DrawAtlas | ✅ multi `QueueImageDraw` | `TestS3c_M3_DrawAtlas` |
| S.03 Present API smoke | ✅ windowless offscreen stand-in | `TestS3c_M3_SurfacePresentAPISmoke` |
| 窗口 Swapchain Present | 🔄 webgpu Surface API 已有 | 需真实窗口/display；书面后置 |
| F.03 完整 filter DAG | ⬜ | M4 / 后置 |

## 实现要点

- `render/filter_ops.go`：Context filter API + 注册钩子（避免 `render ↔ internal/filter` 循环依赖）  
- `render/filters`：blank-import 注册 blur/shadow/color matrix  
- Filter 在 **FlushGPU 后** 于 pixmap 上执行；内容来自真 GPU 读回  
- Present 路径：离屏 texture view（窗口less 等价于 S.03 的 render-to-surface）  
- **V.01**：`Context.DrawVertices` → `gpuContextOps.QueueColoredMesh` → `ConvexDrawCommand.VertexColors` → convex shader Gouraud  
- **V.02**：`Context.DrawAtlas` → 多 `QueueImageDraw` 共享同一 `ImageBuf` 像素上传源  

## 硬规则

1. GPU 相关门禁：`GPUOps>0`  
2. 像素/语义检查  
3. 性能不作为退出条件  
4. 真链路：`WGPU_NATIVE_PATH` + `libwgpu_native`  

## 门禁命令

```bash
export WGPU_NATIVE_PATH=/path/to/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache

go test -count=1 ./render -run 'TestS3c_|TestS3b_|TestS3a_|TestP12GPUFixedPixel'
```

## 退出检查（S3c 整切片）

- [x] M3 首切片固定像素/语义门禁绿 — `TestS3c_*`  
- [x] DrawVertices / DrawAtlas 门禁 — `TestS3c_M3_DrawVertices` / `TestS3c_M3_DrawAtlas`  
- [x] Offscreen / damage / present smoke — windowless 路径  
- [ ] 窗口 Surface/Swapchain 端到端 Present（**书面后置**：依赖 display/HWND 集成）  
- [x] 能力表 V.01/V.02/S.03 离屏 回写  
- [x] 已知差异写回本表  

**S3c：✅ 关闭（M3 必选门禁；窗口 Swapchain Present 书面后置）**

## 书面后置

| 项 | 原因 | 何时 |
|----|------|------|
| 真实窗口 Swapchain Present | 无 headless display 时不可稳定 CI | 平台窗口/surface 集成任务 |
| F.03 filter DAG | 非 UI 2D 阻塞 | M4 |

## 修订

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-15 | 0.2 | V.01 DrawVertices + V.02 DrawAtlas GPU 门禁；S3c 关闭（窗口 Present 后置） |
| 2026-07-15 | 0.1 | S3c 启动：filter/shadow/color + offscreen present + recording 门禁 |
