# P3.5 批 5 负向对照（布尔保曲线 / D6 混合法）

依据：`evidence/P3.5/assessment.md` §门禁「布尔摘回贴→保曲线钉红」+「曲线段保留断言（对比全 flatten 版折线段数）」+「|A∪B|=|A|+|B|−|A∩B| 面积对账」。
方法：临时改实现（摘能力）→ 对应钉必须红 → 还原 → 绿（还原后与 `/tmp/pbr.b5bak` 一致，`grep NEGATIVE CONTROL` = 0）。

| # | 摘除点 | 红掉的钉 | 日志 |
|---|---|---|---|
| 1 | `refitClipperPaths` 整体摘掉（退回纯折线输出） | `TestBooleanPreservesCubicRuns`（loses arcs）、`TestBooleanUncutContourReproducedExactly`（0 cubics，want 4）、`TestBooleanHoleKeepsExactArcsReversed`（hole lost arcs）、`TestBooleanAreaReconciliation`（circle/rect 面积恒等偏差 0.48%、circle/blob 42.8%）、`TestPathBooleanCurveInputsBoundaryAccuracy` | `01-mutate-refit-off-red.log`（还原 `02-restore-green.log`） |
| 2 | 只摘 `recordSharedStart`（曲线段之间的接缝 t=0 不记） | `TestBooleanUncutContourReproducedExactly`（接缝处被 Clipper 的任意起点劈开，不再是逐 verb 复现）、`TestBooleanPreservesCubicRuns`（弧偏离原曲线 2.5e-4）、`TestBooleanTraceRecordsRunsAndGrid`（直线-only 顶点 3≠2）、`TestBooleanAreaReconciliation`（0.4278→0.0408 相对偏差） | `03-mutate-seam-start-red.log`（还原 `04-restore-green.log`） |

## 钉住的标准口径（可复算）

- 回贴弧 = 原曲线精确子段：把输出 cubic/conic 各取 17 个采样点，用三分法数值反演到**原输入曲线**上求距离，
  要求 ≤1e-6（未切割轮廓）/≤1e-3（切割端点落在 Clipper 1/1024 网格顶点上，量化误差本身 ≈4.9e-4）。
- 面积恒等：`|A∪B| + |A∩B| = |A| + |B|`、`|A−B| = |A| − |A∩B|`、`|A⊕B| = |A|+|B|−2|A∩B|`，
  面积用 `Path.Area()` 的解析曲线积分（quad/cubic/conic 各自精确），相对误差 ≤2e-3。
- 曲线段保留 vs 折线段数：`NumVerbs() < len(Flatten(0.1))`（实测 circle∪rect = 8 verbs / 26 折点，
  blob−rect = 7 verbs / 59 折点）。
- 纯直边输入零变化：`TestBooleanStraightInputsStayPolygonal` + 存量 `TestPathBooleanDifferenceNoFragmentation`
  （verbs=10 且 `!HasCurves()`）仍绿——直边与交角保留 Clipper 输出的口径没被破坏。

## 过程中改掉的旧口径（如实记录）

- 存量 `TestPathBooleanCurvesFlattened` → 改名 `TestPathBooleanCurveInputsBoundaryAccuracy`，
  并把它「折线数 ≤200」的容忍收到「≤16 verbs」（实测回贴后 8）——原名/原阈值记的是批 5 之前的**限制**，不是要求。
- 真窗 `examples/ui_render_base_pathbool` 探针 8 由 `pathbool_curves_flattened`（断言**无**曲线，
  注释写着 documented scope limit）改为 `pathbool_curves_preserved`（断言有曲线、弧落在 authored 圆上、
  verb 数收紧），并按用户拍板重录该窗逐位金图（见 `../batch5.md` 金图段）。
