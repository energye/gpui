# ui_render_base_pathdiag

路径构造/查询直驱真窗（O12 批 1c，RENDER_FEATURE_COVERAGE §12 NONE→EX）。

## 覆盖面

- **卡 A PathBuilder 造型**：`Star/RoundRect/Polygon/Circle/Ellipse/Build/Path`，DetectShape 读回（圆→ShapeCircle 参数 center/r、星→ShapeUnknown、RoundRect→ShapeRRect 参数 w/h/cornerR）。
- **卡 B DetectShape 判定矩阵**：rect（MoveTo+3×LineTo+Close）、rrect、手工 kappa 构造圆弧（MoveTo+CubicTo 无 Close）→ShapeArc 恢复 center/r/Angle0/Angle1（300° 扫角）、折线/空路径→ShapeUnknown、`Shape*` 六常量全枚举。
- **卡 C Path 查询**：`CurrentPoint/HasCurrentPoint` 生命周期（空→MoveTo→LineTo→CubicTo→Close）、`Path.Reversed`（verb 数不变、反转后首 MoveTo=原子路末点）、`FlattenCallback(tol=0.1)` 采样数+真曲线中点覆盖。
- **卡 D Append+别名**：`Path.Append` verb 数=a+b、`Build()==Path()` 同指针。

## 发现（引擎第五件，先红钉子同族复现）

- **Append(≥2 子路)→Fill GPU 模式画面为空**：`Path.Append` 拼接两个普通三角形后 `SetPath+Fill`，op 入队无错但画面大面积黑、仅 AA 边缘碎片——比先红钉子 `render/setpath_multisub_fill_test.go` 的环洞更强（普通双三角形也空白）。dc.MoveTo 直画双子路已证可用，窗内画面用该形态。修复面同钉子：render/internal/gpu FillPath 链，台账 O13 子项待修。

## 限界

- C 卡原路径+Reversed 双描边叠画为细线混色特征，不设快照点探（腿探针+黄金图钉死）。
- DetectShape 判定 tolerance 1e-3（shape_detect.go 内部常量）；手写 kappa 弧端点需落在此容差内。

## 门禁

```
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/ui_render_base_pathdiag -auto-only
go run ./examples/ui_render_base_pathdiag                # manual: 常驻
```

首播种 `GPUI_ACCEPT_GOLDEN=1`；卡内容改动需重播种。
