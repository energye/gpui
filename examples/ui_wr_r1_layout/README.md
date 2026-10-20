# ui_wr_r1_layout — R1 局部 NeedsLayout

§2 R1 单能力窗：改一个子树的高度，只脏该 relayout boundary，不惊动整窗。

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=30 go run ./examples/ui_wr_r1_layout
```

可选：`R1_SNAP_DIR` 覆盖快照目录（默认 `/tmp/r1_layout`）。

## Window / Close duration

- `1200×800`，固定尺寸（resize 属 R11，不在本窗混测）。
- R1 表下限 `RUN_SECONDS=5`；关闭证据用 **30s**（§2.5 “可更长”）。
- 为什么是 30s：U21 要求稳态 + 恢复两张 raster 线程快照，每次快照读回都是一次真实 hitch；5s 窗两跳 = 24/min，会误触 `hitch ≤5/min`。30s 窗两跳 = **4/min**，门禁保持 5 不变，不是把阈值放大。
- 相位：Steady `0–10s` / Spike `10–20s` / Recover `20–30s`；目标高度变化在 `12/14/16/18s` 四次，最终回到 120。
- 未设置 `RUN_SECONDS` 时默认跑 30s；显式 `RUN_SECONDS<30` 只做调试，不作关闭证据。

## Visible effect

| Region | Expectation |
|--------|-------------|
| TARGET FRAME | 真引擎 relayout boundary；内部蓝色 target 高度 `120→200→120→200→120`，橙色 marker 随约束重新居中 |
| TARGET 下方 slot | Spike 时被 200 高 target 盖住；Recover 必须重新露出 frame 背景 |
| SIBLING chip | 与 target 同父的 120×24 青色 chip 全程不挪；offset / 像素 / Golden 三证 |
| NEIGHBOR 4×4 + 8 标签 | 460×220 静区，全程不动 |
| STRIPES 三段色带 | 第二块静区，全程不动 |
| HOT 呼吸盘 | 2.5s 周期缩放的 paint-only 动画，不触发 layout |
| 左上 phase lamp | Steady 蓝 / Spike 橙 / Recover 绿；稳态与恢复快照各验一次 |
| 底栏 LiveHUD | `h`、`lay=delta/changes`、`dirty`、fps/p95，PASS 预览色 |

本窗显式 `present_policy=full_paint`：R1 测的是布局隔离，不是 retained 局部脏区；`present_mode=full` / `damage_ratio=1` 是每帧全幅损伤的真实观测，不能拿来冒充局部性。

## Gates

| # | 门禁 | 类型 |
|---|------|------|
| 1 | `RUN_SECONDS ≥ 30`（未设置时默认 30；R1 表 5s 下限之上加长；显式更短 FAIL） | 硬 FAIL |
| 2 | `present_count ≥ 1`，exit 0，无崩 | 硬 FAIL |
| 3 | `layout_boundary_present == true`（target frame 真是 boundary） | 硬 FAIL |
| 4 | `height_changes == 4`，`layout_delta == 4`（一次高度变化恰一次 layout flush） | 硬 FAIL |
| 5 | `layout_isolation_violations == 0`（root/body/top/legend/sibling 全不脏） | 硬 FAIL |
| 6 | `neighbor_shift_px == 0`，`target_frame_shift_px == 0`，`sibling_chip_shift_px == 0` | 硬 FAIL |
| 7 | 终帧 `target_height == 120`，marker `offset.y == 36`，target 已不脏 | 硬 FAIL |
| 8 | 像素断言 10/10（见下） | 硬 FAIL |
| 9 | Golden 静态掩码次跑 `diff == 0` | 硬 FAIL |
| 10 | steady↔recover 同静态掩码 `diff == 0`（跨状态静区不动） | 硬 FAIL |
| 11 | `fps_interval ≥ 55`，`p95 ≤ 22ms`，`hitch ≤ 5/min` | 硬 FAIL |
| 12 | `cpu_pct_avg > 0` 且 ≤ 85；CPU 非双 0 | 硬 FAIL |
| 13 | `vsync_source` 必有；`gpu_fallbacks == 0` | 硬 FAIL |
| 14 | 两张快照存在且非空白（steady 蓝灯 / recover 绿灯） | 硬 FAIL |

像素 10 项：neighbor cell00、target body、marker center、slot restored、sibling chip、stripe0、hot center、recover 绿灯、steady 蓝灯、neighbor 标签文本密度（≥120 px）。

## Measured（同机关闭档实测，非拍脑袋阈值）

2026-10-20 关闭档，两次 30s 均 `exit=0`；第一跑不带 `RUN_SECONDS`（默认 30s，顺带验证无参路径），第二跑显式 `RUN_SECONDS=30`；第一跑存 Golden，第二跑逐位对比。

| 指标 | 第 1 跑（基线） | 第 2 跑（Golden 对比） |
|------|-----|-----|
| `fps_interval` | 59.69 | 59.72 |
| `fps_wall` | 59.33 | 59.37 |
| `interval_p95_ms` | 17.06 | 17.05 |
| `hitch_rate_per_min` | 4.01（2 次 = 2 张快照） | 4.01（2 次 = 2 张快照） |
| `layout_base → final` | 1 → 5 | 1 → 5 |
| `layout_delta / height_changes` | 4 / 4 | 4 / 4 |
| `layout_isolation_violations` | 0 | 0 |
| shifts（neighbor/frame/chip） | 0 / 0 / 0 | 0 / 0 / 0 |
| `target_height_final` | 120 | 120 |
| marker `offset.y` | 36 | 36 |
| `boundary_skip / rerecord` | 7154 / 6 | 7158 / 6 |
| `paint_count_max` | 1790 | 1791 |
| `cpu_pct_avg`（ui/raster） | 22.62（2.17/20.45） | 22.63（2.18/20.45） |
| `rss_start → end` | 73.8 → 287.3 MB | 73.7 → 286.5 MB |
| `rss_slope_kb_per_min` | 65941 | 66350 |
| `vsync_source` | true | true |
| `cpu_fallback_ops` | 1792 | 1793 |
| 像素断言 | 10/10 | 10/10 |
| steady↔recover 静态掩码 | 0.0000% / 421120 px | 0.0000% / 421120 px |
| Golden | 首跑存基线 | `0.0000%` / 421120 px |
| `present_count` | 1790 | 1791 |

## Evidence notes

- **隔离怎么证**：`targetFrame` 设 `SetRelayoutBoundary(true)`；引擎 `MarkNeedsLayout` 是“标记父链，遇到父 boundary 停”（`pipeline_test.go` 的 `TestRelayoutBoundary_StopsBubble` 锁的就是 root→mid(boundary)→leaf）。因此改 `target.FixedHeight` 后只允许 `target` 与 boundary 父 `targetFrame` 变脏；root/body/top/legend/neighbor/stripes/hot/pulse 逐个查 `NeedsLayout()` 必须 false。四次变化 → `LayoutCount` 只 +4，最终静置 12s 不再涨。
- **像素怎么证**：不只查 HUD。target 蓝色、marker 橙色、slot 恢复 frame 底色、同父 chip 青色、neighbor cell、stripe、hot 中心、两相位灯、标签密度共 10 项；终帧与 steady 快照都读 PNG，不碰 GPU backbuffer。
- **Golden 怎么证**：静态掩码只盖顶栏、图例、neighbor、stripes、sibling chip；动态 target/HUD/phase lamp 不入掩码。两次 30s 逐位 `0.0000%`；steady↔recover 同一掩码也 `0.0000%`。
- **快照非空白**：早前 `SnapshotAsync` 里直接 `Context().Image()` 读到的是 present 后的白图；现改走 `PaintPresentTree` 重画 + `SavePNG`（与 C4 的 U21 批量快照同一入口），快照期间 ticker 暂停树变更，raster 重画读的是稳定树；steady 蓝灯/recover 绿灯像素断言锁死，空白图过不了门禁。
- **CPU fallback 是什么**：`cpu_fallback_ops≈1/frame`、`last_cpu_fallback=text:color-layout`，是静态标签文本的既有 CPU 落点，不是 R1 布局回调，也没有被门禁关掉；`gpu_fallbacks=0`。
- **内存口径**：30s 是正确性窗，`rss_start≈74MB → 286–287MB` 主要是 GPU 设备/纹理/图集 + 首帧快照的一次性启动分配，slope ≈66k/min 属启动爬坡观测；**不作泄漏结论**，泄漏看 R15/C10 300s soak（slope≈1.1–1.2k/min）。`rss_after_close` 与峰值同带，30s 窗也不冒充“关闭已释放”。
- **时间口径**：终帧 `frame_raster_ms≈66–70ms` 含快照重画/读回尖峰，fps 门禁只认 interval 指标；`paint_rate_per_sec≈59.3` 是 2s 暖机后 `paint_count_max/elapsed`，不是每帧 submit 数。
- **本窗不证明**：retained 局部 damage、纹理淘汰、跨进程/多窗释放、DPR/resize（R11/R14/R15/C10 各自负责）。

**关闭结论（2026-10-20）**：R1 两跑 30s 均 PASS；`layout_delta=4`、`dirty=0`、三个 sibling shift 全 0，像素 10/10、Golden 次跑 `0.0000%`、steady↔recover 静区 `0.0000%`，R1 转 ✅。
