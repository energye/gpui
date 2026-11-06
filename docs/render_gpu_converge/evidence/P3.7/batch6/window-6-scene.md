批 6 ⑥ · scene 流收口窗 收口账（2026-11-06；设计见同目录 `windows-3456-design.md` 五，窗 `examples/ui_wr_p37_scene`）
====================================================================================

一、收口对象
------------
台账 745（批 3e-1 scene 4×4 投影口）、746（批 3e-2 quad+权重通道）、
751（批 3e-3 item 级材质编码）三行 ☑→✅。归属修正（plan §5）：这三行 engine
窗收不了（`engine/`、`ui/` 对 `render/scene` 零引用是 3e-2 钉下的结构约束），
本窗放 examples/ 顶层 ui_wr_ 族，全链路走 **Scene 录制 → Encoding → Decoder
→ GPUSceneRenderer.RenderScene 回放**。

二、判据与实测（15 枚探针全绿；容差显式：逐位段 0 / 载荷回环逐值 /
语义像素 ±3..5 / 金图 0）
--------------------------------------------------------------------------------
| 臂 | 断言 | 实测 |
|---|---|---|
| 链路真实 | Encoding 走查（载荷宽度全消费、未登记 tag 判失败，纪律承接 materialPayloadFromStream 钉）：投影 tag Set=1/Push+Pop=2、quad tag=2、材质 tag=[索引,0xFFFFFFFF]；`enc_bytes/enc_tags` 进 JSON | `stream_walk_counts` ✓；enc_bytes proj=205/stack=342/quad=130/material=175，enc_tags 17/26/2/19 |
| 载荷回环 | 投影 mode+16 元素逐值等；quad 角点/权重按 float32 编码口径逐值等（encoding 存 float32，期望=float32 折算——首版拿 float64 原值比被自己的口径错打回一次，如实记）；材质 [0,clear]+注册表指向同一枚效果 | `projection_payload_roundtrip` / `quad_payload_roundtrip` / `material_payload_roundtrip_with_clear` ✓ |
| 回放≡手写（三钉窗内像素版） | 投影/quad/材质三路 replay ≡ 手写 Context 调用**逐位 diff=0**，各带反向鉴别（装与不装 ≥100 像素差）与非空转（红 mask 计数>400） | `replay_projection_equals_handwritten` diff=0、鉴别 14351、窗形红 mask 4210px bbox=(18,72)-(137,112)；`replay_quad_weights_equals_handwritten` diff=0 + 透视开关鉴别；`replay_material_equals_handwritten` diff=0 + 粘滞鉴别 ✓；`replay_stack_equals_handwritten` diff=0（push+pop 两枚 tag） |
| CPU 脏区腿两态（批 3 收尾欠账） | 材质 fill 整笔拒画+MaterialSkips++；材质清除后投影下 image 整笔拒画+ProjectionDegrades++；拒画不裸画（落点无像素） | `cpu_leg_stances` ✓（首版没清材质、image 被 material 门先拦、degrades=0——是探针场景顺序写错，按两态各自开门后成立） |
| 拒绝态 | 透视 quad+粘滞材质 ⇒ 校验期具名拒、不入队、不计数、该笔不画（与 control 逐位同底） | `quad_perspective_material_refused` ✓ |
| 零发射/身份 | 不调三动词或装恒等/零值投影 ⇒ tag 流与 Hash 逐位不变；投影/材质进入 Encoding.Hash（场景身份） | `identity_and_zero_projection_emit_nothing` / `hash_swallows_projection_and_material` ✓ |

三、链序口径（实测反推，非公式反推结论）
----------------------------------------
A1 语义锚首版按 `device=D∘P∘M∘user` 字面序预测落点 (82,117)——实测是卡片底。
一次性探针（离屏 + 包内临时测试两件，跑完已删）实测：**经 Translate 的
deviceMatrix 下投影在设备域生效**，红块 bbox=(18,72)-(136,111)，与「P 作用于
D∘M 烘焙后坐标」的解析值吻合；无 translate 的离屏路 bbox=(12,48)-(145,98) 与
手写 `SetProjection` 逐位同（引擎双腿自洽）。这不是洞——1d 钉的链序在两条
独立路各自自洽、投影进 w 除法的位置由设备矩阵合成路径决定，属既有口径。
窗体语义锚因此改为**与窗体同形（Translate+回放）的离屏全窗输出红 mask bbox
中心**反推（P3.5 教训的落地版：坐标从画面同一份输出反推）。

四、三证据合证
--------------
1. 逻辑/像素离屏探针 12 枚。
2. 语义像素 4 枚：恒等面板红块（证明 scene 回落在呈现帧里，防空帧假绿）、
   投影面板红块（同形 bbox 中心）、材质渐变中点（premul 直出公式：
   src+(card)·(1−a)，③ d9 同口径）、清除后平色块精确刷色。
3. Golden 逐位 0（fail-closed、mask y<32、双后端各锁各基线且复核 0.0000%；
   播种/缺基线判红实测）。两基线 `cmp` **不同**（第 37 字节起）——逐像素工具
   核账：差异 171px 全在 y=10..18 指标行（presents 尾数 240/241 文案），
   **裁到金图比对区 y≥32 后像素逐位 0 差**。这坐实 p36 窗头注释的既有口径
   「两后端金图各锁各的」在本窗同样成立，且比对区本身两端一致（不是采样器
   差异——HUD 不参与金图）。
live-only 口径：scene GPU 回放经 `RequireLiveContext`，生产帧每帧 Recorder 走查
合法计数 `live_only_skips≈presents`（native 复核轮 239、终验轮 240、常驻 357——
只报数不判绿；首版误把 shadermask 窗的 `skips==0` 门当标准判了红，按
`ui_render_base_scene` 既有口径收回，scene 内容在呈现帧由语义锚扛——四锚全过）。

五、H15/H16
-----------
- H15：native 门禁相六轮（播种后复核+复核轮+五轮）金图恒 0.0000%（presents
  240/241 两形态）。静态内容窗：常驻相（358 presents）金图同 0，如实记。
- H16 两变异（窗体场景各删一行发射→跑→还原原位→终验复绿）：
  1. 摘 `sceneProj` 的 `SetProjection(projM)`：五枚探针红（stream counts
     proj=0 / payload 回环红 / 回放≡手写 diff=14351 / 鉴别归零红 / Hash 红）
     + **金图 2.0477%**；首跑因探针 2 的 got 串无守卫索引 **panic 退 2**——
     窗自身缺陷（红的形态应是报红不是崩），补安全访问后重跑取到上述数字，
     如实记；且 pixel_ok=true 属设计内：A1 语义锚从同形回放 bbox 反推，自洽
     跟随画面，咬住变异的是鉴别/回环族探针。
  2. 摘 `sceneMaterial` 的 `SetMaterial(gradEffect)`（sceneRefusal 的那行保留
     ——只咬粘滞臂）：四枚探针红（counts mat=0 / 材质回环空流 / 回放≡手写
     diff=20000+鉴别归零 / 粘滞 shell 红）+ 语义像素红（渐变块回实心
     (219,51,56)≠(172,86,37)）+ **金图 3.3924%**。
- 还原核对：grep 发射行回原位（SetProjection×1 + SetMaterial×2）→
  gofmt/vet 净 → 终验 `OK backend=native presents=241 probes=15/15
  golden=0.0000%`。

六、门禁账
----------
见 `window-6-gates.md`；原始跑数见 `window-6-runs.md`。
实现期自身口径纠偏四处（探针打回窗设计，非引擎洞）：quad 回环 float64/float32
口径、手写对照臂的投影顺序与几何对不齐、材质粘滞漏了图像臂的 opts.Material、
CPU 两态场景没清材质导致 degrade 门被 material 门先拦。

七、收口动作
------------
台账 745/746/751 ☑→✅；§7 追加批 6 ⑥ 行；plan §5 补交付进展。批 6 六扇全收。
