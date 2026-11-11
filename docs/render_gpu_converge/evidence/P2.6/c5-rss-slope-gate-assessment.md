# C5：10 条 PASS 记录 RSS 稳态斜率过 30000KB/min 泄漏线 —— 排查评估（2026-11-11）

> 台账 §2.2 C5：`baseline/vram/summary.md §3` 登记「10 条 PASS 记录过泄漏线，P2 判定
> 窗门禁集是否含 RSS 线还是漏判」。本文件只做排查与方向设计，**未动生产代码**。

## 0 结论（直话）

P0.1 阈值真源写了线，但**统一门禁集根本没接这条线**：`wrgate.EvaluateGates` /
`GateOptions` 里没有任何 RSS 斜率判定；只有极少数窗各自手写，且短跑会跳过。
10 条 PASS 记录经核实全部 `rss_slope_valid=true`、`judge=LEAK-LINE`，按口径本应 FAIL——
是门禁覆盖面缺口，不是「这些窗没漏」。

## 1 证据与实测数据

### 1.1 P0.1 阈值真源

`docs/render_gpu_converge/baseline/thresholds.md:122`：

- `rss_slope_steady_kb_per_min > 30000 KB/min` → FAIL；
- **只在 `rss_slope_valid=true`（≥30s，推荐 60s）时判**；
- `rss_slope_valid=false` 的短窗「不能下结论，不是没漏」，只观测；
- 回归带 `≤2000 KB/min` 是治理值，非产品 SLO（`ui/scheduler/baseline.go`）。

### 1.2 P1.6 盘点（baseline/vram/summary.md §3）

- `rss_settle.tsv` 223 行：within-band 8 / over-2k-band 20 / **LEAK-LINE（>30000）14**。
- 14 条里 `ui_text_m5_anycase`×2 在 P1.4 已是 FAIL；**10 条 PASS 过线**：

| 窗/后端 | 稳态斜率 KB/min |
|---|---:|
| ui_wr_r17_bg_throttle go / native | 88801 / 62534 |
| ui_text_m4_virtual go | 49624 |
| ui_wr_c5_anim_over_static native / go | 47890 / 38694 |
| ui_wr_r6_layer_anim native / go | 42120 / 41843 |
| ui_wr_r10_async_image native / go | 41906 / 32255 |
| ui_l1_spinner native | 30180 |

### 1.3 复核 rss_settle.tsv（不凭文档）

- 上述 10 条逐条核对：**全部 `slope_valid=True`、`judge=LEAK-LINE`**，即按 §1.1 口径本应判 FAIL。
- 全表 `slope_valid` 分布：PASS/False 107、FAIL/False 74、PASS/True 32、FAIL/True 10。
  大量短窗过 30000 但 `valid=False` → 按口径「不能下结论」，不算漏判（口径正确，不是洞）。
- 过线且 `valid=True` 的共 14 条：10 条 PASS（漏判）+ 2 条 m5_anycase FAIL + 2 条 r1_layout FAIL（基线本就红）。

### 1.4 根因定位（file:line）

1. `examples/wrgate/report.go:335-490`：`GateOptions` 无 RSS 字段，`EvaluateGates` 无
   `rss_slope_steady` 判定；`Report` 结构体（:60-64）与 schema keys（:308）只是**采集**字段，
   门禁集不判。→ 统一门禁集未含 RSS 线（C5 登记的猜测成立）。
2. 少数窗各自手写：`examples/ui_wr_r10_async_image/main.go:60` `slopeBudget=30000`、
   :519 `fullDuration := gateMode || manualSeconds==0 || manualSeconds>=closeSeconds`、
   :520-556 只在 `fullDuration` 时把 `rss_slope_steady_le_budget` 加入探针并判定。
   短跑（RUN_SECONDS < closeSeconds）跳过 → P1.4 采集里 r10 的 PASS 走的是这条跳过路径。
3. 其余 5 个漏判窗（`ui_wr_r17_bg_throttle` / `ui_text_m4_virtual` /
   `ui_wr_c5_anim_over_static` / `ui_wr_r6_layer_anim` / `ui_l1_spinner`）源码里
   `rss_slope_steady` / `30000` 零命中 → 完全没有判定。
4. 被抓到的 `ui_text_m5_anycase` 源码同样零命中 → 那 2 条 FAIL 来自采集侧
   （`baseline/vram/tools/verify_vram.py` / runner 的基线核对），不是窗自身门禁。
   ⇒ 判定散在「采集脚本 / 个别窗」，没有单一真源。

## 2 标准方案（对齐 Skia/Flutter 与本仓 P0.1）

- 门禁单一真源：`wrgate.GateOptions` 加 `MaxRSSSlopeSteadyKBPerMin`（默认 30000），
  `EvaluateGates` 里统一判定 `r.RSSSlopeValid && r.RSSSlopeSteadyKBPerMin > max → FAIL`，
  与 fps / p95 / pool / boundary 等门禁同处，禁止各窗复制判定（收敛纪律）。
- 短窗口径保持：`RSSSlopeValid=false` 不判（符合 thresholds.md「不是没漏，只观测」），
  由门禁集统一保证，不再靠各窗自己的 `fullDuration` 判断。
- 各窗只需在 `EvaluateGates` 调用处按场景启用（或默认启用）。

## 3 影响面与关键风险

- 若统一门禁默认开启，P1.4 的 10 条 PASS 会转红。但**不能直接判它们为真泄漏**：
  `rss_slope_valid=true` 的 30s+ 稳态斜率 88801 KB/min ≈ 1.45 MB/s，对动画/文本窗
  可能是 Go 堆与预热缓存爬坡（summary.md §3 已定性「RSS 关窗不回落是运行时常态，
  不能当泄漏判据」）。按「动手修门槛」与「禁止猜着修」，必须先复跑定性。
- 复跑需 ≥30s（推荐 60s）单窗自退出、串行、每轮查显示健康（GPU 真窗安全口径）；
  10 窗 × 30–60s ≈ 5–10 分钟。
- 与 C1 余留的「22 窗金图漂移」是两条独立线，不要混批。

## 4 待拍板（停下报告）

- D1 门禁落点：统一进 `wrgate.EvaluateGates`（推荐）/ 各窗各自补判定。
- D2 默认行为：默认开启（10 条 PASS 可能转红）/ 默认关闭 + 按窗 opt-in / 先复跑定性再定。
- D3 是否先做 10 条 ≥30s 复跑，定性「真泄漏 vs 预热爬坡」，再决定门禁默认值与是否
  需要调 P0.1 的 30000 线（调阈值属改真源，需单独拍板，不在本批顺手改）。

证据：`docs/render_gpu_converge/baseline/vram/rss_settle.tsv`、
`docs/render_gpu_converge/baseline/vram/summary.md §3`、
`docs/render_gpu_converge/baseline/thresholds.md:118-124`、
`examples/wrgate/report.go`、`examples/ui_wr_r10_async_image/main.go`。

## 5 复跑定性（2026-11-11，用户拍板「先 10 条 ≥30s 复跑再定」）

串行单窗自退出、每轮查显示健康；`RUN_SECONDS=35`（r17/r10 带 `-auto-only`，
其内部固定 30s 阶段脚本），go 档用 `GPUI_BACKEND=go`。全部记录 `rss_slope_valid=true`。

| 窗/后端 | P1.4 历史 | 本轮 35s 稳态 | 是否仍过线 | 60s 复跑稳态 |
|---|---:|---:|---|---:|
| r17_bg_throttle native | 62534 | 27843.9 | 否 | — |
| r17_bg_throttle go | 88801 | **-9921.6** | 否 | — |
| text_m4_virtual go | 49624 | 31985.1 | 是 | **28586.8** |
| c5_anim_over_static native | 47890 | 22537.2 | 否 | — |
| c5_anim_over_static go | 38694 | 21876.6 | 否 | — |
| r6_layer_anim native | 42120 | 16455.9 | 否 | — |
| r6_layer_anim go | 41843 | 25006.2 | 否 | — |
| r10_async_image native | 41906 | 9693.9 | 否 | — |
| r10_async_image go | 32255 | 24099.3 | 否 | — |
| l1_spinner native | 30180 | 46936.0 | 是 | 35161.0 |

- **9/10 条在当前树不复现历史过线**（r17 go 甚至出现负斜率——BG 最小化后 RSS 回落，
  最小二乘拟合为负）。m4_virtual go 在 60s 复跑也降到 28586.8。⇒ 历史 10 条的高斜率
  主要是**短窗口径 + 启动/预热爬坡**，不是持续泄漏；按 P0.1 口径「RSS 关窗不回落是常态」
  （summary.md §3）也印证。
- **唯一持续过线**：`ui_l1_spinner` native 60s 稳态 35161 KB/min，RSS 从 12MB 涨到 302MB。
  形态是持续爬坡，但 60s 仍过 30000；它是 480×320 小窗、retained 策略，是否真泄漏需
  更长 soak（P0.1 真源另有「2 小时 RSS 涨 ≤5%」档）单独定性，不能在 30s 窗口下直接判死。
- **口径风险（待拍板）**：`ui/scheduler/proc_tracker.go` 的 `RSSSlopeValid` 只需 30s
  （warmup 15s + 稳态 15s），30–35s 窗口会把预热爬坡误判为泄漏。P0.1 thresholds.md:122
  写「≥30s，**推荐 60s**」，两者不一致。门禁默认值不能直接照搬「默认开启」而不动
  valid 口径，否则 l1_spinner 与短跑窗会红。

证据：`evidence/P2.6/c5-recheck/recheck.tsv`、`*.log`（35s）、`60s_*.log`（60s）。

## 6 实现与验收（2026-11-11，用户拍板 D1=统一 wrgate / D2=默认开启+≥60s / l1_spinner 另立 soak）

### 实现（单一真源 + 5 窗薄接线）

- `examples/wrgate/report.go`：
  - `GateOptions` 加 `MaxRSSSlopeSteadyKBPerMin`（默认 30000）与 `MinRSSVerdictSec`（默认 60）；
    零值=用默认，即**门禁默认开启**，覆盖 139 个调用 `EvaluateGates` 的窗。
  - `rssSlopeVerdict(r, maxSlope, minVerdict)` 为唯一判定实现：
    `RSSSlopeValid && ElapsedSec >= minVerdict && steady > maxSlope → FAIL`；
    `EvaluateGates` 与导出的 `CheckRSSSlopeSteady(r)` 共用它，不复制逻辑。
  - 短窗（`valid=false` 或 `elapsed<60`）只观测不判（P0.1 thresholds.md:122「不是没漏」口径）。
- 排查发现：10 条漏判对应的 6 个老窗**没有一个调用 EvaluateGates**（只有 m4_virtual/c5/r6/r17/r10
  用 wrgate.BuildReport 输出 JSON，l1_spinner 是裸 metrics JSON）。故给前 5 窗在 `fmt.Println(string(raw))`
  后各加一行 `wrgate.CheckRSSSlopeSteady(report)` 薄接线；l1_spinner 按用户拍板**不接门禁**，
  另立 2h soak 定性子项（阈值真源另有「2 小时 RSS 涨 ≤5%」档）。
- 阈值真源 30000 与 `proc_tracker.go` 的 30s valid 门槛都**未改**（按纪律不动真源，只在门禁层加时长门槛）。

### 验收（三证据齐）

| 项 | 结果 |
|---|---|
| 逻辑探针/单测 | `examples/wrgate/report_test.go` 新增 5 case：60s+超线红、35s+超线不判、valid=false 不判、低于线绿、显式阈值生效；`go test ./examples/wrgate` PASS |
| 真窗正向（短窗口径） | m4_virtual go 35s：steady=37597.7 > 30000 但 elapsed=35.5<60 → rc=0、无 RSS FAIL（`gate_after_35s_m4_go.log`）——证明不误判预热爬坡 |
| 真窗负向（门禁生效） | m4_virtual go 60s：steady=32694.3 > 30000、elapsed=60.5 → rc=1，stderr `FAIL: rss_slope_steady_kb_per_min=32694.3 > 30000 (verdict window 60.5s >= 60s)`（`gate_after_60s_m4_go_v2.log`）。修前同窗 60s rc=0（门禁未覆盖）→ 修后红，证明接线生效 |
| 全量 | `go build ./...`（139 调用点）、`go vet ./examples/wrgate` rc=0；10 条复跑数据见 §5 / `c5-recheck/recheck.tsv` |

### 余留登记（不掩盖）

- **m4_virtual go 60s 稳态斜率波动大**：三次 60s 复跑 28586.8（绿）/ 37791.2 / 32694.3（红）。新门禁下会时红时绿；与 l1_spinner 一并归「2h soak 定性」子项，不在本批下结论。
- **l1_spinner 2h soak 子项**：唯一 60s 仍过线（35161），RSS 12MB→302MB；按用户拍板单独立项，需 2h 档判真泄漏 vs 缓存上限。
- 22 窗金图漂移（C1 余留）与 C5 是两条独立线，未混批。
