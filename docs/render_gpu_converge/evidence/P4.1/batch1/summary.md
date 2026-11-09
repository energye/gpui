# P4.1 批 1（子项③ 顶点网格 paint alpha/blend 接线）收口记录（2026-11-10）

> 拆批依据：D3a（2026-11-10 用户拍板 ③→②→④→①）。标准口径：Skia「paint alpha 在
> primitive blend 后乘」（SkGr.cpp:452-478）+ Paint.BlendMode 为对画面的最终混合；
> 高级混合走现有 layer/dual-tex（P3.9a D3 拍板，不为 mesh 新建混合管线）。

## 0 直话

- **四个能力点三个像素收口**：paint alpha 接线（真窗 A/D 卡探针 + 单测 12/12）✓、
  advanced blend 层封装（C 卡 + 单测）✓、CPU Porter-Duff（单测 + 引擎洞 #2 修复）✓。
- **一个未收口**：真窗 B 卡（GPU 窗面上的 Porter-Duff 层合成）双后端一致 FAIL
  （got=纯蓝底，mesh 层内容丢失）——引擎洞 #3（PopLayer 分派缺口）的 CPU 侧已修
  已验（headless 单测过），**GPU 侧的 texture blend blit 未验证成功**（方案落地了
  三段：GPUTextureDrawCommand.BlendMode + ensureBlitBlendPipeline +
  QueueGPUTextureDrawBlend，启用时真窗/headless 均丢内容，已回退到 CPU materialize
  兜底并禁用 GPU blit，代码保留待诊断）。**批 1 状态 🟡，不标 ✅**。
- 顺带发现并如实登记第三个洞：**mesh×固定函数 blend 管线组合洞**——fill 命令同链
  生效（TestP12GPUFixedPixel_BlendPlus PASS）、packed mesh 命令管线链逐环正确但画面
  =纯 src；未定位到行号，按动手修门槛不猜着修；mesh 的非 Normal blend 一律走层封装
  （premul 语义与直传等价），固定函数直传恢复归 P4.3 性能线跟踪。

## 1 改动清单（全部在 render 层，无公开 API 变化，apidoc rc=0）

| 文件 | 改动 |
|---|---|
| `render/vertices_mesh.go` | DrawVertices/DrawMesh 接线：paint alpha 乘进顶点色（scaleMeshColorsAlpha，grow-only scratch）；withMeshBlend 统一 blend 决策（Normal 直画 / 非 Normal 层封装）；QueueColoredMesh* 传 blendMode |
| `render/context_damage.go` | gpuContextOps 接口：QueueColoredMesh*/QueueGPUTextureDraw 签名加 blendMode、新增 QueueGPUTextureDrawBlend（内部接口，非公开 API） |
| `render/internal/gpu/gpu_render_context.go` | 两 Queue 实现填 ConvexDrawCommand.BlendMode（buildConvexBlendRangesIndexed :1184 分发自动生效）；QueueGPUTextureDrawBlend 实现 |
| `render/context.go` | Context.meshAlphaScratch 字段 |
| `render/context_layer_blend.go` | **引擎洞 #2 修复**：compositeLayer 加 Porter-Duff 分支 → compositePremulPorterDuff（intBlend.GetBlendFunc，premul 域）；drawGPUPorterDuffLayer（GPU blit 辅助，待验证，当前未启用） |
| `render/context_layer_stack.go` | **引擎洞 #3 修复（CPU 路）**：PopLayer 加 canGPUPorterDuff 分派 → materialize + CPU premul 合成（此前该类层内容直接丢失） |
| `render/internal/image/draw.go` 未改 | intImage.blend() 的 Porter-Duff 缺分支**不改**（blend→image 依赖方向禁止反向 import；在 render 层 compositePremulPorterDuff 接同一标准实现） |
| `render/internal/gpu/gpu_types.go` | GPUTextureDrawCommand.BlendMode 字段（GPU blit 底座，未启用） |
| `render/internal/gpu/image_pipeline.go` | canMergeGPUTextureDraw 纳入 BlendMode；ensureBlitBlendPipeline（blit pass 兼容 1x 无 depth 变体）；RecordBlitDraws blend 分派（待验证，未启用路径） |
| `render/internal/gpu/opt22_indexed_mesh_test.go` | 调用点同步新签名 |

## 2 验证载体与证据（evidence/P4.1/batch1/）

- **能力单测**：`render/vertices_paint_blend_test.go` :: TestVerticesPaintBlendCasesCPU /
  CasesGPU / CPUMatchesGPU（4 场景 × 3 套）+ 数据 `render/testdata/verts_paint_blend_cases.json`
  （先红后绿：alpha/Plus/Multiply 三场景实现前红，实现后 12/12 绿；solid_no_vc_alpha 为
  回归锁定）。逐文件日志 `unit-tests.txt`。
- **真窗**：`examples/ui_wr_p41_mesh_paint_blend`（-auto-only 门禁 + 常驻人工二合一；
  人工窗 -manual-seconds 3 正常退出 rc=0，`manual-window.txt`）。
  四卡：A alpha 0.5 Gouraud / B BlendPlus / C BlendMultiply / D solid alpha。
- **像素探针**：A (170,170,170)、B (85,85,255)、C (43,43,43)、D (255,128,128) + 各卡
  底色锚点。**A/C/D 双后端 ok；B 双后端 FAIL（#3 GPU 路径，见 §0）**。
- **金图**：`examples/ui_wr_p41_mesh_paint_blend/testdata/goldens/p41_mesh_paint_blend_final.png`
  （native）与 `..._go.png`（go/GL），**双后端各自 verify 0.0000% / 763584 px**（逐位，
  P0.1 容差 0）；sha256 见 `golden-sha256.txt`。**注意：金图含 B 卡已知缺口（mesh 缺席），
  #3 GPU 路径收口后需重出金图**。
- **双后端**：webgpu（native）✅ 除 B 卡；gwgpu-gles（GPUI_BACKEND=go）✅ 除 B 卡（两后端
  B 卡同缺口、行为一致）；gwgpu-metal 不可验（无 macOS 真机）。门禁日志 `gate-native.txt` /
  `gate-go.txt`。
- **负向对照**（两路，红了还原复绿）：`GPUI_P41_BREAK=no-alpha` → A_alpha/D_solid 探针红
  （got=(85,85,85)/(255,0,0)）+ break_inactive logic 红、exit 1（`negative-no-alpha.txt`）；
  `GPUI_P41_BREAK=normal-blend` → B/C 探针红（got=(85,85,85)，即 src-over 退化）+ exit 1
  （`negative-normal-blend.txt`）。
- **回归**：render 包全量前后对比——既有 19 个失败集合逐名一致（基线 stash 验证），
  **零新增失败**；`-race`（TestVerticesPaintBlend|TestVerts|TestMeshGradient|TestMeshPatch）ok；
  `go vet ./render/...` 干净；apidoc rc=0（`apidoc.txt`）。

## 3 三个引擎洞登记（按 AGENTS「新问题归位」）

| # | 现象 | 定位 | 状态 |
|---|---|---|---|
| #2 | 层合成（PopLayer CPU 路）的 Porter-Duff 全家族静默退化 SourceOver——intImage.blend() 无该族分支（default→blendNormal），advanced 15 种正常 | `render/internal/image/draw.go blend()` 分派 + `render/context_layer_blend.go compositeLayer` | **✅ 已修已验**：render 层 compositePremulPorterDuff 接 intBlend.GetBlendFunc（blend→image 依赖方向不许反向，故在 render 层接同一标准实现）；单测 porter_duff_plus CPU/GPU 全绿 |
| #3 | PopLayer 分派缺口：Porter-Duff 非 Normal 的 GPU 层（内容在层 RT）既无 GPU 合成也无 CPU 回读——层内容静默丢失 | `render/context_layer_stack.go PopLayer`（canGPUComposite 只认 Normal/Copy、canGPUAdvanced 只认 15 种 advanced） | **🟡 半收口**：CPU materialize + premul 合成已修已验（headless 单测过）；GPU 窗面路径未收口——真窗 B 卡双后端一致 FAIL（got=纯蓝）。GPU blit 方案（GPUTextureDrawCommand.BlendMode + ensureBlitBlendPipeline + QueueGPUTextureDrawBlend）已落码但启用时丢内容（真窗/headless 均复现），已回退禁用，**待诊断**（嫌疑：blit pass 与 blend 管线的 pass 兼容性链、或 gpuTex tier 在 present 链的 flush 时机）。待办挂 P4.1 台账，收口后 B 卡探针+金图重验 |
| M | mesh×固定函数 blend 组合洞：fill 命令同链同管线 Plus 生效（TestP12GPUFixedPixel_BlendPlus），packed mesh 命令管线链逐环正确（build→pipelineForBlend→SetPipeline→Draw 全实证）但画面=纯 src | `render/internal/gpu/convex_renderer.go RecordDraws/pipelineForBlend` 与 packed mesh 展开链的组合，**未定位到确切行** | **登记不修**（动手修门槛：无行号不动手）。规避：mesh 非 Normal 一律层封装（withMeshBlend），premul 语义与直传等价；直传恢复归 P4.3（性能项） |

## 4 下一步

- 批 2（子项② 分数裁剪 AA）按 D3a 顺序开工；#3 GPU 路径与 B 卡收口挂在 P4.1 台账
  （可在批 4 前/后插队处理，不阻塞批 2/3）。
