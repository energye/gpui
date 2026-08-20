# ui_wr_c7_resize_dpr — C7 组合真窗（R11+R19+R3 集成）

W2 组合窗：两次尺寸/DPR 失效波 + 1px 对齐 + 嵌套 boundary。
只做集成回归，**不替代** R11/R19/R3 单能力窗（U6）。

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=15 go run ./examples/ui_wr_c7_resize_dpr
```

`RUN_SECONDS` 可覆盖；`<5` → `FAIL:` + `exit 1`（U16）。

## Window / Close duration

- 客户区 **1200×800**（U15），标题 `gpui ui_wr_c7_resize_dpr — 尺寸/DPR 失效 + 1px + 嵌套`
- 关闭用 **15s**（§2.5 C7 行；本地观察可加长）
- GPU 真窗；无 GPU/X11 → `FAIL: window open (needs_gpu_window)` + exit 1

## Visible effect

| Region | Expectation |
|--------|-------------|
| 左上 区 A（R11 RESIZE） | 深蓝面板 190×140（含内层色块+标签），~3s 时**变到 250×170**（Align 布局驱动，resize 自动跟随）→ 一波重录 → 回 skip（面板内容恢复静态） |
| 中上 区 B（R11 DPR） | 深绿面板 2×2 色格 + 标签，全程尺寸不变；~6s 全量失效 → 一波重录 → 回 skip |
| 左中 区 C（R3 嵌套） | 3 层嵌套 boundary（outer 深蓝 / mid 紫 / inner 内含 30×30 红块）；Steady/Recover 红块每帧变色（仅 inner 重录，outer/mid 静）；失效波窗口（3–7s）暂停 |
| 中下 区 D（R19 1px） | 左半 SNAPPED 绿网格（SnapLine 对齐、crisp）+ 右半 RAW 红网格（小数偏移、可能糊）+ SnapRect 橙色 1px 边框；DPR/尺寸变化时重画 |
| 右上 区 E（静态） | 3×3 色格阵 + 标签，全程静止（skip 累积） |
| 右下 HOT | 26×26 每帧变色（非 boundary live paint，稳态 rerecord 噪声为零） |
| 顶部 TopBar | 能力 ID + 当前相位（Steady/Resize-Spike/DPR-Spike/Recover） |
| 底部 HUD | 实时 fps/p95/policy + `skip=/rr=/inv=/dpr=` + `wave1=/wave2=/t=`；绿=达门禁趋势，红=破线预警 |

## Gates

门禁 = ∪(各 R 门禁)（§3.1.1）：

| 来源 | Gate | 硬 FAIL 线 |
|------|------|-----------|
| R11 | `cache_invalidations` | `>=1`（DPR 全量失效计数；`MinCacheInvalidations`） |
| R11 | `resize_wave_rerecord` | `>=1`（~3s 尺寸变更后 0.8s 内 BoundaryRerecord 增量） |
| R11 | `dpr_wave_rerecord` | `>=1`（~6s 全量失效后 0.8s 内增量） |
| R11 | `steady_rr_frame` | `==0`（4.0s 时单帧 FrameRerecord——R3 内层 hot 在 3–7s 暂停，失效区必须回 skip） |
| R19 | `snap_ok` | `SnapCoord(1.3,dpr)` 必须是 1/dpr 网格整数倍（设备像素对齐） |
| R3 | `boundary_skip` | `>=3`（outer/mid + 静态色格 skip 累积） |
| R3 | `boundary_rerecord` | `>=2`（inner 每帧 + 失效波重录） |
| 持续 tick | `fps_interval` | `>=55`（`RequirePersistentFPS`，§2.2.2 正确性+持续 tick 窗） |
| 持续 tick | `interval_p95_ms` | `<=22` |
| §2.2 | 全族 A–J | wrgate.BuildReport 必采；缺失字段 FAIL（schema 检查） |

诚实性说明（README 契约）：

- `wave1`/`wave2` 是 `BoundaryRerecord` 累计差分（触发时刻采样 → +0.8s 采样），诚实测量"失效造成的重录增量"；`peak1/peak2`（单帧 FrameRerecord 峰值）仅观测（tick/paint 交错可能漏采峰帧）。
- `steadyRR==0` 依赖 R3 内层 hot 在失效波窗口（3–7s）暂停——这是相位脚本的一部分（波检测窗口静态化），Recover 后恢复每帧变，R3 语义不缩水。
- `vsync_source` 如实输出；fallback 禁止宣称锁 60Hz。
- `cpu_ui_pct`/`cpu_raster_pct` 必采（持续 tick 窗禁双 0）。
- 15s 正确性/集成窗：`rss_slope` 仅必采、默认不 FAIL（§2.2.4 短窗允许，`slope_gate=off` 语义同 R4b/R5/R11 先例）。

## 集成验证（§3.1.2 C7 行）

| 跨能力边界 | 本窗如何证明 |
|-----------|-------------|
| 一波 rerecord 后回 skip | wave1/wave2≥1（失效重录发生）+ steadyRR==0（回稳后单帧零重录）+ skip 持续累积 |
| 1px 线不糊 | SnapLine/SnapRect 网格 + SnapCoord 网格对齐断言（snap_ok）；DPR 变化重画 |
| 缓存正确重建 | 两次失效（尺寸/DPR）各自恰好一波 rerecord，随后全部静态区回 skip |
| 内脏不外溢（R3 集成） | 嵌套 inner hot 每帧变只重录 inner，outer/mid skip（BoundaryCache tryReplay 语义与失效波互不干扰） |
