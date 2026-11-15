# ui_render_base_pixmapops — Pixmap 像素直操作+输出族直驱真窗

O12 批 1b W5（RENDER_FEATURE_COVERAGE §12 NONE → examples 能力测试，台账 O13）。
直驱窗（`wrkit.OpenDirect` + `raster.RunGLSync` 纪律）。

## 覆盖面（O12 NONE 条目）

`NewPixmapFromBuffer / FromImage / SetPixel / SetPixelPremul / FillSpan /
FillSpanBlend / ImageView / ToImage / GenerationID / AddGenIDWatcher /
EncodePNG / EncodeJPEG / CropImageToInk` + `Context.SetPixel / WritePixels /
EncodePNG / EncodeJPEG / PixelWidth / PixelHeight`。

- 卡 A 像素直写：SetPixel 8×8 块×2、SetPixelPremul 预乘直写（a=128 半值精确）、
  NewPixmapFromBuffer 往返、Context.SetPixel、WritePixels 块（60×40，GPU 四边形
  上传路径计入 GPU op）、PixelWidth/PixelHeight 读回。
- 卡 B Span 填充：FillSpan 厚带（越界裁剪探针）+ FillSpanBlend 混合数学
  （底×(1-α)+色×α 逐像素精确命中，α=0.5）。
- 卡 C 编码往返：Pixmap.EncodePNG（解码=源逐位）、Pixmap.EncodeJPEG（厚带容差
  30——色度子采样对 1px 线必然重糊，已声明）、Context.EncodePNG。
- 卡 D GenID+墨迹裁剪：SetPixel 不自动换代（文档契约）→ NotifyPixelsChanged
  换代+监视器触发 → remove 后不再触发；CropImageToInk（35×40 + 基线约定原点
  (25,-30)：bitmap left=pen+OriginX，top=baseline−OriginY）+ 裁剪直画。

## 已知限界

- 快照缩略探针只钉实心块（1px 点/单行在 0.75 缩放必被双线性稀释）——细特征由
  腿读回探针（全分辨率）+ 黄金图逐位钉死。
- JPEG 容差 30 属有损声明；PNG 逐位无容差。

## 证据链

逻辑 2+21、像素 7、Golden 逐位 0.0000%、抓屏 ≤1%。

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/ui_render_base_pixmapops -auto-only   # 门禁
go run ./examples/ui_render_base_pixmapops                            # 常驻人工窗
```
