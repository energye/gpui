# render 公开 API 目录（RENDER_API_CATALOG）

> **一句话**：这是 `render/`（含 `render/text`、`render/scene`、`render/recording`、`render/surface`、`render/svg`、`render/filters`、`render/raster`、`render/gpu`）全部公开 API 的总账，并标注「已接线 / 未接线 / 半成品 / 仅测试」状态。
>
> **维护义务（硬）**：**每次增加、修改、删除 render 公开 API（功能或签名），或改变其接线状态，必须同步更新本文件**（新增/变更的行写进对应分类表 + 更新 §7 状态表），并在主体文档 `docs/ENGINE_UI_WIDGET_RENDER.md` §10 修订表追加一行。判定「公开 API」的口径：`go doc ./render`（以及各子包 `go doc ./render/xxx`）可见的顶层符号与导出方法；方法级变更需同步 §3 与 §8 方法速查表。
>
> 核对命令（每次改动后跑）：
> ```bash
> cd <repo>/render
> grep -hE "^(func|type|var|const) [A-Z]" $(ls *.go | grep -v _test.go) > /tmp/api_check.txt   # 主包顶层
> go doc ./render ./render/text ./render/scene ./render/recording ./render/surface ./render/svg ./render/gpu 2>/dev/null | grep -E "^(func|type|var|const) [A-Z]"  # 包级权威清单
> ```
>
> 状态词约定：✅ 有生产接线且 GPU/CPU 实测有效 · 🔗 有生产接线（真实链路在用）· ⚠️ 半成品/有实现但 GPU 未生效（详见 §7.3）· 🔌 无生产消费者（未接线，仅测试/示例/demo）· 🧪 仅测试覆盖。

---

## 0. 包总览

| 包 | 顶层导出规模（2026-08-15 快照） | 接线状态 | 说明 |
|----|------|------|------|
| `render`（主包） | 类型 94 · 顶层函数 109 · Context 导出方法 181 · const/var 若干 | 🔗 生产主链路 | 即时模式 DC，embedder/真窗全部走这里 |
| `render/text` | 包级导出 226（含字体/整形/布局/光栅化） | 🔗 生产主链路 | 字形子系统，`render` 主包文本 API 的底层 |
| `render/scene` | 顶层 132 | 🔗 render 内部（GPU 后端吃 Scene）；ui/examples 零接线 | 保留模式场景图（Scene/Encoding/Renderer） |
| `render/recording` | 顶层 78 | 🔌 仅测试/示例 | SkPicture 式录制回放；PDF/SVG 后端为仓外模块未接线 |
| `render/surface` | 顶层 47 | 🔌 无消费者 | surface 抽象（ImageSurface/GPUSurface）+ 注册表，未接入 render.Context |
| `render/svg` | 顶层 15 | 🔌 无消费者 | 图标风 SVG 渲染（Render/RenderWithColor/Parse） |
| `render/filters` | 0（仅 init 副作用注册） | 🔗 经 render/gpu 副作用注册 | 让 DC.ApplyBlur 等可用；ui 侧 facade 无生产调用 |
| `render/raster` | 0（仅 init 副作用注册） | 🔌 无消费者 | 独立 CPU tile 光栅注册入口；功能已并入 render/gpu |
| `render/gpu` | 顶层 16（管理 API） | 🔗 真窗/门禁全量接线 | GPU 加速注册 + 设备生命周期/策略管理 |

> text 子树内部还有分层子包（`render/text/hint` 等），非本次快照范围，改动时按其自身文档维护。

---

## 1. 主包：基础类型与几何

> 格式约定：每行 = 「功能说明（类型/函数做什么、关键参数）」+「精简（一句话用途）」。方法级简精见 §8 速查表。

| API | 功能说明 | 精简 | 状态 |
|-----|---------|------|------|
| `RGBA` / `RGB` / `RGBA2` / `FromColor` / `Hex` / `ParseHex` / `HSL` | 颜色值（浮点 0-1 通道 + Alpha）及其构造：RGB(a)各 0-1、Hex 解析 `#RRGGBB[AA]`、HSL 色相环 | 颜色表示与构造 | ✅ |
| var `Black/White/Red/Green/Blue/Yellow/Cyan/Magenta/Transparent` | 常用命名色值常量 | 预置色 | ✅ |
| `Point` + `Pt(x,y)` | 二维点类；Pt 构造一元：x 右增、y 下增（屏幕坐标） | 二维点 | ✅ |
| `Vec2` + `V2(x,y)` / `PointToVec2` | 向量类（Point 的运算增强版）；V2 构造 | 向量 | ✅ |
| `Rect` + `NewRect(p1,p2)` / `NewRectFromPoints` | 矩形（两点对角）；含 Width/Height/Contains/Union | 矩形 | ✅ |
| `Line` + `NewLine(p0,p1)` | 线段几何（求值/分割/包围盒） | 线段 | 🔗 |
| `QuadBez` + `NewQuadBez(p0,p1,p2)` / `CubicBez` + `NewCubicBez(p0,p1,p2,p3)` | 二次/三次贝塞尔几何求值族（Eval/Extrema/Subdivide） | 贝塞尔求值几何 | 🔗 |
| `Matrix` + `Identity/Translate/Scale/Rotate/Shear` | 2D 仿射矩阵（x'=a·x+b·y+c，y'=d·x+e·y+f）；构造原语 + TransformPoint/Invert/ScaleFactor | 2D 仿射矩阵 | ✅ |
| `ColorFunc(x,y) RGBA` | 位置→颜色函数签名（驱动 CustomBrush） | 自定义上色函数 | 🔗 |
| `TextureView = gpucontext.TextureView` | GPU 纹理视图句柄（离屏 RT/层/滤镜结果） | GPU 纹理句柄 | 🔗 |
| `GPURenderTarget` | GPU 呈现目标句柄（加速器填充接口参数） | GPU 目标句柄 | 🔗 |
| `MaxTrackedDamageRects` | 每帧最多跟踪的损伤区数量上限 | 损伤区上限 | 🔗 |
| `Logger()/SetLogger(l)` | 全局日志（slog 实例读写）；SetLogger 替换实例 | 全局日志 | 🔗 |
| `GPUContextCount()` | 活动 GPU 上下文注册数（设备切换/恢复诊断） | GPU 上下文计数 | 🔗 诊断 |
| var `ErrFallbackToCPU` / `ErrNilSurfaceView` | 回退到 CPU 渲染的错误 / 空表面视图错误 | 回退/空表面错误 | 🔗 |
| `SolveQuadratic/SolveCubic/SolveQuadraticInUnitInterval/SolveCubicInUnitInterval` | 多项式求根；InUnitInterval 各版本仅返回 [0,1] 实根 | 多项式求根工具 | 🔗 |
| `SetDefaultSampleCount(n)` / `DefaultSampleCount()` + 常量 `MSAASampleCount1`(1) / `MSAASampleCount4`(4) | 全局默认 MSAA 采样数配置：Set(1)=全局 1x、Set(4)=4x、Set(0)=auto（设备探测 4x 支持，兜底 4x）；未设置时 DefaultSampleCount()=0。唯一配置入口（env `GPUI_SURFACE_SAMPLE_COUNT` 已移除，2026-08），作用于之后创建的 GPU 会话；特效离屏（SetEffectSurface）恒走 1x 不受影响 | 默认采样数配置 | ✅（render/sample_count.go，2026-08） |

## 2. 主包：路径

| API | 功能说明 | 精简 | 状态 |
|-----|---------|------|------|
| `Path` + `NewPath()` | 即时路径：MoveTo/LineTo/QuadTo/CubicTo 追加；Rectangle/Circle/Ellipse/Arc/RoundedRectangle/EllipticalArc 形状；Close/Clone/Transform/Append/Iterate/Verbs/Coords；Bounds/Contains/Area/Winding 等查询 | 即时路径全部操作 | ✅ |
| `PathBuilder` + `BuildPath()` | 声明式路径构建链：MoveTo/LineTo/QuadTo/CubicTo/Rect/RoundRect/Circle/Ellipse/Polygon/Star → 收 Path() | 链式路径构建器 | ✅ |
| `PathVerb`（MoveTo/LineTo/QuadTo/CubicTo/Close） | 路径指令枚举（Iterate 迭代产出） | 路径指令枚举 | ✅ |
| `ApplyDash(p, dash)` | 把虚线序列应用到路径、生成带实线段的虚线化路径（描边内部用） | 路径虚线化 | 🔗 software/GPU 两腿都用 |
| `Dash` + `NewDash(lengths...)` | 虚线间隔序列对象（等长或成对交替） | 虚线序列 | 🔗 |
| `ParseSVGPath(d) (*Path, error)` | 解析 SVG path 数据串为 Path（`M/L/C/Q/A…`） | 字符串→路径 | 🔌（svg 包内部用，svg 包本身无消费者） |
| `PathBooleanOp`（Union/Intersect/Difference/Xor）+ `BooleanPath(a,b,op)` / `Path.Op` | 路径布尔并/交/差/异或运算（非凸、自交路径也支持） | 路径布尔运算 | 🧪 仅测试 |
| `PathMetric`（IsEmpty/Length/PositionAt/TangentAt 等）+ `Path.ComputeMetrics` | 对路径做度量：总长、某弧长处取点/切线 | 路径度量 | 🔗 |
| Path 造型：`Trim` / `WithCorners` / `Discrete` / `Flatten` / `Reversed` / `Area` / `Winding` / `Contains` / `BoundingBox` | Flatten 细分为折线/多边形；Area/Winding/Contains/BoundingBox 查询；Trim(子段)/WithCorners(圆角化)/Discrete(随机点化) 高级造型 | 造型与查询 | 前四项内部用；**Trim/WithCorners/Discrete 🔌 仅测试** |

## 3. 主包：绘制上下文 Context（181 导出方法）

> **构造器与选项**：`NewContext(width,height,opts...)` / `NewContextForPixmap(pm)` / `NewContextForImage(img,opts...)` / `NewContextWithScale(w,h,scale)`；`ContextOption` 模式：`WithRenderer/WithPixmap/WithPipelineMode/WithDeviceScale`。状态 ✅（embedder 生产链路）。

> 按文件分组；⚠️/🔌 标注仅为该行方法，未标注行默认 ✅/🔗。

### 3.1 context.go（105）
> 功能/精简按族归纳（§3 逐方法见 §8 速查由 go doc 兜底；族内未标状态者默认 ✅/🔗）。

| 方法族 | 功能（做什么） | 精简 | 状态 |
|--------|--------------|------|------|
| MoveTo/LineTo/QuadraticTo/CubicTo/ClosePath/NewSubPath/GetCurrentPoint | 当前路径的锚点移动、直线/贝塞尔追加、子路径闭合 | 构建路径轮廓 | ✅ |
| DrawArc/DrawCircle/DrawEllipse/DrawEllipticalArc/DrawLine/DrawPoint/DrawRectangle/DrawRoundedRectangle/DrawRoundedRectangleXY | 把圆/椭圆/弧/线/点/矩形/圆角矩形追加进当前路径（再用 Fill/Stroke 绘制） | 形状追加 | ✅ |
| AppendPath/DrawPath/SetPath/ClearPath | 整条路径的拼接、代入、替换、清空 | 路径整体操作 | ✅ |
| Fill/FillPreserve/FillPath/FillRectCPU/Stroke/StrokePreserve/StrokePath | 按当前画刷/描边参数填充或描边；Preserve 保留路径；RectCPU 强制 CPU 矩形填充 | 绘制（填/描） | ✅ |
| SetRGB/SetRGBA/SetHexColor/SetColor/SetFillBrush/SetStrokeBrush/SetFillRule/SetLineWidth/SetLineCap/SetLineJoin/SetMiterLimit/SetStroke/SetDash/SetDashOffset/SetBlendMode/FillBrush/StrokeBrush/GetStroke/IsDashed/ClearDash | 设置绘制时的颜色/画刷/填充规则/线宽/线帽/线接/斜接/虚线/混合模式，并读取当前值 | 样式状态 | ✅ |
| Push/Pop/Translate/Scale/Rotate/RotateAbout/Shear/Transform/SetTransform/Identity/GetTransform/TransformPoint/InvertY | 用户变换栈：矩阵平移/缩放/旋转/错切/替换/取回/变换点 | 变换栈 | ✅ |
| SetAntiAlias/AntiAlias | 开启/关闭抗锯齿（后续绘制生效） | 抗锯齿开关 | ✅ |
| Width/Height/PixelWidth/PixelHeight/DeviceScale/SetDeviceScale/Resize/ResizeTarget | 画布逻辑/物理尺寸、HiDPI 缩放比、resize（含底层 pixmap） | 尺寸与缩放 | ✅ |
| Clear/ClearWithColor/SetPixel/WritePixels/Image/EncodePNG/EncodeJPEG/SavePNG | 清空画布、写像素、导出 image/PNG/JPEG/存盘 | 画布输出 | ✅ |
| SetDamageTracking/FrameDamage/FrameDamageUnion/TrackDamageRect/ResetFrameDamage | 逐操作损伤矩形跟踪（present 决策/增量重绘用） | 损伤跟踪 | ✅ |
| RasterizerMode/SetRasterizerMode/PipelineMode/SetPipelineMode/SetTextMode/TextMode/SetLCDLayout | 选择 CPU 栅格化器、GPU 管线模式（render pass/compute）、文本策略、LCD 子像素布局 | 渲染模式 | ✅ |
| SetEffectSurface/SetSharedEncoder/CreateSharedEncoder/SubmitSharedEncoder | 特效离屏强制 1x 采样、共享命令编码器（单命令缓冲帧，ADR-017） | 离屏/编码器 | ✅ |
| BeginGPUFrame/FlushGPU/FlushGPUWithView/FlushGPUWithViewDamage/FlushGPUWithViewDamageRects/GPURenderContext/DropGPURenderContext | 每帧 GPU 状态重置、把累积 GPU 命令提交并解析到 pixmap/视图（带单/多损伤区）、per-context GPU 会话 | GPU 提交 | ✅ |
| RenderPathStats/ResetRenderPathStats/LastCPUFallbackReason/MemDigCmdBufs/Close | GPU/CPU 路由计数、最近 CPU 回退原因、残留命令缓冲诊断、关闭释放 | 诊断/资源 | 🔗 |

### 3.2 context_clip.go（5）· 裁剪族
| 方法 | 功能 | 精简 | 状态 |
|------|------|------|------|
| `ClipRect(x,y,w,h)` | 矩形裁剪区（设备坐标），GPU 优先走硬件 scissor | 矩形裁剪 | ✅ GPU 有效（实测） |
| `ClipRoundRect(x,y,w,h,r)` | 圆角矩形裁剪（GPU scissor+SDF / CPU 逐像素 SDF） | 圆角矩形裁剪 | ✅ GPU 有效（单测） |
| `Clip()` / `ClipPreserve()` | **任意路径裁剪**（stencil+depth 管线；Preserve 保留路径） | 任意路径裁剪 | ⚠️ **GPU 真窗实测画穿，见 §7.3** |
| `ResetClip()` | 清除全部裁剪，恢复全画布 | 清除裁剪 | ✅ |
| `ClipRectOp(x,y,w,h,op)` / `ClipPathOp(op)`（ClipOpIntersect/Difference/Replace） | 带 ClipOp 运算的矩形/路径裁剪（非相交运算） | 运算式裁剪 | 🧪 仅测试（默认 intersect 是日常路径） |

### 3.3 context_mask.go（6）· 掩码族
| 方法 | 功能 | 精简 | 状态 |
|------|------|------|------|
| `SetMask(mask)` / `ClearMask()` / `GetMask()` / `InvertMask()` | 设置/清除/读取/反转后续绘制的 alpha 蒙版（GPU 侧同步上传 R8 纹理） | alpha 蒙版 | ✅ GPU 有效（内部 clip 差集上传在用） |
| `ApplyMask(mask)` | 对既有像素做 DestinationIn 蒙版合成（后处理帧，非影响未来绘制） | 蒙版后处理 | 🔗 |
| `AsMask()` | 把当前未填充路径栅格化为 Mask（临时光栅化取 alpha） | 路径→掩码 | 🔗 |
| `Mask` 类型：`NewMask/NewMaskFromAlpha/NewLuminanceMask/NewMaskFromData` + At/Set/Fill/Invert/Clone | 蒙版构造（空/alpha 图/亮度图/裸数据）与读写 | 蒙版构造 | 🔗 内部在用；**ui/examples 无 SetMask 生产调用 🔌** |

### 3.4 context_layer.go（7）· 图层族
| 方法 | 功能 | 精简 | 状态 |
|------|------|------|------|
| `PushLayer(blend, opacity)` / `PopLayer()` | 压入/合成渐变层（GPU RT 优先，CPU 损伤合成兜底） | 层合成 | ✅ GPU 有效 |
| `PushLayerIsolated(opacity)` | 强制创建隔离离屏层（Flutter saveLayer 语义） | 隔离离屏层 | ✅ |
| `PushMaskLayer(mask)` | 建层并让整个层内容经 mask 调制后合成 | 蒙版层 | ✅ |
| `PushBackdropLayer(blend, opacity)`（m4_extensions.go） | 用父画布快照预填充层（backdrop/毛玻璃底色播种） | 背景快照层 | 🔗 ui/scene/composite + ui/rendering/filter_draw 在用 |
| `LayerPoolStats()/ResetLayerPoolStats()` | 读取/清零层表面池 gets/puts/hits/misses | 层池统计 | 🔗 |
| `SetBlendMode(mode)` | 设置后续填充/描边的混合模式 | 混合模式 | ✅ |

### 3.5 context_image.go（13）· 图像族
| 方法 | 功能 | 精简 | 状态 |
|------|------|------|------|
| `DrawImage/DrawImageEx` | 绘制图像（走 GPU QueueImageDraw；**Bicubic 显式回退 CPU**） | 图像绘制 | ✅ GPU 有效（双三次除外） |
| `DrawImageCircular/DrawImageRounded` | 在圆形/圆角矩形裁剪区内绘制图像 | 裁剪内画图 | 🔗 |
| `DrawImageQuad(corners)`（m4_extensions.go） | 四角自由变换绘制（透视/梯形） | 透视贴图 | 🧪 仅测试 |
| `DrawImageNine`（nine_patch.go） | 九宫格缩放绘制图像（四角不变、边拉伸） | 九宫格缩放 | 🔗 ui/rendering/image_draw.go 已接门面；**无 Widget 生产调用 🔌** |
| `DrawGPUTexture` / `DrawGPUTextureBase` / `DrawGPUTextureWithOpacity` / `DrawGPUTextureWithOpacityUV` | GPU 纹理直接合成（Base 作 render pass 背景层零读回；Opacity/UV 带不透明度/子区） | GPU 纹理合成 | ✅ |
| `ExportImageBuf(dst **ImageBuf) bool` | 把画布导出/复用为图像缓冲（特效连续帧免分配） | 画布→缓冲 | ✅ |
| `CreateOffscreenTexture(w,h)` | 创建离屏 RT 纹理视图（层/滤镜目标） | 离屏纹理 | ✅ |
| `CreateImagePattern/SetFillPattern/SetStrokePattern` | 用图像区域建立可平铺图案并作为填/描画刷 | 图案纹理着色 | 🧪 仅测试（生产走 Brush） |
| `IsAdvancedBlendMode(mode)` | 判断混合模式是否为高级（非基础 Porter-Duff 集） | 高级混合判定 | 🔗 |
| `ImagePattern` 类型 + `Pattern`/`SolidPattern`/`PatternFromBrush`/`BrushFromPattern` | 图案类型及其与画刷的双向桥接 | 图案桥接 | 🧪 仅测试/桥接 |
| `ImageBuffer`：`ImageBuf`(+`NewImageBuf`/`ImageBufFromImage`/`LoadImage`/`LoadWebP`) / `Pixmap`(+`NewPixmap`/`NewPixmapFromBuffer`/`FromImage`，方法见 §8) / 像素格式 | 图像缓冲与 CPU 像素缓冲的构造与加载 | 图像/像素缓冲 | ✅ |

### 3.6 filter_ops.go（8）· 滤镜族
| 方法 | 功能 | 精简 | 状态 |
|------|------|------|------|
| `ApplyBlur/ApplyBlurXY/ApplyColorMatrix/ApplyDropShadow/ApplyGrayscale/ApplyInvert` | 对当前内容做高斯模糊/颜色矩阵/投影/灰度/反色滤镜 op | 内容滤镜 | 🔗（需 blank-import render/filters 生效，gpu 包已做） |
| `ApplyImageFilterGraph(nodes...)` / `GPUFilterTexture()` | 建 GPU 图像滤镜图（零读回发布）并取结果纹理 | GPU 滤镜图 | 🔗 有 GPU 图注册时生效；ui 侧 facade 无生产调用 |
| `ImageFilterKind`（Blur/BlurXY/ColorMatrix/DropShadow/Grayscale/Invert）+ `ImageFilterNode` | 滤镜图节点类型/数据 | 滤镜节点类型 | 🔗 |
| 注册函数：`RegisterFilterOps` / `RegisterGPUFilterGraph` / `RegisterGPUFilterGraphTexture` / `RegisterGPUFilterGraphFromView` / `SwapGPUFilterGraph` / `FiltersRegistered` / `GPUFilterGraphRegistered` / `FilterPoolStats` / `ResetFilterPoolStats` / `DisableGPUFilterGraphForTest` | 注册滤镜 op 与 GPU 图、统计池、测试开关 | 滤镜注册/统计 | 🔗/🧪 混合（GPU 图注册多为测试/宿主注入） |

### 3.7 frame.go（6）+ present.go（3）+ present_target.go · 帧与呈现族
| API | 功能 | 精简 | 状态 |
|------|------|------|------|
| `BeginFrame/Invalidate/MarkFullRedraw/ResetFrameDamage` | 帧生命周期：重置统计、标记损伤、强制全量重绘 | 帧生命周期 | ✅ |
| `PresentFrame/PresentFrameDamage/PresentFrameDamageRects`（present.go） | 提交 GPU 内容到视图并调用 present（带单/多损伤区） | 呈现提交 | ✅ |
| `PresentFrameAuto/PresentFrameFull`（frame.go） | 自动选路径 / 强制全帧呈现 | 自动/全帧呈现 | ✅（PresentFrameAuto 内部用） |
| `PlanFramePresent/PlanPresent/FramePresentPlan/PresentOutcome/PresentMode` | 依据损伤区与 surface 尺寸规划呈现策略 | 呈现策略规划 | ✅（PresentFrameAuto 内部用） |
| `CoalesceDamageRects` | 把多条损伤矩形合并/裁剪到上限 | 损伤合并 | 🔗 |
| `PresentTarget` + `NewPresentTarget`（方法：Context/Resize/PresentWith/PresentWithAuto/PresentClear/LastPresentOutcome/LastDamageAreaPx/InFullRecovery/SetResizeStormWindow/LogicalSize/Scale/Close） | 呈现目标对象（X11/Wayland/Win32/AppKit；风暴 resize 状态机） | 呈现目标 | ✅ embedder 在用（R4 风暴窗口状态机所在） |
| `PresentNativeSurface` / `PresentPlatform` / var `ErrNilSurfaceView` | 原生表面句柄、平台枚举、空表面错误 | 原生表面/平台 | ✅ |

### 3.8 text.go（15）+ text_decoration.go（2）+ text_mode.go · 文本族
| 方法 | 功能 | 精简 | 状态 |
|------|------|------|------|
| `SetFont/Font/LoadFontFace/LoadFontFaceWithVariations/FontVariationAxes` | 设置字体、读当前字体、加载 TTF/OTF 与可变字体轴 | 字体管理 | ✅ ui/rendering paint_context 在用 |
| `DrawString/DrawStringAnchored/DrawStringWrapped/DrawShapedGlyphs/SetTextDecoration/TextDecoration` | 绘制文本（锚点/自动换行/已整形字形/下划线等装饰） | 文本绘制 | ✅（DrawString/Wrapped 生产在用；**StrokeString/Anchored/TextPath/DrawShapedGlyphs 🧪 仅测试**） |
| `MeasureString/MeasureMultilineString/WordWrap` | 度量单行/多行文本、按宽度断词换行 | 文本度量 | 🔗 内部用（UI 布局未接，用自研估算） |
| `TextMode`（Auto/MSDF/Vector/Bitmap/GlyphMask/Aliased）/ `LCDLayout`（None/RGB/BGR）/ `Align` | 文本渲染策略 / LCD 子像素布局 / 对齐枚举 | 文本模式 | ✅ |

### 3.9 vertices.go（3）+ shapes.go（1）· 顶点/网格族
| 方法 | 功能 | 精简 | 状态 |
|------|------|------|------|
| `DrawVertices(pos,cols,mode)` / `VertexMode` | 顶点数组绘制（三角扇/条带/列表） | 顶点数组 | 🔗 |
| `DrawMesh(mesh)` / `Mesh` | 网格绘制（膨胀路径转三角形） | 网格绘制 | 🔗 |
| `DrawAtlas(img,sprites)` / `AtlasSprite` | 单纹理图集批量精灵绘制 | 图集绘制 | 🔗 |
| `DrawRegularPolygon(n,x,y,r,rot)` | 正多边形形状追加到路径 | 正多边形 | 🔗 |

### 3.10 m4_extensions.go（其余）
| 方法 | 功能 | 精简 | 状态 |
|------|------|------|------|
| `SetDither/Dither` | 启用/关闭解析后有序抖动 | 抖动 | 🧪 仅测试 |
| `DrawImageQuad` / `PushBackdropLayer` | 见 §3.5 / §3.4 | — | — |

---

## 4. 主包：光栅化 / 加速 / 渲染器

| API | 功能 | 精简 | 状态 |
|-----|------|------|------|
| `Renderer` 接口（Fill/Stroke） | 渲染器抽象：实现方提供填充/描边绘制 | 渲染器抽象 | 🔗 |
| `SoftwareRenderer` + `NewSoftwareRenderer` | CPU 软渲染器（像素精确、作为 GPU 不可用兜底） | CPU 软渲染 | 🔗（CPU 兜底） |
| `SDFFilledCircleCoverage/SDFCircleCoverage/SDFFilledRRectCoverage/SDFRRectCoverage` | SDF 覆盖率函数（像素点→形状内覆盖率，填/描边） | SDF 覆盖率 | 🔗（CPU SDF/测试） |
| `SDFAccelerator`（无构造器 `&render.SDFAccelerator{}`） | 独立注册的 CPU SDF 加速器（形状快速填充） | CPU SDF 加速 | 🧪 生产仅作为 GPU 加速器内部 cpuFallback |
| `GPUAccelerator` 接口 + `Accelerator()/RegisterAccelerator/CloseAccelerator/SetAcceleratorDeviceProvider/PurgeAcceleratorSurfaceResources/AbandonAcceleratorDevice/AcceleratorCanRenderDirect/BeginAcceleratorFrame` | GPU 加速器注册/取回/关闭、设备提供者注入、表面资源清理、设备放弃、可直渲判定、帧开始 | GPU 加速管理 | ✅（gpu 包注册，全部真窗在用） |
| 加速器能力接口群：`DeviceProviderAware/GPURenderContextProvider/FrameAware/MSAAAware/GPUTextAccelerator/GPUGlyphMaskAccelerator/GPUAliasedTextAccelerator/GPUShapedTextAccelerator/DirectRenderCapable/AdapterAware/ComputePipelineAware/PipelineModeAware/ForceSDFAware/ClipAware/RRectClipAware/PathClipAware/LCDLayoutAware/MaskAware/SceneStatsTracker` | 加速器可探测/可选择性实现的各项能力接口（宿主据此选路径） | 能力探测接口 | 🔗 |
| `AcceleratedOp`（Fill/Stroke/Scene/Text/Image/Gradient/CircleSDF/RRectSDF） | 加速操作位标记（加速器声明支持哪些 op） | 加速操作位 | 🔗 |
| `CoverageFiller` + `RegisterCoverageFiller/GetCoverageFiller` + `ForceableFiller`（SparseFiller/ComputeFiller） | 覆盖率填充器接口与注册；AdaptiveFiller（4x4/16x16 tile）在此注册 | 覆盖率填充器 | ✅ |
| `RasterizerMode`（Auto/Analytic/SparseStrips/TileCompute/SDF） | CPU 栅格化算法选择 | CPU 栅格化模式 | ✅ |
| `PipelineMode`（Auto/RenderPass/Compute）+ `SelectPipeline` + `SceneStats` | GPU 管线模式选择（自动/渲染通道/计算） | GPU 管线模式 | ✅（SelectPipeline 仅测试调用 🔌） |
| `DetectedShape/ShapeKind/DetectShape` | 从路径检测形状种类（Rect/RRect/Circle/Ellipse/Arc）供 SDF 快速路径 | 形状检测 | 🔗（Arc 检测仅测试） |
| `RenderPathStats`（方法：LogLine） | GPU/CPU 路由计数快照（P1 门禁诊断，LogLine 输出日志行） | 路由计数 | 🔗 |
| `CreateSharedEncoder/SubmitSharedEncoder` | 共享命令编码器创建与提交（ADR-017 单命令缓冲帧） | 共享编码器 | 🔗 |

---

## 5. 主包：画笔 / 渐变 / 描边 / 绘制状态

| API | 功能 | 精简 | 状态 |
|-----|------|------|------|
| `Brush` 接口 / `Solid` / `SolidRGB` / `SolidRGBA` / `SolidHex` + `SolidBrush` 方法（WithAlpha/Opaque/Transparent/Lerp） | 画刷接口与实色画刷构造（RGB/RGBA/十六进制/透明度链式） | 实色画刷 | ✅ |
| `LinearGradientBrush` + `NewLinearGradientBrush`（AddColorStop/SetExtend/ColorAt） | 两端点线性渐变画刷（色标/边界模式/求色） | 线性渐变 | ✅（水滴/矢量真窗实测） |
| `RadialGradientBrush` + `NewRadialGradientBrush`（SetFocus/AddColorStop/SetExtend/ColorAt） | 圆心发散的径向渐变（支持偏心焦点） | 径向渐变 | ✅（水滴真窗实测） |
| `SweepGradientBrush` + `NewSweepGradientBrush` / `SweepGradient` | 圆锥（扫描式）渐变画刷 | 圆锥渐变 | ✅ |
| `ExtendMode`（Pad/Repeat/Reflect）/ `ColorStop` | 渐变越界采样方式与色标 | 渐变参数 | ✅ |
| `CustomBrush` + `NewCustomBrush/Checkerboard/HorizontalGradient/VerticalGradient/LinearGradient/RadialGradient/Stripes` | 以位置函数( ColorFunc)驱动的自定义画刷与内置纹理 | 函数式画刷 | 🔗 |
| `Stroke` + `DefaultStroke/Thin/Thick/Bold/RoundStroke/SquareStroke/DashedStroke/DottedStroke`（WithWidth/WithCap/WithJoin/WithMiterLimit/WithDash/…） | 描边参数对象及预设（线宽/帽/接/斜接/虚线链式） | 描边参数 | ✅ |
| `Paint` + `NewPaint`（SetColor/SetLineWidth/EffectiveLineWidth/EffectiveDash/…） | 绘制状态聚合（画刷+描边+生效值，Surface/旧路径用） | 绘制状态 | 🔗 |
| `LineCap/LineJoin/FillRule` 及常量 | 线帽/线接/填充规则枚举 | 描边/填充规则 | ✅ |
| `Painter` / `SolidPainter` / `FuncPainter` / `PainterFromPaint` | 像素填色抽象（软件逐像素上色接口） | 像素填色 | 🧪 仅测试（软渲染未启用） |

---

## 6. 子包目录

### 6.1 render/text（包级导出 226）
| 族 | 代表 API | 功能 | 精简 | 状态 |
|----|---------|------|------|------|
| 字体与文件 | `RegisterParser/FontSource/FontSourceID/LoadFontFace*/ClearSystemFontPaths/ErrEmptyFontData/ErrUnsupportedFont…` | 字体解析器注册、字体源身份、字体文件加载、系统字体路径清理、字体错误 | 字体加载/解析 | 🔗 |
| 整形 | `Shape/ShapeResult/ShapedGlyph/RunAdvance/CaretXForCluster/HitTestCluster/…` | 文本整形（复杂文字/阿拉伯、泰文等）、字形序列与簇命中 | 文本整形 | 🔗 |
| 绘制 | `Draw/DrawAliased/DrawWithEmoji/Measure/MeasureText` | 字形到目标图像的绘制（含别名/emoji）、文本度量 | 字形绘制 | 🔗 |
| 度量/量化 | `Quantize/QuantizePoint/SubpixelMode/SubpixelConfig` | 子像素量化（LCD/AA 的次像素定位） | 子像素量化 | 🔗 |
| 缓存 | `ClearShapeResultCache/ClearMultiFaceRunsCache/ClearAutoHintCache/ClearFontScanFallbackCache/ResetShapeResultCacheStats` | 各缓存清理与统计复位（整形/多面/自动 hint/字体扫描） | 缓存管理 | 🔗 |
| 光栅 | `RasterizeFT26/GlyphMaskFlags(ADI/LCD…)` | 轮廓点阵光栅化、字形掩码标志 | 字形光栅化 | 🔗 |
| 特性/角色 | `FontRole/FontFeature/TabularNums/AxisWeight/DefaultMultiFontRoleChain/UnicodeRange/RangeBasicLatin/IsCJK/IsPunctuation/IsWhitespace…` | 字体角色链、OpenType 特性、Unicode 区间/分类判定 | 字体特性/分类 | 🔗 |
| 错误/变量 | `ErrCFF2Unsupported/ErrUnsupportedFontType/…`、`DefaultTabWidth` 等 | 字体/格式错误、制表符宽默认 | 错误/常量 | — |

> 完整 226 项请以 `go doc ./render/text` 为准（本表为族级归纳，改动文本 API 时在对应族补行）。

### 6.2 render/scene（顶层 132）
| 族 | 代表 API | 功能 | 精简 | 状态 |
|----|---------|------|------|------|
| 形状 | `Shape/PathBuilder`；Rect/RoundedRect/Circle/Ellipse/Line/Path/Polygon/RegularPolygon/Star/Arc/Pie/Transform/Composite Shape + New*Shape | 保留模式形状集合（矩形/圆角/圆/椭圆/线/路径/多边形/星/弧/扇/变换/组合） | 形状集合 | 🔗（render 内部 GPU 后端在用；ui/examples 零接线） |
| 场景 | `Scene/NewScene/NewImage/ScenePool` | 场景图累积 + 图像资源 + 池 | 场景图 | 🔗 |
| 编码 | `Encoding/NewEncoding/Iterator/Decoder/NewDecoder/EncodingPool/GetEncoding/PutEncoding/DefaultPool` | 场景→不可变编码（可迭代/解码/缓存复用） | 场景编码 | 🔗 |
| 渲染 | `Renderer/NewRenderer/NewGPUSceneRenderer/CanUseGPU/TextRenderer/TextRendererPool/RenderStats` | CPU 分块渲染器与 GPU 场景渲染器、文本渲染、统计 | 场景渲染 | 🔗 |
| 构建 | `SceneBuilder/NewSceneBuilder(From)/Tag/TaggedBounds/DamageTracker/LayerCache/CacheEntry/CacheStats/Filter/FilterType/FilterChain` | 声明式构建、标签、损伤、层缓存、滤镜链 | 构建/缓存 | 🔗 |
| 层/裁剪 | `LayerKind/LayerState/LayerStack/ClipState/ClipStack` | 图层栈与裁剪栈（retained 层绘） | 层/裁剪栈 | 🔗 |
| 编码类型 | `Brush/SolidBrush/StrokeStyle/BlendMode/Affine/Rect/TextFlags/GlyphRunData/…` | 编码所需画刷/描边/混合/仿射/文本数据类型 | 编码数据类型 | 🔗 |

### 6.3 render/recording（顶层 78）
| 族 | 代表 API | 功能 | 精简 | 状态 |
|----|---------|------|------|------|
| 录制 | `Recorder/NewRecorder/Recording` | 录制绘制命令 → 不可变 Recording | 命令录制 | 🔌 仅测试/示例 |
| 命令 | `Command 族（Save/Restore/SetTransform/SetClip/ClearClip/ClipRoundRect/FillPath/StrokePath/FillRect/StrokeRect/DrawImage/DrawText/StrokeText/样式命令…）+ FillRule/LineCap/LineJoin/Stroke/ImageOptions` | 可回放的绘制/状态命令行（SkPicture 风格） | 录制命令 | 🔌 |
| 回放 | `Backend/BackendFactory/NewBackend/MustBackend/Register/Unregister/Backends/IsRegistered/Count`；`WriterBackend/FileBackend/PixmapBackend`；`CommandType/Command/InvalidRef`；`backends/raster` 内置 | 后端注册表与按名回放（内置 raster） | 后端回放 | 🔌（无生产消费者；PDF/SVG 后端仓外未接） |
| 资源 | `ResourcePool/FontRef/PathRef/BrushRef/ImageRef` + 画刷（Solid/Linear/Radial/Sweep/Pattern/gradient stop/RepeatMode/ExtendMode） | 命令引用池与画刷类型 | 资源池/画刷 | 🔌 |

### 6.4 render/surface（顶层 47）
| 族 | 代表 API | 功能 | 精简 | 状态 |
|----|---------|------|------|------|
| Surface | `Surface/SubSurface/ResizableSurface/ClippableSurface/BlendableSurface/CapableSurface/Capabilities` | 渲染目标抽象接口家族（绘制/子面/resize/裁剪/混合/能力） | surface 接口 | 🔌 无消费者 |
| 实现 | `ImageSurface/NewImageSurface(FromImage)`、`GPUSurface/NewGPUSurface` + `GPUBackend` | CPU 图像面与 GPU 面（需外部 backend） | 面实现 | 🔌 |
| 注册表 | `Registry/NewRegistry/Register/Unregister/List/Available/Get/NewSurface*/SurfaceFactory/RegistryEntry/BackendNotFoundError/BackendUnavailableError/ErrNoBackendAvailable` | surface 工厂注册与按名创建、可用性查询 | 注册表 | 🔌 |
| 类型 | `Path/Point/FillRule/LineCap/LineJoin/FillStyle/StrokeStyle/DrawImageOptions/Pattern/SolidPattern/Filter/BlendMode/Options` | surface 绘制所需类型 | 绘制类型 | 🔌 |

### 6.5 render/svg（顶层 15）
| API | 功能 | 精简 | 状态 |
|-----|------|------|------|
| `Render(data,w,h)/RenderWithColor(data,w,h,c)` | 解析图标风 SVG 数据并栅格化为 RGBA 图（可选主题色） | SVG→图像 | 🔌 无消费者 |
| `Parse(data)/Document/ViewBox/Attrs/Element`（具体类型：`PathElement/CircleElement/RectElement/EllipseElement/LineElement/PolygonElement/PolylineElement/GroupElement`） | SVG 文档解析与元素模型 | SVG 解析 | 🔌 |

### 6.6 render/filters（0 顶层）
`init()` 副作用：把 internal/filter 的 blur/colormatrix/shadow/grayscale/invert 注册进 `render.RegisterFilterOps`，使 `Context.Apply*` 可用。**精简**：滤镜 op 注册表（DC.ApplyBlur 等的开关）。**状态：🔗**（经 `render/gpu` 的 `_ import` 自动激活）。

### 6.7 render/raster（0 顶层）
`init()` 副作用注册 AdaptiveFiller（4x4/16x16 tile 光栅）。**精简**：CPU tile 光栅独立注册入口。功能已并入 `render/gpu`（gpu 包 init 同时注册 SDFAccelerator + AdaptiveFiller）；本包是「只要 CPU tile 光栅不要 GPU」的入口，**状态：🔌**（无人走）。

### 6.8 render/gpu（顶层 16）
| API | 功能 | 精简 | 状态 |
|-----|------|------|------|
| `SetDeviceProvider(provider)/ResetAccelerator/AbandonDevice/PurgeSurfaceResources` | 注入共享 GPU 设备、重建加速器、设备放弃、清理表面资源 | 设备生命周期 | 🔗 examples/ggcanvas 注入；**ui/embedder 生产未调 ⚠️** |
| `AdapterPolicy/ResolveAdapterPolicy/RequestAdapterWithPolicy/DeviceDescriptor*/DeviceDescriptorLowVRAM` | 适配器/设备策略选择与描述符 | 适配器策略 | 🔗 |
| `SurfaceLifecycle/NoteTextureOOM/TextureOOMCount/ResetTextureOOMCount/ResolveSurfaceLifecycle` | 表面生命周期（正常/清理/重建）与纹理 OOM 钩子 | 表面/OOM | 🔗 |
| init() 副作用：注册 SDF GPU 加速器 + AdaptiveFiller + filters + OOM/lifecycle 钩子 | 一次 import 激活全部 GPU 加速与滤镜 | 加速注册 | ✅ 全部真窗 blank import 接线 |

---

## 7. 状态总表（接线 × 未接线）

### 7.1 有生产接线（✅/🔗）
Present/帧/呈现链路（frame/present/present_target → ui/embedder）、Context 绘制族、Brush/三种渐变、文本绘制（DrawString 族经 ui/rendering）、图像（GPU QueueImageDraw，Bicubic 例外）、Mask 上传、Layer（GPU RT）、滤镜 op（经 render/gpu 副作用）、SDF/CoverageFiller/AdaptiveFiller、Pixmap、路径基础 API、损伤跟踪、共享编码器。

### 7.2 已实现但无生产消费者（🔌 未接线）
| 功能 | 证据 |
|------|------|
| `render/svg` 整包（SVG 图标渲染） | 全仓无 import |
| `render/surface` 整包（surface 抽象+注册表） | 全仓无 import；与 render.Context 的集成未落地 |
| `render/recording` 整包（录制回放） | 仅测试/示例；PDF/SVG 后端仓外 |
| `render/raster` 独立注册入口 | 功能并入 render/gpu |
| 路径布尔 `BooleanPath/PathOp*` | 仅测试（s3c_m3_residual_gate_test） |
| 裁剪运算 `ClipOpDifference/Replace`（ClipRectOp/ClipPathOp） | 仅测试（p12_clip_mask_gpu_test） |
| Path 高级造型 `Trim/WithCorners/Discrete` | 仅测试 |
| `SetDither/DrawImageQuad`（m4_extensions） | 仅测试 |
| `Pattern/ImagePattern` 旧图案接口（SetFillPattern 等） | 仅测试/桥接；生产走 Brush |
| `Painter/SolidPainter/FuncPainter/PainterFromPaint` | 仅测试 |
| 文本描边/路径 `StrokeString/StrokeStringAnchored/TextPath/DrawShapedGlyphs` | 仅测试 |
| `SelectPipeline`（管线模式选择） | 仅测试调用点 |
| ui/rendering 滤镜 facade（ApplyGrayscale 等 FF-*） | 无生产调用方（需 blank-import filters，由 gpu 包侧效应顶替） |
| `ui.SetMask` widget API（antd 文献提及） | 未实现（非 render 层） |

### 7.3 ⚠️ 半成品 / GPU 未生效（重点：真窗实测）
| 功能 | 现象 | 证据与建议动作 |
|------|------|------|
| **任意路径裁剪 `Clip()`/`ClipPreserve()`（= ui `PushClipPath`）** | GPU 真窗下裁剪不生效：裁剪矩形内填红色/渐变/任意路径，全部画穿整个格；CPU 软路径（离屏单测 TestPushClipPath_TriangleClipsFill）正常 | 静态接线存在（setGPUClipPath → rc.SetClipPath → internal/gpu/render_session 引用 depthClipPipeline.BuildClipResources；depth_clip.go 完整 stencil+depth 管线）；2026-08 真窗 4 组复测（全格实色矩形/任意三角形/渐变路径 fill）全部画穿，参照块坐标验证无误。**疑似渲染会话内 depth clip 命令未实际生效，待 wr-debug/wr-engine 定位**。修复方向三选一：接通管线 / GPU 下显式回退 CPU 软裁 / API 标注不支持。当前能力声明（clip_rrect.go：「Does not claim … ClipPath」）不承诺该能力，不阻塞任何 R/C 关窗。 |
| `render/gpu` 设备生命周期 API 未进 ui/embedder | SetDeviceProvider/AbandonDevice 仅示例注入 | 真窗换设备/恢复场景未覆盖，属预留 |

### 7.4 仅测试/桥接（🧪）
见 §7.2 列表（布尔/SDF 独立注册/图案/画刷版本兼容等），不重复。

### 7.5 重复/重叠 API 对照（2026-08-15 核对）

> 同一能力存在多个入口时在此登记。「重复」分三类：**D= 同包双入口实质重复**，**W= 分层包装/封装（功能重叠但保留合理）**，**C= 平行体系/独立副本（跨包，多为未接线）**。**新增 API 前先查本表，能复用就复用**。

| # | 能力 | 重复成员 | 类型 | 说明 / 建议 |
|---|------|---------|------|------------|
| 1 | 纯色上色 | `Context.SetRGB/SetRGBA/SetHexColor/SetColor` ↔ `Solid/SolidRGB/SolidRGBA/SolidHex` + `SetFillBrush/SetStrokeBrush` | D | 旧 fogleman/gg 风格（Set* 内部生成实色 Solid）与画刷体系并存。新代码走 Solid+Set*Brush；Set* 保留兼容。 |
| 2 | 渐变 | `CustomBrush.LinearGradient/RadialGradient`（函数式）↔ `LinearGradientBrush/RadialGradientBrush`（色标式） | D | 两种实现机制交付同一视觉（两端线性 / 单位圆径向）。径向还多 `SetFocus`、色标 ∞；CustomBrush 仅两 stop。生产已验证走 Brush 数据式。 |
| 3 | 裁剪 | `Context.ClipRect` ↔ `ClipRectOp(o=ClipOpIntersect)`；`Context.Clip` ↔ ui `PaintContext.PushClipPath`；`scene.ClipState/ClipStack` | D+W | ClipRectOp(Intersect) 是 ClipRect 的带运算泛化（默认相交=同义）；ui PushClipPath 是 Context.Clip 的封装（GPU 画穿见 §7.3）；scene 为保留模式第三套。 |
| 4 | 画图体系 | `Context`（即时）↔ `scene.Scene`（保留）↔ `recording.Recorder`（录制）↔ `surface`（目标抽象） | C | 四套都含 Fill/Stroke/Clip/Image 语义。**仅 Context 接入 ui 产线**；scene 只被 render 内部 GPU 后端消费；recording/surface 无消费者（§7.2）。最大的体系级重复，各自文档见 §3/§6。 |
| 5 | 形状轮廓 | `Path.Rectangle/Circle/Ellipse/Arc/RoundedRectangle` ↔ `PathBuilder.Rect/RoundRect/Circle/Ellipse/Polygon/Star` ↔ `Context.Draw*`（DrawRectangle/DrawCircle/…） | D | 前三者都能往一条路径上追加矩形/圆/椭圆，**Path 与 PathBuilder 是两套独立实现、无桥接**（如 PathBuilder.Rect 自己 MoveTo/LineTo/Close，不调 Path.Rectangle）；Context.Draw* 是追加到当前路径的第三处。 |
| 6 | 文本 | `Context.DrawString/MeasureString/DrawStringWrapped` ↔ `text.Draw/Measure/MeasureText` | W | Context 方法是 text 包的封装（ui 链路走 Context 层）；text 包直连 draw.Image 供底层/工具用。 |
| 7 | 呈现 | `PresentTarget.PresentWith/PresentWithAuto` ↔ `Context.PresentFrame(+/Damage/DamageRects)` ↔ `frame.PresentFrameAuto/PresentFrameFull/PlanFramePresent` | W | 三层入口：PresentTarget 高层（draw 闭包）→ Context 低层（view+present 回调）→ frame 策略（选路径）。PresentWithAuto 与 PresentFrameAuto 决策语义重叠。 |
| 8 | 滤镜 | `Context.ApplyBlur/ApplyColorMatrix/…/ApplyImageFilterGraph` ↔ `scene.FilterChain/FilterType` | W | 入口不同，底层共用 internal/filter 实现；scene 侧仅保留模式用。 |
| 9 | 像素缓冲 | `Pixmap` ↔ `ImageBuf` ↔ `surface.ImageSurface` | C | 三种像素缓冲可互转（Pixmap.ToImage/Image()、ImageBufFromImage）；ImageSurface 是 surface 抽象实现，无消费者（§7.2）。 |
| 10 | 编解码导出 | `Context.Image()/SavePNG()/EncodePNG()/EncodeJPEG()` ↔ `Pixmap.ToImage()/SavePNG()/EncodePNG()/EncodeJPEG()` | W | Context 方法即包装底层 pixmap 同名方法（Context 内部持 Pixmap）。 |
| 11 | 画笔/几何副本 | `recording.SolidBrush/Linear/Radial/Sweep/Pattern + Matrix/Rect` ↔ 主包 `Brush` 族/`Matrix`/`Rect` | C | recording 为 SkPicture 移植携带独立副本，无消费者（§7.2）；改动主包 Brush 不影响 recording。 |
| 12 | SDF 覆盖 | `sdf.SDFFilledCircleCoverage/…`（4 函数）↔ `SDFAccelerator.FillShape/StrokeShape` | W | 独立函数供像素级/测试；加速器实现包装同语义。 |

### 7.5.1 收敛执行记录（2026-08-15）

> 收敛依据 = 最准确且对齐 skia/flutter（判据：官方注释声明 / skia 参数集与边界对照 / 生产消费者 / 活跃度）× 淘汰方扩展性（有明确扩展目标的保留，如 recording→PDF/SVG、surface→第三方后端 RFC#46、ColorFunc→程序化纹理）。

| # | 动作 | 执行内容 | 验证 |
|---|------|---------|------|
| 1 | 统一语义（已收敛） | `SetRGB/SetRGBA/SetHexColor/SetColor` 注释标注为 `Solid*`+`SetFillBrush` 的兼容别名（fogleman/gg 兼容层），实现本就等价 | go build/vet ✓ |
| 2 | 标废弃（已收敛） | `CustomBrush.LinearGradient/RadialGradient/HorizontalGradient/VerticalGradient` 加 `Deprecated:` 指向数据式画刷（缺 focus/extend/多色标，零生产消费者）；`ColorFunc/NewCustomBrush/Checkerboard/Stripes` 保留（程序化纹理扩展点） | go build ✓ + TestCustomBrush ✓ |
| 3 | 委托统一（已收敛） | `ClipRect` 委托 `ClipRectOp(…, ClipOpIntersect)`（SkClipOp 默认语义；单一实现，clipRect 为便捷别名） | TestS3a_M1_ClipRect / TestPaintContext_PushClipRect ✓ |
| 5 | 委托统一（已收敛，RoundRect 例外） | `PathBuilder.Rect/Circle/Ellipse` 委托 `Path.Rectangle/Circle/Ellipse`（同一实现）；**RoundRect 保留自身 skia 标准系数 k=0.5522847498**（≠ Path.RoundedRectangle 的精确圆弧公式 alpha≈0.5486，不委托以免改角几何） | TestPath + PathBuilder 相关 ✓ |
| 4/6/7/8/9/10/11/12 | 不收敛 | 分层依赖（文本/呈现/滤镜/编解码/SDF）或有明确扩展目标（scene 内部在用；recording→PDF/SVG；surface→第三方后端） | — |

> 未动：`Clip/ClipPreserve`（任意路径，GPU 半成品见 §7.3，修好前不启用）、`ClipPathOp`（clipPath 泛化）、`Path.RoundedRectangle`（Arc 精确公式实现，保留为 Path 方法组一部分）。
---

## 8. 主包公开类型的导出方法速查（方法级清单，2026-08-15 go/ast）

> Context 的 181 个方法见 §3 全列；下表补齐其余公开类型的导出方法，与 `go doc ./render` 的 `<类型>.<方法>` 一致。状态列同 §0 约定。改动任一方法时同步本表与 §3。

| 类型 | 导出方法 | 功能（做什么） | 精简 | 状态 |
|------|---------|--------------|------|------|
| `Path` | MoveTo/LineTo/QuadTo/CubicTo · Arc·Circle·Ellipse·Rectangle·RoundedRectangle · Append · Clone · Transform · Clear/Reset · Close · ComputeMetrics · TotalLength · PositionAt/TangentAt · Length · Area · BoundingBox/Bounds · Contains · Winding · Flatten/FlattenCallback · Reversed · Trim · WithCorners · Discrete · Iterate · Verbs/Coords · NumVerbs · HasCurves/HasCurrentPoint · CurrentPoint · Op | 路径构建、附加形状、克隆/变换、度量、查询与高级造型 | 即时路径全部操作 | ✅（Trim/WithCorners/Discrete/Op 🔌） |
| `PathBuilder` | MoveTo/LineTo/QuadTo/CubicTo · Rect/RoundRect/Circle/Ellipse/Polygon/Star · Close · Build · Path | 链式声明形状后生成 Path（Rect/Circle/Ellipse 已委托 Path 实现，§7.5.1） | 链式构建器 | ✅ |
| `Paint` | SetColor/SolidColor · SetBrush/GetBrush · ColorAt · SetStroke/GetStroke · SetLineWidth/EffectiveLineWidth · IsSolid/IsDashed · EffectiveDash · EffectiveLineCap/EffectiveLineJoin/EffectiveMiterLimit · Clone | 聚合画刷+描边状态并求生效值 | 绘制状态 | 🔗 |
| `Mask` | At/Set/Fill/Clear/Invert/Clone · Data/Bounds/Width/Height | 蒙版像素读写/反转/裁剪 | alpha 蒙版 | 🔗 |
| `Pixmap` | Width/Height · Data/Stride · At/Set/GetPixel/SetPixel/SetPixelPremul · FillRect · FillSpan/FillSpanBlend · Clear · ColorModel/Image/ImageView/ToImage · EncodePNG/EncodeJPEG/SavePNG · GenerationID/NotifyPixelsChanged · Bounds | CPU 像素缓冲：读写像素、行填充、编解码、世代号 | CPU 像素缓冲 | ✅ |
| `SoftwareRenderer` | Fill/Stroke · Flush/Render/Resize/Clear · Capabilities · SetAntiAlias/SetDeviceScale | 软渲染器生命周期与绘制 | CPU 软渲染 | 🔗 |
| `Renderer`（接口） | Fill/Stroke | 渲染抽象入口 | 渲染器抽象 | 🔗 |
| `SDFAccelerator` | Name/Init/Close · CanAccelerate · SetForceSDF · FillShape/FillPath/StrokeShape/StrokePath · Flush | 形状 SDF 快速填/描与强制开关 | CPU SDF 加速 | 🧪（生产仅内部 cpuFallback） |
| `Scene`（scene 子包同名类型） | → 见 §6.2 | — | — | 🔗 |
| `ImagePattern` | ColorAt · SetAnchor/SetClamp/SetOpacity/SetScale/SetTransform · GPUPatternSource | 图像区域作图案及其锚点/颜色/缩放/GPU 源 | 图像图案 | 🧪 |
| `SolidPattern`（`NewSolidPattern(color)`） | ColorAt | 实色图案采样/构造 | 实色图案 | 🧪 |
| `CustomBrush`（`NewCustomBrush`） | ColorAt · WithName | 函数式画刷求色/命名 | 函数式画刷 | 🔗 |
| `SolidBrush` | ColorAt/Lerp/WithAlpha/Opaque/Transparent | 实色画刷求色/透明度链 | 实色画刷 | ✅ |
| `LinearGradientBrush` | AddColorStop · ColorAt · SetExtend | 线性渐变加色标/越界模式 | 线性渐变 | ✅ |
| `RadialGradientBrush` | AddColorStop · ColorAt · SetExtend/SetFocus | 径向渐变加色标/焦点 | 径向渐变 | ✅ |
| `SweepGradientBrush` | AddColorStop · ColorAt · SetExtend/SetEndAngle | 圆锥渐变加色标/终止角 | 圆锥渐变 | ✅ |
| `Stroke` | WithWidth/WithCap/WithJoin/WithMiterLimit · WithDash/WithDashPattern/WithDashOffset · IsDashed · Clone | 描边参数链式设置 | 描边参数链 | ✅ |
| `Dash` | Clone · IsDashed · NormalizedOffset · PatternLength · Scale · WithOffset | 虚线序列运算（归一化/缩放/偏移） | 虚线序列 | 🔗 |
| `Matrix` | Multiply · TransformPoint/TransformVector · Invert · IsIdentity/IsTranslation/IsTranslationOnly/IsScaleOnly · ScaleFactor/MaxScaleFactor | 仿射矩阵运算与性质查询 | 仿射矩阵 | ✅ |
| `Point` | Add/Sub/Mul/Div · Dot/Cross · Length/LengthSquared · Normalize · Lerp · Distance · Rotate · Approx | 二维点算术/几何 | 点运算 | 🔗 |
| `Vec2` | Add/Sub/Mul/Div/Neg · Dot/Cross · Length/LengthSq · Normalize · Perp/Rotate · Lerp · Angle/Atan2/Approx · IsZero · ToPoint | 向量算术/几何 | 向量运算 | 🔗 |
| `RGBA` | RGBA · Premultiply/Unpremultiply · Lerp · Color | 颜色通道/预乘/插值 | 颜色值 | ✅ |
| `Line` | End/Start/Midpoint · Length · BoundingBox · Reversed · Subdivide/Subsegment · Eval | 线段几何求值/分割 | 线段几何 | 🔗 |
| `QuadBez` | Start/End · Eval/Extrema/Raise · BoundingBox · Subdivide/Subsegment | 二次贝塞尔求值/升阶/分割 | 二次贝塞尔 | 🔗 |
| `CubicBez` | Start/End · Eval/Extrema/Inflections/Deriv · Normal/Tangent · BoundingBox · Subdivide/Subsegment | 三次贝塞尔求值/拐点/切线 | 三次贝塞尔 | 🔗 |
| `Rect` | Width/Height · Contains · Union · (NewRect) | 矩形尺寸/包含/合并 | 矩形 | ✅ |
| `PathMetric` | IsEmpty · Length · PositionAt/TangentAt | 路径度量查询（取点/切线） | 路径度量 | 🔗 |
| `PresentTarget` | Context/Resize/Scale · PresentWith/PresentWithAuto/PresentClear · LastPresentOutcome/LastDamageAreaPx · InFullRecovery/SetResizeStormWindow · LogicalSize · Close | 呈现目标绘制/呈现/恢复状态 | 呈现目标 | ✅ |
| `FuncPainter`/`SolidPainter` | PaintSpan | 逐像素填色段 | 像素填色器 | 🧪 |
| 枚举类型通用方法 `String()` | PathVerb / PipelineMode / PresentMode / RasterizerMode / TextMode 均实现 String() 输出枚举名（日志/调试用） | 枚举打印 | 枚举调试 | 🔗 |
| `GPUAccelerator` 各接口方法 | 见 §4 接口群 | 加速能力探测 | 能力探测 | 🔗 |

> 内部/非导出接收者（LayeredPixmapTarget/PixmapTarget/Target/brushPattern/opacityMulBrush/nopHandler 等）不在公开目录内，改动不影响本表。

---

## 9. 主包常量与变量成员总表（2026-08-15 go/ast 提取，91 常量 + 11 变量）

| 类型族 | 成员 |
|--------|------|
| `AcceleratedOp` | AccelFill · AccelStroke · AccelScene · AccelText · AccelImage · AccelGradient · AccelCircleSDF · AccelRRectSDF |
| `Align` | AlignLeft · AlignCenter · AlignRight |
| `BlendMode` | BlendNormal · BlendClear · BlendCopy · BlendPlus · BlendModulate · BlendDestinationOut · BlendSourceAtop · BlendXor · BlendDestinationOver · BlendSourceOver · BlendDestinationIn · BlendSourceIn · BlendDestinationAtop · BlendSourceOut · BlendColorBurn · BlendColorDodge · BlendColor · BlendDarken · BlendDifference · BlendExclusion · BlendHardLight · BlendHue · BlendLighten · BlendLuminosity · BlendMultiply · BlendOverlay · BlendSaturation · BlendScreen · BlendSoftLight |
| `ClipOp` | ClipOpIntersect · ClipOpDifference · ClipOpReplace |
| `ExtendMode` | ExtendPad · ExtendRepeat · ExtendReflect |
| `FillRule` | FillRuleNonZero · FillRuleEvenOdd |
| `ImageFormat` | FormatGray8 · FormatGray16 · FormatBGRA8 · FormatBGRAPremul · FormatRGB8 · FormatRGBA8 · FormatRGBAPremul |
| `ImageFilterKind` | ImageFilterBlur · ImageFilterBlurXY · ImageFilterColorMatrix · ImageFilterDropShadow · ImageFilterGrayscale · ImageFilterInvert |
| `InterpolationMode` | InterpNearest · InterpBilinear · InterpBicubic |
| `LCDLayout` | LCDLayoutNone · LCDLayoutRGB · LCDLayoutBGR |
| `LineCap` | LineCapButt · LineCapRound · LineCapSquare |
| `LineJoin` | LineJoinMiter · LineJoinRound · LineJoinBevel |
| `PathBooleanOp` | PathOpUnion · PathOpIntersect · PathOpDifference · PathOpXor |
| `PathVerb` | MoveTo · LineTo · QuadTo · CubicTo · Close |
| `PipelineMode` | PipelineModeAuto · PipelineModeRenderPass · PipelineModeCompute |
| `PresentMode` | PresentModeIdle · PresentModeFull · PresentModeDamageUnion · PresentModeDamageMulti |
| `PresentPlatform` | PresentPlatformX11 · PresentPlatformWayland · PresentPlatformWin32 · PresentPlatformAppKit |
| `RasterizerMode` | RasterizerAuto · RasterizerAnalytic · RasterizerSparseStrips · RasterizerTileCompute · RasterizerSDF |
| `ShapeKind` | ShapeUnknown · ShapeCircle · ShapeEllipse · ShapeRect · ShapeRRect · ShapeArc |
| `TextDecoration` | TextDecorationNone · TextDecorationUnderline · TextDecorationOverline · TextDecorationLineThrough |
| `TextMode` | TextModeAuto · TextModeMSDF · TextModeVector · TextModeBitmap · TextModeGlyphMask · TextModeAliased |
| `TextureUsage` | TextureUsageCopySrc · TextureUsageCopyDst · TextureUsageTextureBinding · TextureUsageStorageBinding · TextureUsageRenderAttachment |
| `VertexMode` | VertexModeTriangles · VertexModeTriangleFan |
| `MSAASampleCount` | MSAASampleCount1(=1) · MSAASampleCount4(=4) |
| 阈值/杂项 | MaxTrackedDamageRects · DamageFullCoverageThreshold · DamageMultiWasteRatio |
| var（预置色） | Black · White · Red · Green · Blue · Yellow · Cyan · Magenta · Transparent |
| var（错误） | ErrFallbackToCPU · ErrNilSurfaceView |

> 本表由 go/ast 对 `render/` 主包（package render，非测试文件）顶层声明提取；增删常量时同步本表。

---

## 10. 维护规则（硬）

1. **增/改/删任何 render 公开符号**（顶层 func/type/const/var、Context 或其他类型的导出方法、子包顶层符号）→ 修改本文件对应分类表：新增补行、改名改行、删除删行，并更新 §0 统计（规模数字）与 §7 状态表。
2. **接线状态变化**（某 API 从"仅测试"变"有生产调用"，或反之；GPU 生效/失效）→ 立即更新 §7 对应条目，写清证据（调用点文件/真窗名/实测日期）。
3. **每次主体文档（ENGINE_UI_WIDGET_RENDER.md）改动涉及 render 公开 API 或接线** → 在本文件 §7/相应分类同步一行，并在主体文档 §10 修订表追加记录（版本列写 `API目录同步`）。
4. **新增 API 前先查 §7.5 重复对照**：若与既有 API 实质重复（同包双入口/封装/跨包副本），能复用就复用；确需新增则在 §7.5 补行并说明差异。禁止无谓引入平行入口。
5. 改动完成后跑 §0 核对命令（及 `go run ./scripts/apidoc`），确认清单与 `go doc` 一致后再合入。
6. 状态标注必须可追溯到证据：生产消费者 → 文件:行号；GPU 实测 → 真窗名/日期/现象；禁止凭印象标注。---

## 11. 附录：子包顶层符号完整清单（机器生成，go doc 2026-08-15）

> 本附录由 `go doc ./render/{scene,recording,surface,svg}` 顶层声明提取，与正文 §6 族级归纳互补；**改动子包公开 API 时同步本附录（跑 §0 核对命令后替换对应行）**。`render/text` 包级导出 226 项改以 `go doc ./render/text` 为准（正文 §6.1 族级归纳）。

```text
scene      const DefaultMaxSizeMB = 64 ...
scene      var DefaultPool = NewEncodingPool()
scene      func CanUseGPU() bool
scene      func PutEncoding(enc *Encoding)
scene      func TextAdvance(glyphs []*RenderedGlyph) float32
scene      type Affine struct{ ... }
scene      func AffineFromMatrix(m render.Matrix) Affine
scene      func IdentityAffine() Affine
scene      func NewAffine(a, b, c, d, e, f float32) Affine
scene      func RotateAffine(angle float32) Affine
scene      func ScaleAffine(x, y float32) Affine
scene      func TranslateAffine(x, y float32) Affine
scene      type ArcShape struct{ ... }
scene      func NewArcShape(cx, cy, rx, ry, startAngle, endAngle float32, sweepClockwise bool) *ArcShape
scene      type BlendMode uint32
scene      func AdvancedModes() []BlendMode
scene      func AllBlendModes() []BlendMode
scene      func BlendModeFromInternal(internal blend.BlendMode) BlendMode
scene      func HSLModes() []BlendMode
scene      func PaintBlendModeToScene(mode render.BlendMode) BlendMode
scene      func PorterDuffModes() []BlendMode
scene      type Brush struct{ ... }
scene      func SolidBrush(c render.RGBA) Brush
scene      type BrushKind uint32
scene      type CacheEntry struct{ ... }
scene      type CacheStats struct{ ... }
scene      type CircleShape struct{ ... }
scene      func NewCircleShape(cx, cy, r float32) *CircleShape
scene      type ClipStack struct{ ... }
scene      func NewClipStack() *ClipStack
scene      type ClipState struct{ ... }
scene      func NewClipState(shape Shape, transform Affine) *ClipState
scene      type CompositeShape struct{ ... }
scene      func NewCompositeShape(shapes ...Shape) *CompositeShape
scene      type DamageTracker struct{ ... }
scene      func NewDamageTracker() *DamageTracker
scene      type Decoder struct{ ... }
scene      func NewDecoder(enc *Encoding) *Decoder
scene      type EllipseShape struct{ ... }
scene      func NewEllipseShape(cx, cy, rx, ry float32) *EllipseShape
scene      type Encoding struct{ ... }
scene      func GetEncoding() *Encoding
scene      func NewEncoding() *Encoding
scene      type EncodingPool struct{ ... }
scene      func NewEncodingPool() *EncodingPool
scene      type FillStyle uint32
scene      type Filter interface{ ... }
scene      type FilterChain struct{ ... }
scene      func NewFilterChain(filters ...Filter) *FilterChain
scene      type FilterType uint8
scene      type GPUSceneRenderer struct{ ... }
scene      func NewGPUSceneRenderer(dc *render.Context) *GPUSceneRenderer
scene      type GlyphEntry struct{ ... }
scene      type GlyphRunData struct{ ... }
scene      type Image struct{ ... }
scene      func NewImage(width, height int) *Image
scene      type Iterator struct{ ... }
scene      type LayerCache struct{ ... }
scene      func DefaultLayerCache() *LayerCache
scene      func NewLayerCache(maxSizeMB int) *LayerCache
scene      type LayerKind uint8
scene      type LayerStack struct{ ... }
scene      func NewLayerStack() *LayerStack
scene      type LayerState struct{ ... }
scene      func NewClipLayer(clip Shape) *LayerState
scene      func NewFilteredLayer(blend BlendMode, alpha float32) *LayerState
scene      func NewLayerState(kind LayerKind, blend BlendMode, alpha float32) *LayerState
scene      type LineCap uint32
scene      type LineJoin uint32
scene      type LineShape struct{ ... }
scene      func NewLineShape(x1, y1, x2, y2 float32) *LineShape
scene      type Path struct{ ... }
scene      func NewPath() *Path
scene      type PathBuilder interface{ ... }
scene      type PathElement struct{ ... }
scene      type PathPool struct{ ... }
scene      func NewPathPool() *PathPool
scene      type PathShape struct{ ... }
scene      func NewGGPathShape(ggPath *render.Path) *PathShape
scene      func NewPathShape(path *Path) *PathShape
scene      type PathVerb uint8
scene      type PieShape struct{ ... }
scene      func NewPieShape(cx, cy, r, startAngle, endAngle float32, sweepClockwise bool) *PieShape
scene      type Point struct{ ... }
scene      type PolygonShape struct{ ... }
scene      func NewPolygonShape(points ...float32) *PolygonShape
scene      type Rect struct{ ... }
scene      func EmptyRect() Rect
scene      func TextBounds(glyphs []*RenderedGlyph) Rect
scene      type RectShape struct{ ... }
scene      func NewRectShape(x, y, width, height float32) *RectShape
scene      type RegularPolygonShape struct{ ... }
scene      func NewRegularPolygonShape(cx, cy, r float32, sides int, rotation float32) *RegularPolygonShape
scene      type RenderStats struct{ ... }
scene      type RenderedGlyph struct{ ... }
scene      type Renderer struct{ ... }
scene      func NewRenderer(width, height int, opts ...RendererOption) *Renderer
scene      type RendererOption func(*Renderer)
scene      func WithCache(cache *LayerCache) RendererOption
scene      func WithCacheSize(mb int) RendererOption
scene      func WithTileSize(size int) RendererOption
scene      func WithWorkers(n int) RendererOption
scene      type RoundRectShape struct{ ... }
scene      func NewRoundRectShape(rect Rect, rx, ry float32) *RoundRectShape
scene      func NewRoundRectShapeUniform(rect Rect, r float32) *RoundRectShape
scene      type RoundedRectShape struct{ ... }
scene      func NewRoundedRectShape(x, y, width, height, radius float32) *RoundedRectShape
scene      type Scene struct{ ... }
scene      func NewScene() *Scene
scene      type SceneBuilder struct{ ... }
scene      func NewSceneBuilder() *SceneBuilder
scene      func NewSceneBuilderFrom(scene *Scene) *SceneBuilder
scene      type ScenePool struct{ ... }
scene      func NewScenePool() *ScenePool
scene      type Shape interface{ ... }
scene      type StarShape struct{ ... }
scene      func NewStarShape(cx, cy, outerRadius, innerRadius float32, points int, rotation float32) *StarShape
scene      type StrokeStyle struct{ ... }
scene      func DefaultStrokeStyle() *StrokeStyle
scene      type Tag byte
scene      type TaggedBounds struct{ ... }
scene      type TextFlags uint16
scene      type TextRenderer struct{ ... }
scene      func NewTextRenderer() *TextRenderer
scene      func NewTextRendererWithConfig(config TextRendererConfig) *TextRenderer
scene      type TextRendererConfig struct{ ... }
scene      func DefaultTextRendererConfig() TextRendererConfig
scene      type TextRendererPool struct{ ... }
scene      func NewTextRendererPool() *TextRendererPool
scene      type TextShape struct{ ... }
scene      func NewTextShape(str string, face text.Face, x, y float32) (*TextShape, error)
scene      type TransformShape struct{ ... }
scene      func NewTransformShape(shape Shape, transform Affine) *TransformShape
recording  const InvalidRef = ^uint32(0)
recording  func Backends() []string
recording  func Count() int
recording  func IsRegistered(name string) bool
recording  func Register(name string, factory BackendFactory)
recording  func Unregister(name string)
recording  type Backend interface{ ... }
recording  func MustBackend(name string) Backend
recording  func NewBackend(name string) (Backend, error)
recording  type BackendFactory func() Backend
recording  type Brush interface{ ... }
recording  func BrushFromGG(b render.Brush) Brush
recording  type BrushRef uint32
recording  type ClearClipCommand struct{}
recording  type ClipRoundRectCommand struct{ ... }
recording  type Command interface{ ... }
recording  type CommandType uint8
recording  type DrawImageCommand struct{ ... }
recording  type DrawTextCommand struct{ ... }
recording  type ExtendMode int
recording  type FileBackend interface{ ... }
recording  type FillPathCommand struct{ ... }
recording  type FillRectCommand struct{ ... }
recording  type FillRule uint8
recording  type FontRef uint32
recording  type GradientStop struct{ ... }
recording  type ImageOptions struct{ ... }
recording  func DefaultImageOptions() ImageOptions
recording  type ImageRef uint32
recording  type InterpolationMode uint8
recording  type LineCap uint8
recording  type LineJoin uint8
recording  type LinearGradientBrush struct{ ... }
recording  func NewLinearGradientBrush(x0, y0, x1, y1 float64) *LinearGradientBrush
recording  type Matrix struct{ ... }
recording  func Identity() Matrix
recording  func Rotate(angle float64) Matrix
recording  func Scale(sx, sy float64) Matrix
recording  func Shear(x, y float64) Matrix
recording  func Translate(x, y float64) Matrix
recording  type PathRef uint32
recording  type PatternBrush struct{ ... }
recording  func NewPatternBrush(imageRef ImageRef) *PatternBrush
recording  type PixmapBackend interface{ ... }
recording  type RadialGradientBrush struct{ ... }
recording  func NewRadialGradientBrush(cx, cy, startRadius, endRadius float64) *RadialGradientBrush
recording  type Recorder struct{ ... }
recording  func NewRecorder(width, height int) *Recorder
recording  type Recording struct{ ... }
recording  type Rect struct{ ... }
recording  func NewRect(x, y, width, height float64) Rect
recording  func NewRectFromPoints(x1, y1, x2, y2 float64) Rect
recording  type RepeatMode int
recording  type ResourcePool struct{ ... }
recording  func NewResourcePool() *ResourcePool
recording  type RestoreCommand struct{}
recording  type SaveCommand struct{}
recording  type SetAntiAliasCommand struct{ ... }
recording  type SetClipCommand struct{ ... }
recording  type SetDashCommand struct{ ... }
recording  type SetFillRuleCommand struct{ ... }
recording  type SetFillStyleCommand struct{ ... }
recording  type SetLineCapCommand struct{ ... }
recording  type SetLineJoinCommand struct{ ... }
recording  type SetLineWidthCommand struct{ ... }
recording  type SetMiterLimitCommand struct{ ... }
recording  type SetStrokeStyleCommand struct{ ... }
recording  type SetTransformCommand struct{ ... }
recording  type SolidBrush struct{ ... }
recording  func NewSolidBrush(color render.RGBA) SolidBrush
recording  type Stroke struct{ ... }
recording  func DefaultStroke() Stroke
recording  type StrokePathCommand struct{ ... }
recording  type StrokeRectCommand struct{ ... }
recording  type StrokeTextCommand struct{ ... }
recording  type SweepGradientBrush struct{ ... }
recording  func NewSweepGradientBrush(cx, cy, startAngle float64) *SweepGradientBrush
recording  type WriterBackend interface{ ... }
surface    var ErrNoBackendAvailable = errors.New("surface: no backend available")
surface    func Available() []string
surface    func List() []string
surface    func Register(name string, priority int, factory SurfaceFactory, available func() bool)
surface    func Unregister(name string)
surface    type BackendNotFoundError struct{ ... }
surface    type BackendUnavailableError struct{ ... }
surface    type BlendMode uint8
surface    type BlendableSurface interface{ ... }
surface    type Capabilities struct{ ... }
surface    type CapableSurface interface{ ... }
surface    type ClippableSurface interface{ ... }
surface    type DrawImageOptions struct{ ... }
surface    func DefaultDrawImageOptions() *DrawImageOptions
surface    type FillRule uint8
surface    type FillStyle struct{ ... }
surface    func DefaultFillStyle() FillStyle
surface    type Filter uint8
surface    type GPUBackend interface{ ... }
surface    type GPUSurface struct{ ... }
surface    func NewGPUSurface(width, height int, backend GPUBackend) (*GPUSurface, error)
surface    type ImageSurface struct{ ... }
surface    func NewImageSurface(width, height int) *ImageSurface
surface    func NewImageSurfaceFromImage(img *image.RGBA) *ImageSurface
surface    type LineCap uint8
surface    type LineJoin uint8
surface    type Options struct{ ... }
surface    func DefaultOptions(width, height int) Options
surface    type Path struct{ ... }
surface    func NewPath() *Path
surface    type Pattern interface{ ... }
surface    type Point struct{ ... }
surface    func Pt(x, y float64) Point
surface    type Registry struct{ ... }
surface    func NewRegistry() *Registry
surface    type RegistryEntry struct{ ... }
surface    func Get(name string) (*RegistryEntry, bool)
surface    type ResizableSurface interface{ ... }
surface    type SolidPattern struct{ ... }
surface    type StrokeStyle struct{ ... }
surface    func DefaultStrokeStyle() StrokeStyle
surface    type SubSurface interface{ ... }
surface    type Surface interface{ ... }
surface    func NewSurface(width, height int) (Surface, error)
surface    func NewSurfaceByName(name string, width, height int) (Surface, error)
surface    func NewSurfaceByNameWithOptions(name string, opts Options) (Surface, error)
surface    func NewSurfaceWithOptions(opts Options) (Surface, error)
surface    type SurfaceFactory func(opts Options) (Surface, error)
svg        func Render(data []byte, width, height int) (*image.RGBA, error)
svg        func RenderWithColor(data []byte, width, height int, c color.Color) (*image.RGBA, error)
svg        type Attrs struct{ ... }
svg        type CircleElement struct{ ... }
svg        type Document struct{ ... }
svg        func Parse(data []byte) (*Document, error)
svg        type Element interface{ ... }
svg        type EllipseElement struct{ ... }
svg        type GroupElement struct{ ... }
svg        type LineElement struct{ ... }
svg        type PathElement struct{ ... }
svg        type PolygonElement struct{ ... }
svg        type PolylineElement struct{ ... }
svg        type RectElement struct{ ... }
svg        type ViewBox struct{ ... }
```

### 附录 B：子包枚举/常量成员补充清单（go/ast 2026-08-15）

> 附录 A 只含顶层声明；此清单补枚举成员（各类型常量）。正文 §6 族表未逐个列出时以本清单为准；改动子包常量时同步本清单。

```text
[scene] BlendDestination BlendClear BlendColor BlendColorBurn BlendColorDodge BlendCopy BlendDarken BlendDestinationAtop BlendDestinationIn BlendDestinationOut BlendDestinationOver BlendDifference BlendExclusion BlendHardLight BlendHue BlendLighten BlendLuminosity BlendModulate BlendMultiply BlendNormal BlendOverlay BlendPlus BlendSaturation BlendScreen BlendSoftLight BlendSourceAtop BlendSourceIn BlendSourceOut BlendSourceOver BlendXor
[scene] BrushImage BrushLinearGradient BrushRadialGradient BrushSolid
[scene] FillEvenOdd FillNonZero FilterBlur FilterColorMatrix FilterDropShadow FilterNone
[scene] LayerClip LayerFiltered LayerRegular
[scene] TagBeginClip TagBeginPath TagBrush TagClosePath TagCubicTo TagEndClip TagEndPath TagFill TagFillRoundRect TagImage TagLineTo TagMoveTo TagPopLayer TagPushLayer TagQuadTo TagSetAntiAlias TagStroke TagText TagTransform
[scene] TextFlagCJK TextFlagHinting
[recording] CmdClearClip CmdClipRoundRect CmdDrawImage CmdDrawText CmdFillPath CmdFillRect CmdRestore CmdSave CmdSetAntiAlias CmdSetClip CmdSetDash CmdSetFillRule CmdSetFillStyle CmdSetLineCap CmdSetLineJoin CmdSetLineWidth CmdSetMiterLimit CmdSetStrokeStyle CmdSetTransform CmdStrokePath CmdStrokeRect CmdStrokeText
[recording] InterpolationBilinear InterpolationNearest RepeatBoth RepeatNone RepeatX RepeatY
[surface] BlendModeClear BlendModeCopy BlendModeMultiply BlendModeOverlay BlendModeScreen BlendModeSourceOver FilterBilinear FilterNearest
```
