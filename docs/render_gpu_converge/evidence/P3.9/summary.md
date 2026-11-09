# P3.9 mesh gradient 交付记录（2026-11-09，✅ 像素收口）

> 真源：`docs/ENGINE_RENDER_GPU_CONVERGE_TASKS.md` §2/§2.1/§7；方案：`assessment.md`、`design.md`。
> 结论：**✅ 像素收口**——新能力画面被真窗金图逐位比对（native 与 go/GL 各自 0.0000% / 763584 px）且显式像素探针 16/16 通过；双后端、负向对照、能力单测、录制回放、构建/vet/apidoc 全齐。

## 0 直话

- 新增的是 **patch 绘制原语**，不是“再加一种渐变”：每个 patch 4 个角角色色 + 12/16 个贝塞尔控制点，2×2/3×3 个 patch 由调用方拼成网格。
- 标准落地照 Skia：`SkCanvas::drawPatch`（Coons 12 点）→ `SkPatchUtils::MakeVertices` → `drawVertices`；本仓复用 `DrawMesh`/`DrawVertices` 的彩色网格双路径，**没有新 WGSL 管线**。
- `bicubic` = PDF shading type 7 TensorProduct，16 控制点 4×4 控制格；patch 内颜色仍按 Skia 四角 bilerp（不是 Compose 的 Catmull-Rom）。
- 顺手抓到并修掉一个真实引擎洞：**GLES `DrawIndexed` 完全忽略 WebGPU 的 `baseVertex`**，导致第二个及之后的 mesh 读错顶点。go/GL 首跑四卡只画出第一块 patch 的三角扇就是实证。

## 1 公开 API

| 符号 | 说明 |
|---|---|
| `MeshPatchStyle` + `MeshPatchCoons` / `MeshPatchBicubic` | Coons 12 控制点 / TensorProduct(bicubic) 16 控制点 |
| `MeshPatch{Style, Points, Colors}` | `Colors` 按 TL/TR/BR/BL 角角色；Coons 点序逐字对齐 Skia，Bicubic 为 4×4 行主序 |
| `ErrMeshPatchShape` | 未知风格/点数不匹配；非有限点/色复用 `ErrVertsNonFinite` |
| `(*Context).DrawMeshPatch` | 静默跳过非法 patch |
| `(*Context).DrawMeshPatchEx` | 返回 `VertDrawResult`（Triangles/Degraded/Skipped）与错误 |
| `Canvas.DrawMeshPatch` | 录制面与即时面同一签名 |
| `recording.DrawMeshPatchCommand` + `CmdDrawMeshPatch` + `Backend.DrawMeshPatch` | 录制只存 patch 原样，回放期按 CTM 重新细分（Skia picture 同口径） |

## 2 实现要点

- **细分**：`render/mesh_patch.go`；LOD = 四条边界三次曲线设备空间折线长度/10、下限 8；顶点 >10000 或 LOD >200 时按 Skia 公式压到最大 200，索引 ≤60000（`Mesh.Indices` 是 uint16）。
- **Coons**：`S(u,v)=s0+s1-s2`；**Bicubic**：4×4 Bernstein 张量积。
- **颜色**：四角转 premul → bilerp → 回 straight，和 `SkPatchUtils.cpp:352-357` 同口径。
- **绘制**：生成 `Mesh{Positions,Colors,Indices}` 后调 `Context.DrawMesh`；GPU 走 `QueueColoredMeshIndexed`，CPU 走真 Gouraud；clip/mask/CTM/投影沿用现成能力。
- **不做**：新 WGSL、新 pipeline、纹理坐标 patch、patch 内 Catmull-Rom 颜色、高层 `MeshGradient` 类型（D1/D2/D3 拍板）。

## 3 GLES baseVertex 修复（本项真实引擎洞）

- **现象**：go/GL 首次跑真窗，卡 A 只画出第一块 patch 的三角扇，卡 B/C/D 空白；pixel probes 大面积读到卡片背景 `(18,20,25)`；native 完全正常。
- **根因**：`gpu/gwgpu/gles/command_execute.go` 的 `DrawIndexedCommand.Execute` 只把 `firstIndex` 折成索引偏移，**`baseVertex` 存了但从未使用**。render 侧把多 mesh 顶点拼进同一 buffer，用 `baseVertex=firstVertex` 区分；GL 忽略它 ⇒ 所有索引 mesh 都从 buffer 顶点 0 读。
- **标准修法**：GLES 没有可移植的 `glDrawElementsBaseVertex`（GLES 3.2/EXT 才有），把 `baseVertex` 折进顶点属性偏移——`state_cache.go` 新增 `applyBaseVertex`（只动 `divisor==0` 的逐顶点属性，按 `baseOffset + baseVertex*stride` 重发 `glVertexAttribPointer`）；非索引 `DrawCommand` 先归零，避免上一条 indexed draw 的偏移泄漏。
- **验证**：go/GL 真窗像素 16/16 + 金图 0.0000%；单元测试 `gpu/gwgpu/gles/state_cache_test.go` `TestBaseVertexAttribOffset` 5/5；负向对照 `negative-gl-basevertex.txt`（临时摘掉该行 → 41.3068% 金图差、13 条 pixel FAIL，恢复后复绿）。

## 4 能力单测（逐文件 PASS 日志）

| 文件 | 测试 | 日志 |
|---|---|---|
| `render/mesh_patch_test.go` | Coons 控制点顺序/角角色、平面恒等、premul 角色、LOD/封顶/非有限、Tensor 曲面、2×2/3×3 共享角、非法形状、CTM LOD、CPU 像素探针 | `unit-tests/mesh_patch.txt`（10 PASS） |
| `render/mesh_patch_gpu_test.go` | GPU 角角色/中心、GPU vs CPU（changed ≤0.1%、max ≤2）、2×2 网格接缝 alpha | `unit-tests/mesh_patch_gpu.txt`（3 PASS） |
| `render/recording/mesh_patch_test.go` | 录制→回放与直接画逐像素一致、命令包围盒 | `unit-tests/recording_mesh_patch.txt`（2 PASS） |
| `gpu/gwgpu/gles/state_cache_test.go` | `baseVertexAttribOffset` 5 组 | `unit-tests/gles_basevertex.txt`（5/5） |

红测：`unit-tests/red.txt`——API 未实现时 `go test ./render -run '^TestMeshPatch'` 编译失败（`undefined: MeshPatchStyle/MeshPatch/meshPatchLOD/tessellateMeshPatch`），实现后转绿。

## 5 真窗（三证据）

- 窗口：`examples/ui_wr_p39_meshgradient`（1200×800，四卡）
  - A 2×2 Coons patch 网格（4 patch / 9 共享顶点）
  - B 3×3 Coons patch 网格（9 patch / 16 共享顶点）
  - C 2×2 bicubic（16 控制点 4×4 控制格，白点=控制点）
  - D 角角色 TL/TR/BR/BL + 半透明角 alpha 叠棋盘
- 运行：`-auto-only` 自动判 + 常驻人工窗；`RUN_SECONDS=6`；native 与 `GPUI_BACKEND=go` 各 seed→verify。
- JSON：`window/native-report.json`、`window/go-report.json`（logic 6/6、pixel 16/16、golden 0 / 763584 px、gpu_ops>0、cpu_fallback 0、card_triangles 与期望逐项一致）。
- PNG：`window/p39_meshgradient_native.png`、`window/p39_meshgradient_go.png`；金图：`window/p39_meshgradient_native_golden.png`、`window/p39_meshgradient_go_golden.png`（两后端金图 SHA256 相同）。
- 金图 diff 结论：`golden-diff.txt`——native/go 均 **0.000000% / 763584 px**；P0.1 逐位口径，容差 0。
- 日志：`window-native-seed.txt`、`window-native-verify.txt`、`window-gles-seed.txt`、`window-gles-verify.txt`、`window-native-first.txt`。

## 6 负向对照

1. **角角色破坏**（能力级）：`GPUI_P39_BREAK=swap-corners` 交换 TL/BR 角色色 → 金图 41.6356%、pixel 12 条 FAIL、`break_inactive` 逻辑红，`exit 1`；不设 env 复绿（`negative-swap-corners.txt`）。
2. **GL baseVertex 破坏**（引擎修复级）：临时把 `applyBaseVertex(ctx, c.baseVertex)` 改回 `applyBaseVertex(ctx, 0)` → go/GL 金图 41.3068%、pixel 13 条 FAIL、`exit 1`；恢复后双后端复绿（`negative-gl-basevertex.txt`）。

## 7 门禁与构建

| 项 | 命令 | 结果 |
|---|---|---|
| apidoc | `go run ./scripts/apidoc` | rc=0（`apidoc.txt`；`apidoc-before.txt` 保留修复前 6 个缺符号） |
| vet | `go vet ./render ./render/recording ./gpu/gwgpu/gles ./examples/ui_wr_p39_meshgradient` | rc=0（`gates/vet.txt`） |
| linux/amd64 | `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ...` | rc=0（`gates/build-linux-amd64.txt`） |
| linux/amd64 nogpu | 同上 `-tags nogpu` | rc=0 |
| linux/arm64 | 同上 | rc=0 |
| windows/amd64 | 同上（含 gles） | rc=0 |
| darwin/arm64 | 同上（不含 gles） | rc=0 |
| -race | `CGO_ENABLED=1 go test -race ./render -run '^TestMeshPatch'` / recording | 两份 ok（`gates/race_*.txt`） |
| 既有 mesh 回归 | `go test ./render -run '^(TestVertsMesh|TestVertsDraw|TestVertsSolid|TestVertsUniform|TestVertsGolden|TestVertsCPUGPUParity)'` | ok（`gates/render_verts_mesh.txt`） |
| 既有录制回归 | `go test ./render/recording -run '^(TestRecordingBounds|TestRecordingVisible|TestRecordingPaintOpCount|TestRecordingCPUScratch|TestCanvasBackend)'` | ok（`gates/recording_existing.txt`） |

## 8 已知边界（如实登记）

- gwgpu-metal：本机无 macOS 真机，未验；只算交叉编译通过（darwin/arm64 构建 rc=0）。按 §4.1 写“不可验（无真机）”，不标绿。
- paint 级 alpha/blend：P3.9 只做逐角颜色 alpha；现有彩色 mesh 管线的 paint 级 alpha/blend 缺口维持 P4.1 在册（D3 拍板）。
- patch 网格连续性：相邻 patch 共享角点由调用方保证；本项不做 Compose 式切向量自动推断（D1 单 patch 原语）。
- 纹理坐标 patch：本项不做（Skia drawPatch 的 texCoords 留后续贴图需求）。
- 投影层下的 LOD：`meshPatchLOD` 按 2×3 CTM 计算；投影层生效时几何仍由 `DrawMesh` 逐顶点投影画对，但细分密度不随透视变化（P3.7 投影层的细分优化不在本项范围）。

## 9 文件清单

- 新增：`render/mesh_patch.go`、`render/mesh_patch_test.go`、`render/mesh_patch_gpu_test.go`、`render/testdata/mesh_patch_cases.json`、`render/recording/mesh_patch_test.go`、`render/recording/testdata/mesh_patch_command.json`、`gpu/gwgpu/gles/state_cache_test.go`、`examples/ui_wr_p39_meshgradient/`。
- 修改：`render/canvas.go`、`render/recording/{command.go,command_ops.go,recorder_canvas_ops.go,recorder.go,backend.go,backend_canvas.go,bounds.go,backend_stub_test.go}`、`gpu/gwgpu/gles/{command_execute.go,state_cache.go}`、`docs/ENGINE_RENDER_GPU_CONVERGE_{PLAN,TASKS}.md`、`docs/RENDER_API_CATALOG.md`。
- 未动：`examples/ui_wr_p38_svg_showcase/golden/showcase_showcase.png`（别线）、所有未跟踪的别线文件。
