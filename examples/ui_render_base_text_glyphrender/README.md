# ui_render_base_text_glyphrender

字形渲染器直驱真窗（O12 批 3，RENDER_FEATURE_COVERAGE §12 NONE→EX）。

## 覆盖面（logic 16）

- **TextRenderer**：`NewTextRenderer/SetDefaultFace/SetDefaultSize/SetDefaultColor` × `ShapeAndRender`（"Render"→6 轮廓）/`ShapeAndRenderAt`（"At 42"→5）/`GlyphRenderer()` 内建非 nil——轮廓经 strokeOutlines 实画佐证。
- **GlyphRenderer**：`RenderGlyph`（段数>0）/`RenderGlyphs`/`RenderRun`（ShapedRun{Face,Size,Style:-1}）/`RenderLayout`（LayoutTextSimple 行数）+ `SetCache` 私有缓存注入。
- **RenderParams**：`WithColor/WithOpacity` 不可变链（opacity=0.5 读回）。
- **GlyphRunBuilder**：`NewGlyphRunBuilder(cache)/AddGlyph/AddShapedGlyph/AddShapedGlyphs/AddShapedRun` → `Len()=6`；`Build` 命令数=字形数；`BuildTransformed` 携带仿射（A/D=2）；`Instances`；`Clear` 清空。

## 语义记录

- `ShapedRun.Style=-1` 是 standalone layout 的无样式约定；`AffineTransform` 字段 A/B/C/D/Tx/Ty。
- 轮廓输出是 renderer 产品的最终形态（描边展示），像素级文字由 Context.DrawString 生产路径覆盖。

## 门禁

```
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/ui_render_base_text_glyphrender -auto-only
```

首播种 `GPUI_ACCEPT_GOLDEN=1`；卡内容改动需重播种。
