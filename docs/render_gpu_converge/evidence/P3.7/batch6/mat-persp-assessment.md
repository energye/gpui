材质+透视真实现 影响面评估（2026-11-07，开工前停报告版；批 3b 登记「组合顶点机已就绪，只差权重透传与钉」的兑现项）
========================================================================================================================

被测能力（一句话）：把 quad 片元材质腿从「仿射-only」放开到「透视」——
组合顶点机的权重透传。材质（用户 WGSL 片元）× 透视 quad（自持权重/画布投影层）
组合后真画出来，透视正确插值由既有引擎顶点段承担。

一、码面事实（本会话逐条核实；「已就绪」与「缺失」分列）
--------------------------------------------------------

**已就绪（零改动，实测核对）：**
1. **组合模块顶点段已实现真 clip-space w**：`render/shader_compose.go:182-205`
   ——36B 顶点（pos/uv/tint/hom_w），`out.position = vec4(pos.xyz*w, w)`，注释明文
   "same form as textured_quad_persp.wgsl"：光栅器按 w 除 xy、UV 透视正确插值；
   仿射喂 hom_w=1 恒等。
2. **36B 打包器已按真权重打包**：`image_pipeline.go:1342-1381`
   `buildPerspVerticesInto` 逐角写 `cmd.Weights`（9 float：x,y,u,v,tint,weight）。
3. **材质发射已走 36B 缓冲**：`render_session.go:4458-4462` 材质命令发射调
   `buildPerspVerticesInto`，但**把 Weights 强拍成 `{1,1,1,1}`**——这一行就是
   「权重没透传」的全部本体。
4. **材质槽 draw-call 发射与透视无关**：`render_session.go:4740-4817` 组合管线
   两态通吃（hom_w=1 即仿射恒等），slot.perspective 不参与材质路指派。

**缺失（本项要动的全部）：**
1. `render_session.go:4458`：材质发射去掉 Weights 强拍，透传命令真权重。
2. `render_session.go:4552-4577` 槽循环 **Perspective 分支在材质分支前**——
   材质+透视命令会掉进标准透视腿（textured_quad_persp 管线）**静默丢材质**，
   这正是上游必须具名拒的根因。修=材质分支提前（材质+透视进材质槽，
   canMergeImageDraw 按 Perspective 分槽的现成语义保持，同材质 persp/affine
   各自成段，喂同一条组合管线与 36B materialVertBuf）。
3. `gpu_render_context.go:1493-1501`：`QueueImageDrawMaterial` 无权重参数——
   按接口全量切换制扩签名加权重尾参（调用点 3 处同笔切：m4_quadimage.go:95、
   vertices_atlas.go:95/465）。
4. `m4_quadimage.go:74-77`（校验期）与 `:135-138`（GPU 臂投影组合守卫）两处
   `ErrQuadMaterialUnsupported` 拒绝 → 改带权重排队；**材质+bicubic 保持具名拒**
   （16-tap cubic 进组合模块是另一个组合核，另行评估）。
5. `vertices_atlas.go` 精灵路三处 material+perspective 跳过+计数
   （`:83-102/:153-155/:460-465`）→ 带权重排队（画家序合批语义不变）。
6. 若 buildImageResources 另有同族上游拒绝（批 3b 注释提及）落地时一并摘除。

**连带面（必须同步，否则真窗自相矛盾）：**
- ⑥ `ui_wr_p37_scene` D 面板现在演示的恰是「材质+透视被拒」——开启后该笔
  必画，面板语义反转：改演示 **material+bicubic 拒**（同一哨兵
  `ErrQuadMaterialUnsupported`、同一拒-画-对照结构），probe 9/15（H22 拒绝账）
  断言保持成立；语义像素锚按新拒笔重算，⑥ 金图重播种。
- ③ `p37_material` 加「材质+透视」臂做像素收口：梯形上 gradient 材质
  （g=0.35+0.65·uv.y），透视正确插值的落点**解析可算**（权重非 1 时 uv.y
  按 w 插值，语义像素锚按公式反推）；③ 金图重播种（门禁相 RUN_SECONDS=20 口径不动）。
- `ErrQuadMaterialUnsupported` 哨兵 doc、三入口「affine-only」措辞、
  `docs/RENDER_API_CATALOG.md`（若公开面变化）、批 3b design 口径行更新。
- H22 拒绝账与缺口 4 的 FilterToRender 均不受影响（拒面换成 bicubic 后账照记）。

二、对齐依据（Skia / Flutter / Godot）
----------------------------------------
「任意自定义片元 × 透视正确插值共存」是行业标准形态：Skia 的 SkImageShader/
SkCanvas.drawVertices 让任意 SkShader 骑透视顶点（透视正确性是顶点/光栅器职责）；
Flutter impeller 的 entity 管线自定义片元配任意几何；Godot canvas item material
配任意变换。本引擎组合模块（引擎顶点带 hom_w→真 clip-space w、用户片元按插值
UV 采样）与三者同构——本项不是自创形态，是把行业标准组合补齐到材质腿。

三、验收口径（预告，拍板后细化为红测清单）
------------------------------------------
1. 红测先行：材质+透视不再被拒（identity 材质+透视 ≡ 无材质透视 quad 逐位——
   批 3b 钉 1 同款）、gradient 材质透视插值解析断言、材质+bicubic 仍具名拒、
   精灵路 material+persp 不再 skip+计数。
2. 真窗：③ 新臂 + ⑥ D 面板重构，双后端金图重播种复验（删基线→播种→复验→
   稳定轮→cmp）。
3. 门禁：build/vet/gofmt/apidoc、触及包逐文件回归、`-race`（触及共享发射
   结构时）。
4. 性能：合批语义不回退（gpu_ops 有界断言照跑）。

四、拍板结果（2026-11-07，用户）
--------------------------------
1. **范围=材质+透视与材质+bicubic 都实现**。bicubic 的契约分叉（引擎喂采好样的
   纹素扩展契约前缀 vs 用户自写 16-tap——标准路 bicubic 是引擎内嵌核、组合模块
   世界里用户片元持采样权，Godot TEXTURE 语义）**作为开工轮第一个拍板点**；
   本评估的「权重透传」部分机械就绪度高，「bicubic 契约」部分开工时补评估。
2. **真窗载体=新建专项窗**（材质×透视专项，③⑥ 不动）。
3. **先登记不开工**（2026-11-07）：本文件即立项评估正本，实现轮开工时按
   完整流程走（红测先行→发射链/门面→专项真窗→门禁→分枚提交）。

**连带后果（实现轮必须一并做，与本评估「连带面」一致）**：无论专项窗与否，
⑥ D 面板现演示的「材质+透视被拒」在开启后变必画——拒面迁到 material+bicubic
（同一哨兵，probe 9/15 断言形态保持），⑥ 金图随之重播种；此项属能力开启的
必然连锁，不由专项窗替代。
