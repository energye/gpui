# ui_wr_t_stress — T 三线压力窗

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
go run ./examples/ui_wr_t_stress                  # 默认无限运行，手动关窗退出
RUN_SECONDS=8 go run ./examples/ui_wr_t_stress    # 8s 后自动关闭并输出门禁 JSON
```

## Window / Close duration

1200×800 · 默认**无限运行**（手动关窗退出）；传 `RUN_SECONDS` 按指定秒数自动关闭（`<5 → FAIL (U16)`）。

## 界面布局与预期效果

一屏压满三条线：左边静态图形带，中间按钮动效带，右边特效图片带，底部 HUD。

```
+--------------------------------------------------------------+
| TOP: ui_wr_t_stress  T 三线压力                              |
+----------+---------------------------------------------------+
| LEGEND   | 静态带(300宽)  动效带(按钮+呼吸+位移)  特效带      |
| (260px)  | 8x6 色格+边界  8转圈+波纹+hover  模糊+滤镜+裁剪    |
|          |                大图列表边解边滚                    |
+----------+---------------------------------------------------+
| HUD: T | latency | build | raster | presents | extremes | 门禁  |
+--------------------------------------------------------------+
```

| # | 区域 | 内容 | 预期效果 |
|---|------|------|----------|
| 1 | 静态带 | 8×6 色格，部分带 RepaintBoundary | 静帧 damage 趋零，skip 只增 |
| 2 | 动效带 | 8 转圈 60fps + 波纹按钮 + hover + 呼吸透明度 + 位移动画 | 动画并跑不断流，帧时不爆 |
| 3 | 按钮 | 点我变色（~1s 处合成一次点按） | 点按到上屏 latency 1..2 帧 |
| 4 | 特效带 | 模糊 + 灰度滤镜 + 圆角裁剪同屏 | 同屏特效不爆帧 |
| 5 | 大图列表 | 12 张占位图在 viewport 里持续滚动 | 边解边滚不卡界面线 |
| 6 | 极端项 | X1–X10 断言，X11/X12 只记数 | extremes 10/10，X11/X12 记数不判红 |

### 判断标准

- `input_latency_frames` 1..2 = 输入跟手（T2 生死线）。
- `present_policy = retained`，build/raster 双双有数（防单边假数）。
- 12 条极端里 X11/X12 只记数，其余全绿。
