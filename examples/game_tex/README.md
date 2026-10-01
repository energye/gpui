# game_tex — 3.2/3.3 贴图双 case 独立真窗

一句话：远树拉远不闪（mipmap 开关），边走边出不卡（后台流式），缺图品红占位不崩。

## 跑法

```sh
go run ./examples/game_tex --case=far -auto-only
go run ./examples/game_tex --case=stream -auto-only
go run ./examples/game_tex --case=far -manual-seconds 30
go run ./examples/game_tex --case=stream -manual-seconds 30
go run ./examples/game_tex --case=far
RUN_SECONDS=8 go run ./examples/game_tex --case=stream -auto-only
```

- `--case` 只要 `far` 或 `stream`，双 case，不接受第三个。
- `-auto-only`：先跑三证据自检（不过直接 exit 1），再开窗约 8 秒（`RUN_SECONDS` 覆盖，至少 5 秒），标准输出打 `wrgate` 门禁 JSON，case 专属门限不过则 exit 1。
- `-manual-seconds N`：常驻 N 秒收真事件，每事件打日志并用 `SetTitle` 显示累计数，结束出汇总 JSON；不带 N 则常驻到关窗。
- 无旗：自检后常驻；设了 `RUN_SECONDS` 则按该秒数定时跑后出汇总。

## 画面（1200x800，标题 game_tex）

- 顶栏 + 图例 + 身体 + HUD 照 `wrkit.NewShell`。
- far：左近视图（L0 棋盘常亮）+ 中远视图（缩比 1.0→0.3 来回扫，层级走真 `tex.LevelForScale`，黄框描当前远层）+ 右计数器（缩比/层级/切层数/帧率）。
- stream：红小图 + 棋盘中图 + 大图三卡（黄框=装载中，绿框=就绪，装载前显示品红占位）+ 左下缺图缩略图（常驻品红框）+ 右计数器（就绪数/Poll 均耗时/缺图态/大图后台耗时/帧率）。

## 三证据

- 逻辑探针：far 走 `engine/tex/testdata/mipmap_cases.json`（层数/缩比/采样器契约/双图独立开关）+ 缩到 0.3 必落 L1；stream 走 `engine/tex/testdata/stream_cases.json`（后台与同步逐位一致、Poll 微秒级、大图毫秒级、缺图 missing 占位）。
- 像素断言：冻结 spots 逐点对、棋盘块缝锐边无串色、占位逐像素品红、远层均值守恒，容差写死（stream 侧 0，mipmap 侧 1，见引擎冻结文件）。
- Golden 静态掩码零容差：`testdata/tex_far_golden.png` / `testdata/tex_stream_golden.png`，首跑产生基线，后续逐位比对。

## 门禁 JSON

`wrgate.BuildReport` / `EvaluateGates`，`AbilityID=tex`，`Scenario=game_tex--case=<far|stream>`。

## 阈值

- 通用：`present>=1`，探针全过；`RUN_SECONDS>=5`。
- far 专属：`scale03_level==1`（1 倍缩到 0.3 倍必落一级），`level_switches>=1`（8 秒扫过至少一切层），`scale_min<=0.35`（扫到过远端）。
- stream 专属：`ready==ready_total`（三图全就绪），`ghost==missing`（缺图挂品红占位），探针内 `poll_total<=100ms`（64 次前台 Poll 微秒级）且 `big_bg<=50ms`（大图后台毫秒量级）。
