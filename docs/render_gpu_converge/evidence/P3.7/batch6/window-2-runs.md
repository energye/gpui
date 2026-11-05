批 6 ② · 原始跑数（2026-11-06，本机；日志 tmp 不入库，关键数字誊抄如下）
====================================================================================

金图播种与复核（fail-closed 口径）
--------------------------------
- native 播种：`GPUI_ACCEPT_GOLDEN=1 … -auto-only` → 「golden 已播种，这一跑不算
  PASS」（rc=1，播种判红兑现）→ 复核跑 `OK backend=native presents=241
  probes=22/22 golden=0.0000% (921600 px) tie_frames=240`。
- go 播种：`GPUI_BACKEND=go GPUI_ACCEPT_GOLDEN=1 …` → 播种判红 → 复核跑
  `OK backend=go presents=241 probes=22/22 golden=0.0000% tie_frames=240`。
- 两基线 `cmp`：逐位相同（整窗零差含指标带）。

H15 定帧六轮（native，-auto-only）
--------------------------------
| 轮 | presents | probes | golden |
|---|---|---|---|
| 播种后复核 | 241 | 22/22 | 0.0000% |
| 1 | 240 | 22/22 | 0.0000% |
| 2 | 241 | 22/22 | 0.0000% |
| 3 | 241 | 22/22 | 0.0000% |
| 4 | 241 | 22/22 | 0.0000% |
| 5 | 241 | 22/22 | 0.0000% |

常驻人工相（-manual-seconds 8）
--------------------------------
`resident done backend=native presents=473 probes_ok=true pixel_ok=true golden=0.0755%`
——常驻相末帧未定格（H15 只在门禁相成立），像素与金图只报数不判绿；金图 0.0755%
非零正是「画面真在动」的旁证（对照：门禁相六轮恒 0）。

H16 四变异（改码→跑→取值→还原；还原后终验复绿）
--------------------------------
| 变异 | 探针报警 | 像素报警 | golden diff |
|---|---|---|---|
| 1. A3 ZIndex 5000→-5000 | `z_bucket_cover_order` order=[0 3 1 2] | (358,114) 红→琥珀 | 0.25% |
| 2. B2 键差 1e-9→-0.05 | `tie_keys_equal_approx`（ka=396 kb=395.95 不判等） | (940,364) 橙→紫 | 0.3056% |
| 3. C 子高 1.0→-2.0 | `height_folds_and_projects_monotonic`（foot_y=[514.7,537.4,492.1] 非单调） | (236,520) 红→青 | 0.9566% |
| 4. E k4/k5 色序对调 | `tier_last_submission_wins` | (530,647) 品红→红 | 1.1079% |

还原后终验：`OK backend=native presents=241 probes=22/22 golden=0.0000% tie_frames=240`。
