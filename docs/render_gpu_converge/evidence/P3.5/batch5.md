# P3.5 批 5 — 布尔保留曲线（D6 混合法）交付证据

- 日期：2026-11-02　分支：`feat/p2-收敛整合`　基线 HEAD：`a84f8c38`（批 4）
- 真源：`docs/ENGINE_RENDER_GPU_CONVERGE_TASKS.md` §2.1 载体块「批 5」条 + `evidence/P3.5/assessment.md` §1.5（现状盘点）、§能力定义「布尔保曲线」行、§拍板 D6、§门禁/负向/风险条
- 口径：**原子验证 + 直调真窗**（引擎层布尔在 CPU path 层，生产消费者=零，「已验证待接线」）

## 1. 实现面（文件级）

| 文件 | 改动 | 内容 |
|---|---|---|
| `render/path_boolean_refit.go` | 新增 | 曲线来源记录 + 回贴：`boolCurveRun`/`boolTrace`（采样按 Clipper 网格键登记 `段 id + 参数 t`）、`flattenQuadT`/`flattenCubicT`（与既有展平器**逐点同序**、参数精确）、`flattenConicT`（父参数自适应分裂，双重判据）、`pathToClipperPathsTraced`（含 `recordSharedStart` 接缝登记）、`refitClipperPaths`/`refitContour`/`matchCurveSpan`/`emitCurveSpan`（正反两种走向的子段翻转发射） |
| `render/path_boolean.go` | 改 | `BooleanPath` 换用带 trace 的展平 + 回贴输出；删除不再使用的 `pathToClipperPaths`/`addClipperPoint`/`clipperPathsToPath`（单一真源，不留半新半旧）；文档注释改为「展平只为定交，输出保曲线」 |
| `render/path_boolean_refit_test.go` | 新增 | 包内钉 4 条：展平逐点同序 + 参数精确、conic 容差等级、trace 登记与网格一致性（含 t=0 接缝、直线-only 顶点数） |
| `render/path_boolean_curves_test.go` | 新增 | 行为钉 6 条：直边输入零变化、立方弧保曲线并对 authored 曲线 ≤1e-6、未切割轮廓逐 verb 复现、反向孔洞翻转精确、面积恒等（4 恒等式 × 5 形状对）、conic 输入保 conic、内外分类不变 |
| `render/path_boolean_test.go` | 改 | `TestPathBooleanCurvesFlattened` → `TestPathBooleanCurveInputsBoundaryAccuracy`（原名记的是批 5 前的限制），折线容忍 `verbs ≤ 200` 收紧为 `verbs ≤ 16`（实测 8） |
| `examples/ui_render_base_pathbool/probes.go` | 改 | 探针 8 `pathbool_curves_flattened`（断言**无**曲线）→ `pathbool_curves_preserved`（有曲线、弧落 authored 圆、verb 数收紧、内外判定） |
| `examples/ui_render_base_pathbool/main.go` | 改 | 期望探针名 `pathbool_curves_flattened` → `pathbool_curves_preserved`（否则门禁报「探针未跑」） |
| `examples/ui_render_base_pathbool/testdata/showcase_pathbool_base.png` | 重录 | 按用户拍板重录（几何由折线改精确曲线的必然变化）；旧基线留档 `golden-batch5/showcase_pathbool_base.prefix.png` |

## 2. 对齐标准

- 目标语义 = Skia `SkPath::Op`：布尔结果保留曲线段（`SkPathOps` 原生按 conic/quad/cubic 输出）。
- 本批按 **D6 拍板混合法**落地：Clipper（Vatti 扫描线）继续负责求交与拓扑，输出侧把同一条曲线的连续采样链回贴成该曲线的 `Subsegment` 精确子段；
  直边与交点顶点保留 Clipper 结果。真曲线布尔引擎（Bezier-Bezier 解析求交 + 自研拓扑）按 D6 明确为**远期另立**，不在本批。
- 参数追踪的正确性依据：受限参数区间 `[t0,t1]` 上 de Casteljau 取中点 = 父曲线参数 `(t0+t1)/2`（Bézier 仿射重参数化性质），
  由 `TestBooleanFlattenPointSequenceUnchanged` 逐点钉死（quad/cubic 与既有展平器输出**同序同值**，故 Clipper 所见多边形不变、求交位置不变）。
- conic 例外：`ConicBez.Subsegment` 的子段参数是父参数的 **Möbius 变换**（本机 spike 实测本地 0.5 ≠ 父中点，偏差 0.42–0.53px），
  既有展平器又先把 conic 降解为 quad（丢父参数），故 conic 改走父参数自适应分裂；判据同时约束
  「conic→quad 近似误差 ≤ tol/2」与「quad→弦偏差 ≤ tol/2」两道，维持与既有展平器同级的 0.1px 容差（`TestBooleanConicFlattenSamplesStayOnConic` 钉）。
  conic 只来自批 1 的 `Path.ConicTo` 新口，无存量消费者，无金图依赖。

## 3. 门禁结果

| 门禁 | 结果 | 日志 |
|---|---|---|
| 定向（布尔 + pathops + 曲线族，count=1） | 65 条 PASS（加钉后终验 **66 条**） | `targeted-batch5/targeted.txt`、`targeted-batch5/final-verify.txt` |
| `CGO_ENABLED=1 -race -count=2` | ok（终验复跑亦 ok） | `race-batch5/race.txt`、`targeted-batch5/final-verify.txt` |
| conformance 双后端 | PASS | `conformance-batch5/conformance.txt` |
| `go vet`（render / internal/gpu / recording / pathbool 真窗包） | 4× rc=0 | `vet-batch5.txt` |
| apidoc 机器校验 | **rc=0** | `apidoc-batch5.txt` |
| 五目标构建（per-package + nogpu） | 见 `build-batch5/` | `build-batch5/*.txt` |
| 计数复核 | 144/184/219/18/172/26 **不变**（回贴全为包内非导出符号） | 本文件 §1 |
| 负向对照两路 | 全红→还原绿 | `negative-control-batch5/` |
| 主逐文件 / 补充分包 | **696 PASS/21 SKIP/9 FAIL/1 NO_TESTS** + **72/2/6**：零新增红、1 枚基线红转绿、2 个新测试文件入绿（§4） | `per-file-tests-batch5{,-extra}/` |
| 真窗 `ui_render_base_pathbool` | 逻辑 10/10 + 像素 18/18 + Golden 重录后逐位 0.0000%；抓屏子项在本会话不可用（Wayland 表面，HEAD 同红，见 §5） | `golden-batch5/` |

## 4. 逐文件回归与基线对账（批 4 基线：主 693 PASS/21 SKIP_TAGGED/10 FAIL/1 NO_TESTS，补充 72/2/6）

主门禁（`per-file-tests-batch5/`，最终树）：**696 PASS / 21 SKIP_TAGGED / 9 FAIL / 1 NO_TESTS**（727 行 vs 基线 725 行）。

- 行差分逐项：新增 2 行 = 本批 `render/path_boolean_curves_test.go`、`render/path_boolean_refit_test.go`（均 PASS）；消失行 0。
- 状态变化只有 1 条：`render/s6_9_heavy_budget_test.go` FAIL→PASS（批 3/4 已定性的墙钟 p50 贴线漂移项，本轮转绿=只减不增）。
- 9 枚红名全部落在批 4 基线名单内：`render/context_misc_draw_gpu`、`render/filter_present_coherence`、
  `render/rasterizer_tile_value`、`render/text_transform_gpu`、`render/video_direct_upload`、`ui/application/app_test`、
  `ui/rendering/m1_noquadratic`、`ui/rendering/m5_keystroke_g1`、`ui/scene/internal/gpupixel/blit_rrect_corner`。
  m1/m5 两枚是批 4 已按同机 clean-HEAD A/B 定性的比率门漂移（两树 5/5 同红同量级），本轮继续复现，非本批引入。
- **零新增红**。金图类文件 `render/internal/gpu/golden_test.go`、`golden_software_test.go` 均 PASS
  （除 `ui_render_base_pathbool` 外**存量金图零变化**——布尔输出不经 GPU 管线，唯一受影响的就是这张窗，见 §5）。
- 补充分包（`per-file-tests-batch5-extra/`）：**72 PASS / 2 SKIP_TAGGED / 6 FAIL**，
  6 枚与批 4 名单逐名一致，全为 HEAD 既有坏包 `render/recording/backends/raster`（P3.1 线债，本批未动）。

## 5. 真窗 `ui_render_base_pathbool` 与金图重录（用户拍板：当前桌面单窗，一轮一窗，WM_DELETE 正常关，每轮查显示服务）

| 轮 | 命令 | 结果 | 日志 |
|---|---|---|---|
| 1 | `RUN_SECONDS=6 … -auto-only`（旧基线在场） | 逻辑 10/10（新钉 `pathbool_curves_preserved`：verbs=11 curve_verbs=4 arc_off=0.00576 max_radius=20.006 curves=true）、像素 18/18、**Golden diff=0.1499% over 627200 px** | `round1-vs-old-base.{json,log}` |
| A/B | clean HEAD `a84f8c38` worktree 同窗同基线 | 逻辑 10/10（旧钉 `pathbool_curves_flattened`：verbs=19 curves=false）、**Golden diff=0.0000%**、同样 `screen check grab failed` | `head-window-ab.log` |
| 2 | 清基线 + `GPUI_ACCEPT_GOLDEN=1` | 新基线播种（82252 B，旧基线留档 `showcase_pathbool_base.prefix.png` 79585 B） | `round2-seed.{json,log}` |
| 3 | `RUN_SECONDS=8 … -auto-only -screencheck=false` | 逻辑 10/10、像素 18/18、**Golden diff=0.0000% over 627200 px** | `round3-verify-nograb.{json,log}` |

差异定位（旧基线 vs 新基线整帧 PIL 比对）：**1822 px（0.1898% of 960000）**，
变化带 = 表头 y<48 **0 px**、y<200 108 px、200–500 521 px、y≥500 1193 px，
bbox (165,174)–(1057,775) —— 全部落在四卡绘图区，即曲线参与的那几张卡（B 双圆并、C 菱∩圆、D 远坐标双圆异或），
A 卡（纯直边 Difference + 分数洞）与表头/图例零变化，与「直边输入逐字节不变」的单测口径一致。

显示服务健康：三开窗前后 `Xwayland=1 / gnome-shell=2 / session seat0` 均在场，无崩溃残留。

**顺带发现（非批 5 引入，已归位登记、本批未修）**：gate 模式的抓屏同区子项在本机 Wayland 会话必然失败——
`grabClientArea` 用 `xwininfo -name <标题>` 取窗口几何，而 gpui 在 `XDG_SESSION_TYPE=wayland` 下开的是
Wayland 表面，X11 侧查不到（`xwininfo -root -tree` 也列不出该窗）。clean HEAD 同窗复现同一错误 → 与本批无关。
影响：该窗 `-auto-only` 在本会话的 rc 恒为 1（唯一失败项就是抓屏），逻辑/像素/Golden 三项全绿；
待立案 = 抓屏改走 Wayland 原生通道（portal/`grim`）或在 X11 会话跑（归真窗测量线，不在渲染引擎线）。

## 6. 显式不做与遗留

- **真曲线布尔引擎**（Bezier-Bezier 解析求交 + 曲线级拓扑）按 D6 远期另立；本批求交仍基于 0.1px 展平多边形，
  所以「切割点落在曲线上的位置」精度是展平精度级（≈1e-4~1e-3px 量化 + 弦近似），而**输出段的形状是精确的**。
- 生产消费者仍为零（`BooleanPath`/`Path.Op` 仅测试 + 直调真窗），状态按 AGENTS 口径只到「已验证待接线」，不标生产。
- P3.5 真窗 `examples/ui_wr_p35_pathgeom` 五卡仍未建（批 1–5 同口径遗留）。
- 本批顺带修掉批 4 提交带入的文档表格 defect（catalog §8 `Matrix4` 行尾粘进 `Path1DStyle` 行、台账批 4 修订行缺列），
  并对两本全档做了表行列数一致性扫描。
