# D12 ClipRRect 双路不等价（L3-A3，方案 B→F）证据

## 复现（修前红，实测）
`render/p35_cliprrect_paths_test.go` 前身（两腿对拍）：同几何仅 transform 守卫翻档换渲染路，
弧带 8 像素采样（48px 画布、rrect(4,4)-(44,44)）：
- r=2 skew=1e-9：初测差 17/255（第一版钉），改椭圆真形后同姿态差 96/255（快路 96 vs 真形 0，均为真测）
- r=2 skew=0.01：差 69；r=2 skew=0.1：差 255
- r=8 skew=0.1：差 39；r≥8 微姿态多为 0（弧不跨采样带）
- 旋转 0.01°（修前，负向对照树实测）：微旋转跨守卫差 66/255（旋转必走栅格）

## 拍板链（三次 question 停报）
1. 首拍方向 B：解析圆角矩形带旋转（Skia analytic-AA 标准）→ 实施。
2. 实测补充拍板 F：轴对齐非等比「快路均匀语义 vs 真形语义」的守卫量化差不可零成本消灭
   （保快路存量像素前提下），改判「非等比补进解析腿」——解析椭圆腿与 raster 族在
   ε→0 与边界逐位重合，中段差 ≤67 是栅格 cubic 近似的固有误差（解析=真几何）。
3. 存量像素：快路守卫维持 1e-12 容差（近均匀逐位不变，F2 钉死）。

## 实现
- `render/internal/clip/stack.go`：`RRectClip` 加 `RadiusY/Angle`；`PushRRectOriented`
  （bounds=旋转 AABB+1px 像素中心余量）；`rrectSDF` 三族：圆（原式）/椭圆（牛顿投影
  `ellipseCornerDist`，隐函数定号，严格均匀时与圆式逐位等）/带角（样点反旋）；
  路由谓词 `HasMaskClip/IsRRectOnly/RRectBounds` 把 GPU 不可表示条目（角度/椭圆）按 mask 级处理。
- `render/internal/clip/mask_cache.go`：条目哈希补 RadiusY/Angle。
- `render/context_clip.go`：`ClipRRect` 三分流——快路（轴对齐严格等比，原样原像素）→
  `tryOrientedRRectClip`（simple 型：轴对齐非等比=椭圆真形；旋转等比=角度解析、限圆角）→
  路径栅格（四角异径/九宫格/复杂/剪切变换）；解析腿同步记 `gpuClipPath`（GPU 深度裁剪不变）。
- 负向对照：摘解析腿（`false && ok`）——旋转跨守卫跳回 66/255、F3 表 {0,2,109,187,127}
  （栅格 cubic 近似值）→ 还原绿。

## 钉（三块全绿）
- F1 `GuardContinuity`：微旋转差 ≤2（实测 0，逐位连续）；微非等比观测留档（固有量化差注释说明）。
- F2 `UniformPixelsUnchanged`：ε=1e-13 与严格均匀 0 像素差（快路存量像素钉）。
- F3 `DeformContinuity`：非等比扫掠钉解析表 {0,6,176,224,127} + 5% 角弧回扫边界。
- 回归：`TestD12|TestClip|TestRRect|TestRound|TestCurve` 55 PASS 0 FAIL；
  `go vet ./render/ ./render/internal/clip/` 净。

## 真窗（画面腿）
- p35_rrect4 native：logic 6/6 golden=0.0000%。
- p35_rrect4 go：golden 0.0038%（29px 1-LSB）——与 HEAD(17574a71) worktree 基线**差异像素集合逐点相同**
  → 存量 gles 角点量化分叉，非 D12 引入（登记 findings-register）。
- p31 native/go、p33 go：golden=0.0000%（clip 栈谓词改动零牵连）。
