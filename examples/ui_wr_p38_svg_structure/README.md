# ui_wr_p38_svg_structure — P3.8 批 4c SVG 语义真窗

批 4c 七批语义工作的真窗验收载体（四卡 2×2，1460×800，卡片 SVG 1:1 烘焙走
GPU 图像路径）。覆盖批 4c 新能力面：

| 卡 | 能力 | 语义点 |
|---|---|---|
| 1 switch 条件处理 | usvg switch.rs | systemLanguage 命中（en）/ 选中 display:none 渲染为空**不回退** / 条件失败整组隐藏 |
| 2 CSS | simplecss 语义 | #id / [attr] / g>rect 子代组合器 / specificity（id 胜后置 class）/ !important 忽略（声明序后者胜）/ `<style type>` 非 CSS 整表跳过 |
| 3 image 数据源 | usvg image.rs | 外链相对路径（`ParseConfig.ResourcesDir`→testdata/res63.png）/ SVG 图像源（data URI 子文档递归渲染）/ SVG2 auto（width=126 → 高 61/63×126 等比）/ 显式 height=0 跳过 |
| 4 use·symbol·origin·pattern | usvg use_node/resolve_transform | use opacity=0.5 组层（PushLayer）/ symbol 自身 opacity / SVG2 transform-origin（rotate(45) 绕 viewBox center）/ pattern：oBB 内容按 fill bbox 缩放 + x/y 铺贴相位 |

## 运行

```
export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so LD_LIBRARY_PATH=$PWD/lib
RUN_SECONDS=5 go run ./examples/ui_wr_p38_svg_structure -auto-only   # 门禁
go run ./examples/ui_wr_p38_svg_structure                            # 常驻人工
GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=5 go run ./examples/ui_wr_p38_svg_structure -auto-only  # 种金图
GPUI_BACKEND=go RUN_SECONDS=5 go run ./examples/ui_wr_p38_svg_structure -auto-only       # go/GL 后端
```

## 验收口径

- logic 6/6：parse_all_four（警告计数恰 [0,1,0,0]——卡 2 的 type 跳过具名警告）、
  switch_single_child（选中恰一 + display:none 不回退）、css_materialized_order
  （物化串中 id 声明位于 class 之后=specificity 排序）、image_sources（外链
  63×61 解码 + SVG 子文档 + 零高仍解码）、use_symbol_targets（RefKind=symbol +
  symbol opacity 属性）、origin_pattern_attrs（transform-origin=center + oBB）。
- pixel 20/20：四卡逐语义点颜色断言（含两组 0.5 混合精确值、旋转菱形出界位、
  pattern 三相位）。
- golden 0.0000%：native 种子基线，native ×2 与 go/GL ×2 全部逐位一致
  （跨后端同基线零差）。
- screen check 手动 `-screencheck`（与其它 P3.8 窗同口径：金图逐位+像素探针承重）。

## 资源

testdata/res63.png — 63×61 外链图像（resvg 语料 image-63x61.png 同源）。
