# kit qrcode — G7 QR matrix plus covers window

## 运行

```bash
export DISPLAY=:1 LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=5 go run ./examples/kit/qrcode
# 门禁自检：
go run ./examples/kit/qrcode -auto-only
```

窗口 1200×800 基线，可调大小。RUN_SECONDS>=5。
正确性窗：`slope_gate=off`（5 秒短窗，E 族只采不判）。

## 窗内呈现

| 区 | 内容 | 证明 |
|------|----------|----------|
| 主卡 | 160 边框矩阵位图（25 格，M 抬升 H） | 编码单测+finder 探针 |
| 色块 | 模块实色（缩放矩阵会糊，探针走实色） | 主题单测 |
| 遮罩卡 | expired/loading/scanned 三态实数 | 状态单测 |
| 特性卡 | icon/颜色/尺寸/空值实数 | 宿主单测 |
| 右小卡 | 换肤 `#722ed1` 静态调用 | 只换 Ctx，组件代码不动 |
| 说明块 | 后端可换加同值同码 | 编码单测 |

## 调大小

脚本化走一次：2 秒时调到 1400×900，3 秒时调回 1200×800。
门禁读数前要求 `resize_restored=true` 且回到 1200×800。

## 门禁

| 门禁 | 阈值 | 类型 |
|------|------|------|
| policy | `present_policy == full_paint` | 硬 FAIL |
| presents | `present_count >= 1` | 硬 FAIL |
| 持续 tick FPS | `fps_interval >= 55` 且 `interval_p95_ms <= 22` | 硬 FAIL |
| 像素三探针 | 面板底色 + 模块实色 + 文字密度，全过 | 硬 FAIL |
| Golden | `testdata/showcase_qrcode_base.png` 零容差 | 硬 FAIL |
| 调大小回基线 | `resize_restored=true` 且 `client_px=1200x800` | 硬 FAIL |
| vsync | `vsync_source` 非空 | 硬 FAIL |
