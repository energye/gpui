# ui_render_base_sweepgradient — Sweep（锥形）渐变真窗

覆盖 A 组缺窗清单第 4 项（`docs/RENDER_FEATURE_COVERAGE.md` §1 #9 / §3.1）：
`render.SweepGradientBrush` 的 **NewSweepGradientBrush / SetEndAngle / SetExtend /
ColorAt** 以及 `ui/rendering` 门面 **FillSweepGradient**。Sweep 族此前无独立真窗
（§1 #9 记 ⛔），本窗是它们的第一个直调画面证据，同时兼作 §22.1 窗测轴
（`examples/ui_render_base_*`）。

## 怎么跑

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

go run ./examples/ui_render_base_sweepgradient                 # 人工窗：常驻，关窗后出汇总
go run ./examples/ui_render_base_sweepgradient -manual-seconds 15  # 人工窗，自动关闭
RUN_SECONDS=5 go run ./examples/ui_render_base_sweepgradient   # 定时窗（≥5s，U16）
go run ./examples/ui_render_base_sweepgradient -auto-only      # 门禁：JSON + exit code
```

首次跑会提示 golden 基线缺失；确认画面无误后用
`GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=5 go run ./examples/ui_render_base_sweepgradient -auto-only`
播种 `testdata/showcase_sweepgradient_base.png`，评审后随窗提交；之后每跑必须
**逐位零差**（`pixel_golden_diff_pct == 0`）。

## 画面（四张卡，1200×800）

| 卡 | 演示 | 为什么必须这样画 |
|----|------|------------------|
| A | 7 色标整圈色轮（红→黄→绿→青→蓝→洋红→红），起点 0°（东） | 六个精确停点方向与两个中间角都被像素探针钉住 = 整圈映射（含 sRGB→线性插值）真对；起点在东= 0° 基准方向正确 |
| B | `FillSweepGradient` 门面直画矩形，红→蓝双色标整圈 | 门面真被调用（此前全仓零调用）；缝（t=1→0 硬切换）避开探针，15°/90°/180°/270° 四点按 t=0.042/0.25/0.5/0.75 校验 = 门面构造的刷子与探针 oracle 同参数同结果 |
| C | 左：起始角 π/2（绿→红整圈）；右：负向整圈 −2π（红→蓝） | 左圆「正下=绿（t=0）、东=t 0.75」证明起始角旋转生效；右圆下/上/东三点按 t=0.75/0.25/0 校验 = 负向扫掠方向与 `SetEndAngle(-2π)` 真生效 |
| D | 同参数局部扫掠 0→π/2（红→蓝）× Pad / Repeat / Reflect 三圆 | 112.5°/157.5° 落在范围外（raw t=1.25/1.75）：Pad 钳末端、Repeat 折 0.25/0.75、Reflect 三角折 0.75/0.25 → 三圆在范围外的六个判别探针是 2026-10-19 shader 修复的回归钉 |

## 实现修正（2026-10-19，与建窗同批）

**Sweep 的 ExtendRepeat/ExtendReflect 在部分扫掠下被画成 Pad**（范围外区域
全部钳到末端色）。**修前证据**：D 卡 Repeat/Reflect 圆在 112.5° 实测与 Pad 相同
（末端蓝），而 CPU oracle（同参数 `ColorAt`）给出 frac/triangle 值 —— 画与算直接
不一致；永久测试复现 `repeat@112.5°` 读到 (5,0,254) 而模型是 (99,0,240)。

根因在 GPU 侧 1D ramp 覆盖路径（`render/internal/gpu/`）：

- **ramp 域收窄**：`fillSweepGradientFieldMasked` 的 `tMin/tMax` 按 AABB 角域收窄
  （为主覆盖域服务的优化）对**所有** Extend 生效，shader 里 `(t-tMin)*invSpan`
  再钳到 [0,1] → 范围外的 raw t 永远落到 ramp 边缘 = Pad 行为，wrap/mirror 语义
  整个丢失。
- **修法（按 Skia tile-on-t 口径）**：非 Pad 时 ramp 固定整周期 `[0,1]`，在 cover
  片元里对 raw 参数 t 折叠后再查 ramp：`repeat: t-floor(t)`；`reflect: 三角波`。
  Pad 保留原角域收窄；整圈扫掠折叠恒等 → **Pad 与整圈输出逐位不变**（extend
  uniform 仅在 sweep 模式生效，linear/radial 调用点补 0 参数不受影响）。
- **改动**：`brush_fill.go`（域+tileMode 透传）、`session_textured_cover.go` /
  `render_session.go`（`TexExtend` uniform 通道）、三份 WGSL shader
  （`shaders/cover_textured_linear.wgsl`、`textured_stencil_linear.go`、
  `linear_ramp_mask.go` 均加 `sweep_tile()`）。
- **配套回归**：`render/p1_capability_matrix_closers_test.go` 新增
  `TestP1_Capability_D03_SweepGradientTileExtendGPU`（GPU 上重复/反射/整圈恒等
  判别采样，tol 12/255）；窗内 D 卡 6 探针与永久测试双钉。

## 三证据

1. **逻辑探针**：14 项值级契约（启动时跑一次，独立 sRGB 数学：标准传输函数 +
   线性光混合，不调 render 的混合工具）
   - 整圈：`full_stop0_east`（东=首停点精确）、`full_mid_linear`（中角=线性光
     混合 0.7354 而非 0.5，证插值在线性光做）、`full_stop_frac`（60° 恰落黄停点）、
     `full_center_first`（圆心退化=首停点色）
   - 起始角/负向：`startangle_rotate`（π/2：下=绿 t0、东=t0.75）、
     `neg_full_mirror`（−2π：下 0.75/上 0.25/东 0）
   - 局部 Extend：`partial_pad_clamp` / `partial_repeat_frac` / `partial_reflect_tri` /
     `partial_reflect_157`（四个方向外判别点各自独立算）
   - 恒等与退化：`extend_full_identity`（整圈三种 Extend 逐值相等）、
     `stops_unsorted`（乱序停点等价有序）、`single_stop_const`（单停点常量）、
     `zero_sweep`（零扫掠=首停点）
2. **像素断言**：22 个探针（A 8 / B 4 / C 4 / D 6），oracle = 同参数 CPU 刷子
   `ColorAt` 在**像素中心** (px+0.5, py+0.5) 的值（与永久测试同一口径，tol
   12/255 = ramp texel 量化 + 像素中心取整），每个探针另配独立通道带（±宽裕）
   防 oracle 本身回归时假绿
3. **Golden**：表头 + 图例 + 四张卡共 627200px 逐位零容差对比

另外整窗输出 §2.2 族 A–J 指标 JSON（`wrgate.BuildReport`），门禁模式跑
`EvaluateGates`（`present_policy=full_paint`）+ `vsync_source` 非空。

## 实测（2026-10-19，X11 + wgpu-native，3 连跑 + 人工窗）

- 逻辑探针 14/14；像素 22/22；golden 第 2/3/4 跑均 `diff=0.0000% / 627200px`
- `present_count ≈ 206`、早退 `cpu_fallback_ops=0`、`gpu_fallbacks=0`
- 人工常驻窗（不设 RUN_SECONDS）用 `xwd` 实屏截图确认上屏（标题随事件刷新
  `event #1431`，HUD `paint=1431 presents=1431` 持续增长）；真实
  `close-requested` 事件关窗，汇总 `presents=3651 logic=14/14 pixel=22/22
  golden=0.0000% mode=manual`

## 已知限界（如实记录）

- **AA 矩形 sweep/radial 每帧 CPU 重建 ColorAt 场**（既有性能特性，非本次引入）：
  `render/internal/gpu/brush_native.go:155` 对 AA 矩形优先走
  `fillRadialSweepFieldNative` → `fillColorAtFieldNative`（:614），每帧在 CPU 上
  重建 cap 512² 的 2D 场（本窗 B 卡 382×164 = 62648 texel，~20ms/帧 + 250KB/帧
  分配）。定量证据（性能实验，已还原）：全场景 fps≈41 / cpu_raster≈105%；
  仅圆卡场景 60 / 25.8%；仅 B 矩形卡 43 / 105%；同面积圆 r=140 仍 60；直调画刷
  矩形（不经门面）42.7 → 元凶是矩形场路径而非门面。radial 同样中招；圆/非矩形
  走 1D ramp textured cover 很便宜。修它（genID 缓存或 GPU 端场）超出本项验收
  口径，记为后续优化子项。
- 本窗是**原子验证**（直调 render 接口 + 门面各一次），不是生产接线；
  `ui/rendering` 等胶水层是否转用属后续接线问题。
- 动画/FPS 门不在本窗口径内（正确性窗）：`fps_gate=off_correctness`、
  `slope_gate=off`，A–J 指标照常输出不带闸。
