# 文档级复核报告（RECHECK docs）

> 复核日期：2026-08-23。复核对象：`docs/review/` 下 R1–R7、ROUND2、ROUND3、FINAL_SUMMARY 的最终结论。
> 对照真源：`docs/ENGINE_UI_WIDGET_RENDER.md`（§2 主表 R / §3 组合表 C / §5 分期 W / §10 修订）+ `docs/RENDER_API_CATALOG.md`（§0/§7 接线状态）。
> 方式：只读源码与文档交叉验证，未改任何引擎文件。本轮抽验约 25 次检索/读码。

## 一句话直话

**三轮审查的核心结论基本站得住：抽的 9 条里 7 条与真源一致（既没审查误报、也没文档虚标），1 条审查漏看了一个细节（矢量文本路径其实接了 shaping，但不动摇 X05 主结论），1 条是目录文档的功能描述夸大（布尔运算）。没有发现「文档标 ✅ 但代码没接线」的反向虚标。**

---

## 二、对照总表

| # | 条目 | 审查结论 | 真源文档标注 | 判定 | 证据 |
|---|------|----------|--------------|------|------|
| 1 | 增量 LayerCache 死代码（X06 后半，R3-F01） | `ui/rendering/layer_cache.go` 零生产调用者，唯一引用方 tmp_vlprobe 编译不过 | 真源 §2 R3「Boundary 真缓存」✅ 指的是 **BoundaryCache**（有真窗 `ui_wr_r3_boundary`、boundary_skip 数万次实测背书），**从未**给增量 LayerCache 标任何完成态；§10 W1 收口记录写的也是 BoundaryCache。两个撞名物在 ROUND2 M3 已被审查自己辨析清楚 | **一致** | `ui/rendering/layer_cache.go:32`（NewLayerCache 全仓仅定义处）；`tmp_vlprobe/main.go:76,102` 调 `owner.LayerCache()`/`BuildFramePacketCached`，而 `ui/rendering` 中两符号均不存在（grep 证实）→ 死代码属实 |
| 2 | compositor-only 动画断头（R3-F09） | AnimatedOpacity 产出 Mutation 后没人接；packet.Mutations/CompositorDirtyIDs 无 embedder 生产者 | 真源 §2 R6 层动画 ⬜（W5 未开）、§5 W5 整波 ⬜——真源**没宣称**这条链已通 | **一致** | `ui/animation/implicit.go:53`（产出 Mutation）；全仓 grep `AnimatedOpacity` 仅本文件+status_test.go+doc.go 提及；`CompositorDirtyIDs` 仅 packet.go 定义+CloneShallow 复制，embedder 无生产者（grep 证实） |
| 3 | DamageRectSetter / OS 增量呈现断链（R7） | `PresentWithDamage` 丢参数；`DamageRectSetter` 全仓仅测试 mock 实现，rects 生产环境静默丢弃 | 真源 §2.1 只承诺 retained 下「damage Present（LoadOpLoad）」= **GPU 内** scissor 保留语义；API 目录把 `PresentFrameDamageRects` 标 ✅ 也是指 GPU 内多裁剪提交，未标 OS per-rect present 已通 | **一致**（附一条改进建议） | `gpu/webgpu/surface.go:310-314`（参数名 `_`，注释自认 wgpu-native 不支持）；`render/integration/ggcanvas/canvas.go:349,627` 类型断言；全仓 grep `SetDamageRects` 仅 canvas.go 定义/注释 + canvas_test.go mock。建议：目录 §3.7 给 `PresentFrameDamageRects ✅` 加一句「OS 级 partial present 未接，仅 GPU scissor」，防误读 |
| 4 | render/svg、render/surface、render/recording 零消费者（R4） | svg/surface 连 examples 都没接，彻底孤儿包；recording 仅示例+自测 | API 目录 §0：svg「🔌 无消费者」、surface「🔌 无消费者…未接入 render.Context」、recording「🔌 仅测试/示例」；§7.2 同口径 | **一致** | 本轮实测：全仓（除 docs/review 自身）`gpui/render/svg`、`gpui/render/surface` import **0 命中**；`gpui/render/recording` 仅 `examples/render_recording/main.go` + `render/s3c_m3_gpu_gate_test.go` + 包内自测 |
| 5 | filters/raster 是否被误当问题 | FINAL_SUMMARY 只作事实描述（「filters 仅 6 个 CPU 滤镜注册且未系统审」）；ROUND3 明确判「纯 init 注册包，无可藏雷之处」，**未列为缺陷** | AGENTS.md/CLAUDE.md 已知设计：filters/raster 为纯 init 副作用包（无顶层符号），不参与逐符号校验 | **一致（非误判）** | `docs/review/ROUND3_final_review.md` 第二节前两行；API 目录 §0 第 28–29 行同口径 |
| 6 | X05 主绘制管线绕过 shaping | GPU glyph-mask 主路径 `face.Glyphs(s)` 只有 cmap+advance，无 GSUB/GPOS/kerning | 真源全文无任何「主管线已接 shaping」宣称（§10 修订只记了 hinting/light 模式）；API 目录把 `DrawShapedGlyphs` 标 🧪 仅测试——即 shaping 结果的显式消费路径未接线，与审查同向 | **一致，但审查有一处漏看**（见第三节第 1 条） | `render/internal/gpu/gpu_text.go:179`；`render/text/face.go:160-220`（iterGlyphs 逐 rune GlyphIndex+advance，全程无 GSUB/GPOS）均复核属实 |
| 7 | shaper.go 注释矛盾（R6 报、ROUND2 M4 判属实） | 第 7 行注释写 OwnShaper (default)，第 21 行实际 `NewHbShaper()` | ——（纯代码事实，无真源条目） | **属实（复审确认）** | `render/text/shaper.go:7`「OwnShaper: … (default, ADR-048)」vs `:21` `var defaultShaper = NewHbShaper()`；**SetShaper 注释同样错**：`:28-29`「Pass nil to reset to the default OwnShaper (Pure Go GSUB/GPOS)」——nil 重置实际回到 HbShaper。另注意 `:16-19` 注释自己写明 defaultShaper=HbShaper，同一文件内自相矛盾 |
| 8 | 反向误差抽查：真源有没有「标 ✅ 但代码/窗口不在」 | ——（本次复核主动查的方向） | §2 表 ⬜ 的能力（R1/R6/R14/R15/R17/R20/R22）在 examples/ 下**均无对应目录**；✅ 的能力对应目录全部实存（单能力 19 + 组合 7 = 26 窗，含 main.go+README），与 FINAL_SUMMARY「26 个真窗全实存」对账相符 | **一致（真源状态诚实，无虚标）** | `examples/` 目录清单实测；§2/§3/§5 状态逐行比对 |
| 9 | 目录文档对布尔运算的描述（对照 X04） | X04：布尔运算是逐像素采样近似，EvenOdd 规则必错、2048 静默截断 | API 目录 §2 状态栏标 🧪 仅测试（正确），但功能描述写「**非凸、自交路径也支持**」——与 EvenOdd 必错的事实相抵触，描述夸大 | **文档虚标（轻）** | `docs/RENDER_API_CATALOG.md:70` vs `render/path_boolean.go:62-100` |

---

## 三、「审查漏看 / 文档虚标」专项结论

1. **审查漏看（不改结论，收窄表述）：X05 应收窄为「glyph-mask/MSDF 主路径」。**
   `render/text.go:1008` 的矢量轮廓路径（`drawStringAsOutlines`，2026-08-15 变换文本质量修复引入）调用的是 `text.Shape(s, c.face)`——这条路径**真的走了全局 Shaper（默认 HbShaper）**，连字/kerning 在该路径是生效的。审查三轮都没提这一处。因此 X05 的准确说法是：**字形掩码/MSDF 主路径（UI 生产默认）绕过 shaping**，矢量回退路径已有 shaping。P0 定级不变（UI 默认路径仍是坏的），但修复范围比审查表述的小。

2. **文档虚标（轻，1 条）：API 目录 §2 布尔运算行的功能描述夸大。**
   状态标 🧪 仅测试是对的（没虚标接线状态），但「非凸、自交路径也支持」这句话与 X04 的 EvenOdd 必错直接冲突。建议改为如实描述（如「NonZero 口径像素采样近似；EvenOdd 结果不可靠，2048px 截断」），或等 X04 重写后一并更新。

3. **「文档标 ✅ 但代码没接线」：未发现。**
   重点排查过的三个高危候选全部排除：① R3✅ 是 BoundaryCache 不是死的 LayerCache；② damage/present 相关 ✅ 都限定在 GPU 内 scissor/embedder 决策层，真源从没宣称 OS 级增量呈现通了（R7 审查与真源各说一层，ROUND2 M6 已正确归并）；③ ⬜ 能力对应的真窗目录确实一个都不存在，符合 AGENTS.md「空目录=未建，不得标 ✅」的红线。

4. **审查与 AGENTS.md 已知事实无冲突。**
   filters/raster 按「纯 init 副作用包」的定位被 ROUND3 如实快扫、未被当缺陷上报，符合项目已知设计；「声明未接线族」的判断（uniformStructTypes、ClipDepth 等）也与 API 目录大量 🔌/🧪 标注互相印证。

## 四、大白话总结

把这轮复核说成人话：**审查队没冤枉代码，文档队也没吹牛。**

审查里最扎耳的几条「死代码/断头路」，我们一条条回到代码和真源文档里对过：增量 LayerCache 确实没人用（引用它的那个 tmp 目录还编译不过）；compositor-only 动画确实是半截子——动画类把「变更单」吐出来了，但没有谁来消费；OS 级的增量呈现链确实断在 wgpu 那一层，参数传进去就被扔了。而这些东西在官方真源文档里要么压根没标完成、要么明确只承诺到「GPU 内部省绘」这一层——所以不存在「文档说行了其实不行」的虚标。反过来查也一样：文档里打了 ✅ 的能力，真窗程序都真实存在；还没关的能力（R1/R6/R14 这些）连目录都没建，符合项目自己定的红线。

找到的两处小问题：一是审查漏看了一个细节——文字的「矢量轮廓」备份路径其实已经接上 HarfBuzz 整形了，只是 UI 默认走的「字形贴图」主路还是绕过整形的，所以大结论不变，但要修的范围比审查说的窄一点；二是 API 目录里介绍「路径布尔运算」那行话说大了，写着「自交路径也支持」，实际上偶奇规则一碰就错，得改口。

至于那条吵过的旧案——shaper.go 注释说默认是 OwnShaper、实际是 HbShaper——再次核实：第 7 行和 SetShaper 的注释（第 28-29 行）都在吹 OwnShaper，第 21 行实际挂的是 HarfBuzz，而且同文件第 16-19 行的注释又说了实话，纯属一个文件里前后打架，审查没说错，改注释十分钟的事。

---

*本文档由文档级复核产生，只读验证，未修改引擎代码与既有审查文档。*
