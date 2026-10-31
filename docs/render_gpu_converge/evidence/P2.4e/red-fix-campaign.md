# 红修战役状态（P2.4e 收口后全量清红）

- 日期：2026-10-31 开战役；基线 `2d39194d`
- 目标：用户指令「中间遇到的问题彻底解决，不留尾巴」——当前树全部测试红逐个根因修复。

## 已修复（已提交）

| 批次 | 提交 | 内容 |
|---|---|---|
| e1-e4 内 | `762f2e4e` 前各提交 | I2（swapchain X11 期望改绑）→ 桌面实跑 PASS；J1/J2 防护；C7 三形态；J5 所有权 |
| 红修批次1 | `762f2e4e` | **gpu 包 7→0**：共享布局测试缝（testSharedLayouts helper + 12 文件注入）、session SetMaskBindLayout、opt21 按 C5/ADR-017 语义重构（旧断言属布局 pin 要求落地前从未执行到的死期望） |

## 当前剩余红清单（新鲜逐文件 + 包级）

### render 包（9 文件逐文件红 + 8 条包级耦合红）

| 红文件/测试 | 失败现象 | 诊断进度 |
|---|---|---|
| context_coverage_test TestContextSetFillBrushCoverage | SetFillBrush(Solid) 填充 R=0（SetRGB 同形状正常） | **排查链已排除**：SetBrush→isSolid ✓、applyDeviceSpaceBrush 对 isSolid 早退 ✓、三条 CPU 填充路径（CoverageFiller/Analytic/NoAA）的 solidColorFromPaint 均兼容 isSolid ✓、Solid() 构造 ✓。**下一步**：doFill 分发处活体探针（确认走了哪条分支、GPU 加速器是否截获、paint 状态） |
| context_clip_depth_test TestClipCircle_CPUPixels | 圆形裁剪中心像素黑 | 未开始（同 CPU 像素类，或与 coverage 同因） |
| context_misc_draw_gpu_test DrawColorEffectSurfaceAdvancedBlend | effect multiply=(255,0,0) want(229,0,0)——高级混色在 effect-surface 路径未生效 | 未开始（疑似真实渲染 bug 或 blend 域转换缺口） |
| filter_present_coherence_test TestPresentDamage_ImageMatchesView | present 后 Image() 仍白 | 未开始（疑测试语义：no-op present 不回读 CPU pixmap） |
| m5_color_glyph_api_test DrawShapedColorGlyphsCPU_Emoji | NotoColorEmoji 无 ink | 疑环境（字体缺失）——需验证本机字体在位性；D2 曾判环境性 |
| rasterizer_tile_value_test 菱形(分数行起止) | 光栅化 vs 解析值精度 | 未开始 |
| s6_9_heavy_budget_test TestS69_HeavyBudget_TierGates | 预算档位比 | 未开始 |
| text_transform_gpu_test Auto vs Vector 569px | 旋转文本路由 | 未开始 |
| video_direct_upload_test TestVideoDrawFrameMarksDamage | 视频帧损伤标记 | 未开始 |
| 包级 8 条（DrawShadow/P1GLPresentX11/PresentResize×2/S3c_BlendHue/S64/S69_Contract） | 仅整包跑现红、逐文件各自通过 → **跨测试 GPU 状态顺序耦合**（D2 归 P2.6 的顺序耦合组） | 需测试隔离（每测 GPU 状态清理） |

### ui 包（2）

| 红 | 失败现象 | 处置 |
|---|---|---|
| ui/depcheck TestUIDoesNotImportGPU | ui/rendering 两个测试文件的 gpu 测试缝被无豁免扫描命中 | 修 depcheck：_test.go 测试缝豁免（生产码禁令不变）——机械 |
| ui/focus TestFocus_KeyWithNoPrimary | 焦点键序 | 未开始 |

### gles（1）

| 红 | 现象 | 处置 |
|---|---|---|
| TestPresentDamageX11（G3 在册） | partial present never engaged | G3 车道（GL 增量呈现线），非本战役新债 |

### 已修不在册确认

- swapchain_x11_linux_test（I2）：已修，桌面 X11 PASS。
- gpu_texture_test：e1 已整文件删除（其红随删除消失）。

## 修复纪律（本战役沿用）

1. 每条红：先定性「测试过期 vs 产品 bug vs 环境红」，产品 bug 走真修，测试过期写明依据改绑新语义，禁止为凑绿删断言。
2. 每修一批：跑该批文件测试 + 相邻回归 + 整包对照（红名单只减不增）。
3. 环境红：验证环境前提（字体/显示）后改 t.Skipf 显式标注，禁止静默假绿。

## 2026-10-31 续：类根因突破

- **CPU 像素断言类根因落定**：GPU 库在场时 Context 填充一律被 GPU 截获（探针实测 SetRGB 与 SetFillBrush 均 GPUOps=1、dc.pixmap 不更新——「笔刷特有」假设被对照探针推翻，两条路径 paint 状态完全一致）。断言 CPU 像素的测试需要显式 CPU：`t.Setenv("GOGPU_RENDER_MODE", "cpu")`。
- 已修：`context_coverage_test TestContextSetFillBrushCoverage`、`context_clip_depth_test TestClipCircle_CPUPixels` → 带 lib PASS。
- render 整包现状：**仅剩 8 条包级顺序耦合红**（DrawShadow/P1GLPresentX11/PresentResize×2/S3c_BlendHue/S64/S69_Contract/S69_TierGates）；逐文件红需重跑逐文件清单复核（coverage/clip_depth 已清，misc_draw/m5/rasterizer/s6_9/text_transform/video 的逐文件红待下轮复核定性）。
- 下一步：ui 2 条（depcheck 豁免机械修 + focus 诊断）→ 逐文件复核 → 包级耦合红的测试隔离改造。

## 2026-10-31 续2：批次3（ui 2→0）+ 新暴露

- 已修：depcheck 测试缝豁免（J4 闭合）、FocusManager Tab 空环穿透语义（KeyWithNoPrimary 红→绿）——提交 9cca54e6。
- 新暴露（ui 全量扫描，此前未入逐文件清单的包）：TestSeatCapCBFirstSilent / TestX11S4ProcessKeyEvent / TestX11StateReadback——X11/seat 相关，疑似显示环境依赖（无 DISPLAY 或 NVIDIA 环境行为），下轮先定性（干净环境 A/B + 是否需 display 前置 Skipf）。
- 当前累计进度：gpu 7→0 ✅、render CPU 像素类 2 条 ✅、ui 2→0 ✅；剩余 = render 逐文件复核 7 条 + 包级耦合 8 条 + 新暴露 3 条 + gles G3（在册车道）。

## 2026-10-31 续3：批次4（gpu 逐文件残红 7→1）

- 已修：session 级 `SetMaskBindLayout` 注入口 + 25 处 session 测试注入（opt24/27/28/29/40/gpu_texture/ttc/render_session——这些测试独立 session 渲染，管线需要 mask 布局，此前仅整包模式下被其它测试的全局状态掩盖）；`buildImageResources` 对齐 `ensureImagePipeline` 时序（首次可达的 GetOrUpload/绑组路径此前nil panic）；render_session 撤无 clip 语义测试的误 pin、凸重建测试 mask-only（hasClip=false 契约保持）；ttc 变量名修正。opt24/ttc 转绿。
- **剩余独立立案：gpu-tex 管线 group1 布局契约缺口**——opt27/28/29/gpu_texture 残红（`CreateRenderPipeline: Shader global ResourceBinding { group: 1, binding: 0 } is not available in the pipeline layout`，错误经 wgpu 粘滞滞后面显示在 CreateBuffer 处）。GPU-tex 渲染管线的 layout 未含 shader 所需的 group 1——需专项：定位该管线构建点与 shader 契约，决定补布局还是改 shader 绑定。整包模式绿（其它测试的全局状态掩盖），逐文件必现。
- 当前 gpu 包：整包绿、逐文件 1 个家族残红（上述专项）。
