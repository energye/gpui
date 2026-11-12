# O5 静态/idle 窗门禁回归（2026-11-12）

> 背景：O2 把 `present_count` 从「raster job 完成数」改为「真实 swapchain present」；
> O3 把 RSS 正式判据改为 120s。两者都可能让老窗门禁误红。本批抽静态/idle 与
> retained 代表窗复跑，确认门禁口径是否跟上了。
> 运行：单窗串行、`RUN_SECONDS` 自关、当前 Wayland 桌面；每轮前后无并发窗口。

## 1 抽检结果

| 窗 | 后端/时长 | rc | 关键判据 | 结论 |
|---|---|---:|---|---|
| `ui_base_idle` | native 60s `-auto-only` | 1 | `present_count=1`、warmup 后 0 次呈现、cpu 1.25%、RSS 探针过；**金图 0.8489%** | 门禁口径已修；仅剩既有金图漂移 |
| `ui_base_idle` | go 60s `-auto-only` | 1 | 同上；金图 0.8489%（go 同一基线） | 同上 |
| `ui_l1_blank` | native 10s | 0 | presents=4590（老 `embedder.New` 计数，不受 O2 影响） | 对照 PASS |
| `ui_l2_shell` | native 10s | 0 | presents=2（老 `embedder.New`，静态） | 对照 PASS |
| `ui_wr_c4_shell_overlay` | native 25s `-auto-only` | 1 | 像素探针全过；**金图 2.8912%** | 既有金图漂移 |
| `ui_wr_c8_hit_overlay_xf` | native 15s `-auto-only` | 1 | **金图 0.7967%** + snapshot 超时 | 既有金图漂移/快照超时 |
| `ui_wr_r4_composite` | native 15s `-auto-only` | 1 | **金图 2.5367%** | 既有金图漂移 |
| `ui_wr_c11_policy_switch` | native 15s | 1 | 策略/像素探针过；`hitch_rate_per_min=8.02>5` | 既有环境 hitch |
| `ui_base_multiwin_cap` | 8 窗 | 未跑 | 代码探针已按 O3 修正 | 8 窗批测需隔离显示，按 AGENTS 安全条另批 |

## 2 发现与修复

### 2.1 `present_count` 真计数：未造成误红

`ui_base_idle` 自己的核心断言 `presents_after_warmup ≤ 2` 在真计数下为 **0**
（`present_count=1` 是唯一 warm-up；`frame_count=0`），PASS。抽检集合里没有窗
因为「present 数变少」而误红。静态/空闲窗的 `MinPresents` 门禁只要求 ≥1。

### 2.2 O3 遗留：`rss_slope_valid` 硬探针（已修）

O3 把正式判据改为 120s 后，60s/75s 的 idle/multiwin 轮 `rss_slope_valid=false`
是**正常筛查态**，但三窗把它当硬失败：

| 窗 | 修前 | 修后 |
|---|---|---|
| `ui_base_idle`（60s） | `!RSSSlopeValid` → FAIL | 探针改 `rss_no_leak_or_inconclusive`：RSS 可用 + （无有效结论 或 稳态 ≤30000） |
| `ui_base_multiwin_cap`（75s） | `!RSSSlopeValid` → FAIL | 仅 RSS 不可用 FAIL；valid 且超线才 FAIL |
| `ui_base_batch_cap`（200s） | 同左（200s 本会 valid） | 同上，统一口径 |

- 依据：`baseline/thresholds.md` §5（2026-11-12 O3 修订）——60s/更短轮只筛查，
  `rss_verdict=inconclusive` 不是 FAIL。
- `ui_wr_ime_r3/r4/r5` 的 `!RSSSlopeValid` 分支是 WARN 不是 FAIL，未动。

### 2.3 既有环境红（非 O2/O3/O4）

pre-O2 `7a7fc15d` 干净树同窗复跑，金图/hitch 数值逐字相同：

| 窗 | 当前树 | pre-O2 `7a7fc15d` |
|---|---:|---:|
| `ui_base_idle` 金图 | 0.8489% | 0.8489% |
| `ui_wr_c4_shell_overlay` 金图 | 2.8912% | 2.8912% |
| `ui_wr_c8_hit_overlay_xf` 金图 | 0.7967% + snapshot 超时 | 0.7967% + snapshot 超时 |
| `ui_wr_r4_composite` 金图 | 2.5367% | 2.5367% |
| `ui_wr_c11_policy_switch` hitch | 8.02/min | 环境档（首跑无基线，无对照） |

→ 登记 O7（既有金图漂移/ hitch），按「22 窗金图漂移」同流程逐窗判
「基线过期 vs 渲染洞」，不混进 O5。

## 3 证据

- `ui_base_idle_native/`（修前/修后 stdio）、`ui_base_idle_go/`、`ui_l1_blank_native/`、
  `ui_l2_shell_native/`、`ui_wr_c4_native/`、`ui_wr_c8_native/`、`ui_wr_r4_native/`、
  `ui_wr_c11_native/`。
- pre-O2 对照日志在 `/tmp/o5-pre/`（临时 worktree，已清理；数值写入本文件）。
- 运行中写脏的 current snapshot（idle/c8/r4 的 testdata PNG）已还原，未混入提交。
