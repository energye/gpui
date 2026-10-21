# ui_render_base_geometry

**Geometry 轴**真窗专项 — [`docs/ENGINE_UI_RENDER_BASE.md`](../../docs/ENGINE_UI_RENDER_BASE.md) **§22 序5** / §2–§3 / §23。

实现入口：`ui/rendering` 的 `FillRect` / `Stroke*` / `FillCircle` / …（`draw.go`），**不是**示例私有重实现。

## 覆盖

| 面板 | API（`ui/rendering`） | 母表 ID |
|------|----------------------|---------|
| Fill/Stroke rect | `FillRect` / `StrokeRect` | FC-DRAW-RECT |
| Fill/Stroke rrect | `FillRoundRect` / `StrokeRoundRect` | FC-DRAW-RRECT |
| Linear gradient | `FillLinearGradient` | FS-LINEAR |
| Lines | `StrokeLine` | FC-DRAW-LINE |
| Circle | `FillCircle` / `StrokeCircle` | FC-DRAW-CIRCLE |
| Clip rrect | `PaintContext.PushClipRRect` | FC-CLIP-RRECT（paint） |
| Radial | `FillRadialGradient` | FS-RADIAL |
| Path | `NewPath` + `FillPath` / `StrokePath` | FC-DRAW-PATH |
| Oval + Arc | `FillOval` / `StrokeArc` | FC-DRAW-OVAL / ARC |

## 三证据（2026-10-21）

| 证据 | 结果 |
|---|---|
| 逻辑探针 | 6/6（presents / paint / no layout storm / gpu ops / fallback=0 / boundary 面板 ≥3） |
| 像素断言 | 9/9（根底、面板底、FillRect、RRect、Stroke 面板、脉动圆、Clip 脉动条、Radial、顶部底） |
| Golden | 整窗 **409200 px**，连续两跑 **0.0000%** |
| 自动门禁 | `-auto-only` 5s，连续两跑 exit 0；`presents≈300` |
| 人工窗 | `-manual-seconds 2` exit 0；三个脉动面板恢复动画 |

Gate 档冻结 StrokePulse / CirclePulse / ClipPulse 三个 phase（初始半径、线宽、裁剪偏移固定），manual 档保留脉动。

说明：本窗顶部标签没有设置 `face`，实际 GPU 画面不显示这些文字；像素断言只按实际可见的几何采样点，Golden 也按实际画面逐位对比，**不声称不可见标签文字已断言**。

产物：

- GPU 快照：`testdata/showcase_geometry.png`（660×620）
- Golden：`testdata/showcase_geometry_base.png`
- JSON：gate 档 stdout 输出 `wrgate` 报告；逻辑/像素字段为 `logic_probes_*` / `pixel_probes_*`

## 不在本轴

- Path.computeMetrics、全量 Paragraph、Transform 逆 hit
- dirty-rect Present / Picture 显示列表（序13）
- ClipRRect **Layer** RO 接线（FL-CLIP-RRECT 仍 C）

## 运行

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

# 自动门禁：5s，冻结三个脉动面板
go run ./examples/ui_render_base_geometry -auto-only

# 人工窗：保留脉动，不写快照
go run ./examples/ui_render_base_geometry -manual-seconds 5
```

播种 Golden 只在新基线时执行一次，之后必须不带种子连跑两次 0.0000%：

```bash
GPUI_ACCEPT_GOLDEN=1 go run ./examples/ui_render_base_geometry -auto-only
go run ./examples/ui_render_base_geometry -auto-only
go run ./examples/ui_render_base_geometry -auto-only
```

`RUN_SECONDS=N` 仍可覆盖 gate 时长；`SNAP_PNG=/path.png` 可覆盖快照路径。manual 档不会覆盖 Golden。

单测：`go test ./ui/rendering -run TestDraw_ -count=1`
