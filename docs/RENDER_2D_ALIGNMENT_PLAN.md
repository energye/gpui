# render 2D 对齐实现计划（RENDER_2D_ALIGNMENT_PLAN）

> **目标**：按 Skia/Flutter 工业级渲染语义，修复三个已知 2D 能力洞。状态以本文档 + `docs/RENDER_API_CATALOG.md` §7.3 + `docs/ENGINE_UI_WIDGET_RENDER.md` §10 修订表为准。
>
> **对齐基准**：洞1 → Skia Ganesh（stencil-then-cover 任意路径裁剪）；洞2 → Skia `SkCubicResampler`（GPU 4×4 卷积，Flutter 无 bicubic，引擎已暴露 API 故以 Skia 为基准）；洞3 → Flutter SkParagraph 语义（UAX#14 断行 + 真实 advance；引擎 `render/text` 已实现该层，ui 层接通）。
>
> **实现状态（2026-08-15）**：洞1 ✅ 代码完成（管线激活 + 单测，GPU 真窗复测待有 GPU 环境）；洞2 ✅ 代码完成（shader + 管线变体 + 对照单测 + 负权重 bug 修复）；洞3 ✅ 代码完成（fitRunPrefix 词边界断行 + drawTextWrapped 接 WrapText + 单测）；洞4 ✅ 实现并 GPU 验收（clip_leaf 阶段 + RenderSceneComputeDef 入口 + CPU 参考 even-odd 错位修复；`TestVelloComputeClipGolden`/`TestVelloComputeGolden` 真机 0.00% diff，compute_clip 示例 120000 像素 0 差异）。全仓 `go build ./...` OK。

---

## 洞1：GPU 任意路径裁剪 `Clip()`/`ClipPreserve()` 画穿（§7.3）

### 1.1 根因（已定位，代码证据）

`GPU-CLIP-003a` 深度裁剪链路是**死代码**：`s.depthClipPipeline`（`render/internal/gpu/render_session.go:552` 字段）**从未被初始化**——`NewDepthClipPipeline`（`depth_clip.go:117`）在生产代码零调用，只在 `depth_clip_test.go` 使用。后果链：

- `render_session.go:1538` 守卫 `groups[i].ClipPath != nil && s.depthClipPipeline != nil` **恒为 false** → `grpRes[i].hasDepthClip` 恒 false
- `recordGroupDraws`（`render_session.go:4825`）的两阶段 `RecordDraw`（stencil 填充 → cover 写 Z=0.0，`depth_clip.go:536`）永不执行
- 内容管线 `DepthCompare=GreaterEqual` 变体（`glyph_mask_pipeline.go:308` 等）因 `hasDepthClip=false` 永不被选 → 内容无裁剪 → **画穿**

### 1.2 对齐设计（Skia Ganesh 模式，代码已按此设计，只需激活）

`depth_clip.go` 头注释完整描述了 Skia 模式：同一 render pass 内，Phase1 fan 三角形 stencil 非零环绕（IncrementWrap/DecrementWrap）→ Phase2 cover quad 仅在 stencil≠0 处写深度 Z=0.0 并清零 stencil → 内容管线 `GreaterEqual(0.0, depth)` 通过，`GreaterEqual(0.0, 1.0)` 失败（clear 值 1.0）。嵌套裁剪几何求交（见 depth_clip.go:61-76 注释）。**不新增管线，只激活已存在实现。**

### 1.3 改动清单

| 文件 | 改动 |
|------|------|
| `render/internal/gpu/render_session.go` | `NewGPURenderSession`（:637）增加 `s.depthClipPipeline = NewDepthClipPipeline(device, queue, sampleCount)`（构造函数不触 GPU，`ensurePipeline` 才编译，安全）；`Destroy`（:2007）已有释放逻辑 ✓ |
| `render/internal/gpu/render_session.go` | `ensureStagePipelines`（:2169）`needDepthClip` 分支加防御：若 `s.depthClipPipeline == nil` 则创建（防未来调用方漏初始化） |
| `render/internal/gpu/depth_clip_test.go` | 新增会话级集成测试：`NewGPURenderSession` 后 `depthClipPipeline != nil` + `ensurePipeline` 编译成功（用现有 `createNativeDevice`，无 GPU 自动 skip） |

### 1.4 验收标准

1. **单测**：`go test ./render/internal/gpu -run 'TestDepthClip|TestGPURenderSession'` 全 PASS（含新增会话初始化测试；无 GPU 环境 t.Skipf）
2. **GPU 真窗**：`examples/render_clipping/main.go` 修复前画穿 → 修复后矩形内 fill/渐变/任意路径全部被裁剪（与 CPU 离屏结果一致）
3. **回归**：`p12_clip_mask_gpu_test` / `context_clip_depth_test` / `context_clip_test` 全绿
4. **文档**：`RENDER_API_CATALOG.md` §3.2 `Clip()` 状态 ⚠️→✅（GP 实测日期）；§7.3 删行；`ENGINE_UI_WIDGET_RENDER.md` §10 追加修订

---

## 洞2：Bicubic 采样 GPU 化（`InterpBicubic` 显式 CPU 回退）

### 2.1 根因（已定位）

`render/context_image.go:246-247`：`opts.Interpolation != InterpBicubic` 才走 `tryGPUDrawImage`；Bicubic 走 CPU `intImage.DrawImage` 4×4 卷积。GPU 侧 `image_pipeline.go:334-352` 只创建 linear / nearest 两个采样器（WebGPU 采样器无 bicubic，须 shader 实现）。

### 2.2 对齐设计（Skia `SkCubicResampler`）

Skia GPU 对 kCubic 采样用 fragment shader 做 4×4 双三次卷积（默认 Mitchell，可配 B/C 系数）。引擎实现对齐：

- 新增 `render/internal/gpu/shaders/cover_textured_bicubic.wgsl`：4×4=16 tap 采样，双三次卷积核带 B/C 系数（`SkCubicResampler` 语义，wgsl 常量），**默认 Catmull-Rom（B=0, C=0.5）与现有 CPU `intImage.SampleBicubic` 一致**——像素级对照可验收；Mitchell（B=1/3, C=1/3）即 Skia 默认，改常量即可切换，本期不新增 API
- `image_pipeline.go` `TexturedQuadPipeline`：新增 `pipelineWithBicubic` 变体（`ensureBicubicPipeline()`，同 bind group layout，shader 换 bicubic）；Destroy 释放
- `ImageDrawCommand` 增加 `Interpolation intImage.InterpolationMode` 字段；GPU 命令构建处（`context_image.go` tryGPUDrawImage 的 QueueImageDraw 分支）透传
- `RecordDraws`（image_pipeline.go:413）按 `dc.Interpolation` 选管线：bicubic→bicubic 变体，linear/nearest→现有
- `context_image.go`：`InterpBicubic` 不再强制 CPU 分支——统一走 GPU（GPU 失败才回退 CPU，保留现有 `recordCPUFallbackReason` 兜底语义）

### 2.3 改动清单

| 文件 | 改动 |
|------|------|
| `render/internal/gpu/shaders/cover_textured_bicubic.wgsl` | 新增（16-tap Mitchell 卷积） |
| `render/internal/gpu/image_pipeline.go` | `pipelineWithBicubic` + `ensureBicubicPipeline()` + Destroy + RecordDraws 选择 |
| `render/internal/gpu/gpu_render_context.go`（或 image 命令定义处） | `ImageDrawCommand.Interpolation` 字段 + 填充 |
| `render/context_image.go` | Bicubic 分支改走 GPU（保留 CPU 兜底） |
| `render/internal/gpu/image_pipeline_test.go`（若有）/ 新增 | bicubic 管线编译 + 采样权重单测 |

### 2.4 验收标准

1. **权重单测**：bicubic 卷积权重（Mitchell B=C=1/3）与 CPU `intImage` 双三次同输入输出像素差 ≤ 1/255（或 epsilon 级）
2. **GPU 视觉**：`images_gpu_visual_test` / `examples/render_images` 显式 bicubic 缩放无锯齿、与 CPU 结果一致
3. **回归**：默认 bilinear/nearest 路径零行为变化（`context_image_test` 全绿）
4. **文档**：`RENDER_API_CATALOG.md` §3.5 `DrawImageEx`「Bicubic 显式回退 CPU」→ GPU 有效（保留 CPU 兜底注记）；§10 修订

---

## 洞3：UI 排版接通 `render/text`（Flutter SkParagraph 语义）

### 3.1 根因（已定位）

- ui 文本宽度用 `text.Measure` 估算：`ui/rendering/text.go` `measureLine`（:331）、`paragraph.go` `measureRunString`（:234）——非真实 advance
- 断行用 `render.Context.DrawStringWrapped`（`paint_context.go:363`）——render 层自研断行（`render/text.go`），未接 `render/text` 的 UAX#14 排版器
- `render/text` 已具备对齐层：`LayoutText/LayoutTextWithContext`（`layout.go:119`，`Layout{Lines[]Line{Runs,Glyphs,Width,Ascent,Descent,Y}}`）、`WrapText`（`wrap.go:402`，WrapWordChar 词优先+字符兜底）、`SegmentText`（UAX#14 分段）、bidi（x/text）

### 3.2 对齐设计（Flutter SkParagraph 语义，全部落在 ui/rendering 低风险层）

- **断行**：`paint_context.go drawTextWrapped` 改为先用 `text.WrapText(s, face, maxWidth, text.WrapWordChar)` 断行，再逐行 `DrawString`（不调用 render 自研断行的 DrawStringWrapped）
- **测宽**：`RenderText.measureLine` / `measureRunString` 的 `text.Measure` 估算替换为 `LayoutText`（或 `WrapText` 单行宽度）真实 advance；无 face 时保留现有 approxCharW 兜底
- **多行布局**：`RenderText` 增加可选 `Layout(maxWidth)`：内部 `text.LayoutText` → 行高/基线用 `Line.Ascent/Descent/Y` 真实值（替换 `fs*1.25*ls` 估算），绘制逐行按 `Line.Glyphs`（或逐行 DrawString）
- **已知缺口记录**（本期不实现，见 §3.4）：`Justify` 两端对齐、ellipsis 省略号——`text.LayoutOptions` 无此二字段，属 API 新增，单列后续项

### 3.3 改动清单

| 文件 | 改动 |
|------|------|
| `ui/rendering/text.go` | `measureLine` 用 `text.WrapText`/`LayoutText` 真实 advance（保留 approx 兜底）；`lineHeightLogical` 用 `face.Metrics` 真实值（已有），新增 `layout(maxWidth)` 走 `text.LayoutText` |
| `ui/rendering/paragraph.go` | `measureRunString` 接真实 advance；`ParagraphBuilder` 布局走 `text.LayoutText` 行结构 |
| `ui/rendering/paint_context.go` | `drawTextWrapped`：`text.WrapText` 断行 → 逐行 `DrawString`（去掉对 render 自研断行的依赖） |

### 3.4 验收标准

1. **单测**：`ui/rendering` 文本相关测试全绿；新增宽度一致性测试（估算 vs Layout 真实 advance 在 CJK/Latin 混合上的偏差 < 1px）
2. **回归**：`go test ./ui/...` 全绿（组件布局宽度变化以单测锁定，防回归）
3. **真窗**：现有 ui_wr_* 真窗文本 HUD/正文不串行、不断字（GPU 目测 + skip 门禁不回退）
4. **已知缺口**：Justify/ellipsis 单列后续（`text.LayoutOptions` 需新增 `AlignmentJustify` 与 `Ellipsis` 字段，属 render/text 公开 API 变更，按 API 目录同步纪律走）

---

## 洞4：Vello 计算管线 GPU 裁剪集成（compute_clip GPU vs CPU 结果不一致）

### 4.1 现象与根因（已定位，代码证据）

`render/examples/compute_clip`（Vello 计算管线裁剪演示）GPU 与 CPU 结果不一致、comparison 无意义、GPU 结果不正确。三层事实：

- **着色器层——已实现但从未被使用**：`coarse.wgsl` 已有 `DRAWTAG_BEGIN_CLIP/END_CLIP` 的 PTCL 生成（含 clip_zero_depth 空裁剪优化）；`fine.wgsl` 已有 `CMD_BEGIN_CLIP/END_CLIP` 混合栈执行（`BLEND_STACK_SPLIT=4` + blend_spill 溢出）。主机侧从不构造带裁剪标签的场景 → 这些分支是死代码。
- **管线编排层——功能缺失**：`vello_compute.go` 9 阶段无 clip_leaf 阶段、无 `ClipInp` 缓冲；`draw_leaf.wgsl` 不写 clip 配对输入。CPU 侧 `clipLeafScan`（`clip_leaf.go:24`，栈式配对）在 GPU 无对应物 → 即使塞入裁剪场景，EndClip 的 `draw_monoid.path_ix / scene_offset` 未修正，coarse 读到错误裁剪路径与混合参数 → 裁剪画穿（表现即 bug）。
- **入口层——功能缺失**：`VelloAccelerator` 只有收 `PathDef`（无裁剪）的 `RenderSceneCompute`；`compute_clip/main.go` 只能让 GPU 渲染无裁剪子集。

### 4.2 对齐设计（Skia/Flutter 裁剪语义，Vello 计算管线 tile 版 stencil-then-cover）

Skia/Flutter 裁剪语义 → 本管线映射（全部已实现，本期只补驱动）：

| Skia/Flutter 语义 | 本管线已实现载体 | 状态 |
|------|------|------|
| `save(); clipPath(path, kIntersect, aa); draw; restore()` 求交裁剪 | `CmdBeginClip`（清空像素=层隔离）+ `CmdEndClip`（clip 路径覆盖度 area 遮罩合成） | 着色器已写，本期激活 |
| 嵌套裁剪逐层求交（LIFO） | fine 混合栈 push/pop；`clipLeafScan` 栈式配对 = 保存/恢复栈 | 需 clip_leaf 阶段激活 |
| 裁剪边 AA 部分覆盖（Skia analytic AA） | clip 路径逐像素 area 覆盖度（`fill_path`） | 着色器已写 |
| 空裁剪快速路径（Skia empty-clip） | `clip_zero_depth` 抑制后续 draw | 着色器已写 |
| Flutter ClipPath+Opacity（层不透明度） | `CmdEndClip` 的 `alpha` 合成项（`fg = rgba*area*alpha`） | 着色器已写 |
| 层内混合（source-over） | `blend=0` source-over（细粒度 blend 模式单列，非本期） | 已实现 |

**实现缺口 = 驱动部分**：① `draw_leaf.wgsl` 写 `ClipInp`（BeginClip 记正 path_ix，EndClip 记 `~draw_ix` 补码，与 CPU `drawLeafScan` 一致）；② 新增 `clip_leaf.wgsl` 计算阶段：单 workgroup 串行栈遍历 `ClipInp`，把 EndClip 的 `draw_monoid.path_ix/scene_offset` 修正为 BeginClip 的（与 CPU `clipLeafScan` 1:1 移植，语义必然一致）；③ 主机入口 `RenderSceneComputeDef` 收 `[]SceneElement`，`EncodeSceneDef` 编码（CPU 已有，`scene_encode.go:205`），复用 `buildPathMetadata`（EndClip 空路径自然处理：零 bbox、不占 tile）。

### 4.3 改动清单

| 文件 | 改动 |
|------|------|
| `render/internal/gpu/tilecompute/shaders/clip_leaf.wgsl` | 新增：`VelloStageClipLeaf` 计算阶段（`@workgroup_size(1)` 单 workgroup 串行栈） |
| `render/internal/gpu/tilecompute/shaders/draw_leaf.wgsl` | `@binding(5)` 加 `clip_inp` storage；BeginClip/EndClip 写 `ClipInp`（镜像 CPU `drawLeafScan`） |
| `render/internal/gpu/vello_compute.go` | 枚举插 `VelloStageClipLeaf`（draw_leaf 之后、path_count 之前）；`ClipInp` 缓冲（`n_clip*8` 字节）+ 绑定布局/entries + 派发 + `ComputeWorkgroupCount` 特例（n>0 → 1 workgroup） |
| `render/internal/gpu/vello_accelerator.go` | 新增 `RenderSceneComputeDef(width,height,bg,elements)`；`dispatchComputeSceneDef` + 抽出 `dispatchSceneCore`（PathDef/SceneElement 共用上传→派发→读回） |
| `render/internal/gpu/scene_auto.go` | 新增自动切换入口 `RenderSceneAuto`（收 `[]SceneElement`）：GPU 计算管线可用 → `gpu-compute`；否则回退 CPU 参考 `cpu-reference`（带回退原因），调用方零分支——示例不再手动区分 GPU/CPU |
| `render/examples/compute_clip/main.go` | GPU 改用 `RenderSceneAuto` 渲染**同一** `buildClipScene()`（一套场景、一次调用），输出单图 `tmp/compute_clip.png` |
| `render/internal/gpu/golden_test.go` | 新增 `TestVelloComputeClipGolden`：`RenderSceneComputeDef` vs CPU `RasterizeSceneDefPTCL`（GPU 门禁，无 GPU t.Skipf） |
| `render/internal/gpu/clip_leaf_host_test.go` | 新增主机侧测试（无 GPU）：阶段布局、绑定 entries、clip 场景元数据与 CPU 布局一致 |

### 4.4 验收标准

1. **单测（无 GPU）**：`go test ./render/internal/gpu -run 'ClipLeaf|ClipScene|VelloCompute'` 全 PASS——阶段顺序含 clip_leaf、`ComputeWorkgroupCount(ClipLeaf)`、clip 场景 `buildPathMetadata`（NumPaths/空路径 bbox/样式位）与 CPU `PackScene` 布局一致
2. **GPU golden（真机）**：`TestVelloComputeClipGolden` 通过——`RenderSceneComputeDef` 输出与 CPU `RasterizeSceneDefPTCL` 像素差 ≤ 1.0%；无 GPU 环境 `t.Skipf`（禁假绿）
3. **示例（真机）**：`go run ./render/examples/compute_clip` → 一套场景代码 + `RenderSceneAuto` 自动切换（GPU 可用 `gpu-compute`/不可用回退 `cpu-reference`），输出 `tmp/compute_clip.png`；诊断像素（clip 内外/星形空心中心）正确；`TestVelloComputeClipGolden`/`TestRenderSceneAuto`（双后端 0 差异）真机全绿
4. **回归**：既有 GPU golden（`TestVelloComputeGolden`）不回退——`dispatchSceneCore` 重构保持 PathDef 路径行为不变
5. **文档**：`ENGINE_UI_WIDGET_RENDER.md` §10 追加修订行；本计划文档状态更新

---

## 实现顺序与回归矩阵

| 顺序 | 洞 | 层 | 风险 | 回归范围 |
|------|----|----|------|---------|
| 1 | 洞1 深度裁剪激活 | `render/internal/gpu` | 高 | `go test ./render/internal/gpu/...` + clipping 真窗 + p12/context_clip 族 |
| 2 | 洞2 Bicubic GPU | `render/` + `internal/gpu` | 高 | `go test ./render/...`（context_image/image 族）+ images 真窗 |
| 3 | 洞3 排版接通 | `ui/rendering` | 低 | `go test ./ui/...` + ui_wr_* 真窗文本目测 |
| 4 | 洞4 计算管线裁剪集成 | `render/internal/gpu` | 高 | `go test ./render/internal/gpu/...`（Vello 族）+ compute_clip 真窗（GPU vs CPU 同场景） |

每洞完成后：跑该洞回归 → 更新 `RENDER_API_CATALOG.md`（§3.x/§7.3 状态 + 证据行）→ `ENGINE_UI_WIDGET_RENDER.md` §10 追加修订行（版本列写 `API目录同步` 或洞名）。

## 文档同步义务

- 洞1 收口：`RENDER_API_CATALOG.md` §3.2 `Clip()/ClipPreserve()` ⚠️→✅（真窗名/日期）；§7.3 首行删除；§7.1 加「任意路径裁剪（stencil+depth，Skia 模式）」
- 洞2 收口：§3.5 `DrawImageEx` 状态注记更新；§7.3 若含 bicubic 相关则更新
- 洞3 收口：§3.8 文本族状态「MeasureString 🔗 内部用」补 ui 排版接线证据
- 主体文档 `ENGINE_UI_WIDGET_RENDER.md` §10 修订表每洞一行
