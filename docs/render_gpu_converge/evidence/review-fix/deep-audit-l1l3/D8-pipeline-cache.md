# D8 shader pipeline 缓存首绘毒化（L2-P1-3）证据

## 复现（修前红）
`render/p32_pipeline_cache_test.go`，GPU 环境（CGO=1 + LD_LIBRARY_PATH + WGPU_NATIVE_PATH，DISPLAY=:0）：

- `TestD8UniformShrinkAfterWarmup`：warmup（uniform=32B）绿 → 缩到 16B 二绘
  `pass1 err: BindGroup … set_bind_group … invalid`，fb=1，像素 (255,255)=画漏（背景白）。
- `TestD8LateSetImage`：**首绘即红**——shader 声明了 binding(1)/(2) 纹理槽而运行时无图时，
  BGL 按运行时 `Image()!=nil` 建槽 → bind group 缺槽 → `set_pipeline … RenderPipeline invalid`，fb=1。
  （即 L2 报告的「缺图」腿，比报告写的更靠前：不必等 SetImage 变化，声明纹理+首绘无图就已毒。）
- `TestD8SetImageNilAfterWarmup`：warmup（有图）绿 → SetImage(nil) 二绘
  `set_bind_group … BindGroup invalid`，fb=1，像素 (255,255)。

## 标准方案
- 布局=模块声明派生（Skia SkRuntimeEffect「child 需求由编译产物决定」/naga IO 按模块声明建槽）：
  `render.ShaderEffect.ShaderBinding()` 在 NewShaderEffect 的 Parse→Lower→Validate 同趟
  从 `ir.Module.GlobalVariables` 读 @group(0)@binding(1) 的 ImageType → `UsesImage`（不可变）。
- uniform 槽：布局 `MinBindingSize = render.ShaderUniformBudget(256)`（契约上限，非实时长度），
  逐帧 bind 绑满整个 buffer（`pipe.uniformSize`，本就是 256 的向上取整）——尺寸变化只走内容 write。
- 声明未绑：缓存级 1×1 黑纹理+filtering sampler 占位（Dawn default-bindings / Impeller 模式），
  bind group 恒等于布局，不欠槽。
- 涉及文件：`render/shader_effect.go`（ShaderInterface/ShaderBinding/ShaderUniformBudget/declaresImageBinding）、
  `render/internal/gpu/user_shader.go`（black 三件套 + ensureBlackBindings + 布局派生 + uniformSize）、
  `render/internal/gpu/user_shader_fill.go`（逐帧绑定重排）。

## 负向对照（逐腿归因，均为修后代码上单独摘除）
- A `hasTex := false && …`（布局不派生）：两条 image 测试首绘即红 → 布局派生腿必要。
- B bind 纹理槽只在有图时加（无占位）：LateSetImage 首绘红、SetImageNil 二绘红 → 占位腿必要。
- C `MinBindingSize=max(len,16)`+bind 256：shrink 测试**绿** → wgpu 校验只查 bind≥max(声明,MinBindingSize)，
  布局毒化不独立致命，致命的是旧「bind=实时长度」。如实登记。
- D 布局 256 + bind 实时长度：全部首绘即红（16<256）→ 固定布局反过来要求绑满尺寸，两腿必须成对。
- 恢复全修：4/4 PASS。

## 验后绿
- `TestD8*` 4/4 PASS（含新增 `TestD8PlaceholderNoLeak`：占位单例，反复缺图绘制句柄数 31→稳）。
- shader 家族回归：`TestShader* / TestP32Shader* / TestP31DrawPaintLinearGPUDomain / TestD6*` 14/14 PASS。
- `go vet ./render/ ./render/internal/gpu/` 过；`go test ./render/internal/gpu/`（整包）ok 19.9s。
- 真窗（三证据之画面腿）：p32 native+go golden=0.0000% probes 3/3；p31 native golden=0.0000% probes 3/3。
- per-file 门禁口径：p32_pipeline_cache / p32_shader_effect / p32_shader_leak / d6_state_leak 四文件 PASS
  （日志 docs/render_gpu_converge/evidence/review-fix/per-file-tests/）。

## 存量 SIGABRT 对账（findings-register 约定）
整包 `go test ./render/` 仍在 `TestContextFrameDamage_MultipleFillsSeparateRects` 崩
`conv.rs:519 invalid bind group layout for pipeline layout descriptor`（与修前同点同签名）；
子集带 lib 单跑 ok。**D8 未消解它 → 非同族，登记为独立存量问题（vello accelerator 管线家族），待单开专项。**
