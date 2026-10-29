# P1.7 首帧 / 滚动帧率 / p95 / p99 性能基线

> 生成：2026-10-29；HEAD `390cccb5`；**纯存量分析**——以 P1.4 全部真窗 374 轮（366 非种子轮）的原始 JSON 为底，零 GPU、零补采、不改代码。
> 数据表：`perf_baseline.tsv`（366 行，逐轮 A/H 族字段 + 门禁判定 + P1.4 状态交叉）。

## 范围与口径

- 门禁只来自 `baseline/thresholds.md` §2/§3，未自造数字：
  - **首帧 H 族**（对所有发 `time_to_first_present_ms` 的窗）：普通 UI <2000ms；文本输入类 <1000ms；IME 首焦 <800ms；并要求 `first_present_paint_count>0`。
  - **帧时 A 族硬门禁**（适用域 = windows.md「帧时/调度」标签 17 窗 ∪ thresholds §3 点名 R6/R7/R7b/C3/C5/C10，共 22 窗）：`fps_interval≥55`（无则 `fps_wall`）、`interval_p95_ms≤22`、`hitch_rate_per_min≤5`（仅 ≥60s 轮）。**非适用窗的 fps/p95 一律 obs 只记不判**（如 `ui_base_idle` 是零脏零帧设计窗，其 `fps_wall=0.0165` 是秒/帧量纲，不能按 fps 判）。
  - **p99 只观测**：本轮落盘的 p99 就是基线，P2 起按回归带 ≤8ms 对照涨落。
  - 必采九字段「缺一即 FAIL」只对 A 族适用窗判。
- 首帧实测参考值（thresholds §2 的 R16≈89.6ms、c0≈110ms 等）与本轮数字同量级，无回归迹象。

## 1. 首帧基线（275 轮有值，0 FAIL）

| 项 | 值 |
|---|---|
| 轮数 | 275（其余 91 轮为 NO_JSON/BUILD_FAIL/SKIP/PROBE 等无首帧字段，原因均已在 P1.4 登记） |
| 分布 | median **174ms** / p90 248ms / max **515ms**（top5 全是 go 档 render_base 窗） |
| 预算内 | 275/275（<2000ms 档）；首帧黑屏 0 |
| 紧预算轮（文本输入 <1000 / IME <800） | 18 轮，max **267ms**，全达标 |

## 2. A 族帧时基线（44 轮适用，FAIL 8 轮全部与 P1.4 状态一致）

九字段完备度：**24 轮全齐**；12 轮经别名映射后仅缺 `fps_wall`/`target_hz`（见 §4 字段契约）；6 轮采不了（P1.4 已登记：r15_soak go、r7b go、zz_soak ×2 = NO_JSON，t_stress ×2 = BUILD_FAIL）；2 轮只缺 `interval_p99_ms`（video_vr7_perf，本身 FAIL）。

默认门禁超限共 **8 处，集中在 4 轮**（c3 一轮 fps+p95+hitch 三红），**全部是 P1.4 已登记的 FAIL 窗，零新增、零口径冲突**：

| 窗/后端 | fps | p95 ms | hitch/min | P1.4 状态 |
|---|---|---:|---:|---|
| ui_wr_c3_list_scroll native | **54.66**(<55) | **26.43**(>22) | **52.0**(>5) | FAIL（P1.4 同判） |
| ui_wr_r7_virtlist native | ok | **24.89** | **16.02** | FAIL（P1.4 同判） |
| ui_wr_r7_virtlist go | ok | **22.66** | **15.02** | FAIL（P1.4 同判） |
| ui_wr_r7b_scroll_reuse native | ok | **22.42** | ok | FAIL（P1.4 同判） |

p99 观测基线（36 轮 A 族有值）：median **20.7ms**，max 32.0ms——P2 起任何 p99 相对本表涨超 8ms 记回归。

## 3. 引擎（2.5D）窗对目标线的观测

引擎族 71 轮有 p95 值，median 19.11ms，**71/71 高于 2.5D 目标线 16ms**——这与 thresholds §3 已登记的 S52 豁免一致（本机 59.93Hz 同步底 16.69ms，16ms 物理不可全达），目标线是目标不是 UI 硬门禁，本轮不作判定，仅落盘为对照起点。

## 4. 字段契约差异（P1.4「缺 A 族字段」移交项的落地结论）

P1.4 移交的「缺 A 族字段」窗经排查**不是采集缺口，是两代字段命名并存**：这批窗发 `frame_interval_p50/p95/p99_ms` + `avg_frame_interval_ms`（旧代），其余窗发 `interval_*`（现口径）。本工具按 thresholds 公式做别名映射后数字齐全；**真正仍缺的只有 `fps_wall`、`target_hz`**（旧代窗不发）与个别 native 轮的 `hitch_rate_per_min`。登记 P2：窗口 JSON 的 A 族字段契约需统一到 `interval_*` 现口径（含 fps_wall/target_hz 补齐），统一前以本表别名列为准。受影响窗：`kit/spin_min`、`ui_l1_spinner`、`ui_render_base_perfsoak`、`ui_render_jumprope`、`ui_render_particle`、`ui_render_pelican`（6 窗 × 2 后端）。

## P2 移交清单（P1.7 只登记不修）

1. A 族字段契约两代并存（§4 清单）→ P2 统一后本表别名列退役。
2. c3_list_scroll（fps+p95+hitch 三红）、r7_virtlist（p95+hitch 双红）、r7b_scroll_reuse（p95 临界 22.42）→ P2 性能修复，修复后按本表 + p99 回归带 8ms 复测。
3. 6 轮采不了档沿用 P1.4 登记（NO_JSON/BUILD_FAIL），修复归 P2 对应条目。
4. video_vr7_perf 缺 `interval_p99_ms`（视频极限窗，观测缺口）。

## 机器校验与复现

```bash
python3 docs/render_gpu_converge/baseline/perf/tools/extract_perf.py    # 重新生成 tsv
python3 docs/render_gpu_converge/baseline/perf/tools/verify_perf.py     # 机器校验（非零退出=口径破坏）
```

校验断言（2026-10-29 实测 **ALL PASS** 幂等）：366 行齐、首帧 0 FAIL、A 族 FAIL 轮全部 `p14_status=FAIL`（零 MISMATCH）、别名列只在旧代窗出现、p99 基线列完整。
