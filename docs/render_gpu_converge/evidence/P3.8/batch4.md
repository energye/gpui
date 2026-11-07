# P3.8 批 4 交付：SVG 滤镜（fe 子集 8 原语）+ 离屏求值器

- 日期：2026-11-07（交付即收口：真窗双后端金图逐位）
- 依据：assessment.md D1-D7 + design.md 批 4 细化

## 1 交付物

- filter.go: FilterElement/FilterPrimitive + 求值辅助 (saturateMatrix/hueRotateMatrix/luminanceToAlphaMatrix 20 值 5 列矩阵) + renderFilterElement 离屏求值器
- parser.go: filter 元素解析 + fe 原语通用解析 + filter 外 fe* 具名拒绝
- refs.go: filter 引用校验 (悬空/非 FilterElement/含不支持原语 -> filterInvalid=原样绘制)
- renderer.go: 形状元素 filter 属性接线 + shapeRenderPath

公开面: svg 顶层 43->45 (FilterElement/FilterPrimitive)

## 2 四只实现洞（全部探针/对照实验抓回）

1. colorMatrixValues saturate/hueRotate 从 fp.Numbers 读但解析存 fp.Numbers2 -> filter 永不应用
2. saturateMatrix 公式参数反 (sr=1-s 传入 0.787*sr => s=0 给恒等阵非灰阵)
3. [20]f32 矩阵 4 行 x 4 值缺偏移列 (应 4x5=20 每行 RGBA 系数+偏移)
4. feTurbulence warning Element 期望 (fp.Kind 非 rect tag)

## 3 验证账

- 能力单测: render/svg 全包 PASS (七枚新测 + 批 1/2/3 回归); -race 绿
- 负向 4 条全红->还原绿 (batch4-negatives.txt)
- apidoc rc=0 (catalog 45/43/39/29/15)
- conformance 双后端 PASS
- 真窗 ui_wr_p38_svg_filter 四卡: native seed->verify + gles verify 同一基线 0.0000% (917440px) logic 6/6 pixel 10/10
- 存量: 批 3 窗金图 0.0000% 零变化

## 4 已知边界

- filter 内 transform=translate(0,70) 未生效 (shapeRenderPath/elementBBox 不含 transform, 登记)
- feComposite arithmetic / feTile / feImage / feTurbulence / feDisplacementMap / feComponentTransfer / feConvolveMatrix / feMorphology / lighting 全系: 具名拒绝
- text 元素的 filter: 需布局后处理, 登记后续批
- BackgroundImage: 需页面底图快照, 不在此批范围
