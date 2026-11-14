# O10 P3：full_paint 家族边界回放覆盖分类（2026-11-14）

> 隔离显示：rootless `Xwayland :3`（每轮 `xdpyinfo` 健康检查）；单窗串行、
> `RUN_SECONDS=5`（9 个最短时长窗第二遍 15s、c3 60s）、`-auto-only` 标准自退；未硬杀。
> 口径：`boundary_skip>0` = REPLAY_COVERED（① 层回放窄化真实工作）；
> `skip==0 && rr==0` = NO_BOUNDARY_SCENE（场景无 RepaintBoundary，断言不适用，非缺失）。

## 分类结论（80 窗全量）

| 类 | 窗数 | 说明 |
|---|---:|---|
| REPLAY_COVERED | **19** | skip 888–63874：kit_f0 全部 6 窗 + ui_wr 边界类窗（r0/r1/r3/r3b/r11/r12b/r16/r21/c0/c1/c3/c7/c11）；六 kit 窗本轮已接 `wrgate.GateOptions.MinBoundarySkip:1` 真门（原 HUD `>=0` 恒真假绿已修） |
| NO_BOUNDARY_SCENE | **59** | ui_render_base_* 等**原始绘制窗场景无 RepaintBoundary**——skip 恒 0 是场景事实，「skip>0」断言对此类窗不适用（不许硬塞假门） |
| NO_DATA | **2** | `ui_render_jumprope`（金图 44.44% 存量漂移红在 JSON 打印前退出——动态窗基线类，归 O7/存量）；`ui_ruler_parity`（对齐窗 JSON schema 无边界字段） |

明细：`classify.tsv`（第一遍 80 窗）+ `pass2.tsv`（9 窗 15s 重跑，c3 另跑 60s）。
重跑：`scripts/run_fullpaint_boundary_gate.sh`（runner，含两遍逻辑与健康检查）。

## 关键判断

1. 「89 窗全量加 skip>0」**不可行也不该做**：59/80 窗场景无边界，硬塞是假门；
   正确形态 = 有边界窗接真门（6 kit 窗已做）+ 全家族分类登记（本文件）。
2. rc=1 的窗（kit 六窗、r21、c0 系等）红因均为**金图存量漂移**（O10/O7 已登记类），
   与边界门无关——`kit_f0_layout` 实测门判定只有 golden FAIL、MinBoundarySkip 过门（skip=3822）。
