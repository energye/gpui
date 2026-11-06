批 6 ③ · 材质窗 收口账（2026-11-06；设计见同目录 `windows-3456-design.md`，窗 `examples/engine/p37_material`）
====================================================================================

一、收口对象
------------
台账 747（3a premul）、748（3b 三入口）、749（3c 多纹理）、750（3d 描边洞+哨兵）
四行 ☑→✅。3e-3（751）归 ⑥ scene 流窗（归属修正见 `batch6-window-plan.md` §5）。

二、五臂判据与实测差值（13 枚探针全绿；容差显式：premul 混合 6 档 / 逐位段 0）
----------------------------------------------------------------
| 臂 | 断言 | 实测 |
|---|---|---|
| A 三入口材质 | 同一枚 uv 渐变 shader 挂图像口/quad 口/批绘口：挂材质段与解析 premul src-over 公式吻合（uv.y=0.506 → g=0.679 → 期望 rgb 由同一份常数折算）、与无材质对照差 ≥30 | `material_image/quad/atlas_differs_and_matches` 三枚 ✓；三口同色对账 `entries_color_parity` ✓ |
| | **恒等材质 ≡ 无材质逐位**（TEXTURE 原样返回 shader，内缩 4px 区域字节对账） | `identity_material_bitwise`：diff=0 worst=0 ✓ |
| B premul 半透明 | 白底 + premul 半透明红 shader（vec4(0.5,0,0,0.5)）→ (255,128,128) | `premul_semitransparent_fill_d9` ✓（批 3a premul 修复的画面第一次在窗内出现） |
| C SetImageAt | 双纹理对分屏：左半主图红 (220,60,60)、右半第二张蓝 (60,90,220) | `set_image_at_second_texture_paints` ✓ |
| D 描边吃 shader | AA 描边两端渐变 Δ>30 且非环境实心红（批 3d isGPUSolidPaint 修复的画面） | `stroke_paints_shader_gradient` ✓ |
| E 拒绝态 | 六枚前端哨兵 errors.Is（BadWGSL/NoEntry/BadReturn/SlotConflict/UniformTooLarge/Closed）全命中；透视+材质 quad 口 `ErrQuadMaterialUnsupported` **校验期具名返回**（不入队不计数）、拒绝区保持底色；图像口 bicubic+材质回退路 `MaterialSkips++`（skip ≠ fallback，不记 CPUFallbackOps） | `sentinels_named_six` / `quad_material_unsupported_refused` / `image_entry_material_skip_counted`（skips=1） / `refused_region_stays_backdrop` 全 ✓，底带八色片全绿 |
| gles 承载 | 64KB 顶格 uniform 模块（`ShaderUniformBudgetMax=65536` 前端放行）在后端可否编译 | **本机 native 与 gles 均编译通过**——真触发不可复现于此设备，如实记（`gles_budget_outcome=backend-accepted-on-this-device`）；哨兵链路由批 3d 双 %w 包装钉覆盖；`ShaderBackendFails` 计数与 stderr 接线已验证存在 |

三、三证据合证
--------------
1. 逻辑探针 13 枚（`probes=13/13`），extra 键含 `identity_diff_px=0`、
   `material_skips_delta=1`、`gles_budget_outcome`、`nonfinite_rejects` 等。
2. 语义像素 8 枚（终帧快照；期望色 premul src-over 公式同源）。
3. Golden 逐位 0（fail-closed、mask 指标带、容差 0；native/go 两基线逐位相同）。

四、H15/H16
-----------
- H15：native 门禁相六轮（含播种后复核）金图恒 0.0000%；③ 为静态内容窗，
  常驻相（-manual-seconds 6，305 presents）金图同 0——如实记（无动画相位）。
- H16 三变异三重报警（改码→跑→取值→还原，还原后终验复绿）：
  1. 摘三入口材质挂载：渐变段回无材质色 + **金图 3.2227%**；
  2. `SetImageAt(1, nil)` 摘第二张：右半回空槽色 + `set_image_at_second_texture_paints` 红 + **金图 1.25%**；
  3. 拒绝臂换合法组合（摘 Perspective）：err=nil + 拒绝区被画出 + **金图 0.0347%**。

五、门禁账
----------
`go build ./...` rc=0、vet rc=0、gofmt 净、apidoc rc=0（零新增公开 API）。
门禁相双后端 rc=0（presents=241、skips=0、backend_fails=0）。实现中两次自身
口径纠偏（如实记）：① 期望色公式初版 0-1/0-255 单位混用（探针打回后修正）；
② 材质拒绝语义两次核实——quad 口是**校验期具名返回**（不入队不计数），
`MaterialSkips` 计的是图像口 GPU 腿不接走的回退拒绝，探针按真实语义改写。

六、收口动作
------------
台账 747/748/749/750 ☑→✅（附本窗名+实测值）；§7 追加批 6 ③ 行；
`batch6-window-plan.md` §5 补交付进展。
