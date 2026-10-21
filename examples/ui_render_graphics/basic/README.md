# ui_render_graphics/basic

`render_graphics.md` §1（等级 A）raster primitives 真窗：Point / LineSegment / Triangle。

六格画布（3×2），每格只验证一族：

1. Point 多半径（2/4/8/14px）
2. Point 点阵网格
3. LineSegment 角度（0/90/30/45/60°）
4. LineSegment 线宽（1/2/4/6/8px）
5. Triangle fill（等边/直角/倒）
6. Triangle stroke（细长/描边）

Triangle 没有专用 API，用 `MoveTo + LineTo + ClosePath` 路径绘制。

## 三证据（2026-10-21）

| 证据 | 结果 |
|---|---|
| 逻辑探针 | 5/5（presents / paint / gpu ops 或 picture ops / fallback=0 / picture ops） |
| 像素断言 | 9/9（两格底色、两个 Point、点阵中心、Triangle fill、线宽 6、顶栏底、图例底） |
| Golden | shell 掩码 **821184 px**（header + legend + body），连续两跑 **0.0000%** |
| 自动门禁 | `-auto-only` 5s，连续两跑 exit 0；`present_policy=full_paint` |
| 人工窗 | `-manual-seconds 2` exit 0；静态画面 |

本窗是**静态画面**：gate 档只靠 HUD/窗口呈现调度出帧，`presents≈15`（manual 2s 为 8），**不设持续 FPS 门禁**；不把静态窗的 presents 当性能结论。Golden 排除底部 HUD，只比 Header + Legend + Body 的被测画面。

产物：

- GPU 快照：`testdata/showcase_basic.png`（1200×800）
- Golden：`testdata/showcase_basic_base.png`
- JSON：gate 档 stdout 输出 `wrgate` 报告；逻辑/像素字段为 `logic_probes_*` / `pixel_probes_*`

## 运行

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

# 自动门禁：5s，full paint，采快照并逐位对比 Golden
go run ./examples/ui_render_graphics/basic -auto-only

# 人工窗：静态常驻；不写快照
go run ./examples/ui_render_graphics/basic -manual-seconds 5
```

播种 Golden 只在新基线时执行一次，之后必须不带种子连跑两次 0.0000%：

```bash
GPUI_ACCEPT_GOLDEN=1 go run ./examples/ui_render_graphics/basic -auto-only
go run ./examples/ui_render_graphics/basic -auto-only
go run ./examples/ui_render_graphics/basic -auto-only
```

`RUN_SECONDS=N` 仍可覆盖 gate 时长；`SNAP_PNG=/path.png` 可覆盖快照路径。manual 档不会覆盖 Golden。
