# P3.3 富文本逐 run 样式 — 收口报告（2026-11-01）

> 依据：`assessment.md`（D1 Builder 下沉 render / D2 颜色+装饰+字重 / D3 Justify 空格分配 / D4 MaxLines+Overflow / D5 ui 迁移不做）。

## 1. 交付

- **公开 API（render/paragraph.go 新）**：`TextStyle{Color,Decoration,Weight}`（零值=继承外层作用域）+ `ParagraphBuilder`（`NewParagraphBuilder(face, base)`+`SetFamily`+`PushStyle/PopStyle/AddText/Build`）+ `Paragraph`（`StyleForRun/StyleCount`）+ `Context.DrawParagraphStyled`（逐 run 上色+装饰线+paint 恢复）。样式跨度和 shaper cluster 统一 rune offset，run→span 整数区间解析。
- **styledFace**：包装任意 Face（嵌入接口转发），按 span 边界切分、weight≠0 经 `FontFamily.Face(size, WithWeight)` 派生真 cut/合成 embolden；layout 的 face 切分分派从 `*MultiFace` 类型断言泛化为 `interface{ Runs(string) []FaceRun }`（MultiFace/styledFace 统一）。
- **Justify**（layout.go）：空隙分配到空格 glyph（cluster→原文 rune IsSpace 判定），双写 line.Glyphs 与 line.Runs[].Glyphs，末行不拉伸（Skia/Flutter 同）。
- **MaxLines/Overflow**：LayoutOptions 加 MaxLines+Overflow（Clip/Ellipsis/Visible）；ellipsis=末行尾部测宽收缩+Shape("…") 追加+run 列表合成展平。
- **ShapedRun.Style**：span 索引（-1=无样式，零行为兼容）。

## 2. 门禁终态

| 门禁 | 结果 |
|---|---|
| 能力单测 | ✅ Builder span 语义（Push/Pop 继承+run.Style 区间）/ Justify（内部行拉伸至 MaxWidth+末行不拉伸+双写一致）/ MaxLines+ellipsis（行数+尾宽+高度）/ DrawParagraphStyled paint 恢复+青色像素落地 |
| 真窗 `ui_wr_p33_richtext` | ✅ **native 与 go 像素一致 5/5**（区域探针：红/青/黄三 run 各 ≥1 命中、justify 右缘到达、ellipsis 尾部非背景）；金图两后端各 0.0000%；presents≈300 |
| 负向对照 | ✅ 摘 per-run SetRGBA → 红/青 run 消失（2/5）rc=1；还原复绿 5/5 rc=0（金图被负向轮快照覆盖→重播种后 diff=0 复验——负向跑写快照污染金图的改进项登记） |
| apidoc | ✅ rc=0（§13 P3.3 转正+§0 注记） |
| 五目标构建 / per-file | 后台（build-five-target.txt / tmp/per-file-tests），落盘后补记 |

## 3. 排障实录（全有实证）

Builder face 类型 `*text.sourceFace`（LoadFontFamily 单 ref）≠ `*MultiFace` → styledFace 泛化到任意 Face+family 派生；layout face 切分接口化（原 `*MultiFace` 类型断言吃掉 styledFace——route 单 run 实证）；Justify line 侧 extra 误加所有 glyph（双写 lockstep 单测抓到）；probe 坐标三连错（写死 372=卡 B 位置/越界 300/单点字形脆）改区域探针。

## 4. 偏差与遗留

- 负向跑覆盖快照→金图污染：evaluateGolden 应先 diff 后写（负向/重播两轮证实），登记真窗框架改进项。
- ui paintRuns 迁 DrawParagraphStyled：另步（D5）。
- 字重 run 需 SetFamily（无 family 降级 base，文档锁）；字号逐 run/背景色/段落级样式：第二版。
- 证据：`evidence/P3.3/`（assessment/summary/window-runs×5/build-five-target/per-file-tests）+ 真窗 `examples/ui_wr_p33_richtext/`。
