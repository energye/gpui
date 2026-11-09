# 专项任务：SVG <pattern> 平铺格边竖缝（tilePatt 混合单位）

- 立项：2026-11-09（自 P3.8 批② 收口时拆出，独立专项）
- 状态：**立项 / 未修复**（本会话多次 contained 尝试均失败并回退，代码已恢复到已提交态）
- 归属：render/svg pattern 渲染（pattern 填充机制）

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
