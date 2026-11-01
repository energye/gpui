# P3.5 批 3 / Matrix4 透视绘制 交付证据

**日期**：2026-11-02　**HEAD**：`dd7e2635`+批 2 已提交（HEAD 以 git log 为准）　**分支**：`feat/p2-收敛整合`　**树**：P3.5 批 3 工作树（未提交）
**范围**：仅 P3.5 批 3（D4=A 绘制口级：`Matrix4` 类型+数学+投影工具口 + `DrawImageQuad` 透视校正 CPU 1/w + GPU clip-space 真 w）。批 4 PathEffect、批 5 布尔保曲线未开始。Canvas 级 4×4 栈与路径/文本透视按 D4 拍板挂 P3.7。

## 1. 结论

批 3 的类型、数学、投影工具口、CPU/GPU 双侧透视校正、哨兵校验、录制直通与值级/像素级/对拍测试完成；apidoc、vet、定向 23 条、`-race -count=2`、conformance 双后端、五目标构建、双负向对照（CPU 摘 1/w、GPU 摘 clip-w）均按预期红→绿；逐文件回归零新增红。P3.5 整体仍 🟡（批 4–5 待做）。

## 2. 交付物

### 2.1 公开 API（已登记 `docs/RENDER_API_CATALOG.md`，apidoc rc=0）

| 符号 | 语义 |
|---|---|
| `Matrix4`（matrix4.go） | 4×4 齐次矩阵，SkM44 同位物，行主序 `m[i*4+j]`；零值不可用（全零奇异），单位取 `Matrix4Identity()` |
| `Matrix4Identity()` / `Matrix4FromMatrix(Matrix)` / `ProjectorMatrix4(focal,center,camera)(Matrix4,bool)` | 单位 / 2x3 提升 / Projector 对齐透视矩阵（非法入参返回单位+false，同 `NewProjector` 拒绝规则） |
| `(Matrix4) Mul / Invert()→(Matrix4,bool) / TransformVec4 / At / IsIdentity` | 数学面；Invert 用 2x2 子式分解，奇异（\|det\|<1e-12）或非有限返回 ok=false（不同于 2x3 `Matrix.Invert` 回退单位阵——4×4 真奇异常见，静默回退会画出错东西） |
| `(Matrix4) ProjectPoint(x,y,z)→(Point,w,bool)` | 透视除法投影；w≤0（眼后）或非有限拒绝，对齐 `Projector.Project` |
| `(Matrix4) ProjectQuad(corners,depths)→(screens,weights,bool)` | 四角+逐角深度→屏幕角+齐次权重；任一角不可画整 quad 拒绝（`Projector.ProjectQuad` 同契约）；权重直接喂 `DrawImageQuad` 透视口 |
| `(Matrix4) Affine()→(Matrix,bool)` | 回退判定：仅 w 行=(0,0,0,1)、z 直通、平移在 w 列时返回 2x3；透视矩阵 ok=false，绝不静默压扁 |
| `QuadDrawOptions.Perspective` + `.Weights[4]` | `DrawImageQuad` 透视校正开关+逐角齐次 w（TL/TR/BR/BL，恒有限>0）；零值=历史仿射路径逐位不变 |
| `ErrQuadBadWeights` / `ErrQuadPerspectiveBicubic` | 权重非法（≤0/NaN/Inf）拒画，不留半个梯形；透视+bicubic 无管线变体显式拒绝（对齐 R1 哨兵族风格） |

计数（go/ast 实测，按符号名去重）：类型 +1（`Matrix4`）、顶层函数 +3、变量 +2；`Canvas`/Context 方法/Recorder 计数不变（opts 值拷贝直通，无新命令类型）。

### 2.2 Projector 语义对齐钉

`ProjectorMatrix4(f,center,camera)` 映射 (x,y,d,1)：`X=f·x+cx·d+f(cx−camx)`、`Y` 同理、`W=d+f`，透视除法后逐点 ≡ `engine/camera.Projector.Project`：`screen = center + (world−camera)·f/(f+d)`，权重 = `f+d`（∝ 1/scale）。`TestMatrix4ProjectorMatchesProjectorMath` 用闭式对账（4 组点，含 0 深度/负 x/大 y/远深度）；`TestMatrix4ProjectQuadTracesProjectorContract` 钉梯形上下边长关系与整 quad 拒绝；`TestMatrix4ProjectiveEdgeRatio` 钉定义性质：等权屏幕中点 uv=`w0/(w0+w1)`≠0.5，且该 uv 的世界点投影恰落回屏幕中点（正反映射闭合）。

### 2.3 CPU 1/w 有理插值（m4_trisampler.go / m4_quadimage.go）

`triSampler` 增 `persp/invW`：uv(P) = Σ λᵢ·uvᵢ/wᵢ ÷ Σ λᵢ/wᵢ（λ 为设备平面重心，同 GPU 硬件透视插值定义）；`DrawImageQuad` 校验权重后按 `Perspective` 分流 twin-split 采样器；等权时数学上退化为仿射——`TestQuadPerspectiveEqualWeightsMatchAffine` 钉像素逐字节相等。`ClassifyQuad`/损伤 AABB/`quadBounds` 只依赖角点，透视/仿射共享（`TestQuadPerspectiveDamageBounds` 钉两路 union 逐矩形相等）。

### 2.4 GPU 透视管线变体（D4 拍板「新增透视管线变体」，用户 2026-11-02 二次确认）

- 新 shader `render/internal/gpu/shaders/textured_quad_persp.wgsl`：与主 shader 同 uniform/纹理/clip 绑定与 FS 逻辑，VertexInput 多一路 `@location(3) hom_w: f32`；VS 出 `position = (clip.xy·w, clip.z·w, w)`——除法后屏幕角精确不变，varying 由硬件按 attribute/w 线性插值（WebGPU 规范保证，透视正确），GL 后端经 naga 同一份 WGSL。
- 管线变体一对（stencil/depth-clip）`ensurePerspectivePipelines()`，随 `destroyPipeline` 释放；`pipelineForDraw` 按 `dc.perspective` 选管。
- 命令链：`gpuContextOps.QueueImageDrawQuadW`（新接口方法，`GPURenderContext` 实现）→ `ImageDrawCommand.Perspective/Weights` → `buildImageResources` 把透视 quad 打进**独立 36B 顶点缓冲**（仿射 32B 缓冲索引空间保留 i*6 不变式、零新增字节写入），`canMergeImageDraw` 透视永不合并、不与仿射混批；`RecordDraws` 按 `dc.vertBuf` 重绑顶点缓冲。
- 管线不可用时降级仿射平面发射（warn，不丢绘制，同 blend 变体先例）。
- bicubic/blend/tint 与透视互斥：渲染层拒 bicubic，`QueueImageDrawQuadW` 无 tint/blend 参。

**未动清单**：`textured_quad.wgsl` 等原 13 份 WGSL、仿射顶点 writer、`ImageDrawCommand` 零值语义、`ClipParams`/SDF 快路、`render/scene` 编码——非透视调用方逐字节路径不变。

## 3. 测试钉

| 组 | 钉 | 结果 |
|---|---|---|
| matrix4_test.go（10 条） | 单位/提升/Affine 往返、Mul≡2x3 提升逐位、Invert 往返（affine+projector）/奇异/非有限拒绝、Projector 闭式对账+拒绝规则、behind-eye 拒绝、ProjectQuad 契约、边缘比 homography 性质 | PASS |
| matrix4_quad_perspective_test.go（4 条，CPU） | 权重哨兵族（0/负/NaN/Inf/bicubic/仿射忽略 junk 权重）+拒绝不污染画布；band 闭式 5 探针（判别像素 (180,5)：透视 u=6/7 蓝 vs 仿射 3/4 红，oracle 与采样实现独立推导）；等权≡仿射逐字节；损伤 union 两路相等 | PASS |
| matrix4_quad_persp_gpu_test.go（3 条，native GPU） | 真路由钉（GPUOps>0、CPUFallbackOps=0，防 CPU 回落假绿）；GPU 5 探针闭式落位 + 仿射对照（(180,5) 翻红）；CPU/GPU 逐像素分类+settled 通道对拍（声明容差：settled 差 ≤1/255、band-line 翻转 ≤2%、边带/混合 ≤5%，实测远低） | PASS |
| internal/gpu perspective_quad_test.go（5 条） | 36B 布局四属性（offset/format/location）与 wire 字节序逐通道对账（6 顶点 twin-split 权重/uv/tint）、短缓冲 panic、`canMergeImageDraw` 透视双向隔离+仿射合并不回退、`ImageDrawCommand` 零值=仿射 | PASS |
| recording perspective_quad_test.go（1 条） | `DrawImageQuadCommand.Opts` 原样携带 `Perspective/Weights`（无新命令类型）；实况 vs 回放像素逐字节相等+探针分类钉 | PASS |

定向复跑 23 条（render 17 + internal/gpu 5 + recording 1）`-count=1` 全 PASS；`-race -count=2` 三组 rc=0（`race-batch3.txt`）。

## 4. 负向对照（双侧红→还原绿）

1. **CPU 摘 1/w**（`triSampler.uv` persp 分支短路为仿射）：
```
--- FAIL: TestQuadPerspectiveBandPixels — probe (180,5) oracle u=0.8571428571 want rgb=(0,0,1) got {R:1 G:0 B:0 A:1}
--- FAIL: TestQuadPerspectiveCPUGPUParity — band-line classification flips = 3735, want <= 583 (2%)
```
2. **GPU 摘 clip-w**（`buildPerspVerticesInto` 权重临时钉 1.0，硬件退化为平面插值）：
```
--- FAIL: TestQuadPerspectiveGPUProbesClosedForm — GPU probe (180,5) class = 0, want 1；(180,90) class = 0, want 1
--- FAIL: TestQuadPerspectiveCPUGPUParity — band-line classification flips = 3867, want <= 583 (2%)
```
还原后 23 条全绿（`negative-control-batch3/`：`cpu-rational-removed-red.txt`、`gpu-clipw-removed-red.txt`、`restored-green.txt`）；`grep NEGATIVE-CONTROL-TEMP render/` 空=无残留。

## 5. 门禁

| 门禁 | 命令 | 结果 |
|---|---|---|
| apidoc | `go run ./scripts/apidoc` | ✅ rc=0 |
| 主包计数 | go/ast 复算（口径同批 2 去重） | ✅ 142/175/219/18/169/26 与 catalog §0 一致 |
| conformance 双后端 | `go test ./render -run '^TestConformanceDualBackend$'` | ✅ PASS（`conformance-batch3.txt`） |
| 定向测试 | 23 条 `-count=1` | ✅ 全 PASS |
| `-race` | 三组 `-race -count=2`（CGO_ENABLED=1） | ✅ rc=0（`race-batch3.txt`） |
| vet | `go vet ./render ./render/recording ./render/scene ./render/internal/gpu ./render/internal/stroke` | ✅ rc=0（`vet-batch3.txt`） |
| 五目标构建 | `evidence/P3.5/build-gate.sh`（输出 `build-batch3/`，不覆盖批 1 证据） | ✅ 5 目标 ×（逐包 + `-tags nogpu`）全绿（`build-batch3/summary.txt`；raw exit 1 仅三条既有坏包，同批 1 口径） |
| 相关包全量预览 | `go test ./render/recording ./render/scene ./render/internal/gpu -count=1`（native 环境） | ✅ 三包全 ok（internal/gpu 全包 67.6s） |
| 逐文件回归（主清单） | `tools/per-file-tests.sh tools/per-file-tests.pkgs ...` | ✅ 692 PASS / 21 SKIP_TAGGED / 8 FAIL / 1 NO_TESTS（+4 PASS=本批新测试文件）；8 FAIL 全部在批 1∪批 2 基线红名单内，零新增；与批 2 名单变化=批 1 老红 `s6_9_heavy_budget` 本批轮复现（已 A/B 定性，见 §7 与 `ab-batch3/`），`m1_noquadratic`/`m5_keystroke_g1` 老红本轮转绿（名单只减不增）。见 `per-file-tests-batch3/summary.tsv` |
| 补充分包逐文件 | `tools/per-file-tests.sh evidence/P3.5/per-file-extra.pkgs ...` | ✅ 71 PASS / 2 SKIP_TAGGED / 6 FAIL（+1 PASS=本批 `render/recording/perspective_quad_test.go`）；6 FAIL 全为 HEAD 既有坏包 `render/recording/backends/raster`（P3.1 参数漏改债）；raster/tilecompute/stroke 金图文件全绿=存量金图零变化。见 `per-file-tests-batch3-extra/summary.tsv` |

## 6. 硬约束遵守清单

- **原 13 份 WGSL 零改动**（`git diff` 仅新增 `textured_quad_persp.wgsl`）；仿射 32B 顶点布局/writer/管线/合并逻辑对非透视调用逐字节不变。
- 存量金图零变化：透视仅在 `Perspective=true` 时生效；`TestQuadPerspectiveEqualWeightsMatchAffine` 逐字节钉退化等价；per-file 金图文件结果见 §5。
- `render/` 不出现后端类型（透视全走 `gpuContextOps` 接口 + `hal/types` 顶点格式；创建入口未动）。
- 未在 examples/ 绕洞：本批零示例改动。
- 接口全量切换制：本批纯增量（新接口方法+opts 字段），无改名改签名。

## 7. 已知基线红（非本批引入，不修）

- `render/recording/backends/raster` 测试包编译红 = P3.1 `BlendSpace` 参数漏改债（批 1 §7 在案）。
- 主清单 8 条 FAIL 全落批 1∪批 2 基线红名单；唯一名单变化 `render/s6_9_heavy_budget_test.go`（批 2 轮 PASS、批 3 轮 FAIL）已做 A/B：工作树两连跑红名漂移（run1=H04+H06、run2=H04、门禁窗=H03+H04），干净 HEAD worktree（aae2d377，同库同环境）同样 FAIL 且红名不同（U05/H02，H03/H04 反 PASS），超出幅度全部贴线 2%~6%——墙钟 p50 预算门环境/负载敏感老红（批 1 名单在案），非批 3 回归；原始记录 `ab-batch3/`。
- `ui/rendering/m1_noquadratic`（批 2 轮时序贴线红）与 `m5_keystroke_g1`（批 1/2 老红）本批轮均 PASS，名单只减不增。

## 8. 未做项与遗留

- **真窗未建**：`examples/ui_wr_p35_pathgeom` Matrix4 透视卡（quad 透视贴图 + 透视vs仿射放大对比，双后端+金图）随批 4/5 或专项统一建；当前=单测/像素/双后端对拍/录制回放原子验证（AGENTS「已验证待接线」口径）。
- Canvas 级 4×4 变换栈、路径/文本透视、GPU 纹理 Quad（`QueueGPUTextureDrawQuad*`）透视变体：D4 拍板明确挂 **P3.7**（消费路径=`Matrix4.ProjectQuad`→`DrawImageQuad{Perspective,Weights}`）。
- 透视+bicubic/blend/tint 组合无管线变体，显式哨兵拒绝（批 8 如需再扩）。
- `textured_quad_persp.wgsl` 的 clip 覆盖代码与主 shader 重复一份（同 13 份先例的既有模式；统一核抽取属渲染分层专项，不在本批面）。
- 批 4 PathEffect、批 5 布尔保曲线未开始；P3.5 整体 🟡 不变。

## 9. 修订

- 2026-11-02：批 3 交付登记（代码 + catalog §0/§2/§8/§13 + 台账载体块/焦点/修订表）。GPU 载体方案经用户拍板「新增透视管线变体（风险面最小）」（原 13 份 shader 不动）。
