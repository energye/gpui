材质×透视/bicubic 专项 · 修复正本（2026-11-07 开工；A 批=权重透传腿，B 批=bicubic 纹素契约）
================================================================================================

立项与拍板：`mat-persp-assessment.md`（范围=透视透传+bicubic 两半都做；真窗=新建
专项窗；bicubic 采样权契约拍板=**引擎喂纹素**——契约前缀扩展，对齐 Godot
「TEXTURE 按 item 档位采样」语义）。

A 批：权重透传（2026-11-07，本节）
----------------------------------

**红测先行**（`render/p37_mat_persp_test.go` 五钉，先红后绿，
`matpersp-red-test.txt` 正本——4 能力红 + 拒绝保持绿）：

| 钉 | 判据 |
|---|---|
| TestP37MatPerspQuadIdentityBitwise | quad 口自持权重：identity 材质+透视 ≡ 无材质透视**逐位**（47×49 区 diff=0），不再具名拒 |
| TestP37MatPerspGradientBandAnalytic | 透视正确插值**解析钉**：uv.y 带状材质在梯形（TL/TR w=2、BR/BL w=1）上的色带分界=hom_w 公式反推的 y=70/3≈23.33——(40,26) 探针专抓「权重被拍平成 1」回归（仿射分界 30，该点会错画红） |
| TestP37MatPerspProjectionLayerIdentityBitwise | 材质+画布投影层（perspLeg 权重来自 projMapQuad）identity ≡ 无材质逐位 |
| TestP37MatPerspSpriteRoadDrawn | 精灵路 material+persp 不再 skip+MaterialSkips，画面 ≡ 无材质透视精灵逐位 |
| TestP37MatPerspRefusalsRemain | 拒绝面收缩有界：behind-eye（投影层）仍 ErrQuadBehindEye（带不带材质同错）、材质+blend≠Normal 仍在 blend 校验拒、DrawImageEx 材质+blend 仍 skip+计数 |

**实现**（权重透传四层全通）：

1. `render/m4_quadimage.go`：校验期 material+Perspective 拒绝摘除（材质+bicubic
   保留，B 批摘）；GPU 臂材质分支带 `perspLeg+wFinal` 排队（投影层组合权重
   w=w_proj×w_own 照旧合成）。
2. `gpu_render_context.go QueueImageDrawMaterial`：签名扩展 `perspLeg bool,
   weights [4]float32`（接口全量切换制：调用点 3 处同笔——m4_quadimage /
   vertices_atlas×2；affine 材质 perspLeg=false 逐位不变）。
3. `vertices_atlas.go`：精灵路两处 material+perspSlot 放行（老口/新口，角点权重
   经既有公共段算好透传；behind-eye 仍整精灵拒；**材质+bicubic 透视槽暂仍
   skip+计数**——纹素契约随 B 批）；CPU 腿材质 skip 保持（P3.2 无求值器契约）。
4. `render_session.go`：buildImageResources 材质守卫摘 Perspective 拒（保留
   blend/closed）；`packPerspectiveQuads`/`degradePerspectiveCmds` 排除材质命令
   （材质+透视不进标准透视缓冲、不吃仿射降级——降级即丢材质=裸画）；材质发射
   **透传命令真权重**（批 3a 强拍 `{1,1,1,1}` 的那一行=缺失本体）；仿射材质在
   发射单点归一恒等权重（Weights 是零值字段，直构命令与队列口两条来路都在
   发射处归 1——首版实测抓到：摘强拍未补归一 ⇒ 仿射材质 768 通道差全塌缩，
   发射单点归一后灭）；槽循环**材质分支提前到 Perspective 分支之前**
   （否则材质+透视命令落标准透视腿静默丢材质；canMergeImageDraw 按 Perspective
   分槽现成语义保持，同材质 persp/affine 各自成段喂同一条组合管线）。
5. `context_image_gpu.go tryGPUDrawImage`：图像口 material+投影层放行
   （projW 透传）——**顺修批 3b 存量洞**：projActive 块此前先于材质分支
   return true，文档声明的「材质+投影 ⇒ 返 false」实际不可达，组合被**静默
   画成无材质**（违反非静默纪律的存量洞，本次放行同笔消灭）。
6. `context_damage.go`：`gpuContextOps` 接口声明同笔切换 + doc。

**调用方迁移（同一提交内收齐，树不留半新半旧）**：

- `render/p37_3b_test.go TestP37BMaterialRejections`：material+Perspective 拒绝
  断言撤（画面组合由五钉承管）；图集 material+persp skip+计数断言改为
  「Drawn=1 且不计数」；bicubic/blend 部分不动。
- `render/internal/gpu/p37_3a_material_test.go Guards`：material+perspective
  buildImageResources 改「必须接受」；blend/closed 守卫保持。
- `render/scene/p37_h22_replay_refusal_ledger_test.go`：拒绝场景从「材质+透视」
  迁到「画布投影层眼后」（`render.Matrix4Homography` w(y)=1-0.003y 过零，
  ErrQuadBehindEye）——材质+透视放行后回放腿可拒面收敛到几何族，拒绝账语义
  不变（几何拒同样记账）。
- ⑥ `ui_wr_p37_scene` D 面板：中段拒笔同迁（PushProjection(眼后)+跨眼平面
  quad+材质 → 具名拒不画），拒-画-对照结构不变；probe 9 更名
  `quad_projection_behind_eye_refused`、probe 15 哨兵改 ErrQuadBehindEye。
  **画面零变化实证**：金图未重播种，双后端 native/go 19/19 0.0000%、快照与
  金图 cmp 逐位同（新旧拒笔都是「拒而不画」）。
- ③ `p37_material` 臂 e：现场拒同迁（同几何+眼后投影，四角全过零必拒），
  区域锚 (610,560) 不动；芯片键更名 `quad_behind_eye_refused`；图像口
  bicubic+材质 skip+计数保持（B 批随纹素契约迁移）。双后端 20/20 0.0000%、
  金图未动。

**回归账**：render 包触及文件逐文件——p37_3b/p37_mat_persp/p37_quad_bicubic/
p37_projection/p37_projection_chain/p37_projection_stances/atlas_r4/p37_atlas_persp
全 PASS；`context_misc_draw_gpu_test` 两子测
（DrawColorEffectSurfaceAdvancedBlend/DrawShadow）为**存量红**（git stash 对照
基线同源，非本批带入，如实记）；internal/gpu——image_batch_census/
batch_coalescing/p37_3a_material/opt29 全 PASS。build/vet rc=0。

B 批：bicubic 纹素契约（2026-11-07，同日第二枚）
------------------------------------------------

**拍板口径**：引擎喂纹素——契约前缀第 4 参 `texel: vec4<f32>` =引擎按本笔
采样档采好的纹素（对齐 Godot「TEXTURE 按 item 档位采样」；采样质量归引擎，
用户只管上色）。未声明 texel 的效果自行采样（引擎按命令档绑定采样器，
bicubic 档即 nearest 档），语义如实写进 effect 文档。

**红测先行**（`render/p37_mat_bicubic_test.go` 四钉，红态=A 批树 stash 实测
`matbicubic-red-test.txt`：4 参被 fragmentParams 具名拒「prefix is at most
(uv, tint, hom_w)」+ material+bicubic 被拒）：

| 钉 | 判据 |
|---|---|
| TestP37MatTexelEchoQuadBitwise | **echo 材质（返回引擎纹素原样）≡ 无材质标准路逐位**——bicubic/nearest/linear 三档各一轮（引擎纹素=标准采样的最强证明） |
| TestP37MatTexelEchoOwnBindingsBitwise | 用户自带纹理对声明路：引擎复用用户变量名（不注入），bicubic 档逐位 |
| TestP37MatTexelEchoPerspBitwise | 纹素骑透视正确插值 UV（A 批×B 批组合态），linear 档逐位 |
| TestP37MatTexelEchoSpriteAndImageBitwise | 批绘口与图像口 bicubic echo ≡ 标准路逐位（三口全通） |

**实现**：

1. `render/shader_compose.go`：契约前缀 0..4（第 4 参 texel=vec4）；引擎段
   `gpuiEngTexelSection`——绑定名解析（用户已声明 binding(1)/(2) 对⇒复用
   用户变量名，否则注入保留名 gpui_eng_tex/gpui_eng_samp 并计入绑定面）；
   bicubic 变体在引擎段内嵌 16-tap Catmull-Rom 核（与 textured_quad_bicubic.wgsl
   /CPU SampleBicubic 同常数，采样用命令绑定的 nearest 档）；填充路 texel
   默认 vec4(0)（无纹理可采，语义如实写注释）。params<4 不拼——存量模块
   字节不变。
2. `render/shader_effect.go`：`quadComposedBicubic` 第二变体（eager 组合期
   同验，D-3a-1 口径）；`ComposedQuadWGSLFor(bicubic)`；texel 注入对计入
   绑定面（texturePairs≥1，bgl0/绑定组由此派生）。
3. `render/internal/gpu/material_pipeline.go`：缓存键改 `materialKey{effect,
   bicubic}`（entries/rejected 双 map 同键；evict 双变体同撤；H23 负面缓存
   语义不变——plain 被拒不代表核变体被拒）。
4. 三口放行 + `QueueImageDrawMaterial` 加 bicubic 位（接口全量切换：实现/
   接口/3 调用点同笔）：m4_quadimage 摘 material+bicubic 拒、精灵路摘
   perspSlot+bicubic skip、图像口摘 bicubic 假门（A 批遗留的半截——首版
   echo 钉抓到：image 口 bicubic echo 画成白底=假门还在吞）。
5. 既有钉迁移：`p37_3b_test` material+bicubic 拒绝断言撤（echo 钉承管画面）；
   ③ 臂 e 图像口 skip 探针改 blend+材质（Normal-only 口径不变，skip+计数
   演示保留）。

**边界如实**：材质+透视+bicubic 三合一维持既有拒（`ErrQuadPerspectiveBicubic`
——透视槽无 bicubic 变体，先于材质的边界；材质路虽可实现但 plain 路无对照
锚，归「透视槽 bicubic 变体」另项）。

**回归账**：echo 四钉 + p37_3b/p37_mat_persp/p37_3a/H23 负面缓存钉全绿；
build/vet rc=0、gofmt 净。


专项真窗：ui_wr_p37_mat_persp（2026-11-07，同日第三枚）
------------------------------------------------------

用户拍板的新专项窗（六面板 3×2，满窗即内容，RUN_SECONDS=8 / 240 帧门）：

- **P1 恒等材质+透视 / P3 bicubic 纹素 echo / P4 精灵路 / P5 投影层组合**：
  四对「对照 | 材质」同几何并排逐位（窗内版四钉，diff 全 0）；
- **P2 渐变带**：透视正确插值解析锚双份（离屏+快照）——分界 y=40（仿射 60），
  (80,55) 反拍平锚；
- **P6 拒绝牌**：芯片色由探针事实驱动（眼后现场拒 + blend+材质 skip+计数）；
- 探针 8 枚全绿、gpu_ops=16 有界、语义像素 4 枚；
- 金图口径全走：删基线→播种（播种跑仍判红）→native 复验→go 播种→go 复验→
  稳定轮→两后端基线 cmp **逐位相同**（`matpersp-window-runs.txt` 正本）。

③ 确认轮（臂 e 探针迁移后画面零变化）：双后端 20/20 0.0000%、金图未动。


P6 升级：12 格能力矩阵（2026-11-07，用户「需要有能力窗口」）
------------------------------------------------------------

专项窗 P6 从双芯片升级为 **quad 口 12 组合能力矩阵**（2 透视 × 3 采样档 ×
2 blend × identity 材质）：stance 由 `mpMatrixFacts` 离屏**逐格实测**（每格真调
`DrawImageQuad` 验 err 与出图，不写死）——draw=5 / refuse=7（透视+bicubic=
`ErrQuadPerspectiveBicubic` 透视槽边界先于材质；blend≠Normal 整族=
`ErrQuadUnsupportedBlend`）；窗内绿格每帧真画、红格拒底；探针
`matrix_stance_exact` 逐格哨兵对 + 放行格出图断言 + 快照双色锚；眼后芯片保留
（几何族不在矩阵轴上）。CPU 腿 stance=skip+计数（契约口径注记）。金图口径
重走全链：双后端 0.0000%、基线逐位同、稳定轮绿（`matpersp-window-runs.txt`）。


P5 二修：投影链序的正确用法 + 两条登记（2026-11-07，人工验收三轮）
------------------------------------------------------------------

人工验收（用户肉眼三轮）抓出两个窗侧错误与两条工程登记：

1. **P3 首版误用透视+bicubic**：两格全空（该组合撞先于材质的透视槽边界
   ErrQuadPerspectiveBicubic）。正修=改**仿射** bicubic echo（echo 能力=B 批
   纹素契约，仿射承载）；三合一维持既有拒，归「透视槽 bicubic 变体」另项。
2. **P5 首版面板平移进用户矩阵**：链序拍板 B（device ∘ projection ∘ matrix ∘
   user）下 matrix 在投影**之内**——面板平移的 y 喂进 w（(500+30)/3.07≈173，
   损伤矩形日志实锤）⇒ quad 塌缩成 (124..250,173..185) 的 12px 细条、恰好
   落进 P1 两格区域（探针「内容存在」被细条骗过、pair diff=1568 的真身）。
   **引擎链序无洞**（3D 相机语义正确：面板平移=相机空间移动模型）；正修=
   **面板偏移并进投影矩阵** SetProjection(T∘projM)——T 仿射在 w 除法之后
   生效，本地坐标 w 不被污染；呈现帧两格透视棋盘逐位一致、identity pair
   diff=0、9/9 探针全绿。投影层下的面板放置正解随之立此存照：**偏移走
   投影矩阵合成，不走用户矩阵**。
3. **工程登记 A（Wayland 路窗不可见）**：本会话全部真窗在
   WAYLAND_DISPLAY=wayland-0 面上开出，X 侧（xwininfo/xlsclients）与桌面
   均不可见——人工验收必须 `env -u WAYLAND_DISPLAY` 强制 X11。归
   ui/platform Wayland 路可见性专项（另项）。
4. **工程登记 B（dc.Image() 逐像素读回病）**：`Context.Image()` 每次调用
   做整帧 flush+sync+读回；专项窗 regionBytes 逐像素调它 ⇒ 探针阶段分钟级
   自旋（阶段计时日志实锤，修后 0.5s）。④⑥ 同款 regionBytes 待同修
   （一行提升，登记待办）。

**矩阵 P6 探针防盲区补强**：四对逐位探针补「内容存在」断言（对照格非卡底
像素 ≥ 面积 1/4）——两格都空也 diff=0 的盲区已封（P3 首版两格全空即被
人工肉眼抓到、探针补丁后此类假绿不再可能）。

**金图全链（第三版）**：删基线→播种（播种跑仍判红）→native 复验 9/9
0.0000%→go 播种→go 复验 9/9 0.0000%→稳定轮 0.0000%→两后端基线 cmp
**逐位相同**（`matpersp-window-runs.txt` 三节正本）。
