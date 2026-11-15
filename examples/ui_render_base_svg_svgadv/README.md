# ui_render_base_svg_svgadv

svg 高级面直驱真窗（O12 批 3，RENDER_FEATURE_COVERAGE §12 NONE→EX）。

## 覆盖面（logic 17）

- **RenderTo/RenderToWithColor**：同文档双渲染——override 侧红圆→绿（颜色覆盖全链生效）、原侧红保留。
- **EffectiveStops href 链**：g2 href→g1 经渲染落地断言（渐变填充读回）。
- **值类型**：`ViewBox` 四字段、`PreserveAspectRatio{ParAlignXMidYMid,Slice}`、`LengthUnit` 十一常量（Px=0…Percent/Em/Ex）、`LengthAxisX/Y`、`SpreadMode/CSSDecl/StyleBlock/ParseWarning`（合法文档零警告）/`RefKindNone`。
- **元素面**：`ElementByID` 命中 rect/缺元素=nil。

## 门禁

```
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/ui_render_base_svg_svgadv -auto-only
```

首播种 `GPUI_ACCEPT_GOLDEN=1`；卡内容改动需重播种。
