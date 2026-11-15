# ui_render_base_canvascontract

render.Canvas 接口契约真窗（O12 批 2，RENDER_FEATURE_COVERAGE §12 NONE→EX）：接口方法在 **live `*Context` 与 `recording.Recorder→DrawTo` 回放** 双实现上跑同一脚本，逐位对照。

## 覆盖面（logic 7：四卡双帧逐位 + Measure 双实现）

- **卡 A 状态+变换**：`Push/Pop/Translate/Scale/Rotate/RotateAbout/Shear/GetTransform/SetTransform` + 线宽/虚线状态——双帧 diff=0。
- **卡 B 路径+形状**：`MoveTo..ClosePath/Fill/StrokePreserve/QuadraticTo/CubicTo/SetPath` + `DrawRectangle/RoundedRectangle/Circle/Ellipse/Arc/RRect`——双帧 diff=0。
- **C 图像+文本**：`DrawImage/DrawImageEx/DrawImageRounded/DrawString/DrawStringAnchored/DrawShapedGlyphs`——双帧 diff=0；`MeasureString` live=(82.56,14.00) rec 同值。
- **D 层+裁剪+滤镜**：`PushLayer(0.5)/PushLayerIsolated/PopLayer/ClipRect/ResetClip/ApplyBlur`——双帧 diff=0。

## 发现（两件，修复面已在台账）

- **setpath-multisub（先红钉子复现）**：`SetPath(Append 拼接 ≥2 子路)→Fill` live GPU 空白、Recorder 回放（CPU 路径）正常出画——B 卡契约对照改用 dc 直画双子路已证形态；修复面同 render/internal/gpu FillPath 链。
- **strokestring-empty（洞候选，新）**：`Context.StrokeString`（描边字形=textOutlinePath→doStroke）在 GPU 会话与干净上下文都输出空画面（无墨迹），Recorder 回放侧却能画出描边字形——live 描边字形链路待修（textOutlinePath 展平或描边抵消，与 P3.7 批 1d E 组 fail-closed 注记同族）；登记为 B 组描字专项。

## 限界

- `DrawGPUTexture/TextPath/设备掩码后处理`按接口文档不在 Canvas 契约内（live-only 面），本窗不触；TextureRef 逻辑引用属 recording 面回放能力。
- 双实现对照在 CPU readback 帧上跑（offscreen 直驱），GPU 加速链路的逐位一致性由既有真窗族覆盖。

## 门禁

```
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/ui_render_base_canvascontract -auto-only
go run ./examples/ui_render_base_canvascontract          # manual: 常驻
```

首播种 `GPUI_ACCEPT_GOLDEN=1`；卡内容改动需重播种。
