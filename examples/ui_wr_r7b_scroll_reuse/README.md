# ui_wr_r7b_scroll_reuse — R7b 滚动少重录 cell（W3）

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=60 go run ./examples/ui_wr_r7b_scroll_reuse
```

## Window / Close duration

- 窗口 **1200×800**（U15）
- 关闭用时长 **60s**（§2.5；观察窗 120s）。`RUN_SECONDS` 未设时默认取 60；`RUN_SECONDS<5 → FAIL`（U16）
- GPU 真窗必需；无 GPU/X11 环境 → `FAIL: window open (needs_gpu_window)` + exit 1

## Visible effect

| Region | Expectation |
|--------|-------------|
| TopBar | `R7b 滚动少重录 — 静 cell 保持` 标题 |
| 左 Legend | 6 行图例：持续滚动相位、静 cell 回放、逐 tick 门禁公式、布局驱动说明 |
| Body 列表区 | 1200 项变高异构列表持续滚动 60s（Steady 慢滚 / Spike 快滑 2600px/s / Recover 减速，ping-pong）；滚过的 cell 平移回放，只有新进入视口(+cache)的 cell 录一次 |
| 滚动条轨道 | thumb 随滚动比例移动（Align 布局驱动） |
| 右栏 STATIC DENSE | 4×4 色格（嵌套 boundary）+ 8 标签全程静止；`rows FFFF-LLLL` 仅窗口变化时更新 |
| 底栏 HUD | 能力 ID + 相位 + fps/p95/policy/paint/presents + 核心 `rr=<累计> Δ=<本tick>/<上界> v=<违例>` + PASS/FAIL 预览色 |

## Gates（§2 R7b 行 + §2.2 全族；硬阈值不许放）

| 族 | 门禁 | FAIL 线 |
|----|------|---------|
| C 能力专用① | **`scroll_rerecord` 有上限** | 累计 `>3×item_count(3600)` → FAIL（每 cell 进入窗口只录一次） |
| C 能力专用② | **静 cell 保持（结构证明）** | 任一 tick `Δrr > ⌊\|dy\|/44⌋+⌊\|dvh\|/44⌋+cache两侧+2` → 违例计数 >0 → FAIL（若每帧全量重录挂载 cell ≈11–15 个，稳态必超上界） |
| C 辅助 | 保留 cell 走缓存回放 | `boundary_skip≤0` → FAIL |
| A 帧时 | 滚动 60fps 档 | `fps_interval<55` 或 `interval_p95_ms>22` → FAIL |
| A 长 soak | hitch 预算 | `hitch_rate_per_min>5` → FAIL |
| E 内存 | RSS 斜率（稳态最小二乘） | `rss_slope_kb_per_min>30000` → FAIL |
| 全族 | `vsync_source` 必须输出；A–J 字段齐 | 缺失/unavailable 无原因 → FAIL |

## 实现点六维（U20）

| 维度 | 本窗回答 |
|------|----------|
| 正确性 | 滚动只引起窗口滑动：保留 cell 平移回放（BoundaryCache translate replay），新入 cell 录一次 |
| 脏区 | 每 tick 重录增量被结构上界约束（Δrr 公式），违例即 FAIL——「静 cell 保持」可机器判定非人眼估计 |
| 缓存 | cell Picture 跨帧复用 + 代际淘汰（120 帧未触碰逐出，重入重录一次如实计入 rr）；boundary_skip 累计证明回放命中 |
| 边界条件 | 快滑到顶/底 ping-pong 反向、视口高度变化（dvh 项）、cache 两侧进入均在上界内 |
| 失败模式 | 全量重录（每帧 11–15 个）→ Δrr 恒超上界 → violations>0 FAIL；rr 累计超 cap FAIL；HUD 预览色红 |
| 窗内如何看出 | HUD `rr/Δ/v` 三数字：Δ 通常 0–2、上界 9–10、v=0；skip 持续增长 |
