# kit notice-queue — G2 message plus notification window

## 运行

```bash
export DISPLAY=:1 LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=5 go run ./examples/kit/notice-queue
# 门禁自检：
go run ./examples/kit/notice-queue -auto-only
```

窗口 1200×800 基线，可调大小。RUN_SECONDS>=5。
正确性窗：`slope_gate=off`（5 秒短窗，E 族只采不判）。

## 窗内呈现

| 区 | 内容 | 证明 |
|------|----------|----------|
| 顶中两条 | message 轻条纵向堆叠，top=8 起排 | 超数丢最旧，单测断 FIFO |
| 右两卡 | topRight 独立池 x2，不串池 | 各池独立，单关保邻 |
| 左下卡 | bottomLeft 独立池 | 六角池占位，后补全引擎 |
| 右小卡 | 换肤 `#722ed1` 静态调用 | 只换 Ctx，组件代码不动 |
| 底细条 | 进度色带 | showProgress 走主题主色 |
| 说明块 | 单关保邻加悬停冻结文案 | 文字密度探针 |

## 调大小

脚本化走一次：2 秒时调到 1400×900，3 秒时调回 1200×800。
门禁读数前要求 `resize_restored=true` 且回到 1200×800。

## 门禁

| 门禁 | 阈值 | 类型 |
|------|------|------|
| policy | `present_policy == full_paint` | 硬 FAIL |
| presents | `present_count >= 1` | 硬 FAIL |
| 持续 tick FPS | `fps_interval >= 55` 且 `interval_p95_ms <= 22` | 硬 FAIL |
| 像素三探针 | 消息底色 + 通知底色 + 文字密度，全过 | 硬 FAIL |
| Golden | `testdata/showcase_notice_queue_base.png` 零容差 | 硬 FAIL |
| 调大小回基线 | `resize_restored=true` 且 `client_px=1200x800` | 硬 FAIL |
| vsync | `vsync_source` 非空 | 硬 FAIL |
