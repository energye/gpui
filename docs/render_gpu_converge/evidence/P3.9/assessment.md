# P3.9 mesh gradient 影响面评估（2026-11-09 开工登记）

> 性质：开工前的只读评估，零代码。真源：`docs/ENGINE_RENDER_GPU_CONVERGE_TASKS.md` §2.1/§3 P3/§4.1/§4.2，`docs/ENGINE_RENDER_GPU_CONVERGE_PLAN.md` §5.1/§7.4。
> 口径：所有结论带 file:line；标准形态以本机 Skia 源码为准，不照抄台账措辞。

## 0 直话

- 仓里没有 mesh gradient。要做的不是“给线性/径向渐变再添一档”，而是新增一个 **patch 绘制原语**：每个 patch 带 4 个角角色色 + 12/16 个贝塞尔控制点，2×2 或 3×3 个 patch 拼成网格。
- Skia 的标准路径是 `drawPatch` → `SkPatchUtils::MakeVertices` → `drawVertices`（先把 patch 细分成带逐顶点色的三角形，再走普通顶点网格）。本仓 `DrawVertices/DrawMesh` 的彩色三角网格双路径已经具备，**不需要新 WGSL 管线**；复用现有 mesh 管线才是标准对齐，新增管线反而是重复实现。
- “2×2/3×3 patch”指 **patch 网格尺寸**，不是控制点数量：2×2 网格 = 4 个 patch / 9 个共享顶点；3×3 网格 = 9 个 patch / 16 个共享顶点。Android Compose 的 MeshGradient 也是这个口径（`rows×columns` 个 patch，`(rows+1)×(columns+1)` 个顶点，1×1 网格就是 4 个顶点 1 个 patch）。
- “bicubic/coons 风格”在源码里对应两种 patch 曲面：
  - **Coons**：12 个控制点（4 条三次边界），Skia `drawPatch` 的栅格实现只有这一种；
  - **Tensor-product / bicubic**：16 个控制点（4×4 控制格），PDF 的 patch mesh type 7；Skia PDF 只登记了枚举，没有栅格实现。
  两者都是 patch 几何基；patch 内的**颜色**在 Skia 是 4 个角角色的双线性插值（bilerp），不是 Catmull-Rom。Compose `hasBicubicColor` 的 Catmull-Rom 是 Compose 自己的扩展，先不混进来（见 §5 D2）。

## 1 任务真源与开工条件

### 1.1 任务定义
- `TASKS.md:349`：P3.9 mesh gradient（Skia 口径 2×2/3×3 patch：角角色 + bicubic/coons 风格；2026-11-02 拍板 D2 从 P3.6 剔出单列），依赖 P3.0，状态 ⬜，证据 `evidence/P3.9/`。
- `PLAN.md:151`：归属 render，双后端；现状“全仓零命中”，既有 `render.Mesh` 是逐顶点色 Gouraud，不是 patch。
- `PLAN.md:265`：排在 P3.8 之后，依赖 P3.0。
- `RENDER_API_CATALOG.md:1019`：预留 `MeshPatch`（12/16 控制点 + 角角色 + `MeshPatchStyle{Bicubic,Coons}`）+ `Context.DrawMeshPatch` 类绘制口 + recording 命令 + CPU 孪生；该行还写了“新 WGSL 管线 + GrMesh/SkMeshDraw”，**这两处与源码不符，按 §1.2 修正**。

### 1.2 Skia / PDF 源码实证（本机 `/home/yanghy/app/projects/gogpu/skia`）
- `include/core/SkCanvas.h:2154-2155`：`drawPatch(const SkPoint cubics[12], const SkColor colors[4], const SkPoint texCoords[4], SkBlendMode mode, const SkPaint& paint)`。12 个控制点 + 4 个角角色色 + 可选 4 个纹理坐标；注释明确颜色/纹理按 TL、TR、BR、BL 角顺序。
- `src/core/SkCanvas.cpp:2624-2657`：`drawPatch` 只做空指针/非有限包围盒/快速裁剪检查，然后进 device；`clean_paint_for_drawVertices` 只清掉 mask filter/path effect（`src/core/SkCanvas.cpp:2241-2246`），**AA 也被 drawVertices 忽略**。
- `src/core/SkDevice.cpp:154-162`：标准落地就是 `SkPatchUtils::GetLevelOfDetail` → `MakeVertices` → `drawVertices`。**没有 patch 专用 shader/pipeline。**
- `src/utils/SkPatchUtils.h:24-28`：`kNumCtrlPts=12`、`kNumCorners=4`、`kNumPtsCubic=4`。
- `src/utils/SkPatchUtils.cpp:36-64`：控制点顺序为顺时针的 top(0..3)、right(3..6)、bottom(9..6 反向)、left(0,11,10,9)；角角色枚举为 `kTopLeft_Corner=0, kTopRight, kBottomRight, kBottomLeft`。
- `src/utils/SkPatchUtils.cpp:182-210`：LOD = 四条边界三次曲线在设备空间的折线长度 / 10，X/Y 各自取 max，且**下限 8**；非有限长度返回 (0,0)（不画）。
- `src/utils/SkPatchUtils.cpp:279-289`：顶点/索引封顶——顶点数 >10000 或 LOD >200 时按权重压到最大 200，保证索引不超过 60000（`lodX*lodY*6`）。
- `src/utils/SkPatchUtils.cpp:337-350`：Coons 曲面公式 `S(u,v)=s0+s1-s2`：s0=上下边界按 v 线性插值，s1=左右边界按 u 线性插值，s2=四角双线性。
- `src/utils/SkPatchUtils.cpp:352-357`：颜色 = 四角颜色 **bilerp**；`skcolor_to_float` 转入 premul 空间（`src/utils/SkPatchUtils.cpp:240-250`），所以半透明角色的插值应在 premul 域做。
- `src/utils/SkPatchUtils.cpp:372-379`：每个细分格出两个三角形，索引顺序固定。
- `src/pdf/SkPDFGradientShader.cpp:755-762`：PDF shading type 4/5/6/7 = FreeFormGouraud / LatticeFormGouraud / **CoonsPatchMesh(6)** / **TensorProductPatchMesh(7)**。Coons=12 控制点，TensorProduct=16 控制点；后者就是 bicubic 风格的标准家族。
- 结论：`drawPatch` 是单 patch 原语，Skia 没有“多 patch 网格”API；2×2/3×3 网格由调用方按 patch 逐个画。Compose MeshGradient 的 rows×columns patch / (rows+1)×(columns+1) 顶点是同一网格口径的产品化封装。

### 1.3 开工条件核对
- `§3 P3`：先补文档（能力定义、公开 API、双后端实现方式、验证方案），再实现。→ 本评估 + `design.md`。
- `§4.1`：新能力先写**会红的能力测试**（先红后绿），再实现。→ 测试文件与用例见 `design.md` §4，实际写代码时先落红测。
- `§4.2`：能力单测、真窗、金图容差、双后端四项登记到 §2.1。→ 本次开工登记同步填写。
- 依赖 P3.0 已 ✅；`evidence/P3.9/` 当前不存在（本评估创建）；全仓 Go 代码严格搜 `meshgrad|mesh gradient|DrawPatch|Coons` 仍 0 命中。
- 无阻塞项。GPU 真窗按 `AGENTS.md`：当前桌面只做单窗验收，`-auto-only` 跑完自行正常关闭，不并发、不硬杀。

## 2 现状审计（file:line）

### 2.1 已有可复用底座
- `render/vertices_atlas.go:739-743`：`Mesh{Positions []Point, Colors []RGBA, Indices []uint16}`；文档明确是逐顶点色三角形，不是 patch。
- `render/vertices_mesh.go:26-...`：`DrawVertices` 双路径——GPU 走 `QueueColoredMesh`，CPU 走真 Gouraud；`DrawVerticesEx` 在 `:353` 做非有限校验。
- `render/vertices_mesh.go:498-...`：`DrawMesh` 索引/三角列表双路径；GPU 走 `QueueColoredMeshIndexed`，CPU 展开索引后走 `DrawVertices`；`DrawMeshEx` 在 `:384` 做索引/非有限校验。
- `render/internal/gpu/gpu_render_context.go:1113`：`QueueColoredMesh` 把 straight RGBA premultiply 后写 12B 顶点（pos2+unorm8x4 color）；`:1240` `QueueColoredMeshIndexed` 保留唯一顶点 + uint16 索引。
- `render/internal/gpu/shaders/convex.wgsl:101-128`：`vs_mesh` 固定 coverage=1（SkipAA），`fs_main` 只乘 clip/mask coverage 后输出顶点色；**没有 paint alpha/blend 入口**。
- CPU 真 Gouraud：`render/vertices_mesh.go:145-...`，逐像素重心插值 + source-over + clip/mask，同样没有 paint alpha/blend。
- 录制面：`render/recording/command.go:100` `CmdDrawMesh`；`command_ops.go:213` `DrawMeshCommand`；`recorder_canvas_ops.go:283` `Recorder.DrawMesh`；`recorder.go:326` 回放分派；`backend.go:179` `Backend.DrawMesh`；`backend_canvas.go:410` `CanvasBackend.DrawMesh`；`bounds.go:163` 包围盒。
- `render/canvas.go:160`：`Canvas` 接口的 `DrawMesh`；`drawPatch` 若要 Canvas/Recorder 双实现，需要同样的接口位。

### 2.2 缺口
- 没有任何 patch 类型、控制点布局、LOD、Coons/TensorProduct 求值、patch 细分到 `Mesh` 的代码。
- 没有 patch 的录制命令；`Canvas` 没有 patch 绘制口。
- 没有 mesh gradient 真窗/金图/像素探针。
- 既有 mesh 路径的 paint alpha/blend 缺口是 P4.1 在册项（“顶点网格 alpha/blend”），不是 P3.9 新引入；patch 的逐角 alpha 仍可用（顶点色带 alpha）。

## 3 影响面评估

| 层 | 文件 | 改动 | 风险/说明 |
|---|---|---|---|
| render 主包 | 新 `render/mesh_patch.go` | `MeshPatchStyle` + 2 常量、`MeshPatch`、`Context.DrawMeshPatch/DrawMeshPatchEx`、LOD/Coons/TensorProduct 细分、premul 角色 bilerp | 纯 CPU 几何；无后端类型；无新 shader |
| render 主包 | `render/canvas.go` | `Canvas.DrawMeshPatch` 接口位 | `*Context` 与 `Recorder` 同步实现 |
| render 主包 | `render/vertices_mesh.go`（可选） | 若 Ex 需要复用校验/结果类型，只加调用，不改既有 DrawMesh/DrawVertices 行为 | 存量金图零变化 |
| render/recording | `command.go`/`command_ops.go`/`recorder_canvas_ops.go`/`recorder.go`/`backend.go`/`backend_canvas.go`/`bounds.go` | 新增 `CmdDrawMeshPatch` + `DrawMeshPatchCommand` + Recorder/Backend/CanvasBackend 方法 + 包围盒 + 测试 stub | 只增命令；旧命令/回放不变 |
| 测试 | `render/mesh_patch_test.go`、`render/mesh_patch_gpu_test.go`、`render/testdata/mesh_patch_cases.json`、`render/recording/mesh_patch_test.go` | 先红后绿；数据入 testdata | 按测试文件逐个跑 |
| 真窗 | 新 `examples/ui_wr_p39_meshgradient/` | 2×2 Coons 网格、3×3 Coons 网格、bicubic 对照、角角色/alpha 卡；`-auto-only` + 常驻人工 | 满窗内容按 P3.5 窗先例；单窗验收 |
| 文档 | `docs/RENDER_API_CATALOG.md`、TASKS §2/§2.1/§7、`evidence/P3.9/` | 公开 API 总账 + 状态 + 证据 | apidoc rc=0 |

**不做**：
- 不加 `gpu/hal` 接口、不改 WGSL、不改 shader 翻译器（复用现有 mesh 管线）。
- 不加 `ui/kit` 组件、不改 2.5D 引擎。
- 不在示例层实现 patch 行为；示例只构造 patch 数据并调 `render` 公开口。

## 4 验证方案（详见 `design.md`）

- 能力单测：Coons 控制点顺序/角角色、平面恒等（直边矩形退化为双线性）、LOD 下限/封顶、premul 角色 bilerp、TensorProduct 4×4 曲面值、2×2/3×3 网格共享角一致、非法点数/非有限拒绝、录制命令往返。
- 真窗：`examples/ui_wr_p39_meshgradient`，逻辑探针 + 像素探针 + 金图逐位（P0.1 容差 0）；native 与 go/GL 各跑，metal 不可验写原因。
- 负向对照：示例 env `GPUI_P39_BREAK=swap-corners` 交换 TL/BR 角角色色 → 像素探针/金图必须红；还原后绿。
- 三证据：逻辑探针 JSON、真窗 PNG、金图 diff 计数 + 容差结论；单测逐文件 PASS 日志；双后端结果。

## 5 拍板结果（2026-11-09，用户确认）

- **D1 = A 单 patch 原语**（`MeshPatch` + `Context.DrawMeshPatch`，示例拼 2×2/3×3 网格）。
- **D2 = A 16 点 TensorProduct/bicubic**（Coons=12 点 / bicubic=16 点；颜色按 Skia 四角 bilerp，不做 Catmull-Rom）。
- **D3 = A paint alpha/blend 留给 P4.1**（P3.9 只做逐角 alpha）。

## 6 结论

- 能力确实缺失，标准形态明确：patch 细分到现有彩色顶点网格，不新建 shader。
- 影响面集中在 render 主包 + recording + 一扇新真窗，不动 gpu/hal/WGSL/引擎/组件。
- 开工条件满足；按 §4.1 先写红测。方向待 D1-D3 拍板后动代码。
