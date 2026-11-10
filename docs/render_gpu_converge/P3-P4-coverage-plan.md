# P3/P4 覆盖补测与真窗编排（2026-11-10 复确认版）

> 性质：**计划**。先复确认现有载体，再给缺口排「通用、覆盖全、不重复」的测试与真窗。
> 口径：每个主能力一个**主单测文件/表** + 一个**主真窗**；子分支加 case/加卡，不新开
> 同能力第二扇窗；负向与双后端在既有载体上做。现有 33 扇 P3/P4 能力窗全部实测存在
> （目录 + `-auto-only` + PNG），本计划**不新增能力窗**。
>
> **2026-11-10 能力级审计修订（正本 `P3-P4-capability-inventory.md`）**：任务级 ✅ 不等于
> 能力级全覆盖。新发现 **P3.8 G14 marker 在 D2 声明要做但代码未实现**（`render/svg`
> 只有 feature 字符串，无 Marker 解析/渲染/测试/窗）；P3.4 的 15 个 ImageFilter 节点
> 在 `ui_wr_p34_filters` 只有 3 张卡（AlphaThreshold 无单测无窗、Displacement 基座
> 无单测/GPU aux 未接、Erode/Dilate 只有窗卡无节点单测、其余多节点无主窗卡）。
> 本计划的 P3.4 行由「六项残留」扩展为「全节点矩阵」；marker 是否补实现需用户拍板
> （补实现 = 能力任务，不是测试任务），不能默默按 ✅。

## 0 复确认结论

- 33 扇窗：P3.1–P3.4 各 1、P3.5 4+pathbool、P3.6 3、P3.7 6、P3.8 8、P3.9/9a/9b 3、P4.1 4。
- 单元测试按 evidence 列的测试文件逐个核对存在；缺口只在下列 P4.2/P4.4/P4.3 其余项、
  P4.1 #3 GPU 路、P3.4 六项残留，以及 **live 帧**与 **DrawVertices/DrawMesh 回放**
  两个验证口径。
- 已有窗的门禁以**快照**为主；live present 帧的像素断言只有 P4.1 批 4 的四扇临时
  hook（已删）和本次 mesh 的 op 计数证据。这是验证口径缺口，不是窗口缺失。

## 1 缺口 → 通用实现 → 单测 → 真窗（不新开窗）

| 缺口 | 通用实现（引擎/标准） | 单测（扩既有表/文件） | 真窗（复用/扩卡） | 不重复说明 |
|---|---|---|---|---|
| P4.1 #3：PopLayer 的 Porter-Duff 非 Normal GPU 层内容丢 | 把 `QueueGPUTextureDrawBlend` 做成**全 Porter-Duff 族**的通用 GPU blit（blend state 取 `intBlend.GetBlendFunc`，修 pass/pipeline 兼容链），不是 Plus 特判 | 扩 `render/vertices_paint_blend_test.go` + `render/testdata/verts_paint_blend_cases.json`：Plus/Xor/SrcIn/DstIn/Clear/SrcOut… 全族表，CPU/GPU 双跑 | `ui_wr_p41_mesh_paint_blend` 的 B 卡（Plus）作触发，不新开窗；修后**重种该窗 native+go 金图**（现图含 B 缺失） | 不按每种 blend 各开一扇窗；一张表覆盖全族，一扇窗做集成 |
| DrawVertices/DrawMesh 的 live 回放无 round-trip 钉 | 走既有 `DrawVerticesCommand/DrawMeshCommand` 通用命令链，不新增命令 | **新增一个** `render/recording/vertices_mesh_roundtrip_test.go`：DrawVertices（带色）/DrawMesh（带索引色）direct vs playback + bounds 表 | 无新窗；mesh 窗 live 已由 `picture_op_count 63→67`/`gpu_ops` 证明 | 一个命令族一个文件，不按示例拆 |
| P4.2 GPU/CPU 模糊一致 | 先复跑：模糊/XY/图/渐变 ramp 的通用 GPU-vs-CPU 对照；差异按 Skia premul-sRGB 口径定标准，不调容差糊 | 扩 `render/filter_ops_gpu_test.go`（模糊/图）+ **一个** `render/gradient_consistency_test.go`（linear/radial/ramp 中段域）；不新开 per-case 文件 | 复用 `ui_wr_p34_filters`（模糊/滤镜，绿）+ `ui_wr_p38_svg_paint`/`ui_wr_p31_linear_blend`（渐变 ramp，绿）；不新开窗 | P4.2 是一致性任务，不是新能力；同一批绿窗做 smoke，不重复开窗 |
| P4.4 COLRv1 变量轴 + CPAL | 通用 fvar/avar delta 求值 + CPAL palette 切换/变体（对齐 OpenType 1.9 + Skia scaler），不写单字体特判 | 扩 `render/text/emoji/colrv1_test.go`（Var solid/Var gradient/deltas/palette 0/1/0xFFFF 表）+ `color_font_e2e_test.go` | 扩 `ui_wr_p38_color_font` **一张卡**（变量+CPAL）；测试字体用既有 `mkcolorfont` 生成器加一枚变量字体；不新开窗 | 彩色字形能力已有主窗；新分支加卡，不另开同能力窗 |
| P3.4 六项残留（displacement GPU aux、ApplyMask/PushMaskLayer blur、merge GPU seeding、erode/dilate O(n)、Stroke paint filter、advanced blend×paint filter） | 在既有 filter/mask 通用链上补分支（GPU aux 上传 / blur 采用 / seeding / 算法优化 / stroke 走同一 paint filter / 高级混合复用层），不按示例特判 | 扩既有 `render/p34_*`、`render/svg/filter4b_test.go`、mask/filter 表；一门类一张表 | 扩 `ui_wr_p34_filters` 的卡（六分支各一卡）+ 必要时扩 `ui_wr_p38_svg_filter` 的 displacement 卡；不新开窗 | 六项都是同一能力族的分支；加卡不加窗 |
| P4.3 其余（mask 池化、backdrop 损伤区、抖动） | 先量后改；仅当实测有用户可见成本才做通用优化（池化/损伤区/查表），不先写补丁 | 基准/探针（不是能力单测）：复用 `ui_wr_c10_soak`/`ui_wr_c9_stress_cache` + `ui_render_base_backdrop`/`ui_wr_p34_filters`；一个 bench 文件 | 复用上述既有窗；不新开能力窗 | 性能项没有新能力面；不开窗，先测 |
| live 帧无通用断言 | **一个共享钩子/测试设施**：embedder/wrkit 的 `GPUI_LIVE_CAPTURE`（go/GL FBO）+ recording-replay parity 兜底；按窗复用，不做每窗 bespoke 钩子 | 共享 helper + 现有窗的 `-auto-only` 附加 live 比对；不是每窗一个测试文件 | 现有 33 窗按需开启，不新开窗 | 通用 harness 一次实现，全窗复用；Wayland/native FBO 不可用时用 replay parity，如实登记 |

## 2 去重规则（硬）

1. **一个能力一个主单测文件/表**：新 case 进既有文件；只有新命令族（DrawVertices/DrawMesh）才新增一个文件。
2. **一个能力一扇主真窗**：子分支加卡/加 probe，不开第二扇同能力窗；33 扇现有窗就是全集。
3. **负向/双后端/金图不复制**：每个能力族一次负向 mutation、一次双后端 0.0000%、一次金图重种；不在每个 case 重复。
4. **金图只重种受影响的窗**：P4.1 mesh（#3 修后）、P3.4 p34（加卡后）、P4.4 color_font（加卡后）；其它窗不重种。
5. **不把组合窗当单能力窗**：P4.2/P4.3 是任务/性能线，不做新能力窗；用既有能力窗做 smoke/基准。

## 3 执行顺序

1. **P4.1 #3**（阻塞 P4.1 ✅）：引擎通用 Porter-Duff GPU blit + 全族表 + mesh 窗 B 卡 + 金图重种。
2. **DrawVertices/DrawMesh round-trip**：一个 recording 表，钉住 mesh live 修复。
3. **P4.2**：先复跑通用 GPU-vs-CPU 表（模糊/图/渐变），按标准定结论；只有确认差异才动引擎。
4. **P4.4**：Var/CPAL 表 + color_font 加卡 + 金图重种。
5. **P3.4 六项残留**：按族补分支 + p34 加卡 + 金图重种。
6. **P4.3 其余**：先基准，无实测问题就标 ➖/延期，不写预优化。
7. **live 通用 harness**：可并行，先 go/GL FBO + replay parity 兜底，逐窗接入。

## 4 验收口径（每步）

- 先红后绿（能力表 + 负向 mutation 红→还原绿）；双后端金图 0.0000%（重种需用户批准并归档旧图）；
- 真窗单窗/标准关闭/前后查显示；live 用通用 harness（不可用处如实登记）；
- 显存账本关窗回落；race/vet/apidoc/双构建；干净树 A/B 零新增失败；
- 公开 API 变化才同步 catalog/apidoc；不新增能力窗。

## 5 边界

- P4.2 的渐变 ramp 差异可能最终是**后端采样域**而非 bug；若按 Skia 标准确认是允许差异，登记为显式容差，不强行改画面。
- P4.3 其余三项在无实测成本前不写代码；避免“预优化”变成补丁。
- live 通用 harness 在 Wayland/native 受 FBO 限制；replay parity 是兜底，不是等价替代，需在证据里写清覆盖范围。
