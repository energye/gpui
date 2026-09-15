# game_fx 真窗（5.3 扭曲 + 5.4 后期）

1200x800，标题 `game_fx`。身体左侧原图，中间 warp 水晃区，右侧 grade 胶片区。
顶栏 + 图例 + 身体 + HUD 照 `wrkit.NewShell`，门禁 JSON 照 `wrgate.BuildReport/EvaluateGates`。

## 跑法

```sh
# 自动门禁（RUN_SECONDS 默认 8，JSON stdout，present>=1 且 parity<=1% 且探针全过，否则 exit 1）
RUN_SECONDS=8 go run ./examples/game_fx --case=all -auto-only

# 单能力
go run ./examples/game_fx --case=warp -auto-only
go run ./examples/game_fx --case=grade -auto-only

# 常驻人工（收真事件打日志 + SetTitle，汇总 JSON；不设则直到关窗）
go run ./examples/game_fx --case=all -manual-seconds 30
go run ./examples/game_fx

# RUN_SECONDS 兼容：无旗时也定时
RUN_SECONDS=8 go run ./examples/game_fx --case=all
```

`--case=warp|grade` 默认 `all` 左右对照。`AbilityID` 按 case 为
`fx-warp/fx-grade/fx-all`，`Scenario=game_fx--case=xxx`。
`Extra` 带 `parity_changed_pct/warp_moved_px/grade_dark_corner/probe_ok`
（另带均值、光晕增益、各阶段探针、Golden 数、事件计数）。

## 接的冻接口（只用，不重冻）

- warp：`game/fx` `Warp/NewWarp/DefaultNoise/OffsetAt/WarpPoint/WarpRGBA/SampleClamped`
 （强度 6.0，水晃，强度条可见）。
- grade 整链：`Bloom/Vignette/LUT/Grade`，顺序发光→暗角→查表→映射
 （`Threshold=0.8/Radius=2/Intensity=0.9`，`Inner=0.25/Outer=0.85/Strength=0.45`，
  暖片 LUT 4 点，`TonemapReinhard exposure=1.0`），四角压暗 + 亮日带晕。

## 三证据（容差写死在 main.go 常量区）

- 逻辑探针：warp 偏移量（100,50,t=1.5，模长 0.5..9.0，零强度恒等）、
  grade 各阶段 `IsIdentity/Threshold/FactorAt/Sample/Map/Stages`。
- 像素断言：warp 动了没淹（movedPx>=10 且 movedBytes<总数，阈值 2）、
  grade 拐角比中间暗（差值>0.05）、光晕扩散（晕环减远背景>0.03）。
- Golden 静态掩码零容差：`testdata/fx_grade_golden.png`（96x64 终帧快照，逐位比对，
  差值必须 0）；窗口终帧 `testdata/fx_final.png` 对 `fx_final_base.png`
  经 `wrsoak.EvaluateGolden` 比静态掩码（顶栏 + 图例 + 左右原图/胶片区，
  水晃动区与 HUD 排除），首跑产基线，次跑起零容差。

## C 两边齐先行

`fxParityOffscreen` 在开窗前用老 `DrawImageEx` 离屏对比 CPU（`GOGPU_RENDER_MODE=cpu`）
与 GPU 同一张原图（96x60，白底双线性），`changed_pct<=1%` 才过。
真机实测以窗口 JSON 的 `parity_changed_pct/parity_gpu_ops` 为准。

## 文件

- `examples/game_fx/main.go`（唯一程序）
- `examples/game_fx/testdata/fx_grade_golden.png`（终帧 Golden，确定性生成）
- `examples/game_fx/testdata/fx_final.png` / `fx_final_base.png`（窗口终帧快照，首跑自动产出）
- `examples/game_fx/README.md`（本文件）

只动这三处；`docs` 表、`render/game/ui` 实现、其他 `examples/` 一律不动。
