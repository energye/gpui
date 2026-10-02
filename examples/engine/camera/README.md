# game_camera — S55 镜头长路独立真窗（带 S77 听者）

一句话：镜头追着目标跑，限位钳住不穿帮，远带跑得慢，10 公里长路接缝对不上 1e-9 算输；听者贴在镜头上，中置居中、右偏右、远处微弱但能定位。

## 跑法

```sh
go run ./examples/engine/camera -auto-only
go run ./examples/engine/camera -manual-seconds 30
go run ./examples/engine/camera
RUN_SECONDS=8 go run ./examples/engine/camera -auto-only
```

- `-auto-only`：先跑三证据自检（不过直接 exit 1），再开窗约 8 秒（`RUN_SECONDS` 覆盖，至少 5 秒），标准输出打 `wrgate` 门禁 JSON，`present>=1` 且探针全过且收敛贴住且镜头走过路，否则 exit 1。
- `-manual-seconds N`：常驻 N 秒收真事件，每事件打日志并用 `SetTitle` 显示累计数，结束出汇总 JSON；不带 N 则常驻到关窗。人工用 WASD 推目标、空格抖一下、点一下路把目标拉过去，看镜头跟、限位框、黄框外不穿帮。
- 无旗：自检后常驻；设了 `RUN_SECONDS` 则按该秒数定时跑后出汇总。

## 画面（1200x800，标题 game_camera）

- 顶栏 + 图例 + 身体 + HUD 照 `wrkit.NewShell`。
- 身体左侧 ROAD 长路：天空底 + 两条视差带（远带跟得少、近带跟得多，`engine/camera` 真层算位），红块是目标，黄框是限位（真镜头框过来，不是画死的）。
- 身体右侧读数：目标在哪 / 画面在哪（双位置分开读）/ 听者三路增益声像 / 帧率。
- 操作：WASD 推目标，空格抖一下（shake 当帧可探出限位、随后衰减回来），点路拉目标，2 秒自动换一个航点。

## 三证据

- 逻辑探针：冻表直读 `testdata/camera_cases.json`（窗里不写死数，全走真包）——跟随 30 步逐位对、限位钳住值逐位对、坏限位（翻转/零尺寸）退化为不限位不崩、NaN 限位报 invalid-arg 且状态不动、10 公里镜像缝位移与投影双 1e-9 内、半速带正好落中间、死区内驻外跟、目标/画面分开读、听者中置 1/0 右偏 0.5/0.5 远处 0.05/0.95、中置立体声左右相等、单例抢占与清空、bus 0.8 与合并 0.4 与 mixer 留 2 封顶 1 全对。
- 像素断言：天空/远带/近带/目标色，容差写死 `probePixelTol=4`（0-255 阶）。
- Golden 静态掩码零容差：`testdata/camera_follow_golden.png`，首跑产生基线，后续逐位比对。

## 门禁 JSON

`wrgate.BuildReport` / `EvaluateGates`，`AbilityID=game-camera`，`Scenario=game_camera`，`Extra` 带 `case` / `probe_ok` / `follow_ok` / `converge_frames` / `converge_err` / `converge_ok` / `limit_ok` / `seam_ok` / `parallax_ok` / `dead_ok` / `dual_ok` / `audio_ok` / `bus_ok` / `cam_travel_px` / `moves` / `holds` / `frames`。

## 阈值

- `present>=1`，探针全过，`RUN_SECONDS>=5`。
- 跟随 30 帧（0.5 秒）内误差进 1.0（冻表 `max_frames=30` / `max_err=1.0`，S55 口径 0.5 秒贴住）。
- 10 公里缝 1e-9 内（冻表 `tol=1e-9`）。
- 坏限位钳住不崩（翻转/零尺寸退化、NaN 报 invalid-arg）。
- 听者：中置居中、右偏右、远处微弱可定位；衰减经 bus 0.8 合并与 mixer 封顶覆盖。
