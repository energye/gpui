# P2.3 执行结果：删老一代 GPU 场景渲染器闭合簇（2026-10-29）

> 结论：**PASS**。10 源 + 10 测试 + 7 WGSL 删除完成，3 处存活面按评估迁移；五目标编译、vet 61/58、逐测试文件 0 FAIL、API 快照 −131/+0、负向对照红过复绿。
> 依据：`baseline-p1`（`92ca9216`）分支；开工前评估见同目录 `assessment.md`。

## 1 改动清单

| 类别 | 内容 | 量 |
|---|---|---|
| 删源码 | `backend.go` `renderer.go` `pipeline.go` `pipeline_cache_core.go` `memory.go` `shaders.go` `sparse_strips_gpu.go` `gpu_coarse.go` `gpu_fine.go` `gpu_flatten.go` | 10 文件 / 6515 行 |
| 删测试 | 9 个簇测试 + `integration_test.go` | 10 文件 / 5635 行 / Test 156 / Bench 19 |
| 删 WGSL | `shaders/{blit,blend,strip,composite,coarse,fine,flatten}.wgsl` | 7 文件 / 2071 行 |
| 裁基准 | `benchmark_test.go` 删 11 个测老簇的 Benchmark + `runBackendComparison`，留 `BenchmarkSceneCreation` | −11 Bench |
| 迁移 | slim `Backend`（`mu/device/queue/initialized` + 4 方法）→ `device.go` | 7 处既有测试零改动 |
| 迁移 | `StubPipelineID`/`StubComputePipelineID`/`StubBindGroupID`/`StubBufferID` → `commands.go` | 4 类型 |
| 摘钩子 | `GPUTexture.manager`、`SetMemoryManager`、`Close` 里的回收分支 | 生产恒 nil |
| 重写 | `doc.go` 改为描述 vello/`tilecompute` + 分域缓存新架构 | — |

## 2 门禁结果

| 门禁 | 结果 | 证据 |
|---|---|---|
| `go build ./render/...`（宿主） | exit 0 | 见 `build/`、负向对照 step 1 |
| 五目标交叉编译（linux×2 / windows / darwin×2，含 `-tags nogpu`） | 每目标 4 项中 2 项绿：raw `./...` 与 known-broken 按预期红（仅 `ui_wr_t_stress` 缺组件），实际构建面无错 | `build/*.txt` |
| `go vet` 全仓（default / nogpu） | 61 / 58 条，与 P2.2 **逐条一致**（diff 为空），无新增 | `vet-full-default.txt` `vet-full-nogpu.txt` |
| 逐测试文件（131 文件） | PASS 125 / 无 Test 仅编译 5 / 构建标签排除 1（`stencil_metal_repro_test.go`）/ **FAIL 0** | `per-file-tests.txt` |
| 包整体测试 | `ok render/internal/gpu 1.542s` | `per-file-tests.txt` |
| 残留引用 | 除 `render/scene` 同名 `GPUSceneRenderer`（另一类型，`gpu_renderer.go:37`）外，17 个符号全 0 | `grep-residual.txt` |
| API 快照 | `render/internal/gpu` 符号 761→630，**删 131 / 增 0**；测试 141/1008/109 → **131/852/79**；其余包零差异（`render/render` 缺席为 P2.2 遗留） | `api-diff.txt` |
| 负向对照 | 探针引用 `NewBackend`/`newGPURenderer`/`NewGPUSceneRenderer` → 编译红；撤销后复绿、无残留 | `negative.txt` |
| `webgpu.CreateInstance` 白名单 | 4 → 3（`render/backend.go:100`、`vello_accelerator.go:1330`、`gpu_shared.go:769`） | `grep-residual.txt` |

## 3 与开工前评估的差异

无功能/符号层面的偏差：删后编译缺口正是评估的 3 类符号；测试计数、API 符号数、`CreateInstance` 白名单均与评估一致。
执行中发现并修正的两处口径问题（不影响代码）：

1. 评估 §5 原写「9 个簇测试 Test 142」，实为 **147**（+`integration_test.go` 9 = 156），已改正。
2. `gpu/shader/spirv/internal/codegen/shader_test.go` 的 6 个用例用 `os.ReadFile("../../gg/internal/gpu/shaders/*.wgsl")` 读旧路径——该路径在本仓**从未存在**，用例一直走 `t.Skipf`；删 4 份 shader 后仍跳过，无回归（`tilecompute/shaders/*` 是另一套阶段，不能直接顶替）。已在台账池登记归 P2.1。

## 4 遗留与归位

- **第二层老抽象**（`command_encoder.go`/`commands.go`/`atlas.go`/`gpu_texture.go`）：仍在用（`RectAllocator` 被 `glyph_color_atlas.go` 用、`IndexFormat` 被 `render_pass.go` 用、`*RenderPass` 出现在 `text_pipeline.go`），按符号拆解留给 P2.4e，台账池新增 G2。
- **统一显存预算**：`MemoryManager` 删除后，分域预算（image/glyph/depth-clip/texture_pool）继续生效；统一 Budget 按 P2.4b 重新设计，不复活旧实现。
- **C2 `ui_wr_t_stress`**：缺 `ui/kit/button`/`icon`，与本次无因果，改挂组件线。
- **真窗重放**：本次未开 GPU 真窗（删除不触及任何生产路径，§4.1 例外）；「1 扇 render 基类窗 + P1.4 金图对账」作为加强证据留待 P2.6 全量重放一并执行。
- **D2 中挂 P2.3 的两组 FAIL**：老 backend 并发 SIGABRT 随 `integration_test.go` 一起消失；`m5_color_glyph_api` 4 个用例在 `92ca9216` 与当前 HEAD 均 PASS，属当时环境性 FAIL，非代码债。

## 5 证据清单

```
assessment.md                       开工前影响面评估（含能力替代核对 §0.1）
refs.tsv / tools/scan_refs.go.txt   AST 引用扫描
tools/minimal_patch.py              最小存活面补丁（实验）
build-gate.sh / build/*.txt         五目标构建
api-diff.sh / api-diff.txt          API 快照对账
per-file-tests.sh / per-file-tests.txt   逐测试文件
grep-residual.sh / grep-residual.txt     残留引用
negative.sh / negative.txt          负向对照
vet-full-default.txt / vet-full-nogpu.txt 全仓 vet
```
