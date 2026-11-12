# O3 60s RSS 判据口径复核（2026-11-12）

> 输入：O1 的原始 55s/300s 序列（未开 pprof 的 55s 轮用于校准；300s 轮的 pprof
> 强制 GC 会扰动末段 RSS，只取 60–120s 段）。判据线沿用 P0.1：30000 KB/min。
> 目标：区分「Go 堆高水位启动爬坡」与「持续泄漏」，且不把爬坡误判成泄漏、不把
> 真泄漏洗成绿。

## 0 直话

**60s 单窗口的 RSS 斜率无法区分这两类**：m4 的启动爬坡和 l1 的真泄漏在 55s 内
都是 RSS 30–44MB/min、活堆 12–16MB/min，形状几乎一样。差异只在 **120s 后**才出现：
m4 走平、l1（修前）继续涨。因此「60s 二态判据」在原理上做不到既不放跑又不误伤；
标准对齐的落法是：60s 只做筛查（INCONCLUSIVE），正式判据用 **60s 预热 + 60s 测量
= 120s**。

## 1 原始数据 (KB/min，未开 pprof 的 55s 轮；300s 轮取 60–120s 段)

| run | 类型 | 当前(15s 预热) | 末 15s | 末 30s | 前/后半 | 活堆 | 60–120s |
|---|---|---:|---:|---:|---:|---:|---:|
| l1_spinner_native (55s, 修前) | **真泄漏** | 43895 | 20251 | 43535 | 40477/28731 | 12428 | — |
| m4_virtual_go (54s, 修前) | 启动爬坡 | 37998 | 30061 | 37219 | 42773/35175 | 13622 | — |
| m4_virtual_native (54s, 修前) | 启动爬坡 | 30533 | 7827 | 31927 | 20753/23284 | 16141 | — |
| fix1_l1_native_55s | 修复后 | 1445 | 320 | 1817 | 594/1723 | 0 | — |
| fix1_l1_go_55s | 修复后 | 555 | 63 | 330 | 1001/229 | 668 | — |
| fix1_m4_go_55s | 启动爬坡 | 34430 | 51158 | 33207 | 44126/38258 | 13383 | — |
| l1_native_300s (修前) | 真泄漏 | 25204* | 7911* | -124k* | 26656/28223 | 13270 | **32 MB/min** |
| fix1_l1_native_300s | 修复后 | -96 | 726 | 365 | 131/-555 | 0 | **2 MB/min** |
| fix1_m4_go_300s | 启动后平台 | 1574 | 30698* | 23695* | -2210*/-4787* | 1949 | **26 MB/min** |

\* 300s 轮带 pprof，60/180/280s 的强制 GC 会扰动末段；只信 60–120s 段（无 GC 点）。

- **60s 内无可分性**：真泄漏与爬坡的 RSS/活堆斜率区间重叠；末 15s/末 30s/两段衰减
  三种变体都会在某个窗上误判（如 m4 native 末 15s 7827 会放跑，l1 末 15s 20251 也会
  放跑真泄漏）。
- **120s 可分**：60–120s 段——修前 l1 32MB/min（>线）、m4 26MB/min（<线）、修复后
  l1 2MB/min（<线）。c10 300s 参照 1164KB/min 同向。
- 活堆单独也不可分：真泄漏与爬坡都是 12–16MB/min；活堆只能证明「不是活对象爆涨」，
  不能单独当 60s 泄漏判据。
- 真泄漏的最终确证仍是 pprof 归因 + 2h RSS ≤5%；60s/120s 是筛查。

## 2 候选口径

| 方案 | 做法 | 优点 | 缺点 |
|---|---|---|---|
| **A（推荐）** | 60s 预热 + 60s 测量 = 120s 判据；60s 轮只出 `INCONCLUSIVE`（不是绿） | 数据可分；对齐 Flutter warmup+steady 与 P0.1「短窗不能下结论」；真泄漏不放跑 | 正式判据单轮 120s |
| B | 保留 60s 二态，改用末 15s/30s | 不改时长 | 数据上不可分：m4 native 末15s 7.8 放跑；l1 末15s 20.3 放跑真泄漏 |
| C | 60s 二态改判活堆（强制 GC 差） | 直接针对活对象 | 真泄漏 12MB/min < 30 线 → 漏报；且活堆在爬坡窗同样涨 12–16MB/min |
| D | 60s 只把 FAIL 降级为 WARN/INCONCLUSIVE | 最保守，不误伤 | 60s 再也判不了红，等于取消筛查 |

## 3 推荐落地（A）与验收

- `ui/scheduler.ProcessTracker`：`RSSSlopeSteadyKBPerMin` 的 verdict 预热改为
  `max(elapsed/4, 60s)`，并要求测量段 ≥60s；60s 轮返回 `conclusive=false`
  （新增只读结论字段，不把 INCONCLUSIVE 写成 valid/PASS）。
- `examples/wrgate.GateOptions.MinRSSVerdictSec` 默认 60→**120**；60s 轮输出
  `rss_verdict=inconclusive`（保留 raw slope 只观测），不 FAIL、不冒充 PASS。
- P0.1 thresholds §5 同步：正式判据 ≥120s（60 预热 + 60 测量）；60s 仅筛查。
- 先红后绿用例（合成序列）：
  1. 爬坡序列（前 10s 冲高后走平）60s → 不得 FAIL，结论 INCONCLUSIVE；
  2. 同序列 120s → PASS（测量段走平）；
  3. 恒定 40MB/min 序列 120s → FAIL；
  4. 恒定 40MB/min 序列 60s → INCONCLUSIVE（不冒充 PASS）。
- 真窗复跑：l1/m4 各 120s（60 预热+60 测量）双后端，要求 l1 修复后 PASS、
  m4 PASS；修前 l1 数据 60–120s 32MB/min 作为「本判据能抓真泄漏」的对照（已有）。
- 负向：把测量段注入 40MB/min 分配 → FAIL；去掉注入 → PASS。

## 4 待拍板（停下报告）

- 选 A（120s 判据 + 60s INCONCLUSIVE）还是 B/C/D？
- 若选 A：`MinRSSVerdictSec` 默认改 120 属于 P0.1 真源修订，需用户确认；
  窗口默认 `RUN_SECONDS` 是否同步从 60 改 120（或仅门禁集窗改）也需定。

## 5 落地与验收（2026-11-12，A 口径，用户拍板）

### 实现

- `ui/scheduler/proc_tracker.go`：verdict 预热 `max(elapsed/4, 60s)`，测量段要求 ≥60s；
  60s/更短轮 `RSSSlopeSteadyKBPerMin` 返回 `valid=false`（INCONCLUSIVE），原始
  `rss_slope_kb_per_min` 仍保留只观测。
- `examples/wrgate/report.go`：`DefaultMinRSSVerdictSec` 60→**120**；新增三态
  `rss_verdict`（`pass` / `fail` / `inconclusive`，omitempty），`rssSlopeVerdict`
  与 `BuildReport` 共用同一判定；60s 轮只出 `inconclusive`，既不 FAIL 也不冒充 PASS。
- 真源文档：`baseline/thresholds.md` §5 + §9、`ENGINE_UI_WIDGET_RENDER.md` §2.2.4、
  `ENGINE_UI_RENDER_BASE.md` M-RSS-SLOPE 口径与说明、`baseline/windows.md` 修订行。

### 先红后绿（改常量前先红）

| 用例 | 修前 | 修后 |
|---|---|---|
| `ui/scheduler.TestRSSSlopeSteady_120sBoundary`（120 有效 / 119、60 无效） | 红：60/119 valid=true、warm=15/30 | 绿：warm=60 |
| `TestRSSSlopeSteady_60sInconclusive`（平/漏 60s 均不得出结论） | 红：60s valid=true | 绿：valid=false |
| `TestRSSSlopeSteady_120sRampFolds` | 红：warm=30≠60 | 绿：折叠爬坡 |
| `TestRSSSlopeSteady_120sLeakKeepsTrueSlope` / `_120sMidRunLeakCaught` | 改用 120s 口径 | 绿 |
| `examples/wrgate.TestEvaluateGates_RSSSteadySlope`（120 fail / 60 inconclusive / pass / 显式阈值） | 60s 过线会 FAIL | 绿：60s 不判、`rss_verdict=inconclusive` |

### 真窗 120s 验收（`RUN_SECONDS=120`，单窗自关，串行）

| 窗/后端 | steady KB/min | valid | warmup | rss_verdict |
|---|---:|---|---:|---|
| `ui_l1_spinner` native | **188** | true | 60s | pass（由字段判读） |
| `ui_l1_spinner` go | **370** | true | 60s | pass |
| `ui_text_m4_virtual` go | **21200** | true | 60s | **pass** |
| `ui_text_m4_virtual` native | **21344** | true | 60s | **pass** |

- 60s 轮对照：l1 `valid=false`（raw 304 KB/min）；m4 `rss_verdict=inconclusive`、rc=0、
  stderr 无 `FAIL: rss_slope` —— 不再误伤启动爬坡。
- 负向：单元 120s 过线 → FAIL；60s 过线 → 不 FAIL；真实修前 l1 300s 的
  60–120s 段 32MB/min 作为「120s 判据能抓真泄漏」的实测对照。
- 门禁：`go test ./ui/scheduler ./examples/wrgate` 过、vet 过、
  windows/amd64 + darwin/amd64 + linux/arm64 + nogpu 编译过；
  `ui/scheduler` 的 pacing 计时测试在并行交叉编译负载下曾波动一次，隔离 5/5 过，
  与本判据改动无关（既有计时门噪声）。
