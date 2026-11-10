# P3.8 G14 Marker 通用实现与验收（2026-11-10）

> 性质：用户拍板「补实现（通用能力）」+「全量（1.1 + SVG2 常用）」后的落地记录。
> 前置红测与设计见 `marker-design.md`。本批只做 SVG marker，不动 `render` 主包/GPU。

## 0 直话

**G14 marker 从「未实现」补到「通用实现」**：SVG 1.1 §11.6 全量
（marker-start/mid/end、markerUnits、refX/refY、markerWidth/Height、orient、
viewBox/pAR、overflow）+ SVG 2 `auto-start-reverse` / `context-fill` /
`context-stroke`，宿主 `path/line/polyline/polygon`；顶点与切线由引擎从
`render.Path` 的 verbs/coords 计算，marker 内容走同一 `renderElements` 管线。
单元 19 case 全绿；真窗 `ui_wr_p38_svg_structure` 加第 5 卡，双后端金图
**0.0000%**。

## 1 红 → 绿

| 阶段 | 结果 |
|---|---|
| 实现前 | `render/svg/marker_test.go::TestP38MarkerEndRenders` **FAIL**：line + marker-end 渲染后无红色 marker 像素（`render/svg` 只有 switch feature 字符串，无 Marker 解析/渲染） |
| 实现后 | 19 case 全绿（`TestP38MarkerEndRenders` + `TestP38MarkerSemantics` 表） |

**19 case 覆盖**（`render/svg/testdata/p38/marker/cases.json`）：
line 的 start/mid/end、polygon 闭合 mid、orient auto-start-reverse、markerUnits
strokeWidth/userSpaceOnUse、refX/refY、viewBox/pAR、overflow hidden/visible、
context-fill/context-stroke、缺失 ref fail-closed、marker 简写、多子路径、
无 stroke 的 marker、三次曲线端/中切线、marker 内 use、style 里的 marker-end。

## 2 实现落点

| 文件 | 内容 |
|---|---|
| `render/svg/document.go` | `MarkerElement`（ViewBox/Par/MarkerUnits/RefX/RefY/MarkerW/H/Orient/OverflowVisible/Children）；`Attrs.MarkerStart/Mid/End` + invalid 标记 |
| `render/svg/parser.go` | `parseMarkerElement`（SVG 1.1 默认：markerUnits=strokeWidth、markerWidth/Height=3、refX/refY=0）+ 四宿主 marker 属性/简写解析 |
| `render/svg/refs.go` | `walkElements` 穿 marker 子元素；`validateMarkers` 悬空/错类型 fail-closed + 具名 warning |
| `render/svg/inheritance.go` | `props.markerStart/Mid/End` + presentation/CSS 声明（`marker` 简写与长手级联） |
| `render/svg/marker.go`（新） | 子路径顶点/切线（line/curve/close，mid 角平分线）、markerUnits/ref/viewBox/overflow、context-fill/stroke、marker-in-marker 递归护栏 |
| `render/svg/renderer.go` | 四个宿主在 `fillAndStroke` 后 `renderMarkers`（marker 与 stroke 独立，无 stroke 也画）；`applyFill/StrokeAttrs` 支持 context-* |
| `render/svg/svg.go` | `renderState.root` 供 marker 内容从文档根继承 |

公开 API：`render/svg` +1 类型 `MarkerElement`；`render` 主包/GPU 零改动。
`RENDER_API_CATALOG.md` §0/§6.4 已同步，`go run ./scripts/apidoc` rc=0。

## 3 真窗（`ui_wr_p38_svg_structure`）

- 窗口从 4 卡扩到 **5 卡**：第 5 卡 `marker`（528×160，短卡避让 HUD；卡 1–4 保持
  528×252），展示 polyline 的 start（绿）/mid（琥珀）/end（蓝）+ markerUnits=
  strokeWidth 的红色方块；logic 探针新增 `marker_element`，pixel 探针新增
  `pixel_marker_start/mid/end/units`。
- 尺寸 1460×1000（原 1460×800；旧基线归档 `marker-golden/showcase_svg_structure_base_old.png`，
  sha `a870c68f…`；新基线 `showcase_svg_structure_base.png`，sha `5b25a528…`）。
- 验收（单窗、标准关闭、前后查显示）：

| 后端 | logic | pixel | golden | fps_interval | p95 |
|---|---|---|---|---|---|
| native | 7/7 | 24/24 | **0.0000%** / 1076640 px | 58.84 | 19.85ms |
| go/GL | 7/7 | 24/24 | **0.0000%** / 1076640 px | 57.96 | 19.40ms |

- 金图为**单基线**（marker 卡 CPU 烘焙、native/go 同像素；未用 per-backend 基线）。
- 第 5 卡短卡的原因：body 高度 = winH-top-HUD-gaps；300px 第三行会压到 HUD，
  HUD 的 fps 文字会进卡 5 金图区域导致逐次抖动（实测 0.009–0.01%）；短卡 200px
  整体落在 body 内，HUD 不再覆盖卡内容。

## 4 门禁

| 门禁 | 结果 |
|---|---|
| `go build ./...` / `-tags nogpu ./...` | ✅ 双 rc=0 |
| `go vet ./render/svg ./examples/ui_wr_p38_svg_structure` | ✅ rc=0 |
| `go test ./render/svg -count=1` | ✅ PASS（24.2s） |
| `go test -race ./render/svg -run '^TestP38Marker'` | ✅ PASS |
| `go run ./scripts/apidoc` | ✅ rc=0（`MarkerElement` 已登记 §6.4） |
| 真窗 native / go | ✅ 各 0.0000%、7/7、24/24、显示健康 |

## 5 边界（如实登记）

- marker 宿主只做 `path/line/polyline/polygon`（SVG 规范；rect/circle/ellipse 不硬造
  顶点）。
- marker 内容的继承从文档根 props 起算（`renderState.root`）；若 marker 定义在带
  fill 的 `<g>` 内，祖先 g 的展示属性未捕获（resvg 会继承；当前登记为边界，常见
  `<defs>` 直挂根的场景不受影响）。
- `context-fill/context-stroke` 传引用形状的 paint token；嵌套 marker 的 context
  链按 token 透传，极端嵌套未逐一验。
- resvg conformance 的 marker 子集**未重跑**（本批以 19 case + 真窗双后端为验收；
  若需 conformance 数字，另跑 resvg 对照）。
- 未改 `render` 主包/GPU/recording；现有无 marker 的 SVG 零像素变化（结构窗
  1–4 卡与 `ui_render_base_svg` 旧基线未变）。

## 6 状态

- **P3.8 G14 marker ✅ 补实现**（19 单测 + 5 卡真窗双后端 0.0000%）。
- P3/P4 能力清单里的 marker 缺口关闭；P3.4 全节点矩阵仍待补（下一条）。
