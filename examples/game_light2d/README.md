# game_light2d 灯影对照窗（S82，G21/W14）

一句话：灯影对照窗，只做 shadow 这一个 case，把点光＋方向光＋竖墙影子＋三档柔边摆出来看，手感照抄 lights_and_shadows。

## 对应总账

- 总账原话：等 S63，gfx_light2d，照抄 lights_and_shadows 开关＋三档柔边；对照窗手感一致。
- 本窗落点：`examples/game_light2d`，case 只要 `shadow`；引擎只读调 `engine/light`（Scene 夜色＋双灯，Shadow 竖墙，SoftShadow 三档），`render/`、`engine/` 老文件一律不动。

## 跑法

- `-auto-only`：先跑三证据自检（S63 三档 None/PCF5/PCF13 逻辑探针＋像素＋Golden，零容差，首跑冻结基线；不过直接 exit 1），再开真窗约 8 秒（`RUN_SECONDS` 覆盖，至少 5 秒），标准输出打 `wrgate` 门禁 JSON，`present>=1` 否则 exit 1。
- `-manual-seconds N`：常驻 N 秒收真事件（点窗/按 L 开关灯，按 1/2/3 或空格切三档；每事件打日志并用 `SetTitle` 显示灯＋档＋累计数），结束出汇总 JSON；不带 N 则常驻到关窗。
- 开关说明（照抄 lights_and_shadows 手感）：一个暗色开关同时管两盏灯，开＝双灯＋墙影，关＝纯夜色；三档随时可切，关灯状态下切档也记住，开灯即生效。

## 门禁

`wrgate.BuildReport` / `EvaluateGates`，`AbilityID=light2d-shadow`，`Scenario=game_light2d--case=shadow`，`Extra` 带 `case` / `probe_ok` / `logic_ok` / `pixel_ok` / `golden_ok` / `off_is_night` / 三档 `moved_px` / `person_gain` / `lee_diff` / `mirror_ok` / `golden_diff` / 档间差异像素数 / `final_light_on` / `final_grade` / `toggles` / `grade_changes` / 事件计数。

## 文件

- `main.go`：全部逻辑（自检＋真窗）。
- `testdata/golden_manifest.json`：三档 Golden 清单（96×64 PNG，零容差，首跑冻结）。
- `testdata/soft_*_golden.png`、`light2d_final*.png`：首次运行自动生成，不随代码提交基线。

## 校验

```sh
gofmt -l examples/game_light2d/
go vet ./examples/game_light2d/
go build ./examples/game_light2d/
```

真窗需 GPU 显示环境；`RUN_SECONDS=8 go run ./examples/game_light2d -case=shadow -auto-only`。
