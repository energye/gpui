# M3 解剖与修复：WGSL 前端放行非法 intrinsic 操作数

> 台账来源：§2.2 M3（D10 疑点转正，`evidence/review-fix/deep-audit-l1l3/D10-entry-validation.md:8`）。
> 诊断/修复日期 2026-11-11。用户拍板：A 降低点验参；修完接 K6。

## 现象与复现（2026-11-11 现行树实测）

`textureSampleLevel(samp, samp, uv, 0.0)`（sampler 当 texture 传）两种形态
（裸 sampler 模块 / 声明了纹理的模块）**Parse→Lower→Validate 全绿进 IR**
（入口 5 表达式），绘制期 GLSL 编译才炸。复现用临时测试已删（机制见下行号链）。

## 三关现状（修复前）

1. **降低关零验参**：`gpu/shader/wgsl/internal/lower/lower.go` `lowerTextureSample`
   （:14543-14598）args[0]/args[1] 直接降进 `ExprImageSample{Image, Sampler}`；
   `textureSampleLevel` 分派 :14314；Compare/ClampToEdge/Gather 同族同形态；
   类型判别机制现成（`isTextureArrayed/isTextureDepth` :14856/:14875，
   ident→全局→`Types[gv.Type].Inner`）但只用于形状判断。
2. **校验关只查句柄存在性**：`gpu/shader/ir/validate.go:427-445` `ExprImageSample`
   只查 `isValidExpressionHandle`，不查操作数类型（校验器无类型推导）。
3. **绘制关才炸**：GLSL codegen（`glsl/internal/codegen/expressions.go:1052`
   `writeImageSample`）直写 `textureLod(samp, samp, …)` → 后端编译期报错。

## 标准方案

本仓 `gpu/shader` 即 PLAN §1.1 拍板保留的 naga Go 移植；上游 naga 口径 =
WGSL 前端按内建签名验参（texture/sampler 操作数类型错位编译期报错），
wgpu 生态语义 =「模块校验期拒绝、绝不进 codegen」。D10 已在 render 侧做
契约三关（入口/返回/槽位），intrinsic 验参属前端层洞，按 AGENTS
「底层不满足→交底层」修 `gpu/shader`。

## 修复（降低点验参，用户拍板 A）

`gpu/shader/wgsl/internal/lower/lower.go`：

- `globalDeclaredInnerType`：ident→模块级全局变量→类型 Inner（复用既有
  ident→global→type 判别模式）；解析不了（局部变量/函数参数/复杂表达式）
  返回 ok=false，调用方**不拒绝**——零误伤。
- `checkTextureOperandTypes`：texture 调用家族统一入口检查——image 位
  （全部函数 arg0；`textureGather(component, t, s, …)` 特例 arg1）可解析且
  非 `ir.ImageType` 报错（sampler 当 texture）；采样族（Sample/Bias/Level/
  Grad/Compare/CompareLevel/BaseClampToEdge/GatherCompare）sampler 位
  （arg1；Gather 特例 arg2）可解析且非 `ir.SamplerType` 报错（texture 当
  sampler）。挂在 `lowerTextureCall` 分派头，一处覆盖全部 texture 内建。
- 非法示例报错形如 `textureSampleLevel: texture operand (argument 0) must
  be a texture, found sampler`。

## 钉测（先红后绿，`gpu/shader/wgsl/m3_texture_operand_test.go`）

| 用例 | 修前 | 修后 |
|---|---|---|
| `TestM3_SamplerAsTextureRejected`（裸 sampler / 声明纹理两形态） | 红（放行） | 绿 |
| `TestM3_TextureAsSamplerRejected`（tex 当 sampler） | 红 | 绿 |
| `TestM3_SamplerInTextureLoadRejected`（Load 族 image 位） | 红 | 绿 |
| `TestM3_LegalFormsStillAccepted`（合法 tex+samp） | 绿（护栏） | 绿 |
| `TestM3_TextureParamNotFalseRejected`（纹理经函数参数传递） | 绿（护栏） | 绿（不可解析跳过不误伤） |

负向对照：image 验参腿短路变异 → 拒绝用例转红 → 还原绿（零残留）。

## 门禁（2026-11-11）

- `gpu/shader/...` 9 包全 ok（含 lower 16k 行套件）；render 侧 shader 家族
  （TestD10 八关 + TestShaderEffect*）全绿。
- `TestP32ShaderLazyFirstGPUOp` 无 lib 时红：**存量环境形态非本批引入**
  （干净树 A/B 同红；带 `WGPU_NATIVE_PATH` 即 PASS，H1「无 GPU 轮」口径）。
- vet rc=0；windows/darwin 交叉构建 rc=0（纯 Go 无 build tag 面）；
  apidoc rc=0（wgsl 公开 API 无变化，检查在 internal/lower）。

## 处置插曲（如实登记）

负向对照还原时 `git stash pop` 误弹了别的线（p38/svg）的既有 stash
（`77008a8d`，WIP on e7ffd229），已做字节级还原：`git stash store` 原样回栈 +
16 个波及文件 checkout 回 HEAD；本线与对方线内容零丢失，状态与插曲前一致。
