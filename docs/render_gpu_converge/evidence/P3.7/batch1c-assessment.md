# P3.7 批 1c · 批绘腿吃「四角+权重」— 影响面评估（2026-11-04，开工实测）

- 分支 `feat/p2-收敛整合`，HEAD `07e67f11`（批 1b 已提交）。工作区仅本评估文档新增，零代码变更。
- 方法：全部结论来自本轮读码到行号 + 既有探针/在册记录；方案依据 = `design.md` §2.3（D2 拍板形态）+ `assessment.md` §1.4。
- **标准对齐（用户拍板：渲染层对齐 Skia，2.5D 引擎语义优先对齐 Godot；Godot 源码在本地 `../godot`，master `ca871ccc9c`）**：见 §0。
- GPU 测试三件套照旧：`CGO_ENABLED=1 LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so`。

---

## 0 标准对齐（Godot 实测，2.5D 优先）

本地 Godot 源码逐点核对（`/home/yanghy/app/projects/gogpu/godot`）：

| 能力 | Godot 标准实现（file:line） | 形态 | 对本批的判定 |
|---|---|---|---|
| 投影精灵（2.5D/3D 路径） | `scene/3d/sprite_3d.cpp:190-280` | **引擎生成四顶点 quad 数据**（每角 position/UV/逐顶点 color 全部 CPU 侧写进顶点缓冲），透视完全来自 `Camera3D` 投影矩阵，渲染器顶点着色器出 clip-space w 做**透视正确 UV 插值**；精灵本体不带权重、不碰投影 | **与我们 D2 拍板同构**：引擎（camera 出口）算好投影喂 render，render 只收顶点级数据走透视腿。Godot 的「逐顶点 color」= 我们的 `Tint`（1b 的 premul 下发式同位）；Godot 的 clip-space w = 我们的 36B 透视顶点 w |
| 2D canvas 贴图矩形 | `servers/rendering/renderer_canvas_cull.cpp:1667-1712 canvas_item_add_texture_rect_region` | 只记 rect+src_rect+modulate+flags（flip/transpose/clip_uv），**纯仿射（Transform2D），2D canvas 无透视**；Godot 对「要透视的精灵」的标准答案是走 3D 投影路（Sprite3D），不是给 2D 加透视 | 我们的批绘腿保留仿射零值路（=Godot 2D canvas）+ 新增透视承载（=Godot Sprite3D 的引擎预投影形态）——**双层并存与 Godot 能力分层一致**，D2 的「零值=老路逐位不变」由此有标准背书 |
| Y-sort 画家序（批 2 预研，本批不实施） | `servers/rendering/renderer_canvas_cull.h:117-126 ItemYSort` + `renderer_canvas_cull.cpp:439-460` | 排序键=Y，平票回落 `ysort_index`（树序/插入序），**排序后按序绘制=纯画家算法，无 z-test**；分层 ysort 嵌套由 `_collect_ysort_children` 汇集后统一排 | 与 D3/D3b 拍板（画家算法 + `YSort25D` 稳定排序 + 平票回落 Order）同构；本仓键 `Y+0.001*(X+Z)` 是 Godot 纯 Y 的 2.5D 扩展（平票面更细），语义方向一致。批 2 实施时逐点对拍此比较器 |

**结论：批 1c 的 D2 形态（引擎预投影、render 吃四角+权重、零值保仿射）与 Godot 的「Sprite3D 引擎生成顶点数据 + 相机投影 + 渲染器透视插值」是同一职责划分在 2D 画布上的落地，不需要改形态**；render 侧透视插值数学继续以 P3.5 批 3 已对齐的 Skia 口径为准（1/w 有理插值 + GPU clip-space w）。

---

## 1 现状（全部实测到行号）

### 1.1 render 批绘入口：四角与权重无处放，GPU/CPU 两腿都是仿射拆法

| 环节 | 位置 | 实测状态 |
|---|---|---|
| `AtlasSprite` 字段面 | `render/vertices.go:77-95` | SrcX/Y/W/H、DstX/Y/W/H、Opacity、Rot/Flip/Pivot、Tint、Filter、Mipmap——**无四角/权重位**；`AtlasDrawOptions` 空结构体（`:110`） |
| GPU 腿 | `render/vertices_atlas.go:258-305 DrawAtlasEx` | `atlasSpriteCorners(sp, ctm)`（`:177-207`，用户空间 Dst 经 CTM → 设备四角，先翻转后旋转）→ 恒走 `QueueImageDrawTint`/`QueueImageDraw`（1b 后全 GPU 图集批都带 tint 下发）；**两腿都是仿射双三角** |
| CPU 腿 | `render/vertices_atlas.go:320-469 drawAtlasExCPU` | `newTriSampler` 仿射 UV 插值（`:357-358`），无透视采样分支；损伤跟踪用 `quadBounds(dev)`（`:351-352`） |
| 透视采样器 | `render/m4_trisampler.go:32-37 newTriSamplerPerspective` | **已就绪**：吃任意三角 + 任意 UV 三元组 + 每角 w，1/w 有理插值——图集子矩形 UV 可直接用 |
| 透视 quad 参照 | `render/m4_quadimage.go:23-132 DrawImageQuad` | 角点=**用户空间**（`:55-59` CTM 应用）；校验序=weights（`:38-47`，非有限/≤0 → `ErrQuadBadWeights`；bicubic 组合 → `ErrQuadPerspectiveBicubic`）→ tint → `ClassifyQuad`（`:52-54`）→ CTM 变换 → 设备退化判定（`:60-62`） |
| GPU 透视槽 | `render/internal/gpu/image_pipeline.go:1280-1316 buildPerspVerticesInto` | **每角 UV 本来就逐角携带**（u0,v0/u1,v0/u1,v1/u0,v1 从 `cmd.U0..V1` 读）——图集子矩形 UV 进 36B 透视槽**零改动即用**；36B=pos8+uv8+tint16+w4 |
| 合批 | `render/internal/gpu/image_pipeline.go:1173-1181 canMergeImageDraw`（批 1a） | 透视↔透视按图号/采样键/透明度/视口/图源尺寸/混合模式合——图集透视精灵与 quad 透视贴图**天然同槽合批** |
| 降级 | `render/internal/gpu/render_session.go:4544-4563 degradePerspectiveCmds` | 管线不可用时就地清 `Perspective` 位，仿射打包按 cmd 角点画——图集透视精灵降级后几何保持（设备角已在 cmd 里）、UV 失去透视校正，与 quad 路语义一致，**零新代码** |
| 分组 | `sliceImageResources` 的 `cmdStart/cmdCount`（P3.5 第④洞修复） | 图集透视命令与 quad 透视命令同走图像命令流，按命令区间分组**自动覆盖** |

### 1.2 深度精灵：Depth 只排序不投影，透视承载随 AtlasSprite 白拿

- `render/depth_r5.go:43-47 DepthSprite = Sprite AtlasSprite + Depth + Name`；`DrawDepthSprites:132-169` 校验→（DepthTest 时 `SortDepthSprites` 稳定降序）→ 拍平调 `DrawAtlasEx`。**AtlasSprite 加了四角+权重，DepthSprite 自动承载**；`DrawDepthSprites` 只需把新校验交给 DrawAtlasEx 的统一入口（构造口 `NewDepthSprite:75-83` 加早拒绝）。
- 排序键 `Depth` 与投影 depths 相互独立：排序仍按精灵 Depth（远先近后），投影按每角 depths（引擎算好喂 Corners/Weights）——两层语义不打架。

### 1.3 engine 侧：出口齐备但没接，ProjectQuad 零消费者

| 环节 | 位置 | 实测状态 |
|---|---|---|
| `Projector` | `engine/camera/project.go:22-26` | 纯 core 数值（focal/center/camera 私有）；`Project:76-92` 返回 `screen+scale`；**`ProjectQuad:98-108` 返回四角屏幕坐标+四角 scale，全仓非测试调用点=0**（批 1c 第一个真消费者）；`DepthToScale:57-70` 生产消费者=0（评估 §1.4 同款） |
| 齐次权重换算 | 语义对拍 | render 透视腿的 w ≡ `focal+depth`（`render.Matrix4.ProjectorMatrix4` 投影 w=f+d；P3.5 批 3 钉）；Projector 侧 scale=f/(f+d) ⇒ **w=focal+depth 直接可得**（ProjectQuad 已保证 f+d>0） |
| `engine/sprite.AtlasSprite` | `engine/sprite/atlas.go:70-84` | Dst=世界单位盒、Tint/Filter/Rot/Flip/Pivot 齐——**无透视承载**；`ToRender:213-233` 逐字段转 render（P3.7 评估 §1.6 记过：从不填 `Mipmap` 的断点在批 4 修，本批不动）；冻结契约 `doc.go:31-36`「Additive changes only」——**加字段合规** |
| `renderconv` | `engine/renderconv/conv.go:14-40` | 只有 Vec2/Color/Mat2D 六组转换；**无 quad/角点转换**（camera/project.go:20-21 自陈「render types are converted once at the render boundary in engine/renderconv」——转换层承诺了但没这块，本批补） |
| 生产链消费者 | `examples/engine/{sprite,stage_chase,particles2d,particle}` | 全部走 `sprite.AtlasToRender → DrawAtlasEx`（仿射）；1c 后这些窗零改动（零值=老路），**生产接线由批 2（z 序盖序承载）与批 6 透视批绘窗承接**，本批属「已验证待接线」 |

### 1.4 示例层绕路（要搬走的那处）

- `examples/engine/camera/main.go:924,940`：`proj.Project(sh, 0)` 把返回的 **scale 用 `_` 丢弃**，矩形尺寸写死 80×60 / 60×28（远山与地面条带）。该窗两处都在 **depth=0** 投影 ⇒ scale≡1 ⇒ **把 scale 捡回来后画面逐位不变**（80×1×60、28×1），窗金图（`testdata/camera_follow_golden.png`、`voices_golden.png`）预期零变化——与批 1b persp 窗不同，这次「不改画面」是可推导的（scale=1 解析恒等），窗自检 `-auto-only`（golden mask + 探针，`main.go:781-784`）直接验证。
- 该窗是 camera 语义演示窗（非 1c 真窗）；1c 能力验证按 PLAN §9 走**离屏/逻辑钉**（真窗=批 6 的透视批绘窗，D7 已拍）。

### 1.5 字面量安全面

`grep AtlasSprite{` 全仓（render/engine/examples/ui/测试）：**全部键名字段构造**（`render/nine_patch.go:110`、`engine/sprite/atlas.go:155,225`、`examples/*`、`render/depth_r5_test.go` 等）——`AtlasSprite` 加字段无位置字面量风险。

---

## 2 改动面（按 design §2.3 形态）

### 2.1 render 侧（`render/vertices.go` + `render/vertices_atlas.go` + `render/depth_r5.go`）

1. `AtlasSprite` 加三字段（与 `QuadDrawOptions` 同名词同序 TL/TR/BR/BL）：
   - `Perspective bool`（零值=走现有 rect+Rot+Pivot+Flip 仿射拆法，**逐位不变**）；
   - `Corners [4]Point`（**用户空间**，CTM 照常应用——与 `DrawImageQuad` 的 corners 同语义，见 §3 D-2）；
   - `Weights [4]float64`（齐次 w，恒有限 >0）。
   - `true` ⇒ **忽略 Dst/Rot/Pivot/Flip**（避免「半仿射半透视」混合解释），四角直接取 `Corners` 经 CTM；`atlasDrawable` 的跳过规则对透视精灵改按 `SrcW/SrcH>0` 判（Dst 不再参与几何）。
2. 校验（`DrawAtlasEx` 统一入口 + `NewDepthSprite` 早拒绝；`Perspective=false` 不校验新字段=存量行为不变）：
   - 四角非有限 → 复用 `ErrAtlasNonFinite`；
   - 四角退化（零面积）/自交（bowtie）→ 新增哨兵（见 §3 D-1）；
   - 权重非有限或 ≤0 → 新增哨兵；
   - 透视+bicubic 组合 → 拒绝（见 §3 D-1）。
3. GPU 腿：透视精灵在 `DrawAtlasEx` 的 GPU 循环改走 `QueueImageDrawQuadW`（1b 已扩 tint 形参）——传设备四角、`sp.Weights`、子矩形 UV、premul tint（`tr*ta` 同式）、`nearest`（bicubic 已在校验拒绝）；**与 quad 透视贴图同槽、同 1a 合批谓词**。
4. CPU 腿：`drawAtlasExCPU` 加透视分支——`newTriSamplerPerspective` 双三角（UV 三元组与仿射腿同布局），tint/premul/损伤跟踪逻辑共用。
5. 深度精灵：`DepthDrawOptions`/`DrawDepthSprites` 行为不变（排序承载 + 透传）；透视校验经 DrawAtlasEx 统一生效。

### 2.2 engine 侧（`engine/sprite` + `engine/camera` + `engine/renderconv`）

1. `sprite.AtlasSprite` 加 `Perspective bool` + `Corners [4]core.Vec2` + `Weights [4]float64`（角点承载**已投影的屏幕角**——由 camera 出口产出）；`Validate` 对透视精灵加非有限/权重校验（InvalidArg 早拒）；`ToRender` 逐字段透传（`renderconv.Vec2ToRenderPoint`）；`NewAtlasSprite` 不扩参（透视字段由投影出口填充，构造器保持冻结契约）。
2. `camera` 投影出口（`ProjectQuad` 第一个真消费者）：`Projector` 新方法，吃 world rect + **每角 depths**（单深度传 4 份同值）→ `ProjectQuad` 投影 → 经 `renderconv` 转成 `[4]render.Point` + 权重 `w_i=focal+depth_i` → 返回 ok=false 整体拒绝（behind-eye/非有限，ProjectQuad 既有语义）。单深度情形 = 等权 ⇒ 仿射等价的「按深度缩放」形态（`Projector.Project` 的 scale 由此真正被消费）。
3. `renderconv` 视出口落点补转换（若出口在 camera 内部直接转，则 renderconv 无新函数；两案见 §3 D-3）。

### 2.3 示例层（只做验证，不绕路）

- `examples/engine/camera/main.go:924,940`：捡回 `Project` 的 scale——尺寸改 `80*scale × 60*scale` / `60*scale × 28*scale`（depth=0 ⇒ 画面逐位不变，金图零变化用窗自检验证）；绕路（丢弃返回值+写死尺寸）就此搬走。

---

## 3 待拍板项

- **D-1 哨兵命名面（推荐按 quad 口全对齐）**：新增 `ErrAtlasQuadDegenerate`、`ErrAtlasQuadBowTie`、`ErrAtlasQuadBadWeights` 三个（quad 口同族命名 `ErrQuadDegenerate/ErrQuadBowTie/ErrQuadBadWeights`，`m4_quadclass.go:60-77`）；四角非有限复用 `ErrAtlasNonFinite`；**透视+bicubic 组合复用 `ErrQuadPerspectiveBicubic`**（同一语义不造第二份枚举；备选=新造 `ErrAtlasQuadPerspectiveBicubic` 保持 ErrAtlas* 前缀家族）。
- **D-2 Corners 空间语义（推荐与 quad 口一致=用户空间）**：`AtlasSprite.Corners` 是用户空间角点、CTM 照常应用——与 `DrawImageQuad.corners` 同语义同责任（2.5D 窗里 CTM=identity 时引擎屏幕角直通；若场景有画布变换，由调用方折算，与 Dst 字段同责任）。备选=设备空间直通（跳过 CTM），与 Dst 语义割裂、双层变换责任不清，不推荐。
- **D-3 camera 出口落点（推荐 camera 方法内直转）**：`Projector` 新方法吃 `core.Rect + [4]float64 depths` 返回 `([4]render.Point, [4]float64, ok)`，内部经 `renderconv.Vec2ToRenderPoint` 转换——camera 从 core-only 变为 import renderconv（与 `engine/sprite` import render 同向，模块库→render 是正分层）。备选=camera 返回 core 值、renderconv 出独立转换函数（多一层搬运，调用方两步走）。
- **D-4 老 `DrawAtlas` 对透视精灵的行为（推荐文档写明、行为不改）**：老路只读老字段（`vertices.go:60` 注释惯例），透视精灵经老口会按 Dst 仿射画——文档写明「透视精灵必须走 DrawAtlasEx」；不改行为（老路零变化红线），不加跳过（静默丢绘制同样是错）。备选=老口对透视精灵整批拒绝（无错误返回口，只能静默跳过=另一种画错）。

---

## 4 能力单测（先红后绿）与载体（照 §4.2 模板，开工登记）

| 单测 | 判据 | 载体 |
|---|---|---|
| `TestP37AtlasPerspCPUGPUParity` | 透视图集精灵 GPU vs CPU（`GOGPU_RENDER_MODE=cpu`）稳定区逐通道 ≤1/255（沿用 P3.5 批 3 容差：GPU f32 clip-w vs CPU f64） | 外部像素级，`ExportImageBuf`+`GetRGBA` |
| `TestP37AtlasPerspTrapezoidUV` | 同图同几何，仿射画与透视画在错切区取样点必不同色，且透视取色=按 1/w 有理插值预测值——**防「weights 被忽略退仿射」的假绿** | 同上 |
| `TestP37AtlasPerspZeroIsIdentity` | `Perspective=false` 时 Corners/Weights 全被忽略（塞垃圾值与不塞逐位相同）；零值=历史路 | 同上 |
| `TestP37AtlasPerspSentinels` | 非有限角→`ErrAtlasNonFinite`；零面积→Degenerate；bowtie→BowTie；权重 ≤0/NaN/Inf→BadWeights；透视+bicubic→组合哨兵；错误后不画（画面空白） | 同上 |
| `TestP37AtlasPerspMerges`（internal） | N 张同图透视图集精灵 → `LastImageBatchStats` drawCalls=1；图集透视精灵与 quad 透视贴图同图混排仍合（1a 谓词自动覆盖） | `render/internal/gpu`，真设备 |
| `TestP37DepthSpritesCarryPersp` | DepthSprite 带 Perspective 精灵：排序序保持 + 像素钉（透视校正画面） | 外部像素级 |
| engine 侧钉 | camera 出口：出口角点 == `ProjectQuad` 逐角值、权重 == focal+depth、单深度形态 == scale 缩放矩形（与 `DepthToScale` 对拍）；`sprite.ToRender` 透视字段透传逐值 | `engine/camera`/`engine/sprite` 单测 |
| 真窗/离屏载体 | 本批离屏钉为主（PLAN §9：批 6 前各批用离屏/逻辑钉）；`examples/engine/sprite -auto-only` 双后端复验=**存量金图零变化**（该窗恰是 DrawAtlasEx+DrawDepthSprites 的直接消费者窗）；`examples/engine/camera -auto-only` 复验=scale 捡回后画面逐位不变（scale≡1@depth0 解析恒等） | 离屏 + 两窗自检 |

负向对照（拟，摘修复必红→还原）：N1 图集 GPU 透视腿退回仿射队列口（忽略权重）→ 梯形 UV 钉红；N2 CPU 腿摘透视采样器 → GPU/CPU parity 红；N3 GPU 下发权重改填等权 → 梯形 UV 钉红；N4 摘权重校验 → 哨兵钉红。

## 5 口径与登记（预告）

- **生产消费者**：本批后 `ProjectQuad`/`DepthToScale` 拿到第一个真消费者（camera 出口），但 2.5D 窗与 stage_chase 仍走仿射——批量精灵透视批绘的规模化接线在批 2（盖序承载）与批 6（透视批绘窗）；本批状态=**已验证待接线**，不当接线标绿。
- 登记不做：批 4 的 `ToRender` 补 `Mipmap`（P3.7 评估 §1.6 断点，不属本批）；engine 出口的规模化消费者改造（stage_chase 接投影属批 2/批 6）；老 `DrawAtlas` 行为变更（D-4）。
