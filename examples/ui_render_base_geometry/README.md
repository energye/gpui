# ui_render_base_geometry

**Geometry 轴**真窗专项 — 对应 [`docs/ENGINE_UI_RENDER_BASE.md`](../../docs/ENGINE_UI_RENDER_BASE.md)：

- **§2** `dart:ui.Canvas` 几何绘制（`FC-DRAW-*`）
- **§3** 描边样式（`StrokeStyle`）
- **§5** `PushClipRoundRect`（`FC-CLIP-RRECT` 的 UI 子集）
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
| Clip rrect | `PushClipRoundRect` / `PopClip` | FC-CLIP-RRECT |

## 明确不在本示例（母表 B/D）

- Path 任意路径、Arc/Oval 的 **ui 封装**、DRRect、Vertices/Atlas  
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
