# B2：ui_wr_r6_layer_anim 金图真源对齐（2026-11-11）

> 台账 §2.2 B2：表内金图 golden/baseline_igpu.png 与代码真源不一致（代码读 R6_SNAP_DIR，
> 默认 /tmp/r6_layer_anim/r6_final_base.png，仓内图未被读取）。

## 0 结论

窗口金图基线改到仓内 examples/ui_wr_r6_layer_anim/testdata/goldens/（native 与
GPUI_BACKEND=go 各自基线），运行期快照仍写 R6_SNAP_DIR（默认 /tmp，可覆盖）；
表内路径同步；两次跑双后端均 0.0000%，1px 篡改基线的负向按预期红、还原复绿。

## 1 改动

- examples/ui_wr_r6_layer_anim/main.go
  - evaluateGolden(cur, goldensDir, rects)：基线固定读
    examples/ui_wr_r6_layer_anim/testdata/goldens/r6_final_base.png
    （GPUI_BACKEND=go 读 _go.png）；目录自动创建。
  - 运行期快照路径不变：R6_SNAP_DIR（默认 /tmp/r6_layer_anim）/r6_final.png，
    不把带 HUD 的每跑快照写进仓（避免每次跑脏树）。
- docs/render_gpu_converge/baseline/windows.md：表内金图路径改为
  testdata/goldens/r6_final_base.png（go: _go.png）；§7 追加闭合行。
- README：补金图位置与 R6_SNAP_DIR 覆盖说明。
- 旧 golden/baseline_igpu.png 仅作历史归档，不再被读取。

## 2 验证（证据 evidence/P2.6/b2-windows/）

| 跑 | 结果 |
|---|---|
| native seed | 基线写入 testdata/goldens/r6_final_base.png；该跑 hitch 8.01/min 超 5 红（见旁注） |
| native verify | golden=0.0000%（306280 px）、scripted 6/6、rc=0 |
| go seed | 基线写入 r6_final_base_go.png、rc=0 |
| go verify | golden=0.0000%（306280 px）、scripted 6/6、rc=0 |
| 负向（native，基线 1px 改洋红） | 第 1 次撞 hitch 6.01 先红；第 2 次 pixel_golden_diff_pct=0.0003 want 0 over 306280 px 红 |
| 还原 | 基线与备份 sha256 一致；native/go 复跑 0.0000% |

## 3 旁注（不在 B2 范围）

- 本窗 hitch 预算 ≤5/min 在 30s 窗上对 1-4 次抖动敏感：seed 跑 8.01 红、verify 跑
  2.00/4.02 绿。属既有门禁灵敏度（与 §2.2 C5/E1 性能线同族），B2 不改门禁语义。
- 金图仅覆盖静态掩码（topbar/legend/dense，306280 px），动画板按设计排除。
