# ui_render_base_curvegeom

曲线几何内省直驱真窗（O12 批 1c，RENDER_FEATURE_COVERAGE §12 NONE→EX）。

## 覆盖面

- **卡 A Cubic/Quad 内省**：`CubicBez.Eval/Deriv/Tangent/Normal/Subdivide/Subsegment/Extrema/BoundingBox`、`QuadBez.Eval/Subdivide/Subsegment/Extrema/BoundingBox/Raise`。Deriv 端点=3(P1−P0) 读回、Subdivide 两半重组偏差 <1e-9、Subsegment [0.25,0.75] 与原曲线逐点一致、QuadRaise 与原二次逐点等价、紧致框⊂控制凸包框。画面：曲线+切向箭头+双框叠画。
- **卡 B Conic 族**：`ConicBez.Eval/Tangent/Extrema/BoundingBox/ToQuads`。W=√2/2 ¼圆弧 B(0.5)=(cos45°,sin45°)、W=1 与二次逐点等价、W=0 直线退化、ToQuads 四档 W（0.3/0.7071/1/1.5）发射 quad 逐点对真曲线点集距离 ≤tol=0.5。画面：四条 conic+ToQuads 折线叠画+¼圆弧缩略。
- **卡 C Solver+PathMeasure**：`SolveQuadratic`（两实根/重根/无实根/线性退化）、`SolveQuadraticInUnitInterval`、`SolveCubic`（单根/三根）、`SolveCubicInUnitInterval`；`NewPathMeasure` 多段路径（Line+Quad+Cubic+Close）`Length()` 与逐弦 oracle 相对误差 <1%、`Sample()` 中点/尾段角度、`NewLine.Length/Midpoint/Reversed/Subdivide/BoundingBox`。画面：根刻度四行+路径采样点。
- **卡 D ClassifyQuad**：`ClassifyQuad` 四态（QuadOK/QuadDegenerate 共线/QuadBowTie 自交/QuadNonFinite NaN）+ `QuadKind` 常量序 + `ErrQuad*` 三哨兵 var。画面：三态实画（NonFinite 不实画）。

## 发现

- `quadKindErr`（QuadKind→ErrQuad* 映射）为 internal，窗内以哨兵 var 非空断言登记，语义记录不判死。
- `ConicBez.Chop/Subsegment/quadApprox/quadError` 为 internal，经 `ToQuads` 间接覆盖（发射 quad 逐点误差探针）。

## 限界

- 曲线细线在快照缩略图上 AA 稀释：细线/1px 特征不设快照点探（批 1b 纪律），由腿读回+黄金图逐位钉死。
- `PathMeasure.Sample` 的 angleDeg 是切向角（度），resvg textPath 语义；非路径长度比例。

## 门禁

```
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/ui_render_base_curvegeom -auto-only
go run ./examples/ui_render_base_curvegeom                # manual: 常驻
```

首播种 `GPUI_ACCEPT_GOLDEN=1`；卡内容改动需重播种。
