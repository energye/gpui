# ui_wr_r7_virtlist — R7 VirtualList virtualization host

**Ability:** R7 (W3)
**Window:** **1200×800**
**Close duration:** **`RUN_SECONDS=60`** (§2.5 关闭用 60s — 滚动 + RSS slope 观察)

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=60 go run ./examples/ui_wr_r7_virtlist
```

`RUN_SECONDS<5` → `FAIL:` + `exit 1` (U16)。本地加长观察 RSS slope 可设 `RUN_SECONDS=120`。

## Visible effect

| Region | Effect |
|--------|--------|
| Body 主区 VirtualList @ (284,60) 904×676 | 1000 项异构列表，仅视口内 ~22-30 cell mounted；每项左色块 + 文字行 + 偶尔图片占位条 |
| Row height 变化 | 行高 28/44/60 三档轮换（idx%7%3 选取）— 变高可见 |
| 滚动指示条 @ body 右侧 8×40 | 颜色随 scrollY/maxY 渐变（蓝→红），标滚动进度 |
| LiveHUD band @ y=728 | R7 + phase + fps + p95 + `bind=X/1000 first..last scrollY=Y/total` |
| Legend @ (12,60) 8 行 | 虚拟化 / 变高 / viewport cell / fling / phase / HUD 说明 |

**相位脚本（PhaseClock 60s 窗）：**
- Steady 0–10s：慢滑 ~30px/s，bind 稳定在 ~22
- Spike 10–25s：快滑 ~400px/s + 2 次惯性 fling（下/上交替），bind 升至 ~30
- Recover 25s+：缓回顶部，bind 回落

## Gates (§2.2 全族 + R7 专用)

**族 A 帧时**（动画/滚动类 · §2.2.2）：
- `fps_interval ≥ 55` 或 `interval_p95_ms ≤ 22`（Elapse≥10s 后硬判）
- 必须输出 `vsync_source`（fallback 禁止宣称锁 60Hz）

**族 B 管线**：`pipeline_depth` 持续 > 配置上限 → FAIL

**族 C 脏区**（R7 专用）：
- `bind_peak ≤ 60`（1000 项虚拟化：只绑 ~22-30 cell，不得超过 60）
- `bind_peak > 0`（VirtualList 必须 mount — 滚动驱动未断）
- `scroll_samples ≥ 100`（60s 窗滚动必须实际移动）
- `fling_count ≥ 2`（Spike 必须触发惯性 fling）
- FullPaint 下 `damage_ratio` 可近 1（不得用其冒充 Retained）

**族 D CPU**：`cpu_pct_avg` 长窗（≥60s）`>85%` 且持续 → FAIL；`cpu_ui_pct`/`cpu_raster_pct` 必采（禁双 0 靠降画质过 fps）

**族 E 内存**（R7 长跑 ≥60s）：
- `rss_start/end/peak_kb` 必采（Linux）
- `rss_slope_kb_per_min > 30000` 极端爬升 → FAIL（虚拟化不得泄漏）

**族 F GPU**：热路径 `cpu_fallback_ops` 无故暴涨 → FAIL

**族 G 图/文**：`g_metrics=skipped` + 原因（R7 非 R9/R10；占位条是 RenderColorBox 非解码 buffer）

**族 H 启动**：`warmup=true`；首帧有内容

**族 J 正确性**：`ui` 无 import gpu；无 cgo；本窗不降画质

## U20 实现点六维

| 维度 | R7 说明 |
|------|---------|
| 正确性 | VirtualList 只 mount 视口窗 + CacheExtent；OnViewportScroll 触发 rebind；BindCount 计 mounted children；BoundRange 返 first/last；变高用 prefix-sum 缓存非全行 mount |
| 脏区 | scroll offset 变 → OnViewportScroll → 只 rebind 新进视口 index；FullPaint 重绘视口但 unmounted item 零 paint 成本；damage_ratio 近 1 是 FullPaint 正确语义非冒充 retained |
| 缓存 | prefix-sum extent 缓存（prefixValid）免全行 measure；InvalidateExtents 在 extent 变更时丢 prefix；CacheExtent=60 保视口外缓冲 |
| 边界条件 | 1000 项变高 28/44/60；clamp [0, maxScrollY]；fling ballistic 衰减 clamp；Recover 缓回 index 0；空列表 BoundaryRange 0,0 |
| 失败模式 | BindCount>60 = FAIL（未虚拟化）；fps<55 滚动期 = FAIL；RSS slope>30000 KB/min = FAIL（泄漏）；scroll_samples<100 = 滚动驱动断 |
| 窗内如何看出 | 视口仅 ~22-30 cell；HUD 显 bind=X/1000 first..last scrollY=Y/total；右侧指示条随进度变色；行高变高可见 |
