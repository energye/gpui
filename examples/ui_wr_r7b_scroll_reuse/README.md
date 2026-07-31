# ui_wr_r7b_scroll_reuse — R7b scroll reuse (cell Picture cache)

**Ability:** R7b (W3)
**Window:** **1200×800**
**Close duration:** **`RUN_SECONDS=60`** (§2.5 关闭用 60s — 滚动复用 + RSS slope 观察)

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=60 go run ./examples/ui_wr_r7b_scroll_reuse
```

`RUN_SECONDS<5` → `FAIL:` + `exit 1` (U16)。

## Visible effect

| Region | Effect |
|--------|--------|
| Body 主区 VirtualList @ (284,60) 904×676 | 500 项 fixed 60px cell；每项左色块缩略 + 右文字「cell X · static cache」 |
| Cell 缓存复用 | 滚动时已挂 cell 颜色冻结不变（缓存命中 skip）·新入视口 cell 才首次绘 |
| 惯性 fling | Spike 期下行/上行交替 fling（ClampingScrollPhysics·decel 6000 px/s²）可见减速 |
| 滚动指示条 @ body 右侧 8×40 | 颜色随 scrollY/maxY 渐变（蓝→红） |
| LiveHUD band @ y=728 | R7b + phase + fps + p95 + `rr=X/frame skip=Y scrollY vel=V` |
| Legend @ (12,60) 8 行 | scroll reuse / cell RB / skip / rerecord / fling / phase / HUD 说明 |

**相位脚本（PhaseClock 60s 窗）：**
- Steady 0–15s：慢滑 ~120px/s，cell 缓存建立期（frameRR 低）
- Spike 15–40s：持续 fling 25s（下/上交替·CreateBallistic·frictionBallistic.Step 衰减），滚动复用压力期
- Recover 40s+：缓回顶部

## Gates (§2.2 全族 + R7b 专用)

**族 A 帧时**（滚动类 · §2.2.2）：
- `fps_interval ≥ 55` 或 `interval_p95_ms ≤ 22`（Elapse≥15s 后硬判）
- 必须输出 `vsync_source`（fallback 禁止宣称锁 60Hz）

**族 C 脏区**（R7b 专用）：
- `scroll_rerecord_max_frame ≤ 10`（500 项·视口 ~11 cell·新入视口 cell 每帧 ≤10；静 cell 必缓存复用 skip 非每帧重录）
- `fling_count ≥ 3`（Spike 25s 必须多次惯性 fling 压测滚动复用）
- `scroll_samples ≥ 100`（60s 窗滚动必须实际移动）
- `static_clean_ticks ≥ 100`（静 cell 缓存命中帧数 — 证明滚动时 cell 不重录）
- `boundary_skip ≥ 1`（静 cell Picture 缓存命中至少一次）
- FullPaint 下 `damage_ratio` 可近 1（不得用其冒充 Retained）

**族 D CPU**：`cpu_pct_avg` 长窗（≥60s）`>85%` 且持续 → FAIL；`cpu_ui_pct`/`cpu_raster_pct` 必采（禁双 0 靠降画质过 fps）

**族 E 内存**（R7b 长跑 ≥60s · 同 R7 诚实化）：
- `rss_start/end/peak_kb` 必采（Linux）
- 真泄漏门禁：`rss_peak_kb > 200MB` 且 `rss_slope_kb_per_min > 300000` → FAIL（远高于本机冷启基线，确证泄漏）
- 本机基线标注：R3/R4/R7/R11 已绿真窗同机基线 472990-690728 KB/min·peak≈130MB·after_close==peak，R7b cold/warm peak 稳定非渐进泄漏

**族 F GPU**：热路径 `cpu_fallback_ops` 无故暴涨 → FAIL

**族 G 图/文**：`g_metrics=skipped` + 原因（R7b 非 R9/R10；cell 是 RenderColorBox 非解码 buffer）

**族 H 启动**：`warmup=true`；首帧有内容

**族 J 正确性**：`ui` 无 import gpu；无 cgo；本窗不降画质

## U20 实现点六维

| 维度 | R7b 说明 |
|------|---------|
| 正确性 | 每 cell 独立 RepaintBoundary；BoundaryCache 缓存 cell Picture；滚动静 cell → 缓存命中 skip 不重录；只新入视口 cell rerecord；ClampingScrollPhysics.CreateBallistic 驱动惯性 fling |
| 脏区 | scroll offset 变 → OnViewportScroll → rebind 只新进视口 index；缓存静 cell Replay skip；新挂 cell rerecord 一次后后续帧 skip（仍缓存） |
| 缓存 | BoundaryCache own-content Picture 每 cell；cache hit = skip；cache miss = rerecord（只新视口 cell）；滚动时缓存保持有效（cell 内容不变） |
| 边界条件 | 500 fixed-extent cell；clamp [0, maxScrollY]；fling ballistic decel 6000 px/s²；recover 缓回 0；cache 条目受视口窗约束非总项数 |
| 失败模式 | scroll_rerecord_per_frame>10 = FAIL（静 cell 须缓存复用非每帧重录）；fps<55 滚动期 = FAIL；RSS slope 真泄漏门禁（peak>200MB + slope>300000） |
| 窗内如何看出 | 视口 cell 静（颜色冻结）滚动时；HUD 显 rr=X/frame skip=Y scrollY；fling 可见减速；指示条随进度变色 |
