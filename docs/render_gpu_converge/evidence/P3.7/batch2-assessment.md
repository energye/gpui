# P3.7 批 2 · z 序与世界高度归位 — 影响面评估（2026-11-04，开工实测）

- 分支 `feat/p2-收敛整合`，HEAD 为本会话批 1d 收口后状态（批 1d 四枚提交未推送）。本文档零代码变更。
- 方案依据 = `design.md` §3（批 2 条款）+ D3/D3b 拍板 + 探针正本 `probes/p1-draw-order.txt`。
- 标准对齐：2.5D 优先对齐 Godot（源码 `../godot`，逐点对拍见 §2；本会话已另派研究员核对 `Node2D` y-sort/z_index 语义）。

## 1 现状（四套并存，全部实测到行号）

| # | 真源/副本 | 数据与键 | 消费者（非测试） |
|---|---|---|---|
| 1 | `engine/camera/ysort25d.go`（**设计指定的真源**） | `YSortItem{Pos core.Vec3, Order int}`（`:36`）；键 `YSortKey = Y + 0.001*(X+Z)`（`:43-44`）；`YSortLess` 平票回落 `Order`（`:49-55`）；`sort.SliceStable` + `MaxYSort25D=4000` 拒超量（`:60-70`）；`YSortZ(rank)` 槽函数（`:72`） | `examples/engine/25d_basis/main.go:169,386,522`（三处）+ 单测 |
| 2 | `engine/sprite/ysort.go`（**副本，设计清单未列**） | `Layer int`（`LayerWorld=0/LayerFX=1/LayerUI=2`，`:22-30`）+ `Item{Name,Layer,FeetY}`（`:37-50`）；`Sort/IsSorted` 按 Layer 再 FeetY | `examples/engine/stage_chase/main.go:227,234` + 单测 |
| 3 | `engine/sprite/depth.go`（副本，设计已列） | `DeepItem{Name,Depth,Layer,FeetY}`（`:23-27`）；`deepLess` 三级键：**Depth 降序 → Layer 升序 → FeetY 升序**（`:58-66`）；`DepthSort`（`:73`）/`IsDepthSorted`（`:92`）非有限 fail-closed | `examples/engine/stage_chase/main.go:241,910`（同一窗同时用 #2 与 #3！）+ 单测 |
| 4 | `render/depth_r5.go`（**分层越界副本**） | `DepthSprite{Name,Sprite,Depth}`（`:43`）；`SortDepthSprites` 只按 Depth 降序（`:106-120`）；`IsDepthSorted`（`:123`）；`DepthDrawOptions.DepthTest bool`（`:51`）让 render 内部自排序 | `examples/engine/sprite/main.go:193,196,199,411-413,483,484`（八处，含 true/false/同深三组对比展示）+ **录制面契约**：`render/canvas.go:157`（Canvas 接口）、`recording/backend.go:162`、`recorder_canvas_ops.go:249`、`backend_canvas.go:372`、`bounds.go:152`、`examples/ui_render_base_recording/scene.go:555` spy |

**同一个物理量（远近）四套表达**，且第 3 套自己把 y 换算成深度：`stage_chase/main.go:905` 写 `sprite.NewDeepItem("car", 4000-(s.carY+carH), …)` ⇒ `Depth 降序 ≡ Y 升序` 的活证据。

## 2 四条硬事实（决定修法，全部实测）

1. **真源当前的键装不下 `Layer`**：`YSortKey` 是单一标量（`Y+0.001(X+Z)`）+ `Order` 平票，**没有"分层优先"的位置**；而 #2/#3 的第一比较级都是 `Layer`。⇒ 直接"改委托"会**丢 Layer 语义**（UI 永远盖住 FX、FX 盖住世界这条规则没了）。要么真源键升成两级（先层后键），要么把 Layer 编码进标量（Layer×1e6 之类，脏且有精度悬崖）。**这是本批第一个必须拍的点。**
2. **render 侧的排序口是分层越界**：AGENTS「渲染分层归位」规定"只服务一家的不进 render——玩法与业务归各自模块库"。`SortDepthSprites/IsDepthSorted/SetDepth/DepthTest` 是**玩法排序**不是通用图形能力 ⇒ 该迁走（设计 D3b 已定：键与公式在 camera 一份，`DepthTest` 删除，`DepthSprite` 数据格式随迁保留）。
3. **删 `DepthTest` 会改真窗画面**：`examples/engine/sprite` 窗有意把 `true/false/同深` 三组并排展示（main.go:193-199、411-413、483-484），删开关等于删掉"不排序"那一臂 ⇒ **该窗 golden 必然变化**，金图重出需用户批准并记原因（AGENTS 有先例条款）。测试数据 `render/testdata/…`（`depth_r5_test.go:38` 读 JSON 的 `depth_test` 字段）同步要改。
4. **盖序不变式只有探针、没有永久钉**：`probes/p1-draw-order.txt` 八组混排（跨 tier 双向/同 tier 双向/三段往返/纯路径对照）全 PASS，结论"屏幕盖序 = 提交序"（`gpu_render_context.go:4134-4139` 的 tier + `ensureDrawOrder:4169` 只在回退封批）；但探针测试**跑完即删**（现仓无 `TestP37ProbeP1TierVsSubmissionOrder`）⇒ 批 2 要把它转成常驻钉，否则"画家算法前提"会被后人改 tier 时无声破坏。

另两处口径事实：`YSortZ`（`:72`，基槽 −4000/步长 2/奇数留影子）**全仓零消费者**，`shadow25d.go:22-23` 声称"影子放主人 z 槽上一格"，实测 25d_basis 把影子画在地面线固定 y（`main.go:556-558`）⇒ 属未落地叙述；`engine/world/entity.go:29-32` `Transform{Pos core.Vec2; Rot; Scale}` **无高度字段**，而投影侧 `camera/project.go:59 DepthToScale(depth)` 与批 1c 的 `ProjectSpriteQuad(worldRect, depths[4])` 早已吃"深度"标量 ⇒ 高度缺的是数据入口，不是算法。

## 3 待拍板项（停下报告，逐项拍完才动代码）

- **D-A 真源键的形态（最关键）**：(1) 键升成两级 `Layer` 优先 + 层内 `Y+0.001(X+Z)`（对齐 Godot 的 z_index→y-sort 结构，改 camera 键与签名，`YSortItem` 加 `Layer`）；(2) 键保持单标量，把 Layer 乘大基数折进键（不改签名，精度有悬崖）；(3) 只统一"层内序"，Layer 分组留给调用方（改动最小，但 #2/#3 不能真委托，副本仍在）。
- **D-B `Depth`/`FeetY` 映射**：`DeepItem{Depth,Layer,FeetY}` 喂 `YSortItem{Pos}` 时三个量各放哪个分量（`Y=FeetY`？`Z=Depth`？）；硬约束：**新旧输出逐元素相等**（拿 stage_chase 1000 prop 现序 + 现有单测全集做对拍输入），否则动的是行为不是实现。
- **D-C `DepthTest` 删除后 sprite 窗怎么办**：(1) 删臂 + 重出 golden（记原因）；(2) 窗改用"engine 侧排序 vs 不排序"两条路径对比（保住这一臂的演示价值，画面不变则 golden 不动）；(3) 保留 `DepthTest` 但改成"输入序已排好"的纯诊断（违背"不留垫片"，不推荐）。
- **D-D `Height` 的口径**：`Transform.Height float64`（additive、校验有限、不进 `LocalMatrix`）已定；待定的是 **Height 进不进排序键**（Godot 不管高度；我们若让"高个子站得高但排在后面"会反直觉），以及影子/投影各自怎么取用（投影深度输入 = `depth = -Height`？还是显式传）。
- **D-E `YSortZ` 与影子槽**：(1) 按"标准能力零消费者也保留"不动符号、只改注释与 doc.go（设计原意）；(2) 让 25d_basis 的影子真用上 z 槽（把未落地叙述做成事实，但要动窗画面）；(3) 判为自有约定而非标准能力 ⇒ 删除。
- **D-F 老 `DrawAtlas` 与新 `DrawAtlasEx` 双入口**：批 1d 已把三口投影腿合并为一份实现，但入口仍双。归本批"副本改委托"一起收（老口改薄封装转调 Ex），还是留独立小批？

## 4 能力单测（拟，先红后绿）

1. **真源对拍钉**：stage_chase 1000 prop 现序 + #2/#3 现有单测全集为输入 ⇒ 新旧排序输出**逐元素相等**（D-A/D-B 的结果性验收）。
2. **委托后行为不变钉**：`sprite.DepthSort/Sort` 与 `camera.YSort` 同输入同输出（逐位）；非有限输入的哨兵错文案与形状不变（能力面随迁不裁）。
3. **盖序不变式永久钉**：P1 八组混排转常驻测试，判据"屏幕盖序 = 提交序"，容差 0，双后端（native/gles）各跑；摘 tier 或改 `ensureDrawOrder` 必须红。
4. **`Height` 钉**：字段校验（非有限拒绝）、`LocalMatrix` 逐位不变（高度不进仿射）、投影深度取用与排序键取用各自可断言。
5. **render 侧删口回归钉**：`DepthTest` 删除后 `DrawDepthSprites` 只按输入序绘制（输入序由调用方排好），录制面命令随全量切换改签名/字段，spy/stub/raster 后端同批改。
6. 负向（拟）：把委托改回副本（`sprite.DepthSort` 恢复自带 `deepLess`）⇒ 真源对拍钉仍绿说明"假委托"不可接受 ⇒ 补"键公式只有一处定义"的结构钉（grep 断言或行为钉：改 camera 键后 sprite 输出必变）。

## 5 风险与登记

- 风险面按大小：① D-A 键形态牵动三个真窗（stage_chase/25d_basis/sprite）与录制面契约；② 删 `DepthTest` 改 sprite 窗画面（golden 需批准重出）；③ `IsDepthSorted` 系列语义迁移中"哨兵文案/nil 契约"回归。
- 存量零变化验收项：每批必跑三窗双后端金图；本批若选 D-C(2) 可保住 sprite 窗画面不变，选 (1) 则必须重出并记原因。
- 不做：精灵 z-test（D3，深度附件归裁剪）、`ShapeCast3D`（`shadow25d.go:13-19` 自陈缺该环）、scene 投影编码（批 3e）。

## 6 Godot `Item Y-Sort` 逐点对拍（研究员实测本树，4.8-dev，路径 `/home/yanghy/app/projects/gogpu/godot`）

> 路径与版本以本树为准：`version.py`=4.8-dev；`scene/2d/node_2d.cpp` 里 `y_sort` 命中数 **0**，`compute_y_sort_origin`/`y_sort_source`/`y_sort_reorder` **已不存在**（被移除）；`y_sort_enabled` 上移到 `scene/main/canvas_item.cpp`，真正排序发生在渲染服务 cull 阶段。下面每条都有 file:line。

| 语义点 | Godot 怎么做 | file:line | 对我们真源的含义 |
|---|---|---|---|
| 比较键 | 只比 `ysort_xform.origin.y`（节点原点，**不含** AABB/纹理底边）；`is_equal_approx` 判平票，平票比 `ysort_index` | `servers/rendering/renderer_canvas_cull.h:117-127`；`math_funcs.h:552-562` | 我们 `YSortKey=Y+0.001*(X+Z)` 把 x/z 揉进一个连续标量 ⇒ **与标准结构不同**；标准是「epsilon 判等 + 显式序号」两级 |
| 平票打破 | `SortArray::sort` 是 introsort（**不稳定**），但 comparator 是全序（`ysort_index` 唯一）⇒ 结果确定 | `sort_array.h:298-307`；`renderer_canvas_cull.cpp:132-152` | 我们用 `SliceStable`+`Order` 行为等价，但真源要**声明靠序号破平**而不是靠算法稳定 |
| 序号来源 | 每帧 cull 按子节点**树序** DFS 压平赋值（=场景兄弟索引） | `renderer_canvas_cull.cpp:135,152`；`canvas_item.cpp:523-534` | `Order` 应定义为「容器内序号」，**不是提交序号** |
| origin 空间 | `ysort_xform = 父.ysort_xform × 子局部 xform`，含物理插值与像素吸附；组根置 identity | `renderer_canvas_cull.cpp:119-132,447` | 排序空间是「y-sort 宿主局部系」⇒ 插值/吸附必须**先于**排序发生（我们有同类次序约束：批 1d 的链序） |
| 继承与范围 | 只对开启者的**子孙**生效；宿主自身占 index0；未开启的子整棵子树跟随其槽位；**不跨 CanvasLayer**（层序压过 z_index）；不影响 process/输入序 | `renderer_canvas_cull.cpp:444-460,471-491`；`doc_classes/CanvasItem.xml:705,707`；`RenderingServer.xml:643`；`CanvasLayer.xml:7` | 标准里有「组/槽位」概念，我们当前是**一维全局数组** ⇒ 是否引入必须拍（见 D-G） |
| z 与 y 的优先级 | z 是**外层键**：分桶链表 `r_z_list[z]`；y 只决定桶内次序；文档明写「同 z_index 才互排」；z 范围 ±4096 | `renderer_canvas_cull.cpp:79-100,274-285`；`CanvasItem.xml:706`；`rendering_server_enums.h:40-41` | **真源缺的就是这个外层分桶** ⇒ D-A 的两级键有标准依据；`deepLess` 把 depth/layer/feetY 串成一维比较，语义与标准不同 |
| 跨层级 z 合成 | `z_as_relative` 逐级累加且**每级 CLAMP**；绝对化时直接取自身 | `renderer_canvas_cull.cpp:431-436,145-150`；`canvas_item.cpp:785-795` | 若引入 zIndex，需同时定「是否饱和截断」（Godot 是） |
| 高度概念 | 2D **无离地高度**；`Sprite2D.offset/centered` 只影响绘制矩形、**不进排序键**；3D 无 y-sort（透明队列按相机距离 + `sorting_offset`）；`Light2D.height` 只供阴影 | `sprite_2d.cpp:116-136`；`render_forward_clustered.cpp:981-986`；`visual_instance_3d.cpp:157-159`；`light_2d.cpp:485` | **Height 不进排序键**有标准依据（我上一条推荐方向正确、当时无据，现补上）；我们加"世界高度"本身是**超出 Godot 的新增语义**，其入键方式必须自证 ⇒ `design.md` §3「高度参与遮挡排序键」那句与标准冲突（见 D-D 修订） |
| 序 = 盖序 | 排序序 → 桶内 append 序 → 单条 `items[]` → 一个 draw_list 内按 batch 索引顺序提交；无深度测试 | `renderer_canvas_cull.cpp:458-460,82-100`；`renderer_canvas_render_rd.cpp:897,2198-2254,2304` | 画家算法前提在 Godot 是**结构性保证**；我们的 P1 探针（八组全 PASS）转常驻钉时，判据按此写 |
| 跨提交层 | 遇 screen-texture / canvas-group / SDF-lighting 切换**就地 flush**（只切分、不重排） | `renderer_canvas_render_rd.cpp:820,852,876,906,2201-2233,2395-2399` | **我们没有对应机制描述** ⇒ 盖序钉要写成「tier 切换只 flush 不重排」，若实测发现某 tier 切换会重排，要修 render 而不是放松判据 |

**Godot 有、我们没有对应机制的三项**：① z 外层分桶（本批补，D-A）；② epsilon 判等（现用精确 `!=`，浮点脆，本批补）；③「宿主局部系 + 组/槽位」的继承模型（范围问题，D-G 拍）。

## 7 拍板项修订（按 §6 证据改判，两条与上一条我给的建议不同）

- **D-A 维持两级键方向，但补三条硬条件**（都是标准里明确有的）：外层是「z/层」分桶、层内比 origin 键、**epsilon 判等**、平票靠**显式容器序号**（不再以 `SliceStable` 为语义依据）。待定：外层键用什么承载（复用 `sprite.Layer` 三值 / 新增 `ZIndex` ±4096 级联 CLAMP / 两者），以及 CLAMP 是否要。
- **D-D 改判为有依据**：Height **不进**排序键（Godot 2D 无此物、`Sprite2D.offset` 明确不进键），只作投影深度；⇒ 同时**必须修订 `design.md` §3 那句"高度参与遮挡排序键"并记原因**（真源不合一不能留）。
- **新增 D-G（范围）**：要不要引入 Godot 的「组/槽位」模型（只对开启者子孙生效、宿主占 index0、不跨 CanvasLayer）。做 = 排序从一维数组变按容器分组，牵 world/sprite 数据结构；不做 = 明写"本批只做全局一层"并登记后续。
- **新增 D-H（冻结口径）**：`engine/camera/doc.go:94-95` 把 `MaxYSort25D/YSortZBase/YSortZStep/YSortKey/YSortLess/YSort/YSortZ` 定为**只加不改**。两级键要么申请解冻（开发阶段"接口全量切换制"允许，但要在 doc.go 记原因），要么加法式新增 API、旧 API 内部委托，要么只补 epsilon 不改签名。我上一条把"改签名"写得太轻，这里必须你定。
