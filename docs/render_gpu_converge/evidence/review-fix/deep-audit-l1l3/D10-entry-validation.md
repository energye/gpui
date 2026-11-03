# D10 入口校验只到语法（L2-P1-5）证据

## 复现（修前红，实测即 L2 报告形态）
`render/p32_shader_validate_test.go` 对新入口的六条，修前全部「收进来」（`go test -run TestD10` 六连红）：
无 fs 入口 / 纯 compute / vec3 返回 / f32 标量返回 / uniform 288B 超预算 / sampler 缺配对纹理（binding 2 无 binding 1）
/ 纹理在契约外槽位（binding 3）/ f64 uniform —— 全部通过 Parse→Lower→Validate 进缓存，
到绘制期才按后端分叉炸（native fb+stderr、gles 编译期各异、metal LowerWithSource 另路径）。
附带实验：`textureSampleLevel(samp, samp, uv, 0.0)` 这类非法 WGSL 前端也放行（记入疑点，不扩面）。

## 标准方案（对齐 Flutter FragmentProgram.forShader 编译期拒绝）
`NewShaderEffect` 语法三关后加 `validateShaderContract(ir.Module)`（复用同趟 Lower 产物，零额外编译）：
- 必须有 `@fragment` 入口 `fs_main`；返回必须 vec4<f32>（Vec4+ScalarFloat 直断）。
- 槽位表按契约固定：0=uniform struct（`ir.TypeSize`≤`ShaderUniformBudget`）、1=sampled float 纹理、2=sampler；
  契约外槽位直拒；sampler 无配对纹理直拒。
- 类型池扫 f64（struct 成员/vector/matrix/array/pointer 递归，seen 集防环）。
- 文件：`render/shader_effect.go`。

## 负向对照（逐腿）
- A f64 腿短路（`&& false`）：TestD10F64Uniform 红「got accept」→ 还原绿。
- B 入口只名不级（去 StageFragment）：两测试仍绿（名字本身不命中），**该腿无独立判别力，如实不主张**；
  stage 条件保留属防御性。
- D 预算腿短路：TestD10UniformOverBudget 红 → 还原绿。
- 语法级旧拒绝不受牵连：TestShaderEffectRejectsBadWGSL 前后均绿。

## 验后绿
- TestD10 全部 8 条 PASS；正例护栏 TestD10ValidModulesStillAccepted（5 个合法形态，含 d8/d9 复用源）PASS。
- shader 家族回归 25+ PASS（D8/D9/P32 泄漏/惰性/D6/DrawPaint 线）；`go vet` 净。
- 全量 34 包 per-file 与五目标在收口门禁统一跑（见 summary）。
