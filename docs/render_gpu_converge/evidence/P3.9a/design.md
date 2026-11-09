# P3.9a 正式方案：高层 mesh gradient 网格 + 切向量 + 颜色模型（2026-11-09，待拍板）

> 依据：`evidence/P3.9a/assessment.md` 全量复核 + AndroidX Compose `MeshGradientConfig`/`MeshGradientPainter` 源码 + 本仓 P3.9 内核。
> 状态：方案稿，零代码；D1–D5 拍板前不动实现。

## 0 直话

- 本阶段只做**彩色**高层网格：rows×columns 个 patch、每顶点位置/颜色/四切向量、Bilinear/CatmullRom 两种颜色模型。
- 几何默认 **Coons 12 点**，逐字对齐 Compose 的 Bezier patch 形状；TensorProduct 16 点继续用低层 `MeshPatch`，不在高层默认路径。
- 一次绘制把所有 patch 合成**一个 indexed `Mesh`**，调现有 `DrawMesh`；不新建 GPU 管线、不改 `gpu/hal`、不碰 WGSL。
- 不做纹理坐标/图片变形、paint alpha/blend、AA 新算法、HDR、kit 组件、PDF/SVG、Spine renderer。

## 1 公开 API（拟）

### 1.1 类型

```go
type MeshGradientColorModel uint8

const (
    // MeshGradientBilinear: each patch bilerps its four corner colors
    // (the P3.9 MeshPatch color rule).
    MeshGradientBilinear MeshGradientColorModel = iota
    // MeshGradientCatmullRom: grid-level Catmull-Rom color interpolation.
    MeshGradientCatmullRom
)

type MeshGradientSpace uint8

const (
    // MeshGradientAbsolute: vertex positions are user-space points.
    MeshGradientAbsolute MeshGradientSpace = iota
    // MeshGradientNormalized: positions are in [0,1] and map to Bounds.
    MeshGradientNormalized
)

// MeshGradientVertex is one grid vertex. Left/Top/Right/Bottom are control
// point offsets relative to Pos (Compose semantics). nil means "infer from
// neighbors"; a non-nil zero offset is an explicit sharp/zero tangent.
type MeshGradientVertex struct {
    Pos    Point
    Color  RGBA
    Left   *Point
    Top    *Point
    Right  *Point
    Bottom *Point
}

// MeshGradient is a rows x columns grid of Bezier patches.
// Vertices are row-major: (rows+1)*(columns+1) entries.
type MeshGradient struct {
    Rows, Cols int
    Vertices   []MeshGradientVertex
    ColorModel MeshGradientColorModel
    Space      MeshGradientSpace
    Bounds     Rect // used when Space == MeshGradientNormalized
}
```

### 1.2 校验与绘制口

```go
func NewMeshGradient(rows, cols int, vertices []MeshGradientVertex) (*MeshGradient, error)

func (c *Context) DrawMeshGradient(g *MeshGradient)
func (c *Context) DrawMeshGradientEx(g *MeshGradient, opts VertDrawOptions) (VertDrawResult, error)
```

- `NewMeshGradient` 校验 rows/cols ≥1、顶点数 = (rows+1)×(cols+1)、位置/颜色/切向量有限；颜色模型/坐标空间合法。
- `DrawMeshGradient` 静默跳过非法/非有限；`Ex` 返回哨兵错（`ErrMeshGradientShape`/复用 `ErrVertsNonFinite`）。
- `Canvas`/Recorder 加 `DrawMeshGradient(g *MeshGradient)`；录制命令见 §4。

### 1.3 与低层 `MeshPatch` 的关系

- `MeshPatch` 是单 patch 低层口，保留 TensorProduct/Bicubic。
- `MeshGradient` 是高层网格，默认 Coons；需要 Tensor 的用户继续直接画 `MeshPatch`。
- 高层不新增 patch 风格枚举，避免两套默认值。

## 2 切向量自动推断（逐字对齐 Compose）

AndroidX `MeshGradientConfig.inferBezierControlPointsIfRequired()` 的规则：

- 每个顶点取左/右邻居（边界时邻居=自身）、上/下邻居（边界时邻居=自身）。
- `horizontalVector = normalize(rightNeighbor - leftNeighbor) * 0.33`
- `leftOffset  = -horizontalVector * distance(current, leftNeighbor)`
- `rightOffset =  horizontalVector * distance(current, rightNeighbor)`
- `verticalVector = normalize(bottomNeighbor - topNeighbor) * 0.33`
- `topOffset    = -verticalVector * distance(current, topNeighbor)`
- `bottomOffset =  verticalVector * distance(current, bottomNeighbor)`
- 归一化零向量 → 零；边界距离 0 → 零偏移。
- 只对 `nil` 的切向量推断；显式给的切向量（含零向量）不覆盖。

这套规则保证相邻 patch 在共享边上的控制点来自同一对顶点切向量，C1 连续；显式切向量由调用方负责连续性（可做尖点/折痕）。

## 3 几何与颜色

### 3.1 每个 cell 的 Coons patch
- 四角 TL/TR/BR/BL；边控制点：
  - top: `TL + TL.Right`, `TR + TR.Left`
  - right: `TR + TR.Bottom`, `BR + BR.Top`
  - bottom: `BL + BL.Right`, `BR + BR.Left`
  - left: `TL + TL.Bottom`, `BL + BL.Top`
- 按 P3.9 的 Skia 顺序拼 12 点（top 0..3 / right 3..6 / bottom 9..6 / left 0,11,10,9），得到 `MeshPatch{Style: MeshPatchCoons}`。
- 复用 `meshPatchLOD(patch, ctm)` 与 `meshPatchSurfacePoint(patch,u,v)`；不重写 Coons 求值。

### 3.2 Bilinear 颜色
- 直接复用 `meshPatchColorAt`（四角 premul bilerp → straight）。
- 对每个 cell 用现有 `tessellateMeshPatch` 或抽出的“按颜色函数细分”内部口；把多个 cell 的顶点/索引拼接成一个 `Mesh`（索引加顶点偏移）。
- 与逐 patch 调 `DrawMeshPatch` 的结果逐顶点一致（单元测试证明）。

### 3.3 Catmull-Rom 颜色
- 网格级 4×4 邻域：cell (r,c) 取行 `r-1..r+2`、列 `c-1..c+2`，边界钳制到 [0,rows]/[0,cols]。
- 对每个 `(u,v)`：先在 u 方向做 Catmull-Rom 4 点插值（每行），再在 v 方向做 4 点插值；标准权重：
  ```
  w0 = -0.5t^3 + t^2 - 0.5t
  w1 =  1.5t^3 - 2.5t^2 + 1
  w2 = -1.5t^3 + 2t^2 + 0.5t
  w3 =  0.5t^3 - 0.5t^2
  ```
- 颜色在 premul 域插值（与 P3.9 一致），再 unpremul；钳到 [0,1]。
- 每个 cell 的几何仍走 Coons；只有颜色函数换成网格级 Catmull-Rom。
- 边界颜色连续：相邻 cell 共享同一 1D Catmull-Rom 边，值/一阶导一致；单元测试钉住。

### 3.4 输出与合批
- `DrawMeshGradientEx` 为所有 cell 生成一个 `Mesh`（positions/colors/indices），只调一次 `c.DrawMesh(mesh)`；GPU 走 `QueueColoredMeshIndexed`，CPU 走真 Gouraud。
- LOD 按每个 cell 的 `meshPatchLOD`；拼接时索引加 baseVertex。
- `Triangles` = 所有 cell 的 `lodX*lodY*2` 之和；`Degraded` 与 `DrawMesh` 同口径。
- 不逐 patch 发命令，避免 9/16 个 draw call；这也是本阶段顺带的合批收益。

## 4 录制

- 新 `CmdDrawMeshGradient` + `DrawMeshGradientCommand{MeshGradient render.MeshGradient, Brush BrushRef}`。
- Recorder 存高层网格（克隆 vertices 与切向量），回放期按回放 CTM 重新细分；与 Skia picture 的 drawPatch 行为一致。
- `Backend.DrawMeshGradient`；`CanvasBackend` 调 `ApplyBrush + dc.DrawMeshGradient`；`bounds.go` 用顶点+切向量 AABB。
- 旧的 `DrawMesh`/`DrawMeshPatch` 命令零改动。

## 5 测试与真窗

### 5.1 数据
- `render/testdata/mesh_gradient_cases.json`：rows/cols、顶点位置/颜色/显式切向量、期望推断切向量、期望 Catmull-Rom 采样色、期望 LOD/三角形数。

### 5.2 能力单测
- `render/mesh_gradient_test.go`（新增）：
  1. `TestMeshGradientTangentInference`：3×3 网格，逐顶点对比 Compose 公式（含边界零偏移、0.33 系数）。
  2. `TestMeshGradientC1SharedEdges`：相邻 cell 共享边控制点一致；共享顶点两侧切向量方向相反/长度按公式。
  3. `TestMeshGradientBilinearMatchesPatches`：Bilinear 网格与逐 cell `MeshPatch` 细分结果逐顶点/颜色一致。
  4. `TestMeshGradientCatmullRomColor`：3×3 颜色网格，cell 中心/边/角采样对比 JSON 期望。
  5. `TestMeshGradientBoundarySizes`：1×1、2×2、3×3、4×4 顶点数/索引范围/无 panic。
  6. `TestMeshGradientNormalizedSpace`：归一化顶点 + Bounds → 绝对坐标；边界/零 Bounds 拒绝。
  7. `TestMeshGradientInvalidShape`：rows/cols<1、顶点数不对、非有限位置/颜色/切向量。
  8. `TestMeshGradientCTM`：Scale(2) 后 LOD 增长、位置仍随 CTM。
  9. `TestMeshGradientDrawCPUProbes`：Bilinear/CatmullRom 角点/中心/接缝像素探针。
- `render/mesh_gradient_gpu_test.go`（新增，`!nogpu`）：
  1. `TestMeshGradientGPUProbes`：GPU 角/中心像素。
  2. `TestMeshGradientGPUMatchesCPU`：changed ≤0.1%、max delta ≤2。
  3. `TestMeshGradientGPUSeams`：2×2/3×3 接缝无背景缝。
- `render/recording/mesh_gradient_test.go`（新增）：
  1. `TestRecorderDrawMeshGradientRoundTrip`：录制→回放与直接画逐像素一致。
  2. `TestRecorderDrawMeshGradientBounds`：包围盒含顶点+切向量。
- 负向单测：切向量清零 / 颜色模型交换 → 对应断言红（测试内显式坏数据，不改生产）。

### 5.3 真窗
- `examples/ui_wr_p39a_meshgradient_grid`：
  - 卡 A：1×1（4 顶点）Bilinear vs CatmullRom。
  - 卡 B：2×2 自动切向量。
  - 卡 C：3×3 自动切向量 + 一个显式尖点切向量。
  - 卡 D：4×4 CatmullRom 颜色（16 色）。
  - 逻辑探针：切向量推断值、C1 共享边、Bilinear 对照、CatmullRom 采样、顶点/三角形数。
  - 像素探针：四角、cell 中心、共享边列、CatmullRom 中心色；接缝无背景。
  - 金图：`testdata/goldens/p39a_meshgradient_grid_final.png`，容差 0；native/go 各自基线（若逐位不同）。
  - 负向：`GPUI_P39A_BREAK=zero-tangents` → 接缝/像素红；`GPUI_P39A_BREAK=swap-color-model` → 颜色对照红；还原复绿。

### 5.4 门禁
- 单测逐文件；真窗 native + `GPUI_BACKEND=go` 各 seed→verify；金图 0；apidoc rc=0；vet；五目标构建；`-race` 定向。
- metal 不可验（无 macOS 真机），只写原因。

## 6 影响面与收敛边界

- 只新增 `render/mesh_gradient.go` + Canvas/Recorder 命令 + 测试/真窗/证据；`mesh_patch.go` 只抽内部“按颜色函数细分”口，不改 P3.9 行为。
- 复用：`meshPatchLOD`/`meshPatchSurfacePoint`/`meshPatchColorAt`/`DrawMesh`/`QueueColoredMeshIndexed`/CPU Gouraud/录制资源池。
- 不新建：GPU pipeline、WGSL、纹理管线、贴图缓存、AA、filter graph、scene tag、kit 组件、PDF/SVG、Spine renderer。
- 性能：一次 draw call；每帧 CPU 细分（静态可被 layer/picture 缓存；动画性能归阶段 4 基准）。

## 7 拍板结果（2026-11-09，用户确认）

- **D0 = 阶段 3 暂不做**：本次只做彩色高层网格；纹理/图片变形另立通用 textured mesh 任务。
- **D1-D3 = 推荐组合**：高层 `MeshGradient` + `Context.DrawMeshGradient/Ex` + Canvas/Recorder；`*Point` 四切向量（nil=自动推断，零向量=尖点）；绝对坐标 + 可选 `Bounds` 归一化映射。
- **D4 = 新 `DrawMeshGradientCommand`**：存高层网格，回放期按回放 CTM 重新细分。
- **D5 = 高层默认 Coons 12 点**：TensorProduct 16 点继续用低层 `MeshPatch`。

## 8 完成判定（§4.1 两态）

- ✅ 像素收口：高层网格新画面被真窗金图逐位比对或显式像素探针断言过；1×1/2×2/3×3/4×4、Bilinear/CatmullRom、自动/手动切向量都要被画出来并断言。
- 三证据：逻辑探针 + 像素探针 + 金图；单测逐文件 PASS；双后端；负向红→还原绿。
