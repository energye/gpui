# P3.5 附修 — 面积公式口径不统一（真窗 1 逻辑探针抓到）

- 日期：2026-11-02　发现场合：建 `examples/ui_wr_p35_conic` 真窗的逻辑探针 L4（扇形面积 vs πr²/4）
- 归属：P3.5 批 1 交付面（conic 真 verb + 边界展开 + `quadArea` 精确公式那一条）
- 口径：**引擎层修复 + 单独提交**（用户拍板 2026-11-02：先修面积 bug，单独提交；探针不降口径）

## 现象（实测可复算）

`render` 包内一条 90° 圆锥扇形（圆心 → 半径直线 → 精确 conic 弧 → 闭合）：

| 量 | 值 |
|---|---|
| `Path.Area()` | **68415.926156** |
| 同形状 20000 段密折线（纯直线）面积 | **31415.923306** |
| 解析 πr²/4（r=200） | **31415.926536** |
| 超出量 | **37000.0029 / 37000.0000**（= ½(x_端·y_端 − x_起·y_起)，分毫不差） |

## 根因（定位到函数）

`render/pathops_area.go` 各 verb 的 Green 贡献**口径不统一**：

| 函数 | 原口径 | 与密折线 ½∫(x·dy − y·dx) 的差 |
|---|---|---|
| `lineArea` | 对称式 ✓ | 0.0000 |
| `cubicArea` | 对称式 ✓ | 0.0000 |
| `quadArea` | **∫x·dy** | 每条曲线边偏 **½(x₁y₁ − x₀y₀)**（实测 case3 = 37000.0000） |
| `conicArea` | 累加 `quadArea` | 继承同一偏差（另一 conic 实测 1374.9999） |

两种口径只在**纯曲线闭合环**上等价（该项沿闭合链望远镜相消）；一旦路径混有直线段与曲线段——扇形、圆角矩形、布尔结果——就整体偏移。**为什么以前不红**：批 1 的面积钉 `TestQuadAreaExactAgainstNumericIntegral` 把数值参照也写成 ∫x·dy（两边同口径 → 恒等成立），而历史测试路径都是齐次的；`TestConicArcQuarterCirclePixelFill` 那条混形路径的弧两端点恰好 x·y=0（落在坐标轴上）→ 偏移为 0，侥幸通过。

## 修复

`quadArea` 保留批 1 已验证的 ∫x·dy 闭式，减去端点项 ½(x₁y₁ − x₀y₀) 化成对称式；`conicArea` 随其链条自动归正；`lineArea`/`cubicArea` 一字未动。API 面零变化（两者皆包内函数），计数 144/184/219/18/172/26 不变。

## 新增与改动的钉

- 新增 `render/pathops_area_convention_test.go`（5 条）：
  `quadArea`/`cubicArea`/`lineArea`/`conicArea` 逐一 vs 各自密折线对称积分；
  混合扇形（三组圆心偏移，含刻意避开坐标轴的那组）vs πr²/4 ≤1e-6 相对；
  抛物扇形 vs 同几何密折线 ≤1e-6；圆角矩形 vs **纯直线参照路径**（`densePolylineOf`）≤1e-5，
  并与"理想圆角"解析值仅作 1e-3 信息位比对；`Reversed()` 只改符号；
  另加 `TestMixedAreaEqualsDenseForManyShapes`（conic/quad/cubic/混合链 × 三组偏移圆心）。
- 改口径 `render/curve_conic_test.go::TestQuadAreaExactAgainstNumericIntegral`：数值参照从 ∫x·dy 改对称式，
  注释写清这段历史（批 1 钉的是 ∫x·dy 口径，真窗扇形探针暴露了它与直线口径不兼容）。
- 批 5 面积恒等收紧：`TestBooleanAreaReconciliation` 相对误差 2e-3 → **5e-4**；
  新增 `TestBooleanResultAreaMatchesDenseReference`：布尔混合结果的面积 vs 同 verb 密折线参照 ≤1e-3 相对（绝对口径，不再只靠恒等式自洽）。

## 门禁（`gates.txt`）

定向（Area|Quad|Conic|Cubic|Boolean|PathOp）**136 条 PASS / 0 FAIL**；同集合 `CGO_ENABLED=1 -race -count=2` ok；
conformance 双后端 PASS；`render/internal/gpu`、`render/recording` 整包 ok；`go vet ./render` rc=0；
五目标交叉构建见 `../build-area-fix/`。

## 遗留（登记不隐瞒）

- `conicArea` 仍走 `ToQuads(1e-3)` 分解（扇形残差实测 1.21e-8 相对）：闭式有理积分可再收一个量级，属独立优化项，挂后续。
- 本修复改变了 `Path.Area()` 对**混合路径**的数值（变准）。`Path.Area()` 生产消费者为零
  （仓库内 `.Area()` 消费点全是 `Rect.Area`），故无金图/像素影响；全量逐文件门禁随真窗收口那一轮统一复跑。
