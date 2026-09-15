# game_particle — 5.1 火锥+烟独立真窗

一句话：火锥向上喷，烟缓升带乱流摆，双发射器走真粒子逻辑，合批一次提交。

## 跑法

```sh
go run ./examples/game_particle --case=fire -auto-only
go run ./examples/game_particle --case=fire -manual-seconds 30
go run ./examples/game_particle
RUN_SECONDS=8 go run ./examples/game_particle --case=fire -auto-only
```

- `-auto-only`：先跑三证据自检（不过直接 exit 1），再开窗约 8 秒（`RUN_SECONDS` 覆盖，至少 5 秒），标准输出打 `wrgate` 门禁 JSON，`present>=1` 且 `fire_alive>0` 且 `smoke_alive>0` 且 `batch_calls>=1`，否则 exit 1。
- `-manual-seconds N`：常驻 N 秒收真事件，每事件打日志并用 `SetTitle` 显示累计数，结束出汇总 JSON；不带 N 则常驻到关窗。
- 无旗：自检后常驻；设了 `RUN_SECONDS` 则按该秒数定时跑后出汇总。

## 画面（1200x800，标题 game_particle）

- 顶栏 + 图例 + 身体 + HUD 照 `wrkit.NewShell`。
- 身体左侧火锥区：cone 向上喷，亮黄红渐变上升（`ColorOf` 进染色）。
- 身体右侧烟区：box 缓升 + 乱流摆动，灰色缓飘。
- 身体底部计数：`fire_alive` / `smoke_alive` / `batch_calls`。

## 三证据

- 逻辑探针：全量回放 `game/particle/testdata/emitter_cases.json`，`spawned/alive/child` 逐例对金数据。
- 像素断言：火心亮、锥外暗、烟区灰，容差写死 `probePixelTol=8`（0-255 阶）。
- Golden 静态掩码零容差：`testdata/particle_fire_golden.png`，首跑产生基线，后续逐位比对；`particle_fire_last.png` 为当次终帧快照。

## 门禁 JSON

`wrgate.BuildReport` / `EvaluateGates`，`AbilityID=particle-fire`，`Scenario=game_particle--case=fire`，`Extra` 带 `fire_alive` / `smoke_alive` / `batch_calls` / `fire_spawned` / `probe_ok`。

## 阈值

- `present>=1`，`fire_alive>0`，`smoke_alive>0`，`batch_calls>=1`；`RUN_SECONDS>=5`。
- 千粒子量级：火 `Rate=500/Max=1500`，烟 `Rate=300/Max=1500`，同图合批一次提交。
