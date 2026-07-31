# ui_wr_c3_list_scroll — C3 list+scroll composite (R4+R7+R7b+R10)

**Ability:** C3 (W3 组合窗 · 集成非代替)
**集成:** R4 retained damage + R7 virtualization + R7b scroll reuse + R10 async image→local dirty
**Window:** **1200×800**
**Close duration:** **`RUN_SECONDS=60`** (§2.5 关闭用 60s — 滚动+异步图+retained 观察)

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=60 go run ./examples/ui_wr_c3_list_scroll
```

`RUN_SECONDS<5` → `FAIL:` + `exit 1` (U16)。

## Visible effect

| Region | Effect |
|--------|--------|
| Body 主区 VirtualList @ (284,60) 904×676 | 1000 项异构列表·仅视口内 ~22-30 cell mounted·每项左 RenderImage 缩略 + 右文 label |
| Row 变高 | 60/80/100 三档轮换（idx%7%3 选取）— 变高可见 |
| 滚动复用 | 滚动时已挂 cell Picture 缓存命中 skip·只新入视口 cell rerecord·fling 惯性减速可见 |
| 异步图 | Steady/Spike 期逐项 SetImage（0.6s 一格·左上→右下·色随 idx 变）·**只该格闪变**其他静 |
| retained damage | 滚动时 damage 只在 viewport 移动区·非全屏重绘（damage_ratio≪1） |
| 滚动指示条 @ body 右侧 8×40 | 随 scrollY/maxY 垂直移动 + 颜色渐变 |
| LiveHUD band @ y=728 | C3 + phase + fps + p95 + `bind=X/1000 rr=Y/frame dirty/img=1 dmg=D` |
| Legend @ (12,60) 10 行 | 四能力集成 / 变高 / viewport cell / fling / async img / retained / HUD 说明 |

**相位脚本（PhaseClock 60s 窗）：**
- Steady 0–10s：慢滑 ~100px/s + 缓存建立 + 开始异步图加载
- Spike 10–45s：持续 fling 35s（下/上交替·CreateBallistic·frictionBallistic.Step �衰减）+ 异步图加载压测
- Recover 45s+：缓回顶部

## Gates (§2.2 全族 + C3 四能力集成专用)

**族 A 帧时**：`fps_interval ≥ 55` 或 `interval_p95_ms ≤ 22`（Elapse≥10s 后硬判）·必须输出 `vsync_source`

**族 C 脏区**（C3 集成专用）：
- `bind_peak ≤ 60`（R7 虚拟化·1000 项只绑 ~22-30 cell）
- `scroll_rerecord_max_frame ≤ 10`（R7b 滚动复用·静 cell 缓存命中·只新入视口 cell 重录）
- `scroll_samples ≥ 100`（滚动必须实际移动）
- `max_dirty_per_img_load ≤ 1`（R10 局部脏·SetImage 只脏一格非全树）
- `load_events ≥ 10`（异步图加载压测）
- **`damage_ratio_peak < 0.95`**（R4 retained·滚动时 damage 只在 viewport 移动区·非全屏·FullPaint 下 damage_ratio=1 会冒充 retained 必须用 Composite=true）

**族 D CPU**：`cpu_pct_avg` 长窗 `>85%` 且持续 → FAIL·`cpu_ui_pct`/`cpu_raster_pct` 必采（禁双 0）

**族 E 内存**（C3 长跑 ≥60s · 同 R7/R7b/R10 诚实化）：
- `rss_start/end/peak_kb` 必采（Linux）
- 真泄漏门禁：`rss_peak_kb > 200MB` 且 `rss_slope_kb_per_min > 300000` → FAIL
- 本机基线标注：R3/R4/R7/R7b/R10/R11 已绿真窗同机基线·peak≈130MB·after_close==peak，C3 cold/warm peak 稳定非渐进泄漏

**族 F GPU**：热路径 `cpu_fallback_ops` 无故暴涨 → FAIL

**族 G 图/文**：`g_metrics=skipped` + 原因（C3 组合窗·各能力门禁在各 ui_wr_r* 真窗验·组合窗不重测 g_metrics）

**族 H 启动**：`warmup=true`·首帧有内容

**族 J 正确性**：`ui` 无 import gpu·无 cgo·本窗不降画质

## U20 实现点六维

| 维度 | C3 说明 |
|------|---------|
| 正确性 | VirtualList 宿主变高行·每 row 独立 RepaintBoundary 缓存 cell Picture·row 嵌 RenderImage（独 RB）做异步加载·RenderViewport + ClampingScrollPhysics 启 fling·retained composite 模式 scope damage 到 viewport 滚动 delta |
| 脏区 | scroll offset → OnViewportScroll → rebind 只新进视口 index·缓存静 cell Replay skip·新挂 cell rerecord 一次后 skip·SetImage → MarkNeedsPaint(self) 只脏一格·retained damage 区 = viewport 滚动 delta band �全表面 |
| 缓存 | BoundaryCache per-row cell Picture（R7b 滚动复用）·per-RenderImage cache entry（R10 局部脏）·retained composite cache（R4）— damage scoped �全局重绘 |
| 边界条件 | 1000 项变高 60/80/100·clamp [0, maxScrollY]·fling ballistic 衰减·recover 缓回 0·异步图 SetImage 与滚动交错·retained 模式 damage 永不全表面即使快 fling |
| 失败模式 | bind_peak>60 = FAIL（R7）·scroll_rerecord_per_frame>10 = FAIL（R7b）·dirty_per_img>1 = FAIL（R10）·damage_ratio_peak>=0.95 = FAIL（R4 retained 必须 scope damage 非全屏）·fps<55 滚动期 = FAIL·RSS 真泄漏门禁 |
| 窗内如何看出 | 视口仅 ~22-30 cell 变高·cell 左图缩略（占位→加载解码色块）+ 右文 label·滚动 fling 减速可见·只新加载图格闪变其他静·HUD 显 bind=X/1000 rr=Y/frame dirty/img=1 dmg=D·retained damage 只在滚动区 |
