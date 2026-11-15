# ui_render_base_stylectx — Context 样式/路径直控族直驱真窗

O12 批 1b W4（RENDER_FEATURE_COVERAGE §12 NONE → examples 能力测试，台账 O13）。
直驱窗（`wrkit.OpenDirect` + `raster.RunGLSync` 纪律，见 `wrkit/directdrive.go`）。

## 覆盖面（O12 NONE 条目）

`Context.SetHexColor / SetDashOffset / GetStroke / SetStrokeBrush / StrokeBrush /
FillBrush / SetEffectSurface / PipelineMode / SetPipelineMode / RasterizerMode /
SetRasterizerMode / SetPath / NewSubPath / DrawRRect / DrawRoundedRectangleXY /
FillRectCPU / InvertY`。

- 卡 A 样式快捷与读回：SetHexColor 直画（#FF8040 逐像素）、StrokeBrush 直画、
  GetStroke 读回（宽/虚线数组/偏移全链）、FillBrush/StrokeBrush 读回。
- 卡 B 虚线相位：同 [24,16] 图样 offset 0 vs 13，错相可观察（相位探针一真一假）。
- 卡 C 模式切换：PipelineMode RenderPass/Compute + RasterizerMode Analytic——
  模式读回=设置值，各档独立帧同场景像素正确。
- 卡 D 路径直控：SetPath 单子路 + NewSubPath（文档化 no-op）+ 直接 MoveTo 双子路环
  （对绕=洞）+ DrawRoundedRectangleXY 椭圆角 + FillRectCPU（干净上下文探）+ InvertY
  （自带 Translate(0,h)+Scale(1,-1)，镜像位探针）。

## 发现（先红钉子已落，见台账 O13）

**`SetPath(≥2 子路) → Fill` 在 GPU 模式下 op 入队但画面为空（无错误）**——CPU 光栅
同一路径完美出环（oracle）。ParseSVGPath 的多子路 `<path d="M…M…">` 流程全中。
永久钉子：`render/setpath_multisub_fill_test.go`（GPU 腿红/CPU 腿绿），修复后转绿；
修复面在 `render/internal/gpu` FillPath 链，另批处理。本窗 D 卡改用已证可用形态。

## 已知限界

- `RasterizerAnalytic` 强制 CPU 光栅器：填充进 pixmap，GPU 会话的视图同步读回会
  盖掉——Analytic 档在干净上下文上探（与 FillRectCPU 同族语义）。
- `FillRectCPU` 直写在有 GPU 会话+已标记视图的 dc 上经 `Image()` 不可见（视图同步
  盖掉 CPU 直写）——干净上下文（无视图被标记）可探，语义已记录。
- 8px 描边带/10px 虚线在 0.375 缩略图中约 3px：快照点探受 AA 边缘限制，改由腿读回
  探针 + 黄金图逐位钉死。

## 证据链

逻辑 2+22、像素 6、Golden 逐位 0.0000%（状态带金区外）、抓屏 ≤1%。

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/ui_render_base_stylectx -auto-only   # 门禁
go run ./examples/ui_render_base_stylectx                            # 常驻人工窗
```

黄金图基线按 native wgpu 播种（`GPUI_ACCEPT_GOLDEN=1` 播种后人工过目再提交）。
