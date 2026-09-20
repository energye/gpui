# kit_f0_decor — F0-2 decor window

## 运行

```bash
export DISPLAY=:0 LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=5 go run ./examples/kit_f0_decor
# 门禁自检：
go run ./examples/kit_f0_decor -auto-only
```

窗口 1200×800 基线，可调大小。RUN_SECONDS>=5。
正确性窗：`slope_gate=off`（5 秒短窗）+ `fps_gate=off_correctness`
（静态正确性窗，无动画，按 ENGINE U13 只采 fps 不判 60Hz；
持续 tick 仍开，A-J 照采，性能门禁留到 F0-5）。

## 窗内呈现

| 段 | 内容 | 证明 |
|------|----------|----------|
| Decor 矩阵 | 纯色 + 渐变 + 边框，各带圆角与阴影 | 底边圆角一次拼好，色值全走主题 |
| Opacity | 0.65 混合公式色块并排展示 | 公式验算色块（容差≤12/通道），重画隔离不拖帧 |
| Transform | 主色块旋转 15 度 | 中心颜色不变，命中走逆矩阵 |
| Clip | 圆角 16 裁剪 | 命中与绘制一致，角洞点不中 |
| Custom | 圆环 + 对勾 | 画笔自绘 + 独立重画边界 |
| Follow | 锚块 + 跟随条 | 只调 overlay 定位，不自写算法 |

## 调大小

脚本化走一次：2 秒时调到 1400×900，3 秒时调回 1200×800。
门禁读数前要求 `resize_restored=true` 且回到 1200×800。

## 门禁

| 门禁 | 阈值 |
|------|------|
| policy | `present_policy == full_paint` |
| presents | `present_count >= 1` |
| 持续 tick FPS | `fps_interval >= 55` 且 `interval_p95_ms <= 22` |
| 像素三探针 | 纯色中心 + 混合中心（容差≤12/通道，附公式）+ 裁剪中心 |
| Golden | `testdata/showcase_decor_base.png` 零容差 |
| 调大小回基线 | `resize_restored=true` 且 `client_px=1200x800` |
| vsync | `vsync_source` 非空 |

## 三证据

* 逻辑：单测（`decor_test.go` 混合公式 + 命中）+ 窗内半径与混合色进 JSON
* 像素：三点中心采样 + Golden 掩码逐位对比
* Golden：静态掩码逐位对比，差异超限 FAIL
