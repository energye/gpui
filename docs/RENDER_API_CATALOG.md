# render 公开 API 目录（RENDER_API_CATALOG）

> **一句话**：这是 `render/`（含 `render/text`、`render/scene`、`render/recording`、`render/svg`、`render/filters`、`render/gpu`）全部公开 API 的总账，并标注「已接线 / 未接线 / 半成品 / 仅测试」状态。
>
> **维护义务（硬）**：**每次增加、修改、删除 render 公开 API（功能或签名），或改变其接线状态，必须同步更新本文件**（新增/变更的行写进对应分类表 + 更新 §7 状态表），并在主体文档 `docs/ENGINE_UI_WIDGET_RENDER.md` §10 修订表追加一行。判定「公开 API」的口径：`go doc ./render`（以及各子包 `go doc ./render/xxx`）可见的顶层符号与导出方法；方法级变更需同步 §3 与 §8 方法速查表。
>
> 核对命令（每次改动后跑）：
> ```bash
> cd <repo>/render
> grep -hE "^(func|type|var|const) [A-Z]" $(ls *.go | grep -v _test.go) > /tmp/api_check.txt   # 主包顶层
> go doc ./render ./render/text ./render/scene ./render/recording ./render/svg ./render/gpu 2>/dev/null | grep -E "^(func|type|var|const) [A-Z]"  # 包级权威清单
> ```
>
> 状态词约定：✅ 有生产接线且 GPU/CPU 实测有效 · 🔗 有生产接线（真实链路在用）· ⚠️ 半成品/有实现但 GPU 未生效（详见 §7.3）· 🔌 无生产消费者（未接线，仅测试/示例/直调真窗/demo）· 🧪 仅测试覆盖。
>
> **生产与原子验证口径（硬，见 AGENTS.md）**：生产只认两类调用——组件库真正调用，或 2.5D 引擎与模块功能包真正调用（含 `ui/rendering` 这类替模块干活的胶水包）。真窗示例与单元测试只算原子验证（直接调接口、一次只验一个能力），不算生产接线，拿它们当接线证据的行一律降级。通用能力验证过但两类生产都没调，标已验证待接线（正常中间态，禁止当生产标绿）。

---

## 0. 包总览

| 包 | 顶层导出规模（逐包标注实测日期，未标日期的仍为 2026-08-15 快照） | 接线状态 | 说明 |
|----|------|------|------|
| `render`（主包） | 类型 124 · 顶层函数 155 · Context 导出方法 199 · PresentTarget 导出方法 18 · 常量 137 · 变量 24（2026-10-26 可录制掩码后 go/ast 重测；HEAD 基线 123/155/199/18/137/24，本次 +1 类型 `MaskCanvas`，旧记“154 函数”为文档误差（HEAD 实测已是 155）；2026-10-24 S2b C4 后 go/ast 逐项重测；2026-10-23 S2b C2b 快照为 120/150/199/18/137/24；2026-10-22 为 119/148/200/18/137/24；旧快照 2026-08-15 为 104/131/200/133/16） | 🔗 生产主链路 | 即时模式 DC，embedder/真窗全部走这里（2026-09-05 1.2：适配器策略自 render/gpu 搬入主包，+1 类型 +5 函数 +7 常量；1.3：+OOM 判定与 purge 链 1 类型 +4 函数；2026-09-15 R1：+2 类型 QuadKind/QuadDrawOptions +2 函数 ClassifyQuad/DrawImageQuadEx +1 方法 DrawImageQuadEx +4 常量 QuadOK 系 +5 变量 ErrQuad 系；2026-09-30 P4：+1 类型 Backend +2 函数 ResolveBackend/SelectBackend +2 常量 BackendNative/BackendGo，-2 临时函数 p1GLRequested/p1GLInstance；2026-09-30 P4-Config：+2 函数 ResolveBackendFor/NewPresentTargetWithBackend，老 NewPresentTarget 保留转调；2026-09-30 视频P0：+4 类型 VideoBackendCaps/VideoSlot/VideoPoolStats/VideoTexturePool +4 函数 QueryVideoBackend/BorrowVideoBackend/VideoFallbackTotal/NewVideoTexturePool +1 方法 RecordVideoFallback；2026-10-20 A15：+5 Context 方法 DrawShadow/DrawColor/DrawPaint/ClipBounds/DrawParagraph；2026-10-21 C 组：删除旧两参 Context.DrawImageQuad，Context.DrawImageQuadEx 更名为 Context.DrawImageQuad(img,corners,opts)，Context 导出方法按 go doc 实测 201→200；同日删除无消费者的 render/raster init 包，CPU 分块光栅仍由 render/gpu 注册；删除无消费者的 render/surface 整包（53 个公开 API，与主包渲染重复）；同日 render/svg 建直调真窗并修 fill="none" 继承/根百分比解析，公开签名不变；2026-10-22 S2a：+1 类型 `Canvas`（canvas.go，SkCanvas 同位物、89 方法，主包类型 118→119；`*Context` 侧编译期断言 `var _ Canvas = (*Context)(nil)`，Recorder 侧断言在 render/recording），Context 导出方法按 go doc 仍 200；PresentTarget +1 方法 `LastDamageRectSumPx`（present_frame.go，R4 损伤面积账本，方法 17→18）；2026-10-23 **S2b C2b（录制面补齐，把「能描述」的方法族一次接进 Canvas/Recorder）**：`Canvas` 89→**100** 方法（+11：`SetTransform`/`SetFillPattern`/`SetStrokePattern`/`SetStroke`/`ClipRectOp`/`ClipPathOp`/`DrawVerticesEx`/`DrawMesh`/`DrawMeshEx`/`SetTextMode`/`SetLCDLayout`），主包类型 119→120（+`ImagePatternSource`：图样的完整描述——哪张图、哪个子矩形、预存的采样逆矩阵、不透明度、clamp，图样因此能离开活 Context 旅行）、顶层函数 148→150（+`NewImagePattern`/+`NewImagePatternFromSource`），Context 导出方法 200→**199**（**删** `Context.CreateImagePattern`：由源矩形建图样不需要目标表面，录制面也必须能描述它，故改包级函数，调用方同笔全切）；同笔修 `Context.ClipPathOp` 设备空间二次变换缺陷——路径动词在 `MoveTo/LineTo` 时已被用户矩阵烘焙，折算设备空间只需再乘 deviceMatrix，原实现乘 `totalMatrix()` 把用户 CTM 又乘了一遍，裁剪被挪离它本要保住的几何（永久回归 `render/clip_ops_semantics_test.go` `TestClipOps_PathCtmInvariance`：3 个 ClipOp × 4 种 CTM 变体，逐像素着色数 oracle，mutation 反证已做））。**2026-10-24 S2b C4（纹理源注册族）**：+3 类型 `TextureSource`/`TexturePaintOptions`/`PaintStats`，+4 函数 `RegisterTextureSource`/`UnregisterTextureSource`/`GetTextureSource`/`LiveTextureSources`，`VideoBridge` +3 方法 `PaintTexture`/`PaintStats`/`ID`（类型 120→123、顶层函数 150→154，Context/PresentTarget 方法数不变）；纹理源按 id 解析、源自己负责跨线程同步，视频桥是第一个实现，`ui/scene` 合成与 `RenderVideo` 已接 |
| `render/text` | 包级导出 259（2026-10-27 字体源缓存新增 4 项：`PurgeFontSourceCache/FontSourceCacheSnapshot/FontSourceCacheStats/ResetFontSourceCacheStats`，255→259 沿用 10-26 口径；2026-10-26 实测 `go doc -all` 顶层口径；248→252 为 14/15px 字重模糊修复：新增 `FontRef/FontFamily/NewFontFamily/LoadFontFamily/EmboldenOutline/WeightClassProvider` 六项，删除 `EmboldenRadius/EmboldenResult` 两项；252→255：10-25 A1 新增 `ShapeForDraw/ShapeForDrawFallbacks`（整形优先 + MultiFace 拆段 + cmap 显式回退计数），10-26 新增 `GlyphsHaveColor`（逐字形读 `ColorFont.GlyphType` 判彩色字形，Skia「字体说算」口径；`ui/rendering.buildShapedCarets` 用它闸住 `isColorText`）；`MakeGlyphMaskKey/MakeGlyphMaskKeyAliased/MakeGlyphMaskKeyBucketed` 加 `hinting` 参数并新增 `GlyphMaskFlagHintingShift/Mask` 常量（分组常量不计顶层）；`RasterizeHintedBold/RasterizeAliasedBold/RasterizeLCDBold/FontSource.WeightClass` 为导出方法，不计顶层。含字体/整形/布局/光栅化；+1 `RuneAdvance`、+1 `SegmentReuse`、+3 字距：`ApplySpacing/ApplySpacingDir/SpacingExtra/WrapTextWithSpacing`（字距数学委托第三方 `shaping.AddSpacing`，`WhitespaceRuns` 已删）；`LayoutOptions` 新增 `LetterSpacing/WordSpacing` 字段；6b：`WithWeight/FaceEmbolden/DeriveFace` + `FontFamily` CSS 权重匹配 + `EmboldenOutline` 兜底；新增 `FontWeight` 类型 + `WeightThin..WeightBlack`/`GlyphMaskFlagBold` 常量；`Face.Embolden` 接口方法 + `MultiFace.WithWeight` 方法） | 🔗 生产主链路 | 字形子系统，`render` 主包文本 API 的底层 |
| `render/scene` | 顶层 143 | 🔗 render 内部（GPU 后端吃 Scene）+ 🔌 直调真窗 `ui_render_base_scene`（2026-10-21：GPU/CPU 双后端、LayerCache、DamageTracker，原子验证）；ui/组件/2.5D/视频生产调用仍为零 | 保留模式场景图（Scene/Encoding/Renderer） |
| `render/recording` | 顶层声明 98（2026-10-26 可录制掩码后实测：`go doc` 类型 83 · 函数 12 · const 2 · var 1；`go/ast` 符号名 180 = 类型 83 · 函数 33 · 常量 63 · 变量 1。历史：2026-10-24 S3 后 `go doc` 90（类型 77 · 函数 10 · const 2 · var 1），S3 新增 `DrawGPUTextureCommand` 与可选回放能力 `GPUTextureBackend` 两个类型；2026-10-23 S2b C2b 为 88（类型 75 · 函数 10 · const 2 · var 1），`go/ast` 按符号名展开当时为 164；2026-10-22 为 82 / 155（类型 70 · 函数 28 · 常量 56 · 变量 1），旧记 85 未写口径） | 🔌 直调真窗 `ui_render_base_recording`（2026-10-21）+ 录制面已达成 `render.Canvas` 89/89（2026-10-22 S2a，原子验证：`TestS3c_M3_ReplayParityOnGPU` 逐位实况一致） | SkPicture 式录制回放；`DrawTextCommand`/`StrokeTextCommand` 带 `Font FontRef` 字段；PDF/SVG 后端为仓外模块未接线；生产消费者仍为零（S2b 接线中）。**S2a 变更（2026-10-22）**：`Recorder` 导出方法 86→111（+27：AppendPath/SetPath/DrawPath/FillPath/StrokePath/ClipRect/Font/SetTextDecoration、PushLayer/PushLayerIsolated/PushBackdropLayer/PopLayer、ApplyBlur/ApplyColorMatrix/ApplyDropShadow/ApplyGrayscale、DrawImageEx/DrawImageNine/DrawImageQuad/DrawImageRounded/DrawImageCircular/DrawAtlas/DrawAtlasEx/DrawDepthSprites/DrawVertices/DrawShapedGlyphs/DrawShapedColorGlyphs；-2：DrawImageAnchored/DrawImageScaled，由 Canvas 的 `DrawImage`/`DrawImageEx` 承担），并加 `var _ render.Canvas = (*Recorder)(nil)` 断言；+19 命令类型（Concat/ClipRect/DrawImageNine/DrawImageQuad/DrawImageRounded/DrawImageCircular/DrawAtlas/DrawDepthSprites/DrawVertices/DrawGlyphs/SetTextDecoration/ApplyBlur/ApplyColorMatrix/ApplyDropShadow/ApplyGrayscale/PushLayer/PushLayerIsolated/PushBackdropLayer/PopLayer）+ 对应 19 个 `Cmd*` 常量；+`ContextBackend`（+`NewContextBackend`，31 方法 = `Backend` 契约的 `render.Context` 回放实现）+ `ErrNoContext` + `ApplyBrush`/`ApplyStroke`/`ToRenderBrush`/`TransformPoint`；**删** `SetTransformCommand`+`CmdSetTransform`（变换算子改由 `ConcatCommand` 原样携带、回放到 CTM 上）、`ImageOptions`+`DefaultImageOptions`+`InterpolationMode`（统一用 `render.DrawImageOptions`/`render.InterpolationMode`）；`ResourcePool.AddImage(image.Image)` 改 `AddImageBuf(*render.ImageBuf)`（方法数 14 不变）；`Matrix`/`FillRule`/`LineCap`/`LineJoin` 由自有类型改为 `render.X` 别名并再导出其成员常量；`Backend` 契约同步扩到 31 方法。**S2b C1 变更（2026-10-23，录制期出结论、光栅线程不再量）**：`FinishRecording` 加一趟命令流走查（`bounds.go`：模拟 CTM + Save/Restore 栈），`Recording` 导出方法 5→8——新增 `Bounds() image.Rectangle`（图层局部、与设备缩放无关的落点框，取代 `RasterExtra`/`ExtraBounds` 与光栅线程的 `measureTextBounds`）、`HasVisiblePaint() bool`（`ui/scene/picture.go` `hasVisibleOps` 判空语义的忠实移植：零 alpha 实色、无 alpha stop 的渐变、纯 move-to 路径、零宽高矩形、缺 face 的文字、已 Dispose 缓冲都不算可见；必须有它，因为 `Recorder.Fill` 只跳空路径不跳零 alpha，拿「命令数 > 0」当准入会给看不见的内容分配纹理并打穿透明保护）、`ImageBufs()`（池内缓冲按插入序只读交出）；`ResourcePool` +1 方法 `Images()`（14→15）；`Recorder` 导出方法 111→**108**（删 `SetFillRuleGG`/`SetLineCapGG`/`SetLineJoinGG` 三个与 `SetFillRule`/`SetLineCap`/`SetLineJoin` 字面重复的方法），`BrushFromGG`/`BrushToGG` 改名 `FromRenderBrush`/`ToRenderBrush`（§11 命名债清零）。**S2b C2b 变更（2026-10-23，把 Canvas 剩下 11 个方法族一次接进录制面）**：`Recorder` 导出方法 108→**118**（+10 = Canvas 新增 11 个方法里 `SetTransform` 与原有同名方法合并；`SetFillPattern`/`SetStrokePattern`/`SetStroke`/`ClipRectOp`/`ClipPathOp`/`DrawVerticesEx`/`DrawMesh`/`DrawMeshEx`/`SetTextMode`/`SetLCDLayout`）；+5 命令类型与同名 `Cmd*` 常量（`ClipRectOpCommand`/`ClipPathOpCommand`/`SetTextModeCommand`/`SetLCDLayoutCommand`/`DrawMeshCommand`——裁剪、文字渲染模式、LCD 次像素序、索引网格这些「绘制命令不携带」的状态必须真成一条命令，光靠样式快照会被 `Playback` 吞掉）；`Backend` 契约 31→**37** 方法、`ContextBackend` 同步 37；`PatternBrush` 重写为可描述的数据（`Image *render.ImageBuf` + `SrcX/SrcY/SrcW/SrcH` + `Inverse` + `Opacity` + `Clamp`，方法 `SetSourceRegion`/`SetInverse`/`SetOpacity`/`SetClamp`/`Source()`，存采样逆矩阵而非正变换——回放再求一次逆就不是逐位一致了），+`CustomPatternBrush`/`NewCustomPatternBrush`（承载 `ColorAt` 这类闭包图样：无描述可导出，PDF/SVG 后端也写不出，回放把同一实例交给目标 Context，契约与 `text.Face` 一致——录制即只读）；+`FromRenderPattern`/`ToRenderPattern` 主包互转；**删** `RepeatMode` 类型与 `RepeatNone/RepeatX/RepeatY/RepeatBoth` 四常量（图样重复语义由 `render.ImagePattern` 的源矩形 + clamp 承担，自有枚举是第二套真值）。原子验证：`render/recording/backends/raster/backend_c2b_parity_test.go` 七族实况 vs 回放逐位一致（`TestC2bImagePatternParity`/`TestC2bSetStrokeParity`/`TestC2bClipOpParity`/`TestC2bMeshParity`/`TestC2bTextStateParity`/`TestC2bAntiAliasParity`/`TestC2bSetTransformParity`），逐条做过 mutation 反证（把录制或回放任一侧改坏，对拍必红；如 M3：ClipPathOp 二次变换 → 730/24000 px 差）。**S2b C3 变更（2026-10-23，载体对换 + `scene.Picture` 退役）**：`ContextBackend`→**`CanvasBackend`**、`NewContextBackend`→**`NewCanvasBackend`**、`ErrNoContext`→**`ErrNoCanvas`**（构造口由 `*render.Context` 放宽到 `render.Canvas`，37 方法不变，改名不留兼容垫片）——目标为活 Context 时是生产回放主路，目标为 `*Recorder` 时命令折进外层显示列表，`Bounds`/`HasVisiblePaint`/`ImageBufs`/`PaintOpCount`/`NeedsCPUPassScratch` 同趟跟着长（`TestCanvasBackendPlaybackIntoRecorderFoldsFacts`/`...FoldsInvisibility` 两条逐位锁死，取代旧 `extra != nil` 的「有闭包就全信」猜测）；`Save`/`Restore` 同时保存并还原环境笔刷（`Context.Pop` 不回滚笔刷，不还原就会把颜料漏给调用方后续绘制），为此 `render.Canvas` 100→**102**（+`FillBrush`/`StrokeBrush`），`Recorder` 随之 118→**120**；`Recording` 导出方法 8→**10**（+`PaintOpCount()` 准入与统计用，+`NeedsCPUPassScratch()` 供光栅侧决定是否建 scratch，判据源自命令流而非外部提示）。2026-10-23 时顶层声明两口径为 `go doc` 88（类型 75 · 函数 10 · const 2 · var 1，改名不改数）/ `go/ast` 符号名 164；**2026-10-26 可录制掩码后为 `go doc` 98 / `go/ast` 180**，新增 5 条掩码命令 + `MaskBackend` + `MaskSkips`/`ResetMaskSkips` + 5 个 `Cmd*`，`Recorder` 121→126、`CanvasBackend` 39→44，详见 §6.3 |
| `render/svg` | 顶层 15 | 🔌 直调真窗 `ui_render_base_svg` | 图标风 SVG 渲染（Render/RenderWithColor/Parse）；生产消费者仍为零 |
| `render/filters` | 0（仅 init 副作用注册） | 🔗 经 render/gpu 副作用注册 | 让 DC.ApplyBlur 等可用；ui 侧 facade 无生产调用 |
| `render/gpu` | 顶层 12（8 函数 + 1 类型 + 3 常量，go doc 实数） | 🔗 真窗/门禁全量接线 | GPU 加速注册 + 设备生命周期（2026-09-05 1.2：适配器策略 AdapterPolicy 等 13 符号搬入 render 主包） |

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
| `Rect` + `NewRect(p1,p2)` | 矩形（两点对角）；含 Width/Height/Contains/Union | 矩形 | ✅ |
| `Line` + `NewLine(p0,p1)` | 线段几何（求值/分割/包围盒） | 线段 | 🔗 |
| `QuadBez` + `NewQuadBez(p0,p1,p2)` / `CubicBez` + `NewCubicBez(p0,p1,p2,p3)` | 二次/三次贝塞尔几何求值族（Eval/Extrema/Subdivide） | 贝塞尔求值几何 | 🔗 |
| `Matrix` + `Identity/Translate/Scale/Rotate/Shear` | 2D 仿射矩阵（x'=a·x+b·y+c，y'=d·x+e·y+f）；构造原语 + TransformPoint/Invert/ScaleFactor | 2D 仿射矩阵 | ✅ |
| `ColorFunc(x,y) RGBA` | 位置→颜色函数签名（驱动 CustomBrush） | 自定义上色函数 | 🔗 |
| `TextureView = gpucontext.TextureView` | GPU 纹理视图句柄（离屏 RT/层/滤镜结果） | GPU 纹理句柄 | 🔗 |
| `GPURenderTarget` | GPU 呈现目标句柄（加速器填充接口参数）；`SeedFromData` 为真时先把 `Data` 里的预乘 RGBA CPU 底上传进离屏 `View` 再重放待画命令（零读回滤镜/特效发布用，Skia filter-layer 语义） | GPU 目标句柄 | 🔗 |
| `MaxTrackedDamageRects` | 每帧最多跟踪的损伤区数量上限 | 损伤区上限 | 🔗 |
| `Logger()/SetLogger(l)` | 全局日志（slog 实例读写）；SetLogger 替换实例 | 全局日志 | 🔗 |
| `GPUContextCount()` | 活动 GPU 上下文注册数（设备切换/恢复诊断） | GPU 上下文计数 | 🔗 诊断 |
| var `ErrFallbackToCPU` / `ErrNilSurfaceView` | 回退到 CPU 渲染的错误 / 空表面视图错误 | 回退/空表面错误 | 🔗 |
| `SolveQuadratic/SolveCubic/SolveQuadraticInUnitInterval/SolveCubicInUnitInterval` | 多项式求根；InUnitInterval 各版本仅返回 [0,1] 实根 | 多项式求根工具 | 🔗 |
| `SetMSAASampleCount(n)` / `MSAASampleCount()` + 常量 `MSAASampleCount1`(1) / `MSAASampleCount4`(4) | 全局默认 MSAA 采样数配置：引擎默认 **1x**（analytic fringe 抗锯齿，Skia kCoverage 语义）；Set(1)=强制 1x、Set(4)=显式 4x、Set(0)=重置引擎默认（1x）；未设置时 `MSAASampleCount()` 直接返回 1，调用方无需额外兜底。唯一配置入口（env `GPUI_SURFACE_SAMPLE_COUNT` 已移除，2026-08），作用于之后创建的 GPU 会话；特效离屏（SetEffectSurface）恒走 1x 不受影响 | 默认采样数配置 | ✅（render/sample_count.go，2026-08） |

## 2. 主包：路径

| API | 功能说明 | 精简 | 状态 |
|-----|---------|------|------|
| `Path` + `NewPath()` | 即时路径：MoveTo/LineTo/QuadraticTo/CubicTo 追加；Rectangle/Circle/Ellipse/Arc/RoundedRectangle/EllipticalArc 形状；Close/Clone/Transform/Append/Iterate/Verbs/Coords；Bounds/Contains/Area/Winding 等查询 | 即时路径全部操作 | ✅ |
| `PathBuilder` + `BuildPath()` | 声明式路径构建链：MoveTo/LineTo/QuadTo/CubicTo/Rect/RoundRect/Circle/Ellipse/Polygon/Star → 收 Path() | 链式路径构建器 | ✅ |
| `PathVerb`（MoveTo/LineTo/QuadTo/CubicTo/Close） | 路径指令枚举（Iterate 迭代产出） | 路径指令枚举 | ✅ |
| `ApplyDash(p, dash)` | 把虚线序列应用到路径、生成带实线段的虚线化路径（描边内部用） | 路径虚线化 | 🔗 software/GPU 两腿都用 |
| `Dash` + `NewDash(lengths...)` | 虚线间隔序列对象（等长或成对交替） | 虚线序列 | 🔗 |
| `ParseSVGPath(d) (*Path, error)` | 解析 SVG path 数据串为 Path（`M/L/C/Q/A…`） | 字符串→路径 | 🔌（svg 包内部用，svg 包本身无消费者） |
| `PathBooleanOp`（Union/Intersect/Difference/Xor）+ `BooleanPath(a,b,op)` / `Path.Op` | 路径布尔并/交/差/异或。**2026-10-19 重写为 Clipper 扫描线（Vatti）精确几何布尔**：曲线按 0.1px 展平 → `github.com/ctessum/go.clipper v0.1.2`（BSL-1.0、纯 Go、运行期零传递依赖）NonZero/NonZero → 轮廓回写 Path；坐标 ×1024 定标（精度 1/1024px）、±1e6px 钳位、**删掉 2048px 静默截断**；方框减孔 800 轮廓/4000 verbs → **2 轮廓/10 verbs**，op 48.3ms → **9µs**（两圆并集 379ms → 100µs）；输出方向同时满足 NonZero/EvenOdd 填充。差集挖洞曾判为布尔实现失效，2026-10-19 定位为 CPU tile 栅格器缺陷并修复（`TestS3c_M3_PathBooleanDifference` 绿）。**2026-10-20 真窗 `ui_render_base_pathbool`**：四运算 + 双填充规则 + x≈3000 远坐标；逻辑 10/10、像素 18/18、Golden 627200px 零差、抓屏同区 0.0000%；同批修半透明 stencil path 颜色被重复预乘（RGB×α²）。输出为折线，输入曲线不保曲线 | 路径布尔运算（精确几何） | 🧪 仅测试 + 直调真窗 `ui_render_base_pathbool`（原子验证） |
| `PathMetric`（IsEmpty/Length/PositionAt/TangentAt 等）+ `Path.ComputeMetrics` | 对路径做度量：总长、某弧长处取点/切线 | 路径度量 | 🔗 |
| Path 造型：`Trim` / `WithCorners` / `Discrete` / `Flatten` / `Reversed` / `Area` / `Winding` / `Contains` / `BoundingBox` | Flatten 细分为折线/多边形；Area/Winding/Contains/BoundingBox 查询；Trim(子段)/WithCorners(圆角化)/Discrete(切段虚线 + 确定性 ± 交替横偏；2026-10-19 修「恒单侧偏移 + 周期 2×」) 高级造型 | 造型与查询 | 前四项内部用；**Trim/WithCorners/Discrete 🔌 仅测试**（直调真窗 `ui_render_base_pathshape`） |

## 3. 主包：绘制上下文 Context（199 导出方法，2026-10-23 S2b C2b 后 go/ast 重测；2026-10-22 为 200）

> **构造器与选项**：`NewContext(width,height,opts...)` / `NewContextForPixmap(pm)` / `NewContextForImage(img,opts...)` / `NewContextWithScale(w,h,scale)`；`ContextOption` 模式：`WithRenderer/WithPixmap/WithPipelineMode/WithDeviceScale`。状态 ✅（embedder 生产链路）。

> **`Canvas`（canvas.go，2026-10-22 S2a 建、2026-10-23 S2b C2b 补齐）**：绘制面接口，SkCanvas 同位物，**102 个方法**按 canvas.go 注释分 12 组（表面范围与清除 · 状态 · 变换 · 路径构建 · 绘画状态 · 填与描 · 裁剪 · 形状 · 图像与网格 · 文本 · 合成图层 · 后特效；逐方法清单见 §8 的 `Canvas` 行）。两个实现：`*Context`（即时，调用即光栅）与 `recording.Recorder`（将同一批调用存成显示列表），各带编译期断言（`var _ Canvas = (*Context)(nil)`、`var _ render.Canvas = (*Recorder)(nil)`），因此同一段绘画代码可原样跑在实况与录制两侧。故意**不进** Canvas 的是只有拥有 GPU 的那条线程才存在、或结果是回放期事实的方法：GPU 纹理句柄（`DrawGPUTexture*`）、`FlushGPU`/呈现/离屏通道、编解码导出、`SetPixel`/`WritePixels`、损伤跟踪；设备空间掩码已单独抽为 `MaskCanvas` 侧能力（仍不进 `Canvas`）；`TextPath`（字形轮廓提取）同属 live-only——它要用 Context 自己的轮廓抽取器与字形缓存，显示列表产不出它。UI 侧这类绘制统一走 `rendering.PaintContext.RequireLiveContext()` / `rendering.DrawGPUTexture()` 正口，示例禁止自己类型断言绕洞。**S3 起 GPU 纹理例外**：句柄仍不进 `Canvas`，但 `Recorder.DrawGPUTexture` 能把它录成 `DrawGPUTextureCommand`，回放端由可选 `GPUTextureBackend` 在设备线程绑定；`rendering.DrawGPUTexture` 在录制游标上返回 true 并落命令，只有完全没有纹理能力的目标才计入 `LiveOnlySkips`。状态 🔌（录制侧仅真窗/单测原子验证，生产消费者仍为零；接口本身由 `*Context` 承担现网全部调用）。

> 按文件分组；⚠️/🔌 标注仅为该行方法，未标注行默认 ✅/🔗。

### 3.1 context.go（106）
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
| RasterizerMode/SetRasterizerMode/PipelineMode/SetPipelineMode/SetTextMode/TextMode/SetLCDLayout | 选择 CPU 栅格化器、GPU 管线模式（render pass/compute）、文本策略、LCD 子像素布局 | 渲染模式 | ✅（`SetTextMode`/`TextMode`/`SetLCDLayout` 另有直调真窗 `ui_render_base_textlow`，2026-10-20 **原子验证**：逻辑 16 + 像素 28 + golden 627200px 零差 + 抓屏同区对照；同批修 `SetLCDLayout` 清空图集致灰度+LCD 同帧花屏） |
| SetEffectSurface/SetSharedEncoder/CreateSharedEncoder/SubmitSharedEncoder | 特效离屏强制 1x 采样、共享命令编码器（单命令缓冲帧，ADR-017） | 离屏/编码器 | ✅ |
| BeginGPUFrame/FlushGPU/FlushGPUWithView/FlushGPUWithViewDamage/FlushGPUWithViewDamageRects/GPURenderContext/DropGPURenderContext | 每帧 GPU 状态重置、把累积 GPU 命令提交并解析到 pixmap/视图（带单/多损伤区）、per-context GPU 会话 | GPU 提交 | ✅ |
| `BeginOffscreenPass()` | 开离屏子通道并返回还原 func（调用方须 defer）：激活期间队列命令/裁剪时间线/LoadOp 跟踪独占归子目标，主通路状态挂起、还原时恢复；同时挂起画布裁剪防 scissor 泄漏进子视口。使帧中 retained 纹理重录安全（Skia GrRecordingContext / Flutter EntityPass pass-ownership 语义）；无 GPU ops 时返回空 func | 离屏子通道 | ✅ ui/scene/textured.go:430,513 retained 重录消费 |
| `BeginPassScratch(r)` / `CommitPassScratchToView(view)` | retained 纹理重录子通道内的 CPU 暂存交换：Begin 把画布 pixmap 换成同尺寸透明暂存（CPU 回退像素落暂存而非主画布，半路 GPU 提交读回到暂存，顺序精确）；Commit 把脏区经 WriteTexture 直写视图并标已渲染使收尾提交 Load（干净暂存跳过零开销）。只在 GPU 会话有效时武装，CPU 模式零行为变化 | 离屏子通道 | ✅ ui/scene/textured.go recordWith/recordLocalWith 消费（2026-09-15 修染色混画黑卡） |
| RenderPathStats/ResetRenderPathStats/LastCPUFallbackReason/StagedWriteOverlaps/MemDigCmdBufs/Close | GPU/CPU 路由计数、最近 CPU 回退原因、后台上传守卫（同趟重叠写，0=健康）、残留命令缓冲诊断、关闭释放 | 诊断/资源 | 🔗 |
| `DebugLayerViews()` | 返回 C5VIEWDBG=1 下跟踪的层视图纹理（临时诊断，R20 滤镜线） | 诊断 | 🔗 ui/scene/textured.go 消费 |
| `PixmapForTest()` / `RecordCPUFallbackForTest(reason)` | 测试接缝：前者暴露实时 pixmap（含子通道暂存交换期的暂存）；后者走生产回退漏斗标脏暂存。仅 deterministic 模拟 CPU 回退落像素，不依赖设备相关 GPU 拒收路径 | 测试接缝 | 🧪 仅测试（render/pass_scratch_test.go E6 懒暂存契约消费，2026-09-23） |

### 3.2 context_clip.go（5）· 裁剪族
| 方法 | 功能 | 精简 | 状态 |
|------|------|------|------|
| `ClipRect(x,y,w,h)` | 矩形裁剪区（设备坐标），GPU 优先走硬件 scissor | 矩形裁剪 | ✅ GPU 有效（实测） |
| `ClipRoundRect(x,y,w,h,r)` | 圆角矩形裁剪（GPU scissor+SDF / CPU 逐像素 SDF） | 圆角矩形裁剪 | ✅ GPU 有效（单测） |
| `Clip()` / `ClipPreserve()` | **任意路径裁剪**（stencil+depth 管线；Preserve 保留路径） | 任意路径裁剪 | ✅ GPU 有效（2026-08-16 修复画穿+真窗复测，§7.3） |
| `ResetClip()` | 清除全部裁剪，恢复全画布 | 清除裁剪 | ✅ |
| `ClipRectOp(x,y,w,h,op)` / `ClipPathOp(op)`（ClipOpIntersect/Difference/Replace） | 带 ClipOp 运算的矩形/路径裁剪（非相交运算；difference/replace 走解析覆盖 mask，fill rule 按 `SetFillRule` 透传） | 运算式裁剪 | ✅ 直调真窗 `ui_render_base_clipop`（2026-10-19，14 逻辑 + 23 像素 + golden；同批修 4 缺陷，见 `RENDER_FEATURE_COVERAGE.md` §11）；2026-10-23 S2b C2b 修 `ClipPathOp` **设备空间二次变换**：路径动词在 `MoveTo/LineTo` 时已被用户矩阵烘焙，设备化只再乘 deviceMatrix，原实现乘 `totalMatrix()` 把用户 CTM 又乘一遍，裁剪偏离它要保的几何（带变换的录制/回放通道才暴露）；永久回归 `render/clip_ops_semantics_test.go` `TestClipOps_PathCtmInvariance`（3 算子 × identity/translate/scale/translate+scale 四变体，同一 painter 内由 CTM 反推用户坐标，断言逐像素着色数精确 40×40 / 40×40 / 120×120−40×40，mutation 反证：改回 totalMatrix 立刻红） |

### 3.3 context_mask.go（6）· 掩码族

> **`MaskCanvas`（mask_canvas.go，2026-10-26 新增）**：掩码侧能力接口，嵌入 `Canvas` 的 102 个用户空间方法 + 5 个设备空间掩码方法 `SetMask`/`ClearMask`/`InvertMask`/`ApplyMask`/`PushMaskLayer`。掩码平面按目标设备像素存放，不属于用户空间 `Canvas` 契约；`*Context` 与 `recording.Recorder` 均实现（各带编译期断言），UI 侧统一经 `rendering.PaintContext.MaskCanvas()` 取用。状态 🔌（录制回放已打通 full_paint 全帧，生产直调消费者仍为零；maskops 真窗为原子验证）。

| 方法 | 功能 | 精简 | 状态 |
|------|------|------|------|
| `SetMask(mask)` / `ClearMask()` / `GetMask()` / `InvertMask()` | 设置/清除/读取/反转后续绘制的 alpha 蒙版（GPU 侧同步上传 R8 纹理） | alpha 蒙版 | 🔌 无生产消费者（内部仅 clip 差集上传复用 R8 管线）；2026-10-19 直调真窗 `ui_render_base_maskops`（原子验证：逻辑 14 + 像素 18 + golden + 抓屏对照，`gpu_ops>0`/`cpu_fallback_ops=0`）；**2026-10-26 起经 `MaskCanvas` 可录制/回放**（full_paint 全帧，尺寸/能力不符计 `MaskSkips`；两连上屏 `screen=0.0000%`/`live_only_skips=0`/`mask_skips=0`） |
| `ApplyMask(mask)` | 对既有像素做 DestinationIn 蒙版合成（后处理帧，非影响未来绘制） | 蒙版后处理 | 🔌 无生产消费者；2026-10-19 真窗 `ui_render_base_maskops` 验证 Skia **DstIn** 语义（掩码矩形外像素不动）；2026-10-26 可录制为 `ApplyMaskCommand`，回放端按设备尺寸门执行并计跳过 |
| `AsMask()` | 把当前未填充路径栅格化为 Mask（临时光栅化取 alpha） | 路径→掩码 | 🔗 |
| `Mask` 类型：`NewMask/NewMaskFromAlpha/NewLuminanceMask/NewMaskFromData` + At/Set/Fill/Invert/Clone | 蒙版构造（空/alpha 图/亮度图/裸数据）与读写 | 蒙版构造 | 🔌 无生产消费者（ui/kit、2.5D、视频均未调用；真窗=原子验证，值级契约 14 项覆盖构造/读写/反转/克隆） |

### 3.4 context_layer.go（7）· 图层族
| 方法 | 功能 | 精简 | 状态 |
|------|------|------|------|
| `PushLayer(blend, opacity)` / `PopLayer()` | 压入/合成渐变层（GPU RT 优先，CPU 损伤合成兜底） | 层合成 | ✅ GPU 有效 |
| `PushLayerIsolated(opacity)` | 强制创建隔离离屏层（Flutter saveLayer 语义） | 隔离离屏层 | ✅ |
| `PushMaskLayer(mask)` | 建层并让整个层内容经 mask 调制后合成 | 蒙版层 | 🔌 无生产消费者；2026-10-19 真窗 `ui_render_base_maskops` 验证（图层 RT 经 R8 调制 pass 后排队可重放 blit，无帧中 CPU 读回；CPU 回落保留）；2026-10-26 可录制为 `PushMaskLayerCommand`，回放不支持时退化普通层并计 `MaskSkips` |
| `PushBackdropLayer(blend, opacity)`（m4_extensions.go） | 用父画布快照预填充层（backdrop/毛玻璃底色播种）。快照取父 RT 走 RT→RT（`copyGPUTextureToTarget`，忽略裁剪与层组 alpha）；推入时裁剪即层边界（Skia saveLayer bounds），滤镜输出由 `cropFilterOutputToLayerBounds` 裁到层边界；Pop 合成保持整面（`drawGPUTextureWithOpacityFullSurface`，呈现恢复承重机制） | 背景快照层 | 🔗 ui/scene/composite + ui/rendering/filter_draw 在用；2026-10-19 直调真窗 `ui_render_base_backdrop`（原子验证：逻辑 14 + 像素 21 + golden 627200px 逐位零差 + 抓屏 vs 快照同区零差，`gpu_ops>0`/`cpu_fallback_ops=0`） |
| `LayerPoolStats()/ResetLayerPoolStats()` | 读取/清零层表面池 gets/puts/hits/misses | 层池统计 | 🧪 仅测试（s6_4_layer_filter_test） |
| `SetBlendMode(mode)` | 设置后续填充/描边的混合模式 | 混合模式 | ✅ |

### 3.5 context_image.go（12）· 图像族
| 方法 | 功能 | 精简 | 状态 |
|------|------|------|------|
| `DrawImage/DrawImageEx` | 绘制图像（GPU QueueImageDraw；Bicubic 走 GPU 4×4 卷积变体 Catmull-Rom，GPU 失败回退 CPU） | 图像绘制 | ✅ GPU 有效 |
| `DrawImageOptions{X,Y,DstWidth,DstHeight,SrcRect,Interpolation,Opacity,UseMipmaps,BlendMode}`（context_image.go） | 图像绘制的目标位置/缩放/源矩形/插值/透明度/mipmap/混合选项 | 图像绘制选项 | ✅ DrawImageEx 与内部图像路径使用 |
| `DrawImageCircular/DrawImageRounded` | 在圆形/圆角矩形裁剪区内绘制图像 | 裁剪内画图 | 🔗 |
| `DrawImageQuad(img,corners,opts)` + `QuadDrawOptions{Interpolation,Opacity,BlendMode}`（m4_extensions.go，R1 S01） | 带显式选项的梯形绘制：零值Bilinear/不透明；退化/自交/非有限/非Normal混合返回哨兵错（ErrQuadDegenerate/ErrQuadBowTie/ErrQuadNonFinite/ErrQuadUnsupportedBlend/ErrQuadUnsupportedInterp），不静默画错 | 梯形绘制 | ✅ GPU 有效（真窗game_quad 2026-09-15） |
| `ClassifyQuad(corners) QuadKind` + `QuadKind`（QuadOK/QuadDegenerate/QuadBowTie/QuadNonFinite） | 四边形分类：只有OK可画，其余画不得 | 梯形分类 | ✅（quad_cpu_test 2026-09-15） |
| `DrawImageNine`（nine_patch.go） | 九宫格缩放绘制图像（四角不变、边拉伸） | 九宫格缩放 | 🔗 ui/rendering/image_draw.go 已接门面；**无 Widget 生产调用 🔌** |
| `DrawGPUTexture` / `DrawGPUTextureBase` / `DrawGPUTextureWithOpacity` / `DrawGPUTextureWithOpacityUV` | GPU 纹理直接合成（Base 作 render pass 背景层零读回；Opacity/UV 带不透明度/子区） | GPU 纹理合成 | ✅ |
| `ExportImageBuf(dst **ImageBuf) bool` | 把画布导出/复用为图像缓冲（特效连续帧免分配） | 画布→缓冲 | ✅ |
| `CreateOffscreenTexture(w,h)` | 创建离屏 RT 纹理视图（层/滤镜目标） | 离屏纹理 | ✅ |
| `NewImagePattern(img,x,y,w,h)`（包级，2026-10-23 S2b C2b 由 `Context.CreateImagePattern` 改来：由源矩形建图样不需要目标表面，录制面也要能描述它）+ `NewImagePatternFromSource(ImagePatternSource)` + `Context.SetFillPattern/SetStrokePattern` | 用图像区域建立可平铺图案并作为填/描画刷；`Source()`/`FromSource()` 双向搬运完整描述（图/子矩形/采样逆矩阵/不透明度/clamp），往返逐位一致——回放不许再求一次逆 | 图案纹理着色 | 🧪 仅测试/真窗（生产走 Brush）；`imagepattern_source_roundtrip` 探针锁往返 |
| `IsAdvancedBlendMode(mode)` | 判断混合模式是否为高级（非基础 Porter-Duff 集） | 高级混合判定 | 🔗 |
| `ImagePattern` 类型 + `Pattern`/`SolidPattern`/`PatternFromBrush`/`BrushFromPattern` | 图案类型及其与画刷的双向桥接 | 图案桥接 | 🧪 仅测试/桥接 |
| `ImageBuffer`：`ImageBuf`(+`NewImageBuf`/`ImageBufFromImage`/`LoadImage`/`LoadWebP`) / `Pixmap`(+`NewPixmap`/`NewPixmapFromBuffer`/`FromImage`，方法见 §8) / 像素格式 | 图像缓冲与 CPU 像素缓冲的构造与加载 | 图像/像素缓冲 | ✅ `LoadWebP` 真窗 `ui_render_base_webp`（2026-10-20：无损 vs PNG 四通道零差 + Alpha / 有损 VP8 / `.webp` 与 `.bin` 内容识别 / 缺失·非法·空数据错误；永久回归 `render/webp_contract_test.go`） |

### 3.6 filter_ops.go（8）· 滤镜族
| 方法 | 功能 | 精简 | 状态 |
|------|------|------|------|
| `ApplyBlur/ApplyBlurXY/ApplyColorMatrix/ApplyDropShadow/ApplyGrayscale/ApplyInvert` | 对当前内容做高斯模糊/颜色矩阵/投影/灰度/反色滤镜 op | 内容滤镜 | 🔌 无生产消费者；真窗 `ui_render_base_filterops`（2026-10-20 **原子验证**：逻辑 16 + 像素 26 + golden 627200px 逐位零差 + 抓屏 vs 快照 0.0000%；同批修 GPU 反色预乘 `1−c` → `a−c`）；需 blank-import `render/filters` 生效 |
| `ApplyImageFilterGraph(nodes...)` / `GPUFilterTexture()` | 建 GPU 图像滤镜图（零读回发布）并取结果纹理 | GPU 滤镜图 | 🔌 无生产消费者；真窗 D 卡发布 199×130 视图并每帧 `DrawGPUTexture` 直绘（2026-10-20 **原子验证**）；GPU 图注册时生效，CPU 路径返回 `ok=false`；ui 侧 facade 无生产调用 |
| `ImageFilterKind`（Blur/BlurXY/ColorMatrix/DropShadow/Grayscale/Invert）+ `ImageFilterNode` | 滤镜图节点类型/数据 | 滤镜节点类型 | 🔌 无生产消费者（真窗与永久回归在用） |
| 注册函数：`RegisterFilterOps` / `RegisterGPUFilterGraph` / `RegisterGPUFilterGraphTexture` / `RegisterGPUFilterGraphFromView` / `SwapGPUFilterGraph` / `FiltersRegistered` / `GPUFilterGraphRegistered` / `FilterPoolStats` / `ResetFilterPoolStats` / `DisableGPUFilterGraphForTest` | 注册滤镜 op 与 GPU 图、统计池、测试开关 | 滤镜注册/统计 | 🔗/🧪 混合（GPU 图注册多为测试/宿主注入） |

### 3.7 frame.go（6）+ present.go（3）+ present_target.go · 帧与呈现族
| API | 功能 | 精简 | 状态 |
|------|------|------|------|
| `BeginFrame/Invalidate/MarkFullRedraw/ResetFrameDamage` | 帧生命周期：重置统计、标记损伤、强制全量重绘 | 帧生命周期 | ✅ |
| `PresentFrame/PresentFrameDamage/PresentFrameDamageRects`（present.go） | 提交 GPU 内容到视图并调用 present（带单/多损伤区） | 呈现提交 | ✅ |
| `PresentFrameAuto/PresentFrameFull`（frame.go） | 自动选路径 / 强制全帧呈现 | 自动/全帧呈现 | ✅（PresentFrameAuto 内部用；2026-10-21 GL 后端经 `EndFrameWithDamage` 走 EGL 局部提交——三扩展齐备且 FBO 1:1 时 `SetDamageRegionKHR` + 剪刀 blit + `SwapBuffersWithDamage`，缺一/大损伤/首帧全量兜底；wgpu-native 仍全量。原子验证 `TestPresentDamageX11`） |
| `PlanFramePresent/PlanPresent/FramePresentPlan/PresentOutcome/PresentMode` | 依据损伤区与 surface 尺寸规划呈现策略 | 呈现策略规划 | ✅（PresentFrameAuto 内部用） |
| `CoalesceDamageRects` | 把多条损伤矩形合并/裁剪到上限 | 损伤合并 | 🔗 |
| `PresentTarget` + `NewPresentTarget`（方法：Context/Resize/SetVsync/PresentWith/PresentWithAuto/PresentClear/LastPresentOutcome/LastDamageAreaPx/LastDamageRectSumPx/GPUBackend/Fallbacks/InFullRecovery/SetResizeStormWindow/SetOnSwapchainResized/LogicalSize/Scale/WaitIdle/Close） | 呈现目标对象（X11/Wayland/Win32/AppKit；风暴 resize 状态机；`LastDamageAreaPx`=末帧损伤**并集框**面积，`LastDamageRectSumPx`=末帧合并后**各损伤矩形面积之和**——散点脏层时并集框会把中间没动的缝隙也算进去，rect-sum 才是真实提交代价，2026-10-22 R4 指标新增，`render/present_frame.go:144`） | 呈现目标 | ✅ embedder 在用（R4 风暴窗口状态机所在；**SetOnSwapchainResized 🔗 Wayland 宿主在用作 xdg 窗口几何声明**；**SetVsync 🔗 embedder 在 resize 风暴期切 Mailbox/Immediate 免 Fifo 阻塞；两平台均生效（2026-08-26 修3 起 Wayland 不再 no-op）**；**GPUBackend 🔗 实际显卡类别上报口（discrete/integrated/software），多窗降级链（1.3）复用**；**WaitIdle 🔗 关窗先等 GPU 空闲再清图块缓存（2026-10-04 关窗漏修，ui/embedder PipelineApp.Close 在用）**） |
| `AdapterPolicy`（PolicyDefault/PolicyHigh/PolicyLow + 别名 PolicyNone/PolicyAuto/PolicyHighPerformance/PolicyLowPower）+ `ResolveAdapterPolicy` + `RequestAdapterWithPolicy` + `DeviceDescriptor/DeviceDescriptorLowVRAM/DeviceDescriptorForAdapter`（adapter_policy.go，2026-09-05 自 render/gpu 搬入，函数名与行为一字不动；2026-09-18 B4：`DeviceDescriptorForAdapter` 加台账水位自动 LowVRAM（≥80% 预算，`GPUI_LOW_VRAM_WATERLINE_PCT` 可调）+ `AcquireSharedPresentDevice`（present_target.go，探针/离屏借共享设备，单设备多表面）） | 选卡策略（GPUI_POWER=high/low，默认混搭优先核显）与设备描述符（核显/CPU/水位超限走 LowVRAM 紧限） | 适配器策略 | 🔗 NewPresentTarget 建窗调用（render/present_target.go）；forceFallback 时 stderr 如实记日志 |
| `IsGPUOutOfMemory(err)` + `PurgeEvictable` 接口 + `RegisterPurgeEvictable/UnregisterPurgeEvictable/PurgeEvictables`（oom_purge.go，多窗 1.3）+ `PresentTarget.Fallbacks()` + `OOMExitThreshold/OOMExit`（oom_exit.go，2026-09-18 B3：三帧判死与就绪门同一阈值同一文案，ui/embedder 只做委托） | OOM 统一判定（大小写全匹配三短语）与可重建缓存 purge 链（字形/图片/层池注册、teardown 注销）+ 建窗降级次数上报 + 判死阈值 | OOM 判定与 purge | 🔗 建窗降级循环 + 建纹理 OOM 恢复轮 + embedder 退出判定在用 |
| `Backend`（BackendNative/BackendGo）+ `ResolveBackend`/`ResolveBackendFor` + `SelectBackend` + `NewPresentTargetWithBackend`（backend.go/present_target.go，2026-09-30 P4：后端二选一，`GPUI_BACKEND`环境变量优先，只认native/go，写错报错不瞎猜；带参建窗零值默认，老不带参函数保留转调） | 后端选择（默认WebGPU，加`go`切纯Go GL；创建分支只留建实例和建交换链两句，之后全走hal接口） | 后端二选一 | 🔗 NewPresentTarget 建窗调用（render/present_target.go:ResolveBackend+SelectBackend；render/present_swapchain.go:ResolveBackend建交换链）；真窗双验2026-09-30（go路鹈鹕12秒720帧58fps零回退discrete／默认路723帧58.3fps零回退integrated／写错值直接报错退出） |
| `PresentNativeSurface` / `PresentPlatform` / var `ErrNilSurfaceView` | 原生表面句柄、平台枚举、空表面错误 | 原生表面/平台 | ✅ |
| `FrameAcquireWaitMs/FrameFlushMs/FramePresentWaitMs` | 帧获取/提交/呈现等待毫秒统计（帧耗时诊断） | 帧耗时统计 | 🔗 |
| `PictureCacheFairMax` | 图片缓存公平上限（多窗按下限/平均条目算） | 图片缓存上限 | 🔗 |
| `RecoverSharedDevice/SharedDeviceGeneration/SharedWindowCount` | 共享设备恢复/代次/窗口计数（多窗共享设备账本） | 共享设备账本 | 🔗 |
| `VramBudgetMB/VramLiveBytes/VramLiveCount/VramPeakBytes/VramPressureHigh`（shared_budget.go，2026-10-07 T3 收口：转调从 `webgpu` 换 `hal`，同账本数字不变） | 显存预算/存活字节/存活数/峰值/压力判定（进程账本） | 显存账本 | 🔗 |
| `OOMExit.Note(err)/OOMExit.RunErr()`（oom_exit.go） | 判死记录/取退出错误（连续 OOM 报人话退出） | 判死记录 | 🔗 embedder 在用 |

### 3.8 text.go（16）+ text_decoration.go（2）+ text_mode.go · 文本族
| 方法 | 功能 | 精简 | 状态 |
|------|------|------|------|
| `SetFont/Font/LoadFontFace/LoadFontFaceWithVariations/FontVariationAxes` | 设置字体、读当前字体、加载 TTF/OTF 与可变字体轴 | 字体管理 | ✅ ui/rendering paint_context 在用 |
| `DrawString/DrawStringAnchored/DrawStringWrapped/DrawShapedGlyphs/DrawShapedColorGlyphs/SetTextDecoration/TextDecoration` | 绘制文本（锚点/自动换行/已整形字形/已整形彩色字形/下划线等装饰） | 文本绘制 | ✅（DrawString/Wrapped 生产在用；**DrawShapedGlyphs 🔗 生产在用（ui/rendering TextLayout 批量主路 + ui/scene 录制回放 + render/scene GPU 解析，见 §7.2 行注）**；**DrawShapedColorGlyphs 🔗 生产在用（ui/rendering 彩色段→GPU 颜色图集/CPU 兜底，GPU 真窗已验：ui_text_m5_anycase 10s 回退 0、B 区真彩）**；**StrokeString/StrokeStringAnchored/TextPath 🧪 仅测试 + 直调真窗 `ui_render_base_textstroke`（2026-10-19，原子验证：逻辑 14 + 像素 21 + golden 627200px 零差 + 抓屏同区对照；同批修描边损伤不按线宽外扩 / 轮廓缓存 key 截断分数磅值 2 缺陷）**） |
| `SplitColorGlyphs(cf,glyphs)`（text.go 顶层函数） | 按字形类型把已整形字形拆成彩色/描边两子集（保序），供 dispatch 入口分流 | 彩色字形分流 | 🔗（dispatchText 在用） |
| `MeasureString/MeasureMultilineString/WordWrap` | 度量单行/多行文本、按宽度断词换行 | 文本度量 | `MeasureString` 🔗 内部用（UI 布局未接，用自研估算）；**`MeasureMultilineString/WordWrap` 🧪 仅测试 + 直调真窗 `ui_render_base_textlow`（2026-10-20，原子验证：逻辑 16 + 像素 28 + golden 627200px 逐位零差 + 抓屏同区 `diff=0.0000%`；同批修 WordWrap 行尾空白/CRLF 偏移 + 多行可视块高度口径 2 缺陷，pin `render/text/wrap_test.go` / `render/text_wrap_test.go`）** |
| `TextMode`（Auto/MSDF/Vector/Bitmap/GlyphMask/Aliased）/ `LCDLayout`（None/RGB/BGR）/ `Align` | 文本渲染策略 / LCD 子像素布局 / 对齐枚举 | 文本模式 | ✅ |

### 3.9 vertices.go（3）+ shapes.go（1）· 顶点/网格族
| 方法 | 功能 | 精简 | 状态 |
|------|------|------|------|
| `DrawVertices(pos,cols,mode)` / `VertexMode` | 顶点数组绘制（三角扇/条带/列表） | 顶点数组 | 🔌 无生产消费者（仅测试 + 直调真窗 `ui_render_base_verticesmesh`，2026-10-20 原子验证） |
| `DrawMesh(mesh)` / `Mesh` | 网格绘制（膨胀路径转三角形） | 网格绘制 | 🔌 无生产消费者（仅测试 `p1_capability_matrix_closers_test` + 直调真窗 `ui_render_base_verticesmesh`，2026-10-20 原子验证） |
| `DrawAtlas(img,sprites)` / `AtlasSprite` | 单纹理图集批量精灵绘制 | 图集绘制 | 🔗 |
| `DrawRegularPolygon(n,x,y,r,rot)` | 正多边形形状追加到路径 | 正多边形 | 🔗 |
| `DrawVerticesEx/DrawMeshEx/DrawAtlasEx/DrawDepthSprites`（vertices_mesh.go/vertices_atlas.go） | 带回退原因的顶点/网格/图集/深度精灵绘制（返回 CPU 回退原因） | 顶点绘制扩展 | 🔌 无生产消费者（仅测试 + 直调真窗 `ui_render_base_verticesmesh`；Ex→冻结路径自调用属内部路径，非生产接线） |
| `VertDrawOptions/VertDrawResult/VertCPUFallbackReason` + var `ErrVertsBadIndex/ErrVertsNonFinite` | 顶点绘制选项/结果/回退原因 + 非法索引/非有限坐标错误 | 顶点绘制参数 | 🔗 |
| `AtlasDrawOptions/AtlasDrawResult/AtlasCPUFallbackReason` + var `ErrAtlasNonFinite/ErrAtlasUnsupportedFilter` | 图集绘制选项/结果/回退原因 + 非有限矩形/不支持过滤错误 | 图集绘制参数 | 🔗 |
| `DepthSprite` + `NewDepthSprite` + `DepthDrawOptions/DepthDrawResult/DepthCPUFallbackReason` + var `ErrDepthNonFinite` | 深度排序精灵及绘制选项/结果/回退原因 | 深度精灵 | 🔗 |
| `IsDepthSorted/SortDepthSprites/SetDepth` | 深度排序判定/排序/设深度 | 深度排序 | 🔗 |

### 3.10 m4_extensions.go（其余）
| 方法 | 功能 | 精简 | 状态 |
|------|------|------|------|
| `SetDither/Dither` | 启用/关闭解析后有序抖动（`Image`/`FlushGPU` 读回后的整张 pixmap Bayer 后处理；不影响 Present 主路） | 抖动 | 🧪 仅测试 + 直调真窗 `ui_render_base_dither`（2026-10-20，原子验证；16 逻辑 + 29 像素 + golden + 抓屏；3 项限界见 `RENDER_FEATURE_COVERAGE` §6） |
| `PushBackdropLayer` | 见 §3.4 | — | — |

### 3.11 video_direct.go（9+16）· 视频直传地基族（2026-09-30 视频P0；2026-10-02 P2/P3-A 同步）
| API | 功能 | 精简 | 状态 |
|------|------|------|------|
| `VideoBackendCaps`（HasDevice/AdapterName/SupportsCommandCopies/MaxTexture2D/MaxStagingBytes/RowPitchAlignment） | 当前共享设备的后端中性能力（名字只做诊断，分支只看能力） | 视频后端能力 | ✅（17 扇窗借用） |
| `QueryVideoBackend(device)` | 由借入设备算能力（空设备返回 HasDevice=false，不崩） | 查视频能力 | ✅（桥借用） |
| `BorrowVideoBackend()`（device/queue/adapter/caps/ok） | 只借当前窗口那套共享设备（不拥有不释放，无窗返回 ok=false） | 借视频设备 | ✅（17 扇窗借用） |
| `VideoSlot`（W/H/Texture/View） | 一路视频一块纹理，每帧原地重写，尺寸变了才重建 | 视频纹理槽 | ✅（桥持有） |
| `VideoTexturePool` + `NewVideoTexturePool(device)` | 视频纹理独立池（另列，不进 64MB 通用图缓存，不参与其淘汰） | 视频独立池 | ✅（桥持有） |
| `VideoTexturePool.Acquire/Release/Stats/Close/AcquireForFrame/Upload` + `VideoPoolStats`（Live/Idle/Peak/Evictions/Uploads） | 按 exact 尺寸借还（健康态零淘汰，超 8 槽淘汰最旧并计数）；整块一次上传计 1（P2 证据：上传数对帧数） | 视频池借还 | ✅（桥持有，20 单测绿） |
| `Context.RecordVideoFallback(reason)` + `VideoFallbackTotal()` | 直传走不通回落通用 DrawImage，保证有画面，同时记 cpu_fallback_ops（+ 全局计数） | 视频回落计数 | ✅（桥回落） |
| `VideoDrawOptions`（X/Y/DstWidth/DstHeight/Opacity/Interpolation） | 解码分辨率不动，显示只看目标矩形 | 视频画参 | ✅（桥绘制） |
| `VideoUploadTotal()` | 全进程直传上传计数（平面组计 1，和整块口径一致） | 上传计数 | ✅（门禁） |
| `Context.DrawVideoFrame(img, opts)` | 通用图片路保底绘制（回落用） | 视频保底画 | ✅（桥回落） |
| `Context.DrawVideoSlot(slot, opts)` | 零上传纹理四边形（无 GPU 会话失败闭合） | 视频快画 | ✅（桥快路） |
| `Context.DrawVideoPlanes(slot, opts)` | NV12 平面四边形 + 显卡转色（一份 WGSL 两后端通用，无会话/坏参/变体缺失时失败闭合走 CPU 转回落） | 平面快画 | ✅（桥绘制，缺变体回落） |
| `VideoBridge` + `NewVideoBridge/EnsureDevice/Close` + `BridgeStats`（Frames/Uploads/Fallbacks/Redraws/Live/Idle/Evictions） | 一路视频收敛：tick 上传 + paint 绘制；上传+回落==帧数；无新帧不传 | 视频桥 | ✅（17 扇窗在用） |
| `VideoBridge.Show/ShowSeq/UploadFrame/DrawCurrent/UploadPlanesFrame` | 整块上传绘制 + 同帧多视图免费重画 + 平面上传统一计数 | 桥上传绘制 | ✅（窗 tick/paint） |
| `VideoBridge.UploadPlanesFrameAsync(w,h,y,uv)` / `UploadPending()` | 异步平面口：调用线程只交接，worker 拷贝（跨线程后端直接上传并双缓冲换槽；GL/metal 在光栅绘制时上传）；交接后平面归路线所有，`UploadPending` 为真期间不得复用/再次 Poll；坏参/已关/无设备/上一帧未发布返回 false | 异步平面上传 | ✅（4K 真窗 30s：uploads=frames=107、p95 19.37ms） |
| `VideoBridge.PaintTexture(dc, opts)`（`TextureSource` 实现） | 纹理源入口：光栅线程按 `scene.TextureLayer` 的 id 解析后调用，桥锁内绘制当前帧；`ok=false` 表示没画（无帧/已关/无设备），调用方不得把矩形当已覆盖 | 纹理源绘制 | ✅（`ui/scene/composite.go:281` 合成 + `RenderVideo.Paint`） |
| `VideoBridge.ID()` / `VideoBridge.PaintStats()` + `PaintStats`（Paints/DirectPaints/FallbackPaints/FailedPaints/LastPaintOK/LastPaintDirect/ViewPaints/ViewOK/ViewLastOK/ViewDirect） | 注册 id（0=未注册）与逐视图绘制账本；`ViewOK` 是「曾经画过」、`ViewLastOK` 是「最近一帧画没画」，门禁断当前画面用后者；光栅线程画的也记在这里，生产者/节点只读快照，不靠活对象字段 | 纹理源 id / 绘制账本 | ✅（`RenderVideo.NodeStats` 读，`ui/rendering/video_direct_node.go:153`） |
| `VideoPlaneSlot`（W/H/YTex/YView/UVTex/UVView/垫行暂存） | 一路视频一组平面纹理（Y 用 R8、UV 用 RG8），原地重写 | 平面纹理槽 | ✅（桥持有） |
| `VideoPlanePool` + `NewVideoPlanePool(device)` + `Acquire/Release/Stats/Close/AcquireForFrame/UploadPlanes` | 平面独立池；两平面各整块一次计 1 组上传；非 256 对齐行垫进槽暂存（memcpy，无算术） | 平面池上传 | ✅（桥持有，单测绿） |


### 3.12 context_misc_draw.go（5）· 杂项绘制族（2026-10-20 A 组#15）
> 五个 `Canvas` 对等入口；全部保留当前路径/Paint 状态（DrawColor/DrawPaint 只改目标 clip），无生产消费者，属**已验证待接线**（原子验证：直调真窗 `ui_render_base_miscdraw` + CPU/GPU 永久回归）。

| 方法 | 功能 | 精简 | 状态 |
|------|------|------|------|
| `DrawShadow(path *Path, col RGBA, elevation float64, transparentOccluder bool)` | 按 Impeller/Skia spot 阴影模型画路径海拔阴影：隔离层内把路径平移 `elevation*dpr` 设备像素、以 `(800/600)*elevation*dpr` 模糊、约 25% alpha 的 spot 色；空路径/海拔≤0/全透明色为 no-op。`transparentOccluder` 为签名兼容保留，当前不改变绘制（见 `RENDER_FEATURE_COVERAGE` §6） | 路径海拔阴影 | 🔌 无生产消费者；CPU 契约 + GPU 像素回归 + 真窗 `ui_render_base_miscdraw` A 卡 |
| `DrawColor(col RGBA, mode BlendMode)` | 用独立 Paint（实色、关 AA、清 `TransformScale`、指定混合模式）填满**当前 clip**，忽略调用方当前画刷/alpha/blend；当前路径与 Paint 状态不变。普通模式直接填，高级混合（Multiply/Screen 等）自动开隔离层走 dual-tex 合成 | 整 clip 着色 | 🔌 无生产消费者；CPU 契约 + GPU 像素回归 + 真窗 B 卡 |
| `DrawPaint()` | 用当前 Paint 的克隆填满**当前 clip**（含画刷/渐变/alpha/混合模式/填充规则），只把 `TransformScale` 归 1；当前路径与 Paint 状态不变。高级混合同样走隔离层 | 用 Paint 填 clip | 🔌 无生产消费者；CPU 契约 + GPU 像素回归 + 真窗 C 卡 |
| `ClipBounds() image.Rectangle` | 返回当前裁剪在**本地坐标**下的包围盒：取设备 clip（无裁剪=整画布），反 `totalMatrix` 变换四角再取 AABB；路径裁剪按路径坐标 AABB（含控制点），不再把全画布 mask 当界 | 本地裁剪界 | 🔌 无生产消费者；CPU 契约 + 真窗 D 卡四态 |
| `DrawParagraph(layout *text.Layout, x, y float64)` | 把已布局段落逐行、逐 run 交给 `DrawShapedGlyphs`：基线 `y+line.Y`，字形位置取 run 内坐标，Face 取 run.Face；nil/空布局 no-op。单色（当前画刷），不接管 Paragraph 样式/装饰/命中模型 | 绘段落布局 | 🔌 无生产消费者；CPU 契约 + GPU 像素回归 + 真窗 E 卡（MultiFace 三段 36/34/10 字形） |

### 3.13 texture_source.go（2 类型 + 4 函数）· 纹理源注册族（2026-10-24 S2b C4）
> 给「像素只存在 GPU 纹理里、显示列表描述不了」的内容一个 id 化入口（Flutter `TextureLayer` 同构）：图层只带 id，光栅线程按 id 解析源并绘制，源自己负责与生产者的跨线程同步。视频桥是第一个实现，后续实时纹理（摄像头/外部 GL 纹理）走同一口。

| API | 功能 | 精简 | 状态 |
|-----|------|------|------|
| `TextureSource`（接口，`PaintTexture(dc, opts) (direct, ok)`） | 纹理内容契约：`dc` 当前变换生效；`direct=true` 表示零上传 GPU 快路；`ok=false` 表示没画（已释放/无帧/无设备），调用方不得把矩形当已覆盖 | 纹理源契约 | ✅（`VideoBridge` 实现；`ui/scene` 合成调用） |
| `TexturePaintOptions`（X/Y/Width/Height/Opacity/Interp/DrawIndex） | 一次纹理绘制的位置与视图号；`Opacity=0` 同 `VideoDrawOptions` 表示不透明；`DrawIndex` 区分同一源的多视图（0=首视图） | 纹理画参 | ✅ |
| `RegisterTextureSource(src) uint64` / `UnregisterTextureSource(id)` | 注册/注销纹理源，返回从 1 起的单调 id（0=无源）；nil 源不注册；注册方负责在拆除时注销（资源谁建谁放） | 注册/注销纹理源 | ✅（`NewVideoBridge` 注册 / `Close` 注销） |
| `GetTextureSource(id) (src, ok)` | 按 id 解析（保留图层只存 id，不存指针）；未注册/0 返回 ok=false，调用方画空并记 miss | 解析纹理源 | ✅（`ui/scene/composite.go:281`） |
| `LiveTextureSources() int` | 当前注册数，拆除断言用（每个注册的生产者都必须注销） | 存活纹理源计数 | 🧪 仅测试（`render/texture_source_test.go`） |

---

## 4. 主包：光栅化 / 加速 / 渲染器

| API | 功能 | 精简 | 状态 |
|-----|------|------|------|
| `Renderer` 接口（Fill/Stroke） | 渲染器抽象：实现方提供填充/描边绘制 | 渲染器抽象 | 🔗 |
| `SoftwareRenderer` + `NewSoftwareRenderer` | CPU 软渲染器（像素精确、作为 GPU 不可用兜底） | CPU 软渲染 | 🔗（CPU 兜底） |
| `SDFFilledCircleCoverage/SDFCircleCoverage/SDFFilledRRectCoverage/SDFRRectCoverage` | SDF 覆盖率函数（像素点→形状内覆盖率，填/描边） | SDF 覆盖率 | 🔗（CPU SDF/测试） |
| `SDFAccelerator`（无构造器 `&render.SDFAccelerator{}`） | 独立注册的 CPU SDF 加速器（形状快速填充） | CPU SDF 加速 | 🧪 生产仅作为 GPU 加速器内部 cpuFallback |
| `GPUAccelerator` 接口 + `Accelerator()/RegisterAccelerator/CloseAccelerator/SetAcceleratorDeviceProvider/PurgeAcceleratorSurfaceResources/AbandonAcceleratorDevice/AcceleratorCanRenderDirect/BeginAcceleratorFrame/CPUOnlyMode` | GPU 加速器注册/取回/关闭、设备提供者注入、表面资源清理、设备放弃、可直渲判定、帧开始、CPU-only 模式判定（`GOGPU_RENDER_MODE=cpu` → 全部绘制与窗口呈现走 CPU 光栅） | GPU 加速管理 | ✅（gpu 包注册，全部真窗在用） |
| 加速器能力接口群：`DeviceProviderAware/GPURenderContextProvider/FrameAware/MSAAAware/GPUTextAccelerator/GPUGlyphMaskAccelerator/GPUAliasedTextAccelerator/GPUShapedTextAccelerator/GPUColorGlyphAccelerator/GPUTransformMaskTextAccelerator/DirectRenderCapable/AdapterAware/ComputePipelineAware/PipelineModeAware/ForceSDFAware/ClipAware/RRectClipAware/PathClipAware/LCDLayoutAware/MaskAware/SceneStatsTracker` | 加速器可探测/可选择性实现的各项能力接口（宿主据此选路径）；**`PathClipAware.SetClipPath(path, fillRule)` 于 2026-10-19 增 `fillRule` 参数**（深度裁剪路径按自身 fill type 光栅化，`render/accelerator.go:606`） | 能力探测接口 | 🔗 |
| `AcceleratedOp`（Fill/Stroke/Scene/Text/Image/Gradient/CircleSDF/RRectSDF） | 加速操作位标记（加速器声明支持哪些 op） | 加速操作位 | 🔗 |
| `CoverageFiller` + `RegisterCoverageFiller/GetCoverageFiller` + `ForceableFiller`（SparseFiller/ComputeFiller） | 覆盖率填充器接口与注册；AdaptiveFiller（4x4/16x16 tile）在此注册 | 覆盖率填充器 | ✅ |
| `RasterizerMode`（Auto/Analytic/SparseStrips/TileCompute/SDF） | CPU 栅格化算法选择 | CPU 栅格化模式 | ✅ |
| `PipelineMode`（Auto/RenderPass/Compute）+ `SelectPipeline` + `SceneStats` | GPU 管线模式选择（自动/渲染通道/计算） | GPU 管线模式 | ✅（SelectPipeline 生产调用：render/internal/gpu/gpu_render_context.go:2820，Auto 模式每帧） |
| `DetectedShape/ShapeKind/DetectShape` | 从路径检测形状种类（Rect/RRect/Circle/Ellipse/Arc）供 SDF 快速路径 | 形状检测 | 🔗（Arc 检测仅测试） |
| `RenderPathStats`（方法：LogLine） | GPU/CPU 路由计数快照（P1 门禁诊断；`StagedWriteOverlaps` 于快照时从后端取，见 docs/RENDER_CPU_1PCT_REPLAY_PLAN.md §4） | 路由计数 | 🔗 |
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
| `Paint` + `NewPaint`（/SetBrush/SetStroke/SolidColor/EffectiveLineWidth/EffectiveDash/…） | 绘制状态聚合（画刷+描边+生效值，Surface/旧路径用） | 绘制状态 | 🔗 |
| `LineCap/LineJoin/FillRule` 及常量 | 线帽/线接/填充规则枚举 | 描边/填充规则 | ✅ |
| `Painter` / `SolidPainter` / `FuncPainter` / `PainterFromPaint` | 像素填色抽象（软件逐像素上色接口） | 像素填色 | 🧪 仅测试（软渲染未启用） |

---

## 6. 子包目录

### 6.1 render/text（包级导出 259，6a字距+6b真字重/轮廓兜底+第三方库委托+26.6 缓存键+A1 整形优先+彩色字形字体判定）
| 族 | 代表 API | 功能 | 精简 | 状态 |
|----|---------|------|------|------|
| 字体与文件 | `RegisterParser/FontSource/FontSourceID/DeriveFace/LoadDefaultFace/LoadDefaultFaceFor/LoadMultiFace/NewFontSourceFromFile/PurgeFontSourceCache/FontSourceCacheSnapshot/ResetFontSourceCacheStats/FontSourceCacheStats/ClearSystemFontPaths/FontRef/FontFamily/NewFontFamily/LoadFontFamily/WeightClassProvider/ErrEmptyFontData/ErrUnsupportedFont…`、`MultiFace`（`AtSize`/`WithHinting`/`WithWeight` 等导出方法，单遍派生收敛） | 字体解析器注册、字体源身份、尺寸+字重单入口派生、字体文件加载、多字体链重建（尺寸/渲染选项保留）、静态字重族（name ID 16 组族 + CSS 权重匹配）、系统字体路径清理、字体文件按 (绝对路径,TTC index,解析器) 进程级共享（Skia SkTypefaceCache/Flutter font manager 模型：同一文件只读盘解析一次，缓存源 `Close` 为 no-op，磁盘更新需 `PurgeFontSourceCache` 刷新；计数见 `FontSourceCacheSnapshot`）、字体错误 | 字体加载/解析 | 🔗 |
| 整形 | `Shape/ShapeForDraw/ShapeForDrawFallbacks/ShapedGlyph/RunAdvance/CaretXForCluster/HitTestCluster/…` | 文本整形（复杂文字/阿拉伯、泰文等）、字形序列与簇命中；`ShapeForDraw` 为绘制默认路径的整形优先入口（MultiFace 拆段 + cmap 显式回退计数） | 文本整形 | 🔗 |
| 分段 | `SegmentText/SegmentTextRTL/SegmentReuse/Segment/BuiltinSegmenter/DetectScript` | 双向/脚本分段（UAX#9）与单行击键增量前后缀复用（`SegmentReuse`：`ui/rendering` 单行行内复用经单行分段缓存调用，`render/text/segment_reuse_test.go` 等价锁 + 真窗 `ui_text_m5_anycase` 已验） | 脚本双向分段 | 🔗 |
| 绘制 | `Draw/DrawAliased/DrawWithEmoji/Measure/MeasureText/RuneAdvance` | 字形到目标图像的绘制（含别名/emoji）、文本度量；`RuneAdvance` 为单字形无分配推进（P7 5000 视口） | 字形绘制 | 🔗 |
| 布局/换行 | `LayoutText/LayoutOptions/Line/WrapText/WrapTextWithSpacing/ApplySpacing/ApplySpacingDir/SpacingExtra` | 段落布局（`LetterSpacing/WordSpacing` 字距：簇间隙n-1/词分隔符，末簇无拖尾；换行按含距宽度断行）+ 字距经第三方 `shaping.AddSpacing`（半分分布，`WhitespaceRuns` 已删）/宽度增量；2026-10-20 A15：`shapeSegments` 先按 `MultiFace.Runs` 切逐 face 连续段再整形（修 `MultiFace.Source()==nil` 时段落 0 字形），cluster 偏移 `seg.Start+part.start`，RTL+MultiFace 仍走旧单 face（见 `RENDER_FEATURE_COVERAGE` §6） | 段落布局/换行 | 🔗（6a：`ui/rendering` 字距布局/量宽/换行在用，`spacing_test.go` 端到端锁；第三方库委托后行为：词距按分隔符个数，tab 不计） |
| 字重 | `FontWeight/WeightThin..WeightBlack/WithWeight/FaceEmbolden/EmboldenOutline/Face.Embolden` | 字重选择（Flutter/CSS fontWeight）：FontFamily 成员优先选真实 Regular/SemiBold/Bold 切（`family.go`，name ID 16 组族，`actualWeight>=600` 不二次加粗）；可变 wght 轴走真轮廓（`resolveWeight`，显式 wght 优先）；无粗体兄弟的独立静态字体 ≥600 才走 `EmboldenOutline`（`ppem/24` 轮廓扩张，advance 不变；`ui/rendering` `SetFontWeight`+`DeriveFace` 单派生在用） | 字重/真字重/轮廓兜底 | 🔗（6b：`Embolden` 进 `Face` 接口，`FaceEmbolden` 仅 nil 安全分发；`MultiFace` 派生经 `faceOptionsExcept/rederiveComponent/deriveMultiFace` 单 helper；CPU AA/aliased/LCD + GPU AA/aliased/LCD/MSDF 全走轮廓加粗，`GlyphMaskFlagBold` 分键） |
| 度量/量化 | `Quantize/QuantizePoint/SubpixelMode/SubpixelConfig` | 子像素量化（LCD/AA 的次像素定位） | 子像素量化 | 🔗 |
| 缓存 | `ClearShapeResultCache/ClearMultiFaceRunsCache/ClearAutoHintCache/ClearFontScanFallbackCache/ResetShapeResultCacheStats/FixedSize` | 各缓存清理与统计复位（整形/多面/自动 hint/字体扫描）；`FixedSize` 把 ppem 转 **26.6 定点** `OutlineCacheKey.Size`（字段类型 `int16→int32`，2026-10-19 修分数磅值截断） | 缓存管理/键量化 | 🔗 |
| 光栅 | `RasterizeFT26/GlyphMaskRasterizer`（`RasterizeHinted/Aliased/LCD` + `RasterizeHintedBold/RasterizeAliasedBold/RasterizeLCDBold` 方法）/`GlyphMaskFlagAliased/GlyphMaskFlagLCD/GlyphMaskFlagLCDBGR/GlyphMaskFlagBold` | 轮廓点阵光栅化；Bold 三入口先在轮廓上扩张再光栅化（不再栅格后 mask 膨胀），字形掩码标志分键 | 字形光栅化 | 🔗 |
| 特性/角色 | `FontRole/FontFeature/TabularNums/AxisWeight/DefaultMultiFontRoleChain/UnicodeRange/RangeBasicLatin/IsCJK/IsPunctuation/IsWhitespace/GlyphsHaveColor…` | 字体角色链、OpenType 特性、Unicode 区间/分类判定、彩色字形按字体 `ColorFont.GlyphType` 判定（Skia「字体说算」口径） | 字体特性/分类 | 🔗 |
| 错误/变量 | `ErrCFF2Unsupported/ErrUnsupportedFontType/…`、`DefaultTabWidth` 等 | 字体/格式错误、制表符宽默认 | 错误/常量 | — |

> 完整 259 项请以 `go doc -all ./render/text` 为准（本表为族级归纳，改动文本 API 时在对应族补行；收敛项：`ui/rendering` 布局单入口 `BuildTextLayoutEx→WithSpacing(0,0)`、`effectiveFace→DeriveFace` 单派生，`BoundaryCache.Store/StorePicture→storeEntryLocked` 单写盘，`ui/scene` 回放经 `setOpFill/setOpStroke/drawRoundRectShape` 单设态；字距数学委托第三方 `shaping.AddSpacing`）。

### 6.2 render/scene（顶层 143）

> 状态口径：`render/internal/gpu`、`render/filters` 是生产消费；2026-10-21 新增直调真窗 `ui_render_base_scene` 把形状/场景/编码/渲染/构建/层裁剪/编码类型全族走一遍（GPU 场景渲染器 vs CPU 分块 A/B、`LayerCache`、`DamageTracker`），属**原子验证**，不算组件/2.5D/视频接线。

| 族 | 代表 API | 功能 | 精简 | 状态 |
|----|---------|------|------|------|
| 形状 | `Shape/PathBuilder`；Rect/RoundedRect/Circle/Ellipse/Line/Path/Polygon/RegularPolygon/Star/Arc/Pie/Transform/Composite Shape + New*Shape | 保留模式形状集合（矩形/圆角/圆/椭圆/线/路径/多边形/星/弧/扇/变换/组合） | 形状集合 | 🔗（render 内部 GPU 后端在用；ui/examples 零接线） |
| 场景 | `Scene/NewScene/NewImage/ScenePool` | 场景图累积 + 图像资源 + 池 | 场景图 | 🔗 + 🔌 `ui_render_base_scene` |
| 编码 | `Encoding/NewEncoding/Iterator/Decoder/NewDecoder/EncodingPool/GetEncoding/PutEncoding/DefaultPool`；`EncodeDrawTransform`（2026-10-21 新增） | 场景→不可变编码（可迭代/解码/缓存复用）；`EncodeDrawTransform` 跟踪流内变换状态，单位变换也会在需要时补写 identity 重置 | 场景编码 | 🔗 + 🔌 `ui_render_base_scene`（Decoder 全量回环·Flatten·Append） |
| 渲染 | `Renderer/NewRenderer/NewGPUSceneRenderer/CanUseGPU/TextRenderer/TextRendererPool/RenderStats` | CPU 分块渲染器与 GPU 场景渲染器、文本渲染、统计 | 场景渲染 | 🔗 + 🔌 `ui_render_base_scene`（双后端 A/B、`RenderDirty` 损伤） |
| 构建 | `SceneBuilder/NewSceneBuilder(From)/Tag/TaggedBounds/DamageTracker/LayerCache/CacheEntry/CacheStats/Filter/FilterType/FilterChain` | 声明式构建、标签、损伤、层缓存、滤镜链 | 构建/缓存 | 🔗 + 🔌 `ui_render_base_scene`（brushes 敏感 Hash、两色缓存、移动矩形损伤） |
| 层/裁剪 | `LayerKind/LayerState/LayerStack/ClipState/ClipStack` | 图层栈与裁剪栈（retained 层绘） | 层/裁剪栈 | 🔗 |
| 编码类型 | `Brush/SolidBrush/StrokeStyle/BlendMode/Affine/Rect/TextFlags/GlyphRunData/…` | 编码所需画刷/描边/混合/仿射/文本数据类型 | 编码数据类型 | 🔗 |

### 6.3 render/recording（顶层声明 98 / 符号名 180，2026-10-26 可录制掩码后按 §0 两口径重测：`go doc` = 类型 83 · 函数 12 · const 2 · var 1；`go/ast` 按符号名 = 类型 83 · 函数 33 · 常量 63 · 变量 1；历史：2026-10-24 S3 后 `go doc` 90；旧记 `go/ast` 164，2026-10-26 实测 HEAD 基线为 167（类型 77 · 函数 31 · 常量 58 · 变量 1），本次 +13 符号 = 180；2026-10-22 S2a 与 2026-10-23 S2b C1 两口径均为 82 / 155）

> **S2a 定位（2026-10-22）**：`Recorder` 是框架唯一的录制面——它完整实现 `render.Canvas` 的 100 个方法（2026-10-23 S2b C2b 由 89 补齐到 100；`var _ render.Canvas = (*Recorder)(nil)`），所以同一段绘制代码既能贴到即时的 `*render.Context` 上，也能录成显示列表。变换算子**原样入列**（`ConcatCommand` 存下调用时的相对矩阵，回放时乘到播放 CTM 上），不再像旧版那样每帧把相对变换折算成绝对 `prev⁻¹·(prev·op)`——折算每帧一次求逆，且末位舍入会让回放和实况差 1/255（GPU 光栅才暴露得出来，CPU 光栅把它掩盖了）。几何一律留在用户空间，设备缩放只在光栅边界折入。
>
> **哪些状态必须成命令（2026-10-23 C2b 的判据）**：绘制命令各自携带 brush/stroke 快照，所以「样式命令」在 `Playback` 里可以被吞；但 `TextMode`、`LCDLayout`、`AntiAlias`、裁剪算子、索引网格没有任何绘制命令会顺带描述它们，必须真成一条命令 + `Backend` 方法，否则回放到那一步的状态就丢了。图样是另一类：`PatternBrush` 原来只存 `ImageRef` + 一个正变换 + 自有 `RepeatMode`，回放要再求一次逆（不是逐位一致），现改存完整可描述数据（源矩形 + **采样逆矩阵** + 不透明度 + clamp），闭包型 `ColorAt` 图样则单独用 `CustomPatternBrush` 按引用带走——它无描述可导出（PDF/SVG 后端同样写不出），契约与 `text.Face` 一致：录制即只读。

> **2026-10-26 可录制掩码（maskops 收口）**：新增 5 条设备空间命令 `SetMaskCommand`/`ClearMaskCommand`/`InvertMaskCommand`/`ApplyMaskCommand`/`PushMaskLayerCommand` 与 5 个 `Cmd*`（`CmdSetMask`/`CmdClearMask`/`CmdInvertMask`/`CmdApplyMask`/`CmdPushMaskLayer`），可选回放能力 `MaskBackend`（5 方法），顶层函数 `MaskSkips()`/`ResetMaskSkips()`；`Recorder` 121→126（+5 掩码方法）、`CanvasBackend` 39→44（+5 方法 + `MaskBackend` 断言）、`Backend` 契约仍 38（设备空间掩码是可选能力，不强迫导出后端）。录制时快照 `Mask.Clone()`，回放每次再克隆；`CanvasBackend` 仅在回放目标尺寸等于录制尺寸时执行，不符计 `MaskSkips`，`PushMaskLayer` 退化普通层保层栈平衡，`End` 清理并重装活 Context 外层掩码。

| 族 | 代表 API | 功能 | 精简 | 状态 |
|----|---------|------|------|------|
| 录制 | `Recorder/NewRecorder/Recording`；`Recorder` 导出方法 **126** = `render.Canvas` 102 + 24 个录制面专有别名与出口（`Save`/`Restore`（= Push/Pop）、`SetColor`/`SetHexColor`/`SetFillRGB`/`SetFillRGBA`/`SetStrokeRGB`/`SetStrokeRGBA`/`SetFillStyle`/`SetStrokeStyle`/`StrokeRectangle`/`FillRectangle`/`FillStroke`/`InvertY`、`SetTransform`（**绝对**替换，仍记相对 delta；C2b 起它同时是 Canvas 方法）、`TransformPoint`、`SetFontSize`/`SetFontFamily`、**S3 新增 `DrawGPUTexture`**、**2026-10-26 新增 `SetMask`/`ClearMask`/`InvertMask`/`ApplyMask`/`PushMaskLayer`（`MaskCanvas` 侧）**、`FinishRecording`）；`Recording` 导出方法 10（`Width`/`Height`/`Commands`/`Resources`/`Playback` + **S2b C1 新增** `Bounds() image.Rectangle`、`HasVisiblePaint() bool`、`ImageBufs() []*render.ImageBuf` + **S2b C3 新增** `PaintOpCount()`、`NeedsCPUPassScratch()`） | 录制绘制命令 → 不可变 Recording；`FinishRecording` 同批出一趟命令流走查的结论：`Bounds` = 图层局部、与设备缩放无关的落点框（rect/path/图像/文字全计入，文字按录制当时的 face 量，描边按 Skia `compute_stroke_bounds` 口径外扩，裁剪只作超集不相交，零 alpha 不缩框，带 blur/drop-shadow 的整面滤镜直接报满幅），`HasVisiblePaint` = 这份内容能不能真 put 出像素（零 alpha 实色、无 stop 有 alpha 的渐变、纯 move-to 路径、零宽高矩形、face 缺失的文字、已 Dispose 的图像缓冲都算不可见），`ImageBufs` = 池内缓冲按插入序只读交出（供封包期钉住） | 命令录制 | 🔌 直调真窗 `ui_render_base_recording`（原子验证：逻辑 19/19 + 像素 17/17 + ink 1/1 + golden 627200px 零差）；生产消费者待 S2b 接线 |
| 命令 | 状态：`Save/Restore/Concat/SetClip/ClearClip/ClipRoundRect/ClipRect`；绘制：`FillPath/StrokePath/FillRect/StrokeRect/DrawImage/DrawImageNine/DrawImageQuad/DrawImageRounded/DrawImageCircular/DrawAtlas/DrawDepthSprites/DrawVertices/DrawGlyphs/DrawText/StrokeText/SetTextDecoration` + **S3 `DrawGPUTexture`**（`DrawGPUTextureCommand{View render.TextureView; X,Y float64; W,H int}`——句柄对录制不透明，回放在持有设备的线程绑定，零读回）；样式：`SetFillStyle/SetStrokeStyle/SetLineWidth/SetLineCap/SetLineJoin/SetMiterLimit/SetDash/SetFillRule/SetAntiAlias`；图层与特效：`PushLayer/PushLayerIsolated/PushBackdropLayer/PopLayer/ApplyBlur/ApplyColorMatrix/ApplyDropShadow/ApplyGrayscale` + **S2b C2b 五类** `ClipRectOp/ClipPathOp/SetTextMode/SetLCDLayout/DrawMesh`（共 **46** 类，`Cmd*` 常量同名）+ `FillRule/LineCap/LineJoin/Matrix`（= `render.X` 别名）`/Stroke` | 可回放的绘制/状态命令行（SkPicture 风格）；`DrawTextCommand`/`StrokeTextCommand` 带 `Font FontRef`；图像选项直接用 `render.DrawImageOptions`；`DrawGPUTexture` 补上「显示列表无法描述、但设备线程能绑定」的活纹理 | 录制命令 | 🔗 **生产接线（S3，2026-10-24）**：`ui/embedder/pipeline_app.go` 的 FullPaint 整帧 `recordFullFrame` → `presentRecordedFrame` 回放；`ui/rendering/live_only_draw.go` 的 `rendering.DrawGPUTexture` 在录制游标上落这条命令，示例 `ui_render_base_filterops` 每帧直绘的 GPU 滤镜结果因此不再丢；另有直调真窗 `ui_render_base_recording` 只算原子验证 |
| 掩码 | `MaskBackend` · `SetMaskCommand`/`ClearMaskCommand`/`InvertMaskCommand`/`ApplyMaskCommand`/`PushMaskLayerCommand` · `CmdSetMask`/`CmdClearMask`/`CmdInvertMask`/`CmdApplyMask`/`CmdPushMaskLayer` · `MaskSkips`/`ResetMaskSkips` | 设备空间掩码命令与可选回放能力；录制时快照 `Mask`，回放按目标尺寸门执行，跳过计数 | 设备空间掩码 | 🔌 直调真窗 `ui_render_base_maskops`（2026-10-26 两连上屏绿：`screen=0.0000%`/`pixel=18/18`/`live_only_skips=0`/`mask_skips=0`）；生产直调消费者仍为零 |
| 回放 | `Backend`（**38** 方法契约；2026-10-23 S2b C2b +6：`ClipRectOp`/`ClipPathOp`/`DrawMesh`/`SetTextMode`/`SetLCDLayout`/`SetAntiAlias`——这六项没有绘制命令顺带携带，命令流必须显式搬运；2026-10-24 S3 +1：**`ClipRoundRect`**——回放必须走活画布同一条原生圆角裁剪原语，转成路径会把光栅路线从「圆角覆盖」换成 CPU 裁剪掩码，既慢又是保真差异）/`BackendFactory/NewBackend/MustBackend/Register/Unregister/Backends/IsRegistered/Count`；`WriterBackend/FileBackend/PixmapBackend`；**`CanvasBackend`/`NewCanvasBackend`/`ErrNoCanvas`**（原名 `ContextBackend`/`NewContextBackend`/`ErrNoContext`，S2b C3 改名去兼容垫片：构造口由 `*render.Context` 放宽到 `render.Canvas`，`ApplyBrush/ApplyStroke/ToRenderBrush/TransformPoint` 为其实参助手）；**`GPUTextureBackend`**（S3 可选能力接口，单独一个 `DrawGPUTexture(view,x,y,w,h)`；只有够得着设备的回放端实现——活 Context 与 `CanvasBackend` 会转发，image/PDF/SVG 导出端没有纹理可采样就跳过该命令）；**`MaskBackend`**（2026-10-26 可选能力接口，5 个设备空间掩码方法；`CanvasBackend` 39→44 方法并带 `var _ MaskBackend` 断言，活 Context 与嵌套 Recorder 可执行，导出后端跳过并计 `MaskSkips`）；`CommandType/Command/InvalidRef`；`backends/raster` 内置 | 后端注册表与按名回放；`CanvasBackend` 把命令流打到**任一 `render.Canvas`** 上——目标为活 Context 就是生产回放主路，目标为 `*Recorder` 则命令折进外层显示列表（嵌套录制，Bounds/判空/资源池同趟跟着长，`TestCanvasBackendPlaybackIntoRecorderFoldsFacts`/`TestCanvasBackendPlaybackIntoRecorderKeepsGPUTexture` 锁死）；`backends/raster` 出 `image.RGBA` | 后端回放 | 🔗 **生产接线（S2b C3 + S3，2026-10-23/24）**：retained 窗回放 9 处（`ui/scene/composite.go:88`、`ui/scene/rasterize.go:72`、`ui/scene/textured.go:798,977,1560`、`ui/rendering/boundary_cache.go:348,351`、`ui/rendering/picture_box.go:107`、`render/recording/backends/raster/backend.go:95`）+ FullPaint 整帧回放 1 处（`ui/embedder/pipeline_app.go` 的 `presentRecordedFrame`）——光栅线程只执行不可变命令，不再遍历活树；另有直调真窗 `ui_render_base_recording` 与 `ui_wr_c2_retained_scene/main.go:291`、`ui_wr_r5_picture/main.go:182`（按口径只算原子验证）；PDF/SVG 后端仓外未接 |
| 资源 | `ResourcePool/FontRef/PathRef/BrushRef/ImageRef`（`AddImageBuf`/`GetImage`/`Images`，方法 15）+ 画刷（Solid/Linear/Radial/Sweep/Pattern/CustomPattern/gradient stop/ExtendMode，主包互转 `FromRenderBrush`/`ToRenderBrush` + **S2b C2b** `FromRenderPattern`/`ToRenderPattern`） | 命令引用池与画刷类型；`FontRef` 在回放时解析成 `text.Face`；`Images()` 按插入序只读交出缓冲，供封包期钉住；`PatternBrush` 自 C2b 起存完整可描述数据（缓冲指针 + 源矩形 + 采样逆矩阵 + 不透明度 + clamp，方法 `SetSourceRegion`/`SetInverse`/`SetOpacity`/`SetClamp`/`Source()`），闭包型图样走 `CustomPatternBrush`（`NewCustomPatternBrush`）按引用带走、录制即只读；**删** `RepeatMode` 与 `RepeatNone/RepeatX/RepeatY/RepeatBoth`（重复语义由源矩形 + clamp 承担，不留第二套真值） | 资源池/画刷 | 🔌（真窗原子验证；生产消费者仍为零） |

> **golden 重出记录（2026-10-22，用户批准「重出 golden，并在总账记明原因」）**：`examples/ui_render_base_recording/testdata/showcase_recording_base.png` 已用与实况逐位一致的新快照替换。旧基线把 **HEAD 回放路径的末位误差录进了基线**——GPU 光栅下卡片 D 旋转圆内一点（窗口坐标 1036,494）实况 19、旧基线 18，即 replay≠live 差 1/255；`ConcatCommand` 原样携带变换算子后该点与实况一致，比对从 1 个像素之差（627200px 里 1px，gate 按 `%.4f%%` 打印为 0.0002%）降到 `0.0000%` 两连。基线本身不再有鉴别力，鉴别力改由 `render/s3c_m3_gpu_gate_test.go` 的 `TestS3c_M3_ReplayParityOnGPU` 守住：CPU 光栅会掩盖这类末位差，该测在 GPU 后端上逐像素比实况 vs 回放，且 GPU 掉回 CPU 时 SKIP 并报原因，不算假绿。
>
> **重出前后的逐像素账（2026-10-22 对 HEAD 基线全窗口比对，1200×800）**：共 60 px 不同——**1 px 在 (1036,494)**，落在卡片 D 内，也就是上面那个回放末位差点（18→19），在 golden 比对矩形之内；**其余 59 px 全在 y 741–749 的 LiveHUD 数字带**（header 1200×48 + legend 260×656 + 4 张 430×232 卡片之外的逐帧指标行，本就不进比对）。同批 `showcase_recording.png` 输出件从 HEAD 提交的 1868×1016（别台 HiDPI 机跑出来的产物，与 1× 基线尺寸不符，换机跑会直接 `size mismatch`）归一回 1200×800，与基线同尺寸。
>
> **后端口径（2026-10-22 实测）**：native（wgpu-native）臂 `gpu_ops=5 cpu_fallback_ops=0` PASS；同测在 `GPUI_BACKEND=go` 下也 PASS（0.26s）。**限界如实**：离屏 `render.NewContext` 的光栅器由 `render/gpu` 注册的加速器决定，`GPUI_BACKEND` 真正生效处是建窗/建链（`render/backend.go:ResolveBackend` + `present_construct.go`），本刀未单独证明 GL 臂在离屏上下文里确实接管了光栅——要证这一格需给纯 GL 离屏路补一条身份断言，属后续项，不拿「设了环境变量还绿」冒充双后端已验。

### 6.4 render/svg（顶层 15）
| API | 功能 | 精简 | 状态 |
|-----|------|------|------|
| `Render(data,w,h)/RenderWithColor(data,w,h,c)` | 解析图标风 SVG 数据并栅格化为 RGBA 图（可选主题色） | SVG→图像 | 🔌 直调真窗 `ui_render_base_svg`（2026-10-21）；生产消费者仍为零 |
| `Parse(data)/Document/ViewBox/Attrs/Element`（具体类型：`PathElement/CircleElement/RectElement/EllipseElement/LineElement/PolygonElement/PolylineElement/GroupElement`） | SVG 文档解析与元素模型；根 width/height 百分比按 viewBox 解析，子几何非法值报错 | SVG 解析 | 🔌 直调真窗 `ui_render_base_svg`；无生产消费者 |

### 6.5 render/filters（0 顶层）
`init()` 副作用：把 internal/filter 的 blur/colormatrix/shadow/grayscale/invert 注册进 `render.RegisterFilterOps`，使 `Context.Apply*` 可用。**精简**：滤镜 op 注册表（DC.ApplyBlur 等的开关）。**状态：🔗**（经 `render/gpu` 的 `_ import` 自动激活）。

### 6.6 render/gpu（顶层 18）
| API | 功能 | 精简 | 状态 |
|-----|------|------|------|
| `SetDeviceProvider(provider)/ResetAccelerator/AbandonDevice/PurgeSurfaceResources` | 注入共享 GPU 设备、重建加速器、设备放弃、清理表面资源 | 设备生命周期 | 🔗 examples/ggcanvas 注入；**ui/embedder 生产未调 ⚠️** |
| `AdapterPolicy/ResolveAdapterPolicy/RequestAdapterWithPolicy/DeviceDescriptor*/DeviceDescriptorLowVRAM` | 适配器/设备策略选择与描述符（2026-09-05 已搬入 render 主包，本包不再导出；见 §3.7） | 适配器策略（已搬迁） | 🔗 主包 NewPresentTarget 在用 |
| `SurfaceLifecycle/NoteTextureOOM/TextureOOMCount/ResetTextureOOMCount/ResolveSurfaceLifecycle` | 表面生命周期（正常/清理/重建）与纹理 OOM 钩子 | 表面/OOM | 🔗 |
| init() 副作用：注册 SDF GPU 加速器 + AdaptiveFiller + filters + OOM/lifecycle 钩子 | 一次 import 激活全部 GPU 加速与滤镜 | 加速注册 | ✅ 全部真窗 blank import 接线 |

---

## 7. 状态总表（接线 × 未接线）

### 7.1 有生产接线（✅/🔗）
Present/帧/呈现链路（frame/present/present_target → ui/embedder）、Context 绘制族、Brush/三种渐变、文本绘制（DrawString 族经 ui/rendering；DrawShapedColorGlyphs 🔗 彩色段经 ui/rendering→GPU 颜色图集/CPU 兜底（GPU 真窗已验：ui_text_m5_anycase 10s 回退 0、B 区真彩，证据 /tmp/m5_verify/））、图像（GPU QueueImageDraw，Bicubic 走 GPU 4×4 卷积变体、GPU 失败回退 CPU）、Mask 上传、Layer（GPU RT）、滤镜 op（经 render/gpu 副作用）、SDF/CoverageFiller/AdaptiveFiller、Pixmap、路径基础 API、损伤跟踪、共享编码器。`PresentTarget.SetVsync`（2026-08-19）🔗 embedder resize 风暴期切 Mailbox/Immediate，风暴平静后回 Fifo（内容不跟手修复）；初始 Fifo 排队偏好（2026-08-26 修3：显示周期学习 + Wayland Fifo 化，见 docs/ENGINE_FRAME_PRESENT_STANDARD.md §9）。适配器策略（2026-09-05，多窗 1.2）🔗 `NewPresentTarget` 经 `RequestAdapterWithPolicy(inst, surf, ResolveAdapterPolicy())` 选卡（render/present_target.go）+ `DeviceDescriptorForAdapter` 按卡型取描述符：混搭机默认核显（真窗 ui_l1_blank，`nvidia-smi` 不见进程，DBG adapter=Intel IntegratedGPU）、`GPUI_POWER=high` 回独显（同窗落 NVIDIA DiscreteGPU，`nvidia-smi` 可见）；`PresentTarget.GPUBackend()` 上报 discrete/integrated/software 供 1.3 降级链复用。降级链（2026-09-05，多窗 1.3）🔗 建窗按 resolved→low→software 换卡重试（仅 OOM 触发，`render.IsGPUOutOfMemory` 统一判定；压力真窗 R6+m5 各占独显 321MB 后 accept 要独显触发 `present level "high" out of memory` 一次、落核显 `gpu_fallbacks=1`、exit 0、快照出字正常）；建纹理 OOM 经 flush→purge（`PurgeEvictables`：image-cache + picture-texture 注册）→MSAA 降级重试，日志限 1 条/秒；必需纹理双败置 textureSet 失效位同尺寸 fail-fast；embedder 连续 3 帧 OOM 直接报人话退出（`notePresentErr`，禁黑窗禁崩溃）。字形图集页未进 purge（CPU 侧页，purge 不 relief 显存且会断 GPU 重传——结论见多窗计划 1.3 修订）。纹理源注册族（2026-10-24 S2b C4）🔗 生产链路：`render.NewVideoBridge` 注册（`render/video_bridge.go:58`）、`VideoBridge.Close` 注销（`:394`）、`ui/scene/composite.go:281` 按 id 解析并绘制、`ui/rendering/video_direct_node.go:153` 读逐视图账本；`LiveTextureSources` 仅测试（拆除断言）。

### 7.2 已实现但无生产消费者（🔌 未接线）
| 功能 | 证据 |
|------|------|
| `render/svg` 整包（SVG 图标渲染） | 已建直调真窗 `ui_render_base_svg`；生产仍无 import |
| `render/recording` 整包（录制回放） | 仅测试/示例 + 直调真窗 `ui_render_base_recording`（2026-10-21，原子验证：逻辑 19 + 像素 17 + 墨量 1 + Golden 627200px 零差）；PDF/SVG 后端仓外；生产消费者仍为零——**S2b 正在将 `ui/rendering` 的光栅回放接到 `NewContextBackend`，接线前此行不得升 🔗**——**该前置条件已于 S2b C3（2026-10-23）达成，本行升 🔗 生产接线**：`ui/scene/composite.go:88`、`ui/scene/rasterize.go:72`、`ui/scene/textured.go:798,977,1560`、`ui/rendering/boundary_cache.go:348,351`、`ui/rendering/picture_box.go:107`、`render/recording/backends/raster/backend.go:95` 共 9 处经 `recording.NewCanvasBackend` 回放，`ui/scene.Picture`/`PictureRecorder`/`PictureOp` 整套退役（全仓符号零命中），retained 窗的光栅线程不再执行任何活闭包。2026-10-22 S2a 达成 `render.Canvas` 89/89 录制面（详见 §0/§6.3）；同批按测量结论重出该窗 golden 基线（旧基线录进了 HEAD 回放的 GPU 末位误差）。2026-10-23 S2b C1 补 `Bounds`/`HasVisiblePaint`/`ImageBufs`（原子验证：`render/recording/bounds_test.go` 20 个测试 / 含图像落点表 10 子例共 29 例，逐文件 `-race` 全绿——几何侧含 CTM 映射、旋转取变换后四角 85..115、Save/Restore 隔离、描边外扩、退化几何与 NaN 跳过、裁剪不相交、整面滤镜报满幅，图像侧含 10 型落点表；文字侧用真字体（DejaVu/Liberation 探测，缺字则 `t.Skipf` 报原因）逐位对拍：右边界对齐 `face.Advance`、实况回放的实际着墨矩形被报出的 bounds 完全包住；`-race` 逐文件跑，禁全量）；该笔当时的口径是「生产消费者仍为零、接线待 S2b C3」，C3 已达成（见本行开头）。2026-10-23 S2b C2b 把 Canvas 补齐到 100 方法（`Recorder` 108→118、命令 40→45 类、`Backend` 契约 31→37）：新增七族实况 vs 回放逐位对拍 `render/recording/backends/raster/backend_c2b_parity_test.go`（图样/描边/裁剪算子/网格/文字状态/抗锯齿/绝对变换），逐条 mutation 反证（任一侧改坏对拍必红，例：ClipPathOp 二次变换 → 730/24000 px 差）；`-race` 逐文件跑 `render/recording`（9 ok + 1 skip）与 `render/recording/backends/raster`（6 ok）全绿。**S2b C3 接线证据（2026-10-23）**：回放口 8 处见本行开头；`-race` 逐文件（禁全量）`render/recording` 9 文件 + `ui/scene` 24 文件（无 GPU 环境绿，GPU 专项另跑）+ `ui/rendering` 77 ok（4 条既有红，属计时门与 1-LSB 混合色，见 `ENGINE_THREAD_T_PLAN.md` §18.2 口径）+ `ui/embedder`/`ui/raster` 18 ok；新增 `ui/rendering/layer_build_parity_test.go` 两条坐标口径对拍——① OnPaint 带 `Push/Translate/Rotate/Scale` + 裁剪链时，实况 `root.Paint` 与 `BuildFramePacket`→`CompositeFramePacket` 回放**逐位一致**；② OnPaint 内回放一段 recording 必须折进图层自身的显示列表（`HasVisiblePaint` 为真、像素等值、取样点颜色命中），两条都做变异反证（把 origin 归一或把折叠任一侧改坏即红）。`render/recording/backend_context_test.go` 另补折叠两测（派生事实并集、零 alpha 不可见性传递）。**S3 接线（2026-10-24）**：FullPaint 帧也改走「UI 线程整帧录制 → 光栅只回放」，回放口 +1 = `ui/embedder/pipeline_app.go` 的 `presentRecordedFrame`（`recordFullFrame` 产 `*recording.Recording`，光栅任务里只剩 force 的 Clear+`MarkFullRedraw` 与非 force 的 `PresentWithAuto`）；回放面补 `ClipRoundRect`（`Backend` 37→38，原生转发回活画布同一原语——转路径会触发 CPU 裁剪掩码，实测 `ui_render_base_cliprrect` fps 54.3→16.8，修复后 54.97）与可选能力 `GPUTextureBackend` + `DrawGPUTextureCommand`（设备纹理零读回，导出后端跳过）；`ui/rendering/live_only_draw.go` 的 `rendering.DrawGPUTexture` 改为「目标有能力就录制」。**关门证据**：`ui_wr_r3_boundary -race` DATA RACE 56（S3 前基线）→ 79（中间态）→ **0**，非 race 两连 `skip=3602/rr=1206`（与基线工作树逐位相同，缓存语义零变化）、像素 33/33、Golden 0.0000%/821184px；`ui_render_base_cliprrect`/`ui_render_base_filterops`/`ui_render_base_recording` 与 15+ 扇 retained/FullPaint 窗复跑绿（`ui_render_base_recording` 基线因回放改走原生圆角后的 AA 边缘位移重出，339px 全在卡 B 圆角边、最大 27/255，重出后 0.0000%） |
| 路径布尔 `BooleanPath/PathOp*` | 仅测试（s3c_m3_residual_gate_test 门禁 + path_boolean_test 值级；2026-10-19 重写为 Clipper 扫描线）+ 直调真窗 `ui_render_base_pathbool`（2026-10-20，**原子验证**：逻辑 10 + 像素 18 + golden 627200px 逐位零差 + 抓屏同区 `0.0000%`；同批修半透明 stencil path 重复预乘，pin `TestMakeCoverUniformDoesNotPremultiplyTwice` + GPU `TestRenderSessionStencilPremultipliedColor`；输出为折线，曲线按 0.1px 展平） |
| 裁剪运算 `ClipOpDifference/Replace`（ClipRectOp/ClipPathOp） | 仅测试（p12_clip_mask_gpu_test）+ 直调真窗 `ui_render_base_clipop`（2026-10-19，**原子验证**；同批修 4 缺陷，见 §3.2 行与 `RENDER_FEATURE_COVERAGE.md` §11） |
| 掩码写路径 `SetMask/ClearMask/GetMask/InvertMask/ApplyMask` + `PushMaskLayer`（+`Mask` 构造类型） | 仅测试（mask_test / context_layer_test / layer_rt_clear_test / recording/mask_test 六测）+ 直调真窗 `ui_render_base_maskops`（2026-10-19，**原子验证**：逻辑 14 + 像素 18 + golden 627200px 逐位零差 + 抓屏 vs 快照同区对照；同批修掩码 pop 帧中 CPU 材料化导致的呈现断裂 + 嵌套隔离层 blit-only 离屏清屏不透明黑。**2026-10-26 可录制掩码**：新增 `MaskCanvas` 侧接口 + `recording.MaskBackend`/5 命令，`CanvasBackend` 设备尺寸门/`MaskSkips`，`PaintContext.MaskCanvas()` 正口；maskops 两连 `screen=0.0000%`/`pixel=18/18`/`golden=0.0000%`/`live_only_skips=0`/`mask_skips=0`。生产直调消费者仍为零，见本行 §3.3、`RENDER_FEATURE_COVERAGE.md` §11 与本窗 README） |
| Path 高级造型 `Trim/WithCorners/Discrete` | 仅测试（2026-10-19 修 `Discrete` 恒单侧偏移 + 2× 虚线周期，值级契约 `pathops_subpath_test.go`；直调真窗 `ui_render_base_pathshape`） |
| `SetDither`（m4_extensions） | 仅测试（`dither_contract_test.go` 4 例 CPU+GPU）+ 直调真窗 `ui_render_base_dither`（2026-10-20，**原子验证**：逻辑 16 + 像素 29 + golden 627200px 逐位零差 + 抓屏 vs 快照同区 `diff=0.0000%`；3 项限界：整张 pixmap / 重复读回累加 / 不影响 Present 主路，见 `RENDER_FEATURE_COVERAGE` §6） |
| `Pattern/ImagePattern` 旧图案接口（SetFillPattern 等） | 仅测试/桥接；生产走 Brush |
| 显存账本 `VramBudgetMB/VramLiveBytes/VramLiveCount/VramPeakBytes/VramPressureHigh` + 共享设备 `RecoverSharedDevice/SharedDeviceGeneration/SharedWindowCount` + 图片缓存 `PictureCacheFairMax` + 帧耗时 `FrameAcquireWaitMs/FrameFlushMs/FramePresentWaitMs` + 顶点扩展 `DrawVerticesEx/DrawMeshEx/DrawAtlasEx/DrawDepthSprites` 及 Vert/Atlas/Depth 选项结果错误族 + `OOMExit.Note/RunErr`（2026-10-07 T6 补录：代码早有、总账漏记，T1 拆分搬家后机检喊出，本次按归属补 §3.7/§3.9） | 代码存量，机检 `go run ./scripts/apidoc` 已绿 |
| `Painter/SolidPainter/FuncPainter/PainterFromPaint` | 仅测试 |
| 文本描边/路径 `StrokeString/StrokeStringAnchored/TextPath/TextOutlinePath` | 仅测试 + 直调真窗 `ui_render_base_textstroke`（2026-10-19，**原子验证**：逻辑 14 + 像素 21 + golden 627200px 逐位零差 + 抓屏 vs 快照同区对照；同批修描边损伤不按线宽外扩 + 轮廓缓存 key 截断分数磅值，pin `render/stroke_damage_bounds_test.go`）。**`TextOutlinePath`（2026-10-25，清单 #5）**：纯函数版轮廓路径（face+shaper+每次独立 extractor，无线程/GPU 依属），`Context.TextPath` 的可录制孪生——S3 整帧录制下 TextPath 型内容（textstroke 卡 C/D）经它录成 `FillPath/StrokePath` 命令；`StrokeTextCommand` 回放同步改真描边（旧 DrawText 填充近似拆除）、`DrawStringAnchored/StrokeStringAnchored` 录制期锚点换算（旧 simplification 丢锚点）。 |
| 文本低频 `MeasureMultilineString/WordWrap/SetTextMode/SetLCDLayout` | 仅测试 + 直调真窗 `ui_render_base_textlow`（2026-10-20，**原子验证**：逻辑 16 + 像素 28 + golden 627200px 逐位零差 + 抓屏 vs 快照同区 `diff=0.0000%`；同批修 MSDF 轮廓方向 / WordWrap 行尾空白与 CRLF 偏移 / 多行可视块高度 / `SetLCDLayout` 清图集致灰度+LCD 同帧花屏 / Aliased 文本损伤记账 5 缺陷，pin `render/glyphmask_lcd_mix_test.go` + `render/internal/gpu/glyph_mask_lcd_layout_cache_test.go` + `render/text/wrap_test.go` / `render/text_wrap_test.go`） |
| `DrawShapedGlyphs` | 🔗 生产在用（6 处）：`ui/rendering/text.go:988`（RenderText.Paint 批量主路）· `:1077`（复合分区回退路）· `ui/rendering/text_picture.go:205`（分区提交）· `:232`（彩色段分区）· `:244`（偏移重提交）· `ui/scene/picture.go:225`（录制回放 OpDrawShapedGlyphs）· 另 `render/text.go:158`（描边字形回退）与 `render/scene/gpu_renderer.go:247`（Scene→GPU 解析首选） |
| `DrawVertices/DrawVerticesEx/DrawMesh/DrawMeshEx`（顶点/网格族） | 无生产调用点（仅测试 `p1_capability_matrix_closers_test` + `vertices_mesh_contract_test.go` / `vertices_mesh_gpu_test.go` + 直调真窗 `ui_render_base_verticesmesh`，2026-10-20 **原子验证**：逻辑 19 + 像素 29 + golden 627200px 逐位零差 + 抓屏 vs 快照同区 `diff=0.0000%`；同批修短索引误画 / 越界索引串读邻网格 / 6B 索引上传不对齐 3 缺陷） |
| 内容滤镜 `ApplyInvert/ApplyBlurXY/ApplyImageFilterGraph/GPUFilterTexture` | 仅测试/真窗（`filter_ops_contract_test.go` 3 例 + `filter_ops_gpu_test.go` 5 例 + 直调真窗 `ui_render_base_filterops`，2026-10-20 **原子验证**：逻辑 16 + 像素 26 + golden 627200px 逐位零差 + 抓屏 vs 快照 `0.0000%`；同批修 GPU 反色预乘 + 零读回滤镜 RT 丢 CPU 底；3 项限界见 `RENDER_FEATURE_COVERAGE` §6） |
| 杂项绘制 `DrawShadow/DrawColor/DrawPaint/ClipBounds/DrawParagraph` | 仅测试/真窗（`context_misc_draw_test.go` CPU 契约 + `context_misc_draw_gpu_test.go` 5 API/连续高混/隔离父层内高混/效果面高级着色 GPU 回归 + 直调真窗 `ui_render_base_miscdraw`，2026-10-20 **原子验证**：逻辑 14 + 像素 17 + golden 737760px 逐位零差 + 抓屏 vs 快照 `0.0000%`；同批修顺序高混层丢合成 / MultiFace 段落 0 字形 / ClipBounds 路径全画布 / HiDPI 图层逻辑尺寸 / 模糊投影未标全量合成 / CPU 模式仍建 GPU 上下文 6 缺陷；3 项限界见 `RENDER_FEATURE_COVERAGE` §6） |
| ui/rendering 滤镜 facade（ApplyGrayscale 等 FF-*） | 无生产调用方（需 blank-import filters，由 gpu 包侧效应顶替） |
| 视频直传地基 P0（2026-09-30：`VideoBackendCaps/QueryVideoBackend/BorrowVideoBackend/VideoSlot/VideoTexturePool(+Acquire/Release/Stats/Close/VideoPoolStats)/RecordVideoFallback/VideoFallbackTotal`，render/video_direct.go） | 仅测试（video_direct_test.go 四用例绿：双拷贝模式查询/无窗借用/池借还零泄漏零淘汰/回落记 cpu_fallback_ops）；P2 接直传后转生产 |
| 视频直传 P2+P3-A（2026-10-02：`VideoDrawOptions/VideoUploadTotal/Context.DrawVideoFrame/DrawVideoSlot/VideoBridge(+New/Ensure/Show/ShowSeq/UploadFrame/DrawCurrent/Stats/Close/BridgeStats)`；P3-A 加 `VideoPlaneSlot/VideoPlanePool(+Acquire/Release/Stats/Close/AcquireForFrame/UploadPlanes)/Context.DrawVideoPlanes/VideoBridge.UploadPlanesFrame`，render/video_direct.go + context_image.go；layer 4 加 YUV 转色管线 `render/internal/gpu/shaders/video_yuv.wgsl` + `video_yuv_pipeline.go`，一份 WGSL 两后端通用；layer 5 窗口只调节点单新口 `RenderVideo.UploadPlanes`（平面走 GPU 转色，奇数/异形帧走桥老口兜底记数，`PlanesLast` 作证） | 生产在用（17 扇窗 tick 上传 + paint 绘制：上传+回落==帧数；render 单测绿含平面池借还/紧凑与垫行上传/三原色±1/解码器对拍 mean≤2；YUV 着色器 WebGPU 真编译绿 + GLES 翻译双纹理映射绿，无会话走 CPU 转回落；2K 真窗 80/80 上传零回落首尾亮、yuv_ready=1，p95 随整机抖动待压） |
| `ui.SetMask` widget API（antd 文献提及） | 未实现（非 render 层） |

### 7.3 ⚠️ 半成品 / GPU 未生效（重点：真窗实测）
| 功能 | 现象 | 证据与建议动作 |
|------|------|------|
| **任意路径裁剪 `Clip()`/`ClipPreserve()`（= ui `PushClipPath`）** | ~~GPU 真窗下裁剪不生效（画穿整个格）~~ → **已修复（2026-08-16）**：根因=`render/internal/gpu` `DepthClipPipeline.ensurePipeline()`（编译 shader+uniformBGL+stencil-fill/cover 两管线）在生产路径从未被调用，`uniformBGL` 恒 nil，`BuildClipResources` 的 `CreateBindGroup` 全败（`layout is nil`）→ 深度裁剪从未生效。修复=`ensureStagePipelines(needDepthClip)` 补调 `ensurePipeline()`（render_session.go）。**复测（真窗 `examples/render_clipping`，2026-08-16）**：圆外泄漏 100%→0.6%（仅边缘 AA）、星外条纹 744→0、复杂路径 clip 边界 AA 级残余；新回归单测 `TestDepthClip_ProductionPath_NoManualEnsure`。→ **边缘 AA 补全（2026-08-16）**：无 MSAA（sampleCount==1）下 stencil+depth 两阶段是二值掩码，clip 边界像素被深度测试直接拒绝 → 圆外/星形内边锯齿。修复（Skia kCoverage / Flutter ClipMask 模式）：① `clipPathCoverageMask` 用 CPU 分析 AA（`render/internal/clip.MaskClipper` 同源）生成全帧 R8 coverage mask；② `DepthClipPipeline.BuildClipMask` 上传 mask 纹理 + bind group（L.06 @group(2) 通道）；③ Phase1.5 `stencilExpandPipeline` 用 bandVerts 外带 stencil Replace(1) 扩充深度通过区到 mask 渐变带；④ `recordGroupDraws` 内容管线（SDF/convex/stencil-cover）绑 per-group clip mask 调制 alpha。**复测（真窗 `examples/render_clipping`，2026-08-16）**：ex1 圆边缘 AA 过渡像素 0→418（CPU 367）、ex3 星形外带 0→25（CPU 27）、ex4 嵌套圆 clip 0→276（CPU 332），GPU 与 CPU 边缘渐变一致；新增单测 `TestClipPathCoverageMask_AnalyticAA`/`TestBuildClipMask_SampleCount1`。 |
| `render/gpu` 设备生命周期 API 未进 ui/embedder | SetDeviceProvider/AbandonDevice 仅示例注入 | 真窗换设备/恢复场景未覆盖，属预留 |

### 7.4 仅测试/桥接（🧪）
见 §7.2 列表（布尔/SDF 独立注册/图案/画刷版本兼容等），不重复。

### 7.5 重复/重叠 API 对照（2026-08-15 核对）

> 同一能力存在多个入口时在此登记。「重复」分三类：**D= 同包双入口实质重复**，**W= 分层包装/封装（功能重叠但保留合理）**，**C= 平行体系/独立副本（跨包，多为未接线）**。**新增 API 前先查本表，能复用就复用**。

| # | 能力 | 重复成员 | 类型 | 说明 / 建议 |
|---|------|---------|------|------------|
| 1 | 纯色上色 | `Context.SetRGB/SetRGBA/SetHexColor/SetColor` ↔ `Solid/SolidRGB/SolidRGBA/SolidHex` + `SetFillBrush/SetStrokeBrush` | D | 旧 fogleman/gg 风格（Set* 内部生成实色 Solid）与画刷体系并存。新代码走 Solid+Set*Brush；Set* 保留兼容。 |
| 2 | 渐变 | `CustomBrush.LinearGradient/RadialGradient`（函数式）↔ `LinearGradientBrush/RadialGradientBrush`（色标式） | D | 两种实现机制交付同一视觉（两端线性 / 单位圆径向）。径向还多 `SetFocus`、色标 ∞；CustomBrush 仅两 stop。生产已验证走 Brush 数据式。 |
| 3 | 裁剪 | `Context.ClipRect` ↔ `ClipRectOp(o=ClipOpIntersect)`；`Context.Clip` ↔ ui `PaintContext.PushClipPath`；`scene.ClipState/ClipStack` | D+W | ClipRectOp(Intersect) 是 ClipRect 的带运算泛化（默认相交=同义）；ui PushClipPath 是 Context.Clip 的封装（历史 GPU 画穿已修复，见 §7.3 2026-08-16 条目）；scene 为保留模式第三套。 |
| 4 | 画图体系 | `Context`（即时）↔ `scene.Scene`（保留）↔ `recording.Recorder`（录制） | C→**W（2026-10-22 S2a）** | 三套都含 Fill/Stroke/Clip/Image 语义。**仅 Context 接入 ui 产线**；scene 只被 render 内部 GPU 后端消费（2026-10-21 另有直调真窗 `ui_render_base_scene`，原子验证）；recording 生产无消费者（2026-10-21 另有直调真窗 `ui_render_base_recording`，原子验证；§7.2）。**S2a 后 Context 与 Recorder 不再是平行体系：两者同实现 `render.Canvas` 100 方法，绘制代码一份通吃，剩下的只是「即时光栅」与「命令流」两种落点（W 类分层包装）**；scene 仍是独立保留模式副本。体系级细节见 §3/§6。 |
| 5 | 形状轮廓 | `Path.Rectangle/Circle/Ellipse/Arc/RoundedRectangle` ↔ `PathBuilder.Rect/RoundRect/Circle/Ellipse/Polygon/Star` ↔ `Context.Draw*`（DrawRectangle/DrawCircle/…） | D | 前三者都能往一条路径上追加矩形/圆/椭圆，**Path 与 PathBuilder 是两套独立实现、无桥接**（如 PathBuilder.Rect 自己 MoveTo/LineTo/Close，不调 Path.Rectangle）；Context.Draw* 是追加到当前路径的第三处。 |
| 6 | 文本 | `Context.DrawString/MeasureString/DrawStringWrapped` ↔ `text.Draw/Measure/MeasureText` | W | Context 方法是 text 包的封装（ui 链路走 Context 层）；text 包直连 draw.Image 供底层/工具用。 |
| 7 | 呈现 | `PresentTarget.PresentWith/PresentWithAuto` ↔ `Context.PresentFrame(+/Damage/DamageRects)` ↔ `frame.PresentFrameAuto/PresentFrameFull/PlanFramePresent` | W | 三层入口：PresentTarget 高层（draw 闭包）→ Context 低层（view+present 回调）→ frame 策略（选路径）。PresentWithAuto 与 PresentFrameAuto 决策语义重叠。**2026-10-21：GL 后端已把 `FrameDamage` 接到 EGL `SetDamageRegionKHR` + `SwapBuffersWithDamage`（局部提交，原子验证 `TestPresentDamageX11`）；wgpu-native 仍全量。** |
| 8 | 滤镜 | `Context.ApplyBlur/ApplyColorMatrix/…/ApplyImageFilterGraph` ↔ `scene.FilterChain/FilterType` | W | 入口不同，底层共用 internal/filter 实现；scene 侧仅保留模式用。 |
| 9 | 像素缓冲 | `Pixmap` ↔ `ImageBuf` | W | 两种像素缓冲可互转（Pixmap.ToImage/Image()、ImageBufFromImage）；Context 内部持 Pixmap，ImageBuf 用于 GPU/图像绘制。 |
| 10 | 编解码导出 | `Context.Image()/SavePNG()/EncodePNG()/EncodeJPEG()` ↔ `Pixmap.ToImage()/SavePNG()/EncodePNG()/EncodeJPEG()` | W | Context 方法即包装底层 pixmap 同名方法（Context 内部持 Pixmap）。 |
| 11 | 画笔/几何副本 | `recording.SolidBrush/Linear/Radial/Sweep/Pattern + Rect` ↔ 主包 `Brush` 族/`Rect`（`Matrix/FillRule/LineCap/LineJoin` 已于 S2a 收敛） | C | **2026-10-22 S2a 半收敛**：`Matrix`/`FillRule`/`LineCap`/`LineJoin` 改为 `= render.X` 别名并再导出成员常量，图像选项直接用 `render.DrawImageOptions`（原 `ImageOptions`/`InterpolationMode` 删除），几何与状态枚举不再有两份真值；仍存副本 = 画刷五型 + `recording.Rect`（命令按值存、与主包解耦，PDF/SVG 后端扩展目标明确）。**2026-10-23 S2b C1 命名债清零**：顶层 `BrushFromGG`/`BrushToGG` 改 `FromRenderBrush`/`ToRenderBrush`（与主包 `render.Brush` 互转，名字不再指 fogleman/gg），内部三构造改 `linearGradientFromRender`/`radialGradientFromRender`/`sweepGradientFromRender`（非导出，不入册），`Recorder.SetFillRuleGG`/`SetLineCapGG`/`SetLineJoinGG` **直接删除**而非改名——三型枚举自 S2a 起就是 `= render.X` 别名，这三个方法是 `SetFillRule`/`SetLineCap`/`SetLineJoin` 的字面重复品（同参同义），删后 `Recorder` 导出方法 111→108；全包调用方与单测同批切完，无兼容垫片。改动主包 Brush 不影响 recording 副本。 |
| 12 | SDF 覆盖 | `sdf.SDFFilledCircleCoverage/…`（4 函数）↔ `SDFAccelerator.FillShape/StrokeShape` | W | 独立函数供像素级/测试；加速器实现包装同语义。 |

### 7.5.1 收敛执行记录（2026-08-15）

> 收敛依据 = 最准确且对齐 skia/flutter（判据：官方注释声明 / skia 参数集与边界对照 / 生产消费者 / 活跃度）× 淘汰方扩展性（有明确扩展目标的保留，如 recording→PDF/SVG、ColorFunc→程序化纹理）。

| # | 动作 | 执行内容 | 验证 |
|---|------|---------|------|
| 1 | 统一语义（已收敛） | `SetRGB/SetRGBA/SetHexColor/SetColor` 注释标注为 `Solid*`+`SetFillBrush` 的兼容别名（fogleman/gg 兼容层），实现本就等价 | go build/vet ✓ |
| 2 | 标废弃（已收敛） | `CustomBrush.LinearGradient/RadialGradient/HorizontalGradient/VerticalGradient` 加 `Deprecated:` 指向数据式画刷（缺 focus/extend/多色标，零生产消费者）；`ColorFunc/NewCustomBrush/Checkerboard/Stripes` 保留（程序化纹理扩展点） | go build ✓ + TestCustomBrush ✓ |
| 3 | 委托统一（已收敛） | `ClipRect` 委托 `ClipRectOp(…, ClipOpIntersect)`（SkClipOp 默认语义；单一实现，clipRect 为便捷别名） | TestS3a_M1_ClipRect / TestPaintContext_PushClipRect ✓ |
| 5 | 委托统一（已收敛，RoundRect 例外） | `PathBuilder.Rect/Circle/Ellipse` 委托 `Path.Rectangle/Circle/Ellipse`（同一实现）；**RoundRect 保留自身 skia 标准系数 k=0.5522847498**（≠ Path.RoundedRectangle 的精确圆弧公式 alpha≈0.5486，不委托以免改角几何） | TestPath + PathBuilder 相关 ✓ |
| 4/6/7/8/9/10/11/12 | 不收敛 | 分层依赖（文本/呈现/滤镜/编解码/SDF）或有明确扩展目标（scene 内部在用；recording→PDF/SVG） | — |

> 未动：`ClipPathOp`（clipPath 泛化）、`Path.RoundedRectangle`（Arc 精确公式实现，保留为 Path 方法组一部分）。`Clip/ClipPreserve` 已修复（2026-08-16，见 §7.3）。
---

## 8. 主包公开类型的导出方法速查（方法级清单，2026-08-15 go/ast）

> Context 的 199 个方法见 §3 全列；下表补齐其余公开类型的导出方法，与 `go doc ./render` 的 `<类型>.<方法>` 一致。状态列同 §0 约定。改动任一方法时同步本表与 §3。

| 类型 | 导出方法 | 功能（做什么） | 精简 | 状态 |
|------|---------|--------------|------|------|
| `Canvas`（**接口**，canvas.go，2026-10-22 S2a 建 89 方法，2026-10-23 S2b C2b 补齐到 100，同日晚 S2b C3 补到 102） | 102 方法分七族——范围/清除：Width·Height·Clear·ClearWithColor；状态：Push·Pop；变换：Identity·Translate·Scale·Rotate·RotateAbout·Shear·Transform·**SetTransform**·GetTransform；路径：MoveTo·LineTo·QuadraticTo·CubicTo·ClosePath·ClearPath·NewSubPath·SetPath·AppendPath·DrawPath·GetCurrentPoint；画笔状态：SetRGB·SetRGBA·SetFillBrush·SetStrokeBrush·**FillBrush**·**StrokeBrush**·**SetFillPattern**·**SetStrokePattern**·SetFillRule·SetLineWidth·SetLineCap·SetLineJoin·SetMiterLimit·**SetStroke**·SetDash·SetDashOffset·ClearDash·SetAntiAlias·AntiAlias；绘制：Fill·Stroke·FillPreserve·StrokePreserve·FillPath·StrokePath；裁剪：Clip·ClipPreserve·ClipRect·**ClipRectOp**·**ClipPathOp**·ClipRoundRect·ResetClip；形状：DrawPoint·DrawLine·DrawRectangle·DrawRoundedRectangle·DrawCircle·DrawEllipse·DrawArc·DrawEllipticalArc；图像/网格：DrawImage·DrawImageEx·DrawImageNine·DrawImageQuad·DrawImageRounded·DrawImageCircular·DrawAtlas·DrawAtlasEx·DrawDepthSprites·DrawVertices·**DrawVerticesEx**·**DrawMesh**·**DrawMeshEx**；文本：SetFont·Font·SetTextDecoration·**SetTextMode**·**SetLCDLayout**·DrawString·DrawStringAnchored·DrawStringWrapped·StrokeString·StrokeStringAnchored·DrawShapedGlyphs·DrawShapedColorGlyphs·MeasureString·MeasureMultilineString·WordWrap；图层：PushLayer·PushLayerIsolated·PushBackdropLayer·PopLayer；特效：ApplyBlur·ApplyColorMatrix·ApplyDropShadow·ApplyGrayscale（逐方法语义同 §3 各族表） | 可录制绘制面（SkCanvas 同位物） | 🔗 两实现：`*Context`（即时光栅，生产主链路）+ `recording.Recorder`（命令流，126 方法 = Canvas 102 + 24 录制面专有，其中 5 个为 `MaskCanvas` 掩码方法）；两侧各有编译期断言，接口本身无独立消费者；**C3 新增 `FillBrush`/`StrokeBrush` 的判据**：`Context.Pop` 不还原笔刷（只回退 CTM 与裁剪），回放面要把「进入时的环境笔刷」原样交还调用方，否则一次回放会把颜料漏给目标画布后续绘制——录制期与实况期必须用同一个口取回，故列入 Canvas 而非 recording 专有。**故意不进 Canvas** 的仍是只有拥有 GPU 的线程才存在、或结果是回放期事实的方法（GPU 纹理句柄、`FlushGPU`/呈现/离屏通道、编解码导出、`SetPixel`/`WritePixels`、损伤跟踪，以及 `TextPath` 字形轮廓提取——它要用 Context 自己的轮廓抽取器与字形缓存）；掩码已单独抽为 `MaskCanvas` 侧能力（设备空间平面不混入用户空间几何，仍不进 Canvas）。**S3 补充**：GPU 纹理句柄仍不是 `Canvas` 方法，但录制面可以携带——`Recorder.DrawGPUTexture` 落一条 `DrawGPUTextureCommand`，回放端只有实现可选 `GPUTextureBackend` 的设备线程才绑定（Flutter TextureLayer 语义；image/PDF/SVG 导出端跳过），所以 UI 侧不再把它当「录制期必须拒绝」的 live-only 绘制 |
| `MaskCanvas`（**接口**，mask_canvas.go，2026-10-26 新增） | 嵌入 `Canvas` 的 102 方法 + SetMask·ClearMask·InvertMask·ApplyMask·PushMaskLayer（共 107 方法） | 设备空间 alpha 掩码后处理（按目标设备像素，不是用户空间几何） | 掩码侧能力 | 🔌 `*Context` 与 `recording.Recorder` 实现；UI 经 `rendering.PaintContext.MaskCanvas()` 取用；maskops 全帧录制回放两连绿（`screen=0.0000%`/`pixel=18/18`/`live_only_skips=0`/`mask_skips=0`），生产直调消费者仍为零 |
| `Path` | MoveTo/LineTo/QuadraticTo/CubicTo · Arc·Circle·Ellipse·Rectangle·RoundedRectangle · Append · Clone · Transform · Clear/Reset · Close · ComputeMetrics · TotalLength · PositionAt/TangentAt · Length · Area · BoundingBox/Bounds · Contains · Winding · Flatten/FlattenCallback · Reversed · Trim · WithCorners · Discrete · Iterate · Verbs/Coords · NumVerbs · HasCurves/HasCurrentPoint · CurrentPoint · Op | 路径构建、附加形状、克隆/变换、度量、查询与高级造型 | 即时路径全部操作 | ✅（Trim/WithCorners/Discrete/Op 🔌） |
| `PathBuilder` | MoveTo/LineTo/QuadTo/CubicTo · Rect/RoundRect/Circle/Ellipse/Polygon/Star · Close · Build · Path | 链式声明形状后生成 Path（Rect/Circle/Ellipse 已委托 Path 实现，§7.5.1） | 链式构建器 | ✅ |
| `Paint` | SolidColor · SetBrush/GetBrush · ColorAt · SetStroke/GetStroke · EffectiveLineWidth · IsSolid/IsDashed · EffectiveDash · EffectiveLineCap/EffectiveLineJoin/EffectiveMiterLimit · Clone | 聚合画刷+描边状态并求生效值 | 绘制状态 | 🔗 |
| `Mask` | At/Set/Fill/Clear/Invert/Clone · Data/Bounds/Width/Height | 蒙版像素读写/反转/裁剪 | alpha 蒙版 | 🔗 |
| `Pixmap` | Width/Height · Data · At/Set/GetPixel/SetPixel/SetPixelPremul · FillRect · FillSpan/FillSpanBlend · Clear · ColorModel/ImageView/ToImage · EncodePNG/EncodeJPEG/SavePNG · GenerationID/NotifyPixelsChanged · Bounds | CPU 像素缓冲：读写像素、行填充、编解码、世代号 | CPU 像素缓冲 | ✅ |
| `SoftwareRenderer` | Fill/Stroke · Resize · SetAntiAlias/SetDeviceScale | 软渲染器生命周期与绘制 | CPU 软渲染 | 🔗 |
| `Renderer`（接口） | Fill/Stroke | 渲染抽象入口 | 渲染器抽象 | 🔗 |
| `SDFAccelerator` | Name/Init/Close · CanAccelerate · SetForceSDF · FillShape/FillPath/StrokeShape/StrokePath · Flush | 形状 SDF 快速填/描与强制开关 | CPU SDF 加速 | 🧪（生产仅内部 cpuFallback） |
| `Scene`（scene 子包同名类型） | → 见 §6.2 | — | — | 🔗 |
| `ImagePattern` | ColorAt · SetAnchor/SetClamp/SetOpacity/SetScale/SetTransform · Source（2026-10-23 S2b C2b 由 `GPUPatternSource` 改来，交出完整描述 `ImagePatternSource`） | 图像区域作图案及其锚点/颜色/缩放/GPU 源；建与还原走包级 `NewImagePattern`/`NewImagePatternFromSource` | 图像图案 | 🧪 |
| `SolidPattern`（`NewSolidPattern(color)`） | ColorAt | 实色图案采样/构造 | 实色图案 | 🧪 |
| `CustomBrush`（`NewCustomBrush`） | ColorAt · WithName | 函数式画刷求色/命名 | 函数式画刷 | 🔗 |
| `SolidBrush` | ColorAt/Lerp/WithAlpha/Opaque/Transparent | 实色画刷求色/透明度链 | 实色画刷 | ✅ |
| `LinearGradientBrush` | AddColorStop · ColorAt · SetExtend | 线性渐变加色标/越界模式 | 线性渐变 | ✅ |
| `RadialGradientBrush` | AddColorStop · ColorAt · SetExtend/SetFocus | 径向渐变加色标/焦点 | 径向渐变 | ✅ |
| `SweepGradientBrush` | AddColorStop · ColorAt · SetExtend/SetEndAngle | 圆锥渐变加色标/终止角 | 圆锥渐变 | ✅ |
| `Stroke` | WithWidth/WithCap/WithJoin/WithMiterLimit · WithDash/WithDashPattern/WithDashOffset · IsDashed · Clone | 描边参数链式设置 | 描边参数链 | ✅ |
| `Dash` | Clone · IsDashed · NormalizedOffset · PatternLength · Scale · WithOffset | 虚线序列运算（归一化/缩放/偏移） | 虚线序列 | 🔗 |
| `Matrix` | Multiply · TransformPoint/TransformVector · Invert · IsIdentity/IsTranslation/IsTranslationOnly/IsScaleOnly · ScaleFactor/MaxScaleFactor | 仿射矩阵运算与性质查询 | 仿射矩阵 | ✅ |
| `Point` | Add/Sub/Mul/Div · Dot/Cross · Length/LengthSquared · Normalize · Lerp · Distance · Rotate | 二维点算术/几何 | 点运算 | 🔗 |
| `Vec2` | Add/Sub/Mul/Div/Neg · Dot/Cross · Length/LengthSq · Normalize · Perp/Rotate · Lerp · Angle/Atan2/Approx · IsZero · ToPoint | 向量算术/几何 | 向量运算 | 🔗 |
| `RGBA` | RGBA · Premultiply/Unpremultiply · Lerp · Color | 颜色通道/预乘/插值 | 颜色值 | ✅ |
| `Line` | End/Start/Midpoint · Length · BoundingBox · Reversed · Subdivide/Subsegment · Eval | 线段几何求值/分割 | 线段几何 | 🔗 |
| `QuadBez` | Start/End · Eval/Extrema/Raise · BoundingBox · Subdivide/Subsegment | 二次贝塞尔求值/升阶/分割 | 二次贝塞尔 | 🔗 |
| `CubicBez` | Start/End · Eval/Extrema/Inflections/Deriv · Normal/Tangent · BoundingBox · Subdivide/Subsegment | 三次贝塞尔求值/拐点/切线 | 三次贝塞尔 | 🔗 |
| `Rect` | Width/Height · Contains · Union · (NewRect) | 矩形尺寸/包含/合并 | 矩形 | ✅ |
| `PathMetric` | IsEmpty · Length · PositionAt/TangentAt | 路径度量查询（取点/切线） | 路径度量 | 🔗 |
| `PresentTarget` | Context/Resize/SetVsync/Scale · PresentWith/PresentWithAuto/PresentClear · LastPresentOutcome/LastDamageAreaPx/GPUBackend/Fallbacks · InFullRecovery/SetResizeStormWindow/SetOnSwapchainResized · LogicalSize · WaitIdle · Close | 呈现目标绘制/呈现/恢复状态（SetOnSwapchainResized：swapchain 换尺寸回调，Wayland 宿主用于 xdg 窗口几何声明；SetVsync：运行时切换 Fifo↔Mailbox/Immediate，embedder 在 resize 风暴期关闭 vsync 让内容帧不被 Fifo 阻塞；运行时切换 Fifo↔Mailbox/Immediate（2026-08-26 修3 起 Wayland 同样生效）） | 呈现目标 | ✅ |
| `VideoBridge` | EnsureDevice · Show/ShowSeq · UploadFrame/UploadPlanesFrame · **UploadPlanesFrameAsync/UploadPending** · DrawCurrent · **PaintTexture** · **ID** · **PaintStats** · Stats · Close | 一路视频收敛：tick 上传 + paint 绘制；**异步平面口把 NV12 拷贝/上传交给 worker，跨线程后端双缓冲换槽、GL/metal 在光栅绘制时上传，调用方在 `UploadPending` 为真期间保留平面**；纹理源入口/注册 id/逐视图账本供保留窗按 id 绘制与取证 | 视频桥 | ✅（17 扇窗在用；4K 窗走异步口；`ui/scene` 纹理层） |
| `PaintStats` | —（数据） | 视频桥逐视图绘制账本快照（Paints/DirectPaints/FallbackPaints/FailedPaints/LastPaintOK/LastPaintDirect/ViewPaints/ViewOK/ViewLastOK/ViewDirect） | 绘制账本 | ✅ |
| `TextureSource`（接口） | PaintTexture | 纹理源契约：保留图层按 id 解析后绘制，源自己负责跨线程同步 | 纹理源 | ✅（`VideoBridge` 实现） |
| `TexturePaintOptions` | —（数据） | 纹理绘制位置/不透明度/插值/视图号 | 纹理画参 | ✅ |
| `VideoTexturePool` | Acquire/Release/Stats/Close | 按 exact 尺寸借还视频纹理（独立池，不进通用图缓存） | 视频独立池 | 🧪 |
| `FuncPainter`/`SolidPainter` | PaintSpan | 逐像素填色段 | 像素填色器 | 🧪 |
| 枚举类型通用方法 `String()` | PathVerb / PipelineMode / PresentMode / RasterizerMode / TextMode 均实现 String() 输出枚举名（日志/调试用） | 枚举打印 | 枚举调试 | 🔗 |
| `GPUAccelerator` 各接口方法 | 见 §4 接口群 | 加速能力探测 | 能力探测 | 🔗 |

> 内部/非导出接收者（LayeredPixmapTarget/PixmapTarget/Target/brushPattern/opacityMulBrush/nopHandler 等）不在公开目录内，改动不影响本表。

---

## 9. 主包常量与变量成员总表（2026-08-15 go/ast 提取，120 常量 + 11 变量；2026-09-05 1.2 搬包 +7 → 127 常量）

| 类型族 | 成员 |
|--------|------|
| `AdapterPolicy`（2026-09-05 自 render/gpu 搬入） | PolicyDefault · PolicyHigh · PolicyLow · PolicyNone · PolicyAuto · PolicyHighPerformance · PolicyLowPower |
| `AcceleratedOp` | AccelFill · AccelStroke · AccelScene · AccelText · AccelImage · AccelGradient · AccelCircleSDF · AccelRRectSDF |
| `Align` | AlignLeft · AlignCenter · AlignRight |
| `BlendMode` | BlendNormal · BlendClear · BlendCopy · BlendPlus · BlendModulate · BlendDestinationOut · BlendSourceAtop · BlendXor · BlendDestinationOver · BlendDestinationIn · BlendSourceIn · BlendDestinationAtop · BlendSourceOut · BlendColorBurn · BlendColorDodge · BlendColor · BlendDarken · BlendDifference · BlendExclusion · BlendHardLight · BlendHue · BlendLighten · BlendLuminosity · BlendMultiply · BlendOverlay · BlendSaturation · BlendScreen · BlendSoftLight |
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
| `VertexMode` | VertexModeTriangles · VertexModeTriangleFan |
| `MSAASampleCount` | MSAASampleCount1(=1) · MSAASampleCount4(=4) |
| 阈值/杂项 | MaxTrackedDamageRects · DamageFullCoverageThreshold · DamageMultiWasteRatio |
| var（预置色） | Black · White · Red · Green · Blue · Yellow · Cyan · Magenta · Transparent |
| var（错误） | ErrFallbackToCPU · ErrNilSurfaceView |

> 本表由 go/ast 对 `render/` 主包（package render，非测试文件）顶层声明提取；增删常量时同步本表。
>
> 注：上一版此处曾列 `TextureUsage` 族 5 个常量——它们不在主包，属 `render/render` 子包（`render/render/device.go`，与主包同名但独立 import 路径），已从本表移除；需要时以 `go doc ./render/render` 为准。

---

## 10. 维护规则（硬）

1. **增/改/删任何 render 公开符号**（顶层 func/type/const/var、Context 或其他类型的导出方法、子包顶层符号）→ 修改本文件对应分类表：新增补行、改名改行、删除删行，并更新 §0 统计（规模数字）与 §7 状态表。
2. **接线状态变化**（某 API 从"仅测试"变"有生产调用"，或反之；GPU 生效/失效）→ 立即更新 §7 对应条目，写清证据（调用点文件/真窗名/实测日期）。
3. **每次主体文档（ENGINE_UI_WIDGET_RENDER.md）改动涉及 render 公开 API 或接线** → 在本文件 §7/相应分类同步一行，并在主体文档 §10 修订表追加记录（版本列写 `API目录同步`）。
4. **新增 API 前先查 §7.5 重复对照**：若与既有 API 实质重复（同包双入口/封装/跨包副本），能复用就复用；确需新增则在 §7.5 补行并说明差异。禁止无谓引入平行入口。
5. 改动完成后跑 §0 核对命令（及 `go run ./scripts/apidoc`），确认清单与 `go doc` 一致后再合入。
6. 状态标注必须可追溯到证据：生产消费者 → 文件:行号；GPU 实测 → 真窗名/日期/现象；禁止凭印象标注。---

## 11. 附录：子包顶层符号完整清单（机器生成，go doc；scene/svg 2026-08-15，recording 2026-10-23 S2b C2b 后重出：+`ToRenderPattern`/`FromRenderPattern`/`CustomPatternBrush`/`NewCustomPatternBrush`/五个 `*Command`，`NewPatternBrush` 改收 `*render.ImageBuf`，删 `RepeatMode` 与其四常量；同日晚 S2b C3 再重出：`ContextBackend`/`NewContextBackend`/`ErrNoContext` 改名 `CanvasBackend`/`NewCanvasBackend`/`ErrNoCanvas`，`ApplyBrush`/`ApplyStroke` 实参由 `*render.Context` 放宽到 `render.Canvas`；2026-10-24 S3 再补 `DrawGPUTextureCommand`/`GPUTextureBackend`/`CmdDrawGPUTexture`/`ClipRoundRect`，录制包顶层声明 88→90、命令 45→46、`Backend` 37→38）

> 本附录由 `go doc ./render/{scene,recording,svg}` 顶层声明提取（含 const 块首行与缩进构造器），与正文 §6 族级归纳互补；**改动子包公开 API 时同步本附录（跑 §0 核对命令后替换对应行）**。`render/text` 包级导出 259 项改以 `go doc -all ./render/text` 为准（正文 §6.1 族级归纳）。

```text
scene      const DefaultMaxSizeMB = 64 ...
scene      var DefaultPool = NewEncodingPool()
scene      func CanUseGPU() bool
scene      func PutEncoding(enc *Encoding)
scene      func TextAdvance(glyphs []*RenderedGlyph) float32
scene      type Affine struct{ ... }
scene          func AffineFromMatrix(m render.Matrix) Affine
scene          func IdentityAffine() Affine
scene          func NewAffine(a, b, c, d, e, f float32) Affine
scene          func RotateAffine(angle float32) Affine
scene          func ScaleAffine(x, y float32) Affine
scene          func TranslateAffine(x, y float32) Affine
scene      type ArcShape struct{ ... }
scene          func NewArcShape(cx, cy, rx, ry, startAngle, endAngle float32, sweepClockwise bool) *ArcShape
scene      type BlendMode uint32
scene          const BlendNormal BlendMode = iota ...
scene          func AdvancedModes() []BlendMode
scene          func AllBlendModes() []BlendMode
scene          func BlendModeFromInternal(internal blend.BlendMode) BlendMode
scene          func HSLModes() []BlendMode
scene          func PaintBlendModeToScene(mode render.BlendMode) BlendMode
scene          func PorterDuffModes() []BlendMode
scene      type Brush struct{ ... }
scene          func SolidBrush(c render.RGBA) Brush
scene      type BrushKind uint32
scene          const BrushSolid BrushKind = iota ...
scene      type CacheEntry struct{ ... }
scene      type CacheStats struct{ ... }
scene      type CircleShape struct{ ... }
scene          func NewCircleShape(cx, cy, r float32) *CircleShape
scene      type ClipStack struct{ ... }
scene          func NewClipStack() *ClipStack
scene      type ClipState struct{ ... }
scene          func NewClipState(shape Shape, transform Affine) *ClipState
scene      type CompositeShape struct{ ... }
scene          func NewCompositeShape(shapes ...Shape) *CompositeShape
scene      type DamageTracker struct{ ... }
scene          func NewDamageTracker() *DamageTracker
scene      type Decoder struct{ ... }
scene          func NewDecoder(enc *Encoding) *Decoder
scene      type EllipseShape struct{ ... }
scene          func NewEllipseShape(cx, cy, rx, ry float32) *EllipseShape
scene      type Encoding struct{ ... }
scene          func GetEncoding() *Encoding
scene          func NewEncoding() *Encoding
scene      type EncodingPool struct{ ... }
scene          func NewEncodingPool() *EncodingPool
scene      type FillStyle uint32
scene          const FillNonZero FillStyle = 0 ...
scene      type Filter interface{ ... }
scene      type FilterChain struct{ ... }
scene          func NewFilterChain(filters ...Filter) *FilterChain
scene      type FilterType uint8
scene          const FilterNone FilterType = iota ...
scene      type GPUSceneRenderer struct{ ... }
scene          func NewGPUSceneRenderer(dc *render.Context) *GPUSceneRenderer
scene      type GlyphEntry struct{ ... }
scene      type GlyphRunData struct{ ... }
scene      type Image struct{ ... }
scene          func NewImage(width, height int) *Image
scene      type Iterator struct{ ... }
scene      type LayerCache struct{ ... }
scene          func DefaultLayerCache() *LayerCache
scene          func NewLayerCache(maxSizeMB int) *LayerCache
scene      type LayerKind uint8
scene          const LayerRegular LayerKind = iota ...
scene      type LayerStack struct{ ... }
scene          func NewLayerStack() *LayerStack
scene      type LayerState struct{ ... }
scene          func NewClipLayer(clip Shape) *LayerState
scene          func NewFilteredLayer(blend BlendMode, alpha float32) *LayerState
scene          func NewLayerState(kind LayerKind, blend BlendMode, alpha float32) *LayerState
scene      type LineCap uint32
scene          const LineCapButt LineCap = iota ...
scene      type LineJoin uint32
scene          const LineJoinMiter LineJoin = iota ...
scene      type LineShape struct{ ... }
scene          func NewLineShape(x1, y1, x2, y2 float32) *LineShape
scene      type Path struct{ ... }
scene          func NewPath() *Path
scene      type PathBuilder interface{ ... }
scene      type PathElement struct{ ... }
scene      type PathPool struct{ ... }
scene          func NewPathPool() *PathPool
scene      type PathShape struct{ ... }
scene          func NewGGPathShape(ggPath *render.Path) *PathShape
scene          func NewPathShape(path *Path) *PathShape
scene      type PathVerb uint8
scene          const MoveTo PathVerb = iota ...
scene      type PieShape struct{ ... }
scene          func NewPieShape(cx, cy, r, startAngle, endAngle float32, sweepClockwise bool) *PieShape
scene      type Point struct{ ... }
scene      type PolygonShape struct{ ... }
scene          func NewPolygonShape(points ...float32) *PolygonShape
scene      type Rect struct{ ... }
scene          func EmptyRect() Rect
scene          func TextBounds(glyphs []*RenderedGlyph) Rect
scene      type RectShape struct{ ... }
scene          func NewRectShape(x, y, width, height float32) *RectShape
scene      type RegularPolygonShape struct{ ... }
scene          func NewRegularPolygonShape(cx, cy, r float32, sides int, rotation float32) *RegularPolygonShape
scene      type RenderStats struct{ ... }
scene      type RenderedGlyph struct{ ... }
scene      type Renderer struct{ ... }
scene          func NewRenderer(width, height int, opts ...RendererOption) *Renderer
scene      type RendererOption func(*Renderer)
scene          func WithCache(cache *LayerCache) RendererOption
scene          func WithCacheSize(mb int) RendererOption
scene          func WithTileSize(size int) RendererOption
scene          func WithWorkers(n int) RendererOption
scene      type RoundRectShape struct{ ... }
scene          func NewRoundRectShape(rect Rect, rx, ry float32) *RoundRectShape
scene          func NewRoundRectShapeUniform(rect Rect, r float32) *RoundRectShape
scene      type RoundedRectShape struct{ ... }
scene          func NewRoundedRectShape(x, y, width, height, radius float32) *RoundedRectShape
scene      type Scene struct{ ... }
scene          func NewScene() *Scene
scene      type SceneBuilder struct{ ... }
scene          func NewSceneBuilder() *SceneBuilder
scene          func NewSceneBuilderFrom(scene *Scene) *SceneBuilder
scene      type ScenePool struct{ ... }
scene          func NewScenePool() *ScenePool
scene      type Shape interface{ ... }
scene      type StarShape struct{ ... }
scene          func NewStarShape(cx, cy, outerRadius, innerRadius float32, points int, rotation float32) *StarShape
scene      type StrokeStyle struct{ ... }
scene          func DefaultStrokeStyle() *StrokeStyle
scene      type Tag byte
scene          const TagTransform Tag = 0x01 ...
scene      type TaggedBounds struct{ ... }
scene      type TextFlags uint16
scene          const TextFlagHinting TextFlags = 1 << iota ...
scene      type TextRenderer struct{ ... }
scene          func NewTextRenderer() *TextRenderer
scene          func NewTextRendererWithConfig(config TextRendererConfig) *TextRenderer
scene      type TextRendererConfig struct{ ... }
scene          func DefaultTextRendererConfig() TextRendererConfig
scene      type TextRendererPool struct{ ... }
scene          func NewTextRendererPool() *TextRendererPool
scene      type TextShape struct{ ... }
scene          func NewTextShape(str string, face text.Face, x, y float32) (*TextShape, error)
scene      type TransformShape struct{ ... }
scene          func NewTransformShape(shape Shape, transform Affine) *TransformShape
recording  const FillRuleNonZero = render.FillRuleNonZero ...
recording  const InvalidRef = ^uint32(0)
recording  var ErrNoCanvas = errors.New("recording: CanvasBackend has no canvas")
recording  func ApplyBrush(dc render.Canvas, brush Brush, fill bool)
recording  func ApplyStroke(dc render.Canvas, stroke Stroke)
recording  func Backends() []string
recording  func Count() int
recording  func IsRegistered(name string) bool
recording  func Register(name string, factory BackendFactory)
recording  func ToRenderBrush(b Brush) render.Brush
recording  func ToRenderPattern(b Brush) (render.Pattern, bool)
recording  func TransformPoint(m Matrix, x, y float64) (float64, float64)
recording  func Unregister(name string)
recording  type ApplyBlurCommand struct{ ... }
recording  type ApplyColorMatrixCommand struct{ ... }
recording  type ApplyDropShadowCommand struct{ ... }
recording  type ApplyGrayscaleCommand struct{}
recording  type Backend interface{ ... }
recording      func MustBackend(name string) Backend
recording      func NewBackend(name string) (Backend, error)
recording  type BackendFactory func() Backend
recording  type Brush interface{ ... }
recording      func FromRenderBrush(b render.Brush) Brush
recording      func FromRenderPattern(p render.Pattern, pool *ResourcePool) Brush
recording  type BrushRef uint32
recording  type CanvasBackend struct{ ... }
recording      func NewCanvasBackend(dc render.Canvas) *CanvasBackend
recording  type ClearClipCommand struct{}
recording  type ClipPathOpCommand struct{ ... }
recording  type ClipRectCommand struct{ ... }
recording  type ClipRectOpCommand struct{ ... }
recording  type ClipRoundRectCommand struct{ ... }
recording  type Command interface{ ... }
recording  type CommandType uint8
recording      const CmdSave CommandType = iota ...
recording  type ConcatCommand struct{ ... }
recording  type CustomPatternBrush struct{ ... }
recording      func NewCustomPatternBrush(p render.Pattern) CustomPatternBrush
recording  type DrawAtlasCommand struct{ ... }
recording  type DrawDepthSpritesCommand struct{ ... }
recording  type DrawGlyphsCommand struct{ ... }
recording  type DrawGPUTextureCommand struct{ ... }
recording  type DrawImageCircularCommand struct{ ... }
recording  type DrawImageCommand struct{ ... }
recording  type DrawImageNineCommand struct{ ... }
recording  type DrawImageQuadCommand struct{ ... }
recording  type DrawImageRoundedCommand struct{ ... }
recording  type DrawMeshCommand struct{ ... }
recording  type DrawTextCommand struct{ ... }
recording  type DrawVerticesCommand struct{ ... }
recording  type ExtendMode int
recording      const ExtendPad ExtendMode = iota ...
recording  type FileBackend interface{ ... }
recording  type FillPathCommand struct{ ... }
recording  type FillRectCommand struct{ ... }
recording  type FillRule = render.FillRule
recording  type FontRef uint32
recording  type GPUTextureBackend interface{ ... }
recording  type GradientStop struct{ ... }
recording  type ImageRef uint32
recording  type LineCap = render.LineCap
recording  type LineJoin = render.LineJoin
recording  type LinearGradientBrush struct{ ... }
recording      func NewLinearGradientBrush(x0, y0, x1, y1 float64) *LinearGradientBrush
recording  type Matrix = render.Matrix
recording      func Identity() Matrix
recording      func Rotate(angle float64) Matrix
recording      func Scale(sx, sy float64) Matrix
recording      func Shear(x, y float64) Matrix
recording      func Translate(x, y float64) Matrix
recording  type PathRef uint32
recording  type PatternBrush struct{ ... }
recording      func NewPatternBrush(img *render.ImageBuf) *PatternBrush
recording  type PixmapBackend interface{ ... }
recording  type PopLayerCommand struct{}
recording  type PushBackdropLayerCommand struct{ ... }
recording  type PushLayerCommand struct{ ... }
recording  type PushLayerIsolatedCommand struct{ ... }
recording  type RadialGradientBrush struct{ ... }
recording      func NewRadialGradientBrush(cx, cy, startRadius, endRadius float64) *RadialGradientBrush
recording  type Recorder struct{ ... }
recording      func NewRecorder(width, height int) *Recorder
recording  type Recording struct{ ... }
recording  type Rect struct{ ... }
recording      func NewRect(x, y, width, height float64) Rect
recording      func NewRectFromPoints(x1, y1, x2, y2 float64) Rect
recording  type ResourcePool struct{ ... }
recording      func NewResourcePool() *ResourcePool
recording  type RestoreCommand struct{}
recording  type SaveCommand struct{}
recording  type SetAntiAliasCommand struct{ ... }
recording  type SetClipCommand struct{ ... }
recording  type SetDashCommand struct{ ... }
recording  type SetFillRuleCommand struct{ ... }
recording  type SetFillStyleCommand struct{ ... }
recording  type SetLCDLayoutCommand struct{ ... }
recording  type SetLineCapCommand struct{ ... }
recording  type SetLineJoinCommand struct{ ... }
recording  type SetLineWidthCommand struct{ ... }
recording  type SetMiterLimitCommand struct{ ... }
recording  type SetStrokeStyleCommand struct{ ... }
recording  type SetTextDecorationCommand struct{ ... }
recording  type SetTextModeCommand struct{ ... }
recording  type SolidBrush struct{ ... }
recording      func NewSolidBrush(color render.RGBA) SolidBrush
recording  type Stroke struct{ ... }
recording      func DefaultStroke() Stroke
recording  type StrokePathCommand struct{ ... }
recording  type StrokeRectCommand struct{ ... }
recording  type StrokeTextCommand struct{ ... }
recording  type SweepGradientBrush struct{ ... }
recording      func NewSweepGradientBrush(cx, cy, startAngle float64) *SweepGradientBrush
recording  type WriterBackend interface{ ... }
svg        func Render(data []byte, width, height int) (*image.RGBA, error)
svg        func RenderWithColor(data []byte, width, height int, c color.Color) (*image.RGBA, error)
svg        type Attrs struct{ ... }
svg        type CircleElement struct{ ... }
svg        type Document struct{ ... }
svg            func Parse(data []byte) (*Document, error)
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

### 附录 B：子包枚举/常量成员补充清单（go/ast 2026-08-15；[recording] 各行 2026-10-23 S2b C2b 后重测）

> 附录 A 只含顶层声明；此清单补枚举成员（各类型常量）。正文 §6 族表未逐个列出时以本清单为准；改动子包常量时同步本清单。

```text
[scene] BlendMode: BlendClear BlendColor BlendColorBurn BlendColorDodge BlendCopy BlendDarken BlendDestination BlendDestinationAtop BlendDestinationIn BlendDestinationOut BlendDestinationOver BlendDifference BlendExclusion BlendHardLight BlendHue BlendLighten BlendLuminosity BlendMultiply BlendNormal BlendOverlay BlendPlus BlendSaturation BlendScreen BlendSoftLight BlendSourceAtop BlendSourceIn BlendSourceOut BlendSourceOver BlendXor
[scene] BrushKind: BrushImage BrushLinearGradient BrushRadialGradient BrushSolid
[scene] FillStyle: FillEvenOdd FillNonZero
[scene] FilterType: FilterBlur FilterColorMatrix FilterDropShadow FilterNone
[scene] LayerKind: LayerClip LayerFiltered LayerRegular
[scene] LineCap: LineCapButt LineCapRound LineCapSquare
[scene] LineJoin: LineJoinBevel LineJoinMiter LineJoinRound
[scene] PathVerb: Close CubicTo LineTo MoveTo QuadTo
[scene] Tag: TagBeginClip TagBeginPath TagBrush TagClosePath TagCubicTo TagEndClip TagEndPath TagFill TagFillRoundRect TagImage TagLineTo TagMoveTo TagPopLayer TagPushLayer TagQuadTo TagSetAntiAlias TagStroke TagText TagTransform
[scene] TextFlags: TextFlagCJK TextFlagHinting
[recording] CommandType: CmdSave CmdRestore CmdConcat CmdSetClip CmdClearClip CmdClipRoundRect CmdFillPath CmdStrokePath CmdFillRect CmdStrokeRect CmdDrawImage CmdDrawText CmdStrokeText CmdSetFillStyle CmdSetStrokeStyle CmdSetLineWidth CmdSetLineCap CmdSetLineJoin CmdSetMiterLimit CmdSetDash CmdSetFillRule CmdSetAntiAlias CmdClipRect CmdSetTextDecoration CmdDrawImageNine CmdDrawImageQuad CmdDrawImageRounded CmdDrawImageCircular CmdDrawAtlas CmdDrawDepthSprites CmdDrawVertices CmdDrawGlyphs CmdDrawGPUTexture CmdClipRectOp CmdClipPathOp CmdSetTextMode CmdSetLCDLayout CmdDrawMesh CmdPushLayer CmdPushLayerIsolated CmdPushBackdropLayer CmdPopLayer CmdApplyBlur CmdApplyColorMatrix CmdApplyDropShadow CmdApplyGrayscale（46 类，2026-10-24 S3 后按 command.go const 块顺序重出；S3 新增 CmdDrawGPUTexture）
[recording] ExtendMode: ExtendPad ExtendReflect ExtendRepeat
[recording] FillRule: FillRuleEvenOdd FillRuleNonZero（2026-10-22 S2a 起为 `= render.FillRule` 别名，成员从 render 再导出，值同源）
[recording] LineCap: LineCapButt LineCapRound LineCapSquare（同上，`= render.LineCap` 别名再导出）
[recording] LineJoin: LineJoinBevel LineJoinMiter LineJoinRound（同上，`= render.LineJoin` 别名再导出）
[recording] ~~RepeatMode~~（**2026-10-23 S2b C2b 删除**：`RepeatMode` 与 `RepeatNone/RepeatX/RepeatY/RepeatBoth` 一并移除，图样重复语义由 `render.ImagePattern` 的源矩形 + clamp 承担，`recording.PatternBrush` 改存完整可描述数据，不留第二套真值）
```
