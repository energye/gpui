# ui_render_base_scene_sceneshapes

scene 形状族直驱真窗（O12 批 3，RENDER_FEATURE_COVERAGE §12 NONE→EX）。

## 覆盖面（logic 19）

- **14 构造全读回**：`NewRectShape/RoundedRectShape/CircleShape/EllipseShape/RoundRectShape(+Uniform)/LineShape/PolygonShape/RegularPolygonShape/StarShape/ArcShape/PieShape/TransformShape/CompositeShape/PathShape` × `ToPath/BuildPathInto/Bounds`。verb 数实证：Rect=5、RoundedRect=10、Circle/Ellipse=6、Star(5 角)=11、RegularPolygon(6 边)=7、**Arc(120°)=3**（2 段 cubic）、Pie=5、Line=2。
- **PathBuilder 契约**：`BuildPathInto(pooled)` 与 `ToPath` verb 数一致（池化路径等价）。
- **组合**：`CompositeShape` verb 累加、`TransformShape` 同段数+bounds 放大（40×2=80）。
- **Renderer 实画**：`NewRenderer(WithWorkers(1)).Render(render.Pixmap)` 三形状（Fill 圆/圆角矩形/Stroke 星形）CPU 渲染读回。

## 语义记录

- `scene.Path.VerbCount` 与 render 主包 `Path.NumVerbs` 是两套计数（scene 面独立）。
- ArcShape 段数随扫角变化（120°→3 verb），不做硬上限断言。
- `render.NewPixmap` + `pm.At(x,y)` 是 Renderer 输出的直读口。

## 门禁

```
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/ui_render_base_scene_sceneshapes -auto-only
```

首播种 `GPUI_ACCEPT_GOLDEN=1`；卡内容改动需重播种。
