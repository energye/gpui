# ui_render_base_recording — render/recording 直调真窗

B 组「Recording（render_recording）」的真窗。四张卡都由 `recording.Recorder` 录制、`Recording.Playback` 回放到内置 raster 后端，再把回放出的 `image.RGBA` 画到 GPU 窗口；示例本身不手画任何卡片内容。

## API 覆盖

| 族 | 用到的公开 API |
|---|---|
| 录制 | `NewRecorder`、`Recorder` 的 Fill/Stroke/Path/Clip/Transform/Text/Image 动词、`FinishRecording`、`Recording.Width/Height/Commands/Resources/Playback` |
| 命令 | `Save/Restore/SetTransform/SetClip/ClearClip/ClipRoundRect/FillPath/StrokePath/FillRect/StrokeRect/DrawImage/DrawText/StrokeText` 及 `SetFillStyle/SetStrokeStyle/SetLineWidth/SetLineCap/SetLineJoin/SetMiterLimit/SetDash/SetFillRule/SetAntiAlias`；`Command.Type()` 全量核对 |
| 资源池 | `ResourcePool`、`PathRef/BrushRef/ImageRef/FontRef`、`PathCount/BrushCount/ImageCount/FontCount` |
| 后端 | `Backend` 接口、`Register/Unregister/NewBackend/MustBackend/Backends/IsRegistered/Count`、`WriterBackend/FileBackend/PixmapBackend`；`backends/raster` 注册与 `PixmapBackend` 输出 |
| 绘图原语 | 实色/线性渐变画刷、`FillRect` 快速路径、路径 Fill/Stroke、虚线、变换栈、裁剪、`DrawImageScaled`、`DrawString` |
| 文字 | `text.FontSource.Face` + `Recorder.SetFont` + `DrawTextCommand.Font`（`FontRef` 回放时解析出真实 face） |

四卡：

- **A** 渐变、`FillRectangle`、圆、圆角描边、虚线、`Push/Translate/Rotate` 变换。
- **B** `Clip` / `ResetClip`、`Save/Restore`、`ClipRoundRect`。
- **C** `DrawString` 与 `DrawImageScaled`（字体与图标都在 `testdata/`，不依赖系统字体做像素 oracle）。
- **D** 多次实色 `FillRect`、旋转圆、`StrokeRectangle`，并验证同一 `Recording` 重放两次逐位相同。

## 同批修的回放缺陷

新增的 7 个像素回归覆盖 5 个真实缺陷：

1. `Playback` 没有 `StrokeRectCommand` 分支，矩形描边整条丢失。现在回放时转成矩形路径走 `StrokePath`，不再静默吞掉。
2. raster `SetClip` 只把路径设进上下文、没有调用 `Clip()`，`ClearClip` 是空实现。现在走 `render.Context` 的真实裁剪栈：`Clip()` / `ResetClip()`，并验证 `Push/Pop` 恢复。
3. raster `DrawImage` 只发一个未填充矩形，图像一个像素都不出。现在把 `image.Image` 转 `ImageBuf` 后调用 `DrawImageEx`，处理源裁片、目标矩形、最近邻/双线性与 alpha。
4. `DrawTextCommand`/`StrokeTextCommand` 没有携带字体，`Playback` 传 `nil`，raster `DrawText` 直接 no-op。现在命令新增 `Font FontRef`，录制时把 `Recorder.fontFace` 放进 `ResourcePool`，回放时解析回 `text.Face`，raster 后端用 `SetFont` + `DrawString` 真正落字。
5. `ImageBufFromImage` 会把子图归一化到 `0,0` 原点，旧回放又给源矩形加 `Bounds().Min`，非零原点子图会采样到错误像素（实测左上象限变蓝）。现在源矩形按图像相对坐标裁剪；新增 `TestPlaybackDrawImageSubImage` 钉住。

永久回归：`render/recording/backends/raster/backend_paint_test.go` 七例（`StrokeRect`、裁剪+重置、Save/Restore、图像四象限、非零原点子图、文字墨量、线性渐变）。其中子图例专门钉住 `ImageBufFromImage` 归一化到 0,0 后源矩形不能再加 `Bounds().Min`。`go test ./render/recording/...` 全绿。

## 取证

自动档（`RUN_SECONDS=5 -auto-only`）：

- 逻辑 `19/19`：后端注册/未知后端错误、尺寸、22 种命令类型覆盖（contract 录制 23 条命令）、字体引用、资源池四类计数、不可变重放哈希、渐变/矩形/圆/描边矩形/裁剪/重置/圆角裁剪/Save-Restore/图像四象限/文字墨量、spy 后端分发。
- 像素点 `17/17`，墨量区域 `1/1`。
- Golden：表头 + 图例 + 四卡，`627200 px` 逐位 `diff=0.0000%`。
- `presents=301`、`fps_interval≈60.24`、`interval_p95≈17.32ms`、`hitch_count=0`、`gpu_ops=9936`、`cpu_fallback_ops=0`、`gpu_fallbacks=0`。

人工常驻档（`-manual-seconds 15`，退出 0）：

- `presents=898`、`fps_interval≈59.92`、`interval_p95≈17.15ms`、`hitch_count=0`、`gpu_ops=29637`、`cpu_fallback_ops=0`。
- 逻辑/像素/墨量/Golden 与自动档同绿。

字体 `SourceSans3-Regular.otf` SHA-256 前缀 `08df266400933d31`；图标 `recording_icon.png` SHA-256 前缀 `63d2c77837bf26a8`。

## 已知限界

- **原子验证，无生产消费者**：`render/recording` 仍无组件/2.5D/视频调用；PDF/SVG 后端在仓外且未接入本窗。
- **RSS 启动爬坡（非泄漏，2026-10-21 复核）**：5 秒档 `rss_start≈193.8MB → rss_end≈296.0MB`、`rss_slope≈421,273 KB/min` 是窗口/设备初始化爬坡；60 秒实测 `rss_slope≈8,784 KB/min`，稳态在 30k 预算内。`rss_slope_valid` 在 5s/15s 为 false、60s 为 true；`smaps` 增长只在 Go 堆。原“资源债”判断收回；判漏看 `rss_slope_steady_kb_per_min` 且 `rss_slope_valid=true`。
- **未做抓屏 vs 快照**：本窗只有自动快照 + 人工常驻运行，没有像 maskops/backdrop 那样把 `xwd/x11grab` 抓屏写进门禁；这条限界明确记录，不冒充已做。
- **raster 后端**：`StrokeText` 命令仍按填充文字近似（Backend 接口没有 stroke-text 动词）；`SetTransform` 在录制时已把坐标烘进命令，raster 回放按世界坐标处理；`SetAntiAlias` 没有 raster 开关；`DrawImage` 目标矩形是轴对齐 `Rect`；`ClipRoundRect` 在录制时未烘焙变换，带旋转时只能按原始矩形坐标裁剪。以上均记在 §6。
- 只覆盖 raster 后端；`FileBackend/PDF/SVG` 不在本窗范围。
