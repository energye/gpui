# 批 2 · z 序与世界高度归位（2026-11-04，交付；**未提交**）

## 结论

**远近这件事从四套表达收成一套真源**。原来 `engine/camera`（一维数组＋`Y+0.001*(X+Z)` 标量键＋精确比较）、
`engine/sprite` 的带序口、`engine/sprite` 的深度口、`render/depth_r5.go` 的 `DepthTest` 各自决定"谁盖谁"；
现在判据只在 `engine/camera` 一份：**外层 z 桶（±4096，相对 z 逐级累加且每级 CLAMP）→ 层内 origin 键（Godot
`is_equal_approx` 公式判等）→ 容器序号破平票**，`engine/sprite` 两口径改委托，`render` 只按提交序画、
不再自带排序开关。实体侧补上缺的数据入口 `world.Transform.Height`（只喂投影，不进仿射链、不进排序键），
盖序不变式从"跑完即删的探针"变成常驻钉。

**标准对齐**：判据逐条按 Godot 真源写并有 file:line——比较器 `renderer_canvas_cull.h:117-127`
（`is_equal_approx(origin.y)` 否则 `ysort_index`）、z 累加与每级 CLAMP `renderer_canvas_cull.cpp:145-150,431-436`、
z 范围 ±4096 `rendering_server_enums.h:40-41`、判等公式 `math_funcs.h:552-562` ＋ `math_defs.h:50`（`CMP_EPSILON=1e-5`）、
demo 变体键 `godot-demo-projects/misc/2.5d/addons/node25d/node_25d.gd:134-141` 与 4000 节点护栏
`y_sort_25d.gd:30-33,43-48`；tier 切换"只 flush 不重排"对 `renderer_canvas_render_rd.cpp:820,852,876,906`。
评估与逐点对拍正本 `batch2-assessment.md`（§6 表格），方案与改判正本 `batch2-design.md`（§1 五条 ＋ §8–§12 分步交付）。

**三处对设计稿的改判**（都因实测真源而与草稿不同，理由写进 `batch2-design.md` §1）：
① 判等不取草稿估的平坦 `1e-6`，照抄 Godot `max(ε, ε·|a|)` 公式；
② D-G 草稿写"子树整体跟随祖先槽位"——Godot 无此条，比的是各节点自己的容器系 Y，
   DFS 序号只破平票，钉与注释按事实改；
③ 变体键若只是留一个没人调的函数＝装饰，故加三条带键的对口入口，让 demo 形态真能选。

## 一、改动（按步）

| 步 | 位置 | 内容 |
|---|---|---|
| 2a | `engine/camera/ysort25d.go` | 比较器两级化（`ZIndex` 桶 → `ySortEqualApprox` 键 → `Order`）；`YSortKey` 拆成 `YSortKeyY`（标准）＋`YSortKeySlightXZ`（demo 变体）；新增 `YSortKeyFn`/`YSortWithKey`/`YSortTreeWithKey`/`YSortLessWithKey`/`ClampZIndex`；容器模型 `YSortNode`/`YSortTree`/`YSortEffectiveZ`（父链 z 累加＋每级 CLAMP、环/坏父/非有限/4000 上限全 fail-closed）；D-H 解冻（`doc.go` 记原因与日期） |
| 2b | `engine/sprite/ysort.go` `depth.go` `doc.go` | 删本地 `less`/`deepLess` 副本；带→z 桶 `ZIndexForLayer=camera.ClampZIndex(camera.YSortZ(band))`；`Sort`/`DepthSort`/`IsSorted`/`IsDepthSorted` 判据全交 `camera.YSortLess`（`Depth` 留作本仓扩展外层轴）；**没把 demo 的 4000 上限塞进 sprite 公开口**（那是 z 槽算术护栏，不是排序判据）；公开签名零改动 |
| 2c | `render/depth_r5.go` `canvas.go` `recording/*` | 删 `SortDepthSprites`/`IsDepthSorted`/`SetDepth`/`DepthDrawOptions`＋`DepthDrawResult.Sorted`；`DrawDepthSprites(img, sprites)` 只按提交序画；Canvas 接口、`Backend` 契约、`DrawDepthSpritesCommand`（去 `Opts` 字段）、`Recorder`、`CanvasBackend`、分发、bounds、raster 后端、录制 spy 全量切换不留垫片 |
| 2c | `render/testdata/depth_r5_cases.json` `depth_r5_test.go` | 数据同批改：去 `depth_test`、`three_layers` 写成提交序、删假语义用例 `far_last_input`、`depth_off`→`submission_order`；测试重排并**新增常驻哨**：同批精灵换提交序两张画面必须逐位不同（render 再自己排序就红） |
| 2c | `examples/engine/sprite/main.go` | 八处调用按 D-C(2) 换臂：`depthFarFirstSprites` 先交 `sprite.DepthSort` 排好再提交，对比臂保持"故意近先远后"；`depth_sorted` 指标改由 engine 侧核对 |
| 2d | `engine/world/entity.go` `prefab.go` `scene.go` `doc.go` | `Transform.Height`（additive，非有限拒收，不进 `LocalMatrix`/`WorldMatrix`，不进排序键；父链按普通相加折叠）；堵"文件吞高度"洞：`jsonTransform` 加可选 `height`（`omitempty` ⇒ 零高度旧字节逐位不变），`PrefabVersion`/`SceneFileVersion` 按自身规则 1.0→1.1，1.0 旧档照装读 0 |
| 2d | `docs/.../P3.7/design.md` §3 | "高度参与遮挡排序键"原句按 D-D 改判重写并记日期依据（Godot `sprite_2d.cpp:116-136`、`renderer_canvas_cull.h:117-127`：offset 不进 y-sort 键）；§3 真源/副本两条加交付指针 |
| 2e | `render/p37_draw_order_test.go` | P1 八组混排探针转常驻钉：A/B 跨 tier 双向、C/D 同 tier 双向、E/F 三段往返、G 两次跨 tier、H 纯路径对照；判据＝交点容差 0 等于最后提交者＋角落不污染＋`GPUOps` 批次数按组钉死；再加 CPU/GPU 两腿同色钉；边界明写"单容器一帧内" |
| — | 目录 | `docs/RENDER_API_CATALOG.md` 主包 154/197→**153/194**（go/ast 实测）＋§3.9 删口两行改写；`docs/2.5D/G04.md` 10.1 行补 Height；`docs/ENGINE_RENDER_GPU_CONVERGE_TASKS.md` §2.1 进度行＋§7 流水行 |

**新增测试文件**：`engine/camera/p37_ysort25d_test.go`（7 枚）、`engine/sprite/p37_depth_parity_test.go`（3 枚＋语料）、
`engine/world/p37_height_test.go`（5 枚）、`render/p37_draw_order_test.go`（2 枚八组）、
语料生成器 `scripts/gen_p37_zsort_parity.py` ＋数据 `engine/sprite/testdata/p37_zsort_parity.json`（11 案含两条 1000 项）。

## 二、行为差异（唯一一条，刻意且已钉住）

脚位差落进 Godot 判等容差 `max(1e-5, 1e-5·|y|)` 内时，新旧判据不同：新判据按平票处理、由**容器序号**定序，
旧副本按最后浮点位比 Y。|y|=600 处容差 6e-3 世界单位 ≈ 0.19px（1 单位≈32px），肉眼不可见，
换来的是插值抖动不翻序。D-A′ 拍板里"epsilon 判等"本就是这个含义。
对拍语料里那条 5e-4 差的用例实测被吞进平票 ⇒ 从对拍集移除并改写成 `gap_outside_tolerance`（0.5 单位差，仍逐位一致），
平票行为由 `TestP37SpriteSortUsesCameraTolerance` 单独钉。

## 三、门禁（真实数字，证据全在 `batch2/`）

- **逐文件 sweep**：760 = 729 PASS / 21 SKIP_TAGGED / 9 FAIL / 1 NO_TESTS（基线 759=728/21/9/1）。
  ＋4 全为本批新文件且全绿；FAIL 名单 −`render/mem_window_x11_linux`（回绿）＋`ui/rendering/m5_keystroke_g1`。
  新那枚同机交替 A/B：本树 3 轮 1.35/2.10/2.01 红，纯净 HEAD 同刻 0.89–1.93 也红 ⇒ 墙钟漂移族，非本批
  （`ui/rendering` 一行未碰）。明细 `gates/new-fails-ab.txt`。
- **五目标构建**：10/10 绿（linux-amd64 335／linux-arm64 335／windows-amd64 341／darwin-amd64 333／darwin-arm64 333，
  各自 nogpu 331/331/337/329/329）。`gates/build/*.txt`。
- **`-race`（精确口径）**：camera ok 1.1s、sprite ok 3.2s、`render/recording` 与 raster 后端全绿、
  `render` 本批触及文件（`TestP37DrawOrder*`/`TestDepthR5*`/`TestP37DepthSpritesCarryPersp`）ok 2.9s、
  `engine/world` 除墙钟预算门禁外 ok 2.9s（`-run 'TestP37Height|TestTransform|TestWorld|TestScene|TestPrefab|TestEntity'`）。
  两条**非本批**红已同机 A/B 定性并登记：①整包 `-race ./render` 在 wgpu-native `conv.rs:519` abort（HEAD 同红、单测隔离两绿）；
  ②`engine/world/TestCullBudgetLarge`（1655 实体平均 ≤3.0ms）在 `-race` 插桩下 15.96ms 必红，
  且**并行合跑时基线 3/3 红（4.80/4.54/3.80ms）、本树 1/3 红**，两树单跑各 3/3 绿 ⇒ 工具口径冲突，非本批性能回归。
  `race.txt`、`gates/world-cull-budget-ab.txt`。
- **conformance 双后端**：`TestConformanceDualBackend` native PASS 0.78s / go PASS 0.86s。`conformance.txt`。
- **真窗**：五窗（`25d_basis`/`stage_chase`/`sprite`/`world`/`camera`）× native/go 各一轮，全部正常自管关闭、
  每轮前后 Xwayland=1（无 Xorg 崩溃）。**像素面十轮全绿**：probes ok=true、golden changed=0、
  sprite 窗 golden/parity 0.0000%、`OK presents=…` 收尾齐（stage_chase 两轮因护栏 120s 恰切在其自身
  `runSeconds(120)` 边界上，先被误记为 rc=0，已加护栏 300s 重跑并如实登记）。
  **§5 帧门 `stage_chase` p95 红**：本批 22.95(native)/23.07(go) vs 基线 HEAD 23.13、修排序前 17.51/23.11——
  四读同带、平均帧率一律 ~59.76fps、像素零差 ⇒ 定性为在册尾帧漂移族（先例 `s6_9_heavy_budget`、
  `mem_window_x11_linux` 同族互换），不是批 2 引入。`windows-summary.txt`、`gates/stage-chase-frame-gate-ab.txt`。
- **性能债一条是本批的，已修并反超**：首版委托多带一张键表，100 项档 7.0–8.6µs vs 基线 3.3–3.9µs（≈2×慢）；
  改成排 `[]int` 下标＋栈上构键＋`slices.SortFunc` 后 100 项 3.0–3.5µs、1000 项 189–195µs vs 基线 786–823µs
  （**≈4.2×快**），逐位对拍与真委托钉不变、负向重跑六条全红。`batch2-design.md` §9.1。
- **apidoc**：`go run ./scripts/apidoc` rc=0（渲染域目录覆盖）。

## 四、留下的口子（如实登记，不含糊）

1. `stage_chase` §5 帧门 p95 尾（本批前后同带，基线同红）→ 在册尾帧漂移族，需单列性能专项定位，不放松判据。
2. 整包 `-race ./render` 的 wgpu-native `conv.rs:519` abort（HEAD 复现）→ 新登记在册缺陷，待专项。
3. `engine/world` 的墙钟预算门禁 `TestCullBudgetLarge`（1655 实体平均 ≤3.0ms）与**并行执行**天然冲突：
   同机交替 A/B——合跑四包时基线 HEAD 3/3 红（4.80/4.54/3.80ms）、本树 1/3 红（5.86ms）；
   单跑 world 两树各 3/3 绿。批 2 在 world 只加了一个 `Height` 字段与折叠用的一次浮点加法
   （`Transform` 40B→48B）。正式口径按 AGENTS「逐文件单跑」——sweep 里 `engine/world` 各文件全绿；
   `-race` 下同样必红（§三第 3 条与本条同族）。待给这条门禁定并行/插桩口径（豁免档或单列性能专项），本批不改判据。
   证据 `gates/world-cull-budget-ab.txt`。
4. `world.Transform.Height` 的第一个真消费者（投影深度入口）仍未接线：`camera.ProjectSpriteQuad` 全仓零生产消费者，
   本批只把数据入口与折叠规则打通（`design.md` §3"本批只开数据入口"原话）。
5. `ZIndex`/`YSortTree` 的容器分组语义在 world/sprite 侧还没有真窗载体（D-G 的 2b/2c 只做了判据与委托），
   归批 6 真窗（`25d` 高度 z 序窗）。
6. `engine/light` 另有一套同名三值 `Layer`（`light.go:34-36`）——带轴第四套表达，本批范围外，登记待归位。
7. 影子槽 `YSortZ` 的"奇数槽留影子"仍是约定不是事实（`shadow25d.go` 叙述与 25d_basis 实测不符，批 1d 已记）。
