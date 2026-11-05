批 6 ② · 高度与 z 序窗 收口账（2026-11-06；设计见同目录 `window-2-design.md`，窗 `examples/engine/p37_height_zsort`）
====================================================================================

一、收口对象
------------
台账 743 行（批 2 z 序与世界高度归位，原交付 `1985773d`，☑→✅）。批 2 当时的
五窗十轮证据全是 `changed=0`（无回归），没有任何一帧断言过**新盖序**——本窗
第一次把「谁盖谁」的画面断言出来。

二、五臂判据与实测差值（22 枚探针全绿；容差显式：直色 2 档 / alpha 混合 6 档 / 金图 0）
----------------------------------------------------------------
| 臂 | 断言 | 实测 |
|---|---|---|
| A z 桶 CLAMP | `ClampZIndex` 饱和表：-5000/-4097→-4096、±4096 原样、0→0、4097/5000→4096 | `zclamp_map` 全对（探针 `zclamp_saturates`） |
| | 桶序盖序：ZIndex {-5000,-10,10,5000} → 绘制序 [0 1 2 3]，接缝像素=大桶色 | `z_bucket_cover_order` ✓；接缝 (230,114)=青、(294,114)=琥珀、(358,114)=红，零容差内 |
| | z 槽指派往返：y 序 → `ClampZIndex(YSortZ(rank))` → 再按槽排序 = 同序 | `zslot_roundtrip_same_order` ✓ |
| B1 y 键 | y 差 40 出容差带 → 近者（脚位低）后画盖住：交点 (740,260)=绿、远者右侧 (800,250)=蓝可见 | ✓ 零容差内 |
| | 委托对账：`camera.YSort` 序 == `sprite.Sort` 序（批 2 步 2b 生产面） | `delegate_parity_camera_vs_sprite` ✓ |
| B2 平票 | 键差 1e-9 落进 `is_equal_approx` 容差带（tol=1e-5·396≈3.96e-3）→ 提交序号破平票：交点 (940,364)=橙（Order 1），左侧 (900,364)=紫 | ✓ |
| | 平票不闪：离屏 200 轮翻转 0 + **窗内逐帧重排序 240 帧**（JSON `window_tie_frames_checked=240`、`window_tie_stable=true`） | ✓（两轮 240/241 presents 都数满） |
| C Height | 折叠：world 父 1.0 + 子 {-1,0,1} ⇒ 世界高 {0,1,2}（纯求和）；Basis45 投影脚位 y = [514.745, 537.373, 560.000] 单调 | `height_folds_and_projects_monotonic` ✓ |
| | 盖序随高度变：三只精灵脚位阶梯 22.63px/档，(236,460)=绿（矮者顶部可见）、(236,490)=青（中盖矮）、(236,520)=红（高盖中） | ✓——**排序键仍是投影脚位 Y，高度经投影改盖序不改键公式（D-D 口径）** |
| | 影子：`LandShadow(b, V3(x,y,h), groundY=y)` 落点=脚位同点（真源行为：落点保留 z），色片在精灵两侧可见（(180,515)/(182,560)=影子混合色，六档容差）；`ShadowVisible` 全 true；地下目标 ok=false | `shadow_lands_on_ground_line` ✓ |
| D 非有限 | 树路 NaN/Inf/坏父/环 → InvalidArg；`sprite.NewItem(NaN)` → InvalidArg；`YSortWithKey` 4001 → OutOfMemory；平铺 YSort 非有限**不校验**（批 2 钉下口径如实记）；`LandShadow` 地下 → ok=false | `refusal_family_all_expected` 八枚全绿，底带八色片全绿（窗内可见） |
| E 跨 tier | SDF tier 填充与 image tier 精灵交错提交：交点 (230,647)=蓝（精灵盖填充）、(530,647)=品红（最后提交者赢） | ✓ |

三、三证据合证
--------------
1. **逻辑探针**：22 枚（`-auto-only` JSON `probes=22/22`），extra 键含
   `zclamp_map`/`zslot_rank_slots`/`a_draw_order`/`b1_camera_order`/`b1_sprite_order`/
   `tie_keys`/`tie_equal_approx`/`tie_frames_offscreen`/`c_world_heights`/
   `c_foot_y_screen`/`shadow_landed`/`nonfinite_rejects`。
2. **语义像素**：终帧快照 14 枚 `wrkit.PixelProbe`（`-auto-only` 门禁相判绿，
   常驻相只报数）；坐标全部由画面同一份排序/投影输出反推（P3.5 教训）。
3. **Golden 逐位**：`wrkit.GoldenGateW` fail-closed（缺基线红、播种不算过），
   mask 指标带 `y<32`，容差 **0**。

四、双后端与 H15/H16
--------------------
- **双后端各一轮**：native `OK presents=241 probes=22/22 golden=0.0000% (921600 px) tie_frames=240`；
  `GPUI_BACKEND=go` 同数全绿；**两基线逐位相同**（`cmp` 零差，含指标带）。
- **H15 定帧冻结**：native 六轮（presents 240/241 两形态都出现）金图恒 `0.0000%`；
  常驻相（`-manual-seconds 8`，473 presents）非定格帧金图 `0.0755%` 只报数不判绿
  ——证「画面真在动、冻结才承重」。
- **H16 四变异三重报警**（改码→跑→取值→还原，还原后终验复绿）：
  1. A3 ZIndex 5000→-5000：`z_bucket_cover_order` 变 [0 3 1 2] + 接缝翻色 + **金图 0.25%**；
  2. B2 键差 1e-9→-0.05（出容差带）：`tie_keys_equal_approx` 失败 + (940,364) 紫/橙翻 + **金图 0.31%**；
  3. C 折叠高度 子1.0→-2.0：`height_folds_and_projects_monotonic` 失败（foot_y 非单调）+ 盖序翻 + **金图 0.96%**；
  4. E k4/k5 色序对调：`tier_last_submission_wins` 翻色 + **金图 1.11%**。

五、门禁账
----------
`go build ./...` rc=0；`go vet` 触及包 rc=0；`gofmt -l` 窗文件 0 行；apidoc rc=0
（零新增公开 API——② 的探针全是引擎侧可读值，无需新观测口）；全仓 build rc=0。
浮动计时门（`engine/world cull`、`s6_9`）未作收口证据（§3 硬要求第 5 条）。

六、收口动作
------------
台账 743 行 ☑→✅（附本窗名+差值+收口枚）；`batch6-window-plan.md` §5 补交付进展；
§7 追加批 6 ② 行。
