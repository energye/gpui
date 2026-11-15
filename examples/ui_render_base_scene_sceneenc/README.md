# ui_render_base_scene_sceneenc

scene 编码族直驱真窗（O12 批 3，RENDER_FEATURE_COVERAGE §12 NONE→EX）。

## 覆盖面（logic 16）

- **Encode×13 直调**：`EncodeAntiAlias/EncodeTransform/EncodeTransformFromMatrix/EncodeProjection(ProjectionSet+Matrix4Identity)/EncodeMaterial/EncodePath(render.Path)/EncodeFill/EncodeFillRoundRect/EncodeStroke/EncodePushLayer/EncodePopLayer/EncodeBeginClip`。
- **Iterator 回读**：`NewIterator/Next/Reset`——11 次编码调用回读 16 tag（**EncodePath 按 verb 展开多 tag**），Reset 后重走计数一致（迭代语义自洽）。
- **三池**：`EncodingPool(Get/Put/Warmup)/DefaultPool/ScenePool/PathPool/GetEncoding` 复用语义。
- **值类型**：`AffineFromMatrix`（平移项映射 C/F）、`NewAffine` 六参、`EmptyRect` 反向界、`Rect.UnionPoint`、`DefaultStrokeStyle`（w=1 miter=10）。
- **实画**：编码场景 `Renderer.Render` FillRect 读回。

## 语义记录

- `EncodePath` 收 `*render.Path`（主包路径），不是 scene.Path。
- `EncodeProjection` 用 `render.ProjectionSet` + `render.Matrix4Identity()`（零值 Matrix4 不可用契约）。
- Iterator tag 数 ≥ 编码调用数：Path/Fill 等展开为多条 tag。

## 门禁

```
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/ui_render_base_scene_sceneenc -auto-only
```

首播种 `GPUI_ACCEPT_GOLDEN=1`；卡内容改动需重播种。
