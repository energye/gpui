# kit_f0_scope — F0-1 theme plus scope window

## 运行

```bash
export DISPLAY=:0 LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=5 go run ./examples/kit_f0_scope
# 门禁自检：
go run ./examples/kit_f0_scope -auto-only
```

窗口 1200×800 基线（U15），可调大小。RUN_SECONDS>=5（U16）。
正确性窗：`slope_gate=off`（5 秒短窗，E 族只采不判）。

## 窗内呈现

| 列 | Ctx | 证明 |
|------|----------|----------|
| default | `DefaultScopeCtx()` | 种子浅色主题直接解析：浅色背景块 |
| reskin | 主色换 `#722ed1` | 只换 Ctx，组件代码不动：同位置同尺寸，色块照换 |
| compact | 尺寸切 `small` | 尺寸档随 Ctx 走：标注行显示 small |

每列另有一行禁用行：白底 + 容器禁用 token 半透明盖上去，
图形处理器真混合，探针读混合后的色。

## 调大小

脚本化走一次：2 秒时调到 1400×900，3 秒时调回 1200×800。
门禁读数前要求 `resize_restored=true` 且回到 1200×800。
`EventResize` 走 `shell.Resize` 真布局，探针坐标每次按布局链重算。

## 门禁（Gate）

| 门禁 | 阈值 | 类型 |
|------|------|------|
| policy | `present_policy == full_paint` | 硬 FAIL |
| presents | `present_count >= 1` | 硬 FAIL |
| 持续 tick FPS | `fps_interval >= 55` 且 `interval_p95_ms <= 22` | 硬 FAIL |
| 像素三探针 | 2 点精确色 + 1 区文字密度，全过 | 硬 FAIL |
| Golden | `testdata/showcase_scope_base.png` 零容差，次跑起逐位一致 | 硬 FAIL |
| 调大小回基线 | `resize_restored=true` 且 `client_px=1200x800` | 硬 FAIL |
| vsync | `vsync_source` 非空（fallback 不许宣称锁 60Hz） | 硬 FAIL |

## 三证据

* 逻辑：单测（`ui/kit/internal/scope` 三文件）+ 窗内 `paint_count/presents` 同 JSON
* 像素：色块中心点采样（容差≤8/通道）+ 禁用混合色（容差≤12/通道，附混合公式）+ 文字区非背景像素数
* Golden：静态掩码逐位对比，差异超限 FAIL 并产差异定位
