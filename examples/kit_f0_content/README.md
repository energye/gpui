# kit_f0_content — F0-3 content window

## 运行

```bash
export DISPLAY=:0 LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=15 go run ./examples/kit_f0_content
# 门禁自检：
go run ./examples/kit_f0_content -auto-only
```

窗口 1200×800 基线，可调大小。RUN_SECONDS>=15（图片三态占 5s/9s/10s 相位）。
正确性窗：`slope_gate=off`（15 秒短窗不判爬升）。

## 窗内呈现

| 段 | 内容 | 证明 |
|------|----------|----------|
| single | 单行标签 | 区域墨量 ≥60 |
| multi | 220 宽换行两行 | 区域墨量 ≥60 |
| ellipsis | 220 宽 maxLines=2 省略 | 行数被夹到 2 |
| rich | 两 span 两色 | 区域墨量 ≥60 |
| icons | 12/16/20 三档 | 尺寸跟 Ctx 走 |
| picture | 占位→到图→错 | 到图与报错各只标脏一格（RepaintBoundary 截断：本格 NeedsPaint 真、兄弟格干净、无布局脏；全量重画帧数照计不混） |

## 调大小

脚本化走一次：2 秒时调到 1400×900，3 秒时调回 1200×800。
门禁读数前要求 `resize_restored=true` 且回到 1200×800。

## 门禁

| 门禁 | 阈值 |
|------|------|
| policy | `present_policy == full_paint` |
| presents | `present_count >= 1` |
| 持续 tick FPS | `fps_interval >= 55` 且 `interval_p95_ms <= 22` |
| 文字三区 | 单行/多行/富文本墨量 ≥60（有字体才判，无字体直接 FAIL） |
| 图片三相 | 到图相位成立 + 到图增量 1–3 + 报错增量 0–3 |
| Golden | `testdata/showcase_content_base.png` 零容差 |
| 调大小回基线 | `resize_restored=true` 且 `client_px=1200x800` |
| vsync | `vsync_source` 非空 |

## 三证据

* 逻辑：单测（`content_test.go` 估算+合并+三态+图标）+ 窗内墨量与图片增量进 JSON
* 像素：三区墨量采样 + Golden 掩码逐位对比
* Golden：静态掩码逐位对比，差异超限 FAIL
