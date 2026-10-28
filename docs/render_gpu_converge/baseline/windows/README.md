# P1.4 全部真窗采集证据

本目录是 P1.4 的逐窗证据，按 `docs/render_gpu_converge/baseline/windows.md` 的 162 扇真窗执行。
28 个 headless 例子不列入 P1.4。

## 环境

- 仓库：`/home/yanghy/app/projects/gogpu/gpui`，HEAD `73df408c`
- 运行 worktree：`/home/yanghy/app/projects/gogpu/gpui-p14`
- 显示：`DISPLAY=:3`，独立 Xwayland（rootless，`-noreset -accessx -core`）；未使用当前桌面 `:1`
- GPU：NVIDIA GeForce 940MX / 580.178.04（与 `thresholds.md` §1.1 同机）
- 后端：native（webgpu-native）与 `GPUI_BACKEND=go`（只切呈现层，不是纯 Go accelerator）
- 关闭：窗自身 `RUN_SECONDS`/`-auto-only` 自关；M 探针先 WM_DELETE，未响应才按 AGENTS 兜底；批量窗最后跑

## 结构

```
<窗名>/<变体>/<native|go>/
  record.json      一次运行的 JSON、rc、状态、指标、证据清单
  stdout.txt       原始 stdout（含 wrgate JSON）
  stderr.log       原始 stderr（探针/失败原因）
  snapshots/       窗内 GPU readback 快照或运行期 *_final/*_recover
  golden/          归档的冻结金图（含运行期播种基线）
```

运行期 `0*` 窗（R1/R14/R15/R20/R22/C6/C9/C10）首遍 `GPUI_ACCEPT_GOLDEN=1` 播种，
次遍逐位对比；基线在 `<窗名>/default/native/snapshots/*_base.png`。

## 重要限制

- rootless Xwayland 的 `x11grab` 抓图是黑图，已禁用外部屏幕抓图。视觉证据用窗内 GPU readback 快照；
  引擎窗归档冻结金图，并用 window/offscreen 金图 diff 计数作为运行期视觉证据；窗写出 `*_final/*_last` 的也已归档。
- 有 14 个窗**自身**带 `x11grab` screen check（`ui_render_base_backdrop`/`cliprrect`/`dither`/
  `filterops`/`maskops`/`miscdraw`/`pathbool`/`svg`/`textlow`/`textspacing`/`textstroke`/
  `verticesmesh`/`webp`、`ui_wr_shadermask`）。rootless Xwayland 抓到黑图，screen check 必然红；
  这些 FAIL 是**采集环境口径**，不是产品回归，P2 需在隔离 Xorg/可抓图的显示上重跑。
  `ui_render_base_backdrop` 的 p95≈137ms 也主要被这次黑图 screen check 拉高。
- `GPUI_BACKEND=go` 只切呈现层；JSON `gpu_backend` 仍可能是 `integrated`。不能把本目录的 go 档
  当成纯 Go 加速器基线（P0.4 已纠正）。
- 纯人工/平台探针（`ui_ime_probe`、`ui_pf_win32`、`ui_pf_appkit`）按 M 处理，不伪造 PASS。
- `ui_wr_t_stress` 当前 HEAD 构建失败（缺少 `ui/kit/button`、`ui/kit/icon`），P1.4 只登记，P2 修。
- `ui_wr_r6_layer_anim` 的 `windows.md` 金图列与代码不一致：表写 `golden/baseline_igpu.png`，
  代码 `evaluateGolden` 实际读 `R6_SNAP_DIR`（默认 `/tmp/r6_layer_anim/r6_final_base.png`），
  仓内图未被读取；P1.4 记录首跑 SEED、次跑 diff=0，但不把它当仓内固定金图 PASS，P2 对齐真源/代码。
- 显存/资源账本的专项快照归 P1.6；P1.4 只归档原始 JSON（含窗自身输出的 `vram_*` 字段），不重复造账本。

## 汇总

- `summary.tsv`：逐轮机器可读表
- `summary.md`：状态、FAIL/SKIP、指标诚实性、运行期金图
- 每个 `<窗名>/README.md`：该窗的金图/容差/运行方式/结果/无金图原因
