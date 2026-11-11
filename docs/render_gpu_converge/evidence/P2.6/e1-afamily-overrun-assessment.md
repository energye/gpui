# E1 评估：A 族超限 4 轮（c3/r7/r7b）在现行树不复现——归因排查 + 复测证据（2026-11-11）

> 任务：§2.2 待办池 E1（P1.7 移交：c3_list_scroll native 三红、r7_virtlist 双后端 p95+hitch 双红、r7b_scroll_reuse native p95 22.42 临界；修后按 `baseline/perf/perf_baseline.tsv` + p99 8ms 回归带复测）。
> 性质：排查为主。**结论先行：四个基线超限在现行树全部不复现（双后端 + X11/Wayland 双路径 7 轮），无性能洞可修**；golden rc=1 全部为既登记的金图漂移族。
> 判据纪律：逐值/逐轮，不用 mean。

## 1 基线（P1.4/P1.7，2026-10-29）vs 现行树复测（2026-11-11）

### c3_list_scroll native（基线三红：fps 54.66 / p95 26.43 / hitch 52.0）

| 轮 | 路径 | fps_int | p50 | p95 | p99 | hitch/min | cpu% |
|---|---|---|---|---|---|---|---|
| 基线 | Xwayland :3 | 54.66 ❌ | 17.98 | 26.43 ❌ | 32.04 | 52.0 ❌ | 40.9 |
| 复测 1 | 会话 Wayland | 58.87 ✅ | 16.66 | 19.77 ✅ | 21.90 | 0 ✅ | 36.2 |
| 复测 2（X11 A/B） | Xwayland :3 显式 | 57.17 ✅ | 17.59 | 19.46 ✅ | 20.66 | 0 ✅ | 40.5 |

### r7_virtlist（基线 native p95 24.89 + hitch 16.0；go p95 22.66 + hitch 15.0）

| 轮 | fps_int | p95 | p99 | hitch/min |
|---|---|---|---|---|
| 基线 native | 59.94 ✅ | 24.89 ❌ | 25.18 | 16.02 ❌ |
| 复测 native | 59.58 ✅ | 20.65 ✅ | 23.12 | 0 ✅ |
| 基线 go | 59.95 ✅ | 22.66 ❌ | 27.36 | 15.02 ❌ |
| 复测 go | 59.51 ✅ | 20.78 ✅ | 24.71 | 1.0 ✅ |

### r7b_scroll_reuse native（基线 p95 22.42 临界；go 当时 NO_JSON）

| 轮 | fps_int | p95 | p99 | hitch/min |
|---|---|---|---|---|
| 复测 native | 58.86 ✅ | 21.22 ✅ | 28.78 | 3.0（obs） |
| 复测 go | 58.88 ✅ | 20.54 ✅ | 26.74 | 0 ✅ |

p99 回归带核对：全部 ≤ 基线 max 32.0（带 = +8ms 相对基线），无回归。

## 2 复现/复测命令与环境

```
go build -o /tmp/e1/<窗名> ./examples/<窗名>
# 会话 Wayland 路（WAYLAND_DISPLAY 在环境，平台层优先；DISPLAY=:3 未生效——见 §5 勘误）：
RUN_SECONDS=60 /tmp/e1/<窗名> -auto-only          # native：+ LD_LIBRARY_PATH/WGPU_NATIVE_PATH
GPUI_BACKEND=go RUN_SECONDS=60 /tmp/e1/<窗名> -auto-only
# 强制 X11 A/B（显式 rootless Xwayland :3，单窗串行、标准关闭）：
Xwayland :3 -rootless &   # pid 2181639
env -u WAYLAND_DISPLAY DISPLAY=:3 RUN_SECONDS=60 /tmp/e1/ui_wr_c3_list_scroll -auto-only
```

原始输出 `/tmp/e1/*.{json,stderr}`（会话产物，收口归档时随证据落 `evidence/P2.6/e1/`）。跑窗覆写的两个被跟踪快照已 `git checkout` 还原。

## 3 归因判读

1. **显示路径变量排除**：c3 native 在 Xwayland（复测 2）与会话 Wayland（复测 1）同样干净——超限消失与 Wayland/X11 无关。
2. **后端变量排除**：r7 双后端同净。
3. **毛刺是离散事件且被消除**：基线 hitch 52/min ≈ 1.15s 周期（p50 仍 vsync 锁定 17.98，即稳态帧没问题、周期性插入一次 22–33ms 帧）；现行树双路径 hitch=0，周期源不存在了。基线轮 elapsed 异常（c3 native 92.4s / go 211.3s 完成 60s 门禁）提示当时采集环境有额外等待/contention，与毛刺同期的可能来源包括当时的 present/resolve 链路。
4. **时间窗内已合入的候选改动**（2026-10-29 → 2026-11-11，未逐一 bisect）：SDF 覆盖 LUT（`4a41c660`，AA 段成本下降）、P4.1 MSAA even-odd + GLES resolve/深度修复（go4x 全黑等三洞）、P4.3 mask 平面按尺寸池化（`mask_r8_modulate.go` 池化，滚动期间不再周期性分配 2–3 张临时纹理——**与 1.15s 周期毛刺的机制最吻合**）、M5 格式参数化（22 文件，8bit 行为零变化）、P2.4e/P2.5 收敛。
5. **未做逐提交 bisect**：目标（门禁达标）已由 7 轮复测客观达成，bisect 属可选深挖（每点 60s × 数十提交），登记为可选后续。

## 4 拍板与处置（2026-11-11，用户拍板方向 A）

- **方向 A（复测销账）已执行**：本评估 + 7 轮原始输出（json/stderr）归档 `evidence/P2.6/e1/`；E1 ✅ 销账（基线超限不复现，无修可做）；perf 基线冻结不改，P2.6 重放按现行树对照（p99 带内、hitch≈0）。
- 方向 B（bisect 归因）与 C（继续压 p95）未采纳；候选改动清单（§3.4）留档供后续回溯。
- golden rc=1 说明：三轮 golden FAIL 均为漂移族（c3 2.5137% / r7 2.7033% / r7b 2.5134%，双后端同值），归「22 窗金图漂移专项」，与本项性能结论无关。

## 5 勘误（本轮自查）

- 本轮复测 1–6 轮原意图跑隔离 `:3`，实际因 `WAYLAND_DISPLAY` 在环境被平台层优先、跑在会话 Wayland 上（JSON `backend=wayland` 为证）；发现后补显式 Xwayland :3 做 X11 A/B（复测 2）补齐。**C6 证据文档同病**：其两轮复现同样实际跑在会话 Wayland（非 rootless Xwayland）；C6 的结论（末 256 采样环机制）与显示路径无关，数值合证在单轮内自洽，结论不受影响，但其文档「隔离 rootless Xwayland」表述按本行勘误修正。
- r7b go 的 P1.4/P1.7 NO_JSON 在 Wayland 路不复现（正常出 JSON）——E3 取项时可按此线索先验。
- 金图漂移新增三窗：c3 2.5137%、r7 2.7033%（双后端同值）、r7b 2.5134%（双后端同值），确定性漂移，归「22 窗金图漂移专项」禁止直接重种。
