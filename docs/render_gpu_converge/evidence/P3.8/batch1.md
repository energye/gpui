# P3.8 批 1 交付：SVG 文档语义地基（pAR/单位/继承/引用/defs-use）

- 日期：2026-11-07（交付即收口：真窗双后端金图逐位）
- 依据：`assessment.md`（D1–D7 拍板）+ `design.md` §1
- 语义锚：Skia modules/svg（SkSVGDOM）行为；样本一律 `render/svg/testdata/p38/`

## 1 交付物

**render/svg 六文件改 + 四新文件**（公开面 15→29 顶层符号，go doc 实测）：

| 文件 | 内容 |
|---|---|
| `length.go` 新 | `Length/LengthUnit/LengthAxis/RefSpace` + `Resolve`（px/pt/pc/in/cm/mm/q@96dpi、em/ex×继承 font-size、%×视口轴/归一化对角 sqrt((w²+h²)/2)） |
| `preserve.go` 新 | `PreserveAspectRatio/ParAlign` 九值 + `parsePreserveAspectRatio` + `viewportTransform`（T(tx,ty)∘S(s)∘T(−min)单矩阵，meet=min/slice=max，纯函数值级可钉） |
| `inheritance.go` 新 | `props` 继承求值器（CSS 级联：表现属性<标签规则<类规则<内联 style）+ `<style>` 最小选择器集解析 + currentColor + dasharray 奇数补偶 + font-size 相对解析 |
| `refs.go` 新 | 引用校验：#id 索引（defs 内含）、use 解析分类（RefKind）、**静态环检测/深度≤64**（回放期有界，parse 期拒绝+Warning）、url() 悬空/未支持具名拒绝（fail-closed 不画非黑） |
| `document.go` 改 | 几何字段全量切 `Length`、Attrs 原始字符串化（缺省=继承）、新元素 `Defs/Symbol/Use/SVG/Style`、`Document.Warnings/Styles/ElementByID/Par/OverflowVisible` |
| `parser.go` 改 | 新元素分支、pc.warn（Reason 稳定码+Detail）、根 pAR/overflow/完整 attrs、style 文本捕获 |
| `renderer.go` 改 | props 驱动渲染、use/symbol/嵌套 svg 视口分支、dash/miterlimit 接 `Context.SetDash/SetDashOffset/SetMiterLimit`、视口裁剪 |
| `svg.go` 改 | `renderInternal` 重写：`viewportTransform` + 根视口裁剪（overflow:hidden 默认）+ 无 viewBox=px 不映射（SVG 规范，旧默认 16 缩放废除） |
| `doc.go` 改 | 支持面/不做面整段重写（transform 六函数补记、静态子集定位、具名拒绝口径） |
| `svg_test.go` 改 | 存量适配（Length 字段、`.5` 原文、% 合法化重写、defs 入树重写）+ 新增 `p38_doc_test.go` |

**样本** `render/svg/testdata/p38/` 九枚（手写入库）：par_slice_overflow / units_absolute / units_relative / inherit_tree / style_element / use_symbol / use_cycle / ref_dangling / nested_viewport / basic_square。

## 2 语义修复（行为变化，均已 A/B）

1. **preserveAspectRatio**：独立拉伸（反规范）→ 等比九值 meet/slice（`svg.go:84-86` 旧双轴缩放废除）。
2. **根视口裁剪**：新增（slice 出界必须裁；`RenderTo` 大画布探针钉住）。
3. **url() 悬空**：落黑块 → 不画 + `unresolved-paint` Warning（fail-closed）。
4. **子元素 % 长度**：报错 → 合法（按引用轴解析）。
5. **无 viewBox 文档**：默认 16×16 缩放 → 用户单位=px 不映射（规范口径）。
6. **继承**：仅 fill/stroke 两属性 → 全继承表 + style/class/currentColor。

**存量画面零变化实证**：`ui_render_base_svg` 金图 **diff=0.0000% over 627200 px**（方形 viewBox meet 恒等预判证实；`window-native-seed`/`base-svg-ab.txt`）。

## 3 窗口抓出并修复的回归（1 条）

批 1 重写时 `applyStrokeAttrs` 丢失 `overrideColor` 分支 ⇒ `RenderWithColor` 不再换描边色——**存量真窗 `ui_render_base_svg` 的 logic 探针 `override_keeps_none` 咬住**（金图未盖到此路：D 卡只用填充覆盖）。修复（描边 override 分支恢复，`renderer.go` applyStrokeAttrs 带 st）+ 新包级钉 `TestP38OverrideRecolorsStroke`（描边换色+none 中心仍透明）。修复后本窗双后端金图 0.0000% 未重播种 = 修复零画面副作用。

## 4 验证账（门禁全绿）

- **能力单测**：`render/svg` 全包 PASS（rc=0；p38 红测 14 枚 + 存量适配；先编译红 `red-compile.txt` vet_rc=1 → 后绿）；`-race -count=2` 绿（race-svg.txt rc=0）。
- **负向对照** 5 条变异→红→还原绿（`negatives.txt`）：摘 pAR 换算 / 摘根视口裁剪（**首轮变异无效**——画布=视口时裁剪不承重，补强 `RenderTo` 大画布探针后红，如实记）/ 摘继承求值 / 摘 use 展开（先补像素探针再变异）/ 摘 Warning 收集。
- **apidoc** rc=0（catalog §6.4 svg 节重写 15→29 + §0 行同步；顺带补登存量漏 `ShaderEffect.ComposedQuadWGSLFor`，注明归属）。
- **conformance 双后端** PASS（`conformance.txt` rc=0）。
- **真窗** `examples/ui_wr_p38_svg_doc`（六卡：pAR meet 九宫格 / slice 九宫格 / 单位条 / 继承树 / use+悬空拒绝 / 嵌套视口；1460×800，body 3 列恰 1164px，bake 1:1=344×252）：
  - logic 8/8（pAR 九值序列、slice 旗标、单位换算 5 值、原始属性+零 Warning、use kind、嵌套 pAR、fail-closed Warning）
  - pixel 24/24（同一坐标常量；含悬空区域=卡底色非黑非内容）
  - 金图：native 播种 → **native verify 0.0000% / gles verify 0.0000%（同一基线，双后端逐位一致）** over 917440 px；gpu_ops=34039>0、cpu_fallback_ops=0
  - `window-native-seed3/verify3.txt`、`window-gles-verify2.txt`
- **全仓构建** `go build ./...` rc=0（后台完成 exit 0）。
- **抓屏门**：本窗不设（P3.5/P3.7 线窗先例 + H18 在册：RunFor 到点窗已撤、抓屏与快照两来源）；`ui_render_base_svg` 的 gate 抓屏子项同族空抓（黑图），与渲染内容无关——金图/逻辑/像素三证据全绿承重，如实记不掩盖。

## 5 已知边界（如实）

- % 于 stroke-dasharray/stroke-width 按用户单位处理（求值期无视口），em/ex 正确——已在 `applyDecl` 注释声明，批 2 涉 paint 时如需再提。
- rect rx/ry 沿用旧 max() 单半径圆角（椭圆角属 paint 批口径）。
- `<style>` 只支持标签/单类选择器（逗号组支持，后代/属性/伪类具名跳过+Warning）。
- 根 width/height % 仍按 viewBox 折算（根级旧约定，未在批 1 范围）。
