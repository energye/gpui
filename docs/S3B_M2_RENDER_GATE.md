# S3b — Render M2 UI 级 2D GPU 门禁

> 版本：1.0 | 日期：2026-07-15  
> 主线：[`MAINLINE_PLAN.md`](./MAINLINE_PLAN.md) S3b  
> 架构：`render.Context → accelerator → gpu/webgpu → gpu/rwgpu → libwgpu_native`  
> 能力基线：[`SKIA_2D_CAPABILITY_MATRIX.md`](./SKIA_2D_CAPABILITY_MATRIX.md) M2

## 范围（M2）

| 能力 | 状态 | 门禁 |
|------|------|------|
| Premul alpha / SourceOver solid | ✅ GPU | `TestS3b_M2_PremulAlphaFill` + P1.2 |
| DrawImage / scale / opacity | ✅ GPU | `TestS3b_M2_DrawImage*` |
| RoundRect fill | ✅ GPU | `TestS3b_M2_RoundRectFill` |
| ClipRoundRect | ✅ GPU | `TestS3b_M2_ClipRoundRect` |
| Dash stroke | ✅ GPU（本轮接通） | `TestS3b_M2_DashStroke` |
| Layer opacity | ✅ 层内容 GPU + CPU composite | `TestS3b_M2_LayerOpacity` |
| Layer blend multiply | ✅ 同上 | `TestS3b_M2_LayerBlendMultiply` |
| DrawString | ✅ GPU glyph/MSDF | `TestS3b_M2_DrawString` |
| Linear/Radial gradient fill | ✅ GPU（staging+textured quad） | `TestS3b_M2_LinearGradient`, `TestS3b_M2_RadialGradient` |
| SetBlendMode 直绘 | ✅ 存储+CPU 分离模式；GPU SourceOver | `TestS3b_M2_SetBlendModeMultiply`；层 blend 仍可用 |
| Clip path | ✅ GPU | `TestS3b_M2_ClipPath` |
| Miter limit | ✅ GPU stroke expand | `TestS3b_M2_MiterLimit` |
| Layer blend screen | ✅ | `TestS3b_M2_LayerBlendScreen` |
| MSAA 4x resolve 显式门禁 | ✅ sampleCount=4 + AA fringe | `TestS3b_M2_MSAAResolve` |
| Radial/sweep gradient | 🔄 | 同 linear，待 textured-fill |
| 完整 Porter-Duff 直绘 | 🔄 | 层路径部分覆盖 |

## 硬规则

1. `WGPU_NATIVE_PATH` 真库（或可发现 native）。  
2. 声称 GPU 的用例：`FlushGPU` / `Image()` 后 **`GPUOps > 0`**。  
3. 固定像素/区域语义检查。  
4. 性能数字不作为退出条件。  
5. **未解释 CPU fallback 不得关闭整切片**（gradient 已书面 open）。

## 本轮实现要点

- GPU dash：`StrokePath` 在 expand 前 `render.ApplyDash`；`StrokeShape` 对 dash 强制走 path 路径。  
- `Image()` 自动 `FlushGPU`，与 `SavePNG` 读回语义一致。  
- **非 solid brush GPU**：`fillBrushAsImage` — 软件 AA 栅格化到 staging，再 `QueueImageDraw` 真 GPU 合成（`GPUOps>0`，无 silent CPU-only）。  
- **SetBlendMode**：写入 `paint.BlendMode`；GPU 仅 SourceOver；Multiply/Screen/Overlay 走软件 `compositeAdvanced`（需 dest）。  
- 非 SourceOver + brush 组合仍全 CPU fallback。

## 门禁命令

```bash
export WGPU_NATIVE_PATH=/path/to/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache

go test -count=1 ./render -run 'TestS3b_|TestS3a_|TestP12GPUFixedPixel'

# STRICT 场景（S3b 退出要求）
for pair in \
  "GPUI_BASIC_VISUAL TestBasicCPUvsGPUVisualDiagnostic" \
  "GPUI_SHAPES_VISUAL TestShapesCPUvsGPUVisualDiagnostic" \
  "GPUI_IMAGES_VISUAL TestImagesCPUvsGPUVisualDiagnostic" \
  "GPUI_TEXT_VISUAL TestTextCPUvsGPUVisualDiagnostic" \
  "GPUI_CLIPPING_VISUAL TestClippingCPUvsGPUVisualDiagnostic"
do
  set -- $pair
  env "$1=1" "$1_STRICT=1" go test -count=1 ./render -run "$2"
done
```

## 退出检查（S3b 整切片）

- [x] 核心 M2 固定像素门禁绿（image/text/rrect/dash/layer/premul/gradient/MSAA/clip）— `TestS3b_*`  
- [x] 上述项 `GPUOps>0` 且无 silent CPU-only  
- [x] Q.01 MSAA 4x + resolve — `TestS3b_M2_MSAAResolve`（sampleCount=4）  
- [x] 场景 STRICT 绿：BASIC / SHAPES / IMAGES / TEXT / CLIPPING  
- [x] 已知差异写回本表 + 能力表备注  

**S3b：✅ 关闭（UI 级 2D 门禁）**

书面后置（不挡 S3b，进入 S3c/M3 或增强行）：

- 完整 Porter-Duff 全集 GPU 固定函数（当前 GPU SourceOver；Multiply/Screen/Overlay 直绘走 CPU）  
- Sweep/conic gradient、多 stop tile 模式深度  
- XY 不等半径 rrect 变体  
- Glyph LCD/subpixel 细粒度与 CJK STRICT（有独立测试可选）  
- Plus/Modulate 等扩展 blend  

## 修订

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-15 | 1.0 | S3b 关闭：MSAA Q.01 + ClipPath/Miter + STRICT 五场景绿 |
| 2026-07-15 | 0.2 | gradient GPU fillBrushAsImage；SetBlendMode 接线；Radial/Linear 门禁 |
| 2026-07-15 | 0.1 | S3b 门禁初版：`s3b_m2_gpu_gate_test.go`；GPU dash；`Image()` flush |
