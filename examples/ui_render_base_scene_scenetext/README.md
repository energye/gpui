# ui_render_base_scene_scenetext

scene 文本族直驱真窗（O12 批 3，RENDER_FEATURE_COVERAGE §12 NONE→EX）。

## 覆盖面（logic 15）

- **scene TextRenderer**：`DefaultTextRendererConfig`（FlipY=false/Subpixel=true/Hinting=false）+ `NewTextRendererWithConfig/Config()` 读回 + `RenderGlyph/RenderGlyphs("Scene"→4)/RenderText("Rendered!"→9)/ToCompositePath`。
- **TextShape**：`NewTextShape(str,face,x,y)` 轮廓合成（ToPath verb/Bounds 读回）。
- **TextRendererPool** Get/Put。
- **编码值类型**：`TextFlags`（Hinting=1<<0/CJK=1<<1）、`GlyphRunData`、`GlyphEntry`（gid/X/Y/Cluster）。
- **实画**：`Scene.Fill(TextShape)` CPU 渲染墨迹读回。

## 门禁

```
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/ui_render_base_scene_scenetext -auto-only
```

首播种 `GPUI_ACCEPT_GOLDEN=1`；卡内容改动需重播种。
