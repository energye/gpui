# ui_wr_c9_stress_cache — C9 R14+R3+R7 组合压测

§3 C9 组合窗：多 boundary + 虚拟列表滚动 + 缓存预算压测（W6）。

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=60 go run ./examples/ui_wr_c9_stress_cache
```

可选：`C9_SNAP_DIR` 覆盖快照目录（默认 `/tmp/c9_stress_cache`）；
`C9_TEX_BUDGET` 覆盖纹理 LRU 显式预算（默认 128）。

## Window / Close duration

- `1200×800`，关闭用 `RUN_SECONDS=60`（§2.5 预算/压力档）。
- `RUN_SECONDS<5 → FAIL`（U16）。

## Visible effect

| Region | Expectation |
|--------|-------------|
| LIST 左列 800 行虚拟列表 | Steady 200 / Spike 800 / Recover 50 px/s 持续滚动，滑块跟随 |
| NEST 3 层嵌套 boundary | 内层相位芯片只在相位翻转时变色，外/中层全程静止 |
| DENSE 3×3 静色格 + 静区注记 | 全程静止，淘汰压力下像素不变 |
| HOT 24×24 动块 | 每帧变色 |
| Cache banner | `CACHE: entries=N/128 ev=M bind=K`，2Hz 刷新 |
| 底栏 LiveHUD | 实时 entries/evict/bind/skip + PASS 预览色 |

本窗跑在 W6 retained 默认上（不显式设策略，测的就是默认）。

## Gates

| # | 门禁 | 类型 |
|---|------|------|
| 1 | `present_policy == retained`（默认即 retained） | 硬 FAIL |
| 2 | `texture_entries_max ≤ budget` 且 `ever_at_cap`（预算封顶且真触顶） | 硬 FAIL |
| 3 | `cache_evictions ≥ 3` 且计数严格单调 | 硬 FAIL |
| 4 | `bind_count` 1..64（800 行只绑一小窗，R7） | 硬 FAIL |
| 5 | `scroll_rerecord > 0`（列表真滚起来） | 硬 FAIL |
| 6 | 持续 tick：`fps_interval ≥ 55`，`p95 ≤ 22ms`，`hitch ≤ 5/min` | 硬 FAIL |
| 7 | 60s 长窗：`cpu_pct_avg ≤ 85`，且 CPU 非双 0 | 硬 FAIL |
| 8 | `rss_slope_kb_per_min ≤ 30000`（§2.2.4 极端爬升线） | 硬 FAIL |
| 9 | 像素断言 3/3（嵌套内层色 + 滚入行色 + 静区文本密度） | 硬 FAIL |
| 10 | Golden 静态掩码次跑起逐位一致（`diff == 0`） | 硬 FAIL |
| 11 | `vsync_source` 必有；`fallback` 禁止宣称锁 60Hz | 硬 FAIL |

## Measured（同机关闭档实测，非拍脑袋阈值）

2026-09-04，60s 关闭档三连跑（第 1 跑修场景 bug：静区注记缺失；
第 2 跑 PASS 并产基线；第 3 跑 PASS，Golden 对基线 `diff=0`）：

| 指标 | 第 2 跑（PASS） | 第 3 跑（PASS） |
|------|-----|-----|
| `fps_interval` | 58.7 | 58.7 |
| `interval_p95_ms` | 17.4 | 17.4 |
| `hitch_rate_per_min` | 0 | 1.0 |
| `texture_entries` min/max | 73 / 128（budget 128，触顶） | 73 / 128 |
| `cache_evictions` | 687（单调） | 687（单调） |
| `bind_count` / `item_count` | 16 / 800 | 17 / 800 |
| `scroll_rerecord` | 375 | 375 |
| `cpu_pct_avg` | 36.8 | 38.9 |
| `rss_slope_kb_per_min` | 24457 | 17369（同带，预算内） |
| `cpu_fallback_ops` | 0 | 0 |
| `vsync_source` | true | true |

CPU 门禁说明：上限 85 取 §2.2.3 长窗默认（单核折算），非拍脑袋；
36.8 为同机回归对照基线—— nominal 是“预算内 + 同带”，若后续同机同档
`cpu_pct_avg` 翻倍以上或 `rss_slope` 出预算，视为回归 FAIL 再议。
