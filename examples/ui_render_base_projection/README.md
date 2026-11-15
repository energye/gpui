# ui_render_base_projection

投影层直驱真窗（O12 批 4，RENDER_FEATURE_COVERAGE §12 NONE→EX）。

## 覆盖面（logic 15）

- **栈语义**：`PushProjection/PopProjection/Projection()` LIFO（恒等态归一化快照——零值 Matrix4 污染防护 D6）+ `SetProjection` 直设 + 弹空回恒等。
- **生效面**：非恒等投影（2× scale）`DrawImageEx` 链实画放大（40px→40..120 读回）。
- **fail-closed**：NaN 矩阵被 SetProjection 忽略（恒等兜底，D6 契约）。
- **枚举补卡**：`ProjectionPush/Pop` 常量、`PolicyDefault/High/Low` 三值序 + 四弃用别名同值（None/HighPerformance/LowPower/Auto）+ `AdapterPolicy.String` + `DepthCPUFallbackReason`（string 常量=深度分支降级标记）。`Accel*` 八常量已由批 2 accelcontract CanAccelerate 八 op 覆盖。

## 语义记录

- Matrix4 零值不可用（P3.5 批 3 契约）：SetProjection 把零值按恒等处理。
- `Matrix4FromElements` 列主序（e[12]=Tx）。
- 投影生效链=DrawImageEx 的 projMapQuad 分支（image/atlas 链），pattern 逆变换 2×3 不可表达。
- 投影非恒等 ⇒ DrawString fail-closed 不画（P3.7 批 1d E 组，批 2 canvascontract findings 同族）。

## 门禁

```
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/ui_render_base_projection -auto-only
```

首播种 `GPUI_ACCEPT_GOLDEN=1`；卡内容改动需重播种。
