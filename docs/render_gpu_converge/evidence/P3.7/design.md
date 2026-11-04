# P3.7 2.5D — 正式方案（2026-11-03，拍板后落盘）

> 依据：影响面评估 `assessment.md`（四路实测 + 两探针 + §1.6 补测）；状态真源 `docs/ENGINE_RENDER_GPU_CONVERGE_TASKS.md` §2/§2.1；分层与门禁口径 `AGENTS.md` + `PLAN` §7.4/§7.9。
> 本文只定方案与批次，不含代码改动。每批实现时按 §9 门禁逐批收口，交付证据另起 `batch<N>.md`。

## 0 拍板记录（2026-11-03，三轮 question 逐项拍）

| 编号 | 问题 | 结果 |
|---|---|---|
| D1 | 批次切法 | **HDR 固定单列一批**（原拍五批，经 D10 展开为六批） |
| D2 | 透视能力从哪层进 render | **批绘腿吃「四角+权重」+ 放行合批**（render 不吃相机/投影参数，投影仍由引擎侧算好喂入）；Canvas 级 4×4 栈（P3.5 批 3 D4 挂账）并入批 1 单列一小步 |
| D3 | 世界高度参与遮挡的方式 | **画家算法**：排序键进绘制序，不动深度附件（实测：深度附件归 GPU-CLIP-003a 裁剪占用，精灵 z-test 要抢同一张 Depth24+Stencil8） |
| D3b | 三套 z 序实现归位 | **真源统一到 `engine/camera.YSort25D`**（冻结口径不动）；`engine/sprite/depth.go` 改委托；`render/depth_r5.go` 排序口按接口全量切换制处理，能力面随迁不裁 |
| D4 | 材质第一版范围 | **三件全做 + 扩绑定槽位（放开 binding 3+）+ 扩 uniform 预算（多块 uniform，支持多纹理材质）**〔用户选非推荐档，接受触及 P3.2 入口契约与双后端绑定组〕 |
| D5 | HDR/泛光做多大 | **泛光做 RGBA8 近似**；**真 float 全链另立专项**（显式声明非 HDR，PLAN/台账登记） |
| D6 | mip 档位接线落点 | **并入本项一小批**（批 4） |
| D7 | 真窗拆几扇 | **新建五扇**（透视批绘 / 高度 z 序 / 材质 / mip 出口 / 泛光），mip 出口单独一窗 |
| D8 | 材质执行骨架 | **真·quad 片元材质路**（用户 WGSL 作片元、引擎持顶点级，一次绘制无读回、可合批；弃 P3.2 的两趟 RT 读回骨架） |
| D9 | 透视/染色/材质进不进录制面 | **录制 + scene 编码都进** |
| D10 | 批次表 | **六批**（mip 小批单列；每批独立提交、独立过全套门禁） |

---

## 1 批次表

| 批 | 内容 | 触及层 | 子步 | 独立提交点 |
|---|---|---|---|---|
| **批 1** | 透视收口：合批放行 → 透视腿染色 → 批绘透视腿 → Canvas 级 4×4 栈 | `render` + `render/internal/gpu` | 1a 合批 / 1b tint 腿 / 1c 批绘腿 / 1d 4×4 栈 | 四个子步各自一次提交（1d 风险最大，可整步回退不影响 1a–1c） |
| **批 2** | z 序与世界高度归位：真源合一、副本改委托、render 排序口全量切换、实体高度字段、盖序不变式钉 | `engine/camera`、`engine/sprite`、`engine/world`、`render/depth_r5.go`、`examples/` 调用点 | 单批 | 一次提交 |
| **批 3** | 自定义材质：quad 片元管线 → 图像/批绘入口挂材质 → 绑定与预算契约扩展 → 描边洞 + 导出哨兵 → 录制 + scene 编码 | `render`、`render/internal/gpu`、`render/recording`、`render/scene`、`gpu/shader` 前端 | 3a 骨架 / 3b 入口 / 3c 契约 / 3d 洞+哨兵 / 3e 录制与 scene | 五个子步各一次提交（3e 最大，允许再拆） |
| **批 4** | mip 档位接线：`tex.Filter`↔`sprite.AtlasFilter` 映射 + `ToRender` 补 `Mipmap` + renderconv 补采样转换 | `engine/tex`、`engine/sprite`、`engine/renderconv` | 单批 | 一次提交 |
| **批 5** | 泛光 RGBA8 近似（亮部提取→降采样模糊→叠加合成），参数真源与 `engine/fx` 合一；真 float 链登记专项 | `render`、`render/internal/gpu`、`engine/fx` | 单批 | 一次提交 |
| **批 6** | 五扇真窗收口（三证据 + 双后端 + 金图 + 人工档） | `examples/engine/` | 五窗 | 一次提交（金图入库随窗） |

批间依赖：批 2 依赖批 1c（批绘腿是盖序的最终承载）；批 3a 依赖批 1b/1c（材质随透视腿走）；批 5 依赖批 3a（亮部提取可用材质片元）与批 4（mip 降采样）；批 6 依赖 1–5 全部。

---

## 2 批 1 · 透视

### 2.1 步 1a — 放行透视多 quad 合批

- 现状（实测）：`image_pipeline.go:1178` 对 `a.Perspective || b.Perspective` 无条件 `return false`；`render_session.go:4345-4364` 每个透视 slot 写死 `vertCnt: 6`；而 `packPerspectiveQuads:4552-4597` 已把全部透视命令按序连续打进同一块 36B 缓冲（`pIdx` 递增），`imagePerspVertBuf` 单一持久缓冲已在。
- 改法：
  1. 谓词改为「**透视与仿射互不合，透视与透视照其余判据合**」（`a.Perspective != b.Perspective → false`）。`Weights` 与 `Tint` 一样是逐顶点数据，**不进合批键**（沿用 `:1166-1170` 对 tint 的同款书面理由）。
  2. 发射处按跨度出 `firstVertex = pIdx[i]*6`、`vertCnt = (j-i)*6`；**必须断言组内 pIdx 连续**（`pIdx[k+1]==pIdx[k]+1`），不连续立即分槽——不连续的唯一来源是 `degradePerspectiveCmds:4531` 就地降级（`Perspective=false`）后该命令不再占 pIdx，故降级发生在 pack 之前（现有顺序 `:4244`→`:4291` 已满足），但**同帧混排降级与非降级时必须重测**。
  3. 组切片仍走 `cmdStart/cmdCount`（P3.5 窗口抓出的第 ④ 洞修复位），合批后必须复验该钉不劣化。
- 度量与门禁：draw call 与 uniform slab 用同一探针口径（评估 §1.2 已给基线：N=1000 透视=1000 draw call / 250.0KB slab → 目标 1 / 0.2KB）；`LastImageBatchStats`（`render_session.go:4601`）出数进真窗探针。
- 存量影响：谓词只可能**减少** draw call，不改任何几何字节；仿射命令的槽位与 `firstVertex = i*6` 不变 ⇒ 存量金图应逐位零变化（红线，红了按窗 A/B 归因）。

### 2.2 步 1b — 透视腿补染色（成本比评估预估的小）

- 实测（§1.6）：36B 布局与 `textured_quad_persp.wgsl:27` **已有 tint 通道**，只是 `buildPerspVerticesInto:1264-1272` 写死白。
- 改法：`QueueImageDrawQuadW`（`gpu_render_context.go:1445`）按接口全量切换制直接加 tint 形参（调用点同批改完）；`ImageDrawCommand.Tint*` 字段已在（`image_pipeline.go:104-110`，premul 存储约定与仿射一致，零值=白）；打包函数取值替换常量。WGSL/布局/hal **零改动**。
- 公开面：`QuadDrawOptions` 加 `Tint RGBA`（零值=白=存量逐位不变，同 `AtlasSprite.Tint` 惯例，`render/vertices.go:88-89`）；`DrawImageQuad` 的 BlendMode 仍限 Normal（`ErrQuadUnsupportedBlend` 保留，透视混合模式推后续，登记不装）。
- CPU 腿必须同批接（否则 GPU/CPU 分叉）：`m4_quadimage.go:72,110 drawImageQuadCPUSplit` 的 persp 采样要乘同一 tint，语义照仿射 CPU 腿。
- 负向：摘 tint 取值 → 批 6 材质/灯光窗的染色像素钉红。

### 2.3 步 1c — 批绘入口吃「四角 + 权重」（2.5D 精灵的主腿）

- 现状：`AtlasSprite`（`render/vertices.go:77-95`）无四角/权重位；`AtlasDrawOptions` 是空结构体（`:110`）；`DepthSprite` 的 `Depth` 只参与排序（`depth_r5.go:132` 起）不参与投影；`Projector.Project` 的 scale 全仓非测试调用点丢弃（`examples/engine/camera/main.go:924,940`）。
- 改法（D2 拍定的形态：**引擎算好、render 只收**）：
  - `AtlasSprite` 加 `Perspective bool` + `Corners [4]Point` + `Weights [4]float64`（与 `QuadDrawOptions` 同名词同序 TL/TR/BR/BL，接口一致，避免第二套词汇）。
  - `Perspective=false`（零值）⇒ 走现有 rect+Rot+Pivot+Flip 仿射拆法，**逐位不变**；`true` ⇒ 忽略 Dst/Rot/Pivot/Flip，直接用 `Corners`+`Weights`（避免"半仿射半透视"的混合解释；非法组合显式拒绝，见下条），GPU 走 1a 合批后的透视槽，CPU 走 `newTriSamplerPerspective`。
  - 校验入 `isFiniteAtlasSprite` 同族：四角/权重非有限、权重 ≤0、四角退化或自交 ⇒ 复用 `ErrAtlasNonFinite` 与新增 `ErrAtlasQuadDegenerate`/`ErrAtlasQuadBadWeights`（**按 quad 口现有哨兵命名对齐**：`ErrQuadDegenerate`、`ErrQuadBadWeights`，`m4_quadclass.go:58-`），不静默画错。
  - `DrawDepthSprites` 保留「按输入序批绘 + 深度承载」，投影由引擎侧（`Projector`/`Matrix4.ProjectQuad`）算好后填 `Corners/Weights`。
- 引擎侧配套（同批，engine 模块库）：`engine/sprite` 的 `AtlasSprite` 加透视承载 + `ToRender` 透传；`engine/camera` 出一个「按 Projector 把 world rect + depth 投影成四角+权重」的出口（把 `Projector.ProjectQuad` 的 `core` 结果经 `renderconv` 转成 render 类型——**注意 `ProjectQuad` 生产消费者现为 0**，本批是它第一个真消费者）。
- 示例层禁止绕路：`examples/engine/camera/main.go:924,940` 现在丢弃 scale 写死尺寸，属**示例层代劳**；批 1c 落地后该窗改走新腿（示例只做验证）。
- 负向：摘权重传递（填等权）→ 梯形 UV 钉红；摘 `Perspective` 分支 → 批绘透视像素钉红。

### 2.4 步 1d — Canvas 级 4×4 栈（P3.5 批 3 D4 挂账）

- 现状实测：CTM 是 2×3（`c.matrix` + `c.deviceMatrix`，`context_transform.go:66,128,138,148,174`），非测试 `totalMatrix()` 调用点 **40 处 / 12+ 文件**（`text_gpu.go` 9、`text_dispatch.go` 7、`context_image_gpu.go` 4、`context_clip.go` 4、`vertices_mesh.go` 3、`vertices_atlas.go` 3…）。
- **前置探针（先测后改，缺证据不动手）**：把 40 个调用点逐个分类为「只要屏幕位置/尺寸」vs「要设备空间换算」vs「可直接吃投影后坐标」，分类表落 `batch1.md`。结论决定最终形态：
  - 若绝大多数只要仿射子块 ⇒ **并存式**：`Context` 增加投影层状态（`PushProjection(Matrix4)/PopProjection/SetProjection/Projection()`），仅在图像 quad / 批绘透视腿生效；路径/文字/裁剪继续走 `Matrix4.Affine()` 派生的仿射，**`Affine()` 返回 ok=false 时显式处理，禁止静默压扁**（该判定已在 `matrix4.go:218`）。
  - 若有调用点必须吃真 4×4 ⇒ 逐点改，逐点 A/B 归因。
- 对齐口径：Skia `SkM44` 允许 CTM 带透视但 `SkCanvas` 对透视 CTM 的能力是**限定域**（图像/矩形走 persp2d，路径与文字有额外约束）⇒ 并存式与 Skia 的实际形态一致，不是折中。
- 风险最高的一小步：触及 CTM 即触及全部存量金图，任何差异按窗 A/B 归因 + 显式声明，禁止调容差。

---

## 3 批 2 · z 序与世界高度归位

- **真源**：`engine/camera.YSort25D`（`ysort25d.go:60`，`SliceStable` + 平票回落 `Order`）与 `YSortKey = Y + 0.001*(X+Z)`（`:43`）。冻结口径不动（`doc.go:94` additive only）。
  **（2026-11-04 步 2a 已改判并交付：D-H 申请解冻，键升成两级 = z 桶（±4096 每级 CLAMP）→ 层内 origin Y（Godot `is_equal_approx` 判等）→ 容器 DFS 序号；`YSortKey` 分裂为 `YSortKeyY`（标准）与 `YSortKeySlightXZ`（demo 变体，可选）。现行口径以 `batch2-design.md` §1/§8 为准。）**
- **副本归位**（D3b）：
  - `engine/sprite/depth.go`（`DeepItem:23`/`DepthSort:73`/`IsDepthSorted:92`/`SetDepth:44`）→ 内部改走 YSort25D；`DeepItem` 的 `Layer+FeetY` 平票回落是它自己的语义，映射到 `YSortItem{Pos,Order}` 时必须**逐位保持现有排序结果**（对拍钉：拿 `stage_chase` 的 1000 prop 现序 + 现有单测全集做输入，新旧输出逐元素相等），否则动的是行为不是实现。**（2026-11-04 步 2b 已交付：本地判据副本删除，带→z 桶走 `ZIndexForLayer`，对拍语料 `engine/sprite/testdata/p37_zsort_parity.json` 11 案逐元素相等；见 `batch2-design.md` §9。）**
  - `render/depth_r5.go`：排序口 `SortDepthSprites:99`/`IsDepthSorted:116`/`SetDepth:86` 与引擎重复 ⇒ 按「能力面随迁不裁 + 接口全量切换不留垫片」处理：排序**能力**迁到 engine 域（键与公式在 camera 一份），render 侧 `DrawDepthSprites:132` 保留但 `DepthDrawOptions.DepthTest`（`:48-51`）这个"render 内部排序开关"删除（全量切换），调用点 `examples/engine/sprite/main.go:411-484`、`examples/ui_render_base_recording/scene.go:553` 同批改；`DepthSprite` 的数据格式（Sprite+Depth+Name）随迁保留。**（2026-11-04 步 2c 已交付：`DrawDepthSprites(img, sprites)` 去 opts 只按提交序画，testdata/录制面/spy/示例窗同批切换，sprite 真窗 golden 0.0000% 未重出；见 `batch2-design.md` §10。）**
- **世界高度**（2026-11-04 批 2 步 2d 修订，依据 D-D 改判与 Godot 实测）：`engine/world/entity.go:29-32` `Transform{Pos core.Vec2; Rot; Scale}` 无高度 ⇒ 加 `Height float64`（additive），校验有限值；`LocalMatrix` 与仿射链不变（高度不进 2×3 矩阵）。**高度只参与投影深度输入，不进遮挡排序键**：遮挡排序键仍取脚位 Y（`camera.YSortKeyY`）。原句"高度参与遮挡排序键"与标准冲突——Godot 2D 无离地高度概念，`Sprite2D.offset/centered` 明确不进 y-sort 键（实测 `scene/2d/sprite_2d.cpp:116-136`、`renderer_canvas_cull.h:117-127` 只比 `ysort_xform.origin.y`）；"高个子站得高却排在后面"正是反直觉的那条。修订日期 2026-11-04，原因即上。
- **盖序不变式（必须实测而非假设）**：render 的 tier 系统（`drawTierSDF=1 … drawTierImage=4 … text=6`，`gpu_render_context.go:4134-4139`；`ensureDrawOrder:4169` 只在回退时封批）⇒ 批内探针验证「图像 tier 与 SDF tier 混排时，最终屏幕盖序是否等于提交序」。若不等（tier 优先级压过提交序），画家算法在 render 层就是**假前提**，必须先修提交序保真，再谈 z 序；结论与修复写进 `batch2.md`，不许默认成立。
- **z 槽口径处置**：`YSortZ`（`:72`，基槽 −4000/步长 2/奇数留给影子）**全仓零消费者**；`shadow25d.go:22-23` 说窗把影子放在主人 z 槽上一格，实测 25d_basis 把影子画在地面线固定 y（`examples/engine/25d_basis/main.go:556-558`）⇒ 属未落地叙述。处理：符号按「标准能力零消费者也保留」不动，`engine/camera/doc.go` 与 `shadow25d.go` 头注释**改为事实描述**（遮挡走画家序，z 槽为保留口径未接线），不改行为。
- **不做的**：精灵 z-test（深度附件归裁剪，D3 拍不走）；`ShapeCast3D`（`shadow25d.go:13-19` 自陈缺该环）归后续，本批只把高度数据入口打通。

---

## 4 批 3 · 自定义材质（D4+D8+D9）

### 4.1 步 3a — 真·quad 片元材质骨架

- 现状骨架：P3.2 是「引擎持顶点级全屏三角 + 用户 `fs_main(uv)` 画进离屏 RT → 读回 → 当贴图合成」，`user_shader_fill.go:266-267` 每次 `WaitIdle()` + `readTextureViewRegionRGBA`，scratch 未池化（`:174-175` 自认待池化），面积上限 8M 像素（`brush_fill.go:24`）⇒ 精灵逐张绝不能用这条路。
- 新骨架（对齐 Skia：`SkRuntimeEffect` 产出 `SkShader`，可填任意几何）：
  - 引擎提供一个 **quad 顶点模块**（复用 `textured_quad_persp.wgsl` 的 vs 形态：pos/uv/tint/hom_w，36B），Fragment = 用户 WGSL 模块；管线变体 = (effect × 透视与否 × 是否走裁剪腿)，缓存键沿用 `userShaderCache.entries`（`user_shader.go:67`）加变体维度。
  - 用户片元入口签名从只收 `uv` 扩为**按声明前缀取用**：`@location(0) uv`（必需）、可选 `@location(1) tint: vec4<f32>`、可选 `@location(2) hom_w: f32`；校验在 `validateShaderContract`（`shader_effect.go:117-175`）扩，非法形态在入口拒绝（照 M3 的教训：不能等首绘才炸）。
  - 一次绘制、**无读回**、走 1a 后的合批。
- **premul 口径必须先标定再定**（一处真分叉，不能拍脑袋）：填充路是「用户输 premul → 读回时 `premulToStraightRGBA` 去预乘 → 合成链按 straight 解释」（`user_shader_fill.go:86,160-170`，口径由 `render/m1_colorspace_audit_test.go:13-19` 钉）；quad 路无读回，输出直接进混合。⇒ 两条路必须统一到同一句契约：**用户 shader 一律输 premul**，quad 路按 premul 混合（`one, one-minus-src-alpha`），填充路的去预乘步骤改为等价地改混合方程而不是数值往返。**改前先用探针实测两路对同一 shader 的输出是否一致**，不一致按实测定改法；这是批 3 的前置探针之一（§10）。

### 4.2 步 3b — 图像/批绘入口挂材质

- `DrawImageOptions`、`QuadDrawOptions`、`AtlasSprite` 各加 `Material *ShaderEffect`（零值 nil = 无材质 = 存量逐位不变）；材质是 **per-draw 选项**（与 tint 同位），不是全局 Paint——2.5D 逐精灵不同材质，全局 Paint 口会逼调用方逐张 flush。
- GPU 消费点：图像命令携带材质引用，材质命令**不合批进无材质批**（谓词加一条 `a.Material != b.Material → false`，指针比较，与 D8 的「布局按模块声明派生」同族）。
- CPU 腿语义沿用现契约：**无 CPU 求值器**，CPU 模式跳过并计数（`context_gpu.go:634`、`context_draw.go:324`、`render/p32_shader_premul_test.go:68` 的 `CPUFallbackOps==0` 口径）；此事实写进材质入口文档，不假装支持。

### 4.3 步 3c — 绑定与预算契约扩展（D4 加重档）

- 现契约封死：`shaderSlotUniform/Texture/Sampler = 0/1/2`（`shader_effect.go:109-113`），`binding≥3` 直接拒（`:163`），`ir.TypeSize ≤ 256`（`:141-146`），`ShaderUniformBudget = 256`（`:85`），布局 `layoutSize = max(256, roundup(len,256))`（`user_shader.go:245-246`）。
- 放开形态：绑定条目**继续由模块声明派生**（D8 机制已建立：`declaresImageBinding:244-257` → `ShaderBinding():69` → `user_shader.go:215`），把「单纹理+单采样器」扩为**按声明的 texture/sampler 对数 N**；上限取设备真实 limits（`hal.Adapter.Limits()` 已有：`MaxSampledTexturesPerShaderStage`、`MaxSamplersPerShaderStage`、`MaxBindingsPerBindGroup`，`gpu/types/limits.go:27-43`），`uniform` 预算上限夹到 `MaxUniformBufferBindingSize`（`:47`）。
- 现成先例（不是投机）：P3.4 GPU 滤镜图已用 **binding(3) 第二张纹理**且双后端过金图（`filter_gpu_graph.go:64-67` 声明、`:842` 绑定）。
- 公开面：`ShaderEffect` 的图片口从单张 `SetImage(img)` 扩为按槽位（`SetImageAt(slot, img)` 或等价），`ShaderInterface` 相应扩（现只有 `UsesImage bool`，`shader_effect.go:63-65`）；预算常量 `ShaderUniformBudget` 语义升级为「默认预算」，新增上限查询口。
- 必做前置：binding 4+ 在 gles 经 naga 自动翻译的实际可用性**本机实测**（GLSL 绑序号与 `gles/device.go:1045` 的翻译时机），Metal 本机不可验按标记写明。

### 4.4 步 3d — 引擎洞与错误分派

- **描边快路不吃 shader**（实测矛盾）：`fastStroke → fillPathUser`（`gpu_render_context.go:2413-2423`）与 `isGPUSolidPaint`（`sdf_gpu.go:439-453`）均不查 `Shader`，`SetRGB+SetShader` 的 AA 无虚线描边画成环境实心色，与 `SetShader` 文档「fills **and strokes**」（`shader_effect.go:422-424`）矛盾；全仓无 stroke+shader 测试。⇒ **修代码不改文档**（Skia 里 shader 对 stroke 同样生效）。修后若动存量金图，逐窗 A/B 归因。
- **零导出哨兵**：新增 `ErrShaderBadWGSL`/`ErrShaderNoEntry`/`ErrShaderBadReturn`/`ErrShaderSlotConflict`/`ErrShaderUniformTooLarge`/`ErrShaderClosed`/`ErrShaderBackendCompile`；后端级失败（gles 首绘才炸，`user_shader_fill.go:177-180` 现打 stderr 返 `ErrFallbackToCPU`）改为具名错误并可计数。与在册 M3（WGSL 前端放行非法 intrinsic）同族，**本批只把「我方已能判的」变具名错，不冒充修完前端语义校验**。

### 4.5 步 3e — 录制 + scene 编码（D9）

- 录制：`FillPathCommand`（`recording/command.go:299-310`）现无 Shader 字段（L2 深审登记过该欠账）⇒ 补材质引用进命令；`recording` 对 `ShaderEffect` 现零引用，`SetShader`/材质选项需随 save/restore 状态携带（照批 4 PathEffect 的既有形态：录制期取值、回放只读、命令结构最小新增）。
- scene：`render/scene` 公开面是 Affine-only（`scene.go:138 Fill(…, Affine, …)`、`:253 DrawImage(img, Affine)`、`:422 PushTransform(Affine)`）⇒ 需加透视与材质的场景级入口与编码流（`encoding.go:414` 双流表示）。**这一块是本项最大单体，允许在 3e 内再拆两次提交**；scene 编码改动若触及存量 scene 金图，逐窗归因。
- 生命周期：录制/scene 命令持 `*ShaderEffect` 引用 ⇒ 跨设备与关闭语义走 D1 的广播驱逐（`shader_effect.go:365,405`、`gpu_shared.go:249,676`），不新造第二套释放。

---

## 5 批 4 · mip 档位接线（D6）

实测两处断点（§1.6），不是「补一个函数」那么小：

1. `engine/tex.Filter`（带 `Mip MipMode`，`mipmap.go:107`）与 `engine/sprite.AtlasFilter`（四档，`atlas.go:23-34`）之间**无任何映射**，`SamplerParams`（`:204`）生产消费者为 0。
2. `engine/sprite/atlas.go:217 ToRender()` 逐字段转 `render.AtlasSprite` 时**从不填 `Mipmap`**（字段是 P3.6 批 2 加的，`render/vertices.go:92-94`）。

方案：

- `engine/renderconv` 补采样转换（把 `project.go:20-21` 自陈「render 类型都在 renderconv 转一次」补成真话）：`FilterToRender(tex.Filter) (render.InterpolationMode, render.MipmapMode, aniso uint16)` 与 `SamplerParamsToRender`；转换直接复用 `tex.Filter.SamplerParams()`（**该口由此拿到第一个真消费者**，不再只是自检探针）。
- `engine/sprite.AtlasFilter` 扩 mip 承载：优先方案是 sprite 的过滤字段**改为吃 `tex.Filter` 派生的档位**（避免第三套过滤枚举；现 `AtlasFilter` 与 `tex.Filter`、`render.InterpolationMode` 已三份并存），按「同一能力不留第二份枚举」的 P3.6 批 2 先例（`tex.MipMode` 已改 `render.MipmapMode` 别名）走别名/派生，不留双路。
- `ToRender()` 补 `Mipmap`（+ aniso 若有口）；`render` 侧采样链与档位字段 P3.6 批 2 已全就绪（`EffectiveImageSamplerKey:1090`、`SetPerImageFilter:1110`、`QueueImageDraw*` 收 `MipmapMode`），本批**不动 render**。
- 端到端验证走批 6 的 mip 出口窗 + 单测钉（tex 档位 → GPU 采样键变化，摘传递必红）。

---

## 6 批 5 · 泛光（RGBA8 近似）与真 float 专项（D5）

- **做**：亮部提取 → 降采样模糊 → 叠加回合成，全程 RGBA8。可复用件实测在：P3.4 GPU 滤镜图（blur/merge/aux_tex 双输入，`filter_gpu_graph.go:64-67,842`）、`render/hdr` 的 Reinhard 曲线（CPU，零 importer）、`engine/fx` 的 `Bloom{Threshold,Radius,Intensity}`（`bloom.go:175-179`，上限 `MaxBloomRadius=16`/`MaxBloomIntensity=4`）与四档 tonemap（`lut.go:155-171`，`ApplyTonemap(exposure)` `:259`）。
- **参数真源必须合一**：泛光的阈值/半径/强度在 engine（CPU `fx.Bloom`）与 render（GPU 新链）**不允许两份数字**——按「禁在示例绕、禁第二套实现」，本批把 GPU 链的参数类型定为唯一真源，CPU `fx.Bloom` 改走同一参数（其 `clamp01` 语义如实保留并标注为 LDR 路）。
- **显式声明**（不许冒充 HDR）：RGBA8 上亮部 >1.0 被夹，泛光是阈值伪高亮；文档、真窗探针、catalog 三处都要写「LDR 近似，非 HDR」。
- **登记为专项（本项不做）**：真 float 全链的前置缺口实测在案——`GPURenderTarget` 无 Format 字段（`render/accelerator.go:71-99`）、呈现固定 `BGRA8Unorm`（`present_construct.go:63`）、`internal/gpu` 离屏 RT 全 8-bit（30+ 处）、`render/hdr.FloatTarget` 的 `TextureView()` 显式返回 nil（`float_target.go:74`）且全仓零 importer；驱动能力侧已备（RGBA16Float 三端可渲染 + `RG11B10Ufloat`/`RGB10A2Unorm` 在可用表）。专项内容=目标格式概念 + float 离屏 + tonemap pass + 呈现输出 + 双后端。落台账 §2.2 新行编号接在 M4 之后（**M5**），拟归属=渲染专项（HDR 全链）。

---

## 7 批 6 · 五扇真窗（D7）

按「每主能力独立真窗 + 禁止组合窗代替 + 满窗即内容 + 指标浮在内容上 + 内容往厚里做」，全部落 `examples/engine/`（2.5D 真窗归位纪律）：

| 窗 | 验的能力 | 往里摆的已绿能力 | 关键探针 |
|---|---|---|---|
| 透视批绘窗 | 1a 合批、1b 染色、1c 批绘腿、1d 栈 | `Projector`/`Matrix4.ProjectQuad`/mip 三档/裁剪 | **draw call 与 uniform slab 计数**（`LastImageBatchStats`）、透视校正 vs 仿射对照（同图并排）、深度阶梯、眼后整块拒绝 |
| 高度与 z 序窗 | 批 2 | `YSort25D`/`Basis25D`/`LandShadow`/新 `Height` | 盖序随高度与 y 变、平票不闪、非有限 fail-closed、**跨 tier 提交序=盖序不变式** |
| 材质窗 | 批 3 | 精灵批 + 透视腿 + `engine/light` 灯光染色 | 用户 shader 随精灵变换走、多纹理材质、无读回（GPU op 数不随材质张数线性炸）、描边 shader、坏 WGSL 具名拒绝态 |
| mip 出口窗 | 批 4 | `ui_wr_p36_mipmap` 的三档形态（引擎出口版） | 采样键对账（段间字节差）、tex 档位→屏幕变化、摘传递必回退档 |
| 泛光窗 | 批 5 | P3.4 滤镜链、批 3 材质、mip 降采样 | 阈值/半径/强度扫参、亮部区域标准差与能量、**显式 LDR 声明探针**、fps 与 p95 |

- 每窗三证据合证：逻辑探针（JSON）+ 语义像素断言 + Golden 逐位（容差按 P0.1 默认 0，**金图按后端分文件 `_native/_go`**）；双后端各一轮，`XWayland :0` 一轮一窗、WM_DELETE 正常关闭、每轮查显示服务；自动档 `-auto-only` 判 PASS/FAIL，另留常驻人工窗待用户二合一（一次性验收窗不能代替人工窗）。
- 像素断言坐标一律「从画面所用同一份投影/材质输出反推」，不从屏幕反推结论（P3.5 收口的教训）。

---

## 8 公开 API 变更计划与 catalog 同步（硬：机器校验 `go run ./scripts/apidoc` rc=0）

| 批 | 新增/变更 | 说明 |
|---|---|---|
| 1b | `QuadDrawOptions.Tint` | 零值=白，存量逐位不变 |
| 1c | `AtlasSprite.{Perspective, Corners, Weights}`、`ErrAtlasQuadDegenerate`、`ErrAtlasQuadBadWeights`；engine 侧 sprite 承载 + camera 投影出口 | 与 `QuadDrawOptions` 同词汇 |
| 1d | `Context` 投影层口（`SetProjection/PushProjection/PopProjection/Projection`，形态依 §10 探针结论定） | Canvas 方法数会变，catalog §Canvas 计数同步 |
| 2 | `engine/world.Transform.Height`；删 `render.DepthDrawOptions.DepthTest`；排序口迁 engine | **全量切换不留垫片**；catalog §3/§13 同步 |
| 3 | `DrawImageOptions.Material`、`QuadDrawOptions.Material`、`AtlasSprite.Material`；`ShaderEffect` 多图片槽口 + `ShaderInterface` 扩；7 枚错误哨兵；uniform 预算口 | 契约变更需 catalog P3.2 行与 §13 P3.7 行转正式登记 |
| 4 | `renderconv` 采样转换；`engine/sprite` 过滤档位合一 | 无 render 侧变更 |
| 5 | 泛光参数类型与绘制/后处理口（形态依 §10） | CPU `fx.Bloom` 参数改同源 |
| 6 | 无公开 API（仅 `examples/`） | — |

catalog §13 的 P3.7 规划行（`:900`）在本项收口时转正式登记，`Matrix4`/`ProjectorMatrix4`/`ProjectQuad` 三行的「🧪 待接线（生产消费者=P3.7）」状态随批 1/批 4 改为生产（带 file:line 证据）。

---

## 9 每批收口门禁（照 P3.5/P3.6 既定形态，一步不跳）

1. `go run ./scripts/apidoc` rc=0（公开 API 改动同批同步 catalog）；
2. `go build ./...` + `go vet` 触及包；
3. 五目标 cross build 5/5（CGO=0，另跑 `-tags nogpu`）；
4. `tools/per-file-tests.sh` 逐文件对基线**只减不增**（当前主基线 **750 = 719/21/9/1**；FAIL 九枚名单见 `evidence/review-fix/per-file-tests-dbatch/`；M4 整包 SIGABRT 按 per-file 口径判，不顺手修）；
5. `-race`：触及共享数据必跑（批 3 的 ShaderEffect 生命周期、批 6 的窗口探针）；
6. 每批：单测红→修→绿 + **负向对照摘修复必红** + conformance 双后端 + 三证据真窗（批 6 集中出金图，前面各批用离屏/逻辑钉）；
7. GPU 测试必带三件套（`CGO_ENABLED=1 LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so`），不带必假绿；
8. 金图红线：**存量金图逐位零变化**是每批验收项；任何差异按窗 A/B 归因 + 显式声明理由，禁止调容差数字蒙过。

---

## 10 前置探针（三处「先测后改」，缺证据不动手）

| 探针 | 目的 | 落点 |
|---|---|---|
| **P1 · tier 与提交序**（批 2 前必做） | 图像 tier(4) 与 SDF tier(1)/text tier(6) 混排时，屏幕盖序是否等于提交序。不等 ⇒ 画家算法在 render 层是假前提，先修序保真 | `batch2.md` §探针 |
| **P2 · CTM 调用点分类**（批 1d 前必做） | 40 个 `totalMatrix()` 调用点按「只要屏幕位置/尺寸 / 设备空间换算 / 可直接吃投影后坐标」分类，决定并存式还是整体升级 | `batch1.md` §1d |
| **P3 · premul 双路口径**（批 3a 前必做） | 同一份 shader 分别走「填充路（RT 读回去预乘）」与「quad 片元路（直出混合）」，实测像素是否一致；不一致按实测定契约统一方式 | `batch3.md` §3a |

三条结论一律落文件，不允许「应该一致」这类措辞出现在实现依据里（AGENTS 动手修门槛）。

---

## 11 登记不做 / 已知风险

- **不做（本项）**：精灵 z-test（D3）；`ShapeCast3D` 与 3D 射线命中（批 2 只开高度数据入口）；真 HDR float 全链（→ 拟 M5 专项）；透视腿的非 Normal 混合模式（`ErrQuadUnsupportedBlend` 保留）；`fill+PathEffect`（P3.5 已登记不做）；gles `glGenerateMipmap` 原生产链（P3.6 登记后续）；动图差量上传。
- **顺手不碰的在册欠账**：M2（rrect4 gles 29px 量化分叉）、M3（WGSL 前端非法 intrinsic——批 3d 只把「已能判的」变具名错，不宣称修完）、M4（render 整包 SIGABRT，触整包跑测按 per-file 判）、K2–K5、D5 富文本迁移 ui/rendering。
- **风险面**（按大小排）：① 批 3e 的 scene 编码（公开面从 Affine-only 扩，单体最大，允许再拆提交）；② 批 1d 的 CTM（触及全部存量金图）；③ 批 3c 的绑定扩展触及 P3.2 已收口契约与双后端（Metal 本机不可验，按标记写明）；④ 批 2 的行为等价性（副本合并若排序结果逐位不同就是行为变更，必须靠对拍钉守住）；⑤ 批 5 的参数真源合一（CPU `fx.Bloom` 现零生产消费者，改动要保住其单测语义）。
- **未推送提交现状**：本地 `989f4ff8`/`85e3b490`/`927eab84`/`1d70d150` 四枚在册未推送，本项不动它们；提交与推送均先与用户确认。
