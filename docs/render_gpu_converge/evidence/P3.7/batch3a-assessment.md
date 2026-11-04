# P3.7 批 3a（真·quad 片元材质骨架）· 影响面评估（2026-11-04，开工实测，零代码变更）

分支 `feat/p2-收敛整合`，HEAD 批 3 步 3e-2 收口后（`1b287b4d` + 簿记 `762c436e`，均未推送）。
本批判据源三处：① `evidence/P3.7/design.md` §4.1 的 3a 规划 + 已拍板的 D4（三件全做+扩绑定+扩预算）/D8
（真·quad 片元路，弃两趟 RT 读回）/D9（录制+scene 都进）；② batch3e 拍板改判——材质按 Godot 为
**item（节点）级为主**，命令级材质若要做必须标注超出 Godot ⇒ 3e-3 的 scene 材质编码依赖本批（3a–3d）；
③ Godot 源码本机实测（`/home/yanghy/app/projects/gogpu/godot`，HEAD `ca871ccc9c`）。

## 1 本仓 P3.2 现状（全部实测到行号）

### 1.1 ShaderEffect 契约与公开面

| 面 | 事实 | 位置 |
|---|---|---|
| 类型 | `ShaderEffect{wgsl, uniform []byte, image *ImageBuf, usesImage, closed}`，互斥锁护体；Skia `SkRuntimeEffect`/Flutter `FragmentProgram` 同位物 | `render/shader_effect.go:49-57` |
| 入口契约 | `fn fs_main(uv: vec2<f32>) -> vec4<f32>`，输出 **premul**（L2-P1-4 校准：premul 直喂 straight 链会把 alpha 平方）；**参数面今天没有校验**——零参 `fs_main()` 照样过（测试实测在用） | `shader_effect.go:27-35`（文档）、`:117-186`（只查入口存在+返回类型+绑定+f64，不查参数）、零参实例 `render/p32_shader_leak_test.go:19` |
| 绑定契约 | 固定三槽：binding(0)=uniform（≤256B，16B 对齐）、binding(1)=texture_2d<f32>、binding(2)=sampler；**binding≥3 直接拒**；sampler 无 texture 拒 | `shader_effect.go:109-113`、`:163`、`:166-168` |
| 预算 | `shaderUniformMax=256`，公开 `ShaderUniformBudget`；布局按契约值不按活值 | `shader_effect.go:81-85` |
| 接口派生 | `ShaderBinding()->ShaderInterface{UsesImage}`，由模块声明派生（`declaresImageBinding` 查 binding(1)）；管线缓存布局唯一来源（L2-P1-3 首绘毒化教训） | `shader_effect.go:63-76`、`:244-257` |
| 生命周期 | `RegisterShaderEffectEvictor` 驱逐钩子族 + `Close()` 幂等（关后拒用），跨设备/关窗走广播驱逐 | `shader_effect.go:358-420` |
| 画刷口 | `Context.SetShader/Shader()` 挂 `Paint.Shader`；文档宣称「fills and strokes」（与描边快路现状矛盾，3d 项） | `shader_effect.go:422-438` |
| 前置校验 | `validateUserShader`：Parse→Lower→Validate 全链在 `NewShaderEffect` 拒绝，不进后端 | `shader_effect.go:263-278` |

### 1.2 执行骨架：两趟 RT 读回（D8 拍板要弃的形态）

| 面 | 事实 | 位置 |
|---|---|---|
| pass1 管线 | 顶点=引擎全屏三角（只出 `@location(0) uv`），片元=**用户模块原样当入口**（无任何组合）；blend One/Zero 独占写 RT | `render/internal/gpu/user_shader.go:30-53`（VS）、`:258-276`（管线，blend One/Zero） |
| 布局 | bgl 由声明派生：binding(0) uniform（MinBindingSize=256 预算）+ 声明才有的 binding(1)/(2)；未绑图用 1×1 黑占位（Dawn default-bindings 同款） | `user_shader.go:215-229`、`:68-75`、`:130-160` |
| 缓存 | `userShaderCache.entries map[*ShaderEffect]*userShaderPipeline`——**键只有 effect 一个维度**；Close 驱逐挂钩在位 | `user_shader.go:60-86`、`:165-172` |
| 读回 | 每次 `WaitIdle()` + `readTextureViewRegionRGBA`（scratch RT 每次新建未池化，自认待池化） | `user_shader_fill.go:172-176`、`:266-267` |
| premul 路口 | 读回后 `premulToStraightRGBA` 去预乘 → 当 straight 图走 coverage/mask 链 → `QueueImageDraw` 贴回；**契约=用户输出 premul** | `user_shader_fill.go:74-86`、`:160-170` |
| 面积上限 | `maxBrushFillPixels = 8M`，超了整笔回 CPU | `render/internal/gpu/brush_fill.go:23-24`、`user_shader_fill.go:70-72` |
| 后端失败 | gles 首绘才炸 → stderr + 返 `ErrFallbackToCPU`（无具名错误，3d 项） | `user_shader_fill.go:82-84` |

### 1.3 消费点（P3.7 开工评估已盘点，本批复核）

| 面 | 事实 | 位置 |
|---|---|---|
| 填充路 | `FillPath` 见 `paint.Shader != nil` → `fillShaderAsImage`（读回路）——**今天 shader 的唯一 GPU 消费点** | `render/internal/gpu/gpu_render_context.go:2042-2043` |
| 描边快路 | `fastStroke → fillPathUser` 只吃实心（`isGPUSolidPaint` 不查 Shader）⇒ `SetRGB+SetShader` 的 AA 描边画成实心色，与文档矛盾；全仓无 stroke+shader 测试（3d 修） | `gpu_render_context.go:2421-2430`、`sdf_gpu.go:439-455` |
| 图像三入口 | `DrawImageOptions`（`render/context_image.go:152`）、`QuadDrawOptions`（Tint/Perspective/Weights/采样档，`render/m4_quadimage.go:38-60`）、`AtlasSprite`（Tint/Perspective/Corners/Weights，`render/vertices.go:77-101`）——**均无 Material 字段**（3b 挂点） | 同左 |
| 合批谓词 | `canMergeImageDraw`：透视位相等（1a 拍板：透视邻居互相合）+ GenerationID + 采样/透明度 + BlendMode + FilterKey + 视口 + 图宽高；**无材质维度**（3b 谓词加 `a.Material != b.Material → false`） | `render/internal/gpu/image_pipeline.go:1177-1212` |
| quad 管线族 | `TexturedQuadPipeline` 已按变体扩展出成例：stencil（`:275`）/ depthClip（`:323`）/ bicubic 对（`:366`）/ 透视对（`:451`，36B 布局 `perspVertexLayout`）/ 固定混合族（`:539`）；36B 顶点=pos2+uv2+tint4+hom_w1（`image_pipeline.go:48-52`） | `render/internal/gpu/image_pipeline.go` |
| 透视 VS | `textured_quad_persp.wgsl`：VS 吃 `@location(3) hom_w` 出真 clip-space w，但 **hom_w 不作为 varying 传给 FS**（FS 只有 tex_coord/tint）；RRect SDF 裁剪在 FS 内（`rrect_clip_coverage`），ClipParams 在 @group(1) | `render/internal/gpu/shaders/textured_quad_persp.wgsl:28-48`、`:77-94` |
| 材质三入口并存 | `ShaderEffect`（GPU，零生产消费者，唯一消费者是 FillPath）/ `CustomBrush`（CPU 颜色函数，`render/brush_custom.go:19-46`）/ `engine/fx.Custom`——后两者与 3a 无涉，本批不合并（登记） | `render/brush_custom.go` |

## 2 Godot 2D 材质形态（本批判据源，HEAD `ca871ccc9c` 实测）

1. **材质挂 item 不挂命令**：`canvas_item_set_material(RID item, RID material)` 只是 `canvas_item->material = p_material` + 队更新（`servers/rendering/renderer_canvas_cull.cpp:1977-1983`）；命令结构体全表无 material 字段（`renderer_canvas_render.h:181-193` 的 10 种命令，命令内变换 `CommandTransform{Transform2D xform}` `:286-287`）。
2. **存储与继承**：`Item` 持 `RID material`（`renderer_canvas_render.h:345`）+ `Item *material_owner`（`:362`）；子 item 默认继承最近设材质的祖先（批取 `ci->material_owner == nullptr ? ci->material : ci->material_owner->material`，`renderer_canvas_render_rd.cpp:2212`）；`canvas_item_set_use_parent_material` 是显式继承开关（`renderer_canvas_cull.cpp:1984-1990`）。
3. **材质是合批断点，且断点顺序=clip → material → transform**：`_render_canvas_item` 批循环里先比 `final_clip_owner` 再比 `material != current_batch->material`（断批同时取 `CanvasMaterialData`），transform 断批在 `_record_item_commands` 内（`renderer_canvas_render_rd.cpp:2205-2232`）。
4. **用户片元是模板贴片，不是独立管线**：canvas.glsl 主模板在 `#CODE : FRAGMENT`（`servers/rendering/renderer_rd/shaders/canvas.glsl:701`）处贴入用户 fragment() 代码——用户代码**前后都是引擎代码**（贴片前：基色/UV/法线等引擎变量已备好；贴片后：法线、灯光、`frag_color = color` 输出段 `:856`）；一份管线跑「引擎管道 + 用户代码」，无读回。顶点侧同构（`#CODE : VERTEX` `:274`）——**Godot canvas 材质允许用户动顶点，本批 D8 拍板引擎持顶点级、用户只写片元**，差距登记不冒充。
5. **材质取数据按批**：`CanvasMaterialData` 在断批时从 material storage 取（`renderer_canvas_render_rd.cpp:2222-2226`），材质 uniforms 与基础 uniforms 分属不同 bind group（管线布局按材质声明派生）。

**含义**：Godot 的材质=「引擎模板 + 用户代码贴片 + 独立绑定组 + item 级挂载 + 合批断点」，五个特征里本批 3a 对齐前三个（骨架），后两个归 3b（挂载与合批谓词）/3e-3（scene 编码 item 级）。

## 3 关键设计分叉（本批要定的事）

### 3.1 组合机制：引擎模板合成（推荐，Godot 同构）vs 用户模块直当入口

- **B（用户模块直当入口）**：用户 `fs_main` 就是管线片元入口。死结在引擎后处理——材质窗要求「灯光染色共存」（批 6 规格，`design.md` §6），即引擎必须在用户代码**之后**乘 tint/透明度；裁剪的 RRect SDF 快腿也在 FS 内。用户模块当入口 ⇒ 引擎没有任何代码在用户输出之后跑，语义不完备。
- **A（引擎模板合成，推荐）**：管线片元=引擎生成的组合模块——引擎前奏（无绑定声明，用户绑定原样保留在 @group(0)）+ 用户源码（`@fragment fn fs_main` 改写为可调用普通函数）+ 引擎 `fs_main` 包装（按用户声明的前缀传参调用户函数，之后按引擎序后乘 `opacity × clip_cov [× tint]`）。**组合在 `NewShaderEffect` 期做，组合产物重跑 Parse→Lower→Validate 全链**——手术 bug 当场具名拒绝，不进后端（与既有前置校验同一道闸）。
- A 的对标证据：Godot `#CODE : FRAGMENT` 模板贴片（§2.4）；Skia `SkRuntimeEffect` 产出 `SkShader` 可填任意几何（design §4.1 已引）。
- 绑定布局分配（A 之下）：@group(0)=用户原样（uniform binding(0) + 纹理对 binding(1)/(2)，材质路 binding(1)/(2) **绑精灵自己的纹理**——语义对齐 Godot `TEXTURE`/`UV`，sprite 采样即 item 纹理）；@group(1)=引擎（ClipParams 已在 group(1) 的既有形态，`textured_quad_persp.wgsl:48`；引擎 transform/opacity 归入同一组或 group(2)，方案文档定）。用户源零改写风险。
- 恒等材质可逐位钉：用户 `fs_main` 字面 `return textureSample(...)` 且引擎后乘结构与标准 FS 完全同序（`texel * opacity * in.tint`，opacity 已含 clip）⇒「恒等材质 ≡ 无材质」可做逐位像素钉（这是 A 的可测红利，B 做不到——B 根本没有引擎后乘）。

### 3.2 tint 语义：声明即接管（推荐，Godot 同构）vs 引擎必乘

- **T2（推荐）**：用户片元声明了 `tint` 输入 ⇒ tint 应用是用户的事（引擎不再后乘 tint，只后乘 opacity×clip）；未声明 ⇒ 引擎后乘 tint（存量语义不丢）。对标 Godot：`COLOR` 进用户代码前已含 modulate（`color_interp * texture`，canvas.glsl），用户写 `COLOR=...` 即接管。
- T3（引擎必乘 + tint 输入仅"知情"）：无双乘就要禁用户乘，脚枪换了一处还是脚枪。
- 双乘脚枪无论选哪个都要在契约文档里写死一句。

### 3.3 premul 双路口径（design §10 P3 探针，批 3a 前必做）

- 现状：填充路=用户输 premul → RT 读回 → `premulToStraightRGBA` → 按 straight 进合成链（`user_shader_fill.go:86`）；quad 片元路无读回，用户输出直接进混合。两条路对同一份 shader 必须同画面。
- design 预案：**用户 shader 一律输 premul**，quad 路按 premul 混合（`one, one-minus-src-alpha`），填充路的去预乘步改等价混合方程而非数值往返；**改前先探针实测两路是否一致**，不一致按实测定改法。
- 探针同时是组合机制 A 的可行性实测（组合模块能否过本仓前端 + naga/gles 翻译链）——一石二鸟，落 `batch3a/probe-p3-premul.txt`。

### 3.4 一份 effect 双路可用（fill wrapper 默认值，推荐）

声明了 `tint/hom_w` 输入的 effect 走 `Paint.Shader` 填充路时，填充路包装以默认值喂参（tint=恒等、hom_w=1、opacity=1），后处理仍走读回+coverage 链 ⇒ 一份 effect 两种几何都能填（Skia `SkShader` 同语义），能力面不劈叉。代价：填充路 pass1 也换组合模块（同一生成器参数化），`ensureUserShaderPipeline` 的「用户模块直当入口」形态退役（接口全量切换制，一次改完）。

## 4 影响面清单（3a 触及 / 不触及）

触及（3a）：
- `render/shader_effect.go`：`validateShaderContract` 扩参数面（`@location(0) uv` 必需、`@location(1) tint: vec4<f32>`、`@location(2) hom_w: f32` 可选、声明前缀序）+ 新增组合模块生成与组合期校验。
- `render/internal/gpu/user_shader.go`：`userShaderCache` 键从单 effect 扩为 {effect × 变体}；材质顶点模块（36B、输出 uv/tint/hom_w 三个 varying）；管线族按（变体 × 裁剪腿）建。
- `render/internal/gpu/user_shader_fill.go`：填充路换组合模块（默认值喂参）；读回结构暂不动（池化仍是在册尾巴）。
- `render/internal/gpu/image_pipeline.go`：`ImageDrawCommand` 加 `Material *ShaderEffect`（内部类型，3a 加字段、3b 接公开选项）+ 材质腿的绘制路径。
- 新测试档 `render/p37_3a_*_test.go`（外部钉）+ 包内钉。

不触及（登记归位）：
- 3b：三个公开入口选项加 `Material` 字段、合批谓词加材质断点（`image_pipeline.go:1177` 谓词 + CPU 腿表态）。
- 3c：binding 3+ 多纹理、uniform 预算放开到设备 limits（D4 加重档；本批先把 3a 的绑定布局写成可扩展形态）。
- 3d：描边快路吃 shader（`gpu_render_context.go:2421`、`sdf_gpu.go:439`）+ 七枚具名错误。
- 3e-3：scene 材质编码（item 级，随 3a–3d 之后）；录制面 `FillPathCommand` 材质引用（3e）。
- 批 6：材质真窗（灯光染色共存）与 2.5D 场景级接线。

## 5 口径预登记

- 3a 交付后材质仍**零生产消费者**（消费者在 3b/批 6），状态=「已验证待接线」，台账不得标生产。
- 存量金图零变化是验收项：填充路换组合模块属于 P3.2 既有路径的内部改形态，**逐位不变必须实测钉住**（premul 探针 + 存量 P3.2 测试全绿 + 真窗 `ui_wr_p32_shader` 复跑）。
- 前置探针 P3（premul 双路 + 组合可行性）在动 3a 代码之前跑，结论落文件，禁止「应该一致」进实现依据。
