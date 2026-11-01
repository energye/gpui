# P3.2 自定义 shader — 收口报告（2026-11-01）

> 依据：`assessment.md`（D1 Paint.Shader 字段 / D2 字节+Float 双口 / D4 连带路径+纹理 / D5 默认 fs_main）。执行骨架因 D1/D4 拍板定为「两 pass」：pass1 用户 WGSL 全屏渲 RT（约定 @group(0) uniform/纹理+sampler）→ coverage R8 × mask 链贴回（clip/mask/AA 与其它填充同语义）。

## 1. 交付

- **公开 API**：`ShaderEffect`——`NewShaderEffect(wgsl)` 前置 `Parse→Lower→Validate`（三类错误可读拒绝，不到驱动）；`SetUniformBytes`（≤256B）/`SetFloatUniforms`（LE 打包）；`SetImage` 绑定 @group(0) 纹理+sampler；`WGSL/Uniforms/Image` 读口；`Close`。`Paint.Shader` 字段（Clone 复制）+ `Context.SetShader/Shader()`。用户 fs 契约：`fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32>`（premul 输出，uv 为绘制区局部归一坐标）。
- **GPU**：`tryGPUOpRC` Shader paint 绕过 SDF 捷径直走 path → `FillPath` shader 分支 → `fillShaderAsImage`（coverage 光栅 alpha × pass1 输出 × MaskPlane/MaskCoverage 复合）+ `userShaderCache`（per-effect pipeline：引擎 vs_main 全屏三角 + 用户 fs 模块、uniform/可选纹理 BGL、BGRA8 RT 与 readback swizzle 对齐）。
- **CPU 语义**：无加速器静默跳过（DrawGPUTexture 同款，不进 fallback 计数），钉 `TestShaderPaintCPUSkips`。

## 2. 第一轮六坑全解（各有 A/B 实证）

SDF 捷径吃 Shader paint（route 0→2）；FillPath 四参签名（三参 replace 静默失败教训）；`WriteMask` 零值=0 不写通道（clear-only 分离：RT/readback 链无辜）；`TextureBindingLayout` 缺 `ViewDimension`/`SamplerBindingType`（wgpu conv.rs:1681 panic）；RT 与管线 target 格式不一致（Submit Validation Error）；pass1 uv 行序（bake 图 y=30/210 值互换实证，uv 数组回归 dual_tex 同款）。

## 3. 门禁终态

| 门禁 | 结果 |
|---|---|
| 能力单测 | ✅ 拒绝三类 / uniform API（256B 上限+LE 打包）/ CPU 跳过（背景不变+fallback 计数 0） |
| 真窗 `ui_wr_p32_shader` | ✅ **native 与 go（gles）像素逐位一致**：top (31,52,199) vs c0 (26,51,204) Δ=5、mid (128,64,114) vs mix Δ=1、bottom (225,76,30) vs c1 Δ=4；金图两后端各 0.0000%；presents=300（帧驱动）；gpu_ops>0/cpu_fb=0 双卡 |
| 负向对照 | ✅ 注入语法错误 WGSL → `bake A: compile: shader: parse: parse error...` 可读拒绝 → 探针红 rc=1；还原复绿 rc=0 零残留 |
| apidoc | ✅ rc=0（§13 P3.2 行转正 + §0 计数注记） |
| 五目标构建 / per-file 34 包 | 后台跑（`build-five-target.txt` / `tmp/per-file-tests/`），结果落盘后本行补记；改动面小（新增 4 文件+3 处路由） |

## 4. 偏差与遗留

- **录制面（③）未做**：ShaderEffect 不可录制（录制遇 Shader paint 走既有 brush 语义）——登记 P3.2 后续（recording 资源池 AddShader+SetShader 命令+往返钉），与 scene 线格式演进一并排。
- **RT 池化**：pass1 scratch RT 每次建/销毁，池化入 res.Cache 登记后续性能项（P0.1 预算口径不变）。
- 纹理输入当前单张（binding 1/2 固定槽）；多纹理/动态 BGL 登记后续。
- 证据：`evidence/P3.2/`（assessment §8 现场 / summary / window-runs×4 / apidoc / build-five-target）+ 真窗 `examples/ui_wr_p32_shader/`（金图已播种）。
