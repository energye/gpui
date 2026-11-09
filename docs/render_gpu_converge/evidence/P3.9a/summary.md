# P3.9a 高层 mesh gradient 交付记录（2026-11-09，✅ 像素收口）

> 真源：`docs/ENGINE_RENDER_GPU_CONVERGE_TASKS.md` §2/§2.1/§7；方案：`assessment.md`、`design.md`。
> 结论：**✅ 像素收口**——高层网格新画面被真窗金图逐位比对（native 与 go/GL 各自 0.0000% / 763584 px，跨后端内容区 changed=0）且显式像素探针 8/8 通过；双后端、负向、单测、录制、构建/vet/apidoc/race 全齐。

## 0 直话

- 本阶段只做**彩色**高层网格：rows×columns 个 Coons patch、(rows+1)×(columns+1) 顶点、每顶点位置/颜色/可选四切向量、自动 C1 推断、Bilinear/CatmullRom 颜色模型。
- 一次绘制把所有 cell 合成一个 indexed `Mesh`，复用 P3.9 的 Coons 细分与 `DrawMesh` 彩色网格；没有新 GPU 管线、没有新 WGSL。
- 切向量推断逐字对齐 AndroidX Compose `MeshGradientConfig.inferBezierControlPointsIfRequired`（normalized neighbor vector × 0.33 × 到邻居距离；左/上取负；边界邻居=自身、距离 0 → 0）。
- 阶段 2（paint alpha/blend）仍归 P4.1；阶段 3（通用 textured mesh/图片变形）按用户拍板暂不做；阶段 4（AA/精度/合批/缓存）先验证/基准，不先写代码；阶段 5 其它任务。

## 1 公开 API

| 符号 | 说明 |
|---|---|
| `MeshGradientColorModel` + `MeshGradientBilinear` / `MeshGradientCatmullRom` | 每 patch 四角 bilerp / 网格级 Catmull-Rom |
| `MeshGradientSpace` + `MeshGradientAbsolute` / `MeshGradientNormalized` | 绝对用户空间 / [0,1] 映射 `Bounds` |
| `MeshGradientVertex{Pos,Color,Left/Top/Right/Bottom *Point}` | 四切向量为相对偏移；nil=自动推断，非 nil 零向量=尖点 |
| `MeshGradient{Rows,Cols,Vertices,ColorModel,Space,Bounds}` | 行主序网格 |
| `NewMeshGradient(rows,cols,vertices)` | 校验 + 深拷贝 |
| `ErrMeshGradientShape` | 形状/模型/空间/归一化 Bounds 非法；非有限复用 `ErrVertsNonFinite` |
| `MeshGradient.Validate/Clone/ControlBounds` | 校验、深拷贝、顶点+切向量控制点 AABB（录制用） |
| `Context.DrawMeshGradient/DrawMeshGradientEx` | 一次绘制合成一个 indexed Mesh；Ex 返回 Triangles/Degraded/Skipped |
| `Canvas.DrawMeshGradient` + `recording.DrawMeshGradientCommand`/`CmdDrawMeshGradient`/`Backend.DrawMeshGradient` | 录制存高层网格，回放期按回放 CTM 重新细分 |

## 2 算法

- **切向量推断**：Compose 公式；只对 nil 推断，显式值不覆盖；相邻 cell 共享同一 resolved 顶点，C1 共享边控制点一致。
- **几何**：每个 cell 由四角和四切向量生成 Coons 12 点（Skia 顺序）；LOD 复用 `meshPatchLOD`，表面求值复用 `meshPatchSurfacePoint`。
- **Bilinear**：复用 `meshPatchColorAt`（premul 四角 bilerp）。
- **CatmullRom**：cell 的 4×4 邻域（边界钳制），u/v 两次标准 Catmull-Rom 权重；premul 插值后 unpremul + clamp；相邻 cell 共享同一 1D 边，值/一阶导连续。
- **合批**：所有 cell 依次 tessellate，索引加 baseVertex，合成一个 `Mesh`；超过 uint16 上限时按 chunk flush（每 chunk 一次 DrawMesh）。
- **录制**：存 `MeshGradient` 深拷贝；回放期按回放 CTM 重新推断/细分。

## 3 能力单测（逐文件 PASS 日志）

| 文件 | 测试 | 日志 |
|---|---|---|
| `render/mesh_gradient_test.go` | 切向量推断（Compose 公式）、C1 共享边、Bilinear 与逐 patch 一致、CatmullRom 采样、1×1/2×2/3×3/4×4、归一化空间、非法形状、CTM LOD、CPU 像素 | `unit-tests/mesh_gradient.txt`（9 PASS） |
| `render/mesh_gradient_gpu_test.go` | GPU 角/中心、GPU vs CPU（changed ≤0.1%、max ≤2）、2×2 接缝 | `unit-tests/mesh_gradient_gpu.txt`（3 PASS） |
| `render/recording/mesh_gradient_test.go` | 录制→回放与直接画逐像素一致、包围盒 | `unit-tests/recording_mesh_gradient.txt`（2 PASS） |

红测：`unit-tests/red.txt`——API 未实现时 `go test ./render -run '^TestMeshGradient'` 编译失败（undefined MeshGradient 等），实现后转绿。

## 4 真窗（三证据）

- 窗口：`examples/ui_wr_p39a_meshgradient_grid`（1200×800，四卡）
  - A 1×1：左 Bilinear / 右 CatmullRom
  - B 2×2：自动 C1 切向量
  - C 3×3：自动 + 中心尖点切向量
  - D 4×4：CatmullRom 颜色
- 运行：`-auto-only` + 常驻人工；native 与 `GPUI_BACKEND=go` 各 seed→verify。
- JSON：`window/native-report.json`、`window/go-report.json`（logic 3/3、pixel 8/8、golden 0 / 763584 px、gpu_ops>0、cpu_fallback 0、card_triangles 一致）。
- PNG：`window/p39a_meshgradient_native.png`、`window/p39a_meshgradient_go.png`；金图：`window/p39a_meshgradient_native_golden.png`、`window/p39a_meshgradient_go_golden.png`。
- 金图 diff：`golden-diff.txt`——native/go 各自 0.000000% / 763584 px；跨后端同一内容区 **changed=0 / max=0 / mean=0**（文件 SHA 不同只因 HUD 文本，HUD 已被 ShellContentRects 排除）。

## 5 负向对照

- `GPUI_P39A_BREAK=swap-color-model`（CatmullRom→Bilinear）：金图 15.3974%、`break_inactive` 逻辑红、exit 1；还原复绿（`negative-swap-color-model.txt`）。
- `GPUI_P39A_BREAK=zero-tangents`（全部切向量清零）：金图 38.1758%、`break_inactive` 逻辑红、exit 1；还原复绿（`negative-zero-tangents.txt`）。
- 说明：这两路破坏主要改变几何/颜色模型，中心像素探针可能仍落在同色区；**金图 0 容忍**承担形状变化的红证据，符合「金图逐位」门禁。

## 6 门禁

| 项 | 命令 | 结果 |
|---|---|---|
| apidoc | `go run ./scripts/apidoc` | rc=0（`apidoc.txt`；`apidoc-before.txt` 保留 15 个缺符号） |
| vet | `go vet ./render ./render/recording ./examples/ui_wr_p39a_meshgradient_grid` | rc=0（`gates/vet.txt`） |
| linux/amd64 | `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ...` | rc=0 |
| linux/amd64 nogpu | 同上 `-tags nogpu` | rc=0 |
| linux/arm64 | 同上 | rc=0 |
| windows/amd64 | 同上 | rc=0 |
| darwin/arm64 | 同上 | rc=0 |
| -race | `CGO_ENABLED=1 go test -race ./render -run '^TestMeshGradient' -skip GPU` / recording | 两份 ok（`gates/race_*.txt`） |
| P3.9 回归 | `go test ./render -run '^TestMeshPatch' -skip GPU`、GPU、recording | 三份 ok |

## 7 已知边界（如实登记）

- 阶段 3 纹理坐标/图片变形**未做**（用户拍板暂不做）：`MeshGradient` 没有 UV/纹理；4 角图片变形仍用 `DrawImageQuad`。
- paint 级 alpha/blend 未接（P4.1）；本项只有顶点/网格颜色 alpha。
- AA 未新增：引擎全局 4x MSAA 可 opt-in；本项默认 1x。
- HDR/线性光顶点色、kit 组件、PDF/SVG、Spine renderer 均未做（阶段 5/其它任务）。
- 高层默认 Coons 12 点；TensorProduct 16 点仍用低层 `MeshPatch`（D5）。
- 手动切向量可破坏 C1（尖点/折痕由调用方负责）；自动推断只对 nil 生效。
- Catmull-Rom 颜色按标准权重实现；未与 Android 真机 Compose 渲染器做逐像素对拍（本项以真窗金图/单测为准，切向量公式已逐字对齐 Compose 源码）。
- `Space=Normalized` 不做位置 clamp；`Bounds` 必须非空。

## 8 文件清单

- 新增：`render/mesh_gradient.go`、`render/mesh_gradient_test.go`、`render/mesh_gradient_gpu_test.go`、`render/testdata/mesh_gradient_cases.json`、`render/recording/mesh_gradient_test.go`、`render/recording/testdata/mesh_gradient_command.json`、`examples/ui_wr_p39a_meshgradient_grid/`。
- 修改：`render/mesh_patch.go`（抽 `tessellateMeshPatchColors`，P3.9 行为不变）、`render/canvas.go`、`render/recording/{command.go,command_ops.go,recorder_canvas_ops.go,recorder.go,backend.go,backend_canvas.go,bounds.go,backend_stub_test.go}`、`docs/ENGINE_RENDER_GPU_CONVERGE_{PLAN,TASKS}.md`、`docs/RENDER_API_CATALOG.md`。
- 未动：`examples/ui_wr_p38_svg_showcase/golden/showcase_showcase.png`（别线）、所有未跟踪别线文件。
