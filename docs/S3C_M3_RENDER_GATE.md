# S3c — Render M3 高级 2D / Present 门禁

> 版本：0.1 | 日期：2026-07-15  
> 主线：[`MAINLINE_PLAN.md`](./MAINLINE_PLAN.md) S3c  
> 架构：`render.Context → accelerator → gpu/webgpu → gpu/rwgpu → libwgpu_native`  
> 能力基线：[`SKIA_2D_CAPABILITY_MATRIX.md`](./SKIA_2D_CAPABILITY_MATRIX.md) M3

## 范围（M3 首切片）

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
| 窗口 Swapchain Present | 🔄 webgpu Surface API 已有 | 需真实窗口/display 集成后置 |
| F.03 完整 filter DAG | ⬜ | M4 |
| V.01 DrawVertices | 🔄/⬜ | 后续切片 |

## 实现要点

- `render/filter_ops.go`：Context filter API + 注册钩子（避免 `render ↔ internal/filter` 循环依赖）  
- `render/filters`：blank-import 注册 blur/shadow/color matrix  
- Filter 在 **FlushGPU 后** 于 pixmap 上执行；内容来自真 GPU 读回  
- Present 路径：离屏 texture view（窗口less 等价于 S.03 的 render-to-surface）

## 硬规则

1. GPU 相关门禁：`GPUOps>0`  
2. 像素/语义检查  
3. 性能不作为退出条件  

## 门禁命令

```bash
export WGPU_NATIVE_PATH=/path/to/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache

go test -count=1 ./render -run 'TestS3c_|TestS3b_|TestS3a_|TestP12GPUFixedPixel'
```

## 退出检查（S3c 整切片）

- [x] M3 首切片固定像素/语义门禁绿 — `TestS3c_*`  
- [ ] 窗口 Surface/Swapchain 端到端 Present  
- [ ] DrawVertices / DrawAtlas 门禁  
- [ ] 能力表 M3 必选行清零或书面后置  
- [x] 已知差异写回本表  

**S3c：🔄 进行中（首切片门禁落地；窗口 Present / Vertices 未关）**

## 修订

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-15 | 0.1 | S3c 启动：filter/shadow/color + offscreen present + recording 门禁 |
