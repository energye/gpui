# ui_render_base_brushctor

画刷构造/Painter 直驱真窗（O12 批 1c，RENDER_FEATURE_COVERAGE §12 NONE→EX）。

## 覆盖面

- **卡 A Solid 构造族**：`SolidRGB/SolidRGBA/SolidHex/FromColor(color.NRGBA)/HSL` 数值读回（RGBA 字段为 0-1 float64；HSL h 为度数语义 120=绿）、`SolidBrush.WithAlpha/Opaque/Transparent` 链读回、`BrushWithAlpha` 包裹 ColorAt。画面：五构造色卡+alpha 三阶。
- **卡 B 渐变独立口**：`HorizontalGradient/VerticalGradient/LinearGradient/RadialGradient/SweepGradient/NewSweepGradientBrush+AddColorStop/Stripes`。ColorAt 数学探针：中点半混、径向 r/2、sweep 0°=t0 90°=t0.25、条纹 floor(x/width) 奇偶；端点 t=0 精确、t=1 有 1e-16 浮点尾差（clamp01→Lerp 实证）。画面：六渐变缩略（hg 范围与填充盒对齐）。
- **卡 C 包级口 vs Context 口逐位一致**：同几何 `SetFillBrush(Solid)` 帧与 `SetRGBA` 帧全画布逐位 diff=0；`PainterFromPaint(solid)→SolidPainter` 快路径+PaintSpan 缓冲逐元素、渐变 paint→FuncPainter 逐像素==Fn。
- **卡 D BrushFieldSeed**：SolidBrush=nil=0、WithAlpha 包裹非 0、异 alpha seed 互异、同 alpha 同 seed（GPU field cache key 语义实证）。

## 语义记录（不是洞）

- RGBA 字段为 0-1 float64（非 0-65535）；`RGBA()` 方法才是 color.Color 的 65535 缩放口。
- `HSL(h,…)` 的 h 按度数（mod 360）而非 0-1。
- 渐变独立口 `t=clamp01` 后 `Lerp`：t=0 端点精确、t=1 有浮点尾差，比较需容差。
- `Stripes` 相位=`floor(投影坐标/width)` 奇偶，angle=0 时沿 x。
- 缩略图上渐变/细线每像素都是混色，点探不可用——ColorAt 数学探针+腿读回+黄金图三层钉死。

## 门禁

```
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/ui_render_base_brushctor -auto-only
go run ./examples/ui_render_base_brushctor                # manual: 常驻
```

首播种 `GPUI_ACCEPT_GOLDEN=1`；卡内容改动需重播种。
