# ui_render_base_cliplayer

**ClipLayer 轴** — Boundary / Clip / Opacity / Transform（`ENGINE_UI_RENDER_BASE` §24）。

## 覆盖

- 静态 `RepaintBoundary` 卡片（hot 动画时不应 layout storm）
- `ClipRect` 溢出滚动（paint）
- Opacity 脉动（paint alpha）
- `RenderTransform` → scene `TransformLayer` 旋转

## 三证据（2026-10-21）

| 证据 | 结果 |
|---|---|
| 逻辑探针 | 6/6（presents / paint / no layout storm / gpu ops / fallback=0 / boundary ≥8） |
| 像素断言 | 9/9（根底、两张静态卡、hot 面板、Clip 条纹、ClipRRect 子块、Opacity 混合、Transform 子块、标题墨色） |
| Golden | 整窗 **403200 px**，连续两跑 **0.0000%** |
| 自动门禁 | `-auto-only` 5s，连续两跑 exit 0；`presents≈300` |
| 人工窗 | `-manual-seconds 2` exit 0；Clip/Opacity/Transform 动画保留 |

Gate 档冻结 `phase`、`clipOff`、旋转变换与扩张圆角（初始 0/0/0/10），manual 档保留全部动画。

产物：

- GPU 快照：`testdata/showcase_cliplayer.png`（720×560）
- Golden：`testdata/showcase_cliplayer_base.png`
- JSON：gate 档 stdout 输出 `wrgate` 报告；逻辑/像素字段为 `logic_probes_*` / `pixel_probes_*`

## 运行

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

# 自动门禁：5s，冻结 Clip/Opacity/Transform
go run ./examples/ui_render_base_cliplayer -auto-only

# 人工窗：保留动画，不写快照
go run ./examples/ui_render_base_cliplayer -manual-seconds 5
```

播种 Golden 只在新基线时执行一次，之后必须不带种子连跑两次 0.0000%：

```bash
GPUI_ACCEPT_GOLDEN=1 go run ./examples/ui_render_base_cliplayer -auto-only
go run ./examples/ui_render_base_cliplayer -auto-only
go run ./examples/ui_render_base_cliplayer -auto-only
```

`RUN_SECONDS=N` 仍可覆盖 gate 时长；`SNAP_PNG=/path.png` 可覆盖快照路径。manual 档不会覆盖 Golden。
