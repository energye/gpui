# P3.2 自定义 shader 影响面评估（2026-11-01）

> 任务：P3.2 自定义/运行时着色器（PLAN §5.1 首行 / catalog §13 P3.2 规划行）。依赖 P2.1 ✅（翻译器收敛）、P3.0 ✅。
> 对齐标准：Skia `SkRuntimeEffect`（WGSL/SkSL 片段 → SkShader → paint）/ Flutter `FragmentProgram`（WGSL 编译产物 + FloatUniforms → Paint.shader）。
> 调查：一路深查（Brush 体系/管线 uniform/执行面/翻译链/CPU 回退/真窗先例/安全口径），HEAD `76f8723f`。

## 1. 关键现状

1. **Brush 体系**：`Brush` sealed interface 带 `ColorAt(x,y)`（brush.go:28-37）；`CustomBrush`（ColorFunc 闭包）是 CPU 逐像素占位，`brush_native.go:205-208` 注释明确「Arbitrary Func cannot be compiled to WGSL; true fragment ColorAt remains deferred」——用户 shader 正是该 deferred 能力。
2. **翻译链双后端已通**：gles `CreateRenderPipeline` 对任意 WGSL 自动 Parse→Lower→GLSL（gles/shader.go:139-205，缓存键含完整源）；webgpu WGSL 直通（device_create.go:170-194）。**后端零改动**。gles 链未调 `shader.Validate`——坏 WGSL 失败点推迟到驱动编译期，用户入口必须先 Validate。
3. **执行骨架**：三条离屏链可复用——dual-tex（tex+tex+sampler+uniform BGL，GPUShared.ensure 模式）、mask 链（CPU 光栅 src+R8 mask，brush 仍 CPU 采样，不合适）、**filter graph**（tex-in/out、uniform slab 128B/256B stride、RT 池 res.Cache、ping-pong、publish+DrawGPUTexture 零回读）。新独立 cache 仿 `dualTexBlendCache.ensure`（dual_tex_blend.go:418-566）最干净——filter graph 的 mode 是封闭枚举，改它侵入大。
4. **uniform**：现状惯例=uniform slab + 动态 offset（256B stride 三处先例）；push constant gles 不支持（capabilities MaxPushConstantSize=0）。
5. **CPU 回退语义**：GPU-only 先例=`DrawGPUTexture` 在 CPU 模式静默跳过不计 fallback（context_image_gpu.go:133-139）；若走 ErrFallbackToCPU 会落 FuncPainter 无 ColorFunc=空输出（错语义）。测试层先例=`requireNativeGPU` skip + `gpu_ops>0 && cpu_fallback_ops==0` 断言。
6. **错误路径全无 panic**：Parse/Lower/Validate/glsl.Compile/GL link 均返回 error（gles/shader.go:157-183）；ensure 失败=该次绘制跳过，进程不崩。

## 2. 能力定义（草案）

**自定义 shader**：应用层传入 WGSL 片段模块（指定 entry point），配 uniform 数据，在 GPU 上对绘制区域逐像素求值输出颜色。编译在引擎入口完成（Parse→Lower→Validate 显式拒绝，错误可读）；gles 经既有翻译链 WGSL→GLSL；不支持的执行条件（CPU 模式/设备丢失）显式跳过并计数，不伪装 fallback。

- **第一版范围（拍板 D4）**：矩形区域填充（UV=(局部像素/尺寸)∈[0,1]，对齐 Flutter FragmentProgram 的 uv 语义）；uniform=一个用户 buffer（256B 槽位写入惯例）；WGSL-only。
- **不做（后续）**：路径填充（需 coverage 合成，mask 链改造）、采样 dst（混合类，dual-tex 骨架预留）、图片输入绑定、CPU 求值（WGSL 无 CPU 解释器，显式不支持）、类型化 uniform API。

## 3. 公开 API 草案（拍板 D1/D2）

```go
// render/shader_effect.go（新）
type ShaderEffect struct { ... }           // 编译产物：已 Parse→Lower→Validate 的模块缓存
func NewShaderEffect(wgsl string) (*ShaderEffect, error)   // 默认 entry "fs_main"？或必须命名入口
func (e *ShaderEffect) SetUniformBytes(data []byte)        // 字节布局，256B 内
// 绘制口（D1 拍板）：
func (c *Context) DrawShaderRect(e *ShaderEffect, x, y, w, h float64)  // UV 按矩形局部坐标
```

- D1 形态：**独立绘制口**（推荐）vs `ShaderBrush` 进 Brush 体系（Brush 接口 `ColorAt` 无 CPU 语义，重组 Brush+recording 两个 switch，面大）vs Paint.Shader 字段（Skia 同位，但第一版仅矩形口，挂 Paint 语义过宽）。
- D2 uniform：**`SetUniformBytes` 字节 API**（推荐，最薄）vs Float 类型化（`SetFloatUniform(i, ...)`，Flutter 式；第一版可加 `SetFloatUniforms(f []float32)` 便捷口）。
- shader 模块约束（能力定义锁）：fragment-only、entry 可指定（默认 `main`？定名拍板随 D2）、uniform block ≤256B、禁止 push_constant、采样器/纹理绑定第一版不开放。

## 4. 实现面（GPU 骨架）

- 新 `render/internal/gpu/user_shader_cache.go`：仿 `dualTexBlendCache.ensure`——`CreateShaderModule(用户 WGSL)` → BGL(sampler+uniform+dst? 第一版无纹理：BGL=uniform only) → pipeline（fullscreen triangle vs_main 引擎提供，用户仅 fragment）→ RGBA8 RT（res.Cache 池）→ 结果 `QueueImageDraw`/publish 贴回目标（零回读可选 publish 先例）。
- 入口校验：`shader.Parse→Lower→ir.Validate`（gpu/shader/shader.go:43-112 全备）；gles 建管线自动翻译。
- CPU 模式：`CPUOnlyMode()`/无加速器 → 跳过+独立计数（仿 DrawGPUTexture 先例），`recordCPUFallbackReason("shader:gpu-only")` 观察。
- 录制面：`CmdDrawShaderRect`（记录 effect 引用+rect+uniform 快照）+ `Backend` 契约 +1 方法（P3.1 先例）。

## 5. 验证方案（草案）

- 能力单测（先红后绿）：①入口坏 WGSL 显式拒绝（Parse/Lower/Validate 三类错误各一）②合法 shader 编译+CPU 模式跳过计数③rect UV 语义（CPU 钉不可行→GPU 像素钉 requireNativeGPU）④录制往返⑤uniform 传递（写值→shader 输出对应色）。
- 真窗：`examples/ui_wr_p32_shader`——三卡（UV 渐变棋盘 uniform 驱动色相；坏 uniform 显式失败态；CPU 模式跳过探针），双后端，金图，负向对照=注入非法 WGSL→窗口拒绝态红→修复合法绿。
- 双后端：native WGSL 直通；gles 经翻译链（**重点验**：翻译产物语义一致——同一 shader 双后端像素 ±2）；metal darwin 交叉编译。
- 回归：默认零变化（无 shader 绘制路径不变）；conformance+per-file 改动面。

## 6. 拍板项

| # | 问题 | 推荐 |
|---|---|---|
| D1 | API 形态 | 独立绘制口 `NewShaderEffect + Context.DrawShaderRect`；Brush 体系不动 |
| D2 | uniform API | `SetUniformBytes` 字节口 + `SetFloatUniforms([]float32)` 便捷口 |
| D3 | 执行骨架 | 新独立 `userShaderCache`（仿 dualTex ensure），不动 filter graph 封闭枚举 |
| D4 | 第一版范围 | 矩形+UV+uniform buffer；路径/纹理绑定/dst 采样/CPU 求值不做 |
| D5 | entry 命名 | 默认 entry `fs_main`（与仓内 15 个内建 WGSL 一致），`NewShaderEffect(wgsl)` 不带 entry 参 |

## 7. 风险

- gles 翻译对用户 WGSL 的兼容面（lower 对自由代码的覆盖度）——真窗双后端像素对照直接暴露。
- uniform 对齐（WGSL uniform 布局规则 vec4 对齐）——字节 API 文档锁 16 字节对齐约定+单测钉。
- 安全：Validate 前置后，非法 WGSL 不达驱动；资源释放=effect.Close→ShaderModule.Destroy（hal 接口已备 api.go:238）。
