# game_platformer（S81 横版手感对照窗）

platformer 手感对照：横版挪跳，落地不穿不硬。

- 手感：移速 300 像素/秒（加速度 1800 靠拢），跳速 -725（Y 下为正，重力往下拉），重力 700，坠落上限 700，松键阻尼每 1/60 秒留 0.6，二段跳只给一次。
- 场景：绿地条站人，蓝条浮空可落，橙块是跳人；`--case` 只要 `platformer`。
- 窗：1200x800，标题 `game_platformer`。

跑法：

```sh
RUN_SECONDS=8 go run ./examples/game_platformer -auto-only
go run ./examples/game_platformer -manual-seconds 30
```

手动：←→/AD 挪，空格/W 跳，事件打日志并累计到标题，结束出汇总 JSON。
自检：逻辑（300/-725/700 落位＋满速＋松键＋跳弧＋二段＋坠落上限）+ 像素（容差 8）+ 金图（首跑冻结 `testdata/platformer_golden.png`，后续零容差），不过 `exit 1`；`present>=1` 否则 `exit 1`。
