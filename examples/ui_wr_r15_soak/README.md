# ui_wr_r15_soak — R15 UI/raster 所有权长跑

§2 R15 单能力窗：全主路径组合（静态+动画+滚动+浮层）300s 持续运行（W2+）。

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=300 go run ./examples/ui_wr_r15_soak
```

可选：`R15_SNAP_DIR` 覆盖快照目录（默认 `/tmp/r15_soak`）。

## Window / Close duration

- `1200×800`，关闭用 `RUN_SECONDS=300`（§2.5 Soak 长跑档）。
- `RUN_SECONDS<5 → FAIL`（U16）；`<300` 的显式设置跑只做调试，不作关闭证据。

## Visible effect

| Region | Expectation |
|--------|-------------|
| LIST 左列 600 行虚拟列表 | Steady 150 / Spike 600 / Recover 50 px/s 变速滚动，60s 周期循环 5 轮，滑块跟随 |
| NEST 3 层嵌套 boundary | 内层相位芯片只在相位翻转时变色，外/中层全程静止 |
| DENSE 3×3 静色格 + 静区注记 | 全程静止，300s 后像素不变 |
| HOT 24×24 动块 | 每帧变色 |
| OVERLAY 右下浮层面板 | 每轮 Spike 开 8s 后关（300s 共 5 次），HUD 显示开合态 |
| 底栏 LiveHUD | 实时 fps/p95/bind/skip/overlay 开合态 + PASS 预览色 |

本窗跑在 W6 retained 默认上（不显式设策略，测的就是默认）。
运行中无逐帧读回：像素采样只读终帧 PNG 文件，不碰 GPU backbuffer。

## Gates

| # | 门禁 | 类型 |
|---|------|------|
| 1 | `RUN_SECONDS ≥ 300`（关闭用时长） | 硬 FAIL |
| 2 | `present_count ≥ 1`，无崩（exit 0 才算过） | 硬 FAIL |
| 3 | `boundary_skip ≥ 3` 且 `boundary_rerecord ≥ 1`（嵌套隔离真发生） | 硬 FAIL |
| 4 | `bind_count` 1..64（600 行只绑一小窗） | 硬 FAIL |
| 5 | `scroll_rerecord > 0`（列表真滚起来） | 硬 FAIL |
| 6 | `overlay_opens ≥ 3`（浮层按周期开合） | 硬 FAIL |
| 7 | 持续 tick：`fps_interval ≥ 55`，`p95 ≤ 22ms`，`hitch ≤ 5/min` | 硬 FAIL |
| 8 | 300s 长窗：`cpu_pct_avg ≤ 85`，且 CPU 非双 0 | 硬 FAIL |
| 9 | `rss_slope_kb_per_min ≤ 30000`（§2.2.4 极端爬升线） | 硬 FAIL |
| 10 | 像素断言 4/4（嵌套内层色 + 滚入行色 + 浮层区关闭无残留 + 静区文本密度） | 硬 FAIL |
| 11 | Golden 静态掩码次跑起逐位一致（`diff == 0`） | 硬 FAIL |
| 12 | `vsync_source` 必有；`fallback` 禁止宣称锁 60Hz | 硬 FAIL |

## Measured（同机关闭档实测，非拍脑袋阈值）

2026-09-04，第 1 跑 300s 关闭档 PASS（`exit=0`，首跑产 Golden 基线）：

| 指标 | 第 1 跑（PASS） |
|------|-----|
| `fps_interval` | 58.4 |
| `interval_p95_ms` | 17.7 |
| `hitch_rate_per_min` | 0.2（1 次） |
| `bind_count` / `item_count` | 16 / 600 |
| `scroll_rerecord` | 1423 |
| `overlay_opens` | 5（5 轮每轮 1 次） |
| `boundary_skip` | 1259972（单调） |
| `cpu_pct_avg`（ui/raster） | 43.0（3.0/40.0，非双 0） |
| `rss slope` | 1539 KB/min（预算 30000 内） |
| `vsync_source` | true |
| 像素断言 | 4/4 |
| Golden | 首跑存基线（次跑起逐位比对） |

内存说明：`rss_start≈73MB → 10s 冒烟跑≈380MB → 300s≈384MB`——约 300MB
是一次性启动分配（GPU 设备/纹理/图集 + 首帧快照），10s 后即 plateau，
后 290s 仅 +4MB，稳态 slope 1539 远低于预算，非泄漏。次跑对比 `rss_end`
是否同带可进一步确认（见第 2 跑记录）。

CPU 门禁说明：上限 85 取 §2.2.3 长窗默认（单核折算），非拍脑袋；
43.0 为同机回归对照基线——nominal 是“预算内 + 同带”，若后续同机同档
`cpu_pct_avg` 翻倍以上或 `rss_slope` 出预算，视为回归 FAIL 再议。

**未关闭项（诚实记录）**：Golden 次跑验证还没跑（R15 状态保持 ⬜）。
注意 C10 第三跑暴露的同构边界问题同样适用于本窗——300s 收尾踩在 60s
相位分界上，终帧相位芯片色可能两跑不一致。次跑验证前建议先修相位周期
（备选 55s），待用户另行确认。
