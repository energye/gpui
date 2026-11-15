# ui_render_base_valtypes

值类型谓词直驱真窗（O12 批 1c，RENDER_FEATURE_COVERAGE §12 NONE→EX，logic 为主+可视化佐证）。

## 覆盖面（logic 40）

- **卡 A Matrix**：`Identity/Translate/Scale/Rotate` 构造 × `IsIdentity/IsTranslation/IsTranslationOnly/IsScaleOnly/ScaleFactor/SimilarityScale/MaxScaleFactor/TransformVector`。语义实证：字段布局 [A B C; D E F]（C/F 平移项）、TransformVector 不含平移、非均匀缩放×旋转=非相似（ok=false）、均匀×旋转 scale=2 ok、超大圆角缩夹。画面：平移/缩放方块。
- **卡 B RRect 判型**：`NewRRect/NewRRectXY/NewRRectRadii/SetRect/SetRectXY/SetRectRadii/SetEmpty` × `IsRect/IsOval/IsSimple/IsSimpleCircular/IsNinePatch/IsComplex/Type`。语义实证（SkRRect computeType 对齐）：Oval 需 rX≥半宽 rY≥半高；nine-patch 判据=沿边配对（ul.X==ll.X && ul.Y==ur.Y && ur.X==lr.X && ll.Y==lr.Y）；r=半高≠半宽 的统一圆角是 Simple 不是 Oval。
- **卡 C Paint/Stroke/Dash Effective 链**：`NewPaint` 默认值全链（IsSolid/宽度 1/无 dash/effect）、`GetStroke` 设置读回、`EffectiveLineWidth/Cap/Join/MiterLimit/Dash/PathEffect`、`Stroke.With*` 链式读回、`Dash.NewDash/WithOffset/PatternLength/NormalizedOffset/Scale/Clone`。语义实证：奇数元素 pattern 按 Cairo/Skia 约定复制翻倍（[12,8,4] 长度 48 非偶 24），NormalizedOffset=offset mod（翻倍后）patternLen。
- **卡 D RGBA/Vec2**：`Premultiply/Unpremultiply` 往返精确（alpha<1）、零 alpha 全 0、`Vec2.Approx/ToPoint/PointToVec2` 往返。画面：0.4 alpha 混色块+Vec 旋转箭头。

## 语义记录（不是洞）

- Matrix.IsTranslation 与 IsTranslationOnly 源码口径同真（IsScaleOnly 特判无旋转偏斜，平移矩阵也算 scale-only 特例）。
- Dash 奇数元素翻倍语义与 Cairo/Skia 一致。
- 快照点探 want 需按实际混合值填（0.4 alpha 块=原色×0.4）。

## 门禁

```
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/ui_render_base_valtypes -auto-only
go run ./examples/ui_render_base_valtypes                # manual: 常驻
```

首播种 `GPUI_ACCEPT_GOLDEN=1`；卡内容改动需重播种。
