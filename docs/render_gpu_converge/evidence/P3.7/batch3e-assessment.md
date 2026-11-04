# P3.7 批 3e（scene 投影编码）· 影响面评估（2026-11-04，开工实测）

分支 `feat/p2-收敛整合`，HEAD 批 2 收口后（`1985773d` + 簿记 `2bf20d9c`，均未推送）。本文零代码变更。
用户口径：**一切 2.5D 相关对齐 Godot**；渲染层通用能力按 AGENTS 对齐 Skia/Flutter。本文把这两条落到
`render/scene` 的具体接缝上，并标出两者不重合的地方。

## 1 scene 现状（全部实测到行号）

| 面 | 事实 | 位置 |
|---|---|---|
| 依赖 | `render/scene` **已 import render 主包** ⇒ `render.Matrix4`/`render.Point`/`render.QuadDrawOptions` 可直接用，无环（反向依赖在 `render/internal/gpu/scene_bridge.go:20`） | `render/scene/encoding.go:19` |
| 变换真源 | 只有仿射 `Affine{A,B,C,D,E,F}`（6×**float32**），公开入口 `PushTransform(Affine)`/`Fill(…, Affine, …)`/`DrawImage(*Image, Affine)`；**全树 `Matrix4`/投影 0 命中** | `encoding.go:310`、`scene.go:138/253/422` |
| 双流编码 | `Encoding{tags,pathData,drawData,transforms,textData,brushes,commandBounds,…}`；Tag 共 **19** 个，显式十六进制值（`TagTransform 0x01`…`TagImage 0x51`、`TagText 0x60`） | `encoding.go:417-461`、`tag.go:35-119` |
| 序列化 | **没有字节级 Marshal/魔数/版本号**（「编码」就是内存双流；唯一字节布局是 `textData`，`glyphRunDataSize=30`）⇒ 兼容性风险不在磁盘，在 **`Hash()` 缓存键 + 结构断言** | `encoding.go:402/724-752`、`encoding.go:798`、`cache.go:99` |
| 回放两条腿 | GPU：`GPUSceneRenderer.RenderScene` tag switch（变换用 `dc.SetTransform(base×scene)`，刻意不用 Push/Pop 以免毁裁剪栈；图像走 `DrawImageEx` **只取 A/E/C/F，丢旋转与透视**）；CPU：`Renderer.renderTile` tag switch，**逐命令烘焙仿射**（`currentTransform.TransformPoint` 直接改点） | `gpu_renderer.go:59/84/101/273-305`、`renderer.go:575/673/684-709` |
| 合并口 | `AppendWithImages` 的 tag switch **无 default**（漏加 case ⇒ 图像/画刷索引静默错补）；`AppendWithTranslation` 的 switch **有 default panic**（穷举绊线，新 tag 必登记） | `encoding.go:913-950`、`encoding.go:1041-1122`（panic 在 `:1120-1121`） |
| 消费者 | 零生产消费者：只有 4 扇示例窗（`render_scene`/`render_scene_gpu`/`render_visual_scene`/`render_visual_scene_gpu`）+ `examples/ui_render_base_scene/scene.go`（用到 `RenderWithDamage:311`、`NewGPUSceneRenderer(...).RenderScene:776`、`Encoding.Hash:253`）；`ui/`、`engine/` 全仓 import 清单 **0 命中** | `examples/ui_render_base_scene/scene.go:253/283/311/776` |
| 测试判据风格 | 结构断言（数 tag/drawData：`tagtext_test.go:66-70`）、像素探针（`gpu_renderer_test.go:90`、`gpu_renderer_regression_test.go:25`）、解码回环（`antialias_test.go:113`）、hash 稳定性（`paint_correctness_test.go:283`） | 同左 |

## 2 Godot 2D 命令面（4.8-dev 实测，本项判据源）

1. **2D 变换真源只有一个类型 `Transform2D`，仿射-only**：`core/math/transform_2d.h:58` 是 `Vector2 columns[3]`（两基向量 + 原点 = 6 个数），构造入口只收 6×real_t / 3×Vector2（`:138/:145/:148/:150/:152`），**没有**透视行列、没有从 `Basis`/`Transform3D`/`Projection` 的入口。
2. **RS 的 2D 命令全表里没有任何 3×3/4×4 投影参数**：绘制命令见 `servers/rendering/rendering_server.h:836-855`，唯一矩阵参数是 `canvas_item_add_set_transform(Transform2D)`（`:853`）；item 变换入口 `canvas_item_set_transform(RID, const Transform2D&)`（`:825`，实现 `renderer_canvas_cull.cpp:636→:650`）。
3. **一路到 GPU 都是仿射**：每 item 世界矩阵 `float world[6]`（`renderer_canvas_render_rd.h:357`），打包成 mat2x3（`renderer_canvas_render_rd.cpp:77-84`），shader uniform 是 3 个 `vec2`（`canvas_uniforms_inc.glsl:20-22`）；`canvas.glsl:282` 虽拼了 mat4 但 `vertex = (model_matrix * vec4(vertex,0,1)).xy` —— **w 恒 1、无透视除法**。canvas 侧 `perspective` 0 命中、`noperspective` 全仓 0 命中。
4. **透视内容进 2D 的官方形态 = 算完再喂逐顶点数组**：`canvas_item_add_primitive/polygon/triangle_array` 各吃「顶点 `Vector<Point2>` + 颜色 `Vector<Color>` + UV `Vector<Point2>`」（`rendering_server.h:847-849`），长度校验证明逐顶语义（`renderer_canvas_cull.cpp:1758-1798`：`color_size∈{0,1,pointcount}`、`uv_size∈{0,pointcount}`）；要做真透视校正得调用方自己按 1/w 预除进 UV/颜色，Godot 不代劳。第二条官方路是把 3D 渲进 viewport 再当纹理贴（`scene/main/viewport.cpp:423`）。
5. **保留式命令枚举也没有投影位**：`renderer_canvas_render.h:181-193` 的 10 种 `TYPE_RECT/NINEPATCH/POLYGON/PRIMITIVE/MESH/MULTIMESH/PARTICLES/TRANSFORM/CLIP_IGNORE/ANIMATION_SLICE`；命令内变换结构 `CommandTransform{Transform2D xform;…}`（`:286-287`）。
6. **材质是 item 级，不是命令级**：`canvas_item_set_material`（`rendering_server.h:867`，实现 `renderer_canvas_cull.cpp:1977-1983`），存储 `renderer_canvas_render.h:345 RID material;` + 父级继承 `:362 material_owner`；**所有 Command 结构体里只有 `RID texture`，无 material 字段**；材质只作合批断点（`renderer_canvas_render_rd.cpp:2207,2221-2232`，顺序 clip→material→transform）。

**含义**：2.5D 玩法侧要的是「引擎算好四角+权重再喂」这条正路（与仓里批 1c 的 render 透视腿同构）；
scene 若要 4×4 投影口，那是 **render 层通用能力**（依据 Skia `SkM44` 限定域 + Flutter `DlMatrix = impeller::Matrix`
4×4：`display_list/geometry/dl_geometry_types.h:32`、`flow/diff_context.h:71`、`flow/layers/transform_layer.cc:29`），
**不得**由 2.5D 侧调用。而「材质随保留命令编码」这句（`design.md` §4.5 原文）比 Godot 更细粒度，
按用户拍板改为 **item（节点）级为主 + 命令级另标注为超出 Godot 的扩展**。

## 3 本项要做与不做（范围）

做（拍板结果）：
1. **3e-1 scene 4×4 投影口**（通用层，Skia/Flutter 口径）：`Set/Push/Pop/Projection` 四动词 + 编码 + 回放；
   **2.5D 禁用**，用结构钉把这条边界钉住（`engine/` 与 `ui/` 对 scene 投影零引用）。
2. **3e-2 scene 透视 quad 图像通道**（2.5D 正路，Godot 口径「算好再喂」）：`DrawImageQuad(img, corners[4], opts)`
   编进流，GPU 回放直调 `Context.DrawImageQuad(... Perspective:true, Weights:…)`。
3. 两半各自独立可提交（`design.md` §4.5 早写了「3e 是本项最大单体，允许再拆两次提交」）。

不做（登记）：
- 材质编码（依赖批 3a 的 `ShaderEffect` 能力，且粒度规则已改判为 item 级为主）⇒ 随 3a–3d 之后另议。
- `ui/`、`engine/` 的 scene 接线：scene 至今零生产消费者，本批交付后仍是「已验证待接线」，不得标绿为生产。
- CPU tile 回放腿的真透视采样：按批 1d 的显式表态纪律处理（能仿射提升就提升，不能就**具名计数 + 跳过**，
  禁止静默压扁），不新造一套透视光栅。
