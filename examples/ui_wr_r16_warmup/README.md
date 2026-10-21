# ui_wr_r16_warmup — W0 R16 首帧/WarmUp/恢复

## 0. 这扇窗证明什么

R16 管的是**首帧有没有内容、WarmUp 是否真的全清全画、首帧耗时是否在预算内、恢复重画后内容是否还活着**。

原窗只有逻辑门禁（`warmup` / `t2fp` / `first_present_paint_count` / 恢复后 paint 增长），没有像素断言和 Golden。本版按 `docs/UI_PIXEL_ASSERTION_STANDARD.md` 补成 **pre / post 两档**：

| 档 | 视觉状态 | 证明 |
|----|----------|------|
| **pre** | 冻结首帧内容：Steady 相位、不触发恢复 | 首帧的静网格、12 条文字、`FIRST FRAME CONTENT OK` 横幅、红色动块都在；终帧与首帧内容一致 |
| **post** | 固定触发一次全树恢复并保持到终帧 | 恢复后静网格/文字仍在，恢复横幅和白色动块可见，`paint` 继续增长 |

两档各自出**独立快照 + 独立像素探针 + 独立 Golden**。manual 档保留 Steady→Spike→Recover 动画和约 1 秒的恢复横幅，只做人工观察，不做像素/Golden 断言。

## 1. 运行

```bash
export DISPLAY=:0 LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
```

### 1.1 首次播种两档 Golden

```bash
GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=5 go run ./examples/ui_wr_r16_warmup -auto-only -gate-stage pre
GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=5 go run ./examples/ui_wr_r16_warmup -auto-only -gate-stage post
```

播种会把当前快照复制成：

- `testdata/showcase_r16_warmup_pre_base.png`
- `testdata/showcase_r16_warmup_post_base.png`

播种那一跑 `pixel_golden_total_px=0`、`pixel_golden_first_run=true`，**不算通过**；必须再各跑两次。

### 1.2 每档连跑两次（验收）

```bash
RUN_SECONDS=5 go run ./examples/ui_wr_r16_warmup -auto-only -gate-stage pre
RUN_SECONDS=5 go run ./examples/ui_wr_r16_warmup -auto-only -gate-stage pre
RUN_SECONDS=5 go run ./examples/ui_wr_r16_warmup -auto-only -gate-stage post
RUN_SECONDS=5 go run ./examples/ui_wr_r16_warmup -auto-only -gate-stage post
```

两档都要 `exit 0`，且各档两次 Golden 都是 `0.0000% over 821184 px`。

### 1.3 manual 人工观察

```bash
go run ./examples/ui_wr_r16_warmup -manual-seconds 15
```

manual 前确保没有 `RUN_SECONDS`（有则按 gate 档处理）；manual 不写快照、不跑像素/Golden；JSON 明确写 `pixel_evidence=manual_not_asserted`、`pixel_golden_diff_pct=null`、`pixel_probes_total=0`，避免把「没断言」伪装成 0/0 通过。

## 2. 窗口布局（1200×800，body 904×656）

| 区域 | 内容 |
|------|------|
| TopBar | `R16 首帧/WarmUp/恢复 — 首帧有内容·恢复不黑` |
| Legend | 8 行：WarmUp / 首帧有内容 / pre 冻结 / post 恢复 / 恢复不黑 / 像素 / Golden / gate 命令 |
| Body 左上 | 6×4 静态色格，每格 84×84，RepaintBoundary |
| Body 右上 | 12 条静态文字，3 列×4 行（`warmup 00`…`warmup 11`） |
| Body 右上 | `FIRST FRAME CONTENT OK` 首帧横幅，Open 前放置 |
| Body 右中 | 动块 120×120：pre 红色 @ (600,170)；post 白色 @ (700,480)；manual 随相位变位变色 |
| Body 左下 | `PHASE: …` 相位标签 @ (20,420) |
| Body 左下 | `STAGE: PRE/POST/MANUAL …` 阶段标签 @ (20,480) |
| Body 右中 | `RECOVERY OK - ALIVE` 恢复横幅 @ (600,420)，仅 post/manual 恢复段 |
| HUD | `t2f`、`p1`、stage、rec、paint/fps/p95 等；**不入 Golden** |

坐标均为 body-local。窗口 resize 走 `shell.Resize`；gate 档不 resize，像素坐标固定。

## 3. 逻辑门禁（13 项）

| # | 探针 | 判据 |
|---|------|------|
| 1 | `presents_positive` | `present_count >= 1` |
| 2 | `fps_interval_ge_55` | `fps_interval >= 55`（持续 tick） |
| 3 | `paint_count_covers_presents` | `paint_count > 0` 且 `>= presents*0.9` |
| 4 | `static_content_ge_8` | 静色格 24 + 静文字 12 = 36，且 `>= 8` |
| 5 | `hot_ticks_positive` | ticker 至少跑过一帧 |
| 6 | `present_policy_full` | `present_policy == full_paint` |
| 7 | `damage_area_full_paint` | `damage_area_px >= 1200*800*0.9`（FullPaint 语义） |
| 8 | `vsync_source_present` | `vsync_source` 非空 |
| 9 | `warmup_observed` | `warmup == true`（Open 时 `presentSyncFull` 真实执行） |
| 10 | `first_present_within_budget` | `0 < time_to_first_present_ms < 2000` |
| 11 | `first_present_has_content` | `first_present_paint_count > 0` |
| 12 | `cpu_fallback_zero` | `cpu_fallback_ops == 0` |
| 13 | 阶段探针 | **pre**：`recovery_triggered == false`；**post/manual**：`recovery_triggered == true` 且 `paint_at_recovery > 0` 且终帧 `paint_count > paint_at_recovery` |

任何一项 FAIL，gate 档 `exit 1`。

## 4. 像素探针

像素探针坐标为逻辑坐标，`SnapshotPath` 的 GPU readback 按 `dpr` 缩放；容差按 8-bit 量化和抗锯齿边界显式声明，不静默放水。

### 4.1 公共 9 点 + 2 文字墨量区域

| 探针 | 坐标（body-local） | 期望 | 容差 |
|------|--------------------|------|------|
| `static_00` | (62,62) | (0.15,0.50,0.30) | 0.06 |
| `static_10` | (154,62) | (0.25,0.50,0.30) | 0.06 |
| `static_13` | (154,338) | (0.25,0.50,0.30) | 0.06 |
| `static_32` | (338,246) | (0.45,0.50,0.54) | 0.06 |
| `static_51` | (522,154) | (0.25,0.50,0.42) | 0.06 |
| `grid_gap_bg` | (108,62) | body bg (0.10,0.11,0.13) | 0.05 |
| `body_bg` | (450,600) | body bg | 0.05 |
| `legend_bg` | legend + (20,620) | (0.11,0.12,0.15) | 0.05 |
| `top_bg` | (1000,24) | (0.12,0.14,0.18) | 0.05 |
| `static_label_ink` | (600,20) 60×12 | 非背景墨点 `>=10` | bg 0.04 |
| `first_banner_ink` | (600,116) 260×22 | 非背景墨点 `>=20` | bg 0.04 |

`static_*` 覆盖 5 个不同色格，含左上/左下/中/右上；`grid_gap_bg` 证明格间有缝；`body_bg`/`legend_bg`/`top_bg` 证明三个面板底色没被整片刷错；两个墨量区域证明「首帧横幅 + 静文字」真的画出了字，而不是只有色块。

### 4.2 pre 追加 3 点（合计 14 项）

| 探针 | 坐标 | 期望 | 容差 |
|------|------|------|------|
| `hot_red` | (660,230) | (0.95,0.20,0.20) | 0.06 |
| `spike_slot_bg` | (660,350) | body bg | 0.05 |
| `hot_bottom_bg` | (660,294) | body bg | 0.05 |

证明 pre 动块在 Steady 位置且是红色，Spike 槽位为空，动块下沿之外是背景。

### 4.3 post 追加 3 点 + 1 墨量区域（合计 15 项）

| 探针 | 坐标 | 期望 | 容差 |
|------|------|------|------|
| `hot_white` | (760,540) | (1.0,1.0,1.0) | 0.06 |
| `old_hot_slot_bg` | (660,230) | body bg | 0.05 |
| `hot_bottom_bg` | (760,604) | body bg | 0.05 |
| `recovery_banner_ink` | (600,420) 240×24 | 非背景墨点 `>=20` | bg 0.04 |

证明恢复后动块变成白色并落到 (700,480)，原 Steady 槽位已空，恢复横幅真的画出了字；同时公共静格/静文字探针仍全过，证明恢复后静态内容没被清掉。

## 5. Golden（逐位回归）

- 掩码：`header 1200×48 + legend 260×656 + body 904×656 = 821184 px`
- 排除：底部 HUD 72px（含实时计数，天然不逐位稳定）
- 容差：**零容差**，任一通道差 1 即 FAIL；尺寸不一致直接 FAIL
- 每档独立基线：`showcase_r16_warmup_pre_base.png` / `showcase_r16_warmup_post_base.png`
- 每档播种后必须再跑两次，两次都 `0.0000%` 才算过

### 5.1 门禁反证（2026-10-22）

把 post 基线临时拷到 pre 基线位，pre gate 实测 `diff=4.0806% over 821184 px`、`exit 1`；恢复 pre 基线后再跑 `0.0000%`、`exit 0`。这说明 Golden 门禁不是只写不判。

## 6. 首帧像素口径（重要，别误读）

本窗**没有单独抓第 1 帧的 readback**。pre 档的做法是：

1. 静网格、12 条文字、`FIRST FRAME CONTENT OK` 横幅、红色动块都在 `app.Open()` **之前**放置；
2. gate pre 冻结 Steady，且永不触发恢复，body 的可见状态从首帧到终帧不变；
3. 因此终帧快照与首帧内容一致（HUD 已排除）；
4. 首帧是否真的执行了全清全画，由 `warmup=true` + `first_present_paint_count=1` 直接观测，不靠像素猜。

这是「内容等价的冻结帧 + 首帧 paint 直接观测」的合证，不是拿终帧冒充首帧。post 档则用固定 tick 触发恢复并保持，专门证明恢复重画后内容仍活着。

## 7. 实测（2026-10-22，X11，GPU）

### pre（两次）

- `presents=300`、`paint=299`、`fps_interval=59.92–60.06`、`p95=17.18–17.28ms`
- `t2fp=86.2–88.3ms`、`first_present_paint_count=1`、`warmup=true`
- `recovery_triggered=false`、`recovery_repaint_ok=false`（pre 档不触发恢复，`recovery_na=true`）
- `logic=13/13`、`pixel=14/14`、`golden=0.0000% over 821184 px`（两连）

### post（两次）

- `presents=300`、`paint=299`、`fps_interval=59.94–59.98`、`p95=17.17–17.19ms`
- `t2fp=85.3–86.5ms`、`first_present_paint_count=1`、`warmup=true`
- `recovery_triggered=true`、`paint_at_recovery=29`、终帧 `paint=299`、`recovery_repaint_ok=true`
- `logic=13/13`、`pixel=15/15`、`golden=0.0000% over 821184 px`（两连）

### manual 15s

- `presents=899`、`paint=898`、`fps_interval=59.94`、`phase=Recover`
- `recovery_triggered=true`、`paint_at_recovery=60`、`recovery_repaint_ok=true`
- `logic=13/13`；`pixel_evidence=manual_not_asserted`、`pixel_probes_total=0`、`pixel_golden_diff_pct=null`

## 8. U20 实现点六维

| 维度 | R16 对应 |
|------|----------|
| 正确性 | WarmUp 全清全画首帧；静网格/文字/首帧横幅在 Open 前放置；post 恢复后仍存活 |
| 脏区 | `full_paint` 每帧全树重画，`damage_ratio=1` 属正确语义；恢复时一次 `InvalidateBoundaryCache` |
| 缓存 | 观测 `boundary_skip`/`boundary_rerecord`；R16 不把缓存命中作为门禁 |
| 边界 | pre/post 两档、固定 tick 恢复、manual 动画、resize、gate/manual 两模式 |
| 失败 | 缺 warmup、首帧无 paint、t2fp 超预算、恢复后 paint 不增长、像素/Golden 不符 → FAIL |
| 可见 | HUD 显示 t2f/p1/stage/rec；首帧横幅、恢复横幅、相位/阶段标签肉眼可见 |

## 9. 已知限界

- 5s 正确性窗，`rss_slope_valid=false`，`slope_gate=off`；短窗斜率不当泄漏结论，原始斜率仍输出。
- `g_metrics=skipped`，原因写在 JSON；R16 不把 measure cache 作为门禁。
- `frame_raster_ms` 末帧包含快照 readback，不能当稳态 raster 成本；本窗不设 p95 硬门禁。
- `depcheck=passed_no_cgo_no_ui_gpu_import` 由无 cgo import、无 `ui→gpu` 直引实测得出；本仓当前没有 `TestNoGPUImport` 测试文件（`go test ./ui -run TestNoGPUImport` 报 no tests to run）。复核命令：`grep -rn '^import "C"' ui/ --include='*.go'` 与 `grep -rn '^\s*"github.com/energye/gpui/gpu' ui/ --include='*.go'` 均无输出。
- 目前实测在 X11；Wayland 走 `/gpui-wayland-nested` 另跑，本文数据不代表 Wayland。
- manual 15s 是定时自动关闭的观察窗，**不等于常驻人工操作窗**；需要人工长时间看效果时，去掉 `-manual-seconds` 开窗。
- 两档各写自己的快照/基线；不要拿 pre 基线比 post 快照，反之亦然。
