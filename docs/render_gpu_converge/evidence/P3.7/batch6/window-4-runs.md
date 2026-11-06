批 6 ④ · 原始跑数（2026-11-06，本机；日志 tmp 不入库，关键数字誊抄如下）
====================================================================================

探针迭代三轮（窗自身口径纠偏留痕）
--------------------------------
- 轮 1（首版）：探针 1/2/6/7 绿；三枚红——
  `mip_three_levels_differ_on_screen` 两两 0px（三档全挂 InterpNearest，
  引擎契约屏蔽档位）、`filter_three_levels_differ_on_screen`
  nearest↔bilinear 0px（8px 棋盘 × 整数 4x：bilinear 采样对恒同格）、
  `magnify_bilinear_exact_interior` got=(112,126,82)（断言点落 32/32 纹素
  边界，实测正是解析混合值）。
- 轮 2（臂重建：噪声图 + Bilinear 档路 + 非整数 2.667x + 放大段双点式）：
  10 枚探针全绿；语义像素红一枚 `zero_road_engine_center`
  got=(221,134,95)——轮 1 被前一枚失败短路没露出；按历史路归一 bilinear 的
  解析公式（0.3×亮线+0.7×格色）核为正确行为，期望修正。
- 轮 3：探针 10/10 + 语义 7/7 全绿，仅剩金图缺基线（fail-closed 判红，预期）。

金图播种与复核（fail-closed 口径）
--------------------------------
- native 播种：`GPUI_ACCEPT_GOLDEN=1 … -auto-only` → 「golden 已播种，这一跑
  不算 PASS」（rc=1，播种判红兑现）→ 复核跑 `OK backend=native presents=241
  probes=10/10 golden=0.0000% (921600 px)`。
- go 播种：`GPUI_BACKEND=go GPUI_ACCEPT_GOLDEN=1 …` → 播种判红 → 复核跑
  `OK backend=go presents=240 probes=10/10 golden=0.0000% (921600 px)`。
- 两基线 `cmp`：逐位相同（整窗零差含指标带）。
- 两端探针数字一致：`mip_pair_diffs` {none↔nearest 23032, none↔linear 23032,
  nearest↔linear 22036}、`zero_road_diff_px=0`、`paint_gpu_ops=25`、
  `mapping_combos=12`。

H15 定帧六轮（native，-auto-only）
--------------------------------
| 轮 | presents | probes | golden |
|---|---|---|---|
| 播种后复核 | 241 | 10/10 | 0.0000% |
| 2 | 240 | 10/10 | 0.0000% |
| 3 | 241 | 10/10 | 0.0000% |
| 4 | 241 | 10/10 | 0.0000% |
| 5 | 241 | 10/10 | 0.0000% |
| 6 | 241 | 10/10 | 0.0000% |

常驻人工相（-manual-seconds 6）
--------------------------------
`resident done backend=native presents=352 probes_ok=true pixel_ok=true
golden=0.0000%`——④ 为静态内容窗（无墙钟相位），常驻相金图同 0，如实记；
「画面真在动」的旁证由 H16 变异金图报警承担（对照 ② 动画窗常驻相 0.0755%）。

H16 两变异（改码→跑→取值→还原；变异触及引擎码 ToRender 各删一行）
--------------------------------
| 变异 | 探针报警 | golden diff |
|---|---|---|
| 1. 摘 `Mipmap: s.Mipmap`（atlas.go:282） | `torender_carries_all_combinations`（(bicubic,linear)→(bicubic,none)）+ `mip_three_levels_differ_on_screen`（三档全等图 0px） | **1.9829%** |
| 2. 摘 `Filter: s.Filter`（atlas.go:281） | 携带红（→(default,linear)）+ `filter_three_levels_differ_on_screen`（三档全等图 0px，回退 bilinear 老路） | **1.9669%** |

还原核对：`git checkout -- engine/sprite/atlas.go` → `git diff` 零命中 →
grep 双行（Filter/Mipmap 传递）原位各 1 处 → 终验跑
`OK backend=native presents=240 probes=10/10 golden=0.0000%`。
