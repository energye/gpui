# 渲染功能覆盖总表（RENDER_FEATURE_COVERAGE）

日期：2026-10-18 ｜ 基线：`feat/框架代码整理` @ `4734888b` ｜ 仓库：`/home/yanghy/app/projects/gogpu/gpui`
性质：渲染域覆盖审计总表（非机器校验门禁文档）。

**v2（2026-10-18）：§1/§2 已逐项按源码复核「是否有对应真窗示例」；新增 §3「缺真窗清单（第一步任务）」；§4/§5 中来自其他文档的部分已显式标注「文档口径（本轮未逐行源码复核）」。**

复核方法（全部源码实测）：
1. **真窗判定**：示例目录源码含 `platform.Open(` / `NewPipelineApp(` / `SpawnWindow(` / `OpenReal(` 记为真窗 → **131 个真窗目录**；`examples/render_*`（17）+ `render_visual_*`（10）+ `text_langs_png` 共 28 个为离屏出 PNG，**不产真窗**。
   复核命令：`for d in $(find examples -name main.go -printf '%h\n' | sort -u); do grep -qE 'platform\.Open\(|NewPipelineApp\(|SpawnWindow\(|OpenReal\(' $d/*.go && echo "WIN $d" || echo "OFF $d"; done`
2. **逐功能扫描**：组合 grep 全 `examples/`（真窗 / 离屏分开标注）；示例零命中的再查 `ui/`、`engine/` 层是否有消费者（即“间接路径”）；最后全仓非测试确认「零调用」。
3. 原始扫描产物：`/tmp/opencode/`（`ex_window.txt` / `ex_other.txt` / `feat_scan_out.txt` / `feat_scan2_out.txt`；脚本 `feat_scan.sh` / `feat_scan2.sh` 可重跑复现）。

## 0. 判定口径

| 判定 | 含义 |
|---|---|
| ✅ | 有真窗直用（某个真窗的示例源码里直接调用该 API） |
| 🔁 | 无示例直调，但有间接路径（ui/ 或 engine/ 层在调用，真窗经该层执行到它） |
| 📄 | 仅离屏出图示例（render_*/text_langs_png 产 PNG，不是真窗） |
| ⛔ | 无任何真窗/示例消费者（仅单测、仅库内自用、或全仓零调用） |

## 1. 渲染主包功能矩阵（v2 源码复核）

| # | 功能族 | 代表 API | 真窗证据（源码） | 判定 | 备注 |
|---|---|---|---|---|---|
| 1 | 形状基元 | DrawRectangle/Circle/Oval/Ellipse/RoundedRectangle/Arc/Line/Points/RegularPolygon | 37 个真窗直用：engine/* 22 窗、ui_render_base_{text,image,cliplayer}、ui_render_graphics/{basic,vector}、ui_l1_render_matrix、ui_wr 多窗 | ✅ | 覆盖最厚 |
| 2 | 路径构建（常用） | MoveTo/LineTo/QuadraticTo/CubicTo/ClosePath/AddRect/AddRRect/AddOval/ArcTo | 17 个真窗直用：engine/{anim,camera,step,world}、ui_render_base_geometry、ui_render_graphics/*、kit_f0_*、ui_textinput_ime、ui_wr_* 等 | ✅ | |
| 3 | 路径低频 | AppendPath/FillPreserve/StrokePreserve/GetCurrentPoint/ClearPath | 间接：ui/rendering/paint_context.go:154-155（ClearPath/AppendPath）；其余三 API 全仓零调用 | 🔁 | 直调真窗 0 |
| 4 | 路径造型 | Trim/WithCorners/Discrete | 无（video_player:1126 命中系 strings.Trim 噪音）；全仓非测试零调用 | ⛔ | |
| 5 | 布尔路径 | BooleanPath/Path.Op | 无消费者；差集测试 2026-10-18 实跑 FAIL：`s3c_m3_residual_gate_test.go:242: difference hole expected light, got 255,0,0` | ⛔ | 逐像素近似，先修后窗 |
| 6 | 填充/描边/线型 | Fill/Stroke/SetLineWidth/LineCap/LineJoin/AntiAlias | 37 个真窗直用（同 #1 组） | ✅ | |
| 7 | 描边预设族 | DefaultStroke/Thin/Thick/Bold/Round/Square/Dashed/Dotted/WithDash* | 无（全仓非测试零调用） | ⛔ | |
| 8 | 渐变（线/径） | FillLinearGradient/FillRadialGradient/New{Linear,Radial}GradientBrush | 直用 6 窗：ui_l1_render_matrix、ui_render_base_geometry、ui_render_graphics/vector、ui_render_jumprope、ui_render_particle、ui_render_pelican（径向 3 窗） | ✅ | |
| 9 | Sweep 渐变 | FillSweepGradient/NewSweepGradientBrush | UI 门面存在（ui/rendering/draw.go:365）；全仓无调用方（除 render/brush_custom.go:246、recording 后端） | ⛔ | 门面未接线 |
| 10 | 画笔/图案 | Paint/ImagePattern/CreateImagePattern/SetFillPattern | 仅离屏 render_images/main.go:144、render_visual_images；ui/engine 零消费者 | 📄 | |
| 11 | 变换栈 | Translate/Scale/Rotate/Shear/Push/Pop | 直用命中 3 窗（pelican/c2/r5_picture）+ ui/engine 层广泛 | ✅ | 主体在 ui/engine 层 |
| 12 | 裁剪主路 | Clip/ClipPreserve/ClipRect/ClipRoundRect/ResetClip | 直用：engine/stage_chase:1377、ui_render_base_cliplayer/scene.go:90（pc.DC.ClipRect）；通用路径 ui/rendering/paint_context.go（r3/c1 等窗经它） | ✅ | |
| 13 | 裁剪运算 | ClipRectOp/ClipPathOp（difference/replace） | 无（render/clip_op.go 仅定义；全仓零调用） | ⛔ | |
| 14 | 掩码写路径 | SetMask/ClearMask/InvertMask/ApplyMask/PushMaskLayer | 用户 API 全仓零调用；内部仅 clip-mask 上传管线用（render/context_clip_mask_gpu.go:48） | ⛔ | |
| 15 | 图层 | PushLayer/PushLayerIsolated/SaveLayer | 直用：c6:157/180/208、r18:89/104/130（pc.SaveLayer）；ui/rendering opacity/filter 间接 | ✅ | |
| 16 | Backdrop 层 | PushBackdropLayer/PushBackdrop | ui 层就绪（ui/rendering/filter_draw.go:86、ui/scene/composite.go:220、ui/scene/build.go:130）；示例中零构建（仅注释提及） | ⛔ | 补窗即验 |
| 17 | 滤镜层（灰/糊） | ApplyGrayscale/ApplyBlur/RenderGrayscale/RenderImageFilter | 直用 r20：main.go:201/215（NewRenderGrayscale/NewRenderImageFilter）、main.go:47 blank import render/filters | ✅ | |
| 18 | 图像主路 | DrawImage/DrawImageRect(Ex)/DrawImageNine/Rounded/Circular | 直用 4 窗：ui_render_base_image、pelican、c2、r5_picture；Nine 直用 base_image/scene.go:199、pelican/scene.go:1233 | ✅ | |
| 19 | 图像透视 | DrawImageQuadEx/QuadDrawOptions | 直用 engine/quad | ✅ | |
| 20 | 死导出 | DrawImageQuad（非 Ex） | 全仓零调用（render/m4_quadimage.go:25 定义） | ⛔ | 删签名或补窗待定 |
| 21 | 图像插值/混合 | Interp*/Blend* | 子集直用：InterpNearest 7 文件、InterpBilinear 6、BlendMultiply 4、BlendScreen 4（engine 多窗）；Bicubic 仅离屏；BlendAdd/BlendOverlay 0 | ✅ | 枚举值未用全 |
| 22 | WebP | LoadWebP | 零消费者 | ⛔ | |
| 23 | 视频直传 | 视频桥 | 直用 17 窗（video/* 共 21 目录；vr0–vr3 等 4 个为解码类窗，不走桥） | ✅ | |
| 24 | 帧/呈现 | BeginFrame/FlushGPU/PresentCount/PresentWith | 全窗经 ui/embedder；自检直读如 pelican:175 | ✅ | |
| 25 | 底层呈现开关 | PresentFrame/SetVsync/MarkFullRedraw/Invalidate | 间接 ui/embedder/pipeline_app.go、ui/scene/textured.go；示例直调零 | 🔁 | |
| 26 | 损伤合并 | CoalesceDamageRects/SetDamageRects | render 内部（frame.go:144/228、context_damage.go:142）；无 ui/示例调用（c8:756 只读统计字段） | ⛔ | 内部机制，窗不可控 |
| 27 | 文本主路 | DrawString/Anchor/SetFont/MeasureString | 直用 17 窗 | ✅ | |
| 28 | 文本 shaping | DrawShapedGlyphs/DrawShapedColorGlyphs | 间接：ui/rendering/{text,text_layout,text_picture}.go、ui/scene/{picture,textured}.go；直调零 | 🔁 | 所有文字窗经它 |
| 29 | 文本描边/沿路径 | StrokeString/TextPath | 全仓零调用 | ⛔ | |
| 30 | 文本低频 | MeasureMultilineString/WordWrap/SetTextMode/SetLCDLayout | 全仓零调用 | ⛔ | |
| 31 | VF 变体 | WithVariations/LoadFontFaceWithVariations/FontVariationAxes | 仅离屏 render_variable_font；消费者仅 render 内部（text_font.go:176、text/multi.go:218/221） | 📄 | |
| 32 | 顶点/网格 | DrawVertices(Ex)/DrawMesh(Ex) | 全仓零调用（render 内部仅 Ex→冻结路径自调用 vertices_mesh.go:359；engine/camera/parallax.go:25 仅注释） | ⛔ | |
| 33 | 图集/深度精灵 | DrawAtlas(Ex)/DrawDepthSprites | 直用 engine 4 窗（particle、particles2d、sprite、stage_chase）+ ui/rendering/image_draw.go 间接 | ✅ | |
| 34 | 抖动 | SetDither | 零调用 | ⛔ | |
| 35 | 显存账本 | VramLiveBytes/VramLiveCount/RecoverSharedDevice | 直用 pelican:242-243；间接 ui/scene/textured.go | ✅ | |
| 36 | 离屏纹理/烘焙 | CreateOffscreenTexture/ImageBufFromImage | 直用 6 窗：kit_f0_content、ui_render_particle、pelican、c8、r7b、r7 | ✅ | |
| 37 | 共享编码器 | CreateSharedEncoder/SubmitSharedEncoder | render 内部（context_layer_pass/context_output/gpu_render_context）；无 ui/示例消费者 | ⛔ | |
| 38 | 软渲染 | SoftwareRenderer/SDFCoverage | render 内部（software*.go、coverage_filler.go 等）；无 ui/engine 消费者 | ⛔ | 兜底实现 |
| 39 | 设备提供者 | SetDeviceProvider/AbandonDevice | render 内部定义（vello_accelerator.go:146、sdf_gpu.go:267）；零外部调用 | ⛔ | |
| 40 | 内容滤镜（CPU/GPU 图） | ApplyInvert/ApplyBlurXY/ApplyImageFilterGraph/GPUFilterTexture | ui/engine 零命中 | ⛔ | |
| 41 | 文本字距 | SetLetterSpacing | ui/rendering/text.go:388-390 定义；全仓零调用（TextAlign/TextBaseline 在 ui 层不存在） | ⛔ | |
| 42 | 杂项绘制 API | DrawShadow/DrawColor/DrawPaint/ClipBounds/DrawParagraph | 每项全仓零调用 | ⛔ | |

## 2. 子包矩阵（v2 源码复核）

| 子包 | 判定 | 真窗证据（源码） |
|---|---|---|
| render/text | 🔁 | 间接 ui/rendering/text_layout.go:813（text.Shape）；直接 import 的示例均为离屏（render_text 族 21 文件）。ShapingCache 零消费者（仅 render/text/cache/{shaping,lru}.go） |
| render/scene | 📄 | 仅离屏（render_scene、render_scene_gpu、visual×2）与 render 内部（internal/gpu、filters）；无 ui/engine 直用 |
| render/recording | 📄 | 唯一消费者 examples/render_recording（离屏） |
| render/filters | ✅ | r20 blank import（main.go:47）+ 滤镜节点路径 |
| render/text/emoji | 🔁 | render/text 内部（color_font_adapter.go 等）+ ui/rendering/text_layout.go；真窗 ui_text_m5_anycase（emoji 分区，main.go:3/130） |
| render/gpu | ✅ | blank import 全窗（注册渲染后端） |
| render/svg | ⛔ | 全仓（非测试）零 import |
| render/surface | ⛔ | 全仓（非测试）零 import |
| render/raster | ⛔ | 无独立入口；仅 render/gpu 内部注册链引用 |

## 3. 缺真窗清单（第一步任务：全功能真窗化）

规则：每个功能至少要有「一扇真窗 + 看得见 + 可断言」。

### 3.1 A 组：功能在、但无任何真窗 —— 直接补窗

- 路径低频三 API（FillPreserve/StrokePreserve/GetCurrentPoint）
- 路径造型（Trim/WithCorners/Discrete）
- 描边预设族（Thin/Thick/Bold/Dashed…）
- Sweep 渐变（UI 门面已存在、无调用方）
- 裁剪运算（difference/replace）
- Mask 写路径族（SetMask/ClearMask/InvertMask/ApplyMask/PushMaskLayer）
- Backdrop 层（ui 代码就绪、无窗构建）
- 文本描边/沿路径（StrokeString/TextPath）
- 文本低频四 API（MeasureMultilineString/WordWrap/TextMode/LCDLayout）
- 文本字距（SetLetterSpacing）
- 顶点/网格（DrawVertices(Ex)/DrawMesh(Ex)）
- Dither；DrawShadow/DrawColor/DrawPaint/ClipBounds/DrawParagraph
- WebP
- 内容滤镜（ApplyInvert/ApplyBlurXY/ApplyImageFilterGraph/GPUFilterTexture）
- 机制类（窗是否单开按需）：共享编码器、软渲染/SDFCoverage、设备提供者、损伤合并、底层呈现开关

### 3.2 B 组：仅离屏出图 —— 需要真窗化

- `render_*`（17）+ `render_visual_*`（10）共 27 个离屏示例本身不产真窗；其中大部分功能在别的真窗有覆盖，但「render 直调层的画面证据」只在离屏（与 §7-U21 金图问题同源），若要求真窗像素证据需窗口化。
- 点名的四个：图案/画笔旧接口（render_images）、VF 加载（render_variable_font）、Recording（render_recording）、Emoji（render_color_emoji；另有间接窗 ui_text_m5_anycase）。

### 3.3 C 组：先修实现/先定案，再建窗

- 布尔路径（差集红测 2026-10-18 实跑 FAIL：`s3c_m3_residual_gate_test.go:242 difference hole expected light, got 255,0,0`）
- clip 差集/绕向、MSAA 中帧 UB、rrect 裁剪恒 1.0、增量呈现断头（见 §6）
- svg / surface / raster 整包：零消费者 —— 先定「接入还是废弃」
- DrawImageQuad 死导出：先定「删签名还是补窗」

## 4. UI 母表「画什么」摘要（文档口径，本轮未逐行源码复核）

来源：`ENGINE_UI_RENDER_BASE`（母表 §2–§16 共 201 行逐行统计）。

| 状态 | 行数 | 说明 |
|---|---|---|
| A（UI 可用） | 78 | + A/B 6、A/C 33 → 「A 系」约 117 |
| B（render 有、UI 未暴露） | 39+2+3=44 | 含 FPath 全族、FT-PAINTER 等 |
| C（半成品） | 17+2=19 | FC-SAVECOUNT、FClip-NEST-DEPTH、FT-HEIGHT(*1.25)、FPC-COLOR-FILTER 等 |
| D（远期/故意不做） | 21+5=26 | PlatformView/conic/编辑器项；§25.3 明说「勿当欠债清零」 |

- 母表无逐行绑窗，只有序级绑窗（§22.1）：geometry/text/image/cliplayer/scroll/perfsoak。
- §22.2 收口残项（非阻塞）：ImageShader、真 Gouraud、conic、多做剪路径、per-corner 圆角图、GPU atlas 产品化、层级 Present 默认化、multi-sliver、VRAM/jank50 指标等。

## 5. R/C 能力与真窗

源码事实（2026-10-18）：
- `examples/ui_wr_*` 45 个目录：44 个有 `main.go`；`ui_wr_c12_spin_over_static` 为空目录；**`ui_wr_r1*` 无目录（R1 真窗从未建）**；`ui_wr_t_stress` 有 `main.go`（不在 §R 表）。
- `examples/engine/` 23 个目录：`stage_night` 仅 `doc.go`（无 `main.go`）。
- `examples/kit/` 9 个 + `kit_f0_*` 7 个。

文档口径（WIDGET_RENDER，本轮未逐行源码复核）：R 能力 25/27 ✅（挂账 R1 窗未建、R15 Golden 次跑待补）；C10 未关（相位分界 Golden 1.728%）；W6 挂 C4/C5 Golden 过期待重定；T 线（线程重设计）立项未开工。

## 6. 半成品 / 非默认路径不可信清单（必须先建窗验证）

| 项 | 现象 | 出处 |
|---|---|---|
| **布尔路径差集** | **2026-10-18 实跑 FAIL：difference hole expected light, got 255,0,0** | `render/s3c_m3_residual_gate_test.go:242` |
| 路径布尔 | 「逐像素近似」、>2048px 静默截断 | RV5:24、CAT §7.2-5 |
| clip 差集/绕向 | even-odd 配对待补 | RV5:31 |
| 4x MSAA | 中帧 LoadOpLoad 读已 Discard 内存（UB） | RV7:40 |
| Auto 管线选择 | 死逻辑（ClipDepth/OverlapFactor 生产不填） | RV7:73 |
| rrect 图片/文字裁剪 | textured_quad.wgsl `rrect_clip_coverage` 恒 1.0（静默失效） | RV7:62 |
| OS 级增量呈现链 | PresentWithDamage 丢参数、SetDamageRects 无生产实现者（断头） | RV7:30 |
| LayerCache | 死代码 | RV3:17、FINAL_SUMMARY |
| cullGlyphs 文本裁剪 | 条件 len>200 恒 false——从未生效 | B:557 |
| F09 compositor-only | 断头路 | RV3:22 |
| 纯位移 blit 不报 damage | | RV3:38 |
| render/scene CPU 剖面 | 丢图层 alpha、Hash 不含 brushes、圆角矩形绕裁剪 | RV4:40-44 |
| 变量字体 GPU | variations 丢失 | RV6:38 |
| 字体缓存 key 冲突族 | | RV6:89 |

## 7. 证据体系缺口（“正确渲染”这条轴）

1. **U21 Golden 逐位规范零落地**：review 记录「27 个真窗全为逻辑指标单证」（FINAL_SUMMARY:149、REPAIR_PLAN:78）。
2. GPU vs SoftwareRenderer 像素差对照未纳入 wr 真窗门禁族（RV1:31）。
3. 渲染诊断严格档默认 Skip（clipping/scene/images/basic_gpu_visual，GC 05:7/30）；文本变换无真金图（GC 05:31）。
4. 金图现状两口径待并表：A2 金图资产存在（未带进本树）vs review「逻辑单证」——需按最新真源复核。
5. CAT §7.2 表内有 6 处已知自相矛盾/过期（DrawShapedGlyphs、视频行、OOMExit、VideoTexturePool、行号漂移、SetDeviceProvider），销账前需重写。
6. **`render_*` 家族 27 个示例不产真窗**（离屏 PNG）——「render 直调层」的画面证据全在离屏金图（本表 v2 新发现）。

## 8. CPU / 内存轴现状

- **CPU**：目前只有 spin_min 一条 A/B 线（本树实测：基线中位 15.90% → G1+G10 后 14.59%；原线终态约 3.4~3.5%，尺子 glxgears ≈2.3~3.0%）。组件窗、2.5D 窗的 CPU 未测。
- **内存（RSS/显存）**：只有零散实测（原线：启动爬坡 ~70MB 后 0.3~0.5MB/s、GOGC 实验），未进任何门禁；资源生命周期纪律在 `ENGINE_GPU_RESOURCE_LIFECYCLE.md`。
- 重放计划门禁（`RENDER_CPU_1PCT_REPLAY_PLAN.md` §4）含画面/守卫/性能，不含内存项。

## 9. 待人工确认项

1. `render_color_emoji` 口径（绕开 render 绘制栈；已被 ui_text_m5_anycase 间接覆盖）。
2. `rendering.DrawVertices/FillSweepGradient` 门面：算「有生产接」还是「门面未接线」。
3. `ui_wr_c12_spin_over_static`（空目录）/`engine/stage_night`（仅 doc.go）按真源是否已排期。
4. `DrawImageQuad`（非 Ex）死导出：记入未接线还是删签名。
5. 视频底层 API 口径（经桥用算不算接线）。
6. `render/svg`、`render/surface`、`render/raster`：长期未接线 vs 已废弃。
7. Bicubic GPU 化（RENDER_2D_ALIGNMENT 洞2）修复状态需复核。

## 10. 建议销账顺序

1. **全功能真窗化（第一步，清单见 §3）**：先 A 组（无窗功能），再 B 组（离屏家族窗口化），C 组先修后窗。
2. **补金图**：U21 逐位落地（先挑 10 窗 + 重灾区每功能 1 窗），GPU/CPU 像素差对照进门禁。
3. **非默认开关矩阵**：MSAA/管线模式/插值/裁剪开关跑一遍开关矩阵真窗。
4. **资源账**：每功能代表窗补 CPU+内存（RSS/显存快照）数字，先组件窗与 2.5D 窗。
5. **与重放计划衔接**：G3–G13 每刀按「正确窗 + 组件/2.5D 对照窗」双窗 A/B；产物回写各域总账。

## 11. 修订表

| 日期 | 内容 |
|---|---|
| 2026-10-18 | v1 落档：三路只读审计合成（基线 `4734888b`） |
| 2026-10-18 | **v2：§1/§2 按源码逐项复核真窗覆盖（131 真窗 / 28 离屏）；新增 §3 缺真窗清单；布尔差集红测实跑确认 FAIL；发现 `render_*` 家族不产真窗** |
