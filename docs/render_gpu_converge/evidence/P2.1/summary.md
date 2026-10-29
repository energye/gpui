# P2.1 执行结果：shader 合并（naga 迁顶层 `gpu/shader`，删三条后端链）（2026-10-29）

> 结论：**PASS**。`gpu/gwgpu/naga` 整体迁入 `gpu/shader`（根包名 `naga`→`shader`），`gpu/shader` 独有 SPIR-V/HLSL/DXIL 链与 `gpu/gwgpu/naga` 目录删除；五目标构建、vet 61/58、逐测试文件、GLSL/MSL 逐字节快照、GL 离屏像素 md5、API 对账、负向对照全部按评估预期收口。
> 依据：分支 `feat/p2-收敛整合`，开工前评估 `evidence/P2.1/assessment.md`，D1–D4 按用户拍板「按建议执行」。

## 1 改动清单

| 类别 | 内容 | 量 |
|---|---|---|
| 迁移（naga → `gpu/shader`） | 14 个包整体覆盖到 `gpu/shader` 同名路径，根文件 `naga.go`→`shader.go`（D1），import 前缀 `gpu/gwgpu/naga`→`gpu/shader` | 144 文件 / 116,060 行 |
| 删除 `gpu/shader` 独有后端 | `spirv/` 36、`hlsl/` 39、`dxil/` 63、`internal/backend/` 9、`internal/dxcvalidator/` 18、根 `bench_test.go`+`naga_test.go` | 167 文件 / 132,395 行 |
| 删除根 SPIR-V API（D3） | `Compile` / `CompileWithOptions` / `DefaultOptions` / `CompileOptions` / `GenerateSPIRV` | 5 符号 |
| 删死函数（D3 连带） | `render/internal/gpu.CompileShaderToSPIRV`（SPIR-V 唯一消费者，0 调用）及 `fmt`/`gpu/shader` 导入 | 1 函数 / 25 行 |
| 调用方改 import | gles 5 源 + 4 测试、metal 3 源 + 2 测试、mslmap 1 测试、msl codegen 的 xcrun 测试 | 15 文件 |
| 新增 | `gpu/shader/shader_test.go` 根冒烟（Parse→Lower→Validate + 能力门，D4）；naga 独有能力测试 4 文件随迁 | 5 文件 |
| 保留名（D2） | `GLSLVersionToNaga` 名字不动，顺延 P5 统一改名 | — |

合计暂存删除 **312 文件 / 248,455 行**（311 删除项 + 1 rename），工作区修改 26 文件 +1,054/−191。

## 2 门禁结果

| 门禁 | 结果 | 证据 |
|---|---|---|
| 五目标交叉编译（linux×2 / windows / darwin×2，含 `-tags nogpu`） | 每目标 4 项中 2 项绿：raw `./...` 仅 `ui_wr_t_stress` 缺组件红、known-broken 按预期红；实际构建面 step3/4 全 exit 0（linux 319/315 包、windows 327/323、darwin 319/315） | `build/*.txt` |
| `go vet` 全仓（default / nogpu） | **61 / 58 条，与 P2.3 逐条 diff 为空**（321 / 317 包，剔除 t_stress、render/render 与 30 个已删包） | `vet-full-default.txt` `vet-full-nogpu.txt` |
| 逐测试文件（248 文件，13 个包） | **PASS 220 / 仅编译无 Test 9 / 构建标签排除 18 / FAIL 1**；唯一 FAIL 为迁移前既有环境红（见 §3.2）；`render/internal/gpu` 131 文件 125 PASS/5 仅编译/1 排除/**0 FAIL** 与 P2.3 一致 | `per-file-tests.txt` `per-file-tests-notes.txt` |
| GLSL/MSL 逐字节快照 | 27 WGSL / 48 入口点 × {GL330/430/450/ES300/ES310/ES320 + MSL 默认/mslmap} = **681 文件**，迁移前后 `diff -r` rc=0、两份 sha256 清单一致；4 条预期版本门错误（coarse/path_count 原子在 330/ES300）前后同 | `snapshot-*.txt` `snapshot-*.sha256` |
| GL 离屏像素 | `TestH4C_TriangleOffscreenPixel` PASS，md5 **`59ca40337ff5e016579ea53a09549824`** 与 P1.3 基线逐位一致 | `gl-offscreen-pixel.txt` |
| darwin 测试编译 | `GOOS=darwin amd64/arm64` 对 metal/mslmap/msl-codegen 跑 `go vet`（含测试文件）全 exit 0 | `darwin-test-compile.txt` |
| API 快照 | 包 41→11；`gpu/shader` 根符号 9→5（**删 5 个 SPIR-V 符号 / 增 1 个 `LowerWithCapabilities`**）；`ir` 增 `Capabilities`+`ValidateWithCapabilities`，`wgsl`/`lower` 仅文档补能力说明；`render/internal/gpu` 761→629（**−132 / +0** = P2.3 的 131 + `CompileShaderToSPIRV`）；测试计数（322 包同集合）1226→1210 文件 / 10,600→10,353 函数 | `api-diff.txt` |
| 残留引用 | 已删 6 条 import 路径、4 个符号全 0；`naga.` 唯一命中是 `lower.go:9329` 注释；6 个目录均不存在；临时快照目录已清 | `grep-residual.txt` |
| 负向对照 | 探针 import 已删包/根符号/`CompileShaderToSPIRV` → 4 处全红；撤销后复绿、零残留 | `negative.txt` |

## 3 与开工前评估的差异

功能与符号层面无偏差：删除范围、保留包集合（11）、API 增删清单、快照文件数、GL md5 均与评估一致。执行中三处需要说明：

### 3.1 清理与口径
1. `gpu/shader/spirv/internal/tmp/{sdf_circle.spv,sdf_circle_disasm.txt}` 是 10-28 调试遗留的**未跟踪**产物，随整棵树删除（不进提交）。
2. `git rm` 后空目录 `gpu/shader/spirv/` 残留，已清；残留 grep 复核 6 个已删目录全部不存在。
3. 评估「测试 240 文件 / 5,850 函数 → 70 / 2,139 / 21」指迁移包集合；落盘后 `gpu/shader` 子树为 **71 文件 / 2,141 Test / 21 Bench**（naga 70/2,139/21 + 新增根冒烟 1/2/0），差 1 文件/2 Test 即 D4 冒烟，属预期。

### 3.2 新发现：`TestPresentDamageX11` 既有环境红（非本步因果）
- 现象：gles 逐测试文件唯一 FAIL，断言 `partial present never engaged: full=12 partial=0`（另一次运行表现为 X `BadMatch` on `X_GetImage`）。
- 对照：在 P2.1 改动前的 `4e348fd3` 干净 worktree 上同一用例**同样失败同一断言**（`present_damage_x11-baseline-4e348fd3.txt`）；该用例只走 X11/EGL 增量提交，P2.1 对 gles 只改 import。
- 处置：按「新问题归位」登记进 §2.2 待办池（GL 增量呈现线），本任务不修。

### 3.3 G1 收口
P2.3 登记的 G1（`spirv/internal/codegen/shader_test.go` 6 个读不存在路径的空转用例）随 `spirv/` 整包删除自然闭合，无需重指。

## 4 遗留与归位

- **真窗**：本步零行为（纯平移 + 死代码删除），按 §4.1 例外不新增真窗；全量真窗重放并入 P2.6。
- **Metal 真机**：本机无 Metal，等价证据用 darwin 交叉编译 + MSL 逐字节快照；macOS 真机归 P2.6。
- **`GLSLVersionToNaga` 命名**（D2）：保留，P5 统一。
- **`TestPresentDamageX11` 环境红**：新增入 §2.2 池（GL 增量呈现线）。
- **下一任务**：P2.4a（P2.1–P2.3 全部完成后解锁）。

## 5 证据清单

```
assessment.md                          开工前影响面评估（含能力替代核对 §0.1）
summary.md                             本文件
snapshot-before-run1.txt / -after-run1.txt   快照探针两次运行的汇总行
snapshot-before.sha256 / -after.sha256       681 文件清单（两份一致）
snapshot-diff.txt                      diff -r rc=0 / 0 行
gl-offscreen-pixel.txt                 GL 离屏 md5 逐位一致
darwin-test-compile.txt                darwin amd64/arm64 测试文件交叉类型检查
build/*.txt                            五目标交叉构建
vet-full-default.txt / -nogpu.txt      全仓 vet 61/58
per-file-tests.txt                     248 个测试文件逐个跑批
per-file-tests-notes.txt               构建标签排除与唯一 FAIL 的对照说明
present_damage_x11-baseline-4e348fd3.txt     基线 worktree 上的同断言失败
api-diff.txt                           API 符号/测试计数对账
grep-residual.txt                      残留引用
negative.txt                           负向对照
tools/shader_snapshot.go.txt           快照探针源码（一次性工具，固化留档）
tools/build-gate.sh / vet-full.sh / per-file-tests.sh / api-diff.sh / grep-residual.sh / negative.sh
tools/removed-packages.txt             30 个已删包清单（API/vet 口径）
```
