# ui_render_base_cliplayer

**ClipLayer 轴** — Boundary / Clip / Opacity / Transform（`ENGINE_UI_RENDER_BASE` §24）。

## 覆盖

- 静态 `RepaintBoundary` 卡片（hot 动画时不应 layout storm）
- `ClipRect` 溢出滚动（paint）
- Opacity 脉动（paint alpha）
- `RenderTransform` → scene `TransformLayer` 旋转

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=15 go run ./examples/ui_render_base_cliplayer
```
