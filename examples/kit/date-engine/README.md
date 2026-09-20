# kit date-engine — G4 civil math plus panel window

## 运行

```bash
export DISPLAY=:1 LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=5 go run ./examples/kit/date-engine
# 门禁自检：
go run ./examples/kit/date-engine -auto-only
```

窗口 1200×800 基线，可调大小。RUN_SECONDS>=5。
正确性窗：`slope_gate=off`（5 秒短窗，E 族只采不判）。

## 窗内呈现

| 区 | 内容 | 证明 |
|------|----------|----------|
| 月面板 | 2024年2月闰月6x7格，29号主色填充 | 矩阵单测+选中探针 |
| 右卡 | 排序后的3月范围+静态/函数预设 | order+预设单测 |
| 佛历卡 | 2567-02-29+周起始说明 | 佛历/周起始单测 |
| 右小卡 | 换肤 `#722ed1` 静态调用 | 只换 Ctx，组件代码不动 |
| 说明块 | 闰月排序函数文案 | 文字密度探针 |

## 调大小

脚本化走一次：2 秒时调到 1400×900，3 秒时调回 1200×800。
门禁读数前要求 `resize_restored=true` 且回到 1200×800。

## 门禁

| 门禁 | 阈值 | 类型 |
|------|------|------|
| policy | `present_policy == full_paint` | 硬 FAIL |
| presents | `present_count >= 1` | 硬 FAIL |
| 持续 tick FPS | `fps_interval >= 55` 且 `interval_p95_ms <= 22` | 硬 FAIL |
| 像素三探针 | 面板底色 + 选中填充 + 文字密度，全过 | 硬 FAIL |
| Golden | `testdata/showcase_date_engine_base.png` 零容差 | 硬 FAIL |
| 调大小回基线 | `resize_restored=true` 且 `client_px=1200x800` | 硬 FAIL |
| vsync | `vsync_source` 非空 | 硬 FAIL |
