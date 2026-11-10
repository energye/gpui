# P3.8 G14 Marker 通用实现设计（2026-11-10）

> 性质：**补实现设计 + 红测证据**。用户拍板「补实现（通用能力）」后，先走
> 动手修门槛：真实问题结果已由红测坐实；本文给标准/架构/影响面，停在停下报告。

## 0 真实问题结果（红）

- `render/svg/testdata/p38/marker/line_end.svg`：`<line marker-end="url(#m)">` +
  `<marker><circle fill=red/></marker>`，渲染 120×120。
- `render/svg/marker_test.go::TestP38MarkerEndRenders` 当前 **FAIL**：
  「marker-end not rendered: no red marker pixels at the line end」。
- 代码事实：`render/svg` 只有 `switch.go:53` 的 feature 字符串；无 `MarkerElement`、
  无 `marker-start/mid/end` 属性解析、无渲染。P3.8 D2 声明「marker/switch 做」，
  这是能力缺口，不只是测试缺口。

## 1 标准与范围（通用，不做单案例特判）

- **标准**：SVG 1.1 §11.6 Markers（marker 元素、marker-start/mid/end、markerUnits、
  refX/refY、markerWidth/Height、orient、viewBox/preserveAspectRatio、overflow）；
  + SVG 2 `orient="auto-start-reverse"` 与 `context-fill`/`context-stroke`（resvg
  已支持，P3.8 的对照 oracle 是 resvg）。
- **适用元素**：`path`/`line`/`polyline`/`polygon`（规范定义的 marker 宿主）；
  `rect`/`circle`/`ellipse` 不适用（不硬造顶点）。
- **全量语义**：
  - start = 子路径首顶点；end = 末顶点（闭合子路径 start=end 同点）；mid = 除首末
    外每个顶点；多子路径逐段处理。
  - orient：auto / auto-start-reverse / 角度；auto 用顶点切线；mid 用入/出方向
    角平分线（180° 退化取 outgoing）；start 反向仅 auto-start-reverse。
  - markerUnits：strokeWidth（默认，乘有效 stroke-width）/ userSpaceOnUse。
  - refX/refY 在 marker 用户坐标（有 viewBox 时为 viewBox 坐标）；默认 0。
  - markerWidth/Height 默认 3；有 viewBox 时按 preserveAspectRatio 映射到 viewport；
    overflow 默认按 usvg（视口裁剪），`overflow="visible"` 放开。
  - marker 内容可含任意元素/use/symbol/filter/clip/mask；递归用
    `maxPropChainDepth` 同类护栏 fail-closed。
  - context-fill/context-stroke：marker 内使用时取引用形状的 fill/stroke
    （resvg 语义）；无引用上下文时按 none/默认。
- **命名边界**：marker 内容里的 CSS `marker` 简写若解析器不支持则登记；先支持
  属性形式 marker-start/mid/end + marker 简写。

## 2 架构落点（复用现有 SVG 管线）

| 层 | 改动 |
|---|---|
| `document.go` | 新增 `MarkerElement`（Attrs/ViewBox/Par/HasViewBox/MarkerUnits/RefX/RefY/MarkerW/H/Orient/OverflowVisible/Children）；`Attrs` 加 `MarkerStart/MarkerMid/MarkerEnd` 与 invalid 标记 |
| `parser.go` | 解析 `<marker>`（defs/任意位置、id 收集）；解析四个宿主元素的 marker 属性 + marker 简写；解析期解析 ref（含悬空/错类型 fail-closed，与 mask/clip 同口径） |
| `refs.go`/`Document` | marker id → `*MarkerElement` 查表；递归 guard |
| `renderer.go` | 四个 renderShape 在 `fillAndStroke` 之后调 `renderMarkers(dc, elem, path, a, p, st)`；按子路径顶点/切线渲染；marker 内容走 `renderElements`（同一引擎，无旁路） |
| path 顶点/切线 | 用 `render.Path` 的 verbs/coords 取原始命令端点；C/Q/A/H/V/Close 端点切线；mid 角平分线；零长段跳过；不 flatten 成折线（避免 mid 在细分点重复） |
| 变换 | `translate(vertex)·rotate(orient)·scale(units)·viewBoxTransform·translate(-refX,-refY)`，在宿主元素 transform 内渲染（marker 随之变换） |
| 裁剪 | marker viewport (0,0,markerW,markerH) 裁剪，overflow visible 放开 |

## 3 测试与真窗（通用覆盖，不新开同能力窗）

- **单测**：`render/svg/marker_test.go` 表驱动（testdata/p38/marker 语料）：
  - 宿主：path / line / polyline / polygon（含闭合/多子路径/曲线）；
  - start/mid/end 三个位置、orient auto / auto-start-reverse / 角度；
  - markerUnits strokeWidth / userSpaceOnUse；refX/refY；viewBox + pAR；overflow；
  - context-fill/context-stroke；悬空/错类型 ref fail-closed；marker 简写；
  - 红线：当前 line_end.svg 红；实现后逐项绿。
- **真窗**：在既有 `ui_wr_p38_svg_structure` 加 **1 张 marker 卡**（同 SVG 结构能力，
  不开第二扇窗）：path/line/polyline/polygon + start/mid/end + markerUnits/orient/
  viewBox；pixel 探针 + 金图重种 native+go（旧图归档）。
- **conformance**：跑 resvg marker 子集（现有 73.9% 基线里 marker partial 应改善），
  记录到 P3.8 证据；不作为独立门禁。
- **负向**：摘 marker 渲染 → 红线复红；错误 orient/markerUnits → 像素红；还原绿。

## 4 影响面

- 只动 `render/svg`：parser/document/renderer/refs/attrs；**不动 `render` 主包/GPU**。
- 无 marker 的现有 SVG：解析树多一个 inert 类型，但渲染分支不触发 → **零像素变化**；
  存量 `ui_render_base_svg` / p38 窗金图应 0.0000%（不加卡前）。
- 公开 API：`render/svg` 新增 `MarkerElement` 类型（catalog §6.4/apidoc 同步）；
  `render` 主包无变化。
- 真窗 `ui_wr_p38_svg_structure` 加卡 → 该窗金图重种（需批准）；其它窗不重种。
- 性能：marker 只在有 marker 属性的元素上多一次顶点遍历；静态 SVG 影响可忽略。

## 5 边界（如实登记，不假装覆盖）

- `context-fill`/`context-stroke` 的 CSS context-* 继承细节按 resvg 对照补；若
  oracle 不覆盖则登记为 SVG 2 扩展。
- marker 内容中的 CSS `marker` 简写若现有 CSS 解析器不支持，先支持属性形式，
  简写登记为解析器后续项（不影响 marker 元素本身）。
- marker 在 rect/circle/ellipse 上不实现（规范不定义）。

## 6 停点

- 当前只加了红测 + 语料；**生产代码未动**。
- 请拍板：按上述全量（含 auto-start-reverse + context-fill/stroke）实现，还是先做
  SVG 1.1 核心、context 系列登记出范围？确认后按「先红后绿」落地 + structure 加卡 +
  金图重种 + 双后端/conformance。
