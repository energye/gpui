# P3.5 路径与几何 影响面评估（2026-11-01）

> 任务：P3.5 路径与几何五项——conic 路径段、四角独立圆角、Matrix4 透视、路径效果（1D/2D/Sum/Compose）、布尔运算保留曲线。依赖 P3.0 ✅（依赖图 `evidence/P3.0/dependency-graph.md`：P3.5 不被 P3.1–P3.4 依赖、不阻塞任何人，但 Matrix4 是 P3.7 前置）。
> 对齐标准：Skia `SkPath` conic / `SkRRect` / `SkM44` / `SkPathEffect` / `SkOpBuilder`（catalog §13 P3.5 规划行原文）。
> 调查：三路只读深查（conic+verb 全链 / RRect+Matrix4 / PathEffect+布尔）+ 本机抽查核实，HEAD `6d861410`。

## 1. 关键现状

### 1.1 conic（PLAN §5.1「verb 全集无 Conic」）

- verb 枚举 5 值在 `render/path.go:18-29`（`PathVerb byte`，MoveTo..Close）；每 verb 坐标数 `verbCoordCount`（`render/path_verb.go:35-48`，QuadTo=4/CubicTo=6）；Path 为 SOA 存储 `verbs[]+coords[]`（path_verb.go:54-71，栈缓冲 `verbsBuf[32]`/`coordsBuf[96]`）；`HasCurves` 只认 Quad/Cubic（path_verb.go:125-132）。
- **消费面（新增 verb 的机械同步清单）**：render 主包 14 处 verb switch（path_build:113 / context_path:212 / software_fill:27,135,213 / software_stroke:203 / dashPath→software_stroke.go:201-240 / path_metrics:127 / path_boolean:148 / pathops_subpath:79,130,201 / pathops_winding:24 / pathops_area:23 / pathops_bbox:30,115）+ gpu 包 10 处（path_convert:50 / vello_accelerator:478 / sparse_strips_filler:83 / tessellate:197,709 / stroke_hairline:39,74,92 / transform_uniform:123,165 / gpu_types:147）+ **4 套平行独立 verb 枚举**（`render/scene/path.go:19-33`、`render/internal/stroke/expander.go:171-200`、`render/internal/raster/edge_builder.go:20-34`、text outline 独立 op 枚举）。风险：未知 verb 被 `verbCoordCount` default 0 **静默跳过**，漏改一处即静默错画。
- CPU 栅格：曲线是原生边（`QuadraticEdge`/`CubicEdge` 前向差分，`internal/raster/curve_edge.go:315/:553`）+ Y 极值 chop（`path_geometry.go:58/:128`）；flatten 模式容差 0.1px（edge_builder.go:676-681）。
- **GPU：曲线从不到达 GPU**，三管线全部 CPU flatten——Vello compute 0.25（`tilecompute/flatten.go:25`，path_convert.go:67-79 有 quad 升 cubic 先例）；stencil tess 0.1（`tessellate.go:26`）；scene/SparseStrips 0.1（`internal/gpu/flatten.go:24`）；stroke 展开 flatten 0.25（`stroke/expander.go:245`）。scene tag 无 conic 位（`scene/tag.go:59-65`）。
- Arc 现状：入口级 cubic 逼近（`path_build.go:179-222`，≤90° 每段）；Circle/Ellipse 4×cubic k=0.5522847（path_build.go:147-173）；DrawArc/DrawEllipticalArc（`context_path.go:388-483`）；SVG A → arcToCubics（`path_svg.go:406-530`）。
- **SDF 形状检测依赖精确 verb 模式**（circle=MoveTo+4×CubicTo+Close，`shape_detect.go:83-130`；rrect :289-295）——存量构造改 conic 会破坏检测快路。
- 全仓 conic 路径段零实现（grep 仅 sweep 渐变命名与 ftgrays 文本光栅里「conic=二次曲线」命名）；catalog §13 已锁 `Path.ConicTo` 规划。

### 1.2 四角独立圆角（PLAN §5.1「ClipRoundRect 单半径」）

- 公开面全为单半径或 rx/ry（全四角共用）：`Canvas.ClipRoundRect`（canvas.go:124）、`DrawRoundedRectangle`（canvas.go:131）、`Context.DrawRoundedRectangleXY`（context_path.go:311-354，cubic 拼角）、`Path.RoundedRectangle`（path_build.go:226-238）、`PathBuilder.RoundRect`（path_builder.go:70-82）；recording 镜像（recorder_path_draw.go:197/:396、command.go:282 单 radius 字段）；scene 层 `RoundedRectShape`（scene/shape.go:76-87）、`RoundRectShape{RadiusX,RadiusY}`（shape.go:195-228，ToPath/Contains 取 min）、`TagFillRoundRect` 编码 rx/ry（encoding.go:622-629）。
- GPU 裁剪：`ClipParams` 32B = vec4 rect + radius + enabled + pad2（`internal/gpu/clip_params.go:23-64`）**单半径单槽**；**13 个 wgsl 各复制一份 `rrect_clip_coverage`**（sdf_render/cover/cover_aa/textured_quad/textured_quad_bicubic/glyph_mask/glyph_color/glyph_mask_lcd/msdf_text/video_yuv/convex/cover_textured_linear/cover_textured_pattern，实测 `grep -l ClipParams shaders/*.wgsl` = 13）；`rc.clipRRect` 单指针（gpu_render_context.go:310），嵌套多 RRect 只留最内层（context_clip.go:319-325）。pad 仅 8B，四角化必须扩 uniform 布局。
- GPU 绘制：SDF rrect 单 corner_radius（sdf_render.wgsl:25-28 `param3`；CPU 版 `sdf.go:84-95`）；形状检测 detectRRect 反推单半径（shape_detect.go:290-370）。
- CPU 裁剪：`RRectClip{Rect,Radius}` 单半径（`internal/clip/stack.go:22-25`）；rrectSDF/rrectCoverage（stack.go:245-277）。CPU path clip=掩码栅格化（`internal/clip/mask.go:49-75`）+ GPU 深度掩码（`depth_clip.go:558`）——**非矩形裁剪的通用底座现成**。
- ui 消费（全 uniform 单半径）：decor.go:195-219、clip_rrect.go:19-97、ui/scene textured.go:1823、composite.go:152。
- 结论：全仓无任何四角独立半径的 API/结构体/字段；**Skia 对齐口径**：SkRRect 对 simple（单半径）用解析快路，complex（四角各异）落 path——本仓 path 裁剪/填充底座现成，per-corner 走 path 化与 Skia 行为一致。

### 1.3 Matrix4 透视（PLAN §5.1「Matrix 2x3 仿射」）

- `render.Matrix{A..F}` 纯 2x3（matrix.go:25-28），方法集全仿射（Multiply/Invert/ScaleFactor 等 matrix.go:73-240）；**仓库级无任何 4×4 类型**（仅 shader/字节序列化层有 mat4x4）。
- transform 栈：`c.matrix`+`deviceMatrix`（context_transform.go:83-179），totalMatrix 2x3；Push/Pop 保存 matrix+clip 深度（context_layer_pass.go:37-75）。
- GPU 矩阵只有两种形态：①路径全链 CPU 烘设备空间顶点（context_draw.go:235/:404，gpu_render_context.go:1954-1957 注释自述）+可选仿射两行 uniform（stencil_fill.wgsl:10-33）；②text/glyph `mat4x4 = ortho × 2D 仿射`（render_session.go:3940-3954/:5386-5409）。**w 恒 1，全仓无透视除法**（grep perspective 仅 2 条注释）。
- CPU 栅格：`raster.Transform` 接口仅 `TransformPoint` 2D（edge_builder.go:51-54），调用几乎全传 Identity（路径已烘设备空间）；无 1/w。
- 现有伪 3D：`DepthSprite` 画家算法（depth_r5.go:40-170）；引擎层 `Projector`（engine/camera/project.go:22-129，**ProjectQuad 注释明言「feeds render.DrawImageQuad corner for corner」**）；`Context.DrawImageQuad` 四角贴图（m4_quadimage.go:23-88）——GPU 双三角 **UV 线性插值无透视校正**（textured_quad.wgsl:82-96，vertex w=1），CPU `triSampler` 重心仿射（m4_trisampler.go:12-44）；`DrawVertices` Gouraud 无纹理（vertices_mesh.go:20-71）。
- clip 全链是设备空间 2D 快照（scissor 整数 context_clip.go:297-301、RRect uniform、深度 path clip :372-395），无矩阵无 w；文本 GPU 走仿射批变换（glyph_mask_engine.go:752-766），透视类变换现状=整串轮廓 CPU 烘（text_gpu.go:247-258）。
- 结论：透视唯一正确底座=①4×4 类型+数学 ②带真实 w 的顶点链（GPU varyings 透视校正插值是硬件保证，顶点出 clip-space 真 w 即自动正确）+ CPU 侧 1/w 有理插值。图片 quad 是最短正确路，且正好补齐 `Projector→DrawImageQuad` 的透视缺口（现仅逐三角仿射）。

### 1.4 路径效果（PLAN §5.1「dash 完整 + 两个 1D 子集已有，缺抽象与 2D/Sum/Compose」）

- dash 完整且是「Stroke 字段 + Context 口」形态：`Stroke.Dash`（stroke.go:32/:75/:90/:105）、`Paint.EffectiveDash`（paint.go:322/:330）、`Context.SetDash/SetDashOffset/ClearDash`（context_style.go:327/:353、context_path.go:88-92；Canvas 接口 canvas.go:94-96）；实现 `ApplyDash`（dash.go:194-196 → software_stroke.go:176-243 迭代式，闭合路径补回起点、pattern 跨子路径延续）；4 消费点：CPU stroke（software_stroke.go:38-47）、用户空间扩边（stroke_user_expand.go:53-63）、GPU stroke 带 LRU 缓存（`path_geometry_cache.go:446-480`，键=pathHash+dashHash+scaleBits）、录制回放（recorder_style.go:186-188）。
- 1D 子集两个已是 Path 方法：`WithCorners`（pathops_subpath.go:391-488，Flatten(0.35) 后 quadratic 割角）、`Discrete`（:502-570）；消费者仅测试+pathshape 示例（零生产）。
- 无 PathEffect 抽象（grep 仅注释）；无 Sum/Compose 组合器。
- 2D 所需度量工具已备：`Path.Length/Trim`（pathops_subpath.go:191/:299）、`PathMetric.PositionAt/TangentAt`（path_metrics.go:42/:49）、曲线级 Tangent/Normal/Subsegment（curve_cubicbez.go:196/:204/:66）；stroke join/cap（expander.go:133-150/:407-596）。
- GPU 侧：path effect = path→path 的 CPU 变换，GPU 走展开后几何（GPU stroke ApplyDash+缓存即先例），无新 GPU pass。

### 1.5 布尔保留曲线（PLAN §5.1「Clipper 版已有但曲线 flatten」）

- `BooleanPath`（path_boolean.go:54-95）+ `Path.Op` 包装（:98-100）+ 四常量（:22-27）；Clipper 第三方 `github.com/ctessum/go.clipper v0.1.2`（go.mod:16）；**flatten 在 `pathToClipperPaths`（:159-178，容差 0.1 :40）**；整数网格 1024（:35）；**输出必折线**（`clipperPathsToPath` :206-219 仅 MoveTo/LineTo/Close）。
- 生产消费者零（唯一示例 `ui_render_base_pathbool` + 测试）——「已验证待接线」正常中间态（AGENTS 口径）。
- 可复用曲线工具：QuadBez/CubicBez `Subsegment/Subdivide/Eval`（curve_quadbez.go:61/:46、curve_cubicbez.go:66/:51）。**无 Bezier-Bezier 通用求交**（全仓确认；唯一直曲求交是轴对齐裁剪边 `internal/clip/edge_clipper.go:331/:532`，且 clip 包外零生产消费者）。
- clip 链不走布尔（path 裁剪=掩码栅格化）；曲线级求交与拓扑是「真曲线布尔」的核心缺口。

## 2. 能力定义草案（逐项）

| 项 | Skia 对应 | 语义 | 推荐落法 |
|---|---|---|---|
| conic | `SkPath::conicTo(x1,y1,x2,y2,w)` | 带权二次有理曲线；w=1 退化为 quad；圆弧 w=√2/2 精确表示；仿射变换后仍为 conic（同权） | 真 verb（ConicTo，coords +5 含 weight）+核心层原生（存储/Iterate/Flatten/HasCurves/Length/紧致 bounds）+各消费边界统一降 quads/flatten helper；raster 用 conic→quads 走现成 QuadEdge；GPU 三管线 conic→quads 复用 quad 升 cubic 链；**存量 Arc/Circle/Ellipse cubic 构造不动**（金图零变化、SDF 检测模式不破） |
| 四角 RRect | `SkRRect::setRectRadii`（四角各 (rx,ry)）+ 类型归约 | nine-patch 类型（zero/simple/simple-Squircle/oval/complex） | RRect 类型+四角 Radii+类型归约；新 Draw/Clip 口（per-corner）；**uniform 快路（SDF/ClipParams）不动，complex 落 path**（Skia 同款分流）；存量单半径口与 scene 编码零变化 |
| Matrix4 | `SkM44` / SkMatrix persp 行 | 4×4 数学 + 透视投影 | 第一版：类型+数学（乘/逆/投影/ProjectQuad，对齐 engine Projector 语义）+ `DrawImageQuad` 透视校正（GPU 顶点出 clip-space 真 w→硬件透视插值自动正确；CPU triSampler 1/w 有理插值）；**Canvas 级 4×4 栈与路径/文本透视挂 P3.7 拍板** |
| PathEffect | `SkPathEffect`（filterPath：1D dash/corner/discrete/line2d、2D path-dash/stamp、Sum、Compose） | path→path 变换 | `PathEffect` 接口 + 五构造器（Dash/Corner/Discrete/Path1D stamp/Path2D）+ `Sum/Compose`；`Stroke.PathEffect` 挂点（stroke 链先于 Expand 应用，同 dash 现位）；存量 `Stroke.Dash` 四消费点保留为便捷口（内部复用 ApplyDash），不删不改签名 |
| 布尔保曲线 | `SkOpBuilder`（输出保留曲线段） | 交/并/差/异或后曲线段不塌成折线 | 混合法：Clipper 定交（现状链路）+**曲线段回贴**——输入曲线段带来源 id，交点参数回映原曲线 `Subsegment` 重建曲线段，直边/交角保留 Clipper 输出；真曲线布尔引擎（Bezier-Bezier 求交+拓扑自研）远期另立 |

## 3. 实现批次（拍板 D1）

推荐**本步全做、五批独立提交**（PLAN §5.1 明列五项全为 P3.5 完成标准；每批先红后绿值级单测 → 实现 → 公开口/recording 同步 → 能力钉 → 真窗卡）：

1. **批 1 conic**：存储+核心层+边界降 quads helper+漏点哨兵防护+raster conic→quads+GPU 三管线+`Path.ConicTo` 公开口（recording/svg 往返随口）
2. **批 2 四角 RRect**：RRect 类型+归约+Draw/Clip 新口（complex 走 path）+ClipRRect 分流（uniform→现 SDF 快路 / complex→path clip）
3. **批 3 Matrix4**：类型+数学+DrawImageQuad 透视校正（GPU/CPU）+投影工具口
4. **批 4 PathEffect**：接口+五构造器+Sum/Compose+Stroke.PathEffect 挂点+GPU 展开链缓存
5. **批 5 布尔保曲线**：混合法回贴+段 id 携带+端点匹配

顺序理由：conic 是路径核心表示（RRect 圆角与布尔圆弧回贴可后续消费它）；批 2/3/4 互相独立；布尔最难且可选消费 conic，压轴。

## 4. 拍板项

| # | 问题 | 推荐 |
|---|---|---|
| D1 | 范围与拆批 | 本步全做，五批独立提交 |
| D2 | conic 落法 | **真 verb 全链**（存储原生+边界降 quads helper+未知 verb 哨兵防护）；存量 Arc/Circle/Ellipse 构造不动保金图；备选：`ConicTo` API 存在但存储即降 quads（机械面小一个量级，但非 Skia 对齐、精度损失进存储层） |
| D3 | 四角 RRect GPU 面 | **complex 落 path 裁剪**（Skia 同款分流，13 shader 副本不动）；备选：ClipParams 32→48B 扩 radii vec4×13 shader（解析 AA 快路，机械面大）——若真窗 AA/性能不达标再升 |
| D4 | Matrix4 接入面 | **A：类型+数学+DrawImageQuad 透视校正（GPU 真 w+CPU 1/w）+投影工具口**——交付 P3.7 消费的「Matrix4+透视绘制口」且不动 2x3 transform 栈；备选 B：本步做 Canvas 级 4×4 栈全量（路径/文本/裁剪全吃 w；text 6 提交点+clip 全链+scene 编码，量级数倍） |
| D5 | PathEffect 形态与挂点 | 接口+五构造器+Sum/Compose；挂 `Stroke.PathEffect`（stroke 链先于 Expand，同 dash 现位）；fill+effect（stroke-and-fill）本步不做并显式登记；存量 Stroke.Dash 不删不改 |
| D6 | 布尔保曲线方案 | **混合法（Clipper 定交+曲线回贴）**：输出曲线段=原曲线精确子段；备选：真曲线布尔引擎（自研 Bezier-Bezier 求交+拓扑，量大，远期） |

## 5. 验证方案草案

- **能力单测（先红后绿）**：
  - conic：存储往返（ConicTo→Iterate 还原 5 coords）、w=1 ≡ QuadTo 等价钉、w=√2/2 圆弧与解析圆误差 < 容差钉、flatten 误差界、raster fill 面积对账、GPU 三管线 == CPU 像素、未知 verb 哨兵（防护测试）
  - RRect：四角独立值级、类型归约等价钉（全同→simple 等价输出）、ClipRRect 分流探针（uniform 走快路/complex 走 path）、coverage 像素钉（角内/角外/AA 边带）
  - Matrix4：数学钉（乘/逆/投影往返）、quad 透视 CPU==GPU 像素钉、透视 vs 仿射差异可见性钉（梯形对边采样断言）
  - PathEffect：Sum/Compose 语义钉（作用顺序）、stamp 间距/法向钉、存量 dash 测试全绿回归（收编不破坏）
  - 布尔：回贴段与原曲线 Subsegment 采样等价钉、|A∪B|=|A|+|B|−|A∩B| 面积对账、输出曲线段保留断言（对比全 flatten 版折线段数）
- **真窗**：新增 `examples/ui_wr_p35_pathgeom` 五卡（conic 弧精确性放大对比 / 四角 RRect 各异半径+嵌套 clip / Matrix4 quad 透视贴图 / PathEffect 三族 / 布尔保曲线 vs 折线放大对比），隔离 :3，双后端（native 必跑 + go 档）+金图+自检 JSON
- **金图**：新窗新金图，容差按 P0.1 默认 0；**存量金图零变化**为批 1 验收项（Arc/Circle/Ellipse 构造不动的直接证据）
- **负向对照（每批一，红→还原）**：conic 摘 raster 分支→conic 钉红；RRect 摘角分支→角 coverage 钉红；Matrix4 摘 1/w→透视钉红；PathEffect 摘 Sum 顺序→Compose 钉红；布尔摘回贴→保曲线钉红
- **回归**：逐文件单测（render/*、internal/gpu/*、internal/clip/*、internal/raster/*）+ conformance 双后端 + `-count=2 -race` + 五目标构建 + `go run ./scripts/apidoc` rc=0

## 6. 风险

- **verb 机械面大**（render 14+gpu 10 处 switch、4 套平行枚举）：漏点=静默跳过（verbCoordCount default 0）。防护=批 1 加未知 verb 哨兵（核心遍历点显式报错），全仓 verb switch 清单落 evidence 逐点核对。
- **ClipParams/13 shader**：D3 推荐落 path 后本步不动 shader；若升 B 方案需 13 副本同步清单。
- **透视双三角**：GPU 顶点出真实 w 后 varyings 透视校正插值是 WebGPU 规范保证（平面 quad 两三角结果跨对角线一致）；CPU triSampler 需 1/w 有理插值对齐——CPU==GPU 像素钉验证。
- **布尔回贴端点匹配**：Clipper 1/1024 量化与原曲线端点容差匹配（段 id 携带优先、坐标容差兜底）；近共线交点聚簇为已知难例，测试数据入 testdata。
- **性能**：布尔回贴/PathEffect 2D 均为 CPU path 变换，GPU 走展开后几何（缓存先例）；Matrix4 透视仅顶点格式与 uniform 微调；不新增每帧 GPU pass，帧时预算内。

## 7. 工作区与非本线事项

- 未跟踪 7 处均非本线，**只报告不动**：`.atomcode/artifacts/`、`.qoder/`、`.zcode/`、`examples/ui_wr_c5_anim_over_static/golden/c5_final.png`、`ui_wr_p32_shader`、`ui_wr_p33_richtext`、`ui_wr_p34_filters`（后三个为编译产物二进制）。
- 分支 `feat/p2-收敛整合`，HEAD `6d861410`；P3.4 ✅ 遗留项（summary §4）不阻塞 P3.5。
