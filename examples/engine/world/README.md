# game_world — 10.2 开局独立真窗

一句话：场景 JSON 一次装成活数，开局摆得和文件里一模一样，坏文件各报各的错不崩。

## 跑法

```sh
go run ./examples/engine/world --case=open -auto-only
go run ./examples/engine/world --case=open -manual-seconds 30
go run ./examples/engine/world --case=open
RUN_SECONDS=8 go run ./examples/engine/world --case=open -auto-only
```

- `-auto-only`：先跑三证据自检（不过直接 exit 1），再开窗约 8 秒（`RUN_SECONDS` 覆盖，至少 5 秒），标准输出打 `wrgate` 门禁 JSON，`present>=1` 且开局数 `>=3` 且 walker 位移 `>0` 且实体数恒为 5 且探针全过，否则 exit 1。
- `-manual-seconds N`：常驻 N 秒收真事件，每事件打日志并用 `SetTitle` 显示累计数，结束出汇总 JSON；不带 N 则常驻到关窗。
- 无旗：自检后常驻；设了 `RUN_SECONDS` 则按该秒数定时跑后出汇总。

## 画面（1200x800，标题 game_world）

- 顶栏 + 图例 + 身体 + HUD 照 `wrkit.NewShell`。
- 身体左侧开局面板：四个 filed 实体按冻结世界坐标经同一 `viewXY` 映射落位（hero 蓝、sword 黄挂 hero 下、crate 绿、slime 红挂缩放 crate 下），黄线描出两条父子挂接线，橙块 walker 在 hero 下来回走，每 tick 按活数 `WorldOf` 重摆。
- 每 2 秒从磁盘 `LoadScene` + `Open` 重开一局：活数回到 4 + walker，出生台账只涨不回头。
- 身体右侧计数器：开局数 / 实体数 / walker 位移 / 帧率。

## 三证据

- 逻辑探针：开局四实体世界坐标/矩阵/激活态/挂件逐项对冻结数；缺文件、截断、错爹、重名、错版、坏 kind 各回各的错码；2000 实体解析 10ms 量级、开局 2ms 量级（门内各放 4 倍余量）；200 轮进出回基线台账只涨；大关卡连开 5 次字节零漂移。
- 像素断言：四实体色 + walker 色 + 背景色，容差写死 `probePixelTol=4`（0-255 阶）。
- Golden 静态掩码零容差：`testdata/world_open_golden.png`，首跑产生基线，后续逐位比对。

## 门禁 JSON

`wrgate.BuildReport` / `EvaluateGates`，`AbilityID=world-open`，`Scenario=game_world--case=open`，`Extra` 带 `opens` / `entities` / `moved_px` / `parse_ms` / `open_ms` / `bad_codes` / `probe_ok`。

## 大关（S85 裁剪排序，`--case=large`）

```sh
go run ./examples/engine/world --case=large -auto-only
go run ./examples/engine/world --case=large -manual-seconds 60
RUN_SECONDS=60 go run ./examples/engine/world --case=large -auto-only
```

- 数据：`engine/world/testdata/large_scene.json`（20008 实体：200x100 网格＋500 挂点＋8 出生点）与 `large_cull.json`（四组视口＋预算），窗只读文件不编数。
- 画面（1200x800，标题 game_world-large）：整窗全是实体场，按层染色（0 蓝/1 黄/2 紫），挂点黄线相连，出生点品红，白框是被点的 awake 钉子，左上浮一行帧率/active/进出/更新耗时。
- 自动巡相机：视口每秒走 220 世界单位到头跳回，醒进睡出全计数；WASD/方向键点按加步，点击钉住/松开最近的实体。
- 拉大窗口视野跟着变大（同像素密度，看得更多，块不拉伸），板子铺满窗，屏外钉子只算不画，扩大区不再冒白框。
- `-auto-only` 门：`presents>=1`，`active>=1000`，视口走过（`view_moved>0`），探针全过，更新均值 `<=3ms`（p95/max 同报，单测另以均值守 3ms）；`RUN_SECONDS>=5`。
- 三证据：逻辑（四视口数加头八位＋挂点不断链＋出生点对）、像素（层色＋品红＋底色，容差 4）、Golden（`testdata/world_large_golden.png` 零容差）。
- 门禁 JSON：`AbilityID=world-large`，`Scenario=game_world--case=large`，`Extra` 带 `active` / `total` / `entered_total` / `exited_total` / `view_moved` / `update_ms_mean` / `update_ms_p95` / `update_ms_max` / `keys` / `pixels` / `golden`。

## 阈值

- `present>=1`，`opens>=3`（8 秒含初开约 4 次重开），`moved_px>0`，`entities==5`，探针全过；`RUN_SECONDS>=5`。
- 性能门：`parse_ms<=60`（2000 实体，10ms 量级同阶），`open_ms<=15`（2ms 量级同阶）；超门即 FAIL。
- 坏路门：缺文件 not-found、截断/错爹/重名/坏 kind bad-data、错版 version-mismatch，五码分清。
