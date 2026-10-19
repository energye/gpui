# ui_render_base_strokepreset — 描边预设族真窗

覆盖 A 组缺窗清单第 3 项（`docs/RENDER_FEATURE_COVERAGE.md` §1 #7 / §3.1）：
`render` 的描边预设族 **DefaultStroke / Thin / Thick / Bold / RoundStroke /
SquareStroke / DashedStroke / DottedStroke**（含 `SetStroke` 生效路径）。这些预设
此前全仓非测试零调用、无真窗（§1 #7 记 ⛔），本窗是它们的第一个直调画面证据。

## 怎么跑

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

go run ./examples/ui_render_base_strokepreset                    # 人工窗：常驻，收真事件打日志+改标题，关窗后出汇总+JSON
go run ./examples/ui_render_base_strokepreset -manual-seconds 15 # 人工窗：15s 后自动收尾（出汇总+JSON）
RUN_SECONDS=5 go run ./examples/ui_render_base_strokepreset      # 定时窗（≥5s，U16）
go run ./examples/ui_render_base_strokepreset -auto-only         # 门禁：JSON + exit code
```

首次跑会提示 golden 基线缺失；确认画面无误后用
`GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=5 go run ./examples/ui_render_base_strokepreset -auto-only`
播种 `testdata/showcase_strokepreset_base.png`，评审后随窗提交；之后每跑必须
**逐位零差**（`pixel_golden_diff_pct == 0`）。

## 画面（四张卡，1200×800）

| 卡 | 演示 | 为什么必须这样画 |
|----|------|------------------|
| A | 四条水平线：`Thin()` 0.5px 琥珀 / `WithWidth(1)` 青绿 / `Thick()` 3px 洋红 / `Bold()` 5px 绿 | 四档宽度在像素上逐级变粗，上下边缘各留一行「基本纯背景」= 线宽是真的、不是同一条粗线 |
| B | 三条 9px 线，端点帽不同：`DefaultStroke`(butt) 淡紫 / `SquareStroke` 橙 / `RoundStroke` 黄 | butt 端点收在坐标上、square 外扩半个线宽、round 外扩半圆 = `LineCap` 三种语义各成一景 |
| C | 两个默认 miter 拐角（下→右、上→右）+ 一个 `RoundStroke` 拐角（w=9） | miter 尖角必须**填满直角区**（尖点 = 两条外偏移线的交点）；round 拐角外角为空、只留 1/4 圆弧 = `LineJoin` 两态可判别 |
| D | `DashedStroke(8,5)` 5px 青柠虚线 + `DottedStroke()` 点线 | 虚线画/空交替可数、点线圆帽成 2px 点、2px 空隙 = 预设自带的 dash 链真的独立生效 |

## 实现修正（2026-10-19，与建窗同批）

`render/internal/stroke/expander.go` 的 **miter 拐角在一个转向下尖角算错**，已修：

- **修前证据**：临时探针打印展开轮廓，折线 `(60.5,40.5)→(60.5,140.5)→(170.5,140.5)`
  （下→右转向）width 9 时吐出 `(60.5,145) → (65,145) → (56,140.5)`——miter 尖点
  应为两条外偏移线 `x=56` 与 `y=145` 的交点 `(56,145)`，实际算成 `(65,145)`
  （镜像侧）。轮廓自交后尖角区域被判为外部，视觉上直角被一条角度不对的斜边切掉。
  真窗像素探针同位置读到纯背景 + 邻格 81% 覆盖，与轮廓打印互证。
- **根因**：`applyOuterMiterJoin` 里 `h := ab.Cross(Δ)/crossAbs` 分母用**无符号**
  `crossAbs`，丢掉转向符号 → 交点在两个转向里只对一边。Skia 用**带符号** cross
  求两偏移线交点，两个转向对称。
- **修法**：改 `t := Δ.Cross(cd) / ab.Cross(cd)`、`miterPt := fpLast + ab*t`
  （`ab`/`cd` 的尺度自动抵消）。公开签名未变；行为变化只发生在「转向为
  `ab×cd < 0`」的一类拐角，另一转向输出逐点不变。
- **配套**：新增 `render/internal/stroke/expander_test.go:TestStrokeExpander_MiterJoinTipBothDirections`
  ——正反两个 90° 转向各断言「尖点必须出现 + 镜像点必须不出现」，修前红
  （down-then-right 报 `miter tip {56 145} missing` + `mirrored point {65 145} emitted`）、
  修后绿。
- **连带**：`ui_render_base_pathshape` 的 golden 因本修复有 0.0002%（1 像素）变化，
  同批重播并连跑两次 `diff=0`。

## 三证据

1. **逻辑探针**：12 项值级契约（启动时跑一次，纯值检查、不依赖绘制）
   - 预设值：`preset_widths`（0.5/1/3/5）、`preset_thick_bold`、`preset_round`、
     `preset_square`、`preset_dash_pattern`（8/5）、`preset_dotted`（w2 + 圆帽/圆角 +
     0.1/4 dash，即 Skia 点线惯用法）、`preset_default`（w1 + butt + miter + 4.0）、
     `preset_immutable`（预设返回的是拷贝，`WithWidth` 不改原值）
   - dash 语义：`with_chain_copy`、`dash_pattern_len`、`dotted_skia_gap`、
     `apply_dash_ranges`（区间片段）、`apply_dash_solid`（无 dash 原样）、
     `stroke_clone_deep`
2. **像素断言**：28 个探针（A 四档填充 + 四档「边下基本背景」、B 三帽 9 个判别点、
   C miter 尖角/直角内/镜像外空角 + round 弧内/空角、D 虚实 8 个点），期望值直接读
   调色板常量，容差 8/255（采样点都在色块中央，只有 A 卡两处「边下基本背景」按
   已知限界放宽，见下）
3. **Golden**：表头 + 图例 + 四张卡共 627200px 逐位零容差对比

另外整窗输出 §2.2 族 A–J 指标 JSON（`wrgate.BuildReport`），门禁模式跑
`EvaluateGates`（`present_policy=full_paint`）。

## 实测（2026-10-19，X11 + wgpu-native，3 连跑）

- 逻辑探针 12/12；像素 28/28；golden 播种后连跑两次 `diff=0.0000% / 627200px`
- miter 修复前后对照：C 卡尖角像素 `(352,499)` 从「纯背景 + 邻格 81%」变为
  **100% 尖角色**（`C_miter_tip` / `C_miter_in` 两个探针由红转绿）
- `present_count ≈ 300`、`cpu_fallback_ops=0`；人工窗 `-manual-seconds 7` 实屏
  `xwd` 截图确认上屏（四卡与标题栏事件状态均在屏上）
- 描边相关回归逐文件跑全绿：`render/stroke_test.go`、`stroke_join_solid_test.go`、
  `context_stroke_test.go`、`dash_test.go`、`antialias_test.go`、`hairline_visual_test.go`、
  `render/internal/stroke/`（整包）

## 已知限界（如实记录，均为本窗暴露、未在本批修）

- **sub-1px 描边被钳到 1px**（记录为子项）：`render/software_stroke.go:49-51`
  `if effectiveWidth < 1.0 { effectiveWidth = 1.0 }`（注释 "Minimum 1px stroke for
  visibility"）→ `Thin()`（0.5px）实渲成**满不透明 1px 线**（像素实测 A 卡 0.5 线
  中心为纯琥珀 100%）。展开器本身在 0.5 宽是对的（探针实测 band=[45.25,45.75]），
  钳位发生在 `SoftwareRenderer.Stroke`。Skia 不钳：0.5px 应渲成 ~50% alpha 的 1px 线。
  本窗的 `A_thin_ink` 探针只断言「有墨」（kindNotBG），钳位取消后探针仍成立、
  只需重播 golden。
- **窄线边下 ~1/3 拖尾**（记录为子项）：0.5px（钳后 1px）与 1px 线的正下一行有
  ~33% 墨色混合（实测 A 卡 `A_thin_clear`≈0.36/0.33/0.33、`A_w1_clear`≈0.33），
  3px / 5px 线的同一位置为纯背景。因机制未定因，两处探针按「基本纯背景」类声明
  （限界 0.5），README 如实记录，待单独定位。
- 逻辑探针是值级契约（启动时一次），不依赖绘制路径；绘制正确性由像素探针 +
  golden 承担，两者互为独立证据。
- 本窗是**原子验证**（直调 render 接口一次验一族），不是生产接线；
  `ui/rendering` 等胶水层是否转用这些预设属于后续接线问题。
- 动画/FPS 门不在本窗口径内（正确性窗）：`fps_gate=off_correctness`、
  `slope_gate=off`，A–J 指标照常输出不带闸。
