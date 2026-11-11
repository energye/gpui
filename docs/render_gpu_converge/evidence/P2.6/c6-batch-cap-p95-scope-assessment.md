# C6 评估：ui_base_batch_cap 全程 p95 超 22ms 门禁——采样域错配，非引擎性能洞（2026-11-11）

> 任务：§2.2 待办池 C6（`ui_base_batch_cap` p95 双后端 55.7/62.1ms 超 22ms 门禁，P1.4 同因同量级，拟归属 P2 性能线）。
> 性质：排查为主，本会话零生产代码改动。停下报告待拍板修法方向。
> 判据纪律：全程逐值/逐像素，不用 mean。

## 1 现象（四组独立数据同构）

| 来源 | native p95 | go p95 | 门禁 |
|---|---|---|---|
| P1.4（2026-10-29） | ≈53.1 | ≈63.2 | FAIL（>22） |
| P1.6（2026-10-29，`baseline/vram/summary.md:23-24`） | 55.7 | 62.1 | FAIL |
| P1.7（`baseline/perf/perf_baseline.tsv:110-111`） | 53.11 | 63.15 | FAIL |
| **本会话复现（2026-11-11，现行树）** | **52.98** | **59.89** | FAIL |

复现命令（显示环境勘误 2026-11-11：两轮原意图隔离 rootless Xwayland `DISPLAY=:3`，实际因 `WAYLAND_DISPLAY` 在环境被平台层优先、跑在会话 Wayland 上（JSON `backend=wayland`）；结论不受影响——末 256 采样环机制与显示路径无关，且数值合证（§4）在单轮内自洽；勘误出处 `evidence/P2.6/e1-afamily-overrun-assessment.md` §5。单窗串行，gate 模式 200s 窗内自 Quit=标准关闭，未硬杀）：

```
go build -o /tmp/c6/batch_cap ./examples/ui_base_batch_cap
DISPLAY=:3 LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so \
  RUN_SECONDS=200 /tmp/c6/batch_cap -auto-only     # native
DISPLAY=:3 GPUI_BACKEND=go RUN_SECONDS=200 /tmp/c6/batch_cap -auto-only   # go
```

原始输出：`/tmp/c6/{native,go}.{json,stderr}`（会话产物；正式归档待拍板后随收口批落 `evidence/P2.6/c6/`）。

## 2 逐级数据（本会话，native / go）

| 档 | native fps/p95 | go fps/p95 | 窗内健康线（fps≥55 且 p95≤22） |
|---|---|---|---|
| prims 1k | 59.9 / 18.5 ✅ | 59.4 / 19.4 ✅ | 过 |
| prims 5k | 58.2 / 21.2 ✅ | 56.4 / 26.1 ❌ | go 本轮临界红（基线 19.2 ✅） |
| prims 10k | 56.4 / 24.4 ❌ | 56.6 / 23.5 ❌ | 双端临界红（fps 达标） |
| vector 100 | 59.5 / 20.6 ✅ | 59.5 / 17.9 ✅ | 过（P1.6 native 22.7 ❌ 本轮不复现） |
| vector 300 | 36.3 / 33.0 ❌ | 34.9 / 34.1 ❌ | 刻意超载档 |
| vector 600 | 22.1 / 53.0 ❌ | 19.8 / 59.1 ❌ | 刻意超载档 |

地板门（窗内真门禁）：`max_prims≥1000`、`max_vector≥100` 双后端本轮全过；本轮 rc=1 全部来自金图项（见 §5）。

## 3 机制定位（file:line）

1. **百分位采样域 = 末 256 个帧间隔**：`ui/scheduler/metrics.go:24` `intervalRingCap = 256`，`:358` 逐帧写环，`:845-858` 百分位只读环内样本。报告 `interval_p50/p95/p99_ms`（`examples/wrgate/report.go:33-35`）取自该环——**不是全程分布**。
2. **gate 模式收尾停在刻意超载档**：`examples/ui_base_batch_cap/main.go:420-437`——测量梯最后一个动作是冻结在 vector-600（600 路径 × 24 段 cubic，窗设计目的就是量此处天花板）并出快照，然后 Quit。末 256 样本 ≈ vector-600 档（20fps→50ms 间隔）+ vector-300 尾部（35fps→28.6ms）+ 快照相位。
3. **A 族门禁套错窗型**：`baseline/windows.md:210` 把 batch_cap 标为运行档 `A/200s`，采集 runner 据此套 `baseline/thresholds.md:87` 的 A 族硬门禁 `interval_p95_ms ≤ 22`；但 thresholds.md §3（:82）A 族适用域写明是「R6、R7、R7b、C3、C5、C10 等**动画/滚动/持续 tick 窗**」——容量阶梯窗不在其列。窗自身 JSON 早已声明口径：`"hitch_scope": "ladder includes overload rungs; per-rung pass_60fps is the capacity gate"`（main.go:538）。

## 4 合证（报告百分位 = 超载档自身百分位）

| 量 | 报告值（末 256 环） | 对应档实测 | 结论 |
|---|---|---|---|
| native p95 | 52.98 | vector-600 档内 p95 **53.0**（窗内 collector 独立采样） | 逐值吻合 |
| go p95 | 59.89 | vector-600 档内 p95 **59.1** | 吻合（环含快照相位微差） |
| native p50 | 43.82 | vector-600 avg 间隔 45.2ms 与 vector-300 28.6ms 混合中位 | 吻合 |
| avg | 20.61/21.00 | 全程均值（9023/8857 帧-majority 是 55-60fps 档） | 自洽 |

p50(43.8) > avg(20.6) 的"矛盾"正是环采样域的直接推论：过半环内样本来自超载档，而均值被 4/6 的健康档拉低。**若 p95 是全程百分位，p50 不可能大于 avg。**

结论：22ms 门禁 FAIL 量到的是「vector-600 档的天花板 = 20fps」，这正是窗的数据产品，不是缺陷。**三轮基线 + 本会话复现四组同构，非回归。**

## 5 顺带发现（本会话，归位不混入 C6 结论）

1. **prims 金图漂移 0.1667%（双后端同值）**：`testdata/batch_cap_prims_base.png` vs 本轮快照，掩码内 diff 1472px/883200。形态：x=0 与 x=1199 两条全高边缘列 1-LSB（如 `0e1016`→`0e1116`）+ 数处 2px 竖缝（x=195/284/290…，各 11 行）。与 C1 收口登记「22 窗金图漂移专项」（`evidence/P2.6/c1-screen-grab-black-assessment.md` §9）同族（历史金图 vs 当前渲染的边缘/AA 漂移）。**归属该专项，C6 不修、禁止直接重种掩盖。** vector 金图 0.0000% 未漂。本轮跑窗覆写并已 `git checkout` 还原两个被跟踪快照（`batch_cap_{prims,vector}.png`），漂移样本留档 `/tmp/c6/prims_snap_drifted.png`。
2. **贴线档逐轮抖动**：prims-5k/10k、vector-100 的 p95 在 17.9–27.6 间跨轮波动（P1.6 vs 本轮逐值见 §2），跨后端都有，vsync 16.7ms 底上贴线档固有抖动；窗内地板门（1k/100）双后端稳定过。属容量数据本身，无缺陷证据；如需收敛须先归因（另立项，不在 C6）。
3. **:3 显示生命周期**：按需 rootless Xwayland 无客户端自退（`xdpyinfo` 显示 DOWN 属正常），两轮间自动重生，会话桌面 Wayland 无恙，无 Xorg 崩溃征兆。

## 6 与 E1 的区分

E1（c3_list_scroll / r7_virtlist / r7b）是**均匀负载滚动窗**，末 256 环 ≈ 全程分布，其 p95 22-24ms 是真实稳态尾部——E1 仍是真性能线。C6 是**相位结构窗 + 采样域**问题。两者不同因，不得并批。

## 7 修法方向（停下报告待拍板）

- **方向 A（门禁口径归位，零引擎代码）**：thresholds.md §3 适用域补一句「容量阶梯窗（`ui_base_batch_cap`）不在 A 族全程 p95 门禁域，判定=窗内逐级健康线（地板门已内置）」；`baseline/windows.md:210` 按「只增不改」在 §7 修订表追加勘误行（B1 先例）；台账 C6 销账 ✅。P1.4/P1.6/P1.7 冻结基线不改。
- **方向 B（A + 指标契约补强，小代码）**：A 全部 + 报告 JSON 百分位字段补 provenance 声明（metrics.go 暴露环覆盖时长/样本数，或 report.go 注明「末 256 采样」），让后续消费者不再误读。触及 `ui/scheduler` 公开 JSON 语义（A2「字段统一一次切」先例），需 catalog/apidoc 同步。
- **方向 C（性能优化，不建议）**：把 vector-300/600 提到 60fps = 改测量结果而非修洞（天花板就是数据）；prims 贴线档抖动无缺陷证据。若拍板要做须另立专项先归因。

## 8 停下报告与拍板

C6 判定为**门禁口径错配（采样域），非引擎性能洞**；窗自身门禁（地板+逐级健康线）设计完好且双后端通过。

**拍板（2026-11-11，用户）：方向 A——门禁口径归位，零代码。** 已执行：

1. `baseline/thresholds.md` §3 适用域补「不适用容量阶梯窗」+ §9 修订表追加 C6 行。
2. `baseline/windows.md` §7 修订表追加 C6 勘误行（表内条目按「只增不改」不动）。
3. 台账 C6 → ✅（§2.1 载体 / §2.2 行 / §7 流水）。

方向 B（percentile provenance 补强）未做，留待后续需要时另立；方向 C 否决（改测量结果非修洞）。
