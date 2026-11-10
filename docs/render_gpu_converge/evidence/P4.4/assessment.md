# P4.4 COLRv1 变量轴：评估与方案（2026-11-10，零代码）

## 0 现状

- COLRv1 paint graph 已实现静态求值（P3.8 批 5）：`render/text/emoji/colrv1.go` 解析
  1–32 号 paint，Var* 格式**按静态布局读取、varIndexBase 忽略**；COLR 头的
  `varIndexMapOffset` / `itemVariationStoreOffset` 已捕获但未使用
  （`render/text/emoji/colr.go:137-149`）。
- 变体基础设施在 `render/text` 已有：`normalizeCoords`（fvar 归一化）、
  `avarTable.apply`、`itemVariationStore.computeDelta(outer, inner, coords)`、
  `deltaSetIndexMap`；面对象有 `config.variations`。
- 但 emoji 是 text 的子包，**不能反向 import text** 去拿 IVS ⇒ 需要桥接。

## 1 对齐标准（OpenType COLR 1.9.1 + HarfBuzz 实现）

- `varIndexBase` 是 32 位 delta-set 索引对（outer=高 16 位、inner=低 16 位），
  HarfBuzz 直接用它取 delta（不查 varIndexMap）；COLR 的 varIndexMap 只服务
  glyph-level 映射。本仓按 HarfBuzz 口径实现，varIndexMapOffset 保留不消费。
- 每个 Var* 格式的变量字段顺序与缩放（HarfBuzz `Variable<T>` 逐字）：

| 格式 | 变量字段（顺序） | 缩放 |
|---|---|---|
| VarSolid | alpha | F2DOT14 |
| VarLinearGradient | x0,y0,x1,y1,x2,y2 | FWORD（字体单位） |
| VarRadialGradient | x0,y0,r0,x1,y1,r1 | FWORD / r* 为 UFWORD（钳 ≥0） |
| VarSweepGradient | centerX,centerY,startAngle,endAngle | FWORD / F2DOT14（角度 +1 bias 前加） |
| VarTransform | xx,yx,xy,yy,dx,dy | F16DOT16 |
| VarTranslate | dx,dy | FWORD |
| VarScale | scaleX,scaleY | F2DOT14 |
| VarScaleAroundCenter | scaleX,scaleY,centerX,centerY | F2DOT14 / FWORD |
| VarScaleUniform | scale | F2DOT14 |
| VarScaleUniformAroundCenter | scale,centerX,centerY | F2DOT14 / FWORD |
| VarRotate | angle | F2DOT14（×π 前加） |
| VarRotateAroundCenter | angle,centerX,centerY | 同上 |
| VarSkew | xSkewAngle,ySkewAngle | F2DOT14（×π 前加） |
| VarSkewAroundCenter | 两角 + centerX,centerY | 同上 |
| VarColorStop | offset,alpha | F2DOT14 |

## 2 方案

1. **emoji 侧**：`COLRParser` 加 `vars VarDeltas` 字段 + `PaintRootVar(gid, vars)`
   用**浅拷贝**注入（原 parser 不被改，天然并发安全）；`parsePaint` 各 Var* case
   按上表加 delta；`parseColorLine` 的 VarColorStop 加 offset/alpha。
   `VarDeltas` 接口：`Delta(varIndex uint32) int32`（nil = 静态实例）。
2. **text 侧桥接**：`ownParsedFont.COLRGlyphVar(gid, palette, variations)`——
   从 COLR 头取 IVS 偏移→`parseItemVariationStore`；`normalizeCoords`+`avar`
   得 F2DOT14 坐标；提供 `Delta(idx) = ivs.computeDelta(out, in, coords)`。
3. **接线**：`ColorRasterCache.Image` 增 `variations []FontVariation`（进缓存键），
   经 `rasterizeCOLR` → `COLG glyphVar`；调用点两处：`drawColorRasterGlyph`
   （CPU 腿，face 有余量）与 `render/text_dispatch.go` 的 GPU 颜色引擎。
4. **测试字体**：`skia/resources/fonts/test_glyphs-glyf_colr_1_variable.ttf`
   （COLR v1 + fvar 两轴 + IVS，200 个 v1 字形）拷入 `render/text/testdata/`；
   单测：同一字形默认实例 vs 轴极值，断言 paint 值/渲染像素不同；默认实例回归
   锁旧值。
5. **真窗**：`ui_wr_p38_color_font` 扩一卡（同一字形默认 / 轴两端三实例），
   双后端金图重种。
6. **边界**：VarClipBox（ClipList format 2）本轮不做，登记；varIndexMap 不消费
   （HarfBuzz 口径）。

## 3 载体登记（待批准后开工）

- 单测：`render/text/emoji/colrv1_var_test.go`（delta 表逐格式）、
  `render/text/color_font_var_test.go`（端到端：默认 vs 轴极值像素/值）。
- 真窗：`examples/ui_wr_p38_color_font` 扩卡 5（不新增窗）。
- 金图：该窗双后端重种（旧图含 4 卡布局）。
- 负向：把某个 delta 强行置 0 ⇒ 轴极值与默认实例渲染相同 ⇒ 单测必须红。
