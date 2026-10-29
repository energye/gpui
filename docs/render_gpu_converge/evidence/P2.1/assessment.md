# P2.1 影响面评估（开工前，2026-10-29）

> 性质：**只评估、不动工作区**。结论来自两棵树的逐文件比对、基线 API/测试计数表对账，以及一个一次性快照探针（跑完已删，源码固化在 `tools/shader_snapshot.go.txt`）。
> 结论一句话：`gpu/gwgpu/naga` 不是 `gpu/shader` 的“精简副本”，而是**更新的一份实现**（多能力校验、swizzle 赋值、MSL 括号修复、64 位原子校验）；`gpu/shader` 独有的 140 个文件全是 SPIR-V/HLSL/DXIL 后端与它们的测试。合并方向按 PLAN 走“naga → `gpu/shader`”，等价性用 **GLSL/MSL 逐字节快照 + GL 离屏像素 md5** 证明；删的只是三条无人使用的后端链。

## 0 结论摘要

| 项 | 结论 |
|---|---|
| 两棵树关系 | 同名 138 文件：128 个只差 import 路径，**10 个有真实内容差，方向全是 naga 更新**；`gpu/shader` 独有 140 文件（57 源 + 83 测试，126,048 行）= SPIR-V/HLSL/DXIL 后端 + 根 SPIR-V API；naga 独有 4 文件（1 源 + 3 测试，1,840 行）= 能力校验与两个新特性测试 |
| 合并方式 | 以 naga 为准整体覆盖到 `gpu/shader`（改 import 前缀与根包名 `naga`→`shader`），再删 `gpu/shader` 独有后端与 `gpu/gwgpu/naga` 目录；保留包集合 11 个 |
| 调用面 | 8 个非测试文件 + 7 个测试文件改 import；`render/internal/gpu/shader_helper.go` 的 `CompileShaderToSPIRV` 是仓内 SPIR-V 唯一消费者且 **0 调用**，随 SPIR-V 删除 |
| 包/API | 41 包 → 11 包；路径限定符号 1386 → 428（−962/+4）；`gpu/shader` 根删 5 个 SPIR-V 符号，`ir` 增 4 个符号 + 4 个常量，根增 `LowerWithCapabilities` + 4 个常量；`render/internal/gpu` 删 1 个死函数 |
| 测试计数 | 两树合计 240 文件 / 5850 Test / 68 Bench / 4 Example → **70 / 2139 / 21 / 0**（−170 / −3711 / −47 / −4，逐包有账，见 §3） |
| 快照预演 | 27 个 WGSL / 48 入口点 × {GLSL 330/430/450/ES300/ES310/ES320 + MSL 默认/mslmap} = 681 个输出文件；同一棵树跑两遍逐字节一致；4 条“预期版本错误”也纳入快照（原子计算 shader 在 GL330/ES300） |
| 生产等价证据 | GLSL/MSL 输出逐字节 + `shader_pipeline_offscreen_test.go` 离屏像素 md5 与 P1.3 基线一致（`59ca40337ff5e016579ea53a09549824`） |
| 能力是否有新替代 | **全部有**：GLSL/MSL/IR/WGSL 四条链是同一份代码换路径；SPIR-V/HLSL/DXIL 三条链无目标、无消费者（§0.1） |
| 待用户拍板 | 4 项，见 §8 |

## 0.1 能力替代核对（每个老实现，新路径拿什么接）

| # | 老载体 | 能力 | 新载体 | 结论 |
|---|---|---|---|---|
| 1 | `gpu/shader/wgsl/internal/parser` | WGSL → AST | naga 同包（多 `enable` 指令解析，`enable swizzle_assignment`） | 已替代（同代码 + 新特性） |
| 2 | `gpu/shader/wgsl/internal/lower` | AST → IR | naga 同包（多 swizzle 赋值展开，+672 行） | 已替代 |
| 3 | `gpu/shader/ir` | IR 定义与校验 | naga 同包 + `capabilities.go`（f64/i64/u64/f16 能力位；`Validate` 默认 `CapAll`，行为与旧 `Validate` 等价） | 已替代（新增能力校验，默认路径不变） |
| 4 | `gpu/shader/glsl` | IR → GLSL | naga 同包（逐字节同一实现，仅 import 路径不同） | 已替代 |
| 5 | `gpu/shader/msl` | IR → MSL | naga 同包（多两处修复：二元表达式 swizzle 括号、64 位原子 min/max 校验） | 已替代（naga 更新） |
| 6 | `gpu/shader/spirv` + 根 `Compile*`/`GenerateSPIRV` | IR → SPIR-V | **无替代，也不需要**：WebGPU 后端直接用 WGSL 建 module（`gpu/webgpu/device_create.go:182` 走 `CreateShaderModuleWGSL`）；`desc.SPIRV` 分支只接受外部传入，仓内唯一生产者 `CompileShaderToSPIRV` 0 调用 | 删除（无目标、无消费者，不是能力损失） |
| 7 | `gpu/shader/hlsl` + `gpu/shader/dxil` | IR → HLSL / DXIL | **无替代**：树内没有 D3D12/Windows 渲染目标，全仓无 importer（`grep -rn 'gpu/shader/\(spirv\|hlsl\|dxil\)'` 除自身外为空） | 删除（同一结论） |
| 8 | `gpu/shader/internal/dxcvalidator`（含 bitcheck） | Windows DXC 校验 | 只被 `dxil/validate.go` 使用，随 DXIL 一起删 | 删除 |
| 9 | `gpu/shader/internal/backend`（interface_order/sig_pack） | 后端接口序号与签名打包 | 只被 hlsl/dxil 使用（naga 侧此包 0 使用者） | 删除 |
| 10 | `gpu/shader/internal/registry`、`internal/textutil` | 标识符注册、文本工具 | 被 lower / glsl / msl 使用（naga 侧同样使用） | 保留 |
| 11 | 根 `Compile`/`CompileWithOptions`/`DefaultOptions`/`CompileOptions` | WGSL→SPIR-V 便捷 API | 随 #6 删除；根保留 `Parse`/`Lower`/`LowerWithSource`/`Validate`，并入 naga 的 `LowerWithCapabilities` | 删除 + 平移 |
| 12 | GLES 生产链 `compileWGSLToGLSL`（`gles/shader.go:139`） | 运行时 WGSL→GLSL | 同一实现，`naga.Parse/Lower` → `shader.Parse/Lower`，`glsl.Compile` 路径不变 | 已替代（仅改限定符） |
| 13 | Metal 生产链 `CreateShaderModule`/`compileLibrary`（`metal/device.go:544`） | 运行时 WGSL→IR→MSL | 同一实现，import 路径改 `gpu/shader{,/ir,/msl}` | 已替代（仅改限定符） |

## 1 方法与可复现命令

```bash
# 1) 文件级比对（只读）
for d in gpu/shader gpu/gwgpu/naga; do find $d -name '*.go' | sed "s#^$d/##" | sort > /tmp/$(basename $d)_files.txt; done
comm -23 /tmp/shader_files.txt /tmp/naga_files.txt   # gpu/shader 独有 140
comm -13 /tmp/shader_files.txt /tmp/naga_files.txt   # naga 独有 4

# 2) 把 import 前缀归一后逐文件比内容，找出“真实内容差”
sed 's#gpu/gwgpu/naga#gpu/shader#g' gpu/gwgpu/naga/<f> | diff - gpu/shader/<f>

# 3) 基线计数对账（P1.5 快照）
python3  # 按 baseline/api/test_counts.tsv 汇总两棵子树（见 §3 数字）

# 4) 快照探针（一次性，跑完删除；源码已固化）
cp tools/shader_snapshot.go.txt .p21_snapshot_tmp/main.go   # 先在 naga 树上跑 before
go run ./.p21_snapshot_tmp -out /tmp/p21_before
# 迁移后用同一份源码 sed 成 gpu/shader 导入再跑 after，diff -r 两份输出
```

## 2 两棵树逐类对比

### 2.1 规模

| 树 | Go 文件 | 其中测试 | 总行数 | 非测试行数 |
|---|---|---|---|---|
| `gpu/shader` | 278 | 160 | 239,381 | 118,283 |
| `gpu/gwgpu/naga` | 142 | 80 | 116,060 | 54,570 |
| 同名（合并对象） | 138 | — | shader 113,333 / naga 114,220 | — |
| shader 独有 | 140（57 源 + 83 测试） | 83 | 126,048 | — |
| naga 独有 | 4（1 源 + 3 测试） | 3 | 1,840 | — |

### 2.2 138 个同名文件：128 个只差 import、10 个有真实内容差（方向全是 naga 更新）

| 文件 | shader 侧独有行 | naga 侧独有行 | 内容 |
|---|---|---|---|
| `wgsl/internal/lower/lower.go` | 11 | 672 | swizzle 赋值 / 链式 swizzle 展开；移除一处 `zeroLit == nil` 早退 |
| `wgsl/internal/parser/parser.go` | 12 | 44 | `enable` 指令不再是“跳过”，落成 `Module.Enables` |
| `ir/validate.go` | 6 | 54 | `ValidateWithCapabilities` + 标量能力检查（`Validate` 默认 `CapAll`） |
| `msl/internal/codegen/statements.go` | 0 | 72 | 64 位原子操作 MSL 能力校验 |
| `msl/internal/codegen/expressions.go` | 0 | 13 | 二元/选择表达式 swizzle 加括号（C++ `.` 优先级陷阱） |
| `naga.go` | 93 | 42 | 根 API 差异：naga 去掉 SPIR-V 的 `Compile*`/`GenerateSPIRV`，加 `LowerWithCapabilities` 与 4 个能力常量 |
| `wgsl/wgsl.go` | 1 | 11 | `convertLowerResult` 抽取（行为不变） |
| `msl/internal/codegen/msl_integration5_test.go` | 0 | 60 | 上述括号修复的回归测试 |
| `msl/internal/codegen/msl_xcrun_test.go` | 2 | 44 | 64 位原子 min/max 的 xcrun 编译测试 |
| `msl/internal/codegen/xcrun_helper_test_darwin.go` | 1 | 1 | 一行 `//nolint:gosec` |

> 判断依据：这 10 个差异全部能在 naga 的提交历史里找到对应动作（`e1ac0cc4` naga 最小闭包、`943ad551` metal 自养、注释收敛）；`gpu/shader` 侧没有任何一处“更新但 naga 没有”的实现。

### 2.3 非 Go 资产 / 构建标签

| 项 | 处置 |
|---|---|
| `gpu/shader/internal/dxcvalidator/bitcheck/testdata/golden-dxc/*.dxil`（2 个，已跟踪） | 随 dxcvalidator 删除 |
| `gpu/shader/tmp/*.dxil`（9 个）、`gpu/shader/spirv/internal/tmp/*`（2 个） | 未跟踪、被 `.gitignore: tmp/` 忽略，属本地产物；删目录即可，不产生 git 变更 |
| `//go:build windows`（dxcvalidator 3 文件） | 随包删除；无其他 Windows 专用源码受影响 |
| `//go:build darwin` / `!darwin`（MSL xcrun 测试助手） | 保留（naga 同名） |

## 3 逐包处置与测试计数账

保留 11 包（迁移后计数 = naga 侧计数）：`gpu/shader`（根，0 测试）、`glsl`（0）、`glsl/internal/codegen`（14/496/3）、`internal/registry`（1/18）、`internal/textutil`（1/4）、`ir`（15/331/5）、`msl`（0）、`msl/internal/codegen`（18/529/3）、`wgsl`（2/8）、`wgsl/internal/lower`（15/656/10）、`wgsl/internal/parser`（4/97）。

删除 30 包（41 → 11），测试计数（文件/Test/Bench/Example）取自 `baseline/api/test_counts.tsv`：

| 删除对象 | 计数 | 说明 |
|---|---|---|
| `gpu/shader` 根的两个测试文件 | 2 / 16 / 10 / 0 | `bench_test.go`、`naga_test.go` 只测 SPIR-V `Compile` |
| `gpu/shader/dxil` 及 8 个子包 | 30 / 568 / 0 / 0 | DXIL 后端整链 |
| `gpu/shader/hlsl` 及 codegen | 22 / 474 / 9 / 0 | HLSL 后端整链 |
| `gpu/shader/spirv` 及 codegen | 29 / 366 / 7 / 4 | SPIR-V 后端；4 个 Example 即 G1 里那 6 个读不存在路径的空转测试所在包 |
| `gpu/shader/internal/backend` | 4 / 46 / 0 / 0 | 仅 hlsl/dxil 使用 |
| `gpu/shader/internal/dxcvalidator` + bitcheck | 6 / 54 / 0 / 0 | 仅 dxil 使用 |
| `gpu/gwgpu/naga` 整目录 | 80 / 2239 / 21 / 0 | 实现整体迁入 `gpu/shader` |
| **带入的新增测试** | **+3 / +52 / 0 / 0** | `ir/validate_capabilities_test.go`（+9）、`msl/codegen/int64_atomics_test.go`（+5）、`wgsl/lower/swizzle_assignment_test.go`（+38） |

合计：240/5850/68/4 → 70/2139/21/0，净 **−170/−3711/−47/−4**。删除的 3711 个测试全部属于 SPIR-V/HLSL/DXIL 三条无目标后端链；WGSL→IR→GLSL/MSL 的能力测试一个不少（naga 侧那 2139 个全保留）。

### API 快照账（路径限定符号，沿用 P2.3 口径）

- 包：41 → 11。
- 符号：1386 → 428（removed 962 / added 4）。其中 962 里绝大部分是 naga 包路径迁移（实现没丢）；真正随包消失的是 shader 侧 SPIR-V/HLSL/DXIL/dxcvalidator/backend 与根 SPIR-V API。
- 明确删除的公开符号：`gpu/shader` 根 `Compile`、`CompileWithOptions`、`DefaultOptions`、`GenerateSPIRV`、`CompileOptions`；`render/internal/gpu` 的 `CompileShaderToSPIRV`（仓内 0 调用）。
- 明确新增：`ir.Capabilities`、`ir.CapFloat64`、`ir.CapShaderInt64`、`ir.CapShaderFloat16`、`ir.CapAll`、`(*Capabilities).Contains`、`ir.ValidateWithCapabilities`、根 `LowerWithCapabilities` 与 4 个同名常量（`CapFloat64`/`CapShaderInt64`/`CapShaderFloat16`/`CapAll`）。

> 这个 API diff 超出 §7.7 字面白名单的部分只有两类：①根 `Compile*` 随 `spirv/` 删除的连带删（SPIR-V 无目标）；②`CompileShaderToSPIRV` 随 SPIR-V 删除的连带删。两项都需要在 §8 拍板。

## 4 调用面迁移表（8 非测试 + 7 测试 + 1 死函数）

| 文件 | 现 import | 迁移后 | 符号变化 |
|---|---|---|---|
| `gpu/gwgpu/gles/capabilities.go` | `naga/glsl` | `gpu/shader/glsl` | 无（`GLSLVersionToNaga` 函数名暂不动，见 D2） |
| `gpu/gwgpu/gles/device.go` | `naga/glsl` | `gpu/shader/glsl` | 无 |
| `gpu/gwgpu/gles/device_linux.go` | `naga/glsl` | `gpu/shader/glsl` | 无 |
| `gpu/gwgpu/gles/resource.go` | `naga/glsl` | `gpu/shader/glsl` | 无 |
| `gpu/gwgpu/gles/shader.go` | `naga` + `naga/glsl` | `gpu/shader` + `gpu/shader/glsl` | `naga.Parse/Lower` → `shader.Parse/Lower` |
| `gpu/gwgpu/metal/device.go` | `naga` + `naga/ir` + `naga/msl` | `gpu/shader` + `.../ir` + `.../msl` | `naga.Parse/LowerWithSource` → `shader.*` |
| `gpu/gwgpu/metal/resource.go` | `naga/ir` | `gpu/shader/ir` | 无 |
| `gpu/gwgpu/metal/mslmap/mslmap.go` | `naga/ir` + `naga/msl` | `gpu/shader/ir` + `.../msl` | 无 |
| `gles/{shader_cache,version,shader_pipeline_offscreen,binding}_test.go` | `naga/glsl`/`naga/ir` | `gpu/shader/glsl`/`gpu/shader/ir` | 无 |
| `metal/{compute,encoder}_test.go` | `naga/ir` | `gpu/shader/ir` | 无 |
| `metal/mslmap/mslmap_test.go` | `naga` + `naga/msl` | `gpu/shader` + `.../msl` | `naga.Parse/LowerWithSource` → `shader.*` |
| `render/internal/gpu/shader_helper.go` | `gpu/shader`（根） | **删除 import** | 删 `CompileShaderToSPIRV`；`CreateShaderModule`/`GPUResources` 不动 |
| `gpu/shader/msl/internal/codegen/msl_xcrun_test.go`（树内） | naga 根 | 迁移后自动变 `gpu/shader` 根 | `naga.Parse` → `shader.Parse`（无 import 环：迁移后根只依赖 `wgsl`） |

## 5 快照方案与预演结果

- 工具：`tools/shader_snapshot.go.txt`（源文件存 `.go.txt`，不进入 `./...`；迁移前用 naga 导入跑 `before`，迁移后 `sed 's#gpu/gwgpu/naga#gpu/shader#g'` 跑 `after`）。
- 语料：全仓 27 个 `.wgsl`（`render/internal/gpu/shaders/` 15、`tilecompute/shaders/` 10、`gles/testdata/` 2），共 48 个入口点——即 GLES/Metal 生产路径实际编译的全部 shader。
- 矩阵：
  - GLSL：`Version330/430/450/ES300/ES310/ES320` × 48 入口点，选项取生产原样（`ForceHighPrecision`、`WriterFlagAdjustCoordinateSpace|ForcePointSize`、空 BindingMap），共 288 次编译（284 成功 + 4 条预期版本门控错误：`coarse`/`path_count` 含原子操作，GL330 与 ES300 不支持）。
  - MSL：`DefaultOptions()` 与 `mslmap.Options(module, 30)`（runtime-sized array 的生产路径）各 27 个模块，共 54 份输出。
  - 每份成功输出落 `源码` + `info.json`（TranslationInfo / msl Info 的稳定 JSON）。
- 预演：同一棵树连跑两次 `diff -r` 为空；输出 681 个文件。
- 迁移后的判定：`diff -r before after` 必须为空（含 4 条错误文本）。任何字节差都按“行为变化”处理，不得放行。

## 6 门禁设计

| 门禁 | 做法 | 预期 |
|---|---|---|
| 五目标交叉编译 + `-tags nogpu` | 复用 P2.3 `tools/build-gate.sh` 口径（linux amd64/arm64、windows amd64、darwin amd64/arm64） | 与 P2.3 相同的绿色集合；删除 dxcvalidator 不影响任何目标（无 importer） |
| `go vet` | 按 P1.2b 过滤后的包列表逐包跑（不用 `go vet ./...`） | 61 / 58，与 P2.2/P2.3 逐条一致（被删包当前 vet 无告警，已实测 `go vet ./gpu/shader/... ./gpu/gwgpu/naga/...` 为空） |
| 逐测试文件 | 保留 70 个 shader 测试文件 + `gpu/gwgpu/gles`、`gpu/gwgpu/metal`、`render/internal/gpu` 全部测试文件，逐个 `go test` | 0 FAIL；GL 离屏测试像素 md5 仍为 `59ca40337ff5e016579ea53a09549824` |
| 快照 | §5 | `diff -r` 为空 |
| 残留 grep | `gpu/gwgpu/naga`、`gpu/shader/(spirv\|hlsl\|dxil)`、`CompileShaderToSPIRV`、非迁移文件里的 `naga\.` | 0 命中 |
| API 快照 diff | 复用 P2.2 `tools/api-diff.sh`，包列表去掉 30 个被删包 | 差异只允许 §3 列出的两类连带删 + 路径平移 + `ir` 新增 |
| 负向对照 | 临时探针引用 `gpu/shader/spirv` 与 `gpu/gwgpu/naga` → 编译必红；撤销后复绿、零残留 | 红过复绿 |
| G1（spirv 空转测试） | `spirv/` 整包删除 | 自动消除，无需逐个修 |

## 7 风险与对策

1. **包名 `naga` → `shader`**：`gles/shader.go`、`metal/device.go`、`mslmap_test.go` 里所有 `naga.X` 必须同步改成 `shader.X`；漏改会编译红，门禁能兜住。
2. **根包文件命名**：迁移后 `gpu/shader/naga.go` 名字与包名不符。建议重命名为 `gpu/shader/shader.go`（D1）。
3. **测试包 import 环**：`msl/internal/codegen/msl_xcrun_test.go` 在包内导入根包；迁移后根包只依赖 `wgsl`/`ir`，不依赖 `msl`，无环。已核对。
4. **`GLSLVersionToNaga` 名字残留**：它是 `gles` 的公开函数（`adapter.go:68`、`adapter_linux.go:75` 调用）。P2 不动符号名（D2）。
5. **删除 dxcvalidator 的 Windows 代码**：无 importer、无 D3D12 目标；五目标交叉编译会验证。
6. **`render/internal/gpu` 的 API 变化**：删 `CompileShaderToSPIRV` 会让 P1.5 快照少一个符号，属连带删，需白名单登记（D3）。
7. **根包测试归零**：根的两个测试文件测的是 SPIR-V；迁移后根只剩 0 个自己的测试（其 `Parse/Lower` 由 gles/mslmap 测试间接覆盖，`LowerWithCapabilities` 目前无测试）。建议补一个根冒烟测试（D4）。

## 8 待用户拍板

| 编号 | 事项 | 建议 |
|---|---|---|
| D1 | 根文件 `gpu/shader/naga.go` 是否改名 `shader.go` | **改**（包名与文件名一致，无行为影响） |
| D2 | `gles.GLSLVersionToNaga` 是否随之改名（如 `GLSLVersionToShader`） | **P2 不改**，登记 P5 命名清理（避免把符号改名混进零行为重构） |
| D3 | 连带删除 `gpu/shader` 根 `Compile*`/`GenerateSPIRV` 与 `render/internal/gpu.CompileShaderToSPIRV` | **删**（SPIR-V 链无目标、无消费者；两个符号都在 §7.7 删除对象的依赖闭包里） |
| D4 | 是否补根冒烟测试 `gpu/shader/shader_test.go`（Parse→Lower→Validate；`LowerWithCapabilities` 拒绝 f64 / `CapAll` 通过） | **补**（根包测试不归零，`LowerWithCapabilities` 从无测试变有测试） |
