# P3.7 批 1d · Canvas 级 4×4 栈并存式 — 影响面评估（2026-11-04，开工实测）

- 分支 `feat/p2-收敛整合`，HEAD `b28ff695`（批 1c 已提交）。工作区仅本评估文档新增，零代码变更。
- 方案依据 = `design.md` §2.4 + P2 探针 `p2-ctm-callsites.md`（39 个调用点分类已定）+ P3.5 批 3 D4 挂账。
- 标准对齐：渲染层对齐 Skia（`SkM44` 允许 CTM 带透视但 `SkCanvas` 对透视 CTM 的能力是**限定域**——图像/矩形走 persp2d，路径与文字有额外约束 ⇒ 并存式与 Skia 实际形态一致）；2.5D 侧 Godot 无 2D 透视（`Transform2D` 仿射-only，`renderer_canvas_cull.cpp`）⇒ Godot 对「要透视的内容」的答案是引擎侧算好投影喂渲染器——正是本批并存式的形态。

---

## 1 现状（全部实测到行号）

### 1.1 CTM 结构：纯 2×3 双层，无 4×4

| 环节 | 位置 | 实测 |
|---|---|---|
| 用户变换 | `render/context_transform.go:83-150` | `c.matrix`（2×3）；Translate/Scale/Rotate/Transform/SetTransform 全在 2×3 域 |
| 设备层 | `:174-179 totalMatrix()` | `deviceMatrix × matrix`（DPR 缩放），渲染边界统一入口 |
| 保存栈 | `render/context_layer_pass.go:37-57 Push / :63-105 Pop` | `c.stack` 只存 2×3 `matrix`（+clip 深度/mask/AA），无 4×4 位 |
| 仿射拒绝判定 | `render/matrix4.go:218 Affine()` | 非纯提升仿射返回 ok=false（P3.5 批 3 交付，非静默压扁的判定已在） |

### 1.2 39 个 `totalMatrix()` 调用点（P2 探针正本 `p2-ctm-callsites.md`，本轮抽核无误）

| 组 | 点 | 1d 处置（探针口径） |
|---|---|---|
| A 图像/quad（6） | `context_image_gpu.go:76`（tryGPUDrawImage 四角）、`:141`（prepareGPUTextureDraw=DrawGPUTexture*）、`:312/:394`（视频/GPU 纹理口）、`context_image_draw.go:141`（DrawImageEx CPU 双线性/双三次兜底）、`m4_quadimage.go:52`（DrawImageQuad） | **透视真消费**：投影映射四角+齐次权重进 1b/1c 已备的透视腿 |
| B1 图集批绘（3） | `vertices_atlas.go:46`（老 DrawAtlas）、`:263`（DrawAtlasEx GPU）、`:339`（CPU 腿） | 同上；仿射精灵在非仿射投影下自动升格为透视 quad（`QueueImageDrawQuadW`） |
| B2 网格（3） | `vertices_mesh.go:31`（DrawVertices）、`:439`（drawDeviceTriangle Invert）、`:475`（DrawMesh） | 半真：顶点经投影映射（直线仍直，几何正确）；逐顶点色插值保持仿射（登记近似） |
| C 裁剪（5） | `clip_op.go:40`、`context_clip.go:98,180,195,281` | 非仿射 ⇒ 投影后按路径裁剪降级（投影保持直线，矩形→投影 quad 路径化=精确）+ 计数 |
| D 路径填充/描边（3） | `context_gpu.go:503,526`（SDF/convex 快路判定）、`context_path.go:66`（展开） | 非仿射 ⇒ 展平后逐点投影映射 → 设备空间路径画（Skia 同款：先变换后造型）+ 计数 |
| E 文字（16） | `text_gpu.go` ×9、`text_dispatch.go` ×7 | 非仿射 ⇒ 跳过 + 具名计数（fail-closed；透视文字需 mesh warp，登记后续） |
| F 刷与杂项（3） | `brush_device_space.go:228`（设备空间刷烘焙）、`context_misc_draw.go:153`（阴影抬升比例）、`:218`（Invert） | 非仿射 ⇒ 各点降级/计数（见 §2.2 逐点） |

### 1.3 Canvas 接口面

- `render/canvas.go:40` `Canvas` 接口现 **109 方法**；实现者两个：`*Context`（`:187` 断言）与 `recording.Recorder`（`recorder_canvas_ops.go:35` 断言）⇒ 接口加 4 方法 ⇒ 两实现同批补齐（接口全量切换制）。
- 录制命令形态先例：`ConcatCommand{Matrix}`（`recording/command.go:261`，相对变换、回放组合到目标 CTM）。投影命令镜像此形态：`ProjectionCommand{Proj Matrix4}`（录制期取值、回放 `SetProjection`）——**Recorder 实现新方法=录命令，接口合规与录制保真一并解决**（不存在「推迟录制」选项，见 D-4）。
- `render/scene` 公开面 Affine-only，扩投影/材质编码按 design §4.5 归 **批 3e**（本批不动 scene）。

### 1.4 风险面与护栏

- **存量金图零变化是硬验收项**：投影层默认=恒等（零值）⇒ 9+27 个调用点全部走 `if c.projectionIsIdentity → 原代码路径`，零 Matrix4 数学、逐字节同旧。恒等护栏下存量风险=重构笔误，由全量真窗 + 逐文件 sweep 兜底。
- 非仿射投影下的所有新表态（C 降级/D 映射/E 跳过/F 降级）**只在新 API 的调用者身上生效**——现仓零调用者 ⇒ 无存量风险，行为可钉。

---

## 2 方案形态（待拍板后落实现）

### 2.1 投影层状态与 API

- `Context` 新增：`projection Matrix4` + `projStack []Matrix4` + `projectionIsIdentity bool` 快速位。
- 公开面（`Context` 方法 + `Canvas` 接口同批）：
  - `SetProjection(m Matrix4)`——替换当前投影层（不与 CTM 相乘；组合语义由调用方用 `Matrix4.Mul` 自行表达）；
  - `PushProjection(m Matrix4)`——保存当前投影层并替换；
  - `PopProjection()`——恢复上一投影层（空栈 = no-op）；
  - `Projection() Matrix4`——查询当前（恒等时返回 `Matrix4Identity()`）。
- 全链映射：**设备点 = totalMatrix(2×3) ∘ projection(4×4) 作用于用户点**——投影先映射（产出 mid 点 + 齐次 w），2×3 CTM 再映射 mid→设备（仿射不改 w）⇒ 最终权重=投影 w。2.5D 用法：`SetProjection(ProjectorMatrix4(...))` 后按世界坐标画精灵，窗口级 2×3 变换照常叠在外层。
- 与精灵/quad 自带 `Weights` 的合成：`w_final = w_proj × w_own`（齐次合成，数学精确）。

### 2.2 九个真消费点接入（全部带 identity 快速位护栏）

| 点 | 接入 |
|---|---|
| `m4_quadimage.go:52` DrawImageQuad | 角点经 `ProjectPoint`（任一角 behind-eye ⇒ 整 quad 拒绝，同 `Matrix4.ProjectQuad`/p35「眼后整块拒绝」语义）；`Weights_final = w_proj × opts.Weights`；GPU 走透视槽、CPU 走 `drawImageQuadCPUSplit(persp=true)` |
| `context_image_gpu.go:76` tryGPUDrawImage | DrawImage/DrawImageEx 四角投影映射；非仿射投影下自动走 `QueueImageDrawQuadW`（nearest 位保持；bicubic 在透视槽不可用 ⇒ 降级 bilinear + 计数，登记）；CPU 兜底改走 `drawImageQuadCPUSplit`（已有任意四角+权重 CPU 腿） |
| `vertices_atlas.go:46,263,339` 图集三口 | 仿射精灵（Perspective=false）：四角经投影映射+取 w ⇒ 非仿射投影下升格为 `QueueImageDrawQuadW` 透视 quad（这就是「批绘透视腿」的栈级形态）；透视精灵（1c）：`w_final = w_proj × sp.Weights`；CPU 腿同式 |
| GPU 纹理/视频三口（`:141/:312/:394`） | 投影口是设备矩形（`QueueGPUTextureDraw`/视频平面无 quad 形参），透视 quad 形态不存在 ⇒ 非仿射投影下降级为投影四角 AABB + 具名计数（拍板 D-3） |

### 2.3 27 个仿射点的非仿射表态（每组一个 helper + 计数器，全部带计数具名）

| 组 | 表态 |
|---|---|
| C 裁剪（5） | 裁剪几何（矩形/RRect/路径点）先经投影映射成设备路径 → 走既有路径裁剪机器（投影保直线 ⇒ 矩形→投影 quad 路径化=精确）+ 计数 `clip_projective_degrade` |
| D 填充/描边（3） | 用户路径展平 → 逐点投影映射 → 设备空间路径 → 以 identity CTM 走既有管线（Skia 同款先变换后造型）+ 计数 `path_projective_expand`；SDF/convex 快路自然绕行 |
| E 文字（16） | 跳过 + 计数 `text_projective_skipped`（fail-closed：不留错位画面；透视文字 mesh warp 登记后续专项） |
| F 刷与杂项（3） | `applyDeviceSpaceBrush`：非仿射 ⇒ 跳过设备空间烘焙（刷走普通路，相位近似）+ 计数；阴影抬升/Invert：非仿射 ⇒ 按 2×3 层（不含投影）计算 + 计数（投影只服务图像/批绘腿的既有口径，登记） |

### 2.4 录制面（随接口合规自然入批）

- `recording` 加 `ProjectionCommand{Proj Matrix4}` + `CmdProjection`；`Recorder.SetProjection/PushProjection/PopProjection/Projection` → 录制期取值；`CanvasBackend` 回放 → 调目标 `dc` 同名方法。
- `render/scene` 不动（批 3e）。

## 3 待拍板项

> **拍板结果（2026-11-04 实施时定，本段为准；下面的「推荐」保留当时的建议原文作历史）**
> - **D-1 = 并入 `Push()/Pop()`（非推荐的独立栈）**：投影层与 2×3 矩阵同拍保存/恢复（Skia `SkM44` 风格），实现见 `context_layer_pass.go` 的 Push/Pop + `context_projection.go` 的 `projRestore`。**实施补两条**：① 快照必须存**归一化值**（恒等态存 `Matrix4Identity()`），存 `c.projection` 原值会因零值 `Matrix4.IsIdentity()==false` 把画布污染成「投影已装 + 全零矩阵」，之后绘制全被映射到空（存量级回归，由 `TestD6` 抓到、新钉 `TestP37ProjectionZeroValueNeverActive` 咬住，反证 N10）；② `SetProjection(零值)` 按本文档口径归一为恒等=关闭。
> - **D-2 = 跳过 + 计数，且范围扩到「投影层非恒等即跳」**：链序定稿为 B 后文字族仍未接投影映射器，故不只挡非仿射——宁可不画也不画「半条链吃投影」的错位；描字两枚（`StrokeString`/`StrokeStringAnchored`）经实测（Fill 正常 2052px、描边画空）同族并入 fail-closed，共**九入口**。
> - **D-3 = AABB 降级 + 计数，实施中扩两处**：`DrawVideoSlot`/`DrawVideoPlanes` 眼后改为**不画且返回 true**（返回 false 会让上层 video bridge 兜底再画一遍未投影画面）；另 `DrawVideoFrame` 实测转调 `DrawImageEx` 吃**透视 quad 腿**（优于 AABB），钉按事实写。
> - **D-4 = 随接口合规自然入批**（未推迟）：`ProjectionCommand{Mode,Proj}` 录绝对值、回放只读，`Backend` 契约 38→39。
> - **追加两项实施期拍板**：① **链序 = B**（设备 = `deviceMatrix ∘ 投影 ∘ 用户矩阵`；定稿前两链同一矩形落点差 3371 像素）；② **对外观测口** `ProjectionStats()/ResetProjectionStats()` 且计数按单一语义拆名（`Misc` 拆成 `ShadowApprox`/`PatternApprox`，共八枚）；③ 测试 fixture 可由代码生成（输入图元非标准答案数据），既有像素钉不变。
> - **仍属本文档之外的遗留**：老 `DrawAtlas` 与 `DrawAtlasEx` 双入口的收敛（批 2「副本改委托」同类项），三口投影腿已于本批合并为一份共用实现，入口本身不删（有生产接线）。

### 3.1 当时的问题清单（历史，推荐结论已被上面取代）

- **D-1 栈与 Push/Pop 关系（推荐独立栈）**：`projStack` 与 `Push()/Pop()` 的 2×3 栈**互相独立**（Push/Pop 不动投影层）——并存式自洽、无交互 bug 面；Skia 的 SkM44 并入 save/restore 栈，但那是以「CTM 整体升级 4×4」为前提，本批是并存式，独立栈与并存式同构。备选=并入 Push/Pop。
- **D-2 E 文字非仿射表态（推荐跳过+计数）**：fail-closed 不画错画面；备选=逐字投影差分定位（近似、复杂度高、v1 不值）。
- **D-3 GPU 纹理/视频三口非仿射表态（推荐 AABB 降级+计数）**：画面仍在（近似）；备选=跳过+计数（内容缺失）。
- **D-4 录制面（推荐随接口合规自然进）**：`Canvas` 接口 +4 方法 ⇒ `Recorder` 必须实现 ⇒ 录 `ProjectionCommand` 是最小实现且保回放真值，无推迟选项；scene 编码仍归 3e。

## 4 能力单测（先红后绿，拟）

| 测试 | 判据 |
|---|---|
| 投影栈四动词 | Set/Push/Pop/Projection 语义逐值（含嵌套、空栈 Pop no-op、恒等默认） |
| `DrawImageQuad`×投影 | 恒等=逐位同旧；投影后四角+权重逐点=手算合成（w=w_proj×w_own）；眼后整块拒绝 |
| `DrawImageEx`×投影 | 投影下 GPU 走透视槽（draw call/quad 计数对账）；GPU==CPU parity ≤1/255 |
| `DrawAtlasEx`×投影 | 仿射精灵升格透视（AABB 空角白底+梯形 UV 判别，复用 1c 判别器）；恒等逐位同旧 |
| 裁剪降级 | 投影下矩形裁剪=投影 quad 路径裁剪逐位等价（对拍直接路径裁剪） |
| 路径映射 | 投影下矩形填充=投影 quad 填充逐位等价；描边宽度按设备空间 |
| 文字跳过 | 投影下文字不画 + 计数器具名出现 |
| 负向 | 摘投影映射（恒等冒充）→ 四角钉红；摘升格（仿射口画投影角）→ UV 判别红；摘跳过（文字硬画）→ 计数钉红 |

真窗/离屏载体：离屏钉为主；**存量金图零变化**用 `examples/ui_wr_p35_persp` + `examples/engine/sprite` + `examples/engine/camera` 三窗双后端复验（本批触及全部 CTM 消费面，三窗覆盖 quad/图集/相机三链）。

## 5 口径与登记（预告）

- 公开面：`Canvas` 109→113 方法（+SetProjection/PushProjection/PopProjection/Projection）、`Context` 同步；catalog §Canvas 计数 + §3.9/相关行同步；`recording` +1 命令类型。
- 登记不做：透视文字 mesh warp（后续专项）；scene 编码扩投影（批 3e）；GPU 纹理/视频的透视 quad 形态（队列口无 quad 形参，登记）；B2 网格逐顶点色的透视正确插值（登记近似）。
- 本批状态：投影层 API 新增、现仓零调用者 ⇒ 「已验证待接线」；2.5D 场景级接线（地面/平面共用投影）随批 6 透视批绘窗承接。
