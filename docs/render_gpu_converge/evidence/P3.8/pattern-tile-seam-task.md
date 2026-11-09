# 专项任务：SVG <pattern> 平铺格边竖缝（tilePatt 混合单位）

- 立项：2026-11-09（自 P3.8 批② 收口时拆出，独立专项）
- 状态：**已修复（2026-11-09，终态见文末「修复记录」）**；立项时「位图平铺必然留缝」的结论已勘误
- 归属：render/svg pattern 渲染（pattern 填充机制）

> **勘误（2026-11-09 修复时实测）**：立项时的根因判断（tile 位图左右边缘内容不连续 →
> 任何「烤位图再平铺」都必然现缝）**不成立**。同一 `doc.Render` 在纯 CPU 进程里
> 570×357 与 1024×640 都完全无缝；把这些「位图家族 7 变体」逐个复核，它们全都仍在
> **同一个未钉 CPU 的 tile 离屏 Context** 里光栅化，所以「全部残留」是实验混淆，不是
> 机制必然。真正根因见文末「修复记录」：窗口进程里 tile 被 GPU 光栅化，画布左边缘
> AA 覆盖只有 236/255（列 0 半透明），平铺采样到该列即每格一条暗缝。原文保留作历史。

## 现象
showcase 最下面 `<rect x=680 y=555 w=320 h=70 fill=url(#tilePatt) stroke=#9aa4b0>` 的
两条绿色横带里，**竖格边缝**：ours 每 ~27px 一条暗缝（tile 边界列），resvg 参照**无缝**。
格边列差分（设备坐标 570×357）：ours 绿带 [390,416,443,470,497,523,550]，ref **[]**。

## 已证根因（决定性）
- 该 pattern 是**病态混合单位**：`patternUnits=userSpaceOnUse`（tile 48×48 @x700,y430）
  + `patternContentUnits=objectBoundingBox`（四个 0.5×0.5 象限矩形，按引用元素 bbox 缩放）。
- scale=1（渲染 1024×640）**无缝**；fractional（570×357，scale≈0.5576）**有缝**。
- 位图家族实测 **7 种全留缝**：设备分辨率 / 真实浮点周期 26.72 / 内容按 tw/tileW 填满 /
  bilinear+wrap / 真 scale+整数周期 / 真周期归一 / **设备整数 1:1（tiny-skia 语义）**。
  差分方向恒为「ours 该列更暗，ref 无此列」。
- 结论：**tile 位图自身左右边缘内容不连续**（该 pattern 内容在 tile 边界本就不自洽）→
  任何「烤位图再平铺」都现 1px 缝。resvg 无缝 = 对填充区**逐像素连续求值内容**（无 tile
  位图边界），机制与我们「tile 位图 + brush」对立。

## 修复方向（专项要做的）
把 pattern 填充从「tile 位图 + 用户/设备空间 brush 采样」改为**连续内容求值/绘制**：
- 参考 resvg `render_pattern_pixmap`（精确语义需对照源码）+ tiny-skia pattern fill。
- 候选实现：对填充路径，按 pattern 周期把**矢量内容**在设备空间连续绘制/求值（不是先烤
  一张 tile 位图再采样）；或实现真正设备整数平铺且 tile 内容周期自洽的位图（需先把内容裁到
  单 tile 并验证周期连续）。

## 落点 / 影响 / 风险
- 文件：`render/svg/gradient.go`（`buildPatternBrush*`）、`render/svg/renderer.go`（填充/描边接线）。
- 影响：**所有 `<pattern>`**（4c-5 pattern 那批：oBB 内容 / x·y 相位 / tile 1:1 / 嵌套画刷）+ 金图。
- 风险：中高（填充机制级），需**金图重种**（共享状态，按纪律需用户点头）。

## 自验口径（硬，禁用 mean）
- 隔离红测：最小 pattern（userSpaceOnUse tile + oBB 内容）在 scale≈0.557 渲染，「格边列差分」断言
  **无周期竖缝**（当前红）；scale=1 对照无缝。
- 真窗人审：showcase 两绿带与参照并排肉眼一致。
- 回归：4c-5 pattern 钉测 + 全量 `render/svg` + 金图重种。

## 复现
- 真窗：`go run ./examples/ui_wr_p38_svg_showcase`（左 ours / 右 resvg）。
- 定量：渲染 `doc.Render(570,357)`，裁设备区 x≈378–557/y≈309–349，检测绿带内「比左右邻列暗>20」
  的列（周期性即格边缝）；对照 `testdata/showcase_ref.png`。

---

## 修复记录（2026-11-09，终态）

### 真因（决定性，GPU 设备实测）

- pattern tile 的离屏 Context 建在 `render/svg/gradient.go`：`render.NewContext(tw, th)`，
  **没有** `render.WithCPUOnly()`。`Document.Render` 的根 Context 是 CPUOnly，但子上下文
  不继承，于是：
  - 纯 CPU 进程（`render/svg` 普通单测）：无 GPU 加速器 → tile 走 CPU → 无缝（离线假绿）。
  - 窗口进程（`render/gpu` 已注册 + 设备可用）：tile 被 GPU 光栅化，画布左边缘 AA 覆盖
    实测 **alpha=236/255**（列 0），绿色带里该列 = 85% 绿 + 15% 暗底 → 一条暗缝。
  - 570×357（scale≈0.5566）时采样相位使缝只落在部分格首（设备 x=416/443/523/550）；
    1024×640（scale=1）时每 48px 一格全命中（x=700/748/…/988）。
- 与 pattern 内容、tile 分辨率、周期取整、bilinear/wrap 均无关；把 tile 位图换成哪种
  尺寸都留缝，因为缝来自「tile 位图本身列 0 的 alpha」。

### 修法

1. 新增 `render/svg/offscreen.go`：`newSVGOffscreen(w,h)` = `render.NewContext(w,h,
   render.WithCPUOnly())`。SVG 内部全部离屏（pattern tile / mask / filter / text /
   RenderWithColor）统一走它，保证「窗口内烘焙 == 离线烘焙」逐位一致。
2. `render/filter_ops_gpu.go` 的 `tryApplyFilterGraphGPU` 补
   `if CPUOnlyMode() || c.cpuOnly { return false }`：CPUOnly 离屏的滤镜不能再挂 GPU
   滤镜图（pixmap 未上传，GPU 结果回贴后滤镜等于没应用）。与 4f/批② doFill/doStroke
   的 cpuOnly 门同类。

### 验证（禁用 mean）

- 隔离红测 `render/svg/p38_pattern_seam_test.go::TestP38PatternTileSeamGPU`（需
  `WGPU_NATIVE_PATH`，无设备 skip）：
  - 修复前红：570×357 `[416 443 523]`、1024×640 `[700 748 … 988]`；
  - 修复后绿：两档零列。
- 真窗像素探针 `pixel_pattern_no_seam`（showcase 快照绿带逐列比左右邻列）：
  修复前快照有 4 条暗列，修复后 PASS（pixel 11/11）。
- 真窗快照格边列差分：570 绿带 ours `[]`、ref `[]`；两次窗口运行快照逐位一致。
- `./render/svg` 全量绿（含 WGPU 设备可用档）；`render` 滤镜定向回归绿。

### 影响面

- 所有 SVG 内部离屏统一 CPU：窗口内 SVG 烘焙结果与离线/conformance/resvg 对拍同源，
  不再随 GPU 会话漂移。滤镜/mask 边界细节随之改为离线基线（金图需重种）。
- 性能：SVG 只烘焙一次；窗口启动路径无逐帧成本。
- 公开 API 零变化（`newSVGOffscreen` 未导出），`RENDER_API_CATALOG.md` 无需改。
