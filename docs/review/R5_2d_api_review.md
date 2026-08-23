# R5 · render/ 根包 2D 绘图核心审查报告

审查对象：`render/` 根包 2D 核心（path*.go、stroke.go、dash.go、curve.go、solver.go、shapes.go、paint.go、brush*.go、gradient*.go、pattern.go、blendmode.go、color.go、matrix.go、vec.go、clip_op.go、mask.go、nine_patch.go、sdf*.go、shape_detect.go，含 internal/stroke、internal/clip 抽查）。
方法：基于约 20 万字符的前期深入调查记录，抽其中 8 条最关键 bug 线索逐一回源码核实（全部属实），其余采信记录。已核实的线索在文中标【已核实】。

---

## 一句直话结论

**核心光栅化引擎（AAA 分析式填充、tile 管线、描边展开、dash、图层混合）已经达到可用的工业级水准，UI 引擎内部用没问题；但有四个硬伤——布尔运算是像素级假货、CPU 渐变不随坐标系变换且逐像素做 pow、路径 clip 用 even-odd 配对还每次全画布栅格化、一批状态同步 bug（Reset/Append 边界、SetMiterLimit、damage 不算描边宽度）——所以现在还不能挂"独立通用工业级 2D 图形库"的牌子，修完 TOP5 才够格。**

---

## 二、算法正确性

### P0/P1 级问题

**A1.【P1·已核实】`Path.Reset()` 不清边界缓存，复用路径的包围盒只增不减**
`render/path.go:178-185`。注释写着 "identical to Clear"，但 `Clear()`（path.go:174-175）会置 `boundsValid = false`，`Reset()` 只清 verbs/coords/start/current，**不清 boundsValid**。而 `expandBounds` 只扩不缩，于是任何走 `Reset()` 复用的路径（典型：software.go 的 `strokeResultToPath` 用 `dst.Reset()` 反复重建描边结果），其 `Bounds()` 会永远带着历史最大范围。`Context.Fill/Stroke` 里 `trackDamage(c.path.Bounds())`（context.go:1162、1175）直接消费这个包围盒，脏区越滚越大，最终退化成整屏重绘。

**A2.【P1·已核实】`Path.Append()` 不合并被拼接路径的边界**
`render/path.go:189-196`。只 append 了 verbs/coords 并同步 current/start，**没有对 other 的坐标调 expandBounds，也没合并 other.boundsValid**。p 原来为空时，拼完一条非空路径 `Bounds()` 返回空矩形；p 非空时新几何超出部分丢失。下游 damage 跟踪/裁剪全部跟着错。

**A3.【P0】布尔运算是"逐像素猜"出来的，不是真布尔**
`render/path_boolean.go:62-100`【已核实 62-77 行】。实现是在包围盒内按像素中心逐点调 `Winding()` 判内外，输出整数网格上的矩形条。三个硬伤：
1. **静默截断**：区域超过 2048×2048 直接钳到 2048（62-68 行），大坐标下结果是错的，且无任何告警；
2. **性能灾难**：每像素两次 `Winding()`，每次都整条路径遍历＋递归展平曲线，2048² 上限就是最多 400 万次全路径求值；
3. **精度锁死在 1 像素**：曲线边界变锯齿楼梯，丢掉矢量精度；且只用非零规则，EvenOdd 路径结果错误。
对照 Skia 是 `SkOpBuilder`（精确代数布尔）。这是全套 API 里离"工业级"最远的一块。

**A4.【P1·已核实】路径 clip 的栅格化是 even-odd 配对，忽略绕向**
`render/internal/clip/mask.go:199-224`（AA 版 `rasterizeScanlineAA`，交点排序后 `i += 2` 两两配对）。非 AA 版同样是配对式。后果：**自相交或重叠子路径的裁剪路径（比如两个相交圆想取并集当 clip）会被裁成对称差**，与 Skia（clip 按非零绕向处理）行为不符。这是语义性错误，不是质量问题。

**A5.【P1·已核实】Stroke/Fill 的脏区不含描边外扩**
`render/context.go:1162、1175`。damage 直接用 `c.path.Bounds()`（几何包围盒），但描边会向外扩 width/2＋端帽＋斜接尖角，宽线条场景脏区盖不住实际画到的区域，增量 present 下会留残影。

**A6.【P1·已核实】焦点径向渐变完全没用 StartRadius**
`render/gradient_radial.go:130-190`。`computeTFocal` 解的是"从焦点出发的射线与 EndRadius 圆求交"，然后 `gradientT = pointDist / intersectDist`——注释写 "accounting for start radius"（189 行），**代码里 StartRadius 根本没出现**（86 行只在 ColorAt 里判了 radiusDiff==0）。焦点偏移＋StartRadius>0 的锥形渐变结果错误；对照 SVG/Skia 的 two-point conical 公式缺一半。另外 discriminant<0 时返回 1 属于凑数。

**A7.【P1】CPU 渐变不受 CTM 影响**
渐变 Brush 的 `ColorAt(x,y)` 在软件管线下收到的是设备空间像素坐标，而渐变几何是用户空间定义的，中间没有乘 CTM 的逆矩阵。用户旋转/缩放画布后，渐变轴不跟着动——Skia 的 shader 是活在本地空间、随 CTM 变换的。语义级偏差。

**A8.【P2·已核实】`SetMiterLimit` 在设置过 `SetStroke` 后被静默忽略**
`render/context.go:963-965` 只写 `c.paint.MiterLimit`；`Paint.EffectiveMiterLimit()`（paint.go:275-280）只要 `Stroke != nil` 就读 `Stroke.MiterLimit`。Width/Cap/Join 当年修过这个双轨不同步问题，MiterLimit 漏了，属于同类 bug 的漏网之鱼。

### P2/P3 级问题

- **A9.【P2】`Path.Arc` 中途调用产生畸变**【已核实 328-334、367 行】：非空路径时直接 `CubicTo`，隐式以当前点为三次曲线起点但控制点按弧起点计算，当前点≠弧起点时首段扭曲；Skia arcTo 会先连线到弧起点。
- **A10.【P2】展平精度度量混乱**：winding 展平把 `cubicFlatness` 返回的**平方距离**直接和线性容差 0.1 比（等效容差≈0.316px）；Flatten 版又用 `toleranceSq*16` 的经验系数打补丁。同一指标三处三种比法，深边界处可能误判。
- **A11.【P2】dash 相位每子路径重置**【已核实 software.go:1183-1187】：每个 MoveTo 都把 pattern 状态重置回 offset。SVG/Skia 语义是单条路径跨子路径连续走相位的，多子路径虚线的花纹会和标准渲染不一致。
- **A12.【P2】0 宽度 stroke 没有 hairline 通道**【已核实 software.go:1044-1045】：`effectiveWidth < 1.0` 一律钳到 1px，Skia 的 hairline（亚像素细线）语义缺失；细线在小字号/高 DPI 缩放下全部变粗。
- **A13.【P3】solver 数值边角**：SolveCubic disc==0 分支 `Sqrt(-d0)` 在数值扰动使 d0>0 时出 NaN（有 isFinite 兜底，影响有限）；二次方程解法本身（sc0/sc1 稳定公式）质量不错。
- **A14.【P3】`Matrix.Invert` 奇异时静默返回单位阵**（阈值 |det|<1e-10），调用方无法感知失败；Skia 返回 bool。`ScaleFactor()` 注释称"最大奇异值"实为列范数近似，与正确的 `MaxScaleFactor` 并存易误用。
- **A15.【P3】SDF 加速器椭圆用缩放圆距离近似**，离心率高时 AA 宽度失真（已知取舍，注释有说明）。

**核实为正确、无需修的**：斜接极限公式 `2*hypot < (hypot+dot)*miterLimitSq`（internal/stroke/expander.go）前期调查曾怀疑写反，后经推导证实对应 `sec(φ/2)` 比值，与 kurbo/SVG 一致，**不是 bug**。SVG 弧解析、Extrema 导数系数、Subsegment 控制点、quad/cubic 面积公式（kurbo 系）、方头/圆头帽几何均核对无误。

---

## 三、与 Skia 对齐度

| 项目 | 本库现状 | Skia 标准 | 差距评级 |
|---|---|---|---|
| 路径布尔 | 像素采样近似（path_boolean.go） | SkOpBuilder 精确布尔 | **P0 级差距** |
| 路径 clip 栅格化 | even-odd 配对（internal/clip/mask.go:220） | 非零绕向 | **P1** |
| 渐变插值色彩空间 | 默认线性光（gradient.go:98-133） | 默认 sRGB（CSS/SVG 同） | P2，效果更"物理"但不合默认规范 |
| 渐变空间 | 设备空间采样，不随 CTM | 本地空间随 CTM | **P1** |
| hairline | 钳到 1px | 专用 hairline 管线 | P2 |
| dash 相位 | 每子路径重置 | 跨子路径连续（SVG 语义） | P2 |
| 圆角矩形 | 两套系数并存：builder k=0.5523 vs Path.RoundedRectangle α≈0.5486【已核实 path_builder.go:56-59 注释自认】 | 单一 addRRect | P3 冗余 |
| 斜接极限/端帽/连接 | 与 kurbo/SVG 公式一致 | ✔ | 对齐 |
| AAA 分析式填充、tile 管线、边缘哨兵、隐式闭合 | 移植质量高 | ✔ | 对齐，是本库最强项 |

---

## 四、作为通用 2D 渲染库的能力缺口

对标 Skia/Cairo 的公共能力清单，缺的主要是：

1. **真·矢量布尔运算**（Martinez/scanline boolean 或移植 skia pathops）——现实现不可对外承诺。
2. **路径效果管线**：Trim/Corners/Discrete 是散落在 Path 上的方法，不能像 SkPathEffect 那样组合、链式、进 GPU 录制。
3. **Brush 本地矩阵**：所有渐变/图案 brush 无 transform 字段（见 A7）。
4. **图像滤镜作为绘制属性**（paint/saveLayer 级 SkImageFilter），目前只有全画布后处理式 ApplyBlur。
5. **颜色过滤器（SkColorFilter）per-paint、色彩管理**（无 wide gamut、无 F32 目标、全程假定 sRGB）。
6. **文本沿路径（drawTextOnPath/RSXform）、路径插值 makeLerp、圆锥曲线段（conic）**。
7. **Contains/Winding 只有非零规则**，EvenOdd 填充存在但命中测试不支持。

---

## 五、性能

1. **【P1】渐变逐像素 `math.Pow`**【已核实 gradient.go:98-133】：每次采样两个颜色各做一次 sRGB→linear（每通道 pow），软件填充热路径上每像素 6 次 pow ＋接口虚调用。Skia 用预计算 LUT。这是 CPU 渐变填充最大的性能洞。
2. **【P1】布尔运算 O(面积×路径复杂度)**（见 A3），实际不可用于生产尺寸。
3. **【P2】路径 clip 每次 push 全画布 R8 栅格化**（MaskClipper），Difference 还要再扫一遍全画布求反；大画布多层裁剪内存与带宽抖动大。
4. **【P2】热路径分配**：AAA blit 每 trap 行 `make([]uint8)`×2、Fill 每次 visited map、SparseStrips 每次 fill 转 scene.Path 新分配、`EffectiveDash()/GetStroke()` 每次 Clone——都是稳态帧率下的 GC 压力点，应改缓冲复用。
5. 【P3】`convertVerbsToStroke` 注释说 cast 实际在做拷贝；`Path.Transform` 一律新建路径无法原地。

---

## 六、可读性与冗余

1. **两套圆角矩形几何**（path_builder.go kappa vs path.go Arc α 公式），注释自己承认不委托认为避免改变几何——应统一到一套并回归视觉基线。
2. **几何类型四处重复**：render.Point/Vec2 与 internal/stroke 自带一份、scene/surface/clip 又各有 Rect/Bounds；`pathBounds`（software.go）与 `Path.BoundingBox` 逻辑重复且不看曲线极值。
3. **矩阵 API 双轨**：`ScaleFactor` vs `MaxScaleFactor`、`IsTranslation` vs `IsTranslationOnly` 同义并存。
4. **Paint 三态复杂度**：LineWidth/LineCap/LineJoin/MiterLimit 旧字段与 Stroke 结构双轨，已引发 A8 这类不同步 bug，建议收敛。
5. 展平器五处各写一份（ops/stroke/dash/raster/winding），容差与度量各不相同（见 A10）。
6. 小问题：`Clear()` 注释说"释放底层存储"实则保留容量；sdf.go 注释 0.7 实值 0.75。

---

## 七、这套 2D API 能否当独立工业级 2D 图形库用？

**直话：内部当 UI 引擎的绘图层——能；对外当独立通用 2D 库（对标 Skia/Cairo 给第三方嵌）——现在不能，差一口气。**

能的一面：分析式 AAA 光栅化、tile 管线、描边展开（kurbo 系公式全对齐）、dash、九宫格、混合模式、SVG 路径解析、形状识别加速，这些核心件的算法底子扎实、测试覆盖厚（golden/NaN/压力测试都在）。

不能的一面，就卡在四件事上：①布尔运算是假的（A3）；②裁剪语义错＋贵（A4＋性能3）；③渐变空间/色彩语义与行业标准默认不一致（A6/A7）；④状态管理有一串低级但真实的 bug（A1/A2/A5/A8）。这四类问题任何一个第三方用户踩到都会判定"库不可信"。TOP5 修完、补上真布尔或明确砍掉该 API，才谈得上独立工业级。

---

## 八、评分表（各 10 分）

| 维度 | 得分 | 说明 |
|---|---|---|
| 正确性 | 6.5 | 核心填充/描边/变换扎实；扣分：布尔假货、clip 绕向错、渐变 focal/CTM 错、Reset/Append/SetMiterLimit/damage 五个状态 bug |
| 性能 | 6 | tile/AAA 管线好；扣分：渐变逐像素 pow、布尔 O(面积)、clip 全画布栅格化、热路径分配多 |
| 资源 | 7 | pixmap 池、LRU/sharded 缓存、容量复用意识到位；扣分：clip mask 全画布分配、每 fill 多处临时分配 |
| 可读性 | 6 | 注释密、出处清楚；扣分：双轨 API 多（圆角/矩阵/Paint 三态）、几何类型四处重复、展平器五份 |

总分 25.5/40。

## 九、TOP5 必修清单

1. **修 Path 状态机三连**：`Reset()` 补 `boundsValid=false`、`Append()` 合并边界（path.go:180/189）、`SetMiterLimit` 同步 Stroke（context.go:963）——半天工作量，收益立竿见影。
2. **Stroke/Fill damage 加描边外扩**（context.go:1162/1175）：按 width/2＋miter 上限膨胀包围盒，否则增量呈现必留残影。
3. **clip 栅格化改非零绕向**（internal/clip/mask.go）：配对逻辑改为带方向的 winding 累加，同时给 mask 分配加上尺寸缓存/复用。
4. **渐变对齐 Skia 语义＋提速**：ColorAt 增加 CTM 逆映射（或 brush 本地矩阵）；采样改预计算 LUT 取代逐像素 pow；补 computeTFocal 的 StartRadius 项。
5. **处置布尔运算**：要么排期实现真矢量布尔（扫描线/Martinez），要么先把 API 标记 experimental、去掉 2048 静默截断（超界返回错误），禁止错误结果无声外流。

---
*审查依据：docs/review/.findings_2d_api.txt（已核实后删除）；关键行号均为当前工作区实测。*
