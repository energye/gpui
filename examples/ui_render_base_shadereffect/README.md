# ui_render_base_shadereffect

ShaderEffect 内省面直驱真窗（O12 批 1d，RENDER_FEATURE_COVERAGE §12 NONE→EX）。绘制面（SetShader 实画）在 p32/p37 窗，本窗钉内省/状态面。

## 覆盖面（logic 35）

- **卡 A 构造+绑定面**：`NewShaderEffect` + `ShaderBinding()→ShaderInterface`（UsesImage/TexturePairs/UniformBytes 与模块声明一致：uv-only=0/0/0、uniform struct=16B、image 对=pairs 1）；`WGSL()` 源往返；`ID()` 唯一/互异；`Closed` 生命周期；契约拒绝（缺 @fragment fs_main、uniform 非 struct、f64 标量进类型表）；上限常量 `ShaderUniformBudget/Max/MaxShaderImagePairs`=256/65536/8。
- **卡 B Uniforms/Gen**：`SetUniformBytes→Uniforms()` 读回、`SetFloatUniforms` f32 LE 打包等价、**Gen 状态变更自增**（gen0→1）、无声明模块 SetImage 容忍、Close 幂等。
- **卡 C 组合源**：`ComposedFillWGSL`（357B 含 fs_main/用户表达式）、`ComposedQuadWGSL`（3376B 引擎 36B 顶点布线展开）、`ComposedQuadWGSLFor(true)` bicubic 变体（params<4 时与 quad 同文）。
- **卡 D Context.SetShader 实画+读回**：SetShader→Shader() 同 effect、SetShader(nil) 清除、实画沿 uv.x 渐变（r 20→211）逐像素数学探针（c=(uv.x,0.3+0.4uv.y,1-uv.x)×0.9 premul）；`SetImageAt/ImageAt` slot 往返+超槽命名报错；`ErrShaderBackendCompile` 哨兵登记。

## 语义记录（不是洞）

- WGSL 契约：`@fragment fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32>`，uniform 必须 struct，@group(0)@binding(0)。
- **shader 内容只落 GPU 视图**（GPU-only face，不进 pixmap）：腿读回后须立即拷贝成独立缓冲，后续腿的 present 会覆盖视图内存；否则缩略图变黑。
- f64 拒绝只在类型表（struct/uniform 声明）触发；函数体内局部 `let x: f64` 被 naga-ir 消化不进类型表（实现语义，如实记录）。

## 门禁

```
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/ui_render_base_shadereffect -auto-only
go run ./examples/ui_render_base_shadereffect            # manual: 常驻
```

首播种 `GPUI_ACCEPT_GOLDEN=1`；卡内容改动需重播种。
