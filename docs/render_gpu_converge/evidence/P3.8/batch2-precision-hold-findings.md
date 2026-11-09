# P3.8 批②（精度批）挂起登记：acc=6 虚线圆单行内填 + 图像 2× 上采样差距

- 日期：2026-11-09
- 状态：**引擎修复已就位并自测绿；acc=6 全局启用按用户决定暂挂（生产回 acc=2 默认）**。
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
DenseShortEdges acc=6 avg 1.001 / max 51 / over8 116（acc=2 为 1.016 / 62 / 170，作护栏）。
`./render/internal/raster/`、`./render/svg/` 全绿。

## 1. 挂起问题 A：acc=6 虚线圆「单行内填」（真·acc=6 回归）

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

## 复现

- 真窗：`go run ./examples/ui_wr_p38_svg_showcase`（左 ours / 右 resvg 参照）。
- 定量：临时 harness 以 `svg.Document.Render(570,357)` 在 acc=2/acc=6 下烘焙（acc 经
  `software_construct` 的 `SetPixelAccuracy` 切换），裁圆区 (262,252)-(342,332)，
  统计全图 y=284 的蓝色 x-runs 对照 `testdata/showcase_ref.png`。
