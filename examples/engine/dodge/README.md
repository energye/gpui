# game_dodge（S78 躲避小怪关）

Godot `dodge_the_creeps` 翻版：玩家八向躲怪，撞上死，出屏怪回收，计时加分。

- 玩家：速度 400 像素/秒，八向归一化，出屏 clamp；碰撞走 `engine/physics` 真包（`Body`/`Overlaps`/`Query`），撞上死（藏起 + 不再参与碰撞）。
- 怪：四边随机路点刷出，沿边法线（垂直路点方向）±45°，速度 150–250，每 0.5 秒刷一只，出屏（外扩 48 像素）回收。
- 加分：活着时每秒 +1。
- 窗：1200x800，标题 `game_dodge`；`--case` 只要 `dodge`。

跑法：

```sh
RUN_SECONDS=8 go run ./examples/engine/dodge -auto-only
go run ./examples/engine/dodge -manual-seconds 30
```

手动：WASD/方向键挪玩家，撞怪死一次，事件打日志并累计到标题，结束出汇总 JSON。
自检：逻辑（速度/怪速/角度/碰撞/回收/加分）+ 像素（容差 8）+ 金图（首跑冻结 `testdata/dodge_golden.png`，后续零容差），不过 `exit 1`；`present>=1` 否则 `exit 1`。
