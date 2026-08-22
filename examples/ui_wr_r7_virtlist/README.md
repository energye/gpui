# ui_wr_r7_virtlist — R7 虚拟化宿主（W3）

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=60 go run ./examples/ui_wr_r7_virtlist
```

## Window / Close duration

- 窗口 **1200×800**（U15）
- 关闭用时长 **60s**（§2.5；观察窗 120s）。`RUN_SECONDS` 未设时默认取 60；`RUN_SECONDS<5 → FAIL`（U16）
- GPU 真窗必需；无 GPU/X11 环境 → `FAIL: window open (needs_gpu_window)` + exit 1

## Visible effect

| Region | Expectation |
|--------|-------------|
| TopBar | `R7 虚拟化宿主 — VirtualList 只挂视口` 标题 |
| 左 Legend | 7 行图例：场景构成、门禁、相位脚本、布局驱动说明 |
| Body 左侧列表区（~620px 宽） | 1200 项**变高**异构列表持续滚动：文字行(44px)/色块对(92px)/文字+色签(60px)/图片+文字(76px) 四类混排；只挂视口(+cache) cell，快滑不卡 |
| 滚动条轨道（列表右缘） | 蓝色 thumb 随滚动比例上下移动（Align 布局驱动），到顶/底 ping-pong 反向 |
| 右栏 STATIC DENSE | 4×4 色格（嵌套 boundary）+ 8 条静态标签**始终静止**，不随滚动重绘；顶部 `rows FFFF-LLLL / 1200` 仅在绑定窗口变化时更新数字 |
| 底栏 HUD | 能力 ID + 相位（Steady/Spike/Recover）+ fps/p95/policy/paint/presents + 核心 `bind=XX item=1200 rr=X` + PASS/FAIL 预览色 |

相位脚本（9s 循环）：Steady 慢滚 160px/s → Spike 快滑 2600px/s → Recover 线性减速。

## Gates（§2 R7 行 + §2.2 全族；硬阈值不许放）

| 族 | 门禁 | FAIL 线 |
|----|------|---------|
| C 能力专用 | **`bind_count ≪ item_count`** | `item_count<1200` 或 `bind_count>64` 或 `bind*10>item` → FAIL |
| A 帧时 | 滚动/持续 tick 60fps 档 | `fps_interval<55` 或 `interval_p95_ms>22` → FAIL |
| A 长 soak | hitch 预算 | `hitch_rate_per_min>5` → FAIL |
| E 内存 | RSS 斜率（虚拟化必须平稳） | `rss_slope_kb_per_min>30000` → FAIL |
| 全族 | `vsync_source` 必须输出 | 缺失 → FAIL（`fallback` 如实标注，不宣称锁 60Hz） |
| 全族 | `present_count≥1`；A–J 字段全集出现 | 缺字段/unavailable 无原因 → FAIL |

## 实现点六维（U20）

| 维度 | 本窗回答 |
|------|----------|
| 正确性 | 1200 项只有视口(+2 行 cache) cell 挂载；变高行走前缀和 index↔offset（非 固定高×index） |
| 脏区 | 滚动 rebind 只波及列表子树；右栏静态密集区不随滚动重绘 |
| 缓存 | 已挂 cell 的 Picture 跨滚动偏移复用（保留 cell 清脏位回放）；文字 measure 缓存在重挂行命中 |
| 边界条件 | 快滑到顶/底 clamp + ping-pong 反向；变高前缀和惰性重建；图片 buffer 共享所有权（SetImageShared） |
| 失败模式 | bind==item（全挂载）→ 门禁 FAIL；RSS 随滚动爬升 → slope FAIL；HUD 预览色红 |
| 窗内如何看出 | HUD `bind=XX item=1200` 数字 ≪、`rows FFFF-LLLL` 窗口滑动、thumb 位置、右栏静态区稳定 |
