# P3.8 批②（精度批）挂起登记：acc=6 虚线圆单行内填 + 图像 2× 上采样差距

- 日期：2026-11-09
- 状态：**引擎修复已就位并自测绿；单行内填已修（2026-11-09）；acc=6 全局启用已由用户 2026-11-09 明确拍板「不启用」——继续维持标准对齐的固定 2，生产恒 acc=2**（引擎修复与 acc=6 机器保留但不接线；本文档标题及 §1 沿用的「挂起」字样指当时登记的两个问题，其中单行内填已修，方向以本行终态为准，历史评估里「acc=6 待启用」的登记同样被本次拍板覆盖）。
- 触发：用户在真窗 `go run ./examples/ui_wr_p38_svg_showcase` 人工复验，报两处与
  resvg 参照不一致：① ②⑱ 蓝虚线圆「粗」、⑭ 蓝虚线×2；② ⑧ 橙色 logo（外链图）轮廓锯齿。

## 0. 本批已落地的引擎修复（acc=2 生产态逐位不变，留作 acc=6 铺路）

1. **AET 重复插边**（`analytic_filler.go`）：延迟边按 upperY 插入时 `edgeIdx` 被无条件
   回写拉低，下一行 first-pass 重复插入已入表边 → winding 失衡（acc=6 实测 row57 src=215、
   row58 src=303/304 各插两次，sumWinding −2）。改单调高水位。HEAD 即有的存量洞。
2. **过期太早**（`curve_aet.go` `RemoveExpiredSubpixelRow` + 调用）：线边按像素空间
   LowerY 过期（4e #3），acc=6 不再在行首误删「真结束在行中」的边。**acc=2 下与旧
   BottomY 规则数学恒等**（LowerY/65536 ≡ (LastY+1)/aaScale），逐位不变。

钉测 `render/internal/raster/pixel_accuracy_chord_test.go`：CrossingFan 双 acc PASS；
DenseShortEdges acc=6 avg 1.001 / max 51 / over8 116（acc=2 为 1.016 / 62 / 170，作护栏）；**2026-11-09 短边修复后 acc=6 为 avg 0.388 / max 36 / over8 44，acc=2 不变**。
`./render/internal/raster/`、`./render/svg/` 全绿。

## 1. 已修复问题 A（2026-11-09）：acc=6 虚线圆「单行内填」（真·acc=6 回归）

- 现象：acc=6 时，虚线圆（`<circle cx=540 cy=520 r=46 stroke-width=6 stroke-dasharray="30 14"
  stroke-linecap=round>`）在**恰好一行**把圆内部整条填满（输出 y=284，蓝连续 273→327，55px）；
  acc=2 与 resvg 参照均为左右两小段笔画（各 ~5px）。
- 实测（570×357 输出坐标，blue x-runs）：

  | y | acc=2 | acc=6 | ref |
  |---|---|---|---|
  | 282 | 274-277, 323-326 | 同左 | 同左 |
  | **284** | **273-277, 323-327** | **273-327 (55px 连片)** | 273-277, 323-327 |
  | 286 | 274-276, 324-327 | 同左 | 同左 |

- 性质：acc=2 vs acc=6 整图 diff mean 2.34（集中在个别行）；即 acc=6 特有的单行 winding
  配对错误（该子条带把左右两段描边之间误判为填充区间）。
- 已排除（均无效）：AET 重复插边修复、像素空间过期修复、`edgeCrossingY` 实际交点切分、
  4e #2「延迟边 UpperY 入条带边界」。→ 指向「虚线描边轮廓（stroke outline）× 解析填充」
  在 acc=6 的更深处（疑点：`processSubStripIncremental` 无 `edges_too_close` 合并、
  近竖直/近切点边在 1/64 吸附下的配对次序）。**下一步需在隔离的最小虚线圆上逐条带 trace。**
  > 上述「指向 filler」的疑点已被后续 trace 推翻：真正的洞在**边的入口**，见下节修复记录。

### 修复记录（2026-11-09，终态）

- 真因：`NewLineEdgeWithAccuracy` 用 `top == bottom`（`FDot6Round` 后同一子像素行）当
  「水平线」跳过。虚线圆的 round-cap 轮廓里有一条很短、很陡的 cap 边
  （设备 y≈284.668→284.835，|DX|≈9.9）；两端 `FDot6Round` 都落到 1139，被整条丢掉。
  该行本来有 4 个轮廓交点（外左 -1、内左 +1、cap 内 -1、cap 内 +1），丢掉 cap 的 +1 后
  只剩 3 个，winding 永远不回零，filler 把外左与外右配成一条横跨圆内的梯形 →
  y=284 一行 55px 连片。实线圆同理，只是命中行在 y=281。
- 为什么只在 acc=6 可见：acc=2 的像素空间吸附更粗，丢掉这条短边后 winding 序列仍能在
  外/内边处回零（视觉正确）；acc=6 的 1/64 吸附把 cap 边与 inner 边分开，缺边才暴露。
- 修法：只把**真正水平**（`y0 == y1`）的线判为退化；`top == bottom` 但 `y0 != y1` 的短陡边
  在细精度下保留，`LastY = top` 给 AET 一个子像素行的生命周期（filler 用精确的像素空间
  `UpperY/LowerY` 284.668..284.835）。**acc=2 仍走旧跳过分支**，保持生产逐位不变。
- 验证：
  - 隔离红测 `render/p38_acc6_stroke_inner_fill_test.go`（虚线 y=284/289、实线 y=281，
    断言圆内 x=290..310 alpha≤8）：修前红、修后绿。
  - acc=2 逐位：同一虚线/实线圆的 SHA256 修前修后完全一致
    （`6d6719…` / `1922c4…`）。
  - `render/internal/raster` 全绿；`DenseShortEdges` acc=6 从 avg 1.001 / over8 116
    改善到 **0.388 / 44**，acc=2 护栏保持 1.016 / 170。
  - `render/svg` 全绿；showcase 金图 `diff=0.0000%`（acc=2 生产不变）。
  - 全量 `./render` 的已知单跑红名单不变（acc=6 修复不触碰 acc=2 路径）。

## 2. 已修复：图像 ⑧ 上采样锯齿（CPU bilinear 被丢弃 → nearest）

- `res63.png` 63×61，SVG `<image width=126 height=122>` → 设备 ≈70×68（≈1.11× 放大）。
- 真因：`ImagePattern.ColorAt` 取样只做 `int(lx)` 整数取样（nearest）；且设备空间刷
  `applyDeviceSpaceBrush`（非恒等 CTM 必走，SVG 烘焙即如此）用
  `NewImagePatternFromSource` 重建 pattern 时**丢弃采样模式** → 恒回 nearest → 锯齿。
- 修复：① `ImagePattern` 增 `bilinear` 字段，`ColorAt` 的 clamp 分支用
  `intImage.SampleBilinear`（u=(p.x+lx)/imgW；按 (x+0.5) 网格，1:1 为恒等、放大才插值）；
  ② `DrawImageEx` 默认 bilinear 落地 `bilinear: Interp!=Nearest`；③ `applyDeviceSpaceBrush`
  重建时 `np.bilinear=pat.bilinear` 保留模式；④ SVG `<image>` 用默认 DrawImage（bilinear）。
- 实测（gpu 环境）：logo 区 vs resvg mean 14.3(nearest) → **7.74(bilinear)**，侧拼目视一致；
  bicubic 的 `SampleBicubic` 有振铃/伪绿，未采用（保持 bilinear）。
- 单测：raster / internal/image / svg 全绿；render 定向（Image/Pattern/Brush）除既有
  GPU-env（`FlushGPU: gg 回落 CPU`）外通过。**金图会移**（图像重采样改 bilinear），需重种。

## 3. 已修复：doStroke cpuOnly 缺口（窗口「虚线粗」真因，与精度批无关）

- 真因：`render/context_draw.go` 的 `doStroke` 设备空间 GPU 描边分支只判
  `!CPUOnlyMode()`（全局环境），**漏判 `c.cpuOnly`**（`WithCPUOnly` 的离屏 Context）。
  而 `doFill`(同文件 294 行) 在 4f 已修为 `CPUOnlyMode() || c.cpuOnly`。
- 后果：窗口进程 import 了 `render/gpu`（注册全局 SDF accelerator），SVG 1:1 烘焙
  （`doc.Render` = WithCPUOnly）的**描边**仍被 SDF/GPU 接单 → 粗线+合段；离线探针
  不 import render/gpu（无 accelerator）走 CPU 解析 → 细线正常，故离线「假绿」。
- 复现（决定性）：probe 加 `_ "github.com/energye/gpui/render/gpu"` 后，虚线圆
  由 5 段/5.92px 变为 **3 段/8.00px**；加 `&& !c.cpuOnly` 修复后回到 **5 段/5.94px**
  （与 resvg 参照 5 段/5.92px 一致）。侧拼图目视一致。
- 修改：`context_draw.go` doStroke 门加 `&& !c.cpuOnly`（与 doFill 对齐）。
- 影响：所有 WithCPUOnly 离屏描边（SVG 烘焙 / mask / filter / layer 子上下文）改走
  CPU 解析描边（正确方式）。**金图会位移**（此前窗口金图是 SDF 粗描边），需重种。
- 图片 `<image>` 不受影响（`tryGPUDrawImage` 经 `gpuCtxOps()` 已在 cpuOnly 下走 CPU）。

## 4. 已修复：⑱ Ahem 文字行黄色段右移（SVG 文本空白未按 xml:space 折叠）

- 现象：⑱ 上方 Ahem 文字行，灰色「Ahem body copy & tspan:」正常，但黄色「accent」tspan
  被右移约 75 设备 px（参照里它紧邻灰段左侧）。
- 真因：`flattenTextSpans` 直接用原始 `Text`（含 `:` 到 tspan 之间的源缩进 `\n    `）
  参与字宽推进；SVG `xml:space=default` 应把空白串折叠成**单个空格**。resvg 折叠后
  「accent」落在 23 字 + 1 空格处；我们保留整段缩进 → 多推若干字宽。
- 修复：`render/svg/text.go` 增 `normalizeSVGText`（空白串→单空格），三个 append 点
  归一化，并在整段文本首/尾各 trim 一个空格（内边界单空格保留）。
- 实测：黄色「accent」设备 x-range ours **440–499 = ref 440–499**（此前 ours 515–559）；
  svg 包全绿；**金图会移**（含缩进的多行文本布局统一校正），需重种。

## 5. 已修复（2026-11-09）：tilePatt 格边竖缝 = tile 离屏被 GPU 光栅化

- 现象：最下面 `<rect x=680 y=555 w=320 h=70 fill=url(#tilePatt)>` 两条绿带的**竖格边缝**：
  ours 每 ~26.7px 一条暗缝（tile 边界列），resvg 参照**无缝**。
  格边列差分：ours 绿带 [390,416,443,470,497,523,550]，ref **[]**。
- 已验证根因：scale=1（1024×640）渲染**无缝**；fractional（570×357，scale≈0.5576）**有缝**。
  「先烤 tile 位图 + 用户空间 Brush 按比例逐像素采样」在**非整数设备周期**
  （tileW×scale=48×0.5576≈26.76px）下，格边必有亚像素缝。实测 6 种 contained 变体
  （设备分辨率 / 真实浮点周期 / 内容按 tw/tileW 填满 / bilinear+wrap / 真 scale+整数周期 /
  真周期归一）**全部残留**；差分方向恒为「ours 该列更暗」，ref 无该列 → 是位图平铺缝，
  不是内容特征。
- resvg（tiny-skia）不是采样 brush，而是「**整数尺寸 tile 位图 + 整数偏移平铺**」→ 无缝
  （接受 ~0.9% 均匀缩放）。我们的用户空间 brush 架构做不到整数 1:1 平铺。
- 唯一修法（较大，独立任务）：pattern 填充从「tile 位图 + 用户 brush」改为
  「内容按**整数设备格**平铺渲染进设备尺寸离屏（整数偏移，无缝）→ 1:1 ImagePattern 填路径」。
  落点 `render/svg/gradient.go` + `render/svg/renderer.go` 填充/描边接线；影响所有 pattern + 金图。

> **勘误（2026-11-09）**：以上三条「必然留缝 / 架构做不到 / 唯一修法是重写填充」均为
> 离线观察被 GPU 路径污染后的误判；纯 CPU 进程 570/1024 两档都无缝。真因是 tile 离屏
> Context 未钉 CPU（见下），修法只需统一 CPUOnly，不需重写 pattern 填充机制。原文保留作历史。

- 状态：**已修复（2026-11-09）**。真因纠正：不是「位图平铺必然留缝」——纯 CPU 进程
  570/1024 两档都无缝；窗口进程里 tile 离屏 Context 没带 `render.WithCPUOnly`，
  tile 被 GPU 光栅化后画布左边缘 AA 覆盖 alpha=236/255，平铺采样到 tile 首列即每格
  一条暗缝。修法=SVG 内部离屏统一 `newSVGOffscreen`（WithCPUOnly）+
  `tryApplyFilterGraphGPU` 补 cpuOnly 门；验证=GPU 设备隔离红测
  `TestP38PatternTileSeamGPU`（红→绿）+ 真窗 `pixel_pattern_no_seam` + 快照列差分 0。
  详见 `pattern-tile-seam-task.md` §修复记录。

## 复现

- 真窗：`go run ./examples/ui_wr_p38_svg_showcase`（左 ours / 右 resvg 参照）。
- 定量：临时 harness 以 `svg.Document.Render(570,357)` 在 acc=2/acc=6 下烘焙（acc 经
  `software_construct` 的 `SetPixelAccuracy` 切换），裁圆区 (262,252)-(342,332)，
  统计全图 y=284 的蓝色 x-runs 对照 `testdata/showcase_ref.png`。
