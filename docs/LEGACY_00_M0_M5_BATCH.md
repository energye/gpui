# M0–M5 遗留收尾批次总览

> 本批共 6 个未解决问题，各自独立成篇（见下表），**一次只做一件**，每件都在**新会话**里执行。
> 当前环境有 GPU 真窗，每件都必须真窗验证；每件收尾都要做一遍优化收敛（性能/内存/可读性/冗余正向优化）。
> 状态真源仍是 `docs/ENGINE_UI_WIDGET_RENDER.md`（§2/§3/§5/§10），本批文档只管“怎么做”，不管“绿没绿”。

## 顺序与依赖

| 顺序 | 文档 | 问题 | 前置 |
|---|---|---|---|
| 1 | `LEGACY_01_W6_RETAINED_PLAN.md` | W6 整波：默认切 retained，C9/C11 新建，C0–C5 回归 | 无，本批主线先做 |
| 2 | `LEGACY_02_SEGMENTTEXT_PLAN.md` | 单行 5 万字分段复用已落地（`segment_reuse.go`，约 24→14ms），余量以新 profile 为准 | 无，可与 1 并行排期但不同会话 |
| 3 | `LEGACY_03_GOLDEN_BASELINE_PLAN.md` | accept Golden 已按 M5 截图刷新归档（原差 1.48%，现 `baseline_m5.png`） | 无，但**必须独立评审**，禁顺手办 |
| 4 | `LEGACY_04_R_WINDOWS_PLAN.md` | R22 已关，R1/R15 两个单能力窗未关 | 无，建议按 R1→R15 从小到大做 |
| 5 | `LEGACY_05_C10_SOAK_PLAN.md` | C10 长跑组合窗已建未关（300s 可跑，第三跑金图差 1.728% 卡相位分界） | C10 收尾（R15 首跑已 PASS，次跑待补） |
| 6 | `LEGACY_06_M6_TREND_PLAN.md` | M6 验收主窗散表已有 M0–M2，正式 numbers 行待补 | 例行公事，每期收尾时执行 |

## 每件的固定结构

每篇都含：问题（现状/影响）→ 验收（单测+真窗+门禁）→ 方案步骤 → 会话边界 → 优化收敛 → 原文档同步清单。
做完一件就在 `ENGINE_UI_WIDGET_RENDER.md` §10 加一行，状态表里改状态。

## 明确不做的（本批不动）

R17（后置）、协同编辑/CRDT、RTL 特殊布局、IME 自绘候选窗。M3.5 说明：buffer 已合入，piece tree 仍为条件触发（Q3 未触发不引入）。
