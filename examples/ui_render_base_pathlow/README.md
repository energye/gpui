# ui_render_base_pathlow — 路径低频三 API 真窗

覆盖 A 组缺窗清单第 1 项（`docs/RENDER_FEATURE_COVERAGE.md` §1 #3 / §3.1）：
`render.Context` 的 **FillPreserve / StrokePreserve / GetCurrentPoint**（外加
`Fill`/`Stroke`/`ClearPath` 的对照行为）。这三个 API 此前全仓零调用、无真窗，
本窗是它们的第一个直调画面证据，同时兼作 §22.1 窗测轴（`examples/ui_render_base_*`）。

## 怎么跑

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

go run ./examples/ui_render_base_pathlow                 # 人工窗：常驻，关窗后出汇总
RUN_SECONDS=5 go run ./examples/ui_render_base_pathlow   # 定时窗（≥5s，U16）
go run ./examples/ui_render_base_pathlow -auto-only      # 门禁：JSON + exit code
```

首次跑会提示 golden 基线缺失；确认画面无误后用
`GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=5 go run ./examples/ui_render_base_pathlow -auto-only`
播种 `testdata/showcase_pathlow_base.png`，评审后随窗提交；之后每跑必须
**逐位零差**（`pixel_golden_diff_pct == 0`）。

## 画面（四张卡，1200×800）

| 卡 | 演示 | 为什么必须这样画 |
|----|------|------------------|
| A | 五边形：`FillPreserve()` 填青色 → 再 `Stroke()` 描 6px 橙边 | 两个颜色同时可见 = 填充没吃掉路径；若误用 `Fill()`，橙边必消失 |
| B | 圆角矩形：`StrokePreserve()` 先描 8px 品红 → 再 `Fill()` 深蓝 | 填充压住描边内半，外半留成品红环 = 描边被保留后又叠加填充 |
| C | 折线（直线/二次/三次/闭合）：每步后 `GetCurrentPoint()` 读回，并按**返回值**画黄点 | 黄点必须正落在绿线顶点上；返回值错一点，点就离开线，且精确比对同时失败 |
| D | 矩形：`Fill()` 之后再用 8px 亮色 `Stroke()` | 路径已被消费 → 边界处什么都不该出现；`D_absent` 像素采样证明"该有描边的地方是背景" |

## 三证据

1. **逻辑探针**（`paintProbe`，在真窗 OnPaint 里跑在光栅线程）：
   - `fill_preserve_keeps` / `stroke_consumes` / `stroke_preserve_keeps` /
     `stroke_fill_consumes` / `fill_consumes_d` / `empty_stroke_still_empty` /
     `clear_path_resets`：路径状态跨操作存续与否
   - `cp_move_to` / `cp_line_to` / `cp_quad_to` / `cp_cubic_to` / `cp_close`：
     `GetCurrentPoint()` 返回值与提交点**逐位相等**（先按活矩阵变换到设备空间再比，无容差）
2. **像素断言**：7 个探针（A 填充/描边、B 填充/环、C 两处黄点、D 缺失处），
   期望值直接读调色板常量，容差 8/255（采样点都在色块中央，不碰 AA 边）
3. **Golden**：表头 + 图例 + 四张卡共 627200px 逐位零容差对比

另外整窗输出 §2.2 族 A–J 指标 JSON（`wrgate.BuildReport`），门禁模式跑
`EvaluateGates`（`present_policy=full_paint`）+ `vsync_source` 非空。

## 实测（2026-10-19，X11 + wgpu-native，3 连跑）

- 逻辑探针 12/12，`cp_exact/cp_calls = 1806/1806`、`1812/1812`（多次重录全对）
- 像素 7/7；golden 第 2、3 跑 `diff=0.0000% / 627200px`
- `present_count 300/301`、`fps_interval ≈ 60`、`cpu_fallback_ops=0`、`gpu_fallbacks=0`

## 已知限界（如实记录）

- 黄点用 `Push + SetTransform(Identity)` 在设备空间落点，画面证明力在 **dpr=1**
  下成立；`GetCurrentPoint()` 的精确比对不依赖 dpr（比的是活矩阵变换后的值）。
- 本窗是**原子验证**（直调 render 接口一次验一族），不是生产接线；
  `ui/rendering` 等胶水层是否转用这三个 API 属于后续接线问题。
- 动画/FPS 门不在本窗口径内（正确性窗）：`fps_gate=off_correctness`、
  `slope_gate=off`，A–J 指标照常输出不带闸。
