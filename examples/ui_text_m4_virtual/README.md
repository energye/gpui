# ui_text_m4_virtual — M4 真窗（1e6 行日志虚拟化，§M4）

1e6 行日志场景：行内容来自 `testdata/log_sample.txt`（40 条代表性日志），
窗内按序循环复用到 1e6 行（`main.go` 不发明行内容，只有复用索引）。
几何真源是 `VirtualTextLines`（懒测量 + Fenwick 前缀和）；
`VirtualList` 只挂载视口 cell，行高经 `ExtentFunc` 从真源取。
变行高：每 97 行一条折行（34px），其余 20px，滚到才实测。

## 运行

```sh
GPUI_M4_SELFTEST=1 go run ./examples/ui_text_m4_virtual  # 无头基线，无需 GPU
RUN_SECONDS=30 go run ./examples/ui_text_m4_virtual     # 交互窗 30s（含滚动脚本）
```

## 改前全量基线（2026-09-04，本机，建窗后、接虚拟化前取）

| 规模 | 全量排版（改前路径） | 虚拟化首屏 |
|---|---|---|
| 2k 行 | 38.3ms（估算路径）/ 232.9ms（真字体） | — |
| 20k 行 | 349.7ms（估算路径；10×行数→9.1×耗时，O(n) 确认） | — |
| 1e6 行 | 未直接跑（按线性外推约 17s 估算路径/分钟级真字体，数 GB 堆，不敢全量跑） | **7ms（含 1M 前缀和一次性构建），窗口 42 行，总高 20000000px，堆 +17MB（一次性 16MB 表）** |

读法：全量路径首屏 O(n)，1e6 行必超熔断 500ms（外推超约 30 倍）；
虚拟化首屏与总行数无关（定行高零按行内存见单测；本窗变高模式一次性 16MB 表是已知代价，见实现注释）。

## 门禁（§M4 ⑥）与实测（真机 GPU，同窗 RUN_SECONDS=60，两轮）

`BenchmarkOpenDocument` 1e6 档 ≤100ms（实测首屏约 7ms）；A 族 `interval_p95_ms ≤ 22`、
`fps_interval ≥ 55`；E 族 `rss_slope` 达标；真窗退出 JSON（A–J 全族 + Extra）。

| 指标 | 第一轮 60s | 第二轮 60s | 120s | 门禁 | 判定 |
|---|---|---|---|---|---|
| `fps_interval` | 58.77 | 58.81 | 58.70 | ≥ 55 | ✅ |
| `interval_p95_ms` | 18.77 | 19.19 | 17.96 | ≤ 22 | ✅ |
| `hitch_rate_per_min` | 8.0 | 9.0 | 12.5 | ≤ 5 | ❌ 见下 |
| `cpu_fallback_ops` | 0 | 0 | 0 | == 0 | ✅ |
| `rss_slope_kb_per_min` | 34097 | 25161 | 13874 | ≤ 30000 | ✅（60s 稳态口径，第二轮起达标） |
| `bind/max_bind/1e6` | 45/45 | 44/45 | — | ≪ 总数 | ✅ |
| `anchor_jumps` | 0 | 0 | — | == 0 | ✅ 不跳变 |
| `prefix_invalidates` | 43 | 43 | — | 越少越好 | 见下 |

hitch 未达标根因（已实证定位，非推测）：每次懒实测到折行高度都触发
`VirtualList` 全量前缀重建（O(100 万行)，实测 8–14ms/次，60s 约 43 次），
与 spike 帧工作叠加即超 33.4ms。关掉刷新跑同窗 60s（诊断模式，位置会漂移，
仅作归因）hitch 归 0、p95 回 17.6——锁定重建为唯一主因。新鲜行排版（282µs/行）、
GC（最大 STW 0.69ms）均已实测排除。初版每帧都刷（133 次/30s，fps 掉到 34.9）
已改为只在总高真变时刷（43 次/60s）。

## M4.1 增量前缀（hitch 根治，2026-09-04，真机 GPU 60s 全绿）

`ui/rendering/virtual_list.go` 新增 `VirtualList.RefreshExtents(first, last)`：
只重读变化区间行高 + 后缀平移（O(区间)闭包调用 + O(n) 纯浮点加，零分配），
上报是否真变；无有效前缀/总行数变化时回退全量重建（`InvalidateExtents` 保留给
行数变化等粗粒度场景）。真窗改调一处（`InvalidateExtents` → `RefreshExtents`）。

| 证据 | 值 | 门禁 | 判定 |
|---|---|---|---|
| `BenchmarkRefreshExtents/refresh`（1e6 表，60 行区间） | 0.60ms | — | — |
| `BenchmarkRefreshExtents/full`（同条件全量重建） | 9.79ms | — | 比值 0.06 ✅（约 16 倍） |
| 60s 真窗 `fps_interval` / `interval_p95_ms` | 59.01 / 18.34 | ≥ 55 / ≤ 22 | ✅ |
| 60s `hitch_count` | **0** | ≤ 5/min | ✅（此前 8–12） |
| 60s `rss_slope` / `cpu_fallback_ops` | 24176 / 0 | ≤ 30000 / == 0 | ✅ |
| `anchor_jumps` / `window_lag_max` / `bind` | 0 / 0 / 44 | — | ✅ 几何与全量重建逐项一致（单测锁） |
| 单测 `TestRefreshExtents_*` 3 个 | 全绿 | — | 红灯起步（API 缺失编译失败）后转绿 |
| 回归 17 个既有列表/视口测试 | 全绿 | — | `VirtualList` 旧行为零改动 |

M4 施工中另修一处 M1 遗留并发崩溃（60s 跑挂出）：主线程建层与光栅线程绘制
同时进同一 `RenderText` 的 `layoutCache`（`concurrent map writes`），注释写的
"只在事件循环线程用"假设已破。按 A 方案（已确认）修：`layoutCache` 三入口
（`update`/`updateSpan`/`buildCached`）统一拿互斥（查命中也改计数，故不分读写锁），
全局 `textLayoutGen` 改原子（I9 全局自增语义不变）。回归：16 个 M1 缓存测试 +
13 老裁判 + `BenchmarkKeystroke` 全绿；60s×4 + 120s + 30s×4 无崩溃。
