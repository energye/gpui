# P3.7 批 3d（引擎洞与错误分派）· 实施方案（2026-11-05，design.md §4.4 已在 P3.7 拍板）

## 1 影响面实测（全部到行号）

### 1.1 描边洞根因（比设计稿深一层的单点）

`isGPUSolidPaint`（`sdf_gpu.go:439-455`）不查 `paint.Shader`：`SetRGB+SetShader` 的画刷
`IsSolid()` 为真 ⇒ 判为 solid ⇒ `StrokePath :2276` 走 solid 分支（混合门通过）⇒
`fastStroke` eligible（`strokeSolidFastPathEligible :2260` 同样不查 Shader）⇒
`fillPathUser :2455` 画**环境实心色**，shader 静默丢弃——与 `SetShader` 文档
「fills and strokes」（`shader_effect.go:422`）矛盾（L2 深审登记）。

**修复单点**：`isGPUSolidPaint` 顶部加 `paint.Shader != nil → false`（shader 是颜色源，
永远不是 solid）。一处修，四处全通：
- `StrokePath :2276` → 非 solid 分支（SourceOver ✓）→ 轮廓展开 → `:2445 FillPath` →
  `fillShaderAsImage`（GPU shader 描边，AA、coverage、mask 链全继承）；
- `strokeSolidFastPathEligible :2261` 快路失格（冗余保险）；
- `fillPathUser :2455` 拒绝语义不变（non-solid 本就该走 FillPath）；
- `FillPath :2087` 掩码门永不触及（:2042 shader 检查在最前）。
Skia 对标：shader 对 stroke 同样生效（SkPaint 单 shader 槽）。

### 1.2 FillShape「同族洞」——**审计后如实收回（2026-11-05）**

草稿曾判断 `FillShape :2564` 存在同族洞（shader ⇒ fillBrushNative 必败 ⇒ CPU 画环境色）
并加了一镜像分支；负向 N2 变异钉不红，追因发现 **`tryGPUOpRC`（`context_gpu.go:553`，
深审已装）把 shader 画刷直达 pathFn——FillShape 从不接 shader 画刷，该分支是死代码**。
照「核实后发现没问题的，如实收回判断」纪律：删除死分支、本节留档收回。CPU 描边无洞：
落点 `outline → doFill`（`context_draw.go:500-510`）已有 P3.2 skip（`:343`）。形状路由
由 `TestP37DFillShapeWithShaderGradient` 钉住（标注为路由钉）。

### 1.3 后端失败点与计数口

`fillShaderAsImage` 的 pass1 失败（`user_shader_fill.go`，gles 首绘炸）现打 stderr +
返 `ErrFallbackToCPU`；传播到 `tryGPUFillWithMode`（`context_gpu.go:629-651`）记
`fill:<err>` fallback reason。计数口定在此处：err 链 `ErrShaderBackendCompile` ⇒
`RenderPathStats.ShaderBackendFails++`（新字段）。

## 2 七枚哨兵（design §4.4，全部 errors.Is 可判）

| 哨兵 | 接线点 |
|---|---|
| `ErrShaderBadWGSL` | `validateUserShader` 的 Parse/Lower/Validate 三步 + 组合产物重验 |
| `ErrShaderNoEntry` | `validateShaderContract`/`fragmentParams`：缺 @fragment fs_main |
| `ErrShaderBadReturn` | fs_main 返回类型非 vec4<f32> |
| `ErrShaderSlotConflict` | 绑定/配对违规（孤 sampler/texture、超对、越槽、参数前缀错） |
| `ErrShaderUniformTooLarge` | uniform 超 `ShaderUniformBudgetMax`（契约 + `SetUniformBytes`） |
| `ErrShaderClosed` | 关后拒用（`SetUniformBytes`/`SetImageAt`/`ensureUserShaderPipeline`/`ensureMaterialPipeline`/queue） |
| `ErrShaderBackendCompile` | 后端模块/管线创建失败 + `runUserShaderPass` 失败（**双 %w**：与 `ErrFallbackToCPU` 同链，既有 fallback 语义不破） |

与在册 M3 同族：本批只把「我方已能判的」变具名错，不冒充修完前端语义校验。

## 3 钉（先红后绿，`render/p37_3d_test.go`）

1. 描边+shader 像素钉：SetRGB(红)+SetShader(uv-gradient)+Stroke ⇒ GPU 腿画出渐变描边
   （描边环上沿 ≠ 下沿 + ≠ 平坦环境红）——修复前此钉红（环境实心）。
2. 形状+shader 像素钉：DrawCircle+Fill 同 shader ⇒ 渐变圆盘（≠ 环境色）。
3. 哨兵面 6 例：bad WGSL ⇒ ErrShaderBadWGSL、缺 fs_main ⇒ ErrShaderNoEntry、vec3 返回 ⇒
   ErrShaderBadReturn、孤 sampler ⇒ ErrShaderSlotConflict、uniform 超 ⇒ ErrShaderUniformTooLarge、
   关后 SetUniformBytes ⇒ ErrShaderClosed——全部 errors.Is。
4. `ErrShaderBackendCompile` 双链包装钉：构造 `fmt.Errorf("%w (%w)",…)` 断言两链同判
   （真机后端编译失败无确定性触发器——M3 家族反向工程不在本批，如实登记）。
5. 计数口钉：`RenderPathStats.ShaderBackendFails` 字段存在且默认零（触发路径无确定性
   构造器，字段+接线由哨兵包装钉与 fallback reason 文本覆盖）。
负向：N1 revert isGPUSolidPaint ⇒ 钉 1 红；N2 revert FillShape 分支 ⇒ 钉 2 红；
N3 摘哨兵包装 ⇒ 钉 3 红；N4（计数触发）不可观测，如实登记不冒充红（1a N3 先例）。

## 4 口径

- 修代码不改文档（`SetShader` 文档「fills and strokes」本就正确，是代码违约）。
- 修后存量金图：`ui_wr_p32_shader`（填充路）+ `ui_wr_p35_persp`（quad 路）复跑——描边
  修复只改「shader 画刷的描边路由」，无 shader 画刷路径逐位不变。
- 哨兵为公开 API（7 常量 + 1 计数字段）：目录 §3.16 同步 + apidoc。

## 5 交付记录（2026-11-05，未提交）

**落地清单**：
- 描边洞修复（单点）：`isGPUSolidPaint` 顶部 `paint.Shader != nil → false`（`sdf_gpu.go`）——
  shader 画刷永不判 solid ⇒ `StrokePath :2276` 走非 solid 分支 → 轮廓展开 → `FillPath` →
  `fillShaderAsImage`（GPU shader 描边，AA/coverage/mask 全继承）；`SetShader` 文档
  「fills and strokes」从违约变事实。`SetRGB+SetShader` 存量画刷：有 shader 时 shader 生效
  （语义变化=修复本身，存量无 shader 画刷路径逐位不变——isGPUSolidPaint 其余调用点
  `:2094/:2455/:2564` 对 nil-shader 行为不变）。
- 七枚哨兵：`ErrShaderBadWGSL/NoEntry/BadReturn/SlotConflict/UniformTooLarge/Closed/
  BackendCompile`（`shader_effect.go` 哨兵块），接线：validateUserShader 三步、契约缺入口/
  返回类型/绑定配对/预算、fragmentParams 参数面、SetUniformBytes/SetImageAt 关后与越槽、
  ensureUserShaderPipeline/ensureMaterialPipeline 模块与管线创建、`fillShaderAsImage` pass1
  失败（**双 %w** 与 ErrFallbackToCPU 同链，fallback 语义不破）。
- 计数：`RenderPathStats.ShaderBackendFails`（`context.go`）+ `tryGPUFillWithMode` 的
  `noteShaderBackend` 闭包（`context_gpu.go`，errors.Is 观察，skip 语义不变）。
- **审计收回一处**：FillShape「同族洞」为误判（tryGPUOpRC 深审已拦截，死分支已删）——
  照「如实收回」纪律留档本文件 §1.2 与 `negative.txt` N2。

**钉 4 枚全绿**（`render/p37_3d_test.go`）：描边+shader 渐变环（Δ≥60，修复前=环境实心）、
形状路由钉（tryGPUOpRC 直达 FillPath shader 腿）、哨兵六例 errors.Is + 双链包装、
计数字段零值。**负向 N1/N3 红还原复绿；N2 收回；N4 不可观测如实登记**（1a N3 先例）。

**门禁**：build/vet rc=0、触及 6 文件 gofmt 净、逐文件触及面 8 文件全绿（3d/3a/3b/3c/
depth_r5/draw_order/p32 两文件——既有描边/深度/盖序钉零回归）、`-race` ok 3.4s、
conformance 双后端 PASS、apidoc rc=0（目录补七哨兵+计数+描边洞修复行）、
真窗 `ui_wr_p32_shader` golden 0.0000%（填充路零变化）。

**口径**：材质腿仍零生产消费者，「已验证待接线」；哨兵族不冒充修完 M3；真触发
（ErrShaderBackendCompile）随批 6 材质窗 gles 腿承载。
