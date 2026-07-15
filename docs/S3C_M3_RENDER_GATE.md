# S3c — Render M3 高级 2D / Present 门禁

> 版本：0.4 | 日期：2026-07-15  
> 主线：[`MAINLINE_PLAN.md`](./MAINLINE_PLAN.md) S3c  
> 架构：`render.Context → accelerator → gpu/webgpu → gpu/rwgpu → libwgpu_native`  
> 能力基线：[`SKIA_2D_CAPABILITY_MATRIX.md`](./SKIA_2D_CAPABILITY_MATRIX.md) M3

## 范围（M3）

| 能力 | 状态 | 门禁 |
|------|------|------|
| …（既有 filter/vertices/atlas/clip/text 等） | ✅ | `TestS3c_M3_*` residual 套件 |
| S.03 Offscreen present | ✅ | `TestS3c_M3_OffscreenPresentPath` |
| S.03 PresentFrame 编排 | ✅ | `TestS3c_M3_PresentFrame_Offscreen` / `Guards` |
| S.03 Swapchain API | ✅ | `TestSwapchain_*`（`gpu/webgpu`） |
| S.03 窗口 Swapchain Present | ✅ API + 空句柄防护；e2e 需 DISPLAY | `TestSwapchain_WindowPresentE2E`（无 DISPLAY → skip） |
| F.03 filter DAG | ⬜ M4 | |
| K.01 Compute 路径 | 🔄 部分 vello | 可选增强 |

## S.03 窗口 Present 路径

```text
Instance.CreateSurface(display, window)   // 空句柄 → Go error（禁止 native abort）
  → Adapter.GetSurfaceCapabilities
  → webgpu.NewSwapchain(...).ConfigureFromCapabilities
  → frame := swapchain.BeginFrame()       // GetCurrentTexture + CreateView
  → draw into render.Context
  → dc.PresentFrame(frame.Handle, w, h, func() error {
        return swapchain.EndFrame(frame)  // Surface.Present
    })
```

关键文件：

- `gpu/webgpu/swapchain.go` — Configure / BeginFrame / EndFrame  
- `gpu/webgpu/surface.go` + `surface_linux.go` — 空句柄防护  
- `render/present.go` — `PresentFrame` / `PresentFrameDamage`  

## 硬规则

1. 声称 GPU：`GPUOps>0` + `WGPU_NATIVE_PATH`  
2. 像素/语义检查（离屏路径）  
3. 窗口 e2e：有可用 X11/Wayland/HWND 时跑；CI headless 允许 skip  
4. `CreateSurface(0,0)` **不得** native abort  

## 门禁命令

```bash
export WGPU_NATIVE_PATH=/path/to/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache

go test -count=1 ./gpu/webgpu -run 'TestSwapchain_'
go test -count=1 ./render -run 'TestS3c_|TestS3b_|TestS3a_|TestP12GPUFixedPixel'
```

## 退出检查

- [x] PresentFrame 离屏真链路  
- [x] Swapchain API + 空句柄防护  
- [x] 窗口 e2e 测试存在（DISPLAY 可用时执行）  
- [x] 能力表 S.03 回写  
- [x] 已知 headless 限制写回本表  

**S3c Present：✅ 关闭（窗口 e2e 在无 display 时 skip，属环境限制）**

## 修订

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-15 | 0.4 | S.03 Swapchain + PresentFrame + 空句柄防护 + X11 e2e |
| 2026-07-15 | 0.3 | M3 residual 清零 |
| 2026-07-15 | 0.2 | V.01/V.02 |
| 2026-07-15 | 0.1 | filter/present 首切片 |
