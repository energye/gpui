# game_step — 8.3 追车脏区独立真窗

一句话：追车只更脏区，静态全程不脏，突发全动退整屏对照。

## 跑法

```sh
go run ./examples/game_step --case=dirty -auto-only
go run ./examples/game_step --case=dirty -manual-seconds 30
go run ./examples/game_step
RUN_SECONDS=8 go run ./examples/game_step --case=dirty -auto-only
```

- `-auto-only`：先跑三证据自检（不过直接 exit 1），再开窗约 8 秒（`RUN_SECONDS` 覆盖，至少 5 秒），标准输出打 `wrgate` 门禁 JSON，`present>=1` 且追车位移 `>0` 且探针全过，否则 exit 1。
- `-manual-seconds N`：常驻 N 秒收真事件，每事件打日志并用 `SetTitle` 显示累计数，结束出汇总 JSON；不带 N 则常驻到关窗。
- 无旗：自检后常驻；设了 `RUN_SECONDS` 则按该秒数定时跑后出汇总。

## 画面（1200x800，标题 game_step）

- 顶栏 + 图例 + 身体 + HUD 照 `wrkit.NewShell`。
- 身体左侧静态区：两个大色块，挂 `RepaintBoundary`，对应 `ui/scene` 静态层全程零标记，右栏 `静态skip` 为 live 回放证据。
- 身体中间追车跑道：红色追车块每帧按 `game/step` 的 `DirtyTracker` 算旧并新，镜像写入 `ui/scene` 的 `DirtyLayer`（`MaxDirtyRects=16`），黄框描边即本帧脏区；每 240 帧（另第 60 帧保底一次）突发 17+ 移动退整屏，青条为整屏对照。
- 身体右侧计数器：脏块数 / 整屏回退数 / 帧率，另带静态 skip 与位移。

## 三证据

- 逻辑探针：脏块数、整屏回退、`Frames/FullFallbacks` 统计（`step` 与 `scene` 两边同数对照）。
- 像素断言：动车新位旧位色、脏框外静区色，容差写死 `probePixelTol=4`（0-255 阶）。
- Golden 静态掩码零容差：`testdata/step_dirty_golden.png`，首跑产生基线，后续逐位比对。

## 门禁 JSON

`wrgate.BuildReport` / `EvaluateGates`，`AbilityID=step-dirty`，`Scenario=game_step--case=dirty`，`Extra` 带 `dirty_rects_max` / `full_fallbacks` / `moved_px` / `probe_ok`。

## 阈值

- `present>=1`，`moved_px>0`，探针全过；`RUN_SECONDS>=5`。
- 脏区超 16 块退整屏（`DirtyRects=nil`，`NeedsFull=true`）。
