# ui_wr_r12b_debug_repaint — R12b 重绘调试可视化（三证据 ✅v2）

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

# on 态门禁（8s；首次播种 Golden 才加 GPUI_ACCEPT_GOLDEN=1，seed 不算 PASS）
RUN_SECONDS=8 go run ./examples/ui_wr_r12b_debug_repaint -auto-only
RUN_SECONDS=8 go run ./examples/ui_wr_r12b_debug_repaint -auto-only

# off 态门禁：独立基线，绝不能复用 on 的 Golden
WR_DEBUG_REPAINT=0 RUN_SECONDS=8 go run ./examples/ui_wr_r12b_debug_repaint -auto-only
WR_DEBUG_REPAINT=0 RUN_SECONDS=8 go run ./examples/ui_wr_r12b_debug_repaint -auto-only

# manual：默认无限运行，手动关窗；-manual-seconds 15 自动跑完三相位
go run ./examples/ui_wr_r12b_debug_repaint
go run ./examples/ui_wr_r12b_debug_repaint -manual-seconds 15
```

## Window / Close duration

- 窗口 **1200×800** 逻辑像素，X11/GUI 真窗，GPU 必需。
- R12b 关闭时长 **8s**（§2.5）；gate 档 `RUN_SECONDS < 8` 直接 FAIL。
- gate 档冻结在 **Steady**；manual 档跑完整 **Steady（0–2s）→ Spike（2–5s）→ Recover（5s+）**。
- manual 档 `SnapshotPath` 为空，绝不覆盖 gate 提交的快照与 Golden 基线。

## 场景与布局

窗口仍是完整 wrkit Shell：TopBar 48px + Legend 260×656 + Body 904×656 + LiveHUD 72px。Body 内同时摆静态/热点/门禁活叶，避免“两色块”式简陋场景：

```text
+----------------------------------------------------------------------+
| TopBar 48px: R12b debug重绘 — magenta=live paint                     |
+----------+-----------------------------------------------------------+
| Legend   | Body (284,60) 904×656                                     |
| 260×656  |  4 静态 boundary 蓝块 (20,20) 100×80，间隔 130            |
| 8 行     |  cached 面板 (20,120) 520×190：8 条静态色条 + 标签        |
|          |  grid 面板 (20,330) 300×200：4×4 静态色格 + 标签          |
|          |  footer 面板 (20,550) 520×86：8 个静态色格 + 标签         |
|          |  guide 面板 (560,270) 320×300：文字说明                   |
|          |  hotA 橙块 (580,24) 110×110（boundary）                   |
|          |  hotB 绿块 (580,160) 80×80（boundary）                    |
|          |  gateLive 红叶 (720,24) 100×100（仅 gate，非 boundary）   |
+----------+-----------------------------------------------------------+
| HUD (0,728) 72px: phase/fps/p95/on/draws/skip/rr/门禁                 |
+----------------------------------------------------------------------+
```

- **gate 36 个静态色块 + 29 个静态标签**（含 gateLive 标签）；manual 无 gateLive 标签，为 36 色块 + 28 标签。4 蓝块、8 色条、16 格 grid、8 格 footer，另有面板/图例文字；不是裸 box。
- **10 个内容 RepaintBoundary** + HUD 边界 → `boundary_count=11`、`boundary_max_depth=1`。
- **gateLive 是普通叶子**：不在 boundary 内，FullPaint 每帧都会真正画它；gate 冻结 Steady 后颜色固定，所以叠加像素可逐位复现。
- **hotA/hotB 在 gate 中是干净 boundary 对照**：只首帧记录，之后 Replay，保持原始橙色/绿色，不能出现 magenta。
- **静态块/面板**同样是干净 boundary：首帧记录后 Replay；`boundary_skip` 持续增长但像素不动。

### gate 与 manual 的区别

| 档位 | 相位 | hotA/hotB | gateLive | 像素/Golden |
|------|------|-----------|----------|-------------|
| gate on | Steady 冻结 | 干净对照，不标脏 | 每帧 FullPaint，on 时叠加 | 断言 + 快照 |
| gate off | Steady 冻结 | 同上 | 每帧 FullPaint，off 时不叠加 | 独立断言 + 独立快照 |
| manual | Steady→Spike→Recover | Steady hotA 每 tick 改色；Spike hotA+hotB 每 tick 改色；Recover 停止标脏 | 不存在 | 只做逻辑/人工观察，不断言动画像素 |

## 可见效果

| # | 区域 | 预期效果 |
|---|------|----------|
| 1 | gateLive 红叶 | on：中心为固定 magenta 半透明混合；off：原始红色，全窗无 magenta |
| 2 | hotA 橙块 / hotB 绿块 | gate 全程原始橙/绿（干净 boundary 的 Replay 不叠加） |
| 3 | 静态蓝块 / 色条 / grid / footer / guide | 全程原始颜色，首帧后不再出现叠加 |
| 4 | manual Steady | hotA 颜色逐 tick 变化，肉眼可见真实重绘；静态区不动 |
| 5 | manual Spike | hotA + hotB 同时逐 tick 改色；`draws/rr` 持续增长 |
| 6 | manual Recover | 停止标脏；第一帧可能带上一帧残留（2 次），稳定后 `recover_draws_after=0` |
| 7 | `WR_DEBUG_REPAINT=0` | 任何相位都不画 magenta；`debug_repaint_draws` 恒 0 |
| 8 | HUD | 显示相位、fps/p95、on/off、draws、skip/rr、门禁；Golden 排除 HUD 带 |

### 判断标准

- on gate：`debug_repaint_on=true` 且 `debug_repaint_draws>=1`；gateLive 中心像素是精确 sRGB 混合色，hot/静态区是原始色。
- off gate：`debug_repaint_on=false` 且 `debug_repaint_draws==0`；gateLive 中心是原始红，任何区域都无 magenta。
- gate 冻结后静态 boundary 只 Replay：`boundary_skip` 随帧数增长，`boundary_rerecord` 只在首帧记录一波。
- manual：三相位都出现；Spike 的 draws/rr 明显增长；Recover 稳定后不再增长。
- 像素、Golden、逻辑三类证据必须同时绿；只有 JSON 绿不算关窗。

## Gates

### 逻辑与能力门禁

- `debug_repaint_on=true` → `debug_repaint_draws >= 1`；`debug_repaint_on=false` → `debug_repaint_draws == 0`。
- `present_policy == full_paint`（W1 显式钉住，不继承 W6 retained 默认）。
- `boundary_count == 11`（10 内容 boundary + HUD），`boundary_max_depth == 1`。
- `boundary_skip >= 1`、`boundary_rerecord >= 1`；gate 还需 `static_clean/hotA_clean/hotB_clean/gate_live_clean == true`。
- gate 相位固定 Steady，`draws_spike=0`、`draws_recover=0`、`rr_spike=0`、`rr_recover=0`。
- manual 逐相位检查 `draws_steady/spike/recover` 与 `rr_*`；Spike 必须 >0，Recover 稳定后 `recover_draws_after=0`、`recover_settled=true`。
- `draws_steady+draws_spike+draws_recover == debug_draws_after`；`rr_steady+rr_spike+rr_recover == boundary_rerecord`。
- `presents >= 1`、`paint_count > 0`、`layout_count > 0`、`frame_flushes >= 1`。

### 帧时（§2.2.2 持续 tick）

- 运行 8s（等于 R12b 关闭用时长），有效 FPS 用 `fps_interval >= 55`；`fps_wall` 只作 fallback/参考（含启动/收尾开销）。
- `interval_p95_ms <= 22ms`；`target_hz=60`；`vsync_source=true`。
- 8s 正确性窗不设 `hitch_rate` 硬门禁；实测 `hitch_count=0`。

### 像素与 Golden（U21）

- **像素 36/36**：gateLive on 混合色 / off 原始色；hotA/hotB/4 蓝块/色条/grid/footer/guide/body/legend/top 原始色；文字用墨迹区域计数，避免单点抗锯齿误判。
- **Golden 逐位 0 容差**：mask = header `1200×48` + legend `260×656` + body `904×656` = **821184 px**；HUD 不计入。
- on/off 各自独立快照与基线：
  - on：`testdata/showcase_r12b_debug_on.png` vs `showcase_r12b_debug_on_base.png`
  - off：`testdata/showcase_r12b_debug_off.png` vs `showcase_r12b_debug_off_base.png`
- 两次非 seed 连跑均 `0.0000%`；首次 seed 输出 `SEEDED`，不算 PASS。
- gate 快照是 GPU readback，不是窗口截图；快照 repaint 也带 `debugRepaint`，否则 on/off PNG 会静默丢叠加。

### CPU / 内存 / GPU / 依赖

- CPU：8s 短窗只观测不设 >85% 硬门禁；`cpu_pct_avg`/`cpu_ui_pct`/`cpu_raster_pct` 均非零，UI+raster ≈ 进程 CPU。
- 内存：8s < `rss_warmup_sec=15`，`rss_slope_valid=false`；**不能据此下泄漏结论**。
  - 原始 `rss_slope_kb_per_min` 照常输出（含启动期，数值大是正常现象）。
  - `rss_slope_steady_kb_per_min` 在 `rss_slope_valid=false` 时按 `wrgate` 既有测试**不输出**（无效稳态斜率不得声称数值）；`rss_slope_steady_note` 说明“不能结论，不是没泄漏”。
- GPU：`gpu_ops` 是**累计**路由计数，不是每帧 submit；`cpu_fallback_ops=0`，无回退。
- 图/文缓存：R12b 不是 R9/R10，`g_metrics=skipped` 并写原因；`measure_cache_hit/miss` 只观测，不设门禁。
- 依赖：`ui` 无 cgo（仅 doc 注释提到 `import "C"`）；`ui` 非测试代码无直引 `gpu/`；`TestUIDoesNotImportGPU` 通过。
- 不降画质：gate 36 静态色块 + 29 标签（manual 36 + 28）+ 4×4 grid + 多个 boundary 面板；FullPaint、分辨率、阈值均未为过门禁而降低。

## U20 六维实现点

| 维度 | 本窗实现 |
|------|----------|
| `impl_correctness` | `PaintContext.NoteDebugRepaint` 在真实 live color-box paint 内画叠加；干净 boundary 的 Replay 在叠加前返回；gateLive 是普通叶子，FullPaint 每帧真画，on/off 像素确定；hot/static 保持原始色 |
| `impl_dirty` | gate 冻结 Steady，静态/hot boundary 不标脏；manual Steady hotA 每 tick 改色，Spike hotA+hotB 每 tick 改色，Recover 停止标脏；Spike 的 draws/rr 来自真实内容变化，不是空刷 |
| `impl_cache` | 静态块/面板是 RepaintBoundary：首帧记录，之后 Replay 累计 `boundary_skip`；hotA/hotB 在 gate 是干净对照；内容变化才重录，gate `rr=10` 只是首帧记录 |
| `impl_edge` | on/off 独立快照+独立 Golden；manual 不写 gate 快照；快照 repaint 继承 debug 开关但不累加 live 计数；全量 paint 与 retained packet 的脏标记消费分开 |
| `impl_fail` | on draws=0 / off draws>0；gateLive 像素不是 on 混合/off 原始；hot/static 出现叠加；Golden≠0 或 mask px 不符；policy≠full_paint；fps<55 或 p95>22；逻辑探针失败 → FAIL |
| `impl_visible` | on：gateLive 中心 magenta、其余原始色；off：全窗无 magenta；manual：Steady/Spike 热点肉眼可见，Recover 很快归于静止；HUD 可读 |

## 引擎修复（R12b 收口时发现）

全量 paint 路径原本在 live paint **之前**无条件调用 `PipelineOwner.ConsumeNeedsPaint()`。这会先清掉刚 `MarkNeedsPaint()` 的边界标记；若内容键没变，`BoundaryCache` 就把这次请求当成 clean Replay，既不会真实重录，也看不到 R12b 叠加（小探针实测：先 consume 再画，`draws` 不涨、`rr=0`；不 consume 再画，`draws` 涨、`rr=1`）。

修法按“只在需要的地方消费”：
- **retained textured packet 路径**：`BuildFramePacket` 已把本帧内容记入 packet，下面不跑 live paint，继续在 UI 线程消费脏标记。
- **full_paint / force / recovery 路径**：不提前消费，保留到 raster job 的 `FlushPaint` 画完自然清脏。
- 回归：
  - `ui/rendering/debug_repaint_test.go: TestDebugRepaint_DirtyUnchangedContentLivePaints`（脏但内容不变必须 live paint + `rr=1`）。
  - `ui/embedder/paint_consume_test.go: TestConsumePacketPaintFlagsOnlyRetained`（full paint 保留脏标记，retained 才消费）。

## 实测证据

### gate 两连（2026-10-22，最终代码，非 seed）

| 指标 | on 第 1 次 | on 第 2 次 | off 第 1 次 | off 第 2 次 |
|------|------------|------------|-------------|-------------|
| `presents` / `paint_count` | 480 / 479 | 479 / 478 | 480 / 479 | 480 / 479 |
| `fps_interval` | 60.01 | 59.96 | 59.98 | 59.99 |
| `interval_p95_ms` | 17.27 | 17.19 | 17.26 | 17.25 |
| `hitch_count` | 0 | 0 | 0 | 0 |
| `boundary_skip` | 4800 | 4790 | 4800 | 4800 |
| `boundary_rerecord` | 10 | 10 | 10 | 10 |
| `debug_repaint_draws` | 518 | 517 | **0** | **0** |
| `cpu_pct_avg` / `ui` / `raster` | 26.61 / 2.75 / 23.86 | 26.64 / 2.80 / 23.83 | 26.14 / 2.71 / 23.42 | 26.74 / 2.80 / 23.95 |
| `gpu_ops` / `fallback` | 45161 / 0 | 45067 / 0 | 44643 / 0 | 44643 / 0 |
| `time_to_first_present_ms` | 81.2 | 78.6 | 79.9 | 81.5 |
| `frame_raster_ms` / `live_frame_raster_ms` | 128.4 / 2.17 | 123.5 / 1.67 | 123.8 / 1.91 | 128.7 / 1.85 |
| 像素 | 36/36 | 36/36 | 36/36 | 36/36 |
| Golden | **0.0000% / 821184 px** | **0.0000% / 821184 px** | **0.0000% / 821184 px** | **0.0000% / 821184 px** |
| 逻辑 | 25/25 | 25/25 | 25/25 | 25/25 |

- `boundary_skip = 10 个内容 boundary × 帧数`：4800（480 帧）/ 4790（479 帧）；`boundary_rerecord=10` 是首帧一次性记录，不是每帧重录。
- on `debug_repaint_draws` 每次 = 帧数 + 首帧 38 个静态/hot 色块一次性 live paint：518（480 帧）/ 517（479 帧）；`draws_at_warmup=48`（38 + 前 10 帧 gateLive），`draws_after=470/469`（= 后续 gateLive 帧数）。off 全为 0。
- on 的 gateLive 中心是精确 sRGB 混合（overlay alpha 0.35），off 是原始红；hotA/hotB/静态区在两种状态下都保持原始色。
- gate 的 `frame_raster_ms`（约 118–143ms）包含最后一帧 `SnapshotPath` 的 GPU readback/PNG，**不是**稳态 live 帧；`live_frame_raster_ms≈1.6–2.0ms`。`frame_build_ms≈0.2ms` 是 packet build 观测值，不是整窗 CPU。
- `damage_ratio=1` 是 FullPaint 正确性窗的正常结果，不冒充 Retained 脏区。
- `rss_slope_kb_per_min≈300k` 含启动期，`rss_slope_valid=false`；不能据此下泄漏结论。`rss_slope_steady_kb_per_min` 因无效而不输出（既有 honesty 测试锁死）。

### manual 15s（`-manual-seconds 15`，最终代码）

- `presents=899`、`paint_count=898`、`skip=8501`、`rr=489`、`count=11`。
- 相位：`Steady`、`Spike`、`Recover` 全部出现；`phase_mode=animated`。
- `draws_steady=109`、`draws_spike=359`、`draws_recover=2`；`draws_phase_sum=470 == debug_draws_after`。
- `rr_steady=128`、`rr_spike=359`、`rr_recover=2`；`rr_phase_sum=489 == boundary_rerecord`。
- Spike 两个 boundary 每 tick 都改色，所以约 3 秒得到 359 次真实重录；不是每秒只变一次的内容键命中。
- Recover 的 2 次是相位切换首帧仍带上一帧的 hotA+hotB 脏标记；稳定后 `recover_draws_after=0`、`recover_settled=true`，之后 500+ 帧不再画叠加。
- `fps_interval=59.96`、`p95=17.28ms`、`hitch=0`、`vsync=true`、`fallback=0`；`frame_raster_ms=7.47`、`live_frame_raster_ms=1.64`。
- manual 的 `SnapshotPath` 为空，`snapshot_png=""`；跑完 manual 后四个 gate PNG 的 sha256 与跑前一致（`/tmp/r12b_hashes_before_manual_final.txt` 已逐项校验）。

### 基线文件

- gate 快照：`testdata/showcase_r12b_debug_on.png`、`testdata/showcase_r12b_debug_off.png`（GPU readback）。
- Golden 基线：`testdata/showcase_r12b_debug_on_base.png`、`testdata/showcase_r12b_debug_off_base.png`（1200×800 RGBA）。
- on/off 基线必须分开；用 on 的快照去比 off 的基线会 FAIL，这是故意的。
- 首次 `GPUI_ACCEPT_GOLDEN=1` 只播种基线，输出 `SEEDED`，不算 PASS；必须再跑两次 `0.0000%`。

## 已知边界

- 8s 短窗只证明功能/帧时/像素，不证明无内存泄漏；泄漏结论需 ≥60s soak + `rss_slope_valid=true`。
- gate 是 FullPaint 正确性窗，`damage_ratio=1` 正常；不能拿它冒充 retained 的局部脏区能力。
- `frame_raster_ms` 顶部字段含快照读回；稳态看 `live_frame_raster_ms`，不要拿 120ms 读回当 live 光栅回归。
- 当前只在 X11 后端跑（JSON `backend=x11`）；Wayland 未跑，不能声称 Wayland 绿。
- 本窗是引擎能力的**原子验证**，没有组件/2.5D/视频生产消费者；不按生产接线标绿。
- manual 的 `draws_recover=2` 是相位边界的真实残留，不是把 Recover 当脏区；`recover_draws_after=0` 才是稳定判据。
