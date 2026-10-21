# ui_render_graphics/vector

`render_graphics.md` §2（等级 B 2D 部分）矢量路径真窗：QuadTo / CubicTo / Arc / EllipticalArc。

六格画布（3×2）：

1. QuadTo stroke（平缓/尖锐/反转）
2. QuadTo 闭合 fill（泪滴 + 径向渐变）
3. CubicTo stroke（S 形/回环/直线退化）
4. CubicTo 闭合 fill（花瓣）
5. Arc 角度范围（90/180/270/350°）
6. EllipticalArc（rx:ry 比例）

贝塞尔格内同时画控制顶点与多边形（细灰线），便于核对曲线形状。

## 三证据（2026-10-21）

| 证据 | 结果 |
|---|---|
| 逻辑探针 | 5/5（presents / paint / gpu ops 或 picture ops / fallback=0 / picture ops） |
| 像素断言 | 8/8（两格底色、二次控制点、泪滴高光、花瓣 fill、Arc 红色段、顶栏底、图例底） |
| Golden | shell 掩码 **821184 px**（header + legend + body），连续两跑 **0.0000%** |
| 自动门禁 | `-auto-only` 5s，连续两跑 exit 0；`present_policy=full_paint`，`presents=300` |
| 人工窗 | `-manual-seconds 2` exit 0；静态画面 |

本窗是**静态画面**，不设持续 FPS 门禁；presents 只作观测记录，不拿静态窗数字冒充性能结论。Golden 排除底部 HUD，只比 Header + Legend + Body 的被测画面。

产物：

- GPU 快照：`testdata/showcase_vector.png`（1200×800）
- Golden：`testdata/showcase_vector_base.png`
- JSON：gate 档 stdout 输出 `wrgate` 报告；逻辑/像素字段为 `logic_probes_*` / `pixel_probes_*`

## 运行

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

# 自动门禁：5s，full paint，采快照并逐位对比 Golden
go run ./examples/ui_render_graphics/vector -auto-only

# 人工窗：静态常驻；不写快照
go run ./examples/ui_render_graphics/vector -manual-seconds 5
```

播种 Golden 只在新基线时执行一次，之后必须不带种子连跑两次 0.0000%：

```bash
GPUI_ACCEPT_GOLDEN=1 go run ./examples/ui_render_graphics/vector -auto-only
go run ./examples/ui_render_graphics/vector -auto-only
go run ./examples/ui_render_graphics/vector -auto-only
```

`RUN_SECONDS=N` 仍可覆盖 gate 时长；`SNAP_PNG=/path.png` 可覆盖快照路径。manual 档不会覆盖 Golden。
