# ui_render_base_scene_scenelayer

scene 层族直驱真窗（O12 批 3，RENDER_FEATURE_COVERAGE §12 NONE→EX）。补既有 ui_render_base_scene 窗（SceneBuilder/GPUSceneRenderer/tile/LayerCache/DamageTracker 主链）未触的层型/链面。

## 覆盖面（logic 23）

- **LayerStack**：`NewLayerStack` **初始含根层**（Depth=1）；`Push/Top/Pop`（根层不可弹=nil）；`NewClipLayer` Kind=LayerClip + `UpdateBounds` 求交。
- **FilterChain/FilterType**：空链 Len=0；四枚举 None/Blur/DropShadow/ColorMatrix（`ExpandsOutput`：Blur/DropShadow=true）。
- **Scene 层口**：`PushLayer(BlendMultiply,0.5,nil)/PopLayer/PushClip/PopClip/PushTransform/PopTransform/TransformDepth`。
- **DamageTracker**：`IsFirstRender` 判 **rendered 标志**（MarkRendered 后 false）；`ComputeDamage` 首帧返回零 rect（firstFrame 内部翻转）；移动产损伤（旧+新并集）→静止零损伤→`Reset` 回首帧。
- **RenderWithDamage**：双场景（圆→橙圆）损伤渲染实画读回。

## 语义记录

- `IsFirstRender` 与 `ComputeDamage` 的 firstFrame 是两条独立标志：前者=全量渲染是否已发生（MarkRendered 置位），后者=本次 ComputeDamage 是否首调。
- LayerStack 永远含根层：`IsRoot()==Depth()==1`。

## 门禁

```
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/ui_render_base_scene_scenelayer -auto-only
```

首播种 `GPUI_ACCEPT_GOLDEN=1`；卡内容改动需重播种。
