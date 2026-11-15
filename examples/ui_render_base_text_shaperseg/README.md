# ui_render_base_text_shaperseg

整形/分段直驱真窗（O12 批 3，RENDER_FEATURE_COVERAGE §12 NONE→EX）。

## 覆盖面（logic 17）

- **双 shaper 生产对齐**：`NewHbShaper/NewOwnShaper.Shape` 同文本同 face 逐字形 GID/X 一致（生产对齐面实证）；`ClearCache/RemoveSource` 清理后重整形出同长序列。
- **分段**：`SegmentText("ab中文cd")` 3 段（Latin/Han 分段、`RuneCount` 逐段）；`SegmentTextRTL` Hebrew；`NewBuiltinSegmenterWithDirection(DirectionRTL)` 方向标记。
- **HitTest**：`HitTestCluster/HitTestClusterEdge`——串中点→第 4 簇、负 x→0（前置钳位）、超出→尾簇（9）；`Quantize(10.75,Subpixel4)→(10,3)`。

## 语义记录

- "Handgloves"（9 字符）整形出 10 字形（ligature 拆分真值），断言只钉双 shaper 一致不钉字面长度。

## 门禁

```
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/ui_render_base_text_shaperseg -auto-only
```

首播种 `GPUI_ACCEPT_GOLDEN=1`；卡内容改动需重播种。
