# ui_render_base_text_glyphraster

GlyphMaskRasterizer 全变体直驱真窗（O12 批 3，RENDER_FEATURE_COVERAGE §12 NONE→EX）。真字体 goregular.ttf gid=36（'A'）驱动 11 个 Rasterize 口。

## 覆盖面（logic 21）

- **AA 档**：`Rasterize/RasterizeHinted(HintingFull)/RasterizeHintedBold(bold=true)/RasterizeHintedFT26`——R8 掩码读回墨迹、Bold 墨迹 643>379（外扩实证）、FT26 在 glyf 字体返回 (nil,false,nil)（CFF 专用分支契约命中）。
- **Aliased 档**：`RasterizeAliased/AliasedBold/Outline/OutlineAliased`——严格二值（0/255）、Outline 接 `OutlineExtractor` 产物。
- **LCD 档**：`RasterizeLCD/LCDBold(+bold 参)/LCDOutline(outline 参)`——`LCDMaskResult` RGB 3B/px 缓冲、DefaultLCDFilter+LCDLayoutRGB、Bold 外扩宽度 ≥ 常规。
- **几何**：`GlyphMaskResult` Width/Height/BearingX/BearingY/Advance；subpixelX=0.5 掩码错相采样。

## 语义记录

- `RasterizeHintedFT26` 仅 PostScript（CFF/CFF2）轮廓字体适用；TrueType glyf 走 (nil,false,nil)。
- `RasterizeAliased(…,hinting)` 带显式 hinting 参；`RasterizeLCDBold` 尾参 bool=bold；`RasterizeLCDOutline` 接已提取 outline。
- Bold 掩码尺寸外扩幅度随字号/加粗量变化（不设硬上限断言）。

## 门禁

```
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/ui_render_base_text_glyphraster -auto-only
```

首播种 `GPUI_ACCEPT_GOLDEN=1`；卡内容改动需重播种。
