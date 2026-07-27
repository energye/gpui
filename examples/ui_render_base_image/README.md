# ui_render_base_image

**Image 轴**窗测 — `ENGINE_UI_RENDER_BASE` FImg-* / §24。

## 覆盖

| 项 | 方式 |
|----|------|
| FImg-DRAW | `RenderImage` sync `SetImage` |
| FImg-CODEC | `ui/io.DecodeFile` 工作线程解码 → ticker hop 上 UI |
| FImg-RECT | `DrawImageEx` + `SrcRect` |
| FImg-ROUND / CIRCLE | `DrawImageRounded` / `DrawImageCircular` |
| FImg-NINE | `DrawImageNine` |
| 指标 | layout 不因换图风暴；JSON RSS/CPU/fps |

**不做：** GIF 多帧、TextureLayer、P6 dirty Present。

## 运行

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=15 go run ./examples/ui_render_base_image
```

门禁：`layout_flushes` 远低于 presents；`async_decode_callbacks ≥ 1`（≥2s 跑）。
