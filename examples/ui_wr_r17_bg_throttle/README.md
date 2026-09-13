# ui_wr_r17_bg_throttle — R17 不可见停帧

前台正常渲染 22 秒 → 自己最小化进后台 8 秒，
证明不可见时一帧都不画（停帧）。

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=30 go run ./examples/ui_wr_r17_bg_throttle
```

## Window / Close duration

- 客户区 `1200×800` 逻辑像素（U15）。
- 关闭用 `RUN_SECONDS=30`；`RUN_SECONDS<5` 直接 FAIL（U16）；
  `RUN_SECONDS<30` 相位脚本跑不完，门禁 FAIL（不是阈值放水，是脚本没跑完）。
- 需要 GPU 真窗，无显示环境 `FAIL: window open (needs_gpu_window)`。
- 本窗结束时仍是最小化状态（见下），这是设计，不是卡死。

## Visible effect

| Region | Expectation |
|--------|-------------|
| FG 进度条（左上绿板） | 前台数字递增、进度条右移；最小化后窗口不可见 |
| D 静态 4×4 色格 + 8 标签 | 前台全程存在 |
| HOT 红块 | 前台每帧变色 |
| 顶栏相位 | Steady → Background |
| HUD | 实时显示 `fg/bg presents` 与 `min` 锁存状态 |

## Gates

停帧口径（用户确认方向）：锁存段内零提交，**不是**
门禁表字面的“后台间隔变大 2 倍”。停住的循环根本没有后台
帧间隔采样可比，所以 `fps_wall` 全程偏低是设计使然，
看 `fps_interval`（前台稳态节拍）。

| Gate | Threshold | Meaning |
|------|-----------|---------|
| `fg_presents` | `>= 60` | 最小化前前台在流动 |
| `bg_presents`（锁存边沿到结束） | `<= 2` | 锁存期间零提交（在途帧余量） |
| `minimized_observed` | `true` | WM 上报了最小化锁存 |
| 全族 schema | `wrgate` | A–J 字段齐，否则 FAIL |

恢复不在窗内证明：实测 Mutter 无视裸 `XMapWindow` 恢复
（轮询取证：`WM_STATE` 全程 `Iconic`），而 Wayland 根本没有
程序化恢复原语。恢复由单测覆盖
（`TestHandleLifecycle_MinimizedLatch`：锁存→释放→续帧）与
用户驱动路径（任务栏点击 → `MapNotify` → `StateChanged`）。

## Backend note

- X11：最小化自报 + `WM_STATE` 对账 + `MapNotify` 恢复信号，30s 两连 PASS（fg≈1290、bg=0）。
- Wayland：本机 X11 下嵌套 GNOME Shell 跑过 30s 两连 PASS
 （`WAYLAND_DISPLAY=gpui-nested`，fg≈430、bg=0，`backend=wayland` 确认），
  停帧链路跨后端成立；嵌套环境前台慢且 jank（p95 ~60ms、fallback、CPU 回退多，
  均为嵌套+软件栈代价，门禁不含此项、JSON 如实记录）。
  `xdg-toplevel` 恢复是用户/合成器动作，自动窗只验证到锁存停帧段。
- 跑嵌套 Wayland 需要先起合成器（例：软件渲染
  `dbus-run-session -- gnome-shell --nested --wayland-display=gpui-nested --no-x11`），
  再带 `WAYLAND_DISPLAY=gpui-nested` 跑本窗；`platform.Open` 会自动选 Wayland 后端。
