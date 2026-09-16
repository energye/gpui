# game_stage_chase — 追车关（P3 组合门）

五合一：镜头跟随（1.3）＋排序遮挡（2.3/1.2）＋粒子拖尾（5.2）＋瓦片分区（7.2）＋动态更新（8.3）。

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=120 go run ./examples/game_stage_chase -auto-only
go run ./examples/game_stage_chase -manual-seconds 60
go run ./examples/game_stage_chase
```

## Window / Close duration

1200×800，`RUN_SECONDS<5 → FAIL`，关门跑 120 秒（2 分钟）。

## Visible effect

| Region | Expectation |
|---|---|
| 世界跟随区 | 红车左右循环，镜头跟，格子只画可见块 |
| 计数器 | 镜头/分区/排序/拖尾/脏块/帧率六数一直跳 |
| 黄框 | 本帧脏区（车新旧并集），随车走 |
| 橙条 | 车尾拖尾头宽尾窄渐隐，炉烟在固定篝火处 |
| HUD | stage-chase 相位＋fps＋vis/sorted/trail/dirty |

## Gates

- `present_count>=1`，自检逻辑＋像素＋金全过
- 稳态帧率 `fps_interval>=55`，`p95<=25ms`（严版目标 60 帧/20ms，桌面负载留余量）
- 显存/内存起止涨幅 `<=5%`（2 分钟门；2 小时长跑另测）
- 像素离屏金 0 差，`parity<=1%`
- 业务：`moved>0`，`visible>0`，`sorted>0`，`trail>0`，`batch>=1`
- 夜战第二关（灯光＋后期）等追车绿了再定，先不开
