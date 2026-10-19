# ui_render_base_pathshape — 路径造型三 API 真窗

覆盖 A 组缺窗清单第 2 项（`docs/RENDER_FEATURE_COVERAGE.md` §1 #4 / §3.1）：
`render.Path` 的 **Trim / WithCorners / Discrete**。这三个 API 此前全仓零调用、
无真窗（§1 #4 记 ⛔），本窗是它们的第一个直调画面证据，同时兼作 §22.1 窗测轴
（`examples/ui_render_base_*`）。

## 怎么跑

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

go run ./examples/ui_render_base_pathshape                 # 人工窗：常驻，关窗后出汇总
RUN_SECONDS=5 go run ./examples/ui_render_base_pathshape   # 定时窗（≥5s，U16）
go run ./examples/ui_render_base_pathshape -auto-only      # 门禁：JSON + exit code
```

首次跑会提示 golden 基线缺失；确认画面无误后用
`GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=5 go run ./examples/ui_render_base_pathshape -auto-only`
播种 `testdata/showcase_pathshape_base.png`，评审后随窗提交；之后每跑必须
**逐位零差**（`pixel_golden_diff_pct == 0`）。

## 画面（四张卡，1200×800）

| 卡 | 演示 | 为什么必须这样画 |
|----|------|------------------|
| A | 菱形：整条路径淡色描 3px，`Trim(0.25,0.6)` 保留段叠 8px 橙，两个切点画 amber 圆点 | 保留段亮、被切头尾只剩淡色 = 弧长子段真的只取中段；点分别落在菱形右顶点与底左边 40% 处 = 端点几何正确 |
| B | L 形六边形：尖角版先填暗紫，`WithCorners(20)` 版再填青色 | 每个角留一条暗紫月牙 = 圆角只削了角（12px 内缩处已是青色）；直边段整条青色 = 圆角没波及别处 |
| C | 三次曲线：淡色底 + `Discrete(22,0)` 亮绿 | 绿色只出现在每个周期的前半（11px），空档露出淡底色 = 周期 22 的 11 画/11 空 |
| D | 同曲线：淡色底 + `Discrete(22,7)` 洋红 | 洋红段明显在中心线两侧**交替** ±7 = 修掉了「恒单侧平移」；同一弧长处镜像点采样必须是卡背景 |

## 实现修正（2026-10-19，与建窗同批）

`render/pathops_subpath.go` 的 `Discrete` 实测与自身注释不符，已按注释 + Skia
按 `segLength` 切段的口径修正（**修前证据**：探针实测水平线 `Discrete(22,7)` 的
14 个顶点 y 全为 −7，即「±deviation 交替」从不交替）：

- **偏移方向**：原取号用 `step%2`，而 `emitOn` 与 `step` 每个 chunk 同步翻转 →
  画出的 chunk 永远落在奇数 step → 恒单侧 −deviation（是平移不是抖动）。
  改为按「已画 chunk 序号」交替，两侧真的交替 ±deviation。
- **虚线周期**：注释写「segment length ~segLen/2 drawn then gap」（周期 = segLen），
  实测却是「画 segLen、空 segLen」（周期 2×segLen）。改为 chunk = segLen/2、
  前半画后半空（Skia discrete 的切段间距 = segLength）。
- **配套**：新增 `render/pathops_subpath_test.go`（三 API 值级契约，含「两侧交替」
  「周期/占空」「端点/夹紧」）；回归 `TestP1_Capability_E02_PathEffectsGPU`、
  `TestP1_U1_VectorDesignToolDensity` 均绿。

## 三证据

1. **逻辑探针**：14 项值级契约（启动时跑一次，fixture 与 `render/pathops_subpath_test.go` 同一套）
   - A：`trim_len`（长度比 0.35 ± 1%）/ `trim_ends`（(225,132) 与 (106,177) ±0.05）/
     `trim_open`（结果必是开放折线，无 Close/曲线 verb）/ `trim_empty_inverted`（反向参数 → 空）
   - B：`corner_quads`（闭合 6 个、开放 4 个 QuadTo）/ `corner_open_ends`（开放折线首尾顶点不动）/
     `corner_clone_r0`（r=0 等价克隆）/ `corner_clamp`（r=1000 作用 20×20 方块不越界）
   - C/D fixture：`dash_period`（段长 11、间隔 11、在线上，占空 ~0.5）/ `dash_deterministic` /
     `jit_two_sided`（±7 交替、两侧均衡）/ `jit_deterministic`
   - 窗内实际曲线：`wave_dash_duty`（drawn/base ∈ 0.42–0.58）/ `wave_jit_two_sided`（两侧段各 ≥ 4）
2. **像素断言**：11 个探针（A 保留段/被切段/切点圆点、B 角部月牙/圆角内/直边、
   C 落笔段/空档、D +7 段/−7 段/同弧镜像处），期望值直接读调色板常量，
   容差 8/255（采样点都在色块中央，不碰 AA 边）
3. **Golden**：表头 + 图例 + 四张卡共 627200px 逐位零容差对比

另外整窗输出 §2.2 族 A–J 指标 JSON（`wrgate.BuildReport`），门禁模式跑
`EvaluateGates`（`present_policy=full_paint`）+ `vsync_source` 非空。

## 实测（2026-10-19，X11 + wgpu-native，3 连跑）

- 逻辑探针 14/14；像素 11/11；golden 第 2、3 跑 `diff=0.0000% / 627200px`
- `present_count 300`、`fps_interval ≈ 60.1`、`cpu_fallback_ops=0`、`gpu_fallbacks=0`
- 人工常驻窗用 `xwd` 实屏截图确认上屏（HUD `paint=715 presents=716` 持续增长）

## 已知限界（如实记录）

- 逻辑探针是值级契约（启动时一次），不依赖绘制路径；绘制正确性由像素探针 +
  golden 承担，两者互为独立证据。
- 本窗是**原子验证**（直调 render 接口一次验一族），不是生产接线；
  `ui/rendering` 等胶水层是否转用这三个 API 属于后续接线问题。
- 动画/FPS 门不在本窗口径内（正确性窗）：`fps_gate=off_correctness`、
  `slope_gate=off`，A–J 指标照常输出不带闸。
