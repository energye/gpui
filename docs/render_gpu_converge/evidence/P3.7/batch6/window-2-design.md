批 6 ② · 高度与 z 序窗 设计（2026-11-06，开工前停报告版；钉版要求见 `batch6-window-plan.md` §1 ② 行 + §3 六条硬要求）
====================================================================================

一、窗名与归位
--------------
`examples/engine/p37_height_zsort`（与 ① `p37_persp_batch` 命名同族）；满窗即内容、
指标行浮在内容左上（金图盖 `y<32` 指标带，与 ① 同口径）；金图
`testdata/goldens/p37_height_zsort_final_{native,go}.png`，`wrkit.GoldenGate`
fail-closed（缺基线红、`GPUI_ACCEPT_GOLDEN=1` 播种不算过）。

二、臂清单（钉版五臂，全部新建载体——老 `25d_basis` 窗只用 Order 无 ZIndex 无树，
`world.Cull` 比较器不含容差不含 Height，一律不代答）

| 臂 | 画面 | 断什么 | 探针 JSON 键 |
|---|---|---|---|
| A z 桶分组与 ±4096 CLAMP | 一排交叠精灵，ZIndex 故意越界（-5000…+5000），经 `camera.YSortWithKey`（ZIndex 桶优先）排序后按序绘制——桶大者盖小者，与 y 无关；越界值饱和在 ±4096 | 越界饱和 + 桶序盖序 | `zclamp_map`（输入 ZIndex → `ClampZIndex` 实测值逐枚）、`cover_by_zbucket`（交叠点像素=大桶精灵色） |
| B 层内 y 键 + is_equal_approx 平票 + DFS 破平票 | 两条子带：B1 两精灵 y 差**大于**容差 → y 小（更近/更靠下）者盖 y 大者；B2 两精灵 y 落进 `ySortCmpEpsilon` 判等带（1e-5·\|y\|）→ 键判等 → 提交序号（DFS Order）破平票；B2 输入每帧相同，窗持续动帧 | y 键判序 + 平票不闪 | `order_by_y`（B1 盖序断言）、`tie_pair_keys`（B2 两键实测值 + 判等成立）、`tie_order_frames`（连续 N≥120 帧平票序逐帧记录且恒同一序） |
| C Height→投影→盖序 + LandShadow | 三只精灵同 (x,y) 不同 `world.Transform.Height`（0/中/高），经 `world` Transform 折叠（`composeTransform` 高度纯求和）→ `Basis25D.Project(Vec3{x,y,Height})` 得屏幕脚位 → 脚位 y 进 `camera.YSortKeyY` 排序 → 高者脚位更低（更近）盖矮者；每只精灵由 `camera.LandShadow(b, Vec3{x,y,Height}, groundY)` 落影子到地面线、`ShadowVisible` 判可见 | 高度经投影改变盖序（不改排序键公式——键仍取脚位 Y，D-D 口径）+ 影子落地面 | `height_foot_y`（三档 Height → 投影脚位 y 值）、`cover_by_height`（交叠点像素随 Height 档位变）、`shadow_landed`（LandShadow 落点坐标 + `ShadowVisible` 值 + 影子像素在地面线上的断言） |
| D 非有限 fail-closed（拒绝态底带，与 ① 同形态） | 底带显示四类拒绝实测计数：`camera.YSortTree` 喂 NaN/Inf pos → `core.InvalidArg`；`sprite.NewItem` NaN feetY → `core.InvalidArg`（委托面先拒）；`camera.YSortZ(越限 rank)` → `core.OutOfMemory`；平铺 `camera.YSort` 对非有限**不校验**（批 2 钉下的口径，如实记探针，不算洞） | 拒绝口径分层如实 | `nonfinite_rejects`（四项各自 got/err 计数） |
| E 跨 tier 提交序=盖序不变式 | 精灵（image tier）与路径填充矩形（SDF tier）交错提交：path→sprite→path→sprite，交叠处断言 = **最后提交者**，与 tier 无关（`render/p37_draw_order_test.go` 的窗内像素版） | 提交序=盖序 | `tier_invariant_hits`（逐交点断言值） |

三、画面在动（H15/H16 承重）
----------------------------
- 窗有一路缓慢横移（B1 带 y 键对的脚位随 pan 平移，A/C/D/E 臂固定），常驻相比对区差非零证「画面真在动」；
- B2 平票对**输入恒定**（脚位不随 pan 变），是「平票不闪」的承重载体；
- H15 定帧冻结：`gateFreezeAt=120` 定帧停机，金图六轮恒 0（播种跑与缺基线跑都判红）；
- H16 变异四处（各出实测报警值）：
  1. A 臂某越界精灵 ZIndex +5000→-5000 ⇒ 桶序翻转 ⇒ 金图报警；
  2. B2 平票对其中一只 y 微移**出**容差带 ⇒ y 键接管 ⇒ 盖序翻转 ⇒ 金图报警；
  3. C 臂最高档 Height 归零 ⇒ 三只脚位重合回平票序 ⇒ 盖序变 ⇒ 金图报警；
  4. E 臂最后提交的 path 改为先提交 ⇒ 交点像素翻色 ⇒ 金图报警。

四、像素断言口径（§3 硬要求第 4 条）
------------------------------------
全部从「画面所用同一份输出」反推：精灵屏幕位置 = 窗内同一份 `Basis25D.Project`/排序
输出的返回值（不手写屏幕常数）；交叠探针点 = 两精灵矩形按各自绘制序的交集几何计算。

五、双后端与门禁相
------------------
native（默认）与 `GPUI_BACKEND=go` 各一轮；两基线互比整窗零差（含指标带，① 同口径）；
`-auto-only` 门禁相（`RUN_SECONDS` 定帧 + JSON 出 stdout + exit 1）与 `-manual-seconds`
常驻人工相二合一；一轮一窗、WM_DELETE 正常关闭、每轮查显示健康（GPU 真窗安全纪律）。

六、公开 API 影响面
-------------------
**零新增公开 API**（① 为 H12 加的 `ImageBatchStats` 在 ② 无对应需要——② 的探针全是
引擎侧可读值：排序结果、投影值、错误计数、像素）。`examples/engine/p37_height_zsort`
只调引擎公开口（camera/sprite/world）与 render Canvas——无组件层、无示例层绕洞。

七、证据落点与收口动作
----------------------
`batch6/window-2-height-zsort.md`（判据与差值）+ `window-2-runs.md`（原始跑数）+
`window-2-gates.md`（门禁账）；收口后台账 743 行 ☑→✅ 附窗名+差值+提交号，
`batch6-window-plan.md` §5 补交付进展。

八、待拍板（question 停报告）
------------------------------
1. 窗名 `p37_height_zsort` 与五臂布局（A–E 一屏五带 + 拒绝态底带）是否照此开工；
2. C 臂投影基用 `Basis25D`（与 LandShadow 同基、等轴测口径）而非 `Projector`（透视）——
   design §7 把 Basis25D 列在 ② 窗能力列，且影子落点与脚位必须同源同基；确认；
3. H16 变异四处清单是否照此钉。
