# P3.9 mesh gradient 正式方案（2026-11-09 开工登记，方向待拍板）

> 依据：`evidence/P3.9/assessment.md`（file:line 实证）+ 2026-11-09 交接核对。
> 状态：方案稿，零代码；D1-D3 拍板前不动实现。

## 0 直话

- 一个 patch = 4 个角角色色 + 12 个控制点（Coons）或 16 个控制点（TensorProduct/bicubic）。
- 一个 mesh = 2×2 或 3×3 个 patch；patch 共享角点由调用方给同一坐标/颜色，示例按网格数据构造。
- 画法 = 按 Skia `SkDevice::drawPatch`：先按 LOD 把 patch 细分成带逐顶点色的三角形，再走现有 `DrawMesh` 彩色网格；GPU/CPU 双路径、clip/mask/CTM/投影全部复用现成能力。
- 不加新 WGSL、不改 `gpu/hal`、不改引擎；这是本方案与 catalog 旧口径最大的不同。

## 1 公开 API（拟）

### 1.1 类型

```go
// MeshPatchStyle selects the surface basis of one patch.
type MeshPatchStyle uint8

const (
    // MeshPatchCoons is the 12-control-point Coons patch of Skia's
    // SkCanvas::drawPatch: four cubic edges sharing corners.
    MeshPatchCoons MeshPatchStyle = iota
    // MeshPatchBicubic is the 16-control-point tensor-product (bicubic)
    // patch of PDF shading type 7: a 4x4 Bezier control lattice.
    MeshPatchBicubic
)

// MeshPatch is one colored Bezier patch. Colors are ordered clockwise from
// the top-left corner: TopLeft, TopRight, BottomRight, BottomLeft.
type MeshPatch struct {
    Style  MeshPatchStyle
    Points []Point   // 12 for Coons, 16 for Tensor
    Colors [4]RGBA   // TL, TR, BR, BL
}
```

**Coons 12 点顺序（逐字对齐 Skia）**：
```
0 = TL 角点
1 = top 第一控制点
2 = top 第二控制点
3 = TR 角点
4 = right 第一控制点
5 = right 第二控制点
6 = BR 角点
7 = bottom 第二控制点（注意 bottom 从右往左）
8 = bottom 第一控制点
9 = BL 角点
10 = left 第二控制点
11 = left 第一控制点
```
四条三次曲线：top=(0,1,2,3)，right=(3,4,5,6)，bottom=(9,8,7,6)，left=(0,11,10,9)。

**Tensor 16 点顺序（4×4 行主序）**：
```
 0  1  2  3     // v=0 顶行，u 从左到右
 4  5  6  7
 8  9 10 11
12 13 14 15     // v=1 底行
```
四角：0=TL，3=TR，12=BL，15=BR；颜色数组仍按 TL/TR/BR/BL 取用，不按点索引顺序。

### 1.2 绘制口

```go
// DrawMeshPatch draws one colored Bezier patch through the existing colored
// vertex-mesh path. Invalid/non-finite patches are skipped.
func (c *Context) DrawMeshPatch(patch MeshPatch)

// DrawMeshPatchEx validates then draws, returning the same degradation/result
// shape as DrawMeshEx.
func (c *Context) DrawMeshPatchEx(patch MeshPatch, opts VertDrawOptions) (VertDrawResult, error)
```

`Canvas` 接口新增 `DrawMeshPatch(patch MeshPatch)`，`*Recorder` 同步实现。`DrawMeshPatchEx` 不进 `Canvas`（与 `DrawMeshEx` 一致，Ex 只在 `*Context` 上）。

**非法输入契约**：
- `Style` 未知、`len(Points)` 不是 12/16、颜色/点非有限 → `DrawMeshPatch` 静默跳过；`DrawMeshPatchEx` 返回哨兵错（`ErrVertsNonFinite` 复用；新增 `ErrMeshPatchShape` 一个哨兵覆盖点数/风格）。
- 非有限 LOD、全退化 patch → 跳过，不画（Skia 同口径：GetLevelOfDetail 返回 0 后 MakeVertices 返回 nil）。

### 1.3 录制

- `render/recording/command.go`：新增 `CmdDrawMeshPatch` + 字符串。
- `command_ops.go`：`DrawMeshPatchCommand{Patch render.MeshPatch, Brush BrushRef}`。
- `Recorder.DrawMeshPatch` 记录 patch 原样（不提前细分——LOD 取决于回放时的 CTM，Skia picture 也是回放期算）。
- `recording.Backend` 新增 `DrawMeshPatch(patch render.MeshPatch, brush Brush)`；`CanvasBackend` 实现 `ApplyBrush + dc.DrawMeshPatch`；`stubBackend` 补空实现（测试 mock 都嵌入它）。
- `bounds.go`：按 12/16 个控制点的 AABB 计包围盒；`note` 用 patch 点有限 + 4 色有限。
- 旧命令、旧回放、旧测试零行为变化。

## 2 细分算法（render/mesh_patch.go）

### 2.1 LOD（对齐 SkPatchUtils::GetLevelOfDetail）
- 取四条边界三次曲线，经当前 `Context.totalMatrix()`（投影层生效时逐点 `projTransformPoint`）映射到设备空间；
- 每条按 4 个控制点的折线长度近似弧长；长度非有限 → 不画；
- `lodX = max(8, int(max(topLen,bottomLen)/10))`，`lodY = max(8, int(max(leftLen,rightLen)/10))`；
- 顶点/索引封顶：若 `(lodX+1)*(lodY+1) > 10000` 或 `lodX>200` 或 `lodY>200`，按权重压到最大 200，保证 `lodX*lodY*6 <= 60000`（沿用 Skia 的 60000 索引上限；`Mesh.Indices` 是 uint16）。

### 2.2 Coons 曲面
- 按 Skia 顺序取 top/right/bottom/left 四条三次曲线；
- 对每个 `(u,v)`：
  ```
  s0 = lerp(top(u), bottom(u), v)
  s1 = lerp(left(v), right(v), u)
  s2 = bilerp(cornerTL, cornerTR, cornerBL, cornerBR, u, v)
  P(u,v) = s0 + s1 - s2
  ```
- 颜色：四角转 premul → bilerp → 转回 straight（`float_to_skcolor` 同口径）；保证半透明角色在 premul 域插值。
- 顶点索引 `dataIndex = x*(lodY+1)+y`，每格两个三角形，索引顺序照 `SkPatchUtils.cpp:372-379`。

### 2.3 TensorProduct/bicubic 曲面
- 4×4 控制格 `P[row][col]`，u=col/3，v=row/3；
- `P(u,v)=Σ_i Σ_j B_i(v) B_j(u) P[i][j]`，Bernstein 三次基；预先算 4 个 u 权重和 4 个 v 权重，每点 16 项；
- 颜色仍按四角 bilerp（与 Skia patch 一致；Catmull-Rom 若拍板要做，另立 D2-B）。

### 2.4 输出
- 生成 `Mesh{Positions, Colors, Indices}`，位置保持**用户空间**，由 `DrawMesh` 统一应用 CTM/投影；
- 调 `c.DrawMesh(mesh)`（GPU 索引路径 / CPU 展开路径自动分流）；
- 可选：Context 内复用 `patchPos/patchCol/patchIdx` 三块 scratch，避免每 patch 每帧分配；首版可先每 patch 分配，实测分配压力再收。

### 2.5 明确不做
- 不做 patch 内 Catmull-Rom 颜色（D2 拍板后才做）；
- 不做 patch 网格自动 C1 切向量推断（Compose 的 Unspecified tangent 推断）；共享角点/切向量由调用方给；
- 不做纹理坐标 patch（Skia 有 texCoords，本任务“mesh gradient”不需要；如后续要贴图，另立）；
- 不做新 WGSL / 新 pipeline。

## 3 双后端与像素一致性

- GPU：`DrawMesh` 索引路径 → `QueueColoredMeshIndexed` → `convex.wgsl vs_mesh/fs_main`（SkipAA + unorm8 顶点色）。与 Skia drawPatch 的“忽略 AA”一致。
- CPU：`DrawVertices` 真 Gouraud（预乘重心插值 + source-over + clip/mask）。
- 预期：patch 内颜色/几何两端一致；GPU unorm8 量化 vs CPU 浮点重心会有 ≤1-2/255 的差；像素探针容差按 P0.1 的通道 ±2，金图逐位按各后端自己的基线。
- gwgpu-metal：本机无 macOS 真机，交叉编译通过也只写“不可验（无真机）”。

## 4 测试方案（先红后绿）

### 4.1 数据
- `render/testdata/mesh_patch_cases.json`：所有标准输入/期望值（控制点、角角色色、期望顶点位置/颜色、LOD、三角形数、探针点），测试只读数据，不在测试代码里循环生成标准数据。

### 4.2 能力单测
- `render/mesh_patch_test.go`（新增）：
  1. `TestMeshPatchCoonsControlOrder`：12 点顺序 + 4 角角色色；角点位置/颜色逐值断言。
  2. `TestMeshPatchCoonsPlanarIdentity`：直边矩形 patch 的 Coons 曲面退化为双线性；内部采样点误差 ≤1e-9。
  3. `TestMeshPatchCoonsPremulCornerColors`：半透明四角，中心色 = premul bilerp 后转 straight 的期望值（±1/255）。
  4. `TestMeshPatchLOD`：小 patch LOD=8；大 patch LOD 增长；封顶后索引 ≤60000、顶点 ≤10000；非有限 → 跳过。
  5. `TestMeshPatchBicubicSurface`：4×4 控制格的已知曲面在若干 (u,v) 采样值与解析值一致（≤1e-9）。
  6. `TestMeshPatchBicubicPlanarIdentity`：双线性 4×4 控制格退化为双线性曲面。
  7. `TestMeshPatchGridSharedCorners`：2×2/3×3 网格相邻 patch 共享角坐标/颜色一致；接缝两侧顶点重合。
  8. `TestMeshPatchInvalidShape`：点数/风格非法、非有限点/色 → `DrawMeshPatchEx` 哨兵错；`DrawMeshPatch` 不画。
  9. `TestMeshPatchCTMLOD`：Scale(2) 后 LOD 增长（设备空间弧长），位置仍为用户空间。
- `render/mesh_patch_gpu_test.go`（新增，`//go:build !nogpu`）：
  1. `TestMeshPatchGPUMatchesCPU`：同一 patch GPU 与 CPU 出图，changed 像素 ≤0.1%、max delta ≤2（显式容差）。
  2. `TestMeshPatchGPUCornerColors`：GPU 四角/中心像素探针 ±2。
  3. `TestMeshPatchGPUIndexedSeams`：2×2 网格无缝（接缝列无背景色/暗线）。
- `render/recording/mesh_patch_test.go`（新增）：
  1. `TestRecorderDrawMeshPatchRoundTrip`：录制 → 回放（CanvasBackend）与直接 `DrawMeshPatch` 逐像素一致。
  2. `TestRecorderDrawMeshPatchBounds`：包围盒命令计数/非空判定。
- 负向单测：把角角色顺序换掉，角点颜色断言必须红（在测试内用显式错误数据，不改生产代码）。

### 4.3 真窗
- `examples/ui_wr_p39_meshgradient`（新增）：
  - 卡 A：2×2 Coons patch 网格（3×3 顶点），波浪边 + 9 色。
  - 卡 B：3×3 Coons patch 网格（4×4 顶点），更密颜色。
  - 卡 C：2×2 TensorProduct/bicubic 网格（4×4 控制点/patch），显示内部 4 个控制点，和 Coons 对照。
  - 卡 D：角角色 + 半透明角 alpha 叠在棋盘背景上（验证 TL/TR/BR/BL 顺序与 premul 插值）。
  - 人工模式让波浪相位动；`-auto-only` 冻结相位。
  - 逻辑探针：Coons 平面恒等、LOD、角角色、premul bilerp、Tensor 曲面值、共享角一致、三角形数。
  - 像素探针：四角色、中心色、接缝列、bicubic 内部点、alpha 合成点、patch 外背景。
  - 金图：`testdata/goldens/p39_meshgradient_final.png`，容差 0（P0.1 金图口径）；native 与 go/GL 若像素有差，按 P3.8 color_font 先例各后端一份基线（`_native`/`_go`），并在证据里写清。
  - 负向：`GPUI_P39_BREAK=swap-corners` 交换 TL/BR 角角色色 → `-auto-only` 必须红（像素/金图）；不设 env 复绿。

### 4.4 门禁命令
- 单测逐文件（禁止一次全量）：
  `go test ./render -run 'TestMeshPatch' -count=1`（按文件实际拆）
  `go test ./render/recording -run 'TestRecorderDrawMeshPatch' -count=1`
  `go test ./render -run 'TestMeshPatchGPU' -count=1`（带 `WGPU_NATIVE_PATH`）
- 真窗 native：
  `WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so LD_LIBRARY_PATH=$PWD/lib RUN_SECONDS=6 go run ./examples/ui_wr_p39_meshgradient -auto-only`
- 真窗 go/GL：
  `GPUI_BACKEND=go WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so LD_LIBRARY_PATH=$PWD/lib RUN_SECONDS=6 go run ./examples/ui_wr_p39_meshgradient -auto-only`
- 金图 seed 一次：同命令加 `GPUI_ACCEPT_GOLDEN=1`；seed 后不带该 env 复跑 0.0000%。
- apidoc：`go run ./scripts/apidoc` rc=0。
- 分层/后端残留：`grep -rn "webgpu\.\|gwgpu/\|gles\." render/*.go | grep -v _test.go` 无新增；`grep -rn "gpu/" ui/ | grep -v _test.go` 无新增（本项不动 ui）。

## 5 证据目录

```
docs/render_gpu_converge/evidence/P3.9/
  assessment.md          # 本目录评估
  design.md              # 本方案
  batch1.md              # 交付记录（实现后）
  unit-tests/            # 逐文件 PASS 日志
  window-native-seed.txt / window-native-verify.txt
  window-gles-seed.txt / window-gles-verify.txt
  negative-*.txt         # 负向红→还原绿
  apidoc.txt
  golden-diff.txt
  window/                # JSON 探针、PNG、金图
```

## 6 完成判定（§4.1 两态）

- ✅ 像素收口：新能力画面被真窗金图逐位比对或显式像素探针断言过；本方案的真窗卡 A-D 必须覆盖 Coons 2×2/3×3、Tensor 2×2、角角色/alpha 四类画面。
- 若只做到“旧画面 0.0000%”不算收口；必须看到 patch 自身被画出并被探针/金图断言。
- 三证据缺一不可：能力单测逐文件 PASS + 真窗 JSON/PNG + 金图 diff 计数/容差 + 双后端 + 负向对照红过。

## 7 拍板结果（2026-11-09，用户确认）

- **D1 = A 单 patch 原语**：`MeshPatch` + `Context.DrawMeshPatch/DrawMeshPatchEx`；示例用 4/9 个 patch 拼 2×2/3×3 网格；不加高层 `MeshGradient` 类型。
- **D2 = A 16 点 TensorProduct/bicubic**：Coons=12 控制点（Skia `drawPatch`），bicubic=16 控制点 4×4 控制格（PDF type 7）；patch 内颜色按 Skia 四角 bilerp；不做 Compose 式 Catmull-Rom 颜色。
- **D3 = A paint alpha/blend 留给 P4.1**：P3.9 只做逐角颜色 alpha；现有彩色 mesh 管线的 paint 级 alpha/blend 缺口维持 P4.1 在册，不在本项扩范围。
