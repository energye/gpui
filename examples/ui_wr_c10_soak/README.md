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
| LIST 左列 600 行虚拟列表 | Steady 150 / Spike 600 / Recover 50 px/s 变速滚动，58s 周期循环 5 轮 + 10s 稳态收尾，滑块跟随 |
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

历史（60s 相位周期，2026-09-04）：第 1 跑 300s PASS（首跑产 Golden）；第 2 跑 300s PASS；第 3 跑 `exit=1`，唯一失败是 `pixel_golden_diff_pct=1.728%`（5120px，全部落在 120×40 相位芯片区，topbar/legend/dense 逐位 0，像素断言仍 5/5）。根因是 300s 收尾踩 60s 相位分界，终帧芯片色两跑不一致；该历史数据不再作为关闭依据。

2026-10-20 关闭档：相位周期改 58s（300s = 5×58s + 10s，终帧 cycleT=10 落 Steady、浮层关闭、距相位边界 10s）。原备选 55s 会让 300 mod 55=25 落 Spike 浮层开启区间 [20,28)，终帧 `overlay_zone_closed_no_residue` 必红，故弃用。

| 指标 | 第 1 跑（基线） | 第 2 跑（Golden 对比） |
|------|-----|-----|
| `fps_interval` | 59.94 | 59.94 |
| `interval_p95_ms` | 17.86 | 18.16 |
| `hitch_rate_per_min` | 0.20（1 次） | 0 |
| `bind_count` / `item_count` | 17 / 600 | 17 / 600 |
| `scroll_rerecord` | 1442 | 1442 |
| `overlay_opens` | 5 | 5 |
| `img_loaded` | 4 | 4 |
| `boundary_skip` / `rerecord` | 1,411,432 / 6,610 | 1,411,574 / 6,603 |
| `cpu_pct_avg`（ui/raster） | 24.55（3.14/21.42） | 25.02（3.17/21.85） |
| `rss_slope_kb_per_min` | 1220（预算 30000 内） | 1164（预算 30000 内） |
| `rss_end` | 305 MB | 305 MB |
| `vsync_source` | true | true |
| 像素断言 | 5/5（含图片格 0 色稳定） | 5/5 |
| Golden | 首跑存基线 | `0.0000%` / 296292 px |
| `present_count` | 17980 | 17981 |

内存说明：`rss_start≈74MB → 300s≈305MB`——约 231MB 是一次性启动分配（GPU 设备/纹理/图集 + 首帧快照），之后 plateau；两跑 `rss_end` 同带，slope 1164 远低于预算，非泄漏。

CPU/内存门禁口径同 R15（§2.2.3 长窗默认 + §2.2.4 极端爬升线），非拍脑袋；本表为同机回归对照基线。

**关闭结论（2026-10-20）**：两跑 300s 均 `exit=0`，像素 5/5、Golden 次跑 `0.0000%`，图片 4/4 常驻，相位终帧稳定；C10 转 ✅。两跑均 `vsync_source=true`、`cpu_fallback_ops=0`、`gpu_fallbacks=0`。

**指标口径**：`present_policy=retained` 下 `present_mode=full` / `damage_ratio=1` 是本场景每帧全幅损伤的真实观测（HUD 明示 `retained mode=full`），C10 不用 damage 冒充局部脏区；`frame_raster_ms` 终帧含快照读回尖峰，不作 fps 门禁；`gpu_ops` 是 `RenderPathStats` 累计值，不作每帧 submit 数读。`measure_cache_hit=0/miss=632` 是本场景文本度量的观测，非 C10 能力门禁（命中率门禁归 R9）。
