# P3.5 批 2 / 四角独立 RRect 交付证据

**日期**：2026-11-01　**HEAD**：`dd7e2635`（批 1 已提交未推送）　**分支**：`feat/p2-收敛整合`　**树**：P3.5 批 2 工作树（未提交）
**范围**：仅 P3.5 批 2（`RRect` 类型 + Skia 归一化/类型归约 + `Path.RRect`/`PathBuilder.RRect` + `Context.DrawRRect`/`ClipRRect` + Canvas/Recorder 同步分流）。批 3–5 未开始，按 D3 拍板：uniform 圆角继续走现有 SDF/ClipParams 快路，complex/椭圆/九宫格/旋转/非等比缩放落精确路径裁剪；**不改 13 份 WGSL、不改 `ClipRoundRect`/`ClipParams`/单半径 API/scene 编码**。

## 1. 结论

批 2 的类型、归约、路径构造、Draw/Clip 分流、录制面同步与值级/像素级/回放级测试完成；apidoc、vet、定向测试、`-race -count=2`、负向对照红→绿均通过；逐文件回归零新增红（主 687 PASS/9 FAIL——8 条基线逐名复现+1 条时序贴线红复跑三连绿定性，较批 1 还少 1 条未复现老红；补充分包 70 PASS/6 FAIL 全为 HEAD 既有 P3.1 raster 坏包）。P3.5 整体仍 🟡（批 3 Matrix4、批 4 PathEffect、批 5 布尔保曲线未开始）。

## 2. 交付物

### 2.1 公开 API（已登记 `docs/RENDER_API_CATALOG.md`，apidoc rc=0）

| 符号 | 语义 |
|---|---|
| `RRect` + `NewRRect`/`NewRRectXY`/`NewRRectRadii` | SkRRect 同位物：轴对齐矩形 + 每角独立 (rx,ry)；零值 = empty |
| `RRect.SetEmpty/SetRect/SetOval/SetRectXY/SetRectRadii` | 构造器族，全部按 Skia 规则归一化 |
| `RRect.Type/Rect/Bounds/Width/Height/Radii/RadiiArray` | 类型查询 + 几何查询（`RadiiArray` 返回拷贝） |
| `RRect.IsEmpty/IsRect/IsOval/IsSimple/IsSimpleCircular/IsNinePatch/IsComplex` | 类型判存口 |
| `RRectType`（`RRectEmpty/RRectRect/RRectOval/RRectSimple/RRectNinePatch/RRectComplex` + `String()`） | 六态类型枚举，对齐 `SkRRect::Type` |
| `RRectCorner`（`RRectUpperLeft/RRectUpperRight/RRectLowerRight/RRectLowerLeft`） | 角序 = Skia 同序（UL,UR,LR,LL） |
| `Path.RRect(rr)` / `PathBuilder.RRect(rr)` | 四角独立半径路径构造（10 verb：Move+4×(Line+Cubic)+Close，k=0.5522847498307936） |
| `Context.DrawRRect(rr)` + `Canvas` 同名口 | 追加进当前路径（empty 直返、rect 走 `DrawRectangle`、其余经 `appendUserPath` 更新 bounds/damage） |
| `Context.ClipRRect(rr)` + `Canvas` 同名口 | 圆角裁剪分流（见 §3）；**不清空当前路径** |
| `Recorder.DrawRRect`/`ClipRRect` | 录制面同步；uniform 录 `ClipRoundRectCommand`，complex 录 `SetClipCommand` |

计数（go/ast 实测）：类型 +3、顶层函数 +3、常量 +10、Context 方法 +2、`Canvas` 107→109、`Recorder` 131→133、`MaskCanvas` 112→114（随 Canvas 嵌入口径联动）。

### 2.2 Skia 风格归一化（`render/rrect.go`）

- 矩形非有限或宽高 ≤0 → `Empty`；半径 NaN/±Inf/≤0 → 直角 `Rect`（SkRRect::setRectRadii 同款）。
- 单轴为 0 → 整角变方（`rrClampRadiiToZero`）。
- 相邻半径和超边长 → 按 CSS Backgrounds 5.5 / `SkRRect::scaleRadii` 最小边比等比收缩，收缩后逐对 `rrAdjustRadiiPair` 修浮点回溢（对齐 `SkScaleToSides::AdjustRadii`）。
- 类型归约 `computeType`：全角方→Rect；全等且 ≥ 半宽高→Oval；全等→Simple；侧向配对相等（ul.X==ll.X && ul.Y==ur.Y && ur.X==lr.X && ll.Y==lr.Y）→NinePatch；其余→Complex。

### 2.3 Draw/Clip 分流

`Path.RRect` 全同角与 `PathBuilder.RoundRect` 逐 verb 逐坐标等价（钉见 §4），故 uniform 快路复用现有链不会改变任何存量像素。

| 输入 | `Context.ClipRRect` | `Recorder.ClipRRect` |
|---|---|---|
| Empty | `ClipRect(0×0)`（空区） | 同左（`ClipRectCommand`） |
| Rect（零半径） | `ClipRect`（scissor 快路） | `ClipRectCommand` |
| Simple 圆形 + 轴对齐等比变换（含 Oval 圆） | `ClipRoundRect` → 现 SDF/ClipParams 快路，不动 | 录 `ClipRoundRectCommand`（回放走快路） |
| 椭圆角（rx≠ry）/ NinePatch / Complex | 精确设备路径裁剪（`totalMatrix` 烘路径 → `clipStack.PushPath` + `gpuClipPath` 深度裁剪链） | 用户空间 path 入资源池 + `SetClipCommand`（不清 currentPath） |
| Simple 圆形 + 旋转/错切/非等比缩放 | 同上落路径（SDF 快路在这些变换下不保形） | 同左 |

快路守卫 `canUseRRectClipFastPath`：类型 ∈ {Simple(圆形), Oval(圆)} 且 `tm.B==0 && tm.C==0` 且 `|tm.A|==|tm.E|≠0`（实况侧相对误差容差 1e-12）。

## 3. 测试钉

| 钉 | 内容 | 结果 |
|---|---|---|
| `TestRRectTypeReduction` | 六类型各一例 + 归一化后四角值逐项对账 | PASS |
| `TestRRectTypePresenceHelpers` | IsRect/IsSimple/IsOval/IsNinePatch/IsComplex 判存 | PASS |
| `TestRRectRadiiClampAndScale` | 重叠半径等比缩放到 30、单轴 0 整角变方、负半径→Rect | PASS |
| `TestRRectCornerAccessAndRect` | 四角独立值存取、`RadiiArray` 返回拷贝 | PASS |
| `TestRRectPathUsesPerCornerRadii` | 四角各异路径 10 verb + 9 个关键锚点坐标值级钉 | PASS |
| `TestRRectPathSimpleMatchesRoundRect` | 全同角 vs `PathBuilder.RoundRect` 逐 verb/逐坐标等价（1e-9） | PASS |
| `TestContextDrawRRectSimpleMatchesRoundedRectangle` | 实况 `DrawRRect(uniform)` vs `DrawRoundedRectangle` 像素缓冲逐字节相等 | PASS |
| `TestDrawRRectPerCornerCoverage` | 四角各异（30/10/20/5）fill：角外白、角内红、中心红像素钉 | PASS |
| `TestClipRRectDispatch` | 分流探针 6 例：uniform 快路（`IsRRectOnly`、无 gpuClipPath）/椭圆/九宫格/complex/旋转 uniform/非等比缩放各落精确路径（gpuClipPath 置位） | PASS |
| `TestClipRRectCoveragePixels` | complex 裁剪像素钉：内部红、角外白、角弧内红、AA 边带部分覆盖（G∈(0.2,0.98)） | PASS |
| `TestRecorderDrawRRect` | 录制 10 verb + 起点坐标 + `Fill` 落 `CmdFillPath` | PASS |
| `TestRecorderClipRRectDispatch` | uniform→`ClipRoundRectCommand(20,20,120,120,r=12)`；complex→`SetClipCommand`（路径 10 verb）；旋转 uniform→`SetClipCommand` | PASS |
| `TestRecorderClipRRectKeepsCurrentPath` | complex 裁剪后 currentPath verb 数不变 | PASS |
| `TestRecorderClipRRectPlaybackMatchesLive` | uniform/complex 两例录制回放 vs 实况 `Pix` 逐字节相等 | PASS |

定向复跑：`targeted-render-batch2.txt`；`-race -count=2` 双组：`race-batch2.txt`（rc=0）。

## 4. 负向对照（红→还原绿）

临时把 `Path.RRect` 退化成普通矩形（强制 `p.Rectangle(...)`）：

```
--- FAIL: TestDrawRRectPerCornerCoverage (0.00s)
    rrect_test.go:235: outside top-left corner = {R:1 G:0 B:0 A:1}, want white
--- FAIL: TestClipRRectCoveragePixels (0.00s)
    rrect_test.go:308: outside corner = {R:1 G:0 B:0 A:1}, want white
FAIL	github.com/energye/gpui/render	0.014s
```

还原后两测绿：`ok github.com/energye/gpui/render 0.014s`。原始输出归档 `negative-control-batch2/`（`path-rrect-degraded-red.txt`/`restored-green.txt`）。工作区已确认无「强制矩形」残留（`Path.RRect` 四角 cubic 构造在位，本批定向测试全绿即证）。

## 5. 门禁

| 门禁 | 命令 | 结果 |
|---|---|---|
| apidoc | `go run ./scripts/apidoc` | ✅ rc=0（`apidoc-batch2.txt`） |
| vet | `go vet ./render ./render/recording ./render/scene ./render/internal/gpu ./render/internal/stroke` | ✅ rc=0（`vet-batch2.txt`；批 2 曾引入 recording 测试跨包 `render.Point` 未命名字面量违规，已改 keyed 后复跑绿） |
| 定向测试 | render 10 条 + recording 4 条 `-count=1` | ✅ 全 PASS（`targeted-render-batch2.txt`） |
| `-race` | 双定向组 `-race -count=2`（CGO_ENABLED=1） | ✅ rc=0（`race-batch2.txt`） |
| 逐文件回归（主清单 34 包） | `tools/per-file-tests.sh tools/per-file-tests.pkgs` | ✅ 687 PASS / 21 SKIP_TAGGED / 9 FAIL / 1 NO_TESTS（+1 PASS=本批 `render/rrect_test.go`）；9 FAIL=8 条基线逐名复现 + `ui/rendering/m1_noquadratic_test.go` 时序贴线红（比值 15.34 vs 门禁 15，单文件复跑三连绿，非批 2 代码路径），批 1 老红 `s6_9_heavy_budget` 本轮 PASS；零新增回归。见 `per-file-tests-batch2/summary.txt` |
| 补充分包逐文件 | `tools/per-file-tests.sh evidence/P3.5/per-file-extra.pkgs` | ✅ 70 PASS / 2 SKIP_TAGGED / 6 FAIL（+1 PASS=本批 `render/recording/rrect_test.go`）；6 FAIL 全为 HEAD 既有坏包 `render/recording/backends/raster`（P3.1 参数漏改债）；raster/tilecompute/stroke 金图文件全绿=存量金图零变化像素证据。见 `per-file-tests-batch2-extra/summary.txt` |

## 6. 硬约束遵守清单

- **13 份 WGSL shader 未动**、`ClipParams`/`clip_params.go` 未动、`ClipRoundRect`/`DrawRoundedRectangle(XY)`/`Path.RoundedRectangle`/`PathBuilder.RoundRect` 单半径 API 未动（diff 文件清单可证：本批改动仅 rrect.go、path_build.go、path_builder.go、context_path.go、context_clip.go、canvas.go、recorder_path_draw.go + 两测试文件 + 三文档）。
- **scene 编码未动**（`render/scene/` 零 diff），存量单半径录制命令 `ClipRoundRectCommand` 字段未动，uniform 路径复用原命令原快路。
- **存量金图零变化**：本批不改任何现有构造的输出（`Path.RRect` 为新增 verb 序列构造，全同角等价钉 §3；fast path 分流命中即走原链），补充分包门禁含 raster/tilecompute/stroke 金图文件逐文件 PASS 即像素级证据。
- `render/` 无后端特有类型（grep 检查通过）。
- 未在 examples/ 绕洞：本批零示例改动。

## 7. 已知基线红（非本批引入，不修）

- 主逐文件 9 条 FAIL：8 条与批 1 基线逐名一致（P3.3/P3.2 老红名单）；第 9 条 `ui/rendering/m1_noquadratic_test.go` 为墙钟比值门禁贴线抖动（15.34 vs 门禁 15，测文本 BoxesForRange 复杂度、不调用批 2 任何代码），门禁后单文件复跑三连绿；批 1 老红 `render/s6_9_heavy_budget_test.go` 本轮 PASS，名单只减不增（详见 `per-file-tests-batch2/summary.txt`）。
- 补充分包 6 条 FAIL = HEAD 既有坏包 `render/recording/backends/raster`（P3.1 `render.BlendSpace` 参数漏改债，批 1 已登记 batch1.md §7）。
- 宽泛正则 `TestClipRRect` 会选中既有 `TestClipRRectGPUTextureAndText`，无 `WGPU_NATIVE_PATH` 环境下 GPU 不可用失败——基线环境红，与本批无关，定向清单已用精确测试名规避。

## 8. 未做项与遗留

- **真窗未建**：`examples/ui_wr_p35_pathgeom` 四角 RRect 卡（各异半径+嵌套 clip，双后端+金图）按计划随批 3–5 或批 2 补窗统一建；当前只有单测/像素/录制回放证据（原子验证口径，AGENTS「已验证待接线」）。
- 九宫格/椭圆 uniform 场景不做 SDF 快路（D3 拍板范围），复测如真窗 AA 不达标再议 ClipParams 扩 48B 方案。
- `ui` 层消费接线（decor/clip_rrect 等）不在批 2 范围。
- 批 3 Matrix4、批 4 PathEffect、批 5 布尔保曲线未开始；P3.5 整体 🟡 不变。

## 9. 修订

- 2026-11-01：批 2 交付登记（代码 + catalog §0/§3/§3.2/§8/§9/§13 + 台账 P3.5 行）；`docs/ENGINE_UI_WIDGET_RENDER.md` §10 不补行——P3.0–P3.5 各批修订记录统一落本台账修订表（该行 §10 现无任何 P3.x 行，批 1 同口径），catalog 同步义务由 §0/§13 满足且 apidoc rc=0。
