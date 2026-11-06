批 6 ⑤ · 原始跑数（2026-11-06，本机；日志 tmp 不入库，关键数字誊抄如下）
====================================================================================

探针迭代留痕（窗自身配方纠偏三轮）
--------------------------------
- 轮 1（首版 σ4/远环 98px/中灰 L≈0.61）：四枚红——
  `reference_halo_engages` 近环 15.51≡背景（σ4 尾在 38px 处 e^-10≈0，量化后全空）；
  `threshold_sweep_lights_dim_region` 1.0 差 < +2（L≈0.61 贴阈值线 + ramp 分母
  算错 + 8-bit 域 +3/255 被吞三重叠加）；sigma/intensity 同因（ROI 距离账没算对）。
- 轮 2（中灰改 100,105,115→L≈0.41 远离 0.6；ROI 按半平面尾重排：近环 8px、
  远环 20px；基准 σ8、强度组保持 1.6）：只剩 `sigma_sweep_widens_halo` 红——
  σ12 远环实测差 +3.9 < +5（8-bit 尾薄）。
- 轮 3（半径组抬到域顶 σ16=MaxBloomSigma）：11 枚全绿；仅剩缺基线（预期红）。

金图播种与复核（fail-closed 口径）
--------------------------------
- native 播种：`GPUI_ACCEPT_GOLDEN=1 … -auto-only` → rc=1「golden 已播种，这一跑
  不算 PASS」→ 复核 `OK backend=native presents=241 probes=11/11 golden=0.0000%
  (921600 px)`。
- go 播种：`GPUI_BACKEND=go GPUI_ACCEPT_GOLDEN=1 …` → rc=1 播种判红 → 复核
  `OK backend=go presents=241 probes=11/11 golden=0.0000% (921600 px)`。
- 两基线 `cmp`：逐位相同。
- 两端数字对账：`panel_pair_diffs` 十对**逐键全同**（p0↔p1 53580 / p0↔p2 76724 /
  p0↔p3 101004 / p0↔p4 53580 / p1↔p2 48300 / p1↔p3 85316 / p1↔p4 38480 /
  p2↔p3 107532 / p2↔p4 61620 / p3↔p4 85060）；`bloom_leg_stats_report_only`
  两端同为 `gpu_ops=5 cpu_fb=0 last_fb=""`。

H15 定帧六轮（native，-auto-only）
--------------------------------
| 轮 | presents | probes | golden |
|---|---|---|---|
| 播种后复核 | 241 | 11/11 | 0.0000% |
| 2 | 240 | 11/11 | 0.0000% |
| 3 | 241 | 11/11 | 0.0000% |
| 4 | 241 | 11/11 | 0.0000% |
| 5 | 240 | 11/11 | 0.0000% |
| 6 | 241 | 11/11 | 0.0000% |

常驻人工相（-manual-seconds 6）
--------------------------------
`resident done backend=native presents=354 probes_ok=true pixel_ok=true
golden=0.0000%`——静态内容窗，常驻相金图同 0，如实记。

H16 两变异（窗参一行→跑→还原→终验复绿）
--------------------------------
| 变异 | 探针报警 | golden diff |
|---|---|---|
| 1. refIntensity 0.60→0.00 | 五枚红：halo 15.51≡背景 / 阈值组 104.66≡基准 / 远环≡基准 / pairwise p0↔p1=0 / changes_picture 0 字节 | **8.4939%** |
| 2. refThreshold 0.60→0.05 | `threshold_sweep_lights_mid_region` 关系翻转（基准中灰 128.59 > 阈值组 123.80） | **8.1050%** |

还原核对：`grep` 两参回 0.60/0.60 → `git diff` 窗文件零命中 → 终验
`OK backend=native presents=240 probes=11/11 golden=0.0000%`。
