# P3.9a 开工前全量复核：四阶段是否真缺 + 目标边界（2026-11-09）

> 依据：全仓 Go/WGSL 源码复核 + 本机 Skia 源码 + AndroidX Compose MeshGradient 源码（2026-11-09 拉取）。
> 性质：开工前只读评估，零代码。真源：`docs/ENGINE_RENDER_GPU_CONVERGE_TASKS.md` §2/§3 P3/§4.1/§4.2、`docs/ENGINE_RENDER_GPU_CONVERGE_PLAN.md` §5.1/§7.4。

## 0 直话

- **阶段 1（高层网格 + 切向量 + 颜色模型）真缺**：全仓没有 `MeshGradient` 高层类型、没有每顶点切向量/自动 C1 推断、没有 mesh 级 Catmull-Rom 颜色。P3.9 只有单 patch 内核（`render/mesh_patch.go:60`）。
- **阶段 2（paint alpha/blend）不是新管线，是 P4.1 接线**：`ConvexDrawCommand.BlendMode` 与 `pipelineForBlend` 已在（`render/internal/gpu/convex_renderer.go:77-79,511`），缺的是 `QueueColoredMesh` 没把 `Paint.BlendMode`/alpha 传进去。固定功能混合可直接接；高级混合走现有 layer/dual-tex。
- **阶段 3（通用 textured mesh）真缺，但只在你需要“图片变形/贴图 patch”时才是本目标**：4 角图片变形已有 `DrawImageQuad`/`TexturedQuadPipeline`；逐顶点 UV 只在文字/字形 quad 内部；`engine/anim.Attachment` 有 UV/三角形/权重数据但没有 renderer；`render.Mesh`/`DrawMesh`/recording 没有 UV。**不要做 mesh-gradient 专用贴图管线**；要做就做通用 textured triangle mesh，同时服务 mesh gradient 和 Spine mesh。
- **阶段 4（AA/颜色精度/合批/缓存）没有确认缺失**：
  - AA 已有全局 4x MSAA（`render/sample_count.go:24`、`render/internal/gpu/gpu_shared.go:802` 直接返回 `render.MSAASampleCount()`、mesh 管线 `convex_renderer.go:378` 使用 `sampleCount`）。默认 1x；不需要新写解析 AA，先做 4x 基线验证。
  - 颜色精度是 12B unorm8 顶点色（`convex_renderer.go:41-44`），没有 float 顶点色；但这是质量取舍，未见“必须改”的证据，先测 banding 再决定。
  - 合批/缓存：`meshCompact` 已把所有 mesh 顶点/索引拼进一个 buffer（`render_session.go:3335`），只是 draw range 仍按 patch；`PathGeometryCache` 等 LRU 已在（`path_geometry_cache.go:116`）。先跑全屏 3×3/4×4 基准，再决定是否合并 range/加 mesh 缓存。
- **阶段 5 不在本会话**（用户明确：其它任务开）。

## 1 阶段 1 为什么真缺（源码实证）

- **高层类型零命中**：全仓搜 `MeshGradient`/`hasBicubicColor`/`smoothsColors`/`leftControlPoint` 无 Go 实现；只有 P3.9 单 patch（`render/mesh_patch.go:60`）和示例里的手工拼网格（`examples/ui_wr_p39_meshgradient/main.go:112-195`）。
- **切向量零命中**：`render/path_metrics.go:47` 的 `TangentAt` 是路径弧长切线，不是 patch 顶点切向量；没有“每顶点四切向量 + 自动推断”的数据模型。
- **mesh 级 Catmull-Rom 零命中**：仓里 Catmull-Rom 只在图片采样——`render/internal/image/interp.go:122/199`（CPU 双三次）、`render/internal/gpu/shaders/textured_quad_bicubic.wgsl:8`（GPU 图片采样）、`render/text/color_raster.go:166`（彩色字形缩放）。没有网格颜色插值。
- **Spine mesh 不是 mesh gradient**：`engine/anim/skeleton.go:96` 的 `Attachment` 有 `Verts/UVs/Triangles/Weights`，但 `engine/anim` 不 import render、没有 renderer；它是骨骼动画数据，不是高层渐变 API。
- **kit 没有 mesh**：`ui/kit` 全仓搜 `mesh` 零命中；只有线性渐变 prim（`ui/kit/internal/prim/decor.go:204`）。

## 2 阶段 2/3/4 的复核结论

### 阶段 2：P4.1 接线（不要新管线）
- `ConvexDrawCommand` 有 `BlendMode`（`convex_renderer.go:77-79`）、`pipelineForBlend`（`:511`）、`buildConvexBlendRangesIndexed`（`:1170`）。
- `QueueColoredMesh`/`QueueColoredMeshIndexed`（`gpu_render_context.go:1113/1240`）不接 `Paint.BlendMode`/alpha；`DrawMesh` 忽略 `c.paint`。
- 结论：**能力在 API 行为上缺，实现上是接线**；归 P4.1。高级混合（dual-tex）走现有 layer 路径，不要给 mesh 新建高级混合管线。

### 阶段 3：通用 textured mesh（真缺，但按需）
- 已有 4 角图片变形：`DrawImageQuad`、`TexturedQuadPipeline`、`textured_quad*.wgsl`、`ImageDrawCommand`（4 角 + UV 矩形，`gpu_render_context.go:1392`）。
- 已有逐顶点 UV 的内部管线：`text_pipeline.go:484` `TextVertex{U,V}`、`glyph_mask_pipeline.go`、`glyph_color_atlas.go`；都是文字/字形专用。
- 已有 UV/三角形数据但无 renderer：`engine/anim/skeleton.go:96` `Attachment`。
- 已有采样算法：`internal/image/interp.go`（CPU 双线性/双三次）、`textured_quad_bicubic.wgsl`（GPU Catmull-Rom）、`ImageCache`/`res.Cache`（纹理上传与预算）。
- 结论：**没有通用公开 textured triangle mesh**；如果目标包含图片变形/贴图 patch，需新增一个通用管线，复用上述采样/缓存，不要做 mesh-gradient 专用版本；如果只做彩色渐变，阶段 3 不做。

### 阶段 4：没有确认缺失
- AA：全局 4x MSAA 已在，mesh 管线使用 `sampleCount`；默认 1x。需要的是“4x 基线验证 + 是否做 per-mesh opt-in”，不是新算法。
- 颜色精度：当前 12B unorm8；float 顶点色缺失，但先测 banding。
- 合批：`meshCompact` 已合 buffer；draw range 仍按 patch；先测 draw call/帧时。
- 缓存：路径/描边/dash 有 LRU，mesh 没有；可复用 LRU，但先测静态/动画场景是否真的需要。
- 结论：**阶段 4 = 验证/按需优化，不是必做实现**；不进 P3.9a 范围。

## 3 P3.9a 目标边界（本阶段只做这些）

- 高层 `MeshGradient`：rows×columns 个 patch、(rows+1)×(columns+1) 顶点、每顶点位置/颜色/可选四切向量、Bilinear/CatmullRom 颜色模型、归一化/绝对坐标（D3 拍板）。
- 切向量自动推断：逐字对齐 AndroidX Compose `MeshGradientConfig.inferBezierControlPointsIfRequired`（normalized neighbor vector × 0.33 × 到邻居距离；左/上取负；边界邻居=自身、距离 0 → 0）。
- 几何默认 Coons 12 点（Compose 的 Bezier patch 形状）；TensorProduct 16 点仍由低层 `MeshPatch` 直接用，不在高层网格默认路径（D5 拍板）。
- 颜色：Bilinear 复用 P3.9 四角 bilerp；CatmullRom 走网格级颜色求值，输出逐顶点色 `Mesh`。
- 一次绘制合并成一个 indexed `Mesh`（一个 draw call），不逐 patch 调 `DrawMeshPatch`。
- Canvas/Recorder 同步；录制存高层网格，回放期重新细分（D4 拍板）。

**明确不做**：纹理坐标/图片变形（阶段 3）、paint alpha/blend（P4.1）、AA 新算法（用 MSAA）、HDR/float target、kit 组件、PDF/SVG、Spine renderer、新的 mesh 专用 shader。

## 4 影响面

| 层 | 文件 | 改动 |
|---|---|---|
| render 主包 | 新 `render/mesh_gradient.go` | `MeshGradient`/`MeshGradientVertex`/`MeshGradientColorModel`、切向量推断、网格级细分、Catmull-Rom 颜色、`Context.DrawMeshGradient/Ex` |
| render 主包 | `render/mesh_patch.go` | 抽出可复用的“按颜色函数细分”内部口（不改 P3.9 行为） |
| render 主包 | `render/canvas.go` | `Canvas.DrawMeshGradient` |
| render/recording | command/recorder/backend/bounds | `DrawMeshGradientCommand` + `Backend.DrawMeshGradient`（若 D4 选录制高层网格） |
| 测试 | `render/mesh_gradient_test.go`、`render/mesh_gradient_gpu_test.go`、`render/recording/mesh_gradient_test.go`、`render/testdata/mesh_gradient_cases.json` | 先红后绿；数据入 testdata |
| 真窗 | `examples/ui_wr_p39a_meshgradient_grid` | 1×1/2×2/3×3/4×4、切向量自动/手动、Bilinear/CatmullRom、接缝/颜色探针、金图 |
| 文档 | TASKS/PLAN/evidence/P3.9a | 状态/载体/流水；catalog 等 API 实现后再同步 |

**不动**：`gpu/hal`、WGSL、shader 翻译器、贴图管线、`ui/kit`、engine、PDF/SVG。

## 5 开工条件与验证载体（§4.1/§4.2）

- 依赖 P3.9 ✅；本评估 + `design.md` 落盘；§2.1 四项载体填齐；先写红测再实现。
- 能力单测：`render/mesh_gradient_test.go` :: `TestMeshGradientTangentInference` / `TestMeshGradientC1SharedEdges` / `TestMeshGradientBilinearMatchesPatches` / `TestMeshGradientCatmullRomColor` / `TestMeshGradientBoundarySizes` / `TestMeshGradientNormalizedSpace` / `TestMeshGradientInvalidShape` / `TestMeshGradientCTM`；`render/mesh_gradient_gpu_test.go` :: `TestMeshGradientGPUProbes` / `TestMeshGradientGPUMatchesCPU` / `TestMeshGradientGPUSeams`；`render/recording/mesh_gradient_test.go` :: `TestRecorderDrawMeshGradientRoundTrip` / `TestRecorderDrawMeshGradientBounds`。
- 真窗：`examples/ui_wr_p39a_meshgradient_grid`（`-auto-only` + 常驻人工）。
- 金图：`examples/ui_wr_p39a_meshgradient_grid/testdata/goldens/p39a_meshgradient_grid_final.png`，容差 0（P0.1）；native/go 各自基线（若逐位不同）。
- 双后端：webgpu 待跑；gwgpu-gles 待跑；metal 不可验（无 macOS 真机）。
- 负向：`GPUI_P39A_BREAK=zero-tangents`（切线清零）→ 接缝/像素红；`GPUI_P39A_BREAK=swap-color-model` → CatmullRom/Bilinear 对照红；还原复绿。

## 6 拍板结果（2026-11-09，用户确认）

- **D0 = 阶段 3 暂不做**：本次只做彩色高层网格；纹理/图片变形需要时再单独立通用 textured mesh 任务，不与 P3.9a 混做。
- **D1-D3 = 推荐组合**：高层 `MeshGradient` + `Context.DrawMeshGradient/Ex` + Canvas/Recorder；切向量 `*Point` 四字段（nil=自动推断，显式零向量=尖点）；坐标默认绝对，`Space=Normalized` 时映射 `Bounds`。
- **D4 = 新 `DrawMeshGradientCommand`**：存高层网格，回放期按回放 CTM 重新细分。
- **D5 = 高层默认 Coons 12 点**：TensorProduct 16 点继续用低层 `MeshPatch`，不在高层默认路径。

## 7 结论

- 阶段 1 是真缺、值得做；阶段 2 归 P4.1 接线；阶段 3 是可选通用 textured mesh（按目标决定）；阶段 4 先验证/基准，不先写代码。
- P3.9a 只做彩色高层网格 + 切向量 + 颜色模型；复用 P3.9 内核与现有彩色 mesh，不新建 GPU 管线、不重复图片采样/缓存/滤镜/MSAA。
