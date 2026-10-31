# P2.4e 裁剪 / 混合枚举 / 呈现入口收敛 + 既有缺陷修复 — 影响面评估

- 日期：2026-10-31；基线 HEAD `68a69e69`（P2.4d 收口 + 台账补记）
- 范围来源：PLAN §7.3 P2.4e（B2 裁剪 / B3 混合枚举 / B4 呈现入口，三套各收敛为一）+ §2.2 池带入 I1 / C7 / I2 / C3 / G2 / J1 / J2（J3 已核闭合收回）+ P2.4d summary §6 四条偏差复核
- 盘点方式：四路并行源码探查（裁剪 / 混合 / 呈现+锚点 / G2 逐符号），file:line 均已核到

## 1 B2 裁剪三套收敛 → `clip.ClipStack` 唯一

现状（真栈共 3 个，套 4/5 是套 1 的 GPU 表达，套 6/7/8 无独立机制）：

| 套 | 定义 | 状态 |
|---|---|---|
| 1 标准 | `render/internal/clip/stack.go:30` ClipStack（设备空间、AA 覆盖度、Intersect/Replace/Difference、mask 缓存） | 保留 |
| 2 编码簿记 | `render/scene/layer.go:360` ClipStack + `:297` ClipState（用户空间、无 AA、严格嵌套） | **删** |
| 3 CPU tile 局部 | `render/scene/renderer.go:621` tileClipState（R8 mask、固定 non-zero、无组合 op） | 本步不动（见 D1） |

- 套 2 **无任何渲染消费者**：裁剪实际经 TagBeginClip/TagEndClip（`scene/encoding.go:672/677`）重放进套 1（`scene/gpu_renderer.go:181-206`）。删除损失面仅两个查询面：`Scene.ClipDepth/ClipBounds`（scene.go:579/589，消费者 `examples/ui_render_base_miscdraw`）与 `Layer.ClipStackDepth`（layer.go:83-85）。
- 语义分叉第二处：CPU tile 重放路径自绘 mask，与 GPU 路径（scissor+depth-clip）不对称——本步只登记，不统一。
- 复用亮点如实记：套 5 深度裁剪的 AA mask 直接复用套 1 的 `clip.NewMaskClipper`（depth_clip.go:558）；两套 mask 缓存（internal/clip/mask_cache.go 与 depth_clip.go:154）分账（后者走 `BudgetClipMask`），职责重叠暂不合并。

## 2 B3 混合枚举三套收敛 → `render.BlendMode` 唯一

| 套 | 定义 | 值域 | 生产消费者 |
|---|---|---|---|
| render.BlendMode | `render/blendmode.go:34`（=`internal/image` BlendMode 别名，draw.go:23） | 28 值 | render 包内 58 文件 + 包外 14 文件 |
| scene.BlendMode | `render/scene/encoding.go:24` | 29 值（线格式；**与 render 仅前 4 值同值，第 5 个起分叉**） | layer/scene 定义 + gpu_renderer.go:176 重放 |
| internal/blend.BlendMode | `render/internal/blend/porter_duff.go:18` | 29 值（两段编码） | 非测试消费者仅 `scene/blend_integration.go` 本身 |

- 6 张手写转换表是全部冗余：blend_integration.go 4 张 + blend_gpu.go:57-182 与 dual_tex_blend.go:848+ 两张 GPU 表。
- 语义缺口如实登记：render 独有 `Modulate`（scene 侧折叠为 Multiply，blend_integration.go:146）；scene 独有 `Destination`（降级 Normal，:80）；internal 独有命名 `Source`（=render 的 Copy）。`blendmode.go:26` 提到的第 4 套 `surface.BlendMode` 已不存在（注释过时，顺手清）。
- 方案见 D2：a) internal/blend 枚举改绑 render 值域（删 2 张表，CPU 算法 GetBlendFunc 保留只换入参）；scene 线格式 b) 保留自有值域只留 1 张边界表（不改 wire，零重录）或 a') 常量改绑 render 值（删 2 张表，但已归档 recording 的 wire 值全变，需重录/重采受影响基线）。

## 3 B4 呈现入口收敛 → `frame.PlanFramePresent` 唯一决策

决策路径实际有 5 层（比 PLAN 写的三套还多两层）：

1. `render/frame.go:144` PlanFramePresent（四态：idle/damage_union/damage_multi/full，阈值 frame.go:56-67）——生产唯一调用点 `present_frame.go:372`。
2. `render/present_frame.go:182` PresentTarget.present 自带决策：clipPresentDamage（:245，**与 PlanFramePresent 平行的第二份 damage 合并实现**，present_damage_test.go:20 注释自认同策略）+ forceFull/postResizeFull/resize storm（:217/:312）+ idle 短路（:251）。
3. `render/present.go:97/108/120` PresentFrame* 三入口，决策在调用方手里。
4. `ui/embedder/pipeline_app.go:394` SetPresentPolicy（full_paint/retained/hybrid → useRetained，:2014-2149）——正交产品策略层。
5. 后端：webgpu `swapchain_configure.go:220` 同尺寸 no-op（带 pendingReconfigure 逃生门）+ `swapchain_frame.go:110-220` outdated/suboptimal 重配；gles `swapchain.go:194` 同尺寸 no-op（**无逃生门**）+ `surface.go:240` Configure 短路 + `queue_linux.go:272-290` EGL 三扩展决定真局部；webgpu `surface.go:390-392` PresentWithDamage **直接丢 damage 参数全量**——同一份 damage rects 两后端终局不同。

设计：clipPresentDamage 并入 PlanFramePresent（damage 合并一份）；idle 判定归一；resize 语义三层对齐为「同尺寸 no-op + 显式重配口 + outdated/suboptimal 上报重配」（对齐 Skia/Flutter：同尺寸不重配）；embedder SetPresentPolicy 保留为策略输入不进 render（产品策略非图形算法）。I1 锚点 `gpu/rwgpu/surface.go:517`（wgpuSurfacePresent 阻塞 FFI，gpuMu 锁下无超时）在本步解剖（呈现入口收敛同线，修复方向解剖后拍板）。

## 4 既有缺陷修复（池带入）

| 项 | 锚点 | 方向 |
|---|---|---|
| C7 | `gles/api_linux.go:78/157/190/236`、`resource_linux.go:86/178` 等控制器线程 eglMakeCurrent 撞唯一 GL 线程 Submit keep 窗口期（c7-dissection.md） | **按已拍板解剖结论执行**：开窗 GL 步骤投递 hub GL 线程 FrameJob；修后 go 档真窗（单窗+多窗）应转绿 |
| I2 | `swapchain_configure.go:220` no-op vs `swapchain_x11_linux_test.go:218-219` 期望重配（期望出自 S6，早于 no-op 定型） | 先拍板（D4）：推荐改测试期望对齐 no-op + 新增显式 ForceReconfigure 口；**禁止为凑绿删断言** |
| C3 | `examples/ui_ime_probe/main.go:225-246` 事件循环缺 `EventCloseRequested` 分支（引擎已正确发出，x11_linux.go:1552） | 示例自身缺陷，补分支即闭合（示例层修自身 bug，非绕引擎洞） |
| I1 | `rwgpu/surface.go:517` 阻塞 Present | 本步解剖（D 阶段），**解剖前不定修复方向**（池规则） |
| J1 | res 电平触发回调 + repool 无查重（复现红 `j1-overrelease-red.txt`） | repool 查重一行 + 回调边沿触发（或 Ref 恰一次防护）+ attack_test.go 补「带回调 over-release」负向用例 |
| J2 | rwgpu work-done 记录 drain/invalidate 不观察即滞留（`j2-j3-enum-evidence.txt` 闭环枚举） | drain 补 token 观察 + invalidate 记录清理出口 |

## 5 G2 第二层老抽象逐符号拆解（盘点结论）

| 文件 | 符号 | 生产存活 | 仅测试 | 死 | 处置 |
|---|---|---|---|---|---|
| command_encoder.go | 45 | 0 | 42 | 3 | **整文件删**；牵连 render_pass.go:116/compute_pass.go:107 的 CoreCommandEncoder 字段簇（RenderPassEncoder/ComputePassEncoder 生产不可达，同簇一并评估）+ command_encoder_test.go 全文件 |
| commands.go | 58 | 1 | 3 | 54 | 整文件删；`IndexFormat` 生产实为 `types.IndexFormat`（gpu 版常量值与 types **错位**：Uint16 gpu=0 / types=1，仅测试在用）；text_pipeline.go:823/856 两个仅测试方法删 `*RenderPass` 参数 |
| atlas.go | 34 | 11 | 0 | 23 | 搬 `RectAllocator` 簇 ~190 行（含 AtlasRegion/MinAtlasSize/DefaultShelfPadding）进 glyph_color_atlas.go；`TextureAtlas` 整块 ~210 行**连测试都零引用**，直接删 |
| gpu_texture.go | 32 | 2 | 20 | 10 | 搬 `alignTextureBytesPerRow`+`copyPitchAlignment`（~6 行，12+ 生产文件高频消费）；GPUTexture 家族 ~300 行仅测试（唯一消费者 p0_format_readback_test.go P0 真机校准）——改写直连 hal（推荐，见 D6）或整簇搬移 |

四个文件约 2300 行，生产可达 ~200 行。外部生产导入者仅 render/gpu/gpu.go 与两个 compute 示例（RenderSceneAuto/VelloAccelerator），不涉及其余层。

## 6 实施顺序与门禁（D7 推荐）

一次只动一类（PLAN §7.3 拆步原则），四个子步各自独立提交+门禁：

| 子步 | 内容 | 风险 |
|---|---|---|
| e1 | G2 删/搬 + J1/J2 防护（纯减法+几行防护，热身还债） | 低 |
| e2 | B2 clip：删 scene.ClipStack + 查询面处置 | 低（无渲染消费者） |
| e3 | B3 blend：internal/blend 收敛 + scene 边界处理 | 中（wire 值域方案待拍板） |
| e4 | B4 present：PlanFramePresent 吸收 + resize 三层对齐 + C7/I2/C3 + I1 解剖 | 高（呈现主路 + 后端两侧） |

门禁每子步全量：五目标构建 / vet 逐条不变 / 逐测试文件 / `-race` / conformance 双后端 0/0 / API 快照白名单 / 残留 grep；e4 加真窗四扇（l1_blank、multiwin_cap、bscene、r20_filter）+ go 档（C7 修后应绿）+ swapchain X11。金图：multiwin 16/16 + bscene + r20 final diff=0。R20 自身既有红 I3/I4 不属本步，如实沿用。

## 7 决策点（开工拍板）

- **D1（B2 范围）**：**已拍板（2026-10-31）——删冗余栈实现 `scene.ClipStack/ClipState`，`Scene.ClipDepth/ClipBounds` 作为场景标准查询能力保留**，由 TagBeginClip/TagEndClip 序列派生实现（能力存留不以消费者数为删除依据；删除依据=实现冗余，PLAN §0/③ + AGENTS「已验证待接线是正常中间态」口径；评估原 a 项作废）。tileClipState 登记不统一。
- **D2（B3 方案）**：**已拍板——internal/blend 枚举改绑 render 值域删 2 表**（GetBlendFunc 算法保留只换入参）；**scene.BlendMode 保留线格式值域、只留 1 张边界表**（wire 不变、零重录）。
- **D3（B4 边界）**：按评估推荐执行——clipPresentDamage 并入 PlanFramePresent；idle 归一；resize 三层对齐 no-op+显式重配；SetPresentPolicy 留 embedder。
- **D4（I2 语义）**：**已拍板——改测试期望对齐同尺寸 no-op + 新增显式 ForceReconfigure 口**（对齐 Skia/Flutter；禁止为凑绿删断言）。
- **D5（C7）**：按解剖结论执行 hub FrameJob 方案。
- **D6（G2 GPUTexture 测试簇）**：按评估推荐——p0_format_readback_test.go 改写直连 hal，GPUTexture 家族删；RectAllocator 簇全表面搬移（含 UsedArea/Utilization/AllocCount/Reset/Contains 等标准能力方法，不裁剪）。
- **D7（顺序）**：**已拍板——e1（G2+J1/J2）→ e2（B2 clip）→ e3（B3 blend）→ e4（B4 present+C7/I2/C3+I1 解剖）**，每子步独立提交+全量门禁。
- **D2（B3 方案）**：internal/blend 收敛 a) 枚举改绑 render 值域删 2 表（推荐）；scene.BlendMode b) 保留线格式值域只留 1 张边界表（零重录，推荐）或 a') 改绑 render 值删 2 表（wire 变化，需重录受影响 recording）。
- **D3（B4 边界）**：clipPresentDamage 并入 PlanFramePresent；idle 归一；resize 三层对齐 no-op+显式重配；SetPresentPolicy 留 embedder（推荐全套）。
- **D4（I2 语义）**：a) 改测试期望对齐 no-op + 加显式 ForceReconfigure 口（推荐，对齐 Skia/Flutter）；b) 改产品语义让同尺寸重配。
- **D5（C7）**：按解剖结论执行 hub FrameJob 方案（唯一已知方向，无备选）。
- **D6（G2 GPUTexture 测试簇）**：a) p0_format_readback_test.go 改写直连 hal，GPUTexture 家族删（推荐）；b) 整簇搬移保留。
- **D7（顺序）**：e1→e4 如 §6（推荐）；每子步独立提交（接口全量切换制，原子切换）。
