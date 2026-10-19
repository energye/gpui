# 渲染功能覆盖总表（RENDER_FEATURE_COVERAGE）

日期：2026-10-18 ｜ 基线：`feat/框架代码整理` @ `4734888b` ｜ 仓库：`/home/yanghy/app/projects/gogpu/gpui`
性质：渲染域覆盖审计总表（非机器校验门禁文档）。

**v2（2026-10-18）：§1/§2 已逐项按源码复核「是否有对应真窗示例」；新增 §3「缺真窗清单（第一步任务）」；§4/§5 中来自其他文档的部分已显式标注「文档口径（本轮未逐行源码复核）」。**

复核方法（全部源码实测）：
1. **真窗判定**：示例目录源码含 `platform.Open(` / `NewPipelineApp(` / `SpawnWindow(` / `OpenReal(` 记为真窗 → **131 个真窗目录**（2026-10-19 复核：**136**，新增 `ui_render_base_pathlow` / `ui_render_base_pathshape` / `ui_render_base_strokepreset` / `ui_render_base_sweepgradient` / `ui_render_base_clipop`）；`examples/render_*`（17）+ `render_visual_*`（10）+ `text_langs_png` 共 28 个为离屏出 PNG，**不产真窗**。
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
| 3 | 路径低频 | AppendPath/FillPreserve/StrokePreserve/GetCurrentPoint/ClearPath | 直调真窗 `ui_render_base_pathlow`（FillPreserve/StrokePreserve/GetCurrentPoint 三 API，2026-10-19）；间接：ui/rendering/paint_context.go:154-155（ClearPath/AppendPath） | ✅ | 2026-10-19 建窗，A 组第 1 项关闭 |
| 4 | 路径造型 | Trim/WithCorners/Discrete | 直调真窗 `ui_render_base_pathshape`（三 API，2026-10-19）；同批修 Discrete 恒单侧偏移 + 虚线周期 2×（对齐注释/Skia） | ✅ | 2026-10-19 建窗，A 组第 2 项关闭 |
| 5 | 布尔路径 | BooleanPath/Path.Op | 无消费者（仅单测：`s3c_m3_residual_gate_test` 门禁 + `path_boolean_test` 值级）；差集门禁 2026-10-19 绿 | ⛔ | 2026-10-19 已重写为 Clipper 扫描线（精确几何 / 2 轮廓 10 verbs / 无截断），待建真窗 |
| 6 | 填充/描边/线型 | Fill/Stroke/SetLineWidth/LineCap/LineJoin/AntiAlias | 37 个真窗直用（同 #1 组） | ✅ | |
| 7 | 描边预设族 | DefaultStroke/Thin/Thick/Bold/Round/Square/Dashed/Dotted/WithDash* | 无（全仓非测试零调用） | ✅ | 2026-10-19 建窗 `ui_render_base_strokepreset`，A 组第 3 项关闭（原子验证）；同批修 miter 拐角一个转向的尖点符号错，2 项已知限界记 §6 |
| 8 | 渐变（线/径） | FillLinearGradient/FillRadialGradient/New{Linear,Radial}GradientBrush | 直用 6 窗：ui_l1_render_matrix、ui_render_base_geometry、ui_render_graphics/vector、ui_render_jumprope、ui_render_particle、ui_render_pelican（径向 3 窗） | ✅ | |
| 9 | Sweep 渐变 | FillSweepGradient/NewSweepGradientBrush | 直调真窗 `ui_render_base_sweepgradient`（2026-10-19：7 色标整圈 / 门面矩形 / 起始角+负向整圈 / 局部 0→π/2 × Pad·Repeat·Reflect；同批修 shader 非 Pad Extend 部分扫掠被钳成 Pad） | ✅ | 2026-10-19 建窗，A 组第 4 项关闭（原子验证）；门面生产接线仍为零 |
| 10 | 画笔/图案 | Paint/ImagePattern/CreateImagePattern/SetFillPattern | 仅离屏 render_images/main.go:144、render_visual_images；ui/engine 零消费者 | 📄 | |
| 11 | 变换栈 | Translate/Scale/Rotate/Shear/Push/Pop | 直用命中 3 窗（pelican/c2/r5_picture）+ ui/engine 层广泛 | ✅ | 主体在 ui/engine 层 |
| 12 | 裁剪主路 | Clip/ClipPreserve/ClipRect/ClipRoundRect/ResetClip | 直用：engine/stage_chase:1377、ui_render_base_cliplayer/scene.go:90（pc.DC.ClipRect）；通用路径 ui/rendering/paint_context.go（r3/c1 等窗经它） | ✅ | |
| 13 | 裁剪运算 | ClipRectOp/ClipPathOp（difference/replace） | 直调真窗 `ui_render_base_clipop`（2026-10-19：ClipRectOp/ClipPathOp 的 difference/replace + ClipRect/ResetClip/SetFillRule；同批修多子路径 mask 损坏 / fill rule 恒 even-odd / 矩形差集无 AA / mask 覆盖被应用两次） | ✅ | 2026-10-19 建窗，A 组第 5 项关闭（原子验证） |
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

- ~~路径低频三 API（FillPreserve/StrokePreserve/GetCurrentPoint）~~ → 2026-10-19 已建真窗 `examples/ui_render_base_pathlow`（7 像素探针 + 12 逻辑探针 + golden 627200px 逐位零差）
- ~~路径造型（Trim/WithCorners/Discrete）~~ → 2026-10-19 已建真窗 `examples/ui_render_base_pathshape`（11 像素探针 + 14 逻辑探针 + golden 627200px 逐位零差；同批修 `Discrete` 恒单侧偏移与 2× 虚线周期）
- ~~描边预设族（Thin/Thick/Bold/Dashed…）~~ → 2026-10-19 已建真窗 `examples/ui_render_base_strokepreset`（28 像素探针 + 12 逻辑探针 + golden 627200px 逐位零差；同批修 miter 拐角一个转向的尖点算错，2 项已知限界记 §6 与本窗 README）
- ~~Sweep 渐变（UI 门面已存在、无调用方）~~ → 2026-10-19 已建真窗 `examples/ui_render_base_sweepgradient`（22 像素探针 + 14 逻辑探针 + golden 627200px 逐位零差；同批修 shader 非 Pad Extend 在部分扫掠下被钳成 Pad，1 项性能子项记 §6 与本窗 README）
- ~~裁剪运算（difference/replace）~~ → 2026-10-19 已建真窗 `examples/ui_render_base_clipop`（23 像素探针 + 14 逻辑探针 + golden 627200px 逐位零差；同批修四个缺陷——多子路径 mask 损坏 / 裁剪 fill rule 恒 even-odd（含 GPU 深度裁剪 fillRule 透传）/ 矩形差集无 AA / mask 覆盖被应用两次（GPU 走图与 CPU 回退两处根因），2 项已知限界记 §6 与本窗 README）
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

- 布尔路径（差集红测 2026-10-18 实跑 FAIL：`s3c_m3_residual_gate_test.go:242 difference hole expected light, got 255,0,0`；2026-10-19 定位为 CPU tile 栅格器缺陷（非布尔实现）并修复，该红测已转绿 — 见 §11 修订表）——**仍待建窗**：「先重写布尔几何再建窗」已于 2026-10-19 完成（Clipper 扫描线：精确几何、2 轮廓/10 verbs、op 48.3ms→9µs、无 2048 截断），实现侧已具备建窗条件，列入下一步
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
| **布尔路径差集** | ~~2026-10-18 实跑 FAIL：difference hole expected light, got 255,0,0~~ → 2026-10-19 定位为 CPU tile 栅格器缺陷并修复（门禁绿） | `render/s3c_m3_residual_gate_test.go:242` |
| 路径布尔 | ~~「逐像素近似」、>2048px 静默截断~~ → 2026-10-19 重写为 Clipper 扫描线（精确几何 / 2 轮廓 10 verbs / 无截断） | RV5:24、CAT §7.2-5 |
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
| **sub-1px 描边被钳到 1px**（2026-10-19 记录为子项） | `render/software_stroke.go:49-51` 把 <1px 线宽钳到 1.0（注释 "Minimum 1px stroke for visibility"）→ `Thin()`（0.5px）实渲成满不透明 1px 线（实测中心纯琥珀 100%）；展开器本身 0.5 宽正确（band=[45.25,45.75]），钳位在 `SoftwareRenderer.Stroke`；Skia 不钳：0.5px 应渲 ~50% alpha 的 1px 线 | 2026-10-19 `ui_render_base_strokepreset` A 卡实测 |
| **窄线边下 ~1/3 拖尾**（2026-10-19 记录为子项） | 0.5px（钳后 1px）与 1px 线的正下一行有 ~33% 墨色混合（实测 `A_thin_clear`≈0.36/0.33/0.33、`A_w1_clear`≈0.33），3px/5px 线同位置纯背景；机制未定因，窗内两探针按「基本纯背景」声明限界 0.5 | 同上 |
| **AA 矩形 sweep/radial 每帧 CPU 重建 ColorAt 场**（2026-10-19 记录为性能子项） | `brush_native.go:155` 对 AA 矩形优先 `fillRadialSweepFieldNative`（:593）→ `fillColorAtFieldNative`（:614），每帧在 CPU 重建 cap 512² 2D 场（`make`+逐 texel `ColorAt` 循环；本窗 B 卡 382×164=62648 texel≈20ms/帧 + 250KB/帧分配）。实测（性能实验，已还原）：全场景 41fps/cpu_raster 105% vs 仅圆卡 60fps/25.8% vs 仅矩形卡 43fps/105%；同面积圆 r=140 仍 60fps；直调画刷矩形（不经门面）42.7fps → 元凶为场路径。radial 同样中招；圆/非矩形走 1D ramp textured cover 很便宜 | 2026-10-19 `ui_render_base_sweepgradient` B 卡 + 本窗 README「已知限界」 |
| **GPU 深度裁剪 even-odd 限界（sampleCount>1）**（2026-10-19 记录为子项） | `render/internal/gpu/depth_clip.go` 的 stencil 路径恒按 winding 光栅化；sampleCount==1 时有 R8 mask 相交（even-odd 正确），sampleCount>1（MSAA）时无 mask 可交 → even-odd 裁剪路径在该模式下退化为 NonZero（语义差异仅影响 winding≥2 的嵌套同向路径区域） | 2026-10-19 `ui_render_base_clipop` 实现限界 |
| **分数坐标 intersect 矩形裁剪仍走 scissor**（2026-10-19 记录为子项） | `ClipRectOp(Intersect)` 的轴对齐矩形裁剪走硬件 scissor（按整数像素边界取整），分数坐标不产生 AA 边；difference/replace 走 mask 有解析 AA（本窗 A 卡实测） | 同上 |

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
2. `rendering.DrawVertices/FillSweepGradient` 门面：DrawVertices 仍全仓零调用；FillSweepGradient 2026-10-19 起有真窗直调（`ui_render_base_sweepgradient`，原子验证）但仍无生产调用方 → 口径=「门面未接线」（AGENTS.md 生产/原子验证口径：真窗示例只算原子验证），生产接线待后续。
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
| 2026-10-19 | **CPU tile 栅格器修复（引擎内部，不开真窗；验收=探针+值级金样+逐文件回归+性能门禁+布尔门禁）**：三处根因 = ①`NewLineSegment` 归一化取反破坏方向语义 ②Sparse 4x4 winding 公式错 ③flatten 丢水平边导致 Tile16（vello 移植）缺承重 winding 载体。修法：方向符号统一对齐 vello（上+1/下-1，`down=-1`）、Sparse `CalculateBackdrop` 改 per-pixel-row、fine 反导数精确积分重写（`accumulateSegmentWinding`/`windingCoverage`）、flatten/`AddLine` 保留水平边、coarse 跳过近水平段（`coarseHorizontalEpsilon`，挡布尔输出的 1e-7 斜率碎片，条目 61730→1600）、`visited` 改复用位图。性能优化两轮：① backdrop 按形状 Y 跨度裁剪+缓冲复用（1920×1080 小形状 0.44→0.035ms、数组 518400→28800；全屏 1.71→1.07ms）② Tile16 `fillPath` 不变量提升+按跨度行区间裁剪+常量行拆分与连续平扫 ③ Tile16 `pathTilingMain` 复用 `pathCountMain` 的 DDA 参数（vello 因 GPU 阶段隔离才在两段各算一遍，CPU 侧按线段缓存；tiling 1.57→0.53ms/帧）④ `Rasterize` 去冗余清零 + 结果拷贝提界。等价性：参考实现（HEAD 版 tiling/fillPath）逐位比对通过——tiling 的 tiles/segCounts/segments、fillPath 合成扫描 23322 段 × 2 填充规则 + 管线真实产出批次。产物：`render/rasterizer_tile_value_test.go`（Analytic 金样容差 8/255 + 手算精确面积像素：菱形 43/229、分数矩形 191/96）、`render/rasterizer_tile_bench_test.go`（20 项基准 = 5 形状 × 4 模式，含 FrameClean「同形状干净输入」对照）。结果：六例四路全对；`TestS3c_M3_PathBooleanDifference` 转绿；布尔差集形状在 Tile16 / Sparse / 干净同形状三方**逐位一致**（maxDiff 0/255）。性能对照 HEAD（3 轮两树交替取中位）：Tile16 Rect 0.94x / Diamond 0.75x / FrameClean 0.65x / ManyRects 0.86x / BooleanDiff 1.16x——该输入 HEAD 渲染错误（230400px 实心，孔洞丢失；正确 128000px），同形状干净输入 Tile16 为 0.65x，差额=1600 条水平边 + 1600 条短竖边的固有处理成本（输入碎片化由布尔几何重写消解）；Sparse BooleanDiff 0.62x / FrameClean 0.57x / ManyRects 0.83x / 简单形状 1.00–1.07x；Auto BooleanDiff 0.68x / ManyRects 0.79x；Analytic 全线 0.98–1.02x（未触及）。B/op：Sparse 线回落（BooleanDiff 540→249KB、FrameClean 407→132KB、Diamond 175→98KB）；Tile16 病态项 1.39→2.30MB（正确多处理 1600 条水平边的 tile segments + 每线段 DDA 缓存 230KB/次）。沉淀子项：`TileGrid` map+逐 tile 分配（简单矩形填充约 1 万 allocs/次）待优化、布尔几何扫描线重写（下一步） |
| 2026-10-19 | **布尔几何重写（引擎内部实现，不开真窗；验收=值级测试 + s3c 门禁 + 栅格器基准确认病态消失）**：`BooleanPath`/`Path.Op` 由「逐像素采样拼矩形」改为 **Clipper 扫描线（Vatti）** —— 新增依赖 `github.com/ctessum/go.clipper v0.1.2`（BSL-1.0、纯 Go、运行期零传递依赖，`geom` 仅其测试用）；适配层 `render/path_boolean.go`：曲线按 0.1px 展平（复用 `flattenQuad`/`flattenCubic`）→ 坐标 ×1024 定标进 Clipper（NonZero/NonZero，语义与旧实现一致）→ 轮廓回写为 MoveTo/LineTo/Close；坐标钳位 ±1e6px，删掉 2048px 静默截断。结果：方框减孔 **800 轮廓/4000 verbs → 2 轮廓/10 verbs**；分数坐标交点精确（菱形∩框 5 顶点：5/5.75/15 等，旧实现量化到像素网格）；输出方向同时满足 NonZero 与 EvenOdd 渲染（嵌套带/孔/岛逐像素一致）。速度（同基准、HEAD 基线树 vs 当前树）：方框差集 48.3–49.8ms → **9.0–11.8µs**（约 4500×，内存 191KB → 3.7KB）；两圆并集 379–385ms → **100–141µs**（约 3000×）。语义覆盖（库对照实验钉死）：重叠子路径=并集面积、反向内环=挖孔、同心同向=实心、自相交蝴蝶结=两瓣都填、共边自动合并、退化/NaN/超大坐标不崩。产物 `render/path_boolean_test.go`（8 组值级测试 + 2 项基准）；`TestS3c_M3_PathBooleanDifference/Intersect` 绿；`TestRasterizerValueMatchesAnalytic/差集(布尔产物)` 两路 maxDiff 0。栅格器 20 项基准确认**病态消失**：Tile16 `BooleanDiff` 与同形状 `FrameClean` 由 1.16x/2.30MB 变为同输入同分配（4.22–4.43ms vs 4.00–4.24ms，均 11 allocs / 1.07MB）；Sparse 两项亦同（132KB / 4114 allocs）。取舍：布尔结果为折线（曲线按 0.1px 展平），Skia pathops 保曲线属另一量级工作，已记录。**真窗仍待建**（本项只关实现，不标 ✅） |
| 2026-10-19 | **A 组第 1 项建真窗：`examples/ui_render_base_pathlow`（路径低频三 API）**：`FillPreserve`/`StrokePreserve`/`GetCurrentPoint` 此前全仓非测试零调用、无直调真窗（§1 #3 记「🔁 直调真窗 0」）。窗=1200×800 四卡：A `FillPreserve` 填+描同一条路径（青填橙边同现）/ B `StrokePreserve` 描后叠加填（外半描边留品红环）/ C 每步 `GetCurrentPoint()` 读回并按**返回值**画黄点 / D `Fill` 消费路径后 `Stroke` 空画（`D_absent` 采样证明"该有描边处是背景"）。三证据：①逻辑探针 12 项（路径状态跨操作 7 项 + 当前点逐位相等 5 项；比对先按活矩阵 `GetTransform()` 变换到设备空间、无容差，3 连跑 cp 1806/1806 与 1812/1812 全对）②像素 7 探针（期望值直读调色板常量，容差 8/255）③golden 表头+图例+四卡 627200px 逐位零容差（第 2/3 跑 `diff=0.0000%`）。门禁：`-auto-only`/RUN_SECONDS（≥5s）+ §2.2 A–J JSON（`present_policy=full_paint`、`vsync_source` 非空、`cpu_fallback_ops=0`）+ 人工常驻窗（事件打日志并改标题）。窗归 `ui_render_base_*` 家族（ENGINE_UI_RENDER_BASE §22.1 窗测轴，已登记 §23 表）；真源同步：§1 #3 判定 🔁→✅、§3.1 该项关闭、真窗目录计数 131→132（2026-10-19 复核）。**性质=原子验证**（直调 render 接口一次验一族），不算生产接线 |
| 2026-10-19 | **A 组第 2 项建真窗：`examples/ui_render_base_pathshape`（路径造型三 API）+ 同批修 `Discrete` 与自身注释不符**：`Trim`/`WithCorners`/`Discrete` 此前全仓非测试零调用、无直调真窗（§1 #4 记 ⛔）。侦察先取证：探针实测水平线 `Discrete(22,7)` 的 14 个顶点 y 全为 −7 → 注释「±deviation (deterministic)」的交替从不发生（取号用 `step%2`，而 `emitOn`/`step` 每 chunk 同步翻转 → 画出的 chunk 永远奇数 step → 恒单侧平移）；虚线实测「画 22、空 22」（周期 2×segLen），注释却写「~segLen/2 drawn then gap」（周期 = segLen）。经用户确认按注释 + Skia 按 `segLength` 切段口径修正：chunk 改 segLen/2（前半画后半空）、偏移号改按「已画 chunk 序号」交替；新增 `render/pathops_subpath_test.go`（三 API 值级契约：两侧交替/周期占空/端点夹紧），回归 `TestP1_Capability_E02_PathEffectsGPU`、`TestP1_U1_VectorDesignToolDensity` 绿。窗=1200×800 四卡：A 菱形整条淡描 + `Trim(0.25,0.6)` 保留段亮橙 + 两切点 amber 圆点 / B L 形尖角版暗紫打底 + `WithCorners(20)` 青色覆盖（每角留月牙）/ C 三次曲线淡底 + `Discrete(22,0)` 亮绿（11 画/11 空）/ D 同曲线 + `Discrete(22,7)` 洋红（两侧交替 ±7）。三证据：①逻辑探针 14 项（三 API 值级契约 + 窗内曲线 drawn duty/两侧性）②像素 11 探针（保留段/被切段/切点、角部月牙/圆角内/直边、落笔段/空档、±7 段/同弧镜像=卡背景；容差 8/255）③golden 表头+图例+四卡 627200px 逐位零容差（第 2/3 跑 `diff=0.0000%`）。门禁：`-auto-only`/RUN_SECONDS（≥5s）+ §2.2 A–J JSON（`present_policy=full_paint`、`vsync_source=true`、`cpu_fallback_ops=0`）+ 人工常驻窗实屏截图确认上屏（HUD paint=715/presents=716）。真源同步：§1 #4 ⛔→✅、§3.1 该项关闭、真窗目录计数 132→133；`ENGINE_UI_RENDER_BASE.md` §23 登记、`RENDER_API_CATALOG.md` §2 描述措辞（「随机点化」→ 确定性 ± 交替切段）、`ENGINE_UI_WIDGET_RENDER.md` §10 本行。**性质=原子验证**（直调 render 接口一次验一族），不算生产接线 |
| 2026-10-19 | **A 组第 3 项建真窗：`examples/ui_render_base_strokepreset`（描边预设族 8 预设 + `SetStroke`）+ 同批修 miter 拐角符号 bug**：8 预设此前全仓非测试零调用、无直调真窗（§1 #7 记 ⛔）。侦察先取证：临时探针打印展开轮廓，折线 `(60.5,40.5)→(60.5,140.5)→(170.5,140.5)`（下→右转向）width 9 的 miter 尖点吐 `(65,145)`——应为两条外偏移线 `x=56` 与 `y=145` 的交点 `(56,145)`；真窗像素探针同位置读到纯背景 + 邻格 81% 覆盖（两者互证）。根因 `render/internal/stroke/expander.go:applyOuterMiterJoin` 的 `h := ab.Cross(Δ)/crossAbs` 分母用无符号 cross、丢转向符号（两转向只对一边）；按 Skia「带符号 cross 求两偏移线交点」口径改 `t := Δ.Cross(cd)/ab.Cross(cd)`、`miterPt := fpLast + ab*t`（`ab`/`cd` 尺度自动抵消；公开签名未变，另一转向输出逐点不变）。新增 `render/internal/stroke/expander_test.go:TestStrokeExpander_MiterJoinTipBothDirections`（正反转向各断言「尖点出现 + 镜像点不出现」；修前红→修后绿）。连带：`ui_render_base_pathshape` golden 因修复 0.0002%（1px）变化 → 同批重播并连跑两次 `diff=0`；`ui_render_base_pathlow` 不受影响（`diff=0`）。窗=1200×800 四卡：A 四档宽度 0.5/1/3/5（含边下背景行）/ B 三种 Cap(w9) / C miter 两转向 + round 拐角(w9) / D `DashedStroke(8,5)` + `DottedStroke()` 点线。三证据：①逻辑探针 12 项（8 预设值级 + dash 链语义）②像素 28 项（容差 8/255；miter 尖角修复后由「纯背景/81%」变 100% 满色）③golden 表头+图例+四卡 627200px 逐位零差（第 2/3 跑 `diff=0.0000%`）。门禁：`-auto-only`/RUN_SECONDS（≥5s）+ §2.2 族 A–J JSON + 人工窗实屏截图（同批补 `-manual-seconds` 与真事件日志/标题，满足「自测+人工二合一」）。真源同步：§1 #7 ⛔→✅、§3.1 该项关闭、§6 新增 2 行已知限界子项（sub-1px 钳宽、窄线边下 ~1/3 拖尾）、真窗目录计数 133→134；`ENGINE_UI_RENDER_BASE.md` §23 登记 + §24 修订 1.34、`ENGINE_UI_WIDGET_RENDER.md` §10 本行；`render/stroke.go` `DottedStroke` 注释订正（「1, 2 pattern with 2px width」→ 0.1/4 dash + 圆帽成 2px 点/2px 空）。**性质=原子验证**（直调 render 接口一次验一族），不算生产接线 |
| 2026-10-19 | **A 组第 4 项建真窗：`examples/ui_render_base_sweepgradient`（Sweep 渐变族 + `FillSweepGradient` 门面）+ 同批修 shader 非 Pad Extend 部分扫掠被钳成 Pad**：`NewSweepGradientBrush`/`SetEndAngle`/`SetExtend`/`ColorAt` + 门面（ui/rendering/draw.go:365）此前无真窗（§1 #9 记 ⛔）。侦察先取证：D 卡 Repeat/Reflect 圆在范围外（112.5°/157.5°）实测与 Pad 相同（钳到末端蓝），而 CPU oracle（同参数 `ColorAt`）给出 frac/triangle 值——画算不一致；永久测试复现 `repeat@112.5°` 读 (5,0,254) vs 模型 (99,0,240)。根因：GPU 1D ramp 覆盖路径（`fillSweepGradientFieldMasked`）的 `tMin/tMax` 角域收窄对所有 Extend 生效，shader `(t-tMin)*invSpan` 再钳 [0,1] → 范围外 raw t 永远落 ramp 边缘 = Pad 行为。修法（Skia tile-on-t 口径）：非 Pad 时 ramp 固定整周期 [0,1]，cover FS 对 raw t 折叠（repeat `t-floor(t)`、reflect 三角波）；Pad 保留原角域收窄、整圈折叠恒等 → Pad 与整圈输出逐位不变；linear/radial 调用点补 extend=0 不受影响。改动：`brush_fill.go` / `session_textured_cover.go` / `render_session.go`（`TexExtend`）+ 三份 WGSL（`shaders/cover_textured_linear.wgsl`、`textured_stencil_linear.go`、`linear_ramp_mask.go` 均加 `sweep_tile()`）。新增永久回归 `render/p1_capability_matrix_closers_test.go:TestP1_Capability_D03_SweepGradientTileExtendGPU`（GPU 判别采样 tol 12/255）。窗=1200×800 四卡：A 7 色标整圈色轮（起点东）/ B 门面矩形红→蓝（缝避开探针）/ C 起始角 π/2（绿→红）+ 负向整圈 −2π（红→蓝）/ D 局部 0→π/2 × Pad·Repeat·Reflect 三圆（范围外 112.5°/157.5° 六探针=修复回归钉）。三证据：①逻辑探针 14 项（独立 sRGB 数学：传输函数+线性光混合，不调 render 工具）②像素 22 探针（oracle=同参数 CPU 画刷 `ColorAt` 在像素中心 (px+0.5,py+0.5)，tol 12/255 + 独立通道带）③golden 表头+图例+四卡 627200px 逐位零容差（seed 后第 2/3/4 跑 `diff=0.0000%`）。门禁：`-auto-only`/RUN_SECONDS（≥5s）+ §2.2 A–J JSON（`present_policy=full_paint`、`vsync_source=true`、`cpu_fallback_ops=0`、`gpu_fallbacks=0`）+ 人工常驻窗实屏截图（标题随事件 `event #1431`、HUD paint=1431 增长；`close-requested` 关窗汇总 presents=3651）。同批记录 1 项性能子项（§6）：AA 矩形 sweep/radial 每帧 CPU 重建 ColorAt 场（`brush_native.go:155→593→614`，62648 texel≈20ms/帧+250KB/帧；仅圆卡 60fps/25.8% vs 仅矩形卡 43fps/105%）——修它超本项口径，留后续。真源同步：§1 #9 ⛔→✅、§3.1 该项关闭、§9 第 2 项口径定为「门面未接线（原子验证）」、真窗目录计数 134→135；`ENGINE_UI_RENDER_BASE.md` §23 登记 + §24 修订 1.35。**性质=原子验证**（直调 render 接口+门面各一次验一族），不算生产接线 |
| 2026-10-19 | **A 组第 5 项建真窗：`examples/ui_render_base_clipop`（裁剪运算 difference/replace）+ 同批修四个缺陷**：`ClipRectOp`/`ClipPathOp` 的 difference/replace 此前全仓非测试零调用（§1 #13 记 ⛔）。侦察三路复现（CPU/GPU 探针程序）确认：① 多子路径裁剪 mask 损坏（同向嵌套路径差集应实心扣除、实吐环状——栅格器把每个子路径当独立 mask 叠加）② 裁剪路径 fill rule 恒 even-odd（`Clip()`/`ClipPathOp` 不吃 `SetFillRule`，NonZero 嵌套同向路径中心被误挖）③ 矩形差集整数取整无 AA（分数边 hard edge，Skia 应给解析部分覆盖）。修法：重写 `render/internal/clip/mask.go` 多子路径联合光栅化（winding 累积 + even-odd 解析覆盖）、`Clip()/ClipPreserve()/ClipPathOp` 透传 `c.paint.FillRule`（新增 `Context.gpuClipFillRule`，`PathClipAware.SetClipPath(path, fillRule)` 签名同批原子切换；GPU 深度裁剪 fill rule 一并透传——sampleCount==1 的 R8 mask 按 even-odd 光栅化，与 winding stencil 相交后语义正确）、`render/internal/clip/difference.go` 矩形差集改解析面积覆盖。第四个缺陷在验收实测中暴露：分数覆盖被**应用两次**（值呈平方 64→16、191→112），两处独立根因——GPU 走图路线 `fillMaskedAsImage` 把 `ClipCoverage`（同一张裁剪 mask）烘焙进 src 后被 R8 mask 平面再乘一次（修：src 只栅格形状，裁剪由 GPU 侧 scissor/clipBG/深度裁剪/mask 平面执行）；CPU 回退路线 `installGPUClipMask` 写入的合成 mask（裁剪×用户）与软渲染器的 `ClipCoverage` 再乘一次（修：CPU 回退前 `MaskCoverage` 归一化回用户 mask，`applyMaskToPaint` 每次重置防嵌套残留）。新增自动化回归：`render/clip_ops_semantics_test.go` 8 项（含 CPU 回退变体，验证过「去掉修复转红 239,248,239→恢复绿」）+ `render/internal/clip/mask_fillrule_test.go` 4 项值级。窗=1200×800 四卡：A `ClipRectOp` difference 整洞 + 分数洞（AA 边 25%/43.75%/75%/81.25% 解析覆盖探针）/ B `ClipRectOp` replace（旧裁剪被丢弃）/ C `ClipPathOp` difference × NonZero（实心扣除）/ EvenOdd（环扣除）同向嵌套路径 / D `ClipPathOp` replace 不相交子路径 + 开放三角形（隐式闭合）。三证据：①逻辑探针 14 项（离屏 CPU 解析覆盖模型）②像素 23 探针（oracle=设备空间解析覆盖混合 + 独立通道带，容差 8/255；GPU 路线实测 `gpu_ops=4018 cpu_fallback_ops=0`）③golden 表头+图例+四卡 627200px 逐位零差（seed 后复跑 `diff=0.0000%`）。回归：clip 包 / p12 / s3c M3 clip / context_clip / depth clip / L.06 mask 全绿；`Clip\|Mask` 全量过滤失败项与基线 worktree 逐一对比一致（`TestClipCircle_CPUPixels`、`TestStencilRenderer_EnsureDepthClipPipelines`、clipping 严格档 2 区为**既有红**，非本次回归）。已知限界 2 项记 §6 与本窗 README。真源同步：§1 #13 ⛔→✅、§3.1 该项关闭、真窗目录计数 135→136；`ENGINE_UI_RENDER_BASE.md` §23 登记 + §24 修订 1.36、`ENGINE_UI_WIDGET_RENDER.md` §10 本行、`RENDER_API_CATALOG.md` §3.2/§4/§7.2 同步。**性质=原子验证**（直调 render 接口一次验一族），不算生产接线 |
