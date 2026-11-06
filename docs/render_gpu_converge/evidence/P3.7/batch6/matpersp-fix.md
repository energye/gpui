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

B 批：bicubic 纹素契约（待续）
------------------------------
（契约前缀扩展引擎采样纹素输入、组合模块采样段、材质管缓存键、
bicubic+材质三口放行、③臂 e 图像口 skip 探针迁移——开工后补记。）
