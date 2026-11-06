P3.7 批 6 收官后 · 能力×真窗完备性审计（2026-11-06）
====================================================

**缘起**：用户问「每个功能能力是否都完整？每个功能能力都要有对应的真窗测试」，并明令
「先看建议里说的事是否是真实的。从源码看不要凭记忆」。本文件是审计结论 + 四条缺口的归位账
（按 AGENTS「新问题归位与跨线解决纪律」逐条立状态），全部判定都带 `文件:行号`，可复跑 grep。

**口径**：一条能力算「有真窗」= 有一个 `platform.Open` 真窗在**呈现帧**上画它，且有语义像素
断言 + 逐位金图。离屏 `render.NewContext` 单测按 AGENTS「生产与原子验证口径」只算**原子验证**，
不计入真窗。核查方法：`grep -rn` 全仓调用点 + 逐窗 README/证据的臂清单比对。

---

一、结论：六扇批 6 窗对 11 条收口能力全覆盖，另查出 4 条缺口（1、2 已补证，3 找到真触发子待定性，4 待拍板）
----------------------------------------------------------------------------------------

批 6 六扇（① 透视批绘 / ② 高度 z 序 / ③ 材质 / ④ mip / ⑤ 泛光 / ⑥ scene 流）对应的
11 条能力行都有窗内臂 + 语义像素 + 双后端金图，逐窗账见同目录 `window-*-*.md`。缺口在下面。

二、四条缺口（逐条：现象 / 位置 / 判定依据 / 归属 / 状态）
--------------------------------------------------------

### 缺口 1 · 投影下文字 fail-closed 无真窗证据 —— ✅ 已补（2026-11-06）

- **现象**：文字族在投影层之下应「整笔拒画 + `TextSkips`+1、`Pop` 后同一串字照常在场」，
  全仓没有任何一扇真窗验过。
- **判定依据（源码）**：`grep -rl PushProjection` 命中的示例只有
  `examples/engine/p37_persp_batch`（补臂前只画 quad）与 `examples/ui_wr_p37_scene`（只画 scene 流），
  两扇里 `PushProjection` 段内都没有一笔 `DrawString`。引擎侧底子是硬的：
  `render/p37_projection_stances_test.go:499`（文字族九个公开入口逐个「投影下 0 像素 + `TextSkips`+1」）、
  `render/p37_projection_test.go:505`（离屏导出对照）——但都是**原子验证**。
- **归属**：批 6 窗 ①（投影层四动词的真窗就是它）。
- **处置**：`paintProjectionLayer` 加两笔字（投影段 + `Pop` 后恒等段），新探针
  `text_under_projection_failclosed`（投影段条带墨差必须 0 且 `TextSkips ≥ 1`、恒等段必须 >40、
  字体取不到即红）；探针 14→15，双后端金图按「删基线→播种→复验」重录，H16-e 变异有牙齿。
  明细 `window-1-persp-batch.md` §七。

### 缺口 2 · 材质 uniform 载荷从不决定窗内画面 —— ✅ 已补（2026-11-06，臂 F）

- **现象**：材质能力在窗 ③ 里有五臂真窗证据，但**没有一帧画面是由 uniform 的内容决定的**：
  uniform 只出现在「校验」里（太大拒、太短拒），不出现在「出图」里。
- **判定依据（源码）**：`examples/engine/p37_material/main.go` 里 `SetUniformBytes` 三处——
  `:337`（`ShaderUniformBudgetMax+1` 拒成 `ErrShaderUniformTooLarge`）、`:343`（1 字节短包拒）、
  `:648`（顶格 65536B 设置成功）；窗内着色器 `effA`/`effStroke`/双纹理腿等只吃 `uv` 与
  `textureSample`，`effMaxBudget` 虽读 `u.vals[0].x` 但字节全零、且那段（`:642-674`）只读
  `ShaderBackendFails` 计数、无像素锚。引擎侧原子验证在案：`render/p37_3c_test.go:196-215`
  （`mix_level=1` 的材质区域 ≡ 第二张纹理，逐通道容差 1）、272B > 旧 256B 预算的 gles 真编译前置
  探针 `render/p37_3c_probe_gles_test.go:136`、`:202`（`MinBindingSize: 272`）。
- **归属**：批 6 窗 ③。
- **处置（✅ 已补，2026-11-06）**：③ 加臂 F——272B uniform（`array<vec4<f32>,16>` 占位
  256B，`split`@256B、`gain`@260B 全在旧预算之外）决定分屏边界与右半增益；
  同几何、同纹理、同着色器**只换字节**跑三案：`split` 0.25 / 0.75 / 0，
  固定像素必须跟着翻色（锚点与期望色写死在边界两侧，不随 `fSplit` 自算，免循环断言）。
  探针 13→**16**、语义像素 8→**10**，双后端金图删基线重录后 `16/16 0.0000%` 四连 +
  两基线逐位相同；H16 第四变异（`fSplit` 0.25→0.60）双探针红 + 金图 **0.875%**，还原复绿。
  明细 `window-3-material.md` §二 F 行、§四 四.4。
  **写这枚臂时撞上一条新的真实现场 ⇒ 直接推进缺口 3**（见下）。

### 缺口 3 · 后端真触发 —— ✅ 已收（③ 臂 G 窗内证据 + H23 修复，2026-11-06）

- **现象**：后端具名拒（`ErrShaderBackendCompile` / `ShaderBackendFails` 计数）这条腿，本机没能
  真复现一次「前端放行、后端拒」。
- **判定依据（源码 + 实测）**：③ `:642-674` 用顶格 64KB uniform 模块试过——本机 native 与 gles
  **都编译通过**，实测记在 `window-3-material.md:19`（`gles_budget_outcome=backend-accepted-on-this-device`）；
  计数与 stderr 接线由批 3d 的双 `%w` 包装钉覆盖（`render/internal/gpu/material_pipeline.go:298`、
  `:368`、`user_shader.go:252`、`:291`、消费侧 `render/context_gpu.go:640`）。
- **归属**：批 6 窗 ③ 的观测面（能力本身在引擎，属 3d 交付）。
- **新线索（补臂 F 首跑撞上，原样摘录 `gap3-backend-reject-probe.txt`）**：初版臂 F 的
  uniform 写成 `array<f32, 64>`——uniform 地址空间要求数组 stride 是 16 的倍数，
  **`render.NewShaderEffect`（前端）放行、wgpu-native 的 naga 在 `CreateShaderModule` 拒**。
  这就是本机第一枚「前端过、后端拒」的真触发子。但实测它**没走具名拒/回退那条路**：
  错误以 flush 失败冒出来（`build image resources: material: uniform buffer: wgpu: failed to
  create buffer: … In wgpuDeviceCreateShaderModule …`），同一跑里
  `ErrShaderBackendCompile` 命中 **0** 次、`window_shader_backend_fails=0`、
  `shader_backend_fails_probe=0`、`gpu_fallbacks=0`，且错误文本挂在 `CreateBuffer` 名下
  （naga 的报错串进了下一次调用的错误里）——整帧因此停机（`presents=190 < 240`）。
- **兑现（填充腿，2026-11-06 ③ 臂 G）**：故意模块挂填充腿画进窗里 ⇒ 区内保持底色 +
  `ShaderBackendFails++` + `CPUFallbackOps++`，左右两格是「画得出」的对照；终帧硬门
  `arm_g_frame_check` 把**两端各自钉死**（native：底色+计数 239；纯 Go GL：该模块被
  它自己的 WGSL 前端接受 ⇒ 出 shader 解析色 (128,130,128)+计数 0，二选一都不许是环境
  实心红）。实测数字、变异与逐点分歧见 `window-3-material.md` §七。
- **本轮自查出一条判据作废**：先前那份「三条量在两后端同形」的探针，`GPUI_BACKEND=go`
  那一轮其实仍跑在 Vulkan 上——离屏 `render.NewContext` 走 `OpenStandaloneDevice`，
  而它**按设计不吃 `GPUI_BACKEND`**（`render/device_open.go` 注记）。GL 端的真值只能
  从窗内设备量，已改由臂 G 的终帧门承担；三枚离屏探针改名 `standalone_leg_*` 只声明独立腿。
- **H23 已修（同日，账见 `h23-fix.md`）**：同一枚模块挂在**图像口/quad 口**上原本不走具名链——
  不计数、直接打断 flush、错误文本还归属漂到 `CreateBuffer`。修法=模块创建包进 hal 验证错误域
  （**A 档「换顺序」被实测证伪**：wgpu 的校验是排队的，换谁下一个调用谁背锅；B 档错误域实测
  既同步归位又就地吃掉）+ 构建期把那笔**整笔摘掉**（不是 CPU 回退，批 3b 契约：拒过的材质
  绝不裸画无材质像素）+ **按设备负面缓存**（首次才 `ShaderBackendFails++`、才打一次 stderr）。
  验收：新增 5 枚钉（先红后修）+ 3 条变异（N2 如实记为部分有效）；③ 双后端 `20/20 0.0000%`、
  ⑥ `15/15 0.0000% skips=0`、① `15/15 0.0000%`；`internal/gpu` 逐文件全绿 + `-race` DATA RACE=0。
  遗留（不在本修）：GL 端图像/quad 腿的拒收行为仍只在离屏 Vulkan 量过——GL 无 surfaceless
  独立设备（`render/device_open.go`），要真验得先给窗内腿造拒收用例。

### 缺口 4 · `engine/tex` 的采样档位到不了渲染层 —— ✅ 已收口（2026-11-07，档 A）

- **现象**：`engine/tex` 有一套完整的采样开关（`Filter{Near,Far,Mip,MaxAniso}`），但仓里没有
  任何一条路把它带进渲染命令；批 4 当年登记的理由是「`engine/tex` 无持档位对象」，**与实测不符**。
- **判定依据（源码）**：`engine/renderconv/` 全目录（`conv.go`/`doc.go`）对 `Mip|Filter` **零命中**；
  `engine/tex/mipmap.go:59-68`（`MipMode` 是 `render.MipmapMode` 的别名）、`:151-209`
  （`Normalized` 把非法 `Mip` 归一到 `MipNearest` 并折算出 `(nearLinear, farLinear, mipLinear, aniso)`）
  ——持档位对象就是 `tex.Filter`，"无持档位对象"这句登记作废。
  `engine/sprite/atlas.go:22-45` 走的是另一条：精灵自己直接持 `render.InterpolationMode` /
  `render.MipmapMode` 两个渲染层枚举（批 4 主跳交付），所以 sprite 路是通的。
- **归属**：非批 6 主线（能力归属＝批 4 采样档位的引擎侧接线）。**2026-11-06 用户拍档 A 开工，
  2026-11-07 收口**：`renderconv.FilterToRender`（混合轴/Unknown/aniso>1 具名拒绝三哨兵、
  Mip 原样透传不归一；红测先行先编译红后绿）+ ④ 臂 F（跨臂逐位 f0≡b0/f1≡a1/f2≡a2 diff=0 +
  拒绝红牌）+ ⑥ 面板 E（单图路 tex≡枚举逐位 + mip 三档互异）；双窗双后端金图 0.0000% 基线逐位
  同；变异有牙齿（linear 腿改投 nearest ⇒ ④ 双探针红+像素差 23110/23131，还原复绿）。
  正本：`gap4-tex-filter-assessment.md` / `gap4-window4-runs.txt` / `gap4-window6-runs.txt` /
  `gap4-mutation.txt` / `gap4-red-test.txt`。生产接线口径：消费者=两真窗面板（原子验证），
  引擎内部提交路查 `tex.Table` 属后续批。

---

三、复核命令（任何人可重跑，别信本文行的结论）
--------------------------------------------

```sh
for f in $(grep -rl "PushProjection" --include=*.go examples/); do grep -q DrawString "$f" && echo "$f"; done
#   补臂后只命中 ①（唯一在投影段里画字的窗）；补臂前该行为空 = 缺口 1 的原始判据
grep -n "SetUniformBytes\|ShaderUniformBudgetMax" examples/engine/p37_material/main.go
grep -rn "Mip\|Filter" engine/renderconv/*.go                          # 零命中 = 档位无转换路
grep -rn "ErrShaderBackendCompile" --include=*.go render/ | grep -v _test
```
