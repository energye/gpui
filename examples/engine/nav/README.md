# game_nav（S87 寻路转正窗）

网格烘焙确定性加 AStarGrid 寻路加同屏 100 怪分摊排队，全走 `engine/nav` 真包。

- 烘焙：256x256 两遍逐字节一致，200ms 内；小网格三种终态：有路、无路、起点在墙里。
- 寻路：8 向加拐角禁穿（斜行不穿墙角），Godot AStarGrid2D 默认手感。
- 智能体：速度 200，到点容差 2px。
- 队列：每帧默认 8 条（100 怪约 13 帧排完，数写进 JSON 可调）。

跑法：

```sh
RUN_SECONDS=8 go run ./examples/engine/nav -auto-only
go run ./examples/engine/nav -manual-seconds 30
```

手动：鼠标点格重定目标（点按事件计数），结束出汇总 JSON。
自检：逻辑（烘焙/三终态/容差/分摊）+ 像素（容差 8）+ 金图（首跑冻结 `testdata/nav_golden.png`，后续零容差），不过 `exit 1`；`present>=1` 否则 `exit 1`。
