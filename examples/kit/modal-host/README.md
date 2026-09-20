# kit modal-host — G1 dialog stack window

## 运行

```bash
export DISPLAY=:0 LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=5 go run ./examples/kit/modal-host
# 门禁自检：
go run ./examples/kit/modal-host -auto-only
```

窗口 1200×800 基线，可调大小。RUN_SECONDS>=5。
正确性窗：`slope_gate=off`（5 秒短窗，E 族只采不判）。

## 窗内呈现

| 区 | 内容 | 证明 |
|------|----------|----------|
| 三层卡片 | z1000/1010/1020 重叠，顶层盖底层 | 嵌套不乱序，单元测试断 z 递增 |
| 右卡 | 换肤 `#722ed1` 静态调用 | 只换 Ctx，组件代码不动 |
| 底条 | 合并后 mask 色带 | mask.enabled/closable 合并生效 |
| 说明块 | 关中间层保序文案 | 文字密度探针 |

## 调大小

脚本化走一次：2 秒时调到 1400×900，3 秒时调回 1200×800。
门禁读数前要求 `resize_restored=true` 且回到 1200×800。

## 门禁

| 门禁 | 阈值 | 类型 |
|------|------|------|
| policy | `present_policy == full_paint` | 硬 FAIL |
| presents | `present_count >= 1` | 硬 FAIL |
| 持续 tick FPS | `fps_interval >= 55` 且 `interval_p95_ms <= 22` | 硬 FAIL |
| 像素三探针 | 顶层底色 + mask 带 + 文字密度，全过 | 硬 FAIL |
| Golden | `testdata/showcase_modal_host_base.png` 零容差 | 硬 FAIL |
| 调大小回基线 | `resize_restored=true` 且 `client_px=1200x800` | 硬 FAIL |
| vsync | `vsync_source` 非空 | 硬 FAIL |
