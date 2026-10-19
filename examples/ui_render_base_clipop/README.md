# ui_render_base_clipop — 裁剪运算（difference / replace）真窗

覆盖 A 组缺窗清单第 5 项（`docs/RENDER_FEATURE_COVERAGE.md` §1 #13 / §3.1）：
`render.Context` 的 **ClipRectOp / ClipPathOp**（`ClipOpIntersect` / `ClipOpDifference` /
`ClipOpReplace`），配合 **ClipRect / ResetClip / SetFillRule**。本族此前全仓非测试
零调用（§1 #13 记 ⛔），本窗是它们的第一个直调画面证据，同时兼作 §22.1 窗测轴
（`examples/ui_render_base_*`）。

## 怎么跑

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

go run ./examples/ui_render_base_clipop                 # 人工窗：常驻，关窗后出汇总
go run ./examples/ui_render_base_clipop -manual-seconds 15  # 人工窗，自动关闭
RUN_SECONDS=5 go run ./examples/ui_render_base_clipop   # 定时窗（≥5s，U16）
go run ./examples/ui_render_base_clipop -auto-only      # 门禁：JSON + exit code
```

首次跑会提示 golden 基线缺失；确认画面无误后用
`GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=5 go run ./examples/ui_render_base_clipop -auto-only`
播种 `testdata/showcase_clipop_base.png`，评审后随窗提交；之后每跑必须
**逐位零差**（`pixel_golden_diff_pct == 0`）。

逻辑探针在启动时于离屏 CPU 上下文跑一次：探针阶段进程内临时置
`GOGPU_RENDER_MODE=cpu`（跑完还原），避免开窗前多建一块 GPU 设备；窗口本体仍按
调用方环境走 GPU。

## 画面（四张卡，1200×800）

| 卡 | 演示 | 为什么必须这样画 |
|----|------|------------------|
| A | `ClipRectOp` difference：先 `ClipRect` 建立基础裁剪，再差集挖两个洞——整数洞 (40,40,70,70) + 分数洞 (150.25,40.25,70,70) | 整数洞钉基础差集；分数洞四条边（左 150.25 / 上 40.25 / 右 220.25 / 下 110.25）落在像素内部，四个探针按解析覆盖率钉 25% / 43.75% / 75% / 81.25% 部分覆盖 = 差集有 AA，同时是「覆盖只应用一次」的修复回归钉 |
| B | `ClipRectOp` replace：先 `ClipRect` 左半 (0..215)，再 replace 成新带 (150,60,200,110) | 旧裁剪必须被丢弃：旧区域（带外左侧/下方）读卡片底色 = replace 语义（Skia `Op::kReplace`）真生效 |
| C | `ClipPathOp` difference × 左 NonZero / 右 EvenOdd：同向嵌套方形（外 150² + 内 70²，同绕向）差集 | NonZero 下嵌套同向 = 实心扣除（中心被挖）；EvenOdd 下 = 环扣除（中心保留）——左右对照钉死「裁剪路径吃 fill rule」的透传（修复回归钉） |
| D | `ClipPathOp` replace：两个不相交矩形 + 一个开放三角形（不 ClosePath） | 不相交子路径全保留（不是只留第一个）；开放子路径 fill 时隐式闭合（重心探针在内、底边下方探针在外） |

## 实现修正（2026-10-19，与建窗同批）

四个缺陷：前三处建窗前用 CPU/GPU 探针程序值级复现确认，第四处在建窗跑像素
探针时暴露（分数边说好的 AA 读出来全是被平方的浅值）。

### ① 多子路径裁剪 mask 损坏

`render/internal/clip/mask.go` 的栅格器把每个子路径当独立 mask 处理：子路径之间的
边互相串扰、`Close` 指到错误的起点 → 同一条裁剪路径里放多个子路径时覆盖错乱
（嵌套/不相交多子路径直接画错，本窗 C/D 卡的场景）。**修法**：`MaskClipper` 重写为
整条路径的联合光栅化——按子路径重建边表、`Close` 回到本子路径起点，winding 数
（NonZero）或奇偶计数（EvenOdd）统一求解。

### ② 裁剪路径 fill rule 恒 even-odd

`Clip()`/`ClipPreserve()`/`ClipPathOp` 建立裁剪路径时不读当前 `SetFillRule` 状态
（内部恒按 even-odd 光栅化）——NonZero（默认）下嵌套同向子路径的总覆盖应为实心
（内圈 winding=2），却被挖成环形。**修法**：`Context` 新增 `gpuClipFillRule` 记录当前
fill rule，`Clip()/ClipPreserve()/ClipPathOp()` 取值 `c.paint.FillRule` 一路透传到
CPU mask 栅格器与 GPU 侧（矩形系 `ClipRectOp` 本无 fill rule 概念，GPU 侧状态归位
NonZero）；GPU 深度裁剪路径同批透传——
**`PathClipAware.SetClipPath(path, fillRule)` 签名原子切换**（调用点一次改完），
sampleCount==1 时 R8 覆盖 mask 按 even-odd 光栅化，与 winding stencil 相交后语义一致。

### ③ 矩形差集无 AA

`ClipRectOp(ClipOpDifference)` 的洞边界按整数像素取整（分数边直接 hard edge），
Skia 口径应给解析部分覆盖。**修法**：`render/internal/clip/difference.go` 的矩形差集
改为解析面积覆盖（像素与洞矩形的交叉面积闭式求解）。

### ④ 分数覆盖被应用两次（验收实测暴露）

建窗后像素探针先读出「平方值」：25% 边读 6.25%、75% 边读 56.25%（永久测试同口径
实证：左边缘应 ~(191,229,191)，去掉修复后 CPU 回退变体实读 239,248,239）。两处
独立根因，两处都修：

- **GPU 走图路线**：`fillMaskedAsImage` 把裁剪覆盖率 `ClipCoverage` 烘焙进 src 图，
  随后 R8 mask 平面（同源裁剪 mask）再乘一次 → 覆盖²。修：src 只栅格形状本身，
  裁剪统一交给 GPU 侧（scissor / clipBG / 深度裁剪 / mask 平面）执行。
- **CPU 回退路线**：`installGPUClipMask` 写入的是合成 mask（裁剪 × 用户 mask），
  软渲染器再把 `ClipCoverage` 乘一次 → 覆盖²。修：CPU 回退前把 `MaskCoverage`
  归一化回纯用户 mask；`applyMaskToPaint` 每次重置，防嵌套 doFill/doStroke 残留。

修后插桩实证：srcA=255（src 不再烘焙裁剪）、mask=64、finalA=64 = 恰好单次应用。

## 三证据

1. **逻辑探针**：14 项值级契约（启动时跑一次，离屏 CPU 上下文逐像素；分数期望是
   闭式像素交叠，不调 render 内部 mask 工具）
   - 矩形差集：`rect_diff_hole_int`（整数洞精确）、`rect_diff_frac_left25` /
     `rect_diff_frac_right75` / `rect_diff_frac_corner`（0.25 / 0.75 / 0.4375 解析
     覆盖）、`rect_diff_union_overlap`（重叠洞按并集挖一次）、
     `rect_diff_from_no_base`（空栈差集 = 画布减路径）
   - replace：`rect_replace_discards`（旧裁剪丢弃）
   - 路径差集：`path_diff_nonzero_nested`（NonZero 嵌套同向 = 实心扣除）、
     `path_diff_evenodd_nested`（EvenOdd = 环扣除）、`path_diff_disjoint_kept`
     （不相交子路径都扣除、中间缝保留）
   - 路径 replace：`path_replace_disjoint`（两个不相交子路径都保留）、
     `path_replace_open_triangle`（开放三角形隐式闭合）
   - 其他：`clip_rect_intersect`（默认 intersect = 矩形 ∩ 路径）、
     `reset_clip_restores`（ResetClip 恢复全画布）
2. **像素断言**：23 个探针（A 7 / B 4 / C 6 / D 6），oracle = **设备空间解析覆盖
   混合**（卡片底色 × 填充色，分数边按「像素与洞的交叉长度/面积」闭式算覆盖率，
   再按 dpr 落到设备像素）；每个探针另配独立通道带（±宽裕）防 oracle 本身回归时
   假绿；容差 8/255
3. **Golden**：表头 + 图例 + 四张卡共 627200px 逐位零容差对比

另外整窗输出 §2.2 族 A–J 指标 JSON（`wrgate.BuildReport`），门禁模式跑
`EvaluateGates`（`present_policy=full_paint`）+ `vsync_source` 非空。

## 实测（2026-10-19，X11 + wgpu-native）

- 逻辑探针 14/14；像素 23/23，四组分数边 AA 探针读值与解析 oracle 差 ≤1/255
  （如 25% 左边读 g=0.294 vs oracle 0.296；75% 右边读 0.616 vs 0.619）
- GPU 路线：`gpu_ops=4018 cpu_fallback_ops=0 gpu_fallbacks=0`（像素探针验的是
  GPU 路径而不是 CPU 兜底）
- golden：seed 后复跑 `diff=0.0000% / 627200px`（gate 与 manual 两种模式都复跑过）
- 门禁：`present_policy=full_paint`、`vsync_source=true`、`present_count≈55`
- 人工窗（`-manual-seconds 6`）：真事件到达打日志（`event #61 type=wake`）+ 标题
  同步刷新；关窗汇总 `presents=65 logic=14/14 pixel=23/23 golden=0.0000% mode=manual`
- 回归：新增 12 项测试（`render/clip_ops_semantics_test.go` 8 项含 CPU 回退变体 +
  `render/internal/clip/mask_fillrule_test.go` 4 项值级）；`render/internal/clip`、
  p12、s3c M3 裁剪、context_clip 系、depth clip 系、L.06 mask 全绿；`Clip|Mask`
  全量过滤的失败项与基线 worktree 逐一对比一致（`TestClipCircle_CPUPixels`、
  `TestStencilRenderer_EnsureDepthClipPipelines`、clipping 严格档 2 区为**既有红**，
  非本次回归）

## 已知限界（如实记录）

- **GPU 深度裁剪 even-odd 限界（sampleCount>1）**：`render/internal/gpu/depth_clip.go`
  的 stencil 路径恒按 winding 光栅化；sampleCount==1 时有 R8 mask 相交（even-odd
  正确），sampleCount>1（MSAA）时无 mask 可交 → even-odd 裁剪路径在该模式下退化为
  NonZero（语义差异仅影响 winding≥2 的嵌套同向路径区域）。
- **分数坐标 intersect 矩形裁剪仍走 scissor**：`ClipRectOp(ClipOpIntersect)` 的
  轴对齐矩形裁剪走硬件 scissor（整数像素边界），分数坐标不产生 AA 边；
  difference/replace 走 mask 有解析 AA（本窗 A 卡实测）。
- 本窗是**原子验证**（直调 render 接口一次验一族），不是生产接线；`ui/rendering`
  等胶水层是否转用属后续接线问题。
- 动画/FPS 门不在本窗口径内（正确性窗）：`fps_gate=off_correctness`、
  `slope_gate=off`，A–J 指标照常输出不带闸。
