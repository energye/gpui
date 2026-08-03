# ui_wr_r11_dpr — R11 DPR/尺寸缓存失效（W2 首次关闭）

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=15 go run ./examples/ui_wr_r11_dpr
```

## Window / Close duration

- 窗口：**1200×800**（`winW/winH`，JSON `client_px=1200x800`）
- 关闭用时长：**15s**（§2.5；观察窗 30s）
- `RUN_SECONDS<5` → `FAIL: RUN_SECONDS must be >= 5 ... (U16)` + exit 1
- GPU 真窗（X11 + wgpu）；无 GPU 环境 → `FAIL: window open (needs_gpu_window)` + exit 1

## Visible effect

| Region | Expectation |
|--------|-------------|
| RESIZE-A（左上，Align 0.04/0.06） | 蓝底嵌套 2 层 boundary；~3s 尺寸 190×140→250×170（布局驱动）→ **一波 rerecord（精确 1 次）→ 回 skip**，内容不错位 |
| DPR-B（上中） | 绿底 2×2 色格 boundary；~6s `InvalidateBoundaryCache()` → **全量一波 rerecord（精确 11 次）→ 回 skip** |
| NEST-C（左下） | 3 层嵌套 boundary（outer→mid→leaf）；全程静态，skip 累加 |
| DENSE-D（右下） | 4×4 色格 + 8 标签 + 1 图（40×40 绿块）；全程静态，skip 累加 |
| HOT（右上） | 26×26 每帧变色（**live paint，非 boundary**——零 rerecord 噪声，稳态帧 rerecord 精确为 0） |
| HUD（底栏） | R11 相位 / fps / p95 / skip / rr / inv / wave1 / wave2 实时可见 |

## Gates

§2 主表（L81）：变更后 rerecord **一波**再回稳；无残影、不错位。
§2.6（L347）：`cache_invalidations≥1` + `boundary_rerecord≥1` + `boundary_skip≥1`。

| Gate | 阈值 | 判定 |
|------|------|------|
| present_count | ≥1 | `MinPresents: 1` |
| present_policy | full_paint（正确性窗） | `RequireFullPaintPolicy` |
| boundary_skip | ≥1（静态区 Steady 期 Replay） | `MinBoundarySkip: 1` |
| boundary_rerecord | ≥1（变更波） | `MinBoundaryRerecord: 1` |
| cache_invalidations（ability_extra） | ≥1（~6s DPR 失效） | `MinCacheInvalidations: 1`（`EvaluateRetainedExtras` 自动判定） |
| resize_wave_rerecord | =1（~3s 尺寸变更后 0.8s 采样增量，**精确一波**） | 自定义检查 |
| dpr_wave_rerecord | ≥1（~6s 失效后 0.8s 采样增量 = 全量 11 次） | 自定义检查 |
| steady_rr_frame | =0（变更后 1s 采样单帧——**回稳证明**，无任何 rerecord） | 自定义检查 |

**波计数无噪声设计**：HOT 热点为非 boundary live paint（不贡献 rerecord/skip），
稳态每帧 `FrameRerecord=0`——`resize_wave_rerecord` 精确等于尺寸变更的 1 次重录、
`dpr_wave_rerecord` 精确等于全量失效的 boundary 数（实测 11）。`*_wave_peak_frame`
为观测字段（per-frame 峰值，tick 与 paint 交错时可能漏采，**不设门禁**）。

### §2.2 全族

- **族 A**：`fps_wall/interval_avg/p50/p95/p99`、`hitch_count/rate`、`vsync_source`（fallback 不宣称锁 60）、`target_hz` 全采。15s 正确性窗：fps 预算 ≥55（持续 tick 类），如实采集不偷放。
- **族 C**：`layout_count/paint_count/damage_*`、`present_mode/policy`、能力专用 `boundary_skip/boundary_rerecord`。
- **族 D/E**：`cpu_pct_avg/cpu_ui_pct/cpu_raster_pct`、`rss_start/end/peak/slope_kb_per_min` 全采。
- **族 F**：`gpu_ops/cpu_fallback_ops/last_cpu_fallback/frame_flushes` 全采。
- **族 G**：`g_metrics=skipped`（非文本能力窗）+ `unavailable_reason`。
- **族 J**：无 cgo、`ui` 无 import gpu、本窗不降画质（HUD 走 wrkit LiveHUD，独立 boundary band）。

### U20 实现点六维

| 维度 | 本窗回答 |
|------|----------|
| 正确性 | 尺寸/DPR 变更后缓存正确重建（resize 波=1、DPR 波=11 且 steadyRR=0 回稳），静态区无残影、不错位 |
| 脏区 | skip 累加（静态区每帧 Replay，9877+）；rerecord 只发生在两次变更波（lifetime 精确 12 次） |
| 缓存 | 尺寸 fingerprint miss（RESIZE-A 波=1）+ 全量 Clear（DPR-B 波=11）；一波重录后再次 skip（steadyRR=0） |
| 边界条件 | 3 层嵌套（NEST-C）、图/文/色格混排（DENSE-D）、4 独立 region + HOT live paint 隔离 |
| 失败模式 | skip=0（静态区从未 Replay）、rr=0（无波）、inv=0（失效未触发）、steadyRR≠0（未回稳）→ 各自 FAIL 断言 |
| 窗内如何看出 | HUD skip/rr/inv/wave1/wave2 实时；HOT 每帧闪；变更波由 HUD rr 数字跳变可见 |

### 诚实性声明

- `resize_wave_rerecord` / `dpr_wave_rerecord` 是**变更触发后 0.8s 的 BoundaryRerecord 累计增量**（真实观测窗口）；HOT 为非 boundary 使波计数无噪声（稳态 rr=0），实测 resize 波=1（区域自身重录一次）、DPR 波=11（全部 boundary 重录）。
- `steady_rr_frame` 是变更后 1s 的单帧采样（=0 证明回稳）。
- 15s 正确性窗：`rss_slope_kb_per_min` 不设门禁（§2.2.4 短窗允许 slope_gate=off，JSON 如实输出）。
- HUD 是**独立 RepaintBoundary band**（wrkit LiveHUD），不污染静态 boundary skip 统计。
- `resize_wave_peak_frame` / `dpr_wave_peak_frame` 为观测字段（tick/paint 交错可能漏采峰值帧），不设门禁。
