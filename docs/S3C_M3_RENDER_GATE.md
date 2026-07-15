# S3c — Render M3 高级 2D / Present 门禁

> 版本：0.3 | 日期：2026-07-15  
> 主线：[`MAINLINE_PLAN.md`](./MAINLINE_PLAN.md) S3c  
> 架构：`render.Context → accelerator → gpu/webgpu → gpu/rwgpu → libwgpu_native`  
> 能力基线：[`SKIA_2D_CAPABILITY_MATRIX.md`](./SKIA_2D_CAPABILITY_MATRIX.md) M3

## 范围（M3）

| 能力 | 状态 | 门禁 |
|------|------|------|
| F.01 Blur | ✅ | `TestS3c_M3_ApplyBlur` |
| F.02 Drop shadow | ✅ | `TestS3c_M3_ApplyDropShadow` |
| F.04 Color filter | ✅ | `TestS3c_M3_ApplyGrayscale` |
| S.03 Offscreen present | ✅ | `TestS3c_M3_OffscreenPresentPath` / `SurfacePresentAPISmoke` |
| S.09 Damage present | ✅ | `TestS3c_M3_DamagePresentPath` |
| H.04 Path boolean | ✅ | `TestS3c_M3_PathBoolean*` |
| H.05 Path measure | ✅ | `TestS3c_M3_PathLength` |
| R.01 Recording | ✅ | `TestS3c_M3_RecordingPlayback` |
| CS.01 sRGB | ✅ | `TestS3c_M3_DefaultSRGBSurface` |
| V.01 DrawVertices | ✅ | `TestS3c_M3_DrawVertices` |
| V.02 DrawAtlas | ✅ | `TestS3c_M3_DrawAtlas` |
| B.04 HSL blends | ✅ | `TestS3c_M3_BlendHue` / `BlendColorAndLuminosity` |
| C.06 Clip Replace/Difference | ✅ | `TestS3c_M3_ClipRectDifference` / `Replace` |
| I.04 Bicubic/mipmap | ✅ | `TestS3c_M3_ImageBicubicAndMipmap` |
| I.07 Nine-patch | ✅ | `TestS3c_M3_DrawImageNine` |
| X.08 Text decoration | ✅ | `TestS3c_M3_TextUnderline` |
| X.09 Variable font | ✅ | `TestS3c_M3_VariableFontWeight` |
| X.10 Color emoji path | ✅ API 接入 | `TestS3c_M3_DrawWithEmojiAPI`（需彩色字体资产才有色 glyph） |
| 窗口 Swapchain Present | 🔄 后置 | 需 display/HWND |
| F.03 filter DAG | ⬜ M4 | |
| K.01 Compute 路径 | 🔄 部分 vello | 可选增强 |

## 实现要点

- **B.04**：`BlendHue/Saturation/Color/Luminosity`（CPU `compositeAdvanced` + image `blend`）  
- **C.06**：`ClipRectOp` / `ClipPathOp`；mask difference 强制 CPU `ClipCoverage`（GPU 无 path 时）  
- **H.04**：`BooleanPath` / `Path.Op` 扫描线 winding → run 矩形  
- **I.04**：`InterpBicubic` + `UseMipmaps` → `GenerateMipmaps`  
- **I.07**：`DrawImageNine` → `DrawAtlas` 九切片  
- **X.08**：`SetTextDecoration` 下划线/删除线/上划线  
- **X.09**：`LoadFontFaceWithVariations` + `FontVariationAxes`  
- **X.10**：`drawStringBitmap` → `text.DrawWithEmoji`  

## 硬规则

1. 声称 GPU：`GPUOps>0` + `WGPU_NATIVE_PATH`  
2. 像素/语义检查  
3. 性能不作为退出条件  

## 门禁命令

```bash
export WGPU_NATIVE_PATH=/path/to/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache
go test -count=1 ./render -run 'TestS3c_|TestS3b_|TestS3a_|TestP12GPUFixedPixel'
```

## 退出检查

- [x] M3 必选 + residual 门禁绿 — `TestS3c_*`  
- [x] DrawVertices / DrawAtlas / filter / present smoke  
- [x] B.04 / C.06 / H.04 / I.04 / I.07 / X.08–X.10  
- [ ] 窗口 Surface/Swapchain 端到端 Present（书面后置）  
- [x] 能力表回写  

**S3c：✅ 关闭（窗口 Present / F.03 DAG / K.01 完整 后置）**

## 修订

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-15 | 0.3 | M3 residual：HSL/clip op/path boolean/nine/mipmap/text decor/VF/emoji |
| 2026-07-15 | 0.2 | V.01/V.02 GPU 门禁；S3c 关闭（窗口后置） |
| 2026-07-15 | 0.1 | filter/shadow/present/recording 首切片 |
