# game_physics — S61 物理双 case 独立真窗（S76 随窗）

一句话：红块扫黄门灰墙射线逢门必报；起跳穿蓝条再落住，斜坡吸住灰台带人不丢。

## 跑法

```sh
go run ./examples/game_physics --case=hit -auto-only
go run ./examples/game_physics --case=jump -auto-only
go run ./examples/game_physics --case=hit -manual-seconds 30
go run ./examples/game_physics --case=jump
RUN_SECONDS=8 go run ./examples/game_physics --case=hit -auto-only
GAME_PHYSICS_PROBE_ONLY=1 go run ./examples/game_physics --case=hit
```

- `--case` 只有两档：`hit`（14.1 碰撞射线）与 `jump`（14.2 平台跳跃），别的值直接 exit 1。
- `-auto-only`：先跑三证据自检（不过直接 exit 1），再开窗约 8 秒（`RUN_SECONDS` 覆盖，至少 5 秒），标准输出打 `wrgate` 门禁 JSON，见下节阈值，否则 exit 1。
- `-manual-seconds N`：常驻 N 秒收真事件，每事件打日志并用 `SetTitle` 显示累计数，结束出汇总 JSON；不带 N 则常驻到关窗。
- 无旗：自检后常驻；设了 `RUN_SECONDS` 则按该秒数定时跑后出汇总。
- `GAME_PHYSICS_PROBE_ONLY=1`：只跑本 case 自检（逻辑＋像素＋金图），不开窗，供中央门禁与基线制作。

## 画面（1200x800，标题 game_physics）

- 顶栏 + 图例 + 身体 + HUD 照 `wrkit.NewShell`。
- `hit`：左侧 480x270 竞技场，红块左右扫，黄门=触发区（只报不挡），灰墙=实心（边触即撞），黄线=本帧 +X 射线；右侧计数器：撞门帧数 / 射线命中与入门数 / 帧率 / 位移。
- `jump`：左上 480x270 跳台（绿地条＋蓝单向条＋灰斜坡线＋红跳人，起跳穿蓝条再落住，斜坡脚全程吸附）；左下 360x180 带人台（灰台左右走，红人站台上被带着走，Y-up 世界只在画时翻 Y）；右侧计数器：落住次 / 穿过与在板 / 带着走与吸附 / 帧率 / 状态。
- 引擎只读：两窗只调 `engine/physics` 真包（Query/CastRay/Step/StepWithSnap/SnapGrounded/CarryRider），示例不写任何物理。

## 三证据

- 逻辑探针：回放 `engine/physics/testdata/` 四份冻结数（body 边触/触发/掩码、ray 撞墙 8 米门最近瓷砖格、platform 站落穿顶、snap 吸附加两档），2000 次重放逐位一致，10 步带人链不断；耗时只记数（见下节）。
- 像素断言：人/墙/门/地/板/背景取色，容差写死 `probePixelTol=8`（0-255 阶；hit 人采样点避开射线行）。
- Golden 静态掩码零容差：`testdata/physics_hit_golden.png` 与 `testdata/physics_jump_golden.png`，首跑产生基线，后续逐位比对（jump 金图只罩跳台场景，带人台以 live 计数为证）。

## 门禁 JSON

`wrgate.BuildReport` / `EvaluateGates`，`AbilityID=game-physics`，`Scenario=game_physics--case=hit|jump`，`Extra` 带计数器（hit：`contact_frames`/`ray_hits`/`trigger_seen`/`moved_px`/`body100_us`/`ray100_us`；jump：`lands`/`passes`/`on_floor`/`carries`/`holds`/`slope_us`）与 `probe_ok`。

## 阈值

- `present>=1`，探针全过；`RUN_SECONDS>=5`。
- hit：`moved_px>0`，`contact_frames>=1`，`ray_hits>=1`，`trigger_seen>=1`。
- jump：`lands>=1`，`passes>=1`，`carries>=1`。
- 耗时天花板（debug 探针量级守卫，防机器抖误杀；正式数按 §5 走 release 三遍取最差）：百盒查询 `<=4000us`（实测 250-350us 量级），百盒射线 `<=100us`（实测 3-5us 量级），长坡单步 `<=5.0us`（实测 0.1-0.2us 量级）。

## S76（随本窗）

`engine/physics/snap_mode.go` 纯加法：`MotionMode` 两档（grounded/floating）、D18 默认值（snap 0.1、max 角 45°、margin 0.08）、`SnapConfig`、`FloorNormal`、`SnapGrounded`/`IsOnFloor`、`StepWithSnap`；旧函数一字未动。jump 窗连带验：斜坡脚吸附、单向穿过再落住、带人不断。

## D01–D20 差异

- D18：snap 0.1、max 角 45°、margin 0.08 三数逐位对；`MOTION_MODE` 两档语义对（floating 永不报地板）；有意差异一处：Godot 的 snap 要求“上一帧在地板上”才吸住，本实现只要可走表面落在窗口内就吸（窗口仍以 0.1/0.08 为界），长坡不断更稳。
- D19：layer/mask 原样对，单向面只落不上顶，边触算碰；体内无穿透求解（本包只给数不解算，由调用方推开，窗里跳人不压板即此故）。
