批 6 ③ · 材质窗 收口账（2026-11-06；设计见同目录 `windows-3456-design.md`，窗 `examples/engine/p37_material`）
====================================================================================

一、收口对象
------------
台账 747（3a premul）、748（3b 三入口）、749（3c 多纹理）、750（3d 描边洞+哨兵）
四行 ☑→✅。3e-3（751）归 ⑥ scene 流窗（归属修正见 `batch6-window-plan.md` §5）。

二、七臂判据与实测差值（20 枚探针全绿；容差显式：premul 混合 6 档 / 逐位段 0）
----------------------------------------------------------------
| 臂 | 断言 | 实测 |
|---|---|---|
| A 三入口材质 | 同一枚 uv 渐变 shader 挂图像口/quad 口/批绘口：挂材质段与解析 premul src-over 公式吻合（uv.y=0.506 → g=0.679 → 期望 rgb 由同一份常数折算）、与无材质对照差 ≥30 | `material_image/quad/atlas_differs_and_matches` 三枚 ✓；三口同色对账 `entries_color_parity` ✓ |
| | **恒等材质 ≡ 无材质逐位**（TEXTURE 原样返回 shader，内缩 4px 区域字节对账） | `identity_material_bitwise`：diff=0 worst=0 ✓ |
| B premul 半透明 | 白底 + premul 半透明红 shader（vec4(0.5,0,0,0.5)）→ (255,128,128) | `premul_semitransparent_fill_d9` ✓（批 3a premul 修复的画面第一次在窗内出现） |
| C SetImageAt | 双纹理对分屏：左半主图红 (220,60,60)、右半第二张蓝 (60,90,220) | `set_image_at_second_texture_paints` ✓ |
| D 描边吃 shader | AA 描边两端渐变 Δ>30 且非环境实心红（批 3d isGPUSolidPaint 修复的画面） | `stroke_paints_shader_gradient` ✓ |
| E 拒绝态 | 六枚前端哨兵 errors.Is（BadWGSL/NoEntry/BadReturn/SlotConflict/UniformTooLarge/Closed）全命中；透视+材质 quad 口 `ErrQuadMaterialUnsupported` **校验期具名返回**（不入队不计数）、拒绝区保持底色；图像口 bicubic+材质回退路 `MaterialSkips++`（skip ≠ fallback，不记 CPUFallbackOps） | `sentinels_named_six` / `quad_material_unsupported_refused` / `image_entry_material_skip_counted`（skips=1） / `refused_region_stays_backdrop` 全 ✓，底带八色片全绿 |
| ~~gles 承载~~（**臂 G 取代**） | 64KB 顶格 uniform 模块在后端可否编译 | 编译通过 ⇒ **这一子项当初没按设计兑现**（设计钉的是「后端真触发 + fallback 画面正确」，见 `windows-3456-design.md` 二·臂 e 的 2026-11-06 更正段）；`gles_budget_outcome=backend-accepted-on-this-device` 留作历史读数，真触发由下面的臂 G 兑现 |
| F uniform 出图（2026-11-06 补，缺口 2） | 272B uniform：`split`@256B（旧预算之外）+ `gain`@260B 决定分屏边界与右半增益；**同几何同纹理同着色器只换字节** | `uniform_payload_paints_split`（左 (220,60,60) / 中 (30,45,110)）、`uniform_boundary_follows_payload`（同一像素 split 0.25→0.75 由暗蓝翻成红）、`uniform_boundary_can_be_zero_from_payload`（split 写 0 ⇒ 左端也暗蓝，值确从载荷读；**不用全零载荷**——那会把增益一起归零成纯黑，与「什么都没画」不可分辨）三枚 ✓ |

三、三证据合证
--------------
1. 逻辑探针 20 枚（补证前 13 枚；`probes=20/20`），extra 键含 `identity_diff_px=0`、
   `material_skips_delta=1`、`gles_budget_outcome`、`nonfinite_rejects` 等。
2. 语义像素 12 枚（补证前 8 枚；终帧快照；期望色 premul src-over 公式同源）。臂 G 只放两枚**两端都为真**的对照锚，
   分歧那格交给 §七 的终帧二选一硬门（写死任一端都会把另一端判假）。
3. Golden 逐位 0（fail-closed、mask 指标带、容差 0）。**自臂 G 起两基线不再逐位相同**（两端行为分歧的
   画面证据，逐点数见 §七）；臂 G 之前两基线 `cmp` 逐位相同，那条记录仍然成立。

四、H15/H16
-----------
- H15：native 门禁相六轮（含播种后复核）金图恒 0.0000%；③ 为静态内容窗，
  常驻相（-manual-seconds 6，305 presents）金图同 0——如实记（无动画相位）。
  **2026-11-06 臂 F 落地后重录并复跑**：`seed_*` 播种跑仍判红（播种即非过），
  native/go 复核 + 两轮稳定轮四连 `probes=16/16 golden=0.0000%`（presents=241），
  两基线 `cmp` 逐位相同；原始行 `/tmp/w3_batch.log`。
- H16 四变异多重报警（改码→跑→取值→还原，还原后终验复绿）：
  1. 摘三入口材质挂载：渐变段回无材质色 + **金图 3.2227%**；
  2. `SetImageAt(1, nil)` 摘第二张：右半回空槽色 + `set_image_at_second_texture_paints` 红 + **金图 1.25%**；
  3. 拒绝臂换合法组合（摘 Perspective）：err=nil + 拒绝区被画出 + **金图 0.0347%**；
  4. **（臂 F）`fSplit` 0.25→0.60**：`uniform_payload_paints_split` +
     `uniform_boundary_follows_payload` 双红（中锚 (30,45,110)→(220,60,60)）+
     **金图 0.875%**；还原后 native/go 双后端各一轮 `16/16 0.0000%`（`/tmp/w3_final*.log`）。

五、门禁账
----------
`go build ./...` rc=0、vet rc=0、gofmt 净、apidoc rc=0（零新增公开 API）。
门禁相双后端 rc=0（presents=241、skips=0、backend_fails=0——**臂 G 落地后 native 端
`backend_fails` 变成每帧 +1（终值 239），那是被测行为本身，不是回归，见 §七**）。实现中两次自身
口径纠偏（如实记）：① 期望色公式初版 0-1/0-255 单位混用（探针打回后修正）；
② 材质拒绝语义两次核实——quad 口是**校验期具名返回**（不入队不计数），
`MaterialSkips` 计的是图像口 GPU 腿不接走的回退拒绝，探针按真实语义改写。

六、收口动作
------------
台账 747/748/749/750 ☑→✅（附本窗名+实测值）；§7 追加批 6 ③ 行；
`batch6-window-plan.md` §5 补交付进展。

七、臂 G「后端拒收看得见拒绝」（2026-11-06，审计缺口 3 的设计子项兑现）
--------------------------------------------------------------------
**为什么补**：设计在 ③ 臂 e 就钉了「`ErrShaderBackendCompile` 的后端真触发 +
fallback 画面正确」，交付时用「64KB 顶格 uniform 模块」试，本机两后端都编译通过，
于是记成「不可复现于此设备」——**这一子项当初没兑现**。补臂 F 时撞出真触发子
（uniform 数组 stride 非 16 倍数：前端放行、naga 拒），本轮把它做成窗内证据。

**臂形态**（三格并排，y=600，130×60）：格 1 无材质环境实心红（证明这块地方画得出东西）、
格 2 同环境色挂**故意必拒**模块、格 3 同几何挂合法材质（(51,229,102)）。
中间那格的既定语义=**整笔拒画 + 计数**：区内保持底色 (14,14,19)，
`ShaderBackendFails++`、`CPUFallbackOps++`，**环境实心红不得冒充回退**。

**两端分歧（实测，写进锚点口径而不是抹平）**：
| 后端 | 这枚模块 | 格 2 终帧 | 呈现帧 `ShaderBackendFails` |
|---|---|---|---|
| native（Vulkan/wgpu-native） | naga 在 `CreateShaderModule` 拒 | (14,14,19)=底色 | 239（每帧一次） |
| go（纯 Go GL） | 其 WGSL 前端不查 uniform stride ⇒ **接受并出图** | (128,130,128)=shader 解析色 `vec4(uv.x,uv.y,0.5,1)` | 0 |
解析色不是事后拟合：由 `effBadArrayStrideWGSL` 的输出式与格 2 几何
（uv=(px-260+0.5)/130、(py-600+0.5)/60）算出，与实测逐通道吻合。

**为什么必须看终帧而不是离屏探针**（本轮自己踩的坑，如实记）：离屏 `render.NewContext`
走的是 `OpenStandaloneDevice`，而它**按设计不吃 `GPUI_BACKEND`**
（`render/device_open.go` 注记：纯 Go GL 没有 surfaceless 独立设备）。所以我先前那份
「两后端同形」的三条量，`go` 那一轮其实仍是 Vulkan——**该结论作废并从零窗的
`arm_g_backend_rejects` 重测**（窗内设备才是被测对象）。三枚离屏探针因此改名
`standalone_leg_*`，只声明独立腿。

**终帧硬门 `arm_g_frame_check`**（二选一，两端各自钉死）：
「底色 + 计数 ≥1」**或**「shader 解析色 + 计数 0」；两者都不许是环境实心红，
也不许「底色却零计数」（=拒而不计数=静默消失）。JSON 另存
`arm_g_frame_ok` / `arm_g_backend_rejects` 两个跨端标记键。

**门禁实测**：`RUN_SECONDS=20`（8 秒只跑到 ~193 presents，被 H15 定帧门判红——代价来自
下条），native 三轮 + go 一轮全 `probes=20/20 golden=0.0000% presents=241`
（`/tmp/w3_g3.log`、`/tmp/w3_g4.log`、`/tmp/w3_fin_n{1,2}.log`、`/tmp/w3_fin_g.log`）。
每帧挂一枚必拒模块的代价（**H23 修后已实测改口，见 `h23-fix.md` §五**）：设备模块创建
原本是每帧一次，现为整窗一次（`window_shader_backend_fails` 239→1、stderr `pass1 err`
243→2 行），但 p95 帧间隔 51.13→50.48ms 基本没动——真正的大头是**每帧走 CPU 回退腿画那一格**
（同窗 go 后端无拒收时 p95=27.87ms）。本窗门禁相因此仍需 `RUN_SECONDS=20`。

**H16 第五变异**（把格 2 的拒收模块换成合法模块）：`standalone_leg_reject_counted`
（0→0）+ `standalone_leg_reject_region_keeps_backdrop`（(51,229,102)≠底色）+
终帧 `arm_g_frame_check`（既非底色也非解析色）三处红，金图 native **0.9540%** /
go **0.8464%**；还原后 native/go 复绿。

**两基线分歧点数**（自本臂起 `cmp` 不再相同，逐点核过）：格 2 整格 **7800 px**
（底色 vs 解析色=被测分歧）+ 底带 8 枚色片边缘 **992 px**（每行 32 px×18 行、
1 级 AA 取整差；8 枚色片中心色两端逐一相同 (70,200,120) ⇒ 状态无差，纯取整）。
指标带 y<32 不进比对。


勘误（2026-11-07，材质×透视/bicubic 专项连带）：本行所记「透视+材质
ErrQuadMaterialUnsupported 校验期拒」「图像口 bicubic+材质 MaterialSkips++」
两态已随专项放行迁移——透视+材质画出（A 批权重透传）、bicubic+材质画出
（B 批纹素契约）；现场拒迁投影层眼后（ErrQuadBehindEye），skip 演示改
blend+材质（Normal-only 不变）。探针名相应更替
（quad_behind_eye_refused / image_entry_material_blend_skip_counted），
历史账留痕不改。账 `matpersp-fix.md`。
