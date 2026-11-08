# P3.8 批 4b 影响面评估（2026-11-08，零代码）

> 立项：用户拍板「确认批 4b 要补齐的，立项开做」（总账 §7 2026-11-08 立项行）。
> 范围：批 4 具名拒绝的 8 个滤镜原语 + textPath + shapes 6 枚坏数据 error。
> 目标：filters 组（单组口径 55.2%）→ 90%+，总 conformance 73.9% → ~90%。paint-servers/structure 归因单列批 4c。

## 1. 现状

- 求值器 `render/svg/filter.go`（451 行）：`supportedFilterPrimitive` 白名单 8 原语（feGaussianBlur/feOffset/feFlood/feColorMatrix/feBlend/feComposite/feMerge/feDropShadow），含白名单外原语的 filter 整体不应用（元素照原样画 + Warning，CSS Masking 口径）。
- conformance 基线：filters 组 397 用例单组 55.2%（84 id+135 close）；被拒原语覆盖约 161 用例（40%）。
- 批 4 拒绝是「没接线」为主、「没算法」为辅——见 §2 能力面盘点。

## 2. 能力面盘点（评估核心结论）

**接线型——底层能力已在 render 层（P3.4 家族交付），svg 层接线即可：**

| 原语 | 用例数 | 现成底层 | 接线点 |
|---|---|---|---|
| feMorphology | 14 | `Context.ApplyErode/ApplyDilate`（`filter_ops_cpu.go:341`）+ `internal/filter.MorphologyFilter` | fe* erode/dilate → 现成口 |
| feTile | 7 | `Context.ApplyTile(srcRect)`（`filter_ops_cpu.go:425`）+ `internal/filter.TileFilter` | feTile → 现成口 |
| feDisplacementMap | 1 | `Context.ApplyDisplacementMap(xSel,ySel,scale,disp)`（`lighting.go:193`） | xChannelSelector/yChannelSelector → xSel/ySel 映射 |
| feComponentTransfer | 22 | `internal/filter.NewTableColorFilter(r,g,b,a)`（256 项 LUT）+ `Context.ApplyColorFilter` | SVG 五型（identity/table/discrete/linear/gamma）全编译成 256 LUT |
| feImage | 27 | svg 层 image 渲染能力（批 2 已交付 data: URI/外部解析） | 引用图渲染进离屏作滤镜输入，无需新 Context 口 |

**扩展型——底层有近亲，需扩参数/变体：**

| 原语 | 用例数 | 现状 | 要补 |
|---|---|---|---|
| 光照家族 | 46 | `Context.ApplyLighting` 仅 Point-Diffuse（`filter_ops_cpu.go:407`；`ImageFilterLighting` 节点 LightPos/LightColor/LightKD/SurfaceScale） | 光源三型（Distant/Point/Spot）× 模式两态（Diffuse kd / Specular ks+exponent）；`ImageFilterNode` 扩 LightType/LightDirection/LightPointsAt/LightSpecular 等字段；GPU shader 变体 + CPU 腿扩展（resvg `lighting.rs` 490 行含法线图生成参照） |
| feComposite arithmetic | 3 | `blendModeFor` 已映射 over/in/out/xor；arithmetic 显式拒绝（`filter.go:87`） | 逐像素 `k1·i1·i2 + k2·i1 + k3·i2 + k4`：render 层新公开口（`ApplyCompositeArithmetic` 族），GPU 小 shader + CPU 直译 |

**新写型——算法不存在：**

| 原语 | 用例数 | 参照 | 要补 |
|---|---|---|---|
| feTurbulence | 19 | resvg `turbulence.rs`（305 行：Perlin 格点 + fractalNoise/turbulence 两模式 + stitchTiles）；Skia 上游 `SkPerlinNoiseShader`（本地 flutter 树为桩目录，无源码，按上游口径对齐） | 新 CPU 逐像素算法（conformance 离屏对拍确定性优先），GPU 化后置另批 |
| feConvolveMatrix | 25 | resvg `convolve_matrix.rs`（112 行：order/kernelMatrix/edgeMode/preserveAlpha/divisor） | 新通用卷积（现有 `internal/filter/blur.go` 仅高斯 1D） |

**text 与解析：**

| 项 | 用例数 | 参照 | 要补 |
|---|---|---|---|
| textPath | 44 | resvg `usvg/text/layout.rs resolve_clusters_positions_path`（按 cluster 沿弧长排布）；K5 已补 GlyphEntry.Cluster（批 6 穿链完成）——沿路径排布所需数据已就位 | svg 层 TextElement 路径子元素解析 + render/text 沿路径布局能力；开工先归因 graded-pass-但零实现的容差假绿嫌疑（路径 stroke 画对+文字缺失落 48×48 网格容差内） |
| shapes 坏数据 | 6（error） | spec + resvg：奇数尾坐标丢弃、<2 点不画该元素、文档其余照常 | `svg.Parse` 现 fail-fast 拒整文档（实测 `points="20 40 160"` → `svg: <polyline> points: odd number...` error）→ 改容错降级 + Warning |

## 3. 分批方案

| 批 | 内容 | 性质 | 预期 filters 组增量 |
|---|---|---|---|
| 4b-1 | feMorphology + feTile + feDisplacementMap + feComponentTransfer + feImage | 纯接线（5 原语底层全现成） | +~71 用例权重中大部分 |
| 4b-2 | 光照家族（三光源×两模式）+ feComposite arithmetic | 扩展（GPU 节点字段 + shader 变体 + CPU 腿） | +~49 |
| 4b-3 | feTurbulence + feConvolveMatrix | 新算法（CPU 逐像素直译参照） | +~44 |
| 4b-4 | textPath + shapes 容错 | 布局 + 解析鲁棒性 | text 已 99.4%（假绿归因后重定基线）；shapes error→pass 6 |

## 4. 验证载体

- 单测：红测先行（resvg 期望值对拍，`testdata` 入语料样本）；每原语至少 1 枚负向变异。
- conformance：`scripts/svgconformance -group filters` 逐批复跑，按批回填分组数字。
- 真窗：扩展 `ui_wr_p38_svg_filter`（新原语演示卡走控制动词，组件示例边界纪律）；双后端金图。
- 门禁：apidoc/catalog（新公开口如 ApplyCompositeArithmetic 须同步 §3 目录）、-race 触及面、svg 包逐文件回归。

## 5. 分层与纪律检查

- 新算法全部落 render 层（`internal/filter` 实现 + `Context` 公开口），svg 层只做 fe* 语义解析→现成/新公开口映射，与批 4 模式一致；禁在 svg 层写像素算法。
- render 公开口新增（composite arithmetic、光照扩展字段）→ 同步 RENDER_API_CATALOG + apidoc rc=0。
- feTurbulence/convolve 先 CPU（确定性对拍），GPU 化挂后续性能批（与 P4.3 同线评估），首帧/帧率门禁不因新增离屏算法回退。
- Skia 对齐口径：fe*→image filter 映射（Skia `modules/svg/src/filters/` 上游结构），算法公式以 spec + resvg 双参照（Skia 本地无源码，如实标注）。

## 6. 风险

1. 光照家族是最大头（46 用例、490 行参照），法线图生成 + Spot 光源几何最复杂——放 4b-2 独立批，失败不拖累接线批。
2. feTurbulence stitchTiles 缝合逻辑有边角（resvg 305 行里占约 1/3）——先不做 stitchTiles=stitch 的对拍容差声明，或按参照直译后逐用例对拍。
3. textPath 若归因确认假绿，text 组 99.4% 基线会重定（如实登记，不掩盖）。
4. conformance 全量跑有测试间干扰（37.8% vs 单组 55.2%）——验收口径统一用单组复跑。

## 7. 批 4b-3 交付记录（2026-11-08）

- §2 新写型两行全部落地：feTurbulence/feConvolveMatrix 按 resvg 参照逐字直译（含 stitchTiles 按参照直译后逐用例对拍——风险 2 的后者路线，语料 1 枚 stitch 用例 close 8.37）；§5 分层纪律守住（internal/filter 两新文件 + Context 公开口，svg 层仅解析映射）。
- **计划外发现并修复 conformance 门禁环境洞**：脚本未 import `_ render/filters`，4b-1 起滤镜实现在 conformance 进程从未注册（空转口径 66.0%→82.1% 全部失真）；修复后真实口径 filters 单组 82.6%、全量 86.0%。风险 4 的「干扰」疑云部分源于此。
- 新公开口 7 符号入 catalog §3.20，apidoc rc=0；钉测 17 枚 + 负向；金图三窗 0.0000%。
- 剩余 far 全为 4b-1/4b-2 原语在真实生效口径下暴露的既有洞（morphology 半径/光照系），登记不掩盖，归批 4c 归因。
