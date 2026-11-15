# ui_render_base_svgfilter

SVG 滤镜族直驱真窗（O12 批 1d，RENDER_FEATURE_COVERAGE §12 NONE→EX）——七口 Context.Apply* 首次直调。

## 覆盖面（logic 15）

- **卡 A ApplySVGTurbulence**：生成器语义（忽略输入）；同 seed 两缓冲逐位一致；Offset 平移噪场（diff=76800 全画布）；fractalNoise/turbulence 两型互异；`SVGTurbulenceParams` 全字段。
- **卡 B ApplySVGConvolveMatrix**：identity 核逐位无操作（diff=0）；box 核柔化边缘（213,89,89→144,59,59）；`SVGEdgeModeNone/Duplicate/Wrap` 三常量 + Duplicate vs None 边缘差异（1116 px）。
- **卡 C ApplySVGLighting**：`SVGLightingParams` diffuse distant（azimuth/elevation）——输出不透明（a=255）、高度图=alpha 语义；specular point 档 alpha=max(RGB)。
- **卡 D ApplyDilateXY/ApplyErodeXY/ApplyCompositeArithmetic/ApplyColorFilter**：dilate rx≠ry 各向异性（水平 6px 垂直 3px）、erode 缩边、composite k2=1 保留 in1 公式语义、ColorFilterFunc 直域反转（1-x 逐点精确）。

## 引擎发现（语义边界，同 fillrectcpu-sync 族）

- **present→apply 无消费 → CPU 滤镜被视图同步盖掉**：`PresentFrame` 之后不经 `Image()` 消费直接 `Apply*`，滤镜的 CPU 全画布改动在下一次 `Image()` 读回时被视图同步覆盖（连续两轮 present+invert 的读回恒等原图）。**口径：present → Image()（消费同步）→ Apply* → Image()**，全链生效。窗内 flushSF 已内置消费步。与批 1b 的 FillRectCPU 视图同步盖掉同族，但触发面更宽（任何 present 后的 CPU 滤镜）。
- `render/filters` 包空导入是 Apply* 生效前置（CPU apply 回调注册），缺失时七口静默 no-op。

## 限界

- turbulence 噪场高频在缩略图 0.375 缩放下每像素剧烈混色——不设快照点探（腿读回+黄金图钉死）。
- distant 光 60° 仰角下不同高度照度同满亮（r 相同属语义正确）；高度差异由 specular 档对照。
- 滤镜族全程 CPU applyFilterInPlace（FlushGPU 后全画布替换；GPU graph 仅 dilate/erode 等注册节点优先）。

## 门禁

```
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/ui_render_base_svgfilter -auto-only
go run ./examples/ui_render_base_svgfilter                # manual: 常驻
```

首播种 `GPUI_ACCEPT_GOLDEN=1`；卡内容改动需重播种。
