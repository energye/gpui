H23 影响面评估与方案（2026-11-06，开工前停报告版）
==================================================

被测洞（一句话）：一枚**前端放行、后端拒收**的用户材质模块，挂在**图像口/quad 口**上时
不走批 3d 的具名拒/回退链——既不计数也不具名，而是把整帧 flush 打断；而且每帧重新尝试
编译（无负面缓存）。填充腿那条路是对的（具名 + 计数 + 回退），已由 ③ 窗臂 G 做成窗内证据
（`batch6/window-3-material.md` §七）。

一、真值（全部实测，可复跑）
---------------------------
复现命令与逐字输出：`gap3-backend-reject-probe.txt`（三条量探针 + 填充腿像素实测）。

| 量 | 填充腿（SetShader+矩形） | 图像口 `DrawImageEx(Material:)` | quad 口 `DrawImageQuad(Material:)` |
|---|---|---|---|
| 前端 `render.NewShaderEffect` | 放行（err=nil） | 同左 | 同左 |
| 后端 | naga 在 `CreateShaderModule` 拒（uniform 数组 stride 4 非 16 倍数） | 同左 | 同左 |
| `FlushGPU()` 返回值 | **nil**（帧活着） | **error**：`build image resources: material: uniform buffer: wgpu: failed to create buffer: wgpu: CreateBuffer: Validation Error`，错误体里带的却是 `In wgpuDeviceCreateShaderModule` ⇒ **归属漂移** | 同左 |
| `ShaderBackendFails` | 0→**1** | 0→**0** | 0→**0** |
| `CPUFallbackOps` | 0→**1**（`last_fb="fill:shader: backend compilation failed (gg: falling back to CPU rendering)"`） | 0→0 | 0→0 |
| 画面 | 区内保持底色（环境实心红**没有**冒充回退） | 整帧停在报错前（③ 首轮实测 `presents=193<240`） | 同左 |
| 每帧代价 | 每帧重试编译 + 每帧一整段 stderr：p95 帧间隔 **23.4ms → 51.1ms** | 未测（帧已断） | 未测 |

跨端事实：纯 Go GL 的 WGSL 前端**不查** uniform stride 规则 ⇒ 同一枚模块在 GL 上被接受并出图
（③ 臂 G 终帧实测 (128,130,128)，与着色器输出式的解析值一致）。所以「真触发子」是
**Vulkan/wgpu-native 专属**，方案的验收必须两端各自钉（臂 G 已经这么做）。
⚠ 量这个的时候别用离屏 `render.NewContext`：它走 `OpenStandaloneDevice`，按设计**不吃
`GPUI_BACKEND`**（`render/device_open.go:71-77` 注记），只能量到 Vulkan；GL 端只能从窗内设备量。

二、代码现场（文件:行号）
------------------------
- `render/internal/gpu/material_pipeline.go:296` `CreateShaderModule` ⇒ 同步报错才包
  `fmt.Errorf("%w: material module: %w", ErrShaderBackendCompile, err)`（`:298`）；
- `render/internal/gpu/material_pipeline.go:301-309` 紧接着 `CreateBuffer`（uniform）⇒
  **naga 的排队错误在这一步冒出**，只包成 `"material: uniform buffer: %w"`，**没挂具名错**；
- `render/internal/gpu/render_session.go:4407-4423` 图像/quad 腿的材质循环：任何 err 都
  `return nil, err`（整批图像资源一起丢）；
- `render/internal/gpu/render_session.go:1864-1867` `buildImageResources` 的 err 再上抛成
  `build image resources: %w` ⇒ 帧级失败；
- `render/context_gpu.go:639-645` `noteShaderBackend`：填充腿的计数守卫，**只在拿到已具名
  错误时** `ShaderBackendFails++`（所以填充腿绿、图像腿黑）；
- 对照的**正确先例**：`render/context_image_draw.go:136-139` 图像口统一拒绝
  （投影层/blend/bicubic/CPU 模式全覆盖）= **不入队 + `MaterialSkips++` + 不重画无材质像素**；
  `render/internal/gpu/user_shader_fill.go:86-89` 填充腿双 `%w`（`ErrShaderBackendCompile`
  + `ErrFallbackToCPU`）；
- 缓存结构：`render/internal/gpu/material_pipeline.go:266-330` `ensureMaterialPipeline`
  的 `c.entries[effect]` **只在成功时写入** ⇒ 失败不记，于是每帧重来。

三、方案（对齐既有契约，不新造语义）
----------------------------------
拍板点 P-1（错误归位方式）：
- **A 档（推荐，最小手术）**：`ensureMaterialPipeline` 里把**管线创建提到 uniform 缓冲之前**
  （模块→管线→缓冲），让 naga 的排队错误落在**已经包具名错**的那一步；
  代价=一次顺序调整，零新接口。**这条顺序假设必须先红测验证**（若错误其实在更后的
  提交期才冒，就退 B 档）。
- B 档：用 hal 错误域（`gpu/webgpu/device_convert.go:106-129`、`gpu/rwgpu/errors.go:36/124`
  是真实现；`gwgpu/gles`、`metal` 是 stub——见其 `behavior_shared_test.go` 的 no-op 钉）
  包住模块创建并立刻 pop，把错误同步归位。语义最干净，但 stub 端拿不到保证，
  两端行为会分叉。
- C 档：错误文本嗅探（`Contains("wgpuDeviceCreateShaderModule")`）——**否**，脆且不可移植。

拍板点 P-2（图像/quad 腿的处置语义）：**拒画这一笔**（与 `context_image_draw.go` 的统一拒绝
同形：不入队、`MaterialSkips++`、区域保持底色），**不是** CPU 回退重画无材质像素
（批 3b 契约原文：a refused material draw must never repaint unmaterial'd pixels）。
填充腿保持现状（具名 + 计数 + 回退，实测已是正确形态）。

拍板点 P-3（负面缓存）：被后端拒的 (effect, device) 记一次永久失败（随 effect 关闭/重建失效），
之后：① 入口层直接按「已知不可用」走统一拒绝 ⇒ **不再每帧重试编译**（顺带收掉
p95 23.4→51.1ms 与每帧整段 stderr）；② 计数只在**首次**失败时 +1 还是一帧一次，待定（推荐
一帧一次，与现有 `ShaderBackendFails` 的"帧内可观察"口径一致，并另开累计键便于诊断）。

四、影响面
----------
- 改动落在 `render/internal/gpu`（材质管线 + 图像腿循环）与 `render` 的 `pathStats` 读写，
  不新增公开 API（`ShaderBackendFails`/`MaterialSkips` 已导出）；目录 §6.2/§3.17 无需改计数，
  但语义注记要补一句"后端拒收也走这条链"。
- 消费者：图像口/quad 口/批绘口三条材质腿 + `render/scene` 的回放腿（**H22**：回放腿 `_ =`
  吞返回值，与本洞独立，修完 H23 后 H22 才有可看的错误可打）；`engine/particle` 走的是
  atlas 老口（不带材质）⇒ 不受影响。
- 回归面：`render`（材质/投影/管线缓存族）、`render/internal/gpu`、`render/scene`、
  `render/recording` 触及包 + 真窗 ③（臂 A/E/F/G）与 `ui_wr_p37_scene`、`ui_render_base_scene`；
  按批口径再补 64 包逐文件 sweep + 五目标×{默认,nogpu} + `-race`。
- 风险：①顺序调整可能把错误推到更后的调用（B 档兜底）；②负面缓存要跟着 effect 的
  `Close`/重建走，否则换个同名模块会被误拒（红测里钉"关掉重开必须再试一次"）；
  ③多设备/多 session 下按 (effect, device) 记，不能全局记。

五、动手前的红测（缺一样就不改代码）
------------------------------------
1. `render` 包内测试：故意模块 + 图像口 ⇒ 断言 `FlushGPU()==nil`、
   `ShaderBackendFails>=1`、`MaterialSkips>=1`、区内保持底色、**其余内容照常画出**
   （同帧另一张无材质图必须在）。当前必红（实测就是上面那张表）。
2. 同上 quad 口一枚。
3. 负面缓存钉：连画 3 帧 ⇒ 模块创建尝试次数==1（包内计数钩子）；
   `Close()` 后换一枚合法模块 ⇒ 必须正常出图（防误拒）。
4. 顺序假设验证（P-1 A 档的前置）：临时把管线创建提到缓冲之前，看错误是否落在
   已包具名错的那一步——落得住才走 A 档，落不住转 B 档。
5. 端差异钉：GL 端（窗内）该模块被接受 ⇒ 出图且计数 0（臂 G 的 `arm_g_frame_check`
   已经钉住，回归时不许退化成单端断言）。
