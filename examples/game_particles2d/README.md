# game_particles2d — S83 火/烟/爆炸对照窗

一句话：火锥向上喷，烟缓升带乱流摆，爆炸环扩散带子火星，三发射器走真粒子逻辑，合批一次提交，手感对齐 game_particle。

## 跑法

```sh
go run ./examples/game_particles2d --case=boom -auto-only
go run ./examples/game_particles2d --case=boom -manual-seconds 30
go run ./examples/game_particles2d
RUN_SECONDS=8 go run ./examples/game_particles2d --case=boom -auto-only
```

- `case` 只认 `boom`，别的直接 exit 1。
- `-auto-only`：先跑三证据自检（不过直接 exit 1），再开窗约 8 秒（`RUN_SECONDS` 覆盖，至少 5 秒），标准输出打 `wrgate` 门禁 JSON，`present>=1` 且火/烟/爆三区全活且合批 `>=1`，否则 exit 1。
- `-manual-seconds N`：常驻 N 秒收真事件，点鼠标/按键都重触发一次爆炸，每事件打日志并用 `SetTitle` 显示累计数，结束出汇总 JSON；不带 N 则常驻到关窗。
- 无旗：自检后常驻；设了 `RUN_SECONDS` 则按该秒数定时跑后出汇总。

## 画面（1200x800，标题 game_particles2d）

- 顶栏 + 图例 + 身体 + HUD 照 `wrkit.NewShell`。
- 身体左侧火区：cone 向上喷，亮黄红渐变上升（`ColorOf` 进染色）。
- 身体中间烟区：box 缓升 + 乱流摆动，灰色缓飘。
- 身体右侧爆炸区：ring 向外扩散，死后每粒带 3 颗子火星；约 1.5 秒自动补爆一次，点/按键立即再爆一次。
- 身体底部计数：`fire_alive` / `smoke_alive` / `boom_alive child` / `batch_calls`。

## 三证据

- 逻辑探针：回放 `testdata/boom_cases.json`（火 cone、烟 box、爆炸 ring＋子发射、余烬 point＋子发射），`spawned/alive/child` 逐例对冻结数。
- 像素断言：火亮、烟灰、爆橙、芯亮、外暗，容差写死 `probePixelTol=8`（0-255 阶）。
- Golden 静态掩码零容差：`testdata/particles2d_boom_golden.png`，首跑产生基线，后续逐位比对；`particles2d_boom_last.png` 为当次终帧快照。

## 门禁 JSON

`wrgate.BuildReport` / `EvaluateGates`，`AbilityID=particles2d-boom`，`Scenario=game_particles2d--case=boom`，`Extra` 带 `fire_alive` / `smoke_alive` / `boom_alive` / `boom_child` / `boom_spawned` / `triggers` / `batch_calls` / `probe_ok`。

## 阈值

- `present>=1`，火/烟/爆三区全 `>0`，`boom_spawned>0`，`batch_calls>=1`；`RUN_SECONDS>=5`。
- 千粒子量级：火 `Rate=500/Max=1500`，烟 `Rate=300/Max=1500`，爆 `Max=1500` 定时补爆，同图合批一次提交。
