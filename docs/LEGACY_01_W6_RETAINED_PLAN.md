# W6 整波收口：默认切 retained

> 顺序 1，本批主线。`ENGINE_UI_WIDGET_RENDER.md` §5 W6 当前 🔄。

## 问题

窗口默认是全量重画（`full_paint`），每帧整棵树画一遍。实测证据：m5 窗 `damage_ratio=1`、`paint_count` 与帧数齐平，CPU 的 40 多个点基本是这个固定成本。
`retained` 只画脏区、静态复用，是 CPU 降下来的唯一正路。现在 W6 缺：默认切换、C9/C11 两个组合窗、C0–C5 回归。

## 验收

- 默认 `retained` 后全窗集行为一致：Golden 差异 0、U21 像素断言按形态矩阵全过。
- C9（`ui_wr_c9_stress_cache`，60s）、C11（`ui_wr_c11_policy_switch`，15s）新建并 PASS，包名与 §3 一一对应。
- C0–C5 回归全绿；R14（60s）复跑确认缓存预算不断。
- CPU：以切换前后同机真窗对照定数，门禁写进各窗 README（不拍脑袋定阈值）。

## 方案步骤

1. 新会话先读本篇 + 真源 §2/§2.5/§3/§5，再执行。
2. 切默认 `SetPresentPolicy`，damage Present（LoadOpLoad）链路打通；全幅 Clear 与 CompositeOnly 并存的禁止项复查（§2.1）。
3. 逐窗适配跑通，先 C11（切换正确），再 C9（压缓存），`RUN_SECONDS` 按 §2.5（C9 60s，C11 15s）。
4. C0–C5 回归，有 FAIL 走 wr-debug 完整流程（不停下报告节点不许跳过）。

## 会话边界

新会话执行；GPU 真窗（1200×800）；C9 ≥60s 按 soak 口径采 RSS slope。

## 优化收敛

收尾做一遍：热点 profile（整形/录制/提交占比）、内存（atlas/缓存 slope）、冗余（新旧两套 present 路径去重）、可读性（注释只写非显然约束）。

## 原文档同步清单

- §5 W6 🔄→✅；§3 C9/C11 补状态；§10 加一行；公开 API 有增减才同步 `RENDER_API_CATALOG.md` 并跑 `go run ./scripts/apidoc`。
