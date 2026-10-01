# game_sprite 真窗（2.1 合批 / 2.2 图集 / 1.2 遮挡 / 2.4 帧动画）

一句话：三张卡同屏验精灵，红盖绿、转90、千树一次交，画面不对直接判FAIL；anim 单卡验帧动画，dodge 真帧走跑切换，帧号不对直接判FAIL。

## 跑法

```sh
# 自动判（门禁）：短窗 8 秒，JSON 打 stdout
GOGPU_RENDER_MODE=cpu go run ./examples/game_sprite --case=all -auto-only

# 单能力
go run ./examples/game_sprite --case=rot -auto-only
go run ./examples/game_sprite --case=depth -auto-only
go run ./examples/game_sprite --case=batch -auto-only
go run ./examples/game_sprite --case=anim -auto-only

# 常驻人工看 30 秒（收真事件改标题 events=N，结束出汇总 JSON）
go run ./examples/game_sprite --case=all -manual-seconds 30

# 默认：先自检，再常驻到关窗；RUN_SECONDS 设了就按它定时关
go run ./examples/game_sprite
```

`--case` 只认 rot|depth|batch|anim|all，其他值 exit 2。

## 窗里看什么（1200x800，标题 game_sprite）

- 左卡 ROT：转 90 度红块 + 脚底轴心灰蓝块（红线=轴心线）+ 染色灰块 + 翻转镜像块。
- 中卡 DEPTH：上=true 排序（近红盖远绿）/ 中=false 原序对照 / 下=同深条（不闪）。
- 右卡 BATCH：千树阵列 + 调用数（同图一次交，batch_calls=1）。
- ANIM（--case=anim 独立）：上=四真帧静条（dodge 走1/走2/上1/上2，离屏金覆盖）+ 下=活卡（当前帧实时跟播，每2秒右上互换）。

## 门禁阈值（写死）

- present>=1（wrgate.EvaluateGates，MinPresents=1）。
- parity_changed_pct<=1（CPU vs GPU 离屏，超了 exit 1）。
- probe_ok=1（3 像素探针：红盖绿中心 41,41 / 转90角 40,88 / 批量区 189,129，容差 0.05）。
- golden_changed_pct=0（静态掩码零容差；首跑产基线，次跑起逐位）。
- AbilityID 按 case 填 sprite-rot/sprite-depth/sprite-batch/sprite-anim/sprite-all，
  Scenario 写 game_sprite--case=xxx，
  Extra 带 parity_changed_pct/parity_mean_abs/parity_gpu_ops/batch_calls/depth_sorted/probe_ok/events_*。
- anim 追加门（写死）：sixseq_ok=1（六序列逐事件全对）/ art_ok=1（dodge 真帧尺寸+内容+对帧差异）/
  parity_changed_pct<=1（CPU vs GPU 静条）/ probe_ok=1（四格内容+对帧差异）/
  golden_changed_pct=0（离屏 anim_off_golden.png 零容差）/
  win_golden_diff_pct=0（窗 anim_final_base.png 静条掩码零容差）/
  switches>=3（8秒至少右上互换3次）/ switch_errors=0 / mirror_errors=0（回调与返回逐事件一致）。

## testdata

- sprite_golden.png：离屏画布基线（280x200），改了先查代码，不许直接更新图。
- sprite_last.png：本次运行快照（每次跑覆盖）。
- anim_cases.json：anim 冻结用例（clips/sequences 与 engine/sprite/testdata/flipbook_cases.json 数据一致，
  另加 art 双动画 dodge 真帧表：right/up 各两帧、speed 5.0=200ms、loop）。
- dodge_walk1/walk2.png、dodge_up1/up2.png：dodge_the_creeps 人物真帧（MIT，KidsCanCode 2017，
  见上游 LICENSE；走 108x135/107x127、上 108x135/111x120）。
- anim_off_golden.png：四真帧静条离屏基线（576x180），改了先查代码，不许直接更新图。
- anim_last.png：anim 本次运行快照（每次跑覆盖）。
- anim_final_base.png：anim 窗静条掩码基线（首跑自动存，次跑起逐位）。
