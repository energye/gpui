# ui_wr_c10_soak — C10 R15+全主路径组合长跑

§3 C10 组合窗：全能力组合长跑（静态+动画+滚动+浮层+图片）300s（W2+）。
只做集成，不代替任何 R（R15 有自己的 `ui_wr_r15_soak` 独立窗）。

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=300 go run ./examples/ui_wr_c10_soak
```

可选：`C10_SNAP_DIR` 覆盖快照目录（默认 `/tmp/c10_soak`）。

## Window / Close duration

- `1200×800`，关闭用 `RUN_SECONDS=300`（§2.5 Soak 长跑档 = max(R15)）。
- `RUN_SECONDS<5 → FAIL`（U16）；`<300` 的显式设置跑只做调试，不作关闭证据。

## Visible effect

| Region | Expectation |
|--------|-------------|
| LIST 左列 600 行虚拟列表 | Steady 150 / Spike 600 / Recover 50 px/s 变速滚动，60s 周期循环 5 轮，滑块跟随 |
| NEST 3 层嵌套 boundary | 内层相位芯片只在相位翻转时变色，外/中层全程静止 |
| DENSE 3×3 静色格 + 静区注记 | 全程静止，300s 后像素不变 |
| HOT 24×24 动块 | 每帧变色 |
| OVERLAY 右下浮层面板 | 每轮 Spike 开 8s 后关（300s 共 5 次），HUD 显示开合态 |
| IMG 2×2 解码图片格 | 启动时一次解码（R10 式文件解码路径），300s 全程零解码，格 0 纯色、格 1–3 条纹 |
| 底栏 LiveHUD | 实时 fps/p95/bind/skip/overlay 开合态 + PASS 预览色 |

本窗跑在 W6 retained 默认上（不显式设策略，测的就是默认）。
运行中无逐帧读回：像素采样只读终帧 PNG 文件，不碰 GPU backbuffer。

## Gates（覆盖能力门禁并集 = R15 全部门禁 + 图片项）

| # | 门禁 | 类型 |
|---|------|------|
| 1 | `RUN_SECONDS ≥ 300`（关闭用时长） | 硬 FAIL |
| 2 | `present_count ≥ 1`，无崩（exit 0 才算过） | 硬 FAIL |
| 3 | `boundary_skip ≥ 3` 且 `boundary_rerecord ≥ 1`（嵌套隔离真发生） | 硬 FAIL |
| 4 | `bind_count` 1..64（600 行只绑一小窗） | 硬 FAIL |
| 5 | `scroll_rerecord > 0`（列表真滚起来） | 硬 FAIL |
| 6 | `overlay_opens ≥ 3`（浮层按周期开合） | 硬 FAIL |
| 7 | `img_loaded == 4`（4 格图片启动时全部就位，否则启动即 FAIL） | 硬 FAIL |
| 8 | 持续 tick：`fps_interval ≥ 55`，`p95 ≤ 22ms`，`hitch ≤ 5/min` | 硬 FAIL |
| 9 | 300s 长窗：`cpu_pct_avg ≤ 85`，且 CPU 非双 0 | 硬 FAIL |
| 10 | `rss_slope_kb_per_min ≤ 30000`（§2.2.4 极端爬升线） | 硬 FAIL |
| 11 | 像素断言 5/5（R15 的 4 项 + 图片格 0 色稳定） | 硬 FAIL |
| 12 | Golden 静态掩码次跑起逐位一致（`diff == 0`） | 硬 FAIL |
| 13 | `vsync_source` 必有；`fallback` 禁止宣称锁 60Hz | 硬 FAIL |

跨能力交互验证（`impl_interaction`）：滚动 churn、浮层开合、图片纹理同帧
合成——图片格全程静止证明合成不污染静态纹理，浮层开合不抬升主带重录。

## Measured（同机关闭档实测，非拍脑袋阈值）

2026-09-04，第 1 跑 300s 关闭档 PASS（`exit=0`，首跑产 Golden 基线）：

| 指标 | 第 1 跑（PASS） |
|------|-----|
| `fps_interval` | 58.3 |
| `interval_p95_ms` | 17.4 |
| `hitch_rate_per_min` | 0.8（4 次，预算 5 内） |
| `bind_count` / `item_count` | 16 / 600 |
| `scroll_rerecord` | 1423 |
| `overlay_opens` | 5（5 轮每轮 1 次） |
| `img_loaded` | 4（启动一次解码全就位） |
| `boundary_skip` | 1433024（单调） |
| `cpu_pct_avg` | 41.7（非双 0） |
| `rss slope` | 1612 KB/min（预算 30000 内；`rss_end≈384MB` 与 R15 同带，一次性启动分配） |
| `vsync_source` | true |
| 像素断言 | 5/5（含图片格 0 色稳定） |
| Golden | 首跑存基线（次跑起逐位比对） |

CPU/内存门禁口径同 R15（§2.2.3 长窗默认 + §2.2.4 极端爬升线），非拍脑袋；
本表数值即同机回归对照基线。

2026-09-04，第 2 跑 300s PASS（`exit=0`，改名后新基线）：
fps 58.6、p95 17.4、hitch 0.2/min、bind 16/600、rr 1424、ov 5、img 4/4、
像素 5/5、slope 1195、cpu 38.4、rss_end 384MB（三次同带）。

**未关闭项（诚实记录）**：第 3 跑 300s `exit=1`，唯一失败是
`pixel_golden_diff_pct=1.728%`（5120px，逐位对比定位**全部落在 120×40
相位芯片区**，topbar/legend/dense 三区逐位 0，像素断言仍 5/5）。
根因是 300s 收尾正好踩在 60s 相位分界线上，终帧芯片色两跑不一致——
真窗场景设计的边界问题，不是引擎洞。用户决定先不修，C10 状态保持 ⬜；
备选修法是相位周期改 55s 让收尾落相位正中，待另行确认后再跑 Golden 验证。
