# ui_wr_p38_svg_showcase — P3.8 全面压测 vs resvg 参照

整体大窗：一枚**密集标准压测 SVG**（testdata/showcase.svg，1024×640，全功能
面：多 stop 渐变×2+reflect、径向 fx/fy、pattern（oBB 内容+x/y 相位）、滤镜
（blur/morphology/dropshadow/colormatrix/turbulence+flood·composite·offset·
merge 链）、clipPath（多形状+evenodd 星形）、luminance mask、use/symbol 组
opacity、switch、CSS（#id/.class/[attr]/g>rect）、DOCTYPE 实体、text+tspan、
textPath 沿波线、外链+data-URI SVG 双图像源、skew/rotate/transform-origin、
stroke-dasharray）——左面板本引擎 1:1 渲染，右面板 **resvg 0.42 参照渲染**
（同一文件、Ahem 字体双侧钉死、`--skip-system-fonts --use-font-file`）。

## 门禁

- logic 4/4：解析零警告、参照就位、**网格 grade ≤ 24**（conformance close
  口径；作者时实测 8.15，掩码区域修复后自 11.28 下降）、双面板就位。
- pixel 11/11：文档分数位颜色断言（hero 暖白环/badge 绿/mask 蓝/pattern 绿/头带暗/chips/反射带/标题渐变/正文）+ **pattern 格边竖缝列差分**（pixel_pattern_no_seam，逐列比左右邻列，周期暗列=FAIL）。
- golden 0.0000%：native 种子 → native + go/GL 复验逐位一致。

## 参照再生成

```
/tmp/resvg/target/release/resvg --skip-system-fonts \
  --use-font-file render/text/testdata/ahem.ttf --languages en \
  examples/ui_wr_p38_svg_showcase/testdata/showcase.svg \
  examples/ui_wr_p38_svg_showcase/testdata/showcase_ref.png
```

resvg 构建源：/tmp/resvg（v0.42，语料同源）。参照 PNG 已入库为固定标准。
