# ui_wr_r12_metrics — W0 R12 帧指标字段完备

## 运行

```bash
export DISPLAY=:0 LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=5 go run ./examples/ui_wr_r12_metrics
```

窗口 1200×800（U15），GPU 真窗必需，`RUN_SECONDS>=5`（U16，§2.5：5s / 观察 10s）。

## 可见效果（U17）

| 区域 | 内容 |
|------|------|
| TopBar | 标题 `R12 帧指标字段完备 — §2.2.1 A–J 全族` |
| Legend | 4 行说明：schema_only 主判 / 全族字段缺一 FAIL / 真窗须 Present / fps 如实输出 |
| Body 左 | 4×3 静态色格（RepaintBoundary）+ 8 条静态文本 |
| Body 中 | HOT 动块：相位驱动变位变色（Steady 红 / Spike 黄 / Recover 蓝） |
| HUD | paint / presents 实时计数 |

## 门禁（Gate）

`gate=schema_only`（**R12 唯一允许**，§2 行 + §3 组合表）：

| 门禁 | 阈值 | 类型 |
|------|------|------|
| 真窗 Present | `present_count >= 1` | 硬 FAIL |
| wrgate.CheckSchema | 30 个 RequiredSchemaKeys 全存在 | 硬 FAIL |
| §2.2.1 A–J 全族键 | `fullFamilyKeys` 42 个全存在（含 `last_cpu_fallback`、`frame_flushes`、`rss_after_close_kb`、`measure_cache_hit/miss`、`warmup`、`fps_interval` 等 RequiredSchemaKeys 未覆盖的） | 硬 FAIL |

fps 不做硬门禁（schema_only 主判），但如实输出供观察。

## §2.2.1 A–J 全族字段清单（本窗核对真源）

- **A 帧时**：`fps_wall` `interval_avg_ms` `interval_p50_ms` `interval_p95_ms` `interval_p99_ms` `hitch_count` `hitch_rate_per_min` `vsync_source` `target_hz`
- **B 管线**：`frame_build_ms` `frame_raster_ms` `pipeline_depth` `pipeline_max`
- **C 脏区**：`layout_count` `paint_count` `damage_area_px` `damage_ratio` `present_mode` `present_policy`
- **D CPU**：`cpu_pct_avg` `cpu_ui_pct` `cpu_raster_pct`
- **E 内存**：`rss_start_kb` `rss_end_kb` `rss_peak_kb` `rss_slope_kb_per_min` `rss_after_close_kb`
- **F GPU**：`gpu_ops` `cpu_fallback_ops` `last_cpu_fallback` `frame_flushes`
- **G 图/文**：`measure_cache_hit` `measure_cache_miss`（本窗走 measure 路径，有数据必采）
- **H 启动**：`warmup`
- **外壳**：`ability_id` `scenario` `present_count` `frame_count` `elapsed_sec` `fps_interval` `ability_extra`

## 实现点（六维）

1. **窗口**：1200×800 + wrkit 骨架（TopBar/Legend/Body/HUD），U17 达标
2. **指标**：PipelineApp + ProcessTracker 全程采样，wrgate.BuildReport 输出 §2.2 全族 JSON
3. **schema 验证**：`checkAllKeys` 逐键核对 JSON（缺失列表打印到 stderr 并 FAIL）+ `wrgate.CheckSchema`
4. **真窗 Present**：静态内容 + 相位动块持续 tick，证明字段不是空窗编造
5. **诚实**：字段值全部来自 MetricsStore/ProcessTracker 观测（非伪造）；`gate=schema_only` 显式声明，不偷放任何 fps/内容门禁
6. **时长**：`RUN_SECONDS>=5`（U16），§2.5 推荐 5s / 观察 10s
