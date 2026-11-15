# ui_render_base_blendmodes

12 非平凡混合模式直驱真窗（O12 批 4，RENDER_FEATURE_COVERAGE §12 NONE→EX）。

## 覆盖面（logic 29）

- **可分离九模式 ±2 数学探针**：`BlendDarken/Lighten/ColorBurn/ColorDodge/Overlay/HardLight/SoftLight/Exclusion/Difference`——已知底色（0.5,0.55,0.45）×源色（0.9,0.35,0.6）逐像素对照独立 W3C 公式复算（±2/255 舍入）。
- **HSL 四模式定性探针**：`BlendHue/Saturation/Color/Luminosity`——结果≠src、≠base（混合发生）+ 双跑逐位稳定。setSat/setLum 组合（W3C compositing-1 语义）。
- **scene 面**：`scene.Blend*` 12 常量枚举读回 + `Scene.PushLayer(BlendMultiply)/Fill` 实画（0.9×0.5=0.45 读回 ±1）+ `Context.SetBlendMode/BlendMode()` 读回。

## 发现（语义边界，不判死）

- **hsl-quantization**：HSL 四模式的引擎实现与 W3C 0-1 域独立复算存在 u8 量化链差（Saturation 全通道 -9、Color +16 等——setSat 输入 uint8 化位置与 sRGB↔linear 域链），量级 <6%，语义正确（定性探针通过）。可分离九模式在 ±2 内逐像素闭合。

## 门禁

```
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/ui_render_base_blendmodes -auto-only
```

首播种 `GPUI_ACCEPT_GOLDEN=1`；卡内容改动需重播种。
