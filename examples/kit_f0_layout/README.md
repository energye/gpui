# kit_f0_layout — F0-2 layout window

## 运行

```bash
export DISPLAY=:0 LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=5 go run ./examples/kit_f0_layout
# 门禁自检：
go run ./examples/kit_f0_layout -auto-only
```

窗口 1200×800 基线，可调大小。RUN_SECONDS>=5。
正确性窗：`slope_gate=off`（5 秒短窗）。

## 窗内呈现

| 段 | 内容 | 证明 |
|------|----------|----------|
| Row | 定宽 100 + 权重 1:2 + 间距 8（主题 SizeXS） | prim FlexLayout 权重分配 |
| Column | 三个定高行 + 间距 10 | 纵向依次排 |
| Stack | 底 190×110 + 定位 70×34 于 (24,18) | 叠放定位 |
| Wrap | 五个 120×30 块，420 宽换行 | 流式换行 |
| Virtual | 1000 行 × 32px，170 高视口 | 只建可见行，bind 远小于总数 |

## 调大小

脚本化走一次：2 秒时调到 1400×900，3 秒时调回 1200×800。
门禁读数前要求 `resize_restored=true` 且回到 1200×800。

## 门禁

| 门禁 | 阈值 |
|------|------|
| policy | `present_policy == full_paint` |
| presents | `present_count >= 1` |
| 持续 tick FPS | `fps_interval >= 55` 且 `interval_p95_ms <= 22` |
| 像素三探针 | 行块中心 + 叠放块中心 + 换行块中心，全过（容差≤8/通道） |
| Golden | `testdata/showcase_layout_base.png` 零容差 |
| 虚拟化 | `bind_count << item_count` |
| 调大小回基线 | `resize_restored=true` 且 `client_px=1200x800` |
| vsync | `vsync_source` 非空 |

## 三证据

* 逻辑：单测（`ui/kit/internal/prim` 三文件）+ 窗内行列叠换行几何进 JSON + `bind/items`
* 像素：三点中心采样 + Golden 掩码逐位对比
* Golden：静态掩码逐位对比，差异超限 FAIL
