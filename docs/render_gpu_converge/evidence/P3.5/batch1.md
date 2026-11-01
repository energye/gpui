# P3.5 批 1 / conic 路径段交付证据

**日期**：2026-11-01　**HEAD**：`6d861410`　**分支**：`feat/p2-收敛整合`　**树**：P3.5 工作树（未提交）
**范围**：仅 P3.5 批 1（真 `ConicTo` verb + 边界降 quads + 漏点防护 + `quadArea` 修正）。批 2–5 未开始。

## 1. 结论

批 1 的能力、公开口、边界展开、防护和值级/像素级测试已完成；五目标构建、apidoc、conformance、`-race`、负向对照均绿。主逐文件门禁 686 PASS / 21 SKIP_TAGGED / 9 FAIL，9 条全部命中 P3.3/P3.2 基线红名单，零新增；补充分包除 HEAD 既有坏包 `render/recording/backends/raster`（P3.1 参数漏改债）外全绿。P3.5 整体仍为 🟡（批 2–5 待做），批 1 可独立提交。

## 2. 交付物

### 2.1 公开 API（已登记 `docs/RENDER_API_CATALOG.md`，apidoc rc=0）

| 符号 | 语义 |
|---|---|
| `PathVerb.ConicTo`（值 5） | 追加在 `Close` 后，原 5 个 verb 的数值稳定；coords = `(cx, cy, x, y, w)` |
| `Path.ConicTo(cx,cy,x,y,w)` | `w<=0` 按 Skia 语义退化直线；`w==1` 等价 QuadraticTo；`w==√2/2` 精确四分之一圆弧；仿射变换保权 |
| `PathBuilder.ConicTo` / `Context.ConicTo` / `Canvas.ConicTo` / `Recorder.ConicTo` | 声明式构建器、即时光栅、可录制绘制面全覆盖 |
| `ConicBez` + `NewConicBez` | 有理二次求值几何：`Eval/Start/End/Chop/Subsegment/Tangent/Extrema/BoundingBox/ToQuads` |
| `Path.HasConics()` | 路径是否含 conic |
| `Path.ExpandConicsToQuads(tol)` | 无 conic verb 的子系统边界统一降二次；无 conic 路径原指针返回（零成本） |

计数按 `go/ast` 实测：类型 +1（`ConicBez`）、顶层函数 +1（`NewConicBez`）、常量 +1（`PathVerb.ConicTo`）、Context 方法 +1（`ConicTo`）、`Canvas` 106→107。

### 2.2 核心实现

- `render/curve_conic.go`：有理二次 Eval/Chop/Subsegment/Tangent/Extrema/BoundingBox/ToQuads；Chop 权重 `sqrt((1+w)/2)`；ToQuads 以端点+中点插值二次逼近，按 8 分点误差递归，最多 18 层。
- `render/path.go` / `path_verb.go` / `path_build.go`：真 verb 存储；`Iterate` 对未知 verb 返回 `-1` 并 panic；`HasCurves` 含 conic；`Transform` 保权。
- `render/path_metrics.go` / `pathops_area.go` / `pathops_bbox.go` / `pathops_subpath.go` / `pathops_winding.go` / `path_boolean.go`：Length/ComputeMetrics/Area/BoundingBox/Flatten/Reversed/Trim/Winding/布尔输入都原生处理 conic，不静默跳过。
- `render/software_fill.go` / `software_stroke.go` / `stroke_user_expand.go` / `context_clip.go`：raster/stroke/clip 边界降 quads；dash 原生把 conic 展成线；stroke 探针在 expander 内对 `VerbConicTo` 直接 panic，避免坐标流错位。
- `render/scene/encoding.go` / `render/scene/shape.go`：scene 编码与 `NewGGPathShape` 边界降 quads（scene 格式无 conic 位）。
- `render/recording/recorder_path_draw.go` / `recorder_canvas_ops.go`：录制面往返。
- `render/internal/gpu/gpu_types.go` / `sparse_strips_filler.go` / `path_convert.go` / `vello_accelerator.go`：convex 形状检测拒绝 conic；SparseStrips/Vello 边界降 quads；AA-off 像素吸附保留权重。

### 2.3 `quadArea` 修正

旧公式漏端点项、中间项系数错，已改为 Green 公式精确解：

```
[x0(2y1+y2−3y0) + 2x1(y2−y0) + x2(3y2−2y1−y0)] / 6
```

用独立数值积分（`TestQuadAreaExactAgainstNumericIntegral`，任意控制点）和精确期望 `100/3`、`80/3`、`250/3` 双向钉住。`TestPathArea_Quadratic` 随之收紧到 `1e-9`，不是放宽断言。

## 3. 边界展开清单（conic 不进入无 conic 位的格式）

| 边界 | 位置 | 做法 | 钉 |
|---|---|---|---|
| GPU/裁剪主边界 | `render/context_path.go` `deviceSpacePath` | 设备变换后 `ExpandConicsToQuads(0.1)` | `TestConicGPUMatchesCPU`、`TestContextClipConicPath` |
| CPU fill | `render/software_fill.go` `SoftwareRenderer.Fill` | 入口展开（no-op for conic-free） | `TestSoftwareRendererFillConic`、`TestContextFillConicQuarterDisk` |
| raster 转换 | `render/software_fill.go` `convertGGPathToCorePath` | `case ConicTo` → QuadTo | `TestConvertGGPathToCorePathConic`（负向靶） |
| CPU stroke | `render/software_stroke.go` `Stroke` | 展开后转 stroke verbs | `TestSoftwareRendererStrokeConic`、`TestContextStrokeConicArc` |
| dash | `render/software_stroke.go` `dashPath` | conic 原生 flatten 成线 | `TestDashPathConicEmitsLinesOnly` |
| 用户空间扩边 | `render/stroke_user_expand.go` | 展开后 expander | `TestContextStrokeConicArc` |
| clip 动词 | `render/context_clip.go` `convertPathToClipVerbs` | 二次防御展开 | `TestConvertPathToClipVerbsConic` |
| stroke expander | `render/internal/stroke/expander.go` | `VerbConicTo=5`，到达 Expand 即 panic | 编译期/运行期防护 |
| scene 编码 | `render/scene/encoding.go` `EncodePath` | 边界展开 | `TestEncodePathWithConic` |
| scene Shape | `render/scene/shape.go` `NewGGPathShape` | 边界展开 | `TestGGPathShapeWithConic` |
| convex 检测 | `render/internal/gpu/gpu_types.go` `extractConvexPolygon` | conic 视为曲线，拒绝快路 | `TestExtractConvexPolygonWithConicCurve` |
| SparseStrips | `render/internal/gpu/sparse_strips_filler.go` `convertGGToScenePath` | 边界展开 | `TestConvertGGToScenePathExpandsConic` |
| Vello | `render/internal/gpu/path_convert.go` `convertPathToPathDef` | 边界展开 | `TestConvertPathToPathDefConic` |
| AA-off 吸附 | `render/internal/gpu/vello_accelerator.go` `snapPathToPixelGrid` | 保留 conic，仅圆整坐标/控制点，权重不变 | `TestSnapPathToPixelGrid` |

路径度量与几何查询（Area/BoundingBox/Length/ComputeMetrics/Winding/Reversed/Trim/布尔输入）选择原生 conic 处理而不是先展开，避免精度损失；测试见 §4。

## 4. 测试

| 文件 | 新增/相关钉 | 结果 |
|---|---|---|
| `render/curve_conic_test.go` | 12 条：圆弧解析对账、w=1≡Quad、Chop/Subsegment、ToQuads 容差、紧致 bbox、存储往返、w<=0 退化、仿射保权、未知 verb panic、`quadArea` 数值积分、像素填充 | PASS |
| `render/conic_pipeline_test.go` | 14 条：Context fill/stroke/clip、dash、Length、BBox、Trim、Reversed、Expand、Winding、ComputeMetrics、软件 fill/stroke、clip 动词守卫 | PASS |
| `render/conic_gpu_parity_test.go` | `TestConicGPUMatchesCPU`：GPU 真路由（GPUOps>0、CPU fallback=0）+ 落定像素通道差≤1、分类差 0、AA 边带≤1%、内部像素数差≤0.1% | PASS（native） |
| `render/software_coverage_test.go` | `TestConvertGGPathToCorePathConic` | PASS |
| `render/scene/encoding_test.go` | `TestEncodePathWithConic` | PASS |
| `render/scene/shape_test.go` | `TestGGPathShapeWithConic` | PASS |
| `render/internal/gpu/path_convert_test.go` | `TestConvertPathToPathDefConic` | PASS |
| `render/internal/gpu/sparse_strips_test.go` | `TestConvertGGToScenePathExpandsConic` | PASS |
| `render/internal/gpu/convex_renderer_test.go` | `TestExtractConvexPolygonWithConicCurve` | PASS |
| `render/internal/gpu/vello_antialias_test.go` | `TestSnapPathToPixelGrid` 增加 conic 权重不变断言 | PASS |
| `render/recording/conic_test.go` | `TestRecorderConicPathRoundtrip`：录制面保留 `ConicTo` 五坐标（含权重） | PASS |
| `render/internal/stroke/conic_test.go` | `TestStrokeExpanderRejectsConic`：未降 quads 的 conic 到 expander 必须 panic 而不是错位 | PASS |

批量复跑：`evidence/P3.5/conic-tests.txt`（6 个相关测试文件逐文件 PASS）。

## 5. A/B 定性（开工前 7 条红）

逐条见 `evidence/P3.5/ab/README.md`：

- 2 条 GPU 环境红：缺 `WGPU_NATIVE_PATH`/`LD_LIBRARY_PATH` 导致 CPU fallback；干净 HEAD 与工作树补环境都 PASS。
- 3 条 pattern 顺序耦合：整包一起跑才红；逐文件在工作树/干净 HEAD × 带/不带 GPU 环境四组全绿。
- 1 条有意契约变更：未知 verb 从静默 `0` 改 `-1` + panic。
- 1 条旧期望跟随错误公式：`quadArea` 修正后三组精确值。

## 6. 门禁

| 门禁 | 命令/位置 | 结果 |
|---|---|---|
| 五目标构建 | `evidence/P3.5/build-gate.sh` → `build-gate.txt` + `build/` | ✅ 5 目标 ×（逐包 + `-tags nogpu`）全绿；raw `go build ./...` exit 1 仅来自三条既有坏包（`ui_wr_t_stress`、`ui/scene/internal/gpupixel`、`ui_render_base_recording`，均 HEAD 既有） |
| conformance 双后端 | `go test ./render -run '^TestConformanceDualBackend$'` | ✅ PASS（`evidence/P3.5/conformance/run.txt`） |
| apidoc | `go run ./scripts/apidoc` | ✅ rc=0，主包+scene/recording/svg/gpu/hdr 全覆盖（`evidence/P3.5/apidoc.txt`） |
| vet | `go vet ./render ./render/scene ./render/recording ./render/internal/gpu ./render/internal/stroke` | ✅ rc=0（`evidence/P3.5/vet.txt`） |
| `-race` | `-count=2` render conic/path 子集 + internal/gpu 新边界钉 + scene/recording/stroke 全包 | ✅ rc=0（`evidence/P3.5/race-conic-path.txt`；早前分包记录 `race-scene-recording-stroke.txt`） |
| 负向对照 | 摘 `convertGGPathToCorePath` 的 `case ConicTo` → 钉红 → 还原绿 | ✅ `evidence/P3.5/negative-control/` |
| 逐文件回归（主清单 34 包） | `tools/per-file-tests.sh tools/per-file-tests.pkgs` | ✅ 686 PASS / 21 SKIP_TAGGED / 9 FAIL / 1 NO_TESTS；9 FAIL 全部逐名命中 P3.3/P3.2 基线红名单，**零新增**；P3.3 的 `ui/platform/x11_s6_p0_test.go` 本轮 PASS（环境相关老红未复现）。见 `evidence/P3.5/per-file-tests/summary.txt`（原始 717 行 `summary.tsv`） |
| 补充分包逐文件 | scene/recording/stroke/raster/tilecompute/recording-raster | ✅ 69 PASS / 2 SKIP_TAGGED / 6 FAIL；6 FAIL 全是 HEAD 既有坏包 `render/recording/backends/raster` 的编译失败（P3.1 `render.BlendSpace` 参数漏改），不是本批回归；raster/tilecompute/stroke 金图测试全绿。见 `per-file-tests-extra/summary.txt` |

## 7. 已知非本线债务

- `examples/ui_render_base_recording` 在 HEAD `6d861410` 已编译失败（P3.1 给 `Backend.FillPath` 加了 `render.BlendSpace`，示例 spy 未切）；非 P3.5 回归，构建门禁按既有坏包排除。
- `examples/ui_wr_t_stress` 缺 `ui/kit/button` + `ui/kit/icon`；`ui/scene/internal/gpupixel` 既有坏包；均按 P2.6 口径排除。
- `render/recording/backends/raster` 测试包在 HEAD 已编译失败：P3.1 给 `recording.Backend.FillPath/StrokePath/FillRect` 加 `render.BlendSpace` 后，该包测试仍按旧 4 参调用（HEAD `backend.go:123` vs `backend_test.go:116`）。本批未改该包，补充分包门禁把 6 个文件记为 HEAD 既有红；与 `examples/ui_render_base_recording` 同一条 P3.1 漏改债。
- pattern 三测的整包顺序耦合为既有问题；本批按 AGENTS「逐文件回归」口径验收，并留 A/B 日志。

## 8. 下一步

批 2 四角独立 RRect（类型+归约+Draw/Clip 新口，complex 落 path）；批 3 Matrix4；批 4 PathEffect；批 5 布尔保曲线。P3.5 整体仍 🟡。
