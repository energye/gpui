# P3.7 批 2 · z 序与世界高度归位 — 实施方案（2026-11-04，拍板后定稿）

依据：`batch2-assessment.md`（现状四套 + §6 Godot 逐点对拍 + §7 改判）+ 本轮三条拍板：
- **D-A′ = 直接上 `ZIndex`（完全对齐 Godot）**：外层 z 桶 + 层内 origin 键，不做"先用 Layer 凑合"的过渡档。
- **D-H = 申请解冻**：`engine/camera/doc.go:94-95` 冻结的 7 个符号一次改到位，不留垫片（开发阶段"接口全量切换制"），doc.go 记解冻原因与日期。
- **D-G = 本批就做分组语义**：引入"y-sort 宿主/容器"模型（只对开启者子孙生效、宿主自身占序号 0、子树整体跟随、不跨容器互排）。
- 已定未变：D-C 删 `render` 的 `DepthTest`（换臂对比争取不动 golden）、D-D Height 不进排序键（需同步修订 `design.md` §3 那句并记原因）、D-E `YSortZ` 保留符号改注释。

## 1 真源（camera 域）

Godot 真源结构（`renderer_canvas_cull.h:117-127`、`renderer_canvas_cull.cpp:79-152,431-491`）落成四条：

1. **外层 z 桶**：`ZIndex int`，范围 **±4096**（Godot 同值，`rendering_server_enums.h:40-41`）；`ZAsRelative=true` 时逐级累加父链并**每级 CLAMP**（Godot `renderer_canvas_cull.cpp:431-436` 与 `:145-150`），false 时绝对化。
   - Godot 的 `ItemYSort` 只比 Y，z 是在 `_cull_canvas_item` 里按绝对 z 分桶（`r_z_list[zidx]`）逐桶提交；净效果 = 「先 z 后 Y」。本仓一次性排好，等价且少一层状态（`renderer_canvas_cull.cpp:455-459`）。
2. **层内键 = origin.Y，判等按 Godot 公式**：不取平坦 `1e-6`，照抄 `Math::is_equal_approx`（`core/math/math_funcs.h:552-562`）：先走精确相等分支（±Inf 用），再 `|b-a| < max(CMP_EPSILON, CMP_EPSILON*|a|)`，`CMP_EPSILON = 1e-5`（`core/math/math_defs.h:50`）。**改判理由**：设计稿原本写死 1e-6 是估的，拿到真源公式后按真源；量级在小数值处比 1e-6 宽 10 倍、在大数值处按 |a| 放大，才是"脚位抖动不闪、真实位置差不被吞"的正确形态。
3. **平票靠容器序号**：DFS 压平时按树序赋 `Order`（宿主自身 = 0，其子按兄弟索引递增），比较器最后一级比 `Order` ⇒ 结果是**全序**、与排序算法稳定性无关（Godot 底层是不稳定的 introsort，靠全序 comparator 保确定）。`sort.SliceStable` 保留但**不再作为语义依据**，注释与钉都改口径。
4. **纯 Y 为默认键**：Godot 本体只比 origin.y。现有 `YSortKey = Y + 0.001*(X+Z)` 是 Godot demo 的 `y_sort_slight_xz` **变体**（真源：`godot-demo-projects/misc/2.5d/addons/node25d/node_25d.gd:137-140`，`y_sort_25d.gd:41` 用它排；纯 Y 版在同文件 `:134-135`）⇒ 降为具名函数 `YSortKeySlightXZ`，默认排序用 `YSortKeyY`。
   - **变体必须可选**：光留一个没人调的键函数 = 装饰，不算保留能力 ⇒ 三条入口各加一个带键的对口 `YSortWithKey` / `YSortTreeWithKey` / `YSortLessWithKey`，`YSort`/`YSortTree`/`YSortLess` 变成标准键的薄封装（nil 键 = 标准键）。
5. **D-G 分组语义改判（实测真源后）**：设计稿原写"子树整体跟随祖先槽位"——**Godot 没这条**。`ItemYSort` 比的是 `ysort_xform.columns[2].y`（累加变换的原点 Y，`renderer_canvas_cull.h:119-125`），孙节点 Y 远就排到后面，跟别的分支穿插；父子关系只通过"祖先平移量已含在 Y 里"间接起作用，DFS 序号 `ysort_index` 只破平票。故本仓口径定为：**容器 = 一个 y-sort 宿主及其子孙压平；宿主占序号 0；不跨容器互排；`Pos` 传"已在容器系解析好的位置"**（对应 Godot 的 ysort_xform，调用方持父相对变换时先累加）。`YSortTree` 注释与钉按此写。


### 1.1 解冻后的 API（camera 域）

```go
// 容器内一条可排序记录
type YSortNode struct {
    Parent      int        // -1 = 容器根（Godot：只对开启者子孙生效）
    Pos         core.Vec3  // 已解析到容器系的位置（Godot ysort_xform.origin 同位）
    ZIndex      int        // 外层 z 桶，±4096
    ZAsRelative bool       // true = 累加父链并 CLAMP
}

type YSortKeyFn func(core.Vec3) float64   // 层内键；nil = 标准键

// 压平 + 排序：返回绘制顺序的下标切片（远→近 = 先画 → 后画）
func YSortTree(nodes []YSortNode) ([]int, error)
func YSortTreeWithKey(nodes []YSortNode, key YSortKeyFn) ([]int, error)

// 一维数组入口 = 退化容器（全体兄弟，z 按给定值用）
func YSort(items []YSortItem) ([]YSortItem, error)
func YSortWithKey(items []YSortItem, key YSortKeyFn) ([]YSortItem, error)

func YSortKeyY(p core.Vec3) float64          // 标准键（纯 Y）
func YSortKeySlightXZ(p core.Vec3) float64   // demo 变体，具名保留
func YSortLess(a, b YSortItem) bool          // z 桶 → 容差键 → Order
func YSortLessWithKey(a, b YSortItem, key YSortKeyFn) bool
func YSortEffectiveZ(nodes []YSortNode, i int) (int, error) // 单节点父链解算
const MaxYSort25D = 4000                     // 不动（同 Godot 4000 节点上限）
const YSortZRange = 4096                     // z CLAMP 半幅（新）
const YSortZBase/YSortZStep/YSortZ           // 不动；仅注释改正（D-E）
```
`YSortItem` 加 `ZIndex`/`ZAsRelative` 两字段（additive），旧调用点零改动仍成立。
**与设计稿的偏差（如实记）**：`YSort` 不转调 `YSortTree` —— 扁平入口的 `Order` 是调用方给的
（可以不是下标），树入口的 `Order` 是 DFS 序号，两者语义不同；共用的是同一份比较器
`YSortLessWithKey`，排序骨架各 5 行，没有第二份判据。


## 2 副本改委托（必须逐位等价）

- `engine/sprite/depth.go`：`DeepItem{Depth,Layer,FeetY}` → `YSortNode`：`Pos.Y = FeetY`（层内键）、`ZIndex` 由 `Layer` 映射到 z 桶（`LayerWorld/FX/UI` → 固定段，公开映射函数 `ZIndexForLayer`，避免魔法数）、`Depth` 作为**次级键**保留（Godot 无此物 ⇒ 明确标注为"本仓扩展语义，进键方式=Depth 降序映射为 Y 升序"，与 `stage_chase:905` 现有 `4000-(y+h)` 换算同源）。
- `engine/sprite/ysort.go`：`Item{Layer,FeetY}` → 同一真源（`Sort` 内部转调），语义等价。
- **对拍硬约束**：拿 `examples/engine/stage_chase` 的 1000 prop 现序 + #2/#3 现有单测全集当输入，新旧输出**逐元素相等**；不等即说明"动的是行为不是实现"，停下来重拍，不硬调。

## 3 render 侧去越界（D3b + AGENTS 分层归位）

- 删除 `SortDepthSprites:106` / `IsDepthSorted:123` / `SetDepth:93` / `DepthDrawOptions.DepthTest:51` —— 排序能力与公式只在 camera 一份。
- `DrawDepthSprites:139` **保留**（它是"按给定序画一批 + 子矩形/权重"的图形能力），语义改为**只按输入序绘制**；`DepthDrawResult.Sorted` 字段随之失去意义 ⇒ 一并删（全量切换，不留垫片）。
- 同批改调用点：`examples/engine/sprite/main.go`（193/196/199/411-413/483/484 八处）改法=**换臂不对比开关**，改成「engine 侧排好序的输入」vs「故意乱序的输入」两臂 ⇒ 演示价值保留，若两臂像素与原 true/false 两臂逐位相同则 golden 不动；不同则按规矩停下来问是否重出。
- 录制面全量切换：`Canvas` 接口 `DrawDepthSprites` 签名（去 opts 或改无排序语义）、`recording` 的 `DrawDepthSpritesCommand`/`Backend`/`CanvasBackend`/`bounds.go:152`/stub/**spy（examples/ui_render_base_recording/scene.go:555）** 同批改，不留过渡。
- 测试数据：`render/depth_r5_test.go` 读的 JSON 含 `depth_test` 字段 ⇒ 数据文件与解析同批改（AGENTS：数据改动落 testdata，不在代码里造）。

## 4 世界高度与分组边界

- `engine/world/entity.go:29-32` `Transform` 加 `Height float64`：additive、非有限拒绝（复用 core 校验风格）、**不进** `LocalMatrix`（仿射链逐位不变）；只喂投影深度（批 1c 的 `camera.Projector.ProjectSpriteQuad(worldRect, depths[4])` 是第一个真消费者）。
- **同步修订 `design.md` §3**：删/改"高度参与遮挡排序键"为"高度只参与投影深度；遮挡排序键仍取脚位 Y"（依据 Godot 无 2D 高度、`Sprite2D.offset` 不进键），并记修订日期与原因。
- 分组模型的适用边界（本批）：容器 = `world` 的父链（`Spawn/SetParent/Parent/Children` 已有，`entity.go:108,202,235,245` 带环检测）；**不跨容器互排**写进钉与文档（Godot 同口径：不跨 CanvasLayer）。宿主局部系在 2.5D 下与"世界系"重合（`25d_basis` 传的就是世界位置），差异登记为后续，不假装已实现。

## 5 盖序不变式（永久钉，判据按标准写）

`probes/p1-draw-order.txt` 八组混排转常驻测试，判据三条：
1. 屏幕盖序 = 提交序（画家算法前提），容差 0；
2. **tier 切换只 flush 不重排**（Godot `renderer_canvas_render_rd.cpp:820,852,876,906`）——若某 tier 切换实测会重排，修 render 不放松判据；
3. 适用边界明写"单容器内"（分组语义）。
双后端各跑；负向：改 tier 或 `ensureDrawOrder` 让提交序变 ⇒ 必红。

## 6 实施步骤（每步独立红→绿，全套门禁按批口径）

1. **步 2a** camera 真源升级（`YSortNode/YSortTree/ZIndex/epsilon/Order`）+ 钉（含 z CLAMP、环检测、4000 上限、非有限 fail-closed）。
2. **步 2b** `sprite/depth.go` 与 `sprite/ysort.go` 改委托 + **逐位对拍钉**（1000 prop 现序 + 单测全集）。
3. **步 2c** `render/depth_r5.go` 去越界 + 录制面/示例窗/spy/testdata 全量切换；sprite 窗换臂后跑双后端 golden 判"动不动"。
4. **步 2d** `world` 加 `Height` + 投影深度接点 + `design.md` §3 修订。
5. **步 2e** 盖序永久钉 + 负向。
每步都要：三证据（逻辑/像素/Golden）、存量三窗双后端零变化、逐文件 sweep 对基线只减不增、apidoc rc=0、台账 §2.1/§7 登记。

## 7 待确认（实施前最后一眼）

- ~~`ySortEpsilon = 1e-6` 这个量级可否~~ → **已改判**：照抄 Godot `is_equal_approx` 公式（`max(1e-5, 1e-5*|a|)`），见 §1 第 2 条。
- `ZIndexForLayer` 的分段取值（建议 World=-2000 / FX=0 / UI=+2000，留出 ±4096 内的自由余量给玩法自设）——步 2b 再定；
- sprite 窗换臂后若与原画面不等，是否批准重出 golden（按规矩需你点头）——步 2c 再说。

## 8 步 2a 交付记录（2026-11-04）

改动：`engine/camera/ysort25d.go`（比较器两级化 + Godot 判等公式 + `YSortKeyFn` 三对口 +
`YSortNode/YSortTree/YSortEffectiveZ` 容器模型 + 解冻说明）、`engine/camera/doc.go`
（YSort25D 段记 2026-11-04 解冻原因）、`engine/camera/basis25d_test.go`（用例表加纯 Y 键、
冻结核对 demo 记录序、变体键与标准键同序）、新钉 `engine/camera/p37_ysort25d_test.go`（7 枚）。

钉（全绿）：
| 钉 | 判据 |
| --- | --- |
| `TestP37YSortZBucketBeatsY` | z 桶压过 Y（外层键） |
| `TestP37YSortEqualApproxIsGodotFormula` | 小值下限 1e-5、大值按 \|a\| 放大、±Inf 走精确分支 |
| `TestP37YSortTieUsesOrderNotGeometry` | 容差内平票由 Order 定向；同 Y 不同 X 不比几何；同 Order 完全平票不比出先后 |
| `TestP37YSortKeysAreNamedAndSelectable` | 标准键=纯 Y；变体键可选（扁平 + 树两入口），nil 键 = 标准键 |
| `TestP37YSortTreeZChainClamps` | 相对 z 逐级累加 + 每级 ±4096 饱和；绝对化不吃父链（含穿插第三根做反证） |
| `TestP37YSortTreeSortsOwnYNotSubtree` | 压平是 DFS、比较是各自 Y（子树不聚簇）；平票才轮到 DFS 序号 |
| `TestP37YSortTreeRejectsBadInput` | 环 / 自父 / 越界父 / 越界下标 / NaN / -Inf / 4000 上限（树 + 变体 + 扁平三口） |
| `TestP37YSortIsDeterministicTotalOrder` | 300 项无逆序、两两方向唯一、乱序输入同结果（不靠稳定性） |

负向对照（拆真源判据必红，12 项）：N1 比较器不看 z 桶、N2b 判等退回精确 `!=`、N3 默认键换回 x+z、
N4 树入口不排序、N5 环检测拆掉、N6b/N6c 上下界 CLAMP 各拆、N7 4000 上限拆掉、N8 非有限不校验、
N9 查询口把绝对 z 当相对、N10 树里把绝对 z 当相对、N11 破平票不看 Order。
**如实记两条弯路**：首轮 N6 变异写成 `false \|\| z > YSortZRange`（逻辑恒等 = 没变异）故"仍绿"，
第二轮改 `false &&` 才真红；首轮 N2 在"容差内两脚位"那枚钉上方向与 Order 同向，测不出差异，
补了"容差内但 Order 反向"的手臂后 N2b 才红（钉本身被加强了，不是放水）。

`25d_basis` 行为不变的第一手证据（数据驱动，非手搓）：`basis25d_cases.json` 的 `ysort.order`
在新标准键下逐位复现，且与旧 x+z 变体键结果同序 ⇒ golden 不动的前提成立；真窗 golden 双后端另跑。

## 9 步 2b 交付记录（2026-11-04）

改委托（公开签名一个没动）：
- `engine/sprite/ysort.go`：删本地 `less`；带→z 桶 = `ZIndexForLayer(l) = camera.ClampZIndex(camera.YSortZ(int(l)))`
  （带占偶数槽、奇数槽留影子，与 camera 的槽约定同源，不再是 §7 原先设想的 -2000/0/+2000 魔法分段）；
  `Sort` 交给 `camera.YSortLess` 排（Order=输入序号 ⇒ 不靠排序稳定性）；`IsSorted` 同判据逐对核对。
- `engine/sprite/depth.go`：删本地 `deepLess`；`DepthSort` = 外层 Depth 降序（本仓扩展轴，Godot 无 2D 距离键）
  ＋带内全交给 `camera.YSortLess`；`IsDepthSorted` 同式。
- `engine/camera/ysort25d.go`：+ `ClampZIndex`——把 ±4096 饱和规则交给"要把自己的优先轴映射到 z 轴"的模块，
  一份规则两处用，不在 sprite 里再抄一遍。
- **不给 sprite 加 4000 上限**：那条上限是 demo 的 z 槽算术护栏（`YSortZ` 步长 2 装进 ±4096），
  排序本身 Godot 无上限；把上限随委托塞进 sprite 公开口 = 动能力面，故本次只交"判据"不交"上限"。

对拍钉 `engine/sprite/p37_depth_parity_test.go`（语料 `testdata/p37_zsort_parity.json`，
生成器 `scripts/gen_p37_zsort_parity.py`，语料里每条 `want_order` 都由**改委托前**的旧判据离线复算）：
11 案逐元素相等——含 `props_1000`（stage_chase 同形：depth=4000-(y+h)、LayerWorld）与
`props_1000_mixed_bands`（1000 项三带混合，含同深度整数网格平票）；并断言
`IsSorted/IsDepthSorted` 对结果自认已排、对倒序判未排、`Sort` 不改输入切片。

**唯一的刻意行为差（如实登记）**：脚位差落进 camera 判等容差 `max(1e-5, 1e-5·|y|)` 时，
新判据按平票处理、由输入序号定序，旧副本按最后浮点位比 Y。D-A′ 拍板里"epsilon 判等"本就是要引进的标准条目；
实测语料里那条 5e-4 差（|y|≈100 ⇒ 容差 1e-3）确实被吞进平票 ⇒ 已从对拍语料移除并改写成
`gap_outside_tolerance`（0.5 单位差，仍逐位一致），平票行为由 `TestP37SpriteSortUsesCameraTolerance` 单独钉住。
换算成像素：1 世界单位≈32px，|y|=600 处容差 6e-3 单位 ≈ 0.19px——肉眼不可见，换来插值抖动不翻序。

真委托钉（防"假委托"）：`TestP37SpriteSortUsesCameraTolerance`（公式抄回本地副本即红）、
`TestP37SpriteSortBandSaturatesLikeCamera`（越界带饱和进同桶后由 Y 定序，本地旧判据不可能出现此结果）。

负向对照（9 项全红）：M1/M2b 两口把判据抄回本地副本、M3 带→桶塌成单桶、M4b Depth 外层轴失效、
M5 Depth 方向反、M6 `IsDepthSorted` 方向反、M7b/M8b 带内序检查恒真、M9c `IsDepthSorted(nil)` 契约丢
（该条由存量 `depth_test.go:139-140` 抓住，新钉随后补了同一条断言）。
**如实记三条弯路**：M2/M4 首轮变异写成非法 Go（重复声明/未使用变量）＝无效变异；
M9b 首轮写 `false \|\| items == nil` 是恒等式＝没变异；M7 首轮"仍绿"是真缺口——
当时只断"结果自认已排"、没断"倒序判未排"，补断言后 M7b 才红。

### 9.1 补记：委托引入的每帧排序变慢，实测→修正→反超（2026-11-04 门禁阶段）

批 2 全套门禁跑真窗时才暴露：**首版委托多分配了一张键表**（`[]camera.YSortItem`，1000 项 ≈40–48KB/帧），
`go test -v` 的 100 项档读数 **本批 7.0–8.6µs vs 基线 3.3–3.9µs（≈2 倍慢）**。
这条是我自己引入的性能债，按 AGENTS 性能红线（红了先优化再谈关洞）处理：

- 改法：**不建键表**，改成 `slices.SortFunc` 排 `[]int` 下标，比较时在栈上现构 `camera.YSortItem`
  （`Item.yKey` / `DeepItem.yKey`）；`IsSorted`/`IsDepthSorted` 顺带变成**零分配**逐对核对。
  判据仍然只有 camera 一份，没有回抄。
- 实测（同机交替 3 轮）：100 项档 **3.0–3.5µs vs 基线 3.3–3.9µs**；
  1000 项混合带档（`testdata/p37_zsort_parity.json` 的 `props_1000_mixed_bands`，2000 次/轮）
  **本树 189–195µs vs 基线 786–823µs ⇒ 约 4.2 倍快**（旧副本用 `sort.SliceStable` 搬 48B 结构体，
  现在搬 8B 下标且走泛型比较，无反射交换）。
- 逐位对拍与真委托钉在改法后**仍全绿**（序不变：`Order` 唯一 ⇒ 稳定与不稳定排序结果同）。
- 负向重跑（锚点随代码形状更新，N 系列替代 §9 的 M 系列）：判据抄回本地副本两枚、带→桶塌成单桶、
  Depth 外层轴失效、两口 `IsSorted`/`IsDepthSorted` 恒真 —— 六条全红；还原后 `engine/sprite` 全包绿。
- `stage_chase` 的 §5 帧门（p95≤16ms）在提速前后与本树/基线之间都红，读数落在同一条带
  （基线 native 23.13 / 本批修前 native 17.51、go 23.11 / 本批修后 native 22.95、go 23.07），
  平均帧率一律 ~59.76fps、像素侧四读全为 golden changed=0、parity 0.0000%
  ⇒ 定性：这台机器的 §5 尾帧漂移族（在册先例 `s6_9_heavy_budget`、`mem_window_x11_linux`），
  不是批 2 引入；证据 `gates/stage-chase-frame-gate-ab.txt`。排序变慢这条确属本批，已修并反超基线。


## 10 步 2c 交付记录（2026-11-04）

render 去越界（D3b，全量切换不留垫片）：
- 删 `render/depth_r5.go` 的 `SortDepthSprites`/`IsDepthSorted`/`SetDepth`/`DepthDrawOptions`
  ＋ `DepthDrawResult.Sorted` 字段；`DrawDepthSprites(img, sprites)` 去掉 `opts` 形参，
  语义=**只按提交序画**（文件头口径同改）。`DepthSprite`/`NewDepthSprite`/`ErrDepthNonFinite`/
  `DepthCPUFallbackReason` 随能力保留（数据格式不裁）。
- 同批全量切换的调用面：`render/canvas.go` Canvas 接口、`recording`（`Backend` 契约、
  `DrawDepthSpritesCommand` 去 `Opts` 字段、`Recorder`、`CanvasBackend` 回放、`recorder.go` 分发）、
  `examples/ui_render_base_recording/scene.go` spy、`backend_stub_test.go`、`recorder_test.go`、
  `bounds_test.go`、`backends/raster/backend_coverage_test.go`、`render/p37_atlas_persp_test.go`。
- 测试数据同批改（AGENTS：数据改动落 testdata）：`render/testdata/depth_r5_cases.json`
  去 `depth_test` 字段；需要远先近后的用例把 `sprites` 直接写成提交序（`three_layers` 重排）；
  删 `far_last_input`（该用例的语义＝"render 会替你把倒序排回来"，能力已迁走，留着就是假语义）；
  `depth_off` 改名 `submission_order` 并改注（提交序=盖序这一臂继续钉住）；`depth_note` 重写。
- `render/depth_r5_test.go` 整篇重排：删 `TestDepthR5SortSemantics`（排序语义已归 engine，
  覆盖在 2b 的对拍钉里），其余用例改无 opts 调用，并**新增**一条判据：同一批精灵换提交序，
  两张画面必须不同（`imagediff` 逐位），这条就是"render 不许再自己排序"的常驻哨。
- 示例窗 `examples/engine/sprite` 八处调用：`depthTrueSprites/depthFalseSprites` 换成
  `depthFarFirstSprites/depthNearFirstSprites`，远先那臂改成**先交给 engine 排**（`sortDepthByEngine`
  →`sprite.DepthSort`→camera 判据）再提交（D-C(2) 换臂不对比开关）。
- 目录同步：`docs/RENDER_API_CATALOG.md` 主包口径 154/197→**153/194**（go/ast 实测计数脚本口径同 §0），
  §3.9 两行改写入删口记录；`go run ./scripts/apidoc` rc=0。

实测（存量零变化）：`go build ./...` rc=0；`go vet ./render ./render/recording/... ./engine/...` rc=0；
`TestDepthR5*` 绿、`TestP37DepthSpritesCarryPersp` 绿、`render/recording/...` 全包绿；
**`examples/engine/sprite` 真窗 `-auto-only`：golden 0.0000%、parity 0.0000%、depth_sorted=1、ok=true**
⇒ 换臂没有动画面，**不需要重出 golden**（D-C 那一拍的风险没有兑现）。
负向对照 1 项：把排序偷偷塞回 `DrawDepthSprites` 内部 ⇒ `TestDepthR5FarFirstFromCases`
报"换了提交序画面却一模一样"红，另有 `TestDepthR5LongRunStable`/`TestDepthR5OffscreenGolden` 连带红。

## 11 步 2d 交付记录（2026-11-04）

- `engine/world/entity.go`：`Transform` 加 `Height float64`（additive，仍在 `doc.go` 冻结口径内）。
  口径三条按 D-D 改判写死：**只喂投影**、不进 `LocalMatrix`/`WorldMatrix` 仿射链、
  不进遮挡排序键（键仍 `camera.YSortKeyY` 取脚位）；父链把高度按**普通相加**折叠
  （骑在抬起来的平台上 = 平台高度 ＋ 自己高度；2D 的缩放/旋转不碰这根轴）。
  `checkTransform` 拒收非有限 Height（InvalidArg，被拒后本地量逐位不变）。
- 文件面（设计稿没写、但不写就会**静默压扁**的洞）：`NewPrefab`/`SetLocal` 收 Height ⇒
  `transformFromJSON` 也必须收，否则 `Instantiate` 把高度吞成 0。做法：`jsonTransform` 加可选
  `height`（`omitempty`，零高度旧字节逐位不变），`PrefabVersion`/`SceneFileVersion` 按各自文件头规则
  **1.0 → 1.1**（minor=加字段）；1.0 旧档经 `CompatibleWith(Minor<=cur)` 照旧可装，缺键读 0。
  实测抓到一条真窗回归：`examples/engine/world` 装完 `world_open.json` 要 `Encode` 做字节对拍，
  版本起跳后报 `scene.SceneFile.Encode "1.0": version-mismatch` ⇒ 夹具同改 1.1，
  窗口恢复 `probes ok=true`、golden changed=0、200 轮 drift 逐位一致；
  坏档族（`scene_bad_*.json`）故意留 1.0，正是旧版兼容的证据。
- 新钉 `engine/world/p37_height_test.go`（5 枚全绿）：非有限拒收、仿射链逐位不变（Local/World 矩阵）、
  父链高度相加且位置零改动、prefab＋scene 往返带高度＋1.0 缺键读 0、Height 不进排序键。
- 负向（全红）：K1b 校验口删掉 Height 项、K2 Height 混进 `LocalMatrix`、K3 父链不叠加、
  K4 文件写丢 height、K5 文件读丢 height。
  **K6 记为不可达**：`{"height":1e999}` 这类值在 `encoding/json` 解 float64 阶段就先报 bad-data，
  `transformFromJSON` 里的 finite 校验是第二道保险，拆掉测不出差别
  （第一轮还把变异写成 `true && …` 恒等式，与步 2a 的 N6 同一坑，改成"删项"式才有效）。
- `design.md` §3"世界高度"那句按改判重写并记日期与依据（Godot `sprite_2d.cpp:116-136`、
  `renderer_canvas_cull.h:117-127`：offset 不进 y-sort 键）；域内总账 `docs/2.5D/G04.md` 10.1 行补记；
  `engine/world/doc.go` 冻结段记 Height 与版本起跳。

## 12 步 2e 交付记录（2026-11-04）

探针 `probes/p1-draw-order.txt`（八组混排，跑完即删）转成常驻钉
`render/p37_draw_order_test.go`：

- `TestP37DrawOrderIsSubmissionOrder`：A/B 跨 tier 双向、C/D 同 tier 双向、E/F 三段往返、
  G 两次跨 tier（4→1→4）、H 纯路径对照，判据三条——交点 (6,6) 颜色**容差 0** 等于最后提交者、
  角落必须仍为白（不污染）、`GPUOps` 数按组钉死（2/2/2/2/3/3/3/2，批次数量变=提交被重排或合并）；
  CPU 腿与 GPU 腿各跑一遍（GPU 不可用时具名 skip，不假绿）。
- `TestP37DrawOrderCPUGPUMapsMatch`：同一段提交在两条光栅器上交点必须同色。
- 边界照 §5 明写：**单容器（一张 Context 一帧）内**提交序=盖序；跨窗跨层不在判据内。
- 实测：native 后端绿、`GPUI_BACKEND=go` 后端绿（两条都真跑到 GPUOps>0，非 skip）。
- 负向（拆 tier/封段机制必红）：Q1 `ensureDrawOrder` 永不封段 ⇒ B/E/F 三组交点翻成蓝、
  CPU≠GPU 腿也红；Q2 封段判据方向反 ⇒ 同样三组红；Q4 `drawTierImage` 降到 0（护栏位失效）⇒ 同样三组红。
  **Q3 记为无效变异**：锚 `rc.recordScissorSegment(rc.clipRect)` 在文件里有 5 处，`count=1` 命中了别处，
  没改到 `ensureDrawOrder`——与 Q1 同一缝口，已由 Q1 覆盖，不算缺口。





