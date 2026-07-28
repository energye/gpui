# ui_render_base_geometry

**Geometry 轴**真窗专项 — [`docs/ENGINE_UI_RENDER_BASE.md`](../../docs/ENGINE_UI_RENDER_BASE.md) **§22 序5** / §2–§3 / §23。

实现入口：`ui/rendering` 的 `FillRect` / `Stroke*` / `FillCircle` / …（`draw.go`），**不是**示例私有重实现。

## 覆盖

| 面板 | API（`ui/rendering`） | 母表 ID |
|------|----------------------|---------|
| Fill/Stroke rect | `FillRect` / `StrokeRect` | FC-DRAW-RECT |
| Fill/Stroke rrect | `FillRoundRect` / `StrokeRoundRect` | FC-DRAW-RRECT |
| Linear gradient | `FillLinearGradient` | FS-LINEAR |
| Lines | `StrokeLine` | FC-DRAW-LINE |
| Circle | `FillCircle` / `StrokeCircle` | FC-DRAW-CIRCLE |
| Clip rrect | `PaintContext.PushClipRRect` | FC-CLIP-RRECT（paint） |
| Radial | `FillRadialGradient` | FS-RADIAL |
| Path | `NewPath` + `FillPath` / `StrokePath` | FC-DRAW-PATH |
| Oval + Arc | `FillOval` / `StrokeArc` | FC-DRAW-OVAL / ARC |

## 不在本轴

- Path.computeMetrics、全量 Paragraph、Transform 逆 hit  
- dirty-rect Present / Picture 显示列表（序13）  
- ClipRRect **Layer** RO 接线（FL-CLIP-RRECT 仍 C）

## 运行

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=15 go run ./examples/ui_render_base_geometry
```

单测：`go test ./ui/rendering -run TestDraw_ -count=1`
