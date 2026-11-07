# P3.8 批 5 交付：彩色字形四格式静态完整（sbix / SVG 表 / COLRv1 + 双判断收口）

- 日期：2026-11-07（交付即收口：真窗双后端各自基线金图逐位）
- 依据：assessment.md D4 A 档 + design.md 批 5 细化（2026-11-07 三项拍板：D5-1 渲路=render.Context 离屏；D5-2 读回前置=窗内一次读帧切片；D5-3 节奏=单枚收口）

## 1 交付物

**数据面（render/text + render/text/emoji，纯解析零渲染）**
- `emoji/svg_table.go`（新）：OT 1.9.1 SVG 表解析（version=0 头、DocumentList、record 排序/边界校验 fail-closed、gzip 0x1F8B08 透明解压带 1MiB 上限）+ `SVGGlyph{Doc,UsesCurrentColor,UsesPalette}`
- `emoji/colrv1.go`（新）：COLRv1 头五偏移 + BaseGlyphList/LayerList/ClipList 解析 + Paint 格式 1-32 静态求值数据树（Var* 按默认实例 deltas=0；sweep 角度 +1.0 bias；F2DOT14/Fixed/FWORD 解码；变换族按 spec 公式合成 2×3）；懒解析 + 路径环检测（`ErrPaintCycle`，ColrGlyph 自引用向上传播）+ 深度 64 护栏；子图失败按 spec「忽略该子图继续渲染」（nil child）
- `emoji/colr.go`：v1 头解析、HasGlyph v1 优先（spec 混用规则）、GetGlyph v1 走 PaintRoot+ClipBox、**colr.go:177 假注释改真**（v0 全量 + v1 静态）；`ParseCPALTable` 独立抽出（SVG var() 调色板复用）
- `color_font_adapter.go`：colorBackends +sbix/svg/pal 四字段；GlyphType 产出 GlyphTypeBitmap(sbix)/GlyphTypeSVG；BitmapGlyph CBDT→sbix 分流（BestStrikeForPPEM）；SVGGlyph/Palette 接口方法
- `color_font.go`：ColorFont 接口 +`SVGGlyph`/`Palette`（全量切换，唯一实现者 ownParsedFont 同笔）
- `color_glyph_hook.go`（新）：`SVGTableRenderer`/`COLRv1Renderer` 两个原子钩子（SetFontResolver 同款）——依赖倒置：text 是数据面、像素生产在 render 层（svg→text 单向依赖不可反）
- `color_raster.go`：rasterize +GlyphTypeSVG/GlyphTypeCOLRv1 分支（未装钩子 fail-closed 报错，退轮廓语义）
- `draw_emoji.go`：**COLR 臂落地删 TODO**（draw_emoji.go:70-73 双判断收口）——COLR/SVG 臂走 ColorRasterCache 扁平化（包级共享缓存），不再是轮廓兜底

**像素面（render 层，render 零机制增量）**
- `render/color_glyph_render.go`（新）：COLRv1 静态求值器——绘制树→render.Context 离屏（Solid/三渐变→SetFillBrush（线性 3 点按 spec 注记的投影归约 p3））、PaintGlyph→DrawPath+Clip、Transform 族→共轭矩阵（设备空间绘制，几何显式 mapX/mapY，brush 求值空间无歧义）、Composite 28 模式→双离屏+BlendMode（Dest 特例直画；未识别→CLEAR per spec）、ClipList→ClipRect、无 clip 时 graphBounds 有界性计算（spec boundedness 表）；`CropImageToInk` 共享口（origin 整像素取整）
- `render/svg/color_glyph.go`（新）：SVG 表字形栅格（Parse→RenderTableGlyph→3em 窗口离屏→裁墨迹）；`InstallSVGTableRenderer()` 显式安装
- `render/svg/color_table.go`（新）：`Document.RenderTableGlyph(dc,id,opts)`（use 语义单元素渲染、无根裁剪、currentColor 注入 root color 属性、CPAL→--colorN tokens）+ `resolveVarTokens` var() 解析（fill/stroke/color/stop-color 四拦截点；未知名走 fallback，无 fallback=不画 fail-closed）
- `render/options.go`：+`WithCPUOnly()`（离屏栅格跨进程/后端逐位确定的显式口）
- `render/internal/gpu/glyph_mask_engine.go`：**fontID 混入原始字体字节指纹**（RawFontDataProvider）——同名姊妹字体（goregular 与其彩色派生）图集键碰撞串图的底层洞修复

**测试与字体**
- 单测：emoji（svg_table 5 组 + colrv1 7 组钉测）、text（adapter sbix/SVG 表拼接 + e2e 五字体文件 TestP38ColorFonts_E2ERaster）、render（COLRv1 端到端 5 组像素钉）、svg（表字形栅格 4 组像素钉）
- `render/text/testdata/mkcolorfont/`（新）：goregular 表目录手术生成器，产五枚彩色字体（cbdt/sbix/svg/colrv1/colrv0，自校验后入库 testdata）
- 真窗 `examples/ui_wr_p38_color_font/`（新）：四卡实时文本（Canvas.SetFont+DrawString 自动分流），logic 10 探针 + pixel 7 探针 + 金图双后端

## 2 五只实现洞（全部探针/对照实验抓回）

1. **COLRv1 矩阵约定读错**：render.Matrix 是 row-major（x'=Ax+By+C，C=平移X），初版按 canvas 列向量习惯映射 → 平移变缩放（Y×401 探针抓回）→ spec 2×3 映射改为 {A:xx,B:xy,C:dx,D:yx,E:yy,F:dy}
2. **ColorStop 步长错**：初版按 8B/stop 解析（F2DOT14×2+u16 实为 6B；Var 10B）→ sweep 钉测抓回
3. **dc.Close() 误用**：那是关闭整个 Context（context_construct.go:191）不是闭合子路径（ClosePath 才是）→ fillSurface 全空；三组最小复现隔离出根因
4. **ExtractOutline y-down 约定**：轮廓 y 负值在基线上方（autohint.go 注释在案）而 COLRv1 几何 y-up → COLRv1 字形画到基线下方（快照绿 B 位置钉回）→ glyphDevicePath/graphBounds/ahemBox 三处翻 y
5. **图集键串图（底层洞）**：fontID=hash(FullName+NumGlyphs)，同名姊妹字体（四个 goregular 彩色派生）键全同 → 卡 2 显示卡 1 的蓝位图、卡 4 显示卡 3 的红圆（快照人工复核抓回）→ fontID 混入 RawFontData 字节指纹

**确定性收口（第三只洞的延伸）**：离屏 NewContext 在非 CPUOnly 进程下自动挂 GPU（ensureGPUCtx:358）→ 渐变/AA 的 GPU 光栅化帧间非确定（native 自身复跑 0.0821% 抖动实测）→ +`WithCPUOnly()` 显式口，两个色彩字形栅格器离屏全部走 CPU → 双后端逐位确定。

## 3 验证账

- 门禁：build rc=0 / vet rc=0（触及包）/ apidoc rc=0（catalog 补 WithCPUOnly/InstallCOLRv1Renderer/CropImageToInk/InstallSVGTableRenderer/TableGlyphOptions 五符号）/ conformance PASS / 存量窗 `ui_render_base_svg` 金图 0.0000%（627200px，var() 解析零行为变化实证）
- -race：emoji/svg 绿；render/ + render/internal/gpu/ 绿（fontID 钉测 Opt26 按新契约改写+补同名防碰撞回归钉 TestOpt26_FontID_DistinguishesSameNameSiblings；colorStubParsed 补接口两方法）；render/text 触及面（color*/COLR/SVGTable/adapter/e2e）race 绿——全包 race 双超时（10m/40m，包体量 race 下>40m，非竞争），按纪律分文件跑在案
- 负向 5 条（变异→红→还原绿）：
  - N1 摘 sbix 分流 → e2e 红 ✓
  - N2 摘 SVG 钩子安装 → svg 栅格钉红 ✓
  - N3 摘 COLRv1 钩子安装 → 窗 logic raster_colrv1_wired 红 ✓（单测层经 renderCOLRv1WithPalette 绕过钩子，不可观测——载体在窗 logic）
  - N4 摘 ClipList→graphBounds 回退 → ClipList 尺寸钉红 ✓（注：摘 ClipRect 调用不可观测——canvas 即 clip box，裁剪语义由画布边界承担；正确的变异点是 bounds 来源）
  - N5 摘 Composite 模式映射 → CompositeDestOut 像素钉红 ✓
- 真窗 `ui_wr_p38_color_font`：native seed→verify×2 + go/GL seed→verify 各自基线 0.0000%（912640px）logic 10/10 pixel 7/7；gpu_ops≈35000 cpu_fallback=0
- 存量：批 1-4 svg 窗零影响（ui_render_base_svg 0.0000% 实证）

## 4 已知边界与登记

- **双后端金图按后端分基线**：native 与 go/GL 的图集 quad 在陡峭渐变上有半 texel 级采样差（跨基线 diff 0.27-0.31%，纯色位图免疫故此前潜伏）——底座采样一致性登记为工程项（非批 5 范围）；各后端对自身基线逐位确定（native ×2、GL ×2 实测）
- COLRv1 Var 变体 deltas、CPAL 调色板切换/变体、fvar/avar：P4.4（design.md D4 边界不动）
- SVG 表禁元素（text/foreignObject/switch/script/a/view）：批 1 拒绝面同款处理
- draw_emoji CPU 腿的 COLR/SVG 臂与本批钩子共享 ColorRasterCache；GPU 腿图集键 SVG/COLR 按 fg（currentColor 保守策略，var-free 文档多花图集位登记）
