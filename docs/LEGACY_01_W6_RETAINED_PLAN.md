# W6 整波收口：默认切 retained（已关，2026-10-20）

> 顺序 1，本批主线。**现状：W6 ✅**。默认 `retained` 已切；C9/C11 新建并关闭；C0–C3 回归绿；R14 solo 60s 绿；最后一项 C4/C5 过期 Golden 已按文本线重定并回写。`ENGINE_UI_WIDGET_RENDER.md` §5 W6 = ✅。

## 问题

窗口默认是全量重画（`full_paint`），每帧整棵树画一遍；`retained` 只画脏区、静态复用，是 CPU 降下来的正路。W6 的缺口是默认切换、C9/C11 两个组合窗、C0–C5 回归。默认切换与 C9/C11 已在 2026-09-04 完成；最后剩 C4/C5 的 Golden 基线停更。

## 验收与关闭证据

- 默认 `retained`（`NewPipelineApp`；首帧/改尺寸仍全幅 Clear，稳态 CompositeOnly + damage present）；正确性窗显式钉 `full_paint` 原姿态。
- C9 `ui_wr_c9_stress_cache` 60s 三连跑：entries 触顶 128、evictions 687 单调、bind 16–17/800、rr 375、CPU 36.8–38.9%、RSS slope 17–24k ≤30k、像素 3/3、Golden 296292px 逐位 0。
- C11 `ui_wr_c11_policy_switch` 15s 两连跑：switches=2、retained 相 dmg 0.138 < full 相 1.0、两态 mode 均见、像素 2/2、Golden 390176px 逐位 0。
- C0–C3 回归 ✅；R14 solo 60s ✅（此前 batch 跑 slope 超预算系负载抖动，Golden 系 `/tmp` 陈旧基线，删基线重跑绿）。
- **C4/C5 Golden 重定（2026-10-20）**：旧基线 2026-08-24/25；C4 静态掩码差 4–5%，C5 旧基线与新渲染全图仅 536px 差；均为 09-03/04 文本线后的文本栅格漂移，不是默认 retained 切换或引擎行为变化，**不改引擎代码**。
  - C4：`C4_SNAP_DIR=/tmp/c4_rebase` 首跑 25s 产新 `c4_steady_base`/`c4_recover_base`，次跑 `0.0000%/370680px`×2；回写 `examples/ui_wr_c4_shell_overlay/golden/` 后默认目录再跑 25s，仍 `0.0000%`×2、pixel 10/10、shell_rerecord_scroll=0、main_dirty_excess_frames=0、fps_interval 57.90、p95 18.11ms、hitch 4.81/min、presents=1447。实际只更新两个 `_base`；当前三张快照已与新渲染一致。
  - C5：`C5_SNAP_DIR=/tmp/c5_rebase GPUI_ACCEPT_GOLDEN=1` 首跑 30s 产新 `c5_final_base`，次跑 `0.0000%/409160px`；回写 `examples/ui_wr_c5_anim_over_static/golden/` 后默认目录再跑 30s，仍 `0.0000%`、pixel 6/6、fps_interval 59.95、p95 17.92ms、hitch 2.00/min、paint drift 0、boundary_skip 89090/rr368。`c5_final.png` 为每跑生成物，不提交。
- 所有 C4/C5 跑均 `exit=0`、`present_policy=retained`、`vsync_source=true`、`gpu_fallbacks=0`。`present_mode=full`/`damage_ratio=1` 是该场景全幅损伤的真实观测，不冒充局部脏区。

## 关键口径

- **只重定基线，不重开 C4/C5 能力**：两窗 §3 状态维持 ✅；本次补的是 Golden 证据，不是新能力。
- **baseline 更新方式**：C4 自动种子只在临时目录进行，回写后必须再用默认目录复验零差；C5 的种子必须显式 `GPUI_ACCEPT_GOLDEN=1`，再回写、再复验，禁止“首跑自动绿”。
- **RSS**：C4 25s/C5 30s 的正确性/组合窗仍受启动爬坡影响（C4 slope≈102k、C5≈34k），不作为泄漏结论；泄漏归 R15/C10 300s soak。C4 README 的短窗口径保留。

## 优化收敛

已做：C4 用同一次批量快照（steady/open/recover）控制读回停顿；C5 只保留最终帧证据；两窗都复用同机默认目录做回写后的独立复验；基线只改必要文件，不把每跑生成物入库。

## 原文档同步清单

- `ENGINE_UI_WIDGET_RENDER.md` §5 W6 🔄→✅、§10 加 W6 收口行；C4/C5 §3 状态不变。
- `RENDER_FEATURE_COVERAGE.md` §5 文档口径 W6 已关闭、§11 加行。
- `LEGACY_00_M0_M5_BATCH.md` 顺序 1 状态更新；`LEGACY_04_R_WINDOWS_PLAN.md` 移除“W6 未关”尾注。
- `RETAINED_DEFAULT_AUDIT.md` 顶部加 2026-10-20 更新说明，A7 的 C4/C5 过期项标注已重定。
- C4/C5 README 各加“2026-10-20 Golden 重定基线”记录。
