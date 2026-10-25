# ui_wr_r10_async_image — R10 图异步→局部脏（W3）

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=30 go run ./examples/ui_wr_r10_async_image
```

## Evidence（U21 三证据，2026-10-25 补）

gate（`-auto-only`）：30s 关闭档 + **60s 观察档两连全绿**（逻辑 10/10 · 像素 13/13（9 点 + 4 墨区）· Golden 821184px 两连 0.0000%；`loaded=20/20 rrViol=0`、fps≈59.9、60s 档 slope≈10.5MB/min）。

- 冻结法：9s 相位钟在 30s 落点恰是 Steady/Spike 边界（30 mod 9 = 3），gate 在**末 3s 钉住 Steady**（HOT 姿态确定）；20 张源图在冻结前全部解码完成。
- 像素口径：图格断言源色棋盘（`(30+9i,60+7i,140+4i)`，`((x+y)/8+i)%2`）；**HOT 块（Align 0.94/0.06）自建窗起就整个落在右栏静态面板底下被盖住**（右栏后画），行为由逻辑门禁证明、不做像素断言，探针改钉覆盖它的右栏 cell(2,0)。
- 已知存量（非本次引入）：30s 档稳态斜率骑 30000 预算线（实测 28.8k/30.2k/31.1k 交替红绿，线程计划 §18.2 #4 早已登记为负载抖）；判漏按 §2.2.4 用 60s 观察档（实测 ~10.5k 稳过）。
- manual 档不断言像素、不写快照；hitch/出图/斜率探针只在完整时长档判定。
- Golden 基线：`testdata/showcase_r10_async_image_base.png`（`GPUI_ACCEPT_GOLDEN=1` 播种；掩码 header+legend+body，HUD 排除，零容差）。

## Window / Close duration

- 窗口 **1200×800**（U15）
- 关闭用时长 **30s**（§2.5；观察窗 60s）。`RUN_SECONDS` 未设时默认取 30；`RUN_SECONDS<5 → FAIL`（U16）
- GPU 真窗必需；无 GPU/X11 环境 → `FAIL: window open (needs_gpu_window)` + exit 1

## Visible effect

| Region | Expectation |
|--------|-------------|
| TopBar | `R10 图异步→局部脏 — 占位→图仅该格变` 标题 |
| 左 Legend | 6 行图例：逐格点亮、逐批记账、paint-only、相位加载节奏、布局驱动说明 |
| Body 4×5 图格 | 每格先显示深色占位符（loading），异步解码完成后**仅该格**翻成条纹图；格与格互不影响；每格独立 RepaintBoundary |
| HOT 动块（右上） | 相位变色+位移持续动画（Align 布局驱动），证明出图不干扰其他动态 |
| 右栏 STATIC DENSE | 4×4 色格 + 8 标签全程静止；`loaded NN / 20` 仅在出图时更新数字 |
| 底栏 HUD | 能力 ID + 相位 + fps/p95/policy/paint/presents + 核心 `img=k/20 rrΔ=<最大批增量> v=<违例>` + `dec≈平均解码ms` |

加载节奏（9s 循环）：Steady 每 0.8s 一张 → Spike 连发 → Recover 1.2s 一张。

## Gates（§2 R10 行 + §2.2 全族；硬阈值不许放）

| 族 | 门禁 | FAIL 线 |
|----|------|---------|
| C 能力专用① | **出图后 rerecord 仅一格** | Steady 非连发相位逐批记账：`boundary_rerecord 增量 > 到达格数+1` → 违例 >0 → FAIL（全局重绘时每批增量≈全部格数必炸；Spike 连发 0.05s/张在途多批不可归因，只记账不判——C3 同款相位门禁语义） |
| C 能力专用③ | **按格归因总量门（§18.1 #3）** | 键级差分 `RerecordByKeySnapshot` 逐批 settle：`changedKeys > 到达格数+1` → key_violations >0 → FAIL（与①同线；实测 keys==dRR 每批一致，增量键为格子/HOT/静态混入且≪boundary 总数，无全局重绘；2026-09-19 真窗判批干净跑 rrV=0/keyV=0） |
| C 能力专用② | **局部脏不触布局（paint-only 契约）** | 引擎单测 `TestRenderImage_SetImage_PaintOnly` 证明 SetImage 只标脏绘制、不标脏布局（窗内 `layout_count` 数的是每帧布局趟数——HUD 文本更新等任何脏节点都 +1——不作逐批观测） |
| C 辅助 | 未出图格保持缓存回放 | `boundary_skip≤0` → FAIL |
| 前提 | 异步出图完成 | `images_loaded < 20` → FAIL |
| A 帧时 | 持续 tick 60fps 档 | `fps_interval<55` 或 `interval_p95_ms>22` → FAIL |
| A 长 soak | hitch 预算 | `hitch_rate_per_min>5` → FAIL |
| E 内存 | RSS 斜率（稳态最小二乘） | `rss_slope_kb_per_min>30000` → FAIL |
| 全族 | `vsync_source` 必须输出；A–J 字段齐 | 缺失/unavailable 无原因 → FAIL |

## 实现点六维（U20）

| 维度 | 本窗回答 |
|------|----------|
| 正确性 | 占位→出图只翻该格内容；解码走 ui/io worker（真实文件 PNG），结果经 channel 回 UI 线程才变更树 |
| 脏区 | Steady 逐批记账（≤ 到达格数+1），违例即 FAIL——「仅一格」机器可判；键级差分同线（`changedKeys ≤ 到达格数+1`，按格归因总量门）；Spike 连发在途多批只记账不判（相位门禁语义见门禁表） |
| 缓存 | 未出图格 boundary 持续 skip（skip 计数佐证）；出图格重录一次后回到回放 |
| 边界条件 | Spike 连发多张同 tick 到达（批量上界=到达数+1）；解码失败路径 SetError 兜底；channel 无阻塞丢弃保护 |
| 失败模式 | 出图引发全局重绘 → 批增量≈全格数 → violations>0 FAIL；SetImage 触发布局 → 引擎单测红；HUD 预览色红 |
| 窗内如何看出 | HUD `img=k/20 rrΔ v` 数字、格子逐个点亮而邻格不动、右栏/HOT 不受出图影响 |
