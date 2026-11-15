# ui_render_base_masksrc — 掩码构造族直驱真窗

O12 批 1b W6（RENDER_FEATURE_COVERAGE §12 NONE → examples 能力测试，台账 O13）。
直驱窗（`wrkit.OpenDirect` + `raster.RunGLSync` 纪律）。

## 覆盖面（O12 NONE 条目）

`NewMaskFromAlpha / NewMaskFromData / NewLuminanceMask / Context.AsMask /
SetMaskFilter / MaskFilterRadius / MaskPlaneStats`。

- 卡 A 构造三口：Alpha 通道（圆盘）→ Data 字节（方块）→ Luminance 亮度
  （亮度值=掩码 alpha：盘内纯白全显/盘外纯黑全遮）——各自 SetMask+填充显示即构造形状。
- 卡 B AsMask 路径捕获：DrawCircle（不 Fill，路径保留）→ AsMask 栅格化 →
  Mask.At 盘内 255/盘外 0 → SetMask 显示=圆。Fill 后捕获=空掩码陷阱（文档明载，
  本窗走正确流程）。
- 卡 C 环境滤镜：SetMaskFilter(6) 读回 → 安装掩码原位模糊（软边沿中间值探针：
  edge=82/47/109 vs center=153/89/204 显著衰减）→ 归零后硬边保持（edge=实心值）。
- 卡 D MaskPlaneStats：读回口可用且单调一致。计数器=maskR8 缓存池账本（字形掩码线
  P4.3 消费者），普通填充掩码不经它（如实为 0，findings 记录）。

## 证据链

逻辑 2+16、像素 4、Golden 逐位 0.0000%、抓屏 ≤1%。

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/ui_render_base_masksrc -auto-only   # 门禁
go run ./examples/ui_render_base_masksrc                            # 常驻人工窗
```
