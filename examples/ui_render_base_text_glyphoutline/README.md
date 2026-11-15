# ui_render_base_text_glyphoutline

字形轮廓族直驱真窗（O12 批 3，RENDER_FEATURE_COVERAGE §12 NONE→EX）。

## 覆盖面（logic 18）

- **OutlineExtractor**：`ExtractOutline/ExtractOutlineHinted(HintingFull)/ExtractOutlineHintedVar(nil variations)`——段数读回（'A'=13 段）、hinted 与无 hinted 同段数、无轴字体 Var 口走基础路径。
- **EmboldenOutline**：外扩后段数保持、Bounds X 外扩（31.5→33.4）。
- **GlyfContours**：`ParseGlyfContoursFromSource`——NumContours=2、`ContourPoints(0)` 点云。
- **FilteredFace**：`NewFilteredFace(face, UnicodeRange{Start:'A',End:'Z'})` × `AppendGlyphs/Features/Embolden` 委托读回=源 face 值。
- **MultiFace**：`NewMultiFace(face)` × `AppendGlyphs/Features/Embolden/WithWeight(700)`。

## 语义记录

- `MultiFace.WithWeight` 返回**新** MultiFace 实例（组件 face 按权重重推导），非同指针。
- `text.Rect` 字段为 MinX/MinY/MaxX/MaxY；`UnicodeRange` 字段为 Start/End；`OutlineSegment.Points[3]` 按 Op 取端点。

## 门禁

```
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/ui_render_base_text_glyphoutline -auto-only
```

首播种 `GPUI_ACCEPT_GOLDEN=1`；卡内容改动需重播种。
