# game_25d_basis — S79 真2.5D六视角独立真窗

一句话：3D 的数、2D 的画。6 种视角只换基向量，排序走 Y，影子落线，切视角不闪、前后盖对、影子不飘。

## 跑法

```sh
go run ./examples/game_25d_basis -auto-only
go run ./examples/game_25d_basis -manual-seconds 30
go run ./examples/game_25d_basis
RUN_SECONDS=10 go run ./examples/game_25d_basis -auto-only
```

- `-auto-only`：先跑三证据自检（不过直接 exit 1），再开窗约 8 秒（`RUN_SECONDS` 覆盖，至少 5 秒），标准输出打 `wrgate` 门禁 JSON，`present>=1` 且探针全过且帧走过，否则 exit 1。
- `-manual-seconds N`：常驻 N 秒收真事件，每事件打日志并用 `SetTitle` 显示累计数，结束出汇总 JSON；不带 N 则常驻到关窗。人工按 1-6 切视角、WASD 挪英雄，看盖对和影子。
- 无旗：自检后常驻；设了 `RUN_SECONDS` 则按该秒数定时跑后出汇总。

## 画面（1200x800，标题 game_25d_basis）

- 顶栏 + 图例 + 身体 + HUD 照 `wrkit.NewShell`。
- 身体左侧 STAGE：天空底 + 地面线（影轨），4 个块按 Y 排序从后往前画，红块是英雄，每块配一块深色影子落在地面线上，全走 `engine/camera` 真层算位。
- 身体右侧读数：当前视角 / 盖序 / 影子落点 / 帧率。
- 操作：1-6 切视角，WASD 挪英雄（Y 增减会改变盖序）。

## 三证据

- 逻辑探针：冻表直读 `engine/camera/testdata/basis25d_cases.json`（窗里不写死数，全走真包）——六基向量逐位对、六投影 1e-9 内、排序稳定、影子落点对、地上显示地下藏。
- 像素断言：天空/块/英雄色，容差写死 `probePixelTol=4`（0-255 阶）。
- Golden 静态掩码零容差：`testdata/basis_view_golden.png`，首跑产生基线，后续逐位比对。

## 门禁 JSON

`wrgate.BuildReport` / `EvaluateGates`，`AbilityID=basis-25d`，`Scenario=game_25d_basis`，`Extra` 带 `case` / `probe_ok` / `basis_ok` / `proj_ok` / `sort_ok` / `shadow_ok` / `views` / `sort_n` / `modes` / `frames`。

## 阈值

- `present>=1`，探针全过，`RUN_SECONDS>=5`。
- 六矩阵逐位对，投影 1e-9 内，双金 0 差。
