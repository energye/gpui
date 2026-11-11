# I5 金图债收口：`ui_base_vram_cap` 8 窗重种 + 双后端 300s 0.0000%（2026-11-12）

> 任务：I5 v2 遗留的 `ui_base_vram_cap` 金图 17.5273%（P2.6「22 窗金图漂移」专项后处理）。
> 用户 2026-11-12 拍板：按既有 i5-v2 证据同批重种；不重跑渲染分析。
> 结论：**8 窗金图重种完成；native Mailbox 与 go/gles 各一轮 300s cascade，8/8 窗 `0.0000%`、像素探针 16/16、`crash_free=true`、rc=0。I5 金图债清。**

## 1 重种来源与旧基线归档

- 来源：`evidence/P2.4e/i5-v2/golden/current/vram_win1..8.png`（2026-11-12 01:40 Mailbox 正轮当前图）。
- 目标：`examples/ui_base_vram_cap/testdata/vram_win1..8_base.png`。
- 旧基线归档：`evidence/P2.6/golden-drift/i5-vram/old-bases/vram_win1..8_base.png`（2026-10-28 旧图，均已在 git 历史）。
- 重种哈希（旧 → 新）：见 `reseed.txt` / `i5-vram/old-bases`；8 张全部更新。

## 2 重种前的量化（旧基线 vs i5-v2 当前图）

Golden mask 按窗源码 `[]rect{{x:0,y:hudH,w:640,h:480-hudH}}`（`hudH=40`，总 281,600 px）：

| 窗 | golden diff | 内容区 diff | 最大 Δ | 内部采样 identical | 平移最优 |
|---|---:|---:|---:|---|---|
| win1..win8 | 17.5273% / 窗 | 49,357 / 281,600 px | 35 | 176/176 / 窗 | (0,0)，增益 0 |

- 旧判定材料 `i5-v2/golden/assessment.md` 的 1px 边沿/HUD 结论成立；本次用统一 mask 复核，8 窗内部采样点逐字节相同、无整体错位。
- 逐窗分析 JSON/视觉对照：`i5-vram/analysis/win1..8/`（`diff.png` / `compare.png`）。

## 3 重种后验收（隔离嵌套 Wayland `gpui-p26vram`，cascade 8 窗，300s）

| 轮 | 后端 | 快照行 | golden per-window | pixel probes | ledger peak | hold slope | rc |
|---|---|---|---:|---|---:|---:|---:|
| native（第一轮） | webgpu Mailbox | 7/8（win1 未写出） | win1 17.5273% / 其余 0 | 16/16 | — | — | 1 |
| native（第二轮） | webgpu Mailbox | 8/8 | **8×0.0000%** | 16/16 | 373.82MB | +2215KB/min | 0 |
| go | gles/fifo | 8/8 | **8×0.0000%** | 16/16 | 365.62MB | +397KB/min | 0 |

- native 第二轮原始输出：`verify-native-2.stdout/stderr`；go 原始输出：`verify-go.stdout/stderr`。
- `pixel_golden_per_window=[0,0,0,0,0,0,0,0]`、`pixel_golden_total_px=2,252,800`、`pixel_golden_first_run=false`、`golden_tolerance_pct=0`。
- 账本峰值均 < 768MB×0.9=691.2MB；`crash_free=true`；窗口正常退出（标准 `Quit/Close`）。
- 第一轮 win1 未写出 `snapshot:` 行，gate 读取了陈旧 `vram_win1.png` 并报 17.5273%；第二轮 8/8 写出、全 0。该快照竞态登记 §2.2 `N3`，不影响本轮 I5 语义结论。

## 4 负向对照

- 金图比较器的 1px 负向已由 22 窗专项的 backdrop 窗口级负向覆盖：`SNAP_DIR` 临时目录内把基线平移 1px → `diff=12.9901%`、rc=1；还原 → `0.0000%`、rc=0（`evidence/P2.6/golden-drift/verify/negative-shift1px.*`、`negative-restored.*`）。
- 本 I5 8 窗与 22 窗共用同一套逐像素 golden 判定（reading PNG + mask + zero tolerance）；不重复做 300s 破坏轮。

## 5 状态

- I5 v2：功能/真窗/负向/双后端 300s + 金图 0.0000% 全部齐 → **✅**。
- 残余风险不变（I5 v2 summary §7）：只有 FIFO 时 WARN；FFI 内不可打断；go/gles Wayland 仍走 EGL 阻塞 present（C7 域）；上述均不阻塞 I5 语义收口。
