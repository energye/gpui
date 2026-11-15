# ui_render_base_accelcontract

GPUAccelerator 契约直驱真窗（O12 批 2，RENDER_FEATURE_COVERAGE §12 NONE→EX）。

## 覆盖面（logic 27）

- **卡 A 核心契约**：`Accelerator()`（gpu 空导入注册后 name="sdf-gpu"）× `Name/CanAccelerate 八 op`（fill/stroke/text/circleSDF/rrectSDF=true，scene/image/gradient=false——真设备如实值）、`FillShape/StrokeShape`（SDF 圆+rrect 直调写 CPU readback target，读回像素）、`Flush` 幂等。
- **卡 B 可选能力面**：`CanRenderDirect=true/IsSoftwareAdapter=false/CanCompute=true/SetPipelineMode 切换/MSAASampleCount=1/BeginFrame/SceneStats` 逐接口类型断言读回（未实现接口=登记不判死）。
- **卡 C 硬件剪裁+掩码**：`ClipAware.SetClipRect/ClearClipRect`、`RRectClipAware.SetClipRRect/ClearClipRRect`、`MaskAware.SetMaskTexture/ClearMaskTexture`——直调 err=nil、状态切换无错（画面语义见 findings）。
- **卡 D 文本加速+CoverageFiller**：`DrawGlyphMaskText/Aliased/Shaped` 三口直调（本环境 ErrFallbackToCPU=契约回退语义）；`RegisterCoverageFiller/GetCoverageFiller/FillCoverage` 注入迷你 filler，覆盖值 255×100px 与解析真值全中，注册态还原；`NewGPURenderContext/DropGPURenderContext` 懒建/关往返。

## 发现（语义边界，不判死）

- **clipmask-readback**：`SetClipRect/SetClipRRect/SetMaskTexture` 在 CPU readback 模式（View=nil 自备 Data）下 `FillShape` 直调 err=nil 但 Data 不落地——剪裁/掩码是 GPU 直渲 pass 语义（hal scissor/SDF discard/R8 采样），readback 目标不承载；画面语义由 GPU 直渲窗族覆盖。
- **text-accel-readback**：DrawGlyphMaskText 系返回 ErrFallbackToCPU（gg: falling back to CPU rendering）=契约回退语义，调用方接 CPU 路径；字形掩码画面由 Context.DrawString 生产路径覆盖。

## 限界

- 能力面值随真设备变化（MSAA=1 本机集成显卡），探针只断言接口可调+值如实登记。
- CoverageFiller 注入迷你 filler 属契约验证（FillCoverage 回调 0-255 值域），不是绕过引擎实现。

## 门禁

```
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/ui_render_base_accelcontract -auto-only
go run ./examples/ui_render_base_accelcontract           # manual: 常驻
```

首播种 `GPUI_ACCEPT_GOLDEN=1`；卡内容改动需重播种。
