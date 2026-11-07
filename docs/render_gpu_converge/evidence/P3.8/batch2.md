# P3.8 批 2 交付：SVG paint 完整化（渐变/clipPath/mask/image/pattern）+ GPU 径向 focal 洞修复

- 日期：2026-11-07（交付即收口：真窗双后端金图逐位）
- 依据：`assessment.md`（D1–D7）+ `design.md` §批 2 细化（底座盘点结论：render 层零增量——**本批实际抓出并修复一处 GPU 底座洞**，见 §3）

## 1 交付物（render/svg 五文件新增/改造）

| 能力 | 实现 | 语义要点 |
|---|---|---|
| 2a 渐变 | `gradient.go`（LinearGradientElement/RadialGradientElement/GradientStopElement + build*Brush）+ parser 分支 + transform.go 纯函数化（`parseTransformList`，M=M1·…·Mn 右乘链，与旧 Context 链逐位等价） | stops/gradientUnits 两档（oBB=Translate(bbox)∘Scale(bbox 尺寸)∘gradientTransform）/spreadMethod→`SetExtend` 直映（底座三态 CPU+GPU 已备）/fx,fy→`SetFocus`/href **属性级 first-set-wins 继承**（环安全）；identity 映射走原生刷（GPU 渐变 ramp 路），非恒等映射走 CustomBrush 逆映射（精确，非共形变换下端点映射不成立） |
| 2b clipPath | `clip.go`（ClipPathElement + clipRegionFor + shapePath/transformPath）| 子形状并集 `Path.Op(PathOpUnion)` 保曲线；**单形状保 winding**（双 subpath 反向挖洞成立）；**clip-rule 归位**（单形状按 clip-rule 定 fill rule，批 1 的「借用 fill-rule」作废）；clipPathUnits 两档（oBB 相对被裁元素 bbox）；悬空引用 fail-closed（元素不画+Warning）；`transformPath` 非 conformal 仿射先 `ExpandConicsToQuads`（P3.5 批 1 边界展开） |
| 2c mask | renderer.go `renderMasked`（三离屏用户空间组合）| mask 内容离屏渲→`NewLuminanceMask`→被裁内容离屏渲→`ApplyMask`（DestinationIn）→DrawImage 回主 dc；**组合全在用户空间**→外层 CTM 天然正确（Skia saveLayer/Flutter 同形）；maskUnits 区域（默认 bbox −10%~120%）；maskContentUnits=objectBoundingBox 具名拒绝（罕见形，登记）；悬空引用不画+Warning |
| 2d image | ImageElement + parse 期 data URI 解码 + renderImageElement | **data: URI only**（base64；跨文件引用具名拒绝，D2）；pAR 复用 `viewportTransform`（视口=x/y/w/h、viewBox=图像固有尺寸，slice ClipRect 裁剪）；width/height 缺省=固有尺寸（SVG2）；坏 payload 具名 Warning 不画 |
| 2e pattern | PatternElement + buildPatternBrush | tile 离屏渲→`CustomBrush` 平铺采样（mod 周期）；patternTransform 经逆映射精确；patternUnits/patternContentUnits（默认 userSpaceOnUse——**零值陷阱已修**：oBB 是枚举 0，缺省必须显式设）；href 属性级继承同渐变 |

公开面：svg 顶层 29→**39**（go doc 实测；catalog §6.4 补三行逐名 + §0 行同步；apidoc rc=0）。

## 2 批 2 真洞：GPU 径向 focal 渐变反相（render/internal/gpu/shaders，**非 svg 层**）

- **现象**：真窗卡 2 径向 focal（fx,fy 偏左上）渲染成「中心黑、外围白」——与 CPU ColorAt 完全反相。
- **定位**（对拍链）：底座 CPU `RadialGradientBrush.ColorAt` 值探针 ✓ 正确（hotspot 白/远端黑）→ 离屏 GPU（import render/gpu 的 render.Context）**复现反相** → GPU 径向走 session textured cover（路标确认 mode=3）→ 仓内两份渐变 shader：stencil fallback 的 `linear_ramp_mask.go` 内嵌 WGSL focal 尾部**有** `pointDist/intersectDist` 换算 ✓，session cover 的 **`shaders/cover_textured_linear.wgsl` focal 尾部直接 `return t_ray` ✗**——t_ray 是「以 |FP| 为单位」的射线参数，缺倒数换算 ⇒ 每个 |FP|>1 像素 ramp 反相。
- **修复**：cover_textured_linear.wgsl focal 尾部补 `point_dist/intersect_dist` 换算（与 computeTFocal、stencil fallback 逐句对齐），注释写明洞史。
- **验证**：GPU 离屏对拍曲线 9 点与 CPU ColorAt 全部一致（sRGB 值差 ≤1）；永久钉 `render/p38_radial_focal_gpu_test.go`（session-cover 链路 10 探针 GPU==CPU，|diff|≤3）；负向 N-9 摘换算→钉红→还原绿（`batch2-negatives.txt`）。
- **波及面**：该洞影响所有「GPU 路 + focal 径向渐变」的渲染（不限于 svg）。simple 径向（mode 1）与线性（mode 0）不受影响（对拍曲线确认）。

## 3 窗口/探针迭代中如实记录的三类自误（不掩盖）

1. **挖洞样本内圈同向**：非零绕组挖洞要求内外圈反向——单测样本与窗口样本先后都写过同向版（单测样本当场修，窗口样本 seed2 后修）。
2. **线性域期望手算连续错**：渐变底座为线性域插值；stop 色 #e24a4a 的 r_lin=0.760（非 1.0）、t=0.133 处蓝分量 srgb 已达 0.42——三轮手算错被快照对照抓回（最终以「离屏=快照」双链一致 + 精确线性域值钉死）。
3. **洞/黑段露出的是卡底色不是透明**：探 alpha==0 的断言形态错，改探色 == 卡底 #171a20。

## 4 验证账（门禁全绿）

- **能力单测**：`render/svg` 全包 PASS（新增 p38_paint/p38_clip/p38_mask/p38_image/p38_pattern 五组 21 枚 + 批 1 全量回归）；`-race`（svg+internal/gpu）绿。
- **负向对照 9 条**全红→还原绿（`batch2-negatives.txt`）：摘 spreadMethod / 摘 href 继承 / 摘 clip 并集 / 摘 clip-rule 归位 / 摘 oBB 映射 / 摘 luminance / 摘 tile 周期 / 摘 patternTransform 逆映射 / **摘 GPU focal 换算**。
- **apidoc** rc=0（catalog §6.4 三行逐名 + §0 39/29/15 口径链）。
- **conformance 双后端** PASS。
- **真窗** `examples/ui_wr_p38_svg_paint`（六卡：线性三 spread / 径向 focal / 渐变填充+描边+transform / clip 并集+挖洞环 / mask luminance / pattern+image）：**native seed→verify + gles verify 同一基线三轮 0.0000%**（917440px）、logic 9/9、pixel 18/18；gpu_ops>0、cpu_fallback=0。
- **存量回归**：`render/svg` 全包 ✓、`render/internal/gpu` 全包 ✓（WGSL 改动后）、批 1 窗 `ui_wr_p38_svg_doc` 金图 **0.0000% 逐位零变化**、`-race` 绿。
- **抓屏门**：同批 1 口径不设（P3.5/P3.7 先例 + H18 在册）。

## 5 已知边界（如实）

- 渐变 stop 中段的 GPU ramp 插值域与 CPU 线性域存在中段色差（端点/位置语义已钉）——属 P4.2「GPU/CPU 模糊一致」既有范围，登记不扩批。
- clipPath 多子形状并集为 nonzero 域（evenodd 挖洞走单形状 winding 路径）；maskContentUnits=objectBoundingBox 具名拒绝；image 仅 data: URI。
- mask 每元素三离屏（区域尺寸）——svg.Render 一次性出图可接受；逐帧热路径消费者接线时再做区域缓存（登记）。
- pattern tile 每次求值重建离屏（同上，按 (pattern,bbox) 缓存留批 3+）。
