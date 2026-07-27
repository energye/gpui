# ui_render_base_geometry

**Geometry 轴**真窗专项 — 对应 [`docs/ENGINE_UI_RENDER_BASE.md`](../../docs/ENGINE_UI_RENDER_BASE.md)：

- **§2** `dart:ui.Canvas` 几何绘制（`FC-DRAW-*`）
- **§3** 描边样式（`StrokeStyle`）
- **§5** `PaintContext.PushClipRRect`（`FC-CLIP-RRECT` 库 API）
- **§24** 窗测轴 `Geometry`

## 覆盖（P0 `ui/painting` = 状态 A）

| 面板 | API | 母表 ID |
|------|-----|---------|
| Fill rect | `FillRect` | FC-DRAW-RECT |
| Fill rrect | `FillRoundRect` | FC-DRAW-RRECT |
| Linear gradient | `FillLinearGradient2` | FS-LINEAR |
| Stroke rect | `StrokeRect` + `SetStrokeStyle` | FC-DRAW-RECT stroke / FP-STROKE-* |
| Stroke rrect | `StrokeRoundRect` | FC-DRAW-RRECT stroke |
| Lines | `StrokeLine` | FC-DRAW-LINE |
| Fill circle | `FillCircle` | FC-DRAW-CIRCLE |
| Stroke circle | `StrokeCircle` | FC-DRAW-CIRCLE stroke |
| Clip rrect | `PushClipRRect` / `PopClip` | FC-CLIP-RRECT |
| Radial gradient | `FillRadialGradient2` | FS-RADIAL (**P1**) |
| Path triangle | `FillPath` / `StrokePath` | FC-DRAW-PATH (**P1**) |
| Oval + Arc | `FillOval` / `StrokeArc` | FC-DRAW-OVAL / ARC (**P1**) |

## 明确不在本示例（母表仍后置）

- 完整 Paragraph 富文本、Path.computeMetrics、TransformLayer 场景树  
- P6 dirty-rect Present / Picture 显示列表  

## 运行

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=15 go run ./examples/ui_render_base_geometry

# 可选字体
GPUI_UI_FONT=/path/to.ttf
```

## 指标读法

- stderr：fps、layout_flushes、p50/p99、RSS/CPU、`vsync_source`  
- stdout：`FrameMetrics` JSON（含 P0 收尾字段）  
- **layout_flushes 应 ≪ presents**（仅三块动画 Boundary 脉动）  
- `raster_layer_count` = 脏层统计，**≠** GPU 局部 Present  

## 纪律

- 场景只在 `examples/`，不进 `ui/`  
- 禁止 CGO；`ui → render → gpu`  
