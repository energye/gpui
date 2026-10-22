# ui_wr_r3_boundary — R3 嵌套 Boundary 缓存（三证据 ✅v2）

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

# 首次播种 Golden（SEEDED，不算 PASS）
GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=10 go run ./examples/ui_wr_r3_boundary -auto-only

# 关闭证据：必须再连跑两次，均 0.0000%
RUN_SECONDS=10 go run ./examples/ui_wr_r3_boundary -auto-only
RUN_SECONDS=10 go run ./examples/ui_wr_r3_boundary -auto-only

# manual：默认无限运行；-manual-seconds 15 跑完 Steady→Spike→Recover 自动关
go run ./examples/ui_wr_r3_boundary
go run ./examples/ui_wr_r3_boundary -manual-seconds 15
```

## Window / Close duration

- 窗口 **1200×800** 逻辑像素，X11/GUI 真窗，GPU 必需。
- R3 关闭时长 **10s**（§2.5）；gate 档 `RUN_SECONDS < 10` 直接 FAIL。
  `RUN_SECONDS>=5` 只是通用最低线，不能拿 5s 关 R3。
- manual 档默认无限运行，手动关窗退出；`-manual-seconds 15` 用于三相位人工观察。
- gate 档冻结在 **Steady** 相位；manual 档跑完整 Steady→Spike→Recover。

## 布局

```text
+--------------------------------------------------------------+
| TopBar 48px: R3 嵌套Boundary缓存 — 内脏不外溢                 |
+----------+---------------------------------------------------+
| Legend   | Body (284,60) 904×656                             |
| (12,60)  |  nest1 (304,80): outer 240² → mid 180² → inner      |
| 260×656  |    120² → hot 60²                                  |
| 9 行色块 |  nest2 (584,80): outer2 180² → inner2 100² → hot2   |
| + 文字   |  deep (804,360): deep 200² → d2 150² → d3 100²      |
|          |  4×4 色格 (804,80) 40²/格，8 个静态标签 (304,360)  |
+----------+---------------------------------------------------+
| HUD (0,728) 72px: phase/fps/p95/skip/rr/presents/门禁         |
+--------------------------------------------------------------+
```

- **8 个内容 RepaintBoundary**：outer/mid/inner、outer2/inner2、deep/d2/d3；HUD 自身另算 1 个，`boundary_count=9`。
- **嵌套深度 3**：nest1 三层、deep 链三层；`boundary_max_depth=3`。
- **18 个静态标签**：outer L1–L3（3）+ mid（1）+ inner（1）+ nest2（2）+ deep 链（3）+ 4×4 网格下 8 个（8）。
- **16 个静态色格**：Body 内非 boundary 密集色块，用于“边界脏时静内容不闪”的像素证据。
- Legend 9 行，每行有色块 + 文字（`ColorAt` + `LabelAt`），说明颜色编码和预期行为。

## 可见效果

| # | 区域 | 位置（窗口绝对坐标） | 内容 | 预期效果 |
|---|------|----------------------|------|----------|
| 1 | nest1 三层嵌套 | (304,80) outer 240²；(334,110) mid 180²；(364,140) inner 120² | 三层 RB + 每层背景/静态块/标签 + inner 内 hot 红块 (394,170) 60² | Steady 只有 inner/hot 变；outer/mid 与兄弟不闪 |
| 2 | nest2 兄弟嵌套 | (584,80) outer2 180²；(624,120) inner2 100²；hot2 (649,145) 50² | 独立第二 nest + 静态块/标签 | Steady 蓝块随 hot2 变；Spike（nest1 outer 脏）时 nest2 不重录、不闪 |
| 3 | deep 深链 | (804,360) deep 200²；(829,385) d2 150²；(854,410) d3 100² | 深度 3 静态链 + 标签/色块 | 全程静；只贡献 boundary_count/max_depth，不参与 dirty |
| 4 | 静态 4×4 色格 | (804,80) 起，40²/格，间距 55 | 16 个非 boundary 色格 | 任一相位都不闪 |
| 5 | 静态标签 + Legend | Body 左下 + 左栏 | 18 个标签；Legend 9 行色块+文字 | 文字墨迹区域被像素探针覆盖；Golden 覆盖 header/legend/body |
| 6 | 相位脚本 | — | gate：Steady 冻结；manual：Steady 2s → Spike 3s → Recover 5s+ | gate 前 60 tick 做确定性 hot/hot2 颜色变化后保持最终色（保证 10s 快照逐位稳定）；manual Steady 红/蓝块动，Spike 只 outer 背景变，Recover 无内容脏 |
| 7 | HUD | (0,728) 72px | phase / fps / p95 / skip / rr / presents / 门禁 | 每 ~0.1s 刷新；干净层 `skip` 持续涨，脏层 `rr` 只在 dirty 时涨；Golden 排除 HUD |

### 判断标准

- `boundary_skip > 0`：干净 boundary 走 Replay，不是每帧全树重录。
- `boundary_rerecord > 0`：真正脏的 inner boundary 会重录；不是 0/0 假绿。
- `boundary_count/depth` 达标：场景确实有 8 个内容 RB、深度 3，不是单层假嵌套。
- Spike 时只有 nest1 outer 重录；nest2/deep/grid 的像素不变。
- Recover 期内容无脏，`skip` 继续涨 = 纯 Replay 帧成立。
- 像素/Golden 必须与逻辑指标同时绿；只有 JSON 绿不算关窗。

## Gates

### 逻辑与能力门禁

- `presents >= 1`、`paint_count > 0`；`layout_count = 1` 为观测（初始化布局一次），不作为 R3 门禁。
- `rr_steady > 0`、`rr_recover = 0`，且 `rr_steady + rr_spike + rr_recover == boundary_rerecord`。
- gate 档额外要求 `rr_spike = 0`；manual 档进入 Spike 相位后要求 `rr_spike > 0`。
- `boundary_skip >= 3`、`boundary_rerecord >= 2`、`boundary_count >= 3`、`boundary_max_depth >= 2`。
- `present_policy = full_paint`（W1 FullPaint 主路径，显式钉住，不继承 W6 retained 默认）。
- `static_labels >= 8`（实际 18）。
- `cpu_pct_avg > 0` 且 `cpu_ui_pct`/`cpu_raster_pct` 至少一路 > 0。
- `cpu_fallback_ops = 0`；`vsync_source` 非空。
- gate 档额外要求 `phase = Steady`（冻结相位）。

### 帧时（§2.2.2 正确性+持续 tick）

- 运行 10s（≥5s 才能判），有效 FPS `fps_interval >= 55`。
- `interval_p95_ms <= 22ms`；`hitch_count` 只观测，R3 不设 hitch 硬门禁。
- `target_hz = 60`，`vsync_source=true`。

### 像素与 Golden（U21）

- **像素 33/33**：26 个颜色探针（各层背景、静态块、hot/hot2、5 个网格格、body/legend/top 底色）+ 7 个文字墨迹区域。
  - 颜色容差 0.06；墨迹区域按“与背景色不同的像素数”判定，最小 6–8 px。
  - 探针坐标按 Body 原点锚定，gate 档不 resize，坐标固定。
- **Golden 逐位 0 容差**：mask = header `1200×48` + legend `260×656` + body `904×656` = **821184 px**。
  - HUD 不计入（计数器每 ~0.1s 刷新）。
  - 两次非 seed 连跑均 `0.0000%`。
- manual 档动画中不做像素/Golden 断言：JSON 输出 `pixel_evidence=manual_not_asserted`、`pixel_golden_diff_pct=null`、`pixel_probes_ok/total=0/0`，避免 0/0 冒充通过。

### CPU / 内存 / GPU / 依赖

- CPU：短窗（10s）只必采并记录尖峰，不设 >85% 硬门禁；`cpu_pct_avg`/`cpu_ui_pct`/`cpu_raster_pct` 均非零。
- 内存：10s < `rss_warmup_sec=15`，`rss_slope_valid=false`；**不能据此下泄漏结论**。
  - 原始 `rss_slope_kb_per_min` 照常输出（含启动期，数值大是正常现象）。
  - `rss_slope_steady_kb_per_min` 在 `rss_slope_valid=false` 时按 `wrgate` 既有测试不输出（无效稳态斜率不得声称数值），README 与 `rss_slope_steady_note` 说明原因。
- GPU：`gpu_ops` 累计观测；`cpu_fallback_ops=0`，无回退。
- 图/文缓存：R3 不是 R9/R10，`g_metrics=skipped` 并写原因；`measure_cache_hit/miss` 只观测，不设门禁。
- 依赖：`ui` 无 cgo（仅 doc 注释提到 `import "C"`）；`ui` 非测试代码无直引 `gpu/`；`go vet ./examples/ui_wr_r3_boundary` 通过。
- 不降画质：场景密度、分辨率、full_paint、阈值均未为过门禁而降低。

## U20 六维实现点

| 维度 | 本窗实现 |
|------|----------|
| `impl_correctness` | nest1 outer→mid→inner、nest2 outer2→inner2、deep→d2→d3 三套独立缓存；Steady 只脏 inner，Spike 只脏 nest1 outer，Recover 无内容脏 |
| `impl_dirty` | Steady：hot/hot2 `MarkNeedsPaint`；Spike：只 `outer.Background` + `MarkNeedsPaint`；Recover：不脏内容；HUD 是独立 boundary |
| `impl_cache` | 干净层累加 `boundary_skip`，脏 inner 累加 `boundary_rerecord`；18 标签、16 色格、deep 链保持缓存 |
| `impl_edge` | 3 层 nest1、3 层 deep、兄弟 nest2、16 密集色格、18 标签；resize 重新摆 shell |
| `impl_fail` | skip=0 / rerecord=0 / count<3 / depth<2 / 像素不符 / Golden≠0 / fps<55 / p95>22 / policy≠full_paint → FAIL |
| `impl_visible` | HUD 显示 skip/rr/phase；manual 红/蓝块与 outer 背景肉眼可见；Legend 色块；gate 像素/Golden 证据 |

## 实测证据

### gate 两连（2026-10-22，最终代码，非 seed）

| 指标 | 第 1 次 | 第 2 次 |
|------|---------|---------|
| `presents` / `paint_count` | 600 / 599 | 600 / 599 |
| `boundary_skip` | 4682 | 4682 |
| `boundary_rerecord` | 126 | 126 |
| `boundary_count` / `max_depth` | 9 / 3 | 9 / 3 |
| `fps_interval` | 60.00 | 60.02 |
| `interval_p95_ms` | 17.29 | 17.30 |
| `hitch_count` | 0 | 1（观测，R3 不设 hitch 硬门禁） |
| `cpu_pct_avg` / `ui` / `raster` | 25.6 / 2.22 / 23.36 | 25.3 / 2.16 / 23.09 |
| `gpu_ops` / `fallback` | 51603 / 0 | 51603 / 0 |
| `time_to_first_present_ms` | 75.2 | 74.8 |
| 像素 | 33/33 | 33/33 |
| Golden | **0.0000% / 821184 px** | **0.0000% / 821184 px** |
| 逻辑 | 17/17 | 17/17 |

### manual 15s（-manual-seconds 15）

- `presents=899`、`paint_count=898`、`skip=6943`、`rr=249`、`count=9`、`depth=3`。
- `phases_seen=["Steady","Spike","Recover"]`，`phase_mode=animated`。
- `fps_interval=59.95`、`p95=17.20ms`、`hitch=0`。
- `pixel_evidence=manual_not_asserted`、`pixel_golden_diff_pct=null`、`0/0`；逻辑 16/16（manual 无 `gate_phase_frozen`/`gate_rr_spike_zero`，改为 `manual_rr_spike_positive`）。
- manual 档 `SnapshotPath` 为空，不写 gate 快照；`snapshot_png=""` + `snapshot_note` 说明，跑完 manual 后 `testdata/showcase_r3_boundary.png` 哈希不变。

### 基线文件

- 当前 gate 快照：`testdata/showcase_r3_boundary.png`（GPU readback，不是窗口截图）。
- Golden 基线：`testdata/showcase_r3_boundary_base.png`。
- 首次 `GPUI_ACCEPT_GOLDEN=1` 只播种基线，输出 `SEEDED`，不算 PASS；必须再跑两次 0.0000%。

## 已知边界

- 10s 短窗只证明功能/帧时/像素，不证明无内存泄漏；泄漏结论需 ≥60s soak + `rss_slope_valid=true`。
- gate 前 60 tick 的 hot/hot2 颜色变化是**故意**制造真实内容脏，让 `boundary_rerecord>0`；之后 9 秒保持最终色，保证快照确定性。不是“冻结整个画面”，而是“冻结相位 + 固定最终帧”。
- `frame_raster_ms` 是本窗异步管线观测值（约 120ms 量级），R3 不设 B 族硬门禁；A 族 fps/p95 已绿。
- 本轮只在 X11 后端跑（JSON `backend=x11`）；Wayland 未跑，不能声称 Wayland 绿。
- 本窗是引擎能力的**原子验证**，没有组件/2.5D/视频生产消费者；不按生产接线标绿。
