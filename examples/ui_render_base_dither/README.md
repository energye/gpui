# ui_render_base_dither — `SetDither` / `Dither` 真窗

A 组第 12 项（`RENDER_FEATURE_COVERAGE` §1 #34）的独立真窗。验证 `render.Context.SetDither` / `Dither` 的像素后处理契约，并把三条已确认限界摆到画面上。

## 运行

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

# 门禁：5 秒脚本跑，stdout 出 §2.2 JSON，失败退出 1
RUN_SECONDS=5 go run ./examples/ui_render_base_dither -auto-only

# 人工常驻：开窗直到手动关闭（事件写日志并改标题）
go run ./examples/ui_render_base_dither

# 人工 + 15 秒自动收尾
go run ./examples/ui_render_base_dither -manual-seconds 15
```

Golden 首次播种（只在基图不存在时生效，播种后必须复跑确认 `0.0000%`）：

```bash
GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=5 go run ./examples/ui_render_base_dither -auto-only
RUN_SECONDS=5 go run ./examples/ui_render_base_dither -auto-only
```

## 画面（1200×800，四卡）

| 卡 | 内容 | 证明什么 |
|---|---|---|
| A 线性 | 同一渐变 off / on；右下 4× 最近邻放大；右侧差异×64 | 抖动后颜色数上升、均值偏移有界、差异图可见；4× 放大证明读回图逐像素落到屏幕 |
| B 径向 | 同一径向渐变 off / on | 径向画刷同样吃后处理，颜色数上升、off/on 逐像素不同 |
| C 扫掠 | 同一扫掠渐变 off / on | 扫掠画刷同样吃后处理，四象限色相跨度保持 |
| D 纯色 | 同一纯色 off / on；4× 放大；差异×64 | **已知限界 1**：后处理作用于整张 pixmap，纯色也被改写；差异图显示有序 Bayer 图案 |

指标行浮在画面左上角，Golden 时被盖住；Golden 只比表头 + 图例 + 四卡（627200px），逐位零容差。

## 证据

- 逻辑 16/16：nil 安全、默认关、开关状态、线性/径向/扫掠生效、颜色多样性上升、均值偏移 ≤3/255、alpha 保持、确定性、画后开仍生效、显式关=默认关、Bayer 4×4 周期、纯色限界、重复读回限界、关闭后不再叠加。
- 像素 29/29：11 个点探针（oracle=同参数 `ColorAt` 在像素中心）+ 15 个区域探针（渐变跨度、径向中心、扫掠色相、纯色均匀、off/on 差异比例、4× 最近邻块、差异图非黑）+ 3 个烘焙图探针（Bayer 周期、两张差异图）。
- Golden：表头 + 图例 + 四卡 `627200px`，`diff=0.0000%`。
- 抓屏：`xwininfo` 取客户区 + `ffmpeg x11grab`，与快照同区 `diff=0.0000%`（per-channel tol ±2/255）。
- GPU 路线：窗内 12 张读回图由 GPU 烘焙（`dither_gpu_ops=12`、`dither_cpu_fallback_ops=0`）；窗口绘制 `gpu_ops=54303`、`cpu_fallback_ops=0`。
- 门禁 JSON：`present_policy=full_paint`、`vsync_source=true`、`presents=300`、`fps_gate=off_correctness`（正确性窗，不拿 fps 当及格线）。

## 三条限界（`RENDER_FEATURE_COVERAGE` §6）

1. **整张 pixmap 后处理**：`SetDither(true)` 不区分渐变/纯色，所有像素都按 Bayer 阈值改写；纯色卡就是这条限界的可见证据。
2. **重复读回会累加**：每次 `Image()` / `FlushGPU()` 都会再跑一遍后处理；同一上下文连续读两次像素会继续变化。`SetDither(false)` 只能阻止后续叠加，不能把已经改写的像素还原。
3. **不影响窗口呈现主路**：`FlushGPUWithView` / `PresentWith` 不走这条 pixmap 后处理。本窗展示的是 `Image()` / `FlushGPU()` 的读回结果，不是「窗口呈现自动抖动」的生产接线。

## 性质

原子验证：直接调 `render.Context` 的公开接口，一次验一个能力。示例不算生产接线；`SetDither` 目前仍无生产调用方。
