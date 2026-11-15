# ui_render_base_recording_recordstyle

recording 样式/画刷族直驱真窗（O12 批 3，RENDER_FEATURE_COVERAGE §12 NONE→EX）。补既有 ui_render_base_recording 窗（Recorder/Playback/Backend 主链）未触的画刷换算与补全口。

## 覆盖面（logic 19）

- **Recorder 补全口**（双实现 live/回放逐位 diff=0）：`SetHexColor/SetFillRGB→FillBrush` 读回、`FillStroke`（Recorder 面，Canvas 契约面用 FillPreserve+Stroke 等价）、`NewSubPath/InvertY/SetDashOffset/DrawRRect`。
- **PatternBrush 链**：`NewPatternBrush/SetSourceRegion/SetInverse/SetOpacity/SetClamp/Source`（Image 读回+opacity=0.5 落盘）+ `NewCustomPatternBrush`。
- **换算**：`FromRenderBrush/ToRenderBrush` 往返 ColorAt 一致；`ToRenderPattern` solid **ok=false**（不转语义）；`FromRenderPattern(ImagePattern)`；`ApplyBrush/ApplyStroke(DefaultStroke)` 直调。
- **ResourcePool**：`AddPath` **克隆不可变语义**（Get 返回副本）；`AddBrush/AddImageBuf/Get*/Images/Counts`。

## 语义记录

- `ResourcePool.AddPath` 克隆路径保证录制后不可变——同指针断言不成立是契约。
- `ToRenderPattern(SolidBrush)` 返回 ok=false：solid 不转图案。
- `FillStroke` 是 Recorder 方法不在 render.Canvas 契约内（Canvas 面用 FillPreserve+Stroke）。

## 门禁

```
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/ui_render_base_recording_recordstyle -auto-only
```

首播种 `GPUI_ACCEPT_GOLDEN=1`；卡内容改动需重播种。
