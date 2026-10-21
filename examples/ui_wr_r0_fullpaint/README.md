# ui_wr_r0_fullpaint — R0 FullPaint 正确性

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

# gate（相位冻结在 Steady，5s，JSON + 退出码）
go run ./examples/ui_wr_r0_fullpaint -auto-only
RUN_SECONDS=5 go run ./examples/ui_wr_r0_fullpaint -auto-only

# manual（动画 Steady→Spike→Recover；关窗或 -manual-seconds 到期后出汇总）
go run ./examples/ui_wr_r0_fullpaint
go run ./examples/ui_wr_r0_fullpaint -manual-seconds 15

# 首次播种 Golden（之后必须连跑两次 0.0000% 才算过）
GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=5 go run ./examples/ui_wr_r0_fullpaint -auto-only
```

## Window / Close duration

1200×800 逻辑像素（U15）。`-auto-only` 或 `RUN_SECONDS` 为门禁档（相位冻结，默认 5s，`<5 → FAIL (U16)`）；不带两者为常驻人工窗，`-manual-seconds N` 可定时关闭。

## 界面布局与预期效果

窗口纵向分区：**顶栏 48px（标题）→ 左图例 260px → Body (284,60) 904×656 → 底部 HUD 72px**。

| Region | Expectation |
|--------|-------------|
| Body 左上 6×4 静色格 @ (304..848, 80..440) | 每帧全树重画（FullPaint），颜色稳定不闪；24 格各自 RepaintBoundary |
| Body 右侧 12 行 static text @ (884..1180, 80..200) | 每帧存活，Clear 后不丢；每行多色 |
| PHASE 大字 @ (884,156) | Steady 黄字；manual 下随相位变字/变色 |
| HOT 动块 120×120 | Steady 红 @ (884,200)；Spike 黄 @ (884,320)；Recover 蓝 @ (964,200) |
| 顶栏 | R0 标题 + 相位 + policy=full_paint |
| 底栏 LiveHUD | 实时 FPS / p95 / paint=… / dmg=… / PASS 预览色 |

gate 档把相位冻结在 Steady，保证同一台机器上的快照逐位可复现；manual 档保留完整三相位动画。

## Gates

| # | Gate | FAIL 线 |
|---|------|---------|
| 1 | 持续 tick 帧率 | 运行 ≥5s 后 `fps_interval≥55`（§2.2.2 正确性+持续 tick 档） |
| 2 | Present | `present_count≥1` |
| 3 | FullPaint 全树 | `paint_count ≥ presents×0.9`（每帧全树重画，静态不丢） |
| 4 | 静/动同屏 | `static_visible≥8`（本窗 36）且 `hot_ticks>0` |
| 5 | policy | `present_policy=full_paint`；`damage_ratio≈1` 属 FullPaint 语义，**不得**拿它冒充 Retained |
| 6 | 全屏 damage | `damage_area_px ≥ 90%` 窗口面积（FullPaint 语义） |
| 7 | vsync 来源 | `vsync_source` 非空（禁止 silent fallback） |
| 8 | 启动 | `warmup=true` 且 `time_to_first_present_ms` ∈ (0,2000)（预算写此，默认建议 <2000ms） |
| 9 | GPU 回退 | `cpu_fallback_ops=0`（本窗无 CPU fallback 理由；热路径暴涨即 FAIL） |
| 10 | 像素断言 | 14/14 通过（12 点 + 2 文字墨量区域，明细见下） |
| 11 | Golden | header+legend+body 逐位零容差，`pixel_golden_diff_pct=0.0000%`；首跑只播种基线 |
| 12 | §2.2 schema | `wrgate.EvaluateGates` 的 A–J 必备字段与类型检查全过 |

逻辑探针共 11 项：`presents_positive`、`fps_interval_ge_55`、`paint_count_covers_presents`、`static_content_ge_8`、`hot_ticks_positive`、`present_policy_full`、`damage_area_full_paint`、`vsync_source_present`、`warmup_observed`、`first_present_within_budget`、`cpu_fallback_zero`。

### 像素断言明细

**点探针（12）**

| Probe | Logical @ | Want | 证明 |
|-------|-----------|------|------|
| static_00 | (346,122) | (0.15,0.50,0.30) | 静格 0,0 颜色与位置 |
| static_10 | (438,122) | (0.25,0.50,0.30) | 静格 1,0 颜色 |
| static_13 | (438,398) | (0.25,0.50,0.30) | 静格 1,3 颜色 |
| static_32 | (622,306) | (0.45,0.50,0.54) | 静格 3,2 颜色 |
| static_51 | (806,214) | (0.25,0.50,0.42) | 静格 5,1 颜色 |
| hot_red | (944,260) | (0.95,0.20,0.20) | Steady 动块颜色 |
| hot_bottom_bg | (944,324) | body bg | 动块高度不越界 |
| grid_gap_bg | (392,122) | body bg | 静格之间有缝、不粘连 |
| spike_slot_bg | (944,380) | body bg | gate 冻结 Steady，Spike 位置为空 |
| body_bg | (734,660) | (0.10,0.11,0.13) | Body 底色 |
| legend_bg | (32,680) | (0.11,0.12,0.15) | 图例底色 |
| top_bg | (1000,24) | (0.12,0.14,0.18) | 顶栏底色 |

**文字墨量区域（2）**

| Probe | Region | 断言 |
|-------|--------|------|
| static_label_ink | (884,80) 70×14 | 非 Body 底色像素 ≥10（文字真的画出来了） |
| phase_label_ink | (884,156) 220×24 | 非 Body 底色像素 ≥20（PHASE 大字真的画出来了） |

**容差依据**

- 理论色点探针 ≤0.06/通道：覆盖 GPU 8-bit 舍入、边缘 AA 与 sRGB 读回误差；不是放宽到「看不出来」。
- 底色探针 ≤0.05/通道：纯色面板无混合，留同样量级的读回余量。
- 文字墨量只判「是否离开底色」，不比对字形灰度，避免字体 hinting 造成误报；字形本身由 Golden 逐位覆盖。
- **Golden 零容差**：同机同引擎基线逐像素比较，任一通道或 alpha 不同即 FAIL；尺寸不一致直接 FAIL。mask = 顶栏 1200×48 + 图例 260×656 + Body 904×656 = **821184 px**；底部 HUD 72px 实时数字会变，**不入 Golden**。

## 实现点六维（U20）

| 维度 | 本窗回答 |
|------|----------|
| 正确性 | `full_paint` 下每帧重画整棵树；24 静格 + 12 静文在 Clear 后仍存活，动块与静态同屏 |
| 脏区 | `damage≈全屏` 是 FullPaint 的必然语义，只用于证明「确实全树重画」，不冒充 retained 局部脏 |
| 缓存 | 静格带 RepaintBoundary 只是场景复杂度；本窗不把 boundary_skip 当归一化门禁，避免与 full_paint 语义混淆 |
| 边界条件 | 固定 1200×800 gate；manual 可 resize（`shell.Resize` 重排）；36 个静态节点 + 相位动块 |
| 失败模式 | 无 Present、fps<55、paint<presents×0.9、policy 非 full_paint、warmup 假、damage<90%、像素/Golden 不匹配 → stderr `FAIL:` + exit 1 |
| 窗内如何看出 | HUD 实时 paint/dmg/fps + PASS 预览色；静格/静文/相位大字/动块均肉眼可见；左栏图例说明语义 |

## 已知限界

- 5s 正确性窗，按 §2.2.4 `slope_gate=off`；`rss_slope_valid=false` 是短窗的正常观测（`rss_warmup_sec=15`），长稳测试另开 soak 窗。`gpu_ops` 为累计路由计数，不是每帧 submit 数。
- 本窗不是 R9/R10，`g_metrics=skipped`（原因写在 `g_metrics_reason`）；`measure_cache_*` 只作为观测输出，不设门禁。
- `-manual-seconds` 档只做人工观察，不跑像素/Golden（相位在动，快照不确定）；此时 JSON 明确写 `pixel_evidence=manual_not_asserted`、`pixel_probes_total=0`、`pixel_golden_diff_pct=null`，证据以 gate 档为准。
- J 正确性：本仓当前没有 `TestNoGPUImport` 测试文件（`go test ./ui -run TestNoGPUImport` 报 no tests to run）；本窗实测 `ui/` 无 cgo import、无 `ui→gpu` 直引，JSON 写 `depcheck=passed_no_cgo_no_ui_gpu_import`。复核命令：`grep -rn '^import "C"' ui/ --include='*.go'` 与 `grep -rn '^\s*"github.com/energye/gpui/gpu' ui/ --include='*.go'` 均无输出。
- Golden 是**同机同引擎**基线；换 GPU/驱动/字体或真实内容变更后需 `GPUI_ACCEPT_GOLDEN=1` 重播种，并人工复核新基线。
- 快照走 `SnapshotPath` 的 GPU readback（不是 X11 背存）；若窗口 DPR≠1，采样按 `image.Width/1200` 自动换算。
