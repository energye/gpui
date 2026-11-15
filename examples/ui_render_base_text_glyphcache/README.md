# ui_render_base_text_glyphcache

字形缓存族直驱真窗（O12 批 3，RENDER_FEATURE_COVERAGE §12 NONE→EX）。

## 覆盖面（logic 25）

- **GlyphCache**：`GetOrCreate` 同指针缓存命中（create 次数=1）、`Stats`（**GetOrCreate 口径 miss 不计**：hit=1/ins=1）、`CurrentFrame` 初帧 0（推进由生产帧循环）、`Maintain` 无错、`NewGlyphCacheWithConfig(MaxEntries/FrameLifetime)`。
- **SubpixelCache**：`SubpixelConfig{Mode:Subpixel4,Horizontal:true}` → IsEnabled/CacheMultiplier=4；`SubpixelMode.Divisions`（4/10/None 三档）；`GetOrCreate(offsetX,offsetY)` 槽位同指针；`SetConfig/Maintain`。
- **GlyphMaskAtlas**：惰性分配（初始 pages=0）；`GetOrRasterize` 二次取同 region（缓存快路径不重栅格）；脏页 `[0]`→`MarkClean(0)` 清空；`EntryCount=1/MemoryUsage/PageBytes(0)/PageR8Data` 1024² 页；`PutLCD` region.Width=逻辑宽×3（R8 3×）；`UnderPressure=false`；`TouchPage/DirtyUploads` 读回。

## 语义记录

- `GlyphCache.Stats` 的 miss 计数只在 `Get` 路径累计，`GetOrCreate` 首次插入不算 miss（实证）。
- Atlas 页惰性分配：`NewGlyphMaskAtlasDefault` 后 pages=0，首次 store 才建页。
- `PutLCD` 的 advance 参在 LCDMaskResult 上不存在（LCD 面无 advance 字段），传 0。

## 门禁

```
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/ui_render_base_text_glyphcache -auto-only
```

首播种 `GPUI_ACCEPT_GOLDEN=1`；卡内容改动需重播种。
