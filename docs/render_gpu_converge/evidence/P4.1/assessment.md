# P4.1 正确性：影响面评估 + 标准方案对齐（2026-11-09）

> 性质：只读评估，零代码改动。四个子项现状逐个 file:line 实测（本会话 HEAD `a11b8548`），
> 标准方案对照本机 Skia / Flutter(f-3.47, 737ba8f885d) 源码实证。
> **状态：停在「停下报告」节点，三个方向待用户拍板（见 §4），拍板前不写任何代码。**

## 0 直话

- 四个子项的洞全部实测还在，PLAN §6（2026-10-28 登记）的定位全部成立，行号有漂移但位置一致。
- 子项④ 的现状比 PLAN 登记的完整得多：Flutter 主路（Impeller）当前版本就是 spot 单 pass 无
  ambient，本库现状已是该模型——PLAN §6 该行的修法口径需要重新拍板。
- 子项① 的验收需要 GLES 4x 真窗金图，会被 go4x 整窗全黑洞（findings-go4x-black.md）直接卡死，
  处置必须先定。

## 1 四子项现状（file:line 实测，2026-11-09）

### ① MSAA>1 even-odd 裁剪（真洞）

- `render/internal/gpu/depth_clip.go:625`：`BuildClipMask` 在 `p.sampleCount > 1` 时直接跳过。
- stencil 管线只有 winding 规则（:269-318，IncrWrap/DecrWrap，注释自述 "non-zero winding"）；
  even-odd 孔洞只能靠 sampleCount==1 的 R8 coverage mask 挖孔（:601-627 注释 "even-odd holes
  are carved out ... Skipped when sampleCount>1"）。
- **与真值差异**：MSAA>1 时 even-odd 裁剪的孔洞区域 stencil≠0（winding 非零）→ depth gate
  保留内容 → 孔洞内容被错误保留（裁剪画面错，非模糊问题）。
- MSAA>1 时连普通 path 裁剪的 AA mask 也没有（同 :625 跳过），边缘 AA 只有 4x 子样本粒度。

### ② 分数裁剪 AA（真洞）

- `render/context_clip.go:519-522`：scissor 用 `math.Floor`/`math.Ceil` 外扩取整。
  非整数裁剪矩形（如 x=10.5, w=30.3）→ scissor 外扩每边 ≤1px，溢出内容**无 AA 硬边显示**。
- 纯矩形裁剪走硬件 scissor（整数栅格，永远硬边，`clip/stack.go:458` `IsRectOnly`）；
  RRect 已有 SDF AA（`clip/stack.go:425` `rrectCoverage` 0.75px smoothstep，`:471`
  `IsRRectOnly` 注释 "scissor bounding box + analytic SDF"）。
- CPU 路径 `clipStack.Coverage()` 有真 AA（`rrectCoverage` 同款）→ GPU/CPU 在非整数矩形
  边缘不一致（P4.2 模糊一致的根因之一在此）。

### ③ 顶点网格 alpha/blend（真洞，接线不是新管线）

- `render/internal/gpu/gpu_render_context.go:1113`（`QueueColoredMesh`）/`:1240`
  （`QueueColoredMeshIndexed`）构造的 `ConvexDrawCommand` **从不填 `BlendMode` 字段**
  （:1222-1227 只填 Color/SkipAA/TriangleList/PackedVerts）。
- `render/vertices_mesh.go:26`（`DrawVertices`）/`:498`（`DrawMesh`）：只取
  `solidColorFromPaint(c.paint)` 实色；**有逐顶点色（useVC）时 paint alpha 完全被忽略**，
  `Paint.BlendMode` 设任何值都无效。CPU 路径 `drawVerticesGouraudCPU`（:131）固定 src-over。
- 底座全在：`ConvexDrawCommand.BlendMode` 字段（`convex_renderer.go:77-79`）、
  `pipelineForBlend`（:511）、命令分发 `mode := cmd.BlendMode`（:1184）——与 P3.9a 复核
  结论一致（evidence/P3.9a/assessment.md）：**是接线不是新管线**。
- **与真值差异**：Paint alpha/BlendMode 对顶点网格全无效 = 公开 Paint API 承诺的能力
  在 mesh 路径不存在。

### ④ DrawShadow（现状比 PLAN §6 登记的完整，口径需重新拍板）

- `render/context_misc_draw.go:222-269` + `shadowSpotColor`（:275-297）：
  tonal 公式（0.25 系数 + alphaAdjust/colorAlpha/greyscaleAlpha/unpremul）**逐字对齐**
  Flutter Impeller `dl_dispatcher.cc:879-904` 的 ComputeTonalColors spot 路径；
  抬升 `offsetY = elevation*dpr/scaleY`（:253）与模糊 `blur = (800/600)*elevation*dpr`（:254）
  = Impeller occluder_z / kLightRadius(800/600) 模型（dl_dispatcher.cc:906-924）；
  PushLayerIsolated 隔离防阴影含遮挡物。
- 投影非仿射时：只计数 `projShadowDegrades`（:244-248），仍按 2×3 层参数近似照画
  （注释「剪影仍经 doFill 的投影映射绘制，参数近似，登记」）。
- **Flutter 3.47 实测（2026-11-09）**：Impeller 主路 = spot 单 pass，全树无 AmbientShadow
  实体（`impeller/entity/contents/` 无 ambient 类）；ambient(0.039)+spot(0.25) 双 pass 只在
  Skia 回退路径（`dl_sk_dispatcher.cc:338-357`，`SkShadowUtils::DrawShadow` + z(0,0,dpr*elev)
  + light (0,-1,1)）；透视剪影 Impeller 退化为方向光平移 scale=1.0（`dl_canvas.cc:46-58`
  GetDirectionalParams），真逐顶点透视剪影只在 Skia SkShadowTessellator（不在 Flutter 树内）。
- **与 PLAN §6 登记的口径差异**：PLAN 写「按 Skia 模型补 ambient+透视剪影」，但 Flutter
  主路已放弃 ambient、剪影已退化为方向光平移——本库现状已是 Impeller 同款。
  **真缺的只有一处**：非仿射投影下的剪影精确化（或显式降级）。

## 2 标准方案对齐（Skia / Flutter 源码实证）

| 子项 | 标准实现 | 证据 |
|---|---|---|
| ① MSAA even-odd | stencil **INVERT**（每 fragment 翻转底位=奇偶计数，盖奇数次=1 画、偶次=0 镂空）；winding 用 IncWrap/DecWrap；coverage mask 只给非 MSAA（numSamples<=1 且需 AA）用 | skia `GrPathStencilSettings.h:18-26`（gEOStencilPass=kInvert）/`:29-37`（gEOColorPass=kNotEqual）；`ClipStack.cpp:1528-1546`（分支条件）；`StencilMaskHelper.cpp:320-321`（"kMSAA is the only type of AA on stencil"）；tessellate 同款 `GrPathTessellationShader.h:78-85`；GL 映射 `GrGLGpu.cpp:2642` GR_GL_INVERT |
| ② 分数裁剪 | 非整数矩形不走 scissor，走 analytic coverage/SDF；整数矩形纯 scissor | skia ClipStack 总策略 `ClipStack.cpp:249-252`（analytic FP → stencil → atlas → SW mask） |
| ③ mesh alpha/blend | vertex colors 预乘；paint alpha 在 primitive blend 结果上**后乘**（ModulateRGBA(paintAlpha)）；对 framebuffer 的最终混合用 paint 自己的 blender；高级 blend 走 shader/dst-read（本库拍板=现有 layer 通路，语义等价） | skia `SkGr.cpp:452-478`（"paint's alpha is applied after the paint/primitive color blend"）；raster `SkRasterPipelineBlitter.cpp:185-188`；graphite `PaintParams.cpp:358-380` |
| ④ shadow | A=Skia 双 pass（ambient 0.039 + spot 0.25，tonal 色对，zRatio 方向光平移）；B=Impeller spot 单 pass（现状） | flutter `dl_sk_dispatcher.cc:338-357`（A）；`dl_dispatcher.cc:869-928`（B）；常量 `dl_canvas.h:215-216`（kShadowLightHeight=600/kShadowLightRadius=800）、`dl_canvas.cc:17-28`（ambient 公式） |

本库底座可用性（①）：hal `StencilOperationInvert` 已有（`gpu/types/render.go:481-482`），
GLES 已映射 `gl.INVERT`（`gpu/gwgpu/gles/command_execute.go:1267`，convert_test.go:306 断言）。

## 3 修法草案（拍板后按批细化 + 先红后绿）

1. **③ 接线**：`toPremul` 乘 paint alpha（VC/solid 两路）；`DrawVertices`/`DrawMesh` 传
   `paintBlendMode(c.paint)` 进 Queue 命令 → `ConvexDrawCommand.BlendMode` → :1184 分发自动走
   `pipelineForBlend`；高级 blend（非 Porter-Duff）走现有 layer 通路（F14 基座，1e2fa723）；
   CPU 路径按 paint.BlendMode/alpha 同步。
2. **② 分流升级**：rect-only 判定里检测 bounds 是否整数；非整数 → 升级走 RRect SDF 通路
   （radius=0，scissor 仍作外框粗裁）；整数矩形保持纯 scissor。
3. **④ 按拍板**：B=对齐 Impeller（维持 spot 单 pass，补非仿射投影剪影精确化/显式降级，
   回写 PLAN §6）；A=补 ambient+spot 双 pass（tonal 色对 + zRatio 平移 + ambient blur 公式）。
4. **① + go4x**：even-odd stencil-invert 管线（对照 gEOStencilPass/gEOColorPass，
   sampleCount>1 时启用；sampleCount==1 保持现有 mask 路）；go4x 洞的定位/修复与验收
   路径在拍板后细化。

## 4 待拍板三事项（停下报告，2026-11-09 提问未获答复，挂起等待）

| # | 问题 | 选项 | 会话推荐 |
|---|---|---|---|
| D1 | go4x 整窗全黑洞处置（卡子项① GLES 4x 验收） | a) 修进 P4.1①（同一片代码路径，MSAA>1 + GLES resolve/readback）b) 另立前置任务专修 c) 继续登记不可验 | a |
| D2 | ④ DrawShadow 标准口径 | a) 对齐 Impeller spot 单 pass（回写 PLAN §6）b) 对齐 Skia 补 ambient 双 pass | a |
| D3 | 拆批顺序 | a) ③→②→④→①（每批独立红绿+证据）b) 一次全做 c) 自定义 | a |

**拍板前不动手写任何代码；P4.1 状态保持 ⬜；§2.1 载体登记等拆批定后按 §4.2 填写。**

## 5 影响面与风险

- ①：动 `depth_clip.go` 管线创建与 RecordDraw 分流（sampleCount>1 新增 even-odd 管线）；
  native 4x 与 go 4x 双端都要过金图（go 4x 依赖 D1）；P3.9b 已实测 4x 显存峰值 2.35×/p95
  破线——本子项只加 stencil 管线不加纹理，显存增量可控，但需跑 P3.9b 同款门禁复核。
- ②：动 `clip/stack.go`/`context_clip.go` 分流判定——整数矩形行为必须零变化（金图回归），
  非整数矩形从硬边变为 SDF AA（**这是行为变化点，也是本子项的验收画面**）。
- ③：动公开行为（Paint alpha/BlendMode 开始对 mesh 生效）——既有示例里 mesh 用了非默认
  paint 的地方画面会变（全仓搜 DrawVertices/DrawMesh 调用点核对）；mesh gradient（P3.9/
  P3.9a）自己管理顶点色不经 Paint，预期不受影响，回归确认。
- ④：按 D2 选 a 则画面零变化（现状已是），选 b 则所有 DrawShadow 消费画面变（阴影变淡，
  ambient 补底）。
- go4x：修的话动 `gpu/gwgpu/gles/command_encoder.go`（resolve/readback），影响 GLES 后端
  全部 MSAA 窗（P3.9b 的 go4x 登记、未来所有 GLES 4x 场景）。
