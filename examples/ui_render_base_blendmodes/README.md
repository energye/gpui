# ui_render_base_blendmodes

12 非平凡混合模式直驱真窗（O12 批 4，RENDER_FEATURE_COVERAGE §12 NONE→EX）。

## 覆盖面（logic 29）

- **可分离九模式 ±2 数学探针**：`BlendDarken/Lighten/ColorBurn/ColorDodge/Overlay/HardLight/SoftLight/Exclusion/Difference`——已知底色（0.5,0.55,0.45）×源色（0.9,0.35,0.6）逐像素对照独立 W3C 公式复算（±2/255 舍入）。
- **HSL 四模式定性探针**：`BlendHue/Saturation/Color/Luminosity`——结果≠src、≠base（混合发生）+ 双跑逐位稳定。setSat/setLum 组合（W3C compositing-1 语义）。
- **scene 面**：`scene.Blend*` 12 常量枚举读回 + `Scene.PushLayer(BlendMultiply)/Fill` 实画（0.9×0.5=0.45 读回 ±1）+ `Context.SetBlendMode/BlendMode()` 读回。

## 发现 → 已修（2026-11-15）

- **hsl-quantization（已修）**：原 HSL 四模式引擎实现在 u8 域逐级舍入（setSat→u8→setLum→clip），与 W3C 0-1 域独立复算差 ~9/255。现四个 HSL 函数（blendHue/Saturation/Color/Luminosity）改 float 域计算——setSatF/setLumF/clipColorF 全 float 链、一次量化（render/internal/image/draw.go），钉子 hsl_float_test.go 与独立 W3C 复算 ±1 内一致。旧 u8 域函数链（lumU8/satU8/clipColor/setLum/setSat）无消费者已删。可分离九模式 ±2 逐像素闭合不变。

## 门禁

```
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/ui_render_base_blendmodes -auto-only
```

首播种 `GPUI_ACCEPT_GOLDEN=1`；卡内容改动需重播种。
