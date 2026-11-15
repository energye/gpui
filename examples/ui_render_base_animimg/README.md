# ui_render_base_animimg

动图族直驱真窗（O12 批 4，RENDER_FEATURE_COVERAGE §12 NONE→EX）。**动画帧全部腿读回钉死，不烙单帧 golden**（动态内容纪律）。

## 覆盖面（logic 24）

- **容器面**：`LoadAnimatedImage(gif_keep.gif)` → `FrameCount=3/RepetitionCount=-1（无限）/Canvas 40×30/FrameInfo(0)`（Duration=100ms/RequiredFrame=-1/Blend=SrcOver/Disposal=Keep/DisposalRect 全幅）+ `DecodeFrame(0→1)` 推进 + 快照差分（**Image()=画布引用，publish 原地 copy**）。
- **Disposal 三档**：`gif_restore_bg.gif` frame1=RestoreBGColor、`gif_restore_prev.gif` frame1=RestorePrevious、`gif_keep.gif`=Keep（GIF 容器 disposal 按帧序映射 2/3）+ `Dispose/Disposed` 释放标志。
- **Blend 语义**：`FrameBlendSrc/SrcOver` 两值枚举（GIF 透明索引=SrcOver 表达）+ `NoRequiredFrame=-1`（独立帧）。
- **Encoding/Orientation**：`ImageEncoding` 七常量（Unknown=0..HEIF=6）+ `ImageEncodingOfBytes` GIF 头识别 + `ImageOrientationOfBytes` EXIF 八向（exif_1=TopLeft、exif_6=RightTop）+ 九常量序（Unspecified=0..LeftBottom=8，EXIF 直映射）。

## 门禁

```
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/ui_render_base_animimg -auto-only
```

首播种 `GPUI_ACCEPT_GOLDEN=1`；卡内容改动需重播种。
