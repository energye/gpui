# ui_wr_c3_list_scroll — C3 组合窗：虚拟列表 + 滚复用 + 异步图格（W3）

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=60 go run ./examples/ui_wr_c3_list_scroll
```

## Window / Close duration

- 窗口 **1200×800**（U15）
- 关闭用时长 **60s**（§2.5；观察窗 120s）。`RUN_SECONDS` 未设时默认取 60；`RUN_SECONDS<5 → FAIL`（U16）
- GPU 真窗必需；无 GPU/X11 环境 → `FAIL: window open (needs_gpu_window)` + exit 1

## 覆盖的主能力（集成，不代替任何单 R）

R7 虚拟化宿主 · R7b 滚动少重录 · R10 图异步→局部脏 · R4 语义（壳层静态区局部脏）

## Visible effect

| Region | Expectation |
|--------|-------------|
| TopBar | `C3 组合 — 虚拟列表+滚复用+异步图格` 标题 |
| 左 Legend | 6 行图例：三大能力集成、逐 tick 门禁公式、布局驱动说明 |
| Body 列表区 | 1200 项变高异构列表持续滚动（Steady 慢滚/Spike 快滑/Recover 减速 ping-pong）；图片行（每 4 行 1 个）先占位后**边滚边点亮**（24 张源图异步解码，前 ~15s 内派发完） |
| 滚动条轨道 | thumb 随滚动比例移动（Align 布局驱动） |
| 右栏 STATIC DENSE | 4×4 色格 + 8 标签全程静止；`rows FFFF-LLLL` 与 `img NN / 24` 仅在窗口/出图变化时更新 |
| 底栏 HUD | 能力 ID + 相位 + fps/p95/policy/paint/presents + 核心 `bind/item/srr` + `img k/24 v=违例` + PASS/FAIL 预览色 |

## Gates（§3 C3 行指标要点 bind/scroll_rerecord/p95 + §2.2 全族；硬阈值不许放）

| 族 | 门禁 | FAIL 线 |
|----|------|---------|
| C（R7） | **bind_count ≪ item_count** | `item<1200` 或 `bind>64` 或 `bind*10>item` → FAIL |
| C（R7b）① | **scroll_rerecord 上限** | 累计 `>3×item_count(3600)` → FAIL |
| C（R7b）② | **滚复用结构门禁（含出图额度）** | 每 tick 全部重录增量 > `⌊\|dy\|/44⌋+⌊\|dvh\|/44⌋+cache两侧+2` 且超出部分无出图额度可吸收 → 违例 >0 → FAIL（出图重录可能延迟到 cell 重入视口的 tick，由额度池诚实吸收，总量 ≤24） |
| C（R10） | 异步出图完成 | `images_loaded < 24` → FAIL |
| C 辅助 | 静区/保留 cell 回放 | `boundary_skip≤0` → FAIL |
| A 帧时 | 持续 tick 60fps 档（**p95 为本窗要点**） | `fps_interval<55` 或 `interval_p95_ms>22` → FAIL |
| A 长 soak | hitch 预算 | `hitch_rate_per_min>5` → FAIL |
| E 内存 | RSS 斜率（稳态最小二乘） | `rss_slope_kb_per_min>30000` → FAIL |
| 全族 | `vsync_source` 必须输出；A–J 字段齐 | 缺失/unavailable 无原因 → FAIL |

## 实现点六维（U20）

| 维度 | 本窗回答 |
|------|----------|
| 正确性 | 三能力同窗共存：虚拟化挂载、滚动回放、异步点亮互不破坏；解码经 ui/io worker，结果 channel 回 UI 线程才变更树 |
| 脏区 | 每 tick 全部重录增量被「滚动进入上界+出图额度」约束——滚动归滚动、出图归出图，超额即 FAIL |
| 缓存 | decoded 源图缓存（SetImageShared 共享，重挂载立即有图）+ cell Picture 代际淘汰 + boundary_skip 佐证回放命中 |
| 边界条件 | 出图时 cell 可能已滚出（延迟到重入 tick，额度吸收）；ping-pong 反向；重复到达/空结果保护；24 buffer 统一 Dispose |
| 失败模式 | 全量重录/全局重绘 → Δrr 超 上界+额度 → violations>0 FAIL；虚拟化失效 → bind 门禁 FAIL；HUD 预览色红 |
| 窗内如何看出 | HUD `bind/item/srr` + `img k/24 v` 数字、图片行边滚边亮而邻格不动、右栏静止 |
